    function showProjectKnowledge() {
        const instance = state;
        if (!instance || (!window.AuraGoLearnings || typeof window.AuraGoLearnings.open !== 'function')) return;
        if (instance.knowledgeClose) instance.knowledgeClose();
        const tab = activeTab();
        instance.knowledgeClose = window.AuraGoLearnings.open({
            project: 'code-studio', graph: true, file: tab && tab.path,
            parent: instance.root, title: tr('codeStudio.knowledge', 'Insights'),
            openSource: (file, line) => {
                if (instance.knowledgeClose) instance.knowledgeClose();
                return runAsyncStep(instance, () => openSearchResult(desktopToCodeStudioPath(file), line));
            }
        });
    }

    function splitEditor(direction) {
        if (!state || isVirtualTab(activeTab())) return;
        const next = direction === 'down' ? 'down' : 'right';
        // IDE package 1A: switching between right and down keeps both pane tabs; the same direction closes the split.
        if (state.splitMode === next) resetSplitState();
        else state.splitMode = next;
        renderEditor();
        renderWindowMenus();
        saveState();
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
        pane1.dataset.splitPane = 'primary';
        const divider = document.createElement('div');
        divider.className = 'code-studio-split-divider';
        divider.setAttribute('role', 'separator');
        divider.setAttribute('aria-orientation', isHorizontal ? 'vertical' : 'horizontal');
        const pane2 = document.createElement('div');
        pane2.className = 'code-studio-split-pane';
        pane2.dataset.splitPane = 'secondary';
        editor.append(pane1, divider, pane2);
        // IDE package 1A: every pane shows its own tab and the focused pane shows the active tab. The same tab in both
        // panes keeps the linked views (one history, the second view follows the first).
        const panes = resolveSplitPaneTabs(tab);
        if (panes.primary === panes.secondary) {
            const shared = panes.primary;
            const link = { views: [], syncing: false };
            shared.viewLink = link;
            shared.view = createEditorView(pane1, shared, link);
            shared.secondaryView = createEditorView(pane2, shared, link);
            shared.views = [shared.view, shared.secondaryView];
            link.views = shared.views;
        } else {
            panes.primary.view = createEditorView(pane1, panes.primary, null);
            panes.primary.views = [panes.primary.view];
            panes.secondary.view = createEditorView(pane2, panes.secondary, null);
            panes.secondary.views = [panes.secondary.view];
        }
        updateSplitPaneFocus(editor);
        pane1.addEventListener('focusin', bind(() => focusSplitPane('primary')));
        pane2.addEventListener('focusin', bind(() => focusSplitPane('secondary')));
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
            measureVisibleEditors();
            saveState();
        });
        divider.addEventListener('pointerdown', onPointerDown);
    }

    // IDE package 1A: split view with two different files (A4). The primary pane is left or top, the secondary pane
    // right or bottom. state.splitPrimaryTab and state.splitSecondaryTab hold the tab of each pane, state.splitFocus the
    // focused one. The focused pane's tab is always the active tab, so every activeTab() caller (save, run, find, agent)
    // works on the pane the user is in.

    function resetSplitState() {
        state.splitMode = null;
        state.splitRatio = 0.5;
        state.splitPrimaryTab = null;
        state.splitSecondaryTab = null;
        state.splitFocus = 'primary';
    }

    // The active tab goes into the focused pane; the other pane keeps its tab, or shows the same tab when it has none.
    function resolveSplitPaneTabs(activeSourceTab) {
        const usable = candidate => (candidate && isSourceTab(candidate) && state.openTabs.includes(candidate) ? candidate : null);
        let primary = usable(state.splitPrimaryTab);
        let secondary = usable(state.splitSecondaryTab);
        if (state.splitFocus === 'secondary') secondary = activeSourceTab;
        else primary = activeSourceTab;
        if (!primary) primary = secondary;
        if (!secondary) secondary = primary;
        state.splitPrimaryTab = primary;
        state.splitSecondaryTab = secondary;
        return { primary, secondary };
    }

    function updateSplitPaneFocus(editor) {
        if (!editor) return;
        const separate = !!state.splitPrimaryTab && state.splitPrimaryTab !== state.splitSecondaryTab;
        editor.querySelectorAll('[data-split-pane]').forEach(pane => {
            pane.classList.toggle('cs-split-pane-focused', separate && pane.dataset.splitPane === state.splitFocus);
        });
    }

    // Focusing a pane makes its tab the active tab without rebuilding the editors.
    function focusSplitPane(pane) {
        if (!state.splitMode) return;
        const next = pane === 'secondary' ? 'secondary' : 'primary';
        const changed = state.splitFocus !== next;
        state.splitFocus = next;
        updateSplitPaneFocus(shellPart('[data-editor]'));
        const tab = next === 'secondary' ? state.splitSecondaryTab : state.splitPrimaryTab;
        const index = tab ? state.openTabs.indexOf(tab) : -1;
        if (index < 0 || index === state.activeTabIndex) {
            if (changed) saveState();
            return;
        }
        state.activeTabIndex = index;
        renderTabs();
        renderBreadcrumbs();
        renderStatus();
        if (state.sidebarView === 'outline') renderSidebar();
        saveState();
    }

    // Closing the secondary pane's tab ends the split. Returns the primary pane's tab, which then becomes active.
    function closeSplitForTab(tab) {
        if (!state.splitMode || !tab || tab !== state.splitSecondaryTab) return null;
        const primary = state.splitPrimaryTab;
        const keep = primary && primary !== tab && state.openTabs.includes(primary) ? primary : null;
        resetSplitState();
        return keep;
    }

    // Close others, close all and delete remove tabs without closeTab; a missing secondary tab ends the split here.
    function pruneClosedSplitPane() {
        if (!state.splitMode || state.splitRestore || !state.splitSecondaryTab || state.openTabs.includes(state.splitSecondaryTab)) return;
        resetSplitState();
        renderWindowMenus();
        saveState();
    }

    function splitStatePayload() {
        const ratio = Math.max(0.2, Math.min(0.8, Number(state.splitRatio) || 0.5));
        const focus = state.splitFocus === 'secondary' ? 'secondary' : 'primary';
        if (state.splitMode && state.splitRestore) {
            return { mode: state.splitMode, ratio, primary: state.splitRestore.primary, secondary: state.splitRestore.secondary, focus };
        }
        const primary = state.splitPrimaryTab;
        const secondary = state.splitSecondaryTab;
        if (!state.splitMode || !isSourceTab(primary) || !isSourceTab(secondary) || !state.openTabs.includes(primary) || !state.openTabs.includes(secondary)) return null;
        return { mode: state.splitMode, ratio, primary: primary.path, secondary: secondary.path, focus };
    }

    // loadState: remember the saved split until restoreTabs has reopened the files.
    function restoreSplitPreference(saved) {
        const split = saved && typeof saved === 'object' ? saved : null;
        const mode = split && (split.mode === 'right' || split.mode === 'down') ? split.mode : null;
        state.splitPrimaryTab = null;
        state.splitSecondaryTab = null;
        if (!mode) {
            state.splitMode = null;
            state.splitRatio = 0.5;
            state.splitFocus = 'primary';
            state.splitRestore = null;
            return;
        }
        state.splitMode = mode;
        state.splitRatio = Math.max(0.2, Math.min(0.8, Number(split.ratio) || 0.5));
        state.splitFocus = split.focus === 'secondary' ? 'secondary' : 'primary';
        state.splitRestore = { primary: normalizeCodeStudioPath(split.primary), secondary: normalizeCodeStudioPath(split.secondary) };
    }

    // restoreTabs: map the saved paths onto the reopened tabs; without both files the split is dropped.
    function restoreSplitTabs() {
        const saved = state.splitRestore;
        state.splitRestore = null;
        if (!state.splitMode || !saved) return;
        const find = path => state.openTabs.find(tab => isSourceTab(tab) && tab.path === path) || null;
        const primary = find(saved.primary);
        const secondary = find(saved.secondary);
        if (!primary || !secondary) {
            resetSplitState();
            return;
        }
        state.splitPrimaryTab = primary;
        state.splitSecondaryTab = secondary;
    }

    // Ctrl+\ toggles the split in its current direction (right when it is off).
    function toggleSplitEditor() {
        splitEditor(state.splitMode === 'down' ? 'down' : 'right');
    }

    // Open to the Side: the file goes into the pane that is not focused; the split opens to the right when it is off.
    async function openToSide(path) {
        const target = state;
        if (!isLiveInstance(target)) return;
        const current = activeTab();
        const anchor = isSourceTab(current) ? current : null;
        const side = state.splitMode && state.splitFocus === 'secondary' ? 'primary' : 'secondary';
        if (!state.splitMode) {
            state.splitMode = 'right';
            state.splitRatio = 0.5;
        }
        if (anchor) {
            if (side === 'secondary') state.splitPrimaryTab = anchor;
            else state.splitSecondaryTab = anchor;
        }
        state.splitFocus = side;
        try {
            await openFile(path);
        } catch (err) {
            if (isLiveInstance(target)) {
                runWithInstance(target, () => {
                    showOperationError(err);
                    renderEditor();
                    renderWindowMenus();
                });
            }
        }
    }

    // Every mounted editor re-measures after a zoom or a divider drag; with two files both panes need it.
    function measureVisibleEditors() {
        state.openTabs.forEach(openTab => (openTab.views || []).forEach(view => {
            if (view && typeof view.requestMeasure === 'function') view.requestMeasure();
            if (view && view.textarea) view.textarea.getBoundingClientRect();
        }));
    }
