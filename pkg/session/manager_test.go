package session

import (
	"strings"
	"testing"
)

func TestListSessionKeys_Empty(t *testing.T) {
	sm := NewSessionManager("")
	keys := sm.ListSessionKeys()
	if len(keys) != 0 {
		t.Errorf("expected empty slice, got %v", keys)
	}
}

func TestListSessionKeys_ReturnsSorted(t *testing.T) {
	sm := NewSessionManager("")
	// Create sessions in non-alphabetical order
	sm.GetOrCreate("telegram:999")
	sm.GetOrCreate("discord:123")
	sm.GetOrCreate("telegram:100")

	keys := sm.ListSessionKeys()
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}
	want := []string{"discord:123", "telegram:100", "telegram:999"}
	for i, k := range keys {
		if k != want[i] {
			t.Errorf("keys[%d]=%q, want %q", i, k, want[i])
		}
	}
}

func TestListSessionKeys_AfterAddMessage(t *testing.T) {
	sm := NewSessionManager("")
	sm.AddMessage("qq:group1", "user", "hello")

	keys := sm.ListSessionKeys()
	if len(keys) != 1 || keys[0] != "qq:group1" {
		t.Errorf("expected [qq:group1], got %v", keys)
	}
}

func TestRecentLog_FilterBySenderName(t *testing.T) {
	sm := NewSessionManager("")
	sm.AddToLog("tg:1", "hello from alice", "u1", "Alice")
	sm.AddToLog("tg:1", "hello from bob", "u2", "Bob")
	sm.AddToLog("tg:1", "alice again", "u1", "Alice")

	entries := sm.RecentLog("tg:1", 50, 7, "", "Alice")
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	for _, e := range entries {
		if e.SenderName != "Alice" {
			t.Errorf("expected sender Alice, got %q", e.SenderName)
		}
	}
}

func TestRecentLog_FilterBySenderNameCaseInsensitive(t *testing.T) {
	sm := NewSessionManager("")
	sm.AddToLog("tg:1", "msg1", "u1", "Alice")
	sm.AddToLog("tg:1", "msg2", "u2", "Bob")

	entries := sm.RecentLog("tg:1", 50, 7, "", "alice")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Content != "msg1" {
		t.Errorf("expected msg1, got %q", entries[0].Content)
	}
}

func TestRecentLog_FilterBySenderNamePartialMatch(t *testing.T) {
	sm := NewSessionManager("")
	sm.AddToLog("tg:1", "msg1", "u1", "Alice Smith")
	sm.AddToLog("tg:1", "msg2", "u2", "Bob Jones")

	entries := sm.RecentLog("tg:1", 50, 7, "", "smith")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].SenderName != "Alice Smith" {
		t.Errorf("expected Alice Smith, got %q", entries[0].SenderName)
	}
}

func TestRecentLog_FilterBySenderIDAndName(t *testing.T) {
	sm := NewSessionManager("")
	sm.AddToLog("tg:1", "msg1", "u1", "Alice")
	sm.AddToLog("tg:1", "msg2", "u2", "Alice Two")
	sm.AddToLog("tg:1", "msg3", "u1", "Alice")

	// Both filters must match
	entries := sm.RecentLog("tg:1", 50, 7, "u1", "Alice")
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	for _, e := range entries {
		if e.SenderID != "u1" {
			t.Errorf("expected sender_id u1, got %q", e.SenderID)
		}
	}
}

func TestRecentLog_FilterBySenderNameNoMatch(t *testing.T) {
	sm := NewSessionManager("")
	sm.AddToLog("tg:1", "msg1", "u1", "Alice")

	entries := sm.RecentLog("tg:1", 50, 7, "", "Charlie")
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestGetLog_FilterBySenderName(t *testing.T) {
	sm := NewSessionManager("")
	sm.AddToLog("tg:1", "weather report", "u1", "Alice")
	sm.AddToLog("tg:1", "pizza order", "u2", "Bob")
	sm.AddToLog("tg:1", "meeting notes", "u1", "Alice")

	entries := sm.GetLog("tg:1", 7, "", "bob")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if !strings.Contains(entries[0].Content, "pizza") {
		t.Errorf("expected pizza order, got %q", entries[0].Content)
	}
}
