// AuraGo dashboard – section "system": system gauges, operations, integrations, guardian and GitHub repositories.
// Classic script, loaded with defer in sidebar order after widgets-knowledge.js and before dashboard-events.js.
// Its declarations stay global: inline onclick strings and other dashboard scripts call them by name. The last
// statement registers the section loader with DashSections (dash-sections.js); main.js#loadSectionContent runs it.

        const HIDDEN_INTEGRATIONS = new Set(['onedrive']);

        async function loadSectionSystem({ force = false } = {}) {
            CardState.setLoading('card-system');
            CardState.setLoading('card-operations');
            const opts = sectionFetch(force);
            const cards = Promise.all([
                loadGuardianCard(),
                typeof window.loadRealtimeSpeechStatus === 'function' ? window.loadRealtimeSpeechStatus() : null,
            ]);
            const [systemR, overviewR, githubReposR] = await Promise.all([
                API.getWithStatus('/api/dashboard/system', opts),
                // card-operations and the integration flags (the overview reads /api/dashboard/summary).
                API.getWithStatus('/api/dashboard/overview', opts),
                API.getWithStatus('/api/dashboard/github-repos', opts),
            ]);
            const system = systemR.ok ? systemR.data : null;
            const overview = overviewR.ok ? overviewR.data : null;
            const githubRepos = githubReposR.ok ? githubReposR.data : null;

            systemR.ok ? CardState.setLoaded('card-system') : CardState.setError('card-system', loadSectionSystem, { status: systemR.status });
            overviewR.ok ? CardState.setLoaded('card-operations') : CardState.setError('card-operations', loadSectionSystem, { status: overviewR.status });

            if (system) {
                if (!Charts.cpu) Charts.cpu = createGauge('cpu-chart', system.cpu?.usage_percent || 0);
                updateGauge(Charts.cpu, 'cpu-val', system.cpu?.usage_percent || 0);
                if (!Charts.ram) Charts.ram = createGauge('ram-chart', system.memory?.used_percent || 0);
                updateGauge(Charts.ram, 'ram-val', system.memory?.used_percent || 0);
                if (!Charts.disk) Charts.disk = createGauge('disk-chart', system.disk?.used_percent || 0);
                updateGauge(Charts.disk, 'disk-val', system.disk?.used_percent || 0);
                renderSystemStats(system);
            }
            if (overview) {
                renderOperations(overview);
                renderIntegrations(overview);
            }
            if (githubRepos) renderGitHubRepos(githubRepos);
            await cards;
        }

        function renderSystemStats(data) {
            if (!data) return;
            document.getElementById('net-sent').textContent = formatBytes(data.network?.bytes_sent || 0);
            document.getElementById('net-recv').textContent = formatBytes(data.network?.bytes_recv || 0);
            document.getElementById('sse-clients').textContent = data.sse_clients || 0;
            document.getElementById('uptime-val').textContent = formatUptime(data.uptime_seconds || 0);
        }

        // ══════════════════════════════════════════════════════════════════════════════
        // GITHUB REPOS WIDGET
        // ══════════════════════════════════════════════════════════════════════════════

        function renderGitHubRepos(data) {
            const card = document.getElementById('card-github');
            const container = document.getElementById('github-content');
            if (!data || !data.enabled) {
                dashSetHidden(card, true);
                return;
            }
            dashSetHidden(card, false);

            if (data.error) {
                container.innerHTML = `<div class="empty-state">⚠️ ${esc(data.error)}</div>`;
                return;
            }

            const repos = data.repos || [];
            if (repos.length === 0) {
                container.innerHTML = '<div class="empty-state">' + t('dashboard.github_no_repos') + '</div>';
                return;
            }

            // Sort: tracked first, then by updated_at
            repos.sort((a, b) => {
                if (a.tracked && !b.tracked) return -1;
                if (!a.tracked && b.tracked) return 1;
                return (b.updated_at || '').localeCompare(a.updated_at || '');
            });

            const owner = data.owner || '';
            let countInfo = `<div class="gh-count-row">
                <span class="gh-count-text">
                    👤 <strong>${esc(owner)}</strong> — ${t('dashboard.github_repositories_count', {n: repos.length})}
                </span>
            </div>`;

            const renderRepo = r => {
                const vis = r.private ? '<span class="gh-badge gh-badge-private">🔒 ' + t('dashboard.github_badge_private') + '</span>' : '<span class="gh-badge gh-badge-public">🌐 ' + t('dashboard.github_badge_public') + '</span>';
                const tracked = r.tracked ? '<span class="gh-badge gh-badge-tracked">📌 ' + t('dashboard.github_badge_tracked') + '</span>' : '';
                const lang = r.language ? `<span>💻 ${esc(r.language)}</span>` : '';
                const updated = r.updated_at ? `<span>🕐 ${new Date(r.updated_at).toLocaleDateString()}</span>` : '';
                const desc = r.description ? `<div class="gh-repo-desc">${esc(r.description)}</div>` : '';
                return `<div class="gh-repo">
                    <a href="${esc(r.html_url)}" target="_blank" rel="noopener" class="gh-repo-name">
                        📦 ${esc(r.name)} ${vis} ${tracked}
                    </a>
                    ${desc}
                    <div class="gh-repo-meta">${lang} ${updated}</div>
                </div>`;
            };
            container.innerHTML = countInfo + '<div class="gh-repo-list">' + renderCollapsibleList(repos, renderRepo, 6) + '</div>';
        }

        // ══════════════════════════════════════════════════════════════════════════════
        // OPERATIONS & INTEGRATIONS
        // ══════════════════════════════════════════════════════════════════════════════

        function renderOperations(overview) {
            if (!overview) return;
            const grid = document.getElementById('ops-grid');
            const m = overview.missions || {};
            const inv = overview.invasion || {};
            const idx = overview.indexer || {};
            const mq = overview.mqtt || {};
            const notes = overview.notes || {};
            const sec = overview.security || {};

            const items = [
                { icon: dashIcon('rocket'), val: m.total || 0, lbl: t('dashboard.operations_missions'), sub: m.running ? t('dashboard.operations_missions_active', {n: m.running}) : (m.queued ? t('dashboard.operations_missions_queued', {n: m.queued}) : t('dashboard.operations_missions_active', {n: m.enabled || 0})) },
                { icon: dashIcon('egg'), val: inv.nests || 0, lbl: t('dashboard.operations_nests'), sub: t('dashboard.operations_eggs_connected', {n: inv.connected_eggs || 0}) },
                { icon: dashIcon('folder'), val: idx.indexed_files || 0, lbl: t('dashboard.operations_indexed'), sub: idx.enabled ? t('dashboard.operations_of_files', {n: idx.total_files || 0}) : t('dashboard.operations_disabled') },
                { icon: dashIcon('wifi'), val: mq.connected ? '✓' : '✗', lbl: t('dashboard.operations_mqtt'), sub: mq.enabled ? t('dashboard.operations_buffered', {n: mq.buffer || 0}) : t('dashboard.operations_disabled') },
                { icon: dashIcon('note'), val: notes.open || 0, lbl: t('dashboard.operations_notes_open'), sub: t('dashboard.operations_notes_sub', {total: notes.total || 0, done: notes.done || 0}) },
                { icon: dashIcon('lock'), val: sec.vault_keys || 0, lbl: t('dashboard.operations_vault_keys'), sub: t('dashboard.operations_api_tokens', {n: sec.tokens || 0}) },
                { icon: dashIcon('device'), val: overview.devices || 0, lbl: t('dashboard.operations_devices'), sub: t('dashboard.operations_inventory') },
                { icon: dashIcon('brain'), val: overview.context?.has_summary ? '✓' : '✗', lbl: t('dashboard.operations_summary'), sub: t('dashboard.operations_chars_count', {n: ((overview.context?.total_chars || 0) / 1000).toFixed(1)}) },
                { icon: dashIcon('clipboard'), val: (overview.cheatsheets?.total || 0), lbl: t('dashboard.operations_cheatsheets'), sub: t('dashboard.operations_cheatsheets_active', {n: overview.cheatsheets?.active || 0}) },
                { icon: dashIcon('globe'), val: overview.tunnel?.running ? '✓' : '✗', lbl: t('dashboard.operations_tunnel'), sub: overview.tunnel?.url ? truncate(overview.tunnel.url, 24) : t('dashboard.operations_disabled') },
            ];

            grid.innerHTML = items.map(s =>
                `<div class="ops-stat">
                    <div class="ops-stat-icon">${s.icon}</div>
                    <div class="ops-stat-val">${esc(String(s.val))}</div>
                    <div class="ops-stat-lbl">${esc(s.lbl)}</div>
                    <div class="ops-stat-sub">${esc(s.sub)}</div>
                </div>`
            ).join('');
        }

        function renderIntegrations(overview) {
            if (!overview || !overview.integrations) return;
            const grid = document.getElementById('integration-grid');
            const icons = {
                telegram: dashIcon('chat'), discord: dashIcon('chat'), email: dashIcon('email'), home_assistant: dashIcon('home'),
                docker: dashIcon('docker'), co_agents: dashIcon('bot'), webhooks: dashIcon('link'), webdav: dashIcon('cloud'),
                koofr: dashIcon('cloud'), chromecast: dashIcon('monitor'), bluetooth: dashIcon('speaker'), network_shares: dashIcon('folder'), proxmox: dashIcon('device'), frigate: dashIcon('video'), go2rtc: dashIcon('video'), ruview: dashIcon('wifi'), three_d_printers: dashIcon('printer'), ollama: dashIcon('brain'),
                rocketchat: dashIcon('chat'), tailscale: dashIcon('lock'), ansible: dashIcon('wrench'), invasion: dashIcon('egg'),
                github: dashIcon('package'), mqtt: dashIcon('wifi'), budget: dashIcon('wallet'), indexing: dashIcon('folder'),
                auth: dashIcon('key'), fallback_llm: dashIcon('refresh'), helper_llm: dashIcon('feather'), personality_v2: dashIcon('drama'), user_profiling: dashIcon('user'), tts: dashIcon('speaker'),
                piper_tts: dashIcon('speaker'), supertonic_tts: dashIcon('speaker'),
                paperless_ngx: dashIcon('doc'), cloudflare_tunnel: dashIcon('cloud'),
                fritzbox: dashIcon('plug'), meshcentral: dashIcon('monitor'), a2a: dashIcon('link'),
                adguard: dashIcon('shield'), s3: dashIcon('database'), mcp: dashIcon('plug'), mcp_server: dashIcon('plug'), dograh: dashIcon('cube'),
                memory_analysis: dashIcon('brain'), llm_guardian: dashIcon('shield'), security_proxy: dashIcon('lock'),
                sandbox: dashIcon('box'), ai_gateway: dashIcon('globe'), image_generation: dashIcon('palette'),
                evomap: dashIcon('globe'),
                google_workspace: dashIcon('email'), netlify: dashIcon('rocket'), here_now: dashIcon('rocket'),
                treg: dashIcon('globe'),
                local_wikipedia: dashIcon('book'),
                homepage: dashIcon('home'), virustotal: dashIcon('virus'), brave_search: dashIcon('search'),
                firewall: dashIcon('firewall'), remote_control: dashIcon('device'), web_scraper: dashIcon('spider'),
                skill_manager: dashIcon('puzzle')
            };
            const names = {
                telegram: t('dashboard.integration_telegram'), discord: t('dashboard.integration_discord'),
                email: t('dashboard.integration_email'), home_assistant: t('dashboard.integration_home_assistant'),
                docker: t('dashboard.integration_docker'), co_agents: t('dashboard.integration_co_agents'),
                webhooks: t('dashboard.integration_webhooks'), webdav: t('dashboard.integration_webdav'),
                koofr: t('dashboard.integration_koofr'), chromecast: t('dashboard.integration_chromecast'), bluetooth: t('dashboard.integration_bluetooth'), network_shares: t('dashboard.integration_network_shares'),
                proxmox: t('dashboard.integration_proxmox'), frigate: t('dashboard.integration_frigate'), go2rtc: t('dashboard.integration_go2rtc'), ruview: 'RuView', three_d_printers: t('dashboard.integration_three_d_printers'), ollama: t('dashboard.integration_ollama'),
                rocketchat: t('dashboard.integration_rocketchat'), tailscale: t('dashboard.integration_tailscale'),
                ansible: t('dashboard.integration_ansible'), invasion: t('dashboard.integration_invasion'),
                github: t('dashboard.integration_github'), mqtt: t('dashboard.integration_mqtt'),
                budget: t('dashboard.integration_budget'), indexing: t('dashboard.integration_indexing'),
                auth: t('dashboard.integration_auth'), fallback_llm: t('dashboard.integration_fallback_llm'),
                helper_llm: t('dashboard.integration_helper_llm'),
                personality_v2: t('dashboard.integration_personality_v2'), user_profiling: t('dashboard.integration_user_profiling'),
                tts: t('dashboard.integration_tts'),
                piper_tts: t('dashboard.integration_piper_tts'), supertonic_tts: t('dashboard.integration_supertonic_tts'),
                paperless_ngx: t('dashboard.integration_paperless_ngx'),
                cloudflare_tunnel: t('dashboard.integration_cloudflare_tunnel'),
                fritzbox: t('dashboard.integration_fritzbox'),
                meshcentral: t('dashboard.integration_meshcentral'), a2a: t('dashboard.integration_a2a'),
                adguard: t('dashboard.integration_adguard'), s3: t('dashboard.integration_s3'),
                mcp: t('dashboard.integration_mcp'), mcp_server: t('dashboard.integration_mcp_server'), dograh: t('dashboard.integration_dograh'),
                memory_analysis: t('dashboard.integration_memory_analysis'),
                llm_guardian: t('dashboard.integration_llm_guardian'),
                security_proxy: t('dashboard.integration_security_proxy'),
                sandbox: t('dashboard.integration_sandbox'), ai_gateway: t('dashboard.integration_ai_gateway'),
                evomap: t('dashboard.integration_evomap'),
                image_generation: t('dashboard.integration_image_generation'),
                google_workspace: t('dashboard.integration_google_workspace'),
                netlify: t('dashboard.integration_netlify'), here_now: t('dashboard.integration_here_now'),
                treg: 'treg',
                local_wikipedia: t('dashboard.integration_local_wikipedia'),
                homepage: t('dashboard.integration_homepage'), virustotal: t('dashboard.integration_virustotal'),
                brave_search: t('dashboard.integration_brave_search'), firewall: t('dashboard.integration_firewall'),
                remote_control: t('dashboard.integration_remote_control'),
                web_scraper: t('dashboard.integration_web_scraper'),
                skill_manager: t('dashboard.integration_skill_manager')
            };

            // Sort: active first
            const sorted = Object.entries(overview.integrations)
                .filter(([key]) => !HIDDEN_INTEGRATIONS.has(key))
                .sort((a, b) => (b[1] ? 1 : 0) - (a[1] ? 1 : 0));
            grid.innerHTML = sorted.map(([key, active]) => {
                let cls = active ? 'active' : 'inactive';
                // MQTT: distinguish "enabled but disconnected" from "enabled and connected"
                if (key === 'mqtt' && active && overview.mqtt && overview.mqtt.connected === false) cls = 'active-warning';
                const content = `${icons[key] || '•'} ${esc(names[key] || key)}`;
                if (key === 'ruview') return `<a class="int-badge ${cls}" href="/desktop?app=ruview">${content}</a>`;
                if (key === 'go2rtc') return `<a class="int-badge ${cls}" href="/desktop?app=network-cameras">${content}</a>`;
                if (key === 'local_wikipedia') {
                    const update = active && overview.local_wikipedia && overview.local_wikipedia.update_available;
                    if (update) {
                        const hint = t('dashboard.local_wikipedia_update_hint', { date: update.date });
                        const title = t('dashboard.local_wikipedia_update_title', { date: update.date });
                        return `<a class="int-badge active-warning" href="/config#local_wikipedia" title="${escAttr(title)}">${content} · ${esc(hint)}</a>`;
                    }
                    return `<a class="int-badge ${cls}" href="/config#local_wikipedia">${content}</a>`;
                }
                return `<span class="int-badge ${cls}">${content}</span>`;
            }).join('');
        }

        // ── LLM Guardian Card ───────────────────────────────────────────────────────

        async function loadGuardianCard() {
            const data = await API.get('/api/dashboard/guardian');
            const card = document.getElementById('card-guardian');
            if (!data || !data.enabled) {
                dashSetHidden(card, true);
                return;
            }
            dashSetHidden(card, false);
            renderGuardianCard(data);
        }

        function renderGuardianCard(data) {
            const statusEl = document.getElementById('guardian-status');
            const metricsEl = document.getElementById('guardian-metrics');
            if (!statusEl || !metricsEl) return;

            const levelLabels = { off: '⚪ Off', low: '🟢 Low', medium: '🟡 Medium', high: '🔴 High' };
            const fsLabels = { block: '🚫 Block', quarantine: '⚠️ Quarantine', allow: '✅ Allow' };

            statusEl.innerHTML = `
                <div class="guardian-status-row">
                    <span class="guardian-lbl">${t('dashboard.guardian_level')}:</span>
                    <span class="guardian-val">${levelLabels[data.level] || data.level}</span>
                </div>
                <div class="guardian-status-row">
                    <span class="guardian-lbl">${t('dashboard.guardian_failsafe')}:</span>
                    <span class="guardian-val">${fsLabels[data.fail_safe] || data.fail_safe}</span>
                </div>`;

            const m = data.metrics;
            if (!m || m.total_checks === 0) {
                metricsEl.innerHTML = `<div class="empty-state">${t('dashboard.guardian_no_data')}</div>`;
                return;
            }

            metricsEl.innerHTML = `
                <div class="guardian-metrics-grid">
                    <div class="guardian-metric">
                        <div class="guardian-metric-val">${m.total_checks}</div>
                        <div class="guardian-metric-lbl">${t('dashboard.guardian_total')}</div>
                    </div>
                    <div class="guardian-metric">
                        <div class="guardian-metric-val ok">${m.allows}</div>
                        <div class="guardian-metric-lbl">${t('dashboard.guardian_allowed')}</div>
                    </div>
                    <div class="guardian-metric">
                        <div class="guardian-metric-val warn">${m.quarantines}</div>
                        <div class="guardian-metric-lbl">${t('dashboard.guardian_quarantined')}</div>
                    </div>
                    <div class="guardian-metric">
                        <div class="guardian-metric-val danger">${m.blocks}</div>
                        <div class="guardian-metric-lbl">${t('dashboard.guardian_blocked')}</div>
                    </div>
                    <div class="guardian-metric">
                        <div class="guardian-metric-val">${(m.cache_hit_rate * 100).toFixed(0)}%</div>
                        <div class="guardian-metric-lbl">${t('dashboard.guardian_cache_rate')}</div>
                    </div>
                    <div class="guardian-metric">
                        <div class="guardian-metric-val">${m.total_tokens}</div>
                        <div class="guardian-metric-lbl">${t('dashboard.guardian_tokens')}</div>
                    </div>
                    ${m.clarifications ? `<div class="guardian-metric">
                        <div class="guardian-metric-val">${m.clarifications}</div>
                        <div class="guardian-metric-lbl">${t('dashboard.guardian_clarifications')}</div>
                    </div>` : ''}
                    ${m.content_scans ? `<div class="guardian-metric">
                        <div class="guardian-metric-val">${m.content_scans}</div>
                        <div class="guardian-metric-lbl">${t('dashboard.guardian_content_scans')}</div>
                    </div>` : ''}
                    ${m.errors ? `<div class="guardian-metric">
                        <div class="guardian-metric-val warn">${m.errors}</div>
                        <div class="guardian-metric-lbl">${t('dashboard.guardian_errors')}</div>
                    </div>` : ''}
                </div>
                ${m.last_check_time > 0 ? `<div class="guardian-last-check">${t('dashboard.guardian_last_check')}: ${relativeTime(m.last_check_time)}</div>` : ''}`;
        }

        DashSections.register({ id: 'system', load: loadSectionSystem });
