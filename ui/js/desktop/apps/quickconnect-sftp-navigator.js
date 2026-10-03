    // Quick Connect SFTP navigation state, bundled inside the Desktop shell IIFE
    // (scripts/build-ui-bundles.js). One navigator per open SFTP panel: a newer
    // listing aborts the older request, and the committed path changes only after
    // a successful listing.
    function joinSFTPPath(dir, name) {
        const base = String(dir || '/');
        const leaf = String(name || '');
        return base === '/' ? '/' + leaf : base.replace(/\/+$/, '') + '/' + leaf;
    }

    function parentSFTPPath(dir) {
        const path = String(dir || '/');
        if (path === '/') return '/';
        return path.replace(/\/[^/]+\/?$/, '') || '/';
    }

    function createSFTPNavigator(listDirectory) {
        const nav = { path: '/', entries: [], seq: 0, controller: null, loading: false, disposed: false };
        nav.load = async function load(targetPath) {
            if (nav.disposed) return { status: 'disposed' };
            const seq = ++nav.seq;
            if (nav.controller) nav.controller.abort();
            const controller = new AbortController();
            nav.controller = controller;
            nav.loading = true;
            try {
                const resp = await listDirectory(targetPath, controller.signal);
                if (seq !== nav.seq || nav.disposed) return { status: 'stale' };
                const entries = Array.isArray(resp && resp.entries) ? resp.entries.slice() : [];
                entries.sort((a, b) => {
                    if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1;
                    return String(a.name).localeCompare(String(b.name));
                });
                nav.path = targetPath;
                nav.entries = entries;
                return { status: 'ok', path: targetPath, entries };
            } catch (error) {
                if (seq !== nav.seq || nav.disposed || (error && error.name === 'AbortError')) return { status: 'stale' };
                return { status: 'error', error };
            } finally {
                if (nav.controller === controller) {
                    nav.controller = null;
                    nav.loading = false;
                }
            }
        };
        nav.dispose = function dispose() {
            nav.disposed = true;
            nav.seq += 1;
            if (nav.controller) nav.controller.abort();
            nav.controller = null;
            nav.loading = false;
        };
        return nav;
    }
