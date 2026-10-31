// cfg/uptime_kuma.js — Uptime Kuma integration section module

function renderUptimeKumaSection(section) {
    const data = configData['uptime_kuma'] || {};
    const enabledOn = data.enabled === true;
    const apiKeyPlaceholder = cfgSecretPlaceholder(data.api_key, t('config.uptime_kuma.api_key_placeholder'));
    const form = window.AuraConfigForm;
    const checking = enabledOn && data.base_url;

    const html = form.renderSpec({
        label: section.label,
        desc: section.desc,
        banner: { id: 'uk-status-banner', state: checking ? 'checking' : (enabledOn ? 'unconfigured' : 'off'), detail: enabledOn && !data.base_url ? t('config.uptime_kuma.status_no_url') : '' },
        groups: [{ titleKey: 'config.refresh.operation', fields: [
            form.toggle({ label: t('config.uptime_kuma.enabled_label'), help: t('help.uptime_kuma.enabled'), path: 'uptime_kuma.enabled', value: enabledOn }),
            form.toggle({ label: t('config.uptime_kuma.relay_to_agent_label'), help: t('help.uptime_kuma.relay_to_agent'), path: 'uptime_kuma.relay_to_agent', value: data.relay_to_agent === true }),
            form.textarea({ label: t('config.uptime_kuma.relay_instruction_label'), help: t('help.uptime_kuma.relay_instruction'), path: 'uptime_kuma.relay_instruction', value: data.relay_instruction || '', rows: 5, placeholder: t('config.uptime_kuma.relay_instruction_placeholder') })
        ] }, { titleKey: 'config.refresh.connection', fields: [
            form.field({ label: t('config.uptime_kuma.base_url_label'), help: t('help.uptime_kuma.base_url'), path: 'uptime_kuma.base_url', value: data.base_url || '', placeholder: 'https://uptime-kuma.local:3001' }),
            form.toggle({ label: t('config.uptime_kuma.insecure_ssl_label'), help: t('help.uptime_kuma.insecure_ssl'), path: 'uptime_kuma.insecure_ssl', value: data.insecure_ssl === true }),
            form.number({ label: t('config.uptime_kuma.request_timeout_label'), help: t('help.uptime_kuma.request_timeout'), path: 'uptime_kuma.request_timeout', value: String(data.request_timeout || 15), min: 1, step: 1 }),
            form.number({ label: t('config.uptime_kuma.poll_interval_label'), help: t('help.uptime_kuma.poll_interval_seconds'), path: 'uptime_kuma.poll_interval_seconds', value: String(data.poll_interval_seconds || 30), min: 5, step: 1 })
        ] }, { titleKey: 'config.refresh.credentials', fields: [
            form.password({
                label: t('config.uptime_kuma.api_key_label'),
                help: t('help.uptime_kuma.api_key'),
                id: 'uptime-kuma-api-key',
                path: 'uptime_kuma.api_key',
                value: data.api_key,
                placeholder: apiKeyPlaceholder
            })
        ] }],
        afterHTML: form.actions([
            { html: '<button class="btn-save cfg-test-btn" onclick="uptimeKumaTestConnection()" id="uptime-kuma-test-btn" data-config-action-draft="true">' + t('config.uptime_kuma.test_btn') + '</button>' },
            { html: form.actionResult({ id: 'uptime-kuma-test-result' }) }
        ]) + '<div id="uptime-kuma-summary" class="cfg-stats is-hidden">'
            + '<div class="cfg-stats-title">' + t('config.uptime_kuma.summary_title') + '</div>'
            + '<div id="uptime-kuma-summary-content" class="cfg-stats-grid"></div>'
            + '</div>'
    });

    document.getElementById('content').innerHTML = html;

    if (checking) uptimeKumaCheckStatus();
}

function uptimeKumaCheckStatus() {
    const banner = document.getElementById('uk-status-banner');
    if (!banner) return;
    return cfgStatusBanner.check(banner, () => fetch('/api/uptime-kuma/status'), res => {
        if (!res) return null;
        if (res.status === 'disabled') return { state: 'off' };
        if (res.status === 'no_url') return { state: 'unconfigured', detail: t('config.uptime_kuma.status_no_url') };
        if (res.status === 'no_api_key') return { state: 'unconfigured', detail: t('config.uptime_kuma.status_no_api_key') };
        if (res.status !== 'ok' || !res.data) return { state: 'error', detail: res.message || t('config.uptime_kuma.connection_failed') };
        const summary = res.data.summary || {};
        const down = Number(summary.down || 0);
        uptimeKumaRenderSummary(summary);
        return down > 0
            ? { state: 'degraded', detail: t('config.uptime_kuma.connected') + ' - ' + down + ' ' + t('config.uptime_kuma.down_monitors') }
            : { state: 'connected', detail: t('config.uptime_kuma.connected') };
    });
}

function uptimeKumaRenderSummary(summary) {
    const wrap = document.getElementById('uptime-kuma-summary');
    const content = document.getElementById('uptime-kuma-summary-content');
    if (!wrap || !content) return;
    wrap.classList.remove('is-hidden');

    const downNames = Array.isArray(summary.down_monitor_names) ? summary.down_monitor_names : [];
    let html = '';
    html += '<div><strong>' + t('config.uptime_kuma.summary_up') + '</strong><br>' + escapeHtml(String(summary.up || 0)) + '</div>';
    html += '<div><strong>' + t('config.uptime_kuma.summary_down') + '</strong><br>' + escapeHtml(String(summary.down || 0)) + '</div>';
    html += '<div><strong>' + t('config.uptime_kuma.summary_unknown') + '</strong><br>' + escapeHtml(String(summary.unknown || 0)) + '</div>';
    html += '<div><strong>' + t('config.uptime_kuma.summary_total') + '</strong><br>' + escapeHtml(String(summary.total || 0)) + '</div>';
    if (downNames.length > 0) {
        html += '<div class="pw-u-grid-wide"><strong>' + t('config.uptime_kuma.down_monitors') + '</strong><br>' + escapeHtml(downNames.join(', ')) + '</div>';
    }
    content.innerHTML = html || '<div class="cfg-stats-loaded">' + t('config.uptime_kuma.summary_empty') + '</div>';
}

function uptimeKumaTestConnection() {
    const btn = document.getElementById('uptime-kuma-test-btn');
    return cfgActionResult(btn, () => fetch('/api/uptime-kuma/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ draft: cfgDraftPatch(['uptime_kuma']) })
    }), {
        target: 'uptime-kuma-test-result',
        success: t('config.uptime_kuma.test_ok'),
        failure: t('config.uptime_kuma.test_fail'),
        onSuccess: () => uptimeKumaCheckStatus()
    });
}
