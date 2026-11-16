(function () {
    'use strict';
    const owners = new Map();
    const supportedActions = new Set(['play', 'pause', 'stop', 'previoustrack', 'nexttrack', 'seekbackward', 'seekforward', 'seekto']);
    const actions = new Set(supportedActions);
    const listeners = new Set();
    const statuses = new Set(['idle', 'paused', 'playing', 'buffering', 'ended', 'error']);
    let sequence = 0;
    let lastOwner = null;
    let lastSnapshot = '';

    function selectedEntry() {
        return [...owners.entries()].sort((a, b) => b[1].priority - a[1].priority || b[1].order - a[1].order)[0] || null;
    }

    function metadataSnapshot(metadata) {
        metadata = metadata && typeof metadata === 'object' ? metadata : {};
        const artwork = Array.isArray(metadata.artwork) ? metadata.artwork.map(image => {
            if (!image || typeof image !== 'object') return null;
            const item = {};
            ['src', 'sizes', 'type'].forEach(key => {
                if (typeof image[key] === 'string') item[key] = image[key];
            });
            return item.src ? item : null;
        }).filter(Boolean) : [];
        return {
            title: typeof metadata.title === 'string' ? metadata.title : '',
            artist: typeof metadata.artist === 'string' ? metadata.artist : '',
            album: typeof metadata.album === 'string' ? metadata.album : '',
            artwork
        };
    }

    function snapshotFor(entry) {
        if (!entry) return {
            available: false,
            metadata: metadataSnapshot(null),
            playbackState: 'none',
            status: 'unavailable',
            actions: []
        };
        const owner = entry[1];
        const playbackState = typeof owner.playbackState === 'string' ? owner.playbackState : 'none';
        const status = statuses.has(owner.status) ? owner.status
            : playbackState === 'playing' ? 'playing'
                : playbackState === 'paused' ? 'paused' : 'idle';
        const handlers = owner.handlers && typeof owner.handlers === 'object' ? owner.handlers : {};
        return {
            available: true,
            metadata: metadataSnapshot(owner.metadata),
            playbackState,
            status,
            actions: Object.keys(handlers).filter(action => supportedActions.has(action) && typeof handlers[action] === 'function')
        };
    }

    function notifyIfChanged(entry, snapshot) {
        const signature = JSON.stringify(snapshot);
        const owner = entry ? entry[0] : null;
        if (owner === lastOwner && signature === lastSnapshot) return;
        lastOwner = owner;
        lastSnapshot = signature;
        listeners.forEach(listener => {
            try { listener(snapshotFor(entry)); } catch (_) { /* observers cannot interrupt media ownership */ }
        });
    }

    async function dispatch(action, details) {
        if (!supportedActions.has(action)) return false;
        const entry = selectedEntry();
        const handlers = entry && entry[1].handlers;
        const handler = handlers && Object.prototype.hasOwnProperty.call(handlers, action) ? handlers[action] : null;
        if (typeof handler !== 'function') return false;
        return (await handler(details)) !== false;
    }

    function renderNative(entry) {
        if (typeof navigator === 'undefined' || !('mediaSession' in navigator) || !navigator.mediaSession) return;
        const selected = entry && entry[1];
        for (const action of actions) {
            try {
                const handler = selected?.handlers && Object.prototype.hasOwnProperty.call(selected.handlers, action)
                    ? selected.handlers[action] : null;
                navigator.mediaSession.setActionHandler(action,
                    typeof handler === 'function' ? details => { dispatch(action, details).catch(() => {}); } : null);
            } catch (_) { /* unsupported action */ }
        }
        try {
            if (!selected?.metadata) navigator.mediaSession.metadata = null;
            else if (typeof MediaMetadata === 'function') navigator.mediaSession.metadata = new MediaMetadata(metadataSnapshot(selected.metadata));
        } catch (_) { /* unsupported metadata */ }
        try { navigator.mediaSession.playbackState = selected?.playbackState || 'none'; } catch (_) { /* unsupported state */ }
    }

    function refresh() {
        const entry = selectedEntry();
        renderNative(entry);
        notifyIfChanged(entry, snapshotFor(entry));
    }

    window.AuraDesktopMediaSession = Object.freeze({
        claim(owner, options) {
            if (!owner) return;
            options = options || {};
            const prior = owners.get(owner);
            owners.set(owner, {
                ...options,
                metadata: metadataSnapshot(options.metadata),
                priority: options.priority || 0,
                order: options.activate ? ++sequence : (prior?.order || ++sequence)
            });
            Object.keys(options.handlers || {}).forEach(action => actions.add(action));
            refresh();
        },
        release(owner) {
            if (owners.delete(owner)) refresh();
        },
        snapshot() {
            return snapshotFor(selectedEntry());
        },
        subscribe(listener) {
            if (typeof listener !== 'function') return () => {};
            listeners.add(listener);
            try { listener(snapshotFor(selectedEntry())); } catch (_) { /* observers cannot interrupt media ownership */ }
            return () => listeners.delete(listener);
        },
        dispatch
    });
})();
