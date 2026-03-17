package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sipeed/picoclaw/pkg/media"
)

type SendCallback func(channel, chatID, content string, media []string) error

type MessageTool struct {
	sendCallback   SendCallback
	mu             sync.Mutex
	defaultChannel string
	defaultChatID  string
	workspace      string
}

func NewMessageTool() *MessageTool {
	return &MessageTool{}
}

func (t *MessageTool) Name() string {
	return "message"
}

func (t *MessageTool) Description() string {
	return "Send a message to a chat channel."
}

func (t *MessageTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type":        "string",
				"description": "The message content to send",
			},
			"channel": map[string]interface{}{
				"type":        "string",
				"description": "Optional: target channel (telegram, whatsapp, etc.)",
			},
			"chat_id": map[string]interface{}{
				"type":        "string",
				"description": "Optional: target chat/user ID",
			},
			"media": map[string]interface{}{
				"type":        "array",
				"description": "Optional: list of file paths to send (images, stickers). Files must be in workspace or temp directory.",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
		},
		"required": []string{"content"},
	}
}

func (t *MessageTool) SetContext(channel, chatID string) {
	t.mu.Lock()
	t.defaultChannel = channel
	t.defaultChatID = chatID
	t.mu.Unlock()
}

func (t *MessageTool) SetSendCallback(callback SendCallback) {
	t.sendCallback = callback
}

func (t *MessageTool) SetWorkspace(workspace string) {
	t.mu.Lock()
	t.workspace = workspace
	t.mu.Unlock()
}

func (t *MessageTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	content, ok := args["content"].(string)
	if !ok {
		return "", fmt.Errorf("content is required")
	}

	channel, _ := args["channel"].(string)
	chatID, _ := args["chat_id"].(string)

	var media []string
	if rawMedia, ok := args["media"].([]interface{}); ok {
		for _, m := range rawMedia {
			if s, ok := m.(string); ok {
				media = append(media, s)
			}
		}
	}

	t.mu.Lock()
	if channel == "" {
		channel = t.defaultChannel
	}
	if chatID == "" {
		chatID = t.defaultChatID
	}
	t.mu.Unlock()

	if channel == "" || chatID == "" {
		return "Error: No target channel/chat specified", nil
	}

	if t.sendCallback == nil {
		return "Error: Message sending not configured", nil
	}

	// Validate media paths at the tool layer before reaching any channel.
	// Only allow files under the media temp dir or workspace to prevent
	// exfiltration of arbitrary files via /tmp staging.
	for _, path := range media {
		if !isAllowedMediaPath(path, t.workspace) {
			return fmt.Sprintf("Error: media path not allowed: %s", path), nil
		}
	}

	if err := t.sendCallback(channel, chatID, content, media); err != nil {
		return fmt.Sprintf("Error sending message: %v", err), nil
	}

	return fmt.Sprintf("Message sent to %s:%s", channel, chatID), nil
}

// isAllowedMediaPath validates that a media path is under the media temp dir
// or workspace. Unlike media.IsAllowedPath, this does NOT allow all of /tmp
// to prevent the exfiltration chain: write_file to /tmp -> message with media.
func isAllowedMediaPath(path, workspace string) bool {
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) {
		abs, err := filepath.Abs(cleaned)
		if err != nil {
			return false
		}
		cleaned = abs
	}

	isUnder := func(dir string) bool {
		d := filepath.Clean(dir)
		return strings.HasPrefix(cleaned, d+string(os.PathSeparator)) || cleaned == d
	}

	if isUnder(media.TempDir()) {
		return true
	}
	if workspace != "" {
		return isUnder(workspace)
	}
	return false
}
