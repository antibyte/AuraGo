#!/usr/bin/env node
// Runs ui/js/desktop/apps/terminal-modem.js (after terminal-text.js) in Node with stubs only: baud storage, the
// per-frame baud throttle (including the 1 MiB queue cap), the ATZ/OK/ATDT transcript with skip, overlapping dials and
// dispose, and the sound rules of the key clicks (retro style only, never muted, never under reduced motion), with the
// AudioContext created synchronously inside dial() and no tones scheduled while the context stays suspended, plus the
// C64 bell of terminal-audio.js (one tone per 150 ms however many BELs, never against a suspended clock).
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
    // The throttle's stall timer; section 2b swaps in fake timers.
    setTimeout: (fn, ms) => setTimeout(fn, ms),
    clearTimeout: (id) => clearTimeout(id),
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

// 2b. Throttle units (Contract C): one per code point outside escape sequences, one per complete escape sequence;
// a frame never releases part of a code point or of a sequence.
{
    const encoder = new TextEncoder();
    const strict = new TextDecoder('utf-8', { fatal: true });
    const equal = (name, actual, expected) => same(name, JSON.stringify(actual), JSON.stringify(expected));
    const join = (chunks) => {
        const out = new Uint8Array(chunks.reduce((sum, chunk) => sum + chunk.length, 0));
        let at = 0;
        chunks.forEach((chunk) => { out.set(chunk, at); at += chunk.length; });
        return out;
    };
    // At 300 baud a frame 1000/30 ms after the previous one allows exactly one unit (the first frame only 0.48).
    let now = 0;
    const tick = () => { now += 1000 / 30 + 0.01; frames.shift()(now); };
    const unitThrottle = () => {
        const out = [];
        const throttle = modem.createThrottle((bytes) => out.push(new Uint8Array(bytes)));
        throttle.setBaud(300);
        frames.length = 0;
        return { out, throttle, text: () => new TextDecoder().decode(join(out)) };
    };
    const glyph = String.fromCharCode(0xef08); // a C64 Pro Mono glyph: three UTF-8 bytes
    const sgr = '\x1b[38;2;112;109;235m';
    {
        const { out, throttle, text } = unitThrottle();
        throttle.push(encoder.encode((sgr + glyph).repeat(20)));
        frames.shift()(1000);
        frames.shift()(2000);
        same('300 baud releases 30 units: 15 colour changes and 15 glyphs', text(), (sgr + glyph).repeat(15));
        frames.shift()(3000);
        same('the next second releases the remaining 10 units', text(), (sgr + glyph).repeat(20));
        same('an empty queue schedules no frame', frames.length, 0);
        throttle.dispose();
        check('byte counting would have released far less', out.length > 0);
    }
    {
        const { throttle, text } = unitThrottle();
        throttle.push(encoder.encode(glyph).subarray(0, 1));
        frames.shift()(1000);
        frames.shift()(2000);
        equal('an unfinished code point waits for its rest', [text(), frames.length], ['', 0]);
        throttle.push(encoder.encode(glyph).subarray(1));
        throttle.push(encoder.encode('\x1b[3'));
        throttle.push(encoder.encode('1mA'));
        frames.shift()(3000);
        frames.shift()(4000);
        same('split code points and sequences are released whole', text(), glyph + '\x1b[31mA');
        throttle.dispose();
    }
    {
        const sequences = ['\x1b]0;' + 'title '.repeat(40) + '\x07', '\x1b[2J', '\x1bP1$r0m\x1b\\', '\x1bOA', '\x1b(B', '\x1b7', 'Z', glyph, '\x07', '\r', '\n'];
        const { out, throttle } = unitThrottle();
        throttle.push(encoder.encode(sequences.join('')));
        tick();
        const released = [];
        for (let i = 0; i < sequences.length; i += 1) {
            const mark = out.length;
            tick();
            released.push(new TextDecoder().decode(join(out.slice(mark))));
        }
        equal('OSC, CSI, DCS with ST, SS3, charset, ESC 7, ASCII, a glyph and controls are one unit each', released, sequences);
        throttle.dispose();
    }
    {
        const { out, throttle } = unitThrottle();
        throttle.push(encoder.encode('\x1b]' + 'y'.repeat(2000)));
        tick();
        tick();
        same('an escape still open after 1024 bytes counts as one unit there', join(out).length, 1024);
        tick();
        same('its rest counts as text', join(out).length, 1025);
        throttle.dispose();
    }
    {
        const { out, throttle } = unitThrottle();
        throttle.push(new Uint8Array([0x80, 0xbf, 0x41]));
        tick();
        tick();
        equal('stray continuation bytes are one unit each', [...join(out)], [0x80]);
        throttle.dispose();
    }
    {
        const { out, throttle } = unitThrottle();
        throttle.setBaud(1200);
        const parts = [];
        for (let i = 0; i < 300; i += 1) parts.push(i % 7 === 0 ? '\x1b[' + (i % 40) + ';' + (i % 25) + 'H' : i % 5 === 0 ? sgr : i % 3 === 0 ? '\xe9' : glyph);
        const source = encoder.encode(parts.join(''));
        for (let at = 0; at < source.length; at += 7) throttle.push(source.subarray(at, at + 7));
        const broken = [];
        for (let i = 0; i < 400 && frames.length; i += 1) {
            const mark = out.length;
            frames.shift()(1000 + i * 17);
            const chunk = join(out.slice(mark));
            try {
                if (/\x1b(\[[0-?]*[ -/]*)?$/.test(strict.decode(chunk))) broken.push('frame ' + i + ' ends inside an escape');
            } catch (e) {
                broken.push('frame ' + i + ' splits a code point');
            }
        }
        equal('every frame releases whole units', broken, []);
        same('everything arrives byte for byte', [...join(out)].join(','), [...source].join(','));
        throttle.dispose();
    }
    // Synchronized output (the PETSCII engine's repaints): ESC[?2026h ... ESC[?2026l is one unit and leaves at once.
    const BEGIN = '\x1b[?2026h';
    const END = '\x1b[?2026l';
    const repaint = BEGIN + (sgr + glyph).repeat(200) + '\x1b[?2026h\x1b[?25h' + END;
    {
        const { out, throttle } = unitThrottle();
        throttle.push(encoder.encode('A' + repaint + 'B' + END));
        tick();
        const released = [];
        for (let i = 0; i < 4; i += 1) {
            const mark = out.length;
            tick();
            released.push(new TextDecoder().decode(join(out.slice(mark))));
        }
        equal('a synchronized block (nested begin and ?25h inside) is one unit; a lone end is an ordinary unit', released, ['A', repaint, 'B', END]);
        throttle.dispose();
    }
    {
        const { out, throttle } = unitThrottle();
        throttle.push(encoder.encode(BEGIN + 'xyz' + glyph));
        throttle.push(encoder.encode('\x1b[?20'));
        tick();
        tick();
        equal('an open block waits without spinning frames', [join(out).length, frames.length], [0, 0]);
        throttle.push(encoder.encode('26l' + 'C'));
        tick();
        tick();
        same('a block split across pushes leaves whole once its end arrives', new TextDecoder().decode(join(out)), BEGIN + 'xyz' + glyph + END);
        tick();
        same('the next unit follows on the next allowance', new TextDecoder().decode(join(out)), BEGIN + 'xyz' + glyph + END + 'C');
        throttle.dispose();
    }
    {
        const { out, throttle } = unitThrottle();
        throttle.push(encoder.encode(BEGIN + 'q'.repeat(100)));
        tick();
        tick();
        same('an unterminated block is held', join(out).length, 0);
        throttle.setBaud(0);
        same('switching the throttle off releases it', new TextDecoder().decode(join(out)), BEGIN + 'q'.repeat(100));
        throttle.dispose();
    }
    {
        let total = 0;
        const throttle = modem.createThrottle((bytes) => { total += bytes.length; });
        throttle.setBaud(300);
        throttle.push(encoder.encode(BEGIN));
        throttle.push(new Uint8Array(modem.MAX_QUEUE_BYTES));
        same('an unterminated block beyond the queue limit is released', total, BEGIN.length + modem.MAX_QUEUE_BYTES);
        throttle.dispose();
    }
    // An unterminated block is never held for long: it ends as one unit at 64 KiB, or 1 s after the throttle stalled
    // on it (up to its last complete code point or sequence), and every byte is scanned once.
    const SYNC_LIMIT = 64 * 1024;
    {
        const { out, throttle } = unitThrottle();
        throttle.push(encoder.encode(BEGIN + 'q'.repeat(70 * 1024)));
        tick();
        tick();
        same('an unterminated block of 70 KiB leaves at 64 KiB as one unit', join(out).length, BEGIN.length + SYNC_LIMIT);
        tick();
        same('its rest counts as ordinary units', join(out).length, BEGIN.length + SYNC_LIMIT + 1);
        throttle.dispose();
    }
    const realTimers = [window.setTimeout, window.clearTimeout];
    const timers = new Map();
    let timerSeq = 0;
    window.setTimeout = (fn, ms) => { timerSeq += 1; timers.set(timerSeq, { fn, ms }); return timerSeq; };
    window.clearTimeout = (id) => { timers.delete(id); };
    const fire = () => {
        const due = [...timers.entries()];
        timers.clear();
        due.forEach(([, timer]) => timer.fn());
        return due.map(([, timer]) => timer.ms);
    };
    {
        const { out, throttle, text } = unitThrottle();
        throttle.push(encoder.encode(BEGIN + 'xyz' + glyph + '\x1b[3'));
        tick();
        tick();
        equal('a small open block stalls the throttle without frames and arms one 1 s timer', [join(out).length, frames.length, [...timers.values()].map((timer) => timer.ms)], [0, 0, [1000]]);
        throttle.push(encoder.encode(';4'));
        tick();
        tick();
        same('more bytes of the open block keep the same timer', timers.size, 1);
        same('when the timer fires, the block leaves up to its last complete character', fire().length === 1 && text(), BEGIN + 'xyz' + glyph);
        throttle.push(encoder.encode('1mA'));
        for (let i = 0; i < 20 && frames.length; i += 1) tick();
        same('the unfinished sequence then continues outside the block', text(), BEGIN + 'xyz' + glyph + '\x1b[3;41mA');
        same('and no timer is left', timers.size, 0);
        throttle.dispose();
    }
    {
        const { throttle, text } = unitThrottle();
        throttle.push(encoder.encode(BEGIN + 'a'));
        tick();
        tick();
        throttle.push(encoder.encode(END + 'b'));
        tick();
        tick();
        equal('a block that ends in time clears its timer', [text(), timers.size], [BEGIN + 'a' + END, 0]);
        throttle.push(encoder.encode(BEGIN));
        tick();
        tick();
        same('a later open block arms a new timer', timers.size, 1);
        throttle.flush();
        same('flush clears it', timers.size, 0);
        throttle.push(encoder.encode(BEGIN));
        tick();
        tick();
        throttle.setBaud(0);
        same('switching the throttle off clears it', timers.size, 0);
        throttle.setBaud(300);
        throttle.push(encoder.encode(BEGIN));
        tick();
        tick();
        throttle.dispose();
        same('dispose clears it', timers.size, 0);
    }
    [window.setTimeout, window.clearTimeout] = realTimers;
    {
        // 1 MiB of a repaint that never ends, in 4 KiB pushes with a frame after each: the old byte-for-byte rescan of
        // the open block on every push took seconds; scanning on from the last position is linear.
        const { out, throttle } = unitThrottle();
        const chunk = encoder.encode((sgr + glyph).repeat(186) + 'qqqq');
        const started = Date.now();
        throttle.push(encoder.encode(BEGIN));
        let firstRelease = -1;
        for (let i = 0; i < 256; i += 1) {
            throttle.push(chunk);
            if (frames.length) tick();
            if (firstRelease < 0 && out.length) firstRelease = i;
        }
        const elapsed = Date.now() - started;
        check('1 MiB of an open block in 4 KiB pushes is scanned in linear time', elapsed < 2000, elapsed + ' ms');
        check('the open block leaves after 64 KiB, not 1 MiB', firstRelease >= 0 && firstRelease <= 17, String(firstRelease));
        throttle.setBaud(0);
        same('everything arrives', join(out).length, BEGIN.length + 256 * chunk.length);
        throttle.dispose();
    }
    {
        // Every byte is scanned once: count the scanner's reads of 1000 pushes of 60 bytes into one open block.
        let reads = 0;
        const counted = (bytes) => new Proxy(bytes, {
            get(target, prop) {
                if (typeof prop === 'string' && /^[0-9]+$/.test(prop)) reads += 1;
                const value = Reflect.get(target, prop, target);
                return typeof value === 'function' ? value.bind(target) : value;
            }
        });
        let total = 0;
        const throttle = modem.createThrottle((bytes) => { total += bytes.length; });
        throttle.setBaud(300);
        frames.length = 0;
        throttle.push(counted(encoder.encode(BEGIN)));
        const piece = encoder.encode('w'.repeat(60));
        for (let i = 0; i < 1000; i += 1) {
            throttle.push(counted(piece.slice()));
            if (frames.length) tick();
        }
        throttle.push(counted(encoder.encode(END)));
        for (let i = 0; i < 4 && frames.length; i += 1) tick();
        const bytes = BEGIN.length + 60000 + END.length;
        check('every byte of an open block is scanned once, however often it grows', total === bytes && reads <= bytes, total + ' bytes released, ' + reads + ' reads of ' + bytes + ' bytes');
        throttle.dispose();
    }
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

// 5. The C64 bell (terminal-audio.js beep): the key-click rules, at most one tone per 150 ms however many BELs
// arrive, and never a tone queued against a suspended clock.
{
    let clock = 1000;
    const bellLog = { started: 0, resumed: 0 };
    const bellWindow = {
        localStorage: { getItem: () => '0', setItem() {} },
        matchMedia: () => ({ matches: false }),
        performance: { now: () => clock }
    };
    const bellDocument = { body: { dataset: { animations: 'true' } } };
    let contextState = 'running';
    let resumeTo = 'running';
    bellWindow.AudioContext = function () {
        this.state = contextState;
        this.currentTime = 0;
        this.destination = {};
        const param = () => ({ value: 0, setTargetAtTime() {} });
        const node = () => ({ connect() {}, type: '', frequency: param(), gain: param(), start() { bellLog.started += 1; }, stop() {} });
        this.createOscillator = node;
        this.createGain = node;
        this.resume = () => { bellLog.resumed += 1; this.state = resumeTo; return Promise.resolve(); };
        this.close = () => Promise.resolve();
    };
    const bellContext = vm.createContext({ window: bellWindow, document: bellDocument, Math });
    vm.runInContext(fs.readFileSync(path.join(apps, 'terminal-audio.js'), 'utf8'), bellContext, { filename: 'terminal-audio.js' });
    const Audio = bellWindow.TerminalAudio;
    const c64 = { id: 'commodore64', retro: true };
    const bell = Audio.create();
    bell.setProfile(c64);
    for (let i = 0; i < 10; i += 1) {
        bell.beep();
        clock += 1;
    }
    same('a burst of 10 BELs plays one tone', bellLog.started, 1);
    clock += 50;
    bell.beep();
    same('a BEL within 150 ms of the last tone is ignored', bellLog.started, 1);
    clock += 160;
    bell.beep();
    same('a BEL 160 ms after the last tone plays again', bellLog.started, 2);
    bell.dispose();
    clock += 1000;
    contextState = 'suspended';
    resumeTo = 'suspended';
    const frozen = Audio.create();
    frozen.setProfile(c64);
    frozen.beep();
    clock += 200;
    frozen.beep();
    check('no bell is queued against a suspended clock, which is asked to resume', bellLog.started === 2 && bellLog.resumed >= 1, JSON.stringify(bellLog));
    frozen.dispose();
    clock += 1000;
    resumeTo = 'running';
    const waking = Audio.create();
    waking.setProfile(c64);
    waking.beep();
    same('a context that resumes at once plays the bell', bellLog.started, 3);
    waking.dispose();
    contextState = 'running';
    clock += 1000;
    const quiet = Audio.create();
    quiet.setProfile({ id: 'modern', retro: false });
    quiet.beep();
    quiet.setProfile(c64);
    quiet.setMuted(true);
    quiet.beep();
    quiet.setMuted(false);
    bellWindow.matchMedia = () => ({ matches: true });
    quiet.beep();
    bellWindow.matchMedia = () => ({ matches: false });
    bellDocument.body.dataset.animations = 'false';
    quiet.beep();
    same('the bell follows the key-click rules: retro style, not muted, no reduced motion, animations on', bellLog.started, 3);
    bellDocument.body.dataset.animations = 'true';
    quiet.beep();
    same('and plays once they allow it', bellLog.started, 4);
    quiet.dispose();
}

if (failures) {
    console.log(failures + ' check(s) failed');
    process.exit(1);
}
console.log('terminal modem ok');
