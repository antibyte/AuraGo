// EasyDrag icon map: one meaning, one glyph. The rail and the editor header draw their icons here, so a
// glyph never carries two meanings (the start page and the groups used to share "grid"). The glyph
// drawings stay in easydrag-core.js ICONS; this module only says which meaning uses which glyph and
// owns the short captions under the rail icons.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const core = ED.core;

    // MEANINGS maps a meaning to the name of one glyph of core.ICONS. Keep it one-to-one: the Node test
    // fails when two meanings name the same glyph.
    const MEANINGS = Object.freeze({
        palette: 'plus',
        home: 'home',
        templates: 'sparkles',
        groups: 'group-frame',
        outline: 'list',
        focus: 'fit',
        keys: 'keyboard',
        back: 'arrow-left',
        runs: 'history',
        test: 'flask',
        publish: 'rocket',
        run: 'play',
        stop: 'player-stop',
        approve: 'check',
        more: 'dots',
        settings: 'settings'
    });

    // RAIL maps each rail button to the locale key of its caption. The keys are literals on purpose: the
    // locale tests find literal keys, and a key built from a prefix would need its own family list.
    const RAIL = Object.freeze({
        palette: 'easydrag.ui.rail_palette',
        home: 'easydrag.ui.rail_home',
        templates: 'easydrag.ui.rail_templates',
        groups: 'easydrag.ui.rail_groups',
        outline: 'easydrag.ui.rail_outline',
        focus: 'easydrag.ui.rail_focus',
        keys: 'easydrag.ui.rail_keys'
    });

    // of returns the icon markup of a meaning. An unknown meaning draws core's fallback glyph, like core.icon.
    function of(meaning, cls) {
        const glyph = Object.prototype.hasOwnProperty.call(MEANINGS, meaning) ? MEANINGS[meaning] : '';
        return core.icon(glyph, cls);
    }

    // caption returns the short text under a rail icon. The button keeps its long name in aria-label and
    // title, so the caption is hidden from screen readers (it would read the name twice).
    function caption(t, id) {
        return '<span class="ed-rail-caption" aria-hidden="true">' + core.esc(t(RAIL[id])) + '</span>';
    }

    ED.icons = { MEANINGS, RAIL, of, caption };
})();
