package tools

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
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

	results := bm25SearchMemory(paragraphs, query, limit)
	if len(results) == 0 {
		return "No matching results found.", nil
	}

	var b strings.Builder
	for i, r := range results {
		if i > 0 {
			b.WriteString("\n---\n")
		}
		b.WriteString(fmt.Sprintf("[%s]\n%s", r.Source, r.Content))
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

func bm25SearchMemory(docs []memoryParagraph, query string, limit int) []memoryParagraph {
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
		doc   memoryParagraph
		score float64
	}
	results := make([]scored, 0, len(docs))
	for i, doc := range docs {
		tokens := docTokens[i]
		dl := float64(len(tokens))
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
			results = append(results, scored{doc: doc, score: score})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if len(results) > limit {
		results = results[:limit]
	}

	out := make([]memoryParagraph, len(results))
	for i, r := range results {
		out[i] = r.doc
	}
	return out
}
