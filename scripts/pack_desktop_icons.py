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
ACTION_NAMES = '''check-square square sort refresh undo redo list grid
columns eye eye-off zoom-in zoom-out external keyboard contrast'''.split()

def write(path, data, check):
    if check:
        if not path.exists() or path.read_bytes() != data:
            raise ValueError(f'Outdated desktop mini artifact: {path}')
    else:
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(data)

def pack(check=False):
    manifest = {'version': 2, 'icon_size': 64, 'width': 512, 'height': 640,
                'images': {}, 'sources': {}, 'icons': {}}
    for theme in ('standard', 'fruity'):
        sheet = Image.new('RGBA', (512, 640))
        # Imagegen's cell spacing is not uniform; split within reviewed alpha lanes.
        for suffix, names, offset in (('', NAMES, 0), ('-actions', ACTION_NAMES, 64)):
            source = SOURCES / f'{theme}{suffix}.png'
            image = Image.open(source).convert('RGBA')
            assert image.getchannel('A').getextrema()[0] == 0, 'Real alpha required'
            assert image.size == (1254, 1254), 'Re-review source sheet crop lanes'
            if suffix:
                # Reviewed transparent lanes in the generated 4x4 source sheets.
                xs, ys = [0, 334, 626, 938, 1254], [0, 339, 616, 904, 1254]
            elif theme == 'standard':
                xs = [0, 163, 315, 470, 625, 777, 936, 1084, 1254]
                ys = [0, 163, 317, 466, 618, 766, 916, 1072, 1254]
            else:
                xs = [0, 174, 324, 470, 626, 780, 934, 1080, 1254]
                ys = [0, 190, 340, 489, 644, 793, 938, 1088, 1254]
            for index, name in enumerate(names):
                source_col, source_row = index % (len(xs)-1), index // (len(xs)-1)
                crop = image.crop((xs[source_col], ys[source_row], xs[source_col+1], ys[source_row+1]))
                # Ignore invisible alpha specks only when measuring artwork bounds.
                bounds = crop.getchannel('A').point(lambda a: 255 if a >= 24 else 0).getbbox()
                assert bounds, f'Missing icon {theme}/{name}'
                crop = crop.crop(bounds)
                crop.thumbnail((56, 56), Image.Resampling.LANCZOS)
                col, row = (offset+index) % 8, (offset+index) // 8
                sheet.alpha_composite(crop, (col*64+(64-crop.width)//2, row*64+(64-crop.height)//2))
                manifest['icons'][name] = {'x': col*64, 'y': row*64}
            manifest['sources'][theme+suffix] = hashlib.sha256(source.read_bytes()).hexdigest()
        output = io.BytesIO()
        sheet.save(output, format='PNG', optimize=True)
        write(ASSETS / f'{theme}.png', output.getvalue(), check)
        manifest['images'][theme] = f'/img/desktop-mini/{theme}.png'
    write(ASSETS / 'manifest.json', (json.dumps(manifest, indent=2)+'\n').encode(), check)
    print('Desktop mini icons: 80 motifs x 2 themes, alpha and packing verified')

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--check', action='store_true')
    pack(parser.parse_args().check)
