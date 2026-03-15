#!/bin/sh
set -e

PICOCLAW_HOME="/home/picoclaw/.picoclaw"
WORKSPACE="$PICOCLAW_HOME/workspace"
TEMPLATES="$PICOCLAW_HOME/templates"
SKILLS_BUNDLE="$PICOCLAW_HOME/skills-bundle"

# Fix ownership when mounted as Docker volumes
chown picoclaw:picoclaw "$PICOCLAW_HOME"
chown picoclaw:picoclaw "$PICOCLAW_HOME/config.json" 2>/dev/null || true
chown picoclaw:picoclaw "$PICOCLAW_HOME/.secret_key" 2>/dev/null || true

# Initialize workspace directories
su-exec picoclaw mkdir -p "$WORKSPACE/memory" "$WORKSPACE/skills"

# Copy workspace template files if not present (AGENTS.md, SOUL.md, etc.)
for f in AGENTS.md SOUL.md USER.md IDENTITY.md; do
    if [ ! -f "$WORKSPACE/$f" ] && [ -f "$TEMPLATES/$f" ]; then
        cp "$TEMPLATES/$f" "$WORKSPACE/$f"
    fi
done

# Copy HEARTBEAT.md to memory/ if not present (preserve user edits)
if [ ! -f "$WORKSPACE/memory/HEARTBEAT.md" ] && [ -f "$TEMPLATES/HEARTBEAT.md" ]; then
    cp "$TEMPLATES/HEARTBEAT.md" "$WORKSPACE/memory/HEARTBEAT.md"
fi

# Sync skills from bundle (always overwrite to pick up updates)
if [ -d "$SKILLS_BUNDLE" ]; then
    cp -r "$SKILLS_BUNDLE"/. "$WORKSPACE/skills/"
fi

# Fix workspace ownership after all copies
chown -R picoclaw:picoclaw "$WORKSPACE"

# Allow picoclaw user to access Docker socket (DooD sandbox)
if [ -S /var/run/docker.sock ]; then
    chmod 666 /var/run/docker.sock 2>/dev/null || true
fi

exec su-exec picoclaw picoclaw "$@"
