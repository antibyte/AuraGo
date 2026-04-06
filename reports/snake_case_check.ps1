$files = Get-ChildItem 'C:\Users\Andi\Documents\repo\AuraGo\internal\agent\*.go' -Exclude '*_test.go'
foreach ($f in $files) {
    $lines = Get-Content $f.FullName
    $inStruct = $false
    $inString = $false
    for ($i = 0; $i -lt $lines.Count; $i++) {
        $line = $lines[$i]
        $trimmed = $line.Trim()
        
        # Skip comment-only lines
        if ($trimmed -match '^\s*//') { continue }
        
        # Track struct blocks
        if ($trimmed -match '^type\s+\w+\s+struct\s*\{') { $inStruct = $true; continue }
        if ($inStruct -and $trimmed -match '^\}') { $inStruct = $false; continue }
        
        # Skip lines that are mostly strings (string literals, struct tags)
        if ($trimmed -match '^`') { continue }
        if ($trimmed -match '^"') { continue }
        
        # Pattern 1: struct field with snake_case Go name (inside struct block)
        if ($inStruct -and $trimmed -match '^([a-z][a-z0-9]*_[a-z][a-z0-9]*)\s+(\*?[A-Z]\w*|\[\]|string|int\b|bool\b|error\b|uint|int64|int32|float64|byte)' -and $trimmed -notmatch '^//') {
            $fieldName = $Matches[1]
            Write-Output "STRUCT_FIELD: $($f.Name):$($i+1): $trimmed"
        }
        
        # Pattern 2: function/method declaration with snake_case name
        if ($trimmed -match 'func\s+(\([a-z]+\s+\*?\*?\w+\)\s+)?([a-z][a-z0-9]*_[a-z][a-z0-9]*)\s*\(' -and $trimmed -notmatch '//') {
            $methodName = $Matches[2]
            Write-Output "FUNC_NAME: $($f.Name):$($i+1): $trimmed"
        }
        
        # Pattern 3: type declaration with snake_case name
        if ($trimmed -match '^type\s+([a-z][a-z0-9]*_[a-z][a-z0-9]*)\s+' -and $trimmed -notmatch '//') {
            $typeName = $Matches[1]
            Write-Output "TYPE_NAME: $($f.Name):$($i+1): $trimmed"
        }
        
        # Pattern 4: var declaration with snake_case name
        if ($trimmed -match '^\s*var\s+([a-z][a-z0-9]*_[a-z][a-z0-9]*)\s+' -and $trimmed -notmatch '//') {
            $varName = $Matches[1]
            Write-Output "VAR_DECL: $($f.Name):$($i+1): $trimmed"
        }
        
        # Pattern 5: short var declaration with snake_case (:=)
        # Need to be careful to avoid strings - check the part before :=
        if ($line -match '([a-z][a-z0-9]*_[a-z][a-z0-9]*)\s*:=') {
            $varName = $Matches[1]
            # Try to verify it's not inside a string by checking position
            $beforeAssign = $line.Substring(0, $line.IndexOf(':='))
            # Count quotes before this position - if odd number, we're inside a string
            $quoteCount = ([regex]::Matches($beforeAssign, '"').Count)
            $backtickCount = ([regex]::Matches($beforeAssign, '`').Count)
            if ($quoteCount % 2 -eq 0 -and $backtickCount % 2 -eq 0) {
                Write-Output "SHORT_VAR: $($f.Name):$($i+1): $trimmed"
            }
        }
        
        # Pattern 6: for loop with snake_case variable
        if ($line -match '\bfor\s+([a-z][a-z0-9]*_[a-z][a-z0-9]*)\s*(:=|\b)') {
            $varName = $Matches[1]
            $beforeFor = $line.Substring(0, $line.IndexOf('for'))
            $quoteCount = ([regex]::Matches($beforeFor, '"').Count)
            $backtickCount = ([regex]::Matches($beforeFor, '`').Count)
            if ($quoteCount % 2 -eq 0 -and $backtickCount % 2 -eq 0) {
                Write-Output "FOR_VAR: $($f.Name):$($i+1): $trimmed"
            }
        }
        
        # Pattern 7: range variable with snake_case
        if ($line -match '\b([a-z][a-z0-9]*_[a-z][a-z0-9]*)\s*(,\s*[a-z][a-z0-9_]*)?\s*:=\s*range\b') {
            $varName = $Matches[1]
            $beforeRange = $line.Substring(0, $line.IndexOf(':='))
            $quoteCount = ([regex]::Matches($beforeRange, '"').Count)
            $backtickCount = ([regex]::Matches($beforeRange, '`').Count)
            if ($quoteCount % 2 -eq 0 -and $backtickCount % 2 -eq 0) {
                Write-Output "RANGE_VAR: $($f.Name):$($i+1): $trimmed"
            }
        }
    }
}
Write-Output "--- Done ---"
