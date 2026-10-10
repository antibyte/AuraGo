import assert from 'node:assert/strict';
import fs from 'node:fs';
import {VoxelWorld} from '../internal/gamemaker/runtime/voxel-world.js';
import {VoxelGame,validateVoxelState} from '../internal/gamemaker/runtime/voxel-rules.js';

const definition=JSON.parse(fs.readFileSync(process.argv[2],'utf8'));
const world=new VoxelWorld(definition),same=new VoxelWorld(definition);
assert.deepEqual(world.data,same.data,'same seed must produce identical grid');
assert.deepEqual(world.data,new VoxelWorld({...definition,blocks:[...definition.blocks].reverse()}).data,'identity-preserving reorder must preserve generated terrain');
assert.notDeepEqual(world.data,new VoxelWorld({...definition,seed:'other'}).data);
const spawn=world.spawn();assert.equal(world.collides(spawn),false);
const floor={...spawn};world.move(floor,{x:0,y:-30,z:0});assert(floor.y>=1);assert(world.collides({...floor,y:floor.y-.16}));
const block=definition.blocks[0].id;world.dirty.clear();assert(world.set(15,31,15,block));
assert(world.dirty.has('0,1,0')&&world.dirty.has('1,1,0')&&world.dirty.has('0,2,0')&&world.dirty.has('0,1,1'),'chunk edge invalidates neighbors');
assert(world.collides({x:15.5,y:31,z:15.5}));world.set(15,31,15,0);assert(!world.collides({x:15.5,y:31,z:15.5}));
world.set(15,31,15,block);const mesh=world.mesh('0,1,0');assert(mesh.indices.length>0);assert.equal(mesh.normals.length,mesh.positions.length);
assert.equal(world.ray({x:15.5,y:31.5,z:18.5},[0,0,-1],6)?.id,block);
assert.equal(world.set(-1,2,2,block),false);assert.equal(world.set(2,0,2,0),false);
const game=new VoxelGame({...definition,terrain:'flat',enemies:[]});
let target=game.aim();assert(target,'starter has a normally reachable resource');
const before=game.world.get(...target.cell);game.primary(4);assert.equal(game.world.get(...target.cell),0);assert(game.inventory.some(Boolean));
const unchanged=JSON.stringify(game.inventory);assert.equal(game.craft('metal_tool'),false);assert.equal(JSON.stringify(game.inventory),unchanged,'failed craft consumes nothing');
assert(game.credit('wood',6));const wood=game.count('wood');assert(game.craft('wood_tool'));assert.equal(game.count('wood'),wood-2);assert.equal(game.count('wood_tool'),1);
assert.equal(game.transaction({wood:2},{missing:1}),false);assert.equal(game.count('wood'),wood-2,'invalid output must not consume ingredients');
assert.equal(game.transaction({wood:-1},{}),false);
const full=new VoxelGame({...definition,enemies:[]});full.inventory=Array.from({length:36},()=>({item:'wood',count:64}));const fullBefore=JSON.stringify(full.inventory);
assert.equal(full.craft('wood_tool'),false);assert.equal(JSON.stringify(full.inventory),fullBefore,'full inventory craft must not consume ingredients');
const creative=new VoxelGame({...definition,mode:'creative',enemies:[]});assert(creative.supply('metal_tool'));assert.equal(creative.held().id,'metal_tool');creative.damage(100);assert(creative.alive);
for (const mode of ['creative', 'survival']) {
    const mining = new VoxelGame({...definition, mode, terrain:'flat', enemies:[]});
    const target = mining.aim();assert(target);
    const blockBefore = mining.world.get(...target.cell);
    while (mining.credit('wood', mining.items.get('wood').stack)) {}
    assert.equal(mining.inventory.filter(Boolean).length, 36);
    const inventoryBefore = JSON.stringify(mining.inventory), pickupsBefore = mining.metrics.pickups, revisionBefore = mining.revision;
    mining.primary(4);
    assert.equal(mining.world.get(...target.cell), mode === 'creative' ? 0 : blockBefore);
    assert.equal(mining.metrics.mined, mode === 'creative' ? 1 : 0);
    assert.equal(mining.metrics.pickups, pickupsBefore, 'discarded creative drops must not count as collected');
    assert.deepEqual(mining.progress, {}, 'discarded drops must not advance collection objectives');
    assert.equal(JSON.stringify(mining.inventory), inventoryBefore, 'full inventory must remain unchanged');
    assert(validateVoxelState(mining.definition, mining.snapshot()));
    if (mode === 'creative') {
        assert(mining.revision > revisionBefore);
        const saved = new VoxelGame(mining.definition);saved.restore(mining.snapshot());
        assert.equal(saved.world.get(...target.cell), 0, 'creative removal survives saving');
    }
}
const movingEnemies=new VoxelGame({...definition,terrain:'flat'}),enemyRevision=movingEnemies.revision;
movingEnemies.tickEnemies(.05);assert(movingEnemies.revision>enemyRevision,'enemy movement must mark saves dirty even when the player is idle');
const state=game.snapshot();assert(validateVoxelState(game.definition,state));
const restored=new VoxelGame(game.definition);restored.restore(state);assert.deepEqual(restored.world.data,game.world.data);assert.deepEqual(restored.inventory,game.inventory);
assert.throws(()=>validateVoxelState(definition,{...state,selected:9}));
assert.throws(()=>validateVoxelState(definition,{...state,chunks:[{position:[0,0,0],runs:[65,4096]}]}));
assert.throws(()=>validateVoxelState(definition,{...state,inventory:[{item:'wood',count:99999},...Array(35).fill(null)]}));
game.damage(100);assert(!game.alive);assert(game.respawn());assert.equal(game.player.health,100);
console.log('Voxel deterministic world, mesh, collision, transactions and save validation passed');
