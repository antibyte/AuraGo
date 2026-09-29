"""Deterministic generator for the System World surface detail textures.

The city kit's GLBs carry no UV coordinates, so the renderer projects these
tileable maps triplanar in world or object space (sysworld-surfaces.js). Each map
packs three data channels into one RGB WebP:

    R = albedo factor / 2   (128 means 1.0x the material's own colour)
    G = roughness factor / 2
    B = height (bump via screen-space derivatives; low-frequency lows hold puddles)

Everything is original procedural work (MIT) built from seeded periodic noise and
periodic Voronoi cells, so every map tiles seamlessly. Output is byte-stable for a
given numpy/Pillow installation.

    python assets/system-world/build_textures.py            # write textures + manifest
    python assets/system-world/build_textures.py --check    # verify committed bytes
    python assets/system-world/build_textures.py --preview  # contact sheet in reports/
"""
from __future__ import annotations

import argparse
import hashlib
import io
import json
import sys
from pathlib import Path

import numpy as np
from PIL import Image, ImageDraw

ROOT = Path(__file__).resolve().parents[2]
OUT = ROOT / 'ui' / '3d' / 'system-world' / 'textures' / 'v1'
PREVIEW = ROOT / 'reports' / 'system-world-assets' / 'surface-textures.png'
LICENSE = ROOT / 'ui' / '3d' / 'system-world' / 'v2' / 'LICENSE.txt'
QUALITY = 88


def smoothstep(e0, e1, x):
    t = np.clip((x - e0) / (e1 - e0), 0.0, 1.0)
    return t * t * (3.0 - 2.0 * t)


def grid(size):
    u = (np.arange(size) + 0.5) / size
    return np.meshgrid(u, u)  # x varies along columns, y along rows


def spectral(size, seed, beta=2.0, lo=1.0, hi=None, stretch=(1.0, 1.0)):
    """Periodic 1/f^beta noise between lo and hi cycles per tile, zero mean, unit variance.

    stretch=(sx, sy) weights horizontal/vertical frequencies; a large sy suppresses
    vertical variation and yields streaks that run vertically."""
    rng = np.random.default_rng(seed)
    hi = hi or size / 2
    fy = np.fft.fftfreq(size) * size * stretch[1]
    fx = np.fft.rfftfreq(size) * size * stretch[0]
    f = np.sqrt(fy[:, None] ** 2 + fx[None, :] ** 2)
    amp = np.zeros_like(f)
    nonzero = f > 0
    amp[nonzero] = f[nonzero] ** (-beta / 2.0)
    amp *= smoothstep(lo * 0.5, lo, f) * (1.0 - smoothstep(hi, hi * 1.5, f))
    field = np.fft.irfft2(np.fft.rfft2(rng.standard_normal((size, size))) * amp, s=(size, size))
    field -= field.mean()
    return field / (field.std() + 1e-12)


def voronoi(size, cells, seed, jitter=0.9, warp=None):
    """Periodic jittered-grid Voronoi: nearest/second-nearest distance (cell units) and a random value per cell."""
    rng = np.random.default_rng(seed)
    points = 0.5 + (rng.random((cells, cells, 2)) - 0.5) * jitter
    values = rng.random(cells * cells)
    x, y = grid(size)
    px, py = x * cells, y * cells
    if warp is not None:
        px, py = px + warp[0], py + warp[1]
    ix, iy = np.floor(px).astype(np.int64), np.floor(py).astype(np.int64)
    d1 = np.full((size, size), 1e9)
    d2 = np.full((size, size), 1e9)
    ids = np.zeros((size, size), np.int64)
    for dy in (-1, 0, 1):
        for dx in (-1, 0, 1):
            cx, cy = (ix + dx) % cells, (iy + dy) % cells
            d = np.hypot(px - (ix + dx + points[cy, cx, 0]), py - (iy + dy + points[cy, cx, 1]))
            closer = d < d1
            d2 = np.where(closer, d1, np.minimum(d2, d))
            d1 = np.where(closer, d, d1)
            ids = np.where(closer, cy * cells + cx, ids)
    return d1, d2, values[ids]


def unit(field, lo=2.0, hi=98.0):
    a, b = np.percentile(field, [lo, hi])
    return np.clip((field - a) / (b - a + 1e-12), 0.0, 1.0)


def asphalt(size):
    """Fine mottling, light/dark aggregate, meandering cracks (some sealed and glossy), a patch."""
    x, y = grid(size)
    base = spectral(size, 11, beta=1.5, lo=6, hi=220)
    macro = spectral(size, 12, beta=3.2, lo=1, hi=5)
    d1, _, val = voronoi(size, 150, 13)
    stone = smoothstep(0.45, 0.2, d1)
    light, dark = stone * (val > 0.84), stone * (val < 0.12)
    warp = (spectral(size, 14, beta=2.6, lo=2, hi=24) * 0.12, spectral(size, 15, beta=2.6, lo=2, hi=24) * 0.12)
    c1, c2, cval = voronoi(size, 5, 16, jitter=0.95, warp=warp)
    edge = c2 - c1
    keep = smoothstep(-0.1, 0.5, spectral(size, 17, beta=2.2, lo=2, hi=16)) * (cval > 0.35)
    width = 0.010 + 0.004 * spectral(size, 18, beta=2.0, lo=4, hi=40)
    crack = smoothstep(width, width * 0.25, edge) * keep
    sealed = smoothstep(width * 3.2, width * 2.4, edge) * (cval > 0.72) * (1.0 - crack)
    region = unit(spectral(size, 19, beta=3.0, lo=1, hi=2))  # one or two large patches, no small ring islands
    patch = smoothstep(0.74, 0.77, region)
    seam = smoothstep(0.012, 0.0, np.abs(region - 0.755))
    albedo = (1.0 + 0.06 * base + 0.32 * light - 0.2 * dark - 0.5 * crack - 0.3 * sealed) * (1.0 - 0.13 * patch) - 0.15 * seam
    rough = (1.0 + 0.07 * base + 0.18 * crack - 0.35 * sealed) * (1.0 - 0.12 * patch)
    height = 0.5 + 0.13 * macro / 3.0 + 0.16 * stone * (val - 0.5) + 0.05 * base / 3.0 - 0.35 * crack - 0.06 * sealed - 0.08 * seam
    return albedo, rough, height


def pavers(size):
    """Running-bond pavers (8 x 16 per tile) with bevels, grout, per-stone tone and tilt."""
    x, y = grid(size)
    rows, per_row = 16, 8
    row = np.floor(y * rows).astype(np.int64)
    ly = y * rows - row
    bx = x * per_row + (row % 2) * 0.5
    col = np.floor(bx).astype(np.int64) % per_row
    lx = bx - np.floor(bx)
    ex, ey = np.minimum(lx, 1.0 - lx) * size / per_row, np.minimum(ly, 1.0 - ly) * size / rows
    edge = np.minimum(ex, ey)
    grout = smoothstep(2.4, 1.1, edge)
    bevel = smoothstep(1.1, 7.0, edge)
    rng = np.random.default_rng(21)
    stone = row * per_row + col
    tone, tilt_x, tilt_y, stain = (rng.random(rows * per_row)[stone] for _ in range(4))
    speck = spectral(size, 22, beta=1.1, lo=40, hi=400)
    macro = spectral(size, 23, beta=3.2, lo=1, hi=4)
    wear = unit(spectral(size, 24, beta=2.4, lo=2, hi=20))
    albedo = (0.9 + 0.2 * tone) * (1.0 + 0.045 * speck) * np.where(stain > 0.93, 0.84, 1.0) * (0.96 + 0.08 * wear)
    albedo = albedo * (1.0 - grout) + 0.58 * grout
    rough = (0.95 + 0.1 * tone) * (1.0 + 0.05 * speck) * (1.0 - grout) + 1.28 * grout
    tilt = ((tilt_x - 0.5) * (lx - 0.5) + (tilt_y - 0.5) * (ly - 0.5)) * 0.12
    height = 0.28 + 0.46 * bevel + 0.03 * speck / 3.0 + tilt * bevel + 0.12 * macro / 3.0
    height = height * (1.0 - grout) + 0.14 * grout
    return albedo, rough, height


def concrete(size):
    """Board-formed concrete: mottling, pores, faint rain streaks, 2 x 1 m form seams and tie holes."""
    x, y = grid(size)
    mottle = spectral(size, 31, beta=2.4, lo=2, hi=64)
    fine = spectral(size, 32, beta=1.0, lo=40, hi=256)
    d1, _, val = voronoi(size, 110, 33)
    pore = (val < 0.14) * smoothstep(0.24, 0.07, d1)
    streak = np.clip(spectral(size, 34, beta=1.8, lo=6, hi=90, stretch=(1.0, 7.0)), 0.0, None)
    sx = np.abs(x * 2.0 - np.round(x * 2.0)) * size / 2.0
    sy = np.abs(y * 4.0 - np.round(y * 4.0)) * size / 4.0
    seam = smoothstep(1.7, 0.5, np.minimum(sx, sy))
    hx = np.abs(x * 4.0 - np.floor(x * 4.0) - 0.5) * size / 4.0
    hy = np.abs(y * 8.0 - np.floor(y * 8.0) - 0.5) * size / 8.0
    tie = smoothstep(2.6, 1.2, np.hypot(hx, hy))
    albedo = 1.0 + 0.1 * mottle + 0.03 * fine - 0.28 * pore - 0.13 * seam - 0.07 * streak - 0.35 * tie
    rough = 1.0 + 0.05 * mottle + 0.1 * pore + 0.05 * seam - 0.04 * streak
    height = 0.5 + 0.05 * mottle / 3.0 + 0.03 * fine / 3.0 - 0.3 * pore - 0.26 * seam - 0.32 * tie
    return albedo, rough, height


def panels(size):
    """Metal cladding: 2 x 1 m panels with seams, oil-canning tone, horizontal brushing and corner rivets."""
    x, y = grid(size)
    cols, rows = 4, 8
    col, row = np.floor(x * cols).astype(np.int64), np.floor(y * rows).astype(np.int64)
    lx, ly = x * cols - col, y * rows - row
    ex, ey = np.minimum(lx, 1.0 - lx) * size / cols, np.minimum(ly, 1.0 - ly) * size / rows
    seam = smoothstep(2.0, 0.6, np.minimum(ex, ey))
    rng = np.random.default_rng(41)
    panel = row * cols + col
    tone, gloss, bulge = (rng.random(cols * rows)[panel] for _ in range(3))
    brushed = spectral(size, 42, beta=1.4, lo=12, hi=256, stretch=(9.0, 1.0))
    rivet = smoothstep(2.1, 1.0, np.hypot(ex - 6.0, ey - 6.0))  # inset 6 px from every panel corner
    dome = np.sin(np.pi * lx) * np.sin(np.pi * ly)
    albedo = (0.95 + 0.1 * tone) * (1.0 + 0.025 * brushed) - 0.22 * seam + 0.08 * rivet
    rough = (0.88 + 0.24 * gloss) * (1.0 + 0.05 * brushed) + 0.22 * seam - 0.1 * rivet
    height = 0.55 + 0.1 * dome * bulge + 0.02 * brushed / 3.0 - 0.4 * seam + 0.28 * rivet
    return albedo, rough, height


def gravel(size):
    """Roof gravel: rounded pebbles of varied tone with dark gaps."""
    warp = (spectral(size, 52, beta=2.6, lo=2, hi=40) * 0.18, spectral(size, 53, beta=2.6, lo=2, hi=40) * 0.18)
    d1, d2, val = voronoi(size, 64, 51, jitter=0.95, warp=warp)
    pebble = smoothstep(0.62, 0.12, d1)
    gap = smoothstep(0.09, 0.0, d2 - d1)
    albedo = (0.72 + 0.56 * val) * (0.9 + 0.1 * pebble) * (1.0 - 0.45 * gap)
    rough = 1.1 - 0.1 * pebble * val + 0.2 * gap
    height = 0.22 + 0.62 * pebble * (0.7 + 0.3 * val) - 0.22 * gap
    return albedo, rough, height


def foliage(size):
    """Leaf clumps with dark gaps and large-scale tone variation."""
    warp = (spectral(size, 62, beta=2.6, lo=2, hi=30) * 0.3, spectral(size, 63, beta=2.6, lo=2, hi=30) * 0.3)
    d1, d2, val = voronoi(size, 36, 61, jitter=0.95, warp=warp)
    leaf = smoothstep(0.62, 0.08, d1)
    gap = smoothstep(0.13, 0.0, d2 - d1)
    low = spectral(size, 64, beta=2.5, lo=1, hi=8)
    albedo = (0.72 + 0.56 * val) * (0.74 + 0.26 * leaf) * (1.0 - 0.5 * gap) * (1.0 + 0.12 * low)
    rough = 1.0 + 0.15 * gap - 0.1 * leaf
    height = 0.3 + 0.55 * leaf - 0.3 * gap + 0.05 * low / 3.0
    return albedo, rough, height


# id, generator, pixels, metres per tile (the renderer reads tile sizes from the manifest)
TEXTURES = [
    ('asphalt', asphalt, 1024, 6.0),
    ('pavers', pavers, 1024, 4.8),
    ('concrete', concrete, 512, 4.0),
    ('panels', panels, 512, 8.0),
    ('gravel', gravel, 512, 3.0),
    ('foliage', foliage, 512, 3.0),
]


def encode(albedo, rough, height):
    rgb = np.stack([albedo / 2.0, rough / 2.0, height], axis=-1)
    pixels = np.clip(np.round(rgb * 255.0), 0, 255).astype(np.uint8)
    buffer = io.BytesIO()
    Image.fromarray(pixels, 'RGB').save(buffer, 'WEBP', quality=QUALITY, method=6)
    return buffer.getvalue(), pixels


def build():
    files, previews, entries = {}, [], []
    for name, generator, size, metres in TEXTURES:
        data, pixels = encode(*generator(size))
        files[name + '.webp'] = data
        previews.append((name, pixels, metres))
        entries.append({'id': name, 'file': name + '.webp', 'px': size, 'tile_metres': metres,
                        'bytes': len(data), 'sha256': hashlib.sha256(data).hexdigest()})
    manifest = {
        'schema': 1, 'license': 'MIT', 'generator': 'assets/system-world/build_textures.py',
        'generator_sha256': hashlib.sha256(Path(__file__).read_bytes().replace(b'\r\n', b'\n')).hexdigest(),
        'encoding': {'r': 'albedo factor / 2', 'g': 'roughness factor / 2', 'b': 'height'},
        'textures': entries,
    }
    files['manifest.json'] = (json.dumps(manifest, indent=2) + '\n').encode()
    files['LICENSE.txt'] = LICENSE.read_bytes()
    return files, previews


def preview(previews):
    """Lit swatches (height-derived normals under a raking light) plus the raw channels."""
    tile, pad = 256, 12
    sheet = Image.new('RGB', (pad + 4 * (tile + pad), pad + len(previews) * (tile + pad + 18)), (18, 22, 28))
    draw = ImageDraw.Draw(sheet)
    for index, (name, pixels, metres) in enumerate(previews):
        data = pixels.astype(np.float64) / 255.0
        albedo, rough, height = data[..., 0] * 2.0, data[..., 1] * 2.0, data[..., 2]
        gy, gx = np.gradient(np.pad(height, 1, mode='wrap'))
        gx, gy = gx[1:-1, 1:-1] * 24.0, gy[1:-1, 1:-1] * 24.0
        normal = np.stack([-gx, -gy, np.ones_like(gx)], axis=-1)
        normal /= np.linalg.norm(normal, axis=-1, keepdims=True)
        light = np.array([-0.55, -0.45, 0.7]) / np.linalg.norm([-0.55, -0.45, 0.7])
        lit = np.clip((normal @ light) * 0.85 + 0.15, 0.0, 1.0) * albedo * 0.5
        swatches = [lit, albedo * 0.5, rough * 0.5, height]
        top = pad + index * (tile + pad + 18)
        draw.text((pad, top), f'{name}  {pixels.shape[0]} px / {metres:g} m   lit | albedo | roughness | height', fill=(210, 220, 230))
        for column, swatch in enumerate(swatches):
            image = Image.fromarray(np.clip(swatch * 255.0, 0, 255).astype(np.uint8), 'L').resize((tile, tile), Image.LANCZOS)
            sheet.paste(image.convert('RGB'), (pad + column * (tile + pad), top + 16))
    PREVIEW.parent.mkdir(parents=True, exist_ok=True)
    sheet.save(PREVIEW)
    print('Preview:', PREVIEW)


def main():
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument('--check', action='store_true', help='verify committed textures instead of writing')
    parser.add_argument('--preview', action='store_true', help='write a contact sheet to reports/')
    args = parser.parse_args()
    files, previews = build()
    if args.check:
        stale = [name for name, data in files.items() if not (OUT / name).exists() or (OUT / name).read_bytes() != data]
        extra = sorted({p.name for p in OUT.iterdir()} - set(files)) if OUT.exists() else []
        if stale or extra:
            print('Rebuild surface textures; stale:', ', '.join(stale) or '-', 'unexpected:', ', '.join(extra) or '-')
            return 1
        print('Surface textures verified:', sum(len(d) for n, d in files.items() if n.endswith('.webp')), 'bytes')
        return 0
    OUT.mkdir(parents=True, exist_ok=True)
    for name, data in files.items():
        (OUT / name).write_bytes(data)
    total = sum(len(d) for n, d in files.items() if n.endswith('.webp'))
    print(f'Surface textures written: {len(TEXTURES)} maps, {total} bytes')
    if args.preview:
        preview(previews)
    return 0


if __name__ == '__main__':
    sys.exit(main())
