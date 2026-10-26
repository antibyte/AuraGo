// S3 settings share one saved draft while managed RustFS keeps its own lifecycle.
'use strict';

let s3ConfigView = null;
let s3ConfigSection = null;

function s3Text(key) { return t('config.s3.' + key); }
function s3Value(path, saved = false) {
    return window.AuraConfigState ? AuraConfigState.get(path, { saved }) : getNestedValue(configData, path);
}
function s3SavedIdentity() {
    return JSON.stringify([s3Value('s3', true), s3Value('rustfs', true), s3Value('docker', true)]);
}
function s3Dispose() {
    clearTimeout(s3ConfigView?.timer);
    s3ConfigView?.controller.abort();
    s3ConfigView = null;
}
window.addEventListener('cfg:section-leave', s3Dispose);
window.addEventListener('hashchange', s3Dispose);
window.addEventListener('beforeunload', s3Dispose);
document.addEventListener('cfg:statechange', () => {
    if (!s3ConfigView) return;
    if (s3ConfigView.identity !== s3SavedIdentity()) renderS3Section(s3ConfigSection);
    else s3UpdateActions();
});

function s3ViewCurrent(view) {
    return s3ConfigView === view && view.root.isConnected && view.identity === s3SavedIdentity();
}
function s3Field(label, input, help = '') {
    return '<div class="field-group"><div class="field-label">' + escapeHtml(label) + '</div>'
        + (help ? '<div class="field-help">' + escapeHtml(help) + '</div>' : '') + input + '</div>';
}
function s3Checkbox(path, label, help = '') {
    const enabled = s3Value(path) === true;
    return s3Field(label, '<div class="toggle-wrap"><div class="toggle' + (enabled ? ' on' : '')
        + '" data-path="' + escapeAttr(path) + '" aria-label="' + escapeAttr(label) + '" onclick="toggleBool(this)"></div>'
        + '<span class="toggle-label">' + escapeHtml(t(enabled ? 'config.toggle.active' : 'config.toggle.inactive')) + '</span></div>', help);
}

function renderS3Section(section) {
    s3Dispose();
    s3ConfigSection = section || s3ConfigSection;
    section = s3ConfigSection;
    const data = s3Value('s3') || {};
    const managed = s3Value('rustfs') || {};
    const provider = data.provider || 'external';
    const fields = (schema.find(item => item.yaml_key === 's3') || {}).children || [];
    const baseFields = fields.filter(field => ['enabled', 'readonly'].includes(field.yaml_key));
    const externalFields = fields.filter(field => !['enabled', 'readonly', 'provider'].includes(field.yaml_key));
    let html = '<section class="cfg-section active" id="s3-settings"><div class="section-header">' + escapeHtml(section.label)
        + '</div><div class="section-desc">' + escapeHtml(section.desc) + '</div>';
    html += renderFields(baseFields, data, 's3');
    html += s3Field(s3Text('provider'), '<select class="field-select" id="s3-provider" data-path="s3.provider" aria-label="' + escapeAttr(s3Text('provider')) + '">'
        + '<option value="external"' + (provider === 'external' ? ' selected' : '') + '>' + escapeHtml(s3Text('external')) + '</option>'
        + '<option value="rustfs"' + (provider === 'rustfs' ? ' selected' : ' disabled') + '>' + escapeHtml(s3Text('managed')) + '</option></select>', s3Text('provider_help'));
    html += '<p id="s3-provider-reason" class="field-help" role="status"></p>';
    if (provider === 'external') html += renderFields(externalFields, data, 's3');
    else html += '<details class="cfg-advanced-panel"><summary>' + escapeHtml(s3Text('external')) + '</summary>'
        + '<div class="cfg-advanced-body">' + renderFields(externalFields, data, 's3') + '</div></details>';
    if (window.AuraConfigIntegrationActions) html += AuraConfigIntegrationActions.render('s3');
    html += '<div class="pw-u-subsection-title">' + escapeHtml(s3Text('managed')) + '</div>';
    html += s3Checkbox('rustfs.enabled', s3Text('enabled'), s3Text('retention'));
    html += s3Field(s3Text('bucket'), '<input class="field-input" id="s3-rustfs-bucket" data-path="rustfs.bucket" aria-label="' + escapeAttr(s3Text('bucket')) + '" value="'
        + escapeAttr(managed.bucket || 'aurago') + '" maxlength="63">', s3Text('bucket_help'));
    html += '<div id="s3-rustfs-status" class="pw-u-status-banner" role="status" aria-live="polite">' + escapeHtml(s3Text('checking')) + '</div>';
    html += s3Field(s3Text('endpoint'), '<input class="field-input" id="s3-rustfs-endpoint" readonly aria-label="' + escapeAttr(s3Text('endpoint')) + '">');
    html += '<div class="field-grid two-cols">';
    for (const action of ['preflight', 'install', 'start', 'stop', 'recreate', 'remove']) {
        html += '<button type="button" class="btn-save btn-secondary" id="s3-rustfs-' + action + '">' + escapeHtml(s3Text(action)) + '</button>';
    }
    html += '<button type="button" class="btn-save btn-secondary" id="s3-rustfs-refresh">' + escapeHtml(s3Text('refresh')) + '</button>';
    html += '</div><p id="s3-rustfs-action-reason" class="field-help" role="status"></p>';
    html += '<p id="s3-rustfs-operation" role="status" aria-live="polite"></p>';
    html += '<details class="cfg-advanced-panel"><summary>' + escapeHtml(s3Text('details')) + '</summary>'
        + '<div class="cfg-advanced-body"><textarea class="field-input" id="s3-rustfs-details" rows="8" readonly aria-label="' + escapeAttr(s3Text('details')) + '"></textarea></div></details>';
    html += '<div class="pw-u-subsection-title">' + escapeHtml(s3Text('permissions')) + '</div>';
    html += s3Checkbox('rustfs.agent_access', s3Text('agent_access'), s3Text('permissions_help'));
    for (const permission of ['read', 'write', 'delete']) {
        html += s3Checkbox('rustfs.permissions.allow_' + permission, s3Text('allow_' + permission));
    }
    html += '<div class="field-help">' + escapeHtml(s3Text('prefix_help')) + '</div><div id="s3-locations"></div>';
    html += '<button type="button" class="btn-save btn-secondary" id="s3-location-add">' + escapeHtml(s3Text('add_location')) + '</button></section>';
    document.getElementById('content').innerHTML = html;
    const root = document.getElementById('s3-settings');
    s3ConfigView = { root, controller: new AbortController(), identity: s3SavedIdentity(), status: null, operation: null, operationID: '', busy: false, timer: 0, pollSequence: 0 };
    if (window.AuraConfigState) AuraConfigState.markAtomic('rustfs.permissions.allowed_locations');
    s3RenderLocations();
    root.querySelector('#s3-provider').addEventListener('change', () => {
        AuraConfigState.syncFromDOM(root);
        setDirty(true);
        renderS3Section(section);
    });
    root.querySelector('#s3-location-add').addEventListener('click', () => {
        s3ChangeLocations([...s3Locations(), { bucket: s3Value('rustfs.bucket') || 'aurago', prefixes: [] }], true);
    });
    root.querySelector('#s3-rustfs-refresh').addEventListener('click', () => s3RefreshStatus(s3ConfigView));
    attachChangeListeners();
    if (window.AuraConfigIntegrationActions) AuraConfigIntegrationActions.bind('s3');
    for (const action of ['preflight', 'install', 'start', 'stop', 'recreate', 'remove']) {
        AuraConfigActions.register('s3-rustfs-' + action, { elementId: 's3-rustfs-' + action, requiresSaved: true, run: () => s3RustFSAction(action) });
    }
    s3UpdateActions();
    s3RefreshStatus(s3ConfigView);
}

function s3Locations() {
    return JSON.parse(JSON.stringify(s3Value('rustfs.permissions.allowed_locations') || []));
}
function s3ChangeLocations(locations, render = false) {
    if (render) AuraConfigState.replace('rustfs.permissions.allowed_locations', locations, s3RenderLocations);
    else AuraConfigState.set('rustfs.permissions.allowed_locations', locations);
    setDirty(true);
}
function s3RenderLocations() {
    const container = document.getElementById('s3-locations');
    if (!container) return;
    const locations = s3Locations();
    container.innerHTML = locations.map((location, index) => {
        const prefixes = Array.isArray(location.prefixes) ? location.prefixes : [];
        const whole = prefixes.includes('');
        return '<div class="field-group" data-s3-location="' + index + '"><label class="field-label" for="s3-location-bucket-' + index + '">'
            + escapeHtml(s3Text('bucket')) + '</label><input class="field-input" id="s3-location-bucket-' + index + '" data-s3-field="bucket" value="' + escapeAttr(location.bucket || '') + '" maxlength="63">'
            + '<label class="field-label" for="s3-location-prefixes-' + index + '">' + escapeHtml(s3Text('prefixes')) + '</label>'
            + '<textarea class="field-input" id="s3-location-prefixes-' + index + '" data-s3-field="prefixes" rows="3"' + (whole ? ' disabled' : '') + '>'
            + escapeHtml(prefixes.filter(prefix => prefix !== '').join('\n')) + '</textarea>'
            + '<label class="toggle-wrap"><input type="checkbox" data-s3-field="whole"' + (whole ? ' checked' : '') + '> ' + escapeHtml(s3Text('whole_bucket')) + '</label>'
            + '<button type="button" class="btn-save btn-secondary" data-s3-remove="' + index + '">' + escapeHtml(s3Text('remove_location')) + '</button></div>';
    }).join('') || '<p class="field-help">' + escapeHtml(s3Text('no_locations')) + '</p>';
    container.oninput = event => {
        const field = event.target.dataset.s3Field;
        if (!field || field === 'whole') return;
        const index = Number(event.target.closest('[data-s3-location]').dataset.s3Location);
        const next = s3Locations();
        if (field === 'bucket') next[index].bucket = event.target.value;
        else next[index].prefixes = event.target.value.split(/\r?\n/).filter(prefix => prefix !== '');
        s3ChangeLocations(next);
    };
    container.onchange = event => {
        if (event.target.dataset.s3Field !== 'whole') return;
        const next = s3Locations();
        next[Number(event.target.closest('[data-s3-location]').dataset.s3Location)].prefixes = event.target.checked ? [''] : [];
        s3ChangeLocations(next, true);
    };
    container.onclick = event => {
        const button = event.target.closest('[data-s3-remove]');
        if (!button) return;
        s3ChangeLocations(s3Locations().filter((_, index) => index !== Number(button.dataset.s3Remove)), true);
    };
}

function s3DockerReason() {
    if (s3Value('docker.enabled', true) !== true) return s3Text('docker_disabled');
    if (s3Value('docker.readonly', true) === true) return s3Text('docker_readonly');
    const reason = s3ConfigView?.status?.reason || s3ConfigView?.status?.state;
    if (['docker_unavailable', 'unsupported_placement'].includes(reason)) return s3StateText(reason);
    return '';
}
function s3ActionReason(action) {
    const view = s3ConfigView;
    if (!view) return s3Text('unavailable');
    if (AuraConfigState.isDirty()) return t('config.precision.action_save_first');
    if (action === 'preflight') return view.busy ? s3Text('busy') : '';
    if (s3DockerReason()) return s3DockerReason();
    if (s3Value('rustfs.enabled', true) !== true && !['stop', 'remove'].includes(action)) return s3StateText('disabled');
    const active = view.operation && ['accepted', 'running'].includes(view.operation.state);
    if ((view.busy || active) && action !== 'stop') return s3Text('busy');
    if (!view.status) return s3Text('checking');
    const reason = view.status.reason || view.status.state;
    if (reason === 'foreign_resource') return s3StateText('foreign_resource');
    if (reason === 'recovery_required' && !['stop', 'remove'].includes(action)) return s3StateText('recovery_required');
    if (action === 'install' && view.status.provisioned) return s3Text('already_installed');
    if (['start', 'recreate'].includes(action) && !view.status.provisioned) return s3Text('setup_required');
    if (action === 'start' && view.status.running) return s3Text('already_running');
    if (action === 'stop' && !view.status.running && !active) return s3Text('stopped');
    if (action === 'remove' && !view.status.installed) return s3Text('not_installed');
    return '';
}
function s3UpdateActions() {
    const view = s3ConfigView;
    if (!view?.root.isConnected) return;
    for (const action of ['preflight', 'install', 'start', 'stop', 'recreate', 'remove']) {
        const button = view.root.querySelector('#s3-rustfs-' + action);
        const reason = s3ActionReason(action);
        button.disabled = !!reason || button.dataset.configActionBusy === 'true';
        button.title = reason;
    }
    view.root.querySelector('#s3-rustfs-action-reason').textContent = AuraConfigState.isDirty() ? t('config.precision.action_save_first') : s3DockerReason();
}
function s3StateText(state) {
    state = ({ pending: 'permission_pending', denied: 'permission_denied', outcome_unknown: 'unknown', rustfs_disabled: 'disabled' })[state] || state;
    const known = ['disabled', 'not_installed', 'installing', 'ready', 'stopped', 'starting', 'stopping', 'permission_pending', 'permission_denied', 'docker_unavailable', 'unsupported_placement', 'foreign_resource', 'recovery_required', 'stop_unconfirmed', 'accepted', 'running', 'succeeded', 'failed', 'cancelled', 'unknown'];
    return known.includes(state) ? s3Text('state_' + state) : s3Text('unavailable');
}
function s3OperationText(operation) {
    const phases = ['checking', 'image_pull', 'credentials', 'creating', 'starting', 'iam', 'verifying', 'stopping', 'removing'];
    return s3StateText(operation.state) + (phases.includes(operation.phase) ? ' · ' + s3Text('phase_' + operation.phase) : '');
}
async function s3Request(view, path, options = {}) {
    const response = await fetch(path, { credentials: 'same-origin', ...options, signal: AbortSignal.any([view.controller.signal, AbortSignal.timeout(15000)]) });
    const body = await response.json();
    if (!response.ok) throw new Error(typeof body.code === 'string' ? body.code : 'request_failed');
    return body;
}
function s3ShowStatus(view) {
    if (!s3ViewCurrent(view)) return;
    const status = view.status;
    view.root.querySelector('#s3-rustfs-status').textContent = s3StateText(status.reason || status.state) + ' · ' + s3Text('data_access') + ': ' + s3StateText(status.access_state);
    view.root.querySelector('#s3-rustfs-endpoint').value = status.endpoint || '';
    view.root.querySelector('#s3-rustfs-bucket').readOnly = status.provisioned === true;
    view.root.querySelector('#s3-rustfs-details').value = JSON.stringify(status, null, 2);
    const choice = view.root.querySelector('#s3-provider option[value="rustfs"]');
    choice.disabled = !status.provisioned && view.root.querySelector('#s3-provider').value !== 'rustfs';
    view.root.querySelector('#s3-provider-reason').textContent = status.provisioned ? '' : s3Text('setup_required');
    s3UpdateActions();
}
async function s3RefreshStatus(view) {
    if (!s3ViewCurrent(view)) return;
    clearTimeout(view.timer);
    const sequence = ++view.pollSequence;
    try {
        const status = await s3Request(view, '/api/rustfs/status');
        if (!s3ViewCurrent(view) || sequence !== view.pollSequence) return;
        view.status = status;
        s3ShowStatus(view);
        const operationID = view.operationID || status.operation_id;
        if (operationID) {
            const operation = await s3Request(view, '/api/rustfs/operations/' + encodeURIComponent(operationID));
            if (!s3ViewCurrent(view) || sequence !== view.pollSequence) return;
            view.operation = operation;
            view.root.querySelector('#s3-rustfs-operation').textContent = s3OperationText(operation);
            if (['accepted', 'running'].includes(operation.state)) view.timer = setTimeout(() => s3RefreshStatus(view), 2000);
            s3UpdateActions();
        }
    } catch (error) {
        if (!s3ViewCurrent(view) || sequence !== view.pollSequence || view.controller.signal.aborted) return;
        if (error.message === 'operation_not_found' && view.status) {
            view.operationID = '';
            view.operation = null;
            view.root.querySelector('#s3-rustfs-operation').textContent = s3StateText('unknown');
            s3UpdateActions();
            return;
        }
        view.status = null;
        view.root.querySelector('#s3-rustfs-status').textContent = s3Text('request_failed');
        view.root.querySelector('#s3-rustfs-details').value = error.message;
        s3UpdateActions();
    }
}
async function s3RustFSAction(action) {
    const view = s3ConfigView;
    if (!view || s3ActionReason(action)) return;
    if (['recreate', 'remove'].includes(action) && !await showConfirm(s3Text(action), s3Text(action + '_confirm'))) return;
    if (!s3ViewCurrent(view) || s3ActionReason(action)) return;
    clearTimeout(view.timer);
    view.pollSequence++;
    view.busy = true;
    s3UpdateActions();
    try {
        const result = await s3Request(view, '/api/rustfs/' + action, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}' });
        if (!s3ViewCurrent(view)) return;
        if (action === 'preflight') {
            view.root.querySelector('#s3-rustfs-status').textContent = result.supported && result.mutation_allowed ? s3Text('preflight_ready') : s3Text('preflight_blocked');
            view.root.querySelector('#s3-rustfs-details').value = JSON.stringify(result, null, 2);
        } else {
            if (!result.operation?.id) throw new Error('invalid_receipt');
            view.operationID = result.operation.id;
            view.operation = result.operation;
            await s3RefreshStatus(view);
        }
    } catch (error) {
        if (s3ViewCurrent(view) && !view.controller.signal.aborted) {
            view.root.querySelector('#s3-rustfs-operation').textContent = s3Text('request_failed');
            view.root.querySelector('#s3-rustfs-details').value = error.message;
        }
    } finally {
        if (s3ViewCurrent(view)) {
            view.busy = false;
            // The shared action controller clears disabled after this promise settles.
            setTimeout(s3UpdateActions, 0);
        }
    }
}
