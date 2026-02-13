package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sipeed/picoclaw/pkg/logger"
)

const stmRetentionDays = 7

type STMEntry struct {
	Content   string    `json:"content"`
	SenderID  string    `json:"sender_id"`
	Timestamp time.Time `json:"timestamp"`
}

type STMStore struct {
	dir     string
	entries map[string][]STMEntry
	mu      sync.RWMutex
}

func NewSTMStore(dir string) *STMStore {
	os.MkdirAll(dir, 0755)
	return &STMStore{
		dir:     dir,
		entries: make(map[string][]STMEntry),
	}
}

func sanitizeSessionKey(key string) string {
	return strings.ReplaceAll(key, ":", "_") + ".json"
}

func (s *STMStore) load(sessionKey string) []STMEntry {
	if entries, ok := s.entries[sessionKey]; ok {
		return entries
	}
	filePath := filepath.Join(s.dir, sanitizeSessionKey(sessionKey))
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}
	var entries []STMEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil
	}
	// Prune old entries
	cutoff := time.Now().AddDate(0, 0, -stmRetentionDays)
	pruned := make([]STMEntry, 0, len(entries))
	for _, e := range entries {
		if e.Timestamp.After(cutoff) {
			pruned = append(pruned, e)
		}
	}
	s.entries[sessionKey] = pruned
	return pruned
}

func (s *STMStore) persist(sessionKey string) {
	entries := s.entries[sessionKey]
	data, err := json.Marshal(entries)
	if err != nil {
		logger.ErrorCF("stm", "Failed to marshal STM entries", map[string]interface{}{
			"error":       err.Error(),
			"session_key": sessionKey,
		})
		return
	}
	filePath := filepath.Join(s.dir, sanitizeSessionKey(sessionKey))
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		logger.ErrorCF("stm", "Failed to persist STM entries", map[string]interface{}{
			"error":       err.Error(),
			"session_key": sessionKey,
		})
	}
}

func (s *STMStore) Save(sessionKey, content, senderID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load(sessionKey)
	s.entries[sessionKey] = append(s.entries[sessionKey], STMEntry{
		Content:   content,
		SenderID:  senderID,
		Timestamp: time.Now(),
	})
	s.persist(sessionKey)
}

func (s *STMStore) Recent(sessionKey string, limit, days int) []STMEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries := s.load(sessionKey)
	if days <= 0 || days > stmRetentionDays {
		days = stmRetentionDays
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	var recent []STMEntry
	for _, e := range entries {
		if e.Timestamp.After(cutoff) {
			recent = append(recent, e)
		}
	}
	if limit > 0 && len(recent) > limit {
		recent = recent[len(recent)-limit:]
	}
	return recent
}

func (s *STMStore) Search(sessionKey, query string, days int) []STMEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries := s.load(sessionKey)
	if days <= 0 || days > stmRetentionDays {
		days = stmRetentionDays
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	var docs []STMEntry
	for _, e := range entries {
		if e.Timestamp.After(cutoff) {
			docs = append(docs, e)
		}
	}
	if len(docs) == 0 {
		return nil
	}
	return bm25Search(docs, query)
}

// BM25 search implementation

func tokenize(text string) []string {
	return strings.Fields(strings.ToLower(text))
}

func bm25Search(docs []STMEntry, query string) []STMEntry {
	queryTerms := tokenize(query)
	if len(queryTerms) == 0 {
		return nil
	}

	n := float64(len(docs))
	k1 := 1.2
	b := 0.75

	// Compute average document length
	var totalLen float64
	docTokens := make([][]string, len(docs))
	for i, doc := range docs {
		tokens := tokenize(doc.Content)
		docTokens[i] = tokens
		totalLen += float64(len(tokens))
	}
	avgDL := totalLen / n

	// Compute IDF for each query term
	idf := make(map[string]float64)
	for _, term := range queryTerms {
		df := 0
		for _, tokens := range docTokens {
			for _, t := range tokens {
				if t == term {
					df++
					break
				}
			}
		}
		idf[term] = math.Log((n-float64(df)+0.5)/(float64(df)+0.5) + 1)
	}

	// Score each document
	type scored struct {
		entry STMEntry
		score float64
	}
	results := make([]scored, 0, len(docs))
	for i, doc := range docs {
		tokens := docTokens[i]
		dl := float64(len(tokens))
		// Compute TF for query terms
		tf := make(map[string]int)
		for _, t := range tokens {
			tf[t]++
		}
		var score float64
		for _, term := range queryTerms {
			f := float64(tf[term])
			score += idf[term] * (f * (k1 + 1)) / (f + k1*(1-b+b*dl/avgDL))
		}
		if score > 0 {
			results = append(results, scored{entry: doc, score: score})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	// Return top 10
	limit := 10
	if len(results) < limit {
		limit = len(results)
	}
	out := make([]STMEntry, limit)
	for i := 0; i < limit; i++ {
		out[i] = results[i].entry
	}
	return out
}

// STMTool implements ContextualTool

type STMTool struct {
	store   *STMStore
	channel string
	chatID  string
}

func NewSTMTool(store *STMStore) *STMTool {
	return &STMTool{store: store}
}

func (t *STMTool) Name() string {
	return "short_term_memory"
}

func (t *STMTool) Description() string {
	return "Access recent messages from the current session. Actions: 'recent' returns last N messages (default 10, max 50), 'search' performs BM25-ranked search over recent messages. Use 'days' to narrow the time window (default 7, set 1 for last day)."
}

func (t *STMTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"recent", "search"},
				"description": "Action to perform: 'recent' for last N messages, 'search' for BM25 search",
			},
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query (required for 'search' action)",
			},
			"limit": map[string]interface{}{
				"type":        "number",
				"description": "Max messages to return for 'recent' action (default 10, max 50)",
			},
			"days": map[string]interface{}{
				"type":        "number",
				"description": "Time window in days (default 7, set 1 for last day only)",
			},
		},
		"required": []string{"action"},
	}
}

func (t *STMTool) SetContext(channel, chatID string) {
	t.channel = channel
	t.chatID = chatID
}

func (t *STMTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	action, _ := args["action"].(string)
	sessionKey := fmt.Sprintf("%s:%s", t.channel, t.chatID)

	days := stmRetentionDays
	if d, ok := args["days"].(float64); ok && d > 0 {
		days = int(d)
	}

	switch action {
	case "recent":
		limit := 10
		if l, ok := args["limit"].(float64); ok && l > 0 {
			limit = int(l)
		}
		if limit > 50 {
			limit = 50
		}
		entries := t.store.Recent(sessionKey, limit, days)
		if len(entries) == 0 {
			return "No recent messages found.", nil
		}
		return formatSTMEntries(entries), nil

	case "search":
		query, _ := args["query"].(string)
		if query == "" {
			return "Error: 'query' parameter is required for search action.", nil
		}
		entries := t.store.Search(sessionKey, query, days)
		if len(entries) == 0 {
			return "No matching messages found.", nil
		}
		return formatSTMEntries(entries), nil

	default:
		return "Error: action must be 'recent' or 'search'.", nil
	}
}

func formatSTMEntries(entries []STMEntry) string {
	var b strings.Builder
	for i, e := range entries {
		if i > 0 {
			b.WriteString("\n---\n")
		}
		b.WriteString(fmt.Sprintf("[%s] %s: %s", e.Timestamp.Format("2006-01-02 15:04"), e.SenderID, e.Content))
	}
	return b.String()
}
