(function () {
    'use strict';

    function create({
        ICON_LIST, KINDS, SOURCE_EDIT_FIELDS, clone, iconId, iconLabelKey,
        sourceActions, sourceIdentity, virtualSlice, sourceState, isPlaced, sceneDimensions,
        validCIDR, validHTTPLink, ctx, esc, t, state,
        safeWindowId, root, q, stage, request, messageFor,
        setNotice, statusLabel, icon, nodeById, groupChildren, hostedServices,
        searchableText, displayedNodes, filteredNodes, canWrite, canEditSource, allowedAction,
        actionReason, labelForKind, setBusy, loadSnapshot, selectNode, nextPosition,
        placeByChoice, saveKeyboardMove, persistLayout, updateUndoButton, replaceNode, openEdgeDialog,
        deleteNode, loadSourceEdit, reapplySourceDraft, reapplyNodeDraft, reviewSourceEdit, saveSourceEdit,
        openActionMenu, chooseEdgeEndpoint
    }) {
        const paintToolbar = () => {
            if (state.disposed || !root.isConnected) return;
            q('[data-action="add"]').disabled = !canWrite();
            q('[data-action="add-group"]').disabled = !canWrite();
            q('[data-action="import"]').disabled = !canWrite();
            q('[data-action="add-empty"]').disabled = !canWrite();
            q('[data-action="arrange"]').disabled = !canWrite() || state.busy.has('layout') || !displayedNodes().some(node => !node.placements[state.view]?.pinned);
            q('[data-action="edge-mode"]').disabled = !canWrite() || displayedNodes().length < 2;
            q('[data-action="undo"]').disabled = !canWrite() || !state.layoutUndo || state.busy.has('layout');
            q('[data-action="retry-layout"]').disabled = !canWrite() || !state.pendingLayout.size || state.pendingLayoutView !== state.view || state.busy.has('layout');
            q('[data-action="refresh"]').disabled = state.busy.has('snapshot');
            q('[data-action="refresh"] span:first-child').textContent = state.busy.has('snapshot') ? '◌' : '↻';
        };
        const paintStatus = () => {
            if (state.disposed || !root.isConnected) return;
            const live = q('[data-live]');
            live.classList.toggle('is-error', state.noticeType === 'error');
            live.classList.toggle('is-busy', state.noticeType === 'busy');
            live.querySelector('span').textContent = state.notice || (state.refreshAt ? t('desktop.infra_ready') : t('desktop.infra_loading'));
            q('[data-count]').textContent = String(state.nodes.length);
            const current = state.sources.filter(source => source.state === 'current').length;
            q('[data-source-summary]').textContent = state.sources.length ? t('desktop.infra_sources_count').replace('{current}', current).replace('{total}', state.sources.length) : t('desktop.infra_sources_waiting');
            q('[data-updated]').textContent = state.refreshAt ? t('desktop.infra_updated').replace('{time}', new Date(state.refreshAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })) : '';
            q('[data-scope-count="all"]').textContent = String(state.nodes.length);
            q('[data-scope-count="unplaced"]').textContent = String(state.nodes.filter(node => !isPlaced(node, state.view === 'table' ? 'atlas' : state.view)).length);
            q('[data-scope-count="attention"]').textContent = String(state.nodes.filter(node => ['stale', 'unknown', 'removed'].includes(sourceState(node))).length);
        };
        const paintGroups = () => {
            const groups = state.nodes.filter(node => node.kind === 'group').sort((a, b) => a.name.localeCompare(b.name));
            const groupIDs = new Set(groups.map(group => group.id)), children = new Map();
            for (const group of groups) if (groupIDs.has(group.parent_id)) { const list = children.get(group.parent_id) || []; list.push(group); children.set(group.parent_id, list); }
            const renderGroup = (group, depth = 0) => {
                const count = groupChildren(group.id).length, collapsed = state.collapsed.has(group.id), nested = children.get(group.id) || [];
                const row = `<div class="infra-group-item ${state.selected === group.id ? 'is-selected' : ''}" style="--infra-group-depth:${depth}"><button type="button" class="infra-group-toggle" data-collapse="${esc(group.id)}" aria-label="${esc(collapsed ? t('desktop.infra_expand_group') : t('desktop.infra_collapse_group'))}" aria-expanded="${!collapsed}">${collapsed ? '›' : '⌄'}</button><button type="button" class="infra-group-select" data-select="${esc(group.id)}" aria-pressed="${state.selected === group.id}">${icon('group')}<span>${esc(group.name)}</span><small>${count}</small></button></div>`;
                return row + (!collapsed ? nested.map(child => renderGroup(child, depth + 1)).join('') : '');
            };
            const roots = groups.filter(group => !groupIDs.has(group.parent_id));
            q('[data-groups]').innerHTML = roots.length ? roots.map(group => renderGroup(group)).join('') : `<p class="infra-groups-empty">${esc(t('desktop.infra_no_groups'))}</p>`;
            q('[data-groups]').querySelectorAll('[data-collapse]').forEach(button => button.addEventListener('click', () => {
                const id = button.dataset.collapse;
                if (state.collapsed.has(id)) state.collapsed.delete(id); else state.collapsed.add(id);
                paintGroups(); paintStage(); paintTray();
            }));
            q('[data-groups]').querySelectorAll('[data-select]').forEach(button => button.addEventListener('click', () => selectNode(button.dataset.select)));
        };
        const paintStage = (restoreFocus = false) => {
            const nodes = displayedNodes();
            const stageNodeLayer = q('[data-nodes]');
            const edgeLayer = q('[data-edges]');
            const scene = q('[data-scene]');
            const pending = state.pendingLayoutView === state.view ? state.pendingLayout : new Map();
            const dimensions = sceneDimensions(state.nodes, state.view, pending);
            state.sceneGeometry = dimensions;
            const viewport = q('.infra-stage-scroll');
            const stageWidth = Math.max(Math.round(dimensions.width * state.zoom), viewport.clientWidth), stageHeight = Math.max(Math.round(dimensions.height * state.zoom), viewport.clientHeight);
            stage.style.width = `${stageWidth}px`; stage.style.height = `${stageHeight}px`;
            scene.style.width = `${dimensions.width}px`; scene.style.height = `${dimensions.height}px`;
            scene.style.transform = `scale(${state.zoom})`;
            edgeLayer.setAttribute('viewBox', `0 0 ${dimensions.width} ${dimensions.height}`);
            edgeLayer.setAttribute('width', dimensions.width); edgeLayer.setAttribute('height', dimensions.height);
            q('[data-zoom]').textContent = `${Math.round(state.zoom * 100)}%`;
            const coords = new Map(nodes.map(node => [node.id, pending.get(node.id) || node.placements?.[state.view] || { x: 0, y: 0 }]));
            edgeLayer.innerHTML = state.edges.filter(edge => coords.has(edge.from) && coords.has(edge.to)).map(edge => {
                const a = coords.get(edge.from), b = coords.get(edge.to);
                const x1 = Number(a.x) - dimensions.originX, y1 = Number(a.y) - dimensions.originY, x2 = Number(b.x) - dimensions.originX, y2 = Number(b.y) - dimensions.originY;
                return `<g class="infra-edge infra-edge-${esc(edge.kind)}" data-edge="${esc(edge.id)}"><path d="M${x1} ${y1} L${x2} ${y2}" vector-effect="non-scaling-stroke"/><circle cx="${x1}" cy="${y1}" r="4"/><circle cx="${x2}" cy="${y2}" r="4"/><title>${esc(edge.label || t(`desktop.infra_edge_${edge.kind}`))}</title></g>`;
            }).join('');
            stageNodeLayer.innerHTML = nodes.map(node => {
                const pos = pending.get(node.id) || node.placements?.[state.view] || { x: 0, y: 0 };
                const selected = state.selected === node.id;
                const stateName = sourceState(node);
                const children = node.kind === 'group' ? groupChildren(node.id).length : 0;
                const hint = [node.profile?.vendor, node.profile?.model, node.profile?.ip_address].filter(Boolean).join(' · ');
                return `<button type="button" class="infra-node infra-node-${esc(node.kind)} ${selected ? 'is-selected' : ''} ${state.layoutUndo?.ids?.includes(node.id) ? 'is-layout-dirty' : ''}" data-node="${esc(node.id)}" data-id="${esc(node.id)}" data-state="${esc(stateName)}" aria-pressed="${selected}" aria-label="${esc(`${node.name}, ${labelForKind(node.kind)}, ${statusLabel(stateName)}`)}" style="left:${Number(pos.x) - dimensions.originX}px;top:${Number(pos.y) - dimensions.originY}px;" draggable="false">
                  <span class="infra-node-ring" aria-hidden="true"></span><span class="infra-node-art">${icon(node.icon)}${node.kind === 'group' ? `<b>${children}</b>` : ''}</span><span class="infra-node-copy"><strong>${esc(node.name)}</strong><small>${esc(hint || labelForKind(node.kind))}</small></span><i class="infra-node-status" aria-hidden="true"></i>
                </button>`;
            }).join('');
            q('[data-empty]').hidden = nodes.length > 0;
            q('[data-action="import-empty"]').disabled = !canWrite();
            q('[data-visible-count]').textContent = t('desktop.infra_visible_count').replace('{count}', nodes.length);
            q('[data-stage-title]').textContent = state.view === 'topology' ? t('desktop.infra_topology_title') : t('desktop.infra_atlas_title');
            q('[data-map-help]').textContent = state.edgeMode ? t('desktop.infra_connect_hint') : t('desktop.infra_map_hint');
            stageNodeLayer.querySelectorAll('[data-id]').forEach(button => {
                button.addEventListener('click', event => {
                    if (state.dragMoved) { state.dragMoved = false; event.preventDefault(); return; }
                    if (state.edgeMode) return chooseEdgeEndpoint(button.dataset.id);
                    selectNode(button.dataset.id, 'stage');
                });
                button.addEventListener('keydown', event => {
                    if (event.key === 'Enter' && state.keyboardMove) { event.preventDefault(); void saveKeyboardMove(button.dataset.id); }
                });
            });
            if (restoreFocus && state.selected) stageNodeLayer.querySelector(`[data-id="${CSS.escape(state.selected)}"]`)?.focus({ preventScroll: true });
            updateUndoButton();
        };
        const paintTable = () => {
            const items = filteredNodes().sort((a, b) => a.name.localeCompare(b.name));
            const scroll = q('[data-table-scroll]');
            const slice = virtualSlice(items, scroll.scrollTop, scroll.clientHeight || 600);
            q('[data-table-total]').textContent = t('desktop.infra_asset_count').replace('{count}', items.length);
            const row = node => `<tr data-row="${esc(node.id)}" class="${state.selected === node.id ? 'is-selected' : ''}" tabindex="0" aria-selected="${state.selected === node.id}"><td><button class="infra-table-name" data-select="${esc(node.id)}">${icon(node.icon)}<span>${esc(node.name)}</span></button></td><td>${esc(labelForKind(node.kind))}</td><td>${esc([node.profile?.vendor, node.profile?.model].filter(Boolean).join(' · ') || '—')}</td><td>${esc(node.profile?.ip_address || node.profile?.mac_address || '—')}</td><td>${esc((node.sources || []).map(source => t(`desktop.infra_source_${source.kind}`)).join(', ') || t('desktop.infra_manual'))}</td><td><span class="infra-state-pill" data-state="${esc(sourceState(node))}"><i></i>${esc(statusLabel(sourceState(node)))}</span></td></tr>`;
            q('[data-table-body]').innerHTML = `<tr class="infra-virtual-spacer"><td colspan="6" style="height:${slice.before}px"></td></tr>${slice.rows.map(row).join('')}<tr class="infra-virtual-spacer"><td colspan="6" style="height:${slice.after}px"></td></tr>`;
            q('[data-table-body]').querySelectorAll('[data-select]').forEach(button => button.addEventListener('click', () => selectNode(button.dataset.select, 'table')));
            q('[data-table-body]').querySelectorAll('[data-row]').forEach(tr => tr.addEventListener('keydown', event => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); selectNode(tr.dataset.row); } }));
        };
        const paintTray = () => {
            const items = state.nodes.filter(node => !isPlaced(node, state.view === 'table' ? 'atlas' : state.view) && matchesFilters(node));
            q('[data-tray-count]').textContent = String(items.length);
            q('[data-tray]').hidden = state.view === 'table' || items.length === 0;
            q('[data-tray-items]').innerHTML = items.map(node => `<button type="button" class="infra-tray-item ${state.selected === node.id ? 'is-selected' : ''}" data-place="${esc(node.id)}" aria-label="${esc(t('desktop.infra_place_asset').replace('{name}', node.name))}">${icon(node.icon)}<span><strong>${esc(node.name)}</strong><small>${esc(labelForKind(node.kind))}</small></span><i>＋</i></button>`).join('');
            q('[data-tray-items]').querySelectorAll('[data-place]').forEach(button => button.addEventListener('click', () => {
                state.selected = button.dataset.place;
                state.keyboardMove = true;
                const next = nextPosition(state.view, state.selected);
                void persistLayout(new Map([[state.selected, next]]));
            }));
        };
        const matchesFilters = node => {
            if (state.kindFilter && node.kind !== state.kindFilter) return false;
            if (state.filterSource && !(node.sources || []).some(source => source.kind === state.filterSource)) return false;
            if (state.query && !searchableText(node).includes(state.query.toLocaleLowerCase())) return false;
            if (state.scope === 'attention' && !['stale', 'unknown', 'removed'].includes(sourceState(node))) return false;
            return true;
        };
        const paintView = () => {
            const table = state.view === 'table';
            q('[data-stage-view]').hidden = table;
            q('[data-table-view]').hidden = !table;
            q('[data-tray]').hidden = table || !state.nodes.some(node => !isPlaced(node, state.view) && matchesFilters(node));
            root.querySelectorAll('[data-view]').forEach(button => button.setAttribute('aria-pressed', String(button.dataset.view === state.view)));
            root.querySelectorAll('[data-scope]').forEach(button => button.setAttribute('aria-pressed', String(button.dataset.scope === state.scope)));
            paintStatus(); paintGroups(); paintStage(); paintTable(); paintTray(); paintInspector(); paintToolbar();
        };
        const paintAll = () => { if (!state.disposed && root.isConnected) paintView(); };
        const ensureMapView = () => {
            if (state.view !== 'table') return;
            state.view = 'atlas'; paintAll();
        };
        const profileFields = [
            ['role', 'role'], ['vendor', 'vendor'], ['model', 'model'], ['serial', 'serial'], ['os', 'os'], ['firmware', 'firmware'],
            ['cpu', 'cpu'], ['ram', 'ram'], ['storage', 'storage'], ['rack_position', 'rack_position'], ['ip_address', 'ip_address'], ['mac_address', 'mac_address']
        ];
        const reviewValue = (value, key = '') => {
            if (key === 'credential_id') {
                const credential = state.credentials.find(item => item.id === value);
                if (credential) return credential.name;
            }
            if (Array.isArray(value)) return value.join(key === 'tags' ? '; ' : ', ');
            if (value && typeof value === 'object') return JSON.stringify(value).slice(0, 180);
            return value == null || value === '' ? t('desktop.infra_unknown') : String(value);
        };
        const nodeReviewRows = (latest, draft) => {
            const rows = [];
            const add = (key, before, after, prefix = '') => {
                if (JSON.stringify(before) !== JSON.stringify(after)) {
                    const labels = { name: t('desktop.infra_name'), kind: t('desktop.infra_type'), icon: t('desktop.infra_icon'), parent_id: t('desktop.infra_group') };
                    const display = value => key === 'parent_id' ? (nodeById(value)?.name || (value ? String(value) : t('desktop.infra_no_group'))) : key === 'icon' ? t(iconLabelKey(value)) : reviewValue(value, key);
                    rows.push({ label: prefix ? t(`desktop.infra_field_${key}`) : labels[key] || key, current: display(before), draft: display(after) });
                }
            };
            for (const key of ['name', 'kind', 'icon', 'parent_id']) add(key, latest?.[key], draft?.[key]);
            const profileKeys = new Set([...Object.keys(latest?.profile || {}), ...Object.keys(draft?.profile || {})]);
            for (const key of profileKeys) add(key, latest?.profile?.[key], draft?.profile?.[key], 'profile');
            return rows;
        };
        const reviewDiffMarkup = (rows, currentLabel) => rows.length ? `<div class="infra-review-diff">${rows.map(row => `<div class="infra-review-row"><strong>${esc(row.label)}</strong><div><small>${esc(currentLabel)}</small><span>${esc(row.current)}</span></div><div><small>${esc(t('desktop.infra_your_draft'))}</small><span>${esc(row.draft)}</span></div></div>`).join('')}</div>` : `<p class="infra-hint">${esc(t('desktop.infra_no_field_differences'))}</p>`;
        const sourceReviewMarkup = id => {
            const review = state.sourceReviews.get(id); if (!review) return '';

            const rows = Object.keys(review.submitted).map(key => ({ label: t(`desktop.infra_field_${key}`), current: reviewValue(review.latest.fields?.[key], key), draft: reviewValue(review.submitted[key], key) })).filter((row, index) => row.current !== row.draft);
            return `<section class="infra-conflict-review" role="region" aria-label="${esc(t('desktop.infra_source_compare'))}"><strong>${esc(t('desktop.infra_source_compare'))}</strong>${reviewDiffMarkup(rows, t('desktop.infra_current_source'))}<button type="button" data-action="reapply-source-draft" class="infra-button-primary">${esc(t('desktop.infra_reapply_draft'))}</button></section>`;
        };
        const nodeReviewMarkup = id => {
            const review = state.nodeReviews.get(id); if (!review) return '';
            return `<section class="infra-conflict-review" role="region" aria-label="${esc(t('desktop.infra_node_compare'))}"><strong>${esc(t('desktop.infra_node_compare'))}</strong>${reviewDiffMarkup(nodeReviewRows(review.latest, state.drafts.get(id)), t('desktop.infra_current_inventory'))}<button type="button" data-action="reapply-node-draft" class="infra-button-primary">${esc(t('desktop.infra_reapply_draft'))}</button></section>`;
        };
        const inputFor = (family, index, key, value, type = 'text', attrs = '') => `<label class="infra-field"><span>${esc(t(`desktop.infra_field_${key}`))}</span><input type="${type}" data-list-family="${family}" data-index="${index}" data-key="${key}" value="${esc(value ?? '')}" ${attrs} ${!canWrite() ? 'readonly' : ''}></label>`;
        const listEditor = (family, title, fields, rows, addKey, removeKey) => `<div class="infra-list-editor" data-list-editor="${family}"><div class="infra-list-heading"><span>${esc(t(`desktop.infra_${title}`))}</span><button type="button" data-list-add="${family}" ${!canWrite() ? 'disabled' : ''}>＋ ${esc(t(`desktop.infra_${addKey}`))}</button></div>${rows.length ? rows.map((row, index) => `<div class="infra-list-row" data-list-row="${family}" data-index="${index}"><div class="infra-fields-grid">${fields.map(([key, type, attrs]) => inputFor(family, index, key, row[key], type, attrs || '')).join('')}</div><button type="button" class="infra-list-remove" data-list-remove="${family}" data-index="${index}" aria-label="${esc(t(`desktop.infra_${removeKey}`))}" ${!canWrite() ? 'disabled' : ''}>×</button></div>`).join('') : `<p class="infra-hint">${esc(t('desktop.infra_list_empty'))}</p>`}</div>`;
        const customEditor = (nodeID, profile) => {
            const rows = state.customRows.get(nodeID) || Object.entries(profile.custom || {}).map(([key, value]) => ({ key, value }));
            return `<div class="infra-list-editor" data-list-editor="custom"><div class="infra-list-heading"><span>${esc(t('desktop.infra_custom_fields'))}</span><button type="button" data-list-add="custom" ${!canWrite() ? 'disabled' : ''}>＋ ${esc(t('desktop.infra_add_custom_field'))}</button></div>${rows.length ? rows.map((row, index) => `<div class="infra-list-row" data-list-row="custom" data-index="${index}"><div class="infra-fields-grid">${inputFor('custom', index, 'key', row.key, 'text', 'required maxlength="80" pattern="[A-Za-z0-9_.-]+"')}<label class="infra-field"><span>${esc(t('desktop.infra_custom_value'))}</span><input data-list-family="custom" data-index="${index}" data-key="value" value="${esc(row.value ?? '')}" maxlength="2048" ${!canWrite() ? 'readonly' : ''}></label></div><button type="button" class="infra-list-remove" data-list-remove="custom" data-index="${index}" aria-label="${esc(t('desktop.infra_remove_custom_field'))}" ${!canWrite() ? 'disabled' : ''}>×</button></div>`).join('') : `<p class="infra-hint">${esc(t('desktop.infra_list_empty'))}</p>`}</div>`;
        };
        const appBindingEditor = profile => {
            const bindings = profile.app_bindings || [];
            const selected = new Set(bindings.map(item => `${item.app}\u0000${item.target_id}`));
            const available = new Set(state.appTargets.map(target => `${target.app}\u0000${target.target_id}`));
            const unavailable = bindings.filter(item => !available.has(`${item.app}\u0000${item.target_id}`));
            const targetOption = (target, missing = false) => `<label class="infra-target-option ${missing ? 'is-unavailable' : ''}"><input type="checkbox" data-app-target data-app="${esc(target.app)}" data-target-id="${esc(target.target_id)}" ${selected.has(`${target.app}\u0000${target.target_id}`) ? 'checked' : ''} ${!canWrite() ? 'disabled' : ''}><span><strong>${esc(missing ? `${target.target_id} · ${t('desktop.infra_target_unavailable')}` : target.label || target.target_id)}</strong><small>${esc(t(`desktop.infra_app_${target.app.replaceAll('-', '_')}`))}</small></span></label>`;
            const empty = !state.appTargets.length ? `<p class="infra-hint">${esc(t(state.targetsError ? 'desktop.infra_targets_error' : state.targetsLoaded ? 'desktop.infra_no_app_targets' : 'desktop.infra_targets_loading'))}</p>` : '';
            return `<div class="infra-list-editor" data-list-editor="app-bindings"><div class="infra-list-heading"><span>${esc(t('desktop.infra_application_links'))}</span><span>${esc(t('desktop.infra_explicit_links_only'))}</span></div>${empty}<div class="infra-target-list">${state.appTargets.map(target => targetOption(target)).join('')}${unavailable.map(target => targetOption(target, true)).join('')}</div>${state.targetsError ? `<button type="button" data-action="reload-targets">${esc(t('desktop.infra_retry_targets'))}</button>` : ''}</div>`;
        };
        const appContext = link => ({
            'quick-connect': { device_id: link.target_id },
            'network-cameras': { camera_id: link.target_id },
            'software-store': { app_id: link.target_id },
            meshcore: { conversation_id: link.target_id },
        }[link.app] || { target_id: link.target_id });
        const projectedAppLinks = node => (node.app_links || []).map(link => `<button type="button" class="infra-app-link" data-open-app="${esc(link.app)}" data-target-id="${esc(link.target_id)}">${esc(t(`desktop.infra_app_${link.app.replaceAll('-', '_')}`))}<span aria-hidden="true">↗</span></button>`).join('');
        const sourceFieldMarkup = ([key, value], editable, sourceKind) => {
            const label = esc(t(`desktop.infra_field_${key}`));
            if (key === 'protocol') return `<label class="infra-field"><span>${label}</span><select data-source-field="${esc(key)}" ${editable && canEditSource() ? '' : 'disabled'}>${['none', 'ssh', 'vnc'].map(protocol => `<option value="${protocol}" ${value === protocol ? 'selected' : ''}>${esc(t(`desktop.infra_protocol_${protocol}`))}</option>`).join('')}</select></label>`;
            if (key === 'credential_id') {
                const current = String(value || '');
                const found = state.credentials.some(credential => credential.id === current);
                const extra = current && !found ? `<option value="${esc(current)}" selected>${esc(t('desktop.infra_credential_unlisted'))} · ${esc(current)}</option>` : '';
                const hint = state.credentialsError ? `<small class="infra-field-hint">${esc(t('desktop.infra_credentials_load_failed'))}</small>` : '';
                return `<label class="infra-field"><span>${label}</span><select data-source-field="${esc(key)}" ${editable && canEditSource() ? '' : 'disabled'}><option value="">${esc(t('desktop.infra_no_credential'))}</option>${extra}${state.credentials.map(credential => `<option value="${esc(credential.id)}" ${current === credential.id ? 'selected' : ''}>${esc(credential.name)}${credential.type ? ` · ${esc(credential.type)}` : ''}</option>`).join('')}</select>${hint}</label>`;
            }
            const fieldValue = Array.isArray(value) ? value.join(', ') : value ?? '';
            const placeholder = key === 'tags' && sourceKind === 'proxmox' ? ` placeholder="${esc(t('desktop.infra_pve_tags_hint'))}"` : '';
            return `<label class="infra-field"><span>${label}</span><input data-source-field="${esc(key)}" type="${key === 'port' ? 'number' : 'text'}" value="${esc(fieldValue)}"${placeholder} ${editable && canEditSource() ? '' : 'readonly'} ${key === 'port' ? 'min="1" max="65535"' : ''}></label>`;
        };
        const sourceMarkup = (source, node) => {
            const actions = sourceActions(source.kind);
            const providerHost = source.kind === 'proxmox' && String(source.key || '').startsWith('node:');
            const blocked = actions.filter(action => !allowedAction(node, action, source) && actionReason(node, source, action));
            const reasons = [...new Set(blocked.map(action => actionReason(node, source, action)))];
            const reasonLabel = reasons.length ? t(`desktop.infra_reason_${reasons[0]}`) : '';
            const blockedLabel = blocked.map(action => t(`desktop.infra_action_${action}`)).join(', ');
            return `<div class="infra-source-row" data-source-row><div class="infra-source-icon" data-source="${esc(source.kind)}">${esc(source.kind.slice(0, 1).toUpperCase())}</div><div class="infra-source-copy"><strong>${esc(t(`desktop.infra_source_${source.kind}`))}</strong><span>${esc(source.name || source.instance || source.key || '')}</span>${reasonLabel ? `<small class="infra-action-reason">${esc(blockedLabel)} · ${esc(reasonLabel)}</small>` : ''}</div><span class="infra-state-pill" data-state="${esc(source.state)}"><i></i>${esc(statusLabel(source.state))}</span><small>${esc(source.fetched_at ? new Date(source.fetched_at).toLocaleString() : t('desktop.infra_no_observation'))}</small>${canEditSource() && !providerHost ? `<button type="button" data-edit-source="${esc(source.kind)}">${esc(t('desktop.infra_edit_source'))}</button>` : ''}${actions.some(action => allowedAction(node, action, source)) ? `<button type="button" data-source-action="${esc(source.kind)}" data-source-identity="${esc(sourceIdentity(source))}">${esc(t('desktop.infra_actions'))}</button>` : ''}</div>`;
        };
        const paintInspector = () => {
            if (state.disposed || !root.isConnected) return;
            const node = nodeById(state.selected);
            const panel = q('[data-inspector]');
            panel.classList.toggle('is-empty', !node);
            if (!node) {
                panel.innerHTML = `<div class="infra-inspector-empty"><span aria-hidden="true">◈</span><strong>${esc(t('desktop.infra_select_asset'))}</strong><p>${esc(t('desktop.infra_select_asset_hint'))}</p></div>`;
                return;
            }
            const draft = state.drafts.get(node.id) || clone(node);
            state.drafts.set(node.id, draft);
            const sources = node.sources || [];
            const placement = node.placements?.[state.view === 'table' ? 'atlas' : state.view];
            const placementLabel = placement
                ? t('desktop.infra_position').replaceAll('{x}', Math.round(placement.x)).replaceAll('{y}', Math.round(placement.y))
                : t('desktop.infra_unplaced');
            const dirty = state.dirty.has(node.id);
            const field = (name, label, value, type = 'text') => `<label class="infra-field"><span>${esc(t(`desktop.infra_field_${label}`))}</span><input data-profile="${esc(name)}" type="${type}" value="${esc(value || '')}" ${!canWrite() ? 'readonly' : ''} autocomplete="off"></label>`;
            const sourceEdit = state.sourceDrafts.get(node.id);
            const services = node.kind === 'service' ? [] : hostedServices(node.id);
            panel.innerHTML = `<div class="infra-detail-head"><div class="infra-detail-icon">${icon(node.icon)}</div><div><p class="infra-kicker">${esc(t(`desktop.infra_kind_${node.kind}`))} · ${esc(t('desktop.infra_version').replace('{version}', node.version))}</p><h2>${esc(node.name)}</h2></div><button type="button" class="infra-detail-close" data-action="close-inspector" aria-label="${esc(t('desktop.close'))}" title="${esc(t('desktop.close'))}">×</button><button type="button" class="infra-detail-more" data-action="node-more" aria-label="${esc(t('desktop.infra_more_actions'))}">···</button></div>
              ${dirty ? `<div class="infra-draft-banner" role="status"><i></i>${esc(t('desktop.infra_draft_kept'))}</div>` : ''}${nodeReviewMarkup(node.id)}
              <form class="infra-detail-form" id="infra-detail-form" data-detail-form><div class="infra-form-section"><div class="infra-form-heading"><h3>${esc(t('desktop.infra_identity'))}</h3><span class="infra-state-pill" data-state="${esc(sourceState(node))}"><i></i>${esc(statusLabel(sourceState(node)))}</span></div><label class="infra-field"><span>${esc(t('desktop.infra_name'))}</span><input data-node-field="name" value="${esc(draft.name)}" ${!canWrite() ? 'readonly' : ''} maxlength="120" required></label><div class="infra-field-row"><label class="infra-field"><span>${esc(t('desktop.infra_type'))}</span><select data-node-field="kind" ${!canWrite() ? 'disabled' : ''}>${KINDS.map(kind => `<option value="${kind}" ${draft.kind === kind ? 'selected' : ''}>${esc(labelForKind(kind))}</option>`).join('')}</select></label><label class="infra-field"><span>${esc(t('desktop.infra_icon'))}</span><select data-node-field="icon" ${!canWrite() ? 'disabled' : ''}>${ICON_LIST.map(key => `<option value="${key}" ${iconId(draft.icon) === key ? 'selected' : ''}>${esc(t(iconLabelKey(key)))}</option>`).join('')}</select></label></div></div>
              <div class="infra-form-section"><h3>${esc(t('desktop.infra_hardware'))}</h3><div class="infra-fields-grid">${profileFields.map(([name, label]) => field(name, label, draft.profile?.[name])).join('')}</div>${draft.kind === 'network' ? `<section class="infra-network-segment"><h4>${esc(t('desktop.infra_network_segment'))}</h4><div class="infra-field-row"><label class="infra-field"><span>${esc(t('desktop.infra_cidr'))}</span><input data-profile="cidr" type="text" value="${esc(draft.profile?.cidr || '')}" placeholder="192.168.10.0/24" ${!canWrite() ? 'readonly' : ''}></label><label class="infra-field"><span>${esc(t('desktop.infra_vlan'))}</span><input data-profile="vlan" type="number" min="1" max="4094" value="${esc(draft.profile?.vlan || '')}" placeholder="${esc(t('desktop.infra_unknown'))}" ${!canWrite() ? 'readonly' : ''}></label></div><p class="infra-hint">${esc(t('desktop.infra_cidr_hint'))}</p></section>` : ''}<label class="infra-field"><span>${esc(t('desktop.infra_tags'))}</span><input data-profile="tags" value="${esc((draft.profile?.tags || []).join(', '))}" placeholder="${esc(t('desktop.infra_tags_hint'))}" ${!canWrite() ? 'readonly' : ''}></label><label class="infra-field"><span>${esc(t('desktop.infra_notes'))}</span><textarea data-profile="notes" rows="3" ${!canWrite() ? 'readonly' : ''}>${esc(draft.profile?.notes || '')}</textarea></label>${listEditor('interfaces', 'interfaces', [['name','text','required maxlength="64"'],['ip_address','text'],['mac_address','text'],['vlan','text'],['speed','text']], draft.profile?.interfaces || [], 'add_interface', 'remove_interface')}${listEditor('links', 'links', [['label','text','required maxlength="128"'],['url','url','placeholder="https://" required maxlength="2048"']], draft.profile?.links || [], 'add_link', 'remove_link')}${customEditor(node.id, draft.profile || {})}${appBindingEditor(draft.profile || {})}</div>
              <div class="infra-form-section"><div class="infra-form-heading"><h3>${esc(t('desktop.infra_placement'))}</h3><span>${esc(placementLabel)}</span></div><div class="infra-quick-place"><button type="button" data-place-choice="center" ${!canWrite() ? 'disabled' : ''}>${esc(t('desktop.infra_place_center'))}</button><button type="button" data-place-choice="northwest" ${!canWrite() ? 'disabled' : ''}>${esc(t('desktop.infra_place_northwest'))}</button><button type="button" data-place-choice="keyboard" ${!canWrite() ? 'disabled' : ''}>${esc(t('desktop.infra_place_keyboard'))}</button></div><p class="infra-hint">${esc(t('desktop.infra_keyboard_hint'))}</p></div>
              ${sources.length ? `<div class="infra-form-section"><div class="infra-form-heading"><h3>${esc(t('desktop.infra_sources'))}</h3><span>${sources.length}</span></div><div class="infra-source-list">${sources.map(source => sourceMarkup(source, node)).join('')}</div></div>` : `<div class="infra-form-section"><h3>${esc(t('desktop.infra_sources'))}</h3><p class="infra-hint">${esc(t('desktop.infra_manual_only'))}</p></div>`}
              ${(node.app_links || []).length ? `<div class="infra-form-section"><h3>${esc(t('desktop.infra_open_in'))}</h3><div class="infra-app-links">${projectedAppLinks(node)}</div></div>` : ''}
              ${sourceEdit ? `<div class="infra-form-section infra-source-edit"><div class="infra-form-heading"><h3>${esc(t('desktop.infra_source_edit_title'))}</h3><span>${esc(t(`desktop.infra_source_${sourceEdit.source}`))}</span></div>${state.sourceDirty.has(node.id) ? `<div class="infra-draft-banner" role="status"><i></i>${esc(t('desktop.infra_source_draft'))}</div>` : ''}${state.sourcePending.has(node.id) && !state.sourceDirty.has(node.id) ? `<p class="infra-draft-banner" role="status"><i></i><span>${esc(t('desktop.infra_source_pending'))}</span></p>` : ''}<div data-source-form>${sourceReviewMarkup(node.id)}${Object.entries(sourceEdit.fields || {}).filter(([key]) => SOURCE_EDIT_FIELDS[sourceEdit.source]?.includes(key)).map(entry => sourceFieldMarkup(entry, sourceEdit.editable, sourceEdit.source)).join('')}<p class="infra-hint" data-source-reason>${esc(t(`desktop.infra_reason_${sourceEdit.reason}`))}</p>${state.sourceBlocked.has(node.id) ? `<p class="infra-draft-banner" role="status"><i></i><span>${esc(t(state.sourcePending.has(node.id) ? 'desktop.infra_source_pending_review' : 'desktop.infra_source_review_needed'))}</span></p>` : ''}<div class="infra-form-actions"><button type="button" data-action="close-source-edit">${esc(t('desktop.cancel'))}</button>${state.sourceBlocked.has(node.id) ? `<button type="button" data-action="review-source">${esc(t(state.sourcePending.has(node.id) ? 'desktop.infra_review_pending_source' : 'desktop.infra_review_source'))}</button>` : ''}<button type="button" data-action="save-source" class="infra-button-primary" ${!sourceEdit.editable || !canEditSource() || !state.sourceDirty.has(node.id) || state.sourceBlocked.has(node.id) || state.busy.has(`source-${node.id}`) ? 'disabled' : ''}>${esc(state.busy.has(`source-${node.id}`) ? t('desktop.infra_saving') : t('desktop.infra_save_source'))}</button></div></div></div>` : ''}
              <div class="infra-form-actions infra-detail-actions">${canWrite() ? `<button type="button" data-action="delete-node" class="infra-danger-button">${esc(t('desktop.infra_remove_node'))}</button><span class="infra-flex-spacer"></span>${dirty ? `<button type="button" data-action="revert-node">${esc(t('desktop.infra_discard'))}</button>` : ''}<button type="submit" form="infra-detail-form" class="infra-button-primary" ${dirty ? '' : 'disabled'}>${esc(state.busy.has('node-' + node.id) ? t('desktop.infra_saving') : t('desktop.save'))}</button>` : `<span class="infra-readonly-note">${esc(t('desktop.infra_read_only'))}</span>`}</div></form>
              <div class="infra-relations"><div class="infra-form-heading"><h3>${esc(t('desktop.infra_relations'))}</h3>${canWrite() ? `<button type="button" data-action="add-edge">＋ ${esc(t('desktop.infra_add_relation'))}</button>` : ''}</div>${state.edges.filter(edge => edge.from === node.id || edge.to === node.id).map(edge => { const other = nodeById(edge.from === node.id ? edge.to : edge.from); return `<div class="infra-relation-row"><span class="infra-edge-kind" data-edge-kind="${esc(edge.kind)}"></span><span>${esc(t(`desktop.infra_edge_${edge.kind}`))}</span><strong>${esc(other?.name || edge.to)}</strong><button type="button" data-edit-edge="${esc(edge.id)}" aria-label="${esc(t('desktop.infra_edit_relation'))}" ${!canWrite() || edge.source ? 'disabled' : ''}>✎</button></div>`; }).join('') || `<p class="infra-hint">${esc(t('desktop.infra_no_relations'))}</p>`}</div>
              ${services.length ? `<section class="infra-hosted-services"><div class="infra-form-heading"><h3>${esc(t('desktop.infra_hosted_services'))} <span>${services.length}</span></h3><button type="button" data-toggle-host-services="${esc(node.id)}" aria-expanded="${state.expandedHosts.has(node.id)}" aria-controls="infra-hosted-${esc(safeWindowId)}-${esc(String(node.id).replace(/[^a-zA-Z0-9_-]/g, ''))}">${esc(t(state.expandedHosts.has(node.id) ? 'desktop.infra_collapse_hosted_services' : 'desktop.infra_expand_hosted_services'))}</button></div><div id="infra-hosted-${esc(safeWindowId)}-${esc(String(node.id).replace(/[^a-zA-Z0-9_-]/g, ''))}" class="infra-component-list" ${state.expandedHosts.has(node.id) ? '' : 'hidden'}>${services.map(service => `<button type="button" data-hosted-service="${esc(service.id)}">${icon(service.icon)}<span>${esc(service.name)}</span><small>${esc(t('desktop.infra_kind_service'))}</small></button>`).join('')}</div></section>` : ''}`;
            if (node.kind === 'service') {
                const logicalEdges = state.edges.filter(edge => ['runs_on', 'depends_on', 'powered_by'].includes(edge.kind) && (edge.from === node.id || edge.to === node.id));
                if (logicalEdges.length) {
                    const details = document.createElement('details'); details.className = 'infra-service-components';
                    const summary = document.createElement('summary'); summary.textContent = t('desktop.infra_service_components') + ` (${logicalEdges.length})`; details.append(summary);
                    const list = document.createElement('div'); list.className = 'infra-component-list';
                    for (const edge of logicalEdges) {
                        const other = nodeById(edge.from === node.id ? edge.to : edge.from); if (!other) continue;
                        const button = document.createElement('button'); button.type = 'button'; button.dataset.relatedNode = other.id;
                        button.textContent = `${t(`desktop.infra_edge_${edge.kind}`)} · ${other.name}`; list.append(button);
                    }
                    details.append(list); panel.querySelector('.infra-relations')?.before(details);
                    list.querySelectorAll('[data-related-node]').forEach(button => button.addEventListener('click', () => selectNode(button.dataset.relatedNode)));
                }
            }
            const identity = panel.querySelector('.infra-detail-form .infra-form-section');
            if (identity) {
                const label = document.createElement('label'); label.className = 'infra-field';
                label.innerHTML = `<span>${esc(t('desktop.infra_group'))}</span><select data-node-field="parent_id" ${!canWrite() ? 'disabled' : ''}><option value="">${esc(t('desktop.infra_no_group'))}</option></select>`;
                const select = label.querySelector('select'), descendants = new Set(), queue = [node.id];
                while (queue.length) { const current = queue.pop(); for (const child of groupChildren(current)) if (!descendants.has(child.id)) { descendants.add(child.id); queue.push(child.id); } }
                state.nodes.filter(group => group.kind === 'group' && group.id !== node.id && !descendants.has(group.id)).forEach(group => {
                    const option = document.createElement('option'); option.value = group.id; option.textContent = group.name; option.selected = group.id === draft.parent_id; select.append(option);
                });
                if (!draft.parent_id) select.value = '';
                identity.append(label);
            }
            panel.querySelector('[data-detail-form]')?.addEventListener('input', updateDraft);
            panel.querySelector('[data-detail-form]')?.addEventListener('change', updateDraft);
            panel.querySelector('[data-detail-form]')?.addEventListener('submit', event => { event.preventDefault(); void saveNode(node.id); });
            panel.querySelectorAll('[data-place-choice]').forEach(button => button.addEventListener('click', () => placeByChoice(node.id, button.dataset.placeChoice)));
            panel.querySelectorAll('[data-open-app]').forEach(button => button.addEventListener('click', () => {
                const link = { app: button.dataset.openApp, target_id: button.dataset.targetId };
                ctx.openApp?.(link.app, appContext(link));
            }));
            panel.querySelector('[data-toggle-host-services]')?.addEventListener('click', event => {
                const hostID = event.currentTarget.dataset.toggleHostServices;
                if (state.expandedHosts.has(hostID)) state.expandedHosts.delete(hostID); else state.expandedHosts.add(hostID);
                paintStage(); paintInspector();
            });
            panel.querySelectorAll('[data-hosted-service]').forEach(button => button.addEventListener('click', () => selectNode(button.dataset.hostedService, 'stage')));
            panel.querySelectorAll('[data-edit-source]').forEach(button => button.addEventListener('click', () => void loadSourceEdit(node.id, button.dataset.editSource)));
            panel.querySelectorAll('[data-source-action]').forEach(button => button.addEventListener('click', () => {
                const source = (nodeById(node.id)?.sources || []).find(item => sourceIdentity(item) === button.dataset.sourceIdentity);
                if (source) openActionMenu(node.id, source);
            }));
            panel.querySelector('[data-action="save-source"]')?.addEventListener('click', () => void saveSourceEdit(node.id));
            panel.querySelector('[data-action="review-source"]')?.addEventListener('click', () => void reviewSourceEdit(node.id));
            panel.querySelector('[data-action="close-source-edit"]')?.addEventListener('click', () => { state.sourceDrafts.delete(node.id); state.sourceDirty.delete(node.id); state.sourceBlocked.delete(node.id); state.sourcePending.delete(node.id); state.sourceReviews.delete(node.id); paintInspector(); });
            panel.querySelector('[data-action="reapply-source-draft"]')?.addEventListener('click', () => reapplySourceDraft(node.id));
            panel.querySelector('[data-action="reapply-node-draft"]')?.addEventListener('click', () => reapplyNodeDraft(node.id));
            panel.querySelector('[data-action="delete-node"]')?.addEventListener('click', () => void deleteNode(node.id));
            panel.querySelector('[data-action="revert-node"]')?.addEventListener('click', () => { state.drafts.set(node.id, clone(node)); state.dirty.delete(node.id); state.nodeReviews.delete(node.id); state.customRows.delete(node.id); paintInspector(); });
            panel.querySelectorAll('[data-edit-edge]').forEach(button => button.addEventListener('click', () => openEdgeDialog(button.dataset.editEdge, node.id)));
            panel.querySelector('[data-action="close-inspector"]')?.addEventListener('click', () => {
                const closedID = state.selected;
                state.selected = '';
                paintGroups(); paintStage(); paintTable(); paintTray(); paintInspector();
                const target = state.view === 'table'
                    ? root.querySelector(`[data-row="${CSS.escape(closedID)}"]`)
                    : root.querySelector(`[data-node="${CSS.escape(closedID)}"]`);
                target?.focus({ preventScroll: true });
            });
            const updateSourceDraft = input => {
                const current = state.sourceDrafts.get(node.id);
                if (current) current.fields[input.dataset.sourceField] = input.value;
                state.sourceReviews.delete(node.id);
                state.sourceDirty.add(node.id);
                state.sourceRevision.set(node.id, (state.sourceRevision.get(node.id) || 0) + 1);
                const save = panel.querySelector('[data-action="save-source"]'); if (save) save.disabled = !canEditSource() || state.sourceBlocked.has(node.id);
                if (!panel.querySelector('.infra-source-edit .infra-draft-banner')) { const banner = document.createElement('div'); banner.className = 'infra-draft-banner'; banner.setAttribute('role', 'status'); banner.innerHTML = `<i></i>${esc(t('desktop.infra_source_draft'))}`; panel.querySelector('.infra-source-edit .infra-form-heading')?.after(banner); }
            };
            panel.querySelectorAll('[data-source-field]').forEach(input => {
                input.addEventListener('input', () => updateSourceDraft(input));
                input.addEventListener('change', () => updateSourceDraft(input));
            });
        };
        function updateDraft(event) {
            if (!canWrite() || event.target.dataset.sourceField) return;
            const node = nodeById(state.selected); if (!node) return;
            const draft = state.drafts.get(node.id) || clone(node);
            const target = event.target;
            if (target.dataset.nodeField) draft[target.dataset.nodeField] = target.value;
            if (target.dataset.profile) {
                const key = target.dataset.profile; draft.profile ||= {};
                if (key === 'tags') draft.profile[key] = target.value.split(',').map(value => value.trim()).filter(Boolean);
                else if (key === 'vlan') draft.profile[key] = target.value === '' ? 0 : Number(target.value);
                else draft.profile[key] = target.value;
            }
            const form = q('[data-detail-form]');
            if (form) readProfileLists(form, draft, node.id);
            state.drafts.set(node.id, draft);
            state.dirty.add(node.id);
            state.draftRevision.set(node.id, (state.draftRevision.get(node.id) || 0) + 1);
            const valid = form ? validateDetailForm(form, draft, node.id) : true;
            const save = q('[data-inspector] [type="submit"].infra-button-primary'); if (save) save.disabled = !valid;
            let banner = q('[data-inspector] .infra-draft-banner');
            if (banner?.closest('.infra-source-edit')) banner = null;
            if (!banner) { banner = document.createElement('div'); banner.className = 'infra-draft-banner'; banner.setAttribute('role', 'status'); banner.innerHTML = '<i></i><span></span>'; q('[data-inspector]').prepend(banner); }
            let label = banner.querySelector('span');
            if (!label) { label = document.createElement('span'); banner.append(label); }
            label.textContent = t('desktop.infra_draft_kept');
            if (target.matches('[data-node-field="kind"]')) paintInspector();
        }
        function validateDetailForm(form, draft, id) {
            const cidr = form.querySelector('[data-profile="cidr"]');
            if (cidr) cidr.setCustomValidity(validCIDR(cidr.value) ? '' : t('desktop.infra_invalid_cidr'));
            for (const input of form.querySelectorAll('[data-list-family="links"][data-key="url"]')) {
                input.setCustomValidity(validHTTPLink(input.value) ? '' : t('desktop.infra_invalid_link'));
            }
            const seen = new Set();
            for (const input of form.querySelectorAll('[data-list-family="custom"][data-key="key"]')) {
                const key = input.value.trim();
                const duplicate = key && seen.has(key);
                input.setCustomValidity(duplicate ? t('desktop.infra_duplicate_custom_key') : '');
                if (key) seen.add(key);
            }
            return form.checkValidity();
        }
        function readProfileLists(form, draft, id) {
            draft.profile ||= {};
            draft.profile.interfaces = [...form.querySelectorAll('[data-list-row="interfaces"]')].map(row => Object.fromEntries([...row.querySelectorAll('[data-list-family="interfaces"]')].map(input => [input.dataset.key, input.value.trim()])));
            draft.profile.links = [...form.querySelectorAll('[data-list-row="links"]')].map(row => Object.fromEntries([...row.querySelectorAll('[data-list-family="links"]')].map(input => [input.dataset.key, input.value.trim()])));
            const customRows = [...form.querySelectorAll('[data-list-row="custom"]')].map(row => Object.fromEntries([...row.querySelectorAll('[data-list-family="custom"]')].map(input => [input.dataset.key, input.value])));
            if (customRows.length || state.customRows.has(id)) {
                state.customRows.set(id, customRows);
                draft.profile.custom = Object.fromEntries(customRows.filter(row => row.key).map(row => [row.key.trim(), row.value]));
            }
            draft.profile.app_bindings = [...form.querySelectorAll('[data-app-target]:checked')].map(input => ({ app: input.dataset.app, target_id: input.dataset.targetId }));
        }
        async function saveNode(id) {
            if (!canWrite() || !state.dirty.has(id) || state.busy.has(`node-${id}`)) return;
            const form = q('[data-detail-form]'), draftProfile = state.drafts.get(id)?.profile || {};
            if (form && !validateDetailForm(form, { profile: draftProfile }, id)) { form.reportValidity(); return; }
            const revision = state.draftRevision.get(id) || 0;
            const draft = clone(state.drafts.get(id));
            draft.id = id;
            setBusy(`node-${id}`, true);
            try {
                const result = await request(`/nodes/${encodeURIComponent(id)}`, { method: 'PUT', body: draft });
                const updated = result.node || result;
                if (updated?.id) replaceNode(updated);
                else await loadSnapshot(false);
                if ((state.draftRevision.get(id) || 0) === revision || !state.dirty.has(id)) {
                    state.dirty.delete(id); state.nodeReviews.delete(id); state.drafts.set(id, clone(nodeById(id) || updated));
                    state.customRows.delete(id);
                    setNotice(t('desktop.infra_saved'), 'success');
                } else {
                    const newer = state.drafts.get(id) || clone(updated);
                    newer.version = updated?.version ?? newer.version;
                    state.drafts.set(id, newer); state.dirty.add(id);
                    setNotice(t('desktop.infra_saved_newer_draft'), 'info');
                }
                paintAll();
            } catch (error) {
                if (error.status === 409) {
                    await loadSnapshot(true);
                    const latest = nodeById(id);
                    if (latest) state.nodeReviews.set(id, { latest: clone(latest) });
                    setNotice(t('desktop.infra_node_conflict_review'), 'error');
                    paintAll();
                } else if (error.name !== 'AbortError') setNotice(messageFor(error), 'error');
            }
            finally { setBusy(`node-${id}`, false); }
        }
        return { paintToolbar, paintGroups, paintStage, paintTable, paintTray, matchesFilters, paintAll, ensureMapView, paintInspector, readProfileLists };
    }

    window.InfraAppViews = { create };
})();
