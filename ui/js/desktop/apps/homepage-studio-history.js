/* Homepage Studio history panel: search, type filter, pagination and
   shell-dialog deletes for /api/desktop/integrations/homepage/history entries. */
(function () {
    'use strict';

    const PAGE_SIZE = 50;

    function create(deps) {
        const controls = deps.controls;
        const list = deps.list;
        const isDisposed = typeof deps.isDisposed === 'function' ? deps.isDisposed : () => false;

        const state = {
            query: '',
            filter: '',
            entries: [],
            total: 0,
            loading: false,
            loadingMore: false,
            enabled: true,
            projectId: 0,
            abortCtrl: null,
            debounceTimer: null
        };

        function esc(value) {
            return deps.esc(value == null ? '' : String(value));
        }

        const t = deps.t;

        function notify(message) {
            if (typeof deps.notify === 'function') deps.notify(message);
        }

        function debounce(fn, ms) {
            return function (...args) {
                if (state.debounceTimer) clearTimeout(state.debounceTimer);
                state.debounceTimer = setTimeout(() => {
                    state.debounceTimer = null;
                    fn.apply(this, args);
                }, ms);
            };
        }

        function setEnabled(enabled) {
            state.enabled = !!enabled;
            if (!state.enabled) {
                state.entries = [];
                state.total = 0;
                renderHistory([], t('homepage_studio.history_disabled'));
            }
        }

        function setProjectId(id) {
            const next = Number(id) || 0;
            if (next === state.projectId) return;
            state.projectId = next;
            loadHistory();
        }

        async function loadHistory(append) {
            if (!state.enabled) {
                renderHistory([], t('homepage_studio.history_disabled'));
                return;
            }
            if (state.abortCtrl) state.abortCtrl.abort();
            const abortCtrl = new AbortController();
            state.abortCtrl = abortCtrl;
            const offset = append ? state.entries.length : 0;
            state.loading = !append;
            state.loadingMore = !!append;
            if (!append) renderHistory(null, null, true);
            try {
                const params = new URLSearchParams();
                if (state.query) params.set('q', state.query);
                if (state.filter) params.set('entry_type', state.filter);
                if (state.projectId) params.set('project_id', String(state.projectId));
                params.set('limit', String(PAGE_SIZE));
                params.set('offset', String(offset));
                const url = '/api/desktop/integrations/homepage/history?' + params.toString();
                const data = await deps.api(url, { signal: abortCtrl.signal });
                if (isDisposed()) return;
                if (data && data.status === 'success') {
                    const batch = Array.isArray(data.entries) ? data.entries : [];
                    state.entries = append ? state.entries.concat(batch) : batch;
                    state.total = Number(data.total) || state.entries.length;
                    renderHistory(state.entries);
                } else {
                    renderHistory(append ? state.entries : [], data && data.message ? data.message : t('homepage_studio.history_error'));
                }
            } catch (err) {
                if (err && err.name === 'AbortError') return;
                if (isDisposed()) return;
                renderHistory(append ? state.entries : [], t('homepage_studio.history_error'));
            } finally {
                state.loading = false;
                state.loadingMore = false;
                if (state.abortCtrl === abortCtrl) state.abortCtrl = null;
            }
        }

        function typeLabel(type) {
            const tr = typeof deps.tr === 'function' ? deps.tr : deps.t;
            return tr('homepage_studio.history_type_' + type, type);
        }

        function formatTime(value) {
            if (!value) return '';
            const date = new Date(value);
            if (Number.isNaN(date.getTime())) return '';
            const locale = deps.locale || window.SYSTEM_LANG || document.documentElement.lang || 'en';
            return date.toLocaleString(locale);
        }

        function renderHistory(entries, emptyMessage, loading) {
            if (!list) return;
            /* The list is rebuilt on every render; keep the keyboard position. */
            const active = document.activeElement;
            const activeMore = active && active.classList ? active.classList.contains('vd-hp-history-more') : false;
            if (loading) {
                list.innerHTML = `<div class="vd-hp-history-empty">${esc(t('homepage_studio.history_loading'))}</div>`;
                return;
            }
            if (!entries || entries.length === 0) {
                list.innerHTML = `<div class="vd-hp-history-empty">${esc(emptyMessage || t('homepage_studio.history_empty'))}</div>`;
                return;
            }
            const html = entries.map(entry => {
                const date = entry.created_at ? formatTime(entry.created_at) : '';
                const type = String(entry.entry_type || 'note').replace(/[^a-z_]/g, '') || 'note';
                const source = entry.source ? `<span class="vd-hp-history-source">${esc(entry.source)}</span>` : '';
                const author = entry.author ? `<span class="vd-hp-history-author">${esc(entry.author)}</span>` : '';
                const tags = (entry.tags || []).map(tag => `<span class="vd-hp-history-tag">${esc(tag)}</span>`).join('');
                const id = esc(String(entry.id || ''));
                const deleteLabel = t('homepage_studio.history_delete_entry', { date: date || typeLabel(type) });
                const deleteBtn = deps.readonly ? '' : `
                            <button type="button" class="vd-hp-history-delete" data-id="${id}" title="${esc(deleteLabel)}" aria-label="${esc(deleteLabel)}">×</button>`;
                return `
                    <article class="vd-hp-history-entry vd-hp-history-type-${esc(type)}" role="listitem">
                        <header class="vd-hp-history-entry-header">
                            <span class="vd-hp-history-entry-type">${esc(typeLabel(type))}</span>
                            <time class="vd-hp-history-entry-time" datetime="${esc(entry.created_at || '')}">${esc(date)}</time>${deleteBtn}
                        </header>
                        <p class="vd-hp-history-entry-content">${esc(entry.content || '')}</p>
                        <footer class="vd-hp-history-entry-footer">${source}${author}${tags}</footer>
                    </article>
                `;
            }).join('');
            const hasMore = state.entries.length < state.total;
            const more = hasMore
                ? `<button type="button" class="vd-hp-history-more">${esc(t('homepage_studio.history_load_more'))}</button>`
                : '';
            list.innerHTML = html + more;
            list.querySelectorAll('.vd-hp-history-delete').forEach(btn => {
                btn.addEventListener('click', () => deleteEntry(btn.getAttribute('data-id')));
            });
            const moreBtn = list.querySelector('.vd-hp-history-more');
            if (moreBtn) {
                moreBtn.addEventListener('click', () => {
                    if (state.loadingMore || state.loading) return;
                    loadHistory(true);
                });
                if (activeMore) moreBtn.focus();
            }
        }

        async function deleteEntry(id) {
            if (!id || deps.readonly) return;
            const confirmed = typeof deps.confirmDialog === 'function'
                ? await deps.confirmDialog(t('homepage_studio.history_delete_confirm'))
                : false;
            if (!confirmed) return;
            try {
                await deps.api('/api/desktop/integrations/homepage/history?id=' + encodeURIComponent(id), { method: 'DELETE' });
                state.entries = state.entries.filter(entry => String(entry.id) !== String(id));
                state.total = Math.max(state.entries.length, state.total - 1);
                renderHistory(state.entries);
            } catch (_) {
                notify(t('homepage_studio.history_delete_error'));
            }
        }

        if (controls) {
            const search = controls.querySelector('.vd-hp-history-search');
            const filter = controls.querySelector('.vd-hp-history-filter');
            const refresh = controls.querySelector('.vd-hp-history-refresh');
            if (search) {
                search.addEventListener('input', debounce(() => {
                    state.query = search.value.trim();
                    loadHistory();
                }, 250));
            }
            if (filter) {
                filter.addEventListener('change', () => {
                    state.filter = filter.value;
                    loadHistory();
                });
            }
            if (refresh) {
                refresh.addEventListener('click', () => loadHistory());
            }
        }

        function syncFilters(query, filter) {
            state.query = String(query || '');
            state.filter = String(filter || '');
            if (controls) {
                const search = controls.querySelector('.vd-hp-history-search');
                const typeFilter = controls.querySelector('.vd-hp-history-filter');
                if (search) search.value = state.query;
                if (typeFilter) typeFilter.value = state.filter;
            }
        }

        function getFilters() {
            return { query: state.query, filter: state.filter };
        }

        function dispose() {
            if (state.abortCtrl) {
                state.abortCtrl.abort();
                state.abortCtrl = null;
            }
            if (state.debounceTimer) {
                clearTimeout(state.debounceTimer);
                state.debounceTimer = null;
            }
        }

        return { loadHistory, renderHistory, setEnabled, setProjectId, syncFilters, getFilters, dispose };
    }

    window.HomepageStudioHistory = { create };
})();
