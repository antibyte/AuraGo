#!/bin/bash
# kill_all.sh — reliably stop all AuraGo processes and clean up lock files

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "Stopping AuraGo and Lifeboat..."

_kill_wait() {
    local label="$1"; shift
    local patterns=("$@")

    local found=false
    for pat in "${patterns[@]}"; do
        pgrep -f "$pat" >/dev/null 2>&1 && { found=true; break; }
    done
    $found || { echo "  $label: not running"; return 0; }

    # SIGTERM first
    for pat in "${patterns[@]}"; do
        pkill -TERM -f "$pat" 2>/dev/null || true
    done

    # Wait up to 8 s for clean exit
    local waited=0
    while true; do
        local up=false
        for pat in "${patterns[@]}"; do
            pgrep -f "$pat" >/dev/null 2>&1 && { up=true; break; }
        done
        $up || break
        sleep 1; waited=$((waited + 1))
        [ $waited -ge 8 ] && break
    done

    # SIGKILL if still alive
    for pat in "${patterns[@]}"; do
        if pgrep -f "$pat" >/dev/null 2>&1; then
            echo "  $label still alive after ${waited}s — sending SIGKILL"
            pkill -KILL -f "$pat" 2>/dev/null || true
        fi
    done

    sleep 1
    echo "  $label stopped"
}

_kill_wait "aurago"   "bin/aurago_linux" "bin/aurago"
_kill_wait "lifeboat" "bin/lifeboat_linux" "bin/lifeboat"

# Remove all known lock files
for lockfile in \
    "$DIR/data/aurago.lock" \
    "$DIR/data/maintenance.lock" \
    "$DIR/aurago.lock" \
    "$DIR/lifeboat.lock"
do
    if [ -f "$lockfile" ]; then
        rm -f "$lockfile"
        echo "  Removed lock: $(basename "$lockfile")"
    fi
done

echo "All AuraGo processes stopped."
