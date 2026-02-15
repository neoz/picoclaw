package tools

import (
	"math"
	"sort"
	"strings"
)

// tokenize splits text into lowercase tokens.
func tokenize(text string) []string {
	return strings.Fields(strings.ToLower(text))
}

// bm25Rank scores documents against a query using BM25 and returns indices
// sorted by descending score. Only indices with score > 0 are returned.
// If limit <= 0, all matching indices are returned.
func bm25Rank(docs []string, query string, limit int) []int {
	queryTerms := tokenize(query)
	if len(queryTerms) == 0 || len(docs) == 0 {
		return nil
	}

	n := float64(len(docs))
	k1 := 1.2
	b := 0.75

	// Compute average document length
	var totalLen float64
	docTokens := make([][]string, len(docs))
	for i, doc := range docs {
		tokens := tokenize(doc)
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
		index int
		score float64
	}
	results := make([]scored, 0, len(docs))
	for i := range docs {
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
			results = append(results, scored{index: i, score: score})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	indices := make([]int, len(results))
	for i, r := range results {
		indices[i] = r.index
	}
	return indices
}
