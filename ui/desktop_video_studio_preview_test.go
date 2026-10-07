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
const allMedia = [], audioContexts = [];
class MediaElement {
  constructor() { this.src=''; this.paused=true; this.currentTime=0; this.readyState=2; this.videoWidth=640; this.videoHeight=360; this.removed=0; }
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
global.document={createElement(tag) { assert.equal(tag,'video'); const element=new VideoElement(); allMedia.push(element); return element; }};
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
const drawing={fillRect(){},save(){},translate(){},rotate(){},drawImage(el){if(el instanceof ImageElement){assert(el.naturalWidth>0&&el.naturalHeight>0,'unloaded or broken image reached canvas');imageDraws++;}},restore(){},clearRect(){}};
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
`
	if output, err := exec.Command(node, "-e", script).CombinedOutput(); err != nil {
		t.Fatalf("Video Studio preview lifecycle checks: %v\n%s", err, output)
	}
}
