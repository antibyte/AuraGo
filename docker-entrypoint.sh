#!/usr/bin/env bash
set -e

# Config-Merger Safety Wrapper - Bulletproof configuration management
# This script ensures safe configuration updates with multiple fallback layers

CONFIG_FILE="/app/data/config.yaml"
USER_CONFIG="/app/config.yaml.user"
ENV_FILE="/app/data/.env"
TEMPLATE_FILE="/app/config.yaml.default"
MERGER_BIN="/app/config-merger"

# Helper: Normalize line endings (CRLF -> LF)
normalize_file() {
    local file="$1"
    if [ -f "$file" ]; then
        if ! sed -i 's/\r$//' "$file" 2>/dev/null; then
            echo "[Entrypoint] WARNING: Could not normalize line endings for $file"
        fi
    fi
}

# Normalize the baked-in template immediately — it may have CRLF endings
# because it was COPYd from a Windows development machine.
normalize_file "$TEMPLATE_FILE"

# === 1. INITIAL CONFIG SETUP ===

# Docker creates a directory if the host file doesn't exist when using bind mounts.
# We cannot remove a mounted directory, so just warn and continue.
# All subsequent checks use [ -f "$USER_CONFIG" ] so a directory is safely ignored.
if [ -d "$USER_CONFIG" ]; then
    echo "[Entrypoint] =========================================================="
    echo "[Entrypoint] WARNING: $USER_CONFIG is a directory, not a file!"
    echo "[Entrypoint]"
    echo "[Entrypoint] This happens when the host file did not exist before"
    echo "[Entrypoint] running 'docker compose up'. Docker auto-creates a"
    echo "[Entrypoint] directory instead of a file for missing bind mounts."
    echo "[Entrypoint]"
    echo "[Entrypoint] Fix: stop the container and run these commands on the host:"
    echo "[Entrypoint]   docker compose down"
    echo "[Entrypoint]   rmdir config.yaml        # remove the auto-created directory"
    echo "[Entrypoint]   touch config.yaml         # create an empty file"
    echo "[Entrypoint]   docker compose up -d"
    echo "[Entrypoint]"
    echo "[Entrypoint] Falling back to the built-in template config for this run."
    echo "[Entrypoint] Your settings will NOT be preserved across restarts until"
    echo "[Entrypoint] you create a real config.yaml file on the host."
    echo "[Entrypoint] =========================================================="
fi

if [ ! -f "$CONFIG_FILE" ]; then
    echo "[Entrypoint] No config.yaml found, creating initial configuration..."
    
    # Create backup of any existing config before overwriting (safety measure)
    if [ -f "$CONFIG_FILE" ]; then
        BACKUP_NAME="${CONFIG_FILE}.backup.$(date +%s)"
        cp "$CONFIG_FILE" "$BACKUP_NAME" 2>/dev/null && \
            echo "[Entrypoint] Backed up existing config to $BACKUP_NAME" || true
    fi
    
    if [ -f "$USER_CONFIG" ]; then
        echo "[Entrypoint] Using user-supplied config from $USER_CONFIG..."
        cp "$USER_CONFIG" "$CONFIG_FILE"
    elif [ -f "$TEMPLATE_FILE" ]; then
        echo "[Entrypoint] No user config found — copying built-in template to $CONFIG_FILE"
        echo "[Entrypoint] You can customize settings via the Web UI at http://localhost:8088"
        cp "$TEMPLATE_FILE" "$CONFIG_FILE"
    else
        echo "[Entrypoint] Creating minimal config..."
        cat > "$CONFIG_FILE" << 'EOF'
server:
    port: 8088
    host: 0.0.0.0
EOF
    fi
    
    normalize_file "$CONFIG_FILE"
    chmod 644 "$CONFIG_FILE"
    echo "[Entrypoint] Initial config created successfully"
fi

# Normalize existing config
normalize_file "$CONFIG_FILE"

# Note: server.host is forced to 0.0.0.0 via the AURAGO_SERVER_HOST env var
# (set in the Dockerfile), so no YAML manipulation is needed here.

# === 3. CONFIG MERGE ===
# The V5 config-merger handles everything:
#   - Adds missing sections from the template (deep merge)
#   - Repairs corrupted configs (section-by-section salvage)
#   - Guarantees valid YAML output (marshalled from parsed data)
#   - Atomic writes (temp → rename)

if [ -f "$TEMPLATE_FILE" ] && [ -f "$MERGER_BIN" ]; then
    echo "[Entrypoint] Checking/merging configuration..."
    MERGE_OUTPUT=$("$MERGER_BIN" -source "$CONFIG_FILE" -template "$TEMPLATE_FILE" 2>&1) || true
    echo "$MERGE_OUTPUT"
else
    echo "[Entrypoint] Skipping config merge (template or merger not found)"
fi

# === 3.5 VALIDATE CONFIG ===
# Use the AuraGo binary itself to detect YAML errors before starting.
# If the config is corrupted, restore from factory default and re-validate.
echo "[Entrypoint] Validating configuration syntax..."
VALIDATE_ERR=$(/app/aurago --config "$CONFIG_FILE" --check-config 2>&1)
if [ $? -ne 0 ]; then
    echo "[Entrypoint] WARNING: Config has YAML errors — restoring from factory defaults!"
    echo "[Entrypoint] Error was: $VALIDATE_ERR"
    # Back up the corrupt config so the user can inspect it later.
    CORRUPT_BACKUP="${CONFIG_FILE}.corrupt.$(date +%s)"
    if cp "$CONFIG_FILE" "$CORRUPT_BACKUP" 2>/dev/null; then
        echo "[Entrypoint] Previous config backed up to $CORRUPT_BACKUP"
    fi
    echo "[Entrypoint] Your previous settings have been reset. Reconfigure via the Web UI."
    cp "$TEMPLATE_FILE" "$CONFIG_FILE"
    normalize_file "$CONFIG_FILE"
    # Re-validate the restored template. If this fails, the image itself is broken.
    VALIDATE_ERR2=$(/app/aurago --config "$CONFIG_FILE" --check-config 2>&1)
    if [ $? -ne 0 ]; then
        echo "[Entrypoint] FATAL: Factory default config is also invalid — aborting."
        echo "[Entrypoint] Error was: $VALIDATE_ERR2"
        exit 1
    fi
fi
echo "[Entrypoint] Config OK"

# === 4. MASTER KEY SETUP ===
# Priority: env var > Docker Compose secret > .env file > auto-generate

if [ -z "${AURAGO_MASTER_KEY:-}" ]; then
    # Try Docker Compose file-based secret (mounted at /run/secrets/)
    DOCKER_SECRET="/run/secrets/aurago_master_key"
    if [ -f "$DOCKER_SECRET" ]; then
        echo "[Entrypoint] Loading master key from Docker secret ($DOCKER_SECRET)"
        export AURAGO_MASTER_KEY="$(cat "$DOCKER_SECRET" | tr -d '[:space:]')"
    elif [ -f "$ENV_FILE" ]; then
        echo "[Entrypoint] Loading master key from $ENV_FILE"
        # shellcheck source=/dev/null
        source "$ENV_FILE"
        export AURAGO_MASTER_KEY
    else
        echo "[Entrypoint] Generating new master key..."
        NEW_KEY=$(tr -dc 'a-f0-9' < /dev/urandom | head -c 64)
        export AURAGO_MASTER_KEY="$NEW_KEY"
        
        # Save to .env file (only if Docker secret not available)
        echo "AURAGO_MASTER_KEY=\"$NEW_KEY\"" > "$ENV_FILE"
        chmod 600 "$ENV_FILE"
        
        # SECURITY: Don't log the actual key value
        echo "=========================================================================="
        echo "⚠️  IMPORTANT: A new Master Key was generated"
        echo "   Saved to: $ENV_FILE (inside container volume)"
        echo ""
        echo "   BACKUP THIS KEY IMMEDIATELY:"
        echo "   docker compose exec aurago sh -c 'cat /app/data/.env | grep AURAGO_MASTER_KEY'"
        echo ""
        echo "   For better security on next restart, use Docker Compose secrets:"
        echo "   1. Save the key: echo '<your-key>' > aurago_master.key && chmod 600 aurago_master.key"
        echo "   2. The docker-compose.yml already references this file as a secret"
        echo "   3. Restart: docker compose down && docker compose up -d"
        echo "=========================================================================="
    fi
fi

# === 5. START APPLICATION ===

echo "[Entrypoint] Starting AuraGo..."
exec "$@"
