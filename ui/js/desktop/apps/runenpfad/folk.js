/* Runenpfad folk: the villagers and place residents of Pathrest, their NPC definitions, sprite rows and ambient ids. DOM-free.
 * Adds to RunenpfadNpcs.NPCS (loaded after npcs.js); dialogue data lives in folk-story.js. */
(function (root) {
    'use strict';

    // [sheet, row] of the 4x4 top-down folk sheets (contract section 1), also for the six story NPCs.
    const SPRITES = {
        innkeeper: ['td-folk-village', 0], smith: ['td-folk-village', 1], shopkeeper: ['td-folk-village', 2], elder: ['td-folk-village', 3],
        bard: ['td-folk-guests', 0], patron: ['td-folk-guests', 1], child: ['td-folk-guests', 2], hermit: ['td-folk-guests', 3],
        barkeep: ['td-folk-world', 0], scribe: ['td-folk-world', 1], climber: ['td-folk-world', 2], automaton: ['td-folk-world', 3],
        witch: ['td-folk-story-a', 0], wizard: ['td-folk-story-a', 1], merchant: ['td-folk-story-a', 2], chief: ['td-folk-story-a', 3],
        seer: ['td-folk-story-b', 0], alchemist: ['td-folk-story-b', 1], villager_f: ['td-folk-story-b', 2], villager_m: ['td-folk-story-b', 3]
    };

    // Ids that stand around without a dialogue (no talk prompt).
    const AMBIENT = ['patron', 'child', 'villager_f', 'villager_m'];

    const HUMAN = { pack: 'human-characters-animated', anim: 'ranger_idle' };

    // id -> faction (null: villagers, no reputation), colour, sprite height and trade offers.
    const FOLK = {
        elder: { faction: null, color: '#e6c27a', offers: [] },
        innkeeper: { faction: null, color: '#f59e6b', offers: ['meal', 'potion'] },
        smith: { faction: null, color: '#94a3b8', offers: [] },
        shopkeeper: { faction: null, color: '#86d6a0', offers: ['potion', 'map_charm', 'lucky_amulet', 'rune_attune'] },
        bard: { faction: null, color: '#f0abfc', offers: [] },
        hermit: { faction: 'coven', color: '#a8a29e', offers: [] },
        barkeep: { faction: 'kobolds', color: '#fb7185', offers: ['potion', 'lucky_amulet'], fallback: { pack: 'monsters-animals-animated', anim: 'goblin_idle' } },
        scribe: { faction: 'circle', color: '#fcd34d', offers: ['map_charm', 'rune_magnet'] },
        climber: { faction: 'circle', color: '#7dd3fc', offers: [] },
        automaton: { faction: 'coven', color: '#d6a35c', offers: [], fallback: { pack: 'robots-drones-animated', anim: 'service_robot_idle' } }
    };

    const AMBIENT_COLORS = { patron: '#d6a35c', child: '#fde68a', villager_f: '#f9a8d4', villager_m: '#93c5fd' };

    function build() {
        const defs = {};
        Object.keys(FOLK).forEach(function (id) {
            const f = FOLK[id];
            const fb = f.fallback || HUMAN;
            defs[id] = {
                faction: f.faction, nameKey: 'runenpfad.npc.' + id + '.name', titleKey: 'runenpfad.npc.' + id + '.title', color: f.color, height: 74,
                fallback: { pack: fb.pack, anim: fb.anim, tint: f.fallback ? null : f.color }, offers: f.offers.slice(), duelCoins: 0
            };
        });
        AMBIENT.forEach(function (id) {
            defs[id] = {
                faction: null, nameKey: 'runenpfad.npc.ambient.name', titleKey: 'runenpfad.npc.ambient.title', color: AMBIENT_COLORS[id], height: id === 'child' ? 58 : 74,
                fallback: { pack: HUMAN.pack, anim: HUMAN.anim, tint: AMBIENT_COLORS[id] }, offers: [], duelCoins: 0
            };
        });
        return defs;
    }

    const IDS = Object.keys(FOLK).concat(AMBIENT);
    const defs = build();
    const npcs = root.RunenpfadNpcs;
    if (npcs && npcs.NPCS) {
        Object.keys(defs).forEach(function (id) { npcs.NPCS[id] = defs[id]; });
    }

    root.RunenpfadFolk = { SPRITES: SPRITES, AMBIENT: AMBIENT, IDS: IDS, DEFS: defs };
})(typeof window !== 'undefined' ? window : globalThis);
