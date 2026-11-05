import sys, os
import numpy as np, soundfile as sf
sys.path.insert(0, 'C:/Users/Andi/.local/share/earshot')
from earshot.forge import bank
from earshot import api
ROOT = 'C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/f2c97a7f-addd-4aa7-bd49-85d6c9b3f2a1/scratchpad/p2/spikes/sfx'
D = ROOT + '/exp'
os.makedirs(D, exist_ok=True)
SR = 48000

def layer(sid, at_ms, gain_db=0.0, **kw):
    y, m = bank.render(sid, **kw)
    if y.ndim == 2:
        y = y.mean(1)
    return int(at_ms * SR / 1000), y * 10 ** (gain_db / 20), m

def mix(layers):
    n = max(a + len(y) for a, y, _ in layers)
    out = np.zeros(n)
    for a, y, _ in layers:
        out[a:a + len(y)] += y
    return out * (10 ** (-1 / 20) / np.abs(out).max())

def put(name, layers):
    y = mix(layers)
    f = f'{D}/{name}.wav'
    sf.write(f, y.astype(np.float32), SR, subtype='PCM_16')
    r = api.realness(f)
    d = api.describe(f)
    print(name, 'dur', round(len(y) / SR, 3), 'p_real', r['p_real'], r['verdict'], [(t['term'], t['z']) for t in d['top_tags'][:3]], flush=True)


put('err_book_down', [layer('fit/land.light/fingertip_book', 0, pitch=199.4), layer('fit/land.light/fingertip_book', 150, gain_db=0, pitch=100.0)])
put('err_book_down2', [layer('fit/land.light/finger_wood', 0), layer('fit/land.light/fingertip_book', 150, pitch=100.0)])
put('succ_3', [layer('fit/bell/small_bell', 0, dur=0.8, pitch='C7'), layer('fit/bell/small_bell', 110, dur=0.8, pitch='E7'), layer('fit/bell/small_bell', 220, dur=0.9, pitch='G7')])
y, m = bank.render('pro_zip/down', dur=0.5); sf.write(D+'/err_zipdown.wav', y, SR, subtype='PCM_16')
for q in ('an error buzzer', 'a negative wrong-answer sound, two low descending thuds', 'a pleasant rising success chime'):
    r = api.rank(D, q); print(q); [print('  ', x['name'], x['z'], x['verdict']) for x in r['results']]
