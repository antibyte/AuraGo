// AuraGo dashboard – section "prompts": prompt builder analytics, optimization and output compression.
// Classic script, loaded with defer in sidebar order after widgets-knowledge.js and before dashboard-events.js.
// Its declarations stay global: inline onclick strings and other dashboard scripts call them by name. The last
// statement registers the section loader with DashSections (dash-sections.js); main.js#loadSectionContent runs it.

        async function loadSectionPrompts({ force = false } = {}) {
            CardState.setLoading('card-prompt');
            CardState.setLoading('card-optimization');
            CardState.setLoading('card-compression');
            const opts = sectionFetch(force);
            const [promptStatsR, optR, compR] = await Promise.all([
                API.getWithStatus('/api/dashboard/prompt-stats', opts),
                API.getWithStatus('/api/dashboard/optimization', opts),
                API.getWithStatus('/api/dashboard/compression', opts),
            ]);
            const promptStats = promptStatsR.ok ? promptStatsR.data : null;
            const opt = optR.ok ? optR.data : null;
            const comp = compR.ok ? compR.data : null;

            promptStatsR.ok ? CardState.setLoaded('card-prompt') : CardState.setError('card-prompt', loadSectionPrompts, { status: promptStatsR.status });
            optR.ok ? CardState.setLoaded('card-optimization') : CardState.setError('card-optimization', loadSectionPrompts, { status: optR.status });
            compR.ok ? CardState.setLoaded('card-compression') : CardState.setError('card-compression', loadSectionPrompts, { status: compR.status });

            if (promptStats) {
                renderPromptStats(promptStats);
                if (Charts.promptSize) Charts.promptSize.destroy();
                Charts.promptSize = createPromptSizeChart('prompt-size-chart', promptStats.recent);
                if (Charts.promptTier) Charts.promptTier.destroy();
                Charts.promptTier = createPromptTierChart('prompt-tier-chart', promptStats.tier_distribution);
                if (Charts.promptSavings) Charts.promptSavings.destroy();
                Charts.promptSavings = createPromptSavingsChart('prompt-savings-chart', promptStats.recent_fits);
                if (Charts.promptSectionDist) Charts.promptSectionDist.destroy();
                Charts.promptSectionDist = createPromptSectionDistChart('prompt-section-dist-chart', promptStats.avg_section_sizes);
            }
            if (opt) renderOptimizationStats(opt);
            if (comp) renderCompressionStats(comp);
        }

        // ══════════════════════════════════════════════════════════════════════════════
        // PROMPT BUILDER ANALYTICS
        // ══════════════════════════════════════════════════════════════════════════════

        function renderPromptStats(data) {
            if (!data || !(data.total_builds > 0 || data.fit_count > 0)) {
                dashSetHidden(document.getElementById('prompt-no-data'), false);
                dashSetHidden(document.getElementById('prompt-content'), true);
                return;
            }
            dashSetHidden(document.getElementById('prompt-no-data'), true);
            dashSetHidden(document.getElementById('prompt-content'), false);

            const render = (id, available, items) => {
                const el = document.getElementById(id);
                if (!el) return;
                el.innerHTML = available ? items.map(([value, key, percent]) =>
                    `<div class="prompt-kpi"><div class="prompt-kpi-val">${esc(formatPromptNumber(value, percent))}</div><div class="prompt-kpi-lbl">${esc(t('dashboard.' + key))}</div></div>`
                ).join('') : `<p class="empty-state dash-empty-tight">${esc(t('dashboard.prompt_no_measurements'))}</p>`;
            };
            render('prompt-module-kpis', data.total_builds > 0, [
                [data.avg_modules_loaded, 'prompt_kpi_avg_modules_loaded'],
                [data.avg_modules_used, 'prompt_kpi_avg_modules_used'],
                [data.avg_modules_filtered, 'prompt_kpi_filtered'],
                [data.avg_module_filter_rate_pct, 'prompt_kpi_filter_rate', true],
                [data.avg_filter_savings, 'prompt_kpi_filtered_bytes'],
            ]);
            render('prompt-kpis', data.total_builds > 0, [
                [data.total_builds, 'prompt_kpi_total_builds'],
                [data.avg_raw_len, 'prompt_kpi_raw_bytes'],
                [data.avg_optimized_len, 'prompt_kpi_opt_bytes'],
                [data.avg_optimization_pct, 'prompt_kpi_avg_saving_pct', true],
                [data.total_format_savings, 'prompt_kpi_format_bytes'],
            ]);
            const measured = data.measured_fit_count > 0;
            render('prompt-fit-kpis', data.fit_count > 0, [
                [data.fit_count, 'prompt_kpi_fit_count'],
                [measured ? data.avg_fit_input_tokens : undefined, 'prompt_kpi_input_tokens'],
                [measured ? data.avg_fit_output_tokens : undefined, 'prompt_kpi_output_tokens'],
                [measured ? data.avg_fit_saved_tokens : undefined, 'prompt_kpi_saved_tokens'],
                [measured ? data.avg_fit_input_guides : undefined, 'prompt_kpi_input_guides'],
                [measured ? data.avg_fit_output_guides : undefined, 'prompt_kpi_output_guides'],
                [data.budget_shed_count, 'prompt_kpi_budget_cuts'],
                [data.budget_exceeded_count, 'prompt_kpi_exceeded'],
                [data.shed_rate_pct, 'prompt_kpi_cut_rate', true],
            ]);
            render('prompt-cache-kpis', data.request_count > 0, [
                [data.request_count, 'prompt_kpi_requests'],
                [data.cache_hit_count, 'prompt_kpi_cache_hits'],
                [data.cache_hit_rate_pct, 'prompt_kpi_cache_rate', true],
            ]);

            // Shed section list
            const shedEl = document.getElementById('shed-list');
            const shedCounts = data.shed_section_counts || {};
            const shedKeys = Object.keys(shedCounts).sort((a, b) => shedCounts[b] - shedCounts[a]);
            if (shedKeys.length === 0) {
                shedEl.innerHTML = '<div class="empty-state dash-empty-tight">' + esc(t(data.fit_count > 0 ? 'dashboard.prompt_no_sections_shed' : 'dashboard.prompt_no_measurements')) + '</div>';
            } else {
                shedEl.innerHTML = shedKeys.map(k =>
                    `<div class="shed-item"><span>${esc(k)}</span><span class="shed-count">${shedCounts[k]}×</span></div>`
                ).join('');
            }

            // Prompt section distribution chart + legend
            const avgSections = data.avg_section_sizes || {};
            const sectionOrder = ['modules', 'memories', 'guides', 'personality', 'context'];
            const sectionNameMap = {
                modules:     t('dashboard.prompt_section_modules'),
                memories:    t('dashboard.prompt_section_memories'),
                guides:      t('dashboard.prompt_section_guides'),
                personality: t('dashboard.prompt_section_personality'),
                context:     t('dashboard.prompt_section_context'),
            };
            const legendEl = document.getElementById('prompt-section-legend');
            if (legendEl) legendEl.textContent = '';
            if (legendEl && Object.keys(avgSections).length > 0) {
                const total = sectionOrder.reduce((s, k) => s + (avgSections[k] || 0), 0);
                legendEl.innerHTML = sectionOrder
                    .filter(k => (avgSections[k] || 0) > 0)
                    .map((k, i) => {
                        const pct = formatPromptNumber(total > 0 ? avgSections[k] / total * 100 : 0, true);
                        const colorIdx = sectionOrder.indexOf(k);
                        return `<div class="prompt-section-legend-item">
                            <span class="prompt-section-legend-dot prompt-section-legend-dot-${colorIdx}"></span>
                            <span class="prompt-section-legend-label">${esc(sectionNameMap[k] || k)}</span>
                            <span class="prompt-section-legend-val">${esc(formatPromptNumber(avgSections[k]))} <span class="prompt-section-legend-pct">(${esc(pct)})</span></span>
                        </div>`;
                    }).join('');
            }
        }

        function renderOptimizationStats(opt) {
            const el = document.getElementById('opt-grid');
            if (!el || !opt) return;

            const items = [
                {
                    lbl: t('dashboard.opt_active_overrides'),
                    val: opt.active_overrides || 0
                },
                {
                    lbl: t('dashboard.opt_running_shadows'),
                    val: opt.running_shadows || 0
                },
                {
                    lbl: t('dashboard.opt_rejected_mutations'),
                    val: opt.rejected_mutations || 0
                },
                {
                    lbl: t('dashboard.opt_verified_trace_events'),
                    val: opt.total_trace_events || 0
                },
                {lbl: t('dashboard.opt_exposures'), val: opt.exposures || 0},
                {lbl: t('dashboard.opt_legacy'), val: opt.legacy_trace_events || 0},
                {
                    lbl: t('dashboard.opt_global_success_rate'),
                    val: opt.total_trace_events > 0 ? `${((opt.global_success_rate || 0) * 100).toFixed(1)}%` : '—'
                }
            ];

            el.innerHTML = items.map(s =>
                `<div class="stat-item">
                    <div class="stat-value">${s.val}</div>
                    <div class="stat-label">${s.lbl}</div>
                </div>`
            ).join('');
            if (Array.isArray(opt.variants) && opt.variants.length) {
                const rows = opt.variants.slice(0, 10).map(v => `<li>${esc(v.manual)} · ${esc(t(v.active ? 'dashboard.opt_variant_active' : v.shadow ? 'dashboard.opt_variant_shadow' : 'dashboard.opt_variant_retired'))}: ${esc(t('dashboard.opt_reason_' + v.reason))}</li>`).join('');
                el.insertAdjacentHTML('beforeend', `<details><summary>${esc(t('dashboard.opt_evidence'))}</summary><ul>${rows}</ul></details>`);
            }
        }

        // ── Output Compression Stats ───────────────────────────────────────────
        function formatChars(n) {
            if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M';
            if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K';
            return String(n);
        }

        function renderCompressionStats(comp) {
            const statsEl = document.getElementById('compression-stats');
            const detailsEl = document.getElementById('compression-details');
            const emptyEl = document.getElementById('compression-empty');
            const statusBadge = document.getElementById('compression-status-badge');
            if (!statsEl || !comp) return;

            // Enabled Status Badge aktualisieren
            if (statusBadge) {
                const enabled = comp.enabled !== false;
                statusBadge.className = `status-badge ${enabled ? 'is-active' : 'is-disabled'}`;
                statusBadge.textContent = enabled ? t('dashboard.compression_enabled') : t('dashboard.compression_disabled');
            }

            const applied = comp.compressions_applied || 0;
            const skipped = comp.compressions_skipped || 0;
            const total = applied + skipped;

            // No data yet
            if (total === 0) {
                if (statsEl) statsEl.innerHTML = '';
                if (detailsEl) detailsEl.classList.add('is-hidden');
                if (emptyEl) emptyEl.classList.remove('is-hidden');
                return;
            }
            if (emptyEl) emptyEl.classList.add('is-hidden');
            if (detailsEl) detailsEl.classList.remove('is-hidden');

            const savedChars = comp.total_saved_chars || 0;
            const ratio = comp.average_savings_ratio || 0;
            const rawChars = comp.total_raw_chars || 0;
            const compressedChars = comp.total_compressed_chars || 0;
            const avgProcessing = comp.average_processing_ms || 0;
            const errors = comp.errors_count || 0;

            const items = [
                { lbl: t('dashboard.compression_saved_chars'), val: formatChars(savedChars) },
                { lbl: t('dashboard.compression_savings_ratio'), val: (ratio * 100).toFixed(1) + '%' },
                { lbl: t('dashboard.compression_applied'), val: applied },
                { lbl: t('dashboard.compression_skipped'), val: skipped },
                { lbl: t('dashboard.compression_raw_chars'), val: formatChars(rawChars) },
                { lbl: t('dashboard.compression_compressed_chars'), val: formatChars(compressedChars) },
                { lbl: t('dashboard.compression_avg_processing_ms'), val: avgProcessing.toFixed(2) + ' ms' },
            ];

            // Errors nur anzeigen wenn > 0
            if (errors > 0) {
                items.push({ lbl: t('dashboard.compression_errors'), val: errors, class: 'is-error' });
            }

            statsEl.innerHTML = items.map(s =>
                `<div class="stat-item">
                    <div class="stat-value">${s.val}</div>
                    <div class="stat-label">${s.lbl}</div>
                </div>`
            ).join('');

            // Top tools
            const toolsList = document.getElementById('compression-tools-list');
            const toolsWrap = document.getElementById('compression-tools-wrap');
            const topTools = (comp.top_tools || []).slice(0, 5);
            if (toolsWrap) toolsWrap.classList.toggle('is-hidden', topTools.length === 0);
            if (toolsList && topTools.length > 0) {
                toolsList.innerHTML = topTools.map(e =>
                    `<div class="compression-rank-item">
                        <span class="compression-rank-name">${esc(e.tool)}</span>
                        <span class="compression-rank-bar" data-bar-width="${Math.max(4, (e.savings_ratio || 0) * 100)}"></span>
                        <span class="compression-rank-val">${formatChars(e.saved_chars)} (${(e.savings_ratio * 100).toFixed(0)}%)</span>
                    </div>`
                ).join('');
                if (typeof applyDynamicSurfaceVars === 'function') applyDynamicSurfaceVars(toolsList);
            }

            // Top filters
            const filtersList = document.getElementById('compression-filters-list');
            const filtersWrap = document.getElementById('compression-filters-wrap');
            const topFilters = (comp.top_filters || []).slice(0, 5);
            if (filtersWrap) filtersWrap.classList.toggle('is-hidden', topFilters.length === 0);
            if (filtersList && topFilters.length > 0) {
                const maxSaved = Math.max(1, topFilters[0].saved_chars || 1);
                filtersList.innerHTML = topFilters.map(e =>
                    `<div class="compression-rank-item">
                        <span class="compression-rank-name">${esc(e.filter)}</span>
                        <span class="compression-rank-bar" data-bar-width="${Math.max(4, ((e.saved_chars || 0) / maxSaved) * 100)}"></span>
                        <span class="compression-rank-val">${formatChars(e.saved_chars)} ×${e.count || 0}</span>
                    </div>`
                ).join('');
                if (typeof applyDynamicSurfaceVars === 'function') applyDynamicSurfaceVars(filtersList);
            }
        }

        DashSections.register({ id: 'prompts', load: loadSectionPrompts });
