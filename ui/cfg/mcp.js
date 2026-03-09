// cfg/mcp.js — MCP Servers section module

let mcpServersCache = null;

        async function renderMCPSection(section) {
            // Lazy-load MCP servers on first render
            if (mcpServersCache === null) {
                try {
                    const resp = await fetch('/api/mcp-servers');
                    mcpServersCache = resp.ok ? await resp.json() : [];
                } catch (_) { mcpServersCache = []; }
            }
            const mcpEnabled = configData.mcp && configData.mcp.enabled;
            const allowMcp = configData.agent && configData.agent.allow_mcp;

            let html = `<div class="cfg-section active">
                <div class="section-header">${section.icon} ${section.label}</div>
                <div class="section-desc">${section.desc}</div>`;

            if (!allowMcp) {
                html += `<div class="wh-notice" style="border-color:var(--danger);">
                    <span>🔒</span>
                    <div>
                        <strong>${t('config.mcp.locked_notice')}</strong><br>
                        <small>${t('config.mcp.locked_desc')}</small>
                    </div>
                </div></div>`;
                document.getElementById('content').innerHTML = html;
                return;
            }

            // Enabled toggle
            html += `<div style="display:flex;align-items:center;gap:0.8rem;margin-bottom:1rem;padding:0.6rem 1rem;border-radius:8px;background:var(--bg-tertiary);">
                <span style="font-size:0.85rem;color:var(--text-secondary);">${t('config.mcp.enabled_label')}</span>
                <div class="toggle ${mcpEnabled ? 'on' : ''}" data-path="mcp.enabled" onclick="toggleBool(this)"></div>
            </div>`;

            if (!mcpEnabled) {
                html += `<div class="wh-notice">
                    <span>⚠️</span>
                    <div>
                        <strong>${t('config.mcp.disabled_notice')}</strong><br>
                        <small>${t('config.mcp.disabled_desc')}</small>
                    </div>
                </div>`;
            }

            html += `<div style="display:flex;justify-content:flex-end;margin-bottom:1rem;">
                <button class="btn-save" style="padding:0.45rem 1.1rem;font-size:0.82rem;" onclick="mcpServerAdd()">
                    ＋ ${t('config.mcp.new_server')}
                </button>
            </div>
            <div id="mcp-servers-list"></div>
            <div id="mcp-servers-empty" style="display:none;text-align:center;padding:2rem;color:var(--text-tertiary);font-size:0.85rem;">
                ${t('config.mcp.empty')}
            </div>
            </div>`;
            document.getElementById('content').innerHTML = html;
            attachChangeListeners();
            mcpServerRenderCards();
        }

        function mcpServerRenderCards() {
            const wrap = document.getElementById('mcp-servers-list');
            const empty = document.getElementById('mcp-servers-empty');
            if (!wrap) return;
            if (mcpServersCache.length === 0) {
                wrap.innerHTML = '';
                if (empty) empty.style.display = '';
                return;
            }
            if (empty) empty.style.display = 'none';

            let html = '';
            mcpServersCache.forEach((s, idx) => {
                const enabledBadge = s.enabled
                    ? `<span style="display:inline-block;padding:0.15rem 0.5rem;border-radius:6px;font-size:0.7rem;font-weight:600;background:var(--success);color:#fff;margin-left:0.4rem;">✅ ${t('config.mcp.active_badge')}</span>`
                    : `<span style="display:inline-block;padding:0.15rem 0.5rem;border-radius:6px;font-size:0.7rem;font-weight:600;background:var(--text-tertiary);color:#fff;margin-left:0.4rem;">⏸ ${t('config.mcp.inactive_badge')}</span>`;
                const argsStr = (s.args || []).join(' ');
                const envCount = s.env ? Object.keys(s.env).length : 0;

                html += `
                <div class="provider-card" data-idx="${idx}" style="border:1px solid var(--border-subtle);border-radius:12px;padding:1rem 1.2rem;margin-bottom:0.75rem;background:var(--bg-secondary);transition:border-color 0.15s;">
                    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:0.6rem;">
                        <div style="font-weight:600;font-size:0.9rem;">
                            🔌 ${escapeAttr(s.name || '—')}${enabledBadge}
                        </div>
                        <div style="display:flex;gap:0.4rem;">
                            <button onclick="mcpServerEdit(${idx})" style="background:none;border:none;cursor:pointer;color:var(--accent);font-size:0.85rem;" title="Edit">✏️</button>
                            <button onclick="mcpServerDelete(${idx})" style="background:none;border:none;cursor:pointer;color:var(--danger);font-size:0.85rem;" title="Delete">🗑️</button>
                        </div>
                    </div>
                    <div style="display:grid;grid-template-columns:1fr 1fr;gap:0.3rem 1rem;font-size:0.78rem;">
                        <div><span style="color:var(--text-tertiary);">Command:</span> <code>${escapeAttr(s.command || '—')}</code></div>
                        <div><span style="color:var(--text-tertiary);">Args:</span> ${argsStr ? '<code>' + escapeAttr(argsStr) + '</code>' : '—'}</div>
                        <div><span style="color:var(--text-tertiary);">Env vars:</span> ${envCount}</div>
                    </div>
                </div>`;
            });
            wrap.innerHTML = html;
        }

        function mcpServerAdd() {
            mcpServerShowModal({name:'', command:'', args:[], env:{}, enabled:true}, -1);
        }

        function mcpServerEdit(idx) {
            mcpServerShowModal({...mcpServersCache[idx]}, idx);
        }

        async function mcpServerDelete(idx) {
            const s = mcpServersCache[idx];
            if (!confirm(t('config.mcp.delete_confirm', {name: s.name}))) return;
            mcpServersCache.splice(idx, 1);
            await mcpServerSave();
            mcpServerRenderCards();
        }

        function mcpServerShowModal(data, idx) {
            const isEdit = idx >= 0;
            const argsStr = (data.args || []).join('\n');
            const envStr = data.env ? Object.entries(data.env).map(([k,v]) => k + '=' + v).join('\n') : '';

            const overlay = document.createElement('div');
            overlay.style.cssText = 'position:fixed;inset:0;background:rgba(0,0,0,0.55);display:flex;align-items:center;justify-content:center;z-index:1000;';
            overlay.innerHTML = `
            <div style="background:var(--bg-secondary);border-radius:16px;padding:1.5rem;width:min(480px,90vw);max-height:85vh;overflow-y:auto;border:1px solid var(--border-subtle);">
                <div style="font-weight:700;font-size:1rem;margin-bottom:1rem;">${isEdit ? t('config.mcp.edit_server') : t('config.mcp.new_server')}</div>
                <label style="display:block;margin-bottom:0.8rem;">
                    <span style="font-size:0.78rem;color:var(--text-secondary);">Name</span>
                    <input id="mcp-m-name" class="cfg-input" value="${escapeAttr(data.name)}" placeholder="my-server" style="width:100%;margin-top:0.2rem;">
                </label>
                <label style="display:block;margin-bottom:0.8rem;">
                    <span style="font-size:0.78rem;color:var(--text-secondary);">Command</span>
                    <input id="mcp-m-command" class="cfg-input" value="${escapeAttr(data.command)}" placeholder="npx" style="width:100%;margin-top:0.2rem;">
                </label>
                <label style="display:block;margin-bottom:0.8rem;">
                    <span style="font-size:0.78rem;color:var(--text-secondary);">Args <small style="color:var(--text-tertiary);">(${t('config.mcp.args_hint')})</small></span>
                    <textarea id="mcp-m-args" class="cfg-input" rows="3" placeholder="-y\n@my/mcp-server" style="width:100%;margin-top:0.2rem;font-family:monospace;font-size:0.8rem;">${escapeAttr(argsStr)}</textarea>
                </label>
                <label style="display:block;margin-bottom:0.8rem;">
                    <span style="font-size:0.78rem;color:var(--text-secondary);">Environment Variables <small style="color:var(--text-tertiary);">(KEY=VALUE, ${t('config.mcp.env_hint')})</small></span>
                    <textarea id="mcp-m-env" class="cfg-input" rows="3" placeholder="API_KEY=xxx\nDEBUG=1" style="width:100%;margin-top:0.2rem;font-family:monospace;font-size:0.8rem;">${escapeAttr(envStr)}</textarea>
                </label>
                <label style="display:flex;align-items:center;gap:0.6rem;margin-bottom:1rem;">
                    <input id="mcp-m-enabled" type="checkbox" ${data.enabled ? 'checked' : ''}>
                    <span style="font-size:0.85rem;">${t('config.mcp.enabled_checkbox')}</span>
                </label>
                <div style="display:flex;gap:0.5rem;justify-content:flex-end;">
                    <button class="btn-save" style="background:var(--bg-tertiary);color:var(--text-primary);padding:0.45rem 1rem;" onclick="this.closest('div[style*=fixed]').remove()">${t('config.mcp.cancel')}</button>
                    <button class="btn-save" style="padding:0.45rem 1rem;" id="mcp-m-save">${t('config.mcp.save')}</button>
                </div>
            </div>`;
            document.body.appendChild(overlay);
            overlay.addEventListener('click', e => { if (e.target === overlay) overlay.remove(); });

            document.getElementById('mcp-m-save').addEventListener('click', async () => {
                const entry = {
                    name: document.getElementById('mcp-m-name').value.trim(),
                    command: document.getElementById('mcp-m-command').value.trim(),
                    args: document.getElementById('mcp-m-args').value.split('\n').map(l => l.trim()).filter(Boolean),
                    env: {},
                    enabled: document.getElementById('mcp-m-enabled').checked
                };
                // Parse env vars
                document.getElementById('mcp-m-env').value.split('\n').forEach(line => {
                    const eq = line.indexOf('=');
                    if (eq > 0) entry.env[line.substring(0, eq).trim()] = line.substring(eq + 1).trim();
                });
                if (!entry.name || !entry.command) {
                    alert(t('config.mcp.name_command_required'));
                    return;
                }
                if (isEdit) {
                    mcpServersCache[idx] = entry;
                } else {
                    if (mcpServersCache.some(s => s.name === entry.name)) {
                        alert(t('config.mcp.name_exists'));
                        return;
                    }
                    mcpServersCache.push(entry);
                }
                await mcpServerSave();
                overlay.remove();
                mcpServerRenderCards();
            });
        }

        async function mcpServerSave() {
            try {
                const resp = await fetch('/api/mcp-servers', {
                    method: 'PUT',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify(mcpServersCache)
                });
                if (!resp.ok) throw new Error(await resp.text());
                // Reload to get canonical data
                const reload = await fetch('/api/mcp-servers');
                if (reload.ok) mcpServersCache = await reload.json();
            } catch (e) {
                alert('❌ MCP save failed: ' + e.message);
            }
        }
