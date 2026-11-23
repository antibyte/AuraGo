/* Runenpfad camera rig: look-ahead in the facing direction and a calm vertical follow. Pure math, DOM-free. */
(function (root) {
    'use strict';

    const LEAD_FRAC = 0.4;        // hero sits at 40 % from the trailing edge, 60 % lies ahead
    const FLIP_TIME = 0.35;       // seconds facing the new way before the lead side changes
    const FLIP_DIST = 48;         // or px moved that way
    const LEAD_RATE = 2.2;        // lead side change, full swing per 1 / 2.2 s
    const FOLLOW_X = 10.5;
    const FOLLOW_Y = 9;
    const STAND_FRAC = 0.62;      // standing anchor: hero at 62 % of the view height
    const BAND_TOP = 0.22;
    const BAND_BOTTOM = 0.74;
    const FAST_FALL = 520;
    const SNAP_DIST = 400;

    function clamp(v, lo, hi) {
        return v < lo ? lo : (v > hi ? hi : v);
    }

    function create() {
        let ready = false;
        let camX = 0;
        let camY = 0;
        let leadDir = 1;
        let lead = 1;             // 1 = camera looks right (hero at 40 % from the left), 0 = looks left
        let pendDir = 0;
        let pendT = 0;
        let pendX = 0;
        let standY = 0;
        let lastX = 0;
        let lastY = 0;

        function reset() {
            ready = false;
            pendDir = 0;
            pendT = 0;
        }

        function update(o) {
            const dt = Math.max(0, Math.min(0.25, o.dt || 0));
            const vw = o.vw;
            const vh = o.vh;
            const face = o.facing < 0 ? -1 : 1;
            const moved = ready ? Math.max(Math.abs(o.px - lastX), Math.abs(o.py - lastY)) : 0;
            const snap = !ready || moved > SNAP_DIST;

            if (snap) {
                leadDir = face;
                lead = face > 0 ? 1 : 0;
                pendDir = 0;
                pendT = 0;
                standY = o.py;
            } else {
                if (face !== leadDir) {
                    if (pendDir !== face) { pendDir = face; pendT = 0; pendX = o.px; }
                    pendT += dt;
                    if (pendT >= FLIP_TIME || (o.px - pendX) * face >= FLIP_DIST) {
                        leadDir = face;
                        pendDir = 0;
                        pendT = 0;
                    }
                } else {
                    pendDir = 0;
                    pendT = 0;
                }
                const goal = leadDir > 0 ? 1 : 0;
                const step = LEAD_RATE * dt;
                lead = lead < goal ? Math.min(goal, lead + step) : Math.max(goal, lead - step);
            }

            // Vertical anchor: remember where the hero last stood or climbed; follow in the air only outside the band.
            if (o.grounded || o.climbing || snap) {
                standY = o.py;
            } else {
                const rel = (o.py - camY) / vh;
                if (o.vy > FAST_FALL) standY = o.py;
                else if (rel < BAND_TOP) standY = o.py + vh * (STAND_FRAC - BAND_TOP);
                else if (rel > BAND_BOTTOM) standY = o.py - vh * (BAND_BOTTOM - STAND_FRAC);
            }

            const offset = vw * (LEAD_FRAC + (1 - lead) * (1 - 2 * LEAD_FRAC));
            const targetX = clamp(o.px - offset, o.minX, o.maxX);
            const targetY = o.maxY <= 0 ? o.maxY : clamp(standY - vh * STAND_FRAC, 0, o.maxY);
            if (snap) {
                camX = targetX;
                camY = targetY;
                ready = true;
            } else {
                camX += (targetX - camX) * (1 - Math.exp(-dt * FOLLOW_X));
                camY = o.maxY <= 0 ? targetY : camY + (targetY - camY) * (1 - Math.exp(-dt * FOLLOW_Y));
            }
            lastX = o.px;
            lastY = o.py;
            return { x: camX, y: camY };
        }

        return { reset: reset, update: update };
    }

    root.RunenpfadCameraRig = { create: create };
})(typeof window !== 'undefined' ? window : globalThis);
