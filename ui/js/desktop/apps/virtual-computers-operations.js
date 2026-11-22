    async function launch(state) {
        const modal = state.modal;
        if (!modal || modal.type !== 'launch' || modal.pending) return;
        const read = role => modalField(state, role);
        const body = {
            template: read('template')?.value || 'python',
            allow_internet: capabilities(state).internet === true && read('network')?.value === 'internet'
        };
        const name = (read('launch-name')?.value || '').trim();
        if (name) body.name = name;
        if (read('launch-persistent')?.checked) body.persistent = true;
        else body.ttl_seconds = Number(read('ttl')?.value || limits(state).defaultTTL);
        const volumeId = read('launch-volume')?.value || '';
        if (volumeId) body.volume_id = volumeId;
        await runModalAction(state, async () => {
            const result = await request('/api/virtual-computers/machines', jsonRequest('POST', body));
            const created = result && result.machine && result.machine.id;
            if (created) {
                disconnectRemoteSessions(state);
                state.selectedMachineId = created;
                state.detailMode = 'overview';
                state.selectedShot = null;
                state.screenshotLoading = false;
            }
            await refresh(state);
            return result;
        }, tx(state.context, K + 'toast_launched'), () => {
            if (modal.afterLaunchSection && state.activeSection !== modal.afterLaunchSection) switchSection(state, modal.afterLaunchSection);
        });
    }

    async function destroyMachine(state, id) {
        if (!id || isPending(state, 'destroy')) return false;
        if (state.vncMachineId === id) showOverview(state);
        if (state.terminalMachineId === id) showOverview(state);
        setPending(state, 'destroy', true);
        try {
            await request('/api/virtual-computers/machines/' + encodeURIComponent(id), { method: 'DELETE' });
            await refresh(state);
            return true;
        } finally {
            setPending(state, 'destroy', false);
        }
    }

    function isCurrentScreenshotRequest(state, id, requestID) {
        return state.detailMode === 'screenshot' && state.selectedMachineId === id && state.screenshotRequestID === requestID;
    }

    function settleScreenshot(state, id, requestID, shot) {
        if (!isCurrentScreenshotRequest(state, id, requestID)) return false;
        state.selectedShot = shot || null;
        state.screenshotLoading = false;
        return true;
    }

    async function screenshot(state, id) {
        const machine = state.machines.find(item => item.id === id);
        if (!id || !machine || machine.display !== true) return;
        const requestID = ++state.screenshotRequestID;
        disconnectRemoteSessions(state);
        state.activeSection = 'machines';
        state.detailMode = 'screenshot';
        state.selectedMachineId = id;
        state.selectedShot = null;
        state.screenshotLoading = true;
        draw(state);
        try {
            const body = await request('/api/virtual-computers/machines/' + encodeURIComponent(id) + '/screenshot');
            if (settleScreenshot(state, id, requestID, body.screenshot)) draw(state);
        } catch (error) {
            if (!settleScreenshot(state, id, requestID, null)) return;
            notify(state, tx(state.context, 'desktop.virtual_computers_error') + ' ' + errorText(state, error), 'error');
            draw(state);
        }
    }

    function openModal(state, modal, trigger) {
        modal.renderKey = ++state.modalSeq;
        modal.returnSelector = stableSelector(trigger);
        modal.error = modal.error || '';
        modal.pending = false;
        state.modal = modal;
        state.modalReturnFocus = trigger || state.host.ownerDocument.activeElement;
        draw(state);
    }

    function openConfirmModal(state, options) {
        const definition = confirmDefinitions[options.kind] || {};
        openModal(state, {
            type: 'confirm',
            kind: options.kind,
            title: options.title || definition.title,
            message: options.message || definition.message,
            target: options.target || '',
            confirmLabel: options.confirmLabel || definition.label,
            confirmAction: options.confirmAction || definition.confirmAction || 'confirm-' + options.kind,
            danger: options.danger !== undefined ? options.danger !== false : definition.danger !== false,
            run: options.run,
            success: options.success,
            after: options.after
        }, options.trigger);
    }

    function confirmModalAction(state) {
        const modal = state.modal;
        if (!modal || modal.type !== 'confirm' || modal.pending || typeof modal.run !== 'function') return;
        runModalAction(state, modal.run, modal.success, modal.after);
    }

    // Runs the modal's request: busy state while it runs, close and toast on success,
    // an inline error (and the dialog stays open) on failure.
    async function runModalAction(state, work, success, after) {
        const modal = state.modal;
        if (!modal || modal.pending) return false;
        modal.pending = true;
        modal.error = '';
        patchModal(state);
        let result;
        try {
            result = await work();
        } catch (error) {
            if (state.disposed) return false;
            modal.pending = false;
            if (state.modal === modal) modal.error = errorText(state, error);
            draw(state);
            if (state.modal === modal) focusModalRetry(state);
            return false;
        }
        if (state.disposed) return false;
        modal.pending = false;
        if (state.modal === modal) closeModal(state);
        const message = typeof success === 'function' ? success(result) : success;
        if (message) toast(state, message);
        if (typeof after === 'function') after(result);
        return true;
    }

    // After a failed submit the focus returns to the first invalid field, else to the submit button.
    function focusModalRetry(state) {
        const host = state.host.querySelector('[data-role="modal-host"]');
        if (!host) return;
        const target = host.querySelector('[data-role="modal"] [aria-invalid="true"]') || host.querySelector('[data-role="modal-submit"]');
        if (target && typeof target.focus === 'function') target.focus();
    }

    function closeModal(state, restoreFocus = true) {
        const modal = state.modal;
        const returnFocus = state.modalReturnFocus;
        state.modal = null;
        state.modalReturnFocus = null;
        draw(state);
        if (!restoreFocus) return;
        let target = null;
        if (modal && modal.returnSelector) {
            try { target = state.host.querySelector(modal.returnSelector); } catch (_) { target = null; }
        }
        if (!target && returnFocus && returnFocus.isConnected !== false && typeof returnFocus.focus === 'function') target = returnFocus;
        if (!target) target = state.host.querySelector('[data-action="refresh"]');
        if (target && typeof target.focus === 'function') target.focus();
    }

    function downloadURL(machineId, path) {
        return '/api/virtual-computers/machines/' + encodeURIComponent(machineId) + '/download?path=' + encodeURIComponent(path);
    }

    function readFileBase64(file) {
        return new Promise((resolve, reject) => {
            const reader = new FileReader();
            reader.onerror = () => reject(reader.error || new Error('read failed'));
            reader.onload = () => {
                const text = String(reader.result || '');
                resolve(text.slice(text.indexOf(',') + 1));
            };
            reader.readAsDataURL(file);
        });
    }

    async function renameMachine(state) {
        const modal = state.modal;
        if (!modal || modal.type !== 'rename-machine') return;
        const name = (modalField(state, 'rename-name')?.value || '').trim();
        await runModalAction(state, async () => {
            await request('/api/virtual-computers/machines/' + encodeURIComponent(modal.id), jsonRequest('PATCH', { name }));
            await refresh(state);
        }, tx(state.context, K + 'toast_renamed'));
    }

    async function extendMachine(state, id, ttl) {
        if (!id || !ttl) return;
        await mutate(state, 'extend-' + id, 'machines', '/api/virtual-computers/machines/' + encodeURIComponent(id) + '/extend', jsonRequest('POST', { ttl_seconds: ttl }), {
            success: txv(state.context, K + 'toast_extended', { count: Math.round(ttl / 60) })
        });
    }

    function openSaveMachine(state, id, trigger) {
        const machine = state.machines.find(item => item.id === id) || {};
        openModal(state, {
            type: 'save-machine',
            id,
            volumeId: machine.volume_id || '',
            suggestedName: machine.label || '',
            progressKey: K + 'save_progress'
        }, trigger);
    }

    async function saveMachine(state) {
        const modal = state.modal;
        if (!modal || modal.type !== 'save-machine') return;
        const target = modalField(state, 'save-target')?.value || '__new__';
        const name = (modalField(state, 'save-name')?.value || '').trim();
        await runModalAction(state, async () => {
            let volumeId = target;
            if (target === '__new__') {
                // A retry after a failed save reuses the volume this dialog already created.
                if (!modal.createdVolumeId) {
                    const ttl = Math.min(604800, limits(state).volumeMaxTTL);
                    const created = await request('/api/virtual-computers/volumes', jsonRequest('POST', { name, ttl_seconds: ttl }));
                    modal.createdVolumeId = created && created.volume && created.volume.id;
                }
                volumeId = modal.createdVolumeId;
            }
            await request('/api/virtual-computers/machines/' + encodeURIComponent(modal.id) + '/save', jsonRequest('POST', { volume_id: volumeId }));
            await refreshResource(state, 'volumes');
        }, tx(state.context, K + 'toast_saved'));
    }

    // The checkpoint dialog needs the current volume list, so it is loaded before the dialog opens.
    async function openCheckpoint(state, id, trigger) {
        if (!id) return;
        if (capabilities(state).volumes) await refreshResource(state, 'volumes');
        if (state.disposed) return;
        openModal(state, { type: 'checkpoint', id, label: workspaceLabel(state, id), progressKey: K + 'save_progress' }, trigger);
    }

    async function checkpointWorkspace(state) {
        const modal = state.modal;
        if (!modal || modal.type !== 'checkpoint') return;
        const target = modalField(state, 'checkpoint-target')?.value || '__new__';
        const body = {};
        if (target === '__new__') {
            const name = (modalField(state, 'checkpoint-name')?.value || '').trim();
            if (name) body.name = name;
            const permanent = modalField(state, 'checkpoint-permanent');
            if (permanent && permanent.checked && !permanent.disabled) body.persistent = true;
        } else {
            body.volume_id = target;
        }
        await runModalAction(state, async () => {
            await workspaceCall(state, modal.id, '/checkpoint', jsonRequest('POST', body));
            if (capabilities(state).volumes) await refreshResource(state, 'volumes');
        }, tx(state.context, K + 'toast_checkpoint_created'));
    }

    async function assignMachine(state) {
        const modal = state.modal;
        if (!modal || modal.type !== 'assign-agent') return;
        const text = (modalField(state, 'assign-text')?.value || '').trim();
        if (!text) {
            setModalError(state, tx(state.context, K + 'assign_task_required'));
            modalField(state, 'assign-text')?.focus();
            return;
        }
        await runModalAction(state, async () => {
            const result = await request('/api/virtual-computers/machines/' + encodeURIComponent(modal.id) + '/assign-agent', jsonRequest('POST'));
            await refresh(state);
            return result;
        }, tx(state.context, K + 'toast_assigned'), result => {
            const workspace = result && result.workspace;
            if (workspace && workspace.id) sendToAgent(state, workspace.id, text, true);
        });
    }

    async function uploadFile(state) {
        const modal = state.modal;
        if (!modal || modal.type !== 'upload') return;
        const file = modalField(state, 'upload-file')?.files?.[0];
        if (!file) {
            setModalError(state, tx(state.context, K + 'upload_choose'));
            return;
        }
        if (file.size > uploadLimitBytes) {
            setModalError(state, txv(state.context, K + 'upload_too_large', { size: formatBytes(uploadLimitBytes, state.context) }));
            return;
        }
        await runModalAction(state, async () => {
            const data = await readFileBase64(file);
            await request('/api/virtual-computers/machines/' + encodeURIComponent(modal.id) + '/upload', jsonRequest('POST', { filename: file.name, content_base64: data }));
        }, tx(state.context, K + 'toast_uploaded'));
    }

    async function downloadFile(state) {
        const modal = state.modal;
        if (!modal || modal.type !== 'download') return;
        const path = (modalField(state, 'download-path')?.value || '').trim();
        if (!path) {
            setModalError(state, tx(state.context, K + 'download_path_required'));
            return;
        }
        await runModalAction(state, async () => {
            const resp = await fetch(downloadURL(modal.id, path));
            if (!resp.ok) {
                const failure = await readBody(resp);
                throw responseError(resp, failure, typeof failure === 'string' ? failure.trim().slice(0, 4096) || resp.statusText : resp.statusText);
            }
            const blob = await resp.blob();
            const doc = state.host.ownerDocument;
            const link = doc.createElement('a');
            const href = URL.createObjectURL(blob);
            link.href = href;
            link.download = path.split('/').filter(Boolean).pop() || 'download';
            link.rel = 'noopener';
            doc.body.appendChild(link);
            link.click();
            link.remove();
            setTimeout(() => URL.revokeObjectURL(href), 10000);
        }, tx(state.context, K + 'toast_downloaded'));
    }

    async function createVolume(state) {
        const modal = state.modal;
        if (!modal || modal.type !== 'create-volume') return;
        const name = (modalField(state, 'volume-name')?.value || '').trim();
        const retention = modalField(state, 'volume-retention')?.value || '';
        const body = { name };
        if (retention === 'permanent') body.persistent = true;
        else body.ttl_seconds = Number(retention) || 86400;
        if (modalField(state, 'volume-lock')?.checked) body.locked = true;
        await runModalAction(state, async () => {
            await request('/api/virtual-computers/volumes', jsonRequest('POST', body));
            await refreshResource(state, 'volumes');
        }, tx(state.context, K + 'toast_volume_created'));
    }

    async function importVolume(state) {
        const modal = state.modal;
        if (!modal || modal.type !== 'import-volume') return;
        const id = (modalField(state, 'volume-import-id')?.value || '').trim();
        if (!id) {
            setModalError(state, tx(state.context, K + 'volume_import_required'));
            return;
        }
        await runModalAction(state, async () => {
            await request('/api/virtual-computers/volumes/' + encodeURIComponent(id), { method: 'GET' });
            await refreshResource(state, 'volumes');
        }, tx(state.context, K + 'toast_volume_imported'));
    }

    async function renameVolume(state) {
        const modal = state.modal;
        if (!modal || modal.type !== 'rename-volume') return;
        const name = (modalField(state, 'rename-name')?.value || '').trim();
        await runModalAction(state, async () => {
            await request('/api/virtual-computers/volumes/' + encodeURIComponent(modal.id), jsonRequest('PATCH', { name }));
            await refreshResource(state, 'volumes');
        }, tx(state.context, K + 'toast_volume_updated'));
    }

    async function updateVolumeRetention(state) {
        const modal = state.modal;
        if (!modal || modal.type !== 'volume-retention') return;
        const retention = modalField(state, 'volume-retention')?.value || '';
        const body = retention === 'permanent' ? { persistent: true } : { persistent: false, ttl_seconds: Number(retention) || 86400 };
        await runModalAction(state, async () => {
            await request('/api/virtual-computers/volumes/' + encodeURIComponent(modal.id), jsonRequest('PATCH', body));
            await refreshResource(state, 'volumes');
        }, tx(state.context, K + 'toast_volume_updated'));
    }

    async function toggleVolumeLock(state, id) {
        const volume = state.volumes.find(item => item.id === id);
        if (!id || !volume) return;
        const lock = !volume.locked;
        await mutate(state, 'volume-' + id, 'volumes', '/api/virtual-computers/volumes/' + encodeURIComponent(id), jsonRequest('PATCH', { locked: lock }), {
            success: tx(state.context, K + (lock ? 'toast_volume_locked' : 'toast_volume_unlocked'))
        });
    }

    function copyVolumeId(state, id) {
        const clipboard = typeof navigator !== 'undefined' ? navigator.clipboard : null;
        const failed = () => notify(state, tx(state.context, K + 'toast_copy_failed'), 'error');
        if (!id || !clipboard || typeof clipboard.writeText !== 'function') {
            failed();
            return;
        }
        Promise.resolve().then(() => clipboard.writeText(id)).then(() => toast(state, tx(state.context, K + 'toast_id_copied')), failed);
    }

    // Workspace requests that throw so a confirmation dialog can show the failure inline.
    async function workspaceCall(state, workspaceId, suffix, options, refreshMachines) {
        await request('/api/virtual-computers/workspaces/' + encodeURIComponent(workspaceId) + (suffix || ''), options || {});
        if (refreshMachines) await refresh(state);
        else await refreshResource(state, 'workspaces');
    }

    function confirmCloseWorkspace(state, id, trigger) {
        const workspace = state.workspaces.find(item => item.id === id) || {};
        const adopted = workspace.origin === 'adopted';
        openConfirmModal(state, {
            kind: adopted ? 'release-workspace' : 'close-workspace',
            target: workspaceLabel(state, id),
            trigger,
            run: () => workspaceCall(state, id, '', { method: 'DELETE' }, true),
            success: tx(state.context, K + (adopted ? 'toast_workspace_released' : 'toast_workspace_closed'))
        });
    }

    async function workspaceMutation(state, workspaceId, suffix, options, extra) {
        if (!workspaceId || isPending(state, 'workspace-' + workspaceId)) return null;
        return mutate(state, 'workspace-' + workspaceId, 'workspaces', '/api/virtual-computers/workspaces/' + encodeURIComponent(workspaceId) + (suffix || ''), options || {}, extra);
    }

    async function observeWorkspace(state, workspaceId, machineId, takeControl) {
        window.VirtualComputersWorkspaces.controlAndOpen(state, workspaceId, machineId, takeControl,
            { request, notify, tx, canUseVNC, draw, workspaceMutation, disconnectRemoteSessions, disconnectVNC, setVNCExpanded });
    }

    async function deleteVolume(state, id) {
        if (!id) return false;
        await request('/api/virtual-computers/volumes/' + encodeURIComponent(id), { method: 'DELETE' });
        await refreshResource(state, 'volumes');
        return true;
    }

    // Direct (dialog-free) mutations: returns the response body, or null after reporting the failure.
    async function mutate(state, actionKey, resource, path, options, extra) {
        setPending(state, actionKey, true);
        draw(state);
        try {
            const body = await request(path, options);
            state.resourceErrors[resource] = '';
            if (resource === 'machines') await refresh(state);
            else await refreshResource(state, resource);
            if (extra && extra.success) toast(state, extra.success);
            return body === '' || body == null ? {} : body;
        } catch (error) {
            notify(state, tx(state.context, 'desktop.virtual_computers_error') + ' ' + errorText(state, error), 'error');
            return null;
        } finally {
            setPending(state, actionKey, false);
            draw(state);
        }
    }

    function notify(state, message, type) {
        if (state.context && typeof state.context.notify === 'function') {
            state.context.notify({
                title: tx(state.context, 'desktop.notification'),
                message,
                type: type || 'info'
            });
        }
    }

    function toast(state, message) {
        notify(state, message, 'success');
    }

    // Install / Repair: confirm, start the request and follow the run through the status.
    function confirmRunSetup(state, trigger) {
        if (!isMutable(state) || setupRunning(state)) return;
        openConfirmModal(state, {
            kind: 'run-setup',
            trigger,
            run: () => startSetupInstall(state),
            success: tx(state.context, K + 'toast_setup_started')
        });
    }

    // The install request answers only when the whole run has finished (10-25 minutes). A quick
    // refusal (403, 409) is thrown into the dialog; otherwise the dialog closes after a short wait
    // and the running banner takes over while the request completes in the background.
    async function startSetupInstall(state) {
        const pending = request('/api/virtual-computers/setup/install', jsonRequest('POST', {}));
        let failure = null;
        const settled = pending.then(() => { failure = null; }, error => { failure = error; });
        const wait = new Promise(resolve => {
            state.setupAcceptTimer = setTimeout(() => {
                state.setupAcceptTimer = null;
                resolve();
            }, setupAcceptDelayMs);
        });
        await Promise.race([settled, wait]);
        if (state.setupAcceptTimer) {
            clearTimeout(state.setupAcceptTimer);
            state.setupAcceptTimer = null;
        }
        if (state.disposed) return null;
        if (failure) throw failure;
        state.status = Object.assign({}, state.status, {
            repair_required: false,
            setup: { running: true, action: 'install', started_at: new Date().toISOString(), finished_at: '', last_state: '', last_message: '' }
        });
        state.setupSeenAt = Date.now();
        scheduleSetupStatusRefresh(state);
        settled.then(() => {
            if (state.disposed) return;
            const ok = !failure;
            notify(state, tx(state.context, K + (ok ? 'toast_setup_done' : 'toast_setup_failed')) + (ok ? '' : ' ' + errorText(state, failure)), ok ? 'success' : 'error');
            refresh(state);
        });
        return null;
    }

    // While a run is active the status is re-read every few seconds; the end of the run reloads everything.
    function scheduleSetupStatusRefresh(state) {
        if (state.setupPollTimer) {
            clearTimeout(state.setupPollTimer);
            state.setupPollTimer = null;
        }
        if (state.disposed || !setupRunning(state)) return;
        state.setupPollTimer = setTimeout(async () => {
            state.setupPollTimer = null;
            await refreshSetupStatus(state);
            scheduleSetupStatusRefresh(state);
        }, setupStatusPollIntervalMs);
    }

    async function refreshSetupStatus(state) {
        if (state.disposed || !isMachinePollingVisible(state)) return;
        const generation = state.refreshGeneration;
        try {
            const status = await request('/api/virtual-computers/setup/status');
            if (state.disposed || generation !== state.refreshGeneration || !status) return;
            const wasRunning = setupRunning(state);
            state.status = status;
            if (wasRunning && !setupRunning(state)) {
                await refresh(state);
                return;
            }
            draw(state);
        } catch (_) {
            // Best-effort; the next tick tries again.
        }
    }

    function normalizeMachineList(body) {
        return body && Array.isArray(body.machines) ? body.machines : [];
    }

    function storeMachines(state, machines) {
        const normalized = Array.isArray(machines) ? machines : [];
        state.machines = normalized;
        state.machineSnapshot = JSON.stringify(normalized);
    }

    function isMachinePollingVisible(state) {
        if (!state.host) return false;
        const document = state.host.ownerDocument;
        if (document && document.visibilityState === 'hidden') return false;
        const appWindow = typeof state.host.closest === 'function' ? state.host.closest('.vd-window') : null;
        if (appWindow && (appWindow.hidden || (appWindow.style && appWindow.style.display === 'none'))) return false;
        return typeof state.host.getClientRects !== 'function' || state.host.getClientRects().length > 0;
    }

    async function pollMachines(state) {
        if (state.disposed || state.machinePollInFlight || state.resourceLoading.machines || !isMachinePollingVisible(state)) return;
        const generation = state.refreshGeneration;
        state.machinePollInFlight = true;
        try {
            const body = await request('/api/virtual-computers/machines');
            if (state.disposed || generation !== state.refreshGeneration) return;
            const machines = normalizeMachineList(body);
            const snapshot = JSON.stringify(machines);
            if (snapshot === state.machineSnapshot) return;
            storeMachines(state, machines);
            state.resourceErrors.machines = '';
            reconcileSelection(state);
            reconcileVNC(state);
            reconcileTerminal(state);
            draw(state);
        } catch (_) {
            // Background polling is best-effort; explicit refresh reports errors.
        } finally {
            state.machinePollInFlight = false;
        }
    }

    function scheduleMachineRefresh(state) {
        if (state.machinePollTimer) {
            clearTimeout(state.machinePollTimer);
            state.machinePollTimer = null;
        }
        if (state.disposed) return;
        state.machinePollTimer = setTimeout(async () => {
            state.machinePollTimer = null;
            await pollMachines(state);
            scheduleMachineRefresh(state);
        }, machinePollIntervalMs);
    }

    function scheduleWorkspaceRefresh(state) {
        if (state.workspaceRefreshTimer) {
            clearTimeout(state.workspaceRefreshTimer);
            state.workspaceRefreshTimer = null;
        }
        if (state.disposed || state.activeSection !== 'workspaces' || !capabilities(state).agent_control) return;
        const active = state.workspaces.some(workspace => workspace && ['opening', 'ready', 'closing'].includes(workspace.state));
        if (!active) return;
        state.workspaceRefreshTimer = setTimeout(() => {
            state.workspaceRefreshTimer = null;
            refreshResource(state, 'workspaces');
        }, 3000);
    }

    function dispose(windowId) {
        const state = instances.get(windowId);
        if (!state) return;
        state.disposed = true;
        state.refreshGeneration++;
        if (state.workspaceRefreshTimer) clearTimeout(state.workspaceRefreshTimer);
        if (state.machinePollTimer) clearTimeout(state.machinePollTimer);
        if (state.expiryCountdownTimer) clearTimeout(state.expiryCountdownTimer);
        if (state.setupPollTimer) clearTimeout(state.setupPollTimer);
        if (state.setupAcceptTimer) clearTimeout(state.setupAcceptTimer);
        state.setupPollTimer = null;
        state.setupAcceptTimer = null;
        state.machinePollTimer = null;
        state.workspaceRefreshTimer = null;
        state.expiryCountdownTimer = null;
        disconnectRemoteSessions(state);
        if (state.clickHandler) state.host.removeEventListener('click', state.clickHandler);
        if (state.changeHandler) state.host.removeEventListener('change', state.changeHandler);
        if (state.pointerHandler) state.host.removeEventListener('mousedown', state.pointerHandler);
        if (state.keyHandler) state.host.removeEventListener('keydown', state.keyHandler);
        if (state.inputHandler) state.host.removeEventListener('input', state.inputHandler);
        instances.delete(windowId);
    }

    window.VirtualComputersApp = { render, dispose };
}());
