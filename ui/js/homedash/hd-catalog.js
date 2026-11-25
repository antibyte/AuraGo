// HomeDash widget catalog access (F1). create(json) wraps the server catalog (GET widget-catalog, golden file
// internal/homedash/testdata/widget_catalog.json): default size and size constraints per type and variant, option defaults merged under a
// widget's own options, and registryMismatches(E.widgets): every registered type exists, its variants equal the catalog's, and it implements
// every required binding. The client never copies catalog numbers.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core) throw new Error('hd-catalog.js: load hd-core.js first');
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
        return Object.freeze({ hash: String(cat.hash || ''), has: type => byType.has(type), widget: type => byType.get(type) || null, variant, defaultSize, constraints, optionDefaults, options, registryMismatches });
    }
    E.catalog = Object.freeze({ create });
})();
