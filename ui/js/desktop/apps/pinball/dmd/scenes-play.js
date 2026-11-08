// AuraPinball DMD base scenes (score, modeStatus, attract) and game-flow screens (select, bonus, match, initials,
// gameOver). Pure functions of (draw API, params, t); persistent animation state lives in d.mem. DOM-free.
import { ease, countUp, zoomWord, STAR, burst } from './scenes-events.js';

const sparkSeed = (t, salt = 7) => Math.floor(t * 12) * 131 + salt;
const num = (v, fallback = 0) => (Number.isFinite(v) ? v : fallback);
const ATTRACT_PAGE_S = 4;

// ----- base scenes -----
// score params: {players: [{name, score}], turn, ball, ballsTotal, training}
function drawScore(d, p) {
    const players = Array.isArray(p.players) && p.players.length ? p.players : [{ name: 'P1', score: 0 }];
    const turn = Math.max(0, Math.min(players.length - 1, num(p.turn)));
    const target = Math.max(0, Math.floor(num(players[turn].score)));
    const m = d.mem;
    if (m.shown === undefined || m.turn !== turn) { m.shown = target; m.flash = 0; m.turn = turn; m.target = target; }
    if (target !== m.target) { m.flash = target > m.target ? 0.5 : 0; m.target = target; }
    if (m.shown !== target) {
        const gap = target - m.shown, stepSize = Math.max(10, Math.ceil(Math.abs(gap) * Math.min(1, d.dt * 9) / 10) * 10);
        m.shown = Math.abs(gap) <= stepSize ? target : m.shown + Math.sign(gap) * stepSize;
    }
    m.flash = Math.max(0, m.flash - d.dt);

    d.text('dmd_player', 2, 0, { size: 's', vars: { n: turn + 1 }, maxWidth: 90 });
    if (p.training) d.text('dmd_training', 190, 0, { size: 's', align: 'right', maxWidth: 90 });
    else d.text('dmd_ball', 190, 0, { size: 's', align: 'right', vars: { n: num(p.ball, 1) }, maxWidth: 90 });
    const level = m.flash > 0 && Math.floor(m.flash * 20) % 2 ? 2 : 3;
    d.number(m.shown, 96, 14, { font: 'l', align: 'center', level });

    const others = players.map((pl, i) => ({ pl, i })).filter(e => e.i !== turn);
    if (others.length) {
        const slot = 192 / others.length;
        others.forEach((e, k) => {
            const x = Math.round(k * slot) + 2;
            d.text('P' + (e.i + 1), x, 37, { size: 's', maxWidth: 20 });
            d.number(num(e.pl.score), Math.round((k + 1) * slot) - 3, 40, { font: 's', align: 'right' });
        });
    } else if (!p.training) {
        const total = num(p.ballsTotal, 3), left = total - num(p.ball, 1) + 1;
        for (let i = 0; i < total; i++) d.circle(96 + (i - (total - 1) / 2) * 12, 41, 3, i < left ? 3 : 1, i < left);
    }
}

// modeStatus params: {name, remaining, total, count, target, score, hint}
function drawModeStatus(d, p, t) {
    d.text(p.name || '', 96, 0, { size: 'm', align: 'center', maxWidth: 186, vars: p.vars });
    const frac = p.target ? num(p.count) / p.target : (p.total ? num(p.remaining) / p.total : 0);
    if (Number.isFinite(p.remaining)) d.number(Math.ceil(p.remaining), 2, 17, { font: 'm', sep: '' });
    const barX = Number.isFinite(p.remaining) ? 30 : 4;
    d.progress(barX, 18, 188 - barX, 8, frac, p.remaining < 5 && Math.floor(t * 6) % 2 ? 2 : 3);
    if (p.target) d.text('dmd_to_go', 4, 28, { size: 's', vars: { n: Math.max(0, p.target - num(p.count)) }, maxWidth: 80 });
    else if (p.hint) d.text(p.hint, 4, 28, { size: 's', maxWidth: 110 });
    if (Number.isFinite(p.score)) d.number(p.score, 188, 34, { font: 'm', align: 'right' });
}

function rows(d, lines, startY, pitch) {
    lines.forEach((row, i) => {
        const y = startY + i * pitch;
        d.text(row.label, 4, y, { size: 's', maxWidth: 110 });
        d.number(num(row.score), 188, y + 2, { font: 's', align: 'right' });
    });
}

// attract params: {title, scores: [{name, score}], replayAt, last: [{name, score}], free}
function attractPages(p) {
    const pages = ['title'];
    if (Array.isArray(p.scores) && p.scores.length) pages.push('scores');
    pages.push('start');
    if (num(p.replayAt) > 0) pages.push('replay');
    if (Array.isArray(p.last) && p.last.length) pages.push('last');
    return pages;
}

function drawAttract(d, p, t) {
    const pages = attractPages(p), index = Math.floor(t / ATTRACT_PAGE_S) % pages.length, pt = t % ATTRACT_PAGE_S;
    const slide = Math.round((1 - ease(pt / 0.35)) * 192), name = pages[index];
    if (name === 'title') {
        d.sparkle(16, sparkSeed(pt, 21));
        zoomWord(d, p.title || 'AuraPinball', pt, p.free === false ? 24 : 18, 'xl');
        if (p.free !== false) d.text('dmd_free_play', 96 + slide, 36, { size: 's', align: 'center' });
    } else if (name === 'scores') {
        d.text('dmd_high_scores', 96 + slide, 0, { size: 's', align: 'center' });
        const top = p.scores.slice(0, 3).map((e, i) => ({ label: (i + 1) + '. ' + String(e.name || '---'), score: e.score }));
        rows(d, top, 12, 11);
    } else if (name === 'start') {
        const on = Math.floor(pt * 2.5) % 2 === 0;
        if (on) d.text('dmd_press_start', 96, 24, { size: 'l', align: 'center', valign: 'middle', maxWidth: 164 });
        const w = Math.round((pt / ATTRACT_PAGE_S) * 192);
        d.rect(0, 0, w, 1, 2); d.rect(192 - w, 47, w, 1, 2);
        d.sprite(STAR, 12, 24, 5, pt * 2, on ? 3 : 1, false); d.sprite(STAR, 180, 24, 5, -pt * 2, on ? 3 : 1, false);
    } else if (name === 'replay') {
        d.text('dmd_replay_at', 96, 4, { size: 'm', align: 'center', maxWidth: 184 });
        d.number(countUp(p.replayAt, pt, 0.7), 96 + slide, 22, { font: 'l', align: 'center' });
    } else {
        d.text('dmd_game_over', 96, 0, { size: 's', align: 'center' });
        const last = p.last.slice(0, 4);
        if (last.length === 1) d.number(num(last[0].score), 96 + slide, 18, { font: 'l', align: 'center' });
        else rows(d, last.map((e, i) => ({ label: (i + 1) + '. ' + String(e.name || 'P' + (i + 1)), score: e.score })), 11, 9);
    }
}

// ----- game-flow screens -----
// select params: {items: [{key|name, played, done}], index, remaining, total}
function drawSelect(d, p, t) {
    const items = Array.isArray(p.items) ? p.items : [], index = Math.max(0, Math.min(items.length - 1, num(p.index)));
    const m = d.mem;
    if (m.index !== index) { m.index = index; m.at = t; }
    const slide = Math.round((1 - ease((t - m.at) / 0.18)) * 36);
    d.text('dmd_select_mode', 96, 0, { size: 's', align: 'center', maxWidth: 184 });
    const item = items[index];
    if (item) d.text(item.key || item.name || '', 96 + slide, 15, { size: 'm', align: 'center', maxWidth: 150, vars: item.vars });
    const blink = Math.floor(t * 5) % 2 ? 3 : 1;
    d.sprite([[-4, -5], [3, 0], [-4, 5], [-4, -5]], 184, 21, 1, 0, blink, true);
    d.sprite([[4, -5], [-3, 0], [4, 5], [4, -5]], 8, 21, 1, 0, blink, true);
    items.forEach((it, i) => {
        const x = 96 + (i - (items.length - 1) / 2) * 14, level = i === index ? 3 : it.done ? 2 : it.played ? 1 : 2;
        d.circle(x, 36, i === index ? 4 : 2, i === index && !blink ? 1 : level, i === index || it.done);
    });
    if (p.total > 0) d.progress(4, 44, 184, 4, num(p.remaining) / p.total, 3);
}

// bonus params: {phase: 'line'|'mult'|'total', key, count, value, x, subtotal, total}
function drawBonus(d, p, t) {
    const m = d.mem, id = (p.phase || 'line') + '|' + (p.key || '') + '|' + num(p.x);
    if (m.id !== id) { m.id = id; m.at = t; }
    const age = t - m.at;
    if (p.phase === 'total') {
        d.text('dmd_total_bonus', 96, 2, { size: 'm', align: 'center', maxWidth: 184 });
        d.number(countUp(p.total, age, 0.9), 96, 22, { font: 'l', align: 'center' });
        if (age > 1.0) d.sparkle(12, sparkSeed(age, 31));
    } else if (p.phase === 'mult') {
        zoomWord(d, 'dmd_bonus_x', age, 18, 'xl', { vars: { n: num(p.x, 1) } });
        d.number(num(p.subtotal), 96, 36, { font: 'm', align: 'center' });
    } else {
        d.text('dmd_bonus', 96, 0, { size: 's', align: 'center' });
        const pop = Math.floor(age * 16) % 2 === 0 && age < 0.2 ? 2 : 3;
        const slide = Math.round((1 - ease(age / 0.15)) * 40);
        d.text(p.key || '', 4 - slide, 16, { size: 'm', maxWidth: 80, level: pop });
        if (Number.isFinite(p.count)) d.number(p.count, 114, 19, { font: 'm', align: 'right', level: pop });
        d.number(num(p.value), 188 + slide, 19, { font: 'm', align: 'right', level: pop });
        if (Number.isFinite(p.subtotal)) d.number(p.subtotal, 188, 38, { font: 's', align: 'right', level: 2 });
    }
}

// match params: {match: 0..90 (tens), last: 0..99, hit}
function drawMatch(d, p, t) {
    d.text('dmd_match', 96, 0, { size: 's', align: 'center' });
    const settle = 1.3, tens = Math.round(num(p.match) / 10) * 10;
    const shown = t < settle ? (Math.floor(t * 28) % 10) * 10 : tens;
    d.number(shown, 54, 18, { font: 'l', align: 'center', pad: 2, sep: '', level: t < settle ? 2 : 3 });
    d.rect(88, 23, 16, 2, 2); d.rect(88, 29, 16, 2, 2);
    d.number(num(p.last), 138, 18, { font: 'l', align: 'center', pad: 2, sep: '' });
    d.rect(30, 38, 48, 1, 1); d.rect(114, 38, 48, 1, 1);
    if (p.hit && t > settle) {
        d.sparkle(24, sparkSeed(t, 41));
        if (t < settle + 0.5 && Math.floor(t * 14) % 2 === 0) d.invert();
    }
}

// initials params: {rank, letters: ['A', 'B', '_'], cursor, remaining, total}
function drawInitials(d, p, t) {
    const letters = Array.isArray(p.letters) && p.letters.length ? p.letters.slice(0, 3) : ['A', 'A', 'A'];
    const cursor = Math.max(0, Math.min(letters.length - 1, num(p.cursor)));
    if (Math.floor(t / 1.6) % 2 === 0 && p.rank) d.text('dmd_high_score', 96, 0, { size: 's', align: 'center', vars: { n: p.rank } });
    else d.text('dmd_enter_initials', 96, 0, { size: 's', align: 'center', maxWidth: 184 });
    const on = Math.floor(t * 3.5) % 2 === 0;
    letters.forEach((ch, i) => {
        const x = 96 + (i - (letters.length - 1) / 2) * 30;
        const active = i === cursor, level = active && !on ? 1 : 3;
        d.text(String(ch), x, 22, { size: 'xl', align: 'center', valign: 'middle', level, maxWidth: 24 });
        d.rect(x - 10, 40, 20, 2, active ? (on ? 3 : 1) : 2);
    });
    if (p.total > 0) d.rect(0, 46, Math.round(192 * Math.max(0, Math.min(1, num(p.remaining) / p.total))), 2, 2);
}

function drawGameOver(d, p, t) {
    const scan = Math.floor((t * 90) % 192);
    d.line(scan, 0, scan, 47, 1);
    zoomWord(d, 'dmd_game_over', t, 24, 'xl', { level: Math.floor(t * 4) % 6 === 5 ? 2 : 3 });
    if (t < 0.6) burst(d, 96, 24, t, { rays: 16, rMax: 50, speed: 90 });
}

export const PLAY_SCENES = {
    score: { transition: 'dissolve', draw: drawScore },
    modeStatus: { transition: 'wipeLeft', draw: drawModeStatus },
    attract: { transition: 'dissolve', draw: drawAttract },
    select: { ms: Infinity, priority: 5, transition: 'wipeRight', live: true, draw: drawSelect },
    bonus: { ms: Infinity, priority: 5, transition: 'cut', live: true, draw: drawBonus },
    match: { ms: 3600, priority: 5, transition: 'wipeUp', draw: drawMatch },
    initials: { ms: Infinity, priority: 5, transition: 'dissolve', live: true, draw: drawInitials },
    gameOver: { ms: 3500, priority: 5, transition: 'dissolve', draw: drawGameOver }
};
