package ui

import (
	"os/exec"
	"testing"
)

func TestDesktopVideoStudioPreviewMediaLifecycle(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js required for Video Studio preview checks")
	}
	script := `
const assert = require('node:assert/strict');
global.window = {};
global.location = {origin:'http://localhost'};
require('./js/desktop/apps/video-studio-time.js');
const allMedia = [], audioContexts = [];
class MediaElement {
  constructor() { this.src=''; this.paused=true; this.currentTime=0; this.playbackRate=1; this.readyState=2; this.videoWidth=640; this.videoHeight=360; this.removed=0; }
  addEventListener() {}
  load() { this.loads=(this.loads||0)+1; }
  pause() { this.paused=true; }
  play() { this.paused=false; return Promise.resolve(); }
  removeAttribute(name) { if (name==='src') { this.src=''; this.removed++; } }
}
class VideoElement extends MediaElement {}
global.HTMLMediaElement=MediaElement;
global.HTMLVideoElement=VideoElement;
const images=[];
class ImageElement {
  constructor() { this.naturalWidth=0; this.naturalHeight=0; images.push(this); }
  addEventListener() {}
  removeAttribute(name) { if (name==='src') this.src=''; }
}
global.Image=ImageElement;
const layers=[];
global.document={createElement(tag) { if (tag==='canvas') { const layer={width:0,height:0,getContext:()=>layerDrawing}; layers.push(layer); return layer; } assert.equal(tag,'video'); const element=new VideoElement(); allMedia.push(element); return element; }};
global.requestAnimationFrame=()=>1;
global.cancelAnimationFrame=()=>{};
global.performance={now:()=>100};
class AudioNode {
  constructor() { this.disconnects=0; this.gain={value:0}; }
  connect() {}
  disconnect() { this.disconnects++; }
}
class AudioContext {
  constructor() { this.state='running'; this.destination={}; this.sources=[]; this.gains=[]; audioContexts.push(this); }
  createMediaElementSource() { const node=new AudioNode(); this.sources.push(node); return node; }
  createGain() { const node=new AudioNode(); this.gains.push(node); return node; }
  resume() { return Promise.resolve(); }
  close() { this.closed=true; return Promise.resolve(); }
}
window.AudioContext=AudioContext;
let imageDraws=0;
const layerDraws=[];
const drawing={fillRect(){},save(){},translate(){},rotate(){},beginPath(){},rect(){},clip(){},drawImage(el){if(el instanceof ImageElement){assert(el.naturalWidth>0&&el.naturalHeight>0,'unloaded or broken image reached canvas');imageDraws++;}if(layers.includes(el))layerDraws.push(el);},restore(){},clearRect(){}};
const layerDrawing={...drawing,drawImage(el){this.alphas=(this.alphas||[]).concat([[el.src,this.globalAlpha,this.globalCompositeOperation||'source-over']]);}};
const canvas={width:0,height:0,getContext:()=>drawing};
require('./js/desktop/apps/video-studio-timeline.js');
require('./js/desktop/apps/video-studio-preview.js');
const T=window.VideoStudioTimeline, Preview=window.VideoStudioPreview;
const clip=(id,assetId,start,duration,transition=null)=>({id,asset_id:assetId,start,offset:0,duration,x:0,y:0,width:1,height:1,rotation:0,opacity:1,volume:1,fade_in:0,fade_out:0,fit:'contain',transition});
const shared={id:'shared',kind:'video',media_url:'/media/shared.mp4',has_audio:true,duration_frames:30,width:640,height:360};
const longProject={version:1,name:'256 clips',width:1280,height:720,fps:30,assets:[shared],tracks:[{id:'video',kind:'video',hidden:false,muted:false,clips:Array.from({length:256},(_,i)=>clip('clip-'+i,'shared',i*30,30))}]};
assert(T.validTimeline(longProject),'256 sequential clips must be a valid sub-10-minute timeline');
assert.equal(longProject.tracks[0].clips.at(-1).start+30,7680);
const live=()=>allMedia.filter(element=>element.src);
const preview=Preview.mount(canvas,()=>longProject,()=>0,()=>{},()=>{});
assert.equal(typeof preview.setRate, 'function', 'the shuttle sets the rate');
assert.equal(typeof preview.setLoop, 'function', 'the transport sets the loop range');
assert.equal(typeof preview.setSolo, 'function', 'the timeline sets audio solo');
preview.setRate(-2);
assert.equal(preview.rate(), -2, 'a backward rate is kept');
preview.setRate(20);
assert.equal(preview.rate(), 8, 'rates stop at 8x');
preview.setRate(1);
preview.setLoop({start: 10, end: 5});
preview.setLoop({start: 10, end: 60});
preview.setSolo(['a1']);
preview.setSolo([]);
assert.equal(allMedia.length,3,'only current clip and the two-second lookahead should create players');
assert.equal(live().length,3);
assert.equal(allMedia[0].preload,'auto');
assert(allMedia.slice(1).every(element=>element.preload==='metadata'));
preview.play();
assert.equal(audioContexts[0].sources.length,3);
for (let i=0;i<3;i++) {
  preview.seek(255*30);
  assert.equal(live().length,1,'seeking forward must release distant players');
  preview.seek(0);
  assert.equal(live().length,3,'seeking backward may retain only active and near players');
}
assert.equal(live().length,3,'repeated seeks must not accumulate live media sources');
preview.dispose();
assert.equal(live().length,0);
assert(allMedia.every(element=>element.paused && !element.src && element.removed>0));
assert(audioContexts[0].closed);
assert(audioContexts[0].sources.every(node=>node.disconnects===1));
assert(audioContexts[0].gains.every(node=>node.disconnects===1));

const red={...shared,id:'red',duration_frames:90,media_url:'/media/red.mp4'};
const blue={...shared,id:'blue',duration_frames:60,media_url:'/media/blue.mp4'};
const transitionProject={version:1,name:'Transition',width:1280,height:720,fps:30,assets:[red,blue],tracks:[{id:'video',kind:'video',clips:[clip('outgoing','red',0,90,{type:'dissolve',duration:30}),clip('incoming','blue',60,60)]}]};
assert(T.validTimeline(transitionProject),'overlap must match the outgoing transition');
const transition=Preview.mount(canvas,()=>transitionProject,()=>75,()=>{},()=>{});
assert.equal(live().length,2,'both overlapping transition sources must stay loaded');
assert(live().every(element=>element.preload==='auto'));
assert.equal(layers.length,1,'a dissolve composes its pair in one track layer');
assert(layerDraws.length>0,'the track layer reaches the canvas');
const mid=layerDrawing.alphas.slice(-2);
assert.deepEqual(mid.map(([src,alpha,mode])=>[src,Math.round(alpha*100)/100,mode]),[['http://localhost/media/red.mp4',0.5,'source-over'],['http://localhost/media/blue.mp4',0.5,'lighter']],'outgoing fades out while incoming adds in, like xfade');
transition.play();
const transitionAudio=audioContexts[1];
assert.equal(transitionAudio.sources.length,2);
transition.dispose();
assert.equal(live().length,0);
assert(transitionAudio.closed);
assert(transitionAudio.sources.every(node=>node.disconnects===1));
assert(transitionAudio.gains.every(node=>node.disconnects===1));

const imageAsset={...shared,id:'image',kind:'image',has_audio:false,media_url:'/media/image.png'};
const imageProject={...longProject,assets:[imageAsset],tracks:[{id:'overlay',kind:'overlay',clips:[clip('image-clip','image',0,30)]}]};
const imagePreview=Preview.mount(canvas,()=>imageProject,()=>0,()=>{},()=>{});
assert.equal(imageDraws,0,'metadata dimensions cannot make an undecoded image drawable');
images[0].naturalWidth=640;images[0].naturalHeight=360;
imagePreview.seek(0);
assert.equal(imageDraws,1,'decoded image must still render');
images[0].naturalWidth=0;images[0].naturalHeight=0;
imagePreview.seek(0);
assert.equal(imageDraws,1,'broken image must not break later project refreshes');
imagePreview.dispose();
assert.equal(images[0].src,'');

// Shuttle runs with a hand-driven clock: the end stop, reverse stepping, the loop wrap, mute and solo.
let clock=1000, pending=null, shown=-1;
global.performance={now:()=>clock};
global.requestAnimationFrame=callback=>{pending=callback;return 2;};
const runTo=ms=>{clock=ms;const callback=pending;pending=null;if(callback)callback(ms);};
const shortProject={...longProject,assets:[shared],tracks:[{id:'video',kind:'video',clips:[clip('one','shared',0,60)]}]};
const shuttle=Preview.mount(canvas,()=>shortProject,()=>0,value=>{shown=value;},()=>{});
const element=allMedia.at(-1);
shuttle.setRate(2); shuttle.play();
runTo(1500); assert.equal(shown,30); assert.equal(element.playbackRate,2,'forward rates run the media clock at the rate');
runTo(2200); assert.equal(shown,59,'a shuttle stops on the last frame, not past it');
assert.equal(shuttle.isPlaying(),false); assert.equal(shuttle.rate(),1,'stopping returns the rate to 1x');
shuttle.seek(45); shuttle.setRate(-1); shuttle.play();
runTo(2300); assert.equal(shown,42); assert(element.paused,'backward playback keeps the media clock paused');
runTo(2350); assert.equal(shown,42,'backward playback repaints at most every 100 ms');
runTo(2400); assert.equal(shown,39);
runTo(4000); assert.equal(shown,0,'backward playback stops at the start'); assert.equal(shuttle.isPlaying(),false);
shuttle.setLoop({start:10,end:20}); shuttle.seek(15); shuttle.play();
runTo(4400); assert.equal(shown,10,'looping wraps to the range start');
runTo(4500); assert.equal(shown,13);
const gain=audioContexts.at(-1).gains[0].gain;
assert.equal(gain.value,1,'normal speed sounds');
shuttle.setRate(4); assert.equal(gain.value,0,'fast forward above 2x is silent');
shuttle.setRate(1); shuttle.setSolo(['a1']); assert.equal(gain.value,0,'a solo silences embedded video audio');
shuttle.setSolo([]); assert.equal(gain.value,1);
shuttle.dispose();
`
	if output, err := exec.Command(node, "-e", script).CombinedOutput(); err != nil {
		t.Fatalf("Video Studio preview lifecycle checks: %v\n%s", err, output)
	}
}
