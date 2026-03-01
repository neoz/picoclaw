package memory

import (
	"testing"
	"time"
)

func TestStoreAndGet(t *testing.T) {
	db := openTestDB(t)

	err := db.Store("greeting", "hello world", "core", "")
	if err != nil {
		t.Fatal(err)
	}

	entry := db.Get("greeting")
	if entry == nil {
		t.Fatal("expected entry, got nil")
	}
	if entry.Content != "hello world" {
		t.Fatalf("expected 'hello world', got %q", entry.Content)
	}
	if entry.Owner != "" {
		t.Fatalf("expected empty owner, got %q", entry.Owner)
	}
}

func TestStoreReplacesAcrossOwners(t *testing.T) {
	db := openTestDB(t)

	// Store as shared
	db.Store("fact", "original", "core", "")
	if c := db.Count(); c != 1 {
		t.Fatalf("expected 1, got %d", c)
	}

	// Store same key with an owner - should replace, not duplicate
	db.Store("fact", "updated", "core", "alice")
	if c := db.Count(); c != 1 {
		t.Fatalf("expected 1 after owner change, got %d", c)
	}

	entry := db.Get("fact")
	if entry == nil {
		t.Fatal("expected entry")
	}
	if entry.Owner != "alice" {
		t.Fatalf("expected owner 'alice', got %q", entry.Owner)
	}
	if entry.Content != "updated" {
		t.Fatalf("expected 'updated', got %q", entry.Content)
	}
}

func TestStoreUpsertSameOwner(t *testing.T) {
	db := openTestDB(t)

	db.Store("note", "v1", "core", "alice")
	db.Store("note", "v2", "core", "alice")

	if c := db.Count(); c != 1 {
		t.Fatalf("expected 1, got %d", c)
	}

	entry := db.Get("note")
	if entry.Content != "v2" {
		t.Fatalf("expected 'v2', got %q", entry.Content)
	}
}

func TestGetByOwner(t *testing.T) {
	db := openTestDB(t)

	db.Store("item", "shared version", "core", "")

	// GetByOwner with matching owner
	entry := db.GetByOwner("item", "")
	if entry == nil {
		t.Fatal("expected shared entry")
	}
	if entry.Content != "shared version" {
		t.Fatalf("expected 'shared version', got %q", entry.Content)
	}

	// GetByOwner with non-matching owner
	entry = db.GetByOwner("item", "bob")
	if entry != nil {
		t.Fatal("expected nil for non-matching owner")
	}
}

func TestDeleteByOwner(t *testing.T) {
	db := openTestDB(t)

	db.Store("x", "content", "core", "alice")

	// Wrong owner should not delete
	if db.DeleteByOwner("x", "bob") {
		t.Fatal("should not delete entry owned by alice when targeting bob")
	}
	if db.Count() != 1 {
		t.Fatal("entry should still exist")
	}

	// Correct owner should delete
	if !db.DeleteByOwner("x", "alice") {
		t.Fatal("should delete entry owned by alice")
	}
	if db.Count() != 0 {
		t.Fatal("entry should be gone")
	}
}

func TestDeleteByOwnerShared(t *testing.T) {
	db := openTestDB(t)

	db.Store("shared_fact", "data", "core", "")

	// Delete with owner="" targets shared entries
	if !db.DeleteByOwner("shared_fact", "") {
		t.Fatal("should delete shared entry")
	}
	if db.Count() != 0 {
		t.Fatal("entry should be gone")
	}
}

func TestDeleteAccessibleOwned(t *testing.T) {
	db := openTestDB(t)

	db.Store("fact", "data", "core", "alice")

	// alice can delete her own entry
	if !db.DeleteAccessible("fact", "alice") {
		t.Fatal("should delete alice's entry")
	}
	if db.Count() != 0 {
		t.Fatal("should be empty")
	}
}

func TestDeleteAccessibleShared(t *testing.T) {
	db := openTestDB(t)

	db.Store("fact", "data", "core", "")

	// alice can delete shared entries
	if !db.DeleteAccessible("fact", "alice") {
		t.Fatal("should delete shared entry")
	}
	if db.Count() != 0 {
		t.Fatal("should be empty")
	}
}

func TestDeleteAccessibleProtectsOthers(t *testing.T) {
	db := openTestDB(t)

	db.Store("secret", "data", "core", "bob")

	// alice cannot delete bob's private entry
	if db.DeleteAccessible("secret", "alice") {
		t.Fatal("should not delete bob's entry")
	}
	if db.Count() != 1 {
		t.Fatal("bob's entry should still exist")
	}
}

func TestDeleteAllByKey(t *testing.T) {
	db := openTestDB(t)

	db.Store("k", "data", "core", "alice")

	if !db.Delete("k") {
		t.Fatal("should delete")
	}
	if db.Count() != 0 {
		t.Fatal("should be empty")
	}
}

func TestListOwnerFilter(t *testing.T) {
	db := openTestDB(t)

	db.Store("shared1", "s1", "core", "")
	db.Store("owned1", "o1", "core", "alice")
	db.Store("other1", "x1", "core", "bob")

	// List with owner="alice" should return shared + alice's entries
	entries, err := db.List("", 10, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 (shared + alice), got %d", len(entries))
	}

	keys := make(map[string]bool)
	for _, e := range entries {
		keys[e.Key] = true
	}
	if !keys["shared1"] || !keys["owned1"] {
		t.Fatalf("expected shared1 and owned1, got %v", keys)
	}

	// List with owner="" returns all
	entries, err = db.List("", 10, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 (all), got %d", len(entries))
	}
}

func TestSearchOwnerFilter(t *testing.T) {
	db := openTestDB(t)

	db.Store("user_alice", "Alice likes Go", "core", "")
	db.Store("user_bob", "Bob likes Rust", "core", "bob")

	// Search as alice: should find shared entries, not bob's
	results, err := db.Search("likes", 10, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 (shared only), got %d", len(results))
	}
	if results[0].Entry.Key != "user_alice" {
		t.Fatalf("expected user_alice, got %q", results[0].Entry.Key)
	}
}

func TestMigrateDeduplicateKeys(t *testing.T) {
	db := openTestDB(t)

	// Manually insert duplicates to simulate pre-fix data.
	// Bypass Store() which now prevents duplicates.
	now := "2026-01-01 00:00:00"
	newer := "2026-01-02 00:00:00"
	db.db.Exec(`INSERT INTO memories (key, content, category, owner, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`, "dup", "old shared", "core", "", now, now)
	db.db.Exec(`INSERT INTO memories (key, content, category, owner, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`, "dup", "newer owned", "core", "alice", newer, newer)

	if db.Count() != 2 {
		t.Fatalf("expected 2 pre-migration rows, got %d", db.Count())
	}

	// Reset migration flag so it runs again
	db.db.Exec("DELETE FROM metadata WHERE key = '" + metaDeduplicatedKeys + "'")

	err := db.migrateDeduplicateKeys()
	if err != nil {
		t.Fatal(err)
	}

	if db.Count() != 1 {
		t.Fatalf("expected 1 after dedup, got %d", db.Count())
	}

	// Should keep the newer entry
	entry := db.Get("dup")
	if entry == nil {
		t.Fatal("expected entry")
	}
	if entry.Content != "newer owned" {
		t.Fatalf("expected newer entry kept, got %q", entry.Content)
	}
}

func TestMigrateDeduplicateKeysIdempotent(t *testing.T) {
	db := openTestDB(t)

	db.Store("k1", "v1", "core", "")
	db.Store("k2", "v2", "core", "alice")

	// Reset flag and run - should be safe with no duplicates
	db.db.Exec("DELETE FROM metadata WHERE key = '" + metaDeduplicatedKeys + "'")
	err := db.migrateDeduplicateKeys()
	if err != nil {
		t.Fatal(err)
	}
	if db.Count() != 2 {
		t.Fatalf("expected 2, got %d", db.Count())
	}

	// Run again - should skip (already migrated)
	err = db.migrateDeduplicateKeys()
	if err != nil {
		t.Fatal(err)
	}
	if db.Count() != 2 {
		t.Fatalf("expected 2, got %d", db.Count())
	}
}

// === Fix #1: busy_timeout pragma ===

func TestBusyTimeoutPragma(t *testing.T) {
	db := openTestDB(t)

	var timeout int
	err := db.db.QueryRow("PRAGMA busy_timeout").Scan(&timeout)
	if err != nil {
		t.Fatal(err)
	}
	if timeout != 5000 {
		t.Fatalf("expected busy_timeout=5000, got %d", timeout)
	}
}

// === Fix #2: Store preserves created_at on update ===

func TestStorePreservesCreatedAt(t *testing.T) {
	db := openTestDB(t)

	// Store initial entry
	db.Store("fact", "v1", "core", "")
	entry := db.Get("fact")
	if entry == nil {
		t.Fatal("expected entry")
	}
	originalCreatedAt := entry.CreatedAt

	// Wait a moment so timestamps differ
	time.Sleep(10 * time.Millisecond)

	// Update the same key
	db.Store("fact", "v2", "core", "")
	entry = db.Get("fact")
	if entry == nil {
		t.Fatal("expected entry after update")
	}
	if entry.Content != "v2" {
		t.Fatalf("expected content 'v2', got %q", entry.Content)
	}
	// created_at must be preserved from original insert
	if !entry.CreatedAt.Equal(originalCreatedAt) {
		t.Fatalf("created_at changed: original=%v, after update=%v", originalCreatedAt, entry.CreatedAt)
	}
	// updated_at must be newer or equal
	if entry.UpdatedAt.Before(originalCreatedAt) {
		t.Fatalf("updated_at should be >= created_at")
	}
}

func TestStorePreservesCreatedAtAcrossOwners(t *testing.T) {
	db := openTestDB(t)

	db.Store("fact", "v1", "core", "")
	entry := db.Get("fact")
	originalCreatedAt := entry.CreatedAt

	time.Sleep(10 * time.Millisecond)

	// Re-store with different owner - created_at from original should be preserved
	db.Store("fact", "v2", "core", "alice")
	entry = db.Get("fact")
	if entry == nil {
		t.Fatal("expected entry")
	}
	if entry.Owner != "alice" {
		t.Fatalf("expected owner 'alice', got %q", entry.Owner)
	}
	if !entry.CreatedAt.Equal(originalCreatedAt) {
		t.Fatalf("created_at changed across owner change: original=%v, after=%v", originalCreatedAt, entry.CreatedAt)
	}
}

// === Fix #4: FTS rebuild on every startup ===

func TestFTSRebuildOnReopen(t *testing.T) {
	dir := t.TempDir()

	// Open, store, close
	db1, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	db1.Store("golang", "Go is a programming language", "core", "")
	db1.Store("rustlang", "Rust is a systems language", "core", "")
	db1.Close()

	// Reopen - FTS should be rebuilt and search should work
	db2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()

	results, err := db2.Search("programming", 10, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 search result after reopen, got %d", len(results))
	}
	if results[0].Entry.Key != "golang" {
		t.Fatalf("expected key 'golang', got %q", results[0].Entry.Key)
	}
}

func TestFTSRebuildRecoversTruncatedIndex(t *testing.T) {
	dir := t.TempDir()

	db, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	db.Store("item1", "alpha beta gamma", "core", "")
	db.Store("item2", "delta epsilon zeta", "core", "")

	// Corrupt FTS by dropping it manually
	db.db.Exec("DROP TABLE IF EXISTS memories_fts")
	db.Close()

	// Reopen should rebuild FTS and searches should work again
	db2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()

	results, err := db2.Search("alpha", 10, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result after FTS recovery, got %d", len(results))
	}
}

// === Fix #9: updated_at and category indexes ===

func TestIndexesExist(t *testing.T) {
	db := openTestDB(t)

	indexes := make(map[string]bool)
	rows, err := db.db.Query("SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='memories'")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		rows.Scan(&name)
		indexes[name] = true
	}

	if !indexes["idx_memories_updated_at"] {
		t.Error("missing index idx_memories_updated_at")
	}
	if !indexes["idx_memories_category"] {
		t.Error("missing index idx_memories_category")
	}
	if !indexes["idx_memories_owner"] {
		t.Error("missing index idx_memories_owner")
	}
}

// === Fix #11: parseTime returns safe fallback ===

func TestParseTimeSQLiteFormat(t *testing.T) {
	result := parseTime("2026-01-15 10:30:00")
	if result.Year() != 2026 || result.Month() != 1 || result.Day() != 15 {
		t.Fatalf("failed to parse sqlite format: %v", result)
	}
}

func TestParseTimeRFC3339(t *testing.T) {
	result := parseTime("2026-01-15T10:30:00Z")
	if result.Year() != 2026 || result.Month() != 1 || result.Day() != 15 {
		t.Fatalf("failed to parse RFC3339 format: %v", result)
	}
}

func TestParseTimeRFC3339WithOffset(t *testing.T) {
	result := parseTime("2026-01-15T10:30:00+07:00")
	if result.Year() != 2026 || result.Month() != 1 || result.Day() != 15 {
		t.Fatalf("failed to parse RFC3339 with offset: %v", result)
	}
}

func TestParseTimeEmptyString(t *testing.T) {
	before := time.Now().UTC()
	result := parseTime("")
	after := time.Now().UTC()

	// Should return approximately now, not zero
	if result.IsZero() {
		t.Fatal("parseTime('') should not return zero time")
	}
	if result.Before(before) || result.After(after) {
		t.Fatalf("parseTime('') should return ~now, got %v (expected between %v and %v)", result, before, after)
	}
}

func TestParseTimeGarbageInput(t *testing.T) {
	before := time.Now().UTC()
	result := parseTime("not-a-timestamp")
	after := time.Now().UTC()

	// Should return approximately now, not zero
	if result.IsZero() {
		t.Fatal("parseTime with bad input should not return zero time")
	}
	if result.Before(before) || result.After(after) {
		t.Fatalf("parseTime with bad input should return ~now, got %v", result)
	}
}
