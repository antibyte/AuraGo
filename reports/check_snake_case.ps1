# Comprehensive snake_case violation checker for Go files
# Checks: variable declarations, struct fields, function parameters
# Excludes: strings, comments, struct tags, function calls, ALL_CAPS constants

$results = @()
$inBlockComment = $false
$inStruct = $false

Get-ChildItem -Path 'internal\server' -Filter '*.go' -Recurse | Where-Object { $_.Name -notmatch '_test\.go$' } | ForEach-Object {
    $file = $_.FullName
    $relPath = $_.FullName.Replace((Get-Location).Path + '\', '')
    $lines = Get-Content $file
    $inBlockComment = $false
    $structDepth = 0
    
    for ($i = 0; $i -lt $lines.Count; $i++) {
        $rawLine = $lines[$i]
        $lineNum = $i + 1
        
        # Track block comments
        if ($rawLine -match '/\*') { $inBlockComment = $true }
        if ($rawLine -match '\*/') { $inBlockComment = $false; continue }
        if ($inBlockComment) { continue }
        
        # Skip single-line comments
        $stripped = $rawLine
        if ($rawLine -match '//') {
            $stripped = $rawLine.Substring(0, $rawLine.IndexOf('//'))
        }
        
        # Skip struct tag lines (lines where the entire non-whitespace content is a backtick string)
        if ($stripped -match '^\s*`[^`]*`\s*$') { continue }
        
        # Remove string literals to avoid false positives
        # Replace double-quoted strings
        $cleaned = [regex]::Replace($stripped, '"(?:[^"\\]|\\.)*"', '""')
        # Replace backtick strings
        $cleaned = [regex]::Replace($cleaned, '`[^`]*`', '``')
        
        # Now check for snake_case identifiers in the cleaned line
        
        # 1. Short variable declarations: snake_name :=
        $matches1 = [regex]::Matches($cleaned, '(?:^|\s)([a-z][a-z0-9]*_[a-z0-9_]+)\s*:=')
        foreach ($m in $matches1) {
            $name = $m.Groups[1].Value
            # Skip ALL_CAPS
            if ($name -cmatch '^[a-z]') {
                $results += [PSCustomObject]@{
                    File = $relPath
                    Line = $lineNum
                    Name = $name
                    Type = 'variable (short decl)'
                    Content = $rawLine.Trim()
                }
            }
        }
        
        # 2. var/const declarations
        $matches2 = [regex]::Matches($cleaned, '\b(?:var|const)\s+([a-z][a-z0-9]*_[a-z0-9_]+)\s')
        foreach ($m in $matches2) {
            $name = $m.Groups[1].Value
            $results += [PSCustomObject]@{
                File = $relPath
                Line = $lineNum
                Name = $name
                Type = 'variable (var decl)'
                Content = $rawLine.Trim()
            }
        }
        
        # 3. Struct fields: inside struct {}, snake_case followed by type
        # Detect if we're inside a struct
        if ($cleaned -match 'struct\s*\{') { $structDepth++ }
        if ($cleaned -match '\}') { 
            # Count closing braces
            $closeCount = ([regex]::Matches($cleaned, '\}')).Count
            $structDepth = [Math]::Max(0, $structDepth - $closeCount)
        }
        
        if ($structDepth -gt 0) {
            # Check for struct field: snake_case Go name followed by a type
            $matches3 = [regex]::Matches($cleaned, '^\s+([a-z][a-z0-9]*_[a-z0-9_]+)\s+[\*\[]?\w')
            foreach ($m in $matches3) {
                $name = $m.Groups[1].Value
                # Make sure it's not a struct tag line (json:"...")
                $tagCheck = $rawLine.Trim()
                if ($tagCheck -match '^\w.*`.*json:') { 
                    # This is likely a field with tag - check the Go name
                    $results += [PSCustomObject]@{
                        File = $relPath
                        Line = $lineNum
                        Name = $name
                        Type = 'struct field'
                        Content = $rawLine.Trim()
                    }
                } else {
                    $results += [PSCustomObject]@{
                        File = $relPath
                        Line = $lineNum
                        Name = $name
                        Type = 'struct field'
                        Content = $rawLine.Trim()
                    }
                }
            }
        }
        
        # 4. Function/method parameters: func Foo(snake_name Type)
        $matches4 = [regex]::Matches($cleaned, 'func\s+\w+\s*\(([^)]*)\)')
        foreach ($m in $matches4) {
            $params = $m.Groups[1].Value
            $paramParts = $params -split ','
            foreach ($part in $paramParts) {
                $trimmed = $part.Trim()
                if ($trimmed -match '^([a-z][a-z0-9]*_[a-z0-9_]+)\s+[\*\[]?\w') {
                    $pname = $Matches[1]
                    $results += [PSCustomObject]@{
                        File = $relPath
                        Line = $lineNum
                        Name = $pname
                        Type = 'function parameter'
                        Content = $rawLine.Trim()
                    }
                }
            }
        }
        
        # 5. Method receiver: func (snake_name *Type)
        $matches5 = [regex]::Matches($cleaned, 'func\s+\(\s*([a-z][a-z0-9]*_[a-z0-9_]+)\s+')
        foreach ($m in $matches5) {
            $rname = $m.Groups[1].Value
            $results += [PSCustomObject]@{
                File = $relPath
                Line = $lineNum
                Name = $rname
                Type = 'method receiver'
                Content = $rawLine.Trim()
            }
        }
    }
}

if ($results.Count -eq 0) {
    Write-Output "NO snake_case violations found."
} else {
    Write-Output "Found $($results.Count) potential snake_case violations:"
    Write-Output ""
    foreach ($r in $results) {
        Write-Output "File: $($r.File)"
        Write-Output "Line: $($r.Line)"
        Write-Output "Name: $($r.Name)"
        Write-Output "Type: $($r.Type)"
        Write-Output "Content: $($r.Content)"
        Write-Output "---"
    }
}
