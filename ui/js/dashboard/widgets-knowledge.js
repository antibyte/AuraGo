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
            if (!metrics || !status) return;

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
        }

        function renderKnowledgeGraphDuplicateCandidates(container, candidates, emptyKey) {
            if (!container) return;
            const rows = Array.isArray(candidates) ? candidates : [];
            if (!rows.length) {
                container.innerHTML = `<div class="empty-state">${t(emptyKey)}</div>`;
                return;
            }
            let html = '<table class="kg-table kg-table-compact"><thead><tr>' +
                `<th>${t('dashboard.kg_col_label')}</th>` +
                `<th>${t('dashboard.kg_col_count')}</th>` +
                `<th>${t('dashboard.kg_col_id')}</th>` +
                `<th>${t('dashboard.kg_col_actions')}</th>` +
                '</tr></thead><tbody>';
            rows.forEach(candidate => {
                const rawIDs = Array.isArray(candidate.ids) ? candidate.ids.filter(id => String(id || '').trim()) : [];
                const targetID = rawIDs[0] || '';
                const idLinks = rawIDs.map(id =>
                    `<span class="kg-cell-link" data-kg-open-node="${esc(id)}">${esc(id)}</span>`
                ).join(', ');
                const mergeButtons = rawIDs.slice(1).map(sourceID => `
                    <button type="button" class="btn btn-secondary btn-sm"
                        data-kg-merge-source="${esc(sourceID)}"
                        data-kg-merge-target="${esc(targetID)}"
                        data-kg-merge-label="${esc(candidate.label || candidate.normalized_label || 'Node')}">
                        ${t('dashboard.knowledge_quality_merge_btn')}
                    </button>`).join(' ');
                html += `<tr>
                    <td>${esc(candidate.label || candidate.normalized_label || 'Node')}</td>
                    <td class="text-secondary">${Number(candidate.count || 0)}</td>
                    <td class="text-secondary">${idLinks || '—'}</td>
                    <td class="kg-merge-actions">${mergeButtons || '—'}</td>
                </tr>`;
            });
            html += '</tbody></table>';
            container.innerHTML = html;
        }

        function renderKnowledgeGraphQuality(report) {
            const metrics = document.getElementById('knowledge-quality-metrics');
            const isolated = document.getElementById('knowledge-quality-isolated');
            const untyped = document.getElementById('knowledge-quality-untyped');
            const duplicates = document.getElementById('knowledge-quality-duplicates');
            const idDuplicates = document.getElementById('knowledge-quality-id-duplicates');
            if (!metrics || !isolated || !untyped || !duplicates || !idDuplicates) return;

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
            renderKnowledgeGraphDuplicateCandidates(duplicates, report?.duplicate_candidates, 'dashboard.knowledge_quality_empty_duplicates');
            renderKnowledgeGraphDuplicateCandidates(idDuplicates, report?.id_duplicate_candidates, 'dashboard.knowledge_quality_empty_id_duplicates');
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
        const KG_VISUAL_MIN_HEIGHT = 360;
        const KG_VISUAL_MAX_HEIGHT = 460;

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

        function renderKnowledgeGraphVisual() {
            const wrap = document.getElementById('knowledge-graph-visual');
            const mode = document.getElementById('knowledge-graph-mode');
            const caption = document.getElementById('knowledge-graph-caption');
            const resetButton = document.getElementById('knowledge-graph-reset');
            if (!wrap || !mode || !caption || !resetButton) return;

            const focusedModel = buildKnowledgeGraphFocusedModel(KnowledgeGraphState.focusPayload);
            const model = focusedModel || buildKnowledgeGraphOverviewModel(KnowledgeGraphState.nodes, KnowledgeGraphState.edges);

            if (!model || !model.nodes.length) {
                mode.textContent = t('dashboard.knowledge_visual_overview');
                caption.textContent = t('dashboard.knowledge_visual_empty');
                resetButton.classList.add('is-hidden');
                
                // Clear any existing force graph instance + ResizeObserver to avoid leaks
                if (wrap._forceGraph) {
                    wrap._forceGraph._destructor();
                    delete wrap._forceGraph;
                }
                if (wrap._forceGraphResizeObserver) {
                    wrap._forceGraphResizeObserver.disconnect();
                    delete wrap._forceGraphResizeObserver;
                }
                if (wrap._forceGraphResizeFrame) {
                    window.cancelAnimationFrame(wrap._forceGraphResizeFrame);
                    delete wrap._forceGraphResizeFrame;
                }
                delete wrap._forceGraphSize;
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

            if (!wrap._forceGraph) {
                wrap.innerHTML = '';
                wrap._forceGraph = ForceGraph()(wrap);
                // ResizeObserver keeps the canvas dimensions in sync with the container,
                // which matters for the KG visual that lives inside the (initially hidden)
                // knowledge tab and is also rendered into the focused-detail modal.
                if (typeof ResizeObserver === 'function') {
                    const ro = new ResizeObserver(() => {
                        if (wrap._forceGraphResizeFrame) return;
                        wrap._forceGraphResizeFrame = window.requestAnimationFrame(() => {
                            wrap._forceGraphResizeFrame = 0;
                            if (!wrap._forceGraph || typeof wrap._forceGraph.width !== 'function') return;
                            const size = knowledgeGraphVisualSize(wrap);
                            if (wrap._forceGraphSize && wrap._forceGraphSize.width === size.width && wrap._forceGraphSize.height === size.height) return;
                            wrap._forceGraphSize = size;
                            wrap._forceGraph.width(size.width).height(size.height);
                        });
                    });
                    ro.observe(wrap);
                    wrap._forceGraphResizeObserver = ro;
                }
            }

            const graphSize = knowledgeGraphVisualSize(wrap);
            wrap._forceGraphSize = graphSize;
            wrap._forceGraph
                .width(graphSize.width)
                .height(graphSize.height)
                .backgroundColor('transparent')
                .graphData({
                    nodes: model.nodes.map(n => {
                        const score = n.importance_score ?? 0;
                        const type = n?.properties?.type || '';
                        return {
                            id: n.id,
                            label: truncate(String(n.label || n.id || 'Node'), 18),
                            meta: buildKnowledgeGraphNodeTooltip(n),
                            type: type,
                            val: (n.r || 15) / 2,
                            color: knowledgeGraphNodeColor(n),
                            isFocus: n.isFocus || false,
                            importance: score
                        };
                    }),
                    links: model.edges.map(e => ({
                        source: e.source,
                        target: e.target,
                        relation: truncate(String(e.relation || ''), 18),
                        relationFull: String(e.relation || '')
                    }))
                })
                .nodeId('id')
                .nodeVal('val')
                .nodeColor('color')
                .nodeLabel('meta')
                .linkDirectionalArrowLength(3.5)
                .linkDirectionalArrowRelPos(1)
                .linkColor(() => cv('--border-subtle') || '#334155')
                .linkLabel('relationFull')
                .nodeCanvasObject((node, ctx, globalScale) => {
                    const fontSize = Math.max(12 / globalScale, 4);
                    let radius = node.val;
                    if (node.isFocus) {
                        radius = node.val * 1.5;
                    }
                    
                    ctx.beginPath();
                    ctx.arc(node.x, node.y, radius, 0, 2 * Math.PI, false);
                    ctx.fillStyle = node.color;
                    ctx.fill();
                    
                    if (node.isFocus) {
                        ctx.lineWidth = 2 / globalScale;
                        ctx.strokeStyle = cv('--text-primary') || '#fff';
                        ctx.stroke();
                    }

                    ctx.font = `${fontSize}px Sans-Serif`;
                    ctx.textAlign = 'center';
                    ctx.textBaseline = 'middle';
                    ctx.fillStyle = node.isFocus ? (cv('--text-primary') || '#f8fafc') : (cv('--text-secondary') || '#94a3b8');

                    const textY = node.y + radius + 4 + fontSize/2;
                    ctx.fillText(node.label, node.x, textY);
                })
                .onNodeClick(node => {
                    KnowledgeGraphState.focusNodeId = node.id;
                    loadKnowledgeGraphNodeDetail(node.id);
                });
            
            // Add a small delay then run a quick pulse layout reset if needed
            setTimeout(() => {
                if (wrap._forceGraph && typeof wrap._forceGraph.zoomToFit === 'function') {
                    wrap._forceGraph.zoomToFit(400, 20);
                }
            }, 300);
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
