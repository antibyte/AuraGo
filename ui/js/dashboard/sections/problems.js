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

        function renderErrorPatterns(data) {
            const wrap = document.getElementById('error-patterns-wrap');
            const list = document.getElementById('error-patterns-list');
            if (!wrap || !list) return;
            const frequent = data?.frequent || [];
            const recent = data?.recent || [];
            if (frequent.length === 0 && recent.length === 0) {
                dashSetHidden(wrap, true);
                return;
            }
            dashSetHidden(wrap, false);
            let html = '';
            const renderItem = (p) => {
                const resolved = p.resolution ? `<div class="error-pattern-resolution">✓ ${esc(p.resolution.substring(0, 80))}${p.resolution.length > 80 ? '…' : ''}</div>` : '';
                return `<div class="error-pattern-item">
                    <div class="error-pattern-header">
                        <span class="error-pattern-tool">${esc(p.tool_name || '?')}</span>
                        <span class="error-pattern-msg" title="${escAttr(p.error_message)}">${esc((p.error_message || '').substring(0, 60))}${(p.error_message || '').length > 60 ? '…' : ''}</span>
                        ${p.occurrence_count > 1 ? `<span class="error-pattern-count">${p.occurrence_count}×</span>` : ''}
                    </div>
                    ${resolved}
                </div>`;
            };
            // Deduplicate: show frequent, merge unique recents not already shown
            const shownIds = new Set();
            frequent.forEach(p => shownIds.add(p.id));
            if (frequent.length > 0) {
                html += `<div class="error-section-label">${t('dashboard.errors_frequent')}</div>`;
                html += renderCollapsibleList(frequent, renderItem, 5);
            }
            const newRecent = recent.filter(p => !shownIds.has(p.id));
            if (newRecent.length > 0) {
                html += `<div class="error-section-label">${t('dashboard.errors_recent')}</div>`;
                html += renderCollapsibleList(newRecent, renderItem, 5);
            }
            list.innerHTML = html;
        }

        DashSections.register({ id: 'problems', load: loadSectionProblems });
