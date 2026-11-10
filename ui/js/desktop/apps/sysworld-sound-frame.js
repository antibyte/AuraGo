// The scene's per-frame input for the soundscape (System World beauty pass, phase 5). One object, filled in place every
// frame: both clocks, the wind (the atmosphere's, or windAt on the app clock when the atmosphere zeroes it for the low
// tier or reduced motion, so the leaves keep sounding), the two trams from traffic, the companion's gait and a stable
// door-openness function. sysworld.js forwards it to sysworld-audio.js `frame()`. No allocation in fill().
import {windAt} from './sysworld-atmosphere.js';
const TRAMS=Object.freeze(['tram-0','tram-1']);
export function createSoundFrame({traffic=null,experience=null,companion=null}={}){
  const doorValue=key=>experience&&experience.doorValue?Number(experience.doorValue(key))||0:0,fallback={x:1,z:0,strength:0};
  const frame={time:0,seaTime:0,wind:{x:1,z:0,strength:0},trams:TRAMS.map(()=>({x:0,z:0,heading:0,on:false})),companion:{x:0,z:0,phase:0,on:false},doorValue,reduced:false};
  function fill(elapsed,seaTime,wind,weather,reduced){
    frame.time=Number(elapsed)||0;frame.seaTime=Number(seaTime)||0;frame.reduced=!!reduced;
    const w=wind&&wind.strength>0?wind:windAt(frame.time,weather,fallback);frame.wind.x=w.x;frame.wind.z=w.z;frame.wind.strength=w.strength;
    for(let i=0;i<TRAMS.length;i++){const b=traffic&&traffic.body?traffic.body(TRAMS[i]):null,t=frame.trams[i];t.on=!!b&&b.enabled!==false;if(b){t.x=b.x;t.z=b.z;t.heading=b.heading||0;}}
    if(companion&&companion.soundPose)companion.soundPose(frame.companion);else frame.companion.on=false;
    return frame;
  }
  return {frame,fill};
}
