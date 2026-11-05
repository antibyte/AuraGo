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

for dur in (0.7, 0.9):
    y, m = bank.render('fit/bell/small_bell', dur=dur)
    print('bell dur', dur, m['dur_s'], m['peak_s'], m.get('checks', {}).get('summary'))
# success: two-three bell notes rising (pitch is the measured heard partial; knob range one octave)
for p1, p2, name in (('C7', 'G7', 'succ_C7G7'), ('E6', 'B6', 'succ_E6B6'), ('G6', 'D7', 'succ_G6D7')):
    try:
        put(name, [layer('fit/bell/small_bell', 0, dur=0.8, pitch=p1), layer('fit/bell/small_bell', 140, dur=0.8, pitch=p2)])
    except Exception as e:
        print(name, 'FAIL', e)
# error: descending low double knock / low bell pair
for p, name in (('G2', 'err_book_G2'), ('E2', 'err_book_E2')):
    try:
        put(name, [layer('fit/land.light/fingertip_book', 0, pitch=p), layer('fit/land.light/fingertip_book', 150, gain_db=-1, pitch=p)])
    except Exception as e:
        print(name, 'FAIL', e)
try:
    put('err_wood_G3', [layer('fit/land.light/finger_wood', 0), layer('fit/hit/bamboo_tok', 130)])
except Exception as e:
    print('FAIL', e)
try:
    put('err_bell_lowpair', [layer('fit/bell/small_bell', 0, dur=0.7, pitch='A6'), layer('fit/bell/small_bell', 160, dur=0.7, pitch='E6')])
except Exception as e:
    print('FAIL', e)
