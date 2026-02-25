package session

import "testing"

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
