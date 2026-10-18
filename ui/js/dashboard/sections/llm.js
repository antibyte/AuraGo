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

        DashSections.register({ id: 'llm', load: loadSectionLlm });
