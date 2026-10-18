// AuraGo dashboard – section "memory": memory statistics and health, weekly reflection, curation and hygiene.
// Classic script, loaded with defer in sidebar order after widgets-knowledge.js and before dashboard-events.js.
// Its declarations stay global: inline onclick strings and other dashboard scripts call them by name. The last
// statement registers the section loader with DashSections (dash-sections.js); main.js#loadSectionContent runs it.

        async function loadSectionMemory({ force = false } = {}) {
            CardState.setLoading('card-memory');
            const memDataR = await API.getWithStatus('/api/dashboard/memory', sectionFetch(force));
            const memData = memDataR.ok ? memDataR.data : null;
            memDataR.ok ? CardState.setLoaded('card-memory') : CardState.setError('card-memory', loadSectionMemory, { status: memDataR.status });
            if (memData) {
                renderMemoryStats(memData);
                // renderMemoryHealth also fills card-memory-reflection (renderWeeklyReflection).
                renderMemoryHealth(memData);
                if (Charts.memory) { Charts.memory.destroy(); Charts.memory = null; }
                Charts.memory = createMemoryBarChart('memory-chart', memData);
                renderMilestones(memData.milestones);
            }
        }

        DashSections.register({ id: 'memory', load: loadSectionMemory });
