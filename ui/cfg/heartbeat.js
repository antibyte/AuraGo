// cfg/heartbeat.js — Heartbeat wake-up scheduler section module

window.renderHeartbeatSection = function (section) {
    const data = configData['heartbeat'] || {};
    const enabledOn = data.enabled === true;
    const checkTasksOn = data.check_tasks === true;
    const checkAppointmentsOn = data.check_appointments === true;
    const checkEmailsOn = data.check_emails === true;
    const dayWindow = data.day_time_window || {};
    const nightWindow = data.night_time_window || {};

    let html = '<div class="cfg-section active hb-section">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    // Info banner
    html += '<div class="cfg-info-banner hb-info-banner">' + t('config.heartbeat.info_banner') + '</div>';

    // Enabled Toggle
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.heartbeat.enabled_label') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (enabledOn ? ' on' : '') + '" id="hbEnabledToggle" data-path="heartbeat.enabled" onclick="if(!this.disabled){toggleBool(this);toggleHeartbeatOptions(this.classList.contains(\'on\'));}"></div>';
    html += '<span class="toggle-label" id="hbEnabledLabel">' + (enabledOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    html += '<div id="heartbeatOptionsWrapper" class="hb-options-wrap' + (enabledOn ? '' : ' is-disabled') + '">';

    // Check toggles row
    html += '<div class="hb-toggles-row">';

    html += '<div class="field-group hb-toggle-card">';
    html += '<div class="field-label">' + t('config.heartbeat.check_tasks_label') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (checkTasksOn ? ' on' : '') + '" data-path="heartbeat.check_tasks" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (checkTasksOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    html += '<div class="field-group hb-toggle-card">';
    html += '<div class="field-label">' + t('config.heartbeat.check_appointments_label') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (checkAppointmentsOn ? ' on' : '') + '" data-path="heartbeat.check_appointments" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (checkAppointmentsOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    html += '<div class="field-group hb-toggle-card">';
    html += '<div class="field-label">' + t('config.heartbeat.check_emails_label') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (checkEmailsOn ? ' on' : '') + '" data-path="heartbeat.check_emails" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (checkEmailsOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    html += '</div>'; // end toggles row

    // Additional prompt
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.heartbeat.additional_prompt_label') + '</div>';
    html += '<div class="field-help">' + t('help.heartbeat.additional_prompt') + '</div>';
    html += '<textarea class="field-input" data-path="heartbeat.additional_prompt" rows="3" placeholder="' + escapeAttr(t('config.heartbeat.additional_prompt_placeholder')) + '">' + escapeHtml(data.additional_prompt || '') + '</textarea>';
    html += '</div>';

    // Time windows grid
    html += '<div class="hb-windows-grid">';

    // Day window card
    html += '<div class="field-group hb-window-card hb-day-card">';
    html += '<div class="hb-window-title">☀️ ' + t('config.heartbeat.day_window_title') + '</div>';

    html += '<div class="hb-window-fields">';
    html += '<div class="hb-field">';
    html += '<div class="field-label">' + t('config.heartbeat.window_start_label') + '</div>';
    html += '<input class="field-input" type="time" data-path="heartbeat.day_time_window.start" value="' + escapeAttr(dayWindow.start || '08:00') + '">';
    html += '</div>';

    html += '<div class="hb-field">';
    html += '<div class="field-label">' + t('config.heartbeat.window_end_label') + '</div>';
    html += '<input class="field-input" type="time" data-path="heartbeat.day_time_window.end" value="' + escapeAttr(dayWindow.end || '22:00') + '">';
    html += '</div>';

    html += '<div class="hb-field">';
    html += '<div class="field-label">' + t('config.heartbeat.window_interval_label') + '</div>';
    html += '<select class="field-select" data-path="heartbeat.day_time_window.interval">';
    html += renderIntervalOptions(dayWindow.interval || '1h');
    html += '</select>';
    html += '</div>';
    html += '</div>'; // end window fields

    html += '</div>'; // end day card

    // Night window card
    html += '<div class="field-group hb-window-card hb-night-card">';
    html += '<div class="hb-window-title">🌙 ' + t('config.heartbeat.night_window_title') + '</div>';

    html += '<div class="hb-window-fields">';
    html += '<div class="hb-field">';
    html += '<div class="field-label">' + t('config.heartbeat.window_start_label') + '</div>';
    html += '<input class="field-input" type="time" data-path="heartbeat.night_time_window.start" value="' + escapeAttr(nightWindow.start || '22:00') + '">';
    html += '</div>';

    html += '<div class="hb-field">';
    html += '<div class="field-label">' + t('config.heartbeat.window_end_label') + '</div>';
    html += '<input class="field-input" type="time" data-path="heartbeat.night_time_window.end" value="' + escapeAttr(nightWindow.end || '08:00') + '">';
    html += '</div>';

    html += '<div class="hb-field">';
    html += '<div class="field-label">' + t('config.heartbeat.window_interval_label') + '</div>';
    html += '<select class="field-select" data-path="heartbeat.night_time_window.interval">';
    html += renderIntervalOptions(nightWindow.interval || '4h');
    html += '</select>';
    html += '</div>';
    html += '</div>'; // end window fields

    html += '</div>'; // end night card

    html += '</div>'; // end windows grid

    html += '</div>'; // end options wrapper
    html += '</div>'; // end section

    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
};

window.toggleHeartbeatOptions = function (enabled) {
    const wrap = document.getElementById('heartbeatOptionsWrapper');
    if (wrap) {
        wrap.classList.toggle('is-disabled', !enabled);
    }
    const label = document.getElementById('hbEnabledLabel');
    if (label) {
        label.textContent = enabled ? t('config.toggle.active') : t('config.toggle.inactive');
    }
};

function renderIntervalOptions(selected) {
    const intervals = [
        { value: '15m', label: t('config.heartbeat.interval_15m') },
        { value: '30m', label: t('config.heartbeat.interval_30m') },
        { value: '1h', label: t('config.heartbeat.interval_1h') },
        { value: '2h', label: t('config.heartbeat.interval_2h') },
        { value: '4h', label: t('config.heartbeat.interval_4h') },
        { value: '6h', label: t('config.heartbeat.interval_6h') },
        { value: '12h', label: t('config.heartbeat.interval_12h') }
    ];
    let opts = '';
    for (const iv of intervals) {
        opts += '<option value="' + escapeAttr(iv.value) + '"' + (selected === iv.value ? ' selected' : '') + '>' + escapeHtml(iv.label) + '</option>';
    }
    return opts;
}
