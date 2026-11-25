// HomeDash control kit (P1/W1): rules shared by every control widget and the light sheet. Capabilities, ranges and options come from the entity's
// capabilities (attributes may be null; contract M3 section 2). Read-only reasons and command cues are localized here with literal keys. State line
// rule: host words first (unavailable, unknown, error cue, pending), then armed countdown, read-only "word · reason", blocked capability, stale age, word.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.format || !E.widgets || !E.icons) throw new Error('hd-control.js: load hd-widgets.js first');
    const F = E.format, MID = ' ' + String.fromCodePoint(0xb7) + ' ', num = v => (typeof v === 'number' && Number.isFinite(v) ? v : null);
    const isObj = v => v !== null && typeof v === 'object' && !Array.isArray(v);
    // Same lookup as hd-command (which loads later): a capability is a bare name or an object with a name.
    function capOf(view, name) { for (const c of view && Array.isArray(view.capabilities) ? view.capabilities : []) { if (c === name) return { name, allowed: true }; if (isObj(c) && c.name === name) return c; } return null; }
    const cap = capOf;
    function range(e, name, d) { const c = cap(e, name) || {}, a = (e && e.attributes) || {}, x = d || {};
        return { min: num(c.min) ?? num(a.min) ?? x.min ?? 0, max: num(c.max) ?? num(a.max) ?? x.max ?? 100, step: num(c.step) ?? num(a.step) ?? x.step ?? 1 }; }
    function ctRange(e) { const c = cap(e, 'color_temp') || {}, a = (e && e.attributes) || {}, min = num(c.min) ?? num(a.min_color_temp_kelvin) ?? 2000, max = num(c.max) ?? num(a.max_color_temp_kelvin) ?? 6500;
        return { min, max: max > min ? max : min + 100, step: num(c.step) ?? 50 }; }
    function options(e, name) { const c = cap(e, name), list = c && Array.isArray(c.options) ? c.options : e && e.attributes && Array.isArray(e.attributes.options) ? e.attributes.options : []; return list.filter(x => typeof x === 'string').slice(0, 64); }
    function lightModel(e) { const dead = !e || e.available === false || e.status === 'unavailable' || e.status === 'unknown', on = !dead && F.isOn(e) === true;
        const raw = e && e.attributes ? Number(e.attributes.brightness_pct) : NaN, known = on && Number.isFinite(raw) && raw > 0;
        return { dead, on, known, pct: dead ? null : known ? Math.min(100, Math.max(1, Math.round(raw))) : on ? 100 : 0, dimmable: !!cap(e, 'brightness') }; }
    function reasonKey(e, name) { const c = name ? cap(e, name) : null; return (c && typeof c.blocked_by === 'string' && c.blocked_by) || (e && e.readonlyReason) || ''; }
    function reasonText(t, key, long) { switch (key) {
        case 'homedash.readonly': return long ? t('homedash.reason.homedash_readonly_long') : t('homedash.reason.homedash_readonly');
        case 'virtual_desktop.readonly': return long ? t('homedash.reason.desktop_readonly_long') : t('homedash.reason.desktop_readonly');
        case 'homedash.sources.home_assistant.control': return long ? t('homedash.reason.source_control_long') : t('homedash.reason.source_control');
        case 'home_assistant.allowed_services': return long ? t('homedash.reason.allowed_services_long') : t('homedash.reason.allowed_services');
        case 'home_assistant.blocked_services': return long ? t('homedash.reason.blocked_services_long') : t('homedash.reason.blocked_services');
        case 'home_assistant.readonly': return long ? t('homedash.reason.ha_readonly_long') : t('homedash.reason.ha_readonly');
        default: return long ? t('homedash.reason.unknown_long') : t('homedash.state.readonly'); } }
    const sourceName = (t, src) => (src === 'ha' ? t('homedash.source.ha') : src === 'system' ? t('homedash.source.system') : String(src || ''));
    function cueText(t, err) { switch (err && err.code) {
        case 'timeout': return t('homedash.cue.timeout'); case 'command_failed': case 'source_error': return t('homedash.cue.command_failed');
        case 'source_unavailable': case 'hub_closed': return t('homedash.cue.source_unavailable', { source: sourceName(t, err.source || 'ha') });
        case 'rate_limited': return t('homedash.cue.rate_limited'); case 'network': case 'aborted': return t('homedash.cue.network');
        case 'invalid_args': case 'unsupported_command': case 'bad_request': return t('homedash.cue.invalid'); case 'entity_not_found': return t('homedash.cue.not_found');
        default: return t('homedash.cue.generic', { code: (err && err.code) || '?' }); } }
    const enabled = (api, e, caps) => !!e && e.available !== false && !e.readonly && [].concat(caps).some(c => api.can('primary', c));
    function announceBlocked(api, e, name) { if (!e) return; api.announce(e.available === false ? api.t('homedash.state.unavailable') : reasonText(api.t, reasonKey(e, name), true)); }
    function onOffCommand(api, e) { const on = F.isOn(e); if (on === true && api.can('primary', 'turn_off')) return 'turn_off'; if (on === false && api.can('primary', 'turn_on')) return 'turn_on'; return api.can('primary', 'toggle') ? 'toggle' : null; }
    function activateOnOff(api, e) { if (!e || e.available === false) return false; if (e.readonly) { announceBlocked(api, e, 'toggle'); return false; } const c = onOffCommand(api, e); if (!c) return false; api.command('primary', c, {}); return true; }
    function actions(widget) { const a = (widget && widget.actions) || {}, tap = a.tap && a.tap.do, hold = a.hold && a.hold.do;
        return { tap: ['default', 'toggle', 'more_info', 'none'].includes(tap) ? tap : 'default', hold: hold === 'none' ? 'none' : 'more_info' }; }   // navigate is not an M3 action
    function tap(api, e, primary) { const a = actions(api.widget); if (a.tap === 'none') return; if (a.tap === 'more_info') { api.moreInfo('primary'); return; } primary(); }
    function hold(api) { if (actions(api.widget).hold === 'more_info') api.moreInfo('primary'); }
    const label = (api, e) => api.widget.title || (e && e.name) || '';
    function statusText(api, e, word, o) {
        if (!e || e.available === false || ['unavailable', 'unknown', 'error'].includes(e.status) || e.pending) return null;
        if (e.armed) return api.t('homedash.state.armed', { seconds: e.armed.seconds });
        const c = o && o.cap ? cap(e, o.cap) : null;
        if (e.readonly) return word + MID + reasonText(api.t, reasonKey(e, o && o.cap), false);
        if (c && c.allowed === false) return word + MID + reasonText(api.t, reasonKey(e, o.cap), false);
        if (e.stale) { const old = E.widgets.heroStatus.staleText(api, e); if (old) return old; }
        if (e.confirmed && o && o.done) return api.t('homedash.state.done');
        return word;
    }
    const armMarker = () => E.core.el('span', { class: 'hd-arm', 'aria-hidden': 'true' });
    const decimalsOf = step => { const s = String(step), i = s.indexOf('.'); return i < 0 ? 0 : Math.min(3, s.length - i - 1); };
    E.control = Object.freeze({ cap, range, ctRange, options, lightModel, reasonKey, reasonText, sourceName, cueText, enabled, announceBlocked, onOffCommand, activateOnOff, actions, tap, hold, label, statusText, armMarker, decimalsOf, MID });
})();
