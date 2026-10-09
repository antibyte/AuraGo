#!/usr/bin/env bash
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
#  AuraGo Update Script (Linux)
#
#  Usage:  ./update.sh [--yes] [--no-restart] [--force-reset] [--rebuild]
#  Diverged local deployment commits are merged automatically. --force-reset
#  explicitly replaces them with origin/main instead.
#
#  What it does:
#    1. Fetches the latest commit from GitHub (no clobber of user data)
#    2. Preserves ALL user-specific files:
#         .env, config.yaml, config_debug.yaml,
#         data/*, log/*, agent_workspace/tools/*, agent_workspace/skills/*,
#         agent_workspace/workdir/*, agent_workspace/prompts/* (custom only)
#    3. Applies only code / binary / UI / documentation changes
#    4. Optionally restarts the systemd service or background process
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
set -euo pipefail

# ── UI & Typography ──────────────────────────────────────────────────────
# Text-mode look (logo, cards, steps, spinner). Plain timestamped lines when
# stdout is not a terminal (the in-app updater appends to log/update.log);
# AURAGO_UI=plain, AURAGO_NO_ANIM=1 and NO_COLOR=1 are honoured.
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

info()       { tui_msg info "$*"; }
ok()         { tui_msg ok "$*"; }
warn()       { tui_msg warn "$*"; }
die()        { tui_die "$*"; exit 1; }
section()    { tui_section "$*"; }
subsection() { tui_subsection "$*"; }

# ── CLI flags ──────────────────────────────────────────────────────────
AUTO_YES=false
NO_RESTART=false
FORCE_RESET=false
REBUILD=false
BT_FLAG=""
_AU_ESCAPED=""
for arg in "$@"; do
    case "$arg" in
        --yes)        AUTO_YES=true ;;
        --no-restart) NO_RESTART=true ;;
        --force-reset) FORCE_RESET=true ;;
        --rebuild)     REBUILD=true ;;
        --bluetooth)    BT_FLAG=yes ;;
        --no-bluetooth) BT_FLAG=no ;;
        --escaped)    _AU_ESCAPED=1 ;;   # internal: already running in an independent scope
        --help|-h)
            echo "Usage: $0 [--yes] [--no-restart] [--force-reset] [--rebuild] [--bluetooth|--no-bluetooth]"
            echo "  --yes          Skip confirmation prompts (never answers the Bluetooth question)"
            echo "  --no-restart   Do not restart the service after update"
            echo "  --force-reset  Replace diverged local commits with origin/main instead of preserving them"
            echo "  --rebuild      Rebuild/reinstall even when the version is unchanged"
            echo "  --bluetooth    Use Bluetooth in AuraGo and prepare this server for it"
            echo "  --no-bluetooth Turn Bluetooth off in AuraGo"
            exit 0 ;;
        *) warn "Unknown argument: $arg" ;;
    esac
done

confirm() {
    local msg="$1"
    if $AUTO_YES; then return 0; fi
    printf '  %s?%s  %s [y/N]: ' "$CYAN" "$NC" "$msg" >/dev/tty
    read -r REPLY </dev/tty
    [[ "${REPLY:-n}" =~ ^[Yy]$ ]]
}

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

# Fetches must never stop an unattended updater to ask for Git credentials.
# Cached non-interactive credentials remain usable for private forks, while
# terminal and Git Credential Manager prompts are disabled explicitly.
git_fetch_origin_main() {
    local attempt
    local retry_delay
    for attempt in 1 2 3; do
        if GIT_TERMINAL_PROMPT=0 GCM_INTERACTIVE=never \
            git -c credential.interactive=never -c http.version=HTTP/1.1 -C "$DIR" fetch origin main --quiet; then
            return 0
        fi
        if [ "$attempt" -lt 3 ]; then
            retry_delay=$((attempt * 3))
            warn "Git fetch failed; retrying without interactive authentication in ${retry_delay}s..." >&2
            sleep "$retry_delay"
        fi
    done
    return 1
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

ensure_private_update_runtime_dir() {
    local dir="/tmp/aurago-update-$(id -u)"
    if [ -e "$dir" ] && [ ! -d "$dir" ]; then
        die "Unsafe update runtime path exists and is not a directory: $dir"
    fi
    mkdir -p "$dir"
    chmod 700 "$dir" 2>/dev/null || true
    if [ -L "$dir" ]; then
        die "Unsafe update runtime path is a symlink: $dir"
    fi
    printf '%s\n' "$dir"
}

remove_regular_file_if_present() {
    local path="$1"
    if [ -L "$path" ]; then
        warn "Refusing to remove symlink lock file: $path"
        return 1
    fi
    if [ -f "$path" ]; then
        rm -f -- "$path"
        return 0
    fi
    return 1
}

# Update retention helpers are deliberately self-contained: update.sh also runs
# in binary installations and from a detached copy while the checkout changes.
update_json_string() {
    local value="$1"
    value="${value//\\/\\\\}"
    value="${value//\"/\\\"}"
    value="${value//$'\n'/\\n}"
    value="${value//$'\r'/\\r}"
    value="${value//$'\t'/\\t}"
    printf '"%s"' "$value"
}

update_manifest() {
    local status="$1" tmp
    [ -n "${BACKUP_DIR:-}" ] || return 0
    tmp="$(mktemp "$BACKUP_DIR/.manifest.XXXXXX")" || return 1
    {
        printf '{"version":1,"root":'; update_json_string "$DIR"
        printf ',"id":'; update_json_string "${BACKUP_DIR##*/}"
        printf ',"created":%s,"status":' "$UPDATE_CREATED"; update_json_string "$status"
        printf ',"previous_version":"%s","previous_asset":"%s","new_asset":"%s","new_version":"%s","backup_complete":%s}\n' \
            "$UPDATE_PREVIOUS_VERSION" "$UPDATE_PREVIOUS_ASSET" "${UPDATE_NEW_ASSET:-}" "${UPDATE_NEW_VERSION:-}" "${UPDATE_BACKUP_COMPLETE:-false}"
    } > "$tmp"
    chmod 600 "$tmp"
    mv -f -- "$tmp" "$BACKUP_DIR/manifest.json"
    UPDATE_OUTCOME="$status"
}

update_asset_id() {
    local metadata id
    metadata="$("$1" --assets-info)" || return 1
    id="$(printf '%s' "$metadata" | sed -n 's/.*"asset_set_id"[[:space:]]*:[[:space:]]*"\([a-f0-9]\{64\}\)".*/\1/p')"
    [[ "$id" =~ ^[a-f0-9]{64}$ ]] || return 1
    printf '%s' "$id"
}

update_space_check() {
    local needed="$1" available
    available="$(df -Pk "$DIR" | awk 'NR==2 {print $4}')"
    [[ "$available" =~ ^[0-9]+$ ]] || { echo "Cannot determine free update space." >&2; return 1; }
    [ "$available" -ge "$needed" ] || { echo "Not enough free space for update (need at least $((needed / 1024)) MiB); retained rollback data will not be deleted." >&2; return 1; }
}

update_safe_remove_work() {
    local path="$1" parent
    [ -d "$path" ] && [ ! -L "$path" ] || return 0
    parent="$(cd -- "$(dirname -- "$path")" && pwd -P)" || return 1
    [ "$parent" = "$UPDATE_STATE" ] && [[ "${path##*/}" == work.* ]] || return 1
    rm -rf -- "$path"
}

update_exit_cleanup() {
    local rc=$?
    tui_cleanup
    trap - EXIT
    # Unexpected exit leaves the transaction unresolved and blocks a new update.
    if [ "${UPDATE_OUTCOME:-}" = pending ]; then update_manifest uncertain || true; fi
    if [ -n "${UPDATE_WORK:-}" ]; then update_safe_remove_work "$UPDATE_WORK" || warn "Temporary update cleanup failed."; fi
    if [ -d "${GOCACHE:-/nonexistent}" ] && [ "${GOCACHE:-}" = "$UPDATE_STATE/go-cache" ] && command -v go >/dev/null 2>&1; then
        local cache_kib
        cache_kib="$(du -sk "$GOCACHE" 2>/dev/null | awk '{print $1}')"
        if [[ "$cache_kib" =~ ^[0-9]+$ ]] && [ "$cache_kib" -gt 4194304 ]; then
            go clean -cache || warn "Dedicated Go cache cleanup failed."
        fi
    fi
    remove_regular_file_if_present "$_AU_LOCK" >/dev/null || true
    # Match the service account after root-run updates as for data/bin above.
    if [ -n "${_svc_user:-}" ] && [ -n "${_svc_group:-}" ]; then
        $SUDO chown -R "${_svc_user}:${_svc_group}" "$UPDATE_STATE" 2>/dev/null || warn "Update state ownership could not be restored."
    fi
    remove_regular_file_if_present "${BASH_SOURCE[0]}" >/dev/null || true
    [ -z "${RELEASE_CHECKSUMS_FILE:-}" ] || remove_regular_file_if_present "$RELEASE_CHECKSUMS_FILE" >/dev/null || true
    exit "$rc"
}

update_retention_cleanup() {
    local binary="$1" output
    local -a summary_args=()
    # A rollback may restore a binary predating compact maintenance output.
    if "$binary" --update-maintenance --help 2>&1 | grep -q -- '-summary'; then
        summary_args=(--summary)
    fi
    if output="$("$binary" --update-maintenance --root "$DIR" --apply --adopt-legacy "${summary_args[@]}")"; then
        if [ "${#summary_args[@]}" -gt 0 ]; then
            ok "$output"
        else
            ok "Artifact cleanup completed."
        fi
    else
        [ -z "$output" ] || printf '%s\n' "$output" >&2
        warn "Update is healthy, but artifact cleanup failed; retained data is safe. See the maintenance result above."
    fi
}

mark_executable_if_present() {
    local path="$1"
    [ -f "$path" ] || return 0
    chmod +x "$path" 2>/dev/null || $SUDO chmod +x "$path" 2>/dev/null || true
}

apply_aurago_setcap_if_available() {
    local binary="$DIR/bin/aurago_linux"
    [ -f "$binary" ] || binary="$DIR/bin/aurago"
    [ -f "$binary" ] || return 0
    command -v setcap >/dev/null 2>&1 || return 0
    setcap cap_net_bind_service=+ep "$binary" 2>/dev/null || \
        $SUDO setcap cap_net_bind_service=+ep "$binary" 2>/dev/null || \
        warn "setcap failed on ${binary} — run manually if you need HTTPS on privileged ports."
}

# ── Find installation directory ────────────────────────────────────────
# _AU_ORIG_DIR is exported when re-execing from a temp copy (see below).
# In that case BASH_SOURCE[0] points to /tmp/... so we must use the saved path.
if [ -n "${_AU_ORIG_DIR:-}" ]; then
    DIR="$_AU_ORIG_DIR"
else
    DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
fi
cd "$DIR"
DIR="$(pwd -P)"
if printf '%s' "$DIR" | LC_ALL=C grep -q '[[:cntrl:]]'; then die "Installation path contains control characters."; fi

if [ ! -f "$DIR/go.mod" ] && [ ! -f "$DIR/bin/aurago_linux" ]; then
    die "Could not find AuraGo installation at $DIR. Is update.sh in the right place?"
fi

# ── Single-instance guard ──────────────────────────────────────────────
# Prevents re-entrant execution caused by:
#   • bash lazy re-reads of a script replaced on disk by git pull
#   • git hooks or other subprocesses that inherit the environment
# Any invocation that finds this lock and the owning process alive exits silently.
_AU_RUNTIME_DIR="$(ensure_private_update_runtime_dir)"
_AU_LOCK="${_AU_RUNTIME_DIR}/update.lock"
if [ -f "$_AU_LOCK" ]; then
    _AU_LOCK_PID=$(cat "$_AU_LOCK" 2>/dev/null || echo 0)
    if [ "${_AU_LOCK_PID:-0}" -gt 0 ] && kill -0 "$_AU_LOCK_PID" 2>/dev/null; then
        exit 0  # Another update is already running — silently bail
    fi
    remove_regular_file_if_present "$_AU_LOCK" >/dev/null || true  # Stale lock from a dead process
fi

# ── Architecture detection ─────────────────────────────────────────────
ARCH_RAW=$(uname -m)
case "$ARCH_RAW" in
    x86_64)        GOARCH="amd64" ;;
    aarch64|arm64) GOARCH="arm64" ;;
    armv7l)        GOARCH="arm"; GOARM="7" ;;
    armv6l)        GOARCH="arm"; GOARM="6" ;;
    *)             GOARCH="amd64"; warn "Unknown architecture $ARCH_RAW — assuming amd64" ;;
esac

# ── Sudo strategy ─────────────────────────────────────────────────────
# When no interactive terminal is attached (triggered from web UI / nohup),
# use sudo -n so the command fails immediately instead of hanging on a
# password prompt.  stdin may be redirected by a wrapper while the process
# still has a controlling terminal, so probe the readable and writable
# /dev/tty instead of checking stdin alone.  Plain sudo reads the password
# from that controlling terminal and therefore remains usable in that case.
has_interactive_tty() {
    [ -r /dev/tty ] && [ -w /dev/tty ] && { : </dev/tty; } 2>/dev/null
}

if has_interactive_tty; then
    SUDO="sudo"
else
    SUDO="sudo -n"
fi

# ── Detect install mode ───────────────────────────────────────────────────
# Binary-only installs (no .git directory) are fully supported.
BINARY_ONLY=false
PRE_UPDATE_REF=""
GIT_VER=""
if [ ! -d "$DIR/.git" ]; then
    BINARY_ONLY=true
fi

GITHUB_REPO="antibyte/AuraGo"
RELEASE_BASE=""  # set in "Checking for updates" for binary mode

fetch_url_to_file() {
    local url="$1"
    local out="$2"
    if command -v curl >/dev/null 2>&1; then
        # Retry transient HTTP failures (including 503), not missing assets or
        # authentication failures. Bound connections, attempts and retry time.
        curl -fsSL --connect-timeout 15 --max-time 600 --retry 3 --retry-max-time 120 "$url" -o "$out"
    elif command -v wget >/dev/null 2>&1; then
        wget -q --timeout=60 --tries=4 --waitretry=2 --retry-on-http-error=408,429,500,502,503,504 "$url" -O "$out"
    else
        return 1
    fi
}

fetch_optional_url_to_file() {
    local url="$1"
    local out="$2"
    fetch_url_to_file "$url" "$out" 2>/dev/null
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
    sig_file="$(mktemp "/tmp/aurago-sha256-sig.XXXXXX")"
    cert_file="$(mktemp "/tmp/aurago-sha256-cert.XXXXXX")"

    if ! fetch_optional_url_to_file "${RELEASE_BASE}/SHA256SUMS.sig" "$sig_file" || ! fetch_optional_url_to_file "${RELEASE_BASE}/SHA256SUMS.pem" "$cert_file"; then
        rm -f "$sig_file" "$cert_file"
        if strict_release_verify_enabled; then
            die "Release signature files are missing and AURAGO_STRICT_RELEASE_VERIFY=1 is set."
        fi
        warn "Release signature files not found; continuing with SHA256 manifest verification only."
        return 0
    fi

    if ! command -v cosign >/dev/null 2>&1; then
        rm -f "$sig_file" "$cert_file"
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
        if strict_release_verify_enabled; then
            die "Release checksum signature verification failed."
        fi
        warn "Release checksum signature verification failed; continuing with SHA256 manifest verification only."
        return 0
    fi

    rm -f "$sig_file" "$cert_file"
}

fetch_release_checksums() {
    [ -n "${RELEASE_BASE:-}" ] || die "RELEASE_BASE is not set."
    if [ -n "${RELEASE_CHECKSUMS_FILE:-}" ] && [ -f "${RELEASE_CHECKSUMS_FILE:-}" ]; then
        return 0
    fi
    RELEASE_CHECKSUMS_FILE="$(mktemp "/tmp/aurago-sha256.XXXXXX")"
    if ! fetch_url_to_file "${RELEASE_BASE}/SHA256SUMS" "$RELEASE_CHECKSUMS_FILE"; then
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
    local url="${RELEASE_BASE}/${asset}"
    TUI_RUN_EPHEMERAL=1 TUI_RUN_WATCH="$dest" tui_run "Downloading ${asset}" fetch_url_to_file "$url" "$dest"
    verify_release_asset "$asset" "$dest"
}

_download_release_bin() {
    local name="$1"
    local dest="${2:-$DIR/bin/$name}"
    mkdir -p "$(dirname "$dest")"
    download_release_asset "$name" "$dest"
}

select_release_bins_for_arch() {
    if [ "$GOARCH" = "arm64" ]; then
        REQUIRED_BINS=("aurago_linux_arm64" "config-merger_linux_arm64")
        OPTIONAL_BINS=("aurago-remote_linux_arm64")
    elif [ "$GOARCH" = "amd64" ]; then
        REQUIRED_BINS=("aurago_linux" "config-merger_linux")
        OPTIONAL_BINS=("aurago-remote_linux")
    else
        die "No prebuilt release binaries for architecture ${ARCH_RAW}. Install Go 1.27.2+ to build from source."
    fi
}

fetch_url_stdout() {
    local url="$1"
    local out rc=0
    out="$(mktemp "${UPDATE_WORK:-/tmp}/url.XXXXXX")" || return 1
    # A retry must replace partial JSON, never append it to the output pipe.
    if fetch_url_to_file "$url" "$out"; then
        cat "$out" || rc=$?
    else
        rc=$?
    fi
    rm -f -- "$out"
    return "$rc"
}

latest_release_tag() {
    fetch_url_stdout "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" \
        | grep -o '"tag_name": *"[^"]*"' \
        | head -1 \
        | cut -d'"' -f4
}

read_master_key_from_env() {
    local env_file="$1"
    local raw
    raw=$(grep -E '^AURAGO_MASTER_KEY=' "$env_file" | head -1 || true)
    raw="${raw#AURAGO_MASTER_KEY=}"
    raw="${raw%$'\r'}"
    # Remove surrounding quotes if present
    if [[ "$raw" == \"*\" ]]; then
        raw="${raw:1:-1}"
    elif [[ "$raw" == \'*\' ]]; then
        raw="${raw:1:-1}"
    fi
    printf '%s' "$raw"
}

safe_restore_file() {
    local src="$1"
    local dst="$2"

    [ -f "$src" ] || return 0

    # First try a normal forced overwrite with metadata preserved.
    if cp -fp "$src" "$dst" 2>/dev/null; then
        return 0
    fi

    # If destination exists but is not writable, remove then copy.
    if rm -f "$dst" 2>/dev/null && cp -fp "$src" "$dst" 2>/dev/null; then
        return 0
    fi

    # Last resort: sudo copy (for legacy root-owned files in user installs).
    if command -v sudo >/dev/null 2>&1; then
        if $SUDO cp -fp "$src" "$dst" 2>/dev/null; then
            $SUDO chown "$(id -u):$(id -g)" "$dst" 2>/dev/null || true
            return 0
        fi
    fi

    return 1
}

copy_tree_merge() {
    local src="$1"
    local dst="$2"

    # Ensure destination exists and is writable by the current user when possible.
    mkdir -p "$dst" 2>/dev/null || true
    if [ ! -w "$dst" ] && command -v sudo >/dev/null 2>&1; then
        $SUDO chown -R "$(id -u):$(id -g)" "$dst" 2>/dev/null || true
        $SUDO chmod -R u+rwX "$dst" 2>/dev/null || true
    fi

    if command -v rsync >/dev/null 2>&1; then
        # Avoid owner/group preservation to prevent non-fatal permission errors
        # on systems where destination files may be root-owned.
        # Also avoid timestamp preservation (-t) to prevent "failed to set times" warnings.
        rsync -rl --omit-dir-times --quiet --no-owner --no-group "$src" "$dst"
    else
        cp -r "$src" "$dst"
    fi
}

repair_worktree_permissions() {
    # Make changed tracked files and parent directories writable so git can
    # overwrite/unlink them during update.
    local changed
    changed="$(git -C "$DIR" status --porcelain --untracked-files=no 2>/dev/null | awk '{print $2}')"
    [ -n "$changed" ] || return 0

    while IFS= read -r rel; do
        [ -n "$rel" ] || continue
        local abs="$DIR/$rel"
        local parent
        parent="$(dirname "$abs")"

        if [ -e "$abs" ]; then
            chmod u+rw "$abs" 2>/dev/null || true
            if [ ! -w "$abs" ] && command -v sudo >/dev/null 2>&1; then
                $SUDO chown "$(id -u):$(id -g)" "$abs" 2>/dev/null || true
                $SUDO chmod u+rw "$abs" 2>/dev/null || true
            fi
        fi

        chmod u+rwx "$parent" 2>/dev/null || true
        if [ ! -w "$parent" ] && command -v sudo >/dev/null 2>&1; then
            $SUDO chown "$(id -u):$(id -g)" "$parent" 2>/dev/null || true
            $SUDO chmod u+rwx "$parent" 2>/dev/null || true
        fi
    done <<< "$changed"
}

clean_tracked_changes() {
    # Reset only tracked changes; user data/custom files are restored from backup.
    repair_worktree_permissions

    git -C "$DIR" restore --source=HEAD --staged --worktree . 2>/dev/null || true
    git -C "$DIR" checkout -- . 2>/dev/null || true
    git -C "$DIR" reset --quiet HEAD 2>/dev/null || true

    # Return success if tracked changes are gone.
    git -C "$DIR" diff --quiet && git -C "$DIR" diff --cached --quiet
}

GIT_LOCAL_AHEAD=0
GIT_REMOTE_AHEAD=0
GIT_INTEGRATION_ERROR=""

refresh_git_relationship() {
    local counts
    counts="$(git -C "$DIR" rev-list --left-right --count HEAD...origin/main 2>/dev/null)" || return 1
    read -r GIT_LOCAL_AHEAD GIT_REMOTE_AHEAD <<< "$counts"
    case "$GIT_LOCAL_AHEAD:$GIT_REMOTE_AHEAD" in
        *[!0-9:]*|:*|*:) return 1 ;;
    esac
}

integrate_origin_main() {
    GIT_INTEGRATION_ERROR=""
    if ! refresh_git_relationship; then
        GIT_INTEGRATION_ERROR="relationship_unavailable"
        return 1
    fi

    # The checkout already contains origin/main. Local rollout commits are
    # intentional and must not force a rebuild or be discarded.
    if [ "$GIT_REMOTE_AHEAD" -eq 0 ]; then
        return 0
    fi

    # Normal installations still use a strict fast-forward whenever possible.
    if [ "$GIT_LOCAL_AHEAD" -eq 0 ]; then
        if git -C "$DIR" merge --ff-only origin/main; then
            return 0
        fi
        GIT_INTEGRATION_ERROR="fast_forward_failed"
        return 1
    fi

    if $FORCE_RESET; then
        warn "Branches have diverged. --force-reset was supplied; resetting tracked files to origin/main."
        if git -C "$DIR" reset --hard origin/main; then
            ok "Hard reset complete."
            return 0
        fi
        GIT_INTEGRATION_ERROR="force_reset_failed"
        return 1
    fi

    # A local deployment branch and origin/main naturally diverge after the
    # next upstream commit. Preserve both histories with an explicit merge.
    # The updater runs from a temporary copy, so an incoming update.sh can be
    # merged safely while this process continues using the reviewed script.
    if ! git -C "$DIR" merge-base HEAD origin/main >/dev/null 2>&1; then
        GIT_INTEGRATION_ERROR="unrelated_histories"
        return 1
    fi

    local remote_short
    remote_short="$(git -C "$DIR" rev-parse --short origin/main 2>/dev/null || printf 'origin-main')"
    info "Preserving ${GIT_LOCAL_AHEAD} local commit(s) and merging ${GIT_REMOTE_AHEAD} remote commit(s)."
    if git -C "$DIR" \
        -c user.name="AuraGo Updater" \
        -c user.email="updater@localhost" \
        -c commit.gpgSign=false \
        -c merge.gpgSign=false \
        -c core.hooksPath=/dev/null \
        merge --no-ff --no-edit --no-verify \
        -m "chore(update): merge origin/main at ${remote_short}" origin/main; then
        ok "Remote changes merged; local commits were preserved."
        return 0
    fi

    if git -C "$DIR" rev-parse -q --verify MERGE_HEAD >/dev/null 2>&1; then
        git -C "$DIR" merge --abort >/dev/null 2>&1 || \
            git -C "$DIR" reset --hard "$PRE_UPDATE_REF" >/dev/null 2>&1 || true
        GIT_INTEGRATION_ERROR="merge_conflict"
    else
        GIT_INTEGRATION_ERROR="merge_failed"
    fi
    return 1
}

prepare_untracked_merge_collisions() {
    # Git refuses to update when an untracked file would become tracked.
    # This commonly happens when a runtime asset was deployed manually before
    # the same asset was added to the repository. Remove only byte-identical
    # regular files; preserve a copy in the update backup for audit/rollback.
    local collision_backup="$BACKUP_DIR/untracked_merge_collisions"
    local rel abs backup_path
    local cleared=0

    while IFS= read -r -d '' rel; do
        abs="$DIR/$rel"
        # The diff already contains only paths added by origin/main and absent
        # from HEAD. Inspect those concrete paths instead of scanning every
        # untracked runtime directory, some of which are intentionally private.
        [ -e "$abs" ] || [ -L "$abs" ] || continue
        if [ ! -f "$abs" ] || [ -L "$abs" ]; then
            warn "Untracked path would be replaced by the update but is not a regular file: $rel"
            return 1
        fi
        if ! git -C "$DIR" show "origin/main:$rel" | cmp -s -- "$abs" -; then
            warn "Untracked file differs from the incoming tracked file: $rel"
            warn "Move or rename it, then run the update again; AuraGo will not overwrite it."
            return 1
        fi

        backup_path="$collision_backup/$rel"
        mkdir -p "$(dirname "$backup_path")"
        cp -p -- "$abs" "$backup_path" || return 1
        rm -f -- "$abs" || return 1
        cleared=$((cleared + 1))
        ok "Prepared byte-identical untracked file for repository update: $rel"
    done < <(git -C "$DIR" diff --name-only -z --diff-filter=A --no-renames HEAD..origin/main --)

    if [ "$cleared" -gt 0 ]; then
        info "Saved $cleared replaced untracked file(s) under $collision_backup"
    fi
}

restore_untracked_merge_collisions_after_failure() {
    local collision_backup="$BACKUP_DIR/untracked_merge_collisions"
    local backup_path rel destination
    [ -d "$collision_backup" ] || return 0

    while IFS= read -r -d '' backup_path; do
        rel="${backup_path#"$collision_backup/"}"
        destination="$DIR/$rel"
        if [ -e "$destination" ] || [ -L "$destination" ]; then
            warn "Rollback kept existing path instead of overwriting it: $rel"
            continue
        fi
        mkdir -p "$(dirname "$destination")"
        cp -p -- "$backup_path" "$destination" || return 1
    done < <(find "$collision_backup" -type f -print0)
}

# ── Files & directories that must NEVER be touched ─────────────────────
# These are backed up before git operations and restored afterwards.
PROTECTED_FILES=(
    ".env"
    "config.yaml"
    "config_debug.yaml"
)
# Directories to back up fully (must be small — they go to /tmp).
# data/vectordb, data/embeddings, data/tts, data/vectordb_backup are intentionally excluded:
# they are gitignored (git never touches them) and can be very large.
# agent_workspace/workdir and agent_workspace/github are also excluded
# (ephemeral working state, gitignored, safe).
PROTECTED_DIRS=(
    "agent_workspace/tools"
    "agent_workspace/skills"
)
# Critical data files backed up individually (avoids copying large binary dirs)
DATA_FILES=(
    "data/character_journal.md"
    "data/chat_history.json"
    "data/crontab.json"
    "data/current_plan.md"
    "data/graph.json"
    "data/state.json"
    "data/media_registry.db"
    "data/homepage_registry.db"
    "data/cheatsheets.db"
    "data/inventory.db"
    "data/contacts.db"
    "data/knowledge_graph.db"
    "data/skills.db"
    "data/invasion.db"
    "data/image_gallery.db"
    "data/push.db"
    "data/remote_control.db"
    "data/sql_connections.db"
    "data/short_term.db"
)
# Prompt directories: protect all custom *.md files that are NOT tracked by git
PROMPTS_DIR="$DIR/prompts"

# Escape to a separate systemd scope only when actually running inside
# the aurago service cgroup. Manual shell runs do not need this path.
IN_AURAGO_CGROUP=false
if [ -r "/proc/$$/cgroup" ] && grep -qE 'aurago\.service' "/proc/$$/cgroup"; then
    IN_AURAGO_CGROUP=true
fi

# ── Escape systemd service cgroup ─────────────────────────────────────
# When triggered from the AuraGo web UI, this script runs inside the
# aurago systemd service cgroup.  By default (KillMode=control-group),
# systemd sends SIGTERM to *all* processes in that cgroup — including
# this script — the moment aurago's main process is stopped below.
# To survive that cleanup we try to re-exec ourselves in an independent
# transient scope before we touch any processes.
if $IN_AURAGO_CGROUP && [ -z "${_AU_ESCAPED:-}" ]; then
    if command -v systemd-run >/dev/null 2>&1; then
        # Prefer a user scope (no root required, needs active user session).
        # Pass --escaped as a CLI argument — this is 100% reliable regardless
        # of environment variable inheritance or file replacement mid-execution.
        # env-variable guards (export _AU_ESCAPED=1) can be lost when
        # systemd-run --scope uses the logind session environment instead of
        # the calling process's exported vars, or when git stash pop replaces
        # the running script on disk and bash re-reads the new content.
        if systemd-run --user --scope --quiet -- /bin/bash "$0" "--escaped" "$@" 2>/dev/null; then
            exit 0
        fi
        # Fall back to a system scope via sudo (password-less sudo only).
        if command -v sudo >/dev/null 2>&1; then
            if $SUDO systemd-run --scope --quiet -- /bin/bash "$0" "--escaped" "$@" 2>/dev/null; then
                exit 0
            fi
        fi
    fi
    # No escape possible — continue in the same cgroup.
    # Non-systemd installs are unaffected; systemd installs without sudo
    # may be interrupted by cgroup cleanup.  Use `sudo systemctl stop
    # aurago` + `sudo /path/to/update.sh --yes` for a guaranteed update.
fi

# ── Copy to temp to prevent mid-run file replacement ─────────────────
# bash reads scripts lazily in chunks from disk. git pull replaces this
# file during execution; subsequent reads start at the wrong byte offset
# in the new version, causing re-execution from near the top of the file.
# Running from a temp copy ensures git pull cannot affect our execution.
if [ -z "${_AU_TMPRUN:-}" ]; then
    _TMPS=$(mktemp "${_AU_RUNTIME_DIR}/script.XXXXXX")
    cp -- "$0" "$_TMPS"
    chmod +x "$_TMPS"
    export _AU_TMPRUN=1
    export _AU_ORIG_DIR="$DIR"
    exec /bin/bash "$_TMPS" "$@"
fi
# Running from temp copy: claim the single-instance lock and schedule cleanup.
_AU_RUNTIME_DIR="$(ensure_private_update_runtime_dir)"
_AU_LOCK="${_AU_RUNTIME_DIR}/update.lock"
# A kernel lock, shared with the maintenance CLI, closes the old PID-file race.
UPDATE_STATE="$DIR/.aurago-update"
[ ! -L "$UPDATE_STATE" ] || die "Unsafe update state symlink."
mkdir -p "$UPDATE_STATE"
chmod 700 "$UPDATE_STATE"
[ ! -L "$UPDATE_STATE/update.lock" ] || die "Unsafe update lock symlink."
command -v flock >/dev/null 2>&1 || die "flock is required for safe updates."
exec 9>"$UPDATE_STATE/update.lock"
flock -n 9 || die "Another update or cleanup is running."
export AURAGO_UPDATE_LOCK_FD=9
export GOCACHE="$UPDATE_STATE/go-cache"
[ ! -L "$GOCACHE" ] || die "Unsafe Go cache symlink."
mkdir -p "$UPDATE_STATE/transactions"
[ ! -L "$UPDATE_STATE/transactions" ] || die "Unsafe transaction directory."
UPDATE_OWNER_ID="$(printf '%s' "$DIR" | sha256sum | cut -c1-16)"
shopt -s dotglob
for _pending in "$UPDATE_STATE/transactions/"*; do
    [ -e "$_pending" ] || continue
    [[ "${_pending##*/}" =~ ^\.txn-[a-zA-Z0-9]+\.aurago-retired-${UPDATE_OWNER_ID}$ ]] && continue
    [ ! -L "$_pending" ] && [ -f "$_pending/manifest.json" ] && \
        grep -Eq '"status"[[:space:]]*:[[:space:]]*"(confirmed|rolled_back)"' "$_pending/manifest.json" || \
        die "An unresolved update blocks further updates: $_pending. Verify recovery and use --update-maintenance --resolve."
done
shopt -u dotglob
UPDATE_WORK="$(mktemp -d "$UPDATE_STATE/work.XXXXXX")"
echo $$ > "$_AU_LOCK"
trap update_exit_cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

# ── Banner ─────────────────────────────────────────────────────────────
TUI_STEP_TOTAL=7
tui_banner "Updater" "Keeping your AI Agent up to date"
info "Architecture: $ARCH_RAW → Go target: $GOARCH"
info "Installation: $DIR"
if $BINARY_ONLY; then
    info "Mode:         Binary-only (no git)"
else
    info "Remote:       $(git remote get-url origin 2>/dev/null || echo 'unknown')"
fi
echo ""

# ── Bluetooth (host preparation by the AURAGO-BLUETOOTH-KIT block) ─────
bluetooth_service_user() {
    local user=""
    if [ -f /etc/systemd/system/aurago.service ]; then
        user="$(sed -n 's/^User=//p' /etc/systemd/system/aurago.service | head -n 1)"
    fi
    [ -n "$user" ] || user="$(stat_owner "$DIR" 2>/dev/null || true)"
    printf '%s\n' "${user:-$(id -un)}"
}

bluetooth_apply_choice() {
    local service=""
    [ ! -f /etc/systemd/system/aurago.service ] || service="aurago"
    btk_run_choice "$BT_CHOICE" "$DIR" "$(bluetooth_service_user)" "$service"
}

# The version is current: apply only a new or explicitly requested decision.
bluetooth_apply_without_update() {
    [ "$BT_CHOICE" != skip ] || return 0
    if [ -z "$BT_FLAG" ] && [ "$BT_CHOICE" = "$BT_STORED" ]; then return 0; fi
    bluetooth_apply_choice
    if ! $NO_RESTART && systemctl is-active --quiet aurago 2>/dev/null; then
        if confirm "Restart AuraGo now so the Bluetooth settings take effect?"; then
            $SUDO systemctl restart aurago || warn "Restart failed. Run: sudo systemctl restart aurago"
        else
            info "Restart AuraGo later to apply the Bluetooth settings: sudo systemctl restart aurago"
        fi
    fi
    return 0
}

# Asked once; the answer lives in data/bluetooth-setup.
BT_STORED="$(btk_read_state "$DIR")"
_bt_may_prompt=false
if has_interactive_tty && ! $AUTO_YES; then _bt_may_prompt=true; fi
BT_CHOICE="$(btk_resolve_choice "$DIR" "$BT_FLAG" "$_bt_may_prompt")"

# Select the build path before any release request or service shutdown.
for _godir in /usr/local/go/bin "$HOME/go/bin" /usr/local/bin; do
    [ -d "$_godir" ] && [[ ":$PATH:" != *":$_godir:"* ]] && export PATH="$_godir:$PATH"
done
unset _godir
GO_FOUND=false
if command -v go >/dev/null 2>&1; then
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    GO_FOUND=true
fi
# Binary installations consume release pairs even when Go happens to be present.
if $BINARY_ONLY; then GO_FOUND=false; fi

# ── Check current vs available version ────────────────────────────────
section "Checking for updates"

# Version before the update, shown as "old ➜ new" in the final summary.
PRE_UPDATE_VERSION=""
if $BINARY_ONLY; then
    if [ -f "$DIR/.version" ]; then PRE_UPDATE_VERSION="$(tr -d '\r\n' < "$DIR/.version" 2>/dev/null || true)"; fi
else
    PRE_UPDATE_VERSION="$(git -C "$DIR" log --format='%h (%cd)' --date=short -1 2>/dev/null || true)"
fi

GIT_UP_TO_DATE=false   # set to true when local git is already at latest commit

if $BINARY_ONLY; then
    RELEASE_TAG=$(latest_release_tag || true)
    [ -z "$RELEASE_TAG" ] && die "Could not determine latest release tag from GitHub."
    info "Latest release available: $RELEASE_TAG"
    INSTALLED_RELEASE=""
    if [ -f "$DIR/.version" ]; then
        INSTALLED_RELEASE="$(tr -d '\r\n' < "$DIR/.version")"
    fi
    if [ "$INSTALLED_RELEASE" = "$RELEASE_TAG" ] && ! $REBUILD; then
        ok "AuraGo is already at ${RELEASE_TAG}; no files or services were changed."
        bluetooth_apply_without_update
        exit 0
    fi
    RELEASE_BASE="https://github.com/${GITHUB_REPO}/releases/download/${RELEASE_TAG}"
    fetch_release_checksums || die "Could not download SHA256SUMS for release ${RELEASE_TAG}."
    echo ""
    confirm "Proceed with update to $RELEASE_TAG?" || { info "Update cancelled."; exit 0; }
else
    if ! git_fetch_origin_main; then
        die "Failed to fetch updates from GitHub without interactive authentication. Verify network access and the origin URL, then retry."
    fi

    GIT_UP_TO_DATE=false

    if ! refresh_git_relationship; then
        die "Could not compare the local checkout with origin/main."
    fi

    if [ "$GIT_REMOTE_AHEAD" -eq 0 ]; then
        GIT_UP_TO_DATE=true
        if [ "$GIT_LOCAL_AHEAD" -eq 0 ]; then
            ok "Code is already at the latest version ($(git log --format='%h %s' -1))"
        else
            ok "Local checkout already contains origin/main and preserves ${GIT_LOCAL_AHEAD} local commit(s)."
        fi
        if ! $REBUILD; then
            ok "No rebuild requested; no files or services were changed."
            bluetooth_apply_without_update
            exit 0
        fi
    else
        info "Local:  $(git log --format='%h  %s  (%cd)' --date=short -1)"
        info "Remote: $(git log --format='%h  %s  (%cd)' --date=short -1 origin/main)"
        echo ""
        info "$GIT_REMOTE_AHEAD commit(s) available from origin/main."
        if [ "$GIT_LOCAL_AHEAD" -gt 0 ]; then
            info "$GIT_LOCAL_AHEAD local commit(s) will be preserved by an automatic merge."
        fi
        echo ""

        subsection "Changelog"
        git log HEAD..origin/main --format='%h%x09%s' --no-decorate -n 20 |
            while IFS=$'\t' read -r _cl_hash _cl_subject; do
                printf '  %s%s%s  %s\n' "$DIM" "$_cl_hash" "$NC" "$_cl_subject"
            done
        echo ""
    fi

    confirm "Proceed with update?" || { info "Update cancelled."; exit 0; }
fi

# Release preflight for source checkouts without Go. Binary installations have
# already pinned their tag and manifest above; source builds need neither.
if ! $GO_FOUND && ! $BINARY_ONLY; then
    RELEASE_TAG=$(latest_release_tag || true)
    [ -n "$RELEASE_TAG" ] || die "Could not determine latest release tag; no services were stopped."
    RELEASE_BASE="https://github.com/${GITHUB_REPO}/releases/download/${RELEASE_TAG}"
    fetch_release_checksums || die "Could not download SHA256SUMS for release ${RELEASE_TAG}; no services were stopped."
    info "Using verified release: $RELEASE_TAG"
    warn "Without Go, installed binaries follow this release. Newer Git-only changes require a source build."
fi

# Capture the pre-update readiness contract before stopping anything. Older
# binaries that do not expose the read-only healthcheck remain "unknown".
CURRENT_AURAGO_BIN="$DIR/bin/aurago_linux"
[ -x "$CURRENT_AURAGO_BIN" ] || CURRENT_AURAGO_BIN="$DIR/bin/aurago"
CORE_WAS_READY="unknown"
TSNET_WAS_READY="unknown"
TSNET_STATE_DIR=""

binary_supports_option() {
    local binary="$1"
    local option="$2"
    [ -x "$binary" ] || return 1
    "$binary" --help 2>&1 | grep -q -- "$option"
}

tsnet_failure_guidance() {
    local health_output="$1"
    case "$health_output" in
        *TSNET_TIMEOUT*)
            printf '%s\n' "The tsnet startup retry timed out. Check outbound network, DNS, system time, and 'journalctl -u aurago'; do not reauthenticate unless /api/tsnet/status reports a login or node-key error."
            ;;
        *TSNET_LOGIN_REQUIRED*|*TSNET_NODE_KEY_EXPIRED*|*TSNET_AUTH_KEY_MISSING*|*TSNET_AUTH_KEY_REJECTED*)
            printf '%s\n' "Review /api/tsnet/status and use node-specific reauthentication."
            ;;
        *TSNET_STATE_CORRUPT*)
            printf '%s\n' "The persisted tsnet state could not be loaded. Keep the updater backup and review /api/tsnet/status before replacing state."
            ;;
        *)
            printf '%s\n' "Review /api/tsnet/status and 'journalctl -u aurago' for the node-specific failure code."
            ;;
    esac
}

configured_tsnet_state_dir_fallback() {
    local config_file="$DIR/config.yaml"
    local value=""
    if [ -f "$config_file" ]; then
        value="$(awk '
            /^tailscale:[[:space:]]*($|#)/ { in_tailscale=1; in_tsnet=0; next }
            in_tailscale {
                raw=$0
                line=raw
                sub(/^[[:space:]]+/, "", line)
                indent=length(raw)-length(line)
                if (indent == 0 && line !~ /^($|#)/) {
                    in_tailscale=0
                    in_tsnet=0
                    next
                }
                if (!in_tsnet && line ~ /^tsnet:[[:space:]]*($|#)/) {
                    in_tsnet=1
                    tsnet_indent=indent
                    next
                }
                if (in_tsnet && indent <= tsnet_indent && line !~ /^($|#)/) {
                    in_tsnet=0
                }
            }
            in_tsnet && line ~ /^state_dir:[[:space:]]*/ {
                sub(/^state_dir:[[:space:]]*/, "", line)
                sub(/[[:space:]]*#.*/, "", line)
                gsub(/^[[:space:]]+|[[:space:]]+$/, "", line)
                if ((substr(line,1,1) == "\"" && substr(line,length(line),1) == "\"") ||
                    (substr(line,1,1) == "\047" && substr(line,length(line),1) == "\047")) {
                    line=substr(line,2,length(line)-2)
                }
                print line
                exit
            }
        ' "$config_file")"
    fi
    [ -n "$value" ] || value="$DIR/data/tsnet"
    case "$value" in
        /*) printf '%s\n' "$value" ;;
        *) printf '%s\n' "$DIR/$value" ;;
    esac
}

if binary_supports_option "$CURRENT_AURAGO_BIN" "healthcheck"; then
    if "$CURRENT_AURAGO_BIN" --config "$DIR/config.yaml" --healthcheck --healthcheck-timeout 5s >/dev/null 2>&1; then
        CORE_WAS_READY="ready"
        if "$CURRENT_AURAGO_BIN" --config "$DIR/config.yaml" --healthcheck --healthcheck-timeout 5s --healthcheck-require-tsnet >/dev/null 2>&1; then
            TSNET_WAS_READY="ready"
        else
            TSNET_WAS_READY="not_ready"
        fi
    else
        CORE_WAS_READY="not_ready"
        TSNET_WAS_READY="not_ready"
    fi
fi
if binary_supports_option "$CURRENT_AURAGO_BIN" "print-tsnet-state-dir"; then
    TSNET_STATE_DIR="$("$CURRENT_AURAGO_BIN" --config "$DIR/config.yaml" --print-tsnet-state-dir 2>/dev/null | tail -n 1 || true)"
fi
[ -n "$TSNET_STATE_DIR" ] || TSNET_STATE_DIR="$(configured_tsnet_state_dir_fallback)"
info "Pre-update readiness: core=${CORE_WAS_READY}, tsnet=${TSNET_WAS_READY}"

# ── Stop running instances BEFORE any file changes ────────────────────
[ ! -L "$DIR/assets" ] && [ ! -L "$DIR/assets/web" ] || die "Linked asset directories cannot be updated safely."
if binary_supports_option "$CURRENT_AURAGO_BIN" update-maintenance; then
    "$CURRENT_AURAGO_BIN" --update-maintenance --root "$DIR" --check-pending || die "Update state requires recovery before another update."
fi
UPDATE_PREVIOUS_ASSET="$(update_asset_id "$CURRENT_AURAGO_BIN")" || die "Cannot identify current web assets; no update files were changed."
"$CURRENT_AURAGO_BIN" --assets-dir "$DIR/assets/web" --check-assets || die "Current web assets failed validation."
UPDATE_PREVIOUS_VERSION="$(sha256sum "$CURRENT_AURAGO_BIN" | awk '{print $1}')"
# Budget the protected files plus staging/build overhead before stopping AuraGo.
_backup_kib=0
for _path in "${DATA_FILES[@]}" "${PROTECTED_DIRS[@]}"; do
    [ ! -e "$DIR/$_path" ] || _backup_kib=$((_backup_kib + $(du -sk "$DIR/$_path" | awk '{print $1}')))
done
if $BINARY_ONLY; then
    for _path in prompts agent_workspace ui; do
        [ ! -e "$DIR/$_path" ] || _backup_kib=$((_backup_kib + $(du -sk "$DIR/$_path" | awk '{print $1}')))
    done
    for _asset in "$DIR/assets/"*; do
        [ -e "$_asset" ] && [ "${_asset##*/}" != web ] || continue
        _backup_kib=$((_backup_kib + $(du -sk "$_asset" | awk '{print $1}')))
    done
fi
update_space_check "$((_backup_kib + 8388608))" || die "Free-space preflight failed before shutdown."
# This must happen early so the binary file is not locked and lock files
# are cleaned up before git-pull/build overwrites anything.
section "Stopping running instances"

_find_install_pids() {
    local patterns=("$@")
    local pids=""
    local pat pid exe cwd cmd expected

    # Only stop processes that belong to this AuraGo installation. Matching on
    # broad names such as "bin/aurago_linux" can otherwise kill unrelated test
    # instances in other directories.
    for pat in "${patterns[@]}"; do
        expected="$DIR/$pat"
        while IFS= read -r pid; do
            [ -n "$pid" ] || continue
            [ "$pid" = "$$" ] && continue

            exe="$(readlink -f "/proc/$pid/exe" 2>/dev/null || true)"
            cwd="$(readlink -f "/proc/$pid/cwd" 2>/dev/null || true)"
            cmd="$(tr '\0' ' ' < "/proc/$pid/cmdline" 2>/dev/null || true)"

            if [ "$exe" = "$expected" ] ||
               [[ "$cmd" == *"$expected"* ]] ||
               { [[ "$cwd" == "$DIR" || "$cwd" == "$DIR"/* ]] && [[ "$cmd" == *"$pat"* ]]; }; then
                case " $pids " in
                    *" $pid "*) ;;
                    *) pids="$pids $pid" ;;
                esac
            fi
        done < <(pgrep -f "$pat" 2>/dev/null || true)
    done
    printf '%s\n' "$pids"
}

_kill_proc() {
    local label="$1"; shift
    local pids
    pids="$(_find_install_pids "$@")"
    [ -n "${pids// /}" ] || { info "$label: not running"; return 0; }

    info "Stopping $label (SIGTERM)..."
    for pid in $pids; do
        kill -TERM "$pid" 2>/dev/null || true
    done

    # Wait up to 60 seconds for AuraGo's graceful shutdown contract.
    local waited=0
    while true; do
        local still_up=false
        for pid in $pids; do
            kill -0 "$pid" 2>/dev/null && { still_up=true; break; }
        done
        $still_up || break
        sleep 1; waited=$((waited + 1))
        [ $waited -ge 60 ] && break
    done

    # SIGKILL if still alive
    local killed=false
    for pid in $pids; do
        if kill -0 "$pid" 2>/dev/null; then
            warn "$label still alive after ${waited}s — sending SIGKILL"
            kill -KILL "$pid" 2>/dev/null || true
            killed=true
        fi
    done

    # Final wait after SIGKILL
    if $killed; then
        sleep 2
        for pid in $pids; do
            if kill -0 "$pid" 2>/dev/null; then
                warn "Could not kill $label process $pid — update may fail"
            fi
        done
        return 2
    fi

    ok "$label stopped"
    return 0
}

PRE_START_MODE="stopped"
if command -v systemctl >/dev/null 2>&1 && systemctl is-active --quiet aurago 2>/dev/null; then
    PRE_START_MODE="systemd"
elif [ -n "$(_find_install_pids "bin/aurago_linux" "bin/aurago" | tr -d '[:space:]')" ]; then
    PRE_START_MODE="direct"
fi
info "Pre-update start mode: ${PRE_START_MODE}"

restart_unchanged_after_failed_stop() {
    local launch_pid=""
    case "$PRE_START_MODE" in
        stopped)
            return 0
            ;;
        systemd)
            command -v systemctl >/dev/null 2>&1 || return 1
            $SUDO systemctl start aurago >/dev/null 2>&1 || return 1
            local waited=0
            while ! systemctl is-active --quiet aurago 2>/dev/null && [ "$waited" -lt 20 ]; do
                sleep 1
                waited=$((waited + 1))
            done
            systemctl is-active --quiet aurago 2>/dev/null || return 1
            ;;
        direct)
            [ -x "$CURRENT_AURAGO_BIN" ] || return 1
            mkdir -p "$DIR/log"
            if [ -z "${AURAGO_MASTER_KEY:-}" ] && [ -f "$DIR/.env" ]; then
                AURAGO_MASTER_KEY="$(read_master_key_from_env "$DIR/.env")"
                export AURAGO_MASTER_KEY
            fi
            nohup env -u AURAGO_UPDATE_LOCK_FD "$CURRENT_AURAGO_BIN" --config "$DIR/config.yaml" 9>&- >>"$DIR/log/aurago.log" 2>&1 &
            launch_pid=$!
            sleep 3
            kill -0 "$launch_pid" 2>/dev/null || return 1
            ;;
        *)
            return 1
            ;;
    esac

    if [ "$CORE_WAS_READY" = "ready" ] && binary_supports_option "$CURRENT_AURAGO_BIN" "healthcheck"; then
        "$CURRENT_AURAGO_BIN" --config "$DIR/config.yaml" --healthcheck --healthcheck-timeout 60s >/dev/null 2>&1 || return 1
    elif [ "$PRE_START_MODE" = "systemd" ]; then
        systemctl is-active --quiet aurago 2>/dev/null || return 1
    elif [ "$PRE_START_MODE" = "direct" ]; then
        kill -0 "$launch_pid" 2>/dev/null || return 1
    fi
    return 0
}

abort_before_file_changes() {
    local reason="$1"
    if restart_unchanged_after_failed_stop; then
        if [ "$PRE_START_MODE" = "stopped" ]; then
            die "${reason} AuraGo was not running before the update; no restart was needed and no update files were touched."
        fi
        die "${reason} The unchanged installation was restarted and verified; no update files were touched."
    fi
    die "${reason} AuraGo could not be restarted automatically and is currently stopped. Start it manually with 'sudo systemctl start aurago' or './start.sh'. No update files were touched."
}

# Stop systemd first. An authorization failure is not bypassed with a manual
# kill because Restart=always/on-failure could race the updater.
SYSTEMD_STOP_FORCED=false
if command -v systemctl >/dev/null 2>&1 && systemctl is-active --quiet aurago 2>/dev/null; then
    info "Stopping aurago systemd service..."
    if command -v timeout >/dev/null 2>&1; then
        # GNU timeout normally places the command in a separate process
        # group.  sudo then receives SIGTTIN while reading its password from
        # the controlling terminal and appears to hang.  --foreground keeps
        # the command in the terminal's foreground process group.
        if timeout --help 2>&1 | grep -q -- '--foreground'; then
            if ! timeout --foreground 60s $SUDO systemctl stop aurago; then
                abort_before_file_changes "systemd did not stop AuraGo cleanly within 60 seconds."
            fi
        elif has_interactive_tty && [ "$SUDO" = "sudo" ]; then
            # BusyBox timeout has no --foreground.  Authenticate before
            # entering its process group, then run the timed command without
            # another terminal read.
            if ! $SUDO -v || ! timeout 60s sudo -n systemctl stop aurago; then
                abort_before_file_changes "systemd did not stop AuraGo cleanly within 60 seconds."
            fi
        elif ! timeout 60s $SUDO systemctl stop aurago; then
            abort_before_file_changes "systemd did not stop AuraGo cleanly within 60 seconds."
        fi
    elif ! $SUDO systemctl stop aurago; then
        abort_before_file_changes "systemd could not stop AuraGo."
    fi
    if systemctl is-active --quiet aurago 2>/dev/null; then
        abort_before_file_changes "AuraGo is still active after systemd stop."
    fi
    _systemd_result="$(systemctl show aurago.service -p Result --value 2>/dev/null || true)"
    if [ "$_systemd_result" = "timeout" ]; then
        SYSTEMD_STOP_FORCED=true
    fi
    ok "systemd service stopped"
fi
# Always kill any remaining instances (covers manual starts, systemd restarts, etc.)
_stop_rc=0
_kill_proc "aurago" "bin/aurago_linux" "bin/aurago" || _stop_rc=$?
if [ "$_stop_rc" -ne 0 ]; then
    if [ "$_stop_rc" -eq 2 ]; then
        abort_before_file_changes "AuraGo required SIGKILL after the 60-second shutdown deadline."
    fi
    abort_before_file_changes "AuraGo could not be stopped safely."
fi
if $SYSTEMD_STOP_FORCED; then
    abort_before_file_changes "systemd reported a forced timeout stop."
fi

# ── Remove lock files left by killed processes ─────────────────────────
info "Removing stale lock files..."
for lockfile in \
    "$DIR/data/aurago.lock" \
    "$DIR/data/maintenance.lock" \
    "$DIR/.git/index.lock"
do
    if [ -f "$lockfile" ]; then
        if remove_regular_file_if_present "$lockfile"; then
            ok "Removed: $(basename "$lockfile")"
        fi
    fi
done

ok "AuraGo-owned instances stopped"

# ── Backup protected user data ─────────────────────────────────────────
section "Backing up user data"
BACKUP_DIR="$(mktemp -d "$UPDATE_STATE/transactions/txn-XXXXXX")"
UPDATE_CREATED="$(date +%s%3N)"
UPDATE_BACKUP_COMPLETE=false
UPDATE_NEW_ASSET=""
update_manifest pending
info "Backup location: $BACKUP_DIR"
SYSTEMD_DROPIN_DIR="/etc/systemd/system/aurago.service.d"
SYSTEMD_STOP_TIMEOUT_DROPIN="${SYSTEMD_DROPIN_DIR}/20-aurago-stop-timeout.conf"
SYSTEMD_DROPIN_BACKUP="${BACKUP_DIR}/20-aurago-stop-timeout.conf"
SYSTEMD_DROPIN_EXISTED=false
SYSTEMD_DROPIN_CHANGED=false

if [ -L "$SYSTEMD_DROPIN_DIR" ] || [ -L "$SYSTEMD_STOP_TIMEOUT_DROPIN" ]; then
    abort_before_file_changes "Refusing to update an unsafe symlinked systemd drop-in path."
fi
if [ -f "$SYSTEMD_STOP_TIMEOUT_DROPIN" ]; then
    if cp -p "$SYSTEMD_STOP_TIMEOUT_DROPIN" "$SYSTEMD_DROPIN_BACKUP" 2>/dev/null || \
       $SUDO cp -p "$SYSTEMD_STOP_TIMEOUT_DROPIN" "$SYSTEMD_DROPIN_BACKUP"; then
        SYSTEMD_DROPIN_EXISTED=true
        ok "Backed up the systemd stop-timeout drop-in."
    else
        abort_before_file_changes "Could not back up the existing systemd stop-timeout drop-in."
    fi
fi

validate_tsnet_state_dir() {
    local path="${1%/}"
    [ -n "$path" ] || return 1
    case "$path" in
        /*) ;;
        *) path="$DIR/$path" ;;
    esac
    local current="/"
    local part
    local parts=()
    IFS='/' read -r -a parts <<< "${path#/}"
    for part in "${parts[@]}"; do
        [ -n "$part" ] || continue
        current="${current%/}/$part"
        [ ! -L "$current" ] || return 1
    done
    [ -d "$path" ] || return 1
    local resolved
    resolved="$(cd "$path" 2>/dev/null && pwd -P)" || return 1
    case "$resolved" in
        "/"|"${HOME%/}"|"${DIR%/}") return 1 ;;
    esac
    printf '%s\n' "$resolved"
}

backup_tsnet_state() {
    local resolved candidate
    candidate="${TSNET_STATE_DIR%/}"
    case "$candidate" in
        /*) ;;
        *) candidate="$DIR/$candidate" ;;
    esac
    if [ ! -e "$candidate" ]; then
        if [ "$TSNET_WAS_READY" = "ready" ]; then
            abort_before_file_changes "The previously working tsnet state directory disappeared before backup."
        fi
        warn "No persisted tsnet state directory exists yet; there is nothing to back up."
        return 0
    fi
    resolved="$(validate_tsnet_state_dir "$TSNET_STATE_DIR" || true)"
    if [ -z "$resolved" ]; then
        abort_before_file_changes "Refusing to update because the configured tsnet state path is unsafe."
    fi
    printf '%s\n' "$resolved" > "$BACKUP_DIR/tsnet-state.path"
    if cp -a -- "$resolved" "$BACKUP_DIR/tsnet-state" 2>/dev/null || $SUDO cp -a -- "$resolved" "$BACKUP_DIR/tsnet-state"; then
        ok "Backed up tsnet state with ownership and mode."
        return 0
    fi
    abort_before_file_changes "Could not back up the configured tsnet state directory safely."
}

restore_tsnet_state_backup() {
    [ -d "$BACKUP_DIR/tsnet-state" ] || return 0
    [ -f "$BACKUP_DIR/tsnet-state.path" ] || return 1
    local original resolved failed_copy
    original="$(head -n 1 "$BACKUP_DIR/tsnet-state.path")"
    resolved="$(validate_tsnet_state_dir "$original" || true)"
    if [ -z "$resolved" ]; then
        # The failed runtime may have removed the directory. Validate its parent
        # and original lexical target before recreating it.
        [ ! -L "$original" ] || return 1
        local original_parent resolved_parent
        original_parent="$(dirname "$original")"
        resolved_parent="$(cd "$original_parent" 2>/dev/null && pwd -P)" || return 1
        [ "$resolved_parent" = "$original_parent" ] || return 1
        resolved="$resolved_parent/$(basename "$original")"
    fi
    case "${resolved%/}" in
        ""|"/"|"${HOME%/}"|"${DIR%/}") return 1 ;;
    esac
    failed_copy="$BACKUP_DIR/tsnet-state.failed"
    if [ -e "$resolved" ]; then
        mv -- "$resolved" "$failed_copy" 2>/dev/null || $SUDO mv -- "$resolved" "$failed_copy" || return 1
    fi
    mkdir -p "$(dirname "$resolved")" 2>/dev/null || $SUDO mkdir -p "$(dirname "$resolved")"
    if cp -a -- "$BACKUP_DIR/tsnet-state" "$resolved" 2>/dev/null || $SUDO cp -a -- "$BACKUP_DIR/tsnet-state" "$resolved"; then
        ok "Restored pre-update tsnet state."
        return 0
    fi
    if [ -e "$failed_copy" ]; then
        mv -- "$failed_copy" "$resolved" 2>/dev/null || $SUDO mv -- "$failed_copy" "$resolved" || true
    fi
    return 1
}

backup_current_aurago_binary() {
    mkdir -p "$BACKUP_DIR/bin"
    for _bin in aurago_linux aurago; do
        if [ -f "$DIR/bin/$_bin" ]; then
            cp -p "$DIR/bin/$_bin" "$BACKUP_DIR/bin/$_bin"
            ok "Backed up binary: bin/$_bin"
        fi
    done
}

restore_previous_aurago_binary() {
    local restored=false
    for _bin in aurago_linux aurago; do
        if [ -f "$BACKUP_DIR/bin/$_bin" ]; then
            cp -p "$BACKUP_DIR/bin/$_bin" "$DIR/bin/$_bin"
            mark_executable_if_present "$DIR/bin/$_bin"
            restored=true
        fi
    done
    if $restored; then
        apply_aurago_setcap_if_available
        warn "Restored previous AuraGo binary after failed restart."
        return 0
    fi
    warn "No previous AuraGo binary was available for rollback."
    return 1
}

restore_critical_user_data_after_failure() {
    for f in "${PROTECTED_FILES[@]}"; do
        local bak="$BACKUP_DIR/$(basename "$f")"
        [ -f "$bak" ] || continue
        safe_restore_file "$bak" "$DIR/$f" || return 1
    done
    if [ -d "$BACKUP_DIR/data" ]; then
        mkdir -p "$DIR/data"
        for f in "${DATA_FILES[@]}"; do
            local bak="$BACKUP_DIR/data/$(basename "$f")"
            [ -f "$bak" ] || continue
            safe_restore_file "$bak" "$DIR/$f" || return 1
        done
    fi
}

backup_binary_update_resources() {
    $BINARY_ONLY || return 0
    BINARY_RESOURCE_BACKUP_DIR="$BACKUP_DIR/binary_update_resources"
    mkdir -p "$BINARY_RESOURCE_BACKUP_DIR"
    : > "$BINARY_RESOURCE_BACKUP_DIR/missing.txt"
    for rel in prompts agent_workspace ui update.sh config.yaml.new_template; do
        if [ -e "$DIR/$rel" ]; then
            mkdir -p "$BINARY_RESOURCE_BACKUP_DIR/$(dirname "$rel")"
            cp -a "$DIR/$rel" "$BINARY_RESOURCE_BACKUP_DIR/$rel" 2>/dev/null || \
                copy_tree_merge "$DIR/$rel/" "$BINARY_RESOURCE_BACKUP_DIR/$rel/" || \
                abort_before_file_changes "Could not fully back up $rel for binary rollback."
        else
            printf '%s\n' "$rel" >> "$BINARY_RESOURCE_BACKUP_DIR/missing.txt"
        fi
    done
    # Immutable web sets are shared by ID, never recursively copied per update.
    mkdir -p "$BINARY_RESOURCE_BACKUP_DIR/assets"
    for asset in "$DIR/assets/"*; do
        [ -e "$asset" ] || continue
        [ "${asset##*/}" != web ] || continue
        cp -a -- "$asset" "$BINARY_RESOURCE_BACKUP_DIR/assets/" || abort_before_file_changes "Asset backup failed."
    done
}

restore_binary_update_resources_after_failure() {
    $BINARY_ONLY || return 0
    [ -n "${BINARY_RESOURCE_BACKUP_DIR:-}" ] && [ -d "$BINARY_RESOURCE_BACKUP_DIR" ] || return 0
    for rel in prompts agent_workspace ui update.sh config.yaml.new_template; do
        if grep -Fxq "$rel" "$BINARY_RESOURCE_BACKUP_DIR/missing.txt" 2>/dev/null; then
            rm -rf "$DIR/$rel"
            continue
        fi
        [ -e "$BINARY_RESOURCE_BACKUP_DIR/$rel" ] || continue
        rm -rf "$DIR/$rel"
        mkdir -p "$DIR/$(dirname "$rel")"
        cp -a "$BINARY_RESOURCE_BACKUP_DIR/$rel" "$DIR/$rel" 2>/dev/null || \
            copy_tree_merge "$BINARY_RESOURCE_BACKUP_DIR/$rel/" "$DIR/$rel/" || \
            return 1
    done
    # Restore non-web resources without replacing the versioned web root.
    for asset in "$DIR/assets/"*; do
        [ -e "$asset" ] || continue
        [ "${asset##*/}" != web ] || continue
        rm -rf -- "$asset"
    done
    copy_tree_merge "$BINARY_RESOURCE_BACKUP_DIR/assets/" "$DIR/assets/" || return 1
}

restart_previous_after_rollback() {
    $NO_RESTART && return 0
    CURRENT_AURAGO_BIN="$DIR/bin/aurago_linux"
    [ -x "$CURRENT_AURAGO_BIN" ] || CURRENT_AURAGO_BIN="$DIR/bin/aurago"
    if restart_unchanged_after_failed_stop; then
        if [ "$PRE_START_MODE" = "stopped" ]; then
            info "AuraGo was stopped before the update; rollback left it stopped."
        else
            ok "Previous AuraGo start mode restored and verified after rollback."
        fi
        return 0
    fi
    warn "The previous AuraGo installation was restored but could not be restarted automatically."
    warn "Start it manually with 'sudo systemctl start aurago' or './start.sh'."
    return 1
}

restore_service_stop_timeout_dropin() {
    $SYSTEMD_DROPIN_CHANGED || return 0
    if $SYSTEMD_DROPIN_EXISTED; then
        [ -f "$SYSTEMD_DROPIN_BACKUP" ] || return 1
        $SUDO mkdir -p "$SYSTEMD_DROPIN_DIR" || return 1
        $SUDO install -o root -g root -m 0644 "$SYSTEMD_DROPIN_BACKUP" "$SYSTEMD_STOP_TIMEOUT_DROPIN" || return 1
    else
        $SUDO rm -f -- "$SYSTEMD_STOP_TIMEOUT_DROPIN" || return 1
        $SUDO rmdir "$SYSTEMD_DROPIN_DIR" >/dev/null 2>&1 || true
    fi
    $SUDO systemctl daemon-reload >/dev/null 2>&1 || return 1
    SYSTEMD_DROPIN_CHANGED=false
    return 0
}

abort_update() {
    local msg="$1"
    local rollback_ok=true
    warn "Update failed after shutdown; rolling back to the previous working state."
    if command -v systemctl >/dev/null 2>&1; then
        timeout 60s $SUDO systemctl stop aurago >/dev/null 2>&1 || true
    fi
    _kill_proc "updated AuraGo" "bin/aurago_linux" "bin/aurago" >/dev/null 2>&1 || true
    restore_previous_aurago_binary || rollback_ok=false
    if [ -n "${PRE_UPDATE_REF:-}" ] && [ -d "$DIR/.git" ]; then
        git -C "$DIR" reset --hard "$PRE_UPDATE_REF" >/dev/null 2>&1 || rollback_ok=false
    fi
    restore_untracked_merge_collisions_after_failure || rollback_ok=false
    restore_binary_update_resources_after_failure || rollback_ok=false
    restore_critical_user_data_after_failure || rollback_ok=false
    restore_tsnet_state_backup || rollback_ok=false
    restore_service_stop_timeout_dropin || rollback_ok=false
    if ! restart_previous_after_rollback; then
        update_manifest uncertain
        die "${msg} The previous files were restored, but AuraGo is stopped; start it manually with 'sudo systemctl start aurago' or './start.sh'."
    fi
    if $rollback_ok && [ "$TSNET_WAS_READY" = ready ]; then
        "$CURRENT_AURAGO_BIN" --config "$DIR/config.yaml" --healthcheck --healthcheck-timeout 30s --healthcheck-require-tsnet || rollback_ok=false
    fi
    if $rollback_ok && ! $NO_RESTART && "$CURRENT_AURAGO_BIN" --config "$DIR/config.yaml" --healthcheck --healthcheck-timeout 30s; then
        update_manifest rolled_back
        if binary_supports_option "$CURRENT_AURAGO_BIN" update-maintenance; then update_retention_cleanup "$CURRENT_AURAGO_BIN"; fi
    fi
    die "$msg"
}

backup_current_aurago_binary
backup_tsnet_state

for f in "${PROTECTED_FILES[@]}"; do
    if [ -f "$DIR/$f" ]; then
        cp -p "$DIR/$f" "$BACKUP_DIR/$(basename "$f")"
        ok "Backed up: $f"
    fi
done

# If config.yaml is missing (e.g. re-execution after git deleted it during a
# tracked→untracked transition), recover it from the most recent prior backup.
if [ ! -f "$BACKUP_DIR/config.yaml" ]; then
    _prev_cfg=$(find /tmp -maxdepth 2 -name "config.yaml" \
        -path "*/aurago-backup-*" ! -path "$BACKUP_DIR/*" \
        2>/dev/null | xargs ls -t 2>/dev/null | head -1)
    if [ -n "$_prev_cfg" ]; then
        cp -p "$_prev_cfg" "$BACKUP_DIR/config.yaml"
        ok "Recovered config.yaml from previous backup (re-execution safety net)."
    fi
fi

# Back up individual critical data files
mkdir -p "$BACKUP_DIR/data"
for f in "${DATA_FILES[@]}"; do
    if [ -f "$DIR/$f" ]; then
        if ! safe_restore_file "$DIR/$f" "$BACKUP_DIR/data/$(basename "$f")"; then
            abort_before_file_changes "Could not back up $f; update aborted before changing files."
        fi
    fi
done
ok "Backed up: data/ (critical files)"

for d in "${PROTECTED_DIRS[@]}"; do
    if [ -d "$DIR/$d" ]; then
        local_name="${d//\//__}"      # replace / with __ for flat backup name
        copy_tree_merge "$DIR/$d/" "$BACKUP_DIR/$local_name/" || abort_before_file_changes "Could not fully back up $d/."
        ok "Backed up: $d/"
    fi
done

# Backup custom prompt files
if [ -d "$PROMPTS_DIR" ]; then
    CUSTOM_PROMPTS="$BACKUP_DIR/prompts__custom"
    mkdir -p "$CUSTOM_PROMPTS"
    if $BINARY_ONLY; then
        # Binary install: back up all prompt files (they are always overwritten by update)
        copy_tree_merge "$PROMPTS_DIR/" "$CUSTOM_PROMPTS/" || warn "Could not fully back up prompts/."
        CUSTOM_COUNT=$(find "$PROMPTS_DIR" -type f | wc -l)
    else
        # Git install: back up only untracked/locally modified files
        CUSTOM_COUNT=0
        while IFS= read -r -d '' fp; do
            if [ ! -f "$DIR/$fp" ]; then
                warn "Skipping missing prompt file during backup: $fp"
                continue
            fi
            rel="${fp#prompts/}"
            dest_dir="$CUSTOM_PROMPTS/$(dirname "$rel")"
            mkdir -p "$dest_dir"
            if cp -p "$DIR/$fp" "$dest_dir/"; then
                CUSTOM_COUNT=$((CUSTOM_COUNT + 1))
            else
                abort_before_file_changes "Could not back up prompt file: $fp"
            fi
        done < <(git -C "$DIR" ls-files -z --others --modified -- "prompts/")
    fi
    ok "Backed up $CUSTOM_COUNT prompt file(s)"
fi

backup_binary_update_resources
UPDATE_BACKUP_COMPLETE=true
update_manifest pending

STAGED_RELEASE_DIR=""
REQUIRED_BINS=()
OPTIONAL_BINS=()
if $BINARY_ONLY && ! $GO_FOUND; then
    select_release_bins_for_arch
    STAGED_RELEASE_DIR="$(mktemp -d "$UPDATE_WORK/release.XXXXXX")"
    for BIN_NAME in "${REQUIRED_BINS[@]}"; do
        info "Staging required $BIN_NAME from GitHub Releases..."
        if _download_release_bin "$BIN_NAME" "$STAGED_RELEASE_DIR/$BIN_NAME"; then
            ok "$BIN_NAME downloaded and verified."
        else
            abort_update "Required release artifact $BIN_NAME could not be downloaded or verified."
        fi
    done
    for BIN_NAME in "${OPTIONAL_BINS[@]}"; do
        info "Staging optional $BIN_NAME from GitHub Releases..."
        if _download_release_bin "$BIN_NAME" "$STAGED_RELEASE_DIR/$BIN_NAME"; then
            ok "$BIN_NAME downloaded and verified."
        else
            warn "$BIN_NAME download failed; continuing without optional remote client artifact."
        fi
    done
fi

# ── Apply update ───────────────────────────────────────────────────────
if $BINARY_ONLY; then
    # Binary-only: download resources.dat and extract
    info "Downloading resources.dat ..."
    TMPRES=$(mktemp "$UPDATE_WORK/resources.XXXXXX")
    if ! download_release_asset "resources.dat" "$TMPRES"; then
        abort_update "Failed to download or verify resources.dat from the release."
    fi
    TMPEXT=$(mktemp -d "$UPDATE_WORK/resources.XXXXXX")
    tar -xzf "$TMPRES" -C "$TMPEXT"
    rm -f "$TMPRES"

    # Always overwrite code assets (prompts, ui, agent_workspace).
    # Use -r (no -p) so we don't try to preserve timestamps/ownership from the
    # tar archive — non-root users cannot change timestamps on files they don't
    # own, which produces spurious "Operation not permitted" warnings with cp -a.
    [ -d "$TMPEXT/prompts" ]           && cp -r "$TMPEXT/prompts"           "$DIR/"
    [ -d "$TMPEXT/agent_workspace" ]   && cp -r "$TMPEXT/agent_workspace"   "$DIR/"
    mkdir -p "$DIR/assets"
    for asset_dir in "$TMPEXT/assets/"*; do
        [ -d "$asset_dir" ] || continue
        [ "$(basename "$asset_dir")" = web ] || cp -r "$asset_dir" "$DIR/assets/"
    done
    [ -d "$TMPEXT/ui" ]                && cp -r "$TMPEXT/ui"                "$DIR/" 2>/dev/null || true

    # Treat the extracted config.yaml as the new template for the merger below
    if [ -f "$TMPEXT/config.yaml" ]; then
        cp "$TMPEXT/config.yaml" "$DIR/config.yaml.new_template"
    fi

    if download_release_asset "update.sh" "$DIR/update.sh"; then
        chmod +x "$DIR/update.sh"
        ok "update.sh refreshed"
    else
        warn "Could not refresh update.sh from verified release asset."
    fi

    ASSET_BIN="$STAGED_RELEASE_DIR/aurago_linux"
    [ "$GOARCH" != "arm64" ] || ASSET_BIN="$STAGED_RELEASE_DIR/aurago_linux_arm64"
    chmod +x "$ASSET_BIN"
    UPDATE_NEW_ASSET="$(update_asset_id "$ASSET_BIN")" || abort_update "Cannot identify staged assets."
    update_manifest pending
    if [ -d "$TMPEXT/assets/web" ]; then
        "$ASSET_BIN" --assets-dir "$DIR/assets/web" --import-assets-dir "$TMPEXT/assets/web" || abort_update "Failed to import matching web resources."
    fi
    rm -rf "$TMPEXT"
    ok "Resources updated from release $RELEASE_TAG"
else
    # Git-based update.
    if ! $GIT_UP_TO_DATE; then
        PRE_UPDATE_REF="$(git -C "$DIR" rev-parse HEAD 2>/dev/null || true)"
        if ! prepare_untracked_merge_collisions; then
            abort_update "Update aborted because an untracked file conflicts with incoming repository content."
        fi
        if ! git diff --quiet || ! git diff --cached --quiet; then
            info "Cleaning local tracked changes before update..."
            if ! clean_tracked_changes; then
                warn "Automatic cleanup of tracked changes failed."
                warn "Changed files still present:"
                git -C "$DIR" status --porcelain --untracked-files=no | head -20 || true
                abort_update "Cannot continue update while tracked files are locked/unwritable. Fix permissions or run with sudo."
            fi
        fi

        # Integrate the origin/main state already fetched and shown in the
        # changelog. Refetching after shutdown can fail independently and
        # must not leave a reviewed update dependent on a second network call.
        if ! integrate_origin_main; then
            case "$GIT_INTEGRATION_ERROR" in
                merge_conflict)
                    warn "Automatic merge found real file conflicts; the pre-update checkout was restored."
                    abort_update "Update aborted safely because conflicting local and remote edits require review."
                    ;;
                unrelated_histories)
                    abort_update "Update aborted safely because the local checkout and origin/main have no common ancestor."
                    ;;
                *)
                    warn "Repository integration failed with code: ${GIT_INTEGRATION_ERROR:-unknown}."
                    abort_update "Update aborted safely; local commits were not discarded."
                    ;;
            esac
        fi
        ok "Code updated to $(git log --format='%h  %s' -1)"
        GIT_VER=$(git describe --tags --always 2>/dev/null || git rev-parse --short HEAD 2>/dev/null || echo 'git')
    fi

    # Restore user's config.yaml — git must never win over user's config.
    if [ -f "$BACKUP_DIR/config.yaml" ]; then
        safe_restore_file "$BACKUP_DIR/config.yaml" "$DIR/config.yaml" \
            || abort_update "Could not restore config.yaml (permission denied)."
    fi
fi

# ── Migrate old prompts location (agent_workspace/prompts → prompts/) ─
# In binary-only mode the custom prompt backup covers all files; re-apply
# it now so user customisations are not wiped by the resources.dat extract.
if $BINARY_ONLY && [ -d "$BACKUP_DIR/prompts__custom" ] && [ "$(ls -A "$BACKUP_DIR/prompts__custom")" ]; then
    copy_tree_merge "$BACKUP_DIR/prompts__custom/" "$DIR/prompts/" || warn "Could not fully restore custom prompt files."
    ok "Custom prompt files restored"
fi

OLD_PROMPTS="$DIR/agent_workspace/prompts"
if [ -d "$OLD_PROMPTS" ]; then
    subsection "Migrating prompts directory"
    info "Old location detected: agent_workspace/prompts/ — migrating custom files ..."
    # Copy any files that don't yet exist at the new location (don't overwrite)
    if command -v rsync >/dev/null 2>&1; then
        rsync -rl --quiet --no-owner --no-group --ignore-existing "$OLD_PROMPTS/" "$DIR/prompts/" || warn "Could not fully migrate old prompts directory."
    else
        find "$OLD_PROMPTS" -type f | while read -r f; do
            rel="${f#$OLD_PROMPTS/}"
            dest="$DIR/prompts/$rel"
            if [ ! -f "$dest" ]; then
                mkdir -p "$(dirname "$dest")"
                cp -p "$f" "$dest"
            fi
        done
    fi
    rm -rf "$OLD_PROMPTS"
    ok "Migrated and removed agent_workspace/prompts/"
fi

# No stash to re-apply — we used git checkout instead.
# User data (config.yaml, custom prompts) is restored from backup below.

# ── Restore user data ──────────────────────────────────────────────────
section "Restoring user data"

for f in "${PROTECTED_FILES[@]}"; do
    bak="$BACKUP_DIR/$(basename "$f")"
    if [ -f "$bak" ]; then
        if [ "$f" = "config.yaml" ]; then
            # config.yaml is already restored after git operations.
            # The merger below will handle it. Skip here.
            continue
        fi
        if safe_restore_file "$bak" "$DIR/$f"; then
            ok "Restored: $f"
        else
            warn "Could not restore $f (permission denied)."
        fi
    fi
done

for d in "${PROTECTED_DIRS[@]}"; do
    local_name="${d//\//__}"
    bak="$BACKUP_DIR/$local_name"
    if [ -d "$bak" ]; then
        # Use rsync if available for smart merge; fall back to cp
        copy_tree_merge "$bak/" "$DIR/$d/" || warn "Could not fully restore $d/."
        ok "Restored: $d/"
    fi
done

# Restore critical data files (these are gitignored so git can't touch them,
# but restore from backup for completeness in case of any edge case)
if [ -d "$BACKUP_DIR/data" ]; then
    mkdir -p "$DIR/data"
    for f in "${DATA_FILES[@]}"; do
        bak="$BACKUP_DIR/data/$(basename "$f")"
        if [ -f "$bak" ]; then
            safe_restore_file "$bak" "$DIR/$f" || warn "Could not restore $f (permission denied)."
        fi
    done
fi

# Restore custom prompt files
CUSTOM_PROMPTS="$BACKUP_DIR/prompts__custom"
if [ -d "$CUSTOM_PROMPTS" ] && [ "$(ls -A "$CUSTOM_PROMPTS")" ]; then
    copy_tree_merge "$CUSTOM_PROMPTS/" "$PROMPTS_DIR/" || warn "Could not fully restore custom prompt files."
    ok "Restored custom prompt files"
fi

ok "All user data preserved."

# ── Offer to migrate .env → /etc/aurago/master.key ─────────────────────
# If .env is still in the install directory, offer to move the key to a
# root-owned credential file outside the application directory.
# This is the same mechanism used by install.sh for new systemd installs.
ENV_FILE="$DIR/.env"
CREDENTIAL_DIR="/etc/aurago"
CREDENTIAL_FILE="${CREDENTIAL_DIR}/master.key"

if [ -f "$ENV_FILE" ] && grep -q "AURAGO_MASTER_KEY" "$ENV_FILE"; then
    # Only offer if not already migrated
    if [ -f "$CREDENTIAL_FILE" ] && grep -q "AURAGO_MASTER_KEY" "$CREDENTIAL_FILE"; then
        info "Master key already exists at $CREDENTIAL_FILE."
        info "Removing leftover $ENV_FILE ..."
        rm -f "$ENV_FILE"
        ok "Removed $ENV_FILE (key is in $CREDENTIAL_FILE)."
    else
        echo ""
        tui_box warn "SECURITY RECOMMENDATION" \
            "Your vault master key is stored in ${BOLD}.env${NC} inside the AuraGo" \
            "directory. This file is readable by your user account." \
            "" \
            "It is ${BOLD}strongly recommended${NC} to move it to a root-protected" \
            "location at ${BOLD}/etc/aurago/master.key${NC} (mode 0600, root:root)." \
            "systemd will inject it automatically — no manual sourcing."
        echo ""

        if confirm "Move master key to /etc/aurago/master.key? (strongly recommended)"; then
            AURAGO_MASTER_KEY="$(read_master_key_from_env "$ENV_FILE")"
            if [ -z "${AURAGO_MASTER_KEY:-}" ]; then
                warn "Could not read AURAGO_MASTER_KEY from .env — skipping migration."
            else
                $SUDO mkdir -p "$CREDENTIAL_DIR"
                $SUDO chmod 700 "$CREDENTIAL_DIR"
                printf "AURAGO_MASTER_KEY=%s\n" "$AURAGO_MASTER_KEY" | $SUDO tee "$CREDENTIAL_FILE" > /dev/null
                $SUDO chmod 600 "$CREDENTIAL_FILE"
                $SUDO chown root:root "$CREDENTIAL_DIR" "$CREDENTIAL_FILE"
                rm -f "$ENV_FILE"
                ok "Master key moved to $CREDENTIAL_FILE (root-only, mode 0600)."
                ok "Removed $ENV_FILE."

                # Update systemd unit if it exists and still references .env
                SVC_FILE="/etc/systemd/system/aurago.service"
                if [ -f "$SVC_FILE" ]; then
                    if grep -q "EnvironmentFile=.*\.env" "$SVC_FILE" || grep -q "Environment=.*AURAGO_MASTER_KEY" "$SVC_FILE"; then
                        info "Updating systemd unit to use $CREDENTIAL_FILE ..."
                        # Replace EnvironmentFile pointing to .env
                        $SUDO sed -i "s|EnvironmentFile=.*\.env|EnvironmentFile=${CREDENTIAL_FILE}|g" "$SVC_FILE"
                        # Replace inline Environment= with EnvironmentFile=
                        $SUDO sed -i "s|Environment=\"AURAGO_MASTER_KEY=.*\"|EnvironmentFile=${CREDENTIAL_FILE}|g" "$SVC_FILE"
                        # Remove dash prefix (fail-silent) if present
                        $SUDO sed -i "s|EnvironmentFile=-|EnvironmentFile=|g" "$SVC_FILE"
                        # Add security hardening if not already present
                        if ! grep -q "NoNewPrivileges" "$SVC_FILE"; then
                            $SUDO sed -i "/^\[Install\]/i\\
# Security hardening\\
NoNewPrivileges=true\\
ProtectSystem=strict\\
ReadWritePaths=${DIR} ${CREDENTIAL_DIR}\\
ProtectHome=read-only\\
PrivateTmp=true" "$SVC_FILE"
                        fi
                        $SUDO systemctl daemon-reload
                        ok "systemd unit updated and reloaded."
                    fi
                fi

                echo ""
                tui_box ok "MASTER KEY SECURED" \
                    "Location: ${BOLD}/etc/aurago/master.key${NC} (root-only, mode 0600)" \
                    "The key is injected into AuraGo via systemd." \
                    "${YELLOW}Back up this file! Losing it = losing your vault.${NC}"
            fi
        else
            warn "Keeping .env in place. You can migrate later by re-running this update."
        fi
    fi
fi

# ── Merge config.yaml ──────────────────────────────────────────────────
section "Merging configuration"

# Source:   backup of config.yaml taken before any git/file operations.
# Template: config_template.yaml in the repo (the authoritative template).
#           Binary-only mode: newly extracted config.yaml.new_template.
#           Fallback: git show HEAD:config.yaml.
# Output:   $DIR/config.yaml  (always the final result).
#
# If config.yaml didn't exist before (fresh install): copy template directly.

USER_CONFIG_BAK="$BACKUP_DIR/config.yaml"

if [ ! -f "$USER_CONFIG_BAK" ] && [ ! -f "$DIR/config.yaml" ]; then
    # Fresh install: no prior config at all — create from template.
    if [ -f "$DIR/config_template.yaml" ]; then
        cp "$DIR/config_template.yaml" "$DIR/config.yaml"
        ok "Created config.yaml from template."
    fi
else
    # Existing install: merge user settings with any new template fields.
    if $BINARY_ONLY && [ -f "$DIR/config.yaml.new_template" ]; then
        CURRENT_TEMPLATE="$DIR/config.yaml.new_template"
    elif [ -f "$DIR/config_template.yaml" ]; then
        CURRENT_TEMPLATE="$DIR/config_template.yaml"
    else
        # Fallback: extract template from git history.
        _TMPL=$(mktemp "/tmp/aurago-config-tmpl.XXXXXX")
        if git show HEAD:config_template.yaml > "$_TMPL" 2>/dev/null && [ -s "$_TMPL" ]; then
            CURRENT_TEMPLATE="$_TMPL"
        elif git show HEAD:config.yaml > "$_TMPL" 2>/dev/null && [ -s "$_TMPL" ]; then
            CURRENT_TEMPLATE="$_TMPL"
        else
            CURRENT_TEMPLATE=""
        fi
    fi

    if [ -n "${CURRENT_TEMPLATE:-}" ] && [ -f "$USER_CONFIG_BAK" ]; then
        MERGER_BIN=""
        if [ -f "$DIR/bin/config-merger_linux" ]; then
            MERGER_BIN="$DIR/bin/config-merger_linux"
        elif [ -f "$DIR/bin/config-merger" ]; then
            MERGER_BIN="$DIR/bin/config-merger"
        elif [ -f "$DIR/cmd/config-merger/config-merger" ]; then
            MERGER_BIN="$DIR/cmd/config-merger/config-merger"
        fi

        if [ -n "$MERGER_BIN" ]; then
            info "Running config-merger to integrate your settings..."
            if "$MERGER_BIN" -source "$USER_CONFIG_BAK" -template "$CURRENT_TEMPLATE" -output "$DIR/config.yaml"; then
                ok "Your settings have been merged into the new config.yaml."
            else
                warn "config-merger failed. Restoring your old config.yaml exactly."
                safe_restore_file "$USER_CONFIG_BAK" "$DIR/config.yaml" \
                    || die "Could not restore config.yaml after failed merge (permission denied)."
            fi
        else
            warn "config-merger not found. Keeping your existing config.yaml."
            # User's config is already on disk (restored after git ops). Nothing to do.
        fi
    else
        warn "No template found. Keeping your existing config.yaml."
    fi

    [ -n "${_TMPL:-}" ] && rm -f "$_TMPL"
    [ -f "$DIR/config.yaml.new_template" ] && rm -f "$DIR/config.yaml.new_template"
fi

# ── Update binary ───────────────────────────────────────────────────────
section "Updating binaries"

# Ensure bin directory exists (e.g. if user manually deleted it)
mkdir -p "$DIR/bin"

# Release downloads reuse the tag and checksum manifest pinned before shutdown.
# A source build packages and pins its own resources, independent of Releases.

if $GO_FOUND; then
    # ── Source build (Go available) ───────────────────────────────────────
    info "Go $GO_VERSION found — building from source..."

    if [ "$GOARCH" = "arm" ] && [ -n "${GOARM:-}" ]; then
        export GOARM
    fi

    info "Building aurago_linux ($GOARCH)..."
    update_space_check 6291456 || abort_update "Insufficient staging/build space."
    TUI_RUN_EPHEMERAL=1 tui_run "Packaging web resources" go run ./cmd/assetpack -out "$UPDATE_WORK/packed" -stage assets/web || abort_update "Failed to package web resources."
    UPDATE_NEW_ASSET="$(sed -n 's/.*"asset_set_id"[[:space:]]*:[[:space:]]*"\([a-f0-9]\{64\}\)".*/\1/p' "$UPDATE_WORK/packed/web-assets.json")"
    [[ "$UPDATE_NEW_ASSET" =~ ^[a-f0-9]{64}$ ]] || abort_update "Invalid packed asset identity."
    update_manifest pending
    ASSET_LDFLAGS="$(cat "$UPDATE_WORK/packed/web-assets.ldflags")"
    if TUI_RUN_EPHEMERAL=1 tui_run "Compiling aurago_linux" env CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" go build -trimpath -ldflags="-s -w $ASSET_LDFLAGS" -o bin/aurago_linux ./cmd/aurago; then
        ok "bin/aurago_linux built from source"
    else
        abort_update "Failed to build required bin/aurago_linux. Update aborted."
    fi

    info "Building config-merger_linux ($GOARCH)..."
    if TUI_RUN_EPHEMERAL=1 tui_run "Compiling config-merger_linux" env CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" go build -trimpath -ldflags='-s -w' -o bin/config-merger_linux ./cmd/config-merger; then
        ok "bin/config-merger_linux built from source"
    fi

    info "Building aurago-remote_linux ($GOARCH)..."
    if TUI_RUN_EPHEMERAL=1 tui_run "Compiling aurago-remote_linux" env CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" go build -trimpath -ldflags='-s -w' -o bin/aurago-remote_linux ./cmd/remote; then
        ok "bin/aurago-remote_linux built from source"
        mkdir -p "$DIR/deploy"
        cp "$DIR/bin/aurago-remote_linux" "$DIR/deploy/aurago-remote_linux_${GOARCH}"
    fi

    # Cross-compile aurago-remote for all client platforms so the
    # /api/remote/download/{os}/{arch} endpoint can serve them.
    info "Cross-compiling aurago-remote client binaries..."
    mkdir -p "$DIR/deploy"
    for _target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
        _os="${_target%/*}"
        _arch="${_target#*/}"
        _ext=""
        [ "$_os" = "windows" ] && _ext=".exe"
        _out="$DIR/deploy/aurago-remote_${_os}_${_arch}${_ext}"
        # Skip if we already built this exact combo above
        if [ "$_os" = "linux" ] && [ "$_arch" = "$GOARCH" ] && [ -f "$_out" ]; then
            continue
        fi
        if TUI_RUN_EPHEMERAL=1 tui_run "Cross-compiling $_os/$_arch" env CGO_ENABLED=0 GOOS="$_os" GOARCH="$_arch" go build -trimpath -ldflags='-s -w' -o "$_out" ./cmd/remote; then
            ok "  $_out"
        else
            warn "  cross-compile failed: $_os/$_arch"
        fi
    done

    [ -f "$DIR/bin/aurago_linux" ] || abort_update "Required AuraGo binary missing after source build."
    ok "Main binary built successfully"
    if $BINARY_ONLY; then
        printf '%s' "$RELEASE_TAG" > "$DIR/.version"
    elif [ -n "${GIT_VER:-}" ]; then
        printf '%s' "$GIT_VER" > "$DIR/.version"
    fi

else
    # ── Download binaries from GitHub Releases (no Go available) ─────────
    warn "Go is not installed — downloading pre-built binaries from GitHub Releases."

    select_release_bins_for_arch

    if [ -z "${STAGED_RELEASE_DIR:-}" ]; then
        STAGED_RELEASE_DIR="$(mktemp -d "$UPDATE_WORK/release.XXXXXX")"
        for BIN_NAME in "${REQUIRED_BINS[@]}"; do
            info "Downloading required $BIN_NAME from GitHub Releases..."
            if _download_release_bin "$BIN_NAME" "$STAGED_RELEASE_DIR/$BIN_NAME"; then
                ok "$BIN_NAME downloaded and verified."
            else
                abort_update "Required release artifact $BIN_NAME could not be downloaded or verified."
            fi
        done
        for BIN_NAME in "${OPTIONAL_BINS[@]}"; do
            info "Downloading optional $BIN_NAME from GitHub Releases..."
            if _download_release_bin "$BIN_NAME" "$STAGED_RELEASE_DIR/$BIN_NAME"; then
                ok "$BIN_NAME downloaded and verified."
            else
                warn "$BIN_NAME download failed; continuing without optional remote client artifact."
            fi
        done
    fi

    ASSET_BIN="$STAGED_RELEASE_DIR/aurago_linux"
    [ "$GOARCH" != "arm64" ] || ASSET_BIN="$STAGED_RELEASE_DIR/aurago_linux_arm64"
    chmod +x "$ASSET_BIN"
    UPDATE_NEW_ASSET="$(update_asset_id "$ASSET_BIN")" || abort_update "Cannot identify downloaded assets."
    update_manifest pending
    # Source checkouts without Go also need the release's pinned resource set.
    # Reuse an already verified local set before downloading its shared archive.
    if ! "$ASSET_BIN" --assets-dir "$DIR/assets/web" --check-assets >/dev/null 2>&1; then
        ASSET_SET_ID="$("$ASSET_BIN" --assets-info | sed -n 's/.*"asset_set_id":"\([a-f0-9]\{64\}\)".*/\1/p')"
        [ "${#ASSET_SET_ID}" -eq 64 ] || abort_update "Release binary has no valid resource pin."
        ASSET_ARCHIVE="aurago-web-assets-${ASSET_SET_ID}.tar.gz"
        download_release_asset "$ASSET_ARCHIVE" "$STAGED_RELEASE_DIR/$ASSET_ARCHIVE" || abort_update "Matching web resource download failed."
        "$ASSET_BIN" --assets-dir "$DIR/assets/web" --install-assets "$STAGED_RELEASE_DIR/$ASSET_ARCHIVE" || abort_update "Matching web resource installation failed."
    fi
    "$ASSET_BIN" --assets-dir "$DIR/assets/web" --check-assets || abort_update "Release binary and web resources do not match."
    mkdir -p "$DIR/bin"
    for BIN_NAME in "${REQUIRED_BINS[@]}" "${OPTIONAL_BINS[@]}"; do
        [ -f "$STAGED_RELEASE_DIR/$BIN_NAME" ] || continue
        cp -p "$STAGED_RELEASE_DIR/$BIN_NAME" "$DIR/bin/$BIN_NAME" || abort_update "Could not install verified release artifact $BIN_NAME."
    done

    # Ensure standard names exist (for arm64 → copy to non-suffixed names)
    if [ "$GOARCH" = "arm64" ]; then
        [ -f "$DIR/bin/aurago_linux_arm64" ]             && cp -p "$DIR/bin/aurago_linux_arm64"             "$DIR/bin/aurago_linux"
        [ -f "$DIR/bin/config-merger_linux_arm64" ]      && cp -p "$DIR/bin/config-merger_linux_arm64"      "$DIR/bin/config-merger_linux"
        [ -f "$DIR/bin/aurago-remote_linux_arm64" ]      && cp -p "$DIR/bin/aurago-remote_linux_arm64"      "$DIR/bin/aurago-remote_linux"
    fi

    # Download aurago-remote client binaries for all platforms so the
    # /api/remote/download/{os}/{arch} endpoint can serve them.
    mkdir -p "$DIR/deploy"
    STAGED_DEPLOY_DIR="$(mktemp -d "$UPDATE_WORK/deploy.XXXXXX")"
    info "Downloading aurago-remote client binaries for all platforms..."
    for _t in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
        _ros="${_t%/*}"; _rarch="${_t#*/}"; _rext=""
        [ "$_ros" = "windows" ] && _rext=".exe"
        _rname="aurago-remote_${_ros}_${_rarch}${_rext}"
        if download_release_asset "${_rname}" "$STAGED_DEPLOY_DIR/${_rname}"; then
            cp -p "$STAGED_DEPLOY_DIR/${_rname}" "$DIR/deploy/${_rname}"
            ok "  deploy/${_rname}"
        else
            warn "  Could not download deploy/${_rname} — skipping."
        fi
    done
    mark_executable_if_present "$DIR/deploy/aurago-remote_linux_amd64"
    mark_executable_if_present "$DIR/deploy/aurago-remote_linux_arm64"
    mark_executable_if_present "$DIR/deploy/aurago-remote_linux"

    [ -f "$DIR/bin/aurago_linux" ] || abort_update "Required AuraGo binary missing after update."
    ok "Main binary built successfully"
    printf '%s' "$RELEASE_TAG" > "$DIR/.version"
fi

# Ensure known binaries and helper scripts are executable. Keep this list
# explicit so updates never make arbitrary dropped files executable.
for _exe in \
    "$DIR/bin/aurago_linux" \
    "$DIR/bin/aurago_linux_amd64" \
    "$DIR/bin/aurago_linux_arm64" \
    "$DIR/bin/config-merger_linux" \
    "$DIR/bin/config-merger_linux_amd64" \
    "$DIR/bin/config-merger_linux_arm64" \
    "$DIR/bin/aurago-remote_linux" \
    "$DIR/bin/aurago-remote_linux_amd64" \
    "$DIR/bin/aurago-remote_linux_arm64" \
    "$DIR/start.sh" \
    "$DIR/update.sh" \
    "$DIR/install_service_linux.sh" \
    "$DIR/make_deploy.sh"; do
    mark_executable_if_present "$_exe"
done
apply_aurago_setcap_if_available

# ── Patch service file: ensure User= / Group= are set (migration for root-installs) ──
SVC_FILE="/etc/systemd/system/aurago.service"
if [ -f "$SVC_FILE" ] && ! grep -q '^User=' "$SVC_FILE"; then
    # Detect the right user: prefer install directory owner, then SUDO_USER
    _svc_user=""
    _dir_owner="$(stat_owner "$DIR" 2>/dev/null || echo '')"
    if [ -n "$_dir_owner" ] && [ "$_dir_owner" != "root" ]; then
        _svc_user="$_dir_owner"
    elif [ -n "${SUDO_USER:-}" ]; then
        _svc_user="$SUDO_USER"
    fi

    if [ -n "$_svc_user" ]; then
        _svc_group=$(id -gn "$_svc_user" 2>/dev/null || echo "$_svc_user")
        warn "Service file missing User= — was running as root. Patching to User=${_svc_user}..."
        # Insert User=/Group= after Type= line
        $SUDO sed -i "/^Type=/a User=${_svc_user}\nGroup=${_svc_group}" "$SVC_FILE"
        # Fix ownership of data and bin so the new user can write them
        $SUDO chown -R "${_svc_user}:${_svc_group}" "${DIR}/data" "${DIR}/bin" "${DIR}/agent_workspace" 2>/dev/null || true
        $SUDO systemctl daemon-reload
        ok "Service patched: now runs as ${_svc_user}:${_svc_group}. Data directory re-owned."
    else
        warn "Service file has no User= and could not determine a non-root user."
        warn "Consider adding 'User=<youruser>' to $SVC_FILE manually."
    fi
fi

# Grant existing systemd installations access to GPU render devices without
# requiring permanent changes to the service user's account memberships.
_gpu_groups_line="$(systemd_gpu_groups_line)"
if [ -f "$SVC_FILE" ] && [ -n "$_gpu_groups_line" ]; then
    _current_gpu_groups_line="$(grep '^SupplementaryGroups=' "$SVC_FILE" | head -n 1 || true)"
    if [ "$_current_gpu_groups_line" != "$_gpu_groups_line" ]; then
        if [ -n "$_current_gpu_groups_line" ]; then
            $SUDO sed -i "s/^SupplementaryGroups=.*/${_gpu_groups_line}/" "$SVC_FILE"
        elif grep -q '^Group=' "$SVC_FILE"; then
            $SUDO sed -i "/^Group=/a ${_gpu_groups_line}" "$SVC_FILE"
        elif grep -q '^User=' "$SVC_FILE"; then
            $SUDO sed -i "/^User=/a ${_gpu_groups_line}" "$SVC_FILE"
        fi
        $SUDO systemctl daemon-reload
        ok "Service GPU access updated: ${_gpu_groups_line#SupplementaryGroups=}."
    fi
fi

# Forward numeric host GPU group IDs to managed containers. Host account group
# membership is intentionally left unchanged; Docker receives only the groups
# needed by the isolated Vulkan sidecar.
_gpu_group_ids="$(system_gpu_group_ids)"
_gpu_group_ids_line=""
if [ -n "$_gpu_group_ids" ]; then
    _gpu_group_ids_line="Environment=\"AURAGO_GPU_GROUP_IDS=${_gpu_group_ids}\""
fi
if [ -f "$SVC_FILE" ]; then
    _current_gpu_group_ids_line="$(grep '^Environment=.*AURAGO_GPU_GROUP_IDS=' "$SVC_FILE" | head -n 1 || true)"
    if [ "$_current_gpu_group_ids_line" != "$_gpu_group_ids_line" ]; then
        if [ -z "$_gpu_group_ids_line" ]; then
            $SUDO sed -i '/^Environment=.*AURAGO_GPU_GROUP_IDS=/d' "$SVC_FILE"
        elif [ -n "$_current_gpu_group_ids_line" ]; then
            $SUDO sed -i "/^Environment=.*AURAGO_GPU_GROUP_IDS=/c\\${_gpu_group_ids_line}" "$SVC_FILE"
        elif grep -q '^SupplementaryGroups=' "$SVC_FILE"; then
            $SUDO sed -i "/^SupplementaryGroups=/a ${_gpu_group_ids_line}" "$SVC_FILE"
        elif grep -q '^Group=' "$SVC_FILE"; then
            $SUDO sed -i "/^Group=/a ${_gpu_group_ids_line}" "$SVC_FILE"
        elif grep -q '^User=' "$SVC_FILE"; then
            $SUDO sed -i "/^User=/a ${_gpu_group_ids_line}" "$SVC_FILE"
        fi
        $SUDO systemctl daemon-reload
        if [ -n "$_gpu_group_ids" ]; then
            ok "Managed-container GPU groups updated: ${_gpu_group_ids}."
        else
            ok "Removed stale managed-container GPU group IDs."
        fi
    fi
fi

# ── Bluetooth host preparation (stored decision; repairs every update) ──
bluetooth_apply_choice

# Keep systemd's stop deadline slightly above AuraGo's 45-second internal
# shutdown deadline. A dedicated drop-in avoids position-dependent edits to
# legacy service files.
# Also grant host USB serial access through this backed-up, verified drop-in.
# SupplementaryGroups entries accumulate; account memberships and GPU IDs stay unchanged.
if [ -f "$SVC_FILE" ]; then
    _dropin_tmp="$(mktemp)"
    printf '%s\n' \
        '[Service]' \
        'TimeoutStopSec=60s' \
        "$(systemd_serial_groups_line)" > "$_dropin_tmp"
    if ! $SUDO mkdir -p "$SYSTEMD_DROPIN_DIR" ||
       ! $SUDO install -o root -g root -m 0644 "$_dropin_tmp" "$SYSTEMD_STOP_TIMEOUT_DROPIN"; then
        rm -f -- "$_dropin_tmp"
        abort_update "Could not install the systemd stop-timeout drop-in."
    fi
    rm -f -- "$_dropin_tmp"
    SYSTEMD_DROPIN_CHANGED=true
    if ! $SUDO systemctl daemon-reload ||
       ! systemd-analyze verify aurago.service >/dev/null 2>&1; then
        restore_service_stop_timeout_dropin || warn "Could not restore the previous systemd stop-timeout drop-in after verification failed."
        abort_update "systemd rejected the AuraGo service configuration after installing the stop-timeout drop-in."
    fi
    ok "Service stop timeout drop-in verified at 60 seconds."
    ok "Service USB serial access configured for available dialout/uucp groups."
fi

# ── Service restart ────────────────────────────────────────────────────
section "Restart"

LAUNCH_BIN="$DIR/bin/aurago_linux"
[ -x "$LAUNCH_BIN" ] || LAUNCH_BIN="$DIR/bin/aurago"
UPDATE_NEW_ASSET="$(update_asset_id "$LAUNCH_BIN")" || abort_update "Cannot identify updated assets."
UPDATE_NEW_VERSION="$(sha256sum "$LAUNCH_BIN" | awk '{print $1}')"
update_manifest pending
"$LAUNCH_BIN" --assets-dir "$DIR/assets/web" --check-assets || abort_update "Installed web resource validation failed."
STARTED_AFTER_UPDATE=false
START_MODE=""

start_updated_directly() {
    [ -x "$LAUNCH_BIN" ] || return 1
    mkdir -p "$DIR/log"
    if [ -z "${AURAGO_MASTER_KEY:-}" ] && [ -f "$DIR/.env" ]; then
        AURAGO_MASTER_KEY="$(read_master_key_from_env "$DIR/.env")"
        export AURAGO_MASTER_KEY
    fi
    nohup env -u AURAGO_UPDATE_LOCK_FD "$LAUNCH_BIN" --config "$DIR/config.yaml" 9>&- >>"${DIR}/log/aurago.log" 2>&1 &
    LAUNCH_PID=$!
    START_MODE="direct"
    STARTED_AFTER_UPDATE=true
    info "AuraGo starting directly (PID=$LAUNCH_PID)..."
}

if $NO_RESTART; then
    warn "Skipping restart (--no-restart flag set). Start manually:"
    echo "   sudo systemctl restart aurago   OR   ./start.sh"
elif command -v systemctl >/dev/null 2>&1 && systemctl list-unit-files aurago.service >/dev/null 2>&1; then
    info "Starting aurago systemd service..."
    if $SUDO systemctl start aurago; then
        START_MODE="systemd"
        STARTED_AFTER_UPDATE=true
    else
        warn "sudo not available — starting aurago directly (systemd will adopt on next boot)"
        start_updated_directly || abort_update "No executable AuraGo binary is available after the update."
    fi
else
    start_updated_directly || abort_update "No executable AuraGo binary is available after the update."
fi

if $STARTED_AFTER_UPDATE; then
    if [ "$START_MODE" = "systemd" ]; then
        # A systemd start failed health check is a core failure and must use
        # the same complete rollback path as any other core-readiness failure.
        _active_wait=0
        while ! systemctl is-active --quiet aurago 2>/dev/null && [ "$_active_wait" -lt 20 ]; do
            sleep 1
            _active_wait=$((_active_wait + 1))
        done
        if ! systemctl is-active --quiet aurago 2>/dev/null; then
            abort_update "The updated systemd process did not become active."
        fi
        ok "Updated systemd process is active."
    fi

    if ! "$LAUNCH_BIN" --config "$DIR/config.yaml" --healthcheck --healthcheck-timeout 60s; then
        abort_update "Core readiness failed after the update."
    fi
    ok "Core readiness verified."

    if [ "$TSNET_WAS_READY" = "ready" ]; then
        TSNET_HEALTH_OUTPUT=""
        if ! TSNET_HEALTH_OUTPUT="$("$LAUNCH_BIN" --config "$DIR/config.yaml" --healthcheck --healthcheck-timeout 210s --healthcheck-require-tsnet 2>&1)"; then
            [ -z "$TSNET_HEALTH_OUTPUT" ] || printf '%s\n' "$TSNET_HEALTH_OUTPUT" >&2
            warn "AuraGo core is healthy, but the previously working tsnet node did not recover."
            warn "The updated binary and the state backup are being kept; the core update does not need rollback."
            warn "$(tsnet_failure_guidance "$TSNET_HEALTH_OUTPUT")"
            die "Update incomplete: tsnet readiness failed. Backup: $BACKUP_DIR"
        fi
        ok "tsnet readiness verified."
    fi
fi

if $STARTED_AFTER_UPDATE; then
    update_manifest confirmed
    update_retention_cleanup "$LAUNCH_BIN"
else
    # A manually deferred restart is not evidence of a successful update.
    update_manifest pending
    warn "Retention is deferred until the installed version has passed readiness checks."
fi

# ── Summary ────────────────────────────────────────────────────────────
if $BINARY_ONLY; then
    POST_UPDATE_VERSION="$RELEASE_TAG"
else
    POST_UPDATE_VERSION="$(git log --format='%h (%cd)' --date=short -1 2>/dev/null || true)"
fi
echo ""
tui_box ok "AuraGo updated successfully!" \
    "${DIM}Version:${NC}    ${PRE_UPDATE_VERSION:-unknown} ${T_ARROW} ${BOLD}${POST_UPDATE_VERSION}${NC}" \
    "${DIM}Backup:${NC}     $BACKUP_DIR" \
    "${DIM}Retention:${NC}  current version plus two verified rollback versions" \
    "${DIM}Duration:${NC}   $(tui_elapsed)"
echo ""
