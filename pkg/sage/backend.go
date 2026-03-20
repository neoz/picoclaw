package sage

import (
	"fmt"
	"sort"
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

// cachedRelation stores an entity-to-entity relation in the local graph cache.
type cachedRelation struct {
	Source    string
	Relation string
	Target   string
	MemKey   string
}

// listCache holds a cached listAll result with expiry.
type listCache struct {
	items   []MemoryItem
	owner   string
	fetched time.Time
}

const listCacheTTL = 30 * time.Second

// SageBackend implements memory.MemoryBackend using Sage as the remote store.
type SageBackend struct {
	client   *Client
	identity *IdentityManager
	mu       sync.Mutex
	pending  map[string]*pendingMemory // key -> last submitted memory

	// In-memory entity graph cache populated by AddRelation calls.
	// Enables AllEntityNames and WalkGraphForOwner without a Sage graph query API.
	entities  map[string]struct{}  // entity name set
	relations []cachedRelation     // all cached relations
	keyOwner  map[string]string    // memory key -> owner (for owner-scoped BFS)
	keyIDs    map[string]string    // memory key -> Sage memory ID (for linking)

	// Short-lived cache for listAll to avoid redundant fetches within a context build.
	cached *listCache
}

// NewSageBackend creates a new Sage-backed memory backend.
func NewSageBackend(client *Client, identity *IdentityManager) *SageBackend {
	return &SageBackend{
		client:   client,
		identity: identity,
		pending:  make(map[string]*pendingMemory),
		entities: make(map[string]struct{}),
		keyOwner: make(map[string]string),
		keyIDs:   make(map[string]string),
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

	sb.InvalidateCache()

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

		// Track key->ID and key->owner for graph linking
		sb.mu.Lock()
		sb.keyIDs[key] = resp.ID
		sb.keyOwner[key] = owner
		sb.mu.Unlock()

		// Link to other memories that share entity relations
		sb.linkRelatedMemories(key, resp.ID, owner)
	}

	return nil
}

func (sb *SageBackend) Search(query string, limit int, owner string) ([]memory.SearchResult, error) {
	return sb.SearchWithOptions(memory.SearchOptions{
		Query: query, Limit: limit, Owner: owner,
	})
}

func (sb *SageBackend) SearchByCategory(query, category string, limit int, owner string) ([]memory.SearchResult, error) {
	return sb.SearchWithOptions(memory.SearchOptions{
		Query: query, Category: category, Limit: limit, Owner: owner,
	})
}

func (sb *SageBackend) SearchWithOptions(opts memory.SearchOptions) ([]memory.SearchResult, error) {
	query := opts.Query
	category := opts.Category
	limit := opts.Limit
	owner := opts.Owner

	// Resolve owner based on OwnerScope
	switch opts.OwnerScope {
	case "shared":
		owner = "" // only query shared agent
	case "private":
		// keep owner as-is, but skip shared agent query in semanticSearch
	}

	return sb.searchWithFilter(opts, query, category, limit, owner)
}

func (sb *SageBackend) searchWithFilter(opts memory.SearchOptions, query, category string, limit int, owner string) ([]memory.SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}

	// Try semantic search via Sage embed + query
	if query != "" {
		results, err := sb.semanticSearch(opts, query, category, limit, owner)
		if err == nil && len(results) > 0 {
			return results, nil
		}
		// Fall through to text-match fallback on error (e.g. Ollama unavailable)
		if err != nil {
			logger.WarnCF("sage", "Semantic search unavailable, falling back to text match",
				map[string]interface{}{"error": err.Error()})
		}
	}

	return sb.textSearch(opts, query, category, limit, owner)
}

// semanticSearch uses Sage's embed + query endpoints for server-side vector similarity.
func (sb *SageBackend) semanticSearch(opts memory.SearchOptions, query, category string, limit int, owner string) ([]memory.SearchResult, error) {
	// Get credentials for the query agent
	privKey, agentID, err := sb.identity.GetOrCreate(owner)
	if err != nil {
		return nil, fmt.Errorf("sage identity: %w", err)
	}

	// Generate embedding for the query text
	embedding, err := sb.client.Embed(agentID, privKey, query)
	if err != nil {
		return nil, err
	}

	// Build query request with extended options
	qr := QueryRequest{
		Embedding:     embedding,
		TopK:          limit * 2,
		MinConfidence: opts.MinConfidence,
	}
	// Map domain topic to Sage domain_tag
	if opts.Domain != "" {
		if owner != "" {
			qr.DomainTag = buildDomainTag(owner, opts.Domain)
		} else {
			qr.DomainTag = buildDomainTag("", opts.Domain)
		}
	}

	// Query both shared and user agents' memories
	var allResults []QueryResult

	// Query as the current agent (sees own + accessible memories)
	resp, err := sb.client.QueryMemories(agentID, privKey, qr)
	if err != nil {
		return nil, err
	}
	allResults = append(allResults, resp.Results...)

	// Also query as shared agent if owner is non-empty and not private-only scope
	if owner != "" && opts.OwnerScope != "private" {
		sharedKey, sharedAgent, err := sb.identity.GetOrCreate("")
		if err == nil && sharedAgent != agentID {
			sharedQR := qr
			// Adjust domain_tag for shared agent
			if opts.Domain != "" {
				sharedQR.DomainTag = buildDomainTag("", opts.Domain)
			}
			sharedResp, err := sb.client.QueryMemories(sharedAgent, sharedKey, sharedQR)
			if err == nil {
				allResults = append(allResults, sharedResp.Results...)
			}
		}
	}

	// Compute time cutoff for time_range filter
	var timeCutoff time.Time
	if opts.TimeRange != "" {
		now := time.Now().UTC()
		switch opts.TimeRange {
		case "today":
			timeCutoff = now.Truncate(24 * time.Hour)
		case "week":
			timeCutoff = now.AddDate(0, 0, -7)
		case "month":
			timeCutoff = now.AddDate(0, -1, 0)
		}
	}

	// Deduplicate and convert to SearchResult
	seen := make(map[string]bool)
	var results []memory.SearchResult
	for i, r := range allResults {
		if seen[r.MemoryID] || r.Status == "deprecated" {
			continue
		}
		seen[r.MemoryID] = true

		if category != "" {
			itemCat := sageTypeToCategory(r.MemoryType)
			if itemCat != category {
				continue
			}
		}

		// Owner scope filter
		entryOwner := ownerFromDomainTag(r.DomainTag)
		switch opts.OwnerScope {
		case "shared":
			if entryOwner != "" {
				continue
			}
		case "private":
			if entryOwner == "" || entryOwner != opts.Owner {
				continue
			}
		}

		// Time range filter
		if !timeCutoff.IsZero() {
			updatedAt := parseTimeStr(r.UpdatedAt)
			if updatedAt.Before(timeCutoff) {
				continue
			}
		}

		key, content := parseKey(r.Content)

		entry := memory.MemoryEntry{
			Key:        key,
			Content:    content,
			Category:   sageTypeToCategory(r.MemoryType),
			Owner:      entryOwner,
			Confidence: r.ConfidenceScore,
			CreatedAt:  parseTimeStr(r.CreatedAt),
			UpdatedAt:  parseTimeStr(r.UpdatedAt),
		}
		results = append(results, memory.SearchResult{
			Entry:             entry,
			Rank:              float64(len(allResults) - i), // higher rank for earlier results
			DecayedConfidence: r.ConfidenceScore,
		})

		if len(results) >= limit {
			break
		}
	}
	return results, nil
}

// textSearch is the fallback when semantic search is unavailable.
func (sb *SageBackend) textSearch(opts memory.SearchOptions, query, category string, limit int, owner string) ([]memory.SearchResult, error) {
	items, err := sb.listAll(owner, limit*3)
	if err != nil {
		return nil, err
	}

	queryLower := strings.ToLower(query)
	queryTerms := strings.Fields(queryLower)

	// Compute time cutoff
	var timeCutoff time.Time
	if opts.TimeRange != "" {
		now := time.Now().UTC()
		switch opts.TimeRange {
		case "today":
			timeCutoff = now.Truncate(24 * time.Hour)
		case "week":
			timeCutoff = now.AddDate(0, 0, -7)
		case "month":
			timeCutoff = now.AddDate(0, -1, 0)
		}
	}

	var results []memory.SearchResult
	for _, item := range items {
		key, content := parseKey(item.Content)

		if category != "" {
			itemCat := sageTypeToCategory(item.MemoryType)
			if itemCat != category {
				continue
			}
		}

		// Owner scope filter
		itemOwner := ownerFromDomainTag(item.DomainTag)
		switch opts.OwnerScope {
		case "shared":
			if itemOwner != "" {
				continue
			}
		case "private":
			if itemOwner == "" || itemOwner != opts.Owner {
				continue
			}
		}

		// Domain filter
		if opts.Domain != "" {
			expectedTag := buildDomainTag(itemOwner, opts.Domain)
			if item.DomainTag != expectedTag {
				continue
			}
		}

		// Min confidence filter
		if opts.MinConfidence > 0 && item.Confidence < opts.MinConfidence {
			continue
		}

		// Time range filter
		if !timeCutoff.IsZero() {
			updatedAt := parseTimeStr(item.UpdatedAt)
			if updatedAt.Before(timeCutoff) {
				continue
			}
		}

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
	items, err := sb.listAll("", 100)
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

// AllEntityNames returns all entity names from the local graph cache.
func (sb *SageBackend) AllEntityNames() ([]string, error) {
	sb.mu.Lock()
	names := make([]string, 0, len(sb.entities))
	for name := range sb.entities {
		names = append(names, name)
	}
	sb.mu.Unlock()
	sort.Strings(names)
	return names, nil
}

// WalkGraphForOwner performs BFS over the local entity graph cache,
// scoped to relations whose memory key is accessible by the given owner.
func (sb *SageBackend) WalkGraphForOwner(entityNames []string, maxHops, maxNodes int, owner string) ([]memory.GraphNode, error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if len(sb.entities) == 0 || len(sb.relations) == 0 {
		return nil, nil
	}

	// Build accessible key set for owner filtering
	accessible := make(map[string]bool)
	for key, o := range sb.keyOwner {
		if o == "" || o == owner {
			accessible[key] = true
		}
	}

	// Find seed entities
	seedSet := make(map[string]bool, len(entityNames))
	for _, name := range entityNames {
		nameLower := strings.ToLower(name)
		for ent := range sb.entities {
			if strings.ToLower(ent) == nameLower {
				seedSet[ent] = true
			}
		}
	}
	if len(seedSet) == 0 {
		return nil, nil
	}

	// BFS
	type queueItem struct {
		name  string
		depth int
	}

	visited := make(map[string]*memory.GraphNode)
	var queue []queueItem
	entityID := int64(1) // synthetic IDs for the memory.GraphNode struct

	for name := range seedSet {
		node := &memory.GraphNode{
			Entity: memory.Entity{ID: entityID, Name: name, Type: "thing"},
			Depth:  0,
		}
		visited[name] = node
		queue = append(queue, queueItem{name, 0})
		entityID++
	}

	for len(queue) > 0 && len(visited) < maxNodes {
		cur := queue[0]
		queue = queue[1:]

		if cur.depth >= maxHops {
			continue
		}

		// Find relations involving this entity
		for _, rel := range sb.relations {
			// Owner-scoped: skip relations for inaccessible keys
			if rel.MemKey != "" && !accessible[rel.MemKey] {
				continue
			}

			var neighbor string
			if strings.EqualFold(rel.Source, cur.name) {
				neighbor = rel.Target
			} else if strings.EqualFold(rel.Target, cur.name) {
				neighbor = rel.Source
			} else {
				continue
			}

			// Add relation to current node
			curNode := visited[cur.name]
			curNode.Relations = append(curNode.Relations, memory.Relation{
				Relation:  rel.Relation,
				MemoryKey: rel.MemKey,
			})

			if _, seen := visited[neighbor]; seen {
				continue
			}
			if len(visited) >= maxNodes {
				break
			}

			nextDepth := cur.depth + 1
			node := &memory.GraphNode{
				Entity: memory.Entity{ID: entityID, Name: neighbor, Type: "thing"},
				Depth:  nextDepth,
			}
			visited[neighbor] = node
			queue = append(queue, queueItem{neighbor, nextDepth})
			entityID++
		}
	}

	// Collect results sorted by depth then name
	result := make([]memory.GraphNode, 0, len(visited))
	for _, node := range visited {
		result = append(result, *node)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Depth != result[j].Depth {
			return result[i].Depth < result[j].Depth
		}
		return result[i].Entity.Name < result[j].Entity.Name
	})
	return result, nil
}

// linkRelatedMemories creates Sage links between the newly stored memory
// and any previously stored memories that share entity relations.
func (sb *SageBackend) linkRelatedMemories(key, memoryID, owner string) {
	sb.mu.Lock()
	// Collect memory keys that share entities with this key's relations
	relatedKeys := make(map[string]struct{})
	myEntities := make(map[string]struct{})
	for _, rel := range sb.relations {
		if rel.MemKey == key {
			myEntities[rel.Source] = struct{}{}
			myEntities[rel.Target] = struct{}{}
		}
	}
	for _, rel := range sb.relations {
		if rel.MemKey == key || rel.MemKey == "" {
			continue
		}
		if _, ok := myEntities[rel.Source]; ok {
			relatedKeys[rel.MemKey] = struct{}{}
			continue
		}
		if _, ok := myEntities[rel.Target]; ok {
			relatedKeys[rel.MemKey] = struct{}{}
		}
	}
	// Resolve memory IDs for related keys
	var links []string
	for rk := range relatedKeys {
		if id, ok := sb.keyIDs[rk]; ok {
			links = append(links, id)
		}
	}
	sb.mu.Unlock()

	if len(links) == 0 {
		return
	}

	privKey, agentID, err := sb.identity.GetOrCreate(owner)
	if err != nil {
		return
	}
	for _, targetID := range links {
		if err := sb.client.LinkMemories(agentID, privKey, memoryID, targetID, "related"); err != nil {
			logger.WarnCF("sage", "Failed to link memories",
				map[string]interface{}{"source": memoryID, "target": targetID, "error": err.Error()})
		}
	}
}

// AddRelation buffers a knowledge triple. The triple will be included
// in the next Store call for this memoryKey (single Sage submission).
// Also caches the entity names and relation for local graph traversal.
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

	// Cache entities and relation for local graph traversal
	sb.entities[source] = struct{}{}
	sb.entities[target] = struct{}{}
	sb.relations = append(sb.relations, cachedRelation{
		Source:   source,
		Relation: relation,
		Target:   target,
		MemKey:   memoryKey,
	})
	return nil
}

// RemoveRelationsByMemoryKey clears buffered triples for the given key,
// preserving any buffered tags. Also removes cached relations for this key.
func (sb *SageBackend) RemoveRelationsByMemoryKey(key string) error {
	sb.mu.Lock()
	if pm, ok := sb.pending[key]; ok {
		pm.triples = nil
	}
	// Remove cached relations for this key
	filtered := sb.relations[:0]
	for _, r := range sb.relations {
		if r.MemKey != key {
			filtered = append(filtered, r)
		}
	}
	sb.relations = filtered
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

// listAll fetches memories for the given owner, with short-lived caching
// to avoid redundant fetches when multiple callers (List, ListRecent, etc.)
// run within the same context build cycle.
// When owner is non-empty, queries both the user's agent and the shared agent
// so that shared memories (stored with owner="") are always visible.
// No domain_tag filtering is applied to avoid missing memories stored under
// LLM-assigned or unexpected domain tags.
func (sb *SageBackend) listAll(owner string, limit int) ([]MemoryItem, error) {
	sb.mu.Lock()
	if sb.cached != nil && sb.cached.owner == owner && time.Since(sb.cached.fetched) < listCacheTTL {
		items := sb.cached.items
		sb.mu.Unlock()
		if limit > 0 && len(items) > limit {
			return items[:limit], nil
		}
		return items, nil
	}
	sb.mu.Unlock()

	items, err := sb.fetchAll(owner, limit)
	if err != nil {
		return nil, err
	}

	sb.mu.Lock()
	sb.cached = &listCache{items: items, owner: owner, fetched: time.Now()}
	sb.mu.Unlock()

	return items, nil
}

// InvalidateCache clears the listAll cache, e.g. after Store or Delete.
func (sb *SageBackend) InvalidateCache() {
	sb.mu.Lock()
	sb.cached = nil
	sb.mu.Unlock()
}

// fetchAll performs the actual API calls to retrieve memories.
func (sb *SageBackend) fetchAll(owner string, limit int) ([]MemoryItem, error) {
	// Always fetch from the shared agent (owner="")
	sharedKey, sharedAgent, err := sb.identity.GetOrCreate("")
	if err != nil {
		return nil, fmt.Errorf("sage list shared: %w", err)
	}

	resp, err := sb.client.ListMemories(sharedAgent, sharedKey, nil, limit)
	if err != nil {
		return nil, fmt.Errorf("sage list shared: %w", err)
	}

	seen := make(map[string]struct{}, len(resp.Memories))
	var active []MemoryItem
	for _, m := range resp.Memories {
		if m.Status != "deprecated" {
			active = append(active, m)
			seen[m.ID] = struct{}{}
		}
	}

	// If owner is non-empty, also fetch from the user's own agent
	if owner != "" {
		userKey, userAgent, err := sb.identity.GetOrCreate(owner)
		if err != nil {
			return active, nil // return shared results on user-agent error
		}
		if userAgent != sharedAgent {
			userResp, err := sb.client.ListMemories(userAgent, userKey, nil, limit)
			if err == nil {
				for _, m := range userResp.Memories {
					if m.Status != "deprecated" {
						if _, dup := seen[m.ID]; !dup {
							active = append(active, m)
						}
					}
				}
			}
		}
	}

	return active, nil
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
	if deleted {
		sb.InvalidateCache()
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
