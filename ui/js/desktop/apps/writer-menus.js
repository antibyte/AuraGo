(function () {
    'use strict';
    const SEP = {type:'separator'};
    // The window menus. item(id) turns a registry command into a menu item; extra carries the items
    // only writer.js can build (cloud import and export in `file`, the recent documents in `recent`).
    // A command no surface offers fails scripts/test-writer-menus.mjs.
    function build(item, tr, extra) {
        return [
            {id:'file',labelKey:'desktop.menu_file',items:[item('new'),item('open'),{id:'recent',label:tr('recent'),items:extra.recent},item('drafts'),SEP,
                item('save'),item('saveAs'),SEP,...extra.file,SEP,
                item('docx'),item('html'),item('md'),item('txt'),SEP,item('pageSetup'),item('print')]},
            {id:'edit',labelKey:'desktop.menu_edit',items:[item('undo'),item('redo'),SEP,item('cut'),item('copy'),item('paste'),item('pastePlain'),SEP,
                item('selectAll'),SEP,item('search'),item('goToPage')]},
            {id:'view',label:tr('view'),items:[item('outline'),item('format'),item('review'),SEP,item('marks'),
                {id:'markup',label:tr('markup'),items:[item('markupAll'),item('markupFinal'),item('markupOriginal')]},SEP,
                {id:'zoom',label:tr('zoom'),items:[item('fit'),item('zoomPage'),SEP,item('zoom50'),item('zoom75'),item('zoom100'),item('zoom125'),item('zoom150'),item('zoom200'),SEP,
                    item('zoomIn'),item('zoomOut')]},
                item('focus')]},
            {id:'insert',label:tr('insert'),items:[item('table'),item('image'),item('qr'),item('link'),item('comment'),SEP,
                item('pageBreak'),item('sectionNext'),item('sectionContinuous'),SEP,item('footnote'),item('endnote'),SEP,item('toc'),item('textBox'),SEP,
                item('header'),item('footer'),{id:'fields',label:tr('fields'),items:[item('fieldPage'),item('pageNumber'),item('fieldPages'),item('fieldSectionPages'),SEP,item('insertDate')]},
                item('headerFirstPage'),item('headerOddEven'),item('backDocument')]},
            {id:'format',label:tr('format'),items:[
                {id:'text',label:tr('text'),items:[item('bold'),item('italic'),item('underline'),item('strike'),SEP,item('superscript'),item('subscript')]},
                {id:'paragraph',label:tr('paragraph'),items:[item('alignLeft'),item('alignCenter'),item('alignRight'),item('alignJustify'),SEP,
                    item('bullets'),item('numbering'),SEP,item('indent'),item('outdent')]},
                {id:'spacing',label:tr('line_spacing'),items:[item('spacing100'),item('spacing115'),item('spacing150'),item('spacing200')]},
                {id:'image',label:tr('image'),items:[item('rotateLeft'),item('rotateRight'),SEP,item('deleteImage')]},SEP,
                item('paragraphFormat'),item('pageSetup'),SEP,
                item('copyFormat'),item('pasteFormat'),item('clearFormat')]},
            {id:'table',label:tr('table'),items:[item('table'),SEP,item('rowAbove'),item('rowBelow'),item('columnLeft'),item('columnRight'),SEP,
                item('deleteRow'),item('deleteColumn'),item('deleteTable'),SEP,item('mergeCells'),item('splitCell'),SEP,item('borders'),item('noBorders')]},
            {id:'tools',label:tr('tools'),items:[item('wordCount'),SEP,item('tracking'),item('review'),item('previousChange'),item('nextChange'),SEP,item('shortcuts')]},
            {id:'agent',labelKey:'desktop.menu_agent',items:[item('assist')]}
        ];
    }
    // app: the state facade of writer.js ({ctx, tr, commands, windowId, editor, menuSignature(), fileMenuItems(), recentMenuItems()}).
    function create(app) {
        const {ctx, tr, commands} = app;
        let signature = null;
        // A menu click leaves the focus in the menu bar; after an engine command it belongs to the document again.
        function item(id) {
            const entry = commands.menuItem(id), definition = commands.get(id), action = entry.action;
            if (definition.slot || definition.command) entry.action = () => { action(); app.editor?.focus(); };
            return entry;
        }
        // The shell reads disabled and checked when the menus are set, and setting them closes an
        // open menu, so the bar is rebuilt only when something it shows has changed.
        function sync(force) {
            const next = app.menuSignature();
            if (!force && next === signature) return;
            signature = next;
            ctx.setWindowMenus?.(app.windowId, build(item, tr, {file: app.fileMenuItems(), recent: app.recentMenuItems()}));
        }
        return {sync};
    }
    // The documents Writer opened last, newest first, without the one that is open. entries is the
    // shell's recent list ({path, appId}); two files of one name are told apart by their folder.
    function recent(entries, current, limit = 8) {
        const list = (entries || []).filter(entry => entry && entry.appId === 'writer' && entry.path && entry.path !== current).slice(0, limit);
        const names = list.map(entry => entry.path.split('/').pop());
        return list.map((entry, index) => {
            const folder = entry.path.split('/').slice(0, -1).join('/');
            const twin = names.indexOf(names[index]) !== names.lastIndexOf(names[index]);
            return {path: entry.path, label: twin && folder ? names[index] + ' · ' + folder : names[index]};
        });
    }
    window.WriterMenus = {create, build, recent};
})();
