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
        
        # Only look at function declaration lines
        if ($line -match '^\s*func\s') {
            # Skip lines that are purely comments
            if ($line -match '^\s*//') { continue }
            
            # Remove comment portion
            $stripped = $line -replace '//.*$', ''
            
            # Look for function parameters with snake_case
            # Pattern: func Name(snake_name type, ...) or func (r Type) Name(snake_name type)
            # Match: snake_name followed by a type (capital letter or *Type)
            $paramMatches = [regex]::Matches($stripped, '\b([a-z]\w*_[a-z]\w*(?:_[a-z]\w*)*)\s+\*?(\w+)')
            foreach ($m in $paramMatches) {
                $paramName = $m.Groups[1].Value
                $paramType = $m.Groups[2].Value
                # Filter out common non-violations
                if ($paramType -match '^[A-Z]' -and $paramName -ne '_') {
                    # Extra check: make sure it's not inside a string
                    $beforeMatch = $stripped.Substring(0, $m.Index)
                    $quoteCount = ($beforeMatch.ToCharArray() | Where-Object { $_ -eq '"' }).Count
                    if ($quoteCount % 2 -eq 0) {
                        Write-Output ("PARAM {0}:{1}: {2} (param: {3} {4})" -f $file.FullName, $lineNum, $line.Trim(), $paramName, $paramType)
                    }
                }
            }
        }
    }
}
