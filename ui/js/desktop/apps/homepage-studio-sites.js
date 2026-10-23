/* Homepage Studio sites panel: managed-site list with drift badges, site
   detail (deploy targets, deployments, remote observations) and on-demand
   reconcile. Data comes from /api/desktop/integrations/homepage/sites + /api/desktop/integrations/homepage/sites/{id}. */
(function () {
    'use strict';

    const DRIFT_STATUSES = ['clean', 'local_changed', 'remote_changed', 'remote_unknown', 'not_deployed'];
    const DRIFT_KEYS = {
        clean: 'clean',
        local_changed: 'local-changed',
        remote_changed: 'remote-changed',
        remote_unknown: 'remote-unknown',
        not_deployed: 'not-deployed'
    };

    function driftStatus(status) {
        return DRIFT_STATUSES.includes(status) ? status : 'not_deployed';
    }

    /* Deployment target collection: the sites panel owns the deployment data,
       so it also owns the provider normalisation and the target list. */
    function firstString(...values) {
        for (const value of values) {
            if (typeof value === 'string' && value.trim()) return value;
        }
        return '';
    }

    function normalizeTargetProvider(provider) {
        const value = firstString(provider).toLowerCase();
        if (value === 'homepage') return 'local';
        if (value === 'sftp' || value === 'scp' || value === 'ssh') return 'remote';
        return value || 'remote';
    }

    function addTarget(targets, provider, url, label, previewURL) {
        const normalizedProvider = normalizeTargetProvider(provider);
        const normalizedURL = previewURL(url);
        if (!normalizedProvider || !normalizedURL) return;
        const key = normalizedProvider + '\n' + normalizedURL;
        if (targets.some(item => item.key === key)) return;
        targets.push({
            key,
            provider: normalizedProvider,
            url: normalizedURL,
            label: firstString(label, normalizedProvider)
        });
    }

    function collectTargets(site, targets, previewURL) {
        if (!site || typeof site !== 'object') return;
        addTarget(targets, 'remote', site.last_deploy_url, site.name || site.project_dir, previewURL);
        const deployTargets = Array.isArray(site.deploy_targets) ? site.deploy_targets : [];
        for (const target of deployTargets) {
            const provider = normalizeTargetProvider(target && target.provider);
            const label = firstString(target && target.provider_target_id, site.name, site.project_dir, provider);
            const url = previewURL(target && target.url, target && target.remote_path);
            addTarget(targets, provider, url, label, previewURL);
        }
        const deployments = Array.isArray(site.deployments) ? site.deployments : [];
        for (const deployment of deployments) {
            const provider = normalizeTargetProvider(deployment && deployment.provider);
            const label = firstString(deployment && deployment.provider_deploy_id, site.name, site.project_dir, provider);
            addTarget(targets, provider, deployment && deployment.url, label, previewURL);
        }
        const observations = Array.isArray(site.remote_observations) ? site.remote_observations : [];
        for (const observation of observations) {
            const provider = normalizeTargetProvider(observation && observation.provider);
            const label = firstString(observation && observation.provider_deploy_id, site.name, site.project_dir, provider);
            addTarget(targets, provider, observation && observation.url, label, previewURL);
        }
    }

    function externalTargetURL(target, deploymentTargets) {
        if (!Array.isArray(deploymentTargets) || !deploymentTargets.length) return '';
        const selected = normalizeTargetProvider(target);
        const aliases = {
            local: ['local', 'homepage'],
            vercel: ['vercel'],
            netlify: ['netlify'],
            here_now: ['here_now'],
            remote: ['remote', 'sftp', 'scp', 'ssh']
        };
        const allowed = aliases[selected] || [selected];
        const exact = deploymentTargets.find(item => item && allowed.includes(item.provider) && item.url);
        if (exact) return exact.url;
        const externalTargets = ['remote', 'vercel', 'netlify', 'here_now'];
        if (externalTargets.includes(selected)) {
            const fallback = deploymentTargets.find(item => item && item.provider !== 'local' && item.url);
            if (fallback) return fallback.url;
        }
        return '';
    }

    function create(deps) {
        const container = deps.container;
        const isDisposed = typeof deps.isDisposed === 'function' ? deps.isDisposed : () => false;
        const onSiteSelected = typeof deps.onSiteSelected === 'function' ? deps.onSiteSelected : null;

        const state = {
            sites: [],
            selectedId: 0,
            details: new Map(),
            loading: false,
            error: false,
            reconciling: 0,
            abortCtrl: null,
            enabled: true
        };

        function esc(value) {
            return deps.esc(value == null ? '' : String(value));
        }

        const t = deps.t;

        function notify(message) {
            if (typeof deps.notify === 'function') deps.notify(message);
        }

        /* The canonical URL guard lives in homepage-studio.js (security-lint pinned);
           the sites panel reuses it instead of keeping a second implementation. */
        function safeURL(raw) {
            if (typeof deps.safeExternalURL === 'function') return deps.safeExternalURL(raw);
            return '';
        }

        function formatTime(value) {
            if (!value) return '';
            const date = new Date(value);
            if (Number.isNaN(date.getTime())) return '';
            const locale = deps.locale || window.SYSTEM_LANG || document.documentElement.lang || 'en';
            return date.toLocaleString(locale);
        }

        function driftClass(status) {
            const key = DRIFT_KEYS[status];
            return key ? ' vd-hp-drift-' + key : '';
        }

        function driftLabel(status) {
            const normalized = driftStatus(status);
            const tr = typeof deps.tr === 'function' ? deps.tr : deps.t;
            return tr('homepage_studio.sites_drift_' + normalized, normalized);
        }

        async function load() {
            if (state.loading) return;
            state.loading = true;
            state.error = false;
            render();
            try {
                const data = await deps.api('/api/desktop/integrations/homepage/sites');
                if (isDisposed()) return;
                state.sites = data && Array.isArray(data.sites) ? data.sites : [];
                if (state.selectedId && !state.sites.some(site => Number(site.id) === state.selectedId)) {
                    state.selectedId = 0;
                }
            } catch (_) {
                if (isDisposed()) return;
                state.sites = [];
                state.error = true;
            } finally {
                state.loading = false;
                if (!isDisposed()) render();
            }
            if (state.selectedId) loadDetail(state.selectedId);
        }

        async function loadDetail(id) {
            id = Number(id);
            if (!Number.isFinite(id) || id <= 0) return null;
            try {
                const data = await deps.api('/api/desktop/integrations/homepage/sites/' + encodeURIComponent(String(id)));
                if (isDisposed()) return null;
                const site = data && data.site;
                if (site) {
                    state.details.set(id, site);
                    render();
                    if (onSiteSelected && state.selectedId === id) onSiteSelected(id, site);
                }
                return site || null;
            } catch (_) {
                return null;
            }
        }

        function select(id) {
            id = Number(id);
            if (!Number.isFinite(id) || id <= 0) return;
            state.selectedId = state.selectedId === id ? 0 : id;
            render();
            if (state.selectedId) {
                const cached = state.details.get(state.selectedId);
                if (cached) {
                    if (onSiteSelected) onSiteSelected(state.selectedId, cached);
                } else {
                    loadDetail(state.selectedId);
                }
            } else if (onSiteSelected) {
                onSiteSelected(0, null);
            }
        }

        async function reconcile(id, btn) {
            id = Number(id);
            if (!Number.isFinite(id) || id <= 0 || state.reconciling === id) return;
            state.reconciling = id;
            if (btn) {
                btn.disabled = true;
                btn.classList.add('is-busy');
                const label = btn.querySelector('span');
                if (label) label.textContent = t('homepage_studio.sites_reconciling');
            }
            try {
                const data = await deps.api('/api/desktop/integrations/homepage/sites/' + encodeURIComponent(String(id)) + '/reconcile', { method: 'POST' });
                if (isDisposed()) return;
                if (data && data.site) {
                    state.details.set(id, data.site);
                    const idx = state.sites.findIndex(site => Number(site.id) === id);
                    if (idx >= 0) state.sites[idx] = data.site;
                }
                render();
                if (onSiteSelected && state.selectedId === id) onSiteSelected(id, state.details.get(id) || null);
            } catch (_) {
                if (!isDisposed()) notify(t('homepage_studio.sites_reconcile_error'));
            } finally {
                state.reconciling = 0;
                if (!isDisposed()) render();
            }
        }

        function renderLink(url, label) {
            const safe = safeURL(url);
            const text = esc(label || url || '—');
            if (!safe) return `<span class="vd-hp-site-row-url">${text}</span>`;
            return `<a class="vd-hp-site-row-url" href="${esc(safe)}" target="_blank" rel="noopener noreferrer">${text}</a>`;
        }

        function renderRow(provider, url, time, status) {
            const statusClass = status === 'ok' ? ' vd-hp-site-row-status-ok' : status === 'error' ? ' vd-hp-site-row-status-error' : '';
            return `
                <div class="vd-hp-site-row${statusClass}">
                    <span class="vd-hp-site-row-provider">${esc(provider || '—')}</span>
                    <span class="vd-hp-site-row-url-wrap">${renderLink(url, url)}</span>
                    ${time ? `<span class="vd-hp-site-row-time">${esc(time)}</span>` : ''}
                </div>`;
        }

        function renderFact(label, value, copyValue) {
            if (!value) return '';
            const copy = copyValue
                ? `<button type="button" class="vd-hp-site-copy" data-hp-copy="${esc(String(copyValue))}" title="${esc(t('homepage_studio.copy_path'))}" aria-label="${esc(t('homepage_studio.copy_path'))}">⧉</button>`
                : '';
            return `<div class="vd-hp-site-fact"><dt>${esc(label)}</dt><dd><code class="vd-hp-site-path">${esc(value)}</code>${copy}</dd></div>`;
        }

        function renderDetail(site) {
            if (!site) return '';
            const tr = typeof deps.tr === 'function' ? deps.tr : deps.t;
            const parts = ['<div class="vd-hp-site-detail">'];
            const originKey = site.origin === 'agent_created' ? 'agent' : site.origin === 'adopted' ? 'adopted' : 'unknown';
            parts.push('<p class="vd-hp-site-detail-msg">' + esc(tr('homepage_studio.sites_origin_' + originKey, originKey)) + '</p>');
            const facts = [];
            facts.push(renderFact(t('homepage_studio.sites_local'), site.local_root, site.local_root));
            facts.push(renderFact(t('homepage_studio.sites_container'), site.container_path, site.container_path));
            if (facts.length) parts.push('<dl class="vd-hp-site-facts">' + facts.join('') + '</dl>');
            if (site.last_reconciled_at) parts.push('<p class="vd-hp-site-detail-msg">' + esc(t('homepage_studio.sites_checked')) + ': ' + esc(formatTime(site.last_reconciled_at)) + '</p>');

            if (site.drift_message) {
                parts.push(`<p class="vd-hp-site-detail-msg">${esc(site.drift_message)}</p>`);
            }
            const targets = Array.isArray(site.deploy_targets) ? site.deploy_targets.slice(0, 10) : [];
            if (targets.length) {
                parts.push(`<span class="vd-hp-site-section-label">${esc(t('homepage_studio.sites_targets'))}</span><div class="vd-hp-site-rows">`);
                targets.forEach(target => {
                    const targetKey = target.status === 'active' ? 'active' : target.status === 'inactive' ? 'inactive' : 'unknown';
                    parts.push(renderRow((target.provider || '') + ' · ' + tr('homepage_studio.sites_target_' + targetKey, targetKey), target.url || target.remote_path, formatTime(target.last_seen_at || target.updated_at), target.status));
                });
                parts.push('</div>');
            }
            const deployments = Array.isArray(site.deployments) ? site.deployments.slice(0, 10) : [];
            parts.push(`<span class="vd-hp-site-section-label">${esc(t('homepage_studio.sites_deployments'))}</span>`);
            if (deployments.length) {
                parts.push('<div class="vd-hp-site-rows">');
                deployments.forEach(dep => {
                    parts.push(renderRow(dep.provider, dep.url, formatTime(dep.created_at), dep.status));
                });
                parts.push('</div>');
            } else {
                parts.push(`<p class="vd-hp-site-detail-msg">${esc(t('homepage_studio.sites_no_deployments'))}</p>`);
            }
            const observations = Array.isArray(site.remote_observations) ? site.remote_observations.slice(0, 10) : [];
            if (observations.length) {
                parts.push(`<span class="vd-hp-site-section-label">${esc(t('homepage_studio.sites_observations'))}</span><div class="vd-hp-site-rows">`);
                observations.forEach(obs => {
                    parts.push(renderRow(obs.provider, obs.url, formatTime(obs.observed_at), obs.status));
                });
                parts.push('</div>');
            }

            const events = Array.isArray(site.events) ? site.events : [];
            if (events.length) {
                const seen = new Set();
                parts.push('<span class="vd-hp-site-section-label">' + esc(t('homepage_studio.sites_changes')) + '</span>');
                events.forEach(event => {
                    const key = event.operation_id || String(event.id);
                    if (seen.has(key)) return;
                    seen.add(key);
                    const reason = event.reason && event.reason !== 'unknown' ? event.reason : tr('homepage_studio.sites_reason_unknown', 'unknown');
                    const details = event.details || {};
                    const files = Array.isArray(details.files_changed) ? details.files_changed : [];
                    const outcome = ['succeeded', 'failed', 'pending', 'partial', 'unknown'].includes(event.outcome) ? event.outcome : 'unknown';
                    const outcomeLabel = tr('homepage_studio.sites_outcome_' + outcome, outcome);
                    /* The reason often states the outcome already; do not repeat it. */
                    const summary = String(reason).toLowerCase().indexOf(String(outcomeLabel).toLowerCase()) >= 0
                        ? String(reason)
                        : String(reason) + ' · ' + outcomeLabel;
                    parts.push('<details class="vd-hp-site-event"><summary>' + esc(summary) + '</summary><time>' + esc(formatTime(event.created_at)) + '</time>' + (files.length ? '<ul>' + files.map(file => '<li><code>' + esc(file.path || '') + '</code></li>').join('') + '</ul>' : '') + '</details>');
                });
            }
            if (!deps.readonly) {
                parts.push(`
                    <div class="vd-hp-site-actions">
                        <button type="button" class="vd-hp-btn" data-hp-reconcile="${esc(String(site.id))}">
                            <span>${esc(t('homepage_studio.sites_reconcile'))}</span>
                        </button>
                    </div>`);
            }
            parts.push('</div>');
            return parts.join('');
        }

        function renderSiteCard(site) {
            const id = Number(site.id);
            const selected = id && id === state.selectedId;
            const name = site.name || site.project_dir || '#' + id;
            const lastDeployed = formatTime(site.last_deployed_at);
            const detail = selected ? state.details.get(id) : null;
            /* Roving tabindex: the selected card is the tab stop. */
            const tabStop = selected || (!state.selectedId && state.sites[0] && Number(state.sites[0].id) === id);
            return `
                <article class="vd-hp-site${selected ? ' is-selected' : ''}" data-site-id="${esc(String(id))}" tabindex="${tabStop ? '0' : '-1'}" role="option" aria-selected="${selected ? 'true' : 'false'}">
                    <div class="vd-hp-site-head">
                        <span class="vd-hp-site-name">${esc(name)}</span>
                        ${site.framework ? `<span class="vd-hp-site-framework">${esc(site.framework)}</span>` : ''}
                    </div>
                    <div class="vd-hp-site-meta">
                        <span class="vd-hp-drift${driftClass(site.drift_status)}">${esc(driftLabel(site.drift_status))}</span>
                        ${lastDeployed ? `<span class="vd-hp-site-date">${esc(t('homepage_studio.sites_last_deployed'))}: ${esc(lastDeployed)}</span>` : ''}
                    </div>
                    ${selected ? renderDetail(detail) : ''}
                </article>`;
        }

        /* Move the keyboard focus to a sibling card (listbox arrow keys). */
        function focusSiteCard(id) {
            if (!container) return;
            const cards = Array.from(container.querySelectorAll('.vd-hp-site'));
            const index = cards.findIndex(card => Number(card.getAttribute('data-site-id')) === Number(id));
            if (index < 0) return;
            cards.forEach(card => card.setAttribute('tabindex', '-1'));
            cards[index].setAttribute('tabindex', '0');
            cards[index].focus();
        }

        function render() {
            if (!container) return;
            /* The list is rebuilt on every state change; keep the keyboard position. */
            const active = document.activeElement;
            const activeCard = active && active.closest ? active.closest('.vd-hp-site') : null;
            const focusedSiteId = activeCard ? activeCard.getAttribute('data-site-id') : '';
            if (state.loading && !state.sites.length) {
                container.innerHTML = `<div class="vd-hp-sites-empty">${esc(t('homepage_studio.sites_loading'))}</div>`;
                return;
            }
            if (state.error) {
                container.innerHTML = `<div class="vd-hp-sites-empty">${esc(t('homepage_studio.sites_error'))}</div>`;
                return;
            }
            if (!state.sites.length) {
                container.innerHTML = `<div class="vd-hp-sites-empty">${esc(t('homepage_studio.sites_empty'))}</div>`;
                return;
            }
            container.innerHTML = state.sites.map(renderSiteCard).join('');
            container.querySelectorAll('.vd-hp-site').forEach(card => {
                const id = Number(card.getAttribute('data-site-id'));
                card.addEventListener('click', event => {
                    if (event.target.closest('a,button,details')) return;
                    select(id);
                });
                card.addEventListener('keydown', event => {
                    if (event.target.closest('a,button,details')) return;
                    if (event.key === 'Enter' || event.key === ' ') {
                        event.preventDefault();
                        select(id);
                        return;
                    }
                    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
                        event.preventDefault();
                        const cards = Array.from(container.querySelectorAll('.vd-hp-site'));
                        const index = cards.indexOf(card);
                        const next = cards[index + (event.key === 'ArrowDown' ? 1 : -1)];
                        if (next) focusSiteCard(next.getAttribute('data-site-id'));
                    }
                });
            });
            container.querySelectorAll('[data-hp-reconcile]').forEach(btn => {
                btn.addEventListener('click', () => reconcile(Number(btn.getAttribute('data-hp-reconcile')), btn));
            });
            container.querySelectorAll('[data-hp-copy]').forEach(btn => {
                btn.addEventListener('click', event => {
                    event.stopPropagation();
                    copyPath(btn.getAttribute('data-hp-copy'));
                });
            });
            if (focusedSiteId) {
                const cards = Array.from(container.querySelectorAll('.vd-hp-site'));
                const restored = cards.find(card => card.getAttribute('data-site-id') === focusedSiteId);
                if (restored) restored.focus();
            }
        }

        async function copyPath(value) {
            if (!value) return;
            try {
                if (navigator.clipboard && navigator.clipboard.writeText) {
                    await navigator.clipboard.writeText(String(value));
                }
            } catch (_) {}
        }

        function getSelected() {
            return state.selectedId || 0;
        }

        /* The target resolver reuses this list instead of fetching it again. */
        function getSites() {
            return state.sites.slice();
        }

        /* The already loaded detail of a site (no second request). */
        function getDetail(id) {
            const key = Number(id);
            if (!Number.isFinite(key) || key <= 0) return null;
            return state.details.get(key) || null;
        }

        function setSelected(id, detail) {
            state.selectedId = Number(id) || 0;
            if (detail) state.details.set(state.selectedId, detail);
            render();
        }

        function dispose() {
            if (state.abortCtrl) {
                state.abortCtrl.abort();
                state.abortCtrl = null;
            }
            state.details.clear();
        }

        return { load, select, reconcile, render, getSelected, getSites, getDetail, setSelected, dispose };
    }

    window.HomepageStudioSites = {
        create,
        DRIFT_KEYS,
        driftStatus,
        targets: { collect: collectTargets, add: addTarget, normalize: normalizeTargetProvider, external: externalTargetURL }
    };
})();
