#!/bin/bash
set -e

IMAGE_NAME="picoclaw"
CONTAINER_NAME="picoclaw"
VOLUME_NAME="picoclaw-workspace"
PORT="18790"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
CONFIG_FILE="$SCRIPT_DIR/config.json"
FORCE=false

for arg in "$@"; do
    case "$arg" in
        --force|-f) FORCE=true ;;
    esac
done

# Build only if image doesn't exist or --force
if [ "$FORCE" = true ] || ! docker image inspect "$IMAGE_NAME" >/dev/null 2>&1; then
    echo "Building image..."
    docker build -t "$IMAGE_NAME" .
fi

# Remove existing container if it exists
if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    echo "Stopping existing container..."
    docker stop "$CONTAINER_NAME"
    echo "Removing existing container..."
    docker rm -f "$CONTAINER_NAME"
fi

# if force, remove remove existing volume
if [ "$FORCE" = true ] && docker volume ls --format '{{.Name}}' | grep -q "^${VOLUME_NAME}$"; then
    echo "Removing existing volume..."
    docker volume rm "$VOLUME_NAME"
fi

# Create volume if it doesn't exist
if ! docker volume ls --format '{{.Name}}' | grep -q "^${VOLUME_NAME}$"; then
    echo "Creating volume..."
    docker volume create "$VOLUME_NAME"
fi

# Run the container
echo "Starting container..."
docker run -d \
    --name "$CONTAINER_NAME" \
    --restart unless-stopped \
    -p "$PORT:$PORT" \
    -v "$CONFIG_FILE:/home/picoclaw/.picoclaw/config.json:ro" \
    -v "$VOLUME_NAME:/home/picoclaw/.picoclaw/workspace" \
    -e TZ=Asia/Ho_Chi_Minh \
    "$IMAGE_NAME"

echo "Container '$CONTAINER_NAME' is running on port $PORT"
echo "Logs: docker logs -f $CONTAINER_NAME"
