package sage

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/memory"
)

// pendingMemory tracks buffered metadata for the next Store call.
type pendingMemory struct {
	content  string
	category string
	owner    string
	memoryID string
	triples  []KnowledgeTriple
	tags     []string
	topic    string // LLM-provided domain topic (e.g. "profile", "project")
}

// SageBackend implements memory.MemoryBackend using Sage as the remote store.
type SageBackend struct {
	client   *Client
	identity *IdentityManager
	mu       sync.Mutex
	pending  map[string]*pendingMemory // key -> last submitted memory
}

// NewSageBackend creates a new Sage-backed memory backend.
func NewSageBackend(client *Client, identity *IdentityManager) *SageBackend {
	return &SageBackend{
		client:   client,
		identity: identity,
		pending:  make(map[string]*pendingMemory),
	}
}

// categoryMapping holds Sage memory_type, confidence, and default domain topic.
type categoryMapping struct {
	memoryType   string
	confidence   float64
	defaultTopic string // default topic when LLM doesn't specify one
}

// categoryToSage maps PicoClaw categories to Sage memory_type, confidence, and default topic.
var categoryToSage = map[string]categoryMapping{
	"core":         {"fact", 0.95, "profile"},
	"daily":        {"observation", 0.65, "daily"},
	"conversation": {"observation", 0.40, "conversation"},
	"custom":       {"inference", 0.80, "project"},
}

// buildDomainTag constructs domain_tag as {scope}_{topic}.
// owner="alice", topic="profile" -> "u_alice_profile"
// owner="",      topic="knowledge" -> "shared_knowledge"
func buildDomainTag(owner, topic string) string {
	if owner == "" {
		return "shared_" + topic
	}
	return "u_" + owner + "_" + topic
}

// domainTagsForRecall returns all domain tags to query for a given owner.
// Includes all user-scoped domains + shared domains for comprehensive recall.
func domainTagsForRecall(owner string) []string {
	sharedDomains := []string{
		"shared_knowledge",
		"shared_rules",
		"shared_project",
		"shared_task",
	}
	if owner == "" {
		return sharedDomains
	}
	userDomains := []string{
		"u_" + owner + "_profile",
		"u_" + owner + "_preference",
		"u_" + owner + "_daily",
		"u_" + owner + "_project",
		"u_" + owner + "_conversation",
		"u_" + owner + "_task",
		"u_" + owner + "_learning",
	}
	return append(userDomains, sharedDomains...)
}

// sharedTopicDefaults maps category to topic for shared memories.
var sharedTopicDefaults = map[string]string{
	"core":         "knowledge",
	"daily":        "daily",
	"conversation": "conversation",
	"custom":       "project",
}

// encodeKey embeds the PicoClaw key into the content since Sage has no key field.
func encodeKey(key, content string) string {
	return "[key:" + key + "]\n" + content
}

// parseKey extracts the key and content from Sage's stored content.
func parseKey(raw string) (key, content string) {
	if !strings.HasPrefix(raw, "[key:") {
		return "", raw
	}
	end := strings.Index(raw, "]\n")
	if end < 0 {
		// Try without newline (single-line content)
		end = strings.Index(raw, "]")
		if end < 0 {
			return "", raw
		}
		key = raw[5:end]
		content = strings.TrimPrefix(raw[end+1:], "\n")
		return key, content
	}
	key = raw[5:end]
	content = raw[end+2:]
	return key, content
}

func (sb *SageBackend) Store(key, content, category, owner string) error {
	privKey, agentID, err := sb.identity.GetOrCreate(owner)
	if err != nil {
		return fmt.Errorf("sage store: %w", err)
	}

	mapping, ok := categoryToSage[category]
	if !ok {
		mapping = categoryToSage["core"]
	}

	// Deprecate existing entries with the same key before storing
	sb.deleteByKey(key, owner)

	// Collect buffered triples, tags, and topic
	sb.mu.Lock()
	var triples []KnowledgeTriple
	var tags []string
	var topic string
	if pm, exists := sb.pending[key]; exists {
		triples = pm.triples
		tags = pm.tags
		topic = pm.topic
	}
	delete(sb.pending, key)
	sb.mu.Unlock()

	// Resolve domain topic: LLM-provided > category default
	if topic == "" {
		if owner == "" {
			topic = sharedTopicDefaults[category]
		} else {
			topic = mapping.defaultTopic
		}
		if topic == "" {
			topic = "knowledge"
		}
	}

	domainTag := buildDomainTag(owner, topic)

	resp, err := sb.client.Submit(agentID, privKey, SubmitRequest{
		Content:    encodeKey(key, content),
		MemoryType: mapping.memoryType,
		Confidence: mapping.confidence,
		DomainTag:  domainTag,
		Triples:    triples,
	})
	if err != nil {
		return fmt.Errorf("sage store: %w", err)
	}

	// Apply tags: LLM-provided + auto-generated
	if resp.ID != "" {
		autoTags := []string{category, topic}
		if owner != "" {
			autoTags = append(autoTags, "owner:"+owner)
		} else {
			autoTags = append(autoTags, "shared")
		}
		allTags := append(autoTags, tags...)
		if err := sb.client.SetTags(agentID, privKey, resp.ID, allTags); err != nil {
			logger.WarnCF("sage", "Failed to set tags",
				map[string]interface{}{"memory_id": resp.ID, "error": err.Error()})
		}
	}

	return nil
}

func (sb *SageBackend) Search(query string, limit int, owner string) ([]memory.SearchResult, error) {
	return sb.searchWithFilter(query, "", limit, owner)
}

func (sb *SageBackend) SearchByCategory(query, category string, limit int, owner string) ([]memory.SearchResult, error) {
	return sb.searchWithFilter(query, category, limit, owner)
}

func (sb *SageBackend) searchWithFilter(query, category string, limit int, owner string) ([]memory.SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}

	items, err := sb.listAll(owner, limit*3) // Fetch extra for client-side filtering
	if err != nil {
		return nil, err
	}

	queryLower := strings.ToLower(query)
	queryTerms := strings.Fields(queryLower)

	var results []memory.SearchResult
	for _, item := range items {
		key, content := parseKey(item.Content)

		if category != "" {
			itemCat := sageTypeToCategory(item.MemoryType)
			if itemCat != category {
				continue
			}
		}

		// Client-side text matching
		if query != "" {
			textLower := strings.ToLower(key + " " + content)
			matched := false
			for _, term := range queryTerms {
				if strings.Contains(textLower, term) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		entry := sb.itemToEntry(item)
		results = append(results, memory.SearchResult{
			Entry:             entry,
			Rank:              0,
			DecayedConfidence: item.Confidence,
		})

		if len(results) >= limit {
			break
		}
	}
	return results, nil
}

func (sb *SageBackend) DeleteAccessible(key, owner string) bool {
	return sb.deleteByKey(key, owner)
}

func (sb *SageBackend) List(category string, limit int, owner string) ([]memory.MemoryEntry, error) {
	if limit <= 0 {
		limit = 20
	}

	items, err := sb.listAll(owner, limit*3)
	if err != nil {
		return nil, err
	}

	var entries []memory.MemoryEntry
	for _, item := range items {
		if category != "" {
			itemCat := sageTypeToCategory(item.MemoryType)
			if itemCat != category {
				continue
			}
		}

		entries = append(entries, sb.itemToEntry(item))
		if len(entries) >= limit {
			break
		}
	}
	return entries, nil
}

func (sb *SageBackend) ListRecent(categories []string, days, limit int, owner string) ([]memory.MemoryEntry, error) {
	if limit <= 0 {
		limit = 10
	}

	catSet := make(map[string]bool, len(categories))
	for _, c := range categories {
		catSet[c] = true
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -days)

	items, err := sb.listAll(owner, limit*5)
	if err != nil {
		return nil, err
	}

	var entries []memory.MemoryEntry
	for _, item := range items {
		cat := sageTypeToCategory(item.MemoryType)
		if !catSet[cat] {
			continue
		}

		entry := sb.itemToEntry(item)
		if entry.UpdatedAt.Before(cutoff) {
			continue
		}

		entries = append(entries, entry)
		if len(entries) >= limit {
			break
		}
	}
	return entries, nil
}

func (sb *SageBackend) Get(key string) *memory.MemoryEntry {
	// Search across all domains since Get has no owner parameter
	items, err := sb.listAllDomains(100)
	if err != nil {
		return nil
	}

	for _, item := range items {
		k, _ := parseKey(item.Content)
		if k == key {
			entry := sb.itemToEntry(item)
			return &entry
		}
	}
	return nil
}

// AllEntityNames returns empty for Sage (no graph API).
func (sb *SageBackend) AllEntityNames() ([]string, error) {
	return nil, nil
}

// WalkGraphForOwner returns empty for Sage (no graph API).
func (sb *SageBackend) WalkGraphForOwner(_ []string, _, _ int, _ string) ([]memory.GraphNode, error) {
	return nil, nil
}

// AddRelation buffers a knowledge triple. The triple will be included
// in the next Store call for this memoryKey (single Sage submission).
func (sb *SageBackend) AddRelation(source, relation, target, memoryKey string) error {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	pm, ok := sb.pending[memoryKey]
	if !ok {
		pm = &pendingMemory{}
		sb.pending[memoryKey] = pm
	}
	pm.triples = append(pm.triples, KnowledgeTriple{
		Subject:   source,
		Predicate: relation,
		Object:    target,
	})
	return nil
}

// RemoveRelationsByMemoryKey clears buffered triples for the given key,
// preserving any buffered tags.
func (sb *SageBackend) RemoveRelationsByMemoryKey(key string) error {
	sb.mu.Lock()
	if pm, ok := sb.pending[key]; ok {
		pm.triples = nil
	}
	sb.mu.Unlock()
	return nil
}

// SetTags buffers tags for the given key. Tags will be applied after
// the next Store call for this key.
func (sb *SageBackend) SetTags(key string, tags []string) error {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	pm, ok := sb.pending[key]
	if !ok {
		pm = &pendingMemory{}
		sb.pending[key] = pm
	}
	pm.tags = tags
	return nil
}

// SetDomain buffers a domain topic for the given key.
// The topic is used to build the domain_tag ({scope}_{topic}).
func (sb *SageBackend) SetDomain(key string, topic string) error {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	pm, ok := sb.pending[key]
	if !ok {
		pm = &pendingMemory{}
		sb.pending[key] = pm
	}
	pm.topic = topic
	return nil
}

// Close is a no-op for the HTTP-based Sage backend.
func (sb *SageBackend) Close() error {
	return nil
}

// listAll fetches memories for the given owner (their domain + shared domain).
func (sb *SageBackend) listAll(owner string, limit int) ([]MemoryItem, error) {
	privKey, agentID, err := sb.identity.GetOrCreate(owner)
	if err != nil {
		return nil, fmt.Errorf("sage list: %w", err)
	}

	tags := domainTagsForRecall(owner)
	resp, err := sb.client.ListMemories(agentID, privKey, tags, limit)
	if err != nil {
		return nil, fmt.Errorf("sage list: %w", err)
	}

	// Filter out deprecated memories
	var active []MemoryItem
	for _, m := range resp.Memories {
		if m.Status != "deprecated" {
			active = append(active, m)
		}
	}
	return active, nil
}

// listAllDomains fetches memories using the shared identity (for Get without owner).
func (sb *SageBackend) listAllDomains(limit int) ([]MemoryItem, error) {
	privKey, agentID, err := sb.identity.GetOrCreate("")
	if err != nil {
		return nil, fmt.Errorf("sage list all: %w", err)
	}

	resp, err := sb.client.ListMemories(agentID, privKey, nil, limit)
	if err != nil {
		return nil, fmt.Errorf("sage list all: %w", err)
	}
	return resp.Memories, nil
}

// deleteByKey finds and deprecates all memories with the given key.
func (sb *SageBackend) deleteByKey(key, owner string) bool {
	items, err := sb.listAll(owner, 100)
	if err != nil {
		return false
	}

	deleted := false
	for _, item := range items {
		k, _ := parseKey(item.Content)
		if k == key {
			privKey, agentID, err := sb.identity.GetOrCreate(owner)
			if err != nil {
				continue
			}
			if err := sb.client.DeprecateMemory(agentID, privKey, item.ID); err != nil {
				logger.WarnCF("sage", "Failed to deprecate memory",
					map[string]interface{}{"id": item.ID, "error": err.Error()})
				continue
			}
			deleted = true
		}
	}
	return deleted
}

func (sb *SageBackend) itemToEntry(item MemoryItem) memory.MemoryEntry {
	key, content := parseKey(item.Content)

	owner := ownerFromDomainTag(item.DomainTag)

	createdAt := parseTimeStr(item.CreatedAt)
	updatedAt := parseTimeStr(item.UpdatedAt)

	return memory.MemoryEntry{
		Key:        key,
		Content:    content,
		Category:   sageTypeToCategory(item.MemoryType),
		Owner:      owner,
		Confidence: item.Confidence,
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
	}
}

// ownerFromDomainTag extracts username from domain tag.
// "u_alice_profile" -> "alice", "shared_knowledge" -> ""
func ownerFromDomainTag(tag string) string {
	if !strings.HasPrefix(tag, "u_") {
		return ""
	}
	// Strip "u_" prefix, then take everything before the first "_" (topic separator)
	rest := tag[2:]
	if idx := strings.Index(rest, "_"); idx >= 0 {
		return rest[:idx]
	}
	return rest // fallback: "u_alice" with no topic
}

func sageTypeToCategory(memoryType string) string {
	switch memoryType {
	case "fact":
		return "core"
	case "observation":
		return "daily"
	case "inference":
		return "custom"
	default:
		return "core"
	}
}

func parseTimeStr(s string) time.Time {
	if s == "" {
		return time.Now().UTC()
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		return t
	}
	return time.Now().UTC()
}
