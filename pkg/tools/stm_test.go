package tools

import (
	"strings"
	"testing"
	"time"

	"github.com/sipeed/picoclaw/pkg/session"
)

// --- normalizeReplyFormat ---

func TestNormalizeReplyFormat_NoReply(t *testing.T) {
	input := "just a normal message"
	got := normalizeReplyFormat(input)
	if got != input {
		t.Errorf("expected unchanged, got %q", got)
	}
}

func TestNormalizeReplyFormat_OldSingleLine(t *testing.T) {
	input := "[reply to alice: hello world]\nmy response"
	got := normalizeReplyFormat(input)
	want := "(replying to alice):\n> hello world\nmy response"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNormalizeReplyFormat_OldMultiLine(t *testing.T) {
	input := "[reply to bob: line one\nline two]\nmy reply"
	got := normalizeReplyFormat(input)
	if !strings.Contains(got, "(replying to bob):") {
		t.Errorf("expected replying to bob header, got %q", got)
	}
	if !strings.Contains(got, "> line one\n") {
		t.Errorf("expected blockquoted first line, got %q", got)
	}
	if !strings.Contains(got, "> line two\n") {
		t.Errorf("expected blockquoted second line, got %q", got)
	}
	if !strings.HasSuffix(got, "my reply") {
		t.Errorf("expected actual content at end, got %q", got)
	}
}

func TestNormalizeReplyFormat_OldUserIDFallback(t *testing.T) {
	input := "[reply to userid-12345: some text]\nresponse"
	got := normalizeReplyFormat(input)
	want := "(replying to userid-12345):\n> some text\nresponse"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNormalizeReplyFormat_NewFormatPassthrough(t *testing.T) {
	// New format should not be modified
	input := "(replying to alice):\n> hello world\nmy response"
	got := normalizeReplyFormat(input)
	if got != input {
		t.Errorf("expected new format unchanged, got %q", got)
	}
}

func TestNormalizeReplyFormat_BracketNotReply(t *testing.T) {
	// Brackets that don't match the reply pattern should pass through
	input := "[file: document.pdf]\nsome content"
	got := normalizeReplyFormat(input)
	if got != input {
		t.Errorf("expected unchanged, got %q", got)
	}
}

func TestNormalizeReplyFormat_EmptyContent(t *testing.T) {
	got := normalizeReplyFormat("")
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestNormalizeReplyFormat_OldReplyOnly(t *testing.T) {
	// Old reply with no content after it
	input := "[reply to alice: quoted text]\n"
	got := normalizeReplyFormat(input)
	want := "(replying to alice):\n> quoted text\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// --- formatLogEntries with reply normalization ---

func TestFormatLogEntries_NormalizesOldReply(t *testing.T) {
	entries := []session.MessageLogEntry{
		{
			Timestamp:  time.Date(2025, 3, 1, 14, 30, 0, 0, time.UTC),
			SenderID:   "u1",
			SenderName: "Bob",
			Content:    "[reply to alice: hello]\nmy response",
		},
	}
	got := formatLogEntries(entries)
	if strings.Contains(got, "[reply to") {
		t.Errorf("old reply format should be normalized, got %q", got)
	}
	if !strings.Contains(got, "(replying to alice):") {
		t.Errorf("expected new reply header, got %q", got)
	}
	if !strings.Contains(got, "> hello") {
		t.Errorf("expected blockquoted text, got %q", got)
	}
	if !strings.Contains(got, "my response") {
		t.Errorf("expected actual content, got %q", got)
	}
}

func TestFormatLogEntries_NewFormatUnchanged(t *testing.T) {
	entries := []session.MessageLogEntry{
		{
			Timestamp:  time.Date(2025, 3, 1, 14, 30, 0, 0, time.UTC),
			SenderID:   "u1",
			SenderName: "Alice",
			Content:    "(replying to bob):\n> hey there\nmy reply",
		},
	}
	got := formatLogEntries(entries)
	if !strings.Contains(got, "(replying to bob):") {
		t.Errorf("expected new format preserved, got %q", got)
	}
	if !strings.Contains(got, "> hey there") {
		t.Errorf("expected blockquote preserved, got %q", got)
	}
}

func TestFormatLogEntries_PlainMessageUnchanged(t *testing.T) {
	entries := []session.MessageLogEntry{
		{
			Timestamp:  time.Date(2025, 3, 1, 14, 30, 0, 0, time.UTC),
			SenderID:   "u1",
			SenderName: "Alice",
			Content:    "just a normal message",
		},
	}
	got := formatLogEntries(entries)
	if !strings.Contains(got, "just a normal message") {
		t.Errorf("expected plain content, got %q", got)
	}
}

func TestFormatLogEntries_MixedEntries(t *testing.T) {
	entries := []session.MessageLogEntry{
		{
			Timestamp:  time.Date(2025, 3, 1, 14, 0, 0, 0, time.UTC),
			SenderID:   "u1",
			SenderName: "Alice",
			Content:    "plain message",
		},
		{
			Timestamp:  time.Date(2025, 3, 1, 14, 5, 0, 0, time.UTC),
			SenderID:   "u2",
			SenderName: "Bob",
			Content:    "[reply to alice: plain message]\nmy take on it",
		},
		{
			Timestamp:  time.Date(2025, 3, 1, 14, 10, 0, 0, time.UTC),
			SenderID:   "u1",
			SenderName: "Alice",
			Content:    "(replying to bob):\n> my take on it\nagreed",
		},
	}
	got := formatLogEntries(entries)

	// Should have 2 separators for 3 entries
	if strings.Count(got, "---") != 2 {
		t.Errorf("expected 2 separators, got %d in %q", strings.Count(got, "---"), got)
	}
	// Old format should be normalized
	if strings.Contains(got, "[reply to") {
		t.Errorf("old format should be normalized, got %q", got)
	}
	// Both reply entries should use new format
	if strings.Count(got, "(replying to") != 2 {
		t.Errorf("expected 2 replying-to headers, got %q", got)
	}
}
