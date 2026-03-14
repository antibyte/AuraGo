#!/bin/bash

# Kill existing processes and remove stale lock files
./kill_all.sh 2>/dev/null || true

# Remove stale lock files that kill_all.sh might have missed
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
rm -f "$DIR/data/aurago.lock" "$DIR/data/maintenance.lock" 2>/dev/null || true

echo "Building AuraGo..."
mkdir -p bin

if ! go build -o bin/aurago ./cmd/aurago; then
    echo "Build of aurago failed!"
    exit 1
fi

if ! go build -o bin/lifeboat ./cmd/lifeboat; then
    echo "Build of lifeboat failed!"
    exit 1
fi

# Auto-load master key from .env if not already set in environment
if [ -z "${AURAGO_MASTER_KEY:-}" ] && [ -f "$(dirname "$0")/.env" ]; then
    source "$(dirname "$0")/.env"
fi

if [ -z "${AURAGO_MASTER_KEY:-}" ]; then
    echo ""
    echo -e "\033[1;33mWARN: AURAGO_MASTER_KEY is not set.\033[0m"
    echo "AuraGo needs a 64-character hex key to encrypt its vault."
    read -p "Generate and save a new key to .env automatically? [y/N]: " CONFIRM
    if [[ "$CONFIRM" =~ ^[Yy]$ ]]; then
        GEN_KEY=$(python3 -c 'import secrets; print(secrets.token_hex(32))' 2>/dev/null || python -c 'import secrets; print(secrets.token_hex(32))' 2>/dev/null || echo "")
        if [ -z "$GEN_KEY" ]; then
            echo -e "\033[0;31mERROR: Failed to generate key. Ensure Python is installed.\033[0m"
            exit 1
        fi
        echo "AURAGO_MASTER_KEY=$GEN_KEY" >> .env
        export AURAGO_MASTER_KEY="$GEN_KEY"
        echo -e "\033[0;32mOK: New key generated and saved to .env\033[0m"
    else
        echo -e "\033[0;31mERROR: AURAGO_MASTER_KEY is required to start.\033[0m"
        echo "Set it via:  export AURAGO_MASTER_KEY=your_key_here"
        exit 1
    fi
fi

echo "Starting AuraGo in background..."
./bin/aurago > aurago.log 2>&1 &

echo "AuraGo started. Check aurago.log for output."
