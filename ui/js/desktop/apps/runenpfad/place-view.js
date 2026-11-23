/* Runenpfad place facades and village hub, canvas side: building fronts on the ground line, door prompts, hub props and bark villagers. Read-only on the sim, never throws. */
(function (root) {
    'use strict';

    const FACADE_SHADE = 0.12; // facades sit in the background layer at about 88 % brightness
    const BARK_H = 78;
    const FALLBACK_BODY = { house: '#5a3d26', cave: '#4a4650', arch: '#6b6f80' };

    function Art() { return root.RunenpfadArt || null; }
    function list(v) { return Array.isArray(v) ? v : []; }
    function stageOf(v) { return v && v.stage ? v.stage : v; }

    function create(h) {
        const ctx = h.ctx;
        const shaded = {}; // clip id -> { src, canvas } darkened copy of the facade frame
        const requested = {};
        let requestedFor = null;

        // ---- loading ----

        function ask(id) {
            const A = Art();
            const art = h.art;
            if (!A || !art || requested[id] || typeof A.loadSheet !== 'function') return;
            requested[id] = true;
            A.loadSheet(art, id);
        }

        function sheetOf(clip) { return String(clip || '').split('.')[0]; }

        // Warms the facade sheets of the stage doors (and the hub sheets). Accepts a sim or a stage.
        function prime(v) {
            const st = stageOf(v);
            if (!st || !h.art) return;
            if (requestedFor !== h.art) {
                requestedFor = h.art;
                Object.keys(requested).forEach(function (k) { delete requested[k]; });
                Object.keys(shaded).forEach(function (k) { delete shaded[k]; });
            }
            const doors = list(st.doors);
            for (let i = 0; i < doors.length; i++) if (doors[i].facade) ask(sheetOf(doors[i].facade.clip));
            if (st.hub) {
                ask('vil-props');
                ask('npc-villager');
                ask('npc-guard');
                list(st.hubProps).concat(list(st.decor)).forEach(function (p) { if (p && p.clip) ask(sheetOf(p.clip)); });
            }
        }

        // ---- helpers ----

        function tr(key, fallback) {
            const f = h.t;
            if (typeof f !== 'function' || !key) return fallback;
            const w = f(key);
            return w && w !== key ? w : fallback;
        }

        function fontFor(px) {
            const fam = h.font || '"Segoe UI", sans-serif';
            return /Press Start/.test(fam) ? Math.max(7, Math.round(px * 0.6)) + 'px ' + fam : 'bold ' + px + 'px ' + fam;
        }

        function rrect(x, y, w, hh, r) {
            const rr = Math.min(r, w / 2, hh / 2);
            ctx.beginPath();
            ctx.moveTo(x + rr, y);
            ctx.arcTo(x + w, y, x + w, y + hh, rr);
            ctx.arcTo(x + w, y + hh, x, y + hh, rr);
            ctx.arcTo(x, y + hh, x, y, rr);
            ctx.arcTo(x, y, x + w, y, rr);
            ctx.closePath();
        }

        function frameOf(clip) {
            const art = h.art;
            const c = art && art.clips && art.clips[clip];
            if (!c) return null;
            const f = c.sheet.frames[c.frames[0]];
            return f && f[2] > 0 ? { img: c.sheet.img, f: f } : null;
        }

        // Darkened copy of a facade frame, built once per clip (transparent pixels stay transparent).
        function shadedFrame(clip, fr) {
            const hit = shaded[clip];
            if (hit && hit.src === fr.img) return hit.canvas;
            if (!root.document) return null;
            const cv = root.document.createElement('canvas');
            cv.width = fr.f[2];
            cv.height = fr.f[3];
            const g = cv.getContext('2d');
            if (!g) return null;
            g.drawImage(fr.img, fr.f[0], fr.f[1], fr.f[2], fr.f[3], 0, 0, fr.f[2], fr.f[3]);
            g.globalCompositeOperation = 'source-atop';
            g.fillStyle = 'rgba(8, 6, 14, ' + FACADE_SHADE + ')';
            g.fillRect(0, 0, fr.f[2], fr.f[3]);
            shaded[clip] = { src: fr.img, canvas: cv };
            return cv;
        }

        function groundStrip(cx, y, w) {
            const g = ctx.createLinearGradient(0, y - 3, 0, y + 9);
            g.addColorStop(0, 'rgba(6, 8, 14, 0.34)');
            g.addColorStop(1, 'rgba(6, 8, 14, 0)');
            ctx.save();
            ctx.fillStyle = g;
            ctx.beginPath();
            ctx.ellipse(cx, y + 2, w * 0.5, 7, 0, 0, Math.PI * 2);
            ctx.fill();
            ctx.restore();
        }

        // Sprite at the clip's frame with its pivot at (x, y) and the frame height scaled to `height`.
        function drawFramed(clip, x, y, height, flip) {
            const fr = frameOf(clip);
            if (!fr) return false;
            const cv = shadedFrame(clip, fr);
            if (!cv) return false;
            const sc = height / fr.f[3];
            const dw = fr.f[2] * sc;
            const dh = fr.f[3] * sc;
            ctx.save();
            ctx.imageSmoothingEnabled = true;
            ctx.imageSmoothingQuality = 'high';
            ctx.translate(x, y);
            if (flip) ctx.scale(-1, 1);
            ctx.drawImage(cv, 0, 0, cv.width, cv.height, fr.f[4] * sc, fr.f[5] * sc, dw, dh);
            ctx.restore();
            return true;
        }

        function fallbackDoor(door) {
            const kind = door.kind;
            const w = door.w || 56;
            const x = door.x;
            const y = door.y;
            const doorH = kind === 'arch' ? 118 : 96;
            ctx.save();
            ctx.fillStyle = FALLBACK_BODY[kind] || '#5a3d26';
            rrect(x - w * 0.9, y - doorH - 18, w * 1.8, doorH + 18, 8);
            ctx.fill();
            ctx.fillStyle = 'rgba(255,255,255,0.10)';
            ctx.fillRect(x - w * 0.9 + 3, y - doorH - 15, w * 1.8 - 6, 5);
            ctx.fillStyle = '#07060b';
            if (kind === 'arch' || kind === 'cave') {
                ctx.beginPath();
                ctx.moveTo(x - w / 2, y);
                ctx.lineTo(x - w / 2, y - doorH + w / 2);
                ctx.arc(x, y - doorH + w / 2, w / 2, Math.PI, 0);
                ctx.lineTo(x + w / 2, y);
                ctx.closePath();
                ctx.fill();
            } else {
                ctx.fillRect(x - w / 2, y - doorH, w, doorH);
            }
            ctx.fillStyle = 'rgba(0,0,0,0.5)';
            ctx.fillRect(x - w * 0.9, y - doorH - 8, w * 1.8, 8);
            ctx.restore();
            groundStrip(x, y, w * 2);
        }

        function isSealed(sim, door) {
            const P = root.RunenpfadPlaces;
            try { return !!(P && typeof P.sealed === 'function' && door && door.placeId && P.sealed(sim, door.placeId)); } catch (err) { return false; }
        }

        // Rune seal over a sealed door: a slow amber glow with a small ring of runes (static with reduced motion).
        function drawSeal(x, y, frame) {
            const pulse = frame && frame.reducedMotion ? 0.8 : 0.65 + 0.35 * Math.sin((frame ? frame.nowSec : 0) * 2.4);
            if (typeof ctx.createRadialGradient !== 'function') return;
            const g = ctx.createRadialGradient(x, y, 0, x, y, 54);
            g.addColorStop(0, 'rgba(255, 210, 120, ' + (0.5 * pulse).toFixed(3) + ')');
            g.addColorStop(1, 'rgba(255, 210, 120, 0)');
            ctx.save();
            ctx.globalCompositeOperation = 'lighter';
            ctx.fillStyle = g;
            ctx.fillRect(x - 54, y - 54, 108, 108);
            ctx.globalCompositeOperation = 'source-over';
            ctx.strokeStyle = 'rgba(255, 226, 150, ' + (0.55 + 0.3 * pulse).toFixed(3) + ')';
            ctx.lineWidth = 2;
            ctx.beginPath();
            ctx.arc(x, y, 18, 0, Math.PI * 2);
            ctx.stroke();
            for (let i = 0; i < 6; i++) {
                const a = i * Math.PI / 3;
                ctx.beginPath();
                ctx.moveTo(x + Math.cos(a) * 18, y + Math.sin(a) * 18);
                ctx.lineTo(x + Math.cos(a) * 25, y + Math.sin(a) * 25);
                ctx.stroke();
            }
            ctx.restore();
        }

        function drawFacade(door, sim, frame) {
            const fc = door.facade;
            if (!fc) { fallbackDoor(door); return; }
            const cx = door.x + (fc.dx || 0);
            const sealed = isSealed(sim, door);
            // The sealed frame is f1 of the same sheet; the shade cache is per clip.
            const sealedClip = sealed ? String(fc.clip).replace(/\.f0$/, '.f1') : fc.clip;
            const clip = sealedClip !== fc.clip && frameOf(sealedClip) ? sealedClip : fc.clip;
            groundStrip(cx, door.y, Math.max(90, fc.h * 0.75));
            if (!drawFramed(clip, cx, door.y, fc.h, false)) fallbackDoor(door);
            if (sealed) drawSeal(door.x, door.y - Math.min(70, fc.h * 0.2), frame);
        }

        // Plain props of the hub (and decor without a door): { clip, x, y, h, flip? }.
        function drawProps(st, frame) {
            const props = list(st.hubProps).concat(list(st.decor));
            for (let i = 0; i < props.length; i++) {
                const p = props[i];
                if (!p || !p.clip || p.x + 200 < frame.sx || p.x - 200 > frame.sx + frame.viewW) continue;
                groundStrip(p.x, p.y, (p.h || 80) * 0.6);
                drawFramed(p.clip, p.x, p.y, p.h || 80, !!p.flip);
            }
        }

        function drawBarkSprite(b, frame) {
            const art = h.art;
            const A = Art();
            const sh = b.sheet || 'npc-villager';
            let clip = sh + (b.near ? '.talk' : '.idle');
            if (!(art && A && A.has(art, clip))) clip = sh + '.idle';
            h.drawContactShadow(b.x, b.y, 40);
            if (art && A && A.has(art, clip)) {
                const o = { height: BARK_H, flip: b.facing < 0 };
                if (frame.reducedMotion) o.frame = 0;
                A.draw(ctx, art, clip, frame.nowSec + (b.x % 7) * 0.31, b.x, b.y, o);
                return;
            }
            ctx.save();
            ctx.fillStyle = b.sheet === 'npc-guard' ? '#6b7a99' : '#b08a5a';
            rrect(b.x - 11, b.y - 52, 22, 52, 10);
            ctx.fill();
            ctx.fillStyle = '#e8c9a0';
            ctx.beginPath();
            ctx.arc(b.x, b.y - 58, 9, 0, Math.PI * 2);
            ctx.fill();
            ctx.restore();
        }

        function wrap(text, maxW) {
            const words = String(text).split(/\s+/);
            const lines = [];
            let cur = '';
            for (let i = 0; i < words.length; i++) {
                const t = cur ? cur + ' ' + words[i] : words[i];
                if (cur && ctx.measureText(t).width > maxW) { lines.push(cur); cur = words[i]; } else cur = t;
            }
            if (cur) lines.push(cur);
            return lines.slice(0, 4);
        }

        function drawBubble(b, frame) {
            const lines = list(b.lines);
            if (!b.near || !lines.length) return;
            const key = lines[(b.line | 0) % lines.length];
            const txt = tr(key, '');
            if (!txt) return;
            ctx.save();
            ctx.font = fontFor(12);
            ctx.textAlign = 'left';
            const rows = wrap(txt, 260);
            let w = 0;
            for (let i = 0; i < rows.length; i++) w = Math.max(w, ctx.measureText(rows[i]).width);
            const bw = w + 20;
            const bh = rows.length * 16 + 12;
            const bob = frame.reducedMotion ? 0 : Math.sin(frame.nowSec * 2.2 + b.x) * 1.5;
            const bx = b.x - bw / 2;
            const by = b.y - BARK_H - 16 - bh + bob;
            ctx.fillStyle = 'rgba(250, 244, 226, 0.96)';
            ctx.strokeStyle = 'rgba(40, 28, 16, 0.9)';
            ctx.lineWidth = 2;
            rrect(bx, by, bw, bh, 8);
            ctx.fill();
            ctx.stroke();
            ctx.beginPath();
            ctx.moveTo(b.x - 6, by + bh - 1);
            ctx.lineTo(b.x, by + bh + 8);
            ctx.lineTo(b.x + 6, by + bh - 1);
            ctx.closePath();
            ctx.fill();
            ctx.fillStyle = '#2a1d10';
            for (let i = 0; i < rows.length; i++) ctx.fillText(rows[i], bx + 10, by + 18 + i * 16);
            ctx.restore();
        }

        function keyCap(label, x, y) {
            ctx.save();
            ctx.font = fontFor(13);
            ctx.textAlign = 'center';
            const w = Math.max(24, ctx.measureText(label).width + 14);
            ctx.fillStyle = 'rgba(0, 0, 0, 0.35)';
            rrect(x - w / 2, y - 12, w, 28, 6);
            ctx.fill();
            ctx.fillStyle = '#f6edd6';
            ctx.strokeStyle = '#5b4a30';
            ctx.lineWidth = 2;
            rrect(x - w / 2, y - 15, w, 28, 6);
            ctx.fill();
            ctx.stroke();
            ctx.fillStyle = '#2a1d10';
            ctx.fillText(label, x, y + 5);
            ctx.restore();
            return w;
        }

        function drawDoorPrompt(door, frame) {
            const fh = door.facade ? door.facade.h : 240;
            const top = door.y - Math.min(140, fh * 0.46);
            const bob = frame.reducedMotion ? 0 : Math.sin(frame.nowSec * 4.2) * 3.5;
            const x = door.x;
            ctx.save();
            ctx.fillStyle = '#ffe28a';
            ctx.strokeStyle = '#3a2a10';
            ctx.lineWidth = 2;
            ctx.lineJoin = 'round';
            ctx.beginPath();
            ctx.moveTo(x - 9, top - 14 + bob);
            ctx.lineTo(x + 9, top - 14 + bob);
            ctx.lineTo(x, top - 2 + bob);
            ctx.closePath();
            ctx.fill();
            ctx.stroke();
            ctx.restore();
            const w1 = keyCap('E', x - 18, top - 34 + bob);
            keyCap('↑', x + 14 + w1 / 2 - 12, top - 34 + bob);
        }

        // ---- public ----

        function drawBack(sim, frame) {
            try {
                const st = sim && sim.stage;
                if (!st || !frame) return;
                const doors = list(st.doors);
                const x0 = frame.sx - 260;
                const x1 = frame.sx + frame.viewW + 260;
                if (st.hub) drawProps(st, frame);
                for (let i = 0; i < doors.length; i++) {
                    const d = doors[i];
                    if (d.x < x0 || d.x > x1) continue;
                    drawFacade(d, sim, frame);
                }
                if (st.hub) list(st.barks).forEach(function (b) { if (b.x > x0 && b.x < x1) drawBarkSprite(b, frame); });
            } catch (err) { /* art problems must never stop the frame */ }
        }

        function drawFront(sim, frame) {
            try {
                const st = sim && sim.stage;
                if (!st || !frame) return;
                if (sim.interior || sim.encounter) return;
                const doors = list(st.doors);
                for (let i = 0; i < doors.length; i++) if (doors[i].near) drawDoorPrompt(doors[i], frame);
                if (st.hub) list(st.barks).forEach(function (b) { drawBubble(b, frame); });
            } catch (err) { /* ignore */ }
        }

        function dispose() {
            Object.keys(shaded).forEach(function (k) { delete shaded[k]; });
            Object.keys(requested).forEach(function (k) { delete requested[k]; });
        }

        return { prime: prime, drawBack: drawBack, drawFront: drawFront, dispose: dispose };
    }

    root.RunenpfadPlaceView = { create: create };
})(typeof window !== 'undefined' ? window : globalThis);
