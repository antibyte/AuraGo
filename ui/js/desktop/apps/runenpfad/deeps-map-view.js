/* Runenpfad deeps map panel, canvas side: a small overview of the twelve cave rooms in the top-right corner while the hero is in the deeps. Visited rooms are filled, rooms known from Edda's map are outlined, the current room pulses, links are lines. Hidden while Brandur is awake. Read-only on the sim, never throws. */
(function (root) {
    'use strict';

    // ---- tuning (sizes, colours: adjust here) ----
    const PANEL_W = 150;
    const PANEL_H = 84;
    const MARGIN = 12;
    const GAP_Y = 8; // below the top-bar inset
    const PAD = 8;
    const LABEL_H = 14;
    const NODE_W = 0.74; // node size as a share of a grid cell
    const NODE_H = 0.64;
    const COL_PANEL = 'rgba(10, 7, 9, 0.74)';
    const COL_EDGE = 'rgba(255, 190, 120, 0.45)';
    const COL_LINK = 'rgba(210, 160, 110, 0.55)';
    const COL_SEEN = '#d9892f';
    const COL_KNOWN = 'rgba(190, 160, 120, 0.9)';
    const COL_NOW = '#fff3cf';
    const COL_TEXT = '#ffd9a0';

    function clamp(v, lo, hi) { return v < lo ? lo : v > hi ? hi : v; }
    function num(v, d) { return typeof v === 'number' && isFinite(v) ? v : d; }
    function Rooms() { return root.RunenpfadDeepsRooms || null; }

    function create(h) {
        const ctx = h && h.ctx;

        function tr(key, fallback) {
            const f = h && h.t;
            if (typeof f !== 'function') return fallback;
            const w = f(key);
            return w && w !== key ? w : fallback;
        }

        function fontFor(px) {
            const fam = (h && h.font) || '"Segoe UI", sans-serif';
            return /Press Start/.test(fam) ? Math.max(7, Math.round(px * 0.6)) + 'px ' + fam : 'bold ' + px + 'px ' + fam;
        }

        function awakeGuardian(it) {
            const g = it && it.guardian;
            return !!g && g.state !== 'dead' && g.state !== 'sleep';
        }

        function box(x, y, w, hh, r) {
            if (typeof ctx.roundRect === 'function') { ctx.beginPath(); ctx.roundRect(x, y, w, hh, r); return; }
            ctx.beginPath();
            ctx.rect(x, y, w, hh);
        }

        // Draws the panel; `inset` is the bottom edge of the top bar in canvas px.
        function draw(sim, f, vw, inset) {
            try {
                const R = Rooms();
                const it = sim && sim.interior;
                if (!ctx || !R || !it || it.placeId !== 'deeps' || awakeGuardian(it)) return;
                const st = R.mapState(sim);
                if (!st || !st.rooms.length) return;
                const reduced = !!(f && f.reduced);
                const t = num(f && f.t, 0);
                const px = Math.round(num(vw, 800) - PANEL_W - MARGIN);
                const py = Math.round(num(inset, 0) + GAP_Y);
                ctx.save();
                box(px, py, PANEL_W, PANEL_H, 8);
                ctx.fillStyle = COL_PANEL;
                ctx.fill();
                ctx.strokeStyle = COL_EDGE;
                ctx.lineWidth = 1.5;
                ctx.stroke();
                ctx.font = fontFor(9);
                ctx.textAlign = 'left';
                ctx.fillStyle = COL_TEXT;
                ctx.fillText(tr('runenpfad.deeps.map', 'Deeps'), px + PAD, py + PAD + 5);
                let cols = 1;
                let rows = 1;
                st.rooms.forEach(function (r) { cols = Math.max(cols, r.x + 1); rows = Math.max(rows, r.y + 1); });
                const cw = (PANEL_W - PAD * 2) / cols;
                const ch = (PANEL_H - PAD * 2 - LABEL_H) / rows;
                const gx = px + PAD;
                const gy = py + PAD + LABEL_H;
                const at = function (r) { return { x: gx + (r.x + 0.5) * cw, y: gy + (r.y + 0.5) * ch }; };
                const byId = {};
                st.rooms.forEach(function (r) { byId[r.id] = r; });
                ctx.strokeStyle = COL_LINK;
                ctx.lineWidth = 2;
                st.links.forEach(function (l) {
                    const a = byId[l[0]];
                    const b = byId[l[1]];
                    if (!a || !b || !(a.known && b.known)) return;
                    const pa = at(a);
                    const pb = at(b);
                    ctx.beginPath();
                    ctx.moveTo(pa.x, pa.y);
                    ctx.lineTo(pb.x, pb.y);
                    ctx.stroke();
                });
                const nw = cw * NODE_W;
                const nh = ch * NODE_H;
                st.rooms.forEach(function (r) {
                    if (!r.known && !r.current) return;
                    const p = at(r);
                    box(p.x - nw / 2, p.y - nh / 2, nw, nh, 3);
                    if (r.visited) {
                        ctx.fillStyle = COL_SEEN;
                        ctx.fill();
                    }
                    ctx.strokeStyle = r.visited ? 'rgba(40, 18, 6, 0.85)' : COL_KNOWN;
                    ctx.lineWidth = 1.5;
                    ctx.stroke();
                    if (r.current) {
                        const k = reduced ? 1 : 0.55 + 0.45 * Math.sin(t * 5);
                        ctx.save();
                        ctx.globalAlpha = clamp(k, 0.3, 1);
                        box(p.x - nw / 2 - 2, p.y - nh / 2 - 2, nw + 4, nh + 4, 4);
                        ctx.strokeStyle = COL_NOW;
                        ctx.lineWidth = 2;
                        ctx.stroke();
                        ctx.restore();
                    }
                });
                ctx.restore();
            } catch (err) {
                try { if (ctx) ctx.restore(); } catch (e2) { /* a bad panel must not stop the room */ }
            }
        }

        function dispose() { /* no resources: the panel keeps no state */ }

        return { draw: draw, dispose: dispose };
    }

    root.RunenpfadDeepsMapView = { create: create };
})(typeof window !== 'undefined' ? window : globalThis);
