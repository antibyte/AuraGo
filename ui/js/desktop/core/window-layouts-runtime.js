    const DESKTOP_WINDOW_LAYOUT_ZONES = Object.freeze([
        'left-half', 'right-half', 'top-half', 'bottom-half',
        'top-left', 'top-right', 'bottom-left', 'bottom-right', 'maximize'
    ]);

    function desktopWindowLayoutRect(zone, width, height) {
        const w = Math.max(0, Math.floor(Number(width) || 0));
        const h = Math.max(0, Math.floor(Number(height) || 0));
        const halfW = Math.floor(w / 2);
        const halfH = Math.floor(h / 2);
        const rects = {
            'left-half': { left: 0, top: 0, width: halfW, height: h },
            'right-half': { left: halfW, top: 0, width: w - halfW, height: h },
            'top-half': { left: 0, top: 0, width: w, height: halfH },
            'bottom-half': { left: 0, top: halfH, width: w, height: h - halfH },
            'top-left': { left: 0, top: 0, width: halfW, height: halfH },
            'top-right': { left: halfW, top: 0, width: w - halfW, height: halfH },
            'bottom-left': { left: 0, top: halfH, width: halfW, height: h - halfH },
            'bottom-right': { left: halfW, top: halfH, width: w - halfW, height: h - halfH },
            maximize: { left: 0, top: 0, width: w, height: h }
        };
        return rects[zone] || null;
    }

    function desktopWindowLayoutFits(rect, constraints) {
        const maxWidth = constraints && Number.isFinite(constraints.maxWidth) ? constraints.maxWidth : Infinity;
        const maxHeight = constraints && Number.isFinite(constraints.maxHeight) ? constraints.maxHeight : Infinity;
        return !!rect && rect.width >= (constraints && constraints.minWidth || 0)
            && rect.height >= (constraints && constraints.minHeight || 0)
            && rect.width <= maxWidth && rect.height <= maxHeight;
    }

    function desktopWindowLayoutMaxLimit(value, fallback) {
        const limit = Number.parseFloat(value);
        return Number.isFinite(limit) ? Math.min(fallback, Math.max(0, limit)) : fallback;
    }

    function desktopWindowLayoutOccupiesHalf(item, side) {
        if (!item) return false;
        if (item.maximized) return true;
        const zone = String(item.snapped || '');
        if (zone === side + '-half') return true;
        return zone === 'top-' + side || zone === 'bottom-' + side;
    }

    function desktopWindowKeyboardLayout(zone, key) {
        if (key === 'ArrowLeft') return 'left-half';
        if (key === 'ArrowRight') return 'right-half';
        const side = zone === 'left-half' || zone === 'top-left' || zone === 'bottom-left' ? 'left'
            : zone === 'right-half' || zone === 'top-right' || zone === 'bottom-right' ? 'right' : '';
        if (side && (key === 'ArrowUp' || key === 'ArrowDown')) {
            return (key === 'ArrowUp' ? 'top-' : 'bottom-') + side;
        }
        return key === 'ArrowUp' ? 'maximize' : key === 'ArrowDown' ? 'restore' : '';
    }

    function desktopWindowRememberRestoreBounds(item, bounds) {
        if (!item) return null;
        if (!item.restoreBounds && bounds) item.restoreBounds = { ...bounds };
        return item.restoreBounds;
    }

    async function desktopCloseWindowBatch(ids, hasWindow, closeWindow) {
        const closed = [];
        const skipped = [];
        let cancelled = false;
        for (const id of Array.from(ids || [])) {
            if (typeof hasWindow === 'function' && !hasWindow(id)) {
                skipped.push(id);
                continue;
            }
            let result;
            try { result = await closeWindow(id); }
            catch (_) { cancelled = true; break; }
            if (result === 'cancelled') {
                cancelled = true;
                break;
            }
            if (result !== 'accepted') {
                skipped.push(id);
                continue;
            }
            closed.push(id);
        }
        return {
            status: cancelled ? 'cancelled' : closed.length ? 'accepted' : 'skipped',
            closed,
            skipped
        };
    }
