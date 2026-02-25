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
If the key already exists, the content is updated.

When storing facts involving entities (people, projects, places, concepts), include relations to build a knowledge graph for better context retrieval.
Example: key="team_alice", content="Alice joined PicoClaw team", relations=[{"source":"Alice", "relation":"works_on", "target":"PicoClaw"}]`
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
			"relations": map[string]interface{}{
				"type":        "array",
				"description": "Entity relationships extracted from this memory. Each item: {source, relation, target}",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"source":   map[string]interface{}{"type": "string", "description": "Source entity name"},
						"relation": map[string]interface{}{"type": "string", "description": "Relationship type (e.g. works_on, lives_in, knows)"},
						"target":   map[string]interface{}{"type": "string", "description": "Target entity name"},
					},
				},
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

	// Clear stale relations before upsert (handles key update case)
	if relations, ok := args["relations"].([]interface{}); ok && len(relations) > 0 {
		_ = t.db.RemoveRelationsByMemoryKey(key)
	}

	if err := t.db.Store(key, content, category); err != nil {
		return fmt.Sprintf("Error storing memory: %v", err), nil
	}

	// Process relations if provided
	relCount := 0
	if relations, ok := args["relations"].([]interface{}); ok {
		for _, r := range relations {
			rel, ok := r.(map[string]interface{})
			if !ok {
				continue
			}
			source, _ := rel["source"].(string)
			relation, _ := rel["relation"].(string)
			target, _ := rel["target"].(string)
			if source == "" || relation == "" || target == "" {
				continue
			}
			if err := t.db.AddRelation(source, relation, target, key); err != nil {
				continue
			}
			relCount++
		}
	}

	if relCount > 0 {
		return fmt.Sprintf("Memory stored: key=%q, category=%s, relations=%d", key, category, relCount), nil
	}
	return fmt.Sprintf("Memory stored: key=%q, category=%s", key, category), nil
}
