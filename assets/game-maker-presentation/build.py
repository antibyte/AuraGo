"""Reproducible WAV mastering and catalog build. Needs numpy, ffmpeg; no runtime dependency."""
import argparse
import hashlib
import io
import json
from pathlib import Path
import subprocess
import wave
import zipfile
import numpy as np
from catalog import VERSION, SOUND_GROUPS, LOOPS, effect_assets

ROOT = Path(__file__).resolve().parent
PACKS = ROOT.parents[1] / 'internal/gamemaker/asset_packs'
RATE = 48000
MIT = (ROOT.parents[1] / 'LICENSE').read_text(encoding='utf-8')

def decode(data, seconds=18):
    p = subprocess.run(['ffmpeg','-v','error','-i','pipe:0','-t',str(seconds),'-f','f32le','-ar',str(RATE),'-ac','2','pipe:1'],input=data,stdout=subprocess.PIPE,stderr=subprocess.PIPE,check=True)
    return np.frombuffer(p.stdout,dtype='<f4').reshape(-1,2).copy()

def noise(rng, n, cutoff=1000):
    spectrum = np.fft.rfft(rng.normal(0,1,n))
    frequencies = np.fft.rfftfreq(n,1/RATE)
    spectrum /= np.sqrt(1+(frequencies/cutoff)**4)
    out = np.fft.irfft(spectrum,n)
    return out / max(0.001,np.std(out))

def synth(id, loop):
    rng = np.random.default_rng(int.from_bytes(hashlib.sha256(id.encode()).digest()[:8],'little'))
    seconds = 12 if loop else 4.5 if id in ['thunder','explosion-large'] else 2 if id in ['victory','defeat','reload'] else 1.2
    t = np.arange(int(RATE*seconds))/RATE
    n=len(t); low=noise(rng,n,250); high=noise(rng,n,6500)
    if id.startswith('wind'):
        x=noise(rng,n,900)*(0.3+0.2*np.sin(t*1.3)+0.15*np.sin(t*.7))
    elif id in ['stream','ocean']:
        x=(noise(rng,n,2400)*.3+high*.05)*(0.7+0.3*np.sin(t*(1.2 if id=='stream' else .6)))
        for start in rng.uniform(0,seconds,45):
            u=t-start; x+=np.where(u>=0,np.sin(2*np.pi*(850*u+180*u*u))*np.exp(-np.maximum(0,u)*28)*.08,0)
    elif id in ['industrial','space-hum','engine']:
        f={'industrial':50,'space-hum':38,'engine':65}[id]
        x=sum(np.sin(2*np.pi*f*k*t+np.sin(t*.6)*.15)/k for k in range(1,9))*.12+low*.12
        x*=.8+.15*np.sin(t*2*np.pi/seconds)
    elif id=='fire':
        x=low*.12
        for start in rng.uniform(0,seconds,85): x+=high*np.exp(-np.maximum(0,t-start)*150)*(t>=start)*rng.uniform(.05,.4)
    elif id in ['pistol','rifle','shotgun','explosion-small','explosion-large','thunder']:
        decay=1.4 if id in ['explosion-large','thunder'] else 3 if id=='explosion-small' else 10 if id=='shotgun' else 17
        x=(low*.7+high*np.exp(-t*45)*1.2+np.sin(2*np.pi*(90*t-12*t*t))*.25)*np.exp(-t*decay)
        if id=='thunder': x*=1-np.exp(-t*8)
        for delay,gain in [(.07,.28),(.19,.14),(.37,.07)]:
            k=int(delay*RATE); x[k:]+=x[:-k].copy()*gain
    elif id in ['laser','electric-spark']:
        x=(np.sin(2*np.pi*(1900*t-650*t*t))+np.sin(2*np.pi*3700*t)*.2+high*.15)*np.exp(-t*9)
    elif id in ['victory','defeat','pickup','ui-click']:
        notes={'victory':[523,659,784,1046],'defeat':[392,330,261,196],'pickup':[880,1320],'ui-click':[1200]}[id]
        x=np.zeros(n)
        for i,f in enumerate(notes):
            u=t-i*.14; x+=(np.sin(2*np.pi*f*u)+.15*np.sin(2*np.pi*f*2*u))*np.exp(-np.maximum(0,u)*12)*(u>=0)*.4
    elif id=='reload':
        x=np.zeros(n)
        for start in [.02,.28,.65,1.1,1.35]:
            u=t-start; x+=(high*.25+np.sin(2*np.pi*920*u)*.1)*np.exp(-np.maximum(0,u)*55)*(u>=0)
    else:
        cutoff=350 if id in ['land','impact-flesh','step-grass'] else 2200
        x=(noise(rng,n,cutoff)*.6+np.sin(2*np.pi*160*t)*.2)*np.exp(-t*(9 if id=='water-splash' else 24))
        if id in ['door-metal','impact-metal','step-metal']: x+=sum(np.sin(2*np.pi*f*t)*np.exp(-t*d)*.08 for f,d in [(733,5),(1287,7),(2570,11)])
        if id=='jump': x+=np.sin(2*np.pi*(210*t+250*t*t))*np.exp(-t*9)*.2
    return np.column_stack([x,np.roll(x,113) if loop else x]).astype(np.float32)

def master(x, loop, mono):
    x=x.astype(np.float64); x-=x.mean(axis=0)
    if loop:
        length=min(len(x),12*RATE); x=x[:length]
        k=min(RATE//2,len(x)//8); ramp=np.linspace(0,1,k)[:,None]
        blend=x[-k:]*(1-ramp)+x[:k]*ramp
        x=np.concatenate([blend,x[k:-k]])
    else:
        k=min(240,len(x)//4); x[:k]*=np.linspace(0,1,k)[:,None]; k=min(RATE//20,len(x)//4);x[-k:]*=np.linspace(1,0,k)[:,None]
    if mono: x=x.mean(axis=1,keepdims=True)
    rms=np.sqrt(np.mean(x*x)); gain=min((.1 if loop else .16)/max(rms,1e-9),.707/max(np.max(np.abs(x)),1e-9))
    x*=gain
    pcm=np.round(np.clip(x,-1,1)*32767).astype('<i2')
    out=io.BytesIO()
    with wave.open(out,'wb') as w:
        w.setnchannels(pcm.shape[1]);w.setsampwidth(2);w.setframerate(RATE);w.writeframes(pcm.tobytes())
    return out.getvalue(),len(x)/RATE

def main(check=False):
    sources=json.loads((ROOT/'sources/sources.json').read_text())
    for name,meta in sources.items():
        path=next((ROOT/'sources').glob(name+'.*'))
        assert hashlib.sha256(path.read_bytes()).hexdigest()==meta['sha256'],name
    archives={k:zipfile.ZipFile(ROOT/'sources'/f'{k}.zip') for k in ['impact','rpg']}
    print('Source samples:', {k:z.namelist()[:4] for k,z in archives.items()})
    recordings={k:decode((ROOT/'sources'/f'{k}.{ext}').read_bytes()) for k,ext in [('forest','ogg'),('crickets','mp3'),('rain','ogg')]}
    # Exact source members become a reviewed lock after the first production run.
    samples={
      'step-grass':('impact','footstep_grass'), 'step-gravel':('rpg','footstep03'),
      'step-stone':('impact','footstep_concrete'), 'step-wood':('impact','footstep_wood'),
      'impact-stone':('impact','impactMining'), 'crate-break':('impact','impactWood_heavy'),
      'door-metal':('impact','impactMetal_heavy'), 'door-wood':('impact','impactWood_medium'),
      'impact-flesh':('impact','impactPunch_heavy'), 'impact-metal':('impact','impactPlate_medium'),
    }
    assets=[]; outputs={}
    for category,ids in SOUND_GROUPS.items():
        for id in ids:
            loop=id in LOOPS; origin={'license':'MIT','author':'AuraGo','method':'Deterministic layered noise, modal resonances and transients'}
            key={'forest-day':'forest','forest-night':'crickets','rain-light':'rain','rain-heavy':'rain'}.get(id)
            if key:
                x=recordings[key].copy();origin={**sources[key],'source_id':key}
                if id=='rain-light': x=np.column_stack([np.convolve(x[:,c],np.ones(9)/9,mode='same') for c in range(2)])
            elif id in samples:
                pack,prefix=samples[id]; names=sorted(n for n in archives[pack].namelist() if prefix.lower() in n.lower() and n.endswith(('.ogg','.wav')))
                if not names: raise ValueError('Source sample not found: '+id+' / '+prefix)
                member=names[0]; raw=archives[pack].read(member); x=decode(raw,3)
                origin={**sources[pack],'source_id':pack,'member':member,'member_sha256':hashlib.sha256(raw).hexdigest()}
            else: x=synth(id,loop)
            data,duration=master(x,loop,not loop)
            file=dict(file='sounds/'+id+'.wav',bytes=len(data),sha256=hashlib.sha256(data).hexdigest())
            outputs[PACKS/'aurago-sounds'/file['file']]=data
            aliases=['footstep','footsteps','walking','movement'] if id.startswith('step-') else ['gun','shoot','shot','weapon'] if id in ['pistol','rifle','shotgun','laser'] else ['ambient','environment','loop'] if loop else []
            origin['processing']='Decode to 48 kHz, remove DC, equalize RMS with -3 dB peak ceiling, loop crossfade or click-free transient fades, PCM16 export'
            assets.append(dict(id=id,name=id.replace('-',' ').title(),category=category,description=('Seamless ambience' if loop else 'Spatial sound cue')+': '+id.replace('-',' '),tags=id.split('-')+[category]+aliases,dimensions=['2d','3d'],loop=loop,duration=duration,gain=.6 if loop else .8,files=[file],provenance=origin))
    assert len(assets)==40
    for pack,kind,items in [('aurago-sounds','audio',assets),('aurago-effects','effect',effect_assets())]:
        meta=dict(id=pack,version=VERSION,kind=kind,schema_version=1,name='AuraGo Sounds' if kind=='audio' else 'AuraGo Effects',description='Offline curated game '+kind+' library',tags=[kind,'presentation'],assets=items)
        outputs[PACKS/pack/'manifest.json']=(json.dumps(meta,indent=2)+'\n').encode()
        license_text=MIT+'\nOwn code and synthesized sounds: MIT. Third-party samples: CC0-1.0. See each imported asset metadata record for source, author, processing and checksums.\n'
        if kind=='audio':license_text+='\n'+(ROOT/'CC0-1.0.txt').read_text(encoding='utf-8')
        outputs[PACKS/pack/'LICENSE.txt']=license_text.encode()
    catalog=json.loads((PACKS/'catalog.json').read_text())
    catalog=[p for p in catalog if p['id'] not in ['aurago-sounds','aurago-effects']]
    for id,kind in [('aurago-effects','effect'),('aurago-sounds','audio')]: catalog.append(dict(id=id,kind=kind,version=VERSION,name=id.replace('-',' ').title(),description='Curated offline '+kind+' presets',tags=[kind,'presentation']))
    outputs[PACKS/'catalog.json']=(json.dumps(catalog,indent=2,ensure_ascii=False)+'\n').encode()
    for path,data in outputs.items():
        if check: assert path.read_bytes()==data,'Rebuild '+str(path)
        else: path.parent.mkdir(parents=True,exist_ok=True);path.write_bytes(data)
    size=sum(len(data) for path,data in outputs.items() if path.name!='catalog.json')
    assert size < 48*1024*1024, size
    print(f'40 sounds, {len(effect_assets())} effect/atmosphere presets; {size/1024/1024:.2f} MiB')

if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');main(p.parse_args().check)
