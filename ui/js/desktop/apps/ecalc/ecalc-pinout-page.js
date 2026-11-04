/* ECalc pinout page: the title with favorite and "Copy pinout", the short description, the variant selector (an item with
   `variants`), the view toggle (plug / socket, only for a shape that has two views, with the fixed caption "looking at the
   mating face"), the drawing of ECalcPinoutDraw, the legend of the pin types in use, the detail panel of the selected pin
   (aria-live="polite") and the pin table: number, name, function, type chip, wire swatch (the column is hidden when no pin of
   the variant has a wire) and notes. The table stands beside the drawing when the page is wide and under it when it is narrow.
   A pin is found from both sides: hovering a contact or a row highlights both, a click selects (the row of the selected pin is
   scrolled into view inside the table area, never the page). The drawing is pointer-only (role="img", aria-hidden pins); the
   keyboard path is the button in the first cell of every row. The filter box matches number, name, function, type name, wire
   name and notes, dims the other contacts and hides the other rows. The variant and the view are remembered per item
   (store.saveCalc(item.id, {choices: {variant, view}})).
   options: {item, store, t, icon, uid, onFavorite, pinTypes, wires} (the options object of every page creator, see
   ecalc-shell.js; pinTypes and wires are `pin_types` and `wires` of the catalog); returns {el, layout, refresh, focusFilter, dispose}.
   Publishes window.ECalcPinoutPage = { create }. */
(function () {
    'use strict';

    const Typeset = window.ECalcTypeset, esc = Typeset.escape;
    const NARROW_BELOW = 820; // under this width the table stands under the drawing and the buttons show their icon only
    // The drawing is shown between these scales of its natural size (1 unit = 1 px): a small connector is enlarged, a dense one is
    // shrunk to the width of its host but never below MIN_SCALE, below that it scrolls inside its own area (the page never does).
    const MIN_SCALE = 0.72, MAX_SCALE = 1.6, TALL = 640;
    const COPIED_MS = 1600;
    const oneLine = text => String(text == null ? '' : text).replace(/[\t\r\n]+/g, ' ');

    function create(options) {
        const { item, store, t, icon, uid } = options;
        const Draw = window.ECalcPinoutDraw;
        const shape = item.shape || {};
        const typeInfo = new Map((options.pinTypes || []).map(type => [type.id, type]));
        const wireInfo = new Map((options.wires || []).map(wire => [wire.id, wire]));
        const typeName = id => (typeInfo.get(id) || {}).name || id;
        const wireName = id => (wireInfo.get(id) || {}).name || id;
        const variants = Array.isArray(item.variants) ? item.variants : [];
        const twoViews = !!item.views && (shape.type === 'rows' || shape.type === 'circular');
        const saved = (store.calcState(item.id) || {}).choices || {};
        const st = {
            variant: variants.some(v => v.id === saved.variant) ? saved.variant : (variants[0] ? variants[0].id : ''),
            view: twoViews && (saved.view === 'plug' || saved.view === 'socket') ? saved.view : (shape.drawn === 'socket' ? 'socket' : 'plug'),
            selected: '', hover: '', query: ''
        };
        let copyTimer = 0;
        const pinsNow = () => {
            if (!variants.length) return Array.isArray(item.pins) ? item.pins : [];
            return (variants.find(v => v.id === st.variant) || variants[0]).pins || [];
        };
        const favoriteLabel = () => (store.isFavorite(item.id) ? t('desktop.ecalc_favorite_remove') : t('desktop.ecalc_favorite_add'));
        const paragraphs = text => String(text || '').split(/\n{2,}/).filter(part => part.trim()).map(part => '<p>' + esc(part.trim()) + '</p>').join('');
        const swatch = (type) => {
            const info = typeInfo.get(type) || {};
            return '<svg class="ec-pin-swatch" data-type="' + esc(type) + '" viewBox="0 0 12 12" aria-hidden="true" focusable="false"><rect x="1" y="1" width="10" height="10" rx="2.5" fill="' + esc(Draw.safeColor(info.hex) || '#9e9e9e') + '"/></svg>';
        };
        const wireSwatch = id => {
            const info = wireInfo.get(id) || {}, base = Draw.safeColor(info.hex), stripe = Draw.safeColor(info.stripe);
            if (!base) return '';
            return '<svg class="ec-wire-swatch" data-wire="' + esc(id) + '" viewBox="0 0 24 10" aria-hidden="true" focusable="false"><rect class="ec-wire-base" x="1" y="1" width="22" height="8" rx="3" fill="' + base + '"/>'
                + (stripe ? '<rect class="ec-wire-stripe" x="1" y="4" width="22" height="2.4" fill="' + stripe + '"/>' : '') + '</svg>';
        };

        // ---- markup --------------------------------------------------------------------------------------------
        const el = document.createElement('article');
        el.className = 'ec-page ec-pinout-page';
        el.dataset.pinout = item.id;
        el.dataset.narrow = 'false';
        el.tabIndex = -1;
        el.setAttribute('aria-labelledby', uid + '-title');
        const choice = (name, legend, entries, current, extra) => '<fieldset class="ec-choice" data-choice="' + name + '"><legend>' + esc(legend) + '</legend><div class="ec-seg">'
            + entries.map(entry => '<label><input type="radio" name="' + uid + '-' + name + '" value="' + esc(entry.value) + '"' + (entry.value === current ? ' checked' : '') + '><span>' + esc(entry.label) + '</span></label>').join('') + '</div>' + (extra || '') + '</fieldset>';
        const controls = (variants.length ? choice('variant', t('desktop.ecalc_pin_variant'), variants.map(v => ({ value: v.id, label: v.name })), st.variant) : '')
            + (twoViews ? choice('view', t('desktop.ecalc_pin_view'), [{ value: 'plug', label: item.views.plug }, { value: 'socket', label: item.views.socket }], st.view, '<span class="ec-pin-face">' + esc(t('desktop.ecalc_pin_face')) + '</span>') : '');
        el.innerHTML = '<header class="ec-page-head"><h2 class="ec-title" id="' + uid + '-title">' + esc(item.name) + '</h2><div class="ec-actions">'
            + '<button type="button" class="ec-btn" data-action="favorite" aria-pressed="false">' + icon('star') + '<span class="ec-btn-label" data-label="favorite"></span></button>'
            + '<button type="button" class="ec-btn" data-action="copy" title="' + esc(t('desktop.ecalc_pin_copy')) + '" aria-label="' + esc(t('desktop.ecalc_pin_copy')) + '">' + icon('copy') + '<span class="ec-btn-label" data-label="copy">' + esc(t('desktop.ecalc_pin_copy')) + '</span></button></div></header>'
            + (item.doc ? '<div class="ec-pin-doc">' + paragraphs(item.doc) + '</div>' : '')
            + (controls ? '<div class="ec-pin-controls">' + controls + '</div>' : '')
            + '<div class="ec-pin-main"><section class="ec-stage ec-pin-stage"><div class="ec-pin-art-host"></div><ul class="ec-pin-legend" aria-label="' + esc(t('desktop.ecalc_pin_legend')) + '"></ul>'
            + '<div class="ec-pin-detail" aria-live="polite"></div></section>'
            + '<section class="ec-table-wrap ec-pin-tablewrap"><div class="ec-table-bar"><label class="ec-filter">' + icon('search')
            + '<input class="ec-filter-input" type="search" autocomplete="off" spellcheck="false" maxlength="64" placeholder="' + esc(t('desktop.ecalc_pin_filter')) + '" aria-label="' + esc(t('desktop.ecalc_pin_filter')) + '"><kbd class="ec-kbd">/</kbd></label>'
            + '<span class="ec-table-count" role="status" aria-live="polite"></span></div>'
            + '<div class="ec-table-scroll ec-pin-scroll" role="region" tabindex="0" aria-label="' + esc(item.name) + '"><table class="ec-table ec-pin-table"><caption class="ec-sr">' + esc(item.name) + '</caption><thead></thead><tbody></tbody></table></div></section></div>'
            + (item.notes ? '<section class="ec-table-notes ec-pin-notes"><h3>' + esc(t('desktop.ecalc_table_notes')) + '</h3>' + paragraphs(item.notes) + '</section>' : '')
            + '<p class="ec-source"><strong>' + esc(t('desktop.ecalc_source')) + ':</strong> ' + esc(item.source || '') + '</p>';

        const hostEl = el.querySelector('.ec-pin-art-host'), legendEl = el.querySelector('.ec-pin-legend'), detailEl = el.querySelector('.ec-pin-detail');
        const filterEl = el.querySelector('.ec-filter-input'), countEl = el.querySelector('.ec-table-count');
        const scrollEl = el.querySelector('.ec-pin-scroll'), headEl = el.querySelector('thead'), bodyEl = el.querySelector('tbody');

        // ---- filter, rows ---------------------------------------------------------------------------------------
        const hasWire = () => pinsNow().some(pin => pin.wire);
        const haystack = pin => [pin.number, pin.name, pin.function, typeName(pin.type), pin.wire ? wireName(pin.wire) : '', pin.notes].join('\n').toLowerCase();
        const visiblePins = () => {
            const needle = st.query.trim().toLowerCase();
            return pinsNow().filter(pin => !needle || haystack(pin).includes(needle));
        };

        function renderTable() {
            const wires = hasWire();
            el.dataset.hasWire = wires ? 'true' : 'false';
            headEl.innerHTML = '<tr><th scope="col">' + esc(t('desktop.ecalc_pin_number')) + '</th><th scope="col">' + esc(t('desktop.ecalc_pin_name')) + '</th><th scope="col">' + esc(t('desktop.ecalc_pin_function')) + '</th>'
                + '<th scope="col">' + esc(t('desktop.ecalc_pin_type')) + '</th>' + (wires ? '<th scope="col">' + esc(t('desktop.ecalc_pin_wire')) + '</th>' : '') + '<th scope="col">' + esc(t('desktop.ecalc_pin_notes')) + '</th></tr>';
            const shown = visiblePins();
            bodyEl.innerHTML = shown.length ? shown.map(pin => '<tr data-pin="' + esc(pin.number) + '" data-type="' + esc(pin.type) + '"><th scope="row"><button type="button" class="ec-pin-rowbtn" aria-pressed="false" aria-label="' + esc(t('desktop.ecalc_pin_select', { number: pin.number })) + '">' + esc(pin.number) + '</button></th>'
                + '<td class="ec-pin-cell-name">' + esc(pin.name) + '</td><td>' + esc(pin.function) + '</td>'
                + '<td><span class="ec-pin-chip" data-type="' + esc(pin.type) + '">' + swatch(pin.type) + '<span>' + esc(typeName(pin.type)) + '</span></span></td>'
                + (wires ? '<td>' + (pin.wire ? '<span class="ec-pin-wire">' + wireSwatch(pin.wire) + '<span>' + esc(wireName(pin.wire)) + '</span></span>' : '') + '</td>' : '')
                + '<td class="ec-pin-cell-notes">' + esc(pin.notes || '') + '</td></tr>').join('')
                : '<tr class="ec-table-empty"><td colspan="' + (wires ? 6 : 5) + '">' + esc(t('desktop.ecalc_pin_no_match')) + '</td></tr>';
            const total = pinsNow().length;
            countEl.textContent = st.query.trim() ? t('desktop.ecalc_pin_rows_filtered', { shown: shown.length, total }) : t('desktop.ecalc_pin_rows', { total });
        }

        function renderLegend() {
            const used = [...new Set(pinsNow().map(pin => pin.type))];
            const order = (options.pinTypes || []).map(type => type.id);
            used.sort((a, b) => (order.indexOf(a) + 1 || 99) - (order.indexOf(b) + 1 || 99));
            legendEl.innerHTML = used.map(type => '<li class="ec-pin-legend-item" data-type="' + esc(type) + '">' + swatch(type) + '<span>' + esc(typeName(type)) + '</span></li>').join('');
        }

        function renderDrawing() {
            const pins = pinsNow();
            const view = Draw.layout(shape, pins, { view: st.view });
            const types = {}, wires = {};
            for (const [id, info] of typeInfo) types[id] = { hex: info.hex, ink: info.ink };
            for (const [id, info] of wireInfo) wires[id] = { hex: info.hex, stripe: info.stripe };
            hostEl.innerHTML = Draw.svg({ layout: view, types, wires, label: t('desktop.ecalc_pin_drawing', { name: item.name, count: pins.length }) });
            fitDrawing();
            paintFilter();
            paintMarks();
        }
        function fitDrawing() {
            const art = hostEl.querySelector('svg.ec-pin-art');
            if (!art || !hostEl.clientWidth) return;
            const box = art.viewBox.baseVal, scale = Math.max(MIN_SCALE, Math.min((hostEl.clientWidth - 8) / box.width, MAX_SCALE, Math.max(1, TALL / box.height)));
            art.setAttribute('width', String(Math.round(box.width * scale)));
            art.setAttribute('height', String(Math.round(box.height * scale)));
        }

        function renderDetail() {
            const pin = pinsNow().find(entry => entry.number === st.selected);
            if (!pin) { detailEl.innerHTML = '<p class="ec-pin-hint">' + esc(t('desktop.ecalc_pin_pick')) + '</p>'; return; }
            detailEl.innerHTML = '<div class="ec-pin-detail-head" data-type="' + esc(pin.type) + '"><span class="ec-pin-detail-num">' + esc(pin.number) + '</span><strong class="ec-pin-detail-name">' + esc(pin.name) + '</strong></div>'
                + '<dl class="ec-pin-detail-rows"><dt>' + esc(t('desktop.ecalc_pin_function')) + '</dt><dd>' + esc(pin.function) + '</dd>'
                + '<dt>' + esc(t('desktop.ecalc_pin_type')) + '</dt><dd><span class="ec-pin-chip" data-type="' + esc(pin.type) + '">' + swatch(pin.type) + '<span>' + esc(typeName(pin.type)) + '</span></span></dd>'
                + (pin.wire ? '<dt>' + esc(t('desktop.ecalc_pin_wire')) + '</dt><dd><span class="ec-pin-wire">' + wireSwatch(pin.wire) + '<span>' + esc(wireName(pin.wire)) + '</span></span></dd>' : '')
                + (pin.notes ? '<dt>' + esc(t('desktop.ecalc_pin_notes')) + '</dt><dd>' + esc(pin.notes) + '</dd>' : '') + '</dl>';
        }

        // ---- highlight ------------------------------------------------------------------------------------------
        const pinGroups = () => el.querySelectorAll('g.ec-pin');
        function paintMarks() {
            for (const group of pinGroups()) {
                const number = group.getAttribute('data-pin');
                group.dataset.selected = number === st.selected ? 'true' : 'false';
                group.dataset.hover = number === st.hover ? 'true' : 'false';
            }
            for (const row of bodyEl.querySelectorAll('tr[data-pin]')) {
                const number = row.dataset.pin;
                row.dataset.selected = number === st.selected ? 'true' : 'false';
                row.dataset.hover = number === st.hover ? 'true' : 'false';
                row.querySelector('.ec-pin-rowbtn').setAttribute('aria-pressed', number === st.selected ? 'true' : 'false');
            }
        }
        function paintFilter() {
            const needle = st.query.trim().toLowerCase();
            const hit = new Set(visiblePins().map(pin => pin.number));
            for (const group of pinGroups()) group.dataset.dim = needle && !hit.has(group.getAttribute('data-pin')) ? 'true' : 'false';
        }
        // Puts the row in the middle of the table area when it is not visible there; only the area scrolls.
        function revealRow(row) {
            const area = scrollEl.getBoundingClientRect();
            if (!area.height || scrollEl.scrollHeight <= scrollEl.clientHeight) return;
            const head = headEl.getBoundingClientRect().height, box = row.getBoundingClientRect();
            if (box.top >= area.top + head && box.bottom <= area.bottom) return;
            scrollEl.scrollTop += (box.top - area.top) - Math.max(head, (area.height - box.height) / 2);
        }
        function select(number) {
            st.selected = st.selected === number ? '' : number;
            paintMarks();
            renderDetail();
            const row = st.selected ? bodyEl.querySelector('tr[data-pin="' + CSS.escape(st.selected) + '"]') : null;
            if (row) revealRow(row);
        }
        function hover(number) {
            if (st.hover === number) return;
            st.hover = number;
            paintMarks();
        }

        // ---- copy ------------------------------------------------------------------------------------------------
        function plainText() {
            const wires = hasWire();
            const head = [t('desktop.ecalc_pin_number'), t('desktop.ecalc_pin_name'), t('desktop.ecalc_pin_function'), t('desktop.ecalc_pin_type')].concat(wires ? [t('desktop.ecalc_pin_wire')] : [], [t('desktop.ecalc_pin_notes')]);
            const rows = visiblePins().map(pin => [pin.number, pin.name, pin.function, typeName(pin.type)].concat(wires ? [pin.wire ? wireName(pin.wire) : ''] : [], [pin.notes || '']));
            return [head].concat(rows).map(row => row.map(oneLine).join('\t')).join('\n');
        }

        function paintActions() {
            const button = el.querySelector('[data-action="favorite"]'), on = store.isFavorite(item.id);
            button.setAttribute('aria-pressed', on ? 'true' : 'false');
            button.querySelector('[data-label]').textContent = favoriteLabel();
            button.title = favoriteLabel();
            button.setAttribute('aria-label', favoriteLabel());
        }
        function persist() { store.saveCalc(item.id, { choices: { variant: st.variant, view: st.view } }); }
        function renderAll() {
            renderLegend();
            renderTable();
            renderDrawing();
            if (st.selected && !pinsNow().some(pin => pin.number === st.selected)) st.selected = '';
            paintMarks();
            renderDetail();
        }

        // ---- events ----------------------------------------------------------------------------------------------
        el.addEventListener('pointerover', event => {
            const target = event.target.closest ? event.target.closest('g.ec-pin, tr[data-pin]') : null;
            hover(target && el.contains(target) ? (target.getAttribute('data-pin') || '') : '');
        });
        el.addEventListener('pointerleave', () => hover(''));
        el.addEventListener('change', event => {
            const input = event.target;
            if (!input || input.type !== 'radio') return;
            if (input.name === uid + '-variant') st.variant = input.value;
            else if (input.name === uid + '-view') st.view = input.value;
            else return;
            persist();
            if (input.name === uid + '-variant') renderAll(); else renderDrawing();
        });
        filterEl.addEventListener('input', () => {
            st.query = filterEl.value;
            renderTable();
            paintFilter();
            paintMarks();
        });
        filterEl.addEventListener('keydown', event => {
            if (event.key === 'Escape' && filterEl.value) {
                event.preventDefault();
                event.stopPropagation();
                filterEl.value = '';
                st.query = '';
                renderTable();
                paintFilter();
                paintMarks();
            }
        });
        el.addEventListener('click', event => {
            const pinTarget = event.target.closest('g.ec-pin, tr[data-pin]');
            if (pinTarget && el.contains(pinTarget)) { select(pinTarget.getAttribute('data-pin')); return; }
            const button = event.target.closest('[data-action]');
            if (!button || !el.contains(button)) return;
            if (button.dataset.action === 'favorite') {
                store.toggleFavorite(item.id);
                paintActions();
                if (options.onFavorite) options.onFavorite(item.id);
            } else if (button.dataset.action === 'copy') {
                window.ECalcCalcPage.copyText(plainText());
                const label = button.querySelector('[data-label]');
                button.dataset.done = 'true';
                label.textContent = t('desktop.ecalc_copied');
                button.title = t('desktop.ecalc_copied');
                clearTimeout(copyTimer);
                copyTimer = setTimeout(() => {
                    delete button.dataset.done;
                    label.textContent = t('desktop.ecalc_pin_copy');
                    button.title = t('desktop.ecalc_pin_copy');
                }, COPIED_MS);
            }
        });

        renderAll();
        paintActions();

        return {
            el,
            // The shell calls this whenever the page host changes its size and once the page is attached.
            layout() {
                const width = el.clientWidth;
                if (!width) return;
                el.dataset.narrow = width < NARROW_BELOW ? 'true' : 'false';
                fitDrawing();
            },
            refresh() { paintActions(); },
            focusFilter() {
                filterEl.focus();
                filterEl.select();
            },
            dispose() {
                clearTimeout(copyTimer);
                store.flush();
            }
        };
    }

    window.ECalcPinoutPage = { create };
})();
