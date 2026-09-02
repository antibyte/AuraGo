    let spacesOverviewOpen = false;
    let pagerHoldTimer = 0;
    let pagerHoldTriggered = false;

    function spacesOverviewEnabled() {
        return spacesEnabled();
    }

    function isSpacesOverviewOpen() {
        return spacesOverviewOpen;
    }

    function windowsForSpace(spaceId) {
        return [...state.windows.values()].filter(win => {
            if (!win || win.isGadget || !win.element) return false;
            return windowSpaceId(win) === normalizeSpaceId(spaceId);
        });
    }

    function closeSpacesOverview() {
        spacesOverviewOpen = false;
        const overlay = document.getElementById('vd-spaces-overview');
        if (overlay) {
            overlay.hidden = true;
            overlay.replaceChildren();
        }
        document.body.classList.remove('vd-spaces-overview-open');
    }

    function renderSpacesOverview() {
        let overlay = document.getElementById('vd-spaces-overview');
        if (!overlay) {
            overlay = document.createElement('div');
            overlay.id = 'vd-spaces-overview';
            overlay.className = 'vd-spaces-overview';
            overlay.hidden = true;
            document.body.appendChild(overlay);
        }
        const activeId = normalizeSpaceId(state.activeSpaceId);
        overlay.innerHTML = `<div class="vd-spaces-overview-backdrop" data-action="close-overview"></div>
            <div class="vd-spaces-overview-panel" role="dialog" aria-modal="true" aria-label="${esc(t('desktop.spaces_overview'))}">
                <div class="vd-spaces-overview-header">
                    <div class="vd-spaces-overview-title">${esc(t('desktop.spaces_overview'))}</div>
                    <button type="button" class="vd-spaces-overview-close" data-action="close-overview">${esc(t('desktop.close'))}</button>
                </div>
                <div class="vd-spaces-overview-columns">${SPACE_IDS.map(id => renderSpaceOverviewColumn(id, activeId)).join('')}</div>
            </div>`;
        wireSpacesOverviewEvents(overlay);
    }

    function renderSpaceOverviewColumn(spaceId, activeId) {
        const wins = windowsForSpace(spaceId);
        const isActive = normalizeSpaceId(spaceId) === activeId;
        const cards = wins.length
            ? wins.map(win => renderSpaceOverviewCard(win)).join('')
            : `<div class="vd-space-column-empty">${esc(t('desktop.spaces_overview_empty'))}</div>`;
        return `<section class="vd-space-column${isActive ? ' active' : ''}" data-space-id="${esc(spaceId)}" aria-current="${isActive ? 'true' : 'false'}">
            <button type="button" class="vd-space-column-head" data-space-target="${esc(spaceId)}">${esc(t('desktop.space_n').replace('{{n}}', spaceId))}</button>
            <div class="vd-space-column-body">${cards}</div>
        </section>`;
    }

    function renderSpaceOverviewCard(win) {
        const minimized = win.element.style.display === 'none';
        const preview = windowPreviewMarkup(win, {
            maxW: 180,
            maxH: 110,
            titleClass: 'vd-overview-card-title',
            labelClass: 'vd-overview-card-label',
            iconClass: 'vd-overview-card-icon',
            iconSize: 16,
            fallbackIconSize: 36,
            viewportClass: 'vd-overview-card-viewport',
            scaleClass: 'vd-overview-card-scale',
            fallbackClass: 'vd-overview-card-fallback',
            fallbackIconClass: 'vd-overview-card-fallback-icon'
        });
        return `<button type="button" class="vd-space-window-card${minimized ? ' minimized' : ''}" data-window-id="${esc(win.id)}" draggable="true">${preview}</button>`;
    }

    function wireSpacesOverviewEvents(overlay) {
        overlay.querySelectorAll('[data-action="close-overview"]').forEach(node => {
            node.addEventListener('click', closeSpacesOverview);
        });
        overlay.querySelectorAll('[data-space-target]').forEach(btn => {
            btn.addEventListener('click', () => {
                const spaceId = btn.dataset.spaceTarget;
                if (normalizeSpaceId(spaceId) !== normalizeSpaceId(state.activeSpaceId)) switchSpace(spaceId);
                closeSpacesOverview();
            });
        });
        overlay.querySelectorAll('[data-window-id]').forEach(btn => {
            btn.addEventListener('click', () => {
                const win = state.windows.get(btn.dataset.windowId);
                if (!win) return;
                const targetSpace = windowSpaceId(win);
                if (targetSpace !== normalizeSpaceId(state.activeSpaceId)) switchSpace(targetSpace);
                focusWindow(win.id);
                closeSpacesOverview();
            });
            btn.addEventListener('dragstart', event => {
                event.dataTransfer.setData('text/plain', btn.dataset.windowId);
                event.dataTransfer.effectAllowed = 'move';
                btn.classList.add('dragging');
            });
            btn.addEventListener('dragend', () => btn.classList.remove('dragging'));
        });
        overlay.querySelectorAll('.vd-space-column-body').forEach(body => {
            body.addEventListener('dragover', event => {
                event.preventDefault();
                event.dataTransfer.dropEffect = 'move';
                body.closest('.vd-space-column')?.classList.add('drop-target');
            });
            body.addEventListener('dragleave', () => {
                body.closest('.vd-space-column')?.classList.remove('drop-target');
            });
            body.addEventListener('drop', event => {
                event.preventDefault();
                const column = body.closest('.vd-space-column');
                column?.classList.remove('drop-target');
                const windowId = event.dataTransfer.getData('text/plain');
                const spaceId = column && column.dataset.spaceId;
                if (!windowId || !spaceId) return;
                moveWindowToSpace(windowId, spaceId);
                renderSpacesOverview();
            });
        });
    }

    function openSpacesOverview() {
        if (!spacesOverviewEnabled()) return;
        hideTaskbarThumbnail();
        spacesOverviewOpen = true;
        document.body.classList.add('vd-spaces-overview-open');
        renderSpacesOverview();
        const overlay = document.getElementById('vd-spaces-overview');
        if (overlay) overlay.hidden = false;
    }

    function toggleSpacesOverview() {
        if (spacesOverviewOpen) closeSpacesOverview();
        else openSpacesOverview();
    }

    function openSpacesOverviewForWindow(windowId) {
        const win = state.windows.get(windowId);
        if (!win || !spacesOverviewEnabled()) return;
        const spaceId = windowSpaceId(win);
        if (spaceId !== normalizeSpaceId(state.activeSpaceId)) switchSpace(spaceId);
        openSpacesOverview();
    }

    function handleSpacesOverviewShortcut(event) {
        if (!spacesOverviewEnabled() || isEditableTarget(event.target)) return false;
        if (event.key === 'F3') {
            event.preventDefault();
            toggleSpacesOverview();
            return true;
        }
        if (!event.ctrlKey || !event.altKey || event.metaKey) return false;
        if (event.key === 'ArrowUp') {
            event.preventDefault();
            toggleSpacesOverview();
            return true;
        }
        if (event.key === 'ArrowDown' && spacesOverviewOpen) {
            event.preventDefault();
            closeSpacesOverview();
            return true;
        }
        return false;
    }

    function cancelPagerHoldTimer() {
        if (pagerHoldTimer) {
            window.clearTimeout(pagerHoldTimer);
            pagerHoldTimer = 0;
        }
    }

    function wireSpacePagerOverview(pager) {
        if (!pager || pager.dataset.overviewWired === 'true') return;
        pager.dataset.overviewWired = 'true';
        pager.addEventListener('pointerdown', event => {
            const btn = event.target.closest('[data-space-id]');
            if (!btn || !spacesOverviewEnabled()) return;
            cancelPagerHoldTimer();
            pagerHoldTriggered = false;
            pagerHoldTimer = window.setTimeout(() => {
                pagerHoldTimer = 0;
                pagerHoldTriggered = true;
                openSpacesOverview();
            }, 400);
        });
        pager.addEventListener('pointerup', cancelPagerHoldTimer);
        pager.addEventListener('pointercancel', cancelPagerHoldTimer);
        pager.addEventListener('pointerleave', cancelPagerHoldTimer);
        pager.addEventListener('click', event => {
            if (pagerHoldTriggered) {
                event.preventDefault();
                event.stopPropagation();
                pagerHoldTriggered = false;
                return;
            }
            const btn = event.target.closest('[data-space-id]');
            if (!btn || !spacesOverviewEnabled()) return;
            if (normalizeSpaceId(btn.dataset.spaceId) === normalizeSpaceId(state.activeSpaceId)) {
                event.preventDefault();
                event.stopPropagation();
                openSpacesOverview();
                return;
            }
            switchSpace(btn.dataset.spaceId);
        }, true);
    }

    function initSpacesOverviewRuntime() {
        closeSpacesOverview();
        const pager = document.getElementById('vd-space-pager');
        if (pager) wireSpacePagerOverview(pager);
        window.addEventListener('resize', () => {
            if (!spacesOverviewEnabled()) closeSpacesOverview();
            else if (spacesOverviewOpen) renderSpacesOverview();
        });
    }
