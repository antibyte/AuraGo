// HomeDash widget catalog access (F1). create(json) wraps the server catalog (GET widget-catalog, golden file
// internal/homedash/testdata/widget_catalog.json): default size and size constraints per type and variant, option defaults merged under a
// widget's own options, and registryMismatches(E.widgets): every registered type exists, its variants equal the catalog's, and it implements
// every required binding. The client never copies catalog numbers.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core) throw new Error('hd-catalog.js: load hd-core.js first');
    const CATEGORY_ORDER = Object.freeze(['control', 'climate', 'sensor', 'media', 'camera', 'status', 'layout']);
    const size = s => (s && Number.isFinite(s.w) && Number.isFinite(s.h) ? { w: s.w, h: s.h } : null);
    function create(raw) {
        const cat = raw && typeof raw === 'object' ? raw : {};
        const byType = new Map((Array.isArray(cat.widgets) ? cat.widgets : []).filter(w => w && typeof w.type === 'string').map(w => [w.type, w]));
        function variant(type, id) { const w = byType.get(type), list = w && Array.isArray(w.variants) ? w.variants : []; return list.find(v => v && v.id === id) || list[0] || null; }
        const defaultSize = widget => size((variant(widget.type, widget.variant) || {}).default_size) || { w: 2, h: 1 };
        function constraints(widget) {
            const w = byType.get(widget.type) || {}, v = variant(widget.type, widget.variant) || {}, min = size(v.min_size), max = size(v.max_size) || size(w.max_size);
            return { minW: min ? min.w : 1, minH: min ? min.h : 1, maxW: max ? max.w : 12, maxH: max ? max.h : 24 };
        }
        function optionDefaults(type) {
            const w = byType.get(type), out = {};
            for (const opt of w && Array.isArray(w.options) ? w.options : []) if (opt && typeof opt.key === 'string' && opt.default !== undefined && opt.default !== null) out[opt.key] = opt.default;
            return out;
        }
        const options = widget => Object.assign(optionDefaults(widget.type), widget.options && typeof widget.options === 'object' ? widget.options : {});
        function registryMismatches(registry) {
            const out = [];
            for (const type of registry.types()) {
                const def = registry.get(type), w = byType.get(type);
                if (!w) { out.push(type + ': not in the catalog'); continue; }
                const want = (w.variants || []).map(v => v.id).sort().join(','), have = def.variants.slice().sort().join(',');
                if (want !== have) out.push(type + ': variants ' + have + ' vs catalog ' + want);
                for (const b of w.bindings || []) if (b && b.required && !def.bindings.includes(b.name)) out.push(type + ': missing required binding ' + b.name);
            }
            return out;
        }
        const defaults = Object.freeze((Array.isArray(cat.defaults) ? cat.defaults : []).filter(d => d && typeof d.kind === 'string' && typeof d.type === 'string' && typeof d.variant === 'string'));
        const exact = (type, id) => { const w = byType.get(type); return w && Array.isArray(w.variants) ? w.variants.find(v => v && v.id === id) || null : null; };
        const capNames = (e, allowedOnly) => new Set((e && Array.isArray(e.capabilities) ? e.capabilities : []).filter(c => !allowedOnly || (c && typeof c === 'object' && c.allowed === true)).map(c => (c && typeof c === 'object' ? c.name : c)).filter(n => typeof n === 'string'));
        const usable = (registry, type, id) => !!exact(type, id) && (!registry || !!(registry.get(type) && registry.get(type).variants.includes(id)));
        // pickWidgetFor (frontend 5.3, M4 C10): the first catalog default with the entity's kind, its device_class when set and an allowed capability the entity has
        // when set, whose type and variant the registry implements; else entity/tile.
        function pickWidgetFor(entity, registry) {
            const e = entity || {}, kind = typeof e.kind === 'string' && e.kind ? e.kind : 'generic', dc = typeof e.device_class === 'string' ? e.device_class : '', caps = capNames(e, true);
            for (const d of defaults) { if (d.kind !== kind || (d.device_class && d.device_class !== dc) || (d.capability && !caps.has(d.capability))) continue; if (usable(registry, d.type, d.variant)) return { type: d.type, variant: d.variant }; }
            return usable(registry, 'entity', 'tile') ? { type: 'entity', variant: 'tile' } : null;
        }
        const binding = (type, name) => { const w = byType.get(type); return w && Array.isArray(w.bindings) ? w.bindings.find(b => b && b.name === name) || null : null; };
        function accepts(type, name, entity) {
            const b = binding(type, name), e = entity || {}; if (!b) return false;
            const kinds = Array.isArray(b.kinds) ? b.kinds : []; if (kinds.length && !kinds.includes(e.kind)) return false;
            const any = Array.isArray(b.any_capability) ? b.any_capability : []; if (!any.length) return true; const caps = capNames(e); return any.some(c => caps.has(c));
        }
        const optionSpecs = type => { const w = byType.get(type); return (w && Array.isArray(w.options) ? w.options : []).concat(Array.isArray(cat.common_options) ? cat.common_options : []).filter(s => s && typeof s.key === 'string'); };
        const canToggle = type => { const w = byType.get(type), list = w && w.commands && Array.isArray(w.commands.primary) ? w.commands.primary : []; return list.includes('toggle'); };
        function paletteGroups(registry) {
            const groups = new Map();
            for (const w of byType.values()) {
                const def = registry && registry.get(w.type); if (!def) continue;
                const variants = (w.variants || []).map(v => v && v.id).filter(v => def.variants.includes(v)); if (!variants.length) continue;
                const c = typeof w.category === 'string' && w.category ? w.category : 'status'; if (!groups.has(c)) groups.set(c, []);
                groups.get(c).push(Object.freeze({ type: w.type, variants: Object.freeze(variants) }));
            }
            const order = CATEGORY_ORDER.concat([...groups.keys()].filter(k => !CATEGORY_ORDER.includes(k)).sort());
            return Object.freeze(order.filter(k => groups.has(k)).map(k => Object.freeze({ category: k, types: Object.freeze(groups.get(k)) })));
        }
        return Object.freeze({ hash: String(cat.hash || ''), has: type => byType.has(type), widget: type => byType.get(type) || null, variant, defaultSize, constraints, optionDefaults, options, registryMismatches,
            defaults: () => defaults, pickWidgetFor, accepts, binding, optionSpecs, canToggle, families: () => (Array.isArray(cat.families) ? cat.families.filter(f => typeof f === 'string') : []), paletteTokens: () => (Array.isArray(cat.palette_tokens) ? cat.palette_tokens.filter(n => typeof n === 'string').slice(0, 40) : []), paletteGroups });
    }
    E.catalog = Object.freeze({ create, CATEGORY_ORDER });
})();
