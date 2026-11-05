// Pure reusable blocks: resource adapter, library and isolated draft simulation.
(function () {
    'use strict';
    const ED = window.EasyDrag = window.EasyDrag || {};
    const TYPES = { string: 'text', number: 'number', boolean: 'bool', object: 'object', array: 'list' };
    const clone = value => JSON.parse(JSON.stringify(value));
    function api(transport) {
        const base = '/api/desktop/flow-blocks';
        const enc = value => { if (!/^[A-Za-z0-9_-]{1,128}$/.test(String(value))) throw new Error('Invalid block identity'); return encodeURIComponent(value); };
        const path = id => base + '/' + enc(id);
        const json = (method, body) => ({ method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
        const normalize = res => Object.assign({}, res, { flow: res.block });
        return {
            list: (cursor, signal) => transport(base + '?limit=50' + (cursor ? '&cursor=' + encodeURIComponent(cursor) : ''), { signal }),
            create: body => transport(base, json('POST', body)).then(normalize),
            get: (id, signal) => transport(path(id), { signal }).then(normalize),
            save: (id, doc, revision) => transport(path(id), json('PUT', { doc, base_revision: revision })),
            preview: (id, signal) => transport(path(id) + '/publish-preview', { signal }),
            publish: (id, revision) => transport(path(id) + '/publish', json('POST', { base_revision: revision })).then(normalize),
            remove: (id, revision, liveRevision) => transport(path(id), json('DELETE', { base_revision: revision, base_live_revision: liveRevision })),
            versions: (id, before, signal) => transport(path(id) + '/versions?limit=50' + (before ? '&before_revision=' + enc(before) : ''), { signal }),
            version: (id, revision, signal) => transport(path(id) + '/versions/' + enc(revision), { signal }),
            dependents: (id, cursor, signal) => transport(path(id) + '/dependents?limit=100' + (cursor ? '&cursor=' + encodeURIComponent(cursor) : ''), { signal }),
            simulate: (id, body, signal) => transport(path(id) + '/simulate', Object.assign(json('POST', body), { signal })),
            nodeTypes: () => transport(base + '/node-types?lang=' + encodeURIComponent(ED.core.lang())),
            validate: (doc, mode) => transport(base + '/validate', json('POST', { doc, mode }))
        };
    }
    function contract(value) {
        if (!value || !Array.isArray(value.inputs) || !Array.isArray(value.outputs)) throw new Error('Missing block contract');
        for (const fields of [value.inputs, value.outputs]) {
            const seen = new Set();
            if (fields.length > 64) throw new Error('Invalid block contract');
            for (const field of fields) {
                if (!field || !/^[a-z][a-z0-9_]{0,39}$/.test(field.name) || seen.has(field.name) || !Object.prototype.hasOwnProperty.call(TYPES, field.type) || typeof field.required !== 'boolean' || typeof field.nullable !== 'boolean') throw new Error('Invalid block contract');
                seen.add(field.name);
            }
        }
        return clone(value);
    }
    function fieldsMarkup(ed, fields) {
        return '<ul class="ed-block-contract">' + fields.map(field => '<li><code>' + ed.esc(field.name) + '</code> <span>' + ed.esc(ed.t('easydrag.ui.type_' + TYPES[field.type])) + '</span>' + (field.required ? ' · ' + ed.esc(ed.t('easydrag.ui.block_required')) : '') + (field.nullable ? ' · null' : '') + '</li>').join('') + '</ul>';
    }
    function contractMarkup(ed, value) {
        if (!value) return '';
        return '<h4>' + ed.esc(ed.t('easydrag.ui.block_inputs')) + '</h4>' + fieldsMarkup(ed, value.inputs || []) + '<h4>' + ed.esc(ed.t('easydrag.ui.block_outputs')) + '</h4>' + fieldsMarkup(ed, value.outputs || []);
    }
    function outputMarkup(ed, outputs) {
        if (!outputs || typeof outputs !== 'object' || Array.isArray(outputs)) return '';
        return '<section class="ed-block-result"><h4>' + ed.esc(ed.t('easydrag.ui.block_outputs')) + '</h4><dl>' + Object.keys(outputs).slice(0, 64).map(name => {
            const value = outputs[name];
            let text = value === null ? 'null' : typeof value === 'string' ? value.slice(0, value.length > 1500 ? 1499 : 1500) : typeof value === 'object' ? ED.template.describe(value, ed.t, ED.core.fmt.number) : String(value);
            if (/[\uD800-\uDBFF]$/.test(text)) text = text.slice(0, -1);
            if (typeof value === 'string' && value.length > 1500) text += '…';
            return '<dt><code>' + ed.esc(name) + '</code></dt><dd>' + ed.esc(text) + '</dd>';
        }).join('') + '</dl></section>';
    }
    function dependentsMarkup(ed, items) {
        return '<ul class="ed-block-dependents">' + items.map(item => {
            const parent = item.kind === 'published' ? ed.t('easydrag.ui.state_published') + ' · ' + item.flow_id + ' · v' + item.revision : ed.t('easydrag.ui.runs_title') + ' · ' + (item.run_id || '—') + ' · ' + item.flow_id;
            return '<li>' + ed.esc(parent) + (Number.isInteger(item.block_revision) && item.block_revision > 0 ? '<br><span class="ed-hint">' + ed.esc(ed.t('easydrag.ui.block_revision') + ' · v' + item.block_revision) + '</span>' : '') + '</li>';
        }).join('') + '</ul>';
    }
    function library(app) {
        const core = ED.core, { t, esc } = app, bag = core.bag(), controller = new AbortController();
        let closed = false, seq = 0, denied = !!app.readonly, creating = false, rows = [], cursor = '', loading = false;
        const el = core.el('<div class="ed-home ed-block-library"><header class="ed-home-hero"><div class="ed-home-brand"><span class="ed-logo" aria-hidden="true">' + core.icon('grid') + '</span><div><h1>' + esc(t('easydrag.ui.blocks_title')) + '</h1><p>' + esc(t('easydrag.ui.blocks_hint')) + '</p></div></div><div class="ed-home-actions"><button type="button" class="ed-btn" data-ed-block-home>' + core.icon('arrow-left') + esc(t('easydrag.ui.home_flows')) + '</button><button type="button" class="ed-btn ed-btn--primary" data-ed-new>' + core.icon('plus') + esc(t('easydrag.ui.block_new')) + '</button></div></header><p class="ed-callout" role="status" data-ed-block-state></p><div class="ed-flow-grid" data-ed-block-list></div></div>');
        const more = core.el('<button type="button" class="ed-btn" data-ed-block-library-more hidden>' + esc(t('easydrag.ui.block_load_more')) + '</button>'); el.appendChild(more);
        function paint() {
            el.querySelector('[data-ed-new]').disabled = denied || creating;
            more.hidden = !cursor; more.disabled = loading;
            el.querySelector('[data-ed-block-list]').innerHTML = rows.map(row => '<article class="ed-flow-card"><button type="button" class="ed-block-open" data-ed-block-open="' + esc(row.id) + '"><h3>' + esc(row.name) + '</h3><span class="ed-chip">' + esc(row.live_revision ? 'v' + row.live_revision : t('easydrag.ui.state_draft')) + '</span><p class="ed-hint">' + esc(t('easydrag.ui.block_pure')) + '</p></button></article>').join('');
        }
        async function reload(append) {
            if (loading && append) return;
            const mine = ++seq, status = el.querySelector('[data-ed-block-state]');
            loading = true; more.disabled = true;
            status.hidden = false; status.textContent = t('easydrag.ui.loading');
            try {
                const res = await app.blockApi.list(append ? cursor : '', controller.signal);
                if (closed || mine !== seq) return;
                const loaded = append ? rows.concat(res.blocks || []) : (res.blocks || []);
                rows = Array.from(new Map(loaded.map(row => [row.id, row])).values()); cursor = res.next_cursor || ''; denied = !!app.readonly;
                status.hidden = rows.length > 0; status.textContent = t('easydrag.ui.blocks_empty');
            } catch (err) {
                if (closed || mine !== seq) return;
                denied = true; cursor = '';
                if ([401, 403].includes(err.status) || ['FLOWS_DISABLED', 'unauthorized', 'forbidden', 'desktop_scope_required'].includes(core.errorCode(err))) rows = [];
                status.textContent = core.errorText(t, err);
            }
            loading = false; paint();
        }
        function create() {
            if (closed || denied || creating) return;
            const dialog = core.modal(el, { title: t('easydrag.ui.block_new'), closeLabel: t('easydrag.ui.close'), className: 'ed-modal--block', body: '<label class="ed-label">' + esc(t('easydrag.ui.block_name')) + '<input class="ed-input" data-ed-block-name maxlength="120"></label><p class="ed-error" role="alert" hidden></p>', actions: [{ id: 'cancel', label: t('easydrag.ui.cancel') }, { id: 'create', label: t('easydrag.ui.block_new'), primary: true }], onAction: async (action, view) => {
                if (action !== 'create') return true;
                const name = view.body.querySelector('input').value.trim();
                if (!name || Array.from(name).length > 120 || closed || denied) return false;
                creating = true; paint();
                try { const res = await app.blockApi.create({ name }); if (!closed) app.openBlock(res.flow.id); return true; }
                catch (err) { if (!closed) { const box = view.body.querySelector('.ed-error'); box.hidden = false; box.textContent = core.errorText(t, err); } return false; }
                finally { creating = false; if (!closed) paint(); }
            } });
            return dialog;
        }
        bag.listen(el, 'click', event => {
            if (event.target.closest('[data-ed-block-library-more]') && cursor) reload(true);
            if (event.target.closest('[data-ed-block-home]')) app.openHome();
            if (event.target.closest('[data-ed-new]')) create();
            const button = event.target.closest('[data-ed-block-open]');
            if (button && rows.some(row => row.id === button.dataset.edBlockOpen)) app.openBlock(button.dataset.edBlockOpen);
        });
        bag.listen(document, 'aurago:flows-changed', () => reload());
        paint(); reload();
        return { el, reload, createOff: () => denied, dispose() { closed = true; seq++; controller.abort(); bag.dispose(); el.querySelectorAll('.ed-modal-backdrop').forEach(node => node.remove()); el.remove(); } };
    }
    function runController(ed, canvas) {
        const core = ED.core, bag = core.bag();
        let closed = false, busy = false, opening = false, generation = 0, dialog = null, request = null;
        const results = ED.testing.createResults(ed, canvas);
        async function startTest() {
            if (closed || ed.readonly || busy || opening || dialog || ed.flushPendingEdits() === false) return;
            opening = true;
            const mine = ++generation;
            const openingRequest = new AbortController(); request = openingRequest;
            ed.bus.emit('run', ed.run);
            try {
            if (!(await ed.saver.flush()) || closed || mine !== generation || ed.readonly) return;
            const observed = ed.model.version, revision = ed.saver.revision;
            let preview;
            try { preview = await ed.api.preview(ed.flow.id, openingRequest.signal); }
            catch (err) { if (!closed && mine === generation && err.name !== 'AbortError') ed.ctx.notify({ title: ed.model.doc.name, message: core.errorText(ed.t, err), type: 'error' }); return; }
            if (closed || mine !== generation || ed.readonly || ed.model.version !== observed || preview.draft_revision !== revision) return;
            let shape;
            try { shape = contract(preview.contract); } catch (_) { ed.ctx.notify({ title: ed.model.doc.name, message: ed.t('easydrag.ui.block_contract_unavailable'), type: 'error' }); return; }
            let form;
            dialog = core.modal(ed.root, { title: ed.t('easydrag.ui.block_simulate'), closeLabel: ed.t('easydrag.ui.close'), className: 'ed-modal--block', body: '<p class="ed-hint">' + ed.esc(ed.t('easydrag.ui.block_simulation_hint')) + '</p><div data-ed-block-inputs></div><p class="ed-error" role="alert" hidden></p>', actions: [{ id: 'cancel', label: ed.t('easydrag.ui.cancel') }, { id: 'simulate', label: ed.t('easydrag.ui.block_simulate'), primary: true }], onAction: async (action, view) => {
                if (action !== 'simulate') return true;
                if (closed || mine !== generation || ed.readonly || ed.model.version !== observed || ed.saver.revision !== revision || !form.flush()) return false;
                const inputs = form.values();
                try { ED.testing.parseSample(JSON.stringify(inputs)); } catch (_) { const error = view.body.querySelector('.ed-error'); error.hidden = false; error.textContent = ed.t('easydrag.ui.test_data_incomplete'); return false; }
                busy = true; const activeRequest = new AbortController(); request = activeRequest; ed.bus.emit('run', ed.run);
                try {
                    const res = await ed.api.simulate(ed.flow.id, { base_revision: revision, inputs }, activeRequest.signal);
                    if (closed || mine !== generation || ed.model.version !== observed || ed.saver.revision !== revision) return true;
                    const steps = Array.isArray(res.steps) ? res.steps : [];
                    ed.run = Object.assign({}, res, { mode: 'simulation', steps: new Map(steps.map(step => [step.node_id, step])) });
                    ed.lastRunData = { label: ed.t('easydrag.ui.simulation_title'), roots: Object.fromEntries(steps.filter(step => step.status === 'success').map(step => { const node = ed.model.node(step.node_id); return [node && node.key, step.output]; }).filter(pair => pair[0])) };
                    ed.testEvidence = { mode: 'simulation', revision, status: res.status }; ed.bus.emit('run', ed.run); return true;
                } catch (err) { if (!closed && mine === generation && err.name !== 'AbortError') { const error = view.body.querySelector('.ed-error'); error.hidden = false; error.textContent = core.errorText(ed.t, err); } return false; }
                finally { if (mine === generation) { busy = false; if (request === activeRequest) request = null; if (!closed) ed.bus.emit('run', ed.run); } }
            } });
            form = ED.blockFields.values({ ed, fields: shape.inputs, value: {}, templatable: false, requireComplete: true });
            dialog.body.querySelector('[data-ed-block-inputs]').appendChild(form.el);
            const owned = dialog; owned.done.then(() => { if (mine === generation) { generation++; opening = false; busy = false; if (request) request.abort(); request = null; if (!closed) ed.bus.emit('run', ed.run); } form.dispose(); if (dialog === owned) dialog = null; });
            } finally { if (mine === generation) { opening = false; if (request === openingRequest) request = null; if (!closed) ed.bus.emit('run', ed.run); } }
        }
        return { startTest, isRunning: () => busy || opening, drawerOpen: () => false, hasParked: () => false, loadLast() {}, runFinished() {}, clearRun() { ed.run = null; ed.bus.emit('run', null); }, cancel() { generation++; opening = false; busy = false; if (request) request.abort(); request = null; if (dialog) { dialog.close(null); dialog = null; } if (!closed) ed.bus.emit('run', ed.run); }, dispose() { closed = true; generation++; if (request) request.abort(); if (dialog) dialog.close(null); results.dispose(); bag.dispose(); } };
    }
    async function versions(ed, app) {
        const controller = new AbortController();
        const res = await ed.api.versions(ed.flow.id, 0, controller.signal);
        if (!ed.root.isConnected) return;
        let rows = res.versions || [], cursor = res.next_cursor;
        const dialog = ED.core.modal(ed.root, { title: ed.t('easydrag.ui.versions_title'), closeLabel: ed.t('easydrag.ui.close'), className: 'ed-modal--block', body: '<p class="ed-hint">' + ed.esc(ed.t('easydrag.ui.block_pins_hint')) + '</p><div class="ed-block-versions" data-ed-block-versions></div><button type="button" class="ed-btn" data-ed-block-versions-more>' + ed.esc(ed.t('easydrag.ui.block_load_more')) + '</button><p class="ed-error" role="alert" hidden></p>', actions: [{ id: 'close-dialog', label: ed.t('easydrag.ui.close') }] });
        function paint() { dialog.body.querySelector('[data-ed-block-versions]').innerHTML = rows.map(row => '<button type="button" class="ed-btn" data-ed-block-version="' + ed.esc(row.revision) + '">v' + ed.esc(row.revision) + ' · ' + ed.esc(ED.core.fmt.dateTime(row.published_at)) + '</button>').join(''); dialog.body.querySelector('[data-ed-block-versions-more]').hidden = !cursor; }
        dialog.body.addEventListener('click', async event => {
            const button = event.target.closest('[data-ed-block-version]');
            if (button && rows.some(row => String(row.revision) === button.dataset.edBlockVersion)) { dialog.close(null); app.openBlock(ed.flow.id, { revision: Number(button.dataset.edBlockVersion) }); return; }
            const more = event.target.closest('[data-ed-block-versions-more]'); if (!more || !cursor || more.disabled) return;
            more.disabled = true;
            try { const page = await ed.api.versions(ed.flow.id, cursor, controller.signal); if (!controller.signal.aborted) { rows = Array.from(new Map(rows.concat(page.versions || []).map(row => [row.revision, row])).values()); cursor = page.next_cursor; paint(); } }
            catch (err) { if (!controller.signal.aborted) { const error = dialog.body.querySelector('.ed-error'); error.hidden = false; error.textContent = ED.core.errorText(ed.t, err); } }
            finally { more.disabled = false; }
        });
        dialog.done.then(() => controller.abort()); paint();
        return dialog;
    }
    async function remove(ed, done) {
        if (ed.readonly || ed.flushPendingEdits() === false || !(await ed.saver.flush())) return;
        const revision = ed.saver.revision, liveRevision = ed.flow.live_revision || 0, modelVersion = ed.model.version;
        const controller = new AbortController();
        let page;
        try { page = await ed.api.dependents(ed.flow.id, '', controller.signal); }
        catch (err) { ed.ctx.notify({ title: ed.model.doc.name, message: ED.core.errorText(ed.t, err), type: 'error' }); return; }
        if (!ed.root.isConnected) return;
        let items = page.items || [], cursor = page.next_cursor;
        const dialog = ED.core.modal(ed.root, { title: ed.t('easydrag.ui.block_delete'), closeLabel: ed.t('easydrag.ui.close'), className: 'ed-modal--block', body: '<p>' + ed.esc(ed.t('easydrag.ui.block_delete_hint')) + '</p><div data-ed-block-dependents></div><button type="button" class="ed-btn" data-ed-block-more>' + ed.esc(ed.t('easydrag.ui.block_load_more')) + '</button><p class="ed-error" role="alert" hidden></p>', actions: [{ id: 'cancel', label: ed.t('easydrag.ui.cancel') }, { id: 'delete', label: ed.t('easydrag.ui.block_delete'), danger: true }], onAction: async (action, view) => {
            if (action !== 'delete') return true;
            if (!ed.root.isConnected || ed.readonly || items.length || ed.model.version !== modelVersion || ed.saver.revision !== revision) return false;
            try { await ed.api.remove(ed.flow.id, revision, liveRevision); if (ed.root.isConnected) done(); return true; }
            catch (err) { const box = view.body.querySelector('.ed-error'); box.hidden = false; box.textContent = ED.core.errorText(ed.t, err); return false; }
        } });
        function paint() {
            dialog.body.querySelector('[data-ed-block-dependents]').innerHTML = '<h4>' + ed.esc(ed.t('easydrag.ui.block_dependencies')) + '</h4>' + (!items.length ? '<p>' + ed.esc(ed.t('easydrag.ui.block_dependencies_none')) + '</p>' : dependentsMarkup(ed, items));
            dialog.body.querySelector('[data-ed-block-more]').hidden = !cursor;
            dialog.el.querySelector('[data-ed-action="delete"]').disabled = !!items.length || !!ed.readonly;
        }
        dialog.body.querySelector('[data-ed-block-more]').addEventListener('click', async event => {
            const button = event.currentTarget; button.disabled = true;
            try { const res = await ed.api.dependents(ed.flow.id, cursor, controller.signal); if (!controller.signal.aborted) { items = items.concat(res.items || []); cursor = res.next_cursor; paint(); } }
            catch (err) { if (!controller.signal.aborted) { const box = dialog.body.querySelector('.ed-error'); box.hidden = false; box.textContent = ED.core.errorText(ed.t, err); } }
            finally { button.disabled = false; }
        });
        dialog.done.then(() => controller.abort()); paint(); return dialog;
    }
    ED.blocks = { api, contract, types: TYPES, contractMarkup, outputMarkup, dependentsMarkup, library, runController, versions, remove };
})();
