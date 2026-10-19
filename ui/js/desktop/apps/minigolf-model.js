import { COLOR_IDS, DEFAULT_COLORS, MAX_NAME_LENGTH, WORLD_ID } from './minigolf-shared.js';

const MAX_PLAYERS = 4;
const MAX_COURSES = 100;
const MAX_SAVE_BYTES = 64 * 1024;
const activeShots = new WeakSet();
const CONTROL = /[\x00-\x1f\x7f-\x9f]/;
// full, practice or a world id from the manifest; the game decides which worlds exist.
const validMode = mode => typeof mode === 'string' && WORLD_ID.test(mode);
const validName = name => typeof name === 'string' && name.trim() !== '' && Array.from(name).length <= MAX_NAME_LENGTH && !CONTROL.test(name);

const isObject = value => !!value && typeof value === 'object' && !Array.isArray(value) && (Object.getPrototypeOf(value) === Object.prototype || Object.getPrototypeOf(value) === null);
const finitePosition = value => Array.isArray(value) && value.length === 3 && value.every(number => Number.isFinite(number) && Math.abs(number) <= 100000);
const safeKeys = (value, expected) => isObject(value) && Object.keys(value).every(key => !['__proto__', 'prototype', 'constructor'].includes(key) && expected.includes(key)) && expected.every(key => Object.hasOwn(value, key));

function courseIndex(courses) {
    if (!Array.isArray(courses) || !courses.length || courses.length > MAX_COURSES) throw new TypeError('Minigolf needs between 1 and 100 courses');
    const byId = new Map();
    for (const course of courses) {
        if (!isObject(course) || typeof course.id !== 'string' || !/^[A-Za-z0-9_-]{1,96}$/.test(course.id) || byId.has(course.id) || !finitePosition(course.tee) || !finitePosition(course.cup)) throw new TypeError('Invalid or duplicate minigolf course');
        byId.set(course.id, course);
    }
    return byId;
}

function nextPlayer(round, from = round.turnIndex) {
    for (let offset = 1; offset <= round.players.length; offset++) {
        const index = (from + offset) % round.players.length;
        if (!round.players[index].done) return index;
    }
    return from;
}

export function createRound(courses, { players = ['Player 1'], mode = 'full', holeId } = {}) {
    const byId = courseIndex(courses);
    if (!validMode(mode)) throw new TypeError('Invalid minigolf mode');
    if (!Array.isArray(players) || players.length < 1 || players.length > MAX_PLAYERS) throw new TypeError('Minigolf supports 1 to 4 players');
    // Players are names (default colour by position) or { name, color }.
    const entries = players.map((player, index) => typeof player === 'string'
        ? { name: player.trim(), color: DEFAULT_COLORS[index] }
        : { name: typeof player?.name === 'string' ? player.name.trim() : '', color: player?.color });
    if (entries.some(entry => !validName(entry.name)) || new Set(entries.map(entry => entry.name.toLocaleLowerCase())).size !== entries.length) throw new TypeError('Invalid or duplicate minigolf player name');
    if (entries.some(entry => !COLOR_IDS.includes(entry.color)) || new Set(entries.map(entry => entry.color)).size !== entries.length) throw new TypeError('Invalid or duplicate minigolf player color');
    if (holeId !== undefined && (typeof holeId !== 'string' || !byId.has(holeId))) throw new TypeError('Unknown minigolf course');
    const courseIds = holeId === undefined ? Array.from(byId.keys()) : [holeId];
    const tee = byId.get(courseIds[0]).tee.slice();
    return {
        version: 2, mode, courseIds, holeIndex: 0,
        players: entries.map(({ name, color }) => ({ name, color, scores: [], position: tee.slice(), shots: 0, strokes: 0, penalties: 0, done: false })),
        turnIndex: 0, completed: false
    };
}

export function beginShot(round) {
    if (!round || round.completed || activeShots.has(round)) return null;
    let index = round.turnIndex;
    if (round.players[index]?.done) index = nextPlayer(round, index);
    if (!round.players[index] || round.players[index].done) return null;
    round.turnIndex = index;
    activeShots.add(round);
    const player = round.players[index];
    return { playerIndex: index, player: player.name, position: player.position.slice(), shots: player.shots, strokes: player.strokes };
}

export function resolveShot(round, result, course) {
    if (!round || !activeShots.has(round) || !result || !['rest', 'cup', 'water', 'out'].includes(result.type) || !finitePosition(result.position)) return false;
    const player = round.players[round.turnIndex];
    if (!player || player.done || !course || course.id !== round.courseIds[round.holeIndex]) return false;
    activeShots.delete(round);
    player.shots++;
    player.strokes = Math.min(13, player.strokes + 1);
    if (result.type === 'water' || result.type === 'out') {
        player.penalties++;
        player.strokes = Math.min(13, player.strokes + 1);
    } else {
        player.position = result.position.slice();
        if (result.type === 'cup') player.done = true;
    }
    if (!player.done && player.shots >= 12) {
        player.strokes = 13;
        player.done = true;
    }
    round.turnIndex = nextPlayer(round, round.turnIndex);
    return true;
}

export function advanceHole(round, courses) {
    if (!round || round.completed || !round.players?.length || round.players.some(player => !player.done)) return false;
    const byId = courseIndex(courses);
    const nextIndex = round.holeIndex + 1;
    const nextCourse = nextIndex < round.courseIds.length ? byId.get(round.courseIds[nextIndex]) : null;
    if (nextIndex < round.courseIds.length && !nextCourse) throw new TypeError('Minigolf save references an unknown course');
    for (const player of round.players) {
        player.scores.push(Math.min(13, Math.max(1, player.strokes)));
        player.shots = 0;
        player.strokes = 0;
        player.penalties = 0;
        player.done = false;
    }
    if (nextIndex >= round.courseIds.length) {
        round.holeIndex = round.courseIds.length;
        round.completed = true;
        round.players.forEach(player => { player.done = true; });
        return true;
    }
    round.holeIndex = nextIndex;
    round.turnIndex = 0;
    for (const player of round.players) player.position = nextCourse.tee.slice();
    return true;
}

export function validateSave(data, courses) {
    try {
        if (new TextEncoder().encode(JSON.stringify(data)).byteLength > MAX_SAVE_BYTES) return false;
        if (!safeKeys(data, ['version', 'mode', 'courseIds', 'holeIndex', 'players', 'turnIndex', 'completed']) || data.version !== 2 || !validMode(data.mode) || !Array.isArray(data.courseIds) || data.courseIds.length < 1 || data.courseIds.length > MAX_COURSES || !Array.isArray(data.players) || data.players.length < 1 || data.players.length > MAX_PLAYERS) return false;
        const byId = courseIndex(courses);
        if (data.courseIds.some(id => typeof id !== 'string' || !byId.has(id)) || new Set(data.courseIds).size !== data.courseIds.length) return false;
        if (!Number.isInteger(data.holeIndex) || data.holeIndex < 0 || data.holeIndex > data.courseIds.length || data.completed !== (data.holeIndex === data.courseIds.length) || !Number.isInteger(data.turnIndex) || data.turnIndex < 0 || data.turnIndex >= data.players.length) return false;
        const names = new Set(), colors = new Set();
        for (const player of data.players) {
            if (!safeKeys(player, ['name', 'color', 'scores', 'position', 'shots', 'strokes', 'penalties', 'done']) || !validName(player.name) || names.has(player.name.toLocaleLowerCase()) || !COLOR_IDS.includes(player.color) || colors.has(player.color) || !finitePosition(player.position) || !Array.isArray(player.scores) || player.scores.length !== data.holeIndex || player.scores.some(score => !Number.isInteger(score) || score < 1 || score > 13) || !Number.isInteger(player.shots) || player.shots < 0 || player.shots > 12 || !Number.isInteger(player.strokes) || player.strokes < 0 || player.strokes > 13 || !Number.isInteger(player.penalties) || player.penalties < 0 || player.penalties > 12 || typeof player.done !== 'boolean') return false;
            names.add(player.name.toLocaleLowerCase());
            colors.add(player.color);
            if (data.completed ? !player.done || player.strokes !== 0 || player.shots !== 0 : player.done && player.shots === 0) return false;
        }
        return true;
    } catch (_) {
        return false;
    }
}

export function cloneSave(round) {
    try {
        return {
            version: round.version,
            mode: round.mode,
            courseIds: round.courseIds.slice(),
            holeIndex: round.holeIndex,
            players: round.players.map(player => ({ name: player.name, color: player.color, scores: player.scores.slice(), position: player.position.slice(), shots: player.shots, strokes: player.strokes, penalties: player.penalties, done: player.done })),
            turnIndex: round.turnIndex,
            completed: round.completed
        };
    } catch (_) {
        return null;
    }
}

export function scoreTotal(player) {
    if (!player || !Array.isArray(player.scores)) return 0;
    const completed = player.scores.reduce((total, score) => total + (Number.isInteger(score) && score > 0 ? score : 0), 0);
    return completed + (Number.isInteger(player.strokes) ? Math.max(0, player.strokes) : 0);
}

// Bests v2: rounds per mode (outside practice), holes per course (any mode).
// `set` pins the manifest; a changed course set drops round bests only.
const courseSet = courses => courses.map(course => course.id).sort().join(',');
export function normalizeBest(raw, courses) {
    const set = courseSet(courses), ids = new Set(courses.map(course => course.id));
    const worlds = new Set(courses.map(course => course.world).filter(validMode));
    const best = { version: 2, set, rounds: {}, holes: {} };
    if (!isObject(raw) || raw.version !== 2) return best;
    if (raw.set === set && isObject(raw.rounds)) for (const [mode, value] of Object.entries(raw.rounds)) {
        if ((mode === 'full' || worlds.has(mode)) && Number.isInteger(value) && value >= 1 && value <= 13 * courses.length) best.rounds[mode] = value;
    }
    if (isObject(raw.holes)) for (const [id, value] of Object.entries(raw.holes)) {
        if (ids.has(id) && Number.isInteger(value) && value >= 1 && value <= 13) best.holes[id] = value;
    }
    return best;
}

export function recordHole(best, round) {
    const id = round?.courseIds?.[round.holeIndex];
    if (!best?.holes || !id || !round.players.every(player => player.done)) return false;
    const lowest = Math.min(...round.players.map(player => Math.min(13, Math.max(1, player.strokes))));
    if (best.holes[id] && best.holes[id] <= lowest) return false;
    best.holes[id] = lowest;
    return true;
}

export function recordRound(best, round) {
    if (!best?.rounds || !round?.completed || round.mode === 'practice') return false;
    const lowest = Math.min(...round.players.map(scoreTotal));
    if (best.rounds[round.mode] && best.rounds[round.mode] <= lowest) return false;
    best.rounds[round.mode] = lowest;
    return true;
}
