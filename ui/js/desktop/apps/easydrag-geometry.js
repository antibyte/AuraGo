// EasyDrag geometry: card sizes, port positions, wire curves, hit tests and viewport math.
// Pure functions in world coordinates; screen = world * zoom + (view.x, view.y).
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};

    const NODE_W = 232;
    const NODE_H = 72;
    const PORT_TOP = 36;
    const PORT_PITCH = 24;
    const GRID = 8;
    const MIN_ZOOM = 0.25;
    const MAX_ZOOM = 2;

    function nodeHeight(outputCount) {
        return outputCount > 1 ? PORT_TOP + (outputCount - 1) * PORT_PITCH + 26 : NODE_H;
    }

    // portPoint returns the world position of a port. side is "in" or "out".
    function portPoint(position, side, index, outputCount) {
        const h = nodeHeight(outputCount);
        if (side === 'in') return { x: position.x, y: position.y + h / 2 };
        const y = outputCount > 1 ? PORT_TOP + index * PORT_PITCH : h / 2;
        return { x: position.x + NODE_W, y: position.y + y };
    }

    function controlOffset(a, b) {
        return Math.max(48, Math.min(240, Math.abs(b.x - a.x) * 0.5 + (b.x < a.x ? 80 : 0)));
    }

    function wirePath(a, b) {
        const dx = controlOffset(a, b);
        return 'M' + r(a.x) + ' ' + r(a.y) + ' C' + r(a.x + dx) + ' ' + r(a.y) + ' ' + r(b.x - dx) + ' ' + r(b.y) + ' ' + r(b.x) + ' ' + r(b.y);
    }

    function r(v) { return Math.round(v * 10) / 10; }

    function bezierPoint(a, b, t) {
        const dx = controlOffset(a, b);
        const p1 = { x: a.x + dx, y: a.y };
        const p2 = { x: b.x - dx, y: b.y };
        const u = 1 - t;
        return {
            x: u * u * u * a.x + 3 * u * u * t * p1.x + 3 * u * t * t * p2.x + t * t * t * b.x,
            y: u * u * u * a.y + 3 * u * u * t * p1.y + 3 * u * t * t * p2.y + t * t * t * b.y
        };
    }

    function wireMidpoint(a, b) { return bezierPoint(a, b, 0.5); }

    function distanceToSegment(p, a, b) {
        const vx = b.x - a.x;
        const vy = b.y - a.y;
        const len = vx * vx + vy * vy;
        const t = len ? Math.max(0, Math.min(1, ((p.x - a.x) * vx + (p.y - a.y) * vy) / len)) : 0;
        const x = a.x + t * vx;
        const y = a.y + t * vy;
        return Math.hypot(p.x - x, p.y - y);
    }

    // distanceToWire approximates the distance from p to the wire curve.
    function distanceToWire(p, a, b) {
        let best = Infinity;
        let prev = a;
        for (let i = 1; i <= 24; i++) {
            const next = bezierPoint(a, b, i / 24);
            best = Math.min(best, distanceToSegment(p, prev, next));
            prev = next;
        }
        return best;
    }

    function nodeRect(position, outputCount) {
        return { x: position.x, y: position.y, w: NODE_W, h: nodeHeight(outputCount) };
    }

    function contains(rect, p) {
        return p.x >= rect.x && p.x <= rect.x + rect.w && p.y >= rect.y && p.y <= rect.y + rect.h;
    }

    function intersects(a, b) {
        return a.x < b.x + b.w && a.x + a.w > b.x && a.y < b.y + b.h && a.y + a.h > b.y;
    }

    function normalizeRect(a, b) {
        return { x: Math.min(a.x, b.x), y: Math.min(a.y, b.y), w: Math.abs(b.x - a.x), h: Math.abs(b.y - a.y) };
    }

    function bounds(rects) {
        if (!rects.length) return null;
        let minX = Infinity; let minY = Infinity; let maxX = -Infinity; let maxY = -Infinity;
        rects.forEach(rc => {
            minX = Math.min(minX, rc.x); minY = Math.min(minY, rc.y);
            maxX = Math.max(maxX, rc.x + rc.w); maxY = Math.max(maxY, rc.y + rc.h);
        });
        return { x: minX, y: minY, w: maxX - minX, h: maxY - minY };
    }

    function clampZoom(z) { return Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, z)); }

    // fit returns the view that shows box inside a viewport of size, with padding.
    function fit(box, size, padding, maxZoom) {
        if (!box) return { x: size.w / 2, y: size.h / 2, zoom: 1 };
        const pad = padding == null ? 64 : padding;
        const zoom = clampZoom(Math.min((size.w - pad * 2) / Math.max(box.w, 1), (size.h - pad * 2) / Math.max(box.h, 1), maxZoom || 1));
        return {
            x: (size.w - box.w * zoom) / 2 - box.x * zoom,
            y: (size.h - box.h * zoom) / 2 - box.y * zoom,
            zoom
        };
    }

    // zoomAt changes the zoom and keeps the world point under the screen point fixed.
    function zoomAt(view, zoom, screen) {
        const z = clampZoom(zoom);
        const wx = (screen.x - view.x) / view.zoom;
        const wy = (screen.y - view.y) / view.zoom;
        return { x: screen.x - wx * z, y: screen.y - wy * z, zoom: z };
    }

    function toWorld(view, p) { return { x: (p.x - view.x) / view.zoom, y: (p.y - view.y) / view.zoom }; }
    function toScreen(view, p) { return { x: p.x * view.zoom + view.x, y: p.y * view.zoom + view.y }; }
    function snap(v) { return Math.round(v / GRID) * GRID; }

    // freeSpot finds a position near want that does not overlap existing cards.
    function freeSpot(want, rects) {
        const candidate = { x: snap(want.x), y: snap(want.y) };
        for (let i = 0; i < 40; i++) {
            const box = { x: candidate.x, y: candidate.y, w: NODE_W, h: NODE_H };
            if (!rects.some(rc => intersects(box, { x: rc.x - 16, y: rc.y - 16, w: rc.w + 32, h: rc.h + 32 }))) return candidate;
            candidate.y += NODE_H + 32;
        }
        return candidate;
    }

    ED.geometry = {
        NODE_W, NODE_H, PORT_TOP, PORT_PITCH, GRID, MIN_ZOOM, MAX_ZOOM,
        nodeHeight, portPoint, wirePath, bezierPoint, wireMidpoint, distanceToWire, nodeRect, contains, intersects,
        normalizeRect, bounds, clampZoom, fit, zoomAt, toWorld, toScreen, snap, freeSpot
    };
})();
