    // Read-only geometry bridge. Effects never own application state or pointer handling.
    const desktopWindowEffectSamples = new Map();

    function desktopWindowEffectRecord(item, phase, options) {
        if (!item || !item.element || item.isGadget) return null;
        const element = item.element;
        const bounds = element.getBoundingClientRect();
        const rect = { x: bounds.left, y: bounds.top, width: bounds.width, height: bounds.height };
        const previous = desktopWindowEffectSamples.get(item.id);
        const now = performance.now();
        const elapsed = previous ? Math.max(8, now - previous.time) / 1000 : 0;
        const moving = phase === 'drag' || phase === 'resize';
        const ended = phase === 'drag-end' || phase === 'resize-end';
        const velocity = moving && previous && elapsed < 0.25
            ? { x: (rect.x - previous.rect.x) / elapsed, y: (rect.y - previous.rect.y) / elapsed }
            : ended && previous ? previous.velocity : { x: 0, y: 0 };
        const minimized = item.minimizing || element.hidden || element.style.display === 'none';
        const visible = element.isConnected && !minimized && !element.hasAttribute('data-space-hidden') && bounds.width > 0 && bounds.height > 0;
        if (phase !== 'snapshot') desktopWindowEffectSamples.set(item.id, { rect, velocity, time: now });
        return {
            phase, id: item.id, appId: item.appId, rect,
            z: Number(element.style.zIndex) || 0,
            space: item.spaceId || element.dataset.spaceId || '1',
            velocity, visible, minimized, focused: state.activeWindowId === item.id,
            silent: !!state.sessionRestoring || !!(options && options.silent), element
        };
    }

    function emitDesktopWindowEffect(phase, id, options) {
        // Ordinary themes pay no layout-read cost for effects they do not use.
        if (!currentDesktopTheme().fx) return;
        const detail = desktopWindowEffectRecord(state.windows.get(id), phase, options);
        if (!detail) return;
        window.dispatchEvent(new CustomEvent('aurago:desktop-window', { detail }));
        if (phase === 'close') desktopWindowEffectSamples.delete(id);
    }

    function snapshotDesktopWindowEffects() {
        return [...state.windows.values()].map(item => desktopWindowEffectRecord(item, 'snapshot')).filter(item => item && item.visible && !state.windows.get(item.id)?.closing);
    }

    function refreshDesktopWindowEffects() {
        desktopWindowEffectSamples.clear();
        if (currentDesktopTheme().fx) window.dispatchEvent(new CustomEvent('aurago:desktop-windows-refresh'));
    }

    window.AuraDesktopWindowEffects = Object.freeze({ snapshot: snapshotDesktopWindowEffects });
