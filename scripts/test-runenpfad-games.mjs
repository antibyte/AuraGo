import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(__dirname, '..');

const sandbox = { console, Math, Number, String, Boolean, Array, Object, Map, Set, JSON, Date, Uint8Array, RegExp };
sandbox.window = sandbox;
sandbox.globalThis = sandbox;
sandbox.performance = { now: function () { return clock; } };
sandbox.setTimeout = function (fn) { timers.push(fn); return timers.length; };
sandbox.clearTimeout = function () {};
let clock = 1000;
const timers = [];

// A tiny DOM: enough tree, classList, dataset and click() for the games window.
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
    getContext() { return null; }
    all(out) { out = out || []; this.children.forEach((c) => { out.push(c); c.all(out); }); return out; }
    querySelectorAll(sel) { return this.all().filter((n) => n.tag === 'button' && !n.disabled); }
}
sandbox.document = { createElement: (tag) => new Node(tag) };
vm.createContext(sandbox);

const MODULES = ['biomes', 'fx', 'physics', 'behaviors', 'bosses', 'variety', 'profile', 'runes', 'zones', 'zones-extra', 'gauntlets', 'swing', 'puzzles', 'dragons',
    'npcs', 'npc-story', 'npc-story-late', 'dialogue', 'trade', 'duel-patterns', 'duel', 'place-rooms', 'interior-mobs', 'interior', 'places', 'folk', 'folk-story', 'village-rooms', 'village',
    'cutscenes', 'dice', 'bounties', 'levels', 'combat', 'sim', 'games-ui'];
for (const m of MODULES) {
    const abs = path.join(rootDir, `ui/js/desktop/apps/runenpfad/${m}.js`);
    if (fs.existsSync(abs)) vm.runInContext(fs.readFileSync(abs, 'utf8'), sandbox, { filename: abs });
}
const Sim = sandbox.RunenpfadSim;
const Dice = sandbox.RunenpfadDice;
const Bounties = sandbox.RunenpfadBounties;
const GamesUi = sandbox.RunenpfadGamesUi;
assert.ok(Sim && Dice && Bounties && GamesUi, 'modules loaded');

// The modules run in another realm: compare arrays by value through JSON.
function same(a, b) { assert.equal(JSON.stringify(a), JSON.stringify(b)); }
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

function newSim(seed) {
    const sim = Sim.createSimulation({ levelId: 3 });
    sim.rngState = seed >>> 0;
    sim.events = [];
    return sim;
}
function flag(sim, key) { return sim.profile.npcFlags[key] || 0; }

// ---- dice ----
check('a round is deterministic by seed and keeps the dice in 1..6', () => {
    const run = (seed) => {
        const sim = newSim(seed);
        sim.profile.coins = 100;
        const g = Dice.create(sim, 'gambler');
        assert.ok(Dice.roll(g, 5));
        const first = JSON.stringify([g.mine, g.theirs]);
        Dice.hold(g);
        return JSON.stringify([first, g.result, g.delta, sim.profile.coins]);
    };
    assert.equal(run(7), run(7));
    assert.notEqual(new Set([1, 2, 3, 4, 5, 6, 7, 8].map(run)).size, 1);
    const sim = newSim(3);
    sim.profile.coins = 100;
    const g = Dice.create(sim, 'gambler');
    Dice.roll(g, 10);
    [...g.mine, ...g.theirs].forEach((d) => assert.ok(d >= 1 && d <= 6));
});

check('stakes are limited by both purses and the phase', () => {
    const sim = newSim(1);
    const g = Dice.create(sim, 'gambler');
    sim.profile.coins = 12;
    same(Dice.stakes(g), [5, 10]);
    sim.stage.dicePurse.gambler = 7;
    same(Dice.stakes(g), [5]);
    sim.profile.coins = 3;
    same(Dice.stakes(g), []);
    assert.equal(Dice.status(g), 'poor');
    sim.profile.coins = 50;
    sim.stage.dicePurse.gambler = 3;
    assert.equal(Dice.status(g), 'broke');
    assert.equal(Dice.roll(g, 5), false, 'a stake neither side can pay is refused');
    sim.stage.dicePurse.gambler = 60;
    assert.equal(Dice.reroll(g, 0), false, 'no re-roll before the roll');
    assert.ok(Dice.roll(g, 20));
    assert.equal(Dice.roll(g, 5), false, 'one roll per round');
    assert.ok(Dice.reroll(g, 1));
    assert.equal(g.phase, 'done');
    assert.equal(Dice.hold(g), false);
});

function findSeed(result, opponent) {
    for (let seed = 1; seed < 400; seed++) {
        const sim = newSim(seed);
        sim.profile.coins = 100;
        const g = Dice.create(sim, opponent);
        Dice.roll(g, 5);
        Dice.hold(g);
        if (g.result === result) return seed;
    }
    throw new Error('no seed for ' + result);
}

check('win, lose and tie move coins and the purse; the events are emitted', () => {
    const win = newSim(findSeed('win', 'gambler'));
    win.profile.coins = 100;
    let g = Dice.create(win, 'gambler');
    Dice.roll(g, 5);
    Dice.hold(g);
    assert.equal(win.profile.coins, 105);
    assert.equal(win.stage.dicePurse.gambler, 55);
    assert.equal(flag(win, 'dice.wins'), 1);
    assert.equal(win.storyDirty, true);
    assert.ok(win.events.some((e) => e.type === 'dice_roll'));
    same(win.events.filter((e) => e.type === 'dice_end').map((e) => e.detail.result), ['win']);

    const lose = newSim(findSeed('lose', 'gambler'));
    lose.profile.coins = 100;
    g = Dice.create(lose, 'gambler');
    Dice.roll(g, 10);
    Dice.hold(g);
    assert.equal(lose.profile.coins, 90);
    assert.equal(lose.stage.dicePurse.gambler, 70);
    assert.equal(flag(lose, 'dice.wins'), 0);

    const tie = newSim(findSeed('tie', 'gambler'));
    tie.profile.coins = 100;
    g = Dice.create(tie, 'gambler');
    Dice.roll(g, 5);
    Dice.hold(g);
    assert.equal(tie.profile.coins, 100);
    assert.equal(tie.stage.dicePurse.gambler, 60);
});

check('the purse runs dry: no stake left and "broke" is reported', () => {
    const sim = newSim(findSeed('win', 'gambler'));
    sim.profile.coins = 100;
    sim.stage.dicePurse = { gambler: 8 };
    const g = Dice.create(sim, 'gambler');
    Dice.roll(g, 5);
    Dice.hold(g);
    assert.equal(sim.stage.dicePurse.gambler, 3);
    assert.ok(Dice.next(g));
    same(Dice.stakes(g), []);
    assert.equal(Dice.status(g), 'broke');
});

check('the first win against the barkeep sets barkeep.dice_won, a win elsewhere does not', () => {
    const seed = findSeed('win', 'barkeep');
    const bar = newSim(seed);
    bar.profile.coins = 100;
    assert.equal(flag(bar, 'barkeep.dice_won'), 0);
    const g = Dice.create(bar, 'barkeep');
    Dice.roll(g, 5);
    Dice.hold(g);
    assert.equal(flag(bar, 'barkeep.dice_won'), 1);
    const other = newSim(seed);
    other.profile.coins = 100;
    const h = Dice.create(other, 'gambler');
    Dice.roll(h, 5);
    Dice.hold(h);
    assert.equal(flag(other, 'barkeep.dice_won'), 0);
});

// ---- bounties ----
function ev(type, detail) { return { type: type, detail: detail || null }; }
const EVENTS = {
    1: () => ev('in_mob_die', { kind: 'slime' }), 2: () => ev('in_break', { kind: 'pot' }), 3: () => ev('pickup_gem'), 4: () => ev('stomp'),
    5: () => ev('combo', { count: 5 }), 6: () => ev('in_boss_die', { kind: 'brood' }), 7: () => ev('portal_win'), 8: () => ev('in_puzzle_done')
};

check('every task counts its event and completes exactly once', () => {
    assert.equal(Bounties.TABLE.length, 8);
    Bounties.TABLE.forEach((d) => {
        const sim = newSim(1);
        sim.profile.npcFlags['bounty.won'] = d.id - 1; // rotates the offers so that this task is the first one
        assert.ok(Bounties.offers(sim.profile).includes(d.id));
        assert.ok(Bounties.accept(sim, d.id), 'accept ' + d.id);
        const events = sim.events; // the real call passes sim.events: bounty_done lands in the same array
        for (let i = 0; i < d.need + 3; i++) events.push(EVENTS[d.id]());
        Bounties.track(sim, events);
        assert.equal(flag(sim, 'bounty.n'), d.need, 'progress capped at need for ' + d.id);
        assert.equal(events.filter((e) => e.type === 'bounty_done').length, 1, 'one bounty_done for ' + d.id);
        assert.equal(sim.notice, 'runenpfad.notice.bounty_done');
        sim.events = [EVENTS[d.id]()];
        Bounties.track(sim, sim.events);
        assert.equal(sim.events.length, 1, 'a finished task counts nothing and says nothing again');
    });
});

check('combat tasks respect their conditions and other events do nothing', () => {
    const sim = newSim(1);
    sim.profile.npcFlags['bounty.won'] = 4;
    Bounties.accept(sim, 5);
    Bounties.track(sim, [ev('combo', { count: 4 }), ev('stomp'), ev('combo', null)]);
    assert.equal(flag(sim, 'bounty.n'), 0);
    Bounties.drop(sim);
    sim.profile.npcFlags['bounty.won'] = 6;
    Bounties.accept(sim, 7);
    sim.damageTaken = 1;
    Bounties.track(sim, [ev('portal_win')]);
    assert.equal(flag(sim, 'bounty.n'), 0, 'damage taken: no clean finish');
    sim.damageTaken = 0;
    Bounties.track(sim, [ev('portal_win')]);
    assert.equal(flag(sim, 'bounty.n'), 1);
});

check('offers rotate with the completed tasks; one task at a time', () => {
    const sim = newSim(1);
    same(Bounties.offers(sim.profile), [1, 2, 3]);
    sim.profile.npcFlags['bounty.won'] = 3;
    same(Bounties.offers(sim.profile), [4, 5, 6]);
    sim.profile.npcFlags['bounty.won'] = 7;
    same(Bounties.offers(sim.profile), [8, 1, 2]);
    sim.profile.npcFlags['bounty.won'] = 0;
    assert.equal(Bounties.accept(sim, 4), false, 'a task that is not on offer');
    assert.ok(Bounties.accept(sim, 2));
    assert.equal(Bounties.accept(sim, 3), false, 'only one active task');
    assert.ok(Bounties.drop(sim));
    assert.equal(flag(sim, 'bounty.id'), 0);
});

check('claim pays once, counts the task and rotates the offers', () => {
    const sim = newSim(1);
    sim.profile.coins = 10;
    Bounties.accept(sim, 1);
    assert.equal(Bounties.claim(sim), 0, 'not finished yet');
    sim.events = Array.from({ length: 8 }, EVENTS[1]);
    Bounties.track(sim, sim.events);
    sim.events = [];
    assert.equal(Bounties.claim(sim), 30);
    assert.equal(sim.profile.coins, 40);
    assert.equal(flag(sim, 'bounty.won'), 1);
    assert.equal(flag(sim, 'bounty.id'), 0);
    assert.equal(Bounties.claim(sim), 0, 'a second claim pays nothing');
    assert.equal(sim.profile.coins, 40);
    same(Bounties.offers(sim.profile), [2, 3, 4]);
    assert.ok(sim.events.some((e) => e.type === 'bounty_claim'));
});

check('stepSimulation hands its events to the board once per call', () => {
    const sim = newSim(1);
    const calls = [];
    const real = sandbox.RunenpfadBounties.track;
    sandbox.RunenpfadBounties.track = function (s, events) { calls.push([s, events]); };
    const out = Sim.stepSimulation(sim, Sim.STEP * 3.2, {});
    sandbox.RunenpfadBounties.track = real;
    assert.equal(calls.length, 1);
    assert.equal(calls[0][0], sim);
    assert.equal(calls[0][1], out);
});

// ---- the window ----
function makeUi() {
    const log = { emitted: [], cues: [], changes: 0, flushes: 0 };
    const hud = {
        icon: (kind) => new Node('span'),
        setButton: (b, label, key) => { b.textContent = label; b.keyLabel = key; }
    };
    const ui = GamesUi.create({
        el: (tag, cls, text) => { const n = new Node(tag); if (cls) n.className = cls; if (text !== undefined) n.textContent = text; return n; },
        t: (k) => k, fill: (k, p) => k + JSON.stringify(p || {}), hud: hud,
        emit: (evs) => log.emitted.push(...evs), cue: (id) => log.cues.push(id), onChange: () => { log.changes += 1; },
        flush: () => { log.flushes += 1; }, isBlocked: () => false
    });
    return { ui, log };
}
const key = (code, extra) => Object.assign({ code: code, repeat: false, preventDefault() {} }, extra || {});
const labels = (ui) => ui.root.querySelectorAll('button').map((b) => b.textContent);

check('the board opens, accepts with Enter, shows progress and closes with Esc', () => {
    const { ui, log } = makeUi();
    const sim = newSim(1);
    assert.equal(ui.request('board', { sim }), true);
    assert.equal(ui.isOpen(), true);
    same(log.cues, ['ui_board']);
    assert.equal(ui.handleKey(key('Enter')), true, 'swallowed inside the guard window');
    assert.equal(flag(sim, 'bounty.id'), 0, 'the key that opened the window does not act');
    clock += 400;
    assert.equal(ui.root.querySelectorAll('button').length, 4, 'three offers and the leave button');
    ui.handleKey(key('ArrowDown'));
    ui.handleKey(key('Enter'));
    assert.equal(flag(sim, 'bounty.id'), 2, 'the second offer was taken');
    assert.ok(labels(ui).includes('runenpfad.bounty.drop'));
    assert.equal(log.flushes > 0, true);
    assert.equal(ui.handleKey(key('KeyP')), false, 'P falls through to the pause dialog');
    assert.equal(ui.handleKey(key('Escape')), true);
    assert.equal(ui.isOpen(), false);
    assert.equal(ui.handleKey(key('Escape')), false, 'a closed window consumes nothing');
});

check('the dice table plays a round through the keyboard and forwards its sounds', () => {
    const { ui, log } = makeUi();
    const sim = newSim(findSeed('win', 'gambler'));
    sim.profile.coins = 100;
    ui.request('dice:gambler', { sim });
    clock += 400;
    ui.handleKey(key('Digit1')); // first stake (5)
    const roll = ui.root.querySelectorAll('button').find((b) => b.textContent === 'runenpfad.dice.roll');
    roll.click();
    assert.ok(labels(ui).includes('runenpfad.dice.hold'));
    assert.ok(log.emitted.some((e) => e.type === 'dice_roll'));
    ui.root.querySelectorAll('button').find((b) => b.textContent === 'runenpfad.dice.hold').click();
    assert.ok(log.emitted.some((e) => e.type === 'dice_end'));
    assert.equal(sim.profile.coins, 105);
    assert.ok(labels(ui).includes('runenpfad.dice.again'));
    ui.handleKey(key('Escape'));
    assert.equal(ui.isOpen(), false);
});

check('an open dice round cannot be left; a forced close loses the stake once', () => {
    const { ui, log } = makeUi();
    const sim = newSim(findSeed('win', 'gambler'));
    sim.profile.coins = 100;
    ui.request('dice:gambler', { sim });
    clock += 400;
    ui.handleKey(key('Digit1'));
    ui.root.querySelectorAll('button').find((b) => b.textContent === 'runenpfad.dice.roll').click();
    assert.equal(ui.root.all().find((b) => b.className.includes('rp-games-leave')).disabled, true);
    assert.equal(ui.handleKey(key('Escape')), true);
    assert.equal(ui.isOpen(), true, 'Esc does nothing while the round is open');
    ui.close();
    assert.equal(ui.isOpen(), true);
    ui.reset();
    assert.equal(ui.isOpen(), false);
    assert.equal(sim.profile.coins, 95);
    assert.equal(sim.stage.dicePurse.gambler, 65);
    assert.ok(log.emitted.some((e) => e.type === 'dice_end' && e.detail.result === 'lose'));
});

check('abandon only acts in phase rolled and only once', () => {
    const sim = newSim(5);
    sim.profile.coins = 100;
    const g = Dice.create(sim, 'gambler');
    assert.equal(Dice.abandon(g), false, 'phase stake');
    Dice.roll(g, 10);
    assert.ok(Dice.abandon(g));
    assert.equal(g.phase, 'done');
    assert.equal(g.result, 'lose');
    assert.equal(sim.profile.coins, 90);
    assert.equal(sim.stage.dicePurse.gambler, 70);
    assert.equal(sim.events.filter((e) => e.type === 'dice_end').length, 1);
    assert.equal(Dice.abandon(g), false, 'phase done');
    assert.equal(sim.profile.coins, 90);
    assert.equal(sim.stage.dicePurse.gambler, 70);
});

check('songs set the music override and end it on reset', () => {
    const { ui } = makeUi();
    const calls = [];
    const music = { setOverride: (id) => calls.push(id) };
    assert.equal(ui.request('song:village', { sim: newSim(1), music }), true);
    same(calls, ['village']);
    assert.equal(ui.isOpen(), false);
    ui.reset();
    same(calls, ['village', null]);
    assert.equal(ui.request('bogus', { sim: newSim(1) }), false);
});

console.log(`runenpfad games: ${passed} checks passed`);
