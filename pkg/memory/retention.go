package memory

import (
	"fmt"
	"time"
)

// minConfidenceThreshold is the confidence below which entries are eligible
// for cleanup. Entries with decayed confidence below this are effectively
// invisible to queries and safe to prune.
const minConfidenceThreshold = 0.01

// RunRetention deletes expired entries using confidence decay.
// For each non-core category, entries whose decayed confidence falls below
// minConfidenceThreshold are deleted. The retentionDays map provides a hard
// upper bound: entries older than the configured days are always deleted
// regardless of confidence (safety net for entries with high access counts).
// The "core" category is never deleted (permanent, lambda=0).
// Returns total number of deleted entries.
func (m *MemoryDB) RunRetention(retentionDays map[string]int) (int, error) {
	totalDeleted := 0
	now := time.Now().UTC()

	for category, days := range retentionDays {
		if category == "core" || days <= 0 {
			continue
		}

		// Hard cutoff: always delete entries older than configured retention days
		hardCutoff := now.AddDate(0, 0, -days).Format(sqliteTimeFormat)
		result, err := m.db.Exec(
			"DELETE FROM memories WHERE category = ? AND updated_at < ?",
			category, hardCutoff,
		)
		if err != nil {
			return totalDeleted, fmt.Errorf("retention cleanup for %s: %w", category, err)
		}
		rows, _ := result.RowsAffected()
		totalDeleted += int(rows)

		// Soft cleanup: delete entries whose confidence has decayed below threshold.
		// Scan remaining entries and check computed confidence.
		deleted, err := m.deleteByLowConfidence(category, now)
		if err != nil {
			return totalDeleted, fmt.Errorf("confidence cleanup for %s: %w", category, err)
		}
		totalDeleted += deleted
	}

	// Clean relations whose memory_key no longer exists, then orphaned entities
	if totalDeleted > 0 {
		m.CleanStaleRelations()
		m.CleanOrphanedEntities()
	}

	return totalDeleted, nil
}

// deleteByLowConfidence removes entries in a category whose decayed confidence
// is below the threshold. Returns the number of deleted entries.
func (m *MemoryDB) deleteByLowConfidence(category string, now time.Time) (int, error) {
	rows, err := m.db.Query(
		"SELECT id, confidence, access_count, created_at FROM memories WHERE category = ?",
		category,
	)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var toDelete []int64
	for rows.Next() {
		var id int64
		var conf float64
		var accessCount int
		var createdAt string
		if err := rows.Scan(&id, &conf, &accessCount, &createdAt); err != nil {
			continue
		}
		decayed := ComputeConfidence(conf, parseTime(createdAt), now, accessCount, category)
		if decayed < minConfidenceThreshold {
			toDelete = append(toDelete, id)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	for _, id := range toDelete {
		m.db.Exec("DELETE FROM memories WHERE id = ?", id)
	}
	return len(toDelete), nil
}
