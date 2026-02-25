package channels

import (
	"testing"
)

// --- IsAllowed with agent suffix ---

func TestIsAllowed_EmptyList(t *testing.T) {
	bc := NewBaseChannel("test", nil, nil, []string{})
	if !bc.IsAllowed("anyone") {
		t.Error("empty allow list should allow everyone")
	}
}

func TestIsAllowed_SimpleMatch(t *testing.T) {
	bc := NewBaseChannel("test", nil, nil, []string{"alice", "bob"})
	if !bc.IsAllowed("alice") {
		t.Error("alice should be allowed")
	}
	if !bc.IsAllowed("bob") {
		t.Error("bob should be allowed")
	}
	if bc.IsAllowed("eve") {
		t.Error("eve should not be allowed")
	}
}

func TestIsAllowed_WithAgentSuffix(t *testing.T) {
	bc := NewBaseChannel("test", nil, nil, []string{"alice", "bob:limited"})
	if !bc.IsAllowed("alice") {
		t.Error("alice should be allowed")
	}
	if !bc.IsAllowed("bob") {
		t.Error("bob should be allowed (agent suffix stripped for matching)")
	}
	if bc.IsAllowed("limited") {
		t.Error("agent suffix alone should not match")
	}
}

func TestIsAllowed_CompoundSenderWithAgentSuffix(t *testing.T) {
	bc := NewBaseChannel("test", nil, nil, []string{"12345|bob:limited"})
	if !bc.IsAllowed("12345|bob") {
		t.Error("compound sender should match")
	}
	if !bc.IsAllowed("12345") {
		t.Error("ID-only sender should match")
	}
}

func TestIsAllowed_AtPrefixWithAgentSuffix(t *testing.T) {
	bc := NewBaseChannel("test", nil, nil, []string{"@bob:limited"})
	if !bc.IsAllowed("bob") {
		t.Error("bob should match after stripping @ and :limited")
	}
}

func TestIsAllowed_CompoundSenderMatchUsername(t *testing.T) {
	// Sender "12345|bob" should match allow entry "bob:limited"
	bc := NewBaseChannel("test", nil, nil, []string{"bob:limited"})
	if !bc.IsAllowed("12345|bob") {
		t.Error("compound sender userPart should match bare entry")
	}
}

// --- ResolveAgentID ---

func TestResolveAgentID_NoSuffix(t *testing.T) {
	bc := NewBaseChannel("test", nil, nil, []string{"alice", "bob"})
	if got := bc.ResolveAgentID("alice"); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestResolveAgentID_WithSuffix(t *testing.T) {
	bc := NewBaseChannel("test", nil, nil, []string{"alice", "bob:limited"})
	if got := bc.ResolveAgentID("bob"); got != "limited" {
		t.Errorf("expected %q, got %q", "limited", got)
	}
}

func TestResolveAgentID_DefaultUserNoSuffix(t *testing.T) {
	bc := NewBaseChannel("test", nil, nil, []string{"alice", "bob:limited"})
	if got := bc.ResolveAgentID("alice"); got != "" {
		t.Errorf("expected empty for alice, got %q", got)
	}
}

func TestResolveAgentID_NotInList(t *testing.T) {
	bc := NewBaseChannel("test", nil, nil, []string{"alice"})
	if got := bc.ResolveAgentID("eve"); got != "" {
		t.Errorf("expected empty for unknown user, got %q", got)
	}
}

func TestResolveAgentID_CompoundSender(t *testing.T) {
	bc := NewBaseChannel("test", nil, nil, []string{"12345|bob:limited"})
	if got := bc.ResolveAgentID("12345|bob"); got != "limited" {
		t.Errorf("expected %q, got %q", "limited", got)
	}
	if got := bc.ResolveAgentID("12345"); got != "limited" {
		t.Errorf("expected %q for ID-only, got %q", "limited", got)
	}
}

func TestResolveAgentID_AtPrefix(t *testing.T) {
	bc := NewBaseChannel("test", nil, nil, []string{"@bob:restricted"})
	if got := bc.ResolveAgentID("bob"); got != "restricted" {
		t.Errorf("expected %q, got %q", "restricted", got)
	}
}

func TestResolveAgentID_EmptyList(t *testing.T) {
	bc := NewBaseChannel("test", nil, nil, []string{})
	// Empty list allows everyone but no agent routing
	if got := bc.ResolveAgentID("anyone"); got != "" {
		t.Errorf("expected empty for empty allow list, got %q", got)
	}
}

// --- matchAllowEntry ---

func TestMatchAllowEntry_BackwardCompatibility(t *testing.T) {
	// Existing entries without agent suffix should keep working
	tests := []struct {
		name     string
		allow    []string
		sender   string
		wantOK   bool
		wantEntry string
	}{
		{"plain match", []string{"alice"}, "alice", true, "alice"},
		{"@prefix match", []string{"@alice"}, "alice", true, "@alice"},
		{"compound sender", []string{"12345"}, "12345|alice", true, "12345"},
		{"compound allow", []string{"12345|alice"}, "12345", true, "12345|alice"},
		{"no match", []string{"alice"}, "bob", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bc := NewBaseChannel("test", nil, nil, tt.allow)
			entry, ok := bc.matchAllowEntry(tt.sender)
			if ok != tt.wantOK {
				t.Errorf("matchAllowEntry(%q) ok=%v, want %v", tt.sender, ok, tt.wantOK)
			}
			if entry != tt.wantEntry {
				t.Errorf("matchAllowEntry(%q) entry=%q, want %q", tt.sender, entry, tt.wantEntry)
			}
		})
	}
}

func TestMatchAllowEntry_WithAgentSuffix(t *testing.T) {
	tests := []struct {
		name      string
		allow     []string
		sender    string
		wantOK    bool
		wantEntry string
	}{
		{"suffix match", []string{"bob:limited"}, "bob", true, "bob:limited"},
		{"@prefix+suffix", []string{"@bob:limited"}, "bob", true, "@bob:limited"},
		{"compound+suffix", []string{"12345|bob:limited"}, "12345|bob", true, "12345|bob:limited"},
		{"id-only match with compound+suffix", []string{"12345|bob:limited"}, "12345", true, "12345|bob:limited"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bc := NewBaseChannel("test", nil, nil, tt.allow)
			entry, ok := bc.matchAllowEntry(tt.sender)
			if ok != tt.wantOK {
				t.Errorf("matchAllowEntry(%q) ok=%v, want %v", tt.sender, ok, tt.wantOK)
			}
			if entry != tt.wantEntry {
				t.Errorf("matchAllowEntry(%q) entry=%q, want %q", tt.sender, entry, tt.wantEntry)
			}
		})
	}
}
