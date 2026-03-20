#!/bin/bash
set -e

IMAGE_NAME="picoclaw"
CONTAINER_NAME="picoclaw"
VOLUME_NAME="picoclaw-workspace"
PORT="18790"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
CONFIG_FILE="$SCRIPT_DIR/config.json"
SECRET_KEY_FILE="$SCRIPT_DIR/.secret_key"
SKILLS_DIR="/home/picoclaw/.picoclaw/workspace/skills"

SAGE_IMAGE="ghcr.io/l33tdawg/sage:latest"
SAGE_CONTAINER="sage"
SAGE_VOLUME="sage-data"
SAGE_PORT="8080"
SAGE_CONFIG="$SCRIPT_DIR/sage-config.yaml"

BGE_IMAGE="llama-cpp-server"
BGE_CONTAINER="bge-m3"
BGE_PORT="8081"

PROXY_IMAGE="python:3.11-slim"
PROXY_CONTAINER="ollama-proxy"
PROXY_SCRIPT="$SCRIPT_DIR/ollama-proxy.py"

NETWORK_NAME="picoclaw-net"

# All MCP infra containers (start order: bge-m3 -> ollama-proxy -> sage)
MCP_CONTAINERS=("$BGE_CONTAINER" "$PROXY_CONTAINER" "$SAGE_CONTAINER")

ACTION=""
BUILD=false
CLEAN=false

for arg in "$@"; do
    case "$arg" in
        --build|-b) BUILD=true ;;
        --clean|-c) CLEAN=true ;;
        --stop|-s) ACTION="stop" ;;
        --restart|-r) ACTION="restart" ;;
        --force|-f) BUILD=true; CLEAN=true; ACTION="restart" ;;
        --help|-h) ACTION="help" ;;
        skills-list) ACTION="skills-list" ;;
        skills-import) ACTION="skills-import" ;;
        skills-export) ACTION="skills-export" ;;
        memory-export) ACTION="memory-export" ;;
        sh|shell) ACTION="shell" ;;
    esac
done

# Default action: "run" if no action specified and not build-only
if [ -z "$ACTION" ]; then
    if [ "$BUILD" = true ]; then
        ACTION="build"
    else
        ACTION="run"
    fi
fi

# Helper: stop and remove a container if it exists
stop_container() {
    local name="$1"
    if docker ps -a --format '{{.Names}}' | grep -q "^${name}$"; then
        echo "Stopping $name..."
        docker stop "$name" 2>/dev/null || true
        docker rm -f "$name" 2>/dev/null || true
    fi
}

# Helper: stop all MCP infra containers (reverse order)
stop_mcp_infra() {
    for (( i=${#MCP_CONTAINERS[@]}-1; i>=0; i-- )); do
        stop_container "${MCP_CONTAINERS[$i]}"
    done
}

# Helper: ensure shared network exists
ensure_network() {
    if ! docker network ls --format '{{.Name}}' | grep -q "^${NETWORK_NAME}$"; then
        echo "Creating network $NETWORK_NAME..."
        docker network create "$NETWORK_NAME"
    fi
}

# Helper: start all MCP infra containers
start_mcp_infra() {
    ensure_network

    # bge-m3: embedding model server
    if ! docker ps --format '{{.Names}}' | grep -q "^${BGE_CONTAINER}$"; then
        stop_container "$BGE_CONTAINER"
        echo "Starting $BGE_CONTAINER..."
        docker run -d \
            --name "$BGE_CONTAINER" \
            --network "$NETWORK_NAME" \
            --restart unless-stopped \
            -p "$BGE_PORT:$BGE_PORT" \
            -v "$SCRIPT_DIR/models:/models" \
            -e LLAMA_ARG_MODEL=/models/bge-m3-Q4_k_m.gguf \
            -e LLAMA_ARG_HOST=0.0.0.0 \
            -e LLAMA_ARG_PORT="$BGE_PORT" \
            -e LLAMA_ARG_CTX_SIZE=8192 \
            -e LLAMA_ARG_EMBEDDINGS=1 \
            "$BGE_IMAGE"
    fi

    # ollama-proxy: translates Ollama API to llama.cpp
    if ! docker ps --format '{{.Names}}' | grep -q "^${PROXY_CONTAINER}$"; then
        stop_container "$PROXY_CONTAINER"
        echo "Starting $PROXY_CONTAINER..."
        docker run -d \
            --name "$PROXY_CONTAINER" \
            --network "$NETWORK_NAME" \
            --restart unless-stopped \
            -v "$PROXY_SCRIPT:/app/ollama-proxy.py:ro" \
            -e UPSTREAM_URL="http://$BGE_CONTAINER:$BGE_PORT" \
            -e LISTEN_PORT=11434 \
            "$PROXY_IMAGE" \
            python3 -u /app/ollama-proxy.py
    fi

    # sage: MCP memory server
    if ! docker ps --format '{{.Names}}' | grep -q "^${SAGE_CONTAINER}$"; then
        stop_container "$SAGE_CONTAINER"
        if ! docker volume ls --format '{{.Name}}' | grep -q "^${SAGE_VOLUME}$"; then
            docker volume create "$SAGE_VOLUME"
        fi
        echo "Starting $SAGE_CONTAINER..."
        docker run -d \
            --name "$SAGE_CONTAINER" \
            --network "$NETWORK_NAME" \
            --restart unless-stopped \
            --entrypoint sage-gui \
            -p "$SAGE_PORT:$SAGE_PORT" \
            -v "$SAGE_VOLUME:/root/.sage" \
            -v "$SAGE_CONFIG:/root/.sage/config.yaml:ro" \
            -e TZ=Asia/Ho_Chi_Minh \
            "$SAGE_IMAGE" \
            serve
        echo "Sage is running on port $SAGE_PORT"
    fi
}

# --help: display usage
if [ "$ACTION" = "help" ]; then
    echo "Usage: ./run.sh [options] [command]"
    echo ""
    echo "Options:"
    echo "  --build, -b           Build Docker image (without restarting service)"
    echo "  --clean, -c           Remove workspace volume before run"
    echo "  --stop, -s            Stop all containers (picoclaw + MCP infra)"
    echo "  --restart, -r         Stop and restart all containers"
    echo "  --force, -f           Rebuild image and clean volume"
    echo "  --help, -h            Show this help"
    echo ""
    echo "Skill commands:"
    echo "  skills-list           List installed skills in the container"
    echo "  skills-export         Export skills from container to ./skills-export/"
    echo "  skills-import <dir>   Import local skill folder(s) into the container"
    echo ""
    echo "Data commands:"
    echo "  memory-export         Export memory database from container to ./memory-export/"
    echo ""
    echo "Shell commands:"
    echo "  sh, shell             Open an interactive shell in the container"
    echo ""
    echo "Examples:"
    echo "  ./run.sh                          Run the container"
    echo "  ./run.sh --build                  Build image only"
    echo "  ./run.sh --restart --build        Rebuild and restart"
    echo "  ./run.sh skills-list              List installed skills"
    echo "  ./run.sh skills-import ./weather  Import a skill"
    echo "  ./run.sh memory-export            Export memory DB"
    exit 0
fi

# skills-list: list skills in the container
if [ "$ACTION" = "skills-list" ]; then
    if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
        echo "Container '$CONTAINER_NAME' is not running."
        exit 1
    fi
    echo "Installed skills:"
    docker exec "$CONTAINER_NAME" ls -1 "$SKILLS_DIR" 2>/dev/null || echo "  (none)"
    exit 0
fi

# skills-export: copy skills from container to local ./skills-export/
if [ "$ACTION" = "skills-export" ]; then
    if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
        echo "Container '$CONTAINER_NAME' is not running."
        exit 1
    fi
    EXPORT_DIR="$SCRIPT_DIR/skills-export"
    rm -rf "$EXPORT_DIR"
    mkdir -p "$EXPORT_DIR"
    docker cp "$CONTAINER_NAME:$SKILLS_DIR/." "$EXPORT_DIR/"
    count=$(ls -1d "$EXPORT_DIR"/*/ 2>/dev/null | wc -l)
    echo "Exported $count skill(s) to $EXPORT_DIR"
    exit 0
fi

# skills-import: copy local skill folders into the container
# Usage: ./run.sh skills-import <path-to-skill-folder> [...]
if [ "$ACTION" = "skills-import" ]; then
    if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
        echo "Container '$CONTAINER_NAME' is not running."
        exit 1
    fi
    # Collect paths after "skills-import"
    found=false
    paths=()
    for arg in "$@"; do
        if [ "$found" = true ]; then
            paths+=("$arg")
        fi
        if [ "$arg" = "skills-import" ]; then
            found=true
        fi
    done
    if [ ${#paths[@]} -eq 0 ]; then
        echo "Usage: ./run.sh skills-import <skill-folder> [<skill-folder> ...]"
        echo "Example: ./run.sh skills-import ./my-skills/weather ./my-skills/translate"
        exit 1
    fi
    for path in "${paths[@]}"; do
        if [ ! -d "$path" ]; then
            echo "Skip: '$path' is not a directory"
            continue
        fi
        name=$(basename "$path")
        docker cp "$path" "$CONTAINER_NAME:$SKILLS_DIR/$name"
        docker exec "$CONTAINER_NAME" chown -R picoclaw:picoclaw "$SKILLS_DIR/$name"
        echo "Imported skill: $name"
    done
    exit 0
fi

# memory-export: copy memory folder from container to local ./memory-export/
if [ "$ACTION" = "memory-export" ]; then
    if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
        echo "Container '$CONTAINER_NAME' is not running."
        exit 1
    fi
    EXPORT_DIR="$SCRIPT_DIR/memory-export"
    rm -rf "$EXPORT_DIR"
    mkdir -p "$EXPORT_DIR"
    MEMORY_DIR="/home/picoclaw/.picoclaw/workspace/memory"
    docker cp "$CONTAINER_NAME:$MEMORY_DIR/." "$EXPORT_DIR/"
    echo "Exported memory to $EXPORT_DIR"
    ls -lh "$EXPORT_DIR"
    exit 0
fi

# sh/shell: open interactive shell in the container
if [ "$ACTION" = "shell" ]; then
    if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
        echo "Container '$CONTAINER_NAME' is not running."
        exit 1
    fi
    docker exec -it "$CONTAINER_NAME" sh
    exit 0
fi

# --stop: stop all containers and exit
if [ "$ACTION" = "stop" ]; then
    stop_container "$CONTAINER_NAME"
    stop_mcp_infra
    echo "All containers stopped."
    exit 0
fi

# Build image if requested or missing
if [ "$BUILD" = true ] || ! docker image inspect "$IMAGE_NAME" >/dev/null 2>&1; then
    echo "Building image..."
    docker build -t "$IMAGE_NAME" .
fi

# Build-only: exit after building without stopping the service
if [ "$ACTION" = "build" ]; then
    echo "Image built successfully."
    exit 0
fi

# --restart: stop then run again
if [ "$ACTION" = "restart" ]; then
    stop_container "$CONTAINER_NAME"
    stop_mcp_infra
    ACTION="run"
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

# Ensure secret key file exists (prevents Docker from creating it as a directory)
touch "$SECRET_KEY_FILE"

# Start MCP infra (bge-m3 -> ollama-proxy -> sage)
start_mcp_infra

# Run the container
echo "Starting container..."
docker run -d \
    --name "$CONTAINER_NAME" \
    --network "$NETWORK_NAME" \
    --restart unless-stopped \
    -p "$PORT:$PORT" \
    -v "$CONFIG_FILE:/home/picoclaw/.picoclaw/config.json" \
    -v "$SECRET_KEY_FILE:/home/picoclaw/.picoclaw/.secret_key" \
    -v "$VOLUME_NAME:/home/picoclaw/.picoclaw/workspace" \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -e TZ=Asia/Ho_Chi_Minh \
    "$IMAGE_NAME"

echo "Container '$CONTAINER_NAME' is running on port $PORT"
echo "Logs: docker logs -f $CONTAINER_NAME"
