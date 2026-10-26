[CmdletBinding(DefaultParameterSetName = 'Sign')]
param(
    [Parameter(Mandatory = $true, ParameterSetName = 'Sign')][string]$Manifest,
    [Parameter(Mandatory = $true, ParameterSetName = 'Check')][switch]$CheckPrerequisites
)
$ErrorActionPreference = 'Stop'
if (-not $CheckPrerequisites) {
    if (-not (Test-Path -LiteralPath $Manifest -PathType Leaf)) { throw 'Release checksum manifest is missing.' }
    $Manifest = (Resolve-Path -LiteralPath $Manifest).Path
    $signature = "$Manifest.sig"
    $certificate = "$Manifest.pem"
    foreach ($path in @($signature, $certificate, "$signature.pending", "$certificate.pending")) {
        if (Test-Path -LiteralPath $path) { Remove-Item -LiteralPath $path -Force }
    }
}
$signingKey = $env:AURAGO_RELEASE_SIGNING_KEY
$publicKey = $env:AURAGO_RELEASE_PUBLIC_KEY
$command = Get-Command cosign -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
$cosign = if ($command) { $command.Source } else { $null }
$originalPassword = $env:COSIGN_PASSWORD
$protectedPassword = $null
try {
    if (-not $signingKey -and -not $publicKey -and $env:GITHUB_ACTIONS -ne 'true' -and $env:AURAGO_ALLOW_UNSIGNED_RELEASES -ne '1' -and [Environment]::OSVersion.Platform -eq 'Win32NT' -and $env:LOCALAPPDATA) {
        $profile = Join-Path $env:LOCALAPPDATA 'AuraGo\release-signing'
        if (Test-Path -LiteralPath $profile) {
            $signingKey = Join-Path $profile 'release.key'
            $publicKey = Join-Path $profile 'release.pub'
            $cosign = Join-Path $profile 'cosign.exe'
            foreach ($path in @($signingKey, $publicKey, $cosign)) {
                if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw 'Local release signing profile is incomplete; restore it or run scripts/setup-release-signing.ps1.' }
            }
            if (-not $originalPassword) {
                try {
                    $protectedPassword = ConvertTo-SecureString ([IO.File]::ReadAllText((Join-Path $profile 'password.dpapi')))
                    if (-not $protectedPassword.Length) { throw 'Empty password.' }
                } catch { throw 'Cannot unlock the local release signing profile under this Windows user.' }
            }
        }
    }
    if (-not $signingKey -and $env:GITHUB_ACTIONS -ne 'true') {
        if ($env:AURAGO_ALLOW_UNSIGNED_RELEASES -ne '1') { throw 'Run scripts/setup-release-signing.ps1 or configure release signing; legacy unsigned releases require explicit AURAGO_ALLOW_UNSIGNED_RELEASES=1.' }
        Write-Warning 'Explicit legacy unsigned release; no signature artifacts retained.'
        return
    }
    if (-not $cosign -or -not (Test-Path -LiteralPath $cosign -PathType Leaf)) { throw 'cosign is required for release signing.' }
    if ($signingKey) {
        if (-not $publicKey -or -not (Test-Path -LiteralPath $publicKey -PathType Leaf)) { throw 'A trusted AURAGO_RELEASE_PUBLIC_KEY is required to verify the signature.' }
        if ($signingKey -notmatch '^[a-zA-Z][a-zA-Z0-9+.-]*://' -and -not (Test-Path -LiteralPath $signingKey -PathType Leaf)) { throw 'Release signing private key is missing.' }
    }
    if ($CheckPrerequisites) {
        $null = & $cosign version
        if ($LASTEXITCODE -ne 0) { throw 'Cannot run Cosign.' }
        Write-Host '    Release signing: Cosign ready' -ForegroundColor Green
        return
    }
    # Preserve the detached format; Cosign 3's default signing config conflicts with offline signing.
    $signingOptions = @('--new-bundle-format=false')
    $signingHelp = & $cosign sign-blob --help
    if ($LASTEXITCODE -ne 0) { throw 'Cannot inspect Cosign signing options.' }
    # Cosign 3.1.4 hides this still-required compatibility flag from --help.
    $preference = $ErrorActionPreference
    try {
        $ErrorActionPreference = 'Continue'
        $null = & $cosign sign-blob --use-signing-config=false --help 2>$null
        $supportsSigningConfig = $LASTEXITCODE -eq 0
    } finally { $ErrorActionPreference = $preference }
    if ($supportsSigningConfig) { $signingOptions += '--use-signing-config=false' }
    if ($protectedPassword) {
        $pointer = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($protectedPassword)
        try { $env:COSIGN_PASSWORD = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($pointer) }
        finally { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($pointer) }
    }
    if ($signingKey) {
        & $cosign sign-blob --yes @signingOptions --key $signingKey --tlog-upload=false --output-signature "$signature.pending" $Manifest
        if ($LASTEXITCODE -ne 0) { throw 'Release manifest signing failed.' }
        & $cosign verify-blob --key $publicKey --insecure-ignore-tlog --signature "$signature.pending" $Manifest
        if ($LASTEXITCODE -ne 0) { throw 'Release signature verification failed.' }
    } else {
        & $cosign sign-blob --yes @signingOptions --output-signature "$signature.pending" --output-certificate "$certificate.pending" $Manifest
        if ($LASTEXITCODE -ne 0) { throw 'Release manifest signing failed.' }
        $repository = if ($env:GITHUB_REPOSITORY) { $env:GITHUB_REPOSITORY } else { 'antibyte/AuraGo' }
        & $cosign verify-blob --certificate "$certificate.pending" --signature "$signature.pending" --certificate-identity-regexp "https://github.com/$repository/.*" --certificate-oidc-issuer https://token.actions.githubusercontent.com $Manifest
        if ($LASTEXITCODE -ne 0) { throw 'Release signature verification failed.' }
        Move-Item -LiteralPath "$certificate.pending" -Destination $certificate -Force
    }
    Move-Item -LiteralPath "$signature.pending" -Destination $signature -Force
} finally {
    $env:COSIGN_PASSWORD = $originalPassword
    if ($protectedPassword) { $protectedPassword.Dispose() }
    if (-not $CheckPrerequisites) {
        foreach ($path in @("$signature.pending", "$certificate.pending")) {
            if (Test-Path -LiteralPath $path) { Remove-Item -LiteralPath $path -Force }
        }
    }
}
