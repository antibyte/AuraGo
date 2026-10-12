param(
    [Parameter(Mandatory = $true)]
    [ValidateSet('cpu', 'cuda', 'vulkan', 'metal')]
    [string]$Backend,
    [string]$Target = '',
    [string]$BuildType = 'Release',
    [string]$Generator = ''
)
$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$source = Join-Path $repo 'decisions_runtime'
if (-not $Target) {
    $os = if ($IsWindows -or $env:OS -eq 'Windows_NT') { 'windows' } elseif ($IsMacOS) { 'darwin' } else { 'linux' }
    $arch = if ([Environment]::Is64BitOperatingSystem) { 'amd64' } else { '386' }
    if ($env:PROCESSOR_ARCHITECTURE -match 'ARM64') { $arch = 'arm64' }
    $Target = "$os-$arch"
}
$runtimeDir = Join-Path $repo "bin/decisions-runtime/$Target/$Backend"
$buildDir = Join-Path $repo "bin/decisions-runtime/build-$Target-$Backend"
New-Item -ItemType Directory -Force $runtimeDir, $buildDir | Out-Null
$configure = @('-S', $source, '-B', $buildDir, ('-DAURAGO_RUNTIME_BACKEND=' + $Backend), ('-DAURAGO_RUNTIME_OUTPUT_DIR=' + $runtimeDir), ('-DCMAKE_BUILD_TYPE=' + $BuildType), '-DCMAKE_TRY_COMPILE_CONFIGURATION=Release')
if ($Generator) { $configure += @('-G', $Generator) }
$localSource = Join-Path $repo 'bin/decisions-runtime/llama-src'
if (Test-Path (Join-Path $localSource '.git')) { $configure += ('-DFETCHCONTENT_SOURCE_DIR_LLAMA_CPP=' + $localSource) }
& cmake @configure
if ($LASTEXITCODE -ne 0) { throw "CMake configure failed ($LASTEXITCODE)" }
& cmake --build $buildDir --config $BuildType --parallel 4
if ($LASTEXITCODE -ne 0) { throw "Native runtime build failed ($LASTEXITCODE)" }
$exeName = if ($Target.StartsWith('windows-')) { 'aurago-decisions-runtime.exe' } else { 'aurago-decisions-runtime' }
$binary = Join-Path $runtimeDir $exeName
if (-not (Test-Path -LiteralPath $binary -PathType Leaf)) { throw "Expected runtime binary missing: $binary" }
& $binary --self-test-protocol
if ($LASTEXITCODE -ne 0) { throw "Runtime protocol self-test failed ($LASTEXITCODE)" }
$discoverText = & $binary --discover
if ($LASTEXITCODE -ne 0) { throw "Runtime discovery self-test failed ($LASTEXITCODE)" }
$discover = $discoverText | ConvertFrom-Json
if ($discover.protocol -ne 'aurago.decisions.v1' -or $discover.build.llama_cpp_commit -ne 'd81235049384534c167caea52b85a694f6103d14' -or $discover.devices.Count -lt 1) {
    throw 'Runtime discovery output did not match the pinned protocol.'
}
if (-not (Get-Command python -ErrorAction SilentlyContinue)) { throw 'Python 3 is required to write the release manifest.' }
& python (Join-Path $PSScriptRoot 'write-decisions-runtime-manifest.py') --binary $binary --target $Target --backend $Backend
if ($LASTEXITCODE -ne 0) { throw "Release manifest generation failed ($LASTEXITCODE)" }
Write-Host "Built and locally verified $binary"
