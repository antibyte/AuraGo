// Infra inspector forms: the grouped Details form with its sticky save bar and
// field-level dirty markers, the app links picker, the icon picker, the
// separate source form (Sources tab), drafts, saving and conflict review. The
// inspector shell (infra-inspector.js) owns the header, tabs and panels and
// calls detailsHTML/sourcesHTML for the two form panels; every write goes
// through the store. Drafts live in state.drafts; app bindings live in the
// draft, never in the DOM.
(function () {
    'use strict';

    const KINDS = ['device', 'vm', 'container', 'service', 'network', 'site', 'room', 'rack', 'group'];
    const SOURCE_KINDS = ['registry', 'docker', 'proxmox', 'fritzbox'];
    const SOURCE_EDIT_FIELDS = { registry: ['name', 'type', 'protocol', 'ip_address', 'port', 'username', 'credential_id', 'description', 'tags', 'mac_address'], docker: ['name'], proxmox: ['name', 'hostname', 'description', 'tags'], fritzbox: ['friendly_name'] };
    const HARDWARE_FIELDS = ['vendor', 'model', 'serial', 'os', 'firmware', 'cpu', 'ram', 'storage', 'rack_position'];
    const GROUP_ORDER = ['identity', 'network', 'hardware', 'links', 'notes', 'custom'];
    const sourceEditPending = result => result?.pending === true || result?.fields?.pending === true;
    const sourceFieldValue = (source, key, value) => {
        if (key !== 'tags') return key === 'port' ? Number(value) : value;
        if (source === 'registry') return String(value).split(',').map(item => item.trim()).filter(Boolean);
        if (source === 'proxmox') return String(value).split(/[;,]/).map(item => item.trim()).filter(Boolean).join('; ');
        return value;
    };

    function create(app) {
        const { store, state, t, esc, model } = app;
        const { clone, iconId, iconLabelKey, sourceActions, sourceIdentity, validCIDR, validHTTPLink } = model;
        const S = app.safeId;
        const ICON_LIST = model.iconIds();
        const canWrite = () => store.canWrite();
        const canEditSource = () => store.canEditSource();
        const el = () => app.inspector.el;
        const $ = selector => el().querySelector(selector);
        const readonlyAttr = () => (!canWrite() ? 'readonly' : '');
        const disabledAttr = () => (!canWrite() ? 'disabled' : '');
        const groupOpen = new Map();
        let inline = null, iconPicker = null;

        // ---- review markup -----------------------------------------------------------------
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
                const labels = { name: t('desktop.infra_name'), kind: t('desktop.infra_type'), icon: t('desktop.infra_icon'), parent_id: t('desktop.infra_place') };
                const display = value => (key === 'parent_id' ? (store.nodeById(value)?.name || (value ? String(value) : t('desktop.infra_no_place'))) : key === 'icon' ? t(iconLabelKey(value)) : reviewValue(value, key));
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

        // ---- profile list editors ------------------------------------------------------------
        const inputFor = (family, index, key, value, type = 'text', attrs = '') => `<label class="infra-field"><span>${esc(t(`desktop.infra_field_${key}`))}</span><input type="${type}" data-list-family="${family}" data-index="${index}" data-key="${key}" value="${esc(value ?? '')}" ${attrs} ${readonlyAttr()}></label>`;
        const listEditor = (family, title, fields, rows, addKey, removeKey) => `<div class="infra-list-editor" data-list-editor="${family}" data-field="profile.${family}"><div class="infra-list-heading"><span>${esc(t(`desktop.infra_${title}`))}</span><button type="button" data-list-add="${family}" ${disabledAttr()}>＋ ${esc(t(`desktop.infra_${addKey}`))}</button></div>${rows.length ? rows.map((row, index) => `<div class="infra-list-row" data-list-row="${family}" data-index="${index}"><div class="infra-fields-grid">${fields.map(([key, type, attrs]) => inputFor(family, index, key, row[key], type, attrs || '')).join('')}</div><button type="button" class="infra-list-remove" data-list-remove="${family}" data-index="${index}" aria-label="${esc(t(`desktop.infra_${removeKey}`))}" ${disabledAttr()}>×</button></div>`).join('') : `<p class="infra-hint">${esc(t('desktop.infra_list_empty'))}</p>`}</div>`;
        const customEditor = (nodeID, profile) => {
            const rows = state.customRows.get(nodeID) || Object.entries(profile.custom || {}).map(([key, value]) => ({ key, value }));
            return `<div class="infra-list-editor" data-list-editor="custom" data-field="profile.custom"><div class="infra-list-heading"><span>${esc(t('desktop.infra_custom_fields'))}</span><button type="button" data-list-add="custom" ${disabledAttr()}>＋ ${esc(t('desktop.infra_add_custom_field'))}</button></div>${rows.length ? rows.map((row, index) => `<div class="infra-list-row" data-list-row="custom" data-index="${index}"><div class="infra-fields-grid">${inputFor('custom', index, 'key', row.key, 'text', 'required maxlength="80" pattern="[A-Za-z0-9_.-]+"')}<label class="infra-field"><span>${esc(t('desktop.infra_custom_value'))}</span><input data-list-family="custom" data-index="${index}" data-key="value" value="${esc(row.value ?? '')}" maxlength="2048" ${readonlyAttr()}></label></div><button type="button" class="infra-list-remove" data-list-remove="custom" data-index="${index}" aria-label="${esc(t('desktop.infra_remove_custom_field'))}" ${disabledAttr()}>×</button></div>`).join('') : `<p class="infra-hint">${esc(t('desktop.infra_list_empty'))}</p>`}</div>`;
        };

        // ---- app links ------------------------------------------------------------------------------
        const appLabel = appId => t(`desktop.infra_app_${String(appId).replaceAll('-', '_')}`);
        function chipsHTML(draft) {
            const available = new Set(state.appTargets.map(target => `${target.app}\x00${target.target_id}`));
            const write = canWrite();
            return (draft.profile?.app_bindings || []).map(link => {
                const target = state.appTargets.find(item => item.app === link.app && item.target_id === link.target_id);
                const name = target ? target.label || target.target_id : `${link.target_id} · ${t('desktop.infra_target_unavailable')}`;
                const ok = available.has(`${link.app}\x00${link.target_id}`), remove = model.fill(t('desktop.infra_remove_app_link'), { name });
                return `<li class="infra-app-chip${ok ? '' : ' is-unavailable'}" data-app-binding="${esc(link.app)}:${esc(link.target_id)}"><span class="infra-app-chip-text"><strong>${esc(name)}</strong><small>${esc(appLabel(link.app))}</small></span>`
                    + `${ok ? `<button type="button" data-open-binding="${esc(link.app)}" data-target-id="${esc(link.target_id)}">${esc(t('desktop.infra_open'))}</button>` : '<span></span>'}`
                    + `${write ? `<button type="button" data-remove-binding="${esc(link.app)}" data-target-id="${esc(link.target_id)}" aria-label="${esc(remove)}" title="${esc(remove)}">${app.glyph('close')}</button>` : ''}</li>`;
            }).join('');
        }
        function appLinksHTML(draft) {
            const hint = !state.appTargets.length ? `<p class="infra-hint">${esc(t(state.targetsError ? 'desktop.infra_targets_error' : state.targetsLoaded ? 'desktop.infra_no_app_targets' : 'desktop.infra_targets_loading'))}</p>` : '';
            const retry = state.targetsError ? `<button type="button" class="infra-button" data-action="reload-targets">${esc(t('desktop.infra_retry_targets'))}</button>` : '';
            const link = canWrite() && state.appTargets.length ? `<button type="button" class="infra-button" data-action="link-app" aria-haspopup="listbox">${app.glyph('plus')}<span>${esc(t('desktop.infra_link_app'))}</span></button><div class="infra-app-picker" data-app-picker hidden></div>` : '';
            return `<div class="infra-list-editor" data-list-editor="app-bindings" data-field="profile.app_bindings"><div class="infra-list-heading"><span>${esc(t('desktop.infra_application_links'))}</span></div><ul class="infra-app-chips" data-app-chips>${chipsHTML(draft)}</ul>${hint}${retry}${link}</div>`;
        }
        function appSource(query) {
            const node = store.nodeById(state.selected), draft = node && state.drafts.get(node.id);
            if (!node || !draft) return { groups: [] };
            const result = model.appTargetOptions({ ...node, name: draft.name, icon: draft.icon, profile: draft.profile }, state.appTargets, query);
            return {
                groups: result.groups.map(group => ({
                    id: group.id, label: group.id === 'suggested' ? t('desktop.infra_picker_suggested') : appLabel(group.id), more: group.more || 0,
                    options: group.options.map(option => ({ id: `${option.app}:${option.target_id}`, label: option.label, meta: appLabel(option.app), attrs: { 'data-app-target': '', 'data-app': option.app, 'data-target-id': option.target_id } })),
                })),
                note: result.hiddenMesh ? model.fill(t('desktop.infra_picker_mesh_hidden'), { count: result.hiddenMesh }) : '',
            };
        }
        function renderChips() {
            const draft = state.drafts.get(state.selected), list = $('[data-app-chips]');
            if (draft && list) list.innerHTML = chipsHTML(draft);
        }
        function addBinding(appId, targetId) {
            const id = state.selected, draft = state.drafts.get(id);
            if (!draft || !canWrite() || !appId || !targetId) return;
            draft.profile ||= {};
            const list = draft.profile.app_bindings ||= [];
            if (!list.some(item => item.app === appId && item.target_id === targetId)) list.push({ app: appId, target_id: targetId });
            markDirty(id);
            renderChips();
            inline?.refresh({ clear: true });
        }
        function removeBinding(appId, targetId) {
            const id = state.selected, draft = state.drafts.get(id);
            if (!draft || !canWrite()) return;
            const list = draft.profile?.app_bindings || [];
            const index = list.findIndex(item => item.app === appId && item.target_id === targetId);
            if (index < 0) return;
            list.splice(index, 1);
            markDirty(id);
            renderChips();
            const next = $('[data-app-chips] [data-remove-binding]') || $('[data-action="link-app"]');
            next?.focus();
        }
        function openAppPicker(button) {
            const box = $('[data-app-picker]');
            if (!box || !canWrite()) return;
            const group = button.closest('details');
            if (group) { group.open = true; groupOpen.set(`${state.selected}:links`, true); }
            box.hidden = false;
            if (!inline) inline = app.picker.attachInline(box, { label: t('desktop.infra_link_app'), placeholder: t('desktop.infra_link_app_search'), source: appSource, onPick: option => addBinding(option.attrs['data-app'], option.attrs['data-target-id']) });
            inline.focus();
        }

        // ---- the Details form --------------------------------------------------------------------------
        const field = (draft, name, label, type = 'text') => `<label class="infra-field" data-field="profile.${name}"><span>${esc(t(`desktop.infra_field_${label}`))}</span><input data-profile="${esc(name)}" type="${type}" value="${esc(draft.profile?.[name] ?? '')}" ${readonlyAttr()} autocomplete="off"></label>`;
        function placeOptions(node, draft) {
            const parentOf = id => store.nodeById(id)?.parent_id || '';
            const places = state.nodes.filter(place => model.isContainer(place.kind) && place.id !== node.id && !model.isDescendant(place.id, node.id, parentOf))
                .map(place => ({ place, path: model.placePath(place.id, store.nodeById, { includeSelf: true }).map(item => item.name).join(' › ') })).sort((a, b) => model.naturalCompare(a.path, b.path));
            const stale = draft.parent_id && !places.some(item => item.place.id === draft.parent_id) ? `<option value="${esc(draft.parent_id)}" selected>${esc(store.nodeById(draft.parent_id)?.name || draft.parent_id)}</option>` : '';
            return `<option value="">${esc(t('desktop.infra_no_place'))}</option>${stale}${places.map(item => `<option value="${esc(item.place.id)}" ${draft.parent_id === item.place.id ? 'selected' : ''}>${esc(item.path)}</option>`).join('')}`;
        }
        function groupBody(group, node, draft) {
            if (group === 'identity') {
                return `<div class="infra-field-row"><label class="infra-field" data-field="kind"><span>${esc(t('desktop.infra_type'))}</span><select data-node-field="kind" ${disabledAttr()}>${KINDS.map(kind => `<option value="${kind}" ${draft.kind === kind ? 'selected' : ''}>${esc(app.labelForKind(kind))}</option>`).join('')}</select></label>`
                    + `<label class="infra-field" data-field="icon"><span>${esc(t('desktop.infra_icon'))}</span><select data-node-field="icon" ${disabledAttr()}>${ICON_LIST.map(key => `<option value="${key}" ${iconId(draft.icon) === key ? 'selected' : ''}>${esc(t(iconLabelKey(key)))}</option>`).join('')}</select></label></div>`
                    + `<label class="infra-field" data-field="parent_id"><span>${esc(t('desktop.infra_place'))}</span><select data-node-field="parent_id" ${disabledAttr()}>${placeOptions(node, draft)}</select></label>`
                    + field(draft, 'role', 'role')
                    + `<label class="infra-field" data-field="profile.tags"><span>${esc(t('desktop.infra_tags'))}</span><input data-profile="tags" value="${esc((draft.profile?.tags || []).join(', '))}" placeholder="${esc(t('desktop.infra_tags_hint'))}" ${readonlyAttr()} autocomplete="off"></label>`;
            }
            if (group === 'network') {
                const segment = draft.kind === 'network' ? `<section class="infra-network-segment"><h4>${esc(t('desktop.infra_network_segment'))}</h4><div class="infra-field-row"><label class="infra-field" data-field="profile.cidr"><span>${esc(t('desktop.infra_cidr'))}</span><input data-profile="cidr" type="text" value="${esc(draft.profile?.cidr || '')}" placeholder="192.168.10.0/24" ${readonlyAttr()}></label><label class="infra-field" data-field="profile.vlan"><span>${esc(t('desktop.infra_vlan'))}</span><input data-profile="vlan" type="number" min="1" max="4094" value="${esc(draft.profile?.vlan || '')}" placeholder="${esc(t('desktop.infra_unknown'))}" ${readonlyAttr()}></label></div><p class="infra-hint">${esc(t('desktop.infra_cidr_hint'))}</p></section>` : '';
                return `<div class="infra-fields-grid">${field(draft, 'ip_address', 'ip_address')}${field(draft, 'mac_address', 'mac_address')}</div>${segment}`
                    + listEditor('interfaces', 'interfaces', [['name', 'text', 'required maxlength="64"'], ['ip_address', 'text'], ['mac_address', 'text'], ['vlan', 'text'], ['speed', 'text']], draft.profile?.interfaces || [], 'add_interface', 'remove_interface');
            }
            if (group === 'hardware') return `<div class="infra-fields-grid">${HARDWARE_FIELDS.map(name => field(draft, name, name)).join('')}</div>`;
            if (group === 'links') return listEditor('links', 'links', [['label', 'text', 'required maxlength="128"'], ['url', 'url', 'placeholder="https://" required maxlength="2048"']], draft.profile?.links || [], 'add_link', 'remove_link') + appLinksHTML(draft);
            if (group === 'notes') return `<label class="infra-field" data-field="profile.notes"><span>${esc(t('desktop.infra_notes'))}</span><textarea data-profile="notes" rows="4" ${readonlyAttr()}>${esc(draft.profile?.notes || '')}</textarea></label>`;
            return customEditor(node.id, draft.profile || {});
        }
        function detailsHTML(node, draft) {
            const dirty = state.dirty.has(node.id), write = canWrite(), filled = model.formGroupFill(draft), busy = state.busy.has(`node-${node.id}`);
            const titles = { identity: t('desktop.infra_identity'), network: t('desktop.infra_group_network'), hardware: t('desktop.infra_group_hardware'), links: t('desktop.infra_group_links'), notes: t('desktop.infra_notes'), custom: t('desktop.infra_custom_fields') };
            const groups = GROUP_ORDER.map(group => {
                const open = groupOpen.get(`${node.id}:${group}`) ?? filled[group];
                const add = !filled[group] && write ? `<span class="infra-form-group-add" aria-hidden="true">${app.glyph('plus')}${esc(t('desktop.infra_group_add'))}</span>` : '';
                return `<details class="infra-form-group" data-form-group="${group}"${open ? ' open' : ''}><summary class="infra-form-group-head"><span class="infra-form-group-title">${esc(titles[group])}</span>${add}<span class="infra-form-group-changed" data-group-changed hidden></span></summary><div class="infra-form-group-body">${groupBody(group, node, draft)}</div></details>`;
            }).join('');
            const bar = write
                ? `<div class="infra-savebar" data-savebar${dirty ? '' : ' hidden'}>${dirty ? `<p class="infra-draft-banner" role="status"><i aria-hidden="true"></i><span>${esc(t('desktop.infra_unsaved_changes'))}</span></p>` : ''}<span class="infra-flex-spacer"></span><button type="button" class="infra-button" data-action="revert-node">${esc(t('desktop.infra_discard'))}</button><button type="submit" class="infra-button infra-button-primary" ${dirty && !busy ? '' : 'disabled'}>${esc(busy ? t('desktop.infra_saving') : t('desktop.infra_save_in_infra'))}</button></div>`
                : `<p class="infra-readonly-note">${esc(t('desktop.infra_read_only'))}</p>`;
            return `<form class="infra-detail-form" id="infra-detail-form-${S}" data-detail-form data-node-id="${esc(node.id)}">${groups}${bar}</form>`;
        }

        // ---- drafts ----------------------------------------------------------------------------------------
        function markDirty(id) {
            state.dirty.add(id);
            state.draftRevision.set(id, (state.draftRevision.get(id) || 0) + 1);
            syncDirty();
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
        }
        // syncDirty brings the save bar, the tab marker and the field markers in line with the draft
        // without repainting the panel.
        function syncDirty() {
            const root = app.inspector?.el, id = state.selected, node = store.nodeById(id);
            if (!root || !node) return;
            const draft = state.drafts.get(id) || node, dirty = state.dirty.has(id), busy = state.busy.has(`node-${id}`);
            const form = root.querySelector('[data-detail-form]'), bar = root.querySelector('[data-savebar]');
            if (bar) {
                bar.hidden = !dirty;
                const banner = bar.querySelector('.infra-draft-banner');
                if (dirty && !banner) bar.insertAdjacentHTML('afterbegin', `<p class="infra-draft-banner" role="status"><i aria-hidden="true"></i><span>${esc(t('desktop.infra_unsaved_changes'))}</span></p>`);
                else if (!dirty && banner) banner.remove();
                const submit = bar.querySelector('[type="submit"]');
                if (submit) {
                    submit.disabled = !dirty || busy || !(form && validateDetailForm(form));
                    submit.textContent = busy ? t('desktop.infra_saving') : t('desktop.infra_save_in_infra');
                }
            }
            for (const marker of root.querySelectorAll('.infra-tab-dirty, [data-dirty-label]')) marker.hidden = !dirty;
            const changed = model.draftDiff(node, draft), counts = new Map();
            for (const path of changed) counts.set(model.groupOfField(path), (counts.get(model.groupOfField(path)) || 0) + 1);
            for (const holder of root.querySelectorAll('[data-detail-form] [data-field]')) {
                const on = changed.has(holder.dataset.field), mark = holder.querySelector('[data-dirty-text]');
                if (on) holder.dataset.dirty = 'true'; else delete holder.dataset.dirty;
                if (on && !mark) holder.querySelector('span')?.insertAdjacentHTML('beforeend', `<span class="infra-sr-only" data-dirty-text> · ${esc(t('desktop.infra_field_changed'))}</span>`);
                else if (!on && mark) mark.remove();
            }
            for (const group of root.querySelectorAll('[data-form-group]')) {
                const label = group.querySelector('[data-group-changed]'), count = counts.get(group.dataset.formGroup) || 0;
                label.hidden = count === 0;
                label.textContent = count ? model.fill(t('desktop.infra_group_changed'), { count }) : '';
            }
        }
        // onFieldEvent updates the draft from an input or change in the Details form or the header name.
        function onFieldEvent(event) {
            const target = event.target;
            if (!canWrite() || !target.dataset || target.dataset.sourceField !== undefined) return;
            if (target.dataset.nodeField === undefined && target.dataset.profile === undefined && target.dataset.listFamily === undefined) return;
            const node = store.nodeById(state.selected);
            if (!node) return;
            const draft = state.drafts.get(node.id) || clone(node);
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
            markDirty(node.id);
            if (target.matches('[data-node-field="kind"]')) {
                app.inspector.paint();
                $('[data-node-field="kind"]')?.focus();
            } else if (target.dataset.nodeField === 'name' || target.dataset.nodeField === 'icon') app.inspector.syncHead(draft);
        }
        function onToggle(event) {
            const group = event.target;
            if (!group?.matches?.('[data-form-group]')) return;
            const id = group.closest('[data-detail-form]')?.dataset.nodeId;
            if (id) groupOpen.set(`${id}:${group.dataset.formGroup}`, group.open);
        }
        // submitDetails submits the Details form; an invalid form shows its first problem instead.
        function submitDetails() {
            const form = $('[data-detail-form]');
            if (!form) return;
            const bad = [...form.elements].find(item => item.willValidate && !item.checkValidity());
            if (!bad) { form.requestSubmit(); return; }
            app.inspector.showTab('details');
            const group = bad.closest('details');
            if (group) { group.open = true; groupOpen.set(`${state.selected}:${group.dataset.formGroup}`, true); }
            if (bad.matches('.infra-head-name-input')) app.inspector.startRename();
            bad.reportValidity();
        }

        // ---- saving ---------------------------------------------------------------------------------------------
        async function saveNode(id) {
            if (!canWrite() || !state.dirty.has(id) || state.busy.has(`node-${id}`)) return;
            const form = $('[data-detail-form]');
            if (form && !validateDetailForm(form)) { submitDetails(); return; }
            const revision = state.draftRevision.get(id) || 0;
            const draft = clone(state.drafts.get(id));
            draft.id = id;
            store.setBusy(`node-${id}`, true);
            const review = async () => {
                await store.loadSnapshot(false);
                const latest = store.nodeById(id);
                if (latest) state.nodeReviews.set(id, { latest: clone(latest) });
                store.notify(t('desktop.infra_node_conflict_review'), 'error');
                app.inspector.paint();
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
                app.inspector.paint();
            } catch (error) {
                // A conflict or an uncertain write (a timeout included): reload the confirmed state and
                // let the operator compare it with the retained draft. Hiding or closing the window stays quiet.
                if (error.name === 'AbortError') return;
                if (error.status === 409 || store.uncertainMutation(error)) await review();
                else store.notify(store.messageFor(error), 'error');
            } finally { store.setBusy(`node-${id}`, false); }
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
            const group = trigger.closest('details');
            if (group) groupOpen.set(`${node.id}:${group.dataset.formGroup}`, true);
            state.drafts.set(node.id, draft);
            markDirty(node.id);
            app.inspector.paint();
            if (addList) $(`[data-list-row="${CSS.escape(family)}"]:last-of-type input`)?.focus();
            else $(`[data-list-add="${CSS.escape(family)}"]`)?.focus();
        }

        // ---- source rows and the source form ----------------------------------------------------------------------
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
            return `<form class="infra-form-section infra-source-edit infra-source-form" data-source-form><div class="infra-form-heading"><h3>${esc(t('desktop.infra_source_edit_title'))}</h3><span>${esc(t(`desktop.infra_source_${sourceEdit.source}`))}</span></div>${dirty ? `<div class="infra-draft-banner" role="status"><i></i>${esc(t('desktop.infra_source_draft'))}</div>` : ''}${pending && !dirty ? `<p class="infra-draft-banner" role="status"><i></i><span>${esc(t('desktop.infra_source_pending'))}</span></p>` : ''}${sourceReviewMarkup(node.id)}${fields}<p class="infra-hint" data-source-reason>${esc(t(`desktop.infra_reason_${sourceEdit.reason}`))}</p>${blocked ? `<p class="infra-draft-banner" role="status"><i></i><span>${esc(t(pending ? 'desktop.infra_source_pending_review' : 'desktop.infra_source_review_needed'))}</span></p>` : ''}<div class="infra-form-actions"><button type="button" class="infra-button" data-action="close-source-edit">${esc(t('desktop.cancel'))}</button>${blocked ? `<button type="button" class="infra-button" data-action="review-source">${esc(t(pending ? 'desktop.infra_review_pending_source' : 'desktop.infra_review_source'))}</button>` : ''}<button type="submit" data-action="save-source" class="infra-button infra-button-primary" ${!sourceEdit.editable || !canEditSource() || !dirty || blocked || saving ? 'disabled' : ''}>${esc(saving ? t('desktop.infra_saving') : t('desktop.infra_save_source'))}</button></div></form>`;
        };
        function sourcesHTML(node) {
            const sources = node.sources || [], sourceEdit = state.sourceDrafts.get(node.id);
            const list = sources.length
                ? `<div class="infra-form-section"><div class="infra-form-heading"><h3>${esc(t('desktop.infra_sources'))}</h3><span>${sources.length}</span></div><div class="infra-source-list">${sources.map(source => sourceMarkup(source, node)).join('')}</div></div>`
                : `<div class="infra-form-section"><h3>${esc(t('desktop.infra_sources'))}</h3><p class="infra-hint">${esc(t('desktop.infra_manual_only'))}</p></div>`;
            return `<section class="infra-sources-panel">${list}${sourceEdit ? sourceEditMarkup(node, sourceEdit) : ''}</section>`;
        }
        function onSourceFieldEvent(event) {
            const input = event.target, id = state.selected;
            if (!input?.dataset || input.dataset.sourceField === undefined || !id) return;
            const current = state.sourceDrafts.get(id);
            if (current) current.fields[input.dataset.sourceField] = input.value;
            state.sourceReviews.delete(id);
            state.sourceDirty.add(id);
            state.sourceRevision.set(id, (state.sourceRevision.get(id) || 0) + 1);
            const save = $('[data-action="save-source"]');
            if (save) save.disabled = !canEditSource() || state.sourceBlocked.has(id);
            if (!$('.infra-source-edit .infra-draft-banner')) {
                const banner = document.createElement('div');
                banner.className = 'infra-draft-banner';
                banner.setAttribute('role', 'status');
                banner.innerHTML = `<i></i>${esc(t('desktop.infra_source_draft'))}`;
                $('.infra-source-edit .infra-form-heading')?.after(banner);
            }
        }
        async function loadSourceEdit(id, source) {
            if (!canEditSource() || !SOURCE_KINDS.includes(source)) return;
            if (source === 'registry') void store.loadCredentials();
            try {
                const result = await store.request(`/nodes/${encodeURIComponent(id)}/source-edit?source=${encodeURIComponent(source)}`);
                state.sourceDrafts.set(id, result); state.sourceDirty.delete(id); state.sourceReviews.delete(id);
                if (sourceEditPending(result)) { state.sourcePending.add(id); state.sourceBlocked.add(id); } else { state.sourcePending.delete(id); state.sourceBlocked.delete(id); }
                app.inspector.showTab('sources');
                app.inspector.paint();
            } catch (error) { store.notify(store.messageFor(error), 'error'); }
        }
        function reapplySourceDraft(id) {
            const review = state.sourceReviews.get(id), draft = state.sourceDrafts.get(id);
            if (!review || !draft || !canEditSource()) return;
            state.sourceDrafts.set(id, { ...draft, version: review.latest.version, fields: clone(draft.fields) });
            state.sourceReviews.delete(id); state.sourceBlocked.delete(id); state.sourcePending.delete(id); state.sourceDirty.add(id);
            store.notify(t('desktop.infra_reapply_ready'), 'info'); app.inspector.paint();
        }
        function reapplyNodeDraft(id) {
            const review = state.nodeReviews.get(id), draft = state.drafts.get(id);
            if (!review || !draft || !canWrite()) return;
            draft.version = review.latest.version; state.drafts.set(id, draft); state.nodeReviews.delete(id); state.dirty.add(id);
            state.draftRevision.set(id, (state.draftRevision.get(id) || 0) + 1);
            store.notify(t('desktop.infra_reapply_ready'), 'info'); app.inspector.paint();
        }
        const collectSourceFields = source => {
            const fields = {};
            el().querySelectorAll('[data-source-field]').forEach(input => { fields[input.dataset.sourceField] = sourceFieldValue(source, input.dataset.sourceField, input.value); });
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
                    app.inspector.paint(); return;
                }
                const confirmed = Object.entries(submitted).every(([key, value]) => JSON.stringify(latest.fields?.[key]) === JSON.stringify(value));
                if (confirmed) {
                    clearSourceState(id);
                    store.notify(t('desktop.infra_source_review_confirmed'), 'success');
                } else {
                    state.sourceDrafts.set(id, { ...draft, version: latest.version, fields: submitted }); state.sourceBlocked.add(id); state.sourcePending.delete(id); state.sourceReviews.set(id, { latest, submitted });
                    store.notify(t('desktop.infra_source_reviewed'), 'info');
                }
                app.inspector.paint();
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
                    app.inspector.paint(); return;
                }
                if ((state.sourceRevision.get(id) || 0) === revision) {
                    clearSourceState(id);
                    await store.loadSnapshot();
                    // The saved draft is gone, so the edit form must close even when the snapshot did not change.
                    app.inspector.paint();
                    store.notify(t('desktop.infra_source_saved'), 'success');
                } else {
                    const latest = await store.request(path);
                    const newer = state.sourceDrafts.get(id);
                    if (newer) newer.version = latest.version;
                    state.sourceDirty.add(id); await store.loadSnapshot();
                    store.notify(t('desktop.infra_source_saved_newer_draft'), 'info');
                }
            } catch (error) {
                if (error.status === 409) { state.sourceBlocked.add(id); state.sourcePending.delete(id); state.sourceReviews.delete(id); app.inspector.paint(); store.notify(store.messageFor(error), 'error'); }
                else if (store.uncertainMutation(error)) { state.sourceBlocked.add(id); state.sourcePending.delete(id); app.inspector.paint(); store.notify(t('desktop.infra_source_outcome_unknown'), 'error'); }
                else store.notify(store.messageFor(error), 'error');
            } finally { store.setBusy(busyKey, false); }
        }

        // ---- the icon picker ----------------------------------------------------------------------------------------------
        function closeIconPicker({ restoreFocus = false } = {}) {
            if (!iconPicker) return;
            const { box, anchor, outside, focusout } = iconPicker;
            iconPicker = null;
            document.removeEventListener('pointerdown', outside, true);
            box.removeEventListener('focusout', focusout);
            box.remove();
            if (restoreFocus && anchor?.isConnected) anchor.focus({ preventScroll: true });
        }
        function openIconPicker(anchor) {
            closeIconPicker();
            const layer = app.q('[data-layer]'), id = state.selected, draft = state.drafts.get(id);
            if (!layer || !draft || !canWrite()) return;
            const current = iconId(draft.icon);
            const box = document.createElement('div');
            box.className = 'infra-popover infra-icon-picker'; box.dataset.iconPicker = '';
            box.setAttribute('role', 'dialog'); box.setAttribute('aria-label', t('desktop.infra_change_icon'));
            box.innerHTML = `<input type="search" data-icon-search aria-label="${esc(t('desktop.infra_icon_search'))}" placeholder="${esc(t('desktop.infra_icon_search'))}" autocomplete="off"><div class="infra-icon-grid" role="listbox" aria-label="${esc(t('desktop.infra_change_icon'))}" data-icon-grid>${ICON_LIST.map(key => { const label = t(iconLabelKey(key)); return `<button type="button" class="infra-icon-option" role="option" data-pick-icon="${key}" aria-selected="${key === current}" tabindex="${key === current ? 0 : -1}" title="${esc(label)}" aria-label="${esc(label)}">${app.icon(key)}</button>`; }).join('')}</div>`;
            layer.append(box);
            const rect = anchor.getBoundingClientRect(), bounds = app.root.getBoundingClientRect();
            box.style.left = `${Math.max(8, Math.min(rect.left - bounds.left, bounds.width - (box.offsetWidth || 340) - 8))}px`;
            box.style.top = `${rect.bottom - bounds.top + 6}px`;
            if (rect.bottom - bounds.top + 6 + box.offsetHeight > bounds.height - 8) box.style.top = `${Math.max(8, rect.top - bounds.top - box.offsetHeight - 6)}px`;
            const search = box.querySelector('[data-icon-search]'), grid = box.querySelector('[data-icon-grid]');
            const visible = () => [...grid.querySelectorAll('[data-pick-icon]')].filter(option => !option.hidden);
            const focusOption = option => { if (!option) return; for (const item of grid.querySelectorAll('[data-pick-icon]')) item.tabIndex = item === option ? 0 : -1; option.focus({ preventScroll: true }); };
            search.addEventListener('input', () => {
                const text = search.value.trim().toLocaleLowerCase();
                for (const option of grid.querySelectorAll('[data-pick-icon]')) option.hidden = !!text && !option.title.toLocaleLowerCase().includes(text) && !option.dataset.pickIcon.includes(text);
            });
            search.addEventListener('keydown', event => { if (event.key === 'ArrowDown') { event.preventDefault(); focusOption(visible()[0]); } });
            grid.addEventListener('keydown', event => {
                const options = visible(), index = options.indexOf(document.activeElement);
                const next = { ArrowLeft: index - 1, ArrowRight: index + 1, ArrowUp: index - 8, ArrowDown: index + 8, Home: 0, End: options.length - 1 }[event.key];
                if (next === undefined || index < 0) return;
                event.preventDefault();
                if (next < 0 && event.key === 'ArrowUp') { search.focus(); return; }
                focusOption(options[Math.max(0, Math.min(options.length - 1, next))]);
            });
            grid.addEventListener('click', event => {
                const option = event.target.closest('[data-pick-icon]');
                if (!option) return;
                const next = state.drafts.get(id);
                if (next) {
                    next.icon = option.dataset.pickIcon;
                    const select = $('[data-node-field="icon"]');
                    if (select) select.value = next.icon;
                    markDirty(id);
                    app.inspector.syncHead(next);
                }
                closeIconPicker({ restoreFocus: true });
            });
            box.addEventListener('keydown', event => { if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); closeIconPicker({ restoreFocus: true }); } });
            const outside = event => { if (!box.contains(event.target) && !anchor.contains(event.target)) closeIconPicker(); };
            const focusout = event => { if (event.relatedTarget && !box.contains(event.relatedTarget)) closeIconPicker(); };
            document.addEventListener('pointerdown', outside, true);
            box.addEventListener('focusout', focusout);
            iconPicker = { box, anchor, outside, focusout };
            search.focus({ preventScroll: true });
        }

        // ---- inspector hooks -----------------------------------------------------------------------------------------------------
        function afterPaint() {
            inline?.destroy(); inline = null;
            syncDirty();
        }
        function detach() {
            inline?.destroy(); inline = null;
            closeIconPicker();
        }
        // onClick answers true when the click belonged to a form control.
        function onClick(event) {
            const target = event.target;
            const addList = target.closest('[data-list-add]'), removeList = target.closest('[data-list-remove]');
            if (addList || removeList) { editList(addList, removeList); return true; }
            const open = target.closest('[data-open-binding]');
            if (open) { app.openAppLink({ app: open.dataset.openBinding, target_id: open.dataset.targetId }); return true; }
            const remove = target.closest('[data-remove-binding]');
            if (remove) { removeBinding(remove.dataset.removeBinding, remove.dataset.targetId); return true; }
            const edit = target.closest('[data-edit-source]');
            if (edit) { void loadSourceEdit(state.selected, edit.dataset.editSource); return true; }
            const sourceAction = target.closest('[data-source-action]');
            if (sourceAction) {
                const source = (store.nodeById(state.selected)?.sources || []).find(item => sourceIdentity(item) === sourceAction.dataset.sourceIdentity);
                if (source) app.dialogs.openActionMenu(state.selected, source);
                return true;
            }
            const button = target.closest('[data-action]');
            if (!button) return false;
            const id = state.selected;
            switch (button.dataset.action) {
                case 'link-app': openAppPicker(button); return true;
                case 'revert-node': {
                    const node = store.nodeById(id);
                    if (node) { state.drafts.set(id, clone(node)); state.dirty.delete(id); state.nodeReviews.delete(id); state.customRows.delete(id); app.inspector.paint(); }
                    return true;
                }
                case 'close-source-edit': clearSourceState(id); app.inspector.paint(); return true;
                case 'review-source': void reviewSourceEdit(id); return true;
                case 'reapply-source-draft': reapplySourceDraft(id); return true;
                case 'reapply-node-draft': reapplyNodeDraft(id); return true;
                case 'reload-targets': void store.loadTargets(true); return true;
                default: return false;
            }
        }
        // onSubmit answers true for the two forms of this module.
        function onSubmit(event) {
            const form = event.target;
            if (form.matches?.('[data-detail-form]')) { event.preventDefault(); void saveNode(state.selected); return true; }
            if (form.matches?.('[data-source-form]')) { event.preventDefault(); void saveSourceEdit(state.selected); return true; }
            return false;
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
            if (key.startsWith('node-') && state.selected === key.slice('node-'.length)) syncDirty();
        }
        const isBusy = () => !!iconPicker || !!inline?.isOpen?.();
        function dispose() { detach(); }

        return {
            detailsHTML, sourcesHTML, nodeReviewMarkup, afterPaint, detach, onClick, onFieldEvent, onSourceFieldEvent, onSubmit, onToggle,
            submitDetails, openIconPicker, onBusy, isBusy, dispose,
        };
    }

    window.InfraAppInspectorForms = { create };
})();
