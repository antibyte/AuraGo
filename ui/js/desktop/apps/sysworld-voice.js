// Transient tower speech: the active TTS backend feeds a quiet spatial effects bus.
export function createTowerVoice(context, output) {
  let active=false, disposed=false, timer=0, request=null, source=null, nodes=[], gain=null, filter=null, panner=null;
  let distance=250, pan=0, generation=0, phrases=0, state='idle', ir=null;
  const level=()=>.03+.36/(1+(distance/48)**2);
  function spatial() {
    const now=context.currentTime;
    gain?.gain.setTargetAtTime(level(),now,.18);
    filter?.frequency.setTargetAtTime(1800+4400/(1+distance/95),now,.18);
    panner?.pan.setTargetAtTime(pan,now,.18);
  }
  function clearAudio() {
    if(source){source.onended=null;try{source.stop();}catch(_){}source=null;}
    nodes.forEach(n=>n.disconnect());nodes=[];gain=filter=panner=null;
  }
  function stop() {
    generation++;clearTimeout(timer);timer=0;
    request?.abort();request=null;clearAudio();state='idle';
  }
  function schedule(delay=20000+Math.random()*20000) {
    clearTimeout(timer);
    if(active&&!disposed)timer=setTimeout(()=>{timer=0;void speak();},delay);
  }
  function impulse() {
    if(ir)return ir;
    ir=context.createBuffer(2,Math.ceil(context.sampleRate*.85),context.sampleRate);
    // Procedural stereo room impulse: fixed length, no audio assets or feedback loop.
    let seed=42;
    for(let c=0;c<2;c++){const data=ir.getChannelData(c);
      for(let i=0;i<data.length;i++){seed=(Math.imul(seed,1664525)+1013904223)>>>0;data[i]=(seed/4294967296*2-1)*Math.pow(1-i/data.length,3);}
    }
    return ir;
  }
  async function speak() {
    if(!active||disposed||request||source||context.state!=='running'){schedule();return;}
    const token=generation, controller=new AbortController();request=controller;state='loading';
    const timeout=setTimeout(()=>controller.abort(),32000);
    try {
      const response=await fetch('/api/desktop/system-world/voice',{method:'POST',credentials:'same-origin',cache:'no-store',signal:controller.signal});
      if(response.status===204){state='idle';return;}
      if(!response.ok||!response.headers.get('Content-Type')?.startsWith('audio/'))throw Error('Voice unavailable');
      if(Number(response.headers.get('Content-Length'))>8*1024*1024)throw Error('Voice too large');
      const bytes=await response.arrayBuffer();if(bytes.byteLength>8*1024*1024)throw Error('Voice too large');
      if(token!==generation||!active||disposed)return;
      const buffer=await context.decodeAudioData(bytes);
      if(token!==generation||!active||disposed)return;
      if(!Number.isFinite(buffer.duration)||buffer.duration<=0||buffer.duration>25||buffer.numberOfChannels>2)throw Error('Invalid voice');
      // Normalize peaks once, then cap the complete dry/echo/reverb mix before distance gain.
      let peak=0;
      for(let c=0;c<buffer.numberOfChannels;c++){const data=buffer.getChannelData(c);for(const x of data)peak=Math.max(peak,Math.abs(x));}
      if(!Number.isFinite(peak)||peak<.0001)throw Error('Silent voice');
      const own=n=>(nodes.push(n),n);
      source=own(context.createBufferSource());source.buffer=buffer;
      const input=own(context.createGain()), highpass=own(context.createBiquadFilter());
      input.gain.value=1/peak;highpass.type='highpass';highpass.frequency.value=170;
      filter=own(context.createBiquadFilter());filter.type='lowpass';filter.Q.value=.55;
      const dry=own(context.createGain()), echo=own(context.createGain()), room=own(context.createGain()), mix=own(context.createGain());
      dry.gain.value=.74;echo.gain.value=.17;room.gain.value=.12;
      const delay=own(context.createDelay(.2)), reverb=own(context.createConvolver());
      delay.delayTime.value=.085;reverb.buffer=impulse();
      const limit=own(context.createWaveShaper()), curve=new Float32Array(1024);
      for(let i=0;i<curve.length;i++)curve[i]=Math.max(-.75,Math.min(.75,i*2/(curve.length-1)-1));
      limit.curve=curve;
      gain=own(context.createGain());gain.gain.value=level();
      panner=own(context.createStereoPanner());panner.pan.value=pan;
      source.connect(input).connect(highpass).connect(filter);
      filter.connect(dry).connect(mix);filter.connect(delay).connect(echo).connect(mix);filter.connect(reverb).connect(room).connect(mix);
      mix.connect(limit).connect(gain).connect(panner).connect(output);
      spatial();state='speaking';phrases++;
      // The short room tail finishes before the quiet interval starts.
      source.onended=()=>{if(token!==generation||!active||disposed)return;state='tail';timer=setTimeout(()=>{clearAudio();state='idle';schedule();},900);};
      source.start();
    } catch(_) {if(token===generation&&!disposed)state=controller.signal.aborted?'idle':'unavailable';}
    finally {
      clearTimeout(timeout);
      if(request===controller)request=null;
      if(token===generation&&!disposed&&!source)schedule(state==='unavailable'?60000:undefined);
    }
  }
  return {
    setActive(value){if(active===value||disposed)return;active=value;if(active)schedule(3000+Math.random()*2000);else stop();},
    setListener(x,y,z,forwardX,forwardZ) {
      if(!Number.isFinite(x)||!Number.isFinite(y)||!Number.isFinite(z)||!Number.isFinite(forwardX)||!Number.isFinite(forwardZ))return;
      // Distance to the tower's volume keeps it audible from street level and at its crown.
      const dx=-x,dz=-12-z, horizontal=Math.max(0,Math.hypot(dx,dz)-13);
      distance=Math.hypot(horizontal,Math.max(0,y-87,-y));
      const length=Math.hypot(dx,dz),forward=Math.hypot(forwardX,forwardZ);
      pan=length&&forward?Math.max(-.8,Math.min(.8,(dz*forwardX-dx*forwardZ)/length/forward))*.8:0;
      spatial();
    },
    stats(){return {state,phrases,distance,gain:level(),pan,pending:!!request,scheduled:!!timer};},
    dispose(){if(disposed)return;disposed=true;active=false;stop();ir=null;},
  };
}
