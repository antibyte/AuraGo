import sys, json, os
import soundfile as sf
from earshot.forge import bank
from earshot import api

ROOT = 'C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/f2c97a7f-addd-4aa7-bd49-85d6c9b3f2a1/scratchpad/p2/spikes/sfx'
CAND = ROOT + '/cand'

C = {
 'whoosh': ('a short fast whoosh swish through the air', [
   ('swish/bright', {}), ('swish/flick', {}), ('swish/blade', {}), ('whoosh_fast/tight', {}), ('whoosh_fast/heavy', {}),
   ('fit/whoosh.short/bamboo_swish', {}), ('swipe/cloth', {}), ('swipe/paper', {}), ('swipe/flick', {}),
   ('whoosh_fast/crisp', {'dur': 0.75, 'apex': 0.25}), ('whoosh_fast/whip', {'dur': 0.75, 'apex': 0.25})]),
 'pop': ('a soft natural bubble pop', [
   ('fit/pop/mouth_plop', {}), ('fit/pop/lip_pop', {}), ('ui_pop/soft', {}), ('ui_pop/step', {}), ('ui_pop/boop', {}),
   ('pro_pop_in/round', {}), ('pop/0', {}), ('pop/1', {}), ('pop/3', {}), ('pro_pop/bloop', {}), ('drip/bubble', {}), ('drip/plink', {})]),
 'click': ('a soft UI button click', [
   ('fit/click/floss_lid', {}), ('click/soft', {}), ('click/snap', {}), ('click/thock', {}), ('pro_click/mouse', {}), ('pro_click/soft', {}),
   ('tap/finger', {}), ('fit/snap/switch_flip', {}), ('toggle/on', {}), ('fit/tick/domino', {}), ('shutter/dslr', {})]),
 'ding': ('a short clean bell ding notification', [
   ('notify/mid_tine', {}), ('notify/mid_wood', {}), ('pro_notify/ding_2045', {}), ('pro_notify/ding_1822', {}), ('pro_notify/ding_1930', {}),
   ('mallet/glock', {}), ('mallet/celesta', {}), ('notify/m0', {'dur': 1.0}), ('notify/m2', {'dur': 1.0}), ('fit/bell/small_bell', {'dur': 1.2}), ('pluck/kalimba_soft', {'dur': 1.0})]),
 'success': ('a pleasant rising success chime, two or three bright notes', [
   ('mallet/up4', {}), ('mallet/flourish', {}), ('notify/m1', {'dur': 1.2}), ('notify/m5', {'dur': 1.2}), ('shimmer/m2', {'dur': 1.4}),
   ('logo_sting/spark', {'dur': 1.4}), ('logo_sting/soft_bloom', {'dur': 1.4}), ('pro_zip/up', {}), ('pluck/kalimba', {'dur': 1.2})]),
 'error': ('a low dull error buzz thud, a negative failure sound', [
   ('notify/m3', {'pitch': 'A3', 'dur': 0.9}), ('mallet/marimba', {'pitch': 'D3'}), ('cut_hit/punch', {}), ('cut_hit/sub', {}), ('knock/double', {}),
   ('glitch/hit', {}), ('glitch/mid', {}), ('pro_zip/down', {}), ('pro_zip/down_warm', {}), ('downlifter/tape', {'dur': 1.2}),
   ('pro_sub_drop/quick', {}), ('toggle/off', {}), ('tap/knuckle', {}), ('pro_impact/short', {})]),
 'typing': ('a burst of fast typing on a mechanical keyboard', [
   ('pro_typing_run/standard', {}), ('pro_typing_run/soft', {}), ('pro_typing_run/tick', {}), ('pro_typing_run/glass', {}), ('pro_typing_run/blip', {}),
   ('typing_key/clicky', {}), ('typing_key/crisp', {}), ('typing_key/thock', {}), ('typing_key/laptop', {}), ('fit/key/medium', {}), ('fit/key/lubed', {}), ('fit/key/thock', {})]),
}

def fn(sid, kw):
    return sid.replace('/', '__') + ''.join(f'_{k}{v}' for k, v in kw.items()) + '.wav'

if __name__ == '__main__':
    meta = {}
    for name, (q, lst) in C.items():
        d = f'{CAND}/{name}'
        os.makedirs(d, exist_ok=True)
        for sid, kw in lst:
            try:
                y, m = bank.render(sid, **kw)
            except Exception as e:
                print('FAIL', sid, kw, e); continue
            f = f'{d}/{fn(sid, kw)}'
            sf.write(f, y, 48000, subtype='PCM_16')
            meta[f] = {'id': sid, 'kw': kw, 'dur': m['dur_s'], 'onset': m['onset_s'], 'peak': m['peak_s'], 'lufs': m.get('lufs_m_max'), 'checks': (m['checks'] or {}).get('summary')}
    json.dump(meta, open(ROOT + '/cand_meta.json', 'w'), indent=1)
    out = {}
    for name, (q, lst) in C.items():
        r = api.rank(f'{CAND}/{name}', q)
        rows = []
        for x in r['results']:
            f = x['path'].replace('\\', '/')
            m = meta.get(f) or meta.get(f.replace('//', '/')) or {}
            rows.append({'file': os.path.basename(f), 'z': x.get('z'), 'verdict': x.get('verdict'), 'meta': m})
        out[name] = rows
        print('==', name, q)
        for x in r['results']:
            print(' ', os.path.basename(x['path']), {k: x[k] for k in x if k not in ('path',)})
    json.dump(out, open(ROOT + '/rank.json', 'w'), indent=1, default=str)
