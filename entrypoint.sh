#!/bin/sh
set -e

PICOCLAW_HOME="/home/picoclaw/.picoclaw"
WORKSPACE="$PICOCLAW_HOME/workspace"
TEMPLATES="$PICOCLAW_HOME/templates"
SKILLS_BUNDLE="$PICOCLAW_HOME/skills-bundle"

# Fix ownership when mounted as Docker volumes
chown picoclaw:picoclaw "$PICOCLAW_HOME"
chown picoclaw:picoclaw "$PICOCLAW_HOME/config.json" 2>/dev/null || true
# .secret_key: only chown if it's a regular file (bind-mount of missing host file creates a directory)
if [ -f "$PICOCLAW_HOME/.secret_key" ]; then
    chown picoclaw:picoclaw "$PICOCLAW_HOME/.secret_key" 2>/dev/null || true
fi

# Ensure workspace root is writable before su-exec mkdir
mkdir -p "$WORKSPACE"
chown picoclaw:picoclaw "$WORKSPACE"

# Initialize workspace directories
su-exec picoclaw mkdir -p "$WORKSPACE/memory" "$WORKSPACE/skills"

# Copy workspace template files if not present; notify if template is newer
for f in AGENTS.md SOUL.md USER.md IDENTITY.md; do
    if [ ! -f "$WORKSPACE/$f" ] && [ -f "$TEMPLATES/$f" ]; then
        cp "$TEMPLATES/$f" "$WORKSPACE/$f"
    elif [ -f "$WORKSPACE/$f" ] && [ -f "$TEMPLATES/$f" ] && [ "$TEMPLATES/$f" -nt "$WORKSPACE/$f" ]; then
        cp "$WORKSPACE/$f" "$WORKSPACE/$f.backup"
        cp "$TEMPLATES/$f" "$WORKSPACE/$f"
        echo "Updated: $f (old version saved as $f.backup)"
    fi
done

# Copy HEARTBEAT.md to memory/ if not present; notify if template is newer
if [ ! -f "$WORKSPACE/memory/HEARTBEAT.md" ] && [ -f "$TEMPLATES/HEARTBEAT.md" ]; then
    cp "$TEMPLATES/HEARTBEAT.md" "$WORKSPACE/memory/HEARTBEAT.md"
elif [ -f "$WORKSPACE/memory/HEARTBEAT.md" ] && [ -f "$TEMPLATES/HEARTBEAT.md" ] && [ "$TEMPLATES/HEARTBEAT.md" -nt "$WORKSPACE/memory/HEARTBEAT.md" ]; then
    cp "$WORKSPACE/memory/HEARTBEAT.md" "$WORKSPACE/memory/HEARTBEAT.md.backup"
    cp "$TEMPLATES/HEARTBEAT.md" "$WORKSPACE/memory/HEARTBEAT.md"
    echo "Updated: memory/HEARTBEAT.md (old version saved as memory/HEARTBEAT.md.backup)"
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
