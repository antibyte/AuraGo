// AuraGo dashboard – section "filesync": file sync (../knowledge/file_sync_status.js loads later, so the loader is wrapped).
// Classic script, loaded with defer in sidebar order after widgets-knowledge.js and before dashboard-events.js.
// Its declarations stay global: inline onclick strings and other dashboard scripts call them by name. The last
// statement registers the section loader with DashSections (dash-sections.js); main.js#loadSectionContent runs it.

        DashSections.register({ id: 'filesync', load: () => loadFileSyncStatus() });
