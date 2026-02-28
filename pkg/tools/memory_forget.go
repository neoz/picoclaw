package tools

import (
	"context"
	"fmt"
	"sync"

	"github.com/sipeed/picoclaw/pkg/memory"
)

type MemoryForgetTool struct {
	db    *memory.MemoryDB
	owner string
	mu    sync.Mutex
}

func NewMemoryForgetTool(db *memory.MemoryDB) *MemoryForgetTool {
	return &MemoryForgetTool{db: db}
}

func (t *MemoryForgetTool) SetOwner(owner string) {
	t.mu.Lock()
	t.owner = owner
	t.mu.Unlock()
}

func (t *MemoryForgetTool) Name() string {
	return "memory_forget"
}

func (t *MemoryForgetTool) Description() string {
	return "Delete a memory entry by its key. Removes your own and shared entries for that key. Cannot delete other users' private entries."
}

func (t *MemoryForgetTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"key": map[string]interface{}{
				"type":        "string",
				"description": "The key of the memory entry to delete",
			},
		},
		"required": []string{"key"},
	}
}

func (t *MemoryForgetTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	key, _ := args["key"].(string)
	if key == "" {
		return "Error: 'key' parameter is required.", nil
	}

	t.mu.Lock()
	owner := t.owner
	t.mu.Unlock()

	if t.db.DeleteAccessible(key, owner) {
		return fmt.Sprintf("Memory deleted: key=%q", key), nil
	}
	return fmt.Sprintf("Memory not found: key=%q", key), nil
}
