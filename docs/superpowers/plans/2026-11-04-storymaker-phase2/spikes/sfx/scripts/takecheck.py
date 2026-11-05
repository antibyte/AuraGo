import sys, os, glob, json, itertools
import numpy as np, soundfile as sf
sys.path.insert(0, 'C:/Users/Andi/.local/share/earshot')
from earshot import api
from earshot import vocab
from earshot.forge import bank
ROOT = 'C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/f2c97a7f-addd-4aa7-bd49-85d6c9b3f2a1/scratchpad/p2/spikes/sfx'
SET = ROOT + '/set'
names = ['whoosh', 'pop', 'click', 'ding', 'success', 'error', 'typing']
res = {}

def third_oct(y):
    F = np.abs(np.fft.rfft(y, 1 << 15)) ** 2
    f = np.fft.rfftfreq(1 << 15, 1 / 48000)
    c = 1000 * 2 ** (np.arange(-15, 13) / 3)
    return np.array([10 * np.log10(F[(f >= x / 2 ** (1 / 6)) & (f < x * 2 ** (1 / 6))].sum() + 1e-12) for x in c])

for n in names:
    fs = sorted(glob.glob(f'{SET}/{n}_*.wav'))
    ys = [sf.read(f, dtype='float64')[0] for f in fs]
    E = vocab.embed_files(fs)
    E = E / np.linalg.norm(E, axis=1, keepdims=True)
    rows = []
    for i, j in itertools.combinations(range(len(fs)), 2):
        L = min(len(ys[i]), len(ys[j]))
        a, b = ys[i][:L], ys[j][:L]
        resid = 10 * np.log10(((a - b) ** 2).sum() / ((a ** 2).sum() + 1e-18) + 1e-18)
        xc = float(np.max(np.abs(np.correlate(a, b, 'full'))) / (np.linalg.norm(a) * np.linalg.norm(b) + 1e-18))
        sd = float(np.abs(third_oct(a) - third_oct(b)).mean())
        rows.append({'pair': f'{i + 1}-{j + 1}', 'resid_db': round(resid, 1), 'xcorr_max': round(xc, 3), 'third_oct_mean_abs_db': round(sd, 2), 'clap_cos': round(float(E[i] @ E[j]), 3)})
    res[n] = rows
    print(n, 'pairs', len(rows), 'resid_db min/max', min(r['resid_db'] for r in rows), max(r['resid_db'] for r in rows),
          'xcorr max', max(r['xcorr_max'] for r in rows), 'clap min/max', min(r['clap_cos'] for r in rows), max(r['clap_cos'] for r in rows),
          'bands dB mean', round(np.mean([r['third_oct_mean_abs_db'] for r in rows]), 2), flush=True)
json.dump(res, open(ROOT + '/takecheck.json', 'w'), indent=1)

# realness + describe of the final takes
fin = {}
for n in names:
    for f in sorted(glob.glob(f'{SET}/{n}_*.wav')):
        r = api.realness(f)
        d = api.describe(f)
        fin[os.path.basename(f)] = {'p_real': r['p_real'], 'verdict': r['verdict'], 'tags': [(t['term'], t['z']) for t in d['top_tags'][:2]],
                                    'audioset': [(a['label'], a['p']) for a in d['audioset'][:2]]}
        print(os.path.basename(f), r['p_real'], r['verdict'], fin[os.path.basename(f)]['tags'], fin[os.path.basename(f)]['audioset'], flush=True)
json.dump(fin, open(ROOT + '/final_listening.json', 'w'), indent=1)
