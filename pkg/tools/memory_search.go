package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/sipeed/picoclaw/pkg/memory"
)

type MemorySearchTool struct {
	db *memory.MemoryDB
}

func NewMemorySearchTool(db *memory.MemoryDB) *MemorySearchTool {
	return &MemorySearchTool{db: db}
}

func (t *MemorySearchTool) Name() string {
	return "memory_search"
}

func (t *MemorySearchTool) Description() string {
	return "Search across all memory entries using full-text search with BM25 ranking. Use this to recall past events, decisions, or information. Supports optional category filter. If query is empty, lists recent memory entries."
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

	// Empty query: fall back to listing recent entries
	if strings.TrimSpace(query) == "" {
		entries, err := t.db.List(category, limit)
		if err != nil {
			return fmt.Sprintf("Error listing memories: %v", err), nil
		}
		if len(entries) == 0 {
			return "No memories found.", nil
		}
		var b strings.Builder
		for i, e := range entries {
			if i > 0 {
				b.WriteString("\n---\n")
			}
			b.WriteString(fmt.Sprintf("[%s] (%s) updated:%s\n%s",
				e.Key, e.Category,
				e.UpdatedAt.Format("2006-01-02"),
				e.Content,
			))
		}
		return b.String(), nil
	}

	var results []memory.SearchResult
	var err error
	if category != "" {
		results, err = t.db.SearchByCategory(query, category, limit)
	} else {
		results, err = t.db.Search(query, limit)
	}

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
		b.WriteString(fmt.Sprintf("[%s] (%s) updated:%s\n%s",
			r.Entry.Key,
			r.Entry.Category,
			r.Entry.UpdatedAt.Format("2006-01-02"),
			r.Entry.Content,
		))
	}
	return b.String(), nil
}
