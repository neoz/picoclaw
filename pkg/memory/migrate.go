package memory

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MigrateFromMarkdown performs a one-time migration of existing markdown memory files
// into the SQLite database. It checks the metadata table to avoid re-migration.
// Original markdown files are NOT deleted (kept as backup).
func (m *MemoryDB) MigrateFromMarkdown(memoryDir string) error {
	// Check if already migrated
	var migrated string
	err := m.db.QueryRow("SELECT value FROM metadata WHERE key = 'migrated_markdown'").Scan(&migrated)
	if err == nil && migrated == "true" {
		return nil // Already migrated
	}

	// Migrate MEMORY.md as core entries
	memoryFile := filepath.Join(memoryDir, "MEMORY.md")
	if data, err := os.ReadFile(memoryFile); err == nil {
		content := strings.TrimSpace(string(data))
		if content != "" {
			paragraphs := splitParagraphs(content)
			for i, p := range paragraphs {
				key := fmt.Sprintf("legacy_core_%d", i+1)
				if err := m.Store(key, p, "core", ""); err != nil {
					return fmt.Errorf("migrate core paragraph %d: %w", i+1, err)
				}
			}
		}
	}

	// Migrate daily notes (memory/YYYYMM/YYYYMMDD.md)
	filepath.Walk(memoryDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			return nil
		}
		// Skip MEMORY.md and MEMORY_SNAPSHOT.md
		baseName := strings.ToLower(info.Name())
		if baseName == "memory.md" || baseName == "memory_snapshot.md" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		content := strings.TrimSpace(string(data))
		if content == "" {
			return nil
		}

		// Use filename without extension as part of key
		name := strings.TrimSuffix(info.Name(), ".md")
		key := fmt.Sprintf("legacy_daily_%s", name)
		m.Store(key, content, "daily", "")
		return nil
	})

	// Also check for MEMORY_SNAPSHOT.md to hydrate if DB was empty before migration
	snapshotFile := filepath.Join(memoryDir, "MEMORY_SNAPSHOT.md")
	if data, err := os.ReadFile(snapshotFile); err == nil {
		content := strings.TrimSpace(string(data))
		if content != "" {
			paragraphs := splitParagraphs(content)
			for i, p := range paragraphs {
				key := fmt.Sprintf("snapshot_core_%d", i+1)
				// Use Store which does upsert - won't overwrite existing keys
				m.Store(key, p, "core", "")
			}
		}
	}

	// Mark migration complete
	_, err = m.db.Exec(
		"INSERT OR REPLACE INTO metadata (key, value) VALUES ('migrated_markdown', 'true')",
	)
	return err
}

// migrateAddOwner adds the owner column to existing databases.
// It checks if the column exists; if not, it recreates the table with the new schema.
func (m *MemoryDB) migrateAddOwner() error {
	// Check if owner column already exists
	rows, err := m.db.Query("PRAGMA table_info(memories)")
	if err != nil {
		// Table doesn't exist yet, createSchema will handle it
		return nil
	}
	defer rows.Close()

	hasOwner := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			continue
		}
		if name == "owner" {
			hasOwner = true
		}
	}

	if hasOwner {
		return nil // Already migrated
	}

	// Check if memories table exists at all (PRAGMA returns empty for non-existent tables)
	var tableExists int
	m.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='memories'").Scan(&tableExists)
	if tableExists == 0 {
		return nil // Table doesn't exist yet, createSchema will create it fresh
	}

	// Recreate table with owner column
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration tx: %w", err)
	}
	defer tx.Rollback()

	stmts := []string{
		// Drop FTS triggers first
		"DROP TRIGGER IF EXISTS memories_ai",
		"DROP TRIGGER IF EXISTS memories_ad",
		"DROP TRIGGER IF EXISTS memories_au",
		// Drop FTS table
		"DROP TABLE IF EXISTS memories_fts",
		// Rename old table
		"ALTER TABLE memories RENAME TO memories_old",
		// Create new table with owner column
		`CREATE TABLE memories (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			key        TEXT NOT NULL,
			content    TEXT NOT NULL,
			category   TEXT NOT NULL DEFAULT 'core',
			owner      TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT (datetime('now')),
			updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
			UNIQUE(key, owner)
		)`,
		// Copy data (all existing entries become shared with owner='')
		`INSERT INTO memories (id, key, content, category, owner, created_at, updated_at)
		 SELECT id, key, content, category, '', created_at, updated_at FROM memories_old`,
		// Drop old table
		"DROP TABLE memories_old",
		// Index
		"CREATE INDEX IF NOT EXISTS idx_memories_owner ON memories(owner)",
	}

	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("migration step failed: %w", err)
		}
	}

	return tx.Commit()
}

// splitParagraphs splits content by double newline into non-empty paragraphs.
func splitParagraphs(content string) []string {
	parts := strings.Split(content, "\n\n")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
