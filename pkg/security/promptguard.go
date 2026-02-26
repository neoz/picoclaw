package security

import (
	"regexp"
	"strings"
)

// GuardAction determines the response when a prompt injection is detected.
type GuardAction string

const (
	ActionWarn  GuardAction = "warn"
	ActionBlock GuardAction = "block"
)

// GuardResult contains the outcome of scanning input for prompt injection.
type GuardResult struct {
	Safe     bool
	Patterns []string
	Score    float64
	Action   GuardAction
}

// PromptGuard detects prompt injection attempts using regex-based pattern matching.
type PromptGuard struct {
	action      GuardAction
	sensitivity float64
	categories  []guardCategory
}

type guardCategory struct {
	name    string
	score   float64
	pattern *regexp.Regexp
}

// NewPromptGuard creates a PromptGuard with the given action and sensitivity.
// Sensitivity is clamped to [0.0, 1.0]; default 0.5.
func NewPromptGuard(action string, sensitivity float64) *PromptGuard {
	a := ActionWarn
	if action == "block" {
		a = ActionBlock
	}
	if sensitivity <= 0 {
		sensitivity = 0.5
	}
	if sensitivity > 1 {
		sensitivity = 1.0
	}
	return &PromptGuard{
		action:      a,
		sensitivity: sensitivity,
		categories:  defaultGuardCategories(),
	}
}

func defaultGuardCategories() []guardCategory {
	return []guardCategory{
		{
			name:    "system_override",
			score:   1.0,
			pattern: regexp.MustCompile(`(?i)ignore\s+(the\s+)?(previous|all|above|prior)\s+(instructions?|prompts?|commands?)`),
		},
		{
			name:    "role_confusion",
			score:   0.9,
			pattern: regexp.MustCompile(`(?i)(you\s+are\s+now|act\s+as|pretend\s+(you'?re|to\s+be))`),
		},
		{
			name:    "tool_call_injection",
			score:   0.8,
			pattern: regexp.MustCompile(`(?i)(tool_calls|function_call)\s*[:\[{]`),
		},
		{
			name:    "secret_extraction",
			score:   0.95,
			pattern: regexp.MustCompile(`(?i)(list|show|print|display|reveal|tell\s+me)\s+(me\s+)?(all\s+|the\s+|your\s+|my\s+)?(secrets?|credentials?|passwords?|tokens?|api[_\s-]?keys?)`),
		},
		{
			name:    "command_injection:backtick",
			score:   0.6,
			pattern: regexp.MustCompile("`[^`]*`"),
		},
		{
			name:    "command_injection:subshell",
			score:   0.6,
			pattern: regexp.MustCompile(`\$\([^)]+\)`),
		},
		{
			name:    "command_injection:semicolon",
			score:   0.6,
			pattern: regexp.MustCompile(`(?i);\s*(rm|curl|wget|nc|bash|sh|python|perl|ruby|chmod|chown|dd|mkfs)`),
		},
		{
			name:    "command_injection:pipe",
			score:   0.6,
			pattern: regexp.MustCompile(`(?i)\|\s*(bash|sh|python|perl|ruby)`),
		},
		{
			name:    "command_injection:chain",
			score:   0.6,
			pattern: regexp.MustCompile(`(?i)&&\s*(rm|curl|wget|nc|bash|sh|python|perl|ruby|chmod|chown|dd|mkfs)`),
		},
		{
			name:  "jailbreak",
			score: 0.85,
			pattern: regexp.MustCompile(
				`(?i)(` +
					`DAN\s+mode` +
					`|do\s+anything\s+now` +
					`|developer\s+mode\s+(enabled|on|activated)` +
					`|debug\s+mode\s+(enabled|on|activated)` +
					`|in\s+a\s+hypothetical\s+scenario` +
					`|pretend\s+(there\s+are|you\s+have)\s+no\s+(restrictions|rules|limits)` +
					`|base64\s+decode` +
					`|ignore\s+(safety|content)\s+(filters?|policies|guidelines)` +
					`)`,
			),
		},
		{
			name:  "jailbreak:persona",
			score: 0.85,
			pattern: regexp.MustCompile(
				`(?i)(` +
					`jailbr(eak|oken|oke)` +
					`|evil\s+(mode|version|twin|persona)` +
					`|god\s+mode` +
					`|sudo\s+mode` +
					`|unfiltered\s+(mode|version|response)` +
					`|uncensored\s+(mode|version|response)` +
					`|no\s+(rules|limits|boundaries)\s+mode` +
					`|chaos\s+mode` +
					`|unrestricted\s+(mode|ai|assistant)` +
					`|anti[\s-]?alignment` +
					`)`,
			),
		},
		{
			name:  "jailbreak:roleplay_bypass",
			score: 0.8,
			pattern: regexp.MustCompile(
				`(?i)(` +
					`as\s+a\s+(fictional|hypothetical|imaginary)\s+(ai|character|assistant)` +
					`|for\s+(educational|research|academic)\s+purposes?\s+only` +
					`|this\s+is\s+(just\s+)?(fiction|a\s+story|roleplay|creative\s+writing)` +
					`|in\s+this\s+(fictional|alternate)\s+(universe|world|reality|scenario)` +
					`|imagine\s+you\s+(have|had)\s+no\s+(restrictions|rules|limits|guidelines)` +
					`|hypothetically[\s,]+if\s+you\s+(had|were|could)` +
					`|what\s+would\s+(an?\s+)?(evil|unrestricted|unfiltered)\s+(ai|assistant|version)` +
					`)`,
			),
		},
		{
			name:  "jailbreak:encoding_evasion",
			score: 0.7,
			pattern: regexp.MustCompile(
				`(?i)(` +
					`base64\s+encode` +
					`|rot13` +
					`|hex\s+(encode|decode|convert)` +
					`|unicode\s+escape` +
					`|encode\s+(this|the|your)\s+(response|output|answer)\s+in` +
					`|respond\s+in\s+(base64|hex|rot13|binary|morse)` +
					`|translate\s+(this|your\s+response)\s+(to|into)\s+(base64|hex|rot13|binary)` +
					`|write\s+(it|this|your\s+answer)\s+(backwards|reversed|in\s+reverse)` +
					`)`,
			),
		},
		{
			name:  "prompt_leak",
			score: 0.95,
			pattern: regexp.MustCompile(
				`(?i)(` +
					`(show|print|display|reveal|output|repeat|tell\s+me|give\s+me|what\s+(is|are))\s+(me\s+)?(the\s+|your\s+)?(system\s+prompt|system\s+message|system\s+instructions?)` +
					`|what\s+(is|was)\s+(in\s+)?(the\s+|your\s+)(initial|original|first|hidden|secret)\s+(prompt|instructions?|message)` +
					`|(ignore|forget)\s+everything\s+and\s+(repeat|print|show|output|display)` +
					`|(copy|paste|echo|dump|leak|extract)\s+(the\s+|your\s+)?(system|initial|original|hidden)\s+(prompt|instructions?|message)` +
					`)`,
			),
		},
		{
			name:  "prompt_leak:indirect",
			score: 0.8,
			pattern: regexp.MustCompile(
				`(?i)(` +
					`(what|how)\s+(were|are)\s+you\s+(programmed|configured|instructed|prompted|initialized|set\s+up)` +
					`|(list|enumerate|describe)\s+(all\s+)?(the\s+|your\s+)(rules|instructions?|directives?|constraints?|guidelines?)` +
					`|what\s+(rules|instructions?|guidelines?|constraints?)\s+(are\s+you|do\s+you|were\s+you|have\s+you\s+been)\s+(given|following|under|told|bound\s+by)` +
					`|summarize\s+(your|the)\s+(instructions?|prompt|rules|directives?|guidelines?|programming)` +
					`|above\s+(text|prompt|instructions?|messages?)\s+(verbatim|exactly|word[\s-]for[\s-]word)` +
					`)`,
			),
		},
	}
}

const maxGuardScore = 11.6

// Scan checks input content for prompt injection patterns.
func (pg *PromptGuard) Scan(content string) GuardResult {
	var matched []string
	var totalScore float64

	for _, cat := range pg.categories {
		if cat.pattern.MatchString(content) {
			matched = append(matched, cat.name)
			totalScore += cat.score
		}
	}

	normalized := totalScore / maxGuardScore
	safe := normalized < pg.sensitivity

	action := pg.action
	if safe {
		action = ""
	}

	return GuardResult{
		Safe:     safe,
		Patterns: matched,
		Score:    normalized,
		Action:   action,
	}
}

// ParseGuardAction converts a string to GuardAction, defaulting to ActionWarn.
func ParseGuardAction(s string) GuardAction {
	if strings.ToLower(s) == "block" {
		return ActionBlock
	}
	return ActionWarn
}
