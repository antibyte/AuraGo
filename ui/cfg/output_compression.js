// cfg/output_compression.js — Output Compression section module

        function renderOutputCompressionSection(section) {
            const compCfg = (configData.agent && configData.agent.output_compression) || {};
            const enabled = compCfg.enabled !== false;

            let html = `<div class="cfg-section active">
                <div class="section-header">${section.icon} ${section.label}</div>
                <div class="section-desc">${section.desc}</div>`;

            // Master toggle
            html += `<div class="cfg-toggle-row-highlight">
                <span class="cfg-toggle-label">${t('config.output_compression.enabled_label')}</span>
                <div class="toggle ${enabled ? 'on' : ''}" data-path="agent.output_compression.enabled" onclick="toggleBool(this);renderOutputCompressionSection(_currentSection)"></div>
            </div>`;

            if (!enabled) {
                html += `<div class="wh-notice">
                    <span>🗜️</span>
                    <div>
                        <strong>${t('config.output_compression.disabled_notice')}</strong><br>
                        <small>${t('config.output_compression.disabled_desc')}</small>
                    </div>
                </div>`;
                html += '</div>';
                document.getElementById('content').innerHTML = html;
                return;
            }

            // Info banner
            html += `<div class="wh-notice" style="border-left-color:var(--accent);">
                <span>📊</span>
                <div>
                    <strong>${t('config.output_compression.info_title')}</strong><br>
                    <small>${t('config.output_compression.info_desc')}</small>
                </div>
            </div>`;

            // Min chars
            html += `<div class="field-group">
                <div class="field-label">${t('config.output_compression.min_chars_label')}</div>
                <div class="field-help">${t('config.output_compression.min_chars_desc')}</div>
                <input class="field-input" type="number" min="0" max="100000" step="100" data-path="agent.output_compression.min_chars"
                    value="${compCfg.min_chars || 500}"
                    onchange="setNestedValue(configData,'agent.output_compression.min_chars',parseInt(this.value)||500);setDirty(true)">
            </div>`;

            // Preserve errors toggle
            const preserveErrors = compCfg.preserve_errors !== false;
            html += `<div class="cfg-toggle-row">
                <div>
                    <span class="cfg-toggle-label">${t('config.output_compression.preserve_errors_label')}</span>
                    <div class="field-help" style="margin-top:2px">${t('config.output_compression.preserve_errors_desc')}</div>
                </div>
                <div class="toggle ${preserveErrors ? 'on' : ''}" data-path="agent.output_compression.preserve_errors" onclick="toggleBool(this)"></div>
            </div>`;

            // Sub-toggles header
            html += `<div class="field-group-title" style="margin-top:1rem">${t('config.output_compression.filters_title')}</div>
                <div class="field-group-desc">${t('config.output_compression.filters_desc')}</div>`;

            // Shell compression
            const shellOn = compCfg.shell_compression !== false;
            html += `<div class="cfg-toggle-row">
                <div>
                    <span class="cfg-toggle-label">🐚 ${t('config.output_compression.shell_label')}</span>
                    <div class="field-help" style="margin-top:2px">${t('config.output_compression.shell_desc')}</div>
                </div>
                <div class="toggle ${shellOn ? 'on' : ''}" data-path="agent.output_compression.shell_compression" onclick="toggleBool(this)"></div>
            </div>`;

            // Python compression
            const pythonOn = compCfg.python_compression !== false;
            html += `<div class="cfg-toggle-row">
                <div>
                    <span class="cfg-toggle-label">🐍 ${t('config.output_compression.python_label')}</span>
                    <div class="field-help" style="margin-top:2px">${t('config.output_compression.python_desc')}</div>
                </div>
                <div class="toggle ${pythonOn ? 'on' : ''}" data-path="agent.output_compression.python_compression" onclick="toggleBool(this)"></div>
            </div>`;

            // API compression
            const apiOn = compCfg.api_compression !== false;
            html += `<div class="cfg-toggle-row">
                <div>
                    <span class="cfg-toggle-label">🔌 ${t('config.output_compression.api_label')}</span>
                    <div class="field-help" style="margin-top:2px">${t('config.output_compression.api_desc')}</div>
                </div>
                <div class="toggle ${apiOn ? 'on' : ''}" data-path="agent.output_compression.api_compression" onclick="toggleBool(this)"></div>
            </div>`;

            // Relationship note
            html += `<div class="wh-notice" style="margin-top:1rem;border-left-color:var(--text-secondary);">
                <span>💡</span>
                <div>
                    <small>${t('config.output_compression.note_limit')}</small>
                </div>
            </div>`;

            html += '</div>';
            document.getElementById('content').innerHTML = html;
        }
