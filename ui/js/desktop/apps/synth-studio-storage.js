(function () {
    'use strict';

    const NAMESPACE = 'synth-studio';
    const EXTENSION = '.aurasynth';

    function create(ctx, options) {
        ctx = ctx || {};
        options = options || {};
        const windowId = String(ctx.windowId || 'untitled');
        let path = '', version = null, queue = null, disposed = false, busy = false;
        let readonly = !!ctx.readonly;
        let pendingTarget = '', lastDraft = null, request = null, writeRequest = null, draftTimer = null, draftDeadline = null, draftToken = 0;
        let loadedText = '', generation = 0;
        const state = { name: 'Untitled', path: '', dirty: false, saving: false, loading: false, error: null, readonly, recovered: false, revision: 0 };

        const emit = patch => {
            Object.assign(state, patch || {}, { path, name: basename(path) || 'Untitled', dirty: !!queue?.dirty, saving: !!queue?.pending, revision: queue?.revision || 0 });
            options.onState?.({ ...state });
        };
        const model = () => {
            if (typeof options.validate === 'function') return options.validate;
            if (window.SynthStudioModel?.validate) return window.SynthStudioModel.validate.bind(window.SynthStudioModel);
            throw new Error('Synth Studio project validator is unavailable');
        };
        const validate = value => model()(value);
        const projectJSON = async () => JSON.stringify(validate(await options.serialize()));
        const basename = value => String(value || '').split('/').pop() || '';
        const directory = value => String(value || '').split('/').slice(0, -1).join('/');
        const sessionKey = String(ctx.sessionKey || ctx.state?.windows?.get(windowId)?.sessionKey || windowId);
        const draftKey = (target = path) => location.origin + ':synth-studio:' + (target || ('window:' + sessionKey));
        const draftID = () => {
            const modelNewID = window.SynthStudioModel?.newID;
            if (typeof modelNewID === 'function') return modelNewID('draft');
            return `draft-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}`;
        };
        const setError = error => emit({ error: error || null });
        const updatePath = next => {
            path = next || '';
            ctx.updateWindowContext?.(windowId, { path });
            emit({ recovered: false, error: null });
        };

        async function writeDraft(content, revision) {
            if (disposed) return false;
            const key = draftKey();
            const draft = { id: draftID(), content, revision, path, etag: version, updated: Date.now(), key };
            try {
                await OfficeSession.draft('put', key, draft, NAMESPACE);
                if (disposed) return false;
                lastDraft = draft;
                return true;
            } catch (error) {
                if (!disposed) emit({ error });
                return false;
            }
        }

        async function clearDraft() {
            draftToken++;
            clearTimeout(draftTimer);
            clearTimeout(draftDeadline);
            draftTimer = draftDeadline = null;
            const current = lastDraft;
            if (!current) return;
            lastDraft = null;
            await OfficeSession.draft('deleteIf', current.key, { id: current.id }, NAMESPACE).catch(() => {});
        }

        async function writeFile(content) {
            if (readonly) throw new Error('Desktop is read-only');
            const target = pendingTarget || path;
            if (!target) throw new Error('Choose a save location first');
            const expected = target === path ? version : null;
            const token = generation;
            writeRequest?.abort();
            const activeRequest = new AbortController();
            writeRequest = activeRequest;
            try {
                const saved = await ctx.api('/api/desktop/file', {
                    method: 'PUT',
                    signal: activeRequest.signal,
                    headers: { 'Content-Type': 'application/json', ...(expected ? { 'If-Match': expected } : { 'If-None-Match': '*' }) },
                    body: JSON.stringify({ path: target, content })
                });
                if (disposed || token !== generation) return saved;
                updatePath(saved.path || target);
                version = saved.version || null;
                pendingTarget = '';
                return saved;
            } finally {
                if (writeRequest === activeRequest) writeRequest = null;
            }
        }

        function makeQueue(dirty) {
            queue?.dispose();
            queue = OfficeSession.create({
                readonly: false,
                serialize: projectJSON,
                backup: writeDraft,
                write: writeFile,
                clearBackup: clearDraft,
                onState: value => emit({ error: value.error || null, dirty: value.dirty, saving: value.saving })
            });
            if (!path || readonly) queue.suspend();
            if (dirty) queue.changed();
            emit();
        }

        function scheduleDraft() {
            ++draftToken;
            clearTimeout(draftTimer);
            const save = async () => {
                const token = draftToken;
                clearTimeout(draftTimer);
                clearTimeout(draftDeadline);
                draftTimer = draftDeadline = null;
                const active = queue, revision = active?.revision;
                if (disposed || !active?.dirty) return;
                try {
                    const content = await projectJSON();
                    if (disposed || active !== queue || !active.dirty || revision !== active.revision || token !== draftToken) return;
                    await writeDraft(content, revision);
                } catch (error) { if (!disposed) setError(error); }
            };
            draftTimer = setTimeout(save, 600);
            if (!draftDeadline) draftDeadline = setTimeout(save, 4000);
        }

        async function readFile(target, token) {
            request?.abort();
            request = new AbortController();
            const response = await ctx.api('/api/desktop/file?path=' + encodeURIComponent(target), { signal: request.signal });
            if (disposed || token !== generation) return false;
            if (!/\.aurasynth$/i.test(target)) throw new Error('Open a .aurasynth project file');
            const project = validate(response.content);
            loadedText = JSON.stringify(project);
            path = target;
            version = response.version || null;
            makeQueue(false);
            await options.onLoaded?.(project, { path, name: basename(path), recovered: false });
            if (disposed || token !== generation) return false;
            ctx.updateWindowContext?.(windowId, { path });
            emit({ recovered: false, error: null });
            return true;
        }

        async function acceptRecovery(draft, detail) {
            if (typeof options.onRecovery === 'function') return !!(await options.onRecovery(draft, detail));
            return !!(await ctx.confirmDialog?.(
                ctx.t?.('synthStudio.recover') || 'Recover draft',
                ctx.t?.('synthStudio.recover_prompt') || 'A recovered Synth Studio draft is available. Open it?'
            ));
        }

        async function inspectDraft() {
            let backup;
            try { backup = await OfficeSession.draft('get', draftKey(), undefined, NAMESPACE); }
            catch (error) { if (!disposed) emit({ error }); return; }
            if (disposed) return;
            if (!backup?.content) return;
            if (backup.content === loadedText) {
                await OfficeSession.draft('deleteIf', backup.key || draftKey(), { id: backup.id }, NAMESPACE).catch(() => {});
                return;
            }
            if (!await acceptRecovery(backup, { path, version }) || disposed) return;
            try {
                const project = validate(backup.content);
                const stale = !!(backup.path && backup.path === path && backup.etag && backup.etag !== version);
                if (stale) updatePath('');
                await options.onLoaded?.(project, { path, name: basename(path) || 'Recovered', recovered: true, conflict: stale });
                if (disposed) return;
                makeQueue(true);
                emit({ recovered: true, error: stale ? new Error('The project changed on disk. Save the recovered draft as a new file.') : null });
            } catch (error) { if (!disposed) emit({ error, recovered: true }); }
        }

        async function initialize() {
            state.loading = true;
            emit();
            try {
                const initialPath = String(ctx.path || '');
                if (initialPath) await readFile(initialPath, ++generation);
                else {
                    path = '';
                    version = null;
                    makeQueue(false);
                    await options.onLoaded?.(null, { path: '', name: 'Untitled', recovered: false });
                }
                if (!disposed) await inspectDraft();
            } catch (error) { if (!disposed) setError(error); }
            finally {
                if (!disposed) {
                    state.loading = false;
                    emit();
                }
            }
            return state;
        }

        async function open(target) {
            if (disposed || busy || state.loading) return false;
            state.loading = true;
            emit();
            try {
                if (!target) {
                    const choice = await ctx.openFileDialog?.({
                        title: ctx.t?.('synthStudio.open') || 'Open project',
                        initialPath: directory(path) || 'Documents/Synth Studio',
                        filters: [{ label: 'Synth Studio', extensions: [EXTENSION] }]
                    });
                    if (disposed || !choice?.path || choice.canceled) return false;
                    target = choice.path;
                }
                if (!/\.aurasynth$/i.test(target)) { setError(new Error('Open a .aurasynth project file')); return false; }
                await options.beforeLoad?.();
                if (disposed || !await guard({ forReplacement: true }) || disposed) return false;
                busy = true;
                const token = ++generation;
                const ok = await readFile(target, token);
                if (disposed || token !== generation) return false;
                if (ok) { loadedText = JSON.stringify(validate(await options.serialize())); emit({ error: null }); }
                return ok;
            } catch (error) { if (!disposed) setError(error); return false; }
            finally {
                busy = false;
                if (!disposed) {
                    state.loading = false;
                    emit();
                }
            }
        }

        async function newDocument(project, config) {
            if (disposed || readonly || busy || state.loading) return false;
            state.loading = true;
            emit();
            try {
                await options.beforeLoad?.();
                if (disposed || readonly || !await guard({ forReplacement: true }) || disposed) return false;
                busy = true;
                ++generation;
                request?.abort();
                const normalized = validate(project);
                path = '';
                version = null;
                pendingTarget = '';
                loadedText = '';
                await options.onLoaded?.(normalized, { path: '', name: 'Untitled', recovered: false });
                if (disposed) return false;
                makeQueue(config?.dirty === true);
                if (config?.dirty) scheduleDraft();
                emit({ recovered: false, error: null });
                return true;
            } catch (error) { setError(error); return false; }
            finally {
                busy = false;
                if (!disposed) {
                    state.loading = false;
                    emit();
                }
            }
        }

        async function save(saveAs) {
            if (disposed || readonly || !queue || busy) return false;
            let target = path;
            if (saveAs || !target) {
                const choice = await ctx.saveFileDialog?.({
                    title: ctx.t?.('synthStudio.save') || 'Save project',
                    initialPath: directory(path) || 'Documents/Synth Studio',
                    defaultName: basename(path) || 'Untitled' + EXTENSION,
                    defaultExtension: EXTENSION,
                    filters: [{ label: 'Synth Studio', extensions: [EXTENSION] }]
                });
                if (disposed || readonly || !choice?.path || choice.canceled) return false;
                target = choice.path;
            }
            if (disposed || readonly) return false;
            if (!/\.aurasynth$/i.test(target)) { setError(new Error('Save as a .aurasynth project file')); return false; }
            pendingTarget = target;
            if (target !== path) queue.suspend();
            if (target !== path && !queue.dirty) queue.changed();
            try {
                await queue.save();
                if (disposed) return false;
                loadedText = await projectJSON();
                if (disposed) return false;
                ctx.loadBootstrap?.();
                return true;
            } catch (error) { if (!disposed) emit({ error }); return false; }
            finally {
                pendingTarget = '';
                if (queue && path) queue.resume();
            }
        }

        async function guard(config) {
            if (disposed || busy) return false;
            if (!queue?.dirty) return true;
            if (config?.forReplacement && !path) {
                if (readonly) {
                    try { return await writeDraft(await projectJSON(), queue.revision); }
                    catch (_) { return false; }
                }
                return save(true);
            }
            if (path) {
                try { await queue.save(); return !disposed && !queue.dirty; }
                catch (_) { /* Keep the recovery draft when saving is unavailable. */ }
            }
            let backup = lastDraft;
            try {
                const content = await projectJSON();
                if (disposed) return false;
                backup = await writeDraft(content, queue.revision) ? lastDraft : null;
            } catch (_) { return false; }
            if (!backup) return false;
            if (typeof options.onLeaveDraft === 'function') return !disposed && !!(await options.onLeaveDraft(backup, state.error)) && !disposed;
            const leave = await ctx.confirmDialog?.(
                ctx.t?.('synthStudio.unsaved') || 'Unsaved project',
                ctx.t?.('synthStudio.leave_draft') || 'Leave the project as a recoverable draft?'
            );
            return !disposed && !!leave;
        }

        function changed() {
            if (disposed || readonly || !queue) return;
            queue.changed();
            scheduleDraft();
        }

        function dispose() {
            if (disposed) return;
            disposed = true;
            ++generation;
            draftToken++;
            clearTimeout(draftTimer);
            clearTimeout(draftDeadline);
            request?.abort();
            writeRequest?.abort();
            queue?.dispose();
            if (typeof document !== 'undefined') document.removeEventListener('aurago:desktop-policy', onPolicy);
        }

        function onPolicy(event) {
            if (typeof event?.detail?.readonly !== 'boolean' || readonly === event.detail.readonly) return;
            readonly = event.detail.readonly;
            emit({ readonly });
            if (readonly) {
                queue?.suspend();
                writeRequest?.abort();
            }
            else if (path) {
                queue?.resume();
                if (queue?.dirty) queue.changed();
            }
        }

        if (typeof document !== 'undefined') document.addEventListener('aurago:desktop-policy', onPolicy);

        const api = { ready: initialize(), open, newDocument, save, changed, guard, dispose, get state() { return { ...state }; } };
        return api;
    }

    window.SynthStudioStorage = { create };
})();
