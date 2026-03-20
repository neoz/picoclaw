package tools

import (
	"context"
	"fmt"
	"sync"

	"github.com/sipeed/picoclaw/pkg/memory"
)

type MemoryStoreTool struct {
	db    memory.MemoryBackend
	owner string
	mu    sync.Mutex
}

func NewMemoryStoreTool(db memory.MemoryBackend) *MemoryStoreTool {
	return &MemoryStoreTool{db: db}
}

func (t *MemoryStoreTool) SetOwner(owner string) {
	t.mu.Lock()
	t.owner = owner
	t.mu.Unlock()
}

func (t *MemoryStoreTool) Name() string {
	return "memory_store"
}

func (t *MemoryStoreTool) Description() string {
	return `Store a memory entry. Always write content in English, concise summary form regardless of conversation language. Categories: core (permanent, default), daily (30d), conversation (7d), custom (90d). Existing key = update. Set shared=true for all-user visibility. Include relations for knowledge graph and tags for classification.`
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
			"shared": map[string]interface{}{
				"type":        "boolean",
				"description": "Set to true to store as shared memory (visible to all users). Default: false (owned by current user).",
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
			"tags": map[string]interface{}{
				"type":        "array",
				"description": "Tags for classifying this memory (e.g. [\"project\", \"deadline\", \"IoT\", \"security\"])",
				"items":       map[string]interface{}{"type": "string"},
			},
			"topic": map[string]interface{}{
				"type":        "string",
				"description": "Domain topic for memory organization. Options: profile (user identity), preference (user preferences), daily (daily notes), project (ongoing work), learning (derived knowledge), task (action items), conversation (chat context), knowledge (shared facts), rules (shared agreements)",
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

	t.mu.Lock()
	owner := t.owner
	t.mu.Unlock()

	// Allow agent to store shared memory (owner="") when shared=true
	if shared, ok := args["shared"].(bool); ok && shared {
		owner = ""
	}

	// Buffer domain topic BEFORE store
	if topic, ok := args["topic"].(string); ok && topic != "" {
		_ = t.db.SetDomain(key, topic)
	}

	// Buffer tags BEFORE store so backends (e.g. Sage) can include them
	if tagList, ok := args["tags"].([]interface{}); ok && len(tagList) > 0 {
		var tags []string
		for _, t := range tagList {
			if s, ok := t.(string); ok && s != "" {
				tags = append(tags, s)
			}
		}
		if len(tags) > 0 {
			_ = t.db.SetTags(key, tags)
		}
	}

	// Buffer relations BEFORE store so backends (e.g. Sage) can include
	// them in a single submission rather than re-submitting per relation.
	relCount := 0
	if relations, ok := args["relations"].([]interface{}); ok && len(relations) > 0 {
		_ = t.db.RemoveRelationsByMemoryKey(key)
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

	if err := t.db.Store(key, content, category, owner); err != nil {
		return fmt.Sprintf("Error storing memory: %v", err), nil
	}

	if relCount > 0 {
		return fmt.Sprintf("Memory stored: key=%q, category=%s, relations=%d", key, category, relCount), nil
	}
	return fmt.Sprintf("Memory stored: key=%q, category=%s", key, category), nil
}
