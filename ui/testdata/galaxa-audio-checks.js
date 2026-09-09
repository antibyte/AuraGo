async () => {
 const assert=(v,m)=>{if(!v)throw Error(m)},rate=48000;
 function context(settings={},seconds=1){
  const c=Object.create(GalaxaCore);c.state={disposed:false};c.settings={vol:100,musicVol:100,sfxVol:100,adaptiveMusic:true,...settings};c.G={muted:false,vol:c.settings.vol/100,biome:'nebula',demoMode:false};c.actx=new OfflineAudioContext(2,rate*seconds,rate);GalaxaCore.createAudio(c);c.audio();return c;
 }
 async function level(settings,destination){
  const c=context(settings);c.synthTone('sine',440,440,.3,.3,undefined,0,destination==='music'?c.musicBus:c.sfxBus);
  const b=await c.actx.startRendering(),peak=b.getChannelData(0).reduce((m,n)=>Math.max(m,Math.abs(n)),0);c.disposeAudio();return peak;
 }
 assert(await level({vol:0},'sfx')<1e-7,'master zero leaks');
 assert(await level({sfxVol:0},'sfx')<1e-7,'effect zero leaks');
 assert(await level({musicVol:0},'music')<1e-7,'music zero leaks');
 assert(await level({musicVol:0},'sfx')>.005,'music zero muted effects');
 assert(await level({sfxVol:0},'music')>.005,'effect zero muted music');
 const c=context({},12),a=c.actx;const cached=c.noiseBuffer;
 const samples=[
  ()=>c.SFX.shootTyped('normal',270),()=>c.SFX.shootTyped('ultra_rapid',270),
  ()=>c.SFX.shootTyped('laser',270),()=>c.SFX.rocketLaunch(270),
  ()=>c.SFX.parrySuccess(270),()=>c.SFX.superNovaBarrage(),
  ()=>c.SFX.bossPhaseTransition(),()=>c.SFX.bossDeathStinger(),
  ()=>{for(let i=0;i<40;i++)c.beep('sawtooth',110+i*11,60,.4,.4,270);c.SFX.attackWarning(270);assert(c.audioStats().effects<=32,'voice cap exceeded');}
 ];
 samples[0]();
 for(let i=1;i<samples.length;i++)a.suspend(i*1.2).then(()=>{samples[i]();assert(c.noiseBuffer===cached,'noise buffer reallocated');a.resume()});
 const buffer=await a.startRendering(),left=buffer.getChannelData(0),right=buffer.getChannelData(1);let peak=0,sum=0,clipped=0;
 for(const data of [left,right])for(const n of data){assert(Number.isFinite(n),'nonfinite audio');peak=Math.max(peak,Math.abs(n));sum+=n*n;if(Math.abs(n)>=.999)clipped++}
 assert(peak<.98&&clipped===0,'dense mix clipped: '+peak);assert(peak>.01,'silent effect mix');
 c.disposeAudio();assert(c.audioStats().effects===0&&c.audioStats().music===0,'audio disposal leaked voices');
 const bytes=new Uint8Array(44+left.length*4),v=new DataView(bytes.buffer);function str(at,text){for(let i=0;i<text.length;i++)bytes[at+i]=text.charCodeAt(i)}
 str(0,'RIFF');v.setUint32(4,bytes.length-8,true);str(8,'WAVE');str(12,'fmt ');v.setUint32(16,16,true);v.setUint16(20,1,true);v.setUint16(22,2,true);v.setUint32(24,rate,true);v.setUint32(28,rate*4,true);v.setUint16(32,4,true);v.setUint16(34,16,true);str(36,'data');v.setUint32(40,bytes.length-44,true);
 for(let i=0;i<left.length;i++){v.setInt16(44+i*4,Math.max(-32768,Math.min(32767,left[i]*32767)),true);v.setInt16(46+i*4,Math.max(-32768,Math.min(32767,right[i]*32767)),true)}
 let binary='';for(let i=0;i<bytes.length;i+=16384)binary+=String.fromCharCode(...bytes.subarray(i,i+16384));
 window.audioWav=btoa(binary);
 return {peak,rms:Math.sqrt(sum/(left.length*2)),clipped,voices:c.audioStats().peak,sampleRate:rate};
}
