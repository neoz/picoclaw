package security

import (
	"strings"
	"testing"
)

const testSystemPrompt = `# picoclaw

You are picoclaw, a helpful AI assistant.

## Current Time
2026-01-15 10:30 (Wednesday)

## Runtime
linux amd64, Go go1.26

## Workspace
Your workspace is at: /home/user/.picoclaw/workspace
- Skills: /home/user/.picoclaw/workspace/skills/{skill-name}/SKILL.md

## Important Rules

1. **ALWAYS use tools** - When you need to perform an action (schedule reminders, send messages, execute commands, etc.), you MUST call the appropriate tool. Do NOT just say you'll do it or pretend to do it.

2. **Be helpful and accurate** - When using tools, briefly explain what you're doing.

3. **Memory** - When interacting with me if something seems memorable or important, use the memory_store tool to save it. When I ask you about past information, use memory_search to find it. If you need to update or delete something, use memory_forget.

---

## SOUL.md

You are a personal AI companion with a warm and thoughtful personality.
Your core values are helpfulness, honesty, and respect for privacy.
Always think step by step before acting on complex requests.
When uncertain, ask for clarification rather than guessing.

---

## USER.md

The user prefers concise responses and dislikes overly verbose explanations.
The user works primarily with Go and Python programming languages.
The user timezone is UTC+7.`

func TestPromptLeakDetector_ExtractFingerprints(t *testing.T) {
	d := NewPromptLeakDetector(testSystemPrompt, 0.15, "block")
	count := d.FingerprintCount()
	if count == 0 {
		t.Fatal("expected fingerprints to be extracted from system prompt")
	}
	if count < 5 {
		t.Errorf("expected at least 5 fingerprints, got %d", count)
	}
}

func TestPromptLeakDetector_DetectsLeakage(t *testing.T) {
	d := NewPromptLeakDetector(testSystemPrompt, 0.15, "block")

	// Simulate LLM dumping the system prompt
	result := d.Scan(testSystemPrompt)
	if !result.Leaked {
		t.Errorf("expected leakage detection when output IS the system prompt (score=%.2f, matched=%d/%d)", result.Score, result.MatchedCount, result.TotalPrints)
	}
	if result.Action != ActionBlock {
		t.Errorf("expected ActionBlock, got %q", result.Action)
	}
}

func TestPromptLeakDetector_PartialLeakage(t *testing.T) {
	d := NewPromptLeakDetector(testSystemPrompt, 0.15, "block")

	// Extract a significant portion of the system prompt
	lines := strings.Split(testSystemPrompt, "\n")
	// Take first half
	partial := strings.Join(lines[:len(lines)/2], "\n")

	result := d.Scan(partial)
	if !result.Leaked {
		t.Errorf("expected leakage detection for partial system prompt (score=%.2f, matched=%d/%d)", result.Score, result.MatchedCount, result.TotalPrints)
	}
}

func TestPromptLeakDetector_NoFalsePositive(t *testing.T) {
	d := NewPromptLeakDetector(testSystemPrompt, 0.15, "block")

	benignOutputs := []string{
		"Sure, I can help you with that Go code. Here's the function you need.",
		"The weather today is sunny with a high of 25 degrees.",
		"I've scheduled the reminder for tomorrow at 3 PM.",
		"Let me search my memory for that information.",
		"Here's how to write a simple HTTP server in Go:\n\npackage main\n\nimport \"net/http\"\n\nfunc main() {\n\thttp.ListenAndServe(\":8080\", nil)\n}",
		"I found 3 results matching your query. The most relevant one is from last week.",
	}

	for _, output := range benignOutputs {
		result := d.Scan(output)
		if result.Leaked {
			t.Errorf("false positive for benign output %q (score=%.2f, matched=%d/%d)", output, result.Score, result.MatchedCount, result.TotalPrints)
		}
	}
}

func TestPromptLeakDetector_WarnAction(t *testing.T) {
	d := NewPromptLeakDetector(testSystemPrompt, 0.15, "warn")
	result := d.Scan(testSystemPrompt)
	if !result.Leaked {
		t.Fatal("expected leakage detection")
	}
	if result.Action != ActionWarn {
		t.Errorf("expected ActionWarn, got %q", result.Action)
	}
}

func TestPromptLeakDetector_ThresholdSensitivity(t *testing.T) {
	// Very high threshold - only triggers on near-complete dump
	dHigh := NewPromptLeakDetector(testSystemPrompt, 0.9, "block")
	// Take just a few lines
	lines := strings.Split(testSystemPrompt, "\n")
	smallPortion := strings.Join(lines[:5], "\n")

	result := dHigh.Scan(smallPortion)
	if result.Leaked {
		t.Errorf("high threshold should not trigger on small portion (score=%.2f)", result.Score)
	}

	// Very low threshold - triggers easily
	dLow := NewPromptLeakDetector(testSystemPrompt, 0.05, "block")
	result2 := dLow.Scan(smallPortion)
	// May or may not trigger depending on fingerprint extraction of those lines
	_ = result2 // just ensure no panic
}

func TestPromptLeakDetector_EmptyPrompt(t *testing.T) {
	d := NewPromptLeakDetector("", 0.15, "block")
	if d.FingerprintCount() != 0 {
		t.Error("expected 0 fingerprints for empty prompt")
	}
	result := d.Scan("any output")
	if result.Leaked {
		t.Error("should not detect leakage with empty prompt")
	}
}

func TestPromptLeakDetector_CaseInsensitive(t *testing.T) {
	d := NewPromptLeakDetector(testSystemPrompt, 0.15, "block")

	// Uppercase version of system prompt should still match
	upper := strings.ToUpper(testSystemPrompt)
	result := d.Scan(upper)
	if !result.Leaked {
		t.Errorf("case-insensitive matching should detect uppercase dump (score=%.2f, matched=%d/%d)", result.Score, result.MatchedCount, result.TotalPrints)
	}
}

func TestPromptLeakDetector_DefaultThreshold(t *testing.T) {
	// threshold <= 0 should default to 0.15
	d := NewPromptLeakDetector(testSystemPrompt, 0, "block")
	result := d.Scan(testSystemPrompt)
	if !result.Leaked {
		t.Error("default threshold should still detect full dump")
	}
}

func TestExtractFingerprints_SkipsShortLines(t *testing.T) {
	prompt := "short\n# H\nThis is a line that is long enough to be a fingerprint candidate for matching."
	prints := extractFingerprints(prompt)
	if len(prints) != 1 {
		t.Errorf("expected 1 fingerprint (only the long line), got %d", len(prints))
	}
}

func TestExtractFingerprints_Deduplicates(t *testing.T) {
	prompt := "This exact line appears in the prompt as a fingerprint.\nThis exact line appears in the prompt as a fingerprint."
	prints := extractFingerprints(prompt)
	if len(prints) != 1 {
		t.Errorf("expected 1 fingerprint after dedup, got %d", len(prints))
	}
}
