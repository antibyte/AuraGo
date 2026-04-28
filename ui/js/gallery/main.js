// js/gallery/main.js — Image Gallery page logic

let galleryImages = [];
let galleryOffset = 0;
let galleryTotal = 0;
const GALLERY_LIMIT = 30;
let currentLightboxId = null;
let currentLightboxSource = '';
let galleryDeleteInFlight = false;

document.addEventListener('DOMContentLoaded', function () {
    loadGallery();

    const searchInput = document.getElementById('gallery-search');
    let debounce;
    if (searchInput) searchInput.addEventListener('input', function () {
        clearTimeout(debounce);
        debounce = setTimeout(function () {
            galleryOffset = 0;
            loadGallery();
        }, 400);
    });

    document.getElementById('gallery-provider-filter').addEventListener('change', function () {
        galleryOffset = 0;
        loadGallery();
    });

    document.addEventListener('keydown', function (e) {
        if (e.key === 'Escape') closeLightbox();
    });
});

async function loadGallery() {
    const grid = document.getElementById('gallery-grid');
    grid.innerHTML = '<div class="gallery-loading">' + t('gallery.loading') + '</div>';

    const q = (document.getElementById('gallery-search') || document.getElementById('media-search') || {value: ''}).value;
    const provider = document.getElementById('gallery-provider-filter').value;

    const params = new URLSearchParams({
        limit: GALLERY_LIMIT,
        offset: galleryOffset
    });
    if (q) params.set('q', q);
    if (provider) params.set('provider', provider);

    try {
        const resp = await fetch('/api/image-gallery?' + params.toString());
        const data = await resp.json();

        if (data.status !== 'ok') {
            grid.innerHTML = '<div class="gallery-empty"><div class="gallery-empty-icon">⚠️</div>' + escapeHtml(data.message || t('common.error')) + '</div>';
            return;
        }

        galleryImages = data.images || [];
        galleryTotal = data.total || 0;

        // Populate provider filter if empty
        populateProviderFilter(galleryImages);

        if (galleryImages.length === 0) {
            grid.innerHTML = '<div class="gallery-empty"><div class="gallery-empty-icon">🎨</div><div>' + t('gallery.empty') + '</div></div>';
            document.getElementById('gallery-pagination').classList.add('is-hidden');
            return;
        }

        renderGrid(galleryImages);
        updatePagination();
    } catch (e) {
        grid.innerHTML = '<div class="gallery-empty"><div class="gallery-empty-icon">⚠️</div>' + escapeHtml(e.message || t('common.error')) + '</div>';
    }
}

function renderGrid(images) {
    const grid = document.getElementById('gallery-grid');
    let html = '';
    const selectionActive = typeof isMediaSelectionModeActive === 'function' && isMediaSelectionModeActive();
    images.forEach(function (img) {
        const webPath = img.web_path || ('/files/generated_images/' + img.filename);
        const promptDisplay = escapeHtml(img.prompt || '').substring(0, 100);
        const date = img.created_at ? new Date(img.created_at).toLocaleDateString() : '';
        const providerBadge = img.provider || '';
        const sourceDB = img.source_db || '';
        const selectionKey = sourceDB + ':' + img.id;
        const selectedClass = selectionActive && typeof isMediaItemSelected === 'function' && isMediaItemSelected('images', selectionKey) ? ' media-card-selected' : '';
        html += '<div class="gallery-card' + selectedClass + '" data-source="' + escapeHtml(sourceDB) + '" data-media-id="' + img.id + '" onclick="handleGalleryCardClick(event, this.dataset.mediaId, this.dataset.source)">';
        if (selectionActive) {
            html += '<label class="media-select-check-wrap" onclick="event.stopPropagation()">';
            html += '<input type="checkbox" class="media-select-check" data-tab="images" data-selection-key="' + escapeHtml(selectionKey) + '" data-id="' + img.id + '" data-source="' + escapeHtml(sourceDB) + '"' + (selectedClass ? ' checked' : '') + ' aria-label="' + escapeHtml(t('media.bulk_select_item')) + '">';
            html += '</label>';
        }
        html += '<img src="' + escapeHtml(webPath) + '" loading="lazy" alt="' + escapeHtml(img.prompt || '') + '">';
        html += '<div class="gallery-card-info">';
        html += '<div class="gallery-card-prompt">' + promptDisplay + '</div>';
        html += '<div class="gallery-card-meta"><span>' + escapeHtml(providerBadge) + '</span><span>' + escapeHtml(date) + '</span></div>';
        html += '</div></div>';
    });
    grid.innerHTML = html;
    if (typeof wireGalleryMediaSelectionChecks === 'function') {
        wireGalleryMediaSelectionChecks(grid);
    }
}

function updatePagination() {
    const pag = document.getElementById('gallery-pagination');
    const prevBtn = document.getElementById('gallery-prev');
    const nextBtn = document.getElementById('gallery-next');
    const info = document.getElementById('gallery-page-info');

    if (galleryTotal <= GALLERY_LIMIT) {
        pag.classList.add('is-hidden');
        return;
    }

    pag.classList.remove('is-hidden');
    prevBtn.disabled = galleryOffset === 0;
    nextBtn.disabled = galleryOffset + GALLERY_LIMIT >= galleryTotal;

    const page = Math.floor(galleryOffset / GALLERY_LIMIT) + 1;
    const pages = Math.ceil(galleryTotal / GALLERY_LIMIT);
    info.textContent = page + ' / ' + pages + ' (' + galleryTotal + ' ' + t('gallery.images') + ')';
}

function galleryPrev() {
    if (typeof clearCurrentMediaSelection === 'function') clearCurrentMediaSelection(false);
    galleryOffset = Math.max(0, galleryOffset - GALLERY_LIMIT);
    loadGallery();
}

function galleryNext() {
    if (typeof clearCurrentMediaSelection === 'function') clearCurrentMediaSelection(false);
    galleryOffset += GALLERY_LIMIT;
    loadGallery();
}

function populateProviderFilter(images) {
    const sel = document.getElementById('gallery-provider-filter');
    if (sel.options.length > 1) return; // Already populated
    const providers = new Set();
    images.forEach(function (img) {
        if (img.provider) providers.add(img.provider);
    });
    providers.forEach(function (p) {
        const opt = document.createElement('option');
        opt.value = p;
        opt.textContent = p;
        sel.appendChild(opt);
    });
}

function findGalleryImage(id, source) {
    const numericID = parseInt(id);
    return galleryImages.find(function (i) {
        return i.id === numericID && (!source || i.source_db === source);
    }) || galleryImages.find(function (i) {
        return i.id === numericID;
    });
}

function openLightbox(id, source = '') {
    const img = findGalleryImage(id, source);
    if (!img) return;

    currentLightboxId = img.id;
    currentLightboxSource = img.source_db || '';
    const webPath = img.web_path || ('/files/generated_images/' + img.filename);

    document.getElementById('lightbox-img').src = webPath;
    document.getElementById('lightbox-download').href = webPath;

    let meta = '';
    meta += '<div><strong>' + t('gallery.prompt') + ':</strong> ' + escapeHtml(img.prompt || '') + '</div>';
    if (img.enhanced_prompt) {
        meta += '<div><strong>' + t('gallery.enhanced_prompt') + ':</strong> ' + escapeHtml(img.enhanced_prompt) + '</div>';
    }
    meta += '<div><strong>' + t('gallery.provider') + ':</strong> ' + escapeHtml(img.provider || '') + ' · <strong>' + t('gallery.model') + ':</strong> ' + escapeHtml(img.model || '') + '</div>';
    meta += '<div><strong>' + t('gallery.size') + ':</strong> ' + escapeHtml(img.size || '') + ' · <strong>' + t('gallery.quality') + ':</strong> ' + escapeHtml(img.quality || '') + '</div>';
    if (img.generation_time_ms) {
        meta += '<div><strong>' + t('gallery.duration') + ':</strong> ' + (img.generation_time_ms / 1000).toFixed(1) + 's</div>';
    }
    if (img.cost_estimate) {
        meta += '<div><strong>' + t('gallery.cost') + ':</strong> $' + img.cost_estimate.toFixed(4) + '</div>';
    }
    if (img.created_at) {
        meta += '<div><strong>' + t('gallery.date') + ':</strong> ' + new Date(img.created_at).toLocaleString() + '</div>';
    }
    document.getElementById('lightbox-meta').innerHTML = meta;

    document.getElementById('lightbox').classList.remove('is-hidden');
}

function closeLightbox(event) {
    if (event && event.target !== document.getElementById('lightbox')) return;
    document.getElementById('lightbox').classList.add('is-hidden');
    currentLightboxId = null;
    currentLightboxSource = '';
}

function handleGalleryCardClick(event, id, source = '') {
    if (typeof handleMediaGalleryCardClick === 'function' && handleMediaGalleryCardClick(event, id, source)) {
        return;
    }
    openLightbox(id, source);
}

async function galleryDeleteCurrent() {
    if (currentLightboxId === null) return;

    const id = currentLightboxId;
    const source = currentLightboxSource;
    const confirmed = await showConfirm(t('common.confirm_title'), t('gallery.confirm_delete'));
    if (!confirmed) return;
    await deleteGalleryImage(id, source);
}

async function deleteGalleryImage(id, source = '') {
    if (galleryDeleteInFlight) return;
    galleryDeleteInFlight = true;
    setGalleryDeleteBusy(true);
    var img = findGalleryImage(id, source);
    var sourceDB = source || (img && img.source_db ? img.source_db : '');

    try {
        var url = '/api/image-gallery/' + id;
        if (sourceDB) url += '?source=' + encodeURIComponent(sourceDB);
        const resp = await fetch(url, { method: 'DELETE' });
        const data = await resp.json();
        if (data.status === 'ok') {
            closeModal('delete-modal');
            closeLightbox();
            loadGallery();
        } else {
            showToast(data.message || t('common.error'), 'error');
        }
    } catch (e) {
        showToast(e.message || t('common.error'), 'error');
    } finally {
        galleryDeleteInFlight = false;
        setGalleryDeleteBusy(false);
    }
}

function setGalleryDeleteBusy(busy) {
    const confirmBtn = document.getElementById('gallery-delete-confirm-btn');
    const lightboxBtn = document.getElementById('lightbox-delete');
    if (confirmBtn) {
        confirmBtn.disabled = busy;
    }
    if (lightboxBtn) {
        lightboxBtn.disabled = busy;
    }
}

function escapeHtml(str) {
    if (!str) return '';
    return String(str).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

// ═══════════════════════════════════════════════════════════════
// DELETE CONFIRMATION HANDLER
// ═══════════════════════════════════════════════════════════════

async function confirmDeleteGallery() {
    const id = document.getElementById('delete-target-id').value;
    const type = document.getElementById('delete-target-type').value;

    if (type !== 'gallery-image' || !id) {
        closeModal('delete-modal');
        return;
    }

    await deleteGalleryImage(id);
}
