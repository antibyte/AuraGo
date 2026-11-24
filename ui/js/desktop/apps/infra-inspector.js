// Infra inspector: the right-hand panel for the selected asset (header with
// icon, inline name, place breadcrumb and quick action; Overview, Details,
// Sources and Connections tabs), the selected connection and a multi-selection
// summary. The panel is hidden while nothing is selected; infra.js calls paint()
// on selection events and paintIfIdle() on data events, and flushStale() when
// focus leaves the panel. The Details and Sources forms, drafts and saving live
// in infra-inspector-forms.js (app.forms).
(function () {
    'use strict';

    const TABS = ['overview', 'details', 'sources', 'connections'];
    const PREFS_KEY = 'infra.inspector.v1';
    const EDGE_KINDS = ['ethernet', 'fiber', 'wifi', 'member_of', 'runs_on', 'depends_on', 'powered_by'];
    const LIST_LIMIT = 8;
    const SOURCE_NAMES = ['registry', 'docker', 'proxmox', 'fritzbox'];
    const DIRECTED = new Set(['runs_on', 'depends_on', 'powered_by', 'member_of']);

    function create(app) {
        const { store, state, t, esc, model, root } = app;
        const { clone, sourceState } = model;
        const forms = app.forms;
        const S = app.safeId;
        const canWrite = () => store.canWrite();
        const holder = document.createElement('div');
        holder.innerHTML = `<aside class="infra-inspector" data-inspector aria-label="${esc(t('desktop.infra_details'))}" hidden></aside>`;
        const el = holder.firstElementChild;
        const $ = selector => el.querySelector(selector);
        const titleId = `infra-inspector-title-${S}`;
        let stale = false, prefs = model.readInspectorPrefs(app.readStorage(PREFS_KEY)), activeTab = 'overview', pendingTab = '', lastId = '', renaming = false, renameBefore = '', edgeDirty = false;
        const copyTimers = new Set();
        // Focus survives a repaint: the controls are found again by their data attributes.
        const FOCUS_ATTRS = ['data-action', 'data-inspector-tab', 'data-profile', 'data-node-field', 'data-list-family', 'data-index', 'data-key', 'data-list-add', 'data-list-remove', 'data-jump-tab', 'data-copy-value', 'data-inspector-select', 'data-crumb'];
        let pendingFocus = null;
        const focusKey = node => (node && node !== document.body && el.contains(node) ? { selector: FOCUS_ATTRS.filter(attr => node.hasAttribute(attr)).map(attr => `[${attr}="${CSS.escape(node.getAttribute(attr))}"]`).join('') } : null);
        const rememberFocus = () => { pendingFocus = focusKey(document.activeElement); };
        const draftNoticed = new Set();

        const clearCopyTimers = () => { for (const timer of copyTimers) clearTimeout(timer); copyTimers.clear(); };
        const show = () => { el.hidden = false; root.dataset.inspectorOpen = 'true'; };
        const hide = () => { clearCopyTimers(); forms.detach(); el.hidden = true; el.innerHTML = ''; root.dataset.inspectorOpen = 'false'; lastId = ''; renaming = false; edgeDirty = false; };
        const appLabel = appId => t(`desktop.infra_app_${String(appId).replaceAll('-', '_')}`);
        const closeButton = () => `<button type="button" class="infra-cmd-icon infra-detail-close" data-action="close-inspector" aria-label="${esc(t('desktop.close'))}" title="${esc(t('desktop.close'))}">${app.glyph('close')}</button>`;
        const selectButton = id => `<button type="button" data-inspector-select="${esc(id)}">${esc(store.nodeById(id)?.name || id)}</button>`;
        const edgesOf = node => state.edges.filter(edge => edge.from === node.id || edge.to === node.id);
        // A directed connection read from its target says what the target does for the source ("Hosts", "Powers").
        const edgeLabel = (edge, nodeId) => t(`desktop.infra_edge_${edge.kind}${DIRECTED.has(edge.kind) && edge.to === nodeId && edge.from !== nodeId ? '_inverse' : ''}`);

        // ---- Overview ----------------------------------------------------------------------------
        function overviewHTML(node) {
            const guestIndex = store.guests(), hostId = guestIndex.hostOf.get(node.id), guests = (guestIndex.guestsOf.get(node.id) || []).map(id => store.nodeById(id)).filter(Boolean);
            const profile = node.profile || {}, none = '—';
            const crumbs = model.placePath(node.id, store.nodeById);
            const place = crumbs.map(item => item.name).join(' › ') || t('desktop.infra_no_place');
            const placement = store.effectivePlacement(node.id, app.mapView());
            const autoPlaced = app.canvas.isAutoPlaced(node.id);
            const seen = (node.sources || []).map(source => Date.parse(source.fetched_at)).filter(Number.isFinite).sort((a, b) => b - a)[0];
            const row = (label, value) => `<dt><span>${esc(label)}</span></dt><dd>${value}</dd>`;
            const copyBtn = value => `<button type="button" class="infra-fact-copy" data-copy-value="${esc(value)}" aria-label="${esc(t('desktop.copy'))}" title="${esc(t('desktop.copy'))}">${app.glyph('copy')}</button>`;
            const copyable = value => (value ? `<span class="infra-fact-inline"><span class="infra-fact-num">${esc(value)}</span>${copyBtn(value)}</span>` : esc(none));
            const placeValue = crumbs.length ? `<button type="button" class="infra-fact-link" data-crumb="${esc(crumbs.at(-1).id)}">${esc(place)}</button>` : esc(place);
            let facts = row(t('desktop.infra_address'), copyable(profile.ip_address)) + row(t('desktop.infra_field_mac_address'), copyable(profile.mac_address))
                + row(t('desktop.infra_group_hardware'), esc([profile.vendor, profile.model].filter(Boolean).join(' · ') || none))
                + row(t('desktop.infra_place'), placeValue);
            // The Overview only says whether the asset is on the map; the coordinates are in the Details tab.
            if (!hostId) {
                const placeText = autoPlaced ? t('desktop.infra_topology_auto') : placement ? t('desktop.infra_on_map') : t('desktop.infra_unplaced');
                const placeCell = placement || autoPlaced
                    ? `<button type="button" class="infra-fact-link" data-action="locate-selected" title="${esc(t('desktop.infra_view_map'))}">${esc(placeText)}${app.glyph('locate')}</button>`
                    : esc(placeText);
                facts += row(t('desktop.infra_placement'), placeCell);
            }
            if (hostId && store.nodeById(hostId)) facts += row(t('desktop.infra_fact_host'), `${selectButton(hostId)}${node.host_inferred ? ` <small>· ${esc(t('desktop.infra_host_inferred'))}</small>` : ''}`);
            // Guests sit in their own block under the facts: the heading above, the chips wrapping from the left.
            const guestChip = guest => {
                const key = model.powerLabel(guest.power);
                return `<li><button type="button" data-inspector-select="${esc(guest.id)}"><i class="infra-guest-power" data-power="${esc(guest.power || '')}" aria-hidden="true"></i>${app.icon(guest.icon)}<span>${esc(guest.name)}</span>${key ? `<span class="infra-sr-only">, ${esc(t(key))}</span>` : ''}</button></li>`;
            };
            const guestMore = guests.length > LIST_LIMIT ? `<li><button type="button" data-guest-more="${esc(node.id)}">${esc(model.fill(t('desktop.infra_more_guests'), { count: guests.length - LIST_LIMIT }))}</button></li>` : '';
            const guestsBlock = guests.length ? `<div class="infra-overview-block"><h3 class="infra-overview-heading">${esc(t('desktop.infra_fact_guests'))}</h3><ul class="infra-overview-guests">${guests.slice(0, LIST_LIMIT).map(guestChip).join('')}${guestMore}</ul></div>` : '';
            const connText = model.countText(edgesOf(node).length, t('desktop.infra_connections_summary_one'), t('desktop.infra_connections_summary'));
            facts += row(t('desktop.infra_tab_connections'), `<button type="button" class="infra-fact-link" data-jump-tab="connections">${esc(connText)}${app.glyph('chevronRight')}</button>`)
                + row(t('desktop.infra_fact_last_seen'), esc(seen ? new Date(seen).toLocaleString() : t('desktop.infra_no_observation')));
            const specTiles = [['os', 'desktop.infra_field_os'], ['cpu', 'desktop.infra_field_cpu'], ['ram', 'desktop.infra_field_ram'], ['storage', 'desktop.infra_field_storage'], ['firmware', 'desktop.infra_field_firmware'], ['rack_position', 'desktop.infra_field_rack_position']]
                .filter(([key]) => String(profile[key] || '').trim())
                .map(([key, labelKey]) => `<div class="infra-spec-tile"><span>${esc(t(labelKey))}</span><strong>${esc(profile[key])}</strong></div>`).join('');
            const specs = specTiles ? `<div class="infra-overview-block"><h3 class="infra-overview-heading">${esc(t('desktop.infra_group_hardware'))}</h3><div class="infra-spec-grid">${specTiles}</div></div>` : '';
            const tags = (profile.tags || []).filter(Boolean);
            const tagsBlock = tags.length ? `<div class="infra-overview-block"><h3 class="infra-overview-heading">${esc(t('desktop.infra_tags'))}</h3><ul class="infra-tag-chips">${tags.map(tag => `<li>${esc(tag)}</li>`).join('')}</ul></div>` : '';
            const links = (node.app_links || []).filter(link => model.APP_LINK_TARGETS.has(link.app));
            const apps = links.length ? `<div class="infra-overview-block"><h3 class="infra-overview-heading">${esc(t('desktop.infra_fact_apps'))}</h3><div class="infra-app-links">${links.map(link => `<button type="button" class="infra-app-link" data-open-app="${esc(link.app)}" data-target-id="${esc(link.target_id)}">${esc(appLabel(link.app))}<span aria-hidden="true">↗</span></button>`).join('')}</div></div>` : '';
            const webLinks = (profile.links || []).filter(item => item?.url && model.validHTTPLink(item.url));
            const webLinksBlock = webLinks.length ? `<div class="infra-overview-block"><h3 class="infra-overview-heading">${esc(t('desktop.infra_links'))}</h3><div class="infra-ext-links">${webLinks.map(item => `<a class="infra-ext-link" href="${esc(item.url)}" target="_blank" rel="noopener noreferrer"><span>${esc(item.label || item.url)}</span>${app.glyph('external')}</a>`).join('')}</div></div>` : '';
            const notesText = String(profile.notes || '').trim();
            const notesBlock = notesText ? `<div class="infra-overview-block"><h3 class="infra-overview-heading">${esc(t('desktop.infra_notes'))}</h3><p class="infra-notes-preview">${esc(notesText)}</p></div>` : '';
            // Topology lays assets out by itself; only a guest without a host is placed by hand there, places are not drawn.
            const canPlace = !placement && !hostId && canWrite() && (app.mapView() !== 'topology' || node.kind === 'vm' || node.kind === 'container');
            const logical = node.kind === 'service' ? edgesOf(node).filter(edge => ['runs_on', 'depends_on', 'powered_by'].includes(edge.kind)) : [];
            const components = logical.length ? `<details class="infra-service-components"><summary>${esc(t('desktop.infra_service_components'))} (${logical.length})</summary><div class="infra-component-list">${logical.map(edge => {
                const other = store.nodeById(edge.from === node.id ? edge.to : edge.from);
                return other ? `<button type="button" data-inspector-select="${esc(other.id)}">${esc(edgeLabel(edge, node.id))} · ${esc(other.name)}</button>` : '';
            }).join('')}</div></details>` : '';
            // The move hint names the map and the M key: only in a map view of a writable window.
            const tip = state.view !== 'table' && canWrite() ? `<p class="infra-hint infra-overview-tip">${app.glyph('info')}<span>${esc(t('desktop.infra_move_hint'))}</span></p>` : '';
            return `<dl class="infra-facts infra-facts-card">${facts}</dl>${guestsBlock}${specs}${tagsBlock}${apps}${webLinksBlock}${notesBlock}${components}${canPlace ? `<button type="button" class="infra-button" data-action="place-selected">${esc(t('desktop.infra_place_on_map'))}</button>` : ''}${tip}`;
        }

        // ---- Connections ----------------------------------------------------------------------------
        function connectionsHTML(node) {
            const edges = edgesOf(node), write = canWrite();
            const rows = edges.map(edge => {
                const otherId = edge.from === node.id ? edge.to : edge.from;
                // A connection a source reported cannot be edited here: no dead buttons, a note names the source instead.
                let caption = model.edgeCaption(edge);
                const sourceKind = edge.source && typeof edge.source === 'object' ? String(edge.source.kind || '') : '';
                if (sourceKind) {
                    const known = SOURCE_NAMES.includes(sourceKind);
                    const note = model.fill(t('desktop.infra_edge_reported_by'), { source: known ? t(`desktop.infra_source_${sourceKind}`) : sourceKind });
                    caption = [caption, note].filter(Boolean).join(' · ');
                }
                const buttons = write && !sourceKind
                    ? `<button type="button" data-edit-edge="${esc(edge.id)}" aria-label="${esc(t('desktop.infra_edit_connection'))}" title="${esc(t('desktop.infra_edit_connection'))}">✎</button>`
                    + `<button type="button" data-remove-edge="${esc(edge.id)}" aria-label="${esc(t('desktop.infra_remove_connection'))}" title="${esc(t('desktop.infra_remove_connection'))}">${app.glyph('close')}</button>`
                    : '';
                return `<div class="infra-relation-row" data-edge-row="${esc(edge.id)}"><span class="infra-edge-kind" data-edge-kind="${esc(edge.kind)}" aria-hidden="true"></span><span class="infra-relation-kind">${esc(edgeLabel(edge, node.id))}</span>`
                    + `<button type="button" class="infra-relation-other" data-inspector-select="${esc(otherId)}">${esc(store.nodeById(otherId)?.name || otherId)}</button>${caption ? `<small class="infra-relation-caption">${esc(caption)}</small>` : ''}${buttons}</div>`;
            }).join('');
            const toolbar = write ? `<div class="infra-relation-toolbar"><button type="button" class="infra-button" data-action="add-edge" aria-haspopup="dialog">${app.glyph('plus')}${esc(t('desktop.infra_add_connection'))}</button></div>` : '';
            // The tab already names the panel: the heading stays for assistive technology only.
            return `<h3 class="infra-sr-only">${esc(t('desktop.infra_tab_connections'))} ${edges.length}</h3>${toolbar}${rows || `<p class="infra-hint">${esc(t('desktop.infra_no_connections'))}</p>`}`;
        }

        // ---- the asset ---------------------------------------------------------------------------------
        function nodeMarkup(node, draft) {
            const dirty = state.dirty.has(node.id), write = canWrite();
            const stateName = sourceState(node), crumbs = model.placePath(node.id, store.nodeById);
            const powerKey = node.power ? model.powerLabel(node.power) : '';
            const live = model.liveState(node);
            const livePill = live ? `<span class="infra-live-pill" data-live="${esc(live)}"><i></i>${esc(t(model.liveLabelKey(live)))}</span>` : '';
            const link = (node.app_links || []).find(item => model.APP_LINK_TARGETS.has(item.app));
            // One compact button on one line: the short label of the target app, the full sentence as title and aria-label.
            const full = link ? model.fill(t('desktop.infra_open_in_app'), { app: appLabel(link.app) }) : '';
            const quick = link
                ? `<button type="button" class="infra-button infra-button-primary" data-action="open-first-app" data-open-app="${esc(link.app)}" data-target-id="${esc(link.target_id)}" title="${esc(full)}" aria-label="${esc(full)}"><span>${esc(appLabel(link.app))}</span></button>`
                : write ? `<button type="button" class="infra-button" data-action="edit-details"><span>${esc(t('desktop.infra_edit'))}</span></button>` : '';
            const tab = (id, label, extra = '') => `<button type="button" role="tab" id="infra-tab-${S}-${id}" data-inspector-tab="${id}" aria-controls="infra-panel-${S}-${id}" aria-selected="${activeTab === id}" tabindex="${activeTab === id ? 0 : -1}">${esc(label)}${extra}</button>`;
            const count = value => (value ? `<span class="infra-tab-count">${value}</span>` : '');
            const panel = (id, content) => `<div class="infra-tabpanel" role="tabpanel" id="infra-panel-${S}-${id}" aria-labelledby="infra-tab-${S}-${id}" data-inspector-panel="${id}"${activeTab === id ? '' : ' hidden'}>${content}</div>`;
            const name = write
                ? `<button type="button" class="infra-head-name" data-action="rename" title="${esc(t('desktop.infra_rename'))}">${esc(draft.name)}</button>`
                : `<span class="infra-head-name">${esc(node.name)}</span>`;
            return `<div class="infra-inspector-top" data-kind="${esc(node.kind)}">
  <header class="infra-inspector-head" data-inspector-head>
    <button type="button" class="infra-head-icon" data-action="pick-icon" aria-haspopup="dialog" aria-label="${esc(t('desktop.infra_change_icon'))}" title="${esc(t('desktop.infra_change_icon'))}"${write ? '' : ' disabled'}>${app.icon(draft.icon)}</button>
    <div class="infra-head-main">
      <h2 class="infra-head-title" id="${titleId}">${name}</h2>
      <input class="infra-head-name-input" data-node-field="name" form="infra-detail-form-${S}" value="${esc(draft.name)}" maxlength="120" required autocomplete="off" aria-label="${esc(t('desktop.infra_name'))}" hidden${write ? '' : ' readonly'}>
      <div class="infra-head-meta"><span>${esc(app.labelForKind(node.kind))}</span>${crumbs.length ? `<nav class="infra-crumbs" aria-label="${esc(t('desktop.infra_place'))}"><ol>${crumbs.map(crumb => `<li><button type="button" data-crumb="${esc(crumb.id)}">${esc(crumb.name)}</button></li>`).join('')}</ol></nav>` : ''}</div>
      <p class="infra-head-pills"><span class="infra-state-pill" data-state="${esc(stateName)}"><i></i>${esc(app.statusLabel(stateName))}</span>${livePill}${powerKey ? `<span class="infra-power-pill" data-power="${esc(node.power)}">${esc(t(powerKey))}</span>` : ''}</p>
      ${quick ? `<div class="infra-head-cta">${quick}</div>` : ''}
    </div>
    <div class="infra-head-actions"><button type="button" class="infra-cmd-icon" data-action="node-more" aria-haspopup="menu" aria-label="${esc(t('desktop.infra_more_actions'))}" title="${esc(t('desktop.infra_more_actions'))}">${app.glyph('more')}</button>${closeButton()}</div>
  </header>
  <div class="infra-tabs" role="tablist" aria-label="${esc(t('desktop.infra_inspector_tabs'))}" data-inspector-tabs>
    ${tab('overview', t('desktop.infra_tab_overview'))}${tab('details', t('desktop.infra_tab_details'), `<i class="infra-tab-dirty" aria-hidden="true"${dirty ? '' : ' hidden'}></i><span class="infra-sr-only" data-dirty-label${dirty ? '' : ' hidden'}>, ${esc(t('desktop.infra_unsaved_changes'))}</span>`)}${tab('sources', t('desktop.infra_tab_sources'), count((node.sources || []).length))}${tab('connections', t('desktop.infra_tab_connections'), count(edgesOf(node).length))}
  </div>
</div>
${forms.nodeReviewMarkup(node.id)}
${panel('overview', overviewHTML(node))}${panel('details', forms.detailsHTML(node, draft))}${panel('sources', forms.sourcesHTML(node))}${panel('connections', connectionsHTML(node))}`;
        }
        function paintNode(node) {
            const draft = state.drafts.get(node.id) || clone(node);
            state.drafts.set(node.id, draft);
            const same = lastId === node.id;
            if (!same) activeTab = pendingTab || model.inspectorTab(prefs, node.kind);
            else if (pendingTab) activeTab = pendingTab;
            pendingTab = '';
            lastId = node.id; renaming = false; edgeDirty = false;
            const scrollTop = same ? el.scrollTop : 0;
            // The repaint drops focus to the document: remember the control and give it back afterwards.
            const focus = same ? pendingFocus || focusKey(document.activeElement) : null;
            pendingFocus = null;
            forms.detach();
            show();
            el.innerHTML = nodeMarkup(node, draft);
            el.setAttribute('aria-labelledby', titleId);
            showTab(activeTab);
            forms.afterPaint(node);
            el.scrollTop = scrollTop;
            if (focus) {
                const target = focus.selector && el.querySelector(focus.selector);
                if (target && target.offsetParent !== null && !target.disabled) target.focus({ preventScroll: true });
                else $('[data-inspector-tab][aria-selected="true"]')?.focus({ preventScroll: true });
            }
        }

        // ---- connection and selection ---------------------------------------------------------------------
        function paintEdge(edge) {
            const nameOf = id => store.nodeById(id)?.name || id;
            forms.detach();
            lastId = ''; renaming = false; edgeDirty = false;
            show();
            const editable = canWrite() && !edge.source;
            const ends = `<dt>${esc(t('desktop.infra_relation_from'))}</dt><dd>${selectButton(edge.from)}</dd><dt>${esc(t('desktop.infra_relation_to'))}</dt><dd>${selectButton(edge.to)}</dd>`;
            let body;
            if (!editable) {
                const row = (label, value) => `<dt>${esc(label)}</dt><dd>${esc(value || '—')}</dd>`;
                body = `<dl class="infra-facts">${ends}${row(t('desktop.infra_connection_kind'), t(`desktop.infra_edge_${edge.kind}`))}${row(t('desktop.infra_relation_from_port'), edge.from_port)}${row(t('desktop.infra_relation_to_port'), edge.to_port)}${row(t('desktop.infra_relation_speed'), edge.speed)}${row(t('desktop.infra_relation_label'), edge.label)}</dl>${edge.source ? `<p class="infra-hint">${esc(t('desktop.infra_connection_readonly'))}</p>` : ''}`;
            } else {
                const input = (name, label, max) => `<label class="infra-field"><span>${esc(label)}</span><input name="${name}" value="${esc(edge[name] || '')}" maxlength="${max}" autocomplete="off"></label>`;
                body = `<form class="infra-edge-inline" data-edge-inline-form="${esc(edge.id)}"><dl class="infra-facts">${ends}</dl>`
                    + `<label class="infra-field"><span>${esc(t('desktop.infra_connection_kind'))}</span><select name="kind">${EDGE_KINDS.map(kind => `<option value="${kind}" ${edge.kind === kind ? 'selected' : ''}>${esc(t(`desktop.infra_edge_${kind}`))}</option>`).join('')}</select></label>`
                    + `<div class="infra-field-row">${input('from_port', t('desktop.infra_relation_from_port'), 64)}${input('to_port', t('desktop.infra_relation_to_port'), 64)}</div><div class="infra-field-row">${input('speed', t('desktop.infra_relation_speed'), 64)}${input('label', t('desktop.infra_relation_label'), 128)}</div>`
                    + `<p class="infra-connect-error" data-edge-error role="alert" hidden></p><div class="infra-form-actions"><button type="button" class="infra-button infra-danger-button" data-action="delete-edge">${esc(t('desktop.infra_remove_connection'))}</button><span class="infra-flex-spacer"></span><button type="button" class="infra-button" data-action="cancel-edge" disabled>${esc(t('desktop.cancel'))}</button><button type="submit" class="infra-button infra-button-primary" data-action="save-edge" disabled>${esc(t('desktop.save'))}</button></div></form>`;
            }
            el.innerHTML = `<div class="infra-detail-head"><div class="infra-detail-icon">${app.glyph('connect')}</div><div><p class="infra-detail-kind">${esc(t('desktop.infra_connection'))} · ${esc(t(`desktop.infra_edge_${edge.kind}`))}</p><h2 id="${titleId}">${esc(nameOf(edge.from))} → ${esc(nameOf(edge.to))}</h2></div>${closeButton()}</div><div class="infra-form-section">${body}</div>`;
            el.setAttribute('aria-labelledby', titleId);
        }
        function paintSelection() {
            const nodes = [...state.selection].map(id => store.nodeById(id)).filter(Boolean);
            const counts = new Map();
            for (const node of nodes) counts.set(node.kind, (counts.get(node.kind) || 0) + 1);
            forms.detach();
            lastId = ''; renaming = false; edgeDirty = false;
            show();
            const names = nodes.slice(0, LIST_LIMIT).map(node => `<li><button type="button" data-inspector-select="${esc(node.id)}">${app.icon(node.icon)}<span>${esc(node.name)}</span></button></li>`).join('');
            const rest = nodes.length > LIST_LIMIT ? `<li class="infra-hint">${esc(model.fill(t('desktop.infra_more_guests'), { count: nodes.length - LIST_LIMIT }))}</li>` : '';
            // Arrange in Topology hands the cards back to the automatic layout: it needs a server that can unplace.
            const arrangeAttrs = state.view === 'table' ? ' disabled' : app.mapView() === 'topology' && !app.gestures.canArrangeTopology() ? ` disabled${store.features.unplace ? '' : ` title="${esc(t('desktop.infra_arrange_unsupported'))}"`}` : '';
            const actions = canWrite()
                ? `<div class="infra-form-actions infra-selection-actions"><button type="button" class="infra-button" data-action="move-selection" aria-haspopup="dialog">${esc(t('desktop.infra_move_to_place'))}</button><button type="button" class="infra-button" data-action="arrange-selection"${arrangeAttrs}>${esc(t(app.arrangeLabelKey()))}</button><button type="button" class="infra-button infra-danger-button" data-action="delete-selection">${esc(t('desktop.infra_remove_node'))}</button><button type="button" class="infra-button infra-danger-button" data-action="ignore-selection">${esc(t('desktop.infra_remove_and_ignore'))}</button></div>`
                : '';
            el.innerHTML = `<div class="infra-detail-head"><div class="infra-detail-icon">${app.glyph('select')}</div><div><h2 id="${titleId}">${esc(model.fill(t('desktop.infra_selected_count'), { count: nodes.length }))}</h2></div>${closeButton()}</div>`
                + `<div class="infra-form-section"><ul class="infra-selection-kinds">${[...counts].map(([kind, count]) => `<li>${esc(app.labelForKind(kind))} <b>${count}</b></li>`).join('')}</ul><ul class="infra-selection-names">${names}${rest}</ul>${actions}</div>`;
            el.setAttribute('aria-labelledby', titleId);
        }
        function paint() {
            if (state.disposed || !root.isConnected) return;
            // Leaving a dirty draft behind is said once per asset.
            const leaving = lastId, next = state.view === 'table' && state.selectionOrigin === 'check' ? '' : state.selected;
            for (const id of draftNoticed) if (!state.dirty.has(id) && !state.sourceDirty.has(id)) draftNoticed.delete(id);
            if (leaving && leaving !== next && (state.dirty.has(leaving) || state.sourceDirty.has(leaving)) && !draftNoticed.has(leaving) && store.nodeById(leaving)) {
                draftNoticed.add(leaving);
                app.toast(model.fill(t('desktop.infra_draft_kept'), { name: state.drafts.get(leaving)?.name || store.nodeById(leaving).name }), 'info');
            }
            // In the List view the checkboxes build a selection for the bulk bar; they never open the panel over it.
            if (state.view === 'table' && state.selectionOrigin === 'check') { hide(); return; }
            const node = store.nodeById(state.selected);
            if (node) { paintNode(node); return; }
            const edge = state.selectedEdge ? state.edges.find(item => item.id === state.selectedEdge) : null;
            if (edge) { paintEdge(edge); return; }
            if (state.selection.size > 1) { paintSelection(); return; }
            hide();
        }

        // ---- tabs, header and rename --------------------------------------------------------------------------
        // The tab strip has no scrollbar; a fade at a cut edge marks the overflow (data-fade-start / data-fade-end).
        function paintTabFade() {
            const tabs = $('[data-inspector-tabs]');
            if (!tabs) return;
            const max = tabs.scrollWidth - tabs.clientWidth;
            tabs.toggleAttribute('data-fade-start', tabs.scrollLeft > 1);
            tabs.toggleAttribute('data-fade-end', tabs.scrollLeft < max - 1);
        }
        function showTab(tab, { focus = false, persist = false } = {}) {
            if (!TABS.includes(tab) || !$('[data-inspector-tab]')) return;
            activeTab = tab;
            for (const button of el.querySelectorAll('[data-inspector-tab]')) {
                const on = button.dataset.inspectorTab === tab;
                button.setAttribute('aria-selected', String(on));
                button.tabIndex = on ? 0 : -1;
                if (on) {
                    if (focus) button.focus({ preventScroll: true });
                    const tabs = button.parentElement, bounds = tabs.getBoundingClientRect(), rect = button.getBoundingClientRect();
                    // Keep 24 px of margin so the fade at a cut edge never covers the active tab.
                    const pad = 24;
                    if (rect.left < bounds.left + pad) tabs.scrollLeft -= bounds.left + pad - rect.left;
                    else if (rect.right > bounds.right - pad) tabs.scrollLeft += rect.right - (bounds.right - pad);
                }
            }
            paintTabFade();
            for (const panel of el.querySelectorAll('[data-inspector-panel]')) panel.hidden = panel.dataset.inspectorPanel !== tab;
            const node = store.nodeById(state.selected);
            if (persist && node) { prefs = model.withInspectorTab(prefs, node.kind, tab); app.writeStorage(PREFS_KEY, JSON.stringify(prefs)); }
        }
        // open selects id and shows tab; an asset that is already the only selection just switches tab.
        function open(id, tab = '') {
            if (!store.nodeById(id)) return;
            if (state.selected === id && state.selection.size === 1 && !el.hidden) { if (tab) showTab(tab); return; }
            pendingTab = tab;
            store.select([id]);
            pendingTab = '';
        }
        function syncHead(draft) {
            const button = $('[data-action="rename"]'), icon = $('[data-action="pick-icon"] img');
            if (button && button.textContent !== draft.name) button.textContent = draft.name;
            if (icon) icon.src = model.iconURL(draft.icon);
        }
        function startRename() {
            const input = $('.infra-head-name-input'), button = $('[data-action="rename"]');
            if (!input || !button || !canWrite() || renaming) return;
            renameBefore = input.value; renaming = true;
            button.hidden = true; input.hidden = false;
            input.focus(); input.select();
        }
        function endRename({ focus = true } = {}) {
            if (!renaming) return;
            renaming = false;
            const input = $('.infra-head-name-input'), button = $('[data-action="rename"]');
            if (!input) return;
            if (!input.value.trim()) { input.value = renameBefore; input.dispatchEvent(new Event('input', { bubbles: true })); }
            input.hidden = true;
            if (button) { button.hidden = false; if (focus) button.focus(); }
        }
        function onRenameKey(event) {
            if (event.key === 'Enter') { event.preventDefault(); endRename(); }
            else if (event.key === 'Escape') {
                event.preventDefault(); event.stopPropagation();
                // Cancelling without typing must not mark the draft as changed.
                if (event.target.value !== renameBefore) {
                    event.target.value = renameBefore;
                    event.target.dispatchEvent(new Event('input', { bubbles: true }));
                }
                endRename();
            }
        }
        function onTabKey(event) {
            const index = TABS.indexOf(event.target.dataset.inspectorTab);
            const next = { ArrowRight: (index + 1) % TABS.length, ArrowLeft: (index - 1 + TABS.length) % TABS.length, Home: 0, End: TABS.length - 1 }[event.key];
            if (next === undefined) return;
            event.preventDefault();
            showTab(TABS[next], { focus: true, persist: true });
        }

        // ---- connection form -----------------------------------------------------------------------------------
        function focusEdgeForm() { $('.infra-edge-inline [name="kind"]')?.focus(); }
        function onEdgeInput(event) {
            const form = event.target.closest?.('[data-edge-inline-form]');
            if (!form) return;
            edgeDirty = true;
            for (const button of form.querySelectorAll('[data-action="save-edge"], [data-action="cancel-edge"]')) button.disabled = false;
        }
        async function saveEdge(form) {
            const edge = state.edges.find(item => item.id === form.dataset.edgeInlineForm);
            const errorEl = form.querySelector('[data-edge-error]'), save = form.querySelector('[data-action="save-edge"]');
            const showError = text => { errorEl.textContent = text; errorEl.hidden = !text; };
            if (!edge || !canWrite() || edge.source) return;
            const fields = { from: edge.from, to: edge.to, kind: form.elements.kind.value };
            for (const key of ['label', 'from_port', 'to_port', 'speed']) { const value = form.elements[key].value.trim(); if (value) fields[key] = value; }
            if (state.edges.some(other => other.id !== edge.id && other.from === fields.from && other.to === fields.to && other.kind === fields.kind)) { showError(t('desktop.infra_error_edge_exists')); return; }
            const busyKey = `edge-${edge.id}`;
            if (state.busy.has(busyKey)) return;
            store.setBusy(busyKey, true); save.disabled = true; showError('');
            try {
                await store.updateEdge(edge.id, { id: edge.id, version: edge.version, ...fields });
                edgeDirty = false;
                app.announce(t('desktop.infra_connection_saved'));
                paint();
            } catch (error) {
                if (error?.name === 'AbortError') return;
                if (store.uncertainMutation(error)) await store.writeFailed(error);
                else {
                    if (error?.status === 409) await store.reloadSnapshot();
                    showError(store.messageFor(error));
                }
                save.disabled = false;
            } finally { store.setBusy(busyKey, false); }
        }

        // ---- panel behaviour ---------------------------------------------------------------------------------------
        function close({ restoreFocus = true } = {}) {
            const id = state.selected || state.selectedEdge || [...state.selection][0] || '';
            store.clearSelection();
            if (!restoreFocus || !id) return;
            const target = state.view === 'table'
                ? root.querySelector(`[data-row="${CSS.escape(id)}"] [data-select]`)
                : app.canvas?.cardEl(id) || app.canvas?.regionEl(id)?.querySelector('[data-region-header]') || app.canvas?.viewport;
            target?.focus({ preventScroll: true });
        }
        const copyText = async (button, value) => {
            if (!value) return;
            let ok = false;
            try {
                if (navigator.clipboard?.writeText) { await navigator.clipboard.writeText(value); ok = true; }
            } catch (_) { /* clipboard may be blocked in headless/non-secure contexts */ }
            if (!ok) { app.toast(t('desktop.infra_copy_failed'), 'error'); return; }
            if (!button.isConnected) return;
            button.dataset.copied = 'true';
            button.innerHTML = app.glyph('check');
            const timer = setTimeout(() => {
                copyTimers.delete(timer);
                if (!button.isConnected) return;
                delete button.dataset.copied;
                button.innerHTML = app.glyph('copy');
            }, 1400);
            copyTimers.add(timer);
            app.toast(t('desktop.copied'), 'success');
        };
        const selectedIds = () => [...state.selection].filter(id => store.nodeById(id));
        const onClick = event => {
            if (forms.onClick(event)) return;
            const target = event.target;
            const copy = target.closest('[data-copy-value]');
            if (copy) { void copyText(copy, copy.dataset.copyValue); return; }
            const jumpTab = target.closest('[data-jump-tab]');
            if (jumpTab) { showTab(jumpTab.dataset.jumpTab, { focus: true, persist: true }); return; }
            const tab = target.closest('[data-inspector-tab]');
            if (tab) { showTab(tab.dataset.inspectorTab, { persist: true }); return; }
            const jump = target.closest('[data-crumb], [data-inspector-select]');
            if (jump) {
                const id = jump.dataset.crumb || jump.dataset.inspectorSelect;
                store.select([id]);
                app.canvas?.ensureVisible(id);
                return;
            }
            const openApp = target.closest('[data-open-app]');
            if (openApp) { app.openAppLink({ app: openApp.dataset.openApp, target_id: openApp.dataset.targetId }); return; }
            const more = target.closest('[data-guest-more]');
            if (more) { app.library?.reveal(more.dataset.guestMore); return; }
            const editEdge = target.closest('[data-edit-edge]');
            if (editEdge) { if (!editEdge.disabled) app.dialogs.openEdgeDialog(editEdge.dataset.editEdge); return; }
            const removeEdge = target.closest('[data-remove-edge]');
            if (removeEdge) { if (!removeEdge.disabled) void app.dialogs.removeEdge(removeEdge.dataset.removeEdge); return; }
            const button = target.closest('[data-action]');
            if (!button || button.disabled) return;
            const id = state.selected, ids = selectedIds();
            switch (button.dataset.action) {
                case 'close-inspector': close(); break;
                case 'rename': startRename(); break;
                case 'pick-icon': forms.openIconPicker(button); break;
                case 'edit-details': showTab('details', { persist: true }); $('[data-node-field="kind"]')?.focus(); break;
                case 'node-more': app.menus.openNodeMenu(id, button); break;
                case 'place-selected': void app.gestures.placeAtCenter([id], { moveMode: event.detail === 0 }); break;
                case 'locate-selected':
                    if (state.view === 'table') app.ensureMapView();
                    app.canvas?.renderNow();
                    app.canvas?.centerOn(id, { animate: true });
                    break;
                case 'add-edge': app.picker.openAssetPicker({ anchor: button, label: t('desktop.infra_connect_to'), accept: other => other.id !== id, onPick: option => app.dialogs.openEdgeDialog('', id, option.id) }); break;
                case 'delete-edge': void app.dialogs.removeEdge(state.selectedEdge); break;
                case 'cancel-edge': edgeDirty = false; paint(); focusEdgeForm(); break;
                case 'move-selection': app.picker.openPlacePicker({ anchor: button, exclude: ids, onPick: placeId => { void app.gestures.moveIntoPlace(ids, placeId); } }); break;
                case 'arrange-selection': void app.gestures.arrange(); break;
                case 'delete-selection': void app.dialogs.deleteNodes(ids); break;
                case 'ignore-selection': void app.dialogs.removeAndIgnore(ids); break;
                default: break;
            }
        };
        const onInput = event => {
            forms.onFieldEvent(event);
            forms.onSourceFieldEvent(event);
            onEdgeInput(event);
        };
        const onSubmit = event => {
            if (forms.onSubmit(event)) return;
            const form = event.target.closest?.('[data-edge-inline-form]');
            if (form) { event.preventDefault(); void saveEdge(form); }
        };
        const onKeyDown = event => {
            if (event.target.matches?.('[role="tab"]')) onTabKey(event);
            else if (event.target.matches?.('.infra-head-name-input')) onRenameKey(event);
        };
        const onFocusOut = event => {
            forms.onFieldBlur(event);
            if (event.target.matches?.('.infra-head-name-input')) endRename({ focus: false });
        };
        const onToggle = event => forms.onToggle(event);
        const onScroll = event => { if (event.target.matches?.('[data-inspector-tabs]')) paintTabFade(); };
        const tabResize = typeof ResizeObserver === 'function' ? new ResizeObserver(() => paintTabFade()) : null;
        tabResize?.observe(el);
        el.addEventListener('scroll', onScroll, true);
        el.addEventListener('click', onClick);
        el.addEventListener('input', onInput);
        el.addEventListener('change', onInput);
        el.addEventListener('submit', onSubmit);
        el.addEventListener('keydown', onKeyDown);
        el.addEventListener('focusout', onFocusOut);
        el.addEventListener('toggle', onToggle, true);

        // The panel is not repainted under the operator's hands: a dirty draft, a focused field or an open
        // picker marks it stale instead, and focus leaving flushes it.
        const busyEditing = () => {
            if (state.selected && (state.dirty.has(state.selected) || state.sourceDirty.has(state.selected))) return true;
            return el.contains(document.activeElement) || app.picker.isOpen() || forms.isBusy() || edgeDirty;
        };
        function paintIfIdle() {
            if (busyEditing()) { stale = true; return; }
            stale = false;
            paint();
        }
        function flushStale() {
            setTimeout(() => {
                if (state.disposed || !stale || busyEditing()) return;
                stale = false;
                paint();
            }, 0);
        }
        function onBusy(key) { forms.onBusy(key); }
        // saveDraft (Ctrl S): the source form on the Sources tab, else the Details draft.
        function saveDraft() {
            if (!state.selected) return;
            const sources = $('[data-inspector-panel="sources"]');
            if (sources && !sources.hidden && state.sourceDirty.has(state.selected)) { $('[data-source-form]')?.requestSubmit(); return; }
            if (state.dirty.has(state.selected)) forms.submitDetails();
        }
        function focusName() {
            showTab('details');
            startRename();
        }
        function dispose() {
            clearCopyTimers();
            el.removeEventListener('click', onClick);
            el.removeEventListener('input', onInput);
            el.removeEventListener('change', onInput);
            el.removeEventListener('submit', onSubmit);
            el.removeEventListener('keydown', onKeyDown);
            el.removeEventListener('focusout', onFocusOut);
            el.removeEventListener('toggle', onToggle, true);
            el.removeEventListener('scroll', onScroll, true);
            tabResize?.disconnect();
            forms.dispose();
            el.remove();
        }

        return { el, paint, paintIfIdle, flushStale, onBusy, saveDraft, focusName, startRename, close, open, showTab, syncHead, focusEdgeForm, rememberFocus, dispose };
    }

    window.InfraAppInspector = { create };
})();
