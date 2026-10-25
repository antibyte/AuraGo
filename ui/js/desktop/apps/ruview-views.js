/* RuView views: Status, Boards and Lab (window.RuViewViews).
   Each view is mounted once into its panel: mount(panel, deps) builds the skeleton with escaped static labels and
   returns {update(snapshot), tick(), dispose()}. update() patches text, attributes and hidden flags only when a value
   changed and never replaces an element, so keyboard focus survives every poll.
   Dynamic copy comes only through the literal maps in kit(), so the static i18n lint sees every key. */
(function () {
    'use strict';

    // ---- shared helpers ----------------------------------------------------------------------------------------

    const set = (el, text) => { if (el && el.textContent !== text) el.textContent = text; };
    const show = (el, visible) => { if (el && el.hidden === visible) el.hidden = !visible; };
    function attr(el, name, value) {
        if (!el) return;
        if (value === null || value === undefined || value === false) { if (el.hasAttribute(name)) el.removeAttribute(name); return; }
        const text = value === true ? '' : String(value);
        if (el.getAttribute(name) !== text) el.setAttribute(name, text);
    }
    const pick = (map, key, fallback) => typeof key === 'string' && Object.hasOwn(map, key) ? map[key]() : fallback();

    // Every translation key the Status view can show is spelled out here, once.
    function copyMaps(t) {
        return {
            HEADLINE: {
                headline_loading: () => t('headline_loading'), headline_status_failed: () => t('headline_status_failed'), headline_off: () => t('headline_off'),
                headline_installing: () => t('headline_installing'), headline_readonly_blocked: () => t('headline_readonly_blocked'),
                headline_not_installed: () => t('headline_not_installed'), headline_stopped: () => t('headline_stopped'),
                headline_starting_pull: () => t('headline_starting_pull'), headline_starting: () => t('headline_starting'),
                headline_starting_plain: () => t('headline_starting_plain'), headline_stopping: () => t('headline_stopping'),
                headline_stop_failed: () => t('headline_stop_failed'), headline_external_unchecked: () => t('headline_external_unchecked'),
                headline_external_check_old: () => t('headline_external_check_old'), headline_demo: () => t('headline_demo'),
                headline_waiting: () => t('headline_waiting'), headline_waiting_wifi: () => t('headline_waiting_wifi'),
                headline_disconnected: () => t('headline_disconnected'), headline_live_board: () => t('headline_live_board'),
                headline_live_boards: () => t('headline_live_boards'), headline_live: () => t('headline_live'),
                headline_live_wifi: () => t('headline_live_wifi'), headline_stale: () => t('headline_stale')
            },
            ERROR: {
                disabled: () => t('err_disabled'), readonly: () => t('err_readonly'), external_runtime: () => t('err_external_runtime'),
                docker_readonly: () => t('err_docker_readonly'), already_running: () => t('err_already_running'), not_installed: () => t('err_not_installed'),
                busy: () => t('err_busy'), cancelled: () => t('err_cancelled'), container_conflict: () => t('err_container_conflict'),
                platform_unsupported: () => t('err_platform_unsupported'), invalid_node_id: () => t('err_invalid_node_id'), invalid_config: () => t('err_invalid_config'),
                unreachable: () => t('err_unreachable'), upstream_auth: () => t('err_upstream_auth'), upstream_error: () => t('err_upstream_error'),
                docker_unavailable: () => t('err_docker_unavailable'), image_denied: () => t('err_image_denied'), image_pull_failed: () => t('err_image_pull_failed'),
                contract_unsupported: () => t('err_contract_unsupported'), download_failed: () => t('err_download_failed'), start_failed: () => t('err_start_failed'),
                stop_failed: () => t('err_stop_failed'), process_exited: () => t('err_process_exited'), source_mismatch: () => t('err_source_mismatch'),
                unexpected_simulation: () => t('err_unexpected_simulation'), timeout: () => t('err_timeout'), storage_unreadable: () => t('err_storage_unreadable'),
                storage_full: () => t('err_storage_full'), artifact_missing: () => t('err_artifact_missing'), unknown: () => t('err_unknown'),
                network: () => t('err_network'), forbidden: () => t('err_forbidden')
            },
            PHASE: {
                preparing: () => t('phase_preparing'), pulling_image: () => t('phase_pulling_image'), creating: () => t('phase_creating'),
                waiting_ready: () => t('phase_waiting_ready'), downloading: () => t('phase_downloading'), verifying: () => t('phase_verifying')
            },
            RUNTIME: {
                external: () => t('state_external'), not_started: () => t('state_not_started'), starting: () => t('state_starting'), running: () => t('state_running'),
                stopping: () => t('state_stopping'), stopped: () => t('state_stopped'), exited: () => t('state_exited'), stop_unconfirmed: () => t('state_stop_unconfirmed')
            },
            STOP: {
                user: () => t('stop_reason_user'), config_changed: () => t('stop_reason_config_changed'),
                process_exit: () => t('stop_reason_process_exit'), aurago_shutdown: () => t('stop_reason_aurago_shutdown')
            },
            PILL: {
                checking: () => t('pill_checking'), unavailable: () => t('pill_unavailable'), off: () => t('pill_off'), read_only: () => t('pill_read_only'),
                not_installed: () => t('pill_not_installed'), installing: () => t('pill_installing'), stopped: () => t('stopped_state'), starting: () => t('starting'),
                stopping: () => t('stopping'), problem: () => t('pill_problem'), not_checked: () => t('unchecked'), demo: () => t('demo'), waiting: () => t('pill_waiting'),
                disconnected: () => t('disconnected'), live: () => t('pill_live'), no_new_data: () => t('pill_no_new_data')
            },
            ACTION: {
                action_check_connection: () => t('action_check_connection'), action_check_again: () => t('action_check_again'), action_try_again: () => t('action_try_again'),
                action_open_settings: () => t('action_open_settings'), action_allow_changes: () => t('action_allow_changes'), action_install: () => t('action_install'),
                action_start: () => t('action_start'), action_retry_start: () => t('action_retry_start'), action_start_again: () => t('action_start_again'),
                action_stop_again: () => t('action_stop_again'), action_starting: () => t('action_starting'), action_show_boards: () => t('action_show_boards'),
                action_add_board: () => t('action_add_board'), check_now: () => t('check_now')
            },
            REASON: {
                working: () => t('working'), enable_first: () => t('enable_first'), external_runtime: () => t('external_runtime'), install_native: () => t('install_native'),
                reason_wait_start: () => t('reason_wait_start'), reason_readonly: () => t('reason_readonly'), reason_desktop_readonly: () => t('reason_desktop_readonly')
            },
            STEP: {
                enabled: () => t('step_enabled'), writable: () => t('step_writable'), service: () => t('step_service'), receiver: () => t('step_receiver'),
                checked: () => t('step_checked'), board: () => t('step_board'), data: () => t('step_data')
            },
            STEP_STATE: {done: () => t('check_ok'), current: () => t('check_attention'), todo: () => t('check_unchecked')},
            HINT: {sensing_stale: () => t('hint_sensing_stale'), storage_limit: () => t('hint_storage_limit'), stop_unconfirmed: () => t('hint_stop_unconfirmed')},
            CHIP: {read_only: () => t('pill_read_only'), storage_full: () => t('chip_storage_full'), storage_unreadable: () => t('chip_storage_unreadable')},
            AGE: {ms: () => t('age_ms'), s: () => t('age_s'), min: () => t('age_min'), h: () => t('age_h')},
            SOURCE: {esp32: () => t('esp32'), wifi: () => t('wifi'), simulated: () => t('simulated')},
            MODE: {docker: () => t('docker'), external: () => t('external'), native: () => t('native')},
            RECORDING: {started: () => t('recording_started'), stopped: () => t('recording_stopped')},
            RECORDING_ACTION: {started: () => t('recording_action_started'), stopped: () => t('recording_action_stopped')},
            VIEW: {status: () => t('view_status'), boards: () => t('view_boards'), lab: () => t('view_lab')},
            FRAME_TITLE: {
                'observatory.html': () => t('lab_frame_title_observatory'), 'pose-fusion.html': () => t('lab_frame_title_pose_fusion'),
                'index.html': () => t('lab_frame_title_dashboard')
            }
        };
    }

    // kit(t, lang): the copy maps plus the small text helpers every view and the app share.
    function kit(t, lang, model) {
        const copy = copyMaps(t);
        const numberFormat = (() => { try { return new Intl.NumberFormat(lang || 'en'); } catch (error) { return new Intl.NumberFormat('en'); } })();
        const api = {
            copy, set, show, attr, pick,
            ageText(ms) {
                const parts = model.ageParts(ms);
                return parts ? pick(copy.AGE, parts.unit, () => '{n}').replace('{n}', numberFormat.format(parts.value)) : '—';
            },
            reasonText: id => pick(copy.REASON, id, () => ''),
            actionText: id => pick(copy.ACTION, id, () => String(id || '')),
            errorText: code => pick(copy.ERROR, code, () => t('err_unknown')),
            pillText: id => pick(copy.PILL, id, () => t('pill_checking')),
            time: (ms, now) => model.formatTime(ms, lang, now),
            headlineTemplate(derived) {
                const key = String(derived.headline || '');
                return key.startsWith('err_') ? api.errorText(key.slice(4)) : pick(copy.HEADLINE, key, () => t('headline_loading'));
            },
            // The headline split around {age}, so the view can keep the age in its own element.
            headlineParts(derived) {
                const text = model.fill(api.headlineTemplate(derived), {n: derived.boards, phase: pick(copy.PHASE, derived.phase, () => '')});
                const cut = text.indexOf('{age}');
                return cut < 0 ? {before: text, age: null, after: ''} : {before: text.slice(0, cut), age: api.ageText(derived.ageMs), after: text.slice(cut + 5)};
            },
            headlineText(derived) { const p = api.headlineParts(derived); return p.before + (p.age === null ? '' : p.age) + p.after; },
            secondaryText(secondary, now) {
                if (!secondary) return '';
                const fill = (text, values) => model.fill(text, values);
                switch (secondary.kind) {
                    case 'last_success': return fill(t('secondary_status_last_known'), {time: api.time(secondary.at, now)});
                    case 'about': return t('secondary_off');
                    case 'phase_since': return secondary.sinceMs == null ? '' : fill(t('secondary_phase_since'), {duration: model.formatDuration(secondary.sinceMs)});
                    case 'readonly_blocks': return t('secondary_readonly_blocks');
                    case 'install': return t('secondary_install');
                    case 'not_started': return t('secondary_not_started');
                    case 'phase_step': return fill(t('secondary_phase_step'), {n: secondary.n, total: secondary.total});
                    case 'stop_hint': return t('secondary_stop_hint');
                    case 'demo': return t('secondary_demo');
                    case 'last_checked': return fill(t('secondary_last_checked'), {time: api.time(secondary.at, now)});
                    case 'last_live': return fill(t('secondary_last_live'), {time: api.time(secondary.at, now)});
                    case 'live_meta': return fill(t('secondary_live_meta'), {
                        time: api.time(secondary.at, now), source: pick(copy.SOURCE, secondary.source, () => String(secondary.source || '—')),
                        mode: pick(copy.MODE, secondary.mode, () => String(secondary.mode || '—'))
                    });
                    case 'stop_reason': {
                        const text = pick(copy.STOP, secondary.reason, () => '');
                        return fill(text, {time: secondary.at == null ? '—' : api.time(secondary.at, now)});
                    }
                    default: return '';
                }
            }
        };
        return api;
    }

    // Buttons carry data-action (or data-node); an aria-disabled one does nothing.
    function bindActions(panel, onAction, signal) {
        panel.addEventListener('click', event => {
            const el = event.target.closest('[data-action],[data-node]');
            if (!el || !panel.contains(el) || el.getAttribute('aria-disabled') === 'true') return;
            if (el.dataset.node !== undefined) onAction('flash', {nodeID: Number(el.dataset.node)});
            else onAction(el.dataset.action);
        }, {signal});
    }

    // ---- Status ------------------------------------------------------------------------------------------------

    const STORE_KEY = 'ruview.ui.v1';
    function readUI() { try { const value = JSON.parse(localStorage.getItem(STORE_KEY) || '{}'); return value && typeof value === 'object' ? value : {}; } catch (error) { return {}; } }
    function writeUI(patch) { try { localStorage.setItem(STORE_KEY, JSON.stringify(Object.assign(readUI(), patch))); } catch (error) { /* storage is optional */ } }

    function mountStatus(panel, deps) {
        const {t, esc, model, kit: k, uid} = deps;
        const abort = new AbortController();
        const chips = ['read_only', 'storage_full', 'storage_unreadable'].map(name => '<span class="ruview-chip" data-chip="' + name + '" hidden></span>').join('');
        const rows = ['service', 'source', 'storage', 'recording', 'hints']
            .map(name => '<div class="ruview-detail" data-detail="' + name + '"><dt></dt><dd></dd></div>').join('');
        panel.innerHTML =
            '<section class="ruview-card" data-ruview-card>' +
                '<h2 class="ruview-headline" data-ruview-headline><span class="ruview-glyph" data-ruview-glyph aria-hidden="true"></span>' +
                    '<span class="ruview-headline-text" data-ruview-headline-text><span data-h-before></span><span data-ruview-age aria-live="off"></span><span data-h-after></span></span></h2>' +
                '<p class="ruview-secondary" data-ruview-secondary></p>' +
                '<div class="ruview-next-row"><button type="button" class="ruview-btn ruview-btn-primary" data-ruview-next></button>' +
                    '<button type="button" class="ruview-btn" data-ruview-secondary-action></button></div>' +
                '<p class="ruview-reason" id="' + uid + '-next-reason" data-ruview-next-reason></p>' +
                '<div class="ruview-overlays" data-ruview-overlays>' + chips + '</div>' +
                '<div class="ruview-technical" data-ruview-error hidden><details data-ruview-technical><summary>' + esc(t('technical_details')) + '</summary><pre data-ruview-technical-text></pre></details></div>' +
                '<div class="ruview-action-error" data-ruview-action-error role="group" hidden><p data-ruview-action-error-text></p>' +
                    '<details data-ruview-action-technical><summary>' + esc(t('technical_details')) + '</summary><pre data-ruview-action-technical-text></pre></details></div>' +
            '</section>' +
            '<section class="ruview-card" data-ruview-setup-card hidden><h3>' + esc(t('setup_heading')) + '</h3><ol class="ruview-setup" data-ruview-setup></ol></section>' +
            '<details class="ruview-card ruview-details" data-ruview-details><summary>' + esc(t('details_summary')) + '</summary><dl class="ruview-detail-list">' + rows + '</dl>' +
                '<div class="ruview-detail-actions"><button type="button" class="ruview-btn" data-action="copy-diagnostics">' + esc(t('menu_copy_diagnostics')) + '</button></div>' +
                '<textarea class="ruview-diagnostics" data-ruview-diagnostics rows="10" readonly hidden aria-label="' + esc(t('menu_copy_diagnostics')) + '"></textarea></details>' +
            '<div class="ruview-about" data-ruview-about><p>' + esc(t('quality_short')) + '</p><details data-ruview-about-more><summary>' + esc(t('about_more')) + '</summary><p>' + esc(t('quality')) + '</p></details></div>';
        const q = selector => panel.querySelector(selector);
        const ref = {
            card: q('[data-ruview-card]'), headline: q('[data-ruview-headline]'), glyph: q('[data-ruview-glyph]'), before: q('[data-h-before]'), age: q('[data-ruview-age]'), after: q('[data-h-after]'),
            secondary: q('[data-ruview-secondary]'), next: q('[data-ruview-next]'), second: q('[data-ruview-secondary-action]'), reason: q('[data-ruview-next-reason]'),
            error: q('[data-ruview-error]'), technicalText: q('[data-ruview-technical-text]'),
            actionError: q('[data-ruview-action-error]'), actionText: q('[data-ruview-action-error-text]'), actionDetails: q('[data-ruview-action-technical]'),
            actionTechnical: q('[data-ruview-action-technical-text]'), setupCard: q('[data-ruview-setup-card]'), setup: q('[data-ruview-setup]'),
            details: q('[data-ruview-details]'), diagnostics: q('[data-ruview-diagnostics]'), about: q('[data-ruview-about-more]')
        };
        const chipEl = name => q('[data-chip="' + name + '"]');
        const labels = {service: t('service'), source: t('source'), storage: t('storage_card'), recording: t('recording'), hints: t('details_hints')};
        for (const [name, label] of Object.entries(labels)) set(q('[data-detail="' + name + '"] dt'), label);
        const detail = name => q('[data-detail="' + name + '"] dd');

        // The Details state is the one thing remembered across sessions; it is written only when the user toggles it.
        let quiet = 0;
        const programmatic = open => { if (ref.details.open !== open) { quiet++; ref.details.open = open; } };
        if (readUI().details === true) programmatic(true);
        ref.details.addEventListener('toggle', () => { if (quiet > 0) { quiet--; return; } writeUI({details: ref.details.open}); }, {signal: abort.signal});
        bindActions(panel, deps.onAction, abort.signal);

        // The label of a setup step's fix button.
        const STEP_ACTION = {'open-settings': 'action_open_settings', 'allow-changes': 'action_allow_changes', 'runtime/start': 'action_start',
            test: 'action_check_connection', flash: 'action_add_board', 'show-boards': 'action_show_boards'};
        const steps = new Map();
        function updateSteps(list, state) {
            show(ref.setupCard, !!list);
            if (!list) return;
            const ids = new Set(list.map(step => step.id));
            for (const [id, li] of steps) if (!ids.has(id)) { li.remove(); steps.delete(id); }
            list.forEach((step, index) => {
                let li = steps.get(step.id);
                if (!li) {
                    li = document.createElement('li');
                    li.dataset.step = step.id;
                    li.innerHTML = '<span class="ruview-step-mark" aria-hidden="true"></span><span class="ruview-step-body"><span class="ruview-step-label"></span>' +
                        '<span class="ruview-step-reason" id="' + uid + '-step-reason-' + step.id + '" aria-hidden="true" hidden></span></span><span class="ruview-sr-only"></span>';
                    steps.set(step.id, li);
                }
                if (ref.setup.children[index] !== li) ref.setup.insertBefore(li, ref.setup.children[index] || null);
                attr(li, 'data-step-state', step.state);
                set(li.querySelector('.ruview-step-mark'), step.state === 'done' ? '✓' : step.state === 'current' ? '→' : '○');
                set(li.querySelector('.ruview-step-label'), pick(k.copy.STEP, step.id, () => step.id));
                set(li.querySelector('.ruview-sr-only'), pick(k.copy.STEP_STATE, step.state, () => ''));
                const reason = li.querySelector('.ruview-step-reason');
                let fix = li.querySelector('[data-step-fix]');
                const wanted = step.state === 'current' && step.action;
                if (wanted && !fix) {
                    fix = document.createElement('button');
                    fix.type = 'button';
                    fix.className = 'ruview-btn ruview-btn-small';
                    fix.setAttribute('data-step-fix', '');
                    li.append(fix);
                }
                if (!wanted && fix) fix.remove();
                const block = wanted ? model.blockReason(step.action, {status: state.statusLoaded ? state.status : null, pending: state.pending, desktopReadonly: state.desktopReadonly}) : '';
                set(reason, block ? k.reasonText(block) : '');
                show(reason, block !== '');
                if (wanted && fix) {
                    attr(fix, 'data-action', step.action);
                    attr(fix, 'aria-disabled', block ? 'true' : null);
                    attr(fix, 'aria-describedby', block ? reason.id : null);
                    set(fix, k.actionText(STEP_ACTION[step.action] || ''));
                }
            });
        }
        function detailTexts(state) {
            const s = state.statusLoaded && state.status && typeof state.status === 'object' ? state.status : null;
            if (!s) return {service: '—', source: '—', storage: '—', recording: '—', hints: '—'};
            const runtime = pick(k.copy.RUNTIME, model.runtimeOf(s), () => '—');
            const mode = pick(k.copy.MODE, s.mode, () => String(s.mode || '—'));
            const service = [runtime, mode];
            if (s.mode !== 'external') {
                if (s.revision) service.push(t('details_version') + ' ' + String(s.revision).slice(0, 12));
                service.push(s.revision_verified ? t('revision_verified') : t('revision_unverified'));
            }
            const source = [pick(k.copy.SOURCE, s.source, () => String(s.source || '—'))];
            if (s.source !== 'esp32') source.push(t('devices_not_applicable'));
            else {
                source.push(s.receiver_configured ? t('receiver_set') : t('receiver_missing'));
                source.push(s.udp_scope === 'external' ? t('udp_operator') : s.udp_enabled ? t('udp_on') : t('udp_off'));
            }
            let storage = '—';
            if (s.mode !== 'external' && Number.isFinite(s.storage_budget_bytes)) {
                storage = Number.isFinite(s.storage_used_bytes)
                    ? model.fill(t('storage_of'), {used: model.formatBytes(s.storage_used_bytes, deps.lang), budget: model.formatBytes(s.storage_budget_bytes, deps.lang)})
                    : t('storage_unknown');
            }
            const at = Date.parse(s.recording_at);
            let recording = pick(k.copy.RECORDING_ACTION, s.recording_session, () => '');
            if (recording && Number.isFinite(at) && at > 0) recording = model.fill(t('recording_last'), {action: recording, time: k.time(at, Date.now())});
            else if (recording) recording = pick(k.copy.RECORDING, s.recording_session, () => t('recording_unknown'));
            else recording = t('recording_unknown');
            if (s.recording_ack === 'unconfirmed') recording += ' · ' + t('recording_unconfirmed');
            const hints = Array.isArray(s.hints) && s.hints.length ? s.hints.map(name => pick(k.copy.HINT, name, () => String(name))).join(', ') : t('hints_none');
            return {service: service.join(' · '), source: source.join(' · '), storage, recording, hints};
        }

        function update(snapshot) {
            const {state, derived, now} = snapshot;
            attr(ref.headline, 'data-state', derived.id);
            attr(ref.headline, 'data-tone', derived.tone);
            attr(ref.card, 'data-tone', derived.tone);
            attr(ref.glyph, 'data-busy', derived.busy ? 'true' : 'false');
            set(ref.glyph, derived.glyph);
            const parts = k.headlineParts(derived);
            set(ref.before, parts.before);
            set(ref.age, parts.age === null ? '' : parts.age);
            set(ref.after, parts.after);
            const secondary = k.secondaryText(derived.secondary, now);
            set(ref.secondary, secondary);
            show(ref.secondary, secondary !== '');

            const action = derived.action, block = derived.actionBlock || '';
            show(ref.next, !!action);
            if (action) {
                attr(ref.next, 'data-action', action);
                set(ref.next, k.actionText(derived.actionLabel));
                attr(ref.next, 'aria-disabled', block ? 'true' : null);
                attr(ref.next, 'aria-describedby', block ? ref.reason.id : null);
                attr(ref.next, 'aria-busy', derived.busy ? 'true' : null);
            }
            set(ref.reason, action && block ? k.reasonText(block) : '');
            show(ref.reason, !!(action && block));
            show(ref.second, !!derived.secondaryAction);
            if (derived.secondaryAction) {
                attr(ref.second, 'data-action', derived.secondaryAction);
                set(ref.second, k.actionText('action_open_settings'));
            }
            for (const name of ['read_only', 'storage_full', 'storage_unreadable']) {
                const chip = chipEl(name);
                set(chip, pick(k.copy.CHIP, name, () => name));
                show(chip, derived.overlays.includes(name));
            }
            const technical = derived.id === 'error' || derived.id === 'stop_failed' ? derived.errorDetail : '';
            show(ref.error, technical !== '');
            set(ref.technicalText, technical);

            const failed = state.actionError;
            const failure = failed && failed.code !== 'cancelled' && failed.code !== derived.errorCode ? failed : null;
            show(ref.actionError, !!failure);
            if (failure) {
                const own = failure.detail || (failure.message && failure.message !== failure.code ? failure.message : '');
                const allow = failure.action === 'allow-changes';
                set(ref.actionText, allow ? t('allow_changes_failed') : k.errorText(failure.code));
                const lines = (allow ? [k.errorText(failure.code), own] : [own]).filter(Boolean).join('\n');
                set(ref.actionTechnical, lines);
                show(ref.actionDetails, lines !== '');
            }
            updateSteps(snapshot.steps, state);
            const texts = detailTexts(state);
            for (const [name, value] of Object.entries(texts)) set(detail(name), value);
        }

        return {
            update, tick() {},
            // The clipboard refused: show the text so it can be selected by hand.
            showDiagnostics(text) {
                programmatic(true);
                ref.diagnostics.value = text;
                show(ref.diagnostics, true);
                ref.diagnostics.focus({preventScroll: false});
                ref.diagnostics.select();
            },
            openAbout() { ref.about.open = true; ref.about.querySelector('summary').focus(); ref.about.scrollIntoView({block: 'nearest'}); },
            dispose() { abort.abort(); }
        };
    }

    // ---- Boards ------------------------------------------------------------------------------------------------

    function mountBoards(panel, deps) {
        const {t, esc, model, kit: k, uid} = deps;
        const abort = new AbortController();
        panel.innerHTML = '<div class="ruview-panel-head"><h2>' + esc(t('view_boards')) + ' <span class="ruview-count" data-ruview-board-count></span></h2></div>' +
            '<p class="ruview-note">' + esc(t('devices_known')) + '</p><p class="ruview-feedback" data-ruview-boards-feedback></p>' +
            '<p class="ruview-note" id="' + uid + '-add-reason" data-ruview-add-reason hidden></p><div class="ruview-board-grid" data-node-list></div>';
        const grid = panel.querySelector('[data-node-list]');
        const feedback = panel.querySelector('[data-ruview-boards-feedback]');
        const addReason = panel.querySelector('[data-ruview-add-reason]');
        const count = panel.querySelector('[data-ruview-board-count]');
        const cards = new Map();
        let last = null;
        bindActions(panel, deps.onAction, abort.signal);

        function create(id) {
            const article = document.createElement('article');
            article.className = 'ruview-node';
            article.dataset.nodeId = id;
            article.innerHTML = '<h3></h3>' +
                '<div class="ruview-node-row"><span>' + esc(t('signal_csi')) + '</span><span data-fresh></span></div>' +
                '<div class="ruview-node-row"><span>' + esc(t('signal_rssi')) + '</span><span data-rssi></span></div>' +
                '<div class="ruview-node-row"><span>' + esc(t('last_measurement')) + '</span><span data-age></span></div>' +
                '<button type="button" class="ruview-btn" data-node="' + esc(id) + '">' + esc(t('configure_node')) + '</button>';
            return article;
        }
        function paint(state, nowPerf) {
            const status = state.status && typeof state.status === 'object' ? state.status : {};
            const nodes = state.nodes;
            const block = model.blockReason('flash', {status: state.statusLoaded ? state.status : null, pending: state.pending, desktopReadonly: state.desktopReadonly});
            const message = state.statusLoaded && status.enabled === false ? t('disabled') : nodes.error ? t('devices_stale') : !nodes.loaded ? t('working')
                : nodes.liveState === 'not_applicable' ? t('devices_not_applicable') : nodes.liveState === 'not_running' ? t('devices_not_running') : !nodes.list.length ? t('devices_empty') : '';
            set(feedback, message);
            set(addReason, block ? k.reasonText(block) : '');
            show(addReason, !!block);
            set(count, nodes.loaded ? String(nodes.list.length) : '');
            const ids = nodes.list.map(node => String(node.node_id ?? node.id));
            for (const [id, card] of cards) if (!ids.includes(id)) { card.remove(); cards.delete(id); }
            nodes.list.forEach((node, index) => {
                const id = ids[index];
                let card = cards.get(id);
                if (!card) { card = create(id); cards.set(id, card); }
                if (grid.children[index] !== card) grid.insertBefore(card, grid.children[index] || null);
                set(card.querySelector('h3'), model.fill(t('board_title'), {id}));
                const ageMs = model.nodeAgeMs(node, nodes.receivedAt, nowPerf);
                const age = card.querySelector('[data-age]');
                set(age, k.ageText(ageMs));
                if (ageMs === null) { if (age.dataset.ageMs !== undefined) delete age.dataset.ageMs; } else attr(age, 'data-age-ms', ageMs);
                const fresh = model.boardFreshness(node, {source: status.source, simulated: status.simulated === true, nodesError: nodes.error, ageMs});
                set(card.querySelector('[data-fresh]'), fresh === 'live' ? t('received') : fresh === 'offline' ? t('device_offline') : t('stale'));
                const retained = node.csi_status === 'offline' && node.rssi_dbm != null;
                // A missing value is a dash, never "— dBm".
                set(card.querySelector('[data-rssi]'), node.rssi_dbm == null ? '—' : (retained ? t('rssi_retained') + ' ' : '') + String(node.rssi_dbm) + ' dBm');
                const configure = card.querySelector('[data-node]');
                attr(configure, 'aria-disabled', block ? 'true' : null);
                attr(configure, 'aria-describedby', block ? addReason.id : null);
            });
        }
        return {
            update(snapshot) { last = snapshot; paint(snapshot.state, snapshot.nowPerf); },
            // Ages grow between polls: only text changes here.
            tick() { if (last) paint(last.state, performance.now()); },
            dispose() { abort.abort(); }
        };
    }

    // ---- Lab ---------------------------------------------------------------------------------------------------

    const LAB_PAGES = ['observatory.html', 'pose-fusion.html', 'index.html'];

    function mountLab(panel, deps) {
        const {t, esc, kit: k, base} = deps;
        // Card texts through literal calls, indexed like LAB_PAGES.
        const texts = [
            {title: t('observatory'), desc: t('lab_observatory_desc'), req: t('lab_req_csi')},
            {title: t('pose_fusion'), desc: t('lab_pose_fusion_desc'), req: t('lab_req_csi')},
            {title: t('lab_dashboard'), desc: t('lab_dashboard_desc'), req: t('lab_req_any')}
        ];
        const chip = name => '<span class="ruview-chip" data-chip="' + name + '">' + esc(name === 'english' ? t('chip_english') : t('chip_experimental')) + '</span>';
        panel.innerHTML = '<div data-ruview-lab-landing><p class="ruview-note">' + esc(t('lab_intro')) + '</p><div class="ruview-lab-cards">' +
            LAB_PAGES.map((name, index) => '<article class="ruview-lab-card" data-lab-card="' + name + '"><h3>' + esc(texts[index].title) + '</h3>' +
                '<p>' + esc(texts[index].desc) + '</p><p class="ruview-chips">' + chip('experimental') + chip('english') + '</p>' +
                '<p class="ruview-note">' + esc(texts[index].req) + '</p><button type="button" class="ruview-btn" data-lab-open="' + name + '">' + esc(t('lab_open')) + '</button></article>').join('') +
            '</div></div>' +
            '<div class="ruview-lab-page" data-ruview-lab-page hidden><div class="ruview-lab-bar"><button type="button" class="ruview-btn" data-lab-close="back">' + esc(t('lab_back')) + '</button>' +
                '<strong data-lab-title></strong>' + chip('experimental') + '<button type="button" class="ruview-btn" data-lab-close="close">' + esc(t('close')) + '</button></div>' +
                '<p class="ruview-note" role="note" data-ruview-lab-notice hidden>' + esc(t('fusion_notice')) + '</p><div class="ruview-lab-frame" data-ruview-lab-frame></div></div>';
        const landing = panel.querySelector('[data-ruview-lab-landing]');
        const page = panel.querySelector('[data-ruview-lab-page]');
        const title = panel.querySelector('[data-lab-title]');
        const notice = panel.querySelector('[data-ruview-lab-notice]');
        const holder = panel.querySelector('[data-ruview-lab-frame]');
        let relayDetach = null, generation = 0, current = '';

        // The frame exists only while a page is open: closing, leaving the Lab and disposing remove it and its relay.
        function unmount() {
            generation++;
            relayDetach?.();
            relayDetach = null;
            holder.replaceChildren();
        }
        function close() {
            const opener = current;
            unmount();
            current = '';
            show(page, false);
            show(landing, true);
            panel.removeAttribute('data-lab-active');
            panel.querySelector('[data-lab-open="' + opener + '"]')?.focus();
        }
        async function open(name) {
            unmount();
            const epoch = generation;
            current = name;
            const index = LAB_PAGES.indexOf(name);
            set(title, index < 0 ? name : texts[index].title);
            show(notice, name === 'pose-fusion.html');
            show(landing, false);
            show(page, true);
            panel.setAttribute('data-lab-active', 'true');
            panel.querySelector('[data-lab-close="back"]').focus();
            try {
                const {attachRuViewRelay, tourFrameName} = await import(window.AuraLazyAssets.versionedURL('/js/ruview/relay.mjs'));
                if (epoch !== generation) return;
                const frame = document.createElement('iframe');
                frame.title = pick(k.copy.FRAME_TITLE, name, () => 'RuView');
                frame.sandbox='allow-scripts';frame.allow="serial 'none'; camera 'none'; microphone 'none'";
                try { frame.name = localStorage.getItem('aurago.ruview.onboarding.done') === 'true' ? tourFrameName : ''; } catch (error) { frame.name = ''; }
                holder.append(frame);
                relayDetach = attachRuViewRelay(frame, base);
                frame.src = base + 'ui/' + name;
            } catch (error) {
                if (epoch !== generation) return;
                const message = document.createElement('p');
                message.className = 'ruview-feedback';
                message.textContent = k.errorText('unknown');
                holder.append(message);
            }
        }
        const abort = new AbortController();
        panel.addEventListener('click', event => {
            const opener = event.target.closest('[data-lab-open]');
            if (opener) { void open(opener.dataset.labOpen); return; }
            if (event.target.closest('[data-lab-close]')) close();
        }, {signal: abort.signal});
        return {
            update() {}, tick() {},
            // Leaving the Lab view drops the page, as closing it does.
            hide() { if (current) close(); },
            dispose() { abort.abort(); unmount(); }
        };
    }

    window.RuViewViews = {
        kit,
        status: {mount: mountStatus},
        boards: {mount: mountBoards},
        lab: {mount: mountLab}
    };
})();
