// AuraGo dashboard – section "overview": the overview loader (tiles: dash-overview.js, data: dash-summary.js).
// Classic script, loaded with defer in sidebar order after widgets-knowledge.js and before dashboard-events.js.
// Its declarations stay global: inline onclick strings and other dashboard scripts call them by name. The last
// statement registers the section loader with DashSections (dash-sections.js); main.js#loadSectionContent runs it.

        // The eight tiles of the overview section (markup: dashboard/sections/overview.html).
        const OVERVIEW_TILE_IDS = ['ov-status', 'ov-attention', 'ov-system', 'ov-budget', 'ov-automations', 'ov-knowledge', 'ov-activity', 'ov-agent-state'];

        // Resolves to false when the summary could not be loaded (every tile then shows the error state and a
        // retry); loadSectionContent skips the "Updated" stamp for that.
        async function loadSectionOverview({ force = false } = {}) {
            OVERVIEW_TILE_IDS.forEach(id => CardState.setLoading(id));
            // One summary answer (dash-summary.js, also behind the sidebar status block and badges) renders
            // every overview tile; only the system section still reads the old overview answer (card-operations).
            const result = await DashSummary.refresh({ force });
            const summary = DashSummary.latest();
            if (!result.ok || !summary) {
                // A failed refresh must not leave the last good summary on screen as if it were current.
                const retry = () => loadSectionContent('overview', { force: true });
                OVERVIEW_TILE_IDS.forEach(id => CardState.setError(id, retry, { status: result.status }));
                return false;
            }
            DashOverview.render(summary);
        }

        DashSections.register({ id: 'overview', load: loadSectionOverview });
