"""Register the four produced manifests through the existing public catalog."""
from pathlib import Path
import json
ROOT=Path(__file__).resolve().parents[2]/'internal/gamemaker/asset_packs'
path=ROOT/'catalog.json';catalog=json.loads(path.read_text())
for id in ('aurago-pirates-3d','aurago-pirates-topdown','aurago-pirates-side','aurago-isometric'):
    manifest=json.loads((ROOT/id/'manifest.json').read_text())
    entry={key:manifest[key] for key in ('id','name','description','tags','version','kind')}
    entry['selection_mode']='assets'
    if entry['kind']=='sprite2d':entry['manifest_schema']=2
    found=next((i for i,p in enumerate(catalog) if p['id']==id),None)
    if found is None:catalog.append(entry)
    else:catalog[found]=entry
path.write_text(json.dumps(catalog,indent=2,ensure_ascii=False)+'\n',encoding='utf-8')
