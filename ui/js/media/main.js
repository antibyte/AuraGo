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

function mediaStatusNode(icon, message, title, action) {
    const wrap = document.createElement('div');
    wrap.className = 'empty-state pw-empty-state gallery-empty';
    wrap.innerHTML = '<div class="empty-icon pw-empty-icon gallery-empty-icon">' + (icon ? pwIcon(icon) : '') + '</div>' +
        (title ? '<h3>' + escapeHtml(title) + '</h3>' : '') +
        '<p>' + escapeHtml(message) + '</p>';
    if (action && action.label) {
        const btn = document.createElement('button');
        btn.type = 'button';
        btn.className = 'btn-gallery-action';
        btn.textContent = action.label;
        if (typeof action.onClick === 'function') btn.addEventListener('click', action.onClick);
        wrap.appendChild(btn);
    }
    return wrap;
}

/**
 * Skeleton placeholders shown while a tab loads. `variant` picks the shape:
 * 'tile' for the image grid, 'card' for audio/video cards, 'list' for documents.
 */
function mediaSkeletonGrid(count, variant) {
    const wrap = document.createElement('div');
    wrap.className = 'media-skeleton-grid' + (variant === 'list' ? ' media-skeleton-list' : '');
    wrap.setAttribute('role', 'status');
    wrap.setAttribute('aria-label', t('common.loading'));
    for (let i = 0; i < count; i++) {
        const card = document.createElement('div');
        card.className = 'media-skeleton';
        if (variant === 'tile') {
            const tile = document.createElement('div');
            tile.className = 'media-skeleton-tile';
            card.appendChild(tile);
        }
        const body = document.createElement('div');
        body.className = 'media-skeleton-body';
        const line = document.createElement('div');
        line.className = 'media-skeleton-line';
        const short = document.createElement('div');
        short.className = 'media-skeleton-line is-short';
        body.appendChild(line);
        body.appendChild(short);
        card.appendChild(body);
        wrap.appendChild(card);
    }
    return wrap;
}

/** True while a search term or the provider filter narrows the result set. */
function hasActiveMediaFilter() {
    if (getSearchQuery()) return true;
    const provider = document.getElementById('gallery-provider-filter');
    return !!(provider && provider.value);
}

/** "Nothing matched" state with a one-click reset of search and filter. */
function mediaNoResultsNode(icon) {
    return mediaStatusNode(icon || 'search', t('media.no_results'), t('media.no_results_title'), {
        label: t('media.clear_filters'),
        onClick: clearMediaFilters
    });
}

/** Clears the shared search box and the provider filter, then reloads the tab. */
function clearMediaFilters() {
    const search = document.getElementById('media-search');
    if (search) search.value = '';
    const gallerySearch = document.getElementById('gallery-search');
    if (gallerySearch) gallerySearch.value = '';
    const provider = document.getElementById('gallery-provider-filter');
    if (provider) provider.value = '';
    updateMediaSearchClear();
    clearAllMediaSelections();
    reloadCurrentMediaTab();
}

/** Reloads the active tab from its first page. */
function reloadCurrentMediaTab() {
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
}

/** Localized "Page 2 of 5 · 120 items" label for the pagers. */
function mediaPageInfo(page, pages, total) {
    const template = t('media.page_info');
    if (!template || template === 'media.page_info') {
        return page + ' / ' + pages + ' (' + total + ')';
    }
    return String(template)
        .replace('{page}', String(page))
        .replace('{pages}', String(pages))
        .replace('{total}', String(total));
}

/** The inline clear button only shows while the search box holds a term. */
function updateMediaSearchClear() {
    const clearBtn = document.getElementById('media-search-clear');
    if (!clearBtn) return;
    const search = document.getElementById('media-search');
    clearBtn.hidden = !(search && search.value);
}

function lastMediaOffset(total) {
    return Math.max(0, (Math.ceil(total / MEDIA_LIMIT) - 1) * MEDIA_LIMIT);
}

// ── Video tab state ──────────────────────────────────────────────────────────
let videoItems = [];
let videoOffset = 0;
let videoTotal = 0;
let isLoadingVideos = false;
// Ids whose player is already open, so a re-render keeps the player.
const activatedVideoCards = new Set();

// ── Document tab state ───────────────────────────────────────────────────────
let docItems = [];
let docOffset = 0;
let docTotal = 0;
let currentDocModalId = null;
let isLoadingDocs = false;

// ── Init ─────────────────────────────────────────────────────────────────────
/** Fills static [data-pw-icon] placeholders with the shared inline icons. */
function hydratePwIcons(root) {
    (root || document).querySelectorAll('[data-pw-icon]').forEach(function (el) {
        el.innerHTML = pwIcon(el.getAttribute('data-pw-icon'));
    });
}

document.addEventListener('DOMContentLoaded', function () {
    hydratePwIcons();
    // Wire search to current tab
    document.getElementById('media-search').addEventListener('input', function () {
        clearTimeout(mediaSearchDebounce);
        // Keep gallery-search in sync for the Images tab (gallery.js reads from it)
        const gallerySearchEl = document.getElementById('gallery-search');
        if (gallerySearchEl) gallerySearchEl.value = this.value;
        updateMediaSearchClear();
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

    // One-click reset of the shared search box.
    const searchClearBtn = document.getElementById('media-search-clear');
    if (searchClearBtn) {
        searchClearBtn.addEventListener('click', function () {
            const search = document.getElementById('media-search');
            if (search) {
                search.value = '';
                search.focus();
            }
            const gallerySearchEl = document.getElementById('gallery-search');
            if (gallerySearchEl) gallerySearchEl.value = '';
            updateMediaSearchClear();
            clearAllMediaSelections();
            reloadCurrentMediaTab();
        });
    }
    updateMediaSearchClear();

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
    bindMediaChrome();

    // Delegated change listener for document selection checkboxes so diff-rendered
    // rows do not need to be re-wired after every render.
    const docList = document.getElementById('doc-list');
    if (docList) {
        docList.addEventListener('change', function (event) {
            const input = event.target.closest('.media-select-check[data-tab="documents"]');
            if (!input) return;
            event.stopPropagation();
            toggleMediaItemSelection('documents', input.dataset.selectionKey, { id: parseInt(input.dataset.id, 10) }, input.checked);
            rerenderCurrentMediaTab();
        });
    }
});

const MEDIA_PAGE_ACTIONS = {
    'gallery-first': galleryFirst,
    'gallery-prev': galleryPrev,
    'gallery-next': galleryNext,
    'gallery-last': galleryLast,
    'audio-first': audioFirst,
    'audio-prev': audioPrev,
    'audio-next': audioNext,
    'audio-last': audioLast,
    'video-first': videoFirst,
    'video-prev': videoPrev,
    'video-next': videoNext,
    'video-last': videoLast,
    'doc-first': docFirst,
    'doc-prev': docPrev,
    'doc-next': docNext,
    'doc-last': docLast,
};

function bindMediaChrome() {
    document.querySelectorAll('[data-media-tab]').forEach((btn) => {
        btn.addEventListener('click', () => switchTab(btn.dataset.mediaTab));
    });

    const bulkActions = {
        'toggle-selection': toggleMediaSelectionMode,
        'select-visible': selectVisibleMediaItems,
        'clear-selection': clearCurrentMediaSelection,
        'delete-selected': deleteSelectedMediaItems,
    };
    document.querySelectorAll('[data-media-action]').forEach((btn) => {
        btn.addEventListener('click', () => {
            const handler = bulkActions[btn.dataset.mediaAction];
            if (typeof handler === 'function') handler();
        });
    });

    document.querySelectorAll('[data-media-page]').forEach((btn) => {
        btn.addEventListener('click', () => {
            const handler = MEDIA_PAGE_ACTIONS[btn.dataset.mediaPage];
            if (typeof handler === 'function') handler();
        });
    });
}

// ── Tab switching ─────────────────────────────────────────────────────────────
const MEDIA_TABS_ORDER = ['images', 'audio', 'videos', 'documents'];

document.addEventListener('keydown', function (e) {
    // Only handle arrow keys when no modal is open and no input focused
    const audioModal = document.getElementById('audio-modal');
    if (audioModal && audioModal.classList.contains('active')) return;
    const active = document.activeElement;
    if (active && (active.tagName === 'INPUT' || active.tagName === 'TEXTAREA' || active.tagName === 'SELECT')) return;

    const tablist = document.querySelector('.media-tabs');
    if (!tablist) return;
    // ARIA tablist pattern: arrows drive the tabs while the tab bar owns the
    // focus (or while nothing else is focused); elsewhere the keys stay free.
    const focusInTablist = tablist.contains(active);
    const focusOnPage = !active || active === document.body || active === document.documentElement;
    if (!focusInTablist && !focusOnPage) return;

    const idx = MEDIA_TABS_ORDER.indexOf(currentTab);
    if (idx < 0) return;
    let next = null;
    if (e.key === 'ArrowRight') {
        next = MEDIA_TABS_ORDER[(idx + 1) % MEDIA_TABS_ORDER.length];
    } else if (e.key === 'ArrowLeft') {
        next = MEDIA_TABS_ORDER[(idx - 1 + MEDIA_TABS_ORDER.length) % MEDIA_TABS_ORDER.length];
    } else if (e.key === 'Home') {
        next = MEDIA_TABS_ORDER[0];
    } else if (e.key === 'End') {
        next = MEDIA_TABS_ORDER[MEDIA_TABS_ORDER.length - 1];
    }
    if (!next) return;

    switchTab(next);
    const tabEl = document.getElementById('tab-' + next);
    if (tabEl) tabEl.focus();
    e.preventDefault();
});

function switchTab(tab) {
    if (tab !== currentTab) {
        clearAllMediaSelections();
    }
    currentTab = tab;

    document.querySelectorAll('.media-tab, .pw-tab').forEach(function (btn) {
        btn.classList.remove('active');
        btn.setAttribute('aria-selected', 'false');
        btn.setAttribute('tabindex', '-1');
    });
    document.querySelectorAll('.media-panel').forEach(function (panel) {
        panel.classList.remove('active');
    });

    const activeTab = document.getElementById('tab-' + tab);
    const activePanel = document.getElementById('panel-' + tab);
    if (activeTab) {
        activeTab.classList.add('active');
        activeTab.setAttribute('aria-selected', 'true');
        // Roving tabindex: only the selected tab stays in the tab order.
        activeTab.setAttribute('tabindex', '0');
    }
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
    grid.replaceChildren(mediaSkeletonGrid(6, 'card'));

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
            grid.replaceChildren(mediaStatusNode('alert', data.message || t('common.error')));
            return;
        }

        audioItems = data.items || [];
        audioTotal = data.total || 0;

        if (audioItems.length === 0) {
            grid.replaceChildren(hasActiveMediaFilter()
                ? mediaNoResultsNode('music')
                : mediaStatusNode('music', t('media.empty_audio'), t('media.empty_audio_title')));
            document.getElementById('audio-pagination').classList.add('is-hidden');
            return;
        }

        renderAudioGrid(audioItems);
        updateAudioPagination();
    } catch (e) {
        grid.replaceChildren(mediaStatusNode('alert', e.message || t('common.error')));
    } finally {
        isLoadingAudio = false;
    }
}

function audioCardSnapshot(item) {
    const selectionKey = mediaSelectionKey('audio', item);
    return [
        item.id,
        item.description || '',
        item.filename || '',
        item.format || '',
        item.media_type || '',
        item.created_at || '',
        item.web_path || '',
        mediaSelectionMode ? '1' : '0',
        isMediaItemSelected('audio', selectionKey) ? '1' : '0'
    ].join('|');
}

function renderAudioCard(item) {
    const title = item.description || item.filename || t('media.audio_type_audio');
    const fmt = (item.format || '').toUpperCase();
    const date = item.created_at ? new Date(item.created_at).toLocaleDateString() : '';
    const typeIconMap = { tts: 'tts', music: 'music', audio: 'music' };
    const typeIcon = pwIcon(typeIconMap[item.media_type] || 'music');
    const typeLbl = ({ tts: t('media.audio_type_tts'), music: t('media.audio_type_music'), audio: t('media.audio_type_audio') })[item.media_type] || item.media_type;
    const audioPath = item.web_path || (item.filename ? '/files/audio/' + item.filename : '');
    const hasFile = !!audioPath;
    const selectionKey = mediaSelectionKey('audio', item);
    const selected = isMediaItemSelected('audio', selectionKey);

    const card = document.createElement('div');
    card.className = 'media-audio-card' + (hasFile ? '' : ' media-audio-card--unavailable') + (selected ? ' media-card-selected' : '');
    card.dataset.snapshot = audioCardSnapshot(item);
    if (mediaSelectionMode) {
        card.appendChild(createMediaSelectionCheckbox('audio', selectionKey, { id: item.id }));
    }

    // Title
    const titleEl = document.createElement('div');
    titleEl.className = 'media-audio-card-title';
    titleEl.innerHTML = typeIcon + ' ' + escapeHtml(title);
    if (!hasFile) {
        const warn = document.createElement('span');
        warn.className = 'media-inline-warning-icon';
        warn.innerHTML = ' ' + pwIcon('warning');
        warn.title = t('media.file_not_available');
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
        dlBtn.innerHTML = pwIcon('download') + ' ' + escapeHtml(t('gallery.download'));
        actionsEl.appendChild(dlBtn);

        const delBtn = document.createElement('button');
        delBtn.className = 'btn-gallery-action btn-danger';
        delBtn.innerHTML = pwIcon('trash') + ' ' + escapeHtml(t('gallery.delete'));
        delBtn.addEventListener('click', (function (id) {
            return function () { deleteAudioItem(id); };
        }(item.id)));
        actionsEl.appendChild(delBtn);

        card.appendChild(actionsEl);
    } else {
        const unavailEl = document.createElement('div');
        unavailEl.className = 'audio-error media-audio-unavailable';
        unavailEl.innerHTML = pwIcon('warning') + ' ' + escapeHtml(t('media.file_not_available'));
        card.appendChild(unavailEl);
    }

    return card;
}

function shouldUpdateAudioCard(item, el) {
    return el.dataset.snapshot !== audioCardSnapshot(item);
}

function renderAudioGrid(items) {
    const grid = document.getElementById('audio-grid');
    if (window.AuraDiff) {
        window.AuraDiff.render(grid, items, {
            keyFn: function (item) { return String(item.id); },
            renderFn: renderAudioCard,
            shouldUpdate: shouldUpdateAudioCard
        });
    } else {
        grid.replaceChildren();
        items.forEach(function (item) { grid.appendChild(renderAudioCard(item)); });
    }
}

function updateAudioPagination() {
    const pag = document.getElementById('audio-pagination');
    if (audioTotal <= MEDIA_LIMIT) { pag.classList.add('is-hidden'); return; }
    pag.classList.remove('is-hidden');
    const isFirstPage = audioOffset === 0;
    const isLastPage = audioOffset + MEDIA_LIMIT >= audioTotal;
    document.getElementById('audio-first').disabled = isFirstPage;
    document.getElementById('audio-prev').disabled = isFirstPage;
    document.getElementById('audio-next').disabled = isLastPage;
    document.getElementById('audio-last').disabled = isLastPage;
    const page = Math.floor(audioOffset / MEDIA_LIMIT) + 1;
    const pages = Math.ceil(audioTotal / MEDIA_LIMIT);
    document.getElementById('audio-page-info').textContent = mediaPageInfo(page, pages, audioTotal);
}

function audioFirst() { clearCurrentMediaSelection(false); audioOffset = 0; loadAudio(); }
function audioPrev() { clearCurrentMediaSelection(false); audioOffset = Math.max(0, audioOffset - MEDIA_LIMIT); loadAudio(); }
function audioNext() { clearCurrentMediaSelection(false); audioOffset += MEDIA_LIMIT; loadAudio(); }
function audioLast() { clearCurrentMediaSelection(false); audioOffset = lastMediaOffset(audioTotal); loadAudio(); }

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

    const body = document.getElementById('audio-modal-body');
    body.innerHTML = '';

    document.getElementById('audio-modal-title').textContent =
        item.description || item.filename || t('media.audio_type_audio');

    const audioPath = item.web_path || (item.filename ? '/files/audio/' + item.filename : '');
    if (audioPath) {
        const player = new ChatAudioPlayer(audioPath);
        // Hide the player's built-in download button — the modal footer button serves this role
        // and retains the media library's original filename.
        const playerDlBtn = player.element.querySelector('.audio-download-btn');
        if (playerDlBtn) playerDlBtn.classList.add('is-hidden');
        body.appendChild(player.element);
    } else {
        const unavailEl = document.createElement('div');
        unavailEl.className = 'audio-error media-audio-unavailable';
            unavailEl.innerHTML = pwIcon('warning') + ' ' + escapeHtml(t('media.file_not_available'));
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

    openModal('audio-modal', { onClose: resetAudioModal });
}

function closeAudioModal() {
    closeModal('audio-modal');
}

// Runs on every close path (X, Esc, backdrop, delete).
function resetAudioModal() {
    currentAudioModalId = null;
    // Stop any playing audio when the dialog closes
    const player = document.getElementById('audio-modal').querySelector('audio');
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
    const bulkActions = document.getElementById('media-bulk-actions');
    const count = selectedMediaCount();

    // The bulk actions only exist while selecting, so the default toolbar
    // carries search, filter and the entry point alone.
    if (bulkActions) bulkActions.classList.toggle('is-hidden', !mediaSelectionMode);
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
    // A fresh page of videos starts with fresh posters.
    activatedVideoCards.clear();
    const grid = document.getElementById('video-grid');
    grid.replaceChildren(mediaSkeletonGrid(6, 'card'));

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
            grid.replaceChildren(mediaStatusNode('alert', data.message || t('common.error')));
            return;
        }

        videoItems = data.items || [];
        videoTotal = data.total || 0;

        if (videoItems.length === 0) {
            grid.replaceChildren(hasActiveMediaFilter()
                ? mediaNoResultsNode('video')
                : mediaStatusNode('video', t('media.empty_videos'), t('media.empty_videos_title')));
            document.getElementById('video-pagination').classList.add('is-hidden');
            return;
        }

        renderVideoGrid(videoItems);
        updateVideoPagination();
    } catch (e) {
        grid.replaceChildren(mediaStatusNode('alert', e.message || t('common.error')));
    } finally {
        isLoadingVideos = false;
    }
}

function videoCardSnapshot(item) {
    const selectionKey = mediaSelectionKey('videos', item);
    return [
        item.id,
        item.description || '',
        item.prompt || '',
        item.filename || '',
        item.format || '',
        item.duration_ms || '',
        item.created_at || '',
        item.web_path || '',
        mediaSelectionMode ? '1' : '0',
        isMediaItemSelected('videos', selectionKey) ? '1' : '0'
    ].join('|');
}

function renderVideoCard(item) {
    const title = item.description || item.prompt || item.filename || t('media.video_type_video');
    const fmt = (item.format || fileExtension(item.filename) || '').toUpperCase();
    const date = item.created_at ? new Date(item.created_at).toLocaleDateString() : '';
    const durationLabel = formatVideoDuration(item.duration_ms);
    const videoPath = item.web_path || (item.filename ? '/files/generated_videos/' + item.filename : '');
    const hasFile = !!videoPath;
    const selectionKey = mediaSelectionKey('videos', item);
    const selected = isMediaItemSelected('videos', selectionKey);

    const card = document.createElement('div');
    card.className = 'media-video-card' + (hasFile ? '' : ' media-video-card--unavailable') + (selected ? ' media-card-selected' : '');
    card.dataset.snapshot = videoCardSnapshot(item);
    if (mediaSelectionMode) {
        card.appendChild(createMediaSelectionCheckbox('videos', selectionKey, { id: item.id }));
    }

    const titleEl = document.createElement('div');
    titleEl.className = 'media-video-card-title';
    titleEl.innerHTML = pwIcon('video') + ' ' + escapeHtml(title);
    if (!hasFile) {
        const warn = document.createElement('span');
        warn.className = 'media-inline-warning-icon';
        warn.innerHTML = ' ' + pwIcon('warning');
        warn.title = t('media.file_not_available');
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
        // A poster tile stands in for the player: the video element (and its
        // metadata request) is only created once the user asks for it.
        if (activatedVideoCards.has(String(item.id))) {
            // The card is re-rendered when the selection changes; keep the
            // player instead of dropping back to the poster.
            card.appendChild(buildVideoPlayer(videoPath, item.filename || ''));
        } else {
            const poster = document.createElement('button');
            poster.type = 'button';
            poster.className = 'media-video-poster';
            poster.setAttribute('aria-label', t('media.video_play'));
            poster.innerHTML = '<span class="media-video-poster-play">' + pwIcon('play') + '</span>' +
                (durationLabel ? '<span class="media-video-poster-duration">' + escapeHtml(durationLabel) + '</span>' : '');
            poster.addEventListener('click', function () {
                activateVideoCard(poster, videoPath, item.filename || '', item.id);
            });
            card.appendChild(poster);
        }

        const actionsEl = document.createElement('div');
        actionsEl.className = 'media-video-card-actions';

        const dlBtn = document.createElement('a');
        dlBtn.href = videoPath;
        dlBtn.download = item.filename || 'video';
        dlBtn.className = 'btn-gallery-action';
        dlBtn.innerHTML = pwIcon('download') + ' ' + escapeHtml(t('gallery.download'));
        actionsEl.appendChild(dlBtn);

        const delBtn = document.createElement('button');
        delBtn.className = 'btn-gallery-action btn-danger';
        delBtn.innerHTML = pwIcon('trash') + ' ' + escapeHtml(t('gallery.delete'));
        delBtn.addEventListener('click', (function (id) {
            return function () { deleteVideoItem(id); };
        }(item.id)));
        actionsEl.appendChild(delBtn);

        card.appendChild(actionsEl);
    } else {
        const unavailEl = document.createElement('div');
        unavailEl.className = 'audio-error media-video-unavailable';
        unavailEl.innerHTML = pwIcon('warning') + ' ' + escapeHtml(t('media.file_not_available'));
        card.appendChild(unavailEl);
    }

    return card;
}

/** Builds the player element for a video card. */
function buildVideoPlayer(videoPath, filename) {
    const video = document.createElement('video');
    video.className = 'media-video-player';
    video.controls = true;
    video.preload = 'metadata';
    video.playsInline = true;
    const source = document.createElement('source');
    source.src = videoPath;
    source.type = videoMimeType(filename || videoPath);
    video.appendChild(source);
    return video;
}

/** Swaps the poster tile for a real player and starts playback. */
function activateVideoCard(poster, videoPath, filename, id) {
    if (!poster || !poster.parentNode) return;
    activatedVideoCards.add(String(id));
    const video = buildVideoPlayer(videoPath, filename);
    poster.parentNode.replaceChild(video, poster);
    try { video.focus(); } catch (e) { /* focus is best effort */ }
    const started = video.play();
    if (started && typeof started.catch === 'function') started.catch(function () { /* autoplay may be blocked */ });
}

function shouldUpdateVideoCard(item, el) {
    return el.dataset.snapshot !== videoCardSnapshot(item);
}

function renderVideoGrid(items) {
    const grid = document.getElementById('video-grid');
    if (window.AuraDiff) {
        window.AuraDiff.render(grid, items, {
            keyFn: function (item) { return String(item.id); },
            renderFn: renderVideoCard,
            shouldUpdate: shouldUpdateVideoCard
        });
    } else {
        grid.replaceChildren();
        items.forEach(function (item) { grid.appendChild(renderVideoCard(item)); });
    }
}

function updateVideoPagination() {
    const pag = document.getElementById('video-pagination');
    if (videoTotal <= MEDIA_LIMIT) { pag.classList.add('is-hidden'); return; }
    pag.classList.remove('is-hidden');
    const isFirstPage = videoOffset === 0;
    const isLastPage = videoOffset + MEDIA_LIMIT >= videoTotal;
    document.getElementById('video-first').disabled = isFirstPage;
    document.getElementById('video-prev').disabled = isFirstPage;
    document.getElementById('video-next').disabled = isLastPage;
    document.getElementById('video-last').disabled = isLastPage;
    const page = Math.floor(videoOffset / MEDIA_LIMIT) + 1;
    const pages = Math.ceil(videoTotal / MEDIA_LIMIT);
    document.getElementById('video-page-info').textContent = mediaPageInfo(page, pages, videoTotal);
}

function videoFirst() { clearCurrentMediaSelection(false); videoOffset = 0; loadVideos(); }
function videoPrev() { clearCurrentMediaSelection(false); videoOffset = Math.max(0, videoOffset - MEDIA_LIMIT); loadVideos(); }
function videoNext() { clearCurrentMediaSelection(false); videoOffset += MEDIA_LIMIT; loadVideos(); }
function videoLast() { clearCurrentMediaSelection(false); videoOffset = lastMediaOffset(videoTotal); loadVideos(); }

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
    list.replaceChildren(mediaSkeletonGrid(6, 'list'));

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
            list.replaceChildren(mediaStatusNode('alert', data.message || t('common.error')));
            return;
        }

        docItems = data.items || [];
        docTotal = data.total || 0;

        if (docItems.length === 0) {
            list.replaceChildren(hasActiveMediaFilter()
                ? mediaNoResultsNode('doc')
                : mediaStatusNode('doc', t('media.empty_documents'), t('media.empty_documents_title')));
            document.getElementById('doc-pagination').classList.add('is-hidden');
            return;
        }

        renderDocList(docItems);
        updateDocPagination();
    } catch (e) {
        list.replaceChildren(mediaStatusNode('alert', e.message || t('common.error')));
    } finally {
        isLoadingDocs = false;
    }
}

function docFormatIconMedia(fmt) {
    switch ((fmt || '').toLowerCase()) {
        case 'pdf': case 'docx': case 'doc': case 'pptx': case 'ppt': case 'txt': return 'doc';
        case 'xlsx': case 'xls': case 'csv': return 'table';
        case 'md': return 'note';
        case 'json': case 'xml': return 'file-code';
        case 'html': case 'htm': return 'globe';
        default: return 'paperclip';
    }
}

function isInlineDocFmt(fmt) {
    return ['pdf', 'txt', 'md', 'csv', 'json', 'xml', 'html', 'htm'].indexOf((fmt || '').toLowerCase()) >= 0;
}

function renderDocItemHTML(item) {
    const title = escapeHtml(item.description || item.filename || t('media.tab_documents'));
    // Fall back to file extension when format field is empty (agent may not always set it)
    const fmt = item.format || (item.filename ? item.filename.split('.').pop() : '') || '';
    const icon = pwIcon(docFormatIconMedia(fmt));
    const fmtLabel = escapeHtml(fmt.toUpperCase());
    const date = item.created_at ? new Date(item.created_at).toLocaleDateString() : '';
    const webPath = item.web_path || ('/files/documents/' + item.filename);
    const previewHref = isInlineDocFmt(fmt) ? escapeHtml(webPath + '?inline=1') : '';
    const dlHref = escapeHtml(webPath);
    const filename = escapeHtml(item.filename || 'document');
    const selectionKey = mediaSelectionKey('documents', item);
    const selectedClass = mediaSelectionMode && isMediaItemSelected('documents', selectionKey) ? ' media-card-selected' : '';

    let html = '<div class="media-doc-row' + selectedClass + '" data-selection-mode="' + (mediaSelectionMode ? '1' : '0') + '" data-selected="' + (selectedClass ? '1' : '0') + '">';
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
        html += '<a href="' + previewHref + '" target="_blank" title="' + escapeHtml(t('media.doc_open')) + '" aria-label="' + escapeHtml(t('media.doc_open')) + '">' + pwIcon('search') + '</a>';
    }
    html += '<a href="' + dlHref + '" download="' + filename + '" title="' + escapeHtml(t('gallery.download')) + '" aria-label="' + escapeHtml(t('gallery.download')) + '">' + pwIcon('download') + '</a>';
    html += '<button class="btn-danger" onclick="docDelete(' + item.id + ')" title="' + escapeHtml(t('gallery.delete')) + '" aria-label="' + escapeHtml(t('gallery.delete')) + '">' + pwIcon('trash') + '</button>';
    html += '</div>';
    html += '</div>';
    return html;
}

function shouldUpdateDocItem(item, el) {
    return el.dataset.diffHtml !== renderDocItemHTML(item);
}

function renderDocList(items) {
    const list = document.getElementById('doc-list');
    if (window.AuraDiff) {
        window.AuraDiff.render(list, items, {
            keyFn: function (item) { return String(item.id); },
            renderFn: function (item) {
                var wrap = document.createElement('div');
                var html = renderDocItemHTML(item);
                wrap.innerHTML = html;
                var el = wrap.firstElementChild;
                if (el) el.dataset.diffHtml = html;
                return el;
            },
            shouldUpdate: shouldUpdateDocItem
        });
    } else {
        list.innerHTML = items.map(renderDocItemHTML).join('');
    }
}

function updateDocPagination() {
    const pag = document.getElementById('doc-pagination');
    if (docTotal <= MEDIA_LIMIT) { pag.classList.add('is-hidden'); return; }
    pag.classList.remove('is-hidden');
    const isFirstPage = docOffset === 0;
    const isLastPage = docOffset + MEDIA_LIMIT >= docTotal;
    document.getElementById('doc-first').disabled = isFirstPage;
    document.getElementById('doc-prev').disabled = isFirstPage;
    document.getElementById('doc-next').disabled = isLastPage;
    document.getElementById('doc-last').disabled = isLastPage;
    const page = Math.floor(docOffset / MEDIA_LIMIT) + 1;
    const pages = Math.ceil(docTotal / MEDIA_LIMIT);
    document.getElementById('doc-page-info').textContent = mediaPageInfo(page, pages, docTotal);
}

function docFirst() { clearCurrentMediaSelection(false); docOffset = 0; loadDocuments(); }
function docPrev() { clearCurrentMediaSelection(false); docOffset = Math.max(0, docOffset - MEDIA_LIMIT); loadDocuments(); }
function docNext() { clearCurrentMediaSelection(false); docOffset += MEDIA_LIMIT; loadDocuments(); }
function docLast() { clearCurrentMediaSelection(false); docOffset = lastMediaOffset(docTotal); loadDocuments(); }

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
