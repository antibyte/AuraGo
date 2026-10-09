// cfg/local_wikipedia.js — Local Wikipedia: one offline Kiwix edition, its download, updates and storage.

const LOCAL_WIKI_LANGUAGES = [
    ['cs', 'Čeština'], ['da', 'Dansk'], ['de', 'Deutsch'], ['el', 'Ελληνικά'], ['en', 'English'], ['es', 'Español'],
    ['fr', 'Français'], ['hi', 'हिन्दी'], ['it', 'Italiano'], ['ja', '日本語'], ['nl', 'Nederlands'], ['no', 'Norsk'],
    ['pl', 'Polski'], ['pt', 'Português'], ['sv', 'Svenska'], ['zh', '中文']
];
const LOCAL_WIKI_STATES = ['not_installed', 'downloading', 'verifying', 'ready', 'interrupted', 'error'];
const LOCAL_WIKI_ERROR_CODES = ['insufficient_disk_space', 'free_space_unknown', 'checksum_mismatch', 'download_failed',
    'catalog_unreachable', 'zim_unreadable', 'fulltext_unsupported', 'busy', 'disabled', 'data_dir_invalid',
    'already_installed', 'no_operation', 'unknown_language'];
const LOCAL_WIKI_MARGIN_BYTES = 1073741824;

let _lwSection = null;
let _lwStatus = null;
let _lwCatalog = null;
let _lwCatalogLang = null;
let _lwCatalogError = '';
let _lwCatalogPending = false;
let _lwPollTimer = null;
let _lwActionPending = false;
let _lwMessage = null;
let _lwRuntimeHTML = '';

function localWikiEnsureData() {
    if (!configData.local_wikipedia) configData.local_wikipedia = {};
    const data = configData.local_wikipedia;
    if (data.agent_access === undefined) data.agent_access = true;
    if (data.update_check === undefined) data.update_check = true;
    if (data.variant !== 'maxi' && data.variant !== 'nopic') data.variant = 'nopic';
    if (typeof data.language !== 'string') data.language = '';
    if (typeof data.data_dir !== 'string') data.data_dir = '';
    return data;
}

function renderLocalWikipediaSection(section) {
    if (section) _lwSection = section;
    section = section || _lwSection;
    const data = localWikiEnsureData();
    const status = _lwStatus || {};
    const locked = status.data_dir_locked === true || (typeof isDockerRuntime === 'function' && isDockerRuntime());
    _lwCatalog = null;
    _lwCatalogLang = null;
    _lwCatalogError = '';
    _lwCatalogPending = false;
    _lwMessage = null;
    _lwRuntimeHTML = localWikiRuntimeHTML();

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + escapeHtml(section.label) + '</div>';
    html += '<div class="section-desc">' + escapeHtml(section.desc) + '</div>';
    html += '<div class="cfg-note-banner cfg-note-banner-warning">' + escapeHtml(t('config.local_wikipedia.warning_size')) + '</div>';
    html += '<div class="cfg-group-title">' + escapeHtml(t('config.local_wikipedia.group_settings')) + '</div>';
    html += localWikiToggle('enabled', data.enabled === true);
    html += localWikiToggle('agent_access', data.agent_access !== false);
    html += '<div class="field-grid two-cols">';
    html += localWikiSelect('language', data.language, localWikiLanguageOptions());
    html += localWikiSelect('variant', data.variant, localWikiVariantOptions());
    html += '</div>';
    html += localWikiDataDirField(data.data_dir, status, locked);
    html += localWikiToggle('update_check', data.update_check !== false);
    html += '<div class="cfg-group-title">' + escapeHtml(t('config.local_wikipedia.group_edition')) + '</div>';
    html += '<div id="lw-runtime" class="lw-runtime" data-config-fields>' + _lwRuntimeHTML + '</div>';
    html += '</div>';

    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
    const languageSelect = document.querySelector('[data-path="local_wikipedia.language"]');
    if (languageSelect) languageSelect.addEventListener('change', () => localWikiLoadCatalog(true));
    const variantSelect = document.querySelector('[data-path="local_wikipedia.variant"]');
    if (variantSelect) variantSelect.addEventListener('change', localWikiUpdateRuntimeDOM);
    localWikiRefreshStatus();
}

function localWikiToggle(key, checked) {
    return '<div class="field-group"><div class="field-label">' + escapeHtml(t('config.local_wikipedia.' + key)) + '</div>' +
        '<div class="field-help">' + escapeHtml(t('help.local_wikipedia.' + key)) + '</div>' +
        '<div class="toggle-wrap"><div class="toggle' + (checked ? ' on' : '') + '" data-path="local_wikipedia.' + key +
        '" onclick="toggleBool(this);localWikiUpdateRuntimeDOM()"></div><span class="toggle-label">' +
        escapeHtml(t(checked ? 'config.toggle.active' : 'config.toggle.inactive')) + '</span></div></div>';
}

function localWikiSelect(key, value, options) {
    const label = t('config.local_wikipedia.' + key);
    return '<div class="field-group"><div class="field-label">' + escapeHtml(label) + '</div>' +
        '<div class="field-help">' + escapeHtml(t('help.local_wikipedia.' + key)) + '</div>' +
        '<select class="field-select" data-path="local_wikipedia.' + key + '" aria-label="' + escapeAttr(label) + '">' +
        options.map(option => '<option value="' + escapeAttr(option[0]) + '"' + (String(value) === option[0] ? ' selected' : '') + '>' +
            escapeHtml(option[1]) + '</option>').join('') + '</select></div>';
}

function localWikiDataDirField(value, status, locked) {
    let html = '<div class="field-group"><div class="field-label">' + escapeHtml(t('config.local_wikipedia.data_dir')) + '</div>';
    if (locked) {
        html += '<div class="field-help" id="lw-data-dir-locked">' +
            escapeHtml(t('config.local_wikipedia.data_dir_docker', { path: status.data_dir || '…' })) + '</div>';
        html += '<input class="field-input" type="text" id="lw-data-dir-readonly" value="' + escapeAttr(status.data_dir || '') +
            '" disabled aria-describedby="lw-data-dir-locked">';
    } else {
        html += '<div class="field-help">' + escapeHtml(t('help.local_wikipedia.data_dir')) + '</div>';
        html += '<input class="field-input" type="text" data-path="local_wikipedia.data_dir" value="' + escapeAttr(value || '') +
            '" placeholder="' + escapeAttr(t('config.local_wikipedia.data_dir_placeholder')) + '" autocomplete="off" spellcheck="false">';
    }
    return html + '</div>';
}

function localWikiLanguageName(code) {
    const entry = LOCAL_WIKI_LANGUAGES.find(item => item[0] === code);
    return entry ? entry[1] : String(code || '');
}

function localWikiLanguageOptions() {
    const status = _lwStatus || {};
    const fulltext = {};
    (status.languages || []).forEach(item => { fulltext[item.code] = item.fulltext !== false; });
    const system = status.system_language ? localWikiLanguageName(status.system_language) : '…';
    const options = [['', t('config.local_wikipedia.language_system', { name: system })]];
    LOCAL_WIKI_LANGUAGES.forEach(([code, name]) => {
        options.push([code, fulltext[code] === false ? name + ' (' + t('config.local_wikipedia.title_only') + ')' : name]);
    });
    return options;
}

function localWikiVariantOptions() {
    const variants = (_lwCatalog && _lwCatalog.variants) || {};
    return ['nopic', 'maxi'].map(variant => {
        const label = t('config.local_wikipedia.variant_' + variant);
        const entry = variants[variant];
        return [variant, entry && entry.size ? label + ' · ' + localWikiFormatBytes(entry.size) : label];
    });
}

function localWikiApplyOptionLabels() {
    [['language', localWikiLanguageOptions()], ['variant', localWikiVariantOptions()]].forEach(([key, options]) => {
        const select = document.querySelector('[data-path="local_wikipedia.' + key + '"]');
        if (!select) return;
        options.forEach(([value, label]) => {
            const option = [...select.options].find(item => item.value === value);
            if (option && option.textContent !== label) option.textContent = label;
        });
    });
    const status = _lwStatus || {};
    const lockedHelp = document.getElementById('lw-data-dir-locked');
    if (lockedHelp && status.data_dir) lockedHelp.textContent = t('config.local_wikipedia.data_dir_docker', { path: status.data_dir });
    const readonly = document.getElementById('lw-data-dir-readonly');
    if (readonly && status.data_dir) readonly.value = status.data_dir;
}

function localWikiDraftVariant() {
    const select = document.querySelector('[data-path="local_wikipedia.variant"]');
    return select && select.value === 'maxi' ? 'maxi' : 'nopic';
}

function localWikiLocale() {
    return typeof lang === 'string' && lang ? lang : undefined;
}

function localWikiFormatBytes(value) {
    const bytes = Number(value);
    if (!Number.isFinite(bytes) || bytes < 0) return t('config.local_wikipedia.unknown');
    if (bytes >= 1e9) return (bytes / 1e9).toLocaleString(localWikiLocale(), { maximumFractionDigits: 1 }) + ' GB';
    if (bytes >= 1e6) return Math.round(bytes / 1e6).toLocaleString(localWikiLocale()) + ' MB';
    return Math.max(0, Math.round(bytes / 1e3)).toLocaleString(localWikiLocale()) + ' kB';
}

function localWikiFormatETA(seconds) {
    const value = Number(seconds);
    if (!Number.isFinite(value) || value <= 0) return t('config.local_wikipedia.eta_unknown');
    const hours = Math.floor(value / 3600);
    if (hours > 0) return t('config.local_wikipedia.eta_hours', { hours, minutes: Math.floor((value % 3600) / 60) });
    return t('config.local_wikipedia.eta_minutes', { minutes: Math.max(1, Math.ceil(value / 60)) });
}

function localWikiRequiredBytes(size) {
    const bytes = Number(size) || 0;
    return bytes + Math.max(LOCAL_WIKI_MARGIN_BYTES, Math.ceil(bytes / 100));
}

// Localized text for a refused request or catalog failure. The required/free
// numbers come from the response (insufficient_disk_space).
function localWikiErrorText(code, data) {
    const values = data || {};
    if (LOCAL_WIKI_ERROR_CODES.includes(code)) {
        return t('config.local_wikipedia.error_' + code, {
            required: localWikiFormatBytes(values.required_bytes),
            free: localWikiFormatBytes(values.free_bytes)
        });
    }
    return t('config.local_wikipedia.error_unknown', { code: String(code || values.error || '') });
}

// The status error comes from either the installed edition or the last
// install/update, and the server words its recommendation for that case (a
// download that cannot be read was removed, an installed edition has to be
// deleted). The server text is therefore preferred; the localized per-code text
// is only the fallback for a status without one.
function localWikiStatusErrorText(status) {
    const recommendation = typeof status.recommendation === 'string' ? status.recommendation.trim() : '';
    return recommendation || localWikiErrorText(status.error_code, status);
}

function localWikiBanner(kind, text, alert) {
    const cls = kind === 'warning' ? ' cfg-note-banner-warning' : kind === 'success' ? ' cfg-note-banner-success' :
        kind === 'info' ? ' cfg-note-banner-info' : '';
    return '<div class="cfg-note-banner' + cls + '"' + (alert ? ' role="alert"' : '') + '>' + escapeHtml(text) + '</div>';
}

function localWikiEditionLabel(edition) {
    const variant = edition.variant === 'maxi' ? 'maxi' : 'nopic';
    return localWikiLanguageName(edition.language) + ' · ' + t('config.local_wikipedia.variant_' + variant) +
        (edition.date ? ' · ' + edition.date : '');
}

// readable is false when the installed edition is on disk but cannot be opened;
// the full-text hint only means something for an edition that is being served.
function localWikiEditionFacts(titleKey, edition, fulltext, readable) {
    let html = '<div class="lw-facts"><strong>' + escapeHtml(t('config.local_wikipedia.' + titleKey)) + '</strong>';
    html += '<span>' + escapeHtml(localWikiEditionLabel(edition)) + '</span>';
    html += '<span>' + escapeHtml(t('config.local_wikipedia.size', { size: localWikiFormatBytes(edition.size) })) + '</span>';
    if (Number(edition.article_count) > 0) {
        html += '<span>' + escapeHtml(t('config.local_wikipedia.articles', { count: Number(edition.article_count).toLocaleString(localWikiLocale()) })) + '</span>';
    }
    if (readable === false) {
        html += '<span>' + escapeHtml(t('config.local_wikipedia.unreadable')) + '</span>';
    } else if (typeof fulltext === 'boolean') {
        html += '<span>' + escapeHtml(t(fulltext ? 'config.local_wikipedia.fulltext_on' : 'config.local_wikipedia.fulltext_off')) + '</span>';
    }
    return html + '</div>';
}

function localWikiSelectedHTML() {
    if (_lwCatalogPending) return '<div class="field-help">' + escapeHtml(t('config.local_wikipedia.catalog_loading')) + '</div>';
    if (_lwCatalogError) {
        return localWikiBanner('warning', localWikiErrorText(_lwCatalogError, {}), true) +
            '<div class="cfg-actions-row pw-action-row lw-actions"><button type="button" class="btn-secondary" data-lw-action="retry" ' +
            'onclick="localWikiLoadCatalog(true)">' + escapeHtml(t('config.local_wikipedia.retry')) + '</button></div>';
    }
    if (!_lwCatalog) return '';
    const variant = localWikiDraftVariant();
    const entry = (_lwCatalog.variants || {})[variant];
    if (!entry) return localWikiBanner('info', t('config.local_wikipedia.catalog_missing'));
    const edition = { language: _lwCatalog.language, variant, date: entry.date, size: entry.size, article_count: entry.article_count };
    return localWikiEditionFacts('selected_title', edition, _lwCatalog.fulltext, true) +
        '<div class="field-help">' + escapeHtml(t('config.local_wikipedia.required_space', { required: localWikiFormatBytes(localWikiRequiredBytes(entry.size)) })) + '</div>';
}

function localWikiProgressHTML(status) {
    const total = Number(status.bytes_total) || 0;
    const done = Number(status.bytes_done) || 0;
    const fraction = total > 0 ? Math.max(0, Math.min(1, done / total)) : 0;
    const percent = (fraction * 100).toLocaleString(localWikiLocale(), { maximumFractionDigits: 1 });
    let html = '<progress id="lw-progress" max="1000" value="' + Math.round(fraction * 1000) + '" aria-labelledby="lw-state"></progress>';
    html += '<div class="field-help">' + escapeHtml(t('config.local_wikipedia.progress', {
        done: localWikiFormatBytes(done), total: localWikiFormatBytes(total), percent
    })) + '</div>';
    if (status.state === 'downloading') {
        html += '<div class="field-help">' + escapeHtml(t('config.local_wikipedia.rate_eta', {
            rate: localWikiFormatBytes(status.rate), eta: localWikiFormatETA(status.eta_seconds)
        })) + '</div>';
    }
    return html;
}

function localWikiBlockedReason() {
    if (typeof hasUnsavedConfigChanges === 'function' && hasUnsavedConfigChanges()) return 'save_first';
    const data = configData.local_wikipedia || {};
    return data.enabled === true ? '' : 'enable_first';
}

function localWikiActionsHTML(status) {
    const blocked = localWikiBlockedReason();
    const busy = _lwActionPending || status.operation_in_progress === true;
    const disabled = Boolean(blocked) || busy;
    const readable = status.readable === true;
    const button = (action, labelKey, primary, isDisabled, handler) => '<button type="button" class="' +
        (primary ? 'btn-save' : 'btn-secondary') + '" data-lw-action="' + action + '" onclick="' + handler + '"' +
        (isDisabled ? ' disabled' : '') + '>' + escapeHtml(t('config.local_wikipedia.' + labelKey)) + '</button>';
    const buttons = [];
    if (status.operation_in_progress) {
        buttons.push(button('cancel', 'cancel', false, _lwActionPending, 'localWikiCancel()'));
    } else {
        if (status.state === 'interrupted') buttons.push(button('resume', 'resume', true, disabled, "localWikiInstall('resume')"));
        else if (!status.edition || status.selection_matches_installed === false) buttons.push(button('install', 'install', true, disabled, "localWikiInstall('install')"));
        else if (status.update_available) buttons.push(button('update', 'update', true, disabled, "localWikiInstall('update')"));
        // An edition that cannot be read has nothing to check an update for;
        // deleting it (and installing again) is the way out.
        if (readable && status.selection_matches_installed !== false) buttons.push(button('check', 'check_update', false, disabled, 'localWikiCheckUpdate()'));
        if (status.edition || status.state === 'interrupted') buttons.push(button('delete', 'delete', false, busy, 'localWikiDelete()'));
    }
    let html = '<div class="cfg-actions-row pw-action-row lw-actions">' + buttons.join('') + '</div>';
    if (blocked && !status.operation_in_progress) html += '<div class="field-help">' + escapeHtml(t('config.local_wikipedia.' + blocked)) + '</div>';
    return html;
}

function localWikiRuntimeHTML() {
    const status = _lwStatus;
    if (!status) {
        return _lwMessage ? localWikiBanner(_lwMessage.kind, _lwMessage.text, true) :
            '<div class="cfg-note-banner" role="status">' + escapeHtml(t('config.local_wikipedia.loading')) + '</div>';
    }
    const state = LOCAL_WIKI_STATES.includes(status.state) ? status.state : 'error';
    const readable = status.readable === true;
    const stateClass = state === 'ready' ? ' cfg-note-banner-success' : (state === 'error' || state === 'interrupted') ? ' cfg-note-banner-warning' : '';
    let html = '<div id="lw-state" class="cfg-note-banner' + stateClass + '" role="status" aria-live="polite">' +
        escapeHtml(t('config.local_wikipedia.state_' + state)) + '</div>';
    if (_lwMessage) html += localWikiBanner(_lwMessage.kind, _lwMessage.text, _lwMessage.kind === 'warning');
    if (status.error_code) {
        // A failed update leaves the installed edition online (state "ready"),
        // so the code is shown next to the state instead of replacing it.
        const informational = status.error_code === 'fulltext_unsupported';
        html += localWikiBanner(informational ? 'info' : 'warning', localWikiStatusErrorText(status), !informational);
        if (status.error_code === 'insufficient_disk_space' && Number(status.required_bytes) > 0) {
            html += '<div class="field-help">' + escapeHtml(t('config.local_wikipedia.required_space', { required: localWikiFormatBytes(status.required_bytes) })) + '</div>';
        }
    }
    if (status.edition) html += localWikiEditionFacts('installed_title', status.edition, status.fulltext, readable);
    if (status.edition && status.update_available && status.selection_matches_installed) {
        html += localWikiBanner('info', t('config.local_wikipedia.update_available', {
            date: status.update_available.date, size: localWikiFormatBytes(status.update_available.size)
        }));
    }
    if (status.edition && status.selection_matches_installed === false) html += localWikiBanner('info', t('config.local_wikipedia.selection_mismatch'));
    html += localWikiSelectedHTML();
    html += '<div class="field-help">' + escapeHtml(t('config.local_wikipedia.free_space', {
        path: status.data_dir || '—', free: localWikiFormatBytes(status.free_bytes)
    })) + '</div>';
    if (status.operation_in_progress) html += localWikiProgressHTML(status);
    return html + localWikiActionsHTML(status);
}

// Re-renders the status area only when its markup changed, and hands the focus
// back to the action button the user was on (polling replaces the buttons).
function localWikiUpdateRuntimeDOM() {
    const target = document.getElementById('lw-runtime');
    if (!target) return;
    const html = localWikiRuntimeHTML();
    if (html === _lwRuntimeHTML) return;
    const active = document.activeElement;
    const focused = active && target.contains(active) && active.dataset ? active.dataset.lwAction : '';
    target.innerHTML = html;
    _lwRuntimeHTML = html;
    if (focused) {
        const next = target.querySelector('[data-lw-action="' + focused + '"]');
        if (next && !next.disabled) next.focus();
    }
}

function localWikiSchedulePolling() {
    if (_lwPollTimer) {
        clearTimeout(_lwPollTimer);
        _lwPollTimer = null;
    }
    if (!document.getElementById('lw-runtime') || !_lwStatus) return;
    if (_lwStatus.operation_in_progress === true || _lwActionPending) {
        _lwPollTimer = setTimeout(localWikiRefreshStatus, 2000);
    }
}

async function localWikiRefreshStatus() {
    if (!document.getElementById('lw-runtime')) return;
    try {
        const response = await fetch('/api/local-wikipedia/status');
        let data = {};
        try { data = await response.json(); } catch (_) { data = {}; }
        if (!response.ok) throw new Error(localWikiErrorText(data.error_code || data.error, data));
        _lwStatus = data;
        localWikiApplyOptionLabels();
        if (_lwCatalogLang === null) localWikiLoadCatalog(false);
        localWikiUpdateRuntimeDOM();
    } catch (error) {
        _lwMessage = { kind: 'warning', text: t('config.local_wikipedia.error_prefix') + ': ' + error.message };
        localWikiUpdateRuntimeDOM();
    }
    localWikiSchedulePolling();
}

async function localWikiLoadCatalog(force) {
    const select = document.querySelector('[data-path="local_wikipedia.language"]');
    const language = select ? select.value : '';
    if (!force && _lwCatalog && _lwCatalogLang === language) return;
    _lwCatalogLang = language;
    _lwCatalogPending = true;
    _lwCatalogError = '';
    localWikiUpdateRuntimeDOM();
    try {
        const response = await fetch('/api/local-wikipedia/catalog?lang=' + encodeURIComponent(language));
        let data = {};
        try { data = await response.json(); } catch (_) { data = {}; }
        if (_lwCatalogLang !== language) return;
        if (response.ok) {
            _lwCatalog = data;
        } else {
            _lwCatalog = null;
            _lwCatalogError = data.error_code || 'catalog_unreachable';
        }
    } catch (_) {
        if (_lwCatalogLang !== language) return;
        _lwCatalog = null;
        _lwCatalogError = 'catalog_unreachable';
    }
    _lwCatalogPending = false;
    localWikiApplyOptionLabels();
    localWikiUpdateRuntimeDOM();
}

async function localWikiPost(path, body) {
    const response = await fetch(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body || {}) });
    let data = {};
    try { data = await response.json(); } catch (_) { data = {}; }
    return { response, data: data || {} };
}

function localWikiSelectedEntry() {
    const variants = _lwCatalog && _lwCatalog.variants ? _lwCatalog.variants : {};
    return variants[localWikiDraftVariant()] || null;
}

async function localWikiInstall(kind) {
    if (localWikiBlockedReason()) {
        localWikiUpdateRuntimeDOM();
        return;
    }
    if (kind !== 'resume') {
        const status = _lwStatus || {};
        const entry = localWikiSelectedEntry();
        const language = _lwCatalog ? _lwCatalog.language : (status.selection || {}).language;
        const confirmed = await showConfirm(
            t(kind === 'update' ? 'config.local_wikipedia.confirm_update_title' : 'config.local_wikipedia.confirm_install_title'),
            t('config.local_wikipedia.confirm_install', {
                edition: localWikiEditionLabel({ language, variant: localWikiDraftVariant(), date: entry ? entry.date : '' }),
                size: localWikiFormatBytes(entry ? entry.size : -1),
                free: localWikiFormatBytes(status.free_bytes)
            }));
        if (!confirmed) return;
    }
    await localWikiPostInstall({ replace_mode: 'keep_old', confirm_unknown_space: false });
}

// The install answers 202 once the download runs in the background. 409
// free_space_unknown and 422 insufficient_disk_space (with can_delete_old) are
// questions to the administrator; every other refusal is shown as text.
async function localWikiPostInstall(body) {
    _lwActionPending = true;
    _lwMessage = null;
    localWikiUpdateRuntimeDOM();
    try {
        const { response, data } = await localWikiPost('/api/local-wikipedia/install', body);
        if (response.ok) {
            await localWikiRefreshStatus();
            return;
        }
        const code = data.error_code || data.error || '';
        if (code === 'free_space_unknown' && !body.confirm_unknown_space) {
            const entry = localWikiSelectedEntry();
            if (await showConfirm(t('config.local_wikipedia.confirm_unknown_space_title'),
                t('config.local_wikipedia.confirm_unknown_space', { size: localWikiFormatBytes(entry ? entry.size : -1) }))) {
                await localWikiPostInstall(Object.assign({}, body, { confirm_unknown_space: true }));
            }
            return;
        }
        if (code === 'insufficient_disk_space' && data.can_delete_old === true && body.replace_mode !== 'delete_old_first') {
            if (await showConfirm(t('config.local_wikipedia.confirm_delete_old_title'), t('config.local_wikipedia.confirm_delete_old', {
                required: localWikiFormatBytes(data.required_bytes), free: localWikiFormatBytes(data.free_bytes)
            }))) {
                await localWikiPostInstall(Object.assign({}, body, { replace_mode: 'delete_old_first' }));
            }
            return;
        }
        _lwMessage = { kind: 'warning', text: localWikiErrorText(code, data) };
        // busy, already_installed and disabled mean the status shown is out of date.
        await localWikiRefreshStatus();
    } catch (error) {
        _lwMessage = { kind: 'warning', text: t('config.local_wikipedia.error_prefix') + ': ' + error.message };
    } finally {
        _lwActionPending = false;
        localWikiUpdateRuntimeDOM();
        localWikiSchedulePolling();
    }
}

async function localWikiSimpleAction(path, onSuccess) {
    _lwActionPending = true;
    _lwMessage = null;
    localWikiUpdateRuntimeDOM();
    try {
        const { response, data } = await localWikiPost(path, {});
        if (response.ok) {
            if (onSuccess) onSuccess(data);
        } else {
            _lwMessage = { kind: 'warning', text: localWikiErrorText(data.error_code || data.error, data) };
        }
    } catch (error) {
        _lwMessage = { kind: 'warning', text: t('config.local_wikipedia.error_prefix') + ': ' + error.message };
    } finally {
        _lwActionPending = false;
        await localWikiRefreshStatus();
    }
}

async function localWikiCancel() {
    await localWikiSimpleAction('/api/local-wikipedia/cancel');
}

async function localWikiDelete() {
    if (!(await showConfirm(t('config.local_wikipedia.confirm_delete_title'), t('config.local_wikipedia.confirm_delete')))) return;
    await localWikiSimpleAction('/api/local-wikipedia/delete');
}

async function localWikiCheckUpdate() {
    await localWikiSimpleAction('/api/local-wikipedia/check-update', data => {
        if (!data.update_available) _lwMessage = { kind: 'success', text: t('config.local_wikipedia.no_update') };
    });
}

document.addEventListener('aurago:config-saved', () => {
    if (!document.getElementById('lw-runtime')) return;
    localWikiRefreshStatus();
    localWikiLoadCatalog(true);
});
document.addEventListener('input', () => window.setTimeout(localWikiUpdateRuntimeDOM, 0));
document.addEventListener('change', () => window.setTimeout(localWikiUpdateRuntimeDOM, 0));
window.addEventListener('cfg:section-leave', () => {
    if (_lwPollTimer) {
        clearTimeout(_lwPollTimer);
        _lwPollTimer = null;
    }
});
window.addEventListener('beforeunload', () => {
    if (_lwPollTimer) clearTimeout(_lwPollTimer);
});
