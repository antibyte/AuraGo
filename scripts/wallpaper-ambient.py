"""Derive the ambient light colours of the Desktop's photo wallpapers.

The Virtual Desktop tints its chrome with the wallpaper behind it (taskbar and dock from the
bottom band, the Fruity menubar from the top band, soft glows from the most luminous hue).
Run after adding or replacing an image in ui/img/wallpapers and paste the printed block into
ui/css/desktop-polish.css:

    python scripts/wallpaper-ambient.py
"""
import colorsys
from pathlib import Path

from PIL import Image

ROOT = Path(__file__).resolve().parents[1]
WALLPAPERS = ['groupshoot', 'alpine_dawn', 'city_rain', 'ocean_cliff', 'aurora_glass', 'nebula_flow', 'paper_waves']


def band(pixels, width, top, bottom):
    rows = [pixels[x, y] for y in range(top, bottom) for x in range(width)]
    return tuple(round(sum(p[i] for p in rows) / len(rows)) for i in range(3))


def glow(pixels, width, height):
    # Hue histogram weighted by saturation and brightness: the colour a light source would cast.
    bins = [[0.0, 0.0, 0.0, 0.0] for _ in range(24)]
    for y in range(height):
        for x in range(width):
            r, g, b = pixels[x, y][:3]
            h, s, v = colorsys.rgb_to_hsv(r / 255, g / 255, b / 255)
            weight = (s ** 1.5) * (v ** 2)
            slot = bins[int(h * 24) % 24]
            slot[0] += weight
            slot[1] += r * weight
            slot[2] += g * weight
            slot[3] += b * weight
    best = max(bins, key=lambda slot: slot[0])
    r, g, b = (best[i] / best[0] / 255 for i in (1, 2, 3))
    h, s, v = colorsys.rgb_to_hsv(r, g, b)
    # Normalise to a light-emitting tone: keep the hue, lift it to a usable glow.
    return tuple(round(c * 255) for c in colorsys.hsv_to_rgb(h, min(1, max(s, .45)), max(v, .82)))


def main():
    for name in WALLPAPERS:
        image = Image.open(ROOT / 'ui/img/wallpapers' / (name + '.jpg')).convert('RGB').resize((64, 36), Image.BILINEAR)
        pixels, width, height = image.load(), *image.size
        top, bottom = band(pixels, width, 0, 3), band(pixels, width, height - 5, height)
        light = glow(pixels, width, height)
        print(f'.desktop-body[data-wallpaper="{name}"] {{ --vd-ambient-top: {", ".join(map(str, top))}; '
              f'--vd-ambient-bottom: {", ".join(map(str, bottom))}; --vd-ambient-glow: {", ".join(map(str, light))}; }}')


if __name__ == '__main__':
    main()
