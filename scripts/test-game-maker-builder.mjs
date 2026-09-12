import assert from 'node:assert/strict';
import {createScene2D, createScene3D, normalizeScene} from '../internal/gamemaker/runtime/scene-builder.js';

const scene = {
    schema_version: 1,
    dimension: '2d',
    seed: 1729,
    levels: [{id: 'main', active: true}],
    world_bounds: {min: [0, 0, 0], max: [100, 80, 0]},
    nodes: [
        {id: 'courier', kind: 'player', position: [10, 10, 0], size: [8, 8, 0], properties: {player: true}},
        {id: 'parcel', kind: 'entity', position: [10, 10, 0], size: [8, 8, 0], properties: {points: 3, win_when_cleared: true}},
    ],
    placements: [{id: 'parcel-art', node_id: 'parcel', asset_role: 'parcel', behavior: 'collect', position: [10, 10, 0]}],
};

const normalized = normalizeScene(scene, '2d');
assert.deepEqual(normalized.nodes.map(node => node.id), ['parcel', 'courier']);
assert.equal(normalized.nodes.find(node => node.id === 'parcel').behaviors[0].type, 'collect');

const events = [];
const state = {};
const objects = new Map();
const host = {
    state,
    createNode(node) {
        const object = {x: node.at[0], y: node.at[1], active: true, setPosition(x, y) { this.x = x; this.y = y; return this; }};
        objects.set(node.id, object);
        return {object};
    },
    player: () => ({object: objects.get('courier')}),
    overlap: (_node, object, player) => Math.hypot(object.x - player.object.x, object.y - player.object.y) < 9,
    actualRole: node => node.role,
    actualAssetID: node => node.id === 'parcel' ? 'runtime-parcel' : '',
    event: type => events.push(type),
    removeNode: (_node, object) => { object.active = false; },
    resetNode: (_node, object, at) => { object.active = true; object.setPosition(at[0], at[1]); },
};
const builder = createScene2D(scene, host);
assert.equal(builder.audit().outcome, 'playing');
builder.step(16);
assert.equal(builder.audit().outcome, 'won');
assert.deepEqual(events, ['pickup', 'win']);
assert.equal(builder.audit().events.pickup, 1);
assert.equal(builder.audit().events.win, 1);
assert.equal(builder.audit().roles.find(row => row.asset_id === 'runtime-parcel').count, 1);
builder.reset();
assert.equal(builder.audit().outcome, 'playing');
assert.deepEqual(builder.audit().events, {});
builder.dispose();

const threeScene = {...scene, dimension: '3d', world_bounds: {min: [-2, 0, -2], max: [2, 4, 8]}};
const threeObjects = new Map();
const three = createScene3D(threeScene, {
    state: {},
    createNode: node => {
        const object = {position: {x: node.at[0], y: node.at[1], z: node.at[2]}};
        threeObjects.set(node.id, object);
        return {object};
    },
    player: () => ({object: threeObjects.get('courier')}),
});
assert.equal(three.audit().nodes.length, 2);
assert.equal(three.audit().nodes.find(node => node.id === 'parcel').z, 0);
three.dispose();

console.log('scene builder normalization, collect/win, audit, reset and 3D binding passed');

const manyNodes = Array.from({length: 4097}, (_, index) => ({
    id: 'n' + index,
    kind: 'decorative',
    position: [index % 100, Math.floor(index / 100), 0],
    size: [1, 1, 0],
}));
const manyNormalized = normalizeScene({nodes: manyNodes}, '2d', {width: 100, height: 100});
assert.equal(manyNormalized.nodes.length, 4096);
const multiPlacement = normalizeScene({
    nodes: [{id: 'shared', kind: 'entity', position: [5, 5, 0], size: [4, 4, 0]}],
    placements: [
        {id: 'visual-a', node_id: 'shared', asset_role: 'tree', behavior: 'decorative', position: [5, 5, 0]},
        {id: 'visual-b', node_id: 'shared', asset_role: 'shadow', behavior: 'decorative', position: [5, 5, 0]},
    ],
}, '2d');
assert.deepEqual(multiPlacement.nodes.map(node => node.id), ['shared']);
assert.equal(multiPlacement.nodes[0].placements.length, 2);
assert.deepEqual(multiPlacement.nodes[0].scale, [1, 1]);
const omittedSize = normalizeScene({nodes: [{id: 'omitted-size', size: [0, 0, 0]}]}, '2d');
assert.deepEqual(omittedSize.nodes[0].size, [32, 32]);

function simpleHost(initialObjects, extra = {}) {
    const objects = new Map(Object.entries(initialObjects));
    return {
        state: {},
        createNode(node) {
            const object = {
                x: node.at[0], y: node.at[1], z: node.at[2] || 0, active: true,
                setPosition(x, y) { this.x = x; this.y = y; return this; },
            };
            objects.set(node.id, object);
            return {object};
        },
        player: () => ({object: objects.get('player')}),
        overlap: (_node, object, player) => {
            const target = player?.object || player;
            return !!object?.active && !!target?.active &&
                Math.hypot(object.x - target.x, object.y - target.y) < 8;
        },
        move: (_node, object, delta) => { object.x += delta[0]; object.y += delta[1]; object.z += delta[2] || 0; },
        removeNode: (_node, object) => { object.active = false; },
        resetNode: (_node, object, at) => { object.active = true; object.setPosition(at[0], at[1]); },
        actualRole: node => node.role || '',
        actualAssetID: node => node.asset_id || '',
        ...extra,
    };
}

const zoneEvents = [];
const zoneBuilder = createScene2D({
    nodes: [{id: 'player', kind: 'player', position: [5, 5, 0], size: [2, 2, 0], properties: {player: true}}],
    zones: [
        {id: 'spawn', kind: 'spawn', bounds: {min: [0, 0, 0], max: [10, 10, 0]}},
        {id: 'trigger', kind: 'trigger', bounds: {min: [0, 0, 0], max: [10, 10, 0]}, properties: {event: 'alarm'}},
    ],
}, simpleHost({}, {event: type => zoneEvents.push(type)}));
zoneBuilder.step(16);
assert.deepEqual(zoneEvents, ['spawn', 'alarm']);
assert.equal(zoneBuilder.audit().outcome, 'playing');
zoneBuilder.dispose();

const goalBuilder = createScene2D({
    nodes: [{id: 'player', kind: 'player', position: [5, 5, 0], size: [2, 2, 0], properties: {player: true}}],
    zones: [{id: 'goal-zone', kind: 'goal', bounds: {min: [0, 0, 0], max: [10, 10, 0]}}],
}, simpleHost({}));
goalBuilder.step(16);
assert.equal(goalBuilder.audit().outcome, 'won');
assert.equal(goalBuilder.audit().events.goal, 1);
goalBuilder.dispose();

const lossBuilder = createScene2D({
    nodes: [{id: 'player', kind: 'player', position: [5, 5, 0], size: [2, 2, 0], properties: {player: true}}],
    zones: [{id: 'loss-zone', kind: 'loss', bounds: {min: [0, 0, 0], max: [10, 10, 0]}}],
}, simpleHost({}, {state: {lives: 1}}));
lossBuilder.step(16);
assert.equal(lossBuilder.audit().outcome, 'lost');
lossBuilder.dispose();

const mechanicBuilder = createScene2D({
    nodes: [{id: 'enemy', kind: 'entity', role: 'enemy', position: [20, 20, 0], size: [4, 4, 0]}],
}, simpleHost({}, {mechanics: {
    blocks: [{kind: 'destroy', role: 'enemy', value: true, params: JSON.stringify({health: 2})}],
}}));
assert.equal(mechanicBuilder.scene.nodes[0].behaviors[0].type, 'destroy');
assert.equal(mechanicBuilder.hit('enemy'), true);
assert.equal(mechanicBuilder.hit('enemy'), true);
assert.equal(mechanicBuilder.audit().events.hit, 2);
assert.equal(mechanicBuilder.audit().events.destroy, 1);
mechanicBuilder.dispose();

const waveBuilder = createScene2D({
    nodes: [
        {id: 'spawner', kind: 'entity', position: [20, 20, 0], size: [4, 4, 0],
            behavior: {type: 'waves', count: 1, node_id: 'enemy-template'}},
        {id: 'enemy-template', kind: 'entity', role: 'enemy', position: [40, 20, 0], size: [4, 4, 0],
            behavior: {type: 'destroy'}},
    ],
}, simpleHost({}));
waveBuilder.step(16);
assert.ok(waveBuilder.audit().nodes.some(node => node.id.startsWith('enemy-template-wave-')));
waveBuilder.reset();
assert.equal(waveBuilder.audit().nodes.length, 2);
waveBuilder.dispose();

const projectileBuilder = createScene2D({
    nodes: [{id: 'target', kind: 'entity', role: 'target', position: [5, 0, 0], size: [2, 2, 0],
        behavior: {type: 'destroy'}}],
}, simpleHost({player: {x: -20, y: -20, active: true}}));
const projectileID = projectileBuilder.spawnProjectile({
    id: 'shot', position: [0, 0, 0],
    behavior: {direction: [1, 0, 0], speed: 500, ttl: 1},
});
assert.ok(projectileID);
projectileBuilder.step(16);
assert.equal(projectileBuilder.audit().events.hit, 1);
assert.equal(projectileBuilder.audit().nodes.find(node => node.id === 'target').active, false);
projectileBuilder.dispose();

const blockedEvents = [];
const blockedBuilder = createScene2D({
    nodes: [{id: 'runner', kind: 'entity', position: [0, 0, 0], size: [2, 2, 0],
        behavior: {type: 'patrol', points: [[10, 0, 0], [0, 0, 0]], speed: 20}}],
}, simpleHost({}, {
    canMove: () => false,
    event: type => blockedEvents.push(type),
}));
blockedBuilder.step(100);
assert.equal(blockedEvents.includes('blocked'), true);
blockedBuilder.dispose();

const previousWindow = globalThis.window;
const previousLocation = globalThis.location;
let debugMessage;
let removedDebug = false;
const parentWindow = {};
globalThis.location = {hash: '#gm-channel=channel-42'};
globalThis.window = {
    parent: parentWindow,
    addEventListener: (_type, listener) => { debugMessage = listener; },
    removeEventListener: (_type, listener) => { removedDebug = listener === debugMessage; },
};
const debugBuilder = createScene2D({nodes: []}, {state: {}});
debugMessage({source: parentWindow, origin: '', data: {
    source: 'aurago-studio', type: 'scene-debug', channel: 'channel-42', enabled: true,
}});
assert.equal(debugBuilder.debug.enabled, true);
debugBuilder.dispose();
assert.equal(removedDebug, true);
globalThis.window = previousWindow;
globalThis.location = previousLocation;

console.log('scene builder extended mechanics, zones, limits, waves, projectiles, collision and debug passed');

const actionEvents = [];
const attachedPlacements = [];
const actionBuilder = createScene2D({
    nodes: [
        {id: 'player', kind: 'player', position: [0, 0, 0], size: [2, 2, 0], properties: {player: true}},
        {id: 'target', kind: 'entity', position: [5, 0, 0], size: [2, 2, 0], behavior: {type: 'destroy'}},
    ],
    placements: [
        {id: 'player-body', node_id: 'player', asset_role: 'player-body'},
        {id: 'player-shadow', node_id: 'player', asset_role: 'player-shadow'},
    ],
}, simpleHost({}, {
    mechanics: {blocks: [{
        id: 'shoot', kind: 'projectile', target: 'player',
        params: JSON.stringify({direction: [1, 0, 0], speed: 500, ttl: 1, cooldown: 0.5}),
    }]},
    attachVisual: (_node, placement) => attachedPlacements.push(placement.id),
    event: type => actionEvents.push(type),
}));
assert.deepEqual(attachedPlacements, ['player-body', 'player-shadow']);
assert.equal(actionBuilder.action({input: 'fire'}), 1);
assert.equal(actionBuilder.action({input: 'fire'}), 0);
actionBuilder.step(16);
assert.equal(actionBuilder.audit().events.hit, 1);
assert.equal(actionBuilder.audit().nodes.find(node => node.id === 'target').active, false);
assert.ok(actionEvents.includes('projectile_hit'));
actionBuilder.reset();
assert.equal(actionBuilder.action(), 1);
actionBuilder.dispose();

const invalidMechanicBuilder = createScene2D({
    nodes: [{id: 'player', kind: 'player', position: [0, 0, 0], size: [2, 2, 0], properties: {player: true}}],
}, simpleHost({}, {mechanics: {blocks: [{kind: 'unknown', target: 'player'}]}}));
assert.ok(invalidMechanicBuilder.audit().faults.includes('mechanic_unknown:unknown'));
invalidMechanicBuilder.dispose();

console.log('scene builder action, placement attachment and mechanic validation passed');
const inventoryBuilder = createScene2D({
    nodes: [
        {id: 'player', kind: 'player', position: [5, 5, 0], size: [2, 2, 0], properties: {player: true}},
        {id: 'coin', kind: 'entity', position: [5, 5, 0], size: [2, 2, 0], behavior: {type: 'inventory', key: 'coin'}},
    ],
}, simpleHost({}));
inventoryBuilder.step(16);
inventoryBuilder.step(16);
assert.equal(inventoryBuilder.audit().inventory.coin, 1);
inventoryBuilder.dispose();

const worldBlockedEvents = [];
const worldBuilder = createScene2D({
    world_bounds: {min: [0, 0, 0], max: [10, 10, 0]},
    nodes: [{id: 'runner', kind: 'entity', position: [1, 5, 0], size: [2, 2, 0],
        behavior: {type: 'patrol', points: [[-10, 5, 0], [1, 5, 0]], speed: 20}}],
}, simpleHost({}, {event: type => worldBlockedEvents.push(type)}));
worldBuilder.step(100);
assert.ok(worldBlockedEvents.includes('blocked'));
worldBuilder.dispose();

console.log('scene builder one-shot contact and world-bound fallback passed');
const movementMoves = [];
const movementBuilder = createScene2D({
    nodes: [{id: 'runner', kind: 'entity', position: [0, 0, 0], size: [2, 2, 0]}],
}, simpleHost({}, {
    mechanics: {blocks: [{id: 'patrol', kind: 'movement', target: 'runner',
        params: JSON.stringify({mode: 'patrol', points: [[10, 0, 0], [0, 0, 0]], speed: 20})}]},
    move: (node, object, delta) => { movementMoves.push(delta); object.x += delta[0]; object.y += delta[1]; },
}));
assert.equal(movementBuilder.scene.nodes[0].behaviors[0].type, 'movement');
assert.equal(movementBuilder.scene.nodes[0].behaviors[0].mode, 'patrol');
movementBuilder.step(100);
assert.ok(movementMoves.length > 0);
movementBuilder.dispose();

const playerMovementMoves = [];
const playerMovementBuilder = createScene2D({
    nodes: [{id: 'player', kind: 'player', position: [0, 0, 0], size: [2, 2, 0], properties: {player: true}}],
}, simpleHost({}, {
    mechanics: {blocks: [{id: 'player-move', kind: 'movement', target: 'player',
        params: JSON.stringify({mode: 'patrol', points: [[10, 0, 0], [0, 0, 0]], speed: 20})}]},
    move: (node, object, delta) => { playerMovementMoves.push(delta); object.x += delta[0]; object.y += delta[1]; },
}));
playerMovementBuilder.step(100);
assert.equal(playerMovementMoves.length, 0);
playerMovementBuilder.dispose();

const healthState = {};
const healthBuilder = createScene2D({
    nodes: [{id: 'player', kind: 'player', position: [0, 0, 0], size: [2, 2, 0], properties: {player: true}}],
}, simpleHost({}, {
    state: healthState,
    mechanics: {blocks: [
        {id: 'health', kind: 'health', target: 'player', value: 3},
        {id: 'damage', kind: 'damage', target: 'player', params: JSON.stringify({amount: 2, on_action: true})},
    ]},
}));
assert.equal(healthState.health, 3);
assert.equal(healthBuilder.audit().nodes.find(node => node.id === 'player').health, 3);
assert.equal(healthBuilder.action(), 1);
assert.equal(healthState.health, 1);
assert.equal(healthBuilder.action(), 1);
assert.equal(healthState.health, 0);
assert.equal(healthBuilder.audit().outcome, 'lost');
healthBuilder.dispose();

const collectState = {};
const collectBuilder = createScene2D({
    nodes: [
        {id: 'player', kind: 'player', position: [5, 5, 0], size: [2, 2, 0], properties: {player: true}},
        {id: 'coin', kind: 'entity', position: [5, 5, 0], size: [2, 2, 0],
            behavior: {type: 'collect', points: 4, win_when_cleared: true}},
    ],
}, simpleHost({}, {state: collectState}));
collectBuilder.step(16);
assert.equal(collectState.score, 4);
assert.equal(collectBuilder.audit().outcome, 'won');
collectBuilder.dispose();

const reachState = {};
const reachBuilder = createScene2D({
    nodes: [
        {id: 'player', kind: 'player', position: [5, 5, 0], size: [2, 2, 0], properties: {player: true}},
        {id: 'reach-a', kind: 'entity', position: [5, 5, 0], size: [2, 2, 0], behavior: {type: 'reach', win_when_cleared: true}},
        {id: 'reach-b', kind: 'entity', position: [5, 5, 0], size: [2, 2, 0], behavior: {type: 'reach', win_when_cleared: true}},
    ],
}, simpleHost({}, {state: reachState}));
reachBuilder.step(16);
assert.equal(reachState.goal_remaining, 0);
assert.equal(reachBuilder.audit().outcome, 'won');
reachBuilder.dispose();

const checkpointState = {};
const checkpointBuilder = createScene2D({
    nodes: [
        {id: 'player', kind: 'player', position: [5, 5, 0], size: [2, 2, 0], properties: {player: true}},
        {id: 'cp1', kind: 'entity', position: [5, 5, 0], size: [2, 2, 0],
            behavior: {type: 'checkpoint', sequence: ['cp1', 'cp2']}},
        {id: 'cp2', kind: 'entity', position: [5, 5, 0], size: [2, 2, 0],
            behavior: {type: 'checkpoint', sequence: ['cp1', 'cp2']}},
    ],
}, simpleHost({}, {state: checkpointState}));
checkpointBuilder.step(16);
assert.equal(checkpointState.checkpoint, 'cp2');
assert.equal(checkpointState.checkpoint_index, 2);
checkpointBuilder.dispose();

const attachmentBuilder = createScene2D({
    nodes: [{id: 'player', kind: 'player', position: [0, 0, 0], size: [2, 2, 0], properties: {player: true}}],
    attachments: [{id: 'muzzle', node_id: 'player', asset_id: 'flash', socket: 'muzzle'}],
}, simpleHost({}, {
    resolveAttachment: (attachment, object, node) => ({id: attachment.id, node_id: node.id, object}),
}));
assert.equal(attachmentBuilder.attachment('muzzle').node_id, 'player');
attachmentBuilder.dispose();

const actualAssetBuilder = createScene2D({
    nodes: [{id: 'actor', kind: 'entity', position: [0, 0, 0], size: [2, 2, 0]}],
}, simpleHost({}, {
    actualAssets: () => [
        {role: 'body', asset_id: 'body-runtime'},
        {role: 'shadow', asset_id: 'shadow-runtime'},
    ],
}));
assert.deepEqual(actualAssetBuilder.audit().roles.map(row => row.asset_id), ['body-runtime', 'shadow-runtime']);
actualAssetBuilder.dispose();

const destroyedDynamic = [];
const disposableBuilder = createScene2D({
    nodes: [{id: 'player', kind: 'player', position: [0, 0, 0], size: [2, 2, 0], properties: {player: true}}],
}, simpleHost({}, {destroyNode: node => destroyedDynamic.push(node.id)}));
const disposableProjectile = disposableBuilder.spawnProjectile({position: [0, 0, 0], behavior: {ttl: 10}});
assert.ok(disposableProjectile);
disposableBuilder.dispose();
assert.ok(destroyedDynamic.some(id => id.startsWith('projectile-')));

console.log('scene builder movement, health, goals, checkpoints, attachments, actual assets and disposal passed');