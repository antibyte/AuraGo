param(
    [Parameter(Mandatory = $true)]
    [string]$BuildImage,
    [Parameter(Mandatory = $true)]
    [ValidateSet('cpu', 'cuda', 'vulkan')]
    [string]$Backend
)
$ErrorActionPreference = 'Stop'
if ($BuildImage -notmatch '^[^@\s]+@sha256:[0-9a-f]{64}$') {
    throw 'BuildImage must be an immutable name@sha256:<64 lowercase hex> reference.'
}
if (-not (Get-Command docker -ErrorAction SilentlyContinue)) { throw 'Docker CLI is unavailable.' }
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$image = "aurago-decisions-runtime:$Backend"
$inspect = & docker image inspect $BuildImage 2>$null
if ($LASTEXITCODE -ne 0 -or -not $inspect) { throw 'The exact build image is not present locally; refusing to pull it.' }
& docker build --pull=false --file (Join-Path $repo 'decisions_runtime/Dockerfile') `
    --build-arg "AURAGO_BUILD_IMAGE=$BuildImage" `
    --build-arg "AURAGO_RUNTIME_BACKEND=$Backend" `
    --tag $image $repo
if ($LASTEXITCODE -ne 0) { throw "Docker source build failed ($LASTEXITCODE)" }
$metadata = & docker image inspect --format '{{.Id}} {{json .Config.Labels}}' $image
if ($LASTEXITCODE -ne 0) { throw 'Could not inspect the built local runtime image.' }
Write-Host "Built local development image $image"
Write-Host $metadata
