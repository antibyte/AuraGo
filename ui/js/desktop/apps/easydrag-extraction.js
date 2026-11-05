// Selection extraction owns review and acknowledgement of one atomic server transaction.
(function () {
    'use strict';
    const ED = window.EasyDrag = window.EasyDrag || {};
    const REASONS = new Set(['selection', 'boundary', 'node', 'branch', 'fields', 'reference', 'taint', 'groups', 'budget']);
    const NODE = /^n_[a-z2-7]{8}$/, EDGE = /^e_[a-z2-7]{8}$/, FLOW = /^flow_[a-z2-7]{26}$/, HASH = /^[a-f0-9]{64}$/;
    const object = value => !!value && typeof value === 'object' && !Array.isArray(value);
    function reviewFields(fields) {
        return Array.isArray(fields) && fields.length <= 64 && fields.every(field => object(field) && typeof field.name === 'string' && (/^[a-z][a-z0-9_]{0,39}$/.test(field.name) || field.name.includes('[redacted]') && ED.metadata.textFits(field.name, 400, 1600)) && Object.hasOwn(ED.blocks.types, field.type) && typeof field.required === 'boolean' && typeof field.nullable === 'boolean');
    }

    function validPreview(value, revision) {
        if (!object(value) || value.base_revision !== revision || value.block_revision !== 1 || !object(value.ids) || !FLOW.test(value.ids.block_id) || !HASH.test(value.digest) || !HASH.test(value.parent_hash) || !HASH.test(value.block_hash)) return false;
        if (!['call_id', 'input_id', 'output_id'].every(key => NODE.test(value.ids[key])) || !['entry_edge_id', 'exit_edge_id'].every(key => EDGE.test(value.ids[key]))) return false;
        if (new Set(Object.values(value.ids)).size !== 6) return false;
        const r = value.review;
        return object(r) && reviewFields(r.inputs) && reviewFields(r.outputs) && ['groups_moved', 'groups_shrunk', 'extra_nodes', 'extra_edges'].every(key => Number.isInteger(r[key]) && r[key] >= 0 && r[key] <= 2000) && typeof r.budget_note === 'boolean';
    }

    // Keep the acknowledged list order as well as normal per-ID change descriptors. The
    // latter drive existing canvas/contract observers; the metadata carries exact ordering.
    function replaceParent(model, doc, version, label) {
        if (model.version !== version || !object(doc) || !Array.isArray(doc.nodes) || !Array.isArray(doc.edges) || doc.nodes.length > 500 || doc.edges.length > 2000) return false;
        const next = ED.model.create(doc, { types: new Map() }).toJSON();
        if (next.nodes.length !== doc.nodes.length || next.edges.length !== doc.edges.length || new Set(next.nodes.map(n => n.id)).size !== next.nodes.length || new Set(next.edges.map(e => e.id)).size !== next.edges.length || !ED.model.validGroups(next.groups, new Set(next.nodes.map(n => n.id)))) return false;
        const patch = {};
        new Set(Object.keys(model.doc).concat(Object.keys(next))).forEach(key => { patch[key] = next[key]; });
        patch.groups = next.groups == null ? [] : next.groups;
        model.change(label, rec => {
            model.doc.edges.forEach(edge => rec.removeEdge(edge.id));
            model.doc.nodes.forEach(node => rec.removeNode(node.id));
            next.nodes.forEach(node => rec.addNode(node));
            next.edges.forEach(edge => rec.addEdge(edge));
            rec.meta(patch);
        });
        return true;
    }

    function create(ed, canvas) {
        const core = ED.core, { t, esc } = ed, bag = core.bag();
        let dialog = null, request = null, disposed = false, opening = false, pending = null;
        const banner = core.el('<div class="ed-runview-banner ed-extraction-status" data-ed-extraction-status role="status" hidden><span></span><button type="button" class="ed-btn ed-btn--small" data-ed-extract-check></button><button type="button" class="ed-btn ed-btn--small" data-ed-extract-recover></button></div>');
        ed.root.insertBefore(banner, ed.root.querySelector('.ed-body'));
        const allowed = () => !disposed && ed.root.isConnected && !ed.readonly && !ed.runView && !ed.isBlock && ed.model === ed.draftModel;
        const selection = () => Array.from(ed.selection).filter(id => ed.model.node(id)).sort();
        const sameSelection = ids => JSON.stringify(ids) === JSON.stringify(selection());
        function paintBanner() {
            banner.hidden = !pending;
            if (!pending) return;
            banner.querySelector('span').textContent = t('easydrag.ui.extraction_unknown');
            banner.querySelector('[data-ed-extract-check]').textContent = t('easydrag.ui.extraction_check');
            banner.querySelector('[data-ed-extract-recover]').textContent = t('easydrag.ui.extraction_recover');
            banner.querySelectorAll('button').forEach(button => { button.disabled = !allowed() || !!request; });
        }
        function error(view, text) {
            if (disposed || !view || !view.el.isConnected || view.el.closest('.is-closing')) return;
            const node = view.body.querySelector('.ed-error'); node.hidden = !text; node.textContent = text || '';
        }
        function failed(view, err) {
            const reason = core.errorCode(err) === 'FLOW_EXTRACTION_UNSUPPORTED' && err.body && err.body.reason;
            error(view, REASONS.has(reason) ? t('easydrag.ui.extraction_reason_' + reason) : core.errorText(t, err));
        }
        function current(p) {
            return allowed() && ed.model === p.model && ed.model.version === p.version && ed.saver.revision === p.revision;
        }
        function accepted(result, p) {
            const flow = result && result.flow, block = result && result.block;
            return object(flow) && flow.id === ed.flow.id && flow.draft_revision === p.revision + 1 && object(flow.draft) && object(block) && block.id === p.preview.ids.block_id && block.live_revision === 1 && Array.isArray(flow.draft.nodes) && flow.draft.nodes.some(n => n && n.id === p.preview.ids.call_id && n.type === 'component.call' && n.params && n.params.flow_id === block.id && n.params.revision === 1) && !flow.draft.nodes.some(n => p.ids.includes(n.id));
        }
        async function adopt(result, p) {
            if (!accepted(result, p)) throw new Error('Invalid extraction acknowledgement');
            if (!current(p)) return false;
            const ok = ed.saver.acknowledgeExternal(p.ticket, result.flow.draft_revision, () => replaceParent(p.model, result.flow.draft, p.version, t('easydrag.ui.extraction_title')), result.issues || []);
            if (!ok) return false;
            pending = null; paintBanner();
            Object.assign(ed.flow, result.flow);
            ed.selection.clear(); ed.selection.add(p.preview.ids.call_id); ed.selectedEdge = null;
            ed.bus.emit('selection');
            ed.bus.emit('issues', result.issues || []);
            canvas.ensureSelected();
            ed.ctx.notify({ title: t('easydrag.ui.extraction_title'), message: t('easydrag.ui.extraction_success') });
            return true;
        }
        async function recover() {
            if (!pending || request || !allowed()) return false;
            const p = pending;
            if (!await ed.ctx.confirmDialog(t('easydrag.ui.extraction_recover'), t('easydrag.ui.extraction_recover_hint')) || disposed || pending !== p || !allowed()) return false;
            if (dialog) dialog.close();
            // The existing CAS choice owns subsequent retries. This is never another Apply.
            const transferred = await ed.saver.recoverExternal(p.ticket);
            if (transferred && pending === p) pending = null;
            paintBanner();
            return transferred;
        }
        async function reconcile(view) {
            if (!pending || request || !allowed()) return;
            const p = pending, control = request = new AbortController(); paintBanner();
            try {
                const result = await ed.api.extractReconcile(ed.flow.id, { base_revision: p.revision, block_id: p.preview.ids.block_id, parent_hash: p.preview.parent_hash, block_hash: p.preview.block_hash }, control.signal);
                if (disposed || control.signal.aborted || pending !== p) return;
                if (result && result.status === 'applied_state_confirmed') {
                    if (await adopt(result, p)) { if (dialog) dialog.close(); }
                    else error(view, t('easydrag.ui.extraction_conflict'));
                } else error(view, t('easydrag.ui.extraction_unknown'));
            } catch (err) { if (!control.signal.aborted) failed(view, err); }
            finally { if (request === control) request = null; paintBanner(); }
        }

        async function open() {
            if (!allowed() || dialog || opening) return;
            opening = true;
            try {
                if (!pending && (!ed.flushPendingEdits() || !await ed.saver.flush())) return;
                if (!allowed() || dialog) return;
                const model = ed.model, version = model.version, revision = ed.saver.revision, ids = selection();
                if (!pending && (!ids.length || ids.length > 500)) return;
                let preview = null, previewName = '', composing = false;
                const selectedNodes = (pending ? pending.ids.map(id => pending.model.node(id)) : ids.map(id => model.node(id))).filter(Boolean);
                const selectedMarkup = selectedNodes.map(node => '<li><strong>' + esc(String(node.label || (ed.catalog.types.get(node.type) || {}).label || node.type).slice(0, 240)) + '</strong> <code>' + esc(node.key) + '</code></li>').join('');
                const owned = dialog = core.modal(ed.root, {
                    title: t('easydrag.ui.extraction_title'), closeLabel: t('easydrag.ui.close'), className: 'ed-modal--extraction',
                    body: '<p>' + esc(t('easydrag.ui.extraction_intro')) + '</p><h3 data-ed-extract-count>' + esc(t('easydrag.ui.extraction_selected', { count: pending ? pending.ids.length : ids.length })) + '</h3><ol class="ed-extraction-selection">' + selectedMarkup + '</ol><label class="ed-label">' + esc(t('easydrag.ui.block_name')) + '<input class="ed-input" data-ed-extract-name></label><p class="ed-error" role="alert" hidden></p><section data-ed-extract-review hidden></section>',
                    actions: [{ id: 'cancel', label: t('easydrag.ui.close') }, { id: 'preview', label: t('easydrag.ui.extraction_preview'), primary: true }, { id: 'apply', label: t('easydrag.ui.extraction_apply'), primary: true }, { id: 'reconcile', label: t('easydrag.ui.extraction_check'), primary: true }, { id: 'recover', label: t('easydrag.ui.extraction_recover') }],
                    onAction: async action => {
                        if (action === 'cancel') return true;
                        if (action === 'reconcile') { await reconcile(owned); return false; }
                        if (action === 'recover') { await recover(); return false; }
                        if (request || pending || !allowed()) return false;
                        error(owned, '');
                        const name = input.value.trim();
                        if (composing || !name || !ED.metadata.textFits(name, 80, 320)) { error(owned, t('easydrag.ui.extraction_name_invalid')); return false; }
                        if (ed.model !== model || model.version !== version || ed.saver.revision !== revision || !sameSelection(ids) || !ed.flushPendingEdits() || model.version !== version) { error(owned, t('easydrag.ui.extraction_stale')); return false; }
                        const control = request = new AbortController(); paint();
                        try {
                            if (action === 'preview') {
                                const result = await ed.api.extractPreview(ed.flow.id, { base_revision: revision, node_ids: ids, name }, control.signal);
                                if (disposed || control.signal.aborted || dialog !== owned) return false;
                                if (!allowed() || model.version !== version || ed.saver.revision !== revision || !sameSelection(ids) || composing || input.value.trim() !== name) { error(owned, t('easydrag.ui.extraction_stale')); return false; }
                                if (!validPreview(result, revision)) throw new Error('Invalid extraction preview');
                                preview = result; previewName = name;
                                const region = owned.body.querySelector('[data-ed-extract-review]'); region.hidden = false; region.innerHTML = reviewMarkup(result);
                            } else if (action === 'apply' && preview && previewName === name) {
                                const ticket = ed.saver.beginExternal(revision, version);
                                if (!ticket) { error(owned, t('easydrag.ui.extraction_stale')); return false; }
                                const p = pending = { model, version, revision, ids, name, preview, ticket };
                                paintBanner();
                                const result = await ed.api.extractApply(ed.flow.id, { base_revision: revision, node_ids: ids, name, ids: preview.ids, digest: preview.digest }, control.signal);
                                if (disposed || control.signal.aborted || pending !== p) return false;
                                if (await adopt(result, p)) return true;
                                error(owned, t('easydrag.ui.extraction_conflict'));
                            }
                        } catch (err) {
                            if (disposed || control.signal.aborted) return false;
                            // Only a definitive coded client refusal proves no commit. Network,
                            // server failures and malformed success replies remain uncertain.
                            const status = Number(err && err.status);
                            if (action === 'apply' && pending && status >= 400 && status < 500 && ![408, 429].includes(status) && core.errorCode(err)) {
                                ed.saver.releaseExternal(pending.ticket); pending = null; preview = null;
                            }
                            if (pending) error(owned, t('easydrag.ui.extraction_unknown')); else failed(owned, err);
                        } finally {
                            if (request === control) request = null;
                            paintBanner(); paint();
                        }
                        return false;
                    }
                });
                const input = owned.body.querySelector('[data-ed-extract-name]'); input.value = pending ? pending.name : t('easydrag.ui.extraction_default_name');
                function paint() {
                    if (dialog !== owned) return;
                    input.readOnly = !!pending || !!request;
                    ['preview', 'apply', 'reconcile', 'recover'].forEach(id => {
                        const b = owned.el.querySelector('[data-ed-action="' + id + '"]');
                        b.hidden = id === 'preview' ? !!pending || !!preview : id === 'apply' ? !!pending || !preview : !pending;
                        b.disabled = !allowed() || !!request;
                    });
                }
                owned.setBusy = paint;
                input.addEventListener('compositionstart', () => { composing = true; });
                input.addEventListener('compositionend', () => { composing = false; });
                input.addEventListener('input', () => { preview = null; owned.body.querySelector('[data-ed-extract-review]').hidden = true; paint(); });
                if (pending) error(owned, t('easydrag.ui.extraction_unknown'));
                paint();
                owned.done.then(() => {
                    if (dialog !== owned) return;
                    dialog = null;
                    if (request) { request.abort(); request = null; }
                    paintBanner();
                });
            } finally { opening = false; }
        }
        function reviewMarkup(preview) {
            const r = preview.review;
            return ED.blocks.contractMarkup(ed, r) + (r.inputs.concat(r.outputs).some(field => field.name.includes('[redacted]')) ? '<p class="ed-hint">' + esc(t('easydrag.ui.metadata_hidden')) + '</p>' : '') + '<p>' + esc(t('easydrag.ui.extraction_groups', { moved: r.groups_moved, shrunk: r.groups_shrunk })) + '</p><p class="ed-callout">' + esc(t('easydrag.ui.extraction_budget', { nodes: r.extra_nodes, edges: r.extra_edges })) + '</p><p class="ed-hint">' + esc(t('easydrag.ui.extraction_undo_hint')) + '</p>';
        }
        bag.add(ed.bus.on('extract-selection', open));
        bag.listen(banner.querySelector('[data-ed-extract-check]'), 'click', open);
        bag.listen(banner.querySelector('[data-ed-extract-recover]'), 'click', recover);
        return {
            open, pending: () => !!pending,
            leave: () => pending ? ed.ctx.confirmDialog(t('easydrag.ui.leave_title'), t('easydrag.ui.extraction_leave')) : Promise.resolve(true),
            dispose() { disposed = true; if (request) request.abort(); if (dialog) dialog.close(); dialog = null; bag.dispose(); banner.remove(); }
        };
    }
    ED.extraction = { create, validPreview, replaceParent };
})();
