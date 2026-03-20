package mcp

import (
	"context"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/tools"
)

// Manager manages MCP client connections for a single agent.
type Manager struct {
	agentID string
	clients map[string]*Client
}

// NewManager creates an MCP manager for the given agent.
func NewManager(agentID string) *Manager {
	return &Manager{
		agentID: agentID,
		clients: make(map[string]*Client),
	}
}

// Connect establishes connections to all configured MCP servers and returns adapted tools.
// Connection failures are logged as warnings but do not fail the overall operation.
func (m *Manager) Connect(ctx context.Context, servers map[string]config.MCPServerConfig) ([]tools.Tool, error) {
	var allTools []tools.Tool

	for name, serverCfg := range servers {
		client := NewClient(name, serverCfg)

		if err := client.Connect(ctx); err != nil {
			logger.WarnCF("mcp", "MCP server connection failed",
				map[string]interface{}{
					"agent":  m.agentID,
					"server": name,
					"error":  err.Error(),
				})
			continue
		}

		m.clients[name] = client

		serverTools := client.Tools()
		allowed := filterTools(serverTools, serverCfg)

		for _, t := range allowed {
			allTools = append(allTools, NewMCPToolAdapter(client, name, t))
		}

		logger.InfoCF("mcp", "MCP server connected",
			map[string]interface{}{
				"agent":      m.agentID,
				"server":     name,
				"tools":      len(allowed),
				"total":      len(serverTools),
			})
	}

	return allTools, nil
}

// Shutdown closes all MCP client connections.
func (m *Manager) Shutdown() {
	for name, client := range m.clients {
		if err := client.Close(); err != nil {
			logger.WarnCF("mcp", "MCP client close error",
				map[string]interface{}{
					"agent":  m.agentID,
					"server": name,
					"error":  err.Error(),
				})
		}
	}
	m.clients = nil
}

// filterTools applies per-server allowed_tools / denied_tools filtering.
// allowed_tools takes precedence: if set, only tools in the allowlist pass.
func filterTools(serverTools []*sdkmcp.Tool, cfg config.MCPServerConfig) []*sdkmcp.Tool {
	if len(cfg.AllowedTools) > 0 {
		allowSet := make(map[string]struct{}, len(cfg.AllowedTools))
		for _, name := range cfg.AllowedTools {
			allowSet[name] = struct{}{}
		}
		var filtered []*sdkmcp.Tool
		for _, t := range serverTools {
			if _, ok := allowSet[t.Name]; ok {
				filtered = append(filtered, t)
			}
		}
		return filtered
	}

	if len(cfg.DeniedTools) > 0 {
		denySet := make(map[string]struct{}, len(cfg.DeniedTools))
		for _, name := range cfg.DeniedTools {
			denySet[name] = struct{}{}
		}
		var filtered []*sdkmcp.Tool
		for _, t := range serverTools {
			if _, ok := denySet[t.Name]; !ok {
				filtered = append(filtered, t)
			}
		}
		return filtered
	}

	return serverTools
}
