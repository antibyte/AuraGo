import sys
import numpy as np
sys.path.insert(0, 'C:/Users/Andi/.local/share/earshot')
from earshot.forge import bank, fit as F, sfx
for sid in ['fit/click/floss_lid', 'fit/whoosh.short/bamboo_swish', 'fit/bell/small_bell']:
    m = bank.get(sid)['model']
    print(sid)
    for vary in (0.0, 0.15, 0.3):
        ys = [np.asarray(F.render(m, seed=s, vary=vary, take=True, bed=False), np.float64) for s in (1, 2, 3)]
        same_seed = np.asarray(F.render(m, seed=1, vary=vary, take=True, bed=False), np.float64)
        det = bool(len(same_seed) == len(ys[0]) and np.array_equal(same_seed, ys[0]))
        r = []
        for a, b in ((0, 1), (0, 2), (1, 2)):
            L = min(len(ys[a]), len(ys[b])); x, y = ys[a][:L], ys[b][:L]
            x = x / np.abs(x).max(); y = y / np.abs(y).max()
            r.append(round(10 * np.log10(((x - y) ** 2).sum() / ((x ** 2).sum() + 1e-18) + 1e-18), 1))
        print('  vary', vary, 'same seed identical', det, 'seed-pair residual dB', r, 'len', [len(y) for y in ys])
# forge: seed only, no vary parameter
y1, _ = bank.render('pop/0', seed=5, checks=False); y2, _ = bank.render('pop/0', seed=5, checks=False); y3, _ = bank.render('pop/0', seed=6, checks=False)
print('pop/0 seed5==seed5', bool(np.array_equal(y1, y2)), 'seed5==seed6', bool(np.array_equal(y1, y3)))
