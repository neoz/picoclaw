package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestProvidersConfig_UnmarshalJSON(t *testing.T) {
	jsonData := `{
		"providers": {
			"anthropic": {"api_key": "sk-ant"},
			"openrouter": {"api_key": "sk-or", "api_base": "https://custom.api/v1"},
			"kimi": {"api_key": "sk-kimi", "api_base": "https://api.moonshot.cn/v1", "model_patterns": ["kimi", "moonshot"]}
		}
	}`

	cfg := DefaultConfig()
	if err := json.Unmarshal([]byte(jsonData), cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if p := cfg.Providers["anthropic"]; p == nil || p.APIKey != "sk-ant" {
		t.Error("anthropic provider not unmarshaled correctly")
	}
	if p := cfg.Providers["openrouter"]; p == nil || p.APIBase != "https://custom.api/v1" {
		t.Error("openrouter custom api_base not preserved")
	}
	if p := cfg.Providers["kimi"]; p == nil {
		t.Fatal("kimi provider missing after unmarshal")
	} else {
		if p.APIKey != "sk-kimi" {
			t.Error("kimi api_key wrong")
		}
		if len(p.ModelPatterns) != 2 {
			t.Errorf("kimi model_patterns: got %d, want 2", len(p.ModelPatterns))
		}
	}
}

func TestMergeProviderDefaults_FillsAPIBase(t *testing.T) {
	providers := ProvidersConfig{
		"anthropic": &ProviderConfig{APIKey: "sk-ant"},
		"openai":    &ProviderConfig{APIKey: "sk-oai"},
	}
	mergeProviderDefaults(providers)

	if providers["anthropic"].APIBase != "https://api.anthropic.com/v1" {
		t.Errorf("anthropic APIBase: got %q, want default", providers["anthropic"].APIBase)
	}
	if providers["openai"].APIBase != "https://api.openai.com/v1" {
		t.Errorf("openai APIBase: got %q, want default", providers["openai"].APIBase)
	}
}

func TestMergeProviderDefaults_PreservesCustomAPIBase(t *testing.T) {
	custom := "https://my-proxy.example.com/v1"
	providers := ProvidersConfig{
		"anthropic": &ProviderConfig{APIKey: "sk-ant", APIBase: custom},
	}
	mergeProviderDefaults(providers)

	if providers["anthropic"].APIBase != custom {
		t.Errorf("custom APIBase overwritten: got %q, want %q", providers["anthropic"].APIBase, custom)
	}
}

func TestMergeProviderDefaults_FillsModelPatterns(t *testing.T) {
	providers := ProvidersConfig{
		"anthropic": &ProviderConfig{APIKey: "sk-ant"},
	}
	mergeProviderDefaults(providers)

	patterns := providers["anthropic"].ModelPatterns
	if len(patterns) == 0 {
		t.Fatal("anthropic ModelPatterns not filled from defaults")
	}
	// Should contain "anthropic/" and "claude"
	found := map[string]bool{}
	for _, p := range patterns {
		found[p] = true
	}
	if !found["anthropic/"] || !found["claude"] {
		t.Errorf("unexpected patterns: %v", patterns)
	}
}

func TestMergeProviderDefaults_PreservesCustomPatterns(t *testing.T) {
	providers := ProvidersConfig{
		"anthropic": &ProviderConfig{
			APIKey:        "sk-ant",
			ModelPatterns: []string{"my-custom-pattern"},
		},
	}
	mergeProviderDefaults(providers)

	if len(providers["anthropic"].ModelPatterns) != 1 || providers["anthropic"].ModelPatterns[0] != "my-custom-pattern" {
		t.Errorf("custom ModelPatterns overwritten: %v", providers["anthropic"].ModelPatterns)
	}
}

func TestMergeProviderDefaults_FillsFallback(t *testing.T) {
	providers := ProvidersConfig{
		"openrouter": &ProviderConfig{APIKey: "sk-or"},
	}
	mergeProviderDefaults(providers)

	if !providers["openrouter"].Fallback {
		t.Error("openrouter Fallback not set from defaults")
	}
}

func TestMergeProviderDefaults_IgnoresUnknownProviders(t *testing.T) {
	providers := ProvidersConfig{
		"kimi": &ProviderConfig{
			APIKey:        "sk-kimi",
			APIBase:       "https://api.moonshot.cn/v1",
			ModelPatterns: []string{"kimi"},
		},
	}
	mergeProviderDefaults(providers)

	// Unknown provider should be untouched
	if providers["kimi"].APIBase != "https://api.moonshot.cn/v1" {
		t.Error("unknown provider APIBase changed")
	}
	if len(providers["kimi"].ModelPatterns) != 1 {
		t.Error("unknown provider ModelPatterns changed")
	}
}

func TestGetProviderConfig(t *testing.T) {
	cfg := DefaultConfig()
	mergeProviderDefaults(cfg.Providers)

	if p := cfg.GetProviderConfig("anthropic"); p == nil {
		t.Error("GetProviderConfig(anthropic) returned nil")
	}
	if p := cfg.GetProviderConfig("nonexistent"); p != nil {
		t.Error("GetProviderConfig(nonexistent) should return nil")
	}
}

func TestProviderNames_Sorted(t *testing.T) {
	cfg := DefaultConfig()
	names := cfg.ProviderNames()

	if len(names) != 8 {
		t.Fatalf("ProviderNames count: got %d, want 8", len(names))
	}
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("names not sorted: %v", names)
			break
		}
	}
}

func TestSensitiveFields_IncludesProviderKeys(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Providers["anthropic"].APIKey = "test-key"

	fields := sensitiveFields(cfg)

	found := false
	for _, fp := range fields {
		if *fp == "test-key" {
			found = true
			break
		}
	}
	if !found {
		t.Error("sensitiveFields missing provider API key")
	}
}

func TestSensitiveFields_DynamicProviders(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Providers["kimi"] = &ProviderConfig{APIKey: "kimi-key"}

	fields := sensitiveFields(cfg)

	found := false
	for _, fp := range fields {
		if *fp == "kimi-key" {
			found = true
			break
		}
	}
	if !found {
		t.Error("sensitiveFields missing dynamically added provider key")
	}
}

func TestSensitiveFields_MutatesProviderKey(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Providers["anthropic"].APIKey = "plaintext"

	fields := sensitiveFields(cfg)
	for _, fp := range fields {
		if *fp == "plaintext" {
			*fp = "encrypted"
			break
		}
	}

	if cfg.Providers["anthropic"].APIKey != "encrypted" {
		t.Error("sensitiveFields pointer did not mutate provider key")
	}
}

func TestLoadConfig_MergesDefaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	// Minimal config with just an API key, no api_base or model_patterns
	data := `{"providers":{"anthropic":{"api_key":"sk-test"}}}`
	if err := os.WriteFile(cfgPath, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	p := cfg.Providers["anthropic"]
	if p == nil {
		t.Fatal("anthropic provider missing")
	}
	if p.APIKey != "sk-test" {
		t.Errorf("APIKey: got %q, want sk-test", p.APIKey)
	}
	if p.APIBase == "" {
		t.Error("APIBase not filled from defaults")
	}
	if len(p.ModelPatterns) == 0 {
		t.Error("ModelPatterns not filled from defaults")
	}
}

func TestLoadConfig_CustomProviderPreserved(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	data := `{"providers":{"kimi":{"api_key":"sk-kimi","api_base":"https://api.moonshot.cn/v1","model_patterns":["kimi","moonshot"]}}}`
	if err := os.WriteFile(cfgPath, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	p := cfg.Providers["kimi"]
	if p == nil {
		t.Fatal("kimi provider missing")
	}
	if p.APIKey != "sk-kimi" {
		t.Errorf("APIKey: got %q", p.APIKey)
	}
	if p.APIBase != "https://api.moonshot.cn/v1" {
		t.Errorf("APIBase: got %q", p.APIBase)
	}
	if len(p.ModelPatterns) != 2 {
		t.Errorf("ModelPatterns count: got %d, want 2", len(p.ModelPatterns))
	}
}

func TestLoadConfig_MissingFile_ReturnsDefaults(t *testing.T) {
	cfg, err := LoadConfig(filepath.Join(t.TempDir(), "nonexistent.json"))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Providers == nil {
		t.Fatal("Providers nil on missing file")
	}
	if len(cfg.Providers) != 8 {
		t.Errorf("default providers count: got %d, want 8", len(cfg.Providers))
	}
}

func TestSaveConfig_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	cfg := DefaultConfig()
	cfg.Providers["kimi"] = &ProviderConfig{
		APIKey:        "sk-kimi",
		APIBase:       "https://api.moonshot.cn/v1",
		ModelPatterns: []string{"kimi", "moonshot"},
	}

	if err := SaveConfig(cfgPath, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	loaded, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	p := loaded.Providers["kimi"]
	if p == nil {
		t.Fatal("kimi provider missing after round-trip")
	}
	if p.APIKey != "sk-kimi" {
		t.Errorf("APIKey after round-trip: got %q", p.APIKey)
	}
	if p.APIBase != "https://api.moonshot.cn/v1" {
		t.Errorf("APIBase after round-trip: got %q", p.APIBase)
	}
}

func TestDefaultConfig_AllKnownProviders(t *testing.T) {
	cfg := DefaultConfig()
	known := []string{"anthropic", "openai", "openrouter", "groq", "zhipu", "vllm", "gemini", "nvidia"}
	for _, name := range known {
		if _, ok := cfg.Providers[name]; !ok {
			t.Errorf("DefaultConfig missing provider %q", name)
		}
	}
}
