package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/sipeed/picoclaw/pkg/session"
)

// STMTool provides message history access via SessionManager.
// Implements ContextualTool.
type STMTool struct {
	sessions *session.SessionManager
	channel  string
	chatID   string
}

func NewSTMTool(sessions *session.SessionManager) *STMTool {
	return &STMTool{sessions: sessions}
}

func (t *STMTool) Name() string {
	return "message_history"
}

func (t *STMTool) Description() string {
	return "Access recent messages from the current session. Actions: 'recent' returns last N messages (default 10, max 50), 'search' performs BM25-ranked search over recent messages. Use 'days' to narrow the time window (default 7, set 1 for last day). Use 'sender_id' to filter by a specific user."
}

func (t *STMTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"recent", "search"},
				"description": "Action to perform: 'recent' for last N messages, 'search' for BM25 search",
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
				"description": "Time window in days (default 7, set 1 for last day only)",
			},
			"sender_id": map[string]interface{}{
				"type":        "string",
				"description": "Filter messages by sender ID",
			},
		},
		"required": []string{"action"},
	}
}

func (t *STMTool) SetContext(channel, chatID string) {
	t.channel = channel
	t.chatID = chatID
}

func (t *STMTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	action, _ := args["action"].(string)
	sessionKey := fmt.Sprintf("%s:%s", t.channel, t.chatID)

	days := 7
	if d, ok := args["days"].(float64); ok && d > 0 {
		days = int(d)
	}
	senderID, _ := args["sender_id"].(string)

	switch action {
	case "recent":
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
		query, _ := args["query"].(string)
		if query == "" {
			return "Error: 'query' parameter is required for search action.", nil
		}
		entries := t.sessions.GetLog(sessionKey, days, senderID)
		if len(entries) == 0 {
			return "No matching messages found.", nil
		}
		// Build docs for BM25
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
		return "Error: action must be 'recent' or 'search'.", nil
	}
}

func formatLogEntries(entries []session.MessageLogEntry) string {
	var b strings.Builder
	for i, e := range entries {
		if i > 0 {
			b.WriteString("\n---\n")
		}
		sender := e.SenderID
		if e.SenderName != "" {
			sender = e.SenderName
		}
		b.WriteString(fmt.Sprintf("[%s] %s: %s", e.Timestamp.Format("2006-01-02 15:04"), sender, e.Content))
	}
	return b.String()
}
