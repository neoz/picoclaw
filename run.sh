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
ACTION="run"
BUILD=false
CLEAN=false

for arg in "$@"; do
    case "$arg" in
        --build|-b) BUILD=true ;;
        --clean|-c) CLEAN=true ;;
        --stop|-s) ACTION="stop" ;;
        --restart|-r) ACTION="restart" ;;
        --force|-f) BUILD=true; CLEAN=true ;;
        --help|-h) ACTION="help" ;;
        skills-list) ACTION="skills-list" ;;
        skills-import) ACTION="skills-import" ;;
        skills-export) ACTION="skills-export" ;;
    esac
done

# --help: display usage
if [ "$ACTION" = "help" ]; then
    echo "Usage: ./run.sh [options] [command]"
    echo ""
    echo "Options:"
    echo "  --build, -b           Force rebuild Docker image"
    echo "  --clean, -c           Remove workspace volume before run"
    echo "  --stop, -s            Stop the container"
    echo "  --restart, -r         Stop and restart the container"
    echo "  --force, -f           Rebuild image and clean volume"
    echo "  --help, -h            Show this help"
    echo ""
    echo "Skill commands:"
    echo "  skills-list           List installed skills in the container"
    echo "  skills-export         Export skills from container to ./skills-export/"
    echo "  skills-import <dir>   Import local skill folder(s) into the container"
    echo ""
    echo "Examples:"
    echo "  ./run.sh                          Run the container"
    echo "  ./run.sh --build                  Rebuild and run"
    echo "  ./run.sh --restart --build        Rebuild and restart"
    echo "  ./run.sh skills-list              List installed skills"
    echo "  ./run.sh skills-import ./weather  Import a skill"
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

# --restart: stop then run again
if [ "$ACTION" = "restart" ]; then
    if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
        echo "Stopping container..."
        docker stop "$CONTAINER_NAME"
        docker rm -f "$CONTAINER_NAME"
    fi
    ACTION="run"
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

# Ensure secret key file exists (prevents Docker from creating it as a directory)
touch "$SECRET_KEY_FILE"

# Run the container
echo "Starting container..."
docker run -d \
    --name "$CONTAINER_NAME" \
    --restart unless-stopped \
    -p "$PORT:$PORT" \
    -v "$CONFIG_FILE:/home/picoclaw/.picoclaw/config.json" \
    -v "$SECRET_KEY_FILE:/home/picoclaw/.picoclaw/.secret_key" \
    -v "$VOLUME_NAME:/home/picoclaw/.picoclaw/workspace" \
    -e TZ=Asia/Ho_Chi_Minh \
    "$IMAGE_NAME"

echo "Container '$CONTAINER_NAME' is running on port $PORT"
echo "Logs: docker logs -f $CONTAINER_NAME"
