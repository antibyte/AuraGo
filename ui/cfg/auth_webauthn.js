// Protected auth settings use their own save action and preserve the ordinary config draft.
function renderWebAuthnSection(settings, status) {
    const field = (id, key, input) => `<div class="field-group"><label class="field-label" for="${id}">${t(key)}</label>${input}</div>`;
    return `<section id="auth-webauthn" aria-labelledby="auth-webauthn-title">
        <h2 class="auth-section-title auth-section-title-spaced" id="auth-webauthn-title">${t('webauthn.title')}</h2>
        <p class="field-help">${t('webauthn.intro')}</p>
        <p class="field-help" id="auth-webauthn-capability"></p>
        ${field('wa-enabled', 'common.enabled', `<select class="field-input" id="wa-enabled" data-search-path="auth.webauthn.enabled"><option value="false" ${!settings.enabled ? 'selected' : ''}>${t('common.disabled')}</option><option value="true" ${settings.enabled ? 'selected' : ''}>${t('common.enabled')}</option></select>`)}
        ${field('wa-mode', 'webauthn.mode', `<select class="field-input" id="wa-mode" data-search-path="auth.webauthn.login_mode"><option value="passkey" ${settings.login_mode !== 'second_factor' ? 'selected' : ''}>${t('webauthn.mode_passkey')}</option><option value="second_factor" ${settings.login_mode === 'second_factor' ? 'selected' : ''}>${t('webauthn.mode_second_factor')}</option></select>`)}
        <p class="field-help" id="wa-mode-help"></p>
        ${field('wa-rpid', 'webauthn.rpid', `<input class="field-input" id="wa-rpid" data-search-path="auth.webauthn.rp_id" value="${escapeAttr(settings.rp_id || '')}" placeholder="aurago.example.test" autocomplete="off" spellcheck="false">`)}
        ${field('wa-origins', 'webauthn.origins', `<textarea class="field-input" id="wa-origins" data-search-path="auth.webauthn.origins" rows="2" placeholder="https://aurago.example.test" spellcheck="false">${escapeHtml((settings.origins || []).join('\n'))}</textarea>`)}
        <p class="field-help">${t('webauthn.origin_hint')}</p>
        <p class="field-help">${t('webauthn.step_up')}</p>
        ${field('auth-webauthn-current-pw', 'login.password_label', '<input class="field-input" id="auth-webauthn-current-pw" type="password" autocomplete="current-password">')}
        ${status.totp_enabled ? field('auth-webauthn-current-totp', 'login.totp_label', '<input class="field-input" id="auth-webauthn-current-totp" inputmode="numeric" autocomplete="one-time-code" maxlength="6">') : ''}
        <button type="button" class="auth-primary-btn" data-wa-action="settings">${t('common.btn_save')}</button>
        <p class="field-help">${t('webauthn.verify_hint')}</p>
        ${field('wa-label', 'webauthn.label', '<input class="field-input" id="wa-label" maxlength="64" autocomplete="off">')}
        <button type="button" class="auth-primary-btn" data-wa-action="add">${t('webauthn.add')}</button>
        <button type="button" class="auth-totp-disable-btn is-hidden" data-wa-cancel>${t('common.btn_cancel')}</button>
        <p id="wa-status" class="auth-inline-msg is-hidden" role="status" aria-live="polite"></p>
        <div id="wa-credentials" aria-live="polite"></div>
    </section>`;
}

function initWebAuthnSection() {
    const root = document.getElementById('auth-webauthn');
    if (!root || !globalThis.AuraWebAuthn) return;
    const api = AuraWebAuthn;
    const infoController = new AbortController();
    let actionController = null;
    let registryRP = '';
    let listRevision = 0;
    let pending = false;
    let activeAction = '';
    let savedSettings = configData.auth?.webauthn || {};
    let settingsEdited = false;
    const el = id => root.querySelector('#' + id);
    const status = el('wa-status');
    const capability = root.querySelector('#auth-webauthn-capability');
    capability.textContent = t(api.available() ? 'webauthn.passkey_hint' : 'webauthn.unsupported');
    const modeHelp = () => { el('wa-mode-help').textContent = t(el('wa-mode').value === 'second_factor' ? 'webauthn.second_factor_hint' : 'webauthn.passkey_hint'); };
    el('wa-mode').addEventListener('change', modeHelp); modeHelp();
    const readSettings = () => ({ enabled: el('wa-enabled').value === 'true', login_mode: el('wa-mode').value, rp_id: el('wa-rpid').value.trim(), origins: el('wa-origins').value.split('\n').map(s => s.trim()).filter(Boolean) });
    root.addEventListener('input', event => { if (event.target.matches('[data-search-path]')) settingsEdited = true; });
    root.addEventListener('change', event => { if (event.target.matches('[data-search-path]')) settingsEdited = true; });
    function setBusy(value) {
        pending = value;
        root.querySelectorAll('button, input, select, textarea').forEach(control => { control.disabled = value; });
        const cancel = root.querySelector('[data-wa-cancel]');
        cancel.disabled = false; cancel.classList.toggle('is-hidden', !value || activeAction !== 'add');
        if (!value) root.querySelector('[data-wa-action="add"]').disabled = !api.available();
        root.setAttribute('aria-busy', String(value));
    }
    const observer = new MutationObserver(() => {
        if (!root.isConnected) { infoController.abort(); actionController?.abort(); observer.disconnect(); }
    });
    observer.observe(root.parentNode, { childList: true });
    async function loadCredentials(signal) {
	    const revision = ++listRevision;
        const data = await api.send('/api/auth/webauthn/credentials', null, 'GET', signal);
        if (!root.isConnected || revision !== listRevision) return;
        registryRP = data.registry_rp_id || '';
        savedSettings = data.settings;
        if (!settingsEdited) {
            const baseline = window.AuraConfigState?.get('auth.webauthn') || {};
            configData.auth = configData.auth || {};
            configData.auth.webauthn = savedSettings;
            if (window.AuraConfigState) window.AuraConfigState.commitSectionSent('auth.webauthn', savedSettings, baseline);
            el('wa-enabled').value = String(!!savedSettings.enabled);
            el('wa-mode').value = savedSettings.login_mode || 'passkey';
            el('wa-rpid').value = savedSettings.rp_id || '';
            el('wa-origins').value = (savedSettings.origins || []).join('\n');
            modeHelp();
        }
        const list = el('wa-credentials');
        list.replaceChildren();
        if (!data.credentials.length) { const empty = document.createElement('p'); empty.className = 'field-help'; empty.textContent = t('webauthn.no_keys'); list.appendChild(empty); }
        for (const credential of data.credentials) {
            const row = document.createElement('div'); row.className = 'field-group'; row.dataset.credential = credential.id;
            const label = document.createElement('input'); label.className = 'field-input'; label.value = credential.label; label.maxLength = 64; label.setAttribute('aria-label', t('webauthn.label')); row.appendChild(label);
            if (credential.last_used_at && !credential.last_used_at.startsWith('0001-')) { const date = document.createElement('small'); date.className = 'field-help'; date.textContent = t('webauthn.last_used') + ': ' + new Date(credential.last_used_at).toLocaleString(document.documentElement.lang); row.appendChild(date); }
            for (const [action, key] of [['rename', 'common.btn_save'], ['remove', 'common.btn_delete']]) {
                const button = document.createElement('button'); button.type = 'button'; button.className = action === 'remove' ? 'auth-totp-disable-btn' : 'auth-primary-btn'; button.dataset.waAction = action; button.textContent = t(key); row.appendChild(button);
            }
            list.appendChild(row);
        }
        setBusy(pending);
    }
    root.addEventListener('click', async event => {
        if (event.target.closest('[data-wa-cancel]')) { actionController?.abort(); return; }
        const button = event.target.closest('[data-wa-action]');
        if (!button || pending) return;
        const action = button.dataset.waAction;
        const sentSettings = window.AuraConfigState?.get('auth.webauthn') || {};
        const stepUp = authReadStepUp('auth-webauthn');
        if (authStepUpMissing(stepUp)) { authSetStatus(status, 'error', t('webauthn.step_up')); return; }
        let settings, reset = false;
        if (action === 'settings') {
            settings = readSettings();
            if (registryRP && registryRP !== settings.rp_id) {
                if (settings.enabled || configData.auth?.webauthn?.enabled) { authSetStatus(status, 'error', t('webauthn.reset_required')); return; }
                if (!await showConfirm(t('webauthn.title'), t('webauthn.reset_confirm'))) return;
                reset = true;
            }
        }
        if (action === 'remove' && !await showConfirm(t('webauthn.title'), t('webauthn.remove_confirm'))) return;
        if (!root.isConnected) return;
        if (action === 'add' && !el('wa-label').value.trim()) { authSetStatus(status, 'error', t('webauthn.label')); el('wa-label').focus(); return; }
        if (action === 'add' && JSON.stringify(readSettings()) !== JSON.stringify({ enabled: !!savedSettings.enabled, login_mode: savedSettings.login_mode || 'passkey', rp_id: savedSettings.rp_id || '', origins: savedSettings.origins || [] })) { authSetStatus(status, 'error', t('webauthn.verify_hint')); return; }
        activeAction = action;
        actionController = new AbortController(); setBusy(true);
        authSetStatus(status, '', t('common.loading'));
        try {
            if (action === 'settings') {
                await api.send('/api/auth/webauthn/settings', { settings, reset_credentials: reset, ...stepUp }, 'PUT', actionController.signal);
                if (root.isConnected) {
                    settingsEdited = false;
                    configData.auth = configData.auth || {};
                    configData.auth.webauthn = settings;
                    if (window.AuraConfigState) window.AuraConfigState.commitSectionSent('auth.webauthn', settings, sentSettings);
                }
            } else if (action === 'add') {
                const label = el('wa-label').value.trim();
                await api.register(label, stepUp, actionController.signal);
            } else {
                const row = button.closest('[data-credential]');
                await api.send('/api/auth/webauthn/credentials/' + encodeURIComponent(row.dataset.credential), { ...stepUp, label: row.querySelector('input').value.trim() }, action === 'remove' ? 'DELETE' : 'PATCH', actionController.signal);
            }
            if (!root.isConnected) return;
            authSetStatus(status, 'success', t('webauthn.saved'));
            el('auth-webauthn-current-pw').value = '';
            const code = el('auth-webauthn-current-totp'); if (code) code.value = '';
            await loadCredentials(actionController.signal);
        } catch (error) {
            if (root.isConnected) authSetStatus(status, 'error', t(api.errorKey(error)));
            if (error.code === 'webauthn_uncertain' && root.isConnected) await loadCredentials(infoController.signal).catch(() => {});
        } finally {
            actionController = null;
            activeAction = '';
            if (root.isConnected) { setBusy(false); if (button.isConnected) button.focus(); }
        }
    });
    setBusy(false);
    loadCredentials(infoController.signal).catch(error => { if (root.isConnected) authSetStatus(status, 'error', t(api.errorKey(error))); });
}
