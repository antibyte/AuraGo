import { tableOf, clamp } from './tables.js';

const POINTS = { bumper: 150, sling: 25, target: 750, ramp: 3000, orbit: 2000, upper: 4000, spinner: 100, magnet: 2500, skill: 10000, lock: 5000 };
const player = name => ({ name, score: 0, played: 0, bonus: 0, locks: 0, bank: [], progress: [0, 0, 0], completed: [false, false, false], lastRamp: '', finale: false, jackpots: 0, tilt: false, warnings: 0 });

export function createRound(tableId, names, training = false) {
    const players = (Array.isArray(names) ? names : []).slice(0, training ? 1 : 4).map((name, i) => player(Array.from(String(name || i + 1)).slice(0, 24).join('')));
    if (!players.length) players.push(player('1'));
    const game = { table: tableOf(tableId).id, players, turn: 0, training: !!training, phase: 'playing', time: 0, serial: 0, active: [], actions: [], multiball: false, saverUntil: 0, launched: false, firstHit: false, bonusUntil: 0, tiltEnergy: 0, nextWarning: 0, ballSearchAt: 15, notice: 'shoot', noticeSerial: 0 };
    feed(game, false);
    return game;
}
export const currentPlayer = game => game.players[game.turn];
export const gateOpen = game => !currentPlayer(game).tilt && (currentPlayer(game).bank.length === 3 || game.multiball);
function notice(game, key) { game.notice = key; game.noticeSerial++; }
function feed(game, auto) { const id = ++game.serial; game.active.push(id); game.actions.push({ type: 'feed', id, auto }); }
function score(game, value) { const p = currentPlayer(game); p.score = Math.min(Number.MAX_SAFE_INTEGER, p.score + value); }
export function takeActions(game) { return game.actions.splice(0); }

export function tickRound(game, time) {
    const dt = clamp(time - game.time, 0, 1);
    game.time = time;
    game.tiltEnergy = Math.max(0, game.tiltEnergy - dt * .7);
    if (game.phase !== 'bonus' || time < game.bonusUntil) return;
    const old = currentPlayer(game);
    if (game.training) { resetTurn(game); feed(game, false); return; }
    old.played++;
    if (game.players.every(p => p.played >= 3)) { game.phase = 'finished'; notice(game, 'game_over'); return; }
    do game.turn = (game.turn + 1) % game.players.length; while (currentPlayer(game).played >= 3);
    resetTurn(game); feed(game, false);
}
function resetTurn(game) {
    game.phase = 'playing'; game.multiball = false; game.launched = false; game.firstHit = false; game.saverUntil = 0; game.tiltEnergy = 0; game.nextWarning = 0;
    const p = currentPlayer(game); p.bonus = 0; p.tilt = false; p.warnings = 0;
    notice(game, 'shoot');
}

export function nudgeRound(game, strength = 1) {
    if (game.phase !== 'playing' || !game.launched || currentPlayer(game).tilt) return false;
    game.tiltEnergy += clamp(strength, 0, 1) * .9;
    if (game.tiltEnergy < 1.5 || game.time < game.nextWarning) return true;
    game.nextWarning = game.time + 1;
    const p = currentPlayer(game); p.warnings++;
    if (p.warnings >= 3) { p.tilt = true; p.bonus = 0; game.saverUntil = 0; game.actions.push({ type: 'coils', enabled: false }); notice(game, 'tilt'); }
    else notice(game, 'warning');
    return !p.tilt;
}

export function handleEvent(game, event) {
    if (game.phase !== 'playing' || !event || !game.active.includes(event.ball)) return;
    const p = currentPlayer(game);
    if (event.type === 'launch') {
        if (!game.launched) { game.launched = true; game.saverUntil = game.time + 10; notice(game, 'ball_saver'); }
        return;
    }
    if (event.type === 'drain') {
        game.active.splice(game.active.indexOf(event.ball), 1);
        if (game.training && p.tilt && !game.active.length) resetTurn(game);
        if (game.training && !currentPlayer(game).tilt || !p.tilt && (game.time < game.saverUntil || event.id === 'search')) { feed(game, true); notice(game, game.training ? 'training' : 'saved'); }
        if (game.active.length <= 1) game.multiball = false;
        if (!game.active.length) {
            game.phase = 'bonus'; game.bonusUntil = game.time + 1.5;
            score(game, p.tilt ? 0 : p.bonus); notice(game, 'bonus');
        }
        return;
    }
    if (p.tilt || !Object.hasOwn(POINTS, event.type)) return;
    if (event.type === 'lock' && !gateOpen(game)) { game.actions.push({ type: 'eject', id: event.ball }); return; }
    if (event.type === 'target') {
        if (p.bank.includes(event.id)) return;
        p.bank.push(event.id);
    }
    const first = !game.firstHit;
    game.firstHit = true;
    score(game, event.type === 'skill' ? (first ? POINTS.skill : 500) : POINTS[event.type]);
    p.bonus = Math.min(50000, p.bonus + (event.type === 'bumper' || event.type === 'sling' ? 25 : 250));
    if (event.type === 'skill' && first) notice(game, 'skillshot');
    const table = tableOf(game.table);
    table.goals.forEach((goal, index) => {
        if (p.completed[index] || goal.type !== event.type || goal.alternate && p.lastRamp === event.id) return;
        p.progress[index]++;
        if (p.progress[index] >= goal.count) { p.completed[index] = true; score(game, 15000); notice(game, 'mission_complete'); }
    });
    if (event.type === 'ramp') p.lastRamp = event.id;
    if (p.completed.every(Boolean) && !p.finale) { p.finale = true; notice(game, 'finale_ready'); }
    if ((game.multiball || p.finale) && ['ramp', 'orbit', 'upper'].includes(event.type)) { p.jackpots++; score(game, p.finale ? 50000 : 15000); notice(game, 'jackpot'); }
    if (event.type === 'lock') {
        game.actions.push({ type: 'eject', id: event.ball });
        if (game.multiball) return;
        p.locks++; p.bank = [];
        game.actions.push({ type: 'reset_targets' });
        if (p.locks >= 3) {
            p.locks = 0; game.multiball = true;
            while (game.active.length < 3) feed(game, true);
            game.saverUntil = Math.max(game.saverUntil, game.time + 10);
            notice(game, 'multiball');
        } else notice(game, 'locked');
    }
}
