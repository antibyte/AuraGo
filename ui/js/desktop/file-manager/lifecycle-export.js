    /**
     * Navigate an existing file manager instance to a new path.
     * Called by main.js when the user double-clicks a folder shortcut on the desktop.
     */
    function navigateTo(windowId, path) {
        const instance = instanceForWindow(windowId);
        if (instance && instance.host) {
            setActiveInstance(instance);
            navigate(path);
        }
    }

    async function dropDesktopFiles(windowId, paths, destPath) {
        const instance = instanceForWindow(windowId);
        if (!instance || !instance.host) return false;
        setActiveInstance(instance);
        await moveDroppedDesktopFilesToFolder(paths, destPath == null ? fm.currentPath : destPath);
        return true;
    }

    function dispose(windowId) {
        const instance = instanceForWindow(windowId);
        if (!instance) return;
        if (instance.callbacks && typeof instance.callbacks.clearWindowMenus === 'function') {
            instance.callbacks.clearWindowMenus(windowId);
        }
        if (instance.callbacks && typeof instance.callbacks.closeContextMenu === 'function') {
            instance.callbacks.closeContextMenu();
        }
        if (instance.host) {
            instance.host.innerHTML = '';
            instance.host = null;
        }
        instances.delete(windowId);
        if (fm === instance) {
            const next = instances.values().next();
            fm = next.done ? createInstance() : next.value;
        }
    }

    // Expose the module
    window.FileManager = { render, navigateTo, dropDesktopFiles, dispose };
})();
