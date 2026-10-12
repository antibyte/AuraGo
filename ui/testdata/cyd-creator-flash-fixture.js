// CYD Creator flash fixture (plan 5), loaded after cyd-creator-fixture.js by
// TestDesktopCYDCreatorFlashBrowser: device, artefact, backtrace, export and import routes on top of the
// shell fixture, the cydcc offset on every board profile, two green builds for the seeded project (whose
// layout gets the AuraGo link): ccb_green_000002 carries the link's heartbeat path in its images,
// ccb_green_000001 does not. A fake navigator.serial with a scripted CYD (boot log and a panic after
// every RTS reset) and a fake esptool module that records every write (address, size and which image,
// blob or 0xFF fill it was) and the partition table a full install left on the board.
// window.cydFlashFixture.install() wires the fake esptool and the file picker once the app modules loaded.
(function () {
    'use strict';

    const BASE = '/api/desktop/cyd-creator';
    // ConfigPartitionOffset(4) of internal/cydcreator/partitions.go: both fixture boards have 4 MB flash.
    const CONFIG_OFFSET = 0x3D0000;
    // Only firmware built with CYD_AURAGO_LINK contains this path (the flasher scans the image for it).
    const LINK_MARKER = '/api/cyd-creator/device/heartbeat';
    const previousFetch = window.fetch;
    const reply = (data, status, headers) => new Response(JSON.stringify(data), { status: status || 200, headers: Object.assign({ 'Content-Type': 'application/json; charset=utf-8' }, headers || {}) });
    const fail = (status, code, error, details) => reply(Object.assign({ error: error || code, code }, details ? { details } : {}), status);
    const iso = offset => new Date(Date.now() + (offset || 0)).toISOString();
    const wait = ms => new Promise(r => setTimeout(r, ms));
    const hex = buf => Array.from(new Uint8Array(buf), b => b.toString(16).padStart(2, '0')).join('');
    const fx = window.cydFixture;
    const state = {
        images: {},
        devices: [],
        mints: [],
        // calls: artefact downloads and mint requests in order (the image is checked before minting).
        calls: [],
        patches: [],
        deletes: [],
        decodes: [],
        exports: 0,
        downloads: [],
        imports: [],
        // picks: the files the import picker returns, in order (a foreign archive first).
        picks: ['fremd.zip', 'wetter-import.zip'],
        // mintFail: the next POST /devices answers CYD_BAD_REQUEST with these details (once).
        mintFail: null,
        esptool: { attempts: [], writes: [], after: [], failAt: [921600], mac: 'AA:BB:CC:DD:EE:01', table: '' },
        port: null
    };
    let devSeq = 0;

    // image is a deterministic test image; link puts the heartbeat path at offset at.
    function image(size, seed, link, at) {
        const bytes = new Uint8Array(size).map((_, i) => (i * 31 + seed) & 0xff);
        if (link) bytes.set(Array.from(LINK_MARKER, ch => ch.charCodeAt(0)), at);
        return bytes;
    }

    // ── seed: the board profiles' cydcc offset, AuraGo link on, two green builds ──
    fx.state.caps.boards.forEach(board => { board.config_offset = CONFIG_OFFSET; });
    const seeded = Array.from(fx.state.projects.values())[0];
    fx.state.layouts.get(seeded.id).project.settings.aurago_link = true;
    const ready = Promise.all(['ccb_green_000001', 'ccb_green_000002'].map(async (id, i) => {
        const link = id === 'ccb_green_000002';
        // merged.bin reaches past the partition table (0x8000 + 0xC00) like a real one; the marker sits after it.
        const merged = image(0x9000, 0, link, 0x8E00);
        const app = image(0x2000, 7, link, 0x100);
        const sha = { merged: hex(await crypto.subtle.digest('SHA-256', merged)), app: hex(await crypto.subtle.digest('SHA-256', app)) };
        state.images[id] = { merged, app, sha };
        const build = {
            id, project_id: seeded.id, job_id: '', origin: 'user', status: 'succeeded', board: seeded.board, project_version: 1, error_code: '',
            diagnostics: [], usage: { flash_used: 1210000, flash_total: 1966080, ram_used: 81234, ram_total: 327680, duration_ms: 4100, warnings: 0 },
            artifacts: [{ name: 'merged.bin', size: merged.length, sha256: sha.merged }, { name: 'firmware.bin', size: app.length, sha256: sha.app }],
            created_at: iso(-600000 + i * 60000), started_at: iso(-600000 + i * 60000), finished_at: iso(-590000 + i * 60000)
        };
        fx.state.builds.set(id, build);
        return build;
    })).then(builds => { seeded.last_build = builds[1]; });

    function deviceFor(projectId, buildId) {
        const id = 'ccd_' + String(++devSeq).padStart(12, '0');
        return { id, project_id: projectId, name: 'Wetterstation #' + devSeq, board: seeded.board, status: 'pending', build_id: buildId, target_build_id: buildId,
            ip: '', rssi: 0, free_heap: 0, template_version: '', ota_state: '', last_error: '', last_seen: '', online: false, created_at: iso() };
    }

    async function route(method, path, init) {
        const u = new URL(path, location.origin);
        const parts = u.pathname.slice(BASE.length).split('/').filter(Boolean).map(decodeURIComponent);
        const body = () => { try { return JSON.parse(init.body || '{}'); } catch (_) { return {}; } };
        const [head, id, sub, name] = parts;
        if (head === 'projects' && id && sub === 'devices') {
            if (method === 'GET') return reply({ devices: state.devices.filter(d => d.project_id === id) });
            if (method === 'POST') {
                const b = body();
                state.calls.push('mint:' + b.build_id);
                if (state.mintFail) {
                    const details = state.mintFail;
                    state.mintFail = null;
                    return fail(400, 'CYD_BAD_REQUEST', 'devices cannot reach this server', details);
                }
                state.mints.push(b);
                const device = deviceFor(id, b.build_id);
                state.devices.push(device);
                // The cydcc blob: "AGCC" and the device number, so a write names the blob it carried.
                const blob = new Uint8Array(4096);
                blob.set([65, 71, 67, 67, devSeq]);
                return reply({ device, config_blob: btoa(String.fromCharCode(...blob)), config_offset: CONFIG_OFFSET }, 201);
            }
        }
        if (head === 'devices' && id) {
            const device = state.devices.find(d => d.id === id);
            if (!device) return fail(404, 'CYD_NOT_FOUND', 'device not found');
            if (method === 'PATCH') {
                const b = body();
                state.patches.push(b);
                if (b.name) device.name = b.name;
                if (b.target_build_id) device.target_build_id = b.target_build_id;
                return reply({ device });
            }
            if (method === 'DELETE') {
                state.deletes.push(id);
                state.devices = state.devices.filter(d => d.id !== id);
                return new Response(null, { status: 204 });
            }
        }
        if (head === 'builds' && sub === 'artifacts' && method === 'GET' && state.images[id]) {
            const which = name === 'merged.bin' ? 'merged' : (name === 'firmware.bin' ? 'app' : '');
            if (which) {
                state.calls.push('artifact:' + id + '/' + name);
                const im = state.images[id];
                return new Response(im[which], { status: 200, headers: { 'Content-Type': 'application/octet-stream', 'X-CYD-SHA256': im.sha[which] } });
            }
        }
        if (head === 'builds' && sub === 'decode-backtrace' && method === 'POST') {
            const b = body();
            state.decodes.push({ build: id, addresses: b.addresses });
            return reply({ frames: b.addresses.map((a, i) => ({ address: a, function: i === 0 ? 'app_loop()' : 'loopTask(void*)', file: i === 0 ? 'src/app/app_main.cpp' : 'main.cpp', line: i === 0 ? 42 : 50 })) });
        }
        if (head === 'projects' && sub === 'export' && method === 'GET') {
            state.exports++;
            return new Response(new Uint8Array([80, 75, 5, 6, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]), { status: 200, headers: { 'Content-Type': 'application/zip', 'Content-Disposition': 'attachment; filename=wetterstation.zip' } });
        }
        if (head === 'import' && method === 'POST') {
            const file = init.body && typeof init.body.get === 'function' ? init.body.get('file') : null;
            state.imports.push(file ? file.name : '');
            // Like the server's import checks: an unknown file refuses the archive with its reason and path.
            if (file && file.name === 'fremd.zip') {
                return fail(422, 'CYD_IMPORT_INVALID', 'the archive contains a file that is not part of a CYD Creator project', { reason: 'file_type', path: 'secrets/wlan.txt' });
            }
            const project = fx.addProject({ name: 'Importiert', board: 'esp32-2432s028r', theme: 'night', language: 'de', timezone: 'UTC0' });
            return reply({ project }, 201);
        }
        return null;
    }

    window.fetch = async (url, options) => {
        const path = String(url);
        const init = options || {};
        if (path.startsWith(BASE + '/')) {
            await ready;
            const answer = await route(String(init.method || 'GET').toUpperCase(), path, init);
            if (answer) return answer;
        }
        return previousFetch(url, init);
    };

    // ── downloads: an export's blob link is recorded (name, size) instead of saved to disk ──
    const blobs = new Map();
    const createObjectURL = URL.createObjectURL;
    URL.createObjectURL = obj => {
        const href = createObjectURL.call(URL, obj);
        blobs.set(href, obj);
        return href;
    };
    const anchorClick = HTMLAnchorElement.prototype.click;
    HTMLAnchorElement.prototype.click = function () {
        if (this.download && String(this.href).startsWith('blob:')) {
            const blob = blobs.get(this.href);
            state.downloads.push({ name: this.download, size: blob ? blob.size : -1, type: blob ? blob.type : '' });
            return;
        }
        anchorClick.call(this);
    };

    // ── fake Web Serial: a CYD that logs its boot and panics after a reset ──
    class FakePort {
        constructor() { this.readable = null; this.controller = null; this.opens = []; this.signals = []; this.rts = false; }
        async open(o) {
            this.opens.push(o.baudRate);
            this.readable = new ReadableStream({ start: c => { this.controller = c; } });
        }
        async close() { this.readable = null; this.controller = null; }
        async setSignals(s) {
            this.signals.push(s);
            if (s.requestToSend) { this.rts = true; return; }
            if (this.rts) { this.rts = false; this.boot(); }
        }
        getInfo() { return { usbVendorId: 0x1a86, usbProductId: 0x7523 }; }
        async boot() {
            const lines = ['ets Jun  8 2016 00:22:57', 'rst:0x1 (POWERON_RESET),boot:0x13 (SPI_FAST_FLASH_BOOT)', '[cyd] board esp32-2432s028r, template 1.0.0',
                "Guru Meditation Error: Core  1 panic'ed (LoadProhibited). Exception was unhandled.",
                'Backtrace: 0x400d1234:0x3ffb1f50 0x400d5678:0x3ffb1f70 0x40088e2d:0x3ffb1f90'];
            for (const line of lines) {
                await wait(20);
                if (this.controller) this.controller.enqueue(new TextEncoder().encode(line + '\r\n'));
            }
        }
    }
    const listeners = {};
    const fakeSerial = {
        async requestPort() { state.port = state.port || new FakePort(); return state.port; },
        addEventListener(type, fn) { listeners[type] = fn; },
        removeEventListener() {}
    };
    let serialAvailable = true;
    Object.defineProperty(Navigator.prototype, 'serial', { configurable: true, get: () => (serialAvailable ? fakeSerial : undefined) });

    // ── fake esptool-js ──
    // tag names what a write carried: a build image (by MD5), a device blob or a 0xFF fill.
    function tag(data, md5) {
        const sum = md5(data);
        for (const id of Object.keys(state.images)) {
            if (sum === md5(state.images[id].merged)) return id + '/merged.bin';
            if (sum === md5(state.images[id].app)) return id + '/firmware.bin';
        }
        if (data.every(b => b === 0xff)) return 'ff';
        if (data[0] === 65 && data[1] === 71 && data[2] === 67 && data[3] === 67) return 'blob' + data[4];
        return 'other';
    }
    const esptool = {
        Transport: class { constructor(port) { this.port = port; } async disconnect() { await this.port.close(); } },
        ESPLoader: class {
            constructor(o) {
                this.o = o;
                this.chip = { CHIP_NAME: 'ESP32', readMac: async () => state.esptool.mac };
                state.esptool.attempts.push(o.baudrate);
            }
            async main() {
                await this.o.transport.port.open({ baudRate: this.o.romBaudrate || 115200 });
                this.o.terminal.writeLine('Chip is ESP32-D0WD-V3 (revision v3.1)');
                return 'ESP32-D0WD-V3 (revision v3.1)';
            }
            // flashMd5sum reads back the partition table a full install left on the board.
            async flashMd5sum() { return state.esptool.table || '0'.repeat(32); }
            async writeFlash(options) {
                for (let i = 0; i < options.fileArray.length; i++) {
                    for (let step = 0; step <= 4; step++) {
                        await wait(25);
                        options.reportProgress(i, step * 25, 100);
                        // A baud rate in failAt fails once, half way through the first image.
                        if (step === 2 && state.esptool.failAt.includes(this.o.baudrate)) {
                            state.esptool.failAt = state.esptool.failAt.filter(b => b !== this.o.baudrate);
                            throw new Error('Timeout waiting for packet header');
                        }
                    }
                }
                const md5 = options.calculateMD5Hash;
                const full = options.fileArray.find(f => f.address === 0);
                if (full) state.esptool.table = md5(full.data.subarray(0x8000, 0x8C00));
                state.esptool.writes.push({ baud: this.o.baudrate, files: options.fileArray.map(f => [f.address, f.data.length, tag(f.data, md5)]) });
            }
            async after(mode) { state.esptool.after.push(mode); }
        }
    };

    window.cydFlashFixture = {
        state,
        ready,
        install() {
            const P = window.CYDCreatorParts;
            P.flasher.setLoader(() => esptool);
            P.transfer.pickFile = async () => new File([new Uint8Array([80, 75, 3, 4])], state.picks.shift() || 'wetter-import.zip', { type: 'application/zip' });
        },
        setSerial(on) { serialAvailable = !!on; },
        // deviceChanged applies patch to a device and announces it like the server (device_changed).
        deviceChanged(id, patch) {
            const device = state.devices.find(d => d.id === id);
            Object.assign(device, patch);
            fx.emit(device.project_id, 'device_changed', { device: JSON.parse(JSON.stringify(device)) });
        }
    };
})();
