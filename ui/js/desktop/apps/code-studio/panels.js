    function splitEditor(direction) {
        if (!state) return;
        const root = studioRoot();
        if (!root) return;
        const editor = shellPart('[data-editor]');
        if (!editor) return;
        if (state.splitMode === direction) {
            state.splitMode = null;
            state.splitRatio = 0.5;
            editor.classList.remove('code-studio-split');
            editor.style.gridTemplateColumns = '';
            editor.style.gridTemplateRows = '';
            const panes = editor.querySelectorAll('.code-studio-split-pane');
            panes.forEach(pane => {
                while (pane.firstChild) editor.appendChild(pane.firstChild);
                pane.remove();
            });
            const divider = editor.querySelector('.code-studio-split-divider');
            if (divider) divider.remove();
            renderEditor();
            return;
        }
        state.splitMode = direction;
        const tab = activeTab();
        if (!tab) return;
        const currentView = tab.view;
        editor.innerHTML = '';
        const isHorizontal = direction === 'right';
        const pane1 = document.createElement('div');
        pane1.className = 'code-studio-split-pane';
        const pane2 = document.createElement('div');
        pane2.className = 'code-studio-split-pane';
        const divider = document.createElement('div');
        divider.className = 'code-studio-split-divider';
        if (isHorizontal) {
            editor.style.gridTemplateColumns = `${state.splitRatio}fr 4px ${1 - state.splitRatio}fr`;
            editor.style.gridTemplateRows = '1fr';
        } else {
            editor.style.gridTemplateColumns = '1fr';
            editor.style.gridTemplateRows = `${state.splitRatio}fr 4px ${1 - state.splitRatio}fr`;
        }
        editor.classList.add('code-studio-split');
        editor.appendChild(pane1);
        editor.appendChild(divider);
        editor.appendChild(pane2);
        if (currentView) {
            pane1.appendChild(editor.appendChild(currentView.dom || currentView.textarea || document.createElement('div')));
            if (currentView.dom) pane1.appendChild(currentView.dom);
        }
        const emptyMsg = document.createElement('div');
        emptyMsg.className = 'cs-editor-empty';
        emptyMsg.innerHTML = `<div class="cs-empty-icon">{ }</div><div class="cs-empty-title">${esc(tr('codeStudio.splitRight', 'Split View'))}</div>`;
        pane2.appendChild(emptyMsg);
        wireSplitDivider(divider, editor, isHorizontal);
    }

    function wireSplitDivider(divider, container, isHorizontal) {
        let startPos = 0;
        let startRatio = state.splitRatio;
        const onPointerDown = bind(event => {
            event.preventDefault();
            startPos = isHorizontal ? event.clientX : event.clientY;
            startRatio = state.splitRatio;
            divider.classList.add('dragging');
            divider.setPointerCapture(event.pointerId);
            divider.addEventListener('pointermove', onPointerMove);
            divider.addEventListener('pointerup', onPointerUp);
            divider.addEventListener('pointercancel', onPointerUp);
        });
        const onPointerMove = bind(event => {
            const currentPos = isHorizontal ? event.clientX : event.clientY;
            const containerRect = container.getBoundingClientRect();
            const containerSize = isHorizontal ? containerRect.width : containerRect.height;
            const delta = currentPos - startPos;
            const newRatio = Math.max(0.2, Math.min(0.8, startRatio + delta / containerSize));
            state.splitRatio = newRatio;
            const template = isHorizontal
                ? `${newRatio}fr 4px ${1 - newRatio}fr`
                : `${newRatio}fr 4px ${1 - newRatio}fr`;
            if (isHorizontal) container.style.gridTemplateColumns = template;
            else container.style.gridTemplateRows = template;
        });
        const onPointerUp = bind(event => {
            divider.classList.remove('dragging');
            divider.releasePointerCapture(event.pointerId);
            divider.removeEventListener('pointermove', onPointerMove);
            divider.removeEventListener('pointerup', onPointerUp);
            divider.removeEventListener('pointercancel', onPointerUp);
        });
        divider.addEventListener('pointerdown', onPointerDown);
    }

    function togglePinPanel(panelType) {
        if (!state) return;
        const pinKey = panelType + 'Pinned';
        state[pinKey] = !state[pinKey];
        const root = studioRoot();
        if (root) {
            root.dataset[pinKey] = state[pinKey] ? 'true' : 'false';
        }
        renderWindowMenus();
    }
