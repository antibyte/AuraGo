/**
 * Gallery markup builders. Every function returns an HTML string and receives a
 * view helper object `v` = { t, esc, iconMarkup, readonly, tabs, tileSizes }
 * plus the state it needs. No DOM mutation or event wiring happens here.
 */
(function () {
    'use strict';

    const ICON = 'vd-gallery-action-icon';

    function shellHTML(v, state) {
        const { t, esc, iconMarkup, readonly, tabs, tileSizes } = v;
        return `<div class="vd-gallery" data-gallery-root data-tab="${esc(state.tab)}" data-tile-index="${state.tileIndex}" style="--vd-gallery-tile:${tileSizes[state.tileIndex]}px">
            <div class="vd-toolbar vd-gallery-toolbar">
                <div class="vd-gallery-tabs" role="tablist" aria-label="${esc(t('desktop.gallery_library'))}">
                    ${tabs.map(tab => `<button type="button" class="vd-gallery-tab${tab === state.tab ? ' is-active' : ''}" role="tab" aria-selected="${tab === state.tab}" data-gallery-tab="${tab}">
                        ${iconMarkup(tab === 'Photos' ? 'image' : 'video', tab === 'Photos' ? 'P' : 'V', ICON, 15)}
                        <span>${esc(t(tab === 'Photos' ? 'desktop.gallery_photos' : 'desktop.gallery_videos'))}</span>
                        <span class="vd-gallery-tab-count" data-gallery-tab-count="${tab}"></span>
                    </button>`).join('')}
                </div>
                <label class="vd-gallery-search">
                    ${iconMarkup('search', '?', ICON, 14)}
                    <input type="search" data-gallery-search placeholder="${esc(t('desktop.gallery_search_placeholder'))}" aria-label="${esc(t('desktop.gallery_search_placeholder'))}" autocomplete="off" spellcheck="false" inputmode="search" enterkeyhint="search">
                    <button type="button" class="vd-gallery-search-clear" data-gallery-search-clear hidden aria-label="${esc(t('desktop.gallery_search_clear'))}">${iconMarkup('x', '×', ICON, 12)}</button>
                </label>
                <div class="vd-gallery-toolbar-actions">
                    <button type="button" class="vd-gallery-tool" data-gallery-select-mode aria-pressed="false" title="${esc(t('desktop.gallery_select'))}">
                        ${iconMarkup('check-square', '☑', ICON, 15)}<span class="vd-gallery-tool-label">${esc(t('desktop.gallery_select'))}</span>
                    </button>
                    <button type="button" class="vd-gallery-tool vd-gallery-tool-icon" data-gallery-sort-menu title="${esc(t('desktop.gallery_sort'))}" aria-label="${esc(t('desktop.gallery_sort'))}" aria-haspopup="menu">
                        ${iconMarkup('sort', '⇅', ICON, 15)}
                    </button>
                    <button type="button" class="vd-gallery-tool vd-gallery-tool-icon" data-gallery-info-toggle aria-pressed="${state.infoOpen}" title="${esc(t('desktop.gallery_info'))}" aria-label="${esc(t('desktop.gallery_info'))}">
                        ${iconMarkup('info', 'i', ICON, 15)}
                    </button>
                    <button type="button" class="vd-gallery-tool vd-gallery-tool-icon" data-gallery-reload title="${esc(t('desktop.gallery_refresh'))}" aria-label="${esc(t('desktop.gallery_refresh'))}">
                        ${iconMarkup('refresh', '↻', ICON, 15)}
                    </button>
                </div>
            </div>
            <div class="vd-gallery-body${state.infoOpen ? ' has-info' : ''}">
                <div class="vd-gallery-scroll vd-scroll" data-gallery-scroll tabindex="0" role="listbox" aria-multiselectable="true" aria-label="${esc(t('desktop.app_gallery'))}">
                    <div class="vd-gallery-pill" data-gallery-new hidden>
                        <span data-gallery-new-label></span>
                        <button type="button" class="vd-gallery-pill-button" data-gallery-show-new>${esc(t('desktop.gallery_show_new'))}</button>
                    </div>
                    <div class="vd-gallery-sections" data-gallery-grid></div>
                    <div class="vd-gallery-footer" data-gallery-footer>
                        <button type="button" class="vd-button vd-gallery-more" data-gallery-more hidden>${esc(t('desktop.gallery_load_more'))}</button>
                        <div class="vd-gallery-sentinel" data-gallery-sentinel aria-hidden="true"></div>
                    </div>
                </div>
                <aside class="vd-gallery-info" data-gallery-info-panel aria-label="${esc(t('desktop.gallery_details'))}"${state.infoOpen ? '' : ' hidden'}></aside>
            </div>
            <div class="vd-gallery-statusbar">
                <div class="vd-gallery-status" data-gallery-status aria-live="polite"></div>
                <div class="vd-gallery-selection" data-gallery-selection hidden>
                    <span class="vd-gallery-selection-count" data-gallery-selection-count></span>
                    <button type="button" class="vd-gallery-tool" data-gallery-bulk-download title="${esc(t('desktop.gallery_download'))}">${iconMarkup('gallery-action-download', '↓', ICON, 14)}<span class="vd-gallery-tool-label">${esc(t('desktop.gallery_download'))}</span></button>
                    ${readonly ? '' : `<button type="button" class="vd-gallery-tool vd-gallery-tool-danger" data-gallery-bulk-delete title="${esc(t('desktop.gallery_delete'))}">${iconMarkup('gallery-action-delete', '🗑', ICON, 14)}<span class="vd-gallery-tool-label">${esc(t('desktop.gallery_delete'))}</span></button>`}
                    <button type="button" class="vd-gallery-tool" data-gallery-select-none title="${esc(t('desktop.gallery_select_none'))}">${iconMarkup('x', '×', ICON, 13)}<span class="vd-gallery-tool-label">${esc(t('desktop.gallery_select_none'))}</span></button>
                </div>
                <div class="vd-gallery-zoom" role="group" aria-label="${esc(t('desktop.gallery_tile_size'))}">
                    <button type="button" class="vd-gallery-zoom-button" data-gallery-zoom-out title="${esc(t('desktop.gallery_tile_smaller'))}" aria-label="${esc(t('desktop.gallery_tile_smaller'))}">${iconMarkup('minus', '−', ICON, 12)}</button>
                    <input type="range" class="vd-gallery-zoom-range" data-gallery-zoom min="0" max="${tileSizes.length - 1}" step="1" value="${state.tileIndex}" aria-label="${esc(t('desktop.gallery_tile_size'))}">
                    <button type="button" class="vd-gallery-zoom-button" data-gallery-zoom-in title="${esc(t('desktop.gallery_tile_larger'))}" aria-label="${esc(t('desktop.gallery_tile_larger'))}">${iconMarkup('plus', '+', ICON, 12)}</button>
                </div>
            </div>
            <div class="vd-gallery-progress" data-gallery-progress hidden role="status">
                <span class="vd-gallery-progress-label" data-gallery-progress-label></span>
                <div class="vd-gallery-progress-track"><span class="vd-gallery-progress-bar" data-gallery-progress-bar></span></div>
            </div>
        </div>`;
    }

    function tileHTML(v, item, index, kind, selected, url) {
        const { t, esc, iconMarkup, readonly } = v;
        const media = kind === 'video'
            ? `<video class="vd-gallery-media" muted playsinline preload="none" data-src="${esc(url)}" tabindex="-1" aria-hidden="true"></video>
               <span class="vd-gallery-play" aria-hidden="true">${iconMarkup('play', '▶', ICON, 20)}</span>
               <span class="vd-gallery-badge" data-gallery-duration hidden></span>`
            : kind === 'audio'
                ? `<span class="vd-gallery-audio" aria-hidden="true">${iconMarkup('music', '♪', ICON, 32)}</span>`
                : `<img class="vd-gallery-media" src="${esc(url)}" alt="" loading="lazy" decoding="async" draggable="false">`;
        return `<article class="vd-gallery-card${selected ? ' is-selected' : ''}" role="option" tabindex="-1" aria-selected="${selected}" aria-label="${esc(item.name || '')}" data-gallery-item data-gallery-open data-path="${esc(item.path)}" data-index="${index}" data-kind="${kind}">
            <div class="vd-gallery-thumb">${media}
                <span class="vd-gallery-thumb-fallback" hidden>${iconMarkup(kind === 'video' ? 'video' : 'image', '', ICON, 26)}<span>${esc(t('desktop.gallery_thumbnail_failed'))}</span></span>
            </div>
            <button type="button" class="vd-gallery-check" role="checkbox" aria-checked="${selected}" tabindex="-1" aria-label="${esc(t('desktop.gallery_toggle_select'))}" data-gallery-toggle>${iconMarkup('check', '✓', ICON, 12)}</button>
            <div class="vd-gallery-card-meta">
                <span class="vd-gallery-card-name" data-gallery-name title="${esc(item.name || '')}">${esc(item.name || '')}</span>
                <div class="vd-gallery-actions">
                    <button type="button" class="vd-gallery-action" tabindex="-1" data-gallery-download title="${esc(t('desktop.gallery_download'))}" aria-label="${esc(t('desktop.gallery_download'))}">${iconMarkup('gallery-action-download', 'D', ICON, 16)}</button>
                    ${readonly ? '' : `<button type="button" class="vd-gallery-action" tabindex="-1" data-gallery-rename title="${esc(t('desktop.gallery_rename'))}" aria-label="${esc(t('desktop.gallery_rename'))}">${iconMarkup('gallery-action-edit', 'E', ICON, 16)}</button>
                    <button type="button" class="vd-gallery-action vd-gallery-action-danger" tabindex="-1" data-gallery-delete title="${esc(t('desktop.gallery_delete'))}" aria-label="${esc(t('desktop.gallery_delete'))}">${iconMarkup('gallery-action-delete', 'X', ICON, 16)}</button>`}
                </div>
            </div>
        </article>`;
    }

    function sectionHTML(v, group, count) {
        const { t, esc } = v;
        const countText = typeof v.countLabel === 'function' ? v.countLabel('desktop.gallery_item_count', count) : t('desktop.gallery_item_count', { count });
        const header = group.key === '__all__' ? '' : `<header class="vd-gallery-section-header"><h3>${esc(group.label)}</h3><span class="vd-gallery-section-count">${esc(countText)}</span></header>`;
        return `<section class="vd-gallery-section" data-group="${esc(group.key)}" role="group" aria-label="${esc(group.label)}">${header}<div class="vd-gallery-grid"></div></section>`;
    }

    function skeletonHTML(count) {
        let tiles = '';
        for (let index = 0; index < count; index += 1) tiles += '<div class="vd-gallery-skeleton" aria-hidden="true"></div>';
        return `<section class="vd-gallery-section" aria-hidden="true"><div class="vd-gallery-grid">${tiles}</div></section>`;
    }

    function errorHTML(v, err) {
        const { t, esc, iconMarkup } = v;
        return `<div class="vd-empty vd-gallery-empty vd-gallery-error">
            ${iconMarkup('info', '!', ICON, 30)}
            <strong>${esc(t('desktop.gallery_error_title'))}</strong>
            <p>${esc((err && err.message) || t('desktop.load_failed'))}</p>
            <button type="button" class="vd-button vd-button-primary" data-gallery-retry>${esc(t('desktop.retry'))}</button>
        </div>`;
    }

    function emptyHTML(v, query, tab) {
        const { t, esc, iconMarkup } = v;
        if (query) {
            return `<div class="vd-empty vd-gallery-empty">
                ${iconMarkup('search', '?', ICON, 30)}
                <strong>${esc(t('desktop.gallery_no_results', { query }))}</strong>
                <p>${esc(t('desktop.gallery_no_results_hint'))}</p>
                <button type="button" class="vd-button" data-gallery-search-reset>${esc(t('desktop.gallery_search_clear'))}</button>
            </div>`;
        }
        return `<div class="vd-empty vd-gallery-empty">
            ${iconMarkup(tab === 'Videos' ? 'video' : 'image', '', ICON, 34)}
            <strong>${esc(t(tab === 'Videos' ? 'desktop.gallery_empty_videos' : 'desktop.gallery_empty_photos'))}</strong>
            <p>${esc(t('desktop.gallery_empty_hint'))}</p>
        </div>`;
    }

    function infoMultiHTML(v, count, sizeLabel) {
        const { t, esc, readonly } = v;
        return `<div class="vd-gallery-info-header"><strong>${esc(t('desktop.gallery_selected_count', { count }))}</strong></div>
            <dl class="vd-gallery-info-list">
                <div><dt>${esc(t('desktop.gallery_field_size'))}</dt><dd>${esc(sizeLabel)}</dd></div>
            </dl>
            <div class="vd-gallery-info-actions">
                <button type="button" class="vd-button" data-gallery-bulk-download>${esc(t('desktop.gallery_download'))}</button>
                ${readonly ? '' : `<button type="button" class="vd-button vd-gallery-button-danger" data-gallery-bulk-delete>${esc(t('desktop.gallery_delete'))}</button>`}
            </div>
            ${readonly ? `<p class="vd-gallery-info-hint">${esc(t('desktop.gallery_readonly_hint'))}</p>` : ''}`;
    }

    function infoEmptyHTML(v) {
        const { t, esc, iconMarkup } = v;
        return `<div class="vd-gallery-info-empty">${iconMarkup('info', 'i', ICON, 24)}<p>${esc(t('desktop.gallery_details'))}</p></div>`;
    }

    function infoItemHTML(v, item, kind, rows, previewUrl) {
        const { t, esc, iconMarkup, readonly } = v;
        const preview = kind === 'video'
            ? `<video class="vd-gallery-info-preview" src="${esc(previewUrl)}" muted playsinline preload="metadata"></video>`
            : kind === 'audio'
                ? `<div class="vd-gallery-info-preview vd-gallery-info-audio">${iconMarkup('music', '♪', ICON, 40)}</div>`
                : `<img class="vd-gallery-info-preview" src="${esc(previewUrl)}" alt="" data-gallery-info-image>`;
        return `<div class="vd-gallery-info-media">${preview}</div>
            <div class="vd-gallery-info-header">
                <input class="vd-gallery-info-name" type="text" value="${esc(item.name || '')}" data-gallery-info-name aria-label="${esc(t('desktop.gallery_field_name'))}"${readonly ? ' readonly' : ''} spellcheck="false" autocomplete="off" enterkeyhint="done">
            </div>
            <dl class="vd-gallery-info-list">${rows.map(([label, value]) => `<div><dt>${esc(label)}</dt><dd title="${esc(value)}">${esc(value)}</dd></div>`).join('')}</dl>
            <div class="vd-gallery-info-actions">
                <button type="button" class="vd-button" data-gallery-info-download>${iconMarkup('gallery-action-download', 'D', ICON, 14)}<span>${esc(t('desktop.gallery_download'))}</span></button>
                ${kind === 'image' ? `<button type="button" class="vd-button" data-gallery-info-edit>${iconMarkup('gallery-action-edit', 'E', ICON, 14)}<span>${esc(t('desktop.gallery_edit_pixel'))}</span></button>` : ''}
                <button type="button" class="vd-button" data-gallery-info-reveal>${iconMarkup('folder', '▤', ICON, 14)}<span>${esc(t('desktop.gallery_show_in_files'))}</span></button>
                <button type="button" class="vd-button" data-gallery-info-link>${iconMarkup('link', '⛓', ICON, 14)}<span>${esc(t('desktop.gallery_copy_link'))}</span></button>
                ${readonly ? '' : `<button type="button" class="vd-button vd-gallery-button-danger" data-gallery-info-delete>${iconMarkup('gallery-action-delete', 'X', ICON, 14)}<span>${esc(t('desktop.gallery_delete'))}</span></button>`}
            </div>
            ${readonly ? `<p class="vd-gallery-info-hint">${esc(t('desktop.gallery_readonly_hint'))}</p>` : ''}`;
    }

    window.GalleryView = {
        shellHTML,
        tileHTML,
        sectionHTML,
        skeletonHTML,
        errorHTML,
        emptyHTML,
        infoMultiHTML,
        infoEmptyHTML,
        infoItemHTML
    };
})();
