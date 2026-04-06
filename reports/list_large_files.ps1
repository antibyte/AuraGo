$files = Get-ChildItem 'C:\Users\Andi\Documents\repo\AuraGo\internal\agent\*.go' -Exclude '*_test.go'
$results = @()
foreach ($f in $files) {
    $content = Get-Content $f.FullName -Raw
    $size = $content.Length
    $results += [PSCustomObject]@{Name=$f.Name; Size=$size}
}
$results | Sort-Object Size -Descending | Select-Object -First 20 | ForEach-Object { Write-Output "$($_.Name) ($($_.Size) bytes)" }
