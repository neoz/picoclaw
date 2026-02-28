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
	return `Delete a memory entry by its key. By default deletes your own entry for that key. Set shared=true to delete the shared entry instead. You can only delete your own or shared memories.`
}

func (t *MemoryForgetTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"key": map[string]interface{}{
				"type":        "string",
				"description": "The key of the memory entry to delete",
			},
			"shared": map[string]interface{}{
				"type":        "boolean",
				"description": "Set to true to delete the shared entry (owner='') instead of the current user's entry. Default: false.",
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

	// Determine which owner's entry to delete
	targetOwner := owner
	if shared, ok := args["shared"].(bool); ok && shared {
		targetOwner = ""
	}

	if t.db.DeleteByOwner(key, targetOwner) {
		if targetOwner == "" {
			return fmt.Sprintf("Shared memory deleted: key=%q", key), nil
		}
		return fmt.Sprintf("Memory deleted: key=%q (owner=%s)", key, targetOwner), nil
	}
	return fmt.Sprintf("Memory not found: key=%q", key), nil
}
