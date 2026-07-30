(function () {
    'use strict';

    let activePrompt = null;
    let activeOverlay = null;
    let submitting = false;

    function activeSessionId() {
        if (typeof getActiveSessionId === 'function') return getActiveSessionId();
        return (window.SessionDrawer && window.SessionDrawer.getActiveSessionId && window.SessionDrawer.getActiveSessionId()) || 'default';
    }

    function tr(key, fallback) {
        if (typeof t !== 'function') return fallback;
        const translated = t(key);
        return translated === key ? fallback : translated;
    }

    function clearSecretInput() {
        if (!activeOverlay) return;
        const input = activeOverlay.querySelector('.vault-secret-modal-input');
        if (input) input.value = '';
    }

    function closeVaultSecretPrompt() {
        clearSecretInput();
        if (activeOverlay) activeOverlay.remove();
        activeOverlay = null;
        activePrompt = null;
        submitting = false;
    }

    async function cancelPrompt(prompt, keepalive) {
        if (!prompt || !prompt.request_id || !prompt.session_id) return;
        try {
            await fetch('/api/agent/vault-secret/cancel', {
                method: 'POST',
                credentials: 'same-origin',
                keepalive: Boolean(keepalive),
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    session_id: prompt.session_id,
                    request_id: prompt.request_id
                })
            });
        } catch (_) { }
    }

    async function submitPrompt(input, saveButton, cancelButton) {
        if (!activePrompt || submitting || !input || !input.value) return;
        submitting = true;
        saveButton.disabled = true;
        cancelButton.disabled = true;

        let secretValue = input.value;
        input.value = '';
        let requestBody = JSON.stringify({
            session_id: activePrompt.session_id,
            request_id: activePrompt.request_id,
            vault_key: activePrompt.vault_key,
            value: secretValue
        });
        secretValue = '';
        try {
            const request = fetch('/api/agent/vault-secret/submit', {
                method: 'POST',
                credentials: 'same-origin',
                headers: { 'Content-Type': 'application/json' },
                body: requestBody
            });
            requestBody = '';
            const response = await request;
            if (!response.ok) {
                const error = await response.json().catch(() => ({}));
                const message = error && error.error_code
                    ? tr('chat.vault_secret_error_code', 'The secret could not be stored ({{code}}).').replace('{{code}}', error.error_code)
                    : tr('chat.vault_secret_error', 'The secret could not be stored.');
                const errorNode = activeOverlay && activeOverlay.querySelector('.vault-secret-modal-error');
                if (errorNode) {
                    errorNode.textContent = message;
                    errorNode.hidden = false;
                }
                submitting = false;
                saveButton.disabled = false;
                cancelButton.disabled = false;
                input.value = '';
                input.focus();
                return;
            }
            input.value = '';
            closeVaultSecretPrompt();
        } catch (_) {
            requestBody = '';
            const errorNode = activeOverlay && activeOverlay.querySelector('.vault-secret-modal-error');
            if (errorNode) {
                errorNode.textContent = tr('chat.vault_secret_error', 'The secret could not be stored.');
                errorNode.hidden = false;
            }
            submitting = false;
            saveButton.disabled = false;
            cancelButton.disabled = false;
            input.value = '';
            input.focus();
        }
    }

    function showVaultSecretPrompt(payload) {
        if (!payload || !payload.request_id || !payload.vault_key || !payload.prompt) return;
        if (payload.session_id && payload.session_id !== activeSessionId()) return;
        if (activePrompt && activePrompt.request_id === payload.request_id && activeOverlay) return;
        if (activePrompt) {
            cancelPrompt(activePrompt, false);
            closeVaultSecretPrompt();
        }

        activePrompt = {
            session_id: payload.session_id || activeSessionId(),
            request_id: payload.request_id,
            vault_key: payload.vault_key,
            prompt: payload.prompt
        };

        const overlay = document.createElement('div');
        overlay.className = 'question-modal-overlay vault-secret-modal-overlay';
        overlay.setAttribute('role', 'dialog');
        overlay.setAttribute('aria-modal', 'true');

        const panel = document.createElement('form');
        panel.className = 'question-modal-panel vault-secret-modal-panel';

        const title = document.createElement('h2');
        title.className = 'question-modal-title';
        title.textContent = tr('chat.vault_secret_title', 'Store secret securely');

        const prompt = document.createElement('p');
        prompt.className = 'vault-secret-modal-prompt';
        prompt.textContent = activePrompt.prompt;

        const notice = document.createElement('p');
        notice.className = 'vault-secret-modal-notice';
        notice.textContent = tr('chat.vault_secret_notice', 'The agent never sees your entry. It is stored directly in the encrypted Vault.');

        const label = document.createElement('label');
        label.className = 'vault-secret-modal-label';
        label.textContent = tr('chat.vault_secret_key_label', 'Vault key');

        const key = document.createElement('code');
        key.className = 'vault-secret-modal-key';
        key.textContent = activePrompt.vault_key;
        label.appendChild(key);

        const input = document.createElement('input');
        input.className = 'vault-secret-modal-input';
        input.type = 'password';
        input.autocomplete = 'new-password';
        input.required = true;
        input.maxLength = 65536;
        input.setAttribute('aria-label', tr('chat.vault_secret_input_label', 'Secret value'));

        const error = document.createElement('p');
        error.className = 'vault-secret-modal-error';
        error.hidden = true;

        const actions = document.createElement('div');
        actions.className = 'modal-actions';
        const cancel = document.createElement('button');
        cancel.type = 'button';
        cancel.className = 'modal-btn cancel';
        cancel.textContent = tr('chat.vault_secret_cancel', 'Cancel');
        const save = document.createElement('button');
        save.type = 'submit';
        save.className = 'modal-btn confirm';
        save.textContent = tr('chat.vault_secret_save', 'Store in Vault');
        actions.append(cancel, save);

        panel.append(title, prompt, notice, label, input, error, actions);
        overlay.appendChild(panel);
        document.body.appendChild(overlay);
        activeOverlay = overlay;

        cancel.addEventListener('click', async function () {
            const promptToCancel = activePrompt;
            clearSecretInput();
            closeVaultSecretPrompt();
            await cancelPrompt(promptToCancel, false);
        });
        panel.addEventListener('submit', function (event) {
            event.preventDefault();
            submitPrompt(input, save, cancel);
        });
        window.setTimeout(function () { input.focus(); }, 50);
    }

    async function checkPendingVaultSecretPrompt() {
        const sessionId = activeSessionId();
        try {
            const response = await fetch('/api/agent/vault-secret/status?session_id=' + encodeURIComponent(sessionId), {
                credentials: 'same-origin'
            });
            if (!response.ok) return;
            const status = await response.json();
            if (status && status.status === 'pending' && status.prompt) {
                showVaultSecretPrompt(status.prompt);
            } else if (activePrompt && activePrompt.session_id === sessionId) {
                closeVaultSecretPrompt();
            }
        } catch (_) { }
    }

    async function handleSessionChange(sessionId) {
        if (activePrompt && activePrompt.session_id !== sessionId) {
            const promptToCancel = activePrompt;
            closeVaultSecretPrompt();
            await cancelPrompt(promptToCancel, false);
        }
        await checkPendingVaultSecretPrompt();
    }

    function handleVaultSecretAck(payload) {
        if (!payload || !activePrompt) return;
        if (payload.session_id !== activePrompt.session_id || payload.request_id !== activePrompt.request_id) return;
        closeVaultSecretPrompt();
        if (payload.status === 'error' && window.showToast) {
            const message = payload.error_code
                ? tr('chat.vault_secret_error_code', 'The secret could not be stored ({{code}}).').replace('{{code}}', payload.error_code)
                : tr('chat.vault_secret_error', 'The secret could not be stored.');
            window.showToast(message, 'error');
        }
    }

    window.showVaultSecretPrompt = showVaultSecretPrompt;
    window.handleVaultSecretAck = handleVaultSecretAck;
    window.checkPendingVaultSecretPrompt = checkPendingVaultSecretPrompt;
    window.handleVaultSecretSessionChange = handleSessionChange;

    if (window.AuraDisposer) {
        window.AuraDisposer.add(function () {
            if (activePrompt) cancelPrompt(activePrompt, true);
            closeVaultSecretPrompt();
        });
    }
    window.setTimeout(checkPendingVaultSecretPrompt, 0);
})();
