"""Convert the retained raster masters for delivery; do not repaint or crop them.

Roles are processed only when the master exists. Tables whose id is not yet in TABLE_IDS (ui/js/desktop/apps/pinball/config.js)
are recorded in artwork.json but not delivered; pass --all to deliver them anyway.
Run with `py -3.13` (Pillow cleans the plastics alpha).
"""
import hashlib
import json
from pathlib import Path
import re
import struct
import subprocess
import sys

from PIL import Image

ROOT=Path(__file__).resolve().parents[2]
SOURCE=Path(__file__).parent
OUT=ROOT/'ui/3d/pinball/v1'
ALL_TABLES=['neon','leviathan','temple','observatory']


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def registered_tables():
    text=(ROOT/'ui/js/desktop/apps/pinball/config.js').read_text('utf-8')
    match=re.search(r'TABLE_IDS\s*=\s*Object\.freeze\(\[([^\]]*)\]',text)
    return re.findall(r"'([^']+)'",match.group(1))


def encode(source,target,*args):
    subprocess.run(['ffmpeg','-v','error','-y','-i',str(source),*args,str(target)],check=True)


def fit_size(path,limit):
    """Never upscale: shrink to fit `limit`, keeping the aspect ratio; exact size when nothing shrinks."""
    width,height=struct.unpack('>II',path.read_bytes()[16:24])
    ratio=min(1,limit[0]/width,limit[1]/height)
    if ratio==1:
        return width,height
    return max(2,int(width*ratio)//2*2),max(2,int(height*ratio)//2*2)


def encode_plastics(path,target):
    """Lossy RGBA WebP with a lossless alpha plane (the ffmpeg libwebp wrapper has no alpha_quality option, so Pillow encodes it).

    Alpha < 40 becomes 0 and alpha >= 216 becomes 255 first, so the lossy encode keeps clean die-cut edges."""
    image=Image.open(path).convert('RGBA')
    if image.size!=(1024,1536):
        image=image.resize((1024,1536),Image.LANCZOS)
    alpha=image.getchannel('A').point(lambda a:0 if a<40 else 255 if a>=216 else a)
    image.putalpha(alpha)
    image.save(target,'WEBP',quality=92,alpha_quality=100,method=6)


def run():
    deliver=ALL_TABLES if '--all' in sys.argv else [t for t in ALL_TABLES if t in registered_tables()]
    prompts=json.loads((SOURCE/'prompts.json').read_text('utf-8'))['prompts']
    guided=lambda table,role:any(p['id']==table and p['kind']==role and p.get('guide') for p in prompts)
    OUT.mkdir(parents=True,exist_ok=True)
    records=[]

    def record(table,role,path,**extra):
        records.append({'table':table,'role':role,'file':'masters/'+path.name,'sha256':sha(path),**extra})

    for table in ALL_TABLES:
        concept=SOURCE/'masters'/(table+'-concept.png')
        if concept.exists():
            # The concept masters stay as source art; the lobby previews are game renders, see ui/3d/pinball/AGENTS.md.
            record(table,'concept',concept)
        for role in ['backglass','playfield','plastics','side']:
            path=SOURCE/'masters'/(table+'-'+role+'.png')
            if not path.exists():
                continue
            extra={'rev':2 if guided(table,role) else 1} if role=='playfield' else {}
            record(table,role,path,**extra)
            if table not in deliver:
                continue
            target=OUT/(table+'-'+role+'.webp')
            if role=='plastics':
                encode_plastics(path,target)
            elif role=='side':
                encode(path,target,'-vf','scale=2048:682:flags=lanczos','-c:v','libwebp','-quality','90','-compression_level','6')
            else:
                width,height=fit_size(path,(2048,1152) if role=='backglass' else (2048,4096))
                encode(path,target,'-vf',f'scale={width}:{height}:flags=lanczos','-c:v','libwebp','-quality','92','-compression_level','6')
    room=SOURCE/'masters'/'room-wall.png'
    if room.exists():
        record('room','wall',room)
        width,height=fit_size(room,(2048,2048))
        encode(room,OUT/'room-wall.webp','-vf',f'scale={width}:{height}:flags=lanczos','-c:v','libwebp','-quality','88','-compression_level','6')
    target=SOURCE/'artwork.json'
    previous=json.loads(target.read_text('utf-8')) if target.exists() else {}
    previous.update({'version':1,'generator':'Built-in ImageGen and Codex CLI','assets':records})
    target.write_text(json.dumps(previous,ensure_ascii=False,indent=2)+'\n',encoding='utf-8',newline='\n')


if __name__=='__main__': run()
