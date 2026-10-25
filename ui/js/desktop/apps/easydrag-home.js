// EasyDrag start page: flow cards with mini preview, search and filters, card menu,
// template gallery, import and "new flow".
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const FILTER_KEY = 'aurago.easydrag.home.filter';
    // IMPORT_MAX_BYTES is the server's request-body limit for flow documents (4 MiB). The document
    // itself may be at most 2 MiB (flows.MaxDocumentBytes); the server answers a larger one with
    // FLOW_TOO_LARGE.
    const IMPORT_MAX_BYTES = 4 * 1024 * 1024;

    function previewSVG(nodes, esc, edges) {
        if (!nodes || !nodes.length) return '<svg viewBox="0 0 240 110" aria-hidden="true"><rect class="ed-mini-empty" x="94" y="39" width="52" height="32" rx="8"></rect></svg>';
        const xs = nodes.map(n => n.x);
        const ys = nodes.map(n => n.y);
        const minX = Math.min(...xs); const maxX = Math.max(...xs) + 232;
        const minY = Math.min(...ys); const maxY = Math.max(...ys) + 72;
        const scale = Math.min(220 / Math.max(maxX - minX, 1), 90 / Math.max(maxY - minY, 1), 0.35);
        const ox = (240 - (maxX - minX) * scale) / 2;
        const oy = (110 - (maxY - minY) * scale) / 2;
        const pathBetween = (p, n) => {
            const x1 = ox + (p.x - minX + 232) * scale; const y1 = oy + (p.y - minY + 36) * scale;
            const x2 = ox + (n.x - minX) * scale; const y2 = oy + (n.y - minY + 36) * scale;
            const dx = Math.max(6, Math.abs(x2 - x1) / 2);
            return '<path d="M' + x1.toFixed(1) + ' ' + y1.toFixed(1) + ' C' + (x1 + dx).toFixed(1) + ' ' + y1.toFixed(1) + ' ' + (x2 - dx).toFixed(1) + ' ' + y2.toFixed(1) + ' ' + x2.toFixed(1) + ' ' + y2.toFixed(1) + '"></path>';
        };
        let lines = '';
        if (Array.isArray(edges) && edges.length) {
            lines = edges.map(pair => {
                const a = Array.isArray(pair) ? nodes[pair[0]] : null;
                const b = Array.isArray(pair) ? nodes[pair[1]] : null;
                return a && b ? pathBetween(a, b) : '';
            }).join('');
        } else {
            const sorted = nodes.slice().sort((a, b) => a.x - b.x);
            lines = sorted.slice(1).map((n, i) => pathBetween(sorted[i], n)).join('');
        }
        const rects = nodes.map(n => '<rect data-cat="' + esc(String(n.category).startsWith('tool:') ? 'tool' : n.category) + '" x="' + (ox + (n.x - minX) * scale).toFixed(1) + '" y="' + (oy + (n.y - minY) * scale).toFixed(1) +
            '" width="' + (232 * scale).toFixed(1) + '" height="' + (72 * scale).toFixed(1) + '" rx="' + (10 * scale + 2).toFixed(1) + '"></rect>').join('');
        return '<svg viewBox="0 0 240 110" aria-hidden="true"><g class="ed-mini-wires">' + lines + '</g>' + rects + '</svg>';
    }

    // create builds the start page. opts.focusFlow: the flow the editor just left; its card takes
    // the focus once the list is loaded, if New flow still has it (showHome focused New flow).
    function create(app, opts) {
        const core = ED.core;
        const { t, esc, ctx } = app;
        const bag = core.bag();
        let flows = [];
        let templates = [];
        let filter = core.storage.get(FILTER_KEY, 'all');
        let sortBy = 'updated';
        let query = '';
        let focusFlow = (opts && opts.focusFlow) || '';

        const el = core.el('<div class="ed-home">' +
            '<header class="ed-home-hero"><div class="ed-home-brand"><span class="ed-logo" aria-hidden="true">' + core.icon('git-branch') + '</span>' +
            '<div><h1>EasyDrag</h1><p>' + esc(t('easydrag.ui.home_tagline')) + '</p></div></div>' +
            '<div class="ed-home-actions"><button type="button" class="ed-btn" data-ed-import>' + core.icon('upload') + '<span>' + esc(t('easydrag.ui.home_import')) + '</span></button>' +
            '<button type="button" class="ed-btn ed-btn--primary" data-ed-new>' + core.icon('plus') + '<span>' + esc(t('easydrag.ui.home_new')) + '</span></button>' +
            '<input type="file" accept=".json,.easydrag.json,application/json" hidden data-ed-import-file></div></header>' +
            '<div class="ed-home-toolbar"><div class="ed-search">' + core.icon('search') + '<input type="search" placeholder="' + esc(t('easydrag.ui.home_search')) + '" aria-label="' + esc(t('easydrag.ui.home_search')) + '" enterkeyhint="search" inputmode="search" data-ed-home-search></div>' +
            '<div class="ed-seg ed-seg--small" role="radiogroup" aria-label="' + esc(t('easydrag.ui.home_filter')) + '">' +
            ['all', 'active', 'inactive', 'drafts', 'errors'].map(f => '<button type="button" role="radio" data-ed-filter="' + f + '" aria-checked="' + (f === filter) + '">' + esc(t('easydrag.ui.home_filter_' + f)) + '</button>').join('') + '</div>' +
            '<select class="ed-select ed-home-sort" aria-label="' + esc(t('easydrag.ui.home_sort')) + '" data-ed-home-sort>' +
            '<option value="updated">' + esc(t('easydrag.ui.home_sort_updated')) + '</option>' +
            '<option value="last_run">' + esc(t('easydrag.ui.home_sort_last_run')) + '</option>' +
            '<option value="name">' + esc(t('easydrag.ui.home_sort_name')) + '</option></select></div>' +
            '<section class="ed-home-section"><h2>' + esc(t('easydrag.ui.home_flows')) + '</h2><div class="ed-flow-grid"></div></section>' +
            '<section class="ed-home-section"><h2>' + esc(t('easydrag.ui.home_templates')) + '</h2><p class="ed-hint">' + esc(t('easydrag.ui.home_templates_hint')) + '</p><div class="ed-template-grid"></div></section>' +
            '<div class="ed-sr-only" aria-live="polite" data-ed-home-live></div>' +
            '</div>');
        const grid = el.querySelector('.ed-flow-grid');
        const tplGrid = el.querySelector('.ed-template-grid');
        // The grid is no live region (every refresh would read all cards); meaningful changes
        // are announced on their own (a deleted flow).
        const voice = core.announcer(el.querySelector('[data-ed-home-live]'));

        function triggerLabels(f) {
            const seen = new Set();
            return (f.triggers || []).map(type => (app.catalog && app.catalog.types.get(type) || {}).label || type).filter(l => !seen.has(l) && seen.add(l));
        }

        // statusBadge uses the words of Mission Control: state_draft reads "Not published yet".
        function statusBadge(f) {
            if (!f.published) return '<span class="ed-chip ed-chip--muted">' + esc(t('easydrag.ui.state_draft')) + '</span>';
            if (f.has_unpublished_changes) return '<span class="ed-chip ed-chip--accent">' + esc(t('easydrag.ui.state_changes')) + '</span>';
            return '<span class="ed-chip ed-chip--ok">' + esc(t('easydrag.ui.state_published')) + '</span>';
        }

        // liveBadge: the published version no longer passes the server's current checks (its
        // start-up check); publishing again clears it.
        function liveBadge(f) {
            if (!f.published || !Array.isArray(f.live_issues) || !f.live_issues.length) return '';
            const hint = t('easydrag.ui.live_outdated_hint');
            return '<span class="ed-chip ed-chip--risk" data-ed-live-outdated title="' + esc(hint) + '">' + esc(t('easydrag.ui.state_live_outdated')) +
                '<span class="ed-sr-only"> ' + esc(hint) + '</span></span>';
        }

        function lastRun(f) {
            const r = f.last_run;
            if (!r) return '<span class="ed-muted">' + esc(t('easydrag.ui.home_never_ran')) + '</span>';
            return '<span class="ed-run-dot ed-run-dot--' + esc(r.status) + '"></span><span>' + esc(core.tr(t, 'easydrag.ui.status_' + r.status, r.status)) + ' · ' + esc(core.fmt.relative(r.started_at)) + '</span>';
        }

        function visibleFlows() {
            const q = query.trim().toLowerCase();
            const list = flows.filter(f => {
                // Names, descriptions and trigger labels already on the card. No catalog refetch.
                if (q && !String(f.name).toLowerCase().includes(q) && !String(f.description || '').toLowerCase().includes(q) &&
                    !triggerLabels(f).some(label => String(label).toLowerCase().includes(q))) return false;
                if (filter === 'active') return f.enabled;
                // "Paused" (the stored filter id stays "inactive"): published and switched off. A
                // flow that was never published is not paused; it shows under All.
                if (filter === 'inactive') return f.published && !f.enabled;
                if (filter === 'drafts') return !f.published || !!f.has_unpublished_changes;
                // Cancelled is not an error. Only last_run.status "error" counts.
                if (filter === 'errors') return f.last_run && f.last_run.status === 'error';
                return true;
            });
            if (sortBy === 'name') {
                list.sort((a, b) => String(a.name || '').localeCompare(String(b.name || '')));
            } else if (sortBy === 'last_run') {
                list.sort((a, b) => {
                    const ta = a.last_run && a.last_run.started_at ? Date.parse(a.last_run.started_at) || 0 : 0;
                    const tb = b.last_run && b.last_run.started_at ? Date.parse(b.last_run.started_at) || 0 : 0;
                    return tb - ta;
                });
            }
            return list;
        }

        // loadError replaces the grid: first load never succeeded, or a later answer withdrew the
        // list (FLOWS_DISABLED, or the session can no longer read). staleError keeps the last list
        // in memory only. Nothing is written to storage for either.
        let loadError = null;
        let staleError = null;
        let timersRunning = true;  // GET /flows timers_running; false while Date & time triggers do not fire
        let listed = false;
        let spoken = '';

        function errorIdentity(err) {
            const body = err && err.body || {};
            return String(body.code || body.error || '').toLowerCase();
        }

        // authLoss: the session or the read scope is gone. Every 401 and every 403 counts,
        // whatever its code. desktop_readonly is the exception: the list stays, changes do not.
        function authLoss(err) {
            if (!err) return false;
            const status = Number(err.status) || 0;
            const code = errorIdentity(err);
            if (code === 'desktop_readonly') return false;
            if (status === 401 || status === 403) return true;
            return code === 'unauthorized' || code === 'forbidden' || code === 'permission_denied' || code === 'desktop_scope_required';
        }

        function writesRevoked(err) { return errorIdentity(err) === 'desktop_readonly'; }

        // createOff: no create, import, duplicate, delete, toggle or run. Read-only open stays.
        function createOff() {
            return !!app.readonly || !!staleError || core.errorCode(loadError) === 'FLOWS_DISABLED' || authLoss(loadError) || writesRevoked(loadError);
        }

        function blockReason() {
            if (!staleError) return '';
            return t('easydrag.ui.home_stale_locked', { reason: core.errorText(t, staleError) });
        }

        // errorCard offers the actions of the window's lock card (easydrag.js showError): "Open
        // settings" for FLOWS_DISABLED, and "Try again" (a new list request) for every error.
        function errorCard(err) {
            const retry = '<button type="button" class="ed-btn ed-btn--primary" data-ed-home-retry>' + core.icon('refresh') + '<span>' + esc(t('easydrag.ui.retry')) + '</span></button>';
            return core.errorCode(err) === 'FLOWS_DISABLED'
                ? '<div class="ed-home-empty" role="status">' + core.icon('lock') + '<h3>' + esc(t('easydrag.ui.disabled_title')) + '</h3><p>' + esc(t('easydrag.ui.disabled_text')) + '</p>' +
                    '<div class="ed-row"><button type="button" class="ed-btn" data-ed-home-settings>' + core.icon('settings') + '<span>' + esc(t('easydrag.ui.open_settings')) + '</span></button>' + retry + '</div></div>'
                : '<p class="ed-error" role="alert">' + esc(core.errorText(t, err)) + '</p><div class="ed-row ed-home-retry">' + retry + '</div>';
        }

        function staleBanner() {
            return '<div class="ed-home-stale" role="status"><h3>' + esc(t('easydrag.ui.home_stale_title')) + '</h3><p>' +
                esc(t('easydrag.ui.home_stale_text', { reason: core.errorText(t, staleError) })) + '</p>' +
                '<button type="button" class="ed-btn" data-ed-home-retry>' + core.icon('refresh') + '<span>' + esc(t('easydrag.ui.retry')) + '</span></button></div>';
        }

        // timersBanner: Date & time triggers do not fire (the server keeps retrying to start).
        function timersBanner() {
            if (timersRunning) return '';
            return '<div class="ed-home-stale" role="status" data-ed-timers-off><h3>' + esc(t('easydrag.ui.timers_stopped_title')) + '</h3><p>' +
                esc(t('easydrag.ui.timers_stopped_text')) + '</p></div>';
        }

        function emptyCard() {
            const off = createOff();
            const dis = off ? ' disabled' : '';
            const why = off && blockReason() ? ' title="' + esc(blockReason()) + '"' : '';
            return '<div class="ed-home-empty"><h3>' + esc(t('easydrag.ui.home_empty_title')) + '</h3><p>' + esc(t('easydrag.ui.home_empty_text')) + '</p>' +
                '<div class="ed-empty-actions">' +
                '<button type="button" class="ed-btn" data-ed-home-templates' + dis + why + '>' + core.icon('sparkles') + '<span>' + esc(t('easydrag.ui.home_empty_template')) + '</span></button>' +
                '<button type="button" class="ed-btn ed-btn--primary" data-ed-new' + dis + why + '>' + core.icon('plus') + '<span>' + esc(t('easydrag.ui.home_empty_create')) + '</span></button>' +
                '<button type="button" class="ed-btn" data-ed-import' + dis + why + '>' + core.icon('upload') + '<span>' + esc(t('easydrag.ui.home_import')) + '</span></button>' +
                '</div></div>';
        }

        function noMatchCard() {
            let actions = '';
            if (query.trim()) actions += '<button type="button" class="ed-btn" data-ed-home-clear-search>' + esc(t('easydrag.ui.home_clear_search')) + '</button>';
            if (filter !== 'all') actions += '<button type="button" class="ed-btn" data-ed-home-clear-filter>' + esc(t('easydrag.ui.home_clear_filter')) + '</button>';
            return '<div class="ed-home-empty"><p class="ed-hint">' + esc(t('easydrag.ui.home_no_match')) + '</p><div class="ed-empty-actions">' + actions + '</div></div>';
        }

        function speak(mode, text) {
            if (spoken === mode) return;
            spoken = mode;
            const live = el.querySelector('[data-ed-home-live]');
            if (live) live.textContent = text;
        }

        // reportLoaded tells the shell the list has settled, so the window menu can follow createOff.
        function reportLoaded() {
            if (opts && typeof opts.onLoaded === 'function') opts.onLoaded();
        }

        function render() {
            const list = visibleFlows();
            const off = createOff();
            const banner = (!loadError && staleError ? staleBanner() : '') + (!loadError ? timersBanner() : '');
            if (loadError) {
                grid.innerHTML = errorCard(loadError);
                speak(core.errorCode(loadError) === 'FLOWS_DISABLED' ? 'disabled' : 'error', core.errorCode(loadError) === 'FLOWS_DISABLED' ? t('easydrag.ui.disabled_title') : core.errorText(t, loadError));
            } else if (!flows.length) {
                grid.innerHTML = banner + emptyCard();
                speak(staleError ? 'stale-empty' : 'empty', staleError ? t('easydrag.ui.home_stale_title') : t('easydrag.ui.home_empty_title'));
            } else if (!list.length) {
                grid.innerHTML = banner + noMatchCard();
                speak(staleError ? 'stale-none' : 'none', t('easydrag.ui.home_no_match'));
            } else {
                grid.innerHTML = banner + list.map(f =>
                    '<article class="ed-flow-card' + (f.enabled ? ' is-active' : '') + '" data-ed-flow="' + esc(f.id) + '" tabindex="0" aria-label="' + esc(f.name) + '">' +
                    '<div class="ed-flow-preview">' + previewSVG(f.preview, esc, f.preview_edges) + '</div>' +
                    '<div class="ed-flow-info"><h3>' + esc(f.name) + '</h3><p class="ed-flow-triggers">' + core.icon('bolt') + '<span>' + esc(triggerLabels(f).join(' · ') || t('easydrag.ui.home_no_trigger')) + '</span></p>' +
                    '<div class="ed-flow-meta">' + statusBadge(f) + liveBadge(f) + '<span class="ed-flow-run">' + lastRun(f) + '</span></div></div>' +
                    '<div class="ed-flow-card-actions">' +
                    (f.published && !off ? '<button type="button" class="ed-switch ed-switch--small" role="switch" aria-checked="' + !!f.enabled + '" data-ed-toggle="' + esc(f.id) + '" aria-label="' + esc(t('easydrag.ui.active')) + '" title="' + esc(t('easydrag.ui.active')) + '"><span></span></button>' : '') +
                    '<button type="button" class="ed-icon-btn" data-ed-card-menu="' + esc(f.id) + '" aria-label="' + esc(t('easydrag.ui.more')) + '">' + core.icon('dots') + '</button></div></article>').join('');
                speak(staleError ? 'stale' : 'ready', staleError ? t('easydrag.ui.home_stale_title') : t('easydrag.ui.home_flows'));
            }
            tplGrid.classList.toggle('is-disabled', off);
            tplGrid.innerHTML = templates.map(tp =>
                '<article class="ed-template-card" data-ed-template="' + esc(tp.id) + '" tabindex="' + (off ? '-1' : '0') + '" role="button"' + (off ? ' aria-disabled="true"' : '') + ' aria-label="' + esc(tp.name) + '">' +
                '<div class="ed-template-cats">' + (tp.categories || []).map(c => '<span class="ed-cat-dot" data-cat="' + esc(c) + '"></span>').join('') + '</div>' +
                '<h3>' + esc(tp.name) + '</h3><p>' + esc(tp.description) + '</p><span class="ed-template-use">' + esc(t('easydrag.ui.home_use_template')) + core.icon('chevron-right') + '</span></article>').join('');
            el.querySelectorAll('[data-ed-new]').forEach(b => { b.disabled = off; });
            el.querySelectorAll('[data-ed-import]').forEach(b => { b.disabled = off; });
            el.querySelector('[data-ed-home-search]').disabled = !!loadError;
            const sortSel = el.querySelector('[data-ed-home-sort]');
            if (sortSel) sortSel.disabled = !!loadError;
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
                // A refresh that fails keeps the last authoritative list and blocks changes.
                // The first load, FLOWS_DISABLED and a lost session do not keep protected cards.
                if (authLoss(err)) {
                    flows = [];
                    templates = [];
                    listed = false;
                    staleError = null;
                    loadError = err;
                } else if (core.errorCode(err) === 'FLOWS_DISABLED' || !listed) {
                    flows = [];
                    listed = false;
                    staleError = null;
                    loadError = err;
                } else {
                    staleError = err;
                    loadError = null;
                }
                render();
                reportLoaded();
                return;
            }
            if (mine !== loadSeq) return;
            loadError = null;
            staleError = null;
            listed = true;
            flows = list.flows || [];
            timersRunning = list.timers_running !== false;
            templates = tpl.templates || [];
            render();
            reportLoaded();
            focusLeftFlow();
        }
        // Every autosave broadcasts flows_changed "saved", and every live run's end "run_finished";
        // listing parses every flow on the server: those refreshes wait until they pause for 1.5 s.
        const reloadSoon = core.debounce(reload, 1500);

        // focusLeftFlow moves the focus to the card of the flow the editor just left, once, and only
        // from New flow (where showHome put it) or from nowhere: never away from a control the user
        // chose meanwhile.
        function focusLeftFlow() {
            const id = focusFlow;
            focusFlow = '';
            if (!id) return;
            const card = Array.from(grid.querySelectorAll('[data-ed-flow]')).find(c => c.dataset.edFlow === id);
            const active = document.activeElement;
            const add = el.querySelector('.ed-home-hero [data-ed-new]');
            if (card && (!active || active === document.body || active === add)) card.focus();
        }

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
        // gone is dispose. A confirmation, file read or duplicate fetch can outlive the authority
        // it started with, so the mutation checks again at the request.
        let creating = false;
        let gone = false;
        const runsPending = new Set();
        function mayChange() { return !gone && !createOff(); }

        async function create(body) {
            if (!mayChange() || creating) return;
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
            if (createOff()) return;
            const name = await ctx.promptDialog(t('easydrag.ui.home_new_title'), t('easydrag.ui.new_flow_name'));
            if (name === null || name === undefined) return;
            create({ name: String(name).trim() || t('easydrag.ui.new_flow_name') });
        }

        // importFile reads a flow file; one over the server's request-body limit is not read at all.
        async function importFile(file) {
            if (!mayChange() || !file) return;
            if (Number(file.size) > IMPORT_MAX_BYTES) {
                ctx.notify({ title: t('easydrag.ui.home_import'), message: t('easydrag.ui.error_flow_too_large'), type: 'error' });
                return;
            }
            let doc;
            try { doc = JSON.parse(await file.text()); } catch (err) {
                if (!gone) ctx.notify({ title: t('easydrag.ui.home_import'), message: t('easydrag.ui.import_invalid'), type: 'error' });
                return;
            }
            if (!mayChange()) return;
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
            if (!mayChange()) return;
            try {
                const res = await app.api.get(id);
                if (!mayChange()) return;
                const doc = res.flow.draft;
                doc.name = t('easydrag.ui.copy_of', { name: doc.name });
                create({ import: doc });
            } catch (err) {
                if (!gone) ctx.notify({ title: 'EasyDrag', message: core.errorText(t, err), type: 'error' });
            }
        }

        async function remove(f) {
            if (!mayChange() || !f) return;
            const ok = await ctx.confirmDialog(t('easydrag.ui.delete_title'), t('easydrag.ui.delete_text', { name: f.name }));
            if (!ok || !mayChange()) return;
            try { await app.api.remove(f.id); } catch (err) { if (!gone) ctx.notify({ title: 'EasyDrag', message: core.errorText(t, err), type: 'error' }); return; }
            // The flow's local leftovers go with it: its emergency copy, its view, its confirmed
            // test effects and its last test trigger.
            core.forgetFlow(f.id);
            voice.announce(t('easydrag.ui.home_flow_deleted', { name: f.name }));
            reload();
        }

        // runNow allows one request per flow until that request settles. A failure is not sent again.
        async function runNow(f) {
            if (!mayChange() || !f || !f.published || !f.enabled || runsPending.has(f.id)) return;
            runsPending.add(f.id);
            try {
                await app.api.runNow(f.id);
                if (!gone) ctx.notify({ title: f.name, message: t('easydrag.ui.run_started') });
            } catch (err) {
                if (!gone) ctx.notify({ title: f.name, message: core.errorText(t, err), type: 'error' });
            } finally {
                runsPending.delete(f.id);
            }
        }

        async function toggle(f, button) {
            if (createOff() || !f || !button) return;
            const on = button.getAttribute('aria-checked') !== 'true';
            button.setAttribute('aria-checked', String(on));
            try { await app.api.setEnabled(f.id, on); f.enabled = on; render(); } catch (err) {
                button.setAttribute('aria-checked', String(!on));
                ctx.notify({ title: f.name, message: core.errorText(t, err), type: 'error' });
            }
        }

        function cardMenu(f, x, y) {
            if (!f || typeof ctx.showContextMenu !== 'function') return;
            const locked = createOff();
            // Run now needs a published flow that is switched on (a paused one is refused).
            const paused = !!f.published && !f.enabled;
            const why = blockReason();
            ctx.showContextMenu(x, y, [
                { icon: 'edit', label: t('easydrag.ui.home_open'), action: () => app.openFlow(f.id) },
                { icon: 'play', label: t('easydrag.ui.run_now'), disabled: locked || !f.published || paused || runsPending.has(f.id), disabledHint: why || (paused && !app.readonly ? t('easydrag.ui.error_flow_disabled') : ''), action: () => runNow(f) },
                { icon: 'copy', label: t('easydrag.ui.home_duplicate'), disabled: locked, disabledHint: why, action: () => duplicate(f.id) },
                { icon: 'download', label: t('easydrag.ui.home_export'), action: () => download(f.id, f.name) },
                { icon: 'list', label: t('easydrag.ui.home_mission_control'), action: () => ctx.openApp && ctx.openApp('mission-control') },
                { separator: true },
                { icon: 'trash', label: t('easydrag.ui.home_delete'), disabled: locked, disabledHint: why, action: () => remove(f) }
            ]);
        }

        // focusTemplates moves to the existing template section. It never creates a flow.
        function focusTemplates() {
            const section = tplGrid.parentElement;
            if (section && typeof section.scrollIntoView === 'function') section.scrollIntoView({ block: 'start' });
            const head = section && section.querySelector('h2');
            const card = !createOff() ? tplGrid.querySelector('[data-ed-template]') : null;
            const target = card || head || section;
            if (target === head && head && typeof head.setAttribute === 'function') head.setAttribute('tabindex', '-1');
            if (target && typeof target.focus === 'function') target.focus();
        }

        function clearSearch() {
            query = '';
            const input = el.querySelector('[data-ed-home-search]');
            if (input) {
                input.value = '';
                if (typeof input.focus === 'function') input.focus();
            }
            render();
        }

        function clearFilter() {
            filter = 'all';
            core.storage.set(FILTER_KEY, filter);
            el.querySelectorAll('[data-ed-filter]').forEach(b => b.setAttribute('aria-checked', String(b.dataset.edFilter === 'all')));
            render();
            const all = el.querySelector('[data-ed-filter="all"]');
            if (all && typeof all.focus === 'function') all.focus();
        }

        bag.listen(el, 'click', (event) => {
            if (event.target.closest('[data-ed-home-settings]')) { core.openFlowSettings(); return; }
            if (event.target.closest('[data-ed-home-retry]')) { retry(); return; }
            if (event.target.closest('[data-ed-home-templates]')) { focusTemplates(); return; }
            if (event.target.closest('[data-ed-home-clear-search]')) { clearSearch(); return; }
            if (event.target.closest('[data-ed-home-clear-filter]')) { clearFilter(); return; }
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
        bag.listen(el.querySelector('[data-ed-home-sort]'), 'change', (event) => {
            sortBy = event.target.value || 'updated';
            render();
        });
        bag.listen(el.querySelector('[data-ed-import-file]'), 'change', (event) => {
            const file = event.target.files && event.target.files[0];
            event.target.value = '';
            if (file) importFile(file);
        });
        bag.listen(el, 'dragover', (event) => {
            if (createOff()) return;
            const dt = event.dataTransfer;
            if (!dt || !Array.from(dt.types || []).includes('Files')) return;
            event.preventDefault();
            dt.dropEffect = 'copy';
            el.classList.add('is-drop-target');
        });
        bag.listen(el, 'dragleave', (event) => {
            if (!el.contains(event.relatedTarget)) el.classList.remove('is-drop-target');
        });
        bag.listen(el, 'drop', (event) => {
            el.classList.remove('is-drop-target');
            if (createOff()) return;
            const file = event.dataTransfer && event.dataTransfer.files && event.dataTransfer.files[0];
            if (!file) return;
            event.preventDefault();
            importFile(file);
        });
        bag.listen(document, 'aurago:flows-changed', (event) => {
            const d = event.detail || {};
            // A flow deleted elsewhere leaves nothing behind in this browser either.
            if (d.reason === 'deleted' && d.flow_id) core.forgetFlow(d.flow_id);
            if (d.reason === 'saved' || d.reason === 'run_finished') reloadSoon(); else reload();
        });
        bag.add(() => { loadSeq++; reloadSoon.cancel(); voice.cancel(); });

        render();
        reload();
        return { el, reload, createOff, dispose() { gone = true; bag.dispose(); el.remove(); } };
    }

    ED.home = { create, previewSVG };
})();
