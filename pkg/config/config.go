package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/caarlos0/env/v11"
	"github.com/sipeed/picoclaw/pkg/secrets"
)

type SecretsConfig struct {
	Encrypt bool `json:"encrypt" env:"PICOCLAW_SECRETS_ENCRYPT"`
}

type Config struct {
	Agents    AgentsConfig    `json:"agents"`
	Channels  ChannelsConfig  `json:"channels"`
	Providers ProvidersConfig `json:"providers"`
	Gateway   GatewayConfig   `json:"gateway"`
	Tools     ToolsConfig     `json:"tools"`
	Heartbeat HeartbeatConfig `json:"heartbeat"`
	Memory    MemoryConfig    `json:"memory"`
	Cost      CostConfig      `json:"cost"`
	Secrets   SecretsConfig   `json:"secrets"`
	Security  SecurityConfig  `json:"security"`
	mu        sync.RWMutex
}

type SecurityConfig struct {
	PromptGuard      PromptGuardConfig      `json:"prompt_guard"`
	LeakDetector     LeakDetectorConfig     `json:"leak_detector"`
	PromptLeakGuard  PromptLeakGuardConfig  `json:"prompt_leak_guard"`
}

type PromptGuardConfig struct {
	Enabled     bool    `json:"enabled" env:"PICOCLAW_SECURITY_PROMPT_GUARD_ENABLED"`
	Action      string  `json:"action" env:"PICOCLAW_SECURITY_PROMPT_GUARD_ACTION"`
	Sensitivity float64 `json:"sensitivity" env:"PICOCLAW_SECURITY_PROMPT_GUARD_SENSITIVITY"`
}

type LeakDetectorConfig struct {
	Enabled     bool    `json:"enabled" env:"PICOCLAW_SECURITY_LEAK_DETECTOR_ENABLED"`
	Sensitivity float64 `json:"sensitivity" env:"PICOCLAW_SECURITY_LEAK_DETECTOR_SENSITIVITY"`
}

type PromptLeakGuardConfig struct {
	Enabled   bool    `json:"enabled" env:"PICOCLAW_SECURITY_PROMPT_LEAK_GUARD_ENABLED"`
	Threshold float64 `json:"threshold" env:"PICOCLAW_SECURITY_PROMPT_LEAK_GUARD_THRESHOLD"`
	Action    string  `json:"action" env:"PICOCLAW_SECURITY_PROMPT_LEAK_GUARD_ACTION"`
}

type ModelPriceConfig struct {
	Input  float64 `json:"input"`
	Output float64 `json:"output"`
}

type CostConfig struct {
	Enabled        bool                        `json:"enabled" env:"PICOCLAW_COST_ENABLED"`
	DailyLimitUSD  float64                     `json:"daily_limit_usd" env:"PICOCLAW_COST_DAILY_LIMIT_USD"`
	MonthlyLimitUSD float64                    `json:"monthly_limit_usd" env:"PICOCLAW_COST_MONTHLY_LIMIT_USD"`
	WarnAtPercent  float64                     `json:"warn_at_percent" env:"PICOCLAW_COST_WARN_AT_PERCENT"`
	Prices         map[string]ModelPriceConfig `json:"prices"`
}

type MemoryRetentionConfig struct {
	Daily        int `json:"daily" env:"PICOCLAW_MEMORY_RETENTION_DAILY"`
	Conversation int `json:"conversation" env:"PICOCLAW_MEMORY_RETENTION_CONVERSATION"`
	Custom       int `json:"custom" env:"PICOCLAW_MEMORY_RETENTION_CUSTOM"`
}

type MemoryConfig struct {
	RetentionDays  MemoryRetentionConfig `json:"retention_days"`
	SearchLimit    int                   `json:"search_limit" env:"PICOCLAW_MEMORY_SEARCH_LIMIT"`
	MinRelevance   float64               `json:"min_relevance" env:"PICOCLAW_MEMORY_MIN_RELEVANCE"`
	ContextTopK    int                   `json:"context_top_k" env:"PICOCLAW_MEMORY_CONTEXT_TOP_K"`
	SnapshotOnExit bool                  `json:"snapshot_on_exit" env:"PICOCLAW_MEMORY_SNAPSHOT_ON_EXIT"`
}

type HeartbeatConfig struct {
	Enabled         bool   `json:"enabled" env:"PICOCLAW_HEARTBEAT_ENABLED"`
	IntervalSeconds int    `json:"interval_seconds" env:"PICOCLAW_HEARTBEAT_INTERVAL_SECONDS"`
	Channel         string `json:"channel" env:"PICOCLAW_HEARTBEAT_CHANNEL"`
}

type AgentsConfig struct {
	Defaults AgentDefaults `json:"defaults"`
	List     []AgentConfig `json:"list,omitempty"`
}

type AgentConfig struct {
	ID                string           `json:"id"`
	Name              string           `json:"name,omitempty"`
	Description       string           `json:"description,omitempty"`
	Instructions      string           `json:"instructions,omitempty"`
	Context           []string         `json:"context,omitempty"`
	Workspace         string           `json:"workspace,omitempty"`
	Default           bool             `json:"default,omitempty"`
	Model             string           `json:"model,omitempty"`
	Provider          string           `json:"provider,omitempty"`
	MaxTokens         int              `json:"max_tokens,omitempty"`
	MaxToolIterations int              `json:"max_tool_iterations,omitempty"`
	Temperature       *float64         `json:"temperature,omitempty"`
	Skills            []string         `json:"skills,omitempty"`
	DeniedTools       []string         `json:"denied_tools,omitempty"`
	Subagents         *SubagentsConfig `json:"subagents,omitempty"`
}

type SubagentsConfig struct {
	AllowAgents []string `json:"allow_agents,omitempty"`
}

type AgentDefaults struct {
	Workspace         string  `json:"workspace" env:"PICOCLAW_AGENTS_DEFAULTS_WORKSPACE"`
	Model             string  `json:"model" env:"PICOCLAW_AGENTS_DEFAULTS_MODEL"`
	Provider          string  `json:"provider,omitempty" env:"PICOCLAW_AGENTS_DEFAULTS_PROVIDER"`
	MaxTokens         int     `json:"max_tokens" env:"PICOCLAW_AGENTS_DEFAULTS_MAX_TOKENS"`
	Temperature       float64 `json:"temperature" env:"PICOCLAW_AGENTS_DEFAULTS_TEMPERATURE"`
	MaxToolIterations int     `json:"max_tool_iterations" env:"PICOCLAW_AGENTS_DEFAULTS_MAX_TOOL_ITERATIONS"`
}

type ChannelsConfig struct {
	WhatsApp WhatsAppConfig `json:"whatsapp"`
	Telegram TelegramConfig `json:"telegram"`
	Feishu   FeishuConfig   `json:"feishu"`
	Discord  DiscordConfig  `json:"discord"`
	MaixCam  MaixCamConfig  `json:"maixcam"`
	QQ       QQConfig       `json:"qq"`
	DingTalk DingTalkConfig `json:"dingtalk"`
}

type WhatsAppConfig struct {
	Enabled   bool     `json:"enabled" env:"PICOCLAW_CHANNELS_WHATSAPP_ENABLED"`
	BridgeURL string   `json:"bridge_url" env:"PICOCLAW_CHANNELS_WHATSAPP_BRIDGE_URL"`
	AllowFrom []string `json:"allow_from" env:"PICOCLAW_CHANNELS_WHATSAPP_ALLOW_FROM"`
}

type TelegramConfig struct {
	Enabled        bool     `json:"enabled" env:"PICOCLAW_CHANNELS_TELEGRAM_ENABLED"`
	Token          string   `json:"token" env:"PICOCLAW_CHANNELS_TELEGRAM_TOKEN"`
	AllowFrom      []string `json:"allow_from" env:"PICOCLAW_CHANNELS_TELEGRAM_ALLOW_FROM"`
	TempAllowAgent string   `json:"temp_allow_agent,omitempty" env:"PICOCLAW_CHANNELS_TELEGRAM_TEMP_ALLOW_AGENT"`
}

type FeishuConfig struct {
	Enabled           bool     `json:"enabled" env:"PICOCLAW_CHANNELS_FEISHU_ENABLED"`
	AppID             string   `json:"app_id" env:"PICOCLAW_CHANNELS_FEISHU_APP_ID"`
	AppSecret         string   `json:"app_secret" env:"PICOCLAW_CHANNELS_FEISHU_APP_SECRET"`
	EncryptKey        string   `json:"encrypt_key" env:"PICOCLAW_CHANNELS_FEISHU_ENCRYPT_KEY"`
	VerificationToken string   `json:"verification_token" env:"PICOCLAW_CHANNELS_FEISHU_VERIFICATION_TOKEN"`
	AllowFrom         []string `json:"allow_from" env:"PICOCLAW_CHANNELS_FEISHU_ALLOW_FROM"`
}

type DiscordConfig struct {
	Enabled   bool     `json:"enabled" env:"PICOCLAW_CHANNELS_DISCORD_ENABLED"`
	Token     string   `json:"token" env:"PICOCLAW_CHANNELS_DISCORD_TOKEN"`
	AllowFrom []string `json:"allow_from" env:"PICOCLAW_CHANNELS_DISCORD_ALLOW_FROM"`
}

type MaixCamConfig struct {
	Enabled   bool     `json:"enabled" env:"PICOCLAW_CHANNELS_MAIXCAM_ENABLED"`
	Host      string   `json:"host" env:"PICOCLAW_CHANNELS_MAIXCAM_HOST"`
	Port      int      `json:"port" env:"PICOCLAW_CHANNELS_MAIXCAM_PORT"`
	AllowFrom []string `json:"allow_from" env:"PICOCLAW_CHANNELS_MAIXCAM_ALLOW_FROM"`
}

type QQConfig struct {
	Enabled   bool     `json:"enabled" env:"PICOCLAW_CHANNELS_QQ_ENABLED"`
	AppID     string   `json:"app_id" env:"PICOCLAW_CHANNELS_QQ_APP_ID"`
	AppSecret string   `json:"app_secret" env:"PICOCLAW_CHANNELS_QQ_APP_SECRET"`
	AllowFrom []string `json:"allow_from" env:"PICOCLAW_CHANNELS_QQ_ALLOW_FROM"`
}

type DingTalkConfig struct {
	Enabled          bool     `json:"enabled" env:"PICOCLAW_CHANNELS_DINGTALK_ENABLED"`
	ClientID         string   `json:"client_id" env:"PICOCLAW_CHANNELS_DINGTALK_CLIENT_ID"`
	ClientSecret     string   `json:"client_secret" env:"PICOCLAW_CHANNELS_DINGTALK_CLIENT_SECRET"`
	AllowFrom        []string `json:"allow_from" env:"PICOCLAW_CHANNELS_DINGTALK_ALLOW_FROM"`
}

// ProvidersConfig is a map of provider name to its configuration.
// New providers can be added purely through config.json.
type ProvidersConfig map[string]*ProviderConfig

type ProviderConfig struct {
	APIKey        string   `json:"api_key"`
	APIBase       string   `json:"api_base"`
	UserAgent     string   `json:"user_agent,omitempty"`
	ModelPatterns []string `json:"model_patterns,omitempty"`
	Fallback      bool     `json:"fallback,omitempty"`
}

// builtinProviderDefaults defines default API base URLs and model patterns
// for known providers. These are merged into user config at load time.
var builtinProviderDefaults = map[string]ProviderConfig{
	"anthropic": {
		APIBase:       "https://api.anthropic.com/v1",
		ModelPatterns: []string{"anthropic/", "claude"},
	},
	"openai": {
		APIBase:       "https://api.openai.com/v1",
		ModelPatterns: []string{"openai/", "gpt"},
	},
	"openrouter": {
		APIBase:       "https://openrouter.ai/api/v1",
		ModelPatterns: []string{"openrouter/", "meta-llama/", "deepseek/", "google/"},
		Fallback:      true,
	},
	"groq": {
		APIBase:       "https://api.groq.com/openai/v1",
		ModelPatterns: []string{"groq/", "groq"},
	},
	"zhipu": {
		APIBase:       "https://open.bigmodel.cn/api/paas/v4",
		ModelPatterns: []string{"glm", "zhipu", "zai"},
	},
	"vllm": {
		ModelPatterns: []string{},
	},
	"gemini": {
		APIBase:       "https://generativelanguage.googleapis.com/v1beta",
		ModelPatterns: []string{"gemini"},
	},
	"nvidia": {
		APIBase:       "https://integrate.api.nvidia.com/v1",
		ModelPatterns: []string{"nvidia/"},
	},
}

// mergeProviderDefaults fills in missing APIBase and ModelPatterns from
// builtinProviderDefaults for known providers. Unknown providers are left as-is.
func mergeProviderDefaults(providers ProvidersConfig) {
	for name, defaults := range builtinProviderDefaults {
		p, exists := providers[name]
		if !exists {
			continue
		}
		if p.APIBase == "" && defaults.APIBase != "" {
			p.APIBase = defaults.APIBase
		}
		if len(p.ModelPatterns) == 0 && len(defaults.ModelPatterns) > 0 {
			p.ModelPatterns = defaults.ModelPatterns
		}
		if defaults.Fallback && !p.Fallback {
			p.Fallback = defaults.Fallback
		}
	}
}

// GetProviderConfig returns the ProviderConfig for a given provider name, or nil.
func (c *Config) GetProviderConfig(name string) *ProviderConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Providers[name]
}

// ProviderNames returns sorted provider names.
func (c *Config) ProviderNames() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	names := make([]string, 0, len(c.Providers))
	for name := range c.Providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

type GatewayConfig struct {
	Host string `json:"host" env:"PICOCLAW_GATEWAY_HOST"`
	Port int    `json:"port" env:"PICOCLAW_GATEWAY_PORT"`
}

type WebSearchConfig struct {
	APIKey     string `json:"api_key" env:"PICOCLAW_TOOLS_WEB_SEARCH_API_KEY"`
	MaxResults int    `json:"max_results" env:"PICOCLAW_TOOLS_WEB_SEARCH_MAX_RESULTS"`
}

type OllamaConfig struct {
	APIKey     string `json:"api_key" env:"PICOCLAW_TOOLS_WEB_OLLAMA_API_KEY"`
	MaxResults int    `json:"max_results" env:"PICOCLAW_TOOLS_WEB_OLLAMA_MAX_RESULTS"`
}

type WebToolsConfig struct {
	Search WebSearchConfig `json:"search"`
	Ollama OllamaConfig    `json:"ollama"`
}

type ToolsConfig struct {
	Web                WebToolsConfig `json:"web"`
	RestrictToWorkspace *bool         `json:"restrict_to_workspace" env:"PICOCLAW_TOOLS_RESTRICT_TO_WORKSPACE"`
}

func DefaultConfig() *Config {
	return &Config{
		Agents: AgentsConfig{
			Defaults: AgentDefaults{
				Workspace:         "~/.picoclaw/workspace",
				Model:             "glm-4.7",
				MaxTokens:         8192,
				Temperature:       0.7,
				MaxToolIterations: 20,
			},
		},
		Channels: ChannelsConfig{
			WhatsApp: WhatsAppConfig{
				Enabled:   false,
				BridgeURL: "ws://localhost:3001",
				AllowFrom: []string{},
			},
			Telegram: TelegramConfig{
				Enabled:   false,
				Token:     "",
				AllowFrom: []string{},
			},
			Feishu: FeishuConfig{
				Enabled:           false,
				AppID:             "",
				AppSecret:         "",
				EncryptKey:        "",
				VerificationToken: "",
				AllowFrom:         []string{},
			},
			Discord: DiscordConfig{
				Enabled:   false,
				Token:     "",
				AllowFrom: []string{},
			},
			MaixCam: MaixCamConfig{
				Enabled:   false,
				Host:      "0.0.0.0",
				Port:      18790,
				AllowFrom: []string{},
			},
			QQ: QQConfig{
				Enabled:   false,
				AppID:     "",
				AppSecret: "",
				AllowFrom: []string{},
			},
			DingTalk: DingTalkConfig{
				Enabled:      false,
				ClientID:     "",
				ClientSecret: "",
				AllowFrom:    []string{},
			},
		},
		Providers: ProvidersConfig{
			"anthropic":  &ProviderConfig{},
			"openai":     &ProviderConfig{},
			"openrouter": &ProviderConfig{},
			"groq":       &ProviderConfig{},
			"zhipu":      &ProviderConfig{},
			"vllm":       &ProviderConfig{},
			"gemini":     &ProviderConfig{},
			"nvidia":     &ProviderConfig{},
		},
		Gateway: GatewayConfig{
			Host: "0.0.0.0",
			Port: 18790,
		},
		Heartbeat: HeartbeatConfig{
			Enabled:         false,
			IntervalSeconds: 1800,
			Channel:         "telegram",
		},
		Tools: ToolsConfig{
			Web: WebToolsConfig{
				Search: WebSearchConfig{
					APIKey:     "",
					MaxResults: 5,
				},
				Ollama: OllamaConfig{
					APIKey:     "",
					MaxResults: 5,
				},
			},
		},
		Memory: MemoryConfig{
			RetentionDays: MemoryRetentionConfig{
				Daily:        30,
				Conversation: 7,
				Custom:       90,
			},
			SearchLimit:    20,
			MinRelevance:   0.1,
			ContextTopK:    10,
			SnapshotOnExit: false,
		},
		Cost: CostConfig{
			Enabled:         false,
			DailyLimitUSD:   0,
			MonthlyLimitUSD: 0,
			WarnAtPercent:   80,
			Prices:          map[string]ModelPriceConfig{},
		},
		Secrets: SecretsConfig{
			Encrypt: false,
		},
		Security: SecurityConfig{
			PromptGuard: PromptGuardConfig{
				Enabled:     false,
				Action:      "warn",
				Sensitivity: 0.5,
			},
			LeakDetector: LeakDetectorConfig{
				Enabled:     false,
				Sensitivity: 0.7,
			},
			PromptLeakGuard: PromptLeakGuardConfig{
				Enabled:   false,
				Threshold: 0.15,
				Action:    "block",
			},
		},
	}
}

// sensitiveFields returns pointers to all sensitive string fields in the config.
// Provider API keys are collected dynamically from the providers map.
func sensitiveFields(cfg *Config) []*string {
	fields := []*string{
		&cfg.Channels.Telegram.Token,
		&cfg.Channels.Discord.Token,
		&cfg.Channels.Feishu.AppSecret,
		&cfg.Channels.Feishu.EncryptKey,
		&cfg.Channels.Feishu.VerificationToken,
		&cfg.Channels.QQ.AppSecret,
		&cfg.Channels.DingTalk.ClientSecret,
		&cfg.Tools.Web.Search.APIKey,
		&cfg.Tools.Web.Ollama.APIKey,
	}
	// Collect provider API keys in sorted order for deterministic encryption
	names := make([]string, 0, len(cfg.Providers))
	for name := range cfg.Providers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fields = append(fields, &cfg.Providers[name].APIKey)
	}
	return fields
}

func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Warning: config file not found at %s, using defaults\n", path)
			return cfg, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Ensure providers map is initialized (in case JSON had no providers section)
	if cfg.Providers == nil {
		cfg.Providers = make(ProvidersConfig)
	}

	// Fill in builtin defaults for known providers
	mergeProviderDefaults(cfg.Providers)

	// Check for encrypted and unencrypted sensitive fields
	hasEncrypted := false
	hasPlaintext := false
	for _, fp := range sensitiveFields(cfg) {
		if *fp == "" {
			continue
		}
		if secrets.IsEncrypted(*fp) {
			hasEncrypted = true
		} else {
			hasPlaintext = true
		}
	}

	// Decrypt any encrypted fields before env overrides
	if hasEncrypted {
		keyPath := filepath.Join(filepath.Dir(path), ".secret_key")
		store, err := secrets.NewSecretStore(keyPath)
		if err != nil {
			return nil, fmt.Errorf("config: init secret store: %w", err)
		}
		for _, fp := range sensitiveFields(cfg) {
			decrypted, err := store.Decrypt(*fp)
			if err != nil {
				return nil, fmt.Errorf("config: decrypt field: %w", err)
			}
			*fp = decrypted
		}
	}

	// Auto-encrypt: if encrypt is enabled and any sensitive field was plaintext, save back encrypted
	if cfg.Secrets.Encrypt && hasPlaintext {
		if err := SaveConfig(path, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to auto-encrypt config secrets: %v\n", err)
		}
	}

	if err := env.Parse(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func SaveConfig(path string, cfg *Config) error {
	cfg.mu.RLock()
	defer cfg.mu.RUnlock()

	toSave := cfg
	perm := os.FileMode(0644)

	if cfg.Secrets.Encrypt {
		// Clone via JSON to avoid mutating caller's config
		cloneData, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		var clone Config
		if err := json.Unmarshal(cloneData, &clone); err != nil {
			return err
		}

		keyPath := filepath.Join(filepath.Dir(path), ".secret_key")
		store, err := secrets.NewSecretStore(keyPath)
		if err != nil {
			return fmt.Errorf("config: init secret store: %w", err)
		}

		for _, fp := range sensitiveFields(&clone) {
			encrypted, err := store.Encrypt(*fp)
			if err != nil {
				return fmt.Errorf("config: encrypt field: %w", err)
			}
			*fp = encrypted
		}
		toSave = &clone
		perm = 0600
	}

	data, err := json.MarshalIndent(toSave, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(path, data, perm)
}

func (c *Config) WorkspacePath() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return expandHome(c.Agents.Defaults.Workspace)
}

// GetChannelAllowFrom returns the allow_from list for a given channel name.
func (c *Config) GetChannelAllowFrom(channel string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	switch channel {
	case "telegram":
		return c.Channels.Telegram.AllowFrom
	case "discord":
		return c.Channels.Discord.AllowFrom
	case "whatsapp":
		return c.Channels.WhatsApp.AllowFrom
	case "feishu":
		return c.Channels.Feishu.AllowFrom
	case "qq":
		return c.Channels.QQ.AllowFrom
	case "dingtalk":
		return c.Channels.DingTalk.AllowFrom
	case "maixcam":
		return c.Channels.MaixCam.AllowFrom
	default:
		return nil
	}
}

func (c *Config) IsRestrictToWorkspace() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.Tools.RestrictToWorkspace == nil {
		return true // default: restricted
	}
	return *c.Tools.RestrictToWorkspace
}

func expandHome(path string) string {
	if path == "" {
		return path
	}
	if path[0] == '~' {
		home, _ := os.UserHomeDir()
		if len(path) > 1 && path[1] == '/' {
			return home + path[1:]
		}
		return home
	}
	return path
}
