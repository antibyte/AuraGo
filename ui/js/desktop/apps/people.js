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

    async function loadData(inst) {
        try {
            const [contactsResp, upcomingResp] = await Promise.all([
                fetchList('/api/contacts'),
                fetchList('/api/people/upcoming?days=60')
            ]);
            inst.contacts = contactsResp || [];
            inst.upcoming = upcomingResp || [];
            inst.filtered = inst.contacts;
            renderSidebarGroups(inst);
            renderSidebarBirthdays(inst);
            renderContent(inst);
            loadKGPersons(inst);
        } catch (err) {
            const content = inst.host.querySelector('.vd-people-content');
            if (content) content.innerHTML = `<div class="vd-people-empty"><div class="vd-people-empty-title">${esc(err.message)}</div></div>`;
        }
    }

    async function loadKGPersons(inst) {
        try {
            const data = await fetchList('/api/people/kg-persons?limit=100');
            inst.kgPersons = data || [];
        } catch (_) {}
    }

    async function searchContacts(inst) {
        if (inst.semanticMode) {
            try {
                const data = await fetchList('/api/people/lookup?q=' + encodeURIComponent(inst.searchQuery) + '&mode=fts');
                inst.filtered = (data.nodes || []).map(n => ({
                    id: n.id, name: n.label || n.id,
                    email: n.properties && n.properties.email || '',
                    phone: n.properties && n.properties.phone || '',
                    relationship: n.properties && n.properties.relationship || '',
                    birthday: n.properties && n.properties.birthday || '',
                    _kg: true
                }));
                renderContent(inst);
            } catch (_) {}
        } else {
            try {
                const data = await fetchList('/api/contacts?q=' + encodeURIComponent(inst.searchQuery));
                inst.filtered = data || [];
                renderContent(inst);
            } catch (_) {}
        }
    }

    function applyFilter(inst) {
        if (inst.sidebarFilter === 'all') {
            inst.filtered = inst.contacts;
        } else if (inst.sidebarFilter === 'kg') {
            inst.filtered = inst.kgPersons.map(n => ({
                id: n.id, name: n.label || n.id,
                email: n.properties && n.properties.email || '',
                relationship: n.properties && n.properties.relationship || '',
                birthday: n.properties && n.properties.birthday || '',
                _kg: true
            }));
        } else {
            inst.filtered = inst.contacts.filter(c => (c.relationship || '').toLowerCase() === inst.sidebarFilter);
        }
        renderContent(inst);
    }

    function renderSidebarGroups(inst) {
        const groupsEl = inst.host.querySelector('.vd-people-sidebar-groups');
        if (!groupsEl) return;
        const groups = new Set();
        inst.contacts.forEach(c => { if (c.relationship) groups.add(c.relationship); });
        if (groups.size === 0) { groupsEl.innerHTML = ''; return; }
        groupsEl.innerHTML = Array.from(groups).sort().map(g =>
            `<button class="vd-people-sidebar-item" data-filter="${esc(g.toLowerCase())}" type="button">${esc(g)}</button>`
        ).join('');
    }

    function renderSidebarBirthdays(inst) {
        const bdEl = inst.host.querySelector('.vd-people-sidebar-birthdays');
        if (!bdEl) return;
        if (!inst.upcoming || inst.upcoming.length === 0) {
            bdEl.innerHTML = `<div class="vd-people-sidebar-empty">${esc(t(inst.context, 'desktop.people_kg_no_data'))}</div>`;
            return;
        }
        bdEl.innerHTML = inst.upcoming.slice(0, 10).map(c => {
            const days = daysUntilBirthday(c.birthday);
            const dayLabel = days === 0 ? t(inst.context, 'desktop.people_today')
                : days === 1 ? t(inst.context, 'desktop.people_tomorrow')
                : t(inst.context, 'desktop.people_days_until_birthday', { days });
            return `<button class="vd-people-birthday-item" type="button" data-contact-id="${esc(c.id)}">
                <span class="vd-people-birthday-name">${esc(c.name)}</span>
                <span class="vd-people-birthday-days">${esc(dayLabel)}</span>
            </button>`;
        }).join('');
        bdEl.querySelectorAll('.vd-people-birthday-item').forEach(btn => {
            btn.addEventListener('click', () => {
                const c = inst.contacts.find(cc => cc.id === btn.dataset.contactId);
                if (c) openDetail(inst, c);
            });
        });
    }

    function renderContent(inst) {
        const content = inst.host.querySelector('.vd-people-content');
        if (!content) return;
        if (!inst.filtered || inst.filtered.length === 0) {
            content.innerHTML = `<div class="vd-people-empty">
                <div class="vd-people-empty-title">${esc(t(inst.context, 'desktop.people_empty_title'))}</div>
                <div class="vd-people-empty-hint">${esc(t(inst.context, 'desktop.people_empty_hint'))}</div>
            </div>`;
            return;
        }
        if (inst.viewMode === 'grid') {
            content.innerHTML = `<div class="vd-people-grid">${inst.filtered.map(c => renderCard(c, inst)).join('')}</div>`;
        } else {
            content.innerHTML = `<div class="vd-people-list">
                <div class="vd-people-list-header">
                    <span class="vd-people-list-col vd-people-list-name">${esc(t(inst.context, 'desktop.people_name'))}</span>
                    <span class="vd-people-list-col vd-people-list-email">${esc(t(inst.context, 'desktop.people_email'))}</span>
                    <span class="vd-people-list-col vd-people-list-phone">${esc(t(inst.context, 'desktop.people_phone'))}</span>
                    <span class="vd-people-list-col vd-people-list-birthday">${esc(t(inst.context, 'desktop.people_birthday'))}</span>
                </div>
                ${inst.filtered.map(c => renderListRow(c, inst)).join('')}
            </div>`;
        }
    }

    function renderCard(contact, inst) {
        const days = daysUntilBirthday(contact.birthday);
        const birthdayBadge = (days >= 0 && days <= 30)
            ? `<span class="vd-people-badge vd-people-badge-birthday">${days === 0 ? t(inst.context, 'desktop.people_today') : days + 'd'}</span>`
            : '';
        const relBadge = contact.relationship ? `<span class="vd-people-badge vd-people-badge-rel">${esc(contact.relationship)}</span>` : '';
        const kgBadge = contact._kg ? '<span class="vd-people-badge vd-people-badge-kg">KG</span>' : '';
        return `<div class="vd-people-card" data-contact-id="${esc(contact.id)}">
            <div class="vd-people-avatar">${esc(avatarInitials(contact.name))}</div>
            <div class="vd-people-card-info">
                <div class="vd-people-card-name">${esc(contact.name)}</div>
                <div class="vd-people-card-email">${esc(contact.email || '')}</div>
                <div class="vd-people-card-phone">${esc(contact.phone || contact.mobile || '')}</div>
            </div>
            <div class="vd-people-card-badges">${birthdayBadge}${relBadge}${kgBadge}</div>
        </div>`;
    }

    function renderListRow(contact, inst) {
        const days = daysUntilBirthday(contact.birthday);
        const birthdayCell = contact.birthday
            ? `<span>${esc(formatDate(contact.birthday))}</span>${days >= 0 && days <= 30 ? `<span class="vd-people-badge vd-people-badge-birthday">${days === 0 ? t(inst.context, 'desktop.people_today') : days + 'd'}</span>` : ''}`
            : '';
        return `<div class="vd-people-list-row" data-contact-id="${esc(contact.id)}">
            <span class="vd-people-list-col vd-people-list-name"><span class="vd-people-avatar-sm">${esc(avatarInitials(contact.name))}</span>${esc(contact.name)}</span>
            <span class="vd-people-list-col vd-people-list-email">${esc(contact.email || '')}</span>
            <span class="vd-people-list-col vd-people-list-phone">${esc(contact.phone || contact.mobile || '')}</span>
            <span class="vd-people-list-col vd-people-list-birthday">${birthdayCell}</span>
        </div>`;
    }

    async function openDetail(inst, contact) {
        inst.selectedContact = contact;
        const detail = inst.host.querySelector('.vd-people-detail');
        if (!detail) return;
        detail.hidden = false;
        inst.detailOpen = true;

        const days = daysUntilBirthday(contact.birthday);
        let birthdayInfo = '';
        if (contact.birthday) {
            birthdayInfo = formatDate(contact.birthday);
            if (days === 0) birthdayInfo += ' - ' + t(inst.context, 'desktop.people_today');
            else if (days === 1) birthdayInfo += ' - ' + t(inst.context, 'desktop.people_tomorrow');
            else if (days > 1) birthdayInfo += ' (' + days + ' days)';
        }

        let kgSection = `<div class="vd-people-detail-kg-empty">${esc(t(inst.context, 'desktop.people_kg_no_data'))}</div>`;
        if (!contact._kg) {
            try {
                const kgResp = await fetchList('/api/knowledge-graph/node?id=' + encodeURIComponent('contact_' + contact.id));
                const kgNode = kgResp && kgResp.node;
                if (kgNode && kgNode.id) {
                    const edges = kgResp.edges || [];
                    const neighbors = kgResp.neighbors || [];
                    if (edges.length > 0 || neighbors.length > 0) {
                        kgSection = '<div class="vd-people-detail-kg-list">' +
                            edges.map(e => `<div class="vd-people-detail-kg-edge"><span class="vd-people-detail-kg-rel">${esc(e.relation || '')}</span> <span class="vd-people-detail-kg-target">${esc(e.target_label || e.target || '')}</span></div>`).join('') +
                            neighbors.map(n => `<div class="vd-people-detail-kg-neighbor">${esc(n.label || n.id || '')}</div>`).join('') +
                            '</div>';
                    }
                }
            } catch (_) {}
        }

        detail.innerHTML = `
            <button class="vd-people-detail-close" type="button">&times;</button>
            <div class="vd-people-detail-header">
                <div class="vd-people-avatar-lg">${esc(avatarInitials(contact.name))}</div>
                <div class="vd-people-detail-name">${esc(contact.name)}</div>
                ${contact.relationship ? `<div class="vd-people-detail-rel">${esc(contact.relationship)}</div>` : ''}
            </div>
            <div class="vd-people-detail-fields">
                ${contact.email ? `<div class="vd-people-detail-field"><label>${esc(t(inst.context, 'desktop.people_email'))}</label><span>${esc(contact.email)}</span></div>` : ''}
                ${contact.phone ? `<div class="vd-people-detail-field"><label>${esc(t(inst.context, 'desktop.people_phone'))}</label><span>${esc(contact.phone)}</span></div>` : ''}
                ${contact.mobile ? `<div class="vd-people-detail-field"><label>${esc(t(inst.context, 'desktop.people_mobile'))}</label><span>${esc(contact.mobile)}</span></div>` : ''}
                ${contact.address ? `<div class="vd-people-detail-field"><label>${esc(t(inst.context, 'desktop.people_address'))}</label><span>${esc(contact.address)}</span></div>` : ''}
                ${birthdayInfo ? `<div class="vd-people-detail-field"><label>${esc(t(inst.context, 'desktop.people_birthday'))}</label><span>${esc(birthdayInfo)}</span></div>` : ''}
                ${contact.notes ? `<div class="vd-people-detail-field vd-people-detail-notes"><label>${esc(t(inst.context, 'desktop.people_notes'))}</label><span>${esc(contact.notes)}</span></div>` : ''}
            </div>
            <div class="vd-people-detail-kg">
                <div class="vd-people-detail-kg-title">${esc(t(inst.context, 'desktop.people_detail_kg'))}</div>
                ${kgSection}
            </div>
            <div class="vd-people-detail-actions">
                <button class="vd-people-detail-edit" type="button">${esc(t(inst.context, 'desktop.people_edit'))}</button>
                <button class="vd-people-detail-delete" type="button">${esc(t(inst.context, 'desktop.people_delete'))}</button>
            </div>`;

        detail.classList.add('open');
    }

    function closeDetail(inst) {
        const detail = inst.host.querySelector('.vd-people-detail');
        if (!detail) return;
        detail.classList.remove('open');
        setTimeout(() => { detail.hidden = true; }, 200);
        inst.detailOpen = false;
        inst.selectedContact = null;
    }

    function openModal(inst, contact) {
        const backdrop = inst.host.querySelector('.vd-people-modal-backdrop');
        const modal = inst.host.querySelector('.vd-people-modal');
        if (!backdrop || !modal) return;
        const isEdit = contact && contact.id && !contact._kg;
        const title = isEdit ? t(inst.context, 'desktop.people_edit') : t(inst.context, 'desktop.people_add');
        const c = contact || {};

        modal.innerHTML = `
            <form class="vd-people-modal-form">
                <div class="vd-people-modal-title">${esc(title)}</div>
                <div class="vd-people-modal-fields">
                    <label>${esc(t(inst.context, 'desktop.people_name'))}
                        <input name="name" type="text" value="${esc(c.name || '')}" required autocomplete="off">
                    </label>
                    <label>${esc(t(inst.context, 'desktop.people_email'))}
                        <input name="email" type="email" value="${esc(c.email || '')}" autocomplete="off">
                    </label>
                    <label>${esc(t(inst.context, 'desktop.people_phone'))}
                        <input name="phone" type="tel" value="${esc(c.phone || '')}" autocomplete="off">
                    </label>
                    <label>${esc(t(inst.context, 'desktop.people_mobile'))}
                        <input name="mobile" type="tel" value="${esc(c.mobile || '')}" autocomplete="off">
                    </label>
                    <label>${esc(t(inst.context, 'desktop.people_address'))}
                        <input name="address" type="text" value="${esc(c.address || '')}" autocomplete="off">
                    </label>
                    <label>${esc(t(inst.context, 'desktop.people_relationship'))}
                        <input name="relationship" type="text" value="${esc(c.relationship || '')}" autocomplete="off">
                    </label>
                    <label>${esc(t(inst.context, 'desktop.people_birthday'))}
                        <input name="birthday" type="date" value="${esc(c.birthday || '')}">
                    </label>
                    <label>${esc(t(inst.context, 'desktop.people_reminder'))}
                        <select name="reminder">
                            <option value="none"${c.reminder === 'none' || !c.reminder ? ' selected' : ''}>${esc(t(inst.context, 'desktop.people_reminder_none'))}</option>
                            <option value="day"${c.reminder === 'day' ? ' selected' : ''}>${esc(t(inst.context, 'desktop.people_reminder_day'))}</option>
                            <option value="week"${c.reminder === 'week' ? ' selected' : ''}>${esc(t(inst.context, 'desktop.people_reminder_week'))}</option>
                            <option value="month"${c.reminder === 'month' ? ' selected' : ''}>${esc(t(inst.context, 'desktop.people_reminder_month'))}</option>
                        </select>
                    </label>
                    <label class="vd-people-modal-notes">${esc(t(inst.context, 'desktop.people_notes'))}
                        <textarea name="notes" rows="3">${esc(c.notes || '')}</textarea>
                    </label>
                </div>
                <div class="vd-people-modal-actions">
                    <button type="button" class="vd-people-modal-cancel">${esc(t(inst.context, 'desktop.cancel'))}</button>
                    <button type="submit" class="vd-people-modal-save">${esc(t(inst.context, 'desktop.save'))}</button>
                </div>
            </form>`;

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
