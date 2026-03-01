package agent

import (
	"strings"
	"testing"
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
	// Should NOT contain identity or safety sections
	if strings.Contains(prompt, "# picoclaw") {
		t.Error("lightweight prompt should not contain identity section")
	}
	if strings.Contains(prompt, "## Safety") {
		t.Error("lightweight prompt should not contain safety section")
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
	if strings.Contains(prompt, "## Safety") {
		t.Error("prompt should not include safety section")
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
	msgs := cb.BuildMessages(nil, "", "hello", nil, "test", "123", "")

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
	msgs := cb.BuildMessages(nil, "", "write a poem", nil, "test", "123", "")

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
	msgs := cb.BuildMessages(nil, "", "write a poem", nil, "test", "123", "")

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
	msgs := cb.BuildMessages(nil, "", "hi", nil, "telegram", "42", "")

	system := msgs[0].Content
	if !strings.Contains(system, "Channel: telegram") {
		t.Error("system prompt should include channel info")
	}
	if !strings.Contains(system, "Chat ID: 42") {
		t.Error("system prompt should include chat ID")
	}
}

func TestBuildMessages_SummaryAppended(t *testing.T) {
	cb := newTestContextBuilder(t)
	cb.SetInstructions("You are a bot.", nil)
	msgs := cb.BuildMessages(nil, "Previous discussion about weather.", "hi", nil, "", "", "")

	system := msgs[0].Content
	if !strings.Contains(system, "Summary of Previous Conversation") {
		t.Error("system prompt should include summary section")
	}
	if !strings.Contains(system, "Previous discussion about weather.") {
		t.Error("system prompt should include summary content")
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
