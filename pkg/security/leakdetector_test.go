package security

import (
	"strings"
	"testing"
)

func TestLeakDetector_APIKeys(t *testing.T) {
	ld := NewLeakDetector(0.7)

	tests := []struct {
		input string
		clean bool
		tag   string
	}{
		{"my stripe key is sk_live_abcdefghij1234567890", false, "[REDACTED_API_KEY]"},
		{"sk_test_abcdefghij1234567890 is the test key", false, "[REDACTED_API_KEY]"},
		{"openai key: sk-abcdefghij1234567890abcdef", false, "[REDACTED_API_KEY]"},
		{"anthropic key: sk-ant-abcdefghij1234567890_abc", false, "[REDACTED_API_KEY]"},
		{"google key: AIzaSyA1234567890abcdefghijklmnopqrstuvwx", false, "[REDACTED_API_KEY]"},
		{"github token: ghp_1234567890abcdefghijklmnopqrstuvwxyz", false, "[REDACTED_API_KEY]"},
		{"github pat: github_pat_1234567890abcdefghijkl", false, "[REDACTED_API_KEY]"},
		{"no keys here, just text", true, ""},
		{"sk-short", true, ""}, // too short
	}

	for _, tt := range tests {
		result := ld.Scan(tt.input)
		if result.Clean != tt.clean {
			t.Errorf("Scan(%q): got Clean=%v, want %v (patterns=%v)", tt.input, result.Clean, tt.clean, result.Patterns)
		}
		if tt.tag != "" && !strings.Contains(result.Redacted, tt.tag) {
			t.Errorf("Scan(%q): redacted %q does not contain %q", tt.input, result.Redacted, tt.tag)
		}
	}
}

func TestLeakDetector_AWSCredentials(t *testing.T) {
	ld := NewLeakDetector(0.7)

	tests := []struct {
		input string
		clean bool
		tag   string
	}{
		{"AKIAIOSFODNN7EXAMPLE is my access key", false, "[REDACTED_AWS_CREDENTIAL]"},
		{"aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", false, "[REDACTED_AWS_CREDENTIAL]"},
		{"aws-secret-access-key: mysecretkey123", false, "[REDACTED_AWS_CREDENTIAL]"},
		{"the AWS service is running fine", true, ""},
	}

	for _, tt := range tests {
		result := ld.Scan(tt.input)
		if result.Clean != tt.clean {
			t.Errorf("Scan(%q): got Clean=%v, want %v (patterns=%v)", tt.input, result.Clean, tt.clean, result.Patterns)
		}
		if tt.tag != "" && !strings.Contains(result.Redacted, tt.tag) {
			t.Errorf("Scan(%q): redacted does not contain %q", tt.input, tt.tag)
		}
	}
}

func TestLeakDetector_GenericSecrets(t *testing.T) {
	// With sensitivity > 0.5, generic secrets should be detected
	ld := NewLeakDetector(0.7)

	tests := []struct {
		input string
		clean bool
	}{
		{"password= mysecretpassword", false},
		{"secret= abcdef123456", false},
		{"token= eyJhbGciOiJIUzI1NiJ9", false},
		{"the password field is empty", true},
	}

	for _, tt := range tests {
		result := ld.Scan(tt.input)
		if result.Clean != tt.clean {
			t.Errorf("Scan(%q): got Clean=%v, want %v (patterns=%v)", tt.input, result.Clean, tt.clean, result.Patterns)
		}
	}

	// With sensitivity <= 0.5, generic secrets should NOT be detected
	ldLow := NewLeakDetector(0.5)
	result := ldLow.Scan("password= mysecretpassword")
	if !result.Clean {
		t.Error("expected Clean=true for generic secret at low sensitivity")
	}
}

func TestLeakDetector_PrivateKeys(t *testing.T) {
	ld := NewLeakDetector(0.7)

	tests := []struct {
		input string
		clean bool
		tag   string
	}{
		{"-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBg...", false, "[REDACTED_PRIVATE_KEY]"},
		{"-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCA...", false, "[REDACTED_PRIVATE_KEY]"},
		{"-----BEGIN EC PRIVATE KEY-----\nMHQCAQEEIBkg...", false, "[REDACTED_PRIVATE_KEY]"},
		{"-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC...", false, "[REDACTED_PRIVATE_KEY]"},
		{"-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqh...", true, ""},
	}

	for _, tt := range tests {
		result := ld.Scan(tt.input)
		if result.Clean != tt.clean {
			t.Errorf("Scan(%q): got Clean=%v, want %v", tt.input[:40], result.Clean, tt.clean)
		}
		if tt.tag != "" && !strings.Contains(result.Redacted, tt.tag) {
			t.Errorf("Scan: redacted does not contain %q", tt.tag)
		}
	}
}

func TestLeakDetector_JWT(t *testing.T) {
	ld := NewLeakDetector(0.7)

	jwt := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U"
	result := ld.Scan("auth token: " + jwt)
	if result.Clean {
		t.Error("expected leak detection for JWT")
	}
	if !strings.Contains(result.Redacted, "[REDACTED_JWT]") {
		t.Errorf("redacted %q does not contain JWT tag", result.Redacted)
	}

	result2 := ld.Scan("eyJ is just a base64 prefix")
	if !result2.Clean {
		t.Error("expected clean for non-JWT text")
	}
}

func TestLeakDetector_DatabaseURLs(t *testing.T) {
	ld := NewLeakDetector(0.7)

	tests := []struct {
		input string
		clean bool
		tag   string
	}{
		{"postgres://user:password@localhost:5432/db", false, "[REDACTED_DATABASE_URL]"},
		{"postgresql://admin:secret@db.example.com/prod", false, "[REDACTED_DATABASE_URL]"},
		{"mysql://root:pass123@127.0.0.1:3306/mydb", false, "[REDACTED_DATABASE_URL]"},
		{"mongodb://user:pw@cluster.mongodb.net/test", false, "[REDACTED_DATABASE_URL]"},
		{"mongodb+srv://user:pw@cluster.mongodb.net/test", false, "[REDACTED_DATABASE_URL]"},
		{"redis://default:mypassword@redis.example.com:6379", false, "[REDACTED_DATABASE_URL]"},
		{"the postgres database is running", true, ""},
	}

	for _, tt := range tests {
		result := ld.Scan(tt.input)
		if result.Clean != tt.clean {
			t.Errorf("Scan(%q): got Clean=%v, want %v (patterns=%v)", tt.input, result.Clean, tt.clean, result.Patterns)
		}
		if tt.tag != "" && !strings.Contains(result.Redacted, tt.tag) {
			t.Errorf("Scan(%q): redacted does not contain %q", tt.input, tt.tag)
		}
	}
}

func TestLeakDetector_MultipleLeaks(t *testing.T) {
	ld := NewLeakDetector(0.7)
	input := "Use sk-abcdefghij1234567890abcdef for OpenAI and postgres://admin:secret@db.example.com/prod for DB"
	result := ld.Scan(input)
	if result.Clean {
		t.Error("expected leaks detected for multi-pattern input")
	}
	if len(result.Patterns) < 2 {
		t.Errorf("expected at least 2 patterns, got %d: %v", len(result.Patterns), result.Patterns)
	}
	if !strings.Contains(result.Redacted, "[REDACTED_API_KEY]") {
		t.Error("missing API key redaction")
	}
	if !strings.Contains(result.Redacted, "[REDACTED_DATABASE_URL]") {
		t.Error("missing database URL redaction")
	}
}

func TestLeakDetector_CleanContent(t *testing.T) {
	ld := NewLeakDetector(0.7)
	result := ld.Scan("This is a normal response with no secrets.")
	if !result.Clean {
		t.Errorf("expected clean content, got patterns=%v", result.Patterns)
	}
	if result.Redacted != "This is a normal response with no secrets." {
		t.Error("redacted content should match original for clean input")
	}
}

func TestLeakDetector_RedactionPreservesContext(t *testing.T) {
	ld := NewLeakDetector(0.7)
	input := "Connect with postgres://user:password@localhost:5432/db and enjoy"
	result := ld.Scan(input)
	if !strings.HasPrefix(result.Redacted, "Connect with ") {
		t.Error("redaction should preserve surrounding text")
	}
	if !strings.HasSuffix(result.Redacted, " and enjoy") {
		t.Error("redaction should preserve surrounding text")
	}
}
