(function () {
    'use strict';
    if (window.AuraBrowserAudioLease) return;

    const storageKey = 'aurago.browser-audio-lease.v1';
    const lockName = 'aurago.browser-audio';
    const ttlMs = 7000;
    const renewMs = 2500;
    const settleMs = 80;
    const tabKey = 'aurago.browser-audio-tab-id.v1';
    const listeners = new Set();
    let renewalTimer = null;
    let heldLease = null;
    let channel = null;

    function randomID() {
        if (window.crypto && typeof window.crypto.randomUUID === 'function') return window.crypto.randomUUID();
        const bytes = new Uint8Array(16);
        window.crypto.getRandomValues(bytes);
        return Array.from(bytes, value => value.toString(16).padStart(2, '0')).join('');
    }

    function tabID() {
        let value = '';
        try { value = sessionStorage.getItem(tabKey) || ''; } catch (_) {}
        if (!value) {
            value = randomID();
            try { sessionStorage.setItem(tabKey, value); } catch (_) {}
        }
        return value;
    }

    function readLease() {
        try {
            const value = JSON.parse(localStorage.getItem(storageKey) || 'null');
            if (!value || !value.token || Number(value.expires_at || 0) <= Date.now()) return null;
            return value;
        } catch (_) {
            return null;
        }
    }

    function publish(broadcast) {
        const current = readLease();
        listeners.forEach(listener => {
            try { listener(current); } catch (_) {}
        });
        if (broadcast !== false && channel) {
            try { channel.postMessage({ type: 'lease-change' }); } catch (_) {}
        }
    }

    function writeLease(lease) {
        localStorage.setItem(storageKey, JSON.stringify(lease));
        publish();
    }

    function leaseWins(left, right) {
        if (!right) return true;
        const leftStarted = Number(left && left.acquired_at || 0);
        const rightStarted = Number(right && right.acquired_at || 0);
        if (leftStarted !== rightStarted) return leftStarted < rightStarted;
        return String(left && left.token || '') < String(right && right.token || '');
    }

    function stopRenewal() {
        if (renewalTimer) clearInterval(renewalTimer);
        renewalTimer = null;
    }

    function startRenewal() {
        stopRenewal();
        renewalTimer = setInterval(renew, renewMs);
    }

    function renew() {
        if (!heldLease) {
            stopRenewal();
            return;
        }
        const lease = readLease();
        if (lease && lease.token === heldLease.token) {
            heldLease.expires_at = Date.now() + ttlMs;
            writeLease(heldLease);
            return;
        }
        // A throttled active tab can miss a renewal. The older live holder
        // reasserts its lease so a newer tab cannot silently take its mic.
        if (!lease || heldLease.web_lock || leaseWins(heldLease, lease)) {
            heldLease.expires_at = Date.now() + ttlMs;
            writeLease(heldLease);
            return;
        }
        stopRenewal();
        publish();
    }

    function busyError(message) {
        const error = new Error(message || 'another audio session is active');
        error.code = 'audio_session_busy';
        error.lease = readLease();
        return error;
    }

    function publicLease(lease) {
        return { token: lease.token, owner: lease.owner, kind: lease.kind };
    }

    async function acquireWebLock(kind, owner) {
        if (heldLease) {
            if (heldLease.owner === owner && heldLease.kind === kind) return publicLease(heldLease);
            throw busyError();
        }
        let settle;
        const granted = new Promise((resolve, reject) => { settle = { resolve, reject }; });
        navigator.locks.request(lockName, { mode: 'exclusive', ifAvailable: true }, lock => {
            if (!lock) {
                settle.reject(busyError());
                return undefined;
            }
            let releaseLock;
            const hold = new Promise(resolve => { releaseLock = resolve; });
            const now = Date.now();
            heldLease = {
                kind,
                owner,
                token: randomID(),
                tab_id: tabID(),
                acquired_at: now,
                expires_at: now + ttlMs,
                web_lock: true,
                release_lock: releaseLock
            };
            writeLease(heldLease);
            startRenewal();
            settle.resolve(publicLease(heldLease));
            return hold;
        }).catch(error => settle.reject(error));
        return granted;
    }

    async function acquireStorageLease(kind, owner) {
        if (heldLease) {
            if (heldLease.owner === owner && heldLease.kind === kind) return publicLease(heldLease);
            throw busyError();
        }
        const current = readLease();
        if (current && current.owner !== owner) throw busyError();
        const now = Date.now();
        const candidate = {
            kind,
            owner,
            token: current && current.owner === owner ? current.token : randomID(),
            tab_id: tabID(),
            acquired_at: current && current.owner === owner ? Number(current.acquired_at || now) : now,
            expires_at: now + ttlMs,
            web_lock: false
        };
        writeLease(candidate);
        await new Promise(resolve => setTimeout(resolve, settleMs));
        const confirmed = readLease();
        if (!confirmed || confirmed.token !== candidate.token) throw busyError('audio session lease could not be acquired');
        heldLease = candidate;
        startRenewal();
        return publicLease(candidate);
    }

    async function acquire(kind, owner) {
        const normalizedKind = String(kind || '').trim();
        const normalizedOwner = String(owner || tabID()).trim();
        if (!normalizedKind || !normalizedOwner) throw new Error('audio lease identity is required');
        if (navigator.locks && typeof navigator.locks.request === 'function') {
            return acquireWebLock(normalizedKind, normalizedOwner);
        }
        return acquireStorageLease(normalizedKind, normalizedOwner);
    }

    function release(token) {
        const expected = String(token || (heldLease && heldLease.token) || '');
        const current = readLease();
        if (current && current.token === expected) localStorage.removeItem(storageKey);
        if (heldLease && (!token || expected === heldLease.token)) {
            const releaseLock = heldLease.release_lock;
            heldLease = null;
            stopRenewal();
            if (typeof releaseLock === 'function') releaseLock();
        }
        publish();
    }

    function subscribe(listener) {
        if (typeof listener !== 'function') return function () {};
        listeners.add(listener);
        return function () { listeners.delete(listener); };
    }

    function reconcileExternalChange() {
        const current = readLease();
        if (heldLease && (!current || current.token !== heldLease.token)) {
            renew();
            return;
        }
        publish(false);
    }

    try {
        channel = new BroadcastChannel('aurago-browser-audio-lease-v1');
        channel.addEventListener('message', reconcileExternalChange);
    } catch (_) {
        channel = null;
    }
    window.addEventListener('storage', event => {
        if (event.key === storageKey) reconcileExternalChange();
    });
    window.addEventListener('pagehide', () => release());

    window.AuraBrowserAudioLease = {
        acquire,
        release,
        current: readLease,
        subscribe,
        tabID
    };
})();
