package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// === Fix #8: Robust snapshot separator ===

func TestSnapshotRoundTrip(t *testing.T) {
	db := openTestDB(t)

	db.Store("fact1", "The sky is blue", "core", "")
	db.Store("fact2", "Water is wet", "core", "")

	path := filepath.Join(t.TempDir(), "snapshot.md")
	if err := db.ExportSnapshot(path); err != nil {
		t.Fatal(err)
	}

	// Import into a fresh DB
	db2 := openTestDB(t)
	if err := db2.ImportSnapshot(path); err != nil {
		t.Fatal(err)
	}

	e1 := db2.Get("fact1")
	if e1 == nil || e1.Content != "The sky is blue" {
		t.Fatalf("expected fact1 content 'The sky is blue', got %v", e1)
	}
	e2 := db2.Get("fact2")
	if e2 == nil || e2.Content != "Water is wet" {
		t.Fatalf("expected fact2 content 'Water is wet', got %v", e2)
	}
}

func TestSnapshotContentWithMarkdownHRule(t *testing.T) {
	db := openTestDB(t)

	// Content that contains markdown horizontal rules (---) which previously
	// would break the import parser.
	contentWithHR := "First section\n\n---\n\nSecond section after hr"
	db.Store("tricky", contentWithHR, "core", "")

	path := filepath.Join(t.TempDir(), "snapshot.md")
	if err := db.ExportSnapshot(path); err != nil {
		t.Fatal(err)
	}

	db2 := openTestDB(t)
	if err := db2.ImportSnapshot(path); err != nil {
		t.Fatal(err)
	}

	entry := db2.Get("tricky")
	if entry == nil {
		t.Fatal("expected entry 'tricky'")
	}
	if entry.Content != contentWithHR {
		t.Fatalf("content corrupted by hr in content:\ngot:  %q\nwant: %q", entry.Content, contentWithHR)
	}
}

func TestSnapshotExportUsesNewSeparator(t *testing.T) {
	db := openTestDB(t)

	db.Store("a", "content a", "core", "")
	db.Store("b", "content b", "core", "")

	path := filepath.Join(t.TempDir(), "snapshot.md")
	db.ExportSnapshot(path)

	data, _ := os.ReadFile(path)
	content := string(data)

	if !strings.Contains(content, "@@MEMORY_ENTRY@@") {
		t.Error("exported snapshot should use the new separator marker")
	}
}

func TestSnapshotImportLegacyFormat(t *testing.T) {
	// Simulate a legacy snapshot file that uses \n---\n as separator
	legacy := "# Memory Snapshot\n\n## old_fact\n\nLegacy content here\n---\n## another_fact\n\nMore legacy content"

	path := filepath.Join(t.TempDir(), "legacy.md")
	os.WriteFile(path, []byte(legacy), 0644)

	db := openTestDB(t)
	if err := db.ImportSnapshot(path); err != nil {
		t.Fatal(err)
	}

	entry := db.Get("old_fact")
	if entry == nil {
		t.Fatal("expected 'old_fact' from legacy import")
	}
	if entry.Content != "Legacy content here" {
		t.Fatalf("expected 'Legacy content here', got %q", entry.Content)
	}

	entry2 := db.Get("another_fact")
	if entry2 == nil {
		t.Fatal("expected 'another_fact' from legacy import")
	}
}

func TestSnapshotEmptyDB(t *testing.T) {
	db := openTestDB(t)

	path := filepath.Join(t.TempDir(), "snapshot.md")
	if err := db.ExportSnapshot(path); err != nil {
		t.Fatal(err)
	}

	// File should not be created for empty DB
	if _, err := os.Stat(path); err == nil {
		t.Error("snapshot file should not be created for empty DB")
	}
}
