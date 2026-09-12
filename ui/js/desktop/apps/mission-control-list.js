// Mission Control – left list pane. Keyed in-place DOM updates so SSE refreshes
// never steal focus or scroll position.
(function () {
    'use strict';

    const PRIORITY_RANK = { high: 0, medium: 1, low: 2 };

    function emitter() {
        const handlers = new Map();
        return {
            on(name, cb) { if (!handlers.has(name)) handlers.set(name, new Set()); handlers.get(name).add(cb); return () => handlers.get(name).delete(cb); },
            emit(name, payload) { const set = handlers.get(name); if (set) set.forEach(cb => { try { cb(payload); } catch (err) { console.error('MissionControlList handler failed', err); } }); },
            clear() { handlers.clear(); }
        };
    }

    function create(deps) {
        const { esc, t, lang, svg, triggers, schedule, readonly, fmt } = deps;
        const events = emitter();
        const rows = new Map(); // mission id -> row element
        let missions = [];
        let queue = { items: [], running: '' };
        let selectedId = '';
        let filter = 'all';
        let sort = 'name';
        let query = '';
        let ordered = [];

        const element = document.createElement('div');
        element.className = 'vd-mc-list';
        element.setAttribute('role', 'listbox');
        element.setAttribute('tabindex', '0');
        element.setAttribute('aria-label', t('desktop.mc_section_missions'));

        const ic = (name) => (svg && svg[name]) || '';

        // ── derived state ──
        function isRunning(m) { return m.id === queue.running || m.status === 'running'; }
        function queuePosition(m) { const idx = queue.items.findIndex(item => item.mission_id === m.id); return idx < 0 ? 0 : idx + 1; }
        function matchesFilter(m) {
            switch (filter) {
                case 'manual': case 'scheduled': case 'triggered': return m.execution_type === filter;
                case 'errors': return m.last_result === 'error';
                default: return true;
            }
        }
        function matchesQuery(m) {
            if (!query) return true;
            return `${m.name || ''} ${m.prompt || ''}`.toLowerCase().includes(query);
        }
        function compare(a, b) {
            switch (sort) {
                case 'last_run': return (Date.parse(b.last_run || '') || 0) - (Date.parse(a.last_run || '') || 0) || byName(a, b);
                case 'next_run': return (Date.parse(a.next_run || '') || Infinity) - (Date.parse(b.next_run || '') || Infinity) || byName(a, b);
                case 'priority': return (PRIORITY_RANK[a.priority] ?? 1) - (PRIORITY_RANK[b.priority] ?? 1) || byName(a, b);
                default: return byName(a, b);
            }
        }
        function byName(a, b) { return String(a.name || '').localeCompare(String(b.name || ''), lang || undefined, { sensitivity: 'base' }); }

        function stateOf(m) {
            if (isRunning(m)) return 'running';
            if (queuePosition(m)) return 'queued';
            if (m.enabled === false) return 'paused';
            if (m.last_result === 'error') return 'error';
            if (m.last_result === 'success') return 'ok';
            return m.last_run ? 'idle' : 'never';
        }

        function timeText(m, state) {
            if (state === 'running') return t('desktop.mc_state_running');
            if (state === 'queued') return t('desktop.mc_state_queued');
            if (m.execution_type === 'scheduled' && m.enabled !== false && m.next_run) return t('desktop.mc_next_run_in', { when: fmt.relative(m.next_run) });
            if (m.last_run) return t('desktop.mc_last_run_ago', { when: fmt.relative(m.last_run) });
            return t('desktop.mc_state_never_run');
        }

        // Signature of everything a row renders; unchanged rows keep their DOM.
        function rowSignature(m, state) {
            return [m.name, m.prompt ? m.prompt.length : 0, m.execution_type, m.schedule, m.trigger_type, JSON.stringify(m.trigger_config || null), m.enabled, m.locked, m.runner_type, m.priority, m.last_run, m.last_result, m.next_run, state, m.id === selectedId, readonly].join('|');
        }

        function rowMarkup(m, state) {
            const summary = triggers.summary(m, t, { schedule, lang });
            const busy = state === 'running' || state === 'queued';
            return `
                <span class="vd-mc-row-state" data-state="${esc(state)}" aria-hidden="true"></span>
                <span class="vd-mc-row-main">
                    <span class="vd-mc-row-title">
                        <span class="vd-mc-row-name">${esc(m.name || '')}</span>
                        ${m.locked ? `<span class="vd-mc-row-badge" title="${esc(t('desktop.mc_state_locked'))}">${ic('lock')}</span>` : ''}
                        ${m.runner_type === 'remote' ? `<span class="vd-mc-row-badge vd-mc-row-badge--text">${esc(t('desktop.mc_state_remote'))}</span>` : ''}
                        ${m.enabled === false ? `<span class="vd-mc-row-badge vd-mc-row-badge--text">${esc(t('desktop.mc_state_paused'))}</span>` : ''}
                    </span>
                    <span class="vd-mc-row-sub">${esc(summary)}</span>
                </span>
                <span class="vd-mc-row-meta">
                    <span class="vd-mc-row-time">${esc(timeText(m, state))}</span>
                    <button type="button" class="vd-mc-row-quick" data-mc-quick="run" tabindex="-1" title="${esc(t('desktop.mc_action_run'))}" aria-label="${esc(t('desktop.mc_action_run'))}" ${busy || readonly ? 'disabled' : ''}>${ic('play')}</button>
                </span>`;
        }

        function ensureRow(m, state) {
            let row = rows.get(m.id);
            if (!row) {
                row = document.createElement('div');
                row.className = 'vd-mc-row';
                row.setAttribute('role', 'option');
                row.setAttribute('tabindex', '-1');
                row.dataset.mcId = m.id;
                rows.set(m.id, row);
            }
            const sig = rowSignature(m, state);
            if (row.dataset.sig !== sig) {
                row.dataset.sig = sig;
                row.innerHTML = rowMarkup(m, state);
                row.dataset.state = state;
                row.classList.toggle('is-selected', m.id === selectedId);
                row.classList.toggle('is-running', state === 'running');
                row.classList.toggle('is-paused', state === 'paused');
                row.classList.toggle('has-error', state === 'error');
                row.setAttribute('aria-selected', m.id === selectedId ? 'true' : 'false');
            }
            return row;
        }

        function section(id, labelKey, items) {
            if (!items.length) return null;
            const wrap = document.createElement('div');
            wrap.className = 'vd-mc-section';
            wrap.dataset.mcSection = id;
            const head = document.createElement('div');
            head.className = 'vd-mc-section-title';
            head.setAttribute('role', 'presentation');
            head.innerHTML = `<span>${esc(t(labelKey))}</span><span class="vd-mc-section-count">${items.length}</span>`;
            wrap.appendChild(head);
            items.forEach(({ m, state }) => wrap.appendChild(ensureRow(m, state)));
            return wrap;
        }

        function emptyMarkup() {
            if (!missions.length) {
                return `<div class="vd-mc-list-empty">
                    <div class="vd-mc-list-empty-icon">${ic('workflow')}</div>
                    <div class="vd-mc-list-empty-title">${esc(t('desktop.mc_list_empty_title'))}</div>
                    <div class="vd-mc-list-empty-desc">${esc(t('desktop.mc_list_empty_desc'))}</div>
                    ${readonly ? '' : `<button type="button" class="vd-mc-btn vd-mc-btn--primary" data-mc-list-new>${ic('plus')}<span>${esc(t('desktop.mc_new_mission'))}</span></button>`}
                </div>`;
            }
            return `<div class="vd-mc-list-empty">
                <div class="vd-mc-list-empty-icon">${ic('search')}</div>
                <div class="vd-mc-list-empty-title">${esc(t('desktop.mc_list_no_match_title'))}</div>
                <div class="vd-mc-list-empty-desc">${esc(t('desktop.mc_list_no_match_desc'))}</div>
                <button type="button" class="vd-mc-btn" data-mc-list-clear>${esc(t('desktop.mc_list_clear_filter'))}</button>
            </div>`;
        }

        function render() {
            const visible = missions.filter(m => matchesFilter(m) && matchesQuery(m));
            const running = [], waiting = [], rest = [];
            visible.forEach(m => {
                const state = stateOf(m);
                if (state === 'running') running.push({ m, state });
                else if (state === 'queued') waiting.push({ m, state });
                else rest.push({ m, state });
            });
            running.sort((a, b) => byName(a.m, b.m));
            waiting.sort((a, b) => queuePosition(a.m) - queuePosition(b.m));
            rest.sort((a, b) => compare(a.m, b.m));
            ordered = running.concat(waiting, rest).map(x => x.m.id);

            const scrollTop = element.scrollTop;
            const active = document.activeElement;
            const focusedId = active && element.contains(active) ? (active.closest('[data-mc-id]') || {}).dataset?.mcId : '';

            const keep = new Set(ordered);
            rows.forEach((row, id) => { if (!keep.has(id)) { row.remove(); rows.delete(id); } });

            const frag = document.createDocumentFragment();
            [section('running', 'desktop.mc_section_running', running), section('waiting', 'desktop.mc_section_waiting', waiting), section('missions', 'desktop.mc_section_missions', rest)]
                .filter(Boolean).forEach(node => frag.appendChild(node));
            if (!ordered.length) {
                const empty = document.createElement('div');
                empty.innerHTML = emptyMarkup();
                frag.appendChild(empty.firstElementChild);
            }
            // Replace section wrappers only; row elements are moved, not recreated.
            Array.from(element.children).forEach(child => { if (!child.classList.contains('vd-mc-row')) child.remove(); });
            element.appendChild(frag);
            element.scrollTop = scrollTop;
            if (focusedId && rows.has(focusedId)) rows.get(focusedId).focus({ preventScroll: true });
        }

        // ── interaction ──
        element.addEventListener('click', (event) => {
            if (event.target.closest('[data-mc-list-new]')) { events.emit('new'); return; }
            if (event.target.closest('[data-mc-list-clear]')) { events.emit('clearFilter'); return; }
            const quick = event.target.closest('[data-mc-quick]');
            const row = event.target.closest('[data-mc-id]');
            if (!row) return;
            if (quick) { event.stopPropagation(); if (!quick.disabled) events.emit('run', row.dataset.mcId); return; }
            select(row.dataset.mcId, true);
        });
        element.addEventListener('dblclick', (event) => {
            const row = event.target.closest('[data-mc-id]');
            if (row) events.emit('activate', row.dataset.mcId);
        });
        element.addEventListener('contextmenu', (event) => {
            const row = event.target.closest('[data-mc-id]');
            event.preventDefault();
            if (!row) { events.emit('contextmenu', { x: event.clientX, y: event.clientY, missionId: '', queued: false }); return; }
            select(row.dataset.mcId, false);
            events.emit('contextmenu', { x: event.clientX, y: event.clientY, missionId: row.dataset.mcId, queued: row.dataset.state === 'queued' });
        });
        element.addEventListener('keydown', (event) => {
            if (event.target.matches('input, textarea, select')) return;
            const idx = ordered.indexOf(selectedId);
            switch (event.key) {
                case 'ArrowDown': event.preventDefault(); select(ordered[Math.min(ordered.length - 1, idx + 1)] || ordered[0], true); break;
                case 'ArrowUp': event.preventDefault(); select(ordered[Math.max(0, idx - 1)] || ordered[0], true); break;
                case 'Home': event.preventDefault(); select(ordered[0], true); break;
                case 'End': event.preventDefault(); select(ordered[ordered.length - 1], true); break;
                case 'Enter': if (selectedId) { event.preventDefault(); events.emit('activate', selectedId); } break;
                case 'Delete': if (selectedId && !readonly) { event.preventDefault(); events.emit('delete', selectedId); } break;
                default: break;
            }
        });
        element.addEventListener('focus', () => { if (!selectedId && ordered.length) select(ordered[0], true); else focusSelected(); });

        function select(id, focus) {
            if (!id) return;
            const changed = id !== selectedId;
            selectedId = id;
            rows.forEach((row, rowId) => {
                const on = rowId === id;
                row.classList.toggle('is-selected', on);
                row.setAttribute('aria-selected', on ? 'true' : 'false');
            });
            if (focus) focusSelected();
            if (changed) events.emit('select', id);
        }

        function focusSelected() {
            const row = rows.get(selectedId);
            if (row) { row.focus({ preventScroll: false }); row.scrollIntoView({ block: 'nearest' }); }
        }

        return {
            element,
            setData(data) {
                missions = Array.isArray(data && data.missions) ? data.missions : [];
                queue = (data && data.queue) || { items: [], running: '' };
                if (!Array.isArray(queue.items)) queue = Object.assign({}, queue, { items: [] });
                if (selectedId && !missions.some(m => m.id === selectedId)) selectedId = '';
                render();
            },
            setSelected(id) {
                selectedId = id || '';
                rows.forEach((row, rowId) => { const on = rowId === selectedId; row.classList.toggle('is-selected', on); row.setAttribute('aria-selected', on ? 'true' : 'false'); });
            },
            setFilter(next) { filter = next || 'all'; render(); },
            setSort(next) { sort = next || 'name'; render(); },
            setQuery(text) { query = String(text || '').trim().toLowerCase(); render(); },
            visibleIds() { return ordered.slice(); },
            focusSelected,
            counts() {
                const shown = ordered.length;
                const running = missions.filter(isRunning).length;
                const waiting = queue.items.filter(item => item.mission_id !== queue.running).length;
                return { total: missions.length, shown, running, waiting };
            },
            on: events.on,
            dispose() { events.clear(); rows.clear(); element.replaceChildren(); }
        };
    }

    window.MissionControlList = { create };
})();
