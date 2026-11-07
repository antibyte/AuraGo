param()
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../..'))
$runtime = Join-Path $repo 'ui/img/goldfish-adventure'
$layout = Get-Content -LiteralPath (Join-Path $PSScriptRoot 'art-layout.json') -Raw | ConvertFrom-Json -AsHashtable
$sheets = @{}
$assets = [Collections.Generic.List[object]]::new()

function Write-Json($path, $value) {
    [IO.File]::WriteAllText($path, ((($value | ConvertTo-Json -Depth 50) -replace "`r`n", "`n") + "`n"), [Text.UTF8Encoding]::new($false))
}

foreach ($sheet in $layout.sheets) {
    $source = Join-Path $PSScriptRoot ('masters/' + $sheet.file)
    $image = [Drawing.Bitmap]::new($source)
    try {
        if ($sheet.rowEdges -and ($sheet.rowEdges.Count -ne ($sheet.rows + 1) -or $sheet.rowEdges[0] -ne 0 -or $sheet.rowEdges[-1] -ne $image.Height)) { throw "Invalid authored row bounds: $($sheet.id)" }
        $data = $image.LockBits([Drawing.Rectangle]::new(0, 0, $image.Width, $image.Height), [Drawing.Imaging.ImageLockMode]::ReadOnly, [Drawing.Imaging.PixelFormat]::Format32bppArgb)
        try {
            $pixels = [byte[]]::new([Math]::Abs($data.Stride) * $image.Height)
            [Runtime.InteropServices.Marshal]::Copy($data.Scan0, $pixels, 0, $pixels.Length)
            $stride = $data.Stride
        } finally { $image.UnlockBits($data) }
        $frames = [Collections.Generic.List[object]]::new()
        for ($row = 0; $row -lt $sheet.rows; $row++) {
            for ($col = 0; $col -lt $sheet.columns; $col++) {
                $x = [int][Math]::Floor($col * $image.Width / $sheet.columns)
                $y = if ($sheet.rowEdges) { [int]$sheet.rowEdges[$row] } else { [int][Math]::Floor($row * $image.Height / $sheet.rows) }
                $w = [int][Math]::Floor(($col + 1) * $image.Width / $sheet.columns) - $x
                $h = if ($sheet.rowEdges) { [int]$sheet.rowEdges[$row + 1] - $y } else { [int][Math]::Floor(($row + 1) * $image.Height / $sheet.rows) - $y }
                if ($h -le 0) { throw "Unordered authored row bounds: $($sheet.id)" }
                $left = $w; $right = -1; $bottom = -1
                for ($py = 0; $py -lt $h; $py++) {
                    for ($px = 0; $px -lt $w; $px++) {
                        if ($pixels[($y + $py) * $stride + ($x + $px) * 4 + 3] -gt 40) {
                            $left = [Math]::Min($left, $px); $right = [Math]::Max($right, $px); $bottom = $py
                        }
                    }
                }
                if ($right -lt 0) { throw "Empty frame in $($sheet.id) at $row,$col" }
                # Anchor at the feet/base so an outstretched hand does not shift the body.
                $baseLeft = $w; $baseRight = -1
                $baseTop = [Math]::Max(0, $bottom - [int]($h * 0.12))
                for ($py = $baseTop; $py -le $bottom; $py++) {
                    for ($px = 0; $px -lt $w; $px++) {
                        if ($pixels[($y + $py) * $stride + ($x + $px) * 4 + 3] -gt 40) {
                            $baseLeft = [Math]::Min($baseLeft, $px); $baseRight = [Math]::Max($baseRight, $px)
                        }
                    }
                }
                $frames.Add(@{rect=@($x,$y,$w,$h);pivot=@([Math]::Round(($baseLeft + $baseRight + 1) / (2 * $w),6), [Math]::Round(($bottom + 1) / $h,6))})
                if ($sheet.items) {
                    $item = $sheet.items[$row * $sheet.columns + $col]
                    $destination = Join-Path $runtime ('items/' + $item + '.png')
                    [IO.Directory]::CreateDirectory([IO.Path]::GetDirectoryName($destination)) | Out-Null
                    $cut = $image.Clone([Drawing.Rectangle]::new($x,$y,$w,$h), [Drawing.Imaging.PixelFormat]::Format32bppArgb)
                    try { $cut.Save($destination,[Drawing.Imaging.ImageFormat]::Png) } finally { $cut.Dispose() }
                }
            }
        }
        $sheets[$sheet.id] = @{width=$image.Width;height=$image.Height;frames=$frames}
        if (!$sheet.items) {
            $destination = Join-Path $runtime $sheet.file
            [IO.Directory]::CreateDirectory([IO.Path]::GetDirectoryName($destination)) | Out-Null
            Copy-Item -LiteralPath $source -Destination $destination
        }
        $record = @{source=('masters/' + $sheet.file);sha256=(Get-FileHash -LiteralPath $source -Algorithm SHA256).Hash.ToLower();width=$image.Width;height=$image.Height;columns=$sheet.columns;rows=$sheet.rows}
        if (!$sheet.items) { $record.runtime = $sheet.file }
        $assets.Add($record)
    } finally { $image.Dispose() }
}

foreach ($item in $layout.static) {
    $source = Join-Path $PSScriptRoot ('masters/' + $item.file)
    $destination = Join-Path $runtime $item.runtime
    [IO.Directory]::CreateDirectory([IO.Path]::GetDirectoryName($destination)) | Out-Null
    Copy-Item -LiteralPath $source -Destination $destination
    $image = [Drawing.Bitmap]::new($source)
    try { $assets.Add(@{source=('masters/' + $item.file);runtime=$item.runtime;sha256=(Get-FileHash -LiteralPath $source -Algorithm SHA256).Hash.ToLower();width=$image.Width;height=$image.Height}) }
    finally { $image.Dispose() }
}

$sprites = [ordered]@{}
foreach ($id in $layout.sprites.Keys) {
    $spec = $layout.sprites[$id].Clone()
    $sheet = $layout.sheets | Where-Object id -EQ $spec.sheet
    $frames = @($sheets[$spec.sheet].frames)
    foreach ($extra in $spec.extraSheets) {
        $extraSheet = $layout.sheets | Where-Object id -EQ $extra
        foreach ($frame in $sheets[$extra].frames) {
            $entry = $frame.Clone()
            $entry.src = '/img/goldfish-adventure/' + $extraSheet.file
            $frames += $entry
        }
    }
    $spec.Remove('sheet')
    $spec.Remove('extraSheets')
    $spec.src = '/img/goldfish-adventure/' + $sheet.file
    $spec.frames = $frames
    $sprites[$id] = $spec
}
$manifest = [ordered]@{version=1;audioVolume=$layout.audioVolume;rooms=$layout.rooms;sprites=$sprites;portraits=$layout.portraits;moments=$layout.moments}
Write-Json (Join-Path $runtime 'manifest.json') $manifest
Write-Json (Join-Path $PSScriptRoot 'character-manifest.json') @{version=1;tool='built-in image_gen';processing='Unscaled byte copies and authored grid/row extraction only. Frame pivots use the nontransparent base/feet band.';sheets=$assets}
Write-Output "Built $($layout.sheets.Count) sheets, $($sprites.Count) sprites and $($layout.rooms.Count) room staging entries."
