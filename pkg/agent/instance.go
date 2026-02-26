package agent

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/cost"
	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/memory"
	"github.com/sipeed/picoclaw/pkg/providers"
	"github.com/sipeed/picoclaw/pkg/session"
	"github.com/sipeed/picoclaw/pkg/tools"
)

// AgentInstance holds per-agent state: provider, sessions, context, tools.
type AgentInstance struct {
	ID             string
	Name           string
	Model          string
	Workspace      string
	MaxIterations  int
	MaxTokens      int
	Temperature    float64
	ContextWindow  int
	Provider       providers.LLMProvider
	Sessions       *session.SessionManager
	ContextBuilder *ContextBuilder
	Tools          *tools.ToolRegistry
	Subagents      *config.SubagentsConfig
	SkillsFilter   []string
}

// sharedTools holds tool instances that are shared across all agent instances.
type sharedTools struct {
	messageTool tools.Tool
	spawnTool   tools.Tool
	searchTool  tools.Tool
	fetchTool   tools.Tool
	memStore    tools.Tool
	memForget   tools.Tool
	memSearch   tools.Tool
	costTool    tools.Tool
	stmTool     tools.Tool
}

// newAgentInstance creates a new AgentInstance from an AgentConfig, falling back to defaults.
func newAgentInstance(
	agentCfg config.AgentConfig,
	cfg *config.Config,
	shared *sharedTools,
	memDB *memory.MemoryDB,
	memoryCfg *config.MemoryConfig,
	costTracker *cost.CostTracker,
	msgBus *bus.MessageBus,
) (*AgentInstance, error) {
	// Resolve values with fallback to defaults
	model := agentCfg.Model
	if model == "" {
		model = cfg.Agents.Defaults.Model
	}

	workspace := agentCfg.Workspace
	if workspace == "" {
		workspace = cfg.WorkspacePath()
	} else {
		workspace = expandWorkspacePath(workspace)
	}

	maxIterations := agentCfg.MaxToolIterations
	if maxIterations == 0 {
		maxIterations = cfg.Agents.Defaults.MaxToolIterations
	}

	maxTokens := agentCfg.MaxTokens
	if maxTokens == 0 {
		maxTokens = cfg.Agents.Defaults.MaxTokens
	}

	temperature := cfg.Agents.Defaults.Temperature
	if agentCfg.Temperature != nil {
		temperature = *agentCfg.Temperature
	}

	name := agentCfg.Name
	if name == "" {
		name = agentCfg.ID
	}

	// Create per-agent provider
	provider, err := providers.CreateProviderForModel(model, cfg)
	if err != nil {
		return nil, fmt.Errorf("agent %q: %w", agentCfg.ID, err)
	}

	// Ensure workspace exists
	os.MkdirAll(workspace, 0755)

	// Per-agent sessions
	sessionsManager := session.NewSessionManager(filepath.Join(workspace, "sessions"))

	// Per-agent tools registry
	toolsRegistry := tools.NewToolRegistry()

	// Build denied tools set for filtering
	deniedSet := make(map[string]struct{}, len(agentCfg.DeniedTools))
	for _, name := range agentCfg.DeniedTools {
		deniedSet[name] = struct{}{}
	}
	registerIfAllowed := func(t tools.Tool) {
		if _, denied := deniedSet[t.Name()]; !denied {
			toolsRegistry.Register(t)
		}
	}

	// Workspace-scoped tools
	allowedDir := workspace
	if !cfg.IsRestrictToWorkspace() {
		allowedDir = ""
	}
	registerIfAllowed(tools.NewReadFileTool(allowedDir))
	registerIfAllowed(tools.NewWriteFileTool(allowedDir))
	registerIfAllowed(tools.NewListDirTool(allowedDir))
	execTool := tools.NewExecTool(workspace)
	execTool.SetRestrictToWorkspace(cfg.IsRestrictToWorkspace())
	registerIfAllowed(execTool)
	registerIfAllowed(tools.NewEditFileTool(allowedDir))

	// Register shared tools
	if shared.searchTool != nil {
		registerIfAllowed(shared.searchTool)
	}
	if shared.fetchTool != nil {
		registerIfAllowed(shared.fetchTool)
	}
	if shared.messageTool != nil {
		registerIfAllowed(shared.messageTool)
	}
	if shared.spawnTool != nil {
		registerIfAllowed(shared.spawnTool)
	}
	if shared.memStore != nil {
		registerIfAllowed(shared.memStore)
	}
	if shared.memForget != nil {
		registerIfAllowed(shared.memForget)
	}
	if shared.memSearch != nil {
		registerIfAllowed(shared.memSearch)
	}
	if shared.costTool != nil {
		registerIfAllowed(shared.costTool)
	}

	// Per-agent STM tool (backed by this agent's session manager)
	registerIfAllowed(tools.NewSTMTool(sessionsManager))
	registerIfAllowed(tools.NewSessionMessagesTool(sessionsManager))

	// Context builder
	contextBuilder := NewContextBuilder(workspace)
	
	if memDB != nil {
		contextBuilder.SetMemoryDB(memDB, memoryCfg)
	}

	logger.InfoCF("agent", fmt.Sprintf("Agent instance created: %s (model=%s)", agentCfg.ID, model),
		map[string]interface{}{
			"agent_id":  agentCfg.ID,
			"model":     model,
			"workspace": workspace,
		})

	return &AgentInstance{
		ID:             agentCfg.ID,
		Name:           name,
		Model:          model,
		Workspace:      workspace,
		MaxIterations:  maxIterations,
		MaxTokens:      maxTokens,
		Temperature:    temperature,
		ContextWindow:  maxTokens,
		Provider:       provider,
		Sessions:       sessionsManager,
		ContextBuilder: contextBuilder,
		Tools:          toolsRegistry,
		Subagents:      agentCfg.Subagents,
		SkillsFilter:   agentCfg.Skills,
	}, nil
}

// expandWorkspacePath handles ~ expansion for workspace paths.
func expandWorkspacePath(path string) string {
	if path == "" {
		return path
	}
	if path[0] == '~' {
		home, _ := os.UserHomeDir()
		if len(path) > 1 && path[1] == '/' {
			return home + path[1:]
		}
		return home
	}
	return path
}
