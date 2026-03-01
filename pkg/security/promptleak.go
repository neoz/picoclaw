package security

import (
	"strings"
)

// PromptLeakResult contains the outcome of scanning output for system prompt leakage.
type PromptLeakResult struct {
	Leaked       bool
	MatchedCount int
	TotalPrints  int
	Score        float64
	Action       GuardAction
}

// PromptLeakDetector detects system prompt content in LLM output using fingerprint matching.
// It extracts meaningful lines from the system prompt and checks if the LLM response
// reproduces too many of them — works regardless of what language the extraction request was in.
type PromptLeakDetector struct {
	fingerprints []string
	threshold    float64 // fraction of fingerprints that must match to trigger
	action       GuardAction
}

// NewPromptLeakDetector creates a detector from the system prompt content.
// threshold controls sensitivity: lower = more sensitive (0.05 means 5% of fingerprints matching triggers).
// Clamped to [0.01, 1.0]; default 0.15.
func NewPromptLeakDetector(systemPrompt string, threshold float64, action string) *PromptLeakDetector {
	if threshold <= 0 {
		threshold = 0.15
	}
	if threshold > 1 {
		threshold = 1.0
	}
	if threshold < 0.01 {
		threshold = 0.01
	}

	a := ActionWarn
	if action == "block" {
		a = ActionBlock
	}

	return &PromptLeakDetector{
		fingerprints: extractFingerprints(systemPrompt),
		threshold:    threshold,
		action:       a,
	}
}

// extractFingerprints extracts unique, meaningful lines from the system prompt.
// Short lines, separators, and generic content are excluded to reduce false positives.
func extractFingerprints(prompt string) []string {
	lines := strings.Split(prompt, "\n")
	seen := make(map[string]bool)
	var prints []string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty, short, or generic lines
		if len(line) < 20 {
			continue
		}
		// Skip markdown separators and headings-only lines
		if line == "---" {
			continue
		}
		// Skip lines that are only markdown heading markers
		trimmed := strings.TrimLeft(line, "# ")
		if len(trimmed) < 10 {
			continue
		}
		// Skip very common phrases that would cause false positives
		lower := strings.ToLower(line)
		if lower == "you are a helpful ai assistant." {
			continue
		}

		// Normalize for matching
		normalized := normalizeForMatch(line)
		if seen[normalized] {
			continue
		}
		seen[normalized] = true
		prints = append(prints, normalized)
	}

	return prints
}

// normalizeForMatch lowercases and collapses whitespace for fuzzy matching.
func normalizeForMatch(s string) string {
	s = strings.ToLower(s)
	// Collapse multiple spaces into one
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

// Scan checks if the output content contains system prompt fingerprints.
func (d *PromptLeakDetector) Scan(output string) PromptLeakResult {
	if len(d.fingerprints) == 0 {
		return PromptLeakResult{Leaked: false}
	}

	normalizedOutput := normalizeForMatch(output)
	matched := 0

	for _, fp := range d.fingerprints {
		if strings.Contains(normalizedOutput, fp) {
			matched++
		}
	}

	score := float64(matched) / float64(len(d.fingerprints))
	leaked := score >= d.threshold

	action := d.action
	if !leaked {
		action = ""
	}

	return PromptLeakResult{
		Leaked:       leaked,
		MatchedCount: matched,
		TotalPrints:  len(d.fingerprints),
		Score:        score,
		Action:       action,
	}
}

// FingerprintCount returns the number of extracted fingerprints.
func (d *PromptLeakDetector) FingerprintCount() int {
	return len(d.fingerprints)
}
