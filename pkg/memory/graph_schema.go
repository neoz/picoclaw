package memory

// createGraphSchema adds entity and relation tables for the knowledge graph layer.
func (m *MemoryDB) createGraphSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS entities (
		id   INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		type TEXT NOT NULL DEFAULT 'thing'
	);

	CREATE TABLE IF NOT EXISTS relations (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		source_id  INTEGER NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
		relation   TEXT NOT NULL,
		target_id  INTEGER NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
		memory_key TEXT,
		weight     REAL NOT NULL DEFAULT 1.0,
		created_at DATETIME NOT NULL DEFAULT (datetime('now')),
		UNIQUE(source_id, relation, target_id)
	);

	CREATE INDEX IF NOT EXISTS idx_rel_source ON relations(source_id);
	CREATE INDEX IF NOT EXISTS idx_rel_target ON relations(target_id);
	CREATE INDEX IF NOT EXISTS idx_rel_memory ON relations(memory_key);
	`
	_, err := m.db.Exec(schema)
	return err
}
