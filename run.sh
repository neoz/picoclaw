#!/bin/bash
set -e

IMAGE_NAME="picoclaw"
CONTAINER_NAME="picoclaw"
VOLUME_NAME="picoclaw-workspace"
PORT="18790"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
CONFIG_FILE="$SCRIPT_DIR/config.json"
ACTION="run"
BUILD=false
CLEAN=false

for arg in "$@"; do
    case "$arg" in
        --build|-b) BUILD=true ;;
        --clean|-c) CLEAN=true ;;
        --stop|-s) ACTION="stop" ;;
        --force|-f) BUILD=true; CLEAN=true ;;
    esac
done

# --stop: stop the container and exit
if [ "$ACTION" = "stop" ]; then
    if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
        echo "Stopping container..."
        docker stop "$CONTAINER_NAME"
        docker rm -f "$CONTAINER_NAME"
        echo "Container '$CONTAINER_NAME' stopped."
    else
        echo "Container '$CONTAINER_NAME' is not running."
    fi
    exit 0
fi

# --build: force rebuild image
if [ "$BUILD" = true ] || ! docker image inspect "$IMAGE_NAME" >/dev/null 2>&1; then
    echo "Building image..."
    docker build -t "$IMAGE_NAME" .
fi

# Remove existing container if it exists
if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    echo "Stopping existing container..."
    docker stop "$CONTAINER_NAME"
    docker rm -f "$CONTAINER_NAME"
fi

# --clean: remove existing volume
if [ "$CLEAN" = true ] && docker volume ls --format '{{.Name}}' | grep -q "^${VOLUME_NAME}$"; then
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
    -v "$CONFIG_FILE:/home/picoclaw/.picoclaw/config.json" \
    -v "$VOLUME_NAME:/home/picoclaw/.picoclaw/workspace" \
    -e TZ=Asia/Ho_Chi_Minh \
    "$IMAGE_NAME"

echo "Container '$CONTAINER_NAME' is running on port $PORT"
echo "Logs: docker logs -f $CONTAINER_NAME"
