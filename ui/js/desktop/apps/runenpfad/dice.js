/* Runenpfad rune dice: two dice each, one optional re-roll of one of the hero's dice, the higher total takes the stake.
 * DOM-free and deterministic (rolls come from Physics.rand(sim)). The state is a plain object that holds the sim.
 * Story writes (coins, flags) go through the RunenpfadNpcs helpers so sim.storyDirty is set. */
(function (root) {
    'use strict';

    const STAKES = [5, 10, 20];
    const PURSE = 60;          // opponent coins per stage run (sim.stage.dicePurse[opponentId])
    const OPPONENT_MIN = 7;    // the opponent re-rolls its lower die below this total

    function P() { return root.RunenpfadPhysics; }
    function N() { return root.RunenpfadNpcs || null; }

    function die(sim) { return 1 + Math.min(5, Math.floor(P().rand(sim) * 6)); }
    function sum(list) { return list[0] + list[1]; }
    function push(state, type, detail) { if (state.sim.events) P().pushEvent(state.sim, type, detail); }

    function purseTable(sim) {
        const stage = sim.stage || (sim.stage = {});
        if (!stage.dicePurse) stage.dicePurse = {};
        return stage.dicePurse;
    }

    function purse(state) {
        const table = purseTable(state.sim);
        if (typeof table[state.opponent] !== 'number') table[state.opponent] = PURSE;
        return table[state.opponent];
    }

    function coins(state) { return Math.max(0, Math.floor((state.sim.profile && state.sim.profile.coins) || 0)); }

    function create(sim, opponentId) {
        const state = { sim: sim, opponent: String(opponentId || ''), phase: 'stake', stake: 0, mine: [0, 0], theirs: [0, 0], result: '', delta: 0, rerolled: -1 };
        purse(state);
        return state;
    }

    // Stakes both sides can pay right now.
    function stakes(state) {
        const mine = coins(state);
        const theirs = purse(state);
        return STAKES.filter(function (s) { return mine >= s && theirs >= s; });
    }

    // 'ok', 'poor' (the hero cannot pay the smallest stake) or 'broke' (the opponent cannot).
    function status(state) {
        if (coins(state) < STAKES[0]) return 'poor';
        if (purse(state) < STAKES[0]) return 'broke';
        return 'ok';
    }

    function roll(state, stake) {
        if (state.phase !== 'stake' || stakes(state).indexOf(stake) < 0) return false;
        state.stake = stake;
        state.mine = [die(state.sim), die(state.sim)];
        state.theirs = [die(state.sim), die(state.sim)];
        state.result = '';
        state.delta = 0;
        state.rerolled = -1;
        state.phase = 'rolled';
        push(state, 'dice_roll', { stake: stake });
        return true;
    }

    function pay(state, n) {
        const Npcs = N();
        if (Npcs) Npcs.addCoins(state.sim, n);
    }

    function setFlag(state, key, value) {
        const Npcs = N();
        if (Npcs) Npcs.setFlag(state.sim, key, value);
    }

    function flag(state, key) {
        const Npcs = N();
        return Npcs ? Npcs.getFlag(state.sim, key) : 0;
    }

    // The opponent plays, then the round resolves.
    function finish(state) {
        const t = state.theirs;
        if (sum(t) < OPPONENT_MIN) {
            t[t[0] <= t[1] ? 0 : 1] = die(state.sim);
            push(state, 'dice_roll', { stake: state.stake });
        }
        const a = sum(state.mine);
        const b = sum(t);
        const table = purseTable(state.sim);
        state.result = a > b ? 'win' : a < b ? 'lose' : 'tie';
        if (state.result === 'win') {
            pay(state, state.stake);
            table[state.opponent] = purse(state) - state.stake;
            state.delta = state.stake;
            setFlag(state, 'dice.wins', Math.min(99, flag(state, 'dice.wins') + 1));
            if (state.opponent === 'barkeep') setFlag(state, 'barkeep.dice_won', 1);
        } else if (state.result === 'lose') {
            pay(state, -state.stake);
            table[state.opponent] = purse(state) + state.stake;
            state.delta = -state.stake;
        }
        state.phase = 'done';
        push(state, 'dice_end', { result: state.result });
        return state.result;
    }

    function reroll(state, dieIndex) {
        if (state.phase !== 'rolled' || (dieIndex !== 0 && dieIndex !== 1)) return false;
        state.mine[dieIndex] = die(state.sim);
        state.rerolled = dieIndex;
        push(state, 'dice_roll', { stake: state.stake });
        finish(state);
        return true;
    }

    function hold(state) {
        if (state.phase !== 'rolled') return false;
        finish(state);
        return true;
    }

    // Forced exit while a round is open (stage change, window closed): the round counts as a loss, once.
    function abandon(state) {
        if (state.phase !== 'rolled') return false;
        pay(state, -state.stake);
        const table = purseTable(state.sim);
        table[state.opponent] = purse(state) + state.stake;
        state.result = 'lose';
        state.delta = -state.stake;
        state.phase = 'done';
        push(state, 'dice_end', { result: 'lose' });
        return true;
    }

    // Back to the stake choice for another round.
    function next(state) {
        if (state.phase !== 'done') return false;
        state.phase = 'stake';
        state.stake = 0;
        state.rerolled = -1;
        return true;
    }

    root.RunenpfadDice = { STAKES: STAKES, PURSE: PURSE, create: create, stakes: stakes, status: status, roll: roll, reroll: reroll, hold: hold, abandon: abandon, next: next, purse: purse };
})(typeof window !== 'undefined' ? window : globalThis);
