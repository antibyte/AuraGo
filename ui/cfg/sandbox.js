// cfg/sandbox.js — Sandbox section module

let sandboxStatusCache = null;

        async function renderSandboxSection(section) {
            // Lazy-load sandbox status on first render
            if (sandboxStatusCache === null) {
                try {
                    const resp = await fetch('/api/sandbox/status');
                    sandboxStatusCache = resp.ok ? await resp.json() : {};
                } catch (_) { sandboxStatusCache = {}; }
            }
            const sbCfg = configData.sandbox || {};
            const sbEnabled = sbCfg.enabled === true;
            const st = sandboxStatusCache || {};

            let html = `<div class="cfg-section active">
                <div class="section-header">${section.icon} ${section.label}</div>
                <div class="section-desc">${section.desc}</div>`;

            // Docker socket unavailable banner
            const sbBanner = featureUnavailableBanner('sandbox');
            if (sbBanner) html += sbBanner;
            const sbBlocked = !!(runtimeData.features && runtimeData.features.sandbox && !runtimeData.features.sandbox.available);

            if (sbBlocked) html += '<div class="feature-unavailable-fields">';

            // Status banner
            if (sbEnabled && st.ready) {
                const langList = (st.languages || []).join(', ') || 'python';
                html += `<div class="wh-notice" style="border-color:var(--success);background:rgba(34,197,94,0.06);">
                    <span>✅</span>
                    <div>
                        <strong>${t('config.sandbox.active')}</strong><br>
                        <small>Backend: ${escapeAttr(st.backend || 'docker')} · ${t('config.sandbox.languages')}: ${escapeAttr(langList)}</small>
                    </div>
                </div>`;
            } else if (sbEnabled && !st.ready) {
                html += `<div class="wh-notice" style="border-color:var(--warning);background:rgba(234,179,8,0.06);">
                    <span>⚠️</span>
                    <div>
                        <strong>${t('config.sandbox.not_ready')}</strong><br>
                        <small>${escapeAttr(st.error || t('config.sandbox.unknown_error'))}</small>
                    </div>
                </div>`;
            }

            // Enabled toggle
            html += `<div style="display:flex;align-items:center;gap:0.8rem;margin-bottom:1rem;padding:0.6rem 1rem;border-radius:8px;background:var(--bg-tertiary);">
                <span style="font-size:0.85rem;color:var(--text-secondary);">${t('config.sandbox.enabled_label')}</span>
                <div class="toggle ${sbEnabled ? 'on' : ''}" data-path="sandbox.enabled" onclick="toggleBool(this)"></div>
            </div>`;

            if (!sbEnabled) {
                html += `<div class="wh-notice">
                    <span>📦</span>
                    <div>
                        <strong>${t('config.sandbox.disabled_notice')}</strong><br>
                        <small>${t('config.sandbox.disabled_desc')}</small>
                    </div>
                </div>`;
            }

            // Config fields
            html += `<div style="display:grid;grid-template-columns:1fr 1fr;gap:0.8rem 1.2rem;margin-top:1rem;">`;

            // Backend
            html += `<label style="display:block;">
                <span style="font-size:0.78rem;color:var(--text-secondary);">Backend</span>
                <select class="cfg-input" data-path="sandbox.backend" style="width:100%;margin-top:0.2rem;" onchange="setNestedValue(configData,'sandbox.backend',this.value);setDirty(true)">
                    <option value="docker" ${sbCfg.backend === 'docker' || !sbCfg.backend ? 'selected' : ''}>Docker</option>
                    <option value="podman" ${sbCfg.backend === 'podman' ? 'selected' : ''}>Podman</option>
                </select>
            </label>`;

            // Image
            html += `<label style="display:block;">
                <span style="font-size:0.78rem;color:var(--text-secondary);">Container Image</span>
                <input class="cfg-input" data-path="sandbox.image" value="${escapeAttr(sbCfg.image || 'python:3.11-slim')}" style="width:100%;margin-top:0.2rem;"
                    onchange="setNestedValue(configData,'sandbox.image',this.value);setDirty(true)">
            </label>`;

            // Docker Host
            html += `<label style="display:block;">
                <span style="font-size:0.78rem;color:var(--text-secondary);">Docker Host <small style="color:var(--text-tertiary);">(${t('config.sandbox.empty_auto')})</small></span>
                <input class="cfg-input" data-path="sandbox.docker_host" value="${escapeAttr(sbCfg.docker_host || '')}" placeholder="unix:///var/run/docker.sock" style="width:100%;margin-top:0.2rem;"
                    onchange="setNestedValue(configData,'sandbox.docker_host',this.value);setDirty(true)">
            </label>`;

            // Timeout
            html += `<label style="display:block;">
                <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.sandbox.timeout_label_full')}</span>
                <input type="number" class="cfg-input" data-path="sandbox.timeout_seconds" value="${sbCfg.timeout_seconds || 30}" min="5" max="300" style="width:100%;margin-top:0.2rem;"
                    onchange="setNestedValue(configData,'sandbox.timeout_seconds',parseInt(this.value)||30);setDirty(true)">
            </label>`;

            // Pool Size
            html += `<label style="display:block;">
                <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.sandbox.pool_size_label')} <small style="color:var(--text-tertiary);">(0 = ${t('config.sandbox.no_pooling')})</small></span>
                <input type="number" class="cfg-input" data-path="sandbox.pool_size" value="${sbCfg.pool_size || 0}" min="0" max="20" style="width:100%;margin-top:0.2rem;"
                    onchange="setNestedValue(configData,'sandbox.pool_size',parseInt(this.value)||0);setDirty(true)">
            </label>`;

            html += `</div>`;

            // Toggle row for boolean options
            html += `<div style="display:grid;grid-template-columns:1fr 1fr;gap:0.6rem 1.2rem;margin-top:1rem;">`;

            // Auto Install
            html += `<div style="display:flex;align-items:center;gap:0.6rem;padding:0.5rem 0;">
                <div class="toggle ${sbCfg.auto_install === true || sbCfg.auto_install === undefined ? 'on' : ''}" data-path="sandbox.auto_install" onclick="toggleBool(this)"></div>
                <span style="font-size:0.82rem;color:var(--text-secondary);">Auto-Install llm-sandbox</span>
            </div>`;

            // Network Enabled
            html += `<div style="display:flex;align-items:center;gap:0.6rem;padding:0.5rem 0;">
                <div class="toggle ${sbCfg.network_enabled ? 'on' : ''}" data-path="sandbox.network_enabled" onclick="toggleBool(this)"></div>
                <span style="font-size:0.82rem;color:var(--text-secondary);">${t('config.sandbox.network_label')}</span>
            </div>`;

            // Keep Alive
            html += `<div style="display:flex;align-items:center;gap:0.6rem;padding:0.5rem 0;">
                <div class="toggle ${sbCfg.keep_alive ? 'on' : ''}" data-path="sandbox.keep_alive" onclick="toggleBool(this)"></div>
                <span style="font-size:0.82rem;color:var(--text-secondary);">${t('config.sandbox.keep_alive_label')}</span>
            </div>`;

            html += `</div>`;

            // Docker/Python status info
            if (st.docker_available !== undefined) {
                html += `<div style="margin-top:1.2rem;padding:0.8rem 1rem;border-radius:8px;background:var(--bg-tertiary);font-size:0.8rem;">
                    <div style="font-weight:600;margin-bottom:0.4rem;">${t('config.sandbox.system_status')}</div>
                    <div style="display:grid;grid-template-columns:1fr 1fr;gap:0.3rem 1rem;">
                        <div>${st.docker_available ? '✅' : '❌'} Docker</div>
                        <div>${st.python_available ? '✅' : '❌'} Python</div>
                        <div>${st.package_installed ? '✅' : '❌'} llm-sandbox</div>
                        <div>${st.ready ? '✅' : '❌'} ${t('config.sandbox.ready')}</div>
                    </div>
                </div>`;
            }

            if (sbBlocked) html += '</div>'; // End feature-unavailable-fields
            html += `</div>`;
            document.getElementById('content').innerHTML = html;
            attachChangeListeners();
        }
