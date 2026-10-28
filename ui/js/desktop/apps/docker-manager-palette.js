/* Docker Manager command palette: Ctrl/Cmd+K (or the toolbar button) opens a list of commands built from the current
   data: open a stack, deploy/update/restart/stop it, new stack on a host, switch to a host, prepare a host. Typing narrows
   the list (every word must match, a hit at the start of a word ranks higher); arrow keys move, Enter runs, Escape closes.
   A classic script (window.DockerManagerPalette) with no DOM work at load. The palette is a combobox with a listbox
   (aria-activedescendant), not a native dialog, so it sits inside the app; docker-manager.js decides what a command does
   and refuses to open the palette while a dialog is open. */
(function () {
    'use strict';

    function create(deps) {
        const { t, esc, stackModel: sm, views, root, uid } = deps;
        const listId = uid + '-palette-list';
        let panel = null, input = null, listEl = null, liveEl = null, items = [], results = [], index = 0, returnFocus = null, onRun = null;

        const isOpen = () => !!panel;
        const optionId = i => uid + '-palette-opt-' + i;

        function render() {
            const query = input.value;
            results = sm.rankItems(items, query, 40);
            index = Math.min(index, Math.max(0, results.length - 1));
            listEl.innerHTML = results.length ? results.map((item, i) => '<li class="dm-palette-item" role="option" id="' + optionId(i) + '" data-index="' + i + '" aria-selected="' + (i === index) + '">'
                + sm.highlight(item.label, query).map(part => (part.hit ? '<mark>' + esc(part.text) + '</mark>' : esc(part.text))).join('') + '</li>').join('')
                : '<li class="dm-palette-empty" role="presentation">' + esc(t('palette_empty')) + '</li>';
            input.setAttribute('aria-activedescendant', results.length ? optionId(index) : '');
            liveEl.textContent = results.length ? t('palette_count', { count: results.length }) : t('palette_empty');
            listEl.querySelector('[aria-selected="true"]')?.scrollIntoView({ block: 'nearest' });
        }

        function close() {
            if (!panel) return;
            const target = returnFocus;
            panel.remove();
            panel = input = listEl = liveEl = null;
            returnFocus = null;
            if (target && target.isConnected && typeof target.focus === 'function') target.focus({ preventScroll: true });
        }

        function run(item) {
            if (!item) return;
            close();
            if (onRun) onRun(item);
        }

        // getItems() returns the commands of the moment (stackModel.paletteItems); run(item) executes one.
        function open(getItems, runItem) {
            if (panel) return;
            items = getItems();
            onRun = runItem;
            index = 0;
            returnFocus = document.activeElement;
            panel = document.createElement('div');
            panel.className = 'dm-palette-backdrop';
            panel.dataset.palette = '';
            panel.innerHTML = '<div class="dm-palette" role="dialog" aria-modal="true" aria-label="' + esc(t('palette_title')) + '">'
                + '<div class="dm-palette-input">' + views.glyph('command') + '<input type="text" class="dm-palette-field" role="combobox" aria-expanded="true" aria-controls="' + listId + '" aria-autocomplete="list" aria-activedescendant=""'
                + ' autocomplete="off" spellcheck="false" autocapitalize="off" placeholder="' + esc(t('palette_placeholder')) + '" aria-label="' + esc(t('palette_title')) + '"><kbd class="dm-kbd">Esc</kbd></div>'
                + '<ul class="dm-palette-list" role="listbox" id="' + listId + '" aria-label="' + esc(t('palette_title')) + '"></ul>'
                + '<div class="dm-sr-only" role="status" aria-live="polite" data-palette-live></div></div>';
            root.append(panel);
            input = panel.querySelector('input');
            listEl = panel.querySelector('[role="listbox"]');
            liveEl = panel.querySelector('[data-palette-live]');
            render();
            input.focus();
            input.addEventListener('input', () => { index = 0; render(); });
            panel.addEventListener('click', event => {
                const option = event.target.closest('[data-index]');
                if (option) { run(results[Number(option.dataset.index)]); return; }
                if (event.target === panel) close();
            });
            panel.addEventListener('keydown', event => {
                const mod = event.ctrlKey || event.metaKey;
                if (mod && event.key.toLowerCase() === 'k') { event.preventDefault(); event.stopPropagation(); close(); return; }
                const last = results.length - 1;
                const to = { ArrowDown: index >= last ? 0 : index + 1, ArrowUp: index <= 0 ? last : index - 1, Home: 0, End: last }[event.key];
                if (to !== undefined) { event.preventDefault(); index = Math.max(0, to); render(); }
                else if (event.key === 'Enter') { event.preventDefault(); run(results[index]); }
                else if (event.key === 'Escape') { event.preventDefault(); close(); }
                else if (event.key === 'Tab') event.preventDefault();
                // Nothing typed into the palette reaches the keyboard handling of the app behind it.
                event.stopPropagation();
            });
        }

        return { open, close, isOpen, dispose: close };
    }

    window.DockerManagerPalette = { create };
})();
