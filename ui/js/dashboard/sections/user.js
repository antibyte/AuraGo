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

        function renderProfile(data) {
            const container = document.getElementById('profile-content');
            if (!data || !data.categories || Object.keys(data.categories).length === 0) {
                container.innerHTML = '<div class="empty-state">' + t('dashboard.profile_empty') + '</div>';
                return;
            }
            const catIcons = { tech: '💻', prefs: '⭐', preferences: '⭐', interests: '🎯', context: '📋', comm: '💬', communication: '💬' };
            const catNameMap = {
                tech: t('dashboard.profile_cat_tech'),
                prefs: t('dashboard.profile_cat_preferences'),
                preferences: t('dashboard.profile_cat_preferences'),
                interests: t('dashboard.profile_cat_interests'),
                context: t('dashboard.profile_cat_context'),
                comm: t('dashboard.profile_cat_communication'),
                communication: t('dashboard.profile_cat_communication')
            };
            let html = '';
            for (const [cat, entries] of Object.entries(data.categories)) {
                html += `<div class="profile-category" data-cat="${escAttr(cat)}">
            <div class="profile-cat-header" onclick="toggleCategory(this)">
                <span class="profile-cat-toggle">▶</span>
                ${catIcons[cat] || '📦'} ${catNameMap[cat] || esc(cat)} (${entries.length})
            </div>
            <div class="profile-entries is-hidden">`;
                for (const e of entries) {
                    const confClass = 'conf-' + Math.min(3, Math.max(1, e.confidence || 1));
                    const firstSeen = e.first_seen ? e.first_seen.replace('T', ' ').slice(0, 16) : '';
                    const updatedAt = e.updated_at ? e.updated_at.replace('T', ' ').slice(0, 16) : '';
                    const tip = (firstSeen ? t('dashboard.profile_tooltip_created') + ' ' + firstSeen + '\n' : '') + (updatedAt ? t('dashboard.profile_tooltip_updated') + ' ' + updatedAt : '');
                    html += `<div class="profile-entry" data-search="${escAttr(e.key + ' ' + e.value)}" title="${escAttr(tip)}">
                <span class="profile-key">${esc(e.key)}</span>
                <span class="profile-val">${esc(e.value)}</span>
                <span class="confidence-badge ${confClass}" title="${t('dashboard.profile_confidence')} ${e.confidence}">${e.confidence}</span>
                <span class="profile-actions">
                    <button type="button" class="profile-btn-edit" data-cat="${escAttr(cat)}" data-key="${escAttr(e.key)}" onclick="editProfileEntry(this)" title="${t('dashboard.btn_edit')}">${t('dashboard.btn_edit')}</button>
                    <button type="button" class="profile-btn-delete" data-cat="${escAttr(cat)}" data-key="${escAttr(e.key)}" onclick="deleteProfileEntry(this)" title="${t('dashboard.btn_delete')}">${t('dashboard.btn_delete')}</button>
                </span>
                <span class="profile-edit-form is-hidden">
                    <input type="text" class="profile-edit-input" value="${escAttr(e.value)}">
                    <button class="profile-btn-save" onclick="saveProfileEntry(this)" title="${t('dashboard.profile_edit_save')}">✓</button>
                    <button class="profile-btn-cancel" onclick="cancelProfileEdit(this)" title="${t('dashboard.profile_edit_cancel')}">✗</button>
                </span>
            </div>`;
                }
                html += '</div></div>';
            }
            container.innerHTML = html;
            // Auto-open first category
            const first = container.querySelector('.profile-cat-header');
            if (first) toggleCategory(first);
        }

        function toggleCategory(header) {
            const entries = header.nextElementSibling;
            const toggle = header.querySelector('.profile-cat-toggle');
            if (entries.classList.contains('is-hidden')) {
                entries.classList.remove('is-hidden');
                toggle.classList.add('open');
            } else {
                entries.classList.add('is-hidden');
                toggle.classList.remove('open');
            }
        }

        async function deleteProfileEntry(btn) {
            const cat = btn.dataset.cat;
            const key = btn.dataset.key;
            if (!await showConfirm(t('dashboard.profile_delete_confirm') + ' "' + key + '"?')) return;
            try {
                const r = await fetch('/api/dashboard/profile/entry?' + new URLSearchParams({ category: cat, key: key }), {
                    method: 'DELETE', credentials: 'same-origin'
                });
                if (!r.ok) throw new Error('delete failed');
                loadSectionContent('user', { force: true });
            } catch (_) {
                if (typeof showToast === 'function') showToast(t('dashboard.profile_delete_error'), 'error', 4000);
            }
        }

        function editProfileEntry(btn) {
            const entry = btn.closest('.profile-entry');
            dashSetHidden(entry.querySelector('.profile-val'), true);
            dashSetHidden(entry.querySelector('.profile-actions'), true);
            const editForm = entry.querySelector('.profile-edit-form');
            editForm.classList.remove('is-hidden');
            editForm.classList.add('profile-edit-form-open');
            editForm.querySelector('.profile-edit-input').focus();
        }

        function cancelProfileEdit(btn) {
            const entry = btn.closest('.profile-entry');
            dashSetHidden(entry.querySelector('.profile-val'), false);
            dashSetHidden(entry.querySelector('.profile-actions'), false);
            const editForm = entry.querySelector('.profile-edit-form');
            editForm.classList.remove('profile-edit-form-open');
            editForm.classList.add('is-hidden');
        }

        async function saveProfileEntry(btn) {
            const entry = btn.closest('.profile-entry');
            const cat = entry.closest('.profile-category').dataset.cat;
            const key = entry.querySelector('.profile-key').textContent;
            const newVal = entry.querySelector('.profile-edit-input').value.trim();
            if (!newVal) return;
            try {
                const r = await fetch('/api/dashboard/profile/entry', {
                    method: 'PUT', credentials: 'same-origin',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ category: cat, key: key, value: newVal })
                });
                if (!r.ok) throw new Error('save failed');
                loadSectionContent('user', { force: true });
            } catch (_) {
                if (typeof showToast === 'function') showToast(t('dashboard.profile_save_error'), 'error', 4000);
            }
        }

        // ── Journal Timeline ────────────────────────────────────────────────────────
        const JOURNAL_ICONS = {
            reflection: '💭', milestone: '🏆', preference: '⭐', task_completed: '✅',
            integration: '🔌', learning: '📚', error_recovery: '🔧', system_event: '⚙️',
            decision: '🎯', error: '❌', budget_exceeded: '💸', security_event: '🔒',
            error_learned: '🧠', alert: '⚠️'
        };

        function renderJournalTimeline(entries) {
            const el = document.getElementById('journal-timeline');
            if (!el) return;
            if (!entries || entries.length === 0) {
                el.innerHTML = `<div class="empty-state">${t('dashboard.journal_empty')}</div>`;
                return;
            }
            el.innerHTML = renderCollapsibleList(entries, e => {
                const icon = JOURNAL_ICONS[e.entry_type] || '📔';
                const date = e.created_at ? new Date(e.created_at).toLocaleString(LANG, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }) : '';
                // tags is a JSON array from the backend; support both array and legacy comma-string
                const rawTags = Array.isArray(e.tags) ? e.tags : (typeof e.tags === 'string' ? e.tags.split(',') : []);
                const tags = rawTags.filter(Boolean).map(tag => `<span class="je-tag">${escapeHtml(tag.trim())}</span>`).join('');
                const auto = e.auto_generated ? ' 🤖' : '';
                const imp = e.importance || 2;
                return `<div class="journal-entry" data-importance="${imp}">
                    <div class="je-icon">${icon}</div>
                    <div class="je-body">
                        <div class="je-title">${escapeHtml(e.title || '')}${auto}</div>
                        <div class="je-meta">${date} · ${e.entry_type || 'reflection'}</div>
                        ${tags ? `<div class="je-tags">${tags}</div>` : ''}
                    </div>
                </div>`;
            }, 8);
        }

        function renderJournalSummary(summaries) {
            const el = document.getElementById('journal-summary');
            if (!el) return;
            if (!summaries || summaries.length === 0) {
                el.innerHTML = '';
                return;
            }
            const latest = summaries[0];
            el.innerHTML = `<div class="journal-summary-label">📋 ${latest.date}</div><div>${escapeHtml(latest.summary || '')}</div>`;

            // Sentiment trend + key topics
            const sentimentWrap = document.getElementById('journal-sentiment-wrap');
            const sentimentRow = document.getElementById('journal-sentiment-row');
            const topicsWrap = document.getElementById('journal-key-topics-wrap');
            if (sentimentWrap && sentimentRow) {
                const sentimentEmoji = { positive: '😊', neutral: '😐', frustrated: '😤' };
                const withSentiment = summaries.filter(s => s.sentiment);
                if (withSentiment.length > 0) {
                    dashSetHidden(sentimentWrap, false);
                    sentimentRow.innerHTML = withSentiment.map(s =>
                        `<span class="sentiment-day-badge ${escAttr(s.sentiment)}">${sentimentEmoji[s.sentiment] || '•'} ${esc(s.date)}</span>`
                    ).join('');
                } else {
                    dashSetHidden(sentimentWrap, true);
                }
                // Key topics from latest summary
                if (topicsWrap) {
                    const topics = latest.key_topics && latest.key_topics.length > 0 ? latest.key_topics : [];
                    if (topics.length > 0) {
                        topicsWrap.innerHTML = `<div class="journal-key-topics-label">${t('dashboard.journal_key_topics')}:</div><div class="journal-key-topics">${topics.map(tp => `<span class="journal-topic-chip">${esc(tp)}</span>`).join('')}</div>`;
                    } else {
                        topicsWrap.innerHTML = '';
                    }
                }
            }
        }

        function renderActivityOverview(data) {
            const summaryEl = document.getElementById('activity-overview-summary');
            const highlightsEl = document.getElementById('activity-overview-highlights');
            const pendingEl = document.getElementById('activity-overview-pending');
            const daysEl = document.getElementById('activity-overview-days');
            if (!summaryEl || !highlightsEl || !pendingEl || !daysEl) return;
            if (!data || (!data.overview_summary && !(data.days || []).length)) {
                summaryEl.innerHTML = '';
                highlightsEl.innerHTML = '';
                pendingEl.innerHTML = '';
                daysEl.innerHTML = `<div class="empty-state">${t('dashboard.activity_overview_empty')}</div>`;
                return;
            }

            summaryEl.innerHTML = data.overview_summary ? `<div class="journal-summary-label">🧭 ${t('dashboard.activity_overview_summary')}</div><div>${escapeHtml(data.overview_summary)}</div>` : '';

            const highlights = Array.isArray(data.highlights) ? data.highlights.slice(0, 3) : [];
            highlightsEl.innerHTML = highlights.length
                ? `<div class="journal-summary-label">✨ ${t('dashboard.activity_overview_highlights')}</div><div>${highlights.map(item => `<span class="journal-topic-chip">${escapeHtml(item)}</span>`).join('')}</div>`
                : '';

            const pending = Array.isArray(data.pending_items) ? data.pending_items.slice(0, 5) : [];
            pendingEl.innerHTML = pending.length
                ? `<div class="journal-summary-label">📌 ${t('dashboard.activity_overview_pending')}</div><div>${pending.map(item => `<div class="journal-entry"><div class="je-body"><div class="je-title">${escapeHtml(item)}</div></div></div>`).join('')}</div>`
                : '';

            const days = Array.isArray(data.days) ? data.days.slice(0, 3) : [];
            if (!days.length) {
                daysEl.innerHTML = `<div class="empty-state">${t('dashboard.activity_overview_empty')}</div>`;
                return;
            }
            daysEl.innerHTML = days.map(day => `
                <div class="journal-entry" data-importance="2">
                    <div class="je-icon">🗓️</div>
                    <div class="je-body">
                        <div class="je-title">${escapeHtml(day.date || '')}</div>
                        <div class="je-meta">${escapeHtml(day.summary || '')}</div>
                        ${(Array.isArray(day.highlights) && day.highlights.length) ? `<div class="je-tags">${day.highlights.slice(0, 2).map(item => `<span class="je-tag">${escapeHtml(item)}</span>`).join('')}</div>` : ''}
                    </div>
                </div>
            `).join('');
        }

        async function loadJournal() {
            const results = await Promise.all([
                API.getWithStatus('/api/dashboard/journal?limit=15'),
                API.getWithStatus('/api/dashboard/journal/summaries?days=7')
            ]);
            const [entriesR, summariesR] = results;
            if (!entriesR.ok && !summariesR.ok) {
                CardState.setError('card-journal', loadJournal, { status: entriesR.status || summariesR.status || 0 });
                return;
            }
            CardState.setLoaded('card-journal');
            const entries = entriesR.ok ? entriesR.data : null;
            const summaries = summariesR.ok ? summariesR.data : null;
            renderJournalTimeline(entries?.entries);
            renderJournalSummary(summaries?.summaries);
        }

        DashSections.register({ id: 'user', load: loadSectionUser });
