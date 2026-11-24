// Minimal DOM double for HomeDash Node tests: createElement(NS), attributes, dataset, style.setProperty, textContent,
// append/remove, simple selectors (tag, .class, [attr], [attr="v"], descendant), listeners with signal.
const kebab = s => s.replace(/[A-Z]/g, c => '-' + c.toLowerCase());
class Text { constructor(doc, t) { this.nodeType = 3; this.ownerDocument = doc; this._t = String(t); this.parentNode = null; }
  get textContent() { return this._t; } set textContent(v) { this._t = String(v); } remove() { if (this.parentNode) this.parentNode._drop(this); } }
class Style { constructor() { this._p = new Map(); } setProperty(n, v) { this._p.set(n, String(v)); }
  getPropertyValue(n) { return this._p.has(n) ? this._p.get(n) : ''; } removeProperty(n) { const v = this.getPropertyValue(n); this._p.delete(n); return v; } }
function matchCompound(el, c) {
  if (c === '*') return true;
  const tag = /^[a-zA-Z][\w-]*/.exec(c); if (tag && el.localName.toLowerCase() !== tag[0].toLowerCase()) return false;
  for (const m of c.matchAll(/\.([\w-]+)/g)) if (!el.classList.contains(m[1])) return false;
  for (const m of c.matchAll(/\[([\w-]+)(?:="([^"]*)")?\]/g)) { if (!el.hasAttribute(m[1])) return false; if (m[2] !== undefined && el.getAttribute(m[1]) !== m[2]) return false; }
  return true;
}
function matchComplex(el, sel) {
  const parts = sel.trim().split(/\s+/); if (!matchCompound(el, parts.pop())) return false;
  let node = el.parentNode;
  for (let i = parts.length - 1; i >= 0; i--) { while (node && node.nodeType === 1 && !matchCompound(node, parts[i])) node = node.parentNode; if (!node || node.nodeType !== 1) return false; node = node.parentNode; }
  return true;
}
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
  insertBefore(n, ref) { if (!ref) return this.appendChild(n); if (n.parentNode) n.parentNode._drop(n); n.parentNode = this; this.childNodes.splice(this.childNodes.indexOf(ref), 0, n); return n; }
  replaceChildren(...ns) { for (const c of this.childNodes) c.parentNode = null; this.childNodes = []; this.append(...ns); }
  remove() { if (this.parentNode) this.parentNode._drop(this); }
  _drop(n) { const i = this.childNodes.indexOf(n); if (i >= 0) this.childNodes.splice(i, 1); n.parentNode = null; }
  matches(sel) { return sel.split(',').some(s => matchComplex(this, s)); }
  closest(sel) { for (let n = this; n && n.nodeType === 1; n = n.parentNode) if (n.matches(sel)) return n; return null; }
  querySelectorAll(sel) { const out = []; const walk = n => { for (const c of n.children) { if (c.matches(sel)) out.push(c); walk(c); } }; walk(this); return out; }
  querySelector(sel) { return this.querySelectorAll(sel)[0] || null; }
  addEventListener(t, fn, o) { if (o && o.signal) { if (o.signal.aborted) return; o.signal.addEventListener('abort', () => this.removeEventListener(t, fn)); }
    if (!this._l.has(t)) this._l.set(t, new Set()); this._l.get(t).add(fn); }
  removeEventListener(t, fn) { const s = this._l.get(t); if (s) s.delete(fn); }
  dispatchEvent(ev) { for (const fn of [...(this._l.get(ev.type) || [])]) fn.call(this, ev); return true; }
  getBoundingClientRect() { return { x: 0, y: 0, width: 0, height: 0, top: 0, left: 0, right: 0, bottom: 0 }; }
  focus() { this.ownerDocument.activeElement = this; }
}
export function createDocument() {
  const doc = { activeElement: null };
  doc.createElement = tag => new FakeElement(doc, tag); doc.createElementNS = (ns, tag) => new FakeElement(doc, tag, ns);
  doc.createTextNode = t => new Text(doc, t); doc.body = doc.createElement('body'); doc.documentElement = doc.createElement('html');
  return doc;
}
export function allElements(el) { return [el, ...el.querySelectorAll('*')]; }
