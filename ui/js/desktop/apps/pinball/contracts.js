// Shared vocabulary between physics, rules, lights, DMD and audio. DOM-free.
export const EVENT_TYPES = Object.freeze([
    'launch', 'drain', 'flipper', 'bumper', 'sling', 'drop', 'bankDown', 'bankReset', 'standup', 'spinner',
    'rollover', 'trigger', 'gate', 'hole', 'eject', 'kickback', 'magnet', 'captive',
    'pathEnter', 'pathSwitch', 'pathMade', 'pathFail', 'impact', 'clack', 'search', 'searchFail', 'shooter'
]);

const NOT_SCORING = new Set(['impact', 'clack', 'flipper', 'launch', 'search', 'shooter', 'eject', 'pathEnter', 'magnet']);
export const SCORING_EVENTS = Object.freeze(EVENT_TYPES.filter(type => !NOT_SCORING.has(type)));

export const PHYSICS_COMMANDS = Object.freeze([
    'addBall', 'coils', 'eject', 'lockBall', 'releaseToTrough', 'holeEnable', 'resetBank', 'dropBank',
    'diverter', 'gate', 'door', 'kickback', 'magnet', 'kick', 'nudge', 'removeBall', 'clearBalls'
]);
export const PRESENTATION_COMMANDS = Object.freeze(['dmd', 'dmdBase', 'show', 'flash', 'gi', 'cue', 'music', 'say', 'shake', 'lampColor']);

// Lamp mode codes (index = code).
export const LAMP_MODES = Object.freeze({ off: 0, on: 1, slow: 2, fast: 3, pulse: 4, flicker: 5, dim: 6 });

export const CUES = Object.freeze([
    'jackpot', 'superJackpot', 'lock', 'lockLit', 'multiball', 'extraBall', 'replay', 'modeStart', 'modeComplete',
    'skillShot', 'ballSaved', 'drain', 'gameOver', 'highScore', 'tiltWarning', 'tilt', 'combo', 'bonusTick',
    'bonusTotal', 'match', 'knocker', 'startGame', 'shootAgain', 'wizard'
]);
export const MUSIC_SECTIONS = Object.freeze(['off', 'attract', 'main', 'mode', 'multiball', 'wizard', 'bonus', 'hiscore']);
export const SHOWS = Object.freeze([
    'sweepUp', 'sweepDown', 'sweepLeft', 'sweepRight', 'radial', 'ring', 'chase', 'strobe', 'sparkle',
    'spiral', 'breath', 'wipeDiagonal', 'rainbow', 'blackout'
]);
export const DMD_PRIORITY = Object.freeze({ base: 0, info: 3, award: 5, big: 7, tilt: 9 });

// Pattern grammar: '<type>:<id|*>[@+1|@-1]', e.g. 'pathMade:leftRamp', 'trigger:leftOrbitUp@1', 'bumper:*'.
export function parsePattern(pattern) {
    if (pattern && typeof pattern === 'object') return pattern;
    const m = /^([A-Za-z]+):([A-Za-z0-9_*]+)(?:@([+-]?1))?$/.exec(String(pattern));
    if (!m) return null;
    return { type: m[1], id: m[2], dir: m[3] === undefined ? null : Number(m[3]) };
}

export function matchEvent(pattern, ev) {
    const p = parsePattern(pattern);
    if (!p || !ev || ev.type !== p.type) return false;
    if (p.id !== '*' && ev.id !== p.id) return false;
    if (p.dir !== null && ev.dir !== p.dir) return false;
    return true;
}
