# CLAUDE.md

PicoClaw is an ultra-lightweight personal AI assistant in Go for minimal hardware. Multi-channel messaging (Telegram, Discord, QQ, DingTalk, Feishu, WhatsApp, MaixCam), web search, file ops, scheduled tasks, multi-agent orchestration.

## Build & Test

Always use `make build` to verify compilation. If `make` is unavailable (e.g. Windows bash), fall back to `go build ./...`.

```bash
make build              # Build for current platform -> build/picoclaw-{os}-{arch}
make build-all          # Cross-compile for linux-amd64, linux-arm64, linux-riscv64, windows-amd64
make install            # Build + install binary to ~/.local/bin + copy builtin skills
make uninstall          # Remove binary from ~/.local/bin
make install-skills     # Install builtin skills to ~/.picoclaw/workspace/skills
make fmt                # go fmt ./...
make deps               # go get -u ./... && go mod tidy
make clean              # Remove build artifacts
make run ARGS="agent"   # Build and run with arguments
```

Run tests per package: `go test ./pkg/agent/`, `go test ./pkg/tools/`, `go test ./pkg/memory/`, `go test ./pkg/security/`, `go test ./pkg/channels/`, `go test ./pkg/session/`, `go test ./pkg/config/`, `go test ./pkg/providers/`, `go test ./pkg/secrets/`, `go test ./pkg/cost/`.

## Architecture

Entry point: `cmd/picoclaw/main.go` (CLI commands: `onboard`, `agent`, `gateway`, `status`, `cron`, `skills`, `version`).

Core packages: `agent/` (multi-agent loop + orchestration), `memory/` (SQLite+FTS5, knowledge graph), `providers/` (OpenAI-compatible LLM abstraction), `tools/` (tool interface + implementations), `channels/` (multi-channel messaging), `bus/` (async message routing), `config/` (JSON config + env overrides), `secrets/` (ChaCha20 encryption), `session/` (file-based persistence), `skills/` (markdown SKILL.md system), `cron/` (scheduled jobs), `security/` (prompt guard + leak detector + prompt leak guard), `heartbeat/` (periodic prompts), `cost/` (usage tracking + budgets), `voice/` (Groq Whisper transcription).

## Key Gotchas

- **ContextualTool mutex**: All `ContextualTool` implementations must use `sync.Mutex` on channel/chatID fields -- background goroutines (`maybeSummarize`, `RunDelegateAsync`) read them concurrently with `updateToolContexts()`.
- **DelegateRunner pattern**: Tools needing agent loop access define interfaces in `tools/base.go`, implemented by `AgentLoop` in `loop.go`, to avoid circular imports.
- **Tool file naming**: `message_history` tool is in `stm.go`, `session_messages` tool is in `session_messages.go`. Names don't match filenames.
- **Telegram HTML**: `markdownToTelegramHTML()` uses sequential regex replacements -- bold/italic must run before links to prevent crossed tags. `Send()` retries as plain text on HTML parse errors.
- **Telego reply API**: Uses `ReplyParameters: &telego.ReplyParameters{MessageID: id}`, not a flat `ReplyToMessageID` field.
- **Telegram bypasses BaseChannel**: `telegram.go` `handleMessage()` publishes directly to bus, so agent routing (`allow_from` user:agentID suffix) must also be applied there, not just in `BaseChannel.HandleMessage()`.
- **Shell safety URL stripping**: `guardCommand()` in `executor_host.go` strips URLs before filesystem path checking so URL paths (e.g. `wttr.in/path`) aren't flagged as absolute paths. Deny-list still runs on the original command.
- **Sandbox path alignment**: `DockerExecutor` mounts workspace at the same path inside the container (not `/workspace`) so `exec`/`run_code` and filesystem tools (`read_file`, `write_file`) see identical paths. Both volume mode and bind-mount mode use `workspaceDir` as the mount destination.
- **Sandbox UID matching**: `DockerExecutor` detects the workspace owner's UID:GID at init (`detectWorkspaceOwner()` in `executor_docker_unix.go`) and passes it to `--user`. Falls back to `1000:1000` on Windows.
- **Sandbox allowed_commands**: `ExecToolsConfig.AllowedCommands` wired to `HostExecutor.SetAllowPatterns()` in `instance.go`. Only applies to host mode; Docker mode relies on container isolation.
- **Sessions keyed by chatID**: All users in a group share one session. Group detection uses `isGroupMessage()` in `loop.go`.
- **Message data flow**: When adding fields to message history, update the struct, `AddToLog()` signature, and call sites in `loop.go`.
- **Memory owner model**: `owner=""` = shared, `owner="username"` = private. Keys are globally unique (Store deletes existing key regardless of owner). `OwnerAwareTool` interface in `tools/base.go`.
- **FTS rebuild on startup**: `rebuildFTS()` in `Open()` drops and recreates FTS5 table to self-heal corruption. Must run before any migration using DELETE.
- **Retention cleanup chain**: delete expired memories -> `CleanStaleRelations()` -> `CleanOrphanedEntities()`. All three steps required in order.
- **Web search priority**: Ollama > Brave > DuckDuckGo (free fallback). All implement `web_search` tool name.
- **Bootstrap files**: `context.go` loads AGENTS.md, SOUL.md, USER.md, IDENTITY.md from workspace (not TOOLS.md).

## How-To Recipes

**Add a config field**: (1) struct in `config.go` with json+env tags, (2) `config.example.json`, (3) `DefaultConfig()` if non-zero default.

**Add an AgentConfig field**: (1) `AgentConfig` in `config.go`, (2) `AgentInstance` in `instance.go`, (3) `newAgentInstance()`, (4) `AgentInfo` in `tools/base.go` + `ListAgents()` in `loop.go` if exposed to tools, (5) `config.example.json`.

**Add a sensitive config field**: Add its pointer to `sensitiveFields()` in `config.go` for auto encrypt/decrypt.

**Add an LLM provider**: Just add `api_key`, `api_base`, `model_patterns` to `providers` in config.json. For built-in defaults, add to `builtinProviderDefaults` in `config.go`.

**Add a tool**: Implement `Tool` interface. Per-agent tools in `instance.go`, shared tools in `loop.go` `buildSharedTools()`. `DeniedTools` config filters via `registerIfAllowed()`.

**Add a channel**: Embed `BaseChannel`, implement channel interface, register in `channels/manager.go`.

**Add a memory feature**: `pkg/memory/` for storage, `pkg/tools/memory_*.go` for tool interface, `pkg/agent/context.go` for prompt injection.

**Docker bind-mounts**: (1) add mount in `run.sh` + `docker-compose.yml`, (2) `touch` in `run.sh` before `docker run`, (3) `chown` in `entrypoint.sh`.

## Configuration

Config: `~/.picoclaw/config.json` (template: `config.example.json`). `LoadConfig()` returns defaults if missing.

Key sections: `agents` (defaults + list), `providers` (map with api_key/api_base/model_patterns/fallback), `channels`, `tools.exec` (sandbox mode + docker config + allowed_commands), `tools.web.search`, `gateway`, `memory`, `secrets`, `security`, `heartbeat`, `cost`.

`run.sh` commands: `--build`, `--clean`, `--stop`, `--restart`, `--force`, `skills-list`, `skills-export`, `skills-import <dir>`, `memory-export`.

**Known issue**: Per-provider env var overrides don't work; use JSON config for provider API keys.

## Go Version

Go 1.26.0 (see go.mod).
