/* Homepage Studio sites panel: managed-site list with drift badges, site
   detail (deploy targets, deployments, remote observations) and on-demand
   reconcile. Data comes from /api/homepage/sites + /api/homepage/sites/{id}. */
(function () {
    'use strict';

    const DRIFT_KEYS = {
        clean: 'clean',
        local_changed: 'local-changed',
        remote_changed: 'remote-changed',
        remote_unknown: 'remote-unknown',
        not_deployed: 'not-deployed'
    };

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

        function safeURL(raw) {
            const value = typeof raw === 'string' ? raw.trim() : '';
            if (!value) return '';
            try {
                const url = new URL(value, window.location.origin);
                if (url.protocol === 'http:' || url.protocol === 'https:') return url.href;
            } catch (_) {}
            return '';
        }

        function formatTime(value) {
            if (!value) return '';
            const date = new Date(value);
            if (Number.isNaN(date.getTime())) return '';
            return date.toLocaleString();
        }

        function driftClass(status) {
            const key = DRIFT_KEYS[status];
            return key ? ' vd-hp-drift-' + key : '';
        }

        function driftLabel(status) {
            const known = ['clean', 'local_changed', 'remote_changed', 'remote_unknown', 'not_deployed'];
            const normalized = known.includes(status) ? status : 'not_deployed';
            return t('homepage_studio.sites_drift_' + normalized, normalized);
        }

        async function load() {
            if (state.loading) return;
            state.loading = true;
            state.error = false;
            render();
            try {
                const data = await deps.api('/api/homepage/sites');
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
                const data = await deps.api('/api/homepage/sites/' + encodeURIComponent(String(id)));
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
                const data = await deps.api('/api/homepage/sites/' + encodeURIComponent(String(id)) + '/reconcile', { method: 'POST' });
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
                    ${renderLink(url, url)}
                    ${time ? `<span class="vd-hp-site-row-time">${esc(time)}</span>` : ''}
                </div>`;
        }

        function renderDetail(site) {
            if (!site) return '';
            const parts = ['<div class="vd-hp-site-detail">'];
            if (site.drift_message) {
                parts.push(`<p class="vd-hp-site-detail-msg">${esc(site.drift_message)}</p>`);
            }
            const targets = Array.isArray(site.deploy_targets) ? site.deploy_targets.slice(0, 10) : [];
            if (targets.length) {
                parts.push(`<span class="vd-hp-site-section-label">${esc(t('homepage_studio.sites_targets'))}</span><div class="vd-hp-site-rows">`);
                targets.forEach(target => {
                    parts.push(renderRow(target.provider, target.url || target.remote_path, formatTime(target.last_seen_at || target.updated_at)));
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
            return `
                <article class="vd-hp-site${selected ? ' is-selected' : ''}" data-site-id="${esc(String(id))}" tabindex="0" role="button" aria-pressed="${selected ? 'true' : 'false'}">
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

        function render() {
            if (!container) return;
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
                    if (event.target.closest('a,button')) return;
                    select(id);
                });
                card.addEventListener('keydown', event => {
                    if (event.target.closest('a,button')) return;
                    if (event.key === 'Enter' || event.key === ' ') {
                        event.preventDefault();
                        select(id);
                    }
                });
            });
            container.querySelectorAll('[data-hp-reconcile]').forEach(btn => {
                btn.addEventListener('click', () => reconcile(Number(btn.getAttribute('data-hp-reconcile')), btn));
            });
        }

        function getSelected() {
            return state.selectedId || 0;
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

        return { load, select, reconcile, render, getSelected, setSelected, dispose };
    }

    window.HomepageStudioSites = { create };
})();
