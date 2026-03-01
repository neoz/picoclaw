package tools

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// DelegateTool delegates tasks to other registered agents via the orchestrator pattern.
type DelegateTool struct {
	runner      DelegateRunner
	allowAgents []string
	mu          sync.Mutex
	channel     string
	chatID      string
}

func NewDelegateTool(runner DelegateRunner, allowAgents []string) *DelegateTool {
	return &DelegateTool{
		runner:      runner,
		allowAgents: allowAgents,
		channel:     "cli",
		chatID:      "direct",
	}
}

func (t *DelegateTool) Name() string {
	return "delegate"
}

func (t *DelegateTool) Description() string {
	agents := t.runner.ListAgents()

	// Filter to allowed agents only
	allowed := make(map[string]bool, len(t.allowAgents))
	for _, id := range t.allowAgents {
		allowed[id] = true
	}

	var parts []string
	for _, a := range agents {
		if !allowed[a.ID] {
			continue
		}
		line := fmt.Sprintf("  - %s (%s)", a.ID, a.Name)
		if a.Description != "" {
			line += ": " + a.Description
		}
		parts = append(parts, line)
	}

	desc := "Delegate a task to a specialist agent. You SHOULD use this tool whenever a user's request matches a specialist agent's expertise. The agent runs with its own model, tools, and context, then returns the result."
	if len(parts) > 0 {
		desc += "\n\nAvailable agents:\n" + strings.Join(parts, "\n")
	}
	return desc
}

func (t *DelegateTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"agent_id": map[string]interface{}{
				"type":        "string",
				"description": "ID of the target agent to delegate to",
			},
			"task": map[string]interface{}{
				"type":        "string",
				"description": "The task description for the target agent",
			},
			"mode": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"sync", "async"},
				"description": "sync: wait for result (default), async: run in background and report back later",
			},
			"context": map[string]interface{}{
				"type":        "string",
				"description": "Optional additional context to prepend to the task",
			},
		},
		"required": []string{"agent_id", "task"},
	}
}

func (t *DelegateTool) SetContext(channel, chatID string) {
	t.mu.Lock()
	t.channel = channel
	t.chatID = chatID
	t.mu.Unlock()
}

func (t *DelegateTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	agentID, ok := args["agent_id"].(string)
	if !ok || agentID == "" {
		return "", fmt.Errorf("agent_id is required")
	}

	task, ok := args["task"].(string)
	if !ok || task == "" {
		return "", fmt.Errorf("task is required")
	}

	// Validate against allowlist
	if !t.isAllowed(agentID) {
		return fmt.Sprintf("Error: agent %q is not in the allowed agents list", agentID), nil
	}

	// Prepend optional context
	if extra, ok := args["context"].(string); ok && extra != "" {
		task = extra + "\n\n" + task
	}

	mode, _ := args["mode"].(string)
	if mode == "" {
		mode = "sync"
	}

	t.mu.Lock()
	channel, chatID := t.channel, t.chatID
	t.mu.Unlock()

	switch mode {
	case "sync":
		return t.runner.RunDelegate(ctx, agentID, task, channel, chatID)
	case "async":
		label := fmt.Sprintf("delegate:%s", agentID)
		return t.runner.RunDelegateAsync(ctx, agentID, task, label, channel, chatID)
	default:
		return "", fmt.Errorf("invalid mode %q, must be sync or async", mode)
	}
}

func (t *DelegateTool) isAllowed(agentID string) bool {
	for _, id := range t.allowAgents {
		if id == agentID {
			return true
		}
	}
	return false
}
