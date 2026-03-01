package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/sipeed/picoclaw/pkg/session"
)

// SessionMessagesTool provides cross-session message access by explicit session key.
type SessionMessagesTool struct {
	sessions *session.SessionManager
}

func NewSessionMessagesTool(sessions *session.SessionManager) *SessionMessagesTool {
	return &SessionMessagesTool{sessions: sessions}
}

func (t *SessionMessagesTool) Name() string {
	return "session_messages"
}

func (t *SessionMessagesTool) Description() string {
	return "Access messages from any session by specifying a session key. Actions: 'list' returns available session keys, 'recent' returns last N messages from a session (default 10, max 50), 'search' performs BM25-ranked search over a session's messages. Use 'days' to narrow the time window (default 7). Use 'sender_id' to filter by a specific user."
}

func (t *SessionMessagesTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"list", "recent", "search"},
				"description": "Action to perform: 'list' available sessions, 'recent' for last N messages, 'search' for BM25 search",
			},
			"session_key": map[string]interface{}{
				"type":        "string",
				"description": "Target session key (e.g. 'telegram:12345'). Required for 'recent' and 'search' actions.",
			},
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query (required for 'search' action)",
			},
			"limit": map[string]interface{}{
				"type":        "number",
				"description": "Max messages to return for 'recent' action (default 10, max 50)",
			},
			"days": map[string]interface{}{
				"type":        "number",
				"description": "Time window in days (default 7)",
			},
			"sender_id": map[string]interface{}{
				"type":        "string",
				"description": "Filter messages by sender ID",
			},
		},
		"required": []string{"action"},
	}
}

func (t *SessionMessagesTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	action, _ := args["action"].(string)

	switch action {
	case "list":
		keys := t.sessions.ListSessionKeys()
		if len(keys) == 0 {
			return "No sessions found.", nil
		}
		return fmt.Sprintf("Available sessions (%d):\n%s", len(keys), strings.Join(keys, "\n")), nil

	case "recent":
		sessionKey, _ := args["session_key"].(string)
		if sessionKey == "" {
			return "Error: 'session_key' parameter is required for recent action.", nil
		}
		days := 7
		if d, ok := args["days"].(float64); ok && d > 0 {
			days = int(d)
		}
		senderID, _ := args["sender_id"].(string)
		limit := 10
		if l, ok := args["limit"].(float64); ok && l > 0 {
			limit = int(l)
		}
		if limit > 50 {
			limit = 50
		}
		entries := t.sessions.RecentLog(sessionKey, limit, days, senderID)
		if len(entries) == 0 {
			return "No recent messages found.", nil
		}
		return formatLogEntries(entries), nil

	case "search":
		sessionKey, _ := args["session_key"].(string)
		if sessionKey == "" {
			return "Error: 'session_key' parameter is required for search action.", nil
		}
		query, _ := args["query"].(string)
		if query == "" {
			return "Error: 'query' parameter is required for search action.", nil
		}
		days := 7
		if d, ok := args["days"].(float64); ok && d > 0 {
			days = int(d)
		}
		senderID, _ := args["sender_id"].(string)
		entries := t.sessions.GetLog(sessionKey, days, senderID)
		if len(entries) == 0 {
			return "No matching messages found.", nil
		}
		docs := make([]string, len(entries))
		for i, e := range entries {
			docs[i] = e.Content
		}
		indices := bm25Rank(docs, query, 10)
		if len(indices) == 0 {
			return "No matching messages found.", nil
		}
		ranked := make([]session.MessageLogEntry, len(indices))
		for i, idx := range indices {
			ranked[i] = entries[idx]
		}
		return formatLogEntries(ranked), nil

	default:
		return "Error: action must be 'list', 'recent', or 'search'.", nil
	}
}
