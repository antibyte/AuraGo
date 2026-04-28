/**
 * Firewall Configuration Tab
 */

window.renderFirewallSection = function (sectionParam) {
    let html = '<div class="cfg-section active fw-section">';
    html += '<div class="section-header">' + sectionParam.icon + ' ' + sectionParam.label + '</div>';
    html += '<div class="section-desc">' + sectionParam.desc + '</div>';

    // Banner for non-Linux OS
    html += `<div id="firewallOsBanner" class="fw-os-banner is-hidden">
                ${t('config.firewall.os_banner')}
            </div>`;

    // Banner for Docker container (firewall not possible)
    const fwUnavail = featureUnavailableBanner('firewall', { blocked: true });
    if (fwUnavail) html += fwUnavail;

    const fwConfig = configData.firewall || {};
    const isOn = fwConfig.enabled === true;
    const fwBlocked = !!(runtimeData.features && runtimeData.features.firewall && !runtimeData.features.firewall.available);

    // Wrap fields in graying class when Docker-blocked
    if (fwBlocked) html += '<div class="feature-unavailable-fields">';

    // Enabled Toggle
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.firewall.enable_label') + '</div>';
    html += '<div class="field-help fw-enable-help">' + t('config.firewall.enable_help') + '</div>';
    html += '<div class="toggle-wrap" id="fwToggleWrap">';
    html += '<div class="toggle' + (isOn ? ' on' : '') + '" id="fwEnabledToggle" data-path="firewall.enabled" onclick="if(!this.disabled) { toggleBool(this); toggleFwOptions(this.classList.contains(\'on\')); }"></div>';
    html += '<span class="toggle-label" id="fwEnabledLabel">' + (isOn ? t('config.firewall.active') : t('config.firewall.inactive')) + '</span>';
    html += '</div>';
    html += '</div>';

    html += '<div id="firewallOptionsWrapper" class="fw-options-wrap' + (isOn ? '' : ' is-disabled') + '">';

    // Firewall Mode
    html += renderField('firewall.mode', 'mode', fwConfig.mode || 'readonly', 'firewall', {
        type: 'string',
        sensitive: false
    });

    html += '<div class="field-help fw-field-help-in-card">' + t('help.config.firewall.mode') + '</div>';

    // Poll Interval
    html += renderField('firewall.poll_interval_seconds', 'poll_interval_seconds', fwConfig.poll_interval_seconds || 60, 'firewall', {
        type: 'int',
        sensitive: false
    });

    html += '<div class="field-help fw-field-help-in-card">' + t('help.config.firewall.poll_interval_seconds') + '</div>';

    html += '</div>'; // End wrapper
    if (fwBlocked) html += '</div>'; // End feature-unavailable-fields
    html += '</div>';

    document.getElementById('content').innerHTML = html;

    // Fetch OS and enforce lockdown on inputs if not Linux
    fetch('/api/system/os')
        .then(res => res.json())
        .then(system => {
            if (system.os !== 'linux') {
                const toggle = document.getElementById('fwEnabledToggle');
                const options = document.getElementById('firewallOptionsWrapper');
                const banner = document.getElementById('firewallOsBanner');

                if (toggle) {
                    toggle.disabled = true;
                    toggle.classList.remove('on');
                    const label = document.getElementById('fwEnabledLabel');
                    if (label) label.textContent = t('config.firewall.inactive');
                }
                if (options) {
                    options.classList.add('is-disabled', 'is-locked');
                    // also disable inner inputs
                    options.querySelectorAll('.field-input, .field-select').forEach(el => {
                        el.disabled = true;
                    });
                }
                if (banner) {
                    banner.classList.remove('is-hidden');
                }
            } else {
                // Attach option syncing function for the readonly/guard select to update the layout if needed.
                // Re-wire standard inputs for dirty checking
                attachChangeListeners();
            }
        });
};

window.toggleFwOptions = function (enabled) {
    const wrap = document.getElementById('firewallOptionsWrapper');
    if (wrap) {
        wrap.classList.toggle('is-disabled', !enabled);
    }
};
