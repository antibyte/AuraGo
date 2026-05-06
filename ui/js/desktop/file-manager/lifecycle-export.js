            toggleSearch();
            return;
        }
        if (e.key === '/' && !isInput) {
            e.preventDefault();
            toggleSearch();
            return;
        }
    }

    /**
     * Navigate an existing file manager instance to a new path.
     * Called by main.js when the user double-clicks a folder shortcut on the desktop.
     */
    function navigateTo(windowId, path) {
        if (fm.windowId === windowId && fm.host) {
            navigate(path);
        }
    }

    // Expose the module
    window.FileManager = { render, navigateTo };
})();
