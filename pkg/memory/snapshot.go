package memory

import (
	"fmt"
	"os"
	"strings"
)

// ExportSnapshot writes all core memories to a readable markdown file.
func (m *MemoryDB) ExportSnapshot(path string) error {
	entries, err := m.List("core", 1000)
	if err != nil {
		return fmt.Errorf("list core memories: %w", err)
	}

	if len(entries) == 0 {
		return nil
	}

	var sb strings.Builder
	sb.WriteString("# Memory Snapshot\n\n")
	for i, entry := range entries {
		if i > 0 {
			sb.WriteString("\n---\n\n")
		}
		sb.WriteString(fmt.Sprintf("## %s\n\n%s\n", entry.Key, entry.Content))
	}

	return os.WriteFile(path, []byte(sb.String()), 0644)
}

// ImportSnapshot reads a snapshot file and stores each entry as core category.
// Idempotent via upsert on key.
func (m *MemoryDB) ImportSnapshot(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read snapshot: %w", err)
	}

	content := strings.TrimSpace(string(data))
	if content == "" {
		return nil
	}

	// Parse sections: ## key\n\ncontent
	sections := strings.Split(content, "\n---\n")
	for i, section := range sections {
		section = strings.TrimSpace(section)
		if section == "" {
			continue
		}

		// Try to extract key from ## heading
		key := fmt.Sprintf("imported_%d", i+1)
		body := section

		lines := strings.SplitN(section, "\n", 2)
		if len(lines) >= 1 && strings.HasPrefix(lines[0], "## ") {
			key = strings.TrimPrefix(lines[0], "## ")
			key = strings.TrimSpace(key)
			if len(lines) >= 2 {
				body = strings.TrimSpace(lines[1])
			} else {
				body = ""
			}
		}

		// Skip the top-level heading line
		if strings.HasPrefix(key, "# ") {
			continue
		}

		if body != "" {
			if err := m.Store(key, body, "core"); err != nil {
				return fmt.Errorf("import entry %q: %w", key, err)
			}
		}
	}

	return nil
}
