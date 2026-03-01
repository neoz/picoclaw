package agent

import (
	"context"
	"sort"
	"testing"

	"github.com/sipeed/picoclaw/pkg/tools"
)

// stubTool is a minimal Tool implementation for testing.
type stubTool struct {
	name string
}

func (s *stubTool) Name() string                                                  { return s.name }
func (s *stubTool) Description() string                                           { return "" }
func (s *stubTool) Parameters() map[string]interface{}                             { return nil }
func (s *stubTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) { return "", nil }

func TestDeniedToolsFiltering(t *testing.T) {
	tests := []struct {
		name        string
		allTools    []string
		denied      []string
		wantTools   []string
	}{
		{
			name:      "no denied tools",
			allTools:  []string{"read_file", "write_file", "memory_store"},
			denied:    nil,
			wantTools: []string{"memory_store", "read_file", "write_file"},
		},
		{
			name:      "deny one tool",
			allTools:  []string{"read_file", "write_file", "memory_store"},
			denied:    []string{"memory_store"},
			wantTools: []string{"read_file", "write_file"},
		},
		{
			name:      "deny multiple tools",
			allTools:  []string{"read_file", "write_file", "exec", "memory_store", "memory_forget"},
			denied:    []string{"memory_store", "exec"},
			wantTools: []string{"memory_forget", "read_file", "write_file"},
		},
		{
			name:      "deny nonexistent tool",
			allTools:  []string{"read_file", "write_file"},
			denied:    []string{"nonexistent"},
			wantTools: []string{"read_file", "write_file"},
		},
		{
			name:      "deny all tools",
			allTools:  []string{"read_file", "write_file"},
			denied:    []string{"read_file", "write_file"},
			wantTools: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := tools.NewToolRegistry()

			// Replicate the exact filtering pattern from newAgentInstance
			deniedSet := make(map[string]struct{}, len(tt.denied))
			for _, name := range tt.denied {
				deniedSet[name] = struct{}{}
			}
			registerIfAllowed := func(tool tools.Tool) {
				if _, denied := deniedSet[tool.Name()]; !denied {
					registry.Register(tool)
				}
			}

			for _, name := range tt.allTools {
				registerIfAllowed(&stubTool{name: name})
			}

			got := registry.List()
			sort.Strings(got)
			sort.Strings(tt.wantTools)

			if len(got) != len(tt.wantTools) {
				t.Fatalf("got %v, want %v", got, tt.wantTools)
			}
			for i := range got {
				if got[i] != tt.wantTools[i] {
					t.Errorf("got[%d]=%q, want %q", i, got[i], tt.wantTools[i])
				}
			}
		})
	}
}
