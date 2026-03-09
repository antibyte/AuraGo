// cfg/danger.js — Danger Zone section module

        function renderDangerZoneSection(section) {
            const agentCfg = configData.agent || {};

            const capabilities = [
                {
                    path: 'agent.sudo_enabled',
                    val: agentCfg.sudo_enabled || false,
                    icon: '🔑',
                    title: t('config.danger.sudo.title'),
                    desc: t('config.danger.sudo.desc'),
                    badge: 'execute_sudo'
                },
                {
                    path: 'agent.allow_shell',
                    val: agentCfg.allow_shell === true,
                    icon: '🐚',
                    title: t('config.danger.shell.title'),
                    desc: t('config.danger.shell.desc'),
                    badge: 'execute_shell'
                },
                {
                    path: 'agent.allow_python',
                    val: agentCfg.allow_python === true,
                    icon: '🐍',
                    title: t('config.danger.python.title'),
                    desc: t('config.danger.python.desc'),
                    badge: 'execute_python'
                },
                {
                    path: 'agent.allow_filesystem_write',
                    val: agentCfg.allow_filesystem_write === true,
                    icon: '💾',
                    title: t('config.danger.filesystem.title'),
                    desc: t('config.danger.filesystem.desc'),
                    badge: 'filesystem (write)'
                },
                {
                    path: 'agent.allow_network_requests',
                    val: agentCfg.allow_network_requests === true,
                    icon: '🌐',
                    title: t('config.danger.network.title'),
                    desc: t('config.danger.network.desc'),
                    badge: 'api_request'
                },
                {
                    path: 'agent.allow_remote_shell',
                    val: agentCfg.allow_remote_shell === true,
                    icon: '🖧',
                    title: t('config.danger.remote_shell.title'),
                    desc: t('config.danger.remote_shell.desc'),
                    badge: 'execute_remote_shell'
                },
                {
                    path: 'agent.allow_self_update',
                    val: agentCfg.allow_self_update === true,
                    icon: '🔄',
                    title: t('config.danger.self_update.title'),
                    desc: t('config.danger.self_update.desc'),
                    badge: 'manage_updates'
                },
                {
                    path: 'agent.allow_mcp',
                    val: agentCfg.allow_mcp === true,
                    icon: '🔌',
                    title: t('config.danger.mcp.title'),
                    desc: t('config.danger.mcp.desc'),
                    badge: 'mcp_call'
                }
            ];

            let html = '<div class="cfg-section active">';
            html += '<div class="section-header" style="color:#f87171;">' + section.icon + ' ' + section.label + '</div>';
            html += '<div class="section-desc">' + section.desc + '</div>';

            html += `<div class="danger-banner">
                <strong>⚠️ ${t('config.danger.warning_title')}:</strong>
                ${t('config.danger.warning_desc')}
            </div>`;

            for (const cap of capabilities) {
                const isOn = cap.val;
                const helpKey = cap.path;
                const helpText = (helpTexts[helpKey] || {})[lang] || '';
                html += `<div class="danger-card">
                    <div class="danger-card-header">
                        <div>
                            <div class="danger-card-title">${cap.icon} ${cap.title}</div>
                            <div class="danger-card-desc">${cap.desc}</div>
                            ${helpText ? `<div class="danger-card-desc" style="margin-top:0.3rem;opacity:0.7;">${helpText}</div>` : ''}
                        </div>
                        <div style="display:flex;flex-direction:column;align-items:flex-end;gap:0.4rem;flex-shrink:0;">
                            <span class="danger-card-badge">${cap.badge}</span>
                            <div class="toggle ${isOn ? 'on' : ''}" data-path="${cap.path}" onclick="toggleBool(this)"></div>
                        </div>
                    </div>
                </div>`;
            }

            html += '</div>';
            document.getElementById('content').innerHTML = html;
            attachChangeListeners();
        }
