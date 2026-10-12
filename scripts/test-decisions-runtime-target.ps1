$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'decisions-runtime-target.ps1')

$processorArchitecture = $env:PROCESSOR_ARCHITECTURE
try {
    Remove-Item Env:PROCESSOR_ARCHITECTURE -ErrorAction SilentlyContinue
    $cases = @(
        [pscustomobject]@{ Architecture = [System.Runtime.InteropServices.Architecture]::X64; Expected = 'amd64' }
        [pscustomobject]@{ Architecture = [System.Runtime.InteropServices.Architecture]::Arm64; Expected = 'arm64' }
        [pscustomobject]@{ Architecture = [System.Runtime.InteropServices.Architecture]::X86; Expected = '386' }
        [pscustomobject]@{ Architecture = [System.Runtime.InteropServices.Architecture]::Arm; Expected = 'arm' }
    )

    foreach ($case in $cases) {
        $actual = Get-DecisionsRuntimeArchitecture $case.Architecture
        if ($actual -ne $case.Expected) { throw "$($case.Architecture) mapped to $actual; expected $($case.Expected)." }
    }

    $unsupportedArchitecture = [Enum]::ToObject([System.Runtime.InteropServices.Architecture], 99)
    try {
        Get-DecisionsRuntimeArchitecture $unsupportedArchitecture | Out-Null
        throw "Unsupported architecture $unsupportedArchitecture was accepted."
    } catch {
        if ($_.Exception.Message -notmatch '^Unsupported native runtime architecture:') { throw }
    }

    $nativeArchitecture = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture
    $nativeExpected = Get-DecisionsRuntimeArchitecture $nativeArchitecture
    if ((Get-DecisionsRuntimeArchitecture) -ne $nativeExpected) { throw 'Default architecture detection differs from the native OS architecture.' }
} finally {
    if ($null -eq $processorArchitecture) { Remove-Item Env:PROCESSOR_ARCHITECTURE -ErrorAction SilentlyContinue }
    else { $env:PROCESSOR_ARCHITECTURE = $processorArchitecture }
}

$buildScript = Join-Path $PSScriptRoot 'build-decisions-runtime.ps1'
$tokens = $null
$parseErrors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile($buildScript, [ref]$tokens, [ref]$parseErrors)
if ($parseErrors.Count -gt 0) { throw "Could not parse $buildScript." }
$architectureCalls = $ast.FindAll({ param($node) $node -is [System.Management.Automation.Language.CommandAst] -and $node.GetCommandName() -eq 'Get-DecisionsRuntimeArchitecture' }, $true)
$targetGuard = $architectureCalls[0]
while ($null -ne $targetGuard -and $targetGuard -isnot [System.Management.Automation.Language.IfStatementAst]) { $targetGuard = $targetGuard.Parent }
if ($architectureCalls.Count -ne 1 -or $null -eq $targetGuard -or $targetGuard.Clauses[0].Item1.Extent.Text -ne '-not $Target') {
    throw 'Automatic architecture detection must remain inside the -Target fallback branch.'
}

Write-Output 'Decisions runtime architecture mapping passed.'
