// cfg/daemon_skills.js — Daemon Skills configuration section module

function renderDaemonSkillsSection(section) {
    var data = (configData.tools?.daemon_skills) || {};
    var form = window.AuraConfigForm;
    var prefix = 'tools.daemon_skills.';
    var statusPanel = '<div class="field-group">'
        + '<div class="field-label" id="daemon-status-title">' + t('config.daemon_skills.status_title', { count: '…' }) + '</div>'
        + '<div id="daemon-status-grid" class="daemon-status-grid"></div>'
        + '</div>';

    document.getElementById('content').innerHTML = form.renderSpec({
        key: 'tools.daemon_skills',
        label: section.label,
        desc: section.desc,
        fields: [
            form.toggle({ label: t('config.daemon_skills.enabled_label'), help: t('help.daemon_skills.enabled'), path: prefix + 'enabled', value: data.enabled === true }),
            form.number({ label: t('config.daemon_skills.max_concurrent_daemons_label'), help: t('help.daemon_skills.max_concurrent_daemons'), path: prefix + 'max_concurrent_daemons', value: data.max_concurrent_daemons || 5, min: 1, max: 20, step: 1 }),
            form.number({ label: t('config.daemon_skills.global_rate_limit_secs_label'), help: t('help.daemon_skills.global_rate_limit_secs'), path: prefix + 'global_rate_limit_secs', value: data.global_rate_limit_secs || 60, min: 10, max: 3600, step: 1 }),
            form.number({ label: t('config.daemon_skills.max_wakeups_per_hour_label'), help: t('help.daemon_skills.max_wakeups_per_hour'), path: prefix + 'max_wakeups_per_hour', value: data.max_wakeups_per_hour || 6, min: 1, max: 60, step: 1 }),
            form.number({ label: t('config.daemon_skills.max_budget_per_hour_label'), help: t('help.daemon_skills.max_budget_per_hour'), path: prefix + 'max_budget_per_hour', value: data.max_budget_per_hour || 0.50, min: 0, max: 100, step: 0.01 }),
            statusPanel
        ]
    });
    attachChangeListeners();
    loadDaemonStatus();
}

async function loadDaemonStatus() {
    var grid = document.getElementById('daemon-status-grid');
    var title = document.getElementById('daemon-status-title');
    if (!grid) return;
    try {
        var resp = await fetch('/api/daemons');
        var data = await resp.json();
        if (!resp.ok || data.status !== 'ok' || (data.daemons !== null && !Array.isArray(data.daemons))) {
            throw new Error('Invalid daemon status response');
        }
        if (!data.daemons || data.daemons.length === 0) {
            cfgListState(grid, { state: 'empty', title: t('config.daemon_skills.no_daemons') });
            if (title) title.textContent = t('config.daemon_skills.status_title', { count: 0 });
            return;
        }
        var runningCount = data.daemons.filter(function(d) { return d.status === 'running'; }).length;
        if (title) title.textContent = t('config.daemon_skills.status_title', { count: runningCount });
        // Daemon state to shared badge state: starting is in progress, crashed is an error.
        var badgeState = { running: 'ok', starting: 'running', crashed: 'error', stopped: 'idle', disabled: 'idle' };
        var html = '';
        data.daemons.forEach(function(d) {
            var status = d.status || 'stopped';
            var statusLabel = tOr('config.daemon_skills.status_' + status, status);
            html += '<div class="daemon-status-row">';
            html += '<span class="daemon-status-name">' + esc(d.name || d.id) + '</span>';
            html += cfgBadge(badgeState[status] || 'idle', statusLabel);
            if (d.uptime) html += '<span class="daemon-status-uptime">' + esc(d.uptime) + '</span>';
            if (d.error) html += '<span class="daemon-status-error" title="' + esc(d.error) + '">' + cfgIcon('alert', { label: d.error }) + '</span>';
            html += '</div>';
        });
        grid.innerHTML = html;
    } catch (e) {
        if (title) title.textContent = t('config.daemon_skills.status_title', { count: '—' });
        cfgListState(grid, { state: 'error', title: t('config.daemon_skills.load_error'), retry: loadDaemonStatus });
    }
}
