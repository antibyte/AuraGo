"""Palette-controlled, outlined pixel atlases; never resample animation frames."""
from pathlib import Path
from PIL import Image, ImageFilter
import argparse, json, hashlib, math, importlib.util
HERE=Path(__file__).resolve().parent;ROOT=HERE.parents[1];RAW=HERE/'production/sprites'
spec=importlib.util.spec_from_file_location('poly_catalog',HERE.parent/'game-maker-low-poly/catalog.py')
poly=importlib.util.module_from_spec(spec);spec.loader.exec_module(poly)
spec=importlib.util.spec_from_file_location('world_catalog',HERE/'catalog.py')
world=importlib.util.module_from_spec(spec);spec.loader.exec_module(world)
PALETTE=[(26,31,43)]
for value in poly.PALETTE.values():
    value=value.lstrip('#');rgb=[int(value[i:i+2],16) for i in (0,2,4)]
    for shade in (.8,1,1.18):PALETTE.append(tuple(min(255,round(v*shade)) for v in rgb))
PALETTE=PALETTE[:255];palette=Image.new('P',(1,1));palette.putpalette(sum((list(c) for c in PALETTE),[])+[0]*(768-len(PALETTE)*3))

def pixel_frame(path):
    source=Image.open(path).convert('RGBA');alpha=source.getchannel('A').point(lambda v:255 if v>=128 else 0)
    # Blender Workbench vertex colours are stored in scene-linear space. Restore
    # an sRGB-like display range before selecting the shared authored palette.
    rgb=source.convert('RGB').point(lambda v:round(255*(v/255)**.65))
    quant=rgb.quantize(palette=palette,dither=Image.Dither.NONE).convert('RGBA');quant.putalpha(alpha)
    outline=Image.new('RGBA',source.size,PALETTE[0]+(255,));outline.putalpha(alpha.filter(ImageFilter.MaxFilter(3)))
    outline.alpha_composite(quant);return outline

def pack(view,only=None):
    packid='aurago-isometric' if view=='isometric' else 'aurago-pirates-'+('topdown' if view=='top' else 'side')
    out=ROOT/'internal/gamemaker/asset_packs'/packid;out.mkdir(parents=True,exist_ok=True)
    pages=[];frames=[];assets=[];animations=[]
    definitions={a['id']:a for a in world.entries(view=='isometric')}
    model_path=HERE/'production/isometric-glb/manifest.json' if view=='isometric' else ROOT/'internal/gamemaker/asset_packs/aurago-pirates-3d/manifest.json'
    models={a['id']:a for a in json.loads(model_path.read_text())['assets']}
    for source in sorted((RAW/view).glob('*/frames.json')):
        meta=json.loads(source.read_text());id=meta['id']
        if only and id not in only:continue
        w,h=meta['width'],meta['height'];origin=dict(meta['origin']);left=top=0
        # Identical poses share a rectangle, including static objects. This is
        # lossless: action timing and events still retain every original frame.
        paths=[];aliases={};unique={};pixels={}
        all_paths=[f for c in meta['animations'] for f in c['frames']]+[f for layer in meta.get('layers',{}).values() for f in layer.values()]
        for path in dict.fromkeys(all_paths):
            frame=pixel_frame(RAW/path);digest=hashlib.sha256(frame.tobytes()).hexdigest()
            canonical=unique.get(digest)
            if canonical is None:
                canonical=path;unique[digest]=path;paths.append(path);pixels[path]=frame
            aliases[path]=canonical
        if view=='isometric' and definitions[id]['category']!='terrain':
            # One union crop for every direction, pose and layer preserves the
            # world anchor and native pixels while avoiding huge empty GPU pages.
            boxes=[frame.getbbox() for frame in pixels.values() if frame.getbbox()]
            left=max(0,min(min(b[0] for b in boxes),math.floor(origin['x']*w))-2);top=max(0,min(min(b[1] for b in boxes),math.floor(origin['y']*h))-2)
            right=min(w,max(max(b[2] for b in boxes),math.ceil(origin['x']*w))+2);bottom=min(h,max(max(b[3] for b in boxes),math.ceil(origin['y']*h))+2)
            origin={'x':(origin['x']*w-left)/(right-left),'y':(origin['y']*h-top)/(bottom-top)}
            pixels={path:frame.crop((left,top,right,bottom)) for path,frame in pixels.items()}
            w,h=right-left,bottom-top
        columns_per_page=2048//w;rows_per_page=2048//h;capacity=columns_per_page*rows_per_page
        frame_ids={};preview=None
        for page_index,start in enumerate(range(0,len(paths),capacity)):
            chunk=paths[start:start+capacity];columns=min(columns_per_page,len(chunk));rows=math.ceil(len(chunk)/columns)
            image=Image.new('RGBA',(columns*w,rows*h));name=f'atlases/{id}-{page_index}.png'
            for index,path in enumerate(chunk):
                frame=pixels[path];x=index%columns*w;y=index//columns*h;image.alpha_composite(frame,(x,y))
                frame_ids[path]=len(frames);frames.append(dict(id=len(frames),atlas=name,x=x,y=y,width=w,height=h))
                if preview is None:preview=frame
            target=out/name;target.parent.mkdir(parents=True,exist_ok=True);image.save(target,optimize=True)
            data=target.read_bytes();pages.append(dict(file=name,bytes=len(data),sha256=hashlib.sha256(data).hexdigest(),level=0,width=image.width,height=image.height))
        for path,canonical in aliases.items():frame_ids[path]=frame_ids[canonical]
        definition=definitions[id];first=meta['animations'][0]
        assets.append(dict(id=id,name=definition['name'],description=definition['description'],tags=definition['tags'],view=view,entity=id,
            action=first['action'],direction=first['direction'],directions=meta['directions'],frames=[frame_ids[first['frames'][0]]],origin=origin,width=w,height=h,
            transform=dict(mode='directional',forward_radians=0,flip_x=False,flip_y=False)))
        asset=assets[-1];model=models[id]
        if meta.get('sockets'):
            asset['sockets']=[dict(id=name,pose='rest',directions={direction:dict(x=point['x']-left,y=point['y']-top) for direction,point in points.items()}) for name,points in meta['sockets'].items()]
        if model.get('connections'):
            asset['connections']=dict(units='metres',up='+Y',forward='+Z',items=model['connections'])
        if meta.get('layers'):
            asset['layers']=[dict(id=name,frames={direction:frame_ids[path] for direction,path in layer.items()},depth_offset=order+.1) for order,(name,layer) in enumerate(meta['layers'].items())]
            # Preview includes all layers; runtime keeps them independent.
            for layer in meta['layers'].values():preview.alpha_composite(pixels[aliases[layer[first['direction']]]])
        asset['source_sha256']=meta['source_glb']
        asset['footprint']=[max(.25,(model['bounds']['max'][i]-model['bounds']['min'][i])/2) for i in (0,2)] if view=='isometric' else [1,1]
        if view=='isometric':
            lo,hi=model['bounds']['min'],model['bounds']['max']
            fit=min(1,4/max(hi[0]-lo[0],hi[2]-lo[2])) if definition['category']=='vehicles' else min(1,4/max(.01,hi[1]-lo[1])) if definition['category']=='nature' else 1
            asset['footprint']=[max(.25,v*fit) for v in asset['footprint']]
        if definition['category']=='terrain':asset['footprint']=[1,1]
        if 'waterline' in model:asset['waterline']=model['waterline']
        thumb=out/'previews'/(id+'.png');thumb.parent.mkdir(parents=True,exist_ok=True)
        preview.save(thumb,optimize=True);blob=thumb.read_bytes()
        asset['preview']=thumb.relative_to(out).as_posix();asset['preview_file']=dict(file=asset['preview'],bytes=len(blob),sha256=hashlib.sha256(blob).hexdigest())
        for clip in meta['animations']:
            animations.append(dict(id=id+'-'+clip['action']+'-'+clip['direction'],asset_id=id,entity=id,action=clip['action'],direction=clip['direction'],frames=[frame_ids[f] for f in clip['frames']],frame_rate=clip['frame_rate'],repeat=clip['repeat'],events=clip['events']))
        preview_path=HERE/'production/review'/view/(id+'.png');preview_path.parent.mkdir(parents=True,exist_ok=True);preview.resize((w*4,h*4),Image.Resampling.NEAREST).save(preview_path)
    manifest=dict(schema_version=2,id=packid,version=world.VERSION,kind='sprite2d',name='AuraGo Isometric' if view=='isometric' else 'AuraGo Pirates '+view.title(),description='Original animated isometric pixel worlds.' if view=='isometric' else 'Original animated maritime pixel art.',tags=['isometric','village','city','sci-fi'] if view=='isometric' else ['pirates','steampunk','maritime',view],license='MIT',atlases=pages,frames=frames,assets=assets,animations=animations,production_status='review')
    # Remove only obsolete atlas pages owned by this generated pack.
    retained={p['file'] for p in pages}
    for old in (out/'atlases').glob('*.png'):
        if old.relative_to(out).as_posix() not in retained:old.unlink()
    (out/'manifest.json').write_text(json.dumps(manifest,separators=(',',':'))+'\n',encoding='utf-8')
    (out/'LICENSE.txt').write_text((ROOT/'LICENSE').read_text(encoding='utf-8'),encoding='utf-8')
    total=sum(p.stat().st_size for p in out.rglob('*') if p.is_file());cap=(40 if view=='isometric' else 20 if view=='top' else 16)*1024*1024
    assert total<=cap,(packid,total)
    print(packid,len(assets),'motifs',len(frames),'frames',total,'bytes')

if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--view',choices=['top','side','isometric'],required=True);parser.add_argument('--only')
    args=parser.parse_args();pack(args.view,set(args.only.split(',')) if args.only else None)
