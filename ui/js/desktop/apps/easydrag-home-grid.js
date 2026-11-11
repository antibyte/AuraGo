// EasyDrag start page grid: the flow cards (triggers, state and live badges, last run, favourite,
// switch and menu buttons), the stale and timer banners, the error, empty and no-match cards and
// the template cards. easydrag-home.js keeps the state, the requests and the event handlers.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};

    // create(host) returns {render} for one start page. host: {app, el, grid, tplGrid, previewSVG,
    // libraryControls(), organizationControls(), selectionControls(), createOff(), blockReason(),
    // metadataOf(flow), metadataMarkup(flow), state()}; state() returns the page's current values.
    function create(host) {
        const core = ED.core;
        const { app, el, grid, tplGrid, previewSVG, libraryControls, organizationControls, selectionControls, createOff, blockReason, metadataOf, metadataMarkup } = host;
        const { t, esc } = app;
        // spoken is the grid state announced last: speak says each state once.
        let spoken = '';
        // The page's state, read at the start of every render; the helpers below run only inside render().
        let flows, templates, filter, sortBy, query, templateQuery, templateCategory, prefs, favoritesOnly,
            organizationFolder, organizationTag, selectedFlows, loadError, staleError, timersRunning, templateError;
        function pull() {
            ({ flows, templates, filter, sortBy, query, templateQuery, templateCategory, prefs, favoritesOnly,
                organizationFolder, organizationTag, selectedFlows, loadError, staleError, timersRunning, templateError } = host.state());
        }

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
            return '<span class="ed-run-dot ed-run-dot--' + esc(r.status) + '"></span><span>' + esc(ED.durable ? ED.durable.label(t, r) : core.tr(t, 'easydrag.ui.status_' + r.status, r.status)) + ' · ' + esc(core.fmt.relative(r.started_at)) + (r.durable ? ' · ' + esc(t('easydrag.ui.durable_title')) : '') + '</span>';
        }

        function visibleFlows() {
            const q = query.trim().toLowerCase();
            const list = flows.filter(f => {
                const organization = metadataOf(f);
                if (organizationFolder === ':none' && organization.folder_id || organizationFolder && organizationFolder !== ':none' && organization.folder_id !== organizationFolder) return false;
                if (organizationTag && !(organization.tags || []).includes(organizationTag)) return false;
                if (favoritesOnly && !prefs.favorites.includes(f.id)) return false;
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
            if (filter !== 'all' || favoritesOnly || organizationFolder || organizationTag) actions += '<button type="button" class="ed-btn" data-ed-home-clear-filter>' + esc(t('easydrag.ui.home_clear_filter')) + '</button>';
            return '<div class="ed-home-empty"><p class="ed-hint">' + esc(t('easydrag.ui.home_no_match')) + '</p><div class="ed-empty-actions">' + actions + '</div></div>';
        }

        function speak(mode, text) {
            if (spoken === mode) return;
            spoken = mode;
            const live = el.querySelector('[data-ed-home-live]');
            if (live) live.textContent = text;
        }

        // cardName is a flow card's accessible name (U7): its name and the state the card shows
        // (state, Paused, a failed last run).
        function cardName(f) {
            const parts = [f.name, !f.published ? t('easydrag.ui.state_draft') : f.has_unpublished_changes ? t('easydrag.ui.state_changes') : t('easydrag.ui.state_published')];
            if (f.published && !f.enabled) parts.push(t('easydrag.ui.state_inactive'));
            if (f.last_run && f.last_run.status === 'error') parts.push(t('easydrag.ui.status_error'));
            return parts.join(', ');
        }

        function render() {
            pull();
            const list = visibleFlows();
            const visiblePublished = new Set(list.filter(flow => flow.published).map(flow => flow.id));
            for (const id of selectedFlows.keys()) if (!visiblePublished.has(id)) selectedFlows.delete(id);
            libraryControls();
            organizationControls();
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
                    '<article class="ed-flow-card' + (f.enabled ? ' is-active' : '') + (f.published && !off ? ' is-bulk-selectable' : '') + (selectedFlows.has(f.id) ? ' is-bulk-selected' : '') + '" data-ed-flow="' + esc(f.id) + '" tabindex="0" aria-label="' + esc(cardName(f)) + '">' +
                    (f.published && !off ? '<label class="ed-flow-select" data-ed-select-label="' + esc(f.id) + '"><input type="checkbox" data-ed-select-flow="' + esc(f.id) + '" aria-label="' + esc(t('easydrag.ui.home_bulk_select_flow', { name: f.name })) + '"' + (selectedFlows.has(f.id) ? ' checked' : '') + '><span class="ed-sr-only">' + esc(t('easydrag.ui.home_bulk_select_flow', { name: f.name })) + '</span></label>' : '') +
                    '<div class="ed-flow-preview">' + previewSVG(f.preview, esc, f.preview_edges) + '</div>' +
                    '<div class="ed-flow-info"><h3>' + esc(f.name) + '</h3><p class="ed-flow-triggers">' + core.icon('bolt') + '<span>' + esc(triggerLabels(f).join(' · ') || t('easydrag.ui.home_no_trigger')) + '</span></p>' +
                    '<div class="ed-flow-meta">' + statusBadge(f) + liveBadge(f) + '<span class="ed-flow-run">' + lastRun(f) + '</span></div>' + metadataMarkup(f) + '</div>' +
                    '<div class="ed-flow-card-actions">' +
                    '<button type="button" class="ed-icon-btn ed-flow-favorite" data-ed-favorite="' + esc(f.id) + '" aria-pressed="' + prefs.favorites.includes(f.id) + '" aria-label="' + esc(t(prefs.favorites.includes(f.id) ? 'easydrag.ui.library_favorite_remove' : 'easydrag.ui.library_favorite_add', { name: f.name })) + '" title="' + esc(t(prefs.favorites.includes(f.id) ? 'easydrag.ui.library_favorite_remove' : 'easydrag.ui.library_favorite_add', { name: f.name })) + '">' + core.icon('star') + '</button>' +
                    (f.published && !off ? '<button type="button" class="ed-switch ed-switch--small" role="switch" aria-checked="' + !!f.enabled + '" data-ed-toggle="' + esc(f.id) + '" aria-label="' + esc(t('easydrag.ui.home_active_named', { name: f.name })) + '" title="' + esc(t('easydrag.ui.active')) + '"><span></span></button>' : '') +
                    '<button type="button" class="ed-icon-btn" data-ed-card-menu="' + esc(f.id) + '" aria-label="' + esc(t('easydrag.ui.more')) + '">' + core.icon('dots') + '</button></div></article>').join('');
                speak(staleError ? 'stale' : 'ready', staleError ? t('easydrag.ui.home_stale_title') : t('easydrag.ui.home_flows'));
            }
            tplGrid.classList.toggle('is-disabled', off);
            // Templates that could not be loaded say so here, with their own Try again; the flows above stay.
            if (templateError && !templates.length && !loadError) {
                tplGrid.innerHTML = '<p class="ed-error">' + esc(t('easydrag.ui.home_templates_failed', { reason: core.errorText(t, templateError) })) + '</p>' +
                    '<div class="ed-row"><button type="button" class="ed-btn" data-ed-templates-retry>' + core.icon('refresh') + '<span>' + esc(t('easydrag.ui.retry')) + '</span></button></div>';
            } else tplGrid.innerHTML = templates.filter(tp => (!templateCategory || (tp.categories || []).includes(templateCategory)) &&
                (!templateQuery || (tp.name + ' ' + tp.description).toLocaleLowerCase().includes(templateQuery))).map(tp =>
                '<article class="ed-template-card" data-ed-template="' + esc(tp.id) + '" tabindex="' + (off ? '-1' : '0') + '" role="button"' + (off ? ' aria-disabled="true"' : '') + ' aria-label="' + esc(tp.name) + '">' +
                '<div class="ed-template-cats">' + (tp.categories || []).map(c => '<span class="ed-cat-dot" data-cat="' + esc(c) + '"></span>').join('') + '</div>' +
                '<h3>' + esc(tp.name) + '</h3><p>' + esc(tp.description) + '</p><dl class="ed-recipe-meta"><div><dt>' + esc(t('easydrag.ui.recipe_requirements')) + '</dt><dd>' + esc(t(tp.local_only ? 'easydrag.ui.recipe_local' : 'easydrag.ui.recipe_setup_review')) + '</dd></div>' +
                (tp.effects_known ? (tp.effects || []).length ? '<div><dt>' + esc(t('easydrag.ui.recipe_effects')) + '</dt><dd>' + (tp.effects || []).map(effect => esc(core.tr(t, 'easydrag.ui.effect_' + effect, effect))).join(' · ') + '</dd></div>' : '' : '<div><dd>' + esc(t('easydrag.ui.effects_unchecked')) + '</dd></div>') + '</dl><span class="ed-template-use">' + esc(t('easydrag.ui.home_use_template')) + core.icon('chevron-right') + '</span></article>').join('') || '<p class="ed-hint">' + esc(t('easydrag.ui.input_none')) + '</p>';
            el.querySelector('[data-ed-recipe-intro]').hidden = !!loadError || !templates.some(tp => tp.id === 'local_first_result' && tp.local_only);
            const categories = el.querySelector('[data-ed-template-category]');
            const categoryOptions = '<option value="">' + esc(t('easydrag.ui.template_all')) + '</option>' + Array.from(new Set(templates.flatMap(tp => tp.categories || []))).sort().map(c => '<option value="' + esc(c) + '">' + esc(core.tr(t, 'easydrag.category.' + c, c)) + '</option>').join('');
            if (categories.innerHTML !== categoryOptions) { categories.innerHTML = categoryOptions; categories.value = templateCategory; }
            el.querySelectorAll('[data-ed-new]').forEach(b => { b.disabled = off; });
            el.querySelectorAll('[data-ed-import], [data-ed-pack-import]').forEach(b => { b.disabled = off; });
            el.querySelectorAll('[data-ed-authoring]').forEach(b => { b.disabled = off; });
            el.querySelector('[data-ed-home-search]').disabled = !!loadError;
            const sortSel = el.querySelector('[data-ed-home-sort]');
            if (sortSel) sortSel.disabled = !!loadError;
            el.querySelectorAll('[data-ed-filter]').forEach(b => { b.disabled = !!loadError; });
            selectionControls();
        }

        return { render };
    }

    ED.homeGrid = { create };
})();
