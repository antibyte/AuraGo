(function () {
    'use strict';

    function renderPane(state, ui) {
        const c = state.context;
        const esc = ui.esc;
        if (state.resourceLoading.workspaces && !state.workspaces.length) return `<section class="vc-section-page">${ui.skeletonRows()}</section>`;
        if (state.resourceErrors.workspaces && !state.workspaces.length) return `<section class="vc-section-page">${ui.resourceErrorPane(state, 'workspaces')}</section>`;
        if (!state.workspaces.length) return `<section class="vc-section-page"><div class="vc-empty-state"><span class="vc-empty-icon">${ui.icon(state, 'agent', 'A')}</span><strong>${esc(ui.tx(c, 'desktop.virtual_computers_agent_workspaces_empty'))}</strong><p>${esc(ui.tx(c, 'desktop.virtual_computers_agent_workspaces_hint'))}</p></div></section>`;
        const summaries = new Map(state.workspaceSummaries.map(item => [item.workspace && item.workspace.id, item]));
        return `<section class="vc-section-page"><header class="vc-section-header"><div><span class="vc-eyebrow">${esc(ui.tx(c, 'desktop.virtual_computers_title'))}</span><h3>${esc(ui.tx(c, 'desktop.virtual_computers_agent_workspaces'))}</h3></div></header>
            ${state.resourceErrors.workspaces ? `<div class="vc-inline-error">${esc(state.resourceErrors.workspaces)}</div>` : ''}
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
        const showLiveVNC = !!(machine && machine.display && state.vncWorkspaceId === workspace.id);
        const outputAndActivity = `<div class="vc-workspace-split"><section><h4>${esc(ui.tx(c, 'desktop.virtual_computers_job_output'))}</h4><pre>${esc(jobOutput || (activeJob ? (activeJob.command_summary || activeJob.state) : ui.tx(c, 'desktop.virtual_computers_no_active_job')))}</pre></section><section><h4>${esc(ui.tx(c, 'desktop.virtual_computers_activity'))}</h4><ol>${events.map(event => `<li><span>${esc(event.type || 'event')}</span><small>${esc(ui.formatDate(event.created_at))}</small><p>${esc(event.summary || '')}</p></li>`).join('') || '<li>—</li>'}</ol></section></div>`;
        const runtimeView = showLiveVNC ? `<div class="vc-workspace-live"><section class="vc-workspace-vnc-panel"><h4>${esc(ui.tx(c, 'desktop.virtual_computers_vnc_live'))}</h4><div class="vc-vnc-mount" data-role="workspace-vnc-mount" data-workspace-id="${esc(workspace.id)}"></div></section>${outputAndActivity}</div>` : outputAndActivity;
        return `<article class="vc-workspace-card ${showLiveVNC ? 'is-live' : ''}">
            <header class="vc-detail-header"><div><span class="vc-eyebrow">${esc(workspace.template || 'desktop')} · ${esc(workspace.network_profile || 'internet_lan')}</span><h3>${esc(workspace.id)}</h3><span class="vc-state-chip" data-state="${esc(workspace.state || '')}"><span class="vc-state-dot"></span>${esc(workspace.state || '—')}</span></div>
            <div class="vc-actions">${machine && machine.display ? `<button type="button" class="vc-btn" data-action="observe-workspace" data-id="${esc(workspace.id)}" data-machine-id="${esc(workspace.machine_id)}">${esc(ui.tx(c, 'desktop.virtual_computers_observe'))}</button><button type="button" class="vc-btn" data-action="takeover-workspace" data-id="${esc(workspace.id)}" data-machine-id="${esc(workspace.machine_id)}">${esc(ui.tx(c, 'desktop.virtual_computers_take_control'))}</button>` : ''}<button type="button" class="vc-btn" data-action="checkpoint-workspace" data-id="${esc(workspace.id)}">${esc(ui.tx(c, 'desktop.virtual_computers_checkpoint'))}</button><button type="button" class="vc-btn danger" data-action="close-workspace" data-id="${esc(workspace.id)}">${esc(ui.tx(c, 'desktop.virtual_computers_close_workspace'))}</button></div></header>
            <dl class="vc-meta-grid"><div><dt>${esc(ui.tx(c, 'desktop.virtual_computers_machine'))}</dt><dd>${esc(workspace.machine_id || '—')}</dd></div><div><dt>TTL</dt><dd>${esc(ui.formatExpiryCountdown(workspace.lease_expires_at))}</dd></div><div><dt>${esc(ui.tx(c, 'desktop.virtual_computers_control'))}</dt><dd>${esc(workspace.control_owner || 'agent')}</dd></div><div><dt>${esc(ui.tx(c, 'desktop.virtual_computers_volume'))}</dt><dd>${esc(workspace.volume_id || 'ephemeral')}</dd></div><div class="vc-meta-wide"><dt>${esc(ui.tx(c, 'desktop.virtual_computers_browser_url'))}</dt><dd>${esc(activeBrowser && activeBrowser.url_origin ? activeBrowser.url_origin : '—')}</dd></div><div class="vc-meta-wide"><dt>${esc(ui.tx(c, 'desktop.virtual_computers_current_job'))}</dt><dd>${activeJob ? `${esc(activeJob.command_summary || activeJob.id)} <button type="button" class="vc-text-button" data-action="cancel-workspace-job" data-id="${esc(workspace.id)}" data-job-id="${esc(activeJob.id)}">${esc(ui.tx(c, 'desktop.virtual_computers_task_cancel'))}</button>` : '—'}</dd></div></dl>
            ${runtimeView}
            <footer class="vc-workspace-grants"><strong>${esc(ui.tx(c, 'desktop.virtual_computers_active_grants'))}: ${activeGrants.length}</strong>${activeGrants.map(grant => `<span class="vc-workspace-grant"><span>${esc(grant.credential_id)} · ${esc(grant.usage_type)} · ${esc(grant.origin || grant.job_id || '')} · ${esc(grant.status)}</span>${grant.status === 'pending_approval' ? `<button type="button" class="vc-text-button" data-action="approve-workspace-grant" data-id="${esc(workspace.id)}" data-grant-id="${esc(grant.id)}">${esc(ui.tx(c, 'desktop.virtual_computers_approve_grant'))}</button>` : ''}<button type="button" class="vc-text-button" data-action="revoke-workspace-grant" data-id="${esc(workspace.id)}" data-grant-id="${esc(grant.id)}">${esc(ui.tx(c, 'desktop.virtual_computers_revoke_grant'))}</button></span>`).join('')}</footer>
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

    window.VirtualComputersWorkspaces = { renderPane, controlAndOpen, releaseControl };
}());
