(function () {
    'use strict';

    const instances = new Map();
    const DEBOUNCE_MS = 300;
    const MIN_QUERY_LENGTH = 2;
    const MAX_BIRTHDAY_ITEMS = 8;
    const REMINDER_KEYS = {
        none: 'desktop.people_reminder_none',
        day: 'desktop.people_reminder_day',
        week: 'desktop.people_reminder_week',
        month: 'desktop.people_reminder_month'
    };

    function esc(value) {
        return String(value == null ? '' : value).replace(/[&<>'"]/g, ch => ({
            '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;'
        }[ch]));
    }

    function t(context, key, vars) {
        return context && typeof context.t === 'function' ? context.t(key, vars) : key;
    }

    function localeTag() {
        return String(window.SYSTEM_LANG || document.documentElement.lang || navigator.language || 'en');
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
        const year = Number(parts[0]);
        const month = Number(parts[1]);
        const day = Number(parts[2]);
        if (!year || !month || !day) return isoDate;
        const date = new Date(year, month - 1, day);
        if (Number.isNaN(date.getTime())) return isoDate;
        try {
            return date.toLocaleDateString(localeTag(), { year: 'numeric', month: 'short', day: 'numeric' });
        } catch (_) {
            return parts[0] + '-' + parts[1] + '-' + parts[2];
        }
    }

    function sortByName(list) {
        return (list || []).slice().sort((a, b) => {
            const an = String(a.name || '').trim();
            const bn = String(b.name || '').trim();
            if (!an && !bn) return 0;
            if (!an) return 1;
            if (!bn) return -1;
            try {
                return an.localeCompare(bn, localeTag(), { sensitivity: 'base', numeric: true });
            } catch (_) {
                return an.localeCompare(bn);
            }
        });
    }

    function nodeToContact(node) {
        const props = (node && node.properties) || {};
        return {
            id: node.id,
            name: node.label || node.id,
            email: props.email || '',
            phone: props.phone || '',
            mobile: props.mobile || '',
            address: props.address || '',
            relationship: props.relationship || '',
            birthday: props.birthday || '',
            notes: props.notes || '',
            _kg: true
        };
    }

    function mailtoHref(email) {
        return 'mailto:' + encodeURIComponent(String(email || '').trim());
    }

    function telHref(number) {
        return 'tel:' + String(number || '').replace(/[^\d+*#,;]/g, '');
    }

    function focusWithoutScroll(node) {
        if (!node || typeof node.focus !== 'function') return;
        try {
            node.focus({ preventScroll: true });
        } catch (_) {
            try { node.focus(); } catch (__) { /* detached node */ }
        }
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
            base: [],
            filtered: [],
            selectedContact: null,
            viewMode: 'grid',
            sidebarFilter: 'all',
            searchQuery: '',
            semanticMode: false,
            searchTimer: null,
            searchToken: 0,
            searching: false,
            detailOpen: false,
            modalOpen: false,
            modalKeyHandler: null,
            returnFocus: null,
            host,
            cleanup: []
        };
        instances.set(windowId, inst);

        host.innerHTML = `<div class="vd-people">
            <div class="vd-people-toolbar">
                <div class="vd-people-search-wrap">
                    <span class="vd-people-search-icon" aria-hidden="true">
                        <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" focusable="false"><circle cx="7" cy="7" r="4.25"/><path d="M10.3 10.3 13.5 13.5"/></svg>
                    </span>
                    <input class="vd-people-search" type="text" placeholder="${esc(t(context, 'desktop.people_search_placeholder'))}" aria-label="${esc(t(context, 'desktop.people_search_placeholder'))}" autocomplete="off" spellcheck="false" inputmode="search" enterkeyhint="search" autocapitalize="off">
                    <button class="vd-people-search-clear" type="button" title="${esc(t(context, 'desktop.people_clear_search'))}" aria-label="${esc(t(context, 'desktop.people_clear_search'))}" hidden>&times;</button>
                    <button class="vd-people-semantic-toggle" type="button" title="${esc(t(context, 'desktop.people_semantic_hint'))}" aria-pressed="false">${esc(t(context, 'desktop.people_kg'))}</button>
                </div>
                <div class="vd-people-toolbar-actions">
                    <div class="vd-people-view-toggle">
                        <button class="vd-people-view-btn active" data-view="grid" type="button" title="${esc(t(context, 'desktop.people_grid_view'))}" aria-label="${esc(t(context, 'desktop.people_grid_view'))}" aria-pressed="true">&#9638;</button>
                        <button class="vd-people-view-btn" data-view="list" type="button" title="${esc(t(context, 'desktop.people_list_view'))}" aria-label="${esc(t(context, 'desktop.people_list_view'))}" aria-pressed="false">&#9776;</button>
                    </div>
                    <button class="vd-people-add-btn" type="button">+ ${esc(t(context, 'desktop.people_add'))}</button>
                </div>
            </div>
            <div class="vd-people-body">
                <div class="vd-people-browse">
                <aside class="vd-people-sidebar">
                    <div class="vd-people-sidebar-section">
                        <button class="vd-people-sidebar-item active" data-filter="all" type="button">
                            <span class="vd-people-sidebar-item-label">${esc(t(context, 'desktop.people_all'))}</span>
                            <span class="vd-people-count">0</span>
                        </button>
                    </div>
                    <div class="vd-people-sidebar-section vd-people-sidebar-groups"></div>
                    <div class="vd-people-sidebar-section">
                        <div class="vd-people-sidebar-label">${esc(t(context, 'desktop.people_kg_section'))}</div>
                        <button class="vd-people-sidebar-item" data-filter="kg" type="button">
                            <span class="vd-people-sidebar-item-label">${esc(t(context, 'desktop.people_kg_persons'))}</span>
                            <span class="vd-people-count">0</span>
                        </button>
                    </div>
                    <div class="vd-people-sidebar-section">
                        <div class="vd-people-sidebar-label">${esc(t(context, 'desktop.people_upcoming_birthdays'))}</div>
                        <div class="vd-people-sidebar-birthdays"></div>
                    </div>
                </aside>
                <main class="vd-people-content">
                    <div class="vd-people-loading">${esc(t(context, 'desktop.loading'))}</div>
                </main>
                </div>
                <aside class="vd-people-detail" tabindex="-1" hidden></aside>
            </div>
            <div class="vd-people-modal-backdrop">
                <div class="vd-people-modal" role="dialog" aria-modal="true"></div>
            </div>
        </div>`;

        inst.cleanup.push(window.AuraGogDesktop?.attachSources(host, 'contacts', context) || (() => {}));
        wireEvents(inst);
        loadData(inst);
    }

    function wireEvents(inst) {
        const { host, context } = inst;
        const searchInput = host.querySelector('.vd-people-search');
        const searchClear = host.querySelector('.vd-people-search-clear');
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
                updateSearchUi(inst);
                if (inst.searchQuery.length >= MIN_QUERY_LENGTH) {
                    searchContacts(inst);
                } else {
                    inst.filtered = sortByName(inst.base);
                    renderContent(inst);
                }
            }, DEBOUNCE_MS);
        });
        inst.cleanup.push(() => { if (inst.searchTimer) clearTimeout(inst.searchTimer); });

        searchInput.addEventListener('keydown', event => {
            if (event.key === 'Escape' && searchInput.value) {
                event.preventDefault();
                event.stopPropagation();
                clearSearch(inst);
            }
        });

        searchClear.addEventListener('click', () => {
            clearSearch(inst);
            focusWithoutScroll(searchInput);
        });

        semanticBtn.addEventListener('click', () => {
            inst.semanticMode = !inst.semanticMode;
            semanticBtn.classList.toggle('active', inst.semanticMode);
            semanticBtn.setAttribute('aria-pressed', inst.semanticMode ? 'true' : 'false');
            const kgLabel = t(context, 'desktop.people_kg');
            semanticBtn.textContent = inst.semanticMode ? kgLabel + ' ' + t(context, 'desktop.people_semantic_search') : kgLabel;
            if (inst.searchQuery.length >= MIN_QUERY_LENGTH) searchContacts(inst);
        });

        viewBtns.forEach(btn => {
            btn.addEventListener('click', () => {
                inst.viewMode = btn.dataset.view;
                viewBtns.forEach(b => {
                    const active = b === btn;
                    b.classList.toggle('active', active);
                    b.setAttribute('aria-pressed', active ? 'true' : 'false');
                });
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
            const action = event.target.closest('.vd-people-empty-action');
            if (action) {
                if (action.dataset.action === 'retry') { loadData(inst); return; }
                if (action.dataset.action === 'clear-search') { clearSearch(inst); focusWithoutScroll(searchInput); return; }
                if (action.dataset.action === 'add') { openModal(inst, null); return; }
            }
            const card = event.target.closest('.vd-people-card, .vd-people-list-row');
            if (!card) return;
            const id = card.dataset.contactId;
            const contact = inst.contacts.find(c => c.id === id) || inst.filtered.find(c => c.id === id);
            if (contact) openDetail(inst, contact);
        });

        content.addEventListener('keydown', event => {
            if (event.key !== 'Enter' && event.key !== ' ') return;
            const card = event.target.closest('.vd-people-card, .vd-people-list-row');
            if (!card) return;
            event.preventDefault();
            const id = card.dataset.contactId;
            const contact = inst.contacts.find(c => c.id === id) || inst.filtered.find(c => c.id === id);
            if (contact) openDetail(inst, contact);
        });

        detail.addEventListener('click', event => {
            if (event.target.closest('.vd-people-detail-close')) {
                closeDetail(inst);
                return;
            }
            if (event.target.closest('.vd-people-detail-edit')) {
                if (inst.selectedContact && !inst.selectedContact._kg) openModal(inst, inst.selectedContact);
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

        const onWindowKey = event => {
            if (event.key !== 'Escape') return;
            if (inst.modalOpen) return;
            if (inst.detailOpen) closeDetail(inst);
        };
        document.addEventListener('keydown', onWindowKey);
        inst.cleanup.push(() => document.removeEventListener('keydown', onWindowKey));
    }

    function updateSearchUi(inst) {
        const clear = inst.host.querySelector('.vd-people-search-clear');
        if (clear) clear.hidden = !inst.searchQuery;
    }

    function clearSearch(inst) {
        if (inst.searchTimer) clearTimeout(inst.searchTimer);
        const searchInput = inst.host.querySelector('.vd-people-search');
        if (searchInput) searchInput.value = '';
        inst.searchQuery = '';
        inst.searchToken++;
        updateSearchUi(inst);
        inst.filtered = sortByName(inst.base);
        renderContent(inst);
    }

    async function loadData(inst) {
        const content = inst.host.querySelector('.vd-people-content');
        if (content) content.innerHTML = `<div class="vd-people-loading">${esc(t(inst.context, 'desktop.loading'))}</div>`;
        try {
            const [contactsResp, upcomingResp] = await Promise.all([
                fetchList('/api/desktop/integrations/contacts'),
                fetchList('/api/desktop/integrations/people/upcoming?days=60')
            ]);
            inst.contacts = contactsResp || [];
            inst.upcoming = upcomingResp || [];
            renderSidebarGroups(inst);
            renderSidebarCounts(inst);
            renderSidebarBirthdays(inst);
            applyFilter(inst);
            loadKGPersons(inst);
        } catch (err) {
            const target = inst.host.querySelector('.vd-people-content');
            if (target) {
                target.innerHTML = `<div class="vd-people-empty vd-people-empty-error">
                    <div class="vd-people-empty-title">${esc(t(inst.context, 'desktop.load_failed'))}</div>
                    <button class="vd-people-empty-action" type="button" data-action="retry">${esc(t(inst.context, 'desktop.retry'))}</button>
                </div>`;
            }
        }
    }

    async function loadKGPersons(inst) {
        try {
            const data = await fetchList('/api/desktop/integrations/people/kg-persons?limit=100');
            inst.kgPersons = data || [];
            renderSidebarCounts(inst);
            if (inst.sidebarFilter === 'kg') applyFilter(inst);
        } catch (_) {}
    }

    function contactsForFilter(inst) {
        if (inst.sidebarFilter === 'kg') return sortByName(inst.kgPersons.map(nodeToContact));
        if (inst.sidebarFilter === 'all') return sortByName(inst.contacts);
        return sortByName(inst.contacts.filter(c => (c.relationship || '').toLowerCase() === inst.sidebarFilter));
    }

    function applyFilter(inst) {
        inst.base = contactsForFilter(inst);
        if (inst.searchQuery.length >= MIN_QUERY_LENGTH) {
            searchContacts(inst);
            return;
        }
        inst.filtered = inst.base;
        renderContent(inst);
    }

    async function searchContacts(inst) {
        const token = ++inst.searchToken;
        const query = inst.searchQuery;
        inst.searching = true;
        setContentBusy(inst, true);
        try {
            let results = [];
            if (inst.semanticMode) {
                const data = await fetchList('/api/desktop/integrations/people/lookup?q=' + encodeURIComponent(query) + '&mode=fts');
                results = (data && data.nodes ? data.nodes : []).map(nodeToContact);
            } else if (inst.sidebarFilter === 'kg') {
                results = filterLocalKg(inst, query);
            } else {
                const data = await fetchList('/api/desktop/integrations/contacts?q=' + encodeURIComponent(query));
                results = Array.isArray(data) ? data : [];
            }
            if (token !== inst.searchToken) return;
            inst.filtered = sortByName(filterByGroup(inst, results));
        } catch (_) {
            if (token !== inst.searchToken) return;
            inst.filtered = [];
        } finally {
            if (token === inst.searchToken) {
                inst.searching = false;
                setContentBusy(inst, false);
                renderContent(inst);
            }
        }
    }

    function filterLocalKg(inst, query) {
        const needle = String(query || '').toLowerCase();
        return inst.kgPersons
            .map(nodeToContact)
            .filter(contact => [contact.name, contact.email, contact.phone, contact.relationship]
                .some(field => String(field || '').toLowerCase().includes(needle)));
    }

    function filterByGroup(inst, list) {
        if (inst.sidebarFilter === 'all' || inst.sidebarFilter === 'kg') return list || [];
        return (list || []).filter(c => (c.relationship || '').toLowerCase() === inst.sidebarFilter);
    }

    function setContentBusy(inst, busy) {
        const content = inst.host.querySelector('.vd-people-content');
        if (!content) return;
        content.setAttribute('aria-busy', busy ? 'true' : 'false');
    }

    function renderSidebarGroups(inst) {
        const groupsEl = inst.host.querySelector('.vd-people-sidebar-groups');
        if (!groupsEl) return;
        const groups = new Map();
        inst.contacts.forEach(c => {
            const label = String(c.relationship || '').trim();
            if (!label) return;
            const id = label.toLowerCase();
            const existing = groups.get(id);
            if (existing) existing.count++;
            else groups.set(id, { label, count: 1 });
        });
        if (groups.size === 0) { groupsEl.innerHTML = ''; return; }
        groupsEl.innerHTML = Array.from(groups.entries())
            .sort((a, b) => a[1].label.localeCompare(b[1].label, localeTag(), { sensitivity: 'base' }))
            .map(([id, group]) => `<button class="vd-people-sidebar-item" data-filter="${esc(id)}" type="button">
                <span class="vd-people-sidebar-item-label">${esc(group.label)}</span>
                <span class="vd-people-count">${group.count}</span>
            </button>`).join('');
    }

    function renderSidebarCounts(inst) {
        const sidebar = inst.host.querySelector('.vd-people-sidebar');
        if (!sidebar) return;
        const all = sidebar.querySelector('.vd-people-sidebar-item[data-filter="all"] .vd-people-count');
        if (all) all.textContent = String(inst.contacts.length);
        const kg = sidebar.querySelector('.vd-people-sidebar-item[data-filter="kg"] .vd-people-count');
        if (kg) kg.textContent = String(inst.kgPersons.length);
    }

    function renderSidebarBirthdays(inst) {
        const bdEl = inst.host.querySelector('.vd-people-sidebar-birthdays');
        if (!bdEl) return;
        if (!inst.upcoming || inst.upcoming.length === 0) {
            bdEl.innerHTML = `<div class="vd-people-sidebar-empty">${esc(t(inst.context, 'desktop.people_no_birthdays'))}</div>`;
            return;
        }
        bdEl.innerHTML = inst.upcoming.slice(0, MAX_BIRTHDAY_ITEMS).map(c => {
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
            content.innerHTML = emptyState(inst);
            updateSelection(inst);
            return;
        }
        const status = statusLine(inst);
        if (inst.viewMode === 'grid') {
            content.innerHTML = `${status}<div class="vd-people-grid">${inst.filtered.map(c => renderCard(c, inst)).join('')}</div>`;
        } else {
            content.innerHTML = `${status}<div class="vd-people-list">
                <div class="vd-people-list-header">
                    <span class="vd-people-list-col vd-people-list-name">${esc(t(inst.context, 'desktop.people_name'))}</span>
                    <span class="vd-people-list-col vd-people-list-email">${esc(t(inst.context, 'desktop.people_email'))}</span>
                    <span class="vd-people-list-col vd-people-list-phone">${esc(t(inst.context, 'desktop.people_phone'))}</span>
                    <span class="vd-people-list-col vd-people-list-birthday">${esc(t(inst.context, 'desktop.people_birthday'))}</span>
                </div>
                ${inst.filtered.map(c => renderListRow(c, inst)).join('')}
            </div>`;
        }
        updateSelection(inst);
    }

    function statusLine(inst) {
        const shown = inst.filtered.length;
        const total = inst.base.length;
        let text;
        if (shown === total) {
            text = shown === 1
                ? t(inst.context, 'desktop.people_results_count_one')
                : t(inst.context, 'desktop.people_results_count', { count: shown });
        } else {
            text = t(inst.context, 'desktop.people_results_filtered', { shown, total });
        }
        return `<div class="vd-people-status" role="status">${esc(text)}</div>`;
    }

    function emptyState(inst) {
        const query = String(inst.searchQuery || '').trim();
        if (query) {
            return `<div class="vd-people-empty vd-people-empty-search">
                <div class="vd-people-empty-icon" aria-hidden="true">
                    <svg viewBox="0 0 24 24" width="28" height="28" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" focusable="false"><circle cx="10.5" cy="10.5" r="6.25"/><path d="m15.2 15.2 4.3 4.3"/></svg>
                </div>
                <div class="vd-people-empty-title">${esc(t(inst.context, 'desktop.people_no_results_title'))}</div>
                <div class="vd-people-empty-hint">${esc(t(inst.context, 'desktop.people_no_results_hint', { query }))}</div>
                <button class="vd-people-empty-action" type="button" data-action="clear-search">${esc(t(inst.context, 'desktop.people_clear_search'))}</button>
            </div>`;
        }
        return `<div class="vd-people-empty">
            <div class="vd-people-empty-icon" aria-hidden="true">
                <svg viewBox="0 0 24 24" width="28" height="28" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" focusable="false"><circle cx="9" cy="8" r="3.5"/><path d="M3.5 19.5c0-3 2.5-5 5.5-5s5.5 2 5.5 5"/><path d="M16 7.5a3 3 0 0 1 0 5.8"/><path d="M18 19.5c0-2-.6-3.6-1.6-4.6"/></svg>
            </div>
            <div class="vd-people-empty-title">${esc(t(inst.context, 'desktop.people_empty_title'))}</div>
            <div class="vd-people-empty-hint">${esc(t(inst.context, 'desktop.people_empty_hint'))}</div>
            <button class="vd-people-empty-action" type="button" data-action="add">${esc(t(inst.context, 'desktop.people_add'))}</button>
        </div>`;
    }

    function renderCard(contact, inst) {
        const days = daysUntilBirthday(contact.birthday);
        const birthdayBadge = (days >= 0 && days <= 30)
            ? `<span class="vd-people-badge vd-people-badge-birthday">${days === 0 ? t(inst.context, 'desktop.people_today') : t(inst.context, 'desktop.people_days_until_birthday', { days })}</span>`
            : '';
        const relBadge = contact.relationship ? `<span class="vd-people-badge vd-people-badge-rel">${esc(contact.relationship)}</span>` : '';
        const kgBadge = contact._kg ? '<span class="vd-people-badge vd-people-badge-kg">' + esc(t(inst.context, 'desktop.people_kg')) + '</span>' : '';
        const phone = contact.phone || contact.mobile || '';
        const selected = inst.selectedContact && inst.selectedContact.id === contact.id;
        return `<div class="vd-people-card${selected ? ' is-selected' : ''}" data-contact-id="${esc(contact.id)}" role="button" tabindex="0"${selected ? ' aria-current="true"' : ''}>
            <div class="vd-people-avatar" aria-hidden="true">${esc(avatarInitials(contact.name))}</div>
            <div class="vd-people-card-info">
                <div class="vd-people-card-name" title="${esc(contact.name)}">${esc(contact.name)}</div>
                ${contact.email ? `<div class="vd-people-card-email" title="${esc(contact.email)}">${esc(contact.email)}</div>` : ''}
                ${phone ? `<div class="vd-people-card-phone" title="${esc(phone)}">${esc(phone)}</div>` : ''}
            </div>
            <div class="vd-people-card-badges">${birthdayBadge}${relBadge}${kgBadge}</div>
        </div>`;
    }

    function renderListRow(contact, inst) {
        const days = daysUntilBirthday(contact.birthday);
        const birthdayCell = contact.birthday
            ? `<span>${esc(formatDate(contact.birthday))}</span>${days >= 0 && days <= 30 ? `<span class="vd-people-badge vd-people-badge-birthday">${days === 0 ? t(inst.context, 'desktop.people_today') : t(inst.context, 'desktop.people_days_until_birthday', { days })}</span>` : ''}`
            : '';
        const phone = contact.phone || contact.mobile || '';
        const selected = inst.selectedContact && inst.selectedContact.id === contact.id;
        return `<div class="vd-people-list-row${selected ? ' is-selected' : ''}" data-contact-id="${esc(contact.id)}" role="button" tabindex="0"${selected ? ' aria-current="true"' : ''}>
            <span class="vd-people-list-col vd-people-list-name"><span class="vd-people-avatar-sm" aria-hidden="true">${esc(avatarInitials(contact.name))}</span>${esc(contact.name)}</span>
            <span class="vd-people-list-col vd-people-list-email">${contact.email ? esc(contact.email) : ''}</span>
            <span class="vd-people-list-col vd-people-list-phone">${phone ? esc(phone) : ''}</span>
            <span class="vd-people-list-col vd-people-list-birthday">${birthdayCell}</span>
        </div>`;
    }

    function detailField(label, value, title) {
        if (!value) return '';
        return `<div class="vd-people-detail-field"><label>${esc(label)}</label><span${title ? ` title="${esc(title)}"` : ''}>${esc(value)}</span></div>`;
    }

    function detailLink(label, value, href) {
        if (!value) return '';
        return `<div class="vd-people-detail-field"><label>${esc(label)}</label><a class="vd-people-detail-link" href="${esc(href)}">${esc(value)}</a></div>`;
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
            else if (days > 1) birthdayInfo += ' (' + t(inst.context, 'desktop.people_days_until_birthday', { days }) + ')';
        }

        let kgSection = `<div class="vd-people-detail-kg-empty">${esc(t(inst.context, 'desktop.people_kg_no_data'))}</div>`;
        if (!contact._kg) {
            try {
                const kgResp = await fetchList('/api/desktop/integrations/knowledge-graph/node?id=' + encodeURIComponent('contact_' + contact.id));
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

        const reminder = t(inst.context, REMINDER_KEYS[contact.reminder] || REMINDER_KEYS.none);
        const actions = contact._kg
            ? `<div class="vd-people-detail-note">${esc(t(inst.context, 'desktop.people_kg_readonly'))}</div>`
            : `<div class="vd-people-detail-actions">
                <button class="vd-people-detail-edit" type="button">${esc(t(inst.context, 'desktop.people_edit'))}</button>
                <button class="vd-people-detail-delete" type="button">${esc(t(inst.context, 'desktop.people_delete'))}</button>
            </div>`;

        detail.innerHTML = `
            <button class="vd-people-detail-close" type="button" title="${esc(t(inst.context, 'desktop.close'))}" aria-label="${esc(t(inst.context, 'desktop.close'))}">&times;</button>
            <div class="vd-people-detail-header">
                <div class="vd-people-avatar-lg" aria-hidden="true">${esc(avatarInitials(contact.name))}</div>
                <div class="vd-people-detail-name">${esc(contact.name)}</div>
                ${contact.relationship ? `<div class="vd-people-detail-rel">${esc(contact.relationship)}</div>` : ''}
            </div>
            <div class="vd-people-detail-fields">
                ${detailLink(t(inst.context, 'desktop.people_email'), contact.email, mailtoHref(contact.email))}
                ${detailLink(t(inst.context, 'desktop.people_phone'), contact.phone, telHref(contact.phone))}
                ${detailLink(t(inst.context, 'desktop.people_mobile'), contact.mobile, telHref(contact.mobile))}
                ${detailField(t(inst.context, 'desktop.people_address'), contact.address)}
                ${detailField(t(inst.context, 'desktop.people_birthday'), birthdayInfo)}
                ${detailField(t(inst.context, 'desktop.people_reminder'), reminder)}
                ${contact.notes ? `<div class="vd-people-detail-field vd-people-detail-notes"><label>${esc(t(inst.context, 'desktop.people_notes'))}</label><span>${esc(contact.notes)}</span></div>` : ''}
            </div>
            <div class="vd-people-detail-kg">
                <div class="vd-people-detail-kg-title">${esc(t(inst.context, 'desktop.people_detail_kg'))}</div>
                ${kgSection}
            </div>
            ${actions}`;

        detail.classList.add('open');
        if (contact.name) detail.setAttribute('aria-label', contact.name);
        updateSelection(inst);
        focusWithoutScroll(detail);
    }

    function updateSelection(inst) {
        const selectedId = inst.selectedContact ? inst.selectedContact.id : '';
        inst.host.querySelectorAll('.vd-people-card, .vd-people-list-row').forEach(node => {
            const active = !!selectedId && node.dataset.contactId === selectedId;
            node.classList.toggle('is-selected', active);
            if (active) node.setAttribute('aria-current', 'true');
            else node.removeAttribute('aria-current');
        });
    }

    function closeDetail(inst) {
        const detail = inst.host.querySelector('.vd-people-detail');
        if (!detail) return;
        const trigger = inst.host.querySelector('.vd-people-card.is-selected, .vd-people-list-row.is-selected');
        detail.classList.remove('open');
        detail.removeAttribute('aria-label');
        setTimeout(() => { if (!detail.classList.contains('open')) detail.hidden = true; }, 200);
        inst.detailOpen = false;
        inst.selectedContact = null;
        updateSelection(inst);
        if (trigger && trigger.isConnected) focusWithoutScroll(trigger);
    }

    function openModal(inst, contact) {
        if (contact && contact._kg) return;
        const backdrop = inst.host.querySelector('.vd-people-modal-backdrop');
        const modal = inst.host.querySelector('.vd-people-modal');
        if (!backdrop || !modal) return;
        const isEdit = contact && contact.id && !contact._kg;
        const title = isEdit ? t(inst.context, 'desktop.people_edit') : t(inst.context, 'desktop.people_add');
        const c = contact || {};

        modal.innerHTML = `
            <form class="vd-people-modal-form" novalidate>
                <div class="vd-people-modal-title">${esc(title)}</div>
                <div class="vd-people-modal-fields">
                    <label>${esc(t(inst.context, 'desktop.people_name'))}
                        <input name="name" type="text" value="${esc(c.name || '')}" required autocomplete="off" autofocus>
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
        inst.returnFocus = document.activeElement;

        const form = modal.querySelector('form');
        const cancelBtn = modal.querySelector('.vd-people-modal-cancel');
        const saveBtn = modal.querySelector('.vd-people-modal-save');
        const nameInput = form.querySelector('[name="name"]');

        cancelBtn.addEventListener('click', () => closeModal(inst));
        form.addEventListener('submit', async (event) => {
            event.preventDefault();
            const name = String(nameInput.value || '').trim();
            if (!name) {
                nameInput.setAttribute('aria-invalid', 'true');
                nameInput.focus();
                return;
            }
            nameInput.removeAttribute('aria-invalid');
            const fd = new FormData(form);
            const data = {};
            fd.forEach((v, k) => { data[k] = String(v).trim(); });
            saveBtn.disabled = true;
            saveBtn.setAttribute('aria-busy', 'true');
            try {
                if (isEdit) {
                    await fetchAPI('/api/desktop/integrations/contacts/' + encodeURIComponent(contact.id), {
                        method: 'PUT',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify(data)
                    });
                } else {
                    await fetchAPI('/api/desktop/integrations/contacts', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify(data)
                    });
                }
                closeModal(inst);
                loadData(inst);
            } catch (err) {
                if (inst.context && typeof inst.context.notify === 'function') {
                    inst.context.notify({ title: t(inst.context, 'desktop.notification'), message: t(inst.context, 'desktop.request_failed') });
                }
                saveBtn.disabled = false;
                saveBtn.removeAttribute('aria-busy');
            }
        });

        installModalKeyHandler(inst, form, () => nameInput);
        if (nameInput) focusWithoutScroll(nameInput);
    }

    function installModalKeyHandler(inst, form, primary) {
        if (inst.modalKeyHandler) document.removeEventListener('keydown', inst.modalKeyHandler, true);
        const onKey = event => {
            if (event.key === 'Escape') {
                event.preventDefault();
                event.stopPropagation();
                closeModal(inst);
                return;
            }
            if (event.key !== 'Tab') return;
            const controls = Array.from(form.querySelectorAll('input,select,textarea,button:not([disabled])'))
                .filter(node => node.offsetParent !== null);
            if (!controls.length) return;
            const first = controls[0];
            const last = controls[controls.length - 1];
            const active = document.activeElement;
            if (event.shiftKey && (active === first || !controls.includes(active))) {
                event.preventDefault();
                last.focus();
            } else if (!event.shiftKey && active === last) {
                event.preventDefault();
                first.focus();
            }
        };
        inst.modalKeyHandler = onKey;
        document.addEventListener('keydown', onKey, true);
    }

    function closeModal(inst) {
        const backdrop = inst.host.querySelector('.vd-people-modal-backdrop');
        const modal = inst.host.querySelector('.vd-people-modal');
        if (backdrop) backdrop.classList.remove('active');
        if (modal) modal.innerHTML = '';
        if (inst.modalKeyHandler) {
            document.removeEventListener('keydown', inst.modalKeyHandler, true);
            inst.modalKeyHandler = null;
        }
        inst.modalOpen = false;
        const back = inst.returnFocus;
        inst.returnFocus = null;
        if (back && back.isConnected) focusWithoutScroll(back);
    }

    async function deleteContact(inst, contact) {
        if (!contact || contact._kg || inst.deleteBusy) return;
        const name = contact.name || contact.id;
        const confirmed = await showConfirmModal(inst, t(inst.context, 'desktop.people_delete'), t(inst.context, 'desktop.people_delete_confirm', { name }));
        if (confirmed === null) return;
        inst.deleteBusy = true;
        try {
            const result = await fetchAPI('/api/desktop/integrations/contacts/' + encodeURIComponent(contact.id) + '?delete_memories=' + confirmed, { method: 'DELETE' });
            if (inst.context && typeof inst.context.notify === 'function') {
                const message = t(inst.context, result.status === 'partial' ? 'desktop.people_delete_partial' : 'desktop.people_delete_report', { deleted: result.deleted_graph_items || 0, retained: result.retained_graph_items || 0 });
                inst.context.notify({ title: t(inst.context, 'desktop.people_delete'), message });
            }
            closeDetail(inst);
            loadData(inst);
        } catch (err) {
            if (inst.context && typeof inst.context.notify === 'function') {
                inst.context.notify({ title: t(inst.context, 'desktop.notification'), message: t(inst.context, 'desktop.request_failed') });
            }
        } finally { inst.deleteBusy = false; }
    }

    function showConfirmModal(inst, title, message) {
        return new Promise(resolve => {
            const backdrop = inst.host.querySelector('.vd-people-modal-backdrop');
            const modal = inst.host.querySelector('.vd-people-modal');
            if (!backdrop || !modal) { resolve(null); return; }
            modal.innerHTML = `
                <div class="vd-people-modal-form">
                    <div class="vd-people-modal-title">${esc(title)}</div>
                    <div class="vd-people-modal-copy">${esc(message)}</div>
                    <label class="vd-people-modal-check"><input type="checkbox" class="vd-people-delete-memories"> ${esc(t(inst.context, 'desktop.people_delete_memories'))}</label>
                    <p class="vd-people-modal-note">${esc(t(inst.context, 'desktop.people_delete_backups'))}</p>
                    <div class="vd-people-modal-actions">
                        <button type="button" class="vd-people-modal-cancel">${esc(t(inst.context, 'desktop.cancel'))}</button>
                        <button type="button" class="vd-people-modal-save vd-people-modal-danger">${esc(t(inst.context, 'desktop.people_delete'))}</button>
                    </div>
                </div>`;
            backdrop.classList.add('active');
            inst.modalOpen = true;
            inst.returnFocus = document.activeElement;
            const cancelBtn = modal.querySelector('.vd-people-modal-cancel');
            const confirmBtn = modal.querySelector('.vd-people-modal-save');
            const finish = value => {
                closeModal(inst);
                backdrop.removeEventListener('click', onBackdrop);
                document.removeEventListener('keydown', onKey);
                resolve(value);
            };
            const onKey = event => { if (event.key === 'Escape') { event.preventDefault(); finish(null); } };
            const onBackdrop = event => { if (event.target === backdrop) finish(null); };
            document.addEventListener('keydown', onKey);
            backdrop.addEventListener('click', onBackdrop);
            cancelBtn.addEventListener('click', () => finish(null));
            confirmBtn.addEventListener('click', () => finish(modal.querySelector('.vd-people-delete-memories').checked));
            focusWithoutScroll(modal.querySelector('.vd-people-delete-memories'));
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
        if (inst.modalKeyHandler) document.removeEventListener('keydown', inst.modalKeyHandler, true);
        if (inst.cleanup) inst.cleanup.forEach(fn => { try { fn(); } catch (_) {} });
        instances.delete(windowId);
    }

    window.PeopleApp = { render, dispose };
})();
