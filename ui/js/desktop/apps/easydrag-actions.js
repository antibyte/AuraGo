// EasyDrag action registry. One table of every action (label, icon, shortcut, risk and the shared checks)
// and the logic the owning modules bind to it. Menus, card tools, header buttons, the shortcut table and
// the keyboard list action ids and read this module, so an action has one name, one shortcut and one
// reason for being unavailable on every surface.
//
// A host is the object an action works on: the editor state (ed), the start page or the shell instance.
// The host carries t (the translator), readonly, and optionally actionFacts(): extra facts of the moment.
// A context is { host, ...extras }: extras are flow (the card's flow on the start page), node (the card a
// card menu or tool belongs to), x and y (a point), keyed (window menu entries carry label keys), cached (read the
// host's facts once per turn: a render of many cards), and for keys the facts of the keystroke (see key).
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const MARK = String.fromCodePoint(0x2713);
    const MINUS = String.fromCodePoint(0x2212);
    const DASH = String.fromCodePoint(0x2013);
    const RISKS = ['safe', 'undoable', 'confirm'];
    const SCOPES = ['editor', 'canvas'];

    // Row: [id, label key, menu icon, glyph (core.ICONS name for in-app icon buttons, or ''), shortcut, scope,
    // risk, shared checks, legacy window-menu id (or '')]. The label keys are literals on purpose: the locale
    // tests find literal keys.
    const TABLE = [
        ['flow.home', 'easydrag.ui.back_home', 'list', 'arrow-left', '', 'editor', 'safe', [], 'home'],
        ['flow.save', 'easydrag.ui.save_now', 'save', '', 'Ctrl+S', 'editor', 'safe', ['writable', 'free'], 'save'],
        ['flow.test', 'easydrag.ui.test_draft', 'flask', '', 'Ctrl+Enter', 'editor', 'safe', ['writable', 'free', 'idle'], 'test'],
        ['flow.run_now', 'easydrag.ui.run_now', 'play', '', '', 'editor', 'safe', ['writable', 'free', 'published', 'active'], 'run'],
        ['flow.stop', 'easydrag.ui.run_cancel', 'stop', '', '', 'editor', 'safe', [], ''],
        ['flow.publish', 'easydrag.ui.publish', 'upload', '', '', 'editor', 'safe', ['writable', 'free'], 'publish'],
        ['flow.active', 'easydrag.ui.active', 'check', '', '', 'editor', 'safe', ['writable'], ''],
        ['flow.versions', 'easydrag.ui.versions_title', 'history', '', '', 'editor', 'safe', ['free'], 'versions'],
        ['flow.settings', 'easydrag.ui.flow_settings', 'settings', '', '', 'editor', 'safe', ['free'], 'settings'],
        ['flow.organize', 'easydrag.ui.organize', 'folder', '', '', 'editor', 'safe', ['writable', 'free'], ''],
        ['flow.describe', 'easydrag.ui.authoring_title', 'sparkles', '', '', 'editor', 'safe', ['writable', 'free', 'idle'], ''],
        ['flow.repair_mission', 'easydrag.ui.repair_mission', 'link', '', '', 'editor', 'safe', ['writable', 'free'], ''],
        ['flow.duplicate', 'easydrag.ui.home_duplicate', 'copy', '', '', 'editor', 'safe', ['unlocked'], ''],
        ['flow.export', 'easydrag.ui.home_export', 'download', '', '', 'editor', 'safe', [], 'export'],
        ['flow.export_pack', 'easydrag.ui.pack_export', 'download', '', '', 'editor', 'safe', ['free'], 'export-pack'],
        ['flow.export_email', 'easydrag.ui.export_email', 'mail', '', '', 'editor', 'safe', [], ''],
        ['flow.mission_control', 'easydrag.ui.home_mission_control', 'list', '', '', 'editor', 'safe', [], ''],
        ['flow.delete', 'easydrag.ui.home_delete', 'trash', '', '', 'editor', 'confirm', ['unlocked', 'free'], 'delete'],
        ['flow.open', 'easydrag.ui.home_open', 'edit', '', '', 'editor', 'safe', [], ''],
        ['flow.open_draft', 'easydrag.ui.block_open_draft', 'pencil', '', '', 'editor', 'safe', [], ''],
        ['flow.new', 'easydrag.ui.home_new', 'plus', '', '', 'editor', 'safe', ['unlocked'], 'new'],
        ['flow.import', 'easydrag.ui.home_import', 'upload', '', '', 'editor', 'safe', ['unlocked'], 'import'],
        ['flow.reload', 'easydrag.ui.reload', 'refresh', '', '', 'editor', 'safe', [], 'reload'],
        ['edit.undo', 'easydrag.ui.undo', 'undo', 'undo', 'Ctrl+Z', 'canvas', 'safe', ['writable', 'free', 'quiet'], 'undo'],
        ['edit.redo', 'easydrag.ui.redo', 'redo', 'redo', 'Ctrl+Shift+Z', 'canvas', 'safe', ['writable', 'free', 'quiet'], 'redo'],
        ['edit.cut', 'easydrag.ui.cut', 'scissors', '', 'Ctrl+X', 'canvas', 'undoable', ['writable', 'free', 'quiet', 'selection'], 'cut'],
        ['edit.copy', 'easydrag.ui.copy', 'copy', '', 'Ctrl+C', 'canvas', 'safe', ['free', 'quiet', 'selection'], 'copy'],
        ['edit.paste', 'easydrag.ui.paste', 'clipboard', '', 'Ctrl+V', 'canvas', 'safe', ['writable', 'free', 'quiet'], 'paste'],
        ['edit.duplicate', 'easydrag.ui.duplicate', 'copy', 'copy', 'Ctrl+D', 'canvas', 'safe', ['writable', 'free', 'quiet', 'selection'], 'duplicate'],
        ['edit.delete', 'easydrag.ui.delete', 'trash', 'trash', 'Delete', 'canvas', 'undoable', ['writable', 'free', 'quiet'], 'delete-sel'],
        ['edit.select_all', 'easydrag.ui.select_all', 'check-square', '', 'Ctrl+A', 'canvas', 'safe', ['free', 'quiet'], 'select-all'],
        ['edit.extract', 'easydrag.ui.extraction_title', 'grid', '', '', 'canvas', 'safe', ['writable', 'free', 'quiet', 'selection'], 'extract-selection'],
        ['edit.auto_layout', 'easydrag.ui.auto_layout', 'sparkles', '', 'Alt+Shift+F', 'editor', 'safe', ['writable', 'free', 'steps2'], 'auto-layout'],
        ['edit.group', 'easydrag.ui.group_create', 'grid', '', '', 'canvas', 'safe', ['writable'], ''],
        ['edit.add_step', 'easydrag.ui.menu_add_node', 'plus', '', '', 'canvas', 'safe', ['writable'], ''],
        ['step.open', 'easydrag.ui.menu_open_node', 'edit', '', 'Enter', 'canvas', 'safe', [], ''],
        ['step.test', 'easydrag.ui.node_test', 'play', 'flask', '', 'canvas', 'safe', ['writable', 'idle', 'selection'], ''],
        ['step.toggle', 'easydrag.ui.keys_disable', 'pause', 'eye-off', 'D', 'canvas', 'safe', ['writable', 'selection'], ''],
        ['step.connect', 'easydrag.ui.keys_connect', 'link', 'link', 'C', 'canvas', 'safe', ['writable', 'selection'], ''],
        ['step.add_next', 'easydrag.ui.port_add', 'plus', 'plus', '', 'canvas', 'safe', ['writable'], ''],
        ['step.align_horizontal', 'easydrag.ui.align_horizontal', 'sliders', '', '', 'canvas', 'safe', ['writable'], ''],
        ['step.align_vertical', 'easydrag.ui.align_vertical', 'sliders', '', '', 'canvas', 'safe', ['writable'], ''],
        ['view.zoom_in', 'easydrag.ui.zoom_in', 'zoom-in', '', '+', 'canvas', 'safe', [], 'zoom-in'],
        ['view.zoom_out', 'easydrag.ui.zoom_out', 'zoom-out', '', '-', 'canvas', 'safe', [], 'zoom-out'],
        ['view.zoom_fit', 'easydrag.ui.zoom_fit', 'maximize', '', 'Shift+1', 'canvas', 'safe', [], 'zoom-fit'],
        ['view.zoom_reset', 'easydrag.ui.keys_zoom_reset', 'maximize', '', 'Ctrl+0', 'canvas', 'safe', [], ''],
        ['view.find_step', 'easydrag.ui.find_step', 'search', '', 'Ctrl+F', 'editor', 'safe', ['free'], 'find-step'],
        ['view.search', 'easydrag.ui.keys_search', 'search', '', 'Ctrl+K', 'editor', 'safe', ['free'], 'search'],
        ['view.palette', 'easydrag.ui.palette_title', 'sidebar', '', '', 'editor', 'safe', [], 'palette'],
        ['view.runs', 'easydrag.ui.runs_title', 'history', '', '', 'editor', 'safe', [], 'runs'],
        ['view.shortcuts', 'easydrag.ui.keys_title', 'help', '', '?', 'canvas', 'safe', ['free'], 'keys'],
        ['view.liveliness', 'easydrag.ui.liveliness', 'sparkles', '', '', 'editor', 'safe', [], ''],
        ['view.liveliness_normal', 'easydrag.ui.liveliness_normal', 'play', '', '', 'editor', 'safe', [], 'liveliness-normal'],
        ['view.liveliness_calm', 'easydrag.ui.liveliness_calm', 'pause', '', '', 'editor', 'safe', [], 'liveliness-calm']
    ];

    const ROWS = Object.freeze(TABLE.map(([id, label, icon, glyph, shortcut, scope, risk, needs, menu]) =>
        Object.freeze({ id, label, icon, glyph, shortcut, scope, risk, needs: Object.freeze(needs.slice()), menu })));
    const BY_ID = new Map(ROWS.map(row => [row.id, row]));

    // REASONS: each cause of a shared check has one reason key, so the text is the same on every surface.
    const REASONS = Object.freeze({
        run_view: 'easydrag.ui.reason_run_view',
        readonly: 'easydrag.ui.reason_readonly',
        locked: 'easydrag.ui.reason_locked',
        dialog: 'easydrag.ui.reason_dialog_open',
        restoring: 'easydrag.ui.reason_restoring',
        selection: 'easydrag.ui.reason_no_selection',
        published: 'easydrag.ui.reason_not_published',
        paused: 'easydrag.ui.reason_paused',
        running: 'easydrag.ui.reason_running',
        steps2: 'easydrag.ui.reason_two_steps'
    });

    // The shared checks. Each reads the facts of the host (see facts) and the context, and returns a reason key
    // or ''. writable: nothing may be changed (a past run is open, read-only mode, the flows are locked).
    // unlocked: the same without the run view, for actions on the whole flow that a past run does not stop.
    // free: no dialog, context menu or restore offer is open. quiet: quick-add is closed (edits wait for it). selection: a step is selected (or the context names one).
    const CHECKS = Object.freeze({
        writable: f => (f.runView ? REASONS.run_view : f.readonly ? REASONS.readonly : f.locked ? REASONS.locked : ''),
        unlocked: f => (f.readonly ? REASONS.readonly : f.locked ? REASONS.locked : ''),
        free: f => (f.dialog ? REASONS.dialog : f.restoring ? REASONS.restoring : ''),
        quiet: f => (f.quickAdd ? REASONS.dialog : ''),
        selection: (f, c) => (c.node || f.selection > 0 ? '' : REASONS.selection),
        published: (f, c) => ((c.flow ? c.flow.published : f.published) ? '' : REASONS.published),
        active: (f, c) => {
            const published = c.flow ? c.flow.published : f.published;
            const enabled = c.flow ? c.flow.enabled : f.enabled;
            return published && !enabled ? REASONS.paused : '';
        },
        idle: f => (f.running ? REASONS.running : ''),
        steps2: f => (f.steps >= 2 ? '' : REASONS.steps2)
    });

    // Shell-dispatched shortcuts: the desktop's window-menu dispatch fires them (the menu item carries
    // `shortcut`); the desktop's own search would take Ctrl+K otherwise. Every other shortcut is only displayed
    // by the menus (`shortcutHint`) and fired by key().
    const DESKTOP_KEYS = new Set(['flow.save', 'flow.test', 'view.search']);
    // These work from a text field as well (they never type a character). Every other key needs the focus
    // outside a text field.
    const TEXT_KEYS = new Set(['flow.save', 'flow.test', 'view.search', 'view.find_step']);
    // Ctrl+C with nothing selected is not taken from the browser (its own copy goes on).
    const PASS_WHEN_REFUSED = new Set(['edit.copy']);
    const ALIASES = [['edit.redo', 'Ctrl+Y']];
    const SUBMENUS = { 'view.liveliness': ['view.liveliness_normal', 'view.liveliness_calm'] };
    const MARKED = new Set(['view.liveliness_normal', 'view.liveliness_calm']);

    // TOOLS: the card tools in the order the card draws them: [data-ed-node-tool value, action id].
    const TOOLS = Object.freeze([['add', 'step.add_next'], ['connect', 'step.connect'], ['test', 'step.test'], ['disable', 'step.toggle'], ['duplicate', 'edit.duplicate'], ['delete', 'edit.delete']]);

    const hosts = new WeakMap();
    const globals = new Map();
    let observer = null;

    // bind attaches what an action does: impl is { run(ctx), check(ctx), label(ctx), show(ctx), checked(ctx),
    // accepts(event, where) }. check returns a reason key or nothing, label an other label key or nothing, show
    // false leaves the action out of menus and toolbars, checked draws a check mark, accepts false lets a key
    // pass to the next handler. Without a host the binding is global (the same for every host).
    function bind(id, impl, host) {
        if (!BY_ID.has(id)) throw new Error('unknown action ' + id);
        if (!host) { globals.set(id, impl); return; }
        let map = hosts.get(host);
        if (!map) { map = new Map(); hosts.set(host, map); }
        map.set(id, impl);
    }

    function implOf(host, id) {
        const map = host && hosts.get(host);
        return (map && map.get(id)) || globals.get(id) || null;
    }

    function row(id) { return BY_ID.get(id) || null; }

    // facts are read fresh, except for a caller that draws many buttons in one go (ctx.cached): they are kept for the turn of the
    // event loop. run() and key() never use the kept ones.
    const kept = new WeakMap();
    let epoch = 0;
    let ticking = false;

    // refresh drops the facts kept for host: the next cached read is fresh. A caller that paints many buttons in one go calls it
    // first, so facts kept earlier in the same turn (a card drawn while quick-add was still open) are not reused.
    function refresh(host) { if (host) kept.delete(host); }

    function facts(host, cached) {
        const old = cached && host ? kept.get(host) : null;
        if (old && old.epoch === epoch) return old.value;
        const value = readFacts(host);
        if (cached && host) {
            kept.set(host, { epoch, value });
            if (!ticking) { ticking = true; Promise.resolve().then(() => { epoch++; ticking = false; }); }
        }
        return value;
    }

    // readFacts reads what the shared checks need from a host. The defaults come from the editor state; a host
    // overrides or adds with actionFacts() (restoring, dialog, running, locked ...).
    function readFacts(host) {
        const h = host || {};
        const nodes = h.model && h.model.doc && h.model.doc.nodes;
        const base = {
            readonly: !!h.readonly, runView: !!h.runView, locked: false, restoring: false, dialog: false, quickAdd: !!h.quickAdd, running: false,
            selection: h.selection ? h.selection.size : 0, steps: nodes ? nodes.length : 0,
            published: !!(h.flow && h.flow.live), enabled: !!h.flowEnabled
        };
        return typeof h.actionFacts === 'function' ? Object.assign(base, h.actionFacts()) : base;
    }

    function translator(c) { return c.t || (c.host && c.host.t) || (key => key); }

    function reasonOf(c, key) { return key ? { enabled: false, reason: translator(c)(key) } : null; }

    // state says whether an action may run now, and if not, why (a translated text).
    function state(id, ctx) {
        const c = ctx || {};
        const r = BY_ID.get(id);
        const impl = r && implOf(c.host, id);
        if (!impl) return { enabled: false, reason: '' };
        const f = facts(c.host, !!c.cached);
        for (const need of r.needs) {
            const refused = reasonOf(c, CHECKS[need](f, c));
            if (refused) return refused;
        }
        return reasonOf(c, impl.check ? impl.check(c) : '') || { enabled: true, reason: '' };
    }

    // run checks the state again and runs the action only when it is enabled. It returns false when the
    // action is refused, and what the action returns otherwise.
    function run(id, ctx) {
        const c = ctx || {};
        if (!state(id, c).enabled) return false;
        if (observer) observer(id);
        return implOf(c.host, id).run(c);
    }

    // observe lets a test count the runs; fn(id) is called right before an enabled action runs.
    function observe(fn) { observer = typeof fn === 'function' ? fn : null; return () => { if (observer === fn) observer = null; }; }

    function labelKey(id, c) {
        const impl = implOf(c.host, id);
        return (impl && impl.label && impl.label(c)) || BY_ID.get(id).label;
    }

    function label(id, ctx) {
        const c = ctx || {};
        return translator(c)(labelKey(id, c));
    }

    // display is the shortcut as the surfaces draw it: the platform's modifier symbols, the localized name of Delete.
    function display(shortcut, t) {
        if (!shortcut) return '';
        if (shortcut === 'Delete') return t ? t('easydrag.ui.key_delete') : 'Del';
        if (shortcut === '-') return MINUS;
        return ED.core ? ED.core.shortcut(shortcut) : shortcut;
    }

    function shown(id, impl, c) { return !!impl && !(impl.show && !impl.show(c)); }

    // named gives a menu entry its words: the translated label, or with ctx.keyed the label key, which the desktop
    // translates when it draws the menu (window menus follow a language change that way).
    function named(entry, id, c) {
        if (c.keyed) entry.labelKey = labelKey(id, c); else entry.label = label(id, c);
        return entry;
    }

    function entryOf(id, c) {
        const r = BY_ID.get(id);
        if (!r) throw new Error('unknown action ' + id);
        if (SUBMENUS[id]) {
            const kids = items(SUBMENUS[id], c);
            return kids.length ? named({ id: id.replace('.', '-'), icon: r.icon, items: kids }, id, c) : null;
        }
        const impl = implOf(c.host, id);
        if (!shown(id, impl, c)) return null;
        const st = state(id, c);
        const keys = display(r.shortcut, translator(c));
        const entry = named({ id: r.menu || r.id, icon: r.icon, disabled: !st.enabled, action: () => run(id, c) }, id, c);
        if (!st.enabled && st.reason) entry.disabledHint = st.reason;
        if (keys) { if (DESKTOP_KEYS.has(id)) entry.shortcut = r.shortcut; else entry.shortcutHint = keys; }
        if (r.risk !== 'safe') entry.danger = true;
        if (impl.checked) {
            entry.checked = !!impl.checked(c);
            if (MARKED.has(id)) entry.shortcutHint = entry.checked ? MARK : '';
        }
        return entry;
    }

    // items turns action ids into desktop menu entries (showContextMenu and setWindowMenus format; ctx.keyed: labelKey
    // instead of label, for window menus). '-' is a
    // separator; an action nobody bound here, or one that says show() is false, is left out, and a separator
    // never leads, trails or doubles.
    function items(ids, ctx) {
        const c = ctx || {};
        const out = [];
        ids.forEach(id => {
            if (id === '-') {
                if (out.length && !out[out.length - 1].separator) out.push({ separator: true, type: 'separator' });
                return;
            }
            const entry = entryOf(id, c);
            if (entry) out.push(entry);
        });
        while (out.length && out[out.length - 1].separator) out.pop();
        return out;
    }

    // button returns the attributes of an icon button: aria-label, a title of the label and the shortcut, and
    // when the action is unavailable aria-disabled="true" with the reason in the title and in aria-description
    // (an aria-disabled button stays focusable, so a keyboard user learns why). null: not bound here.
    function button(id, ctx) {
        const c = ctx || {};
        const r = BY_ID.get(id);
        const impl = r && implOf(c.host, id);
        if (!shown(id, impl, c)) return null;
        const st = state(id, c);
        const name = label(id, c);
        const keys = display(r.shortcut, translator(c));
        const attrs = { 'aria-label': name, title: name + (keys ? ' (' + keys + ')' : '') };
        if (!st.enabled) {
            attrs['aria-disabled'] = 'true';
            if (st.reason) { attrs.title += ' ' + DASH + ' ' + st.reason; attrs['aria-description'] = st.reason; }
        }
        return attrs;
    }

    // paint writes button(id, ctx) onto an element. The accessible name is written only with opts.name (an icon
    // button); a button with visible text keeps its text as its name.
    function paint(el, id, ctx, opts) {
        const attrs = button(id, ctx);
        if (!el || !attrs) return false;
        Object.keys(attrs).forEach(name => { if (name !== 'aria-label' || (opts && opts.name)) el.setAttribute(name, attrs[name]); });
        if (!attrs['aria-disabled']) { el.removeAttribute('aria-disabled'); el.removeAttribute('aria-description'); }
        return true;
    }

    // attrString renders an attribute object as markup (for modules that build innerHTML).
    function attrString(attrs) {
        return Object.keys(attrs).map(name => ' ' + name + '="' + ED.core.esc(attrs[name]) + '"').join('');
    }

    // parse reads a canonical shortcut ("Ctrl+Shift+Z", "Shift+1", "+", "D") into modifiers and a key.
    function parse(shortcut) {
        const m = { ctrl: false, shift: false, alt: false, key: '' };
        let rest = shortcut;
        for (;;) {
            if (rest.startsWith('Ctrl+')) { m.ctrl = true; rest = rest.slice(5); }
            else if (rest.startsWith('Shift+')) { m.shift = true; rest = rest.slice(6); }
            else if (rest.startsWith('Alt+')) { m.alt = true; rest = rest.slice(4); }
            else break;
        }
        m.key = rest.toLowerCase();
        return m;
    }

    // The keystrokes, most specific first (Ctrl+Shift+Z before Ctrl+Z).
    const KEYS = ROWS.filter(r => r.shortcut).map(r => ({ row: r, m: parse(r.shortcut) }))
        .concat(ALIASES.map(([id, shortcut]) => ({ row: BY_ID.get(id), m: parse(shortcut) })))
        .sort((a, b) => (b.m.shift + b.m.alt) - (a.m.shift + a.m.alt));

    // matches compares a keydown with a parsed shortcut. A modifier the shortcut does not name is ignored,
    // except Ctrl (the platform's Cmd or Ctrl, ED.core.isMod): a plain key never fires with it held.
    function matches(m, event) {
        if (m.ctrl !== ED.core.isMod(event)) return false;
        if (m.shift && !event.shiftKey) return false;
        if (m.alt && !event.altKey) return false;
        const key = String(event.key).toLowerCase();
        if (m.key === '+') return key === '+' || key === '=';
        if (m.key === '1' && m.shift) return key === '1' || key === '!';
        if (m.key === 'delete') return key === 'delete' || key === 'backspace';
        return key === m.key;
    }

    // key is the one keyboard path for registry shortcuts. where: { host, editable (the target is a text field),
    // quickAdd, onCanvas (the focus is on the canvas, a card or the page), control (the target is a button, link or
    // field inside the canvas) }. It returns true when it took the keystroke: the browser's default is prevented
    // and the action ran once, or was refused with its reason. The shell fires its own shortcuts first and prevents
    // them; the editor does not call key for a prevented event.
    function key(event, where) {
        const w = where || {};
        if (!event || typeof event.key !== 'string') return false;
        for (const entry of KEYS) {
            if (!matches(entry.m, event)) continue;
            const r = entry.row;
            const impl = implOf(w.host, r.id);
            if (!impl) continue;
            if (r.scope === 'editor' ? w.editable && !TEXT_KEYS.has(r.id) : w.editable || w.quickAdd || !w.onCanvas) continue;
            if (impl.accepts && !impl.accepts(event, w)) continue;
            if (!state(r.id, w).enabled && PASS_WHEN_REFUSED.has(r.id)) return false;
            event.preventDefault();
            run(r.id, w);
            return true;
        }
        return false;
    }

    // table builds the shortcut table: layout is [[group key, [action id or [shortcut, label key]]]]. A literal
    // pair is a gesture that is no action (arrows, Space, Esc). Actions without a shortcut are left out.
    function table(layout, ctx) {
        const c = ctx || {};
        return layout.map(([group, entries]) => [group, entries.map(entry => {
            if (Array.isArray(entry)) return entry;
            const r = BY_ID.get(entry);
            return r && r.shortcut ? [r.shortcut, labelKey(entry, c)] : null;
        }).filter(Boolean)]);
    }

    ED.actions = { ROWS, REASONS, CHECKS, TOOLS, bind, state, run, items, button, paint, attrString, key, row, label, display, table, observe, refresh };
})();
