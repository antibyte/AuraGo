import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
const source=await readFile(new URL('../internal/gamemaker/runtime/isometric.js',import.meta.url),'utf8');
const {projectIso,unprojectIso,createIsometricWorld,isoDirection}=await import('data:text/javascript;base64,'+Buffer.from(source).toString('base64'));
for(let x=-10;x<10;x++)for(let y=-10;y<10;y++)for(let z=0;z<5;z++) {
 const origin={x:31,y:80},screen=projectIso([x,y,z],origin);assert.deepEqual(unprojectIso(screen.x,screen.y,z,origin),[x,y,z]);
}
assert.equal(isoDirection(1,0),'down-right');assert.equal(isoDirection(0,1),'down-left');assert.equal(isoDirection(0,0),null);
const scene={schema_version:2,dimension:'2d',projection:{kind:'isometric'},levels:[{id:'outdoors',active:true},{id:'inside',active:false}],
 nodes:[{id:'low',position:[0,0,0],properties:{walkable:true}},{id:'high',position:[1,0,1],properties:{walkable:true}},
 {id:'player',position:[.25,.25,0]},{id:'roof',position:[1,0,2],region_id:'house',properties:{roof:true}},{id:'inside-floor',level_id:'inside',position:[0,1,0],properties:{walkable:true}}],routes:[]};
let creates=0,destroys=0;
const host={create:n=>{creates++;return {...n}},destroy:()=>destroys++,position:(o,p,d)=>{o.screen=p;o.depth=d},visible:(o,v)=>{o.visible=v}};
let world=createIsometricWorld(scene,host);
assert.equal(world.canMove([.5,.5,0],[1.5,.5,1]),false,'height changes need a declared link');
world.dispose();world.dispose();assert.equal(destroys,creates);
scene.routes=[{id:'steps',from:'low',to:'high',kind:'stairs'}];world=createIsometricWorld(scene,host);
assert.equal(world.canMove([.5,.5,0],[1.5,.5,1]),true);assert.equal(world.move('player',1,0),true);
assert.equal(world.records.get('player').at[2],1);assert.equal(world.move('player',0,3),true);assert.ok(world.records.get('player').at[1]<1,'cannot walk past floor edge');
const at=projectIso([1.5,.5,1]);assert.equal(world.pick(at.x,at.y),'high');
world.hideRoof('house');assert.equal(world.records.get('roof').object.visible,false);
world.level('inside');assert.ok(world.records.has('inside-floor'));world.dispose();assert.equal(destroys,creates);
assert.throws(()=>projectIso([Infinity,0,0]));
scene.nodes.push({id:'door',position:[1,0,1],properties:{solid:true}},{id:'wall',position:[1,0,1],properties:{solid:true}});
world=createIsometricWorld(scene,host);
world.setSolid('door',false);
assert.equal(world.canMove([.5,.5,0],[1.5,.5,1]),false,'opening a door must preserve overlapping wall collision');
world.remove('wall');assert.equal(world.canMove([.5,.5,0],[1.5,.5,1]),true);
world.setSolid('door',true);assert.equal(world.canMove([.5,.5,0],[1.5,.5,1]),false);
assert.throws(()=>world.remove('low'),/validated scene/);
world.dispose();assert.equal(destroys,creates);
console.log('PASS: isometric round trips, elevation links, bounded movement, picking, roofs, levels and cleanup');
