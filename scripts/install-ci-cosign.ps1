# CI-only, checksum-pinned tool provisioning. Never creates or reads signing keys.
$ErrorActionPreference = 'Stop'
if ($env:RUNNER_ARCH -ne 'X64') { throw 'CI Cosign installer requires an X64 runner.' }
switch ($env:RUNNER_OS) {
    'Windows' {
        $asset = 'cosign-windows-amd64.exe'
        $name = 'cosign.exe'
        $size = 191066624
        $sha = 'cbe953787074611f1334cbf37dc1cf953995bb8c45398eb4fc7bb781d29c1f2a'
    }
    'Linux' {
        $asset = 'cosign-linux-amd64'
        $name = 'cosign'
        $size = 145688404
        $sha = '424e2c97d0b45fd7a6b608a87f090f35633f4c23ece5f092d48bba11aaf4bbfa'
    }
    default { throw 'Unsupported CI Cosign platform.' }
}
$directory = Join-Path $env:RUNNER_TEMP 'aurago-cosign'
New-Item -ItemType Directory -Path $directory -Force | Out-Null
$pending = Join-Path $directory ($name + '.pending')
$destination = Join-Path $directory $name
try {
    gh release download v3.1.4 --repo sigstore/cosign --pattern $asset --output $pending
    if ($LASTEXITCODE -ne 0) { throw 'Pinned Cosign download failed.' }
    if ((Get-Item -LiteralPath $pending).Length -ne $size) { throw 'Cosign size mismatch.' }
    if ((Get-FileHash -LiteralPath $pending -Algorithm SHA256).Hash.ToLowerInvariant() -ne $sha) { throw 'Cosign SHA-256 mismatch.' }
    if ($env:RUNNER_OS -eq 'Linux') {
        & chmod '0755' -- $pending
        if ($LASTEXITCODE -ne 0) { throw 'Cannot mark verified Cosign executable.' }
    }
    Move-Item -LiteralPath $pending -Destination $destination -Force
    [IO.File]::AppendAllText($env:GITHUB_PATH, $directory + [Environment]::NewLine, [Text.UTF8Encoding]::new($false))
    Write-Host "Cosign v3.1.4 verified for $env:RUNNER_OS."
} finally {
    if (Test-Path -LiteralPath $pending) { Remove-Item -LiteralPath $pending -Force }
}
