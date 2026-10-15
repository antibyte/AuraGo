// CYD Creator designer inspector (right panel "Properties"): project settings when nothing is
// selected (theme with live preview, orientation, Wi-Fi, AuraGo link, libraries, backlight, status
// bar clock, language, timezone), and for one widget its name, geometry, props per kind, style
// overrides and events. Field editors live in cyd-creator-inspector-fields.js. Every edit is a
// designer command (store origin local); values are stored only in the form validate.go accepts and
// a refused entry (an invalid name being typed …) stays in its field with the reason. The inspector's
// own edits do not re-render it (the focused field keeps its caret); handlers read the current
// widget from the store at event time. New issue lists (after every save) only replace the issue
// list, never the fields, and are announced; other store changes re-render with the focus, caret
// and refused entries kept, and wait while an input method composes text. Read-only or locked by an
// agent job (the designer's 'lock' signal), the fields are disabled.
(function () {
    'use strict';

    const P = window.CYDCreatorParts = window.CYDCreatorParts || {};
    // TIMEZONES are the presets of the time zone field (cyd-creator-home.js ZONES holds the same POSIX
    // strings for the new-project dialog).
    const TIMEZONES = [
        ['Europe/Berlin', 'CET-1CEST,M3.5.0,M10.5.0/3'], ['Europe/London', 'GMT0BST,M3.5.0/1,M10.5.0'], ['Europe/Lisbon', 'WET0WEST,M3.5.0/1,M10.5.0'],
        ['Europe/Athens', 'EET-2EEST,M3.5.0/3,M10.5.0/4'], ['UTC', 'UTC0'], ['America/New_York', 'EST5EDT,M3.2.0,M11.1.0'],
        ['America/Chicago', 'CST6CDT,M3.2.0,M11.1.0'], ['America/Denver', 'MST7MDT,M3.2.0,M11.1.0'], ['America/Los_Angeles', 'PST8PDT,M3.2.0,M11.1.0'],
        ['Asia/Kolkata', 'IST-5:30'], ['Asia/Shanghai', 'CST-8'], ['Asia/Tokyo', 'JST-9'], ['Australia/Sydney', 'AEST-10AEDT,M10.1.0,M4.1.0/3']
    ];
    // LIMITS mirror the server: maxCoordinate and intPropLimit (internal/cydcreator/limits.go),
    // maxTimezoneBytes, the backlight and the style ranges (validate.go). Widget sides use
    // P.geometry.MAX_SIDE (maxWidgetSide), texts P.model.LIMITS.
    const LIMITS = { coord: 4096, intProp: 1000000, timezoneBytes: 64, backlight: [10, 100], style: { bg_opacity: 255, border_width: 8, radius: 64 } };
    const STYLE_COLORS = [['bg_color', 'surface'], ['text_color', 'text'], ['accent_color', 'accent'], ['border_color', 'border']];

    // pretty turns a catalog id into an English fallback label ("slide_left" → "Slide left").
    function pretty(id) {
        const s = String(id || '').replace(/_/g, ' ');
        return s.charAt(0).toUpperCase() + s.slice(1);
    }
    // languageName is the language's own name, as in the new-project dialog (cyd-creator-home.js).
    function languageName(code) {
        try {
            const name = new Intl.DisplayNames([code], { type: 'language' }).of(code);
            return name ? name.charAt(0).toUpperCase() + name.slice(1) : code;
        } catch (err) {
            return code;
        }
    }

    function mount(d, host) {
        const { h, dom, icon, swap } = P.render;
        // Number clamps and issue changes are spoken by the designer's polite live region.
        const F = P.designerFields.create(d.t, { announce: text => d.announce(text) });
        const M = P.model;
        const MAX_SIDE = P.geometry.MAX_SIDE;
        const t = d.t;
        const bag = P.util.bag();
        // ui.panel identifies what is shown (project, w:<id>, multi, system); ui.issuesOf lists the
        // issues of that panel for the in-place issue list, ui.issueKey is that list as shown.
        // ui.uploading holds the widget ids with an upload in flight, ui.hooking the pending
        // "Create hook" requests (<widget id>:<event>). While an input method composes text
        // (ui.composing) a re-render waits for compositionend (ui.deferred).
        const ui = { uploading: new Set(), hooking: new Set(), panel: '', issuesOf: null, issueKey: '', composing: false, deferred: false, disposed: false };
        const notify = text => { if (typeof d.inst.notify === 'function') d.inst.notify('error', text); };
        // drafts holds refused entries per field key; a re-render of the same panel shows them again
        // with their message, another panel starts without them.
        const drafts = new Map();
        // live: elements of the shown panel that own edits update in place (the panel is not rebuilt).
        let live = { issues: null, hooks: [] };
        const root = dom(h('div', { class: 'cydc-inspector', role: 'region', tabindex: '-1', 'aria-label': t.or('cyd_creator.designer.inspector_title', 'Properties') }));
        host.appendChild(root);
        const current = id => { const f = d.find(id); return f ? f.widget : null; };
        const widgetLabel = type => t.or('cyd_creator.widget.' + type, pretty(type));
        const str = v => (typeof v === 'string' ? v : v == null ? '' : String(v));
        const themeDefault = () => t.or('cyd_creator.designer.theme_default', 'Theme default');
        const hookReady = () => !!(d.inst.code && typeof d.inst.code.ensureHook === 'function');

        // edit applies ops; rerender=false keeps the inspector as it is (the user is typing).
        function edit(label, ops, coalesce, rerender) {
            d.inspectorEditing = !rerender;
            try { return d.commands.run(label, ops, coalesce); } finally { d.inspectorEditing = false; }
        }
        const setWidget = (id, set, coalesce, rerender) => edit('edit', [{ op: 'update_widget', id, set }], coalesce, rerender);
        const setSettings = (set, coalesce, rerender) => edit('settings', [{ op: 'set_settings', set }], coalesce, rerender);
        const setProject = (set, coalesce, rerender) => edit('project', [{ op: 'set_project', set }], coalesce, rerender);
        const section = (title, ...body) => h('section', { class: 'cydc-insp-section' }, h('h4', {}, title), body);
        // guard gives a checked field its refused entry (draft) and keeps the next one.
        const guard = (key, check, extra) => Object.assign({ check, draft: drafts.get(key), onRefuse: raw => drafts.set(key, raw), onAccept: () => drafts.delete(key) }, extra || {});

        // ── value checks (internal/cydcreator/validate.go) ──────────────────
        const tooLong = max => t.or('cyd_creator.designer.text_too_long', 'Too long: at most {max} bytes.', { max });
        const invalidText = () => t.or('cyd_creator.issue.PROP_INVALID', 'A property has an invalid value.');
        const textCheck = limit => raw => {
            const code = P.designerFields.textIssue(raw, limit);
            return code === 'TEXT_TOO_LONG' ? tooLong(limit) : code ? invalidText() : '';
        };
        const nameMessage = code => {
            if (code === 'NAME_REQUIRED') return t.or('cyd_creator.issue.NAME_REQUIRED', 'Widgets with events need a name.');
            if (code === 'NAME_DUPLICATE') return t.or('cyd_creator.issue.NAME_DUPLICATE', 'Another widget already uses this name.');
            return t.or('cyd_creator.issue.NAME_INVALID', 'Names use lowercase letters, digits and _, start with a letter (max. 32) and must not be reserved for generated code.');
        };
        // timezoneProblem: 1..64 printable ASCII characters.
        const timezoneCheck = raw => {
            const v = String(raw).trim();
            return v && v.length <= LIMITS.timezoneBytes && !/[^\x20-\x7e]/.test(v) ? '' : t.or('cyd_creator.designer.timezone_invalid', 'Enter a POSIX time zone of 1 to 64 plain ASCII characters, e.g. UTC0.');
        };
        // A list holds at most max_len items of at most 1 KiB each; options, tabs, chart series and
        // message box buttons (and the keys of a custom keypad) need at least one item.
        const listCheck = (w, spec, ps) => items => {
            if (items.length > (Number(ps.max_len) || 0)) return t.or('cyd_creator.designer.list_too_many', 'At most {max} entries.', { max: Number(ps.max_len) || 0 });
            const bad = items.map(item => P.designerFields.textIssue(item, M.LIMITS.textBytes)).find(Boolean);
            if (bad) return bad === 'TEXT_TOO_LONG' ? tooLong(M.LIMITS.textBytes) : invalidText();
            const required = ps.name === 'options' || (w.type === 'tabview' && ps.name === 'tabs') || (w.type === 'chart' && ps.name === 'series') ||
                (w.type === 'msgbox' && ps.name === 'buttons') || (w.type === 'keypad' && ps.name === 'keys' && M.propValue(current(w.id) || w, spec, 'layout') === 'custom');
            return required && !items.length ? t.or('cyd_creator.designer.list_required', 'Add at least one entry.') : '';
        };
        const hintCheck = raw => {
            const v = String(raw);
            if (v.includes(String.fromCharCode(0))) return t.or('cyd_creator.issue.ACTION_INVALID', 'The action is incomplete or not possible.');
            return Array.from(v).length > M.LIMITS.hintRunes ? t.or('cyd_creator.issue.HINT_TOO_LONG', 'The logic hint is too long (max. 500 characters).') : '';
        };

        // ── issues (replaced in place when the shell reports a new list) ────
        const issueList = list => (list.length ? h('ul', { class: 'cydc-issues', role: 'list' }, list.map(i => h('li', { class: 'cydc-issue is-' + (i.severity === 'error' ? 'error' : 'warning') }, d.issueText(i)))) : null);
        const issueBox = () => h('div', { class: 'cydc-insp-issues', ref: el => { live.issues = el; } }, issueList(ui.issuesOf ? ui.issuesOf() : []));
        const issueKey = list => JSON.stringify(list.map(i => [i.code, i.severity, i.path || '']));
        function refreshIssues() {
            if (ui.disposed || !live.issues || !ui.issuesOf) return;
            const list = ui.issuesOf();
            const key = issueKey(list);
            if (key === ui.issueKey) return;
            ui.issueKey = key;
            const node = issueList(list);
            live.issues.replaceChildren(...(node ? [dom(node)] : []));
            d.announce(list.length ? t.or('cyd_creator.designer.issue_count', 'Issues: {count}', { count: list.length }) : t.or('cyd_creator.designer.issues_none', 'No issues left.'));
        }

        // ── project settings ────────────────────────────────────────────────
        function themeGrid(currentId) {
            const themes = d.inst.caps.themes || [];
            const pick = id => { d.setPreviewTheme(''); edit('theme', [{ op: 'set_theme', theme: id }], null, true); };
            const tabbable = Math.max(0, themes.findIndex(x => x.id === currentId));
            return h('div', {
                class: 'cydc-theme-grid', role: 'radiogroup', 'aria-label': t.or('cyd_creator.designer.section_theme', 'Theme'),
                on: {
                    keydown(event) {
                        const step = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 }[event.key];
                        if (!step) return;
                        event.preventDefault();
                        const i = themes.findIndex(x => x.id === d.project().theme);
                        const next = themes[(i + step + themes.length) % themes.length];
                        if (!next || d.readonly()) return;
                        pick(next.id);
                        const swatch = Array.from(root.querySelectorAll('[data-cc-theme]')).find(el => el.dataset.ccTheme === next.id);
                        if (swatch) swatch.focus();
                    },
                    pointerleave() { d.setPreviewTheme(''); }
                }
            }, themes.map((th, index) => {
                const tk = th.tokens || {};
                const on = th.id === currentId;
                return h('button', {
                    type: 'button', role: 'radio', class: 'cydc-theme-swatch', 'aria-checked': on ? 'true' : 'false', tabindex: index === tabbable ? '0' : '-1',
                    'data-cc-theme': th.id, 'data-cc-focus': 'theme:' + th.id,
                    on: { click: () => pick(th.id), pointerenter: () => d.setPreviewTheme(th.id), focus: () => d.setPreviewTheme(th.id), blur: () => d.setPreviewTheme('') }
                }, h('span', { class: 'cydc-swatch', style: { background: tk.bg, color: tk.text, 'border-color': tk.border, 'font-family': P.render.fontFamily(th.font) } },
                    h('span', { class: 'cydc-swatch-surface', style: { background: tk.surface } }), h('span', { class: 'cydc-swatch-accent', style: { background: tk.accent } }), 'Aa'),
                h('span', { class: 'cydc-theme-name' }, t.or('cyd_creator.theme.' + th.id, pretty(th.id))));
            }));
        }

        function projectPanel(ro) {
            const p = d.project();
            const s = p.settings || {};
            const board = d.board();
            const display = (board && board.display) || {};
            // An NV3041A panel cannot rotate by register: only its native orientation family works.
            const native = display.driver === 'NV3041A' ? (display.native_orientation || 'landscape') : '';
            const wifiOff = (s.wifi || 'off') === 'off';
            const caps = d.inst.caps;
            const outId = P.util.uid('cc_bl');
            const rangeId = P.util.uid('cc_in');
            const tzList = P.util.uid('cc_tz');
            const backlight = Number.isInteger(s.backlight) && s.backlight >= LIMITS.backlight[0] && s.backlight <= LIMITS.backlight[1] ? s.backlight : LIMITS.backlight[1];
            const libs = new Set(Array.isArray(s.libraries) ? s.libraries : []);
            let out = null;
            return h('div', { class: 'cydc-insp', 'data-cc-inspect': 'project' },
                h('div', { class: 'cydc-insp-head' }, h('h3', {}, t.or('cyd_creator.designer.project_title', 'Project')), h('span', { class: 'cydc-insp-sub' }, str(p.name))),
                issueBox(),
                h('fieldset', { class: 'cydc-fieldset', disabled: ro },
                    section(t.or('cyd_creator.designer.section_theme', 'Theme'), themeGrid(p.theme)),
                    section(t.or('cyd_creator.designer.section_display', 'Display'),
                        board ? h('p', { class: 'cydc-board' }, h('span', {}, board.label), h('span', { class: 'cydc-badge cydc-badge-' + board.status }, t.or('cyd_creator.board.status_' + board.status, pretty(board.status)))) : null,
                        F.select(t.or('cyd_creator.designer.orientation', 'Orientation'), 'orientation',
                            M.ORIENTATIONS.map(o => [o, t.or('cyd_creator.orientation.' + o, pretty(o)), !!native && o.startsWith('portrait') !== (native === 'portrait')]),
                            p.orientation, v => setProject({ orientation: v }, null, true)),
                        h('div', { class: 'cydc-field' }, h('label', { class: 'cydc-field-label', for: rangeId }, t.or('cyd_creator.designer.backlight', 'Backlight')),
                            h('div', { class: 'cydc-range-row' },
                                h('input', {
                                    id: rangeId, type: 'range', min: String(LIMITS.backlight[0]), max: String(LIMITS.backlight[1]), step: '5', value: String(backlight),
                                    'data-cc-field': 'backlight', 'data-cc-focus': 'backlight', 'aria-describedby': outId,
                                    on: {
                                        input: e => {
                                            const n = P.designerFields.clampInt(e.target.value, LIMITS.backlight[0], LIMITS.backlight[1]);
                                            if (n === null) return;
                                            if (out) out.replaceChildren(document.createTextNode(n + ' %'));
                                            setSettings({ backlight: n }, 'backlight', false);
                                        }
                                    }
                                }),
                                h('output', { id: outId, ref: el => { out = el; } }, backlight + ' %')))),
                    section(t.or('cyd_creator.designer.section_connectivity', 'Connection'),
                        F.select(t.or('cyd_creator.designer.wifi', 'Wi-Fi'), 'wifi', M.WIFI_MODES.map(m => [m, t.or('cyd_creator.designer.wifi_' + m, pretty(m))]), s.wifi,
                            v => setSettings(v === 'off' ? { wifi: v, aurago_link: false } : { wifi: v }, null, true)),
                        // The link needs Wi-Fi; a link left on while Wi-Fi is off can still be switched off.
                        F.check(t.or('cyd_creator.designer.aurago_link', 'AuraGo link (heartbeat and OTA updates)'), 'aurago_link', !!s.aurago_link, v => setSettings({ aurago_link: v }, null, true),
                            { disabled: wifiOff && !s.aurago_link, hint: t.or('cyd_creator.designer.aurago_link_hint', 'Needs Wi-Fi.') }),
                        F.check(t.or('cyd_creator.designer.statusbar_clock', 'Clock in the status bar'), 'statusbar_clock', !!s.statusbar_clock, v => setSettings({ statusbar_clock: v }, null, true),
                            { hint: t.or('cyd_creator.designer.statusbar_clock_hint', 'The time comes from the internet and needs Wi-Fi.') })),
                    section(t.or('cyd_creator.designer.section_region', 'Language and time'),
                        F.select(t.or('cyd_creator.designer.language', 'Device language'), 'language', (caps.languages || []).map(c => [c, languageName(c)]), p.language, v => setProject({ language: v }, null, true)),
                        F.text(t.or('cyd_creator.designer.timezone', 'Time zone'), 'timezone', str(p.timezone),
                            guard('timezone', timezoneCheck, { maxlength: LIMITS.timezoneBytes, list: tzList, hint: t.or('cyd_creator.designer.timezone_hint', 'POSIX time zone, e.g. CET-1CEST,M3.5.0,M10.5.0/3 for Berlin.') }),
                            v => setProject({ timezone: v.trim() }, 'timezone', false)),
                        h('datalist', { id: tzList }, TIMEZONES.map(([label, value]) => h('option', { value, label })))),
                    section(t.or('cyd_creator.designer.section_libraries', 'Libraries'),
                        h('div', { class: 'cydc-check-list' }, (caps.libraries || []).map(lib => F.check(lib.name, 'lib:' + lib.id, libs.has(lib.id), on => {
                            const next = new Set(Array.isArray((d.project().settings || {}).libraries) ? d.project().settings.libraries : []);
                            if (on) next.add(lib.id); else next.delete(lib.id);
                            setSettings({ libraries: (caps.libraries || []).map(l => l.id).filter(id => next.has(id)) }, null, true);
                        }))))));
        }

        // ── widget panel ────────────────────────────────────────────────────
        // syncHooks updates the hook rows of the shown panel in place (name from the store, pending
        // requests): the panel is not rebuilt for own edits.
        const syncHooks = () => live.hooks.forEach(fn => fn());

        function nameField(w) {
            const check = raw => {
                const v = String(raw).trim();
                if (!v) { const cur = current(w.id); return cur && Array.isArray(cur.events) && cur.events.length ? nameMessage('NAME_REQUIRED') : ''; }
                const issue = M.nameIssue(d.project(), v, w.id);
                return issue ? nameMessage(issue) : '';
            };
            // An invalid name stays in the field with its message; the layout keeps the last valid name.
            return F.text(t.or('cyd_creator.designer.name', 'Name'), 'name', str(w.name),
                guard('name', check, { maxlength: 32, hint: t.or('cyd_creator.designer.name_hint', 'Lowercase letters, digits and _ (starts with a letter, max. 32). Code reaches the widget as ui_<name>.') }),
                v => { setWidget(w.id, { name: v.trim() }, 'name:' + w.id, false); syncHooks(); });
        }

        function geometry(f) {
            const w = f.widget;
            const labels = { x: t.or('cyd_creator.designer.geo_x', 'X'), y: t.or('cyd_creator.designer.geo_y', 'Y'), w: t.or('cyd_creator.designer.geo_w', 'Width'), h: t.or('cyd_creator.designer.geo_h', 'Height') };
            const range = key => (key === 'w' || key === 'h' ? { min: 1, max: MAX_SIDE } : { min: -LIMITS.coord, max: LIMITS.coord });
            const num = key => F.number(labels[key], 'geo:' + key, w[key], range(key), v => setWidget(w.id, { [key]: v }, 'geo:' + w.id + ':' + key, false));
            const tabs = f.parent && f.parent.type === 'tabview' ? (M.propValue(f.parent, d.spec('tabview'), 'tabs') || []) : null;
            return [
                h('div', { class: 'cydc-grid-2' }, num('x'), num('y'), num('w'), num('h')),
                Array.isArray(tabs) ? F.select(t.or('cyd_creator.designer.tab_page', 'Tab page'), 'tab', tabs.map((label, i) => [String(i), str(label)]), String(w.tab || 0), v => {
                    const tab = P.designerFields.clampInt(v, 0, Math.max(0, tabs.length - 1));
                    if (tab === null) return;
                    setWidget(w.id, { tab }, null, true);
                    d.setTab(f.parent.id, tab);
                }) : null,
                h('div', { class: 'cydc-grid-2' },
                    F.check(t.or('cyd_creator.designer.lock', 'Lock'), 'locked', !!w.locked, v => setWidget(w.id, { locked: v }, null, true)),
                    F.check(t.or('cyd_creator.designer.hide', 'Hide'), 'hidden', !!w.hidden, v => setWidget(w.id, { hidden: v }, null, true)))
            ];
        }

        function propField(w, spec, ps) {
            // A value equal to the catalog default is stored as "no prop" (the default applies).
            const commit = (value, rerender) => {
                const same = JSON.stringify(value) === JSON.stringify(ps.default);
                setWidget(w.id, { props: { [ps.name]: same ? null : value } }, 'prop:' + w.id + ':' + ps.name, !!rerender);
            };
            const value = M.propValue(w, spec, ps.name);
            const label = t.or('cyd_creator.prop.' + ps.name, pretty(ps.name));
            const key = 'prop:' + ps.name;
            const tk = d.theme().tokens || {};
            const fonts = d.inst.caps.fonts || {};
            const limit = Number(ps.max_len) > 0 ? Number(ps.max_len) : M.LIMITS.textBytes;
            if (ps.name === 'icon_animate') {
                const asset = (d.project().assets || []).find(a => a.id === (w.props || {})[w.type === 'button' ? 'icon_asset' : 'asset']);
                if (!asset || asset.w > 64 || asset.h > 64) return null;
            }
            switch (ps.kind) {
            case 'text': return F.textarea(label, key, str(value), guard(key, textCheck(limit), { maxlength: limit }), v => commit(v));
            case 'int': return F.number(label, key, value, { min: typeof ps.min === 'number' ? ps.min : -LIMITS.intProp, max: typeof ps.max === 'number' ? ps.max : LIMITS.intProp }, v => commit(v));
            case 'bool': return F.check(label, key, value === true, v => {
                if (ps.name === 'icon_animate' && v && (w.props || {}).icon_motion === 'frames' && ((w.props || {}).icon_frames || []).length !== 6 && P.iconLibrary) {
                    P.iconLibrary.open(d, w.id, bag, { animated: true });
                } else commit(v, true);
            });
            case 'enum': return F.select(label, key, (ps.options || []).map(o => [o, t.or('cyd_creator.designer.opt_' + ps.name + '_' + o, pretty(o))]), str(value), v => commit(v, true));
            case 'color': return F.color(label, key, str(value), ps.name === 'icon_tint' ? tk.text : tk.accent, v => commit(v), () => commit('', true));
            case 'font': return F.select(label, key, [['', themeDefault()]].concat((fonts.families || []).map(f => [f, t.or('cyd_creator.designer.font_' + f, pretty(f))])), str(value), v => commit(v, true));
            case 'font_size': return F.select(label, key, [['0', themeDefault()]].concat((fonts.sizes || []).map(n => [String(n), n + ' px'])), String(value || 0), v => {
                const n = P.designerFields.clampInt(v, 0, null);
                if (n !== null) commit(n, true);
            });
            case 'symbol': return F.select(label, key, [['', t.or('cyd_creator.designer.symbol_none', 'No symbol')]].concat((d.inst.caps.symbols || []).map(s => [s, P.render.symbolGlyph(s) + '  ' + s])), str(value), v => commit(v, true));
            case 'list': return F.list(label, key, Array.isArray(value) ? value.map(str) : [], ps.name === 'keys', guard(key, listCheck(w, spec, ps)), v => commit(v));
            case 'asset': return [F.asset(d, label, key, str(value), ui.uploading.has(w.id), v => setWidget(w.id, { props: { [ps.name]: v || null, icon_frames: null, icon_animate: null, icon_origin: null } }, null, true), file => upload(w.id, file)),
                P.iconLibrary ? h('button', { type: 'button', class: 'cydc-chip', 'data-cc-focus': 'icon-library', on: { click: () => P.iconLibrary.open(d, w.id, bag) } }, t.or('cyd_creator.icons.title', 'Icon library')) : null];
            default: return F.text(label, key, str(value), guard(key, textCheck(limit), { maxlength: limit }), v => commit(v));
            }
        }

        function styleFields(w) {
            const st = w.style || {};
            const tk = d.theme().tokens || {};
            const setStyle = (k, v, rerender) => setWidget(w.id, { style: { [k]: v } }, 'style:' + w.id + ':' + k, !!rerender);
            const label = k => t.or('cyd_creator.prop.' + k, pretty(k));
            const color = ([k, token]) => F.color(label(k), 'style:' + k, str(st[k]), tk[token], v => setStyle(k, v), () => setStyle(k, null, true));
            const num = k => F.optionalNumber(label(k), 'style:' + k, st[k], { min: 0, max: LIMITS.style[k] }, t.or('cyd_creator.designer.theme_value', 'Theme'), v => setStyle(k, v));
            return [STYLE_COLORS.slice(0, 1).map(color), num('bg_opacity'), STYLE_COLORS.slice(1).map(color),
                h('div', { class: 'cydc-grid-2' }, num('border_width'), num('radius'))];
        }

        function eventsEditor(w, spec) {
            const allowed = (spec && spec.events) || [];
            const events = Array.isArray(w.events) ? w.events : [];
            const none = h('p', { class: 'cydc-panel-empty' }, t.or('cyd_creator.designer.events_none', 'This widget has no events.'));
            // Events on a type without events (an imported or agent-written layout) stay listed, so
            // they can be removed.
            if (!allowed.length && !events.length) return none;
            const screens = d.project().screens || [];
            const wifiOff = ((d.project().settings || {}).wifi || 'off') === 'off';
            const listOf = () => P.util.deepClone((current(w.id) || {}).events || []);
            // Rows are addressed by index: adding or removing one drops the refused entries of all rows.
            const dropEventDrafts = () => Array.from(drafts.keys()).forEach(k => { if (k.startsWith('event:')) drafts.delete(k); });
            const save = (list, rerender, coalesce) => {
                const cur = current(w.id);
                const set = { events: list };
                // Widgets with events need a name (NAME_REQUIRED): an unnamed one gets a proposed name.
                if (cur && !cur.name && list.length) { set.name = M.proposeName(d.project(), cur.type); drafts.delete('name'); }
                setWidget(w.id, set, coalesce || null, rerender);
            };
            const patch = (i, fn, rerender, coalesce) => {
                const list = listOf();
                if (!list[i]) return;
                fn(list[i]);
                save(list, rerender, coalesce);
            };
            // normalize builds the action shapes validate.go accepts: goto_screen {screen (id),
            // transition?}, logic {hint}, back and open_wifi_setup {} — no other keys.
            const normalize = (action, type) => {
                if (type === 'goto_screen') {
                    const all = d.project().screens || [];
                    const known = action.screen ? M.findScreen(d.project(), action.screen) : null;
                    const next = { type, screen: known ? known.screen.id : ((all.find(s => s.id !== d.screenId) || all[0] || {}).id || '') };
                    if (action.transition) next.transition = action.transition;
                    return next;
                }
                if (type === 'logic') return { type, hint: str(action.hint) };
                return { type };
            };
            const rows = events.map((ev, i) => {
                const action = (ev && ev.action) || { type: 'logic' };
                const on = str(ev && ev.on);
                const used = new Set(events.filter((_, j) => j !== i).map(e => e && e.on));
                const hookName = name => 'on_' + (name || '…') + '_' + on + '()';
                const hookOff = name => !M.validName(name) || !hookReady() || ui.hooking.has(w.id + ':' + on);
                let code = null;
                let button = null;
                if (action.type === 'logic') {
                    live.hooks.push(() => {
                        const name = (current(w.id) || {}).name;
                        if (code) code.replaceChildren(document.createTextNode(hookName(name)));
                        if (button) button.disabled = hookOff(name);
                    });
                }
                return h('div', { class: 'cydc-event', 'data-cc-event': String(i) },
                    h('div', { class: 'cydc-event-head' },
                        F.select(t.or('cyd_creator.designer.event_on', 'Event'), 'event:' + i + ':on', allowed.map(e => [e, t.or('cyd_creator.event.' + e, pretty(e)), used.has(e)]), on, v => {
                            // An event of another type, or one bound already, would be EVENT_NOT_ALLOWED.
                            if (!allowed.includes(v) || listOf().some((e, j) => j !== i && e && e.on === v)) return;
                            patch(i, b => { b.on = v; }, true);
                        }),
                        F.select(t.or('cyd_creator.designer.event_action', 'Action'), 'event:' + i + ':action',
                            M.ACTIONS.map(a => [a, t.or('cyd_creator.action.' + a, pretty(a)), a === 'open_wifi_setup' && wifiOff]), str(action.type),
                            v => patch(i, b => { b.action = normalize(b.action || {}, v); }, true)),
                        h('button', {
                            type: 'button', class: 'cydc-tool', 'data-cc-focus': 'event:' + i + ':remove', title: t.or('cyd_creator.designer.event_remove', 'Remove event'), 'aria-label': t.or('cyd_creator.designer.event_remove', 'Remove event'),
                            on: { click: () => { const list = listOf(); list.splice(i, 1); dropEventDrafts(); save(list, true); } }
                        }, icon('trash'))),
                    action.type === 'goto_screen' ? h('div', { class: 'cydc-event-body' },
                        F.select(t.or('cyd_creator.designer.event_screen', 'Target screen'), 'event:' + i + ':screen', screens.map(s => [s.id, str(s.name)]), str(action.screen),
                            v => { if (screens.some(s => s.id === v)) patch(i, b => { b.action.screen = v; }, true); }, t.or('cyd_creator.designer.screen_missing', 'Missing screen')),
                        F.select(t.or('cyd_creator.designer.event_transition', 'Transition'), 'event:' + i + ':transition',
                            [['', t.or('cyd_creator.designer.transition_screen_default', 'Screen’s own transition')]].concat(M.TRANSITIONS.map(x => [x, t.or('cyd_creator.transition.' + x, pretty(x))])),
                            str(action.transition), v => patch(i, b => { if (v) b.action.transition = v; else delete b.action.transition; }, true))) : null,
                    action.type === 'logic' ? h('div', { class: 'cydc-event-body' },
                        F.textarea(t.or('cyd_creator.designer.event_hint', 'What should happen? (for the agent)'), 'event:' + i + ':hint', str(action.hint),
                            guard('event:' + i + ':hint', hintCheck, { placeholder: t.or('cyd_creator.designer.event_hint_placeholder', 'e.g. reload the weather and show it'), counter: raw => Array.from(String(raw)).length + ' / ' + M.LIMITS.hintRunes }),
                            v => patch(i, b => { if (!b.action) b.action = { type: 'logic' }; b.action.hint = v; }, false, 'hint:' + w.id + ':' + i)),
                        h('div', { class: 'cydc-hook-row' },
                            h('code', { class: 'cydc-hook-name', ref: el => { code = el; } }, hookName(w.name)),
                            h('button', {
                                type: 'button', class: 'cydc-chip', 'data-cc-focus': 'event:' + i + ':hook', disabled: hookOff(w.name), ref: el => { button = el; },
                                on: { click: () => createHook(w.id, on) }
                            }, icon('code'), t.or('cyd_creator.designer.create_hook', 'Create hook')))) : null);
            });
            if (!allowed.length) return [none, rows];
            const free = allowed.filter(e => !events.some(b => b && b.on === e));
            return [rows, h('button', {
                type: 'button', class: 'cydc-chip', 'data-cc-focus': 'event:add', disabled: !free.length,
                on: { click: () => { const list = listOf(); list.push({ on: free[0], action: { type: 'logic', hint: '' } }); dropEventDrafts(); save(list, true); } }
            }, icon('plus'), t.or('cyd_creator.designer.event_add', 'Add event'))];
        }

        // createHook asks the code editor for on_<name>_<event>(); ensureHook reports its own
        // failures and switches to Code mode, only an invalid name throws. The button stays disabled
        // while the request runs (a second click would write the stub twice).
        async function createHook(id, event) {
            const w = current(id);
            const key = id + ':' + event;
            if (!w || !M.validName(w.name) || !hookReady() || ui.hooking.has(key)) return;
            ui.hooking.add(key);
            syncHooks();
            try {
                await d.inst.code.ensureHook({ name: w.name, event });
            } catch (err) {
                notify(P.errors.message(t, err));
            } finally {
                ui.hooking.delete(key);
                if (!ui.disposed) syncHooks();
            }
        }

        // upload sends the image (an SVG rasterised to PNG at the widget's size, at most the screen's)
        // through the shell's inst.uploadAsset: it saves pending edits, the server lists the asset in
        // the layout and the window re-reads it (a remote replace) before it answers. Then the
        // widget's prop is set (an image also takes the asset's size); the asset is added only when
        // it is missing, which happens when an edit made during the upload kept the window's copy
        // (the If-Match conflict choice follows on the next save). A file the server would refuse
        // (not PNG/JPEG/SVG, more than 2 MiB) is refused here with the reason.
        async function upload(id, file) {
            if (!file || d.readonly() || typeof d.inst.uploadAsset !== 'function' || ui.uploading.has(id)) return;
            ui.uploading.add(id);
            render();
            try {
                const kind = await P.designerFields.imageKind(file);
                if (!kind) { notify(t.or('cyd_creator.designer.asset_type', 'Only PNG, JPEG and SVG images can be uploaded.')); return; }
                const w = current(id);
                const screen = d.screenSize();
                const side = (v, max) => Math.min(P.designerFields.clampInt(v, 1, MAX_SIDE) || 64, Math.max(1, max || MAX_SIDE));
                const png = kind === 'svg' ? await P.designerFields.rasterizeSVG(file, side(w && w.w, screen.w), side(w && w.h, screen.h)) : file;
                if (!(png.size <= P.designerFields.MAX_UPLOAD_BYTES)) {
                    notify(t.or('cyd_creator.designer.asset_too_large', 'The image is larger than {max} MiB.', { max: P.designerFields.MAX_UPLOAD_BYTES / (1 << 20) }));
                    return;
                }
                const asset = await d.inst.uploadAsset(png);
                // The widget (or the window) is gone: nothing to set.
                if (ui.disposed || d.disposed || !current(id)) return;
                if (!asset || !asset.id) { notify(P.errors.message(t, null)); return; }
                if (d.readonly()) { notify(t.or('cyd_creator.designer.asset_not_applied', 'The image was uploaded but not set: the project became read-only or locked.')); return; }
                const cur = current(id);
                const set = { props: { [cur.type === 'button' ? 'icon_asset' : 'asset']: asset.id, icon_frames: null, icon_animate: null, icon_origin: null } };
                if (cur.type === 'image' && Number.isInteger(asset.w) && Number.isInteger(asset.h) && asset.w > 0 && asset.h > 0) { set.w = asset.w; set.h = asset.h; }
                d.commands.transact('asset', (draft, apply) => {
                    if (!Array.isArray(draft.assets)) draft.assets = [];
                    // Only the fields of internal/cydcreator Asset (the server decodes layouts strictly).
                    if (!draft.assets.some(a => a.id === asset.id)) draft.assets.push({ id: asset.id, file: asset.file, w: asset.w, h: asset.h, alpha: !!asset.alpha });
                    apply([{ op: 'update_widget', id, set }]);
                });
                d.announce(t.or('cyd_creator.designer.asset_uploaded', 'Image uploaded'));
            } catch (err) {
                notify(P.errors.message(t, err));
            } finally {
                ui.uploading.delete(id);
                if (!ui.disposed && !d.disposed) render();
            }
        }

        function widgetPanel(f, ro) {
            const w = f.widget;
            const spec = d.spec(w.type);
            return h('div', { class: 'cydc-insp', 'data-cc-inspect': w.id },
                h('div', { class: 'cydc-insp-head' },
                    h('span', { class: 'cydc-insp-glyph', 'aria-hidden': 'true' }, (P.designerPalette && P.designerPalette.GLYPHS[w.type]) || '◆'),
                    h('h3', {}, widgetLabel(w.type)), h('code', { class: 'cydc-insp-id' }, str(w.id))),
                issueBox(),
                h('fieldset', { class: 'cydc-fieldset', disabled: ro },
                    section(t.or('cyd_creator.designer.section_name', 'Name'), nameField(w)),
                    section(t.or('cyd_creator.designer.section_geometry', 'Position and size'), geometry(f)),
                    spec && (spec.props || []).length ? section(t.or('cyd_creator.designer.section_props', 'Properties'), spec.props.filter(ps => !['icon_frames', 'icon_motion', 'icon_origin'].includes(ps.name)).map(ps => propField(w, spec, ps))) : null,
                    section(t.or('cyd_creator.designer.section_style', 'Appearance'), styleFields(w)),
                    section(t.or('cyd_creator.designer.section_events', 'Events'), eventsEditor(w, spec))));
        }

        function multiPanel(ids) {
            return h('div', { class: 'cydc-insp', 'data-cc-inspect': 'multi' },
                h('div', { class: 'cydc-insp-head' }, h('h3', {}, t.or('cyd_creator.designer.multi_title', '{count} widgets selected', { count: ids.length }))),
                h('ul', { class: 'cydc-multi-list', role: 'list' }, ids.map(id => { const w = current(id); return w ? h('li', {}, h('span', {}, str(w.name) || widgetLabel(w.type)), h('span', { class: 'cydc-layer-type' }, widgetLabel(w.type))) : null; })),
                h('p', { class: 'cydc-panel-hint' }, t.or('cyd_creator.designer.multi_hint', 'Use the toolbar above the screen to align, distribute, lock or delete them.')));
        }

        function render() {
            if (ui.disposed) return;
            // Rebuilding the field an input method composes in would end the composition: wait.
            if (ui.composing) { ui.deferred = true; return; }
            ui.deferred = false;
            const ro = d.readonly();
            const active = document.activeElement;
            const hadFocus = !!active && active !== root && root.contains(active);
            const one = d.selection.length === 1 ? d.find(d.selection[0]) : null;
            const panel = d.isSystem() ? 'system' : one ? 'w:' + one.widget.id : d.selection.length > 1 ? 'multi' : 'project';
            if (panel !== ui.panel) { drafts.clear(); ui.panel = panel; }
            live = { issues: null, hooks: [] };
            ui.issuesOf = null;
            let node;
            if (panel === 'system') {
                node = h('div', { class: 'cydc-insp', 'data-cc-inspect': 'system' }, h('div', { class: 'cydc-insp-head' }, h('h3', {}, t.or('cyd_creator.designer.system_wifi', 'Wi-Fi setup'))),
                    h('p', { class: 'cydc-panel-hint' }, t.or('cyd_creator.designer.system_hint', 'This screen comes from the template and appears while Wi-Fi still needs to be set up. It follows the theme and cannot be edited.')));
            } else if (one) {
                const id = one.widget.id;
                ui.issuesOf = () => { const cur = current(id); return cur ? d.issuesFor(cur) : []; };
                node = widgetPanel(one, ro);
            } else if (panel === 'multi') {
                node = multiPanel(d.selection);
            } else {
                ui.issuesOf = () => d.issues.filter(i => !i.widget && !i.screen);
                node = projectPanel(ro);
            }
            ui.issueKey = ui.issuesOf ? issueKey(ui.issuesOf()) : '';
            swap(root, node);
            root.classList.toggle('is-readonly', ro);
            // A field disabled by the lock (or one that is gone) cannot keep the focus: it goes to the
            // panel, never to the page (the desktop acts on its selected icons from the page's keys).
            if (hadFocus && (ro || !root.contains(document.activeElement))) root.focus({ preventScroll: true });
        }

        bag.add(d.bus.on('project', info => {
            const i = info || {};
            // 'screen' rendered already; an own edit keeps the panel (the caret stays where it is).
            if (i.screenChanged || (i.origin === 'local' && d.inspectorEditing)) return;
            render();
        }));
        ['selection', 'screen', 'lock'].forEach(type => bag.add(d.bus.on(type, render)));
        bag.add(d.bus.on('issues', refreshIssues));
        // Enter, Delete and F2 on a button here must not reach the desktop (it would open, delete or
        // rename its selected icon); Ctrl/Cmd shortcuts (Ctrl+S) still reach the window.
        bag.on(root, 'keydown', event => {
            if (!event.ctrlKey && !event.metaKey && !event.altKey && (event.key === 'Enter' || event.key === 'Delete' || event.key === 'F2')) event.stopPropagation();
        });
        bag.on(root, 'compositionstart', () => { ui.composing = true; });
        bag.on(root, 'compositionend', () => { ui.composing = false; if (ui.deferred) render(); });
        render();
        return { root, render, dispose() { ui.disposed = true; d.setPreviewTheme(''); bag.dispose(); root.remove(); } };
    }

    P.designerInspector = { mount, TIMEZONES, LIMITS };
})();
