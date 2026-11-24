import assert from 'node:assert/strict'; import { loadEngine } from './harness.mjs';
const boot = () => loadEngine(['hd-core.js'], { dom: true });
export const tests = [
  ['loads without a DOM and publishes E.core', () => { const { E } = loadEngine(['hd-core.js']);
    for (const n of ['el','svg','setVars','setAttr','emitter','frame','bag','clamp','uid','randomId','clone','motionReduced','makeT','isTyping']) assert.equal(typeof E.core[n], 'function', n);
    assert.equal(E.el, E.core.el); }],
  ['el/svg build nodes; text stays text; style attributes refused', () => { const { E } = boot();
    const b = E.el('button', { class: 'hd-x', type: 'button', 'aria-checked': true, hidden: false, text: '<b>hi</b>' });
    assert.equal(b.tagName, 'BUTTON'); assert.equal(b.getAttribute('aria-checked'), 'true'); assert.equal(b.hasAttribute('hidden'), false);
    assert.equal(b.textContent, '<b>hi</b>'); assert.equal(b.children.length, 0);
    const s = E.svg('svg', { viewBox: '0 0 24 24' }, [E.svg('path', { d: 'M0 0' }), null, 'x']);
    assert.equal(s.namespaceURI, 'http://www.w3.org/2000/svg'); assert.equal(s.children.length, 1); assert.equal(s.textContent, 'x');
    assert.throws(() => E.el('div', { style: 'color: red' }), /style/);
    const v = E.el('div', { vars: { '--hd-value': 0.5 } }); assert.equal(v.style.getPropertyValue('--hd-value'), '0.5'); assert.equal(v.getAttribute('style'), null);
    assert.throws(() => E.core.setVars(v, { width: '3px' }), /custom properties/);
    E.core.setVars(v, { '--hd-value': null }); assert.equal(v.style.getPropertyValue('--hd-value'), '');
    E.core.setAttr(v, 'data-x', 1); E.core.setAttr(v, 'data-x', false); assert.equal(v.hasAttribute('data-x'), false); }],
  ['makeT fills placeholders; a missing key returns the key and warns once', () => { const { E } = boot(); const w = [];
    const t = E.core.makeT({ 'a.b': 'Hi {{n}} {{n}}' }, { warn: m => w.push(m) });
    assert.equal(t('a.b', { n: 'Ana' }), 'Hi Ana Ana'); assert.equal(t('a.c'), 'a.c'); t('a.c'); assert.equal(w.length, 1); }],
  ['clamp, uid, randomId, clone', () => { const c = boot().E.core;
    assert.equal(c.clamp(5, 0, 3), 3); assert.notEqual(c.uid('x'), c.uid('x')); assert.match(c.randomId('d'), /^d_[a-z0-9]{12}$/);
    const src = { a: [1, { b: 2 }] }, cp = c.clone(src); cp.a[1].b = 3; assert.equal(src.a[1].b, 2); assert.equal(c.clone(undefined), undefined); }],
  ['emitter, frame coalescing, bag', () => { const { E, document } = boot(); const got = [];
    const em = E.core.emitter(); const off = em.on('x', v => got.push(v)); em.emit('x', 1); off(); em.emit('x', 2); assert.deepEqual(got, [1]);
    const q = []; let calls = 0; const f = E.core.frame(() => calls++, { raf: fn => { q.push(fn); return q.length; }, caf: () => {} });
    f(); f(); f(); assert.equal(q.length, 1); q.shift()(); assert.equal(calls, 1); f(); assert.equal(f.pending(), true); f.cancel(); assert.equal(f.pending(), false);
    const b = E.core.bag(), el = document.createElement('div'); let hits = 0, d = 0; b.on(el, 'ping', () => hits++); b.add(() => d++);
    el.dispatchEvent({ type: 'ping' }); b.dispose(); b.dispose(); el.dispatchEvent({ type: 'ping' }); assert.equal(hits, 1); assert.equal(d, 1); assert.equal(b.signal.aborted, true); }],
  ['isTyping and motionReduced', () => { const { E, document } = boot(); const i = document.createElement('input');
    assert.equal(E.core.isTyping(i), true); i.setAttribute('type', 'checkbox'); assert.equal(E.core.isTyping(i), false);
    assert.equal(E.core.isTyping(document.createElement('textarea')), true); assert.equal(E.core.isTyping(null), false);
    const r = document.createElement('div'); r.setAttribute('data-hd-motion', 'none'); assert.equal(E.core.motionReduced(r), true);
    r.setAttribute('data-hd-motion', 'full'); assert.equal(E.core.motionReduced(r), false); }],
  ['el turns numbers into text nodes', () => { const { E } = boot();
    const n = E.el('span', null, 42); assert.equal(n.textContent, '42'); assert.equal(n.childNodes.length, 1); assert.equal(n.childNodes[0].nodeType, 3);
    assert.equal(E.el('span', null, [1, 'a']).textContent, '1a'); }],
  ['the fake DOM rejects unsupported selectors', () => { const { document } = boot();
    const mk = (tag, a) => { const n = document.createElement(tag); for (const k in a || {}) n.setAttribute(k, a[k]); return n; };
    const root = mk('div'), p = mk('div', { id: 'x' }), s1 = mk('span', { 'data-a': 'b' }), inner = mk('div'), s2 = mk('span');
    inner.appendChild(s2); p.appendChild(s1); p.appendChild(inner); root.appendChild(p);
    assert.deepEqual(root.querySelectorAll('#x > span'), [s1]); assert.deepEqual(root.querySelectorAll('#x>span'), [s1]); assert.deepEqual(root.querySelectorAll('#x span'), [s1, s2]);
    assert.equal(root.querySelector('#x'), p); assert.equal(root.querySelector('[data-a=b]'), s1); assert.equal(root.querySelector("[data-a='b']"), s1);
    assert.equal(root.querySelector('[data-a="b"], #nope'), s1); assert.equal(root.querySelector('[data-a="b,c"]'), null);
    for (const bad of ['span:is(.a)', 'a + b', 'a ~ b', '[data-a^="b"]', '[data-a$=b]', '[data-a*=b]', '[data-a~=b]', ':not(.a)']) assert.throws(() => root.querySelector(bad), /unsupported selector/, bad);
    assert.throws(() => p.insertBefore(mk('i'), s2), /not a child/); p.insertBefore(mk('i'), inner); assert.equal(p.childNodes[1].localName, 'i');
    const ev = { type: 'x' }; p.dispatchEvent(ev); assert.equal(ev.target, p); assert.equal(ev.currentTarget, p); }]
];
