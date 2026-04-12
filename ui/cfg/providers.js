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

            setHidden(spinner, false);
            setHidden(resultDiv, true);
            setHidden(errorDiv, true);
            modelsDiv.innerHTML = '';

            const baseUrl = (document.getElementById('prov-url') || {}).value || '';
            const url = '/api/ollama/models' + (baseUrl ? '?url=' + encodeURIComponent(baseUrl) : '');

            try {
                const resp = await fetch(url);
                const json = await resp.json();
                setHidden(resultDiv, false);

                if (!resp.ok || json.available === false) {
                    errorDiv.textContent = json.reason || t('config.ollama.fetch_error');
                    setHidden(errorDiv, false);
                } else if (!json.models || json.models.length === 0) {
                    errorDiv.textContent = t('config.ollama.no_models');
                    setHidden(errorDiv, false);
                } else {
                    modelsDiv.innerHTML = json.models.map(m =>
                        `<button type="button" class="prov-ollama-chip" title="${t('config.ollama.click_to_apply')}" onclick="applyOllamaModelInModal(this.textContent)">${escapeHtml(m)}</button>`
                    ).join('');
                }
            } catch (e) {
                setHidden(resultDiv, false);
                errorDiv.textContent = t('config.ollama.connection_error') + e.message;
                setHidden(errorDiv, false);
            } finally {
                setHidden(spinner, true);
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
            overlay.className = 'prov-or-overlay';
            overlay.onclick = (e) => { if (e.target === overlay) overlay.remove(); };

            overlay.innerHTML = `
            <div class="prov-or-panel" onclick="event.stopPropagation()">
                <!-- Header -->
                <div class="prov-or-header">
                    <div class="prov-or-header-row">
                        <div class="prov-or-title">🌐 OpenRouter ${t('config.openrouter.model_browser')}</div>
                        <button onclick="document.getElementById('or-browser-overlay').remove()" class="prov-or-close">✕</button>
                    </div>
                    <div class="prov-or-controls">
                        <input id="or-search" class="field-input prov-or-search" type="text" placeholder="${t('config.openrouter.search_placeholder')}">
                        <button id="or-free-btn" class="btn-save prov-or-free-btn" title="${t('config.openrouter.free_tooltip')}">
                            🆓 ${t('config.openrouter.free_button')}
                        </button>
                        <div id="or-count" class="prov-or-count"></div>
                    </div>
                </div>
                <!-- Content area: list + detail -->
                <div class="prov-or-content">
                    <!-- Model list -->
                    <div id="or-list-wrap" class="prov-or-list-wrap is-hidden">
                        <div id="or-list" class="prov-or-list"></div>
                    </div>
                    <!-- Detail panel (hidden by default) -->
                    <div id="or-detail" class="prov-or-detail is-hidden"></div>
                </div>
                <!-- Loading / error -->
                <div id="or-loading" class="prov-or-loading">
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
                        loadingDiv.innerHTML = '<span class="prov-text-danger">❌ ' + (json.reason || 'Error') + '</span>';
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
                loadingDiv.innerHTML = '<span class="prov-text-danger">❌ ' + t('config.openrouter.connection_error') + e.message + '</span>';
                return;
            }
            setHidden(loadingDiv, true);
            setHidden(listWrap, false);

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
                    listDiv.innerHTML = '<div class="prov-or-empty">' + t('config.openrouter.no_models') + '</div>';
                    return;
                }

                // Virtual-ish rendering: just render all since DOM handles it fine with simple divs
                listDiv.innerHTML = filtered.map(m => {
                    const free = isFree(m);
                    const promptCost = formatCost(m.pricing.prompt);
                    const completionCost = formatCost(m.pricing.completion);
                    const isSelected = selectedModel && selectedModel.id === m.id;
                    return `<div class="or-model-row ${isSelected ? 'is-selected' : ''}" data-id="${escapeAttr(m.id)}">
                        <div class="prov-or-row-main">
                            <div class="prov-or-row-name" title="${escapeAttr(m.id)}">${escapeHtml(m.name)}</div>
                            <div class="prov-or-row-id">${escapeHtml(m.id)}</div>
                        </div>
                        <div class="prov-or-row-meta">
                            ${free ? '<span class="prov-or-free-pill">' + t('config.providers.free_badge') + '</span>' : '<span class="prov-or-meta-text" title="' + escapeAttr(t('config.providers.cost_input_output')) + '">'+promptCost+' · '+completionCost+'</span>'}
                            <span class="prov-or-meta-text" title="${escapeAttr(t('config.providers.context_label'))}">${formatContext(m.context_length)}</span>
                        </div>
                    </div>`;
                }).join('');

                // Attach click handlers
                listDiv.querySelectorAll('.or-model-row').forEach(row => {
                    row.onclick = () => {
                        const m = allModels.find(x => x.id === row.dataset.id);
                        if (!m) return;
                        selectedModel = m;
                        showDetail(m);
                        // Highlight selected
                        listDiv.querySelectorAll('.or-model-row').forEach(r => r.classList.remove('is-selected'));
                        row.classList.add('is-selected');
                    };
                });
            }

            function showDetail(m) {
                detailDiv.classList.remove('is-hidden');
                const free = isFree(m);
                const promptPerM = (parseFloat(m.pricing.prompt || '0') * 1000000);
                const completionPerM = (parseFloat(m.pricing.completion || '0') * 1000000);
                detailDiv.innerHTML = `
                    <div class="prov-or-detail-title">${escapeHtml(m.name)}</div>
                    <div class="prov-or-detail-id">${escapeHtml(m.id)}</div>
                    ${m.description ? '<div class="prov-or-detail-desc">' + escapeHtml(m.description) + '</div>' : ''}
                    <div class="prov-or-detail-grid">
                        <div><span class="prov-or-detail-label">${t('config.providers.context_label')}:</span></div>
                        <div class="prov-or-detail-value">${formatContext(m.context_length)} ${t('config.providers.tokens_suffix')}</div>
                        <div><span class="prov-or-detail-label">${t('config.providers.input_label')}:</span></div>
                        <div class="prov-or-detail-value ${free ? 'is-free' : ''}">${free ? t('config.openrouter.free_cost') : '$'+promptPerM.toFixed(4)+'/M'}</div>
                        <div><span class="prov-or-detail-label">${t('config.providers.output_label')}:</span></div>
                        <div class="prov-or-detail-value ${free ? 'is-free' : ''}">${free ? t('config.openrouter.free_cost') : '$'+completionPerM.toFixed(4)+'/M'}</div>
                    </div>
                    ${free ? '<div class="prov-or-free-banner">🆓 '+t('config.openrouter.model_is_free')+'</div>' : ''}
                    <button id="or-apply-btn" class="btn-save prov-or-apply-btn">
                        ✅ ${t('config.openrouter.apply_model')}
                    </button>
                    <div class="prov-or-detail-footnote">
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
                freeBtn.classList.toggle('is-active', freeOnly);
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
            showToast(msg, 'info');
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
            if (searchWrap) setHidden(searchWrap, _provModalModels.length < 5);

            const filterQ = filterEl ? filterEl.value.toLowerCase().trim() : '';
            const visible = filterQ
                ? _provModalModels.filter(m => m.name && m.name.toLowerCase().includes(filterQ))
                : _provModalModels;

            if (countEl && _provModalModels.length > 0) {
                countEl.textContent = filterQ
                    ? `${visible.length} / ${_provModalModels.length}`
                    : `${_provModalModels.length} ${t('config.providers.pricing_picker_total')}`;
                setHidden(countEl, false);
            } else if (countEl) {
                setHidden(countEl, true);
            }

            if (_provModalModels.length === 0) {
                tbody.innerHTML = '';
                if (empty) setHidden(empty, false);
                return;
            }
            if (empty) setHidden(empty, true);

            if (visible.length === 0) {
                tbody.innerHTML = `<tr><td colspan="4" class="prov-no-filter-results">${escapeHtml(t('config.providers.no_filter_results', { q: filterQ }))}</td></tr>`;
                return;
            }

            tbody.innerHTML = visible.map(m => {
                const i = _provModalModels.indexOf(m);
                return `
                <tr class="prov-model-row">
                    <td class="prov-model-cell">
                        <input class="field-input prov-model-input" value="${escapeAttr(m.name)}" onchange="providerUpdateModelRow(${i},'name',this.value)">
                    </td>
                    <td class="prov-model-cell">
                        <input class="field-input prov-model-input prov-model-input-num" type="number" step="0.001" min="0" value="${m.input_per_million}" onchange="providerUpdateModelRow(${i},'input_per_million',parseFloat(this.value)||0)">
                    </td>
                    <td class="prov-model-cell">
                        <input class="field-input prov-model-input prov-model-input-num" type="number" step="0.001" min="0" value="${m.output_per_million}" onchange="providerUpdateModelRow(${i},'output_per_million',parseFloat(this.value)||0)">
                    </td>
                    <td class="prov-model-cell prov-model-cell-del">
                        <button type="button" class="prov-model-del-btn" onclick="providerDeleteModelRow(${i})" title="${t('config.providers.card_delete_tooltip')}">✕</button>
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
            showToast(t('config.budget.model_cost_updated', { model: m.id }));
        }

        /**
         * Opens a searchable picker modal to select which models/prices to import.
         * @param {Array<{name,input_per_million,output_per_million}>} allPricing
         */
        function openPricingPickerModal(allPricing) {
            if (!allPricing || allPricing.length === 0) {
                showToast(t('config.providers.no_pricing_found'));
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
            overlay.className = 'prov-pricing-overlay';

            function getVisible() {
                const q = filterQuery.toLowerCase();
                return q ? allPricing.filter(m => m.name.toLowerCase().includes(q)) : allPricing;
            }

            function renderPicker() {
                const visible = getVisible();
                const selCount = selected.size;
                const allVisibleSelected = visible.length > 0 && visible.every(m => selected.has(m.name));

                overlay.innerHTML = `
                    <div class="prov-pricing-panel">
                        <div class="prov-pricing-head">
                            <div class="prov-pricing-title">📡 ${escapeHtml(t('config.providers.pricing_picker_title'))}</div>
                            <input id="pricing-picker-search" class="field-input" type="search" autocomplete="off"
                                   placeholder="🔍 ${escapeHtml(t('config.providers.pricing_picker_search'))}"
                                   value="${escapeAttr(filterQuery)}">
                            <div class="prov-pricing-meta">
                                <span class="prov-pricing-total">${visible.length} / ${allPricing.length} ${escapeHtml(t('config.providers.pricing_picker_total'))}</span>
                                <span id="pp-sel-count" class="prov-pricing-selected">${selCount} ${escapeHtml(t('config.providers.pricing_picker_selected'))}</span>
                            </div>
                        </div>
                        <div class="prov-pricing-list" id="pricing-picker-list">
                            ${visible.length === 0
                                ? `<div class="prov-pricing-empty">${escapeHtml(t('config.providers.no_pricing_found'))}</div>`
                                : visible.map(m => {
                                    const ck = selected.has(m.name);
                                    const fmt = v => v > 0 ? '$' + v.toFixed(3) : (v === 0 ? t('config.providers.free_badge') : '—');
                                    return `<label class="prov-pricing-row">
                                        <input type="checkbox" class="prov-pricing-checkbox" data-name="${escapeAttr(m.name)}" data-in="${m.input_per_million}" data-out="${m.output_per_million}" ${ck ? 'checked' : ''}>
                                        <span class="prov-pricing-name" title="${escapeAttr(m.name)}">${escapeHtml(m.name)}</span>
                                        <span class="prov-pricing-cost">${escapeHtml(fmt(m.input_per_million))} · ${escapeHtml(fmt(m.output_per_million))}</span>
                                    </label>`;
                                }).join('')
                            }
                        </div>
                        <div class="prov-pricing-foot">
                            <label class="prov-pricing-select-all-label">
                                <input type="checkbox" class="prov-pricing-select-all" id="pp-select-all" ${allVisibleSelected ? 'checked' : ''}>
                                ${escapeHtml(t('config.providers.pricing_picker_select_all'))}
                            </label>
                            <div class="prov-pricing-actions">
                                <button class="btn-save prov-btn-muted prov-btn-xs" onclick="document.getElementById('pricing-picker-overlay').remove()">${escapeHtml(t('config.providers.cancel'))}</button>
                                <button class="btn-save prov-btn-xs" id="pp-confirm">${escapeHtml(t('config.providers.pricing_picker_import', { count: selCount }))}</button>
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
                        showToast(t('config.providers.pricing_fetched', { count: toImport.length }));
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
                showToast('❌ ' + e.message);
            } finally {
                if (btn) { btn.disabled = false; btn.textContent = '📡 ' + t('config.providers.fetch_pricing'); }
            }
        }

        function renderMeshCentralTestBlock() {
            const labelTest = t('config.meshcentral.test_label');
            const labelDesc = t('config.meshcentral.test_desc');
            return `
            <div id="mc-test-block" class="prov-mc-block">
                <div class="prov-mc-title">🔌 ${t('config.providers.meshcentral_test_title')}</div>
                <div class="prov-mc-desc">${labelDesc}</div>
                <div class="prov-mc-actions">
                    <button class="btn-save prov-btn-sm" onclick="testMeshCentral()">${labelTest}</button>
                    <span id="mc-test-spinner" class="prov-mc-spinner is-hidden">⏳ ${t('config.meshcentral.connecting')}</span>
                </div>
                <div id="mc-test-result" class="prov-mc-result is-hidden">
                    <div id="mc-test-msg" class="prov-mc-msg"></div>
                </div>
            </div>`;
        }

        async function testMeshCentral() {
            const spinner   = document.getElementById('mc-test-spinner');
            const resultDiv = document.getElementById('mc-test-result');
            const msgDiv    = document.getElementById('mc-test-msg');
            if (!spinner) return;

            setHidden(spinner, false);
            setHidden(resultDiv, true);

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

                setHidden(resultDiv, false);
                if (json.status === 'ok') {
                    msgDiv.classList.remove('is-error');
                    msgDiv.classList.add('is-success');
                    msgDiv.textContent = '✅ ' + (json.message || t('config.meshcentral.success'));
                } else {
                    msgDiv.classList.remove('is-success');
                    msgDiv.classList.add('is-error');
                    msgDiv.textContent = '❌ ' + (json.message || t('config.meshcentral.failed'));
                }
            } catch (e) {
                setHidden(resultDiv, false);
                msgDiv.classList.remove('is-success');
                msgDiv.classList.add('is-error');
                msgDiv.textContent = '❌ ' + e.message;
            } finally {
                setHidden(spinner, true);
            }
        }

        function renderProvidersSection(section) {
            let html = `<div class="cfg-section active">
                <div class="section-header">${section.icon} ${section.label}</div>
                <div class="section-desc">${section.desc}</div>
                <div class="prov-section-actions">
                    <button class="btn-save prov-btn-sm" onclick="providerAdd()">
                        ＋ ${t('config.providers.new_provider')}
                    </button>
                </div>
                <div id="providers-list"></div>
                <div id="providers-empty" class="prov-empty-state is-hidden">
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
                if (empty) setHidden(empty, false);
                return;
            }
            if (empty) setHidden(empty, true);

            let html = '';
            providersCache.forEach((p, idx) => {
                const typeBadge = p.type
                    ? `<span class="prov-provider-pill">${escapeAttr(p.type)}</span>`
                    : '';
                const isOAuth = p.auth_type === 'oauth2';
                const authBadge = isOAuth
                    ? '<span class="prov-provider-pill prov-provider-pill-oauth">🔑 OAuth2</span>'
                    : '';
                let authInfo;
                if (isOAuth) {
                    authInfo = `<span class="prov-text-muted" id="oauth-status-${idx}">🔑 OAuth2</span>`;
                } else {
                    const maskedKey = p.api_key === '••••••••'
                        ? '<span class="prov-text-muted">••••••••</span>'
                        : (p.api_key ? '<span class="prov-text-success">' + t('config.providers.key_set') + '</span>' : '<span class="prov-text-muted">—</span>');
                    authInfo = maskedKey;
                }
                html += `
                <div class="provider-card prov-provider-card" data-idx="${idx}">
                    <div class="prov-provider-head">
                        <div class="prov-provider-title">
                            ${escapeAttr(p.name || p.id)}${typeBadge}${authBadge}
                            <span class="prov-provider-id">ID: ${escapeAttr(p.id)}</span>
                        </div>
                        <div class="prov-provider-head-actions">
                            <button onclick="providerEdit(${idx})" class="prov-icon-btn is-edit" title="${t('config.providers.card_edit_tooltip')}">✏️</button>
                            <button onclick="providerDelete(${idx})" class="prov-icon-btn is-delete" title="${t('config.providers.card_delete_tooltip')}">🗑️</button>
                        </div>
                    </div>
                    <div class="prov-provider-grid">
                        <div><span class="prov-text-muted">${t('config.providers.card_base_url')}</span> ${escapeAttr(p.base_url || '—')}</div>
                        <div><span class="prov-text-muted">${t('config.providers.card_model')}</span> ${escapeAttr(p.model || '—')}</div>
                        <div><span class="prov-text-muted">${isOAuth ? t('config.providers.card_auth') : t('config.providers.card_api_key')}</span> ${authInfo}</div>
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
                                el.innerHTML = '<span class="prov-text-success">✅ ' + t('config.providers.authorized') + '</span>';
                            } else if (st.authorized && st.expired) {
                                el.innerHTML = '<span class="prov-text-warning">⚠️ ' + t('config.providers.token_expired') + '</span>';
                            } else {
                                el.innerHTML = '<span class="prov-text-danger">❌ ' + t('config.providers.not_authorized') + '</span>';
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
            minimax: 'https://api.minimax.io/v1',
            'workers-ai': '',
            custom: ''
        };

        const PROVIDER_HINTS = {
            openrouter: 'config.providers.hint.openrouter',
            ollama: 'config.providers.hint.ollama',
            anthropic: 'config.providers.hint.anthropic',
            google: 'config.providers.hint.google',
            openai: 'config.providers.hint.openai',
            'workers-ai': 'config.providers.hint.workers_ai',
            minimax: 'config.providers.hint.minimax',
            custom: 'config.providers.hint.custom'
        };

        function providerShowModal(title, data, onSave) {
            // Remove existing modal
            const existing = document.getElementById('provider-modal-overlay');
            if (existing) existing.remove();

            const overlay = document.createElement('div');
            overlay.id = 'provider-modal-overlay';
            overlay.className = 'prov-modal-overlay';
            overlay.onclick = (e) => { if (e.target === overlay) overlay.remove(); };

            const currentAuthType = data.auth_type || 'api_key';
            const isOAuth = currentAuthType === 'oauth2';

            overlay.innerHTML = `
            <div class="prov-modal-panel" onclick="event.stopPropagation()">
                <div class="prov-modal-header">
                    <div class="prov-modal-title">${title}</div>
                    <button onclick="document.getElementById('provider-modal-overlay').remove()" class="prov-modal-close-btn">✕</button>
                </div>
                <div class="field-group">
                    <div class="field-label">${t('config.providers.field_id_label')}</div>
                    <div class="field-help">${t('config.providers.id_help')}</div>
                    <input class="field-input ${data._editMode ? 'is-disabled' : ''}" id="prov-id" value="${escapeAttr(data.id || '')}" placeholder="${t('config.providers.id_placeholder')}" ${data._editMode ? 'disabled' : ''}>
                </div>
                <div class="field-group">
                    <div class="field-label">${t('config.providers.field_name')}</div>
                    <input class="field-input" id="prov-name" value="${escapeAttr(data.name || '')}" placeholder="${t('config.providers.display_name')}">
                </div>
                <div class="field-group">
                    <div class="field-label">${t('config.providers.field_type_label')}</div>
                    <div class="field-help">${t('config.providers.type_help')}</div>
                    <select class="field-select" id="prov-type">
                        ${['openai','openrouter','ollama','anthropic','google','minimax','workers-ai','custom'].map(typ =>
                            `<option value="${typ}"${data.type === typ ? ' selected' : ''}>${t('config.providers.type_' + typ.replace('-', '_'))}</option>`
                        ).join('')}
                    </select>
                </div>
                <div class="field-group">
                    <div class="field-label">${t('config.providers.field_base_url_label')}</div>
                    <input class="field-input ${(data.type || '') === 'workers-ai' ? 'is-disabled' : ''}" id="prov-url" value="${escapeAttr(data.base_url || '')}" placeholder="${PROVIDER_BASE_URLS[data.type] || PROVIDER_BASE_URLS.openrouter}" ${(data.type || '') === 'workers-ai' ? 'disabled' : ''}>
                    <div id="prov-url-auto-hint" class="prov-field-hint ${(data.type || '') === 'workers-ai' ? '' : 'is-hidden'}">${t('config.providers.workers_ai_url_auto')}</div>
                </div>

                <!-- Workers AI Account ID (only visible when type = workers-ai) -->
                <div id="prov-account-id-block" class="field-group ${(data.type || '') === 'workers-ai' ? '' : 'is-hidden'}">
                    <div class="field-label">${t('config.providers.account_id')}</div>
                    <div class="field-help">${t('config.providers.account_id_help')}</div>
                    <input class="field-input" id="prov-account-id" value="${escapeAttr(data.account_id || '')}" placeholder="${t('config.providers.account_id_placeholder')}">
                </div>
                <div class="field-group">
                    <div class="field-label">${t('config.providers.field_model_label')}</div>
                    <input class="field-input" id="prov-model" value="${escapeAttr(data.model || '')}" placeholder="${t('config.providers.model_placeholder')}">
                </div>

                <!-- Ollama model query (only visible when type = ollama) -->
                <div id="prov-ollama-block" class="field-group prov-provider-type-block is-ollama ${(data.type || 'openai') === 'ollama' ? '' : 'is-hidden'}">
                    <div class="prov-type-block-actions">
                        <button type="button" class="btn-save prov-btn-xs" onclick="queryOllamaModelsInModal()">
                            🦙 ${t('config.providers.query_models')}
                        </button>
                        <span id="prov-ollama-spinner" class="prov-inline-muted is-hidden">⏳ ${t('config.providers.loading')}</span>
                    </div>
                    <div id="prov-ollama-result" class="prov-ollama-result is-hidden">
                        <div class="prov-ollama-label">${t('config.providers.available_models')}</div>
                        <div id="prov-ollama-models" class="prov-ollama-models"></div>
                        <div id="prov-ollama-error" class="prov-ollama-error is-hidden"></div>
                    </div>
                </div>

                <!-- OpenRouter model browser (only visible when type = openrouter) -->
                <div id="prov-openrouter-block" class="field-group prov-provider-type-block is-openrouter ${(data.type || 'openai') === 'openrouter' ? '' : 'is-hidden'}">
                    <div class="prov-type-block-actions">
                        <button type="button" class="btn-save prov-openrouter-btn" onclick="openOpenRouterBrowser(function(m){document.getElementById('prov-model').value=m.id;document.getElementById('prov-model').dispatchEvent(new Event('input',{bubbles:true}));updateProviderModelCost(m);})">
                            🌐 ${t('config.providers.open_model_browser')}
                        </button>
                        <span class="prov-inline-muted">${t('config.providers.browse_openrouter')}</span>
                    </div>
                </div>

                <!-- Model Pricing Table -->
                <div class="field-group prov-group-divider">
                    <div class="prov-model-pricing-head">
                        <div class="field-label prov-model-pricing-title">💰 ${t('config.providers.model_pricing')}</div>
                        <div class="prov-model-pricing-actions">
                            <button type="button" class="btn-save prov-fetch-pricing-btn ${['openrouter','openai','anthropic','google','ollama'].includes(data.type || 'openai') ? '' : 'is-hidden'}" id="prov-fetch-pricing-btn">
                                📡 ${t('config.providers.fetch_pricing')}
                            </button>
                            <button type="button" class="btn-save prov-btn-muted prov-btn-xs" onclick="providerAddModelRow()">
                                + ${t('config.providers.add_model')}
                            </button>
                        </div>
                    </div>
                    <div id="prov-models-table-wrap">
                        <div id="prov-models-search-wrap" class="is-hidden prov-model-search-wrap">
                            <input class="field-input prov-model-search-input" id="prov-models-filter" type="search" autocomplete="off"
                                   placeholder="🔍 ${escapeHtml(t('config.providers.filter_models'))}"
                                   oninput="providerRenderModelsTable()">
                            <div id="prov-models-count" class="prov-model-count is-hidden"></div>
                        </div>
                        <table class="prov-model-table" id="prov-models-table">
                            <thead>
                                <tr class="prov-model-head-row">
                                    <th class="prov-model-head-cell">${t('config.providers.model_name')}</th>
                                    <th class="prov-model-head-cell is-right">${t('config.providers.input_cost')}</th>
                                    <th class="prov-model-head-cell is-right">${t('config.providers.output_cost')}</th>
                                    <th class="prov-model-head-cell is-actions"></th>
                                </tr>
                            </thead>
                            <tbody id="prov-models-body"></tbody>
                        </table>
                        <div id="prov-models-empty" class="prov-model-empty is-hidden">
                            ${t('config.providers.no_models_configured')}
                        </div>
                    </div>
                </div>

                <!-- Auth Type Toggle -->
                <div class="field-group prov-group-divider">
                    <div class="field-label">${t('config.providers.authentication')}</div>
                    <select class="field-select" id="prov-auth-type">
                        <option value="api_key"${currentAuthType !== 'oauth2' ? ' selected' : ''}>${t('config.providers.auth_type_api_key_option')}</option>
                        <option value="oauth2"${currentAuthType === 'oauth2' ? ' selected' : ''}>${t('config.providers.auth_type_oauth2_option')}</option>
                    </select>
                </div>

                <!-- API Key section (visible when auth_type = api_key) -->
                <div id="prov-apikey-section" class="${isOAuth ? 'is-hidden' : ''}">
                    <div class="field-group" id="prov-copykey-group">
                        <div class="field-label">${t('config.providers.copy_key_label')}</div>
                        <select class="field-select" id="prov-copy-key-from">
                            <option value="">${t('config.providers.copy_key_none')}</option>
                        </select>
                    </div>
                    <div class="field-group">
                        <div class="field-label">${t('config.providers.field_api_key_label')}</div>
                        <div id="prov-key-hint" class="prov-field-hint">${PROVIDER_HINTS[data.type || 'openai'] ? t(PROVIDER_HINTS[data.type || 'openai']) : t(PROVIDER_HINTS.openai)}</div>
                        <div class="password-wrap">
                            <input class="field-input" id="prov-key" type="password" value="${escapeAttr(data.api_key === '••••••••' ? '' : (data.api_key || ''))}" placeholder="${data.api_key === '••••••••' ? t('config.providers.key_placeholder_existing') : t('config.providers.api_key_placeholder')}" autocomplete="off">
                            <button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">${EYE_OPEN_SVG}</button>
                        </div>
                        ${data.api_key === '••••••••' ? `<div class="prov-field-hint">${t('config.providers.keep_existing_key')}</div>` : ''}
                    </div>
                </div>

                <!-- OAuth2 section (visible when auth_type = oauth2) -->
                <div id="prov-oauth-section" class="${isOAuth ? '' : 'is-hidden'}">
                    <div class="field-group">
                        <div class="field-label">${t('config.providers.field_auth_url_label')}</div>
                        <div class="field-help">${t('config.providers.oauth_auth_url_help')}</div>
                        <input class="field-input" id="prov-oauth-auth-url" value="${escapeAttr(data.oauth_auth_url || '')}" placeholder="${t('config.providers.auth_url_placeholder')}">
                    </div>
                    <div class="field-group">
                        <div class="field-label">${t('config.providers.field_token_url_label')}</div>
                        <div class="field-help">${t('config.providers.oauth_token_url_help')}</div>
                        <input class="field-input" id="prov-oauth-token-url" value="${escapeAttr(data.oauth_token_url || '')}" placeholder="${t('config.providers.token_url_placeholder')}">
                    </div>
                    <div class="field-group">
                        <div class="field-label">${t('config.providers.field_client_id_label')}</div>
                        <input class="field-input" id="prov-oauth-client-id" value="${escapeAttr(data.oauth_client_id || '')}" placeholder="${t('config.providers.client_id_placeholder')}">
                    </div>
                    <div class="field-group">
                        <div class="field-label">${t('config.providers.field_client_secret_label')}</div>
                        <div class="password-wrap">
                            <input class="field-input" id="prov-oauth-client-secret" type="password" value="${escapeAttr(data.oauth_client_secret === '••••••••' ? '' : (data.oauth_client_secret || ''))}" placeholder="${data.oauth_client_secret === '••••••••' ? t('config.providers.key_placeholder_existing') : t('config.providers.client_secret_placeholder')}" autocomplete="off">
                            <button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">${EYE_OPEN_SVG}</button>
                        </div>
                        ${data.oauth_client_secret === '••••••••' ? `<div class="prov-field-hint">${t('config.providers.keep_existing_secret')}</div>` : ''}
                    </div>
                    <div class="field-group">
                        <div class="field-label">Scopes</div>
                        <div class="field-help">${t('config.providers.oauth_scopes_help')}</div>
                        <input class="field-input" id="prov-oauth-scopes" value="${escapeAttr(data.oauth_scopes || '')}" placeholder="openid email https://www.googleapis.com/auth/cloud-platform">
                    </div>
                    ${data._editMode && currentAuthType === 'oauth2' ? `
                    <div class="field-group prov-oauth-group">
                        <div id="prov-oauth-status" class="prov-oauth-status">⏳ ${t('config.providers.checking_status')}</div>
                        <div class="prov-oauth-actions">
                            <button class="btn-save prov-oauth-authorize-btn" id="prov-oauth-authorize-btn">
                                🔐 ${t('config.providers.authorize')}
                            </button>
                            <button class="btn-save prov-oauth-revoke-btn" id="prov-oauth-revoke-btn">
                                🗑️ ${t('config.providers.revoke_token')}
                            </button>
                        </div>
                    </div>
                    ` : ''}
                </div>

                <div class="prov-modal-actions">
                    <button class="btn-save prov-btn-muted prov-btn-md" onclick="document.getElementById('provider-modal-overlay').remove()">
                        ${t('config.providers.cancel')}
                    </button>
                    <button class="btn-save prov-btn-md" id="prov-save-btn">
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
            const accountIdBlock = document.getElementById('prov-account-id-block');
            const urlAutoHint = document.getElementById('prov-url-auto-hint');
            const knownUrls = new Set(Object.values(PROVIDER_BASE_URLS).filter(Boolean));
            typeSelect.addEventListener('change', () => {
                const typ = typeSelect.value;
                const currentUrl = urlInput.value.trim();
                const isWorkersAI = typ === 'workers-ai';
                // Auto-fill URL when empty OR when it still contains a known default URL (i.e. user hasn't typed a custom one)
                if (isWorkersAI) {
                    urlInput.value = '';
                    urlInput.disabled = true;
                    urlInput.classList.add('is-disabled');
                } else {
                    urlInput.disabled = false;
                    urlInput.classList.remove('is-disabled');
                    if ((!currentUrl || knownUrls.has(currentUrl)) && PROVIDER_BASE_URLS[typ]) {
                        urlInput.value = PROVIDER_BASE_URLS[typ];
                    }
                }
                // Update placeholder
                urlInput.placeholder = isWorkersAI ? t('config.providers.workers_ai_url_auto') : (PROVIDER_BASE_URLS[typ] || 'https://...');
                // Update hint
                if (hintEl) {
                    const hintKey = PROVIDER_HINTS[typ];
                    hintEl.textContent = hintKey ? t(hintKey) : '';
                }
                // Show/hide Workers AI account ID + URL auto hint
                if (accountIdBlock) setHidden(accountIdBlock, !isWorkersAI);
                if (urlAutoHint) setHidden(urlAutoHint, !isWorkersAI);
                // Show/hide Ollama model query block
                if (ollamaBlock) setHidden(ollamaBlock, typ !== 'ollama');
                // Show/hide OpenRouter model browser block
                if (openrouterBlock) setHidden(openrouterBlock, typ !== 'openrouter');
                // Show/hide fetch pricing button
                if (fetchPricingBtn) {
                    setHidden(fetchPricingBtn, !['openrouter','openai','anthropic','google','ollama','workers-ai'].includes(typ));
                }
            });

            // ── Auth type toggle ──
            const authTypeSelect = document.getElementById('prov-auth-type');
            const apikeySection = document.getElementById('prov-apikey-section');
            const oauthSection = document.getElementById('prov-oauth-section');
            authTypeSelect.addEventListener('change', () => {
                const isOA = authTypeSelect.value === 'oauth2';
                setHidden(apikeySection, isOA);
                setHidden(oauthSection, !isOA);
                if (!isOA) rebuildCopyKeyDropdown();
            });

            // ── Copy-key dropdown: rebuild on provider type change ──
            // rebuildCopyKeyDropdown filters providersCache by same type as the
            // currently selected provider type, excludes the provider being edited,
            // and only shows entries that already have a key set (masked = "••••••••").
            function rebuildCopyKeyDropdown() {
                const copySelect = document.getElementById('prov-copy-key-from');
                const keyInput = document.getElementById('prov-key');
                if (!copySelect) return;
                const selectedType = typeSelect ? typeSelect.value : (data.type || '');
                const currentID = data._editMode ? data.id : null;
                const matches = (providersCache || []).filter(p =>
                    p.api_key === '••••••••' &&
                    p.type === selectedType &&
                    p.id !== currentID &&
                    p.auth_type !== 'oauth2'
                );
                // Preserve current selection if still valid
                const prevVal = copySelect.value;
                copySelect.innerHTML = `<option value="">${t('config.providers.copy_key_none')}</option>`;
                for (const p of matches) {
                    const opt = document.createElement('option');
                    opt.value = p.id;
                    opt.textContent = `${p.name || p.id} (${p.id})`;
                    if (p.id === prevVal) opt.selected = true;
                    copySelect.appendChild(opt);
                }
                // Re-apply key field state based on restored selection
                const hasSelection = copySelect.value !== '';
                if (keyInput) {
                    keyInput.disabled = hasSelection;
                    if (!hasSelection) keyInput.placeholder = data.api_key === '••••••••' ? t('config.providers.key_placeholder_existing') : 'sk-...';
                }
            }

            // Rebuild when provider type changes
            if (typeSelect) {
                typeSelect.addEventListener('change', rebuildCopyKeyDropdown);
            }

            // Copy-key selection → disable/enable key input
            const copyKeySelect = document.getElementById('prov-copy-key-from');
            if (copyKeySelect) {
                copyKeySelect.addEventListener('change', () => {
                    const keyInput = document.getElementById('prov-key');
                    if (!keyInput) return;
                    if (copyKeySelect.value) {
                        keyInput.disabled = true;
                        keyInput.value = '';
                        keyInput.placeholder = t('config.providers.copy_key_none');
                    } else {
                        keyInput.disabled = false;
                        keyInput.placeholder = data.api_key === '••••••••' ? t('config.providers.key_placeholder_existing') : 'sk-...';
                    }
                });
                // Initial population
                rebuildCopyKeyDropdown();
            }

            // ── OAuth Authorize button ──
            const authBtn = document.getElementById('prov-oauth-authorize-btn');
            if (authBtn) {
                authBtn.onclick = async () => {
                    try {
                        const resp = await fetch('/api/oauth/start?provider=' + encodeURIComponent(data.id));
                            if (!resp.ok) { showToast(await resp.text(), 'error'); return; }
                        const result = await resp.json();
                        if (result.auth_url) {
                            window.open(result.auth_url, '_blank', 'width=600,height=700');
                        }
                        } catch (e) { showToast(e.message || t('config.common.error'), 'error'); }
                };
            }

            // ── OAuth Revoke button ──
            const revokeBtn = document.getElementById('prov-oauth-revoke-btn');
            if (revokeBtn) {
                revokeBtn.onclick = async () => {
                    if (!(await showConfirm(t('config.providers.revoke_confirm_title', {default: t('config.providers.revoke_confirm')}), t('config.providers.revoke_confirm')))) return;
                    try {
                        await fetch('/api/oauth/revoke?provider=' + encodeURIComponent(data.id), { method: 'DELETE' });
                        const statusEl = document.getElementById('prov-oauth-status');
                        if (statusEl) statusEl.innerHTML = '<span class="prov-text-danger">❌ ' + t('config.providers.not_authorized') + '</span>';
                        } catch (e) { showToast(e.message || t('config.common.error'), 'error'); }
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
                            statusEl.innerHTML = '<span class="prov-text-success">✅ ' + t('config.providers.authorized') + '</span>'
                                + (st.expiry ? `<span class="prov-oauth-expiry">(${t('config.providers.expires')}: ${new Date(st.expiry).toLocaleString()})</span>` : '');
                        } else if (st.authorized && st.expired) {
                            statusEl.innerHTML = '<span class="prov-text-warning">⚠️ ' + t('config.providers.token_expired_reauth') + '</span>';
                        } else {
                            statusEl.innerHTML = '<span class="prov-text-danger">❌ ' + t('config.providers.not_authorized_click') + '</span>';
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
                const account_id = (document.getElementById('prov-account-id') || {}).value ? document.getElementById('prov-account-id').value.trim() : '';

                    if (!id) { showToast(t('config.providers.id_empty_error'), 'warn'); return; }
                if (type === 'workers-ai') {
                        if (!account_id) { showToast(t('config.providers.account_id_empty_error'), 'warn'); return; }
                } else if (!base_url) {
                        showToast(t('config.providers.url_empty_error'), 'warn'); return;
                }

                const entry = { id, name: name || id, type, base_url, model, auth_type, account_id };

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
                            showToast(t('config.providers.oauth_required_error'), 'warn');
                        return;
                    }
                } else {
                    // Check if user selected "copy from existing provider"
                    const copyFromSelect = document.getElementById('prov-copy-key-from');
                    const copyFromId = copyFromSelect ? copyFromSelect.value : '';
                    let api_key;
                    if (copyFromId) {
                        api_key = '__copy_from__' + copyFromId;
                    } else {
                        api_key = document.getElementById('prov-key').value.trim();
                        if (!api_key && data.api_key === '••••••••') api_key = '••••••••';
                    }
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
                            showToast(t('config.providers.id_exists'), 'warn');
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

        async function providerDelete(idx) {
            const p = providersCache[idx];
            if (!(await showConfirm(t('config.providers.delete_confirm_title', {default: t('config.providers.delete_confirm')}), t('config.providers.delete_confirm', { name: p.name || p.id })))) return;
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
                        showToast(txt || t('config.common.error'), 'error');
                    return;
                }
                const result = await resp.json();
                // Reload providers from server (API keys will be masked)
                const reload = await fetch('/api/providers');
                if (reload.ok) providersCache = await reload.json();
                providerRenderCards();

                // Update dashboard agent banner immediately if model/provider changed.
                if (result.active_llm_model) {
                    const modelEl = document.getElementById('ab-model');
                    if (modelEl) modelEl.textContent = result.active_llm_model;
                }
            } catch (e) {
                    showToast(e.message || t('config.common.error'), 'error');
            }
        }
