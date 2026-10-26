(function () {
    'use strict';
    async function render(host, windowId, app, context, deps) {
        const { t, esc, api, makeSandboxedFrame, registerWindowCleanup, modalDialog, openFileDialog, saveFileDialog } = deps;
        const abort = new AbortController();
        let disposed = false, port = null, dirty = false, busy = false, connected = false, generation = 0, serial = 0;
        let frame = null, frameOrigin = '', handshake = '', editing = 0, openedOnce = false;
        const pending = new Map();
        const project = new AuraZiroFiles.Project(window.fetch.bind(window), abort.signal);
        const text = key => t('desktop.ziro.' + key);
        const errorText = error => /^ziro\./.test(error.message) ? text(error.message.slice(5)) : error.message;
        host.innerHTML = `<div class="vd-ziro" style="height:100%;display:flex;flex-direction:column;background:var(--vd-theme-app-bg,#18181b);color:var(--vd-theme-text,#e8e8e8);font-family:system-ui,sans-serif">
          <div class="vd-store-toolbar" style="flex-wrap:wrap;gap:8px">
            ${['new', 'open', 'save', 'save_as'].map(action => `<button type="button" class="vd-store-btn" style="width:auto;flex:0 0 auto" data-ziro="${action}">${esc(text(action))}</button>`).join('')}
            <span data-ziro-path style="flex:1;overflow-wrap:anywhere"></span>
            <span role="status" aria-live="polite" data-ziro-status>${esc(t('desktop.loading'))}</span>
          </div><div data-ziro-frame style="flex:1;min-height:0;display:flex;overflow:auto"></div></div>`;
        const status = host.querySelector('[data-ziro-status]');
        const pathLabel = host.querySelector('[data-ziro-path]');
        const surface = host.querySelector('[data-ziro-frame]');
        const alive = () => !disposed && host.isConnected;
        function display(key) { if (alive()) status.textContent = text(key); }
        function controls() {
            host.querySelectorAll('[data-ziro]').forEach(button => {
                button.disabled = busy || !connected || (deps.isReadonly() && button.dataset.ziro !== 'open');
            });
        }
        function revoke() {
            connected = false;
            generation++;
            if (port) { port.close(); port = null; }
            for (const entry of pending.values()) { clearTimeout(entry.timer); entry.reject(new Error('ziro.disconnected')); }
            pending.clear();
            controls();
        }
        function request(type, payload) {
            if (!port || !connected || !alive()) return Promise.reject(new Error('ziro.disconnected'));
            const id = ++serial;
            return new Promise((resolve, reject) => {
                const timer = setTimeout(() => { pending.delete(id); reject(new Error('ziro.disconnected')); }, 30000);
                pending.set(id, { resolve, reject, timer });
                port.postMessage({ ziro: 1, id, type, payload });
            });
        }
        async function choose(message, choices, input) {
            return modalDialog({ title: 'Ziro Designer', message, choices: input ? undefined : choices, input: !!input, value: input?.value || '', signal: abort.signal });
        }
        async function cleanBeforeLeave() {
            if (busy) return false;
            if (connected && openedOnce) {
                try { await request('snapshot'); }
                catch (error) { if (alive()) status.textContent = errorText(error); dirty = true; }
            }
            if (!dirty) return true;
            const choice = await choose(text('unsaved'), [
                { value: 'save', label: text('save') }, { value: 'discard', label: text('discard') }
            ]);
            if (choice === 'save') return run(() => save(false));
            if (choice === 'discard') { if (connected) await request('discard'); dirty = false; return true; }
            return false;
        }
        async function run(fn) {
            if (!alive() || busy) return false;
            busy = true; controls();
            try { await fn(); return alive(); }
            catch (error) { if (alive()) status.textContent = error.name === 'AbortError' ? text(dirty ? 'unsaved_status' : 'ready') : errorText(error); return false; }
            finally { busy = false; controls(); }
        }
        async function folderForSave(files) {
            if (!project.folder) await project.mkdir('Shared/Ziro Designer');
            const result = await saveFileDialog({ initialPath: project.folder ? project.folder.slice(0, project.folder.lastIndexOf('/')) : 'Shared/Ziro Designer',
                filename: files.find(file => /\.kicad_pro$/i.test(file.name)).name.split('/').pop(), defaultExtension: 'kicad_pro',
                filters: [{ label: 'KiCad', extensions: ['kicad_pro'] }], signal: abort.signal });
            if (!alive() || result.canceled) throw new DOMException('Cancelled', 'AbortError');
            AuraZiroFiles.shared(result.path);
            // A separate directory keeps a project copy's internal references intact.
            const folder = result.path.replace(/\.kicad_pro$/i, '');
            if (project.folder && (folder === project.folder || folder.startsWith(project.folder + '/'))) throw new Error('ziro.invalid_path');
            return folder;
        }
        async function save(asCopy) {
            if (deps.isReadonly()) throw new Error('ziro.readonly');
            const snapshot = await request('snapshot');
            const captured = editing;
            const files = AuraZiroFiles.filesChecked(snapshot.files);
            if (!files.some(file => /\.kicad_pro$/i.test(file.name))) throw new Error('ziro.invalid_project');
            if (asCopy || !project.folder) {
                const folder = await folderForSave(files);
                // Refuse merging a Save As into an occupied directory.
                const listing = await api('/api/desktop/files?path=' + encodeURIComponent(folder), { signal: abort.signal }).catch(error => {
                    if (error.status === 400 || error.status === 404) return null;
                    throw error;
                });
                if (listing) throw new Error('ziro.folder_exists');
                project.folder = folder;
                project.baseline = new Map();
            }
            display('saving');
            try {
                await project.save(files, (path, conflict) => choose(text('conflict').replace('{{path}}', path), [
                    { value: 'replace', label: text('replace'), disabled: !conflict?.version },
                    { value: 'copy', label: text('copy') }
                ]));
            } catch (error) {
                if (error.copyProject) return save(true);
                throw error;
            }
            if (!alive()) return;
            pathLabel.textContent = project.folder;
            deps.updateContext?.({ path: project.folder + '/' + files.find(file => /\.kicad_pro$/i.test(file.name)).name });
            await request('saved', { generation: snapshot.generation, projectKey: project.folder });
            if (captured === editing) dirty = false;
            display(dirty ? 'unsaved_status' : 'saved');
        }
        async function open(path) {
            const incoming = new AuraZiroFiles.Project(window.fetch.bind(window), abort.signal);
            const files = await incoming.open(path);
            if (!alive()) return;
            const result = await request('open', { projectKey: incoming.folder, files, draftKey: deps.draftKey });
            openedOnce = true;
            dirty = false;
            if (result.draft) {
                const choice = await choose(text('recover'), [{ value: 'recover', label: text('recover_action') }, { value: 'discard', label: text('discard') }]);
                if (!choice) {
                    await request('cancel-open');
                    throw new DOMException('Cancelled', 'AbortError');
                }
                await request(choice === 'recover' ? 'recover' : 'discard');
                dirty = choice === 'recover';
            }
            project.folder = incoming.folder; project.baseline = incoming.baseline;
            pathLabel.textContent = project.folder;
            deps.updateContext?.({ path });
            display(dirty ? 'unsaved_status' : 'ready');
        }
        async function onAction(action) {
            if (action === 'save' || action === 'save_as') return run(() => save(action === 'save_as'));
            return run(async () => {
                if (action === 'new') {
                    if (deps.isReadonly()) throw new Error('ziro.readonly');
                    const name = await choose(text('project_name'), [{ value: 'create', label: text('new') }], { value: 'Project', placeholder: text('project_name') });
                    if (!name) throw new DOMException('Cancelled', 'AbortError');
                    const projectName = typeof name === 'object' ? name.input : name;
                    AuraZiroFiles.relative(projectName);
                    if (projectName.includes('/')) throw new Error('ziro.invalid_path');
                    if (openedOnce) return deps.openProjectWindow({ ziro_new: projectName });
                    await newProject(projectName);
                } else {
                    const picked = await openFileDialog({ initialPath: 'Shared/Ziro Designer', filters: [{ label: 'KiCad', extensions: ['kicad_pro'] }], signal: abort.signal });
                    if (!alive() || picked.canceled) return;
                    if (openedOnce) return deps.openProjectWindow({ path: picked.path });
                    await open(picked.path);
                }
            });
        }
        async function newProject(name) {
            if (deps.isReadonly()) throw new Error('ziro.readonly');
            AuraZiroFiles.relative(name);
            if (name.includes('/')) throw new Error('ziro.invalid_path');
            await request('new', { name, draftKey: deps.draftKey });
            openedOnce = true; dirty = true;
            project.folder = ''; project.baseline = new Map();
            pathLabel.textContent = ''; display('unsaved_status');
        }
        const onKey = event => {
            if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's') { event.preventDefault(); void onAction(event.shiftKey ? 'save_as' : 'save'); }
        };
        host.addEventListener('keydown', onKey);
        host.querySelectorAll('[data-ziro]').forEach(button => button.addEventListener('click', () => void onAction(button.dataset.ziro)));
        function ready(event) {
            if (!alive() || !frame || event.source !== frame.contentWindow || event.origin !== frameOrigin
                || event.data?.ziro !== 1 || event.data.type !== 'ready' || event.data.challenge !== handshake || port) return;
            const channel = new MessageChannel();
            port = channel.port1;
            const current = generation;
            port.onmessage = async message => {
                if (!alive() || current !== generation) return;
                const data = message.data;
                if (data?.ziro !== 1) return;
                if (data.type === 'dirty') { editing++; dirty = true; display('unsaved_status'); return; }
                if (data.type === 'save-request') {
                    const ok = await onAction('save');
                    if (port && current === generation) port.postMessage({ ziro: 1, reply: data.id, ok });
                    return;
                }
                const item = pending.get(data.reply);
                if (!item) return;
                pending.delete(data.reply); clearTimeout(item.timer);
                data.error ? item.reject(new Error(data.error)) : item.resolve(data.payload || {});
            };
            frame.contentWindow.postMessage({ ziro: 1, type: 'connect', challenge: handshake }, frameOrigin, [channel.port2]);
            connected = true; display('ready'); controls();
            if (context?.path) void run(() => open(context.path));
            else void run(async () => {
                const result = await request('draft', { draftKey: deps.draftKey });
                if (!result.draft) { if (context?.ziro_new) await newProject(context.ziro_new); return; }
                openedOnce = true;
                const choice = await choose(text('recover'), [{ value: 'recover', label: text('recover_action') }, { value: 'discard', label: text('discard') }]);
                await request(choice === 'recover' ? 'recover' : choice === 'discard' ? 'discard' : 'cancel-open');
                dirty = choice === 'recover'; display(dirty ? 'unsaved_status' : 'ready');
            });
        }
        const policy = () => controls();
        document.addEventListener('aurago:desktop-policy', policy);
        window.addEventListener('message', ready);
        deps.setBeforeClose(cleanBeforeLeave);
        const beforeUnload = event => { if (dirty) { event.preventDefault(); event.returnValue = ''; } };
        window.addEventListener('beforeunload', beforeUnload);
        registerWindowCleanup(windowId, () => {
            disposed = true; abort.abort(); revoke();
            window.removeEventListener('message', ready);
            window.removeEventListener('beforeunload', beforeUnload);
            document.removeEventListener('aurago:desktop-policy', policy);
            host.removeEventListener('keydown', onKey);
            frame?.remove();
        });
        controls();
        try {
            const body = await api(deps.openURL, { signal: abort.signal });
            if (!alive()) return;
            const url = new URL(body.url, location.href);
            if (!['http:', 'https:'].includes(url.protocol) || url.origin === location.origin) throw new Error('ziro.origin_required');
            frameOrigin = url.origin;
            handshake = Array.from(crypto.getRandomValues(new Uint8Array(24)), b => b.toString(16).padStart(2, '0')).join('');
            url.hash = new URLSearchParams({ aurago_ziro: handshake, parent_origin: location.origin }).toString();
            frame = makeSandboxedFrame(url.href, app.id, '', windowId, 'vd-generated-frame vd-store-app-frame', 'Ziro Designer', { allowSameOrigin: true, allowDownloads: true, allowPointerLock: true, allowFullscreen: true });
            frame.style.cssText = 'width:100%;height:100%;min-width:1100px;min-height:600px;flex-shrink:0;border:0';
            frame.addEventListener('load', () => { if (connected) { revoke(); display('disconnected'); } });
            surface.replaceChildren(frame);
        } catch (error) { if (alive()) status.textContent = errorText(error); }
    }
    window.ZiroDesignerApp = { render };
})();
