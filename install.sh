#!/usr/bin/env bash
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
#  AuraGo Quick Installer  (Linux x86_64 + arm64)
#
#  Usage:
#    Verified install (recommended):
#      curl -fsSLO https://github.com/antibyte/AuraGo/releases/latest/download/install.sh
#      curl -fsSLO https://github.com/antibyte/AuraGo/releases/latest/download/SHA256SUMS
#      sha256sum -c --ignore-missing SHA256SUMS && bash install.sh
#
#    Quick install (runs the script from the main branch):
#      curl -fsSL https://raw.githubusercontent.com/antibyte/AuraGo/main/install.sh | bash
#
#  Two installation modes:
#    A) Source build  — clones repo, requires Go 1.27.1+, builds from source
#    B) Binary install — downloads pre-built binaries + resources from
#       GitHub Releases. No git clone, no Go required.
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
set -euo pipefail

INSTALL_SUCCESS=0
SERVICE_FILE_CREATED=0
CREDENTIAL_FILE_CREATED=0
CREATED_CREDENTIAL_FILE=""
MIGRATED_MASTER_KEY=""
MIGRATED_ENV_FILE=""
SERVICE_ENABLED=0
RELEASE_CHECKSUMS_FILE=""
RELEASE_SIGNATURE_FILE=""
RELEASE_CERTIFICATE_FILE=""
INITIAL_PASSWORD_FILE=""
TMP_GO=""
TMPEXT=""
EXISTING_CONFIG_BAK=""
RELEASE_RESOURCES_FILE=""
RELEASE_ASSET_TEMP=""
PYTHON_PROBE_DIR=""

GITHUB_REPO="antibyte/AuraGo"
REPO="https://github.com/${GITHUB_REPO}.git"
INSTALL_DIR="${AURAGO_DIR:-$HOME/aurago}"
SYSTEMD_SERVICE="aurago"
GO_VERSION="1.27.1"
GO_INSTALL_DIR="/usr/local"

# ── UI & Typography ──────────────────────────────────────────────────────
# Text-mode look (logo, cards, steps, spinner). Plain lines when stdout is not a
# terminal; AURAGO_UI=plain, AURAGO_NO_ANIM=1 and NO_COLOR=1 are honoured.
# Edit scripts/aurago-tui.sh and run scripts/sync-tui-kit.sh - never this block.
# >>> AURAGO-TUI-KIT v1 >>>
# Text-mode presentation layer: gradient logo, gopher, cards, steps, spinner.
# Output only - nothing in here changes control flow or exit codes.
# Stdout is not a terminal (logs, server-started updates), TERM=dumb or
# AURAGO_UI=plain: plain ASCII lines, no escape codes, no animation.
#   AURAGO_UI=plain|full   force plain lines / force the full look
#   AURAGO_NO_ANIM=1       keep colours and layout, skip every animation
#   NO_COLOR=1             no colours (layout stays)
TUI_FULL=0; TUI_ANIM=0; TUI_COLOR=none; TUI_UTF8=0; TUI_TS=0
TUI_COLS=80; TUI_W=74
TUI_STEP=0; TUI_STEP_TOTAL=0; TUI_T0=$SECONDS; TUI_CURSOR_OFF=0
TUI_C=""; TUI_R=0; TUI_G=0; TUI_B=0; TUI_TEXT=""; TUI_LEN=0; TUI_RULE=""
RED=""; YELLOW=""; GREEN=""; CYAN=""; BLUE=""; BOLD=""; DIM=""; NC=""
T_OK='OK'; T_INFO='->'; T_WARN='!!'; T_ERR='XX'; T_DOT='*'; T_ON='#'; T_OFF='-'
T_H='-'; T_V='|'; T_TL='+'; T_TR='+'; T_BL='+'; T_BR='+'; T_ARROW='->'

_tui_detect() {
    local ui="${AURAGO_UI:-auto}" term="${TERM:-dumb}" v first="" probe cols=""
    case "$ui" in
        plain) ;;
        full)  TUI_FULL=1 ;;
        *)     if [ -t 1 ] && [ "$term" != "dumb" ]; then TUI_FULL=1; fi ;;
    esac
    if printf '%(%H)T' -1 >/dev/null 2>&1; then TUI_TS=1; fi
    if [ "$TUI_FULL" != 1 ]; then return 0; fi

    if [ -z "${NO_COLOR:-}" ]; then
        case "${COLORTERM:-}" in
            truecolor|24bit) TUI_COLOR=truecolor ;;
            *) case "$term" in
                   *256color*|xterm*|screen*|tmux*|rxvt*|linux|alacritty|foot|kitty|wezterm*) TUI_COLOR=256 ;;
                   *) TUI_COLOR=basic ;;
               esac ;;
        esac
    fi

    # UTF-8 glyphs need a bash that counts characters, not bytes. When the
    # locale is unset or C/POSIX, fall back to an unexported LC_ALL so child
    # processes keep their environment untouched.
    probe=$'\xe2\x96\x88'
    for v in "${LC_ALL:-}" "${LC_CTYPE:-}" "${LANG:-}"; do
        if [ -n "$v" ]; then first="$v"; break; fi
    done
    case "$first" in
        *[Uu][Tt][Ff]-8*|*[Uu][Tt][Ff]8*|""|C|POSIX)
            if [ "${#probe}" -eq 1 ] && [ -n "$first" ] && [ "$first" != C ] && [ "$first" != POSIX ]; then
                TUI_UTF8=1
            elif [ -z "${LC_ALL:-}" ]; then
                for v in C.UTF-8 C.utf8 en_US.UTF-8 en_US.utf8; do
                    LC_ALL="$v"
                    if [ "${#probe}" -eq 1 ]; then TUI_UTF8=1; break; fi
                done
                if [ "$TUI_UTF8" != 1 ]; then unset LC_ALL; else export -n LC_ALL; fi
            fi ;;
    esac

    cols="${COLUMNS:-}"
    case "$cols" in ''|*[!0-9]*) cols="" ;; esac
    if [ -z "$cols" ]; then cols="$(stty size 2>/dev/null </dev/tty | awk '{print $2}' || true)"; fi
    if [ -z "$cols" ] && command -v tput >/dev/null 2>&1; then cols="$(tput cols 2>/dev/null </dev/tty || true)"; fi
    case "$cols" in ''|*[!0-9]*) cols=80 ;; esac
    if [ "$cols" -lt 20 ]; then cols=80; fi
    TUI_COLS="$cols"
    TUI_W=$((cols - 4))
    if [ "$TUI_W" -gt 76 ]; then TUI_W=76; fi
    if [ "$TUI_W" -lt 36 ]; then TUI_W=36; fi

    if [ "$TUI_COLOR" != none ] && [ -t 1 ] && [ -z "${AURAGO_NO_ANIM:-}" ] && [ -z "${CI:-}" ] \
        && sleep 0.01 2>/dev/null; then
        TUI_ANIM=1
    fi

    if [ "$TUI_UTF8" = 1 ]; then
        T_OK='✔'; T_INFO='›'; T_WARN='▲'; T_ERR='✖'; T_DOT='◆'; T_ON='▰'; T_OFF='▱'
        T_H='─'; T_V='│'; T_TL='╭'; T_TR='╮'; T_BL='╰'; T_BR='╯'; T_ARROW='➜'
    fi
    return 0
}

# Colour escapes. _tui_fg/_tui_bg take r g b (0-255) and set TUI_C.
_tui_fg() {
    case "$TUI_COLOR" in
        truecolor) TUI_C=$'\033[38;2;'"$1;$2;$3"'m' ;;
        256) TUI_C=$'\033[38;5;'"$((16 + 36 * (($1 * 5 + 127) / 255) + 6 * (($2 * 5 + 127) / 255) + (($3 * 5 + 127) / 255)))"'m' ;;
        basic) if [ "$3" -gt "$1" ] && [ "$3" -gt "$2" ]; then TUI_C=$'\033[34m'; else TUI_C=$'\033[36m'; fi ;;
        *) TUI_C="" ;;
    esac
    return 0
}
_tui_bg() {
    case "$TUI_COLOR" in
        truecolor) TUI_C=$'\033[48;2;'"$1;$2;$3"'m' ;;
        256) TUI_C=$'\033[48;5;'"$((16 + 36 * (($1 * 5 + 127) / 255) + 6 * (($2 * 5 + 127) / 255) + (($3 * 5 + 127) / 255)))"'m' ;;
        *) TUI_C="" ;;
    esac
    return 0
}
# _tui_mix t(0-1000) r1 g1 b1 r2 g2 b2  ->  TUI_R/G/B
_tui_mix() {
    TUI_R=$(($2 + ($5 - $2) * $1 / 1000))
    TUI_G=$(($3 + ($6 - $3) * $1 / 1000))
    TUI_B=$(($4 + ($7 - $4) * $1 / 1000))
}
# The aura gradient: turquoise -> azure -> violet.  t = 0..1000
_tui_aura() {
    local t="$1"
    if [ "$t" -lt 0 ]; then t=0; elif [ "$t" -gt 1000 ]; then t=1000; fi
    if [ "$t" -le 500 ]; then
        _tui_mix $((t * 2)) 45 226 200 56 140 255
    else
        _tui_mix $(((t - 500) * 2)) 56 140 255 176 110 255
    fi
}
# aura colour at t, scaled to pct percent brightness -> TUI_C
_tui_aura_fg() {
    _tui_aura "$1"
    _tui_fg $((TUI_R * ${2:-100} / 100)) $((TUI_G * ${2:-100} / 100)) $((TUI_B * ${2:-100} / 100))
}

_tui_palette() {
    if [ "$TUI_COLOR" = none ]; then return 0; fi
    BOLD=$'\033[1m'; DIM=$'\033[2m'; NC=$'\033[0m'
    if [ "$TUI_COLOR" = basic ]; then
        RED=$'\033[31m'; YELLOW=$'\033[33m'; GREEN=$'\033[32m'; CYAN=$'\033[36m'; BLUE=$'\033[34m'
        return 0
    fi
    _tui_fg 255 92 108;  RED="$TUI_C"
    _tui_fg 255 200 64;  YELLOW="$TUI_C"
    _tui_fg 92 224 150;  GREEN="$TUI_C"
    _tui_fg 45 226 200;  CYAN="$TUI_C"
    _tui_fg 70 150 255;  BLUE="$TUI_C"
    return 0
}

# Gradient rule string of width $1 (default TUI_W) at brightness $2 -> TUI_TEXT
_tui_rule_str() {
    local w="${1:-$TUI_W}" pct="${2:-55}" i t out=""
    for ((i = 0; i < w; i++)); do
        t=$((i * 1000 / (w > 1 ? w - 1 : 1)))
        _tui_aura_fg "$t" "$pct"
        out+="${TUI_C}${T_H}"
    done
    TUI_TEXT="${out}${NC}"
    return 0
}

# Visible length of a string that may contain SGR escapes -> TUI_LEN
_tui_vlen() {
    local s="$1" esc=$'\033' pre rest
    while [[ "$s" == *"$esc["* ]]; do
        pre="${s%%"$esc"\[*}"
        rest="${s#*"$esc["}"
        s="${pre}${rest#*m}"
    done
    TUI_LEN=${#s}
    return 0
}

_tui_sleep() {
    if [ "$TUI_ANIM" = 1 ]; then sleep "$1"; fi
    return 0
}
tui_cursor_hide() {
    if [ "$TUI_ANIM" = 1 ]; then printf '\033[?25l'; TUI_CURSOR_OFF=1; fi
    return 0
}
# Call from the script's EXIT trap: a Ctrl-C mid-animation must not leave the
# cursor hidden.
tui_cleanup() {
    if [ "$TUI_CURSOR_OFF" = 1 ]; then printf '\033[?25h%s' "$NC"; TUI_CURSOR_OFF=0; fi
    return 0
}

tui_elapsed() {
    local s=$((SECONDS - TUI_T0))
    if [ "$s" -ge 60 ]; then printf '%dm %02ds' $((s / 60)) $((s % 60)); else printf '%ds' "$s"; fi
    return 0
}

# ── Status lines ───────────────────────────────────────────────────────
# tui_msg info|ok|warn|err "text"
tui_msg() {
    local kind="$1" text="$2" icon col tcol="" tag
    if [ "$TUI_FULL" != 1 ]; then
        case "$kind" in ok) tag="[ OK ]" ;; warn) tag="[WARN]" ;; err) tag="[FAIL]" ;; *) tag="[INFO]" ;; esac
        TUI_TEXT=""
        if [ "$TUI_TS" = 1 ]; then printf -v TUI_TEXT '%(%H:%M:%S)T ' -1; fi
        printf '%s%s %b\n' "$TUI_TEXT" "$tag" "$text"
        return 0
    fi
    case "$kind" in
        ok)   icon="$T_OK";   col="$GREEN" ;;
        warn) icon="$T_WARN"; col="$YELLOW"; tcol="$YELLOW" ;;
        err)  icon="$T_ERR";  col="$RED";    tcol="$RED" ;;
        *)    icon="$T_INFO"; col="$CYAN" ;;
    esac
    printf '  %s%s%s  %s%b%s\n' "$col" "$icon" "$NC" "$tcol" "$text" "$NC"
    return 0
}

# Section header with step counter and progress pips.
tui_section() {
    local title="$1" n="$TUI_STEP_TOTAL" i pips="" left right pad
    TUI_STEP=$((TUI_STEP + 1))
    if [ "$TUI_FULL" != 1 ]; then
        if [ "$n" -gt 0 ]; then printf '\n==> [%s/%s] %s\n' "$TUI_STEP" "$n" "$title"; else printf '\n==> %s\n' "$title"; fi
        return 0
    fi
    left="  ${T_DOT} ${title}"
    if [ "$n" -gt 0 ]; then
        for ((i = 1; i <= n; i++)); do
            if [ "$i" -le "$TUI_STEP" ]; then
                _tui_aura_fg $((i * 1000 / n)); pips+="${TUI_C}${T_ON}"
            else
                pips+="${DIM}${T_OFF}"
            fi
        done
        right="${pips}${NC} ${DIM}${TUI_STEP}/${n}${NC}"
        pad=$((TUI_W + 2 - ${#left} - n - ${#TUI_STEP} - ${#n} - 2))
    else
        right=""; pad=0
    fi
    if [ "$pad" -lt 2 ]; then pad=2; fi
    _tui_aura_fg $((n > 0 ? TUI_STEP * 1000 / n : 0))
    printf '\n%s%s%s%s%*s%s\n' "$TUI_C" "$BOLD" "$left" "$NC" "$pad" "" "$right"
    _tui_rule_str "$TUI_W" 45
    printf '  %s\n' "$TUI_TEXT"
    _tui_sleep 0.05
    return 0
}
# Unnumbered header (does not advance the step counter).
tui_subsection() {
    if [ "$TUI_FULL" != 1 ]; then printf '\n--- %s ---\n' "$1"; return 0; fi
    _tui_aura_fg 500
    printf '\n  %s%s%s %s%s\n' "$TUI_C" "$T_ARROW" "$NC" "$BOLD$1" "$NC"
    return 0
}

# ── Cards ──────────────────────────────────────────────────────────────
# tui_box ok|warn|err|info|brand "TITLE" line...   (lines may hold SGR codes)
tui_box() {
    local kind="$1" title="$2" icon="" r1 g1 b1 r2 g2 b2 line l iw=0 maxlen=0 p total
    local -a border=()
    shift 2
    case "$kind" in
        ok)    icon="$T_OK";   r1=92;  g1=224; b1=150; r2=45;  g2=226; b2=200 ;;
        warn)  icon="$T_WARN"; r1=255; g1=200; b1=64;  r2=255; g2=130; b2=60 ;;
        err)   icon="$T_ERR";  r1=255; g1=92;  b1=108; r2=255; g2=140; b2=90 ;;
        brand) icon="$T_DOT";  r1=45;  g1=226; b1=200; r2=176; g2=110; b2=255 ;;
        *)     icon="$T_INFO"; r1=45;  g1=226; b1=200; r2=70;  g2=150; b2=255 ;;
    esac
    _tui_vlen "$icon $title"; iw=$((TUI_LEN + 6))
    for line in "$@"; do
        _tui_vlen "$line"
        if [ "$TUI_LEN" -gt "$maxlen" ]; then maxlen="$TUI_LEN"; fi
    done
    if [ $((maxlen + 4)) -gt "$iw" ]; then iw=$((maxlen + 4)); fi
    if [ "$iw" -lt 44 ]; then iw=44; fi
    if [ "$iw" -gt $((TUI_W - 2)) ] && [ $((TUI_W - 2)) -ge 40 ]; then iw=$((TUI_W - 2)); fi
    total=$((iw + 2))
    for ((p = 0; p < total; p++)); do
        _tui_mix $((p * 1000 / (total - 1))) "$r1" "$g1" "$b1" "$r2" "$g2" "$b2"
        _tui_fg "$TUI_R" "$TUI_G" "$TUI_B"
        border[p]="$TUI_C"
    done
    # top edge:  ╭─ ✔ TITLE ────────╮
    _tui_vlen "$icon $title"
    l="  ${border[0]}${T_TL}${border[1]}${T_H} ${BOLD}${icon} ${title}${NC} "
    for ((p = TUI_LEN + 4; p < total - 1; p++)); do l+="${border[p]}${T_H}"; done
    printf '%s%s%s\n' "$l" "${border[total - 1]}${T_TR}" "$NC"
    _tui_sleep 0.02
    for line in "$@"; do
        _tui_vlen "$line"
        p=$((iw - 2 - TUI_LEN)); if [ "$p" -lt 0 ]; then p=0; fi
        printf '  %s%s%s  %s%*s%s%s%s%s\n' "${border[0]}" "$T_V" "$NC" "$line" "$p" "" \
            "$NC" "${border[total - 1]}" "$T_V" "$NC"
        _tui_sleep 0.02
    done
    l="  ${border[0]}${T_BL}"
    for ((p = 1; p < total - 1; p++)); do l+="${border[p]}${T_H}"; done
    printf '%s%s%s\n' "$l" "${border[total - 1]}${T_BR}" "$NC"
    return 0
}

# Fatal error card on stderr (callers still `exit 1` themselves).
tui_die() {
    local line
    local -a lines=()
    if [ "$TUI_FULL" != 1 ]; then tui_msg err "$1" >&2; return 0; fi
    while IFS= read -r line; do lines+=("${RED}${line}${NC}"); done < <(printf '%b\n' "$1" | fold -s -w $((TUI_W - 10)) 2>/dev/null || printf '%b\n' "$1")
    if [ "${#lines[@]}" -eq 0 ]; then lines=("$1"); fi
    tui_box err "ERROR" "${lines[@]}" >&2
    return 0
}

# ── Gradient text ──────────────────────────────────────────────────────
# tui_grad "text" [t0=0] [t1=1000] [delay]  - coloured text, no newline
tui_grad() {
    local s="$1" t0="${2:-0}" t1="${3:-1000}" delay="${4:-}" n i ch out=""
    n=${#s}
    if [ "$TUI_COLOR" = none ]; then printf '%s' "$s"; return 0; fi
    for ((i = 0; i < n; i++)); do
        ch="${s:i:1}"
        _tui_aura_fg $((t0 + (t1 - t0) * i / (n > 1 ? n - 1 : 1)))
        if [ -n "$delay" ] && [ "$TUI_ANIM" = 1 ]; then
            printf '%s%s' "$TUI_C" "$ch"; sleep "$delay"
        else
            out+="${TUI_C}${ch}"
        fi
    done
    printf '%s%s' "$out" "$NC"
    return 0
}

# ── Logo ───────────────────────────────────────────────────────────────
TUI_LOGO_H=0; TUI_LOGO_W=0; TUI_LOGO_SHADE=0; TUI_LOGO_MASCOT=0
TUI_LOGO_CH=(); TUI_LOGO_ROW=(); TUI_COL_BRIGHT=(); TUI_COL_DARK=(); TUI_MASCOT=()

# Pixel palette of the gopher; "." is transparent (-1).
_tui_px() {
    case "$1" in
        d) TUI_R=17;  TUI_G=94;  TUI_B=89 ;;
        t) TUI_R=45;  TUI_G=212; TUI_B=191 ;;
        l) TUI_R=167; TUI_G=243; TUI_B=230 ;;
        w) TUI_R=255; TUI_G=255; TUI_B=255 ;;
        k) TUI_R=15;  TUI_G=23;  TUI_B=42 ;;
        n) TUI_R=62;  TUI_G=44;  TUI_B=70 ;;
        *) TUI_R=-1;  TUI_G=-1;  TUI_B=-1 ;;
    esac
    return 0
}
# Half-block pixel gopher (18x12 px -> 18x6 cells): turquoise, big eyes, buck teeth.
_tui_mascot_build() {
    local -a half=("..dd....." ".dttd...." ".dttddddd" "dtttttttt" "dtttwwttt" "dttwkkwtt"
                   "dttwkkwtt" "dtttwwttn" "dttttllln" "dtttllwwd" "dtttllwwd" ".dddddddd")
    local r c k i top tc bc fr fg fb br bg bb line cell
    local -a full=()
    TUI_MASCOT=()
    for ((r = 0; r < 12; r++)); do
        k="${half[r]}"; top=""
        for ((i = 8; i >= 0; i--)); do top+="${k:i:1}"; done
        full[r]="${k}${top}"
    done
    for ((r = 0; r < 12; r += 2)); do
        line=""
        for ((c = 0; c < 18; c++)); do
            tc="${full[r]:c:1}"; bc="${full[r + 1]:c:1}"
            cell=""
            if [ "$tc" = "." ] && [ "$bc" = "." ]; then
                cell=" "
            else
                _tui_px "$tc"; fr=$TUI_R; fg=$TUI_G; fb=$TUI_B
                _tui_px "$bc"; br=$TUI_R; bg=$TUI_G; bb=$TUI_B
                if [ "$tc" = "." ]; then
                    _tui_fg "$br" "$bg" "$bb"; cell="${TUI_C}▄"
                elif [ "$bc" = "." ]; then
                    _tui_fg "$fr" "$fg" "$fb"; cell="${TUI_C}▀"
                elif [ "$tc" = "$bc" ]; then
                    _tui_fg "$fr" "$fg" "$fb"; cell="${TUI_C}█"
                else
                    _tui_fg "$fr" "$fg" "$fb"; cell="${TUI_C}"
                    _tui_bg "$br" "$bg" "$bb"; cell+="${TUI_C}▀"
                fi
                cell+=$'\033[0m'
            fi
            line+="$cell"
        done
        TUI_MASCOT[r / 2]="$line"
    done
    return 0
}

# AURAGO in the ANSI-shadow style, assembled letter by letter so every row
# keeps the exact glyph widths (A=8 U=9 R=8 A=8 G=9 O=9).
_tui_logo_define_big() {
    local A=(" █████╗ " "██╔══██╗" "███████║" "██╔══██║" "██║  ██║" "╚═╝  ╚═╝")
    local U=("██╗   ██╗" "██║   ██║" "██║   ██║" "██║   ██║" "╚██████╔╝" " ╚═════╝ ")
    local R=("██████╗ " "██╔══██╗" "██████╔╝" "██╔══██╗" "██║  ██║" "╚═╝  ╚═╝")
    local G=(" ██████╗ " "██╔════╝ " "██║  ███╗" "██║   ██║" "╚██████╔╝" " ╚═════╝ ")
    local O=(" ██████╗ " "██╔═══██╗" "██║   ██║" "██║   ██║" "╚██████╔╝" " ╚═════╝ ")
    local r
    TUI_LOGO_ROW=()
    for ((r = 0; r < 6; r++)); do
        TUI_LOGO_ROW[r]="${A[r]}${U[r]}${R[r]}${A[r]}${G[r]}${O[r]}"
    done
    TUI_LOGO_SHADE=1
    return 0
}
# "AuraGo" in plain figlet letters - ASCII only, for terminals without UTF-8.
_tui_logo_define_small() {
    TUI_LOGO_ROW=(
        $'    _                     ____       '
        $'   / \\  _   _ _ __ __ _   / ___| ___  '
        $'  / _ \\| | | | \'__/ _` | | |  _ / _ \\ '
        $' / ___ \\ |_| | | | (_| | | |_| | (_) |'
        $'/_/   \\_\\__,_|_|  \\__,_|  \\____|\\___/ ')
    TUI_LOGO_SHADE=0
    return 0
}
# Pick and prepare the logo that fits this terminal.
_tui_logo_prepare() {
    local r c row t
    TUI_LOGO_ROW=(); TUI_LOGO_CH=(); TUI_COL_BRIGHT=(); TUI_COL_DARK=()
    TUI_LOGO_H=0; TUI_LOGO_W=0; TUI_LOGO_MASCOT=0
    if [ "$TUI_UTF8" = 1 ] && [ "$TUI_COLS" -ge 58 ]; then
        _tui_logo_define_big
        if [ "$TUI_COLOR" = truecolor ] || [ "$TUI_COLOR" = 256 ]; then
            if [ "$TUI_COLS" -ge 76 ]; then _tui_mascot_build; TUI_LOGO_MASCOT=1; fi
        fi
    elif [ "$TUI_COLS" -ge 46 ]; then
        _tui_logo_define_small
    else
        TUI_LOGO_ROW=()
        return 0
    fi
    TUI_LOGO_H=${#TUI_LOGO_ROW[@]}
    TUI_LOGO_W=0
    for row in "${TUI_LOGO_ROW[@]}"; do
        if [ "${#row}" -gt "$TUI_LOGO_W" ]; then TUI_LOGO_W=${#row}; fi
    done
    for ((r = 0; r < TUI_LOGO_H; r++)); do
        row="${TUI_LOGO_ROW[r]}"
        while [ "${#row}" -lt "$TUI_LOGO_W" ]; do row+=" "; done
        for ((c = 0; c < TUI_LOGO_W; c++)); do TUI_LOGO_CH[r * TUI_LOGO_W + c]="${row:c:1}"; done
    done
    for ((c = 0; c < TUI_LOGO_W; c++)); do
        t=$((c * 1000 / (TUI_LOGO_W - 1)))
        _tui_aura_fg "$t" 100; TUI_COL_BRIGHT[c]="$TUI_C"
        _tui_aura_fg "$t" 52;  TUI_COL_DARK[c]="$TUI_C"
    done
    return 0
}

# One logo frame; $1 = shimmer centre column (-99 = none), $2 = per-row delay.
_tui_logo_frame() {
    local bc="$1" delay="${2:-}" r c d ch esc last line k
    for ((r = 0; r < TUI_LOGO_H; r++)); do
        line="  "
        if [ "$TUI_LOGO_MASCOT" = 1 ]; then
            k=$((r + (TUI_LOGO_H - 6) / 2))
            if [ "$k" -ge 0 ] && [ "$k" -lt 6 ]; then line+="${TUI_MASCOT[k]}  "; else line+="                  "; fi
        fi
        last=""
        for ((c = 0; c < TUI_LOGO_W; c++)); do
            ch="${TUI_LOGO_CH[r * TUI_LOGO_W + c]}"
            if [ "$ch" = " " ]; then line+=" "; continue; fi
            d=$((c - bc)); if [ "$d" -lt 0 ]; then d=$((-d)); fi
            if [ "$TUI_LOGO_SHADE" = 1 ] && [ "$ch" != "█" ]; then esc="${TUI_COL_DARK[c]}"; else esc="${TUI_COL_BRIGHT[c]}"; fi
            if [ "$d" -lt 6 ] && [ "$TUI_COLOR" != basic ]; then
                # white-hot band sweeping across the letters
                k=$(((6 - d) * 100 / 6))
                _tui_fg $((200 + (55 * k / 100))) $((200 + (55 * k / 100))) 255
                esc="$TUI_C"
            fi
            if [ "$esc" != "$last" ]; then line+="$esc"; last="$esc"; fi
            line+="$ch"
        done
        printf '%s%s\033[K\n' "$line" "$NC"
        if [ -n "$delay" ]; then _tui_sleep "$delay"; fi
    done
    return 0
}

# tui_banner "Quick Installer" "AI Agent Framework for Linux"
tui_banner() {
    local title="$1" sub="${2:-}" c
    if [ "$TUI_FULL" != 1 ]; then
        printf '\nAuraGo %s%s\n' "$title" "${sub:+ - $sub}"
        return 0
    fi
    _tui_logo_prepare
    printf '\n'
    tui_cursor_hide
    if [ "$TUI_LOGO_H" -gt 0 ]; then
        if [ "$TUI_ANIM" = 1 ]; then
            _tui_logo_frame -99 0.045
            for ((c = -6; c <= TUI_LOGO_W + 6; c += 3)); do
                printf '\033[%dA' "$TUI_LOGO_H"
                _tui_logo_frame "$c"
                sleep 0.018
            done
            printf '\033[%dA' "$TUI_LOGO_H"
            _tui_logo_frame -99
        else
            _tui_logo_frame -99
        fi
    else
        printf '  %s' "$BOLD"; tui_grad "AuraGo" 0 1000; printf '\n'
    fi
    printf '\n  '
    tui_grad "One gopher. A ridiculous toolbox." 0 1000 0.012
    printf '\n  %s%s %s%s%s\n' "$DIM" "$T_DOT" "$title" "${sub:+  ·  $sub}" "$NC"
    _tui_rule_str "$TUI_W" 70
    printf '  %s\n' "$TUI_TEXT"
    tui_cleanup
    return 0
}

# ── Spinner ────────────────────────────────────────────────────────────
# tui_run "label" cmd [args...]
#   Animated terminal: runs cmd quietly behind a spinner with a live timer,
#   prints its last output lines only if it fails, and returns its status.
#   Otherwise (logs, NO_ANIM): announces the label and runs cmd unchanged.
#   Optional:  TUI_RUN_WATCH=<file> shows the file's growing size,
#              TUI_RUN_SUDO=1 asks for the sudo password before the spinner,
#              TUI_RUN_EPHEMERAL=1 leaves no status line (the caller reports);
#              a failure still prints the command's last output lines.
tui_run() {
    local label="$1" log pid rc=0 t0=$SECONDS i=0 frame size="" maxl
    local -a spin
    shift
    if [ "$TUI_ANIM" != 1 ]; then
        if [ -n "${TUI_RUN_EPHEMERAL:-}" ]; then "$@"; return $?; fi
        tui_msg info "$label"
        "$@"
        rc=$?
        if [ "$rc" -eq 0 ]; then tui_msg ok "$label ($((SECONDS - t0))s)"; fi
        return "$rc"
    fi
    if [ "${TUI_RUN_SUDO:-}" = 1 ] && [ -n "${SUDO:-}" ]; then
        $SUDO -v </dev/tty || true
    fi
    log="$(mktemp "${TMPDIR:-/tmp}/aurago-step.XXXXXX")" || { tui_msg info "$label"; "$@"; return $?; }
    if [ "$TUI_UTF8" = 1 ]; then
        spin=(⠋ ⠙ ⠹ ⠸ ⠼ ⠴ ⠦ ⠧ ⠇ ⠏)
    else
        spin=('|' '/' '-' '\')
    fi
    maxl=$((TUI_W - 22))
    if [ "${#label}" -gt "$maxl" ]; then label="${label:0:maxl}…"; fi
    "$@" >"$log" 2>&1 </dev/null &
    pid=$!
    tui_cursor_hide
    while kill -0 "$pid" 2>/dev/null; do
        frame="${spin[i % ${#spin[@]}]}"
        _tui_aura_fg $(((i * 41) % 1000))
        if [ -n "${TUI_RUN_WATCH:-}" ] && [ -f "$TUI_RUN_WATCH" ] && [ $((i % 6)) -eq 0 ]; then
            size="$(stat -c %s "$TUI_RUN_WATCH" 2>/dev/null || echo 0)"
            size=" $((size / 1048576)).$(((size % 1048576) * 10 / 1048576)) MB"
        fi
        printf '\r\033[2K  %s%s%s  %s  %s%ss%s%s' "$TUI_C" "$frame" "$NC" "$label" "$DIM" $((SECONDS - t0)) "$size" "$NC"
        i=$((i + 1))
        sleep 0.08
    done
    wait "$pid" || rc=$?
    printf '\r\033[2K'
    tui_cleanup
    if [ "$rc" -eq 0 ]; then
        if [ -z "${TUI_RUN_EPHEMERAL:-}" ]; then
            printf '  %s%s%s  %s  %s%ss%s\n' "$GREEN" "$T_OK" "$NC" "$label" "$DIM" $((SECONDS - t0)) "$NC"
        fi
        rm -f "$log"
    else
        if [ -z "${TUI_RUN_EPHEMERAL:-}" ]; then
            printf '  %s%s%s  %s%s%s  %s(exit %s)%s\n' "$RED" "$T_ERR" "$NC" "$RED" "$label" "$NC" "$DIM" "$rc" "$NC"
        fi
        tail -n 14 "$log" 2>/dev/null | while IFS= read -r frame; do printf '     %s%s%s\n' "$DIM" "$frame" "$NC"; done
        if [ -z "${TUI_RUN_EPHEMERAL:-}" ]; then printf '     %sfull output: %s%s\n' "$DIM" "$log" "$NC"; else rm -f "$log"; fi
    fi
    return "$rc"
}

# ── Finale ─────────────────────────────────────────────────────────────
# Brief twinkling star field above a success card (animated terminals only).
tui_sparkle() {
    local rows=5 f r c line t
    local -a glyph=(✦ ✧ · ˚ ✶ +)
    if [ "$TUI_ANIM" != 1 ] || [ "$TUI_UTF8" != 1 ]; then return 0; fi
    tui_cursor_hide
    for ((r = 0; r < rows; r++)); do printf '\n'; done
    for ((f = 0; f < 16; f++)); do
        printf '\033[%dA' "$rows"
        for ((r = 0; r < rows; r++)); do
            line="  "
            for ((c = 0; c < TUI_W; c++)); do
                if [ $((RANDOM % 100)) -lt $((3 + (f < 8 ? f : 16 - f) / 2)) ]; then
                    t=$((c * 1000 / TUI_W))
                    _tui_aura_fg "$t" $((60 + RANDOM % 41))
                    line+="${TUI_C}${glyph[RANDOM % ${#glyph[@]}]}"
                else
                    line+=" "
                fi
            done
            printf '%s%s\033[K\n' "$line" "$NC"
        done
        sleep 0.06
    done
    printf '\033[%dA' "$rows"
    for ((r = 0; r < rows; r++)); do printf '\033[K\n'; done
    printf '\033[%dA' "$rows"
    tui_cleanup
    return 0
}

_tui_detect
_tui_palette
# <<< AURAGO-TUI-KIT v1 <<<

info() { tui_msg info "$*"; }
ok()   { tui_msg ok "$*"; }
warn() { tui_msg warn "$*"; }
die()  { tui_die "$*"; exit 1; }
section() { tui_section "$*"; }

# >>> AURAGO-BLUETOOTH-KIT v1 >>>
# Bluetooth host preparation (source: scripts/aurago-bluetooth.sh; sync with
# scripts/sync-bluetooth-kit.sh). Needs only the caller's SUDO (empty when
# root). Callers use btk_resolve_choice and btk_run_choice. No step aborts
# the caller; failures end up in a summary with the command to run by hand.

BTK_LOG="${BTK_LOG:-${TMPDIR:-/tmp}/aurago-bluetooth-setup.log}"
BTK_DONE=()
BTK_FAILED=()

btk_has() { command -v "$1" >/dev/null 2>&1; }
btk_info() { if declare -F info >/dev/null; then info "$*"; else printf '[INFO] %s\n' "$*"; fi; }
btk_ok() { if declare -F ok >/dev/null; then ok "$*"; else printf '[ OK ] %s\n' "$*"; fi; }
btk_warn() { if declare -F warn >/dev/null; then warn "$*"; else printf '[WARN] %s\n' "$*"; fi; }
btk_done() { BTK_DONE+=("$1"); btk_ok "$1"; }
# btk_fail <what failed> <command to run by hand>
btk_fail() { BTK_FAILED+=("$1|$2"); btk_warn "$1 failed. Run by hand: $2"; }

# btk_run <label> command...: spinner through the TUI kit when present,
# otherwise the output goes to $BTK_LOG.
btk_run() {
    local label="$1"
    shift
    if declare -F tui_run >/dev/null; then
        TUI_RUN_SUDO=1 tui_run "$label" "$@" </dev/null
    else
        btk_info "$label"
        "$@" >>"$BTK_LOG" 2>&1 </dev/null
    fi
}

btk_in_container() {
    if btk_has systemd-detect-virt && systemd-detect-virt --container --quiet >/dev/null 2>&1; then
        return 0
    fi
    [ -e "${BTK_ROOT:-}/.dockerenv" ] || [ -e "${BTK_ROOT:-}/run/.containerenv" ]
}

# Prints the first adapter name (hci0); fails when there is none.
btk_first_adapter() {
    local entry
    for entry in "${BTK_ROOT:-}"/sys/class/bluetooth/hci*; do
        [ -e "$entry" ] || continue
        case "${entry##*/}" in *:*) continue ;; esac
        printf '%s\n' "${entry##*/}"
        return 0
    done
    return 1
}
btk_adapter_present() { btk_first_adapter >/dev/null; }

btk_state_file() { printf '%s/data/bluetooth-setup\n' "$1"; }

# Prints the stored decision (enabled|declined) or nothing.
btk_read_state() {
    local file value=""
    file="$(btk_state_file "$1")"
    [ -r "$file" ] || return 0
    value="$(head -n 1 "$file" 2>/dev/null | tr -d '[:space:]')" || value=""
    case "$value" in enabled | declined) printf '%s\n' "$value" ;; esac
    return 0
}

btk_normalize_choice() {
    case "$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]')" in
        yes | y | true | 1 | on | enabled) printf 'enabled\n' ;;
        no | n | false | 0 | off | declined) printf 'declined\n' ;;
    esac
    return 0
}

# btk_resolve_choice <installdir> <flag yes|no|""> <may_prompt true|false>
# Prints enabled, declined or skip. Order: container (skip), flag,
# AURAGO_BLUETOOTH, stored decision, question on the terminal. Writes nothing.
btk_resolve_choice() {
    local dir="$1" flag="${2:-}" may_prompt="${3:-false}" choice="" answer=""
    local tty_out="${BTK_TTY_OUT:-/dev/tty}"
    if btk_in_container; then
        btk_info "Bluetooth setup skipped: Bluetooth is not supported inside containers." >&2
        printf 'skip\n'
        return 0
    fi
    choice="$(btk_normalize_choice "$flag")"
    [ -n "$choice" ] || choice="$(btk_normalize_choice "${AURAGO_BLUETOOTH:-}")"
    [ -n "$choice" ] || choice="$(btk_read_state "$dir")"
    if [ -z "$choice" ] && [ "$may_prompt" = true ]; then
        if ! btk_adapter_present; then
            printf '  %s\n' "No Bluetooth adapter detected; everything is prepared for a later dongle." >> "$tty_out"
        fi
        printf '  %b?%b  %s ' "${CYAN:-}" "${NC:-}" "Use Bluetooth in AuraGo (manage devices, headphones and speakers)? [y/N]:" >> "$tty_out"
        read -r answer < "${BTK_TTY_IN:-/dev/tty}" || true
        case "$answer" in
            [yYjJ] | [yY][eE][sS] | [jJ][aA]) choice=enabled ;;
            *) choice=declined ;;
        esac
    fi
    printf '%s\n' "${choice:-skip}"
}

# btk_write_state <installdir> <enabled|declined> [service-user]
btk_write_state() {
    local dir="$1" value="$2" user="${3:-}" file parent created=false
    file="$(btk_state_file "$dir")"
    parent="${file%/*}"
    if [ ! -d "$parent" ]; then
        mkdir -p "$parent" 2>/dev/null || $SUDO mkdir -p "$parent" || return 1
        created=true
    fi
    if ! { printf '%s\n' "$value" > "$file"; } 2>/dev/null; then
        printf '%s\n' "$value" | $SUDO tee "$file" >/dev/null || return 1
    fi
    if [ -n "$user" ] && [ "$user" != "$(id -un)" ]; then
        $SUDO chown "$user" "$file" 2>/dev/null || true
        if [ "$created" = true ]; then $SUDO chown "$user" "$parent" 2>/dev/null || true; fi
    fi
    return 0
}

btk_pkg_manager() {
    if btk_has apt-get; then printf 'apt\n'
    elif btk_has dnf; then printf 'dnf\n'
    elif btk_has pacman; then printf 'pacman\n'
    elif btk_has zypper; then printf 'zypper\n'
    else printf 'unknown\n'
    fi
}

btk_pipewire_pulse_unit() {
    local dir
    for dir in /usr/lib/systemd/user /lib/systemd/user /etc/systemd/user; do
        if [ -e "${BTK_ROOT:-}$dir/pipewire-pulse.service" ]; then return 0; fi
    done
    return 1
}

# An installed PulseAudio server stays; everything else gets PipeWire.
btk_audio_stack() {
    if btk_has pulseaudio && ! btk_pipewire_pulse_unit; then printf 'pulseaudio\n'; else printf 'pipewire\n'; fi
}

# btk_packages <manager> <pipewire|pulseaudio> <with_ffmpeg true|false>
btk_packages() {
    local list
    case "$1:$2" in
        apt:pipewire) list="bluez pipewire pipewire-bin pipewire-pulse wireplumber libspa-0.2-bluetooth" ;;
        apt:pulseaudio) list="bluez pulseaudio-module-bluetooth pulseaudio-utils" ;;
        dnf:pipewire) list="bluez pipewire pipewire-pulseaudio pipewire-utils wireplumber" ;;
        dnf:pulseaudio) list="bluez pulseaudio-module-bluetooth pulseaudio-utils" ;;
        pacman:pipewire) list="bluez bluez-utils pipewire pipewire-pulse wireplumber" ;;
        pacman:pulseaudio) list="bluez bluez-utils pulseaudio-bluetooth libpulse" ;;
        zypper:pipewire) list="bluez pipewire pipewire-pulseaudio pipewire-tools wireplumber" ;;
        zypper:pulseaudio) list="bluez pulseaudio-module-bluetooth pulseaudio-utils" ;;
        *) return 1 ;;
    esac
    if [ "$3" = true ]; then
        if [ "$1" = dnf ]; then list="$list ffmpeg-free"; else list="$list ffmpeg"; fi
    fi
    printf '%s\n' "$list"
}

btk_pkg_installed() {
    case "$1" in
        apt) dpkg-query -W -f='${Status}' "$2" 2>/dev/null | grep -q 'ok installed' ;;
        dnf | zypper) rpm -q "$2" >/dev/null 2>&1 ;;
        pacman) pacman -Q "$2" >/dev/null 2>&1 ;;
        *) return 1 ;;
    esac
}

btk_pkg_command() {
    case "$1" in
        apt) printf 'sudo apt-get install -y\n' ;;
        dnf) printf 'sudo dnf install -y\n' ;;
        pacman) printf 'sudo pacman -S --needed\n' ;;
        zypper) printf 'sudo zypper install\n' ;;
    esac
}

# btk_pkg_add <manager> package...
btk_pkg_add() {
    local mgr="$1"
    shift
    case "$mgr" in
        apt)
            btk_run "Installing $*" $SUDO env DEBIAN_FRONTEND=noninteractive apt-get install -y "$@" && return 0
            btk_run "Refreshing the package lists" $SUDO apt-get update
            btk_run "Installing $*" $SUDO env DEBIAN_FRONTEND=noninteractive apt-get install -y "$@"
            ;;
        dnf) btk_run "Installing $*" $SUDO dnf install -y "$@" ;;
        pacman)
            btk_run "Installing $*" $SUDO pacman -S --needed --noconfirm "$@" && return 0
            btk_run "Installing $*" $SUDO pacman -Sy --needed --noconfirm "$@"
            ;;
        zypper) btk_run "Installing $*" $SUDO zypper --non-interactive install "$@" ;;
        *) return 1 ;;
    esac
}

# btk_install_packages <pipewire|pulseaudio>: installs only what is missing.
btk_install_packages() {
    local mgr list pkg missing="" ffmpeg=false
    mgr="$(btk_pkg_manager)"
    btk_has ffmpeg || ffmpeg=true
    if ! list="$(btk_packages "$mgr" "$1" "$ffmpeg")"; then
        btk_fail "Installing the Bluetooth packages (unknown package manager)" \
            "install bluez, pipewire, pipewire-pulse, wireplumber, the PipeWire Bluetooth plugin and ffmpeg"
        return 1
    fi
    for pkg in $list; do
        btk_pkg_installed "$mgr" "$pkg" || missing="$missing $pkg"
    done
    missing="${missing# }"
    if [ -z "$missing" ]; then
        btk_done "Bluetooth packages are installed"
        return 0
    fi
    # shellcheck disable=SC2086 # package names are single words
    if btk_pkg_add "$mgr" $missing; then
        btk_done "Installed $missing"
        return 0
    fi
    btk_fail "Installing $missing" "$(btk_pkg_command "$mgr") $missing"
    return 1
}

btk_user_uid() { id -u "$1" 2>/dev/null || true; }

btk_user_home() {
    local home=""
    if btk_has getent; then home="$(getent passwd "$1" 2>/dev/null | cut -d: -f6)" || home=""; fi
    printf '%s\n' "$home"
}

# btk_as_user <user> <uid> command...: runs inside that user's systemd and
# D-Bus session (directly, through runuser as root, or through sudo).
btk_as_user() {
    local user="$1" uid="$2" runtime
    shift 2
    runtime="/run/user/$uid"
    if [ "$(id -un)" = "$user" ]; then
        XDG_RUNTIME_DIR="$runtime" DBUS_SESSION_BUS_ADDRESS="unix:path=$runtime/bus" "$@"
    elif [ "$(id -u)" = 0 ] && btk_has runuser; then
        runuser -u "$user" -- env XDG_RUNTIME_DIR="$runtime" DBUS_SESSION_BUS_ADDRESS="unix:path=$runtime/bus" "$@"
    else
        ${SUDO:-sudo} -u "$user" env XDG_RUNTIME_DIR="$runtime" DBUS_SESSION_BUS_ADDRESS="unix:path=$runtime/bus" "$@"
    fi
}

btk_enable_bluez() {
    if ! btk_has systemctl; then
        btk_fail "Starting the Bluetooth service" "start bluetoothd with your init system"
        return 1
    fi
    if systemctl is-enabled --quiet bluetooth.service 2>/dev/null &&
        systemctl is-active --quiet bluetooth.service 2>/dev/null; then
        btk_done "Bluetooth service is running"
    elif $SUDO systemctl enable --now bluetooth.service >>"$BTK_LOG" 2>&1; then
        btk_done "Bluetooth service enabled and started"
    else
        btk_fail "Starting the Bluetooth service" "sudo systemctl enable --now bluetooth.service"
        return 1
    fi
    if btk_has rfkill && rfkill list bluetooth 2>/dev/null | grep -qi 'soft blocked: yes'; then
        $SUDO rfkill unblock bluetooth >>"$BTK_LOG" 2>&1 ||
            btk_warn "Bluetooth is switched off in software; run: sudo rfkill unblock bluetooth"
    fi
    return 0
}

# Linger keeps the user's audio session running without a login.
btk_enable_linger() {
    local user="$1" uid="$2"
    if [ ! -e "${BTK_ROOT:-}/var/lib/systemd/linger/$user" ] &&
        ! $SUDO loginctl enable-linger "$user" >>"$BTK_LOG" 2>&1; then
        btk_fail "Keeping the audio session of $user running without a login" "sudo loginctl enable-linger $user"
        return 1
    fi
    if ! systemctl is-active --quiet "user@$uid.service" 2>/dev/null &&
        ! $SUDO systemctl start "user@$uid.service" >>"$BTK_LOG" 2>&1; then
        btk_fail "Starting the user session of $user" "sudo systemctl start user@$uid.service"
        return 1
    fi
    btk_done "Audio session of $user runs without a login"
}

btk_audio_units() {
    if [ "$1" = pulseaudio ]; then
        printf 'pulseaudio.socket\n'
    else
        printf 'pipewire.socket pipewire-pulse.socket wireplumber.service\n'
    fi
}

# btk_enable_user_units <user> <uid> <pipewire|pulseaudio>
btk_enable_user_units() {
    local user="$1" uid="$2" units
    units="$(btk_audio_units "$3")"
    # shellcheck disable=SC2086 # unit names are single words
    if btk_as_user "$user" "$uid" systemctl --user enable --now $units >>"$BTK_LOG" 2>&1; then
        btk_done "Audio services enabled for $user"
        return 0
    fi
    btk_fail "Enabling the audio services for $user" \
        "sudo -u $user env XDG_RUNTIME_DIR=/run/user/$uid systemctl --user enable --now $units"
    return 1
}

btk_wireplumber_version() {
    btk_has wireplumber || return 0
    wireplumber --version 2>/dev/null | grep -Eo '[0-9]+\.[0-9]+(\.[0-9]+)?' | tail -n 1
    return 0
}

# Without a logind seat (headless servers) WirePlumber keeps its BlueZ
# monitor off; audio devices then pair but fail with
# br-connection-profile-unavailable.
btk_wireplumber_headless() {
    local user="$1" uid="$2" home version dir file content
    home="$(btk_user_home "$user")"
    version="$(btk_wireplumber_version)"
    if [ -z "$home" ] || [ -z "$version" ]; then
        btk_fail "Configuring WirePlumber for a server without a screen" "install WirePlumber, then run ./update.sh --bluetooth"
        return 1
    fi
    case "$version" in
        0.4 | 0.4.*)
            dir="$home/.config/wireplumber/bluetooth.lua.d"
            file="$dir/80-aurago-bluez-headless.lua"
            content='bluez_monitor.properties["with-logind"] = false'
            ;;
        *)
            dir="$home/.config/wireplumber/wireplumber.conf.d"
            file="$dir/80-aurago-bluez-headless.conf"
            content="$(printf '%s\n' 'wireplumber.profiles = {' '  main = {' '    monitor.bluez.seat-monitoring = disabled' '  }' '}')"
            ;;
    esac
    if [ "$(cat "$file" 2>/dev/null)" = "$content" ]; then
        btk_done "WirePlumber handles Bluetooth without a screen"
        return 0
    fi
    if btk_as_user "$user" "$uid" mkdir -p "$dir" >>"$BTK_LOG" 2>&1 &&
        printf '%s\n' "$content" | btk_as_user "$user" "$uid" tee "$file" >/dev/null 2>>"$BTK_LOG"; then
        btk_as_user "$user" "$uid" systemctl --user restart wireplumber.service >>"$BTK_LOG" 2>&1 || true
        btk_done "WirePlumber handles Bluetooth without a screen ($file)"
        return 0
    fi
    btk_fail "Writing $file" "run ./update.sh --bluetooth as $user"
    return 1
}

btk_dropin_path() { printf '%s/etc/systemd/system/%s.service.d/aurago-bluetooth.conf\n' "${BTK_ROOT:-}" "$1"; }

# btk_dropin_content <uid|""> <with_group true|false>
btk_dropin_content() {
    if [ -n "$1" ]; then printf '[Unit]\nWants=user@%s.service\nAfter=user@%s.service\n\n' "$1" "$1"; fi
    printf '[Service]\n'
    if [ -n "$1" ]; then printf 'Environment=XDG_RUNTIME_DIR=/run/user/%s\n' "$1"; fi
    if [ "$2" = true ]; then printf 'SupplementaryGroups=bluetooth\n'; fi
    return 0
}

btk_verify_unit() {
    btk_has systemd-analyze || return 0
    systemd-analyze verify "$1.service" >>"$BTK_LOG" 2>&1
}

btk_remove_dropin() {
    local path
    path="$(btk_dropin_path "$1")"
    [ -e "$path" ] || return 0
    $SUDO rm -f "$path" && $SUDO systemctl daemon-reload >>"$BTK_LOG" 2>&1
}

# btk_write_dropin <service> <user> <uid>: the AuraGo service joins the
# user's audio session. A drop-in systemd rejects is removed again.
btk_write_dropin() {
    local service="$1" user="$2" uid="$3" path tmp content group=false
    path="$(btk_dropin_path "$service")"
    if btk_has getent && getent group bluetooth >/dev/null 2>&1; then group=true; fi
    if [ "$user" = root ]; then uid=""; fi
    if [ -z "$uid" ] && [ "$group" = false ]; then
        btk_remove_dropin "$service"
        return 0
    fi
    content="$(btk_dropin_content "$uid" "$group")"
    if [ "$(cat "$path" 2>/dev/null)" = "$content" ]; then
        btk_done "AuraGo service is set up for Bluetooth"
        return 0
    fi
    tmp="$(mktemp)" || { btk_fail "Writing $path" "see documentation/bluetooth.md, Installer setup"; return 1; }
    printf '%s\n' "$content" > "$tmp"
    if ! $SUDO mkdir -p "${path%/*}" || ! $SUDO install -o root -g root -m 0644 "$tmp" "$path"; then
        rm -f "$tmp"
        btk_fail "Writing $path" "see documentation/bluetooth.md, Installer setup"
        return 1
    fi
    rm -f "$tmp"
    if $SUDO systemctl daemon-reload >>"$BTK_LOG" 2>&1 && btk_verify_unit "$service"; then
        btk_done "AuraGo service joins the audio session ($path)"
        return 0
    fi
    $SUDO rm -f "$path"
    $SUDO systemctl daemon-reload >>"$BTK_LOG" 2>&1 || true
    btk_fail "Checking $path with systemd (the file was removed again)" "sudo systemd-analyze verify $service.service"
    return 1
}

# btk_config_set <config.yaml> <key> <value>: sets bluetooth.<key>, adding
# the key or the section when missing. `cat >` keeps mode and owner.
btk_config_set() {
    local file="$1" tmp
    [ -f "$file" ] || return 1
    tmp="$(mktemp)" || return 1
    if ! awk -v key="$2" -v value="$3" '
        function emit_missing(  pad) {
            if (done) return
            pad = (indent == "" ? "    " : indent)
            print pad key ": " value
            done = 1
        }
        /^bluetooth:/ {
            insec = 1; seen = 1
            if ($0 ~ /^bluetooth:[[:space:]]*(#.*)?$/) print; else print "bluetooth:"
            next
        }
        insec && /^[^[:space:]#]/ { emit_missing(); insec = 0 }
        insec && /^[[:space:]]+[^[:space:]#]/ {
            if (indent == "") { match($0, /^[[:space:]]+/); indent = substr($0, 1, RLENGTH) }
            if (!done && index($0, indent key ":") == 1) { print indent key ": " value; done = 1; next }
        }
        { print }
        END {
            if (insec) emit_missing()
            if (!seen) { print "bluetooth:"; print "    " key ": " value }
        }
    ' "$file" > "$tmp"; then
        rm -f "$tmp"
        return 1
    fi
    if ! { cat "$tmp" > "$file"; } 2>/dev/null && ! $SUDO tee "$file" < "$tmp" >/dev/null; then
        rm -f "$tmp"
        return 1
    fi
    rm -f "$tmp"
    return 0
}

# btk_check_endpoints <user> <uid> <stack>: read-only. Headphones and speakers
# connect only after PipeWire/PulseAudio registered A2DP (0000110b) with BlueZ.
btk_check_endpoints() {
    local adapter uuids i hint
    if ! adapter="$(btk_first_adapter)"; then
        btk_info "No Bluetooth adapter detected. Once one is plugged in, AuraGo shows the Bluetooth app."
        return 0
    fi
    btk_has busctl || return 0
    if [ "$(busctl get-property org.bluez "/org/bluez/$adapter" org.bluez.Adapter1 Powered 2>/dev/null)" = "b false" ]; then
        btk_info "The Bluetooth adapter is off; turn it on in AuraGo's Bluetooth app."
        return 0
    fi
    for ((i = 0; i < ${BTK_ENDPOINT_RETRIES:-5}; i++)); do
        uuids="$(busctl get-property org.bluez "/org/bluez/$adapter" org.bluez.Adapter1 UUIDs 2>/dev/null)" || uuids=""
        case "$uuids" in
            *0000110b-* | *0000110B-*)
                btk_done "Audio devices can connect"
                return 0
                ;;
        esac
        sleep "${BTK_ENDPOINT_DELAY:-2}"
    done
    if [ "$3" = pulseaudio ]; then
        hint="sudo -u $1 env XDG_RUNTIME_DIR=/run/user/$2 pactl load-module module-bluetooth-discover"
    else
        hint="sudo -u $1 env XDG_RUNTIME_DIR=/run/user/$2 systemctl --user restart wireplumber.service"
    fi
    btk_fail "Registering the audio profiles with BlueZ" "$hint"
    return 1
}

btk_summary() {
    local entry kind=ok title="BLUETOOTH READY"
    local -a lines=()
    for entry in "${BTK_DONE[@]}"; do lines+=("${T_OK:-+} $entry"); done
    for entry in "${BTK_FAILED[@]}"; do lines+=("${T_WARN:-!} ${entry%%|*}" "    run: ${entry#*|}"); done
    if [ "${#BTK_FAILED[@]}" -gt 0 ]; then
        kind=warn
        title="BLUETOOTH NEEDS ATTENTION"
        lines+=("Details: $BTK_LOG")
    fi
    if declare -F tui_box >/dev/null; then
        tui_box "$kind" "$title" "${lines[@]}"
    else
        printf '\n== %s ==\n' "$title"
        printf '  %s\n' "${lines[@]}"
    fi
    return 0
}

_btk_apply() {
    local dir="$1" user="$2" service="${3:-}" previous uid stack
    BTK_DONE=()
    BTK_FAILED=()
    { : >>"$BTK_LOG"; } 2>/dev/null || BTK_LOG=/dev/null
    btk_info "Preparing this server for Bluetooth (details: $BTK_LOG)"
    previous="$(btk_read_state "$dir")"
    # Stored first: a failed step is retried by the next update, not asked again.
    btk_write_state "$dir" enabled "$user" || btk_warn "Could not save the Bluetooth choice to $(btk_state_file "$dir")."
    uid="$(btk_user_uid "$user")"
    if [ -z "$uid" ]; then
        btk_fail "Finding the service user $user" "id $user"
        btk_summary
        return 0
    fi
    stack="$(btk_audio_stack)"
    btk_install_packages "$stack"
    btk_enable_bluez
    if [ "$user" = root ]; then
        btk_warn "AuraGo runs as root. Headphones and speakers need a regular user account; device management still works."
    elif btk_enable_linger "$user" "$uid" && btk_enable_user_units "$user" "$uid" "$stack" && [ "$stack" = pipewire ]; then
        btk_wireplumber_headless "$user" "$uid"
    fi
    if [ -n "$service" ] && [ -e "${BTK_ROOT:-}/etc/systemd/system/$service.service" ]; then
        btk_write_dropin "$service" "$user" "$uid"
    fi
    # config.yaml changes only with a new decision; afterwards the Config page owns it.
    if [ "$previous" != enabled ]; then
        if btk_config_set "$dir/config.yaml" enabled true && btk_config_set "$dir/config.yaml" allow_playback true; then
            btk_done "config.yaml: Bluetooth and playback turned on"
        else
            btk_fail "Turning on Bluetooth in $dir/config.yaml" "set bluetooth.enabled: true and bluetooth.allow_playback: true"
        fi
    fi
    if [ "$user" != root ]; then btk_check_endpoints "$user" "$uid" "$stack"; fi
    btk_summary
    return 0
}

# btk_apply <installdir> <service-user> <service-name|"">: never fails the caller.
btk_apply() { ( trap - ERR; set +e +u +o pipefail; _btk_apply "$@" ); return 0; }

_btk_decline() {
    local dir="$1" service="${2:-}" user="${3:-}"
    # A stored "no" is final until the answer changes.
    [ "$(btk_read_state "$dir")" != declined ] || return 0
    btk_write_state "$dir" declined "$user" || btk_warn "Could not save the Bluetooth choice to $(btk_state_file "$dir")."
    if [ -n "$service" ]; then
        btk_remove_dropin "$service" || btk_warn "Could not remove $(btk_dropin_path "$service")."
    fi
    if [ -f "$dir/config.yaml" ] && ! btk_config_set "$dir/config.yaml" enabled false; then
        btk_warn "Could not set bluetooth.enabled: false in $dir/config.yaml."
    fi
    btk_info "Bluetooth stays off in AuraGo. Turn it on later with: ./update.sh --bluetooth"
    return 0
}

# btk_decline <installdir> <service-name|""> [service-user]: never fails the caller.
btk_decline() { ( trap - ERR; set +e +u +o pipefail; _btk_decline "$@" ); return 0; }

# btk_run_choice <enabled|declined|skip> <installdir> <service-user> <service-name|"">
btk_run_choice() {
    case "$1" in
        enabled) btk_apply "$2" "$3" "${4:-}" ;;
        declined) btk_decline "$2" "${4:-}" "$3" ;;
    esac
    return 0
}
# <<< AURAGO-BLUETOOTH-KIT v1 <<<

INTERACTIVE_TTY=false
if [ -r /dev/tty ] && [ -w /dev/tty ] && { : </dev/tty; } 2>/dev/null; then
    INTERACTIVE_TTY=true
fi

prompt_value() {
    local variable="$1"
    local prompt="$2"
    local interactive_default="$3"
    local noninteractive_default="$4"
    local reply=""
    if $INTERACTIVE_TTY; then
        read -r -p "  ${CYAN}?${NC}  ${prompt}" reply < /dev/tty || reply="$interactive_default"
        [ -n "$reply" ] || reply="$interactive_default"
    else
        reply="$noninteractive_default"
    fi
    printf -v "$variable" '%s' "$reply"
}

cleanup_install_failure() {
    local exit_code=$?
    tui_cleanup
    [ -n "${RELEASE_CHECKSUMS_FILE:-}" ] && rm -f "$RELEASE_CHECKSUMS_FILE"
    [ -n "${RELEASE_SIGNATURE_FILE:-}" ] && rm -f "$RELEASE_SIGNATURE_FILE"
    [ -n "${RELEASE_CERTIFICATE_FILE:-}" ] && rm -f "$RELEASE_CERTIFICATE_FILE"
    [ -n "${INITIAL_PASSWORD_FILE:-}" ] && rm -f "$INITIAL_PASSWORD_FILE"
    [ -n "${EXISTING_CONFIG_BAK:-}" ] && rm -f "$EXISTING_CONFIG_BAK"
    [ -n "${RELEASE_RESOURCES_FILE:-}" ] && rm -f "$RELEASE_RESOURCES_FILE"
    [ -n "${RELEASE_ASSET_TEMP:-}" ] && rm -f "$RELEASE_ASSET_TEMP"
    [ -n "${TMP_GO:-}" ] && rm -rf "$TMP_GO"
    [ -n "${TMPEXT:-}" ] && rm -rf "$TMPEXT"
    [ -n "${PYTHON_PROBE_DIR:-}" ] && rm -rf "$PYTHON_PROBE_DIR"
    if [ "$exit_code" -eq 0 ] || [ "$INSTALL_SUCCESS" -eq 1 ]; then
        return 0
    fi

    warn "Installation aborted — cleaning up partially created service artifacts."
    if [ "$SERVICE_ENABLED" -eq 1 ] && command -v systemctl >/dev/null 2>&1; then
        ${SUDO:-} systemctl disable --now "$SYSTEMD_SERVICE" >/dev/null 2>&1 || true
    fi
    if [ "$SERVICE_FILE_CREATED" -eq 1 ] && [ -f "/etc/systemd/system/${SYSTEMD_SERVICE}.service" ]; then
        ${SUDO:-} rm -f "/etc/systemd/system/${SYSTEMD_SERVICE}.service" >/dev/null 2>&1 || true
        command -v systemctl >/dev/null 2>&1 && ${SUDO:-} systemctl daemon-reload >/dev/null 2>&1 || true
    fi
    if [ "$CREDENTIAL_FILE_CREATED" -eq 1 ] && [ -n "${CREATED_CREDENTIAL_FILE:-}" ] &&
        ${SUDO:-} test -f "$CREATED_CREDENTIAL_FILE"; then
        if [ -n "${MIGRATED_ENV_FILE:-}" ] && is_valid_master_key "${MIGRATED_MASTER_KEY:-}" &&
            write_master_key_file "$MIGRATED_ENV_FILE" "$MIGRATED_MASTER_KEY"; then
            ${SUDO:-} rm -f "$CREATED_CREDENTIAL_FILE" >/dev/null 2>&1 || true
            warn "Restored the master key to $MIGRATED_ENV_FILE after service installation failed."
        else
            warn "Could not restore the master key to the install directory; preserving $CREATED_CREDENTIAL_FILE."
        fi
    fi
}

trap cleanup_install_failure EXIT

is_valid_master_key() {
    printf '%s' "${1:-}" | grep -Eq '^[0-9a-fA-F]{64}$'
}

read_env_value() {
    local env_file="$1"
    local env_key="$2"
    [ -f "$env_file" ] || return 1
    awk -F= -v key="$env_key" '
        $1 == key {
            sub(/^[^=]*=/, "", $0)
            gsub(/^[[:space:]]+|[[:space:]]+$/, "", $0)
            gsub(/^["'"'"']|["'"'"']$/, "", $0)
            print $0
            exit
        }
    ' "$env_file"
}

write_master_key_file() {
    local target="$1"
    local key="$2"
    local tmp
    is_valid_master_key "$key" || return 1
    tmp="${target}.tmp.$$"
    (umask 077 && printf 'AURAGO_MASTER_KEY=%s\n' "$key" > "$tmp") || return 1
    mv -f "$tmp" "$target"
}

write_secret_text_file() {
    local target="$1"
    local value="$2"
    local tmp
    tmp="${target}.tmp.$$"
    (umask 077 && printf '%s\n' "$value" > "$tmp") || return 1
    mv -f "$tmp" "$target"
}

generate_master_key() {
    if command -v openssl >/dev/null 2>&1; then
        openssl rand -hex 32 2>/dev/null && return 0
    fi
    if command -v python3 >/dev/null 2>&1; then
        python3 -c "import secrets; print(secrets.token_hex(32))" 2>/dev/null && return 0
    fi
    return 1
}

version_ge() {
    local lhs="${1#v}" rhs="${2#v}" i
    local IFS=.
    local lhs_parts=() rhs_parts=()
    read -r -a lhs_parts <<< "$lhs"
    read -r -a rhs_parts <<< "$rhs"
    local max_len="${#lhs_parts[@]}"
    if [ "${#rhs_parts[@]}" -gt "$max_len" ]; then
        max_len="${#rhs_parts[@]}"
    fi
    for ((i=0; i<max_len; i++)); do
        local l="${lhs_parts[i]:-0}"
        local r="${rhs_parts[i]:-0}"
        l="${l%%[^0-9]*}"
        r="${r%%[^0-9]*}"
        l="${l:-0}"
        r="${r:-0}"
        if ((10#$l > 10#$r)); then
            return 0
        fi
        if ((10#$l < 10#$r)); then
            return 1
        fi
    done
    return 0
}

stat_owner() {
    local path="$1"
    if stat -c '%U' "$path" >/dev/null 2>&1; then
        stat -c '%U' "$path"
    elif stat -f '%Su' "$path" >/dev/null 2>&1; then
        stat -f '%Su' "$path"
    else
        return 1
    fi
}

system_group_exists() {
    local group_name="$1"
    if command -v getent >/dev/null 2>&1; then
        getent group "$group_name" >/dev/null 2>&1
        return
    fi
    grep -q "^${group_name}:" /etc/group 2>/dev/null
}

system_group_id() {
    local group_name="$1"
    local group_record=""
    local group_id=""
    if command -v getent >/dev/null 2>&1; then
        group_record="$(getent group "$group_name" 2>/dev/null | head -n 1 || true)"
    else
        group_record="$(grep -m 1 "^${group_name}:" /etc/group 2>/dev/null || true)"
    fi
    group_id="$(printf '%s\n' "$group_record" | awk -F: '{print $3}')"
    case "$group_id" in
        ""|*[!0-9]*) return 1 ;;
    esac
    [ "$group_id" -gt 0 ] || return 1
    printf '%s' "$group_id"
}

system_gpu_group_ids() {
    local ids=()
    local group_name
    local group_id
    local existing
    local duplicate
    for group_name in render video; do
        group_id="$(system_group_id "$group_name" || true)"
        [ -n "$group_id" ] || continue
        duplicate=false
        for existing in "${ids[@]}"; do
            if [ "$existing" = "$group_id" ]; then
                duplicate=true
                break
            fi
        done
        $duplicate || ids+=("$group_id")
    done
    local IFS=,
    printf '%s' "${ids[*]}"
}

systemd_gpu_groups_line() {
    local groups=()
    local group_name
    for group_name in render video; do
        if system_group_exists "$group_name"; then
            groups+=("$group_name")
        fi
    done
    if [ "${#groups[@]}" -gt 0 ]; then
        local joined
        joined="${groups[*]}"
        printf 'SupplementaryGroups=%s' "$joined"
    fi
}

systemd_serial_groups_line() {
    local groups=() group_name
    for group_name in dialout uucp; do
        if system_group_exists "$group_name"; then
            groups+=("$group_name")
        fi
    done
    if [ "${#groups[@]}" -gt 0 ]; then
        printf 'SupplementaryGroups=%s' "${groups[*]}"
    fi
}

systemd_escape_path_value() {
    local value="$1"
    local escaped=""
    local char
    local index
    for ((index = 0; index < ${#value}; index++)); do
        char="${value:index:1}"
        case "$char" in
            " ") escaped+="\\x20" ;;
            $'\t') escaped+="\\x09" ;;
            '"') escaped+="\\x22" ;;
            "'") escaped+="\\x27" ;;
            "\\") escaped+="\\x5c" ;;
            "%") escaped+="%%" ;;
            $'\r'|$'\n') return 1 ;;
            *) escaped+="$char" ;;
        esac
    done
    printf '%s' "$escaped"
}

primary_lan_ipv4() {
    local candidates=""
    local candidate
    if command -v ip >/dev/null 2>&1; then
        candidates="$(ip -o -4 route get 1.1.1.1 2>/dev/null | awk '{ for (i = 1; i <= NF; i++) if ($i == "src") { print $(i + 1); exit } }' || true)"
    fi
    if [ -z "$candidates" ] && command -v hostname >/dev/null 2>&1; then
        candidates="$(hostname -I 2>/dev/null || true)"
    fi
    for candidate in $candidates; do
        case "$candidate" in
            0.0.0.0|127.*) continue ;;
        esac
        if printf '%s\n' "$candidate" | grep -Eq '^[0-9]{1,3}(\.[0-9]{1,3}){3}$'; then
            printf '%s' "$candidate"
            return 0
        fi
    done
    return 1
}

configured_server_port() {
    local config_file="$1"
    local key="$2"
    local fallback="$3"
    local value=""
    if [ -f "$config_file" ]; then
        value="$(awk -v key="$key" '
            /^server:[[:space:]]*($|#)/ { in_server=1; next }
            in_server && /^[^[:space:]]/ { exit }
            in_server {
                line=$0
                sub(/[[:space:]]*#.*/, "", line)
                if (line ~ "^[[:space:]]+" key ":[[:space:]]*") {
                    sub("^[[:space:]]+" key ":[[:space:]]*", "", line)
                    gsub(/[[:space:]]/, "", line)
                    print line
                    exit
                }
            }
        ' "$config_file")"
    fi
    case "$value" in
        ""|*[!0-9]*) printf '%s' "$fallback" ;;
        *)
            if [ "$value" -ge 1 ] && [ "$value" -le 65535 ]; then
                printf '%s' "$value"
            else
                printf '%s' "$fallback"
            fi
            ;;
    esac
}

latest_release_tag_via_redirect() {
    local latest_url="https://github.com/${GITHUB_REPO}/releases/latest"
    local effective_url=""
    if command -v curl >/dev/null 2>&1; then
        effective_url="$(curl -fsSLI -o /dev/null -w '%{url_effective}' "$latest_url" 2>/dev/null || true)"
    elif command -v wget >/dev/null 2>&1; then
        effective_url="$(wget -S --max-redirect=10 --spider "$latest_url" 2>&1 | awk '/^  Location: / {print $2}' | tail -n1 | tr -d '\r' || true)"
    fi
    [ -n "$effective_url" ] || return 1
    basename "$effective_url"
}

# Downloads the RPM Fusion Free signing key, checks it against a pinned
# fingerprint and imports it into the RPM keyring. Returns 1 after one warning
# that names the cause; never leaves the temporary key file or gpg home behind.
#
# RPM Fusion signs its Free release packages for Fedora 33-46 with one
# year-named key, not a per-release one (source: https://rpmfusion.org/keys,
# "RPM-GPG-KEY-rpmfusion-free-fedora-2020", uid "RPM Fusion free repository for
# Fedora (2020)"). When RPM Fusion rotates keys, change the two values below.
import_rpmfusion_free_key() {
    local key_url="https://rpmfusion.org/keys?action=AttachFile&do=get&target=RPM-GPG-KEY-rpmfusion-free-fedora-2020"
    local key_fpr="E9A491A3DE247814E7E067EAE06F8ECDD651FF2E"
    local key_id keyfile gpg_home gpg_out pub_count primary_fpr
    # rpm names an imported key gpg-pubkey-<last 8 fingerprint digits, lowercase>.
    key_id="$(printf '%s' "${key_fpr: -8}" | tr 'A-F' 'a-f')"

    keyfile="$(mktemp "${TMPDIR:-/tmp}/aurago-rpmfusion-key.XXXXXX")" || {
        warn "Could not create a temporary file for the RPM Fusion signing key; skipping RPM Fusion"
        return 1
    }
    if ! _download_optional "$key_url" "$keyfile"; then
        rm -f "$keyfile"
        warn "Could not download the RPM Fusion signing key; skipping RPM Fusion"
        return 1
    fi

    if command -v gpg >/dev/null 2>&1; then
        # Verify before importing. A throwaway GNUPGHOME keeps gpg from creating
        # ~/.gnupg (and mktemp -d makes it mode 0700, as gpg wants).
        gpg_home="$(mktemp -d "${TMPDIR:-/tmp}/aurago-rpmfusion-gnupg.XXXXXX")" || {
            rm -f "$keyfile"
            warn "Could not create a temporary directory to check the RPM Fusion signing key; skipping RPM Fusion"
            return 1
        }
        # --show-keys needs gnupg 2.1+; older gpg lists a key file when it is
        # simply given as the argument.
        gpg_out="$(GNUPGHOME="$gpg_home" gpg --show-keys --with-colons --with-fingerprint "$keyfile" 2>/dev/null ||
            GNUPGHOME="$gpg_home" gpg --with-colons --with-fingerprint "$keyfile" 2>/dev/null || true)"
        rm -rf "$gpg_home"
        # Pin the PRIMARY key: gpg prints an fpr record after every pub and every
        # sub line, so matching the fingerprint anywhere would also accept an
        # attacker's key that merely carries the RPM Fusion key as a subkey. The
        # file must also hold exactly one pub record, or rpm would import both.
        primary_fpr="$(printf '%s\n' "$gpg_out" | awk -F: '$1=="pub"{want=1; next} want && $1=="fpr"{print $10; exit}')"
        pub_count="$(printf '%s\n' "$gpg_out" | grep -c '^pub:' || true)"
        if [ "$primary_fpr" != "$key_fpr" ] || [ "${pub_count:-0}" -ne 1 ]; then
            rm -f "$keyfile"
            warn "The downloaded RPM Fusion signing key is not the pinned key (fingerprint ${key_fpr}); skipping RPM Fusion"
            return 1
        fi
        if ! $SUDO rpm --import "$keyfile" 2>/dev/null; then
            rm -f "$keyfile"
            warn "Could not import the RPM Fusion signing key; skipping RPM Fusion"
            return 1
        fi
    else
        # Without gpg the fingerprint cannot be read before the import, so import
        # first and then require the pinned 8-digit key id in the RPM keyring.
        # That is a weaker check than the fingerprint (short key ids can be
        # forged), accepted here for a home-lab installer without gnupg.
        if ! $SUDO rpm --import "$keyfile" 2>/dev/null; then
            rm -f "$keyfile"
            warn "Could not import the RPM Fusion signing key; skipping RPM Fusion"
            return 1
        fi
        if ! rpm -q "gpg-pubkey-${key_id}" >/dev/null 2>&1; then
            rm -f "$keyfile"
            warn "RPM Fusion signing key ${key_id} is not in the RPM keyring after the import; skipping RPM Fusion"
            return 1
        fi
    fi
    rm -f "$keyfile"
    return 0
}

install_ffmpeg() {
    case "$PKG_MGR" in
        apt)
            _pkg_install ffmpeg
            ;;
        dnf)
            if $SUDO dnf install -y ffmpeg --allowerasing 2>/dev/null; then
                return 0
            fi
            if [ -f /etc/fedora-release ] && command -v rpm >/dev/null 2>&1; then
                local fedora_release
                fedora_release="$(rpm -E %fedora 2>/dev/null || true)"
                if printf '%s' "$fedora_release" | grep -Eq '^[0-9]+$'; then
                    # Import the pinned RPM Fusion signing key first, then let dnf check
                    # the downloaded release RPM against it instead of trusting the bare
                    # URL. A failure returns 1; ensure_ffmpeg turns that into the "install
                    # manually" warning, so the warnings here only name the cause.
                    import_rpmfusion_free_key || return 1
                    if ! $SUDO dnf install -y --setopt=localpkg_gpgcheck=1 "https://download1.rpmfusion.org/free/fedora/rpmfusion-free-release-${fedora_release}.noarch.rpm" 2>/dev/null; then
                        warn "RPM Fusion release package failed its signature check or install; skipping RPM Fusion"
                        return 1
                    fi
                    $SUDO dnf install -y ffmpeg
                    return $?
                fi
            fi
            return 1
            ;;
        *)
            _pkg_install ffmpeg
            ;;
    esac
}

install_docker_engine() {
    case "$PKG_MGR" in
        apt)
            $SUDO apt-get update
            $SUDO apt-get install -y docker.io
            ;;
        pacman)
            _pkg_install docker
            ;;
        apk)
            _pkg_install docker docker-cli containerd
            ;;
        zypper)
            _pkg_install docker
            ;;
        dnf|yum)
            warn "Automatic Docker installation for ${PKG_MGR} is disabled here to avoid piping a remote script into sh."
            warn "Install Docker manually via your distro's documented repository setup, then rerun this installer."
            return 1
            ;;
        *)
            warn "Cannot install Docker automatically on this system."
            return 1
            ;;
    esac

    if command -v systemctl >/dev/null 2>&1; then
        $SUDO systemctl enable --now docker >/dev/null 2>&1 || warn "Could not enable/start docker.service automatically."
    fi
}

update_source_checkout() {
    local branch upstream
    if ! git -C "$INSTALL_DIR" diff --quiet --ignore-submodules -- || ! git -C "$INSTALL_DIR" diff --cached --quiet --ignore-submodules --; then
        warn "Local tracked changes detected in $INSTALL_DIR — skipping automatic git fast-forward."
        warn "Commit/stash your changes and rerun the installer if you want the source checkout updated."
        return 0
    fi
    git -C "$INSTALL_DIR" fetch --tags --prune origin || return 1
    branch="$(git -C "$INSTALL_DIR" rev-parse --abbrev-ref HEAD 2>/dev/null || echo main)"
    upstream="origin/${branch}"
    if ! git -C "$INSTALL_DIR" rev-parse --verify "$upstream" >/dev/null 2>&1; then
        warn "Upstream branch ${upstream} not found — keeping current checkout unchanged."
        return 0
    fi
    if git -C "$INSTALL_DIR" merge --ff-only "$upstream"; then
        ok "Updated source checkout to latest ${upstream}."
    else
        warn "Fast-forward merge failed for ${upstream} — keeping current checkout unchanged."
    fi
}

warn_if_systemd_hardening_conflicts() {
    local config_path="$1"
    [ -f "$config_path" ] || return 0
    if grep -Eq '^[[:space:]]+sudo_enabled:[[:space:]]*true([[:space:]]|$)' "$config_path"; then
        warn "config.yaml enables sudo features. Keep this only for trusted local installations."
    fi
    if grep -Eq '^[[:space:]]+sudo_unrestricted:[[:space:]]*true([[:space:]]|$)' "$config_path"; then
        warn "config.yaml enables sudo_unrestricted. The generated systemd unit will not use ProtectSystem=strict."
    fi
}

TUI_STEP_TOTAL=7
tui_banner "Quick Installer" "AI Agent Framework for Linux"

# ── Architecture detection ──────────────────────────────────────────────
section "System scan"
ARCH_RAW=$(uname -m)
case "$ARCH_RAW" in
    x86_64)        GOARCH="amd64" ;;
    aarch64|arm64) GOARCH="arm64" ;;
    armv7l|armv6l) die "ARMv6/v7 is not supported by the current installer artifacts yet. Please use a supported release or build manually." ;;
    *)             die "Unsupported architecture: $ARCH_RAW" ;;
esac
ok "Architecture: $ARCH_RAW → target: $GOARCH"

SUDO=""
[ "$(id -u)" -ne 0 ] && SUDO="sudo"

# Host summary: purely informational, every probe is optional.
show_system_scan() {
    local os_name="" mem_kb="" cores="" disk="" init="sysv/other"
    if [ -r /etc/os-release ]; then
        os_name="$(. /etc/os-release 2>/dev/null && printf '%s' "${PRETTY_NAME:-${NAME:-}}" || true)"
    fi
    [ -n "$os_name" ] || os_name="$(uname -s)"
    mem_kb="$(awk '/^MemTotal:/ {print $2; exit}' /proc/meminfo 2>/dev/null || true)"
    cores="$(getconf _NPROCESSORS_ONLN 2>/dev/null || true)"
    disk="$(df -Ph "$(dirname "$INSTALL_DIR")" 2>/dev/null | awk 'NR==2 {print $4" free"}' || true)"
    if [ -d /run/systemd/system ]; then init="systemd"; fi
    info "Host:     ${os_name} · kernel $(uname -r)"
    info "Hardware: ${cores:-?} CPU cores · $(( ${mem_kb:-0} / 1024 )) MB RAM · ${disk:-disk space unknown}"
    info "Target:   ${INSTALL_DIR} · init: ${init} · user: $(id -un)"
    return 0
}
show_system_scan

# ── Package manager detection ────────────────────────────────────────────
_detect_pkg_manager() {
    if   command -v apt-get >/dev/null 2>&1; then echo "apt"
    elif command -v dnf     >/dev/null 2>&1; then echo "dnf"
    elif command -v yum     >/dev/null 2>&1; then echo "yum"
    elif command -v pacman  >/dev/null 2>&1; then echo "pacman"
    elif command -v apk     >/dev/null 2>&1; then echo "apk"
    elif command -v zypper  >/dev/null 2>&1; then echo "zypper"
    else echo "unknown"
    fi
}
PKG_MGR=$(_detect_pkg_manager)

_pkg_install() {
    local -a install_cmd
    case "$PKG_MGR" in
        apt)    install_cmd=($SUDO apt-get install -y "$@") ;;
        dnf)    install_cmd=($SUDO dnf install -y "$@") ;;
        yum)    install_cmd=($SUDO yum install -y "$@") ;;
        pacman) install_cmd=($SUDO pacman -Sy --noconfirm "$@") ;;
        apk)    install_cmd=($SUDO apk add --no-cache "$@") ;;
        zypper) install_cmd=($SUDO zypper install -y "$@") ;;
        *)
            warn "Cannot auto-install packages (unknown package manager). Please install manually: $*"
            return 1
            ;;
    esac
    TUI_RUN_SUDO=1 tui_run "Installing $*" "${install_cmd[@]}"
}

# ── Ensure curl or wget ──────────────────────────────────────────────────
if ! command -v curl >/dev/null 2>&1 && ! command -v wget >/dev/null 2>&1; then
    _pkg_install curl ca-certificates || die "Install curl and ca-certificates, then rerun the installer."
fi
command -v curl >/dev/null 2>&1 || command -v wget >/dev/null 2>&1 || \
    die "Neither curl nor wget is available after dependency installation."

if ! command -v tar >/dev/null 2>&1; then
    _pkg_install tar || die "Install tar, then rerun the installer."
fi
command -v tar >/dev/null 2>&1 || die "tar is required to extract AuraGo release resources."
command -v mktemp >/dev/null 2>&1 || die "mktemp is required. Install coreutils (or your platform equivalent) and rerun the installer."

_download() {
    TUI_RUN_WATCH="$2" tui_run "Downloading ${1##*/}" _download_fetch "$1" "$2"
}

_download_fetch() {
    local url="$1" dest="$2"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$url" -o "$dest"
    elif command -v wget >/dev/null 2>&1; then
        wget -q "$url" -O "$dest"
    else
        die "Neither curl nor wget available."
    fi
}

_download_optional() {
    local url="$1" dest="$2"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$url" -o "$dest" 2>/dev/null
    elif command -v wget >/dev/null 2>&1; then
        wget -q "$url" -O "$dest" 2>/dev/null
    else
        return 1
    fi
}

fetch_url_stdout() {
    local url="$1"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$url"
    elif command -v wget >/dev/null 2>&1; then
        wget -qO- "$url"
    else
        die "Neither curl nor wget available."
    fi
}

sha256_file() {
    local path="$1"
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$path" | awk '{print $1}'
    elif command -v shasum >/dev/null 2>&1; then
        shasum -a 256 "$path" | awk '{print $1}'
    elif command -v openssl >/dev/null 2>&1; then
        openssl dgst -sha256 "$path" | awk '{print $NF}'
    else
        return 1
    fi
}

strict_release_verify_enabled() {
    case "${AURAGO_STRICT_RELEASE_VERIFY:-}" in
        1|true|TRUE|yes|YES) return 0 ;;
        *) return 1 ;;
    esac
}

verify_release_checksums_signature() {
    [ -n "${RELEASE_BASE:-}" ] || die "RELEASE_BASE is not set."
    [ -n "${RELEASE_CHECKSUMS_FILE:-}" ] && [ -f "$RELEASE_CHECKSUMS_FILE" ] || die "Release checksums are not available."

    local sig_file cert_file
    sig_file="$(mktemp "${TMPDIR:-/tmp}/aurago-sha256-sig.XXXXXX")"
    cert_file="$(mktemp "${TMPDIR:-/tmp}/aurago-sha256-cert.XXXXXX")"
    RELEASE_SIGNATURE_FILE="$sig_file"
    RELEASE_CERTIFICATE_FILE="$cert_file"

    if ! _download_optional "${RELEASE_BASE}/SHA256SUMS.sig" "$sig_file" || ! _download_optional "${RELEASE_BASE}/SHA256SUMS.pem" "$cert_file"; then
        rm -f "$sig_file" "$cert_file"
        RELEASE_SIGNATURE_FILE=""
        RELEASE_CERTIFICATE_FILE=""
        if strict_release_verify_enabled; then
            die "Release signature files are missing and AURAGO_STRICT_RELEASE_VERIFY=1 is set."
        fi
        warn "Release signature files not found; continuing with SHA256 manifest verification only."
        return 0
    fi

    if ! command -v cosign >/dev/null 2>&1; then
        rm -f "$sig_file" "$cert_file"
        RELEASE_SIGNATURE_FILE=""
        RELEASE_CERTIFICATE_FILE=""
        if strict_release_verify_enabled; then
            die "cosign is required for strict release signature verification."
        fi
        warn "cosign not found; continuing with SHA256 manifest verification only."
        return 0
    fi

    if cosign verify-blob \
        --certificate "$cert_file" \
        --signature "$sig_file" \
        --certificate-identity-regexp "https://github.com/${GITHUB_REPO}/.*" \
        --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
        "$RELEASE_CHECKSUMS_FILE" >/dev/null 2>&1; then
        ok "Release checksum signature verified."
    else
        rm -f "$sig_file" "$cert_file"
        RELEASE_SIGNATURE_FILE=""
        RELEASE_CERTIFICATE_FILE=""
        if strict_release_verify_enabled; then
            die "Release checksum signature verification failed."
        fi
        warn "Release checksum signature verification failed; continuing with SHA256 manifest verification only."
        return 0
    fi

    rm -f "$sig_file" "$cert_file"
    RELEASE_SIGNATURE_FILE=""
    RELEASE_CERTIFICATE_FILE=""
}

fetch_release_checksums() {
    [ -n "${RELEASE_BASE:-}" ] || die "RELEASE_BASE is not set."
    if [ -n "${RELEASE_CHECKSUMS_FILE:-}" ] && [ -f "${RELEASE_CHECKSUMS_FILE:-}" ]; then
        return 0
    fi
    RELEASE_CHECKSUMS_FILE="$(mktemp "${TMPDIR:-/tmp}/aurago-sha256.XXXXXX")"
    if ! _download "${RELEASE_BASE}/SHA256SUMS" "$RELEASE_CHECKSUMS_FILE"; then
        rm -f "$RELEASE_CHECKSUMS_FILE"
        RELEASE_CHECKSUMS_FILE=""
        return 1
    fi
    verify_release_checksums_signature
}

verify_release_asset() {
    local asset="$1"
    local path="$2"
    local expected actual
    [ -f "$path" ] || { warn "Cannot verify missing file: $path"; return 1; }
    [ -n "${RELEASE_CHECKSUMS_FILE:-}" ] && [ -f "$RELEASE_CHECKSUMS_FILE" ] || { warn "Release checksums are not available."; return 1; }
    expected="$(awk -v target="$asset" '{ sub(/\r$/, "", $2); if ($2 == target) { print $1; exit } }' "$RELEASE_CHECKSUMS_FILE")"
    [ -n "$expected" ] || { warn "Missing checksum entry for ${asset} in release manifest."; return 1; }
    actual="$(sha256_file "$path" || true)"
    [ -n "$actual" ] || { warn "No SHA256 tool available to verify ${asset}."; return 1; }
    [ "$actual" = "$expected" ] || { warn "Checksum verification failed for ${asset}."; return 1; }
}

download_release_asset() {
    local asset="$1"
    local dest="$2"
    RELEASE_ASSET_TEMP="${dest}.download.$$"
    rm -f "$RELEASE_ASSET_TEMP"
    if ! _download "${RELEASE_BASE}/${asset}" "$RELEASE_ASSET_TEMP"; then
        rm -f "$RELEASE_ASSET_TEMP"
        RELEASE_ASSET_TEMP=""
        warn "Could not download release asset ${asset}."
        return 1
    fi
    if ! verify_release_asset "$asset" "$RELEASE_ASSET_TEMP"; then
        rm -f "$RELEASE_ASSET_TEMP"
        RELEASE_ASSET_TEMP=""
        return 1
    fi
    if ! mv -f "$RELEASE_ASSET_TEMP" "$dest"; then
        rm -f "$RELEASE_ASSET_TEMP"
        RELEASE_ASSET_TEMP=""
        warn "Could not publish verified release asset ${asset} to ${dest}."
        return 1
    fi
    RELEASE_ASSET_TEMP=""
}

latest_release_tag() {
    local tag
    tag="$(fetch_url_stdout "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" \
        | grep -o '"tag_name": *"[^"]*"' \
        | head -1 \
        | cut -d'"' -f4)"
    if [ -n "$tag" ]; then
        printf '%s\n' "$tag"
        return 0
    fi
    latest_release_tag_via_redirect
}

ensure_ffmpeg() {
    if command -v ffmpeg >/dev/null 2>&1; then
        ok "ffmpeg found."
        return 0
    fi

    warn "ffmpeg not found."
    prompt_value FF_REPLY "Install ffmpeg? [Y/n]: " "y" "n"
    if [[ "${FF_REPLY:-y}" =~ ^[Yy]$ ]]; then
        if install_ffmpeg; then
            ok "ffmpeg installed."
        else
            warn "ffmpeg installation failed. Install it manually for your distro and rerun the installer if you need voice conversion."
        fi
    else
        warn "Skipping ffmpeg. Telegram voice messages will not work."
    fi
}

ensure_imagemagick() {
    if command -v magick >/dev/null 2>&1 || command -v convert >/dev/null 2>&1; then
        ok "ImageMagick found."
        return 0
    fi

    warn "ImageMagick not found."
    prompt_value IM_REPLY "Install ImageMagick for image conversion? [Y/n]: " "y" "n"
    if [[ "${IM_REPLY:-y}" =~ ^[Yy]$ ]]; then
        if case "$PKG_MGR" in
            dnf) _pkg_install ImageMagick ;;
            *)   _pkg_install imagemagick ;;
        esac
        then
            ok "ImageMagick installed."
        else
            warn "ImageMagick installation failed. Image format conversion will not work."
        fi
    else
        warn "Skipping ImageMagick. Image format conversion will not work."
    fi
}

python_runtime_ready() {
    local python_cmd
    PYTHON_PROBE_DIR="$(mktemp -d "${TMPDIR:-/tmp}/aurago-python-venv.XXXXXX")" || return 1
    for python_cmd in python3 python; do
        command -v "$python_cmd" >/dev/null 2>&1 || continue
        rm -rf "$PYTHON_PROBE_DIR"
        mkdir -p "$PYTHON_PROBE_DIR"
        if "$python_cmd" -m venv "$PYTHON_PROBE_DIR" >/dev/null 2>&1 \
            && "$PYTHON_PROBE_DIR/bin/python" -m pip --version >/dev/null 2>&1; then
            rm -rf "$PYTHON_PROBE_DIR"
            PYTHON_PROBE_DIR=""
            return 0
        fi
    done
    rm -rf "$PYTHON_PROBE_DIR"
    PYTHON_PROBE_DIR=""
    return 1
}

ensure_python_runtime() {
    PYTHON_MISSING=false
    if python_runtime_ready; then
        ok "Python with working venv + pip found."
        return 0
    fi

    warn "Python with working venv + pip not found."
    prompt_value PY_REPLY "Install Python 3, pip and venv? [Y/n]: " "y" "n"
    if [[ "${PY_REPLY:-y}" =~ ^[Yy]$ ]]; then
        if ! case "$PKG_MGR" in
            apt)    _pkg_install python3 python3-pip python3-venv ;;
            dnf|yum) _pkg_install python3 python3-pip ;;
            pacman) _pkg_install python python-pip ;;
            apk)    _pkg_install python3 py3-pip ;;
            zypper) _pkg_install python3 python3-pip ;;
            *)      _pkg_install python3 python3-pip ;;
        esac
        then
            warn "Python installation failed. Python-based tools and skills will not work."
            PYTHON_MISSING=true
            return 0
        fi
        if python_runtime_ready; then
            ok "Python with working venv + pip installed."
        else
            warn "Python packages were installed, but creating a venv with pip still fails."
            warn "Python-based tools and skills will remain unavailable until the venv support is repaired."
            PYTHON_MISSING=true
        fi
    else
        warn "Skipping Python. Python-based tools and skills will not work."
        PYTHON_MISSING=true
    fi
}

ensure_docker_engine() {
    if command -v docker >/dev/null 2>&1; then
        if docker info >/dev/null 2>&1; then
            ok "Docker found and daemon reachable."
            return 0
        fi
        warn "Docker CLI found but the daemon is not reachable."
        if $INTERACTIVE_TTY && command -v systemctl >/dev/null 2>&1; then
            info "Trying to enable/start docker.service..."
            if $SUDO systemctl enable --now docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
                ok "Docker daemon started."
                return 0
            fi
        fi
        warn "Docker daemon is still not reachable. AuraGo can install or repair Docker via the local package manager."
    fi

    echo ""
    tui_box info "Docker not found" \
        "Many useful AuraGo features (Sandbox, tools) need Docker."
    echo ""
    prompt_value DKR_REPLY "Install Docker now? (Recommended) [Y/n]: " "y" "n"
    if [[ "${DKR_REPLY:-y}" =~ ^[Yy]$ ]]; then
        info "Installing Docker via the local package manager..."
        if install_docker_engine; then
            if command -v docker >/dev/null 2>&1 && ! docker info >/dev/null 2>&1; then
                warn "Docker installed, but the daemon is not reachable yet. Check: sudo systemctl status docker"
            fi
            DOCKER_USER="${SUDO_USER:-${USER:-}}"
            if [ -n "$DOCKER_USER" ] && [ "$DOCKER_USER" != "root" ]; then
                if id -nG "$DOCKER_USER" 2>/dev/null | tr ' ' '\n' | grep -qx docker; then
                    ok "$DOCKER_USER is already in the docker group."
                else
                    $SUDO usermod -aG docker "$DOCKER_USER" || warn "Failed to add $DOCKER_USER to docker group. Run manually: sudo usermod -aG docker $DOCKER_USER"
                    warn "$DOCKER_USER must log out and back in before Docker group membership is active."
                fi
            else
                warn "Docker installed, but no non-root login user was detected for docker group membership."
            fi
            ok "Docker installed."
        else
            warn "Docker was not installed automatically."
        fi
    else
        warn "Skipping Docker installation."
    fi
}

# ── Optional system dependencies ─────────────────────────────────────────
section "Dependencies"
if ! $INTERACTIVE_TTY; then
    warn "No interactive TTY detected; using conservative defaults for optional components and services."
fi

ensure_ffmpeg
ensure_imagemagick
ensure_python_runtime
ensure_docker_engine
# Bluetooth: asked here, applied once the service user is known.
BT_CHOICE="$(btk_resolve_choice "$INSTALL_DIR" "" "$INTERACTIVE_TTY")"

# ══════════════════════════════════════════════════════════════════════════
#  Decide installation mode: SOURCE BUILD vs BINARY INSTALL
# ══════════════════════════════════════════════════════════════════════════
section "Installation"
# Add common Go install locations to PATH (in case Go was already installed but not in PATH)
for _godir in /usr/local/go/bin "$HOME/go/bin" /usr/local/bin; do
    [ -d "$_godir" ] && [[ ":$PATH:" != *":$_godir:"* ]] && export PATH="$_godir:$PATH"
done
unset _godir

BUILD_FROM_SOURCE=false

_go_version_ok() {
    local installed
    installed=$(go version 2>/dev/null | awk '{print $3}' | sed 's/go//')
    [ -n "$installed" ] || return 1
    version_ge "$installed" "$GO_VERSION"
}

if _go_version_ok && $INTERACTIVE_TTY; then
    export PATH="$GO_INSTALL_DIR/go/bin:$PATH"
    ok "Go $(go version | awk '{print $3}') found — will build from source."
    BUILD_FROM_SOURCE=true
elif _go_version_ok; then
    ok "Go $(go version | awk '{print $3}') found; non-interactive safe default selects the binary install."
else
    info "Go $GO_VERSION+ not found."
    echo ""
    echo -e "  ${BOLD}Choose installation mode:${NC}"
    echo -e "    ${CYAN}1)${NC} Binary install — download pre-built binaries (no Go needed, fast)"
    echo -e "    ${CYAN}2)${NC} Source build   — install Go $GO_VERSION, clone repo, build from source"
    echo ""
    prompt_value MODE_REPLY "Install mode [1/2, default=1]: " "1" "1"
    if [[ "${MODE_REPLY:-1}" == "2" ]]; then
        info "Installing Go $GO_VERSION for $GOARCH..."
        GO_TAR="go${GO_VERSION}.linux-${GOARCH}.tar.gz"
        GO_URL="https://go.dev/dl/${GO_TAR}"
        TMP_GO="$(mktemp -d "${TMPDIR:-/tmp}/aurago-go.XXXXXX")"

        _download "$GO_URL" "$TMP_GO/$GO_TAR"
        $SUDO rm -rf "$GO_INSTALL_DIR/go"
        TUI_RUN_SUDO=1 tui_run "Unpacking Go ${GO_VERSION}" $SUDO tar -C "$GO_INSTALL_DIR" -xzf "$TMP_GO/$GO_TAR"
        rm -rf "$TMP_GO"
        TMP_GO=""

        export PATH="$GO_INSTALL_DIR/go/bin:$PATH"
        $SUDO tee /etc/profile.d/go.sh > /dev/null <<'GOPATH'
export PATH="/usr/local/go/bin:$PATH"
GOPATH
        ok "Go $GO_VERSION installed to $GO_INSTALL_DIR/go"
        BUILD_FROM_SOURCE=true
    else
        ok "Binary install selected — no Go required."
    fi
fi

# ══════════════════════════════════════════════════════════════════════════
#  MODE A: Source build — clone repo & compile
# ══════════════════════════════════════════════════════════════════════════
if $BUILD_FROM_SOURCE; then
    command -v git >/dev/null 2>&1 || _pkg_install git

    if [ -d "$INSTALL_DIR/.git" ]; then
        info "Existing installation found at $INSTALL_DIR — updating..."
        update_source_checkout || die "Failed to update source checkout."
    else
        tui_run "Cloning into $INSTALL_DIR" git clone "$REPO" "$INSTALL_DIR"
    fi

    cd "$INSTALL_DIR"
    mkdir -p bin data data/embeddings agent_workspace/workdir agent_workspace/tools log

    info "Building AuraGo from source (GOOS=linux GOARCH=$GOARCH)..."
    tui_run "Packaging web resources" go run ./cmd/assetpack -out deploy -stage assets/web || die "Failed to package web resources."
    ASSET_LDFLAGS="$(cat deploy/web-assets.ldflags)"
    tui_run "Building bin/aurago_linux" env CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" \
        go build -trimpath -ldflags="-s -w $ASSET_LDFLAGS" -o bin/aurago_linux ./cmd/aurago

    tui_run "Building bin/config-merger_linux" env CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" \
        go build -trimpath -ldflags="-s -w" -o bin/config-merger_linux ./cmd/config-merger

    tui_run "Building bin/aurago-remote_linux" env CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" \
        go build -trimpath -ldflags="-s -w" -o bin/aurago-remote_linux ./cmd/remote

    # Install config.yaml from template if none exists (source build has config_template.yaml in repo)
    if [ ! -f "$INSTALL_DIR/config.yaml" ] && [ -f "$INSTALL_DIR/config_template.yaml" ]; then
        cp "$INSTALL_DIR/config_template.yaml" "$INSTALL_DIR/config.yaml"
        ok "config.yaml created from template."
    fi

# ══════════════════════════════════════════════════════════════════════════
#  MODE B: Binary install — download from GitHub Releases (no clone)
# ══════════════════════════════════════════════════════════════════════════
else
    info "Binary install — downloading from GitHub Releases..."

    # Resolve the latest release tag
    RELEASE_TAG="$(latest_release_tag || true)"
    [ -z "$RELEASE_TAG" ] && die "Could not determine latest release tag from GitHub."
    info "Latest release: $RELEASE_TAG"

    RELEASE_BASE="https://github.com/${GITHUB_REPO}/releases/download/${RELEASE_TAG}"
    fetch_release_checksums || die "Could not download SHA256SUMS for release ${RELEASE_TAG}."

    # Create install directory + subdirectories
    mkdir -p "$INSTALL_DIR/bin" "$INSTALL_DIR/data" "$INSTALL_DIR/data/embeddings" "$INSTALL_DIR/log"
    mkdir -p "$INSTALL_DIR/agent_workspace/workdir" "$INSTALL_DIR/agent_workspace/tools"
    cd "$INSTALL_DIR"

    if [ -f "$INSTALL_DIR/config.yaml" ]; then
        EXISTING_CONFIG_BAK="$(mktemp "${TMPDIR:-/tmp}/aurago-config.XXXXXX")"
        cp -p "$INSTALL_DIR/config.yaml" "$EXISTING_CONFIG_BAK"
    fi

    # Download resources.dat and extract (contains prompts, skills, config template, UI)
    download_release_asset "resources.dat" "$INSTALL_DIR/resources.dat" || \
        die "Failed to download or verify resources.dat from release ${RELEASE_TAG}."
    RELEASE_RESOURCES_FILE="$INSTALL_DIR/resources.dat"
    # Extract to a temp dir so we can selectively merge (never clobber existing config)
    TMPEXT="$(mktemp -d "${TMPDIR:-/tmp}/aurago-resources.XXXXXX")"
    tar -xzf "$RELEASE_RESOURCES_FILE" -C "$TMPEXT" || die "Failed to extract verified resources.dat."
    rm -f "$RELEASE_RESOURCES_FILE"
    RELEASE_RESOURCES_FILE=""

    # Copy prompts, skills, and other resource dirs (always overwrite — they are code)
    [ -d "$TMPEXT/prompts" ]           && cp -a "$TMPEXT/prompts"           "$INSTALL_DIR/"
    [ -d "$TMPEXT/agent_workspace" ]   && cp -a "$TMPEXT/agent_workspace"   "$INSTALL_DIR/"
    [ -d "$TMPEXT/ui" ]               && cp -a "$TMPEXT/ui"               "$INSTALL_DIR/" 2>/dev/null || true
    mkdir -p "$INSTALL_DIR/assets"
    for asset_dir in "$TMPEXT/assets/"*; do
        [ -d "$asset_dir" ] || continue
        [ "$(basename "$asset_dir")" = web ] || cp -a "$asset_dir" "$INSTALL_DIR/assets/"
    done

    # Save the freshly shipped template for merge/copy after config-merger is available.
    if [ -f "$TMPEXT/config.yaml" ]; then
        cp "$TMPEXT/config.yaml" "$INSTALL_DIR/config.yaml.new_template"
    fi
    ok "Resources extracted."

    # Download binaries
    if [ "$GOARCH" = "arm64" ]; then
        info "Downloading arm64 binaries..."
        download_release_asset "aurago_linux_arm64"                "bin/aurago_linux_arm64"                 || die "Required AuraGo arm64 binary is unavailable or invalid."
        download_release_asset "config-merger_linux_arm64"         "bin/config-merger_linux_arm64"         2>/dev/null || warn "config-merger_linux_arm64 not in release."
        download_release_asset "aurago-remote_linux_arm64"         "bin/aurago-remote_linux_arm64"         2>/dev/null || warn "aurago-remote_linux_arm64 not in release."
        cp bin/aurago_linux_arm64           bin/aurago_linux
        cp bin/config-merger_linux_arm64    bin/config-merger_linux         2>/dev/null || true
        cp bin/aurago-remote_linux_arm64    bin/aurago-remote_linux         2>/dev/null || true
    else
        info "Downloading amd64 binaries..."
        download_release_asset "aurago_linux"                      "bin/aurago_linux"                       || die "Required AuraGo amd64 binary is unavailable or invalid."
        download_release_asset "config-merger_linux"               "bin/config-merger_linux"               2>/dev/null || warn "config-merger_linux not in release."
        download_release_asset "aurago-remote_linux"               "bin/aurago-remote_linux"               2>/dev/null || warn "aurago-remote_linux not in release."
    fi
    # Record installed version for update checks
    printf '%s' "$RELEASE_TAG" > "$INSTALL_DIR/.version"
    ok "Binaries downloaded."
fi

chmod +x bin/aurago_linux bin/config-merger_linux bin/aurago-remote_linux 2>/dev/null || true
if [ -n "${TMPEXT:-}" ] && [ -d "$TMPEXT/assets/web" ]; then
    bin/aurago_linux --assets-dir "$INSTALL_DIR/assets/web" --import-assets-dir "$TMPEXT/assets/web" || die "Failed to import matching web resources."
    rm -rf "$TMPEXT"
    TMPEXT=""
fi
bin/aurago_linux --assets-dir "$INSTALL_DIR/assets/web" --check-assets || die "Binary and web resources do not match."
ok "Binaries ready."

section "Configuration"
if ! $BUILD_FROM_SOURCE; then
    if [ -f "$INSTALL_DIR/config.yaml.new_template" ]; then
        if [ -n "${EXISTING_CONFIG_BAK:-}" ] && [ -f "${EXISTING_CONFIG_BAK:-}" ]; then
            if [ -x "$INSTALL_DIR/bin/config-merger_linux" ]; then
                info "Merging existing config.yaml with new template defaults ..."
                if "$INSTALL_DIR/bin/config-merger_linux" -source "$EXISTING_CONFIG_BAK" -template "$INSTALL_DIR/config.yaml.new_template" -output "$INSTALL_DIR/config.yaml"; then
                    ok "config.yaml merged with latest template defaults."
                else
                    warn "config-merger failed. Restoring previous config.yaml."
                    cp -p "$EXISTING_CONFIG_BAK" "$INSTALL_DIR/config.yaml"
                fi
            else
                warn "config-merger not available. Keeping existing config.yaml unchanged."
            fi
        elif [ ! -f "$INSTALL_DIR/config.yaml" ]; then
            cp "$INSTALL_DIR/config.yaml.new_template" "$INSTALL_DIR/config.yaml"
            ok "config.yaml installed from release template."
        fi
        rm -f "$INSTALL_DIR/config.yaml.new_template"
    fi
    [ -n "${EXISTING_CONFIG_BAK:-}" ] && rm -f "$EXISTING_CONFIG_BAK"
    EXISTING_CONFIG_BAK=""

    if download_release_asset "update.sh" "$INSTALL_DIR/update.sh"; then
        chmod +x "$INSTALL_DIR/update.sh"
        ok "update.sh installed."
    else
        warn "Could not download verified update.sh — install it manually later."
    fi

    [ -n "${RELEASE_CHECKSUMS_FILE:-}" ] && rm -f "$RELEASE_CHECKSUMS_FILE"
    RELEASE_CHECKSUMS_FILE=""
fi

# ── Master key ────────────────────────────────────────────────────────────
ENV_FILE="$INSTALL_DIR/.env"
if [ -f "$ENV_FILE" ] && grep -q "AURAGO_MASTER_KEY" "$ENV_FILE"; then
    EXISTING_MASTER_KEY="$(read_env_value "$ENV_FILE" "AURAGO_MASTER_KEY" || true)"
    is_valid_master_key "$EXISTING_MASTER_KEY" || die "Existing $ENV_FILE contains an invalid AURAGO_MASTER_KEY."
    warn ".env already has AURAGO_MASTER_KEY — keeping existing key."
else
    MASTER_KEY="$(generate_master_key || true)"
    is_valid_master_key "$MASTER_KEY" || die "Failed to generate a valid AURAGO_MASTER_KEY. Please install openssl or python3."
    write_master_key_file "$ENV_FILE" "$MASTER_KEY" || die "Failed to write $ENV_FILE securely."
    ok "Master key generated → $ENV_FILE"
    warn "Keep .env safe! Losing it means losing access to your encrypted vault."
fi

# ── start.sh ─────────────────────────────────────────────────────────────
cat > "$INSTALL_DIR/start.sh" <<'STARTSH'
#!/usr/bin/env bash
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$DIR"

read_env_value() {
    local env_file="$1"
    local env_key="$2"
    [ -f "$env_file" ] || return 1
    awk -F= -v key="$env_key" '
        $1 == key {
            sub(/^[^=]*=/, "", $0)
            gsub(/^[[:space:]]+|[[:space:]]+$/, "", $0)
            gsub(/^["'"'"']|["'"'"']$/, "", $0)
            print $0
            exit
        }
    ' "$env_file"
}

# Load master key: prefer system-wide credential, fall back to local .env
if [ -f /etc/aurago/master.key ]; then
    AURAGO_MASTER_KEY="$(read_env_value /etc/aurago/master.key AURAGO_MASTER_KEY)"
elif [ -f "$DIR/.env" ]; then
    AURAGO_MASTER_KEY="$(read_env_value "$DIR/.env" AURAGO_MASTER_KEY)"
fi
export AURAGO_MASTER_KEY

if [ -z "${AURAGO_MASTER_KEY:-}" ]; then
    echo "ERROR: AURAGO_MASTER_KEY is not set."
    echo "  Expected at: /etc/aurago/master.key  or  $DIR/.env"
    exit 1
fi

echo "Starting AuraGo..."
./bin/aurago_linux > log/aurago.log 2>&1 &
echo "Started (PID=$!). Web UI endpoint: see the installation summary or server settings in $DIR/config.yaml"
echo "Follow logs: tail -f $DIR/log/aurago.log"
STARTSH
chmod +x "$INSTALL_DIR/start.sh"

# ── Network binding & HTTPS ───────────────────────────────────────────────
section "Network & security"
info "Configure external access and HTTPS for this installation."
echo ""

prompt_value HTTPS_REPLY "Is this an internet-facing server and do you want to enable HTTPS (Let's Encrypt)? [y/N]: " "n" "n"

SERVER_HOST="127.0.0.1"
HTTPS_ENABLED="false"

if [[ "${HTTPS_REPLY:-n}" =~ ^[Yy]$ ]]; then
    SERVER_HOST="0.0.0.0"
    HTTPS_ENABLED="true"
    prompt_value HTTPS_DOMAIN "Enter your domain (e.g., aurago.example.com): " "" ""
    prompt_value HTTPS_EMAIL "Enter your email for Let's Encrypt: " "" ""
    ok "Web UI will listen on ALL interfaces (0.0.0.0:443) with HTTPS."
else
    echo ""
    echo "  Only allow network access if AuraGo runs on a trusted local LAN"
    echo "  (e.g. a home server / Proxmox container) — never expose it directly"
    echo "  to the internet without HTTPS / reverse proxy."
    echo ""
    prompt_value NET_REPLY "Enable HTTP access from outside localhost (LAN)? [Y/n]: " "y" "n"
    if [[ "${NET_REPLY:-n}" =~ ^[Yy]$ ]]; then
        SERVER_HOST="0.0.0.0"
        warn "Web UI will listen on ALL interfaces (0.0.0.0:8088) without HTTPS."
    else
        ok "Web UI will only be reachable locally (127.0.0.1:8088)."
    fi
fi

# ── CAP_NET_BIND_SERVICE for HTTPS on standard ports ─────────────────────
# Ports 80 and 443 require root or this capability. Set it so AuraGo
# can bind them as a normal user — prevents a hard startup failure.
if [ "$HTTPS_ENABLED" = "true" ]; then
    if ! command -v setcap >/dev/null 2>&1; then
        info "Installing libcap2-bin (needed for setcap)..."
        _pkg_install libcap2-bin 2>/dev/null || \
        _pkg_install libcap 2>/dev/null || \
        warn "setcap not available. Install libcap2-bin and run manually: sudo setcap cap_net_bind_service=+ep $INSTALL_DIR/bin/aurago_linux"
    fi
    if command -v setcap >/dev/null 2>&1; then
        $SUDO setcap cap_net_bind_service=+ep "$INSTALL_DIR/bin/aurago_linux" 2>/dev/null && \
            ok "CAP_NET_BIND_SERVICE granted — AuraGo can bind port 443 without root." || \
            warn "setcap failed. Run manually: sudo setcap cap_net_bind_service=+ep $INSTALL_DIR/bin/aurago_linux"
    fi
fi

CONFIG_FILE="$INSTALL_DIR/config.yaml"
if [ -f "$CONFIG_FILE" ]; then
    INFO_PASSWORD=$(openssl rand -base64 12 2>/dev/null || python3 -c "import secrets; print(secrets.token_urlsafe(12))")
    INITIAL_PASSWORD_FILE="$(mktemp "${INSTALL_DIR}/.initial-password.XXXXXX")"
    write_secret_text_file "$INITIAL_PASSWORD_FILE" "$INFO_PASSWORD" || die "Failed to write temporary initial password securely."

    # Save first-use password (owner-readable only)
    write_secret_text_file "$INSTALL_DIR/firstpassword.txt" "$INFO_PASSWORD" || die "Failed to write firstpassword.txt securely."

    if [ "$HTTPS_ENABLED" = "true" ]; then
        if ! ./bin/aurago_linux --config "$CONFIG_FILE" --init-only -password-file "$INITIAL_PASSWORD_FILE" -https -domain "$HTTPS_DOMAIN" -email "$HTTPS_EMAIL"; then
            die "Initial password and HTTPS setup failed. Check config.yaml and AURAGO_MASTER_KEY."
        fi
        ok "config.yaml → HTTPS enabled for $HTTPS_DOMAIN"
    else
        if ! ./bin/aurago_linux --config "$CONFIG_FILE" --init-only -password-file "$INITIAL_PASSWORD_FILE"; then
            die "Initial password setup failed. Check config.yaml and AURAGO_MASTER_KEY."
        fi
        awk -v host="$SERVER_HOST" '
            /^server:/ { in_server=1 }
            /^[a-z]/ && !/^server:/ { in_server=0 }
            in_server && /^[[:space:]]+host:/ { sub(/host:.*/, "host: " host) }
            { print }
        ' "$CONFIG_FILE" > "$CONFIG_FILE.tmp" && mv "$CONFIG_FILE.tmp" "$CONFIG_FILE"
        ok "config.yaml → server.host set to $SERVER_HOST"
    fi
else
    # config.yaml still missing — create from template as last resort
    TEMPLATE_FILE="$INSTALL_DIR/config_template.yaml"
    if [ -f "$TEMPLATE_FILE" ]; then
        cp "$TEMPLATE_FILE" "$CONFIG_FILE"
        # Apply chosen host binding directly via awk
        awk -v host="$SERVER_HOST" '
            /^server:/ { in_server=1 }
            /^[a-z]/ && !/^server:/ { in_server=0 }
            in_server && /^[[:space:]]+host:/ { sub(/host:.*/, "host: " host) }
            { print }
        ' "$CONFIG_FILE" > "$CONFIG_FILE.tmp" && mv "$CONFIG_FILE.tmp" "$CONFIG_FILE"
        ok "config.yaml created from template (server.host=$SERVER_HOST)."
    else
        warn "config.yaml not found and no template available — skipping host configuration."
    fi
fi


# ── Optional systemd service ──────────────────────────────────────────────
section "Service"
SERVICE_INSTALLED=false
if ! command -v systemctl >/dev/null 2>&1; then
    info "systemd not available - skipping service installation."
fi
if command -v systemctl >/dev/null 2>&1; then
    echo ""
    prompt_value SVC_REPLY "Install as systemd service (auto-start on boot)? [Y/n]: " "y" "n"
    if [[ "${SVC_REPLY:-y}" =~ ^[Yy]$ ]]; then

        # ── Move master key to /etc/aurago/master.key (root-only) ─────────
        CREDENTIAL_DIR="/etc/aurago"
        CREDENTIAL_FILE="${CREDENTIAL_DIR}/master.key"
        if [ -f "$CREDENTIAL_FILE" ] && grep -q "AURAGO_MASTER_KEY" "$CREDENTIAL_FILE"; then
            warn "$CREDENTIAL_FILE already exists — keeping existing key."
        else
            AURAGO_MASTER_KEY="$(read_env_value "$ENV_FILE" "AURAGO_MASTER_KEY" || true)"
            is_valid_master_key "$AURAGO_MASTER_KEY" || die "Cannot migrate master key — AURAGO_MASTER_KEY is missing or invalid."
            $SUDO mkdir -p "$CREDENTIAL_DIR"
            $SUDO chmod 700 "$CREDENTIAL_DIR"
            printf "AURAGO_MASTER_KEY=%s\n" "$AURAGO_MASTER_KEY" | $SUDO tee "$CREDENTIAL_FILE" > /dev/null
            $SUDO chmod 600 "$CREDENTIAL_FILE"
            $SUDO chown root:root "$CREDENTIAL_DIR" "$CREDENTIAL_FILE"
            CREDENTIAL_FILE_CREATED=1
            CREATED_CREDENTIAL_FILE="$CREDENTIAL_FILE"
            MIGRATED_MASTER_KEY="$AURAGO_MASTER_KEY"
            MIGRATED_ENV_FILE="$ENV_FILE"
            ok "Master key moved to $CREDENTIAL_FILE (root-only, mode 0600)."
        fi

        # Remove the plaintext .env from the install directory
        if [ -f "$ENV_FILE" ]; then
            rm -f "$ENV_FILE"
            ok "Removed $ENV_FILE (no longer needed — key is in $CREDENTIAL_FILE)."
        fi

        # ── Determine service user ─────────────────────────────────────────
        # When invoked via 'sudo ./install.sh', SUDO_USER is the real user.
        # When run directly as a non-root user, use the current user.
        # Avoid running the service as root if at all possible.
        if [ -n "${SUDO_USER:-}" ]; then
            SERVICE_USER="$SUDO_USER"
            SERVICE_GROUP="$(id -gn "$SUDO_USER")"
        elif [ "$(id -u)" -ne 0 ]; then
            SERVICE_USER="$(id -un)"
            SERVICE_GROUP="$(id -gn)"
        else
            # Running directly as root — derive user from install directory owner
            _dir_owner="$(stat_owner "$INSTALL_DIR" 2>/dev/null || echo '')"
            if [ -n "$_dir_owner" ] && [ "$_dir_owner" != "root" ]; then
                SERVICE_USER="$_dir_owner"
                SERVICE_GROUP=$(id -gn "$_dir_owner" 2>/dev/null || echo "$_dir_owner")
            else
                SERVICE_USER="root"
                SERVICE_GROUP="root"
                warn "Could not determine a non-root service user. Service will run as root."
                warn "For better security, create a dedicated user: useradd -r -s /bin/false aurago"
            fi
        fi
        ok "Service will run as: ${SERVICE_USER}:${SERVICE_GROUP}"

        warn_if_systemd_hardening_conflicts "$CONFIG_FILE"

        # Ensure only AuraGo-managed writable paths are owned by the service user.
        CHOWN_TARGETS=()
        for _path in \
            "$INSTALL_DIR/bin" \
            "$INSTALL_DIR/data" \
            "$INSTALL_DIR/log" \
            "$INSTALL_DIR/agent_workspace" \
            "$INSTALL_DIR/config.yaml" \
            "$INSTALL_DIR/config_template.yaml" \
            "$INSTALL_DIR/start.sh" \
            "$INSTALL_DIR/update.sh" \
            "$INSTALL_DIR/firstpassword.txt" \
            "$INSTALL_DIR/.version"; do
            [ -e "$_path" ] && CHOWN_TARGETS+=("$_path")
        done
        if [ "${#CHOWN_TARGETS[@]}" -gt 0 ]; then
            $SUDO chown -R "${SERVICE_USER}:${SERVICE_GROUP}" "${CHOWN_TARGETS[@]}" 2>/dev/null || true
        fi

        # Grant CAP_NET_BIND_SERVICE in the systemd unit so the service can bind
        # ports 80/443 as a non-root user without a setcap dependency on the binary.
        # AmbientCapabilities is compatible with NoNewPrivileges=true (systemd sets
        # the capability before the prctl call).
        AMBIENT_CAPS_LINE=""
        if [ "$HTTPS_ENABLED" = "true" ]; then
            AMBIENT_CAPS_LINE="AmbientCapabilities=CAP_NET_BIND_SERVICE"
        fi
        PROTECT_SYSTEM_LINE="ProtectSystem=strict"
        NO_NEW_PRIVILEGES_LINE="NoNewPrivileges=true"
        if grep -Eq '^[[:space:]]+sudo_enabled:[[:space:]]*true([[:space:]]|$)' "$CONFIG_FILE"; then
            NO_NEW_PRIVILEGES_LINE="# NoNewPrivileges=true disabled because sudo_enabled is enabled"
        fi
        if grep -Eq '^[[:space:]]+sudo_unrestricted:[[:space:]]*true([[:space:]]|$)' "$CONFIG_FILE"; then
            PROTECT_SYSTEM_LINE="# ProtectSystem=strict disabled because sudo_unrestricted is enabled"
        fi
        GPU_GROUPS_LINE="$(systemd_gpu_groups_line)"
        SERIAL_GROUPS_LINE="$(systemd_serial_groups_line)"
        if [ -n "$SERIAL_GROUPS_LINE" ]; then
            info "Granting the service USB serial access: ${SERIAL_GROUPS_LINE#SupplementaryGroups=}"
        fi
        GPU_GROUP_IDS="$(system_gpu_group_ids)"
        GPU_GROUP_IDS_LINE=""
        if [ -n "$GPU_GROUPS_LINE" ]; then
            info "Granting the service access to available GPU groups: ${GPU_GROUPS_LINE#SupplementaryGroups=}"
        fi
        if [ -n "$GPU_GROUP_IDS" ]; then
            GPU_GROUP_IDS_LINE="Environment=\"AURAGO_GPU_GROUP_IDS=${GPU_GROUP_IDS}\""
            info "Forwarding host GPU group IDs to managed containers: ${GPU_GROUP_IDS}"
        fi
        SYSTEMD_INSTALL_DIR="$(systemd_escape_path_value "$INSTALL_DIR")" || die "Install path contains unsupported control characters."
        SYSTEMD_BINARY_PATH="$(systemd_escape_path_value "$INSTALL_DIR/bin/aurago_linux")" || die "Binary path contains unsupported control characters."
        SYSTEMD_CONFIG_PATH="$(systemd_escape_path_value "$INSTALL_DIR/config.yaml")" || die "Config path contains unsupported control characters."
        SYSTEMD_CREDENTIAL_FILE="$(systemd_escape_path_value "$CREDENTIAL_FILE")" || die "Credential path contains unsupported control characters."
        SYSTEMD_CREDENTIAL_DIR="$(systemd_escape_path_value "$CREDENTIAL_DIR")" || die "Credential directory contains unsupported control characters."

        # ── Create systemd unit ──────────────────────────────────────────
        $SUDO tee /etc/systemd/system/${SYSTEMD_SERVICE}.service > /dev/null <<EOF
[Unit]
Description=AuraGo AI Agent
After=network.target
StartLimitIntervalSec=0

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_GROUP}
${GPU_GROUPS_LINE}
${SERIAL_GROUPS_LINE}
${GPU_GROUP_IDS_LINE}
WorkingDirectory=${SYSTEMD_INSTALL_DIR}
ExecStart=${SYSTEMD_BINARY_PATH} --config ${SYSTEMD_CONFIG_PATH}
Restart=on-failure
RestartSec=5
TimeoutStopSec=60s
EnvironmentFile=${SYSTEMD_CREDENTIAL_FILE}
${AMBIENT_CAPS_LINE}
# Security hardening
${NO_NEW_PRIVILEGES_LINE}
${PROTECT_SYSTEM_LINE}
ReadWritePaths=${SYSTEMD_INSTALL_DIR} ${SYSTEMD_CREDENTIAL_DIR}
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF
        SERVICE_FILE_CREATED=1
        if command -v systemd-analyze >/dev/null 2>&1; then
            if ! $SUDO systemd-analyze verify "/etc/systemd/system/${SYSTEMD_SERVICE}.service"; then
                die "Generated systemd unit failed validation."
            fi
        fi
        $SUDO systemctl daemon-reload
        $SUDO systemctl enable "$SYSTEMD_SERVICE"
        SERVICE_ENABLED=1
        btk_run_choice "$BT_CHOICE" "$INSTALL_DIR" "$SERVICE_USER" "$SYSTEMD_SERVICE"
        $SUDO systemctl start "$SYSTEMD_SERVICE"
        SERVICE_INSTALLED=true
        ok "Systemd service installed, enabled and started."

        echo ""
        tui_box ok "MASTER KEY SECURED" \
            "Location: ${BOLD}/etc/aurago/master.key${NC} (root-only, mode 0600)" \
            "The key is injected into AuraGo via systemd." \
            "${YELLOW}Back up this file! Losing it = losing your vault.${NC}"
    fi
fi

# ── Bluetooth without a service: prepare it for the installing user ──────
if [ "$SERVICE_INSTALLED" != "true" ]; then
    btk_run_choice "$BT_CHOICE" "$INSTALL_DIR" "${SUDO_USER:-$(id -un)}" ""
fi

# ── Summary ───────────────────────────────────────────────────────────────
HTTP_PORT="$(configured_server_port "$CONFIG_FILE" "port" "8088")"
HTTPS_PORT="$(configured_server_port "$CONFIG_FILE" "https_port" "443")"
if [ "$HTTPS_ENABLED" = "true" ]; then
    WEB_UI_URL="https://${HTTPS_DOMAIN}:${HTTPS_PORT}"
    WEB_UI_LISTEN="0.0.0.0:${HTTPS_PORT} (HTTPS)"
elif [ "$SERVER_HOST" = "0.0.0.0" ]; then
    LAN_IP="$(primary_lan_ipv4 || true)"
    WEB_UI_URL="http://${LAN_IP:-<server-LAN-IP>}:${HTTP_PORT}"
    WEB_UI_LISTEN="0.0.0.0:${HTTP_PORT} (HTTP, LAN)"
else
    WEB_UI_URL="http://127.0.0.1:${HTTP_PORT}"
    WEB_UI_LISTEN="127.0.0.1:${HTTP_PORT} (HTTP, local only)"
fi

section "Done"
tui_sparkle
tui_box ok "AuraGo successfully installed!" \
    "${DIM}Location:${NC}       $INSTALL_DIR" \
    "${CYAN}Web UI:${NC}         $WEB_UI_URL" \
    "${CYAN}Listening on:${NC}   $WEB_UI_LISTEN" \
    "${DIM}Finished in $(tui_elapsed)${NC}"
echo ""

if [ -n "${INFO_PASSWORD:-}" ]; then
    tui_box brand "FIRST-USE PASSWORD" \
        "Password: ${BOLD}${INFO_PASSWORD}${NC}" \
        "Use this to log in to the Web UI for the first time." \
        "${YELLOW}Change it immediately via Settings -> Login Guard.${NC}" \
        "${DIM}Also saved to: firstpassword.txt (delete after first login)${NC}"
    echo ""
fi

if [ "$SERVICE_INSTALLED" = "true" ]; then
    tui_subsection "Next steps"
    echo "  1. Open the Web UI and go to Config -> Providers"
    echo "     Add your LLM provider and API key there."
    echo "  2. Restart after config change: sudo systemctl restart $SYSTEMD_SERVICE"
    echo ""
    echo -e "  ${CYAN}Service status:${NC}  sudo systemctl status $SYSTEMD_SERVICE"
    echo -e "  ${CYAN}Logs:           ${NC}  sudo journalctl -u $SYSTEMD_SERVICE -f"
    echo -e "  ${CYAN}Master key:    ${NC}  /etc/aurago/master.key (root-only)"
else
    tui_subsection "Next steps"
    echo "  1. Open the Web UI and go to Config -> Providers"
    echo "     Add your LLM provider and API key there."
    echo "  2. Restart after config change: cd $INSTALL_DIR && ./start.sh"
    echo "  3. Open UI:      $WEB_UI_URL"
    echo ""
    echo -e "  ${CYAN}Logs:${NC}  tail -f $INSTALL_DIR/log/aurago.log"

    # Start AuraGo now
    cd "$INSTALL_DIR"
    bash start.sh
fi
echo ""
if $BUILD_FROM_SOURCE; then
    echo -e "  ${CYAN}Update later:${NC}  cd $INSTALL_DIR && bash update.sh"
    echo    "               (manual builds: see documentation/web-assets.md; resource flags are required)"
else
    echo -e "  ${CYAN}Update later:${NC}  cd $INSTALL_DIR && bash update.sh"
    echo    "               (downloads latest release and merges your config automatically)"
fi
echo ""
printf '  '; tui_grad "Setup complete! Finish configuration in the Web UI." 0 1000; printf '\n'
echo -e "  Go to the ${BOLD}CONFIG${NC} section to set up your LLM provider and API keys."
INSTALL_SUCCESS=1
echo ""

if [ "$PYTHON_MISSING" = "true" ]; then
    tui_box warn "Python not installed" \
        "Python tools and skills will not work."
    echo ""
fi
