// Runs ui/js/dashboard/dash-disclosures.js in bare Node vm contexts (stub document, localStorage and Chart) and checks
// the disclosure state of the big cards: parse/serialize (round trip, garbage), the capture-phase toggle listener (it
// stores open blocks, forgets closed ones, resizes the charts of an opened block and ignores other elements), restore
// (opens stored blocks, never closes one), storage failures (the state of the page view survives) and the count badges.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const file = path.join(__dirname, '..', 'js', 'dashboard', 'dash-disclosures.js');
const source = fs.readFileSync(file, 'utf8');
const json = value => JSON.stringify(value);

function block(id, canvases) {
    const attrs = id ? { 'data-disclosure': id } : {};
    return {
        tagName: 'DETAILS',
        open: false,
        getAttribute: name => (Object.prototype.hasOwnProperty.call(attrs, name) ? attrs[name] : null),
        querySelectorAll: sel => (sel === 'canvas' ? (canvases || []) : []),
    };
}

function memoryStorage(initial) {
    const data = Object.assign({}, initial);
    return { data, getItem: k => (Object.prototype.hasOwnProperty.call(data, k) ? data[k] : null), setItem: (k, v) => { data[k] = String(v); } };
}

function load({ storage, blocks = [], badges = {} } = {}) {
    const listeners = [];
    const resized = [];
    const ctx = {
        console,
        localStorage: storage,
        Chart: { getChart: canvas => (canvas.chart ? { resize: () => resized.push(canvas.chart) } : undefined) },
        document: {
            documentElement: { lang: 'en' },
            addEventListener: (type, fn, capture) => listeners.push({ type, fn, capture }),
            querySelectorAll: sel => (sel === 'details[data-disclosure]' ? blocks : []),
            querySelector: sel => badges[sel] || null,
        },
    };
    ctx.window = ctx;
    vm.createContext(ctx);
    vm.runInContext(source, ctx, { filename: file });
    return { D: ctx.DashDisclosures, listeners, resized };
}

// Surface, parse and serialize
{
    const { D, listeners } = load({ storage: memoryStorage() });
    assert.ok(D, 'dash-disclosures.js must define window.DashDisclosures');
    assert.equal(D.KEY, 'dash-disclosures');
    assert.equal(listeners.length, 0, 'nothing is wired at load time');
    assert.equal(json(D.parse('{"card-prompt:base":true,"card-memory:health":true}')), json({ 'card-prompt:base': true, 'card-memory:health': true }));
    for (const garbage of [null, undefined, '', 'not json', '42', 'null', '[]', '["card-prompt:base"]', '{"card-prompt:base":1}', '{"card-prompt:base":"true"}', '{"nope":true}', '{"__proto__":true}', '{"card-prompt:":true}', '{"Card-Prompt:Base":true}']) {
        assert.equal(json(D.parse(garbage)), '{}', 'garbage ' + String(garbage));
    }
    assert.equal(json(D.parse('{"card-prompt:base":true,"card-prompt:fit":false,"x":true}')), json({ 'card-prompt:base': true }), 'closed and invalid ids are dropped');
    assert.equal(D.serialize({ 'card-tooling-telemetry:parse': true, 'card-prompt:base': true, 'card-prompt:fit': false, bad: true }), '{"card-prompt:base":true,"card-tooling-telemetry:parse":true}', 'open valid ids only, sorted');
    assert.equal(D.serialize(null), '{}');
    assert.equal(D.serialize(['card-prompt:base']), '{}');
    assert.equal(json(D.parse(D.serialize({ 'card-memory:curation': true }))), json({ 'card-memory:curation': true }), 'round trip');
}

// The toggle listener
{
    const storage = memoryStorage();
    const base = block('card-prompt:base', [{ chart: 'size' }, {}]);
    const fit = block('card-prompt:fit');
    const { D, listeners, resized } = load({ storage, blocks: [base, fit] });
    D.init();
    assert.equal(listeners.length, 1);
    assert.equal(listeners[0].type, 'toggle');
    assert.equal(listeners[0].capture, true, 'toggle does not bubble: the document listener must capture');
    D.init();
    assert.equal(listeners.length, 1, 'init wires the listener once');
    const toggle = el => listeners[0].fn({ target: el });
    base.open = true; toggle(base);
    assert.equal(storage.data['dash-disclosures'], '{"card-prompt:base":true}');
    assert.equal(json(resized), '["size"]', 'opening resizes the charts inside the block');
    fit.open = true; toggle(fit);
    fit.open = false; toggle(fit);
    assert.equal(storage.data['dash-disclosures'], '{"card-prompt:base":true}', 'a closed block is forgotten');
    assert.equal(json(resized), '["size"]', 'closing resizes nothing');
    const foreign = block(null);
    foreign.open = true; toggle(foreign);
    toggle({ tagName: 'DIV', getAttribute: () => 'card-prompt:fit', open: true });
    assert.equal(storage.data['dash-disclosures'], '{"card-prompt:base":true}', 'other elements are ignored');
}

// restore opens stored blocks and never closes one
{
    const storage = memoryStorage({ 'dash-disclosures': '{"card-prompt:base":true,"card-memory:health":true}' });
    const base = block('card-prompt:base');
    const fit = block('card-prompt:fit');
    fit.open = true; // opened by the user; its toggle event may not have been handled yet
    const { D } = load({ storage, blocks: [base, fit] });
    D.restore();
    assert.equal(base.open, true, 'a stored block opens');
    assert.equal(fit.open, true, 'restore never closes a block');
    const scoped = block('card-memory:health');
    D.restore({ querySelectorAll: sel => (sel === 'details[data-disclosure]' ? [scoped] : []) });
    assert.equal(scoped.open, true, 'restore(root) works below root');
}

// Storage unavailable (private mode) and quota errors: the state of this page view is kept
{
    const broken = { getItem() { throw new Error('denied'); }, setItem() { throw new Error('denied'); } };
    const parse = block('card-tooling-telemetry:parse');
    const { D, listeners } = load({ storage: broken, blocks: [parse] });
    D.init();
    parse.open = true; listeners[0].fn({ target: parse });
    parse.open = false;
    D.restore();
    assert.equal(parse.open, true, 'without storage the page view keeps its own state');
}
{
    const storage = memoryStorage();
    storage.setItem = () => { throw new Error('quota'); };
    const health = block('card-memory:health');
    const { D, listeners } = load({ storage, blocks: [health] });
    D.init();
    health.open = true; listeners[0].fn({ target: health });
    health.open = false;
    D.restore();
    assert.equal(health.open, true, 'after a failed write the page view keeps its own state');
}

// Count badges
{
    const badge = { textContent: 'x', hidden: true, title: '', removeAttribute(name) { if (name === 'title') this.title = ''; } };
    const sel = 'details[data-disclosure="card-memory:curation"] > summary [data-disclosure-count]';
    const { D } = load({ storage: memoryStorage(), badges: { [sel]: badge } });
    D.setCount('card-memory:curation', 1234, 'Open conflicts');
    assert.equal(badge.hidden, false);
    assert.equal(badge.textContent, (1234).toLocaleString('en'));
    assert.equal(badge.title, 'Open conflicts');
    D.setCount('card-memory:curation', 0);
    assert.equal(badge.hidden, true);
    assert.equal(badge.textContent, '');
    assert.equal(badge.title, '', 'a hidden badge has no title');
    for (const bad of [null, undefined, NaN, -3, 'x']) {
        D.setCount('card-memory:curation', bad);
        assert.equal(badge.hidden, true, 'hidden for ' + String(bad));
    }
    D.setCount('bad"] id', 5);
    D.setCount('card-memory:none', 5);
}

console.log('DashDisclosures: parse/serialize, toggle listener, restore, storage failures and count badges pass.');
