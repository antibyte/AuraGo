$ErrorActionPreference = "Stop"

Set-StrictMode -Version Latest

$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $root

$langs = @("cs","da","de","el","en","es","fr","hi","it","ja","nl","no","pl","pt","sv","zh")

$checks = @(
    @{ rel = "ui\\lang\\help"; key = "help.llm.multimodal" },
    @{ rel = "ui\\lang\\help"; key = "help.llm.multimodal_provider_types_extra" },
    @{ rel = "ui\\lang\\config\\misc"; key = "config.llm.multimodal_banner" }
)

function Read-JsonMap([string]$path) {
    $raw = Get-Content -Raw -Path $path
    $obj = ($raw | ConvertFrom-Json)
    $ht = @{}
    $obj.PSObject.Properties | ForEach-Object { $ht[$_.Name] = $_.Value }
    return $ht
}

$failed = $false

foreach ($c in $checks) {
    $enPath = Join-Path $c.rel "en.json"
    $en = Read-JsonMap $enPath
    if (-not $en.ContainsKey($c.key)) { throw "Missing key '$($c.key)' in $enPath" }
    $enVal = [string]$en[$c.key]
    if ([string]::IsNullOrWhiteSpace($enVal)) { throw "Empty key '$($c.key)' in $enPath" }

    foreach ($l in $langs) {
        $p = Join-Path $c.rel ($l + ".json")
        $m = Read-JsonMap $p
        if (-not $m.ContainsKey($c.key)) {
            Write-Host "Missing: $p -> $($c.key)" -ForegroundColor Red
            $failed = $true
            continue
        }
        $v = [string]$m[$c.key]
        if ([string]::IsNullOrWhiteSpace($v)) {
            Write-Host "Empty: $p -> $($c.key)" -ForegroundColor Red
            $failed = $true
            continue
        }
        if ($l -ne "en" -and $v -eq $enVal) {
            Write-Host "English leakage: $p -> $($c.key)" -ForegroundColor Red
            $failed = $true
        }
    }
}

if ($failed) {
    throw "i18n lint failed"
}

Write-Host "i18n lint ok" -ForegroundColor Green
