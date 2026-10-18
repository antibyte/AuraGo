// AuraGo dashboard – section "prompts": prompt builder analytics, optimization and output compression.
// Classic script, loaded with defer in sidebar order after widgets-knowledge.js and before dashboard-events.js.
// Its declarations stay global: inline onclick strings and other dashboard scripts call them by name. The last
// statement registers the section loader with DashSections (dash-sections.js); main.js#loadSectionContent runs it.

        async function loadSectionPrompts({ force = false } = {}) {
            CardState.setLoading('card-prompt');
            CardState.setLoading('card-optimization');
            CardState.setLoading('card-compression');
            const opts = sectionFetch(force);
            const [promptStatsR, optR, compR] = await Promise.all([
                API.getWithStatus('/api/dashboard/prompt-stats', opts),
                API.getWithStatus('/api/dashboard/optimization', opts),
                API.getWithStatus('/api/dashboard/compression', opts),
            ]);
            const promptStats = promptStatsR.ok ? promptStatsR.data : null;
            const opt = optR.ok ? optR.data : null;
            const comp = compR.ok ? compR.data : null;

            promptStatsR.ok ? CardState.setLoaded('card-prompt') : CardState.setError('card-prompt', loadSectionPrompts, { status: promptStatsR.status });
            optR.ok ? CardState.setLoaded('card-optimization') : CardState.setError('card-optimization', loadSectionPrompts, { status: optR.status });
            compR.ok ? CardState.setLoaded('card-compression') : CardState.setError('card-compression', loadSectionPrompts, { status: compR.status });

            if (promptStats) {
                renderPromptStats(promptStats);
                if (Charts.promptSize) Charts.promptSize.destroy();
                Charts.promptSize = createPromptSizeChart('prompt-size-chart', promptStats.recent);
                if (Charts.promptTier) Charts.promptTier.destroy();
                Charts.promptTier = createPromptTierChart('prompt-tier-chart', promptStats.tier_distribution);
                if (Charts.promptSavings) Charts.promptSavings.destroy();
                Charts.promptSavings = createPromptSavingsChart('prompt-savings-chart', promptStats.recent_fits);
                if (Charts.promptSectionDist) Charts.promptSectionDist.destroy();
                Charts.promptSectionDist = createPromptSectionDistChart('prompt-section-dist-chart', promptStats.avg_section_sizes);
            }
            if (opt) renderOptimizationStats(opt);
            if (comp) renderCompressionStats(comp);
        }

        DashSections.register({ id: 'prompts', load: loadSectionPrompts });
