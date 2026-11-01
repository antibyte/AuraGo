/* ECalc solver: rule propagation over a catalog calculator, plus the input bookkeeping of the pages (which variables
   are inputs, which yield when the user types into a computed one). A DOM-free classic script that publishes
   window.ECalcCore.solve; solve() mirrors internal/ecalc/solve.go and is checked against the same vectors, while
   chooseInputs() and the state helpers exist on the UI side only.

   A calculator is the item of the catalog JSON: vars[], choices[], dof, rules[{target, alts}], derived[{id, alts}],
   checks[{when, warn}], sets[[var ids]]. Inputs are base-unit numbers, or arrays of them for list variables. */
(function () {
    'use strict';

    const core = (window.ECalcCore = window.ECalcCore || {});
    const expr = core.expr;

    const isFiniteValue = value => (Array.isArray(value) ? value.every(Number.isFinite) : Number.isFinite(value));

    // The consts of the selected option of every choice (an unknown or missing selection falls back to the default).
    function choiceConsts(calc, choices) {
        const consts = {};
        for (const choice of calc.choices || []) {
            const wanted = choices && choices[choice.id];
            const option = choice.options.find(entry => entry.id === wanted) || choice.options.find(entry => entry.id === choice.default) || choice.options[0];
            if (option) Object.assign(consts, option.consts || {});
        }
        return consts;
    }

    // Solve(calc, inputs, choices, units) -> {values, computed, errors, warnings}. `units` supplies dim(id).positive.
    function solve(calc, inputs, choices, units) {
        const env = Object.create(null), known = new Set();
        const consts = choiceConsts(calc, choices);
        for (const name of Object.keys(consts)) { env[name] = consts[name]; known.add(name); }
        // Only primary variables can be inputs; any other name in `inputs` is ignored.
        for (const v of calc.vars) {
            if (!inputs || !Object.prototype.hasOwnProperty.call(inputs, v.id) || inputs[v.id] === undefined) continue;
            env[v.id] = inputs[v.id];
            known.add(v.id);
        }

        const order = calc.rules.map(rule => ({ target: rule.target, alts: rule.alts })).concat(calc.derived.map(item => ({ target: item.id, alts: item.alts })));
        const computed = [];
        for (let changed = true; changed;) {
            changed = false;
            for (const item of order) {
                if (known.has(item.target)) continue;
                for (const alt of item.alts) {
                    const ast = expr.parseCached(alt);
                    if (!expr.deps(ast).every(name => known.has(name))) continue;
                    let value;
                    try { value = expr.evaluate(ast, env); } catch (error) { value = NaN; }
                    env[item.target] = value;
                    known.add(item.target);
                    computed.push(item.target);
                    changed = true;
                    break;
                }
            }
        }

        const values = {}, errors = {};
        for (const v of calc.vars) {
            if (!known.has(v.id)) {
                values[v.id] = NaN;
                errors[v.id] = 'undetermined';
                continue;
            }
            const value = env[v.id];
            const positive = !!(units && units.dim(v.dim).positive);
            values[v.id] = Array.isArray(value) ? value.slice() : value;
            if (!isFiniteValue(value)) errors[v.id] = 'nonfinite';
            else if (positive && (Array.isArray(value) ? value.some(x => x <= 0) : value <= 0)) errors[v.id] = 'range';
        }
        for (const item of calc.derived) {
            values[item.id] = known.has(item.id) && Number.isFinite(env[item.id]) ? env[item.id] : NaN;
        }

        const warnings = [];
        (calc.checks || []).forEach((check, index) => {
            const ast = expr.parseCached(check.when);
            if (!expr.deps(ast).every(name => known.has(name) && isFiniteValue(env[name]))) return;
            let result;
            try { result = expr.evaluate(ast, env); } catch (error) { return; }
            if (result !== 0 && !Number.isNaN(result)) warnings.push(index);
        });
        return { values, computed, errors, warnings };
    }

    // The user typed into `edited`. Among the solvable sets that contain it and every locked variable the set wins
    // that keeps the most recently edited variables as inputs; a tie goes to the first set. A locked field stays an
    // input until the user unlocks it, so locks are never dropped to make room: when no set holds `edited` together
    // with all locks the edit is refused. recency lists the current inputs from the oldest edit to the newest.
    // Returns {set, recency}, or null when no set contains `edited` and every locked variable.
    function chooseInputs(sets, recency, edited, locked) {
        const pinned = (locked || []).filter(Boolean);
        const candidates = sets.filter(set => set.includes(edited) && pinned.every(name => set.includes(name)));
        if (!candidates.length) return null;
        let best = null, bestScore = -1;
        for (const set of candidates) {
            let score = 0;
            recency.forEach((name, index) => { if (name !== edited && set.includes(name)) score += Math.pow(2, index); });
            if (score > bestScore) { best = set; bestScore = score; }
        }
        const kept = recency.filter(name => name !== edited && best.includes(name));
        const added = best.filter(name => name !== edited && !recency.includes(name));
        return { set: best.slice(), recency: added.concat(kept, [edited]) };
    }

    // ---- input bookkeeping of a page -----------------------------------------------------------------------------
    // A page state is {inputs: {var: raw text}, recency: [var], locked: [var], choices: {choice: option}}; it is what
    // the store persists. `unitOf(varId)` names the display unit of a variable ('' = automatic).

    const hasDefault = v => v.default != null || v.default_list != null;
    const SIG_EXACT = 12;

    // Text for a base-unit value that parses back to the same number: the symbol is part of the text, so the unit
    // dropdown cannot change its meaning. List items lose their blanks to survive the list splitting.
    function exactText(v, value, units, unitOf) {
        const one = x => units.format(x, v.dim, unitOf(v.id), SIG_EXACT);
        return Array.isArray(value) ? value.map(x => one(x).replace(/\s+/g, '')).join(', ') : one(value);
    }

    function defaultState(calc, units, unitOf) {
        const inputs = {}, recency = [];
        for (const v of calc.vars) {
            if (v.input === false || !hasDefault(v)) continue;
            inputs[v.id] = exactText(v, v.list ? v.default_list : v.default, units, unitOf);
            recency.push(v.id);
        }
        const choices = {};
        for (const choice of calc.choices || []) choices[choice.id] = choice.default;
        return { inputs, recency, locked: [], choices };
    }

    // A stored state is only used when its inputs are one of the solvable sets; anything else falls back to the defaults.
    // Only inputs can be locked: a stored lock on any other variable is dropped.
    function restoreState(calc, stored, units, unitOf) {
        const base = defaultState(calc, units, unitOf);
        if (!stored || typeof stored !== 'object' || !stored.inputs || typeof stored.inputs !== 'object') return base;
        const names = Object.keys(stored.inputs);
        const sameSet = calc.sets.some(set => set.length === names.length && set.every(name => names.includes(name)));
        if (!sameSet || !names.every(name => typeof stored.inputs[name] === 'string')) return base;
        const recency = (Array.isArray(stored.recency) ? stored.recency : []).filter(name => names.includes(name));
        for (const name of names) if (!recency.includes(name)) recency.unshift(name);
        const choices = Object.assign({}, base.choices);
        for (const choice of calc.choices || []) {
            const wanted = stored.choices && stored.choices[choice.id];
            if (choice.options.some(option => option.id === wanted)) choices[choice.id] = wanted;
        }
        return {
            inputs: Object.assign({}, stored.inputs),
            recency,
            locked: (Array.isArray(stored.locked) ? stored.locked : []).filter(name => names.includes(name)),
            choices
        };
    }

    // Parse the raw texts of a state. Returns {values, errors}; errors maps a variable to 'parse'.
    function readInputs(calc, state, units, unitOf) {
        const values = {}, errors = {};
        for (const v of calc.vars) {
            if (!Object.prototype.hasOwnProperty.call(state.inputs, v.id)) continue;
            try {
                values[v.id] = v.list ? units.parseList(state.inputs[v.id], v.dim, unitOf(v.id)) : units.parse(state.inputs[v.id], v.dim, unitOf(v.id));
            } catch (error) {
                errors[v.id] = 'parse';
            }
        }
        return { values, errors };
    }

    // The user entered `raw` into `edited`. `lastValues` are the base-unit results shown so far; a computed variable
    // that becomes an input keeps the value it showed. Returns the new state (the argument is not changed), or the
    // very same `state` object when the edit is refused (a locked field would have to give way).
    function applyEdit(calc, state, edited, raw, units, unitOf, lastValues) {
        const chosen = chooseInputs(calc.sets, state.recency, edited, state.locked);
        if (!chosen) return state;
        const inputs = {};
        for (const name of chosen.set) {
            if (name === edited) { inputs[name] = raw; continue; }
            if (Object.prototype.hasOwnProperty.call(state.inputs, name)) { inputs[name] = state.inputs[name]; continue; }
            const v = calc.vars.find(entry => entry.id === name);
            const shown = lastValues && lastValues[name];
            if (v && shown != null && isFiniteValue(shown)) inputs[name] = exactText(v, shown, units, unitOf);
            else if (v && hasDefault(v)) inputs[name] = exactText(v, v.list ? v.default_list : v.default, units, unitOf);
        }
        return { inputs, recency: chosen.recency, locked: state.locked.filter(name => chosen.set.includes(name)), choices: Object.assign({}, state.choices) };
    }

    function toggleLock(state, name) {
        const locked = state.locked.includes(name) ? state.locked.filter(entry => entry !== name) : state.locked.concat([name]);
        return Object.assign({}, state, { locked });
    }

    core.solve = { solve, chooseInputs, choiceConsts, defaultState, restoreState, readInputs, applyEdit, toggleLock, exactText };
})();
