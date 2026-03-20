#!/usr/bin/env bash
set -e

MODEL_DIR="./models"
MODEL_FILE="bge-m3-Q4_k_m.gguf"
MODEL_URL="https://huggingface.co/keisuke-miyako/bge-m3-gguf-q4_k_m/resolve/main/${MODEL_FILE}"

mkdir -p "$MODEL_DIR"

if [ -f "$MODEL_DIR/$MODEL_FILE" ]; then
    echo "Model already exists: $MODEL_DIR/$MODEL_FILE"
else
    echo "Downloading BGE-M3 GGUF model..."
    curl -L --progress-bar -o "$MODEL_DIR/$MODEL_FILE" "$MODEL_URL"

    # Validate GGUF magic bytes
    MAGIC=$(head -c 4 "$MODEL_DIR/$MODEL_FILE" | cat -v)
    if [[ "$MAGIC" != *"GGUF"* ]]; then
        echo "ERROR: $MODEL_FILE is not a valid GGUF file. Removing."
        rm -f "$MODEL_DIR/$MODEL_FILE"
        exit 1
    fi

    echo "Download complete: $MODEL_DIR/$MODEL_FILE"
fi

echo ""
echo "Setup complete. Run 'docker compose up' to start all services."
