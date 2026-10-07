package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDesktopSynthStudioAudioBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	mux := http.NewServeMux()
	mux.Handle("/js/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><head><meta charset="utf-8"></head><body>
<button id="audio-gesture">Enable audio</button>
<script src="/js/vendor/synth-studio/gm-data.js"></script>
<script src="/js/desktop/apps/synth-studio-presets.js"></script>
<script src="/js/desktop/apps/synth-studio-voices.js"></script>
<script src="/js/desktop/apps/synth-studio-audio.js"></script>
</body></html>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	page := newSmokeBrowser(t).MustPage().Timeout(180 * time.Second)
	defer page.Close()
	page.MustNavigate(srv.URL + "/fixture").MustWaitLoad()
	if !page.MustEval(`async()=>{
 const original=AudioContext.prototype.resume;let finish;
 AudioContext.prototype.resume=function(){return new Promise(resolve=>{finish=resolve})};
 const player=SynthStudioAudio.create();let ticks=0;
 const project={version:1,tempo:120,ppq:480,tracks:[{id:'race',instrument:'lead-saw',clips:[{start:0,length:120,notes:[{pitch:60,start:0,duration:120,velocity:0.7}],controllers:[]}]}]};
 const pending=player.play(project,0,()=>ticks++);player.stop();finish();await pending;
 AudioContext.prototype.resume=original;
 player.dispose();return ticks===0;
}`).Bool() {
		t.Fatal("stop before AudioContext unlock completion allowed a stale playback start")
	}
	page.MustEval(`()=>{window.audioTestPlayer=SynthStudioAudio.create();document.querySelector('#audio-gesture').onclick=()=>{window.audioReady=audioTestPlayer.unlock()}}`)
	page.MustElement("#audio-gesture").MustClick()
	result := page.MustEval(`async()=>{
 await window.audioReady;
 const check=(condition,message)=>{if(!condition)throw Error(message)};
 const catalog=window.SynthStudioPresets;
 check(catalog.categories.length===5,'five preset categories');
 check(catalog.presets.length===50,'50 curated presets');
 check(['drums','bass','leads','pads','keys'].every(category=>catalog.presets.filter(p=>p.category===category).length===10),'10 presets in each category');
 check(catalog.gm.length===128&&catalog.gm[0].id==='gm-0'&&catalog.gm[127].id==='gm-127','128 GM melodic patches');
 check(catalog.drums.length===10&&catalog.get('drum-shaker').drumNote===70,'ten standard-note kit sounds');
 check(window.SynthStudioGMData.drums.length===47,'GM drum map spans notes 35 through 81');

 const makeProject=(instrument,notes)=>({version:1,name:'Audio test',tempo:120,ppq:480,beatsPerBar:4,loop:{enabled:false,start:0,end:0},tracks:[{id:'track',name:'Track',instrument,volume:0.8,pan:0,mute:false,solo:false,params:{tone:0.5,attack:0.01,release:0.12,reverb:0,delay:0},clips:[{id:'clip',name:'Clip',start:0,length:notes.length?notes.at(-1).start+notes.at(-1).duration:0,notes,controllers:[]}]}]});
 const render=(instrument,notes)=>window.SynthStudioAudio.renderWav(makeProject(instrument,notes));
 const readPCM=async blob=>{
   const bytes=await blob.arrayBuffer(),view=new DataView(bytes);
   check(view.getUint32(0,false)===0x52494646&&view.getUint32(8,false)===0x57415645,'valid RIFF/WAVE header');
   check(view.getUint16(22,true)===2&&view.getUint32(24,true)===44100&&view.getUint16(34,true)===16,'stereo 44.1 kHz PCM16');
   check(view.getUint32(40,true)===bytes.byteLength-44,'valid PCM data length');
   const samples=new Int16Array(bytes,44),rms=(start,end)=>{
     let sum=0,count=0;
     for(let frame=Math.max(0,Math.floor(start*44100));frame<Math.min(view.getUint32(40,true)/4,Math.ceil(end*44100));frame++){
       const l=samples[frame*2]/32768,r=samples[frame*2+1]/32768;sum+=l*l+r*r;count+=2;
     }
     return count?Math.sqrt(sum/count):0;
   };
   for(const sample of samples)check(Number.isFinite(sample),'finite PCM sample');
   return {bytes:bytes.byteLength,rms,samples};
 };
 const held=await readPCM(await render('lead-saw',[{id:'held',pitch:60,start:0,duration:480,velocity:0.9}]));
 check(held.rms(0.2,0.42)>0.01,'held note remains audible before note-off');
 check(held.rms(1.2,1.4)<0.001,'released note fades to silence');

 const sustained=makeProject('lead-saw',[{id:'sustain',pitch:60,start:0,duration:48,velocity:0.9}]);
 sustained.tracks[0].clips[0].length=240;
 sustained.tracks[0].clips[0].controllers=[{tick:0,type:'sustain',value:1}];
 const sustainedProject=makeProject('lead-saw',sustained.tracks[0].clips[0].notes);
 sustainedProject.tracks[0].clips[0].length=240;
 sustainedProject.tracks[0].clips[0].controllers=[{tick:0,type:'sustain',value:1}];
 const naturalProjectWav=await readPCM(await window.SynthStudioAudio.renderWav(sustainedProject));
 check(naturalProjectWav.rms(0.62,0.8)<0.001,'natural arrangement end releases a sustain-held voice');
 const heldBySustain=makeProject('lead-saw',Array.from({length:257},(_,i)=>({id:'s'+i,pitch:60,start:i*2,duration:1,velocity:0.5})));
 heldBySustain.tracks[0].clips[0].controllers=[{tick:0,type:'sustain',value:1},{tick:800,type:'sustain',value:0}];
 let sustainCode='';try{await window.SynthStudioAudio.renderWav(heldBySustain)}catch(error){sustainCode=error.code}
 check(sustainCode==='SYNTH_STUDIO_RENDER_BUDGET','sustain-held notes count toward the overlap safety budget');

	const sparse=Array.from({length:160},(_,i)=>({id:'n'+i,pitch:48+(i%25),start:i*96,duration:48,velocity:0.75}));
 const long=await readPCM(await render('lead-saw',sparse));
	check(long.bytes>3_000_000,'long arrangement rendered a substantial WAV');
	for(const i of [8,70,150]){
   const start=i*96/960+0.012;
   check(long.rms(start,start+0.035)>0.003,'long arrangement keeps late notes audible at index '+i);
 }

 const one=[{id:'one',pitch:69,start:0,duration:180,velocity:0.8}];
 const saw=await readPCM(await render('lead-saw',one));
 const bell=await readPCM(await render('key-bell',one));
 let difference=0;
 for(let i=0;i<Math.min(saw.samples.length,bell.samples.length);i++)difference+=Math.abs(saw.samples[i]-bell.samples[i]);
 check(difference>100000,'different presets render distinct timbres');

 const overlap=makeProject('lead-saw',Array.from({length:257},(_,i)=>({id:'o'+i,pitch:60,start:0,duration:480,velocity:0.6})));
 let overlapCode='';try{await window.SynthStudioAudio.renderWav(overlap)}catch(error){overlapCode=error.code}
 check(overlapCode==='SYNTH_STUDIO_RENDER_BUDGET','overlap safety budget is enforced');
 const tooLong=makeProject('lead-saw',[{id:'long',pitch:60,start:0,duration:288000,velocity:0.7}]);
 let exportCode='';try{await window.SynthStudioAudio.renderWav(tooLong)}catch(error){exportCode=error.code}
 check(exportCode==='export_limit','five-minute export ceiling includes the effect tail');

	const player=window.audioTestPlayer;
 const loopProject=makeProject('lead-saw',[{id:'loop',pitch:60,start:0,duration:120,velocity:0.5}]);
 loopProject.tempo=300;loopProject.loop={enabled:true,start:0,end:120};
 loopProject.tracks[0].clips[0].length=120;
 loopProject.tracks[0].clips[0].controllers=[{tick:60,type:'sustain',value:1},{tick:60,type:'bend',value:0.5},{tick:60,type:'modulation',value:0.8}];
 let previousTick=-1,wraps=0;
 const loopDone=player.play(loopProject,0,tick=>{if(previousTick>=0&&tick<previousTick-10)wraps++;previousTick=tick});
 await new Promise(resolve=>setTimeout(resolve,320));
 player.stop();await loopDone;
 check(wraps>=2,'loop playback crosses multiple controller-reset cycles');
 check(typeof player.panic==='function'&&player.panic()===true,'live input panic is available without a sequencer');
 const release=player.noteOn('audition',60,0.5);
 player.addController('audition','sustain',1);
 release();
 check(player.panic('audition')===true,'live panic releases sustain-held voices by track');
 player.dispose();
 return JSON.stringify({ok:true,notes:sparse.length,longWavBytes:long.bytes,heldRms:held.rms(0.2,0.42)});
}`).Str()
	if result == "" || result[0] != '{' || result[len(result)-1] != '}' {
		t.Fatalf("audio browser test returned unexpected result: %s", result)
	}
	if result[6:10] != "true" {
		t.Fatalf("audio browser assertions failed: %s", result)
	}
}
