package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	PromptGuard  PromptGuardConfig  `json:"prompt_guard"`
	LeakDetector LeakDetectorConfig `json:"leak_detector"`
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
	AutoSave       bool                  `json:"auto_save" env:"PICOCLAW_MEMORY_AUTO_SAVE"`
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
	Workspace         string           `json:"workspace,omitempty"`
	Default           bool             `json:"default,omitempty"`
	Model             string           `json:"model,omitempty"`
	MaxTokens         int              `json:"max_tokens,omitempty"`
	MaxToolIterations int              `json:"max_tool_iterations,omitempty"`
	Temperature       *float64         `json:"temperature,omitempty"`
	Skills            []string         `json:"skills,omitempty"`
	Subagents         *SubagentsConfig `json:"subagents,omitempty"`
}

type SubagentsConfig struct {
	AllowAgents []string `json:"allow_agents,omitempty"`
}

type AgentDefaults struct {
	Workspace         string  `json:"workspace" env:"PICOCLAW_AGENTS_DEFAULTS_WORKSPACE"`
	Model             string  `json:"model" env:"PICOCLAW_AGENTS_DEFAULTS_MODEL"`
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
	Enabled   bool     `json:"enabled" env:"PICOCLAW_CHANNELS_TELEGRAM_ENABLED"`
	Token     string   `json:"token" env:"PICOCLAW_CHANNELS_TELEGRAM_TOKEN"`
	AllowFrom []string `json:"allow_from" env:"PICOCLAW_CHANNELS_TELEGRAM_ALLOW_FROM"`
	AllowTemp bool     `json:"allow_temp" env:"PICOCLAW_CHANNELS_TELEGRAM_ALLOW_TEMP"`
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

type ProvidersConfig struct {
	Anthropic  ProviderConfig `json:"anthropic"`
	OpenAI     ProviderConfig `json:"openai"`
	OpenRouter ProviderConfig `json:"openrouter"`
	Groq       ProviderConfig `json:"groq"`
	Zhipu      ProviderConfig `json:"zhipu"`
	VLLM       ProviderConfig `json:"vllm"`
	Gemini     ProviderConfig `json:"gemini"`
	Nvidia     ProviderConfig `json:"nvidia"`
}

type ProviderConfig struct {
	APIKey    string `json:"api_key" env:"PICOCLAW_PROVIDERS_{{.Name}}_API_KEY"`
	APIBase   string `json:"api_base" env:"PICOCLAW_PROVIDERS_{{.Name}}_API_BASE"`
	UserAgent string `json:"user_agent,omitempty" env:"PICOCLAW_PROVIDERS_{{.Name}}_USER_AGENT"`
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
			Anthropic:  ProviderConfig{},
			OpenAI:     ProviderConfig{},
			OpenRouter: ProviderConfig{},
			Groq:       ProviderConfig{},
			Zhipu:      ProviderConfig{},
			VLLM:       ProviderConfig{},
			Gemini:     ProviderConfig{},
			Nvidia:     ProviderConfig{},
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
			AutoSave:       false,
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
		},
	}
}

// sensitiveFields returns pointers to all sensitive string fields in the config.
func sensitiveFields(cfg *Config) []*string {
	return []*string{
		&cfg.Providers.Anthropic.APIKey,
		&cfg.Providers.OpenAI.APIKey,
		&cfg.Providers.OpenRouter.APIKey,
		&cfg.Providers.Groq.APIKey,
		&cfg.Providers.Zhipu.APIKey,
		&cfg.Providers.VLLM.APIKey,
		&cfg.Providers.Gemini.APIKey,
		&cfg.Providers.Nvidia.APIKey,
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

func (c *Config) GetAPIKey() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.Providers.OpenRouter.APIKey != "" {
		return c.Providers.OpenRouter.APIKey
	}
	if c.Providers.Anthropic.APIKey != "" {
		return c.Providers.Anthropic.APIKey
	}
	if c.Providers.OpenAI.APIKey != "" {
		return c.Providers.OpenAI.APIKey
	}
	if c.Providers.Gemini.APIKey != "" {
		return c.Providers.Gemini.APIKey
	}
	if c.Providers.Zhipu.APIKey != "" {
		return c.Providers.Zhipu.APIKey
	}
	if c.Providers.Groq.APIKey != "" {
		return c.Providers.Groq.APIKey
	}
	if c.Providers.Nvidia.APIKey != "" {
		return c.Providers.Nvidia.APIKey
	}
	if c.Providers.VLLM.APIKey != "" {
		return c.Providers.VLLM.APIKey
	}
	return ""
}

func (c *Config) GetAPIBase() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.Providers.OpenRouter.APIKey != "" {
		if c.Providers.OpenRouter.APIBase != "" {
			return c.Providers.OpenRouter.APIBase
		}
		return "https://openrouter.ai/api/v1"
	}
	if c.Providers.Zhipu.APIKey != "" {
		return c.Providers.Zhipu.APIBase
	}
	if c.Providers.VLLM.APIKey != "" && c.Providers.VLLM.APIBase != "" {
		return c.Providers.VLLM.APIBase
	}
	return ""
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
