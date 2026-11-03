// ECalc desktop app: parity test of the JavaScript component codes against the shared vectors of the Go package.
// ui/js/desktop/apps/ecalc/ecalc-codes.js (with ecalc-expr.js, ecalc-units.js and ecalc-eseries.js) runs in a Node vm without a document; the
// code items come from internal/ecalc/testdata/catalog.en.json (the catalog JSON of the server route), and every decode,
// encode and error case of internal/ecalc/testdata/vectors/codes.json must give the same result as Go: the code, the bands,
// the dimension, the fields (id, English name, text, value, dimension) and the English warnings are equal, numbers match at a
// relative error of 1e-12 (a zero want allows 1e-12 absolute), a null value is NaN in Go, and an error has the code of the
// vector and a message that contains its fragment. The items are frozen first, so a mutation of the data fails the test.
// Run: node scripts/test-ecalc-codes.mjs   (npm run test:ecalc)
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';

const root = path.resolve(import.meta.dirname, '..');
const read = file => fs.readFileSync(path.join(root, file), 'utf8');
const files = ['ecalc-expr.js', 'ecalc-units.js', 'ecalc-eseries.js', 'ecalc-codes.js'];

const window = {};
const sandbox = vm.createContext({ window });
vm.runInContext(files.map(file => read('ui/js/desktop/apps/ecalc/' + file)).join('\n'), sandbox);
const core = window.ECalcCore;
assert.ok(core && core.units && core.eseries && core.codes && typeof core.codes.decode === 'function' && typeof core.codes.encode === 'function', 'the files publish window.ECalcCore.codes');

const catalog = JSON.parse(read('internal/ecalc/testdata/catalog.en.json'));
const units = core.units.create(catalog.units);
const deepFreeze = value => {
    if (value && typeof value === 'object') {
        for (const key of Object.keys(value)) deepFreeze(value[key]);
        Object.freeze(value);
    }
    return value;
};
const items = new Map(catalog.items.filter(item => item.kind === 'code').map(item => [item.code, deepFreeze(item)]));
const KINDS = ['resistor_bands', 'inductor_bands', 'smd_resistor', 'capacitor', 'ceramic_temp'];
assert.deepStrictEqual([...items.keys()].sort(), [...KINDS].sort(), 'the catalog snapshot carries the five code items');
// A kind that is no code kind: the item is built here.
const itemOf = kind => items.get(kind) || { kind: 'code', code: kind, data: {} };

const plain = value => JSON.parse(JSON.stringify(value));
// A result is plain JSON: strings, finite numbers, null, booleans, arrays and objects; no NaN, no undefined.
function assertPlain(value, label) {
    if (value === null || typeof value === 'string' || typeof value === 'boolean') return;
    if (typeof value === 'number') { assert.ok(Number.isFinite(value), `${label}: ${value} is no JSON number`); return; }
    assert.ok(typeof value === 'object', `${label}: ${typeof value} is no JSON value`);
    for (const key of Object.keys(value)) assertPlain(value[key], `${label}.${key}`);
}
function near(actual, want, label) {
    if (want === null) { assert.equal(actual, null, `${label}: want null, got ${actual}`); return; }
    assert.ok(typeof actual === 'number' && Number.isFinite(actual), `${label}: want ${want}, got ${actual}`);
    const allowed = want === 0 ? 1e-12 : 1e-12 * Math.abs(want);
    assert.ok(Math.abs(actual - want) <= allowed, `${label}: want ${want}, got ${actual}`);
}
function sameResult(actual, kind, want, label) {
    // The result is plain JSON: NaN is null, nothing is undefined.
    assert.deepStrictEqual(Object.keys(actual), ['kind', 'code', 'bands', 'value', 'dim', 'fields', 'warnings'], `${label}: keys`);
    assert.equal(actual.kind, kind, `${label}: kind`);
    assert.equal(actual.code, want.code, `${label}: code`);
    assert.deepStrictEqual(plain(actual.bands), want.bands, `${label}: bands`);
    assert.equal(actual.dim, want.dim, `${label}: dim`);
    assert.deepStrictEqual(plain(actual.warnings), want.warnings, `${label}: warnings`);
    near(actual.value, want.value, `${label}: value`);
    assert.equal(actual.fields.length, want.fields.length, `${label}: ${actual.fields.length} fields ${JSON.stringify(actual.fields)}, want ${want.fields.length}`);
    actual.fields.forEach((f, i) => {
        const w = want.fields[i];
        assert.deepStrictEqual(Object.keys(f), ['id', 'name', 'text', 'value', 'dim'], `${label}: field keys`);
        assert.ok(f.id === w[0] && f.name === w[1] && f.text === w[2] && f.dim === w[4], `${label}: field ${i} = ${JSON.stringify(f)}, want ${JSON.stringify(w)}`);
        near(f.value, w[3], `${label}: field ${f.id}`);
    });
    assertPlain(actual, label);
}

const vectors = JSON.parse(read('internal/ecalc/testdata/vectors/codes.json'));
for (const section of ['decode', 'encode', 'errors']) assert.ok(Array.isArray(vectors[section]) && vectors[section].length > 0, `codes.json has no ${section} vectors`);
const perKind = {};
const count = key => { perKind[key] = (perKind[key] || 0) + 1; };
let cases = 0;

for (const v of vectors.decode) {
    assert.ok(v.ref, `decode ${v.kind} ${v.code} needs a ref`);
    count('decode ' + v.kind);
    const label = `decode ${v.kind} ${JSON.stringify(v.code)}`;
    sameResult(core.codes.decode(itemOf(v.kind), v.code, units), v.kind, v.want, label);
    cases += 1;
}
for (const v of vectors.encode) {
    assert.ok(v.ref, `encode ${v.kind} ${JSON.stringify(v.req)} needs a ref`);
    count('encode ' + v.kind);
    const label = `encode ${v.kind} ${JSON.stringify(v.req)}`;
    sameResult(core.codes.encode(itemOf(v.kind), v.req, units), v.kind, v.want, label);
    cases += 1;
}
for (const v of vectors.errors) {
    assert.ok(v.ref, `error ${v.op} ${v.kind} needs a ref`);
    count('error ' + v.kind);
    const label = `${v.op} ${v.kind} ${JSON.stringify(v.op === 'decode' ? v.code : v.req)}`;
    const run = () => (v.op === 'decode' ? core.codes.decode(itemOf(v.kind), v.code, units) : core.codes.encode(itemOf(v.kind), v.req, units));
    assert.throws(run, error => {
        assert.equal(error.code, v.error, `${label}: error code ${error.code} (${error.message})`);
        assert.ok(error.message.includes(v.contains), `${label}: message ${JSON.stringify(error.message)} lacks ${JSON.stringify(v.contains)}`);
        assert.ok(!error.message.startsWith(v.error + ':'), `${label}: the message carries no code prefix`);
        return true;
    }, label);
    cases += 1;
}
for (const kind of KINDS) {
    for (const what of ['decode', 'encode', 'error']) assert.ok((perKind[`${what} ${kind}`] || 0) >= 8, `${what} ${kind} has ${perKind[`${what} ${kind}`] || 0} vectors, want at least 8`);
}

// ---- what exists on the JavaScript side only ------------------------------------------------------------------------------------
// The request may omit every field; numbers and text for bands are tolerated; the data is not changed (the items are frozen).
sameResult(core.codes.encode(itemOf('resistor_bands'), { value: '6.8k', tolerance: '20%', bands: '3' }, units), 'resistor_bands',
    { code: 'blue grey red', bands: ['blue', 'grey', 'red'], value: 6800, dim: 'resistance', fields: [['tolerance', 'Tolerance', '±20 %', 0.2, 'ratio']], warnings: [] }, 'bands as text');
assert.throws(() => core.codes.encode(itemOf('resistor_bands'), null, units), error => error.code === 'invalid_value', 'a missing request');
assert.throws(() => core.codes.decode(itemOf('resistor_bands'), undefined, units), error => error.code === 'invalid_code', 'a missing code');
assert.throws(() => core.codes.decode({ kind: 'calculator', code: 'resistor_bands' }, 'red', units), error => error.code === 'unsupported', 'an item that is no code item');
assert.throws(() => core.codes.encode(itemOf('capacitor'), { value: '1n', bands: 1.5 }, units), error => error.code === 'invalid_value', 'bands must be a whole number');
cases += 5;

// Decoding what was encoded gives the same result: every E12 value in a few decades, every kind that has a value.
const E12 = core.eseries.TABLES.E12;
for (const [kind, suffix] of [['resistor_bands', ''], ['smd_resistor', ''], ['inductor_bands', 'u'], ['capacitor', 'p']]) {
    for (const decade of [1, 10, 100, 1000, 1e4, 1e5]) {
        for (const m of E12) {
            const req = { value: String(Number((m * decade).toPrecision(6))) + suffix, tolerance: '10%' };
            let encoded;
            try { encoded = core.codes.encode(itemOf(kind), req, units); } catch (error) { assert.equal(error.code, 'unsupported', `${kind} ${req.value}: ${error.message}`); continue; }
            const code = kind === 'capacitor' ? encoded.code.split(' ')[0].replace(/K$/, '') : encoded.code;
            const decoded = core.codes.decode(itemOf(kind), code, units);
            near(decoded.value, encoded.value, `${kind} ${req.value} -> ${code}`);
            cases += 1;
        }
    }
}

console.log(`ecalc codes tests: ${cases} cases ok`);
