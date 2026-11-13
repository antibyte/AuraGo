package invasion

// Check the existing key without sourcing .env or returning its contents. This
// runs before stopping an Egg and again before publishing a missing .env.
func sshEggPreflightScript(baseDir string) string {
	return "set -eu\ndir=" + shellPath(baseDir) + `
env_file="$dir/.env"
if [ -e "$env_file" ] || [ -L "$env_file" ]; then
    if [ ! -f "$env_file" ] || [ ! -r "$env_file" ] || [ -L "$env_file" ]; then
        echo 'Existing Egg .env must be a readable regular private file; recover it before deployment.' >&2
        exit 1
    fi
    if [ "$(grep -c '^AURAGO_MASTER_KEY=' "$env_file" || true)" != '1' ]; then
        echo 'Existing Egg .env must contain exactly one master-key binding; resolve ambiguity before deployment.' >&2
        exit 1
    fi
    key=$(sed -n 's/^AURAGO_MASTER_KEY=//p' "$env_file" | head -n 1 | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')
    case "$key" in
        \"*\") key="${key#\"}"; key="${key%\"}" ;;
        \'*\') key="${key#\'}"; key="${key%\'}" ;;
    esac
    if ! printf '%s' "$key" | grep -Eq '^[0-9a-fA-F]{64}$'; then
        echo 'Existing Egg .env has no valid master key; recover the original key before deployment.' >&2
        exit 1
    fi
    chmod 600 -- "$env_file"
elif [ -e "$dir/data/vault.bin" ] || [ -L "$dir/data/vault.bin" ]; then
    echo 'Existing Egg Vault has no .env master key; authenticated operator recovery is required.' >&2
    exit 1
fi
if [ -x "$dir/aurago" ] && [ "$(cat "$dir/.egg-security-version" 2>/dev/null || true)" = '1' ]; then
    echo recoverable
elif [ -e "$dir/aurago" ] || [ -e "$dir/data/vault.bin" ]; then
    echo unverified
else
    echo fresh
fi
`
}
