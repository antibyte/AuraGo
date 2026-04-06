$files = Get-ChildItem -Path 'C:\Users\Andi\Documents\repo\AuraGo\internal\tools' -Filter '*.go' -Recurse | Where-Object { $_.Name -notmatch '_test\.go$' }

foreach ($file in $files) {
    $lineNum = 0
    $inBlockComment = $false
    $lines = Get-Content $file.FullName
    foreach ($line in $lines) {
        $lineNum++
        
        # Skip block comments
        if ($line -match '/\*') { $inBlockComment = $true }
        if ($inBlockComment) { 
            if ($line -match '\*/') { $inBlockComment = $false }
            continue 
        }
        if ($line -match '\*/') { $inBlockComment = $false; continue }
        
        # Skip single-line comments
        $stripped = $line -replace '//.*$', ''
        
        # Skip if inside a string (rough check - line is mostly a string)
        if ($stripped -match '^\s*"' -and $stripped -match '"\s*$' -and $stripped -notmatch '\bvar\b' -and $stripped -notmatch '\bconst\b') { continue }
        
        # Look for var declarations with snake_case
        if ($stripped -match '\bvar\s+([a-z]\w*_[a-z]\w*)\b') {
            Write-Output ("VAR  {0}:{1}: {2}" -f $file.FullName, $lineNum, $line.Trim())
        }
        
        # Look for := declarations with snake_case
        if ($stripped -match '^\s*([a-z]\w*_[a-z]\w*)\s*:=') {
            Write-Output ("SHORTVAR {0}:{1}: {2}" -f $file.FullName, $lineNum, $line.Trim())
        }
        
        # Look for multi-var declarations like: var ( ... snake_name ...
        if ($stripped -match '^\s+([a-z]\w*_[a-z]\w*)\s*=\s*') {
            # Make sure it's not inside a string literal or struct tag
            if ($stripped -notmatch '`"' -and $stripped -notmatch 'json:"' -and $stripped -notmatch 'yaml:"') {
                Write-Output ("MULTIVAR {0}:{1}: {2}" -f $file.FullName, $lineNum, $line.Trim())
            }
        }
    }
}
