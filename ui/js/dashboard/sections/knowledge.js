// AuraGo dashboard – section "knowledge": the knowledge graph loader (rendering: widgets-knowledge.js).
// Classic script, loaded with defer in sidebar order after widgets-knowledge.js and before dashboard-events.js.
// Its declarations stay global: inline onclick strings and other dashboard scripts call them by name. The last
// statement registers the section loader with DashSections (dash-sections.js); main.js#loadSectionContent runs it.

        async function loadSectionKnowledge({ force = false } = {}) {
            const kgCards = [
                'card-knowledge-graph-summary',
                'card-knowledge-graph-health',
                'card-knowledge-graph-quality',
                'card-knowledge-graph-results',
                'card-knowledge-graph-visual',
            ];
            kgCards.forEach((id) => CardState.setLoading(id));

            const opts = sectionFetch(force);
            const results = await Promise.all([
                API.getWithStatus('/api/knowledge-graph/important?limit=30&min_score=15', opts),
                API.getWithStatus('/api/knowledge-graph/stats', opts),
                API.getWithStatus('/api/knowledge-graph/quality?limit=6', opts),
                API.getWithStatus('/api/knowledge-graph/health', opts),
            ]);
            const [importantR, statsR, qualityR, healthR] = results;
            const anyOk = results.some((r) => r.ok);
            if (!anyOk) {
                const status = importantR.status || statsR.status || 0;
                kgCards.forEach((id) => CardState.setError(id, loadTabKnowledge, { status }));
                return;
            }

            const firstKnowledgeStatus = (...responses) => {
                const failed = responses.find((r) => !r.ok);
                return failed ? failed.status : 0;
            };
            const markKnowledgeCard = (cardId, ok, status) => {
                ok ? CardState.setLoaded(cardId) : CardState.setError(cardId, loadTabKnowledge, { status });
            };
            markKnowledgeCard('card-knowledge-graph-summary', importantR.ok && statsR.ok, firstKnowledgeStatus(importantR, statsR));
            markKnowledgeCard('card-knowledge-graph-health', healthR.ok, healthR.status);
            markKnowledgeCard('card-knowledge-graph-quality', qualityR.ok, qualityR.status);
            markKnowledgeCard('card-knowledge-graph-results', importantR.ok, importantR.status);
            markKnowledgeCard('card-knowledge-graph-visual', importantR.ok, importantR.status);

            const important = importantR.ok ? importantR.data : null;
            const stats = statsR.ok ? statsR.data : null;
            const quality = qualityR.ok ? qualityR.data : null;
            const health = healthR.ok ? healthR.data : null;

            KnowledgeGraphState.importantNodes = Array.isArray(important) ? important : [];
            const importantIDs = new Set(KnowledgeGraphState.importantNodes.map(n => n.id));

            let allEdges = [];
            if (importantIDs.size > 0) {
                const edgeResp = await API.get('/api/knowledge-graph/edges?limit=200', opts);
                allEdges = Array.isArray(edgeResp) ? edgeResp.filter(e => e.relation !== 'co_mentioned_with') : [];
            }
            KnowledgeGraphState.importantEdges = allEdges;
            KnowledgeGraphState.stats = stats || null;

            const displayNodes = KnowledgeGraphState.showAll
                ? await loadAllNodes(opts)
                : KnowledgeGraphState.importantNodes;

            KnowledgeGraphState.nodes = displayNodes;
            KnowledgeGraphState.edges = filterEdgesForDisplay(allEdges, displayNodes);

            if (importantR.ok && statsR.ok) {
                renderKnowledgeGraphSummary(KnowledgeGraphState.importantNodes, KnowledgeGraphState.importantEdges, KnowledgeGraphState.stats);
            }
            if (healthR.ok) {
                renderKnowledgeGraphHealth(health);
            }
            if (qualityR.ok) {
                renderKnowledgeGraphQuality(quality);
            }
            if (importantR.ok) {
                renderKnowledgeGraphLists(KnowledgeGraphState.nodes, KnowledgeGraphState.edges);
                renderKnowledgeGraphVisual();
                renderKnowledgeGraphFilters();
            }

            if (importantR.ok) {
                if (KnowledgeGraphState.focusNodeId) {
                    await loadKnowledgeGraphNodeDetail(KnowledgeGraphState.focusNodeId);
                } else {
                    renderKnowledgeGraphDetailEmpty();
                }
            }

            const searchInput = document.getElementById('knowledge-search-input');
            const query = searchInput ? searchInput.value.trim() : '';
            if (importantR.ok && query) {
                await executeKnowledgeGraphSearch(query);
            } else if (importantR.ok) {
                renderKnowledgeGraphSearchState('');
            }
        }

        async function loadAllNodes(opts) {
            const nodes = await API.get('/api/knowledge-graph/nodes?limit=200', opts);
            return Array.isArray(nodes) ? nodes : [];
        }

        function filterEdgesForDisplay(edges, nodes) {
            if (!Array.isArray(edges) || !Array.isArray(nodes)) return [];
            const nodeIDs = new Set(nodes.map(n => n.id));
            return edges.filter(e => nodeIDs.has(e.source) && nodeIDs.has(e.target));
        }

        // Fresh reload after knowledge-graph edits: widgets-knowledge.js and the knowledge
        // card retry buttons call this name.
        function loadTabKnowledge() {
            return loadSectionContent('knowledge', { force: true });
        }

        DashSections.register({ id: 'knowledge', load: loadSectionKnowledge });
