#!/bin/sh
set -e

# Fix workspace ownership when mounted as a Docker volume
chown -R picoclaw:picoclaw /home/picoclaw/.picoclaw/workspace

exec su-exec picoclaw picoclaw "$@"
