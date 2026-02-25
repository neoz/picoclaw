package memory

import (
	"fmt"
	"time"
)

// RunRetention deletes expired entries per category.
// The "core" category is never deleted (permanent).
// Returns total number of deleted entries.
func (m *MemoryDB) RunRetention(retentionDays map[string]int) (int, error) {
	totalDeleted := 0

	for category, days := range retentionDays {
		if category == "core" || days <= 0 {
			continue
		}

		cutoff := time.Now().UTC().AddDate(0, 0, -days).Format("2006-01-02 15:04:05")
		result, err := m.db.Exec(
			"DELETE FROM memories WHERE category = ? AND updated_at < ?",
			category, cutoff,
		)
		if err != nil {
			return totalDeleted, fmt.Errorf("retention cleanup for %s: %w", category, err)
		}

		rows, _ := result.RowsAffected()
		totalDeleted += int(rows)
	}

	// Clean relations whose memory_key no longer exists, then orphaned entities
	if totalDeleted > 0 {
		m.CleanStaleRelations()
		m.CleanOrphanedEntities()
	}

	return totalDeleted, nil
}
