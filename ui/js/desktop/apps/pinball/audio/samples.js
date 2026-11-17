// AuraPinball sample map: which hash-checked CC0 recordings ship, and how each mechanical role cuts them.
// Pure data (imported by the runtime, the asset builder and the tests; touches no AudioContext).
// SAMPLE_FILES: id -> { pack, path }. pack is 'minigolf' (ui/3d/minigolf/audio) or 'aura-drift' (ui/3d/aura-drift/audio);
// the builder copies the recording unchanged to ui/3d/pinball/v1/audio/<id>.ogg and records its provenance.
// TAKES: role -> list of { s: sample id, off: seconds into the file, dur: seconds played (omit = whole file), g: level trim }.
// Trims are playback offsets only (no re-encoding); the player fades the cut tail so no click is left behind.
// LOOPS: roll textures, cut and crossfaded into seamless loops at runtime.

const F = (pack, path) => ({ pack, path });
export const SAMPLE_FILES = Object.freeze({
    'drift-impact-metal-1': F('aura-drift', 'sfx/impact-metal-1.ogg'),
    'drift-impact-metal-2': F('aura-drift', 'sfx/impact-metal-2.ogg'),
    'drift-impact-metal-3': F('aura-drift', 'sfx/impact-metal-3.ogg'),
    'drift-impact-light-3': F('aura-drift', 'sfx/impact-light-3.ogg'),
    'drift-impact-heavy-1': F('aura-drift', 'sfx/impact-heavy-1.ogg'),
    'drift-prop-cone-1': F('aura-drift', 'sfx/prop-cone-1.ogg'),
    'drift-prop-cone-2': F('aura-drift', 'sfx/prop-cone-2.ogg'),
    'drift-prop-cone-3': F('aura-drift', 'sfx/prop-cone-3.ogg'),
    'drift-crate-break-1': F('aura-drift', 'sfx/crate-break-1.ogg'),
    'drift-crate-break-2': F('aura-drift', 'sfx/crate-break-2.ogg'),
    'drift-prop-barrel-1': F('aura-drift', 'sfx/prop-barrel-1.ogg'),
    'drift-prop-barrel-2': F('aura-drift', 'sfx/prop-barrel-2.ogg'),
    'drift-scrape-metal-1': F('aura-drift', 'sfx/scrape-metal-1.ogg'),
    'golf-impact-wood-medium-1': F('minigolf', 'sfx/impact-wood-medium-1.ogg'),
    'golf-impact-wood-medium-2': F('minigolf', 'sfx/impact-wood-medium-2.ogg'),
    'golf-impact-wood-medium-3': F('minigolf', 'sfx/impact-wood-medium-3.ogg'),
    'golf-impact-stone-3': F('minigolf', 'sfx/impact-stone-3.ogg'),
    'golf-cup-rim-1': F('minigolf', 'sfx/cup-rim-1.ogg'),
    'golf-cup-rim-2': F('minigolf', 'sfx/cup-rim-2.ogg'),
    'golf-cup-rim-3': F('minigolf', 'sfx/cup-rim-3.ogg'),
    'golf-impact-metal-light-1': F('minigolf', 'sfx/impact-metal-light-1.ogg'),
    'golf-impact-metal-light-2': F('minigolf', 'sfx/impact-metal-light-2.ogg'),
    'golf-impact-metal-light-3': F('minigolf', 'sfx/impact-metal-light-3.ogg'),
    'golf-impact-soft-light-1': F('minigolf', 'sfx/impact-soft-light-1.ogg'),
    'golf-cup-rattle-1': F('minigolf', 'sfx/cup-rattle-1.ogg'),
    'golf-roll-hard-1': F('minigolf', 'sfx/roll-hard-1.ogg'),
    'golf-roll-soft-1': F('minigolf', 'sfx/roll-soft-1.ogg'),
    'golf-win-moon-1': F('minigolf', 'sfx/win-moon-1.ogg'),
    'golf-win-temple-1': F('minigolf', 'sfx/win-temple-1.ogg')
});
export const SAMPLE_NAMES = Object.freeze(Object.keys(SAMPLE_FILES));

const T = (s, off = 0, dur, g = 1) => ({ s, off, dur, g });
export const TAKES = Object.freeze({
    // flipper coil: metal-on-metal strikes + heavy solenoid pull
    flipperUp: [T('drift-impact-metal-1', .01, .22), T('drift-impact-metal-2', 0, .19), T('drift-impact-heavy-1', 0, .22, .85)],
    flipperDown: [T('drift-impact-metal-2', 0, .095, .92), T('drift-impact-light-3', 0, .12, .9), T('golf-impact-wood-medium-1', 0, .11, .82)],
    // pop bumper: hard wood/stone/heavy thock (leading silence trimmed on stone-3)
    bumper: [T('drift-prop-barrel-2', 0, .12), T('golf-impact-stone-3', .035, .14), T('drift-prop-barrel-1', .06, .13), T('drift-impact-heavy-1', .01, .15, .85)],
    sling: [T('golf-cup-rim-2', 0, .2), T('golf-cup-rim-3', 0, .17), T('golf-cup-rim-1', 0, .15)],
    drop: [T('drift-prop-cone-1', .16, .28, .85), T('drift-prop-barrel-2', 0, .12, .85), T('drift-prop-barrel-1', 0, .12, .75)],
    standup: [T('golf-impact-wood-medium-2', 0, .11), T('drift-prop-cone-2', .01, .11), T('drift-prop-cone-3', 0, .11)],
    knocker: [T('drift-impact-light-3', 0, .18), T('drift-impact-heavy-1', 0, .36)],
    plungerRelease: [T('drift-impact-metal-2', 0, .3), T('drift-impact-metal-1', 0, .3)],
    scoop: [T('drift-impact-metal-2', 0, .34), T('drift-impact-metal-3', 0, .4, .82)],
    eject: [T('drift-impact-metal-1', .01, .2), T('drift-impact-metal-2', 0, .18)],
    kickback: [T('drift-impact-metal-1', .01, .24), T('drift-impact-metal-3', 0, .28, .85)],
    // crisp steel switch ticks: spinner, ratchet, rollover
    tick: [T('drift-crate-break-1', 0, .04), T('golf-impact-metal-light-1', 0, .045, .95), T('drift-crate-break-2', 0, .04), T('golf-cup-rim-2', 0, .055, .8)],
    clack: [T('golf-cup-rim-2', 0, .12), T('golf-cup-rim-3', 0, .1), T('golf-cup-rim-1', 0, .11), T('golf-impact-metal-light-2', 0, .085)],
    impactMetal: [T('drift-impact-metal-2', 0, .14, .65), T('golf-impact-metal-light-1', 0, .14), T('golf-impact-metal-light-3', 0, .13)],
    impactWood: [T('golf-impact-wood-medium-1', 0, .13), T('golf-impact-wood-medium-3', 0, .13)],
    impactRubber: [T('golf-impact-soft-light-1', 0, .11)],
    impactTarget: [T('drift-prop-cone-2', .01, .12), T('drift-prop-cone-3', 0, .12), T('golf-impact-wood-medium-2', 0, .11)],
    lock: [T('drift-impact-metal-3', 0, .32), T('drift-impact-metal-1', 0, .3)],
    drain: [T('golf-cup-rattle-1')],
    jackpot: [T('golf-win-moon-1')],
    bonus: [T('golf-win-temple-1')]
});

// Roll loops: sample, region [off, off + dur] and crossfade length (all seconds).
export const LOOPS = Object.freeze({
    hard: { s: 'golf-roll-hard-1', off: .15, dur: 2.7, xf: .25 },
    soft: { s: 'golf-roll-soft-1', off: 0, dur: 1.05, xf: .2 },
    scrape: { s: 'drift-scrape-metal-1', off: .25, dur: 1.4, xf: .15 }
});

// Pure: crossfade the tail of a region into its head so the result loops with no seam. data = Float32Array.
export function makeLoop(data, sampleRate, off, dur, xf) {
    const a = Math.max(0, Math.floor(off * sampleRate)), b = Math.min(data.length, a + Math.floor(dur * sampleRate));
    const L = b - a, X = Math.max(1, Math.min(Math.floor(xf * sampleRate), L >> 1)), out = new Float32Array(L - X);
    for (let i = 0; i < out.length; i++) out[i] = data[a + i];
    for (let i = 0; i < X; i++) {
        const k = (i + .5) / X, head = Math.sin(k * Math.PI / 2), tail = Math.cos(k * Math.PI / 2);
        out[i] = data[a + i] * head + data[b - X + i] * tail;
    }
    return out;
}
