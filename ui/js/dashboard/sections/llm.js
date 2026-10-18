// AuraGo dashboard – section "llm": budget, LLM router and helper LLM.
// Classic script, loaded with defer in sidebar order after widgets-knowledge.js and before dashboard-events.js.
// Its declarations stay global: inline onclick strings and other dashboard scripts call them by name. The last
// statement registers the section loader with DashSections (dash-sections.js); main.js#loadSectionContent runs it.

        async function loadSectionLlm({ force = false } = {}) {
            CardState.setLoading('card-budget');
            CardState.setLoading('card-helper-llm');
            const opts = sectionFetch(force);
            const cards = Promise.all([loadTaskRouterStats(), loadHelperLLMCard()]);
            const [budgetR, creditsR] = await Promise.all([
                API.getWithStatus('/api/budget', opts),
                API.getWithStatus('/api/credits', opts),
            ]);
            const budget = budgetR.ok ? budgetR.data : null;
            const credits = creditsR.ok ? creditsR.data : null;

            budgetR.ok ? CardState.setLoaded('card-budget') : CardState.setError('card-budget', loadSectionLlm, { status: budgetR.status });

            if (budget) {
                renderBudget(budget, credits);
                if (budget.enabled) {
                    if (Charts.budget) { Charts.budget.destroy(); Charts.budget = null; }
                    Charts.budget = createBudgetDoughnut('budget-chart', budget.spent_usd || 0, budget.daily_limit_usd || 0);
                    if (Charts.budgetModels) { Charts.budgetModels.destroy(); Charts.budgetModels = null; }
                    Charts.budgetModels = createBudgetModelsChart('budget-models-chart', budget.models || {});
                }
            }
            await cards;
        }

        async function loadTaskRouterStats() {
            const container = document.getElementById('llm-router-stats');
            if (!container) return;
            const response = await API.getWithStatus('/api/llm-router/status');
            container.replaceChildren();
            const line = text => { const p = document.createElement('p'); p.textContent = text; container.appendChild(p); };
            if (!response.ok) { line(t('common.error')); return; }
            const data = response.data;
            const stats = data.stats || {};
            const label = key => t('common.llm_router.' + key);
            line(t(data.enabled ? 'common.enabled' : 'common.disabled') + ' · ' + (data.default_model || ''));
            line(label('since') + ' ' + new Date(stats.since).toLocaleString());
            line(label('helper_calls') + ': ' + Number(stats.helper_attempts || 0) + ' · ' + label('failures') + ': ' + Number(stats.helper_failures || 0));
            const tokens = Number((stats.helper_input_tokens || 0) + (stats.helper_output_tokens || 0));
            const usage = stats.helper_usage_missing ? (stats.helper_usage_reports ? '≥ ' + tokens : '?') : String(tokens);
            line(label('tokens') + ': ' + usage);
            const decision = stats.decision_api || {};
            const decisionTokens = Number(decision.input_tokens || 0) + Number(decision.output_tokens || 0);
            const decisionUsage = decision.usage_missing ? (decision.usage_reports ? '≥ ' + decisionTokens : '?') : String(decisionTokens);
            const decisionCost = decision.cost_missing ? (decision.cost_reports ? '≥ ' + Number(decision.cost_usd).toFixed(4) : '?') : (decision.cost_reports ? Number(decision.cost_usd).toFixed(4) : '?');
            line(label('source_decisions') + ': ' + Number(decision.attempts || 0) + ' · ' + label('failures') + ': ' + Number(decision.failures || 0) + ' · ' + label('decisions_uncertain') + ': ' + Number(decision.uncertain || 0));
            line(label('source_decisions') + ' · ' + label('tokens') + ': ' + decisionUsage + ' · USD: ' + decisionCost);
            line(label('fallback') + ': ' + Number(stats.fallbacks || 0));
            for (const area of ['general', 'easy', 'normal', 'complex', 'coding', 'research', 'creativity', 'security', 'writing']) {
                if (stats.decisions?.[area]) line(label('area_' + area) + ': ' + Number(stats.decisions[area]));
            }
            for (const source of ['rules', 'cache', 'helper', 'decisions', 'default']) {
                line(label('source_' + source) + ': ' + Number(stats.sources?.[source] || 0));
            }
        }

        function renderBudget(data, credits) {
            if (!data || !data.enabled) {
                dashSetHidden(document.getElementById('budget-content'), true);
                dashSetHidden(document.getElementById('budget-disabled'), false);
                return;
            }
            dashSetHidden(document.getElementById('budget-content'), false);
            dashSetHidden(document.getElementById('budget-disabled'), true);
            document.getElementById('budget-spent').textContent = '$' + (data.spent_usd || 0).toFixed(2);
            document.getElementById('budget-sublabel').textContent = t('dashboard.budget_sublabel', {amount: '$' + (data.daily_limit_usd || 0).toFixed(2)});

            // Status badges
            const badgesEl = document.getElementById('budget-badges');
            let badges = '';
            if (data.is_blocked) badges += '<span class="budget-badge danger">🚫 ' + t('dashboard.budget_blocked') + '</span>';
            else if (data.is_exceeded) badges += '<span class="budget-badge danger">⚠️ ' + t('dashboard.budget_exceeded') + '</span>';
            else if (data.is_warning) badges += '<span class="budget-badge warning">⚡ ' + t('dashboard.budget_warning') + '</span>';
            if (data.enforcement) {
                const enfMap = { warn: t('dashboard.budget_enforcement_warn'), partial: t('dashboard.budget_enforcement_partial'), full: t('dashboard.budget_enforcement_full') };
                badges += `<span class="budget-badge info">🛡️ ${esc(enfMap[data.enforcement] || String(data.enforcement))}</span>`;
            }
            badgesEl.innerHTML = badges;

            // Reset countdown
            const resetEl = document.getElementById('budget-reset');
            if (data.reset_time) {
                const resetDate = new Date(data.reset_time);
                const now = new Date();
                const diffMs = resetDate - now;
                if (diffMs > 0) {
                    const hrs = Math.floor(diffMs / 3600000);
                    const mins = Math.floor((diffMs % 3600000) / 60000);
                    resetEl.textContent = t('dashboard.budget_reset_in', {hours: hrs, minutes: mins});
                } else {
                    resetEl.textContent = '';
                }
            } else {
                resetEl.textContent = '';
            }

            // OpenRouter credits
            const creditsRow = document.getElementById('credits-row');
            if (credits && credits.available && !credits.error) {
                dashSetHidden(creditsRow, false);
                document.getElementById('credits-balance').textContent = '$' + (credits.balance || 0).toFixed(2);
            } else {
                dashSetHidden(creditsRow, true);
            }

            // Per-LLM average token consumption chart
            const llmAvgWrap = document.getElementById('budget-llm-avg-wrap');
            const models = data.models || {};
            const hasCallData = Object.values(models).some(m => (m.calls || 0) > 0);
            if (hasCallData && llmAvgWrap) {
                dashSetHidden(llmAvgWrap, false);
                if (Charts.llmAvg) { Charts.llmAvg.destroy(); Charts.llmAvg = null; }
                Charts.llmAvg = createLLMAvgChart('budget-llm-avg-chart', models);
            } else if (llmAvgWrap) {
                dashSetHidden(llmAvgWrap, true);
            }
        }

        async function loadHelperLLMCard() {
            try {
                const data = await API.get('/api/dashboard/helper-llm');
                renderHelperLLMCard(data);
                if (typeof CardState !== 'undefined' && CardState.setLoaded) {
                    CardState.setLoaded('card-helper-llm');
                }
            } catch (_) {
                if (typeof CardState !== 'undefined' && CardState.setError) {
                    CardState.setError('card-helper-llm', loadHelperLLMCard);
                }
            }
        }

        function helperLLMOperationLabel(operation) {
            const labels = {
                analyze_turn: t('dashboard.helper_llm_operation_analyze_turn'),
                maintenance_summary_kg: t('dashboard.helper_llm_operation_maintenance_summary_kg'),
                consolidation_batches: t('dashboard.helper_llm_operation_consolidation_batches'),
                compress_memories: t('dashboard.helper_llm_operation_compress_memories'),
                content_summaries: t('dashboard.helper_llm_operation_content_summaries'),
                rag_batch: t('dashboard.helper_llm_operation_rag_batch'),
            };
            return labels[operation] || String(operation || '').replace(/_/g, ' ');
        }

        function helperLLMOperationDescription(operation) {
            const descriptions = {
                analyze_turn: t('dashboard.helper_llm_operation_desc_analyze_turn'),
                maintenance_summary_kg: t('dashboard.helper_llm_operation_desc_maintenance_summary_kg'),
                rag_batch: t('dashboard.helper_llm_operation_desc_rag_batch'),
            };
            const description = descriptions[operation];
            if (!description || description.startsWith('dashboard.')) return '';
            return description;
        }

        function renderHelperLLMCard(data) {
            const statusEl = document.getElementById('helper-llm-status');
            const metricsEl = document.getElementById('helper-llm-metrics');
            const operationsEl = document.getElementById('helper-llm-operations');
            if (!statusEl || !metricsEl || !operationsEl) return;

            const enabled = !!data?.enabled;
            const updatedAt = data?.updated_at ? Date.parse(data.updated_at) : 0;
            const totals = data?.totals || {};
            const operations = Object.entries(data?.operations || {})
                .sort((a, b) => {
                    const reqDiff = Number(b[1]?.requests || 0) - Number(a[1]?.requests || 0);
                    if (reqDiff !== 0) return reqDiff;
                    return a[0].localeCompare(b[0]);
                });

            statusEl.innerHTML = `
                <div class="guardian-status-row">
                    <span class="guardian-lbl">${t('dashboard.helper_llm_state')}:</span>
                    <span class="guardian-val">${enabled ? t('dashboard.helper_llm_state_enabled') : t('dashboard.helper_llm_state_disabled')}</span>
                </div>
                <div class="guardian-status-row">
                    <span class="guardian-lbl">${t('dashboard.helper_llm_last_update')}:</span>
                    <span class="guardian-val">${updatedAt ? relativeTime(updatedAt) : '—'}</span>
                </div>`;

            if (!enabled && !operations.length) {
                metricsEl.innerHTML = `<div class="empty-state">${t('dashboard.helper_llm_disabled')}</div>`;
                operationsEl.innerHTML = '';
                return;
            }

            metricsEl.innerHTML = `
                <div class="guardian-metrics-grid">
                    <div class="guardian-metric">
                        <div class="guardian-metric-val">${Number(totals.requests || 0)}</div>
                        <div class="guardian-metric-lbl">${t('dashboard.helper_llm_requests')}</div>
                    </div>
                    <div class="guardian-metric">
                        <div class="guardian-metric-val ok">${Number(totals.cache_hits || 0)}</div>
                        <div class="guardian-metric-lbl">${t('dashboard.helper_llm_cache_hits')}</div>
                    </div>
                    <div class="guardian-metric">
                        <div class="guardian-metric-val">${Number(totals.llm_calls || 0)}</div>
                        <div class="guardian-metric-lbl">${t('dashboard.helper_llm_llm_calls')}</div>
                    </div>
                    <div class="guardian-metric">
                        <div class="guardian-metric-val warn">${Number(totals.fallbacks || 0)}</div>
                        <div class="guardian-metric-lbl">${t('dashboard.helper_llm_fallbacks')}</div>
                    </div>
                    <div class="guardian-metric">
                        <div class="guardian-metric-val ok">${Number(totals.saved_calls || 0)}</div>
                        <div class="guardian-metric-lbl">${t('dashboard.helper_llm_saved_calls')}</div>
                    </div>
                    <div class="guardian-metric">
                        <div class="guardian-metric-val">${Number(totals.batched_items || 0)}</div>
                        <div class="guardian-metric-lbl">${t('dashboard.helper_llm_batched_items')}</div>
                    </div>
                </div>`;

            if (!operations.length) {
                operationsEl.innerHTML = `<div class="empty-state dash-empty-tight">${t('dashboard.helper_llm_operation_empty')}</div>`;
                return;
            }

            operationsEl.innerHTML = `
                <div class="helper-llm-operation-list">
                    ${renderCollapsibleList(operations, ([name, stats]) => `
                        <div class="helper-llm-operation-item">
                            <div class="helper-llm-operation-head">
                                <span class="helper-llm-operation-name">${escapeHtml(helperLLMOperationLabel(name))}</span>
                                <span class="helper-llm-operation-pill">${Number(stats.requests || 0)} ${t('dashboard.helper_llm_requests')}</span>
                            </div>
                            <div class="helper-llm-operation-meta">
                                <span class="tooling-telemetry-scope-pill">${Number(stats.cache_hits || 0)} ${t('dashboard.helper_llm_cache_hits')}</span>
                                <span class="tooling-telemetry-scope-pill">${Number(stats.llm_calls || 0)} ${t('dashboard.helper_llm_llm_calls')}</span>
                                <span class="tooling-telemetry-scope-pill">${Number(stats.fallbacks || 0)} ${t('dashboard.helper_llm_fallbacks')}</span>
                                <span class="tooling-telemetry-scope-pill">${Number(stats.saved_calls || 0)} ${t('dashboard.helper_llm_saved_calls')}</span>
                                <span class="tooling-telemetry-scope-pill">${Number(stats.batched_items || 0)} ${t('dashboard.helper_llm_batched_items')}</span>
                            </div>
                            ${helperLLMOperationDescription(name) ? `<div class="helper-llm-operation-detail">${escapeHtml(helperLLMOperationDescription(name))}</div>` : ''}
                            ${stats.last_detail ? `<div class="helper-llm-operation-detail">${t('dashboard.helper_llm_operation_last')}: ${escapeHtml(stats.last_detail)}</div>` : ''}
                        </div>
                    `, 5)}
                </div>`;
        }

        DashSections.register({ id: 'llm', load: loadSectionLlm });
