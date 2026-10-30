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

    function ansibleSidecarReady() {
        const state = window.AuraConfigState;
        if (!state) return true;
        // The button sends the draft, so the draft decides (a stored secret reads as the mask).
        if (state.get('ansible.mode') === 'local') return true;
        return ['ansible.url', 'ansible.token'].every(path => {
            const value = state.get(path);
            return value != null && String(value).trim() !== '';
        });
    }

    function entryFor(section) {
        const entry = actions.find(action => action.section === section) || null;
        if (section === 's3' && window.AuraConfigState?.get('s3.provider', { saved: true }) === 'rustfs') {
            return Object.assign({}, entry, { requiredPaths: [], credentialPaths: [] });
        }
        // URL and token only matter for the sidecar mode; the local mode calls the ansible CLI directly.
        // The check is a function so a saved mode change applies without re-rendering the section.
        if (section === 'ansible') {
            return Object.assign({}, entry, { requiredPaths: [], credentialPaths: [], credentialReady: ansibleSidecarReady });
        }
        return entry;
    }

    function render(section) {
        const entry = entryFor(section);
        if (!entry) return '';
        return `
        <div class="field-group cfg-integration-test" data-integration-test="${entry.section}">
            <div class="field-group-title">${text('config.integration_test.title', 'Connection test')}</div>
            <div class="field-help">${text('config.integration_test.description', 'Uses the saved configuration and performs a read-only check.')}</div>
            <div class="cfg-actions-row">
                <button type="button" class="btn-save adg-test-btn" id="${entry.buttonId}" data-config-action-draft="true">${text('config.integration_test.button', 'Test connection')}</button>
                ${cfgActionResult.html(entry.buttonId + '-result')}
            </div>
        </div>`;
    }

    // Sends the values typed in (the draft) so a test works before saving; the
    // server falls back to the stored secret when a field is masked or empty.
    function request(entry) {
        const feedback = window.AuraConfigFeedback;
        const draft = feedback ? feedback.draftPatch([entry.section]) : {};
        return fetch(entry.endpoint, {
            method: 'POST',
            credentials: 'same-origin',
            headers: { 'Accept': 'application/json', 'Content-Type': 'application/json' },
            body: JSON.stringify({ draft: draft })
        });
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
            credentialReady: entry.credentialReady,
            run: () => request(entry),
            result: {
                target: result,
                pending: () => text('config.integration_test.testing', 'Testing connection...'),
                success: () => text('config.integration_test.success', 'Connection successful.'),
                failure: () => text('config.integration_test.failure', 'Connection test failed.'),
                successDetail: true
            }
        });
    }

    window.AuraConfigIntegrationActions = Object.freeze({
        entries: actions,
        render,
        bind
    });
})();
