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
        # @release-verifier-pins@
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
