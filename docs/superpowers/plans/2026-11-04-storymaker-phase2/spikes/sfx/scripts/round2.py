import sys, json, os, glob
import soundfile as sf
sys.path.insert(0, 'C:/Users/Andi/.local/share/earshot')
from earshot.forge import bank
from earshot import api

ROOT = 'C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/f2c97a7f-addd-4aa7-bd49-85d6c9b3f2a1/scratchpad/p2/spikes/sfx'
CAND = ROOT + '/cand'
extra = {'error': [('mallet/marimba', {'pitch': 150}), ('sub_drop/punch', {'dur': 0.6}), ('pro_pop_in/deep', {}), ('glitch/chirp', {}), ('glitch/data', {}),
                   ('pro_zip/down_warm', {'dur': 0.5}), ('pro_zip/down', {'dur': 0.5}), ('pro_tap/soft', {'pitch': 'B2'}), ('knock/plastic', {'pitch': 'D4'}), ('toggle/latch', {})]}
def fn(sid, kw):
    return sid.replace('/', '__') + ''.join(f'_{k}{v}' for k, v in kw.items()) + '.wav'
for name, lst in extra.items():
    for sid, kw in lst:
        try:
            y, m = bank.render(sid, **kw)
            sf.write(f'{CAND}/{name}/{fn(sid, kw)}', y, 48000, subtype='PCM_16')
        except Exception as e:
            print('FAIL', sid, kw, e)

Q = {'click': ['a single computer mouse click', 'a short plastic button click', 'a crisp light click'],
     'error': ['an error buzzer', 'a low negative error tone', 'a dull wrong-answer thud buzz', 'a computer error beep']}
for name, qs in Q.items():
    for q in qs:
        r = api.rank(f'{CAND}/{name}', q)
        print('==', name, q)
        for x in r['results'][:7]:
            print('  ', x['name'], x['z'], x['verdict'], x.get('sounds_most_like'))
