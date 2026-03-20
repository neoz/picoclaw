package mcp

import (
	"context"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sipeed/picoclaw/pkg/config"
)

func TestMCPToolAdapterName(t *testing.T) {
	client := NewClient("myserver", config.MCPServerConfig{})
	toolDef := &sdkmcp.Tool{Name: "read_file", Description: "Read a file"}
	adapter := NewMCPToolAdapter(client, "myserver", toolDef)

	if got := adapter.Name(); got != "mcp_myserver_read_file" {
		t.Errorf("Name() = %q, want %q", got, "mcp_myserver_read_file")
	}
}

func TestMCPToolAdapterDescription(t *testing.T) {
	client := NewClient("fs", config.MCPServerConfig{})
	toolDef := &sdkmcp.Tool{Name: "list_dir", Description: "List directory contents"}
	adapter := NewMCPToolAdapter(client, "fs", toolDef)

	want := "[MCP:fs] List directory contents"
	if got := adapter.Description(); got != want {
		t.Errorf("Description() = %q, want %q", got, want)
	}
}

func TestMCPToolAdapterParametersFromMap(t *testing.T) {
	client := NewClient("test", config.MCPServerConfig{})
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string"},
		},
		"required": []any{"path"},
	}
	toolDef := &sdkmcp.Tool{Name: "read", InputSchema: schema}
	adapter := NewMCPToolAdapter(client, "test", toolDef)

	params := adapter.Parameters()
	if params["type"] != "object" {
		t.Errorf("Parameters() type = %v, want object", params["type"])
	}
	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("Parameters() properties not a map")
	}
	if _, ok := props["path"]; !ok {
		t.Error("Parameters() missing 'path' property")
	}
}

func TestMCPToolAdapterParametersNilSchema(t *testing.T) {
	client := NewClient("test", config.MCPServerConfig{})
	toolDef := &sdkmcp.Tool{Name: "ping", InputSchema: nil}
	adapter := NewMCPToolAdapter(client, "test", toolDef)

	params := adapter.Parameters()
	if params["type"] != "object" {
		t.Errorf("Parameters() type = %v, want object", params["type"])
	}
}

func TestMCPToolAdapterExecuteNoConnection(t *testing.T) {
	client := NewClient("test", config.MCPServerConfig{Transport: "stdio"})
	toolDef := &sdkmcp.Tool{Name: "ping"}
	adapter := NewMCPToolAdapter(client, "test", toolDef)

	_, err := adapter.Execute(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("Execute() with no connection should return error")
	}
}
