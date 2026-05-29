(function () {
    'use strict';

    const instances = new Map();
    const DEBOUNCE_MS = 300;

    function esc(value) {
        return String(value == null ? '' : value).replace(/[&<>'"]/g, ch => ({
            '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;'
        }[ch]));
    }

    function t(context, key, vars) {
        return context && typeof context.t === 'function' ? context.t(key, vars) : key;
    }

    function avatarInitials(name) {
        const parts = String(name || '').trim().split(/\s+/);
        if (parts.length >= 2) return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
        return (parts[0] ? parts[0][0] : '?').toUpperCase();
    }

    function daysUntilBirthday(isoDate) {
        if (!isoDate || isoDate.length < 10) return -1;
        const today = new Date();
        today.setHours(0, 0, 0, 0);
        const mm = isoDate.substring(5, 7);
        const dd = isoDate.substring(8, 10);
        let year = today.getFullYear();
        let next = new Date(year, parseInt(mm, 10) - 1, parseInt(dd, 10));
        if (next < today) next = new Date(year + 1, parseInt(mm, 10) - 1, parseInt(dd, 10));
        return Math.round((next - today) / 86400000);
    }

    function formatDate(isoDate) {
        if (!isoDate || isoDate.length < 10) return '';
        const parts = isoDate.substring(0, 10).split('-');
        if (parts.length !== 3) return isoDate;
        return parts[1] + '/' + parts[2] + '/' + parts[0];
    }

    function render(host, windowId, context) {
        if (!host) return;
        dispose(windowId);

        const inst = {
            windowId,
            context,
            contacts: [],
            kgPersons: [],
            upcoming: [],
            filtered: [],
            selectedContact: null,
            viewMode: 'grid',
            sidebarFilter: 'all',
            searchQuery: '',
            semanticMode: false,
            searchTimer: null,
            detailOpen: false,
            modalOpen: false,
            host,
            cleanup: []
        };
        instances.set(windowId, inst);

        host.innerHTML = `<div class="vd-people">
            <div class="vd-people-toolbar">
                <div class="vd-people-search-wrap">
                    <input class="vd-people-search" type="text" placeholder="${esc(t(context, 'desktop.people_search_placeholder'))}" autocomplete="off" spellcheck="false">
                    <button class="vd-people-semantic-toggle" type="button" title="${esc(t(context, 'desktop.people_semantic_hint'))}">KG</button>
                </div>
                <div class="vd-people-toolbar-actions">
                    <div class="vd-people-view-toggle">
                        <button class="vd-people-view-btn active" data-view="grid" type="button" title="${esc(t(context, 'desktop.people_grid_view'))}">&#9638;</button>
                        <button class="vd-people-view-btn" data-view="list" type="button" title="${esc(t(context, 'desktop.people_list_view'))}">&#9776;</button>
                    </div>
                    <button class="vd-people-add-btn" type="button">+ ${esc(t(context, 'desktop.people_add'))}</button>
                </div>
            </div>
            <div class="vd-people-body">
                <aside class="vd-people-sidebar">
                    <div class="vd-people-sidebar-section">
                        <button class="vd-people-sidebar-item active" data-filter="all" type="button">${esc(t(context, 'desktop.people_all'))}</button>
                    </div>
                    <div class="vd-people-sidebar-section vd-people-sidebar-groups"></div>
                    <div class="vd-people-sidebar-section">
                        <div class="vd-people-sidebar-label">${esc(t(context, 'desktop.people_kg_section'))}</div>
                        <button class="vd-people-sidebar-item" data-filter="kg" type="button">${esc(t(context, 'desktop.people_kg_persons'))}</button>
                    </div>
                    <div class="vd-people-sidebar-section">
                        <div class="vd-people-sidebar-label">${esc(t(context, 'desktop.people_upcoming_birthdays'))}</div>
                        <div class="vd-people-sidebar-birthdays"></div>
                    </div>
                </aside>
                <main class="vd-people-content">
                    <div class="vd-people-loading">${esc(t(context, 'desktop.loading'))}</div>
                </main>
                <aside class="vd-people-detail" hidden></aside>
            </div>
            <div class="vd-people-modal-backdrop">
                <div class="vd-people-modal" role="dialog" aria-modal="true"></div>
            </div>
        </div>`;

        wireEvents(inst);
        loadData(inst);
    }

    function wireEvents(inst) {
        const { host, context } = inst;
        const searchInput = host.querySelector('.vd-people-search');
        const semanticBtn = host.querySelector('.vd-people-semantic-toggle');
        const viewBtns = host.querySelectorAll('.vd-people-view-btn');
        const addBtn = host.querySelector('.vd-people-add-btn');
        const sidebar = host.querySelector('.vd-people-sidebar');
        const content = host.querySelector('.vd-people-content');
        const detail = host.querySelector('.vd-people-detail');
        const modalBackdrop = host.querySelector('.vd-people-modal-backdrop');

        searchInput.addEventListener('input', () => {
            if (inst.searchTimer) clearTimeout(inst.searchTimer);
            inst.searchTimer = setTimeout(() => {
                inst.searchQuery = searchInput.value.trim();
                if (inst.searchQuery.length >= 2) {
                    searchContacts(inst);
                } else {
                    inst.filtered = inst.contacts;
                    renderContent(inst);
                }
            }, DEBOUNCE_MS);
        });
        inst.cleanup.push(() => { if (inst.searchTimer) clearTimeout(inst.searchTimer); });

        semanticBtn.addEventListener('click', () => {
            inst.semanticMode = !inst.semanticMode;
            semanticBtn.classList.toggle('active', inst.semanticMode);
            semanticBtn.textContent = inst.semanticMode ? 'KG ' + t(context, 'desktop.people_semantic_search') : 'KG';
            if (inst.searchQuery.length >= 2) searchContacts(inst);
        });

        viewBtns.forEach(btn => {
            btn.addEventListener('click', () => {
                inst.viewMode = btn.dataset.view;
                viewBtns.forEach(b => b.classList.toggle('active', b === btn));
                renderContent(inst);
            });
        });

        addBtn.addEventListener('click', () => openModal(inst, null));

        sidebar.addEventListener('click', event => {
            const item = event.target.closest('.vd-people-sidebar-item');
            if (!item) return;
            sidebar.querySelectorAll('.vd-people-sidebar-item').forEach(i => i.classList.remove('active'));
            item.classList.add('active');
            inst.sidebarFilter = item.dataset.filter || 'all';
            applyFilter(inst);
        });

        content.addEventListener('click', event => {
            const card = event.target.closest('.vd-people-card, .vd-people-list-row');
            if (!card) return;
            const id = card.dataset.contactId;
            const contact = inst.contacts.find(c => c.id === id);
            if (contact) openDetail(inst, contact);
        });

        detail.addEventListener('click', event => {
            if (event.target.closest('.vd-people-detail-close')) {
                closeDetail(inst);
                return;
            }
            if (event.target.closest('.vd-people-detail-edit')) {
                if (inst.selectedContact) openModal(inst, inst.selectedContact);
                return;
            }
            if (event.target.closest('.vd-people-detail-delete')) {
                if (inst.selectedContact) deleteContact(inst, inst.selectedContact);
                return;
            }
        });

        modalBackdrop.addEventListener('click', event => {
            if (event.target === modalBackdrop) closeModal(inst);
        });
    }

    function openModal(inst, contact) {
        const backdrop = inst.host.querySelector('.vd-people-modal-backdrop');
        const modal = inst.host.querySelector('.vd-people-modal');
        if (!backdrop || !modal) return;
        
        backdrop.classList.add('active');
        inst.modalOpen = true;

        const form = modal.querySelector('form');
        const cancelBtn = modal.querySelector('.vd-people-modal-cancel');
        cancelBtn.addEventListener('click', () => closeModal(inst));
        form.addEventListener('submit', async (event) => {
            event.preventDefault();
            const fd = new FormData(form);
            const data = {};
            fd.forEach((v, k) => { data[k] = String(v).trim(); });
            if (!data.name) return;
            try {
                if (isEdit) {
                    await fetchAPI('/api/contacts/' + contact.id, {
                        method: 'PUT',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify(data)
                    });
                } else {
                    await fetchAPI('/api/contacts', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify(data)
                    });
                }
                closeModal(inst);
                loadData(inst);
            } catch (err) {
                if (inst.context && typeof inst.context.notify === 'function') {
                    inst.context.notify({ title: t(inst.context, 'desktop.notification'), message: err.message });
                }
            }
        });
    }

    function closeModal(inst) {
        const backdrop = inst.host.querySelector('.vd-people-modal-backdrop');
        if (backdrop) backdrop.classList.remove('active');
        inst.modalOpen = false;
    }

    async function deleteContact(inst, contact) {
        if (!contact || contact._kg) return;
        const name = contact.name || contact.id;
        const confirmed = await showConfirmModal(inst, t(inst.context, 'desktop.people_delete'), t(inst.context, 'desktop.people_delete_confirm', { name }));
        if (!confirmed) return;
        try {
            await fetchAPI('/api/contacts/' + contact.id, { method: 'DELETE' });
            closeDetail(inst);
            loadData(inst);
        } catch (err) {
            if (inst.context && typeof inst.context.notify === 'function') {
                inst.context.notify({ title: t(inst.context, 'desktop.notification'), message: err.message });
            }
        }
    }

    function showConfirmModal(inst, title, message) {
        return new Promise(resolve => {
            const backdrop = inst.host.querySelector('.vd-people-modal-backdrop');
            const modal = inst.host.querySelector('.vd-people-modal');
            if (!backdrop || !modal) { resolve(false); return; }
            modal.innerHTML = `
                <div class="vd-people-modal-form">
                    <div class="vd-people-modal-title">${esc(title)}</div>
                    <div style="font-size:13px;color:var(--vd-muted);margin-bottom:18px">${esc(message)}</div>
                    <div class="vd-people-modal-actions">
                        <button type="button" class="vd-people-modal-cancel">${esc(t(inst.context, 'desktop.cancel'))}</button>
                        <button type="button" class="vd-people-modal-save vd-people-modal-danger">${esc(t(inst.context, 'desktop.people_delete'))}</button>
                    </div>
                </div>`;
            backdrop.classList.add('active');
            inst.modalOpen = true;
            const cancelBtn = modal.querySelector('.vd-people-modal-cancel');
            const confirmBtn = modal.querySelector('.vd-people-modal-save');
            const finish = value => { closeModal(inst); resolve(value); };
            cancelBtn.addEventListener('click', () => finish(false));
            confirmBtn.addEventListener('click', () => finish(true));
            backdrop.addEventListener('click', function handler(event) {
                if (event.target === backdrop) { backdrop.removeEventListener('click', handler); finish(false); }
            });
        });
    }

    function fetchList(url) {
        return fetch(url, { credentials: 'same-origin', cache: 'no-store' })
            .then(r => r.ok ? r.json() : Promise.reject(new Error('HTTP ' + r.status)));
    }

    function fetchAPI(url, options) {
        return fetch(url, Object.assign({ credentials: 'same-origin', cache: 'no-store' }, options || {}))
            .then(r => r.ok ? r.json().catch(() => ({})) : Promise.reject(new Error('HTTP ' + r.status)));
    }

    function dispose(windowId) {
        const inst = instances.get(windowId);
        if (!inst) return;
        if (inst.cleanup) inst.cleanup.forEach(fn => { try { fn(); } catch (_) {} });
        instances.delete(windowId);
    }

    window.PeopleApp = { render, dispose };
})();
