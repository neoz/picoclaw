package memory

import (
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
				if err := m.Store(key, p, "core"); err != nil {
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
		m.Store(key, content, "daily")
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
				m.Store(key, p, "core")
			}
		}
	}

	// Mark migration complete
	_, err = m.db.Exec(
		"INSERT OR REPLACE INTO metadata (key, value) VALUES ('migrated_markdown', 'true')",
	)
	return err
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
