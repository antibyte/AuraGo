    function modalPane(state) {
        const modal = state.modal;
        if (!modal) return '';
        if (modal.type === 'launch') return launchModal(state, modal);
        if (modal.type === 'rename-machine') return renameModal(state, modal, 'desktop.virtual_computers_rename_title');
        if (modal.type === 'rename-volume') return renameModal(state, modal, 'desktop.virtual_computers_volume_rename_title');
        if (modal.type === 'save-machine') return saveMachineModal(state, modal);
        if (modal.type === 'assign-agent') return assignAgentModal(state, modal);
        if (modal.type === 'upload') return uploadModal(state, modal);
        if (modal.type === 'download') return downloadModal(state, modal);
        if (modal.type === 'create-volume') return createVolumeModal(state);
        if (modal.type === 'import-volume') return importVolumeModal(state);
        if (modal.type === 'volume-retention') return volumeRetentionModal(state, modal);
        if (modal.type === 'checkpoint') return checkpointModal(state, modal);
        if (modal.type === 'confirm') return confirmModal(state, modal);
        return '';
    }

    function modalField(state, role) {
        return state.host.querySelector(`[data-role="modal-host"] [data-role="${role}"]`);
    }

    function syncModalFields(state) {
        const modal = state.modal;
        if (!modal) return;
        if (modal.type === 'launch') {
            const persistent = modalField(state, 'launch-persistent');
            const ttl = modalField(state, 'ttl');
            if (ttl) ttl.disabled = !!(persistent && persistent.checked);
        } else if (modal.type === 'save-machine') {
            const target = modalField(state, 'save-target');
            const fields = modalField(state, 'save-new-fields');
            if (fields && target) fields.hidden = target.value !== '__new__';
        } else if (modal.type === 'checkpoint') {
            const target = modalField(state, 'checkpoint-target');
            const fields = modalField(state, 'checkpoint-new-fields');
            if (fields && target) fields.hidden = target.value !== '__new__';
        }
    }

    function requestCloseModal(state) {
        if (!state.modal || state.modal.pending) return;
        closeModal(state);
    }

    function bindActions(state) {
        state.clickHandler = event => {
            const raw = event.target;
            if (state.modal && raw && typeof raw.matches === 'function' && raw.matches('[data-role="modal"]')) {
                const armed = state.backdropDown !== false;
                state.backdropDown = null;
                if (armed) requestCloseModal(state);
                return;
            }
            const target = raw && raw.closest ? raw.closest('[data-action]') : null;
            if (!target || !state.host.contains(target)) return;
            const action = target.dataset.action;
            const id = target.dataset.id || '';
            if (target.dataset.role === 'modal-submit' && state.modal && state.modal.pending) return;
            if (action === 'refresh') refresh(state);
            else if (action === 'config') window.location.href = '/config#virtual_computers';
            else if (action === 'run-setup') confirmRunSetup(state, target);
            else if (action === 'section') switchSection(state, target.dataset.section || 'machines');
            else if (action === 'new-agent-job') {
                switchSection(state, 'workspaces');
                state.host.querySelector('[data-role="agent-request"]')?.focus();
            }
            else if (action === 'new-machine') openModal(state, { type: 'launch' }, target);
            else if (action === 'launch-with-volume') openModal(state, { type: 'launch', volumeId: id, afterLaunchSection: 'machines' }, target);
            else if (action === 'select-machine') selectMachine(state, id);
            else if (action === 'overview') showOverview(state);
            else if (action === 'confirm-launch') launch(state);
            else if (action === 'destroy') openConfirmModal(state, { kind: 'destroy', target: machineLabel(state, id), trigger: target, run: () => destroyMachine(state, id), success: tx(state.context, K + 'toast_destroyed') });
            else if (action === 'rename-machine') openModal(state, { type: 'rename-machine', id, value: (state.machines.find(item => item.id === id) || {}).label || '', submitAction: 'confirm-rename-machine' }, target);
            else if (action === 'confirm-rename-machine') renameMachine(state);
            else if (action === 'extend-machine') extendMachine(state, id, Number(target.dataset.ttl) || 0);
            else if (action === 'save-machine') openSaveMachine(state, id, target);
            else if (action === 'confirm-save-machine') saveMachine(state);
            else if (action === 'assign-agent') openModal(state, { type: 'assign-agent', id, label: machineLabel(state, id) }, target);
            else if (action === 'confirm-assign-agent') assignMachine(state);
            else if (action === 'upload-file') openModal(state, { type: 'upload', id, label: machineLabel(state, id) }, target);
            else if (action === 'confirm-upload') uploadFile(state);
            else if (action === 'download-file') openModal(state, { type: 'download', id, label: machineLabel(state, id) }, target);
            else if (action === 'confirm-download') downloadFile(state);
            else if (action === 'open-workspace') switchSection(state, 'workspaces');
            else if (action === 'screenshot') screenshot(state, id);
            else if (action === 'vnc') openVNC(state, id);
            else if (action === 'terminal') openTerminal(state, id);
            else if (action === 'ask-agent') openAgentRequest(state);
            else if (action === 'delete-volume') openConfirmModal(state, { kind: 'delete-volume', target: volumeLabel(state, id), trigger: target, run: () => deleteVolume(state, id), success: tx(state.context, K + 'toast_volume_deleted') });
            else if (action === 'create-volume') openModal(state, { type: 'create-volume' }, target);
            else if (action === 'confirm-create-volume') createVolume(state);
            else if (action === 'import-volume') openModal(state, { type: 'import-volume' }, target);
            else if (action === 'confirm-import-volume') importVolume(state);
            else if (action === 'rename-volume') openModal(state, { type: 'rename-volume', id, value: (state.volumes.find(item => item.id === id) || {}).name || '', submitAction: 'confirm-rename-volume' }, target);
            else if (action === 'confirm-rename-volume') renameVolume(state);
            else if (action === 'toggle-volume-lock') toggleVolumeLock(state, id);
            else if (action === 'volume-retention') openModal(state, { type: 'volume-retention', id, label: volumeLabel(state, id) }, target);
            else if (action === 'confirm-volume-retention') updateVolumeRetention(state);
            else if (action === 'copy-volume-id') copyVolumeId(state, id);
            else if (action === 'modal-confirm') confirmModalAction(state);
            else if (action === 'close-modal') requestCloseModal(state);
            else if (action === 'retry-resource') refreshResource(state, target.dataset.resource || 'status');
            else if (action === 'close-workspace') confirmCloseWorkspace(state, id, target);
            else if (action === 'checkpoint-workspace') openCheckpoint(state, id, target);
            else if (action === 'confirm-checkpoint') checkpointWorkspace(state);
            else if (action === 'observe-workspace') observeWorkspace(state, id, target.dataset.machineId || '', false);
            else if (action === 'check-native-desktop') window.VirtualComputersWorkspaces.checkNativeDesktop(state, id, { request, draw });
            else if (action === 'takeover-workspace') {
                const machineId = target.dataset.machineId || '';
                openConfirmModal(state, { kind: 'takeover-workspace', target: workspaceLabel(state, id), trigger: target, run: () => {}, after: () => observeWorkspace(state, id, machineId, true) });
            }
            else if (action === 'cancel-workspace-job') {
                const jobId = target.dataset.jobId || '';
                openConfirmModal(state, { kind: 'cancel-workspace-job', target: workspaceLabel(state, id), trigger: target, run: () => workspaceCall(state, id, '/jobs/' + encodeURIComponent(jobId), { method: 'DELETE' }), success: tx(state.context, K + 'toast_job_canceled') });
            }
            else if (action === 'approve-workspace-grant') {
                const grantId = target.dataset.grantId || '';
                openConfirmModal(state, { kind: 'approve-workspace-grant', target: workspaceLabel(state, id), trigger: target, run: () => workspaceCall(state, id, '/credential-grants/' + encodeURIComponent(grantId) + '/approve', jsonRequest('POST')), success: tx(state.context, K + 'toast_grant_approved') });
            }
            else if (action === 'revoke-workspace-grant') {
                const grantId = target.dataset.grantId || '';
                openConfirmModal(state, { kind: 'revoke-workspace-grant', target: workspaceLabel(state, id), trigger: target, run: () => workspaceCall(state, id, '/credential-grants/' + encodeURIComponent(grantId) + '/revoke', jsonRequest('POST')), success: tx(state.context, K + 'toast_grant_revoked') });
            }
        };
        state.changeHandler = () => syncModalFields(state);
        state.pointerHandler = event => {
            state.backdropDown = !!(event.target && typeof event.target.matches === 'function' && event.target.matches('[data-role="modal"]'));
        };
        state.keyHandler = event => {
            if (state.modal) {
                if (event.key === 'Escape') {
                    event.preventDefault();
                    requestCloseModal(state);
                } else if (event.key === 'Enter' && state.modal.type !== 'confirm' && event.target && event.target.tagName === 'INPUT' && event.target.type !== 'file' && event.target.type !== 'checkbox') {
                    const submit = state.host.querySelector('[data-role="modal-host"] [data-role="modal-submit"]');
                    if (submit && !submit.disabled && submit.getAttribute('aria-disabled') !== 'true') {
                        event.preventDefault();
                        submit.click();
                    }
                } else if (event.key === 'Tab') {
                    const focusable = Array.from(state.host.querySelectorAll('[data-role="modal"] button:not([disabled]), [data-role="modal"] select:not([disabled]), [data-role="modal"] textarea:not([disabled]), [data-role="modal"] input:not([disabled]), [data-role="modal"] a[href], [data-role="modal"] [tabindex]:not([tabindex="-1"])')).filter(node => !node.closest('[hidden]'));
                    if (!focusable.length) return;
                    const first = focusable[0];
                    const last = focusable[focusable.length - 1];
                    if (event.shiftKey && event.target === first) {
                        event.preventDefault();
                        last.focus();
                    } else if (!event.shiftKey && event.target === last) {
                        event.preventDefault();
                        first.focus();
                    }
                }
                return;
            }
            const currentTab = event.target.closest && event.target.closest('[role="tab"]');
            if (!currentTab || !['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return;
            const tabs = Array.from(state.host.querySelectorAll('[role="tab"]'));
            const currentIndex = tabs.indexOf(currentTab);
            if (currentIndex < 0 || !tabs.length) return;
            event.preventDefault();
            let nextIndex = event.key === 'Home' ? 0 : event.key === 'End' ? tabs.length - 1 : currentIndex + (event.key === 'ArrowRight' ? 1 : -1);
            nextIndex = (nextIndex + tabs.length) % tabs.length;
            const nextTab = tabs[nextIndex];
            switchSection(state, nextTab.dataset.section || 'machines');
            state.host.querySelector(`#${nextTab.id}`)?.focus();
        };
        state.host.addEventListener('click', state.clickHandler);
        state.host.addEventListener('change', state.changeHandler);
        state.host.addEventListener('mousedown', state.pointerHandler);
        state.host.addEventListener('keydown', state.keyHandler);
        state.inputHandler = event => {
            const field = event.target;
            if (field && typeof field.matches === 'function' && field.matches('[data-role="agent-request"]')) updateAgentRequestCount(state, field);
        };
        state.host.addEventListener('input', state.inputHandler);
    }

    function machineLabel(state, id) {
        const machine = state.machines.find(item => item.id === id);
        return machine ? machineName(machine) : shortId(id);
    }

    function volumeLabel(state, id) {
        return volumeName(state, id);
    }

    function workspaceLabel(state, id) {
        const workspace = state.workspaces.find(item => item.id === id);
        const machine = workspace ? state.machines.find(item => item.id === workspace.machine_id) : null;
        return machine ? machineName(machine) : shortId(id);
    }

    // Opens the agent chat for a workspace. An adopted workspace wraps a computer the user
    // handed over, so the guide tells the agent that closing it only releases it.
    function sendToAgent(state, workspaceId, requestText, adopted) {
        if (typeof state.context.openApp !== 'function') return false;
        let guide = 'Use virtual_workspace and virtual_browser for webpages; use virtual_computer_ui for native applications and dialogs when enabled. Keep the same workspace. Do not use legacy agent task tools.';
        if (adopted) guide += ' This computer was handed over by the user; closing the workspace releases it without destroying it.';
        state.context.openApp('agent-chat', {
            chat_prefill: requestText,
            chat_autosend: true,
            chat_source_app: 'virtual-computers',
            window_context: {
                app_id: 'virtual-computers',
                workspace_id: workspaceId,
                label: tx(state.context, 'desktop.virtual_computers_title'),
                purpose: 'Control isolated Firecracker workspaces with the AuraGo main agent.',
                guide
            }
        });
        return true;
    }

    function updateAgentRequestCount(state, input) {
        const counter = state.host.querySelector('[data-role="agent-request-count"]');
        const workspaces = window.VirtualComputersWorkspaces;
        if (!counter || !input || !workspaces || typeof workspaces.requestCountText !== 'function') return;
        counter.textContent = workspaces.requestCountText(state.context, input.value.length);
    }

    function openAgentRequest(state) {
        const input = state.host.querySelector('[data-role="agent-request"]');
        const choice = state.host.querySelector('[data-role="agent-workspace"]')?.value || '';
        const machineId = choice.startsWith('machine:') ? choice.slice('machine:'.length) : '';
        const workspaceId = machineId ? '' : choice;
        if (machineId) {
            const machine = state.machines.find(item => item.id === machineId);
            if (!machine || machine.workspace_id || String(machine.status || '').toLowerCase() !== 'running') {
                notify(state, tx(state.context, 'desktop.virtual_computers_workspace_unavailable'), 'error');
                return;
            }
        } else if (workspaceId && !state.workspaces.some(w => w.id === workspaceId && w.state === 'ready' && w.owner_session_id === 'virtual-desktop' && !w.mission_id)) {
            notify(state, tx(state.context, 'desktop.virtual_computers_workspace_unavailable'), 'error');
            return;
        }
        const requestText = input ? input.value.trim() : '';
        if (!requestText || typeof state.context.openApp !== 'function') {
            if (input) input.focus();
            return;
        }
        if (machineId) {
            assignAndSend(state, machineId, requestText);
            return;
        }
        const workspace = state.workspaces.find(w => w.id === workspaceId);
        sendToAgent(state, workspaceId, requestText, !!(workspace && workspace.origin === 'adopted'));
        input.value = '';
        updateAgentRequestCount(state, input);
    }

    // A running computer picked in the request box is handed to the agent first; the chat
    // then opens with the workspace the server created (or reused) for it.
    async function assignAndSend(state, machineId, requestText) {
        const key = 'assign-' + machineId;
        if (isPending(state, key)) return;
        if (setupRunning(state) || repairRequired(state)) {
            notify(state, tx(state.context, K + 'assign_blocked'), 'error');
            return;
        }
        setPending(state, key, true);
        try {
            const result = await request('/api/virtual-computers/machines/' + encodeURIComponent(machineId) + '/assign-agent', jsonRequest('POST'));
            if (state.disposed) return;
            const workspace = result && result.workspace;
            if (!workspace || !workspace.id) throw new Error(tx(state.context, 'desktop.virtual_computers_workspace_unavailable'));
            state.agentWorkspaceId = workspace.id;
            toast(state, tx(state.context, K + 'toast_assigned'));
            if (sendToAgent(state, workspace.id, requestText, workspace.origin !== 'launched')) {
                state.agentRequestDraft = '';
                const input = state.host.querySelector('[data-role="agent-request"]');
                if (input) {
                    input.value = '';
                    updateAgentRequestCount(state, input);
                }
            }
            await refresh(state);
            if (!state.disposed) {
                // The refresh re-read the old selection from the page; select the new workspace.
                state.agentWorkspaceId = workspace.id;
                const select = state.host.querySelector('[data-role="agent-workspace"]');
                if (select && Array.from(select.options).some(option => option.value === workspace.id)) select.value = workspace.id;
                // Rebuild now so the stale "unavailable" entry of the handed-over computer disappears.
                const pane = state.host.querySelector('[data-role="content"]');
                if (pane) delete pane.dataset.paneKey;
                draw(state);
            }
        } catch (error) {
            if (!state.disposed) notify(state, tx(state.context, 'desktop.virtual_computers_error') + ' ' + errorText(state, error), 'error');
        } finally {
            setPending(state, key, false);
        }
    }

    function selectMachine(state, id) {
        if (!id || state.selectedMachineId === id) return;
        disconnectRemoteSessions(state);
        state.selectedMachineId = id;
        state.selectedShot = null;
        state.screenshotLoading = false;
        state.detailMode = 'overview';
        draw(state);
    }

    function switchSection(state, section) {
        if (!['machines', 'workspaces', 'volumes'].includes(section) || state.activeSection === section) return;
        disconnectRemoteSessions(state);
        state.activeSection = section;
        state.modal = null;
        draw(state);
        if (section === 'volumes' && capabilities(state).volumes) refreshResource(state, 'volumes');
        if (section === 'workspaces' && capabilities(state).agent_control) refreshResource(state, 'workspaces');
        scheduleWorkspaceRefresh(state);
    }

    function applyResourceResult(state, resource, result) {
        state.resourceLoading[resource] = false;
        if (result.status === 'fulfilled') {
            state.resourceErrors[resource] = '';
            const body = result.value || {};
            if (resource === 'machines') storeMachines(state, normalizeMachineList(body));
            else if (resource === 'templates') {
                state.templates = Array.isArray(body.templates) ? body.templates : [];
                state.templatesFallback = state.templates.length === 0;
            } else if (resource === 'volumes') state.volumes = Array.isArray(body.volumes) ? body.volumes : [];
            else if (resource === 'workspaces') {
                state.workspaces = Array.isArray(body.workspaces) ? body.workspaces : [];
                state.workspaceSummaries = Array.isArray(body.workspace_summaries) ? body.workspace_summaries : [];
            }
            return;
        }
        state.resourceErrors[resource] = result.reason && result.reason.message ? result.reason.message : String(result.reason || '');
        if (resource === 'templates') state.templatesFallback = true;
    }

    async function refresh(state) {
        const generation = ++state.refreshGeneration;
        state.resourceLoading.status = true;
        state.resourceLoading.machines = true;
        state.resourceLoading.templates = true;
        draw(state);
        try {
            const status = await request('/api/virtual-computers/setup/status');
            if (state.disposed || generation !== state.refreshGeneration) return;
            state.status = status;
            state.resourceErrors.status = '';
            state.resourceLoading.status = false;
            if (!status.enabled) {
                storeMachines(state, []);
                state.templates = [];
                state.volumes = [];
                state.workspaces = [];
                state.workspaceSummaries = [];
                state.resourceLoading.machines = false;
                state.resourceLoading.templates = false;
                reconcileSection(state);
                reconcileVNC(state);
                reconcileTerminal(state);
                draw(state);
                return;
            }
            const resources = [
                ['machines', request('/api/virtual-computers/machines')],
                ['templates', request('/api/virtual-computers/templates')]
            ];
            if (status.capabilities && status.capabilities.volumes) {
                state.resourceLoading.volumes = true;
                resources.push(['volumes', request('/api/virtual-computers/volumes')]);
            }
            if (status.capabilities && status.capabilities.agent_control) {
                state.resourceLoading.workspaces = true;
                resources.push(['workspaces', request('/api/virtual-computers/workspaces')]);
            }
            const results = await Promise.allSettled(resources.map(item => item[1]));
            if (state.disposed || generation !== state.refreshGeneration) return;
            results.forEach((result, index) => applyResourceResult(state, resources[index][0], result));
        } catch (error) {
            if (state.disposed || generation !== state.refreshGeneration) return;
            state.resourceLoading.status = false;
            state.resourceLoading.machines = false;
            state.resourceLoading.templates = false;
            state.resourceErrors.status = error.message;
            notify(state, tx(state.context, 'desktop.virtual_computers_error') + ' ' + error.message, 'error');
        }
        reconcileSection(state);
        reconcileSelection(state);
        reconcileVNC(state);
        reconcileTerminal(state);
        draw(state);
        scheduleWorkspaceRefresh(state);
        scheduleSetupStatusRefresh(state);
    }

    async function refreshResource(state, resource) {
        if (resource === 'status' || resource === 'machines' || resource === 'templates') {
            await refresh(state);
            return;
        }
        const paths = {
            volumes: '/api/virtual-computers/volumes',
            workspaces: '/api/virtual-computers/workspaces'
        };
        const path = paths[resource];
        if (!path || state.resourceLoading[resource]) return;
        state.resourceLoading[resource] = true;
        draw(state);
        const result = await Promise.allSettled([request(path)]);
        if (state.disposed) return;
        applyResourceResult(state, resource, result[0]);
        draw(state);
        scheduleWorkspaceRefresh(state);
    }

    function reconcileSelection(state) {
        if (state.selectedMachineId && state.machines.some(item => item.id === state.selectedMachineId)) return;
        if (state.vncSession) disconnectVNC(state);
        if (state.terminalSession) disconnectTerminal(state);
        state.selectedMachineId = state.machines.length ? state.machines[0].id : null;
        state.selectedShot = null;
        state.screenshotLoading = false;
        state.detailMode = 'overview';
    }

    function reconcileSection(state) {
        const available = state.activeSection === 'machines'
            || (state.activeSection === 'volumes' && capabilities(state).volumes)
            || (state.activeSection === 'workspaces' && capabilities(state).agent_control);
        if (available) return;
        disconnectVNC(state);
        if (state.terminalSession) disconnectTerminal(state);
        state.activeSection = 'machines';
    }

    function showOverview(state) {
        disconnectRemoteSessions(state);
        state.detailMode = 'overview';
        state.selectedShot = null;
        state.screenshotLoading = false;
        draw(state);
    }

    function applyVNCLayoutState(state) {
        const root = state.host.querySelector('.vc-app');
        if (root) root.classList.toggle('is-vnc-expanded', state.vncExpanded);
    }

    function setVNCExpanded(state, expanded) {
        state.vncExpanded = expanded === true;
        const root = state.host.querySelector('.vc-app');
        if (root) root.classList.toggle('is-vnc-expanded', state.vncExpanded);
    }

    function openVNC(state, id, viewOnly) {
        const machine = state.machines.find(item => item.id === id);
        if (!canUseVNC(state, machine) || !window.VirtualComputersVNC || typeof window.VirtualComputersVNC.mount !== 'function') {
            notify(state, tx(state.context, 'desktop.virtual_computers_vnc_unavailable'), 'error');
            return;
        }
        disconnectRemoteSessions(state);
        state.activeSection = 'machines';
        state.detailMode = 'vnc';
        state.selectedMachineId = id;
        state.selectedShot = null;
        state.screenshotLoading = false;
        state.vncMachineId = id;
        draw(state);
        const preview = state.host.querySelector('[data-role="preview"]');
        if (!preview) return;
        preview.innerHTML = '<div class="vc-vnc-mount" data-role="vnc-mount"></div>';
        const mountPoint = preview.querySelector('[data-role="vnc-mount"]');
        const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const url = proto + '//' + window.location.host + '/api/virtual-computers/machines/' + encodeURIComponent(machine.id) + '/vnc';
        try {
            state.vncSession = window.VirtualComputersVNC.mount(mountPoint, {
                url,
                machineId: machine.id,
                viewOnly: viewOnly === true,
                t: key => tx(state.context, key),
                notify: (message, type) => notify(state, message, type),
                onExpandedChange: expanded => setVNCExpanded(state, expanded),
                onClose: () => {
                    if (state.detailMode === 'vnc' && state.vncMachineId === machine.id) showOverview(state);
                }
            });
        } catch (_) {
            state.vncSession = null;
            state.vncMachineId = null;
            state.detailMode = 'overview';
            setVNCExpanded(state, false);
            notify(state, tx(state.context, 'desktop.virtual_computers_vnc_unavailable'), 'error');
            draw(state);
        }
    }

    function disconnectVNC(state, preserveWorkspaceControl) {
        const session = state.vncSession;
        const workspaceId = state.vncWorkspaceId;
        const releaseControl = state.vncWorkspaceTakeover && preserveWorkspaceControl !== true;
        state.vncSession = null;
        state.vncMachineId = null;
        state.vncWorkspaceId = null;
        state.vncWorkspaceTakeover = false;
        state.workspaceControlRenewedAt = 0;
        setVNCExpanded(state, false);
        if (session && typeof session.disconnect === 'function') {
            try { session.disconnect(); } catch (_) {}
        }
        if (releaseControl && workspaceId && window.VirtualComputersWorkspaces) {
            window.VirtualComputersWorkspaces.releaseControl(state, workspaceId, { request, notify, tx });
        }
    }

    function reconcileVNC(state) {
        if (state.detailMode !== 'vnc') return;
        const machine = state.machines.find(item => item.id === state.vncMachineId);
        if (state.vncSession && canUseVNC(state, machine)) return;
        disconnectVNC(state);
        state.detailMode = 'overview';
        state.selectedShot = null;
    }

    function openTerminal(state, id) {
        const machine = state.machines.find(item => item.id === id);
        if (!canUseTerminal(state, machine) || !window.VirtualComputersTerminal || typeof window.VirtualComputersTerminal.mount !== 'function') {
            notify(state, tx(state.context, 'desktop.virtual_computers_terminal_unavailable'), 'error');
            return;
        }
        disconnectRemoteSessions(state);
        state.activeSection = 'machines';
        state.detailMode = 'terminal';
        state.selectedMachineId = id;
        state.selectedShot = null;
        state.screenshotLoading = false;
        state.terminalMachineId = id;
        draw(state);
        const preview = state.host.querySelector('[data-role="preview"]');
        if (!preview) return;
        preview.innerHTML = '<div class="vc-terminal-mount" data-role="terminal-mount"></div>';
        const mountPoint = preview.querySelector('[data-role="terminal-mount"]');
        const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const url = proto + '//' + window.location.host + '/api/virtual-computers/machines/' + encodeURIComponent(machine.id) + '/tty';
        try {
            state.terminalSession = window.VirtualComputersTerminal.mount(mountPoint, {
                url,
                machineId: machine.id,
                t: key => tx(state.context, key),
                notify: (message, type) => notify(state, message, type),
                onClose: () => {
                    if (state.detailMode === 'terminal' && state.terminalMachineId === machine.id) showOverview(state);
                }
            });
        } catch (_) {
            state.terminalSession = null;
            state.terminalMachineId = null;
            state.detailMode = 'overview';
            notify(state, tx(state.context, 'desktop.virtual_computers_terminal_unavailable'), 'error');
            draw(state);
        }
    }

    function disconnectTerminal(state) {
        const session = state.terminalSession;
        state.terminalSession = null;
        state.terminalMachineId = null;
        if (session && typeof session.disconnect === 'function') {
            try { session.disconnect(); } catch (_) {}
        }
    }

    function reconcileTerminal(state) {
        if (state.detailMode !== 'terminal') return;
        const machine = state.machines.find(item => item.id === state.terminalMachineId);
        if (state.terminalSession && canUseTerminal(state, machine)) return;
        disconnectTerminal(state);
        state.detailMode = 'overview';
        state.selectedShot = null;
    }

    function disconnectRemoteSessions(state, preserveWorkspaceControl) {
        disconnectVNC(state, preserveWorkspaceControl);
        disconnectTerminal(state);
    }
