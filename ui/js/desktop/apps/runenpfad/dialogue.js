/* Runenpfad dialogue interpreter: graph walking, conditions, effects, tips and the graph validator. DOM-free. */
(function (root) {
    'use strict';

    const SHARED_CHOICES = ['farewell', 'thanks', 'trade', 'leave', 'fight', 'back'];
    const EFFECT_OPS = ['rep', 'coins', 'item', 'give', 'flag', 'unflag', 'quest', 'questDone', 'trade', 'duel', 'provoke', 'gate', 'leave', 'confront', 'ui', 'page', 'notice'];
    const COND_KEYS = ['repMin', 'repMax', 'coinsMin', 'flag', 'notFlag', 'flagMin', 'item', 'hero', 'quest', 'questDone', 'notQuest', 'world', 'feature', 'any', 'not', 'progress', 'loot', 'pages'];
    const TIP_KINDS = ['key_dir', 'gems_left', 'secret_wall', 'chest_dir', 'puzzle_hint', 'boss_weakness'];
    const MOODS = ['neutral', 'happy', 'angry'];
    const MARKERS = ['talk', 'quest', 'duel', 'trade', 'heart'];
    const WORLD_BOSSES = ['boss_boar', 'boss_goblin', 'boss_orb', 'boss_quad', 'boss_mech'];
    const TIP_PREFIX = 'runenpfad.npc.tip.';
    const UI_RE = /^(board|dice:[a-z_]+|song:[a-z_]+|scene:[a-z_]+)$/;
    const NOTICE_RE = /^runenpfad\.notice\.[a-z_]+$/;

    function P() { return root.RunenpfadPhysics; }
    function N() { return root.RunenpfadNpcs; }
    function story() { return root.RunenpfadNpcStory || { MEETINGS: [], GRAPHS: {} }; }
    function prof() { return root.RunenpfadProfile; }

    function graphOf(npcId) { return story().GRAPHS[npcId] || null; }
    function nodeOf(npcId, local) {
        const g = graphOf(npcId);
        return g && g.nodes && Object.prototype.hasOwnProperty.call(g.nodes, local) ? g.nodes[local] : null;
    }
    function nodeKey(npcId, local) { return 'runenpfad.npc.' + npcId + '.' + local; }
    function asList(v) { return Array.isArray(v) ? v : [v]; }
    function findNpc(sim, ref) {
        const enc = sim.encounter;
        if (enc && enc.virtual && enc.virtual.ref === ref) return enc.virtual;
        const inside = ((sim.interior && sim.interior.npcs) || []).filter(function (n) { return n.ref === ref; })[0];
        return inside || ((sim.stage && sim.stage.npcs) || []).filter(function (n) { return n.ref === ref; })[0] || null;
    }

    // ---- conditions ----

    function evalCond(cond, sim) {
        if (!cond) return true;
        const n = N();
        const pr = sim.profile;
        const k = Object.keys(cond);
        for (let i = 0; i < k.length; i++) {
            const key = k[i];
            const v = cond[key];
            let ok = true;
            if (key === 'repMin') ok = Object.keys(v).every(function (f) { return n.getRep(sim, f) >= v[f]; });
            else if (key === 'repMax') ok = Object.keys(v).every(function (f) { return n.getRep(sim, f) <= v[f]; });
            else if (key === 'coinsMin') ok = pr.coins >= v;
            else if (key === 'flag') ok = asList(v).every(function (f) { return n.hasFlag(sim, f); });
            else if (key === 'notFlag') ok = asList(v).every(function (f) { return !n.hasFlag(sim, f); });
            else if (key === 'flagMin') ok = Object.keys(v).every(function (f) { return n.getFlag(sim, f) >= v[f]; });
            else if (key === 'item') ok = Object.keys(v).every(function (id) { return n.itemCount(sim, id) >= v[id]; });
            else if (key === 'hero') ok = sim.heroId === v;
            else if (key === 'quest') ok = n.questActive(sim, v);
            else if (key === 'questDone') ok = n.questDone(sim, v);
            else if (key === 'notQuest') ok = !n.questActive(sim, v) && !n.questDone(sim, v);
            else if (key === 'world') ok = sim.stage.worldIndex === v;
            else if (key === 'feature') ok = v === 'duel' ? !!root.RunenpfadDuel : false;
            else if (key === 'any') ok = v.some(function (c) { return evalCond(c, sim); });
            else if (key === 'not') ok = !evalCond(v, sim);
            else if (key === 'progress') ok = pr.unlockedLevel >= v;
            else if (key === 'loot') ok = (pr.loot || []).indexOf(v) >= 0;
            else if (key === 'pages') ok = pageCount(pr) >= v;
            if (!ok) return false;
        }
        return true;
    }

    function pageCount(pr) {
        const C = root.RunenpfadChronicle;
        if (C) return C.count(pr);
        return (pr.loot || []).filter(function (id) { return /^page_(0[1-9]|10)$/.test(id); }).length;
    }

    // ---- effects ----

    function give(sim, what) {
        const p = sim.player;
        if (what === 'potion') {
            p.potions += 1;
            sim.floatingTexts.push({ x: p.x, y: p.y - 44, text: '+1', icon: 'potion', color: '#c084fc', timer: 1 });
        } else if (what === 'meal') {
            p.hp = p.maxHp;
            sim.floatingTexts.push({ x: p.x, y: p.y - 44, text: 'FULL', icon: 'heart', color: '#f87171', timer: 1 });
        } else if (what === 'heart') {
            p.hp = Math.min(p.maxHp, p.hp + 1);
            sim.floatingTexts.push({ x: p.x, y: p.y - 44, text: '+1', icon: 'heart', color: '#f87171', timer: 1 });
        } else if (/^rune_/.test(what) && root.RunenpfadRunes) {
            root.RunenpfadRunes.grant(sim, what.slice(5), p.x, p.y - 30);
        }
    }

    function openGate(sim, npc) {
        const Z = root.RunenpfadZonesExtra;
        if (!Z) return;
        (sim.stage.gates || []).forEach(function (g) {
            if (!g.open && g.npcGate === npc.npcId) Z.openGate(sim, g);
        });
    }

    // Returns {trade, duel, leave}: what the caller must do after the effects ran.
    function applyEffects(sim, npc, effects) {
        const res = { trade: false, duel: false, leave: false };
        const n = N();
        (effects || []).forEach(function (ef) {
            Object.keys(ef).forEach(function (op) {
                const v = ef[op];
                if (op === 'rep') Object.keys(v).forEach(function (f) { n.addRep(sim, f, v[f]); });
                else if (op === 'coins') n.addCoins(sim, v);
                else if (op === 'item') Object.keys(v).forEach(function (id) { n.addItem(sim, id, v[id]); });
                else if (op === 'give') give(sim, v);
                else if (op === 'flag') {
                    if (typeof v === 'string') n.setFlag(sim, v, 1);
                    else Object.keys(v).forEach(function (key) { n.setFlag(sim, key, v[key] === '+1' ? n.getFlag(sim, key) + 1 : Number(v[key])); });
                } else if (op === 'unflag') asList(v).forEach(function (key) { n.setFlag(sim, key, 0); });
                else if (op === 'quest') n.startQuest(sim, v);
                else if (op === 'questDone') n.completeQuest(sim, v);
                else if (op === 'trade') res.trade = true;
                else if (op === 'duel') res.duel = n.startDuel(sim, npc, { provoked: false }) || res.duel;
                else if (op === 'provoke') {
                    n.provoke(sim, npc);
                    res.duel = !!(sim.encounter && sim.encounter.kind === 'duel') || res.duel;
                } else if (op === 'ui') {
                    sim.uiRequest = v;
                    close(sim);
                } else if (op === 'page') {
                    if (root.RunenpfadChronicle) root.RunenpfadChronicle.give(sim, v);
                } else if (op === 'notice') sim.notice = v;
                else if (op === 'gate') openGate(sim, npc);
                else if (op === 'leave') {
                    npc.state = 'gone';
                    npc.near = false;
                    res.leave = true;
                } else if (op === 'confront' && root.RunenpfadTrade) root.RunenpfadTrade.confront(sim, npc);
            });
        });
        return res;
    }

    function applyHook(sim, npc, hookName) {
        const g = graphOf(npc.npcId);
        const list = g && g.hooks && g.hooks[hookName];
        if (Array.isArray(list)) applyEffects(sim, npc, list);
    }

    // Outcome node (local id) for onDuelWon/onDuelLost: the first entry whose condition holds.
    function hookTarget(sim, npc, hookName) {
        const g = graphOf(npc.npcId);
        const list = g && g.hooks && g.hooks[hookName];
        if (!Array.isArray(list) || hookName === 'onAttack') return null;
        for (let i = 0; i < list.length; i++) {
            if (evalCond(list[i].if, sim)) return list[i].to;
        }
        return null;
    }

    // ---- meetings ----

    function startNode(sim, npc) {
        const s = story();
        const m = s.MEETINGS.filter(function (x) { return x.id === npc.meetingId; })[0];
        let local = null;
        if (m) {
            for (let i = 0; i < m.starts.length; i++) {
                if (evalCond(m.starts[i].if, sim)) {
                    local = m.starts[i].to;
                    break;
                }
            }
        } else if (nodeOf(npc.npcId, 'hello')) {
            local = 'hello';
        }
        const node = local ? nodeOf(npc.npcId, local) : null;
        return node ? { local: local, node: node } : null;
    }

    // ---- tips (generated from live stage data) ----

    function tileDx(stage, dx) { return Math.max(1, Math.round(Math.abs(dx) / stage.tileSize)); }

    function nearest(list, px) {
        let best = null;
        list.forEach(function (it) { if (!best || Math.abs(it.x - px) < Math.abs(best.x - px)) best = it; });
        return best;
    }

    function dirTip(stage, base, target, px) {
        const dx = target.x - px;
        return { key: TIP_PREFIX + base + (dx < 0 ? '_left' : '_right'), params: { count: tileDx(stage, dx) } };
    }

    function tip(sim, kind) {
        const stage = sim.stage;
        const px = sim.player.x;
        if (kind === 'key_dir') {
            const k = stage.key;
            if (!k || k.collected) return { key: TIP_PREFIX + 'key_here', params: {} };
            if (k.guardedByBoss) return { key: TIP_PREFIX + 'key_boss', params: {} };
            if (Math.abs(k.x - px) < stage.tileSize * 2) return { key: TIP_PREFIX + 'key_here', params: {} };
            return dirTip(stage, 'key', k, px);
        }
        if (kind === 'gems_left') {
            const gems = stage.collectibles.filter(function (c) { return c.kind === 'gem' && !c.collected; });
            if (!gems.length) return { key: TIP_PREFIX + 'gems_none', params: {} };
            const t = dirTip(stage, 'gems', nearest(gems, px), px);
            t.params.count = gems.length;
            return t;
        }
        if (kind === 'secret_wall') {
            const w = nearest((stage.secretWalls || []).filter(function (x) { return !x.broken; }), px);
            return w ? dirTip(stage, 'secret', w, px) : { key: TIP_PREFIX + 'secret_none', params: {} };
        }
        if (kind === 'chest_dir') {
            const c = nearest((stage.chests || []).filter(function (x) { return !x.opened; }), px);
            return c ? dirTip(stage, 'chest', c, px) : { key: TIP_PREFIX + 'chest_none', params: {} };
        }
        if (kind === 'puzzle_hint') {
            if ((stage.oracles || []).length) return { key: TIP_PREFIX + 'puzzle_rune_sequence', params: {} };
            if ((stage.levers || []).length) return { key: TIP_PREFIX + 'puzzle_lever_bridge', params: {} };
            if ((stage.rituals || []).length) return { key: TIP_PREFIX + 'puzzle_ritual_chain', params: {} };
            return tip(sim, 'key_dir');
        }
        if (kind === 'boss_weakness') {
            const boss = stage.bossId ? stage.enemies.filter(function (e) { return e.id === stage.bossId; })[0] : null;
            if (boss && boss.defId === 'boss_dragon_frost') return { key: TIP_PREFIX + 'boss_dragon_frost', params: {} };
            return { key: TIP_PREFIX + 'boss_' + WORLD_BOSSES[Math.min(4, Math.max(0, stage.worldIndex))], params: {} };
        }
        return { key: TIP_PREFIX + 'key_here', params: {} };
    }

    function tipKeys() {
        const out = [];
        ['key_left', 'key_right', 'key_here', 'key_boss', 'gems_none', 'gems_left', 'gems_right', 'secret_left', 'secret_right', 'secret_none', 'chest_left', 'chest_right', 'chest_none', 'puzzle_rune_sequence', 'puzzle_lever_bridge', 'puzzle_ritual_chain', 'boss_dragon_frost'].forEach(function (k) { out.push(TIP_PREFIX + k); });
        WORLD_BOSSES.forEach(function (b) { out.push(TIP_PREFIX + 'boss_' + b); });
        return out;
    }

    // ---- session ----

    function params(sim, npc, node) {
        const out = {};
        const vars = node.vars || {};
        Object.keys(vars).forEach(function (name) {
            const parts = String(vars[name]).split(':');
            if (parts[0] === 'coins') out[name] = sim.profile.coins;
            else if (parts[0] === 'need') out[name] = Math.max(0, Number(parts[2]) - N().itemCount(sim, parts[1]));
            else if (parts[0] === 'price') out[name] = root.RunenpfadTrade ? root.RunenpfadTrade.priceOf(sim, npc, parts[1]) : 0;
            else if (parts[0] === 'rep') out[name] = N().getRep(sim, parts[1]);
        });
        return out;
    }

    function close(sim) {
        const enc = sim.encounter;
        if (!enc || enc.kind !== 'dialogue') return false;
        const npc = findNpc(sim, enc.npcRef);
        sim.encounter = null;
        if (npc) {
            if (npc.state !== 'gone') npc.state = 'idle';
            if (root.RunenpfadTrade) root.RunenpfadTrade.onClose(sim, npc);
            N().refresh(sim, npc);
        }
        return true;
    }

    function gotoNode(sim, enc, npc, local) {
        const node = nodeOf(enc.npcId, local);
        if (!node) {
            close(sim);
            return;
        }
        enc.nodeId = enc.npcId + '.' + local;
        enc.returnNode = enc.nodeId;
        enc.panel = 'talk';
        enc.seq += 1;
        npc.mood = node.mood || 'neutral';
        npc.state = 'talk';
        if (node.enter) applyEffects(sim, npc, node.enter);
    }

    function localOf(enc) { return enc.nodeId.slice(enc.npcId.length + 1); }

    function open(sim, npc, nodeId) {
        let local = nodeId;
        if (!local) {
            const st = startNode(sim, npc);
            local = st ? st.local : null;
        }
        if (!local || !nodeOf(npc.npcId, local)) return false;
        sim.encounter = { kind: 'dialogue', npcRef: npc.ref, npcId: npc.npcId, nodeId: npc.npcId + '.' + local, panel: 'talk', returnNode: npc.npcId + '.' + local, seq: 0, openedTick: sim.stepCount };
        pushOpen(sim, npc);
        gotoNode(sim, sim.encounter, npc, local);
        return true;
    }

    // Opens a dialogue with a speaker that has no body in the room (journal, lore, core). The speaker lives on the encounter.
    function openVirtual(sim, npcId, nodeId) {
        if (!sim || sim.encounter || !npcId || !nodeId || !nodeOf(npcId, nodeId)) return false;
        const npc = { ref: 'virtual_' + npcId, npcId: npcId, x: 0, y: 0, meetingId: null, facing: 1, state: 'idle', mood: 'neutral', near: false, marker: null, provoked: false, sold: {}, hurtT: 0, virtual: true };
        sim.encounter = { kind: 'dialogue', npcRef: npc.ref, npcId: npcId, nodeId: npcId + '.' + nodeId, panel: 'talk', returnNode: npcId + '.' + nodeId, seq: 0, openedTick: sim.stepCount, virtual: npc };
        pushOpen(sim, npc);
        gotoNode(sim, sim.encounter, npc, nodeId);
        return true;
    }

    function pushOpen(sim, npc) {
        // Inside an interior the NPC position is in room pixels: no x/y, so the audio does not pan by the world camera.
        if (sim.interior || npc.virtual) P().pushEvent(sim, 'npc_talk', { npcId: npc.npcId });
        else P().pushEvent(sim, 'npc_talk', { npcId: npc.npcId, x: npc.x, y: npc.y });
    }

    function visibleChoices(sim, node) {
        const out = [];
        node.choices.forEach(function (c, i) {
            if (evalCond(c.if, sim)) out.push({ choice: c, n: i + 1 });
        });
        return out;
    }

    function view(sim) {
        const enc = sim.encounter;
        if (!enc || enc.kind !== 'dialogue') return null;
        const npc = findNpc(sim, enc.npcRef);
        const local = localOf(enc);
        const node = nodeOf(enc.npcId, local);
        if (!npc || !node) return null;
        const ps = params(sim, npc, node);
        const v = {
            npcId: enc.npcId,
            mood: node.mood || 'neutral',
            textKey: nodeKey(enc.npcId, local),
            params: ps,
            tip: node.tip ? tip(sim, node.tip) : null,
            choices: visibleChoices(sim, node).map(function (e, i) {
                return { index: i, textKey: e.choice.text || (nodeKey(enc.npcId, local) + '.c' + e.n), params: ps };
            }),
            panel: enc.panel,
            seq: enc.seq
        };
        if (enc.panel === 'trade' && root.RunenpfadTrade) v.offers = root.RunenpfadTrade.offersFor(sim, npc);
        return v;
    }

    // index: 0-based position in view().choices (hidden choices are not counted).
    function choose(sim, index) {
        const enc = sim.encounter;
        const out = { ok: false, closed: false, action: null };
        if (!enc || enc.kind !== 'dialogue' || enc.panel !== 'talk') return out;
        const npc = findNpc(sim, enc.npcRef);
        const node = nodeOf(enc.npcId, localOf(enc));
        const entry = npc && node ? visibleChoices(sim, node)[index] : null;
        if (!entry) return out;
        out.ok = true;
        const c = entry.choice;
        P().pushEvent(sim, 'npc_choice', { npcId: enc.npcId, node: enc.nodeId });
        const res = applyEffects(sim, npc, c.effects);
        if (sim.encounter !== enc) {
            out.closed = true;
            out.action = res.duel ? 'duel' : null;
            return out;
        }
        if (res.trade) {
            out.action = 'trade';
            openTrade(sim);
            return out;
        }
        let to = c.to;
        if (c.roll) to = P().rand(sim) < c.roll.p ? c.to : c.roll.else;
        if (to === null) {
            close(sim);
            out.closed = true;
        } else if (typeof to === 'string') {
            gotoNode(sim, enc, npc, to);
            if (sim.encounter !== enc) out.closed = true;
        } else {
            enc.seq += 1;
        }
        return out;
    }

    function openTrade(sim) {
        const enc = sim.encounter;
        if (!enc || enc.kind !== 'dialogue') return false;
        enc.panel = 'trade';
        enc.returnNode = enc.nodeId;
        enc.seq += 1;
        return true;
    }

    function closeTrade(sim) {
        const enc = sim.encounter;
        if (!enc || enc.kind !== 'dialogue' || enc.panel !== 'trade') return false;
        enc.panel = 'talk';
        enc.nodeId = enc.returnNode;
        enc.seq += 1;
        return true;
    }

    // ---- validation and key lists ----

    function checkEffects(list, where, guard, probs, strictNeg) {
        (list || []).forEach(function (ef) {
            Object.keys(ef).forEach(function (op) {
                const v = ef[op];
                if (EFFECT_OPS.indexOf(op) < 0) { probs.push(where + ': unknown op ' + op); return; }
                if (op === 'rep') Object.keys(v).forEach(function (f) { if (prof().FACTIONS.indexOf(f) < 0 || !Number.isInteger(v[f]) || Math.abs(v[f]) > 2) probs.push(where + ': bad rep ' + f); });
                if (op === 'coins') {
                    if (!Number.isFinite(v)) probs.push(where + ': bad coins');
                    else if (v < 0 && !(guard && guard.coinsMin >= -v)) probs.push(where + ': unguarded coins ' + v);
                }
                if (op === 'item') Object.keys(v).forEach(function (id) {
                    if (!prof().STORY_ITEMS[id]) probs.push(where + ': bad item ' + id);
                    else if (v[id] < 0 && !(guard && guard.item && guard.item[id] >= -v[id])) probs.push(where + ': unguarded item ' + id);
                });
                if (op === 'give' && !(v === 'potion' || v === 'heart' || v === 'meal' || /^rune_(fire|frost|storm|magnet)$/.test(v))) probs.push(where + ': bad give ' + v);
                if (op === 'flag') {
                    if (typeof v === 'string') { if (!prof().FLAG_RE.test(v)) probs.push(where + ': bad flag ' + v); }
                    else Object.keys(v).forEach(function (k) { if (!prof().FLAG_RE.test(k)) probs.push(where + ': bad flag ' + k); });
                }
                if (op === 'unflag') asList(v).forEach(function (k) { if (!prof().FLAG_RE.test(k)) probs.push(where + ': bad flag ' + k); });
                if ((op === 'quest' || op === 'questDone') && prof().QUEST_IDS.indexOf(v) < 0) probs.push(where + ': bad quest ' + v);
                if (op === 'ui' && v !== 'workshop' && v !== 'chronicle' && !(typeof v === 'string' && UI_RE.test(v))) probs.push(where + ': bad ui ' + v);
                if (op === 'page' && !(Number.isInteger(v) && v >= 1 && v <= 10)) probs.push(where + ': bad page ' + v);
                if (op === 'notice' && !(typeof v === 'string' && NOTICE_RE.test(v))) probs.push(where + ': bad notice ' + v);
                if (op === 'gate' && v !== 'open') probs.push(where + ': bad gate');
            });
        });
    }

    function checkCond(c, where, probs) {
        if (!c) return;
        Object.keys(c).forEach(function (k) {
            const v = c[k];
            if (COND_KEYS.indexOf(k) < 0) { probs.push(where + ': unknown cond ' + k); return; }
            if (k === 'repMin' || k === 'repMax') Object.keys(v).forEach(function (f) { if (prof().FACTIONS.indexOf(f) < 0) probs.push(where + ': bad faction ' + f); });
            if (k === 'flag' || k === 'notFlag') asList(v).forEach(function (f) { if (!prof().FLAG_RE.test(f)) probs.push(where + ': bad flag ' + f); });
            if (k === 'flagMin') Object.keys(v).forEach(function (f) { if (!prof().FLAG_RE.test(f)) probs.push(where + ': bad flag ' + f); });
            if (k === 'item') Object.keys(v).forEach(function (id) { if (!prof().STORY_ITEMS[id]) probs.push(where + ': bad item ' + id); });
            if ((k === 'quest' || k === 'questDone' || k === 'notQuest') && prof().QUEST_IDS.indexOf(v) < 0) probs.push(where + ': bad quest ' + v);
            if (k === 'progress' && !(Number.isInteger(v) && v >= 1 && v <= 25)) probs.push(where + ': bad progress ' + v);
            if (k === 'loot' && !prof().LOOT_RE.test(v)) probs.push(where + ': bad loot ' + v);
            if (k === 'pages' && !(Number.isInteger(v) && v >= 1 && v <= 10)) probs.push(where + ': bad pages ' + v);
            if (k === 'any') v.forEach(function (x) { checkCond(x, where, probs); });
            if (k === 'not') checkCond(v, where, probs);
        });
    }

    function closesDialogue(c) {
        return c.to === null || (c.effects || []).some(function (e) { return e.duel || e.provoke || e.ui; });
    }

    function validateGraphs() {
        const probs = [];
        const s = story();
        const reach = {};
        const queue = [];
        const mark = function (npcId, local, where) {
            if (!nodeOf(npcId, local)) { probs.push(where + ': missing node ' + npcId + '.' + local); return; }
            if (!reach[npcId + '.' + local]) {
                reach[npcId + '.' + local] = true;
                queue.push([npcId, local]);
            }
        };
        s.MEETINGS.forEach(function (m) {
            const where = 'meeting ' + m.id;
            if (!graphOf(m.npcId)) probs.push(where + ': no graph for ' + m.npcId);
            if (!m.starts || !m.starts.length) { probs.push(where + ': no starts'); return; }
            if (m.starts[m.starts.length - 1].if) probs.push(where + ': last start has a condition');
            m.starts.forEach(function (st) {
                checkCond(st.if, where, probs);
                mark(m.npcId, st.to, where);
            });
        });
        Object.keys(s.GRAPHS).forEach(function (npcId) {
            const g = s.GRAPHS[npcId];
            const hooks = g.hooks || {};
            if (g.virtual === true && g.nodes) Object.keys(g.nodes).forEach(function (id) { mark(npcId, id, npcId); });
            checkEffects(hooks.onAttack, npcId + '.onAttack', null, probs);
            ['onDuelWon', 'onDuelLost'].forEach(function (h) {
                (hooks[h] || []).forEach(function (e) {
                    checkCond(e.if, npcId + '.' + h, probs);
                    mark(npcId, e.to, npcId + '.' + h);
                });
            });
        });
        while (queue.length) {
            const cur = queue.shift();
            const node = nodeOf(cur[0], cur[1]);
            node.choices.forEach(function (c) {
                if (typeof c.to === 'string') mark(cur[0], c.to, cur[0] + '.' + cur[1]);
                if (c.roll) mark(cur[0], c.roll.else, cur[0] + '.' + cur[1]);
            });
        }
        Object.keys(s.GRAPHS).forEach(function (npcId) {
            const nodes = s.GRAPHS[npcId].nodes;
            const canClose = {};
            let changed = true;
            while (changed) {
                changed = false;
                Object.keys(nodes).forEach(function (id) {
                    if (canClose[id]) return;
                    const ok = nodes[id].choices.some(function (c) {
                        return closesDialogue(c) || (typeof c.to === 'string' && canClose[c.to]) || (c.roll && canClose[c.roll.else]);
                    });
                    if (ok) {
                        canClose[id] = true;
                        changed = true;
                    }
                });
            }
            Object.keys(nodes).forEach(function (id) {
                const n = nodes[id];
                const where = npcId + '.' + id;
                if (!reach[where]) probs.push(where + ': unreachable');
                if (!Array.isArray(n.choices) || n.choices.length < 1 || n.choices.length > 4) { probs.push(where + ': needs 1..4 choices'); return; }
                if (!n.choices.some(function (c) { return !c.if; })) probs.push(where + ': no unconditional choice');
                if (!canClose[id]) probs.push(where + ': dead end');
                if (n.mood && MOODS.indexOf(n.mood) < 0) probs.push(where + ': bad mood');
                if (n.marker && MARKERS.indexOf(n.marker) < 0) probs.push(where + ': bad marker');
                if (n.tip && TIP_KINDS.indexOf(n.tip) < 0) probs.push(where + ': bad tip ' + n.tip);
                checkEffects(n.enter, where + '.enter', null, probs);
                n.choices.forEach(function (c, i) {
                    const cw = where + '.c' + (i + 1);
                    checkCond(c.if, cw, probs);
                    checkEffects(c.effects, cw, c.if, probs);
                    if (typeof c.to === 'string' && !nodes[c.to]) probs.push(cw + ': missing to ' + c.to);
                    if (c.roll && (!(c.roll.p > 0 && c.roll.p < 1) || !nodes[c.roll.else])) probs.push(cw + ': bad roll');
                });
            });
        });
        return probs;
    }

    function textKeys() {
        const out = SHARED_CHOICES.map(function (k) { return 'runenpfad.npc.c.' + k; });
        const g = story().GRAPHS;
        Object.keys(g).forEach(function (npcId) {
            Object.keys(g[npcId].nodes).forEach(function (id) {
                const key = nodeKey(npcId, id);
                out.push(key);
                g[npcId].nodes[id].choices.forEach(function (c, i) {
                    out.push(c.text || (key + '.c' + (i + 1)));
                });
            });
        });
        return out.filter(function (k, i) { return out.indexOf(k) === i; });
    }

    root.RunenpfadDialogue = {
        open: open,
        openVirtual: openVirtual,
        view: view,
        choose: choose,
        close: close,
        openTrade: openTrade,
        closeTrade: closeTrade,
        evalCond: evalCond,
        applyEffects: applyEffects,
        applyHook: applyHook,
        hookTarget: hookTarget,
        startNode: startNode,
        tip: tip,
        tipKeys: tipKeys,
        validateGraphs: validateGraphs,
        textKeys: textKeys
    };
})(typeof window !== 'undefined' ? window : globalThis);
