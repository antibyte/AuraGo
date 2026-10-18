// AuraGo dashboard – section "tools": activity, adaptive tool stats and tooling telemetry.
// Classic script, loaded with defer in sidebar order after widgets-knowledge.js and before dashboard-events.js.
// Its declarations stay global: inline onclick strings and other dashboard scripts call them by name. The last
// statement registers the section loader with DashSections (dash-sections.js); main.js#loadSectionContent runs it.

        async function loadSectionTools({ force = false } = {}) {
            CardState.setLoading('card-activity');
            const opts = sectionFetch(force);
            const [activityR, toolStatsR] = await Promise.all([
                API.getWithStatus('/api/dashboard/activity', opts),
                API.getWithStatus('/api/dashboard/tool-stats', opts),
            ]);
            const activity = activityR.ok ? activityR.data : null;
            const toolStats = toolStatsR.ok ? toolStatsR.data : null;

            activityR.ok ? CardState.setLoaded('card-activity') : CardState.setError('card-activity', loadSectionTools, { status: activityR.status });

            if (activity) renderActivity(activity);
            // One tool-stats answer feeds both tool cards.
            if (toolStats) {
                renderAdaptiveToolStats(toolStats);
                renderToolingTelemetry(toolStats);
            }
        }

        DashSections.register({ id: 'tools', load: loadSectionTools });
