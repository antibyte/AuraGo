    /* MeshCore widget: compact latest-conversations view backed by the
       administrative messenger bootstrap API. Live updates arrive through the
       metadata-only document event 'aurago:meshcore-change'; a visibility-gated
       poll covers missed events. Protected previews are never revealed here,
       and radio text renders via textContent only, never as HTML. */
    const MESH_WIDGET_POLL_MS = 30000;
    const MESH_WIDGET_MAX_ROWS = 5;
    const MESH_WIDGET_ID_PATTERN = /^[a-f0-9]{64}$/;
    const MESH_WIDGET_STATES = ['connected', 'connecting', 'disconnected', 'disabled', 'binding_required', 'binding_changed', 'updating', 'suspended'];

    function meshWidgetStateName(raw) {
        const state = String(raw || 'disconnected');
        return MESH_WIDGET_STATES.includes(state) ? state : 'disconnected';
    }

    function meshWidgetTimeLabel(at) {
        const stamp = new Date(Number(at || 0) * 1000);
        if (!Number.isFinite(stamp.getTime()) || stamp.getTime() <= 0) return '';
        const sameDay = stamp.toDateString() === new Date().toDateString();
        return sameDay
            ? stamp.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
            : stamp.toLocaleDateString([], { day: '2-digit', month: '2-digit' });
    }

    function meshWidgetConversationName(conversation) {
        const raw = String(conversation && conversation.name || '').trim()
            || String(conversation && conversation.target || '').trim()
            || t('desktop.meshcore_unknown');
        return conversation && conversation.kind === 'channel' ? '#' + raw.replace(/^#+/, '') : raw;
    }

    function renderMeshCoreWidget(container) {
        container.innerHTML = `<div class="vd-mesh">
            <div class="vd-mesh-title-row">
                <span class="vd-mesh-title-icon">${iconMarkup('radio', 'M', 'vd-mesh-title-glyph', 15)}</span>
                <span class="vd-mesh-title">${esc(t('desktop.widget_meshcore_title'))}</span>
                <span class="vd-mesh-status" data-mesh="status" data-state="disconnected" role="status"></span>
                <span class="vd-mesh-unread" data-mesh="unread" hidden></span>
            </div>
            <div class="vd-mesh-list" data-mesh="list"></div>
            <div class="vd-mesh-footer">
                <button type="button" class="vd-mesh-open" data-mesh="open">${esc(t('desktop.widget_meshcore_open'))}</button>
            </div>
        </div>`;

        const refs = {
            root: container.querySelector('.vd-mesh'),
            status: container.querySelector('[data-mesh="status"]'),
            unread: container.querySelector('[data-mesh="unread"]'),
            list: container.querySelector('[data-mesh="list"]'),
            open: container.querySelector('[data-mesh="open"]')
        };

        let disposed = false;
        let refreshing = false;
        let refreshTimer = 0;

        function setStatus(state) {
            refs.status.dataset.state = state;
            refs.status.textContent = t('desktop.meshcore_state_' + state);
            refs.status.title = refs.status.textContent;
        }

        function showNotice(text, hint) {
            refs.list.innerHTML = '';
            const notice = document.createElement('div');
            notice.className = 'vd-mesh-notice';
            notice.textContent = text;
            refs.list.appendChild(notice);
            if (hint) {
                const hintEl = document.createElement('div');
                hintEl.className = 'vd-mesh-notice-hint';
                hintEl.textContent = hint;
                refs.list.appendChild(hintEl);
            }
            refs.root.classList.add('is-ready');
        }

        function renderError(error) {
            const code = String(error && error.message || '');
            if (code === 'admin_required') showNotice(t('desktop.meshcore_error_admin_required'));
            else if (code === 'unauthorized') showNotice(t('desktop.meshcore_error_unauthorized'));
            else showNotice(t('desktop.load_failed'));
            setStatus('disconnected');
            refs.unread.hidden = true;
        }

        function renderData(data) {
            const status = data && data.status || {};
            const enabled = !data || data.enabled !== false;
            const state = meshWidgetStateName(enabled ? status.state : 'disabled');
            setStatus(state);
            if (state === 'disabled') {
                refs.unread.hidden = true;
                showNotice(t('desktop.meshcore_state_disabled'), t('desktop.meshcore_setup'));
                return;
            }
            const conversations = Array.isArray(data && data.conversations) ? data.conversations.slice() : [];
            conversations.sort((a, b) => Number(b && b.last_at || 0) - Number(a && a.last_at || 0));
            const totalUnread = conversations.reduce((sum, entry) => sum + Math.max(0, Number(entry && entry.unread || 0)), 0);
            refs.unread.hidden = totalUnread <= 0;
            refs.unread.textContent = totalUnread > 99 ? '99+' : String(totalUnread);
            if (!conversations.length) {
                showNotice(t('desktop.meshcore_no_conversations'));
                return;
            }
            refs.list.innerHTML = '';
            conversations.slice(0, MESH_WIDGET_MAX_ROWS).forEach(conversation => {
                const row = document.createElement('button');
                row.type = 'button';
                row.className = 'vd-mesh-row';
                const id = String(conversation && conversation.id || '');
                if (MESH_WIDGET_ID_PATTERN.test(id)) row.dataset.meshConv = id;

                const main = document.createElement('span');
                main.className = 'vd-mesh-row-main';
                const head = document.createElement('span');
                head.className = 'vd-mesh-row-head';
                const name = document.createElement('span');
                name.className = 'vd-mesh-name';
                name.textContent = meshWidgetConversationName(conversation);
                const time = document.createElement('time');
                time.className = 'vd-mesh-time';
                time.textContent = meshWidgetTimeLabel(conversation && conversation.last_at);
                head.append(name, time);

                const preview = document.createElement('span');
                const previewText = String(conversation && conversation.preview || '').trim();
                if (conversation && conversation.protected && !previewText) {
                    preview.className = 'vd-mesh-preview vd-mesh-preview-protected';
                    const lock = document.createElement('span');
                    lock.className = 'vd-mesh-lock';
                    lock.innerHTML = iconMarkup('lock', 'L', 'vd-mesh-lock-glyph', 11);
                    preview.appendChild(lock);
                    preview.appendChild(document.createTextNode(t('desktop.widget_meshcore_protected')));
                } else {
                    preview.className = 'vd-mesh-preview';
                    preview.textContent = previewText || '…';
                }
                main.append(head, preview);
                row.appendChild(main);

                const unread = Math.max(0, Number(conversation && conversation.unread || 0));
                if (unread > 0) {
                    const badge = document.createElement('span');
                    badge.className = 'vd-mesh-row-unread';
                    badge.textContent = unread > 99 ? '99+' : String(unread);
                    row.appendChild(badge);
                }
                refs.list.appendChild(row);
            });
            refs.root.classList.add('is-ready');
        }

        async function refresh() {
            if (disposed || refreshing) return;
            refreshing = true;
            try {
                const data = await api('/api/meshcore/messenger/bootstrap');
                if (!disposed) renderData(data || {});
            } catch (error) {
                if (!disposed) renderError(error);
            } finally {
                refreshing = false;
            }
        }

        function scheduleRefresh() {
            clearTimeout(refreshTimer);
            refreshTimer = setTimeout(refresh, 250);
        }

        refs.list.addEventListener('click', event => {
            const row = event.target.closest('[data-mesh-conv]');
            if (!row || !refs.list.contains(row)) return;
            const id = row.dataset.meshConv || '';
            if (MESH_WIDGET_ID_PATTERN.test(id)) openApp('meshcore', { conversation_id: id });
        });
        refs.open.addEventListener('click', () => openApp('meshcore'));

        const onChange = () => scheduleRefresh();
        const onVisible = () => { if (!document.hidden) scheduleRefresh(); };
        document.addEventListener('aurago:meshcore-change', onChange);
        document.addEventListener('visibilitychange', onVisible);
        const pollTimer = setInterval(() => { if (!document.hidden) scheduleRefresh(); }, MESH_WIDGET_POLL_MS);

        setStatus('disconnected');
        showNotice(t('desktop.meshcore_loading'));
        refresh();

        registerWidgetCleanup(() => {
            disposed = true;
            document.removeEventListener('aurago:meshcore-change', onChange);
            document.removeEventListener('visibilitychange', onVisible);
            clearInterval(pollTimer);
            clearTimeout(refreshTimer);
        });
    }
