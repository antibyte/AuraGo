#!/usr/bin/env bash
set -e

# Config-Merger Safety Wrapper - Bulletproof configuration management
# This script ensures safe configuration updates with multiple fallback layers

CONFIG_FILE="/app/data/config.yaml"
LEGACY_USER_CONFIG="/app/config.yaml.user"
USER_CONFIG_DIR="/run/optional-config"
USER_CONFIG=""
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

read_master_key_from_env_file() {
    local file="$1"
    local raw=""
    if [ ! -f "$file" ]; then
        return 0
    fi
    raw=$(grep -E '^AURAGO_MASTER_KEY=' "$file" | head -n 1 || true)
    raw="${raw#AURAGO_MASTER_KEY=}"
    raw="$(printf '%s' "$raw" | tr -d '[:space:]')"
    raw="${raw%\"}"
    raw="${raw#\"}"
    raw="${raw%\'}"
    raw="${raw#\'}"
    printf '%s' "$raw"
}

persist_master_key_fallback() {
    local key="$1"
    if [ -z "$key" ]; then
        return 0
    fi
    printf 'AURAGO_MASTER_KEY="%s"\n' "$key" > "$ENV_FILE"
    chmod 600 "$ENV_FILE"
}

# Normalize the baked-in template immediately — it may have CRLF endings
# because it was COPYd from a Windows development machine.
normalize_file "$TEMPLATE_FILE"

# === 1. INITIAL CONFIG SETUP ===

# Resolve optional user config source.
# Preferred modern path: /run/optional-config/config.yaml (directory mount).
# Legacy compatibility: /app/config.yaml.user (file mount from older compose files).
if [ -f "$USER_CONFIG_DIR/config.yaml" ]; then
    USER_CONFIG="$USER_CONFIG_DIR/config.yaml"
elif [ -f "$USER_CONFIG_DIR/config.yml" ]; then
    USER_CONFIG="$USER_CONFIG_DIR/config.yml"
elif [ -f "$LEGACY_USER_CONFIG" ]; then
    USER_CONFIG="$LEGACY_USER_CONFIG"
elif [ -d "$LEGACY_USER_CONFIG" ]; then
    echo "[Entrypoint] =========================================================="
    echo "[Entrypoint] WARNING: $LEGACY_USER_CONFIG is a directory, not a file!"
    echo "[Entrypoint]"
    echo "[Entrypoint] This usually comes from an older compose setup that bind-mounted"
    echo "[Entrypoint] ./config.yaml before the file existed on the host."
    echo "[Entrypoint] AuraGo will ignore it and use the built-in template instead."
    echo "[Entrypoint] Recommended fix: remove the legacy ./config.yaml directory and"
    echo "[Entrypoint] switch to the new optional ./config/config.yaml layout."
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
    
    if [ -n "$USER_CONFIG" ] && [ -f "$USER_CONFIG" ]; then
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
# Priority:
#   1. explicit env var / stack-secret environment injection
#   2. Docker secret (/run/secrets/aurago_master_key)
#   3. optional host-managed secret file (/run/optional-secrets/aurago_master.key)
#   4. persisted data/.env
#   5. auto-generate

PERSISTED_ENV_KEY="$(read_master_key_from_env_file "$ENV_FILE")"

if [ -n "${AURAGO_MASTER_KEY:-}" ]; then
    if [ -n "$PERSISTED_ENV_KEY" ] && [ "$PERSISTED_ENV_KEY" != "$AURAGO_MASTER_KEY" ]; then
        echo "[Entrypoint] =========================================================="
        echo "[Entrypoint] FATAL: External AURAGO_MASTER_KEY does not match persisted"
        echo "[Entrypoint]        master key in $ENV_FILE."
        echo "[Entrypoint]"
        echo "[Entrypoint] Current start would make the existing vault unreadable."
        echo "[Entrypoint] Fix one of these before restarting:"
        echo "[Entrypoint]   1. Restore the previous external AURAGO_MASTER_KEY"
        echo "[Entrypoint]   2. Remove the conflicting env injection / stack secret"
        echo "[Entrypoint]   3. If this is a fresh reset, delete $ENV_FILE and vault.bin"
        echo "[Entrypoint] =========================================================="
        exit 1
    fi
    if [ -z "$PERSISTED_ENV_KEY" ]; then
        echo "[Entrypoint] Persisting environment-provided master key to $ENV_FILE for restart safety"
        persist_master_key_fallback "$AURAGO_MASTER_KEY"
    fi
fi

if [ -z "${AURAGO_MASTER_KEY:-}" ]; then
    # Try Docker Compose file-based secret (mounted at /run/secrets/)
    DOCKER_SECRET="/run/secrets/aurago_master_key"
    OPTIONAL_SECRET_DIR="/run/optional-secrets"
    OPTIONAL_SECRET_FILE=""
    if [ -f "$OPTIONAL_SECRET_DIR/aurago_master.key" ]; then
        OPTIONAL_SECRET_FILE="$OPTIONAL_SECRET_DIR/aurago_master.key"
    elif [ -f "$OPTIONAL_SECRET_DIR/aurago_master_key" ]; then
        OPTIONAL_SECRET_FILE="$OPTIONAL_SECRET_DIR/aurago_master_key"
    fi

    if [ -f "$DOCKER_SECRET" ]; then
        echo "[Entrypoint] Loading master key from Docker secret ($DOCKER_SECRET)"
        export AURAGO_MASTER_KEY="$(cat "$DOCKER_SECRET" | tr -d '[:space:]')"
    elif [ -n "$OPTIONAL_SECRET_FILE" ]; then
        echo "[Entrypoint] Loading master key from optional host secret ($OPTIONAL_SECRET_FILE)"
        export AURAGO_MASTER_KEY="$(cat "$OPTIONAL_SECRET_FILE" | tr -d '[:space:]')"
    elif [ -f "$ENV_FILE" ]; then
        echo "[Entrypoint] Loading master key from $ENV_FILE"
        # shellcheck source=/dev/null
        source "$ENV_FILE"
        export AURAGO_MASTER_KEY
    else
        echo "[Entrypoint] Generating new master key..."
        NEW_KEY=$(tr -dc 'a-f0-9' < /dev/urandom | head -c 64)
        export AURAGO_MASTER_KEY="$NEW_KEY"
        
        # Save to .env file (only when no external secret source is available)
        persist_master_key_fallback "$NEW_KEY"
        
        # SECURITY: Don't log the actual key value
        echo "=========================================================================="
        echo "⚠️  IMPORTANT: A new Master Key was generated"
        echo "   Saved to: $ENV_FILE (inside container volume)"
        echo ""
        echo "   BACKUP THIS KEY IMMEDIATELY:"
        echo "   docker compose exec aurago sh -c 'cat /app/data/.env | grep AURAGO_MASTER_KEY'"
        echo ""
        echo "   For better security on next restart, place the key on the host:"
        echo "   1. mkdir -p secrets"
        echo "   2. echo '<your-key>' > secrets/aurago_master.key && chmod 600 secrets/aurago_master.key"
        echo "   3. Restart: docker compose down && docker compose up -d"
        echo "=========================================================================="
    fi
fi

# === 5. START APPLICATION ===

echo "[Entrypoint] Starting AuraGo..."
exec "$@"
