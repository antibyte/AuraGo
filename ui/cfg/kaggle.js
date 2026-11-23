// Kaggle credentials stay in the Vault; the ordinary form contains policy only.
// The machine shapes a Kaggle run may request; equal to config.KaggleMachineShapes (NormalizeKaggleConfig keeps only these).
const KAGGLE_MACHINE_SHAPES = ['NvidiaTeslaT4', 'NvidiaL4'];
async function renderKaggleSection(section) {
    const label = key => t('config.kaggle.' + key);
    const cfg = configData.kaggle || (configData.kaggle = {});
    const local = cfg.local || (cfg.local = {});
    let html = '<div class="cfg-section active"><div class="section-header">' + escapeAttr(section.label) + '</div><div class="section-desc">' + escapeAttr(section.desc) + '</div>';
    html += cfgStatusBanner.html('kaggle-status', 'checking');
    html += '<p class="field-help">' + label('policy_help') + '</p>';
    const help = key => '<span class="field-help">' + label(key + '_help') + '</span>';
    const toggle = (path, value, key, helpKey) => '<div class="field-group"><div class="field-label">' + label(key) + '</div>' + help(helpKey || key) + '<div class="toggle-wrap"><button type="button" role="switch" aria-checked="' + !!value + '" aria-label="' + escapeAttr(label(key)) + '" class="toggle' + (value ? ' on' : '') + '" data-path="kaggle.' + path + '" onclick="toggleBool(this);this.setAttribute(\'aria-checked\',this.classList.contains(\'on\'))"></button><span class="toggle-label">' + t(value ? 'config.toggle.active' : 'config.toggle.inactive') + '</span></div></div>';
    const number = (path, value, key, fallback, max) => '<label class="field-group"><span class="field-label">' + label(key) + '</span>' + help(key) + '<input class="field-input" type="number" min="1" max="' + (window.AuraConfigCatalog?.validationRules['kaggle.' + path]?.max ?? max) + '" data-path="kaggle.' + path + '" value="' + Number(value || fallback) + '"></label>';
    html += '<div class="field-grid two-cols">' + toggle('enabled', cfg.enabled, 'enabled') + toggle('read_only', cfg.read_only !== false, 'read_only') + '</div>';
    html += '<div class="field-group-title">' + label('credentials') + '</div><p class="field-help">' + label('credentials_help') + '</p>';
    html += '<label class="field-group"><span class="field-label">' + label('auth_mode') + '</span>' + help('auth_mode') + '<select class="field-select" data-path="kaggle.auth_mode"><option value="token"' + (cfg.auth_mode !== 'legacy' ? ' selected' : '') + '>' + label('api_token') + '</option><option value="legacy"' + (cfg.auth_mode === 'legacy' ? ' selected' : '') + '>' + label('legacy') + '</option></select></label>';
    html += '<label class="field-group"><span class="field-label">' + label('owner') + '</span>' + help('owner') + '<input class="field-input" data-path="kaggle.username" value="' + escapeAttr(cfg.username || '') + '" autocomplete="off"></label>';
    for (const key of ['token', 'legacy_key']) html += '<label class="field-group"><span class="field-label">' + label(key === 'token' ? 'api_token' : 'legacy_key') + '</span>' + help(key === 'token' ? 'api_token' : 'legacy_key') + '<input class="field-input" type="password" id="kaggle-' + key + '" autocomplete="new-password" value="" placeholder="' + escapeAttr(label('keep_secret')) + '"><button type="button" class="btn-save pw-u-mt-8" id="kaggle-remove-' + key + '">' + label('verb_delete') + '</button></label>';
    html += '<div class="cfg-actions-row"><button class="btn-save" id="kaggle-secret-save">' + label('save_credentials') + '</button><button class="btn-save" id="kaggle-test">' + label('test') + '</button>' + cfgActionResult.html('kaggle-result') + '</div>';
    html += '<div class="field-group-title">' + label('permissions') + '</div><div class="field-grid two-cols">';
    for (const key of ['download', 'create', 'update', 'publish', 'delete', 'submit', 'remote_execute', 'gpu', 'tpu', 'internet']) html += toggle('allow_' + key, cfg['allow_' + key], 'allow_' + key);
    html += '</div><label class="field-group"><span class="field-label">' + label('allowed_owners') + '</span><span class="field-help">' + label('owners_help') + '</span><input class="field-input" data-type="array" data-path="kaggle.allowed_owners" value="' + escapeAttr((cfg.allowed_owners || []).join(', ')) + '"></label>';
    const shapes = Array.isArray(cfg.allowed_machine_shapes) ? cfg.allowed_machine_shapes.filter(shape => KAGGLE_MACHINE_SHAPES.includes(shape)) : [];
    html += '<div class="field-group" role="group" aria-labelledby="kaggle-shapes-label" aria-describedby="kaggle-shapes-help"><div class="field-label" id="kaggle-shapes-label">' + label('allowed_machine_shapes') + '</div><span class="field-help" id="kaggle-shapes-help">' + label('allowed_machine_shapes_help') + '</span><div class="cfg-choice-items">';
    for (const shape of KAGGLE_MACHINE_SHAPES) html += '<label class="cfg-choice-item"><input type="checkbox" data-cfg-choice data-kaggle-shape="' + shape + '"' + (shapes.includes(shape) ? ' checked' : '') + '><span>' + label('machine_shape_' + shape) + '</span></label>';
    html += '</div><input class="field-input is-hidden" type="text" data-path="kaggle.allowed_machine_shapes" data-type="array" value="' + escapeAttr(shapes.join(', ')) + '" tabindex="-1" aria-hidden="true"></div>';
    html += '<div class="field-grid two-cols">' + number('max_transfer_mb', cfg.max_transfer_mb, 'max_transfer', 2048, 1048576) + number('max_storage_mb', cfg.max_storage_mb, 'max_storage', 10240, 10485760) + number('max_remote_jobs', cfg.max_remote_jobs, 'max_remote_jobs', 1, 16) + number('remote_timeout_minutes', cfg.remote_timeout_minutes, 'remote_timeout', 60, 720) + number('request_timeout_seconds', cfg.request_timeout_seconds, 'request_timeout', 60, 300) + '</div>';
    html += '<div class="field-group-title">' + label('local') + '</div><p class="field-help">' + label('local_help') + '</p><div class="field-grid two-cols">' + toggle('local.enabled', local.enabled, 'local_enabled') + toggle('local.allow_execute', local.allow_execute, 'allow_local_execute') + toggle('local.allow_internet', local.allow_internet, 'allow_internet', 'local_internet') + number('local.max_sessions', local.max_sessions, 'max_sessions', 1, 8) + number('local.cpu_cores', local.cpu_cores, 'cpu_cores', 2, 64) + number('local.memory_mb', local.memory_mb, 'memory', 4096, 262144) + number('local.idle_minutes', local.idle_minutes, 'idle_timeout', 15, 240) + number('local.cell_timeout_minutes', local.cell_timeout_minutes, 'cell_timeout', 30, 720) + '</div></div>';
    document.getElementById('content').innerHTML = html;
    const root = document.getElementById('content').firstElementChild;
    const resultRegion = root.querySelector('#kaggle-result');
    attachChangeListeners();
    const shapeInputs = [...root.querySelectorAll('[data-kaggle-shape]')];
    const shapeField = root.querySelector('[data-path="kaggle.allowed_machine_shapes"]');
    for (const input of shapeInputs) input.addEventListener('change', () => {
        shapeField.value = shapeInputs.filter(box => box.checked).map(box => box.dataset.kaggleShape).join(', ');
        shapeField.dispatchEvent(new Event('input', { bubbles: true }));
        setDirty(true);
    });
    async function action(button, fn) { button.disabled = true; try { const result = await fn(); if (!root.isConnected) return; cfgActionResult.show(resultRegion, 'success', result.account ? label('connected') + ': ' + result.account : label('saved')); await refresh(); } catch (err) { if (root.isConnected) cfgActionResult.show(resultRegion, 'error', err.message); } finally { button.disabled = false; } }
    async function post(path, body) { const response = await fetch('/api/kaggle/' + path, { method: 'POST', credentials: 'same-origin', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }); const data = await response.json(); if (!response.ok) throw Error(data.error || response.status); return data; }
    root.querySelector('#kaggle-secret-save').onclick = function () { const body = {}; for (const key of ['token', 'legacy_key']) { const input = root.querySelector('#kaggle-' + key); if (input.value) body[key] = input.value; } return action(this, async () => { const result = await post('credentials', body); if (root.isConnected) for (const key of ['token', 'legacy_key']) { const input = root.querySelector('#kaggle-' + key); if (input.value === body[key]) input.value = ''; } return result; }); };
    for (const key of ['token', 'legacy_key']) root.querySelector('#kaggle-remove-' + key).onclick = function () { return action(this, () => post('credentials', { [key]: '' })); };
    root.querySelector('#kaggle-test').onclick = function () { return action(this, () => post('test', {})); };
    async function refresh() { if (!root.isConnected) return; await cfgStatusBanner.check(root.querySelector('#kaggle-status'), signal => fetch('/api/kaggle/status', { signal }), data => ({ state: !data.enabled ? 'off' : data.configured ? 'unknown' : 'unconfigured', detail: label(!data.enabled ? 'disabled' : data.configured ? 'configured' : 'credentials_missing') })); }
    await refresh();
}
