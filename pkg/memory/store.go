package memory

import (
	"fmt"
	"strings"
	"time"
)

// Store inserts or updates a memory entry. The key is globally unique:
// any existing entry with the same key (regardless of owner) is replaced.
func (m *MemoryDB) Store(key, content, category, owner string) error {
	category = validateCategory(category)
	now := time.Now().UTC().Format("2006-01-02 15:04:05")

	// Delete any existing entries with this key (all owners) to prevent
	// duplicates from the UNIQUE(key, owner) constraint allowing
	// ("key", "") and ("key", "alice") to coexist.
	_, _ = m.db.Exec("DELETE FROM memories WHERE key = ?", key)

	_, err := m.db.Exec(`
		INSERT INTO memories (key, content, category, owner, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, key, content, category, owner, now, now)
	if err != nil {
		return fmt.Errorf("store memory: %w", err)
	}
	return nil
}

// Get retrieves a memory entry by key. Returns nil if not found.
func (m *MemoryDB) Get(key string) *MemoryEntry {
	row := m.db.QueryRow(`
		SELECT id, key, content, category, owner, created_at, updated_at
		FROM memories WHERE key = ?
	`, key)

	var entry MemoryEntry
	var createdAt, updatedAt string
	err := row.Scan(&entry.ID, &entry.Key, &entry.Content, &entry.Category, &entry.Owner, &createdAt, &updatedAt)
	if err != nil {
		return nil
	}
	entry.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	entry.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &entry
}

// Delete removes a memory entry by key. Returns true if deleted.
func (m *MemoryDB) Delete(key string) bool {
	result, err := m.db.Exec("DELETE FROM memories WHERE key = ?", key)
	if err != nil {
		return false
	}
	rows, _ := result.RowsAffected()
	return rows > 0
}

// DeleteByOwner removes a memory entry matching both key and owner.
// Use owner="" to delete shared entries only. Returns true if deleted.
func (m *MemoryDB) DeleteByOwner(key, owner string) bool {
	result, err := m.db.Exec("DELETE FROM memories WHERE key = ? AND owner = ?", key, owner)
	if err != nil {
		return false
	}
	rows, _ := result.RowsAffected()
	return rows > 0
}

// GetByOwner retrieves a memory entry by key and owner. Returns nil if not found.
func (m *MemoryDB) GetByOwner(key, owner string) *MemoryEntry {
	row := m.db.QueryRow(`
		SELECT id, key, content, category, owner, created_at, updated_at
		FROM memories WHERE key = ? AND owner = ?
	`, key, owner)

	var entry MemoryEntry
	var createdAt, updatedAt string
	err := row.Scan(&entry.ID, &entry.Key, &entry.Content, &entry.Category, &entry.Owner, &createdAt, &updatedAt)
	if err != nil {
		return nil
	}
	entry.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	entry.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &entry
}

// List returns memory entries filtered by category, ordered by updated_at DESC.
// Pass empty category to list all. When owner is non-empty, returns shared (owner='')
// plus that owner's entries. Pass owner="" to return all entries (no filter).
func (m *MemoryDB) List(category string, limit int, owner string) ([]MemoryEntry, error) {
	if limit <= 0 {
		limit = 20
	}

	var conditions []string
	var args []interface{}
	if category != "" {
		conditions = append(conditions, "category = ?")
		args = append(args, category)
	}
	if owner != "" {
		conditions = append(conditions, "(owner = '' OR owner = ?)")
		args = append(args, owner)
	}

	query := "SELECT id, key, content, category, owner, created_at, updated_at FROM memories"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY updated_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list memories: %w", err)
	}
	defer rows.Close()

	var entries []MemoryEntry
	for rows.Next() {
		var entry MemoryEntry
		var createdAt, updatedAt string
		if err := rows.Scan(&entry.ID, &entry.Key, &entry.Content, &entry.Category, &entry.Owner, &createdAt, &updatedAt); err != nil {
			continue
		}
		entry.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		entry.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		entries = append(entries, entry)
	}
	return entries, nil
}

// ListRecent returns entries from the given categories updated within the last N days.
// When owner is non-empty, returns shared + that owner's entries.
func (m *MemoryDB) ListRecent(categories []string, days, limit int, owner string) ([]MemoryEntry, error) {
	if limit <= 0 {
		limit = 10
	}
	if len(categories) == 0 {
		return nil, nil
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -days).Format("2006-01-02 15:04:05")

	// Build placeholders for IN clause
	placeholders := make([]string, len(categories))
	args := make([]interface{}, 0, len(categories)+4)
	for i, c := range categories {
		placeholders[i] = "?"
		args = append(args, c)
	}
	args = append(args, cutoff)

	ownerClause := ""
	if owner != "" {
		ownerClause = " AND (owner = '' OR owner = ?)"
		args = append(args, owner)
	}
	args = append(args, limit)

	query := fmt.Sprintf(`SELECT id, key, content, category, owner, created_at, updated_at
		FROM memories
		WHERE category IN (%s) AND updated_at >= ?%s
		ORDER BY updated_at DESC LIMIT ?`,
		strings.Join(placeholders, ","), ownerClause)

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list recent memories: %w", err)
	}
	defer rows.Close()

	var entries []MemoryEntry
	for rows.Next() {
		var entry MemoryEntry
		var createdAt, updatedAt string
		if err := rows.Scan(&entry.ID, &entry.Key, &entry.Content, &entry.Category, &entry.Owner, &createdAt, &updatedAt); err != nil {
			continue
		}
		entry.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		entry.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		entries = append(entries, entry)
	}
	return entries, nil
}

// Count returns the total number of memory entries.
func (m *MemoryDB) Count() int {
	var count int
	m.db.QueryRow("SELECT COUNT(*) FROM memories").Scan(&count)
	return count
}

// CountByCategory returns the number of entries in a given category.
func (m *MemoryDB) CountByCategory(category string) int {
	var count int
	m.db.QueryRow("SELECT COUNT(*) FROM memories WHERE category = ?", category).Scan(&count)
	return count
}
