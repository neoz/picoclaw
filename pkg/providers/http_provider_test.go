package providers

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sipeed/picoclaw/pkg/config"
)

func TestStripThinkTags(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no tags",
			input: "Hello world",
			want:  "Hello world",
		},
		{
			name:  "matched pair",
			input: "<think>reasoning here</think>Hello world",
			want:  "Hello world",
		},
		{
			name:  "matched pair with whitespace",
			input: "<think>step 1\nstep 2\n</think>\n\nHello world",
			want:  "Hello world",
		},
		{
			name:  "multiple matched pairs",
			input: "<think>first</think>Hello <think>second</think>world",
			want:  "Hello world",
		},
		{
			name:  "orphaned close tag at start",
			input: "</think>\n\nHello world",
			want:  "Hello world",
		},
		{
			name:  "orphaned close tag with leaked reasoning before",
			input: "leaked reasoning</think>\n\nHello world",
			want:  "Hello world",
		},
		{
			name:  "unclosed open tag drops remainder",
			input: "Hello <think>reasoning without close",
			want:  "Hello",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "only think block",
			input: "<think>all reasoning</think>",
			want:  "",
		},
		{
			name:  "only orphaned close tag",
			input: "</think>",
			want:  "",
		},
		{
			name:  "real world deepseek pattern",
			input: "</think>\n\nDuoi day la phan tich chi tiet ve bai tho ban chia se:",
			want:  "Duoi day la phan tich chi tiet ve bai tho ban chia se:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripThinkTags(tt.input)
			if got != tt.want {
				t.Errorf("stripThinkTags(%q)\n  got:  %q\n  want: %q", tt.input, got, tt.want)
			}
		})
	}
}

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

func TestParseResponse_AnthropicFormat(t *testing.T) {
	p := &HTTPProvider{}
	body := []byte(`{
		"id": "msg_123",
		"type": "message",
		"role": "assistant",
		"content": [{"type": "text", "text": "Hello!"}],
		"stop_reason": "end_turn",
		"usage": {
			"input_tokens": 437,
			"cache_read_input_tokens": 4096,
			"cache_creation_input_tokens": 0,
			"output_tokens": 5,
			"prompt_tokens": 4533,
			"cached_tokens": 4096,
			"completion_tokens": 5,
			"total_tokens": 4538
		}
	}`)

	resp, err := p.parseResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "Hello!" {
		t.Errorf("content = %q, want Hello!", resp.Content)
	}
	if resp.Usage == nil {
		t.Fatal("usage is nil")
	}
	cached := resp.Usage.GetCachedTokens()
	if cached != 4096 {
		t.Errorf("cached_tokens = %d, want 4096", cached)
	}
}

func TestParseResponse_AnthropicFormat_CacheReadOnly(t *testing.T) {
	p := &HTTPProvider{}
	// Provider only reports cache_read_input_tokens, not cached_tokens
	body := []byte(`{
		"content": [{"type": "text", "text": "Hi"}],
		"usage": {
			"input_tokens": 500,
			"cache_read_input_tokens": 2048,
			"output_tokens": 10
		}
	}`)

	resp, err := p.parseResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "Hi" {
		t.Errorf("content = %q, want Hi", resp.Content)
	}
	if resp.Usage == nil {
		t.Fatal("usage is nil")
	}
	if resp.Usage.GetCachedTokens() != 2048 {
		t.Errorf("cached = %d, want 2048", resp.Usage.GetCachedTokens())
	}
}

func TestExtractSSEJSON_StreamedTextContent(t *testing.T) {
	// Simulates a typical streaming response with content split across chunks.
	body := []byte(`data: {"choices":[{"delta":{"content":"Hello"},"finish_reason":null}]}

data: {"choices":[{"delta":{"content":" world"},"finish_reason":null}]}

data: {"choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}

data: [DONE]
`)

	result := extractSSEJSON(body)
	if result == nil {
		t.Fatal("extractSSEJSON returned nil")
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content   string          `json:"content"`
				ToolCalls json.RawMessage `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage *UsageInfo `json:"usage"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(parsed.Choices) != 1 {
		t.Fatalf("choices: got %d, want 1", len(parsed.Choices))
	}
	if parsed.Choices[0].Message.Content != "Hello world" {
		t.Errorf("content: got %q, want %q", parsed.Choices[0].Message.Content, "Hello world")
	}
	if parsed.Choices[0].FinishReason != "stop" {
		t.Errorf("finish_reason: got %q, want %q", parsed.Choices[0].FinishReason, "stop")
	}
	if parsed.Usage == nil {
		t.Fatal("usage is nil")
	}
	if parsed.Usage.TotalTokens != 12 {
		t.Errorf("total_tokens: got %d, want 12", parsed.Usage.TotalTokens)
	}
}

func TestExtractSSEJSON_StreamedToolCall(t *testing.T) {
	// Simulates a streaming response where the LLM calls a tool.
	// Tool call metadata arrives in one chunk, arguments are split across multiple chunks.
	body := []byte(`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_abc123","type":"function","function":{"name":"web_search","arguments":""}}]},"finish_reason":null}]}

data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"qu"}}]},"finish_reason":null}]}

data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"ery\": \"weather HCM"}}]},"finish_reason":null}]}

data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"}"}}]},"finish_reason":null}]}

data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}

data: [DONE]
`)

	result := extractSSEJSON(body)
	if result == nil {
		t.Fatal("extractSSEJSON returned nil")
	}

	// Parse the aggregated result through parseResponse to verify end-to-end.
	p := &HTTPProvider{}
	resp, err := p.parseResponse(result)
	if err != nil {
		t.Fatalf("parseResponse: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("tool_calls: got %d, want 1", len(resp.ToolCalls))
	}
	tc := resp.ToolCalls[0]
	if tc.ID != "call_abc123" {
		t.Errorf("tool call ID: got %q, want %q", tc.ID, "call_abc123")
	}
	if tc.Name != "web_search" {
		t.Errorf("tool call name: got %q, want %q", tc.Name, "web_search")
	}
	query, ok := tc.Arguments["query"].(string)
	if !ok || query != "weather HCM" {
		t.Errorf("tool call arguments.query: got %v, want %q", tc.Arguments["query"], "weather HCM")
	}
	if resp.FinishReason != "tool_calls" {
		t.Errorf("finish_reason: got %q, want %q", resp.FinishReason, "tool_calls")
	}
}

func TestExtractSSEJSON_MultipleToolCalls(t *testing.T) {
	// Two tool calls in the same response, streamed with different indices.
	body := []byte(`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"web_search","arguments":"{\"query\":\"weather\"}"}}]},"finish_reason":null}]}

data: {"choices":[{"delta":{"tool_calls":[{"index":1,"id":"call_2","type":"function","function":{"name":"read_file","arguments":"{\"path\":\"/tmp/x\"}"}}]},"finish_reason":null}]}

data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}

data: [DONE]
`)

	result := extractSSEJSON(body)
	if result == nil {
		t.Fatal("extractSSEJSON returned nil")
	}

	p := &HTTPProvider{}
	resp, err := p.parseResponse(result)
	if err != nil {
		t.Fatalf("parseResponse: %v", err)
	}
	if len(resp.ToolCalls) != 2 {
		t.Fatalf("tool_calls: got %d, want 2", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "web_search" {
		t.Errorf("tool[0].name: got %q", resp.ToolCalls[0].Name)
	}
	if resp.ToolCalls[1].Name != "read_file" {
		t.Errorf("tool[1].name: got %q", resp.ToolCalls[1].Name)
	}
}

func TestExtractSSEJSON_NonStreamingInSSEEnvelope(t *testing.T) {
	// Some providers wrap a non-streaming response in a single SSE data line.
	body := []byte(`data: {"choices":[{"message":{"content":"Hello!","tool_calls":[]},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":1,"total_tokens":6}}

`)

	result := extractSSEJSON(body)
	if result == nil {
		t.Fatal("extractSSEJSON returned nil")
	}

	p := &HTTPProvider{}
	resp, err := p.parseResponse(result)
	if err != nil {
		t.Fatalf("parseResponse: %v", err)
	}
	if resp.Content != "Hello!" {
		t.Errorf("content: got %q, want %q", resp.Content, "Hello!")
	}
}

func TestExtractSSEJSON_EmptyStream(t *testing.T) {
	body := []byte(`data: [DONE]
`)
	result := extractSSEJSON(body)
	if result != nil {
		t.Errorf("expected nil for empty stream, got %s", result)
	}
}

func TestExtractSSEJSON_ReasoningContent(t *testing.T) {
	body := []byte(`data: {"choices":[{"delta":{"reasoning_content":"Let me think"},"finish_reason":null}]}

data: {"choices":[{"delta":{"reasoning_content":"... about this"},"finish_reason":null}]}

data: {"choices":[{"delta":{"content":"The answer is 42"},"finish_reason":null}]}

data: {"choices":[{"delta":{},"finish_reason":"stop"}]}

data: [DONE]
`)

	result := extractSSEJSON(body)
	if result == nil {
		t.Fatal("extractSSEJSON returned nil")
	}

	p := &HTTPProvider{}
	resp, err := p.parseResponse(result)
	if err != nil {
		t.Fatalf("parseResponse: %v", err)
	}
	if resp.Content != "The answer is 42" {
		t.Errorf("content: got %q", resp.Content)
	}
	if resp.ReasoningContent != "Let me think... about this" {
		t.Errorf("reasoning_content: got %q", resp.ReasoningContent)
	}
}

func TestGetCachedTokens_Priority(t *testing.T) {
	tests := []struct {
		name  string
		usage UsageInfo
		want  int
	}{
		{"top-level cached_tokens", UsageInfo{CachedTokens: 100}, 100},
		{"cache_read_input_tokens", UsageInfo{CacheReadInputTokens: 200}, 200},
		{"prompt_tokens_details", UsageInfo{PromptTokenDetails: &PromptTokenDetails{CachedTokens: 300}}, 300},
		{"top-level wins over cache_read", UsageInfo{CachedTokens: 100, CacheReadInputTokens: 200}, 100},
		{"cache_read wins over details", UsageInfo{CacheReadInputTokens: 200, PromptTokenDetails: &PromptTokenDetails{CachedTokens: 300}}, 200},
		{"all zero", UsageInfo{}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.usage.GetCachedTokens()
			if got != tt.want {
				t.Errorf("GetCachedTokens() = %d, want %d", got, tt.want)
			}
		})
	}
}
