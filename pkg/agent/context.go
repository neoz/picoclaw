package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/memory"
	"github.com/sipeed/picoclaw/pkg/providers"
	"github.com/sipeed/picoclaw/pkg/skills"
	"github.com/sipeed/picoclaw/pkg/tools"
)

type ContextBuilder struct {
	workspace    string
	skillsLoader *skills.SkillsLoader
	tools        *tools.ToolRegistry
	memoryDB     *memory.MemoryDB
	memoryCfg    *config.MemoryConfig
}

func getGlobalConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".picoclaw")
}

func NewContextBuilder(workspace string) *ContextBuilder {
	// builtin skills: skills directory in current project
	// Use the skills/ directory under the current working directory
	wd, _ := os.Getwd()
	builtinSkillsDir := filepath.Join(wd, "skills")
	globalSkillsDir := filepath.Join(getGlobalConfigDir(), "skills")

	return &ContextBuilder{
		workspace:    workspace,
		skillsLoader: skills.NewSkillsLoader(workspace, globalSkillsDir, builtinSkillsDir),
	}
}

// SetToolsRegistry sets the tools registry for dynamic tool summary generation.
func (cb *ContextBuilder) SetToolsRegistry(registry *tools.ToolRegistry) {
	cb.tools = registry
}

// SetMemoryDB sets the memory database and config for relevance-filtered context.
func (cb *ContextBuilder) SetMemoryDB(db *memory.MemoryDB, cfg *config.MemoryConfig) {
	cb.memoryDB = db
	cb.memoryCfg = cfg
}

func (cb *ContextBuilder) getIdentity() string {
	now := time.Now().Format("2006-01-02 15:04 (Monday)")
	workspacePath, _ := filepath.Abs(filepath.Join(cb.workspace))
	runtime := fmt.Sprintf("%s %s, Go %s", runtime.GOOS, runtime.GOARCH, runtime.Version())

	// Build tools section dynamically
	toolsSection := cb.buildToolsSection()

	return fmt.Sprintf(`# picoclaw

You are picoclaw, a helpful AI assistant.

## Current Time
%s

## Runtime
%s

## Workspace
Your workspace is at: %s
- Memory: SQLite database (use memory_store/memory_search/memory_forget tools)
- Skills: %s/skills/{skill-name}/SKILL.md

%s

## Important Rules

1. **ALWAYS use tools** - When you need to perform an action (schedule reminders, send messages, execute commands, etc.), you MUST call the appropriate tool. Do NOT just say you'll do it or pretend to do it.

2. **Be helpful and accurate** - When using tools, briefly explain what you're doing.

3. **Memory Management** - You have a persistent memory database that survives across sessions. Use it actively:

   **When to STORE (memory_store):**
   - User says "remember", "save this", "don't forget" -> store immediately
   - You learn user facts (name, location, timezone, birthday, job, preferences) -> store as "core"
   - A task is completed, a decision is made, or an important event happens -> store as "daily"
   - You want to save conversation context for short-term follow-up -> store as "conversation"
   - To UPDATE existing info, use the same key -- it overwrites the old content

   **When to SEARCH (memory_search):**
   - User asks "do you remember", "what did I say", "what was that" -> search first
   - User references past events, preferences, or previous conversations -> search first
   - Before answering any question that might have been discussed before -> search first
   - When unsure about user preferences or context -> search to check

   **When to FORGET (memory_forget):**
   - User says "forget this", "delete that", "that's wrong" -> delete the key
   - You stored something incorrect -> delete and re-store with correct content

   **Key naming:** Use descriptive, namespaced keys: "user_name", "user_timezone", "task_project_x_deadline", "pref_language"
   **Categories:** "core" = permanent, "daily" = 30 days, "conversation" = 7 days, "custom" = 90 days`,
		now, runtime, workspacePath, workspacePath, toolsSection)
}

func (cb *ContextBuilder) buildToolsSection() string {
	if cb.tools == nil {
		return ""
	}

	summaries := cb.tools.GetSummaries()
	if len(summaries) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Available Tools\n\n")
	sb.WriteString("**CRITICAL**: You MUST use tools to perform actions. Do NOT pretend to execute commands or schedule tasks.\n\n")
	sb.WriteString("You have access to the following tools:\n\n")
	for _, s := range summaries {
		sb.WriteString(s)
		sb.WriteString("\n")
	}

	return sb.String()
}

func (cb *ContextBuilder) BuildSystemPrompt() string {
	parts := []string{}

	// Core identity section
	parts = append(parts, cb.getIdentity())

	// Bootstrap files
	bootstrapContent := cb.LoadBootstrapFiles()
	if bootstrapContent != "" {
		parts = append(parts, bootstrapContent)
	}

	// Skills - show summary, AI can read full content with read_file tool
	skillsSummary := cb.skillsLoader.BuildSkillsSummary()
	if skillsSummary != "" {
		parts = append(parts, fmt.Sprintf(`# Skills

The following skills extend your capabilities. To use a skill, read its SKILL.md file using the read_file tool.

%s`, skillsSummary))
	}

	// Memory context is now injected per-message via buildRelevantMemoryContext()

	// Join with "---" separator
	return strings.Join(parts, "\n\n---\n\n")
}

func (cb *ContextBuilder) LoadBootstrapFiles() string {
	bootstrapFiles := []string{
		"AGENTS.md",
		"SOUL.md",
		"USER.md",
		"IDENTITY.md",
	}

	var result string
	for _, filename := range bootstrapFiles {
		filePath := filepath.Join(cb.workspace, filename)
		if data, err := os.ReadFile(filePath); err == nil {
			result += fmt.Sprintf("## %s\n\n%s\n\n", filename, string(data))
		}
	}

	return result
}

// buildRelevantMemoryContext returns memory context relevant to the user message.
func (cb *ContextBuilder) buildRelevantMemoryContext(userMessage string) string {
	if cb.memoryDB == nil {
		return ""
	}

	topK := 10
	minRelevance := 0.1
	if cb.memoryCfg != nil {
		if cb.memoryCfg.ContextTopK > 0 {
			topK = cb.memoryCfg.ContextTopK
		}
		if cb.memoryCfg.MinRelevance > 0 {
			minRelevance = cb.memoryCfg.MinRelevance
		}
	}

	// Collect recent core memories
	coreEntries, _ := cb.memoryDB.List("core", 20)
	seenKeys := make(map[string]bool)

	var parts []string

	if len(coreEntries) > 0 {
		var sb strings.Builder
		sb.WriteString("## Core Memories\n\n")
		for _, e := range coreEntries {
			seenKeys[e.Key] = true
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", e.Key, e.Content))
		}
		parts = append(parts, sb.String())
	}

	// FTS5 search for relevant memories
	if userMessage != "" {
		results, err := cb.memoryDB.Search(userMessage, topK)
		if err == nil && len(results) > 0 {
			var sb strings.Builder
			sb.WriteString("## Relevant Memories\n\n")
			added := 0
			for _, r := range results {
				// FTS5 rank is negative (lower = more relevant), filter by absolute value
				if r.Rank < -minRelevance || r.Rank == 0 {
					// Skip if already in core list
					if seenKeys[r.Entry.Key] {
						continue
					}
					seenKeys[r.Entry.Key] = true
					sb.WriteString(fmt.Sprintf("- [%s] (%s): %s\n", r.Entry.Key, r.Entry.Category, r.Entry.Content))
					added++
				}
			}
			if added > 0 {
				parts = append(parts, sb.String())
			}
		}
	}

	if len(parts) == 0 {
		return ""
	}

	return "# Memory\n\n" + strings.Join(parts, "\n")
}

func (cb *ContextBuilder) BuildMessages(history []providers.Message, summary string, currentMessage string, media []string, channel, chatID string) []providers.Message {
	messages := []providers.Message{}

	systemPrompt := cb.BuildSystemPrompt()

	// Append relevance-filtered memory context
	memoryContext := cb.buildRelevantMemoryContext(currentMessage)
	if memoryContext != "" {
		systemPrompt += "\n\n---\n\n" + memoryContext
	}

	// Add Current Session info if provided
	if channel != "" && chatID != "" {
		systemPrompt += fmt.Sprintf("\n\n## Current Session\nChannel: %s\nChat ID: %s", channel, chatID)
	}

	// Log system prompt summary for debugging (debug mode only)
	logger.DebugCF("agent", "System prompt built",
		map[string]interface{}{
			"total_chars": len(systemPrompt),
			"total_lines": strings.Count(systemPrompt, "\n") + 1,
			"section_count": strings.Count(systemPrompt, "\n\n---\n\n") + 1,
		})

	// Log preview of system prompt (avoid logging huge content)
	preview := systemPrompt
	if len(preview) > 500 {
		preview = preview[:500] + "... (truncated)"
	}
	logger.DebugCF("agent", "System prompt preview",
		map[string]interface{}{
			"preview": preview,
		})

	if summary != "" {
		systemPrompt += "\n\n## Summary of Previous Conversation\n\n" + summary
	}

	messages = append(messages, providers.Message{
		Role:    "system",
		Content: systemPrompt,
	})

	// Sanitize history to remove orphaned tool messages that would cause
	// "tool_call_id is not found" API errors (e.g. after TruncateHistory
	// slices in the middle of a tool call sequence).
	messages = append(messages, sanitizeHistory(history)...)

	messages = append(messages, providers.Message{
		Role:    "user",
		Content: currentMessage,
	})

	return messages
}

// sanitizeHistory removes orphaned tool-related messages from session history.
// It ensures every "tool" result message has a preceding "assistant" message
// with a matching tool call ID, and every "assistant" message with tool calls
// has all its tool results following it.
func sanitizeHistory(history []providers.Message) []providers.Message {
	if len(history) == 0 {
		return history
	}

	// Pass 1: collect valid tool_call_ids from assistant messages
	validIDs := make(map[string]bool)
	for _, msg := range history {
		if msg.Role == "assistant" {
			for _, tc := range msg.ToolCalls {
				if tc.ID != "" {
					validIDs[tc.ID] = true
				}
			}
		}
	}

	// Pass 2: filter out orphaned tool result messages
	result := make([]providers.Message, 0, len(history))
	for _, msg := range history {
		if msg.Role == "tool" {
			if msg.ToolCallID == "" || !validIDs[msg.ToolCallID] {
				continue
			}
		}
		result = append(result, msg)
	}

	// Pass 3: collect remaining tool result IDs
	answeredIDs := make(map[string]bool)
	for _, msg := range result {
		if msg.Role == "tool" && msg.ToolCallID != "" {
			answeredIDs[msg.ToolCallID] = true
		}
	}

	// Pass 4: remove assistant messages whose tool calls have no matching results
	final := make([]providers.Message, 0, len(result))
	for _, msg := range result {
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			allAnswered := true
			for _, tc := range msg.ToolCalls {
				if tc.ID != "" && !answeredIDs[tc.ID] {
					allAnswered = false
					break
				}
			}
			if !allAnswered {
				// Keep as plain assistant message without tool calls
				final = append(final, providers.Message{
					Role:    "assistant",
					Content: msg.Content,
				})
				continue
			}
		}
		final = append(final, msg)
	}

	return final
}

func (cb *ContextBuilder) AddToolResult(messages []providers.Message, toolCallID, toolName, result string) []providers.Message {
	messages = append(messages, providers.Message{
		Role:       "tool",
		Content:    result,
		ToolCallID: toolCallID,
	})
	return messages
}

func (cb *ContextBuilder) AddAssistantMessage(messages []providers.Message, content string, toolCalls []map[string]interface{}) []providers.Message {
	msg := providers.Message{
		Role:    "assistant",
		Content: content,
	}
	// Always add assistant message, whether or not it has tool calls
	messages = append(messages, msg)
	return messages
}

func (cb *ContextBuilder) loadSkills() string {
	allSkills := cb.skillsLoader.ListSkills()
	if len(allSkills) == 0 {
		return ""
	}

	var skillNames []string
	for _, s := range allSkills {
		skillNames = append(skillNames, s.Name)
	}

	content := cb.skillsLoader.LoadSkillsForContext(skillNames)
	if content == "" {
		return ""
	}

	return "# Skill Definitions\n\n" + content
}

// GetSkillsInfo returns information about loaded skills.
func (cb *ContextBuilder) GetSkillsInfo() map[string]interface{} {
	allSkills := cb.skillsLoader.ListSkills()
	skillNames := make([]string, 0, len(allSkills))
	for _, s := range allSkills {
		skillNames = append(skillNames, s.Name)
	}
	return map[string]interface{}{
		"total":     len(allSkills),
		"available": len(allSkills),
		"names":     skillNames,
	}
}
