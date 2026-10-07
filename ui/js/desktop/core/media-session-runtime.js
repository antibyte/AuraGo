(function () {
    'use strict';
    const owners = new Map();
    const actions = new Set(['play', 'pause', 'stop', 'previoustrack', 'nexttrack', 'seekbackward', 'seekforward', 'seekto']);
    let sequence = 0;
    function render() {
        if (!('mediaSession' in navigator)) return;
        const selected = [...owners.values()].sort((a, b) => b.priority - a.priority || b.order - a.order)[0];
        for (const action of actions) {
            try { navigator.mediaSession.setActionHandler(action, selected?.handlers?.[action] || null); } catch (_) { /* unsupported action */ }
        }
        try { navigator.mediaSession.metadata = selected?.metadata ? new MediaMetadata(selected.metadata) : null; } catch (_) { /* unsupported metadata */ }
        try { navigator.mediaSession.playbackState = selected?.playbackState || 'none'; } catch (_) { /* unsupported state */ }
    }
    window.AuraDesktopMediaSession = Object.freeze({
        claim(owner, options) {
            if (!owner) return;
            const prior = owners.get(owner);
            owners.set(owner, { ...options, priority: options.priority || 0, order: options.activate ? ++sequence : (prior?.order || ++sequence) });
            Object.keys(options.handlers || {}).forEach(action => actions.add(action));
            render();
        },
        release(owner) { if (owners.delete(owner)) render(); }
    });
})();
