package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/sipeed/picoclaw/pkg/session"
)

func newTestSessionManager(t *testing.T) *session.SessionManager {
	t.Helper()
	return session.NewSessionManager(t.TempDir())
}

func TestSessionMessagesTool_Name(t *testing.T) {
	sm := newTestSessionManager(t)
	tool := NewSessionMessagesTool(sm)
	if tool.Name() != "session_messages" {
		t.Errorf("expected %q, got %q", "session_messages", tool.Name())
	}
}

func TestSessionMessagesTool_ListEmpty(t *testing.T) {
	sm := newTestSessionManager(t)
	tool := NewSessionMessagesTool(sm)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "list",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != "No sessions found." {
		t.Errorf("expected no sessions message, got %q", result)
	}
}

func TestSessionMessagesTool_ListSessions(t *testing.T) {
	sm := newTestSessionManager(t)
	sm.AddToLog("telegram:111", "hello", "user1", "Alice")
	sm.AddToLog("discord:222", "world", "user2", "Bob")

	tool := NewSessionMessagesTool(sm)
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "list",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "telegram:111") {
		t.Errorf("expected telegram:111 in result, got %q", result)
	}
	if !strings.Contains(result, "discord:222") {
		t.Errorf("expected discord:222 in result, got %q", result)
	}
	if !strings.Contains(result, "2") {
		t.Errorf("expected count in result, got %q", result)
	}
}

func TestSessionMessagesTool_RecentMissingKey(t *testing.T) {
	sm := newTestSessionManager(t)
	tool := NewSessionMessagesTool(sm)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "recent",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Error") {
		t.Errorf("expected error for missing session_key, got %q", result)
	}
}

func TestSessionMessagesTool_RecentNoMessages(t *testing.T) {
	sm := newTestSessionManager(t)
	tool := NewSessionMessagesTool(sm)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":      "recent",
		"session_key": "telegram:999",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != "No recent messages found." {
		t.Errorf("expected no messages, got %q", result)
	}
}

func TestSessionMessagesTool_RecentReturnsMessages(t *testing.T) {
	sm := newTestSessionManager(t)
	sm.AddToLog("telegram:111", "first message", "u1", "Alice")
	sm.AddToLog("telegram:111", "second message", "u2", "Bob")

	tool := NewSessionMessagesTool(sm)
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":      "recent",
		"session_key": "telegram:111",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "first message") {
		t.Errorf("expected first message in result, got %q", result)
	}
	if !strings.Contains(result, "second message") {
		t.Errorf("expected second message in result, got %q", result)
	}
}

func TestSessionMessagesTool_RecentLimit(t *testing.T) {
	sm := newTestSessionManager(t)
	for i := 0; i < 5; i++ {
		sm.AddToLog("telegram:111", "msg"+string(rune('A'+i)), "u1", "Alice")
	}

	tool := NewSessionMessagesTool(sm)
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":      "recent",
		"session_key": "telegram:111",
		"limit":       float64(2),
	})
	if err != nil {
		t.Fatal(err)
	}
	// Should only have 2 messages (the last 2)
	count := strings.Count(result, "---")
	if count != 1 { // N-1 separators for N entries
		t.Errorf("expected 1 separator (2 messages), got %d separators", count)
	}
}

func TestSessionMessagesTool_RecentLimitCap(t *testing.T) {
	sm := newTestSessionManager(t)
	sm.AddToLog("telegram:111", "hello", "u1", "Alice")

	tool := NewSessionMessagesTool(sm)
	// limit > 50 should be capped to 50
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":      "recent",
		"session_key": "telegram:111",
		"limit":       float64(100),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "hello") {
		t.Errorf("expected message in result, got %q", result)
	}
}

func TestSessionMessagesTool_RecentSenderFilter(t *testing.T) {
	sm := newTestSessionManager(t)
	sm.AddToLog("telegram:111", "alice says hi", "u1", "Alice")
	sm.AddToLog("telegram:111", "bob says hi", "u2", "Bob")

	tool := NewSessionMessagesTool(sm)
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":      "recent",
		"session_key": "telegram:111",
		"sender_id":   "u1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "alice says hi") {
		t.Errorf("expected alice's message, got %q", result)
	}
	if strings.Contains(result, "bob says hi") {
		t.Errorf("should not contain bob's message, got %q", result)
	}
}

func TestSessionMessagesTool_SearchMissingKey(t *testing.T) {
	sm := newTestSessionManager(t)
	tool := NewSessionMessagesTool(sm)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "search",
		"query":  "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Error") && !strings.Contains(result, "session_key") {
		t.Errorf("expected error for missing session_key, got %q", result)
	}
}

func TestSessionMessagesTool_SearchMissingQuery(t *testing.T) {
	sm := newTestSessionManager(t)
	tool := NewSessionMessagesTool(sm)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":      "search",
		"session_key": "telegram:111",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Error") && !strings.Contains(result, "query") {
		t.Errorf("expected error for missing query, got %q", result)
	}
}

func TestSessionMessagesTool_SearchFindsMatch(t *testing.T) {
	sm := newTestSessionManager(t)
	sm.AddToLog("telegram:111", "the weather is sunny today", "u1", "Alice")
	sm.AddToLog("telegram:111", "I like pizza for lunch", "u2", "Bob")
	sm.AddToLog("telegram:111", "weather forecast says rain tomorrow", "u1", "Alice")

	tool := NewSessionMessagesTool(sm)
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":      "search",
		"session_key": "telegram:111",
		"query":       "weather",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "weather") {
		t.Errorf("expected weather messages in result, got %q", result)
	}
	if strings.Contains(result, "pizza") {
		t.Errorf("should not contain unrelated message, got %q", result)
	}
}

func TestSessionMessagesTool_SearchNoMatch(t *testing.T) {
	sm := newTestSessionManager(t)
	sm.AddToLog("telegram:111", "hello world", "u1", "Alice")

	tool := NewSessionMessagesTool(sm)
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":      "search",
		"session_key": "telegram:111",
		"query":       "xyznonexistent",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "No matching") {
		t.Errorf("expected no match message, got %q", result)
	}
}

func TestSessionMessagesTool_SearchEmptySession(t *testing.T) {
	sm := newTestSessionManager(t)
	tool := NewSessionMessagesTool(sm)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":      "search",
		"session_key": "telegram:999",
		"query":       "anything",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "No matching") {
		t.Errorf("expected no match message, got %q", result)
	}
}

func TestSessionMessagesTool_InvalidAction(t *testing.T) {
	sm := newTestSessionManager(t)
	tool := NewSessionMessagesTool(sm)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "invalid",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Error") {
		t.Errorf("expected error for invalid action, got %q", result)
	}
}

func TestSessionMessagesTool_CrossSessionAccess(t *testing.T) {
	sm := newTestSessionManager(t)
	sm.AddToLog("telegram:111", "telegram message", "u1", "Alice")
	sm.AddToLog("discord:222", "discord message", "u2", "Bob")

	tool := NewSessionMessagesTool(sm)

	// Read from telegram session
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":      "recent",
		"session_key": "telegram:111",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "telegram message") {
		t.Errorf("expected telegram message, got %q", result)
	}
	if strings.Contains(result, "discord message") {
		t.Errorf("should not contain discord message, got %q", result)
	}

	// Read from discord session
	result, err = tool.Execute(context.Background(), map[string]interface{}{
		"action":      "recent",
		"session_key": "discord:222",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "discord message") {
		t.Errorf("expected discord message, got %q", result)
	}
	if strings.Contains(result, "telegram message") {
		t.Errorf("should not contain telegram message, got %q", result)
	}
}
