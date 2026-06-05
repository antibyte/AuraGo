/* AuraGo – Knowledge Center JavaScript */
/* global t, showToast, closeModal */

// ═══════════════════════════════════════════════════════════════
// STATE
// ═══════════════════════════════════════════════════════════════
let allContacts = [];
let allFiles = [];
let allDevices = [];
let allCredentials = [];
let contactSearchTimer = null;
let previewResetTimer = null;
let pendingCredentialCertificateText = '';
let knowledgeDeleteInFlight = false;
let lastFocusedTrigger = null;

const sortState = {
    contacts: { col: 'name', dir: 'asc' },
    files: { col: 'name', dir: 'asc' },
    devices: { col: 'name', dir: 'asc' },
    credentials: { col: 'name', dir: 'asc' },
};

// PDF preview state
let pdfDoc = null;
let pdfCurrentPage = 1;
let pdfScale = 1.2;
if (typeof pdfjsLib !== 'undefined') {
    pdfjsLib.GlobalWorkerOptions.workerSrc = '/js/vendor/pdf.worker.min.js';
}

// ═══════════════════════════════════════════════════════════════
// INIT
// ═══════════════════════════════════════════════════════════════
document.addEventListener('DOMContentLoaded', () => {
    loadContacts();
    loadFiles();
    loadDevices();
    loadCredentials();
    restoreKnowledgeTab();
    initKeyboardNav();
    initSortableHeaders();
    initDropZone();
    updateTabIndicator();
    window.addEventListener('hashchange', onHashChange);
});

// ═══════════════════════════════════════════════════════════════
// TAB SWITCHING & NAVIGATION
// ═══════════════════════════════════════════════════════════════
const validTabs = ['contacts', 'files', 'devices', 'credentials', 'secrets', 'appointments', 'todos'];

function switchKCTab(tab) {
    if (!tab || !validTabs.includes(tab)) return;
    document.querySelectorAll('.kc-tab').forEach(t => {
        t.classList.remove('active');
        t.setAttribute('aria-selected', 'false');
        t.setAttribute('tabindex', '-1');
    });
    document.querySelectorAll('.kc-panel').forEach(p => p.classList.remove('active'));

    const tabBtn = document.getElementById('tab-' + tab);
    const panel = document.getElementById('panel-' + tab);
    if (!tabBtn || !panel) return;

    tabBtn.classList.add('active');
    tabBtn.setAttribute('aria-selected', 'true');
    tabBtn.setAttribute('tabindex', '0');
    panel.classList.add('active');

    try {
        if (window.location.hash !== '#' + tab) {
            history.replaceState(null, '', '#' + tab);
        }
    } catch (error) {
        console.debug('Failed to set hash:', error);
    }

    updateTabIndicator();
    updateTabFade();

    if (tab === 'appointments' && typeof loadAppointments === 'function') {
        loadAppointments();
    } else if (tab === 'todos' && typeof loadTodos === 'function') {
        loadTodos();
    } else if (tab === 'secrets') {
        if (!window._secretsModuleLoaded) {
            window._secretsModuleLoaded = true;
            const s = document.createElement('script');
            s.src = '/cfg/secrets.js';
            s.onload = () => {
                if (typeof renderSecretsSection === 'function') {
                    renderSecretsSection({
                        key: 'secrets',
                        icon: '🔐',
                        label: window.t ? window.t('knowledge.tab_secrets') : 'Secrets',
                        container: 'secrets-section-content'
                    });
                }
            };
            document.head.appendChild(s);
        } else if (typeof renderSecretsSection === 'function') {
            renderSecretsSection({
                key: 'secrets',
                icon: '🔐',
                label: window.t ? window.t('knowledge.tab_secrets') : 'Secrets',
                container: 'secrets-section-content'
            });
        }
    }
}
window.switchKCTab = switchKCTab;

function restoreKnowledgeTab() {
    let tab = 'contacts';
    try {
        const hash = window.location.hash.slice(1);
        if (hash && validTabs.includes(hash) && document.getElementById('tab-' + hash) && document.getElementById('panel-' + hash)) {
            tab = hash;
        }
    } catch (error) {
        console.debug('Failed to restore knowledge tab:', error);
    }
    switchKCTab(tab);
}

function onHashChange() {
    const hash = window.location.hash.slice(1);
    if (hash && validTabs.includes(hash)) {
        switchKCTab(hash);
    }
}

function updateTabIndicator() {
    const activeTab = document.querySelector('.kc-tab.active');
    const indicator = document.getElementById('kc-tab-indicator');
    if (!activeTab || !indicator) return;
    const tabsContainer = activeTab.closest('.kc-tabs');
    if (!tabsContainer) return;
    const containerRect = tabsContainer.getBoundingClientRect();
    const tabRect = activeTab.getBoundingClientRect();
    indicator.style.left = (tabRect.left - containerRect.left + tabsContainer.scrollLeft) + 'px';
    indicator.style.width = tabRect.width + 'px';
}

function updateTabFade() {
    const wrap = document.querySelector('.kc-tabs-wrap');
    const tabs = document.querySelector('.kc-tabs');
    if (!wrap || !tabs) return;
    const showStart = tabs.scrollLeft > 4;
    const showEnd = tabs.scrollLeft < tabs.scrollWidth - tabs.clientWidth - 4;
    wrap.classList.toggle('show-fade-start', showStart);
    wrap.classList.toggle('show-fade-end', showEnd);
}

// ═══════════════════════════════════════════════════════════════
// KEYBOARD NAVIGATION
// ═══════════════════════════════════════════════════════════════
function initKeyboardNav() {
    document.addEventListener('keydown', (e) => {
        if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA' || e.target.tagName === 'SELECT') return;
        if (e.key >= '1' && e.key <= '7') {
            const idx = parseInt(e.key) - 1;
            if (validTabs[idx]) {
                e.preventDefault();
                switchKCTab(validTabs[idx]);
            }
        }
        if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') {
            const currentTab = document.querySelector('.kc-tab.active');
            if (!currentTab || !currentTab.closest('.kc-tabs')?.contains(document.activeElement)) return;
            e.preventDefault();
            const idx = validTabs.indexOf(currentTab.id.replace('tab-', ''));
            const next = e.key === 'ArrowRight'
                ? (idx + 1) % validTabs.length
                : (idx - 1 + validTabs.length) % validTabs.length;
            switchKCTab(validTabs[next]);
            document.getElementById('tab-' + validTabs[next])?.focus();
        }
    });
}

// ═══════════════════════════════════════════════════════════════
// TAB COUNT UPDATES
// ═══════════════════════════════════════════════════════════════
function updateTabCount(tab, count) {
    const el = document.getElementById('count-' + tab);
    if (el) el.textContent = count;
}

function updateAllTabCounts() {
    updateTabCount('contacts', allContacts.length);
    updateTabCount('files', allFiles.length);
    updateTabCount('devices', allDevices.length);
    updateTabCount('credentials', allCredentials.length);
    updateTabCount('appointments', typeof allAppointments !== 'undefined' ? allAppointments.length : 0);
    updateTabCount('todos', typeof allTodos !== 'undefined' ? allTodos.length : 0);
    updateStatsBar();
}

// ═══════════════════════════════════════════════════════════════
// CONTACTS
// ═══════════════════════════════════════════════════════════════

function debounceContactSearch() {
    clearTimeout(contactSearchTimer);
    contactSearchTimer = setTimeout(() => loadContacts(), 300);
}

async function loadContacts() {
    const q = (document.getElementById('contacts-search')?.value || '').trim();
    const url = q ? '/api/contacts?q=' + encodeURIComponent(q) : '/api/contacts';
    showTableSkeleton('contacts-tbody', 5);
    try {
        const resp = await fetch(url).then(r => r.json());
        allContacts = resp || [];
        renderContacts();
        updateTabCount('contacts', allContacts.length);
        updateStatsBar();
    } catch (e) {
        console.error('Failed to load contacts:', e);
        showToast(t('common.error') + ': ' + e.message, 'error');
    }
}

function renderContacts() {
    const tbody = document.getElementById('contacts-tbody');
    const empty = document.getElementById('contacts-empty');
    const tableWrap = tbody.closest('.kc-table-wrap');

    if (!allContacts.length) {
        tbody.innerHTML = '';
        tableWrap.classList.add('is-hidden');
        empty.classList.remove('is-hidden');
        return;
    }
    tableWrap.classList.remove('is-hidden');
    empty.classList.add('is-hidden');

    const sorted = sortArray(allContacts, sortState.contacts);
    tbody.innerHTML = sorted.map(c => `
        <tr>
            <td class="kc-name">${esc(c.name)}</td>
            <td>${c.email ? '<a href="mailto:' + esc(c.email) + '">' + esc(c.email) + '</a>' : '—'}</td>
            <td>${esc(c.phone || '—')}</td>
            <td>${esc(c.mobile || '—')}</td>
            <td>${esc(c.relationship || '—')}</td>
            <td class="kc-actions">
                <button class="kc-icon-btn" onclick="editContact('${esc(c.id)}')" title="${t('common.btn_edit')}">${svgIcon('edit')}</button>
                <button class="kc-icon-btn kc-icon-btn-danger" onclick="askDeleteContact('${esc(c.id)}', '${esc(c.name)}')" title="${t('common.btn_delete')}">${svgIcon('delete')}</button>
            </td>
        </tr>
    `).join('');
}

function openContactModal(contact) {
    lastFocusedTrigger = document.activeElement;
    const modal = document.getElementById('contact-modal');
    const title = document.getElementById('contact-modal-title');

    document.getElementById('contact-id').value = contact ? contact.id : '';
    document.getElementById('contact-name').value = contact ? contact.name : '';
    document.getElementById('contact-email').value = contact ? contact.email : '';
    document.getElementById('contact-phone').value = contact ? contact.phone : '';
    document.getElementById('contact-mobile').value = contact ? contact.mobile : '';
    document.getElementById('contact-address').value = contact ? contact.address : '';
    document.getElementById('contact-relationship').value = contact ? contact.relationship : '';
    document.getElementById('contact-notes').value = contact ? contact.notes : '';

    title.textContent = contact ? t('knowledge.contacts_edit') : t('knowledge.contacts_add');
    modal.classList.add('active');
}

function editContact(id) {
    const c = allContacts.find(x => x.id === id);
    if (c) openContactModal(c);
}

async function saveContact() {
    const id = document.getElementById('contact-id').value;
    const data = {
        name: document.getElementById('contact-name').value.trim(),
        email: document.getElementById('contact-email').value.trim(),
        phone: document.getElementById('contact-phone').value.trim(),
        mobile: document.getElementById('contact-mobile').value.trim(),
        address: document.getElementById('contact-address').value.trim(),
        relationship: document.getElementById('contact-relationship').value.trim(),
        notes: document.getElementById('contact-notes').value.trim(),
    };

    if (!data.name) {
        showToast(t('knowledge.error_name_required'), 'error');
        return;
    }

    try {
        let resp;
        if (id) {
            data.id = id;
            resp = await fetch('/api/contacts/' + encodeURIComponent(id), {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data),
            });
        } else {
            resp = await fetch('/api/contacts', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data),
            });
        }

        if (!resp.ok) {
            const err = await resp.text();
            throw new Error(err);
        }

        closeModal('contact-modal');
        showToast(t('common.success'), 'success');
        loadContacts();
    } catch (e) {
        console.error('Save contact failed:', e);
        showToast(t('common.error') + ': ' + e.message, 'error');
    }
}

function askDeleteContact(id, name) {
    document.getElementById('delete-target-id').value = id;
    document.getElementById('delete-target-type').value = 'contact';
    document.getElementById('delete-confirm-text').textContent =
        t('knowledge.delete_confirm_contact').replace('{name}', name);
    document.getElementById('delete-modal').classList.add('active');
}

// ═══════════════════════════════════════════════════════════════
// KNOWLEDGE FILES
// ═══════════════════════════════════════════════════════════════

async function loadFiles() {
    showTableSkeleton('files-tbody', 3);
    try {
        const resp = await fetch('/api/knowledge').then(r => r.json());
        allFiles = resp || [];
        renderFiles();
        updateTabCount('files', allFiles.length);
        updateStatsBar();
    } catch (e) {
        console.error('Failed to load files:', e);
    }
}

function filterFiles() {
    renderFiles();
}

function renderFiles() {
    const tbody = document.getElementById('files-tbody');
    const empty = document.getElementById('files-empty');
    const tableWrap = tbody.closest('.kc-table-wrap');
    const q = (document.getElementById('files-search')?.value || '').toLowerCase();

    const filtered = q ? allFiles.filter(f => f.name.toLowerCase().includes(q)) : allFiles;

    if (!filtered.length) {
        tbody.innerHTML = '';
        tableWrap.classList.add('is-hidden');
        empty.classList.remove('is-hidden');
        return;
    }
    tableWrap.classList.remove('is-hidden');
    empty.classList.add('is-hidden');

    const sorted = sortArray(filtered, sortState.files);
    tbody.innerHTML = sorted.map(f => {
        const icon = fileIconSvg(f.name);
        const previewName = escAttr(f.name);
        return `
        <tr>
            <td>
                <a class="kc-file-link" href="#" onclick="openFilePreview('${previewName}'); return false;">
                    <span class="kc-file-icon">${icon}</span><span>${esc(f.name)}</span>
                </a>
            </td>
            <td class="kc-size">${formatSize(f.size)}</td>
            <td>${formatDate(f.modified)}</td>
            <td class="kc-actions">
                <button class="kc-icon-btn" onclick="openFilePreview('${previewName}')" title="${t('knowledge.files_preview')}">${svgIcon('eye')}</button>
                <a class="kc-icon-btn" href="/api/knowledge/${encodeURIComponent(f.name)}" target="_blank" title="${t('knowledge.files_download')}">${svgIcon('download')}</a>
                <button class="kc-icon-btn kc-icon-btn-danger" onclick="askDeleteFile('${esc(f.name)}')" title="${t('common.btn_delete')}">${svgIcon('delete')}</button>
            </td>
        </tr>`;
    }).join('');
}

function openFilePreview(name) {
    const modal = document.getElementById('file-preview-modal');
    const title = document.getElementById('file-preview-title');
    const subtitle = document.getElementById('file-preview-subtitle');
    const frame = document.getElementById('file-preview-frame');
    const textEl = document.getElementById('file-preview-text');
    const imgWrap = document.getElementById('file-preview-img-wrap');
    const imgEl = document.getElementById('file-preview-img');
    const fallback = document.getElementById('file-preview-fallback');
    const fallbackTitle = document.getElementById('file-preview-fallback-title');
    const fallbackText = document.getElementById('file-preview-fallback-text');
    const download = document.getElementById('file-preview-download');

    const previewURL = '/api/knowledge-inline/' + encodeURIComponent(name);
    const downloadURL = '/api/knowledge/' + encodeURIComponent(name);
    const ext = (name.split('.').pop() || '').toLowerCase();

    title.textContent = name;
    subtitle.textContent = formatPreviewSubtitle(name);
    download.href = downloadURL;

    // Reset all preview panels
    clearTimeout(previewResetTimer);
    frame.onload = null;
    frame.src = 'about:blank';
    frame.classList.add('is-hidden');
    textEl.textContent = '';
    textEl.classList.add('is-hidden');
    imgEl.src = '';
    imgWrap.classList.add('is-hidden');
    fallback.classList.add('is-hidden');

    modal.classList.add('active');

    if (isImageFile(ext)) {
        // ── Images: <img> tag is reliable for all image types ──
        imgEl.alt = name;
        imgEl.src = previewURL;
        imgWrap.classList.remove('is-hidden');

    } else if (ext === 'pdf') {
        // ── PDF: pdf.js renders natively without iframe ──
        const pdfWrap = document.getElementById('file-preview-pdf-wrap');
        const canvas = document.getElementById('file-preview-pdf-canvas');
        const controls = document.getElementById('file-preview-pdf-controls');
        const pdfPageInfo = document.getElementById('pdf-page-info');
        pdfDoc = null;
        pdfCurrentPage = 1;
        pdfScale = 1.0;
        pdfWrap.classList.remove('is-hidden');
        controls.classList.remove('is-hidden');
        // Clear any stale canvas size; will be set properly after PDF loads
        canvas.width = 0;
        canvas.height = 0;

        fetch(previewURL)
            .then(r => {
                if (!r.ok) throw new Error('HTTP ' + r.status);
                return r.arrayBuffer();
            })
            .then(data => pdfjsLib.getDocument({ data }).promise)
            .then(doc => {
                pdfDoc = doc;
                pdfPageInfo.textContent = `1 / ${doc.numPages}`;
                document.getElementById('pdf-prev-btn').disabled = doc.numPages <= 1;
                document.getElementById('pdf-next-btn').disabled = doc.numPages <= 1;
                renderPdfPage(1);
            })
            .catch(err => {
                console.error('PDF preview failed:', err);
                pdfWrap.classList.add('is-hidden');
                fallbackTitle.textContent = t('knowledge.files_preview_unavailable_title');
                fallbackText.textContent = t('knowledge.files_preview_render_error');
                fallback.classList.remove('is-hidden');
            });

    } else if (ext === 'html' || ext === 'htm') {
        // ── HTML: iframe renders these natively ──
        frame.onload = () => {
            clearTimeout(previewResetTimer);
            fallback.classList.add('is-hidden');
        };
        frame.src = previewURL;
        frame.classList.remove('is-hidden');
        previewResetTimer = setTimeout(() => {
            fallbackTitle.textContent = t('knowledge.files_preview_unavailable_title');
            fallbackText.textContent = t('knowledge.files_preview_render_error');
            fallback.classList.remove('is-hidden');
        }, 4000);

    } else if (isTextFile(ext)) {
        // ── Text / code: fetch and render in <pre> ──
        // This avoids MIME-type download issues (yaml, md, json, csv, etc.)
        textEl.classList.remove('is-hidden');
        textEl.textContent = t('common.loading') || 'Loading…';
        const maxBytes = 512 * 1024; // 512 KB display limit
        fetch(previewURL)
            .then(r => {
                if (!r.ok) throw new Error('HTTP ' + r.status);
                return r.text();
            })
            .then(text => {
                textEl.textContent = text.length > maxBytes
                    ? text.slice(0, maxBytes) + '\n\n[… ' + t('knowledge.files_preview_truncated') + ']'
                    : text;
            })
            .catch(err => {
                console.error('Preview fetch failed:', err);
                textEl.classList.add('is-hidden');
                fallbackTitle.textContent = t('knowledge.files_preview_unavailable_title');
                fallbackText.textContent = t('knowledge.files_preview_render_error');
                fallback.classList.remove('is-hidden');
            });

    } else {
        // ── Unknown type: friendly fallback with download button ──
        fallbackTitle.textContent = t('knowledge.files_preview_unavailable_title');
        fallbackText.textContent = t('knowledge.files_preview_unavailable_desc');
        fallback.classList.remove('is-hidden');
    }
}

// ─── PDF rendering helpers ─────────────────────────────────────────────────

function renderPdfPage(pageNum) {
    if (!pdfDoc) return;
    const canvas = document.getElementById('file-preview-pdf-canvas');
    const wrap = document.getElementById('file-preview-pdf-wrap');
    const ctx = canvas.getContext('2d');

    pdfDoc.getPage(pageNum).then(page => {
        // Get original page size at scale 1
        const originalViewport = page.getViewport({ scale: 1 });

        // Calculate scale to fit the PDF within the available container space
        // Account for padding (16px each side) and controls area (~60px)
        const availableWidth = wrap.clientWidth - 32;
        const availableHeight = wrap.clientHeight - 80;

        if (availableWidth > 0 && availableHeight > 0) {
            const scaleByWidth = availableWidth / originalViewport.width;
            const scaleByHeight = availableHeight / originalViewport.height;
            // fitScale is the scale that fits the whole PDF in the container
            const fitScale = Math.min(scaleByWidth, scaleByHeight);
            // Only update pdfScale if it's still at the default (1.0)
            // This preserves user zoom while ensuring initial fit
            if (pdfScale === 1.0) {
                pdfScale = fitScale;
            }
        }

        const viewport = page.getViewport({ scale: pdfScale });
        canvas.width = viewport.width;
        canvas.height = viewport.height;
        page.render({ canvasContext: ctx, viewport }).promise;
    });

    document.getElementById('pdf-page-info').textContent = `${pageNum} / ${pdfDoc.numPages}`;
    document.getElementById('pdf-prev-btn').disabled = pageNum <= 1;
    document.getElementById('pdf-next-btn').disabled = pageNum >= pdfDoc.numPages;
}

function pdfPreviewPrev() {
    if (pdfCurrentPage <= 1) return;
    pdfCurrentPage--;
    renderPdfPage(pdfCurrentPage);
}

function pdfPreviewNext() {
    if (!pdfDoc || pdfCurrentPage >= pdfDoc.numPages) return;
    pdfCurrentPage++;
    renderPdfPage(pdfCurrentPage);
}

function pdfPreviewZoomIn() {
    pdfScale = Math.min(pdfScale + 0.3, 4);
    renderPdfPage(pdfCurrentPage);
}

function pdfPreviewZoomOut() {
    pdfScale = Math.max(pdfScale - 0.3, 0.4);
    renderPdfPage(pdfCurrentPage);
}

function closeFilePreview() {
    const modal = document.getElementById('file-preview-modal');
    const frame = document.getElementById('file-preview-frame');
    const textEl = document.getElementById('file-preview-text');
    const imgWrap = document.getElementById('file-preview-img-wrap');
    const imgEl = document.getElementById('file-preview-img');
    const pdfWrap = document.getElementById('file-preview-pdf-wrap');
    const fallback = document.getElementById('file-preview-fallback');

    clearTimeout(previewResetTimer);
    previewResetTimer = null;
    frame.onload = null;
    frame.src = 'about:blank';
    frame.classList.add('is-hidden');
    textEl.textContent = '';
    textEl.classList.add('is-hidden');
    imgEl.src = '';
    imgWrap.classList.add('is-hidden');
    pdfWrap.classList.add('is-hidden');
    pdfDoc = null;
    fallback.classList.add('is-hidden');
    modal.classList.remove('active');
}

async function uploadFiles(input) {
    const files = input.files;
    if (!files || !files.length) return;

    const progressWrap = document.querySelector('.kc-upload-progress') || createUploadProgress();
    const progressFill = progressWrap.querySelector('.kc-upload-progress-fill');
    const progressText = progressWrap.querySelector('.kc-upload-progress-text');
    progressWrap.classList.add('is-active');
    progressFill.style.width = '0%';

    let uploaded = 0;
    const total = files.length;

    for (let i = 0; i < files.length; i++) {
        const form = new FormData();
        form.append('file', files[i]);
        progressText.textContent = t('knowledge.files_uploading') + ` ${files[i].name} (${i + 1}/${total})`;

        try {
            const resp = await fetch('/api/knowledge/upload', {
                method: 'POST',
                body: form,
            });
            if (!resp.ok) {
                const err = await resp.text();
                throw new Error(err);
            }
            uploaded++;
            progressFill.style.width = Math.round((uploaded / total) * 100) + '%';
        } catch (e) {
            console.error('Upload failed:', e);
            showToast(t('common.error') + ': ' + e.message, 'error');
        }
    }

    if (uploaded > 0) {
        showToast(t('knowledge.files_uploaded'), 'success');
        loadFiles();
    }

    setTimeout(() => {
        progressWrap.classList.remove('is-active');
    }, 1500);
    input.value = '';
}

function createUploadProgress() {
    const existing = document.querySelector('.kc-upload-progress');
    if (existing) return existing;
    const div = document.createElement('div');
    div.className = 'kc-upload-progress';
    div.innerHTML = `
        <div class="kc-upload-progress-bar"><div class="kc-upload-progress-fill"></div></div>
        <div class="kc-upload-progress-text"></div>
    `;
    const panelHeader = document.querySelector('#panel-files .kc-panel-header');
    if (panelHeader) panelHeader.after(div);
    return div;
}

// Keep backward-compatible single-file function
async function uploadFile(input) {
    return uploadFiles(input);
}

function askDeleteFile(name) {
    document.getElementById('delete-target-id').value = name;
    document.getElementById('delete-target-type').value = 'file';
    document.getElementById('delete-confirm-text').textContent =
        t('knowledge.delete_confirm_file').replace('{name}', name);
    document.getElementById('delete-modal').classList.add('active');
}

// ═══════════════════════════════════════════════════════════════
// DEVICES
// ═══════════════════════════════════════════════════════════════

async function loadDevices() {
    showTableSkeleton('devices-tbody', 4);
    try {
        const resp = await fetch('/api/devices');
        if (!resp.ok) throw new Error(await resp.text());
        allDevices = await resp.json() || [];
        renderDevices();
        updateTabCount('devices', allDevices.length);
        updateStatsBar();
    } catch (e) {
        console.error('Failed to load devices:', e);
        showToast(t('common.error') + ': ' + e.message, 'error');
    }
}

function filterDevices() {
    renderDevices();
}

function renderDevices() {
    const tbody = document.getElementById('devices-tbody');
    const empty = document.getElementById('devices-empty');
    const tableWrap = tbody.closest('.kc-table-wrap');
    const q = (document.getElementById('devices-search')?.value || '').toLowerCase();

    const filtered = q ? allDevices.filter(d => {
        const hay = [
            d.name,
            d.type,
            d.ip_address,
            getDeviceAccessSearchText(d),
            d.description,
            d.mac_address,
            ...(d.tags || [])
        ].join(' ').toLowerCase();
        return hay.includes(q);
    }) : allDevices;

    if (!filtered.length) {
        tbody.innerHTML = '';
        tableWrap.style.display = 'none';
        empty.style.display = '';
        return;
    }
    tableWrap.style.display = '';
    empty.style.display = 'none';

    const sorted = sortArray(filtered, sortState.devices);
    tbody.innerHTML = sorted.map(d => {
        const tags = (d.tags || []).map(tag =>
            '<span class="kc-tag">' + esc(tag) + '</span>'
        ).join('') || '—';
        return `
        <tr>
            <td class="kc-name kc-wrap">${esc(d.name)}</td>
            <td><span class="kc-type-badge">${esc(d.type)}</span></td>
            <td class="kc-mono">${esc(d.ip_address || '—')}</td>
            <td>${d.port || '—'}</td>
            <td class="kc-wrap">${esc(getDeviceAccessLabel(d))}</td>
            <td class="kc-mono kc-size">${esc(d.mac_address || '—')}</td>
            <td class="kc-tags-cell">${tags}</td>
            <td class="kc-actions">
                <button class="kc-icon-btn" onclick="editDevice('${esc(d.id)}')" title="${t('common.btn_edit')}">${svgIcon('edit')}</button>
                <button class="kc-icon-btn kc-icon-btn-danger" onclick="askDeleteDevice('${esc(d.id)}', '${esc(d.name)}')" title="${t('common.btn_delete')}">${svgIcon('delete')}</button>
            </td>
        </tr>`;
    }).join('');
}

function openDeviceModal(device) {
    const modal = document.getElementById('device-modal');
    const title = document.getElementById('device-modal-title');

    document.getElementById('device-id').value = device ? device.id : '';
    document.getElementById('device-name').value = device ? device.name : '';
    document.getElementById('device-type').value = device ? (device.type || 'server') : 'server';
    document.getElementById('device-ip').value = device ? device.ip_address : '';
    document.getElementById('device-port').value = device ? (device.port || 22) : 22;
    document.getElementById('device-description').value = device ? device.description : '';
    document.getElementById('device-mac').value = device ? device.mac_address : '';
    document.getElementById('device-tags').value = device ? (device.tags || []).join(', ') : '';
    document.getElementById('device-credential').innerHTML = renderDeviceCredentialOptions(device ? (device.credential_id || '') : '');

    title.textContent = device ? t('knowledge.devices_edit') : t('knowledge.devices_add');
    modal.classList.add('active');
}

function editDevice(id) {
    const d = allDevices.find(x => x.id === id);
    if (d) openDeviceModal(d);
}

async function saveDevice() {
    const name = document.getElementById('device-name').value.trim();
    if (!name) {
        showToast(t('knowledge.devices_name_required'), 'error');
        return;
    }

    const tagsRaw = document.getElementById('device-tags').value.trim();
    const tags = tagsRaw ? tagsRaw.split(',').map(s => s.trim()).filter(Boolean) : [];

    const data = {
        name: name,
        type: document.getElementById('device-type').value,
        ip_address: document.getElementById('device-ip').value.trim(),
        port: parseInt(document.getElementById('device-port').value) || 22,
        credential_id: document.getElementById('device-credential').value || '',
        description: document.getElementById('device-description').value.trim(),
        mac_address: document.getElementById('device-mac').value.trim(),
        tags: tags
    };

    const id = document.getElementById('device-id').value;
    try {
        let resp;
        if (id) {
            resp = await fetch('/api/devices/' + encodeURIComponent(id), {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data),
            });
        } else {
            resp = await fetch('/api/devices', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data),
            });
        }
        if (!resp.ok) {
            const err = await resp.json().catch(() => ({}));
            throw new Error(err.error || 'Request failed');
        }
        closeModal('device-modal');
        showToast(t('common.success'), 'success');
        loadDevices();
    } catch (e) {
        console.error('Save device failed:', e);
        showToast(t('common.error') + ': ' + e.message, 'error');
    }
}

function askDeleteDevice(id, name) {
    document.getElementById('delete-target-id').value = id;
    document.getElementById('delete-target-type').value = 'device';
    document.getElementById('delete-confirm-text').textContent =
        t('knowledge.devices_delete_confirm').replace('{name}', name);
    document.getElementById('delete-modal').classList.add('active');
}

// ═══════════════════════════════════════════════════════════════
// CREDENTIALS
// ═══════════════════════════════════════════════════════════════

async function loadCredentials() {
    showTableSkeleton('credentials-tbody', 3);
    try {
        const resp = await fetch('/api/credentials');
        if (!resp.ok) throw new Error(await resp.text());
        allCredentials = await resp.json() || [];
        renderCredentials();
        renderDevices();
        updateTabCount('credentials', allCredentials.length);
        updateStatsBar();
    } catch (e) {
        console.error('Failed to load credentials:', e);
        showToast(t('common.error') + ': ' + e.message, 'error');
    }
}

function getCredentialById(id) {
    return allCredentials.find(c => c.id === id) || null;
}

function getCredentialOptionLabel(credential) {
    if (!credential) return t('knowledge.devices_access_none');
    const details = [credential.username, credential.host].filter(Boolean).join('@');
    return details ? `${credential.name} - ${details}` : credential.name;
}

function renderDeviceCredentialOptions(selectedId) {
    const options = [
        `<option value="">${esc(t('knowledge.devices_access_placeholder'))}</option>`
    ];
    allCredentials.forEach(credential => {
        const selected = credential.id === selectedId ? ' selected' : '';
        options.push(`<option value="${escAttr(credential.id)}"${selected}>${esc(getCredentialOptionLabel(credential))}</option>`);
    });
    return options.join('');
}

function getDeviceAccessLabel(device) {
    if (!device || !device.credential_id) {
        return t('knowledge.devices_access_none');
    }
    const credential = getCredentialById(device.credential_id);
    if (!credential) {
        return t('knowledge.devices_access_missing');
    }
    return credential.name;
}

function getDeviceAccessSearchText(device) {
    if (!device) return '';
    const credential = getCredentialById(device.credential_id);
    if (!credential) {
        return '';
    }
    return [credential.name, credential.host, credential.username, credential.description].join(' ');
}

function filterCredentials() {
    renderCredentials();
}

function renderCredentials() {
    const tbody = document.getElementById('credentials-tbody');
    const empty = document.getElementById('credentials-empty');
    const tableWrap = tbody.closest('.kc-table-wrap');
    const q = (document.getElementById('credentials-search')?.value || '').toLowerCase();

    const filtered = q ? allCredentials.filter(c => {
        const hay = [c.name, c.type, c.host, c.username, c.description].join(' ').toLowerCase();
        return hay.includes(q);
    }) : allCredentials;

    if (!filtered.length) {
        tbody.innerHTML = '';
        tableWrap.classList.add('is-hidden');
        empty.classList.remove('is-hidden');
        return;
    }
    tableWrap.classList.remove('is-hidden');
    empty.classList.add('is-hidden');

    const sorted = sortArray(filtered, sortState.credentials);
    tbody.innerHTML = sorted.map(c => `
        <tr>
            <td class="kc-name">${esc(c.name)}</td>
            <td><span class="kc-type-badge">${esc((c.type || 'ssh').toUpperCase())}</span></td>
            <td class="kc-mono">${esc(c.host || '—')}</td>
            <td>${esc(c.username || '—')}</td>
            <td>${c.has_password ? '<span class="kc-state-chip kc-state-ok">' + esc(t('knowledge.credentials_state_present')) + '</span>' : '<span class="kc-state-chip">' + esc(t('knowledge.credentials_state_missing')) + '</span>'}</td>
            <td>${c.has_certificate ? '<span class="kc-state-chip kc-state-ok">' + esc(t('knowledge.credentials_state_present')) + '</span>' : '<span class="kc-state-chip">' + esc(t('knowledge.credentials_state_missing')) + '</span>'}</td>
            <td>${c.allow_python ? '<span class="kc-state-chip kc-state-ok">✓</span>' : '<span class="kc-state-chip">—</span>'}</td>
            <td class="kc-actions">
                <button class="kc-icon-btn" onclick="editCredential('${esc(c.id)}')" title="${t('common.btn_edit')}">${svgIcon('edit')}</button>
                <button class="kc-icon-btn kc-icon-btn-danger" onclick="askDeleteCredential('${esc(c.id)}', '${esc(c.name)}')" title="${t('common.btn_delete')}">${svgIcon('delete')}</button>
            </td>
        </tr>
    `).join('');
}

function openCredentialModal(credential) {
    const modal = document.getElementById('credential-modal');
    document.getElementById('credential-modal-title').textContent = credential ? t('knowledge.credentials_edit') : t('knowledge.credentials_add');
    document.getElementById('credential-id').value = credential ? credential.id : '';
    document.getElementById('credential-name').value = credential ? credential.name : '';
    document.getElementById('credential-type').value = credential ? (credential.type || 'ssh') : 'ssh';
    document.getElementById('credential-host').value = credential ? credential.host : '';
    document.getElementById('credential-username').value = credential ? credential.username : '';
    document.getElementById('credential-password').value = '';
    document.getElementById('credential-token').value = '';
    document.getElementById('credential-description').value = credential ? (credential.description || '') : '';
    document.getElementById('credential-certificate-mode').value = credential ? (credential.certificate_mode || 'text') : 'text';
    document.getElementById('credential-certificate-text').value = '';
    document.getElementById('credential-certificate-file').value = '';
    document.getElementById('credential-certificate-file-state').textContent = '';
    document.getElementById('credential-allow-python').checked = credential ? !!credential.allow_python : false;
    pendingCredentialCertificateText = '';

    document.getElementById('credential-password-state').classList.toggle('is-hidden', !(credential && credential.has_password));
    document.getElementById('credential-certificate-state').classList.toggle('is-hidden', !(credential && credential.has_certificate));
    document.getElementById('credential-token-state').classList.toggle('is-hidden', !(credential && credential.has_token));

    updateCredentialCertificateMode();
    updateCredentialTypeFields();
    modal.classList.add('active');
}

function editCredential(id) {
    const credential = allCredentials.find(x => x.id === id);
    if (credential) openCredentialModal(credential);
}

function updateCredentialCertificateMode() {
    const mode = document.getElementById('credential-certificate-mode').value;
    document.getElementById('credential-certificate-text-group').classList.toggle('is-hidden', mode !== 'text');
    document.getElementById('credential-certificate-file-group').classList.toggle('is-hidden', mode !== 'upload');
}

function updateCredentialTypeFields() {
    const type = document.getElementById('credential-type').value;
    const isSSH = type === 'ssh';
    const isToken = type === 'token';
    // Certificate section: only for SSH
    document.getElementById('credential-certificate-section').classList.toggle('is-hidden', !isSSH);
    // Password group: for SSH and Login, hidden for Token
    document.getElementById('credential-password-group').classList.toggle('is-hidden', isToken);
    // Token group: only for Token type
    document.getElementById('credential-token-group').classList.toggle('is-hidden', !isToken);
    // Host hint: optional for Login and Token types
    document.getElementById('credential-host-hint').classList.toggle('is-hidden', isSSH);
}

async function handleCredentialCertificateUpload(event) {
    const file = event.target.files && event.target.files[0];
    const state = document.getElementById('credential-certificate-file-state');
    pendingCredentialCertificateText = '';
    state.textContent = '';
    if (!file) return;

    try {
        pendingCredentialCertificateText = await file.text();
        state.textContent = t('knowledge.credentials_certificate_loaded').replace('{name}', file.name);
    } catch (e) {
        console.error('Failed to read certificate file:', e);
        state.textContent = t('knowledge.credentials_certificate_load_failed');
        showToast(t('common.error') + ': ' + e.message, 'error');
    }
}

async function saveCredential() {
    const name = document.getElementById('credential-name').value.trim();
    const type = document.getElementById('credential-type').value || 'ssh';
    const host = document.getElementById('credential-host').value.trim();
    const username = document.getElementById('credential-username').value.trim();
    if (!name) {
        showToast(t('knowledge.credentials_name_required'), 'error');
        return;
    }
    if (type === 'ssh' && !host) {
        showToast(t('knowledge.credentials_host_required'), 'error');
        return;
    }
    if (!username) {
        showToast(t('knowledge.credentials_username_required'), 'error');
        return;
    }

    const mode = document.getElementById('credential-certificate-mode').value;
    const data = {
        name,
        type,
        host,
        username,
        description: document.getElementById('credential-description').value.trim(),
        password: document.getElementById('credential-password').value,
        token: document.getElementById('credential-token').value,
        allow_python: document.getElementById('credential-allow-python').checked,
        certificate_mode: mode,
        certificate_text: mode === 'upload'
            ? pendingCredentialCertificateText
            : document.getElementById('credential-certificate-text').value
    };

    const id = document.getElementById('credential-id').value;
    try {
        const resp = await fetch(id ? '/api/credentials/' + encodeURIComponent(id) : '/api/credentials', {
            method: id ? 'PUT' : 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data),
        });
        if (!resp.ok) {
            const err = await resp.json().catch(() => ({}));
            throw new Error(err.error || 'Request failed');
        }
        closeModal('credential-modal');
        showToast(t('common.success'), 'success');
        loadCredentials();
    } catch (e) {
        console.error('Save credential failed:', e);
        showToast(t('common.error') + ': ' + e.message, 'error');
    }
}

function askDeleteCredential(id, name) {
    document.getElementById('delete-target-id').value = id;
    document.getElementById('delete-target-type').value = 'credential';
    document.getElementById('delete-confirm-text').textContent =
        t('knowledge.credentials_delete_confirm').replace('{name}', name);
    document.getElementById('delete-modal').classList.add('active');
}

// ═══════════════════════════════════════════════════════════════
// DELETE CONFIRM
// ═══════════════════════════════════════════════════════════════

async function confirmDelete() {
    if (knowledgeDeleteInFlight) return;
    knowledgeDeleteInFlight = true;
    setKnowledgeDeleteBusy(true);
    const id = document.getElementById('delete-target-id').value;
    const type = document.getElementById('delete-target-type').value;
    const controller = typeof AbortController !== 'undefined' ? new AbortController() : null;
    const timeout = controller ? setTimeout(() => controller.abort(), 15000) : null;

    try {
        let resp;
        if (type === 'contact') {
            resp = await fetch('/api/contacts/' + encodeURIComponent(id), { method: 'DELETE', signal: controller ? controller.signal : undefined });
        } else if (type === 'device') {
            resp = await fetch('/api/devices/' + encodeURIComponent(id), { method: 'DELETE', signal: controller ? controller.signal : undefined });
        } else if (type === 'credential') {
            resp = await fetch('/api/credentials/' + encodeURIComponent(id), { method: 'DELETE', signal: controller ? controller.signal : undefined });
        } else if (type === 'appointment') {
            resp = await fetch('/api/appointments/' + encodeURIComponent(id), { method: 'DELETE', signal: controller ? controller.signal : undefined });
        } else if (type === 'todo') {
            resp = await fetch('/api/todos/' + encodeURIComponent(id), { method: 'DELETE', signal: controller ? controller.signal : undefined });
        } else {
            resp = await fetch('/api/knowledge/' + encodeURIComponent(id), { method: 'DELETE', signal: controller ? controller.signal : undefined });
        }
        if (!resp.ok) {
            const err = await resp.text();
            throw new Error(err);
        }
        closeModal('delete-modal');
        showToast(t('common.success'), 'success');
        if (type === 'contact') loadContacts();
        else if (type === 'device') loadDevices();
        else if (type === 'credential') loadCredentials();
        else if (type === 'appointment') loadAppointments();
        else if (type === 'todo') loadTodos();
        else loadFiles();
    } catch (e) {
        console.error('Delete failed:', e);
        showToast(t('common.error') + ': ' + e.message, 'error');
    } finally {
        if (timeout) clearTimeout(timeout);
        knowledgeDeleteInFlight = false;
        setKnowledgeDeleteBusy(false);
    }
}

function setKnowledgeDeleteBusy(busy) {
    const confirmBtn = document.getElementById('knowledge-delete-confirm-btn');
    if (confirmBtn) {
        confirmBtn.disabled = busy;
    }
}

// ═══════════════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════════════

function formatSize(bytes) {
    if (!bytes || bytes === 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB'];
    let i = 0;
    let size = bytes;
    while (size >= 1024 && i < units.length - 1) { size /= 1024; i++; }
    return size.toFixed(i === 0 ? 0 : 1) + ' ' + units[i];
}

function formatDate(iso) {
    if (!iso) return '—';
    try {
        const d = new Date(iso);
        return d.toLocaleDateString() + ' ' + d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    } catch { return iso; }
}

// Returns true for plain-text based formats that can be fetched and
// displayed in a <pre> block regardless of MIME type.
function isTextFile(ext) {
    return new Set([
        'txt', 'md', 'json', 'yaml', 'yml', 'csv', 'log', 'xml',
        'py', 'js', 'ts', 'go', 'sh', 'bat', 'ps1', 'sql',
        'ini', 'conf', 'cfg', 'toml', 'env', 'gitignore', 'dockerfile',
        'css', 'scss', 'less', 'rs', 'c', 'cpp', 'h', 'java', 'rb',
    ]).has(ext);
}

// Returns true for image formats the browser can display natively via <img>.
function isImageFile(ext) {
    return new Set(['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg', 'bmp', 'ico']).has(ext);
}

function formatPreviewSubtitle(name) {
    const file = allFiles.find(f => f.name === name);
    if (!file) return '';
    return [formatSize(file.size), formatDate(file.modified)].filter(Boolean).join(' • ');
}

function escAttr(value) {
    return String(value)
        .replace(/&/g, '&amp;')
        .replace(/'/g, '&#39;')
        .replace(/"/g, '&quot;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;');
}

// ═══════════════════════════════════════════════════════════════
// SVG ICONS
// ═══════════════════════════════════════════════════════════════

function svgIcon(name) {
    const icons = {
        edit: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17 3a2.828 2.828 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5L17 3z"/></svg>',
        delete: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>',
        eye: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>',
        download: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>',
        preview: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>',
    };
    return icons[name] || '';
}

function fileIconSvg(name) {
    const ext = (name.split('.').pop() || '').toLowerCase();
    const colorMap = {
        md: '#8b5cf6', txt: '#6b7280', json: '#eab308', yaml: '#3b82f6', yml: '#3b82f6',
        csv: '#22c55e', log: '#9ca3af', pdf: '#ef4444', xml: '#f97316', html: '#e11d48', htm: '#e11d48',
        py: '#3b82f6', go: '#06b6d4', js: '#eab308', ts: '#3b82f6', sh: '#6b7280', bat: '#6b7280',
        png: '#8b5cf6', jpg: '#8b5cf6', jpeg: '#8b5cf6', gif: '#8b5cf6', webp: '#8b5cf6', svg: '#8b5cf6',
    };
    const isCode = ['py', 'go', 'js', 'ts', 'sh', 'bat', 'css', 'html'].includes(ext);
    const isImage = ['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg', 'bmp'].includes(ext);
    const isData = ['json', 'yaml', 'yml', 'csv', 'xml'].includes(ext);
    const color = colorMap[ext] || '#6b7280';

    if (isImage) {
        return `<svg viewBox="0 0 16 16" style="color:${color}"><rect x="1.5" y="2.5" width="13" height="11" rx="2"/><circle cx="5.5" cy="6.5" r="1.5"/><path d="M14.5 10.5l-3.5-3.5L4 14"/></svg>`;
    }
    if (isCode) {
        return `<svg viewBox="0 0 16 16" style="color:${color}"><path d="M5 4L1 8l4 4M11 4l4 4-4 4M9 2l-2 12"/></svg>`;
    }
    if (isData) {
        return `<svg viewBox="0 0 16 16" style="color:${color}"><rect x="2" y="1.5" width="12" height="13" rx="2"/><path d="M5 5h6M5 8h6M5 11h4"/></svg>`;
    }
    if (ext === 'pdf') {
        return `<svg viewBox="0 0 16 16" style="color:${color}"><rect x="2" y="1.5" width="12" height="13" rx="2"/><path d="M5 5h6M5 8h4"/></svg>`;
    }
    return `<svg viewBox="0 0 16 16" style="color:${color}"><rect x="2" y="1.5" width="12" height="13" rx="2"/><path d="M5 5h6M5 8h6M5 11h4"/></svg>`;
}

// Keep backward-compatible emoji icon function
function fileIcon(name) {
    const ext = (name.split('.').pop() || '').toLowerCase();
    const icons = {
        md: '📝', txt: '📄', json: '📋', yaml: '⚙️', yml: '⚙️',
        csv: '📊', log: '📃', pdf: '📕', xml: '📰', html: '🌐', htm: '🌐',
        py: '🐍', go: '🔷', js: '🟨', ts: '🔷', sh: '🖥️', bat: '🖥️',
        png: '🖼️', jpg: '🖼️', jpeg: '🖼️', gif: '🖼️', webp: '🖼️', svg: '🖼️',
    };
    return icons[ext] || '📄';
}

// ═══════════════════════════════════════════════════════════════
// SORTING
// ═══════════════════════════════════════════════════════════════

function initSortableHeaders() {
    document.querySelectorAll('th[data-sort-col]').forEach(th => {
        th.addEventListener('click', () => {
            const col = th.dataset.sortCol;
            const table = th.closest('table');
            const tbody = table.querySelector('tbody');
            const tabId = getTabIdFromTable(table);
            if (!tabId || !sortState[tabId]) return;

            if (sortState[tabId].col === col) {
                sortState[tabId].dir = sortState[tabId].dir === 'asc' ? 'desc' : 'asc';
            } else {
                sortState[tabId].col = col;
                sortState[tabId].dir = 'asc';
            }

            updateSortIndicators(table, tabId);
            rerenderTab(tabId);
        });
    });
}

function getTabIdFromTable(table) {
    const id = table.id;
    if (id === 'contacts-table') return 'contacts';
    if (id === 'files-table') return 'files';
    if (id === 'devices-table') return 'devices';
    if (id === 'credentials-table') return 'credentials';
    return null;
}

function updateSortIndicators(table, tabId) {
    table.querySelectorAll('th[data-sort-col]').forEach(th => {
        th.classList.remove('kc-sort-active', 'kc-sort-asc', 'kc-sort-desc');
        if (th.dataset.sortCol === sortState[tabId].col) {
            th.classList.add('kc-sort-active', sortState[tabId].dir === 'asc' ? 'kc-sort-asc' : 'kc-sort-desc');
            th.setAttribute('aria-sort', sortState[tabId].dir === 'asc' ? 'ascending' : 'descending');
        } else {
            th.removeAttribute('aria-sort');
        }
    });
}

function sortArray(arr, state) {
    if (!state || !state.col) return arr;
    const sorted = [...arr];
    const dir = state.dir === 'asc' ? 1 : -1;
    sorted.sort((a, b) => {
        let va = a[state.col];
        let vb = b[state.col];
        if (va == null) va = '';
        if (vb == null) vb = '';
        if (typeof va === 'string') va = va.toLowerCase();
        if (typeof vb === 'string') vb = vb.toLowerCase();
        if (va < vb) return -1 * dir;
        if (va > vb) return 1 * dir;
        return 0;
    });
    return sorted;
}

function rerenderTab(tabId) {
    switch (tabId) {
        case 'contacts': renderContacts(); break;
        case 'files': renderFiles(); break;
        case 'devices': renderDevices(); break;
        case 'credentials': renderCredentials(); break;
    }
}

// ═══════════════════════════════════════════════════════════════
// SKELETON LOADING
// ═══════════════════════════════════════════════════════════════

function showTableSkeleton(tbodyId, cols) {
    const tbody = document.getElementById(tbodyId);
    if (!tbody) return;
    const widths = ['60%', '80%', '50%', '70%', '40%', '55%'];
    let html = '';
    for (let i = 0; i < 4; i++) {
        html += '<tr class="kc-skeleton-row">';
        for (let j = 0; j < cols; j++) {
            html += `<td><span class="kc-skeleton-cell" style="width:${widths[(i + j) % widths.length]}"></span></td>`;
        }
        html += '</tr>';
    }
    tbody.innerHTML = html;
}

// ═══════════════════════════════════════════════════════════════
// DRAG AND DROP
// ═══════════════════════════════════════════════════════════════

function initDropZone() {
    const panel = document.getElementById('panel-files');
    const dropZone = document.getElementById('files-drop-zone');
    if (!panel || !dropZone) return;

    let dragCounter = 0;

    panel.addEventListener('dragenter', (e) => {
        e.preventDefault();
        dragCounter++;
        dropZone.classList.add('kc-drop-active');
    });

    panel.addEventListener('dragleave', (e) => {
        e.preventDefault();
        dragCounter--;
        if (dragCounter <= 0) {
            dragCounter = 0;
            dropZone.classList.remove('kc-drop-active');
        }
    });

    panel.addEventListener('dragover', (e) => {
        e.preventDefault();
    });

    panel.addEventListener('drop', (e) => {
        e.preventDefault();
        dragCounter = 0;
        dropZone.classList.remove('kc-drop-active');
        const files = e.dataTransfer?.files;
        if (files && files.length) {
            const input = document.getElementById('file-upload-input');
            if (input) {
                const dt = new DataTransfer();
                for (let i = 0; i < files.length; i++) dt.items.add(files[i]);
                input.files = dt.files;
                uploadFiles(input);
            }
        }
    });
}

// ═══════════════════════════════════════════════════════════════
// QUICK STATS BAR
// ═══════════════════════════════════════════════════════════════

function updateStatsBar() {
    const bar = document.getElementById('kc-stats-bar');
    if (!bar) return;

    const stats = [
        { icon: '📇', label: t('knowledge.tab_contacts').replace(/📇\s*/, ''), value: allContacts.length, tab: 'contacts' },
        { icon: '📂', label: t('knowledge.tab_files').replace(/📂\s*/, ''), value: allFiles.length, tab: 'files' },
        { icon: '📱', label: t('knowledge.tab_devices').replace(/📱\s*/, ''), value: allDevices.length, tab: 'devices' },
        { icon: '🔑', label: t('knowledge.tab_credentials').replace(/🔑\s*/, ''), value: allCredentials.length, tab: 'credentials' },
    ];

    if (typeof allAppointments !== 'undefined') {
        stats.push({ icon: '📅', label: t('knowledge.tab_appointments').replace(/📅\s*/, ''), value: allAppointments.length, tab: 'appointments' });
    }
    if (typeof allTodos !== 'undefined') {
        stats.push({ icon: '✅', label: t('knowledge.tab_todos').replace(/✅\s*/, ''), value: allTodos.length, tab: 'todos' });
    }

    bar.innerHTML = stats.map(s => `
        <button type="button" class="kc-stat-chip" onclick="switchKCTab('${s.tab}')" title="${esc(s.label)}">
            <span class="kc-stat-chip-icon">${s.icon}</span>
            <span class="kc-stat-chip-value kc-count-anim" data-target="${s.value}">0</span>
            <span>${esc(s.label)}</span>
        </button>
    `).join('');

    bar.classList.add('is-active');
    animateCountUp(bar);
}

function animateCountUp(container) {
    const els = container.querySelectorAll('.kc-count-anim');
    els.forEach(el => {
        const target = parseInt(el.dataset.target) || 0;
        if (target === 0) { el.textContent = '0'; return; }
        const duration = 600;
        const start = performance.now();
        function step(now) {
            const elapsed = now - start;
            const progress = Math.min(elapsed / duration, 1);
            const eased = 1 - Math.pow(1 - progress, 3);
            el.textContent = Math.round(eased * target);
            if (progress < 1) requestAnimationFrame(step);
        }
        requestAnimationFrame(step);
    });
}

// ═══════════════════════════════════════════════════════════════
// FOCUS MANAGEMENT
// ═══════════════════════════════════════════════════════════════

const _origCloseModal = typeof closeModal === 'function' ? closeModal : null;
if (_origCloseModal) {
    window.closeModal = function(id) {
        _origCloseModal(id);
        if (lastFocusedTrigger) {
            lastFocusedTrigger.focus();
            lastFocusedTrigger = null;
        }
    };
}
