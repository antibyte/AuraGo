// EasyDrag editor dialogs: keyboard shortcuts, connect picker, flow settings, save conflict
// and the restore offer for an emergency copy.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};

    // SHORTCUTS lists [keys, label key] per group; "Ctrl" becomes ⌘ on macOS.
    const SHORTCUTS = [
        ['easydrag.ui.keys_group_canvas', [
            ['Tab', 'easydrag.ui.keys_add'],
            ['Enter', 'easydrag.ui.keys_open'],
            ['C', 'easydrag.ui.keys_connect'],
            ['D', 'easydrag.ui.keys_disable'],
            ['Delete', 'easydrag.ui.keys_delete'],
            ['← → ↑ ↓', 'easydrag.ui.keys_navigate'],
            ['Shift+← → ↑ ↓', 'easydrag.ui.keys_nudge'],
            ['Space', 'easydrag.ui.keys_pan'],
            ['Shift+1', 'easydrag.ui.keys_fit'],
            ['+ / −', 'easydrag.ui.keys_zoom'],
            ['Ctrl+0', 'easydrag.ui.keys_zoom_reset']
        ]],
        ['easydrag.ui.keys_group_edit', [
            ['Ctrl+Z', 'easydrag.ui.undo'],
            ['Ctrl+Shift+Z', 'easydrag.ui.redo'],
            ['Ctrl+C', 'easydrag.ui.copy'],
            ['Ctrl+X', 'easydrag.ui.cut'],
            ['Ctrl+V', 'easydrag.ui.paste'],
            ['Ctrl+D', 'easydrag.ui.duplicate'],
            ['Ctrl+A', 'easydrag.ui.select_all']
        ]],
        ['easydrag.ui.keys_group_flow', [
            ['Ctrl+S', 'easydrag.ui.save_now'],
            ['Ctrl+Enter', 'easydrag.ui.test'],
            ['Ctrl+K', 'easydrag.ui.keys_search'],
            ['?', 'easydrag.ui.keys_help'],
            ['Esc', 'easydrag.ui.keys_escape']
        ]]
    ];

    // kbd draws one key per part of keys: "+" joins keys ("Ctrl+S"), but a leading "+" is a key
    // itself ("+ / −" must not become an empty key and " / −").
    function kbd(esc, keys) {
        const text = ED.core.shortcut(keys);
        return (text.startsWith('+') ? [text] : text.split('+')).map(k => '<kbd>' + esc(k) + '</kbd>').join('<span class="ed-kbd-plus">+</span>');
    }

    function shortcuts(ed) {
        const { t, esc } = ed;
        const body = '<div class="ed-keys">' + SHORTCUTS.map(([group, rows]) =>
            '<section><h4 class="ed-section-title">' + esc(t(group)) + '</h4><dl>' +
            rows.map(([keys, label]) => '<div class="ed-keys-row"><dt>' + kbd(esc, keys) + '</dt><dd>' + esc(t(label)) + '</dd></div>').join('') +
            '</dl></section>').join('') + '</div>';
        return ED.core.modal(ed.root, { title: t('easydrag.ui.keys_title'), closeLabel: t('easydrag.ui.close'), className: 'ed-modal--keys', body });
    }

    // connectPicker lists nodes the single selected node can connect to (first free ports).
    function connectPicker(ed, canvas, nodeId) {
        const core = ED.core;
        const { t, esc } = ed;
        const src = ed.model.node(nodeId);
        if (!src) return null;
        const outs = ed.model.outputs(src);
        const candidates = [];
        ed.model.doc.nodes.forEach(n => {
            if (n.id === nodeId) return;
            const ins = ed.model.inputs(n);
            for (const port of outs) {
                const inPort = ins.find(p => ed.model.canConnect(nodeId, port, n.id, p).ok);
                if (inPort) { candidates.push({ node: n, port, inPort }); return; }
            }
        });
        const row = c => {
            const info = ed.model.info(c.node.type);
            const portLabel = outs.length > 1 ? '<span class="ed-chip ed-chip--muted">' + esc(core.tr(t, 'easydrag.ui.port_' + c.port, c.port)) + '</span>' : '';
            return '<button type="button" class="ed-pick-row" data-ed-pick="' + esc(c.node.id) + '" data-cat="' + esc(core.catOf(info)) + '">' +
                '<span class="ed-tile">' + core.icon(info ? info.icon : 'tool') + '</span><span class="ed-pick-label">' + esc(c.node.label || c.node.type) + '</span>' + portLabel + '</button>';
        };
        const body = '<div class="ed-search">' + core.icon('search') + '<input type="search" data-ed-pick-search aria-label="' + esc(t('easydrag.ui.search')) + '" placeholder="' + esc(t('easydrag.ui.search')) + '" autofocus></div>' +
            '<div class="ed-pick-list" role="list">' + (candidates.length ? candidates.map(row).join('') : '<p class="ed-hint">' + esc(t('easydrag.ui.connect_none')) + '</p>') + '</div>';
        const dialog = core.modal(ed.root, {
            title: t('easydrag.ui.connect_title', { name: src.label || src.type }), closeLabel: t('easydrag.ui.close'), className: 'ed-modal--picker', body,
            actions: [{ id: 'new', label: t('easydrag.ui.connect_new'), icon: 'plus' }],
            onAction: (id) => {
                if (id !== 'new') return true;
                const r = canvas.nodeEl(nodeId) ? canvas.nodeEl(nodeId).getBoundingClientRect() : null;
                setTimeout(() => ed.bus.emit('quick-add', { x: r ? r.right + 40 : 0, y: r ? r.top : 0, from: { node: nodeId, port: outs[0] } }), 0);
                return true;
            }
        });
        const search = dialog.body.querySelector('[data-ed-pick-search]');
        search.addEventListener('input', () => {
            const q = search.value.trim().toLowerCase();
            dialog.body.querySelectorAll('[data-ed-pick]').forEach(b => { b.hidden = !!q && !b.textContent.toLowerCase().includes(q); });
        });
        search.addEventListener('keydown', (event) => {
            if (event.key !== 'Enter') return;
            const first = dialog.body.querySelector('[data-ed-pick]:not([hidden])');
            if (first) { event.preventDefault(); first.click(); }
        });
        dialog.body.addEventListener('click', (event) => {
            const b = event.target.closest('[data-ed-pick]');
            if (!b) return;
            const c = candidates.find(x => x.node.id === b.dataset.edPick);
            dialog.close('picked');
            ed.model.connect(nodeId, c.port, c.node.id, c.inPort);
            canvas.announce(t('easydrag.ui.connect_done', { from: src.label || src.type, to: c.node.label || c.node.type }));
        });
        return dialog;
    }

    // flowSettings edits name, description and flow-wide settings as one undo step.
    function flowSettings(ed) {
        const core = ED.core;
        const { t, esc } = ed;
        const doc = ed.model.doc;
        const s = doc.settings || {};
        const ro = !!(ed.readonly || ed.runView);
        const opt = (value, current, label) => '<option value="' + esc(value) + '"' + (value === current ? ' selected' : '') + '>' + esc(label) + '</option>';
        const body =
            '<label class="ed-field"><span class="ed-label">' + esc(t('easydrag.ui.flow_name')) + '</span><input class="ed-input" data-ed-set="name" maxlength="120" value="' + esc(doc.name || '') + '"' + (ro ? ' disabled' : '') + '></label>' +
            '<label class="ed-field"><span class="ed-label">' + esc(t('easydrag.ui.flow_description')) + '</span><textarea class="ed-input" data-ed-set="description" rows="3" maxlength="2000"' + (ro ? ' disabled' : '') + '>' + esc(doc.description || '') + '</textarea></label>' +
            '<label class="ed-field"><span class="ed-label">' + esc(t('easydrag.ui.flow_concurrency')) + '</span><select class="ed-input" data-ed-set="concurrency"' + (ro ? ' disabled' : '') + '>' +
            ['queue', 'parallel', 'skip'].map(v => opt(v, s.concurrency || 'queue', t('easydrag.ui.concurrency_' + v))).join('') + '</select>' +
            '<span class="ed-help">' + esc(t('easydrag.ui.flow_concurrency_help')) + '</span></label>' +
            '<label class="ed-field"><span class="ed-label">' + esc(t('easydrag.ui.flow_max_run')) + '</span><span class="ed-input-unit"><input class="ed-input" type="number" min="10" max="86400" step="10" data-ed-set="max_run_minutes" value="' + Math.round((s.max_run_seconds || 1800) / 60) + '"' + (ro ? ' disabled' : '') + '><span>' + esc(t('easydrag.ui.unit_minutes')) + '</span></span></label>' +
            '<label class="ed-field"><span class="ed-label">' + esc(t('easydrag.ui.flow_notify')) + '</span><select class="ed-input" data-ed-set="notify_on_error"' + (ro ? ' disabled' : '') + '>' +
            ['desktop', 'push', 'telegram', 'off'].map(v => opt(v, s.notify_on_error || 'desktop', t('easydrag.ui.notify_' + v))).join('') + '</select></label>';
        return core.modal(ed.root, {
            title: t('easydrag.ui.flow_settings'), closeLabel: t('easydrag.ui.close'), className: 'ed-modal--settings', body,
            actions: ro ? [{ id: 'close-dialog', label: t('easydrag.ui.close') }] : [{ id: 'cancel', label: t('easydrag.ui.cancel') }, { id: 'apply', label: t('easydrag.ui.apply'), primary: true }],
            onAction: (id, dialog) => {
                if (id !== 'apply') return true;
                const get = name => dialog.body.querySelector('[data-ed-set="' + name + '"]').value;
                const minutes = core.clamp(Math.round(Number(get('max_run_minutes')) || 30), 1, 1440);
                ed.model.setFlow({
                    name: get('name').trim() || doc.name,
                    description: get('description').trim(),
                    settings: Object.assign({}, s, { concurrency: get('concurrency'), max_run_seconds: minutes * 60, notify_on_error: get('notify_on_error') })
                });
                return true;
            }
        });
    }

    // conflict asks whether to load the server's newer draft or keep the local one. Only an explicit
    // "keep" overwrites the server and only an explicit "reload" drops the local edits. Any other
    // result (null when the dialog closed without an answer, e.g. with its window) rejects: the
    // saver then goes offline with this error, keeps the emergency copy and asks again on its next
    // attempt. "reload" would lose the local edits and "keep" the server's, both without a choice.
    function conflict(ed) {
        const { t, esc } = ed;
        const dialog = ED.core.modal(ed.root, {
            title: t('easydrag.ui.conflict_title'), closeLabel: t('easydrag.ui.close'), dismissible: false,
            body: '<p>' + esc(t('easydrag.ui.conflict_text')) + '</p>',
            actions: [{ id: 'reload', label: t('easydrag.ui.conflict_reload'), icon: 'refresh' }, { id: 'keep', label: t('easydrag.ui.conflict_keep'), primary: true }]
        });
        return dialog.done.then(result => {
            if (result === 'reload' || result === 'keep') return result;
            const text = 'the save conflict was not resolved';
            throw Object.assign(new Error(text), { body: { error: text, code: 'FLOW_REVISION_CONFLICT' } });
        });
    }

    // restore offers a local emergency copy that the server never received. It resolves to
    // "restore" or "discard"; anything else (null when the dialog closed without an answer, e.g.
    // with its window) resolves to null: keep the copy and decide when the flow opens again.
    function restore(ed, copy) {
        const { t, esc } = ed;
        const dialog = ED.core.modal(ed.root, {
            title: t('easydrag.ui.restore_title'), closeLabel: t('easydrag.ui.close'), dismissible: false,
            body: '<p>' + esc(t('easydrag.ui.restore_text', { time: ED.core.fmt.dateTime(copy.at) })) + '</p>',
            actions: [{ id: 'discard', label: t('easydrag.ui.restore_discard'), danger: true }, { id: 'restore', label: t('easydrag.ui.restore_apply'), primary: true, icon: 'history' }]
        });
        return dialog.done.then(result => (result === 'restore' || result === 'discard' ? result : null));
    }

    ED.dialogs = { shortcuts, connectPicker, flowSettings, conflict, restore, SHORTCUTS };
})();
