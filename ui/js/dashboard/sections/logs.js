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

        // ══════════════════════════════════════════════════════════════════════════════
        // LOG VIEWER
        // ══════════════════════════════════════════════════════════════════════════════

        let logData = [];

        function renderLogs(data) {
            if (!data) return;
            logData = data.lines || [];
            applyLogFilter();
            const meta = document.getElementById('log-meta');
            if (meta) meta.textContent = t('dashboard.logs_meta', {count: data.count || 0, file: data.log_file || 'unknown'});
        }

        function applyLogFilter() {
            const viewer = document.getElementById('log-viewer');
            const filterStr = (document.getElementById('log-filter')?.value || '').trim();
            let regex = null;
            if (filterStr) {
                try { regex = new RegExp(filterStr, 'i'); } catch (e) { /* ignore invalid regex */ }
            }

            const lines = regex ? logData.filter(l => regex.test(l)) : logData;

            if (lines.length === 0) {
                viewer.innerHTML = '<div class="log-line log-line-muted">' + t('dashboard.logs_no_match') + '</div>';
                return;
            }

            viewer.innerHTML = lines.map(line => {
                let cls = '';
                if (/\blevel=ERROR\b/i.test(line)) cls = 'log-level-error';
                else if (/\blevel=WARN/i.test(line)) cls = 'log-level-warn';
                else if (/\blevel=INFO\b/i.test(line)) cls = 'log-level-info';
                else if (/\blevel=DEBUG\b/i.test(line)) cls = 'log-level-debug';
                return `<div class="log-line ${cls}">${esc(line)}</div>`;
            }).join('');
        }

        function scrollLogsToBottom() {
            const viewer = document.getElementById('log-viewer');
            if (viewer) viewer.scrollTop = viewer.scrollHeight;
        }

        DashSections.register({ id: 'logs', load: loadSectionLogs });
