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
        'cyd-test-btn': { requiredPaths: ['cyd.enabled'] },
        'music-test-btn': { requiredPaths: ['music_generation.provider'] },
		'nf-test-btn': { credentialPaths: ['netlify.token'] },
		'hn-test-btn': { credentialPaths: ['here_now.api_key'] },
        'obsidian-test-btn': { requiredPaths: ['obsidian.host'], credentialPaths: ['obsidian.api_key'] },
        'omniroute-test-btn': { requiredPaths: ['omniroute.mode'] },
        'paperless-test-btn': { requiredPaths: ['paperless_ngx.url'], credentialPaths: ['paperless_ngx.token'] },
        'integration-ansible-test-btn': { requiredPaths: ['ansible.url'], credentialPaths: ['ansible.token'] },
        'integration-discord-test-btn': { credentialPaths: ['discord.bot_token'] },
        'integration-frigate-test-btn': { requiredPaths: ['frigate.url'] },
        'integration-home-assistant-test-btn': { requiredPaths: ['home_assistant.url'], credentialPaths: ['home_assistant.access_token'] },
        'integration-proxmox-test-btn': { requiredPaths: ['proxmox.url', 'proxmox.token_id'], credentialPaths: ['proxmox.secret'] },
        'integration-rocketchat-test-btn': { requiredPaths: ['rocketchat.url', 'rocketchat.user_id'], credentialPaths: ['rocketchat.auth_token'] },
        'integration-s3-test-btn': { requiredPaths: ['s3.bucket'], credentialPaths: ['s3.access_key', 's3.secret_key'] },
        'integration-telegram-test-btn': { credentialPaths: ['telegram.bot_token'] },
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
        'agent.context_window': { type: 'number', min: 0 },
        'circuit_breaker.max_tool_calls': { type: 'number', min: 1 },
        'circuit_breaker.llm_timeout_seconds': { type: 'number', min: 1 }
    });

    window.AuraConfigCatalog = Object.freeze({
        version: 1,
        actionRules,
        validationRules,
        // Fields rendered outside their YAML root belong to their visible Config section.
        searchSections: Object.freeze({
            optimizations: ['agent.optimizer_enabled', 'agent.system_prompt_token_budget', 'agent.adaptive_system_prompt_token_budget', 'agent.context_window', 'agent.memory_compression_char_limit', 'agent.tool_output_limit', 'agent.discover_tools_snapshot_ttl_minutes', 'agent.max_tool_guides', 'agent.core_memory_max_entries', 'agent.core_memory_cap_mode', 'agent.adaptive_tools', 'agent.recovery', 'agent.background_tasks', 'circuit_breaker.max_tool_calls', 'circuit_breaker.llm_timeout_seconds', 'circuit_breaker.maintenance_timeout_minutes', 'circuit_breaker.retry_intervals'],
            info_tools: ['tools.wikipedia', 'tools.ddg_search', 'tools.pdf_extractor'],
            network_tools: ['tools.wol', 'tools.stop_process', 'tools.network_ping', 'tools.network_scan', 'tools.web_capture', 'tools.form_automation', 'tools.upnp_scan'],
            web_scraper: ['tools.web_scraper'],
            browser_automation: ['tools.browser_automation'],
            virtual_desktop: ['tools.virtual_desktop'],
            media_conversion: ['tools.media_conversion'],
            video_download: ['tools.video_download', 'tools.send_youtube_video'],
            document_creator: ['tools.document_creator'],
            skill_manager: ['tools.skill_manager', 'tools.python_tool_bridge'],
            daemon_skills: ['tools.daemon_skills'],
            output_compression: ['agent.output_compression'],
            danger_zone: ['agent.allow_shell', 'agent.allow_python', 'agent.allow_filesystem_write', 'agent.allow_network_requests', 'agent.allow_remote_shell', 'agent.allow_self_update', 'agent.allow_package_manager', 'agent.allow_mcp', 'agent.sudo_enabled']
        }),
        // Only explicitly reviewed tuning fields may be collapsed. Unknown fields stay visible.
        sectionTiers: Object.freeze({
            'server.debug_mode': 'advanced',
            'agent.debug_mode': 'advanced',
            'web_config.session_timeout_minutes': 'advanced',
            'circuit_breaker.llm_timeout_seconds': 'advanced'
        })
    });
})();
