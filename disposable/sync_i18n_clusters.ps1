$ErrorActionPreference = "Continue"
Set-StrictMode -Version Latest

$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $root

$langs = @("cs","da","de","el","en","es","fr","hi","it","ja","nl","no","pl","pt","sv","zh")
$categories = @("containers","dashboard","skills")

function Read-JsonMap([string]$path) {
    $raw = Get-Content -Raw -Path $path -ErrorAction SilentlyContinue
    if (-not $raw) { return @{} }
    $obj = ($raw | ConvertFrom-Json)
    $ht = @{}
    $obj.PSObject.Properties | ForEach-Object { $ht[$_.Name] = $_.Value }
    return $ht
}

function Write-JsonMap([string]$path, [hashtable]$data) {
    $obj = [PSCustomObject]$data
    $obj | ConvertTo-Json -Depth 10 | Set-Content -Path $path -Encoding UTF8
}

$totalChanges = 0

foreach ($cat in $categories) {
    Write-Host "Processing $cat..." -ForegroundColor Cyan
    
    # Get English keys as master reference
    $enPath = Join-Path $root "ui/lang/$cat/en.json"
    if (-not (Test-Path $enPath)) {
        Write-Host "  English file not found: $enPath" -ForegroundColor Red
        continue
    }
    $en = Read-JsonMap $enPath
    $enKeys = [string[]]$en.Keys
    
    Write-Host "  Master has $($enKeys.Count) keys" -ForegroundColor Gray
    
    foreach ($lang in $langs) {
        $langPath = Join-Path $root "ui/lang/$cat/$lang.json"
        
        if ($lang -eq "en") { continue }
        
        $langData = Read-JsonMap $langPath
        $langKeys = [string[]]$langData.Keys
        
        $missing = $enKeys | Where-Object { $langKeys -notcontains $_ }
        
        if ($missing) {
            $missingCount = $missing.Count
            Write-Host "  ${lang}: ${missingCount} missing keys" -ForegroundColor Yellow
            
            foreach ($key in $missing) {
                $langData[$key] = $en[$key]  # Use English as placeholder
                $totalChanges++
            }
            
            Write-JsonMap $langPath $langData
            Write-Host "    Updated ${langPath}" -ForegroundColor Green
        }
    }
}

Write-Host ""
if ($totalChanges -gt 0) {
    Write-Host "Total keys added: $totalChanges" -ForegroundColor Green
} else {
    Write-Host "All keys in sync - no changes needed" -ForegroundColor Green
}
