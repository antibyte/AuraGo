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
        'fb-test-btn': { requiredPaths: ['fritzbox.host', 'fritzbox.username'], credentialPaths: ['fritzbox.password'] },
        'github-test-btn': { credentialPaths: ['github.token'] },
        'grafana-test-btn': { requiredPaths: ['grafana.base_url'], credentialPaths: ['grafana.api_key'] },
        'gw-test-btn': { credentialPaths: ['google_workspace.client_secret'] },
        'hp-test-btn': { requiredPaths: ['homepage.deploy_host', 'homepage.deploy_user'], credentialPaths: [['homepage.deploy_password', 'homepage.deploy_key']] },
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
		'treg-test-btn': { credentialPaths: ['treg.token'] },
        'obsidian-test-btn': { requiredPaths: ['obsidian.host'], credentialPaths: ['obsidian.api_key'] },
        'omniroute-test-btn': { requiredPaths: ['omniroute.mode'] },
        'paperless-test-btn': { requiredPaths: ['paperless_ngx.url'], credentialPaths: ['paperless_ngx.api_token'] },
        'integration-ansible-test-btn': { requiredPaths: ['ansible.url'], credentialPaths: ['ansible.token'] },
        'integration-discord-test-btn': { credentialPaths: ['discord.bot_token'] },
        'integration-frigate-test-btn': { requiredPaths: ['frigate.url'] },
        'integration-home-assistant-test-btn': { requiredPaths: ['home_assistant.url'], credentialPaths: ['home_assistant.access_token'] },
        'integration-proxmox-test-btn': { requiredPaths: ['proxmox.url', 'proxmox.token_id'], credentialPaths: ['proxmox.secret'] },
        'integration-rocketchat-test-btn': { requiredPaths: ['rocketchat.url', 'rocketchat.user_id'], credentialPaths: ['rocketchat.auth_token'] },
        // The S3 renderer registers provider-specific requirements.
        'integration-s3-test-btn': {},
        'integration-telegram-test-btn': { credentialPaths: ['telegram.bot_token'] },
        'sqlconn-test-btn': { requiredSelectors: ['#sqlconn-field-name', '#sqlconn-field-database'] },
        'telnyx-test-btn': { credentialPaths: ['telnyx.api_key'] },
        'truenas-test-btn': { requiredPaths: ['truenas.host'], credentialPaths: ['truenas.api_key'] },
        'ts-api-test-btn': { credentialPaths: ['tailscale.api_key'] },
        'uptime-kuma-test-btn': { requiredPaths: ['uptime_kuma.base_url'], credentialPaths: ['uptime_kuma.api_key'] },
        'vd-cfg-test-btn': { requiredPaths: ['virtual_desktop.workspace_dir'] },
        'vercel-test-btn': { credentialPaths: ['vercel.token'] },
        'video-test-btn': { requiredPaths: ['video_generation.provider'] },
        'webdav-test-btn': { requiredPaths: ['webdav.url', 'webdav.username'], credentialPaths: [['webdav.password', 'webdav.token']] },
        'yepapi-test-btn': { requiredPaths: ['yepapi.provider'] }
    });

    // Switches that ask for confirmation when they are turned on (actions.js guard).
    // The category picks the warning text: delete | destroy | host | write.
    // Modal checkboxes opt in with data-cfg-confirm-enable="<category>" instead.
    const confirmOnEnable = Object.freeze({
        // Tier A: the agent may delete or destroy data.
        'composio.allow_destructive': 'destroy',
        'github.allow_delete': 'delete',
        'here_now.allow_delete': 'delete',
        'network_shares.allow_delete': 'delete',
        'huggingface.allow_delete': 'delete',
        'game_maker.allow_delete': 'delete',
        'jellyfin.allow_destructive': 'destroy',
        'obsidian.allow_destructive': 'destroy',
        'truenas.allow_destructive': 'destroy',
        'rustfs.permissions.allow_delete': 'delete',
        'proxmox.allow_destructive': 'destroy',
        // Tier B: the agent gets host or system rights.
        'agent.sudo_enabled': 'host',
        'agent.sudo_unrestricted': 'host',
        'agent.allow_shell': 'host',
        'agent.allow_python': 'host',
        'agent.allow_unsafe_host_execution': 'host',
        'docker.allow_host_access': 'host',
        'agent.allow_filesystem_write': 'write',
        'agent.allow_remote_shell': 'host',
        'agent.allow_self_update': 'host',
        'agent.allow_package_manager': 'host',
        'agent.allow_unsandboxed_shell': 'host',
        'shell_sandbox.allow_unsafe_fallback': 'host',
        'sandbox.allow_local_python_fallback': 'host',
        'huggingface.allow_job_token_injection': 'host'
    });

    const validationRules = Object.freeze({
        'cloudflare_tunnel.loopback_port': { type: 'number', min: 0, max: 65535, pattern: '^\\d+$' },
        'cloudflare_tunnel.metrics_port': { type: 'number', min: 0, max: 65535, pattern: '^\\d+$' },
        'treg.max_call_cost_micro': { type: 'number', min: 0, max: 1000000000000, required: true },
        'server.port': { type: 'number', min: 1, max: 65535, required: true },
        'auth.session_timeout_hours': { type: 'number', min: 1, max: 8760 },
        'agent.context_window': { type: 'number', min: 0 },
        'circuit_breaker.max_tool_calls': { type: 'number', min: 1 },
        'circuit_breaker.llm_timeout_seconds': { type: 'number', min: 1 },
        // EasyDrag flows limits (cfg/flows.js reads the ranges from here). The server clamps saved
        // values, so changedOnly checks edits only and a hand-written value never blocks other saves.
        'flows.max_parallel_runs': { type: 'number', min: 1, max: 32, changedOnly: true },
        'flows.max_parallel_nodes_per_run': { type: 'number', min: 1, max: 16, changedOnly: true },
        'flows.run_retention_days': { type: 'number', min: 1, max: 365, changedOnly: true },
        'flows.max_runs_per_flow': { type: 'number', min: 10, max: 5000, changedOnly: true }
    });

    // State of a Config section for the sidebar dot (js/config/section_state.js).
    // Without an entry a section is active when "<key>.enabled" is on in the draft.
    //   on:   paths that switch the section on (any one is enough); "set:<path>" means
    //         the value is filled in (a provider, a stored token) instead of a flag.
    //   rule: actionRules id; with every required value and credential present, a
    //         section that is off counts as configured. Only rules that need a
    //         credential are bound, so defaults never look like a setup.
    //   none: core section without an on/off state, it gets no dot.
    const sectionStatePaths = Object.freeze({
        agent: { none: true },
        optimizations: { none: true },
        providers: { none: true },
        llm: { none: true },
        server: { none: true },
        directories: { none: true },
        sqlite: { none: true },
        logging: { none: true },
        tools: { none: true },
        info_tools: { none: true },
        network_tools: { none: true },
        backup_restore: { none: true },
        updates: { none: true },
        prompts_editor: { none: true },
        telephone_agent: { none: true },
        memory_analysis: { none: true },
        danger_zone: { none: true },
        // The device registry lives in the inventory database and has no enabled flag.
        devices: { none: true },
        embeddings: { on: ['set:embeddings.provider'] },
        personality: { on: ['personality.engine'] },
        vision: { on: ['set:vision.provider'] },
        whisper: { on: ['set:whisper.provider'] },
        tts: { on: ['set:tts.provider', 'tts.piper.enabled'] },
        output_compression: { on: ['agent.output_compression.enabled'] },
        web_scraper: { on: ['tools.web_scraper.enabled'] },
        skill_manager: { on: ['tools.skill_manager.enabled'] },
        daemon_skills: { on: ['tools.daemon_skills.enabled'] },
        media_conversion: { on: ['tools.media_conversion.enabled'] },
        video_download: { on: ['tools.video_download.enabled'] },
        document_creator: { on: ['tools.document_creator.enabled'] },
        web_config: { on: ['auth.enabled'] },
        guardian: { on: ['guardian.injection_guard.enabled'] },
        notifications: { on: ['notifications.ntfy.enabled', 'notifications.pushover.enabled'] },
        a2a: { on: ['a2a.server.enabled'] },
        telegram: { on: ['set:telegram.bot_token'], rule: 'integration-telegram-test-btn' },
        adguard: { rule: 'adg-test-btn' },
        agentmail: { rule: 'agentmail-test-btn' },
        ai_gateway: { rule: 'ai-gw-test-btn' },
        composio: { rule: 'composio-test-btn' },
        manus: { rule: 'manus-test-btn' },
        github: { rule: 'github-test-btn' },
        grafana: { rule: 'grafana-test-btn' },
        google_workspace: { rule: 'gw-test-btn' },
        homepage: { rule: 'hp-test-btn' },
        huggingface: { rule: 'huggingface-test-btn' },
        jellyfin: { rule: 'jellyfin-test-btn' },
        koofr: { rule: 'koofr-test-btn' },
        ldap: { rule: 'ldap-test-btn' },
        netlify: { rule: 'nf-test-btn' },
        here_now: { rule: 'hn-test-btn' },
        treg: { rule: 'treg-test-btn' },
        obsidian: { rule: 'obsidian-test-btn' },
        paperless_ngx: { rule: 'paperless-test-btn' },
        ansible: { rule: 'integration-ansible-test-btn' },
        discord: { rule: 'integration-discord-test-btn' },
        home_assistant: { rule: 'integration-home-assistant-test-btn' },
        proxmox: { rule: 'integration-proxmox-test-btn' },
        rocketchat: { rule: 'integration-rocketchat-test-btn' },
        telnyx: { rule: 'telnyx-test-btn' },
        truenas: { rule: 'truenas-test-btn' },
        tailscale: { rule: 'ts-api-test-btn' },
        uptime_kuma: { rule: 'uptime-kuma-test-btn' },
        vercel: { rule: 'vercel-test-btn' },
        webdav: { rule: 'webdav-test-btn' }
    });

    window.AuraConfigCatalog = Object.freeze({
        version: 1,
        actionRules,
        sectionStatePaths,
        validationRules,
        confirmOnEnable,
        // Semantic surfaces declared by the integration renderers. These are exact
        // selectors, never guesses from a field name or DOM nesting depth.
        presentation: Object.freeze({
            a2a: { topics: '.a2a-card', objects: '.a2a-agent-card', headings: '.a2a-card-title' },
            web_config: { markers: '.auth-section-title', topics: '.auth-sec-audit-panel' },
            backup_restore: { topics: '.bck-card', headings: '.bck-card-header', bodies: '.bck-card-body', flows: '.backup-panel' },
            co_agents: { objects: '.ca-card', headings: '.ca-card-header', bodies: '.ca-card-body' },
            danger_zone: { topics: '.danger-card', headings: '.danger-card-header' },
            email: { objects: '.em-card', headings: '.em-card-header' },
            firewall: { topics: '.fw-section' },
            fritzbox: { topics: '.fb-feature-card', headings: '.fb-feature-title', markers: '.cfg-section-title' },
            heartbeat: { topics: '.hb-window-card', headings: '.hb-window-title', flows: '#heartbeatOptionsWrapper' },
            indexing: { topics: '.idx-card, .idx-status-card', headings: '.idx-card-header, .idx-status-header', bodies: '.idx-card-body' },
            mcp: { objects: '.mcp-card', headings: '.mcp-card-header' },
            memory_analysis: { topics: '.ma-block', headings: '.ma-block-header' },
            mqtt: { markers: '.cfg-section-title' },
            onedrive: { markers: '.od-section-title' },
            providers: { objects: '.prov-provider-card, .provider-card' },
            realtime_speech: { topics: '.rs-master-card', objects: '.rs-profile-card', headings: '.rs-profile-head' },
            remote_control: { objects: '.rc-token-card' },
            rules: { topics: '.rules-summary-card, .rules-design-panel, .rules-field-panel, .rules-markdown-panel, .rules-list-panel', headings: '.rules-panel-title, .rules-summary-title, .rules-list-header' },
            sandbox: { topics: '.sb-card', headings: '.sb-card-header', bodies: '.sb-card-body' },
            sip: { topics: '.sip-wizard-shell', headings: '.sip-wizard-title', objects: '.sip-profile-card' },
            sql_connections: { topics: '.sb-card', headings: '.sb-card-header', bodies: '.sb-card-body' },
            telephone_agent: { topics: '.ta-card' },
            tts: { markers: '.tts-subsection-title' },
            webhooks: { objects: '.wh-card', headings: '.wh-card-header', bodies: '.wh-card-body', flows: '.wh-panel', markers: '.wh-subsection-title' }
        }),
        // Fields rendered outside their YAML root belong to their visible Config section.
        searchSections: Object.freeze({
            web_config: ['web_config', 'auth'],
            optimizations: ['agent.optimizer_enabled', 'agent.system_prompt_token_budget', 'agent.adaptive_system_prompt_token_budget', 'agent.context_window', 'agent.memory_compression_char_limit', 'agent.tool_output_limit', 'agent.max_tool_guides', 'agent.core_memory_max_entries', 'agent.core_memory_cap_mode', 'agent.adaptive_tools', 'agent.recovery', 'agent.background_tasks', 'circuit_breaker.max_tool_calls', 'circuit_breaker.llm_timeout_seconds', 'circuit_breaker.maintenance_timeout_minutes', 'circuit_breaker.retry_intervals'],
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
            danger_zone: ['agent.allow_shell', 'agent.allow_python', 'agent.allow_filesystem_write', 'agent.allow_network_requests', 'agent.allow_remote_shell', 'agent.allow_self_update', 'agent.allow_package_manager', 'agent.allow_mcp', 'agent.sudo_enabled', 'docker.allow_host_access']
        }),
        // Only explicitly reviewed tuning fields may be collapsed. Unknown fields stay visible.
        sectionTiers: Object.freeze({
            'server.debug_mode': 'advanced',
            'agent.debug_mode': 'advanced',
            'circuit_breaker.llm_timeout_seconds': 'advanced'
        })
    });
})();
