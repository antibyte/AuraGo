// Rules framework, part 5: high-score qualification and initials entry. DOM-free.
import { dmd, dmdClear, cue } from './features.js';

export const ALPHABET = Object.freeze([...'ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789', ' ', '←']);
export const INITIALS_SECONDS = 45;
export const INITIALS_LENGTH = 3;
const BACK = ALPHABET.length - 1;

// 1-based rank the score would take in a top-10 list (null if it does not qualify).
export function rankOf(scores, score) {
    if (!(score > 0)) return null;
    const list = Array.isArray(scores) ? scores : [];
    let rank = 1;
    for (const row of list) if (row.score >= score) rank++;
    return rank <= 10 ? rank : null;
}

function scene(g) {
    const e = g.initials;
    const letters = Array.from({ length: INITIALS_LENGTH }, (_, i) => e.letters[i] || '_');
    dmd(g, 'initials', {
        rank: e.rank, letters, cursor: Math.min(e.letters.length, INITIALS_LENGTH - 1), char: ALPHABET[e.cur],
        remaining: Math.max(0, e.until - g.time), total: INITIALS_SECONDS
    }, 0, 5);
}

// Takes the next qualifying player off the queue; returns false when nobody is left.
export function nextInitials(g) {
    const e = g.initials;
    while (e.queue.length) {
        const idx = e.queue.shift();
        const rank = rankOf(g.scores, g.players[idx].score);
        if (rank === null) continue;
        e.player = idx; e.letters = ''; e.cur = 0; e.rank = rank; e.until = g.time + INITIALS_SECONDS;
        cue(g, 'highScore');
        scene(g);
        return true;
    }
    e.player = -1;
    dmdClear(g);
    return false;
}

function finish(g) {
    const e = g.initials, p = g.players[e.player];
    const name = e.letters.trimEnd() || (Array.from(p.name.toUpperCase()).filter(c => ALPHABET.includes(c)).slice(0, INITIALS_LENGTH).join('') || 'AAA');
    g.results.push({ name, score: p.score });
    g.scores.push({ name, score: p.score, date: 0 });
    g.scores.sort((a, b) => b.score - a.score);
    g.scores.length = Math.min(10, g.scores.length);
    return nextInitials(g);
}

// Returns false when the last entry finished (the game is over).
export function initialsButton(g, name) {
    const e = g.initials;
    if (e.player < 0) return true;
    if (name === 'left' || name === 'right') {
        e.cur = (e.cur + (name === 'right' ? 1 : -1) + ALPHABET.length) % ALPHABET.length;
    } else if (name === 'start' || name === 'launch') {
        if (e.cur === BACK) e.letters = e.letters.slice(0, -1);
        else e.letters += ALPHABET[e.cur];
        if (e.letters.length >= INITIALS_LENGTH) return finish(g);
    } else return true;
    e.until = g.time + INITIALS_SECONDS;
    scene(g);
    return true;
}

export function tickInitials(g) {
    const e = g.initials;
    return e.player >= 0 && g.time >= e.until ? finish(g) : true;
}
