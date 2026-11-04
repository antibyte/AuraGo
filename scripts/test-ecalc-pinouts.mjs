// ECalc desktop app: geometry test of the pinout drawings. ui/js/desktop/apps/ecalc/ecalc-pinout-draw.js runs in a Node vm
// without a document; the items are the fixture pinouts of scripts/ecalc-pinout-fixtures.js (every outline of rows, every
// shape type, dense connectors) and every pinout of internal/ecalc/testdata/catalog.en.json (none until the content is
// assembled). For every item, variant and view the layout must draw every pin exactly once, on a unique position, inside
// the viewBox (the shell too); the other view of rows and circular is the mirror image about the vertical axis (x ->
// width - x, the same y, the names change their side); the markup has one g.ec-pin per contact with the hex of its type as the
// fill attribute and the ink of its type on the number inside, escapes what the data holds, and never puts a transform with
// a scale on a text. fitNumber is the rule that keeps a number inside its contact.
// Run: node scripts/test-ecalc-pinouts.mjs   (npm run test:ecalc)
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';

const root = path.resolve(import.meta.dirname, '..');
const read = file => fs.readFileSync(path.join(root, file), 'utf8');
// Data made inside the vm has the prototypes of another realm: assert.deepEqual needs plain copies.
const plain = value => JSON.parse(JSON.stringify(value));

const window = {};
const sandbox = vm.createContext({ window });
vm.runInContext(read('ui/js/desktop/apps/ecalc/ecalc-pinout-draw.js'), sandbox);
const Draw = window.ECalcPinoutDraw;
assert.ok(Draw && typeof Draw.layout === 'function' && typeof Draw.svg === 'function' && Array.isArray(Draw.OUTLINES), 'the file publishes window.ECalcPinoutDraw');
assert.equal(Draw.OUTLINES.length, 16, 'sixteen outlines');

const snapshot = JSON.parse(read('internal/ecalc/testdata/catalog.en.json'));
const fixtures = plain(vm.runInContext('(' + read('scripts/ecalc-pinout-fixtures.js') + ')', sandbox)());
assert.ok(fixtures.length >= 30, 'the fixture has its items');
const types = Object.fromEntries(snapshot.pin_types.map(type => [type.id, { hex: type.hex, ink: type.ink }]));
const wires = Object.fromEntries(snapshot.wires.map(wire => [wire.id, { hex: wire.hex, stripe: wire.stripe }]));
assert.deepEqual(Object.keys(types), ['power', 'ground', 'signal', 'data', 'io', 'nc'], 'the pin types of the snapshot');

const real = snapshot.items.filter(item => item.kind === 'pinout');
const items = fixtures.concat(real);

// Every outline of rows is drawn by some fixture, and so is every shape type.
const outlines = new Set(fixtures.filter(item => item.shape.type === 'rows').map(item => item.shape.outline));
assert.deepEqual([...outlines].sort(), [...Draw.OUTLINES].sort(), 'the fixtures draw every outline');
assert.deepEqual([...new Set(fixtures.map(item => item.shape.type))].sort(), ['board', 'circular', 'dip', 'jack', 'rows', 'to']);

const EPS = 0.51; // the layout rounds contacts to 0.01 and the width up to a whole unit
const shapeNumbers = shape => {
    const list = [];
    if (shape.type === 'rows') shape.rows.forEach(row => row.pins.forEach(n => { if (n !== null) list.push(n); }));
    else if (shape.type === 'dip') for (let i = 1; i <= shape.pins; i++) list.push(String(i));
    else if (shape.type === 'to') list.push(...shape.pins);
    else if (shape.type === 'circular') shape.positions.forEach(p => list.push(p.pin));
    else if (shape.type === 'jack') list.push(...shape.segments);
    else if (shape.type === 'board') {
        for (const side of ['left', 'right', 'top', 'bottom']) shape[side].forEach(n => { if (n !== null) list.push(n); });
        shape.inner.forEach(group => group.rows.forEach(row => row.pins.forEach(n => { if (n !== null) list.push(n); })));
    }
    return list;
};

let checked = 0;
for (const item of items) {
    const variantPins = item.variants.length ? item.variants.map(variant => variant.pins) : [item.pins];
    for (const pins of variantPins) {
        const wanted = shapeNumbers(item.shape);
        assert.deepEqual([...pins.map(pin => pin.number)].sort(), [...wanted].sort(), `${item.id}: the pins and the shape name the same numbers`);
        const two = item.shape.type === 'rows' || item.shape.type === 'circular';
        const views = two ? ['plug', 'socket'] : [item.shape.drawn];
        const layouts = views.map(view => plain(Draw.layout(item.shape, pins, { view })));
        layouts.forEach((view, index) => {
            const label = `${item.id} (${views[index]})`;
            assert.ok(view.width > 0 && view.height > 0 && Number.isInteger(view.width) && Number.isInteger(view.height), `${label}: a whole-unit size`);
            assert.deepEqual(view.contacts.map(c => c.number).sort(), [...wanted].sort(), `${label}: every pin drawn exactly once`);
            const seen = new Set();
            for (const c of view.contacts) {
                const key = c.x + ',' + c.y;
                assert.ok(!seen.has(key), `${label}: pin ${c.number} shares the position ${key}`);
                seen.add(key);
                assert.ok(c.x - c.w / 2 >= -EPS && c.x + c.w / 2 <= view.width + EPS && c.y - c.h / 2 >= -EPS && c.y + c.h / 2 <= view.height + EPS, `${label}: pin ${c.number} lies inside the viewBox`);
                assert.ok(c.num.x >= 0 && c.num.x <= view.width && c.num.y >= 0 && c.num.y <= view.height, `${label}: the number of pin ${c.number} lies inside the viewBox`);
                if (c.num.mode === 'in' && c.shape !== 'path') {
                    const fs = Draw.fitNumber(c.number, c.kind, c.w, c.h);
                    assert.ok(fs === c.num.fs || c.num.fs === 8 || c.num.fs === 9 || c.num.fs === 10, `${label}: the font size of the number of pin ${c.number}`);
                }
                if (c.label) assert.ok(c.label.x >= 0 && c.label.x <= view.width, `${label}: the name of pin ${c.number} starts inside the viewBox`);
            }
            const [x0, y0, x1, y1] = view.shellBox;
            assert.ok(x0 >= -EPS && y0 >= -EPS && x1 <= view.width + EPS && y1 <= view.height + EPS && x1 > x0 && y1 > y0, `${label}: the shell lies inside the viewBox`);
            assert.equal(view.mirrored, two && views[index] !== item.shape.drawn, `${label}: mirrored flag`);
            // The markup.
            const markup = Draw.svg({ layout: view, types, wires, label: item.name });
            assert.equal((markup.match(/<g class="ec-pin"/g) || []).length, view.contacts.length, `${label}: one g.ec-pin per contact`);
            assert.ok(markup.includes(`viewBox="0 0 ${view.width} ${view.height}"`), `${label}: viewBox`);
            assert.ok(!/<text[^>]*scale\(/.test(markup) && !/<g[^>]*scale\(-1[^>]*>\s*<text/.test(markup), `${label}: no text is scaled`);
            const shellGroup = markup.match(/<g class="ec-pin-shell"([^>]*)>/)[1];
            assert.equal(shellGroup.includes('scale(-1 1)'), view.mirrored, `${label}: only a mirrored view flips the shell group`);
            const chunks = new Map(markup.split('<g class="ec-pin" ').slice(1).map(chunk => [chunk.match(/data-pin="([^"]*)"/)[1], chunk]));
            for (const c of view.contacts) {
                const style = types[c.type] || types.nc, chunk = chunks.get(c.number.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;'));
                assert.ok(chunk, `${label}: data-pin of ${c.number}`);
                assert.ok(new RegExp(`class="ec-pin-body"[^>]*fill="${style.hex}"`).test(chunk), `${label}: the fill of pin ${c.number} is the hex of ${c.type}`);
                if (c.num.mode === 'in') assert.ok(chunk.includes(`fill="${style.ink}">`), `${label}: the number of pin ${c.number} is written in the ink of ${c.type}`);
                else assert.ok(!/class="ec-pin-num[^"]*"[^>]*fill=/.test(chunk), `${label}: a number outside its contact has no ink of its own`);
            }
            const first = view.contacts.filter(c => c.mark);
            assert.equal(first.length, 1, `${label}: exactly one marked pin`);
            assert.equal(first[0].number, view.contacts.some(c => c.number === '1') ? '1' : view.contacts[0].number, `${label}: the mark is on pin 1, or on the first pin`);
            checked++;
        });
        if (two) {
            const [a, b] = layouts;
            assert.equal(a.width, b.width);
            assert.equal(a.height, b.height);
            const byNumber = new Map(b.contacts.map(c => [c.number, c]));
            for (const c of a.contacts) {
                const m = byNumber.get(c.number);
                assert.ok(Math.abs(c.x + m.x - a.width) < 0.011, `${item.id}: pin ${c.number} is mirrored about the axis (${c.x} / ${m.x} in ${a.width})`);
                assert.equal(c.y, m.y, `${item.id}: pin ${c.number} keeps its height`);
                assert.ok(Math.abs(c.num.x + m.num.x - a.width) < 0.011, `${item.id}: the number of pin ${c.number} is mirrored`);
                assert.equal(c.num.mode, m.num.mode, `${item.id}: the number stays inside or outside`);
                if (c.label) {
                    assert.ok(m.label && m.label.anchor === (c.label.anchor === 'start' ? 'end' : c.label.anchor === 'end' ? 'start' : 'middle'), `${item.id}: the name of pin ${c.number} changes its side`);
                    assert.ok(Math.abs(c.label.x + m.label.x - a.width) < 0.011, `${item.id}: the name of pin ${c.number} is mirrored`);
                }
            }
        }
    }
}
assert.ok(checked >= 60, `checked ${checked} layouts`);

// Escaping: nothing of the data reaches the markup unescaped, and a colour that is no hex falls back.
{
    const item = { shape: { type: 'jack', drawn: 'side', segments: ['"<&', 'R', 'S'], rows: [], pins: null, positions: [], left: [], right: [], top: [], bottom: [], inner: [] }, pins: [
        { number: '"<&', name: '<b>x</b>', type: 'power', wire: '' }, { number: 'R', name: 'R', type: 'bogus', wire: '' }, { number: 'S', name: 'S', type: 'ground', wire: '' }] };
    const markup = Draw.svg({ layout: Draw.layout(item.shape, item.pins, {}), types: { power: { hex: 'red;" onload="x', ink: '#ffffff' }, ground: types.ground, nc: types.nc }, wires: {}, label: '"><script>' });
    assert.ok(!markup.includes('<script>') && !markup.includes('<b>') && !markup.includes('onload'), 'the markup is escaped and the colours are checked');
    assert.ok(markup.includes('data-pin="&quot;&lt;&amp;"'), 'data-pin is escaped');
    assert.ok(markup.includes(`fill="${types.nc.hex}"`), 'an unknown type and a bad colour fall back to the nc colour');
}
// Symbols as pin numbers (a servo header: S + -) are safe data-pin values and fit their contact.
{
    const shape = { type: 'rows', outline: 'jst', contact: 'blade', drawn: 'plug', key: 'none', rows: [{ pins: ['S'], offset: 0 }, { pins: ['+'], offset: 0 }, { pins: ['-'], offset: 0 }], pins: null, positions: [], segments: [], left: [], right: [], top: [], bottom: [], inner: [] };
    const pins = ['S', '+', '-'].map(number => ({ number, name: 'N' + number, type: 'signal', wire: '' }));
    const layout = plain(Draw.layout(shape, pins, { view: 'plug' }));
    assert.deepEqual(layout.contacts.map(c => c.number), ['S', '+', '-']);
    assert.ok(layout.contacts.every(c => c.label && c.name), 'a single column carries its names');
}
// The rule for the numbers: they fit inside or move out.
assert.ok(Draw.fitNumber('12', 'round', 20, 20) > 0, 'two digits fit a round contact of 20');
assert.equal(Draw.fitNumber('A12', 'blade', 13, 16), 0, 'a three-character number does not fit a blade');
assert.equal(Draw.fitNumber('10', 'blade', 12, 16), 0, 'two digits do not fit a thin blade');
console.log(`ecalc pinouts: ${items.length} items (${fixtures.length} fixtures, ${real.length} of the catalog), ${checked} layouts checked`);
