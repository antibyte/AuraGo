#!/usr/bin/env bash
# Sign and verify the completed manifest before any release can be published.
set -euo pipefail
manifest="${1:?Usage: sign-release-checksums.sh PATH/TO/SHA256SUMS}"
[ -f "$manifest" ] || { echo "Release checksum manifest is missing." >&2; exit 1; }
sig="${manifest}.sig"
cert="${manifest}.pem"
rm -f -- "$sig" "$cert" "${sig}.pending" "${cert}.pending"
if [ -z "${AURAGO_RELEASE_SIGNING_KEY:-}" ] && [ "${GITHUB_ACTIONS:-}" != true ]; then
    [ "${AURAGO_ALLOW_UNSIGNED_RELEASES:-}" = 1 ] || { echo "Configure release signing or explicitly set AURAGO_ALLOW_UNSIGNED_RELEASES=1 for a legacy unsigned release." >&2; exit 1; }
    echo "Explicit legacy unsigned release; no signature artifacts retained." >&2
    exit 0
fi
command -v cosign >/dev/null || { echo "cosign is required for release signing." >&2; exit 1; }
# Preserve the detached format; Cosign 3's default signing config conflicts with offline signing.
signing_options=(--new-bundle-format=false)
signing_help="$(cosign sign-blob --help)"
if [[ "$signing_help" == *--use-signing-config* ]]; then signing_options+=(--use-signing-config=false); fi
trap 'rm -f -- "${sig}.pending" "${cert}.pending"' EXIT
if [ -n "${AURAGO_RELEASE_SIGNING_KEY:-}" ]; then
    [ -n "${AURAGO_RELEASE_PUBLIC_KEY:-}" ] && [ -f "$AURAGO_RELEASE_PUBLIC_KEY" ] || { echo "A trusted AURAGO_RELEASE_PUBLIC_KEY is required to verify the signature." >&2; exit 1; }
    cosign sign-blob --yes "${signing_options[@]}" --key "$AURAGO_RELEASE_SIGNING_KEY" --tlog-upload=false --output-signature "${sig}.pending" "$manifest"
    cosign verify-blob --key "$AURAGO_RELEASE_PUBLIC_KEY" --insecure-ignore-tlog --signature "${sig}.pending" "$manifest"
else
    cosign sign-blob --yes "${signing_options[@]}" --output-signature "${sig}.pending" --output-certificate "${cert}.pending" "$manifest"
    cosign verify-blob --certificate "${cert}.pending" --signature "${sig}.pending" \
        --certificate-identity-regexp "https://github.com/${GITHUB_REPOSITORY:-antibyte/AuraGo}/.*" \
        --certificate-oidc-issuer https://token.actions.githubusercontent.com "$manifest"
    mv -- "${cert}.pending" "$cert"
fi
mv -- "${sig}.pending" "$sig"
