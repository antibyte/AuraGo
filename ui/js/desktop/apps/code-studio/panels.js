    function splitEditor(direction) {
        if (!state) return;
        const next = direction === 'down' ? 'down' : 'right';
        state.splitMode = state.splitMode === next ? null : next;
        if (!state.splitMode) state.splitRatio = 0.5;
        renderEditor();
        renderWindowMenus();
    }

    function splitGridTemplate(ratio) {
        const clamped = Math.max(0.2, Math.min(0.8, Number(ratio) || 0.5));
        return `minmax(0, ${clamped}fr) 4px minmax(0, ${1 - clamped}fr)`;
    }

    function renderSplitPanes(editor, tab) {
        const isHorizontal = state.splitMode === 'right';
        editor.classList.add('code-studio-split', isHorizontal ? 'split-right' : 'split-down');
        if (isHorizontal) {
            editor.style.gridTemplateColumns = splitGridTemplate(state.splitRatio);
            editor.style.gridTemplateRows = 'minmax(0, 1fr)';
        } else {
            editor.style.gridTemplateColumns = 'minmax(0, 1fr)';
            editor.style.gridTemplateRows = splitGridTemplate(state.splitRatio);
        }
        const pane1 = document.createElement('div');
        pane1.className = 'code-studio-split-pane';
        const divider = document.createElement('div');
        divider.className = 'code-studio-split-divider';
        divider.setAttribute('role', 'separator');
        divider.setAttribute('aria-orientation', isHorizontal ? 'vertical' : 'horizontal');
        const pane2 = document.createElement('div');
        pane2.className = 'code-studio-split-pane';
        editor.append(pane1, divider, pane2);
        const link = { views: [], syncing: false };
        tab.view = createEditorView(pane1, tab, link);
        tab.secondaryView = createEditorView(pane2, tab, link);
        tab.views = [tab.view, tab.secondaryView];
        link.views = tab.views;
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
            if (!containerSize) return;
            const delta = currentPos - startPos;
            state.splitRatio = Math.max(0.2, Math.min(0.8, startRatio + delta / containerSize));
            if (isHorizontal) container.style.gridTemplateColumns = splitGridTemplate(state.splitRatio);
            else container.style.gridTemplateRows = splitGridTemplate(state.splitRatio);
        });
        const onPointerUp = bind(event => {
            divider.classList.remove('dragging');
            divider.releasePointerCapture(event.pointerId);
            divider.removeEventListener('pointermove', onPointerMove);
            divider.removeEventListener('pointerup', onPointerUp);
            divider.removeEventListener('pointercancel', onPointerUp);
            const tab = activeTab();
            (tab && tab.views || []).forEach(view => {
                if (view && typeof view.requestMeasure === 'function') view.requestMeasure();
            });
        });
        divider.addEventListener('pointerdown', onPointerDown);
    }
