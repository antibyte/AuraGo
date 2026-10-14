// CYD Creator flasher (plan 5): the Flash dialog writes a green build to the board over Web Serial with
// the vendored esptool-js (ui/js/vendor/esptool-js/, imported lazily). Flow (spec §14): pick a build
// (default: the latest green one), choose the port, download merged.bin and check its SHA-256 in the
// browser, mint a device + config blob when the build has the AuraGo link and the user may write,
// connect, compare the chip with the board profile, write it at 0x0 (+ the blob at config_offset) at
// 921600 baud with fallbacks to 460800 and 115200, verify (MD5 read back by esptool), reset and hand the
// port to the serial monitor. Without Web Serial the dialog offers "Download image". runFlash and the
// helpers are pure enough for Node; the esptool loader is injectable (P.flasher.setLoader) for tests.
// Install modes (decision D1): "Full install" writes merged.bin at 0x0 (wipes the NVS: touch calibration
// and Wi-Fi); "Keep settings" writes firmware.bin at app0 (0x10000) and erases otadata (8 KiB 0xFF at
// 0xE000, so app0 boots even after an OTA into app1). "Automatic" (default) keeps the settings when this
// browser's board record says this project flashed the connected board (MAC, chip and board profile)
// and the board's partition table still matches the build's (MD5 read back from flash); a full write
// first drops the board's records, so a failed or foreign install is never treated as known.
// Every flash rewrites the cydcc config partition: the device blob when the BUILD has the AuraGo link
// (its image carries the link's heartbeat path) and the user may mint, else 4 KiB 0xFF, so no stale
// token of an earlier flash survives. Once writing began, a closed window no longer stops the flash:
// it finishes, resets the board and frees the port; the board never stays in the flasher stub.
(function () {
    'use strict';

    const P = window.CYDCreatorParts = window.CYDCreatorParts || {};
    const ESPTOOL_URL = '/js/vendor/esptool-js/esptool-js.min.mjs';
    const BAUDS = [921600, 460800, 115200];
    const ROM_BAUD = 115200;
    const FLASHED_KEY = 'aurago.cydcreator.flashed.v1';
    const BOARDS_KEY = 'aurago.cydcreator.boards.v1';
    const MAX_BOARDS = 32;
    // Every CYD Creator partition table (internal/cydcreator/partitions.go) shares these offsets; the
    // cydcc config offset depends on the flash size and comes from caps.boards[].config_offset.
    const APP_OFFSET = 0x10000;
    const OTADATA_OFFSET = 0xE000;
    const OTADATA_SIZE = 0x2000;
    const CONFIG_SIZE = 0x1000;
    // The ESP-IDF partition table (default offset, 3 KiB area) inside merged.bin and on the board.
    const PART_TABLE_OFFSET = 0x8000;
    const PART_TABLE_SIZE = 0xC00;
    // Only firmware built with CYD_AURAGO_LINK contains the heartbeat path (template cyd_link.cpp).
    const LINK_MARKER = '/api/cyd-creator/device/heartbeat';
    const MODES = ['auto', 'keep', 'full'];
    const MODE_LABELS = { auto: 'cyd_creator.flash.mode_auto', keep: 'cyd_creator.flash.mode_keep', full: 'cyd_creator.flash.mode_full' };
    const MODE_HINTS = { auto: 'cyd_creator.flash.mode_hint', keep: 'cyd_creator.flash.mode_keep_hint', full: 'cyd_creator.flash.mode_full_hint' };
    const FATAL = new Set(['CYD_FLASH_WRONG_CHIP', 'CYD_FLASH_DISCONNECTED', 'CYD_FLASH_PORT_BUSY', 'CYD_SERIAL_UNSUPPORTED', 'FLASH_ABORTED', 'FLASH_FAILED']);

    function flashError(code, message, details) {
        const err = new Error(message || code);
        err.name = 'FlashError';
        err.code = code;
        err.details = details || {};
        return err;
    }

    // nextBaud returns the baud rate after a failed attempt at baud, or 0 when none is left.
    function nextBaud(baud) {
        const index = BAUDS.indexOf(Number(baud));
        return index >= 0 && index < BAUDS.length - 1 ? BAUDS[index + 1] : 0;
    }

    // normalizeChip turns "ESP32-S3" (esptool CHIP_NAME) and "esp32s3" (board profile) into "esp32s3".
    function normalizeChip(name) {
        return String(name || '').toLowerCase().replace(/[^a-z0-9]/g, '');
    }

    function chipMatches(detected, expected) {
        const a = normalizeChip(detected);
        return a !== '' && a === normalizeChip(expected);
    }

    function hex(bytes) {
        return Array.from(new Uint8Array(bytes), b => b.toString(16).padStart(2, '0')).join('');
    }

    async function sha256Hex(bytes) {
        const subtle = (typeof crypto !== 'undefined' && crypto.subtle) || null;
        if (!subtle) throw flashError('IMAGE_CHECK', 'SHA-256 is not available in this browser');
        return hex(await subtle.digest('SHA-256', bytes));
    }

    // verifyImage compares the downloaded image with the SHA-256 of the build record and of the
    // X-CYD-SHA256 header; any mismatch refuses to flash.
    async function verifyImage(bytes, expected, header) {
        const actual = await sha256Hex(bytes);
        const want = [expected, header].filter(Boolean).map(v => String(v).toLowerCase());
        if (!want.length || want.some(v => v !== actual)) throw flashError('IMAGE_CHECK', 'merged.bin does not match its SHA-256', { actual, expected: want });
        return actual;
    }

    // md5Hex (RFC 1321) lets esptool read back and verify every written image.
    const MD5_S = [7, 12, 17, 22, 7, 12, 17, 22, 7, 12, 17, 22, 7, 12, 17, 22, 5, 9, 14, 20, 5, 9, 14, 20, 5, 9, 14, 20, 5, 9, 14, 20,
        4, 11, 16, 23, 4, 11, 16, 23, 4, 11, 16, 23, 4, 11, 16, 23, 6, 10, 15, 21, 6, 10, 15, 21, 6, 10, 15, 21, 6, 10, 15, 21];
    const MD5_K = Array.from({ length: 64 }, (_, i) => Math.floor(Math.abs(Math.sin(i + 1)) * 4294967296) >>> 0);
    function md5Hex(bytes) {
        const len = bytes.length;
        const blocks = ((len + 8) >> 6) + 1;
        const words = new Uint32Array(blocks * 16);
        for (let i = 0; i < len; i++) words[i >> 2] |= bytes[i] << ((i % 4) * 8);
        words[len >> 2] |= 0x80 << ((len % 4) * 8);
        words[blocks * 16 - 2] = (len * 8) >>> 0;
        words[blocks * 16 - 1] = Math.floor(len / 0x20000000);
        let a0 = 0x67452301, b0 = 0xefcdab89, c0 = 0x98badcfe, d0 = 0x10325476;
        for (let block = 0; block < blocks; block++) {
            let a = a0, b = b0, c = c0, d = d0;
            for (let i = 0; i < 64; i++) {
                let f, g;
                if (i < 16) { f = (b & c) | (~b & d); g = i; } else if (i < 32) { f = (d & b) | (~d & c); g = (5 * i + 1) % 16; } else if (i < 48) { f = b ^ c ^ d; g = (3 * i + 5) % 16; } else { f = c ^ (b | ~d); g = (7 * i) % 16; }
                const next = d;
                d = c;
                c = b;
                const sum = (a + f + MD5_K[i] + words[block * 16 + g]) >>> 0;
                b = (b + ((sum << MD5_S[i]) | (sum >>> (32 - MD5_S[i])))) >>> 0;
                a = next;
            }
            a0 = (a0 + a) >>> 0; b0 = (b0 + b) >>> 0; c0 = (c0 + c) >>> 0; d0 = (d0 + d) >>> 0;
        }
        return [a0, b0, c0, d0].map(v => [0, 8, 16, 24].map(s => ((v >>> s) & 0xff).toString(16).padStart(2, '0')).join('')).join('');
    }

    // ── install modes (D1) ──
    // pickMode resolves the dialog's choice: automatic keeps the settings only on a known board.
    function pickMode(mode, known) {
        if (mode === 'keep') return 'keep';
        if (mode === 'auto') return known ? 'keep' : 'full';
        return 'full';
    }

    // configAddress refuses a missing or implausible cydcc offset: a 0 would overwrite the bootloader.
    function configAddress(value, what) {
        const n = Number(value);
        if (!Number.isInteger(n) || n < APP_OFFSET || n % CONFIG_SIZE !== 0) {
            throw flashError('FLASH_FAILED', what + ' has no valid config partition offset (' + String(value) + ')');
        }
        return n;
    }

    // flashFiles returns esptool's fileArray for mode: keep = firmware.bin at app0 + erased otadata,
    // full = merged.bin at 0x0. The config partition follows in both: the blob (AuraGo link) at
    // blobOffset, else 4 KiB 0xFF at configOffset, so an earlier blob (another project's token) is gone.
    function flashFiles(mode, o) {
        let files;
        if (mode === 'keep') {
            if (!o.app || !o.app.length) throw flashError('FLASH_FAILED', 'firmware.bin is missing: keep settings needs the app image');
            files = [{ data: o.app, address: APP_OFFSET }, { data: new Uint8Array(OTADATA_SIZE).fill(0xff), address: OTADATA_OFFSET }];
        } else {
            if (!o.image || !o.image.length) throw flashError('FLASH_FAILED', 'merged.bin is missing');
            files = [{ data: o.image, address: 0 }];
        }
        if (o.blob) files.push({ data: o.blob, address: configAddress(o.blobOffset, 'the device config') });
        else files.push({ data: new Uint8Array(CONFIG_SIZE).fill(0xff), address: configAddress(o.configOffset, 'the board profile') });
        return files;
    }

    // hasLink tells whether a firmware image (firmware.bin or merged.bin) was built with the AuraGo link.
    function hasLink(bytes) {
        if (!bytes || !bytes.length) return false;
        const marker = Array.from(LINK_MARKER, ch => ch.charCodeAt(0));
        const last = bytes.length - marker.length;
        for (let i = bytes.indexOf(marker[0]); i >= 0 && i <= last; i = bytes.indexOf(marker[0], i + 1)) {
            let j = 1;
            while (j < marker.length && bytes[i + j] === marker[j]) j++;
            if (j === marker.length) return true;
        }
        return false;
    }

    // samePartitionTable compares the board's partition table (MD5 read back by the stub) with the one
    // in merged.bin; any doubt (short image, read error) counts as different.
    async function samePartitionTable(loader, merged) {
        if (!merged || merged.length < PART_TABLE_OFFSET + PART_TABLE_SIZE || typeof loader.flashMd5sum !== 'function') return false;
        const want = md5Hex(merged.subarray(PART_TABLE_OFFSET, PART_TABLE_OFFSET + PART_TABLE_SIZE));
        try {
            return String(await loader.flashMd5sum(PART_TABLE_OFFSET, PART_TABLE_SIZE)).toLowerCase() === want;
        } catch (_) {
            return false;
        }
    }

    async function readMac(loader) {
        try {
            const read = loader.chip && loader.chip.readMac;
            return typeof read === 'function' ? normalizeMac(await read.call(loader.chip, loader)) : '';
        } catch (_) {
            return ''; // Secure Download Mode and older stubs cannot read it: the board counts as new.
        }
    }

    function normalizeMac(mac) {
        const m = String(mac || '').trim().toLowerCase();
        return /^[0-9a-f]{2}(:[0-9a-f]{2}){5}$/.test(m) ? m : '';
    }

    // ── esptool loader (injectable) ──
    let loaderFn = null;
    let modulePromise = null;
    function setLoader(fn) { loaderFn = typeof fn === 'function' ? fn : null; modulePromise = null; }
    function loadEsptool() {
        if (loaderFn) return Promise.resolve(loaderFn());
        if (!modulePromise) modulePromise = import(ESPTOOL_URL).catch(err => { modulePromise = null; throw err; });
        return modulePromise;
    }

    async function safeDisconnect(transport) {
        try { await transport.disconnect(); } catch (_) { /* the port may already be closed or lost */ }
    }

    function codeOf(err) {
        if (err && typeof err.code === 'string' && err.code) return err.code;
        return (P.serial && P.serial.classify(err)) || '';
    }

    // runFlash writes the image and the config partition through esptool. o: {manager, load, image
    // (merged.bin), app (firmware.bin), mode ('full' default, 'keep', 'auto'), isKnownBoard({mac, chip}),
    // onMode(mode, {mac}) (once, right before the first write), blob, blobOffset, configOffset, chip,
    // monitor, aborted(), onProgress(fraction, phase, info), onLog(line)}. The mode is decided once, on
    // the first connect (MAC and partition table are readable then). It tries every rate of BAUDS before
    // giving up; a wrong chip, a lost board or a busy port end it at once. aborted() (the window closed)
    // ends it only before the first write: a started write is finished and hard-reset. The flasher holds
    // the port exclusively and releases it before it returns. Result: {baud, chip, mode, mac, after}; a
    // failure carries blobWritten (the device blob was transferred, so its token is on the board).
    async function runFlash(o) {
        const esptool = await (o.load || loadEsptool)();
        const lease = await o.manager.acquire('flasher', { revocable: false });
        const progress = (fraction, phase, info) => { if (typeof o.onProgress === 'function') o.onProgress(fraction, phase, info || {}); };
        const log = line => { if (typeof o.onLog === 'function') o.onLog(String(line)); };
        const aborted = () => typeof o.aborted === 'function' && !!o.aborted();
        const terminal = { clean() {}, writeLine: log, write: log };
        let files = null;
        let total = 0;
        let chosen = '';
        let mac = '';
        let writing = false;
        let blobWritten = false;
        let lastError = null;
        const checkAbort = () => { if (!writing && aborted()) throw flashError('FLASH_ABORTED', 'the flash was aborted (window closed)'); };
        try {
            for (let baud = BAUDS[0]; baud; baud = nextBaud(baud)) {
                try {
                    checkAbort();
                } catch (err) {
                    lastError = err;
                    break;
                }
                const transport = new esptool.Transport(lease.port, false);
                try {
                    progress(0, 'connect', { baud });
                    const loader = new esptool.ESPLoader({ transport, baudrate: baud, romBaudrate: ROM_BAUD, terminal });
                    const description = await loader.main();
                    checkAbort();
                    const chipName = loader.chip && loader.chip.CHIP_NAME;
                    if (!chipMatches(chipName, o.chip)) {
                        throw flashError('CYD_FLASH_WRONG_CHIP', 'detected ' + chipName + ', the board profile needs ' + o.chip, { detected: chipName, expected: o.chip });
                    }
                    if (!files) {
                        mac = await readMac(loader);
                        let known = !!o.app && typeof o.isKnownBoard === 'function' && !!mac && !!o.isKnownBoard({ mac, chip: chipName });
                        // A record can outlive the layout (a failed full install, another tool): check it.
                        if (known && o.mode === 'auto' && !(await samePartitionTable(loader, o.image))) {
                            known = false;
                            log('The partition table on the board differs from this build: full install.');
                        }
                        chosen = pickMode(o.mode, known);
                        files = flashFiles(chosen, o);
                        total = files.reduce((sum, f) => sum + f.data.length, 0);
                        log('Install mode: ' + chosen + (mac ? ' (board ' + mac + ')' : ''));
                        checkAbort();
                        if (typeof o.onMode === 'function') {
                            try { o.onMode(chosen, { mac }); } catch (err) { console.error('cyd-creator flash mode callback failed', err); }
                        }
                    }
                    checkAbort();
                    progress(0, 'write', { baud });
                    writing = true;
                    await loader.writeFlash({
                        fileArray: files, flashMode: 'keep', flashFreq: 'keep', flashSize: 'keep', eraseAll: false, compress: true,
                        calculateMD5Hash: md5Hex,
                        reportProgress: (index, written, size) => {
                            if (o.blob && files[index] && files[index].data === o.blob && size > 0 && written >= size) blobWritten = true;
                            const before = files.slice(0, index).reduce((sum, f) => sum + f.data.length, 0);
                            const part = size > 0 ? Math.min(1, written / size) : 0;
                            progress((before + part * files[index].data.length) / total, 'write', { baud });
                        }
                    });
                    progress(1, 'reset', { baud });
                    // Without the monitor's hand-off (or once the window is gone) esptool resets the board itself.
                    const after = o.monitor && !aborted() ? 'no_reset_stub' : 'hard_reset';
                    await loader.after(after);
                    await safeDisconnect(transport);
                    return { baud, chip: description, mode: chosen, mac, after };
                } catch (err) {
                    await safeDisconnect(transport);
                    lastError = err;
                    if (FATAL.has(codeOf(err))) break;
                    if (nextBaud(baud)) log('Flashing at ' + baud + ' baud failed (' + (err && err.message) + '); trying ' + nextBaud(baud) + ' baud.');
                }
            }
        } finally {
            await lease.release();
        }
        const code = codeOf(lastError);
        const failure = code && !(lastError && lastError.code === code) ? flashError(code, lastError && lastError.message) : (lastError || flashError('FLASH_FAILED', 'flashing failed'));
        if (failure && typeof failure === 'object') failure.blobWritten = blobWritten;
        throw failure;
    }

    // resetBoard restarts the board into its app once through RTS (DTR released) on a short lease.
    async function resetBoard(manager, onLog) {
        let lease = null;
        try {
            lease = await manager.acquire('flasher', { revocable: false });
            await lease.open(ROM_BAUD);
            await lease.setSignals({ dataTerminalReady: false, requestToSend: false });
            await lease.reset();
            return true;
        } catch (err) {
            if (typeof onLog === 'function') onLog('Restarting the board failed: ' + ((err && err.message) || err));
            return false;
        } finally {
            if (lease) await lease.release();
        }
    }

    // handOff ends a flash that stayed in the stub (after: no_reset_stub): the monitor opens the port
    // and resets the board; when it cannot (no monitor or dock, the window is gone, connect failed) the
    // board is reset once here. o: {result, aborted(), monitor, dock, manager, onLog}. Returns
    // 'monitor', 'reset' or '' (nothing to do or the reset failed).
    async function handOff(o) {
        if (!o.result || o.result.after !== 'no_reset_stub') return '';
        if (!o.aborted() && o.monitor && o.dock) {
            try {
                o.dock.show('monitor');
                if (await o.monitor.connect({ reset: true })) return 'monitor';
            } catch (err) {
                console.error('cyd-creator monitor hand-off failed', err);
            }
        }
        return (await resetBoard(o.manager, o.onLog)) ? 'reset' : '';
    }

    // ── per-viewer memory of the build on the board (backtrace decoding) ──
    function readStore(key) {
        try { return JSON.parse(localStorage.getItem(key) || '{}') || {}; } catch (_) { return {}; }
    }
    function writeStore(key, value) {
        try { localStorage.setItem(key, JSON.stringify(value)); } catch (_) { /* storage may be blocked */ }
    }
    function lastFlashed(projectId) { return readStore(FLASHED_KEY)[projectId] || ''; }
    function rememberFlashed(projectId, buildId) {
        const all = readStore(FLASHED_KEY);
        all[projectId] = buildId;
        writeStore(FLASHED_KEY, all);
    }

    // ── per-viewer board records (D1): which boards (MAC) this project flashed, with chip, board
    // profile and the device it minted. Server device rows carry no MAC, so the record lives here.
    function rememberBoard(projectId, info) {
        const mac = normalizeMac(info && info.mac);
        if (!projectId || !mac) return;
        const all = readStore(BOARDS_KEY);
        const boards = all[projectId] && typeof all[projectId] === 'object' ? all[projectId] : {};
        boards[mac] = { chip: normalizeChip(info.chip), board: String(info.board || ''), device: String(info.device || ''), at: Date.now() };
        const newest = Object.keys(boards).sort((a, b) => (boards[b].at || 0) - (boards[a].at || 0)).slice(0, MAX_BOARDS);
        all[projectId] = Object.fromEntries(newest.map(key => [key, boards[key]]));
        writeStore(BOARDS_KEY, all);
    }
    function boardRecord(projectId, mac) {
        const key = normalizeMac(mac);
        const boards = key ? readStore(BOARDS_KEY)[projectId] : null;
        const entry = boards && typeof boards === 'object' && Object.prototype.hasOwnProperty.call(boards, key) ? boards[key] : null;
        return entry && typeof entry === 'object' ? entry : null;
    }
    function knownBoard(projectId, info) {
        const entry = boardRecord(projectId, info && info.mac);
        return !!entry && entry.chip === normalizeChip(info.chip) && entry.chip !== '' && entry.board === String(info.board || '');
    }
    // forgetBoard drops the board from every project's records before a full write: until that write
    // succeeds, nobody may treat the board's bootloader and partition table as known.
    function forgetBoard(mac) {
        const key = normalizeMac(mac);
        if (!key) return;
        const all = readStore(BOARDS_KEY);
        let changed = false;
        Object.keys(all).forEach(projectId => {
            const boards = all[projectId];
            if (boards && typeof boards === 'object' && Object.prototype.hasOwnProperty.call(boards, key)) {
                delete boards[key];
                changed = true;
            }
        });
        if (changed) writeStore(BOARDS_KEY, all);
    }

    // prepareWrite runs right before the first write: it returns the device the board's record of this
    // project names (revoked once the new flash succeeded) and drops the board's records for a full write.
    function prepareWrite(projectId, mode, mac) {
        const previous = (boardRecord(projectId, mac) || {}).device || '';
        if (mode === 'full') forgetBoard(mac);
        return previous;
    }

    // recordFlash stores a successful flash (build for backtrace decoding, board record) and revokes the
    // board's earlier device of this project: its config partition no longer holds that token, and every
    // link flash mints a new device. o: {projectId, buildId, result, chip, board, device, previousDevice,
    // revoke(id)}. Returns the revoked device id ('' when none).
    function recordFlash(o) {
        rememberFlashed(o.projectId, o.buildId);
        rememberBoard(o.projectId, { mac: o.result && o.result.mac, chip: o.chip, board: o.board, device: o.device });
        if (!o.previousDevice || o.previousDevice === o.device || typeof o.revoke !== 'function') return '';
        Promise.resolve().then(() => o.revoke(o.previousDevice)).catch(() => { /* already gone or offline: the row expires with the project */ });
        return o.previousDevice;
    }

    // ── dialog ──
    function greenBuilds(inst) {
        const list = (inst.dock && inst.dock.builds.list()) || [];
        return list.filter(b => b.status === 'succeeded' && (b.artifacts || []).some(a => a.name === 'merged.bin'));
    }
    function artifact(build, name) {
        return (build.artifacts || []).find(a => a.name === name) || null;
    }
    function buildLabel(inst, build, index, count) {
        return inst.t('cyd_creator.flash.build_option', { n: count - index, time: P.format.time(build.finished_at || build.created_at) });
    }

    function open(inst) {
        const { el } = P.util;
        const t = inst.t;
        const manager = P.serial.manager;
        const projectId = inst.projectId;
        const builds = greenBuilds(inst);
        const layout = (inst.layout() || {}).project || {};
        const settings = layout.settings || {};
        const board = ((inst.caps && inst.caps.boards) || []).find(b => b.id === (inst.project && inst.project.board)) || {};
        // The layout's setting only drives the hint; the flash decides from the build's image.
        const wantsLink = !!settings.aurago_link;
        const ota = () => (inst.caps && inst.caps.ota) || {};
        // Minting needs write access and OTA on; both can change while the dialog is open.
        const mayMint = () => !inst.readonly && ota().enabled !== false;
        const linkAllowed = () => wantsLink && mayMint();
        const state = { busy: false, minted: null, done: false, previousDevice: '' };

        const buildSelect = el('select', { class: 'cydc-select', dataset: { ccFlashBuild: '' }, 'aria-label': t('cyd_creator.flash.build') },
            builds.map((b, i) => el('option', { value: b.id, text: buildLabel(inst, b, i, builds.length) })));
        const modeSelect = el('select', { class: 'cydc-select', dataset: { ccFlashMode: '' }, 'aria-label': t('cyd_creator.flash.mode') },
            MODES.map(mode => el('option', { value: mode, text: t(MODE_LABELS[mode]) })));
        const modeHint = el('p', { class: 'cydc-form-help', dataset: { ccFlashModeHint: '' } });
        const modeChosen = el('p', { class: 'cydc-form-help', hidden: true, dataset: { ccFlashModeChosen: '' } });
        const monitorBox = el('input', { type: 'checkbox', checked: true, dataset: { ccFlashMonitor: '' } });
        const phase = el('p', { class: 'cydc-flasher-phase', role: 'status', dataset: { ccFlashPhase: 'idle' }, text: t('cyd_creator.flash.ready') });
        const bar = el('progress', { class: 'cydc-progress', max: '100', value: '0', dataset: { ccFlashProgress: '' }, 'aria-label': t('cyd_creator.flash.progress') });
        const logPre = el('pre', { class: 'cydc-flasher-log', dataset: { ccFlashLog: '' } });
        const download = el('a', { class: 'cydc-btn cydc-btn--ghost', dataset: { ccFlashAction: 'download' }, download: '', text: t('cyd_creator.flash.download') });
        const container = el('div', { class: 'cydc-flasher' });
        const syncDownload = () => {
            const build = builds.find(b => b.id === buildSelect.value);
            download.hidden = !build;
            if (build) {
                download.href = P.api.artifactUrl(build.id, 'merged.bin');
                download.setAttribute('download', (inst.project ? inst.project.slug : 'cyd') + '-' + build.id + '-merged.bin');
            }
        };
        const syncMode = () => {
            modeHint.textContent = t(MODE_HINTS[modeSelect.value] || MODE_HINTS.auto);
            modeChosen.hidden = true;
        };
        buildSelect.addEventListener('change', syncDownload);
        modeSelect.addEventListener('change', syncMode);
        syncMode();

        function linkNote(link) {
            if (wantsLink && inst.readonly) return el('p', { class: 'cydc-banner', dataset: { ccFlashLink: 'readonly' }, text: t('cyd_creator.flash.link_readonly') });
            if (wantsLink && ota().enabled === false) return el('p', { class: 'cydc-banner cydc-banner--warn', dataset: { ccFlashLink: 'off' }, text: t('cyd_creator.flash.link_ota_off') });
            if (!link) return null;
            return el('div', { class: 'cydc-banner', dataset: { ccFlashLink: 'on' } },
                el('span', { text: t('cyd_creator.flash.link_on', { url: ota().public_url || '' }) }),
                ota().insecure ? el('strong', { class: 'cydc-flasher-insecure', text: t('cyd_creator.flash.link_insecure') }) : null);
        }

        // render (re)builds the body: on open, on a desktop serial policy change and on a read-only
        // change while nothing runs (D2). The controls keep their state across renders.
        function render() {
            const supported = manager.support();
            const body = [];
            if (!supported.ok) {
                // A policy reason (the admin switch, a read-only desktop) is not the browser's fault.
                const browser = supported.reason !== 'disabled' && supported.reason !== 'readonly';
                body.push(el('div', { class: 'cydc-banner cydc-banner--warn', role: 'alert', dataset: { ccFlashUnsupported: supported.reason } },
                    browser ? el('strong', { text: t('cyd_creator.error.CYD_SERIAL_UNSUPPORTED') }) : null,
                    el('span', { text: t('cyd_creator.flash.unsupported_' + supported.reason) })));
            }
            if (!builds.length) {
                body.push(el('p', { class: 'cydc-empty', dataset: { ccFlashNoBuild: '' }, text: t('cyd_creator.flash.no_build') }));
            } else {
                body.push(el('label', { class: 'cydc-form-field' }, el('span', { class: 'cydc-form-label', text: t('cyd_creator.flash.build') }), buildSelect));
                const note = linkNote(linkAllowed());
                if (note && supported.ok) body.push(note);
                if (supported.ok) {
                    body.push(el('label', { class: 'cydc-form-field' }, el('span', { class: 'cydc-form-label', text: t('cyd_creator.flash.mode') }), modeSelect), modeHint, modeChosen);
                    body.push(el('label', { class: 'cydc-checkline' }, monitorBox, el('span', { text: t('cyd_creator.flash.open_monitor') })));
                    body.push(el('p', { class: 'cydc-form-help', text: t('cyd_creator.flash.hint') }));
                    body.push(phase, bar, el('details', { class: 'cydc-flasher-details' }, el('summary', { text: t('cyd_creator.flash.details') }), logPre));
                } else {
                    body.push(el('p', { class: 'cydc-form-help', text: t('cyd_creator.flash.download_hint') }));
                }
            }
            container.replaceChildren(...body);
            syncFlashButton(supported);
        }
        function syncFlashButton(supported) {
            const flashBtn = d && d.el.querySelector('[data-cc-dialog-action="flash"]');
            if (flashBtn) flashBtn.hidden = !(builds.length && (supported || manager.support()).ok);
        }

        const actions = [{ id: 'close', label: t('cyd_creator.app.close') }];
        if (builds.length) actions.push({ id: 'flash', label: t('cyd_creator.flash.start'), kind: 'primary' });
        let d = null;
        render();
        d = P.dialog.open(inst.root, {
            title: t('cyd_creator.flash.title'),
            className: 'cydc-dialog--wide cydc-flasher-dialog',
            body: container,
            actions,
            owner: inst.projectBag || inst.bag,
            onAction: (id, dialog) => {
                if (id === 'close') { if (!state.busy) dialog.close('close'); return; }
                if (id === 'flash' && !state.busy) start(dialog);
            }
        });
        syncFlashButton();
        if (builds.length) {
            d.el.querySelector('.cydc-dialog-actions').prepend(download);
            syncDownload();
        }
        // While flashing, Escape and the backdrop must not close the dialog (the board would be left
        // half-written; recoverable, but confusing).
        const backdrop = d.el.parentElement;
        const guardKey = event => { if (state.busy && event.key === 'Escape') { event.preventDefault(); event.stopImmediatePropagation(); } };
        const guardMouse = event => { if (state.busy && event.target === backdrop) event.stopImmediatePropagation(); };
        window.addEventListener('keydown', guardKey, true);
        backdrop.addEventListener('mousedown', guardMouse, true);
        const offPolicy = manager.on(type => { if (type === 'policy' && !state.busy && !d.closed) render(); });
        const offReadonly = inst.bus && typeof inst.bus.on === 'function'
            ? inst.bus.on('readonly:changed', () => { if (!state.busy && !d.closed) render(); })
            : () => {};
        d.promise.then(() => {
            window.removeEventListener('keydown', guardKey, true);
            backdrop.removeEventListener('mousedown', guardMouse, true);
            offPolicy();
            offReadonly();
        });

        // aborted: the dialog's owner (window or project) went away while a flash ran. The user cannot
        // close the dialog while busy, so a closed dialog means teardown.
        const aborted = () => d.closed || inst.disposed || inst.projectId !== projectId;

        function setPhase(key, vars, fraction) {
            phase.dataset.ccFlashPhase = key;
            phase.textContent = t('cyd_creator.flash.phase_' + key, vars);
            if (typeof fraction === 'number') bar.value = String(Math.round(fraction * 100));
        }
        function appendLog(line) {
            logPre.textContent = (logPre.textContent + line + '\n').slice(-20000);
        }
        function setBusy(dialog, busy) {
            state.busy = busy;
            dialog.setBusy(busy);
            buildSelect.disabled = busy;
            modeSelect.disabled = busy;
            monitorBox.disabled = busy;
            download.classList.toggle('is-disabled', busy);
        }
        function showChosen(mode) {
            modeChosen.textContent = t(mode === 'keep' ? 'cyd_creator.flash.mode_chosen_keep' : 'cyd_creator.flash.mode_chosen_full');
            modeChosen.hidden = false;
        }

        async function start(dialog) {
            const build = builds.find(b => b.id === buildSelect.value);
            if (!build) return;
            const mode = MODES.includes(modeSelect.value) ? modeSelect.value : 'auto';
            dialog.setError('');
            setBusy(dialog, true);
            state.minted = null;
            state.done = false;
            state.previousDevice = '';
            try {
                // requestPort needs the click's user activation: ask for the port first.
                setPhase('port', {}, 0);
                const port = await manager.request();
                if (aborted()) return;
                if (!port) { setPhase('cancelled', {}, 0); return; }
                setPhase('download', {}, 0);
                const images = await fetchImages(build, mode);
                if (aborted()) throw flashError('FLASH_ABORTED', 'window closed');
                // The BUILD decides the link (its image), not the layout as it is now.
                const buildLink = hasLink(images.app || images.merged);
                appendLog('AuraGo link in this build: ' + (buildLink ? 'yes' : 'no'));
                let blob = null;
                let offset = 0;
                if (buildLink && mayMint()) {
                    setPhase('device', {}, 0);
                    state.minted = await P.api.mintDevice(projectId, build.id);
                    if (aborted()) throw flashError('FLASH_ABORTED', 'window closed');
                    blob = base64Bytes(state.minted.config_blob);
                    offset = configAddress(state.minted.config_offset, 'the device config');
                    if (board.config_offset && Number(board.config_offset) !== offset) {
                        throw flashError('FLASH_FAILED', 'the device config offset ' + offset + ' differs from the board profile (' + board.config_offset + ')');
                    }
                }
                const result = await runFlash({
                    manager, image: images.merged, app: images.app, mode, blob, blobOffset: offset, configOffset: board.config_offset, chip: board.chip,
                    // The stub hand-off needs a monitor; handOff below resets the board when it cannot happen.
                    monitor: monitorBox.checked && !!inst.monitor,
                    isKnownBoard: info => knownBoard(projectId, { mac: info.mac, chip: board.chip, board: board.id }),
                    onMode: (chosen, info) => {
                        state.previousDevice = prepareWrite(projectId, chosen, info.mac);
                        if (!aborted()) showChosen(chosen);
                    },
                    aborted,
                    onLog: appendLog,
                    onProgress: (fraction, key, info) => { if (!aborted()) setPhase(key, { baud: info.baud, percent: Math.round(fraction * 100) }, fraction); }
                });
                state.done = true;
                recordFlash({
                    projectId, buildId: build.id, result, chip: board.chip, board: board.id, previousDevice: state.previousDevice,
                    device: state.minted && state.minted.device ? state.minted.device.id : '', revoke: id => P.api.revokeDevice(id)
                });
                if (!aborted()) setPhase('done', {}, 1);
                // The dock and the monitor belong to the project view; teardown may have removed them.
                await handOff({ result, aborted, monitor: inst.monitor, dock: inst.dock, manager, onLog: appendLog });
            } catch (err) {
                // A transferred blob means the board holds the new token: keep that device.
                if (!state.done && !(err && err.blobWritten) && state.minted && state.minted.device) {
                    P.api.revokeDevice(state.minted.device.id).catch(() => {});
                }
                if (aborted()) return;
                setPhase('failed', {}, Number(bar.value) / 100);
                dialog.setError(errorText(err));
            } finally {
                setBusy(dialog, false);
            }
        }

        // fetchImages downloads what mode needs: merged.bin (full, automatic) and firmware.bin (keep,
        // automatic when the build has it), each checked against its SHA-256.
        async function fetchImages(build, mode) {
            const hasApp = !!artifact(build, 'firmware.bin');
            if (mode === 'keep' && !hasApp) throw flashError('FLASH_FAILED', 'this build has no firmware.bin');
            const merged = mode === 'keep' ? null : await fetchImage(build, 'merged.bin');
            const app = mode !== 'full' && hasApp ? await fetchImage(build, 'firmware.bin') : null;
            return { merged, app };
        }

        async function fetchImage(build, name) {
            const info = artifact(build, name);
            const resp = await fetch(P.api.artifactUrl(build.id, name), { credentials: 'same-origin', cache: 'no-store' });
            if (!resp.ok) {
                let data = null;
                try { data = await resp.json(); } catch (_) { data = null; }
                throw P.api.toError(resp.status, data);
            }
            const bytes = new Uint8Array(await resp.arrayBuffer());
            await verifyImage(bytes, info && info.sha256, resp.headers.get('X-CYD-SHA256'));
            return bytes;
        }

        function errorText(err) {
            const code = codeOf(err);
            const details = (err && err.details) || {};
            const reason = typeof details.reason === 'string' ? details.reason : '';
            if (code === 'IMAGE_CHECK') return t('cyd_creator.flash.error_image');
            if (code === 'CYD_SERIAL_UNSUPPORTED' && reason) return t('cyd_creator.flash.unsupported_' + reason);
            // D4: the server cannot give the board an address or a certificate pin (MintDevice).
            if (code === 'CYD_BAD_REQUEST' && reason === 'device_url') return t('cyd_creator.flash.error_device_url');
            if (code === 'CYD_BAD_REQUEST' && (reason === 'tls_pin' || reason === 'tls_certificate')) return t('cyd_creator.flash.error_tls_pin');
            if (err && err.name === 'ApiError') return P.errors.message(t, err);
            if (code && t.has('cyd_creator.error.' + code)) return t('cyd_creator.error.' + code);
            return t('cyd_creator.flash.error_failed', { message: String((err && err.message) || err || '') });
        }
        return d;
    }

    function base64Bytes(text) {
        const raw = atob(String(text || ''));
        const out = new Uint8Array(raw.length);
        for (let i = 0; i < raw.length; i++) out[i] = raw.charCodeAt(i);
        return out;
    }

    P.flasher = {
        open, runFlash, setLoader, loadEsptool, nextBaud, normalizeChip, chipMatches, sha256Hex, verifyImage, md5Hex,
        lastFlashed, rememberFlashed, rememberBoard, knownBoard, boardRecord, forgetBoard, prepareWrite, recordFlash, pickMode, flashFiles, hasLink,
        samePartitionTable, resetBoard, handOff, base64Bytes, BAUDS, ESPTOOL_URL, APP_OFFSET, OTADATA_OFFSET, OTADATA_SIZE,
        CONFIG_SIZE, PART_TABLE_OFFSET, PART_TABLE_SIZE
    };
})();
