// Minimal DOM double for HomeDash Node tests: createElement(NS), attributes, dataset, style.setProperty, textContent,
// append/remove, simple selectors (tag, .class, [attr], [attr="v"], descendant), listeners with signal.
const kebab = s => s.replace(/[A-Z]/g, c => '-' + c.toLowerCase());
class Text { constructor(doc, t) { this.nodeType = 3; this.ownerDocument = doc; this._t = String(t); this.parentNode = null; }
  get textContent() { return this._t; } set textContent(v) { this._t = String(v); } remove() { if (this.parentNode) this.parentNode._drop(this); } }
class Style { constructor() { this._p = new Map(); } setProperty(n, v) { this._p.set(n, String(v)); }
  getPropertyValue(n) { return this._p.has(n) ? this._p.get(n) : ''; } removeProperty(n) { const v = this.getPropertyValue(n); this._p.delete(n); return v; } }
// Strict selector engine: supports tag, *, #id, .class, [attr], [attr=v], [attr="v"], [attr='v'], descendant, ">" child, comma lists.
// Anything else throws, so a test can never pass on a node matched by syntax this double does not implement.
function unsupported(selector) { return new Error('fake DOM: unsupported selector ' + selector); }
// Split on a top-level character set (outside [...] and quotes). Returns pieces; for combinator splitting keeps separators.
function splitTop(text, isSep, keepSep) {
  const out = []; let cur = ''; let depth = 0; let quote = '';
  for (const ch of text) {
    if (quote) { cur += ch; if (ch === quote) quote = ''; continue; }
    if (ch === '"' || ch === "'") { quote = ch; cur += ch; continue; }
    if (ch === '[') depth++; else if (ch === ']') depth = Math.max(0, depth - 1);
    if (depth === 0 && isSep(ch)) { out.push(cur); if (keepSep) out.push(ch); cur = ''; continue; }
    cur += ch;
  }
  out.push(cur); return out;
}
function parseCompound(text, selector) {
  const c = { tag: null, id: null, classes: [], attrs: [] }; let s = text; let m;
  if (s === '*') return c;
  if ((m = /^[a-zA-Z][\w-]*/.exec(s))) { c.tag = m[0].toLowerCase(); s = s.slice(m[0].length); }
  while (s.length) {
    if ((m = /^#([\w-]+)/.exec(s))) c.id = m[1];
    else if ((m = /^\.([\w-]+)/.exec(s))) c.classes.push(m[1]);
    else if ((m = /^\[([\w-]+)(?:=(?:"([^"]*)"|'([^']*)'|([^\]"'\s]+)))?\]/.exec(s))) c.attrs.push({ name: m[1], value: m[2] ?? m[3] ?? m[4] });
    else throw unsupported(selector);
    s = s.slice(m[0].length);
  }
  if (!c.tag && !c.id && !c.classes.length && !c.attrs.length) throw unsupported(selector);
  return c;
}
// A complex selector becomes [compound, comb, compound, ...] where comb is ' ' or '>'.
function parseComplex(text, selector) {
  const pieces = splitTop(text.trim(), ch => ch === '>' || /\s/.test(ch), true);
  const seq = []; let comb = null; let expectCompound = true;
  for (const p of pieces) {
    if (p === '>') { if (expectCompound) throw unsupported(selector); comb = '>'; expectCompound = true; continue; }
    if (p === '') continue;
    if (/^\s$/.test(p)) { if (!expectCompound && comb === null) comb = ' '; continue; }
    seq.push(...(seq.length ? [comb] : []), parseCompound(p, selector)); comb = null; expectCompound = false;
  }
  if (!seq.length || expectCompound || comb === '>') throw unsupported(selector);
  return seq;
}
function parseSelector(selector) {
  if (typeof selector !== 'string') throw unsupported(selector);
  return splitTop(selector, ch => ch === ',').map(part => parseComplex(part, selector));
}
function matchCompound(el, c) {
  if (c.tag && el.localName.toLowerCase() !== c.tag) return false;
  if (c.id !== null && el.getAttribute('id') !== c.id) return false;
  for (const k of c.classes) if (!el.classList.contains(k)) return false;
  for (const a of c.attrs) { if (!el.hasAttribute(a.name)) return false; if (a.value !== undefined && el.getAttribute(a.name) !== a.value) return false; }
  return true;
}
function matchSeq(el, seq, i) {
  if (!matchCompound(el, seq[i])) return false;
  if (i === 0) return true;
  const comb = seq[i - 1]; const parent = el.parentNode;
  if (!parent || parent.nodeType !== 1) return false;
  if (comb === '>') return matchSeq(parent, seq, i - 2);
  for (let n = parent; n && n.nodeType === 1; n = n.parentNode) if (matchSeq(n, seq, i - 2)) return true;
  return false;
}
function matchesParsed(el, list) { return list.some(seq => matchSeq(el, seq, seq.length - 1)); }
export class FakeElement {
  constructor(doc, tag, ns) {
    this.nodeType = 1; this.ownerDocument = doc; this.namespaceURI = ns || 'http://www.w3.org/1999/xhtml';
    this.localName = tag; this.tagName = ns && ns.endsWith('/svg') ? tag : tag.toUpperCase();
    this._a = new Map(); this.childNodes = []; this.parentNode = null; this._l = new Map(); this.style = new Style();
    const self = this;
    this.classList = { contains: c => self._cls().includes(c), add: (...c) => self.setAttribute('class', [...new Set([...self._cls(), ...c])].join(' ')),
      remove: (...c) => self.setAttribute('class', self._cls().filter(x => !c.includes(x)).join(' ')),
      toggle: (c, f) => { const on = f === undefined ? !self.classList.contains(c) : !!f; on ? self.classList.add(c) : self.classList.remove(c); return on; } };
    this.dataset = new Proxy({}, { get: (_, k) => self.getAttribute('data-' + kebab(String(k))) ?? undefined,
      set: (_, k, v) => { self.setAttribute('data-' + kebab(String(k)), v); return true; }, deleteProperty: (_, k) => { self.removeAttribute('data-' + kebab(String(k))); return true; } });
  }
  _cls() { return (this.getAttribute('class') || '').split(/\s+/).filter(Boolean); }
  get children() { return this.childNodes.filter(n => n.nodeType === 1); }
  get firstElementChild() { return this.children[0] || null; }
  get parentElement() { return this.parentNode && this.parentNode.nodeType === 1 ? this.parentNode : null; }
  get id() { return this.getAttribute('id') || ''; } set id(v) { this.setAttribute('id', v); }
  setAttribute(n, v) { this._a.set(String(n), String(v)); } getAttribute(n) { return this._a.has(n) ? this._a.get(n) : null; }
  hasAttribute(n) { return this._a.has(n); } removeAttribute(n) { this._a.delete(n); } getAttributeNames() { return [...this._a.keys()]; }
  toggleAttribute(n, f) { const on = f === undefined ? !this.hasAttribute(n) : !!f; on ? this.setAttribute(n, '') : this.removeAttribute(n); return on; }
  get textContent() { return this.childNodes.map(n => n.textContent).join(''); }
  set textContent(v) { this.replaceChildren(); const s = String(v ?? ''); if (s) this.appendChild(new Text(this.ownerDocument, s)); }
  appendChild(n) { if (n.parentNode) n.parentNode._drop(n); n.parentNode = this; this.childNodes.push(n); return n; }
  append(...ns) { for (const n of ns) this.appendChild(typeof n === 'string' ? new Text(this.ownerDocument, n) : n); }
  insertBefore(n, ref) { if (!ref) return this.appendChild(n); if (!this.childNodes.includes(ref)) throw new Error('fake DOM: insertBefore reference is not a child'); if (n.parentNode) n.parentNode._drop(n); n.parentNode = this; this.childNodes.splice(this.childNodes.indexOf(ref), 0, n); return n; }
  replaceChildren(...ns) { for (const c of this.childNodes) c.parentNode = null; this.childNodes = []; this.append(...ns); }
  remove() { if (this.parentNode) this.parentNode._drop(this); }
  _drop(n) { const i = this.childNodes.indexOf(n); if (i >= 0) this.childNodes.splice(i, 1); n.parentNode = null; }
  matches(sel) { return matchesParsed(this, parseSelector(sel)); }
  contains(n) { for (let x = n; x; x = x.parentNode) if (x === this) return true; return false; }
  closest(sel) { for (let n = this; n && n.nodeType === 1; n = n.parentNode) if (n.matches(sel)) return n; return null; }
  querySelectorAll(sel) { const list = parseSelector(sel); const out = []; const walk = n => { for (const c of n.children) { if (matchesParsed(c, list)) out.push(c); walk(c); } }; walk(this); return out; }
  querySelector(sel) { return this.querySelectorAll(sel)[0] || null; }
  addEventListener(t, fn, o) { if (o && o.signal) { if (o.signal.aborted) return; o.signal.addEventListener('abort', () => this.removeEventListener(t, fn)); }
    if (!this._l.has(t)) this._l.set(t, new Set()); this._l.get(t).add(fn); if (o && o.capture) (this._cap || (this._cap = new Set())).add(fn); }
  removeEventListener(t, fn) { const s = this._l.get(t); if (s) s.delete(fn); }
  // Capture listeners run before the others (as at the target in a browser); stopImmediatePropagation ends the loop. No propagation between nodes.
  dispatchEvent(ev) { if (!ev.target) ev.target = this; if (!ev.currentTarget) ev.currentTarget = this;
    let halted = false; if (typeof ev.stopImmediatePropagation !== 'function') ev.stopImmediatePropagation = () => { halted = true; };
    const all = [...(this._l.get(ev.type) || [])], cap = this._cap || new Set();
    for (const fn of [...all.filter(f => cap.has(f)), ...all.filter(f => !cap.has(f))]) { if (halted) break; fn.call(this, ev); } return true; }
  getBoundingClientRect() { return { x: 0, y: 0, width: 0, height: 0, top: 0, left: 0, right: 0, bottom: 0 }; }
  focus() { this.ownerDocument.activeElement = this; }
}
export function createDocument() {
  const doc = { activeElement: null, _l: new Map() };
  // The document takes listeners and events like an element (popover outside-click tests); no propagation between nodes either.
  for (const m of ['addEventListener', 'removeEventListener', 'dispatchEvent']) doc[m] = FakeElement.prototype[m];
  doc.createElement = tag => new FakeElement(doc, tag); doc.createElementNS = (ns, tag) => new FakeElement(doc, tag, ns);
  doc.createTextNode = t => new Text(doc, t); doc.body = doc.createElement('body'); doc.documentElement = doc.createElement('html');
  return doc;
}
export function allElements(el) { return [el, ...el.querySelectorAll('*')]; }
