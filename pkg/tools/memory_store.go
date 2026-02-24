package tools

import (
	"context"
	"fmt"

	"github.com/sipeed/picoclaw/pkg/memory"
)

type MemoryStoreTool struct {
	db *memory.MemoryDB
}

func NewMemoryStoreTool(db *memory.MemoryDB) *MemoryStoreTool {
	return &MemoryStoreTool{db: db}
}

func (t *MemoryStoreTool) Name() string {
	return "memory_store"
}

func (t *MemoryStoreTool) Description() string {
	return `Store a memory entry with a unique key. Categories control retention:
- "core": permanent, never auto-deleted (default)
- "daily": auto-deleted after 30 days
- "conversation": auto-deleted after 7 days
- "custom": auto-deleted after 90 days
If the key already exists, the content is updated.`
}

func (t *MemoryStoreTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"key": map[string]interface{}{
				"type":        "string",
				"description": "Unique key for this memory (e.g. 'user_birthday', 'project_deadline')",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "The content to remember",
			},
			"category": map[string]interface{}{
				"type":        "string",
				"description": "Memory category: core (permanent), daily (30d), conversation (7d), custom (90d). Default: core",
				"enum":        []string{"core", "daily", "conversation", "custom"},
			},
		},
		"required": []string{"key", "content"},
	}
}

func (t *MemoryStoreTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	key, _ := args["key"].(string)
	if key == "" {
		return "Error: 'key' parameter is required.", nil
	}

	content, _ := args["content"].(string)
	if content == "" {
		return "Error: 'content' parameter is required.", nil
	}

	category := "core"
	if c, ok := args["category"].(string); ok && c != "" {
		category = c
	}

	if err := t.db.Store(key, content, category); err != nil {
		return fmt.Sprintf("Error storing memory: %v", err), nil
	}

	return fmt.Sprintf("Memory stored: key=%q, category=%s", key, category), nil
}
