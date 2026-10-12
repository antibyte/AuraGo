// CYD Creator serial monitor (plan 5): the dock tab "Monitor" (inst.dock.registerTab) shows the board's
// serial output through the shared connection manager (P.serial.manager, revocable owner "monitor": a
// flash takes the port over and the flasher hands it back). Baud rate, timestamps, filter, pause, clear,
// a 5000-line ring buffer, reset through DTR/RTS, "Decode" on ESP32 "Backtrace:" lines (POST
// /builds/{id}/decode-backtrace against the build last flashed from this browser) and "Attach log"
// (≤ 16 KiB of the selected or visible lines) into the agent chat. Pure helpers run in Node.
// Connecting follows the desktop serial policy (decision D2, P.serial.support): off or read-only
// disconnects and shows why. The agent panel is resolved when "Attach log" is used (it may not exist yet
// when the monitor attaches).
(function () {
    'use strict';

    const P = window.CYDCreatorParts = window.CYDCreatorParts || {};
    const MAX_LINES = 5000;
    const ATTACH_MAX_BYTES = 16 * 1024;
    const BAUD_RATES = [9600, 57600, 115200, 230400, 460800, 921600];
    const DEFAULT_BAUD = 115200;
    const PARTIAL_FLUSH_MS = 250;
    const RENDER_MS = 80;
    const BACKTRACE = /Backtrace:((?:\s+0x[0-9a-fA-F]{8}:0x[0-9a-fA-F]{8})+)/;
    const PREFS_KEY = 'aurago.cydcreator.monitor.v1';
    const STATUS_TONES = { connected: 'ok', flashing: 'warning', lost: 'danger', disconnected: '' };

    // createRing keeps the newest max entries; dropped counts the removed ones.
    function createRing(max) {
        const limit = Math.max(1, Number(max) || MAX_LINES);
        let items = [];
        let dropped = 0;
        return {
            push(item) {
                items.push(item);
                if (items.length > limit) {
                    const extra = items.length - limit;
                    items = items.slice(extra);
                    dropped += extra;
                }
            },
            all: () => items.slice(),
            size: () => items.length,
            dropped: () => dropped,
            clear() { items = []; dropped = 0; }
        };
    }

    // createLineSplitter turns UTF-8 chunks into lines (CRLF and LF); flush() returns the partial line
    // a prompt without a newline leaves behind.
    function createLineSplitter() {
        const decoder = new TextDecoder('utf-8');
        let rest = '';
        return {
            push(chunk) {
                rest += decoder.decode(chunk, { stream: true });
                const parts = rest.split('\n');
                rest = parts.pop();
                return parts.map(line => line.replace(/\r$/, ''));
            },
            flush() {
                const line = rest.replace(/\r$/, '');
                rest = '';
                return line;
            },
            pending: () => rest.length > 0
        };
    }

    // parseBacktrace returns the program counters of an ESP32/ESP32-S3 panic "Backtrace:" line
    // (PC:SP pairs), at most 64, or [] for any other line.
    function parseBacktrace(line) {
        const m = BACKTRACE.exec(String(line || ''));
        if (!m) return [];
        return m[1].trim().split(/\s+/).map(pair => pair.split(':')[0].toLowerCase()).slice(0, 64);
    }

    function formatTime(ms) {
        const d = new Date(ms);
        const pad = (n, w) => String(n).padStart(w || 2, '0');
        return pad(d.getHours()) + ':' + pad(d.getMinutes()) + ':' + pad(d.getSeconds()) + '.' + pad(d.getMilliseconds(), 3);
    }

    function byteLength(text) { return new TextEncoder().encode(text).length; }

    // attachText joins lines and keeps their end within ATTACH_MAX_BYTES (starting at a line break).
    function attachText(lines) {
        const text = lines.join('\n');
        if (byteLength(text) <= ATTACH_MAX_BYTES) return text;
        if (P.agent && typeof P.agent.tailBytes === 'function') return P.agent.tailBytes(text, ATTACH_MAX_BYTES);
        let out = text;
        while (byteLength(out) > ATTACH_MAX_BYTES) out = out.slice(out.indexOf('\n') + 1 || 1);
        return out;
    }

    // unsupportedKey is the notice for a support() reason: the desktop policy has its own sentences.
    function unsupportedKey(reason) {
        return reason === 'disabled' || reason === 'readonly' ? 'cyd_creator.flash.unsupported_' + reason : 'cyd_creator.monitor.unsupported';
    }

    function readPrefs() {
        try { return JSON.parse(localStorage.getItem(PREFS_KEY) || '{}') || {}; } catch (_) { return {}; }
    }
    function writePrefs(prefs) {
        try { localStorage.setItem(PREFS_KEY, JSON.stringify(prefs)); } catch (_) { /* storage may be blocked */ }
    }

    // attach registers the Monitor tab for inst and returns the controller (inst.monitor). The serial
    // session lives in the controller, so output is buffered while the tab is hidden.
    function attach(inst) {
        const { el, bag } = P.util;
        const t = inst.t;
        const b = bag();
        const manager = P.serial.manager;
        const prefs = Object.assign({ baud: DEFAULT_BAUD, timestamps: false }, readPrefs());
        const ring = createRing(MAX_LINES);
        const state = { status: 'disconnected', lease: null, stopReader: null, splitter: null, paused: false, filter: '', seq: 0, rendered: 0, disposed: false, decoded: new Map() };
        let partialTimer = 0;
        let renderTimer = 0;

        const statusPill = el('span', { class: 'cc-pill', role: 'status', dataset: { ccMonitorStatus: 'disconnected' } });
        const connectBtn = el('button', { type: 'button', class: 'cc-btn cc-btn--small cc-btn--primary', dataset: { ccMonitorAction: 'connect' }, text: t('cyd_creator.monitor.connect') });
        const disconnectBtn = el('button', { type: 'button', class: 'cc-btn cc-btn--small', dataset: { ccMonitorAction: 'disconnect' }, text: t('cyd_creator.monitor.disconnect') });
        const resetBtn = el('button', { type: 'button', class: 'cc-btn cc-btn--small', dataset: { ccMonitorAction: 'reset' }, text: t('cyd_creator.monitor.reset') });
        const baudSelect = el('select', { class: 'cc-select', dataset: { ccMonitorBaud: '' }, 'aria-label': t('cyd_creator.monitor.baud') },
            BAUD_RATES.map(rate => el('option', { value: String(rate), selected: rate === Number(prefs.baud), text: t('cyd_creator.monitor.baud_option', { baud: rate }) })));
        const stampBox = el('input', { type: 'checkbox', checked: !!prefs.timestamps, dataset: { ccMonitorTimestamps: '' } });
        const filterInput = el('input', { type: 'search', class: 'cc-input cc-monitor-filter', placeholder: t('cyd_creator.monitor.filter'), 'aria-label': t('cyd_creator.monitor.filter'), dataset: { ccMonitorFilter: '' } });
        const pauseBtn = el('button', { type: 'button', class: 'cc-btn cc-btn--small', 'aria-pressed': 'false', dataset: { ccMonitorAction: 'pause' }, text: t('cyd_creator.monitor.pause') });
        const clearBtn = el('button', { type: 'button', class: 'cc-btn cc-btn--small cc-btn--ghost', dataset: { ccMonitorAction: 'clear' }, text: t('cyd_creator.monitor.clear') });
        const attachBtn = el('button', { type: 'button', class: 'cc-btn cc-btn--small', dataset: { ccMonitorAction: 'attach' }, text: t('cyd_creator.monitor.attach') });
        const notice = el('p', { class: 'cc-monitor-notice', hidden: true, dataset: { ccMonitorNotice: '' } });
        const lines = el('div', { class: 'cc-monitor-lines', role: 'log', 'aria-live': 'off', tabindex: '0', dataset: { ccMonitorLines: '' } });
        const empty = el('p', { class: 'cc-empty', dataset: { ccMonitorEmpty: '' }, text: t('cyd_creator.monitor.empty') });
        const panel = el('div', { class: 'cc-monitor' },
            el('div', { class: 'cc-log-head cc-monitor-head' }, statusPill, connectBtn, disconnectBtn, resetBtn, baudSelect,
                el('label', { class: 'cc-checkline' }, stampBox, el('span', { text: t('cyd_creator.monitor.timestamps') })),
                filterInput, el('span', { class: 'cc-dock-spacer' }), pauseBtn, clearBtn, attachBtn),
            notice, empty, lines);

        // The agent panel is created after the monitor (app.js): look it up when it is needed.
        const agent = () => (inst.agent && typeof inst.agent.attachLog === 'function' ? inst.agent : null);

        function setStatus(status, noticeKey, vars) {
            state.status = status;
            statusPill.dataset.ccMonitorStatus = status;
            if (STATUS_TONES[status]) statusPill.dataset.tone = STATUS_TONES[status];
            else delete statusPill.dataset.tone;
            statusPill.textContent = t('cyd_creator.monitor.status_' + status);
            const connected = status === 'connected';
            const support = manager.support();
            connectBtn.hidden = connected;
            connectBtn.disabled = !support.ok;
            disconnectBtn.hidden = !connected;
            resetBtn.disabled = !connected;
            attachBtn.hidden = !(agent() || (P.agent && typeof P.agent.create === 'function'));
            notice.hidden = !noticeKey && support.ok;
            notice.textContent = noticeKey ? t(noticeKey, vars) : (support.ok ? '' : t(unsupportedKey(support.reason)));
        }

        // ── lines ──
        function visible(entry) {
            return !state.filter || entry.text.toLowerCase().includes(state.filter);
        }
        function lineNode(entry) {
            const node = el('div', { class: 'cc-monitor-line', dataset: { ccMonitorLine: String(entry.id) } },
                prefs.timestamps ? el('span', { class: 'cc-monitor-time', text: formatTime(entry.time) + ' ' }) : null,
                el('span', { class: 'cc-monitor-text', text: entry.text }));
            const addresses = parseBacktrace(entry.text);
            if (addresses.length) {
                node.classList.add('is-backtrace');
                node.appendChild(el('button', { type: 'button', class: 'cc-btn cc-btn--small', dataset: { ccMonitorDecode: String(entry.id) }, text: t('cyd_creator.monitor.decode'), onClick: () => decode(entry, addresses, node) }));
                const known = state.decoded.get(entry.id);
                if (known) node.appendChild(known);
            }
            return node;
        }
        function renderAll() {
            const entries = ring.all().filter(visible);
            lines.replaceChildren(...entries.map(lineNode));
            state.rendered = state.seq;
            empty.hidden = ring.size() > 0;
            lines.scrollTop = lines.scrollHeight;
        }
        function renderNew() {
            renderTimer = 0;
            if (state.paused || state.disposed) return;
            const fresh = ring.all().filter(entry => entry.id > state.rendered && visible(entry));
            state.rendered = state.seq;
            const stick = lines.scrollHeight - lines.scrollTop - lines.clientHeight < 24;
            fresh.forEach(entry => lines.appendChild(lineNode(entry)));
            while (lines.childElementCount > MAX_LINES) lines.firstElementChild.remove();
            empty.hidden = ring.size() > 0;
            if (stick) lines.scrollTop = lines.scrollHeight;
        }
        function addLine(text) {
            state.seq += 1;
            ring.push({ id: state.seq, time: Date.now(), text });
            if (!renderTimer && !state.disposed) renderTimer = setTimeout(renderNew, RENDER_MS);
        }
        function onData(chunk) {
            if (!state.splitter) return;
            state.splitter.push(chunk).forEach(addLine);
            clearTimeout(partialTimer);
            if (state.splitter.pending()) {
                partialTimer = setTimeout(() => { if (state.splitter && state.splitter.pending()) addLine(state.splitter.flush()); }, PARTIAL_FLUSH_MS);
            }
        }

        async function decode(entry, addresses, node) {
            const buildId = (P.flasher && P.flasher.lastFlashed(inst.projectId)) || ((inst.lastGreenBuild() || {}).id) || '';
            const box = el('div', { class: 'cc-monitor-frames', dataset: { ccMonitorFrames: String(entry.id) } });
            const old = node.querySelector('.cc-monitor-frames');
            if (old) old.remove();
            node.appendChild(box);
            if (!buildId) { box.textContent = t('cyd_creator.monitor.decode_no_build'); return; }
            box.textContent = t('cyd_creator.monitor.decoding');
            try {
                const frames = await P.api.decodeBacktrace(buildId, addresses);
                box.replaceChildren(el('p', { class: 'cc-monitor-frames-head', text: t('cyd_creator.monitor.decoded_with', { build: buildId }) }),
                    el('ol', {}, frames.map(f => el('li', {},
                        el('code', { text: f.address }), ' ', el('strong', { text: f.function || '??' }),
                        f.file ? el('span', { class: 'cc-monitor-where', text: ' ' + f.file + (f.line ? ':' + f.line : '') }) : null))));
                state.decoded.set(entry.id, box);
            } catch (err) {
                box.textContent = P.errors.message(t, err);
            }
        }

        // ── serial session ──
        async function stopSession() {
            clearTimeout(partialTimer);
            const lease = state.lease;
            state.lease = null;
            if (state.stopReader) { const stop = state.stopReader; state.stopReader = null; await stop(); }
            if (lease) await lease.release();
            if (state.splitter && state.splitter.pending()) addLine(state.splitter.flush());
            state.splitter = null;
        }
        async function connect(opts) {
            const o = opts || {};
            if (state.disposed) return false;
            try {
                // D2: the desktop serial policy (loaded first when not known yet) decides before any port.
                const support = await manager.ensure();
                if (!support.ok) {
                    await stopSession();
                    if (!state.disposed) setStatus('disconnected', unsupportedKey(support.reason));
                    return false;
                }
                if (!manager.hasPort()) {
                    const port = await manager.request();
                    if (!port) return false;
                }
                if (state.disposed) return false;
                await stopSession();
                const lease = await manager.acquire('monitor', {
                    revocable: true,
                    onRevoke: async () => {
                        await stopSession();
                        if (!state.disposed) setStatus('flashing', 'cyd_creator.monitor.flashing');
                    }
                });
                state.lease = lease;
                await lease.open(Number(baudSelect.value) || DEFAULT_BAUD);
                await lease.setSignals({ dataTerminalReady: false, requestToSend: false });
                state.splitter = createLineSplitter();
                state.stopReader = lease.startReader(onData, (error, stopped) => {
                    if (stopped || state.disposed || state.lease !== lease) return;
                    stopSession().then(() => setStatus(error ? 'lost' : 'disconnected', error ? 'cyd_creator.error.CYD_FLASH_DISCONNECTED' : ''));
                });
                if (state.disposed) { await stopSession(); return false; }
                setStatus('connected');
                if (o.reset) await lease.reset();
                return true;
            } catch (err) {
                await stopSession();
                if (state.disposed) return false;
                const code = P.serial.classify(err);
                const reason = err && err.details && err.details.reason;
                const key = code === 'CYD_SERIAL_UNSUPPORTED' && reason ? unsupportedKey(reason) : (code ? 'cyd_creator.error.' + code : 'cyd_creator.monitor.connect_failed');
                setStatus('disconnected', key);
                return false;
            }
        }
        async function disconnect() {
            await stopSession();
            if (!state.disposed) setStatus('disconnected');
        }

        b.on(connectBtn, 'click', () => connect({}));
        b.on(disconnectBtn, 'click', disconnect);
        b.on(resetBtn, 'click', async () => {
            if (!state.lease) return;
            try { await state.lease.reset(); } catch (err) { setStatus(state.status, 'cyd_creator.error.CYD_FLASH_DISCONNECTED'); }
        });
        b.on(baudSelect, 'change', () => {
            prefs.baud = Number(baudSelect.value) || DEFAULT_BAUD;
            writePrefs(prefs);
            if (state.status === 'connected') connect({});
        });
        b.on(stampBox, 'change', () => { prefs.timestamps = stampBox.checked; writePrefs(prefs); renderAll(); });
        b.on(filterInput, 'input', () => { state.filter = filterInput.value.trim().toLowerCase(); renderAll(); });
        b.on(pauseBtn, 'click', () => {
            state.paused = !state.paused;
            pauseBtn.setAttribute('aria-pressed', state.paused ? 'true' : 'false');
            pauseBtn.textContent = t(state.paused ? 'cyd_creator.monitor.resume' : 'cyd_creator.monitor.pause');
            if (!state.paused) renderAll();
        });
        b.on(clearBtn, 'click', () => { ring.clear(); state.decoded.clear(); renderAll(); });
        b.on(attachBtn, 'click', () => {
            const target = agent();
            if (!target) return;
            const selection = typeof window.getSelection === 'function' ? window.getSelection() : null;
            const selected = selection && !selection.isCollapsed && lines.contains(selection.anchorNode) ? String(selection) : '';
            const text = selected ? attachText(selected.split('\n')) : attachText(ring.all().filter(visible).map(e => e.text));
            if (!text.trim()) { setStatus(state.status, 'cyd_creator.monitor.attach_empty'); return; }
            target.attachLog(text);
        });
        b.add(manager.on((type, detail) => {
            if (state.disposed) return;
            if (type === 'disconnect' && state.lease && detail === manager.port()) {
                stopSession().then(() => { if (!state.disposed) setStatus('lost', 'cyd_creator.error.CYD_FLASH_DISCONNECTED'); });
            } else if (type === 'policy') {
                // D2: a policy that no longer allows Web Serial ends the session, like Quick Connect.
                const support = manager.support();
                if (!support.ok && state.lease) stopSession().then(() => { if (!state.disposed) setStatus('disconnected', unsupportedKey(support.reason)); });
                else setStatus(state.status);
            } else if (type === 'owner' && detail === '' && state.status === 'flashing' && !state.lease) {
                // The flash ended without handing the port back (monitor toggle off, failure, abort).
                setStatus('disconnected');
            }
        }));

        inst.dock.registerTab('monitor', { labelKey: 'cyd_creator.dock.monitor', mount: host => { host.appendChild(panel); setStatus(state.status); renderAll(); } });
        setStatus('disconnected');

        const controller = {
            connect,
            disconnect,
            status: () => state.status,
            lines: () => ring.all().map(e => e.text),
            dispose() {
                state.disposed = true;
                clearTimeout(partialTimer);
                clearTimeout(renderTimer);
                stopSession();
                b.dispose();
            }
        };
        return controller;
    }

    P.monitor = { attach, createRing, createLineSplitter, parseBacktrace, attachText, formatTime, MAX_LINES, ATTACH_MAX_BYTES, BAUD_RATES };
})();
