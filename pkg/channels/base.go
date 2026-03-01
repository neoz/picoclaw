package channels

import (
	"context"
	"fmt"
	"sync/atomic"
	"strings"
	"github.com/sipeed/picoclaw/pkg/bus"
)

type Channel interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Send(ctx context.Context, msg bus.OutboundMessage) error
	IsRunning() bool
	IsAllowed(senderID string) bool
}

type BaseChannel struct {
	config    interface{}
	bus       *bus.MessageBus
	running   atomic.Bool
	name      string
	allowList []string
}

func NewBaseChannel(name string, config interface{}, bus *bus.MessageBus, allowList []string) *BaseChannel {
	return &BaseChannel{
		config:    config,
		bus:       bus,
		name:      name,
		allowList: allowList,
	}
}

func (c *BaseChannel) Name() string {
	return c.name
}

func (c *BaseChannel) IsRunning() bool {
	return c.running.Load()
}

func (c *BaseChannel) IsAllowed(senderID string) bool {
	if len(c.allowList) == 0 {
		return true
	}

	_, matched := c.matchAllowEntry(senderID)
	return matched
}

// ResolveAgentID returns the agent ID suffix from the matching allow_from entry,
// or empty string if no suffix or no match. Format: "user:agentID" -> "agentID".
func (c *BaseChannel) ResolveAgentID(senderID string) string {
	entry, matched := c.matchAllowEntry(senderID)
	if !matched {
		return ""
	}
	// Strip leading "@" and check for ":agentID" suffix
	trimmed := strings.TrimPrefix(entry, "@")
	if idx := strings.LastIndex(trimmed, ":"); idx > 0 {
		return trimmed[idx+1:]
	}
	return ""
}

// matchAllowEntry returns the raw allow_from entry that matched the senderID
// and whether a match was found.
func (c *BaseChannel) matchAllowEntry(senderID string) (string, bool) {
	// Extract parts from compound senderID like "123456|username"
	idPart := senderID
	userPart := ""
	if idx := strings.Index(senderID, "|"); idx > 0 {
		idPart = senderID[:idx]
		userPart = senderID[idx+1:]
	}

	for _, allowed := range c.allowList {
		// Strip leading "@" and agent suffix for matching
		trimmed := strings.TrimPrefix(allowed, "@")
		bare := trimmed
		if idx := strings.LastIndex(bare, ":"); idx > 0 {
			bare = bare[:idx]
		}

		allowedID := bare
		allowedUser := ""
		if idx := strings.Index(bare, "|"); idx > 0 {
			allowedID = bare[:idx]
			allowedUser = bare[idx+1:]
		}

		// Support either side using "id|username" compound form.
		// This keeps backward compatibility with legacy Telegram allowlist entries.
		if senderID == bare ||
			idPart == bare ||
			idPart == allowedID ||
			(allowedUser != "" && senderID == allowedUser) ||
			(userPart != "" && (userPart == bare || userPart == allowedID || userPart == allowedUser)) {
			return allowed, true
		}
	}

	return "", false
}

func (c *BaseChannel) HandleMessage(senderID, chatID, content string, media []string, metadata map[string]string) {
	if !c.IsAllowed(senderID) {
		return
	}

	// Route to specific agent if allow_from entry has ":agentID" suffix
	if agentID := c.ResolveAgentID(senderID); agentID != "" {
		if metadata == nil {
			metadata = map[string]string{}
		}
		metadata["agent_id"] = agentID
	}

	// Build session key: channel:chatID
	sessionKey := fmt.Sprintf("%s:%s", c.name, chatID)

	msg := bus.InboundMessage{
		Channel:    c.name,
		SenderID:   senderID,
		ChatID:     chatID,
		Content:    content,
		Media:      media,
		SessionKey: sessionKey,
		Metadata:   metadata,
	}

	c.bus.PublishInbound(msg)
}

func (c *BaseChannel) setRunning(running bool) {
	c.running.Store(running)
}
