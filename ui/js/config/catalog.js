// Versioned UX metadata for the opt-in Precision Workspace config surface.
(function () {
    'use strict';

    const actionRules = Object.freeze({
        'adg-test-btn': { requiredPaths: ['adguard.url'], credentialPaths: ['adguard.password'] },
        'agentmail-test-btn': { requiredPaths: ['agentmail.base_url'], credentialPaths: ['agentmail.api_key'] },
        'ai-gw-test-btn': { requiredPaths: ['ai_gateway.provider'] },
        'ba-test-btn': { requiredPaths: ['browser_automation.base_url'] },
        'composio-test-btn': { credentialPaths: ['composio.api_key'] },
        'fb-test-btn': { requiredPaths: ['fritzbox.address', 'fritzbox.username'], credentialPaths: ['fritzbox.password'] },
        'github-test-btn': { requiredPaths: ['github.owner'], credentialPaths: ['github.token'] },
        'grafana-test-btn': { requiredPaths: ['grafana.url'], credentialPaths: ['grafana.api_key'] },
        'gw-test-btn': { credentialPaths: ['google_workspace.client_secret'] },
        'hp-test-btn': { requiredPaths: ['homepage.workspace_path'] },
        'huggingface-test-btn': { credentialPaths: ['huggingface.token'] },
        'jellyfin-test-btn': { requiredPaths: ['jellyfin.url'], credentialPaths: ['jellyfin.api_key'] },
        'koofr-test-btn': { requiredPaths: ['koofr.username'], credentialPaths: ['koofr.password'] },
        'ldap-test-btn': { requiredPaths: ['ldap.url', 'ldap.bind_dn'], credentialPaths: ['ldap.bind_password'] },
        'mqtt-test-btn': { requiredPaths: ['mqtt.broker'] },
        'nf-test-btn': { credentialPaths: ['netlify.token'] },
        'obsidian-test-btn': { requiredPaths: ['obsidian.vault_path'] },
        'paperless-test-btn': { requiredPaths: ['paperless_ngx.url'], credentialPaths: ['paperless_ngx.token'] },
        'proxmox-test-btn': { requiredPaths: ['proxmox.url', 'proxmox.user'], credentialPaths: ['proxmox.token_secret'] },
        's3-test-btn': { requiredPaths: ['s3.endpoint', 's3.bucket'], credentialPaths: ['s3.secret_access_key'] },
        'truenas-test-btn': { requiredPaths: ['truenas.url'], credentialPaths: ['truenas.api_key'] },
        'uptime-kuma-test-btn': { requiredPaths: ['uptime_kuma.url'], credentialPaths: ['uptime_kuma.api_key'] },
        'vercel-test-btn': { credentialPaths: ['vercel.token'] },
        'webdav-test-btn': { requiredPaths: ['webdav.url', 'webdav.username'], credentialPaths: ['webdav.password'] }
    });

    window.AuraConfigCatalog = Object.freeze({
        version: 1,
        actionRules,
        sectionTiers: Object.freeze({})
    });
})();
