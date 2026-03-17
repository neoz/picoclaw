package security

import (
	"encoding/base64"
	"regexp"
	"strings"
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
		// PII patterns (active when sensitivity > 0.5)
		{
			name:        "email",
			alwaysOn:    false,
			pattern:     regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`),
			replacement: "[REDACTED_EMAIL]",
		},
		{
			name:     "phone",
			alwaysOn: false,
			pattern: regexp.MustCompile(
				`(?:^|\s)(` +
					`\+?1?\s*\(?\d{3}\)?[\s.\-]?\d{3}[\s.\-]?\d{4}` + // US/CA
					`|\+\d{1,3}[\s.\-]?\d{4,14}` + // international
					`)(?:\s|$|[.,;])`,
			),
			replacement: " [REDACTED_PHONE] ",
		},
		{
			name:        "ssn",
			alwaysOn:    false,
			pattern:     regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
			replacement: "[REDACTED_SSN]",
		},
		{
			name:        "credit_card",
			alwaysOn:    false,
			pattern:     regexp.MustCompile(`\b(?:\d{4}[\s\-]?){3}\d{4}\b`),
			replacement: "[REDACTED_CARD]",
		},
	}
}

// base64Pattern matches base64 strings long enough to potentially contain secrets (>=20 chars).
var base64Pattern = regexp.MustCompile(`[A-Za-z0-9+/]{20,}={0,2}`)

// Scan checks content for credential patterns and returns a redacted version.
// Also detects secrets hidden inside base64-encoded strings.
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

	// Check for secrets hidden in base64-encoded strings.
	if b64Matches := base64Pattern.FindAllString(redacted, 10); len(b64Matches) > 0 {
		for _, b64 := range b64Matches {
			decoded, err := base64.StdEncoding.DecodeString(b64)
			if err != nil {
				decoded, err = base64.RawStdEncoding.DecodeString(b64)
			}
			if err != nil || len(decoded) < 10 {
				continue
			}
			decodedStr := string(decoded)
			// Check if decoded content contains any secret patterns.
			for _, cat := range ld.categories {
				if !cat.alwaysOn && ld.sensitivity <= 0.5 {
					continue
				}
				if cat.pattern.MatchString(decodedStr) {
					matched = append(matched, "base64:"+cat.name)
					redacted = strings.Replace(redacted, b64, "[REDACTED_BASE64_"+strings.ToUpper(cat.name)+"]", 1)
					break
				}
			}
		}
	}

	return LeakResult{
		Clean:    len(matched) == 0,
		Patterns: matched,
		Redacted: redacted,
	}
}
