/* Runenpfad crafts: the kitchen (two ingredients make a meal for the next run) and the forge (a three strike timing minigame that
 * mends the bell or forges the scale shield). DOM-free; the window is crafts-ui.js. Story writes go through the RunenpfadNpcs helpers. */
(function (root) {
    'use strict';

    const INGREDIENTS = ['turnip', 'ham', 'frostberry', 'honey'];
    // Unordered pairs.
    const RECIPES = { stew: ['ham', 'turnip'], tea: ['frostberry', 'honey'], broth: ['turnip', 'honey'] };
    const MEAL_FLAG = { stew: 1, tea: 2, broth: 3 };
    const FORGE = {
        shield: { item: 'dragon_scale', coins: 40, flag: 'dr.shield' },
        bell: { item: 'bell_bronze', coins: 0, flag: 'dr.bell' }
    };
    const FORGE_IDS = ['shield', 'bell'];
    const STRIKES = 3;
    const GOOD_ZONE = 0.14;       // a strike is good within this distance of the middle of the bar
    const START_SPEED = 1.2;      // bar widths per second
    const SPEED_UP = 1.1;

    function P() { return root.RunenpfadPhysics; }
    function N() { return root.RunenpfadNpcs; }
    function E() { return root.RunenpfadEmber || null; }
    function pushEvent(sim, type, detail) { P().pushEvent(sim, type, detail); }

    // ---- kitchen ----

    function recipeFor(a, b) {
        if (!a || !b || a === b) return null;
        const ids = Object.keys(RECIPES);
        for (let i = 0; i < ids.length; i++) {
            const r = RECIPES[ids[i]];
            if ((r[0] === a && r[1] === b) || (r[0] === b && r[1] === a)) return ids[i];
        }
        return null;
    }

    function mealPending(sim) {
        if (sim.meal) return true;
        const E1 = E();
        if (E1 && E1.mealOf && E1.mealOf(sim)) return true;
        return N().getFlag(sim, 'dr.meal') > 0;
    }

    // 'ok', 'none' (no recipe for the pair), 'full' (a meal is already waiting) or 'missing' (an ingredient is not in the bag).
    function canCook(sim, a, b) {
        if (!recipeFor(a, b)) return 'none';
        if (mealPending(sim)) return 'full';
        if (N().itemCount(sim, a) < 1 || N().itemCount(sim, b) < 1) return 'missing';
        return 'ok';
    }

    function setMeal(sim, kind) {
        const E1 = E();
        if (E1 && E1.setMeal) return !!E1.setMeal(sim, kind);
        // Without the chapter module only the flag and the runtime field are kept.
        N().setFlag(sim, 'dr.meal', MEAL_FLAG[kind]);
        sim.meal = kind;
        return true;
    }

    function cook(sim, a, b) {
        if (canCook(sim, a, b) !== 'ok') return { ok: false, recipe: null };
        const recipe = recipeFor(a, b);
        if (!setMeal(sim, recipe)) return { ok: false, recipe: null };
        N().addItem(sim, a, -1);
        N().addItem(sim, b, -1);
        sim.notice = 'runenpfad.notice.meal_' + recipe;
        sim.storyDirty = true;
        pushEvent(sim, 'craft_cook', { recipe: recipe });
        return { ok: true, recipe: recipe };
    }

    // ---- forge ----

    function forgeOptions(sim) {
        const n = N();
        return FORGE_IDS.map(function (id) {
            const f = FORGE[id];
            const done = n.getFlag(sim, f.flag) > 0;
            const have = n.itemCount(sim, f.item) > 0;
            const coins = sim.profile ? sim.profile.coins : 0;
            return { id: id, done: done, ready: !done && have && coins >= f.coins, item: f.item, coins: f.coins, have: have };
        });
    }

    function forgeStart(sim, id) {
        const opt = forgeOptions(sim).filter(function (o) { return o.id === id; })[0];
        if (!opt || !opt.ready) return null;
        return { id: id, phase: 'run', pos: 0, dir: 1, speed: START_SPEED, strikes: [], ok: null };
    }

    // The needle ping-pongs 0..1 while the run is on.
    function forgeStep(state, dt) {
        if (!state || state.phase !== 'run') return;
        let pos = state.pos + state.dir * state.speed * Math.max(0, dt || 0);
        let guard = 0;
        while ((pos > 1 || pos < 0) && guard++ < 8) {
            if (pos > 1) { pos = 2 - pos; state.dir = -1; }
            else { pos = -pos; state.dir = 1; }
        }
        state.pos = Math.max(0, Math.min(1, pos));
    }

    function finishForge(sim, state) {
        const f = FORGE[state.id];
        const goods = state.strikes.filter(function (q) { return q === 'good'; }).length;
        state.phase = 'done';
        state.ok = goods >= 2;
        // The work costs the coins either way; the material is only used up by a good piece.
        if (f.coins > 0) N().addCoins(sim, -f.coins);
        if (state.ok) {
            N().addItem(sim, f.item, -1);
            N().setFlag(sim, f.flag, 1);
            sim.notice = 'runenpfad.notice.forged_' + state.id;
        }
        sim.storyDirty = true;
        pushEvent(sim, 'craft_done', { recipe: state.id, ok: state.ok });
    }

    function forgeStrike(sim, state) {
        if (!state || state.phase !== 'run') return 'bad';
        const quality = Math.abs(state.pos - 0.5) <= GOOD_ZONE ? 'good' : 'bad';
        state.strikes.push(quality);
        state.speed *= SPEED_UP;
        pushEvent(sim, 'craft_strike', { quality: quality });
        if (state.strikes.length >= STRIKES) finishForge(sim, state);
        return quality;
    }

    root.RunenpfadCrafts = {
        INGREDIENTS: INGREDIENTS,
        RECIPES: RECIPES,
        FORGE: FORGE,
        STRIKES: STRIKES,
        GOOD_ZONE: GOOD_ZONE,
        recipeFor: recipeFor,
        canCook: canCook,
        cook: cook,
        forgeOptions: forgeOptions,
        forgeStart: forgeStart,
        forgeStep: forgeStep,
        forgeStrike: forgeStrike
    };
})(typeof window !== 'undefined' ? window : globalThis);
