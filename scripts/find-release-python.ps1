# Return a verified interpreter path so both Windows release builders use the same Python.
$ErrorActionPreference = 'Stop'

foreach ($name in @('py', 'python', 'python3')) {
    $command = Get-Command $name -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    if (-not $command) { continue }
    $arguments = @()
    if ($name -eq 'py') { $arguments = @('-3') }
    try {
        $executable = & $command.Source @arguments -c 'import sys; sys.exit(1) if sys.version_info < (3, 10) else print(sys.executable)' 2>$null
        if ($LASTEXITCODE -eq 0 -and $executable -and (Test-Path -LiteralPath $executable -PathType Leaf)) {
            return $executable.Trim()
        }
    } catch {
        # An unavailable launcher or Windows Store alias must not hide a working alternative.
    }
}

throw 'Python 3.10+ is required to prepare Phonon release resources. Install Python with the Windows py launcher or add python/python3 to PATH.'
