#!/usr/bin/env bash
# One-time operator migration for old updaters that cannot bootstrap themselves.
set -euo pipefail
umask 077
GITHUB_REPO=antibyte/AuraGo
DIR="$(pwd -P)"
RUN_UPDATE=false
while [ "$#" -gt 0 ]; do
    case "$1" in
        --root) [ "$#" -ge 2 ] || exit 2; DIR="$(cd "$2" && pwd -P)"; shift ;;
        --run) RUN_UPDATE=true ;;
        *) echo "Usage: repair-update.sh [--root INSTALL_DIR] [--run]" >&2; exit 2 ;;
    esac
    shift
done
die() { echo "Updater repair failed: $*" >&2; exit 1; }
warn() { echo "$*" >&2; }
ok() { echo "$*"; }
[ -f "$DIR/config.yaml" ] && [ -d "$DIR/bin" ] || die "Not an AuraGo installation: $DIR"
[ ! -L "$DIR/update.sh" ] && [ -f "$DIR/update.sh" ] || die "Unsafe or missing installed updater."
UPDATE_STATE="$DIR/.aurago-update"
[ ! -L "$UPDATE_STATE" ] || die "Unsafe update state symlink."
mkdir -p "$UPDATE_STATE"
chmod 700 "$UPDATE_STATE"
[ ! -L "$UPDATE_STATE/update.lock" ] || die "Unsafe update lock symlink."
command -v flock >/dev/null || die "flock is required for safe updates."
exec 9>"$UPDATE_STATE/update.lock"
flock -n 9 || die "Another update or cleanup is running."
WORK="$(mktemp -d "$DIR/.aurago-updater-repair.XXXXXX")"
trap 'rm -f -- "$WORK/metadata" "$WORK/SHA256SUMS" "$WORK/update.sh" "$WORK/previous-update.sh"; rmdir -- "$WORK" 2>/dev/null || true' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
fetch_optional_url_to_file() {
    if command -v curl >/dev/null; then
        curl --fail --silent --show-error --location --connect-timeout 15 --max-time 180 --retry 2 --proto '=https' "$1" -o "$2" 2>/dev/null
    elif command -v wget >/dev/null; then
        wget -q --timeout=180 --tries=3 -O "$2" "$1" 2>/dev/null
    else
        die "Neither curl nor wget is available."
    fi
}
sha256_file() {
    if command -v sha256sum >/dev/null; then sha256sum "$1" | awk '{print $1}'
    elif command -v shasum >/dev/null; then shasum -a 256 "$1" | awk '{print $1}'
    else openssl dgst -sha256 "$1" | awk '{print $NF}'; fi
}
strict_release_verify_enabled() { return 0; }
# BEGIN GENERATED RELEASE VERIFIER
# Canonical bootstrap; embedded in installer/updater/repair by build-release-verifier.py.
release_verifier_spec() {
    local os arch
    case "$(uname -s)" in
        Linux*) os=linux ;; Darwin*) os=darwin ;; MINGW*|MSYS*|CYGWIN*) os=windows ;;
        *) echo "No release verifier for this operating system." >&2; return 1 ;;
    esac
    case "$(uname -m)" in
        x86_64|amd64) arch=amd64 ;; aarch64|arm64) arch=arm64 ;;
        *) echo "No release verifier for this CPU architecture." >&2; return 1 ;;
    esac
    case "$1:$os:$arch" in
        native:linux:amd64) printf '%s\n' 'release-verifier_linux_amd64|3817596|b96421ef04cd4ef9472002f9072ec6b1436fb9dc62a8ae8322ec4d3314fed462|https://github.com/antibyte/AuraGo/releases/download/release-verifier-v1' ;;
        native:linux:arm64) printf '%s\n' 'release-verifier_linux_arm64|3539068|df50ed6812fe97ecd347519f93f2c314e1af77671a70d09610f2860ddd44d0c9|https://github.com/antibyte/AuraGo/releases/download/release-verifier-v1' ;;
        native:darwin:amd64) printf '%s\n' 'release-verifier_darwin_amd64|3928720|fefe16b25f95174b93c731b581c3a42a2782c5b2ec36641b3990f145fa464ab9|https://github.com/antibyte/AuraGo/releases/download/release-verifier-v1' ;;
        native:darwin:arm64) printf '%s\n' 'release-verifier_darwin_arm64|3625138|63310260d63c59511d5839f5bab8bef914215e887d13e2c314740b0982cf1607|https://github.com/antibyte/AuraGo/releases/download/release-verifier-v1' ;;
        native:windows:amd64) printf '%s\n' 'release-verifier_windows_amd64.exe|3947008|e345726a9bdf780bfcb1d547d9ea9a57fd2b6a6dccd774b5c8da9b6db33d0fa7|https://github.com/antibyte/AuraGo/releases/download/release-verifier-v1' ;;
        native:windows:arm64) printf '%s\n' 'release-verifier_windows_arm64.exe|3568640|e742caba916f8cf087a1eb46ee81cd1866f93631e61c628eaf8961aa3f31b324|https://github.com/antibyte/AuraGo/releases/download/release-verifier-v1' ;;
        cosign:linux:amd64) printf '%s\n' 'cosign-linux-amd64|145688404|424e2c97d0b45fd7a6b608a87f090f35633f4c23ece5f092d48bba11aaf4bbfa|https://github.com/sigstore/cosign/releases/download/v3.1.4' ;;
        cosign:linux:arm64) printf '%s\n' 'cosign-linux-arm64|136437761|ade83b7aac62e131cacfe96d958948b2150cc9efec46a570a2649b8e27dedad2|https://github.com/sigstore/cosign/releases/download/v3.1.4' ;;
        cosign:darwin:amd64) printf '%s\n' 'cosign-darwin-amd64|105951973|9d1e4abe2b6be279ed0d2ff706fde1ccd738747c834e3e6d44039c238867334f|https://github.com/sigstore/cosign/releases/download/v3.1.4' ;;
        cosign:darwin:arm64) printf '%s\n' 'cosign-darwin-arm64|99023586|9f9b8cd557cf2e5b3df3373a00baa005a86b02391e3e06dd0ce37b3a1533d3a4|https://github.com/sigstore/cosign/releases/download/v3.1.4' ;;
        cosign:windows:amd64) printf '%s\n' 'cosign-windows-amd64.exe|191066624|cbe953787074611f1334cbf37dc1cf953995bb8c45398eb4fc7bb781d29c1f2a|https://github.com/sigstore/cosign/releases/download/v3.1.4' ;;
        *) echo "No pinned verification tool for $os/$arch." >&2; return 1 ;;
    esac
}

release_verifier_fetch() (
    set -o pipefail
    local url="$1" out="$2" limit="$3"
    # Bound streamed bytes even without Content-Length or a working ulimit (Git Bash).
    if declare -F fetch_optional_url_to_file >/dev/null; then
        fetch_optional_url_to_file "$url" - | head -c "$((limit + 1))" > "$out" || return 1
    else
        _download_optional "$url" - | head -c "$((limit + 1))" > "$out" || return 1
    fi
    [ -f "$out" ] && [ "$(wc -c < "$out")" -le "$limit" ]
)

release_verifier_tool() (
    umask 077
    local spec asset size hash base state cache dir tool pending=''
    spec="$(release_verifier_spec "$1")" || return 1
    IFS='|' read -r asset size hash base <<< "$spec"
    state="${UPDATE_STATE:-${INSTALL_DIR:-${DIR:-$HOME/aurago}}/.aurago-update}"
    cache="$state/tools/${asset}-${hash:0:16}"
    for dir in "$state" "$state/tools" "$cache"; do
        [ ! -L "$dir" ] || { echo "Unsafe release verifier cache symlink." >&2; return 1; }
        mkdir -p "$dir" && [ -O "$dir" ] && chmod 700 "$dir" || return 1
    done
    tool="$cache/$asset"
    [ ! -L "$tool" ] || { echo "Unsafe release verifier symlink." >&2; return 1; }
    if [ -e "$tool" ] && [ ! -f "$tool" ]; then
        echo "Unsafe release verifier file type." >&2
        return 1
    fi
    if [ -f "$tool" ] && [ "$(wc -c < "$tool")" -eq "$size" ] &&
        [ "$(sha256_file "$tool")" = "$hash" ]; then
        chmod 700 "$tool" || return 1
        printf '%s\n' "$tool"
        return 0
    fi
    pending="$(mktemp "$cache/.download.XXXXXX")" || return 1
    trap 'rm -f -- "$pending"' EXIT
    trap 'exit 130' INT
    trap 'exit 143' TERM
    release_verifier_fetch "$base/$asset" "$pending" "$size" || return 1
    [ "$(wc -c < "$pending")" -eq "$size" ] && [ "$(sha256_file "$pending")" = "$hash" ] || {
        echo "Downloaded release verifier failed its pinned size/SHA-256 check." >&2
        return 1
    }
    chmod 700 "$pending" && mv -f -- "$pending" "$tool" || return 1
    printf '%s\n' "$tool"
)

verify_release_checksums_signature() (
    [ -n "${RELEASE_BASE:-}" ] || die "RELEASE_BASE is not set."
    [ -n "${RELEASE_CHECKSUMS_FILE:-}" ] && [ -f "$RELEASE_CHECKSUMS_FILE" ] || die "Release checksums are not available."
    [ "$(wc -c < "$RELEASE_CHECKSUMS_FILE")" -le 4194304 ] || die "Release checksum manifest is too large."
    umask 077
    local work sig cert keyring keyring_sig native cosign_tool have_sig=false have_cert=false
    work="$(mktemp -d "${TMPDIR:-/tmp}/aurago-release-verify.XXXXXX")" || exit 1
    sig="$work/SHA256SUMS.sig"
    cert="$work/SHA256SUMS.pem"
    keyring="$work/release-keyring.json"
    keyring_sig="$work/release-keyring.json.sig"
    trap 'rm -f -- "$sig" "$cert" "$keyring" "$keyring_sig"; rmdir -- "$work" 2>/dev/null || true' EXIT
    trap 'exit 130' INT
    trap 'exit 143' TERM
    release_verifier_fetch "$RELEASE_BASE/SHA256SUMS.sig" "$sig" 4096 && have_sig=true
    if [ -z "${AURAGO_RELEASE_PUBLIC_KEY:-}" ]; then
        release_verifier_fetch "$RELEASE_BASE/SHA256SUMS.pem" "$cert" 16384 && have_cert=true
    fi
    if { [ "$have_sig" = false ] && [ -s "$sig" ]; } || { [ "$have_cert" = false ] && [ -s "$cert" ]; }; then
        die "Release signature metadata is damaged or exceeds its size limit."
    fi
    if [ "$have_sig" = false ]; then
        if [ "$have_cert" = true ] || [ "$GITHUB_REPO" = antibyte/AuraGo ] ||
            strict_release_verify_enabled || [ -n "${AURAGO_RELEASE_PUBLIC_KEY:-}" ]; then
            die "Release signature is missing or incomplete; refusing an unsigned downgrade."
        fi
        warn "Legacy fork release has no signature; verifying its SHA256 manifest only."
        return 0
    fi
    native="$(release_verifier_tool native)" || die "Cannot provision the pinned release verifier; no installation was changed."
    local args=(--manifest "$RELEASE_CHECKSUMS_FILE" --signature "$sig")
    [ -z "${AURAGO_RELEASE_PUBLIC_KEY:-}" ] || args+=(--key "$AURAGO_RELEASE_PUBLIC_KEY")
    if "$native" "${args[@]}" >/dev/null 2>&1; then
        ok "Release checksum signature verified."
        return 0
    fi
    if [ -z "${AURAGO_RELEASE_PUBLIC_KEY:-}" ] && [ "$have_cert" = false ]; then
        # Successor keys are authorized by the embedded root, never self-declared.
        if release_verifier_fetch "$RELEASE_BASE/release-keyring.json" "$keyring" 65536 &&
            release_verifier_fetch "$RELEASE_BASE/release-keyring.json.sig" "$keyring_sig" 4096 &&
            "$native" "${args[@]}" --keyring "$keyring" --keyring-signature "$keyring_sig" >/dev/null 2>&1; then
            ok "Release checksum signature verified with an authorized successor key."
            return 0
        fi
        die "Release checksum signature verification failed."
    fi
    # Cosign remains responsible for Fulcio identity/issuer checks and custom key types.
    cosign_tool="$(release_verifier_tool cosign)" || die "Cannot provision Cosign for the release's signature format."
    if [ -n "${AURAGO_RELEASE_PUBLIC_KEY:-}" ]; then
        "$cosign_tool" verify-blob --key "$AURAGO_RELEASE_PUBLIC_KEY" --insecure-ignore-tlog \
            --signature "$sig" "$RELEASE_CHECKSUMS_FILE" >/dev/null 2>&1 || die "Release checksum signature verification failed."
    else
        "$cosign_tool" verify-blob --certificate "$cert" --signature "$sig" \
            --certificate-identity-regexp "https://github.com/${GITHUB_REPO}/.*" \
            --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
            "$RELEASE_CHECKSUMS_FILE" >/dev/null 2>&1 || die "Release checksum signature verification failed."
    fi
    ok "Release checksum signature verified."
)
# END GENERATED RELEASE VERIFIER

# REPAIR RELEASE PREFLIGHT
release_verifier_fetch "https://api.github.com/repos/$GITHUB_REPO/releases/latest" "$WORK/metadata" 1048576 || die "Cannot resolve the current release."
RELEASE_TAG="$(sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$WORK/metadata" | head -n 1)"
[[ "$RELEASE_TAG" =~ ^[a-zA-Z0-9._-]+$ ]] || die "Invalid release tag."
RELEASE_BASE="https://github.com/$GITHUB_REPO/releases/download/$RELEASE_TAG"
RELEASE_CHECKSUMS_FILE="$WORK/SHA256SUMS"
release_verifier_fetch "$RELEASE_BASE/SHA256SUMS" "$RELEASE_CHECKSUMS_FILE" 4194304 || die "Cannot download the checksum manifest."
verify_release_checksums_signature
release_verifier_fetch "$RELEASE_BASE/update.sh" "$WORK/update.sh" 1048576 || die "Cannot download the updater."
EXPECTED="$(awk '{ sub(/\r$/, "", $2); if ($2 == "update.sh") print $1 }' "$RELEASE_CHECKSUMS_FILE")"
[[ "$EXPECTED" =~ ^[0-9a-f]{64}$ ]] && [ "$(sha256_file "$WORK/update.sh")" = "$EXPECTED" ] || die "Updater checksum mismatch."
grep -Fq '# BEGIN GENERATED RELEASE VERIFIER' "$WORK/update.sh" || die "This release does not yet include automatic signature verification."
chmod 755 "$WORK/update.sh"
cp -p "$DIR/update.sh" "$WORK/previous-update.sh"
mv -f "$WORK/previous-update.sh" "$UPDATE_STATE/update.sh.before-repair"
mv -f "$WORK/update.sh" "$DIR/update.sh"
ok "Installed the verified updater for $RELEASE_TAG. Configuration and running services were preserved."
if [ "$RUN_UPDATE" = true ]; then
    rm -f -- "$WORK/metadata" "$WORK/SHA256SUMS"
    rmdir -- "$WORK"
    trap - EXIT
    flock -u 9
    cd "$DIR"
    exec bash "$DIR/update.sh" --yes
fi
