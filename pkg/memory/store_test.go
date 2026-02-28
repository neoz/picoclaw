package memory

import (
	"testing"
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
	db.db.Exec("DELETE FROM metadata WHERE key = 'deduplicated_keys'")

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
	db.db.Exec("DELETE FROM metadata WHERE key = 'deduplicated_keys'")
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
