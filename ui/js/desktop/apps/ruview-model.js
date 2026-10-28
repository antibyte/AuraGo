/* RuView view model: the pure state derivation behind the Status view.
   A classic script with no DOM, no fetch and no translation calls. It returns
   ids and translation key ids (without the "ruview." prefix); the views turn
   them into words. It also loads under Node vm for the table tests. */
(function () {
    'use strict';

    const VIEWS = Object.freeze(['status', 'boards', 'lab']);
    // One id per row of the state table, in table order.
    const STATE_IDS = Object.freeze([
        'loading', 'status_failed', 'off', 'installing', 'readonly_blocked', 'not_installed', 'stopped', 'starting_pull',
        'starting', 'stopping', 'stop_failed', 'error', 'external_unchecked', 'external_check_old', 'demo', 'waiting',
        'disconnected', 'live', 'stale', 'checking'
    ]);
    // Mirrors the Go ErrorCodes() list (sorted), plus the two codes only the browser can produce.
    const KNOWN_ERROR_CODES = Object.freeze([
        'already_running', 'artifact_missing', 'busy', 'cancelled', 'container_conflict', 'contract_unsupported', 'disabled',
        'docker_readonly', 'docker_unavailable', 'download_failed', 'external_runtime', 'image_denied', 'image_pull_failed',
        'invalid_config', 'invalid_node_id', 'not_installed', 'platform_unsupported', 'process_exited', 'readonly',
        'source_mismatch', 'start_failed', 'stop_failed', 'storage_full', 'storage_unreadable', 'timeout', 'unexpected_simulation',
        'unknown', 'unreachable', 'upstream_auth', 'upstream_error', 'network', 'forbidden'
    ]);
    const RUNTIMES = ['external', 'not_started', 'starting', 'running', 'stopping', 'stopped', 'exited', 'stop_unconfirmed'];
    // Error texts an older backend sent in the legacy `error` field.
    const LEGACY_ERROR_CODES = ['stop_failed', 'process_exited', 'source_mismatch', 'unexpected_simulation'];
    const FRESH_MS = 10000;
    const DOCKER_PHASES = ['preparing', 'pulling_image', 'creating', 'waiting_ready'];
    const NATIVE_PHASES = ['preparing', 'verifying', 'creating', 'waiting_ready'];
    const START_CODES = ['image_denied', 'image_pull_failed', 'docker_unavailable', 'start_failed', 'timeout', 'cancelled', 'busy', 'container_conflict'];
    const SETTINGS_CODES = ['docker_readonly', 'invalid_config', 'artifact_missing', 'platform_unsupported', 'source_mismatch',
        'unexpected_simulation', 'upstream_auth', 'disabled', 'external_runtime'];
    const WRITE_ACTIONS = ['runtime/start', 'runtime/install', 'flash'];
    // Stopping stays possible while the integration is off: disabling it stops the service, and that stop can fail.
    const NO_ENABLE_ACTIONS = ['refresh', 'open-settings', 'copy-diagnostics', 'about', 'show-boards', 'runtime/stop'];
    const NODE_LIVE_STATES = ['ok', 'not_applicable', 'not_running', 'unreachable', 'invalid'];
    const DASH = '—';

    const isNum = value => typeof value === 'number' && Number.isFinite(value);
    const str = value => typeof value === 'string' ? value : '';
    const has = (collection, key) => !!collection && (typeof collection.has === 'function' ? collection.has(key) : Array.isArray(collection) && collection.includes(key));
    // RFC 3339 text or epoch milliseconds to epoch milliseconds; Go's zero time and junk become null.
    function timeMs(value) {
        if (isNum(value)) return value;
        if (typeof value !== 'string' || !value) return null;
        const ms = Date.parse(value);
        return Number.isFinite(ms) && ms > 0 ? ms : null;
    }

    function runtimeOf(status) {
        const s = status || {};
        if (RUNTIMES.includes(s.runtime)) return s.runtime;
        // Older backends have no runtime field: derive it the way the service does.
        if (s.mode === 'external') return 'external';
        if (s.data_state === 'stopping') return 'stopping';
        if (s.error === 'stop_failed') return 'stop_unconfirmed';
        if (s.data_state === 'starting') return 'starting';
        if (s.ready) return 'running';
        if (s.error === 'process_exited') return 'exited';
        if (s.data_state === 'stopped') return 'stopped';
        return 'not_started';
    }

    function errorCodeOf(status) {
        const s = status || {};
        const code = str(s.error_code).trim();
        if (code) return code;
        if (typeof s.error === 'string' && s.error) return LEGACY_ERROR_CODES.includes(s.error) ? s.error : 'unknown';
        return '';
    }

    function availableViews(status) {
        const s = status || {};
        const views = ['status'];
        if (s.enabled === true && s.source === 'esp32') views.push('boards');
        if (s.enabled === true) views.push('lab');
        return views;
    }

    function blockReason(action, context) {
        const c = context || {};
        const status = c.status || null;
        const mode = status ? status.mode : '';
        if (has(c.pending, action)) return 'working';
        if ((WRITE_ACTIONS.includes(action) || action === 'allow-changes') && c.desktopReadonly) return 'reason_desktop_readonly';
        if (WRITE_ACTIONS.includes(action) && status && status.readonly) return 'reason_readonly';
        if (String(action).startsWith('runtime/') && mode === 'external') return 'external_runtime';
        if (action === 'runtime/install' && mode !== 'native') return 'install_native';
        if ((action === 'runtime/start' || action === 'runtime/install') && runtimeOf(status) === 'starting') return 'reason_wait_start';
        if (!NO_ENABLE_ACTIONS.includes(action) && status && !status.enabled) return 'enable_first';
        return '';
    }

    function phaseStep(mode, phase) {
        const phases = mode === 'docker' ? DOCKER_PHASES : mode === 'native' ? NATIVE_PHASES : null;
        const index = phases ? phases.indexOf(phase) : -1;
        return index < 0 ? null : {n: index + 1, total: phases.length};
    }

    // ---- boards ------------------------------------------------------------------------------------------------

    // Keeps the last RSSI of a board that went offline within one scope, and normalizes live_state.
    function mergeNodes(prev, data) {
        const incoming = Array.isArray(data) ? data : data && data.nodes;
        if (!Array.isArray(incoming)) throw Error('Invalid nodes response');
        const scope = Array.isArray(data) ? undefined : data.scope;
        const previous = new Map(prev && Array.isArray(prev.list) && prev.scope === scope ? prev.list.map(node => [node.node_id, node]) : []);
        const list = incoming.map(node => {
            const old = previous.get(node.node_id);
            if (node.csi_status !== 'offline' || !old || node.rssi_dbm != null || old.rssi_dbm == null) return node;
            return Object.assign({}, node, {rssi_dbm: old.rssi_dbm});
        });
        // live_state says why the live list is absent; an older backend only sends `available`.
        const source = Array.isArray(data) ? {} : data;
        const liveState = NODE_LIVE_STATES.includes(source.live_state) ? source.live_state : source.available === false ? 'unreachable' : 'ok';
        return {list, scope: scope || '', liveState, error: liveState === 'unreachable' || liveState === 'invalid' ? 'unavailable' : ''};
    }

    // Mirrors presenceFresh in internal/ruview/read.go: real ESP32 CSI that is active, sequenced and at most 10 s old.
    function boardFreshness(node, options) {
        const o = options || {};
        const n = node || {};
        if (!o.nodesError && o.source === 'esp32' && !o.simulated && n.csi_status === 'active' && n.csi_sequence != null
            && isNum(o.ageMs) && o.ageMs >= 0 && o.ageMs <= FRESH_MS) return 'live';
        return n.csi_status === 'offline' ? 'offline' : 'stale';
    }

    // The board's age grows between polls: the reported age plus the time since the list arrived.
    // Without both timestamps the age is unknown (null), never "just now".
    function nodeAgeMs(node, receivedAtPerf, nowPerf) {
        const seen = node ? node.csi_last_seen_ms : null;
        if (!isNum(seen) || seen < 0 || !isNum(receivedAtPerf) || !isNum(nowPerf)) return null;
        return Math.max(0, Math.round(seen + nowPerf - receivedAtPerf));
    }

    // A board list says something about boards only when it was read: an unreadable, invalid or not-applicable
    // list, or one that has not arrived yet, is "unknown" and must not read as "no boards".
    function boardListKnown(nodes) {
        const n = nodes || {};
        return Array.isArray(n.list) && n.loaded !== false && !n.error
            && n.liveState !== 'unreachable' && n.liveState !== 'invalid' && n.liveState !== 'not_applicable' && n.liveState !== 'not_running';
    }

    function liveBoardCount(nodes, status, nowPerf) {
        const n = nodes || {};
        const s = status || {};
        if (!boardListKnown(n) || s.source !== 'esp32') return null;
        let live = 0;
        for (const node of n.list) {
            const ageMs = nodeAgeMs(node, n.receivedAt, nowPerf);
            if (boardFreshness(node, {source: s.source, simulated: s.simulated === true, nodesError: n.error, ageMs}) === 'live') live++;
        }
        return live;
    }

    // ---- formatting --------------------------------------------------------------------------------------------

    function ageParts(ms) {
        if (!isNum(ms) || ms < 0) return null;
        if (ms < 1000) return {value: Math.round(ms), unit: 'ms'};
        if (ms < 60000) {
            const seconds = ms / 1000;
            // One decimal under 10 s, but only when it is not a zero.
            return {value: ms < 10000 && Math.round(seconds * 10) % 10 ? Number(seconds.toFixed(1)) : Math.round(seconds), unit: 's'};
        }
        if (ms < 3600000) return {value: Math.round(ms / 60000), unit: 'min'};
        return {value: Math.round(ms / 3600000), unit: 'h'};
    }

    function numberFormat(lang, options) {
        try { return new Intl.NumberFormat(lang || 'en', options); } catch (error) { return new Intl.NumberFormat('en', options); }
    }
    function dateFormat(lang, options) {
        try { return new Intl.DateTimeFormat(lang || 'en', options); } catch (error) { return new Intl.DateTimeFormat('en', options); }
    }

    // Binary division, shown as megabyte/gigabyte units; null (not measured) is a dash.
    function formatBytes(bytes, lang) {
        if (!isNum(bytes) || bytes < 0) return DASH;
        const mb = bytes / 1048576;
        const large = mb >= 1024;
        const value = large ? mb / 1024 : mb;
        return numberFormat(lang, {style: 'unit', unit: large ? 'gigabyte' : 'megabyte', unitDisplay: 'short', maximumFractionDigits: value < 10 ? 1 : 0}).format(value);
    }

    // Time of day; a timestamp from another day also shows its date.
    function formatTime(ms, lang, now) {
        if (!isNum(ms)) return DASH;
        const date = new Date(ms);
        const today = new Date(isNum(now) ? now : Date.now());
        const options = date.toDateString() === today.toDateString() ? {hour: '2-digit', minute: '2-digit'} : {dateStyle: 'short', timeStyle: 'short'};
        return dateFormat(lang, options).format(date);
    }

    function formatDuration(ms) {
        if (!isNum(ms)) return DASH;
        const total = Math.max(0, Math.floor(ms / 1000));
        const hours = Math.floor(total / 3600), minutes = Math.floor(total % 3600 / 60), seconds = total % 60;
        const two = value => String(value).padStart(2, '0');
        return hours ? hours + ':' + two(minutes) + ':' + two(seconds) : minutes + ':' + two(seconds);
    }

    function fill(template, values) {
        const v = values || {};
        return String(template == null ? '' : template).replace(/\{(\w+)\}/g, (match, name) =>
            Object.prototype.hasOwnProperty.call(v, name) && v[name] != null ? String(v[name]) : match);
    }

    // ---- setup steps -------------------------------------------------------------------------------------------

    function setupSteps(status, nodes, options) {
        const s = status;
        // A real status always carries a boolean `enabled`; anything else is "not loaded yet".
        if (!s || typeof s !== 'object' || typeof s.enabled !== 'boolean') return null;
        if (s.last_fresh_at || s.fresh || s.source === 'simulated') return null;
        const external = s.mode === 'external', esp32 = s.source === 'esp32';
        const desktopReadonly = !!(options && options.desktopReadonly);
        // An unreadable board list cannot tell "no board yet": that step is then neither done nor the one to fix.
        const boardsKnown = boardListKnown(nodes);
        const steps = {
            enabled: {done: s.enabled === true, action: 'open-settings'},
            writable: {done: s.readonly !== true, action: desktopReadonly ? 'open-settings' : 'allow-changes'},
            service: {done: runtimeOf(s) === 'running', action: 'runtime/start'},
            // M30: UDP must really be listening, not merely enabled in the settings.
            receiver: {done: s.receiver_configured === true && s.udp_ready === true, action: 'open-settings'},
            checked: {done: !!s.observed_at && s.ready === true, action: 'test'},
            board: {done: boardsKnown && nodes.list.length > 0, unknown: !boardsKnown, action: 'flash'},
            data: {done: false, action: esp32 ? 'show-boards' : null}
        };
        const order = external
            ? (esp32 ? ['enabled', 'checked', 'board', 'data'] : ['enabled', 'checked', 'data'])
            : (esp32 ? ['enabled', 'writable', 'service', 'receiver', 'board', 'data'] : ['enabled', 'service', 'data']);
        let current = false;
        return order.map(id => {
            let state = 'done';
            if (!steps[id].done) {
                state = current || steps[id].unknown ? 'todo' : 'current';
                if (state === 'current') current = true;
            }
            return {id, state, action: steps[id].action};
        });
    }

    // ---- state derivation --------------------------------------------------------------------------------------

    // Row 11 action by error group: the thing the user can do about it.
    function errorAction(code, mode) {
        let pick = {action: 'test', label: 'action_check_again'};
        if (START_CODES.includes(code)) pick = {action: 'runtime/start', label: 'action_retry_start'};
        else if (code === 'process_exited') pick = {action: 'runtime/start', label: 'action_start_again'};
        else if (SETTINGS_CODES.includes(code)) pick = {action: 'open-settings', label: 'action_open_settings'};
        else if (code === 'not_installed' || code === 'download_failed') pick = {action: 'runtime/install', label: 'action_install'};
        else if (code === 'readonly') pick = {action: 'allow-changes', label: 'action_allow_changes'};
        // An external service has no runtime to start or install: all it offers is another look.
        if (mode === 'external' && pick.action.startsWith('runtime/')) pick = {action: 'test', label: 'action_check_again'};
        return pick;
    }

    function pluralOne(n, lang) {
        try { return new Intl.PluralRules(lang || 'en').select(n) === 'one'; } catch (error) { return n === 1; }
    }

    function deriveState(input) {
        const inp = input || {};
        const loaded = inp.statusLoaded === true, failed = inp.statusFailed === true;
        const status = inp.status && typeof inp.status === 'object' ? inp.status : {};
        // A real status always has a boolean `enabled`. Anything else that claims to be loaded is a broken answer,
        // and it must read as "unavailable", never as "turned off".
        const malformed = loaded && typeof status.enabled !== 'boolean';
        const now = isNum(inp.now) ? inp.now : Date.now();
        const statusAt = isNum(inp.statusAt) && inp.statusAt > 0 ? inp.statusAt : null;
        const nodes = inp.nodes && typeof inp.nodes === 'object' ? inp.nodes : {};
        const views = Array.isArray(inp.views) ? inp.views : availableViews(status);
        const hasView = view => views.includes(view);
        const desktopReadonly = inp.desktopReadonly === true;
        const mode = str(status.mode), source = str(status.source), phase = str(status.phase);
        const rt = runtimeOf(status);
        const rawCode = errorCodeOf(status);
        const code = rawCode ? (KNOWN_ERROR_CODES.includes(rawCode) ? rawCode : 'unknown') : '';
        // A storage problem is a chip, never the headline: rows 6 and 11 look past it.
        const blocking = code === 'storage_unreadable' ? '' : code;
        const ready = status.ready === true;
        const frameAge = isNum(status.last_frame_age_ms) ? status.last_frame_age_ms : null;
        // The age grows between polls, so the 10 s expiry flips the state without a new status. Without the time of
        // the last answer the stored age cannot be aged, so nothing is vouched for as fresh.
        const ageMs = frameAge === null ? null : frameAge + (statusAt === null ? 0 : Math.max(0, now - statusAt));
        const fresh = status.fresh === true && statusAt !== null && !(ageMs !== null && ageMs > FRESH_MS);
        const observedAt = timeMs(status.observed_at);
        const lastFreshAt = timeMs(status.last_fresh_at);
        const boards = source === 'esp32' && isNum(nodes.liveCount) ? nodes.liveCount : null;
        // "No boards yet" only when the list was read; an unreadable list is unknown, not empty.
        const emptyBoards = nodes.loaded === true && boardListKnown(nodes) && nodes.list.length === 0;
        const since = value => { const at = timeMs(value); return at === null ? null : Math.max(0, now - at); };
        const lastLive = () => lastFreshAt === null ? null : {kind: 'last_live', at: lastFreshAt};
        const lastChecked = () => observedAt === null ? null : {kind: 'last_checked', at: observedAt};

        function overlays(id) {
            const chips = [];
            if (status.enabled === true && status.readonly === true && id !== 'readonly_blocked') chips.push('read_only');
            if (status.storage_limit === true) chips.push('storage_full');
            if (code === 'storage_unreadable') chips.push('storage_unreadable');
            return chips;
        }
        function result(row, id, tone, glyph, pill, headline, o) {
            const x = o || {};
            const action = x.action || null;
            const block = !action ? '' : x.block !== undefined ? x.block : blockReason(action, {status: loaded ? status : null, pending: inp.pending, desktopReadonly});
            return {
                id, row, tone, glyph, busy: x.busy === true, headline, pill,
                pillAgeMs: x.pillAgeMs === undefined ? null : x.pillAgeMs,
                ageMs: x.ageMs === undefined ? ageMs : x.ageMs,
                action, actionLabel: action ? x.label : null, actionBlock: block,
                secondaryAction: x.secondaryAction || null, secondary: x.secondary || null,
                errorCode: x.bare ? '' : code, errorDetail: x.bare ? '' : str(status.error_detail),
                phase, boards, overlays: overlays(id)
            };
        }

        if (!loaded && !failed) return result('1', 'loading', 'neutral', '○', 'checking', 'headline_loading', {bare: true, ageMs: null});
        if (failed || malformed) {
            return result('2', 'status_failed', 'warning', '▲', 'unavailable', 'headline_status_failed', {
                bare: true, ageMs: null, action: 'refresh', label: 'action_try_again',
                secondary: loaded && !malformed && statusAt !== null ? {kind: 'last_success', at: statusAt} : null
            });
        }
        // A stop that failed or is unconfirmed outranks everything below, "turned off" included: disabling the
        // integration stops the service, and that stop can fail. A stop in progress stays row 9.
        if ((code === 'stop_failed' || rt === 'stop_unconfirmed') && rt !== 'stopping') {
            return result('10', 'stop_failed', 'danger', '✕', 'problem', 'headline_stop_failed', {
                action: 'runtime/stop', label: 'action_stop_again',
                secondary: Array.isArray(status.hints) && status.hints.includes('stop_unconfirmed') ? {kind: 'stop_hint'} : null
            });
        }
        if (!status.enabled) {
            return result('3', 'off', 'neutral', '○', 'off', 'headline_off', {action: 'open-settings', label: 'action_open_settings', secondary: {kind: 'about'}});
        }
        if (phase === 'downloading') {
            return result('3a', 'installing', 'info', '◐', 'installing', 'headline_installing', {busy: true, secondary: {kind: 'phase_since', sinceMs: since(status.phase_since)}});
        }
        if (mode !== 'external' && status.readonly === true && (rt === 'not_started' || rt === 'stopped') && !blocking) {
            // On a read-only desktop the switch is not offered here: the settings are the way out.
            return result('4', 'readonly_blocked', 'info', '◇', 'read_only', 'headline_readonly_blocked', desktopReadonly
                ? {action: 'open-settings', label: 'action_open_settings', secondary: {kind: 'readonly_blocks'}}
                : {action: 'allow-changes', label: 'action_allow_changes', secondaryAction: 'open-settings', secondary: {kind: 'readonly_blocks'}});
        }
        if (mode === 'native' && code === 'not_installed') {
            return result('5', 'not_installed', 'neutral', '○', 'not_installed', 'headline_not_installed', {action: 'runtime/install', label: 'action_install', secondary: {kind: 'install'}});
        }
        if (mode !== 'external' && (rt === 'not_started' || rt === 'stopped') && !blocking) {
            const reason = str(status.stop_reason);
            return result('6', 'stopped', 'neutral', '○', 'stopped', 'headline_stopped', {
                action: 'runtime/start', label: 'action_start',
                secondary: reason ? {kind: 'stop_reason', reason, at: timeMs(status.stopped_at)} : {kind: 'not_started'}
            });
        }
        if (rt === 'starting' && phase === 'pulling_image') {
            const step = phaseStep(mode, phase);
            return result('7', 'starting_pull', 'info', '◐', 'starting', 'headline_starting_pull', {
                busy: true, action: 'runtime/start', label: 'action_starting', block: 'reason_wait_start',
                secondary: step ? {kind: 'phase_step', n: step.n, total: step.total} : null
            });
        }
        if (rt === 'starting') {
            const sinceMs = since(status.phase_since);
            return result('8', 'starting', 'info', '◐', 'starting', phase ? 'headline_starting' : 'headline_starting_plain', {
                busy: true, action: 'runtime/start', label: 'action_starting', block: 'reason_wait_start',
                secondary: phase && sinceMs !== null ? {kind: 'phase_since', sinceMs} : null
            });
        }
        if (rt === 'stopping') return result('9', 'stopping', 'info', '◐', 'stopping', 'headline_stopping', {busy: true});
        if (blocking) {
            const pick = errorAction(blocking, mode);
            return result('11', 'error', 'danger', '✕', 'problem', 'err_' + blocking, {action: pick.action, label: pick.label, secondary: lastLive()});
        }
        if (rt === 'external' && observedAt === null) {
            return result('12', 'external_unchecked', 'neutral', '○', 'not_checked', 'headline_external_unchecked', {action: 'test', label: 'action_check_connection'});
        }
        if (rt === 'external' && now - observedAt > FRESH_MS) {
            return result('12a', 'external_check_old', 'neutral', '○', 'not_checked', 'headline_external_check_old', {
                action: 'test', label: 'check_now', pillAgeMs: now - observedAt, ageMs: now - observedAt, secondary: lastChecked()
            });
        }
        if (ready && status.simulated === true && source === 'simulated') {
            return result('13', 'demo', 'info', '◇', 'demo', 'headline_demo', {
                action: hasView('live') ? 'open-live' : null, label: 'action_open_live', secondaryAction: 'open-settings', secondary: {kind: 'demo'}
            });
        }
        if (ready && (status.waiting_for_frames === true || (source === 'esp32' && frameAge === null && status.data_state !== 'disconnected'))) {
            const next = emptyBoards && hasView('boards') ? {action: 'flash', label: 'action_add_board'}
                : hasView('boards') ? {action: 'show-boards', label: 'action_show_boards'} : {};
            return result('14', 'waiting', 'info', '◐', 'waiting', source === 'wifi' ? 'headline_waiting_wifi' : 'headline_waiting', Object.assign({secondary: lastChecked()}, next));
        }
        if (ready && status.data_state === 'disconnected') {
            return result('15', 'disconnected', 'warning', '▲', 'disconnected', 'headline_disconnected',
                Object.assign({secondary: lastLive()}, source === 'esp32' && hasView('boards')
                    ? {action: 'show-boards', label: 'action_show_boards'} : {action: 'open-settings', label: 'action_open_settings'}));
        }
        if (ready && fresh && status.simulated !== true) {
            const headline = source === 'wifi' ? 'headline_live_wifi'
                : source === 'esp32' && boards !== null && boards >= 1 ? (pluralOne(boards, inp.lang) ? 'headline_live_board' : 'headline_live_boards')
                : 'headline_live';
            const next = hasView('live') ? {action: 'open-live', label: 'action_open_live'}
                : hasView('boards') ? {action: 'show-boards', label: 'action_show_boards'} : {};
            return result('16', 'live', 'success', '●', 'live', headline, Object.assign({
                pillAgeMs: ageMs, secondary: {kind: 'live_meta', at: observedAt, source, mode}
            }, next));
        }
        if (ready && (status.data_state === 'stale' || (!fresh && ageMs !== null && ageMs > FRESH_MS))) {
            return result('17', 'stale', 'warning', '▲', 'no_new_data', 'headline_stale',
                Object.assign({secondary: lastLive()}, source === 'esp32' && hasView('boards') ? {action: 'show-boards', label: 'action_show_boards'} : {}));
        }
        return result('F', 'checking', 'neutral', '○', 'checking', 'headline_loading', {action: 'test', label: 'check_now'});
    }

    // ---- diagnostics -------------------------------------------------------------------------------------------

    // An Authorization header: a quoted value, or everything to the end of the line (the scheme word and its
    // parameters, which for Digest are several words). Failing closed here beats guessing where the credential ends.
    const AUTH_HEADER = /authorization["']?\s*[:=]\s*(?:"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'|[^\r\n]*)/gi;
    // Another secret-looking key (optionally quoted, prefixed or suffixed: access_token, "Api-Key") with : or =,
    // then the whole value: a quoted string, or an optional auth scheme word and the rest of the word.
    const SECRET_PAIR = /(?:token|secret|passw(?:or)?d|api[_-]?key|credentials?)[\w.-]*["']?\s*[:=]\s*(?:"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'|(?:(?:basic|bearer)\s+)?\S+)/gi;

    // One value as plain text: no URL, no secret value, one line, bounded.
    function clean(value) {
        if (value === null || value === undefined) return '';
        let text = (Array.isArray(value) ? value.map(String).join(',') : String(value)).slice(0, 2000);
        text = text.replace(/\b[a-z][a-z0-9+.-]*:\/\/\S+/gi, '[url]')
            .replace(AUTH_HEADER, '[redacted]')
            .replace(SECRET_PAIR, '[redacted]')
            .replace(/\bbearer\s+[\w.~+\/=-]{8,}/gi, '[redacted]')
            .replace(/\s+/g, ' ').trim();
        return text.length > 240 ? text.slice(0, 240).trim() : text;
    }

    // Language-neutral key=value lines for a bug report. The legacy `error` text is never included.
    // meta = {statusLoaded, statusFailed, statusAt} says what the status is worth: a status that never arrived (or
    // arrived malformed) prints `status=not_loaded` / `status=invalid` and none of its fields, because defaults
    // would read as "off"; one that failed after a success prints `status=last_known`, `last_success` and the old values.
    function diagnostics(status, nodes, now, meta) {
        const m = meta || {};
        const present = !!status && typeof status === 'object' && !Array.isArray(status);
        const usable = present && typeof status.enabled === 'boolean';
        const known = m.statusLoaded === undefined ? present : m.statusLoaded === true && present;
        const s = known && usable ? status : {};
        const n = nodes || {};
        const when = isNum(now) ? now : Date.now();
        const lines = ['RuView diagnostics ' + new Date(when).toISOString()];
        const add = (key, value) => lines.push(key + '=' + clean(value));
        const list = Array.isArray(n.list) ? n.list : null;
        let live = isNum(n.liveCount) ? n.liveCount : null;
        if (live === null && list) live = liveBoardCount(n, s, typeof performance === 'undefined' ? null : performance.now());
        const state = !known ? 'not_loaded' : !usable ? 'invalid' : m.statusFailed === true ? 'last_known' : 'current';
        lines.push('status=' + state);
        if (state === 'last_known' && isNum(m.statusAt) && m.statusAt > 0) lines.push('last_success=' + new Date(m.statusAt).toISOString());
        if (state === 'current' || state === 'last_known') {
            for (const key of ['revision', 'revision_verified', 'mode', 'source', 'enabled', 'readonly', 'agent_access']) add(key, s[key]);
            add('runtime', runtimeOf(s));
            for (const key of ['phase', 'phase_since', 'ready', 'data_state', 'fresh', 'simulated', 'waiting_for_frames', 'last_frame_age_ms',
                'error_code', 'error_detail', 'observed_at', 'last_fresh_at', 'stop_reason', 'stopped_at',
                'udp_scope', 'udp_enabled', 'udp_ready', 'receiver_configured', 'storage_used_bytes', 'storage_budget_bytes', 'storage_limit',
                'recording_session', 'recording_ack', 'recording_at', 'hints']) add(key, s[key]);
        }
        lines.push('boards live_state=' + clean(n.liveState) + ' known=' + (list ? list.length : '') + ' live=' + clean(live));
        return lines.join('\n');
    }

    window.RuViewModel = {
        VIEWS, STATE_IDS, KNOWN_ERROR_CODES,
        runtimeOf, errorCodeOf, availableViews, deriveState, blockReason, setupSteps,
        mergeNodes, boardFreshness, nodeAgeMs, liveBoardCount,
        ageParts, formatBytes, formatTime, formatDuration, fill, phaseStep, diagnostics
    };
})();
