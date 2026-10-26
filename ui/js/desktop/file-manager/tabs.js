    // Tab Management for File Manager

    function initTabs(fmInstance) {
        if (!fmInstance.tabs) {
            const tabId = 'tab-' + Date.now() + '-' + Math.random().toString(36).slice(2, 9);
            fmInstance.tabs = [{
                id: tabId,
                path: fmInstance.currentPath || '',
                storage: { ...fmInstance.storage },
                history: [...fmInstance.history],
                historyIndex: fmInstance.historyIndex,
                selectedPaths: new Set(fmInstance.selectedPaths),
                scrollPosition: 0
            }];
            fmInstance.activeTabIndex = 0;
        }
    }

    function renderTabBarHtml() {
        initTabs(fm);
        
        const tabItems = fm.tabs.map((tab, idx) => {
            const active = idx === fm.activeTabIndex ? ' active' : '';
            const sourceName = storageName({ ...fm, storage: tab.storage });
            const title = tab.path ? (tab.storage?.provider !== 'local' ? sourceName + ' · ' : '') + (tab.storage?.name || tab.path.split('/').pop()) : sourceName;
            return `<div class="fm-tab${active}" data-tab-index="${idx}" draggable="true" role="tab" aria-selected="${idx === fm.activeTabIndex ? 'true' : 'false'}">
                <span class="fm-tab-icon">${iconMarkup('folder', 'F', 'fm-tab-folder-icon', 12, 'folder')}</span>
                <span class="fm-tab-title" title="${esc(tab.path || '/')}">${esc(title)}</span>
                ${fm.tabs.length > 1 ? `<button type="button" class="fm-tab-close" data-action="close-tab" data-tab-index="${idx}" title="${esc(t('desktop.close'))}">&times;</button>` : ''}
            </div>`;
        }).join('');
        
        return `<div class="fm-tab-bar" role="tablist">
            <div class="fm-tabs-container">${tabItems}</div>
            <button type="button" class="fm-tab-new" data-action="new-tab" title="${esc(t('desktop.fm.new_tab'))}">+</button>
        </div>`;
    }

    function createNewTab(path) {
        initTabs(fm);
        const location = storageLocation(path == null ? fm.currentPath : path);
        const newTab = {
            id: 'tab-' + Date.now() + '-' + Math.random().toString(36).slice(2, 9),
            path: location.path,
            storage: { ...location },
            history: [location],
            historyIndex: 0,
            selectedPaths: new Set(),
            scrollPosition: 0
        };
        fm.tabs.push(newTab);
        switchTab(fm.tabs.length - 1);
    }

    function closeTab(index) {
        initTabs(fm);
        if (fm.tabs.length <= 1) return;
        syncActiveTab();
        const activeID = fm.tabs[fm.activeTabIndex].id;
        fm.tabs.splice(index, 1);
        const next = fm.tabs.findIndex(tab => tab.id === activeID);
        fm.activeTabIndex = -1;
        switchTab(next >= 0 ? next : Math.min(index, fm.tabs.length - 1));
    }

    function switchTab(index) {
        initTabs(fm);
        if (index < 0 || index >= fm.tabs.length) return;
        
        // Save current state to the active tab before switching
        const currentTab = fm.tabs[fm.activeTabIndex];
        if (currentTab) {
            currentTab.path = fm.currentPath;
            currentTab.storage = { ...fm.storage };
            currentTab.history = [...fm.history];
            currentTab.historyIndex = fm.historyIndex;
            currentTab.selectedPaths = new Set(fm.selectedPaths);
            const main = fm.host ? fm.host.querySelector('[data-fm-main]') : null;
            currentTab.scrollPosition = main ? main.scrollTop : 0;
        }
        
        fm.activeTabIndex = index;
        const targetTab = fm.tabs[index];
        
        // Load state from the target tab
        fm.currentPath = targetTab.path;
        fm.storage = { ...targetTab.storage };
        fm.history = [...targetTab.history];
        fm.historyIndex = targetTab.historyIndex;
        fm.selectedPaths = new Set(targetTab.selectedPaths);
        fm.searchQuery = ''; // Reset search on tab switch
        
        const searchInput = fm.host ? fm.host.querySelector('.fm-search-input') : null;
        if (searchInput) searchInput.value = '';
        
        const instance = fm;
        const selection = new Set(targetTab.selectedPaths);
        const location = storageLocation();
        navigate(location, false).then(() => {
            if (!isLiveInstance(instance) || instance.tabs[instance.activeTabIndex]?.id !== targetTab.id || locationKey(storageLocation(instance.currentPath, instance)) !== locationKey(location)) return;
            instance.selectedPaths = selection;
            withInstance(instance, renderFileContent);
            const main = instance.host.querySelector('[data-fm-main]');
            if (main) main.scrollTop = targetTab.scrollPosition || 0;
        });
    }

    let dragTabSourceIndex = null;
    
    function handleTabDragStart(e) {
        const index = parseInt(e.currentTarget.dataset.tabIndex);
        dragTabSourceIndex = index;
        e.dataTransfer.effectAllowed = 'move';
        e.stopPropagation();
    }
    
    function handleTabDragOver(e) {
        e.preventDefault();
        e.dataTransfer.dropEffect = 'move';
        e.stopPropagation();
    }
    
    function handleTabDrop(e) {
        e.preventDefault();
        e.stopPropagation();
        const targetIndex = parseInt(e.currentTarget.dataset.tabIndex);
        if (dragTabSourceIndex !== null && dragTabSourceIndex !== targetIndex) {
            const moved = fm.tabs.splice(dragTabSourceIndex, 1)[0];
            fm.tabs.splice(targetIndex, 0, moved);
            
            // Adjust activeTabIndex if needed
            if (fm.activeTabIndex === dragTabSourceIndex) {
                fm.activeTabIndex = targetIndex;
            } else if (fm.activeTabIndex > dragTabSourceIndex && fm.activeTabIndex <= targetIndex) {
                fm.activeTabIndex--;
            } else if (fm.activeTabIndex < dragTabSourceIndex && fm.activeTabIndex >= targetIndex) {
                fm.activeTabIndex++;
            }
            
            renderAll();
        }
        dragTabSourceIndex = null;
    }

    function syncActiveTab() {
        initTabs(fm);
        if (fm.tabs && fm.tabs[fm.activeTabIndex]) {
            const currentTab = fm.tabs[fm.activeTabIndex];
            currentTab.path = fm.currentPath;
            currentTab.storage = { ...fm.storage };
            currentTab.history = [...fm.history];
            currentTab.historyIndex = fm.historyIndex;
            currentTab.selectedPaths = new Set(fm.selectedPaths);
        }
    }
