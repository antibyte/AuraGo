(() => {
    const TOOL_ICON_DEFINITIONS = Object.freeze([
        { slot: 0, key: 'thinking', label: 'Thinking', row: 0, col: 0, aliases: Object.freeze(['reasoning', 'thought']) },
        { slot: 1, key: 'coding', label: 'Coding', row: 0, col: 1, aliases: Object.freeze(['code']) },
        { slot: 2, key: 'co_agent_spawn', label: 'Co-agent', row: 0, col: 2, aliases: Object.freeze(['co_agent', 'co_agents']) },
        { slot: 3, key: 'execute_shell', label: 'Shell', row: 0, col: 3, aliases: Object.freeze(['shell', 'bash', 'command']) },
        { slot: 4, key: 'execute_python', label: 'Python', row: 0, col: 4, aliases: Object.freeze(['python']) },
        { slot: 5, key: 'execute_sandbox', label: 'Sandbox', row: 0, col: 5, aliases: Object.freeze(['sandbox']) },
        { slot: 6, key: 'execute_sudo', label: 'Sudo', row: 0, col: 6, aliases: Object.freeze(['sudo']) },
        { slot: 7, key: 'execute_skill', label: 'Run skill', row: 0, col: 7, aliases: Object.freeze(['skill']) },
        { slot: 8, key: 'list_skills', label: 'List skills', row: 0, col: 8, aliases: Object.freeze([]) },
        { slot: 9, key: 'save_tool', label: 'Save tool', row: 0, col: 9, aliases: Object.freeze([]) },
        { slot: 10, key: 'skill_manager', label: 'Skill manager', row: 1, col: 0, aliases: Object.freeze(['skills_engine', 'skill_templates']) },
        { slot: 11, key: 'mcp_call', label: 'MCP', row: 1, col: 1, aliases: Object.freeze(['mcp']) },
        { slot: 12, key: 'ollama', label: 'Ollama', row: 1, col: 2, aliases: Object.freeze([]) },
        { slot: 13, key: 'filesystem', label: 'Filesystem', row: 1, col: 3, aliases: Object.freeze(['files']) },
        { slot: 14, key: 'file_reader_advanced', label: 'Advanced read', row: 1, col: 4, aliases: Object.freeze(['file_reader']) },
        { slot: 15, key: 'smart_file_read', label: 'Smart read', row: 1, col: 5, aliases: Object.freeze([]) },
        { slot: 16, key: 'file_search', label: 'File search', row: 1, col: 6, aliases: Object.freeze([]) },
        { slot: 17, key: 'file_editor', label: 'File editor', row: 1, col: 7, aliases: Object.freeze([]) },
        { slot: 18, key: 'detect_file_type', label: 'File type', row: 1, col: 8, aliases: Object.freeze([]) },
        { slot: 19, key: 'archive', label: 'Archive', row: 1, col: 9, aliases: Object.freeze([]) },
        { slot: 20, key: 'json_editor', label: 'JSON', row: 2, col: 0, aliases: Object.freeze([]) },
        { slot: 21, key: 'yaml_editor', label: 'YAML', row: 2, col: 1, aliases: Object.freeze([]) },
        { slot: 22, key: 'xml_editor', label: 'XML', row: 2, col: 2, aliases: Object.freeze([]) },
        { slot: 23, key: 'toml_editor', label: 'TOML', row: 2, col: 3, aliases: Object.freeze([]) },
        { slot: 24, key: 'text_diff', label: 'Diff', row: 2, col: 4, aliases: Object.freeze([]) },
        { slot: 25, key: 'code_analysis', label: 'Code analysis', row: 2, col: 5, aliases: Object.freeze([]) },
        { slot: 26, key: 'golangci_lint', label: 'Go lint', row: 2, col: 6, aliases: Object.freeze(['golangci']) },
        { slot: 27, key: 'system_metrics', label: 'System metrics', row: 2, col: 7, aliases: Object.freeze(['metrics']) },
        { slot: 28, key: 'process_management', label: 'Processes', row: 2, col: 8, aliases: Object.freeze([]) },
        { slot: 29, key: 'service_manager', label: 'Services', row: 2, col: 9, aliases: Object.freeze([]) },
        { slot: 30, key: 'network_ping', label: 'Ping', row: 3, col: 0, aliases: Object.freeze(['ping']) },
        { slot: 31, key: 'port_scanner', label: 'Ports', row: 3, col: 1, aliases: Object.freeze([]) },
        { slot: 32, key: 'dns_lookup', label: 'DNS', row: 3, col: 2, aliases: Object.freeze([]) },
        { slot: 33, key: 'follow_up', label: 'Follow-up', row: 3, col: 3, aliases: Object.freeze(['wait_for_event']) },
        { slot: 34, key: 'upnp_scan', label: 'UPnP', row: 3, col: 4, aliases: Object.freeze([]) },
        { slot: 35, key: 'certificate_manager', label: 'Certificates', row: 3, col: 5, aliases: Object.freeze([]) },
        { slot: 36, key: 'firewall', label: 'Firewall', row: 3, col: 6, aliases: Object.freeze([]) },
        { slot: 37, key: 'wake_on_lan', label: 'Wake on LAN', row: 3, col: 7, aliases: Object.freeze(['wol']) },
        { slot: 38, key: 'remote_execution', label: 'Remote exec', row: 3, col: 8, aliases: Object.freeze([]) },
        { slot: 39, key: 'remote_control', label: 'Remote control', row: 3, col: 9, aliases: Object.freeze([]) },
        { slot: 40, key: 'ssh_key_manager', label: 'SSH keys', row: 4, col: 0, aliases: Object.freeze([]) },
        { slot: 41, key: 'ansible', label: 'Ansible', row: 4, col: 1, aliases: Object.freeze([]) },
        { slot: 42, key: 'docker', label: 'Docker', row: 4, col: 2, aliases: Object.freeze(['docker_management']) },
        { slot: 43, key: 'proxmox', label: 'Proxmox', row: 4, col: 3, aliases: Object.freeze([]) },
        { slot: 44, key: 'truenas', label: 'TrueNAS', row: 4, col: 4, aliases: Object.freeze([]) },
        { slot: 45, key: 'home_assistant', label: 'Home Assistant', row: 4, col: 5, aliases: Object.freeze(['homeassistant']) },
        { slot: 46, key: 'fritzbox_network', label: 'Fritz network', row: 4, col: 6, aliases: Object.freeze([]) },
        { slot: 47, key: 'fritzbox_smarthome', label: 'Fritz smart home', row: 4, col: 7, aliases: Object.freeze([]) },
        { slot: 48, key: 'fritzbox_telephony', label: 'Fritz telephony', row: 4, col: 8, aliases: Object.freeze([]) },
        { slot: 49, key: 'meshcentral', label: 'MeshCentral', row: 4, col: 9, aliases: Object.freeze([]) },
        { slot: 50, key: 'tailscale', label: 'Tailscale', row: 5, col: 0, aliases: Object.freeze([]) },
        { slot: 51, key: 'cloudflare_tunnel', label: 'Cloudflare tunnel', row: 5, col: 1, aliases: Object.freeze([]) },
        { slot: 52, key: 'adguard', label: 'AdGuard', row: 5, col: 2, aliases: Object.freeze([]) },
        { slot: 53, key: 'uptime_kuma', label: 'Uptime Kuma', row: 5, col: 3, aliases: Object.freeze([]) },
        { slot: 54, key: 'grafana', label: 'Grafana', row: 5, col: 4, aliases: Object.freeze(['dashboard', 'dashboards', 'observability', 'metrics', 'prometheus', 'loki']) },
        { slot: 55, key: 'web_scraper', label: 'Web scraper', row: 5, col: 5, aliases: Object.freeze([]) },
        { slot: 56, key: 'web_capture', label: 'Web capture', row: 5, col: 6, aliases: Object.freeze([]) },
        { slot: 57, key: 'web_performance_audit', label: 'Performance', row: 5, col: 7, aliases: Object.freeze(['web_performance']) },
        { slot: 58, key: 'browser_automation', label: 'Browser', row: 5, col: 8, aliases: Object.freeze(['space_agent', 'virtual_desktop']) },
        { slot: 59, key: 'api_request', label: 'API request', row: 5, col: 9, aliases: Object.freeze(['call_webhook', 'manage_webhooks', 'manage_outgoing_webhooks']) },
        { slot: 60, key: 'brave_search', label: 'Search', row: 6, col: 0, aliases: Object.freeze(['ddg_search']) },
        { slot: 61, key: 'wikipedia_search', label: 'Wikipedia', row: 6, col: 1, aliases: Object.freeze([]) },
        { slot: 62, key: 'github', label: 'GitHub', row: 6, col: 2, aliases: Object.freeze([]) },
        { slot: 63, key: 'netlify', label: 'Netlify', row: 6, col: 3, aliases: Object.freeze([]) },
        { slot: 64, key: 'vercel', label: 'Vercel', row: 6, col: 4, aliases: Object.freeze([]) },
        { slot: 65, key: 's3_storage', label: 'S3 storage', row: 6, col: 5, aliases: Object.freeze([]) },
        { slot: 66, key: 'webdav', label: 'WebDAV', row: 6, col: 6, aliases: Object.freeze([]) },
        { slot: 67, key: 'koofr', label: 'Koofr', row: 6, col: 7, aliases: Object.freeze([]) },
        { slot: 68, key: 'onedrive', label: 'OneDrive', row: 6, col: 8, aliases: Object.freeze([]) },
        { slot: 69, key: 'google_workspace', label: 'Google Workspace', row: 6, col: 9, aliases: Object.freeze([]) },
        { slot: 70, key: 'email', label: 'Email', row: 7, col: 0, aliases: Object.freeze(['fetch_email', 'send_email', 'list_email_accounts']) },
        { slot: 71, key: 'discord', label: 'Discord', row: 7, col: 1, aliases: Object.freeze([]) },
        { slot: 72, key: 'mqtt', label: 'MQTT', row: 7, col: 2, aliases: Object.freeze(['mqtt_publish', 'mqtt_subscribe', 'mqtt_unsubscribe', 'mqtt_get_messages']) },
        { slot: 73, key: 'telnyx', label: 'Telnyx', row: 7, col: 3, aliases: Object.freeze(['telnyx_sms', 'telnyx_call', 'telnyx_manage']) },
        { slot: 74, key: 'send_notification', label: 'Notification', row: 7, col: 4, aliases: Object.freeze([]) },
        { slot: 75, key: 'chromecast', label: 'Chromecast', row: 7, col: 5, aliases: Object.freeze([]) },
        { slot: 76, key: 'jellyfin', label: 'Jellyfin', row: 7, col: 6, aliases: Object.freeze([]) },
        { slot: 77, key: 'media_registry', label: 'Media registry', row: 7, col: 7, aliases: Object.freeze([]) },
        { slot: 78, key: 'media_conversion', label: 'Media conversion', row: 7, col: 8, aliases: Object.freeze([]) },
        { slot: 79, key: 'send_image', label: 'Send image', row: 7, col: 9, aliases: Object.freeze([]) },
        { slot: 80, key: 'send_video', label: 'Send video', row: 8, col: 0, aliases: Object.freeze(['send_youtube_video']) },
        { slot: 81, key: 'send_audio', label: 'Send audio', row: 8, col: 1, aliases: Object.freeze([]) },
        { slot: 82, key: 'send_document', label: 'Send document', row: 8, col: 2, aliases: Object.freeze([]) },
        { slot: 83, key: 'analyze_image', label: 'Analyze image', row: 8, col: 3, aliases: Object.freeze([]) },
        { slot: 84, key: 'transcribe_audio', label: 'Transcribe audio', row: 8, col: 4, aliases: Object.freeze([]) },
        { slot: 85, key: 'tts', label: 'Text to speech', row: 8, col: 5, aliases: Object.freeze(['tts_minimax']) },
        { slot: 86, key: 'generate_image', label: 'Generate image', row: 8, col: 6, aliases: Object.freeze([]) },
        { slot: 87, key: 'generate_video', label: 'Generate video', row: 8, col: 7, aliases: Object.freeze([]) },
        { slot: 88, key: 'generate_music', label: 'Generate music', row: 8, col: 8, aliases: Object.freeze([]) },
        { slot: 89, key: 'document_creator', label: 'Documents', row: 8, col: 9, aliases: Object.freeze([]) },
        { slot: 90, key: 'paperless', label: 'Paperless', row: 9, col: 0, aliases: Object.freeze([]) },
        { slot: 91, key: 'obsidian', label: 'Obsidian', row: 9, col: 1, aliases: Object.freeze([]) },
        { slot: 92, key: 'knowledge_graph', label: 'Knowledge graph', row: 9, col: 2, aliases: Object.freeze([]) },
        { slot: 93, key: 'query_memory', label: 'Query memory', row: 9, col: 3, aliases: Object.freeze([]) },
        { slot: 94, key: 'manage_memory', label: 'Memory', row: 9, col: 4, aliases: Object.freeze(['core_memory', 'remember', 'optimize_memory', 'smart_memory']) },
        { slot: 95, key: 'manage_notes', label: 'Notes', row: 9, col: 5, aliases: Object.freeze(['manage_todos', 'manage_appointments', 'manage_plan', 'manage_missions']) },
        { slot: 96, key: 'manage_journal', label: 'Journal', row: 9, col: 6, aliases: Object.freeze([]) },
        { slot: 97, key: 'secrets_vault', label: 'Secrets vault', row: 9, col: 7, aliases: Object.freeze([]) },
        { slot: 98, key: 'cron_scheduler', label: 'Scheduler', row: 9, col: 8, aliases: Object.freeze(['manage_schedule']) },
        { slot: 99, key: 'generic_tool', label: 'Generic tool', row: 9, col: 9, aliases: Object.freeze(['_default', 'generic']) },
        { slot: 101, key: 'question_user', label: 'Question', row: 3, col: 3, aliases: Object.freeze([]) },
    ]);

    const DEFAULT_TOOL_ICON_KEY = 'generic_tool';
    const definitionsByKey = new Map();
    const aliasesByKey = new Map();

    function normalizeToolName(toolName) {
        return String(toolName || DEFAULT_TOOL_ICON_KEY)
            .trim()
            .toLowerCase()
            .replace(/[^a-z0-9]+/g, '_')
            .replace(/^_+|_+$/g, '') || DEFAULT_TOOL_ICON_KEY;
    }

    TOOL_ICON_DEFINITIONS.forEach((definition) => {
        definitionsByKey.set(definition.key, definition);
        definition.aliases.forEach((alias) => aliasesByKey.set(normalizeToolName(alias), definition.key));
    });

    function getDefinition(toolName) {
        const normalized = normalizeToolName(toolName);
        const canonical = aliasesByKey.get(normalized) || normalized;
        return definitionsByKey.get(canonical) || definitionsByKey.get(DEFAULT_TOOL_ICON_KEY);
    }

    function applyIcon(el, toolName) {
        if (!el) return getDefinition(toolName);
        const definition = getDefinition(toolName);
        el.classList.add('tool-icon-sprite');
        el.dataset.toolIcon = definition.key;
        el.title = definition.label;
        el.style.setProperty('--tool-icon-position-x', `${definition.col * (100 / 9)}%`);
        el.style.setProperty('--tool-icon-position-y', `${definition.row * (100 / 9)}%`);
        return definition;
    }

    function createIcon(toolName, className = '') {
        const el = document.createElement('span');
        el.className = ['tool-icon-sprite', className].filter(Boolean).join(' ');
        el.setAttribute('aria-hidden', 'true');
        applyIcon(el, toolName);
        return el;
    }

    window.AuraToolIcons = {
        definitions: TOOL_ICON_DEFINITIONS,
        normalizeToolName,
        getDefinition,
        applyIcon,
        createIcon,
    };
})();
