"""Contact strips from delivered atlas rectangles, including every action/direction."""
from pathlib import Path
from functools import lru_cache
from PIL import Image,ImageDraw
import json
ROOT=Path(__file__).resolve().parents[2]
REPORT=ROOT/'reports/game-maker-worlds/motion'
REPORT.mkdir(parents=True,exist_ok=True)
for pack in ('aurago-pirates-topdown','aurago-pirates-side','aurago-isometric'):
    root=ROOT/'internal/gamemaker/asset_packs'/pack
    data=json.loads((root/'manifest.json').read_text());frames={f['id']:f for f in data['frames']}
    @lru_cache(maxsize=4)
    def page(name):return Image.open(root/name).convert('RGBA')
    for asset in data['assets']:
        clips=[c for c in data['animations'] if c['asset_id']==asset['id']]
        # All directions are recorded. Static per-direction contact strips are
        # useful too: they reveal asymmetric art and an incorrect forward angle.
        for start in range(0,len(clips),18):
            group=clips[start:start+18];board=Image.new('RGB',(1152,len(group)*108),(22,32,41));draw=ImageDraw.Draw(board)
            for row,clip in enumerate(group):
                label=clip['action']+' / '+clip['direction'];draw.text((8,row*108+3),label,fill='#ffffff')
                for index in range(8):
                    frame=frames[clip['frames'][min(len(clip['frames'])-1,index*len(clip['frames'])//8)]]
                    image=page(frame['atlas']).crop((frame['x'],frame['y'],frame['x']+frame['width'],frame['y']+frame['height']))
                    image.thumbnail((138,85),Image.Resampling.NEAREST)
                    board.paste(image,(index*144+(144-image.width)//2,row*108+20+(85-image.height)//2),image)
            board.save(REPORT/f'{pack}-{asset["id"]}-{start//18}.png')
    page.cache_clear()
print('Motion strips:',len(list(REPORT.glob('*.png'))))
