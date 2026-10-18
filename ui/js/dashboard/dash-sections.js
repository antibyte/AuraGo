// AuraGo dashboard – section registry (window.DashSections). Classic script, loaded with defer after dash-overview.js
// and before main.js. The section modules (sections/<id>.js) register their loader here and
// main.js#loadSectionContent looks it up. register({ id, load }) throws on an unknown or duplicate id and on a missing
// loader; load({ force }) returns a Promise that resolves to false when the section failed (no "Updated" stamp) or
// to anything else. SSE wiring stays in dashboard-events.js. Nothing touches the DOM or another script at load time,
// so the file also runs in a bare Node vm context (ui/testdata/dash-sections.cjs).
(function () {
    'use strict';

    // Same ids and order as DashRouter.SECTIONS (dash-router.js); ui/testdata/dash-sections.cjs keeps them equal.
    const SECTION_IDS = Object.freeze([
        'overview', 'personality', 'memory', 'user', 'knowledge', 'filesync', 'problems',
        'automations', 'system', 'audit', 'logs', 'llm', 'prompts', 'tools',
    ]);
    const registry = new Map();

    function register(def) {
        const id = def && typeof def.id === 'string' ? def.id : '';
        if (SECTION_IDS.indexOf(id) === -1) throw new Error('DashSections.register: unknown section id "' + id + '"');
        if (registry.has(id)) throw new Error('DashSections.register: section "' + id + '" is already registered');
        if (typeof def.load !== 'function') throw new Error('DashSections.register: section "' + id + '" needs a load function');
        registry.set(id, Object.freeze({ id, load: def.load }));
    }

    // The registered definition { id, load }, or null.
    function get(id) {
        return registry.get(id) || null;
    }

    // Registered ids in registration order (the page order of the module scripts).
    function ids() {
        return Array.from(registry.keys());
    }

    window.DashSections = { register, get, ids, SECTION_IDS };
})();
