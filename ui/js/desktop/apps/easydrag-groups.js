// Flat presentation groups: real nodes remain the sole executable graph and selection values.
(function () {
    'use strict';
    const ED = window.EasyDrag = window.EasyDrag || {};
    function create(ed, canvas) {
        const core = ED.core, G = ED.geometry, bag = core.bag(), key = 'aurago.easydrag.group-view.' + ed.flow.id;
        const rawStored = core.storage.get(key, {}), stored = rawStored && typeof rawStored === 'object' ? rawStored : {};
        const validIDs = values => Array.isArray(values) ? values.slice(0, 100).filter(id => typeof id === 'string' && /^g_[a-z2-7]{8}$/.test(id)) : [];
        const collapsed = new Set(validIDs(stored.collapsed)), locked = new Set(validIDs(stored.locked));
        const frames = core.el('<div class="ed-group-frames" aria-hidden="true"></div>'), heads = core.el('<div class="ed-group-heads"></div>');
        canvas.world.insertBefore(frames, canvas.world.firstChild); canvas.world.appendChild(heads);
        let groups = [], memberOf = new Map(), rows = new Map(), active = null, closed = false, invalid = false, issueNodes = new Set((ed.issues || []).filter(issue => issue.severity === 'error').map(issue => issue.node_id)), boundaryCounts = new Map(), viewVersion = 0, inspector = null;
        const writable = () => !closed && !ed.readonly && !ed.runView;
        const save = () => core.storage.set(key, { collapsed: Array.from(collapsed).slice(0, 100), locked: Array.from(locked).slice(0, 100) });
        function rebuild() {
            const raw = ed.model.doc.groups;
            invalid = !ED.model.validGroups(raw, new Set(ed.model.doc.nodes.map(node => node.id)));
            groups = invalid || !Array.isArray(raw) ? [] : raw;
            memberOf = new Map(); groups.forEach(group => group.members.forEach(id => memberOf.set(id, group.id)));
            boundaryCounts = new Map();
            ed.model.doc.edges.forEach(edge => { const a = memberOf.get(edge.source.node), b = memberOf.get(edge.target.node); if (a !== b) [a, b].forEach(id => { if (id) boundaryCounts.set(id, (boundaryCounts.get(id) || 0) + 1); }); });
            if (active && !group(active)) active = null;
        }
        function group(id) { return groups.find(item => item.id === id) || null; }
        function bounds(item) {
            const box = G.bounds(item.members.map(id => canvas.nodeRect(id)).filter(Boolean));
            if (!box) return null;
            const header = item.note ? 128 : 88;
            return { x: box.x - 18, y: box.y - header, w: collapsed.has(item.id) ? 300 : Math.max(300, box.w + 36), h: collapsed.has(item.id) ? (item.note ? 156 : 120) : box.h + header + 18 };
        }
        function countText(item) {
            let errors = 0, running = 0, success = 0, disabled = 0;
            item.members.forEach(id => { const node = ed.model.node(id), status = ed.run?.steps?.get(id)?.status; if (status === 'error' || issueNodes.has(id)) errors++; if (['running', 'queued'].includes(status)) running++; if (status === 'success') success++; if (node?.settings.disabled) disabled++; });
            return [ed.t('easydrag.ui.group_member_count', { count: item.members.length }), ed.t('easydrag.ui.group_edges_count', { count: boundaryCounts.get(item.id) || 0 }), errors ? ed.t('easydrag.ui.group_errors', { count: errors }) : '', running ? ed.t('easydrag.ui.group_running', { count: running }) : '', success ? ed.t('easydrag.ui.group_success', { count: success }) : '', disabled ? ed.t('easydrag.ui.group_disabled', { count: disabled }) : ''].filter(Boolean).join(' · ');
        }
        function paintCounts(item) {
            const row = rows.get(item.id); if (!row) return;
            const count = row.head.querySelector('.ed-group-counts'), text = countText(item); if (count.textContent !== text) count.textContent = text;
            const outline = ed.outlineOpen && canvas.listRegion.querySelector('[data-ed-outline-count="' + item.id + '"]'); if (outline && outline.textContent !== text) outline.textContent = text;
        }
        function paint(item) {
            let row = rows.get(item.id);
            if (!row) {
                const frame = core.el('<div class="ed-group-frame"></div>'); frames.appendChild(frame);
                const head = core.el('<section class="ed-group-head"><div class="ed-group-heading"><button type="button" class="ed-group-title" data-ed-group-drag></button><div class="ed-group-controls"><button type="button" class="ed-icon-btn" data-ed-group-action="collapse"></button><button type="button" class="ed-icon-btn" data-ed-group-action="lock"></button><button type="button" class="ed-icon-btn" data-ed-group-action="edit"></button></div></div><p class="ed-group-note"></p><p class="ed-group-counts"></p></section>'); heads.appendChild(head);
                row = { frame, head }; rows.set(item.id, row);
            }
            const box = bounds(item); if (!box) return;
            const isCollapsed = collapsed.has(item.id), isLocked = locked.has(item.id), cls = ' ed-group--' + item.color + (isCollapsed ? ' is-collapsed' : '') + (isLocked ? ' is-locked' : '') + (active === item.id ? ' is-selected' : '');
            row.frame.className = 'ed-group-frame' + cls; row.head.className = 'ed-group-head' + cls;
            row.frame.dataset.edGroup = item.id; row.head.dataset.edGroup = item.id;
            Object.assign(row.frame.style, { transform: 'translate(' + box.x + 'px,' + box.y + 'px)', width: box.w + 'px', height: box.h + 'px' });
            Object.assign(row.head.style, { transform: 'translate(' + box.x + 'px,' + box.y + 'px)', width: box.w + 'px' });
            const title = row.head.querySelector('[data-ed-group-drag]'); title.dataset.edGroupDrag = item.id; title.textContent = item.title; title.title = item.title; title.setAttribute('aria-label', ed.t('easydrag.ui.group_select') + ' · ' + item.title);
            const note = row.head.querySelector('.ed-group-note'); note.textContent = item.note || ''; note.hidden = !item.note; note.title = item.note || '';
            const toggle = row.head.querySelector('[data-ed-group-action="collapse"]'), lock = row.head.querySelector('[data-ed-group-action="lock"]');
            toggle.innerHTML = core.icon(isCollapsed ? 'chevron-right' : 'chevron-down'); toggle.setAttribute('aria-label', ed.t(isCollapsed ? 'easydrag.ui.group_expand' : 'easydrag.ui.group_collapse')); toggle.setAttribute('aria-expanded', String(!isCollapsed));
            lock.innerHTML = core.icon('lock'); lock.setAttribute('aria-label', ed.t(isLocked ? 'easydrag.ui.group_unlock' : 'easydrag.ui.group_lock')); lock.setAttribute('aria-pressed', String(isLocked));
            const edit = row.head.querySelector('[data-ed-group-action="edit"]'); edit.innerHTML = core.icon('pencil'); edit.setAttribute('aria-label', ed.t('easydrag.ui.group_edit'));
            paintCounts(item);
        }
        function syncNodes() { ed.model.doc.nodes.forEach(node => { const card = canvas.nodeEl(node.id); if (card) { const hidden = collapsed.has(memberOf.get(node.id)); if (card.hidden !== hidden) card.hidden = hidden; } }); }
        function render() {
            rebuild(); const present = new Set(groups.map(item => item.id));
            rows.forEach((row, id) => { if (!present.has(id)) { row.frame.remove(); row.head.remove(); rows.delete(id); } });
            groups.forEach(paint); syncNodes();
        }
        function select(id, additive) {
            const item = group(id); if (!item) return;
            active = id; if (!additive) ed.selection.clear(); item.members.forEach(member => ed.selection.add(member)); ed.selectedEdge = null; ed.bus.emit('selection', ed.selection);
        }
        function toggle(id, kind) {
            if (!group(id)) return; const target = kind === 'lock' ? locked : collapsed;
            if (target.has(id)) target.delete(id); else target.add(id); save(); viewVersion++; render(); ed.bus.emit('group-view', { id, kind });
        }
        function createFromSelection() {
            if (!writable() || !ed.selection.size || ed.flushPendingEdits && ed.flushPendingEdits() === false) return;
            const id = ed.model.createGroup(Array.from(ed.selection), ed.t('easydrag.ui.group_default')); if (!id) ed.ctx.notify({ title: ed.t('easydrag.ui.groups'), message: ed.t('easydrag.ui.issue_group_invalid'), type: 'error' }); else { select(id); open(id, true); }
        }
        const label = id => { const node = ed.model.node(id); return node ? node.label || ed.model.info(node.type)?.label || node.type : id; };
        // A presentation endpoint never changes the graph's real node IDs, ports or positions.
        function visualRect(id) { const owner = memberOf.get(id); return owner && collapsed.has(owner) ? bounds(group(owner)) : canvas.nodeRect(id); }
        function edgeVisible(edge) { const a = memberOf.get(edge.source.node); return !a || !collapsed.has(a) || a !== memberOf.get(edge.target.node); }
        function endpoint(id, side, fallback) {
            const owner = memberOf.get(id); if (!owner || !collapsed.has(owner)) return fallback;
            const box = bounds(group(owner)); return box ? { x: side === 'out' ? box.x + box.w : box.x, y: box.y + box.h / 2 } : fallback;
        }
        function move(id, dx, dy) {
            const item = group(id); if (!writable() || !item || locked.has(id) || !Number.isFinite(dx) || !Number.isFinite(dy) || Math.abs(dx) > 100000 || Math.abs(dy) > 100000 || ed.flushPendingEdits && !ed.flushPendingEdits()) return false;
            ed.model.endCoalescing(); ed.model.moveNodes(item.members, dx, dy); ed.model.endCoalescing(); return true;
        }
        function open(id, rename) {
            if (closed || !group(id) || ed.flushPendingEdits && !ed.flushPendingEdits()) return false;
            if (ed.detail) ed.detail.close();
            const model = ed.model, local = core.bag(), esc = ed.esc, t = ed.t;
            let disposed = false, composing = false, written = '', membersSignature = '';
            const current = () => !disposed && !closed && ed.model === model && group(id);
            const canWrite = () => !!current() && writable();
            const labels = { title: 'easydrag.ui.group_title', note: 'easydrag.ui.group_note', color: 'easydrag.ui.group_color' };
            const field = (key, input) => '<label class="ed-group-field"><span>' + esc(t(labels[key])) + '</span>' + input + '</label>';
            const el = core.el('<aside class="ed-detail-backdrop is-open ed-group-inspector" data-ed-group-inspector><div class="ed-detail" role="region" aria-label="' + esc(t('easydrag.ui.group_edit')) + '"><header class="ed-detail-head"><div class="ed-detail-title"><strong>' + esc(t('easydrag.ui.group_edit')) + '</strong><span class="ed-detail-type">' + esc(t('easydrag.ui.groups')) + '</span></div><div class="ed-detail-actions"><button type="button" class="ed-icon-btn" data-ed-detail-close aria-label="' + esc(t('easydrag.ui.close')) + '">' + core.icon('x') + '</button></div></header><div class="ed-group-body">' +
                '<p class="ed-hint">' + esc(t('easydrag.ui.group_view_hint')) + '</p><p class="ed-hint" data-ed-group-readonly hidden>' + esc(t('easydrag.ui.group_readonly')) + '</p>' +
                field('title', '<input class="ed-input" data-ed-group-title spellcheck="true">') + field('note', '<textarea class="ed-input" rows="3" data-ed-group-note></textarea>') +
                field('color', '<select class="ed-input" data-ed-group-color>' + ED.model.GROUP_COLORS.map(color => '<option value="' + color + '">' + esc(t('easydrag.ui.group_color_' + color)) + '</option>').join('') + '</select>') +
                '<p class="ed-error" role="alert" data-ed-group-error hidden></p><div class="ed-group-view-actions"><button type="button" class="ed-btn" data-ed-group-view="collapse"></button><button type="button" class="ed-btn" data-ed-group-view="lock"></button></div><details class="ed-group-help"><summary>' + esc(t('easydrag.ui.group_limit')) + '</summary><p class="ed-hint">' + esc(t('easydrag.ui.group_lock_hint')) + '</p></details>' +
                '<section><h3>' + esc(t('easydrag.ui.group_members')) + '</h3><ul class="ed-group-members" data-ed-group-members></ul><div class="ed-group-add"><select class="ed-input" data-ed-group-add-member aria-label="' + esc(t('easydrag.ui.group_choose_member')) + '"></select><button type="button" class="ed-btn" data-ed-group-add>' + core.icon('plus') + esc(t('easydrag.ui.group_add_member')) + '</button></div><button type="button" class="ed-btn" data-ed-group-add-selection>' + esc(t('easydrag.ui.group_add_selection')) + '</button><p class="ed-hint">' + esc(t('easydrag.ui.group_reassign_hint')) + '</p></section>' +
                '<section class="ed-group-move"><h3>' + esc(t('easydrag.ui.group_move')) + '</h3><div class="ed-group-delta"><label>' + esc(t('easydrag.ui.settings_position_x')) + '<input type="number" class="ed-input" value="0" step="16" min="-100000" max="100000" data-ed-group-dx></label><label>' + esc(t('easydrag.ui.settings_position_y')) + '<input type="number" class="ed-input" value="0" step="16" min="-100000" max="100000" data-ed-group-dy></label><button type="button" class="ed-btn" data-ed-group-move>' + esc(t('easydrag.ui.group_move')) + '</button></div></section><button type="button" class="ed-btn ed-btn--danger" data-ed-group-ungroup>' + esc(t('easydrag.ui.group_ungroup')) + '</button></div></div></aside>');
            ed.root.querySelector('.ed-body').appendChild(el);
            const title = el.querySelector('[data-ed-group-title]'), note = el.querySelector('[data-ed-group-note]'), color = el.querySelector('[data-ed-group-color]'), error = el.querySelector('[data-ed-group-error]');
            function flush() {
                if (!current() || !canWrite()) return !composing;
                if (composing) return false;
                const item = group(id), patch = { title: title.value, note: note.value, color: color.value }, proposed = Object.assign({}, item, patch);
                if (!ED.model.validGroups([proposed], new Set(model.doc.nodes.map(node => node.id)))) { error.textContent = t('easydrag.ui.issue_group_invalid'); error.hidden = false; return false; }
                error.hidden = true;
                const signature = JSON.stringify(patch); if (signature === written) return true;
                // Record our own content before the synchronous model event can refresh this view.
                written = signature; return model.setGroup(id, patch);
            }
            function close(force) {
                if (disposed || !force && !flush()) return false;
                disposed = true; local.dispose(); el.remove(); ed.root.classList.remove('is-inspector');
                if (ed.detail === detail) ed.detail = null;
                if (inspector === detail) inspector = null;
                ed.bus.emit('detail-closed', null); return true;
            }
            function sync() {
                if (!current()) { close(true); return; }
                const item = group(id), ro = !canWrite(), next = JSON.stringify({ title: item.title, note: item.note || '', color: item.color });
                if (next !== written && !composing) { title.value = item.title; note.value = item.note || ''; color.value = item.color; written = next; error.hidden = true; }
                title.readOnly = note.readOnly = ro; color.disabled = ro; el.querySelector('[data-ed-group-readonly]').hidden = !ro;
                el.querySelector('[data-ed-group-view="collapse"]').textContent = t(collapsed.has(id) ? 'easydrag.ui.group_expand' : 'easydrag.ui.group_collapse');
                const lock = el.querySelector('[data-ed-group-view="lock"]'); lock.textContent = t(locked.has(id) ? 'easydrag.ui.group_unlock' : 'easydrag.ui.group_lock'); lock.setAttribute('aria-pressed', String(locked.has(id)));
                const signature = JSON.stringify([item.members, model.doc.nodes.map(node => [node.id, label(node.id)])]);
                if (signature !== membersSignature) {
                    membersSignature = signature;
                    el.querySelector('[data-ed-group-members]').innerHTML = item.members.map(member => '<li><button type="button" class="ed-btn ed-btn--ghost" data-ed-group-focus="' + esc(member) + '">' + esc(label(member)) + '</button><button type="button" class="ed-icon-btn" data-ed-group-remove="' + esc(member) + '" aria-label="' + esc(t('easydrag.ui.group_remove_member') + ' · ' + label(member)) + '">' + core.icon('x') + '</button></li>').join('');
                    el.querySelector('[data-ed-group-add-member]').innerHTML = '<option value="">' + esc(t('easydrag.ui.group_choose_member')) + '</option>' + model.doc.nodes.filter(node => !item.members.includes(node.id)).map(node => '<option value="' + esc(node.id) + '">' + esc(label(node.id)) + '</option>').join('');
                }
                el.querySelectorAll('[data-ed-group-remove], [data-ed-group-add], [data-ed-group-add-selection], [data-ed-group-add-member], [data-ed-group-ungroup]').forEach(control => { control.disabled = ro; });
                el.querySelectorAll('[data-ed-group-dx], [data-ed-group-dy], [data-ed-group-move]').forEach(control => { control.disabled = ro || locked.has(id); });
                const width = ed.root.getBoundingClientRect().width; ed.root.classList.toggle('is-inspector', width > 0 && width < 1200);
            }
            const detail = { groupId: () => id, nodeId: () => null, flushPendingEdits: flush, flushNote: flush, close: () => close(true) };
            ed.detail = inspector = detail;
            [title, note].forEach(input => { local.listen(input, 'compositionstart', () => { composing = true; }); local.listen(input, 'compositionend', () => { composing = false; }); local.listen(input, 'change', flush); });
            local.listen(color, 'change', flush);
            local.listen(el, 'keydown', event => { if (event.key === 'Escape' && !event.isComposing && !composing) { event.stopPropagation(); event.preventDefault(); close(false); } });
            local.listen(el, 'click', event => {
                const button = event.target.closest('button'); if (!button || button.disabled || !current()) return;
                if (button.hasAttribute('data-ed-detail-close')) { close(false); return; }
                if (!flush()) return;
                if (button.dataset.edGroupView) { toggle(id, button.dataset.edGroupView); return; }
                if (button.dataset.edGroupFocus) { const member = button.dataset.edGroupFocus; close(false); ed.bus.emit('focus-node', member); return; }
                if (!canWrite()) return;
                if (button.dataset.edGroupRemove) model.setGroup(id, { members: group(id).members.filter(member => member !== button.dataset.edGroupRemove) });
                else if (button.hasAttribute('data-ed-group-add') || button.hasAttribute('data-ed-group-add-selection')) {
                    const add = button.hasAttribute('data-ed-group-add') ? [el.querySelector('[data-ed-group-add-member]').value] : Array.from(ed.selection);
                    const members = Array.from(new Set(group(id).members.concat(add.filter(member => model.node(member))))); model.setGroup(id, { members });
                } else if (button.hasAttribute('data-ed-group-ungroup')) { model.ungroup(id); active = null; ed.selection.clear(); ed.bus.emit('selection', ed.selection); }
                else if (button.hasAttribute('data-ed-group-move')) { const dx = el.querySelector('[data-ed-group-dx]'), dy = el.querySelector('[data-ed-group-dy]'); if (dx.value !== '' && dy.value !== '') move(id, Number(dx.value), Number(dy.value)); }
            });
            local.add(ed.bus.on('model', sync)); local.add(ed.bus.on('group-view', sync)); local.listen(window, 'resize', sync);
            sync(); ed.bus.emit('detail-opened', el);
            if (rename && canWrite()) { title.focus(); title.select(); } else el.querySelector('[data-ed-detail-close]').focus({ preventScroll: true });
            return true;
        }
        function openList() {
            if (closed || ed.flushPendingEdits && !ed.flushPendingEdits()) return;
            const dialog = core.modal(ed.root, { title: ed.t('easydrag.ui.groups'), closeLabel: ed.t('easydrag.ui.close'), body: '<p class="ed-hint">' + ed.esc(ed.t('easydrag.ui.group_view_hint')) + '</p><div data-ed-groups-list></div><button type="button" class="ed-btn ed-btn--primary" data-ed-group-create>' + core.icon('plus') + ed.esc(ed.t('easydrag.ui.group_create')) + '</button>', actions: [{ id: 'close', label: ed.t('easydrag.ui.close') }] });
            const list = dialog.body.querySelector('[data-ed-groups-list]');
            // This is a bounded display copy only. Malformed author data stays untouched in the model.
            let left = 200;
            function preview(value, depth) { if (left-- <= 0 || depth > 8) return '…'; if (typeof value === 'string') return value.length > 1000 ? value.slice(0, 1000) + '…' : value; if (!value || typeof value !== 'object') return value; if (Array.isArray(value)) return value.slice(0, Math.min(200, left)).map(item => preview(item, depth + 1)); const out = Object.create(null); for (const key of Object.keys(value).slice(0, Math.min(200, left))) out[key.slice(0, 256)] = preview(value[key], depth + 1); return out; }
            if (invalid) list.innerHTML = '<p class="ed-error">' + ed.esc(ed.t('easydrag.ui.group_invalid')) + '</p><div class="ed-json-tree">' + ED.detail.jsonTree(preview(ed.model.doc.groups, 0), ed.esc) + '</div><button type="button" class="ed-btn ed-btn--danger" data-ed-group-clear>' + ed.esc(ed.t('easydrag.ui.group_clear_invalid')) + '</button>';
            else list.innerHTML = groups.map(item => '<button type="button" class="ed-btn ed-group-list-item" data-ed-group-open="' + ed.esc(item.id) + '"><strong>' + ed.esc(item.title) + '</strong><span>' + ed.esc(countText(item)) + '</span></button>').join('');
            dialog.body.querySelector('[data-ed-group-create]').disabled = !writable() || invalid || !ed.selection.size;
            const clear = dialog.body.querySelector('[data-ed-group-clear]'); if (clear) clear.disabled = !writable();
            const model = ed.model, version = model.version;
            dialog.body.addEventListener('click', async event => {
                const button = event.target.closest('button'); if (!button || button.disabled || closed || !dialog.el.isConnected) return;
                if (button.dataset.edGroupOpen) { const id = button.dataset.edGroupOpen; dialog.close(null); select(id); open(id); }
                else if (button.hasAttribute('data-ed-group-create')) { dialog.close(null); createFromSelection(); }
                else if (button.hasAttribute('data-ed-group-clear') && writable()) { const yes = await ed.ctx.confirmDialog(ed.t('easydrag.ui.groups'), ed.t('easydrag.ui.group_clear_invalid')); if (yes && dialog.el.isConnected && writable() && ed.model === model && model.version === version) { model.setGroups([]); dialog.close(null); } }
            });
            const off = ed.bus.on('model', () => dialog.close(null)); dialog.done.then(off);
            bag.add(() => dialog.close(null));
        }
        bag.listen(heads, 'pointerdown', event => { if (event.target.closest('.ed-group-controls')) event.stopPropagation(); });
        bag.listen(heads, 'click', event => { const host = event.target.closest('[data-ed-group]'); if (!host) return; event.stopPropagation(); const button = event.target.closest('[data-ed-group-action]'); if (button) { if (button.dataset.edGroupAction === 'edit') open(host.dataset.edGroup); else toggle(host.dataset.edGroup, button.dataset.edGroupAction); } else if (event.target.closest('[data-ed-group-drag]') && !ed.dragging) select(host.dataset.edGroup, event.shiftKey || core.isMod(event)); });
        bag.listen(heads, 'dblclick', event => { const host = event.target.closest('[data-ed-group]'); if (host) { event.stopPropagation(); if (event.target.closest('[data-ed-group-drag]')) open(host.dataset.edGroup); } });
        bag.listen(heads, 'keydown', event => { if (event.key === 'Enter' && event.target.closest('[data-ed-group-drag]')) { event.preventDefault(); event.stopPropagation(); open(event.target.dataset.edGroupDrag); } });
        bag.add(ed.bus.on('model', render));
        bag.add(ed.bus.on('issues', () => { issueNodes = new Set((ed.issues || []).filter(issue => issue.severity === 'error').map(issue => issue.node_id)); groups.forEach(paintCounts); }));
        bag.add(ed.bus.on('selection', () => { if (active && !group(active)?.members.every(id => ed.selection.has(id))) active = null; rows.forEach((row, id) => { row.frame.classList.toggle('is-selected', active === id); row.head.classList.toggle('is-selected', active === id); }); }));
        let lastRun = null, seen = 0;
        bag.add(ed.bus.on('run', () => { if (!groups.length) return; const run = ed.run, log = run?.changes; if (run !== lastRun || !Array.isArray(log) || seen > log.length) { groups.forEach(paintCounts); } else new Set(log.slice(seen).map(id => memberOf.get(id))).forEach(id => { const item = group(id); if (item) paintCounts(item); }); lastRun = run; seen = Array.isArray(log) ? log.length : 0; }));
        render();
        return { render, syncNodes, group, bounds, select, toggle, createFromSelection, open, openList, move, visualRect, endpoint, edgeVisible, countText, collapsed, locked, get active() { return active; }, get invalid() { return invalid; }, get version() { return viewVersion; }, list: () => groups,
            clearActive() { active = null; },
            rects() { return ed.model.doc.nodes.filter(node => !memberOf.has(node.id)).map(node => canvas.nodeRect(node.id)).concat(groups.map(bounds)).filter(Boolean); },
            memberOf: id => memberOf.get(id), hidden: id => collapsed.has(memberOf.get(id)), movable: ids => Array.from(new Set(ids)).filter(id => !locked.has(memberOf.get(id))),
            expandFor(id) { const owner = memberOf.get(id); if (owner && collapsed.has(owner)) toggle(owner, 'collapse'); },
            dispose() { closed = true; if (inspector) inspector.close(); bag.dispose(); frames.remove(); heads.remove(); }
        };
    }
    ED.groups = { create };
})();
