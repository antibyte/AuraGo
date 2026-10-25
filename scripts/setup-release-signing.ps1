#Requires -Version 5.1
# Creates a Windows-user-bound signing profile outside the checkout. Never rotates existing keys.
$ErrorActionPreference = 'Stop'
if ([Environment]::OSVersion.Platform -ne 'Win32NT' -or -not [Environment]::Is64BitOperatingSystem -or -not $env:LOCALAPPDATA) {
    throw 'This setup requires 64-bit Windows and LOCALAPPDATA. Other hosts can configure Cosign keys manually.'
}
$signer = Join-Path $PSScriptRoot 'sign-release-checksums.ps1'
$root = [IO.Path]::GetFullPath((Join-Path $env:LOCALAPPDATA 'AuraGo'))
$profile = Join-Path $root 'release-signing'
if (Test-Path -LiteralPath $profile) {
    foreach ($name in @('cosign.exe','release.key','release.pub','password.dpapi')) {
        if (-not (Test-Path -LiteralPath (Join-Path $profile $name) -PathType Leaf)) { throw 'Existing signing profile is incomplete; restore it without replacing the original keys.' }
    }
    $secure = ConvertTo-SecureString ([IO.File]::ReadAllText((Join-Path $profile 'password.dpapi')))
    try { if (-not $secure.Length) { throw 'Existing signing profile password is empty.' } } finally { $secure.Dispose() }
    Write-Host "Existing signing profile retained: $profile"
    return
}
New-Item -ItemType Directory -Path $root -Force | Out-Null
$stage = Join-Path $root ('release-signing-' + [guid]::NewGuid().ToString('N') + '.pending')
New-Item -ItemType Directory -Path $stage | Out-Null
$saved = @{}
foreach ($name in @('COSIGN_PASSWORD','AURAGO_RELEASE_SIGNING_KEY','AURAGO_RELEASE_PUBLIC_KEY','AURAGO_ALLOW_UNSIGNED_RELEASES','PATH')) {
    $saved[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
}
try {
    $acl = New-Object Security.AccessControl.DirectorySecurity
    $acl.SetAccessRuleProtection($true, $false)
    $user = [Security.Principal.WindowsIdentity]::GetCurrent().User
    $acl.SetOwner($user)
    foreach ($sid in @($user, [Security.Principal.SecurityIdentifier]::new('S-1-5-18'))) {
        $acl.AddAccessRule([Security.AccessControl.FileSystemAccessRule]::new($sid, 'FullControl', 'ContainerInherit, ObjectInherit', 'None', 'Allow'))
    }
    Set-Acl -LiteralPath $stage -AclObject $acl
    $expectedHash = 'cbe953787074611f1334cbf37dc1cf953995bb8c45398eb4fc7bb781d29c1f2a'
    $expectedSize = 191066624
    $cosign = Join-Path $stage 'cosign.exe'
    $installed = Get-Command cosign -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($installed -and (Get-FileHash -LiteralPath $installed.Source -Algorithm SHA256).Hash -eq $expectedHash) {
        Copy-Item -LiteralPath $installed.Source -Destination $cosign
    } else {
        & gh release download v3.1.4 --repo sigstore/cosign --pattern cosign-windows-amd64.exe --output $cosign
        if ($LASTEXITCODE -ne 0) { throw 'Cannot download the pinned Cosign v3.1.4 binary.' }
    }
    if ((Get-Item -LiteralPath $cosign).Length -ne $expectedSize -or (Get-FileHash -LiteralPath $cosign -Algorithm SHA256).Hash -ne $expectedHash) {
        throw 'Cosign binary size or SHA-256 does not match the official v3.1.4 release.'
    }
    $bytes = New-Object byte[] 32
    $random = [Security.Cryptography.RandomNumberGenerator]::Create()
    try { $random.GetBytes($bytes) } finally { $random.Dispose() }
    $env:COSIGN_PASSWORD = [Convert]::ToBase64String($bytes)
    $secure = ConvertTo-SecureString $env:COSIGN_PASSWORD -AsPlainText -Force
    try { [IO.File]::WriteAllText((Join-Path $stage 'password.dpapi'), (ConvertFrom-SecureString $secure), [Text.Encoding]::ASCII) }
    finally { $secure.Dispose() }
    $prefix = Join-Path $stage 'release'
    & $cosign generate-key-pair --output-key-prefix $prefix
    if ($LASTEXITCODE -ne 0) { throw 'Cannot generate the encrypted release signing key.' }
    $env:AURAGO_RELEASE_SIGNING_KEY = "$prefix.key"
    $env:AURAGO_RELEASE_PUBLIC_KEY = "$prefix.pub"
    $env:AURAGO_ALLOW_UNSIGNED_RELEASES = $null
    $env:PATH = $stage + [IO.Path]::PathSeparator + $saved['PATH']
    $probe = Join-Path $stage 'SHA256SUMS.check'
    [IO.File]::WriteAllText($probe, (('0' * 64) + "  profile-check.bin`n"), [Text.UTF8Encoding]::new($false))
    & $signer -Manifest $probe
    Remove-Item -LiteralPath $probe, "$probe.sig" -Force
    # The completed directory becomes visible only after a real sign/verify round trip.
    foreach ($path in @($stage, $profile)) {
        if (-not [IO.Path]::GetFullPath($path).StartsWith($root.TrimEnd('\') + '\', [StringComparison]::OrdinalIgnoreCase)) { throw 'Signing profile move escaped its application-data directory.' }
    }
    [IO.Directory]::Move($stage, $profile)
    Write-Host "Release signing profile ready: $profile" -ForegroundColor Green
    Write-Host ('Trusted public-key SHA-256: ' + (Get-FileHash -LiteralPath (Join-Path $profile 'release.pub') -Algorithm SHA256).Hash.ToLowerInvariant())
} finally {
    foreach ($name in $saved.Keys) { [Environment]::SetEnvironmentVariable($name, $saved[$name], 'Process') }
    if (Test-Path -LiteralPath $stage) {
        if (-not [IO.Path]::GetFullPath($stage).StartsWith($root.TrimEnd('\') + '\', [StringComparison]::OrdinalIgnoreCase) -or ((Get-Item -LiteralPath $stage).Attributes -band [IO.FileAttributes]::ReparsePoint)) {
            throw 'Refusing signing setup cleanup outside the private staging directory.'
        }
        Remove-Item -LiteralPath $stage -Recurse -Force
    }
}
