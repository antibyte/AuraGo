#!/usr/bin/env node
// Runs ui/js/desktop/apps/terminal-modem.js (after terminal-text.js) in Node with stubs only: baud storage, the
// per-frame baud throttle (including the 1 MiB queue cap), the ATZ/OK/ATDT transcript with skip, overlapping dials and
// dispose, and the sound rules of the key clicks (retro style only, never muted, never under reduced motion), with the
// AudioContext created synchronously inside dial() and no tones scheduled while the context stays suspended.
// Usage: node scripts/test-terminal-modem.mjs
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const apps = path.join(here, '..', 'ui', 'js', 'desktop', 'apps');

const store = new Map();
let storageThrows = false;
const frames = [];
const window = {
    localStorage: {
        getItem: (key) => { if (storageThrows) throw new Error('blocked'); return store.has(key) ? store.get(key) : null; },
        setItem: (key, value) => { if (storageThrows) throw new Error('blocked'); store.set(key, String(value)); }
    },
    requestAnimationFrame: (fn) => { frames.push(fn); return frames.length; },
    cancelAnimationFrame: () => {},
    matchMedia: () => ({ matches: true })
};
const document = { body: { dataset: { animations: 'false' } } };
const context = vm.createContext({ window, document, setTimeout, clearTimeout, Promise, Math, Number, String, Uint8Array, Float32Array, Error });
for (const file of ['terminal-text.js', 'terminal-modem.js']) {
    vm.runInContext(fs.readFileSync(path.join(apps, file), 'utf8'), context, { filename: file });
}
const modem = window.TerminalModem;

let failures = 0;
function check(name, cond, detail) {
    if (cond) {
        console.log('ok   ' + name);
        return;
    }
    failures += 1;
    console.log('FAIL ' + name + (detail ? ' - ' + detail : ''));
}
const same = (name, actual, expected) => check(name, actual === expected, JSON.stringify(actual) + ' !== ' + JSON.stringify(expected));
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const screen = () => ({ out: '', write(text) { this.out += text; } });
function motion(allowed) {
    document.body.dataset.animations = allowed ? 'true' : 'false';
    window.matchMedia = () => ({ matches: !allowed });
}

// 1. Baud storage.
same('baud rates', [...modem.BAUD_RATES].join(','), '0,300,1200,2400,9600,14400');
same('default baud is off', modem.loadBaud(), 0);
same('save normalizes a known rate', modem.saveBaud('2400'), 2400);
same('saved rate loads', modem.loadBaud(), 2400);
same('unknown rates normalize to off', modem.saveBaud('1234'), 0);
storageThrows = true;
same('blocked storage loads off', modem.loadBaud(), 0);
same('blocked storage still normalizes', modem.saveBaud(9600), 9600);
storageThrows = false;

// 2. Throttle: baud/10 bytes per second per animation frame, flush on off, 1 MiB queue cap.
{
    const written = [];
    const throttle = modem.createThrottle((bytes) => written.push(...bytes));
    throttle.push(new Uint8Array([1, 2, 3]));
    same('baud 0 writes immediately', written.length, 3);
    throttle.setBaud(300);
    throttle.push(new Uint8Array(100));
    same('output is queued at 300 baud', written.length, 3);
    frames.shift()(1000);
    frames.shift()(2000);
    same('300 baud releases 30 bytes per second', written.length, 33);
    throttle.setBaud(0);
    same('switching off flushes the queue', written.length, 103);
    throttle.dispose();
    throttle.push(new Uint8Array(5));
    same('a disposed throttle drops output', written.length, 103);
    frames.length = 0;
}
{
    let total = 0;
    const throttle = modem.createThrottle((bytes) => { total += bytes.length; });
    throttle.setBaud(300);
    throttle.push(new Uint8Array(1 << 20));
    same('up to 1 MiB stays queued', total, 0);
    throttle.push(new Uint8Array(1));
    same('more than 1 MiB queued flushes everything', total, (1 << 20) + 1);
    throttle.dispose();
    frames.length = 0;
}

// 3. Transcript: reduced motion, typing + skip, overlapping dials, dispose during a dial.
{
    motion(false);
    const term = screen();
    const unit = modem.create({ term, getProfile: () => ({ retro: true }), isMuted: () => false });
    same('reduced motion finishes at once', await unit.dial('telehack.com', 'telehack'), 'done');
    same('transcript', term.out, 'ATZ\r\nOK\r\nATDT telehack.com\r\n');
    unit.connectLine(0);
    unit.connectLine(2400);
    check('connect lines', term.out.endsWith('CONNECT 14400\r\nCONNECT 2400\r\n'));
    unit.dispose();
}
{
    motion(true);
    const term = screen();
    const unit = modem.create({ term, getProfile: () => ({ retro: false }), isMuted: () => true });
    const pending = unit.dial('vert.synchro.net\u001b[2J', 'vertrauen');
    await sleep(60);
    check('typing in progress', term.out.length > 0 && term.out.length < 'ATZ\r\nOK\r\n'.length, JSON.stringify(term.out));
    unit.skip();
    same('skip resolves skipped', await pending, 'skipped');
    same('skip prints the rest; control characters are stripped', term.out, 'ATZ\r\nOK\r\nATDT vert.synchro.net[2J\r\n');
    unit.dispose();
}
{
    motion(true);
    const term = screen();
    const unit = modem.create({ term, getProfile: () => ({ retro: false }), isMuted: () => true });
    const first = unit.dial('first.example', 'a');
    await sleep(60);
    const typed = term.out;
    const second = unit.dial('second.example', 'b');
    same('an overlapping dial ends the first as skipped', await first, 'skipped');
    unit.skip();
    same('the second dial is skipped on request', await second, 'skipped');
    same('the first dial writes nothing more', term.out, typed + 'ATZ\r\nOK\r\nATDT second.example\r\n');
    unit.dispose();
}
{
    motion(true);
    const term = screen();
    const unit = modem.create({ term, getProfile: () => ({ retro: false }), isMuted: () => true });
    const pending = unit.dial('gone.example', 'gone');
    await sleep(60);
    unit.dispose();
    same('dispose during a dial resolves skipped', await pending, 'skipped');
    const length = term.out.length;
    await sleep(150);
    unit.connectLine(0);
    same('nothing is written after dispose', term.out.length, length);
    same('a dial after dispose is skipped at once', await unit.dial('again.example', 'again'), 'skipped');
}

// 4. Sound rules: the key-click rules, a context created inside the gesture, no tones against a suspended clock.
let audioLog = null;
function installAudio(options = {}) {
    const initial = options.state || 'running';
    const resumes = options.resumes !== false;
    audioLog = { contexts: 0, started: 0, stopped: 0, resumed: 0 };
    const node = () => ({
        connect() {}, disconnect() {},
        gain: { setValueAtTime() {}, linearRampToValueAtTime() {} },
        frequency: { value: 0 }, Q: { value: 0 },
        start() { audioLog.started += 1; },
        stop(at) { if (at === 0) audioLog.stopped += 1; }
    });
    window.AudioContext = function () {
        audioLog.contexts += 1;
        this.state = initial;
        this.currentTime = 0;
        this.sampleRate = 8000;
        this.destination = {};
        this.createGain = node;
        this.createOscillator = node;
        this.createBiquadFilter = node;
        this.createBufferSource = node;
        this.createBuffer = () => ({ getChannelData: () => new Float32Array(8000) });
        this.resume = () => { audioLog.resumed += 1; if (resumes) this.state = 'running'; return Promise.resolve(); };
        this.close = () => Promise.resolve();
    };
}
async function dialSounds(profile, muted, setup, expected = 'skipped') {
    motion(true);
    if (setup) setup();
    const unit = modem.create({ term: screen(), getProfile: () => profile, isMuted: () => muted });
    const result = unit.dial('a.b', 'x');
    const createdInGesture = audioLog.contexts;
    await sleep(1400);
    unit.skip();
    const resolved = await result;
    unit.dispose();
    return { ...audioLog, createdInGesture, resolved };
}
{
    installAudio();
    const loud = await dialSounds({ retro: true }, false);
    same('retro + unmuted creates the AudioContext synchronously in dial()', loud.createdInGesture, 1);
    check('retro + unmuted plays DTMF digits', loud.contexts === 1 && loud.started >= 2, JSON.stringify(loud));
    check('skip stops scheduled tones', loud.stopped >= 1, JSON.stringify(loud));
    installAudio();
    const muted = await dialSounds({ retro: true }, true);
    check('muted is silent and creates no context', muted.contexts === 0 && muted.started === 0, JSON.stringify(muted));
    installAudio();
    const modern = await dialSounds({ retro: false }, false);
    check('modern style is silent', modern.contexts === 0 && modern.started === 0, JSON.stringify(modern));
    installAudio();
    window.TerminalAudio = { shouldSilence: () => true };
    const silenced = await dialSounds({ retro: true }, false);
    delete window.TerminalAudio;
    check('TerminalAudio.shouldSilence wins', silenced.contexts === 0 && silenced.started === 0, JSON.stringify(silenced));
    installAudio();
    const reduced = await dialSounds({ retro: true }, false, () => { window.matchMedia = () => ({ matches: true }); }, 'done');
    check('reduced motion is silent and finishes at once', reduced.resolved === 'done' && reduced.contexts === 0 && reduced.started === 0, JSON.stringify(reduced));
    installAudio({ state: 'suspended' });
    const resumed = await dialSounds({ retro: true }, false);
    check('a suspended context is resumed and then plays', resumed.createdInGesture === 1 && resumed.resumed >= 1 && resumed.started >= 2, JSON.stringify(resumed));
    installAudio({ state: 'suspended', resumes: false });
    const frozen = await dialSounds({ retro: true }, false);
    check('no tones are scheduled while the context stays suspended', frozen.resumed >= 2 && frozen.started === 0, JSON.stringify(frozen));
    delete window.AudioContext;
}

if (failures) {
    console.log(failures + ' check(s) failed');
    process.exit(1);
}
console.log('terminal modem ok');
