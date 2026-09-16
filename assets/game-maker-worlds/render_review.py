"""Review images are rendered from exported GLBs, never substituted author meshes."""
import importlib.util, json, hashlib
from pathlib import Path
HERE=Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location('poly_render',HERE.parent/'game-maker-low-poly/render.py')
renderer=importlib.util.module_from_spec(spec);spec.loader.exec_module(renderer)
renderer.OUT=HERE.parents[1]/'internal/gamemaker/asset_packs/aurago-pirates-3d'
manifest=json.loads((renderer.OUT/'manifest.json').read_text())
for asset in manifest['assets']:
    renderer.render(asset,384)
    data=(renderer.OUT/asset['preview']).read_bytes()
    asset['preview_file']=dict(file=asset['preview'],bytes=len(data),sha256=hashlib.sha256(data).hexdigest())
(renderer.OUT/'manifest.json').write_text(json.dumps(manifest,separators=(',',':'))+'\n',encoding='utf-8')
