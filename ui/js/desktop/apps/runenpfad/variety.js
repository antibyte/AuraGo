/* Runenpfad Rune Trials: per-level modifiers, shrine blessings and variety helpers. DOM-free. */
(function (root) {
    'use strict';

    // A trial changes how a stage plays. Effects are read by levels.js, sim.js and view.js.
    const MODS = [
        {
            id: 'swift',
            nameKey: 'runenpfad.mod_swift',
            descKey: 'runenpfad.mod_swift_desc',
            effects: { enemySpeed: 1.4, enemyHp: 1.15, enemyScore: 1.25 }
        },
        {
            id: 'fragile',
            nameKey: 'runenpfad.mod_fragile',
            descKey: 'runenpfad.mod_fragile_desc',
            effects: { crumbleTime: 0.4 }
        },
        {
            id: 'dark',
            nameKey: 'runenpfad.mod_dark',
            descKey: 'runenpfad.mod_dark_desc',
            effects: { darkness: 0.3 }
        },
        {
            id: 'greedy',
            nameKey: 'runenpfad.mod_greedy',
            descKey: 'runenpfad.mod_greedy_desc',
            effects: { coinValue: 2.5, enemyScore: 1.6, extraEnemies: 1 }
        },
        {
            id: 'storm',
            nameKey: 'runenpfad.mod_storm',
            descKey: 'runenpfad.mod_storm_desc',
            effects: { platformSpeed: 1.9, wind: 46 }
        },
        {
            id: 'mend',
            nameKey: 'runenpfad.mod_mend',
            descKey: 'runenpfad.mod_mend_desc',
            effects: { potionBonus: 1, heartBonus: 2 }
        }
    ];

    const RUNE_POWER_DURATION = 12;

    function modById(id) {
        return MODS.find(function (m) { return m.id === id; }) || null;
    }

    // Deterministic trial selection: level 1 stays a clean tutorial, the rest cycle through trials.
    function pickMod(levelId, floor) {
        if (floor) {
            const idx = Math.abs(Math.floor(floor) - 1) % MODS.length;
            return MODS[idx];
        }
        const id = Math.floor(Number(levelId) || 0);
        if (id <= 1) return null;
        return MODS[(id - 2) % MODS.length];
    }

    // Applies a trial to a freshly built stage. Enemy defs are cloned so shared tables stay pristine.
    function applyToStage(stage, mod) {
        stage.mod = mod || null;
        stage.modEffects = mod ? mod.effects : {};
        stage.coinValue = mod && mod.effects.coinValue ? mod.effects.coinValue : 1;
        stage.scoreMul = mod && mod.effects.enemyScore ? mod.effects.enemyScore : 1;
        stage.darkness = (mod && mod.effects.darkness) || 0;
        if (!mod) return stage;

        const speedMul = mod.effects.enemySpeed || 1;
        const hpMul = mod.effects.enemyHp || 1;
        stage.enemies.forEach(function (e) {
            if (speedMul !== 1) {
                e.def = Object.assign({}, e.def, { speed: e.def.speed * speedMul });
                e.vx = e.vx >= 0 ? e.def.speed : -e.def.speed;
            }
            if (hpMul !== 1) {
                e.maxHp = Math.max(1, Math.round(e.maxHp * hpMul));
                e.hp = e.maxHp;
            }
            e.score = Math.round(e.def.score * stage.scoreMul);
        });

        const platformMul = mod.effects.platformSpeed || 1;
        if (platformMul !== 1) {
            stage.movingPlatforms.forEach(function (mp) {
                mp.speed = mp.speed * platformMul;
            });
        }

        const crumbleMul = mod.effects.crumbleTime || 1;
        if (crumbleMul !== 1) {
            stage.platforms.forEach(function (p) {
                if (p.kind === 'trapdoor') p.crumbleTime = 0.45 * crumbleMul;
            });
        }
        if (mod.id === 'storm') {
            if (stage.ambience) stage.ambience.secondary = 'rain-heavy';
            stage.weather = 'rain';
        }
        return stage;
    }

    // Blessing state used by the rune shrine and the hero damage pipeline.
    function createBlessing() {
        return { runePower: 0, shrineCount: 0 };
    }

    function damageMultiplier(blessing) {
        return blessing && blessing.runePower > 0 ? 2 : 1;
    }

    root.RunenpfadVariety = {
        MODS: MODS,
        RUNE_POWER_DURATION: RUNE_POWER_DURATION,
        modById: modById,
        pickMod: pickMod,
        applyToStage: applyToStage,
        createBlessing: createBlessing,
        damageMultiplier: damageMultiplier
    };
})(typeof window !== 'undefined' ? window : globalThis);
