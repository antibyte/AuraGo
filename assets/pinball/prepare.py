"""Convert retained raster masters for delivery; do not repaint or crop them."""
import hashlib
import json
from pathlib import Path
import struct
import subprocess

ROOT=Path(__file__).resolve().parents[2]
SOURCE=Path(__file__).parent
OUT=ROOT/'ui/3d/pinball/v1'


def run():
    OUT.mkdir(parents=True,exist_ok=True)
    records=[]
    for table in ['neon','leviathan','temple']:
        for role,size in [('concept',(640,960)),('backglass',(2048,1152)),('playfield',(2048,4096))]:
            path=SOURCE/'masters'/(table+'-'+role+'.png')
            width,height=struct.unpack('>II',path.read_bytes()[16:24])
            ratio=min(1,size[0]/width,size[1]/height)
            target_width=max(2,int(width*ratio)//2*2);target_height=max(2,int(height*ratio)//2*2)
            subprocess.run(['ffmpeg','-v','error','-y','-i',str(path),'-vf',f'scale={target_width}:{target_height}:flags=lanczos','-c:v','libwebp','-quality','92','-compression_level','6',str(OUT/(table+'-'+role+'.webp'))],check=True)
            records.append({'table':table,'role':role,'file':'masters/'+path.name,'sha256':hashlib.sha256(path.read_bytes()).hexdigest()})
    target=SOURCE/'artwork.json'
    previous=json.loads(target.read_text('utf-8')) if target.exists() else {}
    previous.update({'version':1,'generator':'Built-in ImageGen','assets':records})
    target.write_text(json.dumps(previous,ensure_ascii=False,indent=2)+'\n',encoding='utf-8',newline='\n')


if __name__=='__main__': run()
