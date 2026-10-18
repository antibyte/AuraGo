// AuraGo dashboard – section "automations": cron jobs, mission history and daemon skills.
// Classic script, loaded with defer in sidebar order after widgets-knowledge.js and before dashboard-events.js.
// Its declarations stay global: inline onclick strings and other dashboard scripts call them by name. The last
// statement registers the section loader with DashSections (dash-sections.js); main.js#loadSectionContent runs it.

        async function loadSectionAutomations() {
            CardState.setLoading('card-mission-history');
            await Promise.all([loadTabCronjobs(), loadMissionHistory(), loadDaemonsCard()]);
        }

        DashSections.register({ id: 'automations', load: loadSectionAutomations });
