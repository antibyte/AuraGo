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
            effects: { platformSpeed: 1.9, gusts: true }
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
            // Rain only under an open sky (woodland 0, citadel 4); caves, desert and glacier keep their own weather and get a wind bed.
            const wi = stage.worldIndex;
            if (wi === 0 || wi === 4) {
                if (stage.ambience) stage.ambience.secondary = 'rain-heavy';
                stage.weather = 'rain';
            } else if (stage.ambience && !/^wind/.test(stage.ambience.primary || '')) {
                stage.ambience.secondary = wi === 3 ? 'wind-strong' : 'wind-gentle';
            }
        }
        return stage;
    }

    // ---- Storm gusts: calm -> warn (telegraph) -> gust -> calm, scheduled only with the gameplay RNG ----
    const GUST = { calmMin: 3.5, calmMax: 6, warn: 0.9, gustMin: 1.6, gustMax: 2.2, fade: 0.3, groundPush: -35, airPush: -60 };

    function physics() { return root.RunenpfadPhysics || null; }

    function windRoll(sim, a, b) {
        const P = physics();
        return a + (b - a) * (P ? P.rand(sim) : 0.5);
    }

    // Advances sim.wind; null for stages without the Storm trial. Emits gust_warn / gust / gust_end at the hero position.
    function stepWind(sim, dt) {
        const fx = sim.stage && sim.stage.modEffects;
        if (!fx || !fx.gusts) return;
        const P = physics();
        if (!sim.wind) sim.wind = { state: 'calm', t: windRoll(sim, GUST.calmMin, GUST.calmMax), dur: 0, strength: 0, dir: -1 };
        const w = sim.wind;
        const p = sim.player;
        const emit = function (type) { if (P) P.pushEvent(sim, type, { x: p.x, y: p.y }); };
        if (sim.bossIntro) return;
        w.t -= dt;
        if (w.state === 'calm') {
            w.strength = 0;
            if (w.t <= 0) { w.state = 'warn'; w.t = GUST.warn; w.dur = GUST.warn; emit('gust_warn'); }
        } else if (w.state === 'warn') {
            w.strength = 0;
            if (w.t <= 0) { w.state = 'gust'; w.dur = windRoll(sim, GUST.gustMin, GUST.gustMax); w.t = w.dur; emit('gust'); }
        } else {
            const age = w.dur - w.t;
            w.strength = Math.max(0, Math.min(1, age / GUST.fade, w.t / GUST.fade));
            if (w.t <= 0) { w.state = 'calm'; w.strength = 0; w.t = windRoll(sim, GUST.calmMin, GUST.calmMax); w.dur = 0; emit('gust_end'); }
        }
    }

    // Horizontal velocity added to the hero's target vx: braced on the ground when idle, a small push while walking, a bigger one in the air.
    function windPush(sim, moving) {
        const w = sim.wind;
        if (!w || w.state !== 'gust' || w.strength <= 0) return 0;
        const p = sim.player;
        if (p.onGround) return moving ? GUST.groundPush * w.strength : 0;
        return GUST.airPush * w.strength;
    }

    // Blessing state used by the rune shrine and the hero damage pipeline.
    function createBlessing() {
        return { runePower: 0, shrineCount: 0 };
    }

    function damageMultiplier(blessing) {
        return blessing && blessing.runePower > 0 ? 2 : 1;
    }

    function resonanceNotice(sim, key) {
        const replaceable = ['', 'runenpfad.notice.find_key', 'runenpfad.notice.resonance_complete', 'runenpfad.notice.resonance_expire'];
        if (replaceable.indexOf(sim.notice || '') >= 0) sim.notice = key;
    }

    function resonanceEvent(sim, type, rune, index, timeLeft) {
        const P = physics();
        if (P) P.pushEvent(sim, type, {
            x: rune.x,
            y: rune.y,
            index: index,
            nextRune: sim.stage.resonancePath.nextRune,
            timeLeft: timeLeft == null ? sim.stage.resonancePath.timeLeft : timeLeft,
            timeLimit: sim.stage.resonancePath.timeLimit
        });
    }

    // Collect one visible rune at a time; only the normal simulation tick advances its timer.
    function stepResonance(sim, dt) {
        const path = sim && sim.stage && sim.stage.resonancePath;
        if (!path || path.complete || sim.state !== 'playing' || !path.runes || !path.runes.length) return;
        const P = physics();
        if (path.started) {
            path.timeLeft = Math.max(0, path.timeLeft - Math.max(0, Number(dt) || 0));
            if (path.timeLeft <= 0) {
                const missed = path.runes[path.nextRune];
                const index = path.nextRune;
                path.started = false;
                path.expired = true;
                path.needsLeave = true;
                path.nextRune = 0;
                path.timeLeft = path.timeLimit;
                resonanceEvent(sim, 'resonance_expire', missed, index, 0);
                resonanceNotice(sim, 'runenpfad.notice.resonance_expire');
                if (P) P.fxBurst(sim, 'rune', missed.x, missed.y, 5, { color: '#7ee8fa', speed: 90, life: 0.45 });
                return;
            }
        }

        const index = path.nextRune;
        const rune = path.runes[index];
        if (!rune) return;
        const distance = Math.hypot(rune.x - sim.player.x, rune.y - (sim.player.y - 22));
        if (path.needsLeave) {
            if (distance >= 30) path.needsLeave = false;
            return;
        }
        if (distance >= 30) return;
        const first = index === 0;
        const last = index === path.runes.length - 1;
        path.nextRune += 1;
        if (first) {
            if (path.expired && sim.notice === 'runenpfad.notice.resonance_expire') sim.notice = 'runenpfad.notice.find_key';
            path.started = true;
            path.expired = false;
            path.timeLeft = path.timeLimit;
            resonanceEvent(sim, 'resonance_start', rune, index);
        } else if (last) {
            path.started = false;
            path.complete = true;
            path.expired = false;
            path.timeLeft = 0;
            const coins = Math.max(1, Math.floor(Number(path.rewardCoins) || 3));
            sim.stageCoins += coins;
            sim.profile.coins += coins;
            sim.stageScore += coins * 25;
            sim.floatingTexts.push({ x: rune.x, y: rune.y - 12, text: '+' + coins, icon: 'coin', color: '#ffd65a', timer: 0.9 });
            resonanceEvent(sim, 'resonance_complete', rune, index);
            resonanceNotice(sim, 'runenpfad.notice.resonance_complete');
            if (P) {
                P.fxBurst(sim, 'rune', rune.x, rune.y, 18, { color: '#7ee8fa', speed: 210, life: 0.85 });
                P.ring(sim, rune.x, rune.y, { color: '#7ee8fa', size: 12, grow: 180, life: 0.4 });
                P.fxFlash(sim, '#7ee8fa', 0.2, 0.3);
                P.fxShake(sim, 2.5);
            }
            return;
        } else {
            resonanceEvent(sim, 'resonance_rune', rune, index);
        }
        if (P) P.fxBurst(sim, 'rune', rune.x, rune.y, first ? 6 : 8, { color: '#7ee8fa', speed: 120, life: 0.5 });
    }

    root.RunenpfadVariety = {
        MODS: MODS,
        RUNE_POWER_DURATION: RUNE_POWER_DURATION,
        modById: modById,
        pickMod: pickMod,
        applyToStage: applyToStage,
        GUST: GUST,
        stepWind: stepWind,
        windPush: windPush,
        createBlessing: createBlessing,
        damageMultiplier: damageMultiplier,
        stepResonance: stepResonance
    };
})(typeof window !== 'undefined' ? window : globalThis);
