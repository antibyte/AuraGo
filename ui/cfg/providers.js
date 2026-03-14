// cfg/providers.js — Provider Management section module (includes Ollama, OpenRouter, MeshCentral)
// providersCache is declared in config.html core (used by renderField for provider_ref dropdowns)

let _orModelsCache = null;
let _orModelsCacheTime = 0;
const OR_CACHE_TTL = 5 * 60 * 1000;

        async function queryOllamaModelsInModal() {
            const spinner = document.getElementById('prov-ollama-spinner');
            const resultDiv = document.getElementById('prov-ollama-result');
            const modelsDiv = document.getElementById('prov-ollama-models');
            const errorDiv = document.getElementById('prov-ollama-error');
            if (!spinner) return;

            spinner.style.display = 'inline';
            resultDiv.style.display = 'none';
            errorDiv.style.display = 'none';
            modelsDiv.innerHTML = '';

            const baseUrl = (document.getElementById('prov-url') || {}).value || '';
            const url = '/api/ollama/models' + (baseUrl ? '?url=' + encodeURIComponent(baseUrl) : '');

            try {
                const resp = await fetch(url);
                const json = await resp.json();
                resultDiv.style.display = 'block';

                if (!resp.ok || json.available === false) {
                    errorDiv.textContent = json.reason || t('config.ollama.fetch_error');
                    errorDiv.style.display = 'block';
                } else if (!json.models || json.models.length === 0) {
                    errorDiv.textContent = t('config.ollama.no_models');
                    errorDiv.style.display = 'block';
                } else {
                    modelsDiv.innerHTML = json.models.map(m =>
                        `<span style="display:inline-block;padding:0.2rem 0.6rem;border-radius:6px;background:var(--bg-primary);border:1px solid var(--border-subtle);font-family:monospace;font-size:0.75rem;color:var(--accent);cursor:pointer;transition:background 0.15s;" title="${t('config.ollama.click_to_apply')}" onmouseover="this.style.background='var(--accent)';this.style.color='#fff'" onmouseout="this.style.background='var(--bg-primary)';this.style.color='var(--accent)'" onclick="applyOllamaModelInModal(this.textContent)">${m}</span>`
                    ).join('');
                }
            } catch (e) {
                resultDiv.style.display = 'block';
                errorDiv.textContent = t('config.ollama.connection_error') + e.message;
                errorDiv.style.display = 'block';
            } finally {
                spinner.style.display = 'none';
            }
        }

        function applyOllamaModelInModal(modelName) {
            const modelInput = document.getElementById('prov-model');
            if (modelInput) {
                modelInput.value = modelName;
                modelInput.dispatchEvent(new Event('input', { bubbles: true }));
            }
        }

        /* ── OpenRouter Model Browser (reusable modal) ── */

        /**
         * Opens a full-screen OpenRouter model browser modal.
         * @param {function} onSelect - callback({id, name, pricing, context_length}) when user picks a model
         */
        async function openOpenRouterBrowser(onSelect) {

            // Remove existing modal if any
            const existing = document.getElementById('or-browser-overlay');
            if (existing) existing.remove();

            const overlay = document.createElement('div');
            overlay.id = 'or-browser-overlay';
            overlay.style.cssText = 'position:fixed;inset:0;background:rgba(0,0,0,0.6);z-index:1100;backdrop-filter:blur(4px);display:flex;align-items:center;justify-content:center;';
            overlay.onclick = (e) => { if (e.target === overlay) overlay.remove(); };

            overlay.innerHTML = `
            <div style="background:var(--bg-secondary);border-radius:16px;width:min(760px,95vw);height:min(85vh,720px);display:flex;flex-direction:column;border:1px solid var(--border-subtle);overflow:hidden;" onclick="event.stopPropagation()">
                <!-- Header -->
                <div style="padding:1rem 1.2rem;border-bottom:1px solid var(--border-subtle);flex-shrink:0;">
                    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:0.7rem;">
                        <div style="font-weight:700;font-size:1rem;">🌐 OpenRouter ${t('config.openrouter.model_browser')}</div>
                        <button onclick="document.getElementById('or-browser-overlay').remove()" style="background:none;border:none;color:var(--text-secondary);font-size:1.2rem;cursor:pointer;">✕</button>
                    </div>
                    <div style="display:flex;gap:0.5rem;align-items:center;flex-wrap:wrap;">
                        <input id="or-search" class="field-input" type="text" placeholder="${t('config.openrouter.search_placeholder')}" style="flex:1;min-width:180px;font-size:0.82rem;padding:0.4rem 0.7rem;">
                        <button id="or-free-btn" class="btn-save" style="padding:0.35rem 0.8rem;font-size:0.75rem;background:var(--bg-tertiary);color:var(--text-primary);border:1px solid var(--border-subtle);white-space:nowrap;" title="${t('config.openrouter.free_tooltip')}">
                            🆓 ${t('config.openrouter.free_button')}
                        </button>
                        <div id="or-count" style="font-size:0.72rem;color:var(--text-tertiary);white-space:nowrap;"></div>
                    </div>
                </div>
                <!-- Content area: list + detail -->
                <div style="flex:1;display:flex;overflow:hidden;">
                    <!-- Model list -->
                    <div id="or-list-wrap" style="flex:1;overflow-y:auto;min-width:0;">
                        <div id="or-list" style="padding:0.3rem;"></div>
                    </div>
                    <!-- Detail panel (hidden by default) -->
                    <div id="or-detail" style="width:280px;flex-shrink:0;border-left:1px solid var(--border-subtle);overflow-y:auto;display:none;padding:1rem;font-size:0.8rem;"></div>
                </div>
                <!-- Loading / error -->
                <div id="or-loading" style="padding:2rem;text-align:center;color:var(--text-secondary);font-size:0.85rem;">
                    ⏳ ${t('config.openrouter.loading')}
                </div>
            </div>`;
            document.body.appendChild(overlay);

            const searchInput = document.getElementById('or-search');
            const freeBtn = document.getElementById('or-free-btn');
            const listDiv = document.getElementById('or-list');
            const detailDiv = document.getElementById('or-detail');
            const countDiv = document.getElementById('or-count');
            const loadingDiv = document.getElementById('or-loading');
            const listWrap = document.getElementById('or-list-wrap');

            let allModels = [];
            let freeOnly = false;
            let selectedModel = null;

            // Fetch models
            try {
                const now = Date.now();
                if (_orModelsCache && (now - _orModelsCacheTime) < OR_CACHE_TTL) {
                    allModels = _orModelsCache;
                } else {
                    const resp = await fetch('/api/openrouter/models');
                    const json = await resp.json();
                    if (json.available === false) {
                        loadingDiv.innerHTML = '<span style="color:var(--danger);">❌ ' + (json.reason || 'Error') + '</span>';
                        return;
                    }
                    allModels = (json.data || []).map(m => ({
                        id: m.id || '',
                        name: m.name || m.id || '',
                        description: m.description || '',
                        context_length: m.context_length || 0,
                        pricing: m.pricing || {},
                        architecture: m.architecture || {},
                        top_provider: m.top_provider || {},
                    }));
                    // Sort by name
                    allModels.sort((a, b) => a.name.localeCompare(b.name));
                    _orModelsCache = allModels;
                    _orModelsCacheTime = now;
                }
            } catch (e) {
                loadingDiv.innerHTML = '<span style="color:var(--danger);">❌ ' + t('config.openrouter.connection_error') + e.message + '</span>';
                return;
            }
            loadingDiv.style.display = 'none';
            listWrap.style.display = 'block';

            function isFree(m) {
                return m.pricing && parseFloat(m.pricing.prompt || '1') === 0 && parseFloat(m.pricing.completion || '1') === 0;
            }

            function formatCost(perToken) {
                const val = parseFloat(perToken || '0');
                if (val === 0) return t('config.openrouter.free_cost');
                const perMillion = val * 1000000;
                if (perMillion < 0.01) return '$' + perMillion.toFixed(4) + '/M';
                if (perMillion < 1) return '$' + perMillion.toFixed(3) + '/M';
                return '$' + perMillion.toFixed(2) + '/M';
            }

            function formatContext(ctx) {
                if (!ctx) return '—';
                if (ctx >= 1000000) return (ctx / 1000000).toFixed(1) + 'M';
                if (ctx >= 1000) return Math.round(ctx / 1000) + 'K';
                return ctx.toString();
            }

            function renderList() {
                const query = searchInput.value.toLowerCase().trim();
                const filtered = allModels.filter(m => {
                    if (freeOnly && !isFree(m)) return false;
                    if (query) {
                        return m.id.toLowerCase().includes(query) || m.name.toLowerCase().includes(query);
                    }
                    return true;
                });

                countDiv.textContent = filtered.length + t('config.openrouter.models_count') + (freeOnly ? t('config.openrouter.free_only') : '');

                if (filtered.length === 0) {
                    listDiv.innerHTML = '<div style="padding:1.5rem;text-align:center;color:var(--text-tertiary);font-size:0.82rem;">' + t('config.openrouter.no_models') + '</div>';
                    return;
                }

                // Virtual-ish rendering: just render all since DOM handles it fine with simple divs
                listDiv.innerHTML = filtered.map(m => {
                    const free = isFree(m);
                    const promptCost = formatCost(m.pricing.prompt);
                    const completionCost = formatCost(m.pricing.completion);
                    const isSelected = selectedModel && selectedModel.id === m.id;
                    return `<div class="or-model-row" data-id="${escapeAttr(m.id)}" style="padding:0.5rem 0.7rem;border-bottom:1px solid var(--border-subtle);cursor:pointer;display:flex;align-items:center;gap:0.6rem;transition:background 0.12s;${isSelected?'background:var(--accent-dim);':''}">
                        <div style="flex:1;min-width:0;">
                            <div style="font-size:0.8rem;font-weight:600;color:var(--text-primary);white-space:nowrap;overflow:hidden;text-overflow:ellipsis;" title="${escapeAttr(m.id)}">${escapeHtml(m.name)}</div>
                            <div style="font-size:0.68rem;color:var(--text-tertiary);font-family:monospace;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;">${escapeHtml(m.id)}</div>
                        </div>
                        <div style="flex-shrink:0;display:flex;gap:0.4rem;align-items:center;">
                            ${free ? '<span style="font-size:0.65rem;padding:0.1rem 0.4rem;border-radius:4px;background:rgba(72,199,142,0.15);color:#48c78e;font-weight:600;">FREE</span>' : '<span style="font-size:0.65rem;color:var(--text-tertiary);" title="Input / Output">'+promptCost+' · '+completionCost+'</span>'}
                            <span style="font-size:0.65rem;color:var(--text-tertiary);" title="Context">${formatContext(m.context_length)}</span>
                        </div>
                    </div>`;
                }).join('');

                // Attach click handlers
                listDiv.querySelectorAll('.or-model-row').forEach(row => {
                    row.onmouseover = () => { if (!row.style.background.includes('accent')) row.style.background='var(--bg-tertiary)'; };
                    row.onmouseout = () => { if (selectedModel && selectedModel.id === row.dataset.id) row.style.background='var(--accent-dim)'; else row.style.background=''; };
                    row.onclick = () => {
                        const m = allModels.find(x => x.id === row.dataset.id);
                        if (!m) return;
                        selectedModel = m;
                        showDetail(m);
                        // Highlight selected
                        listDiv.querySelectorAll('.or-model-row').forEach(r => r.style.background='');
                        row.style.background = 'var(--accent-dim)';
                    };
                });
            }

            function showDetail(m) {
                detailDiv.style.display = 'block';
                const free = isFree(m);
                const promptPerM = (parseFloat(m.pricing.prompt || '0') * 1000000);
                const completionPerM = (parseFloat(m.pricing.completion || '0') * 1000000);
                detailDiv.innerHTML = `
                    <div style="font-weight:700;font-size:0.9rem;color:var(--text-primary);margin-bottom:0.3rem;">${escapeHtml(m.name)}</div>
                    <div style="font-family:monospace;font-size:0.7rem;color:var(--accent);margin-bottom:0.7rem;word-break:break-all;">${escapeHtml(m.id)}</div>
                    ${m.description ? '<div style="font-size:0.75rem;color:var(--text-secondary);margin-bottom:0.8rem;line-height:1.4;max-height:120px;overflow-y:auto;">' + escapeHtml(m.description) + '</div>' : ''}
                    <div style="display:grid;grid-template-columns:1fr 1fr;gap:0.4rem 0.8rem;font-size:0.75rem;margin-bottom:0.8rem;">
                        <div><span style="color:var(--text-tertiary);">Context:</span></div>
                        <div style="font-weight:600;">${formatContext(m.context_length)} tokens</div>
                        <div><span style="color:var(--text-tertiary);">Input:</span></div>
                        <div style="font-weight:600;${free?'color:#48c78e;':''}">${free ? t('config.openrouter.free_cost') : '$'+promptPerM.toFixed(4)+'/M'}</div>
                        <div><span style="color:var(--text-tertiary);">Output:</span></div>
                        <div style="font-weight:600;${free?'color:#48c78e;':''}">${free ? t('config.openrouter.free_cost') : '$'+completionPerM.toFixed(4)+'/M'}</div>
                    </div>
                    ${free ? '<div style="padding:0.4rem 0.6rem;border-radius:7px;background:rgba(72,199,142,0.1);border:1px solid rgba(72,199,142,0.2);font-size:0.72rem;color:#48c78e;margin-bottom:0.8rem;">🆓 '+t('config.openrouter.model_is_free')+'</div>' : ''}
                    <button id="or-apply-btn" class="btn-save" style="width:100%;padding:0.5rem;font-size:0.82rem;">
                        ✅ ${t('config.openrouter.apply_model')}
                    </button>
                    <div style="font-size:0.68rem;color:var(--text-tertiary);margin-top:0.5rem;text-align:center;">
                        ${!free ? t('config.openrouter.costs_auto_imported') : ''}
                    </div>
                `;
                document.getElementById('or-apply-btn').onclick = () => {
                    if (onSelect) onSelect({
                        id: m.id,
                        name: m.name,
                        pricing: m.pricing,
                        context_length: m.context_length,
                        inputPerMillion: parseFloat(m.pricing.prompt || '0') * 1000000,
                        outputPerMillion: parseFloat(m.pricing.completion || '0') * 1000000,
                    });
                    overlay.remove();
                };
            }

            // Search with debounce
            let searchTimer;
            searchInput.oninput = () => {
                clearTimeout(searchTimer);
                searchTimer = setTimeout(renderList, 150);
            };

            // Free toggle
            freeBtn.onclick = () => {
                freeOnly = !freeOnly;
                freeBtn.style.background = freeOnly ? 'rgba(72,199,142,0.2)' : 'var(--bg-tertiary)';
                freeBtn.style.color = freeOnly ? '#48c78e' : 'var(--text-primary)';
                freeBtn.style.borderColor = freeOnly ? 'rgba(72,199,142,0.4)' : 'var(--border-subtle)';
                renderList();
            };

            // Initial render
            renderList();
            searchInput.focus();
        }

        /**
         * Auto-update budget.models with pricing from OpenRouter model selection.
         * Adds new entry or updates existing one.
         * @param {object} m - model data with id, inputPerMillion, outputPerMillion
         */
        function updateBudgetModelCost(m) {
            if (!m || !m.id) return;
            // Only update if there are non-zero costs
            const inputPM = m.inputPerMillion || 0;
            const outputPM = m.outputPerMillion || 0;

            // Ensure budget.models exists
            if (!configData.budget) configData.budget = {};
            if (!Array.isArray(configData.budget.models)) configData.budget.models = [];

            // Check if model already exists
            const existing = configData.budget.models.find(
                e => e.name && e.name.toLowerCase() === m.id.toLowerCase()
            );

            if (existing) {
                existing.input_per_million = inputPM;
                existing.output_per_million = outputPM;
            } else {
                configData.budget.models.push({
                    name: m.id,
                    input_per_million: inputPM,
                    output_per_million: outputPM,
                });
            }
            setDirty(true);

            // Show a brief toast-style notification
            const msg = t('config.budget.model_cost_updated', { model: m.id });
            showBudgetToast(msg);
        }

        function showBudgetToast(message) {
            const existing = document.getElementById('budget-toast');
            if (existing) existing.remove();
            const toast = document.createElement('div');
            toast.id = 'budget-toast';
            toast.style.cssText = 'position:fixed;bottom:1.5rem;left:50%;transform:translateX(-50%);background:var(--bg-secondary);color:var(--text-primary);padding:0.6rem 1.2rem;border-radius:10px;font-size:0.8rem;border:1px solid rgba(72,199,142,0.3);box-shadow:0 4px 16px rgba(0,0,0,0.3);z-index:1200;animation:fadeInUp 0.3s;';
            toast.textContent = message;
            document.body.appendChild(toast);
            setTimeout(() => {
                toast.style.opacity = '0';
                toast.style.transition = 'opacity 0.3s';
                setTimeout(() => toast.remove(), 300);
            }, 3500);
        }

        // ── Model Pricing Table helpers (inside provider modal) ──

        // Temporary model list for the currently open provider modal
        let _provModalModels = [];

        function providerInitModelsTable(models) {
            _provModalModels = (models || []).map(m => ({...m}));
            providerRenderModelsTable();
        }

        function providerRenderModelsTable() {
            const tbody = document.getElementById('prov-models-body');
            const empty = document.getElementById('prov-models-empty');
            const searchWrap = document.getElementById('prov-models-search-wrap');
            const filterEl = document.getElementById('prov-models-filter');
            const countEl = document.getElementById('prov-models-count');
            if (!tbody) return;

            // Show filter bar only when enough rows to be useful
            if (searchWrap) searchWrap.style.display = _provModalModels.length >= 5 ? '' : 'none';

            const filterQ = filterEl ? filterEl.value.toLowerCase().trim() : '';
            const visible = filterQ
                ? _provModalModels.filter(m => m.name && m.name.toLowerCase().includes(filterQ))
                : _provModalModels;

            if (countEl && _provModalModels.length > 0) {
                countEl.textContent = filterQ
                    ? `${visible.length} / ${_provModalModels.length}`
                    : `${_provModalModels.length} model${_provModalModels.length !== 1 ? 's' : ''}`;
                countEl.style.display = '';
            } else if (countEl) {
                countEl.style.display = 'none';
            }

            if (_provModalModels.length === 0) {
                tbody.innerHTML = '';
                if (empty) empty.style.display = '';
                return;
            }
            if (empty) empty.style.display = 'none';

            if (visible.length === 0) {
                tbody.innerHTML = `<tr><td colspan="4" style="padding:0.8rem;text-align:center;color:var(--text-tertiary);font-size:0.75rem;">${escapeHtml(t('config.providers.no_filter_results', { q: filterQ }))}</td></tr>`;
                return;
            }

            tbody.innerHTML = visible.map(m => {
                const i = _provModalModels.indexOf(m);
                return `
                <tr style="border-bottom:1px solid var(--border-subtle);">
                    <td style="padding:0.3rem 0.4rem;">
                        <input class="field-input" style="font-size:0.75rem;padding:0.2rem 0.4rem;" value="${escapeAttr(m.name)}" onchange="providerUpdateModelRow(${i},'name',this.value)">
                    </td>
                    <td style="padding:0.3rem 0.4rem;">
                        <input class="field-input" type="number" step="0.001" min="0" style="font-size:0.75rem;padding:0.2rem 0.4rem;text-align:right;width:90px;" value="${m.input_per_million}" onchange="providerUpdateModelRow(${i},'input_per_million',parseFloat(this.value)||0)">
                    </td>
                    <td style="padding:0.3rem 0.4rem;">
                        <input class="field-input" type="number" step="0.001" min="0" style="font-size:0.75rem;padding:0.2rem 0.4rem;text-align:right;width:90px;" value="${m.output_per_million}" onchange="providerUpdateModelRow(${i},'output_per_million',parseFloat(this.value)||0)">
                    </td>
                    <td style="padding:0.3rem 0;text-align:center;">
                        <button type="button" style="background:none;border:none;cursor:pointer;color:var(--danger);font-size:0.8rem;" onclick="providerDeleteModelRow(${i})" title="Delete">✕</button>
                    </td>
                </tr>`;
            }).join('');
        }

        function providerAddModelRow() {
            _provModalModels.push({ name: '', input_per_million: 0, output_per_million: 0 });
            providerRenderModelsTable();
            // Focus the new name field
            setTimeout(() => {
                const inputs = document.querySelectorAll('#prov-models-body tr:last-child input');
                if (inputs.length) inputs[0].focus();
            }, 30);
        }

        function providerUpdateModelRow(idx, field, value) {
            if (_provModalModels[idx]) _provModalModels[idx][field] = value;
        }

        function providerDeleteModelRow(idx) {
            _provModalModels.splice(idx, 1);
            providerRenderModelsTable();
        }

        function providerGetModels() {
            return _provModalModels.filter(m => m.name && m.name.trim());
        }

        // Called from OpenRouter browser when a model is selected
        function updateProviderModelCost(m) {
            if (!m || !m.id) return;
            const inputPM = m.inputPerMillion || 0;
            const outputPM = m.outputPerMillion || 0;

            const existing = _provModalModels.find(
                e => e.name && e.name.toLowerCase() === m.id.toLowerCase()
            );
            if (existing) {
                existing.input_per_million = inputPM;
                existing.output_per_million = outputPM;
            } else {
                _provModalModels.push({
                    name: m.id,
                    input_per_million: inputPM,
                    output_per_million: outputPM,
                });
            }
            providerRenderModelsTable();
            showBudgetToast(t('config.budget.model_cost_updated', { model: m.id }));
        }

        /**
         * Opens a searchable picker modal to select which models/prices to import.
         * @param {Array<{name,input_per_million,output_per_million}>} allPricing
         */
        function openPricingPickerModal(allPricing) {
            if (!allPricing || allPricing.length === 0) {
                showBudgetToast(t('config.providers.no_pricing_found'));
                return;
            }

            let filterQuery = '';
            const activeNames = new Set(_provModalModels.map(m => (m.name || '').toLowerCase()));
            // Pre-select already configured models
            const selected = new Set(
                allPricing.filter(m => activeNames.has(m.name.toLowerCase())).map(m => m.name)
            );

            const overlay = document.createElement('div');
            overlay.id = 'pricing-picker-overlay';
            overlay.style.cssText = 'position:fixed;inset:0;background:rgba(0,0,0,0.65);z-index:9100;display:flex;align-items:center;justify-content:center;padding:1rem;';

            function getVisible() {
                const q = filterQuery.toLowerCase();
                return q ? allPricing.filter(m => m.name.toLowerCase().includes(q)) : allPricing;
            }

            function renderPicker() {
                const visible = getVisible();
                const selCount = selected.size;
                const allVisibleSelected = visible.length > 0 && visible.every(m => selected.has(m.name));

                overlay.innerHTML = `
                    <div style="background:var(--bg-secondary);border:1px solid var(--border-subtle);border-radius:14px;width:min(580px,96vw);max-height:84vh;display:flex;flex-direction:column;box-shadow:0 24px 64px rgba(0,0,0,0.55);">
                        <div style="padding:0.9rem 1.1rem 0.6rem;border-bottom:1px solid var(--border-subtle);flex-shrink:0;">
                            <div style="font-size:0.9rem;font-weight:700;color:var(--text-primary);margin-bottom:0.55rem;">📡 ${escapeHtml(t('config.providers.pricing_picker_title'))}</div>
                            <input id="pricing-picker-search" class="field-input" type="search" autocomplete="off"
                                   placeholder="🔍 ${escapeHtml(t('config.providers.pricing_picker_search'))}"
                                   style="width:100%;font-size:0.8rem;padding:0.32rem 0.55rem;" value="${escapeAttr(filterQuery)}">
                            <div style="display:flex;justify-content:space-between;align-items:center;margin-top:0.35rem;">
                                <span style="font-size:0.7rem;color:var(--text-tertiary);">${visible.length} / ${allPricing.length} ${escapeHtml(t('config.providers.pricing_picker_total'))}</span>
                                <span id="pp-sel-count" style="font-size:0.7rem;color:var(--accent);font-weight:600;">${selCount} ${escapeHtml(t('config.providers.pricing_picker_selected'))}</span>
                            </div>
                        </div>
                        <div style="overflow-y:auto;flex:1;" id="pricing-picker-list">
                            ${visible.length === 0
                                ? `<div style="padding:2rem;text-align:center;color:var(--text-tertiary);font-size:0.82rem;">${escapeHtml(t('config.providers.no_pricing_found'))}</div>`
                                : visible.map(m => {
                                    const ck = selected.has(m.name);
                                    const fmt = v => v > 0 ? '$' + v.toFixed(3) : (v === 0 ? 'free' : '—');
                                    return `<label style="display:flex;align-items:center;gap:0.55rem;padding:0.36rem 1rem;cursor:pointer;border-bottom:1px solid rgba(255,255,255,0.04);" onmouseover="this.style.background='var(--bg-tertiary)'" onmouseout="this.style.background=''">
                                        <input type="checkbox" data-name="${escapeAttr(m.name)}" data-in="${m.input_per_million}" data-out="${m.output_per_million}" ${ck ? 'checked' : ''} style="width:14px;height:14px;flex-shrink:0;cursor:pointer;accent-color:var(--accent);">
                                        <span style="flex:1;min-width:0;font-size:0.78rem;color:var(--text-primary);white-space:nowrap;overflow:hidden;text-overflow:ellipsis;" title="${escapeAttr(m.name)}">${escapeHtml(m.name)}</span>
                                        <span style="font-size:0.67rem;color:var(--text-tertiary);flex-shrink:0;font-family:monospace;">${escapeHtml(fmt(m.input_per_million))} · ${escapeHtml(fmt(m.output_per_million))}</span>
                                    </label>`;
                                }).join('')
                            }
                        </div>
                        <div style="padding:0.6rem 1rem;border-top:1px solid var(--border-subtle);display:flex;justify-content:space-between;align-items:center;gap:0.5rem;flex-shrink:0;">
                            <label style="display:flex;align-items:center;gap:0.4rem;font-size:0.75rem;color:var(--text-secondary);cursor:pointer;">
                                <input type="checkbox" id="pp-select-all" ${allVisibleSelected ? 'checked' : ''} style="cursor:pointer;accent-color:var(--accent);">
                                ${escapeHtml(t('config.providers.pricing_picker_select_all'))}
                            </label>
                            <div style="display:flex;gap:0.45rem;">
                                <button class="btn-save" style="padding:0.32rem 0.9rem;font-size:0.77rem;background:var(--bg-tertiary);color:var(--text-primary);" onclick="document.getElementById('pricing-picker-overlay').remove()">${escapeHtml(t('config.providers.cancel'))}</button>
                                <button class="btn-save" id="pp-confirm" style="padding:0.32rem 0.9rem;font-size:0.77rem;">${escapeHtml(t('config.providers.pricing_picker_import', { count: selCount }))}</button>
                            </div>
                        </div>
                    </div>`;

                // Wire: search
                const searchEl = document.getElementById('pricing-picker-search');
                if (searchEl) {
                    searchEl.focus();
                    let debounce;
                    searchEl.addEventListener('input', () => {
                        clearTimeout(debounce);
                        debounce = setTimeout(() => { filterQuery = searchEl.value; renderPicker(); }, 120);
                    });
                }

                // Wire: checkboxes — update selection + footer counts without full re-render
                document.querySelectorAll('#pricing-picker-list input[type=checkbox]').forEach(cb => {
                    cb.addEventListener('change', () => {
                        if (cb.checked) selected.add(cb.dataset.name); else selected.delete(cb.dataset.name);
                        const sc = document.getElementById('pp-sel-count');
                        if (sc) sc.textContent = selected.size + ' ' + t('config.providers.pricing_picker_selected');
                        const btn = document.getElementById('pp-confirm');
                        if (btn) btn.textContent = t('config.providers.pricing_picker_import', { count: selected.size });
                        const sa = document.getElementById('pp-select-all');
                        if (sa) { const v = getVisible(); sa.checked = v.length > 0 && v.every(m => selected.has(m.name)); }
                    });
                });

                // Wire: select all
                const saEl = document.getElementById('pp-select-all');
                if (saEl) {
                    saEl.addEventListener('change', () => {
                        getVisible().forEach(m => saEl.checked ? selected.add(m.name) : selected.delete(m.name));
                        renderPicker();
                    });
                }

                // Wire: confirm
                const confirmBtn = document.getElementById('pp-confirm');
                if (confirmBtn) {
                    confirmBtn.addEventListener('click', () => {
                        const toImport = allPricing.filter(m => selected.has(m.name));
                        for (const p of toImport) {
                            const ex = _provModalModels.find(e => e.name && e.name.toLowerCase() === p.name.toLowerCase());
                            if (ex) { ex.input_per_million = p.input_per_million; ex.output_per_million = p.output_per_million; }
                            else _provModalModels.push({ name: p.name, input_per_million: p.input_per_million, output_per_million: p.output_per_million });
                        }
                        overlay.remove();
                        providerRenderModelsTable();
                        showBudgetToast(t('config.providers.pricing_fetched', { count: toImport.length }));
                    });
                }
            }

            renderPicker();
            document.body.appendChild(overlay);
        }

        async function providerFetchPricing() {
            const provId = (document.getElementById('prov-id') || {}).value;
            const provType = (document.getElementById('prov-type') || {}).value;

            const btn = document.getElementById('prov-fetch-pricing-btn');
            if (btn) { btn.disabled = true; btn.textContent = '⏳ ' + t('config.providers.loading'); }

            try {
                let url;
                if (provId && providersCache.some(p => p.id === provId)) {
                    url = '/api/providers/pricing?id=' + encodeURIComponent(provId);
                } else {
                    url = '/api/openrouter/models';
                }

                const resp = await fetch(url);
                if (!resp.ok) throw new Error(await resp.text());
                const json = await resp.json();

                let pricing;
                if (json.data) {
                    const prefixMap = { openai: 'openai/', anthropic: 'anthropic/', google: 'google/' };
                    const prefix = prefixMap[provType] || '';
                    pricing = (json.data || [])
                        .filter(m => m.pricing && (prefix ? m.id.startsWith(prefix) : true))
                        .map(m => ({
                            name: prefix ? m.id.substring(prefix.length) : m.id,
                            input_per_million: parseFloat(m.pricing.prompt || '0') * 1000000,
                            output_per_million: parseFloat(m.pricing.completion || '0') * 1000000,
                        }));
                } else if (Array.isArray(json)) {
                    pricing = json.map(m => ({
                        name: m.model_id,
                        input_per_million: m.input_per_million,
                        output_per_million: m.output_per_million,
                    }));
                } else {
                    pricing = [];
                }

                // Open picker modal instead of bulk-importing
                openPricingPickerModal(pricing);
            } catch (e) {
                showBudgetToast('❌ ' + e.message);
            } finally {
                if (btn) { btn.disabled = false; btn.textContent = '📡 ' + t('config.providers.fetch_pricing'); }
            }
        }

        function renderMeshCentralTestBlock() {
            const labelTest = t('config.meshcentral.test_label');
            const labelDesc = t('config.meshcentral.test_desc');
            return `
            <div id="mc-test-block" style="margin-top:1.5rem;padding:1rem 1.2rem;border:1px solid var(--border-subtle);border-radius:12px;background:var(--bg-secondary);">
                <div style="font-size:0.8rem;font-weight:600;color:var(--accent);margin-bottom:0.5rem;">🔌 MeshCentral Test</div>
                <div style="font-size:0.78rem;color:var(--text-secondary);margin-bottom:0.8rem;">${labelDesc}</div>
                <div style="display:flex;gap:0.6rem;align-items:center;flex-wrap:wrap;">
                    <button class="btn-save" style="padding:0.4rem 1.1rem;font-size:0.78rem;" onclick="testMeshCentral()">${labelTest}</button>
                    <span id="mc-test-spinner" style="display:none;font-size:0.75rem;color:var(--text-secondary);">⏳ ${t('config.meshcentral.connecting')}</span>
                </div>
                <div id="mc-test-result" style="margin-top:0.8rem;display:none;">
                    <div id="mc-test-msg" style="font-size:0.82rem;padding:0.45rem 0.7rem;border-radius:7px;"></div>
                </div>
            </div>`;
        }

        async function testMeshCentral() {
            const spinner   = document.getElementById('mc-test-spinner');
            const resultDiv = document.getElementById('mc-test-result');
            const msgDiv    = document.getElementById('mc-test-msg');
            if (!spinner) return;

            spinner.style.display = 'inline';
            resultDiv.style.display = 'none';

            // Read values from the existing config fields on the page.
            // If the user edited them, those values are sent; otherwise they're
            // empty/masked and the backend falls back to the saved config.
            const getField = (path) => {
                const el = document.querySelector(`[data-path="${path}"]`);
                if (!el) return '';
                const v = el.value.trim();
                // Password placeholder '\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022' means unchanged — send empty so backend falls back to saved config
                if (el.type === 'password' && v === '\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022') return '';
                return v;
            };

            const body = {
                url:         getField('meshcentral.url'),
                username:    getField('meshcentral.username'),
                password:    getField('meshcentral.password'),
                login_token: getField('meshcentral.login_token'),
            };

            try {
                const resp = await fetch('/api/meshcentral/test', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(body),
                });
                const json = await resp.json();

                resultDiv.style.display = 'block';
                if (json.status === 'ok') {
                    msgDiv.style.background = 'rgba(34,197,94,0.12)';
                    msgDiv.style.color = 'var(--success, #22c55e)';
                    msgDiv.style.border = '1px solid rgba(34,197,94,0.3)';
                    msgDiv.textContent = '✅ ' + (json.message || t('config.meshcentral.success'));
                } else {
                    msgDiv.style.background = 'rgba(239,68,68,0.10)';
                    msgDiv.style.color = 'var(--danger, #ef4444)';
                    msgDiv.style.border = '1px solid rgba(239,68,68,0.25)';
                    msgDiv.textContent = '❌ ' + (json.message || t('config.meshcentral.failed'));
                }
            } catch (e) {
                resultDiv.style.display = 'block';
                msgDiv.style.background = 'rgba(239,68,68,0.10)';
                msgDiv.style.color = 'var(--danger, #ef4444)';
                msgDiv.style.border = '1px solid rgba(239,68,68,0.25)';
                msgDiv.textContent = '❌ ' + e.message;
            } finally {
                spinner.style.display = 'none';
            }
        }

        function renderProvidersSection(section) {
            let html = `<div class="cfg-section active">
                <div class="section-header">${section.icon} ${section.label}</div>
                <div class="section-desc">${section.desc}</div>
                <div style="display:flex;justify-content:flex-end;margin-bottom:1rem;">
                    <button class="btn-save" style="padding:0.45rem 1.1rem;font-size:0.82rem;" onclick="providerAdd()">
                        ＋ ${t('config.providers.new_provider')}
                    </button>
                </div>
                <div id="providers-list"></div>
                <div id="providers-empty" style="display:none;text-align:center;padding:2rem;color:var(--text-tertiary);font-size:0.85rem;">
                    ${t('config.providers.empty')}
                </div>
            </div>`;
            document.getElementById('content').innerHTML = html;
            providerRenderCards();
        }

        function providerRenderCards() {
            const wrap = document.getElementById('providers-list');
            const empty = document.getElementById('providers-empty');
            if (!wrap) return;
            if (providersCache.length === 0) {
                wrap.innerHTML = '';
                if (empty) empty.style.display = '';
                return;
            }
            if (empty) empty.style.display = 'none';

            let html = '';
            providersCache.forEach((p, idx) => {
                const typeBadge = p.type
                    ? `<span style="display:inline-block;padding:0.15rem 0.5rem;border-radius:6px;font-size:0.7rem;font-weight:600;background:var(--accent);color:#fff;margin-left:0.4rem;">${escapeAttr(p.type)}</span>`
                    : '';
                const isOAuth = p.auth_type === 'oauth2';
                const authBadge = isOAuth
                    ? '<span style="display:inline-block;padding:0.15rem 0.5rem;border-radius:6px;font-size:0.7rem;font-weight:600;background:#8e44ad;color:#fff;margin-left:0.4rem;">🔑 OAuth2</span>'
                    : '';
                let authInfo;
                if (isOAuth) {
                    authInfo = `<span style="color:var(--text-tertiary);" id="oauth-status-${idx}">🔑 OAuth2</span>`;
                } else {
                    const maskedKey = p.api_key === '••••••••'
                        ? '<span style="color:var(--text-tertiary);">••••••••</span>'
                        : (p.api_key ? '<span style="color:var(--success);">' + t('config.providers.key_set') + '</span>' : '<span style="color:var(--text-tertiary);">—</span>');
                    authInfo = maskedKey;
                }
                html += `
                <div class="provider-card" data-idx="${idx}" style="border:1px solid var(--border-subtle);border-radius:12px;padding:1rem 1.2rem;margin-bottom:0.75rem;background:var(--bg-secondary);transition:border-color 0.15s;">
                    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:0.6rem;">
                        <div style="font-weight:600;font-size:0.9rem;">
                            ${escapeAttr(p.name || p.id)}${typeBadge}${authBadge}
                            <span style="font-size:0.72rem;color:var(--text-tertiary);margin-left:0.5rem;">ID: ${escapeAttr(p.id)}</span>
                        </div>
                        <div style="display:flex;gap:0.4rem;">
                            <button onclick="providerEdit(${idx})" style="background:none;border:none;cursor:pointer;color:var(--accent);font-size:0.85rem;" title="Edit">✏️</button>
                            <button onclick="providerDelete(${idx})" style="background:none;border:none;cursor:pointer;color:var(--danger);font-size:0.85rem;" title="Delete">🗑️</button>
                        </div>
                    </div>
                    <div style="display:grid;grid-template-columns:1fr 1fr;gap:0.3rem 1rem;font-size:0.78rem;">
                        <div><span style="color:var(--text-tertiary);">Base URL:</span> ${escapeAttr(p.base_url || '—')}</div>
                        <div><span style="color:var(--text-tertiary);">Model:</span> ${escapeAttr(p.model || '—')}</div>
                        <div><span style="color:var(--text-tertiary);">${isOAuth ? 'Auth:' : 'API Key:'}</span> ${authInfo}</div>
                    </div>
                </div>`;
            });
            wrap.innerHTML = html;

            // Async: fetch OAuth status for OAuth2 providers
            providersCache.forEach((p, idx) => {
                if (p.auth_type === 'oauth2') {
                    fetch('/api/oauth/status?provider=' + encodeURIComponent(p.id))
                        .then(r => r.json())
                        .then(st => {
                            const el = document.getElementById('oauth-status-' + idx);
                            if (!el) return;
                            if (st.authorized && !st.expired) {
                                el.innerHTML = '<span style="color:var(--success);">✅ ' + t('config.providers.authorized') + '</span>';
                            } else if (st.authorized && st.expired) {
                                el.innerHTML = '<span style="color:var(--warning);">⚠️ ' + t('config.providers.token_expired') + '</span>';
                            } else {
                                el.innerHTML = '<span style="color:var(--danger);">❌ ' + t('config.providers.not_authorized') + '</span>';
                            }
                        }).catch(() => {});
                }
            });
        }

        // Known base URLs for auto-fill when creating new providers
        const PROVIDER_BASE_URLS = {
            openrouter: 'https://openrouter.ai/api/v1',
            openai: 'https://api.openai.com/v1',
            ollama: 'http://localhost:11434',
            anthropic: 'https://api.anthropic.com/v1',
            google: 'https://generativelanguage.googleapis.com/v1beta/openai',
            custom: ''
        };

        const PROVIDER_HINTS = {
            openrouter: 'config.providers.hint.openrouter',
            ollama: 'config.providers.hint.ollama',
            anthropic: 'config.providers.hint.anthropic',
            google: 'config.providers.hint.google',
            openai: 'config.providers.hint.openai',
            custom: 'config.providers.hint.custom'
        };

        function providerShowModal(title, data, onSave) {
            // Remove existing modal
            const existing = document.getElementById('provider-modal-overlay');
            if (existing) existing.remove();

            const overlay = document.createElement('div');
            overlay.id = 'provider-modal-overlay';
            overlay.style.cssText = 'position:fixed;inset:0;background:rgba(0,0,0,0.55);z-index:1000;backdrop-filter:blur(4px);display:flex;align-items:center;justify-content:center;';
            overlay.onclick = (e) => { if (e.target === overlay) overlay.remove(); };

            const currentAuthType = data.auth_type || 'api_key';
            const isOAuth = currentAuthType === 'oauth2';

            overlay.innerHTML = `
            <div style="background:var(--bg-secondary);border-radius:16px;padding:1.5rem;width:min(520px,92vw);max-height:85vh;overflow-y:auto;border:1px solid var(--border-subtle);" onclick="event.stopPropagation()">
                <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:1.2rem;">
                    <div style="font-weight:700;font-size:1rem;">${title}</div>
                    <button onclick="document.getElementById('provider-modal-overlay').remove()" style="background:none;border:none;color:var(--text-secondary);font-size:1.2rem;cursor:pointer;">✕</button>
                </div>
                <div class="field-group">
                    <div class="field-label">ID</div>
                    <div class="field-help">${t('config.providers.id_help')}</div>
                    <input class="field-input" id="prov-id" value="${escapeAttr(data.id || '')}" placeholder="my-provider" ${data._editMode ? 'disabled style="opacity:0.55;cursor:not-allowed;"' : ''}>
                </div>
                <div class="field-group">
                    <div class="field-label">Name</div>
                    <input class="field-input" id="prov-name" value="${escapeAttr(data.name || '')}" placeholder="${t('config.providers.display_name')}">
                </div>
                <div class="field-group">
                    <div class="field-label">Type</div>
                    <div class="field-help">${t('config.providers.type_help')}</div>
                    <select class="field-select" id="prov-type">
                        ${['openai','openrouter','ollama','anthropic','google','custom'].map(typ =>
                            `<option value="${typ}"${data.type === typ ? ' selected' : ''}>${typ}</option>`
                        ).join('')}
                    </select>
                </div>
                <div class="field-group">
                    <div class="field-label">Base URL</div>
                    <input class="field-input" id="prov-url" value="${escapeAttr(data.base_url || '')}" placeholder="${PROVIDER_BASE_URLS[data.type] || PROVIDER_BASE_URLS.openrouter}">
                </div>
                <div class="field-group">
                    <div class="field-label">Model</div>
                    <input class="field-input" id="prov-model" value="${escapeAttr(data.model || '')}" placeholder="gpt-4o / llama3 / ...">
                </div>

                <!-- Ollama model query (only visible when type = ollama) -->
                <div id="prov-ollama-block" class="field-group" style="display:${(data.type || 'openai') === 'ollama' ? 'block' : 'none'};margin-top:0;padding:0.7rem 0.9rem;border-radius:9px;background:rgba(99,179,237,0.06);border:1px solid rgba(99,179,237,0.18);">
                    <div style="display:flex;gap:0.6rem;align-items:center;flex-wrap:wrap;">
                        <button type="button" class="btn-save" style="padding:0.3rem 0.9rem;font-size:0.75rem;" onclick="queryOllamaModelsInModal()">
                            🦙 ${t('config.providers.query_models')}
                        </button>
                        <span id="prov-ollama-spinner" style="display:none;font-size:0.72rem;color:var(--text-secondary);">⏳ ${t('config.providers.loading')}</span>
                    </div>
                    <div id="prov-ollama-result" style="margin-top:0.6rem;display:none;">
                        <div style="font-size:0.72rem;font-weight:600;color:var(--text-secondary);margin-bottom:0.35rem;">${t('config.providers.available_models')}</div>
                        <div id="prov-ollama-models" style="display:flex;flex-wrap:wrap;gap:0.35rem;"></div>
                        <div id="prov-ollama-error" style="font-size:0.75rem;color:var(--danger);margin-top:0.35rem;display:none;"></div>
                    </div>
                </div>

                <!-- OpenRouter model browser (only visible when type = openrouter) -->
                <div id="prov-openrouter-block" class="field-group" style="display:${(data.type || 'openai') === 'openrouter' ? 'block' : 'none'};margin-top:0;padding:0.7rem 0.9rem;border-radius:9px;background:rgba(72,199,142,0.06);border:1px solid rgba(72,199,142,0.18);">
                    <div style="display:flex;gap:0.6rem;align-items:center;flex-wrap:wrap;">
                        <button type="button" class="btn-save" style="padding:0.3rem 0.9rem;font-size:0.75rem;background:rgba(72,199,142,0.15);color:#48c78e;border:1px solid rgba(72,199,142,0.3);" onclick="openOpenRouterBrowser(function(m){document.getElementById('prov-model').value=m.id;document.getElementById('prov-model').dispatchEvent(new Event('input',{bubbles:true}));updateProviderModelCost(m);})">
                            🌐 ${t('config.providers.open_model_browser')}
                        </button>
                        <span style="font-size:0.72rem;color:var(--text-tertiary);">${t('config.providers.browse_openrouter')}</span>
                    </div>
                </div>

                <!-- Model Pricing Table -->
                <div class="field-group" style="margin-top:0.8rem;padding-top:0.8rem;border-top:1px solid var(--border-subtle);">
                    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:0.5rem;">
                        <div class="field-label" style="margin-bottom:0;">💰 ${t('config.providers.model_pricing')}</div>
                        <div style="display:flex;gap:0.4rem;">
                            <button type="button" class="btn-save" id="prov-fetch-pricing-btn" style="padding:0.25rem 0.65rem;font-size:0.72rem;background:rgba(72,199,142,0.12);color:#48c78e;border:1px solid rgba(72,199,142,0.25);display:${['openrouter','openai','anthropic','google','ollama'].includes(data.type || 'openai') ? 'inline-block' : 'none'};">
                                📡 ${t('config.providers.fetch_pricing')}
                            </button>
                            <button type="button" class="btn-save" style="padding:0.25rem 0.65rem;font-size:0.72rem;background:var(--bg-tertiary);color:var(--text-primary);" onclick="providerAddModelRow()">
                                + ${t('config.providers.add_model')}
                            </button>
                        </div>
                    </div>
                    <div id="prov-models-table-wrap">
                        <div id="prov-models-search-wrap" style="display:none;margin-bottom:0.4rem;">
                            <input class="field-input" id="prov-models-filter" type="search" autocomplete="off"
                                   placeholder="🔍 ${escapeHtml(t('config.providers.filter_models'))}"
                                   style="width:100%;font-size:0.77rem;padding:0.28rem 0.5rem;"
                                   oninput="providerRenderModelsTable()">
                            <div id="prov-models-count" style="font-size:0.7rem;color:var(--text-tertiary);text-align:right;margin-top:0.18rem;display:none;"></div>
                        </div>
                        <table style="width:100%;border-collapse:collapse;font-size:0.78rem;" id="prov-models-table">
                            <thead>
                                <tr style="border-bottom:1px solid var(--border-subtle);color:var(--text-tertiary);font-size:0.7rem;">
                                    <th style="text-align:left;padding:0.3rem 0.4rem;">${t('config.providers.model_name')}</th>
                                    <th style="text-align:right;padding:0.3rem 0.4rem;">${t('config.providers.input_cost')}</th>
                                    <th style="text-align:right;padding:0.3rem 0.4rem;">${t('config.providers.output_cost')}</th>
                                    <th style="width:2rem;"></th>
                                </tr>
                            </thead>
                            <tbody id="prov-models-body"></tbody>
                        </table>
                        <div id="prov-models-empty" style="text-align:center;padding:0.8rem;color:var(--text-tertiary);font-size:0.75rem;display:none;">
                            ${t('config.providers.no_models_configured')}
                        </div>
                    </div>
                </div>

                <!-- Auth Type Toggle -->
                <div class="field-group" style="margin-top:0.8rem;padding-top:0.8rem;border-top:1px solid var(--border-subtle);">
                    <div class="field-label">${t('config.providers.authentication')}</div>
                    <select class="field-select" id="prov-auth-type">
                        <option value="api_key"${currentAuthType !== 'oauth2' ? ' selected' : ''}>🔑 API Key</option>
                        <option value="oauth2"${currentAuthType === 'oauth2' ? ' selected' : ''}>🔐 OAuth2 Authorization Code</option>
                    </select>
                </div>

                <!-- API Key section (visible when auth_type = api_key) -->
                <div id="prov-apikey-section" style="display:${isOAuth ? 'none' : 'block'};">
                    <div class="field-group">
                        <div class="field-label">API Key</div>
                        <div id="prov-key-hint" style="font-size:0.72rem;color:var(--text-tertiary);margin-bottom:0.3rem;">${PROVIDER_HINTS[data.type || 'openai'] ? t(PROVIDER_HINTS[data.type || 'openai']) : t(PROVIDER_HINTS.openai)}</div>
                        <div class="password-wrap">
                            <input class="field-input" id="prov-key" type="password" value="${escapeAttr(data.api_key === '••••••••' ? '' : (data.api_key || ''))}" placeholder="${data.api_key === '••••••••' ? t('config.providers.key_placeholder_existing') : 'sk-...'}" autocomplete="off">
                            <button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">${EYE_OPEN_SVG}</button>
                        </div>
                        ${data.api_key === '••••••••' ? `<div style="font-size:0.72rem;color:var(--text-tertiary);margin-top:0.2rem;">${t('config.providers.keep_existing_key')}</div>` : ''}
                    </div>
                </div>

                <!-- OAuth2 section (visible when auth_type = oauth2) -->
                <div id="prov-oauth-section" style="display:${isOAuth ? 'block' : 'none'};">
                    <div class="field-group">
                        <div class="field-label">Authorization URL</div>
                        <div class="field-help">${t('config.providers.oauth_auth_url_help')}</div>
                        <input class="field-input" id="prov-oauth-auth-url" value="${escapeAttr(data.oauth_auth_url || '')}" placeholder="https://accounts.google.com/o/oauth2/v2/auth">
                    </div>
                    <div class="field-group">
                        <div class="field-label">Token URL</div>
                        <div class="field-help">${t('config.providers.oauth_token_url_help')}</div>
                        <input class="field-input" id="prov-oauth-token-url" value="${escapeAttr(data.oauth_token_url || '')}" placeholder="https://oauth2.googleapis.com/token">
                    </div>
                    <div class="field-group">
                        <div class="field-label">Client ID</div>
                        <input class="field-input" id="prov-oauth-client-id" value="${escapeAttr(data.oauth_client_id || '')}" placeholder="123456789.apps.googleusercontent.com">
                    </div>
                    <div class="field-group">
                        <div class="field-label">Client Secret</div>
                        <div class="password-wrap">
                            <input class="field-input" id="prov-oauth-client-secret" type="password" value="${escapeAttr(data.oauth_client_secret === '••••••••' ? '' : (data.oauth_client_secret || ''))}" placeholder="${data.oauth_client_secret === '••••••••' ? t('config.providers.key_placeholder_existing') : 'GOCSPX-...'}" autocomplete="off">
                            <button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">${EYE_OPEN_SVG}</button>
                        </div>
                        ${data.oauth_client_secret === '••••••••' ? `<div style="font-size:0.72rem;color:var(--text-tertiary);margin-top:0.2rem;">${t('config.providers.keep_existing_secret')}</div>` : ''}
                    </div>
                    <div class="field-group">
                        <div class="field-label">Scopes</div>
                        <div class="field-help">${t('config.providers.oauth_scopes_help')}</div>
                        <input class="field-input" id="prov-oauth-scopes" value="${escapeAttr(data.oauth_scopes || '')}" placeholder="openid email https://www.googleapis.com/auth/cloud-platform">
                    </div>
                    ${data._editMode && currentAuthType === 'oauth2' ? `
                    <div class="field-group" style="margin-top:0.5rem;">
                        <div id="prov-oauth-status" style="font-size:0.8rem;color:var(--text-tertiary);">⏳ ${t('config.providers.checking_status')}</div>
                        <div style="display:flex;gap:0.5rem;margin-top:0.5rem;">
                            <button class="btn-save" style="padding:0.35rem 1rem;font-size:0.78rem;background:#8e44ad;" id="prov-oauth-authorize-btn">
                                🔐 ${t('config.providers.authorize')}
                            </button>
                            <button class="btn-save" style="padding:0.35rem 1rem;font-size:0.78rem;background:var(--danger);" id="prov-oauth-revoke-btn">
                                🗑️ ${t('config.providers.revoke_token')}
                            </button>
                        </div>
                    </div>
                    ` : ''}
                </div>

                <div style="display:flex;justify-content:flex-end;gap:0.6rem;margin-top:1.2rem;">
                    <button class="btn-save" style="padding:0.45rem 1.4rem;font-size:0.82rem;background:var(--bg-tertiary);color:var(--text-primary);" onclick="document.getElementById('provider-modal-overlay').remove()">
                        ${t('config.providers.cancel')}
                    </button>
                    <button class="btn-save" style="padding:0.45rem 1.4rem;font-size:0.82rem;" id="prov-save-btn">
                        ${t('config.providers.save')}
                    </button>
                </div>
            </div>`;
            document.body.appendChild(overlay);

            // ── Initialize model pricing table ──
            providerInitModelsTable(data.models);

            // ── Wire up fetch pricing button ──
            const fetchPricingBtn = document.getElementById('prov-fetch-pricing-btn');
            if (fetchPricingBtn) fetchPricingBtn.onclick = () => providerFetchPricing();

            // ── Auto-fill Base URL when type changes (only if URL field is empty or was auto-filled) ──
            const typeSelect = document.getElementById('prov-type');
            const urlInput = document.getElementById('prov-url');
            const hintEl = document.getElementById('prov-key-hint');
            const ollamaBlock = document.getElementById('prov-ollama-block');
            const openrouterBlock = document.getElementById('prov-openrouter-block');
            const knownUrls = new Set(Object.values(PROVIDER_BASE_URLS).filter(Boolean));
            typeSelect.addEventListener('change', () => {
                const typ = typeSelect.value;
                const currentUrl = urlInput.value.trim();
                // Auto-fill URL when empty OR when it still contains a known default URL (i.e. user hasn't typed a custom one)
                if ((!currentUrl || knownUrls.has(currentUrl)) && PROVIDER_BASE_URLS[typ]) {
                    urlInput.value = PROVIDER_BASE_URLS[typ];
                }
                // Update placeholder
                urlInput.placeholder = PROVIDER_BASE_URLS[typ] || 'https://...';
                // Update hint
                if (hintEl) {
                    const hintKey = PROVIDER_HINTS[typ];
                    hintEl.textContent = hintKey ? t(hintKey) : '';
                }
                // Show/hide Ollama model query block
                if (ollamaBlock) ollamaBlock.style.display = typ === 'ollama' ? 'block' : 'none';
                // Show/hide OpenRouter model browser block
                if (openrouterBlock) openrouterBlock.style.display = typ === 'openrouter' ? 'block' : 'none';
                // Show/hide fetch pricing button
                if (fetchPricingBtn) {
                    fetchPricingBtn.style.display = ['openrouter','openai','anthropic','google','ollama'].includes(typ) ? 'inline-block' : 'none';
                }
            });

            // ── Auth type toggle ──
            const authTypeSelect = document.getElementById('prov-auth-type');
            const apikeySection = document.getElementById('prov-apikey-section');
            const oauthSection = document.getElementById('prov-oauth-section');
            authTypeSelect.addEventListener('change', () => {
                const isOA = authTypeSelect.value === 'oauth2';
                apikeySection.style.display = isOA ? 'none' : 'block';
                oauthSection.style.display = isOA ? 'block' : 'none';
            });

            // ── OAuth Authorize button ──
            const authBtn = document.getElementById('prov-oauth-authorize-btn');
            if (authBtn) {
                authBtn.onclick = async () => {
                    try {
                        const resp = await fetch('/api/oauth/start?provider=' + encodeURIComponent(data.id));
                        if (!resp.ok) { alert(await resp.text()); return; }
                        const result = await resp.json();
                        if (result.auth_url) {
                            window.open(result.auth_url, '_blank', 'width=600,height=700');
                        }
                    } catch (e) { alert('Error: ' + e.message); }
                };
            }

            // ── OAuth Revoke button ──
            const revokeBtn = document.getElementById('prov-oauth-revoke-btn');
            if (revokeBtn) {
                revokeBtn.onclick = async () => {
                    if (!confirm(t('config.providers.revoke_confirm'))) return;
                    try {
                        await fetch('/api/oauth/revoke?provider=' + encodeURIComponent(data.id), { method: 'DELETE' });
                        const statusEl = document.getElementById('prov-oauth-status');
                        if (statusEl) statusEl.innerHTML = '<span style="color:var(--danger);">❌ ' + t('config.providers.not_authorized') + '</span>';
                    } catch (e) { alert('Error: ' + e.message); }
                };
            }

            // ── Fetch OAuth status for edit mode ──
            if (data._editMode && currentAuthType === 'oauth2' && data.id) {
                fetch('/api/oauth/status?provider=' + encodeURIComponent(data.id))
                    .then(r => r.json())
                    .then(st => {
                        const statusEl = document.getElementById('prov-oauth-status');
                        if (!statusEl) return;
                        if (st.authorized && !st.expired) {
                            statusEl.innerHTML = '<span style="color:var(--success);">✅ ' + t('config.providers.authorized') + '</span>'
                                + (st.expiry ? `<span style="margin-left:0.5rem;font-size:0.72rem;color:var(--text-tertiary);">(${t('config.providers.expires')}: ${new Date(st.expiry).toLocaleString()})</span>` : '');
                        } else if (st.authorized && st.expired) {
                            statusEl.innerHTML = '<span style="color:var(--warning);">⚠️ ' + t('config.providers.token_expired_reauth') + '</span>';
                        } else {
                            statusEl.innerHTML = '<span style="color:var(--danger);">❌ ' + t('config.providers.not_authorized_click') + '</span>';
                        }
                    }).catch(() => {});
            }

            // ── Save handler ──
            document.getElementById('prov-save-btn').onclick = () => {
                const id = document.getElementById('prov-id').value.trim();
                const name = document.getElementById('prov-name').value.trim();
                const type = document.getElementById('prov-type').value;
                const base_url = document.getElementById('prov-url').value.trim();
                const model = document.getElementById('prov-model').value.trim();
                const auth_type = document.getElementById('prov-auth-type').value;

                if (!id) { alert(t('config.providers.id_empty_error')); return; }
                if (!base_url) { alert(t('config.providers.url_empty_error')); return; }

                const entry = { id, name: name || id, type, base_url, model, auth_type };

                if (auth_type === 'oauth2') {
                    entry.api_key = data.api_key === '••••••••' ? '••••••••' : '';
                    entry.oauth_auth_url = document.getElementById('prov-oauth-auth-url').value.trim();
                    entry.oauth_token_url = document.getElementById('prov-oauth-token-url').value.trim();
                    entry.oauth_client_id = document.getElementById('prov-oauth-client-id').value.trim();
                    let client_secret = document.getElementById('prov-oauth-client-secret').value.trim();
                    if (!client_secret && data.oauth_client_secret === '••••••••') client_secret = '••••••••';
                    entry.oauth_client_secret = client_secret;
                    entry.oauth_scopes = document.getElementById('prov-oauth-scopes').value.trim();

                    if (!entry.oauth_auth_url || !entry.oauth_token_url || !entry.oauth_client_id) {
                        alert(t('config.providers.oauth_required_error'));
                        return;
                    }
                } else {
                    let api_key = document.getElementById('prov-key').value.trim();
                    if (!api_key && data.api_key === '••••••••') api_key = '••••••••';
                    entry.api_key = api_key;
                    // Clear OAuth fields
                    entry.oauth_auth_url = '';
                    entry.oauth_token_url = '';
                    entry.oauth_client_id = '';
                    entry.oauth_client_secret = '';
                    entry.oauth_scopes = '';
                }

                // Include model pricing data
                entry.models = providerGetModels();

                onSave(entry);
                overlay.remove();
            };

            // Focus first editable field
            setTimeout(() => {
                const focus = data._editMode ? document.getElementById('prov-name') : document.getElementById('prov-id');
                if (focus) focus.focus();
            }, 50);

            // Trigger initial auto-fill for new providers
            if (!data._editMode && !data.base_url) {
                const initType = typeSelect.value;
                if (PROVIDER_BASE_URLS[initType]) {
                    urlInput.value = PROVIDER_BASE_URLS[initType];
                }
            }
        }

        function providerAdd() {
            providerShowModal(
                t('config.providers.new_provider'),
                {},
                async (entry) => {
                    // Check unique ID
                    if (providersCache.some(p => p.id === entry.id)) {
                        alert(t('config.providers.id_exists'));
                        return;
                    }
                    providersCache.push(entry);
                    await providerSave();
                }
            );
        }

        function providerEdit(idx) {
            const p = { ...providersCache[idx], _editMode: true };
            providerShowModal(
                t('config.providers.edit_provider'),
                p,
                async (entry) => {
                    providersCache[idx] = entry;
                    await providerSave();
                }
            );
        }

        function providerDelete(idx) {
            const p = providersCache[idx];
            if (!confirm(t('config.providers.delete_confirm', { name: p.name || p.id }))) return;
            providersCache.splice(idx, 1);
            providerSave();
        }

        async function providerSave() {
            try {
                const resp = await fetch('/api/providers', {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(providersCache)
                });
                if (!resp.ok) {
                    const txt = await resp.text();
                    alert('Error: ' + txt);
                    return;
                }
                // Reload providers from server (API keys will be masked)
                const reload = await fetch('/api/providers');
                if (reload.ok) providersCache = await reload.json();
                providerRenderCards();
            } catch (e) {
                alert('Error: ' + e.message);
            }
        }
