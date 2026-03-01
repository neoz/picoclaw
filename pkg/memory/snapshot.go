package memory

import (
	"fmt"
	"os"
	"strings"
)

// snapshotSeparator is a unique separator between entries in snapshot files.
// Uses a marker unlikely to appear in normal markdown content.
const snapshotSeparator = "\n<!-- @@MEMORY_ENTRY@@ -->\n"

// legacySeparator is the old separator for backward compatibility on import.
const legacySeparator = "\n---\n"

// ExportSnapshot writes all core memories to a readable markdown file.
func (m *MemoryDB) ExportSnapshot(path string) error {
	entries, err := m.List("core", 1000, "")
	if err != nil {
		return fmt.Errorf("list core memories: %w", err)
	}

	if len(entries) == 0 {
		return nil
	}

	var sb strings.Builder
	sb.WriteString("# Memory Snapshot\n")
	for _, entry := range entries {
		sb.WriteString(snapshotSeparator)
		sb.WriteString(fmt.Sprintf("## %s\n\n%s\n", entry.Key, entry.Content))
	}

	return os.WriteFile(path, []byte(sb.String()), 0644)
}

// ImportSnapshot reads a snapshot file and stores each entry as core category.
// Idempotent via upsert on key. Supports both new and legacy separator formats.
func (m *MemoryDB) ImportSnapshot(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read snapshot: %w", err)
	}

	content := strings.TrimSpace(string(data))
	if content == "" {
		return nil
	}

	// Use new separator if present anywhere, otherwise fall back to legacy
	var sections []string
	if strings.Contains(content, "@@MEMORY_ENTRY@@") {
		sections = strings.Split(content, snapshotSeparator)
	} else {
		sections = strings.Split(content, legacySeparator)
	}

	for i, section := range sections {
		section = strings.TrimSpace(section)
		if section == "" {
			continue
		}

		// If section starts with a top-level heading (e.g., "# Memory Snapshot"),
		// strip it and continue with the remainder.
		if strings.HasPrefix(strings.TrimSpace(section), "# ") && !strings.HasPrefix(strings.TrimSpace(section), "## ") {
			if idx := strings.Index(section, "\n## "); idx >= 0 {
				section = strings.TrimSpace(section[idx+1:])
			} else {
				continue // header-only section, no entries
			}
		}

		// Extract key from ## heading if present
		lines := strings.SplitN(section, "\n", 2)
		firstLine := strings.TrimSpace(lines[0])
		key := fmt.Sprintf("imported_%d", i+1)
		body := section
		if strings.HasPrefix(firstLine, "## ") {
			key = strings.TrimPrefix(firstLine, "## ")
			key = strings.TrimSpace(key)
			if len(lines) >= 2 {
				body = strings.TrimSpace(lines[1])
			} else {
				body = ""
			}
		}

		if body != "" {
			if err := m.Store(key, body, "core", ""); err != nil {
				return fmt.Errorf("import entry %q: %w", key, err)
			}
		}
	}

	return nil
}
