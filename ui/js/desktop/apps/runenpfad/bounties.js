/* Runenpfad task board: eight tasks, one active at a time, progress counted from the events of each simulation call.
 * DOM-free and deterministic. State lives in npcFlags (bounty.id, bounty.n, bounty.won) and is written only through
 * the RunenpfadNpcs helpers, so sim.storyDirty is set and the shell saves it. */
(function (root) {
    'use strict';

    // `type` is the counted event; `min` (combo count) and `clean` (portal_win without damage) narrow it.
    const TABLE = [
        { id: 1, key: 'runenpfad.bounty.t1', type: 'in_mob_die', need: 8, reward: 30 },
        { id: 2, key: 'runenpfad.bounty.t2', type: 'in_break', need: 15, reward: 25 },
        { id: 3, key: 'runenpfad.bounty.t3', type: 'pickup_gem', need: 6, reward: 35 },
        { id: 4, key: 'runenpfad.bounty.t4', type: 'stomp', need: 10, reward: 30 },
        { id: 5, key: 'runenpfad.bounty.t5', type: 'combo', need: 1, reward: 40, min: 5 },
        { id: 6, key: 'runenpfad.bounty.t6', type: 'in_boss_die', need: 1, reward: 60 },
        { id: 7, key: 'runenpfad.bounty.t7', type: 'portal_win', need: 1, reward: 50, clean: true },
        { id: 8, key: 'runenpfad.bounty.t8', type: 'in_puzzle_done', need: 3, reward: 40 }
    ];
    const OFFER_COUNT = 3;

    function N() { return root.RunenpfadNpcs || null; }
    function P() { return root.RunenpfadPhysics; }

    function def(id) { return TABLE[id - 1] || null; }

    function read(profile, key) {
        const flags = profile && profile.npcFlags;
        return (flags && flags[key]) || 0;
    }

    // Three task ids that rotate with the number of completed tasks.
    function offers(profile) {
        const start = read(profile, 'bounty.won') % TABLE.length;
        const out = [];
        for (let i = 0; i < OFFER_COUNT; i++) out.push(TABLE[(start + i) % TABLE.length].id);
        return out;
    }

    // { id, def, n, need, done } for the active task, or null.
    function active(profile) {
        const d = def(read(profile, 'bounty.id'));
        if (!d) return null;
        const n = Math.min(d.need, read(profile, 'bounty.n'));
        return { id: d.id, def: d, n: n, need: d.need, done: n >= d.need };
    }

    function accept(sim, id) {
        const Npcs = N();
        if (!Npcs || read(sim.profile, 'bounty.id') || offers(sim.profile).indexOf(id) < 0) return false;
        Npcs.setFlag(sim, 'bounty.id', id);
        Npcs.setFlag(sim, 'bounty.n', 0);
        return true;
    }

    function drop(sim) {
        const Npcs = N();
        if (!Npcs || !read(sim.profile, 'bounty.id')) return false;
        Npcs.setFlag(sim, 'bounty.id', 0);
        Npcs.setFlag(sim, 'bounty.n', 0);
        return true;
    }

    // Pays the reward once; returns the coins paid (0 when nothing is ready).
    function claim(sim) {
        const Npcs = N();
        const a = active(sim.profile);
        if (!Npcs || !a || !a.done) return 0;
        Npcs.addCoins(sim, a.def.reward);
        Npcs.setFlag(sim, 'bounty.won', read(sim.profile, 'bounty.won') + 1);
        Npcs.setFlag(sim, 'bounty.id', 0);
        Npcs.setFlag(sim, 'bounty.n', 0);
        if (sim.events) P().pushEvent(sim, 'bounty_claim', { id: a.id, coins: a.def.reward });
        return a.def.reward;
    }

    function counts(d, ev, sim) {
        if (ev.type !== d.type) return false;
        if (d.min) return !!(ev.detail && ev.detail.count >= d.min);
        if (d.clean) return sim.damageTaken === 0;
        return true;
    }

    // Counts the events of one simulation call. Iterates over a snapshot of the length: bounty_done lands in the same array.
    function track(sim, events) {
        const Npcs = N();
        const a = sim && Npcs && events ? active(sim.profile) : null;
        if (!a || a.done) return;
        let n = a.n;
        const end = events.length;
        for (let i = 0; i < end && n < a.need; i++) if (counts(a.def, events[i], sim)) n += 1;
        if (n === a.n) return;
        Npcs.setFlag(sim, 'bounty.n', n);
        if (n >= a.need) {
            P().pushEvent(sim, 'bounty_done', { id: a.id });
            sim.notice = 'runenpfad.notice.bounty_done';
        }
    }

    root.RunenpfadBounties = { TABLE: TABLE, def: def, offers: offers, active: active, accept: accept, drop: drop, claim: claim, track: track };
})(typeof window !== 'undefined' ? window : globalThis);
