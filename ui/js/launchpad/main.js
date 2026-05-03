/* AuraGo – Launchpad page JS */
/* global I18N, t, applyI18n, showModal, esc */
'use strict';

let allLinks = [];
let currentCategory = '';
let currentSearch = '';
let selectedIconURL = null;
let iconSearchDebounce = null;

// ── Initialization ──────────────────────────────────────────────────────────

document.addEventListener('DOMContentLoaded', () => {
    loadLinks();
    loadCategories();
    setupEventListeners();
});

function setupEventListeners() {
    document.getElementById('lp-add-btn').addEventListener('click', () => openLinkModal());
    document.getElementById('lp-modal-cancel').addEventListener('click', closeLinkModal);
    document.getElementById('lp-modal-overlay').addEventListener('click', (e) => {
        if (e.target.id === 'lp-modal-overlay') closeLinkModal();
    });
    document.getElementById('lp-form').addEventListener('submit', handleFormSubmit);
    document.getElementById('lp-search').addEventListener('input', (e) => {
        currentSearch = e.target.value.toLowerCase().trim();
        renderGrid();
    });
    document.getElementById('lp-category-filter').addEventListener('change', (e) => {
        currentCategory = e.target.value;
        loadLinks();
    });

    // Icon tabs
    document.querySelectorAll('.lp-icon-tab').forEach(tab => {
        tab.addEventListener('click', () => {
            document.querySelectorAll('.lp-icon-tab').forEach(t => t.classList.remove('active'));
            document.querySelectorAll('.lp-icon-panel').forEach(p => p.classList.remove('active'));
            tab.classList.add('active');
            const panelId = 'lp-icon-panel-' + tab.dataset.tab;
            document.getElementById(panelId).classList.add('active');
        });
    });

    // Icon search
    document.getElementById('lp-icon-search').addEventListener('input', (e) => {
        clearTimeout(iconSearchDebounce);
        iconSearchDebounce = setTimeout(() => searchIcons(e.target.value), 400);
    });
    document.getElementById('lp-icon-search-btn').addEventListener('click', () => {
        searchIcons(document.getElementById('lp-icon-search').value);
    });

    // Custom URL preview
    document.getElementById('lp-icon-url').addEventListener('input', (e) => {
        const url = e.target.value.trim();
        const preview = document.getElementById('lp-icon-preview');
        if (url) {
            preview.innerHTML = `<img src="${esc(url)}" alt="preview" onerror="this.style.display='none'">`;
        } else {
            preview.innerHTML = '';
        }
    });
}

// ── Data fetching ───────────────────────────────────────────────────────────

async function loadLinks() {
    try {
        const url = currentCategory ? `/api/launchpad/links?category=${encodeURIComponent(currentCategory)}` : '/api/launchpad/links';
        const resp = await fetch(url);
        if (resp.status === 503) {
            showEmptyState();
            return;
        }
        allLinks = await resp.json();
        renderGrid();
    } catch (e) {
        console.error('Failed to load links:', e);
    }
}

async function loadCategories() {
    try {
        const resp = await fetch('/api/launchpad/categories');
        if (!resp.ok) return;
        const cats = await resp.json();
        const datalist = document.getElementById('lp-category-list');
        const select = document.getElementById('lp-category-filter');
        datalist.innerHTML = '';
        // Keep first option (All categories)
        const firstOpt = select.options[0];
        select.innerHTML = '';
        select.appendChild(firstOpt);
        cats.forEach(cat => {
            const opt = document.createElement('option');
            opt.value = cat;
            opt.textContent = cat;
            select.appendChild(opt);

            const dlOpt = document.createElement('option');
            dlOpt.value = cat;
            datalist.appendChild(dlOpt);
        });
        select.value = currentCategory;
    } catch (e) {
        console.error('Failed to load categories:', e);
    }
}

// ── Rendering ───────────────────────────────────────────────────────────────

function renderGrid() {
    const grid = document.getElementById('lp-grid');
    const empty = document.getElementById('lp-empty');

    let links = allLinks;
    if (currentSearch) {
        links = links.filter(l =>
            (l.title || '').toLowerCase().includes(currentSearch) ||
            (l.description || '').toLowerCase().includes(currentSearch) ||
            (l.url || '').toLowerCase().includes(currentSearch) ||
            (l.category || '').toLowerCase().includes(currentSearch)
        );
    }

    if (links.length === 0) {
        grid.style.display = 'none';
        empty.style.display = '';
        return;
    }

    grid.style.display = 'grid';
    empty.style.display = 'none';

    grid.innerHTML = links.map(link => {
        const iconHtml = link.icon_path
            ? `<img class="lp-tile-icon" src="/files/${esc(link.icon_path)}" alt="" loading="lazy" onerror="this.style.display='none';this.nextElementSibling.style.display='flex'">`
            : '';
        const fallbackHtml = `<div class="lp-tile-icon-fallback" style="display:${link.icon_path ? 'none' : 'flex'}">🌐</div>`;
        const descHtml = link.description ? `<div class="lp-tile-desc">${esc(link.description)}</div>` : '';

        return `
            <div class="lp-tile" data-id="${esc(link.id)}" onclick="openLink('${esc(link.url)}')">
                ${iconHtml}${fallbackHtml}
                <div class="lp-tile-title">${esc(link.title)}</div>
                ${descHtml}
                <div class="lp-tile-actions" onclick="event.stopPropagation()">
                    <button type="button" class="lp-tile-btn" title="${t('launchpad.edit') || 'Edit'}" onclick="editLink('${esc(link.id)}')">✏️</button>
                    <button type="button" class="lp-tile-btn" title="${t('launchpad.delete') || 'Delete'}" onclick="deleteLink('${esc(link.id)}')">🗑️</button>
                </div>
            </div>
        `;
    }).join('');
}

function showEmptyState() {
    document.getElementById('lp-grid').style.display = 'none';
    document.getElementById('lp-empty').style.display = '';
}

function openLink(url) {
    if (url) window.open(url, '_blank', 'noopener,noreferrer');
}

// ── Modal ───────────────────────────────────────────────────────────────────

function openLinkModal(link) {
    const overlay = document.getElementById('lp-modal-overlay');
    const title = document.getElementById('lp-modal-title');
    const form = document.getElementById('lp-form');

    form.reset();
    document.getElementById('lp-icon-results').innerHTML = '';
    document.getElementById('lp-icon-preview').innerHTML = '';
    document.querySelectorAll('.lp-icon-result').forEach(r => r.classList.remove('selected'));
    selectedIconURL = null;
    document.getElementById('lp-icon-path').value = '';

    if (link) {
        title.textContent = t('launchpad.modal_edit_title') || 'Edit Link';
        document.getElementById('lp-link-id').value = link.id;
        document.getElementById('lp-title').value = link.title || '';
        document.getElementById('lp-url').value = link.url || '';
        document.getElementById('lp-category').value = link.category || '';
        document.getElementById('lp-description').value = link.description || '';
        document.getElementById('lp-icon-path').value = link.icon_path || '';
        if (link.icon_path) {
            document.getElementById('lp-icon-preview').innerHTML = `<img src="/files/${esc(link.icon_path)}" alt="preview">`;
        }
    } else {
        title.textContent = t('launchpad.modal_add_title') || 'Add Link';
        document.getElementById('lp-link-id').value = '';
    }

    overlay.style.display = 'flex';
    if (overlay.classList) overlay.classList.add('active');
}

function closeLinkModal() {
    const overlay = document.getElementById('lp-modal-overlay');
    overlay.style.display = 'none';
    if (overlay.classList) overlay.classList.remove('active');
}

async function handleFormSubmit(e) {
    e.preventDefault();
    const id = document.getElementById('lp-link-id').value;
    const title = document.getElementById('lp-title').value.trim();
    const url = document.getElementById('lp-url').value.trim();
    const category = document.getElementById('lp-category').value.trim();
    const description = document.getElementById('lp-description').value.trim();
    let iconPath = document.getElementById('lp-icon-path').value;

    // Download icon if a search result was selected
    const activeTab = document.querySelector('.lp-icon-tab.active');
    if (activeTab && activeTab.dataset.tab === 'search' && selectedIconURL) {
        try {
            const dlResp = await fetch('/api/launchpad/icons/download', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ image_url: selectedIconURL, link_id: id || 'new' })
            });
            if (dlResp.ok) {
                const dlData = await dlResp.json();
                iconPath = dlData.local_path;
            }
        } catch (err) {
            console.error('Icon download failed:', err);
        }
    } else if (activeTab && activeTab.dataset.tab === 'url') {
        const customUrl = document.getElementById('lp-icon-url').value.trim();
        if (customUrl) {
            try {
                const dlResp = await fetch('/api/launchpad/icons/download', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ image_url: customUrl, link_id: id || 'new' })
                });
                if (dlResp.ok) {
                    const dlData = await dlResp.json();
                    iconPath = dlData.local_path;
                }
            } catch (err) {
                console.error('Icon download failed:', err);
            }
        }
    }

    const payload = { title, url, category, description, icon_path: iconPath };

    try {
        let resp;
        if (id) {
            resp = await fetch(`/api/launchpad/links/${encodeURIComponent(id)}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload)
            });
        } else {
            resp = await fetch('/api/launchpad/links', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload)
            });
        }

        if (!resp.ok) {
            const data = await resp.json().catch(() => ({}));
            showModal(t('common.error') || 'Error', data.error || t('launchpad.save_failed') || 'Failed to save link.', false);
            return;
        }

        closeLinkModal();
        await loadLinks();
        await loadCategories();
    } catch (err) {
        console.error('Save link failed:', err);
        showModal(t('common.error') || 'Error', t('launchpad.save_failed') || 'Failed to save link.', false);
    }
}

// ── Edit / Delete ───────────────────────────────────────────────────────────

async function editLink(id) {
    try {
        const resp = await fetch(`/api/launchpad/links/${encodeURIComponent(id)}`);
        if (!resp.ok) return;
        const link = await resp.json();
        openLinkModal(link);
    } catch (e) {
        console.error('Failed to load link for edit:', e);
    }
}

async function deleteLink(id) {
    const confirmed = await showModal(
        t('launchpad.delete_title') || 'Delete Link',
        t('launchpad.delete_confirm') || 'Are you sure you want to delete this link?',
        true
    );
    if (!confirmed) return;

    try {
        const resp = await fetch(`/api/launchpad/links/${encodeURIComponent(id)}`, { method: 'DELETE' });
        if (!resp.ok) return;
        await loadLinks();
        await loadCategories();
    } catch (e) {
        console.error('Failed to delete link:', e);
    }
}

// ── Icon search ─────────────────────────────────────────────────────────────

async function searchIcons(query) {
    const resultsEl = document.getElementById('lp-icon-results');
    if (!query.trim()) {
        resultsEl.innerHTML = '';
        return;
    }
    resultsEl.innerHTML = '<div class="lp-loading">' + (t('common.loading') || 'Loading...') + '</div>';

    try {
        const resp = await fetch(`/api/launchpad/icons/search?q=${encodeURIComponent(query)}`);
        if (!resp.ok) {
            resultsEl.innerHTML = '';
            return;
        }
        const results = await resp.json();
        renderIconResults(results);
    } catch (e) {
        console.error('Icon search failed:', e);
        resultsEl.innerHTML = '';
    }
}

function renderIconResults(results) {
    const el = document.getElementById('lp-icon-results');
    if (!results || results.length === 0) {
        el.innerHTML = '<div class="lp-empty-text" style="padding:12px;">' + (t('launchpad.no_icons') || 'No icons found.') + '</div>';
        return;
    }
    el.innerHTML = results.map(r => {
        const imgUrl = r.url_png || r.url_webp || r.url_svg;
        return `
            <div class="lp-icon-result" data-url="${esc(imgUrl)}" onclick="selectIcon(this, '${esc(imgUrl)}')">
                <img src="${esc(imgUrl)}" alt="${esc(r.name)}" loading="lazy">
                <span>${esc(r.name)}</span>
            </div>
        `;
    }).join('');
}

function selectIcon(el, url) {
    document.querySelectorAll('.lp-icon-result').forEach(r => r.classList.remove('selected'));
    el.classList.add('selected');
    selectedIconURL = url;
}

// ── Utilities ───────────────────────────────────────────────────────────────

function esc(str) {
    if (str == null) return '';
    return String(str)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#039;');
}
