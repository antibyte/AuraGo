(function () {
    'use strict';
    // The shared icon atlases carry no keys for text alignment, underline, indent, the
    // colour controls or the overflow button, so Autor draws those few glyphs itself, in the toolbar's line style.
    const GLYPHS = {
        'align-left': '<path d="M4 6h16M4 12h9M4 18h16"/>',
        'align-center': '<path d="M4 6h16M7.5 12h9M4 18h16"/>',
        'align-right': '<path d="M4 6h16M11 12h9M4 18h16"/>',
        'align-justify': '<path d="M4 6h16M4 12h16M4 18h16"/>',
        'format-underline': '<path d="M7 4v8a5 5 0 0 0 10 0V4"/><path d="M6 20h12"/>',
        'indent': '<path d="M4 5h16M4 12h8M4 19h16"/><path d="M12 8l4 4-4 4"/>',
        'outdent': '<path d="M4 5h16M4 12h8M4 19h16"/><path d="M16 8l-4 4 4 4"/>',
        'text-color': '<path d="M12 4L5.5 15M12 4l6.5 11"/><path d="M8.6 10.6h6.8"/><path d="M6 19.5h12" stroke-width="3"/>',
        'highlight': '<path d="M14 4l6 6-9 9H5v-6z"/><path d="M11 7l6 6"/><path d="M6 20.5h12" stroke-width="3"/>',
        'more': '<circle cx="5" cy="12" r="1.3"/><circle cx="12" cy="12" r="1.3"/><circle cx="19" cy="12" r="1.3"/>'
    };
    // icon(key, fallback): a local glyph, else the shell's icon, else the text stand-in.
    function icons(ctx, esc) {
        return (key, fallback) => {
            const shape = GLYPHS[key];
            if (shape) return '<svg class="writer-glyph" viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false">' + shape + '</svg>';
            return ctx.iconMarkup?.(key, fallback, 'writer-icon', 18, 'action') || esc(fallback ?? '');
        };
    }
    // Bar order. A group with a higher priority stays in the bar longer when the window narrows.
    const GROUPS = [
        {id: 'history', priority: 9, items: ['undo', 'redo']},
        {id: 'fonts', priority: 8, className: 'writer-fonts', items: ['style', 'fontFamily', 'fontSize']},
        {id: 'marks', priority: 7, items: ['bold', 'italic', 'underline', 'strike']},
        {id: 'alignment', priority: 4, items: ['alignLeft', 'alignCenter', 'alignRight', 'alignJustify']},
        {id: 'lists', priority: 3, items: ['bullets', 'numbering', 'outdent', 'indent']},
        {id: 'colors', priority: 5, className: 'writer-colors', items: ['textColor', 'highlight']},
        {id: 'insert', priority: 6, className: 'writer-tool-tail', items: ['insert']}
    ];
    // The ids of the groups that must leave a bar of `available` pixels. groups: [{id, priority, width}]
    // in bar order. The lowest priority leaves first; the bar always keeps one group.
    function fit(groups, available, moreWidth, gap) {
        const width = list => list.reduce((sum, group) => sum + group.width, 0) + Math.max(0, list.length - 1) * gap;
        if (width(groups) <= available) return [];
        const hidden = [];
        let visible = groups.slice();
        for (const group of groups.slice().sort((a, b) => a.priority - b.priority)) {
            if (visible.length <= 1 || width(visible) + gap + moreWidth <= available) break;
            visible = visible.filter(candidate => candidate !== group);
            hidden.push(group.id);
        }
        return hidden;
    }
    const STYLE_LABELS ={normal: 'style_normal', title: 'style_title', subtitle: 'style_subtitle', quote: 'style_quote', caption: 'style_caption', nospacing: 'style_no_spacing', listparagraph: 'style_list_paragraph'};
    // app: the state facade of writer.js ({root, tr, esc, ctx, commands, editor, ...}).
    function create(app) {
        const {root, tr, esc, ctx, commands} = app, icon = icons(ctx, esc), colors = window.WriterCommands.colors, life = new AbortController();
        const wrap = root.querySelector('[data-toolbar]');
        // A slot control carries data-slot; an app action carries data-action, which writer.js handles.
        const attrs = id => 'data-tool="' + id + '" ' + (commands.get(id).slot ? 'data-slot="' + commands.get(id).slot + '"' : 'data-action="' + id + '"');
        function control(id) {
            const definition = commands.get(id), name = esc(commands.label(id));
            if (definition.control === 'select') return '<select ' + attrs(id) + ' aria-label="' + name + '"></select>';
            // The one number control is the font size, in points.
            if (definition.control === 'number') return '<input type="number" ' + attrs(id) + ' required min="1" max="200" step=".5" value="11" aria-label="' + name + '">';
            if (definition.control === 'color') return '<label class="writer-color" title="' + name + '">' + icon(definition.icon, '') + '<input type="color" ' + attrs(id) + ' value="' + definition.value + '" aria-label="' + name + '"></label>';
            return '<button type="button" ' + attrs(id) + ' title="' + esc(commands.tooltip(id)) + '" aria-label="' + name + '">' + icon(definition.icon, definition.fallback) + '</button>';
        }
        const moreLabel = esc(tr('toolbar_more')), overflowId = 'writer-toolbar-overflow-' + String(root.dataset.writer).replace(/[^\w-]/g, '-');
        wrap.innerHTML = '<div class="writer-toolbar" role="toolbar" aria-label="' + esc(tr('format_bar')) + '">' +
            GROUPS.map(group => '<div class="writer-toolgroup' + (group.className ? ' ' + group.className : '') + '" data-group="' + group.id + '">' + group.items.map(control).join('') + '</div>').join('') +
            '<button type="button" class="writer-toolbar-more" data-toolbar-more aria-controls="' + overflowId + '" aria-expanded="false" title="' + moreLabel + '" aria-label="' + moreLabel + '" hidden>' + icon('more', '⋯') + '</button></div>' +
            '<div class="writer-toolbar-overflow" id="' + overflowId + '" data-toolbar-overflow role="group" aria-label="' + moreLabel + '" hidden></div>';
        const bar = wrap.querySelector('.writer-toolbar'), more = wrap.querySelector('[data-toolbar-more]'), overflow = wrap.querySelector('[data-toolbar-overflow]');
        const nodes = GROUPS.map(group => ({id: group.id, priority: group.priority, node: wrap.querySelector('[data-group="' + group.id + '"]')}));
        const FIELDS = 'button,select,input';
        function closeOverflow() {
            if (overflow.hidden) return false;
            overflow.hidden = true;
            more.setAttribute('aria-expanded', 'false');
            return true;
        }
        // The bar is one tab stop: the control that last had focus, else the first enabled one.
        function rove(target) {
            const list = [...bar.querySelectorAll(FIELDS)].filter(el => !el.disabled && !el.hidden);
            const current = list.includes(target) ? target : list.find(el => el.tabIndex === 0) || list[0];
            for (const el of bar.querySelectorAll(FIELDS)) el.tabIndex = el === current ? 0 : -1;
        }
        // Put every group back, measure, then move what does not fit into the popover.
        function layout() {
            if (!bar.clientWidth) { closeOverflow(); return; }   // hidden in focus mode: keep the arrangement, close the popover
            const focused = wrap.contains(document.activeElement) ? document.activeElement : null;
            more.hidden = false;
            for (const group of nodes) bar.insertBefore(group.node, more);
            const style = getComputedStyle(bar), gap = parseFloat(style.columnGap) || 0;
            const available = bar.clientWidth - parseFloat(style.paddingLeft) - parseFloat(style.paddingRight);
            const hidden = new Set(fit(nodes.map(group => ({id: group.id, priority: group.priority, width: group.node.offsetWidth})), available, more.offsetWidth, gap));
            for (const group of nodes) if (hidden.has(group.id)) overflow.appendChild(group.node);
            for (const el of overflow.querySelectorAll(FIELDS)) el.tabIndex = 0;
            more.hidden = !hidden.size;
            if (!hidden.size) closeOverflow();
            rove();
            // Moving a node blurs it; give the focus back where it was, or to the More button
            // when the control now sits in the closed popover.
            const landing = focused && overflow.hidden && overflow.contains(focused) ? more : focused;
            if (landing && landing.isConnected && landing !== document.activeElement) landing.focus({preventScroll: true});
        }
        let frame = 0;
        const observer = new ResizeObserver(() => { cancelAnimationFrame(frame); frame = requestAnimationFrame(layout); });
        observer.observe(wrap);
        const valueCommand = el => commands.valueCommand(el.dataset.slot, el.dataset.slot === 'font.size' ? el.valueAsNumber : el.value);
        const unavailable = reason => tr('command_unavailable') + (reason ? ' · ' + reason : '');
        // Tooltips change rarely; writing an unchanged one restarts the browser's tooltip timer.
        const setTitle = (el, title) => { if (el.title !== title) el.title = title; };
        function styleLabel(style) {
            const id = String(style.styleId || '').toLowerCase(), heading = id.match(/^heading([1-9])$/);
            if (heading) return tr('style_heading', {level: heading[1]});
            return STYLE_LABELS[id] ? tr(STYLE_LABELS[id]) : style.name;
        }
        // The style and font lists come from the loaded document.
        function fill() {
            const editor = app.editor, option = (value, text) => '<option value="' + esc(value) + '">' + esc(text) + '</option>';
            wrap.querySelector('[data-slot="styles.style"]').innerHTML = editor.getDocumentStyles().filter(style => style.type === 'paragraph').map(style => option(style.styleId, styleLabel(style))).join('');
            wrap.querySelector('[data-slot="font.family"]').innerHTML = editor.getAvailableFonts().map(font => option(font, font)).join('');
        }
        function refresh() {
            const editor = app.editor;
            if (!editor) return;
            const fmt = editor.snapshot().formatting || {};
            for (const el of wrap.querySelectorAll('[data-slot]')) {
                const id = el.dataset.tool, slot = el.dataset.slot;
                if (el.matches('button')) {
                    const state = commands.state(id);
                    el.disabled = !state.enabled;
                    setTitle(el, state.enabled ? commands.tooltip(id) : commands.tooltip(id) + ' · ' + unavailable(state.reason));
                    if (commands.get(id).toggle) el.setAttribute('aria-pressed', String(state.active));
                    continue;
                }
                if (el.type === 'color') {
                    const hex = slot === 'text.color' ? colors.hex(fmt.color) : colors.highlightHex(fmt.highlight);
                    if (hex && el !== document.activeElement) el.value = hex;
                } else if (el !== document.activeElement) {
                    el.value = slot === 'styles.style' ? fmt.styleId || 'Normal' : slot === 'font.family' ? fmt.fontFamily || 'Calibri' : fmt.fontSizePt || 11;
                } else continue;
                const allowed = commands.can(valueCommand(el));
                el.disabled = !allowed.ok;
                setTitle(el, allowed.ok ? '' : unavailable(allowed.reason));
            }
            rove();
        }
        wrap.addEventListener('click', event => {
            const el = event.target.closest('button[data-slot]');
            if (el && !el.disabled) commands.invoke(el.dataset.tool);
        }, {signal: life.signal});
        wrap.addEventListener('change', event => {
            const el = event.target;
            if (!el.dataset.slot) return;
            if (!el.checkValidity()) { el.reportValidity(); return; }
            commands.run(valueCommand(el));
        }, {signal: life.signal});
        more.addEventListener('click', () => {
            overflow.hidden = !overflow.hidden;
            more.setAttribute('aria-expanded', String(!overflow.hidden));
        }, {signal: life.signal});
        document.addEventListener('pointerdown', event => { if (!wrap.contains(event.target)) closeOverflow(); }, {signal: life.signal});
        bar.addEventListener('focusin', event => { if (event.target.matches(FIELDS)) rove(event.target); }, {signal: life.signal});
        // Buttons: Left and Right (wrapping), Home and End move through the bar. Select and colour
        // fields: Left and Right move, Up, Down, Home and End stay native. The size field keeps all
        // four keys for its caret; there Tab and Shift+Tab step to the neighbouring control, without
        // wrapping, so the roving tab stop never traps the keyboard.
        bar.addEventListener('keydown', event => {
            if (event.altKey || event.ctrlKey || event.metaKey) return;
            const list = [...bar.querySelectorAll(FIELDS)].filter(el => !el.disabled && !el.hidden), index = list.indexOf(event.target);
            if (index < 0) return;
            if (event.target.matches('input[type="number"]')) {
                const step = event.key === 'Tab' ? (event.shiftKey ? -1 : 1) : 0, neighbour = list[index + step];
                if (!step || !neighbour) return;
                event.preventDefault();
                rove(neighbour);
                neighbour.focus();
                return;
            }
            if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return;
            const edge = event.key === 'Home' || event.key === 'End';
            if (edge && !event.target.matches('button')) return;
            event.preventDefault();
            const next = event.key === 'Home' ? list[0] : event.key === 'End' ? list.at(-1) : list[(index + (event.key === 'ArrowRight' ? 1 : -1) + list.length) % list.length];
            rove(next);
            next.focus();
        }, {signal: life.signal});
        // Escape closes the popover and returns to its button; false when it was not open.
        function escape() {
            if (!closeOverflow()) return false;
            more.focus();
            return true;
        }
        return {fill, refresh, escape, dispose() { life.abort(); observer.disconnect(); cancelAnimationFrame(frame); }};
    }
    window.WriterToolbar = {create, icons, groups: GROUPS, fit};
})();
