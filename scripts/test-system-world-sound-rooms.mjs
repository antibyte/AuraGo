// Phase 5 room tones: the table covers every room of sysworld-zones.js, events on the audio clock, bounded voices,
// switching and fading, the leak at an open door, the low tier, dispose.
import assert from 'node:assert/strict';
import {createFakeAudio} from './system-world-audio-fake.mjs';
import * as R from '../ui/js/desktop/apps/sysworld-sound-rooms.js';
import {ROOMS} from '../ui/js/desktop/apps/sysworld-zones.js';
for(const r of Object.values(ROOMS))assert.ok(R.ROOM_TONES[r.tone],'tone '+r.tone);
for(const [name,t]of Object.entries(R.ROOM_TONES)){assert.ok(t.beds.length>=1,name);
  for(const b of t.beds)assert.ok(['sine','saw','noise'].includes(b.wave)&&b.gain>0&&b.gain<=.12,name+' bed');
  for(const e of t.events)assert.ok(['blip','tick','chime','clunk','drop'].includes(e.kind)&&e.every[0]>0&&e.every[1]>=e.every[0],name+' event '+e.kind);}
const seeded=s=>()=>(s=(Math.imul(s,1664525)+1013904223)>>>0)/4294967296;
function rig(tier='high'){const fake=createFakeAudio(),c=fake.context,room=fake.node('room'),send=fake.node('send');return {fake,c,tones:R.createRoomTones(c,{room,send,noise:c.createBuffer(1,c.sampleRate),random:seeded(9),tier})};}
const W=(tone=null,leakTone=null)=>({tone,leakTone});
{
  const {fake,c,tones}=rig();let maxEvents=0;
  for(let k=0;k<60*25;k++){const t=k/25;c.currentTime=t;tones.update(W('agent'),t);maxEvents=Math.max(maxEvents,tones.stats().events);if(k%3===0)fake.flush();}
  const s=tones.stats();assert.equal(s.tone,'agent');assert.ok(s.fired>=7&&s.fired<=21,'agent blips in a minute: '+s.fired);assert.ok(maxEvents<=R.MAX_ROOM_EVENTS);
  tones.update(W('machine'),60);assert.deepEqual([tones.stats().tone,tones.stats().fading],['machine',1]);fake.flush();assert.equal(tones.stats().fading,0,'the old tone is released after its fade');
  tones.update(W(null,'departure'),61);assert.equal(tones.stats().tone,'departure','an open door lets the hall be heard');
  tones.update(W(),62);fake.flush();assert.equal(tones.stats().tone,null);assert.equal(tones.stats().nodes,0);
  tones.dispose();assert.equal(fake.live().filter(n=>!['destination','room','send'].includes(n.kind)).length,0);
}
{
  const {fake,c,tones}=rig('low');tones.update(W('agent'),0);assert.equal(fake.live('oscillator').length+fake.live('source').length,2,'low: the first bed only (its source and its LFO)');
  for(let k=0;k<60*25;k++){c.currentTime=k/25;tones.update(W('agent'),k/25);}assert.equal(tones.stats().fired,0,'low: no room events');
}
console.log('System World room tones: table, events, voices, switching, leak, low tier and dispose verified');
