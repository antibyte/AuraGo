(function () {
    'use strict';
    // The shared icon atlases carry no keys for text alignment, underline, indent or the
    // colour controls, so Autor draws those few glyphs itself, in the toolbar's line style.
    const GLYPHS = {
        'align-left': '<path d="M4 6h16M4 12h9M4 18h16"/>',
        'align-center': '<path d="M4 6h16M7.5 12h9M4 18h16"/>',
        'align-right': '<path d="M4 6h16M11 12h9M4 18h16"/>',
        'align-justify': '<path d="M4 6h16M4 12h16M4 18h16"/>',
        'format-underline': '<path d="M7 4v8a5 5 0 0 0 10 0V4"/><path d="M6 20h12"/>',
        'indent': '<path d="M4 5h16M4 12h8M4 19h16"/><path d="M12 8l4 4-4 4"/>',
        'outdent': '<path d="M4 5h16M4 12h8M4 19h16"/><path d="M16 8l-4 4 4 4"/>',
        'text-color': '<path d="M12 4L5.5 15M12 4l6.5 11"/><path d="M8.6 10.6h6.8"/><path d="M6 19.5h12" stroke-width="3"/>',
        'highlight': '<path d="M14 4l6 6-9 9H5v-6z"/><path d="M11 7l6 6"/><path d="M6 20.5h12" stroke-width="3"/>'
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
    const STYLE_LABELS = {normal: 'style_normal', title: 'style_title', subtitle: 'style_subtitle', quote: 'style_quote', caption: 'style_caption', nospacing: 'style_no_spacing', listparagraph: 'style_list_paragraph'};
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
        wrap.innerHTML = '<div class="writer-toolbar" role="toolbar" aria-label="' + esc(tr('format_bar')) + '">' +
            GROUPS.map(group => '<div class="writer-toolgroup' + (group.className ? ' ' + group.className : '') + '" data-group="' + group.id + '">' + group.items.map(control).join('') + '</div>').join('') + '</div>';
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
        return {fill, refresh, dispose() { life.abort(); }};
    }
    window.WriterToolbar = {create, icons, groups: GROUPS};
})();
