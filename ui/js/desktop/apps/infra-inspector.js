// Infra inspector: the right-hand panel for the selected asset (identity,
// profile, placement, sources, relations, drafts and conflict review), the
// selected connection and a multi-selection summary. The panel is hidden while
// nothing is selected; infra.js calls paint() on selection events and
// paintIfIdle() on data events, and flushStale() when focus leaves the panel.
(function () {
    'use strict';

    const KINDS = ['device', 'vm', 'container', 'service', 'network', 'site', 'room', 'rack', 'group'];
    const SOURCE_KINDS = ['registry', 'docker', 'proxmox', 'fritzbox'];
    const SOURCE_EDIT_FIELDS = { registry: ['name', 'type', 'protocol', 'ip_address', 'port', 'username', 'credential_id', 'description', 'tags', 'mac_address'], docker: ['name'], proxmox: ['name', 'hostname', 'description', 'tags'], fritzbox: ['friendly_name'] };
    const PROFILE_FIELDS = ['role', 'vendor', 'model', 'serial', 'os', 'firmware', 'cpu', 'ram', 'storage', 'rack_position', 'ip_address', 'mac_address'];
    const APP_LINK_TARGETS = new Set(['quick-connect', 'network-cameras', 'software-store', 'meshcore']);
    const appContext = link => ({
        'quick-connect': { device_id: link.target_id },
        'network-cameras': { camera_id: link.target_id },
        'software-store': { app_id: link.target_id },
        meshcore: { conversation_id: link.target_id },
    }[link.app] || { target_id: link.target_id });
    const sourceEditPending = result => result?.pending === true || result?.fields?.pending === true;
    const sourceFieldValue = (source, key, value) => {
        if (key !== 'tags') return key === 'port' ? Number(value) : value;
        if (source === 'registry') return String(value).split(',').map(item => item.trim()).filter(Boolean);
        if (source === 'proxmox') return String(value).split(/[;,]/).map(item => item.trim()).filter(Boolean).join('; ');
        return value;
    };

    function create(app) {
        const { store, state, t, esc, model, root, ctx } = app;
        const { clone, iconId, iconLabelKey, sourceActions, sourceIdentity, sourceState, validCIDR, validHTTPLink } = model;
        const ICON_LIST = model.iconIds();
        const S = app.safeId;
        const canWrite = () => store.canWrite();
        const canEditSource = () => store.canEditSource();
        const holder = document.createElement('div');
        holder.innerHTML = `<aside class="infra-inspector" data-inspector aria-label="${esc(t('desktop.infra_details'))}" hidden></aside>`;
        const el = holder.firstElementChild;
        const $ = selector => el.querySelector(selector);
        let stale = false;

        const show = () => { el.hidden = false; root.dataset.inspectorOpen = 'true'; };
        const hide = () => { el.hidden = true; el.innerHTML = ''; root.dataset.inspectorOpen = 'false'; };
        const readonlyAttr = () => (!canWrite() ? 'readonly' : '');
        const disabledAttr = () => (!canWrite() ? 'disabled' : '');

        // ---- review markup -------------------------------------------------------
        const reviewItem = (item, key) => model.listEntryText(key, item, { targets: state.appTargets, t });
        const reviewValue = (value, key = '') => {
            if (key === 'credential_id') {
                const credential = state.credentials.find(item => item.id === value);
                if (credential) return credential.name;
            }
            if (Array.isArray(value)) return value.map(item => reviewItem(item, key)).filter(Boolean).join(key === 'tags' ? '; ' : ', ');
            if (value && typeof value === 'object') return JSON.stringify(value).slice(0, 180);
            return value == null || value === '' ? t('desktop.infra_unknown') : String(value);
        };
        const nodeReviewRows = (latest, draft) => {
            const rows = [];
            const add = (key, before, after, prefix = '') => {
                if (JSON.stringify(before) === JSON.stringify(after)) return;
                const labels = { name: t('desktop.infra_name'), kind: t('desktop.infra_type'), icon: t('desktop.infra_icon'), parent_id: t('desktop.infra_group') };
                const display = value => (key === 'parent_id' ? (store.nodeById(value)?.name || (value ? String(value) : t('desktop.infra_no_group'))) : key === 'icon' ? t(iconLabelKey(value)) : reviewValue(value, key));
                rows.push({ label: prefix ? t(`desktop.infra_field_${key}`) : labels[key] || key, current: display(before), draft: display(after) });
            };
            for (const key of ['name', 'kind', 'icon', 'parent_id']) add(key, latest?.[key], draft?.[key]);
            const profileKeys = new Set([...Object.keys(latest?.profile || {}), ...Object.keys(draft?.profile || {})]);
            for (const key of profileKeys) add(key, latest?.profile?.[key], draft?.profile?.[key], 'profile');
            return rows;
        };
        const reviewDiffMarkup = (rows, currentLabel) => (rows.length
            ? `<div class="infra-review-diff">${rows.map(row => `<div class="infra-review-row"><strong>${esc(row.label)}</strong><div><small>${esc(currentLabel)}</small><span>${esc(row.current)}</span></div><div><small>${esc(t('desktop.infra_your_draft'))}</small><span>${esc(row.draft)}</span></div></div>`).join('')}</div>`
            : `<p class="infra-hint">${esc(t('desktop.infra_no_field_differences'))}</p>`);
        const sourceReviewMarkup = id => {
            const review = state.sourceReviews.get(id);
            if (!review) return '';
            const rows = Object.keys(review.submitted).map(key => ({ label: t(`desktop.infra_field_${key}`), current: reviewValue(review.latest.fields?.[key], key), draft: reviewValue(review.submitted[key], key) })).filter(row => row.current !== row.draft);
            return `<section class="infra-conflict-review" role="region" aria-label="${esc(t('desktop.infra_source_compare'))}"><strong>${esc(t('desktop.infra_source_compare'))}</strong>${reviewDiffMarkup(rows, t('desktop.infra_current_source'))}<button type="button" data-action="reapply-source-draft" class="infra-button-primary">${esc(t('desktop.infra_reapply_draft'))}</button></section>`;
        };
        const nodeReviewMarkup = id => {
            const review = state.nodeReviews.get(id);
            if (!review) return '';
            return `<section class="infra-conflict-review" role="region" aria-label="${esc(t('desktop.infra_node_compare'))}"><strong>${esc(t('desktop.infra_node_compare'))}</strong>${reviewDiffMarkup(nodeReviewRows(review.latest, state.drafts.get(id)), t('desktop.infra_current_inventory'))}<button type="button" data-action="reapply-node-draft" class="infra-button-primary">${esc(t('desktop.infra_reapply_draft'))}</button></section>`;
        };

        // ---- profile list editors --------------------------------------------------
        const inputFor = (family, index, key, value, type = 'text', attrs = '') => `<label class="infra-field"><span>${esc(t(`desktop.infra_field_${key}`))}</span><input type="${type}" data-list-family="${family}" data-index="${index}" data-key="${key}" value="${esc(value ?? '')}" ${attrs} ${readonlyAttr()}></label>`;
        const listEditor = (family, title, fields, rows, addKey, removeKey) => `<div class="infra-list-editor" data-list-editor="${family}"><div class="infra-list-heading"><span>${esc(t(`desktop.infra_${title}`))}</span><button type="button" data-list-add="${family}" ${disabledAttr()}>＋ ${esc(t(`desktop.infra_${addKey}`))}</button></div>${rows.length ? rows.map((row, index) => `<div class="infra-list-row" data-list-row="${family}" data-index="${index}"><div class="infra-fields-grid">${fields.map(([key, type, attrs]) => inputFor(family, index, key, row[key], type, attrs || '')).join('')}</div><button type="button" class="infra-list-remove" data-list-remove="${family}" data-index="${index}" aria-label="${esc(t(`desktop.infra_${removeKey}`))}" ${disabledAttr()}>×</button></div>`).join('') : `<p class="infra-hint">${esc(t('desktop.infra_list_empty'))}</p>`}</div>`;
        const customEditor = (nodeID, profile) => {
            const rows = state.customRows.get(nodeID) || Object.entries(profile.custom || {}).map(([key, value]) => ({ key, value }));
            return `<div class="infra-list-editor" data-list-editor="custom"><div class="infra-list-heading"><span>${esc(t('desktop.infra_custom_fields'))}</span><button type="button" data-list-add="custom" ${disabledAttr()}>＋ ${esc(t('desktop.infra_add_custom_field'))}</button></div>${rows.length ? rows.map((row, index) => `<div class="infra-list-row" data-list-row="custom" data-index="${index}"><div class="infra-fields-grid">${inputFor('custom', index, 'key', row.key, 'text', 'required maxlength="80" pattern="[A-Za-z0-9_.-]+"')}<label class="infra-field"><span>${esc(t('desktop.infra_custom_value'))}</span><input data-list-family="custom" data-index="${index}" data-key="value" value="${esc(row.value ?? '')}" maxlength="2048" ${readonlyAttr()}></label></div><button type="button" class="infra-list-remove" data-list-remove="custom" data-index="${index}" aria-label="${esc(t('desktop.infra_remove_custom_field'))}" ${disabledAttr()}>×</button></div>`).join('') : `<p class="infra-hint">${esc(t('desktop.infra_list_empty'))}</p>`}</div>`;
        };
        const appBindingEditor = profile => {
            const bindings = profile.app_bindings || [];
            const selected = new Set(bindings.map(item => `${item.app}\x00${item.target_id}`));
            const available = new Set(state.appTargets.map(target => `${target.app}\x00${target.target_id}`));
            const unavailable = bindings.filter(item => !available.has(`${item.app}\x00${item.target_id}`));
            const targetOption = (target, missing = false) => `<label class="infra-target-option ${missing ? 'is-unavailable' : ''}"><input type="checkbox" data-app-target data-app="${esc(target.app)}" data-target-id="${esc(target.target_id)}" ${selected.has(`${target.app}\x00${target.target_id}`) ? 'checked' : ''} ${disabledAttr()}><span><strong>${esc(missing ? `${target.target_id} · ${t('desktop.infra_target_unavailable')}` : target.label || target.target_id)}</strong><small>${esc(t(`desktop.infra_app_${target.app.replaceAll('-', '_')}`))}</small></span></label>`;
            const empty = !state.appTargets.length ? `<p class="infra-hint">${esc(t(state.targetsError ? 'desktop.infra_targets_error' : state.targetsLoaded ? 'desktop.infra_no_app_targets' : 'desktop.infra_targets_loading'))}</p>` : '';
            return `<div class="infra-list-editor" data-list-editor="app-bindings"><div class="infra-list-heading"><span>${esc(t('desktop.infra_application_links'))}</span><span>${esc(t('desktop.infra_explicit_links_only'))}</span></div>${empty}<div class="infra-target-list">${state.appTargets.map(target => targetOption(target)).join('')}${unavailable.map(target => targetOption(target, true)).join('')}</div>${state.targetsError ? `<button type="button" data-action="reload-targets">${esc(t('desktop.infra_retry_targets'))}</button>` : ''}</div>`;
        };
        const projectedAppLinks = node => (node.app_links || []).filter(link => APP_LINK_TARGETS.has(link.app)).map(link => `<button type="button" class="infra-app-link" data-open-app="${esc(link.app)}" data-target-id="${esc(link.target_id)}">${esc(t(`desktop.infra_app_${link.app.replaceAll('-', '_')}`))}<span aria-hidden="true">↗</span></button>`).join('');

        // ---- sources ----------------------------------------------------------------
        const sourceFieldMarkup = ([key, value], editable, sourceKind) => {
            const label = esc(t(`desktop.infra_field_${key}`));
            const locked = editable && canEditSource() ? '' : 'disabled';
            if (key === 'protocol') return `<label class="infra-field"><span>${label}</span><select data-source-field="${esc(key)}" ${locked}>${['none', 'ssh', 'vnc'].map(protocol => `<option value="${protocol}" ${value === protocol ? 'selected' : ''}>${esc(t(`desktop.infra_protocol_${protocol}`))}</option>`).join('')}</select></label>`;
            if (key === 'credential_id') {
                const current = String(value || '');
                const found = state.credentials.some(credential => credential.id === current);
                const extra = current && !found ? `<option value="${esc(current)}" selected>${esc(t('desktop.infra_credential_unlisted'))} · ${esc(current)}</option>` : '';
                const hint = state.credentialsError ? `<small class="infra-field-hint">${esc(t('desktop.infra_credentials_load_failed'))}</small>` : '';
                return `<label class="infra-field"><span>${label}</span><select data-source-field="${esc(key)}" ${locked}><option value="">${esc(t('desktop.infra_no_credential'))}</option>${extra}${state.credentials.map(credential => `<option value="${esc(credential.id)}" ${current === credential.id ? 'selected' : ''}>${esc(credential.name)}${credential.type ? ` · ${esc(credential.type)}` : ''}</option>`).join('')}</select>${hint}</label>`;
            }
            const fieldValue = Array.isArray(value) ? value.join(', ') : value ?? '';
            const placeholder = key === 'tags' && sourceKind === 'proxmox' ? ` placeholder="${esc(t('desktop.infra_pve_tags_hint'))}"` : '';
            return `<label class="infra-field"><span>${label}</span><input data-source-field="${esc(key)}" type="${key === 'port' ? 'number' : 'text'}" value="${esc(fieldValue)}"${placeholder} ${editable && canEditSource() ? '' : 'readonly'} ${key === 'port' ? 'min="1" max="65535"' : ''}></label>`;
        };
        const sourceMarkup = (source, node) => {
            const actions = sourceActions(source.kind);
            const providerHost = source.kind === 'proxmox' && String(source.key || '').startsWith('node:');
            const blocked = actions.filter(action => !store.allowedAction(node, action, source) && store.actionReason(node, source, action));
            const reasons = [...new Set(blocked.map(action => store.actionReason(node, source, action)))];
            const reasonLabel = reasons.length ? t(`desktop.infra_reason_${reasons[0]}`) : '';
            const blockedLabel = blocked.map(action => t(`desktop.infra_action_${action}`)).join(', ');
            const powerKey = source.state === 'current' ? model.powerLabel(source.observed?.power) : '';
            const power = powerKey ? `<span class="infra-power-pill" data-power="${esc(source.observed.power)}">${esc(t(powerKey))}</span>` : '';
            return `<div class="infra-source-row" data-source-row><div class="infra-source-icon" data-source="${esc(source.kind)}">${esc(source.kind.slice(0, 1).toUpperCase())}</div><div class="infra-source-copy"><strong>${esc(t(`desktop.infra_source_${source.kind}`))}</strong><span>${esc(source.name || source.instance || source.key || '')}</span>${reasonLabel ? `<small class="infra-action-reason">${esc(blockedLabel)} · ${esc(reasonLabel)}</small>` : ''}</div>${power}<span class="infra-state-pill" data-state="${esc(source.state)}"><i></i>${esc(app.statusLabel(source.state))}</span><small>${esc(source.fetched_at ? new Date(source.fetched_at).toLocaleString() : t('desktop.infra_no_observation'))}</small>${canEditSource() && !providerHost ? `<button type="button" data-edit-source="${esc(source.kind)}">${esc(t('desktop.infra_edit_source'))}</button>` : ''}${actions.some(action => store.allowedAction(node, action, source)) ? `<button type="button" data-source-action="${esc(source.kind)}" data-source-identity="${esc(sourceIdentity(source))}">${esc(t('desktop.infra_actions'))}</button>` : ''}</div>`;
        };
        const sourceEditMarkup = (node, sourceEdit) => {
            const dirty = state.sourceDirty.has(node.id), blocked = state.sourceBlocked.has(node.id), pending = state.sourcePending.has(node.id), saving = state.busy.has(`source-${node.id}`);
            const fields = Object.entries(sourceEdit.fields || {}).filter(([key]) => SOURCE_EDIT_FIELDS[sourceEdit.source]?.includes(key)).map(entry => sourceFieldMarkup(entry, sourceEdit.editable, sourceEdit.source)).join('');
            return `<div class="infra-form-section infra-source-edit"><div class="infra-form-heading"><h3>${esc(t('desktop.infra_source_edit_title'))}</h3><span>${esc(t(`desktop.infra_source_${sourceEdit.source}`))}</span></div>${dirty ? `<div class="infra-draft-banner" role="status"><i></i>${esc(t('desktop.infra_source_draft'))}</div>` : ''}${pending && !dirty ? `<p class="infra-draft-banner" role="status"><i></i><span>${esc(t('desktop.infra_source_pending'))}</span></p>` : ''}<div data-source-form>${sourceReviewMarkup(node.id)}${fields}<p class="infra-hint" data-source-reason>${esc(t(`desktop.infra_reason_${sourceEdit.reason}`))}</p>${blocked ? `<p class="infra-draft-banner" role="status"><i></i><span>${esc(t(pending ? 'desktop.infra_source_pending_review' : 'desktop.infra_source_review_needed'))}</span></p>` : ''}<div class="infra-form-actions"><button type="button" data-action="close-source-edit">${esc(t('desktop.cancel'))}</button>${blocked ? `<button type="button" data-action="review-source">${esc(t(pending ? 'desktop.infra_review_pending_source' : 'desktop.infra_review_source'))}</button>` : ''}<button type="button" data-action="save-source" class="infra-button-primary" ${!sourceEdit.editable || !canEditSource() || !dirty || blocked || saving ? 'disabled' : ''}>${esc(saving ? t('desktop.infra_saving') : t('desktop.infra_save_source'))}</button></div></div></div>`;
        };

        // ---- node sections ---------------------------------------------------------------
        function placementSection(node) {
            const placement = store.effectivePlacement(node.id, app.mapView());
            const label = placement ? t('desktop.infra_position').replaceAll('{x}', Math.round(placement.x)).replaceAll('{y}', Math.round(placement.y)) : t('desktop.infra_unplaced');
            const canPlace = !placement && !store.guests().hostOf.has(node.id) && canWrite();
            return `<div class="infra-form-section"><div class="infra-form-heading"><h3>${esc(t('desktop.infra_placement'))}</h3><span>${esc(label)}</span></div>${canPlace ? `<button type="button" class="infra-button" data-action="place-selected">${esc(t('desktop.infra_place_on_map'))}</button>` : ''}<p class="infra-hint">${esc(t('desktop.infra_move_hint'))}</p></div>`;
        }
        function hostedSection(node) {
            const guests = (store.guests().guestsOf.get(node.id) || []).map(id => store.nodeById(id)).filter(Boolean);
            if (!guests.length) return '';
            return `<section class="infra-hosted-services"><div class="infra-form-heading"><h3>${esc(t('desktop.infra_hosted_services'))} <span>${guests.length}</span></h3></div><div class="infra-component-list">${guests.map(guest => `<button type="button" data-inspector-guest="${esc(guest.id)}">${app.icon(guest.icon)}<span>${esc(guest.name)}</span><small>${esc(app.labelForKind(guest.kind))}</small></button>`).join('')}</div></section>`;
        }
        function relationsSection(node) {
            const rows = state.edges.filter(edge => edge.from === node.id || edge.to === node.id).map(edge => {
                const other = store.nodeById(edge.from === node.id ? edge.to : edge.from);
                return `<div class="infra-relation-row"><span class="infra-edge-kind" data-edge-kind="${esc(edge.kind)}"></span><span>${esc(t(`desktop.infra_edge_${edge.kind}`))}</span><strong>${esc(other?.name || edge.to)}</strong><button type="button" data-edit-edge="${esc(edge.id)}" aria-label="${esc(t('desktop.infra_edit_relation'))}" ${!canWrite() || edge.source ? 'disabled' : ''}>✎</button></div>`;
            }).join('');
            return `<div class="infra-relations"><div class="infra-form-heading"><h3>${esc(t('desktop.infra_relations'))}</h3>${canWrite() ? `<button type="button" data-action="add-edge">＋ ${esc(t('desktop.infra_add_relation'))}</button>` : ''}</div>${rows || `<p class="infra-hint">${esc(t('desktop.infra_no_relations'))}</p>`}</div>`;
        }
        function nodeMarkup(node, draft) {
            const sources = node.sources || [];
            const dirty = state.dirty.has(node.id);
            const sourceEdit = state.sourceDrafts.get(node.id);
            const field = (name, label, value, type = 'text') => `<label class="infra-field"><span>${esc(t(`desktop.infra_field_${label}`))}</span><input data-profile="${esc(name)}" type="${type}" value="${esc(value || '')}" ${readonlyAttr()} autocomplete="off"></label>`;
            const sourcesSection = sources.length
                ? `<div class="infra-form-section"><div class="infra-form-heading"><h3>${esc(t('desktop.infra_sources'))}</h3><span>${sources.length}</span></div><div class="infra-source-list">${sources.map(source => sourceMarkup(source, node)).join('')}</div></div>`
                : `<div class="infra-form-section"><h3>${esc(t('desktop.infra_sources'))}</h3><p class="infra-hint">${esc(t('desktop.infra_manual_only'))}</p></div>`;
            const network = draft.kind === 'network' ? `<section class="infra-network-segment"><h4>${esc(t('desktop.infra_network_segment'))}</h4><div class="infra-field-row"><label class="infra-field"><span>${esc(t('desktop.infra_cidr'))}</span><input data-profile="cidr" type="text" value="${esc(draft.profile?.cidr || '')}" placeholder="192.168.10.0/24" ${readonlyAttr()}></label><label class="infra-field"><span>${esc(t('desktop.infra_vlan'))}</span><input data-profile="vlan" type="number" min="1" max="4094" value="${esc(draft.profile?.vlan || '')}" placeholder="${esc(t('desktop.infra_unknown'))}" ${readonlyAttr()}></label></div><p class="infra-hint">${esc(t('desktop.infra_cidr_hint'))}</p></section>` : '';
            const actions = canWrite()
                ? `<button type="button" data-action="delete-node" class="infra-danger-button">${esc(t('desktop.infra_remove_node'))}</button><span class="infra-flex-spacer"></span>${dirty ? `<button type="button" data-action="revert-node">${esc(t('desktop.infra_discard'))}</button>` : ''}<button type="submit" form="infra-detail-form-${S}" class="infra-button-primary" ${dirty ? '' : 'disabled'}>${esc(state.busy.has(`node-${node.id}`) ? t('desktop.infra_saving') : t('desktop.save'))}</button>`
                : `<span class="infra-readonly-note">${esc(t('desktop.infra_read_only'))}</span>`;
            return `<div class="infra-detail-head"><div class="infra-detail-icon">${app.icon(node.icon)}</div><div><p class="infra-detail-kind">${esc(app.labelForKind(node.kind))} · ${esc(t('desktop.infra_version').replace('{version}', node.version))}</p><h2>${esc(node.name)}</h2></div><button type="button" class="infra-detail-close" data-action="close-inspector" aria-label="${esc(t('desktop.close'))}" title="${esc(t('desktop.close'))}">×</button><button type="button" class="infra-detail-more" data-action="node-more" aria-label="${esc(t('desktop.infra_more_actions'))}">···</button></div>
              ${dirty ? `<div class="infra-draft-banner" role="status"><i></i>${esc(t('desktop.infra_draft_kept'))}</div>` : ''}${nodeReviewMarkup(node.id)}
              <form class="infra-detail-form" id="infra-detail-form-${S}" data-detail-form><div class="infra-form-section"><div class="infra-form-heading"><h3>${esc(t('desktop.infra_identity'))}</h3><span class="infra-state-pill" data-state="${esc(sourceState(node))}"><i></i>${esc(app.statusLabel(sourceState(node)))}</span></div><label class="infra-field"><span>${esc(t('desktop.infra_name'))}</span><input data-node-field="name" value="${esc(draft.name)}" ${readonlyAttr()} maxlength="120" required></label><div class="infra-field-row"><label class="infra-field"><span>${esc(t('desktop.infra_type'))}</span><select data-node-field="kind" ${disabledAttr()}>${KINDS.map(kind => `<option value="${kind}" ${draft.kind === kind ? 'selected' : ''}>${esc(app.labelForKind(kind))}</option>`).join('')}</select></label><label class="infra-field"><span>${esc(t('desktop.infra_icon'))}</span><select data-node-field="icon" ${disabledAttr()}>${ICON_LIST.map(key => `<option value="${key}" ${iconId(draft.icon) === key ? 'selected' : ''}>${esc(t(iconLabelKey(key)))}</option>`).join('')}</select></label></div></div>
              <div class="infra-form-section"><h3>${esc(t('desktop.infra_hardware'))}</h3><div class="infra-fields-grid">${PROFILE_FIELDS.map(name => field(name, name, draft.profile?.[name])).join('')}</div>${network}<label class="infra-field"><span>${esc(t('desktop.infra_tags'))}</span><input data-profile="tags" value="${esc((draft.profile?.tags || []).join(', '))}" placeholder="${esc(t('desktop.infra_tags_hint'))}" ${readonlyAttr()}></label><label class="infra-field"><span>${esc(t('desktop.infra_notes'))}</span><textarea data-profile="notes" rows="3" ${readonlyAttr()}>${esc(draft.profile?.notes || '')}</textarea></label>${listEditor('interfaces', 'interfaces', [['name', 'text', 'required maxlength="64"'], ['ip_address', 'text'], ['mac_address', 'text'], ['vlan', 'text'], ['speed', 'text']], draft.profile?.interfaces || [], 'add_interface', 'remove_interface')}${listEditor('links', 'links', [['label', 'text', 'required maxlength="128"'], ['url', 'url', 'placeholder="https://" required maxlength="2048"']], draft.profile?.links || [], 'add_link', 'remove_link')}${customEditor(node.id, draft.profile || {})}${appBindingEditor(draft.profile || {})}</div>
              ${placementSection(node)}
              ${sourcesSection}
              ${(node.app_links || []).length ? `<div class="infra-form-section"><h3>${esc(t('desktop.infra_open_in'))}</h3><div class="infra-app-links">${projectedAppLinks(node)}</div></div>` : ''}
              ${sourceEdit ? sourceEditMarkup(node, sourceEdit) : ''}
              <div class="infra-form-actions infra-detail-actions">${actions}</div></form>
              ${relationsSection(node)}
              ${hostedSection(node)}`;
        }
        // decorate adds the parts that are built with DOM calls: the service
        // component list and the group selector.
        function decorate(node, draft) {
            if (node.kind === 'service') {
                const logicalEdges = state.edges.filter(edge => ['runs_on', 'depends_on', 'powered_by'].includes(edge.kind) && (edge.from === node.id || edge.to === node.id));
                if (logicalEdges.length) {
                    const details = document.createElement('details');
                    details.className = 'infra-service-components';
                    const summary = document.createElement('summary');
                    summary.textContent = `${t('desktop.infra_service_components')} (${logicalEdges.length})`;
                    details.append(summary);
                    const list = document.createElement('div');
                    list.className = 'infra-component-list';
                    for (const edge of logicalEdges) {
                        const other = store.nodeById(edge.from === node.id ? edge.to : edge.from);
                        if (!other) continue;
                        const button = document.createElement('button');
                        button.type = 'button';
                        button.dataset.relatedNode = other.id;
                        button.textContent = `${t(`desktop.infra_edge_${edge.kind}`)} · ${other.name}`;
                        list.append(button);
                    }
                    details.append(list);
                    $('.infra-relations')?.before(details);
                    list.querySelectorAll('[data-related-node]').forEach(button => button.addEventListener('click', () => store.select([button.dataset.relatedNode])));
                }
            }
            const identity = $('.infra-detail-form .infra-form-section');
            if (!identity) return;
            const label = document.createElement('label');
            label.className = 'infra-field';
            label.innerHTML = `<span>${esc(t('desktop.infra_group'))}</span><select data-node-field="parent_id" ${disabledAttr()}><option value="">${esc(t('desktop.infra_no_group'))}</option></select>`;
            const select = label.querySelector('select'), descendants = new Set(), queue = [node.id];
            while (queue.length) {
                const current = queue.pop();
                for (const child of store.childrenOf(current)) if (!descendants.has(child.id)) { descendants.add(child.id); queue.push(child.id); }
            }
            state.nodes.filter(group => model.isContainer(group.kind) && group.id !== node.id && !descendants.has(group.id)).forEach(group => {
                const option = document.createElement('option');
                option.value = group.id;
                option.textContent = group.name;
                option.selected = group.id === draft.parent_id;
                select.append(option);
            });
            if (!draft.parent_id) select.value = '';
            identity.append(label);
        }
        function bindNode(node) {
            const form = $('[data-detail-form]');
            form?.addEventListener('input', updateDraft);
            form?.addEventListener('change', updateDraft);
            form?.addEventListener('submit', event => { event.preventDefault(); void saveNode(node.id); });
            el.querySelectorAll('[data-open-app]').forEach(button => button.addEventListener('click', () => ctx.openApp?.(button.dataset.openApp, appContext({ app: button.dataset.openApp, target_id: button.dataset.targetId }))));
            el.querySelectorAll('[data-edit-source]').forEach(button => button.addEventListener('click', () => void loadSourceEdit(node.id, button.dataset.editSource)));
            el.querySelectorAll('[data-source-action]').forEach(button => button.addEventListener('click', () => {
                const source = (store.nodeById(node.id)?.sources || []).find(item => sourceIdentity(item) === button.dataset.sourceIdentity);
                if (source) app.dialogs.openActionMenu(node.id, source);
            }));
            $('[data-action="save-source"]')?.addEventListener('click', () => void saveSourceEdit(node.id));
            $('[data-action="review-source"]')?.addEventListener('click', () => void reviewSourceEdit(node.id));
            $('[data-action="close-source-edit"]')?.addEventListener('click', () => {
                for (const set of [state.sourceDirty, state.sourceBlocked, state.sourcePending]) set.delete(node.id);
                state.sourceDrafts.delete(node.id); state.sourceReviews.delete(node.id);
                paint();
            });
            $('[data-action="reapply-source-draft"]')?.addEventListener('click', () => reapplySourceDraft(node.id));
            $('[data-action="reapply-node-draft"]')?.addEventListener('click', () => reapplyNodeDraft(node.id));
            $('[data-action="delete-node"]')?.addEventListener('click', () => void app.dialogs.deleteNode(node.id));
            $('[data-action="node-more"]')?.addEventListener('click', () => app.dialogs.openNodeActions(node.id));
            $('[data-action="add-edge"]')?.addEventListener('click', () => app.dialogs.openEdgeDialog('', node.id));
            $('[data-action="revert-node"]')?.addEventListener('click', () => {
                state.drafts.set(node.id, clone(node)); state.dirty.delete(node.id); state.nodeReviews.delete(node.id); state.customRows.delete(node.id);
                paint();
            });
            el.querySelectorAll('[data-edit-edge]').forEach(button => button.addEventListener('click', () => app.dialogs.openEdgeDialog(button.dataset.editEdge, node.id)));
            const updateSourceDraft = input => {
                const current = state.sourceDrafts.get(node.id);
                if (current) current.fields[input.dataset.sourceField] = input.value;
                state.sourceReviews.delete(node.id);
                state.sourceDirty.add(node.id);
                state.sourceRevision.set(node.id, (state.sourceRevision.get(node.id) || 0) + 1);
                const save = $('[data-action="save-source"]');
                if (save) save.disabled = !canEditSource() || state.sourceBlocked.has(node.id);
                if (!$('.infra-source-edit .infra-draft-banner')) {
                    const banner = document.createElement('div');
                    banner.className = 'infra-draft-banner';
                    banner.setAttribute('role', 'status');
                    banner.innerHTML = `<i></i>${esc(t('desktop.infra_source_draft'))}`;
                    $('.infra-source-edit .infra-form-heading')?.after(banner);
                }
            };
            el.querySelectorAll('[data-source-field]').forEach(input => {
                input.addEventListener('input', () => updateSourceDraft(input));
                input.addEventListener('change', () => updateSourceDraft(input));
            });
        }
        function paintNode(node) {
            const draft = state.drafts.get(node.id) || clone(node);
            state.drafts.set(node.id, draft);
            show();
            el.innerHTML = nodeMarkup(node, draft);
            decorate(node, draft);
            bindNode(node);
        }

        // ---- connection and selection ---------------------------------------------------------
        function paintEdge(edge) {
            const nameOf = id => store.nodeById(id)?.name || id;
            show();
            el.innerHTML = `<div class="infra-detail-head"><div class="infra-detail-icon">${app.glyph('connect')}</div><div><p class="infra-detail-kind">${esc(t('desktop.infra_connection'))} · ${esc(t(`desktop.infra_edge_${edge.kind}`))}</p><h2>${esc(nameOf(edge.from))} → ${esc(nameOf(edge.to))}</h2></div><button type="button" class="infra-detail-close" data-action="close-inspector" aria-label="${esc(t('desktop.close'))}" title="${esc(t('desktop.close'))}">×</button></div>
              <div class="infra-form-section"><dl class="infra-facts"><dt>${esc(t('desktop.infra_relation_from_port'))}</dt><dd>${esc(edge.from_port || '—')}</dd><dt>${esc(t('desktop.infra_relation_to_port'))}</dt><dd>${esc(edge.to_port || '—')}</dd><dt>${esc(t('desktop.infra_relation_speed'))}</dt><dd>${esc(edge.speed || '—')}</dd><dt>${esc(t('desktop.infra_relation_label'))}</dt><dd>${esc(edge.label || '—')}</dd></dl>
              ${canWrite() && !edge.source ? `<div class="infra-form-actions"><button type="button" class="infra-button" data-action="edit-edge">${esc(t('desktop.infra_edit_connection'))}</button><button type="button" class="infra-button infra-danger-button" data-action="delete-edge">${esc(t('desktop.infra_remove_connection'))}</button></div>` : ''}</div>`;
        }
        function paintSelection() {
            const nodes = [...state.selection].map(id => store.nodeById(id)).filter(Boolean);
            const counts = new Map();
            for (const node of nodes) counts.set(node.kind, (counts.get(node.kind) || 0) + 1);
            show();
            el.innerHTML = `<div class="infra-detail-head"><div class="infra-detail-icon">${app.glyph('select')}</div><div><h2>${esc(t('desktop.infra_selected_count').replace('{count}', nodes.length))}</h2></div><button type="button" class="infra-detail-close" data-action="close-inspector" aria-label="${esc(t('desktop.close'))}" title="${esc(t('desktop.close'))}">×</button></div>
              <div class="infra-form-section"><ul class="infra-selection-kinds">${[...counts].map(([kind, count]) => `<li>${esc(app.labelForKind(kind))} <b>${count}</b></li>`).join('')}</ul>
              ${canWrite() ? `<div class="infra-form-actions"><button type="button" class="infra-button" data-action="arrange-selection">${esc(t('desktop.infra_arrange'))}</button><button type="button" class="infra-button infra-danger-button" data-action="delete-selection">${esc(t('desktop.infra_remove_node'))}</button></div>` : ''}</div>`;
        }
        function paint() {
            if (state.disposed || !root.isConnected) return;
            const node = store.nodeById(state.selected);
            if (node) { paintNode(node); return; }
            const edge = state.selectedEdge ? state.edges.find(item => item.id === state.selectedEdge) : null;
            if (edge) { paintEdge(edge); return; }
            if (state.selection.size > 1) { paintSelection(); return; }
            hide();
        }

        // ---- drafts ----------------------------------------------------------------------------------
        function updateDraft(event) {
            if (!canWrite() || event.target.dataset.sourceField) return;
            const node = store.nodeById(state.selected);
            if (!node) return;
            const draft = state.drafts.get(node.id) || clone(node);
            const target = event.target;
            if (target.dataset.nodeField) draft[target.dataset.nodeField] = target.value;
            if (target.dataset.profile) {
                const key = target.dataset.profile;
                draft.profile ||= {};
                if (key === 'tags') draft.profile[key] = target.value.split(',').map(value => value.trim()).filter(Boolean);
                else if (key === 'vlan') draft.profile[key] = target.value === '' ? 0 : Number(target.value);
                else draft.profile[key] = target.value;
            }
            const form = $('[data-detail-form]');
            if (form) readProfileLists(form, draft, node.id);
            state.drafts.set(node.id, draft);
            state.dirty.add(node.id);
            state.draftRevision.set(node.id, (state.draftRevision.get(node.id) || 0) + 1);
            const valid = form ? validateDetailForm(form) : true;
            const save = $('[type="submit"].infra-button-primary');
            if (save) save.disabled = !valid;
            let banner = $('.infra-draft-banner');
            if (banner?.closest('.infra-source-edit')) banner = null;
            if (!banner) {
                banner = document.createElement('div');
                banner.className = 'infra-draft-banner';
                banner.setAttribute('role', 'status');
                banner.innerHTML = '<i></i><span></span>';
                el.prepend(banner);
            }
            let label = banner.querySelector('span');
            if (!label) { label = document.createElement('span'); banner.append(label); }
            label.textContent = t('desktop.infra_draft_kept');
            if (target.matches('[data-node-field="kind"]')) paint();
        }
        function validateDetailForm(form) {
            const cidr = form.querySelector('[data-profile="cidr"]');
            if (cidr) cidr.setCustomValidity(validCIDR(cidr.value) ? '' : t('desktop.infra_invalid_cidr'));
            for (const input of form.querySelectorAll('[data-list-family="links"][data-key="url"]')) input.setCustomValidity(validHTTPLink(input.value) ? '' : t('desktop.infra_invalid_link'));
            const seen = new Set();
            for (const input of form.querySelectorAll('[data-list-family="custom"][data-key="key"]')) {
                const key = input.value.trim();
                input.setCustomValidity(key && seen.has(key) ? t('desktop.infra_duplicate_custom_key') : '');
                if (key) seen.add(key);
            }
            return form.checkValidity();
        }
        function readProfileLists(form, draft, id) {
            draft.profile ||= {};
            const rowsOf = family => [...form.querySelectorAll(`[data-list-row="${family}"]`)].map(row => Object.fromEntries([...row.querySelectorAll(`[data-list-family="${family}"]`)].map(input => [input.dataset.key, family === 'custom' ? input.value : input.value.trim()])));
            draft.profile.interfaces = rowsOf('interfaces');
            draft.profile.links = rowsOf('links');
            const customRows = rowsOf('custom');
            if (customRows.length || state.customRows.has(id)) {
                state.customRows.set(id, customRows);
                draft.profile.custom = Object.fromEntries(customRows.filter(row => row.key).map(row => [row.key.trim(), row.value]));
            }
            draft.profile.app_bindings = [...form.querySelectorAll('[data-app-target]:checked')].map(input => ({ app: input.dataset.app, target_id: input.dataset.targetId }));
        }
        async function saveNode(id) {
            if (!canWrite() || !state.dirty.has(id) || state.busy.has(`node-${id}`)) return;
            const form = $('[data-detail-form]');
            if (form && !validateDetailForm(form)) { form.reportValidity(); return; }
            const revision = state.draftRevision.get(id) || 0;
            const draft = clone(state.drafts.get(id));
            draft.id = id;
            store.setBusy(`node-${id}`, true);
            const review = async () => {
                await store.loadSnapshot(false);
                const latest = store.nodeById(id);
                if (latest) state.nodeReviews.set(id, { latest: clone(latest) });
                store.notify(t('desktop.infra_node_conflict_review'), 'error');
                paint();
            };
            try {
                const result = await store.request(`/nodes/${encodeURIComponent(id)}`, { method: 'PUT', body: draft });
                const updated = result.node || result;
                if (updated?.id) store.replaceNode(updated); else await store.loadSnapshot(false);
                if ((state.draftRevision.get(id) || 0) === revision || !state.dirty.has(id)) {
                    state.dirty.delete(id); state.nodeReviews.delete(id); state.drafts.set(id, clone(store.nodeById(id) || updated));
                    state.customRows.delete(id);
                    store.notify(t('desktop.infra_saved'), 'success');
                } else {
                    const newer = state.drafts.get(id) || clone(updated);
                    newer.version = updated?.version ?? newer.version;
                    state.drafts.set(id, newer); state.dirty.add(id);
                    store.notify(t('desktop.infra_saved_newer_draft'), 'info');
                }
                paint();
            } catch (error) {
                // A conflict or an uncertain write (a timeout included): reload the confirmed state and
                // let the operator compare it with the retained draft. Hiding or closing the window stays quiet.
                if (error.name === 'AbortError') return;
                if (error.status === 409 || store.uncertainMutation(error)) await review();
                else store.notify(store.messageFor(error), 'error');
            } finally { store.setBusy(`node-${id}`, false); }
        }

        // ---- source editing ------------------------------------------------------------------------------
        async function loadSourceEdit(id, source) {
            if (!canEditSource() || !SOURCE_KINDS.includes(source)) return;
            if (source === 'registry') void store.loadCredentials();
            try {
                const result = await store.request(`/nodes/${encodeURIComponent(id)}/source-edit?source=${encodeURIComponent(source)}`);
                state.sourceDrafts.set(id, result); state.sourceDirty.delete(id); state.sourceReviews.delete(id);
                if (sourceEditPending(result)) { state.sourcePending.add(id); state.sourceBlocked.add(id); } else { state.sourcePending.delete(id); state.sourceBlocked.delete(id); }
                paint();
            } catch (error) { store.notify(store.messageFor(error), 'error'); }
        }
        function reapplySourceDraft(id) {
            const review = state.sourceReviews.get(id), draft = state.sourceDrafts.get(id);
            if (!review || !draft || !canEditSource()) return;
            state.sourceDrafts.set(id, { ...draft, version: review.latest.version, fields: clone(draft.fields) });
            state.sourceReviews.delete(id); state.sourceBlocked.delete(id); state.sourcePending.delete(id); state.sourceDirty.add(id);
            store.notify(t('desktop.infra_reapply_ready'), 'info'); paint();
        }
        function reapplyNodeDraft(id) {
            const review = state.nodeReviews.get(id), draft = state.drafts.get(id);
            if (!review || !draft || !canWrite()) return;
            draft.version = review.latest.version; state.drafts.set(id, draft); state.nodeReviews.delete(id); state.dirty.add(id);
            state.draftRevision.set(id, (state.draftRevision.get(id) || 0) + 1);
            store.notify(t('desktop.infra_reapply_ready'), 'info'); paint();
        }
        const collectSourceFields = source => {
            const fields = {};
            el.querySelectorAll('[data-source-field]').forEach(input => { fields[input.dataset.sourceField] = sourceFieldValue(source, input.dataset.sourceField, input.value); });
            return fields;
        };
        const clearSourceState = id => { for (const set of [state.sourceDirty, state.sourceBlocked, state.sourcePending]) set.delete(id); state.sourceDrafts.delete(id); state.sourceReviews.delete(id); };
        async function reviewSourceEdit(id) {
            const draft = state.sourceDrafts.get(id);
            if (!draft || !canEditSource()) return;
            try {
                const latest = await store.request(`/nodes/${encodeURIComponent(id)}/source-edit?source=${encodeURIComponent(draft.source)}`);
                const submitted = collectSourceFields(draft.source);
                if (sourceEditPending(latest)) {
                    state.sourceDrafts.set(id, { ...latest, fields: submitted }); state.sourceDirty.add(id); state.sourceBlocked.add(id); state.sourcePending.add(id);
                    state.sourceReviews.delete(id);
                    store.notify(t('desktop.infra_source_still_pending'), 'info');
                    paint(); return;
                }
                const confirmed = Object.entries(submitted).every(([key, value]) => JSON.stringify(latest.fields?.[key]) === JSON.stringify(value));
                if (confirmed) {
                    clearSourceState(id);
                    store.notify(t('desktop.infra_source_review_confirmed'), 'success');
                } else {
                    state.sourceDrafts.set(id, { ...draft, version: latest.version, fields: submitted }); state.sourceBlocked.add(id); state.sourcePending.delete(id); state.sourceReviews.set(id, { latest, submitted });
                    store.notify(t('desktop.infra_source_reviewed'), 'info');
                }
                paint();
            } catch (error) { store.notify(store.messageFor(error), 'error'); }
        }
        async function saveSourceEdit(id) {
            const draft = state.sourceDrafts.get(id);
            if (!draft || !canEditSource() || !draft.editable || state.sourceBlocked.has(id)) return;
            const busyKey = `source-${id}`;
            if (state.busy.has(busyKey)) return;
            const fields = collectSourceFields(draft.source);
            const revision = state.sourceRevision.get(id) || 0;
            const path = `/nodes/${encodeURIComponent(id)}/source-edit?source=${encodeURIComponent(draft.source)}`;
            store.setBusy(busyKey, true);
            try {
                const result = await store.request(path, { method: 'PUT', body: { source: draft.source, fields, version: draft.version } });
                if (result.status === 'unknown' || result.status === 'pending' || sourceEditPending(result)) {
                    state.sourceBlocked.add(id);
                    if (result.status === 'pending' || sourceEditPending(result)) { state.sourcePending.add(id); store.notify(t('desktop.infra_source_pending'), 'info'); }
                    else store.notify(t('desktop.infra_source_outcome_unknown'), 'error');
                    paint(); return;
                }
                if ((state.sourceRevision.get(id) || 0) === revision) {
                    clearSourceState(id);
                    await store.loadSnapshot();
                    // The saved draft is gone, so the edit form must close even when the snapshot did not change.
                    paint();
                    store.notify(t('desktop.infra_source_saved'), 'success');
                } else {
                    const latest = await store.request(path);
                    const newer = state.sourceDrafts.get(id);
                    if (newer) newer.version = latest.version;
                    state.sourceDirty.add(id); await store.loadSnapshot();
                    store.notify(t('desktop.infra_source_saved_newer_draft'), 'info');
                }
            } catch (error) {
                if (error.status === 409) { state.sourceBlocked.add(id); state.sourcePending.delete(id); state.sourceReviews.delete(id); paint(); store.notify(store.messageFor(error), 'error'); }
                else if (store.uncertainMutation(error)) { state.sourceBlocked.add(id); state.sourcePending.delete(id); paint(); store.notify(t('desktop.infra_source_outcome_unknown'), 'error'); }
                else store.notify(store.messageFor(error), 'error');
            } finally { store.setBusy(busyKey, false); }
        }

        // ---- panel behaviour ---------------------------------------------------------------------------------
        function close({ restoreFocus = true } = {}) {
            const id = state.selected || state.selectedEdge || [...state.selection][0] || '';
            store.clearSelection();
            if (!restoreFocus || !id) return;
            const target = state.view === 'table'
                ? root.querySelector(`[data-row="${CSS.escape(id)}"] [data-select]`)
                : app.canvas?.cardEl(id) || app.canvas?.regionEl(id)?.querySelector('[data-region-header]') || app.canvas?.viewport;
            target?.focus({ preventScroll: true });
        }
        async function deleteSelectedEdge() {
            const edge = state.edges.find(item => item.id === state.selectedEdge);
            if (!edge || !canWrite()) return;
            if (!await app.dialogs.confirmAction(t('desktop.infra_remove_connection'), t('desktop.infra_remove_relation_confirm'))) return;
            try {
                if (await store.dispatch(window.InfraAppStore.commands.deleteEdge(edge))) app.announce(t('desktop.infra_connection_removed'));
            } catch (error) { store.notify(store.messageFor(error), 'error'); }
        }
        function editList(addList, removeList) {
            if (!canWrite()) return;
            const node = store.nodeById(state.selected), form = $('[data-detail-form]');
            if (!node || !form) return;
            const draft = state.drafts.get(node.id) || clone(node);
            readProfileLists(form, draft, node.id);
            const trigger = addList || removeList;
            const family = trigger.dataset.listAdd || trigger.dataset.listRemove;
            draft.profile ||= {};
            if (family === 'custom') {
                const rows = state.customRows.get(node.id) || [];
                if (addList) rows.push({ key: '', value: '' }); else rows.splice(Number(removeList.dataset.index), 1);
                state.customRows.set(node.id, rows);
                draft.profile.custom = Object.fromEntries(rows.filter(row => row.key.trim()).map(row => [row.key.trim(), row.value]));
            } else {
                const rows = draft.profile[family] || [];
                const blank = family === 'interfaces' ? { name: '', ip_address: '', mac_address: '', vlan: '', speed: '' } : { label: '', url: '' };
                if (addList) rows.push(blank); else rows.splice(Number(removeList.dataset.index), 1);
                draft.profile[family] = rows;
            }
            state.drafts.set(node.id, draft); state.dirty.add(node.id);
            state.draftRevision.set(node.id, (state.draftRevision.get(node.id) || 0) + 1);
            paint();
            if (addList) $(`[data-list-row="${CSS.escape(family)}"]:last-child input`)?.focus();
        }
        const onClick = event => {
            const addList = event.target.closest('[data-list-add]'), removeList = event.target.closest('[data-list-remove]');
            if (addList || removeList) { editList(addList, removeList); return; }
            const guest = event.target.closest('[data-inspector-guest]');
            if (guest) { store.select([guest.dataset.inspectorGuest]); return; }
            switch (event.target.closest('[data-action]')?.dataset.action) {
                case 'close-inspector': close(); break;
                case 'place-selected': void app.gestures.placeAtCenter([state.selected], { moveMode: true }); break;
                case 'arrange-selection': void app.gestures.arrange(); break;
                case 'delete-selection': void app.dialogs.deleteNodes([...state.selection]); break;
                case 'edit-edge': app.dialogs.openEdgeDialog(state.selectedEdge); break;
                case 'delete-edge': void deleteSelectedEdge(); break;
                case 'reload-targets': void store.loadTargets(true); break;
                default: break;
            }
        };
        el.addEventListener('click', onClick);

        // The panel is not repainted under the operator's hands: a dirty draft or a
        // focused field marks it stale instead, and focus leaving flushes it.
        const busyEditing = () => {
            if (!state.selected) return false;
            if (state.dirty.has(state.selected) || state.sourceDirty.has(state.selected)) return true;
            return el.contains(document.activeElement);
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
        function onBusy(key) {
            const busy = state.busy.has(key);
            if (key.startsWith('source-')) {
                const id = key.slice('source-'.length);
                const save = state.selected === id ? $('[data-action="save-source"]') : null;
                if (save) {
                    save.disabled = busy || !canEditSource() || !state.sourceDirty.has(id) || state.sourceBlocked.has(id);
                    save.textContent = busy ? t('desktop.infra_saving') : t('desktop.infra_save_source');
                }
            }
            if (key.startsWith('node-')) {
                const submit = $('[data-detail-form] button[type="submit"]');
                if (submit) submit.disabled = busy || !state.dirty.has(key.slice('node-'.length));
            }
        }
        const saveDraft = () => { if (state.selected && state.dirty.has(state.selected)) $('[data-detail-form]')?.requestSubmit(); };
        function focusName() {
            if (!$('[data-node-field="name"]')) paint();
            $('[data-node-field="name"]')?.focus();
        }
        function dispose() { el.removeEventListener('click', onClick); el.remove(); }

        // open selects id; the tabbed inspector (which takes the tab) replaces this.
        const open = id => store.select([id]);
        return { el, paint, paintIfIdle, flushStale, onBusy, saveDraft, focusName, close, open, dispose };
    }

    window.InfraAppInspector = { create };
})();
