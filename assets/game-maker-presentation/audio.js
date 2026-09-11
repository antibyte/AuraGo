// MIT. One game-owned Web Audio graph; samples are local, never fetched from a provider.
export function createAudio({sounds=[], base, root, report=console.warn, dimension='3d'}) {
    const definitions=new Map(sounds.map(s=>[s.id,s])), buffers=new Map(), pending=new Map(), voices=new Set(), cooldown=new Map(),stops=new Map();
    const controller=new AbortController(); let ctx, master, compressor, wet, convolver, roomFilter, disposed=false, paused=false, muted=false, volume=.55;
    const buses={}, listener=[0,0,0], starting=new Set(), failures=new Set(); let room='outside', generation=0;
    function wire() {
        if(ctx||disposed)return;
        ctx=new AudioContext({sampleRate:48000});master=ctx.createGain();compressor=ctx.createDynamicsCompressor();
        compressor.threshold.value=-12;compressor.knee.value=12;compressor.ratio.value=8;compressor.attack.value=.003;compressor.release.value=.2;
        master.connect(compressor).connect(ctx.destination);
        for(const name of ['effects','ambience','music']) {buses[name]=ctx.createGain();buses[name].gain.value=name==='ambience'?.45:1;buses[name].connect(master)}
        convolver=ctx.createConvolver();wet=ctx.createGain();roomFilter=ctx.createBiquadFilter();roomFilter.type='lowpass';roomFilter.frequency.value=8000;
        convolver.connect(roomFilter).connect(wet).connect(master);setRoom(room);applyGain();
    }
    function applyGain(){if(master)master.gain.setTargetAtTime(muted||paused?0:volume*volume,ctx.currentTime,.03)}
    function setRoom(name){
        if(!['outside','small-room','hall'].includes(name))throw Error('audio: unknown room');room=name;if(!ctx)return;
        wet.gain.value=name==='outside'?0:name==='hall'?.2:.1;
        const n=Math.floor(ctx.sampleRate*(name==='hall'?1.7:.35)),b=ctx.createBuffer(2,n,ctx.sampleRate);let seed=1973;
        for(let c=0;c<2;c++){const x=b.getChannelData(c);for(let i=0;i<n;i++){seed=(Math.imul(seed,1664525)+1013904223)>>>0;x[i]=(seed/2147483648-1)*Math.pow(1-i/n,3)}}convolver.buffer=b;
    }
    async function load(id){
        if(buffers.has(id))return buffers.get(id);if(pending.has(id))return pending.get(id);
        const sound=definitions.get(id);if(!sound)throw Error('audio: unknown imported sound '+id);
        const task=(async()=>{
            const f=sound.files?.[0];if(!f||!f.file.startsWith('sounds/')||!/^sounds\/[a-z0-9-]+\.wav$/.test(f.file))throw Error('audio: invalid sample path');
            const parent=new URL(base,document.baseURI),url=new URL(f.file,parent);
            if(url.origin!==new URL(document.baseURI).origin||!url.pathname.startsWith(parent.pathname))throw Error('audio: sample outside game');
            const response=await fetch(url,{signal:controller.signal});if(!response.ok)throw Error('audio: '+id+' HTTP '+response.status);
            const bytes=await response.arrayBuffer();if(bytes.byteLength!==f.bytes)throw Error('audio: size mismatch '+id);
            if(crypto.subtle){const hash=[...new Uint8Array(await crypto.subtle.digest('SHA-256',bytes))].map(v=>v.toString(16).padStart(2,'0')).join('');if(hash!==f.sha256)throw Error('audio: checksum mismatch '+id)}
            const data=await ctx.decodeAudioData(bytes);if(disposed)return null;
            buffers.set(id,data);return data;
        })();pending.set(id,task);
        try{return await task}finally{pending.delete(id)}
    }
    function stop(v){if(!voices.has(v))return;voices.delete(v);try{v.source.stop()}catch{}v.source.disconnect();v.gain.disconnect();v.pan?.disconnect()}
    function fadeStop(v,seconds=.18){if(!voices.has(v)||v.fading)return;v.fading=true;v.gain.gain.setTargetAtTime(0,ctx.currentTime,seconds/3);try{v.source.stop(ctx.currentTime+seconds)}catch{}}
    function spatial2D(v){if(!v.position||!v.pan?.pan||v.fading)return;v.pan.pan.value=Math.max(-1,Math.min(1,(v.position[0]-listener[0])/450));v.gain.gain.setTargetAtTime(v.volume/(1+Math.hypot(v.position[0]-listener[0],v.position[1]-listener[1])/500),ctx.currentTime,.03)}
    function start(id,buffer,options={}){
        if(!buffer||disposed||paused||muted||ctx.state!=='running')return null;
        const def=definitions.get(id),loop=options.loop??def?.loop??false,bus=options.bus||(loop&&id!=='engine'?'ambience':'effects');
        if(!buses[bus])throw Error('audio: unknown bus');
        const existing=loop&&[...voices].find(v=>v.loop&&v.id===id&&!v.fading);
        if(existing){if(options.position){existing.position=options.position.slice();if(existing.pan?.positionX)[existing.pan.positionX.value,existing.pan.positionY.value,existing.pan.positionZ.value]=options.position;else spatial2D(existing)}return existing}
        if(bus==='ambience'&&[...voices].filter(v=>v.bus==='ambience').length>=4)return null;
        if(voices.size>=32){const oldest=[...voices].filter(v=>!v.loop).sort((a,b)=>a.priority-b.priority||a.start-b.start)[0];if(!oldest||oldest.priority>(options.priority??1))return null;stop(oldest)}
        const source=ctx.createBufferSource(),gain=ctx.createGain();source.buffer=buffer;source.loop=loop;
        source.playbackRate.value=options.rate??(loop?1:1+(Math.random()-.5)*.06);
        const volume=Math.min(1,Math.max(0,options.gain??def?.gain??.6));gain.gain.setValueAtTime(loop?0:volume,ctx.currentTime);if(loop)gain.gain.linearRampToValueAtTime(volume,ctx.currentTime+.3);
        let pan;source.connect(gain);
        if(options.position){
            if(dimension==='3d'){pan=ctx.createPanner();pan.panningModel='HRTF';pan.distanceModel='inverse';pan.refDistance=2;pan.maxDistance=80;pan.rolloffFactor=1.3;[pan.positionX.value,pan.positionY.value,pan.positionZ.value]=options.position}
            else pan=ctx.createStereoPanner();
            gain.connect(pan).connect(buses[bus]);
        }else gain.connect(buses[bus]);
        if(bus==='effects')(pan||gain).connect(convolver);
        const v={id,source,gain,pan,loop,bus,volume,position:options.position?.slice(),priority:options.priority??1,start:ctx.currentTime};if(dimension==='2d'&&pan){gain.gain.cancelScheduledValues(ctx.currentTime);spatial2D(v)}voices.add(v);source.onended=()=>stop(v);source.start();return v;
    }
    async function play(id,options={}){
        if(!definitions.has(id)){report('audio: unknown imported sound '+id);return null}
        // Do not queue pre-gesture or inactive one-shots; stale shots must never replay.
        if(!ctx||disposed||paused||muted||ctx.state!=='running')return null;
        if(options.position&&(!Array.isArray(options.position)||options.position.length!==3||options.position.some(v=>!Number.isFinite(v))))throw Error('audio: invalid position');
        for(const key of ['rate','gain','cooldown','priority'])if(options[key]!==undefined&&!Number.isFinite(options[key]))throw Error('audio: invalid '+key);
        const epoch=generation,stopEpoch=stops.get(id),now=ctx.currentTime;if(now-(cooldown.get(id)??-99)<(options.cooldown??.045))return null;cooldown.set(id,now);
        const loop=options.loop??definitions.get(id).loop;
        try{const buffer=await load(id);if(epoch!==generation||stopEpoch!==stops.get(id)||!loop&&ctx.currentTime-now>.2||options.ambient&&!requestedAmbience.includes(id))return null;return start(id,buffer,options)}catch(e){if(!disposed&&e.name!=='AbortError'&&!failures.has(id)){failures.add(id);report(String(e))}return null}
    }
    let requestedAmbience=[];
    async function ambience(ids){
        requestedAmbience=[...new Set(ids)].slice(0,4);const wanted=new Set(requestedAmbience);
        for(const v of voices)if(v.bus==='ambience'&&!wanted.has(v.id))fadeStop(v);
        for(const id of requestedAmbience)if(!starting.has(id)&&![...voices].some(v=>v.loop&&v.id===id&&!v.fading)){starting.add(id);play(id,{loop:true,ambient:true,bus:'ambience'}).finally(()=>starting.delete(id))}
    }
    async function unlock(event){
        if(!event.isTrusted||disposed||paused)return;
        wire();try{await ctx.resume();await Promise.all([...definitions.keys()].map(id=>load(id).catch(e=>{if(!failures.has(id)&&e.name!=='AbortError'){failures.add(id);report('audio: '+e.message)}})));if(!disposed)await ambience(requestedAmbience)}catch(e){if(!disposed)report('audio: '+e.message)}
    }
    root.addEventListener('pointerdown',unlock,{signal:controller.signal});root.addEventListener('keydown',unlock,{signal:controller.signal});
    return {
        play,ambience,setRoom,unlock,stop(id){if(!definitions.has(id))return;stops.set(id,(stops.get(id)||0)+1);for(const v of [...voices])if(v.id===id)stop(v)},
        setVolume(v){volume=Math.max(0,Math.min(1,Number.isFinite(v)?v:.55));applyGain()},
        setMuted(v){muted=!!v;generation++;if(muted)for(const x of [...voices])stop(x);applyGain();if(!muted)ambience(requestedAmbience)},
        setPaused(v){if(paused===!!v)return;paused=!!v;generation++;if(paused){for(const x of [...voices])stop(x);ctx?.suspend().catch(()=>{})}else if(ctx){ctx.resume().then(()=>ambience(requestedAmbience)).catch(()=>{})}applyGain()},
        listener(position,forward=[0,0,-1],up=[0,1,0]){listener.splice(0,3,...position);if(!ctx)return;if(dimension==='2d')for(const v of voices)spatial2D(v);const l=ctx.listener;for(const [key,vec] of [['position',position],['forward',forward],['up',up]])for(let i=0;i<3;i++)if(l[key+'XYZ'[i]])l[key+'XYZ'[i]].value=vec[i]},
        connectMusic(node){if(!ctx)throw Error('audio: unlock with player interaction first');if(node.context!==ctx)throw Error('audio: music must use the game AudioContext');node.connect(buses.music);return()=>node.disconnect(buses.music)},
        get context(){return ctx},
        reset(){generation++;for(const v of [...voices])stop(v);cooldown.clear();ambience(requestedAmbience)},
        stats(){return {voices:voices.size,loops:[...voices].filter(v=>v.loop).length,buffers:buffers.size,state:ctx?.state||'locked'}},
        dispose(){if(disposed)return;disposed=true;controller.abort();for(const v of [...voices])stop(v);buffers.clear();pending.clear();convolver?.disconnect();roomFilter?.disconnect();wet?.disconnect();Object.values(buses).forEach(n=>n.disconnect());master?.disconnect();compressor?.disconnect();ctx?.close().catch(()=>{})},
    };
}
