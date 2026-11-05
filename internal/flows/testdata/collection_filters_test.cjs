'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
global.window = {};
require(path.resolve(__dirname, '../../../ui/js/desktop/apps/easydrag-template.js'));
const { applyFilter, evaluate } = window.EasyDrag.template;
const cases = JSON.parse(fs.readFileSync(path.join(__dirname, 'collection_filters.json'), 'utf8'));
for (const tc of cases) {
    const before = JSON.stringify(tc.input);
    if (tc.error) assert.throws(() => applyFilter(tc.filter, tc.input, tc.args), undefined, tc.name);
    else assert.deepStrictEqual(JSON.parse(JSON.stringify(applyFilter(tc.filter, tc.input, tc.args))), tc.want, tc.name);
    assert.equal(JSON.stringify(tc.input), before, tc.name + ': input mutated');
}
for (const input of [Array(10001).fill(null), ['a'.repeat(2 * 1024 * 1024 + 1)], [NaN], [Infinity]]) {
    assert.throws(() => applyFilter('unique', input, []));
}
const manyFields = {}; for (let i = 0; i <= 256; i++) manyFields['x'.repeat(i)] = null;
assert.throws(() => applyFilter('pick', manyFields, ['x']));
assert.deepStrictEqual(applyFilter('unique', Array(10000).fill(null), []), [null]);
const roots = { rows: [{ active: true, n: 2 }, { active: false, n: 10 }, { active: true, n: 3 }] };
assert.equal(evaluate('{{rows | where("active", "eq", true) | sort("n", "desc") | sum("n")}}', roots), 5);
console.log('collection filter parity: ' + cases.length + ' cases');
