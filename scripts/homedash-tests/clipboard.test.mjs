import assert from 'node:assert/strict'; import { loadEngine, APP_ORDER, read, eq, fc } from './harness.mjs';
const FILES = APP_ORDER.slice(0, APP_ORDER.indexOf('hd-clipboard.js') + 1), CATALOG = JSON.parse(read('internal/homedash/testdata/widget_catalog.json'));
const boot = () => { const { E, window } = loadEngine(FILES), cat = E.catalog.create(CATALOG); return { E, window, cat }; };
const docOf = (E, cat) => E.doc.normalise({ name: 'T', pages: [{ id: 'main', name: 'Main', widgets: [
  { id: 'a', type: 'sensor', variant: 'numeral', title: 'Temp', bindings: { primary: 'ha:sensor.temp' }, options: { decimals: 2 }, actions: { tap: { do: 'more_info' } } },
  { id: 'b', type: 'switch', variant: 'tile', bindings: { primary: 'ha:switch.x' }, actions: { tap: { do: 'toggle' } }, family: 'glass' }],
  layouts: { lg: [{ id: 'a', x: 4, y: 0, w: 3, h: 2 }, { id: 'b', x: 0, y: 0, w: 2, h: 1 }] } }] }, { sizeOf: cat.defaultSize, constraints: cat.constraints });
const wrap = (widget, lgSize) => JSON.stringify({ v: 1, widgets: [{ widget, lgSize: lgSize || { w: 2, h: 1 } }] });
const one = (E, cat, widget, pages, size) => E.clipboard.decode(wrap(widget, size), { catalog: cat, pages: pages || new Set(['main']) })[0];
export const tests = [
  ['clipboard: encode keeps lg reading order, strips ids and carries the lg size; decode round-trips', () => { const { E, cat } = boot(), D = docOf(E, cat);
    const text = E.clipboard.encode(D, 'main', ['a', 'b']), data = JSON.parse(text);
    eq(data.widgets.map(w => w.widget.type), ['switch', 'sensor']); assert.ok(data.widgets.every(w => !('id' in w.widget))); eq(data.widgets.map(w => w.lgSize), [{ w: 2, h: 1 }, { w: 3, h: 2 }]); assert.equal(data.v, 1);
    const back = E.clipboard.decode(text, { catalog: cat, pages: new Set(['main']) }); eq(back.map(e => e.widget.type), ['switch', 'sensor']); eq(back[1].size, { w: 3, h: 2 }); eq(back[1].widget.options, { decimals: 2 });
    assert.equal(back[0].widget.family, 'glass'); eq(back[0].widget.actions, { tap: { do: 'toggle' } }); assert.equal(E.clipboard.encode(D, 'nope', ['a']), ''); assert.equal(E.clipboard.encode(D, 'main', ['zz']), ''); }],
  ['clipboard: hostile text (size, version, shape, unknown type) yields nothing', () => { const { E, cat } = boot(), d = t => E.clipboard.decode(t, { catalog: cat, pages: new Set(['main']) });
    eq(d('x'.repeat(300 * 1024)), []); eq(d(JSON.stringify({ v: 1, widgets: [{ widget: { type: 'sensor', title: 'x'.repeat(262144) } }] })), []); eq(d('{"v":2,"widgets":[]}'), []); eq(d('[]'), []); eq(d('nope'), []); eq(d(''), []); eq(d(null), []);
    eq(d('{"v":1,"widgets":"x"}'), []); eq(d(wrap({ type: 'nope', variant: 'x' })), []); eq(d(wrap({ type: '__proto__' })), []); eq(d(wrap('str')), []); eq(d(JSON.stringify({ v: 1, widgets: [null, 5, 'x', { widget: null }] })), []);
    assert.equal(E.clipboard.decode(JSON.stringify({ v: 1, widgets: Array.from({ length: 300 }, () => ({ widget: { type: 'sensor' } })) }), { catalog: cat }).length, 120); eq(E.clipboard.decode(wrap({ type: 'sensor' })), []); }],
  ['clipboard: sanitize keeps known fields only, strips control characters, caps title at 80 runes', () => { const { E, cat } = boot();
    const long = 'a' + String.fromCharCode(7) + 'b'.repeat(200), w = one(E, cat, { type: 'sensor', variant: 'nope', title: long, evil: { x: 1 }, id: 'zz', icon: 'Bad Icon', bindings: { primary: 'ha:sensor.x', other: 'ha:sensor.y' }, family: 'neon', confirm: 'yes' }).widget;
    assert.equal(w.title.length, 80); assert.ok(!w.title.includes(String.fromCharCode(7))); assert.equal(w.variant, cat.variant('sensor', '').id); assert.ok(!('evil' in w) && !('id' in w) && !('icon' in w) && !('family' in w)); eq(w.bindings, { primary: 'ha:sensor.x' }); assert.equal(w.confirm, false);
    const emoji = fc(0x1f600).repeat(100), t2 = one(E, cat, { type: 'sensor', title: emoji }).widget.title; assert.equal(Array.from(t2).length, 80);
    assert.equal(one(E, cat, { type: 'sensor', title: 'a' + fc(0x85) + 'b' + fc(0x7f) + 'c' + fc(0x202e) + 'd' }).widget.title, 'abcd', 'C0, DEL, C1 and bidi controls are dropped like the server does');
    for (const url of ['see http://x.y', 'javascript:alert(1)', '//evil', 'DATA:text/html,x']) assert.equal(one(E, cat, { type: 'sensor', title: url }).widget.title, '', 'url_not_allowed: ' + url);
    assert.equal(one(E, cat, { type: 'sensor', icon: 'bulb-2' }).widget.icon, 'bulb-2'); assert.equal(one(E, cat, { type: 'sensor', confirm: true }).widget.confirm, true); }],
  ['clipboard: entity ids follow the server grammar (source 2-16 chars, native printable without space)', () => { const { E, cat } = boot(), b = id => one(E, cat, { type: 'sensor', bindings: { primary: id } }).widget.bindings.primary;
    assert.equal(b('ha:sensor.x'), 'ha:sensor.x'); assert.equal(b('system:cpu'), 'system:cpu');
    for (const bad of ['light.x', 'a b:c', 'HA:x', 'ha:', 'ha: x', ':x', 'ha:' + 'x'.repeat(201), 5, null, 'ha:' + fc(7)]) assert.equal(b(bad), undefined, String(bad));
    assert.equal(b('x:abc'), undefined, 'a one-character source is refused by the server (sourceIDRe needs 2-16)'); assert.equal(b('abcdefghijklmnopq:x'), undefined, 'a 17-character source is refused by the server');
    assert.equal(b('bt:aa:bb:cc'), 'bt:aa:bb:cc', 'the native part may hold colons (strings.Cut on the first one)'); assert.equal(b('javascript:x'), 'javascript:x', 'syntactically a valid id; an unknown source is only entity_unknown'); }],
  ['clipboard: options are typed, clamped and limited to the catalog; actions need real pages and toggle support', () => { const { E, cat } = boot(), P = new Set(['main', 'other']);
    const s = one(E, cat, { type: 'sensor', options: { decimals: 9, hours: '5', bogus: 1, accent: '#ABCDEF' } }, P).widget; eq(s.options, { decimals: 3, accent: '#abcdef' });
    assert.equal(one(E, cat, { type: 'sensor', options: { decimals: 1.6 } }, P).widget.options.decimals, 2); eq(one(E, cat, { type: 'sensor', options: { accent: 'red' } }, P).widget.options, {});
    eq(one(E, cat, { type: 'gauge', options: { warn: null, min: 5, max: Infinity } }, P).widget.options, { warn: null, min: 5 });
    const a = one(E, cat, { type: 'switch', actions: { tap: { do: 'navigate', page: 'other' }, hold: { do: 'toggle', extra: 1 } } }, P).widget.actions; eq(a, { tap: { do: 'navigate', page: 'other' }, hold: { do: 'toggle' } });
    eq(one(E, cat, { type: 'switch', actions: { tap: { do: 'navigate', page: 'gone' }, hold: { do: 'launch' } } }, P).widget.actions, {}); eq(one(E, cat, { type: 'sensor', actions: { tap: { do: 'toggle' }, hold: { do: 'none' } } }, P).widget.actions, { hold: { do: 'none' } });
    eq(E.clipboard.decode(wrap({ type: 'switch', actions: { tap: { do: 'navigate', page: 'other' } } }), { catalog: cat })[0].widget.actions, {}, 'without a pages set no navigate survives'); }],
  ['clipboard: string options use the same text rule as titles (control, bidi, URL) and their own length cap', () => { const { E, cat } = boot();
    const fake = Object.assign({}, cat, { optionSpecs: () => [{ key: 'label', type: 'string', max_len: 12 }] }), s = v => E.clipboard.decode(wrap({ type: 'sensor', options: { label: v } }), { catalog: fake, pages: new Set() })[0].widget.options.label;
    assert.equal(s('ok' + fc(0x85) + fc(0x202e) + 'go'), 'okgo'); assert.equal(s('a'.repeat(30)), 'a'.repeat(12)); for (const bad of ['http://x', 'javascript:1', '//evil']) assert.equal(s(bad), '', bad); assert.equal(E.clipboard.decode(wrap({ type: 'sensor', options: { label: 5 } }), { catalog: fake })[0].widget.options.label, undefined); }],
  ['clipboard: lgSize is clamped to the variant limits and missing size falls back to the default', () => { const { E, cat } = boot(), c = cat.constraints({ type: 'sensor', variant: 'sparkline' });
    const e = one(E, cat, { type: 'sensor', variant: 'sparkline' }, null, { w: 99, h: -3 }); eq(e.size, { w: c.maxW, h: c.minH }); assert.equal(e.widget.variant, 'sparkline');
    eq(one(E, cat, { type: 'sensor', variant: 'sparkline' }, null, { w: 'x' }).size, { w: cat.defaultSize({ type: 'sensor', variant: 'sparkline' }).w, h: cat.defaultSize({ type: 'sensor', variant: 'sparkline' }).h });
    eq(E.clipboard.decode(JSON.stringify({ v: 1, widgets: [{ widget: { type: 'sensor', variant: 'sparkline' } }] }), { catalog: cat })[0].size, cat.defaultSize({ type: 'sensor', variant: 'sparkline' })); }],
  ['clipboard: the channel tells its own realm at once and other tabs through storage events; a throwing store fails soft', () => { const { E, window } = boot(), mem = new Map(), seen = [], listeners = {};
    const store = { getItem: k => (mem.has(k) ? mem.get(k) : null), setItem: (k, v) => { mem.set(k, v); } }, win = { addEventListener: (t, fn) => { listeners[t] = fn; }, removeEventListener: t => { delete listeners[t]; } };
    const ch = E.clipboard.channel({ storage: store, win }), off = ch.onChange(v => seen.push(v)); assert.equal(ch.has(), false); assert.equal(ch.write('{"v":1}'), true); eq(seen, ['{"v":1}']); assert.equal(ch.has(), true); assert.equal(ch.read(), '{"v":1}');
    mem.set(E.clipboard.KEY, 'other'); listeners.storage({ key: E.clipboard.KEY }); listeners.storage({ key: 'else' }); eq(seen, ['{"v":1}', 'other']); listeners.storage({ key: null }); assert.equal(seen.length, 3);
    assert.equal(ch.write(''), false); assert.equal(ch.write('x'.repeat(E.clipboard.MAX_BYTES + 1)), false); assert.equal(ch.write(5), false); off(); ch.write('{"v":1}'); assert.equal(seen.length, 3);
    const bad = E.clipboard.channel({ storage: { getItem() { throw new Error('denied'); }, setItem() { throw new Error('quota'); } } }); assert.equal(bad.write('x'), false); assert.equal(bad.read(), null); assert.equal(E.clipboard.channel({}).write('x'), false);
    ch.onChange(() => { throw new Error('listener'); }); ch.dispose(); assert.equal(listeners.storage, undefined); void window; }]
];
