// cfg/daemon_skills.js — Daemon Skills configuration section module

function renderDaemonSkillsSection(section) {
    var data = (configData.tools?.daemon_skills) || {};
    var enabledOn = data.enabled === true;
    var maxConc = data.max_concurrent_daemons || 5;
    var globalRate = data.global_rate_limit_secs || 60;
    var maxWake = data.max_wakeups_per_hour || 6;
    var maxBudget = data.max_budget_per_hour || 0.50;

    var html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    // ── Enabled toggle ──
    var helpEnabled = t('help.daemon_skills.enabled');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.daemon_skills.enabled_label') + '</div>';
    if (helpEnabled) html += '<div class="field-help">' + helpEnabled + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (enabledOn ? ' on' : '') + '" data-path="tools.daemon_skills.enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (enabledOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Max Concurrent Daemons ──
    var helpMax = t('help.daemon_skills.max_concurrent_daemons');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.daemon_skills.max_concurrent_daemons_label') + '</div>';
    if (helpMax) html += '<div class="field-help">' + helpMax + '</div>';
    html += '<input type="number" class="field-input" data-path="tools.daemon_skills.max_concurrent_daemons" value="' + maxConc + '" min="1" max="20" step="1">';
    html += '</div>';

    // ── Global Rate Limit ──
    var helpRate = t('help.daemon_skills.global_rate_limit_secs');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.daemon_skills.global_rate_limit_secs_label') + '</div>';
    if (helpRate) html += '<div class="field-help">' + helpRate + '</div>';
    html += '<input type="number" class="field-input" data-path="tools.daemon_skills.global_rate_limit_secs" value="' + globalRate + '" min="10" max="3600" step="1">';
    html += '</div>';

    // ── Max Wake-ups per Hour ──
    var helpWake = t('help.daemon_skills.max_wakeups_per_hour');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.daemon_skills.max_wakeups_per_hour_label') + '</div>';
    if (helpWake) html += '<div class="field-help">' + helpWake + '</div>';
    html += '<input type="number" class="field-input" data-path="tools.daemon_skills.max_wakeups_per_hour" value="' + maxWake + '" min="1" max="60" step="1">';
    html += '</div>';

    // ── Max Budget per Hour ──
    var helpBudget = t('help.daemon_skills.max_budget_per_hour');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.daemon_skills.max_budget_per_hour_label') + '</div>';
    if (helpBudget) html += '<div class="field-help">' + helpBudget + '</div>';
    html += '<input type="number" class="field-input" data-path="tools.daemon_skills.max_budget_per_hour" value="' + maxBudget + '" min="0" max="100" step="0.01">';
    html += '</div>';

    // ── Live Daemon Status ──
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.daemon_skills.status_title') + '</div>';
    html += '<div id="daemon-status-grid" class="daemon-status-grid"></div>';
    html += '</div>';

    html += '</div>';
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
    loadDaemonStatus();
}

async function loadDaemonStatus() {
    var grid = document.getElementById('daemon-status-grid');
    if (!grid) return;
    try {
        var resp = await fetch('/api/daemons');
        var data = await resp.json();
        if (data.status !== 'ok' || !data.daemons || data.daemons.length === 0) {
            grid.innerHTML = '<div class="empty-state">' + (t('config.daemon_skills.no_daemons') || 'No daemon skills configured') + '</div>';
            return;
        }
        var html = '';
        data.daemons.forEach(function(d) {
            var statusClass = 'daemon-status-' + (d.status || 'stopped');
            var statusLabel = t('config.daemon_skills.status_' + (d.status || 'stopped')) || d.status || 'stopped';
            html += '<div class="daemon-status-row">';
            html += '<span class="daemon-status-name">' + esc(d.name || d.id) + '</span>';
            html += '<span class="daemon-status-badge ' + statusClass + '">' + statusLabel + '</span>';
            if (d.uptime) html += '<span class="daemon-status-uptime">' + esc(d.uptime) + '</span>';
            if (d.error) html += '<span class="daemon-status-error" title="' + esc(d.error) + '">⚠️</span>';
            html += '</div>';
        });
        grid.innerHTML = html;
    } catch (e) {
        grid.innerHTML = '<div class="empty-state">' + (t('config.daemon_skills.load_error') || 'Failed to load daemon status') + '</div>';
    }
}
