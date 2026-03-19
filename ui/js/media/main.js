// js/media/main.js — Media View page logic (Images / Audio / Documents tabs)
// The gallery.js is also loaded and handles the Images tab via its own functions.

let currentTab = 'images';
let mediaSearchDebounce;

// ── Audio tab state ──────────────────────────────────────────────────────────
let audioItems = [];
let audioOffset = 0;
let audioTotal = 0;
let currentAudioModalId = null;
let isLoadingAudio = false;
const MEDIA_LIMIT = 30;

// ── Document tab state ───────────────────────────────────────────────────────
let docItems = [];
let docOffset = 0;
let docTotal = 0;
let currentDocModalId = null;
let isLoadingDocs = false;

// ── Init ─────────────────────────────────────────────────────────────────────
document.addEventListener('DOMContentLoaded', function () {
    // Wire search to current tab
    document.getElementById('media-search').addEventListener('input', function () {
        clearTimeout(mediaSearchDebounce);
        // Keep gallery-search in sync for the Images tab (gallery.js reads from it)
        const gallerySearchEl = document.getElementById('gallery-search');
        if (gallerySearchEl) gallerySearchEl.value = this.value;
        mediaSearchDebounce = setTimeout(function () {
            if (currentTab === 'images') {
                galleryOffset = 0;
                loadGallery();
            } else if (currentTab === 'audio') {
                audioOffset = 0;
                loadAudio();
            } else if (currentTab === 'documents') {
                docOffset = 0;
                loadDocuments();
            }
        }, 400);
    });

    // Override gallery search source so it reads from media-search too
    const gallerySearchEl = document.getElementById('gallery-search');
    if (gallerySearchEl) {
        // Keep gallery-search hidden/synced with the unified search bar
        gallerySearchEl.style.display = 'none';
    }
});

// ── Tab switching ─────────────────────────────────────────────────────────────
const MEDIA_TABS_ORDER = ['images', 'audio', 'documents'];

document.addEventListener('keydown', function (e) {
    // Only handle arrow keys when no modal is open and no input focused
    if (document.getElementById('audio-modal').style.display === 'flex') return;
    if (document.activeElement && (document.activeElement.tagName === 'INPUT' || document.activeElement.tagName === 'TEXTAREA')) return;
    if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') {
        const idx = MEDIA_TABS_ORDER.indexOf(currentTab);
        if (idx < 0) return;
        const next = e.key === 'ArrowRight'
            ? MEDIA_TABS_ORDER[(idx + 1) % MEDIA_TABS_ORDER.length]
            : MEDIA_TABS_ORDER[(idx - 1 + MEDIA_TABS_ORDER.length) % MEDIA_TABS_ORDER.length];
        switchTab(next);
        const tabEl = document.getElementById('tab-' + next);
        if (tabEl) tabEl.focus();
        e.preventDefault();
    }
});

function switchTab(tab) {
    currentTab = tab;

    document.querySelectorAll('.media-tab').forEach(function (btn) {
        btn.classList.remove('active');
        btn.setAttribute('aria-selected', 'false');
    });
    document.querySelectorAll('.media-panel').forEach(function (panel) {
        panel.classList.remove('active');
    });

    const activeTab = document.getElementById('tab-' + tab);
    const activePanel = document.getElementById('panel-' + tab);
    if (activeTab) { activeTab.classList.add('active'); activeTab.setAttribute('aria-selected', 'true'); }
    if (activePanel) { activePanel.classList.add('active'); }

    if (tab === 'images') {
        loadGallery();
    } else if (tab === 'audio') {
        if (audioItems.length === 0) loadAudio();
    } else if (tab === 'documents') {
        if (docItems.length === 0) loadDocuments();
    }
}

// ── Helper: read unified search value ────────────────────────────────────────
function getSearchQuery() {
    return (document.getElementById('media-search').value || '').trim();
}

// ── Audio tab ─────────────────────────────────────────────────────────────────
async function loadAudio() {
    if (isLoadingAudio) return;
    isLoadingAudio = true;
    const grid = document.getElementById('audio-grid');
    grid.innerHTML = '<div class="gallery-loading">' + t('gallery.loading') + '</div>';

    const params = new URLSearchParams({
        type: 'audio',
        limit: MEDIA_LIMIT,
        offset: audioOffset,
    });
    const q = getSearchQuery();
    if (q) params.set('q', q);

    try {
        const resp = await fetch('/api/media?' + params.toString());
        const data = await resp.json();

        if (data.status !== 'ok') {
            grid.innerHTML = '<div class="gallery-empty"><div class="gallery-empty-icon">⚠️</div>' + escapeHtml(data.message || 'Error') + '</div>';
            return;
        }

        audioItems = data.items || [];
        audioTotal = data.total || 0;

        if (audioItems.length === 0) {
            grid.innerHTML = '<div class="gallery-empty"><div class="gallery-empty-icon">🎵</div><div>' + t('media.empty_audio') + '</div></div>';
            document.getElementById('audio-pagination').style.display = 'none';
            return;
        }

        renderAudioGrid(audioItems);
        updateAudioPagination();
    } catch (e) {
        grid.innerHTML = '<div class="gallery-empty"><div class="gallery-empty-icon">⚠️</div>' + escapeHtml(e.message) + '</div>';
    } finally {
        isLoadingAudio = false;
    }
}

function renderAudioGrid(items) {
    const grid = document.getElementById('audio-grid');
    let html = '';
    items.forEach(function (item) {
        const title = escapeHtml(item.description || item.filename || 'Audio');
        const fmt = escapeHtml((item.format || '').toUpperCase());
        const date = item.created_at ? new Date(item.created_at).toLocaleDateString() : '';
        html += '<div class="media-audio-card" onclick="openAudioModal(' + item.id + ')">';
        html += '<div class="media-audio-card-title">🎵 ' + title + '</div>';
        html += '<div class="media-audio-card-meta"><span>' + fmt + '</span><span>' + escapeHtml(date) + '</span></div>';
        html += '</div>';
    });
    grid.innerHTML = html;
}

function updateAudioPagination() {
    const pag = document.getElementById('audio-pagination');
    if (audioTotal <= MEDIA_LIMIT) { pag.style.display = 'none'; return; }
    pag.style.display = 'flex';
    document.getElementById('audio-prev').disabled = audioOffset === 0;
    document.getElementById('audio-next').disabled = audioOffset + MEDIA_LIMIT >= audioTotal;
    const page = Math.floor(audioOffset / MEDIA_LIMIT) + 1;
    const pages = Math.ceil(audioTotal / MEDIA_LIMIT);
    document.getElementById('audio-page-info').textContent = page + ' / ' + pages + ' (' + audioTotal + ')';
}

function audioPrev() { audioOffset = Math.max(0, audioOffset - MEDIA_LIMIT); loadAudio(); }
function audioNext() { audioOffset += MEDIA_LIMIT; loadAudio(); }

function openAudioModal(id) {
    const item = audioItems.find(function (i) { return i.id === id; });
    if (!item) return;
    currentAudioModalId = id;

    const modal = document.getElementById('audio-modal');
    const body = document.getElementById('audio-modal-body');
    body.innerHTML = '';

    const title = item.description || item.filename || 'Audio';
    const titleEl = document.createElement('div');
    titleEl.className = 'media-doc-row-title';
    titleEl.style.marginBottom = '0.75rem';
    titleEl.style.fontSize = '1rem';
    titleEl.textContent = title;
    body.appendChild(titleEl);

    const player = new ChatAudioPlayer(item.web_path || ('/files/audio/' + item.filename));
    body.appendChild(player.element);

    const metaEl = document.createElement('div');
    metaEl.className = 'gallery-card-meta';
    metaEl.style.marginTop = '0.75rem';
    metaEl.style.fontSize = '0.8rem';
    const date = item.created_at ? new Date(item.created_at).toLocaleString() : '';
    metaEl.innerHTML = '<span>' + escapeHtml((item.format || '').toUpperCase()) + '</span><span>' + escapeHtml(date) + '</span>';
    body.appendChild(metaEl);

    const dlHref = item.web_path || ('/files/audio/' + item.filename);
    document.getElementById('audio-modal-download').href = dlHref;
    document.getElementById('audio-modal-download').download = item.filename || 'audio';

    modal.style.display = 'flex';
}

function closeAudioModal(event) {
    if (event && event.target !== document.getElementById('audio-modal')) return;
    const modal = document.getElementById('audio-modal');
    modal.style.display = 'none';
    currentAudioModalId = null;
    // Stop any playing audio when modal closes
    const player = modal.querySelector('audio');
    if (player) { player.pause(); player.currentTime = 0; }
}

async function audioDeleteCurrent() {
    if (currentAudioModalId === null) return;
    if (!confirm(t('gallery.confirm_delete'))) return;
    try {
        const resp = await fetch('/api/media/' + currentAudioModalId, { method: 'DELETE' });
        const data = await resp.json();
        if (data.status === 'ok') {
            closeAudioModal();
            audioItems = [];
            audioOffset = 0;
            loadAudio();
        } else {
            alert(data.message || 'Delete failed');
        }
    } catch (e) { alert(e.message); }
}

// ── Documents tab ─────────────────────────────────────────────────────────────
async function loadDocuments() {
    if (isLoadingDocs) return;
    isLoadingDocs = true;
    const list = document.getElementById('doc-list');
    list.innerHTML = '<div class="gallery-loading">' + t('gallery.loading') + '</div>';

    const params = new URLSearchParams({
        type: 'document',
        limit: MEDIA_LIMIT,
        offset: docOffset,
    });
    const q = getSearchQuery();
    if (q) params.set('q', q);

    try {
        const resp = await fetch('/api/media?' + params.toString());
        const data = await resp.json();

        if (data.status !== 'ok') {
            list.innerHTML = '<div class="gallery-empty"><div class="gallery-empty-icon">⚠️</div>' + escapeHtml(data.message || 'Error') + '</div>';
            return;
        }

        docItems = data.items || [];
        docTotal = data.total || 0;

        if (docItems.length === 0) {
            list.innerHTML = '<div class="gallery-empty"><div class="gallery-empty-icon">📄</div><div>' + t('media.empty_documents') + '</div></div>';
            document.getElementById('doc-pagination').style.display = 'none';
            return;
        }

        renderDocList(docItems);
        updateDocPagination();
    } catch (e) {
        list.innerHTML = '<div class="gallery-empty"><div class="gallery-empty-icon">⚠️</div>' + escapeHtml(e.message) + '</div>';
    } finally {
        isLoadingDocs = false;
    }
}

function docFormatIconMedia(fmt) {
    switch ((fmt || '').toLowerCase()) {
        case 'pdf': return '📄';
        case 'docx': case 'doc': return '📝';
        case 'xlsx': case 'xls': return '📊';
        case 'pptx': case 'ppt': return '📑';
        case 'csv': return '📋';
        case 'md': return '📓';
        case 'txt': return '📃';
        case 'json': return '🔧';
        case 'xml': return '🗂️';
        case 'html': case 'htm': return '🌐';
        default: return '📎';
    }
}

function isInlineDocFmt(fmt) {
    return ['pdf', 'txt', 'md', 'csv', 'json', 'xml', 'html', 'htm'].indexOf((fmt || '').toLowerCase()) >= 0;
}

function renderDocList(items) {
    const list = document.getElementById('doc-list');
    let html = '';
    items.forEach(function (item) {
        const title = escapeHtml(item.description || item.filename || 'Document');
        const fmt = item.format || '';
        const icon = docFormatIconMedia(fmt);
        const fmtLabel = escapeHtml(fmt.toUpperCase());
        const date = item.created_at ? new Date(item.created_at).toLocaleDateString() : '';
        const webPath = item.web_path || ('/files/documents/' + item.filename);
        const previewHref = isInlineDocFmt(fmt) ? escapeHtml(webPath + '?inline=1') : '';
        const dlHref = escapeHtml(webPath);
        const filename = escapeHtml(item.filename || 'document');

        html += '<div class="media-doc-row">';
        html += '<div class="media-doc-row-icon">' + icon + '</div>';
        html += '<div class="media-doc-row-info">';
        html += '<div class="media-doc-row-title">' + title + '</div>';
        html += '<div class="media-doc-row-meta">' + fmtLabel + (date ? ' · ' + escapeHtml(date) : '') + '</div>';
        html += '</div>';
        html += '<div class="media-doc-row-actions">';
        if (previewHref) {
            html += '<a href="' + previewHref + '" target="_blank" title="' + t('media.doc_open') + '">🔍</a>';
        }
        html += '<a href="' + dlHref + '" download="' + filename + '" title="' + t('gallery.download') + '">⬇</a>';
        html += '<button class="btn-danger" onclick="docDelete(' + item.id + ')" title="' + t('gallery.delete') + '">🗑</button>';
        html += '</div>';
        html += '</div>';
    });
    list.innerHTML = html;
}

function updateDocPagination() {
    const pag = document.getElementById('doc-pagination');
    if (docTotal <= MEDIA_LIMIT) { pag.style.display = 'none'; return; }
    pag.style.display = 'flex';
    document.getElementById('doc-prev').disabled = docOffset === 0;
    document.getElementById('doc-next').disabled = docOffset + MEDIA_LIMIT >= docTotal;
    const page = Math.floor(docOffset / MEDIA_LIMIT) + 1;
    const pages = Math.ceil(docTotal / MEDIA_LIMIT);
    document.getElementById('doc-page-info').textContent = page + ' / ' + pages + ' (' + docTotal + ')';
}

function docPrev() { docOffset = Math.max(0, docOffset - MEDIA_LIMIT); loadDocuments(); }
function docNext() { docOffset += MEDIA_LIMIT; loadDocuments(); }

async function docDelete(id) {
    if (!confirm(t('gallery.confirm_delete'))) return;
    try {
        const resp = await fetch('/api/media/' + id, { method: 'DELETE' });
        const data = await resp.json();
        if (data.status === 'ok') {
            docItems = [];
            docOffset = 0;
            loadDocuments();
        } else {
            alert(data.message || 'Delete failed');
        }
    } catch (e) { alert(e.message); }
}

// ── Helper same as gallery ────────────────────────────────────────────────────
function escapeHtml(str) {
    if (!str) return '';
    return String(str).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}
