package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type MemorySearchTool struct {
	workspace string
}

func NewMemorySearchTool(workspace string) *MemorySearchTool {
	return &MemorySearchTool{workspace: workspace}
}

func (t *MemorySearchTool) Name() string {
	return "memory_search"
}

func (t *MemorySearchTool) Description() string {
	return "Search across all long-term memory files (MEMORY.md and daily notes) using BM25 ranking. Use this to recall past events, decisions, or information from any date."
}

func (t *MemorySearchTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query to find in memory files",
			},
			"limit": map[string]interface{}{
				"type":        "number",
				"description": "Maximum number of results to return (default 10)",
			},
		},
		"required": []string{"query"},
	}
}

type memoryParagraph struct {
	Source  string
	Content string
}

func (t *MemorySearchTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return "Error: 'query' parameter is required.", nil
	}

	limit := 10
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	memoryDir := filepath.Join(t.workspace, "memory")
	paragraphs := collectMemoryParagraphs(memoryDir)
	if len(paragraphs) == 0 {
		return "No memory files found.", nil
	}

	docs := make([]string, len(paragraphs))
	for i, p := range paragraphs {
		docs[i] = p.Content
	}
	indices := bm25Rank(docs, query, limit)
	if len(indices) == 0 {
		return "No matching results found.", nil
	}

	var b strings.Builder
	for i, idx := range indices {
		if i > 0 {
			b.WriteString("\n---\n")
		}
		b.WriteString(fmt.Sprintf("[%s]\n%s", paragraphs[idx].Source, paragraphs[idx].Content))
	}
	return b.String(), nil
}

func collectMemoryParagraphs(memoryDir string) []memoryParagraph {
	var paragraphs []memoryParagraph

	filepath.Walk(memoryDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		relPath, _ := filepath.Rel(memoryDir, path)
		content := string(data)

		// Split into paragraphs by double newline
		parts := strings.Split(content, "\n\n")
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed == "" {
				continue
			}
			paragraphs = append(paragraphs, memoryParagraph{
				Source:  relPath,
				Content: trimmed,
			})
		}

		return nil
	})

	return paragraphs
}

