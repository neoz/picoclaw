package mcp

import (
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sipeed/picoclaw/pkg/config"
)

func makeTools(names ...string) []*sdkmcp.Tool {
	tools := make([]*sdkmcp.Tool, len(names))
	for i, name := range names {
		tools[i] = &sdkmcp.Tool{Name: name, Description: name + " tool"}
	}
	return tools
}

func TestFilterToolsAllowedOnly(t *testing.T) {
	tools := makeTools("read", "write", "delete", "list")
	cfg := config.MCPServerConfig{
		AllowedTools: []string{"read", "list"},
	}

	filtered := filterTools(tools, cfg)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(filtered))
	}
	names := map[string]bool{}
	for _, tool := range filtered {
		names[tool.Name] = true
	}
	if !names["read"] || !names["list"] {
		t.Errorf("expected read and list, got %v", names)
	}
}

func TestFilterToolsDeniedOnly(t *testing.T) {
	tools := makeTools("read", "write", "delete", "list")
	cfg := config.MCPServerConfig{
		DeniedTools: []string{"delete"},
	}

	filtered := filterTools(tools, cfg)
	if len(filtered) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(filtered))
	}
	for _, tool := range filtered {
		if tool.Name == "delete" {
			t.Error("delete should be filtered out")
		}
	}
}

func TestFilterToolsAllowedTakesPrecedence(t *testing.T) {
	tools := makeTools("read", "write", "delete")
	cfg := config.MCPServerConfig{
		AllowedTools: []string{"read", "write"},
		DeniedTools:  []string{"write"},
	}

	// allowed_tools takes precedence, so denied_tools is ignored
	filtered := filterTools(tools, cfg)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(filtered))
	}
	names := map[string]bool{}
	for _, tool := range filtered {
		names[tool.Name] = true
	}
	if !names["write"] {
		t.Error("write should be included (allowed_tools takes precedence)")
	}
}

func TestFilterToolsNoFilters(t *testing.T) {
	tools := makeTools("a", "b", "c")
	cfg := config.MCPServerConfig{}

	filtered := filterTools(tools, cfg)
	if len(filtered) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(filtered))
	}
}
