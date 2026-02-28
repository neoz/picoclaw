package tools

import "context"

type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]interface{}
	Execute(ctx context.Context, args map[string]interface{}) (string, error)
}

// ContextualTool is an optional interface that tools can implement
// to receive the current message context (channel, chatID)
type ContextualTool interface {
	Tool
	SetContext(channel, chatID string)
}

// OwnerAwareTool is an optional interface that tools can implement
// to receive the current memory owner (username) for scoped access.
type OwnerAwareTool interface {
	Tool
	SetOwner(owner string)
}

// DelegateRunner is the interface that the agent loop implements to allow
// the delegate tool to invoke other agents without circular imports.
type DelegateRunner interface {
	RunDelegate(ctx context.Context, agentID, task, channel, chatID string) (string, error)
	RunDelegateAsync(ctx context.Context, agentID, task, label, channel, chatID string) (string, error)
	ListAgents() []AgentInfo
}

// AgentInfo holds basic metadata about an available agent.
type AgentInfo struct {
	ID          string
	Name        string
	Description string
}

func ToolToSchema(tool Tool) map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        tool.Name(),
			"description": tool.Description(),
			"parameters":  tool.Parameters(),
		},
	}
}
