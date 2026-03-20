package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPToolAdapter wraps an MCP server tool as a PicoClaw tools.Tool.
type MCPToolAdapter struct {
	client     *Client
	serverName string
	toolDef    *mcp.Tool
	prefixName string
}

// NewMCPToolAdapter creates an adapter that presents an MCP tool as a PicoClaw tool.
func NewMCPToolAdapter(client *Client, serverName string, toolDef *mcp.Tool) *MCPToolAdapter {
	return &MCPToolAdapter{
		client:     client,
		serverName: serverName,
		toolDef:    toolDef,
		prefixName: fmt.Sprintf("mcp_%s_%s", serverName, toolDef.Name),
	}
}

func (a *MCPToolAdapter) Name() string {
	return a.prefixName
}

func (a *MCPToolAdapter) Description() string {
	return fmt.Sprintf("[MCP:%s] %s", a.serverName, a.toolDef.Description)
}

func (a *MCPToolAdapter) Parameters() map[string]interface{} {
	if a.toolDef.InputSchema == nil {
		return map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}
	}
	// InputSchema from the client is a map[string]any (default JSON unmarshaling)
	if schema, ok := a.toolDef.InputSchema.(map[string]any); ok {
		return schema
	}
	// Fallback: empty object schema
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (a *MCPToolAdapter) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	return a.client.CallTool(ctx, a.toolDef.Name, args)
}
