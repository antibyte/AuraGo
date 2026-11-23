import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(__dirname, '..');

// Fake browser bits: a pad list the tests rewrite and a document with an active element.
const pads = [];
const sandbox = { console, Math, Number, String, Boolean, Array, Object, Map, Set, JSON, Date, Uint8Array, Promise };
sandbox.window = sandbox;
sandbox.globalThis = sandbox;
sandbox.navigator = { getGamepads: function () { return pads; } };
sandbox.document = { activeElement: null };
vm.createContext(sandbox);

// Profile needs the level tables; same order as module-loader.js up to levels.
const MODULES = ['biomes', 'fx', 'physics', 'behaviors', 'bosses', 'variety', 'profile', 'runes', 'zones', 'zones-extra', 'gauntlets', 'swing', 'puzzles', 'dragons',
    'npcs', 'npc-story', 'npc-story-late', 'dialogue', 'trade', 'duel-patterns', 'duel', 'place-rooms', 'interior-mobs', 'interior', 'places', 'folk', 'folk-story', 'village-rooms', 'village',
    'cutscenes', 'levels', 'gamepad'];
for (const m of MODULES) {
    const abs = path.join(rootDir, `ui/js/desktop/apps/runenpfad/${m}.js`);
    if (fs.existsSync(abs)) vm.runInContext(fs.readFileSync(abs, 'utf8'), sandbox, { filename: abs });
}
const Profile = sandbox.RunenpfadProfile;
const Gamepad = sandbox.RunenpfadGamepad;
assert.ok(Profile && Gamepad, 'modules loaded');

let passed = 0;
function check(name, fn) {
    try {
        fn();
        passed++;
        console.log(`PASS ${name}`);
    } catch (err) {
        console.error(`FAIL ${name}`);
        console.error(err && err.stack ? err.stack : err);
        process.exitCode = 1;
    }
}

function makePad(extra) {
    const buttons = [];
    for (let i = 0; i < 17; i++) buttons.push({ pressed: false, value: 0 });
    return Object.assign({ connected: true, mapping: 'standard', axes: [0, 0, 0, 0], buttons: buttons }, extra || {});
}
function setButton(pad, i, on) { pad.buttons[i].pressed = on; pad.buttons[i].value = on ? 1 : 0; }

function rig(opts) {
    const log = [];
    const gp = Gamepad.create(Object.assign({
        root: {}, press: function (c) { log.push('+' + c); }, release: function (c) { log.push('-' + c); },
        activeDialog: function () { return null; }, cardOpen: function () { return false; }, reduced: function () { return false; }
    }, opts || {}));
    return { gp: gp, log: log };
}

check('no pad: poll makes no calls and the pad stays unseen', () => {
    pads.length = 0;
    pads.push(null);
    const { gp, log } = rig();
    gp.poll(); gp.poll();
    assert.deepEqual(log, []);
    assert.equal(gp.seen(), false);
});

check('a non-standard pad is ignored', () => {
    pads.length = 0;
    const pad = makePad({ mapping: '' });
    setButton(pad, 0, true);
    pads.push(pad);
    const { gp, log } = rig();
    gp.poll();
    assert.deepEqual(log, []);
});

check('button A: exactly one press and one release', () => {
    pads.length = 0;
    const pad = makePad();
    pads.push(pad);
    const { gp, log } = rig();
    gp.poll();
    assert.deepEqual(log, []);
    assert.equal(gp.seen(), true);
    setButton(pad, 0, true);
    gp.poll(); gp.poll(); gp.poll();
    assert.deepEqual(log, ['+Space']);
    setButton(pad, 0, false);
    gp.poll(); gp.poll();
    assert.deepEqual(log, ['+Space', '-Space']);
});

check('button map: X J, B K, Y Q, LB and RB E, Start P', () => {
    pads.length = 0;
    const pad = makePad();
    pads.push(pad);
    const { gp, log } = rig();
    [[2, 'KeyJ'], [1, 'KeyK'], [3, 'KeyQ'], [4, 'KeyE'], [5, 'KeyE'], [9, 'KeyP']].forEach(([i, code]) => {
        log.length = 0;
        setButton(pad, i, true); gp.poll();
        setButton(pad, i, false); gp.poll();
        assert.deepEqual(log, ['+' + code, '-' + code], `button ${i}`);
    });
});

check('stick left: one press, one release; inside the dead zone nothing', () => {
    pads.length = 0;
    const pad = makePad();
    pads.push(pad);
    const { gp, log } = rig();
    pad.axes = [-0.3, 0, 0, 0]; gp.poll();
    assert.deepEqual(log, []);
    pad.axes = [-0.8, 0, 0, 0]; gp.poll(); gp.poll();
    assert.deepEqual(log, ['+ArrowLeft']);
    pad.axes = [0, 0, 0, 0]; gp.poll();
    assert.deepEqual(log, ['+ArrowLeft', '-ArrowLeft']);
});

check('weak diagonal push does not produce Up; a clear push does', () => {
    pads.length = 0;
    const pad = makePad();
    pads.push(pad);
    const { gp, log } = rig();
    pad.axes = [0.7, -0.6, 0, 0]; gp.poll(); // y below 0.7
    pad.axes = [0.85, -0.75, 0, 0]; gp.poll(); // y beats 0.7 but x dominates
    assert.ok(!log.includes('+ArrowUp'), log.join(','));
    assert.ok(log.includes('+ArrowRight'));
    pad.axes = [0, 0, 0, 0]; gp.poll();
    log.length = 0;
    pad.axes = [0.1, -0.9, 0, 0]; gp.poll();
    assert.deepEqual(log, ['+ArrowUp']);
    pad.axes = [0.1, 0.9, 0, 0]; gp.poll();
    assert.deepEqual(log, ['+ArrowUp', '-ArrowUp', '+ArrowDown']);
});

check('D-pad up is Up', () => {
    pads.length = 0;
    const pad = makePad();
    pads.push(pad);
    const { gp, log } = rig();
    setButton(pad, 12, true); gp.poll();
    assert.deepEqual(log, ['+ArrowUp']);
});

check('unplugging releases held keys', () => {
    pads.length = 0;
    const pad = makePad();
    pads.push(pad);
    const { gp, log } = rig();
    setButton(pad, 0, true); gp.poll();
    pads[0] = null; gp.poll();
    assert.deepEqual(log, ['+Space', '-Space']);
});

check('open dialog: D-pad walks focus, A clicks, B is Escape, no game keys', () => {
    pads.length = 0;
    const pad = makePad();
    pads.push(pad);
    const clicks = [];
    const mk = (name, extra) => {
        const n = Object.assign({ name: name, hidden: false, closest: () => null, focus() { sandbox.document.activeElement = n; }, click() { clicks.push(name); } }, extra || {});
        return n;
    };
    const nodes = [mk('resume'), mk('restart'), mk('hidden', { hidden: true }), mk('sound')];
    const dialog = { querySelectorAll: () => nodes };
    sandbox.document.activeElement = null;
    const { gp, log } = rig({ activeDialog: () => dialog });
    setButton(pad, 15, true); gp.poll(); // right -> first
    assert.equal(sandbox.document.activeElement.name, 'resume');
    setButton(pad, 15, false); gp.poll();
    setButton(pad, 13, true); gp.poll(); // down -> restart
    assert.equal(sandbox.document.activeElement.name, 'restart');
    setButton(pad, 13, false); gp.poll();
    setButton(pad, 13, true); gp.poll(); // down -> sound (hidden skipped)
    assert.equal(sandbox.document.activeElement.name, 'sound');
    setButton(pad, 13, false); gp.poll();
    setButton(pad, 12, true); gp.poll(); // up -> restart
    assert.equal(sandbox.document.activeElement.name, 'restart');
    setButton(pad, 12, false); gp.poll();
    setButton(pad, 0, true); gp.poll(); gp.poll();
    assert.deepEqual(clicks, ['restart']);
    setButton(pad, 0, false); gp.poll();
    setButton(pad, 1, true); gp.poll();
    setButton(pad, 1, false); gp.poll();
    assert.deepEqual(log, ['+Escape', '-Escape']);
});

check('a range input moves by its step with the D-pad and fires input and change', () => {
    pads.length = 0;
    const pad = makePad();
    pads.push(pad);
    const seen = [];
    sandbox.Event = function (type) { this.type = type; };
    const slider = { type: 'range', min: '0', max: '100', step: '5', value: '50', hidden: false, closest: () => null, focus() { sandbox.document.activeElement = slider; }, click() {}, dispatchEvent(ev) { seen.push(ev.type); } };
    const dialog = { querySelectorAll: () => [slider] };
    slider.focus();
    const { gp } = rig({ activeDialog: () => dialog });
    setButton(pad, 15, true); gp.poll();
    assert.equal(slider.value, '55');
    setButton(pad, 15, false); gp.poll();
    setButton(pad, 14, true); gp.poll();
    assert.equal(slider.value, '50');
    assert.deepEqual(seen, ['input', 'change', 'input', 'change']);
});

check('a key held while a dialog closes does not jump', () => {
    pads.length = 0;
    const pad = makePad();
    pads.push(pad);
    let dialog = {};
    dialog.querySelectorAll = () => [];
    let open = true;
    const { gp, log } = rig({ activeDialog: () => (open ? dialog : null) });
    setButton(pad, 0, true); gp.poll();
    open = false; gp.poll(); gp.poll();
    assert.deepEqual(log, []);
    setButton(pad, 0, false); gp.poll();
    setButton(pad, 0, true); gp.poll();
    assert.deepEqual(log, ['+Space']);
});

check('round card: B sends Escape', () => {
    pads.length = 0;
    const pad = makePad();
    pads.push(pad);
    const { gp, log } = rig({ cardOpen: () => true });
    setButton(pad, 1, true); gp.poll();
    assert.ok(log.includes('+Escape') && log.includes('-Escape'), log.join(','));
});

check('rumble: hurt and slam play dual-rumble, reduced motion and no actuator stay silent', () => {
    pads.length = 0;
    const effects = [];
    const pad = makePad({ vibrationActuator: { playEffect: (kind, p) => { effects.push([kind, p]); return Promise.reject(new Error('no')); } } });
    pads.push(pad);
    let calm = false;
    const { gp } = rig({ reduced: () => calm });
    gp.poll();
    gp.onEvents([{ type: 'jump' }]);
    assert.equal(effects.length, 0);
    gp.onEvents([{ type: 'player_hurt' }]);
    assert.equal(effects.length, 1);
    assert.equal(effects[0][0], 'dual-rumble');
    assert.equal(effects[0][1].duration, 140);
    assert.equal(effects[0][1].strongMagnitude, 0.6);
    gp.onEvents([{ type: 'slam_land' }]);
    assert.equal(effects[1][1].duration, 180);
    calm = true;
    gp.onEvents([{ type: 'boss_slam' }]);
    assert.equal(effects.length, 2);
    pads[0] = makePad();
    calm = false;
    gp.poll();
    gp.onEvents([{ type: 'player_hurt' }]); // no actuator: no throw
});

check('profile: defaults, clamping and rounding of both volumes', () => {
    const def = Profile.createDefaultProfile();
    assert.equal(def.sfxVol, 1);
    assert.equal(def.musicVol, 1);
    const base = Profile.exportProfile(def);
    const imp = (extra) => Profile.importProfile(Object.assign({}, base, extra));
    assert.equal(imp({}).sfxVol, 1);
    assert.equal(imp({ sfxVol: 0.5, musicVol: 0 }).sfxVol, 0.5);
    assert.equal(imp({ sfxVol: 0.5, musicVol: 0 }).musicVol, 0);
    assert.equal(imp({ sfxVol: 7 }).sfxVol, 1);
    assert.equal(imp({ sfxVol: -3 }).sfxVol, 0);
    assert.equal(imp({ sfxVol: 0.52 }).sfxVol, 0.5);
    assert.equal(imp({ sfxVol: 0.58 }).sfxVol, 0.6);
    ['x', null, NaN, Infinity, {}, [], '0.2', true].forEach((bad) => {
        assert.equal(imp({ sfxVol: bad }).sfxVol, 1, String(bad));
        assert.equal(imp({ musicVol: bad }).musicVol, 1, String(bad));
    });
});

check('profile: volumes survive export and the save key stays', () => {
    const p = Profile.createDefaultProfile();
    p.sfxVol = 0.35;
    p.musicVol = 0.8;
    const out = Profile.exportProfile(p);
    assert.equal(out.sfxVol, 0.35);
    assert.equal(out.musicVol, 0.8);
    assert.equal(out.v, 1);
    assert.equal(Profile.STORAGE_KEY, 'aurago.runenpfad.v1');
});

console.log(`Runenpfad shell tests: ${passed} checks passed`);
