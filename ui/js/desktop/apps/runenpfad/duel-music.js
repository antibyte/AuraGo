/* Runenpfad duel music: rich 8-bar combat loops per duelling NPC. DOM-free, deterministic over music.js. */
(function (root) {
    'use strict';

    const EIGHTHS = [0, 0.5, 1, 1.5, 2, 2.5, 3, 3.5];
    const SIXTEENTHS = [0, 0.25, 0.5, 0.75, 1, 1.25, 1.5, 1.75, 2, 2.25, 2.5, 2.75, 3, 3.25, 3.5, 3.75];

    const SPECS = {
        witch: {
            base: 'boss_cavern', scale: 'phrygian', bpm: 104, prog: [0, 1, 0, 6],
            arpInstr: 'darkbell', leadInstr: 'darkbell', bossArp: 0.65, ostInstr: 'string', ostGrid: EIGHTHS,
            drums: { kick: [0], snare: [2], hat: [1.5, 3.5], tom: [1, 3], taiko: [0] },
            melody: [
                [[0, 1.25, 4, 0.9], [1.5, 0.75, 5, 0.8], [2.25, 1.5, 1, 0.86]],
                [[0, 1.5, 2, 0.88], [1.75, 1.0, 4, 0.84], [2.75, 1.1, 0, 0.82]],
                [[0, 1.0, 5, 0.9], [1.25, 0.75, 6, 0.85], [2.25, 1.5, 4, 0.88]],
                [[0, 1.75, 1, 0.9], [2.0, 1.8, 0, 0.86]]
            ]
        },
        wizard: {
            base: 'boss_glacier', scale: 'harmonicMinor', bpm: 112, prog: [0, 3, 4, 0],
            arpInstr: 'glass', leadInstr: 'glass', bossArp: 0.75, ostInstr: 'ostinato', ostGrid: SIXTEENTHS,
            drums: { kick: [0, 2.5], snare: [2], hat: [1, 3], tom: [3.5] },
            melody: [
                [[0, 0.75, 4, 0.88], [0.75, 0.75, 6, 0.84], [1.5, 1.0, 7, 0.92], [2.75, 1.0, 5, 0.84]],
                [[0, 1.0, 3, 0.88], [1.0, 1.0, 4, 0.86], [2.0, 1.75, 6, 0.90]],
                [[0, 0.5, 7, 0.9], [0.5, 0.5, 6, 0.82], [1.0, 1.0, 4, 0.86], [2.25, 1.5, 2, 0.84]],
                [[0, 1.5, 6, 0.9], [1.75, 1.8, 0, 0.86]]
            ]
        },
        merchant: {
            base: 'boss_woodland', scale: 'minor', bpm: 132, prog: [0, 5, 3, 4],
            arpInstr: 'lute', leadInstr: 'pluck', bossArp: 0.55, ostInstr: 'ostinato', ostGrid: EIGHTHS,
            drums: { kick: [0, 1.5, 2.5], snare: [2], hat: [0.5, 1, 1.5, 2.5, 3, 3.5], tamb: [1, 3] },
            melody: [
                [[0, 0.5, 0, 0.88], [0.5, 0.5, 2, 0.82], [1.0, 1.0, 4, 0.9], [2.0, 0.5, 5, 0.84], [2.5, 1.0, 4, 0.86]],
                [[0, 0.75, 3, 0.88], [0.75, 0.75, 2, 0.82], [1.5, 1.0, 4, 0.88], [2.75, 1.0, 1, 0.82]],
                [[0, 0.5, 4, 0.9], [0.5, 0.5, 5, 0.85], [1.0, 0.5, 6, 0.88], [1.5, 1.0, 7, 0.92], [2.75, 1.0, 4, 0.84]],
                [[0, 1.0, 2, 0.86], [1.25, 0.75, 4, 0.84], [2.0, 1.75, 0, 0.88]]
            ]
        },
        chief: {
            base: 'boss_woodland', scale: 'phrygian', bpm: 118, prog: [0, 1, 0, 4],
            leadInstr: 'horn', arpInstr: 'horn', bossArp: 0, ostInstr: 'ostinato', ostGrid: EIGHTHS,
            drums: { kick: [0, 1.5], snare: [2], hat: [3], taiko: [0.5, 1, 1.5, 2.5, 3, 3.5] },
            melody: [
                [[0, 1.5, 0, 0.92], [1.5, 0.5, 1, 0.82], [2.0, 1.75, 4, 0.90]],
                [[0, 1.25, 4, 0.92], [1.5, 0.5, 3, 0.82], [2.0, 1.75, 1, 0.88]],
                [[0, 1.5, 2, 0.90], [1.5, 0.5, 4, 0.86], [2.0, 1.75, 5, 0.92]],
                [[0, 1.5, 4, 0.92], [1.75, 1.8, 0, 0.88]]
            ]
        },
        seer: {
            base: 'boss_desert', scale: 'phrygian', bpm: 100, prog: [0, 1, 0, 6],
            leadInstr: 'reed', arpInstr: 'oud', bossArp: 0.35, ostInstr: 'ostinato', ostGrid: EIGHTHS,
            drums: { kick: [0, 1.5], snare: [2], hat: [1, 3], tom: [0.75, 2.75, 3.5] },
            melody: [
                [[0, 1.75, 1, 0.9], [2.0, 1.6, 4, 0.86]],
                [[0, 1.25, 4, 0.88], [1.5, 0.5, 3, 0.82], [2.0, 1.75, 1, 0.88]],
                [[0, 1.5, 1, 0.9], [1.75, 0.75, 2, 0.84], [2.5, 1.3, 0, 0.86]],
                [[0, 1.6, 4, 0.9], [2.0, 1.7, 1, 0.92]]
            ]
        },
        alchemist: {
            base: 'boss_citadel', scale: 'harmonicMinor', bpm: 124, prog: [0, 5, 3, 4],
            leadInstr: 'lead', arpInstr: 'pizz', bossArp: 0.42, ostInstr: 'ostinato', ostGrid: EIGHTHS,
            drums: { kick: [0, 1.5], snare: [], hat: [3.5], clank: [2], tick: [1, 3], tickQuiet: true },
            melody: [
                [[0, 1.8, 2, 0.9], [2.0, 1.6, 5, 0.86]],
                [[0, 1.6, 4, 0.88], [1.75, 1.8, 2, 0.84]],
                [[0, 1.25, 6, 0.9], [1.5, 1.0, 4, 0.84], [2.5, 1.3, 3, 0.86]],
                [[0, 1.75, 5, 0.9], [2.0, 1.7, 0, 0.86]]
            ]
        }
    };

    const OSTINATO = [0, 0, 1, 0, 0, 4, 0, 1];
    function degMidi(th, d) {
        return th.root + th.scale[((d % 7) + 7) % 7] + 12 * Math.floor(d / 7);
    }
    function fold(m, lo, hi) {
        let x = m;
        while (x > hi) x -= 12;
        while (x < lo) x += 12;
        return x;
    }

    function makeComposer(npc, s) {
        return function (th, layer, bar) {
            const b8 = ((bar % 8) + 8) % 8;
            const sec = Math.floor(b8 / 4);
            const phr = b8 % 4;
            const d = th.prog[sec][phr];
            const r = th.root;
            const out = [];
            if (layer === 'pad') {
                const voiceType = npc === 'witch' ? 'choir' : 'pad';
                [d, d + 2, d + 4].forEach(function (deg, i) {
                    out.push({ t: 0, dur: 3.9, midi: fold(degMidi(th, deg), r - 2, r + 14), vel: i === 0 ? 0.86 : 0.72, type: voiceType });
                });
                if (npc === 'seer') out.push({ t: 0, dur: 3.8, midi: fold(degMidi(th, 1), r - 14, r - 3), vel: 0.76, type: 'choir' });
            } else if (layer === 'bass') {
                const lo = fold(degMidi(th, d), r - 14, r - 3);
                const fi = fold(degMidi(th, d + 4), r - 14, r - 3);
                const bt = npc === 'alchemist' ? 'fuzzbass' : 'bass';
                [[0, 1.35, 0.96, lo], [1.5, 0.45, 0.82, fi], [2, 1.2, 0.92, lo], [3.25, 0.65, 0.84, fi]].forEach(function (n) {
                    out.push({ t: n[0], dur: n[1], midi: n[3], vel: n[2], type: bt });
                });
            } else if (layer === 'arp') {
                const gap = s.ostGrid.length > 8 ? 0.5 : 0.5;
                EIGHTHS.forEach(function (t, i) {
                    const deg = d + [0, 2, 4, 7][i % 4];
                    out.push({ t: t, dur: gap * 1.15, midi: fold(degMidi(th, deg) + 12, r + 7, r + 24), vel: i % 2 ? 0.66 : 0.84, type: s.arpInstr });
                });
            } else if (layer === 'melody') {
                const phrase = s.melody[phr];
                const lo = npc === 'seer' ? r - 12 : (npc === 'alchemist' ? r - 7 : r);
                const hi = npc === 'seer' ? r - 1 : (npc === 'alchemist' ? r + 6 : r + 14);
                const mType = npc === 'merchant' ? (sec ? 'fiddle' : 'lute') : s.leadInstr;
                phrase.forEach(function (n) {
                    out.push({ t: n[0], dur: n[1], midi: fold(degMidi(th, n[2] + (sec === 1 && phr === 2 ? 1 : 0)), lo, hi), vel: n[3], type: mType });
                });
            } else if (layer === 'perc') {
                const dr = s.drums;
                (dr.kick || []).forEach(function (t) { out.push({ t: t, dur: 0.3, midi: null, vel: 0.9, type: 'kick' }); });
                (dr.snare || []).forEach(function (t) { out.push({ t: t, dur: 0.2, midi: null, vel: 0.66, type: 'snare' }); });
                (dr.hat || []).forEach(function (t) { out.push({ t: t, dur: 0.12, midi: null, vel: 0.46, type: 'hat' }); });
                (dr.tom || []).forEach(function (t, i) { out.push({ t: t, dur: 0.25, midi: null, vel: 0.72 + 0.1 * (i % 2), type: 'tom' }); });
                (dr.taiko || []).forEach(function (t, i) { out.push({ t: t, dur: 0.4, midi: null, vel: 0.82 + 0.12 * (i % 2), type: 'taiko' }); });
                (dr.tamb || []).forEach(function (t) { out.push({ t: t, dur: 0.16, midi: null, vel: 0.48, type: 'tamb' }); });
                (dr.tick || []).forEach(function (t, i) { out.push({ t: t, dur: 0.05, midi: null, vel: dr.tickQuiet ? 0.32 : (i % 4 === 0 ? 0.85 : 0.5), type: 'tick' }); });
                (dr.clank || []).forEach(function (t) { out.push({ t: t, dur: 0.32, midi: null, vel: 0.76, type: 'clank' }); });
            } else if (layer === 'boss') {
                out.push({ t: 0, dur: 3.9, midi: fold(degMidi(th, 0), r - 24, r - 13), vel: th.droneVel || 1, type: 'drone' });
                [[0, [d, d + 1], 0.95], [1.5, [d, d + 4], 0.8], [2.75, [d + 1, d], 0.88]].forEach(function (h) {
                    h[1].forEach(function (deg) {
                        out.push({ t: h[0], dur: 0.4, midi: fold(degMidi(th, deg), r - 12, r - 1), vel: h[2], type: 'stab' });
                    });
                });
                const fast = s.ostGrid.length > 8;
                s.ostGrid.forEach(function (t, i) {
                    out.push({
                        t: t, dur: fast ? 0.2 : 0.4,
                        midi: fold(degMidi(th, d + OSTINATO[(fast ? i >> 1 : i) % 8]), th.ostLow ? r - 18 : r - 12, th.ostLow ? r - 6 : r + 2),
                        vel: i % 2 === 0 ? 0.96 : 0.7,
                        type: s.ostInstr
                    });
                });
            }
            return out;
        };
    }

    const IDS = Object.keys(SPECS).map(function (npc) { return 'duel_' + npc; });

    function install(Music) {
        if (!Music || !Music.THEMES || !Music.SCALES) return 0;
        let made = 0;
        Object.keys(SPECS).forEach(function (npc) {
            const s = SPECS[npc];
            const base = Music.THEMES[s.base];
            if (!base || !Music.SCALES[s.scale]) return;
            const id = 'duel_' + npc;
            Music.THEMES[id] = Object.assign({}, base, {
                id: id, boss: true, scaleName: s.scale, scale: Music.SCALES[s.scale], bpm: s.bpm, prog: [s.prog, s.prog],
                arpInstr: s.arpInstr, melInstr: s.leadInstr, leadInstr: s.leadInstr, arpGrid: EIGHTHS, bossArp: s.bossArp,
                ostInstr: s.ostInstr, ostGrid: s.ostGrid, drums: s.drums,
                leadStyle: npc === 'seer' ? 'charmer' : (npc === 'alchemist' ? 'slow' : ''),
                ostLow: npc === 'alchemist', droneVel: npc === 'seer' ? 1.4 : 1,
                compose: makeComposer(npc, s)
            });
            made++;
        });
        return made;
    }

    function themeFor(sim) {
        const e = sim && sim.encounter;
        if (!e || e.kind !== 'duel') return null;
        const npc = e.npcId || (e.duel && e.duel.npcId);
        const id = 'duel_' + npc;
        return IDS.indexOf(id) >= 0 ? id : null;
    }

    install(root.RunenpfadMusic);

    root.RunenpfadDuelMusic = { SPECS: SPECS, THEME_IDS: IDS, install: install, themeFor: themeFor };
})(typeof window !== 'undefined' ? window : globalThis);
