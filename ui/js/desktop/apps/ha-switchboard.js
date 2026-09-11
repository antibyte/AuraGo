(function () {
    'use strict';

    const KEY = 'ha_switchboard.board';
    const BASE = '/api/desktop/home-assistant/';
    const instances = new Map();
    const boardJSON = board => JSON.stringify({ version: 1, switches: (board.switches || []).map(e => ({ entity_id: e.entity_id, label: e.label || '' })) });
    const copyBoard = board => JSON.parse(boardJSON(board));

    function render(host, windowId, ctx) {
        dispose(windowId);
        const esc = ctx.esc;
        const win = host.closest('.vd-window');
        let board = { version: 1, switches: [] }, data = {}, loaded = false, connected = false;
        let disposed = false, generation = 0, poll = null, visible = false, checkedAt = '';
        let drawer = null, draft = null, draftBase = '', conflict = false, saving = false;
        let catalog = [], catalogReady = false, catalogRequest = null;
        const states = new Map(), pending = new Map(), errors = new Map(), requests = new Set();
        const asset = '/img/ha-switchboard/switch-atlas.png?v=' + encodeURIComponent(window.BUILD_VERSION || 'dev');
        const uid = 'ha-' + String(windowId).replace(/[^a-zA-Z0-9_-]/g, '');

        // Clip the original photographic atlas in SVG; no background pixels reach the wood.
        const hardware = `<svg class="ha-hardware" viewBox="0 0 512 1024" aria-hidden="true" focusable="false">
            <defs>
                <clipPath id="${uid}-on"><rect x="98" y="233" width="316" height="593" rx="9"/><ellipse cx="177" cy="111" rx="67" ry="71"/><path d="M144 164L199 149L223 238L169 252Z"/></clipPath>
                <clipPath id="${uid}-mid"><rect x="100" y="233" width="314" height="593" rx="9"/></clipPath>
                <clipPath id="${uid}-off"><rect x="100" y="233" width="314" height="593" rx="9"/><path d="M263 792L334 775L361 860L300 884Z"/><ellipse cx="340" cy="900" rx="72" ry="71"/></clipPath>
            </defs>
            <g class="ha-pose ha-pose-on" clip-path="url(#${uid}-on)"><image href="${asset}" width="1536" height="1024"/></g>
            <g class="ha-pose ha-pose-mid" clip-path="url(#${uid}-mid)"><image href="${asset}" x="-512" width="1536" height="1024"/></g>
            <g class="ha-pose ha-pose-off" clip-path="url(#${uid}-off)"><image href="${asset}" x="-1024" width="1536" height="1024"/></g>
        </svg>`;
        // Define clips once. Instances of the switch reference the window-local definitions.
        const switchArt = hardware.replace(/<defs>[\s\S]*?<\/defs>/, '');
        const bolt = '<svg viewBox="0 0 32 40" aria-hidden="true"><path d="M18 2 5 23h10l-1 15 13-23H17Z"/></svg>';
        // The scale and needle share one pivot and sweep, independent of CSS/font sizing.
        const polar = (angle, radius) => [100 + Math.sin(angle * Math.PI / 180) * radius, 104 - Math.cos(angle * Math.PI / 180) * radius];
        const scale = Array.from({ length: 41 }, (_, i) => {
            const angle = -120 + i * 6, major = i % 10 === 0;
            const [x1, y1] = polar(angle, major ? 70 : 76), [x2, y2] = polar(angle, 82), [x, y] = polar(angle, 57);
            return `<line x1="${x1}" y1="${y1}" x2="${x2}" y2="${y2}" class="ha-dial-tick ${major ? 'ha-major-tick' : ''}"/>` + (major ? `<text class="ha-dial-mark" x="${x}" y="${y}">${i * 2.5}</text>` : '');
        }).join('');
        host.innerHTML = `<section class="ha-board" aria-label="HA Switchboard">
            <div class="ha-definitions" aria-hidden="true">${hardware}</div>
            <div class="ha-deck"><div class="ha-bays"></div>
                <aside class="ha-instruments ha-panel">
                    <div class="ha-plaque ha-instrument-plaque">HOME<br>ASSISTANT</div>
                    <div class="ha-dial" role="img" aria-label="${esc(ctx.t('desktop.ha_on_count'))}">
                        <div class="ha-dial-face"><svg class="ha-dial-drawing" viewBox="0 0 200 200" aria-hidden="true"><defs><linearGradient id="${uid}-needle"><stop stop-color="#e9d8b1"/><stop offset=".4" stop-color="#302519"/><stop offset="1" stop-color="#776044"/></linearGradient></defs>${scale}<text class="ha-dial-unit" x="100" y="77">%</text><path class="ha-needle" fill="url(#${uid}-needle)" d="M100 26L103 108L100 125L97 108Z"/><circle class="ha-dial-hub" cx="100" cy="104" r="7" fill="url(#${uid}-needle)"/><text class="ha-dial-label" x="100" y="166">${esc(ctx.t('desktop.ha_on_count'))}</text></svg></div>
                    </div>
                    <div class="ha-meter-readout"><strong data-ha="total">— / —</strong><span>${esc(ctx.t('desktop.ha_on_count'))}</span></div>
                    <div class="ha-indicators"><p><i class="ha-lamp" data-ha="connection-lamp"></i><span data-ha="connection">${esc(ctx.t('desktop.ha_loading'))}</span></p><p><i class="ha-lamp" data-ha="warning-lamp"></i><span data-ha="unknown"></span></p></div>
                    <div class="ha-plaque ha-signature">HA<br><small>SWITCHBOARD</small></div>
                </aside>
            </div>
            <footer class="ha-console"><div class="ha-maker ha-panel"><span>AURAGO</span><small>HOME ASSISTANT</small><i aria-hidden="true">◆</i></div>
                <div class="ha-status-glass" role="status" aria-live="polite"><strong data-ha="status">${esc(ctx.t('desktop.ha_loading'))}</strong><span data-ha="summary"></span><small data-ha="updated"></small></div>
                <div class="ha-actions ha-panel"><button type="button" class="ha-metal-button" data-ha="manage">${esc(ctx.t('desktop.ha_manage'))}</button><button type="button" class="ha-refresh" data-ha="refresh">${esc(ctx.t('desktop.ha_refresh'))}</button><a href="/config#home_assistant" data-ha="setup" hidden>${esc(ctx.t('desktop.ha_setup'))}</a></div>
            </footer>
        </section>`;
        const root = host.querySelector('.ha-board');
        const q = name => root.querySelector('[data-ha="' + name + '"]');
        const bays = root.querySelector('.ha-bays');
        const image = new Image();
        image.onload = () => { if (!disposed) root.classList.add('ha-art-ready'); };
        image.src = asset;

        async function request(url, options = {}, controller = new AbortController()) {
            requests.add(controller);
            const timeout = setTimeout(() => controller.abort(), 25000);
            try { return await ctx.api(url, Object.assign({}, options, { signal: controller.signal })); }
            finally { clearTimeout(timeout); requests.delete(controller); }
        }

        function isVisible() {
            return !disposed && !document.hidden && (!win || (!win.hidden && !win.classList.contains('vd-space-hidden') && win.getClientRects().length > 0 && getComputedStyle(win).display !== 'none'));
        }

        function invalidatePoll() {
            generation++;
            if (poll) poll.controller.abort();
            poll = null;
        }

        function adoptBoard(next) {
            if (!next || next.version !== 1 || !Array.isArray(next.switches)) return;
            const raw = boardJSON(next);
            if (raw === boardJSON(board)) return;
            if (draft && raw !== draftBase && !(saving && raw === boardJSON(draft))) {
                conflict = true;
                drawerMessage('conflict');
            }
            board = copyBoard(next);
            for (const id of pending.keys()) if (!board.switches.some(e => e.entity_id === id)) pending.delete(id);
            renderBays();
        }

        function renderBays() {
            if (!board.switches.length) {
                bays.innerHTML = `<div class="ha-empty ha-panel"><div class="ha-empty-emblem" aria-hidden="true">${bolt}</div><h2>${esc(ctx.t('desktop.ha_empty'))}</h2><p>${esc((loaded && !data.ready ? ctx.t('desktop.ha_setup_hint') : ctx.t('desktop.ha_empty_hint')))}</p><button type="button" class="ha-metal-button" data-ha-empty>${esc((loaded && !data.ready ? ctx.t('desktop.ha_setup') : ctx.t('desktop.ha_manage')))}</button></div>`;
                return;
            }
            bays.innerHTML = board.switches.map((entry, index) => `<article class="ha-bay ha-panel" data-entity="${esc(entry.entity_id)}" data-state="unknown">
                <div class="ha-plaque ha-name" title="${esc(entry.label || states.get(entry.entity_id)?.friendly_name || entry.entity_id)}"><span class="ha-name-label">${esc(entry.label || states.get(entry.entity_id)?.friendly_name || entry.entity_id)}</span></div>
                <div class="ha-bay-symbol">${bolt}</div>
                <button type="button" class="ha-switch" role="switch" aria-checked="false" aria-label="${esc(entry.label || states.get(entry.entity_id)?.friendly_name || entry.entity_id)}" aria-describedby="${uid}-state-${index}" disabled>
                    <span class="ha-switch-fallback" aria-hidden="true"><i></i></span>${switchArt}
                </button><div class="ha-switch-status"><i class="ha-lamp"></i><span id="${uid}-state-${index}"></span></div>
                <span class="ha-channel">${esc(ctx.t('desktop.ha_channel'))} ${String(index + 1).padStart(2, '0')}</span>
            </article>`).join('');
        }

        function update() {
            if (disposed) return;
            let on = 0, unknown = 0;
            root.querySelectorAll('.ha-bay').forEach(bay => {
                const id = bay.dataset.entity, entity = states.get(id), job = pending.get(id);
                const state = entity?.state || 'missing';
                if (state === 'on') on++;
                if (!connected || !['on', 'off'].includes(state)) unknown++;
                const label = board.switches.find(e => e.entity_id === id)?.label || entity?.friendly_name || id;
                bay.querySelector('.ha-name-label').textContent = label;
                bay.querySelector('.ha-name').title = label + '\n' + id;
                bay.dataset.state = state;
                bay.classList.toggle('ha-pending', !!job);
                bay.classList.toggle('ha-stale', !connected);
                const button = bay.querySelector('.ha-switch');
                button.setAttribute('aria-label', label);
                button.title = id + ': ' + ctx.t('desktop.ha_' + state);
                button.setAttribute('aria-checked', String(state === 'on'));
                button.setAttribute('aria-busy', String(!!job));
                button.disabled = !connected || !data.ready || !!job || !['on', 'off'].includes(state) || !(state === 'on' ? data.can_off : data.can_on);
                const status = !connected ? (loaded ? 'offline' : 'loading') : job ? 'pending' : errors.get(id)?.key || state;
                bay.querySelector('.ha-switch-status span').textContent = ctx.t('desktop.ha_' + status);
                bay.querySelector('.ha-lamp').dataset.light = !connected ? 'warning' : job ? 'pending' : errors.has(id) || !['on', 'off'].includes(state) ? 'warning' : state;
            });
            q('total').textContent = (connected ? on : '—') + ' / ' + board.switches.length;
            const status = !loaded ? 'loading' : !data.ready ? 'setup_hint' : !connected ? 'offline' : data.readonly ? 'readonly' : 'connected';
            q('connection').textContent = ctx.t('desktop.ha_' + status);
            q('connection-lamp').dataset.light = connected ? 'on' : 'warning';
            q('warning-lamp').dataset.light = unknown ? 'warning' : 'off';
            q('unknown').textContent = ctx.t('desktop.ha_unavailable_count').replace('{count}', String(unknown));
            q('status').textContent = ctx.t('desktop.ha_' + status);
            q('summary').textContent = ctx.t('desktop.ha_summary').replace('{on}', connected ? String(on) : '—').replace('{total}', String(board.switches.length));
            q('updated').textContent = checkedAt ? ctx.t('desktop.ha_updated') + ' ' + new Date(checkedAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }) : '';
            root.querySelector('.ha-needle').style.transform = 'rotate(' + (-120 + (board.switches.length ? on / board.switches.length * 240 : 0)) + 'deg)';
            root.querySelector('.ha-dial').classList.toggle('ha-stale', !connected || unknown > 0);
            root.querySelector('.ha-dial').setAttribute('aria-label', q('summary').textContent + '. ' + q('unknown').textContent);
            q('manage').disabled = !loaded || !data.ready || !!data.board_readonly;
            q('setup').hidden = !loaded || !!data.ready;
            const emptyButton = bays.querySelector('[data-ha-empty]');
            if (emptyButton) emptyButton.disabled = !loaded || (data.ready && !!data.board_readonly);
        }

        async function refresh() {
            if (!isVisible() || poll || [...pending.values()].some(job => !job.accepted)) return;
            const current = { controller: new AbortController(), generation };
            poll = current;
            try {
                const next = await request(BASE + 'states', {}, current.controller);
                if (disposed || current.generation !== generation) return;
                const previousReady = data.ready;
                data = next;
                connected = !!next.ready;
                loaded = true;
                adoptBoard(next.board);
                if (next.ready) {
                    states.clear();
                    for (const entity of next.entities || []) states.set(entity.entity_id, entity);
                    checkedAt = next.checked_at;
                    for (const [id, job] of pending) {
                        if (job.accepted && states.get(id)?.state === job.target) { pending.delete(id); errors.delete(id); }
                        else if (job.accepted && Date.now() > job.deadline) { pending.delete(id); errors.set(id, { key: 'unconfirmed', target: job.target }); }
                    }
                    // A lost HTTP reply may still have operated the device; fresh state reconciles it.
                    for (const [id, error] of errors) if (states.get(id)?.state === error.target) errors.delete(id);
                }
                if (!board.switches.length && previousReady !== next.ready) renderBays();
            } catch (_) {
                if (disposed || current.generation !== generation) return;
                connected = false;
                loaded = true;
                // A read failure does not mean setup is missing.
                if (typeof data.ready !== 'boolean') data.ready = true;
            } finally {
                if (poll === current) poll = null;
                update();
            }
        }

        async function toggle(id) {
            const entity = states.get(id);
            if (!connected || !entity || pending.has(id) || !['on', 'off'].includes(entity.state)) return;
            const target = entity.state === 'on' ? 'off' : 'on';
            if (!(target === 'on' ? data.can_on : data.can_off)) return;
            invalidatePoll();
            const job = { target, accepted: false, deadline: 0 };
            pending.set(id, job); errors.delete(id); update();
            try {
                const result = await request(BASE + 'switch', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ entity_id: id, state: target }) });
                if (disposed || pending.get(id) !== job) return;
                if (!result.accepted) throw Error('unaccepted');
                job.accepted = true;
                job.deadline = Date.now() + 15000;
            } catch (_) {
                if (disposed) return;
                pending.delete(id); errors.set(id, { key: 'switch_error', target });
            }
            invalidatePoll(); update(); refresh();
        }

        function drawerMessage(key) {
            if (!drawer) return;
            drawer.querySelector('[data-ha-dialog-message]').textContent = key ? ctx.t('desktop.ha_' + key) : '';
            drawer.querySelector('[data-ha-apply]').disabled = saving || conflict || !catalogReady;
            drawer.querySelector('[data-ha-reload]').hidden = !conflict;
        }

        function renderSelection() {
            if (!drawer || !draft) return;
            const selected = new Set(draft.switches.map(e => e.entity_id));
            const search = drawer.querySelector('[data-ha-search]').value.toLocaleLowerCase();
            const list = catalog.filter(e => (e.friendly_name + ' ' + e.entity_id).toLocaleLowerCase().includes(search));
            drawer.querySelector('[data-ha-results]').innerHTML = list.map(entity => `<label class="ha-result"><input type="checkbox" data-ha-select="${esc(entity.entity_id)}" ${selected.has(entity.entity_id) ? 'checked' : ''}><span><strong>${esc(entity.friendly_name)}</strong><small>${esc(entity.entity_id)}</small></span><em>${esc(ctx.t('desktop.ha_' + entity.state))}</em></label>`).join('') || `<p class="ha-drawer-empty">${esc((catalog.length ? ctx.t('desktop.ha_no_matches') : ctx.t('desktop.ha_no_switches')))}</p>`;
            drawer.querySelector('[data-ha-selected]').innerHTML = draft.switches.map((entry, i) => {
                const entity = catalog.find(e => e.entity_id === entry.entity_id);
                const label = entity?.friendly_name || entry.entity_id;
                return `<div class="ha-selected-row" data-ha-selected-id="${esc(entry.entity_id)}"><span class="ha-order">${i + 1}</span><label><span>${esc(label)}</span><input type="text" maxlength="80" data-ha-label="${esc(entry.entity_id)}" value="${esc(entry.label)}" placeholder="${esc(ctx.t('desktop.ha_label'))}" aria-label="${esc(ctx.t('desktop.ha_label') + ': ' + label)}"></label><div class="ha-order-actions"><button type="button" data-ha-up="${i}" aria-label="${esc(ctx.t('desktop.ha_up'))}" ${i === 0 ? 'disabled' : ''}>↑</button><button type="button" data-ha-down="${i}" aria-label="${esc(ctx.t('desktop.ha_down'))}" ${i === draft.switches.length - 1 ? 'disabled' : ''}>↓</button><button type="button" data-ha-remove="${i}" aria-label="${esc(ctx.t('desktop.ha_remove'))}">×</button></div></div>`;
            }).join('') || `<p class="ha-drawer-empty">${esc(ctx.t('desktop.ha_empty'))}</p>`;
            drawer.querySelector('[data-ha-selected-count]').textContent = ctx.t('desktop.ha_selected') + ' · ' + draft.switches.length + ' / 60';
        }

        async function openDrawer() {
            if (drawer || disposed || !data.ready || data.board_readonly) return;
            draft = copyBoard(board); draftBase = boardJSON(board); conflict = false; catalogReady = false;
            drawer = document.createElement('dialog'); drawer.className = 'ha-drawer';
            drawer.setAttribute('aria-label', ctx.t('desktop.ha_manage'));
            drawer.innerHTML = `<form method="dialog"><header><h2>${esc(ctx.t('desktop.ha_manage'))}</h2><button type="button" data-ha-cancel aria-label="${esc(ctx.t('desktop.ha_cancel'))}">×</button></header><p class="ha-drawer-hint">${esc(ctx.t('desktop.ha_selection_hint'))}</p><div class="ha-drawer-columns"><section><label class="ha-search-label">${esc(ctx.t('desktop.ha_search'))}<input type="search" data-ha-search placeholder="${esc(ctx.t('desktop.ha_search_hint'))}"></label><div class="ha-results" data-ha-results></div></section><section><h3 data-ha-selected-count>${esc(ctx.t('desktop.ha_selected'))}</h3><div class="ha-selected" data-ha-selected></div></section></div><p class="ha-drawer-message" data-ha-dialog-message role="status"></p><footer><button type="button" data-ha-reload hidden>${esc(ctx.t('desktop.ha_reload'))}</button><button type="button" data-ha-retry>${esc(ctx.t('desktop.ha_refresh'))}</button><button type="button" data-ha-cancel>${esc(ctx.t('desktop.ha_cancel'))}</button><button type="button" class="ha-metal-button" data-ha-apply>${esc(ctx.t('desktop.ha_apply'))}</button></footer></form>`;
            root.append(drawer);
            drawer.addEventListener('cancel', event => { event.preventDefault(); if (!saving) closeDrawer(); });
            drawer.addEventListener('submit', event => event.preventDefault());
            drawer.addEventListener('click', drawerClick);
            drawer.addEventListener('input', drawerInput);
            drawer.showModal();
            renderSelection(); loadCatalog();
            drawer.querySelector('[data-ha-search]').focus();
        }

        async function loadCatalog() {
            if (!drawer || saving) return;
            if (catalogRequest) catalogRequest.abort();
            const controller = new AbortController(); catalogRequest = controller;
            const currentDrawer = drawer;
            drawerMessage('loading');
            try {
                const result = await request(BASE + 'entities', {}, controller);
                if (disposed || drawer !== currentDrawer || catalogRequest !== controller) return;
                if (!result.ready) throw Error('not ready');
                catalog = result.entities || []; catalogReady = true;
                adoptBoard(result.board);
                renderSelection(); drawerMessage(conflict ? 'conflict' : '');
            } catch (_) {
                if (drawer === currentDrawer && catalogRequest === controller) drawerMessage('catalog_error');
            } finally { if (catalogRequest === controller) catalogRequest = null; }
        }

        function closeDrawer() {
            if (catalogRequest) catalogRequest.abort();
            catalogRequest = null;
            if (drawer) { drawer.close(); drawer.remove(); }
            drawer = null; draft = null; saving = false;
            if (!disposed) q('manage').focus();
        }

        function drawerInput(event) {
            if (saving) return;
            const el = event.target;
            if (el.matches('[data-ha-search]')) { renderSelection(); return; }
            if (el.dataset.haLabel) {
                const entry = draft.switches.find(e => e.entity_id === el.dataset.haLabel);
                if (entry) entry.label = el.value;
                return;
            }
            if (!el.dataset.haSelect) return;
            if (el.checked) {
                if (draft.switches.length >= 60) { el.checked = false; drawerMessage('limit'); return; }
                draft.switches.push({ entity_id: el.dataset.haSelect, label: '' });
            } else draft.switches = draft.switches.filter(e => e.entity_id !== el.dataset.haSelect);
            renderSelection(); drawerMessage(conflict ? 'conflict' : '');
        }

        function drawerClick(event) {
            const el = event.target.closest('button');
            if (!el || saving) return;
            if (el.hasAttribute('data-ha-cancel')) return closeDrawer();
            if (el.hasAttribute('data-ha-apply')) return save();
            if (el.hasAttribute('data-ha-retry')) return loadCatalog();
            if (el.hasAttribute('data-ha-reload')) { draft = copyBoard(board); draftBase = boardJSON(board); conflict = false; renderSelection(); drawerMessage(''); return; }
            if (el.hasAttribute('data-ha-remove')) draft.switches.splice(Number(el.dataset.haRemove), 1);
            else if (el.hasAttribute('data-ha-up') || el.hasAttribute('data-ha-down')) {
                const index = Number(el.dataset.haUp ?? el.dataset.haDown), next = index + (el.hasAttribute('data-ha-up') ? -1 : 1);
                if (next >= 0 && next < draft.switches.length) [draft.switches[index], draft.switches[next]] = [draft.switches[next], draft.switches[index]];
            }
            renderSelection();
        }

        async function save() {
            if (!draft || conflict || saving || !catalogReady) return;
            saving = true;
            drawer.classList.add('ha-saving'); drawerMessage('saving');
            const currentDrawer = drawer;
            try {
                const latest = await request('/api/desktop/settings');
                if (disposed || drawer !== currentDrawer) return;
                const latestBoard = JSON.parse(latest.settings[KEY]);
                if (boardJSON(latestBoard) !== draftBase) { adoptBoard(latestBoard); conflict = true; throw Error('conflict'); }
                const raw = boardJSON(draft);
                await ctx.saveSetting(KEY, raw);
                if (disposed) return;
                adoptBoard(draft);
                document.dispatchEvent(new CustomEvent('aurago:ha-board-change', { detail: { raw } }));
                closeDrawer(); invalidatePoll(); update(); refresh();
            } catch (_) {
                if (drawer === currentDrawer) { saving = false; drawer.classList.remove('ha-saving'); drawerMessage(conflict ? 'conflict' : 'save_error'); }
            }
        }

        function boardChanged(event) {
            try { adoptBoard(JSON.parse(event.detail.raw)); } catch (_) { return; }
            invalidatePoll(); update(); refresh();
        }
        function visibilityChanged() {
            const next = isVisible();
            if (next === visible) return;
            visible = next;
            invalidatePoll();
            if (visible) refresh();
        }
        function click(event) {
            if (event.target.closest('.ha-drawer')) return;
            const switchButton = event.target.closest('.ha-switch');
            if (switchButton && !switchButton.disabled) return toggle(switchButton.closest('.ha-bay').dataset.entity);
            const action = event.target.closest('[data-ha], [data-ha-empty]');
            if (!action) return;
            if (action.dataset.ha === 'manage' || action.hasAttribute('data-ha-empty')) {
                if (loaded && !data.ready) window.location.assign('/config#home_assistant'); else openDrawer();
            }
            if (action.dataset.ha === 'refresh') refresh();
        }
        root.addEventListener('click', click);
        document.addEventListener('visibilitychange', visibilityChanged);
        document.addEventListener('aurago:ha-board-change', boardChanged);
        const observer = new MutationObserver(visibilityChanged);
        if (win) observer.observe(win, { attributes: true, attributeFilter: ['style', 'class', 'hidden', 'data-space-hidden'] });
        const timer = setInterval(refresh, 5000);
        instances.set(windowId, () => {
            disposed = true; invalidatePoll(); clearInterval(timer); observer.disconnect();
            requests.forEach(controller => controller.abort()); requests.clear(); closeDrawer();
            document.removeEventListener('visibilitychange', visibilityChanged);
            document.removeEventListener('aurago:ha-board-change', boardChanged);
            root.removeEventListener('click', click); image.onload = null;
        });
        renderBays(); update(); visibilityChanged();
    }

    function dispose(windowId) {
        const cleanup = instances.get(windowId);
        if (cleanup) cleanup();
        instances.delete(windowId);
    }

    window.HASwitchboardApp = { render, dispose };
})();
