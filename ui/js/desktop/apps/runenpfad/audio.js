/* Runenpfad Web Audio controller: sample pack + procedural synth layers, buses, per-biome reverb. */
(function (root) {
    'use strict';

    const BASE_URL = '/api/game-maker/asset-packs/aurago-sounds/';
    const REQUIRED_SOUNDS = [
        'forest-day', 'forest-night', 'wind-gentle', 'wind-strong', 'industrial',
        'step-grass', 'step-stone', 'step-gravel', 'step-wood', 'step-metal',
        'jump', 'land', 'pistol', 'laser', 'impact-flesh', 'impact-metal',
        'impact-stone', 'explosion-small', 'explosion-large', 'crate-break',
        'door-wood', 'door-metal', 'pickup', 'ui-click', 'victory', 'defeat',
        'rifle', 'shotgun', 'reload', 'electric-spark', 'fire', 'thunder',
        'stream', 'engine', 'space-hum', 'water-splash', 'rain-light', 'rain-heavy'
    ];
    const PRELOAD_SFX = [
        'ui-click', 'jump', 'land', 'step-grass', 'laser', 'pickup', 'crate-break',
        'impact-flesh', 'impact-metal', 'impact-stone', 'explosion-small', 'electric-spark'
    ];
    const FADE_SECONDS = 1.2;
    const COIN_LADDER = [0, 2, 4, 7, 9, 12, 14, 16];
    const COIN_RESET_SECONDS = 0.7;
    const BIOME_BY_AMBIENCE = { 'forest-day': 'woodland', 'forest-night': 'cavern', 'wind-strong': 'desert', 'wind-gentle': 'glacier', industrial: 'citadel' };
    const SYNTH_CAP = 24;
    const SYNTH_HARD_CAP = 40;
    const DRIP_BEDS = { cavern: 'rain-light' };
    // Reverb tail length (s) and wet level per biome.
    const REVERBS = {
        woodland: { seconds: 1.2, wet: 0.16 },
        cavern: { seconds: 2.8, wet: 0.35 },
        desert: { seconds: 0.9, wet: 0.1 },
        glacier: { seconds: 2.2, wet: 0.26 },
        citadel: { seconds: 1.6, wet: 0.2 }
    };

    // Event -> cue table. `sound: null` means the event carries its own sound id, `sound: false` means synth only.
    // `synth` names a procedural layer (gain `sg`, pitch/speed scale `sr`); `also` adds more synth layers
    // ({ synth, sg, sr, delay }); `voice` adds the per-biome boss voice ({ gain, sr, short }); `vary` is a
    // deterministic detune amount; `by` overrides fields by an event detail value (or '@biome').
    // Unknown events have no entry and stay silent.
    const BOSS_ATTACKS = {
        charge: { sound: 'impact-stone', rate: 0.8, synth: 'whoosh', sg: 0.6, sr: 0.5, voice: { gain: 0.5, sr: 1, short: true } },
        ground_pound: {
            sound: 'impact-stone', rate: 0.7, gain: 0.8,
            also: [{ synth: 'boom', sg: 0.6, sr: 1.2 }, { synth: 'impact', sg: 0.7, sr: 0.6 }]
        },
        leap: { sound: 'jump', rate: 0.6, synth: 'whoosh', sg: 0.5, sr: 0.7, voice: { gain: 0.4, sr: 1.2, short: true } },
        volley: { synth: 'whoosh', sg: 0.6, sr: 1.3, also: [{ synth: 'twang', sg: 0.3, sr: 1.2 }] },
        summon: { sound: 'electric-spark', rate: 0.6, gain: 0.6, also: [{ synth: 'shimmer', sg: 0.4, sr: 0.55 }, { synth: 'hum', sg: 0.3, sr: 0.6 }] },
        ring: { sound: 'laser', rate: 0.7, synth: 'zap', sg: 0.4, sr: 0.7, also: [{ synth: 'shimmer', sg: 0.3, sr: 0.7 }] },
        laser: { sound: 'laser', rate: 0.55, gain: 0.7, synth: 'zap', sg: 0.35, sr: 0.5, also: [{ synth: 'crack', sg: 0.5 }] },
        dive: { synth: 'whoosh', sg: 0.7, sr: 0.6, also: [{ synth: 'whistle', sg: 0.45, sr: 0.9 }, { synth: 'impact', sg: 0.5, sr: 0.8, delay: 0.12 }] },
        missiles: { sound: 'reload', gain: 0.7, synth: 'whoosh', sg: 0.4, also: [{ synth: 'clank', sg: 0.4, sr: 1.2 }] },
        fire_strip: { sound: 'fire', gain: 0.7, also: [{ synth: 'flame', sg: 0.6 }] },
        // Dragon boss patterns: the voice resolves to the dragon roar (see dragonVoice()).
        strafe: { sound: 'fire', gain: 0.8, rate: 0.7, synth: 'dragonbreath', sg: 0.85, also: [{ synth: 'dragonpass', sg: 0.6 }], voice: { gain: 0.6, sr: 1, short: true } },
        fireballs: { sound: 'fire', gain: 0.5, rate: 1.2, synth: 'fwoosh', sg: 0.6, sr: 0.7, also: [{ synth: 'fwoosh', sg: 0.5, sr: 0.9, delay: 0.14 }, { synth: 'fwoosh', sg: 0.45, sr: 1.1, delay: 0.28 }] },
        roar: { sound: 'thunder', gain: 0.5, rate: 0.7, synth: 'dragonroar', sg: 0.95, also: [{ synth: 'boom', sg: 0.5, sr: 0.6 }] }
    };
    BOSS_ATTACKS.pound = BOSS_ATTACKS.ground_pound; // legacy ids
    BOSS_ATTACKS.groundfire = BOSS_ATTACKS.fire_strip;
    // Telegraph kinds come from bosses.js (`telegraph(e, kind, ...)`).
    const BOSS_TELEGRAPHS = {
        charge: { synth: 'charge', sg: 0.35, sr: 0.8, voice: { gain: 0.3, sr: 1.1, short: true } },
        pound: { synth: 'charge', sg: 0.4, sr: 0.55 },
        leap: { synth: 'creak', sg: 0.5, sr: 1.3 },
        volley: { synth: 'charge', sg: 0.3, sr: 1.6 },
        summon: { synth: 'shimmer', sg: 0.35, sr: 0.55, also: [{ synth: 'hum', sg: 0.3, sr: 0.6 }] },
        ring: { synth: 'charge', sg: 0.35, sr: 2 },
        beam: { synth: 'charge', sg: 0.4, sr: 2.6, also: [{ synth: 'hiss', sg: 0.25 }] },
        dive: { synth: 'whistle', sg: 0.4 },
        strafe: { synth: 'dragonwarn', sg: 0.5, also: [{ synth: 'hiss', sg: 0.25 }] },
        fireballs: { synth: 'charge', sg: 0.35, sr: 0.7, also: [{ synth: 'fwoosh', sg: 0.3, sr: 0.8 }] },
        roar: { synth: 'dragonwarn', sg: 0.6, sr: 1.1 },
        missiles: { synth: 'ratchet', sg: 0.5 },
        fire: { synth: 'hiss', sg: 0.5, also: [{ synth: 'charge', sg: 0.2, sr: 0.5 }] },
        '*': { synth: 'warn', sg: 0.3, sr: 1 }
    };
    const BOSS_VOICES = {
        woodland: [{ synth: 'roar', sr: 0.8 }],
        cavern: [{ synth: 'roar', sr: 1.45 }],
        desert: [{ synth: 'servo', sr: 0.9 }],
        glacier: [{ synth: 'rotor', sr: 1 }],
        citadel: [{ synth: 'servo', sr: 0.6 }, { synth: 'clank', sr: 0.7 }]
    };
    const DRAGON_VOICES = {
        ember: [{ synth: 'dragonroar', sr: 1 }],
        frost: [{ synth: 'dragonroar_frost', sr: 1 }]
    };
    const DRAGON_PATTERNS = { strafe: 1, fireballs: 1, roar: 1 };
    const GLYPH_TONES = {
        0: { synth: 'glyph', sr: 1 },
        1: { synth: 'glyph', sr: 1.26 },
        2: { synth: 'glyph', sr: 1.5 },
        3: { synth: 'glyph', sr: 1.68 },
        '*': { synth: 'glyph', sr: 1 }
    };
    const SOLVED_FANFARE = { sound: 'pickup', rate: 1.2, gain: 0.35 };
    const RUNE_PICKUPS = {
        fire: { sound: 'fire', rate: 1.3, synth: 'fireup', sg: 0.5, also: [{ synth: 'shimmer', sg: 0.3, sr: 0.9 }] },
        frost: { sound: false, synth: 'frostchime', sg: 0.5, also: [{ synth: 'shimmer', sg: 0.25, sr: 1.4 }] },
        storm: { sound: 'electric-spark', rate: 1.2, synth: 'crackle', sg: 0.5, also: [{ synth: 'shimmer', sg: 0.25, sr: 1.2 }] },
        magnet: { sound: false, synth: 'hum', sg: 0.5, also: [{ synth: 'shimmer', sg: 0.3, sr: 0.8 }] },
        '*': { sound: false, synth: 'shimmer', sg: 0.45 }
    };
    // ---- NPC encounters: duel pattern sounds, keyed `<npc>_<a|b|c>` like the pattern table in duel-patterns.js ----
    // Tells are quiet anticipation cues; attacks are the release. Only existing synths are used.
    const DUEL_TELLS = {
        witch_a: { synth: 'charge', sg: 0.35, sr: 1.6 },
        witch_b: { synth: 'hum', sg: 0.3, sr: 0.7, also: [{ synth: 'ghost', sg: 0.2, sr: 0.7 }] },
        witch_c: { synth: 'whistle', sg: 0.4, sr: 0.8 },
        wizard_a: { synth: 'magicping', sg: 0.4, sr: 1.4, also: [{ synth: 'shimmer', sg: 0.25, sr: 1.2 }] },
        wizard_b: { synth: 'ghost', sg: 0.35, sr: 0.8, also: [{ synth: 'shimmer', sg: 0.25, sr: 0.8 }] },
        wizard_c: { synth: 'charge', sg: 0.35, sr: 0.6, also: [{ synth: 'hum', sg: 0.25, sr: 0.8 }] },
        merchant_a: { synth: 'ratchet', sg: 0.4, sr: 1.2 },
        merchant_b: { synth: 'ratchet', sg: 0.4, sr: 0.8, also: [{ synth: 'hiss', sg: 0.2, sr: 1.2 }] },
        merchant_c: { synth: 'puff', sg: 0.3, sr: 1.2 },
        chief_a: { synth: 'charge', sg: 0.4, sr: 0.5, also: [{ synth: 'creak', sg: 0.35, sr: 0.8 }] },
        chief_b: { synth: 'roar', sg: 0.45, sr: 1.3 },
        chief_c: { synth: 'stomp', sg: 0.45, sr: 0.8, also: [{ synth: 'charge', sg: 0.3, sr: 0.7 }] },
        seer_a: { synth: 'whoosh', sg: 0.3, sr: 0.5, also: [{ synth: 'hiss', sg: 0.2, sr: 0.9 }] },
        seer_b: { synth: 'shimmer', sg: 0.35, sr: 0.7, also: [{ synth: 'ghost', sg: 0.25, sr: 1.1 }] },
        seer_c: { synth: 'charge', sg: 0.4, sr: 2.6, also: [{ synth: 'hiss', sg: 0.25, sr: 1 }] },
        alchemist_a: { synth: 'fizzle', sg: 0.4, sr: 1.2 },
        alchemist_b: { synth: 'ratchet', sg: 0.4, sr: 0.8, also: [{ synth: 'clank', sg: 0.3, sr: 1.1 }] },
        alchemist_c: { synth: 'hiss', sg: 0.4, sr: 0.9, also: [{ synth: 'steam', sg: 0.3, sr: 1.2 }] },
        '*': { synth: 'warn', sg: 0.3, sr: 1 }
    };
    const DUEL_ATTACKS = {
        witch_a: { synth: 'zap', sg: 0.4, sr: 0.9 },
        witch_b: { synth: 'puff', sg: 0.45, sr: 0.7 },
        witch_c: { synth: 'screech', sg: 0.45, sr: 1 },
        wizard_a: { synth: 'magicping', sg: 0.45, sr: 1.1 },
        wizard_b: { synth: 'ghost', sg: 0.45, sr: 1.3 },
        wizard_c: { synth: 'hum', sg: 0.45, sr: 0.9 },
        merchant_a: { synth: 'whoosh', sg: 0.5, sr: 1.3 },
        merchant_b: { synth: 'boom', sg: 0.5, sr: 1.1 },
        merchant_c: { synth: 'puff', sg: 0.45, sr: 1.5 },
        chief_a: { synth: 'impact', sg: 0.6, sr: 0.7, also: [{ synth: 'boom', sg: 0.5, sr: 1 }] },
        chief_b: { synth: 'roar', sg: 0.5, sr: 1 },
        chief_c: { synth: 'stomp', sg: 0.6, sr: 0.9 },
        seer_a: { synth: 'steam', sg: 0.5, sr: 0.8 },
        seer_b: { synth: 'ghost', sg: 0.45, sr: 1 },
        seer_c: { synth: 'warn', sg: 0.4, sr: 1.4, also: [{ synth: 'flame', sg: 0.5, sr: 1 }] },
        alchemist_a: { synth: 'puff', sg: 0.45, sr: 1, also: [{ synth: 'fizzle', sg: 0.35, sr: 1 }] },
        alchemist_b: { synth: 'ratchet', sg: 0.4, sr: 1, also: [{ synth: 'clank', sg: 0.45, sr: 1 }] },
        alchemist_c: { synth: 'steam', sg: 0.5, sr: 1, also: [{ synth: 'hiss', sg: 0.35, sr: 1 }] },
        '*': { synth: 'whoosh', sg: 0.4, sr: 1 }
    };
    // Quest jingles built from delayed chimes.
    const QUEST_JINGLES = {
        start: { synth: 'chime', sg: 0.3, sr: 1, also: [{ synth: 'chime', sg: 0.28, sr: 1.5, delay: 0.1 }, { synth: 'shimmer', sg: 0.2, sr: 1.1, delay: 0.14 }] },
        done: {
            synth: 'solved', sg: 0.5, sr: 1.122, sound: 'pickup', rate: 1.2, gain: 0.3,
            also: [{ synth: 'chime', sg: 0.3, sr: 2, delay: 0.3 }, { synth: 'shimmer', sg: 0.3, sr: 1, delay: 0.4 }]
        }
    };
    QUEST_JINGLES['*'] = QUEST_JINGLES.start;
    const CUE_MAP = {
        step: { sound: null, gain: 0.38, cooldown: 220, vary: 0.14 },
        jump: { sound: 'jump', gain: 0.58, cooldown: 90, vary: 0.1 },
        dash: { sound: 'jump', gain: 0.36, cooldown: 140, rate: 1.32, vary: 0.08, synth: 'whoosh', sg: 0.4, sr: 1.3 },
        spring: { sound: 'jump', gain: 0.5, cooldown: 90, rate: 1.45, vary: 0.08, synth: 'boing', sg: 0.35 },
        bounce: { sound: 'jump', gain: 0.4, cooldown: 90, rate: 1.6, vary: 0.08, synth: 'boing', sg: 0.4, sr: 1.2 },
        land: { sound: 'land', gain: 0.5, cooldown: 110, vary: 0.12 },
        slam: { sound: 'shotgun', gain: 0.78, cooldown: 120, rate: 0.92, also: [{ synth: 'boom', sg: 0.5, sr: 1.1 }] },
        slam_land: { sound: 'impact-stone', gain: 0.74, cooldown: 120, rate: 0.95, also: [{ synth: 'impact', sg: 0.7, sr: 0.9 }, { synth: 'boom', sg: 0.4, sr: 1.3 }] },
        attack_melee: { sound: false, gain: 0.5, cooldown: 110, vary: 0.12, synth: 'whoosh', sg: 0.2, also: [{ synth: 'sword', sg: 1.3 }] },
        attack_ranged: { sound: 'laser', gain: 0.45, cooldown: 90, rate: 1.05, vary: 0.1, synth: 'zap', sg: 0.3, also: [{ synth: 'twang', sg: 0.55 }] },
        crate_break: { sound: 'crate-break', gain: 0.65, cooldown: 80, vary: 0.16, also: [{ synth: 'impact', sg: 0.4, sr: 1.2 }] },
        enemy_hit: { sound: null, gain: 0.6, cooldown: 70, vary: 0.16, synth: 'impact', sg: 0.45, sr: 1.1 },
        enemy_kill: { sound: 'explosion-small', gain: 0.6, cooldown: 100, vary: 0.14, synth: 'poof', sg: 0.9, also: [{ synth: 'impact', sg: 0.45 }] },
        enemy_shot: { sound: 'pistol', gain: 0.4, cooldown: 90, vary: 0.14 },
        enemy_attack: {
            sound: false, gain: 0.4, cooldown: 120, vary: 0.1,
            by: {
                key: 'kind',
                map: {
                    shot: { synth: 'zap', sg: 0.22, sr: 0.8 },
                    lunge: { synth: 'whoosh', sg: 0.5, sr: 0.7 },
                    dive: { synth: 'screech', sg: 0.45 },
                    swoop: { synth: 'screech', sg: 0.45 },
                    lob: { synth: 'puff', sg: 0.3, sr: 1.4 },
                    fireball: { synth: 'fwoosh', sg: 0.5, also: [{ synth: 'puff', sg: 0.3, sr: 1.8 }] },
                    spore: { synth: 'puff', sg: 0.3, sr: 1.4 }
                }
            },
            // The attacking enemy kind (when the simulation reports it) wins over the generic shot/lunge sound.
            byEnemy: { crystal_bat: { synth: 'screech', sg: 0.45 }, sporecap: { synth: 'puff', sg: 0.3, sr: 1.4 } }
        },
        enemy_dive: { sound: false, gain: 0.4, cooldown: 250, synth: 'screech', sg: 0.45 },
        enemy_block: { sound: 'impact-metal', gain: 0.6, cooldown: 90, vary: 0.1, synth: 'clang', sg: 0.6 },
        enemy_phase: {
            sound: false, gain: 0.4, cooldown: 120, synth: 'ghost', sg: 0.35,
            by: { key: 'phased', map: { true: { sr: 0.7 }, false: { sr: 1.3 } } }
        },
        enemy_windup: { sound: false, gain: 0.4, cooldown: 300, synth: 'ratchet', sg: 0.5 },
        enemy_slam: { sound: 'impact-stone', gain: 0.8, cooldown: 150, rate: 0.75, also: [{ synth: 'boom', sg: 0.6, sr: 1.1 }, { synth: 'clank', sg: 0.5, sr: 0.8 }] },
        spore_lob: { sound: false, gain: 0.4, cooldown: 150, synth: 'puff', sg: 0.3, sr: 1.4 },
        spore_burst: { sound: 'water-splash', gain: 0.2, cooldown: 120, rate: 1.6, synth: 'puff', sg: 0.5, sr: 0.8 },
        stomp: { sound: 'land', gain: 0.4, cooldown: 80, rate: 1.2, synth: 'stomp', sg: 0.6, also: [{ synth: 'boing', sg: 0.25, sr: 1.3 }] },
        chest_open: { sound: false, gain: 0.5, cooldown: 200, synth: 'creak', sg: 1.6, also: [{ synth: 'shimmer', sg: 0.4, sr: 1.1, delay: 0.55 }] },
        rune_pickup: { sound: false, gain: 0.6, cooldown: 200, by: { key: 'kind', map: RUNE_PICKUPS } },
        rune_attuned: { sound: false, gain: 0.3, cooldown: 300, synth: 'shimmer', sg: 0.3, sr: 1.3 },
        rune_expire: { sound: false, gain: 0.4, cooldown: 200, synth: 'down', sg: 0.4 },
        rune_shield_break: { sound: false, gain: 0.3, cooldown: 150, synth: 'ice', sg: 1 },
        rune_zap: { sound: 'electric-spark', gain: 0.55, cooldown: 70, rate: 1.2, synth: 'zap', sg: 0.5, also: [{ synth: 'crack', sg: 0.4 }] },
        geyser_burst: { sound: 'water-splash', gain: 0.1, cooldown: 250, rate: 0.8, synth: 'steam', sg: 0.95 },
        switch_on: { sound: 'ui-click', gain: 0.3, cooldown: 120, rate: 1.6, synth: 'ping', sg: 0.5 },
        gate_open: { sound: 'impact-stone', gain: 0.55, cooldown: 400, rate: 0.6, synth: 'grind', sg: 0.7 },
        jet_warn: { sound: false, gain: 0.4, cooldown: 200, synth: 'hiss', sg: 0.5 },
        gust_warn: { sound: 'wind-gentle', gain: 0.3, cooldown: 1500, rate: 1.15, synth: 'whoosh', sg: 0.18, sr: 0.7 },
        gust: { sound: 'wind-strong', gain: 0.5, cooldown: 1500, rate: 1, synth: 'whoosh', sg: 0.3, sr: 0.55 },
        jet_fire: { sound: 'fire', gain: 0.7, cooldown: 200, rate: 0.8, synth: 'flame', sg: 0.8 },
        boss_intro: {
            sound: 'thunder', gain: 0.55, cooldown: 600, rate: 0.75, synth: 'warn', sg: 0.32, sr: 0.5,
            also: [{ synth: 'boom', sg: 0.64, sr: 0.7 }], voice: { gain: 0.6, sr: 0.85 }
        },
        boss_phase: { sound: 'thunder', gain: 0.8, cooldown: 300, also: [{ synth: 'boom', sg: 0.6, sr: 0.8 }, { synth: 'crack', sg: 0.4 }], voice: { gain: 0.8, sr: 0.8 } },
        boss_defeat: {
            sound: 'explosion-large', gain: 0.86, cooldown: 200,
            also: [
                { synth: 'boom', sg: 0.9, sr: 0.8 }, { synth: 'boom', sg: 0.7, sr: 0.6, delay: 0.2 },
                { synth: 'boom', sg: 0.6, sr: 0.5, delay: 0.45 }, { synth: 'shimmer', sg: 0.5, sr: 0.8, delay: 0.7 }
            ],
            voice: { gain: 0.7, sr: 0.55 }
        },
        boss_slam: { sound: 'explosion-large', gain: 0.7, cooldown: 200, rate: 0.8, also: [{ synth: 'boom', sg: 0.7, sr: 0.9 }, { synth: 'impact', sg: 0.6, sr: 0.7 }] },
        boss_telegraph: { sound: false, gain: 0.4, cooldown: 350, by: { key: 'kind', map: BOSS_TELEGRAPHS } },
        boss_attack: { sound: false, gain: 0.6, cooldown: 150, by: { key: 'pattern', map: BOSS_ATTACKS } },
        bomb_blast: { sound: 'explosion-large', gain: 0.7, cooldown: 120, rate: 1.05, also: [{ synth: 'boom', sg: 0.6, sr: 1.1 }] },
        boulder_land: { sound: 'impact-stone', gain: 0.62, cooldown: 160, rate: 0.9, also: [{ synth: 'impact', sg: 0.6, sr: 0.7 }] },
        player_hurt: { sound: null, gain: 0.66, cooldown: 180, vary: 0.1, synth: 'impact', sg: 0.35, sr: 1.3 },
        player_defeat: { sound: 'defeat', gain: 0.75, cooldown: 500 },
        player_fall: {
            sound: false, gain: 0.6, cooldown: 400,
            by: { key: '@biome', map: { woodland: { sound: 'water-splash' }, glacier: { sound: 'water-splash' }, '*': { synth: 'whoosh', sg: 0.6, sr: 0.35 } } }
        },
        pickup_coin: { sound: 'pickup', gain: 0.42, cooldown: 40, rate: 1.0, ladder: true, synth: 'chime', sg: 0.3 },
        pickup_gem: { sound: 'laser', gain: 0.6, cooldown: 90, rate: 0.92, also: [{ synth: 'shimmer', sg: 0.25, sr: 1.3 }] },
        pickup_star: { sound: 'electric-spark', gain: 0.62, cooldown: 90, rate: 1.1, synth: 'chime', sg: 0.35, sr: 2 },
        pickup_treasure: { sound: 'pickup', gain: 0.6, cooldown: 120, rate: 0.8, synth: 'chime', sg: 0.4, sr: 1.5 },
        pickup_heart: { sound: 'pickup', gain: 0.56, cooldown: 120, rate: 0.88 },
        pickup_potion: { sound: 'electric-spark', gain: 0.56, cooldown: 120, rate: 0.86 },
        pickup_key: { sound: 'electric-spark', gain: 0.72, cooldown: 140, rate: 0.94, also: [{ synth: 'shimmer', sg: 0.3, sr: 1.1 }] },
        pickup_shrine: { sound: 'electric-spark', gain: 0.75, cooldown: 200, rate: 0.8, also: [{ synth: 'shimmer', sg: 0.4, sr: 0.8 }] },
        checkpoint: { sound: 'fire', gain: 0.55, cooldown: 200, also: [{ synth: 'ping', sg: 0.35, sr: 0.8 }, { synth: 'shimmer', sg: 0.3, sr: 1 }] },
        heal: { sound: 'pickup', gain: 0.56, cooldown: 150, rate: 0.84 },
        combo: { sound: false, gain: 0.5, cooldown: 90, synth: 'combo', sg: 0.35 },
        secret_break: { sound: 'impact-stone', gain: 0.6, cooldown: 200, rate: 0.85, synth: 'chime', sg: 0.4, sr: 1.5 },
        lantern_lit: { sound: 'fire', gain: 0.42, cooldown: 200, rate: 1.2, synth: 'chime', sg: 0.3, sr: 1.8 },
        hazard: {
            sound: 'fire', gain: 0.4, cooldown: 260,
            by: {
                key: 'kind',
                map: {
                    spikes: { sound: 'impact-metal', gain: 0.5 },
                    boulder: { sound: 'impact-stone', gain: 0.55 },
                    laser: { sound: 'laser', gain: 0.5, synth: 'zap', sg: 0.3 },
                    poison: { sound: false, synth: 'puff', sg: 0.4, sr: 0.9 },
                    spore: { sound: false, synth: 'puff', sg: 0.4, sr: 0.9 }
                }
            }
        },
        spike_rise: { sound: 'impact-metal', gain: 0.3, cooldown: 200, rate: 1.7 },
        door_open: { sound: 'door-wood', gain: 0.65, cooldown: 300, by: { key: 'material', map: { wood: { sound: 'door-wood' }, metal: { sound: 'door-metal' } } } },
        cannon: { sound: 'shotgun', gain: 0.45, cooldown: 150, vary: 0.1, also: [{ synth: 'boom', sg: 0.3, sr: 1.4 }] },
        portal_win: { sound: 'victory', gain: 0.8, cooldown: 500 },
        // ---- round 4: swinging ropes, puzzles, dragons ----
        swing_grab: { sound: false, gain: 0.5, cooldown: 150, synth: 'swinggrab', sg: 0.6 },
        swing_release: { sound: false, gain: 0.5, cooldown: 150, synth: 'swingwhoosh', sg: 0.5, sr: 1, scale: { key: 'speed', base: 0.7, per: 0.0015, min: 0.7, max: 1.6, gain: true } },
        seq_show: { sound: false, gain: 0.5, cooldown: 60, by: { key: 'glyph', map: GLYPH_TONES }, sg: 0.5 },
        seq_hit: { sound: false, gain: 0.5, cooldown: 60, by: { key: 'glyph', map: GLYPH_TONES }, sg: 0.65, also: [{ synth: 'ping', sg: 0.1, sr: 0.7 }] },
        seq_fail: { sound: false, gain: 0.5, cooldown: 300, synth: 'seqfail', sg: 0.55 },
        seq_solved: Object.assign({ cooldown: 500, synth: 'solved', sg: 0.6 }, SOLVED_FANFARE),
        bridge_done: Object.assign({ cooldown: 500, synth: 'solved', sg: 0.6, sr: 1.122 }, SOLVED_FANFARE),
        ritual_done: Object.assign({ cooldown: 500, synth: 'solved', sg: 0.6, sr: 0.89, also: [{ synth: 'shimmer', sg: 0.25, sr: 1, delay: 0.5 }] }, SOLVED_FANFARE),
        lever_pull: { sound: 'impact-metal', gain: 0.5, cooldown: 150, rate: 0.7, synth: 'clank', sg: 0.5, sr: 0.5, also: [{ synth: 'ratchet', sg: 0.35, sr: 0.9, delay: 0.04 }] },
        bridge_toggle: { sound: false, gain: 0.4, cooldown: 90, synth: 'magicping', sg: 0.5, by: { key: 'on', map: { true: { sr: 1.2 }, false: { sr: 0.7 }, '*': { sr: 1.2 } } } },
        plate_down: { sound: 'impact-stone', gain: 0.5, cooldown: 200, rate: 1.5, synth: 'impact', sg: 0.5, sr: 1.5 },
        stairs_up: { sound: 'impact-stone', gain: 0.35, cooldown: 400, rate: 0.6, synth: 'stairs', sg: 0.7, sr: 1.3 },
        stairs_down: { sound: 'impact-stone', gain: 0.35, cooldown: 400, rate: 0.6, synth: 'stairs', sg: 0.7, sr: 0.75 },
        brazier_lit: { sound: 'fire', gain: 0.3, cooldown: 120, rate: 1.3, synth: 'ignite', sg: 0.6, scale: { key: 'order', base: 1, per: 0.12, min: 0.8, max: 2 } },
        ritual_fail: { sound: false, gain: 0.5, cooldown: 400, synth: 'fizzle', sg: 0.55, also: [{ synth: 'seqfail', sg: 0.25, sr: 0.8 }] },
        dragon_roar: {
            sound: 'thunder', gain: 0.55, cooldown: 800, rate: 0.6, synth: 'dragonroar', sg: 1, also: [{ synth: 'boom', sg: 0.5, sr: 0.6 }],
            by: { key: 'kind', map: { frost: { synth: 'dragonroar_frost', sound: false } } }
        },
        dragon_warn: { sound: false, gain: 0.5, cooldown: 1000, synth: 'dragonwarn', sg: 0.7 },
        dragon_breath: {
            sound: 'fire', gain: 0.7, cooldown: 600, rate: 0.7, synth: 'dragonbreath', sg: 0.9,
            by: { key: 'kind', map: { frost: { sound: false, synth: 'dragonbreath_frost' } } }
        },
        dragon_pass: { sound: false, gain: 0.5, cooldown: 800, synth: 'dragonpass', sg: 0.8 },
        fireball_hit: { sound: 'explosion-small', gain: 0.55, cooldown: 100, rate: 0.9, vary: 0.1, synth: 'puff', sg: 0.5, sr: 0.8 },
        // ---- NPC encounters: dialogue, trade, reputation ----
        npc_near: { sound: false, gain: 0.3, cooldown: 600, synth: 'ping', sg: 0.35, sr: 1.5 },
        npc_talk: { sound: 'ui-click', gain: 0.4, cooldown: 200, rate: 1.1, synth: 'magicping', sg: 0.35, sr: 1.1 },
        npc_close: { sound: 'ui-click', gain: 0.3, cooldown: 200, rate: 0.8, synth: 'magicping', sg: 0.25, sr: 0.7 },
        // Typewriter text: a soft, tiny page blip at a low cooldown; the cue hash varies the pitch per call.
        npc_text: { sound: false, gain: 0.2, cooldown: 75, synth: 'chime', sg: 0.1, sr: 2.6, vary: 0.2 },
        npc_choice: { sound: 'ui-click', gain: 0.45, cooldown: 90, rate: 1.25, vary: 0.05 },
        npc_rep: {
            sound: false, gain: 0.5, cooldown: 250,
            by: {
                key: 'dir',
                map: {
                    up: { synth: 'chime', sg: 0.35, sr: 1.5, also: [{ synth: 'shimmer', sg: 0.3, sr: 1.3, delay: 0.08 }] },
                    down: { synth: 'down', sg: 0.4, sr: 0.8, also: [{ synth: 'boom', sg: 0.45, sr: 1.6 }] },
                    '*': { synth: 'shimmer', sg: 0.3, sr: 1 }
                }
            }
        },
        npc_trade: { sound: 'pickup', gain: 0.5, cooldown: 120, rate: 1.1, synth: 'chime', sg: 0.35, sr: 1.8, also: [{ synth: 'chime', sg: 0.25, sr: 2.4, delay: 0.07 }] },
        npc_quest: { sound: false, gain: 0.5, cooldown: 300, by: { key: 'state', map: QUEST_JINGLES } },
        quest_pickup: { sound: 'pickup', gain: 0.5, cooldown: 100, rate: 1.1, synth: 'shimmer', sg: 0.3, sr: 1.2 },
        npc_provoke: { sound: 'impact-stone', gain: 0.55, cooldown: 300, rate: 0.9, synth: 'impact', sg: 0.5, sr: 0.8 },
        // ---- close-up duel (events carry no x: they play centred) ----
        duel_start: {
            sound: 'thunder', gain: 0.55, cooldown: 600, rate: 0.8, synth: 'warn', sg: 0.3, sr: 0.6,
            also: [{ synth: 'boom', sg: 0.6, sr: 0.7, delay: 0.1 }]
        },
        duel_tell: { sound: false, gain: 0.45, cooldown: 250, by: { key: 'pattern', map: DUEL_TELLS } },
        duel_attack: { sound: false, gain: 0.6, cooldown: 120, by: { key: 'pattern', map: DUEL_ATTACKS } },
        duel_hit: {
            sound: false, gain: 0.6, cooldown: 70, vary: 0.14,
            by: {
                key: 'target',
                map: {
                    hero: { sound: 'impact-flesh', gain: 0.7, synth: 'impact', sg: 0.4, sr: 1.1 },
                    npc: { sound: false, synth: 'impact', sg: 0.55, sr: 1.3, also: [{ synth: 'crack', sg: 0.2, sr: 1.2 }] },
                    '*': { sound: false, synth: 'impact', sg: 0.45, sr: 1.2 }
                }
            }
        },
        duel_block: {
            sound: 'impact-metal', gain: 0.6, cooldown: 90, vary: 0.1, synth: 'clang', sg: 0.55,
            by: { key: 'by', map: { npc: { sr: 0.8 }, hero: { sr: 1.1 } } }
        },
        duel_roll: { sound: false, gain: 0.4, cooldown: 200, synth: 'whoosh', sg: 0.35, sr: 1.1 },
        duel_phase: { sound: 'thunder', gain: 0.7, cooldown: 400, synth: 'boom', sg: 0.6, sr: 0.8, also: [{ synth: 'roar', sg: 0.3, sr: 0.9, delay: 0.1 }] },
        duel_special: { sound: false, gain: 0.7, cooldown: 200, by: { key: 'kind', map: RUNE_PICKUPS } },
        duel_end: {
            sound: false, gain: 0.7, cooldown: 500,
            by: {
                key: 'result',
                map: {
                    won: { synth: 'solved', sg: 0.6, sound: 'pickup', rate: 1.2, gain: 0.4, also: [{ synth: 'shimmer', sg: 0.3, sr: 1, delay: 0.5 }] },
                    lost: { sound: 'defeat', gain: 0.65 },
                    '*': { sound: false, synth: 'down', sg: 0.4 }
                }
            }
        }
    };

    function validateSoundManifest(manifest) {
        if (!manifest || !Array.isArray(manifest.assets)) {
            throw new Error('runenpfad audio manifest missing');
        }
        const byId = {};
        manifest.assets.forEach(function (asset) {
            if (asset && asset.id && Array.isArray(asset.files) && asset.files[0] && asset.files[0].file) {
                byId[asset.id] = asset;
            }
        });
        REQUIRED_SOUNDS.forEach(function (id) {
            if (!byId[id]) {
                throw new Error('runenpfad audio missing ' + id);
            }
        });
        return byId;
    }

    // Deterministic [0,1) hash of (cueId, counter) for pitch variation.
    function hashUnit(cueId, counter) {
        const s = String(cueId) + ':' + counter;
        let h = 2166136261 >>> 0;
        for (let i = 0; i < s.length; i++) {
            h ^= s.charCodeAt(i);
            h = Math.imul(h, 16777619) >>> 0;
        }
        h ^= h >>> 15;
        h = Math.imul(h, 2246822519) >>> 0;
        h ^= h >>> 13;
        return (h >>> 0) / 4294967296;
    }

    function panFor(x, camera) {
        if (typeof x !== 'number' || !camera || !(camera.w > 0)) return 0;
        const half = camera.w / 2;
        return Math.max(-1, Math.min(1, (x - (camera.x + half)) / half)) * 0.7;
    }

    function buildImpulse(ac, biome) {
        const spec = REVERBS[biome] || REVERBS.woodland;
        const sr = ac.sampleRate;
        const len = Math.floor(spec.seconds * sr);
        const buf = ac.createBuffer(2, len, sr);
        for (let ch = 0; ch < 2; ch++) {
            const data = buf.getChannelData(ch);
            let seed = Math.floor(hashUnit(biome, ch) * 4294967295) >>> 0;
            let lp = 0;
            for (let i = 0; i < len; i++) {
                seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0;
                const n = seed / 2147483648 - 1;
                const k = i / len;
                lp += (n - lp) * (0.65 - 0.52 * k);
                data[i] = lp * Math.exp(-5.2 * k) * (1 - k);
            }
        }
        return buf;
    }

    // Procedural layers. Each one builds its own nodes into `dest` and disconnects when finished.
    const SYNTHS = {
        whoosh: function (ac, noise, dest, t, o) {
            const dur = 0.16 / o.sr;
            const src = ac.createBufferSource();
            src.buffer = noise;
            const bp = ac.createBiquadFilter();
            bp.type = 'bandpass';
            bp.Q.value = 1.3;
            bp.frequency.setValueAtTime(600 * o.sr, t);
            bp.frequency.exponentialRampToValueAtTime(2400 * o.sr, t + dur);
            const g = ac.createGain();
            g.gain.setValueAtTime(0.0001, t);
            g.gain.linearRampToValueAtTime(o.gain, t + dur * 0.35);
            g.gain.exponentialRampToValueAtTime(0.0001, t + dur);
            src.connect(bp);
            bp.connect(g);
            g.connect(dest);
            src.start(t, o.v * 0.5);
            src.stop(t + dur + 0.02);
            return [src, [src, bp, g]];
        },
        zap: function (ac, noise, dest, t, o) {
            const dur = 0.12;
            const osc = ac.createOscillator();
            osc.type = 'square';
            osc.frequency.setValueAtTime(2200 * o.sr, t);
            osc.frequency.exponentialRampToValueAtTime(520 * o.sr, t + dur);
            const bp = ac.createBiquadFilter();
            bp.type = 'bandpass';
            bp.frequency.value = 1500 * o.sr;
            bp.Q.value = 2;
            const g = ac.createGain();
            g.gain.setValueAtTime(o.gain * 0.4, t);
            g.gain.exponentialRampToValueAtTime(0.0001, t + dur);
            osc.connect(bp);
            bp.connect(g);
            g.connect(dest);
            osc.start(t);
            osc.stop(t + dur + 0.02);
            return [osc, [osc, bp, g]];
        },
        boing: function (ac, noise, dest, t, o) {
            const dur = 0.2;
            const osc = ac.createOscillator();
            osc.type = 'sine';
            osc.frequency.setValueAtTime(260 * o.sr, t);
            osc.frequency.exponentialRampToValueAtTime(760 * o.sr, t + dur * 0.7);
            const g = ac.createGain();
            g.gain.setValueAtTime(o.gain * 0.6, t);
            g.gain.exponentialRampToValueAtTime(0.0001, t + dur);
            osc.connect(g);
            g.connect(dest);
            osc.start(t);
            osc.stop(t + dur + 0.02);
            return [osc, [osc, g]];
        },
        warn: function (ac, noise, dest, t, o) {
            const dur = 0.5;
            const osc = ac.createOscillator();
            osc.type = 'triangle';
            osc.frequency.setValueAtTime(220 * o.sr * 2, t);
            osc.frequency.linearRampToValueAtTime(440 * o.sr * 2, t + dur);
            const g = ac.createGain();
            g.gain.setValueAtTime(0.0001, t);
            g.gain.linearRampToValueAtTime(o.gain * 0.5, t + dur * 0.6);
            g.gain.exponentialRampToValueAtTime(0.0001, t + dur);
            osc.connect(g);
            g.connect(dest);
            osc.start(t);
            osc.stop(t + dur + 0.02);
            return [osc, [osc, g]];
        },
        chime: function (ac, noise, dest, t, o) {
            const dur = 0.55;
            const f = (o.freq || 1046) * o.sr;
            const osc = ac.createOscillator();
            osc.type = 'sine';
            osc.frequency.value = f;
            const osc2 = ac.createOscillator();
            osc2.type = 'sine';
            osc2.frequency.value = f * 2.01;
            const g2 = ac.createGain();
            g2.gain.value = 0.25;
            const g = ac.createGain();
            g.gain.setValueAtTime(0.0001, t);
            g.gain.linearRampToValueAtTime(o.gain * 0.5, t + 0.006);
            g.gain.exponentialRampToValueAtTime(0.0001, t + dur);
            osc.connect(g);
            osc2.connect(g2);
            g2.connect(g);
            g.connect(dest);
            osc.start(t);
            osc2.start(t);
            osc.stop(t + dur + 0.02);
            osc2.stop(t + dur + 0.02);
            return [osc, [osc, osc2, g2, g]];
        },
        combo: function (ac, noise, dest, t, o) {
            const step = COIN_LADDER[Math.max(0, Math.min(COIN_LADDER.length - 1, (o.count || 2) - 2))];
            return SYNTHS.chime(ac, noise, dest, t, { gain: o.gain, sr: Math.pow(2, step / 12), freq: 523, v: o.v });
        }
    };

    function playFallbackTone(audioCtx, id, volume, outNode) {
        if (!audioCtx || audioCtx.state !== 'running') return;
        try {
            const now = audioCtx.currentTime;
            const osc = audioCtx.createOscillator();
            const gain = audioCtx.createGain();
            let fStart = 320;
            let fEnd = 220;
            let dur = 0.08;
            let type = 'triangle';
            if (id === 'jump') { fStart = 240; fEnd = 520; dur = 0.11; }
            else if (id === 'land') { fStart = 150; fEnd = 75; dur = 0.06; }
            else if (id === 'pickup') { fStart = 587; fEnd = 880; dur = 0.12; type = 'sine'; }
            else if (id === 'laser' || id === 'electric-spark') { fStart = 700; fEnd = 1200; dur = 0.13; type = 'sine'; }
            else if (id === 'rifle' || id === 'shotgun') { fStart = 460; fEnd = 180; dur = 0.09; type = 'sawtooth'; }
            else if (id === 'victory') { fStart = 523; fEnd = 1046; dur = 0.25; type = 'sine'; }
            else if (id === 'defeat') { fStart = 280; fEnd = 110; dur = 0.25; }
            else if (id === 'ui-click') { fStart = 660; fEnd = 440; dur = 0.04; type = 'sine'; }
            else if (id.indexOf('step-') === 0) { fStart = 130; fEnd = 90; dur = 0.035; }
            osc.type = type;
            osc.frequency.setValueAtTime(fStart, now);
            osc.frequency.exponentialRampToValueAtTime(Math.max(40, fEnd), now + dur);
            const amp = Math.max(0.02, Math.min(0.28, (volume || 0.4) * 0.25));
            gain.gain.setValueAtTime(amp, now);
            gain.gain.exponentialRampToValueAtTime(0.001, now + dur);
            osc.connect(gain);
            gain.connect(outNode || audioCtx.destination);
            osc.start(now);
            osc.stop(now + dur + 0.01);
            osc.onended = function () { try { osc.disconnect(); gain.disconnect(); } catch (err) { /* gone */ } };
        } catch (err) { /* fallback tone suppressed */ }
    }

    function createAudioController(options) {
        options = options || {};
        const assetsById = validateSoundManifest(options.manifest);
        const signal = options.signal;
        const base = options.baseUrl || BASE_URL;

        let enabled = options.enabled !== undefined ? !!options.enabled : true;
        let active = true;
        let disposed = false;
        let ctx = null;
        let master = null;
        let sfxBus = null;
        let ambBus = null;
        let musicBus = null;
        let reverbIn = null;
        let reverbOut = null;
        let convolver = null;
        let noiseBuf = null;
        let biome = 'woodland';
        let ambienceSpec = { primary: '', secondary: '', tertiary: '' };
        let ambienceDuck = 1;
        let liveSynths = 0;
        let coinStep = -1;
        let lastCoinAt = -9;
        const layers = [];
        const buffers = new Map();
        const pendingLoads = new Map();
        const impulses = new Map();
        const lastPlay = new Map();
        const counters = new Map();

        function ensureContext() {
            if (disposed || !enabled) return null;
            if (!ctx) {
                const AudioCtx = root.AudioContext || root.webkitAudioContext;
                if (!AudioCtx) return null;
                ctx = new AudioCtx();
                // Buses -> master -> glue compressor -> brickwall-ish limiter.
                master = ctx.createGain();
                master.gain.value = 0.92;
                const glue = ctx.createDynamicsCompressor();
                glue.threshold.value = -14;
                glue.knee.value = 22;
                glue.ratio.value = 7;
                glue.attack.value = 0.004;
                glue.release.value = 0.2;
                const limiter = ctx.createDynamicsCompressor();
                limiter.threshold.value = -3;
                limiter.knee.value = 0;
                limiter.ratio.value = 20;
                limiter.attack.value = 0.001;
                limiter.release.value = 0.08;
                master.connect(glue);
                glue.connect(limiter);
                limiter.connect(ctx.destination);
                sfxBus = ctx.createGain();
                ambBus = ctx.createGain();
                musicBus = ctx.createGain();
                musicBus.gain.value = 0.45;
                [sfxBus, ambBus, musicBus].forEach(function (b) { b.connect(master); });
                reverbIn = ctx.createGain();
                reverbOut = ctx.createGain();
                convolver = ctx.createConvolver();
                reverbIn.connect(convolver);
                convolver.connect(reverbOut);
                reverbOut.connect(master);
                const sfxSend = ctx.createGain();
                sfxSend.gain.value = 0.55;
                sfxBus.connect(sfxSend);
                sfxSend.connect(reverbIn);
                noiseBuf = ctx.createBuffer(1, ctx.sampleRate, ctx.sampleRate);
                const data = noiseBuf.getChannelData(0);
                let seed = 0x9E3779B9;
                for (let i = 0; i < data.length; i++) {
                    seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0;
                    data[i] = seed / 2147483648 - 1;
                }
                applyBiome(biome);
            }
            if (ctx.state === 'suspended') {
                ctx.resume().catch(function () {});
            }
            return ctx;
        }

        function applyBiome(next) {
            biome = REVERBS[next] ? next : 'woodland';
            if (!ctx || !convolver) return;
            if (!impulses.has(biome)) impulses.set(biome, buildImpulse(ctx, biome));
            convolver.buffer = impulses.get(biome);
            reverbOut.gain.setTargetAtTime(REVERBS[biome].wet, ctx.currentTime, 0.2);
        }

        function outNode() {
            return sfxBus || (ctx ? ctx.destination : null);
        }

        async function loadBuffer(id) {
            if (disposed || !assetsById[id]) return null;
            if (buffers.has(id)) return buffers.get(id);
            if (pendingLoads.has(id)) return pendingLoads.get(id);
            const audioCtx = ensureContext();
            if (!audioCtx) return null;

            const file = assetsById[id].files[0].file;
            const promise = fetch(base + file, { signal: signal, credentials: 'same-origin' })
                .then(function (resp) {
                    if (!resp.ok) throw new Error('audio fetch ' + id);
                    return resp.arrayBuffer();
                })
                .then(function (arr) {
                    if (disposed) return null;
                    return audioCtx.decodeAudioData(arr);
                })
                .then(function (decoded) {
                    if (decoded && !disposed) buffers.set(id, decoded);
                    pendingLoads.delete(id);
                    return decoded;
                })
                .catch(function () {
                    pendingLoads.delete(id);
                    return null;
                });
            pendingLoads.set(id, promise);
            return promise;
        }

        function killLayer(layer) {
            try { layer.source.stop(); } catch (err) { /* stopped */ }
            try { layer.source.disconnect(); } catch (err) { /* disconnected */ }
            try { layer.gain.disconnect(); } catch (err) { /* disconnected */ }
        }

        function fadeOutLayer(layer) {
            const idx = layers.indexOf(layer);
            if (idx >= 0) layers.splice(idx, 1);
            if (!ctx) { killLayer(layer); return; }
            try {
                const now = ctx.currentTime;
                layer.gain.gain.cancelScheduledValues(now);
                layer.gain.gain.setValueAtTime(layer.gain.gain.value, now);
                layer.gain.gain.linearRampToValueAtTime(0, now + FADE_SECONDS);
                layer.source.onended = function () { killLayer(layer); };
                layer.source.stop(now + FADE_SECONDS + 0.05);
            } catch (err) { killLayer(layer); }
        }

        function stopAmbience() {
            layers.splice(0).forEach(killLayer);
        }

        function ambienceLevel(id, scale) {
            if (!id || !assetsById[id]) return 0;
            return (Number(assetsById[id].gain) || 0.6) * 0.42 * scale;
        }

        function applyAmbienceGain() {
            if (!ctx) return;
            layers.forEach(function (layer) {
                layer.gain.gain.cancelScheduledValues(ctx.currentTime);
                layer.gain.gain.setTargetAtTime(ambienceLevel(layer.id, layer.scale) * ambienceDuck, ctx.currentTime, 0.15);
            });
        }

        function startAmbienceLayer(id, scale, slot) {
            const audioCtx = ensureContext();
            if (!audioCtx) return;
            loadBuffer(id).then(function (buffer) {
                if (!buffer || disposed || !enabled || !active) return;
                if (ambienceSpec.primary !== id && ambienceSpec.secondary !== id && ambienceSpec.tertiary !== id) return;
                layers.filter(function (l) { return l.slot === slot; }).forEach(fadeOutLayer);
                const source = audioCtx.createBufferSource();
                source.buffer = buffer;
                source.loop = true;
                const gain = audioCtx.createGain();
                const now = audioCtx.currentTime;
                gain.gain.setValueAtTime(0, now);
                gain.gain.linearRampToValueAtTime(ambienceLevel(id, scale) * ambienceDuck, now + FADE_SECONDS);
                source.connect(gain);
                gain.connect(ambBus || outNode());
                source.start(0);
                layers.push({ id: id, slot: slot, scale: scale, source: source, gain: gain });
            });
        }

        // Accepts a plain id or { primary, secondary } for layered biome ambience.
        async function setAmbience(spec) {
            const next = typeof spec === 'string' || !spec
                ? { primary: spec || '', secondary: '' }
                : { primary: spec.primary || '', secondary: spec.secondary || '', tertiary: spec.tertiary || '' };
            // The cavern gets a very quiet light-rain bed that reads as dripping water.
            if (!next.tertiary && DRIP_BEDS[BIOME_BY_AMBIENCE[next.primary]]) next.tertiary = DRIP_BEDS[BIOME_BY_AMBIENCE[next.primary]];
            next.tertiary = next.tertiary || '';
            const prev = ambienceSpec;
            ambienceSpec = next;
            if (BIOME_BY_AMBIENCE[next.primary] && BIOME_BY_AMBIENCE[next.primary] !== biome) applyBiome(BIOME_BY_AMBIENCE[next.primary]);
            if (next.primary === prev.primary && next.secondary === prev.secondary && next.tertiary === prev.tertiary) return;
            // Layers whose sound is no longer wanted crossfade out; new ones fade in.
            layers.slice().forEach(function (layer) {
                if (layer.id !== next.primary && layer.id !== next.secondary && layer.id !== next.tertiary) fadeOutLayer(layer);
            });
            if (!enabled || !active || disposed) return;
            if (!next.primary) return;
            const has = function (id, slot) { return layers.some(function (l) { return l.id === id && l.slot === slot; }); };
            if (!has(next.primary, 1)) startAmbienceLayer(next.primary, 1, 1);
            if (next.secondary && !has(next.secondary, 2)) startAmbienceLayer(next.secondary, 0.55, 2);
            if (next.tertiary && !has(next.tertiary, 3)) startAmbienceLayer(next.tertiary, 0.3, 3);
        }

        // Ducks the ambience bed (used while a world boss is alive).
        function setAmbienceDuck(level) {
            ambienceDuck = Math.max(0, Math.min(1, Number(level) || 0));
            applyAmbienceGain();
        }

        function route(pan) {
            if (pan && ctx && typeof ctx.createStereoPanner === 'function') {
                const p = ctx.createStereoPanner();
                p.pan.value = pan;
                p.connect(outNode());
                return p;
            }
            return outNode();
        }

        function releaseRoute(node) {
            if (node && node !== sfxBus && (!ctx || node !== ctx.destination) && typeof node.pan !== 'undefined') {
                try { node.disconnect(); } catch (err) { /* gone */ }
            }
        }

        function playBuffer(audioCtx, buffer, id, gainScale, rate, pan) {
            const source = audioCtx.createBufferSource();
            source.buffer = buffer;
            if (rate && rate > 0) source.playbackRate.value = rate;
            const gain = audioCtx.createGain();
            gain.gain.value = Math.min(1, (Number(assetsById[id].gain) || 0.75) * (gainScale || 0.75));
            const dest = route(pan);
            source.connect(gain);
            gain.connect(dest);
            source.onended = function () {
                try { source.disconnect(); gain.disconnect(); } catch (err) { /* gone */ }
                releaseRoute(dest);
            };
            source.start(0);
        }

        // Cooldown gate on the audio clock (seconds); returns true when the cue may play.
        function gate(key, cooldownMs) {
            const now = ctx.currentTime;
            if (lastPlay.has(key) && lastPlay.get(key) + (cooldownMs || 65) / 1000 > now) return false;
            lastPlay.set(key, now);
            return true;
        }

        function playSample(id, gainScale, rate, pan) {
            const audioCtx = ctx;
            const cached = buffers.get(id);
            if (cached) {
                playBuffer(audioCtx, cached, id, gainScale, rate, pan);
                return;
            }
            const t0 = audioCtx.currentTime;
            playFallbackTone(audioCtx, id, gainScale, outNode());
            loadBuffer(id).then(function (buffer) {
                if (!buffer || disposed || !enabled || !ctx) return;
                if (ctx.currentTime - t0 < 0.25) playBuffer(ctx, buffer, id, gainScale, rate, pan);
            });
        }

        // Live synth anchors are capped; important cues (priority) may exceed the soft cap up to the hard cap.
        function playSynth(name, o, pan, priority) {
            if (liveSynths >= (priority ? SYNTH_HARD_CAP : SYNTH_CAP)) return;
            const fn = SYNTHS[name] || (root.RunenpfadSfx && root.RunenpfadSfx.SYNTHS[name]);
            if (!fn || !ctx || ctx.state !== 'running' || !noiseBuf) return;
            const dest = route(pan);
            try {
                const res = fn(ctx, noiseBuf, dest, ctx.currentTime + 0.005 + (o.delay || 0), o);
                liveSynths++;
                let done = false;
                res[0].onended = function () {
                    if (!done) { done = true; liveSynths = Math.max(0, liveSynths - 1); }
                    res[1].forEach(function (n) { try { n.disconnect(); } catch (err) { /* gone */ } });
                    releaseRoute(dest);
                };
            } catch (err) { releaseRoute(dest); }
        }

        function variation(key, amount) {
            if (!amount) return 1;
            const n = counters.get(key) || 0;
            counters.set(key, n + 1);
            return 1 + (hashUnit(key, n) * 2 - 1) * amount;
        }

        // Plays an arbitrary sample cue (used by the UI click); respects mute and the cooldown.
        function playCue(id, gainScale, cooldownMs, allowWhilePaused, rate) {
            if (disposed || !enabled || (!active && !allowWhilePaused) || !assetsById[id]) return;
            if (!ensureContext() || !gate('cue:' + id, cooldownMs)) return;
            playSample(id, gainScale, rate, 0);
        }

        function unlock() {
            if (disposed || !enabled) return;
            const audioCtx = ensureContext();
            if (!audioCtx) return;
            if (audioCtx.state === 'suspended') {
                audioCtx.resume().then(function () {
                    if (active && ambienceSpec.primary && !layers.length) {
                        restartAmbience();
                    }
                }).catch(function () {});
            }
            PRELOAD_SFX.forEach(function (id) { loadBuffer(id); });
            if (active && ambienceSpec.primary && !layers.length) {
                restartAmbience();
            }
        }

        function restartAmbience() {
            const spec = ambienceSpec;
            ambienceSpec = { primary: '', secondary: '', tertiary: '' };
            setAmbience(spec);
        }

        function resolveCue(ev, cue, d) {
            let out = cue;
            if (cue.by) {
                const k = cue.by.key === '@biome' ? biome : (d ? d[cue.by.key] : undefined);
                const over = cue.by.map[k] || cue.by.map['*'];
                if (over) out = Object.assign({}, cue, over);
            }
            if (cue.byEnemy && d) {
                const kind = d.enemy || d.defId || d.enemyKind || d.def || d.type;
                if (typeof kind === 'string' && cue.byEnemy[kind]) out = Object.assign({}, out, cue.byEnemy[kind]);
            }
            return out;
        }

        // Dragon bosses speak with the dragon roar instead of the biome voice (by pattern or definition id).
        function dragonVoice(d) {
            if (!d) return null;
            const id = String(d.defId || d.boss || d.enemy || d.id || '');
            const isDragon = /dragon/i.test(id) || (typeof d.pattern === 'string' && DRAGON_PATTERNS[d.pattern] === 1);
            if (!isDragon) return null;
            return /frost/i.test(id) || d.kind === 'frost' ? DRAGON_VOICES.frost : DRAGON_VOICES.ember;
        }

        // Extra synth layers and the per-biome boss voice of a resolved cue.
        function playExtras(cue, key, d, pan) {
            if (liveSynths >= SYNTH_CAP) return;
            const v = hashUnit(key, counters.get(key) || 0);
            (cue.also || []).forEach(function (a) {
                playSynth(a.synth, { gain: a.sg || 0.4, sr: a.sr || 1, v: v, delay: a.delay || 0, count: d && d.count }, pan);
            });
            if (cue.voice) {
                (dragonVoice(d) || BOSS_VOICES[biome] || BOSS_VOICES.woodland).forEach(function (bv) {
                    playSynth(bv.synth, { gain: cue.voice.gain || 0.5, sr: bv.sr * (cue.voice.sr || 1), v: v, short: !!cue.voice.short }, pan);
                });
            }
        }

        function handleEvents(events, worldAmbience, camera) {
            if (Array.isArray(events) && events.length && !disposed && enabled && active) {
                const audioCtx = ensureContext();
                for (let i = 0; audioCtx && i < events.length; i++) {
                    const ev = events[i];
                    const base = ev && CUE_MAP[ev.type];
                    if (!base) continue;
                    const d = ev.detail || null;
                    if (ev.type === 'enemy_kill' && d && d.boss) continue;
                    const cue = resolveCue(ev, base, d);
                    const key = ev.type + ':' + ((d && (d.pattern || d.kind || d.material || d.enemy || d.defId || d.target || d.by || d.result || (typeof d.glyph === 'number' ? 'g' + d.glyph : ''))) || '');
                    if (!gate(key, cue.cooldown)) continue;
                    let id = cue.sound;
                    if (id === null) {
                        id = (d && d.sound) || null;
                        if (!id && ev.type === 'step') id = 'step-grass';
                        if (!id && ev.type === 'player_hurt') id = 'impact-flesh';
                    }
                    const pan = panFor(d && (d.x !== undefined ? d.x : d.x1), camera);
                    let rate = (cue.rate || 1) * variation(key, cue.vary);
                    let sr = cue.sr || 1;
                    if (cue.ladder) {
                        if (audioCtx.currentTime - lastCoinAt > COIN_RESET_SECONDS) coinStep = 0;
                        else coinStep = Math.min(COIN_LADDER.length - 1, coinStep + 1);
                        lastCoinAt = audioCtx.currentTime;
                        const ratio = Math.pow(2, COIN_LADDER[coinStep] / 12);
                        rate *= ratio;
                        sr = ratio;
                    }
                    let sg = cue.sg || 0.4;
                    if (cue.scale && d && typeof d[cue.scale.key] === 'number') {
                        const k = Math.max(cue.scale.min, Math.min(cue.scale.max, cue.scale.base + cue.scale.per * d[cue.scale.key]));
                        sr *= k;
                        if (cue.scale.gain) sg *= 0.6 + 0.4 * k;
                    }
                    if (id && assetsById[id]) playSample(id, cue.gain, rate, pan);
                    if (cue.synth) playSynth(cue.synth, { gain: sg, sr: sr, v: hashUnit(key, counters.get(key) || 0), count: d && d.count, freq: cue.ladder ? 1046 : 0 }, pan, /^(boss_|dragon_)/.test(ev.type) || ev.type === 'player_hurt' || ev.type === 'enemy_kill');
                    if (cue.also || cue.voice) playExtras(cue, key, d, pan);
                }
            }
            if (worldAmbience) {
                const spec = typeof worldAmbience === 'string' ? { primary: worldAmbience, secondary: '' } : worldAmbience;
                if ((spec.primary !== ambienceSpec.primary || spec.secondary !== ambienceSpec.secondary) && enabled && active) {
                    setAmbience(spec);
                }
            }
        }

        function syncPlaybackState() {
            if (disposed) return;
            if (!enabled) {
                stopAmbience();
                if (ctx && ctx.state === 'running') {
                    ctx.suspend().catch(function () {});
                }
            } else if (!active) {
                stopAmbience();
            } else {
                ensureContext();
                if (ambienceSpec.primary && !layers.length) {
                    restartAmbience();
                }
            }
        }

        function setEnabled(next) {
            enabled = !!next;
            syncPlaybackState();
            if (enabled) {
                unlock();
                playCue('ui-click', 0.5, 50, true);
            }
        }

        function setActive(next) {
            active = !!next;
            syncPlaybackState();
        }

        function dispose() {
            if (disposed) return;
            disposed = true;
            liveSynths = 0;
            stopAmbience();
            buffers.clear();
            pendingLoads.clear();
            impulses.clear();
            const closing = ctx;
            ctx = null;
            sfxBus = ambBus = musicBus = reverbIn = reverbOut = convolver = master = noiseBuf = null;
            if (closing) {
                closing.close().catch(function () {});
            }
        }

        // Lazy port for the music controller: the context only exists after the first unlock.
        const port = {
            get ctx() { return ctx; },
            get bus() { return musicBus; },
            get reverb() { return reverbIn; }
        };

        return {
            isEnabled: function () { return enabled; },
            setEnabled: setEnabled,
            setActive: setActive,
            unlock: unlock,
            setAmbience: setAmbience,
            setAmbienceDuck: setAmbienceDuck,
            playUiClick: function () { playCue('ui-click', 0.5, 50, true); },
            handleEvents: handleEvents,
            liveSynths: function () { return liveSynths; },
            musicPort: function () { return port; },
            dispose: dispose
        };
    }

    root.RunenpfadAudio = {
        BASE_URL: BASE_URL,
        REQUIRED_SOUNDS: REQUIRED_SOUNDS,
        CUE_MAP: CUE_MAP,
        BOSS_VOICES: BOSS_VOICES,
        COIN_LADDER: COIN_LADDER,
        REVERBS: REVERBS,
        hasSynth: function (name) { return !!(SYNTHS[name] || (root.RunenpfadSfx && root.RunenpfadSfx.SYNTHS[name])); },
        hashUnit: hashUnit,
        panFor: panFor,
        validateSoundManifest: validateSoundManifest,
        createAudioController: createAudioController
    };
})(typeof window !== 'undefined' ? window : globalThis);
