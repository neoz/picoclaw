package memory

import (
	"fmt"
	"strings"
	"time"
)

// SearchResult represents a search hit with its BM25 rank and decayed confidence.
type SearchResult struct {
	Entry          MemoryEntry
	Rank           float64
	DecayedConfidence float64 // Confidence after time decay + access boost
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
			SELECT m.id, m.key, m.content, m.category, m.owner, m.confidence, m.access_count,
				m.created_at, m.updated_at, rank
			FROM memories_fts
			JOIN memories m ON memories_fts.rowid = m.id
			WHERE memories_fts MATCH ? AND (m.owner = '' OR m.owner = ?)
			ORDER BY rank
			LIMIT ?`
		args = []interface{}{ftsQuery, owner, limit}
	} else {
		sqlQuery = `
			SELECT m.id, m.key, m.content, m.category, m.owner, m.confidence, m.access_count,
				m.created_at, m.updated_at, rank
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

	results, err := scanSearchResults(rows)
	if err != nil {
		return results, err
	}

	// Auto-increment access counts for returned results
	ids := make([]int64, len(results))
	for i, r := range results {
		ids[i] = r.Entry.ID
	}
	m.IncrementAccessCount(ids)

	return results, nil
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
			SELECT m.id, m.key, m.content, m.category, m.owner, m.confidence, m.access_count,
				m.created_at, m.updated_at, rank
			FROM memories_fts
			JOIN memories m ON memories_fts.rowid = m.id
			WHERE memories_fts MATCH ? AND m.category = ? AND (m.owner = '' OR m.owner = ?)
			ORDER BY rank
			LIMIT ?`
		args = []interface{}{ftsQuery, category, owner, limit}
	} else {
		sqlQuery = `
			SELECT m.id, m.key, m.content, m.category, m.owner, m.confidence, m.access_count,
				m.created_at, m.updated_at, rank
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

	results, err := scanSearchResults(rows)
	if err != nil {
		return results, err
	}

	ids := make([]int64, len(results))
	for i, r := range results {
		ids[i] = r.Entry.ID
	}
	m.IncrementAccessCount(ids)

	return results, nil
}

// SearchWithOptions performs FTS5 search with extended filtering.
// Supports category, time_range, owner_scope, and min_confidence.
// Domain and tag filters are not applicable to the SQLite backend.
func (m *MemoryDB) SearchWithOptions(opts SearchOptions) ([]SearchResult, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}

	query := strings.TrimSpace(opts.Query)
	if query == "" {
		return nil, nil
	}

	ftsQuery := sanitizeFTS5Query(query)
	if ftsQuery == "" {
		return nil, nil
	}

	// Build dynamic WHERE clauses
	conditions := []string{"memories_fts MATCH ?"}
	args := []interface{}{ftsQuery}

	// Owner scoping
	switch opts.OwnerScope {
	case "shared":
		conditions = append(conditions, "m.owner = ''")
	case "private":
		if opts.Owner != "" {
			conditions = append(conditions, "m.owner = ?")
			args = append(args, opts.Owner)
		}
	default: // "all" or ""
		if opts.Owner != "" {
			conditions = append(conditions, "(m.owner = '' OR m.owner = ?)")
			args = append(args, opts.Owner)
		}
	}

	// Category filter
	if opts.Category != "" {
		conditions = append(conditions, "m.category = ?")
		args = append(args, opts.Category)
	}

	// Time range filter
	if opts.TimeRange != "" {
		var cutoff time.Time
		now := time.Now().UTC()
		switch opts.TimeRange {
		case "today":
			cutoff = now.Truncate(24 * time.Hour)
		case "week":
			cutoff = now.AddDate(0, 0, -7)
		case "month":
			cutoff = now.AddDate(0, -1, 0)
		}
		if !cutoff.IsZero() {
			conditions = append(conditions, "m.updated_at >= ?")
			args = append(args, cutoff.Format(time.RFC3339))
		}
	}

	args = append(args, limit)

	sqlQuery := fmt.Sprintf(`
		SELECT m.id, m.key, m.content, m.category, m.owner, m.confidence, m.access_count,
			m.created_at, m.updated_at, rank
		FROM memories_fts
		JOIN memories m ON memories_fts.rowid = m.id
		WHERE %s
		ORDER BY rank
		LIMIT ?`, strings.Join(conditions, " AND "))

	rows, err := m.db.Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("search memories with options: %w", err)
	}
	defer rows.Close()

	results, err := scanSearchResults(rows)
	if err != nil {
		return results, err
	}

	// Post-filter by min_confidence
	if opts.MinConfidence > 0 {
		filtered := results[:0]
		for _, r := range results {
			if r.DecayedConfidence >= opts.MinConfidence {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}

	ids := make([]int64, len(results))
	for i, r := range results {
		ids[i] = r.Entry.ID
	}
	m.IncrementAccessCount(ids)

	return results, nil
}

func scanSearchResults(rows interface {
	Next() bool
	Scan(dest ...interface{}) error
	Err() error
}) ([]SearchResult, error) {
	now := time.Now().UTC()
	var results []SearchResult
	for rows.Next() {
		var result SearchResult
		var createdAt, updatedAt string
		if err := rows.Scan(
			&result.Entry.ID, &result.Entry.Key, &result.Entry.Content,
			&result.Entry.Category, &result.Entry.Owner,
			&result.Entry.Confidence, &result.Entry.AccessCount,
			&createdAt, &updatedAt, &result.Rank,
		); err != nil {
			continue
		}
		result.Entry.CreatedAt = parseTime(createdAt)
		result.Entry.UpdatedAt = parseTime(updatedAt)
		result.DecayedConfidence = ComputeConfidence(
			result.Entry.Confidence, result.Entry.CreatedAt, now,
			result.Entry.AccessCount, result.Entry.Category,
		)
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
