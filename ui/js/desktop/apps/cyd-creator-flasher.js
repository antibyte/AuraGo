// CYD Creator flasher (plan 5): the Flash dialog writes a green build to the board over Web Serial with
// the vendored esptool-js (ui/js/vendor/esptool-js/, imported lazily). Flow (spec §14): pick a build
// (default: the latest green one), mint a device + config blob when the project has the AuraGo link and
// the user may write, choose the port, connect, compare the chip with the board profile, download
// merged.bin and check its SHA-256 in the browser, write it at 0x0 (+ the blob at config_offset) at
// 921600 baud with fallbacks to 460800 and 115200, verify (MD5 read back by esptool), reset and hand the
// port to the serial monitor. Without Web Serial the dialog offers "Download image". runFlash and the
// helpers are pure enough for Node; the esptool loader is injectable (P.flasher.setLoader) for tests.
// Install modes (decision D1): "Full install" writes merged.bin at 0x0 (wipes the NVS: touch calibration
// and Wi-Fi); "Keep settings" writes firmware.bin at app0 (0x10000) and erases otadata (8 KiB 0xFF at
// 0xE000, so app0 boots even after an OTA into app1). "Automatic" (default) keeps the settings when this
// browser's board record says this project flashed the connected board (MAC, chip and board profile).
(function () {
    'use strict';

    const P = window.CYDCreatorParts = window.CYDCreatorParts || {};
    const ESPTOOL_URL = '/js/vendor/esptool-js/esptool-js.min.mjs';
    const BAUDS = [921600, 460800, 115200];
    const ROM_BAUD = 115200;
    const FLASHED_KEY = 'aurago.cydcreator.flashed.v1';
    const BOARDS_KEY = 'aurago.cydcreator.boards.v1';
    const MAX_BOARDS = 32;
    // Every CYD Creator partition table (internal/cydcreator/partitions.go) shares these offsets.
    const APP_OFFSET = 0x10000;
    const OTADATA_OFFSET = 0xE000;
    const OTADATA_SIZE = 0x2000;
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

    // flashFiles returns esptool's fileArray for mode: keep = firmware.bin at app0 + erased otadata,
    // full = merged.bin at 0x0; the config blob (AuraGo link) follows at its offset in both.
    function flashFiles(mode, o) {
        let files;
        if (mode === 'keep') {
            if (!o.app || !o.app.length) throw flashError('FLASH_FAILED', 'firmware.bin is missing: keep settings needs the app image');
            files = [{ data: o.app, address: APP_OFFSET }, { data: new Uint8Array(OTADATA_SIZE).fill(0xff), address: OTADATA_OFFSET }];
        } else {
            if (!o.image || !o.image.length) throw flashError('FLASH_FAILED', 'merged.bin is missing');
            files = [{ data: o.image, address: 0 }];
        }
        if (o.blob) files.push({ data: o.blob, address: o.blobOffset });
        return files;
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

    // runFlash writes the image (and the optional config blob) through esptool. o: {manager, load,
    // image (merged.bin), app (firmware.bin), mode ('full' default, 'keep', 'auto'), isKnownBoard({mac,
    // chip}), onMode(mode, {mac}), blob, blobOffset, chip, monitor, aborted(), onProgress(fraction,
    // phase, info), onLog(line)}. The mode is decided once, on the first connect (the MAC is known
    // then). It tries every rate of BAUDS before giving up; a wrong chip, a lost board, a busy port or
    // aborted() (the window closed) end it at once. The flasher holds the port exclusively and releases
    // it before it returns. Result: {baud, chip, mode, mac}.
    async function runFlash(o) {
        const esptool = await (o.load || loadEsptool)();
        const lease = await o.manager.acquire('flasher', { revocable: false });
        const progress = (fraction, phase, info) => { if (typeof o.onProgress === 'function') o.onProgress(fraction, phase, info || {}); };
        const log = line => { if (typeof o.onLog === 'function') o.onLog(String(line)); };
        const checkAbort = () => { if (typeof o.aborted === 'function' && o.aborted()) throw flashError('FLASH_ABORTED', 'the flash was aborted (window closed)'); };
        const terminal = { clean() {}, writeLine: log, write: log };
        let files = null;
        let total = 0;
        let chosen = '';
        let mac = '';
        let lastError = null;
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
                        const known = !!o.app && typeof o.isKnownBoard === 'function' && !!mac && !!o.isKnownBoard({ mac, chip: chipName });
                        chosen = pickMode(o.mode, known);
                        files = flashFiles(chosen, o);
                        total = files.reduce((sum, f) => sum + f.data.length, 0);
                        log('Install mode: ' + chosen + (mac ? ' (board ' + mac + ')' : ''));
                        if (typeof o.onMode === 'function') o.onMode(chosen, { mac });
                    }
                    progress(0, 'write', { baud });
                    await loader.writeFlash({
                        fileArray: files, flashMode: 'keep', flashFreq: 'keep', flashSize: 'keep', eraseAll: false, compress: true,
                        calculateMD5Hash: md5Hex,
                        reportProgress: (index, written, size) => {
                            checkAbort();
                            const before = files.slice(0, index).reduce((sum, f) => sum + f.data.length, 0);
                            const part = size > 0 ? Math.min(1, written / size) : 0;
                            progress((before + part * files[index].data.length) / total, 'write', { baud });
                        }
                    });
                    checkAbort();
                    progress(1, 'reset', { baud });
                    await loader.after(o.monitor ? 'no_reset_stub' : 'hard_reset');
                    await safeDisconnect(transport);
                    return { baud, chip: description, mode: chosen, mac };
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
        if (code) throw (lastError && lastError.code === code) ? lastError : flashError(code, lastError && lastError.message);
        throw lastError || flashError('FLASH_FAILED', 'flashing failed');
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
    function knownBoard(projectId, info) {
        const mac = normalizeMac(info && info.mac);
        const entry = mac ? (readStore(BOARDS_KEY)[projectId] || {})[mac] : null;
        return !!entry && entry.chip === normalizeChip(info.chip) && entry.chip !== '' && entry.board === String(info.board || '');
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
        const wantsLink = !!settings.aurago_link;
        const ota = () => (inst.caps && inst.caps.ota) || {};
        // The link needs write access and OTA on; both can change while the dialog is open.
        const linkAllowed = () => wantsLink && !inst.readonly && ota().enabled !== false;
        const state = { busy: false, minted: null, done: false };

        const buildSelect = el('select', { class: 'cc-select', dataset: { ccFlashBuild: '' }, 'aria-label': t('cyd_creator.flash.build') },
            builds.map((b, i) => el('option', { value: b.id, text: buildLabel(inst, b, i, builds.length) })));
        const modeSelect = el('select', { class: 'cc-select', dataset: { ccFlashMode: '' }, 'aria-label': t('cyd_creator.flash.mode') },
            MODES.map(mode => el('option', { value: mode, text: t(MODE_LABELS[mode]) })));
        const modeHint = el('p', { class: 'cc-form-help', dataset: { ccFlashModeHint: '' } });
        const modeChosen = el('p', { class: 'cc-form-help', hidden: true, dataset: { ccFlashModeChosen: '' } });
        const monitorBox = el('input', { type: 'checkbox', checked: true, dataset: { ccFlashMonitor: '' } });
        const phase = el('p', { class: 'cc-flasher-phase', role: 'status', dataset: { ccFlashPhase: 'idle' }, text: t('cyd_creator.flash.ready') });
        const bar = el('progress', { class: 'cc-progress', max: '100', value: '0', dataset: { ccFlashProgress: '' }, 'aria-label': t('cyd_creator.flash.progress') });
        const logPre = el('pre', { class: 'cc-flasher-log', dataset: { ccFlashLog: '' } });
        const download = el('a', { class: 'cc-btn cc-btn--ghost', dataset: { ccFlashAction: 'download' }, download: '', text: t('cyd_creator.flash.download') });
        const container = el('div', { class: 'cc-flasher' });
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
            if (wantsLink && inst.readonly) return el('p', { class: 'cc-banner', dataset: { ccFlashLink: 'readonly' }, text: t('cyd_creator.flash.link_readonly') });
            if (wantsLink && ota().enabled === false) return el('p', { class: 'cc-banner cc-banner--warn', dataset: { ccFlashLink: 'off' }, text: t('cyd_creator.flash.link_ota_off') });
            if (!link) return null;
            return el('div', { class: 'cc-banner', dataset: { ccFlashLink: 'on' } },
                el('span', { text: t('cyd_creator.flash.link_on', { url: ota().public_url || '' }) }),
                ota().insecure ? el('strong', { class: 'cc-flasher-insecure', text: t('cyd_creator.flash.link_insecure') }) : null);
        }

        // render (re)builds the body: on open, on a desktop serial policy change and on a read-only
        // change while nothing runs (D2). The controls keep their state across renders.
        function render() {
            const supported = manager.support();
            const body = [];
            if (!supported.ok) {
                // A policy reason (the admin switch, a read-only desktop) is not the browser's fault.
                const browser = supported.reason !== 'disabled' && supported.reason !== 'readonly';
                body.push(el('div', { class: 'cc-banner cc-banner--warn', role: 'alert', dataset: { ccFlashUnsupported: supported.reason } },
                    browser ? el('strong', { text: t('cyd_creator.error.CYD_SERIAL_UNSUPPORTED') }) : null,
                    el('span', { text: t('cyd_creator.flash.unsupported_' + supported.reason) })));
            }
            if (!builds.length) {
                body.push(el('p', { class: 'cc-empty', dataset: { ccFlashNoBuild: '' }, text: t('cyd_creator.flash.no_build') }));
            } else {
                body.push(el('label', { class: 'cc-form-field' }, el('span', { class: 'cc-form-label', text: t('cyd_creator.flash.build') }), buildSelect));
                const note = linkNote(linkAllowed());
                if (note && supported.ok) body.push(note);
                if (supported.ok) {
                    body.push(el('label', { class: 'cc-form-field' }, el('span', { class: 'cc-form-label', text: t('cyd_creator.flash.mode') }), modeSelect), modeHint, modeChosen);
                    body.push(el('label', { class: 'cc-checkline' }, monitorBox, el('span', { text: t('cyd_creator.flash.open_monitor') })));
                    body.push(el('p', { class: 'cc-form-help', text: t('cyd_creator.flash.hint') }));
                    body.push(phase, bar, el('details', { class: 'cc-flasher-details' }, el('summary', { text: t('cyd_creator.flash.details') }), logPre));
                } else {
                    body.push(el('p', { class: 'cc-form-help', text: t('cyd_creator.flash.download_hint') }));
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
            className: 'cc-dialog--wide cc-flasher-dialog',
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
            d.el.querySelector('.cc-dialog-actions').prepend(download);
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
            try {
                // requestPort needs the click's user activation: ask for the port first.
                setPhase('port', {}, 0);
                const port = await manager.request();
                if (aborted()) return;
                if (!port) { setPhase('cancelled', {}, 0); return; }
                let blob = null;
                let offset = 0;
                if (linkAllowed()) {
                    setPhase('device', {}, 0);
                    state.minted = await P.api.mintDevice(projectId, build.id);
                    if (aborted()) throw flashError('FLASH_ABORTED', 'window closed');
                    blob = base64Bytes(state.minted.config_blob);
                    offset = Number(state.minted.config_offset) || 0;
                }
                setPhase('download', {}, 0);
                const images = await fetchImages(build, mode);
                if (aborted()) throw flashError('FLASH_ABORTED', 'window closed');
                const result = await runFlash({
                    manager, image: images.merged, app: images.app, mode, blob, blobOffset: offset, chip: board.chip, monitor: monitorBox.checked,
                    isKnownBoard: info => knownBoard(projectId, { mac: info.mac, chip: board.chip, board: board.id }),
                    onMode: chosen => { if (!aborted()) showChosen(chosen); },
                    aborted,
                    onLog: appendLog,
                    onProgress: (fraction, key, info) => { if (!aborted()) setPhase(key, { baud: info.baud, percent: Math.round(fraction * 100) }, fraction); }
                });
                state.done = true;
                rememberFlashed(projectId, build.id);
                rememberBoard(projectId, { mac: result.mac, chip: board.chip, board: board.id, device: state.minted && state.minted.device ? state.minted.device.id : '' });
                if (aborted()) return;
                setPhase('done', {}, 1);
                // The dock and the monitor belong to the project view; teardown may have removed them.
                if (monitorBox.checked && inst.monitor && inst.dock) {
                    inst.dock.show('monitor');
                    await inst.monitor.connect({ reset: true });
                }
            } catch (err) {
                if (!state.done && state.minted && state.minted.device) {
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
        lastFlashed, rememberFlashed, rememberBoard, knownBoard, pickMode, flashFiles, base64Bytes, BAUDS, ESPTOOL_URL,
        APP_OFFSET, OTADATA_OFFSET, OTADATA_SIZE
    };
})();
