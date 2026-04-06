Get-ChildItem -Path 'internal\server' -Filter '*.go' -Recurse | Where-Object { $_.Name -notmatch '_test\.go$' } | ForEach-Object {
    $file = $_.FullName
    $lines = Get-Content $file
    for ($i = 0; $i -lt $lines.Count; $i++) {
        $line = $lines[$i]
        $ln = $i + 1
        # Skip comments
        if ($line -match '^\s*//') { continue }
        # short var declarations: snake_name :=
        if ($line -match '(?:^|\s)([a-z][a-z0-9]*_[a-z0-9_]+)\s*:=') {
            Write-Output "${file}:${ln}: $($line.Trim())"
        }
        # var/const declarations
        if ($line -match '\b(?:var|const)\s+([a-z][a-z0-9]*_[a-z0-9_]+)\s') {
            Write-Output "${file}:${ln}: $($line.Trim())"
        }
    }
}
