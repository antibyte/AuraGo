// AuraGo dashboard – section "problems": error patterns (operational issues: operational-issues.js).
// Classic script, loaded with defer in sidebar order after widgets-knowledge.js and before dashboard-events.js.
// Its declarations stay global: inline onclick strings and other dashboard scripts call them by name. The last
// statement registers the section loader with DashSections (dash-sections.js); main.js#loadSectionContent runs it.

        async function loadSectionProblems({ force = false } = {}) {
            CardState.setLoading('card-error-patterns');
            const issues = typeof window.loadOperationalIssues === 'function' ? window.loadOperationalIssues() : null;
            const errorsR = await API.getWithStatus('/api/dashboard/errors', sectionFetch(force));
            const card = document.getElementById('card-error-patterns');
            if (errorsR.ok) {
                CardState.setLoaded('card-error-patterns');
                renderErrorPatterns(errorsR.data);
                // renderErrorPatterns hides #error-patterns-wrap when nothing recurs; hide the card then.
                const wrap = document.getElementById('error-patterns-wrap');
                dashSetHidden(card, !!wrap && wrap.classList.contains('is-hidden'));
            } else {
                dashSetHidden(card, false);
                CardState.setError('card-error-patterns', loadSectionProblems, { status: errorsR.status });
            }
            await issues;
        }

        DashSections.register({ id: 'problems', load: loadSectionProblems });
