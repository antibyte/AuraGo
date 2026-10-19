param([Parameter(Mandatory = $true)][string]$Manifest)
$ErrorActionPreference = 'Stop'
if (-not (Test-Path -LiteralPath $Manifest -PathType Leaf)) { throw 'Release checksum manifest is missing.' }
$Manifest = (Resolve-Path -LiteralPath $Manifest).Path
$signature = "$Manifest.sig"
$certificate = "$Manifest.pem"
foreach ($path in @($signature, $certificate, "$signature.pending", "$certificate.pending")) {
    if (Test-Path -LiteralPath $path) { Remove-Item -LiteralPath $path -Force }
}
if (-not $env:AURAGO_RELEASE_SIGNING_KEY -and $env:GITHUB_ACTIONS -ne 'true') {
    if ($env:AURAGO_ALLOW_UNSIGNED_RELEASES -ne '1') { throw 'Configure release signing or explicitly set AURAGO_ALLOW_UNSIGNED_RELEASES=1 for a legacy unsigned release.' }
    Write-Warning 'Explicit legacy unsigned release; no signature artifacts retained.'
    return
}
if (-not (Get-Command cosign -ErrorAction SilentlyContinue)) { throw 'cosign is required for release signing.' }
# Keep the detached signature format consumed by install.sh/update.sh. Cosign 3
# also enables a signing-service configuration by default; older versions lack it.
$signingOptions = @('--new-bundle-format=false')
$signingHelp = & cosign sign-blob --help
if ($LASTEXITCODE -ne 0) { throw 'Cannot inspect Cosign signing options.' }
if (($signingHelp -join "`n") -match '--use-signing-config') { $signingOptions += '--use-signing-config=false' }
try {
    if ($env:AURAGO_RELEASE_SIGNING_KEY) {
        if (-not $env:AURAGO_RELEASE_PUBLIC_KEY -or -not (Test-Path -LiteralPath $env:AURAGO_RELEASE_PUBLIC_KEY -PathType Leaf)) { throw 'A trusted AURAGO_RELEASE_PUBLIC_KEY is required to verify the signature.' }
        & cosign sign-blob --yes @signingOptions --key $env:AURAGO_RELEASE_SIGNING_KEY --tlog-upload=false --output-signature "$signature.pending" $Manifest
        if ($LASTEXITCODE -ne 0) { throw 'Release manifest signing failed.' }
        & cosign verify-blob --key $env:AURAGO_RELEASE_PUBLIC_KEY --insecure-ignore-tlog --signature "$signature.pending" $Manifest
        if ($LASTEXITCODE -ne 0) { throw 'Release signature verification failed.' }
    } else {
        & cosign sign-blob --yes @signingOptions --output-signature "$signature.pending" --output-certificate "$certificate.pending" $Manifest
        if ($LASTEXITCODE -ne 0) { throw 'Release manifest signing failed.' }
        $repository = if ($env:GITHUB_REPOSITORY) { $env:GITHUB_REPOSITORY } else { 'antibyte/AuraGo' }
        & cosign verify-blob --certificate "$certificate.pending" --signature "$signature.pending" --certificate-identity-regexp "https://github.com/$repository/.*" --certificate-oidc-issuer https://token.actions.githubusercontent.com $Manifest
        if ($LASTEXITCODE -ne 0) { throw 'Release signature verification failed.' }
        Move-Item -LiteralPath "$certificate.pending" -Destination $certificate -Force
    }
    Move-Item -LiteralPath "$signature.pending" -Destination $signature -Force
} finally {
    foreach ($path in @("$signature.pending", "$certificate.pending")) {
        if (Test-Path -LiteralPath $path) { Remove-Item -LiteralPath $path -Force }
    }
}
