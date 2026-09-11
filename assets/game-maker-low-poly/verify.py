"""Validate production contracts and assemble contact sheets from exported renders."""
from pathlib import Path
from collections import Counter
import argparse
import hashlib
import json
import struct
import re
from catalog import entries, EXPECTED_COUNTS, HUMAN_ACTIONS

ROOT=Path(__file__).resolve().parents[2]
PACK=ROOT/'internal/gamemaker/asset_packs/aurago-low-poly'
REPORTS=ROOT/'reports/low-poly'


def document(path):
    data=path.read_bytes()
    magic,version,size=struct.unpack_from('<III',data)
    assert (magic,version,size)==(0x46546c67,2,len(data)),path
    length,kind=struct.unpack_from('<II',data,12)
    assert kind==0x4e4f534a
    result=json.loads(data[20:20+length])
    assert not any(x.get('uri') for x in result.get('buffers',[])+result.get('images',[])),path
    return result


def verify():
    manifest=json.loads((PACK/'manifest.json').read_text())
    assert {a['id'] for a in manifest['assets']}=={e['id'] for e in entries()}
    assert Counter(a['category'] for a in manifest['assets'])==EXPECTED_COUNTS
    checked=set();humanoid_joints=None;triangles={};clips=0
    for a in manifest['assets']:
        assert a['up']=='+Y' and a['forward']=='+Z' and a['units']=='metres'
        assert all(lo<=hi for lo,hi in zip(a['bounds']['min'],a['bounds']['max']))
        assert (PACK/a['preview']).is_file(),a['id']
        if a.get('preview_file'):
            data=(PACK/a['preview']).read_bytes()
            assert len(data)==a['preview_file']['bytes'] and hashlib.sha256(data).hexdigest()==a['preview_file']['sha256'],a['id']
        docs=[]
        for f in a['lods']+[a['animation_library']] if a.get('animation_library') else a['lods']:
            path=PACK/f['file'];data=path.read_bytes()
            assert len(data)==f['bytes'] and hashlib.sha256(data).hexdigest()==f['sha256'],path
            d=document(path);docs.append(d);checked.add(f['file'])
            assert len(d.get('materials',[]))<=4,(a['id'],'materials')
            for m in d.get('meshes',[]):
                for p in m['primitives']:
                    assert 'COLOR_0' in p['attributes'],path
        nodes={n.get('name') for n in docs[0]['nodes']}
        assert not any(re.search(r'[\s.\[\]:/]',n or '') for n in nodes),(a['id'],'Three.js node-name sanitization')
        for socket in a['sockets']:assert socket['node'] in nodes,(a['id'],socket)
        for part in a['moving_parts']:assert part['node'] in nodes,(a['id'],part)
        animation_doc=docs[-1] if a.get('animation_library') else docs[0]
        actual={c['name'] for c in animation_doc.get('animations',[])}
        assert actual=={c['id'] for c in a['animations']},(a['id'],actual)
        for c in a['animations']:
            assert c['duration']>0 and all(0<=e['time']<=c['duration'] for e in c.get('events',[]))
        if a['category']=='humans':
            assert actual==set(HUMAN_ACTIONS)
            joints=[docs[0]['nodes'][i]['name'] for i in docs[0]['skins'][0]['joints']]
            if humanoid_joints is None:humanoid_joints=joints
            assert joints==humanoid_joints,a['id']
        for level in range(1,len(a['lods'])):
            assert a['lods'][level]['triangles']<a['lods'][level-1]['triangles'],a['id']
        triangles[a['id']]=a['lods'][0]['triangles'];clips+=len(a['animations'])
        if a['category']=='architecture':assert triangles[a['id']]<=12000,(a['id'],'building triangle budget')
    total=sum(p.stat().st_size for p in PACK.rglob('*') if p.is_file())
    assert total<=100*1024*1024,total
    REPORTS.mkdir(parents=True,exist_ok=True)
    result=dict(models=220,glbs=len(checked),declared_actions=clips,runtime_bytes=total,
                largest_models=sorted(triangles.items(),key=lambda x:x[1],reverse=True)[:15])
    (REPORTS/'contracts.json').write_text(json.dumps(result,indent=2)+'\n')
    print(json.dumps(result,indent=2))
    return manifest


def boards(manifest):
    from PIL import Image, ImageDraw, ImageFont
    font=ImageFont.truetype('C:/Windows/Fonts/segoeui.ttf',14)
    title=ImageFont.truetype('C:/Windows/Fonts/seguisb.ttf',22)
    for category in manifest['categories']:
        assets=[a for a in manifest['assets'] if a['category']==category['id']]
        cols=min(6,len(assets));w=192;h=210
        canvas=Image.new('RGB',(cols*w,52+((len(assets)+cols-1)//cols)*h),'#16232c')
        draw=ImageDraw.Draw(canvas)
        draw.text((15,12),category['name']+' / exported GLB previews',font=title,fill='#e4d5b6')
        for i,a in enumerate(assets):
            im=Image.open(PACK/a['preview']).convert('RGB');im.thumbnail((184,180))
            x=(i%cols)*w;y=52+(i//cols)*h
            canvas.paste(im,(x+(w-im.width)//2,y))
            draw.text((x+8,y+182),a['name'],font=font,fill='#e4d5b6')
        canvas.save(REPORTS/('contact-'+category['id']+'.webp'),quality=92)


if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--boards',action='store_true')
    options=parser.parse_args();manifest=verify()
    if options.boards:boards(manifest)
