// AuraGo dashboard – section "personality": mood, affect panel, character notes and emotion history.
// Classic script, loaded with defer in sidebar order after widgets-knowledge.js and before dashboard-events.js.
// Its declarations stay global: inline onclick strings and other dashboard scripts call them by name. The last
// statement registers the section loader with DashSections (dash-sections.js); main.js#loadSectionContent runs it.

        async function loadSectionPersonality({ force = false } = {}) {
            CardState.setLoading('card-personality');
            const opts = sectionFetch(force);
            const [personalityR, moodHistoryR, emotionHistoryR] = await Promise.all([
                API.getWithStatus('/api/personality/state', opts),
                API.getWithStatus('/api/dashboard/mood-history?hours=' + currentMoodHours, opts),
                API.getWithStatus('/api/dashboard/emotion-history?hours=' + currentMoodHours, opts),
            ]);
            const personality = personalityR.ok ? personalityR.data : null;
            const moodHistory = moodHistoryR.ok ? moodHistoryR.data : null;
            const emotionHistory = emotionHistoryR.ok ? emotionHistoryR.data : null;

            personalityR.ok ? CardState.setLoaded('card-personality') : CardState.setError('card-personality', loadSectionPersonality, { status: personalityR.status });

            if (personality) {
                renderMoodBadge(personality);
                if (personality.enabled) {
                    if (Charts.traits) { Charts.traits.destroy(); Charts.traits = null; }
                    Charts.traits = createRadarChart('traits-chart', personality.traits);
                }
            }
            if (Charts.mood) { Charts.mood.destroy(); Charts.mood = null; }
            Charts.mood = createMoodLineChart('mood-chart', moodHistory || []);
            renderEmotionHistory(emotionHistory, personality);
        }

        DashSections.register({ id: 'personality', load: loadSectionPersonality });
