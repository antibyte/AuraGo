# Local fixture only: no publication, OIDC login, provider calls or operator keys.
$ErrorActionPreference = 'Stop'
$isWindowsHost = [Environment]::OSVersion.Platform -eq 'Win32NT'
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$scratchRoot = [IO.Path]::GetFullPath((Join-Path $repo 'disposable'))
$fixture = Join-Path $scratchRoot ("_release-signing-" + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $fixture -Force | Out-Null
$saved = @{}
foreach ($name in @('COSIGN_PASSWORD', 'AURAGO_RELEASE_SIGNING_KEY', 'AURAGO_RELEASE_PUBLIC_KEY', 'AURAGO_ALLOW_UNSIGNED_RELEASES', 'LOCALAPPDATA', 'GITHUB_ACTIONS', 'PATH')) {
    $saved[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
}
function Assert-FixtureRejects([scriptblock]$Action, [string]$Name) {
    $rejected = $false
    try { & $Action } catch { $rejected = $true }
    if (-not $rejected) { throw "Invalid signing fixture accepted: $Name" }
}
try {
    if ($isWindowsHost -and -not (Get-Command cosign -CommandType Application -ErrorAction SilentlyContinue) -and $env:LOCALAPPDATA) {
        $localTools = Join-Path $env:LOCALAPPDATA 'AuraGo\release-signing'
        if (Test-Path -LiteralPath (Join-Path $localTools 'cosign.exe') -PathType Leaf) { $env:PATH = $localTools + [IO.Path]::PathSeparator + $env:PATH }
    }
    $env:COSIGN_PASSWORD = 'fixture-only'
    $prefix = Join-Path $fixture 'fixture'
    & cosign generate-key-pair --output-key-prefix $prefix
    if ($LASTEXITCODE -ne 0) { throw 'Cannot generate fixture signing key.' }
    $manifest = Join-Path $fixture 'SHA256SUMS'
    [IO.File]::WriteAllText($manifest, ((('0' * 64) + "  fixture.bin`n")), [Text.UTF8Encoding]::new($false))
    $env:AURAGO_RELEASE_SIGNING_KEY = "$prefix.key"
    $env:AURAGO_RELEASE_PUBLIC_KEY = "$prefix.pub"
    $env:AURAGO_ALLOW_UNSIGNED_RELEASES = $null
    if ($isWindowsHost) {
        & (Join-Path $PSScriptRoot 'sign-release-checksums.ps1') -Manifest $manifest
    } else {
        & bash (Join-Path $PSScriptRoot 'sign-release-checksums.sh') $manifest
        if ($LASTEXITCODE -ne 0) { throw 'Linux manifest signer failed.' }
    }
    & cosign verify-blob --key "$prefix.pub" --insecure-ignore-tlog --signature "$manifest.sig" $manifest
    if ($LASTEXITCODE -ne 0) { throw 'Fixture signature was not valid.' }
    if ($isWindowsHost) {
        $gitBash = Join-Path ([Environment]::GetFolderPath('ProgramFiles')) 'Git\bin\bash.exe'
        if (Test-Path -LiteralPath $gitBash) {
            & $gitBash ((Join-Path $PSScriptRoot 'sign-release-checksums.sh') -replace '\\', '/') ($manifest -replace '\\', '/')
            if ($LASTEXITCODE -ne 0) { throw 'Git Bash manifest signer failed.' }
            & cosign verify-blob --key "$prefix.pub" --insecure-ignore-tlog --signature "$manifest.sig" $manifest
            if ($LASTEXITCODE -ne 0) { throw 'Git Bash fixture signature was not valid.' }
        }
    }
    [IO.File]::AppendAllText($manifest, "tampered`n")
    $ErrorActionPreference = 'Continue'
    & cosign verify-blob --key "$prefix.pub" --insecure-ignore-tlog --signature "$manifest.sig" $manifest 2>$null
    $ErrorActionPreference = 'Stop'
    if ($LASTEXITCODE -eq 0) { throw 'Altered checksum manifest passed verification.' }
    Write-Host 'Offline release fixture passed: signed manifest accepted; altered manifest rejected.'
    if ($isWindowsHost) {
        $signer = Join-Path $PSScriptRoot 'sign-release-checksums.ps1'
        $env:LOCALAPPDATA = Join-Path $fixture 'local-appdata'
        $profile = Join-Path $env:LOCALAPPDATA 'AuraGo\release-signing'
        New-Item -ItemType Directory -Path $profile -Force | Out-Null
        Copy-Item -LiteralPath (Get-Command cosign -CommandType Application | Select-Object -First 1).Source -Destination (Join-Path $profile 'cosign.exe')
        Copy-Item -LiteralPath "$prefix.key" -Destination (Join-Path $profile 'release.key')
        Copy-Item -LiteralPath "$prefix.pub" -Destination (Join-Path $profile 'release.pub')
        $secure = ConvertTo-SecureString 'fixture-only' -AsPlainText -Force
        try { $encrypted = ConvertFrom-SecureString $secure } finally { $secure.Dispose() }
        $passwordFile = Join-Path $profile 'password.dpapi'
        [IO.File]::WriteAllText($passwordFile, $encrypted)
        $env:COSIGN_PASSWORD = $null
        $env:AURAGO_RELEASE_SIGNING_KEY = $null
        $env:AURAGO_RELEASE_PUBLIC_KEY = $null
        $env:GITHUB_ACTIONS = $null
        $oldSignature = [IO.File]::ReadAllText("$manifest.sig")
        & $signer -CheckPrerequisites
        if ([IO.File]::ReadAllText("$manifest.sig") -ne $oldSignature -or $env:COSIGN_PASSWORD) { throw 'Preflight modified artifacts or exposed the profile password.' }
        $profileManifest = Join-Path $fixture 'SHA256SUMS.profile'
        [IO.File]::WriteAllText($profileManifest, (('0' * 64) + "  fixture.bin`n"))
        & $signer -Manifest $profileManifest
        if ($env:COSIGN_PASSWORD -or $env:AURAGO_RELEASE_SIGNING_KEY -or $env:AURAGO_RELEASE_PUBLIC_KEY) { throw 'Signing profile credentials leaked into the caller environment.' }
        & cosign verify-blob --key "$prefix.pub" --insecure-ignore-tlog --signature "$profileManifest.sig" $profileManifest
        if ($LASTEXITCODE -ne 0) { throw 'Protected profile signature was not valid.' }
        $keyHash = (Get-FileHash -LiteralPath (Join-Path $profile 'release.key') -Algorithm SHA256).Hash
        & (Join-Path $PSScriptRoot 'setup-release-signing.ps1')
        if ((Get-FileHash -LiteralPath (Join-Path $profile 'release.key') -Algorithm SHA256).Hash -ne $keyHash) { throw 'Repeated setup rotated an existing signing key.' }
        $processPath = $env:PATH
        try {
            $env:PATH = [Environment]::SystemDirectory
            & (Join-Path ([Environment]::SystemDirectory) 'WindowsPowerShell\v1.0\powershell.exe') -NoProfile -ExecutionPolicy Bypass -File $signer -CheckPrerequisites
            if ($LASTEXITCODE -ne 0) { throw 'Profile preflight failed in a fresh Windows PowerShell process without Cosign on PATH.' }
        } finally { $env:PATH = $processPath }
        [IO.File]::WriteAllText($passwordFile, 'invalid-dpapi-fixture')
        Assert-FixtureRejects { & $signer -CheckPrerequisites } 'corrupt protected password'
        Assert-FixtureRejects { & $signer -Manifest $profileManifest } 'corrupt profile during signing'
        if ($env:COSIGN_PASSWORD -or (Test-Path -LiteralPath "$profileManifest.sig.pending")) { throw 'Failed signing retained a password or pending signature.' }
        $env:COSIGN_PASSWORD = 'fixture-only'
        $env:AURAGO_RELEASE_SIGNING_KEY = "$prefix.key"
        $env:AURAGO_RELEASE_PUBLIC_KEY = "$prefix.pub"
        $env:PATH = $profile + [IO.Path]::PathSeparator + $processPath
        & $signer -CheckPrerequisites
        if ($env:COSIGN_PASSWORD -ne 'fixture-only') { throw 'Explicit signing configuration was overwritten.' }
        $env:PATH = $processPath
        $env:COSIGN_PASSWORD = $null
        $env:AURAGO_RELEASE_SIGNING_KEY = $null
        $env:AURAGO_RELEASE_PUBLIC_KEY = $null
        [IO.File]::WriteAllText($passwordFile, $encrypted)
        Remove-Item -LiteralPath (Join-Path $profile 'release.key')
        Assert-FixtureRejects { & $signer -CheckPrerequisites } 'missing profile key'
        Assert-FixtureRejects { & (Join-Path $PSScriptRoot 'setup-release-signing.ps1') } 'incomplete existing profile'
        if (Test-Path -LiteralPath (Join-Path $profile 'release.key')) { throw 'Incomplete profile caused implicit key rotation.' }
        $env:LOCALAPPDATA = Join-Path $fixture 'unconfigured-appdata'
        Assert-FixtureRejects { & $signer -CheckPrerequisites } 'unconfigured signing'
        $env:AURAGO_ALLOW_UNSIGNED_RELEASES = '1'
        & $signer -CheckPrerequisites
        [IO.File]::WriteAllText("$profileManifest.sig", 'stale fixture')
        [IO.File]::WriteAllText("$profileManifest.pem", 'stale fixture')
        & $signer -Manifest $profileManifest
        if ((Test-Path -LiteralPath "$profileManifest.sig") -or (Test-Path -LiteralPath "$profileManifest.pem")) { throw 'Legacy unsigned release retained signature metadata.' }
        $env:AURAGO_ALLOW_UNSIGNED_RELEASES = $null
        $env:GITHUB_ACTIONS = 'true'
        & $signer -CheckPrerequisites
        Write-Host 'Windows protected-profile fixture passed: automatic discovery, isolated preflight, credential cleanup, explicit precedence, stable keys and fail-closed errors.'
    }
} finally {
    foreach ($name in $saved.Keys) { [Environment]::SetEnvironmentVariable($name, $saved[$name], 'Process') }
    $resolved = [IO.Path]::GetFullPath($fixture)
    if (-not $resolved.StartsWith($scratchRoot + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) { throw 'Fixture cleanup escaped its scratch directory.' }
    Remove-Item -LiteralPath $resolved -Recurse -Force
}
exit 0
