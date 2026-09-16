# Rebuild the changed maritime source models, then their actual exported views.
$ErrorActionPreference = 'Stop'
$blender = 'D:\Blender 5.2\blender.exe'
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Set-Location $root
$ids = (python -c "import sys;sys.path.insert(0,'assets/game-maker-worlds');import catalog;print(','.join(e['id'] for e in catalog.entries() if e['category']=='ships' or e['category']=='harbor' and e['design'] in ['beach-hut','stilt-house','warehouse','tavern','shipyard','watchtower','lighthouse','naval-quarters'] or e['id']=='equipment-ship-wheel'))")
& $blender --background --python assets/game-maker-worlds/build.py -- --only $ids
if ($LASTEXITCODE) { throw 'Model production failed' }
& $blender --background --python assets/game-maker-worlds/render_review.py
if ($LASTEXITCODE) { throw 'Review rendering failed' }
& $blender --background --python assets/game-maker-worlds/render_sprites.py -- --only $ids
if ($LASTEXITCODE) { throw 'Sprite rendering failed' }
foreach ($view in @('top','side')) {
    python assets/game-maker-worlds/pack_sprites.py --view $view
    if ($LASTEXITCODE) { throw 'Atlas packing failed' }
}
python assets/game-maker-worlds/verify.py
if ($LASTEXITCODE) { throw 'Integrity verification failed' }
