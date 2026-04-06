Get-ChildItem -Path 'internal\server' -Filter '*.go' -Recurse | Where-Object { $_.Name -notmatch '_test\.go$' } | ForEach-Object {
    $file = $_.FullName
    $lines = Get-Content $file
    for ($i = 0; $i -lt $lines.Count; $i++) {
        $line = $lines[$i]
        $ln = $i + 1
        # Skip comments
        if ($line -match '^\s*//') { continue }
        # function parameters: (snake_name Type or , snake_name Type
        if ($line -match '(?:func\s+\w+\s*\(|,)\s*([a-z][a-z0-9]*_[a-z0-9_]+)\s+[\*\[]?[A-Z]') {
            Write-Output "${file}:${ln}: $($line.Trim())"
        }
        # Also check method receivers: func (s *Type) or func (snake_name *Type)
        if ($line -match 'func\s+\(\s*([a-z][a-z0-9]*_[a-z0-9_]+)\s+[\*\[]?\w+') {
            Write-Output "${file}:${ln}: $($line.Trim())"
        }
    }
}
