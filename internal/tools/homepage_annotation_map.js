() => {
  // Homepage Studio annotation map: visible elements of the rendered page with
  // a unique selector, a short text, a few attributes and the page-coordinate
  // box. Runs inside the captured page; the result is JSON-serialized by rod.
  const MAX_ELEMENTS = 2000;
  const SKIP = new Set(['SCRIPT', 'STYLE', 'NOSCRIPT', 'TEMPLATE', 'HEAD', 'META', 'LINK', 'BR', 'WBR']);
  const SIGNIFICANT = new Set(['A', 'BUTTON', 'INPUT', 'SELECT', 'TEXTAREA', 'IMG', 'SVG', 'VIDEO', 'H1', 'H2', 'H3', 'H4', 'H5', 'H6', 'SECTION', 'HEADER', 'FOOTER', 'NAV', 'MAIN', 'ARTICLE', 'ASIDE', 'FORM', 'FIGURE']);
  const ATTRS = ['href', 'src', 'alt', 'aria-label', 'name', 'type', 'placeholder'];
  const doc = document.documentElement;
  const body = document.body || doc;
  const width = Math.max(doc.scrollWidth, body.scrollWidth || 0);
  const height = Math.max(doc.scrollHeight, body.scrollHeight || 0);
  const sx = window.scrollX;
  const sy = window.scrollY;

  const escapeIdent = value => (window.CSS && CSS.escape) ? CSS.escape(value) : String(value).replace(/[^a-zA-Z0-9_-]/g, ch => '\\' + ch);
  const unique = selector => {
    try { return document.querySelectorAll(selector).length === 1; } catch (_) { return false; }
  };
  const selectorFor = el => {
    if (el.id && unique('#' + escapeIdent(el.id))) return '#' + escapeIdent(el.id);
    for (const attr of el.getAttributeNames()) {
      if (!attr.startsWith('data-')) continue;
      const value = el.getAttribute(attr);
      if (!value || value.length > 80) continue;
      const candidate = el.tagName.toLowerCase() + '[' + attr + '="' + value.replace(/"/g, '\\"') + '"]';
      if (unique(candidate)) return candidate;
    }
    const parts = [];
    let node = el;
    while (node && node.nodeType === 1 && node !== doc) {
      if (node !== el && node.id && unique('#' + escapeIdent(node.id))) {
        parts.unshift('#' + escapeIdent(node.id));
        break;
      }
      let part = node.tagName.toLowerCase();
      const classes = Array.from(node.classList).filter(c => /^[a-zA-Z_-][\w-]*$/.test(c)).slice(0, 2);
      if (classes.length) part += '.' + classes.join('.');
      const parent = node.parentElement;
      if (parent) {
        const same = Array.from(parent.children).filter(c => c.tagName === node.tagName);
        if (same.length > 1) part += ':nth-of-type(' + (same.indexOf(node) + 1) + ')';
      }
      parts.unshift(part);
      if (unique(parts.join(' > '))) return parts.join(' > ');
      node = parent;
    }
    return parts.join(' > ');
  };
  const textOf = el => {
    const text = (el.innerText || el.textContent || '').replace(/\s+/g, ' ').trim();
    return text.length > 120 ? text.slice(0, 119) + '…' : text;
  };
  const visible = (el, rect) => {
    if (rect.width < 4 || rect.height < 4) return false;
    const style = getComputedStyle(el);
    return style.display !== 'none' && style.visibility !== 'hidden' && Number(style.opacity) !== 0;
  };

  const out = [];
  const index = new Map();
  const walker = document.createTreeWalker(body, NodeFilter.SHOW_ELEMENT);
  for (let el = walker.currentNode; el && out.length < MAX_ELEMENTS; el = walker.nextNode()) {
    if (SKIP.has(el.tagName)) continue;
    const rect = el.getBoundingClientRect();
    if (!visible(el, rect)) continue;
    let parent = -1;
    for (let p = el.parentElement; p; p = p.parentElement) {
      if (index.has(p)) { parent = index.get(p); break; }
    }
    const attrs = {};
    for (const name of ATTRS) {
      const value = el.getAttribute(name);
      if (value) attrs[name] = value.slice(0, 200);
    }
    const item = {
      i: out.length,
      selector: selectorFor(el),
      tag: el.tagName.toLowerCase(),
      role: el.getAttribute('role') || '',
      text: textOf(el),
      attrs: attrs,
      box: [rect.left + sx, rect.top + sy, rect.width, rect.height].map(n => Math.round(n)),
      parent: parent,
      tab: SIGNIFICANT.has(el.tagName) || el.hasAttribute('role')
    };
    index.set(el, item.i);
    out.push(item);
  }
  return { width: width, height: height, elements: out };
}
