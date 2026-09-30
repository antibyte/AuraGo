#!/usr/bin/env bash
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
#  AuraGo TUI kit - source of truth for the text-mode look of install.sh
#  and update.sh.  Both scripts must stay single-file (curl | bash, release
#  assets), so the block between the KIT markers below is embedded verbatim.
#
#    1. edit the block between the markers in THIS file
#    2. run  scripts/sync-tui-kit.sh  to copy it into install.sh / update.sh
#    3. an audit test (internal/audit) fails if the copies ever drift
#
#  Preview the look:   bash scripts/aurago-tui.sh [--plain]
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

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

# ── Demo: `bash scripts/aurago-tui.sh` previews every element ─────────────
if [ "${BASH_SOURCE[0]}" = "$0" ]; then
    case "${1:-}" in --plain) AURAGO_UI=plain; _tui_detect; _tui_palette ;; esac
    TUI_STEP_TOTAL=4
    tui_banner "Quick Installer" "AI Agent Framework for Linux"
    tui_section "System scan"
    tui_msg info "Detecting hardware and package manager"
    tui_msg ok "Architecture: x86_64 ${T_ARROW} target: amd64"
    tui_msg warn "Docker not found - sandbox features will be limited"
    tui_section "Dependencies"
    tui_run "Installing ffmpeg" sleep 1.2
    tui_run "Compiling aurago (this one fails)" bash -c 'echo "go: downloading x"; echo "boom: undefined: Foo"; exit 3' || true
    tui_subsection "Changelog"
    tui_msg info "abc1234  fix(security): require setup bootstrap token"
    tui_section "Configuration"
    tui_box warn "SECURITY RECOMMENDATION" \
        "Your vault master key is stored in ${BOLD}.env${NC} inside the AuraGo" \
        "directory. Move it to ${BOLD}/etc/aurago/master.key${NC} (mode 0600)."
    tui_section "Done"
    tui_sparkle
    tui_box ok "AuraGo successfully installed!" \
        "${DIM}Location:${NC} /home/user/aurago" \
        "${CYAN}Web UI:${NC}         http://192.168.1.20:8088" \
        "${DIM}Finished in $(tui_elapsed)${NC}"
    tui_die "Could not determine latest release tag from GitHub. Check your network and retry."
    printf '\n'
fi
