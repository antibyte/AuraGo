(function () {
    'use strict';
    const instances = new Map();
    const API = '/api/meshcore/messenger/';
    const encoder = new TextEncoder();
    const esc = value => String(value ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
    const node = (tag, text, cls) => { const el = document.createElement(tag); if (text !== undefined) el.textContent = text; if (cls) el.className = cls; return el; };
    const tr = (s, key, vars) => s.context.t('desktop.meshcore_' + key, vars);
    const button = (s, key, action, cls = '') => `<button type="button" class="${cls}" data-mc="${action}">${esc(tr(s, key))}</button>`;

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
            <header class="mc-toolbar"><div class="mc-brand"><strong>MeshCore</strong><span data-mc-role="status" role="status"></span></div><div class="mc-actions">${button(s, 'refresh', 'refresh')}${button(s, 'self', 'self')}${button(s, 'settings', 'settings')}<a href="/config#meshcore" target="_blank" rel="noopener">${esc(tr(s, 'connection'))}</a></div></header>
            <div class="mc-feedback" data-mc-role="error" role="alert" hidden></div>
            <div class="mc-body"><aside class="mc-sidebar"><div class="mc-sidebar-tools"><input type="search" data-mc-role="search" aria-label="${esc(tr(s, 'search'))}" placeholder="${esc(tr(s, 'search'))}"><div class="mc-filters" role="group" aria-label="${esc(tr(s, 'filter'))}">${['all', 'direct', 'channel', 'unread'].map(key => button(s, key, 'filter-' + key)).join('')}</div><div class="mc-actions">${button(s, 'add_contact', 'add-contact')}${button(s, 'add_channel', 'add-channel')}</div></div><nav class="mc-conversations" aria-label="${esc(tr(s, 'conversations'))}" data-mc-role="conversations"></nav></aside>
            <main class="mc-chat"><header class="mc-chat-head">${button(s, 'back', 'back', 'mc-back')}<div class="mc-chat-title"><strong data-mc-role="title">${esc(tr(s, 'choose'))}</strong><span data-mc-role="subtitle"></span></div>${button(s, 'details', 'details')}</header>
            <div class="mc-history-search"><input type="search" data-mc-role="query" aria-label="${esc(tr(s, 'search_history'))}" placeholder="${esc(tr(s, 'search_history'))}"></div>
            <div class="mc-messages" data-mc-role="messages" tabindex="0" aria-label="${esc(tr(s, 'messages'))}"><p class="mc-empty">${esc(tr(s, 'choose'))}</p></div>
            <button type="button" class="mc-new" data-mc="latest" hidden>${esc(tr(s, 'new_messages'))}</button>
            <form class="mc-composer"><label class="mc-sr-only" for="mc-compose-${esc(windowId)}">${esc(tr(s, 'message'))}</label><textarea id="mc-compose-${esc(windowId)}" data-mc-role="compose" rows="2" maxlength="1200" placeholder="${esc(tr(s, 'message'))}"></textarea><div class="mc-compose-footer"><span data-mc-role="counter" aria-live="polite"></span><button type="submit" class="mc-primary" data-mc-role="send">${esc(tr(s, 'send'))}</button></div><details class="mc-parts"><summary>${esc(tr(s, 'preview'))}</summary><div data-mc-role="parts"></div></details><p class="mc-hint" data-mc-role="send-hint"></p></form>
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
        if (!items.length) list.append(node('p', tr(s, s.settings?.enabled === false ? 'setup' : 'no_conversations'), 'mc-empty'));
        for (const c of items) {
            const btn = node('button', undefined, 'mc-conversation'); btn.type = 'button'; btn.setAttribute('aria-current', String(c.id === s.selected));
            const avatar = node('span', c.kind === 'channel' ? '#' : (c.name || '?').slice(0, 2).toUpperCase(), 'mc-avatar'); avatar.setAttribute('aria-hidden', 'true');
            const text = node('span', undefined, 'mc-conversation-copy');
            const line = node('span', undefined, 'mc-conversation-line'); line.append(node('strong', (c.favorite ? '★ ' : '') + displayName(s, c)));
            if (c.last_at) line.append(node('time', formatTime(c.last_at)));
            const preview = node('span', c.protected ? tr(s, 'protected') : c.preview || tr(s, c.kind === 'channel' ? 'channel' : 'direct'), 'mc-preview');
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
        const c = current(s); s.el('title').textContent = c ? displayName(s, c) : tr(s, 'choose');
        s.el('subtitle').textContent = c ? (!c.active ? tr(s, 'archived') : c.kind === 'channel' ? tr(s, c.channel_kind || 'private') : c.target.slice(0, 12)) : tr(s, 'offline_hint');
        s.root.querySelector('[data-mc="details"]').disabled = !c;
    }

    async function loadMessages(s, older = false, reset = false) {
        if (!s.selected || s.disposed) return;
        const id = s.selected, generation = ++s.generation, wasBottom = nearBottom(s), box = s.el('messages'), oldHeight = box.scrollHeight, oldTop = box.scrollTop;
        const before = older && s.messages.length ? s.messages[0].seq : 0;
        if (reset) { box.replaceChildren(node('p', tr(s, 'loading'), 'mc-empty mc-busy')); s.messages = []; }
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
        if (!s.messages.length) box.append(node('p', tr(s, s.query ? 'no_results' : 'empty_chat'), 'mc-empty'));
        let day = '';
        for (const msg of s.messages) {
            const date = new Date(msg.at * 1000).toLocaleDateString();
            if (date !== day) { box.append(node('div', date, 'mc-day')); day = date; }
            const row = node('article', undefined, 'mc-message ' + (msg.direction === 'outgoing' ? 'mc-outgoing' : 'mc-incoming'));
            row.dataset.messageId = msg.id;
            if (msg.origin === 'agent') row.append(node('strong', 'AuraGo', 'mc-message-author'));
            const body = node('div', msg.protected ? s.revealed.get(msg.id) ?? tr(s, 'protected') : msg.text, 'mc-message-text'); row.append(body);
            const meta = node('div', undefined, 'mc-message-meta'); meta.append(node('time', formatTime(msg.at)));
            if (msg.direction === 'outgoing') { const state = ['sending', 'queued', 'device_accepted', 'delivered', 'not_sent', 'outcome_unknown'].includes(msg.send_state) ? msg.send_state : 'unknown'; const status = node('span'); const icon = node('span', ({ sending: '◷', queued: '◷', device_accepted: '✓', delivered: '✓✓', not_sent: '×' })[state] || '?'); icon.setAttribute('aria-hidden', 'true'); status.append(icon, document.createTextNode(' ' + tr(s, 'send_' + state))); meta.append(status); }
            const action = (label, fn) => { const btn = node('button', tr(s, label)); btn.type = 'button'; btn.addEventListener('click', () => fn(btn).catch(error => showError(s, error))); meta.append(btn); };
            if (msg.protected && !s.revealed.has(msg.id)) action('reveal', async btn => { btn.disabled = true; try { const data = await request(s, 'reveal', { id: msg.id }); if (!s.disposed && row.isConnected) { s.revealed.set(msg.id, data.text); body.textContent = data.text; btn.hidden = true; } } finally { btn.disabled = false; } });
            else action('copy', async () => { await navigator.clipboard.writeText(msg.text); });
            if (msg.origin === 'manual' && ['not_sent', 'outcome_unknown', 'device_accepted'].includes(msg.send_state)) action('retry', async () => confirmAction(s, 'retry_warning', async () => { s.pendingSend = null; try { localStorage.removeItem('aurago.meshcore.pending.' + s.selected); } catch (_) { /* Optional browser storage. */ } s.el('compose').value = msg.text; saveDraft(s); updateComposer(s); await send(s); }));
            row.append(meta);
            if (msg.parts?.length > 1) row.append(node('small', msg.parts.map(p => p.number + ': ' + tr(s, 'send_' + p.state)).join(' · '), 'mc-part-states'));
            box.append(row);
        }
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
        panel.append(node('h3', displayName(s, c)), node('p', tr(s, c.kind === 'channel' ? c.channel_kind || 'private' : 'identity')), node('code', c.kind === 'channel' ? c.identity_key : c.target));
        panel.append(node('p', tr(s, c.kind === 'channel' && c.channel_kind !== 'private' ? 'public_hint' : 'trust_hint'), 'mc-hint'));
        const action = (key, fn, disabled = false) => { const btn = node('button', tr(s, key)); btn.type = 'button'; btn.disabled = disabled; btn.addEventListener('click', () => fn().catch(error => showError(s, error))); panel.append(btn); };
        action(c.favorite ? 'unfavorite' : 'favorite', async () => { await request(s, 'conversation', { conversation: c.id, favorite: !c.favorite }); await refresh(s, false); renderDetail(s); });
        action(c.muted ? 'unmute' : 'mute', async () => { await request(s, 'conversation', { conversation: c.id, muted: !c.muted }); await refresh(s, false); renderDetail(s); });
        action('share', async () => shareDialog(s, c.id), !c.active || s.status.state !== 'connected');
        action('clear_history', async () => confirmAction(s, 'clear_warning', async () => { await request(s, 'conversation', { conversation: c.id, clear: true }); s.messages = []; await loadMessages(s, false, true); await refresh(s, false); }));
        action('remove', async () => confirmAction(s, 'remove_warning', async () => { await manage(s, { action: c.kind === 'channel' ? 'channel_remove' : 'contact_remove', conversation: c.id }); panel.hidden = true; }), !c.active || !['direct', 'channel'].includes(c.kind) || !!s.context.readonly);
        action('close', async () => { panel.hidden = true; });
    }

    function dialog(s, title) {
        s.dialog?.close();
        const el = node('dialog', undefined, 'mc-dialog');
        const head = node('header'); const close = node('button', '×'); close.type = 'button'; close.setAttribute('aria-label', tr(s, 'close')); head.append(node('h3', tr(s, title)), close);
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
    function confirmAction(s, key, fn) { const d = dialog(s, 'confirm'); d.body.append(node('p', tr(s, key))); dialogButton(s, d, 'confirm', async () => { await fn(); d.el.close(); }); }

    function editDialog(s, type) {
        const isChannel = type === 'add-channel', d = dialog(s, isChannel ? 'add_channel' : 'add_contact');
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
        const d = dialog(s, 'share'); d.body.append(node('p', tr(s, 'share_hint')));
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
        const d = dialog(s, 'self'); d.body.append(node('p', s.status.name || 'MeshCore'), node('code', s.status.identity_key || ''), node('p', tr(s, 'advert_hint')));
        dialogButton(s, d, 'share', async () => { d.el.close(); shareDialog(s, 'self'); });
        dialogButton(s, d, 'zero_hop', async () => { await manage(s, { action: 'advert', flood: false }); d.el.close(); });
        dialogButton(s, d, 'flood', async () => { await manage(s, { action: 'advert', flood: true }); d.el.close(); });
    }

    function settingsDialog(s) {
        const d = dialog(s, 'settings');
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
