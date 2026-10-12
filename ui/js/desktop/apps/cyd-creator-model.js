// CYD Creator designer model: the layout store (frozen snapshots, undo/redo with coalescing),
// layout ops that mirror the structural part of Go ApplyOps (internal/cydcreator/patch.go), tree
// lookups, name rules and proposals. Pure: no DOM; P.util (core module) supplies deepClone,
// deepFreeze and uid. Unlike ApplyOps, apply() has no validation gate: interactive edits may leave
// the layout temporarily invalid; the server validates on save and the shell reports the issues
// (layout:issues). Only structurally impossible ops (unknown target, parent that cannot hold
// widgets, moving into itself, wrong value types) throw.
(function () {
    'use strict';

    const P = window.CYDCreatorParts = window.CYDCreatorParts || {};

    const NAME_RE = /^[a-z][a-z0-9_]{0,31}$/;
    // Mirrors validate.go: widget names become ui_<name>, on_<name>_<event>, ui_show_/ui_hide_<name>
    // and ui_<name>_series, so these names would collide with generated symbols.
    const RESERVED_NAMES = ['init', 'goto', 'back', 'theme_colors', 'event_index', 'async', 'screen_t', 'transition_t'];
    const RESERVED_PREFIXES = ['scr_', 'show_', 'hide_', 'font_'];
    const RESERVED_SUFFIXES = ['_series'];
    const LIMITS = { screens: 16, widgetsPerScreen: 200, nesting: 4, textBytes: 1024, hintRunes: 500 };
    const TRANSITIONS = ['none', 'fade', 'slide_left', 'slide_right', 'slide_up', 'slide_down'];
    const ACTIONS = ['goto_screen', 'back', 'open_wifi_setup', 'logic'];
    const ORIENTATIONS = ['landscape', 'portrait', 'landscape_flipped', 'portrait_flipped'];
    const WIFI_MODES = ['off', 'optional', 'required'];
    const STYLE_KEYS = ['bg_color', 'bg_opacity', 'text_color', 'accent_color', 'border_color', 'border_width', 'radius'];
    const PROJECT_KEYS = ['orientation', 'language', 'timezone', 'start_screen'];
    const PREFIX = {
        label: 'lbl', button: 'btn', image: 'img', panel: 'pnl', line: 'line', switch: 'sw', checkbox: 'chk',
        slider: 'sld', arc: 'arc', dropdown: 'dd', roller: 'rol', textarea: 'ta', spinbox: 'spn', keypad: 'kp',
        bar: 'bar', spinner: 'spin', led: 'led', chart: 'chart', gauge: 'gauge', table: 'tbl', qrcode: 'qr',
        tabview: 'tabs', list: 'list', msgbox: 'msg', canvas: 'cnv', statusbar: 'status'
    };

    const clone = v => P.util.deepClone(v);
    function freeze(v) { P.util.deepFreeze(v); return v; }

    // ModelError mirrors the Go *Error CYD_INVALID_LAYOUT with details {index, op, reason}.
    class ModelError extends Error {
        constructor(index, op, reason) {
            super('layout op ' + index + ' (' + op + ') failed: ' + reason);
            this.name = 'ModelError';
            this.code = 'CYD_INVALID_LAYOUT';
            this.details = { index, op, reason };
        }
    }

    // ── store ───────────────────────────────────────────────────────────────

    // HISTORY_MAX_BYTES caps the undo history by the summed JSON length of its snapshots (a frozen
    // snapshot takes about 2.5 times its JSON length on the heap): typical layouts keep all `limit`
    // steps, a layout near the 1 MiB server maximum keeps about 30.
    const HISTORY_MAX_BYTES = 16 * 1024 * 1024;

    // createStore keeps the layout as frozen snapshots. change() runs fn on a deep clone; a thrown
    // error leaves everything unchanged, a change without effect records nothing. A change with the
    // same coalesce key as the newest undo step, made within coalesceMs of the previous one, joins
    // that step. undo/redo/replace end coalescing. The history keeps at most `limit` undo steps
    // whose snapshots sum to at most `maxBytes` JSON bytes (the newest step always stays); only
    // change() adds snapshots, undo/redo move them between the two stacks.
    function createStore(project, opts) {
        const o = opts || {};
        const limit = o.limit || 300;
        const maxBytes = o.maxBytes || HISTORY_MAX_BYTES;
        const coalesceMs = o.coalesceMs == null ? 1200 : o.coalesceMs;
        const now = typeof o.now === 'function' ? o.now : () => Date.now();
        const subs = new Set();
        const past = [];
        const future = [];
        let current = freeze(clone(project));
        // currentJson caches JSON.stringify(current) for the no-op check (null = not computed since
        // an undo/redo); currentBytes is its length, the byte weight current gets in a history stack.
        let currentJson = JSON.stringify(current);
        let currentBytes = currentJson.length;
        let pastBytes = 0;
        let barrier = true;

        function emit(label, origin) {
            Array.from(subs).forEach(fn => {
                try { fn(current, { label, origin }); } catch (err) { console.error('cyd-creator store subscriber', err); }
            });
        }
        return {
            get: () => current,
            change(label, fn, copts) {
                const draft = clone(current);
                const result = fn(draft);
                const json = JSON.stringify(draft);
                if (currentJson === null) currentJson = JSON.stringify(current);
                if (json === currentJson) return result;
                const key = (copts && copts.coalesce) || null;
                const at = now();
                const top = past[past.length - 1];
                if (key && !barrier && top && top.coalesce === key && at - top.at < coalesceMs) {
                    top.at = at;
                } else {
                    past.push({ snapshot: current, bytes: currentBytes, label, coalesce: key, at });
                    pastBytes += currentBytes;
                    while (past.length > 1 && (past.length > limit || pastBytes > maxBytes)) pastBytes -= past.shift().bytes;
                }
                barrier = false;
                future.length = 0;
                current = freeze(draft);
                currentJson = json;
                currentBytes = json.length;
                emit(label, 'local');
                return result;
            },
            replace(next, ropts) {
                current = freeze(clone(next));
                currentJson = JSON.stringify(current);
                currentBytes = currentJson.length;
                if (ropts && ropts.resetHistory) { past.length = 0; future.length = 0; pastBytes = 0; }
                barrier = true;
                emit('replace', 'remote');
            },
            undo() {
                const step = past.pop();
                if (!step) return false;
                pastBytes -= step.bytes;
                future.push({ snapshot: current, bytes: currentBytes, label: step.label });
                current = step.snapshot;
                currentJson = null;
                currentBytes = step.bytes;
                barrier = true;
                emit(step.label, 'undo');
                return true;
            },
            redo() {
                const step = future.pop();
                if (!step) return false;
                past.push({ snapshot: current, bytes: currentBytes, label: step.label, coalesce: null, at: 0 });
                pastBytes += currentBytes;
                current = step.snapshot;
                currentJson = null;
                currentBytes = step.bytes;
                barrier = true;
                emit(step.label, 'redo');
                return true;
            },
            canUndo: () => past.length > 0,
            canRedo: () => future.length > 0,
            subscribe(fn) { subs.add(fn); return () => subs.delete(fn); }
        };
    }

    // ── catalog helpers ─────────────────────────────────────────────────────

    // specMap turns the capabilities widget list into a Map type → WidgetSpec.
    function specMap(widgets) { return new Map((widgets || []).map(s => [s.type, s])); }
    function specOf(specs, type) { return specs ? specs.get(type) || null : null; }
    function propSpec(spec, name) { return spec ? (spec.props || []).find(p => p.name === name) || null : null; }

    // propValue returns the widget's prop or the catalog default (missing = default).
    function propValue(widget, spec, name) {
        const props = widget.props || {};
        if (Object.prototype.hasOwnProperty.call(props, name) && props[name] !== null && props[name] !== undefined) return props[name];
        const ps = propSpec(spec, name);
        return ps ? ps.default : undefined;
    }

    function tabCount(widget, spec) {
        const tabs = propValue(widget, spec, 'tabs');
        return Array.isArray(tabs) ? tabs.length : 0;
    }

    // contentBox is the children area of a container relative to its own top-left corner, as
    // validate.go bounds children: the whole panel, a tabview minus its tab bar.
    function contentBox(widget, spec) {
        if (widget.type !== 'tabview') return { x: 0, y: 0, w: widget.w, h: widget.h };
        const pos = propValue(widget, spec, 'bar_position') || 'top';
        const bar = Math.max(0, Math.min(Number(propValue(widget, spec, 'bar_size')) || 0, pos === 'left' || pos === 'right' ? widget.w : widget.h));
        if (pos === 'bottom') return { x: 0, y: 0, w: widget.w, h: widget.h - bar };
        if (pos === 'left') return { x: bar, y: 0, w: widget.w - bar, h: widget.h };
        if (pos === 'right') return { x: 0, y: 0, w: widget.w - bar, h: widget.h };
        return { x: 0, y: bar, w: widget.w, h: widget.h - bar };
    }

    // ── tree lookups ────────────────────────────────────────────────────────

    // walk visits every widget depth first; fn(widget, ctx) returning false stops the walk.
    // ctx: {screen, screenIndex, parent, list, index, depth, ancestors}.
    function walk(project, fn) {
        const screens = project.screens || [];
        let stopped = false;
        function visit(list, screen, screenIndex, parent, depth, ancestors) {
            for (let i = 0; i < list.length && !stopped; i++) {
                const w = list[i];
                if (fn(w, { screen, screenIndex, parent, list, index: i, depth, ancestors }) === false) { stopped = true; return; }
                if (w.children && w.children.length) visit(w.children, screen, screenIndex, w, depth + 1, ancestors.concat([w]));
            }
        }
        for (let s = 0; s < screens.length && !stopped; s++) visit(screens[s].widgets || [], screens[s], s, null, 1, []);
    }

    function findScreen(project, ref) {
        const screens = project.screens || [];
        let index = screens.findIndex(s => s.id === ref);
        if (index < 0 && ref) index = screens.findIndex(s => s.name === ref);
        return index < 0 ? null : { screen: screens[index], index };
    }

    // findWidget finds a widget by id, else by name; screenRef limits the search to one screen.
    function findWidget(project, ref, screenRef) {
        if (!ref) return null;
        const limit = screenRef ? findScreen(project, screenRef) : null;
        if (screenRef && !limit) return null;
        let byId = null;
        let byName = null;
        walk(project, (w, ctx) => {
            if (limit && ctx.screenIndex !== limit.index) return true;
            if (w.id === ref) { byId = Object.assign({ widget: w }, ctx); return false; }
            if (!byName && w.name && w.name === ref) byName = Object.assign({ widget: w }, ctx);
            return true;
        });
        return byId || byName;
    }

    function countWidgets(list) { return (list || []).reduce((n, w) => n + 1 + countWidgets(w.children), 0); }
    function subtreeDepth(w) { return 1 + (w.children || []).reduce((m, c) => Math.max(m, subtreeDepth(c)), 0); }
    function containsId(w, id) { return w.id === id || (w.children || []).some(c => containsId(c, id)); }

    // usedIds mirrors patch.go usedIDs: screen, asset and widget ids share one namespace.
    function usedIds(project) {
        const ids = new Set();
        (project.screens || []).forEach(s => ids.add(s.id));
        (project.assets || []).forEach(a => ids.add(a.id));
        walk(project, w => { if (w.id) ids.add(w.id); });
        return ids;
    }
    function usedNames(project) {
        const names = new Set();
        walk(project, w => { if (w.name) names.add(w.name); });
        return names;
    }

    // absOrigin is the screen position of the content area a widget is placed in (its parent's).
    // env: {specs: Map type → WidgetSpec}.
    function absOrigin(project, ref, env) {
        const found = findWidget(project, ref);
        if (!found) return null;
        const specs = env && env.specs;
        let x = 0, y = 0;
        found.ancestors.forEach(a => { const b = contentBox(a, specOf(specs, a.type)); x += a.x + b.x; y += a.y + b.y; });
        return { x, y };
    }
    function absRect(project, ref, env) {
        const found = findWidget(project, ref);
        if (!found) return null;
        const o = absOrigin(project, ref, env);
        return { x: o.x + found.widget.x, y: o.y + found.widget.y, w: found.widget.w, h: found.widget.h };
    }

    // topLevel drops ids whose ancestor is selected too (a moved/copied container carries them).
    function topLevel(project, ids) {
        const set = new Set(ids);
        return ids.filter(id => {
            const f = findWidget(project, id);
            return f && !f.ancestors.some(a => set.has(a.id));
        });
    }
    function prune(project, ids) { return ids.filter(id => !!findWidget(project, id)); }

    // ── names ───────────────────────────────────────────────────────────────

    function validName(name) { return NAME_RE.test(String(name || '')); }
    function reservedName(name) {
        const n = String(name || '');
        return RESERVED_NAMES.includes(n) || RESERVED_PREFIXES.some(p => n.startsWith(p)) || RESERVED_SUFFIXES.some(s => n.endsWith(s));
    }

    // nameIssue returns '' or the validation code for a widget name (empty = no name, allowed).
    // img_<asset id> of an existing asset collides with the generated image symbol ui_img_<id> (1c).
    function nameIssue(project, name, selfId) {
        if (!name) return '';
        if (!validName(name) || reservedName(name)) return 'NAME_INVALID';
        if ((project.assets || []).some(a => name === 'img_' + a.id)) return 'NAME_INVALID';
        let dup = false;
        walk(project, w => { if (w.name === name && w.id !== selfId) { dup = true; return false; } return true; });
        return dup ? 'NAME_DUPLICATE' : '';
    }
    function screenNameIssue(project, name, selfId) {
        if (!validName(name)) return 'SCREEN_NAME_INVALID';
        return (project.screens || []).some(s => s.name === name && s.id !== selfId) ? 'SCREEN_NAME_DUPLICATE' : '';
    }

    function nextFree(base, taken, skip) {
        for (let n = 1; ; n++) {
            const suffix = '_' + n;
            const candidate = base.slice(0, 32 - suffix.length) + suffix;
            if (!taken.has(candidate) && !(skip && skip(candidate))) return candidate;
        }
    }
    // proposeName returns the first free, unreserved "<prefix>_<n>" (btn_1, lbl_2 …) for a type.
    function proposeName(project, type, extra) {
        const taken = usedNames(project);
        (extra || []).forEach(n => taken.add(n));
        return nextFree(PREFIX[type] || 'w', taken, reservedName);
    }
    function proposeScreenName(project, base) {
        const taken = new Set((project.screens || []).map(s => s.name));
        const stem = validName(base) ? String(base).replace(/_\d+$/, '') : 'screen';
        return nextFree(stem || 'screen', taken);
    }

    // ── ids ─────────────────────────────────────────────────────────────────

    function freshId(prefix, used) {
        let id = P.util.uid(prefix);
        while (used.has(id)) id = P.util.uid(prefix);
        used.add(id);
        return id;
    }

    // cloneSubtree copies a widget with fresh ids and, for named widgets, fresh proposed names.
    function cloneSubtree(project, widget, takenNames) {
        const used = usedIds(project);
        const names = takenNames || usedNames(project);
        const copy = clone(widget);
        (function renew(w) {
            w.id = freshId('w', used);
            if (w.name) { const n = nextFree(PREFIX[w.type] || 'w', names, reservedName); names.add(n); w.name = n; }
            (w.children || []).forEach(renew);
        })(copy);
        return copy;
    }

    // newWidget builds a widget of spec centred nowhere yet: default size (default_w 0 = full
    // screen width), a proposed name and a fresh id. A statusbar starts at the top left.
    function newWidget(project, spec, at, screenSize) {
        return {
            id: freshId('w', usedIds(project)),
            type: spec.type,
            name: proposeName(project, spec.type),
            x: spec.type === 'statusbar' ? 0 : Math.round(at.x),
            y: spec.type === 'statusbar' ? 0 : Math.round(at.y),
            w: spec.default_w || (screenSize ? screenSize.w : 0),
            h: spec.default_h
        };
    }

    // ── ops (structural mirror of internal/cydcreator/patch.go) ──────────────

    function isInt(v) { return typeof v === 'number' && Number.isInteger(v); }
    function isObject(v) { return v !== null && typeof v === 'object' && !Array.isArray(v); }

    // prepare mirrors prepareNewWidget: fresh ids for the widget and its children, zero w/h filled
    // from the catalog (default_w 0 = the parent's width).
    function prepare(w, used, specs, areaW) {
        w.id = freshId('w', used);
        const spec = specOf(specs, w.type);
        if (spec) {
            if (!w.w) w.w = spec.default_w || areaW || 0;
            if (!w.h) w.h = spec.default_h;
        }
        (w.children || []).forEach(c => prepare(c, used, specs, w.w));
    }
    // checkInts rejects non-integer geometry in a new widget tree, like move/update do (the server
    // decodes x/y/w/h as ints, so a fraction would make the whole layout unsavable).
    function checkInts(w, fail) {
        ['x', 'y', 'w', 'h'].forEach(k => { if (w[k] != null && !isInt(w[k])) fail(k + ' must be an integer'); });
        (w.children || []).forEach(c => checkInts(c, fail));
    }
    // resolveScreens mirrors resolveActionScreens: goto_screen targets given by name become ids.
    function resolveScreens(project, w) {
        (w.events || []).forEach(b => {
            const a = b && b.action;
            if (!a || a.type !== 'goto_screen' || !a.screen) return;
            const s = findScreen(project, a.screen);
            if (s) a.screen = s.screen.id;
        });
        (w.children || []).forEach(c => resolveScreens(project, c));
    }

    function mergeProps(target, patch, fail) {
        if (patch === null) { delete target.props; return; }
        if (!isObject(patch)) fail('props must be an object');
        const next = Object.assign({}, target.props || {});
        Object.keys(patch).forEach(k => { if (patch[k] === null || patch[k] === undefined) delete next[k]; else next[k] = clone(patch[k]); });
        if (Object.keys(next).length) target.props = next; else delete target.props;
    }
    function mergeStyle(target, patch, fail, keepEmpty) {
        if (patch === null) { if (keepEmpty) target.style = {}; else delete target.style; return; }
        if (!isObject(patch)) fail('style must be an object');
        const next = Object.assign({}, target.style || {});
        Object.keys(patch).forEach(k => {
            if (!STYLE_KEYS.includes(k)) fail('unknown style field ' + k);
            const v = patch[k];
            if (v === null || v === undefined || v === '') { delete next[k]; return; }
            if (/_color$/.test(k) ? typeof v !== 'string' : !isInt(v)) fail('style.' + k + ' has a wrong type');
            next[k] = v;
        });
        if (Object.keys(next).length || keepEmpty) target.style = next; else delete target.style;
    }

    // resolveParent returns where an added or moved widget goes: the screen root or a container.
    function resolveParent(draft, screen, parentRef, fail, specs) {
        if (!parentRef) return { parent: null, list: screen.widgets, spec: null };
        const f = findWidget(draft, parentRef, screen.id);
        if (!f) fail('parent: widget ' + JSON.stringify(parentRef) + ' not found');
        const spec = specOf(specs, f.widget.type);
        if (!spec || !spec.container) fail('parent ' + f.widget.type + ' cannot contain widgets');
        if (!f.widget.children) f.widget.children = [];
        return { parent: f.widget, list: f.widget.children, spec };
    }
    // placeTab mirrors patch.go: a tab index is kept only under a tabview, anywhere else a
    // non-zero tab fails (the tab range itself is a validation issue, TAB_INVALID).
    function placeTab(w, target, tab, fail) {
        const t = tab == null ? 0 : tab;
        if (!isInt(t)) fail('tab must be an integer');
        if (target.parent && target.parent.type === 'tabview') { if (t) w.tab = t; else delete w.tab; return; }
        if (t) fail('tab is only allowed when the parent is a tabview');
        delete w.tab;
    }

    const OPS = {
        add_widget(draft, op, fail, ctx) {
            const sc = findScreen(draft, op.screen);
            if (!sc) fail('screen ' + JSON.stringify(op.screen || '') + ' not found');
            if (!isObject(op.widget)) fail('widget is required');
            const w = clone(op.widget);
            if (!specOf(ctx.specs, w.type)) fail('unknown widget type ' + JSON.stringify(w.type));
            if (op.x != null) w.x = op.x;
            if (op.y != null) w.y = op.y;
            checkInts(w, fail);
            if (!sc.screen.widgets) sc.screen.widgets = [];
            const target = resolveParent(draft, sc.screen, op.parent, fail, ctx.specs);
            const tab = op.tab != null ? op.tab : (w.tab || 0);
            delete w.tab;
            placeTab(w, target, tab, fail);
            prepare(w, usedIds(draft), ctx.specs, target.parent ? target.parent.w : (ctx.screenSize ? ctx.screenSize.w : 0));
            resolveScreens(draft, w);
            target.list.push(w);
            return w.id;
        },
        update_widget(draft, op, fail) {
            const f = findWidget(draft, op.id, op.screen);
            if (!f) fail('widget ' + JSON.stringify(op.id || '') + ' not found');
            const set = op.set || {};
            if (!Object.keys(set).length) fail('set is empty');
            const w = clone(f.widget);
            Object.keys(set).sort().forEach(key => {
                const v = set[key];
                if (key === 'props') mergeProps(w, v, fail);
                else if (key === 'style') mergeStyle(w, v, fail, false);
                else if (key === 'name') { if (typeof v !== 'string') fail('name must be a string'); if (v) w.name = v; else delete w.name; }
                else if (key === 'locked' || key === 'hidden') { if (typeof v !== 'boolean') fail(key + ' must be true or false'); if (v) w[key] = true; else delete w[key]; }
                else if (key === 'events') { if (v !== null && !Array.isArray(v)) fail('events must be a list'); if (v && v.length) w.events = clone(v); else delete w.events; }
                else if (['x', 'y', 'w', 'h', 'tab'].includes(key)) { if (!isInt(v)) fail(key + ' must be an integer'); if (key === 'tab' && !v) delete w.tab; else w[key] = v; }
                else if (['id', 'type', 'children'].includes(key)) fail(key + ' cannot be changed with update_widget');
                else fail('unknown widget field ' + JSON.stringify(key));
            });
            resolveScreens(draft, w);
            f.list[f.index] = w;
            return w.id;
        },
        move_widget(draft, op, fail, ctx) {
            const f = findWidget(draft, op.id);
            if (!f) fail('widget ' + JSON.stringify(op.id || '') + ' not found');
            if (op.x == null && op.y == null && !op.screen && !op.parent) fail('move_widget needs x/y, screen or parent');
            const w = f.widget;
            const xy = target => {
                if (op.x != null) { if (!isInt(op.x)) fail('x must be an integer'); target.x = op.x; }
                if (op.y != null) { if (!isInt(op.y)) fail('y must be an integer'); target.y = op.y; }
            };
            if (!op.screen && !op.parent) { xy(w); return w.id; }
            const sc = op.screen ? findScreen(draft, op.screen) : { screen: f.screen, index: f.screenIndex };
            if (!sc) fail('screen ' + JSON.stringify(op.screen) + ' not found');
            if (op.parent) {
                const pf = findWidget(draft, op.parent, sc.screen.id);
                if (pf && containsId(w, pf.widget.id)) fail('cannot move a widget into itself');
            }
            f.list.splice(f.index, 1);
            const target = resolveParent(draft, sc.screen, op.parent, fail, ctx.specs);
            delete w.tab;
            placeTab(w, target, op.tab, fail);
            xy(w);
            target.list.push(w);
            return w.id;
        },
        delete_widget(draft, op, fail) {
            const f = findWidget(draft, op.id, op.screen);
            if (!f) fail('widget ' + JSON.stringify(op.id || '') + ' not found');
            f.list.splice(f.index, 1);
            return f.widget.id;
        },
        add_screen(draft, op, fail, ctx) {
            if (!isObject(op.screen_data)) fail('screen_data is required');
            if (!draft.screens) draft.screens = [];
            const s = clone(op.screen_data);
            const used = usedIds(draft);
            s.id = freshId('s', used);
            if (!s.transition) s.transition = 'none';
            if (s.title == null) s.title = '';
            if (!s.style) s.style = {};
            if (!s.widgets) s.widgets = [];
            s.widgets.forEach(w => prepare(w, used, ctx.specs, ctx.screenSize ? ctx.screenSize.w : 0));
            draft.screens.push(s);
            s.widgets.forEach(w => resolveScreens(draft, w));
            if (!draft.start_screen) draft.start_screen = s.id;
            return s.id;
        },
        update_screen(draft, op, fail) {
            const sc = findScreen(draft, op.id || op.screen);
            if (!sc) fail('screen ' + JSON.stringify(op.id || op.screen || '') + ' not found');
            const set = op.set || {};
            if (!Object.keys(set).length) fail('set is empty');
            const s = clone(sc.screen);
            Object.keys(set).sort().forEach(key => {
                const v = set[key];
                if (key === 'style') mergeStyle(s, v, fail, true);
                else if (['name', 'title', 'transition'].includes(key)) { if (typeof v !== 'string') fail(key + ' must be a string'); s[key] = v; }
                else if (key === 'id' || key === 'widgets') fail(key + ' cannot be changed with update_screen');
                else fail('unknown screen field ' + JSON.stringify(key));
            });
            draft.screens[sc.index] = s;
            return s.id;
        },
        // delete_screen keeps one screen (a designer guard; Go reports START_SCREEN_MISSING instead).
        delete_screen(draft, op, fail) {
            const sc = findScreen(draft, op.id || op.screen);
            if (!sc) fail('screen ' + JSON.stringify(op.id || op.screen || '') + ' not found');
            if (draft.screens.length <= 1) fail('the last screen cannot be deleted');
            draft.screens.splice(sc.index, 1);
            if (draft.start_screen === sc.screen.id) draft.start_screen = draft.screens[0].id;
            return sc.screen.id;
        },
        set_theme(draft, op, fail, ctx) {
            if (!op.theme || (ctx.themes && !ctx.themes.includes(op.theme))) fail('unknown theme ' + JSON.stringify(op.theme || ''));
            draft.theme = op.theme;
            return op.theme;
        },
        set_settings(draft, op, fail) {
            const set = op.set || {};
            if (!Object.keys(set).length) fail('set is empty');
            const st = Object.assign({}, draft.settings || {});
            Object.keys(set).sort().forEach(key => {
                const v = set[key];
                if (key === 'wifi') { if (typeof v !== 'string') fail('wifi must be a string'); st.wifi = v; }
                else if (key === 'aurago_link' || key === 'statusbar_clock') { if (typeof v !== 'boolean') fail(key + ' must be true or false'); st[key] = v; }
                else if (key === 'backlight') { if (!isInt(v)) fail('backlight must be an integer'); st.backlight = v; }
                else if (key === 'libraries') { if (v !== null && !(Array.isArray(v) && v.every(x => typeof x === 'string'))) fail('libraries must be a list of strings'); st.libraries = v ? v.slice() : []; }
                else fail('unknown settings field ' + JSON.stringify(key));
            });
            draft.settings = st;
            return '';
        },
        // Designer-only ops (no Go counterpart; the designer saves whole layouts with PUT).
        set_project(draft, op, fail) {
            const set = op.set || {};
            if (!Object.keys(set).length) fail('set is empty');
            Object.keys(set).forEach(key => {
                if (!PROJECT_KEYS.includes(key)) fail('unknown project field ' + JSON.stringify(key));
                if (typeof set[key] !== 'string') fail(key + ' must be a string');
                draft[key] = set[key];
            });
            return '';
        },
        reorder_widget(draft, op, fail) {
            const f = findWidget(draft, op.id);
            if (!f) fail('widget ' + JSON.stringify(op.id || '') + ' not found');
            if (!isInt(op.index)) fail('index must be an integer');
            f.list.splice(f.index, 1);
            f.list.splice(Math.max(0, Math.min(f.list.length, op.index)), 0, f.widget);
            return f.widget.id;
        },
        reorder_screen(draft, op, fail) {
            const sc = findScreen(draft, op.id);
            if (!sc) fail('screen ' + JSON.stringify(op.id || '') + ' not found');
            if (!isInt(op.index)) fail('index must be an integer');
            draft.screens.splice(sc.index, 1);
            draft.screens.splice(Math.max(0, Math.min(draft.screens.length, op.index)), 0, sc.screen);
            return sc.screen.id;
        }
    };

    // apply runs ops on a mutable draft and returns OpResult-like entries {index, op, id}.
    // ctx: {specs: Map type → WidgetSpec, screenSize: {w, h}, themes: [theme id]}. A structural
    // failure throws a ModelError and may leave the draft half-changed: callers apply inside
    // store.change (which discards the draft on error) or use applyOps.
    function apply(draft, ops, ctx) {
        const c = ctx || {};
        return (ops || []).map((op, index) => {
            const name = op && op.op;
            const fn = name && Object.prototype.hasOwnProperty.call(OPS, name) ? OPS[name] : null;
            const fail = reason => { throw new ModelError(index, name || '', reason); };
            if (!fn) fail('unknown op ' + JSON.stringify(name || ''));
            return { index, op: name, id: fn(draft, op, fail, c) || '' };
        });
    }

    // applyOps is the atomic, pure form: the input project is never modified.
    function applyOps(project, ops, ctx) {
        const draft = clone(project);
        const results = apply(draft, ops, ctx);
        return { project: draft, results };
    }

    P.model = {
        NAME_RE, RESERVED_NAMES, RESERVED_PREFIXES, RESERVED_SUFFIXES, LIMITS, TRANSITIONS, ACTIONS, ORIENTATIONS, WIFI_MODES, STYLE_KEYS, PREFIX, HISTORY_MAX_BYTES, ModelError,
        createStore, specMap, specOf, propSpec, propValue, tabCount, contentBox,
        walk, findScreen, findWidget, countWidgets, subtreeDepth, usedIds, usedNames, absOrigin, absRect, topLevel, prune,
        validName, reservedName, nameIssue, screenNameIssue, proposeName, proposeScreenName, cloneSubtree, newWidget,
        apply, applyOps
    };
})();
