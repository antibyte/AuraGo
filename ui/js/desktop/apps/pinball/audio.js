import { ASSET_ROOT, clamp } from './tables.js';

const SAMPLES = { flipper: 'flipper', bumper: 'bumper', sling: 'sling', target: 'target', launch: 'launch', drain: 'drain', lock: 'lock', jackpot: 'jackpot', bonus: 'bonus' };

export function createAudio(table, settings, assetURL) {
    let context = null, effects = null, ambience = null, roll = null, rollingGain = null, disposed = false, paused = true, loading = null, voices = new Set(), config = settings;
    const buffers = new Map(), continuous = [], controller = new AbortController();
    function setSettings(value) {
        config = value;
        if (!context) return;
        effects.gain.setTargetAtTime(value.effects,context.currentTime,.03);
        ambience.gain.setTargetAtTime(value.ambience,context.currentTime,.05);
    }
    function continuousSound() {
        const buffer = context.createBuffer(1, context.sampleRate * 4, context.sampleRate), data = buffer.getChannelData(0);
        let seed=271828;
        for (let i=0;i<data.length;i++) { seed=(Math.imul(seed,1664525)+1013904223)>>>0; const t=i/data.length; data[i]=((seed/4294967296)*2-1)*Math.sin(Math.PI*Math.min(1,t*40,(1-t)*40))**2; }
        const source=context.createBufferSource(); source.buffer=buffer;source.loop=true;
        const filter=context.createBiquadFilter();filter.type='lowpass';filter.frequency.value=table==='leviathan'?350:table==='temple'?1800:650;
        const gain=context.createGain();gain.gain.value=.035;source.connect(filter).connect(gain).connect(ambience);source.start();continuous.push(source);
        const root=table==='neon'?73.416:table==='leviathan'?43.65:65.406;
        for (const [ratio,volume] of [[1,.035],[1.5,.012],[2.25,.006]]) { const tone=context.createOscillator(),level=context.createGain();tone.type='sine';tone.frequency.value=root*ratio;level.gain.value=volume;tone.connect(level).connect(ambience);tone.start();continuous.push(tone); }
        roll=context.createBufferSource();roll.buffer=buffer;roll.loop=true;
        const rollingFilter=context.createBiquadFilter();rollingFilter.type='bandpass';rollingFilter.frequency.value=600;rollingFilter.Q.value=.7;
        rollingGain=context.createGain();rollingGain.gain.value=0;roll.connect(rollingFilter).connect(rollingGain).connect(effects);roll.start();continuous.push(roll);
    }
    async function unlock() {
        if (disposed) return false;
        if (!context) {
            const Audio = window.AudioContext || window.webkitAudioContext;
            if (!Audio) return false;
            context=new Audio({latencyHint:'interactive'});effects=context.createGain();ambience=context.createGain();
            const limiter=context.createDynamicsCompressor();limiter.threshold.value=-8;limiter.knee.value=6;limiter.ratio.value=12;limiter.attack.value=.003;limiter.release.value=.15;
            effects.connect(limiter);ambience.connect(limiter);limiter.connect(context.destination);setSettings(config);continuousSound();
            loading=Promise.all([...new Set(Object.values(SAMPLES))].map(async name=>{
                const response=await fetch(assetURL(ASSET_ROOT+'audio/'+name+'.ogg'),{signal:controller.signal});if(!response.ok)throw Error('audio');
                const bytes=await response.arrayBuffer();const buffer=await context.decodeAudioData(bytes);
                if(!disposed)buffers.set(name,buffer);
            })).catch(()=>false);
        }
        if (!paused) await context.resume().catch(()=>{});
        return true;
    }
    function effect(type, position=0, strength=1) {
        if(disposed||paused||!context||context.state!=='running'||config.effects===0||voices.size>=20)return;
        const buffer=buffers.get(SAMPLES[type]);if(!buffer)return;
        const source=context.createBufferSource(),gain=context.createGain(),pan=context.createStereoPanner();source.buffer=buffer;
        gain.gain.value=clamp(strength,0,1)*.55;pan.pan.value=clamp(position/.3,-1,1)*.75;
        source.connect(gain).connect(pan).connect(effects);voices.add(source);source.onended=()=>{voices.delete(source);source.disconnect();gain.disconnect();pan.disconnect();};source.start();
    }
    function pause() { paused=true;for(const v of voices){try{v.stop();}catch(_){}}voices.clear();if(context&&context.state==='running')void context.suspend().catch(()=>{}); }
    return { unlock,effect,setSettings,pause,
        resume() { paused=false;if(context&&context.state==='suspended')void context.resume().catch(()=>{}); },
        rolling(speed) { if(rollingGain&&context)rollingGain.gain.setTargetAtTime(paused?0:Math.min(.035,speed*.01),context.currentTime,.05); },
        inspect:()=>({unlocked:!!context,state:context?.state||'locked',voices:voices.size,buffers:buffers.size,paused}),
        async ready() { return loading ? await loading : false; },
        dispose() { if(disposed)return;disposed=true;controller.abort();pause();for(const node of continuous){try{node.stop();}catch(_){}node.disconnect();}continuous.length=0;buffers.clear();if(context)void context.close().catch(()=>{}); }
    };
}
