(function () {
    'use strict';

    const K = 'desktop.virtual_computers_';
    const maxRequestLength = 4000;
    // Every kind maps to a family of dedicated language keys (K + prefix + value). An
    // unknown value falls back to a readable form of the raw value instead of an unrelated label.
    const labelPrefixes = {
        native: 'native_', control: 'control_', template: 'template_', network: 'network_',
        ws_state: 'ws_state_', job_state: 'job_state_', grant_state: 'grant_state_',
        usage: 'usage_', event: 'event_'
    };
    const nativeAliases = { repair_required: 'repair', session_unreachable: 'unreachable' };

    function humanize(value) {
        const text = String(value == null ? '' : value).replace(/[_-]+/g, ' ').trim();
        return text ? text.charAt(0).toUpperCase() + text.slice(1) : '';
    }

    function valueLabel(context, kind, value, ui) {
        const normalized = String(value || '').trim();
        if (!normalized) return '';
        let slug = normalized.toLowerCase().replace(/[^a-z0-9_]+/g, '_');
        if (kind === 'native' && nativeAliases[slug]) slug = nativeAliases[slug];
        const prefix = labelPrefixes[kind];
        if (!prefix) return humanize(normalized);
        const key = K + prefix + slug;
        const text = ui.tx(context, key);
        return text && text !== key ? text : humanize(normalized);
    }

    function shortId(id) {
        const text = String(id == null ? '' : id);
        return text.length > 14 ? text.slice(0, 12) + '…' : text;
    }

    function machineName(machine) {
        return (machine && (machine.label || machine.name || machine.id)) || '';
    }

    function requestCountText(context, length) {
        const count = Math.max(0, Number(length) || 0);
        const text = context && typeof context.t === 'function' ? context.t(K + 'request_count', { count, max: maxRequestLength }) : '';
        return text && text !== K + 'request_count' ? text : count + ' / ' + maxRequestLength;
    }

    function readyWorkspaces(state) {
        return state.workspaces.filter(w => w.state === 'ready' && w.owner_session_id === 'virtual-desktop' && !w.mission_id);
    }

    function workspaceName(state, workspace) {
        const machine = state.machines.find(item => item.id === workspace.machine_id);
        return machine ? machineName(machine) : shortId(workspace.machine_id || workspace.id);
    }

    function agentRequest(state, ui) {
        const c = state.context;
        const esc = ui.esc;
        const selected = state.agentWorkspaceId || '';
        const available = readyWorkspaces(state);
        const running = state.machines.filter(machine => machine && machine.id && !machine.workspace_id && String(machine.status || '').toLowerCase() === 'running');
        const workspaceOptions = available.map(w => `<option value="${esc(w.id)}" ${w.id === selected ? 'selected' : ''}>${esc(workspaceName(state, w))} · ${esc(valueLabel(c, 'template', w.template, ui))}</option>`).join('');
        const machineOptions = running.map(machine => `<option value="machine:${esc(machine.id)}" ${'machine:' + machine.id === selected ? 'selected' : ''}>${esc(machineName(machine))} · ${esc(valueLabel(c, 'template', machine.template, ui))}</option>`).join('');
        const missing = selected && !available.some(w => w.id === selected) && !running.some(machine => 'machine:' + machine.id === selected);
        const missingLabel = selected.startsWith('machine:') ? shortId(selected.slice('machine:'.length)) : shortId(selected);
        const groups = `${workspaceOptions ? `<optgroup label="${esc(ui.tx(c, K + 'agent_workspaces'))}">${workspaceOptions}</optgroup>` : ''}${machineOptions ? `<optgroup label="${esc(ui.tx(c, K + 'picker_running_computers'))}">${machineOptions}</optgroup>` : ''}`;
        const selector = `<label class="vc-agent-workspace"><span>${esc(ui.tx(c, K + 'agent_workspaces'))}</span><select data-role="agent-workspace"><option value="">${esc(ui.tx(c, K + 'workspace_auto'))}</option>${groups}${missing ? `<option value="${esc(selected)}" selected disabled>${esc(missingLabel)} · ${esc(ui.tx(c, K + 'workspace_unavailable'))}</option>` : ''}</select></label>`;
        const draft = state.agentRequestDraft || '';
        return `<div class="vc-agent-request">${selector}<div class="vc-agent-field"><label><span>${esc(ui.tx(c, 'desktop.agent_task_for_agent'))}</span><textarea data-role="agent-request" rows="3" maxlength="${maxRequestLength}" placeholder="${esc(ui.tx(c, K + 'request_placeholder'))}">${esc(draft)}</textarea></label><small class="vc-agent-count" data-role="agent-request-count">${esc(requestCountText(c, draft.length))}</small></div><button type="button" class="vc-btn vc-primary" data-action="ask-agent">${ui.icon(state, 'agent', 'A')}${esc(ui.tx(c, K + 'task_start'))}</button></div>`;
    }

    function renderPane(state, ui) {
        const c = state.context;
        const esc = ui.esc;
        if (state.resourceLoading.workspaces && !state.workspaces.length) return `<section class="vc-section-page">${ui.skeletonRows()}</section>`;
        if (state.resourceErrors.workspaces && !state.workspaces.length) return `<section class="vc-section-page">${ui.resourceErrorPane(state, 'workspaces')}</section>`;
        if (!state.workspaces.length) return `<section class="vc-section-page"><div class="vc-empty-state"><span class="vc-empty-icon">${ui.icon(state, 'agent', 'A')}</span><strong>${esc(ui.tx(c, K + 'agent_workspaces_empty'))}</strong><p>${esc(ui.tx(c, K + 'agent_workspaces_hint'))}</p>${agentRequest(state, ui)}</div></section>`;
        const summaries = new Map(state.workspaceSummaries.map(item => [item.workspace && item.workspace.id, item]));
        return `<section class="vc-section-page"><header class="vc-section-header"><div><span class="vc-eyebrow">${esc(ui.tx(c, K + 'title'))}</span><h3>${esc(ui.tx(c, K + 'agent_workspaces'))}</h3></div></header>
            ${agentRequest(state, ui)}
            ${state.resourceErrors.workspaces ? `<div class="vc-inline-error" role="alert">${esc(state.resourceErrors.workspaces)}</div>` : ''}
            <div class="vc-workspace-grid">${state.workspaces.map(workspace => workspaceCard(state, workspace, summaries.get(workspace.id) || {}, ui)).join('')}</div></section>`;
    }

    function workspaceCard(state, workspace, summary, ui) {
        const c = state.context;
        const esc = ui.esc;
        const jobs = Array.isArray(summary.jobs) ? summary.jobs : [];
        const grants = Array.isArray(summary.credential_grants) ? summary.credential_grants : [];
        const browserSessions = Array.isArray(summary.browser_sessions) ? summary.browser_sessions : [];
        const jobOutput = summary.job_output && typeof summary.job_output.data === 'string' ? summary.job_output.data : '';
        const events = Array.isArray(summary.events) ? summary.events.slice(-8).reverse() : [];
        const activeJob = jobs.find(job => job.state === 'running' || job.state === 'queued');
        const activeGrants = grants.filter(grant => grant.status === 'active' || grant.status === 'pending_approval');
        const activeBrowser = browserSessions.find(session => session.state === 'open');
        const machine = state.machines.find(item => item.id === workspace.machine_id);
        const adopted = workspace.origin === 'adopted';
        // Closed, failed and lost workspaces keep their history but offer no actions.
        const isReady = workspace.state === 'ready';
        const isLive = isReady || workspace.state === 'opening';
        const name = workspaceName(state, workspace);
        const volume = workspace.volume_id ? (state.volumes || []).find(item => item.id === workspace.volume_id) : null;
        const volumeText = workspace.volume_id ? ((volume && volume.name) || shortId(workspace.volume_id)) : ui.tx(c, K + 'volume_none');
        const nativeEnabled = !!(state.status && state.status.capabilities && state.status.capabilities.native_apps);
        const nativeStatus = nativeEnabled ? ((state.nativeDesktopStatus || {})[workspace.id] || { state: 'unchecked' }) : { state: 'disabled' };
        const nativePanel = workspace.template === 'desktop' && isLive ? `<div class="vc-inline-note"><strong>${esc(ui.tx(c, K + 'native_title'))}</strong>: <span role="status">${esc(valueLabel(c, 'native', nativeStatus.state, ui))}${nativeStatus.version ? ' · ' + esc(nativeStatus.version) : ''}</span> <button type="button" class="vc-btn" data-action="check-native-desktop" data-id="${esc(workspace.id)}" ${nativeStatus.state === 'checking' ? 'disabled' : ''}>${esc(ui.tx(c, K + 'native_check'))}</button></div>` : '';
        const showLiveVNC = !!(machine && machine.display && state.vncWorkspaceId === workspace.id);
        const outputAndActivity = `<div class="vc-workspace-split"><section><h4>${esc(ui.tx(c, K + 'job_output'))}</h4><pre>${esc(jobOutput || (activeJob ? (activeJob.command_summary || valueLabel(c, 'job_state', activeJob.state, ui)) : ui.tx(c, K + 'no_active_job')))}</pre></section><section><h4>${esc(ui.tx(c, K + 'activity'))}</h4><ol>${events.map(event => `<li><span>${esc(valueLabel(c, 'event', event.type, ui) || ui.tx(c, K + 'activity'))}</span><small>${esc(ui.formatDate(event.created_at))}</small><p>${esc(event.type === 'desktop_action' ? ui.tx(c, K + 'native_title') : (event.summary || ''))}</p></li>`).join('') || '<li>—</li>'}</ol></section></div>`;
        const runtimeView = showLiveVNC ? `<div class="vc-workspace-live"><section class="vc-workspace-vnc-panel"><h4>${esc(ui.tx(c, K + 'vnc_live'))}</h4><div class="vc-vnc-mount" data-role="workspace-vnc-mount" data-workspace-id="${esc(workspace.id)}"></div></section>${outputAndActivity}</div>` : outputAndActivity;
        const observe = isReady && machine && machine.display ? `<button type="button" class="vc-btn" data-action="observe-workspace" data-id="${esc(workspace.id)}" data-machine-id="${esc(workspace.machine_id)}">${esc(ui.tx(c, K + 'observe'))}</button><button type="button" class="vc-btn" data-action="takeover-workspace" data-id="${esc(workspace.id)}" data-machine-id="${esc(workspace.machine_id)}">${esc(ui.tx(c, K + 'take_control'))}</button>` : '';
        const checkpoint = isReady ? `<button type="button" class="vc-btn" data-action="checkpoint-workspace" data-id="${esc(workspace.id)}">${esc(ui.tx(c, K + 'checkpoint'))}</button>` : '';
        const closeButton = !isLive ? '' : adopted
            ? `<button type="button" class="vc-btn" data-action="close-workspace" data-id="${esc(workspace.id)}" title="${esc(ui.tx(c, K + 'confirm_release_workspace_desc'))}">${esc(ui.tx(c, K + 'release_workspace'))}</button>`
            : `<button type="button" class="vc-btn danger" data-action="close-workspace" data-id="${esc(workspace.id)}">${esc(ui.tx(c, K + 'close_workspace'))}</button>`;
        const adoptedBadge = adopted ? `<span class="vc-badge is-assigned">${esc(ui.tx(c, K + 'badge_assigned_vm'))}</span>` : '';
        return `<article class="vc-workspace-card ${showLiveVNC ? 'is-live' : ''}" data-workspace-id="${esc(workspace.id)}" data-origin="${esc(workspace.origin || 'launched')}">
            <header class="vc-detail-header"><div><span class="vc-eyebrow">${esc(valueLabel(c, 'template', workspace.template || 'desktop', ui))} · ${esc(valueLabel(c, 'network', workspace.network_profile || 'internet_lan', ui))}</span><h3>${esc(name)}</h3><div class="vc-workspace-tags"><span class="vc-state-chip" data-state="${esc(workspace.state || '')}"><span class="vc-state-dot"></span>${esc(valueLabel(c, 'ws_state', workspace.state, ui) || '—')}</span>${adoptedBadge}</div></div>
            <div class="vc-actions">${observe}${checkpoint}${closeButton}</div></header>
            <dl class="vc-meta-grid"><div><dt>${esc(ui.tx(c, K + 'machine'))}</dt><dd title="${esc(workspace.machine_id || '')}">${esc(machine ? machineName(machine) : (workspace.machine_id ? shortId(workspace.machine_id) : '—'))}</dd></div><div><dt>${esc(ui.tx(c, K + 'expires'))}</dt><dd>${esc(ui.formatExpiryCountdown(workspace.lease_expires_at, undefined, c))}</dd></div><div><dt>${esc(ui.tx(c, K + 'control'))}</dt><dd>${esc(valueLabel(c, 'control', workspace.control_owner || 'agent', ui))}</dd></div><div><dt>${esc(ui.tx(c, K + 'volume'))}</dt><dd title="${esc(workspace.volume_id || '')}">${esc(volumeText)}</dd></div><div class="vc-meta-wide"><dt>${esc(ui.tx(c, K + 'browser_url'))}</dt><dd>${esc(activeBrowser && activeBrowser.url_origin ? activeBrowser.url_origin : '—')}</dd></div><div class="vc-meta-wide"><dt>${esc(ui.tx(c, K + 'current_job'))}</dt><dd>${activeJob ? `${esc(activeJob.command_summary || activeJob.id)} <button type="button" class="vc-text-button" data-action="cancel-workspace-job" data-id="${esc(workspace.id)}" data-job-id="${esc(activeJob.id)}">${esc(ui.tx(c, K + 'task_cancel'))}</button>` : '—'}</dd></div></dl>
            ${nativePanel}
            ${runtimeView}
            <footer class="vc-workspace-grants"><strong>${esc(ui.tx(c, K + 'active_grants'))}: ${activeGrants.length}</strong>${activeGrants.map(grant => `<span class="vc-workspace-grant"><span>${esc(grant.credential_id)} · ${esc(valueLabel(c, 'usage', grant.usage_type, ui))} · ${esc(grant.origin || grant.job_id || '')} · ${esc(valueLabel(c, 'grant_state', grant.status, ui))}</span>${grant.status === 'pending_approval' ? `<button type="button" class="vc-text-button" data-action="approve-workspace-grant" data-id="${esc(workspace.id)}" data-grant-id="${esc(grant.id)}">${esc(ui.tx(c, K + 'approve_grant'))}</button>` : ''}<button type="button" class="vc-text-button" data-action="revoke-workspace-grant" data-id="${esc(workspace.id)}" data-grant-id="${esc(grant.id)}">${esc(ui.tx(c, K + 'revoke_grant'))}</button></span>`).join('')}</footer>
        </article>`;
    }

    function openVNC(state, workspaceId, machineId, takeControl, api) {
        const machine = state.machines.find(item => item.id === machineId);
        if (!api.canUseVNC(state, machine) || !window.VirtualComputersVNC || typeof window.VirtualComputersVNC.mount !== 'function') {
            api.notify(state, api.tx(state.context, 'desktop.virtual_computers_vnc_unavailable'), 'error');
            return;
        }
        api.disconnectRemoteSessions(state, true);
        state.activeSection = 'workspaces';
        state.vncMachineId = machineId;
        state.vncWorkspaceId = workspaceId;
        state.vncWorkspaceTakeover = takeControl === true;
        state.workspaceControlRenewedAt = Date.now();
        api.draw(state);
        const mountPoint = state.host.querySelector('[data-role="workspace-vnc-mount"]');
        if (!mountPoint) return;
        const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const url = proto + '//' + window.location.host + '/api/virtual-computers/machines/' + encodeURIComponent(machine.id) + '/vnc';
        try {
            state.vncSession = window.VirtualComputersVNC.mount(mountPoint, {
                url, machineId: machine.id, viewOnly: takeControl !== true, lockViewOnly: takeControl !== true,
                t: key => api.tx(state.context, key),
                notify: (message, type) => api.notify(state, message, type),
                onInputActivity: () => renewControl(state, workspaceId, api),
                onExpandedChange: expanded => api.setVNCExpanded(state, expanded),
                onClose: () => closeWorkspaceVNC(state, workspaceId, api)
            });
        } catch (_) {
            api.disconnectVNC(state);
            api.notify(state, api.tx(state.context, 'desktop.virtual_computers_vnc_unavailable'), 'error');
            api.draw(state);
        }
    }

    async function controlAndOpen(state, workspaceId, machineId, takeControl, api) {
        if (!workspaceId || !machineId) return;
        if (state.vncWorkspaceTakeover && state.vncWorkspaceId && state.vncWorkspaceId !== workspaceId) {
            const previousWorkspaceId = state.vncWorkspaceId;
            api.disconnectVNC(state, true);
            if (!await releaseControl(state, previousWorkspaceId, api)) return;
        }
        const owner = takeControl ? 'human' : 'agent';
        const ok = await api.workspaceMutation(state, workspaceId, '/control', {
            method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ owner })
        });
        if (ok) openVNC(state, workspaceId, machineId, takeControl, api);
    }

    function closeWorkspaceVNC(state, workspaceId, api) {
        const release = state.vncWorkspaceTakeover;
        state.vncSession = null;
        state.vncMachineId = null;
        state.vncWorkspaceId = null;
        state.vncWorkspaceTakeover = false;
        api.setVNCExpanded(state, false);
        if (release) releaseControl(state, workspaceId, api);
        api.draw(state);
    }

    function renewControl(state, workspaceId, api) {
        if (!state.vncWorkspaceTakeover || state.vncWorkspaceId !== workspaceId) return;
        const now = Date.now();
        if (now - state.workspaceControlRenewedAt < 30000) return;
        state.workspaceControlRenewedAt = now;
        api.request('/api/virtual-computers/workspaces/' + encodeURIComponent(workspaceId) + '/control', {
            method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ owner: 'human' })
        }).catch(error => api.notify(state, api.tx(state.context, 'desktop.virtual_computers_error') + ' ' + error.message, 'error'));
    }

    async function releaseControl(state, workspaceId, api) {
        try {
            await api.request('/api/virtual-computers/workspaces/' + encodeURIComponent(workspaceId) + '/control', {
                method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ owner: 'agent' })
            });
            return true;
        } catch (error) {
            api.notify(state, api.tx(state.context, 'desktop.virtual_computers_error') + ' ' + error.message, 'error');
            return false;
        }
    }

    async function checkNativeDesktop(state, workspaceId, api) {
        if (!workspaceId || state.disposed) return;
        state.nativeDesktopStatus = state.nativeDesktopStatus || {};
        if (state.nativeDesktopStatus[workspaceId]?.state === 'checking') return;
        const marker = {};
        state.nativeDesktopPending = state.nativeDesktopPending || {};
        state.nativeDesktopPending[workspaceId] = marker;
        state.nativeDesktopStatus[workspaceId] = { state: 'checking' };
        api.draw(state);
        try {
            const result = await api.request('/api/virtual-computers/workspaces/' + encodeURIComponent(workspaceId) + '/desktop/status');
            if (state.disposed || state.nativeDesktopPending[workspaceId] !== marker) return;
            state.nativeDesktopStatus[workspaceId] = result.desktop || { state: 'session_unreachable' };
        } catch (_) {
            if (state.disposed || state.nativeDesktopPending[workspaceId] !== marker) return;
            state.nativeDesktopStatus[workspaceId] = { state: 'session_unreachable' };
        }
        api.draw(state);
    }

    window.VirtualComputersWorkspaces = { renderPane, controlAndOpen, releaseControl, checkNativeDesktop, requestCountText };
}());
