// AuraGo dashboard – section "system": system gauges, operations, integrations, guardian and GitHub repositories.
// Classic script, loaded with defer in sidebar order after widgets-knowledge.js and before dashboard-events.js.
// Its declarations stay global: inline onclick strings and other dashboard scripts call them by name. The last
// statement registers the section loader with DashSections (dash-sections.js); main.js#loadSectionContent runs it.

        const HIDDEN_INTEGRATIONS = new Set(['onedrive']);

        async function loadSectionSystem({ force = false } = {}) {
            CardState.setLoading('card-system');
            CardState.setLoading('card-operations');
            const opts = sectionFetch(force);
            const cards = Promise.all([
                loadGuardianCard(),
                typeof window.loadRealtimeSpeechStatus === 'function' ? window.loadRealtimeSpeechStatus() : null,
            ]);
            const [systemR, overviewR, githubReposR] = await Promise.all([
                API.getWithStatus('/api/dashboard/system', opts),
                // card-operations and the integration flags (the overview reads /api/dashboard/summary).
                API.getWithStatus('/api/dashboard/overview', opts),
                API.getWithStatus('/api/dashboard/github-repos', opts),
            ]);
            const system = systemR.ok ? systemR.data : null;
            const overview = overviewR.ok ? overviewR.data : null;
            const githubRepos = githubReposR.ok ? githubReposR.data : null;

            systemR.ok ? CardState.setLoaded('card-system') : CardState.setError('card-system', loadSectionSystem, { status: systemR.status });
            overviewR.ok ? CardState.setLoaded('card-operations') : CardState.setError('card-operations', loadSectionSystem, { status: overviewR.status });

            if (system) {
                if (!Charts.cpu) Charts.cpu = createGauge('cpu-chart', system.cpu?.usage_percent || 0);
                updateGauge(Charts.cpu, 'cpu-val', system.cpu?.usage_percent || 0);
                if (!Charts.ram) Charts.ram = createGauge('ram-chart', system.memory?.used_percent || 0);
                updateGauge(Charts.ram, 'ram-val', system.memory?.used_percent || 0);
                if (!Charts.disk) Charts.disk = createGauge('disk-chart', system.disk?.used_percent || 0);
                updateGauge(Charts.disk, 'disk-val', system.disk?.used_percent || 0);
                renderSystemStats(system);
            }
            if (overview) {
                renderOperations(overview);
                renderIntegrations(overview);
            }
            if (githubRepos) renderGitHubRepos(githubRepos);
            await cards;
        }

        DashSections.register({ id: 'system', load: loadSectionSystem });
