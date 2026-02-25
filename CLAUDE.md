# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

PicoClaw is an ultra-lightweight personal AI assistant written in Go, designed to run on minimal hardware ($10 boards, <10MB RAM). It provides AI agent capabilities including web search, file operations, scheduled tasks, and multi-channel messaging (Telegram, Discord, QQ, DingTalk, Feishu, WhatsApp).

## Build Commands

Always use `make build` to verify compilation. If `make` is unavailable (e.g. Windows bash), fall back to `go build ./...`.

```bash
make build              # Build for current platform -> build/picoclaw-{os}-{arch}
make build-all          # Cross-compile for linux-amd64, linux-arm64, linux-riscv64, windows-amd64
make install            # Build + install binary to ~/.local/bin + copy builtin skills
make install-skills     # Install builtin skills to ~/.picoclaw/workspace/skills
make fmt                # Format Go code (go fmt ./...)
make deps               # Update dependencies (go get -u ./... && go mod tidy)
make clean              # Remove build artifacts
make run ARGS="agent"   # Build and run with arguments
```

Test files: `pkg/logger/logger_test.go`, `pkg/channels/telegram_test.go`, `pkg/security/promptguard_test.go`, `pkg/security/leakdetector_test.go`. Run with `go test ./pkg/logger/` or `go test ./pkg/security/`.

## Architecture

### Entry Point

`cmd/picoclaw/main.go` - Single large file (~1200 lines) containing all CLI commands: `onboard`, `agent`, `gateway`, `status`, `cron`, `skills`, `version`.

### Core Packages (pkg/)

- **agent/** - Multi-agent system. `loop.go` orchestrates message processing via `AgentRegistry`. `instance.go` defines `AgentInstance` (per-agent provider, sessions, tools, context builder) with factory `newAgentInstance()`. `registry.go` provides `AgentRegistry` for agent lookup by ID with a default fallback. `context.go` builds system prompts from workspace files and injects relevance-filtered memory context per message. Agent dispatch: `msg.Metadata["agent_id"]` routes to a specific agent; unset routes to default.

- **memory/** - SQLite + FTS5 memory system (pure Go, no CGO via `modernc.org/sqlite`). `db.go` manages schema/lifecycle, `store.go` CRUD, `search.go` full-text search with BM25, `retention.go` category-based cleanup, `migrate.go` one-time markdown migration, `snapshot.go` export/import. Database at `workspace/memory/memory.db`. Categories: core (permanent), daily (30d), conversation (7d), custom (90d).

- **providers/** - LLM provider abstraction. `HTTPProvider` implements a generic OpenAI-compatible API client. Provider is selected by model name/prefix pattern matching (e.g., `anthropic/claude-opus-4-5` routes to Anthropic, model names containing "gpt" route to OpenAI). Supported: OpenRouter, Anthropic, OpenAI, Gemini, Zhipu, Groq, Nvidia, VLLM. Falls back to OpenRouter if no pattern matches. `CreateProviderForModel(model, cfg)` creates a provider for a specific model; `CreateProvider(cfg)` delegates to it using `agents.defaults.model`.

- **tools/** - Tool implementations following the `Tool` interface (`Name()`, `Description()`, `Parameters()`, `Execute()`). Registered via `ToolRegistry`. Key tools: `filesystem.go` (read/write/list), `shell.go` (command execution with safety deny-list), `web.go` (web search + fetch), `edit.go` (file editing), `cron.go` (scheduling), `message.go` (channel messaging), `spawn.go`/`subagent.go` (sub-agent spawning), `delegate.go` (orchestrator delegation to specialist agents).

- **channels/** - Multi-channel messaging. Each channel embeds `BaseChannel` and implements `Start()`, `Stop()`, `Send()`. `manager.go` coordinates all channels. Supported: Telegram, Discord, QQ, DingTalk, Feishu, WhatsApp, MaixCAM.

- **bus/** - Central message bus for async inbound/outbound message routing between channels and the agent.

- **config/** - JSON config from `~/.picoclaw/config.json`. All values overridable via env vars (pattern: `PICOCLAW_SECTION_KEY`).

- **secrets/** - ChaCha20-Poly1305 AEAD encryption for sensitive config values. `SecretStore` in `secrets.go` manages key loading/generation and encrypt/decrypt. Key file: `~/.picoclaw/.secret_key` (32 random bytes, hex-encoded, 0600 perms). Encrypted format: `enc:<hex(nonce||ciphertext||tag)>`. Values without `enc:` prefix pass through as plaintext.

- **session/** - File-based session persistence (JSON in `workspace/sessions/`). Sessions keyed as `channel:chatID`. Also stores `MessageLog` (searchable message history with 30-day retention) used by the `message_history` tool. `MessageLogEntry` includes `sender_name` for human-readable display. Filenames use `SanitizeSessionKey()` (`:` replaced with `_`) for Windows compatibility.

- **skills/** - Markdown-based skill system. Skills are SKILL.md files auto-discovered from `workspace/skills/`. Can be installed from GitHub repos.

- **cron/** - Scheduled job service with both interval ("every N seconds") and cron expression support. Jobs stored in `workspace/cron/jobs.json`.

- **security/** - Input/output security scanning. `promptguard.go` detects prompt injection (system override, role confusion, tool call injection, secret extraction, command injection subtypes, jailbreak) via regex scoring with configurable sensitivity/action. `leakdetector.go` detects and redacts credentials (API keys, AWS, private keys, JWTs, database URLs, generic secrets) in outbound content. Both initialized in `loop.go` `NewAgentLoop()` when `cfg.Security` is enabled. Tests: `go test ./pkg/security/`.

- **voice/** - Voice transcription via Groq Whisper API, attached to Telegram/Discord channels.

### Key Patterns

- **Tool registration**: New tools implement the `Tool` interface. Workspace-scoped tools (filesystem, exec, edit) are per-agent in `instance.go`. Shared tools (web, memory, message, cost) are created once in `loop.go` `buildSharedTools()` and registered on all agents. External tools (e.g. cron) use `agentLoop.RegisterTool()` which registers on the default agent. Config-conditional tools (e.g. `delegate`) are registered by `initDelegateTools()` at end of `NewAgentLoop()`, only on agents with `subagents.allow_agents` set.
- **ContextualTool**: Tools needing channel/chatID implement `SetContext(channel, chatID)`. Context is updated per-message in `updateToolContexts()` in `loop.go`. All ContextualTool implementations must use `sync.Mutex` to guard their channel/chatID fields, since background goroutines (`maybeSummarize`, `RunDelegateAsync`) may read them concurrently with the main loop writing via `updateToolContexts()`.
- **DelegateRunner pattern**: Tools needing agent loop access (e.g. `delegate.go`) define an interface in `tools/base.go` (`DelegateRunner`), implemented by `AgentLoop` in `loop.go`. This avoids circular imports between `pkg/tools` and `pkg/agent`.
- **Shared utilities**: `tools/bm25.go` provides `tokenize()` and `bm25Rank()` for BM25-ranked text search. Used by `message_history` tool. The `memory_search` tool now uses SQLite FTS5 instead.
- **Channel registration**: New channels embed `BaseChannel`, implement the channel interface, and are registered in `channels/manager.go`.
- **Telegram HTML conversion**: `telegram.go` `markdownToTelegramHTML()` converts markdown to Telegram HTML via sequential regex replacements. Order matters: bold/italic must be processed before links to prevent crossed HTML tags. Italic regex excludes `<>` to avoid wrapping around tags from earlier steps. The `Send()` method has a fallback that retries as plain text on HTML parse errors.
- **Telego reply API**: `SendMessageParams` uses `ReplyParameters: &telego.ReplyParameters{MessageID: id}`, not a flat `ReplyToMessageID` field. Use `go doc github.com/mymmrac/telego.SendMessageParams` to check struct fields.
- **Telegram temp access**: `/allow @username` command in groups (handled at channel level, never reaches agent). Uses `sync.Map` with time-window TTL (not one-shot). Allowed users only.
- **Telegram reply context**: `handleMessage()` prepends `[reply to Name: text]` to content when the message replies to a non-bot user. `ReplyToMessage` is a full `*Message` with `Text`, `Caption`, `From`. Skip replies to the bot itself (already in session history).
- **Provider routing**: Models with explicit provider prefixes (e.g., `anthropic/...`) route to that provider; otherwise the fallback chain is used.
- **Shell safety**: `tools/shell.go` enforces `restrictToWorkspace: true` by default, confining `exec` to the workspace directory. The `guardCommand()` function applies: (1) regex deny-list for destructive commands and sensitive file patterns (config files, SSH keys, private keys, password databases), (2) `~`/`$HOME`/`${HOME}` expansion before path checking to prevent bypass, (3) `working_dir` parameter validation. All tool security boundaries (exec, read_file, write_file, list_dir, edit_file) must remain consistent - the workspace directory is the sandbox.
- **Message data flow**: Channel populates `bus.InboundMessage` with `Metadata` (username, first_name, user_id, etc.) -> `agent/loop.go` extracts metadata and calls `sessions.AddToLog()` -> persisted in `session.MessageLogEntry`. When adding fields to message history, update all three: the struct, `AddToLog()` signature, and the call sites in `loop.go`. In group chats, `processMessage()` prepends `[senderName]: ` to the user message so the LLM can distinguish users. Group detection uses `isGroupMessage()` (checks `is_group`, `is_dm`, `group_id`, `conversation_type`, `chat_type` across channels). Sessions are keyed by chatID, not userID, so all users in a group share one session.
- **Channel metadata keys**: Telegram: `username`, `first_name`, `user_id`, `is_group`. Discord: `username`, `display_name`, `user_id`, `guild_id`, `is_dm`. DingTalk: `sender_name`, `conversation_type` ("2"=group). QQ: `group_id` (present for group msgs). Feishu: `chat_type` ("group"). When adding group-aware features, update `isGroupMessage()` and `getSenderDisplayName()` in `loop.go`.
- **Security scanning** (`loop.go`): PromptGuard scans at two points: (1) user input in `processMessage()` (warn or block), (2) tool results in `runLLMIteration()` (warn only). LeakDetector scans outbound content in `runAgentLoop()` step 5.5 (auto-redacts before session save and bus publish). Adding a new scan category: add to `defaultGuardCategories()` or `defaultLeakCategories()`, update `maxGuardScore` if adding guard categories, and add tests.
- **Workspace files**: Agent context is assembled from markdown files in `~/.picoclaw/workspace/` (AGENTS.md, SOUL.md, IDENTITY.md, USER.md, TOOLS.md). Memory is now served from SQLite via relevance-filtered context injection in `context.go`.

## Configuration

Config file: `~/.picoclaw/config.json` (see `config.example.json` for template). `LoadConfig()` silently returns defaults if the file is missing. `CreateProvider()` then fails with "no API key configured" - check the config file path first when debugging provider errors.

**NewAgentLoop signature**: `NewAgentLoop(cfg, msgBus) (*AgentLoop, error)` -- provider creation is internal. Entry points (`agentCmd`, `gatewayCmd` in `main.go`) no longer call `CreateProvider` directly.

Key sections: `agents.defaults` (model, max_tokens, temperature, workspace), `agents.list` (optional array of `AgentConfig` for multi-agent; when empty, implicit "main" agent is synthesized from defaults), `providers` (api_key + api_base per provider), `channels` (enabled + credentials + allow_from per channel), `tools.web.search` (Brave API key), `gateway` (host/port, default 0.0.0.0:18790), `memory` (retention_days, search_limit, min_relevance, context_top_k, auto_save, snapshot_on_exit), `secrets` (encrypt toggle for config field encryption), `security` (prompt_guard: enabled/action/sensitivity, leak_detector: enabled/sensitivity).

Adding a config field: (1) add to struct in `pkg/config/config.go` with json + env tags, (2) update `config.example.json`, (3) update `DefaultConfig()` if non-zero default needed, (4) use in consuming code. `DefaultConfig()` flows through to `onboard` via `SaveConfig`, so no separate onboard change is needed.

**Adding a sensitive config field**: Also add its pointer to `sensitiveFields()` in `config.go`. This registers it for automatic encrypt-on-save and decrypt-on-load. Currently 17 fields: 8 provider API keys, 5 Feishu/Telegram/Discord tokens, 2 QQ/DingTalk secrets, 2 web tool API keys.

**Config encryption**: `secrets.encrypt` toggle (default `false`). Decryption is always active on load (prefix-driven). Encryption only on `SaveConfig` when enabled. `SaveConfig` JSON-clones the config before encrypting to avoid mutating the caller. Key auto-generated on first encrypt. Test with `go test ./pkg/secrets/`.

**Docker secrets**: `.secret_key` is bind-mounted from host alongside `config.json` in `run.sh`/`docker-compose.yml`. `run.sh` uses `touch` to create the file if missing (Docker would create a directory otherwise). `entrypoint.sh` must `chown` all bind-mounted files to `picoclaw` user. `NewSecretStore` treats empty key files as missing and generates a new key. When adding new bind-mounted files: (1) add mount in `run.sh` + `docker-compose.yml`, (2) `touch` in `run.sh` before `docker run`, (3) `chown` in `entrypoint.sh`.

**Web search priority** (in `loop.go`): Ollama Search (if `tools.web.ollama.api_key` set) > Brave Search (if `tools.web.search.api_key` set) > DuckDuckGo (free, no key required, always available as fallback). All three implement the same `web_search` tool name. `web_fetch` is always registered (Ollama fetch when using Ollama, standard fetch otherwise).

**Known issue**: `ProviderConfig` env tags use `{{.Name}}` template syntax that `caarlos0/env` doesn't expand. Per-provider env var overrides don't work; only JSON config values are effective for provider API keys.

## Memory System

Memory is SQLite-backed (`pkg/memory/`). Three tools: `memory_store`, `memory_search`, `memory_forget`. Shared single SQLite DB across all agents (memory is about the user, not per-agent). Initialized in `agent/loop.go` `NewAgentLoop()` with graceful degradation if DB fails to open. On first run, `MigrateFromMarkdown()` imports existing MEMORY.md and daily notes. `RunRetention()` cleans expired entries on startup. `context.go` `buildRelevantMemoryContext()` injects core memories + FTS5-matched results into each system prompt. `Shutdown()` optionally exports a snapshot.

Adding a memory feature: modify `pkg/memory/` for storage logic, `pkg/tools/memory_*.go` for tool interface, `pkg/agent/context.go` for prompt injection.

**Knowledge graph layer**: `graph.go` stores entity-relation triples (entities + relations tables) linked to memories via `memory_key` (text field, NOT a FK). `graph_schema.go` holds DDL. `memory_store` tool accepts optional `relations` parameter. `context.go` `buildGraphMemoryContext()` does entity matching + BFS walk for graph-first context injection, falling back to FTS5. **Retention cleanup chain** (in `retention.go`): delete expired memories -> `CleanStaleRelations()` (remove relations whose `memory_key` no longer exists in memories) -> `CleanOrphanedEntities()` (remove entities with zero relations). All three steps are required in order.

**Memory tests**: `pkg/memory/graph_test.go`. Helper `openTestDB(t)` uses `t.TempDir()` + `t.Cleanup`. For retention tests, backdate `updated_at` via `db.db.Exec` (same-package access) since `RunRetention` skips `days <= 0` and freshly-stored entries won't be older than the cutoff.

**Auto-save flow**: When `memory.auto_save=true`, each user message is stored in `loop.go` (step 3.5) with key `conv_{channel}_{chatID}_{millisTimestamp}` and category `conversation` (7-day retention). Only user messages are saved, not assistant responses. `memory_search` with empty query falls back to `List()` to support browsing.

## Agent Orchestrator

The `delegate` tool enables an orchestrator pattern: a default agent routes tasks to specialist agents. `DelegateTool` (in `pkg/tools/delegate.go`) uses the `DelegateRunner` interface (in `base.go`) implemented by `AgentLoop`. Sync mode calls `runAgentLoop()` on the target agent's `AgentInstance` with a `delegate:{agentID}:{chatID}:{timestamp}` session key. Async mode runs in a goroutine and publishes results back via bus as system messages (same pattern as `spawn`/`subagent.go`). Access control: only agents listed in `subagents.allow_agents` can be delegated to. The `delegate` tool is only registered on agents that have this config set. `updateToolContexts()` in `loop.go` must be updated when adding new `ContextualTool` implementations.

## Go Version

Go 1.26.0 (see go.mod).
