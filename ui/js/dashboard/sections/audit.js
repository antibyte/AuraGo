// AuraGo dashboard – section "audit": audit log (a section load starts at the first page).
// Classic script, loaded with defer in sidebar order after widgets-knowledge.js and before dashboard-events.js.
// Its declarations stay global: inline onclick strings and other dashboard scripts call them by name. The last
// statement registers the section loader with DashSections (dash-sections.js); main.js#loadSectionContent runs it.

        DashSections.register({ id: 'audit', load: () => loadTabAudit() });
