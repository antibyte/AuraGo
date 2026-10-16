param([ValidateSet('amd64','arm64')][string]$Architecture)
$ErrorActionPreference = 'Stop'
$version = '1.4.321.1'
$platform = if ($Architecture -eq 'arm64') { 'warm' } else { 'windows' }
$expected = if ($Architecture -eq 'arm64') { 'd6a6f9eceb3447acb4556a458132793201125d7877c533ce0b436fadf3005004' } else { 'baaa4f7ca11ed3d82aa1c102b21208915485bbaa473068c763daa425cca468bd' }
$installer = Join-Path $env:RUNNER_TEMP 'decisions-vulkan-sdk.exe'
Invoke-WebRequest "https://sdk.lunarg.com/sdk/download/$version/$platform/vulkan_sdk.exe" -OutFile $installer
if ((Get-FileHash -LiteralPath $installer -Algorithm SHA256).Hash.ToLowerInvariant() -ne $expected) { throw 'Vulkan SDK digest mismatch' }
$sdk = Join-Path $env:RUNNER_TEMP 'decisions-vulkan-sdk'
$process = Start-Process -FilePath $installer -ArgumentList @('--root', $sdk, '--accept-licenses', '--default-answer', '--confirm-command', 'install') -WindowStyle Hidden -PassThru -Wait
if ($process.ExitCode -ne 0) { throw "Vulkan SDK installation failed: $($process.ExitCode)" }
"VULKAN_SDK=$sdk" | Out-File -FilePath $env:GITHUB_ENV -Encoding utf8 -Append
(Join-Path $sdk 'Bin') | Out-File -FilePath $env:GITHUB_PATH -Encoding utf8 -Append
