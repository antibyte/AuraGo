// EasyDrag start page: flow cards with mini preview, search and filters, card menu,
// template gallery, import and "new flow".
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const FILTER_KEY = 'aurago.easydrag.home.filter';
    const VIEW_PREFIX = 'aurago.easydrag.view.';
    // IMPORT_MAX_BYTES is the server's request-body limit for flow documents (4 MiB). The document
    // itself may be at most 2 MiB (flows.MaxDocumentBytes); the server answers a larger one with
    // FLOW_TOO_LARGE.
    const IMPORT_MAX_BYTES = 4 * 1024 * 1024;

    function previewSVG(nodes, esc) {
        if (!nodes || !nodes.length) return '<svg viewBox="0 0 240 110" aria-hidden="true"><rect class="ed-mini-empty" x="94" y="39" width="52" height="32" rx="8"></rect></svg>';
        const xs = nodes.map(n => n.x);
        const ys = nodes.map(n => n.y);
        const minX = Math.min(...xs); const maxX = Math.max(...xs) + 232;
        const minY = Math.min(...ys); const maxY = Math.max(...ys) + 72;
        const scale = Math.min(220 / Math.max(maxX - minX, 1), 90 / Math.max(maxY - minY, 1), 0.35);
        const ox = (240 - (maxX - minX) * scale) / 2;
        const oy = (110 - (maxY - minY) * scale) / 2;
        const sorted = nodes.slice().sort((a, b) => a.x - b.x);
        const lines = sorted.slice(1).map((n, i) => {
            const p = sorted[i];
            const x1 = ox + (p.x - minX + 232) * scale; const y1 = oy + (p.y - minY + 36) * scale;
            const x2 = ox + (n.x - minX) * scale; const y2 = oy + (n.y - minY + 36) * scale;
            const dx = Math.max(6, (x2 - x1) / 2);
            return '<path d="M' + x1.toFixed(1) + ' ' + y1.toFixed(1) + ' C' + (x1 + dx).toFixed(1) + ' ' + y1.toFixed(1) + ' ' + (x2 - dx).toFixed(1) + ' ' + y2.toFixed(1) + ' ' + x2.toFixed(1) + ' ' + y2.toFixed(1) + '"></path>';
        }).join('');
        const rects = nodes.map(n => '<rect data-cat="' + esc(String(n.category).startsWith('tool:') ? 'tool' : n.category) + '" x="' + (ox + (n.x - minX) * scale).toFixed(1) + '" y="' + (oy + (n.y - minY) * scale).toFixed(1) +
            '" width="' + (232 * scale).toFixed(1) + '" height="' + (72 * scale).toFixed(1) + '" rx="' + (10 * scale + 2).toFixed(1) + '"></rect>').join('');
        return '<svg viewBox="0 0 240 110" aria-hidden="true"><g class="ed-mini-wires">' + lines + '</g>' + rects + '</svg>';
    }

    function create(app) {
        const core = ED.core;
        const { t, esc, ctx } = app;
        const bag = core.bag();
        let flows = [];
        let templates = [];
        let filter = core.storage.get(FILTER_KEY, 'all');
        let query = '';

        const el = core.el('<div class="ed-home">' +
            '<header class="ed-home-hero"><div class="ed-home-brand"><span class="ed-logo" aria-hidden="true">' + core.icon('git-branch') + '</span>' +
            '<div><h1>EasyDrag</h1><p>' + esc(t('easydrag.ui.home_tagline')) + '</p></div></div>' +
            '<div class="ed-home-actions"><button type="button" class="ed-btn" data-ed-import>' + core.icon('upload') + '<span>' + esc(t('easydrag.ui.home_import')) + '</span></button>' +
            '<button type="button" class="ed-btn ed-btn--primary" data-ed-new>' + core.icon('plus') + '<span>' + esc(t('easydrag.ui.home_new')) + '</span></button>' +
            '<input type="file" accept=".json,.easydrag.json,application/json" hidden data-ed-import-file></div></header>' +
            '<div class="ed-home-toolbar"><div class="ed-search">' + core.icon('search') + '<input type="search" placeholder="' + esc(t('easydrag.ui.home_search')) + '" aria-label="' + esc(t('easydrag.ui.home_search')) + '" enterkeyhint="search" inputmode="search" data-ed-home-search></div>' +
            '<div class="ed-seg ed-seg--small" role="radiogroup" aria-label="' + esc(t('easydrag.ui.home_filter')) + '">' +
            ['all', 'active', 'inactive', 'errors'].map(f => '<button type="button" role="radio" data-ed-filter="' + f + '" aria-checked="' + (f === filter) + '">' + esc(t('easydrag.ui.home_filter_' + f)) + '</button>').join('') + '</div></div>' +
            '<section class="ed-home-section"><h2>' + esc(t('easydrag.ui.home_flows')) + '</h2><div class="ed-flow-grid" aria-live="polite"></div></section>' +
            '<section class="ed-home-section"><h2>' + esc(t('easydrag.ui.home_templates')) + '</h2><p class="ed-hint">' + esc(t('easydrag.ui.home_templates_hint')) + '</p><div class="ed-template-grid"></div></section>' +
            '</div>');
        const grid = el.querySelector('.ed-flow-grid');
        const tplGrid = el.querySelector('.ed-template-grid');

        function triggerLabels(f) {
            const seen = new Set();
            return (f.triggers || []).map(type => (app.catalog && app.catalog.types.get(type) || {}).label || type).filter(l => !seen.has(l) && seen.add(l));
        }

        function statusBadge(f) {
            if (!f.published) return '<span class="ed-chip ed-chip--muted">' + esc(t('easydrag.ui.state_draft')) + '</span>';
            if (f.has_unpublished_changes) return '<span class="ed-chip ed-chip--accent">' + esc(t('easydrag.ui.state_changes')) + '</span>';
            return '<span class="ed-chip ed-chip--ok">' + esc(t('easydrag.ui.state_published')) + '</span>';
        }

        function lastRun(f) {
            const r = f.last_run;
            if (!r) return '<span class="ed-muted">' + esc(t('easydrag.ui.home_never_ran')) + '</span>';
            return '<span class="ed-run-dot ed-run-dot--' + esc(r.status) + '"></span><span>' + esc(core.tr(t, 'easydrag.ui.status_' + r.status, r.status)) + ' · ' + esc(core.fmt.relative(r.started_at)) + '</span>';
        }

        function visibleFlows() {
            const q = query.trim().toLowerCase();
            return flows.filter(f => {
                if (q && !String(f.name).toLowerCase().includes(q) && !String(f.description || '').toLowerCase().includes(q)) return false;
                if (filter === 'active') return f.enabled;
                if (filter === 'inactive') return !f.enabled;
                if (filter === 'errors') return f.last_run && f.last_run.status === 'error';
                return true;
            });
        }

        // loadError is the error of the last failed list request. While it is set the flow grid shows
        // it (a calm lock card for FLOWS_DISABLED) instead of cards that may be gone, and search and
        // filters wait; the templates still show.
        let loadError = null;

        // errorCard offers the actions of the window's lock card (easydrag.js showError): "Open
        // settings" for FLOWS_DISABLED, and "Try again" (a new list request) for every error.
        function errorCard(err) {
            const retry = '<button type="button" class="ed-btn ed-btn--primary" data-ed-home-retry>' + core.icon('refresh') + '<span>' + esc(t('easydrag.ui.retry')) + '</span></button>';
            return core.errorCode(err) === 'FLOWS_DISABLED'
                ? '<div class="ed-home-empty">' + core.icon('lock') + '<h3>' + esc(t('easydrag.ui.disabled_title')) + '</h3><p>' + esc(t('easydrag.ui.disabled_text')) + '</p>' +
                    '<div class="ed-row"><button type="button" class="ed-btn" data-ed-home-settings>' + core.icon('settings') + '<span>' + esc(t('easydrag.ui.open_settings')) + '</span></button>' + retry + '</div></div>'
                : '<p class="ed-error">' + esc(core.errorText(t, err)) + '</p><div class="ed-row ed-home-retry">' + retry + '</div>';
        }

        // createOff: nothing can be created, the desktop is read-only or flows are switched off.
        function createOff() { return !!app.readonly || core.errorCode(loadError) === 'FLOWS_DISABLED'; }

        function render() {
            const list = visibleFlows();
            const off = createOff();
            if (loadError) {
                grid.innerHTML = errorCard(loadError);
            } else if (!flows.length) {
                grid.innerHTML = '<div class="ed-home-empty">' + core.icon('sparkles') + '<h3>' + esc(t('easydrag.ui.home_empty_title')) + '</h3><p>' + esc(t('easydrag.ui.home_empty_text')) + '</p>' +
                    '<button type="button" class="ed-btn ed-btn--primary" data-ed-new>' + core.icon('plus') + '<span>' + esc(t('easydrag.ui.home_new')) + '</span></button></div>';
            } else if (!list.length) {
                grid.innerHTML = '<p class="ed-hint">' + esc(t('easydrag.ui.home_no_match')) + '</p>';
            } else {
                grid.innerHTML = list.map(f =>
                    '<article class="ed-flow-card' + (f.enabled ? ' is-active' : '') + '" data-ed-flow="' + esc(f.id) + '" tabindex="0" aria-label="' + esc(f.name) + '">' +
                    '<div class="ed-flow-preview">' + previewSVG(f.preview, esc) + '</div>' +
                    '<div class="ed-flow-info"><h3>' + esc(f.name) + '</h3><p class="ed-flow-triggers">' + core.icon('bolt') + '<span>' + esc(triggerLabels(f).join(' · ') || t('easydrag.ui.home_no_trigger')) + '</span></p>' +
                    '<div class="ed-flow-meta">' + statusBadge(f) + '<span class="ed-flow-run">' + lastRun(f) + '</span></div></div>' +
                    '<div class="ed-flow-card-actions">' +
                    (f.published && !app.readonly ? '<button type="button" class="ed-switch ed-switch--small" role="switch" aria-checked="' + !!f.enabled + '" data-ed-toggle="' + esc(f.id) + '" aria-label="' + esc(t('easydrag.ui.active')) + '" title="' + esc(t('easydrag.ui.active')) + '"><span></span></button>' : '') +
                    '<button type="button" class="ed-icon-btn" data-ed-card-menu="' + esc(f.id) + '" aria-label="' + esc(t('easydrag.ui.more')) + '">' + core.icon('dots') + '</button></div></article>').join('');
            }
            tplGrid.classList.toggle('is-disabled', off);
            tplGrid.innerHTML = templates.map(tp =>
                '<article class="ed-template-card" data-ed-template="' + esc(tp.id) + '" tabindex="' + (off ? '-1' : '0') + '" role="button"' + (off ? ' aria-disabled="true"' : '') + ' aria-label="' + esc(tp.name) + '">' +
                '<div class="ed-template-cats">' + (tp.categories || []).map(c => '<span class="ed-cat-dot" data-cat="' + esc(c) + '"></span>').join('') + '</div>' +
                '<h3>' + esc(tp.name) + '</h3><p>' + esc(tp.description) + '</p><span class="ed-template-use">' + esc(t('easydrag.ui.home_use_template')) + core.icon('chevron-right') + '</span></article>').join('');
            el.querySelectorAll('[data-ed-new]').forEach(b => { b.disabled = off; });
            el.querySelector('[data-ed-import]').disabled = off;
            el.querySelector('[data-ed-home-search]').disabled = !!loadError;
            el.querySelectorAll('[data-ed-filter]').forEach(b => { b.disabled = !!loadError; });
        }

        // loadSeq numbers the list requests: only the newest answer is shown, none after dispose.
        let loadSeq = 0;

        // reload fetches the flow list (and the templates once). A failure is shown and not retried
        // by itself: the next flows_changed or the user (Reload) asks again. Flows switched off
        // (FLOWS_DISABLED, possible at any time) show a calm state instead of an error.
        async function reload() {
            reloadSoon.cancel();
            const mine = ++loadSeq;
            let list;
            let tpl;
            try {
                [list, tpl] = await Promise.all([app.api.list(), templates.length ? Promise.resolve({ templates }) : app.api.templates()]);
            } catch (err) {
                if (mine !== loadSeq) return;
                flows = [];
                loadError = err;
                render();
                return;
            }
            if (mine !== loadSeq) return;
            loadError = null;
            flows = list.flows || [];
            templates = tpl.templates || [];
            render();
        }
        // Every autosave broadcasts flows_changed "saved", and listing parses every flow on the
        // server: those refreshes wait until saving pauses for 1.5 s.
        const reloadSoon = core.debounce(reload, 1500);

        // retry is "Try again" on an error card. The new grid replaces the button, so the focus goes
        // to the new "Try again" or, once the flows are back, to the first card. It moves only when
        // it was lost with the old button (on the body, or on the removed button): never away from
        // a control the user moved to meanwhile, here or in another window.
        async function retry() {
            await reload();
            const active = document.activeElement;
            if (active && active !== document.body && active.isConnected !== false) return;
            const next = grid.querySelector('[data-ed-home-retry]') || grid.querySelector('[data-ed-flow]') || el.querySelector('[data-ed-new]');
            if (next && !next.disabled) next.focus();
        }

        // creating allows one create at a time (a held Enter on a template card, a double click).
        let creating = false;

        async function create(body) {
            if (creating) return;
            creating = true;
            try {
                const res = await app.api.create(body);
                app.openFlow(res.flow.id);
            } catch (err) {
                ctx.notify({ title: 'EasyDrag', message: core.errorText(t, err), type: 'error' });
            } finally {
                creating = false;
            }
        }

        async function newFlow() {
            if (app.readonly) return;
            const name = await ctx.promptDialog(t('easydrag.ui.home_new_title'), t('easydrag.ui.new_flow_name'));
            if (name === null || name === undefined) return;
            create({ name: String(name).trim() || t('easydrag.ui.new_flow_name') });
        }

        // importFile reads a flow file; one over the server's request-body limit is not read at all.
        async function importFile(file) {
            if (Number(file.size) > IMPORT_MAX_BYTES) {
                ctx.notify({ title: t('easydrag.ui.home_import'), message: t('easydrag.ui.error_flow_too_large'), type: 'error' });
                return;
            }
            let doc;
            try { doc = JSON.parse(await file.text()); } catch (err) {
                ctx.notify({ title: t('easydrag.ui.home_import'), message: t('easydrag.ui.import_invalid'), type: 'error' });
                return;
            }
            create({ import: doc });
        }

        function download(id, name) {
            const a = document.createElement('a');
            a.href = app.api.exportUrl(id);
            a.download = (name || 'flow') + '.easydrag.json';
            document.body.appendChild(a);
            a.click();
            a.remove();
        }

        async function duplicate(id) {
            try {
                const res = await app.api.get(id);
                const doc = res.flow.draft;
                doc.name = t('easydrag.ui.copy_of', { name: doc.name });
                create({ import: doc });
            } catch (err) { ctx.notify({ title: 'EasyDrag', message: core.errorText(t, err), type: 'error' }); }
        }

        async function remove(f) {
            const ok = await ctx.confirmDialog(t('easydrag.ui.delete_title'), t('easydrag.ui.delete_text', { name: f.name }));
            if (!ok) return;
            try { await app.api.remove(f.id); } catch (err) { ctx.notify({ title: 'EasyDrag', message: core.errorText(t, err), type: 'error' }); return; }
            // The flow's local leftovers go with it: its emergency copy and its viewport.
            ED.saver.dropEmergencyCopy(f.id);
            core.storage.remove(VIEW_PREFIX + f.id);
            reload();
        }

        async function toggle(f, button) {
            const on = button.getAttribute('aria-checked') !== 'true';
            button.setAttribute('aria-checked', String(on));
            try { await app.api.setEnabled(f.id, on); f.enabled = on; render(); } catch (err) {
                button.setAttribute('aria-checked', String(!on));
                ctx.notify({ title: f.name, message: core.errorText(t, err), type: 'error' });
            }
        }

        function cardMenu(f, x, y) {
            if (typeof ctx.showContextMenu !== 'function') return;
            const ro = !!app.readonly;
            ctx.showContextMenu(x, y, [
                { icon: 'edit', label: t('easydrag.ui.home_open'), action: () => app.openFlow(f.id) },
                { icon: 'play', label: t('easydrag.ui.run_now'), disabled: ro || !f.published, action: async () => { try { await app.api.runNow(f.id); ctx.notify({ title: f.name, message: t('easydrag.ui.run_started') }); } catch (err) { ctx.notify({ title: f.name, message: core.errorText(t, err), type: 'error' }); } } },
                { icon: 'copy', label: t('easydrag.ui.home_duplicate'), disabled: ro, action: () => duplicate(f.id) },
                { icon: 'download', label: t('easydrag.ui.home_export'), action: () => download(f.id, f.name) },
                { icon: 'list', label: t('easydrag.ui.home_mission_control'), action: () => ctx.openApp && ctx.openApp('mission-control') },
                { separator: true },
                { icon: 'trash', label: t('easydrag.ui.home_delete'), disabled: ro, action: () => remove(f) }
            ]);
        }

        bag.listen(el, 'click', (event) => {
            if (event.target.closest('[data-ed-home-settings]')) { core.openFlowSettings(); return; }
            if (event.target.closest('[data-ed-home-retry]')) { retry(); return; }
            if (event.target.closest('[data-ed-new]')) { newFlow(); return; }
            if (event.target.closest('[data-ed-import]')) { el.querySelector('[data-ed-import-file]').click(); return; }
            const f = event.target.closest('[data-ed-filter]');
            if (f) {
                filter = f.dataset.edFilter;
                core.storage.set(FILTER_KEY, filter);
                el.querySelectorAll('[data-ed-filter]').forEach(b => b.setAttribute('aria-checked', String(b === f)));
                render();
                return;
            }
            const tg = event.target.closest('[data-ed-toggle]');
            if (tg) { event.stopPropagation(); toggle(flows.find(x => x.id === tg.dataset.edToggle), tg); return; }
            const menu = event.target.closest('[data-ed-card-menu]');
            if (menu) { const r = menu.getBoundingClientRect(); cardMenu(flows.find(x => x.id === menu.dataset.edCardMenu), r.left, r.bottom); return; }
            const tpl = event.target.closest('[data-ed-template]');
            if (tpl) { if (!createOff()) create({ template: tpl.dataset.edTemplate }); return; }
            const card = event.target.closest('[data-ed-flow]');
            if (card) app.openFlow(card.dataset.edFlow);
        });
        bag.listen(el, 'keydown', (event) => {
            if (event.repeat || (event.key !== 'Enter' && event.key !== ' ')) return;
            const card = event.target.closest('[data-ed-flow], [data-ed-template]');
            if (!card || event.target !== card) return;
            event.preventDefault();
            card.click();
        });
        bag.listen(el, 'contextmenu', (event) => {
            const card = event.target.closest('[data-ed-flow]');
            if (!card) return;
            event.preventDefault();
            cardMenu(flows.find(x => x.id === card.dataset.edFlow), event.clientX, event.clientY);
        });
        bag.listen(el.querySelector('[data-ed-home-search]'), 'input', core.debounce(event => { query = event.target.value; render(); }, 120));
        bag.listen(el.querySelector('[data-ed-import-file]'), 'change', (event) => {
            const file = event.target.files && event.target.files[0];
            event.target.value = '';
            if (file) importFile(file);
        });
        bag.listen(document, 'aurago:flows-changed', (event) => {
            const d = event.detail || {};
            if (d.reason === 'saved') reloadSoon(); else reload();
        });
        bag.add(() => { loadSeq++; reloadSoon.cancel(); });

        render();
        reload();
        return { el, reload, dispose() { bag.dispose(); el.remove(); } };
    }

    ED.home = { create, previewSVG };
})();
