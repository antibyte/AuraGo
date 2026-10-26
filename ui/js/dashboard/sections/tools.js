// AuraGo dashboard – section "tools": activity, adaptive tool stats and tooling telemetry.
// Classic script, loaded with defer in sidebar order after widgets-knowledge.js and before dashboard-events.js.
// Its declarations stay global: inline onclick strings and other dashboard scripts call them by name. The last
// statement registers the section loader with DashSections (dash-sections.js); main.js#loadSectionContent runs it.

        async function loadSectionTools({ force = false } = {}) {
			if (window.AuraSelfTests) void window.AuraSelfTests.load();
            CardState.setLoading('card-activity');
            const opts = sectionFetch(force);
            const [activityR, toolStatsR] = await Promise.all([
                API.getWithStatus('/api/dashboard/activity', opts),
                API.getWithStatus('/api/dashboard/tool-stats', opts),
            ]);
            const activity = activityR.ok ? activityR.data : null;
            const toolStats = toolStatsR.ok ? toolStatsR.data : null;

            activityR.ok ? CardState.setLoaded('card-activity') : CardState.setError('card-activity', loadSectionTools, { status: activityR.status });

            if (activity) renderActivity(activity);
            // One tool-stats answer feeds both tool cards.
            if (toolStats) {
                renderAdaptiveToolStats(toolStats);
                renderToolingTelemetry(toolStats);
            }
        }

        function renderActivity(data) {
            if (!data) return;
            const statsEl = document.getElementById('activity-stats');
            const detailsEl = document.getElementById('activity-details');

            const cronCount = Array.isArray(data.cron_jobs) ? data.cron_jobs.length : 0;
            const procCount = Array.isArray(data.processes) ? data.processes.length : 0;
            const whCount = (data.webhooks || {}).count || 0;
            const coCount = Array.isArray(data.coagents) ? data.coagents.length : 0;
            const bgSummary = data.background_task_summary || {};
            const bgTasks = Array.isArray(data.background_tasks) ? data.background_tasks : [];
            const bgActive = (bgSummary.queued || 0) + (bgSummary.waiting || 0) + (bgSummary.running || 0);

            statsEl.innerHTML = [
                { icon: '⏰', val: cronCount, lbl: t('dashboard.activity_scheduled') },
                { icon: '🔄', val: procCount, lbl: t('dashboard.activity_processes') },
                { icon: '🔗', val: whCount, lbl: t('dashboard.activity_webhooks') },
                { icon: '🤖', val: coCount, lbl: t('dashboard.activity_coagents') },
                { icon: '🗂️', val: bgActive, lbl: t('dashboard.activity_background') },
            ].map(s =>
                `<div class="activity-stat">
            <div class="activity-stat-icon">${s.icon}</div>
            <div class="activity-stat-val">${s.val}</div>
            <div class="activity-stat-lbl">${s.lbl}</div>
        </div>`
            ).join('');

            let details = '';

            // Cron Jobs
            if (cronCount > 0) {
                details += '<div class="activity-section"><div class="activity-section-title">⏰ ' + t('dashboard.activity_scheduled_tasks') + '</div>';
                for (const job of data.cron_jobs) {
                    // Text positions take esc, the data-cron-* attributes escAttr (a prompt may contain quotes).
                    const safeId     = esc(job.id || 'unknown');
                    const safeExpr   = esc(job.cron_expr || '');
                    const attrId     = escAttr(job.id || 'unknown');
                    const safeDisabled = job.disabled ? 'true' : 'false';
                    // A flow's schedule (source "flow") is managed in EasyDrag; the server refuses edits.
                    const actions = job.source === 'flow' ? cronjobManagedLabel() : `<button class="cf-fact-btn"
                            data-cron-id="${attrId}"
                            data-cron-expr="${escAttr(job.cron_expr || '')}"
                            data-cron-prompt="${escAttr(job.task_prompt || '')}"
                            data-cron-source="${escAttr(job.source || 'agent')}"
                            data-cron-disabled="${safeDisabled}"
                            onclick="openCronEditModal(this)"
                            title="${t('dashboard.cron_edit_title')}">${t('dashboard.btn_edit')}</button>
                        <button class="cf-fact-btn danger"
                            data-cron-id="${attrId}"
                            onclick="deleteCronJob(this.dataset.cronId)"
                            title="${t('dashboard.cron_btn_delete')}">${t('dashboard.btn_delete')}</button>`;
                    details += `<div class="activity-item">
                <span class="activity-item-name">${safeId}</span>
                <div class="activity-item-row">
                    <span class="activity-item-detail">${safeExpr} — ${esc(truncate(job.task_prompt || '', 60))}</span>
                    <span class="activity-item-actions-inline">
                        ${actions}
                    </span>
                </div>
            </div>`;
                }
                details += '</div>';
            }

            // Processes
            if (procCount > 0) {
                details += '<div class="activity-section"><div class="activity-section-title">🔄 ' + t('dashboard.activity_running_processes') + '</div>';
                for (const p of data.processes) {
                    const alive = p.alive ? 'pill-running' : 'pill-idle';
                    details += `<div class="activity-item">
                <span class="activity-item-name">PID ${p.pid}</span>
                <span><span class="pill-status ${alive}">${p.alive ? t('dashboard.activity_process_active') : t('dashboard.activity_process_stopped')}</span>
                <span class="activity-item-detail activity-item-detail-spaced">${esc(p.uptime || '')}</span></span>
            </div>`;
                }
                details += '</div>';
            }

            // Co-Agents
            if (coCount > 0) {
                details += '<div class="activity-section"><div class="activity-section-title">🤖 ' + t('dashboard.activity_coagents') + '</div>';
                for (const ca of data.coagents) {
                    const stateMap = {
                        queued: t('dashboard.activity_coagent_queued'),
                        running: t('dashboard.activity_coagent_running'),
                        completed: t('dashboard.activity_coagent_completed'),
                        failed: t('dashboard.activity_coagent_failed'),
                        cancelled: t('dashboard.activity_coagent_cancelled')
                    };
                    const stateClass = ca.state === 'queued' ? 'pill-idle' :
                        ca.state === 'running' ? 'pill-running' :
                        ca.state === 'completed' ? 'pill-completed' :
                            ca.state === 'failed' ? 'pill-failed' : 'pill-idle';
                    const specIcons = { researcher: '\uD83D\uDD0D', coder: '\uD83D\uDCBB', designer: '\uD83C\uDFA8', security: '\uD83D\uDEE1\uFE0F', writer: '\u270D\uFE0F' };
                    const specBadge = ca.specialist && specIcons[ca.specialist] ? '<span class="ca-spec-icon" title="' + escAttr(ca.specialist) + '">' + specIcons[ca.specialist] + '</span>' : '';
                    const extra = [];
                    if (ca.queue_position) extra.push('Q' + esc(String(ca.queue_position)));
                    if (ca.retry_count) extra.push('R' + esc(String(ca.retry_count)));
                    if (ca.last_event) extra.push(esc(String(ca.last_event)));
                    details += `<div class="activity-item">
                <span class="activity-item-name">${specBadge}${esc(truncate(ca.task || ca.id, 50))}</span>
                <span><span class="pill-status ${stateClass}">${esc(stateMap[ca.state] || ca.state)}</span>
                <span class="activity-item-detail activity-item-detail-spaced">${esc(ca.runtime || '')}${extra.length ? ' · ' + extra.join(' · ') : ''}</span></span>
            </div>`;
                }
                details += '</div>';
            }

            // Background Tasks
            if (bgTasks.length > 0) {
                details += '<div class="activity-section"><div class="activity-section-title">🗂️ ' + t('dashboard.activity_background_tasks') + '</div>';
                for (const task of bgTasks) {
                    const status = String(task.status || 'queued');
                    const ruleBlocked = status === 'blocked' && (task.last_error === 'action_rule_required' || String(task.last_error || '').startsWith('action_rule_required:'));
                    const statusClass =
                        status === 'running' ? 'pill-running' :
                        status === 'completed' ? 'pill-completed' :
                        status === 'failed' ? 'pill-failed' :
                        status === 'waiting' || status === 'blocked' ? 'pill-warning' :
                        status === 'canceled' ? 'pill-idle' : 'pill-idle';
                    const detailParts = [
                        task.type ? esc(task.type) : '',
                        ruleBlocked ? esc(t('dashboard.activity_action_rule_required')) + ' <a href="/config#danger" target="_blank" rel="noopener">' + esc(t('dashboard.activity_action_rules')) + '</a>' : task.last_error ? esc(truncate(task.last_error, 60)) : '',
                        task.result && !task.last_error ? esc(truncate(task.result, 60)) : ''
                    ].filter(Boolean);
                    details += `<div class="activity-item">
                <span class="activity-item-name">${esc(truncate(task.description || task.id || 'background-task', 56))}</span>
                <span><span class="pill-status ${statusClass}">${esc(status === 'blocked' ? t('dashboard.activity_task_blocked') : status)}</span>
                <span class="activity-item-detail activity-item-detail-spaced">${detailParts.join(' · ')}</span></span>
            </div>`;
                }
                details += '</div>';
            }

            if (!details) {
                details = '<div class="empty-state">' + t('dashboard.activity_no_automations') + '</div>';
            }
            detailsEl.innerHTML = details;
        }

        // ══════════════════════════════════════════════════════════════════════════════
        // ADAPTIVE TOOL STATS
        // ══════════════════════════════════════════════════════════════════════════════

        function renderAdaptiveToolStats(data) {
            const card = document.getElementById('card-adaptive-tools');
            if (!data || !data.adaptive_enabled) {
                dashSetHidden(card, true);
                return;
            }

            const scores = data.adaptive_scores || [];
            const totalTracked = scores.length;
            const maxTools = data.max_tools || 0;
            const maxTotalTools = data.max_total_tools || 0;
            const lastReport = data.last_tool_filter_report || {};
            const lastToolCount = lastReport.final_tool_count || 0;
            const originalSchemaTokens = Number(lastReport.original_schema_tokens || 0);
            const finalSchemaTokens = Number(lastReport.final_schema_tokens || 0);
            const schemaReduction = originalSchemaTokens > 0
                ? Math.max(0, Math.round(((originalSchemaTokens - finalSchemaTokens) / originalSchemaTokens) * 100))
                : 0;
            const largestSchemas = Array.isArray(lastReport.largest_schemas) ? lastReport.largest_schemas : [];
            const activeCount = maxTools > 0 ? Math.min(totalTracked, maxTools) : totalTracked;
            const totalCalls = data.total_calls || 0;

            dashSetHidden(card, false);

            const kpis = document.getElementById('adaptive-tools-kpis');
            if (kpis) {
                const kpiItems = [
                    { val: `${activeCount}/${totalTracked}`, lbl: t('dashboard.adaptive_tools_active') },
                    { val: maxTools || '∞', lbl: t('dashboard.adaptive_tools_adaptive_cap') },
                    { val: maxTotalTools || '∞', lbl: t('dashboard.adaptive_tools_total_cap') },
                    { val: lastToolCount || '-', lbl: t('dashboard.adaptive_tools_last_count') },
                    { val: finalSchemaTokens ? finalSchemaTokens.toLocaleString() : '-', lbl: t('dashboard.adaptive_tools_schema_tokens') },
                    { val: originalSchemaTokens ? schemaReduction + '%' : '-', lbl: t('dashboard.adaptive_tools_schema_reduction') },
                    { val: totalCalls.toLocaleString(), lbl: t('dashboard.adaptive_tools_total_calls') },
                ];
                kpis.innerHTML = kpiItems.map(k =>
                    `<div class="prompt-kpi"><div class="prompt-kpi-val">${k.val}</div><div class="prompt-kpi-lbl">${k.lbl}</div></div>`
                ).join('');
            }

            const list = document.getElementById('adaptive-tools-list');
            if (list) {
                const maxScore = scores[0]?.score || 1;
                const maxSchemaTokens = Math.max(...largestSchemas.map(s => Number(s.rough_tokens || 0)), 1);
                const schemaHtml = largestSchemas.length > 0
                    ? `<div class="cfg-group-title cfg-group-title-underlined">${esc(t('dashboard.adaptive_tools_largest_schemas'))}</div>
                        <div class="adaptive-tools-grid">` + largestSchemas.slice(0, 5).map(s => {
                            const tokens = Number(s.rough_tokens || 0);
                            const pct = Math.max(1, Math.round((tokens / maxSchemaTokens) * 100));
                            return `<div class="adaptive-tool-row">
                                <span class="adaptive-tool-name" title="${escAttr(s.name)}">${esc(s.name)}</span>
                                <div class="adaptive-tool-bar-bg">
                                    <div class="adaptive-tool-bar-fill w-pct-${pct}"></div>
                                </div>
                                <span class="adaptive-tool-count">${tokens.toLocaleString()}</span>
                            </div>`;
                        }).join('') + `</div>`
                    : '';
                const scoreHtml = scores.length > 0 ? `<div class="adaptive-tools-grid">` +
                    scores.slice(0, 30).map((s, i) => {
                        const pct = maxScore > 0 ? Math.round((s.score / maxScore) * 100) : 0;
                        const isActive = maxTools <= 0 || i < maxTools;
                        return `<div class="adaptive-tool-row ${isActive ? '' : 'adaptive-tool-row-inactive'}">
                            <span class="adaptive-tool-name" title="${escAttr(s.tool)}">${esc(s.tool)}</span>
                            <div class="adaptive-tool-bar-bg">
                                <div class="adaptive-tool-bar-fill w-pct-${pct}"></div>
                            </div>
                            <span class="adaptive-tool-count">${s.count}×</span>
                        </div>`;
                    }).join('') + `</div>` : '';
                list.innerHTML = schemaHtml + scoreHtml;
            }
        }

        function toolingTelemetryParseLabel(key) {
            const labels = {
                native: t('dashboard.tooling_telemetry_parse_native'),
                reasoning_clean_json: t('dashboard.tooling_telemetry_parse_reasoning_clean_json'),
                content_json: t('dashboard.tooling_telemetry_parse_content_json'),
            };
            return labels[key] || key;
        }

        function toolingTelemetryClassifierLabel(key) {
            const labels = {
                classifier_empty_response_announcement_only: t('dashboard.tooling_telemetry_classifier_empty_response_announcement_only'),
                classifier_empty_response_no_output: t('dashboard.tooling_telemetry_classifier_empty_response_no_output'),
                classifier_format_error_raw_code: t('dashboard.tooling_telemetry_classifier_format_error_raw_code'),
                classifier_format_error_orphaned_bracket_tag: t('dashboard.tooling_telemetry_classifier_format_error_orphaned_bracket_tag'),
                classifier_format_error_bare_xml_in_native_mode: t('dashboard.tooling_telemetry_classifier_format_error_bare_xml_in_native_mode'),
                classifier_format_error_tool_in_fence: t('dashboard.tooling_telemetry_classifier_format_error_tool_in_fence'),
                classifier_schema_error_incomplete_tool_call: t('dashboard.tooling_telemetry_classifier_schema_error_incomplete_tool_call'),
                classifier_schema_error_xml_fallback_format: t('dashboard.tooling_telemetry_classifier_schema_error_xml_fallback_format'),
                classifier_schema_error_invalid_native_args: t('dashboard.tooling_telemetry_classifier_schema_error_invalid_native_args'),
                classifier_schema_error_missing_action_field: t('dashboard.tooling_telemetry_classifier_schema_error_missing_action_field'),
                classifier_schema_error_malformed_native_args: t('dashboard.tooling_telemetry_classifier_schema_error_malformed_native_args'),
                classifier_schema_error_unrecognized_json_structure: t('dashboard.tooling_telemetry_classifier_schema_error_unrecognized_json_structure'),
            };
            return labels[key] || key;
        }

        function toolingTelemetryRecoveryLabel(key) {
            const labels = {
                provider_422_recovered: t('dashboard.tooling_telemetry_recovery_provider_422_recovered'),
                provider_422_aborted: t('dashboard.tooling_telemetry_recovery_provider_422_aborted'),
                empty_response_recovered: t('dashboard.tooling_telemetry_recovery_empty_response_recovered'),
                duplicate_tool_call_blocked: t('dashboard.tooling_telemetry_recovery_duplicate_tool_call_blocked'),
                identical_tool_error_blocked: t('dashboard.tooling_telemetry_recovery_identical_tool_error_blocked'),
                tool_output_truncated: t('dashboard.tooling_telemetry_recovery_tool_output_truncated'),
                error_output_truncated_preserved: t('dashboard.tooling_telemetry_recovery_error_output_truncated_preserved'),
            };
            if (labels[key]) return labels[key];
            if (String(key || '').startsWith('classifier_')) {
                return toolingTelemetryClassifierLabel(key);
            }
            return key;
        }

        function toolingTelemetryPolicyLabel(key) {
            const labels = {
                conservative_profile_applied: t('dashboard.tooling_telemetry_policy_conservative_profile_applied'),
                prompt_tier_compact: t('dashboard.tooling_telemetry_policy_prompt_tier_compact'),
            };
            if (labels[key]) return labels[key];
            if (String(key || '').startsWith('family_guarded_')) {
                const family = String(key).replace('family_guarded_', '');
                return t('dashboard.tooling_telemetry_policy_family_guarded', { family });
            }
            return labels[key] || key;
        }

        function toolingTelemetryRetrievalLabel(key) {
            const rawKey = String(key || '');
            if (!rawKey) return rawKey;

            const labels = {
                rag_auto_attempt: t('dashboard.tooling_telemetry_retrieval_rag_auto_attempt'),
                rag_auto_hit: t('dashboard.tooling_telemetry_retrieval_rag_auto_hit'),
                rag_auto_miss: t('dashboard.tooling_telemetry_retrieval_rag_auto_miss'),
                rag_auto_filtered_out: t('dashboard.tooling_telemetry_retrieval_rag_auto_filtered_out'),
                rag_auto_error: t('dashboard.tooling_telemetry_retrieval_rag_auto_error'),
                rag_predictive_attempt: t('dashboard.tooling_telemetry_retrieval_rag_predictive_attempt'),
                rag_predictive_hit: t('dashboard.tooling_telemetry_retrieval_rag_predictive_hit'),
                rag_predictive_miss: t('dashboard.tooling_telemetry_retrieval_rag_predictive_miss'),
                rag_predictive_error: t('dashboard.tooling_telemetry_retrieval_rag_predictive_error'),
            };
            if (labels[rawKey]) return labels[rawKey];
            if (rawKey.startsWith('memory_prompt_share_value:')) {
                return t('dashboard.tooling_telemetry_retrieval_memory_prompt_share_value', { pct: rawKey.split(':')[1] });
            }
            if (rawKey.startsWith('rag_auto_source:')) {
                return t('dashboard.tooling_telemetry_retrieval_rag_auto_source', { source: rawKey.split(':')[1].replaceAll('_', ' ') });
            }
            if (rawKey.startsWith('rag_predictive_source:')) {
                return t('dashboard.tooling_telemetry_retrieval_rag_predictive_source', { source: rawKey.split(':')[1].replaceAll('_', ' ') });
            }
            if (rawKey.startsWith('rag_auto_latency:')) {
                return t('dashboard.tooling_telemetry_retrieval_rag_auto_latency', { range: rawKey.split(':')[1].replaceAll('_', '-') });
            }
            if (rawKey.startsWith('rag_predictive_latency:')) {
                return t('dashboard.tooling_telemetry_retrieval_rag_predictive_latency', { range: rawKey.split(':')[1].replaceAll('_', '-') });
            }
            if (rawKey.startsWith('memory_prompt_tokens:')) {
                return t('dashboard.tooling_telemetry_retrieval_memory_prompt_tokens', { range: rawKey.split(':')[1].replaceAll('_', '-') });
            }
            if (rawKey.startsWith('memory_prompt_share:')) {
                return t('dashboard.tooling_telemetry_retrieval_memory_prompt_share', { range: rawKey.split(':')[1].replaceAll('_', '-') });
            }
            return rawKey;
        }

        function toolingTelemetrySummarizeRetrieval(eventMap) {
            const entries = Object.entries(eventMap || {}).filter(([, count]) => Number(count || 0) > 0);
            let total = 0;
            let weightedShare = 0;
            let weightedShareCount = 0;
            const visibleEntries = [];

            entries.forEach(([key, count]) => {
                const numericCount = Number(count || 0);
                total += numericCount;
                if (String(key).startsWith('memory_prompt_share_value:')) {
                    const share = Number(String(key).split(':')[1] || 0);
                    if (Number.isFinite(share)) {
                        weightedShare += share * numericCount;
                        weightedShareCount += numericCount;
                    }
                    return;
                }
                visibleEntries.push([key, numericCount]);
            });

            visibleEntries.sort((a, b) => Number(b[1]) - Number(a[1]) || String(a[0]).localeCompare(String(b[0])));
            return {
                total,
                visibleEntries,
                avgShare: weightedShareCount > 0 ? (weightedShare / weightedShareCount) : 0,
                primary: visibleEntries[0] || null,
            };
        }

        function renderTelemetryGroup(container, items, labelFn) {
            if (!container) return;
            if (!items.length) {
                container.innerHTML = '<div class="empty-state dash-empty-tight">' + t('dashboard.tooling_telemetry_empty') + '</div>';
                return;
            }

            container.innerHTML = '<div class="tooling-telemetry-list">' + items.map(([key, count]) => `
                <div class="tooling-telemetry-chip">
                    <span class="tooling-telemetry-chip-value">${Number(count || 0).toLocaleString()}</span>
                    <span class="tooling-telemetry-chip-label">${esc(labelFn(key))}</span>
                </div>
            `).join('') + '</div>';
        }

        function renderToolingSummary(summaryEl, parseSources, recoveryEvents, policyEvents, retrievalSummary, failingTools) {
            if (!summaryEl) return;
            const totalRecoveries = recoveryEvents.reduce((sum, [, count]) => sum + Number(count || 0), 0);
            const totalPolicyEvents = policyEvents.reduce((sum, [, count]) => sum + Number(count || 0), 0);
            const primaryRecovery = recoveryEvents[0] ? toolingTelemetryRecoveryLabel(recoveryEvents[0][0]) : t('dashboard.tooling_telemetry_none');
            const primaryPolicy = policyEvents[0] ? toolingTelemetryPolicyLabel(policyEvents[0][0]) : t('dashboard.tooling_telemetry_none');
            const primaryRetrieval = retrievalSummary.primary ? toolingTelemetryRetrievalLabel(retrievalSummary.primary[0]) : t('dashboard.tooling_telemetry_none');
            const primaryFailureTool = failingTools[0]?.tool || t('dashboard.tooling_telemetry_none');

            const items = [
                { value: totalRecoveries.toLocaleString(), label: t('dashboard.tooling_telemetry_recoveries_total') },
                { value: totalPolicyEvents.toLocaleString(), label: t('dashboard.tooling_telemetry_policy_adjustments_total') },
                { value: retrievalSummary.total.toLocaleString(), label: t('dashboard.tooling_telemetry_retrieval_total') },
                { value: t('dashboard.tooling_telemetry_retrieval_share_short', { pct: retrievalSummary.avgShare.toFixed(0) }), label: t('dashboard.tooling_telemetry_retrieval_avg_share') },
                { value: failingTools.length.toLocaleString(), label: t('dashboard.tooling_telemetry_tools_with_failures') },
                { value: primaryRecovery, label: t('dashboard.tooling_telemetry_primary_issue') },
                { value: primaryPolicy, label: t('dashboard.tooling_telemetry_active_policy_signal') },
                { value: primaryRetrieval, label: t('dashboard.tooling_telemetry_retrieval_primary') },
                { value: primaryFailureTool, label: t('dashboard.tooling_telemetry_primary_failure_tool') },
            ];

            summaryEl.innerHTML = '<div class="tooling-telemetry-summary">' + items.map(item => `
                <div class="tooling-telemetry-summary-item">
                    <span class="tooling-telemetry-summary-value">${esc(item.value)}</span>
                    <span class="tooling-telemetry-summary-label">${esc(item.label)}</span>
                </div>
            `).join('') + '</div>';
        }

        function renderFailureTools(container, failingTools) {
            if (!container) return;
            if (!failingTools.length) {
                container.innerHTML = '<div class="empty-state dash-empty-tight">' + t('dashboard.tooling_telemetry_no_failures') + '</div>';
                return;
            }

            container.innerHTML = '<div class="tooling-telemetry-failure-list">' + renderCollapsibleList(failingTools, item => `
                <div class="tooling-telemetry-failure-item">
                    <div>
                        <span class="tooling-telemetry-failure-tool">${esc(item.tool)}</span>
                        <span class="tooling-telemetry-failure-meta">${esc(t('dashboard.tooling_telemetry_failure_count', { fails: item.failures, calls: item.total }))}</span>
                    </div>
                    <span class="tooling-telemetry-failure-badge">${Number(item.failures).toLocaleString()}</span>
                </div>
            `, 6) + '</div>';
        }

        function toolingTelemetryFamilyLabel(key) {
            const labels = {
                files: t('dashboard.tooling_telemetry_family_files'),
                shell: t('dashboard.tooling_telemetry_family_shell'),
                coding: t('dashboard.tooling_telemetry_family_coding'),
                memory: t('dashboard.tooling_telemetry_family_memory'),
                web: t('dashboard.tooling_telemetry_family_web'),
                deployment: t('dashboard.tooling_telemetry_family_deployment'),
                network: t('dashboard.tooling_telemetry_family_network'),
                infra: t('dashboard.tooling_telemetry_family_infra'),
                communication: t('dashboard.tooling_telemetry_family_communication'),
                automation: t('dashboard.tooling_telemetry_family_automation'),
                media: t('dashboard.tooling_telemetry_family_media'),
                misc: t('dashboard.tooling_telemetry_family_misc'),
            };
            return labels[key] || key;
        }

        function classifyToolFamily(tool) {
            const name = String(tool || '').toLowerCase();
            if (!name) return 'misc';
            if (name.startsWith('file') || name === 'filesystem' || name.includes('_editor') || name === 'pdf_operations' || name === 'detect_file_type' || name === 'archive') return 'files';
            if (name.includes('shell') || name.includes('sudo') || name === 'process_analyzer' || name === 'process_management') return 'shell';
            if (name.includes('python') || name.includes('sandbox') || name.includes('skill') || name.includes('generate_image') || name === 'document_creator') return 'coding';
            if (name.includes('memory') || name === 'remember' || name === 'knowledge_graph' || name === 'cheatsheet' || name.includes('journal') || name.includes('notes')) return 'memory';
            if (name.includes('web_') || name === 'site_crawler' || name === 'api_request' || name === 'virustotal_scan' || name === 'form_automation') return 'web';
            if (name.includes('homepage') || name === 'netlify' || name.startsWith('here_now_') || name.includes('update') || name === 'cloudflare_tunnel') return 'deployment';
            if (name.includes('network') || name.includes('dns_') || name.includes('port_') || name.includes('mdns_') || name.includes('whois') || name.includes('upnp') || name.includes('wake_on_lan') || name.includes('fritzbox')) return 'network';
            if (name === 'docker' || name === 'proxmox' || name === 'frigate' || name === 'tailscale' || name === 'ansible' || name === 'github' || name === 'mcp_call' || name.startsWith('sql_') || name === 'manage_sql_connections' || name.includes('meshcentral') || name.includes('remote_') || name === 'invasion_control' || name === 'home_assistant' || name === 'ollama' || name === 'adguard' || name.startsWith('mqtt_') || name === 's3_storage') return 'infra';
            if (name.includes('email') || name.includes('webhook') || name.includes('telnyx') || name === 'address_book') return 'communication';
            if (name.includes('cron') || name.includes('follow_up') || name.includes('mission') || name === 'co_agent' || name === 'co_agents') return 'automation';
            if (name.includes('image') || name.includes('audio') || name === 'tts' || name.includes('transcribe') || name.includes('media_')) return 'media';
            return 'misc';
        }

        function renderToolFamilies(container, byTool) {
            if (!container) return;
            const families = new Map();
            Object.entries(byTool || {}).forEach(([tool, stats]) => {
                const family = classifyToolFamily(tool);
                if (!families.has(family)) {
                    families.set(family, { family, total: 0, failures: 0, tools: 0 });
                }
                const entry = families.get(family);
                entry.total += Number(stats?.total_calls || 0);
                entry.failures += Number(stats?.failure_count || 0);
                entry.tools += 1;
            });

            const ranked = Array.from(families.values())
                .filter(item => item.total > 0)
                .map(item => ({
                    ...item,
                    failureRate: item.total > 0 ? item.failures / item.total : 0,
                }))
                .sort((a, b) =>
                    (b.failureRate - a.failureRate) ||
                    (b.failures - a.failures) ||
                    (b.total - a.total) ||
                    a.family.localeCompare(b.family)
                );

            if (!ranked.length) {
                container.innerHTML = '<div class="empty-state dash-empty-tight">' + t('dashboard.tooling_telemetry_no_tool_families') + '</div>';
                return;
            }

            container.innerHTML = '<div class="tooling-telemetry-family-list">' + renderCollapsibleList(ranked, item => {
                const risk = item.failureRate >= 0.35 ? t('dashboard.tooling_telemetry_family_risk_high')
                    : item.failureRate >= 0.15 ? t('dashboard.tooling_telemetry_family_risk_medium')
                    : t('dashboard.tooling_telemetry_family_risk_low');
                return `
                    <div class="tooling-telemetry-family-item">
                        <div>
                            <span class="tooling-telemetry-family-name">${esc(toolingTelemetryFamilyLabel(item.family))}</span>
                            <span class="tooling-telemetry-family-meta">${esc(t('dashboard.tooling_telemetry_family_meta', { fails: item.failures, calls: item.total, tools: item.tools }))}</span>
                        </div>
                        <div class="tooling-telemetry-family-side">
                            <span class="tooling-telemetry-family-rate">${esc(t('dashboard.tooling_telemetry_failure_rate_short', { pct: (item.failureRate * 100).toFixed(0) }))}</span>
                            <span class="tooling-telemetry-family-risk">${esc(risk)}</span>
                        </div>
                    </div>
                `;
            }, 6) + '</div>';
        }

        function renderTelemetryScopes(container, scopes) {
            if (!container) return;
            if (!scopes.length) {
                container.innerHTML = '<div class="empty-state dash-empty-tight">' + t('dashboard.tooling_telemetry_no_scopes') + '</div>';
                return;
            }

            container.innerHTML = '<div class="tooling-telemetry-scope-list">' + renderCollapsibleList(scopes, scope => {
                const parseTotal = Object.values(scope.parse_sources || {}).reduce((sum, count) => sum + Number(count || 0), 0);
                const recoveryTotal = Object.values(scope.recovery_events || {}).reduce((sum, count) => sum + Number(count || 0), 0);
                const policyTotal = Object.values(scope.policy_events || {}).reduce((sum, count) => sum + Number(count || 0), 0);
                const retrieval = toolingTelemetrySummarizeRetrieval(scope.retrieval_events || {});
                const provider = scope.provider_type || t('dashboard.tooling_telemetry_none');
                const model = scope.model || t('dashboard.tooling_telemetry_none');
                return `
                    <div class="tooling-telemetry-scope-item">
                        <div class="tooling-telemetry-scope-head">
                            <div>
                                <span class="tooling-telemetry-scope-model">${esc(model)}</span>
                                <span class="tooling-telemetry-scope-provider">${esc(provider)}</span>
                            </div>
                            <span class="tooling-telemetry-scope-total">${t('dashboard.tooling_telemetry_total_events', { count: Number(scope.total_events || 0).toLocaleString() })}</span>
                        </div>
                        <div class="tooling-telemetry-scope-meta">
                            <span class="tooling-telemetry-scope-pill">${esc(t('dashboard.tooling_telemetry_parse_sources_short', { count: parseTotal }))}</span>
                            <span class="tooling-telemetry-scope-pill">${esc(t('dashboard.tooling_telemetry_recovery_events_short', { count: recoveryTotal }))}</span>
                            <span class="tooling-telemetry-scope-pill">${esc(t('dashboard.tooling_telemetry_policy_events_short', { count: policyTotal }))}</span>
                            <span class="tooling-telemetry-scope-pill">${esc(t('dashboard.tooling_telemetry_retrieval_events_short', { count: retrieval.total }))}</span>
                            <span class="tooling-telemetry-scope-pill">${esc(t('dashboard.tooling_telemetry_retrieval_share_short', { pct: retrieval.avgShare.toFixed(0) }))}</span>
                            <span class="tooling-telemetry-scope-pill">${esc(t('dashboard.tooling_telemetry_tool_calls_short', { count: Number(scope.tool_calls || 0) }))}</span>
                            <span class="tooling-telemetry-scope-pill">${esc(t('dashboard.tooling_telemetry_success_rate_short', { pct: ((Number(scope.success_rate || 0) * 100).toFixed(0)) }))}</span>
                            <span class="tooling-telemetry-scope-pill">${esc(t('dashboard.tooling_telemetry_failure_rate_short', { pct: ((Number(scope.failure_rate || 0) * 100).toFixed(0)) }))}</span>
                        </div>
                    </div>
                `;
            }, 6) + '</div>';
        }

        function toolingTelemetryScopeStatus(item) {
            if (item.failureRate >= 0.4 || item.recoveryRate >= 0.5 || item.fallbackRate >= 0.7) {
                return {
                    key: 'struggling',
                    label: t('dashboard.tooling_telemetry_status_struggling'),
                    className: 'is-struggling',
                };
            }
            if (item.failureRate >= 0.2 || item.recoveryRate >= 0.25 || item.fallbackRate >= 0.45) {
                return {
                    key: 'mixed',
                    label: t('dashboard.tooling_telemetry_status_mixed'),
                    className: 'is-mixed',
                };
            }
            return {
                key: 'stable',
                label: t('dashboard.tooling_telemetry_status_stable'),
                className: 'is-stable',
            };
        }

        function renderTelemetryComparison(container, scopes) {
            if (!container) return;
            if (!scopes.length) {
                container.innerHTML = '<div class="empty-state dash-empty-tight">' + t('dashboard.tooling_telemetry_no_model_comparison') + '</div>';
                return;
            }

            const comparison = scopes.map(scope => {
                const parseSources = scope.parse_sources || {};
                const recoveryEvents = scope.recovery_events || {};
                const policyEvents = scope.policy_events || {};
                const retrieval = toolingTelemetrySummarizeRetrieval(scope.retrieval_events || {});
                const parseTotal = Object.values(parseSources).reduce((sum, count) => sum + Number(count || 0), 0);
                const nativeCount = Number(parseSources.native || 0);
                const fallbackCount = Math.max(0, parseTotal - nativeCount);
                const recoveryTotal = Object.values(recoveryEvents).reduce((sum, count) => sum + Number(count || 0), 0);
                const policyTotal = Object.values(policyEvents).reduce((sum, count) => sum + Number(count || 0), 0);
                const toolCalls = Number(scope.tool_calls || 0);
                return {
                    provider: scope.provider_type || t('dashboard.tooling_telemetry_none'),
                    model: scope.model || t('dashboard.tooling_telemetry_none'),
                    successRate: Number(scope.success_rate || 0),
                    failureRate: Number(scope.failure_rate || 0),
                    fallbackRate: parseTotal > 0 ? fallbackCount / parseTotal : 0,
                    recoveryRate: toolCalls > 0 ? recoveryTotal / toolCalls : 0,
                    retrievalTotal: retrieval.total,
                    retrievalAvgShare: retrieval.avgShare,
                    toolCalls,
                    policyTotal,
                    totalEvents: Number(scope.total_events || 0),
                };
            }).sort((a, b) =>
                (b.failureRate - a.failureRate) ||
                (b.retrievalAvgShare - a.retrievalAvgShare) ||
                (b.recoveryRate - a.recoveryRate) ||
                (b.fallbackRate - a.fallbackRate) ||
                (b.toolCalls - a.toolCalls) ||
                a.model.localeCompare(b.model)
            );

            container.innerHTML = '<div class="tooling-telemetry-compare-list">' + renderCollapsibleList(comparison, item => {
                const status = toolingTelemetryScopeStatus(item);
                return `
                    <div class="tooling-telemetry-compare-item">
                        <div class="tooling-telemetry-compare-head">
                            <div>
                                <span class="tooling-telemetry-compare-model">${esc(item.model)}</span>
                                <span class="tooling-telemetry-compare-provider">${esc(item.provider)}</span>
                            </div>
                            <span class="tooling-telemetry-compare-status ${status.className}">${esc(status.label)}</span>
                        </div>
                        <div class="tooling-telemetry-compare-meta">
                            <span class="tooling-telemetry-scope-pill">${esc(t('dashboard.tooling_telemetry_success_rate_short', { pct: (item.successRate * 100).toFixed(0) }))}</span>
                            <span class="tooling-telemetry-scope-pill">${esc(t('dashboard.tooling_telemetry_failure_rate_short', { pct: (item.failureRate * 100).toFixed(0) }))}</span>
                            <span class="tooling-telemetry-scope-pill">${esc(t('dashboard.tooling_telemetry_fallback_rate_short', { pct: (item.fallbackRate * 100).toFixed(0) }))}</span>
                            <span class="tooling-telemetry-scope-pill">${esc(t('dashboard.tooling_telemetry_recovery_rate_short', { pct: (item.recoveryRate * 100).toFixed(0) }))}</span>
                            <span class="tooling-telemetry-scope-pill">${esc(t('dashboard.tooling_telemetry_retrieval_events_short', { count: item.retrievalTotal }))}</span>
                            <span class="tooling-telemetry-scope-pill">${esc(t('dashboard.tooling_telemetry_retrieval_share_short', { pct: item.retrievalAvgShare.toFixed(0) }))}</span>
                            <span class="tooling-telemetry-scope-pill">${esc(t('dashboard.tooling_telemetry_policy_events_short', { count: item.policyTotal }))}</span>
                            <span class="tooling-telemetry-scope-pill">${esc(t('dashboard.tooling_telemetry_tool_calls_short', { count: item.toolCalls }))}</span>
                        </div>
                    </div>
                `;
            }, 6) + '</div>';
        }

        // Headline counters of the tooling telemetry card (P5 KPI row). Format errors are the classifier_format_error_*
        // and classifier_schema_error_* recovery events: model output that failed to parse and was recovered. Counts
        // below 1 and non-numbers are skipped, as in the lists; failing tools have at least one failed call.
        function toolingTelemetryKpis(data) {
            const telemetry = data?.agent_telemetry || {};
            const sum = (events, keep) => Object.entries(events || {}).reduce((total, [key, count]) => {
                const value = Number(count);
                return value > 0 && keep(key) ? total + value : total;
            }, 0);
            return {
                formatErrors: sum(telemetry.recovery_events, key => key.startsWith('classifier_format_error_') || key.startsWith('classifier_schema_error_')),
                recoveries: sum(telemetry.recovery_events, () => true),
                policySignals: sum(telemetry.policy_events, () => true),
                failingTools: Object.values(data?.by_tool || {}).filter(stats => Number(stats?.failure_count || 0) > 0).length,
            };
        }

        function renderToolingTelemetry(data) {
            const card = document.getElementById('card-tooling-telemetry');
            const telemetry = data?.agent_telemetry || {};
            const parseSources = Object.entries(telemetry.parse_sources || {}).filter(([, count]) => Number(count) > 0);
            const recoveryEvents = Object.entries(telemetry.recovery_events || {}).filter(([, count]) => Number(count) > 0);
            const policyEvents = Object.entries(telemetry.policy_events || {}).filter(([, count]) => Number(count) > 0);
            const retrievalSummary = toolingTelemetrySummarizeRetrieval(telemetry.retrieval_events || {});
            const scopes = Array.isArray(telemetry.scopes) ? telemetry.scopes.filter(scope => Number(scope?.total_events || 0) > 0) : [];
            const failingTools = Object.entries(data?.by_tool || {})
                .map(([tool, stats]) => ({
                    tool,
                    failures: Number(stats?.failure_count || 0),
                    total: Number(stats?.total_calls || 0),
                }))
                .filter(item => item.failures > 0)
                .sort((a, b) => (b.failures - a.failures) || (b.total - a.total) || a.tool.localeCompare(b.tool));

            if (!parseSources.length && !recoveryEvents.length && !policyEvents.length && !retrievalSummary.visibleEntries.length && !failingTools.length && !scopes.length) {
                dashSetHidden(card, true);
                return;
            }

            dashSetHidden(card, false);
            parseSources.sort((a, b) => Number(b[1]) - Number(a[1]));
            recoveryEvents.sort((a, b) => Number(b[1]) - Number(a[1]));
            policyEvents.sort((a, b) => Number(b[1]) - Number(a[1]));

            const kpis = toolingTelemetryKpis(data);
            renderDashKpiRow(document.getElementById('tooling-telemetry-kpis'), [
                { label: t('dashboard.tooling_telemetry_format_errors'), value: kpis.formatErrors },
                { label: t('dashboard.tooling_telemetry_recoveries_total'), value: kpis.recoveries },
                { label: t('dashboard.tooling_telemetry_policy_events'), value: kpis.policySignals },
                { label: t('dashboard.tooling_telemetry_tools_with_failures'), value: kpis.failingTools },
            ]);
            window.DashDisclosures?.setCount('card-tooling-telemetry:failures', kpis.failingTools);

            renderToolingSummary(document.getElementById('tooling-telemetry-summary'), parseSources, recoveryEvents, policyEvents, retrievalSummary, failingTools);
            renderTelemetryGroup(document.getElementById('tooling-telemetry-parse'), parseSources, toolingTelemetryParseLabel);
            renderTelemetryGroup(document.getElementById('tooling-telemetry-recovery'), recoveryEvents, toolingTelemetryRecoveryLabel);
            renderTelemetryGroup(document.getElementById('tooling-telemetry-policy'), policyEvents, toolingTelemetryPolicyLabel);
            renderTelemetryGroup(document.getElementById('tooling-telemetry-retrieval'), retrievalSummary.visibleEntries, toolingTelemetryRetrievalLabel);
            renderFailureTools(document.getElementById('tooling-telemetry-failures'), failingTools);
            renderToolFamilies(document.getElementById('tooling-telemetry-families'), data?.by_tool || {});
            renderTelemetryScopes(document.getElementById('tooling-telemetry-scopes'), scopes);
            renderTelemetryComparison(document.getElementById('tooling-telemetry-comparison'), scopes);
        }

        DashSections.register({ id: 'tools', load: loadSectionTools });
