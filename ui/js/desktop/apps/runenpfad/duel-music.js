/* Runenpfad duel music: one short, tense boss-style loop per duelling NPC. DOM-free, pure data over music.js. */
(function (root) {
    'use strict';

    const EIGHTHS = [0, 0.5, 1, 1.5, 2, 2.5, 3, 3.5];
    const SIXTEENTHS = [0, 0.25, 0.5, 0.75, 1, 1.25, 1.5, 1.75, 2, 2.25, 2.5, 2.75, 3, 3.25, 3.5, 3.75];

    // Per-NPC flavour on top of an existing boss theme (`base`). Each loop is a single 4-bar chord cycle played
    // twice, so the duel reads as a short tense ostinato. Instruments and drum voices all exist in music.js.
    const SPECS = {
        // Witch: eerie dark bells, sparse drums, flat second.
        witch: { base: 'boss_cavern', scale: 'phrygian', bpm: 104, prog: [0, 1, 0, 6], arpInstr: 'darkbell', leadInstr: 'darkbell', bossArp: 0.6, ostInstr: 'string', ostGrid: EIGHTHS, drums: { kick: [0], snare: [2], hat: [1.5, 3.5], tom: [1, 3] } },
        // Wizard: arcane glass bells over a fast sixteenth ostinato in harmonic minor.
        wizard: { base: 'boss_glacier', scale: 'harmonicMinor', bpm: 112, prog: [0, 3, 4, 0], arpInstr: 'glass', leadInstr: 'glass', bossArp: 0.7, ostInstr: 'ostinato', ostGrid: SIXTEENTHS, drums: { kick: [0, 2.5], snare: [2], hat: [1, 3], tom: [3.5] } },
        // Merchant: sly plucked skirmish, quick and light.
        merchant: { base: 'boss_woodland', scale: 'minor', bpm: 132, prog: [0, 5, 3, 4], arpInstr: 'pluck', leadInstr: 'pluck', bossArp: 0.5, ostInstr: 'ostinato', ostGrid: EIGHTHS, drums: { kick: [0, 1.5, 2.5], snare: [2], hat: [0.5, 1, 1.5, 2.5, 3, 3.5] } },
        // Chief: rowdy kobold horn with war drums.
        chief: { base: 'boss_woodland', scale: 'phrygian', bpm: 118, prog: [0, 1, 0, 4], leadInstr: 'horn', arpInstr: 'horn', bossArp: 0, ostInstr: 'ostinato', ostGrid: EIGHTHS, drums: { kick: [0, 1.5], snare: [2], hat: [3], taiko: [0.5, 1, 1.5, 2.5, 3, 3.5] } },
        // Seer: desert snake-charmer reed over a heavy drone (stays phrygian, never a major third).
        seer: { base: 'boss_desert', scale: 'phrygian', bpm: 100, prog: [0, 1, 0, 6], leadInstr: 'reed', arpInstr: 'reed', bossArp: 0, ostInstr: 'ostinato', ostGrid: EIGHTHS, drums: { kick: [0, 1.5], snare: [2], hat: [1, 3], tom: [0.75, 2.75, 3.5] } },
        // Alchemist: mechanical ticks and clanks with a fuzz bass.
        alchemist: { base: 'boss_citadel', scale: 'harmonicMinor', bpm: 124, prog: [0, 5, 3, 4], leadInstr: 'lead', arpInstr: 'pluck', bossArp: 0.35, ostInstr: 'ostinato', ostGrid: EIGHTHS, drums: { kick: [0, 1.5], snare: [], hat: [3.5], clank: [2], tick: [1, 3], tickQuiet: true } }
    };

    const IDS = Object.keys(SPECS).map(function (npc) { return 'duel_' + npc; });

    // Registers one 'duel_<npc>' theme per spec into RunenpfadMusic.THEMES (the same object music.js composes from).
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
                ostInstr: s.ostInstr, ostGrid: s.ostGrid, drums: s.drums, leadStyle: npc === 'seer' ? 'charmer' : (npc === 'alchemist' ? 'slow' : ''),
                ostLow: npc === 'alchemist', droneVel: npc === 'seer' ? 1.5 : 1
            });
            made++;
        });
        return made;
    }

    // Theme id for the running duel (null outside a duel). music.js calls this from themeIdFor while latched on boss.
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
