// CYD Creator serial connection manager (plan 5): exactly one owner holds the Web Serial port at a time
// (flasher or monitor). It requests and remembers the port, opens and closes it, runs the reader loop,
// resets the board through DTR/RTS (classic ESP32 auto-reset circuit) and hands the port from the
// flasher to the monitor. Web Serial needs a secure context and the top-level /desktop document: the
// server denies it everywhere else through Permissions-Policy and iframe apps never get it, which is
// why CYD Creator is a builtin app. Pure parts (support, classify, the manager with a fake port) run in Node.
// Like Quick Connect's browser serial, flashing and the monitor also need the desktop's serial switch
// (virtual_desktop.serial_browser_enabled, admins only) on a desktop that is not read-only (decision D2).
// The manager covers CYD Creator only: a port another app (Quick Connect) holds open is CYD_FLASH_PORT_BUSY.
(function () {
    'use strict';

    const P = window.CYDCreatorParts = window.CYDCreatorParts || {};
    const RESET_PULSE_MS = 100;
    const BOOTSTRAP_URL = '/api/desktop/bootstrap';

    function serialError(code, message, cause, details) {
        const err = new Error(message || code);
        err.name = 'SerialError';
        err.code = code;
        if (cause) err.cause = cause;
        err.details = details || {};
        return err;
    }

    function sleep(ms) { return new Promise(resolve => setTimeout(resolve, ms)); }

    // ── desktop serial policy (D2) ──
    // serial is virtual_desktop.serial_browser_enabled as the server reports it for this user (off for
    // non-admins and on read-only desktops), readonly the desktop's read-only state; null = not known yet.
    // The desktop announces both with aurago:desktop-policy on document (welcome: the whole bootstrap; a
    // lost event socket: serial off until the next welcome). The first value comes from the bootstrap.
    const desktop = { serial: null, readonly: null, loading: null, listeners: new Set() };

    function policyState() { return { serial: desktop.serial, readonly: desktop.readonly }; }

    function applyPolicy(detail) {
        const d = detail || {};
        let changed = false;
        if (typeof d.serial_browser_enabled === 'boolean' && d.serial_browser_enabled !== desktop.serial) {
            desktop.serial = d.serial_browser_enabled;
            changed = true;
        }
        if (typeof d.readonly === 'boolean' && d.readonly !== desktop.readonly) {
            desktop.readonly = d.readonly;
            changed = true;
        }
        if (changed) {
            desktop.listeners.forEach(fn => {
                try { fn(policyState()); } catch (err) { console.error('cyd-creator serial policy listener failed', err); }
            });
        }
        return changed;
    }

    // loadPolicy reads the bootstrap once while the policy is unknown; events win over a late answer.
    function loadPolicy() {
        if (desktop.serial !== null) return Promise.resolve(policyState());
        if (!desktop.loading && typeof fetch === 'function') {
            desktop.loading = fetch(BOOTSTRAP_URL, { credentials: 'same-origin', cache: 'no-store' })
                .then(resp => (resp && resp.ok ? resp.json() : null))
                .then(body => { if (body && desktop.serial === null) applyPolicy(body); })
                .catch(() => { /* unknown stays off */ })
                .then(() => { desktop.loading = null; return policyState(); });
        }
        return desktop.loading || Promise.resolve(policyState());
    }

    if (typeof document !== 'undefined' && typeof document.addEventListener === 'function') {
        document.addEventListener('aurago:desktop-policy', event => applyPolicy(event && event.detail));
        if (typeof fetch === 'function') loadPolicy();
    }

    // defaultEnv reads the browser facts support() needs. A cross-origin parent makes window.top
    // throw; that page is framed as well.
    function defaultEnv() {
        const w = typeof window !== 'undefined' ? window : {};
        const nav = typeof navigator !== 'undefined' ? navigator : {};
        const doc = typeof document !== 'undefined' ? document : {};
        let top;
        try { top = w.top; } catch (_) { top = {}; }
        return {
            isSecureContext: !!w.isSecureContext,
            serial: nav.serial || null,
            top,
            self: w.self || w,
            policy: doc.permissionsPolicy || doc.featurePolicy || null,
            desktop: policyState(),
            loadPolicy
        };
    }

    // support(env) → {ok, reason}: reason is readonly (read-only desktop), disabled (the desktop's
    // serial switch is off or not known yet), insecure (no secure context), unsupported (no
    // navigator.serial: Firefox, Safari), framed (not the top-level document) or policy (denied by
    // Permissions-Policy). An env without desktop (tests, other callers) skips the desktop policy.
    function support(env) {
        const e = env || defaultEnv();
        if (e.desktop) {
            if (e.desktop.readonly === true) return { ok: false, reason: 'readonly' };
            if (e.desktop.serial !== true) return { ok: false, reason: 'disabled' };
        }
        if (!e.isSecureContext) return { ok: false, reason: 'insecure' };
        if (!e.serial || typeof e.serial.requestPort !== 'function') return { ok: false, reason: 'unsupported' };
        if (e.top && e.self && e.top !== e.self) return { ok: false, reason: 'framed' };
        if (e.policy && typeof e.policy.allowsFeature === 'function' && !e.policy.allowsFeature('serial')) return { ok: false, reason: 'policy' };
        return { ok: true, reason: '' };
    }

    // classify maps a Web Serial or esptool exception to a CYD_* code ('' when unknown).
    function classify(err) {
        if (err && typeof err.code === 'string' && err.code.startsWith('CYD_')) return err.code;
        const name = (err && err.name) || '';
        const message = String((err && err.message) || err || '').toLowerCase();
        if (name === 'InvalidStateError' || message.includes('already open') || message.includes('failed to open')) return 'CYD_FLASH_PORT_BUSY';
        if (message.includes('lost') || message.includes('disconnect') || name === 'NetworkError') return 'CYD_FLASH_DISCONNECTED';
        return '';
    }

    function wrap(err) {
        const code = classify(err);
        return code && !(err && err.code === code) ? serialError(code, (err && err.message) || code, err) : err;
    }

    // createManager(env) builds a manager; P.serial.manager is the page-wide one (a port can only be
    // opened once per page). Owners: 'flasher' (not revocable) and 'monitor' (revocable: a flash asks
    // the monitor to let go first).
    function createManager(env) {
        const state = { port: null, lease: null, listeners: new Set(), watching: false };
        const getEnv = () => env || defaultEnv();

        function emit(type, detail) {
            state.listeners.forEach(fn => {
                try { fn(type, detail); } catch (err) { console.error('cyd-creator serial listener failed', err); }
            });
        }
        // The page-wide manager tells its owners about policy changes (they re-check support()).
        if (!env) desktop.listeners.add(policy => emit('policy', policy));

        // gate waits for an unknown desktop policy, then refuses when Web Serial may not be used.
        async function gate() {
            let e = getEnv();
            if (e.desktop && e.desktop.serial == null && typeof e.loadPolicy === 'function') {
                await e.loadPolicy();
                e = getEnv();
            }
            const s = support(e);
            if (!s.ok) throw serialError('CYD_SERIAL_UNSUPPORTED', 'Web Serial is not available: ' + s.reason, null, { reason: s.reason });
            return e;
        }

        function watch() {
            const serial = getEnv().serial;
            if (state.watching || !serial || typeof serial.addEventListener !== 'function') return;
            state.watching = true;
            serial.addEventListener('disconnect', event => {
                const port = (event && (event.port || event.target)) || null;
                if (port && port === state.port) emit('disconnect', port);
            });
        }

        function makeLease(owner, opts) {
            const o = opts || {};
            const lease = {
                owner,
                revocable: !!o.revocable,
                port: state.port,
                opened: false,
                active: true,
                closing: false,
                reader: null,
                loop: null,
                async open(baudRate) {
                    try {
                        await lease.port.open({ baudRate: Number(baudRate) || 115200 });
                    } catch (err) {
                        throw wrap(err);
                    }
                    lease.opened = true;
                },
                // startReader(onData(Uint8Array), onEnd(error|null, stopped)) runs the read loop until
                // stop() (returned), close() or an error; it always releases the reader lock.
                startReader(onData, onEnd) {
                    if (!lease.opened || !lease.port.readable) return async () => {};
                    let stopped = false;
                    const reader = lease.port.readable.getReader();
                    lease.reader = reader;
                    lease.loop = (async () => {
                        let failure = null;
                        try {
                            for (;;) {
                                const { value, done } = await reader.read();
                                if (done) break;
                                if (value && value.length) onData(value);
                            }
                        } catch (err) {
                            if (!stopped && !lease.closing) failure = wrap(err);
                        } finally {
                            try { reader.releaseLock(); } catch (_) { /* already released */ }
                            lease.reader = null;
                        }
                        if (typeof onEnd === 'function') onEnd(failure, stopped || lease.closing);
                    })();
                    return async () => {
                        stopped = true;
                        try { await reader.cancel(); } catch (_) { /* the stream may be gone */ }
                        await lease.loop;
                    };
                },
                async setSignals(signals) {
                    if (lease.opened && typeof lease.port.setSignals === 'function') await lease.port.setSignals(signals);
                },
                // reset pulses EN low through RTS with IO0 high (DTR off): the board restarts into the app.
                async reset() {
                    await lease.setSignals({ dataTerminalReady: false, requestToSend: true });
                    await sleep(RESET_PULSE_MS);
                    await lease.setSignals({ dataTerminalReady: false, requestToSend: false });
                },
                async close() {
                    lease.closing = true;
                    if (lease.reader) {
                        try { await lease.reader.cancel(); } catch (_) { /* the stream may be gone */ }
                    }
                    if (lease.loop) {
                        try { await lease.loop; } catch (_) { /* ended */ }
                    }
                    if (lease.opened) {
                        lease.opened = false;
                        try { await lease.port.close(); } catch (_) { /* already closed or lost */ }
                    }
                    lease.closing = false;
                },
                async revoke() {
                    if (typeof o.onRevoke === 'function') {
                        try { await o.onRevoke(); } catch (err) { console.error('cyd-creator serial revoke failed', err); }
                    }
                    await lease.release();
                },
                async release() {
                    if (!lease.active) return;
                    lease.active = false;
                    await lease.close();
                    if (state.lease === lease) {
                        state.lease = null;
                        emit('owner', '');
                    }
                }
            };
            return lease;
        }

        return {
            support: () => support(getEnv()),
            // ensure() → support() after an unknown desktop policy was loaded (call it first on a click).
            async ensure() {
                const e = getEnv();
                if (e.desktop && e.desktop.serial == null && typeof e.loadPolicy === 'function') await e.loadPolicy();
                return support(getEnv());
            },
            port: () => state.port,
            hasPort: () => !!state.port,
            owner: () => (state.lease ? state.lease.owner : ''),
            on(fn) {
                state.listeners.add(fn);
                return () => state.listeners.delete(fn);
            },
            // request opens the browser's port chooser (call it directly from a click). It returns the
            // port, or null when the user closed the chooser.
            async request(filters) {
                const e = await gate();
                watch();
                let port;
                try {
                    port = await e.serial.requestPort(filters && filters.length ? { filters } : {});
                } catch (err) {
                    if (err && (err.name === 'NotFoundError' || err.name === 'AbortError')) return null;
                    throw wrap(err);
                }
                if (state.lease && port !== state.port) throw serialError('CYD_FLASH_PORT_BUSY', state.lease.owner + ' holds another port');
                state.port = port;
                emit('port', port);
                return port;
            },
            // acquire hands the port to owner. A revocable holder (the monitor) is asked to let go
            // first; a flash in progress makes everybody else wait with CYD_FLASH_PORT_BUSY.
            async acquire(owner, opts) {
                if (!state.port) throw serialError('CYD_FLASH_DISCONNECTED', 'no serial port selected');
                await gate();
                if (state.lease && state.lease.owner === owner && state.lease.active) return state.lease;
                if (state.lease) {
                    const current = state.lease;
                    if (!current.revocable) throw serialError('CYD_FLASH_PORT_BUSY', current.owner + ' holds the port');
                    await current.revoke();
                }
                const lease = makeLease(owner, opts);
                state.lease = lease;
                emit('owner', owner);
                return lease;
            },
            // forget drops the remembered port (after the owner released it).
            forget() {
                if (!state.lease) state.port = null;
            }
        };
    }

    P.serial = { support, classify, createManager, manager: createManager(), RESET_PULSE_MS };
    P.serial.policy = { apply: applyPolicy, state: policyState, load: loadPolicy };
})();
