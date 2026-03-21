package agent

import (
	"strings"
	"testing"

	"github.com/sipeed/picoclaw/pkg/memory"
	"github.com/sipeed/picoclaw/pkg/providers"
)

// newTestContextBuilder creates a ContextBuilder with a temp workspace (no real files).
func newTestContextBuilder(t *testing.T) *ContextBuilder {
	t.Helper()
	return NewContextBuilder(t.TempDir())
}

func TestBuildSystemPrompt_NoInstructions_FullPrompt(t *testing.T) {
	cb := newTestContextBuilder(t)
	prompt := cb.BuildSystemPrompt()

	// Full prompt must contain identity section
	if !strings.Contains(prompt, "# picoclaw") {
		t.Error("full prompt missing identity section")
	}
	// Full prompt must contain safety section
	if !strings.Contains(prompt, "## Safety") {
		t.Error("full prompt missing safety section")
	}
}

func TestBuildSystemPrompt_InstructionsOnly(t *testing.T) {
	cb := newTestContextBuilder(t)
	cb.SetInstructions("You are a poet.", nil)
	prompt := cb.BuildSystemPrompt()

	if !strings.Contains(prompt, "You are a poet.") {
		t.Error("prompt missing instructions text")
	}
	// Should NOT contain identity section (not opted in)
	if strings.Contains(prompt, "# picoclaw") {
		t.Error("lightweight prompt should not contain identity section")
	}
	// Safety is always included regardless of context sections
	if !strings.Contains(prompt, "## Safety") {
		t.Error("lightweight prompt should always contain safety section")
	}
}

func TestBuildSystemPrompt_InstructionsWithSafety(t *testing.T) {
	cb := newTestContextBuilder(t)
	cb.SetInstructions("You are a poet.", []string{"safety"})
	prompt := cb.BuildSystemPrompt()

	if !strings.Contains(prompt, "You are a poet.") {
		t.Error("prompt missing instructions text")
	}
	if !strings.Contains(prompt, "## Safety") {
		t.Error("prompt should include safety section")
	}
	if strings.Contains(prompt, "# picoclaw") {
		t.Error("prompt should not include identity section")
	}
}

func TestBuildSystemPrompt_InstructionsWithIdentity(t *testing.T) {
	cb := newTestContextBuilder(t)
	cb.SetInstructions("You are a poet.", []string{"identity"})
	prompt := cb.BuildSystemPrompt()

	if !strings.Contains(prompt, "You are a poet.") {
		t.Error("prompt missing instructions text")
	}
	if !strings.Contains(prompt, "# picoclaw") {
		t.Error("prompt should include identity section")
	}
	// Safety is always included regardless of context sections
	if !strings.Contains(prompt, "## Safety") {
		t.Error("prompt should always include safety section")
	}
}

func TestBuildSystemPrompt_InstructionsWithAllSections(t *testing.T) {
	cb := newTestContextBuilder(t)
	cb.SetInstructions("You are a poet.", []string{"identity", "bootstrap", "safety", "skills", "memory"})
	prompt := cb.BuildSystemPrompt()

	if !strings.Contains(prompt, "You are a poet.") {
		t.Error("prompt missing instructions text")
	}
	if !strings.Contains(prompt, "# picoclaw") {
		t.Error("prompt should include identity section")
	}
	if !strings.Contains(prompt, "## Safety") {
		t.Error("prompt should include safety section")
	}
}

func TestBuildSystemPrompt_DelegationAlwaysIncluded(t *testing.T) {
	cb := newTestContextBuilder(t)
	cb.SetInstructions("You are a router.", nil)
	cb.SetSubagents([]SubagentInfo{
		{ID: "poet", Name: "Poet", Description: "Writes poems"},
	})
	prompt := cb.BuildSystemPrompt()

	if !strings.Contains(prompt, "## Delegation") {
		t.Error("delegation section should always be included when subagents exist")
	}
	if !strings.Contains(prompt, "poet") {
		t.Error("delegation section should list the subagent")
	}
}

func TestBuildSystemPrompt_DelegationOmittedWhenNoSubagents(t *testing.T) {
	cb := newTestContextBuilder(t)
	cb.SetInstructions("You are a poet.", nil)
	prompt := cb.BuildSystemPrompt()

	if strings.Contains(prompt, "## Delegation") {
		t.Error("delegation section should not appear without subagents")
	}
}

func TestBuildMessages_MemoryGating_NoInstructions(t *testing.T) {
	cb := newTestContextBuilder(t)
	// No instructions = full prompt; memory context should be attempted (no DB, so no crash)
	msgs := cb.BuildMessages(nil, "", "hello", nil, "test", "123", "", false)

	if len(msgs) < 2 {
		t.Fatalf("expected at least 2 messages (system + user), got %d", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Errorf("first message should be system, got %q", msgs[0].Role)
	}
	if msgs[len(msgs)-1].Content != "hello" {
		t.Errorf("last message should be user content, got %q", msgs[len(msgs)-1].Content)
	}
}

func TestBuildMessages_MemoryGating_InstructionsWithoutMemory(t *testing.T) {
	cb := newTestContextBuilder(t)
	cb.SetInstructions("You are a poet.", []string{"safety"})
	msgs := cb.BuildMessages(nil, "", "write a poem", nil, "test", "123", "", false)

	system := msgs[0].Content
	// Should contain instructions and safety, but no memory header
	if !strings.Contains(system, "You are a poet.") {
		t.Error("system prompt missing instructions")
	}
	if strings.Contains(system, "# Memory") {
		t.Error("memory should not be injected when not in context sections")
	}
}

func TestBuildMessages_MemoryGating_InstructionsWithMemory(t *testing.T) {
	cb := newTestContextBuilder(t)
	cb.SetInstructions("You are a poet.", []string{"memory"})
	// No actual memoryDB set, so no memory content - but the code path should be entered without panic
	msgs := cb.BuildMessages(nil, "", "write a poem", nil, "test", "123", "", false)

	if len(msgs) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(msgs))
	}
	if !strings.Contains(msgs[0].Content, "You are a poet.") {
		t.Error("system prompt missing instructions")
	}
}

func TestBuildMessages_SessionInfo(t *testing.T) {
	cb := newTestContextBuilder(t)
	cb.SetInstructions("You are a bot.", nil)
	msgs := cb.BuildMessages(nil, "", "hi", nil, "telegram", "42", "", false)

	// Session info should be in the dynamic context message (second system message),
	// not in the static system prompt (first), to keep the static prompt cacheable.
	if len(msgs) < 2 {
		t.Fatal("expected at least 2 messages (static system + dynamic context)")
	}
	if strings.Contains(msgs[0].Content, "Channel: telegram") {
		t.Error("static system prompt should NOT include channel info")
	}
	dynamic := msgs[1].Content
	if !strings.Contains(dynamic, "Channel: telegram") {
		t.Error("dynamic context should include channel info")
	}
	if !strings.Contains(dynamic, "Chat ID: 42") {
		t.Error("dynamic context should include chat ID")
	}
	if !strings.Contains(dynamic, "Chat type: direct message") {
		t.Error("dynamic context should include chat type for DM")
	}
}

func TestBuildMessages_SessionInfo_GroupChat(t *testing.T) {
	cb := newTestContextBuilder(t)
	cb.SetInstructions("You are a bot.", nil)
	msgs := cb.BuildMessages(nil, "", "hi", nil, "telegram", "42", "", true)

	if len(msgs) < 2 {
		t.Fatal("expected at least 2 messages")
	}
	dynamic := msgs[1].Content
	if !strings.Contains(dynamic, "Chat type: group chat") {
		t.Errorf("dynamic context should include group chat type, got %q", dynamic)
	}
}

func TestBuildMessages_SummaryAppended(t *testing.T) {
	cb := newTestContextBuilder(t)
	cb.SetInstructions("You are a bot.", nil)
	msgs := cb.BuildMessages(nil, "Previous discussion about weather.", "hi", nil, "", "", "", false)

	// Summary should be in the dynamic context message, not the static system prompt.
	if len(msgs) < 2 {
		t.Fatal("expected at least 2 messages (static system + dynamic context)")
	}
	if strings.Contains(msgs[0].Content, "Summary of Previous Conversation") {
		t.Error("static system prompt should NOT include summary")
	}
	dynamic := msgs[1].Content
	if !strings.Contains(dynamic, "Summary of Previous Conversation") {
		t.Error("dynamic context should include summary section")
	}
	if !strings.Contains(dynamic, "Previous discussion about weather.") {
		t.Error("dynamic context should include summary content")
	}
}

func TestSetInstructions_ContextSectionsParsed(t *testing.T) {
	cb := newTestContextBuilder(t)
	cb.SetInstructions("test", []string{"identity", "safety", "memory"})

	if !cb.contextSections["identity"] {
		t.Error("identity should be in context sections")
	}
	if !cb.contextSections["safety"] {
		t.Error("safety should be in context sections")
	}
	if !cb.contextSections["memory"] {
		t.Error("memory should be in context sections")
	}
	if cb.contextSections["bootstrap"] {
		t.Error("bootstrap should not be in context sections")
	}
	if cb.contextSections["skills"] {
		t.Error("skills should not be in context sections")
	}
}

func TestSetInstructions_NilSections(t *testing.T) {
	cb := newTestContextBuilder(t)
	cb.SetInstructions("test", nil)

	if cb.contextSections == nil {
		t.Fatal("contextSections should be initialized even with nil input")
	}
	if len(cb.contextSections) != 0 {
		t.Errorf("contextSections should be empty, got %d entries", len(cb.contextSections))
	}
}

func TestBuildSystemPrompt_SeparatorsBetweenSections(t *testing.T) {
	cb := newTestContextBuilder(t)
	cb.SetInstructions("You are a poet.", []string{"safety"})
	prompt := cb.BuildSystemPrompt()

	// Instructions and safety should be separated by ---
	if !strings.Contains(prompt, "\n\n---\n\n") {
		t.Error("sections should be separated by --- delimiter")
	}
}

// === Fix #3: containsWord with word boundary matching ===

func TestContainsWordExactMatch(t *testing.T) {
	if !containsWord("hello world", "hello") {
		t.Error("should match 'hello' at start")
	}
	if !containsWord("hello world", "world") {
		t.Error("should match 'world' at end")
	}
	if !containsWord("the quick fox", "quick") {
		t.Error("should match 'quick' in middle")
	}
}

func TestContainsWordSingleWord(t *testing.T) {
	if !containsWord("alice", "alice") {
		t.Error("should match entire string")
	}
}

func TestContainsWordNoFalseSubstring(t *testing.T) {
	if containsWord("going forward", "go") {
		t.Error("'go' should NOT match inside 'going'")
	}
	if containsWord("algorithm design", "algo") {
		t.Error("'algo' should NOT match inside 'algorithm'")
	}
	if containsWord("picoclaw is great", "claw") {
		t.Error("'claw' should NOT match inside 'picoclaw'")
	}
}

func TestContainsWordWithPunctuation(t *testing.T) {
	if !containsWord("hello, alice!", "alice") {
		t.Error("should match 'alice' next to punctuation")
	}
	if !containsWord("(alice) is here", "alice") {
		t.Error("should match 'alice' inside parens")
	}
	if !containsWord("ask alice.", "alice") {
		t.Error("should match 'alice' before period")
	}
}

func TestContainsWordMultipleOccurrences(t *testing.T) {
	// First occurrence is substring, second is word
	if !containsWord("going to go home", "go") {
		t.Error("should match standalone 'go' even if earlier substring 'going' exists")
	}
}

func TestContainsWordNoMatch(t *testing.T) {
	if containsWord("hello world", "xyz") {
		t.Error("should not match absent word")
	}
}

func TestContainsWordDigitBoundary(t *testing.T) {
	if containsWord("v2release", "release") {
		t.Error("'release' should NOT match when preceded by digit without space")
	}
	if !containsWord("v2 release", "release") {
		t.Error("should match 'release' after space")
	}
}

func TestContainsWordEmptyInputs(t *testing.T) {
	if containsWord("", "test") {
		t.Error("should not match in empty haystack")
	}
	if containsWord("test", "") {
		t.Error("should not match empty needle")
	}
	if containsWord("", "") {
		t.Error("should not match when both empty")
	}
}

// --- GetContextStats ---

func TestGetContextStats_FullPrompt(t *testing.T) {
	cb := newTestContextBuilder(t)
	stats := cb.GetContextStats(nil, "", "hello", "", "telegram", "42", false)

	// Full prompt should have identity, operational, safety
	names := make(map[string]bool)
	for _, p := range stats.SystemParts {
		names[p.Name] = true
		if p.Chars <= 0 || p.Tokens <= 0 {
			t.Errorf("part %q has zero chars/tokens", p.Name)
		}
	}
	if !names["identity"] {
		t.Error("missing identity part")
	}
	if !names["operational"] {
		t.Error("missing operational part")
	}
	if !names["safety"] {
		t.Error("missing safety part")
	}
}

func TestGetContextStats_InstructionsMode(t *testing.T) {
	cb := newTestContextBuilder(t)
	cb.SetInstructions("You are a poet.", []string{"safety"})
	stats := cb.GetContextStats(nil, "", "hi", "", "", "", false)

	names := make(map[string]bool)
	for _, p := range stats.SystemParts {
		names[p.Name] = true
	}
	if !names["instructions"] {
		t.Error("missing instructions part")
	}
	if !names["safety"] {
		t.Error("missing safety part")
	}
	if names["identity"] {
		t.Error("identity should not be included")
	}
	if names["operational"] {
		t.Error("operational should not be included in instructions mode")
	}
}

func TestGetContextStats_DynamicParts(t *testing.T) {
	cb := newTestContextBuilder(t)
	stats := cb.GetContextStats(nil, "A summary of things.", "hello", "", "telegram", "42", true)

	names := make(map[string]bool)
	for _, p := range stats.DynamicParts {
		names[p.Name] = true
	}
	if !names["session_info"] {
		t.Error("missing session_info dynamic part")
	}
	if !names["summary"] {
		t.Error("missing summary dynamic part")
	}
}

func TestGetContextStats_NoDynamicWithoutChannelOrSummary(t *testing.T) {
	cb := newTestContextBuilder(t)
	stats := cb.GetContextStats(nil, "", "hi", "", "", "", false)

	if len(stats.DynamicParts) != 0 {
		t.Errorf("expected no dynamic parts, got %d", len(stats.DynamicParts))
	}
}

func TestGetContextStats_HistoryTokens(t *testing.T) {
	cb := newTestContextBuilder(t)
	history := []providers.Message{
		{Role: "user", Content: strings.Repeat("a", 400)},
		{Role: "assistant", Content: strings.Repeat("b", 800)},
	}
	stats := cb.GetContextStats(history, "", "hi", "", "", "", false)

	if stats.HistoryCount != 2 {
		t.Errorf("expected 2 history messages, got %d", stats.HistoryCount)
	}
	// 400/4 + 800/4 = 300
	if stats.HistoryTokens != 300 {
		t.Errorf("expected 300 history tokens, got %d", stats.HistoryTokens)
	}
}

func TestGetContextStats_TotalIncludesAll(t *testing.T) {
	cb := newTestContextBuilder(t)
	history := []providers.Message{
		{Role: "user", Content: strings.Repeat("x", 100)},
	}
	stats := cb.GetContextStats(history, "some summary", "hi", "", "tg", "1", false)

	systemSum := 0
	for _, p := range stats.SystemParts {
		systemSum += p.Tokens
	}
	dynamicSum := 0
	for _, p := range stats.DynamicParts {
		dynamicSum += p.Tokens
	}
	expected := systemSum + dynamicSum + stats.HistoryTokens
	if stats.TotalTokens != expected {
		t.Errorf("total %d != system(%d) + dynamic(%d) + history(%d) = %d",
			stats.TotalTokens, systemSum, dynamicSum, stats.HistoryTokens, expected)
	}
}

func TestGetContextStats_DelegationIncluded(t *testing.T) {
	cb := newTestContextBuilder(t)
	cb.SetSubagents([]SubagentInfo{
		{ID: "poet", Name: "Poet", Description: "Writes poems"},
	})
	stats := cb.GetContextStats(nil, "", "hi", "", "", "", false)

	names := make(map[string]bool)
	for _, p := range stats.SystemParts {
		names[p.Name] = true
	}
	if !names["delegation"] {
		t.Error("missing delegation part when subagents set")
	}
}

// --- compressOldMessages ---

func TestCompressOldToolResults_TruncatesConsumedResults(t *testing.T) {
	longResult := strings.Repeat("A poem about BIDV. ", 100) // ~1900 chars
	messages := []providers.Message{
		{Role: "user", Content: "write a poem"},
		{Role: "assistant", Content: "", ToolCalls: []providers.ToolCall{{ID: "tc1"}}},
		{Role: "tool", Content: longResult, ToolCallID: "tc1"},
		{Role: "assistant", Content: "Here is the poem!"}, // final response - marks tc1 as consumed
		{Role: "user", Content: "hello"},
	}

	result := compressOldMessages(messages, 200)

	// Tool result at index 2 should be truncated
	if len([]rune(result[2].Content)) > 220 { // 200 + "... (truncated)"
		t.Errorf("consumed tool result should be truncated, got %d chars", len(result[2].Content))
	}
	if !strings.Contains(result[2].Content, "truncated") {
		t.Error("truncated result should contain truncation marker")
	}
	// ToolCallID must be preserved
	if result[2].ToolCallID != "tc1" {
		t.Errorf("ToolCallID lost, got %q", result[2].ToolCallID)
	}
}

func TestCompressOldToolResults_KeepsRecentResults(t *testing.T) {
	longResult := strings.Repeat("X", 500)
	messages := []providers.Message{
		{Role: "user", Content: "do something"},
		{Role: "assistant", Content: "", ToolCalls: []providers.ToolCall{{ID: "tc1"}}},
		{Role: "tool", Content: longResult, ToolCallID: "tc1"},
		// No final assistant response yet - tool result is still "active"
	}

	result := compressOldMessages(messages, 200)

	// Tool result should NOT be truncated (no final response after it)
	if result[2].Content != longResult {
		t.Error("active tool result should not be truncated")
	}
}

func TestCompressOldToolResults_MultipleRoundsOnlyOldTruncated(t *testing.T) {
	oldResult := strings.Repeat("old poem content ", 50)
	newResult := strings.Repeat("new poem content ", 50)

	messages := []providers.Message{
		{Role: "user", Content: "poem 1"},
		{Role: "assistant", Content: "", ToolCalls: []providers.ToolCall{{ID: "tc1"}}},
		{Role: "tool", Content: oldResult, ToolCallID: "tc1"},
		{Role: "assistant", Content: "Here is poem 1"}, // consumed
		{Role: "user", Content: "poem 2"},
		{Role: "assistant", Content: "", ToolCalls: []providers.ToolCall{{ID: "tc2"}}},
		{Role: "tool", Content: newResult, ToolCallID: "tc2"},
		// No final response for tc2 yet
	}

	result := compressOldMessages(messages, 200)

	// Old result (index 2) should be truncated
	if len([]rune(result[2].Content)) > 220 {
		t.Errorf("old tool result should be truncated, got %d chars", len(result[2].Content))
	}
	// New result (index 6) should be kept in full
	if result[6].Content != newResult {
		t.Error("current tool result should not be truncated")
	}
}

func TestCompressOldToolResults_ShortResultsUnchanged(t *testing.T) {
	messages := []providers.Message{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "", ToolCalls: []providers.ToolCall{{ID: "tc1"}}},
		{Role: "tool", Content: "short result", ToolCallID: "tc1"},
		{Role: "assistant", Content: "done"},
		{Role: "user", Content: "bye"},
	}

	result := compressOldMessages(messages, 200)

	if result[2].Content != "short result" {
		t.Error("short tool results should not be modified")
	}
}

func TestCompressOldToolResults_NoToolMessages(t *testing.T) {
	messages := []providers.Message{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "hello"},
	}

	result := compressOldMessages(messages, 200)

	if len(result) != 2 {
		t.Errorf("expected 2 messages, got %d", len(result))
	}
}

func TestCompressOldMessages_CompressesOldAssistantResponses(t *testing.T) {
	longResponse := strings.Repeat("Here is a very long poem about BIDV. ", 50)
	messages := []providers.Message{
		{Role: "user", Content: "write a poem"},
		{Role: "assistant", Content: "", ToolCalls: []providers.ToolCall{{ID: "tc1"}}},
		{Role: "tool", Content: "poem result", ToolCallID: "tc1"},
		{Role: "assistant", Content: longResponse}, // old final response
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "Hi there!"}, // latest final response
	}

	result := compressOldMessages(messages, 200)

	// Old assistant response (index 3) should be truncated
	if len([]rune(result[3].Content)) > 220 {
		t.Errorf("old assistant response should be truncated, got %d chars", len([]rune(result[3].Content)))
	}
	if !strings.Contains(result[3].Content, "truncated") {
		t.Error("truncated response should contain marker")
	}
	// Latest assistant response (index 5) should be kept
	if result[5].Content != "Hi there!" {
		t.Errorf("latest response should be unchanged, got %q", result[5].Content)
	}
}

func TestCompressOldMessages_KeepsAssistantWithToolCalls(t *testing.T) {
	messages := []providers.Message{
		{Role: "user", Content: "do something"},
		{Role: "assistant", Content: strings.Repeat("X", 500), ToolCalls: []providers.ToolCall{{ID: "tc1"}}},
		{Role: "tool", Content: "ok", ToolCallID: "tc1"},
		{Role: "assistant", Content: "done"},
	}

	result := compressOldMessages(messages, 200)

	// Assistant with tool calls (index 1) should NOT be compressed
	if len(result[1].Content) != 500 {
		t.Errorf("assistant with tool calls should not be compressed, got %d chars", len(result[1].Content))
	}
}

func TestCompressOldMessages_IntegrationWithSanitize(t *testing.T) {
	longPoem := strings.Repeat("BIDV vuong vang tua nui cao. ", 100)

	history := []providers.Message{
		{Role: "user", Content: "write poem about BIDV"},
		{Role: "assistant", Content: "", ToolCalls: []providers.ToolCall{{ID: "tc1", Name: "delegate"}}},
		{Role: "tool", Content: longPoem, ToolCallID: "tc1"},
		{Role: "assistant", Content: "Here is the poem: " + longPoem}, // echoes full poem
		{Role: "user", Content: "write another poem"},
		{Role: "assistant", Content: "", ToolCalls: []providers.ToolCall{{ID: "tc2", Name: "delegate"}}},
		{Role: "tool", Content: longPoem, ToolCallID: "tc2"},
		{Role: "assistant", Content: "Here is poem 2: " + longPoem},
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "Hi!"},
	}

	result := sanitizeHistory(history)

	for i, msg := range result {
		if msg.Role == "tool" && len([]rune(msg.Content)) > 220 {
			t.Errorf("tool result at index %d should be truncated, got %d chars", i, len([]rune(msg.Content)))
		}
	}
	// Old assistant final responses should also be truncated
	for i, msg := range result {
		if msg.Role == "assistant" && len(msg.ToolCalls) == 0 && msg.Content != "" && len([]rune(msg.Content)) > 220 {
			// Only the last final assistant should be long (but "Hi!" is short)
			t.Errorf("old assistant response at index %d should be truncated, got %d chars", i, len([]rune(msg.Content)))
		}
	}
}

// --- Tiered Memory Injection ---

// newTestCBWithMemory creates a ContextBuilder with a real SQLite memory DB for testing.
func newTestCBWithMemory(t *testing.T) (*ContextBuilder, *memory.MemoryDB) {
	t.Helper()
	dir := t.TempDir()
	cb := NewContextBuilder(dir)
	db, err := memory.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	cb.SetMemoryDB(db, nil)
	return cb, db
}

func TestMemoryTier1_GroupContextByChatID(t *testing.T) {
	cb, db := newTestCBWithMemory(t)

	// Group memory contains chatID in content
	db.Store("group_reaonline", "GROUP: Reaonline | ID: -1003269096966 | Persona: friendly", "core", "")
	db.Store("group_cauca", "GROUP: Cauca | ID: -4661401806 | Persona: cheeky", "core", "")
	db.Store("users_reaonline", "Members of reaonline group ID: -1003269096966", "core", "")

	ctx := cb.buildRelevantMemoryContext("hello", "", "-1003269096966")

	// Tier 1: should include group_reaonline and users_reaonline (contain chatID)
	if !strings.Contains(ctx, "group_reaonline") {
		t.Error("should include group_reaonline matching chatID")
	}
	if !strings.Contains(ctx, "users_reaonline") {
		t.Error("should include users_reaonline matching chatID")
	}
	// Should NOT include group_cauca (different chatID)
	if strings.Contains(ctx, "Group Context") && strings.Contains(ctx, "group_cauca") {
		t.Error("should not include group_cauca in Group Context section")
	}
}

func TestMemoryTier2_SenderProfileByOwner(t *testing.T) {
	cb, db := newTestCBWithMemory(t)

	db.Store("user_khiemnnm_insights", "khiemnnm is the admin, likes automation", "core", "")
	db.Store("user_litt0709", "litt0709 is a policy critic", "core", "")
	db.Store("group_reaonline", "GROUP: Reaonline | ID: -100 | @khiemnnm @litt0709", "core", "")

	ctx := cb.buildRelevantMemoryContext("hello", "khiemnnm", "-100")

	// Tier 2: should include khiemnnm's profile
	if !strings.Contains(ctx, "user_khiemnnm_insights") {
		t.Error("should include sender's own profile")
	}
	// Should NOT include litt0709 in Sender Context (not the sender)
	if strings.Contains(ctx, "Sender Context") && strings.Contains(ctx, "user_litt0709") {
		t.Error("should not include other user's profile in Sender Context")
	}
}

func TestMemoryTier2_MatchesByAtMention(t *testing.T) {
	cb, db := newTestCBWithMemory(t)

	// Memory that mentions @alice in content but key doesn't contain "alice"
	db.Store("team_members", "Team: @alice (dev), @bob (pm)", "core", "")

	ctx := cb.buildRelevantMemoryContext("hi", "alice", "")

	if !strings.Contains(ctx, "team_members") {
		t.Error("should match memory containing @alice for sender alice")
	}
}

func TestMemoryTier3_DailyNotesAlwaysIncluded(t *testing.T) {
	cb, db := newTestCBWithMemory(t)

	db.Store("daily_2026_03_14", "Today: election day, gold prices up", "daily", "")
	db.Store("group_other", "GROUP: Other | ID: -999", "core", "")

	ctx := cb.buildRelevantMemoryContext("hello", "", "-123")

	// Daily notes always included regardless of chatID
	if !strings.Contains(ctx, "daily_2026_03_14") {
		t.Error("daily notes should always be included")
	}
}

func TestMemoryTier4_FTSFindsUnmatchedCores(t *testing.T) {
	cb, db := newTestCBWithMemory(t)

	// Core memory not matching chatID or owner
	db.Store("user_litt0709", "litt0709 policy critic gold", "core", "")
	db.Store("group_reaonline", "GROUP: Reaonline | ID: -100", "core", "")

	// Use exact content term for FTS match
	ctx := cb.buildRelevantMemoryContext("litt0709", "khiemnnm", "-100")

	if !strings.Contains(ctx, "user_litt0709") {
		t.Error("FTS should surface unmatched core memories when message content is relevant")
	}
}

func TestMemoryTier4_GraphWalkFindsRelatedMemories(t *testing.T) {
	cb, db := newTestCBWithMemory(t)

	db.Store("user_alice_profile", "Alice is a backend engineer", "core", "")
	db.AddRelation("Alice", "works_on", "PicoClaw", "user_alice_profile")

	// Message mentions "PicoClaw" — graph should walk to Alice's memory
	ctx := cb.buildRelevantMemoryContext("who works on PicoClaw", "", "")

	if !strings.Contains(ctx, "user_alice_profile") {
		t.Error("graph walk should surface memories related to mentioned entities")
	}
}

func TestMemoryNoMemoryDB_ReturnsEmpty(t *testing.T) {
	cb := newTestContextBuilder(t)
	// No memoryDB set
	ctx := cb.buildRelevantMemoryContext("hello", "user1", "-123")
	if ctx != "" {
		t.Errorf("expected empty context without memoryDB, got %q", ctx)
	}
}

func TestMemoryEmptyChatID_SkipsTier1(t *testing.T) {
	cb, db := newTestCBWithMemory(t)

	db.Store("group_reaonline", "GROUP: Reaonline | ID: -100", "core", "")
	db.Store("user_bob", "Bob likes fishing", "core", "")

	ctx := cb.buildRelevantMemoryContext("hello", "bob", "")

	// No chatID → no Group Context section
	if strings.Contains(ctx, "Group Context") {
		t.Error("should not have Group Context without chatID")
	}
	// But Tier 2 (sender) should still work
	if !strings.Contains(ctx, "user_bob") {
		t.Error("sender profile should still be included without chatID")
	}
}

func TestMemoryEmptyOwner_SkipsTier2(t *testing.T) {
	cb, db := newTestCBWithMemory(t)

	db.Store("user_khiemnnm", "Admin user", "core", "")
	db.Store("group_test", "GROUP: Test | ID: -42", "core", "")

	ctx := cb.buildRelevantMemoryContext("hello", "", "-42")

	// No owner → no Sender Context section
	if strings.Contains(ctx, "Sender Context") {
		t.Error("should not have Sender Context without owner")
	}
	// Tier 1 (group) should still work
	if !strings.Contains(ctx, "group_test") {
		t.Error("group context should still be included without owner")
	}
}

func TestMemoryHeaderCompressed(t *testing.T) {
	cb, db := newTestCBWithMemory(t)

	db.Store("daily_test", "some daily note", "daily", "")

	ctx := cb.buildRelevantMemoryContext("hello", "", "")

	if !strings.Contains(ctx, "# Memory") {
		t.Error("should have memory header")
	}
	// Verify the header is the shorter version
	if strings.Contains(ctx, "When interacting with me if something seems memorable") {
		t.Error("should use compressed memory header, not the old verbose one")
	}
}

func TestMemoryTierIsolation_OtherGroupNotInGroupContext(t *testing.T) {
	cb, db := newTestCBWithMemory(t)

	db.Store("group_rea", "GROUP: Rea | ID: -100 | Persona: cute", "core", "")
	db.Store("group_cauca", "GROUP: Cauca | ID: -200 | Persona: cheeky", "core", "")
	db.Store("users_rea", "Members: @alice @bob, group ID: -100", "core", "")
	db.Store("users_cauca", "Members: @charlie @dave, group ID: -200", "core", "")

	ctx := cb.buildRelevantMemoryContext("hello", "alice", "-100")

	// Count occurrences in Group Context section only
	groupIdx := strings.Index(ctx, "## Group Context")
	senderIdx := strings.Index(ctx, "## Sender Context")

	if groupIdx < 0 {
		t.Fatal("missing Group Context section")
	}

	// Extract just the Group Context section
	groupSection := ctx[groupIdx:]
	if senderIdx > groupIdx {
		groupSection = ctx[groupIdx:senderIdx]
	}

	if strings.Contains(groupSection, "group_cauca") {
		t.Error("Group Context should not contain other group's config")
	}
	if strings.Contains(groupSection, "users_cauca") {
		t.Error("Group Context should not contain other group's member list")
	}
}
