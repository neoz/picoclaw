package security

import (
	"regexp"
)

// LeakResult contains the outcome of scanning output for credential leaks.
type LeakResult struct {
	Clean    bool
	Patterns []string
	Redacted string
}

// LeakDetector detects and redacts credential patterns in outbound content.
type LeakDetector struct {
	sensitivity float64
	categories  []leakCategory
}

type leakCategory struct {
	name        string
	alwaysOn    bool // if false, only active when sensitivity > 0.5
	pattern     *regexp.Regexp
	replacement string
}

// NewLeakDetector creates a LeakDetector with the given sensitivity.
// Sensitivity is clamped to [0.0, 1.0]; default 0.7.
func NewLeakDetector(sensitivity float64) *LeakDetector {
	if sensitivity <= 0 {
		sensitivity = 0.7
	}
	if sensitivity > 1 {
		sensitivity = 1.0
	}
	return &LeakDetector{
		sensitivity: sensitivity,
		categories:  defaultLeakCategories(),
	}
}

func defaultLeakCategories() []leakCategory {
	return []leakCategory{
		{
			name:     "api_key",
			alwaysOn: true,
			pattern: regexp.MustCompile(
				`(` +
					`sk_(live|test)_[a-zA-Z0-9]{20,}` + // Stripe
					`|sk-[a-zA-Z0-9]{20,}` + // OpenAI
					`|sk-ant-[a-zA-Z0-9_-]{20,}` + // Anthropic
					`|AIza[a-zA-Z0-9_-]{35}` + // Google
					`|gh[pousr]_[a-zA-Z0-9]{36,}` + // GitHub (classic)
					`|github_pat_[a-zA-Z0-9_]{22,}` + // GitHub (fine-grained)
					`)`,
			),
			replacement: "[REDACTED_API_KEY]",
		},
		{
			name:     "aws_credential",
			alwaysOn: true,
			pattern: regexp.MustCompile(
				`(` +
					`AKIA[A-Z0-9]{16}` + // AWS Access Key
					`|(?i)aws[_-]?secret[_-]?access[_-]?key\s*[=:]\s*\S+` + // AWS Secret Key assignment
					`)`,
			),
			replacement: "[REDACTED_AWS_CREDENTIAL]",
		},
		{
			name:        "private_key",
			alwaysOn:    true,
			pattern:     regexp.MustCompile(`-----BEGIN\s+(RSA\s+|EC\s+|OPENSSH\s+)?PRIVATE\s+KEY-----`),
			replacement: "[REDACTED_PRIVATE_KEY]",
		},
		{
			name:        "jwt",
			alwaysOn:    true,
			pattern:     regexp.MustCompile(`eyJ[a-zA-Z0-9_-]{10,}\.eyJ[a-zA-Z0-9_-]{10,}\.[a-zA-Z0-9_-]{10,}`),
			replacement: "[REDACTED_JWT]",
		},
		{
			name:     "database_url",
			alwaysOn: true,
			pattern: regexp.MustCompile(
				`(?i)(postgres(ql)?|mysql|mongodb(\+srv)?|redis)://[^\s]+:[^\s]+@[^\s]+`,
			),
			replacement: "[REDACTED_DATABASE_URL]",
		},
		{
			// generic_secret must be last so specific patterns (API keys, JWTs) match first.
			// Uses negative lookahead equivalent: \S+ that doesn't start with [REDACTED
			name:     "generic_secret",
			alwaysOn: false, // only when sensitivity > 0.5
			pattern: regexp.MustCompile(
				`(?i)(` +
					`password\s*[=:]\s*[^\s\[]\S*` +
					`|secret\s*[=:]\s*[^\s\[]\S*` +
					`|token\s*[=:]\s*[^\s\[]\S*` +
					`)`,
			),
			replacement: "[REDACTED_SECRET]",
		},
	}
}

// Scan checks content for credential patterns and returns a redacted version.
func (ld *LeakDetector) Scan(content string) LeakResult {
	var matched []string
	redacted := content

	for _, cat := range ld.categories {
		if !cat.alwaysOn && ld.sensitivity <= 0.5 {
			continue
		}
		if cat.pattern.MatchString(redacted) {
			matched = append(matched, cat.name)
			redacted = cat.pattern.ReplaceAllString(redacted, cat.replacement)
		}
	}

	return LeakResult{
		Clean:    len(matched) == 0,
		Patterns: matched,
		Redacted: redacted,
	}
}
