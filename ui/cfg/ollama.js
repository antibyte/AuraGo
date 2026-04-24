// cfg/ollama.js — Ollama config section with managed container control
/* global configData, t, escapeAttr, toggleBool, renderFields, schema, helpTexts, lang */
'use strict';

async function renderOllamaSection(section) {
    const data = configData['ollama'] || {};
    const miData = data.managed_instance || {};
    const managedEnabled = miData.enabled === true;

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    // ── Basic Ollama settings ──
    html += _ollamaField('ollama.enabled', t('config.ollama.enabled_label'), t('config.ollama.enabled_help'),
        _ollamaToggle('ollama.enabled', data.enabled === true));

    html += _ollamaField('ollama.readonly', t('config.ollama.readonly_label'), t('config.ollama.readonly_help'),
        _ollamaToggle('ollama.readonly', data.readonly === true));

    html += _ollamaField('ollama.url', t('config.ollama.url_label'), t('config.ollama.url_help'),
        '<input class="field-input" type="text" data-path="ollama.url" value="' + escapeAttr(data.url || '') + '" placeholder="http://localhost:11434">');

    // ── Managed Instance section header ──
    html += '<div style="font-weight:600;font-size:0.92rem;color:var(--accent);border-bottom:1px solid var(--border-subtle);padding-bottom:0.4rem;margin:1.5rem 0 0.8rem;">🐳 ' + t('config.ollama.managed_title') + '</div>';

    // Status banner (only shown when managed is enabled)
    html += '<div id="ollama-managed-banner" style="margin-bottom:1rem;padding:0.8rem 1rem;border-radius:10px;font-size:0.84rem;background:var(--bg-tertiary);color:var(--text-secondary);">' + t('config.ollama.managed_checking') + '</div>';

    html += _ollamaField('ollama.managed_instance.enabled', t('config.ollama.managed_enabled_label'), t('config.ollama.managed_enabled_help'),
        _ollamaToggle('ollama.managed_instance.enabled', managedEnabled, 'ollamaManagedToggled(this)'));

    html += _ollamaField('ollama.managed_instance.container_port', t('config.ollama.managed_port_label'), t('config.ollama.managed_port_help'),
        '<input class="field-input" type="number" data-path="ollama.managed_instance.container_port" value="' + (miData.container_port || 11434) + '" min="1" max="65535">');

    html += _ollamaField('ollama.managed_instance.use_host_gpu', t('config.ollama.managed_gpu_label'), t('config.ollama.managed_gpu_help'),
        _ollamaToggle('ollama.managed_instance.use_host_gpu', miData.use_host_gpu === true));

    // GPU backend dropdown
    const gpuVal = miData.gpu_backend || 'auto';
    const gpuOptions = ['auto', 'nvidia', 'amd', 'intel', 'vulkan'];
    let gpuSelect = '<select class="field-select" data-path="ollama.managed_instance.gpu_backend">';
    for (const opt of gpuOptions) {
        gpuSelect += '<option value="' + opt + '"' + (gpuVal === opt ? ' selected' : '') + '>' + opt + '</option>';
    }
    gpuSelect += '</select>';
    html += _ollamaField('ollama.managed_instance.gpu_backend', t('config.ollama.managed_gpu_backend_label'), t('config.ollama.managed_gpu_backend_help'), gpuSelect);

    html += _ollamaField('ollama.managed_instance.default_models', t('config.ollama.managed_models_label'), t('config.ollama.managed_models_help'),
        '<input class="field-input" type="text" data-path="ollama.managed_instance.default_models" value="' + escapeAttr((miData.default_models || []).join(',')) + '" placeholder="llama3,mistral">');

    html += _ollamaField('ollama.managed_instance.memory_limit', t('config.ollama.managed_memory_label'), t('config.ollama.managed_memory_help'),
        '<input class="field-input" type="text" data-path="ollama.managed_instance.memory_limit" value="' + escapeAttr(miData.memory_limit || '') + '" placeholder="8g">');

    html += _ollamaField('ollama.managed_instance.volume_path', t('config.ollama.managed_volume_label'), t('config.ollama.managed_volume_help'),
        '<input class="field-input" type="text" data-path="ollama.managed_instance.volume_path" value="' + escapeAttr(miData.volume_path || '') + '" placeholder="">');

    // ── Recreate button (shown when managed is enabled) ──
    html += '<div id="ollama-recreate-wrap" style="' + (managedEnabled ? '' : 'display:none;') + 'margin-top:1rem;">';
    html += '<button class="btn-save" style="padding:0.5rem 1.2rem;font-size:0.85rem;" onclick="ollamaManagedRecreate()">';
    html += '🔄 ' + t('config.ollama.managed_recreate_btn') + '</button>';
    html += '<span id="ollama-recreate-status" style="margin-left:0.8rem;font-size:0.82rem;color:var(--text-secondary);"></span>';
    html += '</div>';

    html += '</div>';

    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
    if (typeof applyManagedDockerGuards === 'function') {
        applyManagedDockerGuards('ollama');
    }

    // Auto-check status
    ollamaManagedCheckStatus(managedEnabled);
}

function _ollamaField(path, label, help, inputHtml) {
    let h = '<div class="field-group">';
    h += '<div class="field-label">' + label + '</div>';
    if (help) h += '<div class="field-help">' + help + '</div>';
    h += inputHtml;
    h += '</div>';
    return h;
}

function _ollamaToggle(path, isOn, extra) {
    const onchange = extra ? ' onclick="toggleBool(this);' + extra + '"' : ' onclick="toggleBool(this)"';
    return '<div class="toggle-wrap">'
        + '<div class="toggle' + (isOn ? ' on' : '') + '" data-path="' + path + '"' + onchange + '></div>'
        + '<span class="toggle-label">' + (isOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>'
        + '</div>';
}

// eslint-disable-next-line no-unused-vars
function ollamaManagedToggled(el) {
    const isOn = el.classList.contains('on');
    const wrap = document.getElementById('ollama-recreate-wrap');
    if (wrap) wrap.style.display = isOn ? '' : 'none';
    ollamaManagedCheckStatus(isOn);
}

function ollamaManagedCheckStatus(enabled) {
    const banner = document.getElementById('ollama-managed-banner');
    if (!banner) return;

    if (!enabled) {
        banner.textContent = '⚪ ' + t('config.ollama.managed_status_disabled');
        banner.style.background = 'var(--bg-tertiary)';
        banner.style.color = 'var(--text-secondary)';
        return;
    }

    banner.textContent = '⏳ ' + t('config.ollama.managed_checking');
    banner.style.background = 'var(--bg-tertiary)';
    banner.style.color = 'var(--text-secondary)';

    fetch('/api/ollama/managed/status')
        .then(r => r.json())
        .then(res => {
            if (res.status === 'disabled') {
                banner.textContent = '⚪ ' + t('config.ollama.managed_status_disabled');
                banner.style.background = 'var(--bg-tertiary)';
                banner.style.color = 'var(--text-secondary)';
            } else if (res.running === true) {
                banner.textContent = '🟢 ' + t('config.ollama.managed_status_running');
                banner.style.background = 'rgba(72,199,142,0.1)';
                banner.style.color = '#48c78e';
            } else if (res.status === 'not_found') {
                banner.innerHTML = '🔴 ' + t('config.ollama.managed_status_not_found');
                banner.style.background = 'rgba(255,82,82,0.08)';
                banner.style.color = '#ff5252';
            } else if (res.running === false) {
                banner.textContent = '🟡 ' + t('config.ollama.managed_status_stopped');
                banner.style.background = 'rgba(255,183,77,0.1)';
                banner.style.color = '#ffb74d';
            } else {
                banner.textContent = '🔴 ' + t('config.ollama.managed_status_error');
                banner.style.background = 'rgba(255,82,82,0.08)';
                banner.style.color = '#ff5252';
            }
        })
        .catch(() => {
            banner.textContent = '🔴 ' + t('config.ollama.managed_status_error');
            banner.style.background = 'rgba(255,82,82,0.08)';
            banner.style.color = '#ff5252';
        });
}

// eslint-disable-next-line no-unused-vars
async function ollamaManagedRecreate() {
    const statusEl = document.getElementById('ollama-recreate-status');
    if (statusEl) statusEl.textContent = '⏳ ' + t('config.ollama.managed_recreating');

    try {
        const resp = await fetch('/api/ollama/managed/recreate', { method: 'POST' });
        const data = await resp.json();
        if (data.status === 'ok') {
            if (statusEl) statusEl.textContent = '✅ ' + t('config.ollama.managed_recreate_started');
            // Recheck status after a short delay to reflect new container state
            setTimeout(() => {
                ollamaManagedCheckStatus(true);
                if (statusEl) statusEl.textContent = '';
            }, 5000);
        } else {
            if (statusEl) statusEl.textContent = '❌ ' + (data.message || t('common.error'));
        }
    } catch (e) {
        if (statusEl) statusEl.textContent = '❌ ' + t('common.error');
    }
}
