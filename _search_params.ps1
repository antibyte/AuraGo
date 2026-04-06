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
        
        # Track raw string literals
        if ($line -match '`' -and -not $inRawString) { $inRawString = $true; continue }
        if ($inRawString) {
            if ($line -match '`') { $inRawString = $false }
            continue
        }
        
        # Only look at function lines
        if ($line -notmatch '^\s*func\s') { continue }
        
        # Remove inline comments
        $stripped = $line -replace '//.*$', ''
        
        # Check if we're not in a string at each match
        function CountQuotesBefore($text, $pos) {
            $count = 0
            for ($i = 0; $i -lt $pos -and $i -lt $text.Length; $i++) {
                if ($text[$i] -eq [char]34) { $count++ }
            }
            return $count
        }
        
        # Match function parameters with snake_case names followed by a type
        # Pattern: (snake_name Type or snake_name *Type
        $paramPattern = '\b([a-z][a-z0-9]*_[a-z][a-z0-9]*(?:_[a-z][a-z0-9]*)*)\s+\*?([A-Z]\w*|\*?\w+\.\w+)'
        $mc = [regex]::Matches($stripped, $paramPattern)
        foreach ($m in $mc) {
            $paramName = $m.Groups[1].Value
            $typeName = $m.Groups[2].Value
            $qc = CountQuotesBefore $stripped $m.Index
            if ($qc % 2 -eq 0) {
                # Make sure we're inside the parameter list (after opening paren)
                $beforeMatch = $stripped.Substring(0, $m.Index)
                $openParens = ($beforeMatch.ToCharArray() | Where-Object { $_ -eq '(' }).Count
                $closeParens = ($beforeMatch.ToCharArray() | Where-Object { $_ -eq ')' }).Count
                if ($openParens -gt $closeParens) {
                    Write-Output ("PARAM|{0}|{1}|{2}|{3}" -f $file.Name, $lineNum, $paramName, $line.Trim())
                }
            }
        }
    }
}
