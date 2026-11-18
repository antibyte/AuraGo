/* Runenpfad NPC encounters core: camps, talk range, reputation, flags, quests and the world-hold hook. DOM-free. */
(function (root) {
    'use strict';

    const ZONE_IDS = ['npc_camp'];
    const TALK_DX = 72;
    const TALK_DY = 48;
    const FACE_RANGE = 200;
    const ATTACK_RANGE = 70;
    const TALK_HINT = 'runenpfad.npc.ui.talk_hint';
    const CAMP_TILES = 12;

    // `offers` ids are resolved by trade.js; `rune_any` is the merchant's one seeded rune per stage.
    const NPCS = {
        witch: { faction: 'coven', nameKey: 'runenpfad.npc.witch.name', titleKey: 'runenpfad.npc.witch.title', color: '#a78bfa', height: 74, fallback: { pack: 'human-characters-animated', anim: 'ranger_idle', tint: '#a78bfa' }, offers: ['potion', 'rune_fire'], duelCoins: 20 },
        wizard: { faction: 'circle', nameKey: 'runenpfad.npc.wizard.name', titleKey: 'runenpfad.npc.wizard.title', color: '#60a5fa', height: 74, fallback: { pack: 'human-characters-animated', anim: 'ranger_idle', tint: '#60a5fa' }, offers: ['rune_storm', 'rune_attune'], duelCoins: 30 },
        merchant: { faction: 'kobolds', nameKey: 'runenpfad.npc.merchant.name', titleKey: 'runenpfad.npc.merchant.title', color: '#facc15', height: 74, fallback: { pack: 'monsters-animals-animated', anim: 'goblin_idle', tint: null }, offers: ['potion', 'rune_any', 'map_charm', 'lucky_amulet', 'discount_token'], duelCoins: 15 },
        chief: { faction: 'kobolds', nameKey: 'runenpfad.npc.chief.name', titleKey: 'runenpfad.npc.chief.title', color: '#f87171', height: 74, fallback: { pack: 'monsters-animals-animated', anim: 'goblin_idle', tint: '#f87171' }, offers: ['lucky_amulet'], duelCoins: 25 },
        seer: { faction: 'circle', nameKey: 'runenpfad.npc.seer.name', titleKey: 'runenpfad.npc.seer.title', color: '#fbbf24', height: 74, fallback: { pack: 'human-characters-animated', anim: 'ranger_idle', tint: '#fbbf24' }, offers: ['rune_magnet', 'map_charm'], duelCoins: 20 },
        alchemist: { faction: 'coven', nameKey: 'runenpfad.npc.alchemist.name', titleKey: 'runenpfad.npc.alchemist.title', color: '#fb923c', height: 74, fallback: { pack: 'human-characters-animated', anim: 'ranger_idle', tint: '#fb923c' }, offers: ['potion', 'rune_frost', 'discount_token'], duelCoins: 30 }
    };

    // Quest -> item kind, number needed, worlds the items spawn in and the floating label.
    const QUESTS = {
        witch_mushrooms: { kind: 'glow_mushroom', need: 3, worlds: [0, 1], float: { key: 'runenpfad.npc.float_mushroom', text: 'MUSHROOM', color: '#7ee8fa' } },
        seer_shards: { kind: 'sun_shard', need: 3, worlds: [2], float: { key: 'runenpfad.npc.float_shard', text: 'SHARD', color: '#fbbf24' } },
        alchemist_gears: { kind: 'gear_part', need: 3, worlds: [4], float: { key: 'runenpfad.npc.float_gear', text: 'GEAR', color: '#fb923c' } }
    };

    function P() { return root.RunenpfadPhysics; }
    function dlg() { return root.RunenpfadDialogue || null; }
    function trade() { return root.RunenpfadTrade || null; }
    function prof() { return root.RunenpfadProfile; }
    function pushEvent(sim, type, detail) { P().pushEvent(sim, type, detail); }

    function has(zone) {
        return ZONE_IDS.indexOf(zone) >= 0;
    }

    // ---- profile story helpers (every mutation sets sim.storyDirty) ----

    function story(sim) {
        const pr = sim.profile;
        if (!pr.rep) pr.rep = { coven: 0, circle: 0, kobolds: 0 };
        if (!pr.npcFlags) pr.npcFlags = {};
        if (!pr.inventory) pr.inventory = {};
        if (!pr.quests) pr.quests = { active: [], done: [] };
        return pr;
    }

    function dirty(sim) { sim.storyDirty = true; }

    function floatText(sim, x, y, text, color, key, suffix, icon) {
        const f = { x: x, y: y, text: text, color: color, timer: 1.1 };
        if (key) f.key = key;
        if (suffix) f.suffix = suffix;
        if (icon) f.icon = icon;
        sim.floatingTexts.push(f);
    }

    function getRep(sim, faction) {
        return story(sim).rep[faction] || 0;
    }

    function addRep(sim, faction, delta) {
        if (prof().FACTIONS.indexOf(faction) < 0) return 0;
        const pr = story(sim);
        const before = pr.rep[faction] || 0;
        const after = Math.max(-5, Math.min(5, before + Math.round(Number(delta) || 0)));
        if (after === before) return 0;
        pr.rep[faction] = after;
        dirty(sim);
        const change = after - before;
        const p = sim.player;
        pushEvent(sim, 'npc_rep', { faction: faction, delta: change, dir: change > 0 ? 'up' : 'down' });
        floatText(sim, p.x, p.y - 64, faction.toUpperCase(), change > 0 ? '#86efac' : '#fca5a5', 'runenpfad.npc.faction.' + faction, (change > 0 ? ' +' : ' ') + change, 'heart');
        return change;
    }

    function getFlag(sim, key) {
        return story(sim).npcFlags[key] || 0;
    }

    function hasFlag(sim, key) {
        return getFlag(sim, key) > 0;
    }

    // value: number (set; 0 clears); invalid keys and a full table (64 keys) are ignored.
    function setFlag(sim, key, value) {
        if (!prof().FLAG_RE.test(String(key))) return false;
        const flags = story(sim).npcFlags;
        const n = Math.max(0, Math.min(99, Math.floor(Number(value == null ? 1 : value) || 0)));
        const had = flags[key] || 0;
        if (n === had) return true;
        if (n === 0) {
            delete flags[key];
        } else {
            if (had === 0 && Object.keys(flags).length >= 64) return false;
            flags[key] = n;
        }
        dirty(sim);
        return true;
    }

    function addCoins(sim, n) {
        const pr = story(sim);
        const next = Math.max(0, Math.floor(pr.coins + (Number(n) || 0)));
        if (next === pr.coins) return 0;
        const diff = next - pr.coins;
        pr.coins = next;
        dirty(sim);
        return diff;
    }

    function itemCount(sim, id) {
        return story(sim).inventory[id] || 0;
    }

    // Returns the real change after the per-item cap.
    function addItem(sim, id, n) {
        const cap = prof().STORY_ITEMS[id];
        if (!cap) return 0;
        const inv = story(sim).inventory;
        const before = inv[id] || 0;
        const after = Math.max(0, Math.min(cap, before + Math.round(Number(n) || 0)));
        if (after === before) return 0;
        if (after === 0) delete inv[id];
        else inv[id] = after;
        dirty(sim);
        return after - before;
    }

    function questActive(sim, id) { return story(sim).quests.active.indexOf(id) >= 0; }
    function questDone(sim, id) { return story(sim).quests.done.indexOf(id) >= 0; }

    function startQuest(sim, id) {
        const q = story(sim).quests;
        if (prof().QUEST_IDS.indexOf(id) < 0 || q.active.indexOf(id) >= 0 || q.done.indexOf(id) >= 0 || q.active.length >= 8) return false;
        q.active.push(id);
        dirty(sim);
        pushEvent(sim, 'npc_quest', { questId: id, state: 'start' });
        floatText(sim, sim.player.x, sim.player.y - 64, 'QUEST', '#fde68a', 'runenpfad.npc.ui.quest_started');
        spawnQuestItems(sim);
        return true;
    }

    function completeQuest(sim, id) {
        const q = story(sim).quests;
        if (prof().QUEST_IDS.indexOf(id) < 0 || q.done.indexOf(id) >= 0) return false;
        const i = q.active.indexOf(id);
        if (i >= 0) q.active.splice(i, 1);
        if (q.done.length < 16) q.done.push(id);
        dirty(sim);
        pushEvent(sim, 'npc_quest', { questId: id, state: 'done' });
        floatText(sim, sim.player.x, sim.player.y - 64, 'DONE', '#86efac', 'runenpfad.npc.ui.quest_done');
        return true;
    }

    // ---- camp builder ----

    function placeColumn(ctx, tx) {
        const tiles = [];
        for (let y = 0; y <= ctx.groundY - 1; y++) {
            ctx.setTile(tx, y, ctx.world.groundFill, 'solid');
            tiles.push({ tx: tx, ty: y, tile: ctx.tiles[ctx.tiles.length - 1] });
        }
        const gate = { id: 'gt_' + ctx.gates.length, x: tx * ctx.TILE, y: 0, w: ctx.TILE, h: ctx.groundY * ctx.TILE, open: false, tiles: tiles, switchIds: [], puzzleId: 'npc_chief', portcullis: true, npcGate: 'chief' };
        ctx.gates.push(gate);
        return gate;
    }

    // A flat camp: no enemies, no hazards, no rng use. Variants: 'witch', 'a+b' (two NPCs), 'chief_gate'.
    function build(zone, ctx) {
        const cx = ctx.cursorX;
        const g = ctx.groundY;
        const T = ctx.TILE;
        const variant = ctx.variant || 'merchant';
        const gated = variant === 'chief_gate';
        const ids = (gated ? 'chief' : variant).split('+').filter(function (id) { return NPCS[id]; });
        if (!ids.length) ids.push('merchant');
        const width = gated ? CAMP_TILES + 1 : CAMP_TILES;
        ctx.fillGround(cx, width, g);
        const props = ctx.world.props;
        ctx.scenery.push({ assetId: props[ctx.zIdx % props.length], x: (cx + 1) * T, y: g * T });
        ctx.scenery.push({ assetId: props[(ctx.zIdx + 1) % props.length], x: (cx + 10.5) * T, y: g * T });
        ctx.scenery.push({ assetId: 'sign', x: (cx + 2.5) * T, y: g * T });
        let gate = null;
        if (gated) gate = placeColumn(ctx, cx + 10);
        const slots = ids.length > 1 ? [4.5, 8] : [gated ? 8 : 6];
        ids.forEach(function (id, i) {
            ctx.extras.npcs.push({
                ref: 'npc_' + ctx.extras.npcs.length, npcId: id, x: (cx + slots[i]) * T, y: g * T,
                campX0: cx * T, campX1: (cx + width) * T, gateId: gate ? gate.id : null,
                facing: -1, state: 'idle', mood: 'neutral', near: false, marker: null, provoked: false, sold: {}, hurtT: 0
            });
        });
        return width;
    }

    // ---- stage resolve (first hold call) ----

    function npcList(sim) {
        return (sim.stage && sim.stage.npcs) || [];
    }

    function meetingFor(sim, npc) {
        const S = root.RunenpfadNpcStory;
        if (!S) return null;
        const endless = sim.mode === 'endless';
        return S.MEETINGS.filter(function (m) {
            if (m.npcId !== npc.npcId) return false;
            return m.levels === 'endless' ? endless : (!endless && Array.isArray(m.levels) && m.levels.indexOf(sim.levelId) >= 0);
        })[0] || null;
    }

    function refresh(sim, npc) {
        const D = dlg();
        const start = D ? D.startNode(sim, npc) : null;
        npc.marker = start && start.node.marker ? start.node.marker : null;
        npc.mood = start && start.node.mood ? start.node.mood : 'neutral';
    }

    function refreshAll(sim) {
        npcList(sim).forEach(function (npc) { refresh(sim, npc); });
    }

    function relocateEnemies(stage, npc) {
        const T = stage.tileSize;
        (stage.enemies || []).forEach(function (e) {
            if (e.def && e.def.boss) return;
            const inside = e.spawnX > npc.campX0 && e.spawnX < npc.campX1;
            if (inside) {
                const nx = npc.campX1 + 2 * T;
                e.x = nx;
                e.spawnX = nx;
                e.minX = Math.max(e.minX, npc.campX1 + T);
                e.maxX = Math.max(e.maxX, e.minX + 4 * T);
            } else if (e.spawnX >= npc.campX1) {
                e.minX = Math.max(e.minX, npc.campX1);
            } else {
                e.maxX = Math.min(e.maxX, npc.campX0);
            }
        });
    }

    function resolve(sim) {
        const stage = sim.stage;
        if (!stage || stage.npcsReady) return;
        stage.npcsReady = true;
        if (!stage.npcs) stage.npcs = [];
        if (!stage.questItems) stage.questItems = [];
        stage.npcs.forEach(function (npc) {
            const m = meetingFor(sim, npc);
            npc.meetingId = m ? m.id : null;
            relocateEnemies(stage, npc);
        });
        spawnQuestItems(sim);
        if (trade()) trade().applyStageStart(sim);
        refreshAll(sim);
    }

    // ---- quest items ----

    function questSpots(stage, need) {
        const T = stage.tileSize;
        const first = (stage.npcs || []).reduce(function (m, n) { return Math.min(m, n.campX1); }, Infinity);
        const from = Number.isFinite(first) ? first : 0;
        const plats = (stage.platforms || []).filter(function (pl) { return pl.kind === 'platform' && !pl.puzzle && pl.x >= from; }).sort(function (a, b) { return a.x - b.x || a.y - b.y; });
        const n = plats.length;
        const idx = [Math.floor(n / 4), Math.floor(n / 2), Math.floor(3 * n / 4)];
        const spots = [];
        for (let i = 0; i < need; i++) {
            const pl = n >= 3 ? plats[idx[i % 3]] : null;
            if (pl) spots.push({ x: pl.x + T / 2, y: pl.y - 24 });
            else spots.push({ x: stage.width * (0.25 + 0.25 * (i % 3)), y: stage.groundY - 24 });
        }
        return spots;
    }

    // Idempotent per quest: items exist once per stage; `taken` ones stay in the list.
    function spawnQuestItems(sim) {
        const stage = sim.stage;
        if (!stage) return;
        if (!stage.questItems) stage.questItems = [];
        story(sim).quests.active.forEach(function (qid) {
            const q = QUESTS[qid];
            if (!q || q.worlds.indexOf(stage.worldIndex) < 0) return;
            if (stage.questItems.some(function (it) { return it.questId === qid; })) return;
            const count = itemCount(sim, q.kind);
            const missing = Math.max(0, q.need - count);
            questSpots(stage, missing).forEach(function (s, i) {
                stage.questItems.push({ id: 'qi_' + stage.questItems.length, kind: q.kind, questId: qid, x: s.x, y: s.y, taken: false });
            });
        });
    }

    // Pickup within 30 px (called from the zone hook every interactable tick).
    function interact(sim) {
        const stage = sim.stage;
        const p = sim.player;
        (stage.questItems || []).forEach(function (it) {
            if (it.taken || Math.hypot(it.x - p.x, it.y - (p.y - 22)) > 30) return;
            it.taken = true;
            const q = QUESTS[it.questId];
            addItem(sim, it.kind, 1);
            dirty(sim);
            const count = itemCount(sim, it.kind);
            pushEvent(sim, 'quest_pickup', { kind: it.kind, x: it.x, y: it.y, count: count });
            floatText(sim, it.x, it.y - 18, q.float.text, q.float.color, q.float.key, ' ' + count + '/' + q.need);
            refreshAll(sim);
        });
    }

    // ---- provoke / duel hooks (the duel module is optional) ----

    function startDuel(sim, npc, opts) {
        const D = root.RunenpfadDuel;
        if (!D || typeof D.create !== 'function' || (sim.encounter && sim.encounter.kind === 'duel')) return false;
        const o = opts || {};
        sim.encounter = {
            kind: 'duel', npcRef: npc.ref, npcId: npc.npcId, nodeId: null, panel: 'talk', returnNode: null, seq: 0, openedTick: sim.stepCount,
            duel: D.create(sim, npc.npcId, { provoked: !!o.provoked, rune: sim.player.rune ? sim.player.rune.kind : null })
        };
        npc.state = 'alert';
        pushEvent(sim, 'duel_start', { npcId: npc.npcId });
        return true;
    }

    function finishDuel(sim) {
        const enc = sim.encounter;
        const npc = npcList(sim).filter(function (n) { return n.ref === enc.npcRef; })[0];
        const won = enc.duel && enc.duel.result === 'won';
        const p = sim.player;
        if (won) {
            addCoins(sim, NPCS[enc.npcId].duelCoins);
            setFlag(sim, enc.npcId + '.beaten', getFlag(sim, enc.npcId + '.beaten') + 1);
        } else {
            p.hp = Math.max(1, p.hp - 1);
            p.invulnTimer = 2;
            sim.damageTaken += 1;
            setFlag(sim, enc.npcId + '.duel_lost', getFlag(sim, enc.npcId + '.duel_lost') + 1);
        }
        pushEvent(sim, 'duel_end', { npcId: enc.npcId, result: won ? 'won' : 'lost' });
        sim.encounter = null;
        if (!npc) return;
        npc.state = 'idle';
        const D = dlg();
        const target = D ? D.hookTarget(sim, npc, won ? 'onDuelWon' : 'onDuelLost') : null;
        if (target) D.open(sim, npc, target);
        else refresh(sim, npc);
    }

    // Once per NPC and stage: reputation -1, graph `onAttack` effects, event, then the duel if available.
    function provoke(sim, npc) {
        if (!npc || npc.provoked || npc.state === 'gone') return false;
        npc.provoked = true;
        addRep(sim, NPCS[npc.npcId].faction, -1);
        if (dlg()) dlg().applyHook(sim, npc, 'onAttack');
        pushEvent(sim, 'npc_provoke', { npcId: npc.npcId, x: npc.x, y: npc.y });
        npc.state = 'hurt';
        npc.hurtT = 0.3;
        startDuel(sim, npc, { provoked: true });
        return true;
    }

    // ---- per-tick world hold ----

    function hint(sim, on) {
        if (on) {
            if (sim.notice === TALK_HINT) return;
            sim.npcPrevNotice = sim.notice;
            sim.notice = TALK_HINT;
        } else if (sim.notice === TALK_HINT) {
            sim.notice = sim.npcPrevNotice || '';
        }
    }

    // Returns true when the world must not advance this tick (dialogue, duel, or a talk just opened).
    function hold(sim, input, dt) {
        const stage = sim.stage;
        if (!stage.npcsReady) resolve(sim);
        const list = stage.npcs || [];
        list.forEach(function (npc) {
            if (npc.hurtT > 0) {
                npc.hurtT = Math.max(0, npc.hurtT - dt);
                if (npc.hurtT === 0 && npc.state === 'hurt') npc.state = 'idle';
            }
        });
        const enc = sim.encounter;
        if (enc) {
            if (enc.kind === 'duel') {
                const D = root.RunenpfadDuel;
                if (D && enc.duel && typeof D.step === 'function') {
                    D.step(sim, enc.duel, dt, input);
                    if (D.done(enc.duel)) finishDuel(sim);
                } else {
                    sim.encounter = null;
                }
            }
            return true;
        }
        if (!list.length) return false;
        const p = sim.player;
        let nearest = null;
        let best = Infinity;
        list.forEach(function (npc) {
            const dx = p.x - npc.x;
            if (npc.state !== 'gone' && Math.abs(dx) < FACE_RANGE) npc.facing = dx < 0 ? 1 : -1;
            const near = npc.state !== 'gone' && Math.abs(dx) <= TALK_DX && Math.abs(p.y - npc.y) <= TALK_DY;
            if (near && !npc.near) pushEvent(sim, 'npc_near', { npcId: npc.npcId, x: npc.x, y: npc.y });
            npc.near = near;
            if (near && Math.abs(dx) < best) {
                best = Math.abs(dx);
                nearest = npc;
            }
        });
        hint(sim, !!nearest);
        if (!nearest) return false;
        const D = dlg();
        const talk = input.talkPressed;
        if (talk && D && !p.swing && !p.onLadder && (talk !== 'up' || (p.onGround && Math.abs(p.vx) < 60))) {
            if (D.open(sim, nearest)) {
                input.jumpPressed = false;
                input.attackPressed = false;
                p.vx = 0;
                if (p.onGround) p.vy = 0;
                p.animState = 'idle';
                hint(sim, false);
                return true;
            }
        }
        if (input.attackPressed) {
            const target = list.filter(function (npc) {
                const dx = npc.x - p.x;
                return npc.state !== 'gone' && dx * p.facing > 0 && Math.abs(dx) <= ATTACK_RANGE && Math.abs(p.y - npc.y) <= TALK_DY;
            })[0];
            if (target && provoke(sim, target) && sim.encounter) {
                input.attackPressed = false;
                return true;
            }
        }
        return false;
    }

    function textKeys() {
        const out = ['runenpfad.npc.ui.talk_hint', 'runenpfad.npc.ui.quest_started', 'runenpfad.npc.ui.quest_done'];
        Object.keys(NPCS).forEach(function (id) {
            out.push(NPCS[id].nameKey, NPCS[id].titleKey);
        });
        prof().FACTIONS.forEach(function (f) { out.push('runenpfad.npc.faction.' + f); });
        Object.keys(QUESTS).forEach(function (q) { out.push(QUESTS[q].float.key); });
        return out;
    }

    root.RunenpfadNpcs = {
        ZONE_IDS: ZONE_IDS,
        NPCS: NPCS,
        QUESTS: QUESTS,
        TALK_HINT: TALK_HINT,
        has: has,
        build: build,
        resolve: resolve,
        hold: hold,
        interact: interact,
        refresh: refresh,
        refreshAll: refreshAll,
        provoke: provoke,
        startDuel: startDuel,
        finishDuel: finishDuel,
        addRep: addRep,
        getRep: getRep,
        setFlag: setFlag,
        getFlag: getFlag,
        hasFlag: hasFlag,
        addCoins: addCoins,
        addItem: addItem,
        itemCount: itemCount,
        questActive: questActive,
        questDone: questDone,
        startQuest: startQuest,
        completeQuest: completeQuest,
        spawnQuestItems: spawnQuestItems,
        textKeys: textKeys
    };
})(typeof window !== 'undefined' ? window : globalThis);
