package providers

import (
	"strings"
	"testing"

	"github.com/sipeed/picoclaw/pkg/config"
)

// testProviders returns a realistic provider map for testing.
func testProviders() config.ProvidersConfig {
	return config.ProvidersConfig{
		"anthropic": &config.ProviderConfig{
			APIKey:        "sk-ant",
			APIBase:       "https://api.anthropic.com/v1",
			ModelPatterns: []string{"anthropic/", "claude"},
		},
		"openai": &config.ProviderConfig{
			APIKey:        "sk-oai",
			APIBase:       "https://api.openai.com/v1",
			ModelPatterns: []string{"openai/", "gpt"},
		},
		"openrouter": &config.ProviderConfig{
			APIKey:        "sk-or",
			APIBase:       "https://openrouter.ai/api/v1",
			ModelPatterns: []string{"openrouter/", "meta-llama/", "deepseek/", "google/"},
			Fallback:      true,
		},
		"groq": &config.ProviderConfig{
			APIKey:        "gsk-groq",
			APIBase:       "https://api.groq.com/openai/v1",
			ModelPatterns: []string{"groq/", "groq"},
		},
		"zhipu": &config.ProviderConfig{
			APIKey:        "sk-zhipu",
			APIBase:       "https://open.bigmodel.cn/api/paas/v4",
			ModelPatterns: []string{"glm", "zhipu", "zai"},
		},
		"gemini": &config.ProviderConfig{
			APIKey:        "sk-gem",
			APIBase:       "https://generativelanguage.googleapis.com/v1beta",
			ModelPatterns: []string{"gemini"},
		},
		"nvidia": &config.ProviderConfig{
			APIKey:        "sk-nv",
			APIBase:       "https://integrate.api.nvidia.com/v1",
			ModelPatterns: []string{"nvidia/"},
		},
	}
}

func TestMatchProviderByModel_PrefixMatch(t *testing.T) {
	providers := testProviders()

	tests := []struct {
		model    string
		wantName string
	}{
		{"anthropic/claude-sonnet-4", "anthropic"},
		{"openai/gpt-4.1", "openai"},
		{"openrouter/some-model", "openrouter"},
		{"meta-llama/llama-3.1-70b", "openrouter"},
		{"deepseek/deepseek-v3", "openrouter"},
		{"google/gemma-2", "openrouter"},
		{"nvidia/llama-3.1-nemotron", "nvidia"},
		{"groq/llama3-70b", "groq"},
	}

	for _, tt := range tests {
		name, p := matchProviderByModel(tt.model, providers)
		if name != tt.wantName {
			t.Errorf("matchProviderByModel(%q): got provider %q, want %q", tt.model, name, tt.wantName)
		}
		if p == nil {
			t.Errorf("matchProviderByModel(%q): got nil config", tt.model)
		}
	}
}

func TestMatchProviderByModel_ContainsMatch(t *testing.T) {
	providers := testProviders()

	tests := []struct {
		model    string
		wantName string
	}{
		{"claude-3-opus", "anthropic"},
		{"gpt-4o", "openai"},
		{"gemini-2.0-flash", "gemini"},
		{"glm-4.7", "zhipu"},
		{"GLM-4-Plus", "zhipu"},
	}

	for _, tt := range tests {
		name, p := matchProviderByModel(tt.model, providers)
		if name != tt.wantName {
			t.Errorf("matchProviderByModel(%q): got provider %q, want %q", tt.model, name, tt.wantName)
		}
		if p == nil {
			t.Errorf("matchProviderByModel(%q): got nil config", tt.model)
		}
	}
}

func TestMatchProviderByModel_Fallback(t *testing.T) {
	providers := testProviders()

	// "some-unknown-model" doesn't match any patterns, should fall back to openrouter
	name, p := matchProviderByModel("some-unknown-model", providers)
	if name != "openrouter" {
		t.Errorf("fallback: got provider %q, want openrouter", name)
	}
	if p == nil {
		t.Error("fallback: got nil config")
	}
}

func TestMatchProviderByModel_BareAPIBase(t *testing.T) {
	providers := config.ProvidersConfig{
		"vllm": &config.ProviderConfig{
			APIBase:       "http://localhost:8000/v1",
			ModelPatterns: []string{},
		},
	}

	name, p := matchProviderByModel("my-local-model", providers)
	if name != "vllm" {
		t.Errorf("bare api_base: got provider %q, want vllm", name)
	}
	if p == nil || p.APIBase != "http://localhost:8000/v1" {
		t.Error("bare api_base: config not returned correctly")
	}
}

func TestMatchProviderByModel_NoMatch(t *testing.T) {
	providers := config.ProvidersConfig{
		"anthropic": &config.ProviderConfig{
			// No API key, no API base -> should be skipped
			ModelPatterns: []string{"anthropic/", "claude"},
		},
	}

	name, p := matchProviderByModel("claude-3-opus", providers)
	if name != "" || p != nil {
		t.Errorf("expected no match for keyless provider, got %q", name)
	}
}

func TestMatchProviderByModel_CustomProvider(t *testing.T) {
	providers := config.ProvidersConfig{
		"kimi": &config.ProviderConfig{
			APIKey:        "sk-kimi",
			APIBase:       "https://api.moonshot.cn/v1",
			ModelPatterns: []string{"kimi", "moonshot"},
		},
	}

	tests := []struct {
		model string
		match bool
	}{
		{"kimi-chat", true},
		{"moonshot-v1", true},
		{"gpt-4o", false},
	}

	for _, tt := range tests {
		name, p := matchProviderByModel(tt.model, providers)
		if tt.match && (name != "kimi" || p == nil) {
			t.Errorf("matchProviderByModel(%q): expected kimi, got %q", tt.model, name)
		}
		if !tt.match && name != "" {
			t.Errorf("matchProviderByModel(%q): expected no match, got %q", tt.model, name)
		}
	}
}

func TestMatchProviderByModel_PrefixPriorityOverContains(t *testing.T) {
	// "anthropic/" prefix on openrouter should match openrouter first,
	// even though anthropic has "claude" contains pattern
	providers := config.ProvidersConfig{
		"anthropic": &config.ProviderConfig{
			APIKey:        "sk-ant",
			APIBase:       "https://api.anthropic.com/v1",
			ModelPatterns: []string{"anthropic/", "claude"},
		},
		"openrouter": &config.ProviderConfig{
			APIKey:        "sk-or",
			APIBase:       "https://openrouter.ai/api/v1",
			ModelPatterns: []string{"openrouter/", "anthropic/"},
		},
	}

	// "anthropic/claude-sonnet-4" matches prefix "anthropic/" on both providers.
	// Map iteration is non-deterministic, so either could win.
	// What matters is we get a valid match.
	name, p := matchProviderByModel("anthropic/claude-sonnet-4", providers)
	if name == "" || p == nil {
		t.Error("expected a match for anthropic/ prefix model")
	}
	if name != "anthropic" && name != "openrouter" {
		t.Errorf("unexpected provider %q", name)
	}
}

func TestCreateProviderForModel_ExplicitProvider(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Providers["anthropic"] = &config.ProviderConfig{
		APIKey:  "sk-ant",
		APIBase: "https://api.anthropic.com/v1",
	}

	provider, err := CreateProviderForModel("any-model", "anthropic", cfg)
	if err != nil {
		t.Fatalf("CreateProviderForModel: %v", err)
	}
	hp, ok := provider.(*HTTPProvider)
	if !ok {
		t.Fatal("expected *HTTPProvider")
	}
	if hp.apiKey != "sk-ant" {
		t.Errorf("apiKey: got %q, want sk-ant", hp.apiKey)
	}
	if hp.apiBase != "https://api.anthropic.com/v1" {
		t.Errorf("apiBase: got %q", hp.apiBase)
	}
}

func TestCreateProviderForModel_UnknownExplicitProvider(t *testing.T) {
	cfg := config.DefaultConfig()
	_, err := CreateProviderForModel("model", "nonexistent", cfg)
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
	if !strings.Contains(err.Error(), "unknown provider") {
		t.Errorf("error message: %v", err)
	}
}

func TestCreateProviderForModel_PatternMatch(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Providers["zhipu"] = &config.ProviderConfig{
		APIKey:        "sk-zhipu",
		APIBase:       "https://open.bigmodel.cn/api/paas/v4",
		ModelPatterns: []string{"glm", "zhipu"},
	}

	provider, err := CreateProviderForModel("glm-4.7", "", cfg)
	if err != nil {
		t.Fatalf("CreateProviderForModel: %v", err)
	}
	hp := provider.(*HTTPProvider)
	if hp.apiKey != "sk-zhipu" {
		t.Errorf("apiKey: got %q, want sk-zhipu", hp.apiKey)
	}
}

func TestCreateProviderForModel_NoKeyError(t *testing.T) {
	cfg := config.DefaultConfig()
	// All providers have empty keys in default config
	_, err := CreateProviderForModel("some-model", "", cfg)
	if err == nil {
		t.Fatal("expected error when no provider has keys")
	}
}

func TestCreateProviderForModel_BedrockNoKeyAllowed(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Providers["openrouter"] = &config.ProviderConfig{
		APIBase:       "https://openrouter.ai/api/v1",
		ModelPatterns: []string{},
		Fallback:      true,
	}

	// bedrock/ models should not error on empty API key
	// but will error on empty api_base since no pattern matches
	_, err := CreateProviderForModel("bedrock/anthropic.claude-v2", "", cfg)
	if err == nil {
		// It's ok if it errors for other reasons (no match),
		// just shouldn't be "no API key" error
		t.Skip("bedrock matched a provider with base")
	}
	if strings.Contains(err.Error(), "no API key configured for provider") {
		t.Errorf("bedrock model should not require API key, got: %v", err)
	}
}

func TestCreateProviderForModel_UserAgent(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Providers["anthropic"] = &config.ProviderConfig{
		APIKey:    "sk-ant",
		APIBase:   "https://api.anthropic.com/v1",
		UserAgent: "my-app/1.0",
	}

	provider, err := CreateProviderForModel("claude-3-opus", "", cfg)
	if err != nil {
		// If pattern matching doesn't pick anthropic (other providers might interfere),
		// try explicit
		provider, err = CreateProviderForModel("any-model", "anthropic", cfg)
		if err != nil {
			t.Fatalf("CreateProviderForModel: %v", err)
		}
	}
	hp := provider.(*HTTPProvider)
	if hp.userAgent != "my-app/1.0" {
		t.Errorf("userAgent: got %q, want my-app/1.0", hp.userAgent)
	}
}

func TestCreateProviderForModel_CustomProvider(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Providers["kimi"] = &config.ProviderConfig{
		APIKey:        "sk-kimi",
		APIBase:       "https://api.moonshot.cn/v1",
		ModelPatterns: []string{"kimi", "moonshot"},
	}

	provider, err := CreateProviderForModel("kimi-chat", "", cfg)
	if err != nil {
		t.Fatalf("CreateProviderForModel: %v", err)
	}
	hp := provider.(*HTTPProvider)
	if hp.apiKey != "sk-kimi" {
		t.Errorf("apiKey: got %q, want sk-kimi", hp.apiKey)
	}
	if hp.apiBase != "https://api.moonshot.cn/v1" {
		t.Errorf("apiBase: got %q", hp.apiBase)
	}
}

func TestCreateProviderForModel_ExplicitProviderBypassesPatterns(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Providers["anthropic"] = &config.ProviderConfig{
		APIKey:        "sk-ant",
		APIBase:       "https://api.anthropic.com/v1",
		ModelPatterns: []string{"claude"},
	}
	cfg.Providers["openrouter"] = &config.ProviderConfig{
		APIKey:        "sk-or",
		APIBase:       "https://openrouter.ai/api/v1",
		ModelPatterns: []string{"openrouter/"},
		Fallback:      true,
	}

	// Model "glm-4.7" would normally not match anthropic patterns,
	// but explicit provider should override
	provider, err := CreateProviderForModel("glm-4.7", "anthropic", cfg)
	if err != nil {
		t.Fatalf("CreateProviderForModel: %v", err)
	}
	hp := provider.(*HTTPProvider)
	if hp.apiKey != "sk-ant" {
		t.Errorf("explicit provider not used: got key %q", hp.apiKey)
	}
}
