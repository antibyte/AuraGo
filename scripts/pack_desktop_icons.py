"""Pack reviewed Imagegen miniature icons; requires Pillow.
Crop, scale and pack supplied artwork preserving soft alpha and 4px gutters.
"""
import argparse, hashlib, io, json
from pathlib import Path
from PIL import Image
ROOT = Path(__file__).resolve().parents[1]
ASSETS = ROOT / 'ui/img/desktop-mini'
SOURCES = ROOT / 'assets/desktop-mini'
NAMES = '''file-plus folder-plus clipboard copy scissors trash edit save
settings wallpaper layout apps search upload download printer
file folder home monitor terminal code archive notes
chat mail phone microphone bell users shield key
calendar todo clock analytics cpu network server database
image camera video music speaker headphones radio games
palette eyedropper crop cube sliders puzzle package globe
run bookmark star heart attachment link help info'''.split()

def write(path, data, check):
    if check:
        if not path.exists() or path.read_bytes() != data:
            raise ValueError(f'Outdated desktop mini artifact: {path}')
    else:
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(data)

def pack(check=False):
    manifest = {'version': 1, 'icon_size': 64, 'width': 512, 'height': 512,
                'images': {}, 'sources': {}, 'icons': {}}
    for theme in ('standard', 'fruity'):
        source = SOURCES / f'{theme}.png'
        image = Image.open(source).convert('RGBA')
        assert image.getchannel('A').getextrema()[0] == 0, 'Real alpha required'
        sheet = Image.new('RGBA', (512, 512))
        for index, name in enumerate(NAMES):
            col, row = index % 8, index // 8
            crop = image.crop((round(col*image.width/8), round(row*image.height/8),
                               round((col+1)*image.width/8), round((row+1)*image.height/8)))
            # Ignore invisible alpha specks only when measuring artwork bounds.
            bounds = crop.getchannel('A').point(lambda a: 255 if a >= 24 else 0).getbbox()
            assert bounds, f'Missing icon {theme}/{name}'
            crop = crop.crop(bounds)
            crop.thumbnail((56, 56), Image.Resampling.LANCZOS)
            sheet.alpha_composite(crop, (col*64+(64-crop.width)//2, row*64+(64-crop.height)//2))
            manifest['icons'][name] = {'x': col*64, 'y': row*64}
        output = io.BytesIO()
        sheet.save(output, format='PNG', optimize=True)
        write(ASSETS / f'{theme}.png', output.getvalue(), check)
        manifest['images'][theme] = f'/img/desktop-mini/{theme}.png'
        manifest['sources'][theme] = hashlib.sha256(source.read_bytes()).hexdigest()
    write(ASSETS / 'manifest.json', (json.dumps(manifest, indent=2)+'\n').encode(), check)
    print('Desktop mini icons: 64 motifs x 2 themes, alpha and packing verified')

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--check', action='store_true')
    pack(parser.parse_args().check)
