/* ECalc shell: the top bar with the search box and the settings popover, the category rail (Favorites, Recent, then the
   categories that have items), the item list and the page host. Below 720 px of width the three columns collapse into
   one stack (rail, list, page) with a back button. Search covers the name in the current language, the English name and
   the catalog tags. launch(payload) opens a page with pre-filled inputs (the pre-fill bridge of the agent tool). Publishes
   window.ECalcShell; ecalc.js owns the data and the window lifecycle. */
(function () {
    'use strict';

    const core = window.ECalcCore, Typeset = window.ECalcTypeset;
    const esc = Typeset.escape;
    const COMPACT_BELOW = 720;

    const ICONS = {
        search: '<circle cx="11" cy="11" r="6.5"/><path d="M16 16l4.5 4.5"/>',
        settings: '<path d="M4 7h10M18 7h2M4 17h2M10 17h10"/><circle cx="16" cy="7" r="2"/><circle cx="8" cy="17" r="2"/>',
        back: '<path d="M19 12H6M11 6l-6 6 6 6"/>',
        star: '<path d="M12 3.8l2.5 5.2 5.7.8-4.1 4 1 5.7-5.1-2.7-5.1 2.7 1-5.7-4.1-4 5.7-.8z"/>',
        clock: '<circle cx="12" cy="12" r="8"/><path d="M12 7.5V12l3 2"/>',
        copy: '<rect x="8" y="8" width="11" height="12" rx="2"/><path d="M16 8V6a2 2 0 0 0-2-2H7a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h1"/>',
        reset: '<path d="M5 12a7 7 0 1 0 2.3-5.2"/><path d="M5 4.5V8h3.5"/>',
        lock: '<rect x="6" y="11" width="12" height="9" rx="2"/><path class="ec-shackle" d="M8.5 11V8a3.5 3.5 0 0 1 7 0v3"/>',
        warning: '<path d="M12 4l9 16H3z"/><path d="M12 10v4M12 17v.5"/>'
    };
    const icon = name => '<svg class="ec-glyph" viewBox="0 0 24 24" aria-hidden="true" focusable="false">' + (ICONS[name] || '') + '</svg>';

    // options: {root, catalog, t, store, schematics, decimalComma, uid, launch}; launch is an optional payload that opens the
    // app on a pre-filled page instead of the remembered one.
    function create(options) {
        const { root, catalog, t, store, schematics } = options;
        const units = core.units.create(catalog.units);
        const items = catalog.items;
        const itemById = new Map(items.map(item => [item.id, item]));
        const categories = (catalog.categories || []).filter(category => items.some(item => item.category === category.id));
        const categoryName = new Map((catalog.categories || []).map(category => [category.id, category.name]));
        const ui = { rail: '', query: '', selected: '', view: 'page', compact: false };
        let page = null, popoverOpen = false, observer = null, hostObserver = null;
        const mac = /mac|iphone|ipad/i.test(String(navigator.platform || ''));

        root.innerHTML = '<header class="ec-top">'
            + '<button type="button" class="ec-icon-btn ec-back" data-action="back" hidden aria-label="' + esc(t('desktop.ecalc_back')) + '" title="' + esc(t('desktop.ecalc_back')) + '">' + icon('back') + '</button>'
            + '<label class="ec-search">' + icon('search') + '<input class="ec-search-input" type="search" autocomplete="off" spellcheck="false" placeholder="' + esc(t('desktop.ecalc_search_placeholder')) + '" aria-label="' + esc(t('desktop.ecalc_search_placeholder')) + '"><kbd class="ec-kbd">' + (mac ? '⌘K' : 'Ctrl K') + '</kbd></label>'
            + '<div class="ec-settings-wrap"><button type="button" class="ec-icon-btn" data-action="settings" aria-haspopup="dialog" aria-expanded="false" aria-label="' + esc(t('desktop.ecalc_settings')) + '" title="' + esc(t('desktop.ecalc_settings')) + '">' + icon('settings') + '</button>'
            + '<div class="ec-pop" role="dialog" aria-label="' + esc(t('desktop.ecalc_settings')) + '" hidden>'
            + '<label class="ec-set"><span>' + esc(t('desktop.ecalc_settings_digits')) + '</span><select data-set="sig">' + [3, 4, 5, 6].map(n => '<option value="' + n + '">' + n + '</option>').join('') + '</select></label>'
            + '<label class="ec-set"><span>' + esc(t('desktop.ecalc_eseries_series')) + '</span><select data-set="eseries">' + core.eseries.SERIES.map(name => '<option value="' + name + '">' + name + '</option>').join('') + '</select></label>'
            + '<fieldset class="ec-set"><legend>' + esc(t('desktop.ecalc_settings_symbols')) + '</legend><div class="ec-seg"><label><input type="radio" name="' + options.uid + '-symbols" value="iec"><span>' + esc(t('desktop.ecalc_symbols_iec')) + '</span></label>'
            + '<label><input type="radio" name="' + options.uid + '-symbols" value="ansi"><span>' + esc(t('desktop.ecalc_symbols_ansi')) + '</span></label></div></fieldset></div></div></header>'
            + '<div class="ec-body"><nav class="ec-rail"></nav>'
            + '<section class="ec-list"><h2 class="ec-list-title"></h2><ul class="ec-items"></ul><p class="ec-empty" hidden></p></section>'
            + '<main class="ec-page-host"></main></div>';

        const q = selector => root.querySelector(selector);
        const searchEl = q('.ec-search-input'), railEl = q('.ec-rail'), listEl = q('.ec-list'), itemsEl = q('.ec-items'), emptyEl = q('.ec-empty');
        const titleEl = q('.ec-list-title'), hostEl = q('.ec-page-host'), backEl = q('.ec-back'), popEl = q('.ec-pop'), settingsEl = q('[data-action="settings"]');

        // ---- data ----------------------------------------------------------------------------------------------

        function listed(ids) {
            return ids.map(id => itemById.get(id)).filter(Boolean);
        }
        function search(query) {
            const words = query.toLowerCase().split(/\s+/).filter(Boolean), found = [];
            items.forEach((item, index) => {
                const haystack = [item.name, item.name_en || '', (item.tags || []).join(' '), item.id.replace(/_/g, ' ')].join(' ').toLowerCase();
                if (!words.every(word => haystack.includes(word))) return;
                const name = item.name.toLowerCase();
                found.push({ item, index, rank: name.startsWith(words[0]) ? 0 : name.includes(words[0]) ? 1 : 2 });
            });
            return found.sort((a, b) => a.rank - b.rank || a.index - b.index).map(entry => entry.item);
        }
        function currentItems() {
            if (ui.query.trim()) return search(ui.query);
            if (ui.rail === 'favorites') return listed(store.prefs().favorites);
            if (ui.rail === 'recent') return listed(store.prefs().recents);
            return items.filter(item => item.category === ui.rail);
        }

        // ---- painting ------------------------------------------------------------------------------------------

        function railRows() {
            const rows = [
                { id: 'favorites', label: t('desktop.ecalc_favorites'), count: listed(store.prefs().favorites).length, glyph: 'star' },
                { id: 'recent', label: t('desktop.ecalc_recent'), count: listed(store.prefs().recents).length, glyph: 'clock' }
            ];
            return rows.concat(categories.map(category => ({ id: category.id, label: category.name, count: items.filter(item => item.category === category.id).length, glyph: '' })));
        }
        function paintRail() {
            const searching = !!ui.query.trim();
            railEl.innerHTML = railRows().map((row, index) => (index === 2 ? '<div class="ec-rail-sep" role="separator"></div>' : '')
                + '<button type="button" class="ec-rail-btn" data-rail="' + esc(row.id) + '" title="' + esc(row.label) + '"' + (!searching && ui.rail === row.id ? ' aria-current="true"' : '') + '>'
                + (row.glyph ? icon(row.glyph) : '<span class="ec-glyph ec-glyph-dot"></span>') + '<span class="ec-rail-label">' + esc(row.label) + '</span><span class="ec-count">' + row.count + '</span></button>').join('');
        }
        function paintList() {
            const searching = !!ui.query.trim(), rows = currentItems();
            const row = railRows().find(entry => entry.id === ui.rail);
            titleEl.textContent = searching ? '' : row ? row.label : '';
            titleEl.hidden = searching;
            itemsEl.innerHTML = rows.map(item => '<li><button type="button" class="ec-item" data-id="' + esc(item.id) + '"' + (ui.selected === item.id ? ' aria-current="true"' : '') + '>'
                + '<span class="ec-item-text"><span class="ec-item-name">' + esc(item.name) + '</span>'
                + (searching || ui.rail === 'favorites' || ui.rail === 'recent' ? '<span class="ec-item-sub">' + esc(categoryName.get(item.category) || '') + '</span>' : '') + '</span>'
                + (store.isFavorite(item.id) ? '<span class="ec-item-fav">' + icon('star') + '</span>' : '') + '</button></li>').join('');
            emptyEl.hidden = rows.length > 0;
            emptyEl.textContent = rows.length ? '' : searching ? t('desktop.ecalc_no_results') : ui.rail === 'favorites' ? t('desktop.ecalc_empty_favorites') : ui.rail === 'recent' ? t('desktop.ecalc_empty_recent') : t('desktop.ecalc_no_results');
        }
        function paintSettings() {
            popEl.querySelector('[data-set="sig"]').value = String(store.prefs().sig);
            popEl.querySelector('[data-set="eseries"]').value = store.prefs().eseries;
            for (const radio of popEl.querySelectorAll('input[type="radio"]')) radio.checked = radio.value === store.prefs().symbols;
        }

        function setView(view) {
            ui.view = view;
            root.dataset.view = view;
            backEl.hidden = !(ui.compact && view !== 'rail');
        }
        function setCompact(compact) {
            ui.compact = compact;
            root.dataset.compact = compact ? 'true' : 'false';
            setView(ui.view);
        }

        function mountPage(item) {
            if (page) page.dispose();
            page = window.ECalcCalcPage.create({
                calc: item, units, store, t, icon, schematics, uid: options.uid + '-' + item.id, decimalComma: options.decimalComma,
                onFavorite: () => { paintRail(); paintList(); }
            });
            hostEl.replaceChildren(page.el);
            hostEl.scrollTop = 0;
            page.layout();
        }
        function openItem(id) {
            const item = itemById.get(id);
            if (!item) return;
            ui.selected = id;
            store.pushRecent(id);
            if (!ui.query.trim()) ui.rail = ui.rail === 'favorites' || ui.rail === 'recent' ? ui.rail : item.category;
            mountPage(item);
            paintRail();
            paintList();
            if (ui.compact) setView('page');
        }
        // Opens the page of a launch payload {page, values, choices} (see ECalcCalcPage.launchState). The raw texts of `values`
        // become the inputs of the calculator exactly as if they were typed; a payload with a page id alone just opens the
        // page and keeps what it holds. An unknown page id changes nothing. Returns whether a page was opened.
        function launch(payload) {
            const item = payload && typeof payload === 'object' && typeof payload.page === 'string' ? itemById.get(payload.page) : null;
            if (!item) return false;
            const given = object => !!object && typeof object === 'object' && !Array.isArray(object) && Object.keys(object).length > 0;
            if (item.kind === 'calculator' && (given(payload.values) || given(payload.choices))) {
                const Page = window.ECalcCalcPage;
                store.saveCalc(item.id, Page.launchState(item, payload, units, Page.unitFor(item, store, units)));
            }
            ui.query = '';
            searchEl.value = '';
            openItem(item.id);
            setView('page');
            return true;
        }
        // In the single-column layout the results of a query live in the list: searching from the rail or from a page
        // moves there.
        function showResults() {
            if (ui.compact && ui.query.trim() && ui.view !== 'list') setView('list');
        }
        function pickRail(id) {
            ui.rail = id;
            ui.query = '';
            searchEl.value = '';
            paintRail();
            paintList();
            if (ui.compact) setView('list');
        }

        // ---- events --------------------------------------------------------------------------------------------

        function closePopover() {
            if (!popoverOpen) return;
            popoverOpen = false;
            popEl.hidden = true;
            settingsEl.setAttribute('aria-expanded', 'false');
            document.removeEventListener('pointerdown', onOutside, true);
        }
        function onOutside(event) {
            if (!popEl.contains(event.target) && !settingsEl.contains(event.target)) closePopover();
        }
        function openPopover() {
            popoverOpen = true;
            paintSettings();
            popEl.hidden = false;
            settingsEl.setAttribute('aria-expanded', 'true');
            document.addEventListener('pointerdown', onOutside, true);
        }

        root.addEventListener('click', event => {
            const action = event.target.closest('[data-action]');
            if (action && action.dataset.action === 'settings') { if (popoverOpen) closePopover(); else openPopover(); return; }
            if (action && action.dataset.action === 'back') {
                if (ui.view === 'page') setView('list');
                else { ui.query = ''; searchEl.value = ''; paintRail(); paintList(); setView('rail'); }
                return;
            }
            const rail = event.target.closest('[data-rail]');
            if (rail && railEl.contains(rail)) { pickRail(rail.dataset.rail); return; }
            const item = event.target.closest('.ec-item');
            if (item && itemsEl.contains(item)) openItem(item.dataset.id);
        });
        popEl.addEventListener('change', event => {
            const target = event.target;
            if (target.dataset.set === 'sig') store.set('sig', Number(target.value));
            else if (target.dataset.set === 'eseries') store.set('eseries', target.value);
            else if (target.type === 'radio') store.set('symbols', target.value);
            else return;
            if (page) page.refresh();
        });
        searchEl.addEventListener('input', () => {
            ui.query = searchEl.value;
            paintRail();
            paintList();
            showResults();
        });
        searchEl.addEventListener('keydown', event => {
            if (event.key === 'Escape' && searchEl.value) {
                event.preventDefault();
                event.stopPropagation();
                searchEl.value = '';
                ui.query = '';
                paintRail();
                paintList();
            } else if (event.key === 'Enter') {
                const first = currentItems()[0];
                if (first) openItem(first.id);
            } else if (event.key === 'ArrowDown') {
                const first = itemsEl.querySelector('.ec-item');
                if (first) { event.preventDefault(); first.focus(); }
            }
        });
        // Up and Down move through the rail and the list; the buttons stay ordinary tab stops.
        function arrowKeys(container, selector) {
            container.addEventListener('keydown', event => {
                const keys = { ArrowDown: 1, ArrowUp: -1, Home: 'first', End: 'last' };
                if (!(event.key in keys) || !event.target.matches(selector)) return;
                const buttons = [...container.querySelectorAll(selector)], index = buttons.indexOf(event.target);
                const next = keys[event.key] === 'first' ? 0 : keys[event.key] === 'last' ? buttons.length - 1 : Math.max(0, Math.min(buttons.length - 1, index + keys[event.key]));
                event.preventDefault();
                buttons[next].focus();
            });
        }
        arrowKeys(railEl, '.ec-rail-btn');
        arrowKeys(itemsEl, '.ec-item');
        root.addEventListener('keydown', event => {
            if (event.key === 'Escape' && popoverOpen) { event.preventDefault(); event.stopPropagation(); closePopover(); settingsEl.focus(); }
        });

        const measure = width => setCompact(width > 0 && width < COMPACT_BELOW);
        if (typeof ResizeObserver === 'function') {
            observer = new ResizeObserver(entries => measure(entries[entries.length - 1].contentRect.width));
            observer.observe(root);
        } else {
            measure(root.clientWidth);
        }
        // The page lays itself out for the width of its host, which is not changed by that layout.
        if (typeof ResizeObserver === 'function') {
            hostObserver = new ResizeObserver(() => { if (page) page.layout(); });
            hostObserver.observe(hostEl);
        }

        // ---- start ---------------------------------------------------------------------------------------------

        const remembered = itemById.get(store.prefs().last) || null;
        const first = remembered || items[0] || null;
        ui.rail = first ? first.category : categories.length ? categories[0].id : 'favorites';
        ui.view = remembered ? 'page' : 'rail';
        paintRail();
        paintList();
        setView(ui.view);
        const wanted = options.launch && typeof options.launch.page === 'string' ? itemById.get(options.launch.page) || null : null;
        if (first && !wanted) {
            ui.selected = first.id;
            mountPage(first);
            paintList();
        }
        if (wanted) launch(options.launch);

        return {
            openItem,
            launch,
            focusSearch() { showResults(); searchEl.focus(); searchEl.select(); },
            dispose() {
                closePopover();
                if (observer) observer.disconnect();
                if (hostObserver) hostObserver.disconnect();
                if (page) page.dispose();
                page = null;
                store.flush();
            }
        };
    }

    window.ECalcShell = { create, icon };
})();
