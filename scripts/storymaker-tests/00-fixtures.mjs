// Task C1: the shared fixtures and the catalog-mini stub files against contract section 3a (rig semantics): the 13 bones
// with a rest transform per view, a slot for every rig parameter, the variants that the mini character and its expressions
// use, props and environments in the vector part format.
import assert from 'node:assert/strict';
import { read, readJSON, MINI } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, catalog, files, firewall, minimal } = t;
const BONES = ['root', 'hips', 'torso', 'neck', 'head', 'upper_arm_l', 'lower_arm_l', 'upper_arm_r', 'lower_arm_r', 'upper_leg_l', 'lower_leg_l', 'upper_leg_r', 'lower_leg_r'];
const PARAMS = ['body', 'skin', 'hair', 'hair_back', 'eyes', 'brows', 'mouth', 'hand', 'top', 'bottom', 'shoes', 'glasses', 'facial_hair', 'accessory'];
const MOUTHS = ['rest', 'ai', 'e', 'o', 'u', 'mbp', 'fv', 'smile', 'frown'];
const VIEWS = ['front', 'three_quarter', 'profile'];

function checkShapes(shapes, where) {
    assert.ok(Array.isArray(shapes), where + ' is a list of shapes');
    for (const s of shapes) {
        assert.equal(typeof s.d, 'string', where + ': a shape has path data');
        assert.ok(s.d.length > 2 && /^[MmLlHhVvCcSsQqTtAaZz0-9 ,.-]+$/.test(s.d), where + ': plain SVG path data');
        for (const c of [s.fill, s.stroke]) if (typeof c === 'string') assert.ok(c === 'none' || /^(#[0-9a-fA-F]{3,8}|\$[a-z_0-9]+)$/.test(c), where + ': colour ' + c);
        if (s.gradient) assert.ok(['linear', 'radial'].includes(s.gradient.type) && s.gradient.stops.length >= 2, where + ': gradient');
    }
}

check('fixtures: the shared stories, the code scene and the catalog manifest are the contract content', () => {
    assert.equal(minimal.title, 'Minimal'); assert.equal(minimal.scenes.length, 1);
    assert.deepEqual(firewall.scenes.map(s => s.id), ['s1', 's2', 's3']);
    assert.equal(firewall.scenes[2].kind, 'code');
    assert.match(read('internal/storymaker/testdata/scenes/s3.js'), /^export default function scene\(api\) \{\r?\n  api\.duration\(5\);/);
    assert.equal(catalog.schema, 'storymaker-catalog/1'); assert.equal(catalog.version, 'mini00000000'); assert.equal(catalog.entries.length, 20);
    assert.deepEqual(catalog.stage, { width: 1920, height: 1080, safe: { x: 5, y: 5, w: 90, h: 90 } });
    for (const e of catalog.entries) if (e.file) assert.ok(typeof readJSON(MINI + e.file) === 'object', e.file + ' exists and parses');
    assert.equal(readJSON(MINI + catalog.rig.file).id, catalog.rig.id);
});

check('catalog-mini rig: 13 bones with a rest transform per view, slots for every param, z order per view', () => {
    const rig = files.rig;
    assert.equal(rig.id, 'rig.human/1');
    assert.ok(rig.units_height > 0 && rig.height > 0 && rig.height < 1);
    assert.deepEqual(rig.bones.map(b => b.name), BONES);
    const names = new Set(BONES);
    for (const b of rig.bones) {
        assert.ok(b.parent === null ? b.name === 'root' : names.has(b.parent), b.name + ' has a known parent');
        for (const v of VIEWS) assert.ok(['x', 'y', 'angle'].every(k => typeof b.rest[v][k] === 'number'), b.name + ' rest.' + v);
    }
    const slotNames = rig.slots.map(s => s.name);
    assert.equal(new Set(slotNames).size, slotNames.length, 'slot names are unique');
    for (const s of rig.slots) {
        assert.ok(names.has(s.bone), s.name + ' sits on a bone');
        assert.ok(PARAMS.includes(s.param), s.name + ' has a known param ' + s.param);
        for (const v of VIEWS) assert.equal(typeof s.z[v], 'number', s.name + ' z.' + v);
    }
    assert.deepEqual([...new Set(rig.slots.map(s => s.param))].sort(), [...PARAMS].sort(), 'a slot for every param');
    assert.ok(['hand_l', 'hand_r'].every(n => slotNames.includes(n)), 'the hands are slots on the lower arms');
    assert.equal(rig.slots.find(s => s.name === 'hand_l').bone, 'lower_arm_l');
    assert.ok(['shoe_l', 'shoe_r'].every(n => slotNames.includes(n)), 'the feet are slots on the lower legs');
});

check('catalog-mini rig: the variants that the mini character and the three expressions use exist', () => {
    const rig = files.rig, ch = catalog.entries.find(e => e.id === 'char.office-woman-1').params, outfit = rig.params.outfit[ch.outfit];
    for (const s of rig.slots) assert.ok(rig.variants[s.name], 'variants for slot ' + s.name);
    const has = (slot, key) => {
        const v = rig.variants[slot][key];
        assert.ok(v, slot + ' has variant ' + key);
        for (const view of VIEWS) checkShapes(v[view], slot + '/' + key + '/' + view);
    };
    has('head', ch.body); has('neck', ch.body); has('hair', ch.hair); has('hair_back', ch.hair);
    for (const skin of Object.keys(rig.params.skin)) has('ears', skin);
    has('torso', outfit.top); has('sleeve_upper_l', outfit.top); has('sleeve_lower_r', outfit.top);
    has('pelvis', outfit.bottom); has('leg_upper_l', outfit.bottom); has('leg_lower_r', outfit.bottom);
    has('shoe_l', outfit.shoes); has('shoe_r', outfit.shoes);
    has('facial_hair', ch.facial_hair); has('glasses', ch.glasses); has('accessory', ch.accessory);
    for (const e of ['neutral', 'wide', 'smile']) has('eyes', ch.eyes + '/' + e);
    for (const e of ['neutral', 'raised']) has('brows', ch.brows + '/' + e);
    for (const m of MOUTHS) has('mouth', m);
    for (const h of ['open', 'fist', 'point', 'hold']) { has('hand_l', h); has('hand_r', h); }
    // every expression of the manifest maps to variants that exist
    for (const e of catalog.entries.filter(x => x.kind === 'expression')) {
        assert.ok(rig.variants.eyes[ch.eyes + '/' + e.eyes] && rig.variants.brows[ch.brows + '/' + e.brows] && rig.variants.mouth[e.mouth], e.id + ' maps to variants');
    }
    assert.deepEqual(rig.variants.facial_hair.none.front, []); assert.deepEqual(rig.variants.glasses.none.front, []);
    assert.ok(rig.params.skin[ch.skin] && rig.params.hair_color[ch.hair_color] && outfit.colors.top && outfit.colors.bottom && outfit.colors.shoes);
    assert.ok(['outline', 'eye_white', 'iris', 'lip'].every(k => rig.params.fixed[k]), 'the fixed colour tokens');
    assert.equal(rig.bodies[ch.body].scale, 1);
    assert.deepEqual(MOUTHS.slice(0, 7), ['rest', 'ai', 'e', 'o', 'u', 'mbp', 'fv'], 'the 7 speech mouth shapes come first');
});

check('catalog-mini props and environments: the vector part format with palette and environment tokens', () => {
    for (const [file, part] of Object.entries(files.props)) {
        assert.equal(part.viewbox.length, 4, file);
        checkShapes(part.shapes, file);
    }
    for (const [file, env] of Object.entries(files.envs)) {
        assert.deepEqual(env.viewbox, [0, 0, 1920, 1080], file);
        assert.ok(env.layers.length >= 3 && env.layers[0].parallax === 0 && env.layers.at(-1).parallax === 1, file + ': sky at parallax 0, floor at 1');
        for (const l of env.layers) { assert.equal(typeof l.name, 'string'); checkShapes(l.shapes, file + '/' + l.name); }
        assert.deepEqual(Object.keys(env.tokens.day).sort(), Object.keys(env.tokens.night).sort(), file + ': day and night tokens match');
        const used = new Set([...JSON.stringify(env.layers).matchAll(/"\$([a-z_0-9]+)"/g)].map(m => m[1]));
        const palette = Object.keys(catalog.palettes[0].tokens);
        for (const name of used) assert.ok(name in env.tokens.day || palette.includes(name), file + ': token $' + name + ' resolves');
    }
    for (const id of ['prop.lock', 'prop.shield']) {
        const e = catalog.entries.find(x => x.id === id);
        assert.ok(files.props[e.file] && e.size.h > 0 && e.anchor.y === 1);
    }
});
