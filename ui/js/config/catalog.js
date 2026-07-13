// Versioned UX metadata for the opt-in Precision Workspace config surface.
(function () {
    'use strict';

    const actionRules = Object.freeze({
        'adg-test-btn': { requiredPaths: ['adguard.url'], credentialPaths: ['adguard.password'] },
        'agentmail-test-btn': { requiredPaths: ['agentmail.base_url'], credentialPaths: ['agentmail.api_key'] },
        'ai-gw-test-btn': { requiredPaths: ['ai_gateway.account_id', 'ai_gateway.gateway_id'], credentialPaths: ['ai_gateway.token'] },
        'a2a-test-btn': { requiredPaths: ['a2a.server.agent_name'] },
        'ba-test-btn': { requiredPaths: ['browser_automation.url'] },
        'composio-test-btn': { credentialPaths: ['composio.api_key'] },
        'manus-test-btn': { credentialPaths: ['manus.api_key'] },
        'dc-test-btn': { requiredPaths: ['tools.document_creator.gotenberg.url'] },
        'dograh-test-btn': { requiredPaths: ['dograh.mode'] },
        'ea-test-btn': { requiredSelectors: ['#ea-imap-host', '#ea-smtp-host', '#ea-username'] },
        'evomap-test-btn': { requiredPaths: ['evomap.node_id'] },
        'fb-test-btn': { requiredPaths: ['fritzbox.address', 'fritzbox.username'], credentialPaths: ['fritzbox.password'] },
        'github-test-btn': { requiredPaths: ['github.owner'], credentialPaths: ['github.token'] },
        'grafana-test-btn': { requiredPaths: ['grafana.base_url'], credentialPaths: ['grafana.api_key'] },
        'gw-test-btn': { credentialPaths: ['google_workspace.client_secret'] },
        'hp-test-btn': { requiredPaths: ['homepage.workspace_path'] },
        'huggingface-test-btn': { credentialPaths: ['huggingface.token'] },
        'imggen-test-btn': { requiredPaths: ['image_generation.provider'] },
        'jellyfin-test-btn': { requiredPaths: ['jellyfin.host'], credentialPaths: ['jellyfin.api_key'] },
        'koofr-test-btn': { requiredPaths: ['koofr.username'], credentialPaths: ['koofr.app_password'] },
        'ldap-test-btn': { requiredPaths: ['ldap.host', 'ldap.bind_dn'], credentialPaths: ['ldap.bind_password'] },
        'manifest-test-btn': { requiredPaths: ['manifest.mode'] },
        'mc-test-btn': { requiredPaths: [] },
        'mcp-m-test': { requiredSelectors: ['#mcp-m-name'], requiredAnySelectors: [['#mcp-m-command', '#mcp-m-url']] },
        'mqtt-test-btn': { requiredPaths: ['mqtt.broker'] },
        'music-test-btn': { requiredPaths: ['music_generation.provider'] },
        'nf-test-btn': { credentialPaths: ['netlify.token'] },
        'obsidian-test-btn': { requiredPaths: ['obsidian.host'], credentialPaths: ['obsidian.api_key'] },
        'omniroute-test-btn': { requiredPaths: ['omniroute.mode'] },
        'paperless-test-btn': { requiredPaths: ['paperless_ngx.url'], credentialPaths: ['paperless_ngx.token'] },
        'proxmox-test-btn': { requiredPaths: ['proxmox.url', 'proxmox.user'], credentialPaths: ['proxmox.token_secret'] },
        's3-test-btn': { requiredPaths: ['s3.endpoint', 's3.bucket'], credentialPaths: ['s3.secret_access_key'] },
        'sqlconn-test-btn': { requiredSelectors: ['#sqlconn-field-name', '#sqlconn-field-database'] },
        'telnyx-test-btn': { requiredPaths: ['telnyx.phone_number'], credentialPaths: ['telnyx.api_key'] },
        'truenas-test-btn': { requiredPaths: ['truenas.host'], credentialPaths: ['truenas.api_key'] },
        'ts-api-test-btn': { requiredPaths: ['tailscale.tailnet'], credentialPaths: ['tailscale.api_key'] },
        'uptime-kuma-test-btn': { requiredPaths: ['uptime_kuma.base_url'], credentialPaths: ['uptime_kuma.api_key'] },
        'vd-cfg-test-btn': { requiredPaths: ['virtual_desktop.workspace_dir'] },
        'vercel-test-btn': { credentialPaths: ['vercel.token'] },
        'video-test-btn': { requiredPaths: ['video_generation.provider'] },
        'webdav-test-btn': { requiredPaths: ['webdav.url', 'webdav.username'], credentialPaths: ['webdav.password'] },
        'yepapi-test-btn': { requiredPaths: ['yepapi.provider'] }
    });

    const validationRules = Object.freeze({
        'server.port': { type: 'number', min: 1, max: 65535, required: true },
        'web_config.session_timeout_minutes': { type: 'number', min: 1 },
        'agent.context_window': { type: 'number', min: 1024 },
        'circuit_breaker.max_tool_calls': { type: 'number', min: 1 },
        'circuit_breaker.llm_timeout_seconds': { type: 'number', min: 1 }
    });

    window.AuraConfigCatalog = Object.freeze({
        version: 1,
        actionRules,
        validationRules,
        sectionTiers: Object.freeze({}),
        advancedPathPatterns: Object.freeze([
            'debug', 'timeout', 'interval', 'retry', 'proxy', 'headers', 'tls_',
            'container_', 'docker_image', 'advanced', 'max_', 'min_', 'log_'
        ])
    });
})();
