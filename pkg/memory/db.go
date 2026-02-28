package memory

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// MemoryEntry represents a single memory record.
type MemoryEntry struct {
	ID        int64
	Key       string
	Content   string
	Category  string
	Owner     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ValidCategories defines the allowed memory categories.
var ValidCategories = map[string]bool{
	"core":         true,
	"daily":        true,
	"conversation": true,
	"custom":       true,
}

// MemoryDB manages the SQLite-backed memory database.
type MemoryDB struct {
	db        *sql.DB
	workspace string
	dbPath    string
}

// Open creates or opens the memory database at workspace/memory/memory.db.
func Open(workspace string) (*MemoryDB, error) {
	memoryDir := filepath.Join(workspace, "memory")
	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		return nil, fmt.Errorf("create memory dir: %w", err)
	}

	dbPath := filepath.Join(memoryDir, "memory.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Enable WAL mode for better concurrent read performance
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	// Enable foreign keys for cascade deletes (graph relations)
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	mdb := &MemoryDB{
		db:        db,
		workspace: workspace,
		dbPath:    dbPath,
	}

	if err := mdb.migrateAddOwner(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate owner column: %w", err)
	}

	if err := mdb.createSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}

	if err := mdb.createGraphSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("create graph schema: %w", err)
	}

	// Rebuild FTS index to repair any corruption from out-of-sync triggers.
	// Must run before migrations that use DELETE (dedup).
	if err := mdb.rebuildFTS(); err != nil {
		db.Close()
		return nil, fmt.Errorf("rebuild fts: %w", err)
	}

	if err := mdb.migrateDeduplicateKeys(); err != nil {
		db.Close()
		return nil, fmt.Errorf("deduplicate keys: %w", err)
	}

	return mdb, nil
}

// Close closes the database connection.
func (m *MemoryDB) Close() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}

// DBPath returns the path to the database file.
func (m *MemoryDB) DBPath() string {
	return m.dbPath
}

// Workspace returns the workspace path.
func (m *MemoryDB) Workspace() string {
	return m.workspace
}

func (m *MemoryDB) createSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS memories (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		key        TEXT NOT NULL,
		content    TEXT NOT NULL,
		category   TEXT NOT NULL DEFAULT 'core',
		owner      TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT (datetime('now')),
		updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
		UNIQUE(key, owner)
	);

	CREATE INDEX IF NOT EXISTS idx_memories_owner ON memories(owner);

	CREATE TABLE IF NOT EXISTS metadata (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);

	CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
		key,
		content,
		category,
		content='memories',
		content_rowid='id'
	);

	-- Triggers to keep FTS in sync
	CREATE TRIGGER IF NOT EXISTS memories_ai AFTER INSERT ON memories BEGIN
		INSERT INTO memories_fts(rowid, key, content, category)
		VALUES (new.id, new.key, new.content, new.category);
	END;

	CREATE TRIGGER IF NOT EXISTS memories_ad AFTER DELETE ON memories BEGIN
		INSERT INTO memories_fts(memories_fts, rowid, key, content, category)
		VALUES ('delete', old.id, old.key, old.content, old.category);
	END;

	CREATE TRIGGER IF NOT EXISTS memories_au AFTER UPDATE ON memories BEGIN
		INSERT INTO memories_fts(memories_fts, rowid, key, content, category)
		VALUES ('delete', old.id, old.key, old.content, old.category);
		INSERT INTO memories_fts(rowid, key, content, category)
		VALUES (new.id, new.key, new.content, new.category);
	END;
	`
	_, err := m.db.Exec(schema)
	return err
}

// validateCategory checks if the category is valid, defaults to "core".
func validateCategory(category string) string {
	if category == "" {
		return "core"
	}
	if ValidCategories[category] {
		return category
	}
	return "core"
}
