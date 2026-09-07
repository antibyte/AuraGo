        function renderKnowledgeGraphSummary(nodes, edges, stats) {
            const grid = document.getElementById('knowledge-summary-grid');
            if (!grid) return;

            const totalNodes = stats?.total_nodes ?? nodes.length;
            const totalEdges = stats?.total_edges ?? edges.length;
            const meaningfulEdges = stats?.meaningful_edges ?? edges.length;

            const types = new Map();
            (nodes || []).forEach(node => {
                const type = node?.properties?.type || 'untyped';
                types.set(type, (types.get(type) || 0) + 1);
            });

            const statsHTML = `
                <div class="mem-stat">
                    <div class="mem-stat-val">${esc(String(totalNodes))}</div>
                    <div class="mem-stat-lbl">${t('dashboard.knowledge_nodes')}</div>
                </div>
                <div class="mem-stat">
                    <div class="mem-stat-val">${esc(String(meaningfulEdges))}</div>
                    <div class="mem-stat-lbl">${t('dashboard.knowledge_meaningful_edges')}</div>
                </div>
                <div class="mem-stat">
                    <div class="mem-stat-val">${esc(String(types.size))}</div>
                    <div class="mem-stat-lbl">${t('dashboard.knowledge_types')}</div>
                </div>
            `;

            const typeBadges = Array.from(types.entries())
                .sort((a, b) => b[1] - a[1])
                .slice(0, 6)
                .map(([type, count]) => {
                    const color = knowledgeGraphTypeColor(type);
                    return `<span class="knowledge-type-badge" data-badge-color="${esc(color)}">${esc(type)} (${count})</span>`;
                }).join('');

            grid.innerHTML = statsHTML + (typeBadges ? `<div class="knowledge-type-badges">${typeBadges}</div>` : '');
            applyDynamicSurfaceVars(grid);
        }

        function renderKnowledgeGraphHealth(health) {
            const metrics = document.getElementById('knowledge-health-metrics');
            const status = document.getElementById('knowledge-health-status');
            const consistency = document.getElementById('knowledge-health-consistency');
            if (!metrics || !status || !consistency) return;

            const semanticEnabled = !!health?.semantic_enabled;
            const stats = [
                { val: Number(health?.dirty_nodes || 0), lbl: t('dashboard.knowledge_health_dirty_nodes') },
                { val: Number(health?.dirty_edges || 0), lbl: t('dashboard.knowledge_health_dirty_edges') },
                { val: Number(health?.accepted_edges || 0), lbl: t('dashboard.knowledge_health_accepted_edges') },
                { val: Number(health?.superseded_edges || 0), lbl: t('dashboard.knowledge_health_superseded_edges') },
                { val: Number(health?.retracted_edges || 0), lbl: t('dashboard.knowledge_health_retracted_edges') },
                { val: Number(health?.open_conflicts || 0), lbl: t('dashboard.knowledge_health_open_conflicts') },
                { val: Number(health?.isolated_nodes || 0), lbl: t('dashboard.knowledge_health_isolated_nodes') },
                { val: Number(health?.label_duplicate_groups || 0), lbl: t('dashboard.knowledge_health_label_duplicate_groups') },
                { val: Number(health?.id_duplicate_groups || 0), lbl: t('dashboard.knowledge_health_id_duplicate_groups') },
                { val: Number(health?.dropped_access_hits || 0), lbl: t('dashboard.knowledge_health_dropped_hits') },
                {
                    val: semanticEnabled ? t('dashboard.knowledge_health_semantic_on') : t('dashboard.knowledge_health_semantic_off'),
                    lbl: t('dashboard.knowledge_health_semantic_enabled'),
                },
            ];
            metrics.innerHTML = stats.map(stat => `
                <div class="mem-stat">
                    <div class="mem-stat-val">${esc(String(stat.val))}</div>
                    <div class="mem-stat-lbl">${esc(stat.lbl)}</div>
                </div>
            `).join('');

            const pills = [];
            if (health?.needs_reindex) {
                pills.push(`<span class="pill-status pill-warning">${t('dashboard.knowledge_health_needs_reindex')}</span>`);
            } else {
                pills.push(`<span class="pill-status pill-completed">${t('dashboard.knowledge_health_index_ok')}</span>`);
            }
            if (health?.reindex_backlog) {
                pills.push(`<span class="pill-status pill-warning">${t('dashboard.knowledge_health_reindex_backlog')}</span>`);
            }
            status.innerHTML = pills.join('');
            renderKnowledgeGraphConsistency(consistency, health?.consistency);
        }

        function renderKnowledgeGraphConsistency(container, consistency) {
            if (!container) return;
            container.setAttribute('aria-label', t('dashboard.knowledge_health_consistency_title'));
            const rows = [
                { key: 'nodes_missing_from_index', val: Number(consistency?.nodes_missing_from_index || 0), lbl: t('dashboard.knowledge_health_consistency_nodes_missing') },
                { key: 'edges_missing_from_index', val: Number(consistency?.edges_missing_from_index || 0), lbl: t('dashboard.knowledge_health_consistency_edges_missing') },
                { key: 'stale_nodes', val: Number(consistency?.stale_nodes || 0), lbl: t('dashboard.knowledge_health_consistency_stale_nodes') },
                { key: 'index_orphans', val: Number(consistency?.index_orphans || 0), lbl: t('dashboard.knowledge_health_consistency_index_orphans') },
            ].filter(row => row.val > 0);
            if (!rows.length) {
                container.innerHTML = `<div class="empty-state">${t('dashboard.knowledge_health_consistency_ok')}</div>`;
                return;
            }
            container.innerHTML = rows.map(row => `
                <div class="kg-health-row" data-consistency-key="${esc(row.key)}">
                    <span>${esc(row.lbl)}</span>
                    <strong>${esc(String(row.val))}</strong>
                </div>
            `).join('');
        }

        function renderKnowledgeGraphDuplicateCandidates(container, candidates, emptyKey, allowMerge) {
            if (!container) return;
            const rows = Array.isArray(candidates) ? candidates : [];
            if (!rows.length) {
                container.innerHTML = `<div class="empty-state">${t(emptyKey)}</div>`;
                return;
            }
            let html = '<div class="kg-table-wrap"><table class="kg-table kg-table-compact"><thead><tr>' +
                `<th>${t('dashboard.kg_col_label')}</th>` +
                `<th>${t('dashboard.kg_col_count')}</th>` +
                `<th>${t('dashboard.kg_col_id')}</th>` +
                (allowMerge ? `<th>${t('dashboard.kg_col_actions')}</th>` : '') +
                '</tr></thead><tbody>';
            rows.forEach(candidate => {
                const rawIDs = Array.isArray(candidate.ids) ? candidate.ids.filter(id => String(id || '').trim()) : [];
                const recommendedTargetID = String(candidate.recommended_target_id || '').trim();
                if (recommendedTargetID && rawIDs.includes(recommendedTargetID)) {
                    rawIDs.sort((left, right) => Number(right === recommendedTargetID) - Number(left === recommendedTargetID));
                }
                const idLinks = rawIDs.map(id =>
                    `<span class="kg-cell-link" data-kg-open-node="${esc(id)}">${esc(id)}</span>`
                ).join(', ');
                const mergeButtons = allowMerge ? rawIDs.flatMap(targetID =>
                    rawIDs.filter(sourceID => sourceID !== targetID).map(sourceID => {
                        const direction = `${sourceID} → ${targetID}`;
                        return `
                            <button type="button" class="btn btn-secondary btn-sm"
                                data-kg-merge-source="${esc(sourceID)}"
                                data-kg-merge-target="${esc(targetID)}"
                                data-kg-merge-label="${esc(candidate.label || candidate.normalized_label || 'Node')}"
                                title="${esc(t('dashboard.knowledge_quality_merge_btn'))}: ${esc(direction)}">
                                ${esc(direction)}
                            </button>`;
                    })
                ).join(' ') : '';
                html += `<tr>
                    <td>${esc(candidate.label || candidate.normalized_label || 'Node')}</td>
                    <td class="text-secondary">${Number(candidate.count || 0)}</td>
                    <td class="text-secondary">${idLinks || '—'}</td>
                    ${allowMerge ? `<td class="kg-merge-actions">${mergeButtons || '—'}</td>` : ''}
                </tr>`;
            });
            html += '</tbody></table></div>';
            container.innerHTML = html;
        }

        function renderKnowledgeGraphQuality(report) {
            const metrics = document.getElementById('knowledge-quality-metrics');
            const isolated = document.getElementById('knowledge-quality-isolated');
            const untyped = document.getElementById('knowledge-quality-untyped');
            const generic = document.getElementById('knowledge-quality-generic');
            const duplicates = document.getElementById('knowledge-quality-duplicates');
            const idDuplicates = document.getElementById('knowledge-quality-id-duplicates');
            if (!metrics || !isolated || !untyped || !generic || !duplicates || !idDuplicates) return;

            const stats = [
                { val: Number(report?.protected_nodes || 0), lbl: t('dashboard.knowledge_quality_protected') },
                { val: Number(report?.pending_edges || 0), lbl: t('dashboard.knowledge_quality_pending_edges') },
                { val: Number(report?.low_confidence_edges || 0), lbl: t('dashboard.knowledge_quality_low_confidence_edges') },
                { val: Number(report?.pending_co_mention_edges || 0), lbl: t('dashboard.knowledge_quality_pending_co_mentions') },
                { val: Number(report?.co_mention_edges || 0), lbl: t('dashboard.knowledge_quality_co_mentions') },
                { val: Number(report?.semantic_edges || 0), lbl: t('dashboard.knowledge_quality_semantic_edges') },
                { val: Number(report?.generic_nodes || 0), lbl: t('dashboard.knowledge_quality_generic_nodes') },
                { val: Number(report?.duplicate_groups || 0) + Number(report?.id_duplicate_groups || 0), lbl: t('dashboard.knowledge_quality_duplicate_suggestions') },
                { val: Number(report?.isolated_nodes || 0), lbl: t('dashboard.knowledge_quality_isolated') },
                { val: Number(report?.untyped_nodes || 0), lbl: t('dashboard.knowledge_quality_untyped') },
                { val: Number(report?.duplicate_groups || 0), lbl: t('dashboard.knowledge_quality_label_duplicates') },
                { val: Number(report?.id_duplicate_groups || 0), lbl: t('dashboard.knowledge_quality_id_duplicates') },
            ];
            metrics.innerHTML = stats.map(stat => `
                <div class="mem-stat">
                    <div class="mem-stat-val">${esc(String(stat.val))}</div>
                    <div class="mem-stat-lbl">${esc(stat.lbl)}</div>
                </div>
            `).join('');

            renderKnowledgeGraphQualityNodeList(isolated, report?.isolated_sample, 'dashboard.knowledge_quality_empty_isolated');
            renderKnowledgeGraphQualityNodeList(untyped, report?.untyped_sample, 'dashboard.knowledge_quality_empty_untyped');
            renderKnowledgeGraphQualityNodeList(generic, report?.generic_sample, 'dashboard.knowledge_quality_empty_generic');
            renderKnowledgeGraphDuplicateCandidates(duplicates, report?.duplicate_candidates, 'dashboard.knowledge_quality_empty_duplicates', false);
            renderKnowledgeGraphDuplicateCandidates(idDuplicates, report?.id_duplicate_candidates, 'dashboard.knowledge_quality_empty_id_duplicates', true);
        }

        function renderKnowledgeGraphQualityNodeList(container, nodes, emptyKey) {
            if (!container) return;
            if (!Array.isArray(nodes) || nodes.length === 0) {
                container.innerHTML = `<div class="empty-state">${t(emptyKey)}</div>`;
                return;
            }
            let html = '<table class="kg-table kg-table-compact"><thead><tr>' +
                `<th>${t('dashboard.kg_col_label')}</th>` +
                `<th>${t('dashboard.kg_col_type')}</th>` +
                `<th>${t('dashboard.kg_col_id')}</th>` +
                '</tr></thead><tbody>';
            nodes.forEach(node => {
                html += `<tr>
                    <td class="kg-cell-link" data-kg-open-node="${esc(node.id || '')}">${esc(node.label || node.id || 'Node')}</td>
                    <td class="text-secondary">${esc(node?.properties?.type || '—')}</td>
                    <td class="text-secondary kg-cell-id">${esc(node.id || '')}</td>
                </tr>`;
            });
            html += '</tbody></table>';
            container.innerHTML = html;
        }

        function renderKnowledgeGraphLists(nodes, edges) {
            renderKnowledgeNodeTable(document.getElementById('knowledge-node-list'), nodes);
            renderKnowledgeEdgeTable(document.getElementById('knowledge-edge-list'), edges);
        }

        function renderKnowledgeNodeTable(container, nodes) {
            if (!container) return;
            if (!Array.isArray(nodes) || nodes.length === 0) {
                container.innerHTML = `<div class="empty-state">${t('dashboard.knowledge_empty')}</div>`;
                return;
            }
            let html = `<table class="kg-table"><thead><tr>
                <th>${t('dashboard.kg_col_label')}</th>
                <th>${t('dashboard.kg_col_type')}</th>
                <th>${t('dashboard.kg_col_id')}</th>
                <th>${t('dashboard.kg_col_source')}</th>
                <th>${t('dashboard.kg_col_score')}</th>
                <th></th>
            </tr></thead><tbody>`;
            nodes.forEach(node => {
                const typeColor = node?.properties?.type ? knowledgeGraphTypeColor(node.properties.type) : '';
                const typeCell = typeColor
                    ? `<td><span class="kg-type-badge" data-badge-color="${esc(typeColor)}">${esc(node.properties.type)}</span></td>`
                    : '<td class="text-secondary">—</td>';
                const score = typeof node.importance_score === 'number' ? node.importance_score : '—';
                const flags = node.protected ? '<span class="kg-flag-protected" title="Protected">🔒</span>' : '';
                html += `<tr>
                    <td class="kg-cell-link" data-kg-open-node="${esc(node.id || '')}">${esc(node.label || node.id || 'Node')}</td>
                    ${typeCell}
                    <td class="text-secondary kg-cell-id">${esc(node.id || '')}</td>
                    <td class="text-secondary">${esc(node?.properties?.source || '—')}</td>
                    <td class="text-secondary">${score}</td>
                    <td>${flags}</td>
                </tr>`;
            });
            html += '</tbody></table>';
            container.innerHTML = html;
            applyDynamicSurfaceVars(container);
        }

        function renderKnowledgeEdgeTable(container, edges) {
            if (!container) return;
            if (!Array.isArray(edges) || edges.length === 0) {
                container.innerHTML = `<div class="empty-state">${t('dashboard.knowledge_empty')}</div>`;
                return;
            }
            let html = `<table class="kg-table"><thead><tr>
                <th>${t('dashboard.kg_col_relation')}</th>
                <th>${t('dashboard.kg_col_source')}</th>
                <th>${t('dashboard.kg_col_target')}</th>
            </tr></thead><tbody>`;
            edges.forEach(edge => {
                html += `<tr>
                    <td class="kg-cell-link">${esc(edge.relation || '')}</td>
                    <td class="kg-cell-link" data-kg-open-node="${esc(edge.source || '')}">${esc(edge.source || '')}</td>
                    <td class="kg-cell-link" data-kg-open-node="${esc(edge.target || '')}">${esc(edge.target || '')}</td>
                </tr>`;
            });
            html += '</tbody></table>';
            container.innerHTML = html;
        }

        function renderKnowledgeProps(properties) {
            if (!properties || typeof properties !== 'object') return '';
            const entries = Object.entries(properties)
                .filter(([key, value]) => value && !['source', 'extracted_at'].includes(String(key)))
                .slice(0, 4);
            if (!entries.length) return '';
            return entries.map(([key, value]) => `
                <span class="knowledge-item-prop">${escapeHtml(String(key))}: ${escapeHtml(truncate(String(value), 42))}</span>
            `).join('');
        }

        function renderKnowledgeGraphSearchState(query, payload) {
            const meta = document.getElementById('knowledge-search-meta');
            const results = document.getElementById('knowledge-search-results');
            if (!meta || !results) return;

            if (!query) {
                meta.textContent = t('dashboard.knowledge_search_hint');
                results.innerHTML = `<div class="empty-state">${t('dashboard.knowledge_no_search')}</div>`;
                return;
            }

            const nodes = Array.isArray(payload?.nodes) ? payload.nodes : [];
            const edges = Array.isArray(payload?.edges) ? payload.edges : [];
            meta.textContent = t('dashboard.knowledge_search_meta', { query, nodes: nodes.length, edges: edges.length });

            if (!nodes.length && !edges.length) {
                results.innerHTML = `<div class="empty-state">${t('dashboard.knowledge_search_empty')}</div>`;
                return;
            }

            let html = '<table class="kg-table kg-table-search"><thead><tr>' +
                `<th>${t('dashboard.kg_col_kind')}</th>` +
                `<th>${t('dashboard.kg_col_primary')}</th>` +
                `<th>${t('dashboard.kg_col_secondary')}</th>` +
                `<th>${t('dashboard.kg_col_type')}</th>` +
                '</tr></thead><tbody>';

            nodes.forEach(node => {
                html += `<tr>
                    <td><span class="kg-kind-badge">${t('dashboard.knowledge_nodes')}</span></td>
                    <td class="kg-cell-link" data-kg-open-node="${esc(node.id || '')}">${esc(node.label || node.id || 'Node')}</td>
                    <td class="text-secondary">${esc(node.id || '')}</td>
                    <td class="text-secondary">${esc(node?.properties?.type || '—')}</td>
                </tr>`;
            });
            edges.forEach(edge => {
                html += `<tr>
                    <td><span class="kg-kind-badge">${t('dashboard.knowledge_edges')}</span></td>
                    <td class="kg-cell-link">${esc(edge.relation || '')}</td>
                    <td class="text-secondary">${esc(edge.source || '')} → ${esc(edge.target || '')}</td>
                    <td class="text-secondary">—</td>
                </tr>`;
            });
            html += '</tbody></table>';
            results.innerHTML = html;
        }

        async function executeKnowledgeGraphSearch(query) {
            const payload = await API.get('/api/knowledge-graph/search?q=' + encodeURIComponent(query));
            renderKnowledgeGraphSearchState(query, payload || { nodes: [], edges: [] });
        }

        function applyDynamicSurfaceVars(root) {
            const scope = root || document;
            scope.querySelectorAll('[data-badge-color]').forEach((el) => {
                const color = el.getAttribute('data-badge-color');
                if (color) el.style.setProperty('--badge-color', color);
            });
            scope.querySelectorAll('[data-dot-color]').forEach((el) => {
                const color = el.getAttribute('data-dot-color');
                if (color) el.style.setProperty('--dot-color', color);
            });
            scope.querySelectorAll('[data-bar-width]').forEach((el) => {
                const width = el.getAttribute('data-bar-width');
                if (width) el.style.setProperty('--bar-width', width + '%');
            });
        }

        let _kgDetailSeq = 0;
        let _kgDetailAbort = null;

        function renderKnowledgeGraphDetailEmpty() {
        }

        function openKGDetailModal(nodeID, triggerEl) {
            const overlay = document.getElementById('kgDetailOverlay');
            const body = document.getElementById('kgDetailBody');
            if (!overlay || !body) return;
            KnowledgeGraphState.modalNodeId = nodeID;
            KnowledgeGraphState.modalTriggerEl = triggerEl || null;
            body.innerHTML = `<div class="empty-state">${t('dashboard.knowledge_detail_loading')}</div>`;
            overlay.classList.add('open');
            loadKnowledgeGraphNodeDetail(nodeID);
        }

        function closeKGDetailModal() {
            const overlay = document.getElementById('kgDetailOverlay');
            if (!overlay) return;
            overlay.classList.remove('open');
            KnowledgeGraphState.modalNodeId = '';
            KnowledgeGraphState.editingNodeId = '';
            KnowledgeGraphState.editingEdgeKey = '';
            if (_kgDetailAbort) {
                _kgDetailAbort.abort();
                _kgDetailAbort = null;
            }
            const trigger = KnowledgeGraphState.modalTriggerEl;
            if (trigger && typeof trigger.focus === 'function') {
                setTimeout(() => trigger.focus(), 100);
            }
            KnowledgeGraphState.modalTriggerEl = null;
        }

        async function loadKnowledgeGraphNodeDetail(nodeID) {
            const body = document.getElementById('kgDetailBody');
            if (!body || !nodeID) return;

            const seq = ++_kgDetailSeq;
            if (_kgDetailAbort) _kgDetailAbort.abort();
            _kgDetailAbort = new AbortController();
            const signal = _kgDetailAbort.signal;

            body.innerHTML = `<div class="empty-state">${t('dashboard.knowledge_detail_loading')}</div>`;
            let payload = null;
            try {
                const resp = await fetch('/api/knowledge-graph/node?id=' + encodeURIComponent(nodeID) + '&limit=20', {
                    credentials: 'same-origin',
                    signal,
                });
                if (!resp.ok) throw new Error('detail fetch failed');
                payload = await resp.json();
            } catch (err) {
                if (err && err.name === 'AbortError') return;
                if (seq !== _kgDetailSeq) return;
                body.innerHTML = `<div class="empty-state">${t('dashboard.knowledge_detail_missing')}</div>`;
                return;
            }
            if (seq !== _kgDetailSeq) return;

            const node = payload?.node;
            const neighbors = Array.isArray(payload?.neighbors) ? payload.neighbors : [];
            const edges = Array.isArray(payload?.edges) ? payload.edges : [];

            if (!node) {
                KnowledgeGraphState.focusNodeId = '';
                KnowledgeGraphState.focusPayload = null;
                KnowledgeGraphState.editingEdgeKey = '';
                renderKnowledgeGraphVisual();
                body.innerHTML = `<div class="empty-state">${t('dashboard.knowledge_detail_missing')}</div>`;
                return;
            }

            KnowledgeGraphState.focusNodeId = node.id || nodeID;
            KnowledgeGraphState.focusPayload = payload;
            renderKnowledgeGraphVisual();

            const isEditing = KnowledgeGraphState.editingNodeId === node.id;
            const isProtected = !!node.protected;
            const filteredNodeProperties = filterKnowledgeGraphEditableProperties(node.properties);
            const editingEdge = edges.find(edge => knowledgeGraphEdgeIdentity(edge) === KnowledgeGraphState.editingEdgeKey) || null;

            const nodeProps = Object.entries(filteredNodeProperties).map(([key, value]) => `
                <div class="knowledge-detail-row"><strong>${escapeHtml(String(key))}</strong>: ${escapeHtml(String(value))}</div>
            `).join('') || `<div class="knowledge-detail-row">${t('dashboard.knowledge_empty')}</div>`;

            const neighborRows = neighbors.map(neighbor => `
                <div class="knowledge-detail-row clickable" data-kg-open-node="${escapeHtml(neighbor.id || '')}"><strong>${escapeHtml(neighbor.label || neighbor.id || '')}</strong> <span class="knowledge-detail-id">${escapeHtml(neighbor.id || '')}</span></div>
            `).join('') || `<div class="knowledge-detail-row">${t('dashboard.knowledge_detail_no_neighbors')}</div>`;

            const edgeRows = edges.map(edge => `
                <div class="knowledge-detail-row knowledge-edge-row">
                    <div><strong>${escapeHtml(edge.relation || '')}</strong>: ${escapeHtml(edge.source || '')} → ${escapeHtml(edge.target || '')}</div>
                    <div class="knowledge-detail-actions">
                        <button type="button" class="btn btn-secondary btn-sm" onclick="toggleKnowledgeGraphEdgeEdit('${escapeJsString(knowledgeGraphEdgeIdentity(edge))}')">${KnowledgeGraphState.editingEdgeKey === knowledgeGraphEdgeIdentity(edge) ? t('dashboard.knowledge_action_cancel') : t('dashboard.knowledge_action_edit')}</button>
                        <button type="button" class="btn btn-danger btn-sm" onclick="deleteKnowledgeGraphEdge('${escapeJsString(edge.source || '')}', '${escapeJsString(edge.target || '')}', '${escapeJsString(edge.relation || '')}')">${t('dashboard.knowledge_edge_delete')}</button>
                    </div>
                </div>
            `).join('') || `<div class="knowledge-detail-row">${t('dashboard.knowledge_detail_no_edges')}</div>`;

            if (seq !== _kgDetailSeq) return;

            body.innerHTML = `
                <div class="knowledge-detail-panel">
                    <div class="knowledge-detail-head">
                        <div>
                            <div class="knowledge-detail-title">${escapeHtml(node.label || node.id || 'Node')}</div>
                            <div class="knowledge-detail-id">${escapeHtml(node.id || '')}</div>
                        </div>
                        <div class="knowledge-detail-actions">
                            ${isProtected ? `<span class="knowledge-item-badge knowledge-item-badge-protected">${t('dashboard.knowledge_detail_protected')}</span>` : ''}
                            <button type="button" class="btn btn-secondary btn-sm" onclick="toggleKnowledgeGraphEdit('${escapeJsString(node.id || '')}')">${isEditing ? t('dashboard.knowledge_action_cancel') : t('dashboard.knowledge_action_edit')}</button>
                            <button type="button" class="btn btn-secondary btn-sm" onclick="toggleKnowledgeGraphProtection('${escapeJsString(node.id || '')}', ${isProtected ? 'false' : 'true'})">${isProtected ? t('dashboard.knowledge_action_unprotect') : t('dashboard.knowledge_action_protect')}</button>
                            <button type="button" class="btn btn-danger btn-sm" onclick="deleteKnowledgeGraphNode('${escapeJsString(node.id || '')}', '${escapeJsString(node.label || node.id || 'Node')}')">${t('dashboard.knowledge_action_delete')}</button>
                        </div>
                    </div>
                    ${renderKnowledgeGraphNodeEditor(node, isEditing)}
                    ${renderKnowledgeGraphEdgeEditor(editingEdge)}
                    <div class="knowledge-detail-grid">
                        <div class="knowledge-detail-section">
                            <h4 class="card-section-title">${t('dashboard.knowledge_detail_properties')}</h4>
                            <div class="knowledge-detail-list">${nodeProps}</div>
                        </div>
                        <div class="knowledge-detail-section">
                            <h4 class="card-section-title">${t('dashboard.knowledge_detail_neighbors')}</h4>
                            <div class="knowledge-detail-list">${neighborRows}</div>
                        </div>
                        <div class="knowledge-detail-section">
                            <h4 class="card-section-title">${t('dashboard.knowledge_detail_edges')}</h4>
                            <div class="knowledge-detail-list">${edgeRows}</div>
                        </div>
                    </div>
                </div>
            `;
        }

        function resetKnowledgeGraphFocus() {
            KnowledgeGraphState.focusNodeId = '';
            KnowledgeGraphState.focusPayload = null;
            KnowledgeGraphState.editingNodeId = '';
            KnowledgeGraphState.editingEdgeKey = '';
            renderKnowledgeGraphVisual();
            closeKGDetailModal();
        }

        function filterKnowledgeGraphEditableProperties(properties) {
            const out = {};
            Object.entries(properties || {}).forEach(([key, value]) => {
                if (!key || ['protected', 'access_count'].includes(String(key))) return;
                out[key] = value;
            });
            return out;
        }

        function renderKnowledgeGraphNodeEditor(node, isEditing) {
            if (!isEditing) return '';
            const propsJSON = JSON.stringify(filterKnowledgeGraphEditableProperties(node.properties), null, 2);
            return `
                <div class="knowledge-detail-editor">
                    <h4 class="card-section-title">${t('dashboard.knowledge_editor_title')}</h4>
                    <div class="knowledge-editor-grid">
                        <label class="knowledge-editor-field">
                            <span>${t('dashboard.knowledge_editor_label')}</span>
                            <input type="text" id="knowledge-edit-label" class="profile-search" value="${escapeHtml(node.label || '')}">
                        </label>
                        <label class="knowledge-editor-field">
                            <span>${t('dashboard.knowledge_editor_properties')}</span>
                            <textarea id="knowledge-edit-properties" class="knowledge-editor-textarea" spellcheck="false">${escapeHtml(propsJSON)}</textarea>
                        </label>
                    </div>
                    <div class="knowledge-editor-hint">${t('dashboard.knowledge_editor_hint')}</div>
                    <div class="knowledge-detail-actions">
                        <button type="button" class="btn btn-primary btn-sm" onclick="saveKnowledgeGraphNodeEdit('${escapeJsString(node.id || '')}')">${t('dashboard.knowledge_action_save')}</button>
                        <button type="button" class="btn btn-secondary btn-sm" onclick="toggleKnowledgeGraphEdit('${escapeJsString(node.id || '')}')">${t('dashboard.knowledge_action_cancel')}</button>
                    </div>
                </div>
            `;
        }

        function renderKnowledgeGraphEdgeEditor(edge) {
            if (!edge) return '';
            const propsJSON = JSON.stringify(filterKnowledgeGraphEditableProperties(edge.properties), null, 2);
            return `
                <div class="knowledge-detail-editor">
                    <h4 class="card-section-title">${t('dashboard.knowledge_edge_editor_title')}</h4>
                    <div class="knowledge-editor-grid">
                        <label class="knowledge-editor-field">
                            <span>${t('dashboard.knowledge_edge_editor_relation')}</span>
                            <input type="text" id="knowledge-edit-edge-relation" class="profile-search" value="${escapeHtml(edge.relation || '')}">
                        </label>
                        <label class="knowledge-editor-field">
                            <span>${t('dashboard.knowledge_edge_editor_properties')}</span>
                            <textarea id="knowledge-edit-edge-properties" class="knowledge-editor-textarea" spellcheck="false">${escapeHtml(propsJSON)}</textarea>
                        </label>
                    </div>
                    <div class="knowledge-editor-hint">${t('dashboard.knowledge_editor_hint')}</div>
                    <div class="knowledge-detail-actions">
                        <button type="button" class="btn btn-primary btn-sm" onclick="saveKnowledgeGraphEdgeEdit('${escapeJsString(edge.source || '')}', '${escapeJsString(edge.target || '')}', '${escapeJsString(edge.relation || '')}')">${t('dashboard.knowledge_action_save')}</button>
                        <button type="button" class="btn btn-secondary btn-sm" onclick="toggleKnowledgeGraphEdgeEdit('${escapeJsString(knowledgeGraphEdgeIdentity(edge))}')">${t('dashboard.knowledge_action_cancel')}</button>
                    </div>
                </div>
            `;
        }

        function toggleKnowledgeGraphEdit(nodeID) {
            KnowledgeGraphState.editingNodeId = KnowledgeGraphState.editingNodeId === nodeID ? '' : nodeID;
            if (KnowledgeGraphState.editingNodeId) {
                KnowledgeGraphState.editingEdgeKey = '';
            }
            if (KnowledgeGraphState.focusNodeId) {
                loadKnowledgeGraphNodeDetail(KnowledgeGraphState.focusNodeId);
            }
        }

        function toggleKnowledgeGraphEdgeEdit(edgeKey) {
            KnowledgeGraphState.editingEdgeKey = KnowledgeGraphState.editingEdgeKey === edgeKey ? '' : edgeKey;
            if (KnowledgeGraphState.editingEdgeKey) {
                KnowledgeGraphState.editingNodeId = '';
            }
            if (KnowledgeGraphState.focusNodeId) {
                loadKnowledgeGraphNodeDetail(KnowledgeGraphState.focusNodeId);
            }
        }

        async function saveKnowledgeGraphNodeEdit(nodeID) {
            const labelInput = document.getElementById('knowledge-edit-label');
            const propsInput = document.getElementById('knowledge-edit-properties');
            if (!labelInput || !propsInput || !nodeID) return;

            let properties = {};
            const raw = propsInput.value.trim();
            if (raw) {
                try {
                    properties = JSON.parse(raw);
                } catch (err) {
                    if (typeof showAlert === 'function') {
                        await showAlert(t('dashboard.knowledge_editor_invalid_title'), t('dashboard.knowledge_editor_invalid'));
                    }
                    return;
                }
                if (!properties || Array.isArray(properties) || typeof properties !== 'object') {
                    if (typeof showAlert === 'function') {
                        await showAlert(t('dashboard.knowledge_editor_invalid_title'), t('dashboard.knowledge_editor_invalid'));
                    }
                    return;
                }
                properties = Object.entries(properties).reduce((acc, [key, value]) => {
                    if (!key || value === null || value === undefined) return acc;
                    acc[String(key)] = String(value);
                    return acc;
                }, {});
            }

            const response = await fetch('/api/knowledge-graph/node', {
                method: 'PUT',
                credentials: 'same-origin',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    id: nodeID,
                    label: labelInput.value.trim(),
                    properties,
                }),
            });
            const payload = await safeReadJSON(response);
            if (!response.ok) {
                if (typeof showToast === 'function') showToast(payload?.error || t('dashboard.knowledge_mutation_failed'), 'error', 5000);
                return;
            }

            KnowledgeGraphState.editingNodeId = '';
            if (typeof showToast === 'function') showToast(t('dashboard.knowledge_saved'), 'success', 2500);
            await loadTabKnowledge();
        }

        async function saveKnowledgeGraphEdgeEdit(source, target, relation) {
            const relationInput = document.getElementById('knowledge-edit-edge-relation');
            const propsInput = document.getElementById('knowledge-edit-edge-properties');
            if (!relationInput || !propsInput) return;

            let properties = {};
            const raw = propsInput.value.trim();
            if (raw) {
                try {
                    properties = JSON.parse(raw);
                } catch (err) {
                    if (typeof showAlert === 'function') {
                        await showAlert(t('dashboard.knowledge_editor_invalid_title'), t('dashboard.knowledge_editor_invalid'));
                    }
                    return;
                }
                if (!properties || Array.isArray(properties) || typeof properties !== 'object') {
                    if (typeof showAlert === 'function') {
                        await showAlert(t('dashboard.knowledge_editor_invalid_title'), t('dashboard.knowledge_editor_invalid'));
                    }
                    return;
                }
                properties = Object.entries(properties).reduce((acc, [key, value]) => {
                    if (!key || value === null || value === undefined) return acc;
                    acc[String(key)] = String(value);
                    return acc;
                }, {});
            }

            const response = await fetch('/api/knowledge-graph/edge', {
                method: 'PUT',
                credentials: 'same-origin',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    source,
                    target,
                    relation,
                    new_relation: relationInput.value.trim(),
                    properties,
                }),
            });
            const payload = await safeReadJSON(response);
            if (!response.ok) {
                if (typeof showToast === 'function') showToast(payload?.error || t('dashboard.knowledge_edge_mutation_failed'), 'error', 5000);
                return;
            }

            KnowledgeGraphState.editingEdgeKey = '';
            if (typeof showToast === 'function') showToast(t('dashboard.knowledge_edge_saved'), 'success', 2500);
            await loadTabKnowledge();
        }

        async function mergeKnowledgeGraphNodes(targetID, sourceID, label) {
            targetID = String(targetID || '').trim();
            sourceID = String(sourceID || '').trim();
            if (!targetID || !sourceID || targetID === sourceID) return;

            const confirmed = typeof showConfirm === 'function'
                ? await showConfirm(
                    t('dashboard.knowledge_quality_merge_confirm_title'),
                    t('dashboard.knowledge_quality_merge_confirm', { source: sourceID, target: targetID, label: label || sourceID })
                )
                : true;
            if (!confirmed) return;

            const response = await fetch('/api/knowledge-graph/merge', {
                method: 'POST',
                credentials: 'same-origin',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ target_id: targetID, source_id: sourceID }),
            });
            const payload = await safeReadJSON(response);
            if (!response.ok) {
                if (typeof showToast === 'function') {
                    showToast(payload?.error || t('dashboard.knowledge_quality_merge_error'), 'error', 5000);
                }
                return;
            }
            if (typeof showToast === 'function') {
                showToast(t('dashboard.knowledge_quality_merge_success'), 'success', 2500);
            }
            await loadTabKnowledge();
        }

        async function toggleKnowledgeGraphProtection(nodeID, shouldProtect) {
            const response = await fetch('/api/knowledge-graph/node/protect', {
                method: 'POST',
                credentials: 'same-origin',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ id: nodeID, protected: !!shouldProtect }),
            });
            const payload = await safeReadJSON(response);
            if (!response.ok) {
                if (typeof showToast === 'function') showToast(payload?.error || t('dashboard.knowledge_mutation_failed'), 'error', 5000);
                return;
            }

            if (typeof showToast === 'function') showToast(shouldProtect ? t('dashboard.knowledge_protected') : t('dashboard.knowledge_unprotected'), 'success', 2500);
            await loadTabKnowledge();
        }

        async function deleteKnowledgeGraphEdge(source, target, relation) {
            const confirmed = typeof showConfirm === 'function'
                ? await showConfirm(t('dashboard.knowledge_edge_delete_title'), t('dashboard.knowledge_edge_delete_confirm', { relation }))
                : true;
            if (!confirmed) return;

            const url = '/api/knowledge-graph/edge?source=' + encodeURIComponent(source) + '&target=' + encodeURIComponent(target) + '&relation=' + encodeURIComponent(relation);
            const response = await fetch(url, {
                method: 'DELETE',
                credentials: 'same-origin',
            });
            const payload = await safeReadJSON(response);
            if (!response.ok) {
                if (typeof showToast === 'function') showToast(payload?.error || t('dashboard.knowledge_edge_delete_failed'), 'error', 5000);
                return;
            }

            KnowledgeGraphState.editingEdgeKey = '';
            if (typeof showToast === 'function') showToast(t('dashboard.knowledge_edge_deleted'), 'success', 2500);
            await loadTabKnowledge();
        }

        async function deleteKnowledgeGraphNode(nodeID, label) {
            const confirmed = typeof showConfirm === 'function'
                ? await showConfirm(t('dashboard.knowledge_delete_title'), t('dashboard.knowledge_delete_confirm', { label: label || nodeID }))
                : true;
            if (!confirmed) return;

            const response = await fetch('/api/knowledge-graph/node?id=' + encodeURIComponent(nodeID), {
                method: 'DELETE',
                credentials: 'same-origin',
            });
            const payload = await safeReadJSON(response);
            if (!response.ok) {
                if (typeof showToast === 'function') showToast(payload?.error || t('dashboard.knowledge_delete_failed'), 'error', 5000);
                return;
            }

            KnowledgeGraphState.editingNodeId = '';
            KnowledgeGraphState.focusNodeId = '';
            KnowledgeGraphState.focusPayload = null;
            if (typeof showToast === 'function') showToast(t('dashboard.knowledge_deleted'), 'success', 2500);
            closeKGDetailModal();
            await loadTabKnowledge();
        }

        async function safeReadJSON(response) {
            try {
                return await response.json();
            } catch (_) {
                return null;
            }
        }

        function knowledgeGraphEdgeIdentity(edge) {
            return `${edge?.source || ''}::${edge?.target || ''}::${edge?.relation || ''}`;
        }



        function buildKnowledgeGraphOverviewModel(nodes, edges) {
            const safeNodes = dedupeKnowledgeGraphNodes(nodes || []);
            const safeEdges = Array.isArray(edges) ? edges : [];
            if (!safeNodes.length) return null;

            const degree = new Map();
            safeEdges.forEach(edge => {
                if (edge?.source) degree.set(edge.source, (degree.get(edge.source) || 0) + 1);
                if (edge?.target) degree.set(edge.target, (degree.get(edge.target) || 0) + 1);
            });

            const MAX_PER_TYPE = 4;
            const MAX_NODES = 15;
            const typeCount = new Map();
            const selectedNodes = [];

            const sortedNodes = [...safeNodes].sort((a, b) => {
                const scoreA = a.importance_score ?? 0;
                const scoreB = b.importance_score ?? 0;
                if (scoreB !== scoreA) return scoreB - scoreA;
                return String(a.label || a.id || '').localeCompare(String(b.label || b.id || ''));
            });

            for (const node of sortedNodes) {
                if (selectedNodes.length >= MAX_NODES) break;
                const type = node?.properties?.type || '_untyped';
                const currentTypeCount = typeCount.get(type) || 0;
                if (currentTypeCount >= MAX_PER_TYPE) continue;
                typeCount.set(type, currentTypeCount + 1);
                selectedNodes.push(node);
            }

            const selectedIDs = new Set(selectedNodes.map(node => node.id));
            const selectedEdges = safeEdges
                .filter(edge => selectedIDs.has(edge?.source) && selectedIDs.has(edge?.target))
                .slice(0, 25);

            const maxScore = Math.max(1, ...selectedNodes.map(n => n.importance_score ?? 0));

            return {
                nodes: selectedNodes.map(n => {
                    const score = n.importance_score ?? 0;
                    const radius = 10 + (score / maxScore) * 15;
                    return {...n, isFocus: false, r: radius};
                }),
                edges: selectedEdges,
                width: 720,
                height: 360,
                focusNode: null,
            };
        }

        function buildKnowledgeGraphFocusedModel(payload) {
            const node = payload?.node;
            if (!node) return null;

            const neighbors = dedupeKnowledgeGraphNodes((payload?.neighbors || []).slice(0, 10));
            const nodes = [node, ...neighbors];
            const nodeIDs = new Set(nodes.map(item => item.id));
            const edges = (payload?.edges || [])
                .filter(edge => nodeIDs.has(edge?.source) && nodeIDs.has(edge?.target))
                .slice(0, 20);

            return {
                nodes: [
                    {
                        ...node,
                        r: 20,
                        isFocus: true,
                    },
                    ...neighbors.map(n => ({...n, isFocus: false, r: 15}))
                ],
                edges,
                width: 720,
                height: 360,
                focusNode: node,
            };
        }

        const KG_VISUAL_MIN_WIDTH = 320;
        const KG_VISUAL_MAX_WIDTH = 1600;
        const KG_VISUAL_MIN_HEIGHT = 420;
        const KG_VISUAL_MAX_HEIGHT = 560;
        const KG_VIEW_STORAGE_KEY = 'aurago.dashboard.kgview.v1';

        function knowledgeGraphVisualSize(wrap) {
            const rect = wrap.getBoundingClientRect ? wrap.getBoundingClientRect() : { width: 0 };
            const style = window.getComputedStyle ? window.getComputedStyle(wrap) : null;
            const cssHeight = style ? parseFloat(style.height) : 0;
            const width = Math.floor(rect.width || wrap.clientWidth || 720);
            const height = Math.floor(cssHeight || rect.height || KG_VISUAL_MIN_HEIGHT);
            return {
                width: Math.min(KG_VISUAL_MAX_WIDTH, Math.max(KG_VISUAL_MIN_WIDTH, width)),
                height: Math.min(KG_VISUAL_MAX_HEIGHT, Math.max(KG_VISUAL_MIN_HEIGHT, height)),
            };
        }

        function kgPrefersReducedMotion() {
            try {
                return typeof window.matchMedia === 'function' && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
            } catch (_) {
                return false;
            }
        }

        let _kgWebglSupport = null;
        function kgWebGLSupported() {
            if (_kgWebglSupport !== null) return _kgWebglSupport;
            let ok = false;
            try {
                const canvas = document.createElement('canvas');
                ok = !!(window.WebGLRenderingContext && (canvas.getContext('webgl2') || canvas.getContext('webgl') || canvas.getContext('experimental-webgl')));
            } catch (_) {
                ok = false;
            }
            _kgWebglSupport = ok;
            return ok;
        }

        function kgStoredViewMode() {
            try {
                const mode = localStorage.getItem(KG_VIEW_STORAGE_KEY);
                if (mode === '2d' || mode === '3d') return mode;
            } catch (_) {}
            return '3d';
        }

        function kgEffectiveViewMode() {
            if (kgStoredViewMode() === '2d') return '2d';
            if (kgWebGLSupported() && typeof ForceGraph3D === 'function') return '3d';
            return '2d';
        }

        function setKnowledgeGraphViewMode(mode) {
            try { localStorage.setItem(KG_VIEW_STORAGE_KEY, mode === '2d' ? '2d' : '3d'); } catch (_) {}
            renderKnowledgeGraphVisual();
        }

        function kgUpdateViewToggle() {
            const toggle = document.getElementById('knowledge-graph-view-toggle');
            if (!toggle) return;
            const effective = kgEffectiveViewMode();
            const can3d = kgWebGLSupported() && typeof ForceGraph3D === 'function';
            toggle.querySelectorAll('[data-kg-view]').forEach(btn => {
                const is3D = btn.dataset.kgView === '3d';
                const active = btn.dataset.kgView === effective;
                btn.classList.toggle('active', active);
                btn.setAttribute('aria-pressed', String(active));
                if (is3D) {
                    btn.disabled = !can3d;
                    btn.title = can3d ? t('dashboard.knowledge_visual_view_3d_title') : t('dashboard.knowledge_visual_3d_unavailable');
                }
            });
        }

        function kgParseColor(color) {
            const fallback = [148, 163, 184];
            if (!color) return fallback;
            const raw = String(color).trim();
            const hex = /^#([0-9a-f]{3}|[0-9a-f]{6})$/i.exec(raw);
            if (hex) {
                let h = hex[1];
                if (h.length === 3) h = h.split('').map(c => c + c).join('');
                const num = parseInt(h, 16);
                return [(num >> 16) & 255, (num >> 8) & 255, num & 255];
            }
            const rgb = /^rgba?\(\s*(\d+)[,\s]+(\d+)[,\s]+(\d+)/i.exec(raw);
            if (rgb) return [Number(rgb[1]), Number(rgb[2]), Number(rgb[3])];
            return fallback;
        }

        function kgAlpha(color, alpha) {
            const [r, g, b] = kgParseColor(color);
            return `rgba(${r},${g},${b},${alpha})`;
        }

        function kgMix(color, other, weight) {
            const a = kgParseColor(color);
            const b = kgParseColor(other);
            const w = Math.min(1, Math.max(0, weight));
            return `rgb(${Math.round(a[0] + (b[0] - a[0]) * w)},${Math.round(a[1] + (b[1] - a[1]) * w)},${Math.round(a[2] + (b[2] - a[2]) * w)})`;
        }

        function kgIsLightTheme() {
            return (document.documentElement.getAttribute('data-theme') || '').toLowerCase() === 'light';
        }

        function kgMulberry32(seed) {
            let a = seed >>> 0;
            return function () {
                a |= 0; a = (a + 0x6D2B79F5) | 0;
                let t = Math.imul(a ^ (a >>> 15), 1 | a);
                t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
                return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
            };
        }

        function kgStarField(count, seed) {
            const rand = kgMulberry32(seed);
            const stars = [];
            for (let i = 0; i < count; i++) {
                stars.push({
                    x: rand(),
                    y: rand(),
                    r: 0.4 + rand() * 1.2,
                    a: 0.25 + rand() * 0.65,
                    p: rand() * Math.PI * 2,
                    s: 0.35 + rand() * 1.1,
                });
            }
            return stars;
        }

        function kgPaintStars2D(wrap, ctx) {
            if (!wrap._kgStars2d) wrap._kgStars2d = kgStarField(150, 0xA67A9);
            const w = ctx.canvas.width;
            const h = ctx.canvas.height;
            if (!w || !h) return;
            const reduced = kgPrefersReducedMotion();
            const light = kgIsLightTheme();
            const now = reduced ? 0 : performance.now() / 1000;
            const dpr = window.devicePixelRatio || 1;
            const base = light ? '100,116,139' : '203,213,225';
            ctx.save();
            ctx.setTransform(1, 0, 0, 1, 0, 0);
            wrap._kgStars2d.forEach(star => {
                const twinkle = reduced ? 1 : (0.55 + 0.45 * Math.sin(now * star.s + star.p));
                ctx.fillStyle = `rgba(${base},${(star.a * twinkle * (light ? 0.35 : 0.55)).toFixed(3)})`;
                ctx.beginPath();
                ctx.arc(star.x * w, star.y * h, star.r * dpr, 0, Math.PI * 2, false);
                ctx.fill();
            });
            ctx.restore();
        }

        function kgPaintVignette2D(ctx) {
            const w = ctx.canvas.width;
            const h = ctx.canvas.height;
            if (!w || !h) return;
            const light = kgIsLightTheme();
            ctx.save();
            ctx.setTransform(1, 0, 0, 1, 0, 0);
            const vignette = ctx.createRadialGradient(w / 2, h / 2, Math.min(w, h) * 0.3, w / 2, h / 2, Math.max(w, h) * 0.75);
            vignette.addColorStop(0, 'rgba(0,0,0,0)');
            vignette.addColorStop(1, light ? 'rgba(100,116,139,0.08)' : 'rgba(2,6,23,0.38)');
            ctx.fillStyle = vignette;
            ctx.fillRect(0, 0, w, h);
            ctx.restore();
        }

        function kgBuildGraphData(model) {
            const topLabels = new Set([...model.nodes]
                .sort((a, b) => (b.importance_score ?? 0) - (a.importance_score ?? 0))
                .slice(0, 8)
                .map(n => n.id));
            const colorById = new Map();
            const nodes = model.nodes.map(n => {
                const score = n.importance_score ?? 0;
                const color = knowledgeGraphNodeColor(n);
                colorById.set(n.id, color);
                return {
                    id: n.id,
                    label: truncate(String(n.label || n.id || 'Node'), 18),
                    meta: buildKnowledgeGraphNodeTooltip(n),
                    type: n?.properties?.type || '',
                    val: (n.r || 15) / 2,
                    color: color,
                    isFocus: n.isFocus || false,
                    importance: score,
                    topLabel: topLabels.has(n.id),
                };
            });
            const links = model.edges.map(e => ({
                source: e.source,
                target: e.target,
                relation: truncate(String(e.relation || ''), 18),
                relationFull: String(e.relation || ''),
            }));
            return { nodes, links, colorById };
        }

        function kgLinkEndpointId(endpoint) {
            return typeof endpoint === 'object' && endpoint ? endpoint.id : endpoint;
        }

        function kgComputeHover(state, graphData, node) {
            state.hover = node || null;
            state.links = new Set();
            state.neighborIds = new Set();
            if (!node) return;
            state.neighborIds.add(node.id);
            (graphData.links || []).forEach(link => {
                const s = kgLinkEndpointId(link.source);
                const tgt = kgLinkEndpointId(link.target);
                if (s === node.id || tgt === node.id) {
                    state.links.add(link);
                    state.neighborIds.add(s);
                    state.neighborIds.add(tgt);
                }
            });
        }

        function renderKnowledgeGraphVisual2D(wrap, model, graphSize) {
            const graph = wrap._forceGraph;
            const reduced = kgPrefersReducedMotion();
            const light = kgIsLightTheme();
            const labelColor = cv('--text-secondary') || '#94a3b8';
            const labelStrongColor = cv('--text-primary') || '#f8fafc';
            const haloColor = light ? 'rgba(248,250,252,0.92)' : 'rgba(2,6,23,0.88)';
            const edgeBase = cv('--border-subtle') || '#334155';

            const state = { hover: null, links: new Set(), neighborIds: new Set() };
            const data = kgBuildGraphData(model);
            const linkSourceColor = link => data.colorById.get(kgLinkEndpointId(link.source)) || edgeBase;

            graph
                .width(graphSize.width)
                .height(graphSize.height)
                .backgroundColor('transparent')
                .graphData({ nodes: data.nodes, links: data.links })
                .nodeId('id')
                .nodeVal('val')
                .nodeLabel('meta')
                .linkLabel('relationFull')
                .linkCurvature(0.14)
                .linkDirectionalArrowLength(4)
                .linkDirectionalArrowRelPos(1)
                .linkDirectionalArrowColor(link => kgAlpha(linkSourceColor(link), 0.85))
                .linkWidth(link => state.hover ? (state.links.has(link) ? 1.7 : 0.35) : 0.7)
                .linkColor(link => {
                    if (state.hover) {
                        return state.links.has(link) ? kgAlpha(linkSourceColor(link), 0.9) : kgAlpha(edgeBase, 0.1);
                    }
                    return kgAlpha(edgeBase, light ? 0.55 : 0.65);
                })
                .linkDirectionalParticles(reduced ? 0 : 2)
                .linkDirectionalParticleWidth(link => state.links.has(link) ? 2.6 : 1.7)
                .linkDirectionalParticleSpeed(0.0045)
                .linkDirectionalParticleColor(link => kgAlpha(linkSourceColor(link), 0.95))
                .nodePointerAreaPaint((node, color, ctx) => {
                    const radius = (node.isFocus ? node.val * 1.45 : node.val) + 4;
                    ctx.fillStyle = color;
                    ctx.beginPath();
                    ctx.arc(node.x, node.y, radius, 0, 2 * Math.PI, false);
                    ctx.fill();
                })
                .nodeCanvasObject((node, ctx, globalScale) => {
                    const isHover = state.hover === node;
                    const inNeighborhood = state.hover && state.neighborIds.has(node.id);
                    const dimmed = state.hover && !isHover && !inNeighborhood;
                    const radius = node.isFocus ? node.val * 1.45 : node.val;

                    ctx.save();
                    if (dimmed) ctx.globalAlpha = 0.14;

                    // halo glow
                    const glowRadius = radius * (node.isFocus ? 2.7 : 2.15);
                    const glow = ctx.createRadialGradient(node.x, node.y, radius * 0.5, node.x, node.y, glowRadius);
                    glow.addColorStop(0, kgAlpha(node.color, isHover ? 0.6 : (node.isFocus ? 0.5 : 0.38)));
                    glow.addColorStop(1, kgAlpha(node.color, 0));
                    ctx.fillStyle = glow;
                    ctx.beginPath();
                    ctx.arc(node.x, node.y, glowRadius, 0, 2 * Math.PI, false);
                    ctx.fill();

                    // focus pulse ring (repaint is driven by the directional particles)
                    if (node.isFocus && !reduced) {
                        const phase = ((performance.now() / 1000) % 1.6) / 1.6;
                        const pulseRadius = radius * (1.25 + phase * 1.35);
                        ctx.strokeStyle = kgAlpha(node.color, (1 - phase) * 0.75);
                        ctx.lineWidth = (1.4 + (1 - phase) * 1.2) / globalScale;
                        ctx.beginPath();
                        ctx.arc(node.x, node.y, pulseRadius, 0, 2 * Math.PI, false);
                        ctx.stroke();
                    }

                    // core sphere with a fake top-left light
                    const core = ctx.createRadialGradient(
                        node.x - radius * 0.38, node.y - radius * 0.42, radius * 0.12,
                        node.x, node.y, radius
                    );
                    core.addColorStop(0, kgMix(node.color, '#ffffff', light ? 0.55 : 0.72));
                    core.addColorStop(0.5, node.color);
                    core.addColorStop(1, kgMix(node.color, '#020617', light ? 0.25 : 0.42));
                    ctx.fillStyle = core;
                    ctx.beginPath();
                    ctx.arc(node.x, node.y, radius, 0, 2 * Math.PI, false);
                    ctx.fill();

                    // rim light
                    ctx.strokeStyle = isHover ? kgAlpha('#ffffff', 0.95) : kgAlpha(kgMix(node.color, '#ffffff', 0.45), node.isFocus ? 0.9 : 0.5);
                    ctx.lineWidth = (isHover ? 1.7 : node.isFocus ? 1.4 : 0.9) / globalScale + 0.3;
                    ctx.stroke();

                    // label only where it adds value: focus, hover neighborhood, top nodes, deep zoom
                    const showLabel = !dimmed && (node.isFocus || isHover || inNeighborhood || node.topLabel || globalScale >= 1.3);
                    if (showLabel) {
                        const fontSize = Math.max(12 / globalScale, 4.2);
                        ctx.font = `600 ${fontSize}px 'Geist', 'Segoe UI', sans-serif`;
                        ctx.textAlign = 'center';
                        ctx.textBaseline = 'middle';
                        const textY = node.y + radius + 2.5 + fontSize / 2;
                        ctx.lineWidth = fontSize / 4.5;
                        ctx.strokeStyle = haloColor;
                        ctx.strokeText(node.label, node.x, textY);
                        ctx.fillStyle = (node.isFocus || isHover) ? labelStrongColor : labelColor;
                        ctx.fillText(node.label, node.x, textY);
                    }
                    ctx.restore();
                })
                .onRenderFramePre((ctx) => kgPaintStars2D(wrap, ctx))
                .onRenderFramePost((ctx) => kgPaintVignette2D(ctx))
                .onNodeHover(node => {
                    kgComputeHover(state, graph.graphData(), node);
                    wrap.style.cursor = node ? 'pointer' : '';
                })
                .onNodeClick(node => {
                    if (!node) return;
                    KnowledgeGraphState.focusNodeId = node.id;
                    loadKnowledgeGraphNodeDetail(node.id);
                })
                .warmupTicks(reduced ? 0 : 40)
                .cooldownTime(reduced ? 600 : 6500);

            setTimeout(() => {
                if (wrap._forceGraph && typeof wrap._forceGraph.zoomToFit === 'function') {
                    wrap._forceGraph.zoomToFit(reduced ? 0 : 650, 46);
                }
            }, 320);
        }

        let _kgGlowTexture = null;
        function kgGlowTexture() {
            if (_kgGlowTexture) return _kgGlowTexture;
            const size = 128;
            const canvas = document.createElement('canvas');
            canvas.width = size;
            canvas.height = size;
            const g = canvas.getContext('2d');
            const grad = g.createRadialGradient(size / 2, size / 2, 0, size / 2, size / 2, size / 2);
            grad.addColorStop(0, 'rgba(255,255,255,0.85)');
            grad.addColorStop(0.25, 'rgba(255,255,255,0.32)');
            grad.addColorStop(0.6, 'rgba(255,255,255,0.08)');
            grad.addColorStop(1, 'rgba(255,255,255,0)');
            g.fillStyle = grad;
            g.fillRect(0, 0, size, size);
            _kgGlowTexture = new THREE.CanvasTexture(canvas);
            return _kgGlowTexture;
        }

        function kgTextSprite3D(text, color) {
            const fontSize = 42;
            const padding = 18;
            const font = `600 ${fontSize}px 'Geist', 'Segoe UI', sans-serif`;
            const measure = document.createElement('canvas').getContext('2d');
            measure.font = font;
            const width = Math.ceil(measure.measureText(text).width) + padding * 2;
            const height = fontSize + padding * 2;
            const canvas = document.createElement('canvas');
            canvas.width = width;
            canvas.height = height;
            const g = canvas.getContext('2d');
            g.font = font;
            g.textAlign = 'center';
            g.textBaseline = 'middle';
            g.lineWidth = 8;
            g.strokeStyle = kgIsLightTheme() ? 'rgba(248,250,252,0.95)' : 'rgba(2,6,23,0.9)';
            g.strokeText(text, width / 2, height / 2);
            g.fillStyle = color;
            g.fillText(text, width / 2, height / 2);
            const texture = new THREE.CanvasTexture(canvas);
            texture.minFilter = THREE.LinearFilter;
            texture.generateMipmaps = false;
            const material = new THREE.SpriteMaterial({ map: texture, transparent: true, depthWrite: false });
            const sprite = new THREE.Sprite(material);
            const scale = 0.14;
            sprite.scale.set(width * scale, height * scale, 1);
            return sprite;
        }

        function kgStarPoints3D() {
            const rand = kgMulberry32(0x51F3D);
            const count = 320;
            const positions = new Float32Array(count * 3);
            for (let i = 0; i < count; i++) {
                const radius = 260 + rand() * 520;
                const theta = rand() * Math.PI * 2;
                const phi = Math.acos(2 * rand() - 1);
                positions[i * 3] = radius * Math.sin(phi) * Math.cos(theta);
                positions[i * 3 + 1] = radius * Math.sin(phi) * Math.sin(theta);
                positions[i * 3 + 2] = radius * Math.cos(phi);
            }
            const geometry = new THREE.BufferGeometry();
            geometry.setAttribute('position', new THREE.BufferAttribute(positions, 3));
            const material = new THREE.PointsMaterial({
                color: kgIsLightTheme() ? 0x94a3b8 : 0xcbd5e1,
                size: 1.7,
                sizeAttenuation: true,
                transparent: true,
                opacity: kgIsLightTheme() ? 0.35 : 0.55,
                depthWrite: false,
            });
            return new THREE.Points(geometry, material);
        }

        function kgApply3DNodeDim(graph, state) {
            const nodes = (graph.graphData() && graph.graphData().nodes) || [];
            nodes.forEach(n => {
                const dim = !!(state.hover && !state.neighborIds.has(n.id));
                if (n.__kgSphere) n.__kgSphere.material.opacity = dim ? 0.12 : n.__kgBase.sphereOpacity;
                if (n.__kgGlow) n.__kgGlow.material.opacity = dim ? 0.04 : n.__kgBase.glowOpacity;
                if (n.__kgLabel) n.__kgLabel.material.opacity = dim ? 0.05 : 1;
            });
        }

        function renderKnowledgeGraphVisual3D(wrap, model, graphSize) {
            const graph = wrap._forceGraph3d;
            const reduced = kgPrefersReducedMotion();
            const light = kgIsLightTheme();
            const edgeBase = cv('--border-subtle') || '#334155';
            const labelColor = cv('--text-primary') || '#f8fafc';
            const glowTexture = kgGlowTexture();

            const state = { hover: null, links: new Set(), neighborIds: new Set() };
            const data = kgBuildGraphData(model);
            const linkSourceColor = link => data.colorById.get(kgLinkEndpointId(link.source)) || edgeBase;

            graph
                .width(graphSize.width)
                .height(graphSize.height)
                .backgroundColor('rgba(0,0,0,0)')
                .showNavInfo(false)
                .graphData({ nodes: data.nodes, links: data.links })
                .nodeId('id')
                .nodeLabel('meta')
                .linkLabel('relationFull')
                .nodeThreeObject(node => {
                    const group = new THREE.Group();
                    const radius = (node.isFocus ? node.val * 1.6 : node.val) * 1.45;
                    const sphere = new THREE.Mesh(
                        new THREE.SphereGeometry(radius, 24, 18),
                        new THREE.MeshLambertMaterial({
                            color: kgMix(node.color, '#ffffff', 0.12),
                            emissive: node.color,
                            emissiveIntensity: 0.55,
                            transparent: true,
                            opacity: 0.98,
                        })
                    );
                    group.add(sphere);

                    const glowMaterial = new THREE.SpriteMaterial({
                        map: glowTexture,
                        color: node.color,
                        transparent: true,
                        opacity: node.isFocus ? 0.95 : 0.75,
                        blending: THREE.AdditiveBlending,
                        depthWrite: false,
                    });
                    const glow = new THREE.Sprite(glowMaterial);
                    const glowScale = radius * (node.isFocus ? 6.8 : 5.2);
                    glow.scale.set(glowScale, glowScale, 1);
                    group.add(glow);

                    node.__kgSphere = sphere;
                    node.__kgGlow = glow;
                    node.__kgGlowScale = glowScale;
                    node.__kgBase = { sphereOpacity: 0.98, glowOpacity: glowMaterial.opacity };

                    if (node.isFocus || node.topLabel) {
                        const label = kgTextSprite3D(node.label, labelColor);
                        label.position.set(0, radius + label.scale.y * 0.5 + 1.2, 0);
                        group.add(label);
                        node.__kgLabel = label;
                    }
                    return group;
                })
                .linkOpacity(light ? 0.3 : 0.42)
                .linkWidth(link => state.links.has(link) ? 1.5 : 0.55)
                .linkColor(link => state.hover
                    ? (state.links.has(link) ? kgAlpha(linkSourceColor(link), 0.95) : kgAlpha(edgeBase, 0.06))
                    : kgAlpha(edgeBase, light ? 0.5 : 0.75))
                .linkCurvature(0.1)
                .linkDirectionalArrowLength(3.4)
                .linkDirectionalArrowRelPos(1)
                .linkDirectionalParticles(reduced ? 0 : 2)
                .linkDirectionalParticleWidth(link => state.links.has(link) ? 2.4 : 1.5)
                .linkDirectionalParticleSpeed(0.0055)
                .linkDirectionalParticleColor(link => kgAlpha(linkSourceColor(link), 0.95))
                .onNodeHover(node => {
                    kgComputeHover(state, graph.graphData(), node);
                    kgApply3DNodeDim(graph, state);
                    // Re-assigning the accessors triggers the library's own restyle pass
                    graph
                        .linkColor(graph.linkColor())
                        .linkWidth(graph.linkWidth())
                        .linkDirectionalParticleWidth(graph.linkDirectionalParticleWidth());
                    wrap.style.cursor = node ? 'pointer' : '';
                })
                .onNodeClick(node => {
                    if (!node) return;
                    KnowledgeGraphState.focusNodeId = node.id;
                    loadKnowledgeGraphNodeDetail(node.id);
                })
                .cooldownTime(reduced ? 800 : 9000)
                .warmupTicks(80);

            // spread the constellation so glow halos and labels stay readable
            try {
                const charge = typeof graph.d3Force === 'function' ? graph.d3Force('charge') : null;
                if (charge && typeof charge.strength === 'function') charge.strength(-150);
            } catch (_) {}

            // focus glow pulse, driven by the still-warm force engine
            if (!reduced) {
                graph.onEngineTick(() => {
                    const t = performance.now() / 1000;
                    const nodes = (graph.graphData() && graph.graphData().nodes) || [];
                    nodes.forEach(n => {
                        if (n.isFocus && n.__kgGlow && n.__kgGlowScale) {
                            const s = n.__kgGlowScale * (1 + 0.13 * Math.sin(t * 2.6));
                            n.__kgGlow.scale.set(s, s, 1);
                        }
                    });
                });
            }

            // starfield backdrop, one per 3D instance
            if (!wrap._kgStars3d && typeof graph.scene === 'function') {
                try {
                    const stars = kgStarPoints3D();
                    graph.scene().add(stars);
                    wrap._kgStars3d = stars;
                } catch (_) {}
            }

            // gentle auto-rotation with interaction pause
            const controls = typeof graph.controls === 'function' ? graph.controls() : null;
            if (controls && !wrap._kgRotateWired) {
                wrap._kgRotateWired = true;
                let resumeTimer = null;
                if (!reduced) {
                    controls.autoRotate = true;
                    controls.autoRotateSpeed = 0.75;
                }
                controls.addEventListener('start', () => {
                    controls.autoRotate = false;
                    if (resumeTimer) { clearTimeout(resumeTimer); resumeTimer = null; }
                });
                controls.addEventListener('end', () => {
                    if (resumeTimer) clearTimeout(resumeTimer);
                    resumeTimer = setTimeout(() => {
                        if (!kgPrefersReducedMotion()) controls.autoRotate = true;
                    }, 7000);
                });
            }

            // cinematic intro flight only once per instance, then settle into the frame
            if (!reduced && !wrap._kg3dIntroDone && typeof graph.cameraPosition === 'function') {
                try { graph.cameraPosition({ x: 0, y: -60, z: 560 }, { x: 0, y: 0, z: 0 }, 0); } catch (_) {}
            }
            setTimeout(() => {
                if (!wrap._forceGraph3d || typeof wrap._forceGraph3d.cameraPosition !== 'function') return;
                try {
                    const first = !wrap._kg3dIntroDone;
                    kg3DFitCamera(wrap._forceGraph3d, first ? (reduced ? 0 : 1500) : 600);
                    wrap._kg3dIntroDone = true;
                } catch (_) {}
            }, wrap._kg3dIntroDone ? 260 : 420);
        }

        // kg3DFitCamera frames the node cloud tighter than the library's sphere-based
        // zoomToFit: it uses the axis spans plus a slightly angled approach so the
        // constellation fills the stage and reads as 3D immediately.
        function kg3DFitCamera(graph, duration) {
            const nodes = (graph.graphData() && graph.graphData().nodes) || [];
            if (!nodes.length) return;
            let minX = Infinity, maxX = -Infinity, minY = Infinity, maxY = -Infinity, minZ = Infinity, maxZ = -Infinity;
            nodes.forEach(n => {
                if (typeof n.x !== 'number' || typeof n.y !== 'number' || typeof n.z !== 'number') return;
                minX = Math.min(minX, n.x); maxX = Math.max(maxX, n.x);
                minY = Math.min(minY, n.y); maxY = Math.max(maxY, n.y);
                minZ = Math.min(minZ, n.z); maxZ = Math.max(maxZ, n.z);
            });
            if (!isFinite(minX)) return;
            const cx = (minX + maxX) / 2;
            const cy = (minY + maxY) / 2;
            const cz = (minZ + maxZ) / 2;
            const camera = typeof graph.camera === 'function' ? graph.camera() : null;
            const fov = (camera && camera.fov ? camera.fov : 50) * Math.PI / 180;
            const aspect = (graph.width() || 1) / Math.max(1, graph.height() || 1);
            const spanX = maxX - minX;
            const spanY = maxY - minY;
            const spanZ = maxZ - minZ;
            const fitSpan = Math.max(spanY, spanX / Math.max(aspect, 0.1)) + spanZ * 0.6;
            const dist = Math.max(140, (fitSpan / 2) / Math.tan(fov / 2) + 36);
            const dir = { x: 0.35, y: -0.28, z: 1 };
            const len = Math.sqrt(dir.x * dir.x + dir.y * dir.y + dir.z * dir.z);
            graph.cameraPosition(
                { x: cx + dir.x / len * dist, y: cy + dir.y / len * dist, z: cz + dir.z / len * dist },
                { x: cx, y: cy, z: cz },
                duration
            );
        }

        function destroyKnowledgeGraphVisual(wrap) {
            if (wrap._kgVisibilityHandler) {
                document.removeEventListener('visibilitychange', wrap._kgVisibilityHandler);
                delete wrap._kgVisibilityHandler;
            }
            if (wrap._forceGraph) {
                try { wrap._forceGraph._destructor(); } catch (_) {}
                delete wrap._forceGraph;
            }
            if (wrap._forceGraph3d) {
                try { wrap._forceGraph3d._destructor(); } catch (_) {}
                delete wrap._forceGraph3d;
            }
            if (wrap._forceGraphResizeObserver) {
                wrap._forceGraphResizeObserver.disconnect();
                delete wrap._forceGraphResizeObserver;
            }
            if (wrap._forceGraphResizeFrame) {
                window.cancelAnimationFrame(wrap._forceGraphResizeFrame);
                delete wrap._forceGraphResizeFrame;
            }
            if (wrap._kgStars3d) {
                try {
                    wrap._kgStars3d.geometry.dispose();
                    wrap._kgStars3d.material.dispose();
                } catch (_) {}
                delete wrap._kgStars3d;
            }
            delete wrap._forceGraphSize;
            delete wrap._kgRenderer;
            delete wrap._kgStars2d;
            delete wrap._kg3dIntroDone;
            delete wrap._kgRotateWired;
            wrap.innerHTML = '';
        }

        function ensureKnowledgeGraphVisualResize(wrap) {
            if (wrap._forceGraphResizeObserver || typeof ResizeObserver !== 'function') return;
            // ResizeObserver keeps the canvas dimensions in sync with the container,
            // which matters for the KG visual that lives inside the (initially hidden)
            // knowledge tab.
            const ro = new ResizeObserver(() => {
                if (wrap._forceGraphResizeFrame) return;
                wrap._forceGraphResizeFrame = window.requestAnimationFrame(() => {
                    wrap._forceGraphResizeFrame = 0;
                    const instance = wrap._forceGraph || wrap._forceGraph3d;
                    if (!instance || typeof instance.width !== 'function') return;
                    const size = knowledgeGraphVisualSize(wrap);
                    if (wrap._forceGraphSize && wrap._forceGraphSize.width === size.width && wrap._forceGraphSize.height === size.height) return;
                    wrap._forceGraphSize = size;
                    instance.width(size.width).height(size.height);
                });
            });
            ro.observe(wrap);
            wrap._forceGraphResizeObserver = ro;
        }

        function ensureKnowledgeGraphVisibilityPause(wrap) {
            if (wrap._kgVisibilityHandler) return;
            const handler = () => {
                try {
                    const instance = wrap._forceGraph3d || wrap._forceGraph;
                    if (!instance) return;
                    if (document.hidden && typeof instance.pauseAnimation === 'function') instance.pauseAnimation();
                    else if (!document.hidden && typeof instance.resumeAnimation === 'function') instance.resumeAnimation();
                } catch (_) {}
            };
            document.addEventListener('visibilitychange', handler);
            wrap._kgVisibilityHandler = handler;
        }

        let _kgThemeListenerWired = false;
        function ensureKnowledgeGraphThemeListener() {
            if (_kgThemeListenerWired) return;
            _kgThemeListenerWired = true;
            window.addEventListener('aurago:themechange', () => {
                const wrap = document.getElementById('knowledge-graph-visual');
                if (!wrap) return;
                // Rebuild theme-colored scene decorations, then repaint everything
                if (wrap._kgStars3d) {
                    try {
                        if (wrap._kgStars3d.parent) wrap._kgStars3d.parent.remove(wrap._kgStars3d);
                        wrap._kgStars3d.geometry.dispose();
                        wrap._kgStars3d.material.dispose();
                    } catch (_) {}
                    delete wrap._kgStars3d;
                }
                delete wrap._kgStars2d;
                renderKnowledgeGraphVisual();
            });
        }

        function renderKnowledgeGraphVisual() {
            const wrap = document.getElementById('knowledge-graph-visual');
            const mode = document.getElementById('knowledge-graph-mode');
            const caption = document.getElementById('knowledge-graph-caption');
            const resetButton = document.getElementById('knowledge-graph-reset');
            if (!wrap || !mode || !caption || !resetButton) return;

            ensureKnowledgeGraphThemeListener();
            kgUpdateViewToggle();

            const focusedModel = buildKnowledgeGraphFocusedModel(KnowledgeGraphState.focusPayload);
            const model = focusedModel || buildKnowledgeGraphOverviewModel(KnowledgeGraphState.nodes, KnowledgeGraphState.edges);

            if (!model || !model.nodes.length) {
                mode.textContent = t('dashboard.knowledge_visual_overview');
                caption.textContent = t('dashboard.knowledge_visual_empty');
                resetButton.classList.add('is-hidden');
                destroyKnowledgeGraphVisual(wrap);
                wrap.innerHTML = `<div class="empty-state">${t('dashboard.knowledge_visual_empty')}</div>`;
                return;
            }

            if (focusedModel) {
                const focusLabel = focusedModel.focusNode?.label || focusedModel.focusNode?.id || t('dashboard.knowledge_nodes');
                mode.textContent = t('dashboard.knowledge_visual_focus');
                caption.textContent = t('dashboard.knowledge_visual_focus_caption', { label: focusLabel, neighbors: Math.max(0, focusedModel.nodes.length - 1) });
                resetButton.classList.remove('is-hidden');
            } else {
                mode.textContent = t('dashboard.knowledge_visual_overview');
                caption.textContent = t('dashboard.knowledge_visual_overview_caption', { nodes: model.nodes.length, edges: model.edges.length });
                resetButton.classList.add('is-hidden');
                renderKnowledgeGraphLegend();
            }

            if (kgStoredViewMode() === '3d' && kgEffectiveViewMode() === '2d') {
                caption.textContent += ` · ${t('dashboard.knowledge_visual_3d_unavailable')}`;
            }

            const viewMode = kgEffectiveViewMode();
            if (wrap._kgRenderer && wrap._kgRenderer !== viewMode) {
                destroyKnowledgeGraphVisual(wrap);
            }

            if (viewMode === '3d') {
                if (!wrap._forceGraph3d) {
                    wrap.innerHTML = '';
                    try {
                        wrap._forceGraph3d = ForceGraph3D({ controlType: 'orbit' })(wrap);
                    } catch (err) {
                        delete wrap._forceGraph3d;
                        _kgWebglSupport = false;
                        kgUpdateViewToggle();
                    }
                }
                if (wrap._forceGraph3d) {
                    wrap._kgRenderer = '3d';
                    ensureKnowledgeGraphVisualResize(wrap);
                    ensureKnowledgeGraphVisibilityPause(wrap);
                    const size3d = knowledgeGraphVisualSize(wrap);
                    wrap._forceGraphSize = size3d;
                    renderKnowledgeGraphVisual3D(wrap, model, size3d);
                    return;
                }
            }

            if (!wrap._forceGraph) {
                wrap.innerHTML = '';
                wrap._forceGraph = ForceGraph()(wrap);
            }
            wrap._kgRenderer = '2d';
            ensureKnowledgeGraphVisualResize(wrap);
            ensureKnowledgeGraphVisibilityPause(wrap);
            const graphSize = knowledgeGraphVisualSize(wrap);
            wrap._forceGraphSize = graphSize;
            renderKnowledgeGraphVisual2D(wrap, model, graphSize);
        }

        function dedupeKnowledgeGraphNodes(nodes) {
            const seen = new Set();
            return (Array.isArray(nodes) ? nodes : []).filter(node => {
                const id = node?.id;
                if (!id || seen.has(id)) return false;
                seen.add(id);
                return true;
            });
        }

        function buildKnowledgeGraphNodeTooltip(node) {
            const parts = [];
            const label = node?.label || node?.id || 'Node';
            parts.push(label);
            const type = node?.properties?.type;
            if (type) parts.push(`[${type}]`);
            const score = node?.importance_score;
            if (typeof score === 'number') parts.push(`Score: ${score}`);
            const ip = node?.properties?.ip;
            if (ip) parts.push(`IP: ${ip}`);
            const os = node?.properties?.os;
            if (os) parts.push(`OS: ${os}`);
            return parts.join(' | ');
        }

        const KNOWLEDGE_GRAPH_TYPE_COLORS = {
            'device':     '#3b82f6',
            'service':    '#22c55e',
            'person':     '#f59e0b',
            'container':  '#8b5cf6',
            'software':   '#14b8a6',
            'location':   '#ef4444',
            'concept':    '#ec4899',
            'event':      '#f97316',
            'network':    '#06b6d4',
            'organization': '#6366f1',
            '_default':   '#6b7280',
        };

        function knowledgeGraphTypeColor(type) {
            if (!type) return KNOWLEDGE_GRAPH_TYPE_COLORS._default;
            const lower = String(type).toLowerCase();
            for (const [key, color] of Object.entries(KNOWLEDGE_GRAPH_TYPE_COLORS)) {
                if (key !== '_default' && lower.includes(key)) return color;
            }
            return KNOWLEDGE_GRAPH_TYPE_COLORS._default;
        }

        function knowledgeGraphNodeColor(node) {
            if (node?.isFocus) return cv('--accent') || '#3b82f6';
            const type = node?.properties?.type || '';
            return knowledgeGraphTypeColor(type);
        }

        function renderKnowledgeGraphLegend() {
            const legend = document.getElementById('knowledge-graph-legend');
            if (!legend) return;

            const typesInGraph = new Set();
            const allNodes = KnowledgeGraphState.nodes || [];
            allNodes.forEach(n => {
                if (n?.properties?.type) typesInGraph.add(n.properties.type);
            });

            if (typesInGraph.size === 0) {
                legend.innerHTML = '';
                return;
            }

            const entries = Array.from(typesInGraph).sort().map(type => {
                const color = knowledgeGraphTypeColor(type);
                const count = allNodes.filter(n => n?.properties?.type === type).length;
                return `<span class="kg-legend-entry"><span class="kg-legend-dot" data-dot-color="${esc(color)}"></span>${esc(type)} (${count})</span>`;
            }).join('');

            legend.innerHTML = `<div class="kg-legend-row">${entries}</div>`;
            applyDynamicSurfaceVars(legend);
        }

        function renderKnowledgeGraphFilters() {
            const filterBar = document.getElementById('knowledge-filter-bar');
            if (!filterBar) return;

            const stats = KnowledgeGraphState.stats;
            if (!stats) { filterBar.innerHTML = ''; return; }

            const types = stats.by_type || {};
            const sources = stats.by_source || {};

            const typeOptions = Object.entries(types).sort((a, b) => b[1] - a[1]).map(([type, count]) =>
                `<option value="${escapeHtml(type)}" ${KnowledgeGraphState.filterType === type ? 'selected' : ''}>${escapeHtml(type)} (${count})</option>`
            ).join('');

            const sourceOptions = Object.entries(sources).sort((a, b) => b[1] - a[1]).map(([source, count]) =>
                `<option value="${escapeHtml(source)}" ${KnowledgeGraphState.filterSource === source ? 'selected' : ''}>${escapeHtml(source)} (${count})</option>`
            ).join('');

            filterBar.innerHTML = `
                <label class="kg-filter-toggle">
                    <input type="checkbox" id="kg-show-all" ${KnowledgeGraphState.showAll ? 'checked' : ''} onchange="toggleKnowledgeGraphShowAll()">
                    <span>${t('dashboard.knowledge_filter_show_all')}</span>
                </label>
                <select class="kg-filter-select" id="kg-filter-type" onchange="applyKnowledgeGraphFilters()">
                    <option value="">${t('dashboard.knowledge_filter_all_types')}</option>
                    ${typeOptions}
                </select>
                <select class="kg-filter-select" id="kg-filter-source" onchange="applyKnowledgeGraphFilters()">
                    <option value="">${t('dashboard.knowledge_filter_all_sources')}</option>
                    ${sourceOptions}
                </select>
            `;

            renderKnowledgeGraphLegend();
        }

        function toggleKnowledgeGraphShowAll() {
            const checkbox = document.getElementById('kg-show-all');
            KnowledgeGraphState.showAll = checkbox ? checkbox.checked : false;
            loadTabKnowledge();
        }

        async function applyKnowledgeGraphFilters() {
            const typeSelect = document.getElementById('kg-filter-type');
            const sourceSelect = document.getElementById('kg-filter-source');
            KnowledgeGraphState.filterType = typeSelect ? typeSelect.value : '';
            KnowledgeGraphState.filterSource = sourceSelect ? sourceSelect.value : '';

            let baseNodes = KnowledgeGraphState.showAll
                ? await loadAllNodes()
                : KnowledgeGraphState.importantNodes;

            if (KnowledgeGraphState.filterType) {
                baseNodes = baseNodes.filter(n => n?.properties?.type === KnowledgeGraphState.filterType);
            }
            if (KnowledgeGraphState.filterSource) {
                baseNodes = baseNodes.filter(n => n?.properties?.source === KnowledgeGraphState.filterSource);
            }

            KnowledgeGraphState.nodes = baseNodes;
            KnowledgeGraphState.edges = filterEdgesForDisplay(KnowledgeGraphState.importantEdges, baseNodes);

            renderKnowledgeGraphLists(KnowledgeGraphState.nodes, KnowledgeGraphState.edges);
            renderKnowledgeGraphVisual();
            renderKnowledgeGraphLegend();
        }


        // ══════════════════════════════════════════════════════════════════════════════
