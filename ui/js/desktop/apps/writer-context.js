(function () {
    'use strict';
    // What the caret is in decides the contextual bar: a selected image wins over a header or
    // footer, which wins over a table.
    function kind(snapshot) {
        if (!snapshot) return '';
        if (snapshot.image) return 'image';
        if (snapshot.scope?.kind === 'headerFooter') return 'headerFooter';
        if (snapshot.table) return 'table';
        return '';
    }
    // Bar contents by kind: command ids, '|' for a divider. `text` lists the ids drawn with
    // their label instead of an icon. A header or footer bar is titled by what is being edited.
    const BARS = {
        table: {items: ['rowAbove', 'rowBelow', 'columnLeft', 'columnRight', '|', 'deleteRow', 'deleteColumn', 'deleteTable', '|', 'mergeCells', 'splitCell', '|', 'borders', 'noBorders']},
        image: {items: ['rotateLeft', 'rotateRight', '|', 'deleteImage', 'format'], text: ['deleteImage', 'format']},
        headerFooter: {items: ['backDocument', '|', 'fieldPage', 'pageNumber', 'fieldPages', 'insertDate', '|', 'headerFirstPage', 'headerOddEven'],
            text: ['fieldPage', 'pageNumber', 'fieldPages', 'insertDate', 'headerFirstPage', 'headerOddEven', 'backDocument']}
    };
    // The right-click menu as sections. An entry is a registry id, '@name' for an item only the
    // app can build, or {submenu, items}. target: {review, link, table, image, toc, scope}.
    function sections(target) {
        return [
            target.review ? ['@accept', '@reject'] : [],
            ['cut', 'copy', 'paste', 'pastePlain'],
            target.link ? ['link', '@unlink'] : ['link'],
            ['comment'],
            ['paragraphFormat'],
            target.table ? [{submenu: 'table', items: BARS.table.items}] : [],
            target.image ? ['rotateLeft', 'rotateRight', 'deleteImage'] : [],
            target.toc ? ['@updateToc'] : [],
            target.scope === 'headerFooter' ? ['backDocument'] : [],
            ['assist']
        ];
    }
    // The shell draws an unknown icon key as text, so every menu item names an icon that exists.
    const ICONS = {pastePlain: 'paste', rowAbove: 'arrow-up', rowBelow: 'arrow-down', columnLeft: 'columns', columnRight: 'columns', deleteRow: 'trash',
        deleteColumn: 'trash', deleteTable: 'trash', mergeCells: 'columns', splitCell: 'columns', borders: 'columns', noBorders: 'columns',
        rotateLeft: 'undo', rotateRight: 'redo', backDocument: 'undo'};
    const PRESSABLE = new Set(['headerFirstPage', 'headerOddEven', 'format']);
    // app: the state facade of writer.js ({root, tr, esc, ctx, commands, editor, readonly, notice, refresh}).
    function create(app) {
        const {root, tr, esc, ctx, commands} = app, icon = window.WriterToolbar.icons(ctx, esc), life = new AbortController();
        const bar = root.querySelector('[data-contextbar]'), mount = root.querySelector('[data-editor]');
        let shown = '';
        function button(id, text) {
            const definition = commands.get(id), name = esc(commands.label(id));
            return '<button type="button" data-tool="' + id + '" title="' + esc(commands.tooltip(id)) + '" aria-label="' + name + '"' + (text ? ' class="writer-context-text"' : '') + '>' +
                (text ? name : icon(definition.glyph || definition.icon, definition.fallback || '')) + '</button>';
        }
        function heading(current) {
            if (current === 'headerFooter') return app.editor.getHeaderFooterState()?.editing === 'footer' ? tr('footer') : tr('header');
            return current === 'image' ? tr('image') : tr('table');
        }
        // Builds the bar when the context changes and keeps its buttons in step with the registry.
        function refresh() {
            const editor = app.editor, current = editor ? kind(editor.snapshot()) : '', title = current ? heading(current) : '';
            if (current + ':' + title !== shown) {
                shown = current + ':' + title;
                bar.hidden = !current;
                bar.setAttribute('aria-label', title);
                bar.innerHTML = !current ? '' : '<strong>' + esc(title) + '</strong>' + BARS[current].items.map(id => id === '|'
                    ? '<span class="writer-context-divider" aria-hidden="true"></span>' : button(id, (BARS[current].text || []).includes(id))).join('');
            }
            for (const el of bar.querySelectorAll('button[data-tool]')) {
                const id = el.dataset.tool, state = commands.state(id);
                el.disabled = !state.enabled;
                if (PRESSABLE.has(id)) el.setAttribute('aria-pressed', String(state.active));
                const tip = commands.tooltip(id) + (!state.enabled && state.reason ? ' · ' + state.reason : '');
                if (el.title !== tip) el.title = tip;
            }
        }
        bar.addEventListener('click', event => {
            const el = event.target.closest('button[data-tool]');
            if (el && !el.disabled) commands.invoke(el.dataset.tool);
        }, {signal: life.signal});
        // On macOS the context click is Ctrl+click, a left press.
        const mac = /Mac|iPhone|iPad/.test(navigator.platform || '');
        const contextPress = event => event.button === 2 || (mac && event.button === 0 && event.ctrlKey);
        // The engine moves its caret on a context press like on a left one, which would drop the
        // selection the menu is about. With a selection or a selected image the press is kept
        // from the engine; with a plain caret the caret follows the click.
        mount.addEventListener('pointerdown', event => {
            const snapshot = contextPress(event) && app.editor ? app.editor.snapshot() : null;
            if (!snapshot || (snapshot.selectionCollapsed && !snapshot.image)) return;
            event.preventDefault();
            event.stopPropagation();
        }, {capture: true, signal: life.signal});
        // What the right-click is about. The engine has already noted a table of contents under the pointer.
        function target(point) {
            const editor = app.editor, snapshot = editor.snapshot();
            const hit = point ? editor.getReviewItemsAt?.(point[0], point[1])[0]?.placement : null;
            return {
                review: hit && hit.kind === 'revision' && !hit.readOnly && !app.readonly ? hit.key : '',
                link: !!editor.query({type: 'hyperlinkAt'}), table: !!snapshot.table, image: !!snapshot.image,
                toc: !app.readonly && snapshot.tocContext?.id || '', scope: snapshot.scope?.kind || 'body'
            };
        }
        function review(key, accept) {
            const result = accept ? app.editor.acceptReviewItem(key) : app.editor.rejectReviewItem(key);
            if (!result.ok) app.notice(tr('command_unavailable') + (result.reason ? ' · ' + result.reason : ''), true);
            app.refresh();
        }
        const custom = {
            '@accept': at => ({id: 'accept', label: tr('accept'), icon: 'check', action: () => review(at.review, true)}),
            '@reject': at => ({id: 'reject', label: tr('reject'), icon: 'undo', action: () => review(at.review, false)}),
            '@unlink': () => ({id: 'unlink', label: tr('link_remove'), icon: 'link', disabled: app.readonly, action: () => commands.run({type: 'removeHyperlink'})}),
            '@updateToc': at => ({id: 'updateToc', label: tr('toc_update'), icon: 'list', action: () => commands.run({type: 'refreshToc', tocId: at.toc})})
        };
        // A registry command as a context-menu item. The focus returns to the document after an engine command.
        function command(id) {
            const entry = commands.menuItem(id), definition = commands.get(id), action = entry.action;
            entry.icon = entry.icon || ICONS[id] || 'edit';
            if (definition.slot || definition.command) entry.action = () => { action(); app.editor?.focus(); };
            return entry;
        }
        function item(entry, at) {
            if (typeof entry !== 'string') return {id: entry.submenu, label: tr('table'), icon: 'columns', items: entry.items.map(id => id === '|' ? {separator: true} : command(id))};
            if (!entry.startsWith('@')) return command(entry);
            const made = custom[entry](at), action = made.action;
            made.action = () => { action(); app.editor?.focus(); };
            return made;
        }
        // The engine acts on a selected image only, and a right-click does not select one. A left
        // press at the same point is the engine's own way to select it.
        function select(drawing, point) {
            const at = {clientX: point[0], clientY: point[1], bubbles: true, cancelable: true, composed: true, button: 0, pointerId: 1, pointerType: 'mouse', isPrimary: true};
            for (const type of ['pointerdown', 'mousedown', 'pointerup', 'mouseup', 'click'])
                drawing.dispatchEvent(new (type.startsWith('pointer') ? PointerEvent : MouseEvent)(type, Object.assign({buttons: type.endsWith('down') ? 1 : 0}, at)));
        }
        // The selection lands with the engine's next update; the menu waits for it, briefly.
        function whenSelected(id, done, tries) {
            if (life.signal.aborted) return;
            if (!app.editor || app.editor.snapshot().image?.id === id || !tries) return done();
            setTimeout(() => whenSelected(id, done, tries - 1), 20);
        }
        // Opens the menu for a contextmenu event inside the document; false when it is not Writer's to answer.
        function open(event) {
            if (!app.editor || !ctx.showContextMenu || !mount.contains(event.target)) return false;
            // Shift+F10 and the menu key arrive without a pointer position.
            const keyboard = event.button === -1 || (!event.clientX && !event.clientY), box = mount.getBoundingClientRect();
            const point = keyboard ? null : [event.clientX, event.clientY], drawing = point && event.target.closest?.('.docx-drawing');
            const present = () => {
                if (life.signal.aborted || !app.editor) return;
                const at = target(point), items = [];
                for (const section of sections(at)) {
                    if (!section.length) continue;
                    if (items.length) items.push({separator: true});
                    items.push(...section.map(entry => item(entry, at)));
                }
                ctx.showContextMenu(point ? point[0] : box.left + Math.min(120, box.width / 2), point ? point[1] : box.top + 60, items);
            };
            // Another image than the selected one is selected first, so the menu is about what was clicked.
            const id = drawing ? drawing.dataset.drawingNodeId : '';
            if (id && app.editor.snapshot().image?.id !== id) { select(drawing, point); whenSelected(id, present, 10); }
            else present();
            return true;
        }
        mount.addEventListener('contextmenu', event => {
            if (!open(event)) return;
            event.preventDefault();
            event.stopPropagation();
        }, {signal: life.signal});
        return {refresh, open, dispose() { life.abort(); }};
    }
    window.WriterContext = {create, kind, bars: BARS, sections};
})();
