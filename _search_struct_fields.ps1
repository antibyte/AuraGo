$files = Get-ChildItem -Path 'C:\Users\Andi\Documents\repo\AuraGo\internal\tools' -Filter '*.go' -Recurse | Where-Object { $_.Name -notmatch '_test\.go$' }

foreach ($file in $files) {
    $lines = Get-Content $file.FullName
    $lineNum = 0
    $inBlock = $false
    $inRawString = $false
    $inStruct = $false
    
    foreach ($line in $lines) {
        $lineNum++
        
        # Track block comments
        if ($line -match '/\*') { $inBlock = $true }
        if ($inBlock) { 
            if ($line -match '\*/') { $inBlock = $false }
            continue 
        }
        
        # Track raw string literals
        if ($line -match '`' -and -not $inRawString) { $inRawString = $true; continue }
        if ($inRawString) {
            if ($line -match '`') { $inRawString = $false }
            continue
        }
        
        # Skip pure comment lines
        if ($line -match '^\s*//') { continue }
        
        # Track struct boundaries
        # Match: type Name struct {
        if ($line -match '^\s*type\s+\w+\s+struct\s*\{') { 
            $inStruct = $true
            continue 
        }
        if ($inStruct -and $line -match '^\s*\}\s*$') { 
            $inStruct = $false
            continue 
        }
        
        if ($inStruct) {
            # Remove inline comments
            $stripped = $line -replace '//.*$', ''
            
            # Match: snake_case_name Type (Go field declaration)
            if ($stripped -match '^\s+([a-z][a-z0-9]*_[a-z][a-z0-9]*(?:_[a-z][a-z0-9]*)*)\s+(\*?[A-Z]\w*|\*?\w+\.\w+|\[\]|\[)\s*') {
                $fieldName = $Matches[1]
                Write-Output ("FIELD|{0}|{1}|{2}|{3}" -f $file.Name, $lineNum, $fieldName, $line.Trim())
            }
        }
    }
}
