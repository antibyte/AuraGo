// ECalc desktop app: parity test of the JavaScript core against the shared vectors of the Go package.
// The DOM-free files ui/js/desktop/apps/ecalc/ecalc-{expr,units,eseries,solve,typeset}.js run in a Node vm (no document),
// the calculators come from internal/ecalc/testdata/catalog.en.json (the catalog JSON of the server route), and every case
// of internal/ecalc/testdata/vectors/core.json and vectors/calculators/*.json must give the same result as Go: numbers match
// at a relative error of 1e-12 (calculator vectors 1e-9), a zero want allows an absolute error of 1e-12, and the strings
// "nan", "inf" and "-inf" stand for the non-finite doubles. The second half checks what exists on the UI side only:
// chooseInputs, the input bookkeeping of a page, the display units and the escaping of the typesetter.
// Run: node scripts/test-ecalc-core.mjs   (npm run test:ecalc)
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';

const root = path.resolve(import.meta.dirname, '..');
const read = file => fs.readFileSync(path.join(root, file), 'utf8');
const files = ['ecalc-expr.js', 'ecalc-units.js', 'ecalc-eseries.js', 'ecalc-solve.js', 'ecalc-typeset.js'];
const plain = value => JSON.parse(JSON.stringify(value));
// The core runs in its own vm realm, so what it returns is compared as plain JSON.
const eq = (actual, expected, message) => assert.deepStrictEqual(plain(actual), plain(expected), message);

const window = {};
vm.runInNewContext(files.map(file => read('ui/js/desktop/apps/ecalc/' + file)).join('\n'), { window });
const core = window.ECalcCore, Typeset = window.ECalcTypeset;
assert.ok(core && core.expr && core.units && core.eseries && core.solve && Typeset, 'the core files publish window.ECalcCore and window.ECalcTypeset');
const catalog = JSON.parse(read('internal/ecalc/testdata/catalog.en.json'));
const units = core.units.create(catalog.units);
const calcOf = id => {
    const calc = catalog.items.find(item => item.id === id);
    assert.ok(calc, 'catalog has ' + id);
    return calc;
};
const calculators = catalog.items.filter(item => item.kind === 'calculator');

const decode = value => {
    if (typeof value !== 'string') return value;
    const word = value.toLowerCase();
    if (word === 'nan') return NaN;
    if (word === 'inf' || word === '+inf') return Infinity;
    if (word === '-inf') return -Infinity;
    return value;
};
function same(actual, expected, label, tolerance = 1e-12) {
    const want = decode(expected);
    if (Number.isNaN(want)) { assert.ok(Number.isNaN(actual), `${label}: want NaN, got ${actual}`); return; }
    if (!Number.isFinite(want)) { assert.equal(actual, want, label); return; }
    assert.ok(Number.isFinite(actual), `${label}: want ${want}, got ${actual}`);
    const allowed = want === 0 ? 1e-12 : tolerance * Math.abs(want);
    assert.ok(Math.abs(actual - want) <= allowed, `${label}: want ${want}, got ${actual}`);
}
// The prefix notation of the vectors: (op l r), (neg a), (pos a), (fn args), (= parts), numbers as written.
function sexp(node) {
    switch (node.t) {
        case 'num': return node.raw;
        case 'id': return node.name;
        case 'neg': case 'pos': return `(${node.t} ${sexp(node.a)})`;
        case 'bin': return `(${node.op} ${sexp(node.l)} ${sexp(node.r)})`;
        case 'call': return `(${node.name} ${node.args.map(sexp).join(' ')})`;
        case 'eq': return `(= ${node.parts.map(sexp).join(' ')})`;
        default: throw new Error('unknown node ' + node.t);
    }
}
const throwsCode = (fn, code, label) => assert.throws(fn, error => error.code === code, label);
let count = 0;

// ---- core.json: expressions, units, E-series --------------------------------------------------------------------------
const vectors = JSON.parse(read('internal/ecalc/testdata/vectors/core.json'));
const sections = ['expr', 'eval_error', 'parse', 'parse_error', 'formula', 'formula_error', 'deps', 'round_sig', 'eseries', 'eseries_error', 'units_parse', 'units_parse_error', 'units_list', 'units_list_error', 'units_format'];
for (const section of sections) assert.ok(Array.isArray(vectors[section]) && vectors[section].length > 0, `core.json has no ${section} vectors`);
const expr = core.expr;
const environment = v => Object.assign({}, v.env || {}, v.lists || {});
for (const v of vectors.expr) { same(expr.evaluate(expr.parse(v.src), environment(v)), v.want, 'expr ' + v.src); count += 1; }
for (const v of vectors.eval_error) { throwsCode(() => expr.evaluate(expr.parse(v.src), environment(v)), 'eval', 'eval error: ' + v.src); count += 1; }
for (const v of vectors.parse) { assert.equal(sexp(expr.parse(v.src)), v.ast, 'parse ' + v.src); count += 1; }
for (const src of vectors.parse_error) { throwsCode(() => expr.parse(src), 'parse', 'parse error: ' + JSON.stringify(src)); count += 1; }
for (const v of vectors.formula) { assert.equal(sexp(expr.parseFormula(v.src)), v.ast, 'formula ' + v.src); count += 1; }
for (const src of vectors.formula_error) { throwsCode(() => expr.parseFormula(src), 'parse', 'formula error: ' + JSON.stringify(src)); count += 1; }
for (const v of vectors.deps) { eq(expr.deps(expr.parse(v.src)), v.deps, 'deps ' + v.src); count += 1; }
for (const v of vectors.round_sig) { assert.equal(core.units.roundSig(v.x, v.n), v.want, `roundSig(${v.x}, ${v.n})`); count += 1; }
for (const v of vectors.eseries) {
    const found = core.eseries.nearest(v.series, decode(v.v));
    assert.ok(found, `eseries ${v.series} ${v.v} has a value`);
    same(found.value, v.want, `eseries ${v.series} ${v.v}`);
    same(found.dev, v.dev, `eseries ${v.series} ${v.v} deviation`);
    count += 1;
}
for (const v of vectors.eseries_error) { assert.equal(core.eseries.nearest(v.series, decode(v.v)), null, `eseries error ${v.series} ${v.v}`); count += 1; }
assert.ok(vectors.eseries_tables, 'core.json carries the E-series tables');
for (const [name, table] of Object.entries(vectors.eseries_tables)) { eq(core.eseries.TABLES[name], table, 'table ' + name); count += 1; }
eq(Object.keys(core.eseries.TABLES), Object.keys(vectors.eseries_tables), 'the same series');
for (const v of vectors.units_parse) { same(units.parse(v.in, v.dim, v.unit || ''), v.want, `parse ${JSON.stringify(v.in)} as ${v.dim} ${v.unit || ''}`); count += 1; }
for (const v of vectors.units_parse_error) { throwsCode(() => units.parse(v.in, v.dim, v.unit || ''), 'parse', `parse error ${JSON.stringify(v.in)} as ${v.dim} ${v.unit || ''}`); count += 1; }
for (const v of vectors.units_list) { eq(units.parseList(v.in, v.dim, v.unit || '').map((x, i) => (Math.abs(x - v.want[i]) <= 1e-12 * Math.abs(v.want[i]) ? v.want[i] : x)), v.want, `list ${JSON.stringify(v.in)}`); count += 1; }
for (const v of vectors.units_list_error) { throwsCode(() => units.parseList(v.in, v.dim, v.unit || ''), 'parse', `list error ${JSON.stringify(v.in)}`); count += 1; }
for (const v of vectors.units_format) { assert.equal(units.format(decode(v.v), v.dim, v.unit || '', v.sig || 0), v.want, `format ${v.v} as ${v.dim} ${v.unit || ''}`); count += 1; }

// ---- calculators/*.json: consistent solutions and single solves -----------------------------------------------------
const split = values => {
    const env = {};
    for (const [name, value] of Object.entries(values)) env[name] = Array.isArray(value) ? value.map(decode) : decode(value);
    return env;
};
const SOLVE_TOLERANCE = 1e-9;
for (const item of calculators) {
    const file = path.join(root, 'internal/ecalc/testdata/vectors/calculators', item.id + '.json');
    assert.ok(fs.existsSync(file), `no vectors for the calculator ${item.id}`);
    const set = JSON.parse(fs.readFileSync(file, 'utf8'));
    assert.equal(set.calc, item.id);
    assert.ok(set.solve.length > 0, `${item.id} has no solve vectors`);
    assert.ok(set.sets.length >= 2, `${item.id} needs at least two consistent solutions`);
    set.sets.forEach((entry, index) => {
        const label = `${item.id} set ${index} (${entry.ref})`;
        const values = split(entry.values);
        // (a) every alternative of every rule and derived quantity reproduces its target
        const env = Object.assign({}, core.solve.choiceConsts(item, entry.choices || {}), values);
        for (const target of item.rules.map(rule => ({ name: rule.target, alts: rule.alts })).concat(item.derived.map(d => ({ name: d.id, alts: d.alts })))) {
            target.alts.forEach((alt, altIndex) => same(expr.evaluate(expr.parseCached(alt), env), values[target.name], `${label}: ${target.name} alternative ${altIndex} (${alt})`, SOLVE_TOLERANCE));
        }
        // (b) every solvable input set reproduces the whole solution
        for (const names of item.sets) {
            const inputs = {};
            for (const name of names) inputs[name] = values[name];
            const result = core.solve.solve(item, inputs, entry.choices || {}, units);
            eq(result.errors, {}, `${label} from ${names}: no errors`);
            for (const [name, want] of Object.entries(values)) {
                if (Array.isArray(want)) eq(result.values[name], want, `${label} from ${names}: ${name}`);
                else same(result.values[name], want, `${label} from ${names}: ${name}`, SOLVE_TOLERANCE);
            }
            count += 1;
        }
    });
    for (const [index, v] of set.solve.entries()) {
        const label = `${item.id} solve ${index} (${v.ref})`;
        const result = core.solve.solve(item, split(v.inputs), v.choices || {}, units);
        for (const [name, want] of Object.entries(v.expect || {})) same(result.values[name], want, `${label}: ${name}`, SOLVE_TOLERANCE);
        eq(result.errors, v.errors || {}, `${label}: errors`);
        eq(result.warnings, v.warnings || [], `${label}: warnings`);
        count += 1;
    }
}

// ---- the defaults of every calculator solve, and every solvable set reproduces them (every choice option) ---------------
for (const item of calculators) {
    const selections = [{}];
    for (const choice of item.choices) for (const option of choice.options) selections.push({ [choice.id]: option.id });
    for (const choices of selections) {
        const auto = () => '';
        const state = core.solve.defaultState(item, units, auto);
        state.choices = Object.assign({}, state.choices, choices);
        const base = core.solve.solve(item, core.solve.readInputs(item, state, units, auto).values, state.choices, units);
        eq(base.errors, {}, `${item.id} ${JSON.stringify(choices)}: the defaults solve without errors`);
        for (const names of item.sets) {
            const inputs = {};
            for (const name of names) inputs[name] = base.values[name];
            const again = core.solve.solve(item, inputs, state.choices, units);
            eq(again.errors, {}, `${item.id} from ${names}: no errors`);
            for (const v of item.vars) {
                if (Array.isArray(base.values[v.id])) eq(again.values[v.id], base.values[v.id]);
                else same(again.values[v.id], base.values[v.id], `${item.id} ${JSON.stringify(choices)} from ${names}: ${v.id}`, SOLVE_TOLERANCE);
            }
            count += 1;
        }
    }
}

// ---- chooseInputs: which variables stay inputs after the user typed into one ------------------------------------
{
    const choose = core.solve.chooseInputs;
    const divider = [['vin', 'r1', 'r2'], ['vin', 'r1', 'vout'], ['vin', 'r2', 'vout'], ['r1', 'r2', 'vout']];
    eq(calcOf('voltage_divider').sets, divider, 'the catalog lists the divider sets in declaration order');
    eq(choose(divider, ['vin', 'r1', 'r2'], 'vout', []), { set: ['r1', 'r2', 'vout'], recency: ['r1', 'r2', 'vout'] }, 'the oldest input yields');
    eq(choose(divider, ['vin', 'r1', 'r2'], 'r1', []), { set: ['vin', 'r1', 'r2'], recency: ['vin', 'r2', 'r1'] }, 'editing an input keeps the set and makes it the newest');
    eq(choose(divider, ['vin', 'r1', 'r2'], 'vout', ['vin']), { set: ['vin', 'r2', 'vout'], recency: ['vin', 'r2', 'vout'] }, 'a locked input never yields');
    eq(choose(divider, ['vin', 'r1', 'r2'], 'vout', ['vin', 'r1']), { set: ['vin', 'r1', 'vout'], recency: ['vin', 'r1', 'vout'] }, 'two locks');
    eq(choose(divider, ['vin', 'r1', 'r2'], 'vout', ['vin', 'r1', 'r2']), null, 'locks that no set can honour refuse the edit');
    eq(choose(divider, ['vin', 'r1', 'r2'], 'r1', ['vin', 'r1', 'r2']).set, ['vin', 'r1', 'r2'], 'editing a locked input is fine');
    eq(choose(divider, ['vin', 'r1', 'r2'], 'nothing', []), null, 'a variable in no set');
    eq(choose([['x', 'b'], ['x', 'c']], ['a'], 'x', []).set, ['x', 'b'], 'a tie goes to the first set');
    const starDelta = [['rab', 'rbc', 'rca'], ['ra', 'rb', 'rc']];
    eq(calcOf('star_delta').sets, starDelta, 'the catalog lists exactly the delta set and the star set');
    eq(choose(starDelta, ['rab', 'rbc', 'rca'], 'ra', []), { set: ['ra', 'rb', 'rc'], recency: ['rb', 'rc', 'ra'] }, 'star-delta switches the whole set');
    eq(choose(starDelta, ['rb', 'rc', 'ra'], 'rbc', []), { set: ['rab', 'rbc', 'rca'], recency: ['rab', 'rca', 'rbc'] }, 'and back');
    eq(choose(calcOf('ohms_law').sets, ['u', 'i'], 'p', []).set, ['i', 'p'], 'ohms law: typing a power keeps the newer current');
    count += 13;
}

// ---- the input bookkeeping of a page ------------------------------------------------------------------------------
{
    const S = core.solve, auto = () => '';
    const divider = calcOf('voltage_divider');
    let state = S.defaultState(divider, units, auto);
    eq(state, { inputs: { vin: '12 V', r1: '4.7 kΩ', r2: '2.2 kΩ' }, recency: ['vin', 'r1', 'r2'], locked: [], choices: {} });
    let read = S.readInputs(divider, state, units, auto);
    eq(read.values, { vin: 12, r1: 4700, r2: 2200 });
    let shown = S.solve(divider, read.values, state.choices, units).values;
    same(shown.vout, 3.826086956521739, 'divider default output');

    state = S.applyEdit(divider, state, 'vout', '4 V', units, auto, shown);
    eq(state, { inputs: { r1: '4.7 kΩ', r2: '2.2 kΩ', vout: '4 V' }, recency: ['r1', 'r2', 'vout'], locked: [], choices: {} }, 'typing into the output makes the oldest input (vin) computed');
    read = S.readInputs(divider, state, units, auto);
    shown = S.solve(divider, read.values, state.choices, units).values;
    same(shown.vin, 4 * 6900 / 2200, 'divider input from the output');

    state = S.toggleLock(state, 'r1');
    eq(state.locked, ['r1']);
    state = S.applyEdit(divider, state, 'vin', '9 V', units, auto, shown);
    eq(state.inputs, { vin: '9 V', r1: '4.7 kΩ', vout: '4 V' }, 'the locked r1 stays an input, the oldest unlocked r2 yields');
    eq(S.toggleLock(state, 'r1').locked, [], 'unlocking');

    eq(S.readInputs(divider, { inputs: { vin: 'abc', r1: '1k', r2: '1k' } }, units, auto).errors, { vin: 'parse' }, 'a text that does not parse is reported');
    eq(S.restoreState(divider, { inputs: { vin: '5 V', r1: '1k', r2: '1k' }, recency: ['r2', 'vin', 'r1'], locked: ['r1', 'ghost'], choices: {} }, units, auto),
        { inputs: { vin: '5 V', r1: '1k', r2: '1k' }, recency: ['r2', 'vin', 'r1'], locked: ['r1'], choices: {} }, 'a stored state is restored');
    eq(S.restoreState(divider, { inputs: { vin: '5 V', r1: '1k' } }, units, auto), S.defaultState(divider, units, auto), 'a stored state that is no solvable set falls back to the defaults');
    eq(S.restoreState(divider, 'junk', units, auto), S.defaultState(divider, units, auto));

    // A locked field stays an input until the user unlocks it: an edit that would need its place is refused.
    {
        const ohm = calcOf('ohms_law');
        let ohmState = S.toggleLock(S.toggleLock(S.defaultState(ohm, units, auto), 'u'), 'i');
        eq(ohmState.locked, ['u', 'i']);
        const ohmShown = S.solve(ohm, S.readInputs(ohm, ohmState, units, auto).values, ohmState.choices, units).values;
        const refused = S.applyEdit(ohm, ohmState, 'r', '100', units, auto, ohmShown);
        assert.equal(refused, ohmState, 'Ohm: typing R with U and I locked is refused (the state object is returned unchanged)');
        eq(refused, { inputs: { u: '12 V', i: '20 mA' }, recency: ['u', 'i'], locked: ['u', 'i'], choices: {} }, 'Ohm: inputs, recency and locks stay as they were');
        assert.equal(S.applyEdit(ohm, ohmState, 'p', '1', units, auto, ohmShown), ohmState, 'Ohm: so is typing P');
        ohmState = S.toggleLock(ohmState, 'i');
        assert.notEqual(S.applyEdit(ohm, ohmState, 'p', '1', units, auto, ohmShown), ohmState, 'Ohm: typing P with only U locked is fine');
        eq(S.applyEdit(ohm, ohmState, 'p', '1', units, auto, ohmShown).inputs, { u: '12 V', p: '1' }, 'Ohm: the unlocked current yields, the locked U stays');
        eq(S.applyEdit(ohm, ohmState, 'p', '1', units, auto, ohmShown).locked, ['u'], 'Ohm: the locks follow the inputs');

        const rms = calcOf('rms_waveform');
        let rmsState = S.toggleLock(S.defaultState(rms, units, auto), 'vrms');
        const rmsShown = S.solve(rms, S.readInputs(rms, rmsState, units, auto).values, rmsState.choices, units).values;
        assert.equal(S.applyEdit(rms, rmsState, 'vpk', '10', units, auto, rmsShown), rmsState, 'RMS: typing V_pk with V_rms locked is refused');
        assert.notEqual(S.applyEdit(rms, rmsState, 'vrms', '10', units, auto, rmsShown), rmsState, 'RMS: editing the locked V_rms itself is fine');
        rmsState = S.toggleLock(rmsState, 'vrms');
        eq(S.applyEdit(rms, rmsState, 'vpk', '10', units, auto, rmsShown).inputs, { vpk: '10' }, 'RMS: unlocked, V_pk takes over');

        eq(S.restoreState(divider, { inputs: { vin: '5 V', r1: '1k', r2: '1k' }, recency: [], locked: ['vout', 'r2'], choices: {} }, units, auto).locked, ['r2'], 'a stored lock on a variable that is not an input is dropped');
        eq(S.restoreState(divider, { locked: ['vout'] }, units, auto).locked, [], 'and the defaults carry no lock');
        count += 13;
    }

    // A computed variable that becomes an input keeps the value it showed.
    const star = calcOf('star_delta');
    let starState = S.defaultState(star, units, auto);
    const starShown = S.solve(star, S.readInputs(star, starState, units, auto).values, starState.choices, units).values;
    starState = S.applyEdit(star, starState, 'ra', '12', units, auto, starShown);
    eq(starState, { inputs: { ra: '12', rb: '10 Ω', rc: '30 Ω' }, recency: ['rb', 'rc', 'ra'], locked: [], choices: {} }, 'star-delta: rb and rc keep their computed values');
    same(S.solve(star, S.readInputs(star, starState, units, auto).values, {}, units).values.rab, (12 * 10 + 10 * 30 + 30 * 12) / 30, 'star-delta result after the switch');

    const list = calcOf('series_parallel');
    const listState = S.defaultState(list, units, auto);
    eq(listState.inputs, { rs: '4.7kΩ, 10kΩ, 2.2kΩ' }, 'list items are written without blanks');
    eq(S.readInputs(list, listState, units, auto).values, { rs: [4700, 10000, 2200] });

    const ratio = calcOf('decibel_ratio');
    eq(S.defaultState(ratio, units, auto).choices, { qty: 'voltage' });
    eq(S.restoreState(ratio, { inputs: { a1: '1', a2: '10' }, recency: [], locked: [], choices: { qty: 'power' } }, units, auto).choices, { qty: 'power' });
    eq(S.restoreState(ratio, { inputs: { a1: '1', a2: '10' }, recency: [], locked: [], choices: { qty: 'bogus' } }, units, auto).choices, { qty: 'voltage' });
    count += 14;
}

// ---- display units of the pages (fixed prefixes) and the folded input characters ----------------------------------------
{
    assert.equal(units.format(0.0038, 'voltage', 'prefix:m', 4), '3.8 mV');
    assert.equal(units.format(0.0038, 'voltage', 'prefix:', 4), '0.0038 V');
    assert.equal(units.format(12, 'voltage', 'prefix:k', 4), '0.012 kV');
    same(units.parse('5', 'voltage', 'prefix:m'), 0.005, 'a plain number is in the chosen prefix unit');
    same(units.parse('5V', 'voltage', 'prefix:m'), 5, 'a typed unit wins');
    same(units.parse('5k', 'voltage', 'prefix:m'), 5000, 'a typed prefix wins');
    throwsCode(() => units.parse('5', 'voltage', 'prefix:x'), 'parse', 'an unknown fixed prefix');
    eq(units.options('voltage').map(option => option.id), ['', 'prefix:n', 'prefix:µ', 'prefix:m', 'prefix:', 'prefix:k', 'prefix:M', 'dBV', 'dBu']);
    eq(units.options('number').map(option => option.id), ['']);
    eq(units.options('resistance').map(option => option.label), ['', 'mΩ', 'Ω', 'kΩ', 'MΩ', 'GΩ']);
    same(units.parse('4.7kΩ', 'resistance', ''), 4700, 'the ohm sign');
    same(units.parse('3μA', 'current', ''), 3e-6, 'the Greek mu');
    count += 12;
}

// ---- the typesetter: structure and escaping ---------------------------------------------------------------------------
{
    const ctx = { labelOf: name => ({ vout: 'V_out', vin: 'V_in', r2: 'R_2', r1: 'R_1' }[name] || ''), constOf: name => (name === 'k' ? '20' : undefined) };
    const html = Typeset.formulaHTML(expr.parseFormula('vout = vin * r2 / (r1 + r2)'), ctx);
    assert.ok(html.includes('<span class="ec-frac">') && html.includes('V<sub>out</sub>') && html.includes('·') && !html.includes('((') , 'a fraction with subscripts and a centred dot');
    assert.ok(Typeset.formulaHTML(expr.parse('x^2 + sqrt(y)'), ctx).includes('<sup class="ec-sup">') && Typeset.formulaHTML(expr.parse('sqrt(y)'), ctx).includes('ec-sqrt'), 'superscript and radical');
    assert.ok(Typeset.formulaHTML(expr.parse('k * pi'), ctx).includes('20') && Typeset.formulaHTML(expr.parse('k * pi'), ctx).includes('π'), 'a choice const as its value, pi as the Greek letter');
    const hostile = Typeset.formulaHTML(expr.parse('a_b + c'), { labelOf: () => '<img src=x onerror=alert(1)>_<b>' });
    assert.ok(!hostile.includes('<img') && !hostile.includes('<b>') && hostile.includes('&lt;img'), 'labels from the catalog are escaped');
    const live = Typeset.liveHTML(expr.parseFormula('vout = vin / r2'), Object.assign({}, ctx, { valueOf: name => (name === 'vin' ? { html: '12 V', prec: 3 } : name === 'vout' ? { html: '3.8 V', prec: 3 } : null) }));
    assert.ok(live.includes('12 V') && live.includes('3.8 V') && live.includes('ec-u'), 'the live line shows values, the unknown variable keeps its label');
    count += 5;
}

console.log(`ecalc core tests: ${count} cases ok`);
