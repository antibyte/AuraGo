#!/usr/bin/env node
// Runs the pure parts of the CYD Creator flash modules (plan 5) in Node: Web Serial support detection and
// error classification, the single-owner connection manager with a fake port (reader loop, DTR/RTS reset,
// owner handoff flasher → monitor), the monitor's ring buffer, line splitter, backtrace parser and attach
// cap, and the flasher's baud fallback, chip check, SHA-256 image check, MD5 and runFlash with a fake esptool.
// Coordinator decisions: D1 install modes (keep settings / full install / automatic by board record) and
// D2 desktop serial policy gating (virtual_desktop.serial_browser_enabled, read-only desktop), plus a
// flash aborted because its window closed. Devices/transfer: the rollback hint of a stored last_error and
// the sentences of a refused ZIP import (CYD_IMPORT_INVALID details.reason).
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { createHash, webcrypto } from 'node:crypto';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const apps = path.join(here, '..', 'ui', 'js', 'desktop', 'apps');
const store = new Map();
const sandbox = {
    window: { I18N: {}, SYSTEM_LANG: 'de' },
    console, JSON, Math, Date, Number, String, Array, Object, Set, Map, Promise, Error, RegExp, Uint8Array, Uint32Array, Intl,
    TextDecoder, TextEncoder, crypto: webcrypto, structuredClone, atob, Response, Headers, FormData, Blob,
    setTimeout, clearTimeout, setInterval, clearInterval, encodeURIComponent, fetch: null,
    localStorage: { getItem: k => (store.has(k) ? store.get(k) : null), setItem: (k, v) => store.set(k, String(v)) }
};
sandbox.globalThis = sandbox;
vm.createContext(sandbox);
for (const file of ['cyd-creator-core.js', 'cyd-creator-api.js', 'cyd-creator-serial.js', 'cyd-creator-flasher.js', 'cyd-creator-monitor.js', 'cyd-creator-devices.js', 'cyd-creator-transfer.js']) {
    const full = path.join(apps, file);
    vm.runInContext(fs.readFileSync(full, 'utf8'), sandbox, { filename: full });
}
const P = sandbox.window.CYDCreatorParts;

let failures = 0;
function check(name, cond, detail) {
    if (cond) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + (detail ? ' — ' + detail : ''));
}
function eq(name, got, want) {
    check(name, JSON.stringify(got) === JSON.stringify(want), 'got ' + JSON.stringify(got) + ' want ' + JSON.stringify(want));
}
async function rejects(name, promise, code) {
    try {
        await promise;
        check(name, false, 'resolved');
    } catch (err) {
        eq(name, err && err.code, code);
    }
}
const tick = () => new Promise(resolve => setImmediate(resolve));

// ── fake Web Serial ──
class FakePort {
    constructor() {
        this.opened = false;
        this.signals = [];
        this.log = [];
        this.controller = null;
        this.readable = null;
        this.failOpen = null;
    }
    async open(options) {
        if (this.failOpen) throw this.failOpen;
        if (this.opened) { const e = new Error('The port is already open.'); e.name = 'InvalidStateError'; throw e; }
        this.opened = true;
        this.baud = options.baudRate;
        this.log.push('open ' + options.baudRate);
        this.readable = new ReadableStream({ start: c => { this.controller = c; } });
    }
    async close() {
        if (this.readable && this.readable.locked) throw new Error('Cannot close a locked stream');
        this.opened = false;
        this.readable = null;
        this.log.push('close');
    }
    async setSignals(s) { this.signals.push(s); }
    feed(text) { this.controller.enqueue(new TextEncoder().encode(text)); }
}
function fakeSerial(port) {
    const listeners = {};
    return {
        port,
        cancelled: false,
        async requestPort() {
            if (this.cancelled) { const e = new Error('No port selected by the user.'); e.name = 'NotFoundError'; throw e; }
            return port;
        },
        addEventListener(type, fn) { listeners[type] = fn; },
        fire(type, target) { if (listeners[type]) listeners[type]({ target }); }
    };
}
const okEnv = serial => ({ isSecureContext: true, serial, top: 1, self: 1, policy: { allowsFeature: () => true } });

// ── support and classify ──
{
    const serial = fakeSerial(new FakePort());
    eq('support needs a secure context', P.serial.support(Object.assign(okEnv(serial), { isSecureContext: false })).reason, 'insecure');
    eq('support needs navigator.serial', P.serial.support(Object.assign(okEnv(null))).reason, 'unsupported');
    eq('support refuses a framed page', P.serial.support(Object.assign(okEnv(serial), { top: 1, self: 2 })).reason, 'framed');
    eq('support honours Permissions-Policy', P.serial.support(Object.assign(okEnv(serial), { policy: { allowsFeature: f => f !== 'serial' } })).reason, 'policy');
    eq('support accepts the top-level secure desktop', P.serial.support(okEnv(serial)), { ok: true, reason: '' });
    const named = (name, message) => Object.assign(new Error(message), { name });
    eq('classify: port already open', P.serial.classify(named('InvalidStateError', 'The port is already open.')), 'CYD_FLASH_PORT_BUSY');
    eq('classify: port used elsewhere', P.serial.classify(named('NetworkError', 'Failed to open serial port.')), 'CYD_FLASH_PORT_BUSY');
    eq('classify: board unplugged', P.serial.classify(named('NetworkError', 'The device has been lost.')), 'CYD_FLASH_DISCONNECTED');
    eq('classify: unknown', P.serial.classify(new Error('Timeout')), '');
}

// ── D2: the desktop serial policy gates flashing and the monitor ──
{
    const serial = fakeSerial(new FakePort());
    const env = desktop => Object.assign(okEnv(serial), { desktop });
    eq('D2: the desktop serial switch off disables flashing and the monitor', P.serial.support(env({ serial: false, readonly: false })).reason, 'disabled');
    eq('D2: an unknown desktop policy counts as off', P.serial.support(env({ serial: null, readonly: null })).reason, 'disabled');
    eq('D2: a read-only desktop disables flashing and the monitor', P.serial.support(env({ serial: true, readonly: true })).reason, 'readonly');
    eq('D2: the switch on allows the top-level secure desktop', P.serial.support(env({ serial: true, readonly: false })), { ok: true, reason: '' });
    try {
        await P.serial.createManager(env({ serial: false, readonly: false })).request();
        check('D2: a disabled desktop cannot open the port chooser', false, 'resolved');
    } catch (err) {
        eq('D2: a disabled desktop cannot open the port chooser', [err.code, err.details && err.details.reason], ['CYD_SERIAL_UNSUPPORTED', 'disabled']);
    }
    const lazy = env({ serial: null, readonly: null });
    lazy.loadPolicy = async () => { lazy.desktop = { serial: true, readonly: false }; };
    check('D2: an unknown policy is loaded before the chooser opens', (await P.serial.createManager(lazy).request()) === serial.port);
    const flipping = env({ serial: true, readonly: false });
    const gated = P.serial.createManager(flipping);
    await gated.request();
    flipping.desktop = { serial: true, readonly: true };
    await rejects('D2: a desktop turned read-only refuses the port to the monitor', gated.acquire('monitor', { revocable: true }), 'CYD_SERIAL_UNSUPPORTED');
    const seen = [];
    const off = P.serial.manager.on(type => seen.push(type));
    P.serial.policy.apply({ serial_browser_enabled: true, readonly: false });
    eq('D2: an aurago:desktop-policy event sets the switch', P.serial.policy.state(), { serial: true, readonly: false });
    P.serial.policy.apply({ serial_browser_enabled: false, serial_host_enabled: false });
    eq('D2: a lost desktop socket turns serial off and keeps read-only', P.serial.policy.state(), { serial: false, readonly: false });
    eq('D2: the page manager announces policy changes', seen, ['policy', 'policy']);
    off();
}

// ── manager: request, reader, reset, handoff ──
{
    const port = new FakePort();
    const serial = fakeSerial(port);
    const manager = P.serial.createManager(okEnv(serial));
    await rejects('unsupported browsers cannot request a port', P.serial.createManager(okEnv(null)).request(), 'CYD_SERIAL_UNSUPPORTED');
    serial.cancelled = true;
    eq('a closed chooser returns null', await manager.request(), null);
    serial.cancelled = false;
    check('request remembers the port', (await manager.request()) === port && manager.hasPort());

    let revoked = 0;
    const monitor = await manager.acquire('monitor', { revocable: true, onRevoke: async () => { revoked++; } });
    await monitor.open(115200);
    const chunks = [];
    let ended = null;
    monitor.startReader(chunk => chunks.push(new TextDecoder().decode(chunk)), (err, stopped) => { ended = { err, stopped }; });
    port.feed('boot\r\n');
    await tick(); await tick();
    eq('the reader loop delivers chunks', chunks, ['boot\r\n']);
    await monitor.reset();
    eq('reset pulses RTS with DTR released', port.signals, [{ dataTerminalReady: false, requestToSend: true }, { dataTerminalReady: false, requestToSend: false }]);
    eq('the monitor owns the port', manager.owner(), 'monitor');

    const flasher = await manager.acquire('flasher', { revocable: false });
    eq('a flash revokes the monitor first', [revoked, manager.owner(), port.opened], [1, 'flasher', false]);
    check('the revoked reader ended as stopped', ended && ended.stopped === true && !ended.err, JSON.stringify(ended));
    await rejects('nobody takes the port during a flash', manager.acquire('monitor', { revocable: true }), 'CYD_FLASH_PORT_BUSY');
    check('the flasher gets the raw port for esptool', flasher.port === port);
    await flasher.release();
    eq('released: no owner', manager.owner(), '');
    const again = await manager.acquire('monitor', { revocable: true });
    await again.open(115200);
    eq('the monitor takes the port back after the flash (handoff)', [manager.owner(), port.baud], ['monitor', 115200]);
    let lost = null;
    manager.on((type, p) => { if (type === 'disconnect') lost = p; });
    serial.fire('disconnect', port);
    check('a lost port is announced', lost === port);
    port.failOpen = Object.assign(new Error('Failed to open serial port.'), { name: 'NetworkError' });
    await again.release();
    const busy = await manager.acquire('monitor', { revocable: true });
    await rejects('a port held by another program is busy', busy.open(115200), 'CYD_FLASH_PORT_BUSY');
    await busy.release();
}

// ── monitor helpers ──
{
    const ring = P.monitor.createRing(5000);
    for (let i = 1; i <= 5003; i++) ring.push({ id: i });
    eq('the ring keeps 5000 lines', [ring.size(), ring.dropped(), ring.all()[0].id, ring.all()[4999].id], [5000, 3, 4, 5003]);
    ring.clear();
    eq('clear empties the ring', [ring.size(), ring.dropped()], [0, 0]);

    const split = P.monitor.createLineSplitter();
    const enc = new TextEncoder();
    const umlaut = enc.encode('Grüße\n');
    eq('lines split on CRLF and LF', split.push(enc.encode('abc\r\nde')), ['abc']);
    eq('a split line joins the next chunk', split.push(enc.encode('f\n')), ['def']);
    eq('a multi-byte character may be cut between chunks', [...split.push(umlaut.slice(0, 3)), ...split.push(umlaut.slice(3))], ['Grüße']);
    split.push(enc.encode('prompt> '));
    eq('flush returns the partial line', [split.pending(), split.flush(), split.pending()], [true, 'prompt> ', false]);

    const esp32 = 'Backtrace: 0x400d1234:0x3ffb1f50 0x400d5678:0x3ffb1f70 0x40088e2d:0x3ffb1f90 |<-CORRUPTED';
    eq('backtrace PCs of an ESP32 panic', P.monitor.parseBacktrace(esp32), ['0x400d1234', '0x400d5678', '0x40088e2d']);
    eq('backtrace of an ESP32-S3 panic', P.monitor.parseBacktrace('Backtrace: 0x42002A3C:0x3FC9A0E0 0x4037A1B2:0x3FC9A100'), ['0x42002a3c', '0x4037a1b2']);
    eq('other lines have no backtrace', [P.monitor.parseBacktrace('Guru Meditation Error'), P.monitor.parseBacktrace('Backtrace: none')], [[], []]);

    const lines = Array.from({ length: 800 }, (_, i) => 'line ' + i + ' ' + 'x'.repeat(30));
    const text = P.monitor.attachText(lines);
    check('attach keeps at most 16 KiB', new TextEncoder().encode(text).length <= 16384, String(text.length));
    check('attach keeps the newest lines from a line start', text.endsWith(lines[799]) && /^line \d+ x/.test(text));
    eq('short logs are attached whole', P.monitor.attachText(['a', 'b']), 'a\nb');
}

// ── flasher helpers ──
{
    eq('baud fallback 921600 → 460800 → 115200 → none', [P.flasher.nextBaud(921600), P.flasher.nextBaud(460800), P.flasher.nextBaud(115200), P.flasher.nextBaud(9600)], [460800, 115200, 0, 0]);
    eq('chip names are compared normalized', [P.flasher.chipMatches('ESP32', 'esp32'), P.flasher.chipMatches('ESP32-S3', 'esp32s3'), P.flasher.chipMatches('ESP32-S3', 'esp32'), P.flasher.chipMatches('', '')], [true, true, false, false]);
    const random = new Uint8Array(1 << 20);
    for (let i = 0; i < random.length; i += 65536) webcrypto.getRandomValues(random.subarray(i, i + 65536));
    for (const sample of [new Uint8Array(0), new TextEncoder().encode('abc'), new Uint8Array(55), new Uint8Array(64), random]) {
        eq('md5 of ' + sample.length + ' bytes matches node:crypto', P.flasher.md5Hex(sample), createHash('md5').update(sample).digest('hex'));
    }
    const image = new TextEncoder().encode('merged image');
    const sha = createHash('sha256').update(image).digest('hex');
    eq('sha256Hex matches node:crypto', await P.flasher.sha256Hex(image), sha);
    eq('verifyImage accepts matching hashes', await P.flasher.verifyImage(image, sha, sha.toUpperCase()), sha);
    await rejects('verifyImage refuses a wrong record hash', P.flasher.verifyImage(image, 'ab'.repeat(32), sha), 'IMAGE_CHECK');
    await rejects('verifyImage refuses a wrong header', P.flasher.verifyImage(image, sha, '00'), 'IMAGE_CHECK');
    await rejects('verifyImage refuses without any hash', P.flasher.verifyImage(image, '', ''), 'IMAGE_CHECK');
    eq('base64 config blob decodes', Array.from(P.flasher.base64Bytes('QUdDQw==')), [65, 71, 67, 67]);
    P.flasher.rememberFlashed('ccp_1', 'ccb_9');
    eq('the flashed build is remembered per project', [P.flasher.lastFlashed('ccp_1'), P.flasher.lastFlashed('ccp_2')], ['ccb_9', '']);
}

// ── runFlash with a fake esptool ──
function fakeEsptool(plan) {
    const calls = { attempts: [], disconnects: 0, after: [], files: null, progress: [] };
    class Transport { constructor(port) { this.port = port; } async disconnect() { calls.disconnects++; } }
    class ESPLoader {
        constructor(o) {
            this.o = o;
            this.chip = { CHIP_NAME: plan.chip || 'ESP32', readMac: async () => plan.mac || 'aa:bb:cc:dd:ee:ff' };
            calls.attempts.push(o.baudrate);
        }
        async main() {
            if (plan.mainError) throw plan.mainError;
            return 'ESP32-D0WD-V3 (revision v3.1)';
        }
        async writeFlash(options) {
            calls.files = options.fileArray.map(f => [f.address, f.data.length]);
            calls.data = options.fileArray.map(f => f.data);
            calls.md5 = typeof options.calculateMD5Hash === 'function';
            if (plan.failAt && plan.failAt.includes(this.o.baudrate)) throw new Error('Timeout waiting for packet header');
            options.fileArray.forEach((f, i) => { options.reportProgress(i, 0, 10); options.reportProgress(i, 5, 10); options.reportProgress(i, 10, 10); });
        }
        async after(mode) { calls.after.push(mode); }
    }
    return { module: { Transport, ESPLoader }, calls };
}
async function flashWith(plan, extra) {
    const port = new FakePort();
    const manager = P.serial.createManager(okEnv(fakeSerial(port)));
    await manager.request();
    const fake = fakeEsptool(plan);
    const progress = [];
    const run = P.flasher.runFlash(Object.assign({
        manager, load: async () => fake.module, image: new Uint8Array(1000), blob: new Uint8Array(4096), blobOffset: 0x3D0000,
        chip: 'esp32', monitor: true, onProgress: f => progress.push(f), onLog: () => {}
    }, extra || {}));
    return { run, fake, progress, manager };
}
{
    const ok = await flashWith({});
    const result = await ok.run;
    eq('a flash writes the image at 0x0 and the blob at config_offset', ok.fake.calls.files, [[0, 1000], [0x3D0000, 4096]]);
    eq('the first attempt runs at 921600 baud with the ROM at 115200', [result.baud, ok.fake.calls.attempts], [921600, [921600]]);
    check('esptool verifies with MD5', ok.fake.calls.md5 === true);
    check('progress grows to 100 %', ok.progress.every((v, i) => i === 0 || v >= ok.progress[i - 1]) && ok.progress[ok.progress.length - 1] === 1, JSON.stringify(ok.progress));
    eq('with the monitor on, esptool leaves the reset to the monitor', ok.fake.calls.after, ['no_reset_stub']);
    eq('the flasher released the port', ok.manager.owner(), '');

    const plain = await flashWith({}, { monitor: false, blob: null });
    await plain.run;
    eq('without the monitor esptool hard-resets; no blob without a link', [plain.fake.calls.after, plain.fake.calls.files], [['hard_reset'], [[0, 1000]]]);

    const fallback = await flashWith({ failAt: [921600] });
    eq('a failed write falls back to 460800 baud', [(await fallback.run).baud, fallback.fake.calls.attempts, fallback.fake.calls.disconnects], [460800, [921600, 460800], 2]);

    const broken = await flashWith({ failAt: [921600, 460800, 115200] });
    try { await broken.run; check('three failed rates fail the flash', false); } catch (err) {
        eq('three failed rates fail the flash', [broken.fake.calls.attempts, /Timeout/.test(err.message), broken.manager.owner()], [[921600, 460800, 115200], true, '']);
    }

    const wrong = await flashWith({ chip: 'ESP32-S3' });
    await rejects('a wrong chip stops at once', wrong.run, 'CYD_FLASH_WRONG_CHIP');
    eq('a wrong chip does not retry', wrong.fake.calls.attempts, [921600]);

    const unplugged = await flashWith({ mainError: Object.assign(new Error('The device has been lost.'), { name: 'NetworkError' }) });
    await rejects('an unplugged board stops at once', unplugged.run, 'CYD_FLASH_DISCONNECTED');
    eq('an unplugged board does not retry', unplugged.fake.calls.attempts, [921600]);

    // The monitor holds the port: the flash takes it over and gives it back afterwards.
    const port = new FakePort();
    const manager = P.serial.createManager(okEnv(fakeSerial(port)));
    await manager.request();
    let paused = false;
    const mon = await manager.acquire('monitor', { revocable: true, onRevoke: async () => { paused = true; } });
    await mon.open(115200);
    const fake = fakeEsptool({});
    await P.flasher.runFlash({ manager, load: async () => fake.module, image: new Uint8Array(10), chip: 'esp32', monitor: true });
    const back = await manager.acquire('monitor', { revocable: true });
    await back.open(115200);
    eq('owner handoff monitor → flasher → monitor', [paused, manager.owner(), port.log], [true, 'monitor', ['open 115200', 'close', 'open 115200']]);
}

// ── D1: install modes ──
{
    const keep = await flashWith({ mac: 'AA:BB:CC:DD:EE:01' }, { mode: 'keep', app: new Uint8Array(700) });
    const result = await keep.run;
    eq('D1 keep settings: the app at 0x10000, otadata erased at 0xE000, then the blob', keep.fake.calls.files, [[0x10000, 700], [0xE000, 0x2000], [0x3D0000, 4096]]);
    const otadata = keep.fake.calls.data[1];
    check('D1 keep settings: the otadata entry is a Uint8Array of 8 KiB 0xFF', otadata instanceof Uint8Array && otadata.length === 8192 && otadata.every(b => b === 0xff));
    check('D1 every fileArray entry is a Uint8Array (esptool-js 0.7.0)', keep.fake.calls.data.every(d => d instanceof Uint8Array));
    eq('D1 the result names the mode and the board MAC', [result.mode, result.mac], ['keep', 'aa:bb:cc:dd:ee:01']);
    await rejects('D1 keep settings needs firmware.bin', (await flashWith({}, { mode: 'keep', app: null })).run, 'FLASH_FAILED');

    P.flasher.rememberBoard('ccp_1', { mac: 'AA:BB:CC:DD:EE:01', chip: 'ESP32', board: 'esp32-2432s028r', device: 'ccd_1' });
    const known = info => P.flasher.knownBoard('ccp_1', { mac: info.mac, chip: 'esp32', board: 'esp32-2432s028r' });
    eq('D1 a board record matches project, MAC, chip and board profile', [
        known({ mac: 'aa:bb:cc:dd:ee:01' }),
        known({ mac: 'aa:bb:cc:dd:ee:02' }),
        P.flasher.knownBoard('ccp_1', { mac: 'aa:bb:cc:dd:ee:01', chip: 'esp32s3', board: 'esp32-2432s028r' }),
        P.flasher.knownBoard('ccp_1', { mac: 'aa:bb:cc:dd:ee:01', chip: 'esp32', board: 'esp32-3248s035c' }),
        P.flasher.knownBoard('ccp_2', { mac: 'aa:bb:cc:dd:ee:01', chip: 'esp32', board: 'esp32-2432s028r' }),
        P.flasher.knownBoard('ccp_1', { mac: '', chip: 'esp32', board: 'esp32-2432s028r' })
    ], [true, false, false, false, false, false]);

    const chosen = [];
    const autoKnown = await flashWith({ mac: 'aa:bb:cc:dd:ee:01', failAt: [921600] }, { mode: 'auto', app: new Uint8Array(700), isKnownBoard: known, onMode: m => chosen.push(m) });
    await autoKnown.run;
    eq('D1 automatic keeps the settings of a board this project flashed, decided once across retries', [chosen, autoKnown.fake.calls.files[0], autoKnown.fake.calls.attempts], [['keep'], [0x10000, 700], [921600, 460800]]);
    const autoNew = await flashWith({ mac: 'aa:bb:cc:dd:ee:02' }, { mode: 'auto', app: new Uint8Array(700), isKnownBoard: known });
    eq('D1 automatic does a full install on a board new to the project', [(await autoNew.run).mode, autoNew.fake.calls.files], ['full', [[0, 1000], [0x3D0000, 4096]]]);
    const forcedFull = await flashWith({ mac: 'aa:bb:cc:dd:ee:01' }, { mode: 'full', app: new Uint8Array(700), isKnownBoard: known });
    eq('D1 the user can force a full install on a known board', (await forcedFull.run).mode, 'full');
}

// ── a window closed mid-flash aborts the flash cleanly ──
{
    let closed = false;
    const torn = await flashWith({}, {
        aborted: () => closed,
        onProgress: (fraction, phase) => { if (phase === 'write' && fraction > 0) closed = true; }
    });
    await rejects('closing the window mid-write aborts the flash', torn.run, 'FLASH_ABORTED');
    eq('an aborted flash does not retry or reset and frees the port', [torn.fake.calls.attempts, torn.fake.calls.after, torn.fake.calls.disconnects, torn.manager.owner()], [[921600], [], 1, '']);
    const early = await flashWith({}, { aborted: () => true });
    await rejects('a flash whose window is already gone never connects', early.run, 'FLASH_ABORTED');
    eq('…and releases the port without an attempt', [early.fake.calls.attempts, early.manager.owner()], [[], '']);
}

// ── devices: the rollback hint; transfer: a refused ZIP import names its reason ──
{
    const words = {
        'cyd_creator.devices.error_rolled_back': 'Rolled back.',
        'cyd_creator.devices.rollback_hint': 'Same build again needs a restart.',
        'cyd_creator.devices.error_failed': 'Failed: {{reason}}',
        'cyd_creator.transfer.importing': 'Importing {{name}}',
        'cyd_creator.transfer.limit_asset_bytes': 'Images over {{size}}.',
        'cyd_creator.transfer.limit_app_files': 'Too many source files.',
        'cyd_creator.error.CYD_IMPORT_INVALID': 'Not an export.',
        'cyd_creator.error.CYD_TEMPLATE_TOO_NEW': 'Too new.'
    };
    Object.assign(sandbox.window.I18N, words);
    const t = P.i18n({});
    eq('a rollback explains that re-picking the failed build waits for a restart',
        P.devices.errorText(t, 'rolled_back: rolled back from ccb_2'), 'Rolled back. (rolled back from ccb_2) Same build again needs a restart.');
    eq('a rollback without detail still carries the hint', P.devices.errorText(t, 'rolled_back'), 'Rolled back. Same build again needs a restart.');
    eq('a failed download has no rollback hint', P.devices.errorText(t, 'failed: http 404'), 'Failed: http 404');

    const notes = [];
    const inst = { t, readonly: false, notify: (kind, text) => notes.push([kind, text]) };
    const realImport = P.api.importZip;
    const realPick = P.transfer.pickFile;
    P.transfer.pickFile = async () => ({ name: 'wetter.zip' });
    const refuse = async details => {
        P.api.importZip = async () => { throw new P.api.ApiError(422, 'CYD_IMPORT_INVALID', 'refused', details); };
        notes.length = 0;
        const result = await P.transfer.importZip(inst);
        return [result, notes.at(-1)];
    };
    eq('import: images over the asset budget name the limit', await refuse({ reason: 'asset', limit: 'asset_bytes', max: 16 * 1024 * 1024, path: 'assets/big.png' }),
        [null, ['error', 'Images over 16.00 MiB.']]);
    eq('import: too many source files use the src/app limit sentence', await refuse({ reason: 'app_files', limit: 64 }), [null, ['error', 'Too many source files.']]);
    eq('import: a letter-case clash names the path', await refuse({ reason: 'app_files', path: 'src/app/Lib/a.cpp', existing: 'src/app/lib' }),
        [null, ['error', 'Not an export. (src/app/Lib/a.cpp)']]);
    eq('import: an unknown board names the board', await refuse({ reason: 'board', board: 'esp32-foo' }), [null, ['error', 'Not an export. (esp32-foo)']]);
    eq('import: not a ZIP shows the generic sentence', await refuse({ reason: 'not_zip' }), [null, ['error', 'Not an export.']]);
    P.api.importZip = async () => { throw new P.api.ApiError(422, 'CYD_TEMPLATE_TOO_NEW', 'too new', {}); };
    await rejects('import: other refusals reach the caller', P.transfer.importZip(inst), 'CYD_TEMPLATE_TOO_NEW');
    P.api.importZip = async file => ({ id: 'ccp_9', name: file.name });
    eq('import: a good archive returns the new project', [await P.transfer.importZip(inst), notes.at(-1)], [{ id: 'ccp_9', name: 'wetter.zip' }, ['info', 'Importing wetter.zip']]);
    await rejects('import: read-only users cannot import', P.transfer.importZip(Object.assign({}, inst, { readonly: true })), 'CYD_READONLY');
    P.api.importZip = realImport;
    P.transfer.pickFile = realPick;
    Object.keys(words).forEach(key => delete sandbox.window.I18N[key]);
}

if (failures) {
    console.log(failures + ' check(s) failed');
    process.exit(1);
}
console.log('all CYD Creator flash checks passed');
