// EasyDrag status map (W1a A, FD-08): every run, step, effect and flow status maps to one of twelve values, and a value has one token, one glyph, one word and one aria sentence. Surfaces draw a status only through this module.
(function () {
    'use strict';
    const ED = window.EasyDrag = window.EasyDrag || {};
    const VALUES = ['ok', 'stopped', 'failed', 'running', 'waiting', 'uncertain', 'blocked', 'skipped', 'paused', 'simulated', 'not_run', 'cancelled'];
    // Glyphs are 16x16 line drawings in currentColor (the owner-approved mock). The disc sets the colour.
    const SQUARE = '<rect x="4.8" y="4.8" width="6.4" height="6.4" rx="1" fill="currentColor"/>';
    const GLYPHS = {
        ok: '<path d="M3.5 8.5l3 3 6-7"/>',
        stopped: SQUARE,
        failed: '<path d="M4.5 4.5l7 7M11.5 4.5l-7 7"/>',
        running: '<path d="M8 2.5a5.5 5.5 0 1 1-5.5 5.5"/>',
        waiting: '<circle cx="8" cy="8" r="5.5"/><path d="M8 5v3.2l2.2 1.4"/>',
        uncertain: '<path d="M5.8 6a2.3 2.3 0 1 1 3.2 2.1c-.7.3-1 .8-1 1.5v.4"/><circle cx="8" cy="12.3" r=".6" fill="currentColor"/>',
        blocked: '<circle cx="8" cy="8" r="5.5"/><path d="M4.2 11.8l7.6-7.6"/>',
        skipped: '<path d="M3 8h7M7.5 5l3 3-3 3M13 4.5v7"/>',
        paused: '<path d="M6 4.5v7M10 4.5v7"/>',
        simulated: '<path d="M8 2.8l5.2 5.2L8 13.2 2.8 8z"/>',
        not_run: '<circle cx="8" cy="8" r="5.2" stroke-dasharray="2 2"/>',
        cancelled: SQUARE
    };
    const TOKENS = { ok: '--ed-ok', stopped: '--ed-ok', failed: '--ed-danger', running: '--ed-info', waiting: '--ed-info', uncertain: '--ed-warn', blocked: '--ed-warn' };
    const table = {};
    VALUES.forEach(value => {
        table[value] = Object.freeze({ token: TOKENS[value] || '--ed-status-neutral', glyph: GLYPHS[value], word: 'easydrag.status.' + value, aria: 'easydrag.status.' + value + '_aria' });
    });
    Object.freeze(table);

    // Today's raw values (internal/flows/run_types.go; durable states in easydrag-durable.js). A raw value that no map knows is
    // "uncertain": an unknown status must never read as a success.
    const RAW = {
        run: { queued: 'waiting', running: 'running', waiting: 'waiting', success: 'ok', error: 'failed', cancelled: 'cancelled' },
        step: { pending: 'not_run', running: 'running', waiting: 'waiting', success: 'ok', error: 'failed', skipped: 'skipped', cancelled: 'cancelled' },
        effect: { effect_uncertain: 'uncertain', uncertain: 'uncertain', unknown: 'uncertain' },
        flow: { paused: 'paused', disabled: 'paused', inactive: 'paused', off: 'paused' }
    };
    // Raw values that mean the same in every kind: an open approval or a missing setup (blocked), a step that ran in Simulation.
    const ANY = { blocked: 'blocked', approval_pending: 'blocked', needs_setup: 'blocked', simulated: 'simulated', effect_uncertain: 'uncertain' };
    const own = (object, key) => Object.prototype.hasOwnProperty.call(object, key);
    let bound = null;

    function bind(t) { bound = typeof t === 'function' ? t : null; }
    function known(value) { return typeof value === 'string' && own(table, value) ? value : 'uncertain'; }
    function of(raw, kind) {
        if (raw === false && kind === 'effect') return 'ok';
        const key = raw === true ? 'uncertain' : typeof raw === 'string' ? raw : '';
        const maps = RAW[kind] ? [RAW[kind]] : [RAW.run, RAW.step];
        for (const map of maps) if (own(map, key)) return map[key];
        if (own(ANY, key)) return ANY[key];
        return own(table, key) && kind !== 'flow' ? key : 'uncertain';
    }
    // ofStep is the value of a step record; a step that succeeded in a simulation is "simulated".
    function ofStep(step, simulation) {
        const value = of(step && step.status, 'step');
        return simulation && value === 'ok' ? 'simulated' : value;
    }
    const escape = text => String(text).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&#39;');
    function translate(t, key) {
        const fn = typeof t === 'function' ? t : bound;
        const text = fn ? fn(key) : '';
        return text && text !== key ? text : '';
    }
    function word(t, value) { const v = known(value); return translate(t, table[v].word) || v; }
    function aria(t, value) { const v = known(value); return translate(t, table[v].aria) || word(t, v); }
    // disc(value, size, opts): opts.t translates, opts.title adds a tooltip, opts.hidden hides the disc from screen readers
    // (label() does that: its word already says it).
    function disc(value, size, opts) {
        const v = known(value), o = opts || {};
        const px = Number.isFinite(size) && size > 0 ? Math.round(size) : 18;
        return '<span class="ed-status-disc" data-status="' + v + '" role="img" aria-label="' + escape(aria(o.t, v)) + '"' + (o.hidden ? ' aria-hidden="true"' : '') + (o.title ? ' title="' + escape(o.title) + '"' : '') +
            ' style="--ed-disc:' + px + 'px"><svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false">' + table[v].glyph + '</svg></span>';
    }
    // label(t, value, text): the disc plus the word; text replaces the word when a surface has a more exact one ("Stoppt…").
    function label(t, value, text) {
        const v = known(value);
        return '<span class="ed-status-label" data-status="' + v + '">' + disc(v, 14, { t, hidden: true }) + '<span class="ed-status-word">' + escape(text || word(t, v)) + '</span></span>';
    }

    ED.status = { VALUES: Object.freeze(VALUES.slice()), table, of, ofStep, disc, word, aria, label, bind };
})();
