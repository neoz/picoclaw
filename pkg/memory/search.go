package memory

import (
	"fmt"
	"strings"
)

// SearchResult represents a search hit with its BM25 rank.
type SearchResult struct {
	Entry MemoryEntry
	Rank  float64
}

// Search performs FTS5 full-text search with BM25 ranking.
// When owner is non-empty, returns shared + that owner's entries.
func (m *MemoryDB) Search(query string, limit int, owner string) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}

	ftsQuery := sanitizeFTS5Query(query)
	if ftsQuery == "" {
		return nil, nil
	}

	var sqlQuery string
	var args []interface{}
	if owner != "" {
		sqlQuery = `
			SELECT m.id, m.key, m.content, m.category, m.owner, m.created_at, m.updated_at,
				rank
			FROM memories_fts
			JOIN memories m ON memories_fts.rowid = m.id
			WHERE memories_fts MATCH ? AND (m.owner = '' OR m.owner = ?)
			ORDER BY rank
			LIMIT ?`
		args = []interface{}{ftsQuery, owner, limit}
	} else {
		sqlQuery = `
			SELECT m.id, m.key, m.content, m.category, m.owner, m.created_at, m.updated_at,
				rank
			FROM memories_fts
			JOIN memories m ON memories_fts.rowid = m.id
			WHERE memories_fts MATCH ?
			ORDER BY rank
			LIMIT ?`
		args = []interface{}{ftsQuery, limit}
	}

	rows, err := m.db.Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("search memories: %w", err)
	}
	defer rows.Close()

	return scanSearchResults(rows)
}

// SearchByCategory performs FTS5 search filtered by category.
// When owner is non-empty, returns shared + that owner's entries.
func (m *MemoryDB) SearchByCategory(query, category string, limit int, owner string) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}

	ftsQuery := sanitizeFTS5Query(query)
	if ftsQuery == "" {
		return nil, nil
	}

	var sqlQuery string
	var args []interface{}
	if owner != "" {
		sqlQuery = `
			SELECT m.id, m.key, m.content, m.category, m.owner, m.created_at, m.updated_at,
				rank
			FROM memories_fts
			JOIN memories m ON memories_fts.rowid = m.id
			WHERE memories_fts MATCH ? AND m.category = ? AND (m.owner = '' OR m.owner = ?)
			ORDER BY rank
			LIMIT ?`
		args = []interface{}{ftsQuery, category, owner, limit}
	} else {
		sqlQuery = `
			SELECT m.id, m.key, m.content, m.category, m.owner, m.created_at, m.updated_at,
				rank
			FROM memories_fts
			JOIN memories m ON memories_fts.rowid = m.id
			WHERE memories_fts MATCH ? AND m.category = ?
			ORDER BY rank
			LIMIT ?`
		args = []interface{}{ftsQuery, category, limit}
	}

	rows, err := m.db.Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("search memories by category: %w", err)
	}
	defer rows.Close()

	return scanSearchResults(rows)
}

func scanSearchResults(rows interface {
	Next() bool
	Scan(dest ...interface{}) error
	Err() error
}) ([]SearchResult, error) {
	var results []SearchResult
	for rows.Next() {
		var result SearchResult
		var createdAt, updatedAt string
		if err := rows.Scan(
			&result.Entry.ID, &result.Entry.Key, &result.Entry.Content,
			&result.Entry.Category, &result.Entry.Owner, &createdAt, &updatedAt, &result.Rank,
		); err != nil {
			continue
		}
		result.Entry.CreatedAt = parseTime(createdAt)
		result.Entry.UpdatedAt = parseTime(updatedAt)
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return results, fmt.Errorf("scan search results: %w", err)
	}
	return results, nil
}

// fts5Replacer removes FTS5 special characters from query tokens.
var fts5Replacer = strings.NewReplacer(
	"*", "", "\"", "", "(", "", ")", "",
	":", "", "^", "", "{", "", "}", "",
)

// sanitizeFTS5Query escapes special FTS5 characters and wraps tokens in quotes.
func sanitizeFTS5Query(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return ""
	}

	// Split into tokens and wrap each in quotes to handle special chars
	tokens := strings.Fields(query)
	var quoted []string
	for _, t := range tokens {
		t = fts5Replacer.Replace(t)
		t = strings.TrimSpace(t)
		if t != "" {
			quoted = append(quoted, "\""+t+"\"")
		}
	}

	if len(quoted) == 0 {
		return ""
	}

	// Join with OR for broader matching
	return strings.Join(quoted, " OR ")
}
