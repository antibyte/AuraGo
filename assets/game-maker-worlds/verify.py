"""Verify generated runtime identities, dependencies, budgets and review boards."""
from pathlib import Path
from PIL import Image,ImageDraw
import json,hashlib,struct,collections
from catalog import entries
ROOT=Path(__file__).resolve().parents[2];PACKS=ROOT/'internal/gamemaker/asset_packs';REPORT=ROOT/'reports/game-maker-worlds'
PACKSIZES={'aurago-pirates-3d':48,'aurago-pirates-topdown':20,'aurago-pirates-side':16,'aurago-isometric':40}
def checked(root,f):
    p=root/f['file'];assert p.resolve().is_relative_to(root.resolve());blob=p.read_bytes()
    assert len(blob)==f['bytes'] and hashlib.sha256(blob).hexdigest()==f['sha256'],str(p)
    return blob
def gltf(blob):
    magic,version,size=struct.unpack_from('<III',blob);assert magic==0x46546c67 and version==2 and size==len(blob)
    n,kind=struct.unpack_from('<II',blob,12);assert kind==0x4e4f534a
    d=json.loads(blob[20:20+n]);assert not any(i.get('uri') for i in d.get('images',[])+d.get('buffers',[]));return d
def verify():
    REPORT.mkdir(parents=True,exist_ok=True);report={'packs':{},'clipping':[]};grand=0
    for id,cap in PACKSIZES.items():
        root=PACKS/id;m=json.loads((root/'manifest.json').read_text());expected=entries(id=='aurago-isometric')
        assert {a['id'] for a in m['assets']}=={a['id'] for a in expected};assert m['license']=='MIT' and m['version']=='1.0.0'
        previews=[];animations=0;clipped=[]
        if m['kind']=='model3d':
            for a in m['assets']:
                assert a['units']=='metres' and a['up']=='+Y' and a['forward']=='+Z'
                docs=[gltf(checked(root,f)) for f in a['lods']]
                nodes={n.get('name') for n in docs[0]['nodes']}
                assert all(s['node'] in nodes for s in a['sockets']),a['id']
                if a.get('animation_library'):docs.append(gltf(checked(root,a['animation_library'])))
                clips={v.get('name') for d in docs for v in d.get('animations',[])}
                assert {c['id'] for c in a['animations']}<=clips,(a['id'],clips)
                if a['category']=='ships':assert a['waterline']==0 and {'bow','stern','wake'}<={s['id'] for s in a['sockets']}
                animations+=len(a['animations']);previews.append((a['id'],Image.open(root/a['preview']).convert('RGBA')))
        else:
            pages={p['file']:Image.open(root/p['file']).convert('RGBA') for p in m['atlases']}
            for p in m['atlases']:checked(root,p);assert pages[p['file']].size==(p['width'],p['height']) and max(p['width'],p['height'])<=2048
            frames={f['id']:f for f in m['frames']};assets={a['id']:a for a in m['assets']}
            for a in m['assets']:
                if a.get('preview_file'):checked(root,a['preview_file'])
                previews.append((a['id'],Image.open(root/a['preview']).convert('RGBA')))
                assert 0<=a['origin']['x']<=1 and 0<=a['origin']['y']<=1,a['id']
                dirs={d['id'] for d in a['directions']};required=16 if id.endswith('topdown') and a['id'].startswith('ships-') else 8 if id!='aurago-pirates-side' and a['id'].startswith(('people-','animals-')) else 2 if id.endswith('side') else 1
                assert len(dirs)>=required,(a['id'],len(dirs))
                own=[c for c in m['animations'] if c['asset_id']==a['id']]
                assert {c['direction'] for c in own}==dirs
                for c in own:
                    animations+=1;assert len(c['frames'])>0 and 0<c['frame_rate']<=60
                    for event in c['events']:assert 0<=event['frame']<len(c['frames'])
                    for number in set(c['frames']):
                        f=frames[number];p=pages[f['atlas']];assert f['x']+f['width']<=p.width and f['y']+f['height']<=p.height
                        frame=p.crop((f['x'],f['y'],f['x']+f['width'],f['y']+f['height']));box=frame.getbbox();assert box,(a['id'],c['id'],number)
                        if a['id'].startswith(('people-','animals-','ships-')) and (box[0]==0 or box[1]==0 or box[2]==frame.width or box[3]==frame.height):clipped.append((a['id'],c['id'],number))
        for start in range(0,len(previews),60):
            group=previews[start:start+60];board=Image.new('RGB',(1200,((len(group)+7)//8)*156),(24,33,43));draw=ImageDraw.Draw(board)
            for i,(name,img) in enumerate(group):
                img.thumbnail((142,126),Image.Resampling.NEAREST if m['kind']=='sprite2d' else Image.Resampling.LANCZOS);x=(i%8)*150;y=(i//8)*156;board.paste(img,(x+(150-img.width)//2,y+(128-img.height)//2),img);draw.text((x+3,y+131),name[:24],fill='#eef3f8')
            board.save(REPORT/f'{id}-{start//60}.png')
        total=sum(p.stat().st_size for p in root.rglob('*') if p.is_file());assert total<=cap*1024**2,(id,total,cap);grand+=total
        report['packs'][id]={'entries':len(m['assets']),'animations':animations,'bytes':total,'budget':cap*1024**2};report['clipping'].extend(clipped)
    report['pack_bytes']=grand;assert grand<=124*1024**2
    # Count the entire shared helpers touched by this feature, conservatively,
    # including pre-existing sprite functionality rather than only its delta.
    helpers=['runtime/isometric.js','runtime/aurago-game-1.js','skills/aurago-game-assets/SKILL.md']
    shared=sum((PACKS.parent/f).stat().st_size for f in helpers)
    assert shared<=4*1024**2
    report['shared_bytes']=shared;report['total_bytes']=grand+shared;assert grand+shared<=128*1024**2
    (REPORT/'verification.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({k:v for k,v in report.items() if k!='clipping'},indent=2));print('Review clipped frames:',len(report['clipping']))
if __name__=='__main__':verify()
