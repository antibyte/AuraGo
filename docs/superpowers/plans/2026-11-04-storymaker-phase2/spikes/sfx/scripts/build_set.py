"""python build_set.py recipe.json OUTDIR  -> OUTDIR/<name>_<k>.wav + OUTDIR/sheet.json (all numbers)
Run with earshot's venv python. Take k of a sound renders each layer on seed = entry seed + k."""
import sys, os, json, hashlib, subprocess
import numpy as np, soundfile as sf
sys.path.insert(0, 'C:/Users/Andi/.local/share/earshot')
from earshot.forge import bank, kit, sfx
from earshot import api

SR = 48000
recipe = json.load(open(sys.argv[1]))
out = sys.argv[2]
os.makedirs(out, exist_ok=True)
norm = recipe.get('normalise')


def render_take(spec, k, j=0):
    layers = []
    for L in spec['layers']:
        e = bank.get(L['sound'])
        y, m = bank.render(L['sound'], seed=int(e['seed']) + k + 1000 * j, checks=False, **L.get('edits', {}))
        y = np.asarray(y, np.float64)
        if y.ndim == 2:
            y = y.mean(1)
        layers.append((int(round(L['at_ms'] * SR / 1000)), y * 10 ** (L.get('gain_db', 0) / 20), int(e['seed']) + k + 1000 * j, e.get('type', 'forge')))
    n = max(a + len(y) for a, y, _, _ in layers)
    mix = np.zeros(n)
    for a, y, _, _ in layers:
        mix[a:a + len(y)] += y
    return mix * (10 ** (-1 / 20) / np.abs(mix).max()), [l[2] for l in layers]


raw = {}
for name, spec in recipe['sounds'].items():
    raw[name] = []
    for k in range(spec['takes']):
        for j in range(recipe.get('reroll', {}).get('max', 0) + 1):
            y, seeds = render_take(spec, k, j)
            ok = sfx.check(y.astype(np.float32))['ok']
            if ok or j == recipe.get('reroll', {}).get('max', 0):
                break
        raw[name].append((y, seeds, j, ok))

gains = {}
for name, takes in raw.items():
    gl = []
    for t in takes:
        y = t[0]
        g = norm['target_lufs'] - kit.loud_lufs(y)
        room = norm['ceiling_dbfs'] - 20 * np.log10(np.abs(y).max())
        gl.append(round(float(min(g, room)), 3))
    gains[name] = gl

sheet = {}
for name, takes in raw.items():
    rows = []
    for k, (y, seeds, rerolls, ok0) in enumerate(takes):
        y = y * 10 ** (gains[name][k] / 20)
        f = f'{out}/{name}_{k + 1}.wav'
        sf.write(f, y.astype(np.float32), SR, subtype='PCM_16')
        q, _ = sf.read(f, dtype='float64')
        t = bank.measure(q)
        mm = api.measure(f)
        chk = sfx.check(q.astype(np.float32))
        b = open(f, 'rb').read()
        rows.append({
            'file': os.path.basename(f), 'seeds': seeds, 'rerolls': rerolls, 'impact_ms': round((t['onset_s'] if recipe['sounds'][name]['impact'] == 'onset' else t['peak_s']) * 1000, 1), 'bytes': len(b), 'sha256': hashlib.sha256(b).hexdigest()[:16],
            'dur_ms': round(len(q) / SR * 1000, 1), 'onset_ms': round(t['onset_s'] * 1000, 1), 'peak_ms': round(t['peak_s'] * 1000, 1),
            'end_ms': round(t['end_s'] * 1000, 1), 'lufs_m_max': t['lufs_m_max'],
            'true_peak_dbtp': mm.get('true_peak_dbtp'), 'peak_dbfs': round(float(20 * np.log10(np.abs(q).max())), 2),
            'l50_lufs': kit.loud_lufs(q), 'check_ok': chk['ok'], 'check': chk['summary'],
        })
    sheet[name] = {'gain_db': gains[name], 'l50_spread_lu': round(max(r['l50_lufs'] for r in rows) - min(r['l50_lufs'] for r in rows), 2), 'layers': [(l['sound']) for l in recipe['sounds'][name]['layers']], 'takes': rows}
json.dump(sheet, open(f'{out}/sheet.json', 'w'), indent=1)
for n, s in sheet.items():
    print(n, 'gain', s['gain_db'])
    for r in s['takes']:
        print('  ', {k: r[k] for k in r if k not in ('check',)}, '|', r['check'][:90])
