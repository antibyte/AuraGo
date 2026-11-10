// System World view levelling: the pitch decays with PITCH_TAU, frame-rate independent, unchanged without time; four seconds
// of walking from the district focus pitch bring the horizon into the middle third of a 43 degree view.
import assert from 'node:assert/strict';
import {easePitch,PITCH_TAU,STEER_HOLD} from '../ui/js/desktop/apps/sysworld-scene.js';
assert.ok(Math.abs(easePitch(.6,PITCH_TAU)-.6/Math.E)<1e-12,'one time constant');
let a=.5;for(let i=0;i<60;i++)a=easePitch(a,1/60);assert.ok(Math.abs(a-easePitch(.5,1))<1e-12,'frame-rate independent');
assert.equal(easePitch(.4,0),.4);assert.equal(easePitch(.4,-1),.4);assert.ok(Number.isNaN(easePitch(NaN,.1)));
const start=Math.atan2(87*.4-2.4,20),limit=Math.atan(Math.tan(43*Math.PI/360)/3);
assert.ok(Math.abs(easePitch(start,4))<limit,'horizon in the middle third after 4 s: '+easePitch(start,4).toFixed(3));
assert.ok(PITCH_TAU>=1&&PITCH_TAU<=2.5&&STEER_HOLD>=.5&&STEER_HOLD<=1.5,'constants in their ranges');
console.log(`System World view: pitch time constant ${PITCH_TAU} s, steering hold ${STEER_HOLD} s; ${start.toFixed(2)} rad levels to ${easePitch(start,4).toFixed(3)} in 4 s.`);
