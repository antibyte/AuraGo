(function () {
    'use strict';

    const instances = new Map();
    const PAGE_SIZE = 36;
    const esc = value => String(value ?? '').replace(/[&<>"']/g, char => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[char]);
    const fileExt = value => String(value || '').split(/[?#]/, 1)[0].split('.').pop().toLowerCase();
    const isBookPath = path => /\.(epub|pdf|mobi|azw|azw3|fb2(?:\.zip)?|cbz|txt(?:\.gz)?|html?|cbr|rtf|docx|odt|djvu|lit|pdb|lrf)$/i.test(String(path || ''));
    const canReadDirectly = book => /^(epub|pdf|mobi|azw|azw3|fb2(?:\.zip)?|cbz|txt|html?)$/i.test(String(book?.format || fileExt(book?.path || '')));
    const localBookURL = book => '/api/desktop/bookshelf/books/' + encodeURIComponent(book.id) + '/cover?v=' + encodeURIComponent(String(book.revision || 0));
    const defaultPrefs = { fontSize: 18, lineHeight: 1.65, width: 'comfortable', theme: 'sepia', fit: 'page', zoom: null, spread: false, direction: 'ltr', encoding: 'auto' };
    const readerWidth = { narrow: 460, comfortable: 640, wide: 900 };
    const sharedLabels = {
        cancel: 'desktop.cancel', close: 'desktop.close', loading: 'desktop.fm.loading',
        save: 'desktop.save', search: 'desktop.search', upload: 'desktop.fm.upload'
    };
    const appName = context => {
        const key = 'desktop.app_bookshelf';
        const value = context?.t ? context.t(key) : '';
        return value && value !== key ? value : 'Bookshelf';
    };

    function render(host, windowId, context) {
        dispose(windowId);
        const ctx = context || {};
        const abort = new AbortController();
        const api = window.BookshelfAPI.create(ctx, abort.signal);
        const state = {
            host, windowId, ctx, api, abort, disposed: false,
            capabilities: {}, cache: null, sources: [], collections: [], books: [], total: 0,
            selected: null, metadata: null, focusedBookId: '', restoreShelfFocus: false, sourceId: '', collectionId: '',
            query: '', statusFilter: 'all', sort: 'recent', sortExplicit: false, offset: 0, density: 'covers', page: 'library', panel: '',
            busy: false, error: '', notice: '', job: null, jobTimer: 0,
            searchTimer: 0, progressTimer: 0, progressQueue: new Map(),
            progressWrite: Promise.resolve(), progressWriter: null, reader: null, readerToken: 0, readerRevision: '', renditionRevision: '', shelfScrollTop: 0,
            readerData: null, readerPrefs: Object.assign({}, defaultPrefs), readerReady: false, thumbnailObserver: null, thumbnailURLs: new Map(), thumbnailCount: 48,
            toc: [], annotations: [], searchResults: [], details: false, modalResolver: null,
            folderChooser: null, bookLoadGeneration: 0, passwordResolver: null, releaseThumbnails: null
        };
        state.progressWriter = window.BookshelfProgress.createWriter((item, expectedRevision) =>
            request(api.saveProgress, item.bookId, item.progress, expectedRevision));
        instances.set(windowId, state);

        const t = key => {
            const id = 'bookshelf.' + key;
            const value = ctx.t ? ctx.t(id) : id;
            if (value && value !== id) return value;
            const alias = sharedLabels[key];
            const shared = alias && ctx.t ? ctx.t(alias) : '';
            return shared && shared !== alias ? shared : key.replaceAll('_', ' ');
        };
        const readerSettings = settings => Object.assign({}, settings, { width: readerWidth[settings?.width] || 640, zoom: settings?.zoom == null ? null : Number(settings.zoom) / 100 });
        const appDisabled = () => !state.capabilities.enabled;
        const readOnly = () => !state.capabilities.can_write || state.capabilities.readonly || ctx.readonly;
        const request = (fn, ...args) => fn(...args);

        function shell() {
            host.innerHTML = `<div class="bs-app" data-page="library" data-density="covers">
                <header class="bs-toolbar">
                    <div class="bs-brand"><span class="bs-mark" aria-hidden="true">◧</span><strong>${esc(appName(ctx))}</strong></div>
                    <nav class="bs-nav" aria-label="${esc(t('navigation'))}">
                        <button type="button" class="is-active" data-action="library">${esc(t('library'))}</button>
                        <button type="button" data-action="collections">${esc(t('collections'))}</button>
                    </nav>
                    <label class="bs-search"><span aria-hidden="true">⌕</span><input type="search" data-search placeholder="${esc(t('search_placeholder'))}" aria-label="${esc(t('search'))}"></label>
                    <button type="button" class="bs-primary" data-action="add" ${readOnly() ? 'disabled' : ''}><span aria-hidden="true">＋</span>${esc(t('add_books'))}</button>
                    <button type="button" class="bs-source-button" data-action="sources" aria-label="${esc(t('sources'))}"><span class="bs-source-icon" aria-hidden="true">▤</span><span class="bs-source-label">${esc(t('sources'))}</span><span class="bs-source-chevron" aria-hidden="true">⌄</span></button>
                </header>
                <div class="bs-notice" role="status" aria-live="polite" hidden></div>
                <section class="bs-library-view" aria-label="${esc(t('library'))}">
                    <aside class="bs-reading-preview" aria-label="${esc(t('reading_now'))}"></aside>
                    <main class="bs-library">
                        <div class="bs-library-head"><div><span class="bs-eyebrow" data-collection-label>${esc(t('all_books'))}</span><h1 data-library-title>${esc(t('library'))}</h1><p data-library-count></p></div>
                            <div class="bs-library-tools"><label>${esc(t('source'))}<select data-source-filter aria-label="${esc(t('source'))}"><option value="">${esc(t('all_sources'))}</option></select></label>
                            <label>${esc(t('reading_state'))}<select data-status-filter aria-label="${esc(t('reading_state'))}"><option value="all">${esc(t('all_statuses'))}</option><option value="unread">${esc(t('unread'))}</option><option value="reading">${esc(t('reading'))}</option><option value="finished">${esc(t('finished'))}</option></select></label>
                            <label>${esc(t('sort_by'))}<select data-sort aria-label="${esc(t('sort_by'))}"><option value="recent">${esc(t('sort_recent'))}</option><option value="collection" disabled>${esc(t('sort_collection'))}</option><option value="title">${esc(t('sort_title'))}</option><option value="author">${esc(t('sort_author'))}</option><option value="progress">${esc(t('sort_progress'))}</option></select></label>
                            <label class="bs-density-label">${esc(t('view_density'))}<select data-density aria-label="${esc(t('view_density'))}"><option value="covers">${esc(t('covers'))}</option><option value="spines">${esc(t('spines'))}</option><option value="list">${esc(t('list'))}</option></select></label></div></div>
                        <div class="bs-collection-strip" data-collection-strip hidden></div>
                        <div class="bs-shelf-scroll" data-shelf tabindex="0" aria-label="${esc(t('library'))}"></div>
                    </main>
                    <aside class="bs-details-panel" data-details-panel hidden></aside>
                </section>
                <section class="bs-reader-view" data-reader hidden></section>
                <dialog class="bs-dialog" data-dialog aria-labelledby="bs-dialog-title"><form method="dialog"><header><div><span class="bs-eyebrow" data-dialog-eyebrow>${esc(t('source_setup'))}</span><h2 id="bs-dialog-title" data-dialog-title></h2></div><button type="button" class="bs-icon-button" data-dialog-close aria-label="${esc(t('close'))}">×</button></header><div class="bs-dialog-body" data-dialog-body></div></form></dialog>
            </div>`;
        }

        shell();
        const root = host.querySelector('.bs-app');
        const by = selector => root.querySelector(selector);
        const inform = (text, isError = false) => {
            state.error = isError ? text : '';
            state.notice = isError ? '' : text;
            const node = by('.bs-notice');
            node.textContent = text || '';
            node.hidden = !text;
            node.classList.toggle('is-error', isError);
        };

        function sourceLabel(book) {
            const source = state.sources.find(item => item.id === book.source_id);
            return source?.name || (book.provider ? t('provider_' + book.provider) : t('unknown_source'));
        }

        function formatBytes(value) {
            const bytes = Math.max(0, Number(value) || 0);
            if (bytes < 1024) return bytes + ' B';
            const units = ['KB', 'MB', 'GB', 'TB'];
            let amount = bytes / 1024;
            let unit = 0;
            while (amount >= 1024 && unit < units.length - 1) { amount /= 1024; unit++; }
            return amount.toFixed(amount >= 10 ? 0 : 1) + ' ' + units[unit];
        }

        function recognizedFiles(items) {
            const formats = new Set(['epub', 'pdf', 'mobi', 'azw', 'azw3', 'fb2', 'fb2.zip', 'cbz', 'txt', 'txt.gz', 'html', 'htm', 'cbr', 'rtf', 'docx', 'odt', 'djvu', 'lit', 'pdb', 'lrf']);
            return (Array.isArray(items) ? items : []).filter(item => {
                if (!item || item.is_dir) return false;
                const format = String(item.format || fileExt(item.name || item.path)).toLowerCase().replace(/^\./, '');
                return formats.has(format);
            });
        }

        function folderPreviewMarkup(items, recursive) {
            const books = recognizedFiles(items);
            const sample = books.slice(0, 5).map(item => `<li>${esc(item.name || item.path || t('untitled'))}${item.format ? ` <small>${esc(String(item.format).toUpperCase())}</small>` : ''}</li>`).join('');
            return `<div class="bs-folder-preview" role="status"><strong>${esc(t('recognized_preview').replace('{count}', String(books.length)))}</strong>${sample ? `<ul>${sample}</ul>` : `<p>${esc(t('recognized_none'))}</p>`}${books.length > 5 ? `<small>${esc(t('recognized_more').replace('{count}', String(books.length - 5)))}</small>` : ''}${recursive ? `<p>${esc(t('recognized_recursive'))}</p>` : ''}</div>`;
        }

        function readingState(book) {
            const stored = String(book?.reading_state || '').toLowerCase();
            if (['unread', 'reading', 'finished'].includes(stored)) return stored;
            const fraction = Number(book?.progress?.fraction) || 0;
            return fraction >= 1 ? 'finished' : fraction > 0 ? 'reading' : 'unread';
        }

        async function loadCache() {
            try { state.cache = await request(api.cache); }
            catch (error) { if (!state.disposed && error.name !== 'AbortError') state.cache = null; }
        }

        function cover(book, extraClass = '') {
            const title = esc(book.title || t('untitled'));
            const author = esc(book.author || t('unknown_author'));
            return `<div class="bs-cover ${extraClass}" data-cover="${esc(book.id)}"><img loading="lazy" src="${esc(localBookURL(book))}" alt="${esc(t('cover_alt').replace('{title}', book.title || t('untitled')))}" onerror="this.hidden=true;this.parentElement.classList.add('is-missing')"><span class="bs-cover-fallback"><b>${title}</b><small>${author}</small></span><i aria-hidden="true"></i></div>`;
        }

        function bookTabIndex(book) {
            const active = state.focusedBookId || state.selected?.id;
            return active === book.id ? 0 : -1;
        }

        function handleShelfKeydown(event) {
            const current = event.target.closest?.('[data-book]');
            if (!current || !['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(event.key)) return;
            const shelf = by('[data-shelf]');
            const books = Array.from(shelf.querySelectorAll('[data-book]'));
            const index = books.indexOf(current);
            if (index < 0) return;
            const columns = state.density === 'list' ? 1 : Math.max(1, getComputedStyle(by('.bs-book-grid')).gridTemplateColumns.split(' ').length);
            const delta = event.key === 'ArrowLeft' ? -1 : event.key === 'ArrowRight' ? 1 : event.key === 'ArrowUp' ? -columns : columns;
            const next = books[Math.max(0, Math.min(books.length - 1, index + delta))];
            if (!next || next === current) return;
            event.preventDefault();
            state.focusedBookId = next.dataset.book;
            state.selected = state.books.find(book => book.id === state.focusedBookId) || state.selected;
            state.metadata = state.selected;
            books.forEach(button => button.closest('.bs-book,.bs-list-row')?.classList.toggle('is-selected', button.dataset.book === state.focusedBookId));
            preview();
            next.focus({ preventScroll: true });
        }

        function shelfBook(book) {
            const selected = state.selected?.id === book.id;
            const fraction = Math.max(0, Math.min(1, Number(book.progress?.fraction) || 0));
            if (state.density === 'spines') return `<div class="bs-spine-item"><button type="button" tabindex="${bookTabIndex(book)}" class="bs-spine ${selected ? 'is-selected' : ''}" data-book="${esc(book.id)}" style="--spine-hue:${Math.abs(hash(book.id)) % 36}deg" aria-label="${esc(book.title + ', ' + (book.author || t('unknown_author')))}"><span>${esc(book.title)}</span><small>${esc(book.author || '')}</small><i style="--progress:${fraction * 100}%"></i></button>${bookOrderMarkup(book)}</div>`;
            if (state.density === 'list') return `<article class="bs-list-row ${selected ? 'is-selected' : ''}"><button type="button" tabindex="${bookTabIndex(book)}" class="bs-list-main" data-book="${esc(book.id)}">${cover(book, 'bs-list-cover')}<span><strong>${esc(book.title || t('untitled'))}</strong><small>${esc(book.author || t('unknown_author'))} · ${esc(String(book.format || '').toUpperCase())}</small><span class="bs-list-progress"><i style="width:${fraction * 100}%"></i></span></span></button>${bookOrderMarkup(book)}<button type="button" class="bs-icon-button" data-details="${esc(book.id)}" aria-label="${esc(t('details'))}: ${esc(book.title || t('untitled'))}">···</button></article>`;
            return `<article class="bs-book ${selected ? 'is-selected' : ''}"><button type="button" tabindex="${bookTabIndex(book)}" class="bs-book-select" data-book="${esc(book.id)}" aria-label="${esc(book.title + ', ' + (book.author || t('unknown_author')))}">${cover(book)}<span class="bs-book-title">${esc(book.title || t('untitled'))}</span><span class="bs-book-author">${esc(book.author || t('unknown_author'))}</span><span class="bs-book-meter" aria-label="${esc(t('progress'))}: ${Math.round(fraction * 100)}%"><i style="width:${fraction * 100}%"></i></span></button>${bookOrderMarkup(book)}<button type="button" class="bs-book-more" data-details="${esc(book.id)}" aria-label="${esc(t('details'))}: ${esc(book.title || t('untitled'))}">···</button></article>`;
        }

        function bookOrderMarkup(book) {
            if (!state.collectionId) return '';
            const collection = state.collections.find(item => item.id === state.collectionId);
            const ids = Array.isArray(collection?.book_ids) ? collection.book_ids : [];
            const index = ids.indexOf(book.id);
            if (index < 0) return '';
            return `<span class="bs-book-order"><button type="button" data-book-order="up" data-book-id="${esc(book.id)}" aria-label="${esc(t('move_book_up'))}" ${readOnly() || index === 0 ? 'disabled' : ''}>↑</button><button type="button" data-book-order="down" data-book-id="${esc(book.id)}" aria-label="${esc(t('move_book_down'))}" ${readOnly() || index === ids.length - 1 ? 'disabled' : ''}>↓</button></span>`;
        }

        function hash(value) {
            let result = 0;
            for (const char of String(value || '')) result = (result * 31 + char.charCodeAt(0)) | 0;
            return Math.abs(result);
        }

        function preview() {
            const hostPreview = by('.bs-reading-preview');
            const book = state.selected;
            const lastRead = book && Number(book.progress?.fraction) > 0;
            if (!book) {
                hostPreview.innerHTML = `<div class="bs-paper-page bs-paper-empty"><span class="bs-eyebrow">${esc(t('a_quiet_place'))}</span><h2>${esc(t('no_books_title'))}</h2><p>${esc(t('no_books_copy'))}</p><button type="button" class="bs-primary" data-action="add" ${readOnly() ? 'disabled' : ''}>＋ ${esc(t('add_books'))}</button></div><div class="bs-page-foot"><span>${esc(t('reading_now'))}</span></div>`;
                return;
            }
            const metadata = state.metadata || book;
            const fraction = Math.max(0, Math.min(1, Number(book.progress?.fraction) || 0));
            const hasEPUBRendition = (metadata.renditions || book.renditions || []).some(item => String(item.format || '').toLowerCase() === 'epub' && item.available !== false);
            const readable = canReadDirectly(book) || hasEPUBRendition;
            hostPreview.innerHTML = `<div class="bs-paper-page"><span class="bs-eyebrow">${esc(t(lastRead ? 'reading_now' : 'picked_for_you'))}</span><div class="bs-preview-cover">${cover(book, 'bs-preview-art')}</div><h2>${esc(metadata.title || book.title || t('untitled'))}</h2><p class="bs-preview-author">${esc(metadata.author || book.author || t('unknown_author'))}</p><div class="bs-page-rule" aria-hidden="true"></div><p class="bs-preview-description" dir="auto">${esc(metadata.description || t(lastRead ? 'resume_blurb' : 'choose_book_blurb'))}</p><div class="bs-preview-meta"><span>${esc(String(book.format || '').toUpperCase())}</span><span>${esc(sourceLabel(book))}</span></div><div class="bs-progress-copy"><span>${esc(lastRead ? t('reading_progress') : t('not_started'))}</span><strong>${Math.round(fraction * 100)}%</strong></div><div class="bs-progress-track" role="progressbar" aria-valuenow="${Math.round(fraction * 100)}" aria-valuemin="0" aria-valuemax="100" aria-label="${esc(t('reading_progress'))}"><i style="width:${fraction * 100}%"></i></div><div class="bs-preview-actions"><button type="button" class="bs-primary" data-read="${esc(book.id)}" ${!readable || book.available === false ? 'disabled' : ''}>${esc(t(lastRead ? 'continue_reading' : 'read_book'))}<span aria-hidden="true"> →</span></button><button type="button" class="bs-text-button" data-details="${esc(book.id)}">${esc(t('details'))}</button></div></div><div class="bs-page-foot"><span>${esc(t('reading_now'))}</span><span>${esc(t('library'))}</span></div>`;
        }

        function collectionStrip() {
            const strip = by('[data-collection-strip]');
            strip.hidden = state.page !== 'collections';
            const sortControl = by('[data-sort]');
            const collectionSort = sortControl?.querySelector('option[value="collection"]');
            if (collectionSort) collectionSort.disabled = !state.collectionId;
            if (sortControl) sortControl.value = state.sort;
            const collections = orderedCollections();
            strip.innerHTML = state.page === 'collections' ? `<button type="button" class="bs-collection-chip ${!state.collectionId ? 'is-active' : ''}" data-collection="">${esc(t('all_books'))}</button>${collections.map((item, index) => `<div class="bs-collection-group"><button type="button" class="bs-collection-chip ${state.collectionId === item.id ? 'is-active' : ''}" data-collection="${esc(item.id)}">${esc(item.name)}<small>${Number(item.book_count) || 0}</small></button><span class="bs-collection-order"><button type="button" data-collection-order="up" data-collection-id="${esc(item.id)}" aria-label="${esc(t('move_collection_up'))}" ${readOnly() || index === 0 ? 'disabled' : ''}>↑</button><button type="button" data-collection-order="down" data-collection-id="${esc(item.id)}" aria-label="${esc(t('move_collection_down'))}" ${readOnly() || index === collections.length - 1 ? 'disabled' : ''}>↓</button></span></div>`).join('')}<button type="button" class="bs-collection-create" data-action="new-collection" ${readOnly() ? 'disabled' : ''}>＋ ${esc(t('new_collection'))}</button>` : '';
            by('[data-collection-label]').textContent = state.collectionId ? (state.collections.find(x => x.id === state.collectionId)?.name || t('collections')) : state.page === 'collections' ? t('collections') : t('all_books');
            by('[data-library-title]').textContent = state.page === 'collections' ? t('collections') : t('library');
        }

        function orderedCollections() {
            return [...state.collections].sort((a, b) => (Number(a.position) || 0) - (Number(b.position) || 0) || String(a.name || '').localeCompare(String(b.name || '')));
        }

        async function moveCollection(id, direction) {
            if (readOnly()) return;
            const ordered = orderedCollections();
            const index = ordered.findIndex(item => item.id === id);
            const targetIndex = index + (direction === 'up' ? -1 : 1);
            if (index < 0 || targetIndex < 0 || targetIndex >= ordered.length) return;
            try {
                for (let position = 0; position < ordered.length; position++) {
                    const item = ordered[position];
                    if (Number(item.position) === position) continue;
                    const result = await request(api.updateCollection, item.id, { position }, item.revision);
                    Object.assign(item, result.collection || { position });
                }
                const current = ordered[index];
                const neighbor = ordered[targetIndex];
                const currentPosition = Number(current.position);
                const neighborPosition = Number(neighbor.position);
                const updatedCurrent = await request(api.updateCollection, current.id, { position: neighborPosition }, current.revision);
                Object.assign(current, updatedCurrent.collection || { position: neighborPosition });
                const updatedNeighbor = await request(api.updateCollection, neighbor.id, { position: currentPosition }, neighbor.revision);
                Object.assign(neighbor, updatedNeighbor.collection || { position: currentPosition });
            } catch (error) {
                // A failed mutation may have reached the server. Read back its authoritative order; never replay or compensate an uncertain write.
                await loadCollections();
                collectionStrip();
                throw error;
            }
            await loadCollections();
            collectionStrip();
            inform(t('collection_order_saved'));
        }

        async function moveBookInCollection(bookId, direction) {
            if (readOnly() || !state.collectionId) return;
            const collection = state.collections.find(item => item.id === state.collectionId);
            const bookIds = Array.isArray(collection?.book_ids) ? [...collection.book_ids] : [];
            const index = bookIds.indexOf(bookId);
            const targetIndex = index + (direction === 'up' ? -1 : 1);
            if (index < 0 || targetIndex < 0 || targetIndex >= bookIds.length) return;
            [bookIds[index], bookIds[targetIndex]] = [bookIds[targetIndex], bookIds[index]];
            try {
                await request(api.reorderCollectionBooks, collection.id, bookIds, collection.revision);
            } catch (error) {
                // Reconcile an uncertain mutation from the server before reporting it; never replay its payload.
                await loadCollections();
                collectionStrip();
                throw error;
            }
            await loadCollections();
            collectionStrip();
            const targetOffset = Math.floor(targetIndex / PAGE_SIZE) * PAGE_SIZE;
            await loadBooks(targetOffset);
            inform(t('collection_order_saved'));
        }

        function paintShelf() {
            if (state.page !== 'library' && state.page !== 'collections') return;
            const shelf = by('[data-shelf]');
            const activeBook = document.activeElement?.closest?.('[data-book]');
            const activeOrderBook = document.activeElement?.closest?.('[data-book-order]');
            if (activeBook && shelf.contains(activeBook)) {
                state.focusedBookId = activeBook.dataset.book || state.focusedBookId;
                state.restoreShelfFocus = true;
            } else if (activeOrderBook && shelf.contains(activeOrderBook)) {
                state.focusedBookId = activeOrderBook.dataset.bookId || state.focusedBookId;
                state.restoreShelfFocus = true;
            }
            const rows = state.books.map(shelfBook).join('');
            const empty = state.loading ? `<div class="bs-empty-state"><span class="bs-spinner" aria-hidden="true"></span><p>${esc(t('loading'))}</p></div>` : state.error ? `<div class="bs-empty-state"><h2>${esc(t('load_failed'))}</h2><p>${esc(state.error)}</p><button type="button" data-action="retry">${esc(t('try_again'))}</button></div>` : `<div class="bs-empty-state"><span aria-hidden="true">✦</span><h2>${esc(state.query || state.sourceId ? t('no_results_title') : t('no_books_title'))}</h2><p>${esc(state.query || state.sourceId ? t('no_results_copy') : t('no_books_copy'))}</p><button type="button" class="bs-primary" data-action="add" ${readOnly() ? 'disabled' : ''}>＋ ${esc(t('add_books'))}</button></div>`;
            const pageNav = state.total > PAGE_SIZE ? `<nav class="bs-page-nav" aria-label="${esc(t('library_pagination'))}"><button type="button" data-action="page" data-offset="${Math.max(0, state.offset - PAGE_SIZE)}" ${state.offset === 0 ? 'disabled' : ''}>‹ ${esc(t('previous_page'))}</button><span>${esc(t('showing_count').replace('{shown}', String(Math.min(state.offset + state.books.length, state.total))).replace('{total}', String(state.total)))}</span><button type="button" data-action="page" data-offset="${state.offset + PAGE_SIZE}" ${state.offset + state.books.length >= state.total ? 'disabled' : ''}>${esc(t('next_page'))} ›</button></nav>` : '';
            shelf.dataset.density = state.density;
            shelf.innerHTML = state.books.length ? `<div class="bs-shelf-board ${state.density === 'list' ? 'is-list' : ''}"><div class="bs-book-grid">${rows}</div></div>${pageNav}` : empty;
            by('[data-library-count]').textContent = t('showing_count').replace('{shown}', String(Math.min(state.offset + state.books.length, state.total))).replace('{total}', String(state.total));
            preview();
            if (state.restoreShelfFocus) {
                state.restoreShelfFocus = false;
                requestAnimationFrame(() => {
                    const target = Array.from(shelf.querySelectorAll('[data-book]')).find(item => item.dataset.book === state.focusedBookId)
                        || Array.from(shelf.querySelectorAll('[data-book]')).find(item => item.dataset.book === state.selected?.id);
                    target?.focus({ preventScroll: true });
                });
            }
        }

        function updateFilters() {
            const select = by('[data-source-filter]');
            const current = state.sourceId;
            select.innerHTML = `<option value="">${esc(t('all_sources'))}</option>${state.sources.map(source => `<option value="${esc(source.id)}">${esc(source.name || source.provider)}</option>`).join('')}`;
            select.value = state.sources.some(source => source.id === current) ? current : '';
            by('[data-density]').value = state.density;
        }

        async function loadBooks(pageOffset = 0) {
            if (state.disposed || appDisabled()) return;
            const generation = ++state.bookLoadGeneration;
            const offset = Math.max(0, Math.floor(Number(pageOffset) / PAGE_SIZE) * PAGE_SIZE);
            const params = {
                q: state.query, source_id: state.sourceId, collection_id: state.collectionId,
                status: state.statusFilter, sort: state.sort,
                offset, limit: PAGE_SIZE
            };
            state.loading = true;
            state.offset = offset;
            state.books = [];
            state.error = '';
            paintShelf();
            try {
                const result = await request(api.books, params);
                if (state.disposed || generation !== state.bookLoadGeneration) return;
                const page = Array.isArray(result.books) ? result.books : [];
                state.books = page;
                state.total = Number(result.total) || 0;
                state.offset = offset;
                state.selected = state.books.find(item => item.id === state.selected?.id) || state.books.find(item => Number(item.progress?.fraction) > 0) || state.books[0] || null;
                if (!state.books.some(item => item.id === state.focusedBookId)) state.focusedBookId = state.selected?.id || '';
                state.metadata = state.selected;
            } catch (error) {
                if (!state.disposed && generation === state.bookLoadGeneration && error.name !== 'AbortError') state.error = readableError(error, t('load_failed'));
            } finally {
                if (generation === state.bookLoadGeneration) {
                    state.loading = false;
                    if (!state.disposed) paintShelf();
                }
            }
        }

        function readableError(error, fallback) {
            const code = error?.body?.code || error?.body?.error;
            if (code && typeof code === 'string') {
                const key = 'error_' + code.toLowerCase().replace(/[^a-z0-9_]+/g, '_');
                const value = t(key);
                if (value !== key.replaceAll('_', ' ')) return value;
            }
            return error?.message || fallback;
        }

        async function loadMetadata(book) {
            state.selected = book;
            state.metadata = book;
            paintShelf();
            try {
                const result = await request(api.book, book.id);
                const details = result?.book || result || {};
                state.metadata = Object.assign({}, book, details, {
                    collections: details.collections ?? result?.collections ?? book.collections ?? [],
                    renditions: details.renditions ?? result?.renditions ?? book.renditions ?? [],
                    progress: details.progress ?? result?.progress ?? book.progress
                });
                if (state.selected?.id === book.id && !state.disposed) renderDetails();
            } catch (error) {
                if (!state.disposed && error.name !== 'AbortError') inform(readableError(error, t('details_failed')), true);
            }
        }

        function renderDetails() {
            const panel = by('[data-details-panel]');
            panel.hidden = !state.details;
            if (!state.details || !state.selected) return;
            const book = state.metadata || state.selected;
            const format = String(book.format || '').toUpperCase();
            const converter = state.capabilities.converter || {};
            const convertible = converter.available === true && (converter.formats || []).map(value => String(value).toLowerCase()).includes(String(book.format || '').toLowerCase());
            const converted = (book.renditions || []).some(item => String(item.format || '').toLowerCase() === 'epub' && item.available !== false);
            const readable = canReadDirectly(book) || converted;
            const memberships = new Set((Array.isArray(book.collections) ? book.collections : []).map(item => typeof item === 'string' ? item : item.id).filter(Boolean));
            const cacheSummary = state.cache ? `<small>${esc(t('cache_usage').replace('{used}', formatBytes(state.cache.used_bytes)).replace('{pinned}', formatBytes(state.cache.pinned_bytes)).replace('{limit}', formatBytes(state.cache.limit_bytes)))}</small>` : '';
            panel.innerHTML = `<button type="button" class="bs-details-close" data-action="details-close" aria-label="${esc(t('close'))}">×</button><div class="bs-detail-cover">${cover(book)}</div><span class="bs-eyebrow">${esc(t('book_details'))}</span><h2>${esc(book.title || t('untitled'))}</h2><p class="bs-detail-author">${esc(book.author || t('unknown_author'))}</p><dl><dt>${esc(t('format'))}</dt><dd>${esc(format || '—')}</dd><dt>${esc(t('source'))}</dt><dd>${esc(sourceLabel(book))}</dd><dt>${esc(t('availability'))}</dt><dd>${esc(book.available === false ? t('unavailable') : t('available'))}</dd>${book.series ? `<dt>${esc(t('series'))}</dt><dd>${esc(book.series)}</dd>` : ''}${book.tags?.length ? `<dt>${esc(t('tags'))}</dt><dd>${book.tags.map(esc).join(', ')}</dd>` : ''}</dl>${book.description ? `<p class="bs-detail-description" dir="auto">${esc(book.description)}</p>` : ''}<section class="bs-cache-card"><div><strong>${esc(t('keep_on_server'))}</strong><small>${esc(book.cache_pinned ? t('cache_pinned') : t('cache_not_pinned'))}</small></div><button type="button" data-pin-toggle ${readOnly() ? 'disabled' : ''}>${esc(book.cache_pinned ? t('remove_pin') : t('pin_copy'))}</button>${cacheSummary}</section>${state.collections.length ? `<label class="bs-detail-collection">${esc(t('add_to_collection'))}<select data-membership><option value="">${esc(t('choose_collection'))}</option>${state.collections.filter(item => !memberships.has(item.id) && !(item.book_ids || []).includes(book.id)).map(item => `<option value="${esc(item.id)}">${esc(item.name)}</option>`).join('')}</select></label>` : ''}${memberships.size ? `<div class="bs-membership-list"><span>${esc(t('in_collections'))}</span>${Array.from(memberships).map(id => { const item = state.collections.find(value => value.id === id); return item ? `<button type="button" data-remove-membership="${esc(id)}" ${readOnly() ? 'disabled' : ''}>${esc(item.name)} <span aria-hidden="true">×</span></button>` : ''; }).join('')}</div>` : ''}<label class="bs-cover-upload">${esc(t('replace_cover'))}<input type="file" accept="image/png,image/jpeg,image/webp" data-cover-upload ${readOnly() ? 'disabled' : ''}><small>${esc(t('cover_upload_help'))}</small></label>${converted ? `<p class="bs-conversion-note">${esc(t('converted_copy_notice'))}</p>` : convertible ? `<p class="bs-conversion-note">${esc(t('conversion_copy_notice'))}</p>` : `<p class="bs-conversion-note">${esc(t('conversion_unavailable'))}</p>`}<div class="bs-detail-actions">${converted ? `<button type="button" class="bs-primary" data-read="${esc(book.id)}" data-read-format="epub" ${book.available === false ? 'disabled' : ''}>${esc(t('read_converted'))}</button>` : `<button type="button" class="bs-primary" data-read="${esc(book.id)}" ${book.available === false || !readable ? 'disabled' : ''}>${esc(Number(book.progress?.fraction) > 0 ? t('continue_reading') : t('read_book'))}</button>`}${convertible && !converted ? `<button type="button" class="bs-text-button" data-convert="${esc(book.id)}" ${readOnly() ? 'disabled' : ''}>${esc(t('convert_book'))}</button>` : ''}<button type="button" class="bs-text-button" data-action="edit-details" ${readOnly() ? 'disabled' : ''}>${esc(t('edit_metadata'))}</button><button type="button" class="bs-text-button bs-danger" data-action="forget-book" ${readOnly() ? 'disabled' : ''}>${esc(t('forget_book'))}</button></div>`;
            const readingControl = document.createElement('label');
            readingControl.className = 'bs-reading-state';
            readingControl.innerHTML = `${esc(t('reading_state'))}<select data-reading-state ${readOnly() ? 'disabled' : ''}><option value="unread">${esc(t('unread'))}</option><option value="reading">${esc(t('reading'))}</option><option value="finished">${esc(t('finished'))}</option></select>`;
            panel.querySelector('.bs-detail-actions')?.before(readingControl);
            readingControl.querySelector('select').value = readingState(book);
        }

        async function togglePinnedCopy() {
            const book = state.metadata || state.selected;
            if (!book || readOnly()) return;
            const pinned = !book.cache_pinned;
            const result = await request(api.setPinned, book.id, pinned, book.revision);
            const updated = Object.assign({}, book, result?.book || {}, { cache_pinned: result?.book?.cache_pinned ?? pinned });
            state.selected = updated;
            state.metadata = updated;
            await Promise.all([loadCache(), loadBooks()]);
            if (state.selected?.id === updated.id) {
                state.metadata = Object.assign({}, state.metadata || updated, { cache_pinned: updated.cache_pinned });
                renderDetails();
            }
            inform(t(pinned ? 'cache_pinned' : 'cache_unpinned'));
        }

        function showDialog(title, eyebrow, body, onSubmit) {
            const dialog = by('[data-dialog]');
            by('[data-dialog-title]').textContent = title;
            by('[data-dialog-eyebrow]').textContent = eyebrow || '';
            by('[data-dialog-body]').innerHTML = body;
            const form = dialog.querySelector('form');
            form.onsubmit = event => {
                if (event.target !== form) return;
                event.preventDefault();
                Promise.resolve(onSubmit?.(new FormData(form), form)).catch(error => {
                    const errorHost = form.querySelector('[data-form-error]');
                    if (errorHost) { errorHost.hidden = false; errorHost.textContent = readableError(error, t('save_failed')); }
                    const submit = form.querySelector('[type=submit]');
                    if (submit) submit.disabled = false;
                });
            };
            if (!dialog.open) dialog.showModal();
            const focusable = dialog.querySelector('input:not([type=hidden]),select,button:not([data-dialog-close])');
            focusable?.focus();
        }

        function closeDialog() {
            const dialog = by('[data-dialog]');
            if (dialog.open) dialog.close();
            if (state.passwordResolver) {
                const resolve = state.passwordResolver;
                state.passwordResolver = null;
                resolve(null);
            }
            if (state.folderChooser) {
                const chooser = state.folderChooser;
                state.folderChooser = null;
                if (!chooser.inline) chooser.resolve(null);
            }
        }

        function openAddDialog() {
            const providers = Array.isArray(state.capabilities.sources) ? state.capabilities.sources : [];
            const providerOptions = providers.map(item => `<option value="${esc(item.provider)}" ${item.available === false ? 'disabled' : ''}>${esc(t('provider_' + item.provider))}${item.available === false ? ' — ' + esc(item.reason || t('unavailable')) : ''}</option>`).join('');
            const roots = Array.isArray(state.capabilities.local_roots) ? state.capabilities.local_roots : [];
            const mounts = providers.flatMap(item => (item.mounts || []).map(mount => Object.assign({ provider: item.provider }, mount)));
            showDialog(t('add_books'), t('source_setup'), `<p>${esc(t('source_setup_copy'))}</p><div class="bs-form-grid"><label>${esc(t('source_provider'))}<select name="provider" required><option value="">${esc(t('choose_provider'))}</option>${providerOptions}</select></label><label>${esc(t('source_name'))}<input name="name" maxlength="120" required></label><label class="bs-provider-fields" data-provider="local" hidden>${esc(t('location'))}<select name="root_id">${roots.map(rootItem => `<option value="${esc(rootItem.id)}">${esc(rootItem.label)}</option>`).join('')}</select></label><label class="bs-provider-fields" data-provider="cloud" hidden>${esc(t('location'))}<select name="mount_id">${mounts.map(mount => `<option value="${esc(mount.id)}" data-provider="${esc(mount.provider)}">${esc(mount.name || mount.label || mount.id)}</option>`).join('')}</select></label><label class="bs-folder-field">${esc(t('folder_path'))}<div class="bs-folder-value"><input name="path" value="" readonly aria-describedby="bs-folder-help"><input type="hidden" name="path_selected" value="false"><button type="button" data-pick-folder>${esc(t('choose_folder'))}</button></div><small id="bs-folder-help" data-folder-help>${esc(t('folder_picker_help'))}</small></label><label class="bs-provider-fields" data-provider="smb" hidden>${esc(t('smb_server'))}<input name="server" autocomplete="url" required></label><label class="bs-provider-fields" data-provider="smb" hidden>${esc(t('smb_share'))}<input name="share" autocomplete="off" required></label><label class="bs-provider-fields" data-provider="smb" hidden>${esc(t('username'))}<input name="username" autocomplete="username"></label><label class="bs-provider-fields" data-provider="smb" hidden>${esc(t('domain'))}<input name="domain" autocomplete="organization"></label><label class="bs-provider-fields" data-provider="smb" hidden>${esc(t('password'))}<input type="password" name="password" autocomplete="new-password"></label><label class="bs-check"><input type="checkbox" name="recursive" checked><span>${esc(t('include_subfolders'))}</span></label><label>${esc(t('refresh_schedule'))}<select name="refresh_minutes"><option value="0">${esc(t('refresh_manual'))}</option><option value="60">${esc(t('refresh_hourly'))}</option><option value="360">${esc(t('refresh_six_hours'))}</option><option value="1440">${esc(t('refresh_daily'))}</option></select><small>${esc(t('refresh_opt_in_help'))}</small></label></div><div data-folder-picker class="bs-folder-picker" hidden></div><div data-selected-folder-preview hidden></div><p class="bs-form-error" data-form-error role="alert" hidden></p><footer class="bs-dialog-actions"><button type="button" data-dialog-close>${esc(t('cancel'))}</button><button type="submit" class="bs-primary">${esc(t('add_and_scan'))}</button></footer>`, async (form, formElement) => {
                const provider = String(form.get('provider') || '');
                if (!provider) throw new Error(t('choose_provider'));
                const source = { provider, name: String(form.get('name') || '').trim(), path: String(form.get('path') || '').trim(), recursive: form.has('recursive'), refresh_minutes: Number(form.get('refresh_minutes')) || 0 };
                if (provider === 'local') source.mount_id = String(form.get('root_id') || '');
                if (provider === 'koofr') source.mount_id = String(form.get('mount_id') || '');
                const credential = provider === 'smb' ? { password: String(form.get('password') || '') } : null;
                if (provider === 'smb') {
                    const username = String(form.get('username') || '').trim();
                    const password = String(form.get('password') || '');
                    if (!username || !password) throw new Error(t('smb_credentials_required'));
                    Object.assign(source, { path: '', host: String(form.get('server') || '').trim(), share: String(form.get('share') || '').trim(), username, domain: String(form.get('domain') || '').trim() });
                    credential.password = password;
                }
                if (provider !== 'smb' && form.get('path_selected') !== 'true') throw new Error(t('choose_folder'));
                formElement.querySelector('[type=submit]').disabled = true;
                const result = await request(api.addSource, source, credential);
                closeDialog();
                if (provider === 'smb' && result.source?.id) {
                    inform(t('choose_smb_folder'));
                    const selectedPath = await chooseFolder({ sourceId: result.source.id, startPath: '' });
                    if (selectedPath == null) {
                        inform(t('source_added_no_scan'));
                        await loadSources();
                        return;
                    }
                    await request(api.updateSource, result.source.id, { path: selectedPath, recursive: source.recursive, name: source.name, refresh_minutes: source.refresh_minutes }, result.source.generation);
                }
                inform(t('source_added'));
                await loadSources();
                if (result.source?.id) await startScan(result.source);
            });
            const providerSelect = by('[data-dialog] [name=provider]');
            providerSelect.addEventListener('change', () => {
                const value = providerSelect.value;
                by('[data-dialog] [data-provider="local"]').hidden = value !== 'local';
                by('[data-dialog] [data-provider="cloud"]').hidden = value !== 'koofr';
                root.querySelectorAll('.bs-provider-fields[data-provider="smb"]').forEach(node => {
                    node.hidden = value !== 'smb';
                    node.querySelectorAll('input,select').forEach(input => {
                        input.disabled = value !== 'smb';
                        input.required = value === 'smb' && ['server', 'share'].includes(input.name);
                    });
                });
                by('[data-dialog] [name=path]').value = '';
                by('[data-dialog] [name=path_selected]').value = 'false';
                by('[data-dialog] [data-folder-picker]').hidden = true;
                state.folderChooser = null;
                const mountSelect = by('[data-dialog] [name=mount_id]');
                if (mountSelect) {
                    Array.from(mountSelect.options).forEach(option => option.hidden = option.dataset.provider !== value);
                    mountSelect.value = value === 'koofr' ? (Array.from(mountSelect.options).find(option => option.dataset.provider === value)?.value || '') : '';
                    mountSelect.disabled = value !== 'koofr';
                }
                const picker = by('[data-dialog] [data-pick-folder]');
                if (picker) picker.disabled = value === 'smb' || !value || (value === 'koofr' && !mountSelect?.value);
                by('[data-dialog] [data-folder-help]').textContent = value === 'smb' ? t('smb_folder_after_connect') : t('folder_picker_help');
                by('[data-dialog] [data-selected-folder-preview]').hidden = true;
                if (value === 'webdav') openInlineFolderPicker();
                if (value === 'koofr' && mountSelect && !Array.from(mountSelect.options).some(option => option.dataset.provider === value)) {
                    mountSelect.disabled = true;
                    request(api.browse, value, '', '').then(result => {
                        if (providerSelect.value !== value) return;
                        const mounts = Array.isArray(result.mounts) ? result.mounts : [];
                        mounts.forEach(mount => {
                            const option = document.createElement('option');
                            option.value = mount.id || mount.mount_id || '';
                            option.dataset.provider = value;
                            option.textContent = mount.name || mount.label || mount.id || option.value;
                            mountSelect.append(option);
                        });
                    }).catch(error => { if (providerSelect.value === value) inform(readableError(error, t('browse_failed')), true); }).finally(() => {
                        if (providerSelect.value === value) {
                            mountSelect.disabled = false;
                            const providerMounts = Array.from(mountSelect.options).filter(option => option.dataset.provider === value);
                            if (providerMounts.length === 1) {
                                mountSelect.selectedIndex = 0;
                                mountSelect.value = providerMounts[0].value;
                                if (picker) picker.disabled = false;
                                openInlineFolderPicker();
                            }
                        }
                    });
                }
                if (value === 'koofr' && mountSelect?.value && picker) {
                    picker.disabled = false;
                    openInlineFolderPicker();
                }
            });
            by('[data-dialog] [name=root_id]')?.addEventListener('change', () => by('[data-dialog] [name=path_selected]').value = 'false');
            by('[data-dialog] [name=mount_id]')?.addEventListener('change', event => {
                by('[data-dialog] [name=path_selected]').value = 'false';
                const picker = by('[data-dialog] [data-pick-folder]');
                if (picker) picker.disabled = !event.target.value;
                if (providerSelect.value === 'koofr' && event.target.value) openInlineFolderPicker();
            });
        }

        function openInlineFolderPicker() {
            const form = by('[data-dialog] form');
            const provider = String(form.elements.provider?.value || '');
            const mountId = provider === 'local' ? String(form.elements.root_id?.value || '') : provider === 'koofr' ? String(form.elements.mount_id?.value || '') : '';
            if (!provider || (provider === 'koofr' && !mountId) || provider === 'smb') return;
            const panel = by('[data-dialog] [data-folder-picker]');
            panel.hidden = false;
            state.folderChooser = { inline: true, provider, mountId, current: '' };
            loadFolderPage();
        }

        function parentFolder(path) {
            const clean = String(path || '').replace(/\\/g, '/').replace(/\/+$/, '');
            if (!clean || clean === '.') return '';
            const slash = clean.lastIndexOf('/');
            return slash < 0 ? '' : clean.slice(0, slash);
        }

        function chooseFolder(options) {
            return new Promise(resolve => {
                state.folderChooser = Object.assign({ current: options?.startPath || '', resolve }, options || {});
                showDialog(t('choose_folder'), t('source_setup'), `<div data-folder-picker></div><p class="bs-form-error" data-form-error role="alert" hidden></p><footer class="bs-dialog-actions"><button type="button" data-dialog-close>${esc(t('cancel'))}</button><button type="submit" class="bs-primary">${esc(t('use_this_folder'))}</button></footer>`, async () => {
                    const chooser = state.folderChooser;
                    if (!chooser) return;
                    state.folderChooser = null;
                    closeDialog();
                    chooser.resolve(chooser.current || '');
                });
                loadFolderPage();
            });
        }

        async function loadFolderPage() {
            const chooser = state.folderChooser;
            if (!chooser || state.disposed) return;
            const picker = by('[data-folder-picker]');
            if (picker) picker.innerHTML = `<div class="bs-folder-loading"><span class="bs-spinner" aria-hidden="true"></span>${esc(t('loading'))}</div>`;
            try {
                const result = chooser.sourceId
                    ? await request(api.browseSource, chooser.sourceId, chooser.current)
                    : await request(api.browse, chooser.provider, chooser.mountId, chooser.current);
                if (state.folderChooser !== chooser || state.disposed) return;
                chooser.items = Array.isArray(result.items) ? result.items : [];
                const folders = (Array.isArray(result.items) ? result.items : []).filter(item => item.is_dir);
                const path = chooser.current || '';
                const rows = folders.length ? folders.map(item => `<button type="button" class="bs-folder-row" data-folder-enter="${esc(item.path)}"><span aria-hidden="true">📁</span><span>${esc(item.name || item.path || t('unnamed_folder'))}</span><span aria-hidden="true">›</span></button>`).join('') : `<p class="bs-folder-empty">${esc(t('no_subfolders'))}</p>`;
                const form = by('[data-dialog] form');
                const recursive = Boolean(form?.elements.recursive?.checked);
                const preview = folderPreviewMarkup(chooser.items, recursive);
                by('[data-folder-picker]').innerHTML = `<div class="bs-folder-path"><button type="button" data-folder-up ${path ? '' : 'disabled'}>↑ ${esc(t('up_one_level'))}</button><span title="${esc(path || t('root_folder'))}">${esc(path || t('root_folder'))}</span></div><div class="bs-folder-list" role="list">${rows}</div>${preview}<p class="bs-folder-help">${esc(t('folder_picker_help'))}</p><button type="button" data-folder-use class="bs-primary">${esc(t('use_this_folder'))}</button>`;
            } catch (error) {
                if (state.folderChooser !== chooser) return;
                by('[data-folder-picker]').innerHTML = `<p class="bs-folder-error" role="alert">${esc(readableError(error, t('browse_failed')))}</p><button type="button" data-folder-retry>${esc(t('try_again'))}</button>`;
            }
        }

        async function loadSources() {
            const result = await request(api.sources);
            state.sources = Array.isArray(result.sources) ? result.sources : [];
            updateFilters();
        }

        async function startScan(source) {
            if (!source?.id) return;
            try {
                const result = await request(api.scanSource, source.id);
                if (result.job_id) pollJob(result.job_id, source.name || source.provider);
            } catch (error) {
                if (!state.disposed) inform(readableError(error, t('scan_failed')), true);
            }
        }

        async function convertBook(id) {
            if (readOnly()) return;
            const book = state.selected;
            if (!book || book.id !== id) return;
            const result = await request(api.convert, id);
            if (result.job_id) pollJob(result.job_id, book.title || t('untitled'), 'conversion');
        }

        function pollJob(jobId, label, kind = 'scan') {
            clearTimeout(state.jobTimer);
            const poll = async () => {
                try {
                    const result = await request(api.job, jobId);
                    if (state.disposed) return;
                    state.job = result;
                    const status = String(result.status || '').toLowerCase();
                    const terminal = ['complete', 'completed', 'succeeded', 'failed', 'partial', 'canceled', 'cancelled'].includes(status);
                    const counts = t('scan_counts').replace('{scanned}', String(Number(result.scanned) || 0)).replace('{imported}', String(Number(result.imported) || 0)).replace('{skipped}', String(Number(result.skipped) || 0));
                    const statusCopy = status === 'failed'
                        ? t(kind === 'conversion' ? 'conversion_failed' : 'scan_failed')
                        : status === 'partial'
                            ? (kind === 'scan' ? t('scan_partial') : t('job_completed'))
                            : status === 'canceled' || status === 'cancelled'
                                ? t('job_cancelled')
                                : ['complete', 'completed', 'succeeded'].includes(status)
                                    ? t('job_completed')
                                    : t('job_running');
                    if (status === 'failed') inform(readableError({ message: result.error }, kind === 'conversion' ? t('conversion_failed') : t('scan_failed')), true);
                    else if (kind === 'conversion') inform(`${label}: ${statusCopy}`);
                    else inform(`${label}: ${statusCopy} · ${counts}`);
                    if (terminal) {
                        state.job = null;
                        state.jobTimer = 0;
                        await loadBooks();
                        if (kind === 'conversion' && state.selected) await loadMetadata(state.selected);
                        return;
                    }
                    state.jobTimer = setTimeout(poll, 1400);
                } catch (error) {
                    if (!state.disposed && error.name !== 'AbortError') inform(readableError(error, t('scan_failed')), true);
                }
            };
            poll();
        }

        function sourcesDialog() {
            const providers = Array.isArray(state.capabilities.sources) ? state.capabilities.sources : [];
            showDialog(t('sources'), t('connected_locations'), `<div class="bs-source-list">${state.sources.map(source => `<article><span class="bs-source-glyph" aria-hidden="true">▤</span><div><strong>${esc(source.name || t('provider_' + source.provider))}</strong><small>${esc(t('provider_' + source.provider))} · ${esc(source.path || t('root_folder'))}</small><span>${esc(source.available === false ? t('unavailable') : t('connected'))}</span><small>${esc(t('refresh_schedule'))}: ${esc(source.refresh_minutes ? t(source.refresh_minutes === 60 ? 'refresh_hourly' : source.refresh_minutes === 360 ? 'refresh_six_hours' : source.refresh_minutes === 1440 ? 'refresh_daily' : 'refresh_custom').replace('{minutes}', String(source.refresh_minutes)) : t('refresh_manual'))}</small></div><button type="button" data-source-refresh="${esc(source.id)}" ${readOnly() ? 'disabled' : ''}>${esc(t('refresh_schedule'))}</button><button type="button" data-source-root="${esc(source.id)}" ${readOnly() ? 'disabled' : ''}>${esc(t('change_folder'))}</button><button type="button" data-scan="${esc(source.id)}" ${readOnly() ? 'disabled' : ''}>↻ ${esc(t('scan_now'))}</button><button type="button" data-remove-source="${esc(source.id)}" ${readOnly() ? 'disabled' : ''} aria-label="${esc(t('remove_source'))}">×</button></article>`).join('') || `<p class="bs-source-empty">${esc(t('no_sources'))}</p>`}</div><div class="bs-provider-status">${providers.map(item => `<p><span>${esc(t('provider_' + item.provider))}</span><span>${esc(item.available ? t('available') : item.reason || t('not_configured'))}</span></p>`).join('')}</div>${state.cache ? `<p class="bs-cache-summary">${esc(t('cache_usage').replace('{used}', formatBytes(state.cache.used_bytes)).replace('{pinned}', formatBytes(state.cache.pinned_bytes)).replace('{limit}', formatBytes(state.cache.limit_bytes)))}</p>` : ''}<footer class="bs-dialog-actions"><button type="button" data-dialog-close>${esc(t('close'))}</button><button type="button" class="bs-primary" data-action="add-source" ${readOnly() ? 'disabled' : ''}>＋ ${esc(t('add_source'))}</button></footer>`, null);
        }

        function editSourceRefresh(id) {
            const source = state.sources.find(item => item.id === id);
            if (!source) return;
            const options = [[0, 'refresh_manual'], [60, 'refresh_hourly'], [360, 'refresh_six_hours'], [1440, 'refresh_daily']];
            showDialog(t('refresh_schedule'), source.name || t('sources'), `<label>${esc(t('refresh_schedule'))}<select name="refresh_minutes">${options.map(([minutes, key]) => `<option value="${minutes}" ${Number(source.refresh_minutes) === minutes ? 'selected' : ''}>${esc(t(key))}</option>`).join('')}</select><small>${esc(t('refresh_opt_in_help'))}</small></label><p class="bs-form-error" data-form-error role="alert" hidden></p><footer class="bs-dialog-actions"><button type="button" data-dialog-close>${esc(t('cancel'))}</button><button type="submit" class="bs-primary">${esc(t('save'))}</button></footer>`, async form => {
                const refresh_minutes = Number(form.get('refresh_minutes')) || 0;
                await request(api.updateSource, source.id, { refresh_minutes }, source.generation);
                closeDialog();
                await loadSources();
                sourcesDialog();
                inform(t('refresh_saved'));
            });
        }

        async function changeSourceRoot(id) {
            const source = state.sources.find(item => item.id === id);
            if (!source) return;
            const path = await chooseFolder({ sourceId: source.id });
            if (path == null) return;
            await request(api.updateSource, source.id, { path }, source.generation);
            closeDialog();
            await loadSources();
            await startScan(source);
            sourcesDialog();
        }

        async function removeSource(id) {
            const source = state.sources.find(item => item.id === id);
            if (!source) return;
            const accepted = ctx.confirmDialog ? await ctx.confirmDialog(t('remove_source'), t('remove_source_confirm').replace('{name}', source.name || source.provider)) : false;
            if (!accepted) return;
            await request(api.removeSource, id);
            state.sources = state.sources.filter(item => item.id !== id);
            await loadSources();
            await loadBooks();
            sourcesDialog();
            inform(t('source_removed'));
        }

        function collectionDialog() {
            showDialog(t('new_collection'), t('collections'), `<label>${esc(t('collection_name'))}<input name="name" maxlength="80" required autofocus></label><p class="bs-form-error" data-form-error role="alert" hidden></p><footer class="bs-dialog-actions"><button type="button" data-dialog-close>${esc(t('cancel'))}</button><button type="submit" class="bs-primary">${esc(t('create_collection'))}</button></footer>`, async form => {
                const name = String(form.get('name') || '').trim();
                if (!name) throw new Error(t('collection_name_required'));
                const nextPosition = orderedCollections().reduce((maximum, item) => Math.max(maximum, Number(item.position) || 0), -1) + 1;
                await request(api.createCollection, { name, position: nextPosition });
                closeDialog();
                await loadCollections();
                state.page = 'collections';
                state.collectionId = state.collections.find(item => item.name === name)?.id || '';
                if (state.collectionId && !state.sortExplicit) state.sort = 'collection';
                by('[data-sort]').value = state.sort;
                collectionStrip();
                await loadBooks();
            });
        }

        async function loadCollections() {
            const result = await request(api.collections);
            state.collections = Array.isArray(result.collections) ? result.collections : [];
            state.collections.sort((a, b) => (Number(a.position) || 0) - (Number(b.position) || 0) || String(a.name || '').localeCompare(String(b.name || '')));
        }

        function uploadDialog() {
            let imported = 0;
            let maxBytes = Number(state.capabilities.limits?.max_book_bytes) || 0;
            showDialog(t('upload_books'), t('browser_upload'), `<p>${esc(t('upload_copy_notice'))}</p><p data-upload-limit ${maxBytes > 0 ? '' : 'hidden'}>${esc(t('upload_limit').replace('{limit}', formatBytes(maxBytes)))}</p><label class="bs-upload-drop">${esc(t('choose_files'))}<input type="file" name="files" accept=".epub,.pdf,.mobi,.azw,.azw3,.fb2,.fb2.zip,.cbz,.txt,.txt.gz,.html,.htm,.cbr,.rtf,.docx,.odt,.djvu,.lit,.pdb,.lrf" multiple required><small data-upload-label>${esc(t('no_files_chosen'))}</small></label><p class="bs-form-error" data-form-error role="alert" hidden></p><footer class="bs-dialog-actions"><button type="button" data-dialog-close>${esc(t('cancel'))}</button><button type="submit" class="bs-primary">${esc(t('upload'))}</button></footer>`, async (form, formEl) => {
                const button = formEl.querySelector('[type=submit]');
                if (button.disabled) return;
                const input = formEl.querySelector('[name=files]');
                const files = Array.from(input.files || []);
                const errorHost = formEl.querySelector('[data-form-error]');
                errorHost.hidden = true;
                if (!files.length) throw new Error(t('choose_files'));
                const oversized = files.slice(imported).find(file => maxBytes > 0 && file.size > maxBytes);
                if (oversized) throw new Error(t('upload_too_large').replace('{name}', oversized.name).replace('{limit}', formatBytes(maxBytes)));
                button.disabled = true;
                input.disabled = true;
                try {
                    while (imported < files.length) {
                        const file = files[imported];
                        button.textContent = t('uploading_count').replace('{done}', String(imported)).replace('{total}', String(files.length));
                        const payload = new FormData();
                        payload.append('file', file, file.name);
                        try {
                            await request(api.upload, payload);
                        } catch (error) {
                            if (state.disposed || !input.isConnected || !by('[data-dialog]').open) return;
                            const code = error?.body?.code || error?.body?.error;
                            const tooLarge = error?.status === 413 || code === 'limit_exceeded';
                            const responseLimit = Number(error?.body?.max_bytes);
                            if (tooLarge && Number.isSafeInteger(responseLimit) && responseLimit > 0) {
                                maxBytes = responseLimit;
                                const hint = formEl.querySelector('[data-upload-limit]');
                                hint.hidden = false;
                                hint.textContent = t('upload_limit').replace('{limit}', formatBytes(maxBytes));
                            }
                            const key = tooLarge ? (maxBytes > 0 ? 'upload_too_large' : 'upload_limit_exceeded') : 'upload_failed';
                            errorHost.textContent = t(key).replace('{name}', file.name).replace('{limit}', formatBytes(maxBytes));
                            if (imported) errorHost.textContent += ' ' + t('upload_partial').replace('{done}', String(imported)).replace('{total}', String(files.length));
                            errorHost.hidden = false;
                            return;
                        }
                        imported++;
                        if (state.disposed || !input.isConnected || !by('[data-dialog]').open) return;
                    }
                    closeDialog();
                    inform(t('upload_complete').replace('{count}', String(imported)));
                } finally {
                    button.disabled = false;
                    button.textContent = t('upload');
                    input.disabled = false;
                    if (imported && !state.disposed) await loadBooks();
                }
            });
            const input = by('[data-dialog] [name=files]');
            input.addEventListener('change', () => {
                imported = 0;
                by('[data-form-error]').hidden = true;
                by('[data-upload-label]').textContent = Array.from(input.files || []).map(file => file.name).join(', ') || t('no_files_chosen');
            });
        }

        function addBooksDialog() {
            showDialog(t('add_books'), t('library'), `<div class="bs-add-choices"><button type="button" data-add-choice="source"><span>▤</span><strong>${esc(t('connect_source'))}</strong><small>${esc(t('connect_source_copy'))}</small></button><button type="button" data-add-choice="upload"><span>↑</span><strong>${esc(t('upload_books'))}</strong><small>${esc(t('upload_copy_notice'))}</small></button></div><footer class="bs-dialog-actions"><button type="button" data-dialog-close>${esc(t('close'))}</button></footer>`, null);
        }

        async function beginRead(book, format) {
            if (state.busy || state.disposed || !book) return;
            if (appDisabled()) { inform(t('disabled'), true); return; }
            if (book.available === false) { inform(t('book_unavailable'), true); return; }
            if (!format && !canReadDirectly(book) && !(book.renditions || []).some(item => String(item.format || '').toLowerCase() === 'epub' && item.available !== false)) {
                inform(t(state.capabilities.converter?.available ? 'convert_book' : 'conversion_unavailable'), true);
                return;
            }
            state.focusedBookId = book.id;
            state.shelfScrollTop = by('.bs-shelf-scroll').scrollTop;
            state.selected = book;
            state.busy = true;
            inform(t('preparing_book'));
            try {
                const prepared = await request(api.prepare, book.id, format || '');
                const url = new URL(prepared.content_url || '', window.location.origin);
                    if (url.origin !== window.location.origin || !url.pathname.startsWith('/api/desktop/bookshelf/books/')) throw new Error(t('reader_unavailable'));
                if (!window.BookshelfReader || typeof window.BookshelfReader.create !== 'function') throw new Error(t('reader_unavailable'));
                const preparedBook = Object.assign({}, book, prepared.format ? { format: prepared.format } : {});
                await mountReader(preparedBook, url.href, prepared.progress || book.progress || null, prepared.revision || '');
                inform('');
            } catch (error) {
                if (!state.disposed && error.name !== 'AbortError') inform(readableError(error, t('reader_unavailable')), true);
            } finally {
                state.busy = false;
            }
        }

        async function mountReader(book, contentURL, progress, renditionRevision) {
            state.page = 'reader';
            state.readerToken++;
            const token = state.readerToken;
            state.renditionRevision = renditionRevision || '';
            state.readerReady = false;
            state.readerData = null;
            state.thumbnailCount = 48;
            state.readerPrefs = readPrefs(book.id);
            const area = by('[data-reader]');
            area.hidden = false;
            root.dataset.page = 'reader';
            area.innerHTML = `<header class="bs-reader-toolbar"><button type="button" data-action="back" aria-label="${esc(t('back_to_library'))}">← <span>${esc(t('library'))}</span></button><strong>${esc(book.title || t('untitled'))}</strong><span class="bs-reader-save-state" role="status"><span data-save-state>${esc(t('progress_saved'))}</span><button type="button" data-progress-retry hidden>${esc(t('retry_save'))}</button></span><div><button type="button" data-reader-action="prev" aria-label="${esc(t('previous_page'))}">‹</button><button type="button" data-reader-action="next" aria-label="${esc(t('next_page'))}">›</button><button type="button" data-reader-action="toc">☷ ${esc(t('contents'))}</button><button type="button" data-reader-action="thumbnails" hidden>▦ ${esc(t('thumbnails'))}</button><button type="button" data-reader-action="search">⌕ ${esc(t('find_in_book'))}</button><button type="button" data-reader-action="annotations">☆ ${esc(t('notes_bookmarks'))}</button><button type="button" data-reader-action="settings">A ${esc(t('reading_settings'))}</button><button type="button" data-reader-action="fullscreen" aria-label="${esc(t('fullscreen'))}">⛶</button></div></header><div class="bs-reader-workspace"><aside class="bs-reader-panel" data-reader-panel hidden></aside><div class="bs-reader-host" data-reader-host aria-label="${esc(t('reader'))}"></div></div>`;
            const options = {
                book, contentURL, progress, settings: readerSettings(state.readerPrefs), t, signal: state.abort.signal,
                onPassword: reason => passwordDialog(reason),
                onProgress: value => { if (token === state.readerToken) queueProgress(book, value, renditionRevision); },
                onMetadata: value => { if (token === state.readerToken) state.readerData = Object.assign({}, state.readerData || {}, { metadata: value }); },
                onReady: value => { if (token === state.readerToken) { state.readerData = Object.assign({}, state.readerData || {}, value || {}); state.readerReady = true; state.toc = Array.isArray(value?.toc) ? value.toc : []; const thumbs = by('[data-reader-action="thumbnails"]'); if (thumbs) thumbs.hidden = !value?.isFixedLayout; if (!state.panel) readerPanel('toc'); } },
                onExternalLink: href => { if (token === state.readerToken) externalLinkDialog(href); },
                onError: error => { if (token === state.readerToken) inform(readableError(error, t('reader_error')), true); }
            };
            state.reader = await window.BookshelfReader.create(by('[data-reader-host]'), options);
            if (state.disposed || token !== state.readerToken) state.reader?.dispose?.();
            if (token === state.readerToken && !state.disposed) await loadAnnotations(book, token);
            if (state.ctx.setWindowBeforeClose) state.ctx.setWindowBeforeClose(windowId, async () => {
                if (await flushProgress()) return true;
                return confirmDiscardProgress();
            });
        }

        function passwordDialog(reason) {
            return new Promise(resolve => {
                state.passwordResolver = resolve;
                showDialog(t('pdf_password_title'), t('reader'), `<p>${esc(reason === 2 ? t('pdf_password_invalid') : t('pdf_password_copy'))}</p><label>${esc(t('password'))}<input type="password" name="reader_password" autocomplete="current-password" required></label><p class="bs-form-error" data-form-error role="alert" hidden></p><footer class="bs-dialog-actions"><button type="button" data-dialog-close>${esc(t('cancel'))}</button><button type="submit" class="bs-primary">${esc(t('unlock_book'))}</button></footer>`, async form => {
                    const password = String(form.get('reader_password') || '');
                    const done = state.passwordResolver;
                    state.passwordResolver = null;
                    closeDialog();
                    done?.(password || null);
                });
            });
        }

        function externalLinkDialog(rawHref) {
            let url;
            try {
                url = new URL(String(rawHref || ''));
                if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password) throw new Error('unsupported');
            } catch (_) {
                inform(t('external_link_blocked'), true);
                return;
            }
            showDialog(t('external_link_title'), t('reader'), `<p>${esc(t('external_link_copy'))}</p><code class="bs-external-link-url">${esc(url.href)}</code><footer class="bs-dialog-actions"><button type="button" data-dialog-close>${esc(t('cancel'))}</button><a class="bs-external-link" href="${esc(url.href)}" target="_blank" rel="noopener noreferrer">${esc(t('open_external_link'))} ↗</a></footer>`, null);
        }

        function readPrefs(bookId) {
            try { return Object.assign({}, defaultPrefs, JSON.parse(localStorage.getItem('aurago.bookshelf.reader.' + bookId) || '{}')); } catch (_) { return Object.assign({}, defaultPrefs); }
        }

        function savePrefs(bookId, prefs) {
            state.readerPrefs = Object.assign({}, state.readerPrefs, prefs);
            try { localStorage.setItem('aurago.bookshelf.reader.' + bookId, JSON.stringify(state.readerPrefs)); } catch (_) {}
            state.reader?.setPreferences?.(readerSettings(state.readerPrefs));
        }

        function setProgressStatus(bookID, key, retry = false) {
            if (state.page !== 'reader' || state.selected?.id !== bookID) return;
            const status = by('[data-save-state]');
            if (status) status.textContent = t(key);
            const retryButton = by('[data-progress-retry]');
            if (retryButton) retryButton.hidden = !retry;
        }

        function queueProgress(book, progress, renditionRevision = state.renditionRevision) {
            if (readOnly() || !progress || !progress.locator) return;
            const previous = state.progressQueue.get(book.id);
            const expectedRevision = previous?.expectedRevision ?? state.progressWriter.revisionFor({ bookId: book.id, renditionRevision }, book.progress?.revision ?? 0);
            const pending = {
                book,
                bookId: book.id,
                expectedRevision,
                renditionRevision,
                progress: { rendition_revision: renditionRevision, locator: progress.locator, fraction: Math.max(0, Math.min(1, Number(progress.fraction) || 0)) }
            };
            state.progressQueue.set(book.id, pending);
            setProgressStatus(book.id, 'progress_saving');
            clearTimeout(state.progressTimer);
            state.progressTimer = setTimeout(flushProgress, 1000);
        }

        function restoreProgress(pending) {
            if (!state.progressQueue.has(pending.book.id)) state.progressQueue.set(pending.book.id, pending);
            setProgressStatus(pending.book.id, 'progress_unsaved', true);
        }

        function confirmDiscardProgress() {
            if (!state.progressQueue.size) return Promise.resolve(true);
            return ctx.confirmDialog ? ctx.confirmDialog(t('progress_unsaved'), t('progress_unsaved_copy')) : Promise.resolve(false);
        }

        function flushProgress() {
            clearTimeout(state.progressTimer);
            state.progressTimer = 0;
            if (!state.progressQueue.size || readOnly()) return state.progressWrite.then(() => !state.progressQueue.size);
            const pending = Array.from(state.progressQueue.values());
            pending.forEach(item => state.progressQueue.delete(item.book.id));
            state.progressWrite = state.progressWrite.then(async () => {
                let savedAll = true;
                for (const next of pending) {
                    try {
                        const saved = await state.progressWriter.enqueue(next);
                        const progress = saved.progress || saved;
                        next.book.progress = Object.assign({}, next.book.progress || {}, progress, { revision: progress.revision ?? next.book.progress?.revision });
                        if (state.selected?.id === next.book.id) state.selected.progress = next.book.progress;
                        const newer = state.progressQueue.get(next.book.id);
                        if (newer) {
                            newer.expectedRevision = next.book.progress.revision;
                            setProgressStatus(next.book.id, 'progress_saving');
                        } else {
                            setProgressStatus(next.book.id, 'progress_saved');
                        }
                    } catch (error) {
                        let resolvedConflict = false;
                        if (error.status === 409 || error.status === 412) {
                            let freshProgress = null;
                            try {
                                const fresh = await request(api.book, next.book.id);
                                const freshBook = fresh.book || fresh;
                                freshProgress = freshBook.progress || null;
                                if (freshBook.revision != null) next.book.revision = freshBook.revision;
                                if (freshProgress) {
                                    next.book.progress = freshProgress;
                                    const newer = state.progressQueue.get(next.book.id);
                                    if (newer) newer.expectedRevision = freshProgress.revision ?? 0;
                                }
                                if (state.selected?.id === next.book.id && freshProgress) state.selected.progress = freshProgress;
                            } catch (_) { /* Keep the pending position if the latest revision cannot be read. */ }
                            const sameReader = state.page === 'reader' && state.selected?.id === next.book.id && state.renditionRevision === next.renditionRevision;
                            if (sameReader && freshProgress && freshProgress.rendition_revision === next.renditionRevision) {
                                resolvedConflict = await showProgressConflict(next, freshProgress);
                            } else {
                                inform(t('progress_conflict_copy'), true);
                            }
                        } else if (error.name !== 'AbortError') {
                            inform(readableError(error, t('progress_unsaved_copy')), true);
                        }
                        if (!resolvedConflict) {
                            restoreProgress(next);
                            savedAll = false;
                        }
                    }
                }
                return savedAll && !state.progressQueue.size;
            });
            return state.progressWrite;
        }

        function showProgressConflict(next, saved) {
            return new Promise(resolve => {
                const dialog = by('[data-dialog]');
                let settled = false;
                const finish = value => {
                    if (settled) return;
                    settled = true;
                    dialog.removeEventListener('close', onClose);
                    resolve(value);
                };
                const onClose = () => finish(false);
                showDialog(t('progress_conflict'), t('reader'), `<p>${esc(t('progress_conflict_copy'))}</p><div class="bs-conflict-choices"><button type="button" data-position="saved">${esc(t('use_saved_position'))}</button><button type="button" data-position="current">${esc(t('keep_current_position'))}</button></div>`, null);
                dialog.addEventListener('close', onClose, { once: true });
                dialog.querySelectorAll('[data-position]').forEach(button => button.addEventListener('click', async () => {
                    if (button.dataset.position === 'saved') {
                        state.progressQueue.delete(next.book.id);
                        next.book.progress = saved;
                        if (state.selected?.id === next.book.id) state.selected.progress = saved;
                        state.progressWriter.remember(next.book.id, next.renditionRevision, saved.revision);
                        state.reader?.goTo?.(saved.locator);
                        const pendingAfterJump = state.progressQueue.get(next.book.id);
                        if (pendingAfterJump) pendingAfterJump.expectedRevision = saved.revision ?? 0;
                        setProgressStatus(next.book.id, pendingAfterJump ? 'progress_saving' : 'progress_saved', !!pendingAfterJump);
                        if (pendingAfterJump) state.progressTimer = setTimeout(flushProgress, 1000);
                        finish(true);
                        closeDialog();
                        return;
                    }
                    const queued = state.progressQueue.get(next.book.id) || next;
                    state.progressQueue.delete(next.book.id);
                    queued.expectedRevision = saved.revision ?? 0;
                    queued.book.progress = saved;
                    try {
                        const result = await state.progressWriter.writeNow(queued, saved.revision ?? 0);
                        const progress = result.progress || result;
                        queued.book.progress = Object.assign({}, saved, progress, { revision: progress.revision ?? saved.revision });
                        if (state.selected?.id === queued.book.id) state.selected.progress = queued.book.progress;
                        const newer = state.progressQueue.get(queued.book.id);
                        if (newer) newer.expectedRevision = queued.book.progress.revision;
                        setProgressStatus(queued.book.id, 'progress_saved');
                        finish(true);
                    } catch (error) {
                        restoreProgress(queued);
                        inform(readableError(error, t('progress_unsaved_copy')), true);
                        finish(false);
                    }
                    closeDialog();
                }));
            });
        }

        function readerPanel(which, force = false) {
            const previous = state.panel;
            state.panel = previous === which && !force ? '' : which;
            if (previous === 'thumbnails' && state.panel !== 'thumbnails') disposeThumbnails();
            const panel = by('[data-reader-panel]');
            panel.hidden = !state.panel;
            if (!state.panel) return;
            if (state.panel === 'toc') {
                let renderedItems = 0;
                const renderToc = (items, parentPath = '', depth = 0) => {
                    if (depth >= 16 || renderedItems >= 8000) return '';
                    const entries = [];
                    for (let index = 0; index < items.length && renderedItems < 8000; index++) {
                        renderedItems++;
                        const item = items[index];
                        if (!item || typeof item !== 'object') continue;
                        const path = parentPath ? `${parentPath}.${index}` : String(index);
                        const children = Array.isArray(item.subitems) && item.subitems.length ? renderToc(item.subitems, path, depth + 1) : '';
                        entries.push(`<li><button type="button" data-toc="${path}">${esc(item.label || item.title || t('chapter') + ' ' + (index + 1))}</button>${children}</li>`);
                    }
                    return entries.length ? `<ol>${entries.join('')}</ol>` : '';
                };
                const contents = renderToc(state.toc);
                panel.innerHTML = `<h2>${esc(t('contents'))}</h2>${contents || `<p>${esc(t('contents_unavailable'))}</p>`}`;
            }
            if (state.panel === 'search') panel.innerHTML = `<h2>${esc(t('find_in_book'))}</h2><form data-reader-search><label>${esc(t('search'))}<input type="search" name="query" autofocus></label><button type="submit" class="bs-primary">${esc(t('search'))}</button></form><div data-reader-results></div>`;
            if (state.panel === 'settings') {
                const format = String(state.selected?.format || '').toLowerCase();
                const fixed = state.readerData?.isFixedLayout === true;
                const textDocument = ['txt', 'html', 'htm'].includes(format);
                let controls = '';
                if (fixed) controls = `<label>${esc(t('fit_mode'))}<select data-pref="fit"><option value="page">${esc(t('fit_page'))}</option><option value="width">${esc(t('fit_width'))}</option></select></label><label>${esc(t('zoom'))}<select data-pref="zoom"><option value="auto">${esc(t('zoom_auto'))}</option>${[75, 100, 125, 150, 200, 250].map(value => `<option value="${value}">${value}%</option>`).join('')}</select></label><label class="bs-check"><input type="checkbox" data-pref="spread" ${state.readerPrefs.spread ? 'checked' : ''}><span>${esc(t('two_page_spread'))}</span></label><label>${esc(t('reading_direction'))}<select data-pref="direction"><option value="ltr">${esc(t('direction_ltr'))}</option><option value="rtl">${esc(t('direction_rtl'))}</option></select></label>`;
                else controls = `<label>${esc(t('font_size'))}<input type="range" min="14" max="32" step="1" value="${Number(state.readerPrefs.fontSize)}" data-pref="fontSize"></label><label>${esc(t('line_spacing'))}<input type="range" min="1.2" max="2.2" step="0.05" value="${Number(state.readerPrefs.lineHeight)}" data-pref="lineHeight"></label><label>${esc(t('page_width'))}<select data-pref="width"><option value="narrow">${esc(t('narrow'))}</option><option value="comfortable">${esc(t('comfortable'))}</option><option value="wide">${esc(t('wide'))}</option></select></label><label>${esc(t('page_theme'))}<select data-pref="theme"><option value="paper">${esc(t('paper_theme'))}</option><option value="sepia">${esc(t('sepia_theme'))}</option><option value="dark">${esc(t('dark_theme'))}</option></select></label>`;
                if (textDocument) controls += `<label>${esc(t('text_encoding'))}<select data-pref="encoding"><option value="auto">${esc(t('encoding_auto'))}</option><option value="utf-8">UTF-8</option><option value="windows-1252">Windows-1252</option><option value="windows-1251">Windows-1251</option><option value="utf-16le">UTF-16 LE</option><option value="utf-16be">UTF-16 BE</option></select></label>`;
                panel.innerHTML = `<h2>${esc(t('reading_settings'))}</h2>${controls}`;
                for (const key of ['width', 'theme', 'fit', 'direction', 'encoding']) {
                    const control = panel.querySelector(`[data-pref="${key}"]`);
                    if (control) control.value = state.readerPrefs[key];
                }
                const zoom = panel.querySelector('[data-pref="zoom"]');
                if (zoom) zoom.value = state.readerPrefs.zoom == null ? 'auto' : String(state.readerPrefs.zoom);
            }
            if (state.panel === 'thumbnails') renderThumbnails(panel);
            if (state.panel === 'annotations') {
                panel.innerHTML = `<h2>${esc(t('notes_bookmarks'))}</h2><div class="bs-annotation-actions"><button type="button" data-annotation-add="bookmark">☆ ${esc(t('add_bookmark'))}</button><button type="button" data-annotation-add="highlight">▰ ${esc(t('highlight'))}</button><button type="button" data-annotation-add="note">✎ ${esc(t('add_note'))}</button></div><div class="bs-annotation-list">${state.annotations.length ? state.annotations.map((item, index) => { const currentRendition = Boolean(item.rendition_revision) && item.rendition_revision === state.renditionRevision; return `<article class="${currentRendition ? '' : 'is-historical'}"><button type="button" class="bs-annotation-jump" data-annotation-jump="${index}" ${currentRendition ? '' : 'disabled'}><strong>${esc(t(item.kind || 'bookmark'))}</strong><span>${esc(item.text || (item.locator?.page ? t('page') + ' ' + item.locator.page : t('bookmarked_location')))}</span>${currentRendition ? '' : `<small>${esc(t('annotation_previous_edition'))}</small>`}</button><button type="button" data-annotation-delete="${esc(item.id)}" aria-label="${esc(t('delete_annotation'))}" ${readOnly() ? 'disabled' : ''}>×</button></article>`; }).join('') : `<p>${esc(t('no_annotations'))}</p>`}</div>`;
            }
        }

        function disposeThumbnails() {
            state.thumbnailObserver?.disconnect();
            state.thumbnailObserver = null;
            for (const url of state.thumbnailURLs.values()) URL.revokeObjectURL(url);
            state.thumbnailURLs.clear();
        }
        state.releaseThumbnails = disposeThumbnails;

        function renderThumbnails(panel) {
            const pageCount = Math.max(0, Math.min(10000, Number(state.readerData?.pageCount) || 0));
            const visibleCount = Math.min(pageCount, state.thumbnailCount);
            panel.innerHTML = `<h2>${esc(t('thumbnails'))}</h2>${pageCount ? `<div class="bs-page-thumbnails">${Array.from({ length: visibleCount }, (_, index) => `<button type="button" class="bs-page-thumbnail" data-thumb-page="${index + 1}" aria-label="${esc(t('page'))} ${index + 1}"><img data-thumb-index="${index}" alt=""><span>${index + 1}</span></button>`).join('')}</div>${visibleCount < pageCount ? `<button type="button" class="bs-thumb-more" data-action="thumbnail-more">${esc(t('load_more'))}</button>` : ''}` : `<p>${esc(t('thumbnails_unavailable'))}</p>`}`;
            const imgs = Array.from(panel.querySelectorAll('[data-thumb-index]'));
            for (const img of imgs) {
                const url = state.thumbnailURLs.get(Number(img.dataset.thumbIndex));
                if (url) img.src = url;
            }
            if (typeof IntersectionObserver !== 'function') return;
            state.thumbnailObserver?.disconnect();
            state.thumbnailObserver = new IntersectionObserver(entries => {
                for (const entry of entries) {
                    if (!entry.isIntersecting) continue;
                    const img = entry.target;
                    const index = Number(img.dataset.thumbIndex);
                    state.thumbnailObserver?.unobserve(img);
                    if (state.thumbnailURLs.has(index) || typeof state.reader?.getThumbnail !== 'function') continue;
                    const token = state.readerToken;
                    Promise.resolve(state.reader.getThumbnail(index)).then(blob => {
                        if (!(blob instanceof Blob)) return;
                        const url = URL.createObjectURL(blob);
                        if (state.disposed || token !== state.readerToken || state.panel !== 'thumbnails') { URL.revokeObjectURL(url); return; }
                        state.thumbnailURLs.set(index, url);
                        img.src = url;
                    }).catch(error => { if (!state.disposed && error.name !== 'AbortError') img.classList.add('is-error'); });
                }
            }, { root: panel, rootMargin: '160px' });
            imgs.forEach(img => state.thumbnailObserver.observe(img));
        }

        async function loadAnnotations(book, token) {
            try {
                const result = await request(api.annotations, book.id);
                if (state.disposed || token !== state.readerToken) return;
                state.annotations = Array.isArray(result.annotations) ? result.annotations : [];
                for (const item of state.annotations) {
                    if (item.kind === 'bookmark') continue;
                    if (item.locator && item.rendition_revision && item.rendition_revision === state.renditionRevision) state.reader?.addAnnotation?.(item);
                }
                if (state.panel === 'annotations') { state.panel = ''; readerPanel('annotations'); }
            } catch (error) {
                if (!state.disposed && token === state.readerToken && error.name !== 'AbortError') inform(readableError(error, t('annotations_unavailable')), true);
            }
        }

        async function addAnnotation(kind) {
            const book = state.selected;
            if (!book || readOnly()) return;
            let selection = state.reader?.getSelection?.();
            if (kind !== 'bookmark' && (!selection?.text || !selection.locator)) { inform(t('select_text_first'), true); return; }
            if (kind === 'bookmark') {
                const locator = book.progress?.locator;
                if (!locator) { inform(t('select_text_first'), true); return; }
                selection = { text: '', locator };
            }
            const save = async text => {
                const body = { kind, rendition_revision: state.renditionRevision, locator: selection.locator, text: String(text || selection.text || '').slice(0, 4000), color: '#e4b574' };
                const result = await request(api.createAnnotation, book.id, body);
                const annotation = result.annotation || result;
                state.annotations.push(annotation);
                if (kind !== 'bookmark') state.reader?.addAnnotation?.(annotation);
                state.panel = '';
                readerPanel('annotations');
                inform(t('annotation_added'));
            };
            if (kind === 'note') {
                showDialog(t('add_note'), t('notes_bookmarks'), `<p>${esc(selection.text.slice(0, 500))}</p><label>${esc(t('annotation_text'))}<textarea name="annotation_text" rows="4" maxlength="4000" required></textarea></label><p class="bs-form-error" data-form-error role="alert" hidden></p><footer class="bs-dialog-actions"><button type="button" data-dialog-close>${esc(t('cancel'))}</button><button type="submit" class="bs-primary">${esc(t('save'))}</button></footer>`, async form => {
                    await save(String(form.get('annotation_text') || '').trim());
                    closeDialog();
                });
            } else {
                try { await save(kind === 'highlight' ? selection.text : ''); }
                catch (error) { inform(readableError(error, t('save_failed')), true); }
            }
        }

        async function leaveReader() {
            if (!(await flushProgress())) {
                if (!(await confirmDiscardProgress())) return false;
                state.progressQueue.clear();
            }
            state.readerToken++;
            disposeThumbnails();
            state.reader?.dispose?.();
            state.reader = null;
            state.page = 'library';
            state.panel = '';
            root.dataset.page = 'library';
            by('[data-reader]').hidden = true;
            if (state.ctx.setWindowBeforeClose) state.ctx.setWindowBeforeClose(windowId, null);
            paintShelf();
            requestAnimationFrame(() => {
                by('.bs-shelf-scroll').scrollTop = state.shelfScrollTop;
                const focusable = Array.from(by('[data-shelf]').querySelectorAll('[data-book]')).find(item => item.dataset.book === state.focusedBookId);
                focusable?.focus({ preventScroll: true });
            });
            return true;
        }

        function openFileContext() {
            const path = String(ctx.path || '');
            if (!path || !isBookPath(path)) return;
            if (!ctx.api) return;
            const provider = String(ctx.provider || 'local');
            if (provider !== 'local') {
                const source = state.sources.find(item => item.provider === provider && String(item.mount_id || '') === String(ctx.mount_id || ''));
                if (!source) { inform(t('remote_source_required'), true); return; }
                const clean = value => String(value || '').replace(/\\/g, '/').replace(/^\.\//, '').replace(/^\/+|\/+$/g, '');
                const sourcePath = clean(source.path);
                const remotePath = clean(path);
                const relative = sourcePath && remotePath.startsWith(sourcePath + '/') ? remotePath.slice(sourcePath.length + 1) : (remotePath === sourcePath ? '' : remotePath);
                findSourceBook(source, relative).then(book => book ? openResolvedBook(book) : inform(t('file_not_in_library'), true)).catch(error => { if (!state.disposed) inform(readableError(error, t('file_not_in_library')), true); });
                return;
            }
            const location = { path, provider: ctx.provider || 'local', mount_id: ctx.mount_id || '' };
            request(api.resolve, location).then(result => {
                const book = result.book || result;
                if (book?.id) openResolvedBook(book);
            }).catch(error => { if (!state.disposed) inform(readableError(error, t('file_not_in_library')), true); });
        }

        function openResolvedBook(book) {
            if (canReadDirectly(book)) { beginRead(book); return; }
            state.selected = book;
            state.metadata = book;
            state.details = true;
            loadMetadata(book).finally(() => { if (!state.disposed) { renderDetails(); preview(); } });
        }

        async function findSourceBook(source, path) {
            let offset = 0;
            while (!state.disposed) {
                const page = await request(api.books, { source_id: source.id, offset, limit: 100 });
                const match = (page.books || []).find(book => String(book.path || '').replace(/\\/g, '/') === path);
                if (match) return match;
                offset += (page.books || []).length;
                if (!page.books?.length || offset >= Number(page.total || 0)) return null;
            }
            return null;
        }

        function openEditMetadata() {
            const book = state.metadata || state.selected;
            if (!book) return;
            const fields = ['title', 'author', 'series', 'description'];
            showDialog(t('edit_metadata'), book.title || '', `<div class="bs-form-grid">${fields.map(field => `<label>${esc(t(field))}${field === 'description' ? `<textarea name="${field}" rows="4" maxlength="4000">${esc(book[field] || '')}</textarea>` : `<input name="${field}" maxlength="240" value="${esc(book[field] || '')}">`}</label>`).join('')}<label>${esc(t('tags'))}<input name="tags" maxlength="4000" value="${esc((book.tags || []).join(', '))}"><small>${esc(t('tags_help'))}</small></label></div><p class="bs-form-error" data-form-error role="alert" hidden></p><footer class="bs-dialog-actions"><button type="button" data-dialog-close>${esc(t('cancel'))}</button><button type="submit" class="bs-primary">${esc(t('save'))}</button></footer>`, async form => {
                const values = {};
                fields.forEach(field => { values[field] = String(form.get(field) || '').trim(); });
                values.tags = String(form.get('tags') || '').split(',').map(value => value.trim()).filter(Boolean).slice(0, 128);
                await request(api.updateBook, book.id, values, book.revision);
                closeDialog();
                await loadBooks();
                if (state.selected) await loadMetadata(state.selected);
                inform(t('metadata_saved'));
            });
        }

        function handleClick(event) {
            const orderButton = event.target.closest('[data-collection-order]');
            if (orderButton) {
                if (!orderButton.disabled) moveCollection(orderButton.dataset.collectionId, orderButton.dataset.collectionOrder).catch(error => inform(readableError(error, t('save_failed')), true));
                return;
            }
            const bookOrderButton = event.target.closest('[data-book-order]');
            if (bookOrderButton) {
                if (!bookOrderButton.disabled) {
                    state.focusedBookId = bookOrderButton.dataset.bookId || state.focusedBookId;
                    moveBookInCollection(bookOrderButton.dataset.bookId, bookOrderButton.dataset.bookOrder).catch(error => inform(readableError(error, t('save_failed')), true));
                }
                return;
            }
            const folderEnter = event.target.closest('[data-folder-enter]')?.dataset.folderEnter;
            const folderUse = event.target.closest('[data-folder-use]');
            const folderChooser = state.folderChooser;
            if (folderEnter !== undefined && folderChooser) { folderChooser.current = folderEnter; loadFolderPage(); return; }
            if (event.target.closest('[data-folder-up]') && folderChooser) { folderChooser.current = parentFolder(folderChooser.current); loadFolderPage(); return; }
            if (event.target.closest('[data-folder-retry]') && folderChooser) { loadFolderPage(); return; }
            if (event.target.closest('[data-pick-folder]')) { openInlineFolderPicker(); return; }
            if (folderUse && folderChooser) {
                const selectedPath = folderChooser.current || '';
                state.folderChooser = null;
                if (folderChooser.inline) {
                    by('[data-dialog] [name=path]').value = selectedPath;
                    by('[data-dialog] [name=path_selected]').value = 'true';
                    by('[data-dialog] [data-folder-help]').textContent = selectedPath || t('root_folder');
                    by('[data-dialog] [data-folder-picker]').hidden = true;
                    const preview = by('[data-selected-folder-preview]');
                    if (preview) { preview.innerHTML = folderPreviewMarkup(folderChooser.items, by('[data-dialog] [name=recursive]')?.checked); preview.hidden = false; }
                } else {
                    closeDialog();
                    folderChooser.resolve(selectedPath);
                }
                return;
            }
            const action = event.target.closest('[data-action]')?.dataset.action;
            const bookId = event.target.closest('[data-book]')?.dataset.book;
            const readId = event.target.closest('[data-read]')?.dataset.read;
            const detailsId = event.target.closest('[data-details]')?.dataset.details;
            const collectionId = event.target.closest('[data-collection]')?.dataset.collection;
            const scanId = event.target.closest('[data-scan]')?.dataset.scan;
            const removeId = event.target.closest('[data-remove-source]')?.dataset.removeSource;
            const rootId = event.target.closest('[data-source-root]')?.dataset.sourceRoot;
            const refreshId = event.target.closest('[data-source-refresh]')?.dataset.sourceRefresh;
            const readerAction = event.target.closest('[data-reader-action]')?.dataset.readerAction;
            const addChoice = event.target.closest('[data-add-choice]')?.dataset.addChoice;
            if (bookId) { state.focusedBookId = bookId; const book = state.books.find(item => item.id === bookId); if (book) { state.selected = book; state.metadata = book; if (canReadDirectly(book)) beginRead(book); else { state.details = true; loadMetadata(book); renderDetails(); preview(); } } return; }
            if (readId) { const book = state.selected?.id === readId ? state.selected : state.books.find(item => item.id === readId); beginRead(book, event.target.closest('[data-read]')?.dataset.readFormat); return; }
            const convertId = event.target.closest('[data-convert]')?.dataset.convert;
            if (convertId) { convertBook(convertId).catch(error => inform(readableError(error, t('conversion_failed')), true)); return; }
            if (detailsId) { state.selected = state.books.find(item => item.id === detailsId) || state.selected; state.details = true; loadMetadata(state.selected); renderDetails(); return; }
            if (collectionId !== undefined) {
                state.collectionId = collectionId;
                if (collectionId && !state.sortExplicit) state.sort = 'collection';
                else if (!collectionId && state.sort === 'collection') { state.sort = 'recent'; state.sortExplicit = false; }
                collectionStrip();
                loadBooks();
                return;
            }
            if (scanId) { const source = state.sources.find(item => item.id === scanId); closeDialog(); startScan(source); return; }
            if (rootId) { changeSourceRoot(rootId).catch(error => inform(readableError(error, t('save_failed')), true)); return; }
            if (refreshId) { editSourceRefresh(refreshId); return; }
            if (removeId) { removeSource(removeId).catch(error => inform(readableError(error, t('save_failed')), true)); return; }
            if (readerAction) {
                if (readerAction === 'prev') { Promise.resolve(state.reader?.prev?.()).catch(error => inform(readableError(error, t('reader_error')), true)); return; }
                if (readerAction === 'next') { Promise.resolve(state.reader?.next?.()).catch(error => inform(readableError(error, t('reader_error')), true)); return; }
                if (readerAction === 'fullscreen') { Promise.resolve(root.requestFullscreen?.()).catch(() => {}); return; }
                readerPanel(readerAction); return;
            }
            if (event.target.closest('[data-thumb-page]')) { state.reader?.goTo?.({ page: Number(event.target.closest('[data-thumb-page]').dataset.thumbPage) }); return; }
            if (event.target.closest('[data-action="thumbnail-more"]')) { state.thumbnailCount = Math.min(10000, state.thumbnailCount + 48); readerPanel('thumbnails', true); return; }
            if (addChoice) { addChoice === 'source' ? openAddDialog() : uploadDialog(); return; }
            if (event.target.closest('[data-dialog-close]')) { closeDialog(); return; }
            if (event.target.closest('[data-position]')) return;
            if (event.target.closest('[data-toc]')) {
                let items = state.toc, item;
                for (const index of event.target.closest('[data-toc]').dataset.toc.split('.').map(Number)) {
                    item = Array.isArray(items) ? items[index] : null;
                    if (!item) return;
                    items = item.subitems;
                }
                state.reader?.goTo?.(item.href || item.locator || item);
                return;
            }
            if (event.target.closest('[data-annotation-add]')) { addAnnotation(event.target.closest('[data-annotation-add]').dataset.annotationAdd).catch(error => inform(readableError(error, t('save_failed')), true)); return; }
            if (event.target.closest('[data-annotation-jump]')) {
                const item = state.annotations[Number(event.target.closest('[data-annotation-jump]').dataset.annotationJump)];
                if (item?.locator && item.rendition_revision && item.rendition_revision === state.renditionRevision) state.reader?.goTo?.(item.locator);
                return;
            }
            if (event.target.closest('[data-annotation-delete]')) {
                const button = event.target.closest('[data-annotation-delete]');
                const item = state.annotations.find(annotation => annotation.id === button.dataset.annotationDelete);
                if (!item) return;
                request(api.removeAnnotation, item.id, item.revision).then(async () => {
                    await state.reader?.removeAnnotation?.(item);
                    state.annotations = state.annotations.filter(annotation => annotation.id !== item.id);
                    state.panel = '';
                    readerPanel('annotations');
                    inform(t('annotation_removed'));
                }).catch(error => inform(readableError(error, t('save_failed')), true));
                return;
            }
            if (event.target.closest('[data-remove-membership]')) {
                const collection = event.target.closest('[data-remove-membership]').dataset.removeMembership;
                if (state.selected?.id && collection) request(api.removeFromCollection, collection, state.selected.id).then(() => loadCollections()).then(() => { collectionStrip(); return loadMetadata(state.selected); }).then(() => loadBooks()).then(() => inform(t('collection_updated'))).catch(error => inform(readableError(error, t('save_failed')), true));
                return;
            }
            if (event.target.closest('[data-search-result]')) {
                const result = state.searchResults[Number(event.target.closest('[data-search-result]').dataset.searchResult)];
                if (result?.locator) state.reader?.goTo?.(result.locator);
                return;
            }
            if (event.target.closest('[data-membership]')) return;
            if (event.target.closest('[data-action="library"]')) { state.page = 'library'; state.collectionId = ''; if (state.sort === 'collection') { state.sort = 'recent'; state.sortExplicit = false; } root.dataset.page = 'library'; collectionStrip(); loadBooks(); return; }
            if (event.target.closest('[data-action="collections"]')) { state.page = 'collections'; state.collectionId = ''; if (state.sort === 'collection') { state.sort = 'recent'; state.sortExplicit = false; } root.dataset.page = 'library'; collectionStrip(); loadBooks(); return; }
            if (event.target.closest('[data-action="add"]')) { addBooksDialog(); return; }
            if (event.target.closest('[data-action="sources"]')) { sourcesDialog(); return; }
            if (event.target.closest('[data-action="add-source"]')) { openAddDialog(); return; }
            if (event.target.closest('[data-action="new-collection"]')) { collectionDialog(); return; }
            if (event.target.closest('[data-action="retry"]')) { loadBooks(); return; }
            const pageButton = event.target.closest('[data-action="page"]');
            if (pageButton) { loadBooks(Number(pageButton.dataset.offset) || 0); return; }
            if (event.target.closest('[data-action="details-close"]')) { state.details = false; renderDetails(); return; }
            if (event.target.closest('[data-action="edit-details"]')) { openEditMetadata(); return; }
            if (event.target.closest('[data-pin-toggle]')) { togglePinnedCopy().catch(error => inform(readableError(error, t('save_failed')), true)); return; }
            if (event.target.closest('[data-action="forget-book"]')) { forgetBook().catch(error => inform(readableError(error, t('save_failed')), true)); return; }
            if (event.target.closest('[data-progress-retry]')) { flushProgress(); return; }
            if (event.target.closest('[data-action="back"]')) { leaveReader(); return; }
        }

        function handleInput(event) {
            if (event.target.matches('[data-search]')) {
                state.query = event.target.value;
                clearTimeout(state.searchTimer);
                state.searchTimer = setTimeout(() => loadBooks(), 240);
                return;
            }
            if (event.target.matches('[data-source-filter]')) { state.sourceId = event.target.value; loadBooks(); return; }
            if (event.target.matches('[data-density]')) { state.density = event.target.value; root.dataset.density = state.density; paintShelf(); return; }
            if (event.target.matches('[data-pref]')) {
                const key = event.target.dataset.pref;
                const value = key === 'fontSize' || key === 'lineHeight' ? Number(event.target.value)
                    : key === 'spread' ? event.target.checked
                        : key === 'zoom' ? (event.target.value === 'auto' ? null : Number(event.target.value)) : event.target.value;
                savePrefs(state.selected?.id || '', { [key]: value });
                return;
            }
        }

        function handleSubmit(event) {
            if (!event.target.matches('[data-reader-search]')) return;
            event.preventDefault();
            const query = String(new FormData(event.target).get('query') || '').trim();
            Promise.resolve(state.reader?.search?.(query)).then(results => {
                const hostResults = by('[data-reader-results]');
                if (hostResults) hostResults.innerHTML = (results || []).map((result, index) => `<button type="button" data-search-result="${index}">${esc(result.text || result.label || t('search_result'))}</button>`).join('') || `<p>${esc(t('no_search_results'))}</p>`;
                state.searchResults = results || [];
            }).catch(error => inform(readableError(error, t('search_failed')), true));
        }

        function handleChange(event) {
            if (event.target.matches('[data-pref]')) handleInput(event);
            if (event.target.matches('[data-reader-panel] [data-pref]')) handleInput(event);
            if (event.target.matches('[data-status-filter]')) { state.statusFilter = event.target.value || 'all'; loadBooks(); return; }
            if (event.target.matches('[data-sort]')) { state.sort = event.target.value || 'recent'; state.sortExplicit = true; loadBooks(); return; }
            if (event.target.matches('[data-reading-state]')) {
                const book = state.metadata || state.selected;
                if (!book || readOnly()) return;
                const nextState = event.target.value;
                request(api.updateReadingState, book.id, nextState, book.revision).then(async result => {
                    const updated = result.book || result;
                    Object.assign(book, updated);
                    await loadBooks();
                    if (state.selected) await loadMetadata(state.selected);
                    inform(t('status_updated'));
                }).catch(error => {
                    event.target.value = readingState(book);
                    inform(readableError(error, t('save_failed')), true);
                });
                return;
            }
            if (event.target.matches('[data-membership]')) {
                const id = event.target.value, bookId = state.selected?.id;
                if (id && bookId) request(api.addToCollection, id, bookId).then(() => loadCollections()).then(() => { collectionStrip(); return loadBooks(); }).then(() => inform(t('collection_updated'))).catch(error => inform(readableError(error, t('save_failed')), true));
            }
            if (event.target.matches('[data-cover-upload]')) uploadCover(event.target).catch(error => inform(readableError(error, t('save_failed')), true));
        }

        async function uploadCover(input) {
            const file = input.files?.[0], book = state.selected;
            if (!file || !book) return;
            if (file.size > 8 * 1024 * 1024) throw new Error(t('cover_too_large'));
            const payload = new FormData();
            payload.append('cover', file, file.name);
            await request(api.updateCover, book.id, payload, book.revision);
            await loadMetadata(book);
            await loadBooks();
            inform(t('cover_saved'));
        }

        async function forgetBook() {
            const book = state.selected;
            if (!book) return;
            const accepted = ctx.confirmDialog ? await ctx.confirmDialog(t('forget_book'), t('forget_book_confirm').replace('{title}', book.title || t('untitled'))) : false;
            if (!accepted) return;
            await request(api.removeBook, book.id);
            state.details = false;
            state.selected = null;
            await loadBooks();
            inform(t('book_forgotten'));
        }

        root.addEventListener('click', handleClick);
        root.addEventListener('keydown', handleShelfKeydown);
        root.addEventListener('input', handleInput);
        root.addEventListener('change', handleChange);
        root.addEventListener('submit', handleSubmit);
        by('[data-dialog-close]').addEventListener('click', closeDialog);
        by('[data-density]').addEventListener('change', handleInput);
        by('[data-source-filter]').addEventListener('change', handleInput);
        by('[data-dialog]').addEventListener('click', event => { if (event.target.matches('dialog')) closeDialog(); });
        updateFilters();
        loadInitial();
        const open = async nextContext => {
            if (!nextContext || !nextContext.path || !isBookPath(nextContext.path)) return false;
            if (state.page === 'reader' && !(await leaveReader())) return false;
            Object.assign(ctx, { path: nextContext.path, provider: nextContext.provider || 'local', mount_id: nextContext.mount_id || '' });
            state.details = false;
            openFileContext();
            return true;
        };
        state.open = open;
        return { dispose: () => dispose(windowId), open };

        async function loadInitial() {
            try {
                state.capabilities = await request(api.capabilities);
                root.querySelectorAll('[data-action="add"]').forEach(button => { button.disabled = readOnly() || appDisabled(); });
                if (!state.capabilities.enabled) {
                    root.dataset.page = 'disabled';
                    by('.bs-library-view').innerHTML = `<section class="bs-disabled"><span aria-hidden="true">◧</span><h1>${esc(t('disabled_title'))}</h1><p>${esc(t('disabled_copy'))}</p><small>${esc(state.capabilities.reason || t('enable_in_config'))}</small></section>`;
                    return;
                }
                await Promise.all([loadSources(), loadCollections(), loadCache()]);
                updateFilters();
                collectionStrip();
                await loadBooks();
                if (ctx.path) openFileContext();
            } catch (error) {
                if (!state.disposed) { state.error = readableError(error, t('load_failed')); paintShelf(); }
            }
        }
    }

    function dispose(windowId) {
        const state = instances.get(windowId);
        if (!state) return;
        state.disposed = true;
        state.bookLoadGeneration++;
        state.readerToken++;
        clearTimeout(state.searchTimer);
        clearTimeout(state.progressTimer);
        clearTimeout(state.jobTimer);
        state.abort.abort();
        state.releaseThumbnails?.();
        state.reader?.dispose?.();
        state.reader = null;
        if (state.ctx.setWindowBeforeClose) state.ctx.setWindowBeforeClose(windowId, null);
        instances.delete(windowId);
    }

    function open(windowId, context) {
        return instances.get(windowId)?.open?.(context) || false;
    }

    window.BookshelfApp = { render, dispose, open, instances };
})();
