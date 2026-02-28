// PicoClaw - Ultra-lightweight personal AI agent
// Inspired by and based on nanobot: https://github.com/HKUDS/nanobot
// License: MIT
//
// Copyright (c) 2026 PicoClaw contributors

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/cost"
	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/memory"
	"github.com/sipeed/picoclaw/pkg/providers"
	"github.com/sipeed/picoclaw/pkg/security"
	"github.com/sipeed/picoclaw/pkg/tools"
	"github.com/sipeed/picoclaw/pkg/utils"
)

type AgentLoop struct {
	bus         *bus.MessageBus
	cfg         *config.Config
	registry    *AgentRegistry
	running     atomic.Bool
	summarizing sync.Map
	memoryDB    *memory.MemoryDB
	memoryCfg    *config.MemoryConfig
	costTracker  *cost.CostTracker
	promptGuard       *security.PromptGuard
	leakDetector      *security.LeakDetector
	promptLeakGuards  sync.Map // agentID -> *security.PromptLeakDetector
}

// processOptions configures how a message is processed
type processOptions struct {
	SessionKey      string            // Session identifier for history/context
	Channel         string            // Target channel for tool execution
	ChatID          string            // Target chat ID for tool execution
	UserMessage     string            // User message content (may include prefix)
	DefaultResponse string            // Response when LLM returns empty
	EnableSummary   bool              // Whether to trigger summarization
	SendResponse    bool              // Whether to send response via bus
	Metadata        map[string]string // Original inbound message metadata
	Owner           string            // Memory owner (username for scoped access)
}

func NewAgentLoop(cfg *config.Config, msgBus *bus.MessageBus) (*AgentLoop, error) {
	workspace := cfg.WorkspacePath()
	os.MkdirAll(workspace, 0755)

	// Initialize shared memory database
	memDB, err := memory.Open(workspace)
	if err != nil {
		logger.ErrorCF("memory", "Failed to open memory database, continuing without memory",
			map[string]interface{}{"error": err.Error()})
	}

	if memDB != nil {
		// One-time migration from markdown files
		memoryDir := filepath.Join(workspace, "memory")
		if migErr := memDB.MigrateFromMarkdown(memoryDir); migErr != nil {
			logger.ErrorCF("memory", "Failed to migrate markdown memory",
				map[string]interface{}{"error": migErr.Error()})
		}

		// Run retention cleanup on startup
		retentionMap := map[string]int{
			"daily":        cfg.Memory.RetentionDays.Daily,
			"conversation": cfg.Memory.RetentionDays.Conversation,
			"custom":       cfg.Memory.RetentionDays.Custom,
		}
		if deleted, retErr := memDB.RunRetention(retentionMap); retErr != nil {
			logger.ErrorCF("memory", "Retention cleanup failed",
				map[string]interface{}{"error": retErr.Error()})
		} else if deleted > 0 {
			logger.InfoCF("memory", "Retention cleanup completed",
				map[string]interface{}{"deleted": deleted})
		}
	}

	// Initialize shared cost tracker
	var costTracker *cost.CostTracker
	if cfg.Cost.Enabled {
		var costErr error
		costTracker, costErr = cost.NewCostTracker(&cfg.Cost, workspace)
		if costErr != nil {
			logger.ErrorCF("cost", "Failed to initialize cost tracker, continuing without cost tracking",
				map[string]interface{}{"error": costErr.Error()})
		}
	}

	// Build shared tool instances
	shared := buildSharedTools(cfg, msgBus, memDB, costTracker, workspace)

	// Build agent registry
	registry := NewAgentRegistry()

	agentList := cfg.Agents.List
	if len(agentList) == 0 {
		// Synthesize implicit "main" agent from defaults
		agentList = []config.AgentConfig{{
			ID:      "main",
			Default: true,
		}}
	}

	for _, agentCfg := range agentList {
		inst, err := newAgentInstance(agentCfg, cfg, shared, memDB, &cfg.Memory, costTracker, msgBus)
		if err != nil {
			return nil, fmt.Errorf("failed to create agent %q: %w", agentCfg.ID, err)
		}
		registry.Register(inst)
		if agentCfg.Default {
			registry.SetDefault(agentCfg.ID)
		}
	}

	al := &AgentLoop{
		bus:         msgBus,
		cfg:         cfg,
		registry:    registry,
		summarizing: sync.Map{},
		memoryDB:    memDB,
		memoryCfg:   &cfg.Memory,
		costTracker: costTracker,
	}

	// Initialize security modules
	if cfg.Security.PromptGuard.Enabled {
		al.promptGuard = security.NewPromptGuard(cfg.Security.PromptGuard.Action, cfg.Security.PromptGuard.Sensitivity)
		logger.InfoCF("security", "Prompt guard enabled",
			map[string]interface{}{"action": cfg.Security.PromptGuard.Action, "sensitivity": cfg.Security.PromptGuard.Sensitivity})
	}
	if cfg.Security.LeakDetector.Enabled {
		al.leakDetector = security.NewLeakDetector(cfg.Security.LeakDetector.Sensitivity)
		logger.InfoCF("security", "Leak detector enabled",
			map[string]interface{}{"sensitivity": cfg.Security.LeakDetector.Sensitivity})
	}

	al.initDelegateTools()
	return al, nil
}

// buildSharedTools creates tool instances that are shared across all agents.
func buildSharedTools(cfg *config.Config, msgBus *bus.MessageBus, memDB *memory.MemoryDB, costTracker *cost.CostTracker, workspace string) *sharedTools {
	shared := &sharedTools{}

	// Web search / fetch tools
	ollamaAPIKey := cfg.Tools.Web.Ollama.APIKey
	if ollamaAPIKey != "" {
		shared.searchTool = tools.NewOllamaSearchTool(ollamaAPIKey, cfg.Tools.Web.Ollama.MaxResults)
		shared.fetchTool = tools.NewOllamaFetchTool(ollamaAPIKey)
	} else {
		braveAPIKey := cfg.Tools.Web.Search.APIKey
		if braveAPIKey != "" {
			shared.searchTool = tools.NewWebSearchTool(braveAPIKey, cfg.Tools.Web.Search.MaxResults)
		} else {
			shared.searchTool = tools.NewDuckDuckGoSearchTool(5)
		}
		shared.fetchTool = tools.NewWebFetchTool(50000)
	}

	// Message tool
	messageTool := tools.NewMessageTool()
	messageTool.SetSendCallback(func(channel, chatID, content string) error {
		msgBus.PublishOutbound(bus.OutboundMessage{
			Channel: channel,
			ChatID:  chatID,
			Content: content,
		})
		return nil
	})
	shared.messageTool = messageTool

	// Spawn tool (uses default provider -- will be created per first agent)
	// We use a deferred provider approach: create with nil, set later
	// For now, spawn needs a provider. We create one from defaults.
	defaultProvider, provErr := providers.CreateProvider(cfg)
	if provErr == nil {
		subagentManager := tools.NewSubagentManager(defaultProvider, workspace, msgBus)
		shared.spawnTool = tools.NewSpawnTool(subagentManager)
	}

	// Memory tools
	if memDB != nil {
		shared.memStore = tools.NewMemoryStoreTool(memDB)
		shared.memForget = tools.NewMemoryForgetTool(memDB)
		shared.memSearch = tools.NewMemorySearchTool(memDB)
	}

	// Cost tool
	shared.costTool = tools.NewCostSummaryTool(costTracker)

	return shared
}

func (al *AgentLoop) Run(ctx context.Context) error {
	al.running.Store(true)

	for al.running.Load() {
		select {
		case <-ctx.Done():
			return nil
		default:
			msg, ok := al.bus.ConsumeInbound(ctx)
			if !ok {
				continue
			}

			// Resolve agent for this message
			inst := al.resolveAgent(msg)

			senderName := msg.Metadata["username"]
			if senderName == "" {
				senderName = msg.Metadata["user_id"]
			}
			inst.Sessions.AddToLog(msg.SessionKey, msg.Content, msg.SenderID, senderName)

			if msg.Metadata["observe_only"] == "true" {
				continue
			}

			response, err := al.processMessage(ctx, inst, msg)
			if err != nil {
				logger.ErrorCF("agent", "Failed to process message", map[string]interface{}{
					"error":   err.Error(),
					"channel": msg.Channel,
					"chat_id": msg.ChatID,
				})
				response = "Something went wrong, please try again later."
			}

			if response != "" {
				al.bus.PublishOutbound(bus.OutboundMessage{
					Channel: msg.Channel,
					ChatID:  msg.ChatID,
					Content: response,
				})
			}
		}
	}

	return nil
}

// resolveAgent picks the agent instance for a message.
// Uses msg.Metadata["agent_id"] if set, otherwise the default agent.
func (al *AgentLoop) resolveAgent(msg bus.InboundMessage) *AgentInstance {
	if agentID := msg.Metadata["agent_id"]; agentID != "" {
		if inst, ok := al.registry.Get(agentID); ok {
			return inst
		}
	}
	return al.registry.GetDefault()
}

func (al *AgentLoop) Stop() {
	al.running.Store(false)
}

// Shutdown performs cleanup: optional snapshot export and closes the memory DB.
func (al *AgentLoop) Shutdown() {
	if al.memoryDB == nil {
		return
	}

	defaultInst := al.registry.GetDefault()
	workspace := al.cfg.WorkspacePath()
	if defaultInst != nil {
		workspace = defaultInst.Workspace
	}

	if al.memoryCfg != nil && al.memoryCfg.SnapshotOnExit {
		snapshotPath := filepath.Join(workspace, "memory", "MEMORY_SNAPSHOT.md")
		if err := al.memoryDB.ExportSnapshot(snapshotPath); err != nil {
			logger.ErrorCF("memory", "Failed to export snapshot on shutdown",
				map[string]interface{}{"error": err.Error()})
		} else {
			logger.InfoC("memory", "Memory snapshot exported on shutdown")
		}
	}

	if err := al.memoryDB.Close(); err != nil {
		logger.ErrorCF("memory", "Failed to close memory database",
			map[string]interface{}{"error": err.Error()})
	}
}

// RegisterTool registers a tool on the default agent's tool registry.
func (al *AgentLoop) RegisterTool(tool tools.Tool) {
	inst := al.registry.GetDefault()
	if inst != nil {
		inst.Tools.Register(tool)
	}
}

func (al *AgentLoop) ProcessDirect(ctx context.Context, content, sessionKey string) (string, error) {
	return al.ProcessDirectWithChannel(ctx, content, sessionKey, "cli", "direct")
}

func (al *AgentLoop) ProcessDirectWithChannel(ctx context.Context, content, sessionKey, channel, chatID string) (string, error) {
	inst := al.registry.GetDefault()

	msg := bus.InboundMessage{
		Channel:    channel,
		SenderID:   "cron",
		ChatID:     chatID,
		Content:    content,
		SessionKey: sessionKey,
	}

	return al.processMessage(ctx, inst, msg)
}

func (al *AgentLoop) processMessage(ctx context.Context, inst *AgentInstance, msg bus.InboundMessage) (string, error) {
	// Add message preview to log
	preview := utils.Truncate(msg.Content, 80)
	logger.InfoCF("agent", fmt.Sprintf("Processing message from %s:%s: %s", msg.Channel, msg.SenderID, preview),
		map[string]interface{}{
			"channel":     msg.Channel,
			"chat_id":     msg.ChatID,
			"sender_id":   msg.SenderID,
			"session_key": msg.SessionKey,
			"agent_id":    inst.ID,
		})

	// Route system messages to processSystemMessage
	if msg.Channel == "system" {
		return al.processSystemMessage(ctx, inst, msg)
	}

	// In group chats, prepend sender name so the LLM can distinguish users
	userMessage := msg.Content
	if isGroupMessage(msg.Metadata) {
		name := getSenderDisplayName(msg.Metadata)
		if name != "" {
			userMessage = fmt.Sprintf("[%s]: %s", name, userMessage)
		}
	}

	// Prompt guard: scan user input
	if al.promptGuard != nil {
		guardResult := al.promptGuard.Scan(userMessage)
		if !guardResult.Safe {
			logger.WarnCF("security", "Prompt injection detected in user input",
				map[string]interface{}{
					"patterns": guardResult.Patterns,
					"score":    guardResult.Score,
					"action":   string(guardResult.Action),
					"channel":  msg.Channel,
					"chat_id":  msg.ChatID,
				})
			if guardResult.Action == security.ActionBlock {
				return "Message blocked by security policy.", nil
			}
		}
	}

	// Process as user message
	return al.runAgentLoop(ctx, inst, processOptions{
		SessionKey:      msg.SessionKey,
		Channel:         msg.Channel,
		ChatID:          msg.ChatID,
		UserMessage:     userMessage,
		DefaultResponse: "I've completed processing but have no response to give.",
		EnableSummary:   true,
		SendResponse:    false,
		Metadata:        msg.Metadata,
		Owner:           resolveOwner(msg.Metadata),
	})
}

// isGroupMessage checks whether the inbound message comes from a group chat
// across all supported channels.
func isGroupMessage(meta map[string]string) bool {
	// Telegram: explicit is_group flag
	if meta["is_group"] == "true" {
		return true
	}
	// Discord: has guild_id means it's a server channel (not DM)
	if meta["is_dm"] == "false" && meta["guild_id"] != "" {
		return true
	}
	// QQ: group messages have group_id
	if meta["group_id"] != "" {
		return true
	}
	// DingTalk: conversation_type "2" is group
	if meta["conversation_type"] == "2" {
		return true
	}
	// Feishu: chat_type "group"
	if meta["chat_type"] == "group" {
		return true
	}
	return false
}

// getSenderDisplayName extracts the best available display name from message metadata.
func getSenderDisplayName(meta map[string]string) string {
	// Prefer username, then first_name (Telegram)
	if name := meta["username"]; name != "" {
		return name
	}
	if name := meta["first_name"]; name != "" {
		return name
	}
	// Discord: display_name
	if name := meta["display_name"]; name != "" {
		return name
	}
	// DingTalk: sender_name
	if name := meta["sender_name"]; name != "" {
		return name
	}
	return ""
}

// resolveOwner extracts the memory owner from message metadata.
// Prefers username, then user_id, falls back to "" (shared).
func resolveOwner(meta map[string]string) string {
	if name := meta["username"]; name != "" {
		return name
	}
	if uid := meta["user_id"]; uid != "" {
		return uid
	}
	return ""
}

func (al *AgentLoop) processSystemMessage(ctx context.Context, inst *AgentInstance, msg bus.InboundMessage) (string, error) {
	// Verify this is a system message
	if msg.Channel != "system" {
		return "", fmt.Errorf("processSystemMessage called with non-system message channel: %s", msg.Channel)
	}

	logger.InfoCF("agent", "Processing system message",
		map[string]interface{}{
			"sender_id": msg.SenderID,
			"chat_id":   msg.ChatID,
		})

	// Parse origin from chat_id (format: "channel:chat_id")
	var originChannel, originChatID string
	if idx := strings.Index(msg.ChatID, ":"); idx > 0 {
		originChannel = msg.ChatID[:idx]
		originChatID = msg.ChatID[idx+1:]
	} else {
		// Fallback
		originChannel = "cli"
		originChatID = msg.ChatID
	}

	// Use the origin session for context
	sessionKey := fmt.Sprintf("%s:%s", originChannel, originChatID)

	// Process as system message with routing back to origin
	return al.runAgentLoop(ctx, inst, processOptions{
		SessionKey:      sessionKey,
		Channel:         originChannel,
		ChatID:          originChatID,
		UserMessage:     fmt.Sprintf("[System: %s] %s", msg.SenderID, msg.Content),
		DefaultResponse: "Background task completed.",
		EnableSummary:   false,
		SendResponse:    true, // Send response back to original channel
	})
}

// runAgentLoop is the core message processing logic.
// It handles context building, LLM calls, tool execution, and response handling.
func (al *AgentLoop) runAgentLoop(ctx context.Context, inst *AgentInstance, opts processOptions) (string, error) {
	// 1. Update tool contexts
	al.updateToolContexts(inst, opts.Channel, opts.ChatID, opts.Owner)

	// 2. Build messages
	history := inst.Sessions.GetHistory(opts.SessionKey)
	summary := inst.Sessions.GetSummary(opts.SessionKey)
	messages := inst.ContextBuilder.BuildMessages(
		history,
		summary,
		opts.UserMessage,
		nil,
		opts.Channel,
		opts.ChatID,
		opts.Owner,
	)

	// 3. Save user message to session
	inst.Sessions.AddMessage(opts.SessionKey, "user", opts.UserMessage)

	// 4. Run LLM iteration loop
	finalContent, iteration, err := al.runLLMIteration(ctx, inst, messages, opts)
	if err != nil {
		return "", err
	}

	// 5. Handle empty response
	if finalContent == "" {
		finalContent = opts.DefaultResponse
	}

	// 5.5. Leak detector: scan outbound content
	if al.leakDetector != nil {
		leakResult := al.leakDetector.Scan(finalContent)
		if !leakResult.Clean {
			logger.WarnCF("security", "Credential leak detected in response",
				map[string]interface{}{
					"patterns":    leakResult.Patterns,
					"session_key": opts.SessionKey,
				})
			finalContent = leakResult.Redacted
		}
	}

	// 5.6. Prompt leak guard: detect system prompt content in output
	if al.cfg.Security.PromptLeakGuard.Enabled {
		plg := al.getPromptLeakGuard(inst)
		if plg != nil {
			plResult := plg.Scan(finalContent)
			if plResult.Leaked {
				logger.WarnCF("security", "System prompt leakage detected in response",
					map[string]interface{}{
						"matched":     plResult.MatchedCount,
						"total":       plResult.TotalPrints,
						"score":       plResult.Score,
						"action":      string(plResult.Action),
						"session_key": opts.SessionKey,
					})
				if plResult.Action == security.ActionBlock {
					finalContent = "I'm unable to share my system instructions."
				}
			}
		}
	}

	// 6. Save final assistant message to session
	inst.Sessions.AddMessage(opts.SessionKey, "assistant", finalContent)
	inst.Sessions.AddToLog(opts.SessionKey, finalContent, "assistant", "")
	inst.Sessions.Save(inst.Sessions.GetOrCreate(opts.SessionKey))

	// 7. Optional: summarization
	if opts.EnableSummary {
		al.maybeSummarize(inst, opts.SessionKey)
	}

	// 8. Optional: send response via bus
	if opts.SendResponse {
		al.bus.PublishOutbound(bus.OutboundMessage{
			Channel: opts.Channel,
			ChatID:  opts.ChatID,
			Content: finalContent,
		})
	}

	// 9. Log response
	responsePreview := utils.Truncate(finalContent, 120)
	logger.InfoCF("agent", fmt.Sprintf("Response: %s", responsePreview),
		map[string]interface{}{
			"session_key":  opts.SessionKey,
			"iterations":   iteration,
			"final_length": len(finalContent),
		})

	return finalContent, nil
}

// getPromptLeakGuard returns a cached PromptLeakDetector for the given agent instance.
// The detector is lazily created from the agent's stable system prompt (excluding
// per-message memory/session context) and cached in promptLeakGuards.
func (al *AgentLoop) getPromptLeakGuard(inst *AgentInstance) *security.PromptLeakDetector {
	if v, ok := al.promptLeakGuards.Load(inst.ID); ok {
		return v.(*security.PromptLeakDetector)
	}
	systemPrompt := inst.ContextBuilder.BuildSystemPrompt()
	plg := security.NewPromptLeakDetector(
		systemPrompt,
		al.cfg.Security.PromptLeakGuard.Threshold,
		al.cfg.Security.PromptLeakGuard.Action,
	)
	al.promptLeakGuards.Store(inst.ID, plg)
	logger.DebugCF("security", "Prompt leak guard initialized",
		map[string]interface{}{
			"agent_id":     inst.ID,
			"fingerprints": plg.FingerprintCount(),
		})
	return plg
}

// runLLMIteration executes the LLM call loop with tool handling.
// Returns the final content, iteration count, and any error.
func (al *AgentLoop) runLLMIteration(ctx context.Context, inst *AgentInstance, messages []providers.Message, opts processOptions) (string, int, error) {
	iteration := 0
	var finalContent string

	for iteration < inst.MaxIterations {
		iteration++

		logger.DebugCF("agent", "LLM iteration",
			map[string]interface{}{
				"iteration": iteration,
				"max":       inst.MaxIterations,
			})

		// Build tool definitions
		toolDefs := inst.Tools.GetDefinitions()
		providerToolDefs := make([]providers.ToolDefinition, 0, len(toolDefs))
		for _, td := range toolDefs {
			providerToolDefs = append(providerToolDefs, providers.ToolDefinition{
				Type: td["type"].(string),
				Function: providers.ToolFunctionDefinition{
					Name:        td["function"].(map[string]interface{})["name"].(string),
					Description: td["function"].(map[string]interface{})["description"].(string),
					Parameters:  td["function"].(map[string]interface{})["parameters"].(map[string]interface{}),
				},
			})
		}

		// Log LLM request details
		logger.DebugCF("agent", "LLM request",
			map[string]interface{}{
				"iteration":         iteration,
				"model":             inst.Model,
				"messages_count":    len(messages),
				"tools_count":       len(providerToolDefs),
				"max_tokens":        8192,
				"temperature":       inst.Temperature,
				"system_prompt_len": len(messages[0].Content),
			})

		// Log full messages (detailed)
		logger.DebugCF("agent", "Full LLM request",
			map[string]interface{}{
				"iteration":     iteration,
				"messages_json": formatMessagesForLog(messages),
				"tools_json":    formatToolsForLog(providerToolDefs),
			})

		// Budget check before LLM call
		if al.costTracker != nil {
			check := al.costTracker.CheckBudget(0)
			if check.Status == cost.BudgetExceeded {
				msg := fmt.Sprintf("Budget exceeded: $%.4f / $%.4f %s limit",
					check.CurrentUSD, check.LimitUSD, check.Period)
				logger.ErrorCF("cost", msg, nil)
				return msg, iteration, nil
			}
			if check.Status == cost.BudgetWarning {
				logger.WarnCF("cost", fmt.Sprintf("Budget warning: $%.4f / $%.4f %s limit",
					check.CurrentUSD, check.LimitUSD, check.Period), nil)
			}
		}

		// Call LLM
		response, err := inst.Provider.Chat(ctx, messages, providerToolDefs, inst.Model, map[string]interface{}{
			"max_tokens":  8192,
			"temperature": inst.Temperature,
		})

		if err != nil {
			logger.ErrorCF("agent", "LLM call failed",
				map[string]interface{}{
					"iteration": iteration,
					"error":     err.Error(),
				})
			return "", iteration, fmt.Errorf("LLM call failed: %w", err)
		}

		// Record usage after successful LLM call
		if al.costTracker != nil && response.Usage != nil {
			al.costTracker.RecordUsage(inst.Model, response.Usage.PromptTokens, response.Usage.CompletionTokens)
		}

		// Check if no tool calls - we're done
		if len(response.ToolCalls) == 0 {
			finalContent = response.Content
			logger.InfoCF("agent", "LLM response without tool calls (direct answer)",
				map[string]interface{}{
					"iteration":     iteration,
					"content_chars": len(finalContent),
				})
			break
		}

		// Log tool calls
		toolNames := make([]string, 0, len(response.ToolCalls))
		for _, tc := range response.ToolCalls {
			toolNames = append(toolNames, tc.Name)
		}
		logger.InfoCF("agent", "LLM requested tool calls",
			map[string]interface{}{
				"tools":     toolNames,
				"count":     len(toolNames),
				"iteration": iteration,
			})

		// React to sender message to indicate tool call activity
		if msgID := opts.Metadata["message_id"]; msgID != "" {
			al.bus.PublishOutbound(bus.OutboundMessage{
				Channel: opts.Channel,
				ChatID:  opts.ChatID,
				Metadata: map[string]string{
					"type":       "reaction",
					"message_id": msgID,
				},
			})
		}

		// Build assistant message with tool calls
		assistantMsg := providers.Message{
			Role:             "assistant",
			Content:          response.Content,
			ReasoningContent: response.ReasoningContent,
		}
		for _, tc := range response.ToolCalls {
			argumentsJSON, _ := json.Marshal(tc.Arguments)
			assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, providers.ToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: &providers.FunctionCall{
					Name:      tc.Name,
					Arguments: string(argumentsJSON),
				},
			})
		}
		messages = append(messages, assistantMsg)

		// Save assistant message with tool calls to session
		inst.Sessions.AddFullMessage(opts.SessionKey, assistantMsg)

		// Execute tool calls
		for _, tc := range response.ToolCalls {
			// Log tool call with arguments preview
			argsJSON, _ := json.Marshal(tc.Arguments)
			argsPreview := utils.Truncate(string(argsJSON), 200)
			logger.InfoCF("agent", fmt.Sprintf("Tool call: %s(%s)", tc.Name, argsPreview),
				map[string]interface{}{
					"tool":      tc.Name,
					"iteration": iteration,
				})

			result, err := inst.Tools.ExecuteWithContext(ctx, tc.Name, tc.Arguments, opts.Channel, opts.ChatID)
			if err != nil {
				result = fmt.Sprintf("Error: %v", err)
			}

			// Prompt guard: scan tool results for injection attempts
			if al.promptGuard != nil {
				toolGuard := al.promptGuard.Scan(result)
				if !toolGuard.Safe {
					logger.WarnCF("security", "Prompt injection detected in tool result",
						map[string]interface{}{
							"tool":     tc.Name,
							"patterns": toolGuard.Patterns,
							"score":    toolGuard.Score,
						})
				}
			}

			toolResultMsg := providers.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			}
			messages = append(messages, toolResultMsg)

			// Save tool result message to session
			inst.Sessions.AddFullMessage(opts.SessionKey, toolResultMsg)
		}
	}

	return finalContent, iteration, nil
}

// updateToolContexts updates the context for tools that need channel/chatID info.
func (al *AgentLoop) updateToolContexts(inst *AgentInstance, channel, chatID, owner string) {
	if tool, ok := inst.Tools.Get("message"); ok {
		if mt, ok := tool.(*tools.MessageTool); ok {
			mt.SetContext(channel, chatID)
		}
	}
	if tool, ok := inst.Tools.Get("spawn"); ok {
		if st, ok := tool.(*tools.SpawnTool); ok {
			st.SetContext(channel, chatID)
		}
	}
	if tool, ok := inst.Tools.Get("message_history"); ok {
		if st, ok := tool.(*tools.STMTool); ok {
			st.SetContext(channel, chatID)
		}
	}
	if tool, ok := inst.Tools.Get("delegate"); ok {
		if dt, ok := tool.(*tools.DelegateTool); ok {
			dt.SetContext(channel, chatID)
		}
	}
	// Set owner on memory tools for scoped access
	for _, name := range []string{"memory_store", "memory_search", "memory_forget"} {
		if tool, ok := inst.Tools.Get(name); ok {
			if ot, ok := tool.(tools.OwnerAwareTool); ok {
				ot.SetOwner(owner)
			}
		}
	}
}

// initDelegateTools creates and registers a DelegateTool on each agent that has
// subagents.allow_agents configured.
func (al *AgentLoop) initDelegateTools() {
	for _, inst := range al.registry.List() {
		if inst.Subagents == nil || len(inst.Subagents.AllowAgents) == 0 {
			continue
		}
		dt := tools.NewDelegateTool(al, inst.Subagents.AllowAgents)
		inst.Tools.Register(dt)
		logger.InfoCF("agent", fmt.Sprintf("Registered delegate tool on agent %q (targets: %v)", inst.ID, inst.Subagents.AllowAgents), nil)

		// Build subagent info for system prompt injection
		var subagentInfos []SubagentInfo
		for _, targetID := range inst.Subagents.AllowAgents {
			if target, ok := al.registry.Get(targetID); ok {
				subagentInfos = append(subagentInfos, SubagentInfo{
					ID:          target.ID,
					Name:        target.Name,
					Description: target.Description,
				})
			}
		}
		inst.ContextBuilder.SetSubagents(subagentInfos)
	}
}

// RunDelegate invokes a target agent's full LLM+tool loop synchronously.
func (al *AgentLoop) RunDelegate(ctx context.Context, agentID, task, channel, chatID string) (string, error) {
	inst, ok := al.registry.Get(agentID)
	if !ok {
		return "", fmt.Errorf("agent %q not found", agentID)
	}

	sessionKey := fmt.Sprintf("delegate:%s:%s:%d", agentID, chatID, time.Now().UnixMilli())

	return al.runAgentLoop(ctx, inst, processOptions{
		SessionKey:      sessionKey,
		Channel:         channel,
		ChatID:          chatID,
		UserMessage:     task,
		DefaultResponse: "Delegated task completed with no output.",
		EnableSummary:   false,
		SendResponse:    false,
	})
}

// RunDelegateAsync invokes a target agent in the background and publishes the
// result back via the message bus as a system message (same pattern as spawn).
func (al *AgentLoop) RunDelegateAsync(ctx context.Context, agentID, task, label, channel, chatID string) (string, error) {
	inst, ok := al.registry.Get(agentID)
	if !ok {
		return "", fmt.Errorf("agent %q not found", agentID)
	}

	go func() {
		sessionKey := fmt.Sprintf("delegate:%s:%s:%d", agentID, chatID, time.Now().UnixMilli())

		result, err := al.runAgentLoop(context.Background(), inst, processOptions{
			SessionKey:      sessionKey,
			Channel:         channel,
			ChatID:          chatID,
			UserMessage:     task,
			DefaultResponse: "Delegated task completed with no output.",
			EnableSummary:   false,
			SendResponse:    false,
		})

		content := result
		if err != nil {
			content = fmt.Sprintf("Delegate to %s failed: %v", agentID, err)
		}

		al.bus.PublishInbound(bus.InboundMessage{
			Channel:  "system",
			SenderID: fmt.Sprintf("delegate:%s", agentID),
			ChatID:   fmt.Sprintf("%s:%s", channel, chatID),
			Content:  fmt.Sprintf("Task '%s' completed.\n\nResult:\n%s", label, content),
		})
	}()

	return fmt.Sprintf("Delegated task to agent %q (async). Result will be reported when done.", agentID), nil
}

// ListAgents returns metadata for all registered agents.
func (al *AgentLoop) ListAgents() []tools.AgentInfo {
	agents := al.registry.List()
	infos := make([]tools.AgentInfo, 0, len(agents))
	for _, a := range agents {
		infos = append(infos, tools.AgentInfo{ID: a.ID, Name: a.Name, Description: a.Description})
	}
	return infos
}

// maybeSummarize triggers summarization if the session history exceeds thresholds.
func (al *AgentLoop) maybeSummarize(inst *AgentInstance, sessionKey string) {
	newHistory := inst.Sessions.GetHistory(sessionKey)
	tokenEstimate := al.estimateTokens(newHistory)
	threshold := inst.ContextWindow * 75 / 100

	if len(newHistory) > 20 || tokenEstimate > threshold {
		if _, loading := al.summarizing.LoadOrStore(sessionKey, true); !loading {
			go func() {
				defer al.summarizing.Delete(sessionKey)
				al.summarizeSession(inst, sessionKey)
			}()
		}
	}
}

// GetStartupInfo returns information about loaded tools and skills for logging.
func (al *AgentLoop) GetStartupInfo() map[string]interface{} {
	info := make(map[string]interface{})

	inst := al.registry.GetDefault()
	if inst == nil {
		return info
	}

	// Tools info
	toolNames := inst.Tools.List()
	info["tools"] = map[string]interface{}{
		"count": len(toolNames),
		"names": toolNames,
	}

	// Skills info
	info["skills"] = inst.ContextBuilder.GetSkillsInfo()

	// Agent count
	info["agents"] = map[string]interface{}{
		"count": al.registry.Count(),
		"ids":   al.registry.ListIDs(),
	}

	return info
}

// formatMessagesForLog formats messages for logging
func formatMessagesForLog(messages []providers.Message) string {
	if len(messages) == 0 {
		return "[]"
	}

	var result string
	result += "[\n"
	for i, msg := range messages {
		result += fmt.Sprintf("  [%d] Role: %s\n", i, msg.Role)
		if msg.ToolCalls != nil && len(msg.ToolCalls) > 0 {
			result += "  ToolCalls:\n"
			for _, tc := range msg.ToolCalls {
				result += fmt.Sprintf("    - ID: %s, Type: %s, Name: %s\n", tc.ID, tc.Type, tc.Name)
				if tc.Function != nil {
					result += fmt.Sprintf("      Arguments: %s\n", utils.Truncate(tc.Function.Arguments, 200))
				}
			}
		}
		if msg.Content != "" {
			content := utils.Truncate(msg.Content, 200)
			result += fmt.Sprintf("  Content: %s\n", content)
		}
		if msg.ToolCallID != "" {
			result += fmt.Sprintf("  ToolCallID: %s\n", msg.ToolCallID)
		}
		result += "\n"
	}
	result += "]"
	return result
}

// formatToolsForLog formats tool definitions for logging
func formatToolsForLog(toolDefs []providers.ToolDefinition) string {
	if len(toolDefs) == 0 {
		return "[]"
	}

	var result string
	result += "[\n"
	for i, tool := range toolDefs {
		result += fmt.Sprintf("  [%d] Type: %s, Name: %s\n", i, tool.Type, tool.Function.Name)
		result += fmt.Sprintf("      Description: %s\n", tool.Function.Description)
		if len(tool.Function.Parameters) > 0 {
			result += fmt.Sprintf("      Parameters: %s\n", utils.Truncate(fmt.Sprintf("%v", tool.Function.Parameters), 200))
		}
	}
	result += "]"
	return result
}

// summarizeSession summarizes the conversation history for a session.
func (al *AgentLoop) summarizeSession(inst *AgentInstance, sessionKey string) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	history := inst.Sessions.GetHistory(sessionKey)
	summary := inst.Sessions.GetSummary(sessionKey)

	// Keep last 4 messages for continuity
	if len(history) <= 4 {
		return
	}

	toSummarize := history[:len(history)-4]

	// Oversized Message Guard
	// Skip messages larger than 50% of context window to prevent summarizer overflow
	maxMessageTokens := inst.ContextWindow / 2
	validMessages := make([]providers.Message, 0)
	omitted := false

	for _, m := range toSummarize {
		if m.Role != "user" && m.Role != "assistant" {
			continue
		}
		// Estimate tokens for this message
		msgTokens := len(m.Content) / 4
		if msgTokens > maxMessageTokens {
			omitted = true
			continue
		}
		validMessages = append(validMessages, m)
	}

	if len(validMessages) == 0 {
		return
	}

	// Multi-Part Summarization
	// Split into two parts if history is significant
	var finalSummary string
	if len(validMessages) > 10 {
		mid := len(validMessages) / 2
		part1 := validMessages[:mid]
		part2 := validMessages[mid:]

		s1, _ := al.summarizeBatch(ctx, inst, part1, "")
		s2, _ := al.summarizeBatch(ctx, inst, part2, "")

		// Merge them
		mergePrompt := fmt.Sprintf("Merge these two conversation summaries into one cohesive summary:\n\n1: %s\n\n2: %s", s1, s2)
		resp, err := inst.Provider.Chat(ctx, []providers.Message{{Role: "user", Content: mergePrompt}}, nil, inst.Model, map[string]interface{}{
			"max_tokens":  1024,
			"temperature": 0.3,
		})
		if err == nil {
			finalSummary = resp.Content
		} else {
			finalSummary = s1 + " " + s2
		}
	} else {
		finalSummary, _ = al.summarizeBatch(ctx, inst, validMessages, summary)
	}

	if omitted && finalSummary != "" {
		finalSummary += "\n[Note: Some oversized messages were omitted from this summary for efficiency.]"
	}

	if finalSummary != "" {
		inst.Sessions.SetSummary(sessionKey, finalSummary)
		inst.Sessions.TruncateHistory(sessionKey, 4)
		inst.Sessions.Save(inst.Sessions.GetOrCreate(sessionKey))
	}
}

// summarizeBatch summarizes a batch of messages.
func (al *AgentLoop) summarizeBatch(ctx context.Context, inst *AgentInstance, batch []providers.Message, existingSummary string) (string, error) {
	prompt := "Provide a concise summary of this conversation segment, preserving core context and key points.\n"
	if existingSummary != "" {
		prompt += "Existing context: " + existingSummary + "\n"
	}
	prompt += "\nCONVERSATION:\n"
	for _, m := range batch {
		prompt += fmt.Sprintf("%s: %s\n", m.Role, m.Content)
	}

	response, err := inst.Provider.Chat(ctx, []providers.Message{{Role: "user", Content: prompt}}, nil, inst.Model, map[string]interface{}{
		"max_tokens":  1024,
		"temperature": 0.3,
	})
	if err != nil {
		return "", err
	}
	return response.Content, nil
}

// estimateTokens estimates the number of tokens in a message list.
func (al *AgentLoop) estimateTokens(messages []providers.Message) int {
	total := 0
	for _, m := range messages {
		total += len(m.Content) / 4 // Simple heuristic: 4 chars per token
	}
	return total
}
