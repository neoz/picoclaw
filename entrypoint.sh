#!/bin/sh
set -e

# Fix ownership when mounted as Docker volumes
chown picoclaw:picoclaw /home/picoclaw/.picoclaw
chown picoclaw:picoclaw /home/picoclaw/.picoclaw/config.json 2>/dev/null || true
chown picoclaw:picoclaw /home/picoclaw/.picoclaw/.secret_key 2>/dev/null || true
chown -R picoclaw:picoclaw /home/picoclaw/.picoclaw/workspace

exec su-exec picoclaw picoclaw "$@"
