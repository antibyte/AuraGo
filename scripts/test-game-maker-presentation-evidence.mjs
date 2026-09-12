import assert from 'node:assert/strict';
import {createPresentation} from '../assets/game-maker-presentation/presentation.js';

globalThis.document = Object.assign(new EventTarget(), {hidden:false});
globalThis.matchMedia = () => Object.assign(new EventTarget(), {matches:false});
const delivered=[];
const adapter={dimension:'2d',set(){},quality(){},reset(){delivered.length=0},dispose(){},stats(){return {}},emit(id){delivered.push(id)}};
const fx=createPresentation({adapter,root:new EventTarget(),controls:false,config:{
 effects:[{id:'pickup-glow',defaults:{}}],sounds:[{id:'pickup'}],bindings:[{event:'pickup',sound:'pickup'}],quality:'low'
}});
fx.event('pickup');
assert.deepEqual(delivered,['pickup-glow']);
assert.deepEqual(fx.audit(),{events:{pickup:1},effects:{'pickup-glow':1},sounds:{pickup:1}});
assert.equal(fx.audio.stats().state,'locked','request counts must not imply audible output');
fx.audit().events.pickup=100;
assert.equal(fx.audit().events.pickup,1,'audit must be a detached snapshot');
fx.setPaused(true);fx.event('pickup');
assert.equal(fx.audit().events.pickup,1);
fx.reset();assert.deepEqual(fx.audit(),{events:{},effects:{},sounds:{}});
fx.setPaused(false);
assert.throws(()=>fx.bindEvent('pickup',{effect:'invented'}),/not imported/);
assert.throws(()=>fx.bindEvent('pickup',{sound:'invented'}),/not imported/);
fx.bindEvent('pickup',{effect:'pickup-glow',sound:'pickup'});fx.event('pickup');
assert.deepEqual(delivered,['pickup-glow'],'explicit binding replaces defaults without duplicate feedback');
assert.equal(fx.audit().sounds.pickup,1);
fx.bindEvent('charge',{effect:'pickup-glow'});fx.event('charge');
assert.equal(fx.audit().effects['pickup-glow'],2);
fx.reset();fx.dispose();fx.event('pickup');
assert.deepEqual(fx.audit(),{events:{},effects:{},sounds:{}});
console.log('presentation delivery, pause, reset and detached evidence passed');
