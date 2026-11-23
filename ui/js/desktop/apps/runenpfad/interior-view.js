/* Runenpfad interior renderer: top-down room (baked layer, y-sorted props, chests, NPCs, mobs, hero), lights, prompts, floats and the iris wipe. Canvas only, read-only on the sim, never throws. */
(function (root) {
    'use strict';

    const CELL = 48;
    const HERO_H = 62;
    const FOLK_H = 62;
    const WIPE_S = 0.35;
    const TOP_INSET = 62; // the window's DOM top bar covers the first css pixels of the canvas
    const RIM = '#f4c874';
    const HERO_FALLBACK = { knight: '#5b8def', ranger: '#4fbf6a' };
    const MOB_COLOR = { rat: '#8b7a6a', slime: '#5ed06a', bat: '#6b4f8a', spider: '#3b3144', mummy: '#cdb98a', icewisp: '#8ad8ff', clockbug: '#c08a3a' };
    const MOB_R = { rat: 14, slime: 18, bat: 14, spider: 17, mummy: 20, icewisp: 16, clockbug: 16 };
    const FLYERS = { bat: 1, icewisp: 1 };
    const SHOT_COLOR = { venom: '#8df08a', spit: '#8df08a', shard: '#bfe9ff', ice: '#bfe9ff', ember: '#ffb060' };
    const ICON_FOR = { talk: 'talk', quest: 'quest', trade: 'trade', heart: 'heart', duel: 'duel', ask: 'ask' };
    const PROMPT_ICON = { talk: 'talk', open: 'coins', rest: 'heart', locked: null };

    function Art() { return root.RunenpfadArt || null; }
    function Bake() { return root.RunenpfadInteriorBake || null; }
    function clamp(v, lo, hi) { return v < lo ? lo : v > hi ? hi : v; }
    function list(v) { return Array.isArray(v) ? v : []; }

    function create(h) {
        const ctx = h.ctx;
        const requested = {};
        let requestedFor = null;
        let baked = null;
        let bakedRoom = null;
        let bakedSig = '';
        let lightCv = null;
        let lightCtx = null;
        const cam = { x: 0, y: 0, for: null, ready: false };
        const chestSeen = new WeakMap(); // chest -> { open: bool, t0: seconds } (visual clock only)
        const glows = {};
        let insetCache = TOP_INSET;
        let insetAge = 99;
        let TV = null; // things, guardians and prompt icons (interior-things-view.js), optional
        try { const TM = root.RunenpfadInteriorThingsView; TV = TM && typeof TM.create === 'function' ? TM.create(h) : null; } catch (err) { TV = null; }
        function mobSheet(kind) { return TV ? TV.mobSheet(kind) : 'td-' + kind; }

        // ---- loading ----

        function ask(id) {
            const A = Art();
            const art = h.art;
            if (!A || !art || !id || requested[id] || typeof A.loadSheet !== 'function') return;
            requested[id] = true;
            A.loadSheet(art, id);
        }

        function resetRequests() {
            if (requestedFor === h.art) return;
            requestedFor = h.art;
            Object.keys(requested).forEach(function (k) { delete requested[k]; });
        }

        function folkSheet(npcId) {
            const F = root.RunenpfadFolk;
            const sp = F && F.SPRITES ? F.SPRITES[npcId] : null;
            return sp ? { sheet: sp[0], row: sp[1] | 0 } : null;
        }

        function askRoomDef(def, extra) {
            if (!def) return;
            ask('tex-' + def.theme);
            list(def.things).forEach(function (t) {
                if (t.prop) ask(String(t.prop).split('.')[0]);
                if (t.mob) ask(mobSheet(t.mob));
                if (t.npc) { const f = folkSheet(t.npc); if (f) ask(f.sheet); }
            });
            if (TV) TV.sheetsOfDef(def).forEach(ask);
            void extra;
        }

        function askLive(sim) {
            const iv = sim.interior;
            if (!iv) return;
            ask('td-' + (sim.heroId === 'ranger' ? 'ranger' : 'knight'));
            if (iv.room && iv.room.def) askRoomDef(iv.room.def);
            list(iv.room && iv.room.props).forEach(function (p) { ask(String(p.clip).split('.')[0]); });
            list(iv.mobs).forEach(function (m) { ask(mobSheet(m.kind)); });
            list(iv.npcs).forEach(function (n) { const f = folkSheet(n.npcId); if (f) ask(f.sheet); });
            // The other rooms of this place, so doors between rooms never show a bare floor.
            const R = root.RunenpfadPlaceRooms;
            const place = iv.place || (R && R.PLACES ? R.PLACES[iv.placeId] : null);
            if (place && R && R.ROOMS) list(place.rooms).forEach(function (id) { askRoomDef(R.ROOMS[id]); });
        }

        // Called every level frame: warms the sheets of the active place, or of the place whose door the hero stands at.
        function prime(sim) {
            if (!sim || !h.art) return;
            resetRequests();
            if (sim.interior) { askLive(sim); return; }
            const doors = sim.stage && sim.stage.doors;
            const R = root.RunenpfadPlaceRooms;
            if (!doors || !R || !R.PLACES || !R.ROOMS) return;
            for (let i = 0; i < doors.length; i++) {
                if (!doors[i].near) continue;
                ask('td-' + (sim.heroId === 'ranger' ? 'ranger' : 'knight'));
                const place = R.PLACES[doors[i].placeId];
                if (place) list(place.rooms).forEach(function (id) { askRoomDef(R.ROOMS[id]); });
            }
        }

        function wants(sim) { return !!(sim && sim.interior); }

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

        function makeCanvas(w, hh) {
            const doc = ctx.canvas && ctx.canvas.ownerDocument;
            if (!doc || typeof doc.createElement !== 'function') return null;
            const cv = doc.createElement('canvas');
            cv.width = Math.max(1, Math.round(w));
            cv.height = Math.max(1, Math.round(hh));
            return cv;
        }

        function hasClip(clip) {
            const A = Art();
            return !!(A && h.art && A.has(h.art, clip));
        }

        // Atlas sprite, pivot at (x, y); pixel-crisp once the sprite is magnified.
        function spr(clip, time, x, y, height, o, screen) {
            const A = Art();
            const art = h.art;
            if (!A || !art || !A.has(art, clip)) return false;
            const opt = o || {};
            opt.height = height;
            opt.pixel = (height / art.clips[clip].sheet.refH) * screen >= 0.95;
            return A.draw(ctx, art, clip, time, x, y, opt);
        }

        function rrect(x, y, w, hh, r) {
            const B = Bake();
            if (B) { B.roundRect(ctx, x, y, w, hh, r); return; }
            ctx.beginPath();
            ctx.rect(x, y, w, hh);
        }

        function shadow(x, y, w, alpha) {
            ctx.save();
            ctx.fillStyle = 'rgba(4, 6, 12, ' + (alpha == null ? 0.34 : alpha) + ')';
            ctx.beginPath();
            ctx.ellipse(x, y - 1, w, Math.max(4, w * 0.36), 0, 0, Math.PI * 2);
            ctx.fill();
            ctx.restore();
        }

        function glowCanvas(color) {
            if (glows[color]) return glows[color];
            const c = makeCanvas(64, 64);
            const g = c ? c.getContext('2d') : null;
            if (!g) return null;
            const rg = g.createRadialGradient(32, 32, 0, 32, 32, 32);
            rg.addColorStop(0, color);
            rg.addColorStop(0.4, color.replace(/[\d.]+\)$/, '0.35)'));
            rg.addColorStop(1, 'rgba(0,0,0,0)');
            g.fillStyle = rg;
            g.fillRect(0, 0, 64, 64);
            glows[color] = c;
            return c;
        }

        function glow(x, y, r, color, alpha, add) {
            const c = glowCanvas(color);
            if (!c) return;
            ctx.save();
            ctx.globalAlpha = alpha;
            if (add !== false) ctx.globalCompositeOperation = 'lighter';
            ctx.drawImage(c, x - r, y - r, r * 2, r * 2);
            ctx.restore();
        }

        function text(str, x, y, px, color, align) {
            ctx.save();
            ctx.font = fontFor(px);
            ctx.textAlign = align || 'center';
            ctx.lineJoin = 'round';
            ctx.lineWidth = 3;
            ctx.strokeStyle = '#09111a';
            ctx.strokeText(str, x, y);
            ctx.fillStyle = color || '#fff6dc';
            ctx.fillText(str, x, y);
            ctx.restore();
        }

        // How far the window's DOM top bar reaches into the canvas (measured now and then, it wraps on narrow windows).
        function topInset(vh) {
            if (vh <= 400) return 0;
            if (++insetAge > 30) {
                insetAge = 0;
                try {
                    const cv = ctx.canvas;
                    const shell = cv && cv.closest ? cv.closest('.rp-app') : null;
                    const bar = shell ? shell.querySelector('.rp-topbar') : null;
                    if (bar) insetCache = clamp(Math.ceil(bar.getBoundingClientRect().bottom - cv.getBoundingClientRect().top) + 4, 0, vh * 0.3);
                } catch (err) { insetCache = TOP_INSET; }
            }
            return insetCache;
        }

        // ---- room layer ----

        function ensureBake(room) {
            const B = Bake();
            if (!B || !room) return;
            const sig = B.signature(room, h.art);
            if (bakedRoom === room && bakedSig === sig && baked) return;
            bakedRoom = room;
            bakedSig = sig;
            baked = B.bakeRoom(room, h.art, makeCanvas);
        }

        function drawRoomLayer(room) {
            if (baked) {
                ctx.imageSmoothingEnabled = true;
                ctx.drawImage(baked, 0, 0, room.w, room.h);
                return;
            }
            // Without a DOM bake (or before the first one) paint a flat plan so the room never vanishes.
            const map = room.def && room.def.map ? room.def.map : [];
            for (let cy = 0; cy < map.length; cy++) {
                for (let cx = 0; cx < map[cy].length; cx++) {
                    const ch = map[cy].charAt(cx);
                    ctx.fillStyle = ch === '#' ? '#241810' : ch === ' ' ? '#000' : '#6a4a2c';
                    ctx.fillRect(cx * CELL, cy * CELL, CELL, CELL);
                }
            }
        }

        // ---- objects ----

        function drawProp(p, screen) {
            const B = Bake();
            const w = Math.max(18, (p.wCells || 1) * CELL * 0.94);
            shadow(p.x, p.y, w * 0.46, 0.3);
            if (B) B.drawProp(ctx, h.art, p, { screen: screen });
        }

        function chestState(c, iv) {
            let s = chestSeen.get(c);
            if (!s) { s = { open: !!c.opened, t0: c.opened ? -99 : 0 }; chestSeen.set(c, s); }
            if (c.opened && !s.open) { s.open = true; s.t0 = iv.t; }
            if (!c.opened) s.open = false;
            return s;
        }

        function drawChest(c, iv, f) {
            const s = chestState(c, iv);
            const A = Art();
            const art = h.art;
            shadow(c.x, c.y, 26, 0.34);
            const bob = f.reduced ? 0.7 : 0.55 + 0.45 * Math.sin(f.t * 3 + c.x);
            if (!c.opened && !c.locked) glow(c.x, c.y - 22, 54, 'rgba(255, 214, 120, 0.8)', 0.4 * bob);
            if (art && A && A.has(art, 'prop-chest.closed')) {
                if (c.opened) A.draw(ctx, art, 'prop-chest.open', Math.max(0, iv.t - s.t0), c.x, c.y, { scale: 0.27 });
                else A.draw(ctx, art, 'prop-chest.closed', 0, c.x, c.y, { scale: 0.27, tint: c.locked ? '#1a1a2e' : '', tintAmount: 0.25 });
            } else {
                ctx.save();
                ctx.fillStyle = c.opened ? '#6a4a24' : '#9a6a30';
                ctx.strokeStyle = '#2a1a0a';
                ctx.lineWidth = 2;
                rrect(c.x - 22, c.y - 30, 44, 30, 6);
                ctx.fill();
                ctx.stroke();
                ctx.fillStyle = c.opened ? '#1a1008' : '#d9a93c';
                ctx.fillRect(c.x - 22, c.y - 20, 44, 5);
                ctx.restore();
            }
            if (c.locked && !c.opened) {
                ctx.save();
                ctx.fillStyle = '#c9ced8';
                ctx.strokeStyle = '#20242c';
                ctx.lineWidth = 2;
                rrect(c.x - 7, c.y - 46, 14, 11, 3);
                ctx.fill();
                ctx.stroke();
                ctx.beginPath();
                ctx.arc(c.x, c.y - 46, 5, Math.PI, 0);
                ctx.stroke();
                ctx.restore();
            }
        }

        function drawPickup(p, f) {
            if (p.kind === 'key' && TV && TV.drawKey(p, f)) return;
            const bob = f.reduced ? 0 : Math.sin(f.t * 4 + p.x * 0.05) * 3;
            shadow(p.x, p.y, 10, 0.3);
            glow(p.x, p.y - 14 + bob, 28, p.kind === 'heart' ? 'rgba(253, 164, 175, 0.8)' : p.kind === 'potion' ? 'rgba(167, 139, 250, 0.8)' : 'rgba(255, 224, 102, 0.8)', 0.55);
            if (h.sheets && h.sheets['jump-and-run'] && typeof h.drawStaticAsset === 'function') {
                h.drawStaticAsset('jump-and-run', p.kind, p.x - 14, p.y - 32 + bob, 28, 28);
                return;
            }
            ctx.save();
            ctx.fillStyle = p.kind === 'heart' ? '#f87185' : p.kind === 'potion' ? '#a78bfa' : '#fbbf24';
            ctx.strokeStyle = '#2a1a0a';
            ctx.lineWidth = 2;
            ctx.beginPath();
            ctx.arc(p.x, p.y - 16 + bob, 8, 0, Math.PI * 2);
            ctx.fill();
            ctx.stroke();
            ctx.restore();
        }

        function folkClip(n, talking) {
            const f = folkSheet(n.npcId);
            if (f) return { clip: f.sheet + '.r' + f.row + '_' + (talking ? 'talk' : 'idle'), height: FOLK_H };
            return { clip: 'npc-' + n.npcId + '.' + (talking ? 'talk' : 'idle'), height: 72 };
        }

        function npcDef(n) {
            const N = root.RunenpfadNpcs;
            return N && N.NPCS ? N.NPCS[n.npcId] : null;
        }

        function drawCapsule(x, y, w, hh, color, face) {
            ctx.save();
            ctx.fillStyle = color;
            ctx.strokeStyle = 'rgba(0,0,0,0.6)';
            ctx.lineWidth = 2;
            rrect(x - w / 2, y - hh, w, hh * 0.72, w / 2);
            ctx.fill();
            ctx.stroke();
            ctx.fillStyle = '#ecd2ac';
            ctx.beginPath();
            ctx.arc(x, y - hh + w * 0.45, w * 0.4, 0, Math.PI * 2);
            ctx.fill();
            ctx.stroke();
            if (face) {
                ctx.fillStyle = '#2a1d10';
                ctx.fillRect(x + face * w * 0.14 - 1.5, y - hh + w * 0.4, 3, 3);
            }
            ctx.restore();
        }

        function drawNpc(n, sim, f) {
            const enc = sim.encounter;
            const talking = !!(enc && enc.npcRef === n.ref);
            const def = npcDef(n);
            const c = folkClip(n, talking);
            const flip = n.facing < 0;
            const hurt = n.hurtT > 0 ? Math.min(0.8, n.hurtT * 3) : 0;
            shadow(n.x, n.y, 17, 0.36);
            const o = { flip: flip, flash: hurt };
            if (f.reduced) o.frame = 0;
            const t = f.t + (n.x % 5) * 0.2;
            if (!(hasClip(c.clip) && spr(c.clip, t, n.x, n.y, c.height, o, f.s))) {
                const o2 = { flip: flip, flash: hurt };
                if (!(hasClip('npc-' + n.npcId + '.idle') && spr('npc-' + n.npcId + '.' + (talking && hasClip('npc-' + n.npcId + '.talk') ? 'talk' : 'idle'), t, n.x, n.y, 72, o2, f.s))) {
                    drawCapsule(n.x, n.y, 26, 58, (def && def.color) || '#b88a5a', flip ? -1 : 1);
                }
            }
        }

        function drawNpcOverlay(n, sim, f) {
            const enc = sim.encounter;
            if (enc && enc.npcRef === n.ref) return;
            const bob = f.reduced ? 0 : Math.sin(f.t * 3 + n.x * 0.05) * 2.5;
            let icon = null;
            if (n.marker) icon = ICON_FOR[n.marker] || 'talk';
            else if (n.near && n.marker !== null) icon = 'talk';
            const pr = sim.interior && sim.interior.prompt;
            // The prompt of the targeted NPC already carries the icon.
            if (pr && pr.kind === 'talk' && Math.abs(pr.x - n.x) < 6 && Math.abs(pr.y - n.y) < 6) icon = null;
            if (icon) {
                const key = 'ui-npc-icons.' + icon;
                const y = n.y - FOLK_H - 22 + bob;
                if (hasClip(key)) spr(key, 0, n.x, y, 28, { alpha: 1 }, f.s);
                else {
                    ctx.save();
                    ctx.fillStyle = '#fff6dc';
                    ctx.strokeStyle = '#09111a';
                    ctx.lineWidth = 3;
                    rrect(n.x - 11, y - 12, 22, 20, 6);
                    ctx.fill();
                    ctx.stroke();
                    ctx.restore();
                }
            }
            if (n.near) {
                const def = npcDef(n);
                const name = def ? tr(def.nameKey, '') : '';
                if (name) text(name, n.x, n.y + 14, 12, (def && def.color) || '#ffe066');
            }
        }

        function mobDef(kind) {
            const M = root.RunenpfadInteriorMobs;
            return M && M.DEFS ? M.DEFS[kind] : null;
        }

        function drawMob(m, f) {
            const def = mobDef(m.kind);
            const r = m.r > 0 ? m.r : (def && def.r) || MOB_R[m.kind] || 12;
            const boss = !!(TV && TV.isBoss(m));
            const height = boss ? TV.bossHeight(clamp(r * 4.6, 34, 84)) : clamp(r * 4.6, 34, 84);
            const dead = m.state === 'dead';
            let alpha = 1;
            if (dead) alpha = clamp(1 - ((m.deadT || 0) - 0.12) / 0.45, 0, 1);
            if (alpha <= 0.01) return;
            const lift = FLYERS[m.kind] && !dead ? 16 + (f.reduced ? 0 : Math.sin(f.t * 6 + m.x) * 4) : 0;
            shadow(m.x, m.y, r * 1.1 * (1 - lift / 90), 0.32 * alpha);
            const clip = mobSheet(m.kind) + '.' + (dead ? 'death' : m.state === 'tell' ? 'tell' : m.state === 'attack' ? 'attack' : m.state === 'hurt' ? 'hurt' : 'move');
            const flash = m.hurtT > 0 && !dead ? Math.min(0.85, m.hurtT * 6) : 0;
            const o = { flip: m.face < 0, alpha: alpha, flash: flash };
            if (f.reduced) o.frame = 0;
            const y = m.y - lift;
            if (hasClip(clip) && spr(clip, m.state === 'move' ? f.t + m.x * 0.01 : 0, m.x, y, height, o, f.s)) { if (boss) TV.drawBossOverlay(m, f); return; }
            ctx.save();
            ctx.globalAlpha = alpha;
            ctx.fillStyle = flash > 0.3 ? '#ffffff' : MOB_COLOR[m.kind] || (boss && TV.bossColor(m.kind)) || '#8a6a6a';
            ctx.strokeStyle = 'rgba(0,0,0,0.6)';
            ctx.lineWidth = 2;
            ctx.beginPath();
            ctx.ellipse(m.x, y - r * 0.9, r, r * 0.85, 0, 0, Math.PI * 2);
            ctx.fill();
            ctx.stroke();
            ctx.fillStyle = '#10080a';
            ctx.fillRect(m.x - r * 0.45, y - r * 1.1, 3, 5);
            ctx.fillRect(m.x + r * 0.2, y - r * 1.1, 3, 5);
            ctx.restore();
            if (boss) TV.drawBossOverlay(m, f);
        }

        // Pulsing warning ring on the floor for the whole tell.
        function drawTell(m, f) {
            if (m.state !== 'tell') return;
            const def = mobDef(m.kind);
            const r = Math.max(26, (m.r > 0 ? m.r : (def && def.r) || 16) * 2.8);
            const p = f.reduced ? 0.8 : 0.55 + 0.45 * Math.sin((m.t || 0) * 22);
            ctx.save();
            ctx.fillStyle = 'rgba(239, 68, 68, ' + (0.16 + 0.14 * p).toFixed(3) + ')';
            ctx.strokeStyle = 'rgba(255, 120, 100, ' + (0.55 + 0.4 * p).toFixed(3) + ')';
            ctx.lineWidth = 3;
            ctx.beginPath();
            ctx.ellipse(m.x, m.y - 2, r * (0.9 + 0.1 * p), r * 0.62 * (0.9 + 0.1 * p), 0, 0, Math.PI * 2);
            ctx.fill();
            ctx.stroke();
            ctx.restore();
        }

        // ---- hero ----

        function heroClip(hero, id) {
            const base = 'td-' + id + '.';
            const dir = hero.dir === 'left' ? 'right' : (hero.dir || 'down');
            const flip = hero.dir === 'left';
            if (hero.hurtT > 0 && hasClip(base + 'hurt')) return { clip: base + 'hurt', flip: flip, frame: 0 };
            if (hero.attackT > 0 && hasClip(base + 'atk_' + dir)) return { clip: base + 'atk_' + dir, flip: flip, frame: 0 };
            return { clip: base + dir, flip: flip, frame: hero.moving ? -1 : 0 };
        }

        function heroSprite(hero, id, x, y, alpha, flash, tint, f) {
            const c = heroClip(hero, id);
            if (!hasClip(c.clip)) return false;
            const o = { flip: c.flip, alpha: alpha, flash: flash };
            if (tint) { o.tint = tint; o.tintAmount = 0.6; }
            if (c.frame >= 0 || f.reduced) o.frame = Math.max(0, c.frame);
            return spr(c.clip, hero.walkT || 0, x, y, HERO_H, o, f.s);
        }

        function drawHero(iv, sim, f) {
            const hero = iv.hero;
            const id = sim.heroId === 'ranger' ? 'ranger' : 'knight';
            shadow(hero.x, hero.y, 18, 0.38);
            if (!f.reduced && hero.dashT > 0) {
                const sp = Math.hypot(hero.vx || 0, hero.vy || 0) || 1;
                const nx = (hero.vx || 0) / sp;
                const ny = (hero.vy || 0) / sp;
                for (let i = 3; i >= 1; i--) {
                    const gx = hero.x - nx * i * 14;
                    const gy = hero.y - ny * i * 14;
                    const ga = 0.34 - i * 0.07;
                    if (!heroSprite(hero, id, gx, gy, ga, 0, '#9fd4ff', f)) {
                        ctx.save();
                        ctx.globalAlpha = ga;
                        drawCapsule(gx, gy, 24, HERO_H - 4, '#9fd4ff', 0);
                        ctx.restore();
                    }
                }
            }
            const hurt = hero.hurtT > 0 ? Math.min(0.9, hero.hurtT * 4 + 0.3) : 0;
            let alpha = 1;
            if (hero.invuln > 0 && !(hero.dashT > 0) && !(hero.hurtT > 0)) alpha = f.reduced ? 0.6 : (Math.floor(f.t * 16) & 1 ? 0.35 : 1);
            if (!heroSprite(hero, id, hero.x, hero.y, alpha, hurt, '', f)) {
                ctx.save();
                ctx.globalAlpha = alpha;
                drawCapsule(hero.x, hero.y, 24, HERO_H - 4, hurt > 0.3 ? '#ffffff' : HERO_FALLBACK[id], hero.dir === 'left' ? -1 : hero.dir === 'right' ? 1 : 0);
                ctx.restore();
            }
        }

        // Pale swing arc in the facing direction while the attack lasts.
        function drawSwing(iv, sim) {
            const hero = iv.hero;
            if (!(hero.attackT > 0)) return;
            const knight = sim.heroId !== 'ranger';
            const arc = (knight ? 120 : 70) * Math.PI / 180;
            const reach = knight ? 52 : 46;
            const dir = hero.dir === 'up' ? -Math.PI / 2 : hero.dir === 'left' ? Math.PI : hero.dir === 'right' ? 0 : Math.PI / 2;
            const p = clamp(hero.attackT / 0.3, 0, 1);
            const cy = hero.y - 24;
            ctx.save();
            ctx.globalCompositeOperation = 'lighter';
            ctx.lineCap = 'round';
            for (let i = 0; i < 3; i++) {
                ctx.strokeStyle = 'rgba(235, 245, 255, ' + (0.18 + 0.5 * p * (1 - i * 0.3)).toFixed(3) + ')';
                ctx.lineWidth = 9 - i * 3;
                ctx.beginPath();
                ctx.arc(hero.x, cy, reach - i * 4, dir - arc / 2, dir + arc / 2);
                ctx.stroke();
            }
            ctx.restore();
        }

        // ---- shots, floats, prompt ----

        function drawShot(s, f) {
            if (TV && TV.drawShot(s)) return;
            const col = SHOT_COLOR[s.kind] || (TV && TV.shotColor(s.kind)) || '#ffb060';
            const sp = Math.hypot(s.vx || 0, s.vy || 0) || 1;
            const tx = (s.vx || 0) / sp;
            const ty = (s.vy || 0) / sp;
            const r = Math.max(4, s.r || 6);
            const y = s.y - 12;
            shadow(s.x, s.y + 2, r * 0.9, 0.22);
            ctx.save();
            ctx.globalCompositeOperation = 'lighter';
            const trail = f.reduced ? 0 : 0.07 * sp;
            if (trail > 1) {
                const g = ctx.createLinearGradient(s.x, y, s.x - tx * trail, y - ty * trail);
                g.addColorStop(0, col);
                g.addColorStop(1, 'rgba(0,0,0,0)');
                ctx.strokeStyle = g;
                ctx.globalAlpha = 0.7;
                ctx.lineWidth = r * 1.4;
                ctx.lineCap = 'round';
                ctx.beginPath();
                ctx.moveTo(s.x, y);
                ctx.lineTo(s.x - tx * trail, y - ty * trail);
                ctx.stroke();
            }
            ctx.restore();
            glow(s.x, y, r * 3, 'rgba(255, 255, 255, 0.9)', 0.35);
            ctx.save();
            ctx.fillStyle = col;
            ctx.strokeStyle = 'rgba(255,255,255,0.85)';
            ctx.lineWidth = 1.5;
            ctx.beginPath();
            ctx.arc(s.x, y, r, 0, Math.PI * 2);
            ctx.fill();
            ctx.stroke();
            ctx.restore();
        }

        function drawFloats(iv) {
            const fl = list(iv.floats);
            if (!fl.length) return;
            ctx.save();
            ctx.font = fontFor(14);
            ctx.textAlign = 'center';
            ctx.lineJoin = 'round';
            ctx.lineWidth = 3;
            ctx.strokeStyle = '#09111a';
            fl.forEach(function (ft) {
                const word = ft.key ? tr(ft.key, '') : '';
                const label = word ? word + (ft.suffix || '') : String(ft.text || '');
                const x = ft.icon ? ft.x - 9 : ft.x;
                ctx.globalAlpha = clamp(ft.timer * 1.6, 0, 1);
                ctx.strokeText(label, x, ft.y);
                ctx.fillStyle = ft.color || '#ffe066';
                ctx.fillText(label, x, ft.y);
                if (ft.icon && h.sheets && h.sheets['jump-and-run']) h.drawStaticAsset('jump-and-run', ft.icon, x + ctx.measureText(label).width / 2, ft.y - 17, 22, 22);
            });
            ctx.restore();
        }

        function keyCap(label, x, y) {
            ctx.save();
            ctx.font = fontFor(14);
            ctx.textAlign = 'center';
            const w = Math.max(26, ctx.measureText(label).width + 14);
            ctx.fillStyle = 'rgba(0, 0, 0, 0.38)';
            rrect(x - w / 2, y - 12, w, 29, 6);
            ctx.fill();
            ctx.fillStyle = '#f6edd6';
            ctx.strokeStyle = '#5b4a30';
            ctx.lineWidth = 2;
            rrect(x - w / 2, y - 15, w, 29, 6);
            ctx.fill();
            ctx.stroke();
            ctx.fillStyle = '#2a1d10';
            ctx.fillText(label, x, y + 5);
            ctx.restore();
        }

        function drawPrompt(iv, f) {
            const p = iv.prompt;
            if (!p) return;
            const bob = f.reduced ? 0 : Math.sin(f.t * 4.2) * 3.5;
            const top = p.y - (p.kind === 'talk' ? FOLK_H + 14 : 54);
            const cy = top - 34 + bob;
            const icon = PROMPT_ICON[p.kind];
            if (p.kind === 'locked') {
                ctx.save();
                ctx.fillStyle = '#ff9a8a';
                ctx.strokeStyle = '#09111a';
                ctx.lineWidth = 3;
                rrect(p.x - 9, cy - 38, 18, 15, 4);
                ctx.fill();
                ctx.stroke();
                ctx.beginPath();
                ctx.arc(p.x, cy - 38, 6, Math.PI, 0);
                ctx.stroke();
                ctx.restore();
            } else if (TV && TV.promptIcon(p.kind, p.x, cy - 28)) {
                // new prompt kinds draw their own icon
            } else if (icon && hasClip('ui-npc-icons.' + icon)) {
                spr('ui-npc-icons.' + icon, 0, p.x, cy - 24, 26, {}, f.s);
            }
            keyCap('E', p.x, cy);
        }

        // ---- lights ----

        function ensureLight(w, hh) {
            if (!lightCv) {
                lightCv = makeCanvas(w, hh);
                lightCtx = lightCv ? lightCv.getContext('2d') : null;
            }
            if (!lightCtx) return null;
            if (lightCv.width !== w || lightCv.height !== hh) { lightCv.width = w; lightCv.height = hh; }
            return lightCtx;
        }

        function flicker(l, i, f) {
            if (!l.flicker || f.reduced) return 1;
            return 1 + 0.06 * Math.sin(f.t * 9.3 + i * 1.7) + 0.04 * Math.sin(f.t * 23.1 + i * 3.1) + 0.03 * Math.sin(f.t * 41 + i);
        }

        // Static room lights plus the dynamic ones of the things (lit braziers, lecterns with a new page).
        function lightsOf(room, iv) {
            const base = list(room.lights);
            const T = root.RunenpfadInteriorThings;
            if (!T || typeof T.lights !== 'function' || !iv) return base;
            try { const dyn = T.lights(iv); return Array.isArray(dyn) && dyn.length ? base.concat(dyn) : base; } catch (err) { return base; }
        }

        // Darkness over the screen, punched by the room lights and the hero light (screen space).
        function darkPass(room, iv, f, vw, vh) {
            const dark = clamp(room.def && room.def.dark > 0 ? room.def.dark : 0, 0, 0.9);
            if (dark <= 0.01) return;
            const q = 0.5;
            const lw = Math.max(2, Math.round(vw * q));
            const lh = Math.max(2, Math.round(vh * q));
            const g = ensureLight(lw, lh);
            if (!g) { ctx.fillStyle = 'rgba(4, 5, 12, ' + (dark * 0.8).toFixed(3) + ')'; ctx.fillRect(0, 0, vw, vh); return; }
            g.setTransform(1, 0, 0, 1, 0, 0);
            g.globalCompositeOperation = 'source-over';
            g.clearRect(0, 0, lw, lh);
            g.fillStyle = 'rgba(4, 5, 14, ' + dark.toFixed(3) + ')';
            g.fillRect(0, 0, lw, lh);
            g.globalCompositeOperation = 'destination-out';
            const punch = function (wx, wy, r, k) {
                const x = (wx - cam.x) * f.s * q;
                const y = ((wy - cam.y) * f.s + f.oy) * q;
                const rr = r * f.s * q;
                if (x < -rr || y < -rr || x > lw + rr || y > lh + rr) return;
                const rg = g.createRadialGradient(x, y, 0, x, y, rr);
                rg.addColorStop(0, 'rgba(0,0,0,' + clamp(k, 0, 1).toFixed(3) + ')');
                rg.addColorStop(0.55, 'rgba(0,0,0,' + clamp(k * 0.6, 0, 1).toFixed(3) + ')');
                rg.addColorStop(1, 'rgba(0,0,0,0)');
                g.fillStyle = rg;
                g.fillRect(x - rr, y - rr, rr * 2, rr * 2);
            };
            const lights = lightsOf(room, iv);
            for (let i = 0; i < lights.length; i++) {
                const l = lights[i];
                punch(l.x, l.y, (l.r || 160) * flicker(l, i, f), 1);
            }
            punch(iv.hero.x, iv.hero.y - 26, 150, 0.95);
            ctx.save();
            ctx.imageSmoothingEnabled = true;
            ctx.imageSmoothingQuality = 'high';
            ctx.drawImage(lightCv, 0, 0, lw, lh, 0, 0, vw, vh);
            ctx.restore();
        }

        // Warm additive glow at light sources (also in bright rooms, so fireplaces stay alive).
        function warmGlows(room, f, iv) {
            const lights = lightsOf(room, iv);
            const dark = room.def && room.def.dark > 0.01;
            for (let i = 0; i < lights.length; i++) {
                const l = lights[i];
                const col = l.color && /^#/.test(l.color) ? hexToRgba(l.color) : 'rgba(255, 176, 96, 0.9)';
                const k = flicker(l, i, f);
                glow(l.x, l.y, (l.r || 160) * 0.55 * k, col, (l.flicker ? 0.34 : 0.2) * (dark ? 0.8 : 1) * (0.9 + 0.1 * k));
            }
        }

        function hexToRgba(hex) {
            const m = /^#([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})$/i.exec(hex) || /^#([0-9a-f])([0-9a-f])([0-9a-f])$/i.exec(hex);
            if (!m) return 'rgba(255, 176, 96, 0.9)';
            const c = m.slice(1).map(function (v) { return parseInt(v.length === 1 ? v + v : v, 16); });
            return 'rgba(' + c[0] + ', ' + c[1] + ', ' + c[2] + ', 0.9)';
        }

        // ---- render ----

        function updateCamera(iv, room, f, vw, vh, dt) {
            const visW = vw / f.s;
            const visH = (vh - f.oy) / f.s;
            const tx = room.w <= visW ? (room.w - visW) / 2 : clamp(iv.hero.x - visW / 2, 0, room.w - visW);
            const ty = room.h <= visH ? (room.h - visH) / 2 : clamp(iv.hero.y - 20 - visH / 2, 0, room.h - visH);
            if (!cam.ready || cam.for !== room) {
                cam.x = tx;
                cam.y = ty;
                cam.ready = true;
                cam.for = room;
            } else {
                const k = 1 - Math.exp(-dt * 12);
                cam.x += (tx - cam.x) * k;
                cam.y += (ty - cam.y) * k;
            }
        }

        function render(sim, frame) {
            const iv = sim && sim.interior;
            if (!iv || !iv.room || !frame) return;
            const vw = frame.viewW;
            const vh = frame.viewH;
            const room = iv.room;
            const dpr = Math.min(2, root.devicePixelRatio || 1);
            const s = clamp(vh / (11 * CELL), 1, 2.5);
            const f = { s: s, t: frame.nowSec, reduced: !!frame.reducedMotion, oy: topInset(vh) };
            resetRequests();
            askLive(sim);
            ensureBake(room);
            updateCamera(iv, room, f, vw, vh, frame.dt || 0.016);

            ctx.save();
            ctx.fillStyle = '#000';
            ctx.fillRect(0, 0, vw, vh);
            const tx = -Math.round(cam.x * s * dpr) / dpr;
            const ty = -Math.round(cam.y * s * dpr) / dpr + f.oy;
            ctx.translate(tx, ty);
            ctx.scale(s, s);
            drawRoomLayer(room);

            // Floor markers first: tell rings sit under everything.
            const mobs = list(iv.mobs);
            for (let i = 0; i < mobs.length; i++) drawTell(mobs[i], f);
            if (TV) TV.drawFloor(iv, sim, f);

            const objs = [];
            list(room.props).forEach(function (p) { if (p.layer !== 'floor') objs.push({ y: p.y, k: 0, o: p }); });
            list(iv.chests).forEach(function (c) { objs.push({ y: c.y, k: 1, o: c }); });
            list(iv.pickups).forEach(function (p) { if (!p.taken) objs.push({ y: p.y, k: 2, o: p }); });
            list(iv.npcs).forEach(function (n) { if (n.state !== 'gone') objs.push({ y: n.y, k: 3, o: n }); });
            mobs.forEach(function (m) { objs.push({ y: m.y - (m.state === 'dead' ? 4 : 0), k: 4, o: m }); });
            if (TV) TV.sorted(iv).forEach(function (e) { objs.push({ y: e.y, k: 6, o: e.o }); });
            objs.push({ y: iv.hero.y, k: 5, o: iv.hero });
            objs.sort(function (a, b) { return a.y - b.y; });
            for (let i = 0; i < objs.length; i++) {
                const e = objs[i];
                try {
                    if (e.k === 0) drawProp(e.o, s);
                    else if (e.k === 1) drawChest(e.o, iv, f);
                    else if (e.k === 2) drawPickup(e.o, f);
                    else if (e.k === 3) drawNpc(e.o, sim, f);
                    else if (e.k === 4) drawMob(e.o, f);
                    else if (e.k === 6) TV.drawObject(e.o, iv, sim, f);
                    else { drawHero(iv, sim, f); drawSwing(iv, sim); }
                } catch (err) { /* a bad sprite must not stop the room */ }
            }
            list(iv.shots).forEach(function (sh) { drawShot(sh, f); });
            ctx.restore();

            darkPass(room, iv, f, vw, vh);

            ctx.save();
            ctx.translate(tx, ty);
            ctx.scale(s, s);
            warmGlows(room, f, iv);
            list(iv.npcs).forEach(function (n) { if (n.state !== 'gone') drawNpcOverlay(n, sim, f); });
            drawFloats(iv);
            drawPrompt(iv, f);
            ctx.restore();

            if (TV) TV.drawBossBar(iv, f, vw, f.oy);
            drawWipe(sim, vw, vh, frame);
        }

        // ---- wipe ----

        function coverOf(iv) {
            const w = iv && iv.wipe;
            if (!w) return 0;
            const p = clamp((w.t || 0) / WIPE_S, 0, 1);
            return w.dir < 0 ? p : 1 - p;
        }

        function drawWipe(sim, vw, vh, frame) {
            const iv = sim && sim.interior;
            const cover = coverOf(iv);
            if (!(cover > 0.001)) return;
            if (frame && frame.reducedMotion) {
                ctx.fillStyle = 'rgba(8, 6, 14, ' + clamp(cover, 0, 1).toFixed(3) + ')';
                ctx.fillRect(0, 0, vw, vh);
                return;
            }
            const s = clamp(vh / (11 * CELL), 1, 2.5);
            const fx = clamp((iv.hero.x - cam.x) * s, 0, vw);
            const fy = clamp((iv.hero.y - 30 - cam.y) * s + insetCache, 0, vh);
            const maxR = Math.hypot(Math.max(fx, vw - fx), Math.max(fy, vh - fy)) + 8;
            const r = (1 - clamp(cover, 0, 1)) * maxR;
            ctx.save();
            ctx.fillStyle = '#07050d';
            ctx.beginPath();
            ctx.rect(0, 0, vw, vh);
            if (r > 0.5) ctx.arc(fx, fy, r, 0, Math.PI * 2, true);
            ctx.fill('evenodd');
            if (r > 2 && r < maxR - 4) {
                ctx.strokeStyle = RIM;
                ctx.globalAlpha = 0.85;
                ctx.lineWidth = 3;
                ctx.beginPath();
                ctx.arc(fx, fy, r, 0, Math.PI * 2);
                ctx.stroke();
            }
            ctx.restore();
        }

        function dispose() {
            Object.keys(requested).forEach(function (k) { delete requested[k]; });
            Object.keys(glows).forEach(function (k) { delete glows[k]; });
            baked = null;
            bakedRoom = null;
            lightCv = null;
            lightCtx = null;
            cam.ready = false;
            if (TV) TV.dispose();
        }

        return { wants: wants, prime: prime, render: render, drawWipe: drawWipe, dispose: dispose };
    }

    root.RunenpfadInteriorView = { create: create };
})(typeof window !== 'undefined' ? window : globalThis);
