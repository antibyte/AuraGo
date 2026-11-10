// Phase 5 sound frame: copies, the wind fallback, stable objects, door routing and missing parts.
import assert from 'node:assert/strict';
import {createSoundFrame} from '../ui/js/desktop/apps/sysworld-sound-frame.js';
import {windAt} from '../ui/js/desktop/apps/sysworld-atmosphere.js';
const bodies=new Map([['tram-0',{x:10,z:-77,heading:1.2,enabled:true}],['tram-1',{x:0,z:0,heading:0,enabled:false}]]);
const s=createSoundFrame({traffic:{body:id=>bodies.get(id)},experience:{doorValue:key=>key==='pavilion:agent'?.4:key==='lobby:agent:0'?.9:0},
  companion:{soundPose(out){out.on=true;out.x=30;out.z=69.5;out.phase=3;return out;}}}),f=s.frame;
assert.equal(s.fill(12,5,{x:.5,z:.8,strength:.6},'clear',false),f,'fill returns the reused frame');
assert.deepEqual([f.time,f.seaTime,f.wind.strength,f.trams[0].on,f.trams[0].x,f.trams[0].heading,f.trams[1].on,f.companion.phase],[12,5,.6,true,10,1.2,false,3]);
assert.equal(f.doorValue('pavilion:agent'),.4);assert.equal(f.doorValue('lobby:agent:0'),.9);
const trams=f.trams,wind=f.wind;s.fill(13,5,{x:1,z:0,strength:0},'rain',true);
assert.ok(Math.abs(f.wind.strength-windAt(13,'rain',{}).strength)<1e-12,'a zeroed atmosphere wind (low tier, reduced motion) keeps the wind sound');
assert.equal(f.trams,trams);assert.equal(f.wind,wind);assert.equal(f.reduced,true);
const bare=createSoundFrame({});bare.fill(1,1,null,'clear',false);
assert.deepEqual([bare.frame.trams[0].on,bare.frame.companion.on,bare.frame.doorValue('lobby:agent:0')],[false,false,0]);
console.log('System World sound frame: clocks, wind fallback, trams, companion, doors and stable objects verified');
