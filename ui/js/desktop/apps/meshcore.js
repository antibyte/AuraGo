(function () {
    'use strict';
    const instances = new Map();
    const API = '/api/meshcore/messenger/';
    const encoder = new TextEncoder();
    const GROUP_GAP_SECONDS = 300;
    const esc = value => String(value ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
    const node = (tag, text, cls) => { const el = document.createElement(tag); if (text !== undefined) el.textContent = text; if (cls) el.className = cls; return el; };
    const tr = (s, key, vars) => s.context.t('desktop.meshcore_' + key, vars);

    const ICONS = {
        mesh: '<circle cx="12" cy="12" r="2.1" fill="currentColor" stroke="none"/><path d="M8.5 8.5a5 5 0 0 0 0 7"/><path d="M15.5 8.5a5 5 0 0 1 0 7"/><path d="M5.6 5.6a9 9 0 0 0 0 12.8"/><path d="M18.4 5.6a9 9 0 0 1 0 12.8"/>',
        refresh: '<polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/>',
        self: '<path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/>',
        settings: '<line x1="4" y1="21" x2="4" y2="14"/><line x1="4" y1="10" x2="4" y2="3"/><line x1="12" y1="21" x2="12" y2="12"/><line x1="12" y1="8" x2="12" y2="3"/><line x1="20" y1="21" x2="20" y2="16"/><line x1="20" y1="12" x2="20" y2="3"/><line x1="2" y1="14" x2="6" y2="14"/><line x1="10" y1="8" x2="14" y2="8"/><line x1="18" y1="16" x2="22" y2="16"/>',
        external: '<path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/><polyline points="15 3 21 3 21 9"/><line x1="10" y1="14" x2="21" y2="3"/>',
        back: '<line x1="19" y1="12" x2="5" y2="12"/><polyline points="12 19 5 12 12 5"/>',
        info: '<circle cx="12" cy="12" r="9"/><line x1="12" y1="11" x2="12" y2="16"/><line x1="12" y1="7.5" x2="12.01" y2="7.5"/>',
        send: '<line x1="22" y1="2" x2="11" y2="13"/><polygon points="22 2 15 22 11 13 2 9 22 2"/>',
        'user-plus': '<path d="M16 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="8.5" cy="7" r="4"/><line x1="20" y1="8" x2="20" y2="14"/><line x1="17" y1="11" x2="23" y2="11"/>',
        hash: '<line x1="4" y1="9" x2="20" y2="9"/><line x1="4" y1="15" x2="20" y2="15"/><line x1="10" y1="3" x2="8" y2="21"/><line x1="16" y1="3" x2="14" y2="21"/>',
        search: '<circle cx="11" cy="11" r="7"/><line x1="21" y1="21" x2="16.5" y2="16.5"/>',
        copy: '<rect x="9" y="9" width="12" height="12" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>',
        star: '<polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"/>',
        bell: '<path d="M18 8a6 6 0 0 0-12 0c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.7 21a2 2 0 0 1-3.4 0"/>',
        'bell-off': '<path d="M8.7 3.4A6 6 0 0 1 18 8c0 2 .4 3.8.9 5.3"/><path d="M6.3 6.3C6.1 6.9 6 7.4 6 8c0 7-3 9-3 9h14"/><path d="M13.7 21a2 2 0 0 1-3.4 0"/><line x1="2" y1="2" x2="22" y2="22"/>',
        share: '<circle cx="18" cy="5" r="3"/><circle cx="6" cy="12" r="3"/><circle cx="18" cy="19" r="3"/><line x1="8.6" y1="13.5" x2="15.4" y2="17.5"/><line x1="15.4" y1="6.5" x2="8.6" y2="10.5"/>',
        trash: '<polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>',
        eraser: '<path d="m7 21-4.3-4.3c-1-1-1-2.5 0-3.4l9.6-9.6c1-1 2.5-1 3.4 0l5.6 5.6c1 1 1 2.5 0 3.4L13 21"/><path d="M22 21H7"/><path d="m5 11 9 9"/>',
        close: '<line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>',
        check: '<polyline points="20 6 9 17 4 12"/>',
        checks: '<polyline points="2 12.5 6.5 17 16 6"/><polyline points="11 16.5 12.5 18 22 7"/>',
        clock: '<circle cx="12" cy="12" r="9"/><polyline points="12 7 12 12 15.5 13.5"/>',
        alert: '<path d="M10.3 3.9 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/>',
        help: '<circle cx="12" cy="12" r="9"/><path d="M9.1 9a3 3 0 0 1 5.8 1c0 2-3 3-3 3"/><line x1="12" y1="17" x2="12.01" y2="17"/>',
        shield: '<path d="M12 22s8-3.6 8-10V5l-8-3-8 3v7c0 6.4 8 10 8 10z"/>',
        eye: '<path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7-10-7-10-7z"/><circle cx="12" cy="12" r="3"/>',
        down: '<line x1="12" y1="5" x2="12" y2="19"/><polyline points="19 12 12 19 5 12"/>',
        qr: '<rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/>',
        megaphone: '<path d="m3 11 18-5v12L3 13v-2z"/><path d="M11.6 16.8a3 3 0 1 1-5.8-1.6"/>'
    };
    const SEND_STATE_ICONS = { sending: 'clock', queued: 'clock', device_accepted: 'check', delivered: 'checks', not_sent: 'close', outcome_unknown: 'alert', unknown: 'help' };

    const icon = name => `<svg class="mc-icon" viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">${ICONS[name] || ''}</svg>`;
    function iconEl(name, cls) {
        const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
        svg.setAttribute('viewBox', '0 0 24 24');
        svg.setAttribute('aria-hidden', 'true');
        svg.setAttribute('fill', 'none');
        svg.setAttribute('stroke', 'currentColor');
        svg.setAttribute('stroke-width', '2');
        svg.setAttribute('stroke-linecap', 'round');
        svg.setAttribute('stroke-linejoin', 'round');
        svg.setAttribute('class', 'mc-icon' + (cls ? ' ' + cls : ''));
        svg.innerHTML = ICONS[name] || '';
        return svg;
    }

    const textBtn = (s, key, action, cls = '') => `<button type="button" class="${cls}" data-mc="${action}">${esc(tr(s, key))}</button>`;
    const iconBtn = (s, key, action, iconName, cls = 'mc-icon-btn') => `<button type="button" class="${cls}" data-mc="${action}" title="${esc(tr(s, key))}">${icon(iconName)}<span class="mc-sr-only">${esc(tr(s, key))}</span></button>`;
    const labelBtn = (s, key, action, iconName, cls = '') => `<button type="button" class="${cls}" data-mc="${action}">${icon(iconName)}<span>${esc(tr(s, key))}</span></button>`;
    const emptyMarkup = (iconMarkup, text, hint) => `<div class="mc-empty"><span class="mc-empty-icon" aria-hidden="true">${iconMarkup}</span><p>${esc(text)}</p>${hint ? `<span class="mc-empty-hint">${esc(hint)}</span>` : ''}</div>`;

    function hueOf(value) { let h = 0; const str = String(value || ''); for (let i = 0; i < str.length; i++) h = (h * 31 + str.charCodeAt(i)) % 360; return h; }

    async function request(s, path, body) {
        const controller = new AbortController();
        s.requests.add(controller);
        const timer = setTimeout(() => controller.abort(), 25000);
        try {
            const response = await fetch(API + path, { method: body === undefined ? 'GET' : 'POST', credentials: 'same-origin', cache: 'no-store', signal: controller.signal,
                headers: body === undefined ? {} : { 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body) });
            const data = await response.json();
            if (!response.ok) { const error = new Error('request_failed'); error.code = data.error || 'operation_failed'; throw error; }
            return data;
        } catch (error) {
            if (error.name === 'AbortError') error.code = 'timeout';
            throw error;
        } finally { clearTimeout(timer); s.requests.delete(controller); }
    }

    function errorText(s, error) {
        const known = ['invalid_request', 'invalid_text', 'invalid_target', 'invalid_contact', 'invalid_channel', 'invalid_invitation', 'unsupported_invitation', 'contact_exists', 'channels_full', 'binding_required', 'not_connected', 'busy', 'idempotency_conflict', 'send_ledger_full', 'outcome_unknown', 'config_unavailable', 'message_unavailable', 'timeout', 'admin_required', 'unauthorized'];
        return tr(s, 'error_' + (known.includes(error.code) ? error.code : 'operation_failed'));
    }

    function render(host, windowId, context) {
        dispose(windowId);
        const s = { host, windowId, context, requests: new Set(), conversations: [], messages: [], revealed: new Map(), selected: '', filter: 'all', search: '', query: '', status: {}, generation: 0, disposed: false, sending: false, readSeq: 0, loaded: false };
        instances.set(windowId, s);
        host.innerHTML = `<div class="vd-meshcore">
            <header class="mc-toolbar"><div class="mc-brand"><span class="mc-logo" aria-hidden="true">${icon('mesh')}</span><strong>MeshCore</strong><span class="mc-status" data-mc-role="status" role="status"></span></div><div class="mc-actions">${iconBtn(s, 'refresh', 'refresh', 'refresh')}${iconBtn(s, 'self', 'self', 'self')}${iconBtn(s, 'settings', 'settings', 'settings')}<a class="mc-link" href="/config#meshcore" target="_blank" rel="noopener">${icon('external')}<span>${esc(tr(s, 'connection'))}</span></a></div></header>
            <div class="mc-feedback" data-mc-role="error" role="alert" hidden></div>
            <div class="mc-body"><aside class="mc-sidebar"><div class="mc-sidebar-tools"><div class="mc-search">${icon('search')}<input type="search" data-mc-role="search" aria-label="${esc(tr(s, 'search'))}" placeholder="${esc(tr(s, 'search'))}"></div><div class="mc-filters" role="group" aria-label="${esc(tr(s, 'filter'))}">${['all', 'direct', 'channel', 'unread'].map(key => textBtn(s, key, 'filter-' + key)).join('')}</div><div class="mc-actions mc-sidebar-actions">${labelBtn(s, 'add_contact', 'add-contact', 'user-plus')}${labelBtn(s, 'add_channel', 'add-channel', 'hash')}</div></div><nav class="mc-conversations" aria-label="${esc(tr(s, 'conversations'))}" data-mc-role="conversations"></nav></aside>
            <main class="mc-chat"><header class="mc-chat-head">${iconBtn(s, 'back', 'back', 'back', 'mc-icon-btn mc-back')}<span class="mc-avatar mc-avatar-sm" data-mc-role="peer-avatar" aria-hidden="true" hidden></span><div class="mc-chat-title"><strong data-mc-role="title">${esc(tr(s, 'choose'))}</strong><span data-mc-role="subtitle"></span></div>${iconBtn(s, 'details', 'details', 'info')}</header>
            <div class="mc-history-search"><div class="mc-search">${icon('search')}<input type="search" data-mc-role="query" aria-label="${esc(tr(s, 'search_history'))}" placeholder="${esc(tr(s, 'search_history'))}"></div></div>
            <div class="mc-chat-scroll"><div class="mc-messages" data-mc-role="messages" tabindex="0" aria-label="${esc(tr(s, 'messages'))}">${emptyMarkup(icon('mesh'), tr(s, 'choose'), tr(s, 'offline_hint'))}</div>
            <button type="button" class="mc-new" data-mc="latest" hidden>${icon('down')}<span>${esc(tr(s, 'new_messages'))}</span></button></div>
            <form class="mc-composer"><div class="mc-composer-row"><label class="mc-sr-only" for="mc-compose-${esc(windowId)}">${esc(tr(s, 'message'))}</label><textarea id="mc-compose-${esc(windowId)}" data-mc-role="compose" rows="2" maxlength="1200" placeholder="${esc(tr(s, 'message'))}"></textarea><button type="submit" class="mc-primary mc-send" data-mc-role="send" title="${esc(tr(s, 'send'))}">${icon('send')}<span class="mc-sr-only">${esc(tr(s, 'send'))}</span></button></div><div class="mc-compose-footer"><span class="mc-counter" data-mc-role="counter" aria-live="polite"></span><span class="mc-hint" data-mc-role="send-hint"></span></div><details class="mc-parts"><summary>${esc(tr(s, 'preview'))}</summary><div data-mc-role="parts"></div></details></form>
            </main><aside class="mc-detail" data-mc-role="detail" hidden></aside></div></div>`;
        s.root = host.firstElementChild;
        s.el = role => s.root.querySelector(`[data-mc-role="${role}"]`);
        s.root.addEventListener('click', event => { const el = event.target.closest('[data-mc]'); if (el) act(s, el.dataset.mc).catch(error => showError(s, error)); });
        s.el('search').addEventListener('input', event => { s.search = event.target.value; renderList(s); });
        s.el('query').addEventListener('input', event => { s.query = event.target.value; clearTimeout(s.searchTimer); s.searchTimer = setTimeout(() => loadMessages(s, false, true).catch(error => showError(s, error)), 300); });
        s.root.querySelector('form').addEventListener('submit', event => { event.preventDefault(); send(s); });
        s.el('compose').addEventListener('input', () => { saveDraft(s); updateComposer(s); });
        s.el('compose').addEventListener('keydown', event => { if (event.key === 'Enter' && !event.shiftKey && !event.isComposing && event.keyCode !== 229) { event.preventDefault(); send(s); } });
        s.el('messages').addEventListener('scroll', () => { if (nearBottom(s)) { s.root.querySelector('[data-mc="latest"]').hidden = true; markRead(s); } });
        s.el('conversations').addEventListener('keydown', event => {
            if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return;
            const items = [...s.el('conversations').querySelectorAll('.mc-conversation')];
            if (!items.length) return;
            const index = items.indexOf(event.target.closest('.mc-conversation'));
            const next = event.key === 'ArrowDown' ? (index + 1) % items.length : (index - 1 + items.length) % items.length;
            event.preventDefault();
            items[next].focus();
        });
        s.root.addEventListener('keydown', event => {
            if (event.key !== 'Escape' || event.defaultPrevented || s.root.querySelector('dialog[open]')) return;
            if (!s.el('detail').hidden) { s.el('detail').hidden = true; event.preventDefault(); }
            else if (s.root.classList.contains('mc-has-chat')) { s.root.classList.remove('mc-has-chat'); event.preventDefault(); }
        });
        s.onChange = event => { const id = event.detail?.conversation_id; if (!id || id === s.selected) scheduleRefresh(s); else refresh(s, false).catch(error => showError(s, error)); };
        s.onVisible = () => { if (!document.hidden) scheduleRefresh(s); };
        document.addEventListener('aurago:meshcore-change', s.onChange);
        document.addEventListener('visibilitychange', s.onVisible);
        s.poll = setInterval(() => { if (!document.hidden && s.root.getClientRects().length) scheduleRefresh(s); }, 15000);
        updateComposer(s);
        refresh(s).then(() => { const id = context.conversation_id; if (id) selectConversation(s, id); }).catch(error => showError(s, error));
    }

    function showError(s, error) { if (!s.disposed) { s.el('error').hidden = false; s.el('error').textContent = errorText(s, error); } }
    function clearError(s) { s.el('error').hidden = true; s.el('error').textContent = ''; }
    function current(s) { return s.conversations.find(c => c.id === s.selected); }
    function nearBottom(s) { const el = s.el('messages'); return el.scrollHeight - el.scrollTop - el.clientHeight < 56; }
    function scheduleRefresh(s) { clearTimeout(s.refreshTimer); s.refreshTimer = setTimeout(() => refresh(s).catch(error => showError(s, error)), 120); }

    async function refresh(s, messages = true) {
        if (s.disposed || s.refreshing) return;
        s.refreshing = true;
        s.root.querySelector('[data-mc="refresh"]').disabled = true;
        s.el('status').classList.add('mc-busy');
        try {
            const data = await request(s, 'bootstrap');
            if (s.disposed) return;
            s.status = data.status || {}; s.conversations = data.conversations || []; s.settings = data;
            s.loaded = true;
            const state = ['connected', 'connecting', 'disconnected', 'disabled', 'binding_required', 'binding_changed', 'updating', 'suspended'].includes(s.status.state) ? s.status.state : 'disconnected';
            s.el('status').textContent = tr(s, 'state_' + state);
            s.el('status').dataset.state = state;
            for (const action of ['self', 'add-contact', 'add-channel']) s.root.querySelector(`[data-mc="${action}"]`).disabled = state !== 'connected' || !!s.context.readonly;
            renderList(s); renderHead(s); updateComposer(s);
            if (messages && s.selected) await loadMessages(s);
        } finally {
            s.refreshing = false;
            if (!s.disposed) { s.root.querySelector('[data-mc="refresh"]').disabled = false; s.el('status').classList.remove('mc-busy'); }
        }
    }

    function renderList(s) {
        const list = s.el('conversations'); list.replaceChildren();
        const needle = s.search.toLocaleLowerCase();
        const items = s.conversations.filter(c => (s.filter === 'all' || s.filter === c.kind || s.filter === 'unread' && c.unread > 0) && (!needle || (c.name + ' ' + c.target + ' ' + c.preview).toLocaleLowerCase().includes(needle)));
        items.sort((a, b) => Number(b.favorite) - Number(a.favorite) || b.last_at - a.last_at || a.name.localeCompare(b.name));
        s.root.querySelectorAll('.mc-filters button').forEach(btn => btn.setAttribute('aria-pressed', String(btn.dataset.mc === 'filter-' + s.filter)));
        if (!items.length) {
            const empty = node('div', undefined, 'mc-empty mc-empty-list');
            const emptyIcon = node('span', undefined, 'mc-empty-icon'); emptyIcon.setAttribute('aria-hidden', 'true'); emptyIcon.append(iconEl('mesh'));
            empty.append(emptyIcon, node('p', tr(s, s.settings?.enabled === false ? 'setup' : 'no_conversations')));
            if (s.settings?.enabled !== false) empty.append(node('span', tr(s, 'no_conversations_hint'), 'mc-empty-hint'));
            list.append(empty);
        }
        for (const c of items) {
            const btn = node('button', undefined, 'mc-conversation'); btn.type = 'button'; btn.setAttribute('aria-current', String(c.id === s.selected));
            const avatar = node('span', c.kind === 'channel' ? '#' : (c.name || '?').slice(0, 2).toUpperCase(), 'mc-avatar'); avatar.setAttribute('aria-hidden', 'true');
            avatar.classList.toggle('mc-avatar-channel', c.kind === 'channel');
            avatar.style.setProperty('--mc-avatar-hue', String(c.kind === 'channel' ? 168 : hueOf(c.target)));
            const text = node('span', undefined, 'mc-conversation-copy');
            const line = node('span', undefined, 'mc-conversation-line');
            const name = node('strong');
            if (c.favorite) name.append(iconEl('star', 'mc-star'));
            name.append(document.createTextNode(displayName(s, c)));
            line.append(name);
            if (c.muted) line.append(iconEl('bell-off', 'mc-muted'));
            if (c.last_at) line.append(node('time', formatTime(c.last_at)));
            const preview = node('span', undefined, 'mc-preview');
            if (c.protected) preview.append(iconEl('shield', 'mc-preview-icon'));
            preview.append(document.createTextNode(c.protected ? tr(s, 'protected') : c.preview || tr(s, c.kind === 'channel' ? 'channel' : 'direct')));
            text.append(line, preview); btn.append(avatar, text);
            if (c.unread) btn.append(node('span', String(c.unread), 'mc-unread'));
            btn.addEventListener('click', () => selectConversation(s, c.id)); list.append(btn);
        }
    }

    function displayName(s, c) { return c.kind === 'unknown' ? tr(s, 'unknown') + ' · ' + c.target.slice(0, 12) : c.name || c.target.slice(0, 12); }
    function formatTime(at) { return new Date(at * 1000).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }); }
    function draftKey(s) { const c = current(s); return c ? 'aurago.meshcore.draft.' + c.identity_key + '.' + c.id : ''; }
    function saveDraft(s) { const key = draftKey(s); if (key) try { const text = s.el('compose').value; if (text) localStorage.setItem(key, text); else localStorage.removeItem(key); } catch (_) { /* Browser storage can be disabled. */ } }

    async function selectConversation(s, id) {
        if (s.disposed || !s.conversations.some(c => c.id === id)) return;
        saveDraft(s); s.selected = id; s.messages = []; s.revealed.clear(); s.readSeq = 0; s.query = ''; s.el('query').value = ''; s.el('detail').hidden = true; s.root.classList.add('mc-has-chat');
        s.context.updateWindowContext?.(s.windowId, { conversation_id: id });
        try { s.el('compose').value = localStorage.getItem(draftKey(s)) || ''; } catch (_) { s.el('compose').value = ''; }
        renderList(s); renderHead(s); updateComposer(s); clearError(s);
        try { await loadMessages(s, false, true); if (!s.disposed) s.el('compose').focus(); } catch (error) { showError(s, error); }
    }

    function renderHead(s) {
        const c = current(s);
        s.el('title').textContent = c ? displayName(s, c) : tr(s, 'choose');
        s.el('subtitle').textContent = c ? (!c.active ? tr(s, 'archived') : c.kind === 'channel' ? tr(s, c.channel_kind || 'private') : c.target.slice(0, 12)) : tr(s, 'offline_hint');
        s.root.querySelector('[data-mc="details"]').disabled = !c;
        const avatar = s.el('peer-avatar');
        avatar.hidden = !c;
        if (c) {
            avatar.textContent = c.kind === 'channel' ? '#' : (c.name || '?').slice(0, 2).toUpperCase();
            avatar.classList.toggle('mc-avatar-channel', c.kind === 'channel');
            avatar.style.setProperty('--mc-avatar-hue', String(c.kind === 'channel' ? 168 : hueOf(c.target)));
        }
    }

    function skeletonList() {
        const wrap = node('div', undefined, 'mc-skeleton-list');
        wrap.setAttribute('aria-busy', 'true');
        wrap.append(node('div', undefined, 'mc-skeleton mc-skeleton-in'), node('div', undefined, 'mc-skeleton mc-skeleton-out'), node('div', undefined, 'mc-skeleton mc-skeleton-in'));
        return wrap;
    }

    async function loadMessages(s, older = false, reset = false) {
        if (!s.selected || s.disposed) return;
        const id = s.selected, generation = ++s.generation, wasBottom = nearBottom(s), box = s.el('messages'), oldHeight = box.scrollHeight, oldTop = box.scrollTop;
        const before = older && s.messages.length ? s.messages[0].seq : 0;
        if (reset) { box.replaceChildren(skeletonList()); s.messages = []; }
        const data = await request(s, 'messages?conversation=' + encodeURIComponent(id) + '&before=' + before + '&q=' + encodeURIComponent(s.query));
        if (s.disposed || generation !== s.generation || id !== s.selected) return;
        const rows = data.messages || [];
        const merged = new Map((reset ? [] : s.messages).map(msg => [msg.id, msg])); rows.forEach(msg => merged.set(msg.id, msg));
        s.messages = [...merged.values()].sort((a, b) => a.seq - b.seq);
        if (older || reset || !before && s.messages.length <= 50) s.hasOlder = rows.length === 50;
        renderMessages(s);
        if (older) box.scrollTop = oldTop + box.scrollHeight - oldHeight;
        else if (wasBottom || reset) { box.scrollTop = box.scrollHeight; markRead(s); }
        else { box.scrollTop = oldTop; s.root.querySelector('[data-mc="latest"]').hidden = false; }
    }

    function renderMessages(s) {
        const box = s.el('messages'); box.replaceChildren();
        if (s.hasOlder) { const more = node('button', tr(s, 'older'), 'mc-load-more'); more.type = 'button'; more.addEventListener('click', () => { more.disabled = true; loadMessages(s, true).catch(error => { more.disabled = false; showError(s, error); }); }); box.append(more); }
        if (!s.messages.length) {
            const empty = node('div', undefined, 'mc-empty mc-empty-chat');
            const emptyIcon = node('span', undefined, 'mc-empty-icon'); emptyIcon.setAttribute('aria-hidden', 'true'); emptyIcon.append(iconEl('mesh'));
            empty.append(emptyIcon, node('p', tr(s, s.query ? 'no_results' : 'empty_chat')));
            if (!s.query) empty.append(node('span', tr(s, 'empty_chat_hint'), 'mc-empty-hint'));
            box.append(empty);
        }
        let day = '', prev = null;
        const total = s.messages.length;
        s.messages.forEach((msg, i) => {
            const date = new Date(msg.at * 1000).toLocaleDateString();
            if (date !== day) { box.append(node('div', date, 'mc-day')); day = date; prev = null; }
            const samePrev = prev && prev.direction === msg.direction && prev.origin === msg.origin && msg.at - prev.at < GROUP_GAP_SECONDS;
            const nextMsg = s.messages[i + 1];
            const sameNext = nextMsg && i + 1 < total && new Date(nextMsg.at * 1000).toLocaleDateString() === date && nextMsg.direction === msg.direction && nextMsg.origin === msg.origin && nextMsg.at - msg.at < GROUP_GAP_SECONDS;
            let cls = 'mc-message ' + (msg.direction === 'outgoing' ? 'mc-outgoing' : 'mc-incoming');
            if (samePrev && sameNext) cls += ' mc-group-mid';
            else if (samePrev) cls += ' mc-group-end';
            else if (sameNext) cls += ' mc-group-start';
            if (msg.protected) cls += ' mc-protected';
            const row = node('article', undefined, cls);
            row.dataset.messageId = msg.id;
            if (msg.origin === 'agent') row.append(node('strong', 'AuraGo', 'mc-message-author'));
            const body = node('div', undefined, 'mc-message-text');
            if (msg.protected && !s.revealed.has(msg.id)) body.append(iconEl('shield', 'mc-lock'));
            body.append(document.createTextNode(msg.protected ? s.revealed.get(msg.id) ?? tr(s, 'protected') : msg.text));
            row.append(body);
            const meta = node('div', undefined, 'mc-message-meta'); meta.append(node('time', formatTime(msg.at)));
            if (msg.direction === 'outgoing') { const state = ['sending', 'queued', 'device_accepted', 'delivered', 'not_sent', 'outcome_unknown'].includes(msg.send_state) ? msg.send_state : 'unknown'; const status = node('span', undefined, 'mc-send-state'); status.title = tr(s, 'send_' + state); status.append(iconEl(SEND_STATE_ICONS[state] || 'help', 'mc-state-icon'), document.createTextNode(tr(s, 'send_' + state))); meta.append(status); }
            const action = (label, iconName, fn) => { const btn = node('button'); btn.type = 'button'; btn.title = tr(s, label); btn.append(iconEl(iconName), node('span', tr(s, label), 'mc-sr-only')); btn.addEventListener('click', () => fn(btn).catch(error => showError(s, error))); meta.append(btn); };
            if (msg.protected && !s.revealed.has(msg.id)) action('reveal', 'eye', async btn => { btn.disabled = true; try { const data = await request(s, 'reveal', { id: msg.id }); if (!s.disposed && row.isConnected) { s.revealed.set(msg.id, data.text); body.textContent = data.text; btn.hidden = true; } } finally { btn.disabled = false; } });
            else action('copy', 'copy', async () => { await navigator.clipboard.writeText(msg.text); });
            if (msg.origin === 'manual' && ['not_sent', 'outcome_unknown', 'device_accepted'].includes(msg.send_state)) action('retry', 'refresh', async () => confirmAction(s, 'retry_warning', async () => { s.pendingSend = null; try { localStorage.removeItem('aurago.meshcore.pending.' + s.selected); } catch (_) { /* Optional browser storage. */ } s.el('compose').value = msg.text; saveDraft(s); updateComposer(s); await send(s); }));
            row.append(meta);
            if (msg.parts?.length > 1) row.append(node('small', msg.parts.map(p => p.number + ': ' + tr(s, 'send_' + p.state)).join(' · '), 'mc-part-states'));
            box.append(row);
            prev = msg;
        });
    }

    function markRead(s) {
        const c = current(s); if (!c || s.query || document.hidden || !s.root.getClientRects().length || !nearBottom(s)) return;
        const win = s.root.closest('.vd-window'); if (win && !win.classList.contains('active')) return;
        const seq = Math.max(0, ...s.messages.map(msg => msg.seq)); if (seq <= s.readSeq) return;
        s.readSeq = seq;
        request(s, 'conversation', { conversation: c.id, read: seq }).then(() => { c.unread = 0; if (!s.disposed) renderList(s); }).catch(() => { s.readSeq = 0; });
    }

    function splitPreview(text, limit) {
        text = text.trim(); if (!text) return [];
        if (text.includes('\0') || limit < 16) return null;
        if (encoder.encode(text).length <= limit) return [text];
        const parts = []; let part = '', size = 0;
        for (const char of text) { const bytes = encoder.encode(char).length; if (size + bytes > limit - 6) { parts.push(part); part = ''; size = 0; } part += char; size += bytes; }
        if (part) parts.push(part); if (parts.length > 3) return null;
        return parts.map((value, i) => `[${i + 1}/${parts.length}] ${value}`);
    }

    function updateComposer(s) {
        const c = current(s), text = s.el('compose').value;
        const limit = c?.kind === 'channel' ? (s.settings?.channel_text_limit || Math.min(133, 160 - encoder.encode(s.status.name || '').length - 2)) : 133;
        const parts = splitPreview(text, limit);
        s.el('counter').textContent = `${encoder.encode(text.trim()).length} B · ${parts ? parts.length : '>3'}/3`;
        s.el('parts').replaceChildren(...(parts || []).map(part => node('pre', part)));
        s.el('send').disabled = s.sending || !c?.can_send || !parts?.length || !!s.context.readonly;
        s.el('compose').disabled = !c || s.sending || !!s.context.readonly;
        s.el('send').classList.toggle('mc-busy', s.sending);
        s.el('send-hint').textContent = !parts ? tr(s, 'too_long') : c && !c.can_send ? tr(s, 'send_locked') : tr(s, 'composer_hint');
    }

    async function send(s) {
        if (s.el('send').disabled || s.sending) return;
        const c = current(s), text = s.el('compose').value.trim(); if (!c || !text) return;
        const snapshot = { conversation: c.id, text };
        // Retain this ID after a browser timeout; a repeated click reconciles the
        // same server reservation instead of transmitting another radio message.
        const pendingKey = 'aurago.meshcore.pending.' + c.id;
        if (!s.pendingSend || s.pendingSend.conversation !== c.id) try { s.pendingSend = JSON.parse(localStorage.getItem(pendingKey) || 'null'); } catch (_) { s.pendingSend = null; }
        if (!s.pendingSend || s.pendingSend.conversation !== c.id || s.pendingSend.text !== text) s.pendingSend = { ...snapshot, id: crypto.randomUUID() };
        try { localStorage.setItem(pendingKey, JSON.stringify(s.pendingSend)); } catch (_) { /* In-memory idempotency remains available. */ }
        s.sending = true; updateComposer(s); clearError(s);
        try {
            await request(s, 'send', s.pendingSend);
            if (s.disposed) return;
            s.pendingSend = null;
            try { localStorage.removeItem(pendingKey); } catch (_) { /* Optional browser storage. */ }
            if (s.selected === c.id && s.el('compose').value.trim() === text) { s.el('compose').value = ''; saveDraft(s); }
            await refresh(s);
        } catch (error) { showError(s, error); }
        finally { s.sending = false; if (!s.disposed) { updateComposer(s); s.el('compose').focus(); } }
    }

    async function act(s, action) {
        if (action.startsWith('filter-')) { s.filter = action.slice(7); renderList(s); return; }
        if (action === 'refresh') { clearError(s); await refresh(s); return; }
        if (action === 'back') { s.root.classList.remove('mc-has-chat'); return; }
        if (action === 'latest') { s.el('messages').scrollTop = s.el('messages').scrollHeight; markRead(s); return; }
        if (action === 'details') { s.el('detail').hidden = !s.el('detail').hidden; renderDetail(s); return; }
        if (action === 'add-contact' || action === 'add-channel') { editDialog(s, action); return; }
        if (action === 'self') { selfDialog(s); return; }
        if (action === 'settings') settingsDialog(s);
    }

    function renderDetail(s) {
        const c = current(s), panel = s.el('detail'); panel.replaceChildren(); if (!c) return;
        const head = node('div', undefined, 'mc-detail-head');
        const avatar = node('span', c.kind === 'channel' ? '#' : (c.name || '?').slice(0, 2).toUpperCase(), 'mc-avatar mc-avatar-lg'); avatar.setAttribute('aria-hidden', 'true');
        avatar.classList.toggle('mc-avatar-channel', c.kind === 'channel');
        avatar.style.setProperty('--mc-avatar-hue', String(c.kind === 'channel' ? 168 : hueOf(c.target)));
        const titleWrap = node('div', undefined, 'mc-detail-title');
        titleWrap.append(node('h3', displayName(s, c)), node('p', tr(s, c.kind === 'channel' ? c.channel_kind || 'private' : 'identity')));
        head.append(avatar, titleWrap); panel.append(head);
        const keyChip = node('div', undefined, 'mc-key');
        keyChip.append(node('code', c.kind === 'channel' ? c.identity_key : c.target));
        panel.append(keyChip);
        panel.append(node('p', tr(s, c.kind === 'channel' && c.channel_kind !== 'private' ? 'public_hint' : 'trust_hint'), 'mc-hint'));
        const actions = node('div', undefined, 'mc-detail-actions');
        const action = (key, iconName, fn, disabled = false) => { const btn = node('button', tr(s, key)); btn.type = 'button'; btn.disabled = disabled; btn.prepend(iconEl(iconName)); btn.addEventListener('click', () => fn().catch(error => showError(s, error))); actions.append(btn); };
        action(c.favorite ? 'unfavorite' : 'favorite', 'star', async () => { await request(s, 'conversation', { conversation: c.id, favorite: !c.favorite }); await refresh(s, false); renderDetail(s); });
        action(c.muted ? 'unmute' : 'mute', c.muted ? 'bell' : 'bell-off', async () => { await request(s, 'conversation', { conversation: c.id, muted: !c.muted }); await refresh(s, false); renderDetail(s); });
        action('share', 'share', async () => shareDialog(s, c.id), !c.active || s.status.state !== 'connected');
        action('clear_history', 'eraser', async () => confirmAction(s, 'clear_warning', async () => { await request(s, 'conversation', { conversation: c.id, clear: true }); s.messages = []; await loadMessages(s, false, true); await refresh(s, false); }));
        action('remove', 'trash', async () => confirmAction(s, 'remove_warning', async () => { await manage(s, { action: c.kind === 'channel' ? 'channel_remove' : 'contact_remove', conversation: c.id }); panel.hidden = true; }), !c.active || !['direct', 'channel'].includes(c.kind) || !!s.context.readonly);
        action('close', 'close', async () => { panel.hidden = true; });
        panel.append(actions);
    }

    function dialog(s, title, iconName) {
        s.dialog?.close();
        const el = node('dialog', undefined, 'mc-dialog');
        const head = node('header');
        const titleWrap = node('div', undefined, 'mc-dialog-title');
        titleWrap.append(iconEl(iconName || 'info', 'mc-dialog-icon'), node('h3', tr(s, title)));
        const close = node('button', '×'); close.type = 'button'; close.setAttribute('aria-label', tr(s, 'close')); head.append(titleWrap, close);
        const body = node('div', undefined, 'mc-dialog-body'), error = node('p', '', 'mc-feedback'); error.setAttribute('role', 'alert'); error.hidden = true;
        el.append(head, body, error); s.root.append(el); s.dialog = el;
        close.addEventListener('click', () => el.close());
        el.addEventListener('close', () => { el.replaceChildren(); el.remove(); if (s.dialog === el) s.dialog = null; });
        el.showModal();
        return { el, body, error };
    }

    function field(s, parent, key, type = 'text', value = '') {
        const label = node('label', tr(s, key)); const input = node('input'); input.type = type; input.value = value; input.autocomplete = 'off'; if (type !== 'number' && type !== 'file') input.maxLength = key === 'invitation' ? 2048 : 128; label.append(input); parent.append(label); return input;
    }

    function dialogButton(s, d, key, fn) {
        const btn = node('button', tr(s, key), 'mc-primary'); btn.type = 'button'; d.body.append(btn);
        btn.addEventListener('click', async () => { if (btn.disabled) return; btn.disabled = true; btn.classList.add('mc-busy'); d.error.hidden = true; try { await fn(); } catch (error) { if (d.el.isConnected) { d.error.textContent = errorText(s, error); d.error.hidden = false; } } finally { btn.disabled = false; btn.classList.remove('mc-busy'); } });
        return btn;
    }

    async function manage(s, data) { await request(s, 'manage', { ...data, identity: s.status.identity_key }); await refresh(s); }
    function confirmAction(s, key, fn) { const d = dialog(s, 'confirm', 'alert'); d.body.append(node('p', tr(s, key))); dialogButton(s, d, 'confirm', async () => { await fn(); d.el.close(); }); }

    function editDialog(s, type) {
        const isChannel = type === 'add-channel', d = dialog(s, isChannel ? 'add_channel' : 'add_contact', isChannel ? 'hash' : 'user-plus');
        const invite = field(s, d.body, 'invitation');
        if ('BarcodeDetector' in window) {
            const file = field(s, d.body, 'scan_qr', 'file'); file.accept = 'image/*';
            file.addEventListener('change', async () => { const selected = file.files[0]; if (!selected) return; let bitmap; try { if (selected.size > 5 * 1024 * 1024) throw Error('image_size'); bitmap = await createImageBitmap(selected); const codes = await new BarcodeDetector({ formats: ['qr_code'] }).detect(bitmap); if (!codes.length) throw Error('qr_missing'); invite.value = codes[0].rawValue; } catch (_) { d.error.hidden = false; d.error.textContent = tr(s, 'error_invalid_invitation'); } finally { bitmap?.close(); file.value = ''; } });
        }
        const name = field(s, d.body, 'name'); name.maxLength = 31;
        const label = node('label', tr(s, 'type')), select = node('select'); label.append(select); d.body.append(label);
        const options = isChannel ? [['public', 'public'], ['hashtag', 'hashtag'], ['private', 'private']] : [['1', 'companion'], ['2', 'repeater'], ['3', 'room'], ['4', 'sensor']];
        for (const [value, key] of options) select.append(new Option(tr(s, key), value));
        const key = field(s, d.body, isChannel ? 'secret' : 'public_key', isChannel ? 'password' : 'text');
        if (isChannel) {
            const hint = node('p', tr(s, 'secret_hint'), 'mc-hint'); d.body.append(hint);
            const syncKind = () => { key.parentElement.hidden = hint.hidden = select.value !== 'private'; };
            select.addEventListener('change', syncKind); syncKind();
        }
        d.body.append(node('p', tr(s, 'trust_hint'), 'mc-hint'));
        dialogButton(s, d, 'save', async () => {
            await manage(s, { action: isChannel ? 'channel_add' : 'contact_add', invitation: invite.value.trim(), name: name.value.trim(), key: isChannel ? '' : key.value.trim(), type: isChannel ? 0 : Number(select.value), kind: isChannel ? select.value : '', secret: isChannel ? key.value.trim() : '' });
            key.value = ''; invite.value = ''; d.el.close();
        });
    }

    function shareDialog(s, id) {
        const d = dialog(s, 'share', 'qr'); d.body.append(node('p', tr(s, 'share_hint')));
        const show = dialogButton(s, d, 'show_invitation', async () => {
            const data = await request(s, 'invitation', { identity: s.status.identity_key, conversation: id });
            if (!d.el.isConnected || s.disposed) return;
            show.hidden = true; const qr = node('div', undefined, 'mc-qr'); d.body.append(qr);
            if (window.QRCode) new window.QRCode(qr, { text: data.invitation, width: 220, height: 220, colorDark: '#000000', colorLight: '#ffffff' });
            const code = node('textarea'); code.value = data.invitation; code.readOnly = true; code.setAttribute('aria-label', tr(s, 'invitation')); d.body.append(code);
            dialogButton(s, d, 'copy', async () => navigator.clipboard.writeText(code.value));
        });
    }

    function selfDialog(s) {
        const d = dialog(s, 'self', 'self'); d.body.append(node('p', s.status.name || 'MeshCore'), node('code', s.status.identity_key || ''), node('p', tr(s, 'advert_hint')));
        dialogButton(s, d, 'share', async () => { d.el.close(); shareDialog(s, 'self'); });
        dialogButton(s, d, 'zero_hop', async () => { await manage(s, { action: 'advert', flood: false }); d.el.close(); });
        dialogButton(s, d, 'flood', async () => { await manage(s, { action: 'advert', flood: true }); d.el.close(); });
    }

    function settingsDialog(s) {
        const d = dialog(s, 'settings', 'settings');
        const days = field(s, d.body, 'history_days', 'number', s.settings?.history_days || 90); days.min = 1; days.max = 3650;
        const messages = field(s, d.body, 'history_messages', 'number', s.settings?.history_messages || 10000); messages.min = 1; messages.max = 100000;
        d.body.append(node('p', tr(s, 'hardware_unverified'), 'mc-hint'));
        dialogButton(s, d, 'save', async () => { if (!days.reportValidity() || !messages.reportValidity()) return; await request(s, 'settings', { history_days: Number(days.value), history_messages: Number(messages.value) }); d.el.close(); await refresh(s); });
        if (s.status.state === 'binding_changed') { d.body.append(node('p', tr(s, 'mapping_hint'))); dialogButton(s, d, 'confirm_mapping', async () => { await manage(s, { action: 'confirm_mapping' }); d.el.close(); }); }
    }

    function openConversation(windowId, context) { const s = instances.get(windowId); if (s && context?.conversation_id) refresh(s, false).then(() => selectConversation(s, context.conversation_id)).catch(error => showError(s, error)); }
    function dispose(windowId) {
        const s = instances.get(windowId); if (!s) return;
        saveDraft(s); s.disposed = true; s.generation++;
        clearInterval(s.poll); clearTimeout(s.searchTimer); clearTimeout(s.refreshTimer);
        s.requests.forEach(controller => controller.abort()); s.requests.clear(); s.dialog?.close();
        document.removeEventListener('aurago:meshcore-change', s.onChange); document.removeEventListener('visibilitychange', s.onVisible);
        s.messages = []; s.revealed.clear(); s.pendingSend = null; instances.delete(windowId);
    }
    window.MeshCoreApp = { render, dispose, openConversation };
})();
