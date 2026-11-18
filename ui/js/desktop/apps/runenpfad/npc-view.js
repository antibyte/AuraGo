/* Runenpfad NPC encounters, canvas side: stage NPC sprites, camp fires, speech markers, name labels and quest items. Read-only on the sim. */
(function (root) {
    'use strict';

    const BASE_H = 81; // hero height 74 x 1.1
    const SCALE = { chief: 1.18, merchant: 0.97 };
    const SHADOW_W = { chief: 54, merchant: 50 };
    const QUEST_ASSET = { glow_mushroom: 'blue_mushrooms', sun_shard: 'amber_crystals', gear_part: 'green_crystals' };
    const QUEST_GLOW = { glow_mushroom: 'rgba(126, 232, 250, 0.7)', sun_shard: 'rgba(251, 191, 36, 0.7)', gear_part: 'rgba(134, 239, 172, 0.7)' };
    const ICON_FOR = { talk: 'talk', quest: 'quest', trade: 'trade', heart: 'heart', duel: 'duel' };

    function Art() { return root.RunenpfadArt || null; }

    function create(h) {
        const ctx = h.ctx;
        const fade = {}; // npc.ref -> label/bubble alpha (visual only)

        function list(sim) {
            return (sim.stage && sim.stage.npcs) || [];
        }

        function seedOf(npc) {
            const s = String(npc.ref || npc.npcId || '');
            let n = 0;
            for (let i = 0; i < s.length; i++) n = (n * 31 + s.charCodeAt(i)) % 97;
            return n * 0.137;
        }

        function encounterOf(sim, npc) {
            const enc = sim.encounter;
            return enc && enc.npcRef === npc.ref ? enc : null;
        }

        function hostile(npc) {
            return !!(npc.provoked || npc.state === 'alert' || npc.state === 'hurt' || npc.marker === 'duel' || npc.mood === 'angry');
        }

        function clipFor(sim, npc) {
            const enc = encounterOf(sim, npc);
            if (npc.state === 'hurt' || npc.hurtT > 0) return 'hurt';
            if (enc && enc.kind === 'duel') return 'alert';
            if (enc && enc.panel === 'trade') return 'cast';
            if (enc) return 'talk';
            if (hostile(npc)) return 'alert';
            return 'idle';
        }

        // Fire sits to the side of a single NPC and between two.
        function fireSpot(sim, npc) {
            const same = list(sim).filter(function (n) { return n.campX0 === npc.campX0; });
            if (same.length > 1) return same[0] === npc ? { x: (same[0].x + same[1].x) / 2, y: npc.y } : null;
            const T = (sim.stage && sim.stage.tileSize) || 48;
            const mid = (npc.campX0 + npc.campX1) / 2;
            return { x: npc.x + (npc.x < mid ? 3.2 : -3.2) * T, y: npc.y };
        }

        function drawFire(spot, view) {
            const art = h.art;
            const t = view.nowSec;
            h.drawGlow(spot.x, spot.y - 14, 70, 'rgba(255, 170, 70, 0.7)', 0.5);
            ctx.save();
            ctx.fillStyle = '#4a3020';
            ctx.translate(spot.x, spot.y - 3);
            ctx.rotate(0.35);
            ctx.fillRect(-13, -3, 26, 6);
            ctx.rotate(-0.7);
            ctx.fillRect(-13, -3, 26, 6);
            ctx.restore();
            const A = Art();
            if (!(art && A && A.draw(ctx, art, 'fx-groundfire.loop', t, spot.x, spot.y - 4, view.reducedMotion ? { height: 34, frame: 3 } : { height: 34, alpha: 0.95 }))) {
                ctx.save();
                ctx.fillStyle = '#ffb347';
                ctx.beginPath();
                ctx.ellipse(spot.x, spot.y - 12, 6, 11 + (view.reducedMotion ? 0 : Math.sin(t * 9) * 2), 0, 0, Math.PI * 2);
                ctx.fill();
                ctx.restore();
            }
        }

        function drawNpcSprite(sim, npc, def, view) {
            const art = h.art;
            const A = Art();
            const clip = clipFor(sim, npc);
            const flip = npc.facing < 0; // art sheets are painted facing right, like the hero packs
            const t = view.nowSec + seedOf(npc);
            const flash = clip === 'hurt' ? Math.min(0.8, Math.max(0.25, (npc.hurtT || 0) * 3)) : 0;
            const mul = SCALE[npc.npcId] || 1;
            const sheetClip = 'npc-' + npc.npcId + '.' + clip;
            if (art && A && A.has(art, sheetClip)) {
                const o = { height: BASE_H * mul, flip: flip, flash: flash };
                if (view.reducedMotion) o.frame = 0;
                A.draw(ctx, art, sheetClip, clip === 'hurt' ? 0 : t, npc.x, npc.y, o);
                return;
            }
            const fb = def && def.fallback;
            if (!fb) return;
            h.drawAnimation(fb.pack, fb.anim, view.reducedMotion ? 0 : t, npc.x - 32, npc.y - 59, 64, 64, flip, 1);
        }

        function drawIcon(id, x, y, height, alpha, view) {
            const art = h.art;
            const A = Art();
            const key = 'ui-npc-icons.' + id;
            if (!(art && A && A.has(art, key))) return false;
            A.draw(ctx, art, key, 0, x, y, { height: height, alpha: alpha });
            return true;
        }

        function nameOf(def) {
            const tr = h.t;
            if (!def || !tr) return '';
            const s = tr(def.nameKey);
            return s && s !== def.nameKey ? s : '';
        }

        function drawWorld(sim, view) {
            const npcs = list(sim);
            const Npcs = root.RunenpfadNpcs;
            const seen = {};
            npcs.forEach(function (npc) {
                if (npc.state === 'gone' || seen[npc.campX0]) return;
                seen[npc.campX0] = true;
                const spot = fireSpot(sim, npc);
                if (spot) drawFire(spot, view);
            });
            npcs.forEach(function (npc) {
                if (npc.state === 'gone') return;
                const def = Npcs && Npcs.NPCS ? Npcs.NPCS[npc.npcId] : null;
                h.drawContactShadow(npc.x, npc.y, SHADOW_W[npc.npcId] || 40);
                drawNpcSprite(sim, npc, def, view);
            });
            (sim.stage.questItems || []).forEach(function (it) {
                if (it.taken) return;
                const bob = view.reducedMotion ? 0 : Math.sin(view.nowSec * 2.6 + it.x * 0.01) * 3;
                h.drawGlow(it.x, it.y + bob, 40, QUEST_GLOW[it.kind] || 'rgba(255,255,255,0.6)', 0.7);
                h.drawStaticAsset('nature-assets', QUEST_ASSET[it.kind] || 'blue_crystals', it.x - 18, it.y - 22 + bob, 36, 36, false, 1);
            });
        }

        // Map charm: glow rings on visible gems, edge arrows toward the ones off screen. Read-only; keeps the bottom 40 px clear.
        function drawCharm(sim, view) {
            if (!sim.mapCharm) return;
            const items = (sim.stage && sim.stage.collectibles) || [];
            const z = view.zoom || 1;
            const t = view.reducedMotion ? 0 : view.nowSec;
            const pulse = view.reducedMotion ? 0.8 : 0.65 + 0.35 * Math.sin(t * 3);
            const x0 = view.sx, y0 = view.sy, x1 = view.sx + view.viewW, y1 = view.sy + view.viewH;
            const m = 26 / z;
            const top = y0 + 64 / z;
            const bottom = y1 - (40 + 26) / z;
            ctx.save();
            items.forEach(function (it) {
                if (it.kind !== 'gem' || it.collected) return;
                const color = '#7ee8fa';
                const on = it.x >= x0 && it.x <= x1 && it.y >= y0 && it.y <= y1;
                ctx.globalAlpha = 0.55 * pulse;
                ctx.strokeStyle = color;
                ctx.fillStyle = color;
                if (on) {
                    h.drawGlow(it.x, it.y, 46, 'rgba(126, 232, 250, 0.6)', 0.55 * pulse);
                    ctx.globalAlpha = 0.55 * pulse;
                    ctx.lineWidth = 3 / z;
                    ctx.beginPath();
                    ctx.arc(it.x, it.y, 24 + 4 * pulse, 0, Math.PI * 2);
                    ctx.stroke();
                    return;
                }
                const cx = (x0 + x1) / 2, cy = (y0 + y1) / 2;
                const dx = it.x - cx, dy = it.y - cy;
                const hw = (x1 - x0) / 2 - m, hh = Math.max(10, (bottom - top) / 2);
                const k = Math.min(hw / Math.max(1e-6, Math.abs(dx)), hh / Math.max(1e-6, Math.abs(dy)));
                const ax = cx + dx * k;
                const ay = Math.min(bottom, Math.max(top, (top + bottom) / 2 + dy * k));
                const ang = Math.atan2(dy, dx);
                const sz = (11 + 2 * pulse) / z;
                ctx.globalAlpha = 0.45 + 0.4 * pulse;
                ctx.save();
                ctx.translate(ax, ay);
                ctx.rotate(ang);
                ctx.lineJoin = 'round';
                ctx.lineWidth = 2 / z;
                ctx.strokeStyle = '#09111a';
                ctx.beginPath();
                ctx.moveTo(sz, 0);
                ctx.lineTo(-sz * 0.7, -sz * 0.75);
                ctx.lineTo(-sz * 0.3, 0);
                ctx.lineTo(-sz * 0.7, sz * 0.75);
                ctx.closePath();
                ctx.fill();
                ctx.stroke();
                ctx.restore();
            });
            ctx.restore();
        }

        function drawOverlay(sim, view) {
            drawCharm(sim, view);
            const Npcs = root.RunenpfadNpcs;
            const dt = view.dt || 0.016;
            list(sim).forEach(function (npc) {
                if (npc.state === 'gone') return;
                const def = Npcs && Npcs.NPCS ? Npcs.NPCS[npc.npcId] : null;
                const enc = encounterOf(sim, npc);
                const hot = hostile(npc);
                const want = !enc && (npc.near || hot) ? 1 : 0;
                const cur = fade[npc.ref] == null ? 0 : fade[npc.ref];
                const next = view.reducedMotion ? want : cur + Math.sign(want - cur) * Math.min(Math.abs(want - cur), dt * 5);
                fade[npc.ref] = next;
                const top = npc.y - BASE_H * (SCALE[npc.npcId] || 1);
                const bob = view.reducedMotion ? 0 : Math.sin(view.nowSec * 3 + seedOf(npc)) * 2.5;
                let icon = null;
                let alpha = next;
                if (!enc) {
                    if (hot) icon = npc.marker === 'duel' ? 'duel' : 'alert';
                    else if (npc.near) icon = ICON_FOR[npc.marker] || 'talk';
                    else if (npc.marker === 'quest') {
                        icon = 'quest';
                        alpha = 0.75;
                    }
                }
                if (icon && alpha > 0.02) drawIcon(icon, npc.x, top - 20 + bob, 28, alpha, view);
                if (next > 0.02 && !hot) {
                    const name = nameOf(def);
                    if (name) {
                        ctx.save();
                        ctx.font = 'bold 12px "Segoe UI", sans-serif';
                        ctx.textAlign = 'center';
                        ctx.lineJoin = 'round';
                        ctx.lineWidth = 3;
                        ctx.strokeStyle = '#09111a';
                        ctx.globalAlpha = next;
                        ctx.strokeText(name, npc.x, npc.y + 15);
                        ctx.fillStyle = (def && def.color) || '#ffe066';
                        ctx.fillText(name, npc.x, npc.y + 15);
                        ctx.restore();
                    }
                }
            });
        }

        function collectLights(sim, view, lights) {
            const seen = {};
            list(sim).forEach(function (npc) {
                if (npc.state === 'gone' || seen[npc.campX0]) return;
                seen[npc.campX0] = true;
                const spot = fireSpot(sim, npc);
                if (spot) lights.push({ x: spot.x, y: spot.y - 16, radius: 150, intensity: 0.8, flicker: true });
            });
            (sim.stage.questItems || []).forEach(function (it) {
                if (!it.taken) lights.push({ x: it.x, y: it.y, radius: 70, intensity: 0.4, flicker: false });
            });
        }

        function dispose() {
            Object.keys(fade).forEach(function (k) { delete fade[k]; });
        }

        return { drawWorld: drawWorld, drawOverlay: drawOverlay, collectLights: collectLights, dispose: dispose };
    }

    root.RunenpfadNpcView = { create: create };
})(typeof window !== 'undefined' ? window : globalThis);
