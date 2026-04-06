$files = Get-ChildItem -Path 'C:\Users\Andi\Documents\repo\AuraGo\internal\tools' -Filter '*.go' -Recurse | Where-Object { $_.Name -notmatch '_test\.go$' }

foreach ($file in $files) {
    $lineNum = 0
    $inBlockComment = $false
    $inStruct = $false
    $lines = Get-Content $file.FullName
    foreach ($line in $lines) {
        $lineNum++
        
        # Track block comments
        if ($line -match '/\*') { $inBlockComment = $true }
        if ($inBlockComment) { 
            if ($line -match '\*/') { $inBlockComment = $false }
            continue 
        }
        
        # Track struct boundaries
        if ($line -match '^\s*type\s+\w+\s+struct\s*\{') { $inStruct = $true; continue }
        if ($inStruct -and $line -match '^\s*\}') { $inStruct = $false; continue }
        
        # Inside struct, look for snake_case field names
        if ($inStruct) {
            # Skip comment lines
            if ($line -match '^\s*//') { continue }
            
            # Check for snake_case field name (must be Go identifier followed by type)
            if ($line -match '^\s+([a-z]\w*_[a-z]\w*(?:_[a-z]\w*)*)\s+(\*?\w+)') {
                $fieldName = $Matches[1]
                $typePart = $Matches[2]
                # Make sure the type looks like a Go type (not a string, etc.)
                if ($typePart -match '^[A-Z*]') {
                    Write-Output ("FIELD {0}:{1}: {2} (field: {3})" -f $file.FullName, $lineNum, $line.Trim(), $fieldName)
                }
            }
        }
    }
}
