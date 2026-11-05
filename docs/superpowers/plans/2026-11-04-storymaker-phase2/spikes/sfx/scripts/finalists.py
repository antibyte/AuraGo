import sys, json, os
sys.path.insert(0, 'C:/Users/Andi/.local/share/earshot')
from earshot import api
ROOT = 'C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/f2c97a7f-addd-4aa7-bd49-85d6c9b3f2a1/scratchpad/p2/spikes/sfx'
C = ROOT + '/cand/'
F = {
 'whoosh': ['fit__whoosh.short__bamboo_swish', 'whoosh_fast__tight', 'swish__blade'],
 'pop': ['pop__0', 'fit__pop__mouth_plop', 'pro_pop__bloop', 'pop__3'],
 'click': ['pro_click__soft', 'fit__click__floss_lid', 'fit__snap__switch_flip', 'click__snap'],
 'ding': ['notify__mid_wood', 'pro_notify__ding_1822', 'mallet__glock', 'fit__bell__small_bell_dur1.2', 'pro_notify__ding_2045'],
 'success': ['mallet__up4', 'mallet__flourish', 'shimmer__m2_dur1.4'],
 'error': ['pro_zip__down_dur0.5', 'glitch__mid', 'pro_sub_drop__quick', 'glitch__chirp'],
 'typing': ['typing_key__thock', 'fit__key__thock', 'fit__key__lubed', 'fit__key__medium'],
}
out = {}
for name, lst in F.items():
    for f in lst:
        p = f'{C}{name}/{f}.wav'
        r = api.realness(p)
        d = api.describe(p)
        tags = [(t['term'], t['z']) for t in d['top_tags'][:3]]
        out[f'{name}/{f}'] = {'p_real': r.get('p_real'), 'verdict': r.get('verdict'), 'tags': tags, 'audioset': [(a['label'],a['p']) for a in d['audioset'][:3]]}
        print(name, f, r.get('p_real'), r.get('verdict'), tags, str([(a['label'],a['p']) for a in d['audioset'][:3]])[:150], flush=True)
json.dump(out, open(ROOT + '/finalists.json', 'w'), indent=1, default=str)
