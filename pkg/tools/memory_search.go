package tools

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sipeed/picoclaw/pkg/memory"
)

type MemorySearchTool struct {
	db    memory.MemoryBackend
	owner string
	mu    sync.Mutex
}

func NewMemorySearchTool(db memory.MemoryBackend) *MemorySearchTool {
	return &MemorySearchTool{db: db}
}

func (t *MemorySearchTool) SetOwner(owner string) {
	t.mu.Lock()
	t.owner = owner
	t.mu.Unlock()
}

func (t *MemorySearchTool) Name() string {
	return "memory_search"
}

func (t *MemorySearchTool) Description() string {
	return "Search across all memory entries. Use this to recall past events, decisions, or information. Supports optional category filter. If query is empty, lists recent memory entries."
}

func (t *MemorySearchTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query to find in memory",
			},
			"category": map[string]interface{}{
				"type":        "string",
				"description": "Optional: filter by category (core, daily, conversation, custom)",
				"enum":        []string{"core", "daily", "conversation", "custom"},
			},
			"domain": map[string]interface{}{
				"type":        "string",
				"description": "Optional: filter by domain topic (e.g. profile, project, rules, knowledge, daily)",
			},
			"min_confidence": map[string]interface{}{
				"type":        "number",
				"description": "Optional: minimum confidence threshold 0.0-1.0",
			},
			"time_range": map[string]interface{}{
				"type":        "string",
				"description": "Optional: filter by recency",
				"enum":        []string{"today", "week", "month"},
			},
			"owner_scope": map[string]interface{}{
				"type":        "string",
				"description": "Optional: search scope - shared (group knowledge only), private (sender only), all (default)",
				"enum":        []string{"shared", "private", "all"},
			},
			"limit": map[string]interface{}{
				"type":        "number",
				"description": "Maximum number of results to return (default 10)",
			},
		},
		"required": []string{},
	}
}

func (t *MemorySearchTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	query, _ := args["query"].(string)

	limit := 10
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	category, _ := args["category"].(string)
	domain, _ := args["domain"].(string)
	timeRange, _ := args["time_range"].(string)
	ownerScope, _ := args["owner_scope"].(string)

	var minConfidence float64
	if mc, ok := args["min_confidence"].(float64); ok {
		minConfidence = mc
	}

	t.mu.Lock()
	owner := t.owner
	t.mu.Unlock()

	// Empty query: fall back to listing recent entries
	if strings.TrimSpace(query) == "" {
		entries, err := t.db.List(category, limit, owner)
		if err != nil {
			return fmt.Sprintf("Error listing memories: %v", err), nil
		}
		if len(entries) == 0 {
			return "No memories found.", nil
		}
		now := time.Now().UTC()
		var b strings.Builder
		for i, e := range entries {
			if i > 0 {
				b.WriteString("\n---\n")
			}
			ownerLabel := "shared"
			if e.Owner != "" {
				ownerLabel = "owner:" + e.Owner
			}
			conf := memory.ComputeConfidence(e.Confidence, e.CreatedAt, now, e.AccessCount, e.Category)
			b.WriteString(fmt.Sprintf("[%s] (%s) (%s) confidence:%.0f%% updated:%s\n%s",
				e.Key, e.Category, ownerLabel,
				conf*100,
				e.UpdatedAt.Format("2006-01-02"),
				e.Content,
			))
		}
		return b.String(), nil
	}

	results, err := t.db.SearchWithOptions(memory.SearchOptions{
		Query:         query,
		Category:      category,
		Domain:        domain,
		MinConfidence: minConfidence,
		TimeRange:     timeRange,
		OwnerScope:    ownerScope,
		Limit:         limit,
		Owner:         owner,
	})

	if err != nil {
		return fmt.Sprintf("Error searching memory: %v", err), nil
	}

	if len(results) == 0 {
		return "No matching results found.", nil
	}

	var b strings.Builder
	for i, r := range results {
		if i > 0 {
			b.WriteString("\n---\n")
		}
		ownerLabel := "shared"
		if r.Entry.Owner != "" {
			ownerLabel = "owner:" + r.Entry.Owner
		}
		b.WriteString(fmt.Sprintf("[%s] (%s) (%s) confidence:%.0f%% updated:%s\n%s",
			r.Entry.Key,
			r.Entry.Category,
			ownerLabel,
			r.DecayedConfidence*100,
			r.Entry.UpdatedAt.Format("2006-01-02"),
			r.Entry.Content,
		))
	}
	return b.String(), nil
}
