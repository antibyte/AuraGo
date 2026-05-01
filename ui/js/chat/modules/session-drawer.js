/* ── Session Drawer Module ────────────────────────────────────────────────────
 * Manages the chat session list, creation, switching, and deletion.
 * Exposes a global `window.SessionDrawer` object used by main.js.
 * ──────────────────────────────────────────────────────────────────────────── */

window.SessionDrawer = (function () {
    'use strict';

    // ── State ──
    let activeSessionId = localStorage.getItem('aurago-session-id') || 'default';
    let sessions = [];
    let isOpen = false;

    // ── DOM refs ──
    const drawer = document.getElementById('session-drawer');
    const backdrop = document.getElementById('session-backdrop');
    const listEl = document.getElementById('session-list');
    const toggleBtn = document.getElementById('session-toggle-btn');
    const closeBtn = document.getElementById('session-drawer-close');
    const newBtn = document.getElementById('session-new-btn');

    // ── Helpers ──
    function t(key) {
        return typeof I18N !== 'undefined' && I18N[key] ? I18N[key] : key;
    }

    function escapeHtml(str) {
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    }

    function escapeAttr(str) {
        return String(str).replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/'/g, '&#39;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
    }

    function parseDate(dateStr) {
        if (!dateStr) return null;
        // Strategy 1: Already valid ISO 8601 / RFC3339
        let d = new Date(dateStr);
        if (!isNaN(d.getTime())) return d;
        // Strategy 2: SQLite format "2006-01-02 15:04:05" → add T and Z
        d = new Date(dateStr.replace(' ', 'T') + 'Z');
        if (!isNaN(d.getTime())) return d;
        // Strategy 3: SQLite format without Z
        d = new Date(dateStr.replace(' ', 'T'));
        if (!isNaN(d.getTime())) return d;
        // Strategy 4: Replace space with T, keep existing timezone info
        d = new Date(dateStr.replace(' ', 'T').replace(/(\d{2}:\d{2}:\d{2})$/, '$1Z'));
        if (!isNaN(d.getTime())) return d;
        return null;
    }

    function formatTimeAgo(dateStr) {
        if (!dateStr) return '';
        try {
            const date = parseDate(dateStr);
            if (!date) return dateStr;
            const now = new Date();
            const diffMs = now - date;
            const diffMin = Math.floor(diffMs / 60000);
            if (diffMin < 1) return t('chat.sessions_just_now');
            if (diffMin < 60) return t('chat.sessions_minutes_ago').replace('{n}', diffMin);
            const diffH = Math.floor(diffMin / 60);
            if (diffH < 24) return t('chat.sessions_hours_ago').replace('{n}', diffH);
            const diffD = Math.floor(diffH / 24);
            if (diffD < 7) return t('chat.sessions_days_ago').replace('{n}', diffD);
            return date.toLocaleDateString();
        } catch {
            return dateStr;
        }
    }

    // ── API calls ──
    async function fetchSessions() {
        try {
            const res = await fetch('/api/chat/sessions', { credentials: 'same-origin' });
            if (!res.ok) return;
            const data = await res.json();
            sessions = data.sessions || [];
        } catch (err) {
            console.error('Failed to fetch sessions:', err);
        }
    }

    async function createSession() {
        try {
            const res = await fetch('/api/chat/sessions', { method: 'POST', credentials: 'same-origin' });
            if (!res.ok) return null;
            const data = await res.json();
            return data.session;
        } catch (err) {
            console.error('Failed to create session:', err);
            return null;
        }
    }

    async function deleteSession(id) {
        try {
            await fetch('/api/chat/sessions/' + encodeURIComponent(id), { method: 'DELETE', credentials: 'same-origin' });
        } catch (err) {
            console.error('Failed to delete session:', err);
        }
    }

    // ── Rendering ──
    function renderList() {
        if (!listEl) return;
        if (sessions.length === 0) {
            listEl.innerHTML = `<div class="session-empty">${escapeHtml(t('chat.sessions_empty'))}</div>`;
            return;
        }
        listEl.innerHTML = sessions.map(s => {
            const isActive = s.id === activeSessionId;
            const preview = s.preview || t('chat.sessions_no_preview');
            const timeAgo = formatTimeAgo(s.last_active_at);
            return `
                <div class="session-item${isActive ? ' session-active' : ''}" data-session-id="${escapeAttr(s.id)}">
                    <div class="session-item-content">
                        <div class="session-item-preview">${escapeHtml(preview)}</div>
                        <div class="session-item-meta">
                            <span class="session-item-time">${escapeHtml(timeAgo)}</span>
                            ${s.message_count > 0 ? `<span class="session-item-count">${s.message_count}</span>` : ''}
                        </div>
                    </div>
                    ${!isActive ? `<button class="session-item-delete" data-delete-id="${escapeAttr(s.id)}" title="${escapeAttr(t('chat.sessions_delete'))}">${window.chatUiIconMarkup ? window.chatUiIconMarkup('close') : ''}</button>` : ''}
                </div>`;
        }).join('');

        // Bind click events
        listEl.querySelectorAll('.session-item').forEach(el => {
            el.addEventListener('click', (e) => {
                // Ignore if clicking delete button
                if (e.target.closest('.session-item-delete')) return;
                const id = el.dataset.sessionId;
                if (id && id !== activeSessionId) {
                    switchSession(id);
                }
                close();
            });
        });

        listEl.querySelectorAll('.session-item-delete').forEach(btn => {
            btn.addEventListener('click', async (e) => {
                e.stopPropagation();
                const id = btn.dataset.deleteId;
                if (id) {
                    await deleteSession(id);
                    await fetchSessions();
                    renderList();
                }
            });
        });
    }

    // ── Session switching ──
    async function switchSession(sessionId) {
        activeSessionId = sessionId;
        localStorage.setItem('aurago-session-id', sessionId);

        // Notify main.js to reload chat for the new session
        if (typeof window.onSessionSwitch === 'function') {
            window.onSessionSwitch(sessionId);
        }
    }

    // ── New session ──
    async function handleNewSession() {
        const sess = await createSession();
        if (sess) {
            activeSessionId = sess.id;
            localStorage.setItem('aurago-session-id', sess.id);
            if (typeof window.onSessionSwitch === 'function') {
                window.onSessionSwitch(sess.id);
            }
            await fetchSessions();
            renderList();
            close();
        }
    }

    // ── Open / Close ──
    function open() {
        if (!drawer) return;
        isOpen = true;
        drawer.classList.add('open');
        backdrop.classList.add('active');
        if (toggleBtn) toggleBtn.setAttribute('aria-expanded', 'true');
        fetchSessions().then(renderList);
    }

    function close() {
        if (!drawer) return;
        isOpen = false;
        drawer.classList.remove('open');
        backdrop.classList.remove('active');
        if (toggleBtn) toggleBtn.setAttribute('aria-expanded', 'false');
    }

    function toggle() {
        if (isOpen) close();
        else open();
    }

    // ── Ghost session guard ──
    // Validate that the stored session ID still exists on the server.
    // If it was rotated or deleted, fall back to "default".
    async function validateStoredSession() {
        if (activeSessionId === 'default') return;
        try {
            const res = await fetch('/api/chat/sessions/' + encodeURIComponent(activeSessionId), { credentials: 'same-origin' });
            if (!res.ok) {
                // Session no longer exists – fall back
                activeSessionId = 'default';
                localStorage.setItem('aurago-session-id', 'default');
                return;
            }
            const data = await res.json();
            if (!data.session) {
                activeSessionId = 'default';
                localStorage.setItem('aurago-session-id', 'default');
            }
        } catch (_) {
            // Network error – keep current, don't block init
        }
    }

    // ── Init ──
    function init() {
        if (toggleBtn) {
            toggleBtn.addEventListener('click', toggle);
        }
        if (closeBtn) {
            closeBtn.addEventListener('click', close);
        }
        if (backdrop) {
            backdrop.addEventListener('click', close);
        }
        if (newBtn) {
            newBtn.addEventListener('click', handleNewSession);
        }
        // Validate stored session on page load
        validateStoredSession();
    }

    // ── Public API ──
    return {
        init,
        open,
        close,
        toggle,
        getActiveSessionId: () => activeSessionId,
        setActiveSessionId: (id) => {
            activeSessionId = id;
            localStorage.setItem('aurago-session-id', id);
        },
        refresh: () => fetchSessions().then(renderList),
    };
})();
