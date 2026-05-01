/* ── Integrations Drawer Module ───────────────────────────────────────────── */

window.IntegrationsDrawer = (function () {
    'use strict';

    let isOpen = false;
    let webhosts = [];

    const drawer = document.getElementById('integrations-drawer');
    const backdrop = document.getElementById('integrations-backdrop');
    const listEl = document.getElementById('integrations-list');
    const toggleBtn = document.getElementById('integrations-toggle-btn');
    const closeBtn = document.getElementById('integrations-drawer-close');

    function t(key) {
        return typeof I18N !== 'undefined' && I18N[key] ? I18N[key] : key;
    }

    function escapeHtml(str) {
        const div = document.createElement('div');
        div.textContent = str || '';
        return div.innerHTML;
    }

    function escapeAttr(str) {
        return String(str || '').replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/'/g, '&#39;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
    }

    async function fetchWebhosts() {
        if (!listEl) return;
        listEl.innerHTML = `<div class="integrations-empty">${escapeHtml(t('chat.integrations_loading'))}</div>`;
        try {
            const res = await fetch('/api/integrations/webhosts', { credentials: 'same-origin' });
            if (!res.ok) throw new Error(`HTTP ${res.status}`);
            const data = await res.json();
            webhosts = Array.isArray(data.webhosts) ? data.webhosts : [];
        } catch (err) {
            console.error('Failed to fetch integrations:', err);
            webhosts = [];
            listEl.innerHTML = `<div class="integrations-empty">${escapeHtml(t('chat.integrations_error'))}</div>`;
            return;
        }
        renderList();
    }

    function renderList() {
        if (!listEl) return;
        if (!webhosts.length) {
            listEl.innerHTML = `<div class="integrations-empty">${escapeHtml(t('chat.integrations_empty'))}</div>`;
            return;
        }
        listEl.innerHTML = webhosts.map(item => {
            const status = item.status || 'starting';
            const isRunning = status === 'running';
            const url = item.url || '';
            const homepageUrl = isRunning && url ? url : '';
            return `
                <div class="integration-item" data-integration-id="${escapeAttr(item.id)}">
                    <div class="integration-icon">${window.chatUiIconMarkup ? window.chatUiIconMarkup(item.icon || 'web') : ''}</div>
                    <div class="integration-main">
                        <div class="integration-name">${escapeHtml(item.name || item.id)}</div>
                        ${item.description ? `<div class="integration-desc">${escapeHtml(item.description)}</div>` : ''}
                        <div class="integration-meta">
                            <span class="integration-status ${escapeAttr(status)}" aria-hidden="true"></span>
                            <span class="integration-status-text">${escapeHtml(t('chat.integrations_status_' + status) || status)}</span>
                        </div>
                        ${homepageUrl ? `
                        <a class="integration-homepage" href="${escapeAttr(homepageUrl)}" target="_blank" rel="noopener noreferrer">
                            ${window.chatUiIconMarkup ? window.chatUiIconMarkup('link', 'integration-homepage-icon') : ''}
                            <span class="integration-homepage-url">${escapeHtml(homepageUrl)}</span>
                        </a>` : ''}
                    </div>
                    ${homepageUrl ? `<button type="button" class="integration-open" data-url="${escapeAttr(homepageUrl)}">${escapeHtml(t('chat.integrations_open'))}</button>` : ''}
                </div>`;
        }).join('');

        listEl.querySelectorAll('.integration-open').forEach(btn => {
            btn.addEventListener('click', () => {
                const url = btn.dataset.url;
                if (!url) return;
                window.open(url, '_blank', 'noopener,noreferrer');
            });
        });
    }

    function open() {
        if (!drawer) return;
        isOpen = true;
        drawer.classList.add('open');
        if (backdrop) backdrop.classList.add('active');
        if (toggleBtn) toggleBtn.setAttribute('aria-expanded', 'true');
        fetchWebhosts();
    }

    function close() {
        if (!drawer) return;
        isOpen = false;
        drawer.classList.remove('open');
        if (backdrop) backdrop.classList.remove('active');
        if (toggleBtn) toggleBtn.setAttribute('aria-expanded', 'false');
    }

    function toggle() {
        if (isOpen) close();
        else open();
    }

    if (toggleBtn) toggleBtn.addEventListener('click', toggle);
    if (closeBtn) closeBtn.addEventListener('click', close);
    if (backdrop) backdrop.addEventListener('click', close);
    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape' && isOpen) close();
    });

    return {
        open,
        close,
        refresh: fetchWebhosts
    };
})();
