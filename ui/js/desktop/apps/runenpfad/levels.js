/* Runenpfad world, biome, enemy, hero, upgrade and 25-level campaign definitions. DOM-free. */
(function (root) {
    'use strict';

    const TILE = 48;

    const REQUIRED_PACK_IDS = [
        'jump-and-run',
        'human-characters-animated',
        'monsters-animals-animated',
        'robots-drones-animated',
        'assets-animated',
        'nature-assets'
    ];

    const PACK_SPECS = {
        'jump-and-run': {
            assets: [
                'grass_block', 'grass_ledge_left', 'grass_ledge_bottom', 'grass_ledge_right', 'grass_island',
                'earth_block', 'earth_stones', 'earth_dark', 'earth_roots', 'earth_light', 'stone_round', 'stone_cracked', 'stone_dark', 'stone_moss',
                'sand_block', 'sand_waves', 'sand_pebbles', 'sand_bricks', 'sand_cracked', 'sand_ledge', 'sand_arch',
                'snow_block', 'snow_ledge', 'ice_stone', 'ice_block', 'ice_ledge', 'snow_island', 'ice_pillar',
                'wood_platform', 'rope_platform', 'log_platform', 'stone_platform', 'chain_platform', 'grass_platform', 'earth_platform',
                'red_mushroom_platform', 'violet_mushroom_platform', 'ice_platform',
                'barrel', 'crate', 'ladder', 'sign', 'lantern',
                'coin', 'red_gem', 'blue_gem', 'green_gem', 'key', 'heart', 'apple', 'potion', 'treasure', 'star',
                'spikes', 'stalactite', 'saw', 'lava', 'fire', 'thorn', 'trapdoor', 'cannon', 'magic_platform', 'spike_trap', 'stone_weight'
            ],
            animations: ['magic_platform', 'spike_trap']
        },
        'human-characters-animated': {
            assets: [
                'ranger_idle', 'ranger_walk', 'ranger_run', 'ranger_jump', 'ranger_attack', 'ranger_hurt', 'ranger_death',
                'knight_idle', 'knight_walk', 'knight_run', 'knight_jump', 'knight_attack', 'knight_hurt', 'knight_death'
            ],
            animations: [
                'ranger_idle', 'ranger_walk', 'ranger_run', 'ranger_jump', 'ranger_attack', 'ranger_hurt', 'ranger_death',
                'knight_idle', 'knight_walk', 'knight_run', 'knight_jump', 'knight_attack', 'knight_hurt', 'knight_death'
            ]
        },
        'monsters-animals-animated': {
            assets: [
                'slime_idle', 'slime_move', 'slime_attack', 'slime_hurt', 'slime_death',
                'goblin_idle', 'goblin_move', 'goblin_attack', 'goblin_hurt', 'goblin_death',
                'wolf_idle', 'wolf_move', 'wolf_attack', 'wolf_hurt', 'wolf_death',
                'boar_idle', 'boar_move', 'boar_attack', 'boar_hurt', 'boar_death'
            ],
            animations: [
                'slime_idle', 'slime_move', 'slime_attack', 'slime_hurt', 'slime_death',
                'goblin_idle', 'goblin_move', 'goblin_attack', 'goblin_hurt', 'goblin_death',
                'wolf_idle', 'wolf_move', 'wolf_attack', 'wolf_hurt', 'wolf_death',
                'boar_idle', 'boar_move', 'boar_attack', 'boar_hurt', 'boar_death'
            ]
        },
        'robots-drones-animated': {
            assets: [
                'service_robot_idle', 'service_robot_move', 'service_robot_attack', 'service_robot_hurt', 'service_robot_death',
                'combat_mech_idle', 'combat_mech_move', 'combat_mech_attack', 'combat_mech_hurt', 'combat_mech_death',
                'spider_drone_idle', 'spider_drone_move', 'spider_drone_attack', 'spider_drone_hurt', 'spider_drone_death',
                'quadcopter_idle', 'quadcopter_move', 'quadcopter_attack', 'quadcopter_hurt', 'quadcopter_death',
                'security_orb_idle', 'security_orb_move', 'security_orb_attack', 'security_orb_hurt', 'security_orb_death'
            ],
            animations: [
                'service_robot_idle', 'service_robot_move', 'service_robot_attack', 'service_robot_hurt', 'service_robot_death',
                'combat_mech_idle', 'combat_mech_move', 'combat_mech_attack', 'combat_mech_hurt', 'combat_mech_death',
                'spider_drone_idle', 'spider_drone_move', 'spider_drone_attack', 'spider_drone_hurt', 'spider_drone_death',
                'quadcopter_idle', 'quadcopter_move', 'quadcopter_attack', 'quadcopter_hurt', 'quadcopter_death',
                'security_orb_idle', 'security_orb_move', 'security_orb_attack', 'security_orb_hurt', 'security_orb_death'
            ]
        },
        'assets-animated': {
            assets: ['explosion', 'campfire', 'smoke', 'portal', 'coin_spin', 'chest_open', 'torch', 'door_open', 'spike_trap'],
            animations: ['explosion', 'campfire', 'smoke', 'portal', 'coin_spin', 'chest_open', 'torch', 'door_open', 'spike_trap']
        },
        'nature-assets': {
            assets: [
                'oak_tree', 'pine_tree', 'birch_tree', 'fern', 'daisies', 'poppies',
                'red_mushrooms', 'violet_mushrooms', 'blue_crystals', 'mossy_crystals', 'amber_crystals',
                'tall_cactus', 'round_cactus', 'desert_shrub', 'sandstone',
                'snowy_pine', 'snowy_bush', 'icy_rock', 'icicles', 'ice_crystal', 'bare_tree', 'mossy_boulder'
            ],
            animations: []
        }
    };

    function validateSpriteManifests(manifests) {
        if (!manifests || typeof manifests !== 'object') {
            throw new Error('runenpfad sprite manifests missing');
        }
        const indexed = {};
        REQUIRED_PACK_IDS.forEach(function (packId) {
            const manifest = manifests[packId];
            if (!manifest || !Array.isArray(manifest.frames) || !Array.isArray(manifest.assets)) {
                throw new Error('runenpfad invalid pack manifest: ' + packId);
            }
            if (manifest.frames.length < 100 || !manifest.image) {
                throw new Error('runenpfad incomplete sheet frames: ' + packId);
            }
            const framesByIndex = {};
            manifest.frames.forEach(function (frame) {
                if (typeof frame.index === 'number') {
                    framesByIndex[frame.index] = frame;
                }
            });
            const assetsById = {};
            manifest.assets.forEach(function (asset) {
                if (asset && asset.id) assetsById[asset.id] = asset;
            });
            const animationsById = {};
            (manifest.animations || []).forEach(function (anim) {
                if (anim && anim.id) animationsById[anim.id] = anim;
            });
            const spec = PACK_SPECS[packId];
            spec.assets.forEach(function (assetId) {
                const entry = assetsById[assetId];
                if (!entry || !Array.isArray(entry.frames) || entry.frames.length === 0) {
                    throw new Error('runenpfad missing asset ' + packId + '.' + assetId);
                }
            });
            spec.animations.forEach(function (animId) {
                const anim = animationsById[animId];
                if (!anim || !Array.isArray(anim.frames) || anim.frames.length === 0) {
                    throw new Error('runenpfad missing animation ' + packId + '.' + animId);
                }
            });
            indexed[packId] = {
                manifest: manifest,
                framesByIndex: framesByIndex,
                assetsById: assetsById,
                animationsById: animationsById
            };
        });
        return indexed;
    }

    const WORLDS = [
        {
            id: 'woodland',
            nameKey: 'runenpfad.world_1',
            skyTop: '#16354d',
            skyBottom: '#3c7a69',
            hillBack: '#1c4648',
            hillFront: '#255c4d',
            groundTop: 'grass_block',
            groundFill: 'earth_block',
            altFill: 'earth_stones',
            platform: 'wood_platform',
            spring: 'red_mushroom_platform',
            weather: 'leaf',
            ambience: 'forest-day',
            stepSound: 'step-grass',
            friction: 1.0,
            props: ['oak_tree', 'pine_tree', 'fern', 'daisies', 'poppies']
        },
        {
            id: 'cavern',
            nameKey: 'runenpfad.world_2',
            skyTop: '#141129',
            skyBottom: '#2d234a',
            hillBack: '#20193b',
            hillFront: '#2f2752',
            groundTop: 'stone_moss',
            groundFill: 'earth_dark',
            altFill: 'earth_roots',
            platform: 'earth_platform',
            spring: 'violet_mushroom_platform',
            weather: 'spore',
            ambience: 'forest-night',
            stepSound: 'step-stone',
            friction: 1.0,
            props: ['violet_mushrooms', 'blue_crystals', 'mossy_crystals', 'red_mushrooms']
        },
        {
            id: 'desert',
            nameKey: 'runenpfad.world_3',
            skyTop: '#3c1e34',
            skyBottom: '#c97141',
            hillBack: '#683938',
            hillFront: '#8c533c',
            groundTop: 'sand_block',
            groundFill: 'sand_bricks',
            altFill: 'sand_cracked',
            platform: 'rope_platform',
            spring: 'red_mushroom_platform',
            weather: 'sand',
            ambience: 'wind-strong',
            stepSound: 'step-gravel',
            friction: 1.0,
            props: ['tall_cactus', 'round_cactus', 'desert_shrub', 'sandstone']
        },
        {
            id: 'glacier',
            nameKey: 'runenpfad.world_4',
            skyTop: '#112945',
            skyBottom: '#6ea8c9',
            hillBack: '#264b6e',
            hillFront: '#38688f',
            groundTop: 'snow_block',
            groundFill: 'ice_stone',
            altFill: 'ice_block',
            platform: 'ice_platform',
            spring: 'violet_mushroom_platform',
            weather: 'snow',
            ambience: 'wind-gentle',
            stepSound: 'step-wood',
            friction: 0.36,
            props: ['snowy_pine', 'snowy_bush', 'icy_rock', 'ice_crystal']
        },
        {
            id: 'citadel',
            nameKey: 'runenpfad.world_5',
            skyTop: '#13131e',
            skyBottom: '#3e1e2b',
            hillBack: '#1f1b2c',
            hillFront: '#2e2538',
            groundTop: 'stone_cracked',
            groundFill: 'stone_dark',
            altFill: 'stone_round',
            platform: 'chain_platform',
            spring: 'red_mushroom_platform',
            weather: 'ember',
            ambience: 'industrial',
            stepSound: 'step-metal',
            friction: 1.0,
            props: ['bare_tree', 'amber_crystals', 'mossy_boulder']
        }
    ];

    const HERO_DEFS = {
        knight: {
            id: 'knight',
            nameKey: 'runenpfad.hero_knight',
            descKey: 'runenpfad.hero_knight_desc',
            pack: 'human-characters-animated',
            prefix: 'knight_',
            baseHp: 5,
            speed: 235,
            jumpVel: 565,
            maxAirJumps: 0,
            baseDamage: 2,
            attackCooldown: 0.34,
            attackDuration: 0.24,
            dashEnabled: false,
            slamEnabled: true
        },
        ranger: {
            id: 'ranger',
            nameKey: 'runenpfad.hero_ranger',
            descKey: 'runenpfad.hero_ranger_desc',
            pack: 'human-characters-animated',
            prefix: 'ranger_',
            baseHp: 4,
            speed: 270,
            jumpVel: 575,
            maxAirJumps: 1,
            baseDamage: 1,
            attackCooldown: 0.26,
            attackDuration: 0.20,
            dashEnabled: true,
            slamEnabled: false
        }
    };

    const UPGRADE_DEFS = [
        { id: 'vitality', nameKey: 'runenpfad.upgrade_vitality', descKey: 'runenpfad.upgrade_vitality_desc', maxLevel: 5, baseCost: 15, stepCost: 20 },
        { id: 'might', nameKey: 'runenpfad.upgrade_might', descKey: 'runenpfad.upgrade_might_desc', maxLevel: 5, baseCost: 20, stepCost: 25 },
        { id: 'boots', nameKey: 'runenpfad.upgrade_boots', descKey: 'runenpfad.upgrade_boots_desc', maxLevel: 5, baseCost: 18, stepCost: 22 },
        { id: 'alchemy', nameKey: 'runenpfad.upgrade_alchemy', descKey: 'runenpfad.upgrade_alchemy_desc', maxLevel: 5, baseCost: 14, stepCost: 18 }
    ];

    const ENEMY_DEFS = {
        slime: { id: 'slime', pack: 'monsters-animals-animated', prefix: 'slime_', hp: 2, speed: 62, damage: 1, behavior: 'patrol', score: 40, scale: 1.0, flying: false, ranged: false, hitSound: 'impact-flesh' },
        goblin: { id: 'goblin', pack: 'monsters-animals-animated', prefix: 'goblin_', hp: 3, speed: 92, damage: 1, behavior: 'patrol', score: 70, scale: 1.05, flying: false, ranged: false, hitSound: 'impact-flesh' },
        wolf: { id: 'wolf', pack: 'monsters-animals-animated', prefix: 'wolf_', hp: 3, speed: 132, damage: 1, behavior: 'charger', score: 90, scale: 1.05, flying: false, ranged: false, hitSound: 'impact-flesh' },
        boar: { id: 'boar', pack: 'monsters-animals-animated', prefix: 'boar_', hp: 5, speed: 108, damage: 1, behavior: 'charger', score: 120, scale: 1.15, flying: false, ranged: false, hitSound: 'impact-flesh' },
        service_robot: { id: 'service_robot', pack: 'robots-drones-animated', prefix: 'service_robot_', hp: 4, speed: 84, damage: 1, behavior: 'patrol', score: 110, scale: 1.05, flying: false, ranged: false, hitSound: 'impact-metal' },
        spider_drone: { id: 'spider_drone', pack: 'robots-drones-animated', prefix: 'spider_drone_', hp: 4, speed: 124, damage: 1, behavior: 'jumper', score: 130, scale: 1.0, flying: false, ranged: false, hitSound: 'impact-metal' },
        quadcopter: { id: 'quadcopter', pack: 'robots-drones-animated', prefix: 'quadcopter_', hp: 3, speed: 95, damage: 1, behavior: 'flyer', score: 140, scale: 1.0, flying: true, ranged: true, hitSound: 'impact-metal' },
        security_orb: { id: 'security_orb', pack: 'robots-drones-animated', prefix: 'security_orb_', hp: 4, speed: 80, damage: 1, behavior: 'flyer', score: 150, scale: 1.0, flying: true, ranged: true, hitSound: 'impact-metal' },
        combat_mech: { id: 'combat_mech', pack: 'robots-drones-animated', prefix: 'combat_mech_', hp: 7, speed: 78, damage: 2, behavior: 'shooter', score: 200, scale: 1.2, flying: false, ranged: true, hitSound: 'impact-metal' },
        sentry_bot: { id: 'sentry_bot', pack: 'robots-drones-animated', prefix: 'service_robot_', hp: 6, speed: 0, damage: 1, behavior: 'turret', score: 160, scale: 1.05, flying: false, ranged: true, hitSound: 'impact-metal' },
        bomber_drone: { id: 'bomber_drone', pack: 'robots-drones-animated', prefix: 'spider_drone_', hp: 3, speed: 150, damage: 2, behavior: 'bomber', score: 140, scale: 1.0, flying: false, ranged: false, hitSound: 'impact-metal' },
        stalker: { id: 'stalker', pack: 'monsters-animals-animated', prefix: 'wolf_', hp: 4, speed: 168, damage: 2, behavior: 'ambusher', score: 150, scale: 1.1, flying: false, ranged: false, hitSound: 'impact-flesh' },
        sporecap: { id: 'sporecap', art: 'enemy-sporecap', pack: 'monsters-animals-animated', prefix: 'slime_', hp: 3, speed: 40, damage: 1, behavior: 'spitter', score: 110, scale: 1.0, flying: false, ranged: false, hitSound: 'impact-flesh' },
        crystal_bat: { id: 'crystal_bat', art: 'enemy-crystalbat', pack: 'robots-drones-animated', prefix: 'security_orb_', hp: 2, speed: 110, damage: 1, behavior: 'swooper', score: 100, scale: 0.9, flying: true, ranged: false, hitSound: 'impact-metal' },
        scarab: { id: 'scarab', art: 'enemy-scarab', pack: 'monsters-animals-animated', prefix: 'boar_', hp: 4, speed: 78, damage: 1, behavior: 'shielded', score: 160, scale: 1.0, flying: false, ranged: false, hitSound: 'impact-metal' },
        frost_wraith: { id: 'frost_wraith', art: 'enemy-frostwraith', pack: 'robots-drones-animated', prefix: 'security_orb_', hp: 3, speed: 70, damage: 1, behavior: 'phaser', score: 150, scale: 1.0, flying: true, ranged: false, hitSound: 'impact-flesh' },
        clock_knight: { id: 'clock_knight', art: 'enemy-clockknight', pack: 'robots-drones-animated', prefix: 'combat_mech_', hp: 7, speed: 55, damage: 2, behavior: 'brute', score: 220, scale: 1.2, flying: false, ranged: false, hitSound: 'impact-metal' },
        drakeling: { id: 'drakeling', art: 'enemy-drakeling', pack: 'robots-drones-animated', prefix: 'quadcopter_', hp: 3, speed: 70, damage: 1, behavior: 'drakeling', score: 150, scale: 1.0, flying: true, ranged: false, hitSound: 'impact-flesh' },
        boss_boar: { id: 'boss_boar', art: 'boss-boar', bossNameKey: 'runenpfad.boss_1', pack: 'monsters-animals-animated', prefix: 'boar_', hp: 20, speed: 135, damage: 1, behavior: 'boss_ground', score: 600, scale: 2.0, flying: false, ranged: false, boss: true, hitSound: 'impact-flesh' },
        boss_goblin: { id: 'boss_goblin', art: 'boss-goblin', bossNameKey: 'runenpfad.boss_2', pack: 'monsters-animals-animated', prefix: 'goblin_', hp: 26, speed: 125, damage: 1, behavior: 'boss_jumper', score: 800, scale: 1.95, flying: false, ranged: true, boss: true, hitSound: 'impact-flesh' },
        boss_orb: { id: 'boss_orb', art: 'boss-orb', bossNameKey: 'runenpfad.boss_3', pack: 'robots-drones-animated', prefix: 'security_orb_', hp: 32, speed: 115, damage: 1, behavior: 'boss_flyer', score: 1000, scale: 2.0, flying: true, ranged: true, boss: true, hitSound: 'impact-metal' },
        boss_quad: { id: 'boss_quad', art: 'boss-quad', bossNameKey: 'runenpfad.boss_4', pack: 'robots-drones-animated', prefix: 'quadcopter_', hp: 38, speed: 130, damage: 1, behavior: 'boss_flyer', score: 1200, scale: 2.0, flying: true, ranged: true, boss: true, hitSound: 'impact-metal' },
        boss_mech: { id: 'boss_mech', art: 'boss-mech', bossNameKey: 'runenpfad.boss_5', pack: 'robots-drones-animated', prefix: 'combat_mech_', hp: 48, speed: 110, damage: 2, behavior: 'boss_ground', score: 1800, scale: 2.2, flying: false, ranged: true, boss: true, hitSound: 'impact-metal' },
        boss_dragon_ember: { id: 'boss_dragon_ember', art: 'dragon-ember', bossNameKey: 'runenpfad.boss_dragon_ember', dragon: 'ember', pack: 'robots-drones-animated', prefix: 'quadcopter_', hp: 60, speed: 130, damage: 1, behavior: 'boss_flyer', score: 2200, scale: 3.2, flying: true, ranged: false, boss: true, hitSound: 'impact-flesh' },
        boss_dragon_frost: { id: 'boss_dragon_frost', art: 'dragon-frost', bossNameKey: 'runenpfad.boss_dragon_frost', dragon: 'frost', pack: 'robots-drones-animated', prefix: 'quadcopter_', hp: 60, speed: 130, damage: 1, behavior: 'boss_flyer', score: 2200, scale: 3.2, flying: true, ranged: false, boss: true, hitSound: 'impact-flesh' }
    };

    // 25 campaign stages (5 worlds x 5 stages). Every 5th stage culminates in a Boss Arena.
    const CAMPAIGN_LEVELS = [
        { id: 1, worldIndex: 0, stageInWorld: 1, nameKey: 'runenpfad.level_1', parTime: 75, zones: ['meadow', 'step_ledge', 'crate_yard', 'key_sanctum'], enemies: ['slime', 'slime', 'goblin'] },
        { id: 2, worldIndex: 0, stageInWorld: 2, nameKey: 'runenpfad.level_2', parTime: 100, zones: ['meadow', 'bridge_gap', 'mushroom_canopy', 'rune_sequence:vault', 'key_sanctum'], enemies: ['slime', 'goblin', 'wolf'] },
        { id: 3, worldIndex: 0, stageInWorld: 3, nameKey: 'runenpfad.level_3', parTime: 125, zones: ['meadow', 'ladder_tower', 'swing_gorge', 'bounce_grove', 'thorn_hollow', 'spike_gallery', 'key_sanctum'], enemies: ['goblin', 'wolf', 'sporecap', 'slime'] },
        { id: 4, worldIndex: 0, stageInWorld: 4, nameKey: 'runenpfad.level_4', parTime: 135, zones: ['meadow', 'mushroom_canopy', 'lantern_path', 'lever_bridge', 'secret_vault', 'rune_gate', 'key_sanctum'], enemies: ['wolf', 'boar', 'sporecap', 'wolf'] },
        { id: 5, worldIndex: 0, stageInWorld: 5, nameKey: 'runenpfad.level_5', parTime: 125, zones: ['meadow', 'step_ledge', 'bounce_grove', 'boss_arena'], enemies: ['slime', 'sporecap'], boss: 'boss_boar' },

        { id: 6, worldIndex: 1, stageInWorld: 1, nameKey: 'runenpfad.level_6', parTime: 120, zones: ['meadow', 'ritual_chain', 'lantern_path', 'rune_gate', 'mushroom_canopy', 'key_sanctum'], enemies: ['slime', 'sporecap', 'spider_drone', 'drakeling'] },
        { id: 7, worldIndex: 1, stageInWorld: 2, nameKey: 'runenpfad.level_7', parTime: 130, zones: ['meadow', 'crumble_chasm', 'pendulum_hall', 'swing_gorge', 'thorn_hollow', 'key_sanctum'], enemies: ['goblin', 'crystal_bat', 'boar'] },
        { id: 8, worldIndex: 1, stageInWorld: 3, nameKey: 'runenpfad.level_8', parTime: 140, zones: ['meadow', 'dragon_strafe', 'ladder_tower', 'crumble_chasm', 'updraft_shaft', 'boulder_gallery', 'key_sanctum'], enemies: ['spider_drone', 'boar', 'crystal_bat', 'drakeling'] },
        { id: 9, worldIndex: 1, stageInWorld: 4, nameKey: 'runenpfad.level_9', parTime: 165, zones: ['meadow', 'thorn_hollow', 'secret_vault', 'rope_chasm', 'rune_sequence', 'lantern_path', 'mushroom_canopy', 'key_sanctum'], enemies: ['boar', 'crystal_bat', 'drakeling', 'stalker'] },
        { id: 10, worldIndex: 1, stageInWorld: 5, nameKey: 'runenpfad.level_10', parTime: 135, zones: ['meadow', 'crumble_chasm', 'lantern_path', 'boss_arena'], enemies: ['crystal_bat', 'spider_drone', 'drakeling'], boss: 'boss_goblin' },

        { id: 11, worldIndex: 2, stageInWorld: 1, nameKey: 'runenpfad.level_11', parTime: 115, zones: ['meadow', 'lever_bridge', 'updraft_shaft', 'cannon_bastion', 'geyser_field', 'key_sanctum'], enemies: ['scarab', 'wolf', 'quadcopter'] },
        { id: 12, worldIndex: 2, stageInWorld: 2, nameKey: 'runenpfad.level_12', parTime: 135, zones: ['meadow', 'crumble_chasm', 'bounce_grove|secret_vault', 'cannon_bastion', 'rune_shrine', 'dragon_strafe', 'key_sanctum'], enemies: ['wolf', 'scarab', 'drakeling', 'quadcopter'] },
        { id: 13, worldIndex: 2, stageInWorld: 3, nameKey: 'runenpfad.level_13', parTime: 150, zones: ['meadow', 'ladder_tower', 'conveyor_works', 'rope_chasm', 'ritual_chain:req', 'cannon_bastion', 'key_sanctum'], enemies: ['boar', 'scarab', 'security_orb', 'wolf'] },
        { id: 14, worldIndex: 2, stageInWorld: 4, nameKey: 'runenpfad.level_14', parTime: 160, zones: ['meadow', 'thorn_hollow', 'updraft_shaft', 'cannon_bastion', 'secret_vault', 'rune_sequence:vault', 'key_sanctum'], enemies: ['security_orb', 'quadcopter', 'scarab', 'drakeling'] },
        { id: 15, worldIndex: 2, stageInWorld: 5, nameKey: 'runenpfad.level_15', parTime: 140, zones: ['meadow', 'cannon_bastion', 'updraft_shaft', 'boss_arena'], enemies: ['scarab', 'quadcopter', 'drakeling'], boss: 'boss_orb' },

        { id: 16, worldIndex: 3, stageInWorld: 1, nameKey: 'runenpfad.level_16', parTime: 130, zones: ['meadow', 'ice_slide', 'dragon_strafe', 'lantern_path', 'ambush_den', 'crate_yard', 'key_sanctum'], enemies: ['wolf', 'frost_wraith', 'spider_drone'] },
        { id: 17, worldIndex: 3, stageInWorld: 2, nameKey: 'runenpfad.level_17', parTime: 145, zones: ['meadow', 'ice_slide', 'lever_bridge', 'bounce_grove', 'rune_gate', 'secret_vault', 'key_sanctum'], enemies: ['service_robot', 'crystal_bat', 'frost_wraith', 'spider_drone'] },
        { id: 18, worldIndex: 3, stageInWorld: 3, nameKey: 'runenpfad.level_18', parTime: 150, zones: ['meadow', 'moving_bridge', 'ice_slide', 'ladder_tower', 'pendulum_hall', 'ritual_chain:vault', 'key_sanctum'], enemies: ['spider_drone', 'crystal_bat', 'frost_wraith', 'wolf'] },
        { id: 19, worldIndex: 3, stageInWorld: 4, nameKey: 'runenpfad.level_19', parTime: 170, zones: ['meadow', 'swing_gorge', 'ice_slide', 'rope_chasm', 'updraft_shaft', 'rune_sequence', 'key_sanctum'], enemies: ['quadcopter', 'frost_wraith', 'security_orb', 'sentry_bot'] },
        { id: 20, worldIndex: 3, stageInWorld: 5, nameKey: 'runenpfad.level_20', parTime: 145, zones: ['meadow', 'ice_slide', 'pendulum_hall', 'boss_arena'], enemies: ['frost_wraith', 'spider_drone'], boss: 'boss_quad' },

        { id: 21, worldIndex: 4, stageInWorld: 1, nameKey: 'runenpfad.level_21', parTime: 140, zones: ['meadow', 'cannon_bastion', 'swing_gorge', 'moving_bridge', 'flame_jets', 'key_sanctum'], enemies: ['service_robot', 'spider_drone', 'clock_knight', 'drakeling'] },
        { id: 22, worldIndex: 4, stageInWorld: 2, nameKey: 'runenpfad.level_22', parTime: 150, zones: ['meadow', 'thorn_hollow', 'lantern_path', 'ladder_tower', 'pendulum_hall', 'rune_sequence:vault', 'rune_shrine', 'key_sanctum'], enemies: ['spider_drone', 'clock_knight', 'combat_mech', 'security_orb'] },
        { id: 23, worldIndex: 4, stageInWorld: 3, nameKey: 'runenpfad.level_23', parTime: 165, zones: ['meadow', 'crumble_chasm', 'conveyor_works', 'dragon_strafe', 'secret_vault', 'cannon_bastion', 'key_sanctum'], enemies: ['combat_mech', 'clock_knight', 'security_orb', 'drakeling', 'bomber_drone'] },
        { id: 24, worldIndex: 4, stageInWorld: 4, nameKey: 'runenpfad.level_24', parTime: 165, zones: ['meadow', 'cannon_bastion', 'thorn_hollow', 'pendulum_hall|conveyor_works', 'lever_bridge', 'ritual_chain:req', 'mushroom_canopy', 'key_sanctum'], enemies: ['combat_mech', 'clock_knight', 'quadcopter', 'spider_drone', 'security_orb'] },
        { id: 25, worldIndex: 4, stageInWorld: 5, nameKey: 'runenpfad.level_25', parTime: 160, zones: ['meadow', 'moving_bridge', 'conveyor_works', 'boss_arena'], enemies: ['clock_knight', 'security_orb', 'quadcopter'], boss: 'boss_mech' }
    ];

    // Variety zones (zones.js) join the endless pool next to the classic archetypes.
    const NEW_ZONES = ['bounce_grove', 'updraft_shaft', 'conveyor_works', 'pendulum_hall', 'secret_vault', 'lantern_path', 'rope_chasm'];
    // Extra zones (zones-extra.js) by world: geyser fields in the desert and glacier, flame jets in the desert and citadel, rune gates anywhere.
    const EXTRA_ZONES = [['rune_gate'], ['rune_gate'], ['rune_gate', 'geyser_field', 'flame_jets'], ['rune_gate', 'geyser_field'], ['rune_gate', 'flame_jets']];
    // Round 4 zones (swing.js, puzzles.js, dragons.js); dragon strafes start in world 2.
    const R4_ZONES = ['swing_gorge', 'rune_sequence', 'lever_bridge', 'ritual_chain'];
    // Zone modules in dispatch order: the first one whose has(zone) is true builds the zone.
    function zoneModules() {
        return [root.RunenpfadZones, root.RunenpfadZonesExtra, root.RunenpfadSwing, root.RunenpfadPuzzles, root.RunenpfadDragons, root.RunenpfadNpcs];
    }

    // Zone slots such as 'a|b' resolve by level-id seed, so a campaign level always builds the same variant.
    function resolveZone(zone, cfg, zIdx) {
        if (zone.indexOf('|') < 0) return zone;
        const parts = zone.split('|');
        const pick = makeRng((cfg.id || 1) * 2713 + zIdx * 97 + 5)();
        return parts[Math.floor(pick * parts.length)];
    }

    // Presentation data lives in biomes.js; fall back gracefully when it is not loaded.
    function biomeApi() {
        return root.RunenpfadBiomes || null;
    }

    function makeRng(seed) {
        let s = (Number(seed) || 1) >>> 0;
        if (s === 0) s = 1337;
        return function () {
            s = (Math.imul(1664525, s) + 1013904223) >>> 0;
            return s / 4294967296;
        };
    }

    function createEndlessConfig(floor, seed) {
        const idx = Math.max(1, Math.floor(Number(floor) || 1));
        const rng = makeRng((seed || 9001) + idx * 7919);
        const worldIndex = (idx - 1) % WORLDS.length;
        const pool = ['bridge_gap', 'step_ledge', 'ladder_tower', 'mushroom_canopy', 'moving_bridge', 'crumble_chasm', 'thorn_hollow', 'cannon_bastion', 'spike_gallery', 'boulder_gallery', 'rune_shrine', 'ambush_den'].concat(NEW_ZONES, EXTRA_ZONES[worldIndex], R4_ZONES, worldIndex > 0 ? ['dragon_strafe'] : []);
        if (worldIndex === 3) pool.push('ice_slide');
        const zones = ['meadow'];
        const count = Math.min(6, 3 + Math.floor(idx / 3));
        for (let i = 0; i < count; i++) {
            zones.push(pool[Math.floor(rng() * pool.length)]);
        }
        const isBoss = idx % 3 === 0;
        // From floor 6 every second tower boss is a dragon (frost in the glacier, ember elsewhere).
        const dragonFloor = isBoss && idx >= 6 && (idx / 3) % 2 === 0;
        zones.push(isBoss ? 'boss_arena' : 'key_sanctum');
        const roster = worldIndex === 0 ? ['slime', 'goblin', 'wolf', 'boar', 'stalker', 'sporecap']
            : worldIndex === 1 ? ['goblin', 'spider_drone', 'boar', 'slime', 'stalker', 'bomber_drone', 'crystal_bat', 'sporecap']
            : worldIndex === 2 ? ['wolf', 'boar', 'quadcopter', 'security_orb', 'stalker', 'scarab']
            : worldIndex === 3 ? ['wolf', 'service_robot', 'spider_drone', 'quadcopter', 'sentry_bot', 'frost_wraith', 'crystal_bat']
            : ['service_robot', 'spider_drone', 'quadcopter', 'security_orb', 'combat_mech', 'sentry_bot', 'bomber_drone', 'clock_knight'];
        const enemies = [];
        const enemyCount = 3 + Math.min(5, Math.floor(idx / 2));
        for (let i = 0; i < enemyCount; i++) {
            enemies.push(roster[Math.floor(rng() * roster.length)]);
        }
        // Every fourth floor gets a merchant camp; inserted after all rng draws so seeds stay unchanged.
        if (idx % 4 === 2) zones.splice(1, 0, 'npc_camp:merchant');
        const bosses = ['boss_boar', 'boss_goblin', 'boss_orb', 'boss_quad', 'boss_mech'];
        return {
            id: 1000 + idx,
            mode: 'endless',
            endlessFloor: idx,
            seed: seed || 9001,
            worldIndex: worldIndex,
            stageInWorld: ((idx - 1) % 5) + 1,
            nameKey: 'runenpfad.endless_stage',
            parTime: 90 + Math.min(50, idx * 4),
            zones: zones,
            enemies: enemies,
            boss: isBoss ? (dragonFloor ? (worldIndex === 3 ? 'boss_dragon_frost' : 'boss_dragon_ember') : bosses[worldIndex]) : null
        };
    }

    // Builds one enemy record in world pixels; bosses use it for their summons as well.
    function createEnemy(defId, x, y, minX, maxX, opts) {
        const o = opts || {};
        const def = ENEMY_DEFS[defId] || ENEMY_DEFS.slime;
        const hp = def.hp * (def.boss ? 1 : (o.hpScale || 1));
        return {
            id: o.id || ('e_' + defId),
            defId: def.id,
            def: def,
            x: x,
            y: y,
            spawnX: x,
            spawnY: y,
            vx: def.speed * (o.dir || 1),
            vy: 0,
            minX: minX,
            maxX: maxX,
            hp: hp,
            maxHp: hp,
            facing: -1,
            onGround: false,
            animState: 'move',
            animTime: 0,
            hurtTimer: 0,
            shootTimer: o.shootTimer == null ? 1.5 : o.shootTimer,
            phase: 1,
            phased: false,
            windup: 0,
            blockFlash: 0,
            diving: false,
            dead: false,
            deathTimer: 0
        };
    }

    function buildStage(levelOrOptions) {
        let cfg;
        if (levelOrOptions && typeof levelOrOptions === 'object' && Array.isArray(levelOrOptions.zones)) {
            cfg = levelOrOptions;
        } else if (levelOrOptions && typeof levelOrOptions === 'object' && levelOrOptions.mode === 'endless') {
            cfg = createEndlessConfig(levelOrOptions.endlessFloor || 1, levelOrOptions.seed || 9001);
        } else {
            const id = typeof levelOrOptions === 'number' ? levelOrOptions : (levelOrOptions && levelOrOptions.levelId) || 1;
            cfg = CAMPAIGN_LEVELS.find(function (l) { return l.id === id; }) || CAMPAIGN_LEVELS[0];
        }

        const world = WORLDS[cfg.worldIndex];
        const biomeRef = biomeApi();
        const biome = biomeRef ? biomeRef.biomeFor(cfg.worldIndex) : null;
        const rows = 16;
        const groundY = 13;
        const rng = makeRng((cfg.id || 1) * 48271 + 17);

        const solidMap = {};
        const tiles = [];
        const platforms = [];
        const movingPlatforms = [];
        const ladders = [];
        const hazards = [];
        const crates = [];
        const collectibles = [];
        const checkpoints = [];
        const cannons = [];
        const scenery = [];
        const enemies = [];
        const boulders = [];
        const shrines = [];
        const windZones = [];
        const pendulums = [];
        const secretWalls = [];
        const lanterns = [];
        const ropes = [];
        const geysers = [];
        const switches = [];
        const gates = [];
        const flameJets = [];
        const chests = [];
        // Round 4 stage arrays filled by swing.js, puzzles.js and dragons.js.
        const extras = { swings: [], oracles: [], glyphStones: [], bridgeSegs: [], levers: [], rituals: [], dragonRuns: [], npcs: [], questItems: [], npcsReady: false };

        function setTile(tx, ty, assetId, kind) {
            if (tx < 0 || ty < 0 || ty >= rows) return;
            const key = tx + ',' + ty;
            if (kind === 'solid') solidMap[key] = true;
            tiles.push({ tx: tx, ty: ty, assetId: assetId, kind: kind || 'solid' });
        }

        function fillGround(startX, width, topY) {
            const top = topY == null ? groundY : topY;
            for (let x = startX; x < startX + width; x++) {
                setTile(x, top, world.groundTop, 'solid');
                for (let y = top + 1; y < rows; y++) {
                    setTile(x, y, y === rows - 1 ? world.altFill : world.groundFill, 'solid');
                }
            }
        }

        function addPlatformRow(startX, width, ty, kind) {
            const asset = kind === 'spring' ? world.spring : kind === 'trapdoor' ? 'trapdoor' : world.platform;
            for (let x = startX; x < startX + width; x++) {
                platforms.push({
                    tx: x,
                    ty: ty,
                    x: x * TILE,
                    y: ty * TILE,
                    w: TILE,
                    h: 16,
                    assetId: asset,
                    kind: kind || 'platform',
                    crumbleTimer: 0,
                    respawnTimer: 0,
                    solid: true
                });
            }
        }

        function addCoinLine(startX, count, ty, arc) {
            for (let i = 0; i < count; i++) {
                const offsetY = arc ? -Math.sin((i / Math.max(1, count - 1)) * Math.PI) * 0.8 : 0;
                collectibles.push({
                    id: 'coin_' + collectibles.length,
                    kind: 'coin',
                    assetId: 'coin',
                    animated: true,
                    animId: 'coin_spin',
                    x: (startX + i + 0.5) * TILE,
                    y: (ty + offsetY + 0.5) * TILE,
                    collected: false
                });
            }
        }

        function addEnemy(defId, tx, ty, minX, maxX) {
            const hpScale = cfg.mode === 'endless' ? 1 + Math.floor((cfg.endlessFloor - 1) / 4) : 1;
            enemies.push(createEnemy(defId, (tx + 0.5) * TILE, ty * TILE, minX * TILE, maxX * TILE, {
                id: 'e_' + enemies.length,
                hpScale: hpScale,
                dir: enemies.length % 2 === 0 ? 1 : -1,
                shootTimer: 1.2 + rng() * 0.8
            }));
        }

        function addProp(tx, ty) {
            const prop = world.props[Math.floor(rng() * world.props.length)];
            scenery.push({ assetId: prop, x: (tx + 0.5) * TILE, y: ty * TILE });
        }

        // Start boundary wall and safe spawn
        for (let y = 0; y < rows; y++) setTile(0, y, world.groundFill, 'solid');
        const spawn = { x: 2.5 * TILE, y: groundY * TILE };
        let cursorX = 1;
        let gemIndex = 0;
        const gemTypes = ['red_gem', 'blue_gem', 'green_gem'];
        const enemyPool = cfg.enemies && cfg.enemies.length ? cfg.enemies : ['slime'];
        let enemyCursor = 0;
        let checkpointAdded = false;
        let keyPos = null;
        let doorPos = null;
        let portalPos = null;
        let bossTarget = null;

        function nextEnemyId() {
            const id = enemyPool[enemyCursor % enemyPool.length];
            enemyCursor++;
            return id;
        }

        function placeGem(tx, ty) {
            if (gemIndex >= 3) return;
            collectibles.push({
                id: 'gem_' + gemIndex,
                kind: 'gem',
                gemIndex: gemIndex,
                assetId: gemTypes[gemIndex],
                animated: false,
                x: (tx + 0.5) * TILE,
                y: (ty + 0.5) * TILE,
                collected: false
            });
            gemIndex++;
        }

        function addCheckpoint(tx) {
            if (checkpointAdded) return;
            checkpoints.push({ id: 'cp_' + checkpoints.length, x: (tx + 0.5) * TILE, y: (groundY - 1) * TILE, activated: false });
            checkpointAdded = true;
        }

        // Handed to zones.js: everything a zone archetype needs to lay out its tiles and props.
        const zoneCtx = {
            cfg: cfg, world: world, rows: rows, groundY: groundY, TILE: TILE, rng: rng, cursorX: 1, zIdx: 0,
            setTile: setTile, fillGround: fillGround, addPlatformRow: addPlatformRow, addCoinLine: addCoinLine,
            addEnemy: addEnemy, nextEnemyId: nextEnemyId, placeGem: placeGem, gemCount: function () { return gemIndex; },
            addProp: addProp, addCheckpoint: addCheckpoint,
            solidMap: solidMap, tiles: tiles, platforms: platforms, hazards: hazards, collectibles: collectibles,
            crates: crates, scenery: scenery, enemies: enemies,
            windZones: windZones, pendulums: pendulums, secretWalls: secretWalls, lanterns: lanterns, ropes: ropes,
            geysers: geysers, switches: switches, gates: gates, flameJets: flameJets, chests: chests,
            extras: extras, checkpoints: checkpoints, variant: ''
        };

        cfg.zones.forEach(function (zoneSlot, zIdx) {
            const slot = resolveZone(zoneSlot, cfg, zIdx).split(':');
            const zone = slot[0];
            const zonesRef = zoneModules().find(function (z) { return z && z.has(zone); });
            if (zonesRef) {
                zoneCtx.variant = slot[1] || '';
                zoneCtx.cursorX = cursorX;
                zoneCtx.zIdx = zIdx;
                cursorX += zonesRef.build(zone, zoneCtx);
            } else if (zone === 'meadow') {
                fillGround(cursorX, 10, groundY);
                addProp(cursorX + 2, groundY);
                addProp(cursorX + 7, groundY);
                scenery.push({ assetId: 'sign', x: (cursorX + 3.5) * TILE, y: groundY * TILE });
                addPlatformRow(cursorX + 5, 3, groundY - 2, 'platform');
                addCoinLine(cursorX + 5, 3, groundY - 3, true);
                if (zIdx === 0) placeGem(cursorX + 6, groundY - 3);
                if (cfg.id === 1) {
                    // Accessible starter crate and first slime on screen 1
                    crates.push({ id: 'c_' + crates.length, assetId: 'crate', x: (cursorX + 8.5) * TILE, y: (groundY - 1) * TILE, w: 38, h: 38, hp: 1, broken: false, drop: 'coin' });
                }
                cursorX += 10;
            } else if (zone === 'step_ledge') {
                fillGround(cursorX, 12, groundY);
                fillGround(cursorX + 4, 4, groundY - 2);
                addPlatformRow(cursorX + 8, 3, groundY - 4, 'platform');
                addCoinLine(cursorX + 4, 4, groundY - 3, false);
                if (gemIndex < 2) placeGem(cursorX + 9, groundY - 5);
                addEnemy(nextEnemyId(), cursorX + 2, groundY, cursorX + 1, cursorX + 4);
                addEnemy(nextEnemyId(), cursorX + 9, groundY, cursorX + 8, cursorX + 12);
                addProp(cursorX + 1, groundY);
                cursorX += 12;
            } else if (zone === 'bridge_gap') {
                fillGround(cursorX, 4, groundY);
                // 3-tile gap with hazard floor below and bridge platform
                for (let x = cursorX + 4; x < cursorX + 8; x++) {
                    setTile(x, rows - 1, world.groundFill, 'solid');
                    hazards.push({ kind: 'lava', assetId: cfg.worldIndex >= 2 ? 'lava' : 'spikes', x: x * TILE, y: (rows - 2) * TILE, w: TILE, h: TILE });
                }
                addPlatformRow(cursorX + 4, 4, groundY - 1, 'platform');
                addCoinLine(cursorX + 4, 4, groundY - 2, true);
                fillGround(cursorX + 8, 5, groundY);
                addEnemy(nextEnemyId(), cursorX + 10, groundY, cursorX + 8, cursorX + 12);
                crates.push({ id: 'c_' + crates.length, assetId: 'barrel', x: (cursorX + 11.5) * TILE, y: (groundY - 1) * TILE, w: 38, h: 38, hp: 1, broken: false, drop: 'apple' });
                cursorX += 13;
            } else if (zone === 'crate_yard') {
                fillGround(cursorX, 11, groundY);
                addPlatformRow(cursorX + 3, 4, groundY - 2, 'platform');
                crates.push({ id: 'c_' + crates.length, assetId: 'crate', x: (cursorX + 2.5) * TILE, y: (groundY - 1) * TILE, w: 38, h: 38, hp: 2, broken: false, drop: 'coin' });
                crates.push({ id: 'c_' + crates.length, assetId: 'barrel', x: (cursorX + 5.5) * TILE, y: (groundY - 1) * TILE, w: 38, h: 38, hp: 1, broken: false, drop: 'heart' });
                addEnemy(nextEnemyId(), cursorX + 7, groundY, cursorX + 3, cursorX + 10);
                if (!checkpointAdded) {
                    checkpoints.push({ id: 'cp_' + checkpoints.length, x: (cursorX + 9.5) * TILE, y: (groundY - 1) * TILE, activated: false });
                    checkpointAdded = true;
                }
                addCoinLine(cursorX + 3, 4, groundY - 3, false);
                cursorX += 11;
            } else if (zone === 'mushroom_canopy') {
                fillGround(cursorX, 12, groundY);
                addPlatformRow(cursorX + 2, 2, groundY - 1, 'spring');
                addPlatformRow(cursorX + 5, 4, groundY - 5, 'platform');
                addCoinLine(cursorX + 5, 4, groundY - 6, true);
                if (gemIndex < 3) placeGem(cursorX + 7, groundY - 6);
                collectibles.push({ id: 'pot_' + collectibles.length, kind: 'potion', assetId: 'potion', animated: false, x: (cursorX + 5.5) * TILE, y: (groundY - 5.5) * TILE, collected: false });
                addEnemy(nextEnemyId(), cursorX + 7, groundY, cursorX + 4, cursorX + 11);
                addProp(cursorX + 9, groundY);
                cursorX += 12;
            } else if (zone === 'ladder_tower') {
                fillGround(cursorX, 12, groundY);
                addPlatformRow(cursorX + 3, 5, groundY - 3, 'platform');
                for (let ly = groundY - 3; ly < groundY; ly++) {
                    ladders.push({ tx: cursorX + 3, ty: ly, x: (cursorX + 3) * TILE, y: ly * TILE, w: TILE, h: TILE });
                }
                addPlatformRow(cursorX + 6, 4, groundY - 6, 'platform');
                for (let ly = groundY - 6; ly < groundY - 3; ly++) {
                    ladders.push({ tx: cursorX + 6, ty: ly, x: (cursorX + 6) * TILE, y: ly * TILE, w: TILE, h: TILE });
                }
                if (gemIndex < 3) placeGem(cursorX + 8, groundY - 7);
                addCoinLine(cursorX + 4, 3, groundY - 4, false);
                addEnemy(nextEnemyId(), cursorX + 9, groundY, cursorX + 5, cursorX + 11);
                if (!checkpointAdded) {
                    checkpoints.push({ id: 'cp_' + checkpoints.length, x: (cursorX + 10.5) * TILE, y: (groundY - 1) * TILE, activated: false });
                    checkpointAdded = true;
                }
                cursorX += 12;
            } else if (zone === 'moving_bridge') {
                fillGround(cursorX, 3, groundY);
                for (let x = cursorX + 3; x < cursorX + 9; x++) {
                    setTile(x, rows - 1, world.groundFill, 'solid');
                    hazards.push({ kind: 'lava', assetId: cfg.worldIndex >= 2 ? 'lava' : 'thorn', x: x * TILE, y: (rows - 2) * TILE, w: TILE, h: TILE });
                }
                movingPlatforms.push({
                    id: 'mp_' + movingPlatforms.length,
                    startX: (cursorX + 3.5) * TILE,
                    endX: (cursorX + 7.5) * TILE,
                    x: (cursorX + 3.5) * TILE,
                    prevX: (cursorX + 3.5) * TILE,
                    y: (groundY - 1) * TILE,
                    w: TILE * 2,
                    h: 18,
                    speed: 75 + cfg.worldIndex * 10,
                    dir: 1
                });
                addCoinLine(cursorX + 4, 4, groundY - 3, true);
                fillGround(cursorX + 9, 4, groundY);
                addEnemy(nextEnemyId(), cursorX + 10, groundY, cursorX + 9, cursorX + 12);
                cursorX += 13;
            } else if (zone === 'crumble_chasm') {
                fillGround(cursorX, 3, groundY);
                for (let x = cursorX + 3; x < cursorX + 9; x++) {
                    setTile(x, rows - 1, world.groundFill, 'solid');
                    hazards.push({ kind: 'spikes', assetId: 'spikes', x: x * TILE, y: (rows - 2) * TILE, w: TILE, h: TILE });
                }
                addPlatformRow(cursorX + 3, 2, groundY - 1, 'trapdoor');
                addPlatformRow(cursorX + 6, 2, groundY - 2, 'trapdoor');
                addCoinLine(cursorX + 4, 4, groundY - 3, true);
                fillGround(cursorX + 9, 4, groundY);
                if (gemIndex < 3) placeGem(cursorX + 10, groundY - 2);
                cursorX += 13;
            } else if (zone === 'thorn_hollow') {
                fillGround(cursorX, 12, groundY);
                hazards.push({ kind: 'spikes', assetId: cfg.worldIndex === 0 ? 'thorn' : 'spikes', x: (cursorX + 4) * TILE, y: (groundY - 1) * TILE, w: TILE, h: 32 });
                hazards.push({ kind: 'saw', assetId: 'saw', x: (cursorX + 8) * TILE, y: (groundY - 1) * TILE, w: TILE, h: 36 });
                addPlatformRow(cursorX + 3, 6, groundY - 2, 'platform');
                addCoinLine(cursorX + 4, 4, groundY - 3, true);
                addEnemy(nextEnemyId(), cursorX + 10, groundY, cursorX + 9, cursorX + 11);
                if (!checkpointAdded) {
                    checkpoints.push({ id: 'cp_' + checkpoints.length, x: (cursorX + 1.5) * TILE, y: (groundY - 1) * TILE, activated: false });
                    checkpointAdded = true;
                }
                cursorX += 12;
            } else if (zone === 'cannon_bastion') {
                fillGround(cursorX, 12, groundY);
                fillGround(cursorX + 5, 2, groundY - 2);
                cannons.push({ id: 'can_' + cannons.length, x: (cursorX + 5.5) * TILE, y: (groundY - 2.5) * TILE, dir: -1, timer: 1.5, interval: Math.max(1.8, 3.0 - cfg.worldIndex * 0.2) });
                addPlatformRow(cursorX + 3, 5, groundY - 4, 'platform');
                if (gemIndex < 3) placeGem(cursorX + 5, groundY - 5);
                addEnemy(nextEnemyId(), cursorX + 9, groundY, cursorX + 7, cursorX + 11);
                crates.push({ id: 'c_' + crates.length, assetId: 'crate', x: (cursorX + 2.5) * TILE, y: (groundY - 1) * TILE, w: 38, h: 38, hp: 1, broken: false, drop: 'heart' });
                cursorX += 12;
            } else if (zone === 'ice_slide') {
                fillGround(cursorX, 12, groundY);
                addPlatformRow(cursorX + 2, 8, groundY - 2, 'platform');
                hazards.push({ kind: 'spikes', assetId: 'stalactite', x: (cursorX + 6) * TILE, y: (groundY - 1) * TILE, w: TILE, h: 32 });
                addCoinLine(cursorX + 3, 5, groundY - 3, false);
                addEnemy(nextEnemyId(), cursorX + 9, groundY, cursorX + 7, cursorX + 11);
                cursorX += 12;
            } else if (zone === 'spike_gallery') {
                // Animated spike traps force a rhythm of hops instead of a straight run.
                fillGround(cursorX, 13, groundY);
                for (let x = cursorX + 2; x < cursorX + 11; x += 2) {
                    hazards.push({ kind: 'spike_trap', assetId: 'spike_trap', x: x * TILE, y: (groundY - 1) * TILE, w: TILE, h: 30 });
                }
                addPlatformRow(cursorX + 3, 3, groundY - 3, 'platform');
                addPlatformRow(cursorX + 7, 3, groundY - 4, 'platform');
                addCoinLine(cursorX + 3, 4, groundY - 4, true);
                addEnemy(nextEnemyId(), cursorX + 11, groundY, cursorX + 9, cursorX + 12);
                if (gemIndex < 3) placeGem(cursorX + 8, groundY - 5);
                cursorX += 13;
            } else if (zone === 'boulder_gallery') {
                // Falling stones: the ceiling drops hazards right where the player walks.
                fillGround(cursorX, 14, groundY);
                for (let i = 0; i < 3; i++) {
                    const bx = cursorX + 3 + i * 3;
                    boulders.push({
                        id: 'b_' + boulders.length,
                        x: bx * TILE,
                        y: (groundY - 7) * TILE,
                        spawnY: (groundY - 7) * TILE,
                        w: TILE,
                        h: TILE,
                        vy: 0,
                        state: 'idle',
                        timer: 0
                    });
                }
                addPlatformRow(cursorX + 2, 4, groundY - 2, 'platform');
                addCoinLine(cursorX + 3, 5, groundY - 3, false);
                addEnemy(nextEnemyId(), cursorX + 10, groundY, cursorX + 8, cursorX + 12);
                if (gemIndex < 3) placeGem(cursorX + 6, groundY - 3);
                cursorX += 14;
            } else if (zone === 'rune_shrine') {
                // The shrine grants a rune blessing: double damage for a short window.
                fillGround(cursorX, 13, groundY);
                shrines.push({ id: 'sh_' + shrines.length, x: (cursorX + 5.5) * TILE, y: (groundY - 1.5) * TILE, taken: false });
                addPlatformRow(cursorX + 2, 3, groundY - 3, 'platform');
                addCoinLine(cursorX + 2, 4, groundY - 4, true);
                collectibles.push({
                    id: 'st_' + collectibles.length,
                    kind: 'star',
                    assetId: 'star',
                    animated: false,
                    x: (cursorX + 5.5) * TILE,
                    y: (groundY - 4.5) * TILE,
                    collected: false
                });
                addEnemy(nextEnemyId(), cursorX + 9, groundY, cursorX + 7, cursorX + 11);
                if (!checkpointAdded) {
                    checkpoints.push({ id: 'cp_' + checkpoints.length, x: (cursorX + 11.5) * TILE, y: (groundY - 1) * TILE, activated: false });
                    checkpointAdded = true;
                }
                cursorX += 13;
            } else if (zone === 'ambush_den') {
                // Stalkers wait in the dark and only charge once the hero comes close.
                fillGround(cursorX, 13, groundY);
                addPlatformRow(cursorX + 3, 4, groundY - 3, 'platform');
                hazards.push({ kind: 'spikes', assetId: cfg.worldIndex === 0 ? 'thorn' : 'spikes', x: (cursorX + 8) * TILE, y: (groundY - 1) * TILE, w: TILE, h: 32 });
                addEnemy('stalker', cursorX + 6, groundY, cursorX + 4, cursorX + 11);
                addEnemy(nextEnemyId(), cursorX + 10, groundY, cursorX + 8, cursorX + 11);
                addCoinLine(cursorX + 4, 4, groundY - 4, true);
                if (gemIndex < 3) placeGem(cursorX + 4, groundY - 5);
                cursorX += 13;
            } else if (zone === 'key_sanctum') {
                fillGround(cursorX, 14, groundY);
                addPlatformRow(cursorX + 2, 4, groundY - 2, 'platform');
                if (!checkpointAdded) {
                    checkpoints.push({ id: 'cp_' + checkpoints.length, x: (cursorX + 1.5) * TILE, y: (groundY - 1) * TILE, activated: false });
                    checkpointAdded = true;
                }
                while (gemIndex < 3) {
                    placeGem(cursorX + 2 + gemIndex, groundY - 3);
                }
                keyPos = { x: (cursorX + 4.5) * TILE, y: (groundY - 2.5) * TILE, collected: false };
                doorPos = { x: (cursorX + 8.5) * TILE, y: (groundY - 1) * TILE, opened: false };
                collectibles.push({ id: 'tr_' + collectibles.length, kind: 'treasure', assetId: 'treasure', animated: false, x: (cursorX + 6.5) * TILE, y: (groundY - 0.5) * TILE, collected: false });
                portalPos = { x: (cursorX + 11.5) * TILE, y: (groundY - 1) * TILE };
                scenery.push({ assetId: world.props[0], x: (cursorX + 12.5) * TILE, y: groundY * TILE });
                cursorX += 14;
            } else if (zone === 'boss_arena') {
                fillGround(cursorX, 20, groundY);
                checkpoints.push({ id: 'cp_' + checkpoints.length, x: (cursorX + 1.5) * TILE, y: (groundY - 1) * TILE, activated: false });
                addPlatformRow(cursorX + 3, 3, groundY - 3, 'platform');
                addPlatformRow(cursorX + 9, 3, groundY - 3, 'platform');
                collectibles.push({ id: 'pot_' + collectibles.length, kind: 'potion', assetId: 'potion', animated: false, x: (cursorX + 4) * TILE, y: (groundY - 3.5) * TILE, collected: false });
                while (gemIndex < 3) {
                    placeGem(cursorX + 3 + gemIndex * 3, groundY - 4);
                }
                const bossDefId = cfg.boss || 'boss_boar';
                addEnemy(bossDefId, cursorX + 11, groundY, cursorX + 3, cursorX + 15);
                bossTarget = enemies[enemies.length - 1];
                keyPos = { x: (cursorX + 11) * TILE, y: (groundY - 0.6) * TILE, collected: false, guardedByBoss: true };
                doorPos = { x: (cursorX + 15.5) * TILE, y: (groundY - 1) * TILE, opened: false };
                portalPos = { x: (cursorX + 18) * TILE, y: (groundY - 1) * TILE };
                cursorX += 20;
            }
        });

        // Fallback safety in case a custom zone sequence omitted key/portal
        if (!keyPos || !portalPos) {
            fillGround(cursorX, 8, groundY);
            while (gemIndex < 3) placeGem(cursorX + 1 + gemIndex, groundY - 2);
            if (!keyPos) keyPos = { x: (cursorX + 2.5) * TILE, y: (groundY - 1.5) * TILE, collected: false };
            if (!doorPos) doorPos = { x: (cursorX + 4.5) * TILE, y: (groundY - 1) * TILE, opened: false };
            if (!portalPos) portalPos = { x: (cursorX + 6.5) * TILE, y: (groundY - 1) * TILE };
            cursorX += 8;
        }

        // Right boundary wall
        for (let y = 0; y < rows; y++) setTile(cursorX, y, world.groundFill, 'solid');
        const widthTiles = cursorX + 1;

        const stage = {
            id: cfg.id,
            mode: cfg.mode || 'campaign',
            endlessFloor: cfg.endlessFloor || 0,
            worldIndex: cfg.worldIndex,
            stageInWorld: cfg.stageInWorld,
            world: world,
            biome: biome,
            ambience: {
                primary: biome ? biome.ambience.primary : world.ambience,
                secondary: biome ? biome.ambience.secondary : ''
            },
            nameKey: cfg.nameKey,
            parTime: cfg.parTime,
            tileSize: TILE,
            widthTiles: widthTiles,
            heightTiles: rows,
            width: widthTiles * TILE,
            height: rows * TILE,
            groundY: groundY * TILE,
            spawn: spawn,
            solidMap: solidMap,
            tiles: tiles,
            platforms: platforms,
            movingPlatforms: movingPlatforms,
            ladders: ladders,
            hazards: hazards,
            boulders: boulders,
            shrines: shrines,
            windZones: windZones,
            pendulums: pendulums,
            secretWalls: secretWalls,
            lanterns: lanterns,
            ropes: ropes,
            geysers: geysers,
            switches: switches,
            gates: gates,
            flameJets: flameJets,
            chests: chests,
            crates: crates,
            collectibles: collectibles,
            checkpoints: checkpoints,
            cannons: cannons,
            scenery: scenery,
            enemies: enemies,
            bossId: bossTarget ? bossTarget.id : null,
            key: keyPos,
            door: doorPos,
            portal: portalPos
        };

        Object.assign(stage, extras);

        // Every stage from campaign level 3 on (and every endless floor) holds at least one rune chest.
        if (root.RunenpfadRunes && (cfg.mode === 'endless' || cfg.id >= 3)) root.RunenpfadRunes.placeChest(stage, cfg);

        // Rune trials layer a per-level twist on top of the generated stage.
        const variety = root.RunenpfadVariety || null;
        if (variety) {
            const mod = variety.pickMod(cfg.mode === 'endless' ? 0 : cfg.id, cfg.mode === 'endless' ? cfg.endlessFloor : 0);
            if (mod && mod.effects.extraEnemies) {
                for (let i = 0; i < mod.effects.extraEnemies; i++) {
                    const tx = 6 + Math.floor(rng() * Math.max(1, widthTiles - 14));
                    addEnemy(enemyPool[Math.floor(rng() * enemyPool.length)], tx, groundY, Math.max(1, tx - 4), Math.min(widthTiles - 1, tx + 4));
                }
            }
            if (mod && mod.effects.heartBonus) {
                for (let i = 0; i < mod.effects.heartBonus; i++) {
                    const tx = 4 + Math.floor(rng() * Math.max(1, widthTiles - 10));
                    collectibles.push({
                        id: 'hb_' + collectibles.length,
                        kind: 'heart',
                        assetId: 'heart',
                        animated: false,
                        x: (tx + 0.5) * TILE,
                        y: (groundY - 1.5) * TILE,
                        collected: false
                    });
                }
            }
            variety.applyToStage(stage, mod);
        }
        return stage;
    }

    root.RunenpfadLevels = {
        TILE: TILE,
        REQUIRED_PACK_IDS: REQUIRED_PACK_IDS,
        PACK_SPECS: PACK_SPECS,
        WORLDS: WORLDS,
        HERO_DEFS: HERO_DEFS,
        ENEMY_DEFS: ENEMY_DEFS,
        UPGRADE_DEFS: UPGRADE_DEFS,
        CAMPAIGN_LEVELS: CAMPAIGN_LEVELS,
        validateSpriteManifests: validateSpriteManifests,
        createEndlessConfig: createEndlessConfig,
        createEnemy: createEnemy,
        buildStage: buildStage
    };
})(typeof window !== 'undefined' ? window : globalThis);
