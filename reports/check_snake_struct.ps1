Get-ChildItem -Path 'internal\server' -Filter '*.go' -Recurse | Where-Object { $_.Name -notmatch '_test\.go$' } | ForEach-Object {
    $file = $_.FullName
    $lines = Get-Content $file
    for ($i = 0; $i -lt $lines.Count; $i++) {
        $line = $lines[$i]
        $ln = $i + 1
        # Skip comments
        if ($line -match '^\s*//') { continue }
        # Skip lines that are only struct tags or in string context
        if ($line -match '^\s*`') { continue }
        # Check for struct fields: snake_case Go name followed by a type (Capital letter or pointer/array of type)
        if ($line -match '^\s+([a-z][a-z0-9]*_[a-z0-9_]+)\s+[\*\[]?[A-Z]') {
            Write-Output "${file}:${ln}: $($line.Trim())"
        }
    }
}
