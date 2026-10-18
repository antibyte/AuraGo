// AuraGo dashboard – section "user": profile, activity overview, journal and newspaper.
// Classic script, loaded with defer in sidebar order after widgets-knowledge.js and before dashboard-events.js.
// Its declarations stay global: inline onclick strings and other dashboard scripts call them by name. The last
// statement registers the section loader with DashSections (dash-sections.js); main.js#loadSectionContent runs it.

        async function loadSectionUser({ force = false } = {}) {
            CardState.setLoading('card-profile');
            CardState.setLoading('card-activity-overview');
            CardState.setLoading('card-journal');
            CardState.setLoading('card-newspaper');

            const opts = sectionFetch(force);
            const [profileR, activityOverviewR, newspaperCapsR, newspaperEditionsR] = await Promise.all([
                API.getWithStatus('/api/dashboard/profile', opts),
                API.getWithStatus('/api/memory/activity-overview?days=7', opts),
                API.getWithStatus('/api/desktop/newspaper/capabilities', opts),
                API.getWithStatus('/api/desktop/newspaper/editions', opts),
            ]);
            const profile = profileR.ok ? profileR.data : null;
            const activityOverview = activityOverviewR.ok ? activityOverviewR.data : null;

            profileR.ok ? CardState.setLoaded('card-profile') : CardState.setError('card-profile', loadSectionUser, { status: profileR.status });
            activityOverviewR.ok ? CardState.setLoaded('card-activity-overview') : CardState.setError('card-activity-overview', loadSectionUser, { status: activityOverviewR.status });

            if (profile) renderProfile(profile);
            if (activityOverview) renderActivityOverview(activityOverview);
            const newsStatus = document.getElementById('newspaper-dashboard-status');
            const newsNext = document.getElementById('newspaper-dashboard-next');
            if (newsStatus && newsNext) {
                const caps = newspaperCapsR.data, issues = newspaperEditionsR.data?.editions || [];
                newsStatus.textContent = !newspaperCapsR.ok ? t('dashboard.newspaper_unavailable') : !caps.enabled ? t('dashboard.newspaper_disabled') : issues.length ? t('dashboard.newspaper_latest') + ': ' + issues[0].lead : t('dashboard.newspaper_empty');
                newsNext.textContent = caps?.daily && caps.next_run ? t('dashboard.newspaper_next') + ': ' + new Date(caps.next_run).toLocaleString(LANG) : '';
                CardState.setLoaded('card-newspaper');
            }
            await loadJournal();
        }

        DashSections.register({ id: 'user', load: loadSectionUser });
