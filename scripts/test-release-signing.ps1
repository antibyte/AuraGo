# Local fixture only: no publication, OIDC login, provider calls or operator keys.
$ErrorActionPreference = 'Stop'
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$scratchRoot = [IO.Path]::GetFullPath((Join-Path $repo 'disposable'))
$fixture = Join-Path $scratchRoot ("_release-signing-" + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $fixture -Force | Out-Null
$saved = @{}
foreach ($name in @('COSIGN_PASSWORD', 'AURAGO_RELEASE_SIGNING_KEY', 'AURAGO_RELEASE_PUBLIC_KEY', 'AURAGO_ALLOW_UNSIGNED_RELEASES')) {
    $saved[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
}
try {
    $env:COSIGN_PASSWORD = 'fixture-only'
    $prefix = Join-Path $fixture 'fixture'
    & cosign generate-key-pair --output-key-prefix $prefix
    if ($LASTEXITCODE -ne 0) { throw 'Cannot generate fixture signing key.' }
    $manifest = Join-Path $fixture 'SHA256SUMS'
    [IO.File]::WriteAllText($manifest, ((('0' * 64) + "  fixture.bin`n")), [Text.UTF8Encoding]::new($false))
    $env:AURAGO_RELEASE_SIGNING_KEY = "$prefix.key"
    $env:AURAGO_RELEASE_PUBLIC_KEY = "$prefix.pub"
    $env:AURAGO_ALLOW_UNSIGNED_RELEASES = $null
    if ($env:OS -eq 'Windows_NT') {
        & (Join-Path $PSScriptRoot 'sign-release-checksums.ps1') -Manifest $manifest
    } else {
        & bash (Join-Path $PSScriptRoot 'sign-release-checksums.sh') $manifest
        if ($LASTEXITCODE -ne 0) { throw 'Linux manifest signer failed.' }
    }
    & cosign verify-blob --key "$prefix.pub" --insecure-ignore-tlog --signature "$manifest.sig" $manifest
    if ($LASTEXITCODE -ne 0) { throw 'Fixture signature was not valid.' }
    [IO.File]::AppendAllText($manifest, "tampered`n")
    & cosign verify-blob --key "$prefix.pub" --insecure-ignore-tlog --signature "$manifest.sig" $manifest 2>$null
    if ($LASTEXITCODE -eq 0) { throw 'Altered checksum manifest passed verification.' }
    Write-Host 'Offline release fixture passed: signed manifest accepted; altered manifest rejected.'
} finally {
    foreach ($name in $saved.Keys) { [Environment]::SetEnvironmentVariable($name, $saved[$name], 'Process') }
    $resolved = [IO.Path]::GetFullPath($fixture)
    if (-not $resolved.StartsWith($scratchRoot + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) { throw 'Fixture cleanup escaped its scratch directory.' }
    Remove-Item -LiteralPath $resolved -Recurse -Force
}
exit 0
