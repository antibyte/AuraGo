/* Runenpfad ember chapter core (global `RunenpfadEmber`): the derived chapter step, the hub variants of Pathrest, the fate of the
 * ember wyrm Brandur, the rewards (Ember Heart, Brandur's Blessing) and the meals. DOM-free: no document, no window, no canvas,
 * no random numbers, no clock. The chapter step is derived from the profile (seen scenes and the flag dr.fate), never stored.
 * Story writes go through the RunenpfadNpcs helpers; texts live in the locale files (`runenpfad.notice.*`). */
(function (root) {
    'use strict';

    const FLAGS = { fate: 'dr.fate', tabs: 'dr.tabs', bell: 'dr.bell', shield: 'dr.shield', meal: 'dr.meal', map: 'dr.map', visitA: 'dr.visit_a', visitB: 'dr.visit_b' };
    const SCENES = ['dr_tremor', 'dr_night', 'dr_legend', 'dr_wake', 'dr_slain', 'dr_sealed'];
    const MEALS = { stew: 1, tea: 2, broth: 3 };
    const MEAL_NAMES = ['stew', 'tea', 'broth'];
    const FATES = [null, 'beaten', 'slain', 'sealed'];
    const SLAY_COINS = 300;
    const NOTICE = 'runenpfad.notice.';
    // The burned barn is a slightly taller frame than the whole one; the height keeps the same art scale.
    const BURNED_RATIO = 399 / 388;

    function P() { return root.RunenpfadPhysics; }
    function N() { return root.RunenpfadNpcs; }
    function pushEvent(sim, type, detail) { if (P()) P().pushEvent(sim, type, detail); }

    function flagOf(profile, key) {
        const f = profile && profile.npcFlags;
        return f && f[key] > 0 ? f[key] : 0;
    }

    function seenHas(profile, id) {
        return !!(profile && Array.isArray(profile.seen) && profile.seen.indexOf(id) >= 0);
    }

    function fate(profile) {
        return FATES[Math.min(3, flagOf(profile, FLAGS.fate))] || null;
    }

    // 0 nothing yet, 1 tremor seen, 2 night seen (the well is open), 3 Brandur beaten and the choice pending, 5 slain or sealed.
    function step(profile) {
        const f = flagOf(profile, FLAGS.fate);
        if (f >= 2) return 5;
        if (f === 1) return 3;
        if (seenHas(profile, 'dr_night')) return 2;
        if (seenHas(profile, 'dr_tremor')) return 1;
        return 0;
    }

    // The scene the hub should start now, or null.
    function hubScene(profile) {
        const lv = profile && +profile.unlockedLevel > 0 ? +profile.unlockedLevel : 0;
        if (lv >= 6 && !seenHas(profile, 'dr_tremor')) return 'dr_tremor';
        if (lv >= 11 && seenHas(profile, 'dr_tremor') && !seenHas(profile, 'dr_night')) return 'dr_night';
        return null;
    }

    // ---- hub variants of Pathrest ----

    const BARKS_A = { 'npc-villager': [7, 8, 9], 'npc-guard': [10, 11, 12] };
    const BARKS_B = { 'npc-villager': [13, 14, 15], 'npc-guard': [16, 17, 18] };

    function keysOf(nums) { return nums.map(function (n) { return 'runenpfad.village.bark.' + n; }); }

    function propById(stage, id) {
        return (stage.hubProps || []).filter(function (p) { return p.id === id; })[0] || null;
    }

    function applyHub(stage, s) {
        if (!stage) return;
        const well = propById(stage, 'well');
        if (well) {
            well.hidden = s >= 2;
            well.steam = s === 1;
        }
        const barn = propById(stage, 'barn');
        if (barn) {
            if (!barn.base) barn.base = { clip: barn.clip, h: barn.h };
            const burned = s === 2 || s === 3;
            barn.clip = burned ? 'fac-deep.f1' : barn.base.clip;
            barn.h = burned ? Math.round(barn.base.h * BURNED_RATIO) : barn.base.h;
        }
        (stage.doors || []).forEach(function (d) {
            if (d.placeId !== 'deeps') return;
            d.off = s < 2;
            d.steam = s === 2 || s === 3;
        });
        const set = s >= 5 ? BARKS_B : (s >= 1 ? BARKS_A : null);
        (stage.barks || []).forEach(function (b) {
            if (!b.base) b.base = b.lines.slice();
            const nums = set && set[b.sheet];
            b.lines = nums ? keysOf(nums) : b.base.slice();
            b.line = b.lines.length ? b.line % b.lines.length : 0;
        });
    }

    // Called every hub world tick (village.js stepBarks): follows the step and asks for the story scenes.
    function stepHub(sim, dt) {
        const st = sim && sim.stage;
        if (!st || !st.hub || sim.mode !== 'campaign') return;
        const s = step(sim.profile);
        if (st.drStep !== s) {
            st.drStep = s;
            applyHub(st, s);
        }
        if (!sim.interior && !sim.encounter && sim.uiRequest == null) {
            const id = hubScene(sim.profile);
            if (id && st.drAsked !== id) {
                st.drAsked = id;
                sim.uiRequest = 'scene:' + id;
            }
        }
    }

    // ---- fate and rewards ----

    function notice(sim, key) { sim.notice = NOTICE + key; }

    // Never downgrades; slain and sealed only follow beaten. Returns true when the fate changed.
    function setFate(sim, to) {
        const idx = FATES.indexOf(to);
        const n = N();
        if (idx < 1 || !sim || !sim.profile || !n) return false;
        const cur = flagOf(sim.profile, FLAGS.fate);
        if (idx === 1 ? cur !== 0 : cur !== 1) return false;
        return n.setFlag(sim, FLAGS.fate, idx);
    }

    function maxHpBonus(profile) { return flagOf(profile, FLAGS.fate) === 2 ? 1 : 0; }

    // Dialogue op `ember`: 'slay' (the Ember Heart and gold) or 'seal' (needs the mended bell).
    function effect(sim, what) {
        const n = N();
        if (!sim || !sim.profile || !n) return;
        if (what === 'slay') {
            if (!setFate(sim, 'slain')) return;
            n.addCoins(sim, SLAY_COINS);
            sim.stageCoins = (sim.stageCoins || 0) + SLAY_COINS;
            const p = sim.player;
            if (p) {
                p.maxHp += 1;
                p.hp = Math.min(p.maxHp, p.hp + 1);
                if (Array.isArray(sim.floatingTexts)) sim.floatingTexts.push({ x: p.x, y: p.y - 64, text: '+1', key: NOTICE + 'ember_heart', icon: 'heart', color: '#ff8a5c', timer: 1.4 });
            }
            notice(sim, 'ember_heart');
            pushEvent(sim, 'dr_outcome', { fate: 'slain' });
        } else if (what === 'seal') {
            if (!n.hasFlag(sim, FLAGS.bell) || !setFate(sim, 'sealed')) return;
            notice(sim, 'blessing_gained');
            pushEvent(sim, 'dr_outcome', { fate: 'sealed' });
        }
    }

    // Brandur's Blessing: once per stage run a lethal hit leaves one heart (the caller sets hp to 1 when this returns true).
    function spareLethal(sim) {
        if (!sim || sim.blessUsed || flagOf(sim.profile, FLAGS.fate) !== 3) return false;
        sim.blessUsed = true;
        notice(sim, 'blessing');
        const p = sim.player;
        pushEvent(sim, 'dr_blessing', sim.interior || !p ? null : { x: p.x, y: p.y });
        return true;
    }

    // ---- meals ----

    function mealOf(sim) { return sim && MEAL_NAMES.indexOf(sim.meal) >= 0 ? sim.meal : null; }

    function applyStew(sim) {
        const p = sim.player;
        if (!p || sim.mealHp) return;
        p.maxHp += 1;
        p.hp = Math.min(p.maxHp, p.hp + 1);
        sim.mealHp = true;
    }

    function setMeal(sim, kind) {
        const n = N();
        if (!sim || !n || !MEALS[kind] || mealOf(sim)) return false;
        if (!n.setFlag(sim, FLAGS.meal, MEALS[kind])) return false;
        sim.meal = kind;
        if (kind === 'stew') applyStew(sim);
        return true;
    }

    // Stage start: the pending meal takes effect; a level or endless run consumes it, the hub keeps it.
    function onStageStart(sim) {
        const n = N();
        if (!sim || !sim.profile || !n) return;
        const kind = MEAL_NAMES[flagOf(sim.profile, FLAGS.meal) - 1] || null;
        sim.meal = kind;
        sim.mealHp = false;
        sim.blessUsed = false;
        if (kind === 'stew') applyStew(sim);
        if (kind && !(sim.stage && sim.stage.hub)) n.setFlag(sim, FLAGS.meal, 0);
    }

    // ---- the bell of the old forge (interior use `bell`) ----

    function ringBell(sim, it, thing) {
        const mended = flagOf(sim.profile, FLAGS.bell) > 0;
        const sealed = fate(sim.profile) === 'sealed';
        pushEvent(sim, 'dr_bell', { mended: mended, sealed: sealed });
        if (step(sim.profile) < 2) notice(sim, 'bell_rung');
        else if (!mended) notice(sim, 'bell_cracked');
        else if (sealed) notice(sim, 'bell_song');
        else notice(sim, 'bell_mended');
    }

    const TH = root.RunenpfadInteriorThings;
    if (TH && TH.USES) TH.USES.bell = { prompt: 'ring', use: ringBell };

    root.RunenpfadEmber = {
        FLAGS: FLAGS,
        SCENES: SCENES,
        MEALS: MEALS,
        step: step,
        fate: fate,
        hubScene: hubScene,
        stepHub: stepHub,
        applyHub: applyHub,
        setFate: setFate,
        effect: effect,
        spareLethal: spareLethal,
        maxHpBonus: maxHpBonus,
        mealOf: mealOf,
        setMeal: setMeal,
        onStageStart: onStageStart,
        ringBell: ringBell
    };
})(typeof window !== 'undefined' ? window : globalThis);
