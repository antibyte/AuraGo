// js/media/main.js — Media View page logic (Images / Audio / Videos / Documents tabs)
// The gallery.js is also loaded and handles the Images tab via its own functions.

let currentTab = 'images';
let mediaSearchDebounce;
let mediaSelectionMode = false;
const mediaSelections = {
    images: new Map(),
    audio: new Map(),
    videos: new Map(),
    documents: new Map()
};
window.mediaSelectionMode = false;

// ── Audio tab state ──────────────────────────────────────────────────────────
let audioItems = [];
let audioOffset = 0;
let audioTotal = 0;
let currentAudioModalId = null;
let isLoadingAudio = false;
const MEDIA_LIMIT = 30;

// ── Video tab state ──────────────────────────────────────────────────────────
let videoItems = [];
let videoOffset = 0;
let videoTotal = 0;
let isLoadingVideos = false;

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
            clearAllMediaSelections();
            if (currentTab === 'images') {
                galleryOffset = 0;
                loadGallery();
            } else if (currentTab === 'audio') {
                audioOffset = 0;
                loadAudio();
            } else if (currentTab === 'videos') {
                videoOffset = 0;
                loadVideos();
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
        gallerySearchEl.classList.add('is-hidden');
    }

    const providerFilter = document.getElementById('gallery-provider-filter');
    if (providerFilter) {
        providerFilter.addEventListener('change', function () {
            clearAllMediaSelections();
        });
    }

    updateMediaBulkToolbar();
});

// ── Tab switching ─────────────────────────────────────────────────────────────
const MEDIA_TABS_ORDER = ['images', 'audio', 'videos', 'documents'];

document.addEventListener('keydown', function (e) {
    // Only handle arrow keys when no modal is open and no input focused
    if (!document.getElementById('audio-modal').classList.contains('is-hidden')) return;
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
    if (tab !== currentTab) {
        clearAllMediaSelections();
    }
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
        else renderAudioGrid(audioItems);
    } else if (tab === 'videos') {
        if (videoItems.length === 0) loadVideos();
        else renderVideoGrid(videoItems);
    } else if (tab === 'documents') {
        if (docItems.length === 0) loadDocuments();
        else renderDocList(docItems);
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
    grid.innerHTML = '<div class="gallery-loading">' + t('common.loading') + '</div>';

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
            grid.innerHTML = '<div class="gallery-empty"><div class="gallery-empty-icon">⚠️</div>' + escapeHtml(data.message || t('common.error')) + '</div>';
            return;
        }

        audioItems = data.items || [];
        audioTotal = data.total || 0;

        if (audioItems.length === 0) {
            grid.innerHTML = '<div class="gallery-empty"><div class="gallery-empty-icon">🎵</div><div>' + t('media.empty_audio') + '</div></div>';
            document.getElementById('audio-pagination').classList.add('is-hidden');
            return;
        }

        renderAudioGrid(audioItems);
        updateAudioPagination();
    } catch (e) {
        grid.innerHTML = '<div class="gallery-empty"><div class="gallery-empty-icon">⚠️</div>' + escapeHtml(e.message || t('common.error')) + '</div>';
    } finally {
        isLoadingAudio = false;
    }
}

function renderAudioGrid(items) {
    const grid = document.getElementById('audio-grid');
    grid.innerHTML = '';

    items.forEach(function (item) {
        const title = item.description || item.filename || t('media.audio_type_audio');
        const fmt = (item.format || '').toUpperCase();
        const date = item.created_at ? new Date(item.created_at).toLocaleDateString() : '';
        const typeIconMap = { tts: '🗣️', music: '🎶', audio: '🎵' };
        const typeIcon = typeIconMap[item.media_type] || '🎵';
        const typeLbl = ({ tts: t('media.audio_type_tts'), music: t('media.audio_type_music'), audio: t('media.audio_type_audio') })[item.media_type] || item.media_type;
        const audioPath = item.web_path || (item.filename ? '/files/audio/' + item.filename : '');
        const hasFile = !!audioPath;

        const card = document.createElement('div');
        card.className = 'media-audio-card' + (hasFile ? '' : ' media-audio-card--unavailable');
        const selectionKey = mediaSelectionKey('audio', item);
        if (mediaSelectionMode) {
            if (isMediaItemSelected('audio', selectionKey)) card.classList.add('media-card-selected');
            card.appendChild(createMediaSelectionCheckbox('audio', selectionKey, { id: item.id }));
        }

        // Title
        const titleEl = document.createElement('div');
        titleEl.className = 'media-audio-card-title';
        titleEl.textContent = typeIcon + ' ' + title;
        if (!hasFile) {
            const warn = document.createElement('span');
            warn.className = 'media-inline-warning-icon';
            warn.textContent = ' ⚠️';
            titleEl.appendChild(warn);
        }
        card.appendChild(titleEl);

        // Meta
        const metaEl = document.createElement('div');
        metaEl.className = 'media-audio-card-meta';
        const badge = document.createElement('span');
        badge.className = 'media-type-badge';
        badge.textContent = typeLbl;
        metaEl.appendChild(badge);
        const fmtSpan = document.createElement('span');
        fmtSpan.textContent = fmt;
        metaEl.appendChild(fmtSpan);
        const dateSpan = document.createElement('span');
        dateSpan.textContent = date;
        metaEl.appendChild(dateSpan);
        card.appendChild(metaEl);

        if (hasFile) {
            // Inline player
            const player = new ChatAudioPlayer(audioPath);
            const playerDlBtn = player.element.querySelector('.audio-download-btn');
            if (playerDlBtn) playerDlBtn.classList.add('is-hidden');
            card.appendChild(player.element);

            // Actions: download + delete
            const actionsEl = document.createElement('div');
            actionsEl.className = 'media-audio-card-actions';

            const dlBtn = document.createElement('a');
            dlBtn.href = audioPath;
            dlBtn.download = item.filename || 'audio';
            dlBtn.className = 'btn-gallery-action';
            dlBtn.textContent = '⬇ ' + t('gallery.download');
            actionsEl.appendChild(dlBtn);

            const delBtn = document.createElement('button');
            delBtn.className = 'btn-gallery-action btn-danger';
            delBtn.textContent = '🗑 ' + t('gallery.delete');
            delBtn.addEventListener('click', (function (id) {
                return function () { deleteAudioItem(id); };
            }(item.id)));
            actionsEl.appendChild(delBtn);

            card.appendChild(actionsEl);
        } else {
            const unavailEl = document.createElement('div');
            unavailEl.className = 'audio-error media-audio-unavailable';
            unavailEl.textContent = '⚠️ ' + t('media.file_not_available');
            card.appendChild(unavailEl);
        }

        grid.appendChild(card);
    });
}

function updateAudioPagination() {
    const pag = document.getElementById('audio-pagination');
    if (audioTotal <= MEDIA_LIMIT) { pag.classList.add('is-hidden'); return; }
    pag.classList.remove('is-hidden');
    document.getElementById('audio-prev').disabled = audioOffset === 0;
    document.getElementById('audio-next').disabled = audioOffset + MEDIA_LIMIT >= audioTotal;
    const page = Math.floor(audioOffset / MEDIA_LIMIT) + 1;
    const pages = Math.ceil(audioTotal / MEDIA_LIMIT);
    document.getElementById('audio-page-info').textContent = page + ' / ' + pages + ' (' + audioTotal + ')';
}

function audioPrev() { clearCurrentMediaSelection(false); audioOffset = Math.max(0, audioOffset - MEDIA_LIMIT); loadAudio(); }
function audioNext() { clearCurrentMediaSelection(false); audioOffset += MEDIA_LIMIT; loadAudio(); }

async function deleteAudioItem(id) {
    const confirmed = await showConfirm(t('common.confirm_title'), t('gallery.confirm_delete'));
    if (!confirmed) return;
    try {
        const resp = await fetch('/api/media/' + id, { method: 'DELETE' });
        const data = await resp.json();
        if (data.status === 'ok') {
            audioItems = [];
            audioOffset = 0;
            loadAudio();
        } else {
            await showAlert(t('common.error'), data.message || t('common.error'));
        }
    } catch (e) { await showAlert(t('common.error'), e.message || t('common.error')); }
}

function openAudioModal(id) {
    const item = audioItems.find(function (i) { return i.id === id; });
    if (!item) return;
    currentAudioModalId = id;

    const modal = document.getElementById('audio-modal');
    const body = document.getElementById('audio-modal-body');
    body.innerHTML = '';

    const title = item.description || item.filename || t('media.audio_type_audio');
    const titleEl = document.createElement('div');
    titleEl.className = 'media-doc-row-title';
    titleEl.classList.add('media-modal-title');
    titleEl.textContent = title;
    body.appendChild(titleEl);

    const audioPath = item.web_path || (item.filename ? '/files/audio/' + item.filename : '');
    if (audioPath) {
        const player = new ChatAudioPlayer(audioPath);
        // Hide the player's built-in download button — the modal footer button serves this role
        // and uses the correct filename instead of the generic 'audio-message.mp3'
        const playerDlBtn = player.element.querySelector('.audio-download-btn');
        if (playerDlBtn) playerDlBtn.classList.add('is-hidden');
        body.appendChild(player.element);
    } else {
        const unavailEl = document.createElement('div');
        unavailEl.className = 'audio-error media-audio-unavailable';
            unavailEl.textContent = '⚠️ ' + t('media.file_not_available');
        body.appendChild(unavailEl);
    }

    const metaEl = document.createElement('div');
    metaEl.className = 'gallery-card-meta';
    metaEl.classList.add('media-modal-meta');
    const date = item.created_at ? new Date(item.created_at).toLocaleString() : '';
    metaEl.innerHTML = '<span>' + escapeHtml((item.format || '').toUpperCase()) + '</span><span>' + escapeHtml(date) + '</span>';
    body.appendChild(metaEl);

    const dlHref = item.web_path || (item.filename ? '/files/audio/' + item.filename : '#');
    document.getElementById('audio-modal-download').href = dlHref;
    document.getElementById('audio-modal-download').download = item.filename || 'audio';
    document.getElementById('audio-modal-download').classList.toggle('is-hidden', !audioPath);

    modal.classList.remove('is-hidden');
}

function closeAudioModal(event) {
    if (event && event.target !== document.getElementById('audio-modal')) return;
    const modal = document.getElementById('audio-modal');
    modal.classList.add('is-hidden');
    currentAudioModalId = null;
    // Stop any playing audio when modal closes
    const player = modal.querySelector('audio');
    if (player) { player.pause(); player.currentTime = 0; }
}

async function audioDeleteCurrent() {
    if (currentAudioModalId === null) return;
    const confirmed = await showConfirm(t('common.confirm_title'), t('gallery.confirm_delete'));
    if (!confirmed) return;
    try {
        const resp = await fetch('/api/media/' + currentAudioModalId, { method: 'DELETE' });
        const data = await resp.json();
        if (data.status === 'ok') {
            closeAudioModal();
            audioItems = [];
            audioOffset = 0;
            loadAudio();
        } else {
            await showAlert(t('common.error'), data.message || t('common.error'));
        }
    } catch (e) { await showAlert(t('common.error'), e.message || t('common.error')); }
}

function isMediaSelectionModeActive() {
    return mediaSelectionMode;
}

function mediaSelectionKey(tab, item) {
    if (!item) return '';
    if (tab === 'images') {
        return String(item.source || item.source_db || '') + ':' + String(item.id || '');
    }
    return String(item.id || '');
}

function currentMediaSelection() {
    return mediaSelections[currentTab] || new Map();
}

function isMediaItemSelected(tab, key) {
    const map = mediaSelections[tab];
    return !!(map && map.has(String(key)));
}

function selectedMediaCount(tab) {
    const map = mediaSelections[tab || currentTab];
    return map ? map.size : 0;
}

function mediaTranslateCount(key, count) {
    const template = t(key);
    return String(template || '').replace('{count}', String(count));
}

function updateMediaBulkToolbar() {
    window.mediaSelectionMode = mediaSelectionMode;
    document.body.classList.toggle('media-selection-active', mediaSelectionMode);

    const selectBtn = document.getElementById('media-select-mode-btn');
    const selectVisibleBtn = document.getElementById('media-select-visible-btn');
    const clearBtn = document.getElementById('media-clear-selection-btn');
    const deleteBtn = document.getElementById('media-delete-selected-btn');
    const countEl = document.getElementById('media-selected-count');
    const count = selectedMediaCount();

    if (selectBtn) {
        selectBtn.textContent = mediaSelectionMode ? t('media.bulk_select_done') : t('media.bulk_select');
        selectBtn.classList.toggle('active', mediaSelectionMode);
    }
    if (selectVisibleBtn) selectVisibleBtn.disabled = !mediaSelectionMode;
    if (clearBtn) clearBtn.disabled = count === 0;
    if (deleteBtn) deleteBtn.disabled = count === 0;
    if (countEl) {
        countEl.textContent = count > 0
            ? mediaTranslateCount('media.bulk_selected_count', count)
            : t('media.bulk_selected_none');
    }
}

function rerenderCurrentMediaTab() {
    if (currentTab === 'images' && typeof renderGrid === 'function') {
        renderGrid(galleryImages || []);
    } else if (currentTab === 'audio') {
        renderAudioGrid(audioItems || []);
    } else if (currentTab === 'videos') {
        renderVideoGrid(videoItems || []);
    } else if (currentTab === 'documents') {
        renderDocList(docItems || []);
    }
}

function toggleMediaSelectionMode() {
    mediaSelectionMode = !mediaSelectionMode;
    if (!mediaSelectionMode) clearAllMediaSelections(false);
    updateMediaBulkToolbar();
    rerenderCurrentMediaTab();
}

function clearCurrentMediaSelection(rerender = true) {
    currentMediaSelection().clear();
    updateMediaBulkToolbar();
    if (rerender) rerenderCurrentMediaTab();
}

function clearAllMediaSelections(rerender = false) {
    Object.keys(mediaSelections).forEach(function (tab) {
        mediaSelections[tab].clear();
    });
    updateMediaBulkToolbar();
    if (rerender) rerenderCurrentMediaTab();
}

function getCurrentVisibleMediaItems() {
    if (currentTab === 'images') {
        return (galleryImages || []).map(function (item) {
            const payload = { id: item.id, source: item.source_db || '' };
            return { key: mediaSelectionKey('images', payload), payload: payload };
        });
    }
    if (currentTab === 'audio') {
        return (audioItems || []).map(function (item) {
            return { key: mediaSelectionKey('audio', item), payload: { id: item.id } };
        });
    }
    if (currentTab === 'videos') {
        return (videoItems || []).map(function (item) {
            return { key: mediaSelectionKey('videos', item), payload: { id: item.id } };
        });
    }
    if (currentTab === 'documents') {
        return (docItems || []).map(function (item) {
            return { key: mediaSelectionKey('documents', item), payload: { id: item.id } };
        });
    }
    return [];
}

function selectVisibleMediaItems() {
    if (!mediaSelectionMode) {
        mediaSelectionMode = true;
    }
    const map = currentMediaSelection();
    getCurrentVisibleMediaItems().forEach(function (entry) {
        if (entry.key) map.set(String(entry.key), entry.payload);
    });
    updateMediaBulkToolbar();
    rerenderCurrentMediaTab();
}

async function reloadCurrentMediaTabAfterDelete() {
    if (currentTab === 'images') {
        await loadGallery();
        if ((galleryImages || []).length === 0 && galleryOffset > 0) {
            galleryOffset = Math.max(0, galleryOffset - GALLERY_LIMIT);
            await loadGallery();
        }
    } else if (currentTab === 'audio') {
        await loadAudio();
        if ((audioItems || []).length === 0 && audioOffset > 0) {
            audioOffset = Math.max(0, audioOffset - MEDIA_LIMIT);
            await loadAudio();
        }
    } else if (currentTab === 'videos') {
        await loadVideos();
        if ((videoItems || []).length === 0 && videoOffset > 0) {
            videoOffset = Math.max(0, videoOffset - MEDIA_LIMIT);
            await loadVideos();
        }
    } else if (currentTab === 'documents') {
        await loadDocuments();
        if ((docItems || []).length === 0 && docOffset > 0) {
            docOffset = Math.max(0, docOffset - MEDIA_LIMIT);
            await loadDocuments();
        }
    }
}

async function deleteSelectedMediaItems() {
    const selection = Array.from(currentMediaSelection().values());
    const count = selection.length;
    if (count === 0) return;

    const confirmed = await showConfirm(t('common.confirm_title'), mediaTranslateCount('media.bulk_confirm_delete', count));
    if (!confirmed) return;

    try {
        let resp;
        if (currentTab === 'images') {
            resp = await fetch('/api/image-gallery/bulk-delete', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ items: selection })
            });
        } else {
            resp = await fetch('/api/media/bulk-delete', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ ids: selection.map(function (item) { return item.id; }) })
            });
        }
        const data = await resp.json();
        if (!resp.ok || data.status === 'error') {
            await showAlert(t('common.error'), data.message || t('common.error'));
            return;
        }

        const deleted = Number(data.deleted || 0);
        const failed = Array.isArray(data.failed) ? data.failed.length : 0;
        clearCurrentMediaSelection(false);
        if (data.status === 'partial') {
            showToast(mediaTranslateCount('media.bulk_partial_deleted', deleted).replace('{failed}', String(failed)), 'warning');
        } else {
            showToast(mediaTranslateCount('media.bulk_deleted', deleted), 'success');
        }
        await reloadCurrentMediaTabAfterDelete();
        updateMediaBulkToolbar();
    } catch (e) {
        await showAlert(t('common.error'), e.message || t('common.error'));
    }
}

function toggleMediaItemSelection(tab, key, payload, checked) {
    const map = mediaSelections[tab];
    if (!map || !key) return;
    if (checked) {
        map.set(String(key), payload);
    } else {
        map.delete(String(key));
    }
    updateMediaBulkToolbar();
}

function createMediaSelectionCheckbox(tab, key, payload) {
    const label = document.createElement('label');
    label.className = 'media-select-check-wrap';
    label.addEventListener('click', function (event) {
        event.stopPropagation();
    });

    const input = document.createElement('input');
    input.type = 'checkbox';
    input.className = 'media-select-check';
    input.checked = isMediaItemSelected(tab, key);
    input.setAttribute('aria-label', t('media.bulk_select_item'));
    input.addEventListener('change', function (event) {
        event.stopPropagation();
        toggleMediaItemSelection(tab, key, payload, input.checked);
        rerenderCurrentMediaTab();
    });
    label.appendChild(input);
    return label;
}

function wireGalleryMediaSelectionChecks(root) {
    root.querySelectorAll('.media-select-check[data-tab="images"]').forEach(function (input) {
        input.addEventListener('click', function (event) {
            event.stopPropagation();
        });
        input.addEventListener('change', function (event) {
            event.stopPropagation();
            const payload = { id: parseInt(input.dataset.id, 10), source: input.dataset.source || '' };
            toggleMediaItemSelection('images', input.dataset.selectionKey, payload, input.checked);
            rerenderCurrentMediaTab();
        });
    });
}

function wireDocumentMediaSelectionChecks(root) {
    root.querySelectorAll('.media-select-check[data-tab="documents"]').forEach(function (input) {
        input.addEventListener('click', function (event) {
            event.stopPropagation();
        });
        input.addEventListener('change', function (event) {
            event.stopPropagation();
            toggleMediaItemSelection('documents', input.dataset.selectionKey, { id: parseInt(input.dataset.id, 10) }, input.checked);
            rerenderCurrentMediaTab();
        });
    });
}

function handleMediaGalleryCardClick(event, id, source) {
    if (!mediaSelectionMode) return false;
    if (event) {
        event.preventDefault();
        event.stopPropagation();
    }
    const payload = { id: parseInt(id, 10), source: source || '' };
    const key = mediaSelectionKey('images', payload);
    const nextChecked = !isMediaItemSelected('images', key);
    toggleMediaItemSelection('images', key, payload, nextChecked);
    rerenderCurrentMediaTab();
    return true;
}

// ── Videos tab ────────────────────────────────────────────────────────────────
async function loadVideos() {
    if (isLoadingVideos) return;
    isLoadingVideos = true;
    const grid = document.getElementById('video-grid');
    grid.innerHTML = '<div class="gallery-loading">' + t('common.loading') + '</div>';

    const params = new URLSearchParams({
        type: 'video',
        limit: MEDIA_LIMIT,
        offset: videoOffset,
    });
    const q = getSearchQuery();
    if (q) params.set('q', q);

    try {
        const resp = await fetch('/api/media?' + params.toString());
        const data = await resp.json();

        if (data.status !== 'ok') {
            grid.innerHTML = '<div class="gallery-empty"><div class="gallery-empty-icon">⚠️</div>' + escapeHtml(data.message || t('common.error')) + '</div>';
            return;
        }

        videoItems = data.items || [];
        videoTotal = data.total || 0;

        if (videoItems.length === 0) {
            grid.innerHTML = '<div class="gallery-empty"><div class="gallery-empty-icon">🎬</div><div>' + t('media.empty_videos') + '</div></div>';
            document.getElementById('video-pagination').classList.add('is-hidden');
            return;
        }

        renderVideoGrid(videoItems);
        updateVideoPagination();
    } catch (e) {
        grid.innerHTML = '<div class="gallery-empty"><div class="gallery-empty-icon">⚠️</div>' + escapeHtml(e.message || t('common.error')) + '</div>';
    } finally {
        isLoadingVideos = false;
    }
}

function renderVideoGrid(items) {
    const grid = document.getElementById('video-grid');
    grid.innerHTML = '';

    items.forEach(function (item) {
        const title = item.description || item.prompt || item.filename || t('media.video_type_video');
        const fmt = (item.format || fileExtension(item.filename) || '').toUpperCase();
        const date = item.created_at ? new Date(item.created_at).toLocaleDateString() : '';
        const durationLabel = formatVideoDuration(item.duration_ms);
        const videoPath = item.web_path || (item.filename ? '/files/generated_videos/' + item.filename : '');
        const hasFile = !!videoPath;

        const card = document.createElement('div');
        card.className = 'media-video-card' + (hasFile ? '' : ' media-video-card--unavailable');
        const selectionKey = mediaSelectionKey('videos', item);
        if (mediaSelectionMode) {
            if (isMediaItemSelected('videos', selectionKey)) card.classList.add('media-card-selected');
            card.appendChild(createMediaSelectionCheckbox('videos', selectionKey, { id: item.id }));
        }

        const titleEl = document.createElement('div');
        titleEl.className = 'media-video-card-title';
        titleEl.textContent = '🎬 ' + title;
        if (!hasFile) {
            const warn = document.createElement('span');
            warn.className = 'media-inline-warning-icon';
            warn.textContent = ' ⚠️';
            titleEl.appendChild(warn);
        }
        card.appendChild(titleEl);

        const metaEl = document.createElement('div');
        metaEl.className = 'media-video-card-meta';
        const badge = document.createElement('span');
        badge.className = 'media-type-badge';
        badge.textContent = t('media.video_type_video');
        metaEl.appendChild(badge);
        [fmt, durationLabel, date].filter(Boolean).forEach(function (value) {
            const span = document.createElement('span');
            span.textContent = value;
            metaEl.appendChild(span);
        });
        card.appendChild(metaEl);

        if (hasFile) {
            const video = document.createElement('video');
            video.className = 'media-video-player';
            video.controls = true;
            video.preload = 'metadata';
            video.playsInline = true;
            const source = document.createElement('source');
            source.src = videoPath;
            source.type = videoMimeType(item.filename || videoPath);
            video.appendChild(source);
            card.appendChild(video);

            const actionsEl = document.createElement('div');
            actionsEl.className = 'media-video-card-actions';

            const dlBtn = document.createElement('a');
            dlBtn.href = videoPath;
            dlBtn.download = item.filename || 'video';
            dlBtn.className = 'btn-gallery-action';
            dlBtn.textContent = '⬇ ' + t('gallery.download');
            actionsEl.appendChild(dlBtn);

            const delBtn = document.createElement('button');
            delBtn.className = 'btn-gallery-action btn-danger';
            delBtn.textContent = '🗑 ' + t('gallery.delete');
            delBtn.addEventListener('click', (function (id) {
                return function () { deleteVideoItem(id); };
            }(item.id)));
            actionsEl.appendChild(delBtn);

            card.appendChild(actionsEl);
        } else {
            const unavailEl = document.createElement('div');
            unavailEl.className = 'audio-error media-video-unavailable';
            unavailEl.textContent = '⚠️ ' + t('media.file_not_available');
            card.appendChild(unavailEl);
        }

        grid.appendChild(card);
    });
}

function updateVideoPagination() {
    const pag = document.getElementById('video-pagination');
    if (videoTotal <= MEDIA_LIMIT) { pag.classList.add('is-hidden'); return; }
    pag.classList.remove('is-hidden');
    document.getElementById('video-prev').disabled = videoOffset === 0;
    document.getElementById('video-next').disabled = videoOffset + MEDIA_LIMIT >= videoTotal;
    const page = Math.floor(videoOffset / MEDIA_LIMIT) + 1;
    const pages = Math.ceil(videoTotal / MEDIA_LIMIT);
    document.getElementById('video-page-info').textContent = page + ' / ' + pages + ' (' + videoTotal + ')';
}

function videoPrev() { clearCurrentMediaSelection(false); videoOffset = Math.max(0, videoOffset - MEDIA_LIMIT); loadVideos(); }
function videoNext() { clearCurrentMediaSelection(false); videoOffset += MEDIA_LIMIT; loadVideos(); }

async function deleteVideoItem(id) {
    const confirmed = await showConfirm(t('common.confirm_title'), t('gallery.confirm_delete'));
    if (!confirmed) return;
    try {
        const resp = await fetch('/api/media/' + id, { method: 'DELETE' });
        const data = await resp.json();
        if (data.status === 'ok') {
            videoItems = [];
            videoOffset = 0;
            loadVideos();
        } else {
            await showAlert(t('common.error'), data.message || t('common.error'));
        }
    } catch (e) { await showAlert(t('common.error'), e.message || t('common.error')); }
}

function fileExtension(filename) {
    const name = String(filename || '');
    const idx = name.lastIndexOf('.');
    return idx >= 0 ? name.slice(idx + 1) : '';
}

function videoMimeType(filename) {
    switch (fileExtension(filename).toLowerCase()) {
        case 'webm': return 'video/webm';
        case 'mov': return 'video/quicktime';
        case 'ogv':
        case 'ogg': return 'video/ogg';
        default: return 'video/mp4';
    }
}

function formatVideoDuration(ms) {
    const total = Number(ms || 0);
    if (!Number.isFinite(total) || total <= 0) return '';
    const seconds = Math.round(total / 1000);
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    if (mins <= 0) return secs + 's';
    return mins + ':' + String(secs).padStart(2, '0');
}

// ── Documents tab ─────────────────────────────────────────────────────────────
async function loadDocuments() {
    if (isLoadingDocs) return;
    isLoadingDocs = true;
    const list = document.getElementById('doc-list');
    list.innerHTML = '<div class="gallery-loading">' + t('common.loading') + '</div>';

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
            list.innerHTML = '<div class="gallery-empty"><div class="gallery-empty-icon">⚠️</div>' + escapeHtml(data.message || t('common.error')) + '</div>';
            return;
        }

        docItems = data.items || [];
        docTotal = data.total || 0;

        if (docItems.length === 0) {
            list.innerHTML = '<div class="gallery-empty"><div class="gallery-empty-icon">📄</div><div>' + t('media.empty_documents') + '</div></div>';
            document.getElementById('doc-pagination').classList.add('is-hidden');
            return;
        }

        renderDocList(docItems);
        updateDocPagination();
    } catch (e) {
        list.innerHTML = '<div class="gallery-empty"><div class="gallery-empty-icon">⚠️</div>' + escapeHtml(e.message || t('common.error')) + '</div>';
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
        const title = escapeHtml(item.description || item.filename || t('media.tab_documents'));
        // Fall back to file extension when format field is empty (agent may not always set it)
        const fmt = item.format || (item.filename ? item.filename.split('.').pop() : '') || '';
        const icon = docFormatIconMedia(fmt);
        const fmtLabel = escapeHtml(fmt.toUpperCase());
        const date = item.created_at ? new Date(item.created_at).toLocaleDateString() : '';
        const webPath = item.web_path || ('/files/documents/' + item.filename);
        const previewHref = isInlineDocFmt(fmt) ? escapeHtml(webPath + '?inline=1') : '';
        const dlHref = escapeHtml(webPath);
        const filename = escapeHtml(item.filename || 'document');
        const selectionKey = mediaSelectionKey('documents', item);
        const selectedClass = mediaSelectionMode && isMediaItemSelected('documents', selectionKey) ? ' media-card-selected' : '';

        html += '<div class="media-doc-row' + selectedClass + '">';
        if (mediaSelectionMode) {
            html += '<label class="media-select-check-wrap" onclick="event.stopPropagation()">';
            html += '<input type="checkbox" class="media-select-check" data-tab="documents" data-selection-key="' + escapeHtml(selectionKey) + '" data-id="' + item.id + '"' + (selectedClass ? ' checked' : '') + ' aria-label="' + escapeHtml(t('media.bulk_select_item')) + '">';
            html += '</label>';
        }
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
    wireDocumentMediaSelectionChecks(list);
}

function updateDocPagination() {
    const pag = document.getElementById('doc-pagination');
    if (docTotal <= MEDIA_LIMIT) { pag.classList.add('is-hidden'); return; }
    pag.classList.remove('is-hidden');
    document.getElementById('doc-prev').disabled = docOffset === 0;
    document.getElementById('doc-next').disabled = docOffset + MEDIA_LIMIT >= docTotal;
    const page = Math.floor(docOffset / MEDIA_LIMIT) + 1;
    const pages = Math.ceil(docTotal / MEDIA_LIMIT);
    document.getElementById('doc-page-info').textContent = page + ' / ' + pages + ' (' + docTotal + ')';
}

function docPrev() { clearCurrentMediaSelection(false); docOffset = Math.max(0, docOffset - MEDIA_LIMIT); loadDocuments(); }
function docNext() { clearCurrentMediaSelection(false); docOffset += MEDIA_LIMIT; loadDocuments(); }

async function docDelete(id) {
    const confirmed = await showConfirm(t('common.confirm_title'), t('gallery.confirm_delete'));
    if (!confirmed) return;
    try {
        const resp = await fetch('/api/media/' + id, { method: 'DELETE' });
        const data = await resp.json();
        if (data.status === 'ok') {
            docItems = [];
            docOffset = 0;
            loadDocuments();
        } else {
            await showAlert(t('common.error'), data.message || t('common.error'));
        }
    } catch (e) { await showAlert(t('common.error'), e.message || t('common.error')); }
}

// ── Helper same as gallery ────────────────────────────────────────────────────
function escapeHtml(str) {
    if (!str) return '';
    return String(str).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}
