// Phase 5 sound meter: BS.1770 calibration, gating, true peak, the skip window and a WAV round trip.
import assert from 'node:assert/strict';
import {readWav,writeWav,measure,integratedLoudness} from './system-world-sound-meter.mjs';
const R=48000,sine=(f,a,s,phase=0)=>Float32Array.from({length:Math.round(R*s)},(_,i)=>a*Math.sin(2*Math.PI*f*i/R+phase));
const s=sine(997,.1,10);let m=measure([s,s],R);
assert.ok(Math.abs(m.lufs+20)<.1,'a 997 Hz sine at 0.1 in both channels reads -20 LUFS: '+m.lufs);assert.ok(Math.abs(m.samplePeakDb+20)<.01);assert.ok(Math.abs(m.rmsDb+23.01)<.02);
const q=sine(12000,.5,2,Math.PI/4);m=measure([q,q],R);
assert.ok(Math.abs(m.samplePeakDb+9.03)<.05,'sample peak '+m.samplePeakDb);assert.ok(Math.abs(m.truePeakDb+6.02)<.3,'true peak '+m.truePeakDb);
const g=new Float32Array(R*10);g.set(sine(997,.1,5));assert.ok(Math.abs(integratedLoudness([g,g],R)+20)<.2,'silence is gated out');
assert.equal(integratedLoudness([new Float32Array(R*3),new Float32Array(R*3)],R),-Infinity);
const ramp=new Float32Array(R*4);ramp.set(sine(997,.9,1));ramp.set(sine(997,.1,3),R);assert.ok(Math.abs(measure([ramp,ramp],R,{skip:1}).lufs+20)<.1,'skip drops the start');
const back=readWav(writeWav([s.subarray(0,1000),q.subarray(0,1000)],R));assert.equal(back.sampleRate,R);assert.deepEqual(back.channels[0],s.subarray(0,1000));assert.deepEqual(back.channels[1],q.subarray(0,1000));
const b24=readWav(writeWav([s.subarray(0,1000),q.subarray(0,1000)],R,{bits:24}));assert.equal(b24.sampleRate,R);
for(let i=0;i<1000;i++){assert.ok(Math.abs(b24.channels[0][i]-s[i])<2e-7,'24-bit round trip');assert.ok(Math.abs(b24.channels[1][i]-q[i])<2e-7);}
console.log('System World sound meter: BS.1770 calibration, gating, true peak, skip window and WAV round trip (float32, pcm24) verified');
