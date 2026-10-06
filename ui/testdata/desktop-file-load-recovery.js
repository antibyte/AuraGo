async docxBase64 => {
    const check = (condition, message) => { if (!condition) throw Error(message); };
    const wait = fn => csWait(fn, 30000);
    const docx = Uint8Array.from(atob(docxBase64), c => c.charCodeAt(0));
    const files = new Map([
        ['Documents/existing.txt', 'Existing text remains intact.'],
        ['Documents/existing.docx', docx],
        ['Documents/Notes/existing.md', '# Existing note'],
    ]);
    const failures = new Map();
    const writes = [];
    const reads = [];
    const fetchOriginal = window.fetch;
    const json = (data, status = 200) => new Response(JSON.stringify(data), {status, headers: {'Content-Type': 'application/json', ETag: '"1"'}});
    const note = path => ({path, content: files.get(path), version: '"1"', title: 'Existing note', modified: new Date().toISOString()});
    window.fetch = async (input, options = {}) => {
        const url = new URL(typeof input === 'string' ? input : input.url, location.href);
        const path = url.searchParams.get('path'), method = options.method || 'GET';
        if (url.pathname === '/api/desktop/bootstrap') return json(codeStudioTest.state.bootstrap);
        if (url.pathname === '/api/desktop/files') return path?.includes('missing') ? json({error: 'Folder not found'}, 404) : json({files: [{path: 'Documents/existing.txt', name: 'existing.txt', type: 'file', size: 29}]});
        if (['/api/desktop/file', '/api/desktop/office/document', '/api/desktop/notes'].includes(url.pathname)) {
            if (failures.has(path)) return json({error: 'Temporary read failure'}, failures.get(path));
            if (method === 'PUT' || method === 'POST') {
                const body = url.pathname.endsWith('/document') ? null : JSON.parse(options.body);
                const target = path || body?.path || 'Documents/Notes/new.md';
                const headers = new Headers(options.headers);
                if (method === 'PUT' && (files.has(target) ? headers.get('If-Match') !== '"1"' : headers.get('If-None-Match') !== '*')) return json({error: 'Write precondition missing'}, 412);
                writes.push(target);files.set(target, body?.content ?? new Uint8Array(await new Blob([options.body]).arrayBuffer()));
                return json(url.pathname.endsWith('/notes') ? note(target) : {path: target, version: '"1"'});
            }
            if (url.pathname.endsWith('/notes') && !path) return json({notes: [...files.keys()].filter(p => p.endsWith('.md')).map(note), folders: [], total: 1});
            if (!files.has(path)) return json({error: 'File not found'}, 404);
            if (url.pathname.endsWith('/document')) return new Response(files.get(path), {headers: {ETag: '"1"'}});
            return json(url.pathname.endsWith('/notes') ? note(path) : {path, content: files.get(path), version: '"1"'});
        }
        if (url.pathname === '/api/desktop/archive/list') return path.includes('missing') ? json({error: 'File not found'}, 404) : json({entries: []});
        if (url.pathname === '/api/desktop/archive') { writes.push(JSON.parse(options.body).dest); return json({path: 'Documents/new.zip'}); }
        if (url.pathname === '/api/desktop/viewer/content') return path.includes('missing') ? json({error: 'File not found'}, 404) : json({type: 'markdown', content: '# Existing preview'});
        if (url.pathname === '/api/desktop/preview') {
            if (path.includes('missing')) return json({error: 'File not found'}, 404);
            const canvas = document.createElement('canvas');canvas.width = 8;canvas.height = 8;
            return new Response(await new Promise(resolve => canvas.toBlob(resolve)), {headers: {ETag: '"1"'}});
        }
        if (url.pathname === '/api/desktop/download' && path?.endsWith('.stl')) {
            reads.push(path);
            return path.includes('missing') ? json({error: 'File not found'}, 404) : new Response('solid example\nfacet normal 0 0 1\nouter loop\nvertex 0 0 0\nvertex 1 0 0\nvertex 0 1 0\nendloop\nendfacet\nendsolid example');
        }
        return fetchOriginal(input, options);
    };
    const api = async (url, options) => { const response = await fetch(url, options);const body = await response.json();if (!response.ok) throw Object.assign(Error(body.error), {status: response.status});return body; };
    const host = id => { const el = document.createElement('div');el.id = id;el.style.cssText = 'position:fixed;inset:0;z-index:9999;background:#141922';document.body.append(el);return el; };
    let chosen = '', menus = [], contextPath = '', dialogs = 0, notices = [];
    const context = extra => ({t: window.t, api, path: '', notify: value => notices.push(value), confirmDialog: async () => true,
        setWindowMenus: (_, value) => { menus = value; }, clearWindowMenus() {}, setWindowBeforeClose() {},
        updateWindowContext: (_, patch) => { contextPath = patch.path; },
        promptDialog: async () => 'new.zip', saveFileDialog: async () => ({path: chosen}),
        openFileDialog: async () => { dialogs++;return {path: chosen, paths: [chosen]}; }, ...extra});
    const load = id => AuraDesktopModules.loadAppAssets(id);
    window.recoveryCases = {
        async writer() {
            await load('writer');const el = host('recovery-writer');
            const missing = 'Documents/missing.docx';chosen = 'Documents/existing.docx';
            const ctx = context({path: missing});WriterApp.render(el, 'recovery', ctx);
            const app = () => WriterApp.instances.get('recovery');
            await wait(() => el.querySelector('[data-load-actions]')?.hidden === false);
            check(contextPath === '' && !app().editor && !app().session, 'Writer did not detach the missing file');
            check(el.querySelector('[data-notice] [data-action="saveAs"]').hidden, 'Writer offered saving an unloaded document');
            el.querySelector('[data-load-actions] [data-action="new"]').click();
            await wait(() => app().editor && el.querySelector('[data-loading]').hidden);
            check(app().path !== missing && contextPath === app().path, 'Writer New retained the missing path');
            await app().act('save');check(!files.has(missing) && files.has(app().path), 'Writer recreated the missing document');
            WriterApp.render(el, 'recovery', ctx);await wait(() => !el.querySelector('[data-load-actions]').hidden);
            el.querySelector('[data-load-actions] [data-action="open"]').click();
            await wait(() => app().editor && el.querySelector('[data-loading]').hidden);
            check(app().editor.surface.session.bodyText().includes('Existing document'), 'Writer Open did not recover');
            failures.set(chosen, 503);WriterApp.render(el, 'recovery', context({path: chosen}));
            await wait(() => !el.querySelector('[data-load-actions]').hidden);
            check(contextPath === chosen, 'Writer forgot the path after a temporary error');
            failures.delete(chosen);await app().act('retry');
            check(app().editor && el.querySelector('[data-notice]').hidden && el.querySelector('[data-load-actions]').hidden, 'Writer retry retained the error state');
            WriterApp.render(el, 'recovery', context({path: missing, readonly: true}));
            await wait(() => !el.querySelector('[data-load-actions]').hidden);
            check(el.querySelector('[data-load-actions] [data-action="new"]').disabled, 'Writer recovery bypassed readonly');
            WriterApp.dispose('recovery');el.remove();
        },
        async editor() {
            const shell = codeStudioTest;
            shell.state.bootstrap.builtin_apps.push({id: 'editor', name: 'Text Editor', icon: 'text'});
            const missing = 'Documents/missing.txt';shell.openApp('editor', {path: missing});
            const win = [...shell.state.windows.values()].find(w => w.appId === 'editor');
            const el = win.element, save = () => shell.state.windowMenus.get(win.id).rawMenus.find(m => m.id === 'file').items.find(i => i.id === 'save');
            await wait(() => el.querySelector('[data-load-actions]')?.hidden === false);
            check(win.context.path === '' && el.querySelector('textarea').readOnly && save().disabled && el.querySelector('[data-new]').getBoundingClientRect().width > 0, 'Editor did not block saving an unloaded file or show recovery controls');
            await save().action();check(!files.has(missing), 'Editor silently recreated the missing file');
            el.querySelector('[data-new]').click();await wait(() => !el.querySelector('textarea').readOnly);
            check(win.context.path && win.context.path !== missing, 'Editor New retained the missing path');
            el.querySelector('textarea').value = 'New text';await save().action();
            check(files.get(win.context.path) === 'New text', 'Editor New could not save');
            // The existing-window route must retain load intent, including empty files.
            const path = win.context.path;failures.set(path, 503);shell.openApp('editor', {path});
            await wait(() => !el.querySelector('[data-load-actions]').hidden);
            check(win.context.path === path && save().disabled, 'Editor discarded a transient failure or enabled Save');
            failures.delete(path);el.querySelector('[data-retry]').click();await wait(() => !el.querySelector('textarea').readOnly);
            check(el.querySelector('textarea').value === 'New text' && !el.querySelector('[data-load-actions]').getClientRects().length, 'Editor retry did not recover or hide its recovery controls');
            failures.set(path, 503);shell.openApp('editor', {path});await wait(() => !el.querySelector('[data-load-actions]').hidden);
            el.querySelector('[data-open]').click();
            await wait(() => document.querySelector('[data-file-dialog-list] [data-path="Documents/existing.txt"]'));
            document.querySelector('[data-file-dialog-list] [data-path="Documents/existing.txt"]').click();document.querySelector('[data-file-dialog-confirm]').click();
            await wait(() => el.querySelector('textarea').value === files.get('Documents/existing.txt'));
            check(win.context.path === 'Documents/existing.txt' && !save().disabled, 'Editor Open failed');failures.delete(path);
            await shell.closeWindow(win.id);
            shell.openApp('editor', {path: 'Documents/created.txt', content: ''});
            const fresh = [...shell.state.windows.values()].find(w => w.appId === 'editor');
            await wait(() => fresh.element.querySelector('textarea')?.readOnly === false);
            check(fresh.element.querySelector('[data-load-actions]').hidden, 'Explicit empty new file failed');await shell.closeWindow(fresh.id);
            shell.state.bootstrap.readonly = true;shell.openApp('editor', {path: missing});
            const locked = [...shell.state.windows.values()].find(w => w.appId === 'editor');
            await wait(() => locked.element.querySelector('[data-load-actions]')?.hidden === false);
            check(locked.element.querySelector('[data-new]').disabled, 'Editor recovery bypassed readonly');
            await shell.closeWindow(locked.id);shell.state.bootstrap.readonly = false;
        },
        async notes() {
            await load('notes');const el = host('recovery-notes');chosen = 'Documents/Notes/existing.md';
            const ctx = context({path: 'Documents/Notes/missing.md'});
            await NotesApp.render(el, 'recovery', ctx).ready;
            check(!el.querySelector('[data-action="new"]').disabled && el.querySelector('[data-loading]').hidden, 'Notes remained busy');
            await NotesApp.instances.get('recovery').act('new');
            check(NotesApp.instances.get('recovery').editor && !files.has(ctx.path), 'Notes New failed or recreated the missing note');
            NotesApp.dispose('recovery');await NotesApp.render(el, 'recovery', ctx).ready;
            await NotesApp.instances.get('recovery').act('open');
            check(NotesApp.instances.get('recovery').path === chosen, 'Notes Open failed');NotesApp.dispose('recovery');el.remove();
        },
        async codeStudio() {
            await wait(() => csRoot()?.querySelector('.cs-tree-item'));
            const id = csState().windowId;
            localStorage.setItem('aurago.codeStudio.state.v1', JSON.stringify({openTabs: ['/workspace/missing.go', '/workspace/main.go'], activeTabIndex: 0}));
            await CodeStudioApp.render(csState().root, id, {...csState().context, path: '/workspace/missing.go'});
            await wait(() => csRoot()?.querySelector('.cs-tree-item'));
            check(!csRoot().querySelector('.code-studio-error') && csState().openTabs.some(tab => tab.path === '/workspace/main.go') && !csState().openTabs.some(tab => tab.path === '/workspace/missing.go'), 'Code Studio failed to skip a missing restored tab');
            const creating = CodeStudioApp.command('createNewFile', [], id);
            const form = document.querySelector('.cs-modal-backdrop form');form.elements.value.value = 'recovered.go';form.requestSubmit();await creating;
            check(csState().openTabs.some(tab => tab.path === '/workspace/recovered.go'), 'Code Studio New failed');
            await CodeStudioApp.openFile('/workspace/main.go', true, id);
            check(csState().openTabs[csState().activeTabIndex].path === '/workspace/main.go', 'Code Studio Open failed');
        },
        async pixel() {
            await load('pixel');const el = host('recovery-pixel');notices = [];chosen = 'Pictures/existing.png';
            PixelApp.render(el, 'recovery', context({path: 'Pictures/missing.png'}));
            await wait(() => notices.length);
            el.querySelector('[data-action="new-image"]').click();
            const form = document.querySelector('.vd-modal-backdrop form');check(form, 'Pixel New did not open');form.requestSubmit();
            await wait(() => el.querySelector('canvas')?.width === 1024);
            el.querySelector('[data-action="open"]').click();await wait(() => el.querySelector('canvas')?.width === 8);
            PixelApp.dispose('recovery');el.remove();
        },
        async zipper() {
            await load('zipper');const el = host('recovery-zipper');notices = [];chosen = 'Documents/existing.zip';
            ZipperApp.render(el, 'recovery', context({path: 'Documents/missing.zip'}));await wait(() => notices.length);
            el.querySelector('[data-action="open"]').click();await wait(() => el.querySelector('.zipper-path').textContent === chosen);
            el.querySelector('[data-action="new-archive"]').click();await wait(() => writes.includes('new.zip'));
            ZipperApp.dispose('recovery');el.remove();
        },
        async viewer() {
            await load('viewer');const el = host('recovery-viewer');
            ViewerApp.render(el, 'recovery', context({path: 'Documents/missing.md'}));await wait(() => el.querySelector('.vd-viewer-error'));
            ViewerApp.render(el, 'recovery', context({path: 'Documents/existing.md'}));await wait(() => el.querySelector('.vd-viewer-content')?.textContent.includes('Existing preview'));
            ViewerApp.dispose('recovery');el.remove();
        },
        async viewer3d() {
            await load('viewer-3d');const el = host('recovery-viewer3d');notices = [];
            Viewer3DApp.render(el, 'recovery', context({path: 'Documents/missing.stl'}));await wait(() => notices.length);
            check(reads.includes('Documents/missing.stl'), '3D fixture did not reach the file read');
            Viewer3DApp.render(el, 'recovery', context({path: 'Documents/existing.stl'}));await wait(() => !el.querySelector('[data-loading]'));
            Viewer3DApp.dispose('recovery');el.remove();
            check(!document.getElementById('recovery-viewer3d'), '3D viewer could not close after a missing file');
        },
    };
    window.showRecoveryScreenshot = async () => {
        const el = host('recovery-screenshot');
        WriterApp.render(el, 'recovery', context({path: 'Documents/missing.docx'}));
        await wait(() => !el.querySelector('[data-load-actions]').hidden);
    };
}
