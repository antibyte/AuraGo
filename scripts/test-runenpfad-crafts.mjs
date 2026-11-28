import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(__dirname, '..');

// ---- sandbox: sim core of round 11 (without the deeps) plus a tiny fake DOM for the games window ----
const sandbox = { console, Math, Number, String, Boolean, Array, Object, Map, Set, JSON, Date, Uint8Array, RegExp, Proxy };
sandbox.window = sandbox;
sandbox.globalThis = sandbox;
let clock = 1000;
sandbox.performance = { now: () => clock };
sandbox.setTimeout = () => 1;
sandbox.clearTimeout = () => {};
const frames = [];
let rafOn = false;
sandbox.requestAnimationFrame = (fn) => { if (rafOn) frames.push(fn); return frames.length; };

// A recording 2D context: every method is a no-op, gradients accept colour stops.
function stubCtx() {
    const calls = [];
    const ctx = new Proxy({}, {
        get(t, k) {
            if (k === 'calls') return calls;
            if (k === 'createRadialGradient' || k === 'createLinearGradient') return () => ({ addColorStop() {} });
            if (k in t) return t[k];
            return (...a) => { calls.push(String(k)); };
        },
        set(t, k, v) { t[k] = v; return true; }
    });
    return ctx;
}

class Node {
    constructor(tag) {
        this.tag = tag; this.children = []; this.className = ''; this.dataset = {}; this.hidden = false; this.disabled = false;
        this.style = {}; this.attrs = {}; this.listeners = {}; this._text = '';
        this.classList = {
            toggle: (c, on) => { const has = this.className.split(' ').includes(c); if (on && !has) this.className += ' ' + c; if (!on && has) this.className = this.className.split(' ').filter((x) => x !== c).join(' '); },
            add: (c) => this.classList.toggle(c, true), remove: (c) => this.classList.toggle(c, false)
        };
    }
    set textContent(v) { this._text = String(v); this.children = []; }
    get textContent() { return this._text + this.children.map((c) => c.textContent).join(''); }
    append(...n) { n.forEach((c) => this.children.push(c)); }
    replaceChildren(...n) { this.children = n; this._text = ''; }
    setAttribute(k, v) { this.attrs[k] = v; }
    addEventListener(type, fn) { (this.listeners[type] = this.listeners[type] || []).push(fn); }
    click() { if (!this.disabled) (this.listeners.click || []).forEach((fn) => fn({})); }
    get offsetWidth() { return 1; }
    getContext() { return this.tag === 'canvas' ? (this.ctx || (this.ctx = stubCtx())) : null; }
    all(out) { out = out || []; this.children.forEach((c) => { out.push(c); c.all(out); }); return out; }
    querySelectorAll() { return this.all().filter((n) => n.tag === 'button' && !n.disabled); }
}
sandbox.document = { createElement: (tag) => new Node(tag) };
vm.createContext(sandbox);

const MODULES = ['biomes', 'fx', 'physics', 'behaviors', 'bosses', 'variety', 'profile', 'runes', 'zones', 'zones-extra', 'gauntlets', 'swing', 'puzzles', 'dragons',
    'npcs', 'npc-story', 'npc-story-late', 'dialogue', 'trade', 'duel-patterns', 'duel', 'place-rooms', 'interior-mobs', 'interior-bosses', 'interior-things', 'interior', 'places',
    'folk', 'folk-story', 'journal-story', 'village-rooms', 'place-rooms-deep', 'village', 'cutscenes', 'chronicle', 'ember', 'crafts', 'dice', 'bounties', 'levels', 'combat', 'sim',
    'crafts-ui', 'games-ui'];
for (const m of MODULES) {
    const abs = path.join(rootDir, `ui/js/desktop/apps/runenpfad/${m}.js`);
    if (fs.existsSync(abs)) vm.runInContext(fs.readFileSync(abs, 'utf8'), sandbox, { filename: abs });
}

const Sim = sandbox.RunenpfadSim;
const Rooms = sandbox.RunenpfadPlaceRooms;
const Interior = sandbox.RunenpfadInterior;
const Things = sandbox.RunenpfadInteriorThings;
const Mobs = sandbox.RunenpfadInteriorMobs;
const Profile = sandbox.RunenpfadProfile;
const Npcs = sandbox.RunenpfadNpcs;
const Trade = sandbox.RunenpfadTrade;
const Crafts = sandbox.RunenpfadCrafts;
const Ember = sandbox.RunenpfadEmber || null;
const GamesUi = sandbox.RunenpfadGamesUi;
assert.ok(Sim && Rooms && Interior && Things && Profile && Npcs && Trade && Crafts && GamesUi && sandbox.RunenpfadCraftsUi, 'modules loaded');

const STEP = Sim.STEP;
let passed = 0;
function check(name, fn) {
    try {
        fn();
        passed += 1;
    } catch (err) {
        console.error('FAILED: ' + name);
        throw err;
    }
}
const same = (a, b, msg) => assert.equal(JSON.stringify(a), JSON.stringify(b), msg);

function tick(sim, input, n) {
    const events = [];
    for (let i = 0; i < (n || 1); i++) events.push(...Sim.stepSimulation(sim, STEP * 1.000001, input || {}));
    return events;
}
function hubSim(profile) {
    const sim = Sim.createSimulation({ mode: 'campaign', levelId: 0, hero: 'knight', profile: profile ? Profile.importProfile(Object.assign({ v: 1 }, profile)) : undefined });
    tick(sim, {}, 30);
    sim.events = [];
    return sim;
}
function houseSim(placeId, profile, roomId, at) {
    const sim = hubSim(profile);
    sim.interior = Interior.create(sim, placeId, null);
    if (roomId) Interior.enterRoom(sim, sim.interior, roomId, at);
    sim.player.hp = 99;
    sim.player.maxHp = 99;
    return sim;
}
const thingsOf = (it, kind) => it.things.filter((t) => t.kind === kind);
const isFree = (it, cx, cy) => cx >= 0 && cy >= 0 && cx < it.room.cols && cy < it.room.rows && !it.room.solid[cy * it.room.cols + cx];
const heroCell = (it) => ({ cx: Math.floor(it.hero.x / 48), cy: Math.floor((it.hero.y - 7) / 48) });
function putAt(it, cx, cy) {
    it.hero.x = (cx + 0.5) * 48;
    it.hero.y = (cy + 0.5) * 48 + 7;
    it.hero.vx = 0;
    it.hero.vy = 0;
}
function cellPath(it, from, to) {
    const key = (c) => c.cx + ',' + c.cy;
    const prev = { [key(from)]: null };
    const queue = [from];
    while (queue.length) {
        const c = queue.shift();
        if (c.cx === to.cx && c.cy === to.cy) {
            const out = [];
            for (let k = c; k; k = prev[key(k)]) out.unshift(k);
            return out;
        }
        for (const [dx, dy] of [[1, 0], [-1, 0], [0, 1], [0, -1]]) {
            const n = { cx: c.cx + dx, cy: c.cy + dy };
            if (!isFree(it, n.cx, n.cy) || (key(n) in prev)) continue;
            prev[key(n)] = c;
            queue.push(n);
        }
    }
    return null;
}
function walkTo(sim, it, cx, cy) {
    const path2 = cellPath(it, heroCell(it), { cx, cy });
    assert.ok(path2, 'a way to ' + cx + ',' + cy);
    for (const c of path2.slice(1)) {
        const tx = (c.cx + 0.5) * 48;
        const ty = (c.cy + 0.5) * 48 + 7;
        for (let i = 0; i < 150 && (Math.abs(it.hero.x - tx) > 3 || Math.abs(it.hero.y - ty) > 3); i++) {
            const input = {};
            if (Math.abs(it.hero.x - tx) >= Math.abs(it.hero.y - ty)) input[it.hero.x < tx ? 'right' : 'left'] = true;
            else input[it.hero.y < ty ? 'down' : 'up'] = true;
            tick(sim, input);
            assert.ok(sim.interior === it, 'still in the room while walking');
        }
    }
}
function touch(sim, it, t) {
    const side = [[0, 1, 'up'], [0, -1, 'down'], [-1, 0, 'right'], [1, 0, 'left']].filter(([dx, dy]) => isFree(it, t.cx + dx, t.cy + dy) && cellPath(it, heroCell(it), { cx: t.cx + dx, cy: t.cy + dy }))[0];
    assert.ok(side, 'a free side of ' + (t.id || t.lootId));
    walkTo(sim, it, t.cx + side[0], t.cy + side[1]);
    const input = {};
    input[side[2]] = true;
    tick(sim, input, 12);
}
function use(sim, it, t) {
    touch(sim, it, t);
    tick(sim, {}, 2);
    return tick(sim, { talkPressed: 'e' });
}
function smash(sim, it, t) {
    touch(sim, it, t);
    const dir = Math.sign(heroCell(it).cx - t.cx) || 0;
    it.hero.ax = -dir;
    it.hero.ay = dir ? 0 : Math.sign(t.cy - heroCell(it).cy);
    for (let i = 0; i < 120 && it.hero.attackCool > 0; i++) tick(sim, {});
    return tick(sim, { attackPressed: true });
}
function chestOf(it, id) { return it.chests.filter((c) => c.lootId === id)[0]; }
function openChest(sim, it, id) {
    const c = chestOf(it, id);
    assert.ok(c, 'chest ' + id);
    touch(sim, it, { cx: Math.floor(c.x / 48), cy: Math.floor((c.y - 7) / 48), id });
    tick(sim, {}, 2);
    return tick(sim, { talkPressed: 'e' });
}
const closeTalk = (sim) => { if (sim.encounter) sim.encounter = null; };
const NEW_ROOMS = ['tavern_kitchen', 'tavern_upstairs', 'shop_store', 'shop_attic', 'smithy_forge', 'smithy_coal', 'hall_archive', 'hall_bell', 'edda_home', 'edda_study', 'edda_attic'];

// ---- 1-2: data ----
check('the rooms validate and the places list the new rooms', () => {
    same(Rooms.validate(), []);
    same(Rooms.PLACES.tavern.rooms, ['tavern_main', 'tavern_cellar', 'tavern_kitchen', 'tavern_upstairs']);
    same(Rooms.PLACES.shop.rooms, ['shop_main', 'shop_store', 'shop_attic']);
    same(Rooms.PLACES.smithy.rooms, ['smithy_main', 'smithy_forge', 'smithy_coal']);
    same(Rooms.PLACES.hall.rooms, ['hall_main', 'hall_archive', 'hall_bell']);
    same(Rooms.PLACES.edda_house.rooms, ['edda_home', 'edda_study', 'edda_attic']);
    assert.equal(Rooms.PLACES.edda_house.facade.clip, 'fac-deep.f3');
    assert.equal(Rooms.PLACES.edda_house.facade.h, 260);
    NEW_ROOMS.forEach((id) => assert.ok(Rooms.ROOMS[id], id));
});

check('every new room has a light and something to use or look at; the pinned cells of the old rooms stay', () => {
    NEW_ROOMS.forEach((id) => {
        const lay = Rooms.layout(id);
        const hasThingLight = lay.lights.length > 0;
        assert.ok(hasThingLight, id + ' has a light');
        const d = Rooms.ROOMS[id];
        assert.ok(d.things.some((t) => t.use || t.chest || t.sign || t.brazier || t.crack || t.block || t.npc || t.pot || t.prop), id + ' has something in it');
    });
    const uses = (id) => Rooms.ROOMS[id].things.filter((t) => t.use != null).length;
    const count = (id, ch) => Rooms.layout(id).markers[ch].length;
    assert.equal(count('tavern_main', 'Y'), 2, 'the dice table keeps its two cells');
    assert.equal(count('hall_main', 'B'), 2, 'the task board keeps its two cells');
    assert.equal(uses('tavern_main'), 1);
    assert.equal(uses('hall_main'), 1);
    assert.equal(Rooms.layout('hall_main', hubSim()).lights.filter((l) => l.color === '#9ad8ff').length, 0, 'no pedestal light without the story');
    assert.equal(Rooms.ROOMS.tavern_cellar.things.filter((t) => t.mob === 'rat').length, 1);
    assert.equal(Rooms.layout('tavern_cellar').markers.r.length, 5, 'five rats');
    assert.equal(Rooms.ROOMS.tavern_cellar.things.filter((t) => t.chest).length, 1);
    // every old marker thing is still there
    const main = Rooms.ROOMS.tavern_main.things;
    ['innkeeper', 'bard', 'patron', 'child', 'gambler'].forEach((n) => assert.ok(main.some((t) => t.npc === n), n));
});

check('every new room is reachable by walking the exits, and every exit can be walked to', () => {
    const sim = houseSim('tavern');
    const it = sim.interior;
    const walkAll = (placeId, entryRoom, profile) => {
        const s = houseSim(placeId, profile);
        const seen = {};
        const stack = [[entryRoom, null]];
        while (stack.length) {
            const [rid, at] = stack.pop();
            if (seen[rid]) continue;
            seen[rid] = true;
            Interior.enterRoom(s, s.interior, rid, at);
            const room = s.interior.room;
            const start = heroCell(s.interior);
            assert.ok(isFree(s.interior, start.cx, start.cy), rid + ': the arrival cell is free');
            room.exits.forEach((e) => {
                if (e.to === 'out') return;
                assert.ok(cellPath(s.interior, start, { cx: e.cx, cy: e.cy }), rid + ' -> ' + e.to + ' is reachable');
                stack.push([e.to, e.at]);
            });
        }
        return Object.keys(seen);
    };
    assert.equal(it.roomId, 'tavern_main');
    same(walkAll('tavern', 'tavern_main').sort(), ['tavern_cellar', 'tavern_kitchen', 'tavern_main', 'tavern_upstairs']);
    same(walkAll('shop', 'shop_main').sort(), ['shop_attic', 'shop_main', 'shop_store']);
    same(walkAll('smithy', 'smithy_main').sort(), ['smithy_coal', 'smithy_forge', 'smithy_main']);
    same(walkAll('edda_house', 'edda_home').sort(), ['edda_attic', 'edda_home', 'edda_study']);
    // the archive door is shut at the start, so the walk to it is blocked until the first tremor
    const hall = houseSim('hall', undefined, 'hall_main', null);
    const W = hall.interior.room.exits.filter((e) => e.to === 'hall_archive')[0];
    assert.equal(cellPath(hall.interior, heroCell(hall.interior), { cx: W.cx, cy: W.cy }), null, 'shut at step 0');
    assert.ok(cellPath(hall.interior, heroCell(hall.interior), { cx: hall.interior.room.exits.filter((e) => e.to === 'hall_bell')[0].cx, cy: 3 }), 'the bell room is open');
});

// ---- 3: the use cells ----
check('the kitchen table asks for the cook window and the furnace for the forge window', () => {
    const k = houseSim('tavern', undefined, 'tavern_kitchen', 'd');
    const cook = thingsOf(k.interior, 'use');
    assert.equal(cook.length, 2);
    assert.equal(cook[0].prompt, 'cook');
    use(k, k.interior, cook[0]);
    assert.equal(k.uiRequest, 'cook');
    const f = houseSim('smithy', undefined, 'smithy_forge', 'w');
    const furnace = thingsOf(f.interior, 'use');
    assert.equal(furnace.length, 2);
    assert.equal(furnace[0].prompt, 'forge');
    use(f, f.interior, furnace[0]);
    assert.equal(f.uiRequest, 'forge');
});

// ---- 4: archive, shelves, coal ----
check('the archive gate opens with the first tremor, the shelves open the niche in the order of the sun', () => {
    const gate = (profile) => {
        const sim = houseSim('hall', profile, 'hall_main', null);
        return thingsOf(sim.interior, 'gate').filter((g) => g.gid === 'archive')[0];
    };
    assert.equal(gate().open, false, 'shut at step 0');
    if (Ember) {
        assert.equal(gate({ seen: ['dr_tremor'] }).open, true, 'open after dr_tremor');
        const sim = houseSim('hall', { seen: ['dr_tremor'] }, 'hall_main', null);
        const W = sim.interior.room.exits.filter((e) => e.to === 'hall_archive')[0];
        const start = heroCell(sim.interior);
        putAt(sim.interior, W.cx + 1, W.cy);
        for (let i = 0; i < 80 && sim.interior.roomId === 'hall_main'; i++) tick(sim, { left: true });
        assert.equal(sim.interior.roomId, 'hall_archive', 'walked through the archive door from ' + JSON.stringify(start));
    }
    const sim = houseSim('hall', undefined, 'hall_archive', 'x');
    const it = sim.interior;
    const shelves = thingsOf(it, 'brazier').filter((b) => b.set === 'books');
    assert.equal(shelves.length, 4);
    const byId = (id) => shelves.filter((b) => b.bid === id)[0];
    const niche = () => thingsOf(it, 'gate').filter((g) => g.gid === 'niche')[0];
    assert.equal(niche().open, false);
    // a wrong first pull resets the order
    const wrong = use(sim, it, byId('shelf_s'));
    assert.ok(wrong.some((e) => e.type === 'in_puzzle_fail'));
    ['shelf_e', 'shelf_s', 'shelf_w'].forEach((id) => { use(sim, it, byId(id)); assert.equal(niche().open, false); });
    use(sim, it, byId('shelf_n'));
    tick(sim, {}, 30);
    assert.equal(niche().open, true, 'the niche opens');
    const coins = sim.profile.coins;
    const ev = openChest(sim, it, 'hall_niche');
    assert.ok(ev.some((e) => e.type === 'in_chest' && e.detail.lootId === 'hall_niche' && !e.detail.empty));
    assert.equal(sim.profile.coins, coins + 60);
    // the tablets: read once each
    const tabs = thingsOf(it, 'sign').filter((s) => s.node === 'tab_1' || s.node === 'tab_2');
    assert.equal(tabs.length, 2);
});

check('coal cellar: three rats, a cracked wall, the locked chest', () => {
    const sim = houseSim('smithy', undefined, 'smithy_coal', 'e');
    const it = sim.interior;
    assert.equal(it.mobs.filter((m) => m.kind === 'rat').length, 3);
    const crack = thingsOf(it, 'crack')[0];
    const chest = chestOf(it, 'smithy_coal');
    assert.ok(chest.locked, 'locked until the rats are down');
    assert.equal(cellPath(it, heroCell(it), { cx: Math.floor(chest.x / 48) - 1, cy: Math.floor((chest.y - 7) / 48) }), null, 'the crack shuts the nook');
    it.mobs.forEach((m) => Mobs.hit(sim, it, m, 99, 0, 1));
    tick(sim, {}, 3);
    assert.equal(chest.locked, false);
    smash(sim, it, crack);
    smash(sim, it, crack);
    tick(sim, {}, 4);
    assert.equal(crack.broken, true, 'two blows break the crack');
    const coins = sim.profile.coins;
    const potions = sim.player.potions;
    const ev = openChest(sim, it, 'smithy_coal');
    assert.ok(ev.some((e) => e.type === 'in_chest' && e.detail.lootId === 'smithy_coal' && !e.detail.empty));
    assert.equal(sim.profile.coins, coins + 40);
    assert.equal(sim.player.potions, potions + 1);
});

// ---- 5: shop crates, attic, keys ----
check('shop store: push a crate up the column to reach the amulet; the attic chest pays', () => {
    const sim = houseSim('shop', undefined, 'shop_store', 'e');
    const it = sim.interior;
    const crates = thingsOf(it, 'block');
    assert.equal(crates.length, 4);
    const c = chestOf(it, 'shop_shelf');
    const cc = { cx: Math.floor(c.x / 48), cy: Math.floor((c.y - 7) / 48) };
    const reach = () => [[-1, 0], [1, 0], [0, 1], [0, -1]].some(([dx, dy]) => isFree(it, cc.cx + dx, cc.cy + dy) && cellPath(it, heroCell(it), { cx: cc.cx + dx, cy: cc.cy + dy }));
    assert.equal(reach(), false, 'the nook is shut by the crates');
    const crate = crates.filter((b) => b.cx === 10)[0];
    putAt(it, 10, crate.cy + 1);
    for (let n = 0; n < 2; n++) {
        const from = crate.cy;
        for (let i = 0; i < 160 && crate.cy === from; i++) tick(sim, { up: true });
        tick(sim, {}, 20);
        assert.equal(crate.cy, from - 1, 'pushed up by one');
        putAt(it, 10, crate.cy + 1);
    }
    assert.equal(reach(), true, 'a way in');
    const coins = sim.profile.coins;
    const ev = openChest(sim, it, 'shop_shelf');
    assert.ok(ev.some((e) => e.type === 'in_chest' && e.detail.lootId === 'shop_shelf' && !e.detail.empty));
    assert.equal(sim.profile.coins, coins + 50);
    assert.equal(Npcs.itemCount(sim, 'lucky_amulet'), 1);
    const attic = houseSim('shop', undefined, 'shop_attic', 'd');
    const c0 = attic.profile.coins;
    openChest(attic, attic.interior, 'shop_attic');
    assert.equal(attic.profile.coins, c0 + 30);
    assert.ok(attic.profile.loot.includes('shop_attic'));
});

check("Edda's trunk gives the room key; it opens the room upstairs; the desk needs its own key", () => {
    // the key from the attic trunk
    const a = houseSim('edda_house', undefined, 'edda_attic', 'd');
    assert.equal(Npcs.itemCount(a, 'room_key'), 0);
    openChest(a, a.interior, 'edda_attic');
    assert.equal(Npcs.itemCount(a, 'room_key'), 1);
    assert.ok(a.profile.loot.includes('edda_attic'));
    // the gate upstairs: shut without the key, open with it
    const gateOf = (s) => thingsOf(s.interior, 'gate').filter((g) => g.gid === 'edda_room')[0];
    const shut = houseSim('tavern', undefined, 'tavern_upstairs', 'd');
    assert.equal(gateOf(shut).open, false);
    const withKey = houseSim('tavern', { inventory: { room_key: 1 } }, 'tavern_upstairs', 'd');
    assert.equal(gateOf(withKey).open, true, 'a gate opens while the key is held');
    assert.equal(Npcs.itemCount(withKey, 'room_key'), 1, 'keys are not consumed');
    const coins = withKey.profile.coins;
    const ev = openChest(withKey, withKey.interior, 'edda_chest');
    assert.ok(ev.some((e) => e.type === 'in_chest' && e.detail.lootId === 'edda_chest' && !e.detail.empty));
    assert.equal(withKey.profile.coins, coins + 60);
    assert.equal(Npcs.itemCount(withKey, 'rune_attune'), 1);
    // the desk: refuses without desk_key
    const desk = houseSim('edda_house', undefined, 'edda_study', 'w');
    const c0 = desk.profile.coins;
    const refused = openChest(desk, desk.interior, 'edda_desk');
    assert.ok(refused.some((e) => e.type === 'in_locked'));
    assert.equal(desk.profile.coins, c0);
    assert.equal(desk.profile.loot.includes('edda_desk'), false);
    Npcs.addItem(desk, 'desk_key', 1);
    const ok = openChest(desk, desk.interior, 'edda_desk');
    assert.ok(ok.some((e) => e.type === 'in_chest' && e.detail.lootId === 'edda_desk' && !e.detail.empty));
    assert.equal(desk.profile.coins, c0 + 50);
});

// ---- 6: kitchen rules ----
check('recipes are unordered pairs; cooking consumes the pair and sets the meal', () => {
    assert.equal(Crafts.recipeFor('ham', 'turnip'), 'stew');
    assert.equal(Crafts.recipeFor('turnip', 'ham'), 'stew');
    assert.equal(Crafts.recipeFor('honey', 'frostberry'), 'tea');
    assert.equal(Crafts.recipeFor('turnip', 'honey'), 'broth');
    assert.equal(Crafts.recipeFor('ham', 'honey'), null);
    assert.equal(Crafts.recipeFor('ham', 'ham'), null);
    const sim = hubSim();
    assert.equal(Crafts.canCook(sim, 'ham', 'turnip'), 'missing');
    assert.equal(Crafts.canCook(sim, 'ham', 'honey'), 'none');
    ['ham', 'turnip', 'honey', 'frostberry'].forEach((id) => Npcs.addItem(sim, id, 2));
    assert.equal(Crafts.canCook(sim, 'ham', 'turnip'), 'ok');
    sim.events = [];
    const r = Crafts.cook(sim, 'turnip', 'ham');
    same([r.ok, r.recipe], [true, 'stew']);
    assert.equal(Npcs.itemCount(sim, 'ham'), 1);
    assert.equal(Npcs.itemCount(sim, 'turnip'), 1);
    assert.equal(Npcs.getFlag(sim, 'dr.meal'), 1, 'stew is meal 1');
    assert.equal(sim.meal, 'stew');
    assert.equal(sim.notice, 'runenpfad.notice.meal_stew');
    const ev = sim.events.filter((e) => e.type === 'craft_cook');
    assert.equal(ev.length, 1);
    assert.equal(ev[0].detail.recipe, 'stew');
    assert.ok(!('x' in ev[0].detail) && !('y' in ev[0].detail), 'no position on interior events');
    // a meal is waiting: no second one
    assert.equal(Crafts.canCook(sim, 'honey', 'frostberry'), 'full');
    const again = Crafts.cook(sim, 'honey', 'frostberry');
    assert.equal(again.ok, false);
    assert.equal(Npcs.itemCount(sim, 'honey'), 2, 'nothing consumed on a refusal');
    assert.equal(Npcs.getFlag(sim, 'dr.meal'), 1);
});

// ---- 7: forge rules ----
check('forge: three good strikes make the piece, coins and scale are used up', () => {
    const sim = hubSim({ coins: 100 });
    assert.equal(Crafts.forgeStart(sim, 'shield'), null, 'no scale, no work');
    Npcs.addItem(sim, 'dragon_scale', 1);
    const opts = Crafts.forgeOptions(sim);
    same(opts.map((o) => [o.id, o.ready, o.done]), [['shield', true, false], ['bell', false, false]]);
    const run = Crafts.forgeStart(sim, 'shield');
    same([run.phase, run.pos, run.dir, run.speed, run.ok], ['run', 0, 1, 1.2, null]);
    // the needle ping-pongs
    Crafts.forgeStep(run, 0.6);
    assert.ok(Math.abs(run.pos - 0.72) < 1e-9);
    Crafts.forgeStep(run, 0.6);
    assert.ok(Math.abs(run.pos - 0.56) < 1e-9 && run.dir === -1, 'bounced off the end');
    sim.events = [];
    const speeds = [];
    for (let i = 0; i < 3; i++) {
        run.pos = 0.5 + (i - 1) * 0.1;
        speeds.push(run.speed);
        assert.equal(Crafts.forgeStrike(sim, run), 'good');
    }
    assert.ok(speeds[1] > speeds[0] && speeds[2] > speeds[1], 'faster with every strike');
    same([run.phase, run.ok], ['done', true]);
    assert.equal(Npcs.itemCount(sim, 'dragon_scale'), 0);
    assert.equal(sim.profile.coins, 60);
    assert.equal(Npcs.getFlag(sim, 'dr.shield'), 1);
    assert.equal(sim.notice, 'runenpfad.notice.forged_shield');
    assert.equal(sim.events.filter((e) => e.type === 'craft_strike' && e.detail.quality === 'good').length, 3);
    const done = sim.events.filter((e) => e.type === 'craft_done');
    assert.equal(done.length, 1);
    same([done[0].detail.recipe, done[0].detail.ok], ['shield', true]);
    assert.equal(Crafts.forgeOptions(sim)[0].done, true);
    assert.equal(Crafts.forgeStart(sim, 'shield'), null, 'once is enough');
});

check('forge: two bad strikes fail, the material stays, the coins are gone', () => {
    const sim = hubSim({ coins: 100 });
    Npcs.addItem(sim, 'dragon_scale', 1);
    const run = Crafts.forgeStart(sim, 'shield');
    run.pos = 0.9;
    assert.equal(Crafts.forgeStrike(sim, run), 'bad');
    run.pos = 0.5;
    assert.equal(Crafts.forgeStrike(sim, run), 'good');
    run.pos = 0.1;
    assert.equal(Crafts.forgeStrike(sim, run), 'bad');
    same([run.phase, run.ok], ['done', false]);
    assert.equal(Npcs.itemCount(sim, 'dragon_scale'), 1, 'the scale is kept');
    assert.equal(sim.profile.coins, 60, 'the work was paid');
    assert.equal(Npcs.getFlag(sim, 'dr.shield'), 0);
    assert.equal(Crafts.forgeStrike(sim, run), 'bad', 'a finished run takes no more strikes');
    assert.equal(run.strikes.length, 3);
    // the bell costs nothing and mends with two good strikes
    const bell = hubSim({ coins: 0 });
    Npcs.addItem(bell, 'bell_bronze', 1);
    const br = Crafts.forgeStart(bell, 'bell');
    assert.ok(br);
    [0.5, 0.95, 0.45].forEach((p) => { br.pos = p; Crafts.forgeStrike(bell, br); });
    assert.equal(br.ok, true);
    assert.equal(Npcs.getFlag(bell, 'dr.bell'), 1);
    assert.equal(Npcs.itemCount(bell, 'bell_bronze'), 0);
    assert.equal(bell.notice, 'runenpfad.notice.forged_bell');
    // too poor for the shield: not ready
    const poor = hubSim({ coins: 10 });
    Npcs.addItem(poor, 'dragon_scale', 1);
    assert.equal(Crafts.forgeOptions(poor)[0].ready, false);
});

// ---- 8: Tilda's ingredients ----
check("Tilda's ingredient offers: prices, stock and the item cap", () => {
    const prices = { turnip: 4, ham: 6, frostberry: 6, honey: 5 };
    Object.keys(prices).forEach((id) => {
        const o = Trade.OFFERS[id];
        assert.ok(o, id);
        same([o.base, o.stock, o.kind], [prices[id], 3, 'item']);
        assert.equal(o.nameKey, 'runenpfad.npc.item.' + id + '.name');
    });
    ['bell_bronze', 'dragon_scale', 'desk_key', 'room_key', 'clan_charm', 'turnip', 'honey'].forEach((id) => {
        assert.ok(Trade.textKeys().includes('runenpfad.npc.item.' + id + '.name'), id + ' has text keys');
        assert.ok(Trade.textKeys().includes('runenpfad.npc.item.' + id + '.desc'));
    });
    if (!Npcs.NPCS.shopkeeper || !Npcs.NPCS.shopkeeper.offers.includes('turnip')) return; // Tilda's list lives in folk.js (another package)
    const sim = hubSim({ coins: 200 });
    const npc = { npcId: 'shopkeeper', sold: {} };
    const listed = Trade.offersFor(sim, npc).filter((o) => prices[o.id]);
    assert.equal(listed.length, 4);
    same(listed.map((o) => [o.id, o.price, o.stock]), [['turnip', 4, 3], ['ham', 6, 3], ['frostberry', 6, 3], ['honey', 5, 3]]);
    for (let i = 0; i < 3; i++) assert.ok(Trade.buy(sim, npc, 'turnip').ok);
    assert.equal(Trade.buy(sim, npc, 'turnip').reason, 'stock');
    assert.equal(Npcs.itemCount(sim, 'turnip'), 3);
    assert.equal(sim.profile.coins, 188);
});

// ---- 9: the windows ----
function makeUi() {
    const log = { emitted: [], flushes: 0 };
    const ui = GamesUi.create({
        el: (tag, cls, text) => { const n = new Node(tag); if (cls) n.className = cls; if (text !== undefined) n.textContent = text; return n; },
        t: (k) => k, fill: (k, p) => k + JSON.stringify(p || {}), hud: { icon: () => new Node('span'), setButton: (b, label, key) => { b.textContent = label; b.keyLabel = key; } },
        emit: (evs) => log.emitted.push(...evs), cue: () => {}, onChange: () => {}, flush: () => { log.flushes += 1; }, isBlocked: () => false
    });
    return { ui, log };
}
const key = (code) => ({ code, repeat: false, preventDefault() {} });
const labels = (ui) => ui.root.querySelectorAll().map((b) => b.textContent);
const bodyText = (ui) => ui.root.textContent;

check('the cook window opens, picks two ingredients by key and cooks', () => {
    const { ui, log } = makeUi();
    const sim = hubSim();
    ['ham', 'turnip', 'honey'].forEach((id) => Npcs.addItem(sim, id, 1));
    assert.equal(ui.request('cook', { sim }), true);
    assert.equal(ui.isOpen(), true);
    assert.equal(ui.root.attrs['aria-label'], 'runenpfad.craft.cook_title');
    assert.equal(ui.root.dataset.rpGames, 'cook');
    clock += 400;
    // ham, turnip, honey are enabled (frostberry has none), cook is disabled, leave is last
    assert.equal(labels(ui).length, 4, 'three ingredients and leave');
    assert.ok(bodyText(ui).includes('runenpfad.craft.pick'));
    ui.handleKey(key('Digit1')); // ham (the first enabled button; the list is turnip, ham, frostberry, honey so this is turnip)
    ui.handleKey(key('Digit2'));
    assert.ok(bodyText(ui).includes('runenpfad.craft.stew'), 'the preview names the stew');
    assert.ok(labels(ui).includes('runenpfad.craft.cook'));
    // Enter on the cook button: move the focus onto it first
    const list = ui.root.querySelectorAll();
    const idx = list.findIndex((b) => b.textContent === 'runenpfad.craft.cook');
    for (let i = 0; i < idx; i++) ui.handleKey(key('ArrowDown'));
    ui.handleKey(key('Enter'));
    assert.equal(Npcs.getFlag(sim, 'dr.meal'), 1);
    assert.equal(Npcs.itemCount(sim, 'ham'), 0);
    assert.ok(log.emitted.some((e) => e.type === 'craft_cook'), 'the audio layer got the event');
    assert.ok(bodyText(ui).includes('runenpfad.craft.stew_desc'), 'the result is shown');
    // a second meal is refused while one waits
    assert.equal(Crafts.canCook(sim, 'honey', 'turnip'), 'full');
    assert.equal(ui.handleKey(key('Escape')), true);
    assert.equal(ui.isOpen(), false);
    assert.ok(log.flushes > 0, 'the story was saved on close');
});

check('the forge window: pick a job, Space strikes, three good strikes forge the shield', () => {
    const { ui, log } = makeUi();
    const sim = hubSim({ coins: 100 });
    Npcs.addItem(sim, 'dragon_scale', 1);
    // aim every strike: the needle is set into the good zone just before the rules run
    const origStrike = Crafts.forgeStrike;
    Crafts.forgeStrike = (s, run) => { run.pos = 0.5; return origStrike(s, run); };
    try {
        assert.equal(ui.request('forge', { sim }), true);
        assert.equal(ui.root.attrs['aria-label'], 'runenpfad.craft.forge_title');
        clock += 400;
        assert.ok(bodyText(ui).includes('runenpfad.craft.shield_desc'));
        assert.ok(bodyText(ui).includes('runenpfad.craft.needs'));
        ui.handleKey(key('Enter')); // the first button is the shield job
        const strikeBtn = ui.root.querySelectorAll()[0];
        assert.equal(strikeBtn.textContent, 'runenpfad.craft.strike', 'Strike is the first focusable control');
        assert.ok(ui.root.all().some((n) => n.tag === 'canvas'), 'the meter canvas');
        assert.ok(ui.root.all().filter((n) => n.tag === 'canvas')[0].ctx.calls.length > 5, 'the meter was painted');
        ui.handleKey(key('Space'));
        ui.handleKey(key('Enter'));
        assert.equal(Npcs.getFlag(sim, 'dr.shield'), 0, 'two strikes are not enough');
        ui.handleKey(key('Space'));
        assert.equal(Npcs.getFlag(sim, 'dr.shield'), 1);
        assert.ok(bodyText(ui).includes('runenpfad.craft.success'));
        assert.ok(log.emitted.filter((e) => e.type === 'craft_strike').length === 3);
        assert.ok(log.emitted.some((e) => e.type === 'craft_done' && e.detail.ok));
        const next = labels(ui);
        assert.ok(next.includes('runenpfad.craft.again') && next.includes('runenpfad.craft.leave'));
        ui.handleKey(key('Escape'));
        assert.equal(ui.isOpen(), false);
    } finally {
        Crafts.forgeStrike = origStrike;
    }
});

check('the forge needle moves with the animation frames and stops with the window', () => {
    const { ui } = makeUi();
    const sim = hubSim({ coins: 100 });
    Npcs.addItem(sim, 'dragon_scale', 1);
    const origStep = Crafts.forgeStep;
    const seen = [];
    Crafts.forgeStep = (run, dt) => { seen.push(run.pos); return origStep(run, dt); };
    rafOn = true;
    try {
        ui.request('forge', { sim });
        clock += 400;
        ui.handleKey(key('Enter'));
        assert.equal(frames.length, 1, 'one loop is running');
        for (let i = 0; i < 6; i++) { clock += 16; frames.shift()(); }
        assert.equal(seen.length, 6);
        assert.ok(seen[5] > seen[0], 'the needle advanced');
        assert.ok(seen.every((p) => p >= 0 && p <= 1));
        clock += 5000; // a stalled tab: the step is clamped
        const before = seen.length;
        frames.shift()();
        assert.equal(seen.length, before + 1);
        const last = frames.length;
        ui.handleKey(key('Escape'));
        assert.equal(ui.isOpen(), false);
        while (frames.length) frames.shift()();
        assert.equal(seen.length, before + 1, 'no frame runs after the window closed');
        assert.equal(last, 1);
    } finally {
        rafOn = false;
        frames.length = 0;
        Crafts.forgeStep = origStep;
    }
});

check('the old windows still work and craft state is not a dice round', () => {
    const { ui } = makeUi();
    const sim = hubSim();
    assert.equal(ui.request('board', { sim }), true);
    assert.equal(ui.root.dataset.rpGames, 'board');
    ui.close();
    assert.equal(ui.request('dice:gambler', { sim }), true);
    assert.equal(ui.root.dataset.rpGames, 'dice');
    ui.close();
    assert.equal(ui.request('cook', { sim }), true);
    ui.reset();
    assert.equal(ui.isOpen(), false);
});

check('the archive tablets 1 and 2 are given once each', () => {
    const sim = houseSim('hall', undefined, 'hall_archive', 'x');
    const it = sim.interior;
    const bits = () => Npcs.getFlag(sim, 'dr.tabs');
    assert.equal(bits(), 0);
    const signs = ['tab_1', 'tab_2'].map((n) => thingsOf(it, 'sign').filter((t) => t.node === n)[0]);
    signs.forEach((t, i) => {
        const ev = use(sim, it, t);
        assert.ok(ev.some((e) => e.type === 'in_tablet' && e.detail.tablet === i + 1 && e.detail.first), 'tablet ' + (i + 1) + ' first read');
        closeTalk(sim);
        assert.equal(bits() & (1 << i), 1 << i);
    });
    assert.equal(bits(), 3);
    const again = use(sim, it, signs[0]);
    assert.ok(!again.some((e) => e.type === 'in_tablet'), 'no second gift');
    assert.equal(bits(), 3);
});

console.log('runenpfad crafts: ' + passed + ' checks passed');
