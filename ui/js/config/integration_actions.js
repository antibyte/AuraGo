// Generic read-only connection tests for schema-rendered configuration sections.
(function () {
    'use strict';

    const actions = Object.freeze([
        {
            section: 'telegram',
            buttonId: 'integration-telegram-test-btn',
            endpoint: '/api/telegram/test-connection',
            requiredPaths: [],
            credentialPaths: ['telegram.bot_token']
        },
        {
            section: 'discord',
            buttonId: 'integration-discord-test-btn',
            endpoint: '/api/discord/test',
            requiredPaths: [],
            credentialPaths: ['discord.bot_token']
        },
        {
            section: 'rocketchat',
            buttonId: 'integration-rocketchat-test-btn',
            endpoint: '/api/rocketchat/test',
            requiredPaths: ['rocketchat.url', 'rocketchat.user_id'],
            credentialPaths: ['rocketchat.auth_token']
        },
        {
            section: 'home_assistant',
            buttonId: 'integration-home-assistant-test-btn',
            endpoint: '/api/home-assistant/test',
            requiredPaths: ['home_assistant.url'],
            credentialPaths: ['home_assistant.access_token']
        },
        {
            section: 'proxmox',
            buttonId: 'integration-proxmox-test-btn',
            endpoint: '/api/proxmox/test',
            requiredPaths: ['proxmox.url', 'proxmox.token_id'],
            credentialPaths: ['proxmox.secret']
        },
        {
            section: 's3',
            buttonId: 'integration-s3-test-btn',
            endpoint: '/api/s3/test',
            requiredPaths: ['s3.bucket'],
            credentialPaths: ['s3.access_key', 's3.secret_key']
        },
        {
            section: 'frigate',
            buttonId: 'integration-frigate-test-btn',
            endpoint: '/api/frigate/test',
            requiredPaths: ['frigate.url'],
            credentialPaths: []
        },
        {
            section: 'ansible',
            buttonId: 'integration-ansible-test-btn',
            endpoint: '/api/ansible/test',
            requiredPaths: ['ansible.url'],
            credentialPaths: ['ansible.token']
        }
    ]);

    function text(key, fallback) {
        if (typeof t === 'function') {
            const value = t(key);
            if (value && value !== key) return value;
        }
        return fallback;
    }

    function entryFor(section) {
        return actions.find(action => action.section === section) || null;
    }

    function render(section) {
        const entry = entryFor(section);
        if (!entry) return '';
        return `
        <div class="field-group cfg-integration-test" data-integration-test="${entry.section}">
            <div class="field-group-title">${text('config.integration_test.title', 'Connection test')}</div>
            <div class="field-help">${text('config.integration_test.description', 'Uses the saved configuration and performs a read-only check.')}</div>
            <div class="cfg-actions-row">
                <button type="button" class="btn-save adg-test-btn" id="${entry.buttonId}">${text('config.integration_test.button', 'Test connection')}</button>
                <span id="${entry.buttonId}-result" class="adg-test-result" role="status" aria-live="polite"></span>
            </div>
        </div>`;
    }

    function setResult(result, state, message) {
        if (!result) return;
        result.className = 'adg-test-result' + (state ? ' ' + state : '');
        result.textContent = message || '';
    }

    async function run(entry, result) {
        setResult(result, '', text('config.integration_test.testing', 'Testing connection…'));
        try {
            const response = await fetch(entry.endpoint, {
                method: 'POST',
                credentials: 'same-origin',
                headers: { 'Accept': 'application/json', 'Content-Type': 'application/json' },
                body: '{}'
            });
            let payload = {};
            try { payload = await response.json(); } catch (_) { /* use the HTTP status below */ }
            const ok = response.ok && payload.status !== 'error';
            const fallback = ok
                ? text('config.integration_test.success', 'Connection successful.')
                : text('config.integration_test.failure', 'Connection test failed.');
            const detail = typeof payload.message === 'string' ? payload.message.trim() : '';
            setResult(result, ok ? 'is-success' : 'is-danger', detail ? fallback + ' ' + detail : fallback);
        } catch (_) {
            setResult(result, 'is-danger', text('config.integration_test.failure', 'Connection test failed.'));
        }
    }

    function bind(section) {
        const entry = entryFor(section);
        if (!entry) return;
        const button = document.getElementById(entry.buttonId);
        const result = document.getElementById(entry.buttonId + '-result');
        if (!button || !result) return;
        const controller = window.AuraConfigActions;
        if (!controller || typeof controller.register !== 'function') return;
        controller.register(entry.buttonId, {
            element: button,
            requiresSaved: true,
            requiredPaths: entry.requiredPaths,
            credentialPaths: entry.credentialPaths,
            run: () => run(entry, result)
        });
    }

    window.AuraConfigIntegrationActions = Object.freeze({
        entries: actions,
        render,
        bind
    });
})();
