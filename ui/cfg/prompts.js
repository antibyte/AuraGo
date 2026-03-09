// cfg/prompts.js — Prompts & Personas section module

let persState = { personalities: [], active: '', editName: undefined, isCore: false };

        async function renderPromptsSection(section) {
            const agentCfg = configData.agent || {};
            const additionalPromptVal = agentCfg.additional_prompt || '';

            let personalities = [];
            let active = '';
            try {
                const resp = await fetch('/api/personalities');
                const data = await resp.json();
                personalities = data.personalities || [];
                active = data.active || '';
            } catch (e) {
                console.error('Failed to load personalities', e);
            }
            persState.personalities = personalities;
            persState.active = active;
            if (persState.editName === undefined) persState.editName = null;

            const addHelp = (helpTexts['agent.additional_prompt'] || {})[lang] || '';

            let html = '<div class="cfg-section active">';
            html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
            html += '<div class="section-desc">' + section.desc + '</div>';

            // Additional Prompt
            html += `
            <div style="margin-bottom:0.5rem;font-weight:600;font-size:0.85rem;color:var(--accent);border-bottom:1px solid var(--border-subtle);padding-bottom:0.3rem;">
                📝 ${t('config.prompts.additional_prompt_title')}
            </div>
            <div class="field-group">
                <div class="field-label">${t('config.prompts.additional_prompt_label')}</div>
                ${addHelp ? `<div class="field-help">${addHelp}</div>` : ''}
                <textarea class="field-input" data-path="agent.additional_prompt" rows="6"
                    style="resize:vertical;font-family:monospace;font-size:0.82rem;margin-top:0.35rem;"
                    oninput="markDirty()" onchange="markDirty()">${esc(additionalPromptVal)}</textarea>
            </div>`;

            // Personalities
            html += `
            <div style="margin-top:1.5rem;margin-bottom:0.5rem;font-weight:600;font-size:0.85rem;color:var(--accent);border-bottom:1px solid var(--border-subtle);padding-bottom:0.3rem;">
                🎭 ${t('config.prompts.personalities_title')}
            </div>
            <div class="field-group">
                <div class="pers-toolbar">
                    <span class="field-help" style="margin:0;">${t('config.prompts.active_label')}<strong style="color:var(--accent);">${esc(active)}</strong></span>
                    <button class="wh-btn wh-btn-primary wh-btn-sm" onclick="persNew()">+ ${t('config.prompts.new_personality')}</button>
                </div>
                <div class="pers-chips" id="pers-chips">`;
            for (const p of personalities) {
                const isActive = p.name === active;
                const coreLock = p.core ? ` <span style="font-size:0.7rem;opacity:0.7;" title="${t('config.prompts.core_tooltip')}">🔒</span>` : '';
                html += `<div class="pers-chip${isActive ? ' pers-chip-active' : ''}" onclick="persSelectForEdit('${escapeAttr(p.name)}')" title="${escapeAttr(p.name)}">${esc(p.name)}${coreLock}${isActive ? ' ✦' : ''}</div>`;
            }
            html += `</div>
                <div id="pers-editor" class="pers-editor" style="display:none;">
                    <div id="pers-editor-name-row" style="margin-bottom:0.75rem;">
                        <div class="field-label">${t('config.prompts.name_label')}</div>
                        <input id="pers-name-input" class="field-input" type="text"
                            placeholder="${t('config.prompts.name_placeholder')}"
                            style="margin-top:0.25rem;">
                    </div>
                    <div style="font-weight:600;font-size:0.78rem;color:var(--accent);margin-bottom:0.5rem;">
                        ${t('config.prompts.params_title')}
                    </div>
                    <div class="pers-meta-grid">
                        <div class="pers-slider-row">
                            <div class="pers-slider-header">
                                <span class="pers-slider-label">⚡ ${t('config.prompts.volatility_label')}</span>
                                <span class="pers-slider-val" id="pers-val-volatility">1.0</span>
                            </div>
                            <div class="pers-slider-desc">${t('config.prompts.volatility_desc')}</div>
                            <input id="pers-meta-volatility" class="pers-slider" type="range" min="0" max="2" step="0.1" value="1"
                                oninput="document.getElementById('pers-val-volatility').textContent=parseFloat(this.value).toFixed(1)">
                        </div>
                        <div class="pers-slider-row">
                            <div class="pers-slider-header">
                                <span class="pers-slider-label">💗 ${t('config.prompts.empathy_label')}</span>
                                <span class="pers-slider-val" id="pers-val-empathy">1.0</span>
                            </div>
                            <div class="pers-slider-desc">${t('config.prompts.empathy_desc')}</div>
                            <input id="pers-meta-empathy" class="pers-slider" type="range" min="0" max="2" step="0.1" value="1"
                                oninput="document.getElementById('pers-val-empathy').textContent=parseFloat(this.value).toFixed(1)">
                        </div>
                        <div class="pers-slider-row">
                            <div class="pers-slider-header">
                                <span class="pers-slider-label">💔 ${t('config.prompts.loneliness_label')}</span>
                                <span class="pers-slider-val" id="pers-val-loneliness">1.0</span>
                            </div>
                            <div class="pers-slider-desc">${t('config.prompts.loneliness_desc')}</div>
                            <input id="pers-meta-loneliness" class="pers-slider" type="range" min="0" max="2" step="0.1" value="1"
                                oninput="document.getElementById('pers-val-loneliness').textContent=parseFloat(this.value).toFixed(1)">
                        </div>
                        <div class="pers-slider-row">
                            <div class="pers-slider-header">
                                <span class="pers-slider-label">⏳ ${t('config.prompts.decay_label')}</span>
                                <span class="pers-slider-val" id="pers-val-decay">1.0</span>
                            </div>
                            <div class="pers-slider-desc">${t('config.prompts.decay_desc')}</div>
                            <input id="pers-meta-decay" class="pers-slider" type="range" min="0.1" max="2" step="0.1" value="1"
                                oninput="document.getElementById('pers-val-decay').textContent=parseFloat(this.value).toFixed(1)">
                        </div>
                        <div class="pers-select-row">
                            <div class="pers-select-label">⚔️ ${t('config.prompts.conflict_label')}</div>
                            <div class="pers-select-desc">${t('config.prompts.conflict_desc')}</div>
                            <select id="pers-meta-conflict" class="field-select">
                                <option value="neutral">${t('config.prompts.conflict_neutral')}</option>
                                <option value="submissive">${t('config.prompts.conflict_submissive')}</option>
                                <option value="assertive">${t('config.prompts.conflict_assertive')}</option>
                            </select>
                        </div>
                    </div>
                    <div class="field-label" style="margin-bottom:0.25rem;">${t('config.prompts.prompt_text_label')}</div>
                    <div class="field-help" style="margin-bottom:0.35rem;">${t('config.prompts.metadata_hint')}</div>
                    <textarea id="pers-content-input" class="field-input" rows="12"
                        style="resize:vertical;font-family:monospace;font-size:0.82rem;"></textarea>
                    <div class="pers-editor-actions">
                        <button class="wh-btn wh-btn-primary wh-btn-sm" onclick="persSave()">💾 ${t('config.prompts.save')}</button>
                        <button class="wh-btn wh-btn-sm" id="pers-activate-btn" onclick="persActivate()" style="display:none;">⚡ ${t('config.prompts.set_active')}</button>
                        <button class="wh-btn wh-btn-sm" id="pers-delete-btn" onclick="persDelete()" style="color:#f87171;border-color:rgba(239,68,68,0.3);display:none;">🗑 ${t('config.prompts.delete')}</button>
                        <button class="wh-btn wh-btn-sm" onclick="persCancel()">✕ ${t('config.prompts.cancel')}</button>
                    </div>
                    <div id="pers-editor-status" class="field-help" style="margin-top:0.5rem;min-height:1.2em;"></div>
                </div>
            </div>`;

            html += '</div>';
            document.getElementById('content').innerHTML = html;
            attachChangeListeners();
        }

        function persSetMetaDefaults() {
            const set = (id, v) => { const el = document.getElementById(id); if (el) el.value = v; };
            const setVal = (id, v) => { const el = document.getElementById(id); if (el) el.textContent = parseFloat(v).toFixed(1); };
            set('pers-meta-volatility', 1.0); setVal('pers-val-volatility', 1.0);
            set('pers-meta-empathy', 1.0); setVal('pers-val-empathy', 1.0);
            set('pers-meta-loneliness', 1.0); setVal('pers-val-loneliness', 1.0);
            set('pers-meta-decay', 1.0); setVal('pers-val-decay', 1.0);
            const cf = document.getElementById('pers-meta-conflict'); if (cf) cf.value = 'neutral';
        }

        function persApplyMeta(meta) {
            if (!meta) { persSetMetaDefaults(); return; }
            const set = (id, v) => { const el = document.getElementById(id); if (el) el.value = v; };
            const setVal = (id, v) => { const el = document.getElementById(id); if (el) el.textContent = parseFloat(v).toFixed(1); };
            set('pers-meta-volatility', meta.volatility ?? 1.0); setVal('pers-val-volatility', meta.volatility ?? 1.0);
            set('pers-meta-empathy', meta.empathy_bias ?? 1.0); setVal('pers-val-empathy', meta.empathy_bias ?? 1.0);
            set('pers-meta-loneliness', meta.loneliness_susceptibility ?? 1.0); setVal('pers-val-loneliness', meta.loneliness_susceptibility ?? 1.0);
            set('pers-meta-decay', meta.trait_decay_rate ?? 1.0); setVal('pers-val-decay', meta.trait_decay_rate ?? 1.0);
            const cf = document.getElementById('pers-meta-conflict'); if (cf) cf.value = meta.conflict_response || 'neutral';
        }

        function persBuildFrontmatter(name) {
            const fv = (id) => parseFloat(document.getElementById(id)?.value ?? 1.0).toFixed(1);
            const cv = document.getElementById('pers-meta-conflict')?.value || 'neutral';
            return `---\nid: "${name}"\ntags: ["core"]\npriority: 100\nmeta:\n  volatility: ${fv('pers-meta-volatility')}\n  empathy_bias: ${fv('pers-meta-empathy')}\n  conflict_response: "${cv}"\n  loneliness_susceptibility: ${fv('pers-meta-loneliness')}\n  trait_decay_rate: ${fv('pers-meta-decay')}\n---\n\n`;
        }

        function persNew() {
            persState.editName = null;
            persState.isCore = false;
            const editor = document.getElementById('pers-editor');
            if (!editor) return;
            document.getElementById('pers-editor-name-row').style.display = 'block';
            document.getElementById('pers-activate-btn').style.display = 'none';
            document.getElementById('pers-delete-btn').style.display = 'none';
            document.getElementById('pers-name-input').value = '';
            document.getElementById('pers-content-input').value = '';
            document.getElementById('pers-editor-status').textContent = '';
            persSetMetaDefaults();
            persApplyCoreMode(false);
            editor.style.display = 'block';
            document.querySelectorAll('.pers-chip').forEach(c => c.classList.remove('selected'));
            document.getElementById('pers-name-input').focus();
        }

        async function persSelectForEdit(name) {
            persState.editName = name;
            // Determine if this is a built-in (read-only) persona
            const entry = persState.personalities.find(p => p.name === name);
            persState.isCore = entry ? entry.core : false;

            document.querySelectorAll('.pers-chip').forEach(c => {
                const chipName = c.textContent.replace(' ✦', '').replace('🔒', '').trim();
                c.classList.toggle('selected', chipName === name);
            });
            const editor = document.getElementById('pers-editor');
            if (!editor) return;
            document.getElementById('pers-editor-name-row').style.display = 'none';
            document.getElementById('pers-activate-btn').style.display = name === persState.active ? 'none' : 'inline-block';
            document.getElementById('pers-delete-btn').style.display = name === persState.active ? 'none' : 'inline-block';
            document.getElementById('pers-content-input').value = '';
            const status = document.getElementById('pers-editor-status');
            status.textContent = t('config.prompts.loading');
            editor.style.display = 'block';
            try {
                const resp = await fetch('/api/config/personality-files?name=' + encodeURIComponent(name));
                if (!resp.ok) throw new Error('HTTP ' + resp.status);
                const data = await resp.json();
                document.getElementById('pers-content-input').value = data.body || '';
                persApplyMeta(data.meta);
                status.textContent = '';
            } catch (e) {
                status.textContent = '❌ ' + e.message;
            }
            // Apply read-only mode for core personas AFTER content is loaded
            persApplyCoreMode(persState.isCore);
        }

        // Toggles the editor between read-only (core) and editable (user) mode.
        function persApplyCoreMode(isCore) {
            const inputs = ['pers-meta-volatility', 'pers-meta-empathy', 'pers-meta-loneliness', 'pers-meta-decay', 'pers-meta-conflict', 'pers-content-input'];
            inputs.forEach(id => {
                const el = document.getElementById(id);
                if (el) el.disabled = isCore;
            });
            const saveBtn = document.querySelector('.pers-editor-actions .wh-btn-primary');
            if (saveBtn) saveBtn.style.display = isCore ? 'none' : '';
            const delBtn = document.getElementById('pers-delete-btn');
            if (delBtn && isCore) delBtn.style.display = 'none';
            const statusEl = document.getElementById('pers-editor-status');
            if (statusEl) {
                if (isCore) {
                    statusEl.innerHTML = '<span style="color:var(--text-secondary);">🔒 ' +
                        t('config.prompts.core_readonly') + '</span>';
                } else {
                    // Only clear if it currently shows the lock message
                    if (statusEl.textContent.includes('🔒')) statusEl.textContent = '';
                }
            }
        }

        async function persSave() {
            if (persState.isCore) {
                const s = document.getElementById('pers-editor-status');
                if (s) s.textContent = '❌ ' + t('config.prompts.core_cannot_modify');
                return;
            }
            const isNew = persState.editName === null;
            const nameInput = document.getElementById('pers-name-input');
            const contentInput = document.getElementById('pers-content-input');
            const status = document.getElementById('pers-editor-status');
            const name = isNew ? (nameInput ? nameInput.value.trim() : '') : persState.editName;
            if (!name) { status.textContent = '❌ ' + t('config.prompts.name_required'); return; }
            status.textContent = t('config.prompts.saving');
            // Reconstruct full file: YAML front matter + body
            const frontmatter = persBuildFrontmatter(name);
            const fullContent = frontmatter + (contentInput.value || '');
            try {
                const resp = await fetch('/api/config/personality-files', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ name, content: fullContent })
                });
                if (!resp.ok) { const errText = await resp.text(); throw new Error(errText || 'HTTP ' + resp.status); }
                status.textContent = '✓ ' + t('config.prompts.saved');
                if (isNew) {
                    persState.editName = name;
                    const sectionMeta = SECTIONS.flatMap(g => g.items).find(s => s.key === 'prompts_editor');
                    await renderPromptsSection(sectionMeta);
                    await persSelectForEdit(name);
                }
            } catch (e) { status.textContent = '❌ ' + e.message; }
        }

        async function persActivate() {
            const name = persState.editName;
            if (!name) return;
            const status = document.getElementById('pers-editor-status');
            status.textContent = t('config.prompts.setting_active');
            try {
                const resp = await fetch('/api/personality', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ id: name })
                });
                if (!resp.ok) throw new Error('HTTP ' + resp.status);
                if (!configData.agent) configData.agent = {};
                configData.agent.core_personality = name;
                persState.active = name;
                const cfgResp = await fetch('/api/config');
                configData = await cfgResp.json();
                const sectionMeta = SECTIONS.flatMap(g => g.items).find(s => s.key === 'prompts_editor');
                await renderPromptsSection(sectionMeta);
                await persSelectForEdit(name);
            } catch (e) { const s = document.getElementById('pers-editor-status'); if (s) s.textContent = '❌ ' + e.message; }
        }

        async function persDelete() {
            const name = persState.editName;
            if (!name) return;
            const confirmed = confirm(t('config.prompts.delete_confirm', {name: name}));
            if (!confirmed) return;
            const status = document.getElementById('pers-editor-status');
            status.textContent = t('config.prompts.deleting');
            try {
                const resp = await fetch('/api/config/personality-files?name=' + encodeURIComponent(name), { method: 'DELETE' });
                if (!resp.ok) { const errText = await resp.text(); throw new Error(errText || 'HTTP ' + resp.status); }
                persState.editName = undefined;
                const sectionMeta = SECTIONS.flatMap(g => g.items).find(s => s.key === 'prompts_editor');
                await renderPromptsSection(sectionMeta);
            } catch (e) { const s = document.getElementById('pers-editor-status'); if (s) s.textContent = '❌ ' + e.message; }
        }

        function persCancel() {
            const editor = document.getElementById('pers-editor');
            if (editor) editor.style.display = 'none';
            document.querySelectorAll('.pers-chip').forEach(c => c.classList.remove('selected'));
            persState.editName = undefined;
        }
