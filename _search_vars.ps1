$files = Get-ChildItem -Path 'C:\Users\Andi\Documents\repo\AuraGo\internal\tools' -Filter '*.go' -Recurse | Where-Object { $_.Name -notmatch '_test\.go$' }

foreach ($file in $files) {
    $lines = Get-Content $file.FullName
    $lineNum = 0
    $inBlock = $false
    $inRawString = $false
    
    foreach ($line in $lines) {
        $lineNum++
        
        # Track block comments
        if ($line -match '/\*') { $inBlock = $true }
        if ($inBlock) { 
            if ($line -match '\*/') { $inBlock = $false }
            continue 
        }
        
        # Track raw string literals (backtick strings in Go)
        if ($line -match '`' -and -not $inRawString) { $inRawString = $true; continue }
        if ($inRawString) {
            if ($line -match '`') { $inRawString = $false }
            continue
        }
        
        # Skip pure comment lines
        if ($line -match '^\s*//') { continue }
        
        # Remove inline comments  
        $stripped = $line -replace '//.*$', ''
        
        # Count quotes before the match position to check we're not in a string
        function Test-NotInString($text, $pos) {
            $dquoteCount = 0
            for ($i = 0; $i -lt $pos -and $i -lt $text.Length; $i++) {
                if ($text[$i] -eq '"') { $dquoteCount++ }
            }
            return ($dquoteCount % 2 -eq 0)
        }
        
        # Pattern 1: snake_name := (short variable declaration)
        $mc = [regex]::Matches($stripped, '(?:^|,|\s)\s*([a-z]\w*_[a-z]\w*(?:_[a-z]\w*)*)\s*:=')
        foreach ($m in $mc) {
            if (Test-NotInString $stripped $m.Index) {
                Write-Output ("SHORTVAR|{0}|{1}|{2}|{3}" -f $file.Name, $lineNum, $m.Groups[1].Value, $line.Trim())
            }
        }
        
        # Pattern 2: var snake_name (with or without type/assignment)
        $mc2 = [regex]::Matches($stripped, '\bvar\s+([a-z]\w*_[a-z]\w*(?:_[a-z]\w*)*)\b')
        foreach ($m in $mc2) {
            if (Test-NotInString $stripped $m.Index) {
                Write-Output ("VAR|{0}|{1}|{2}|{3}" -f $file.Name, $lineNum, $m.Groups[1].Value, $line.Trim())
            }
        }
        
        # Pattern 3: Inside var() block - indented snake_name = or snake_name Type
        if ($stripped -match '^\s+([a-z]\w*_[a-z]\w*(?:_[a-z]\w*)*)\s*=' -and $stripped -notmatch '^\s+//') {
            $m = [regex]::Match($stripped, '^\s+([a-z]\w*_[a-z]\w*(?:_[a-z]\w*)*)\s*=')
            if (Test-NotInString $stripped $m.Index) {
                Write-Output ("BLOCKVAR|{0}|{1}|{2}|{3}" -f $file.Name, $lineNum, $m.Groups[1].Value, $line.Trim())
            }
        }
    }
}
