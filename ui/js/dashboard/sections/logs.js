// AuraGo dashboard – section "logs": log viewer.
// Classic script, loaded with defer in sidebar order after widgets-knowledge.js and before dashboard-events.js.
// Its declarations stay global: inline onclick strings and other dashboard scripts call them by name. The last
// statement registers the section loader with DashSections (dash-sections.js); main.js#loadSectionContent runs it.

        async function loadSectionLogs({ force = false } = {}) {
            CardState.setLoading('card-logs');
            const logResultsR = await API.getWithStatus('/api/dashboard/logs?lines=100', sectionFetch(force));
            const logResults = logResultsR.ok ? logResultsR.data : null;
            logResultsR.ok ? CardState.setLoaded('card-logs') : CardState.setError('card-logs', loadSectionLogs, { status: logResultsR.status });
            if (logResults) {
                renderLogs(logResults);
                scrollLogsToBottom();
            }
        }

        DashSections.register({ id: 'logs', load: loadSectionLogs });
