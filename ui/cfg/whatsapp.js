// WhatsApp: config fields plus the link panel. Link, unlink and test act on
// saved settings only. The QR code is the server-rendered PNG data URI. It and
// the pairing code exist only in the panel's DOM: they are never logged,
// stored, or kept once the session ends or the section is left.
function renderWhatsAppSection(section) {
    const c = { enabled: false, mode: 'dedicated', owner_numbers: [], readonly: false, send_read_receipts: true, self_chat_prefix: '', ...configData.whatsapp };
    const form = window.AuraConfigForm;
    const field = (key, label, options = {}) => form.field({ id: 'whatsapp-' + key, path: 'whatsapp.' + key, label, value: c[key], ...options });
    const toggle = (key, label, help) => field(key, label, { type: 'toggle', help: help || '', value: c[key] === true });
    const button = (id, label, primary = false) => `<button id="whatsapp-${id}" type="button" class="${primary ? 'btn-save' : 'btn-secondary'}">${escapeHtml(label)}</button>`;
    document.getElementById('content').innerHTML = form.renderSpec({
        label: section.label,
        desc: t('config.whatsapp.intro'),
        beforeHTML: cfgStatusBanner.html('whatsapp-status', 'checking'),
        groups: [
            { titleKey: 'config.refresh.operation', content:
                toggle('enabled', t('config.whatsapp.enabled')) +
                field('mode', t('config.whatsapp.mode'), { type: 'select', help: t('help.whatsapp.mode'), options: [
                    { value: 'dedicated', label: t('config.whatsapp.mode_dedicated') },
                    { value: 'self_chat', label: t('config.whatsapp.mode_self_chat') }
                ] }) +
                '<div id="whatsapp-self-chat-notice" class="cfg-note is-warning" role="status" hidden>' + escapeHtml(t('config.whatsapp.self_chat_notice')) + '</div>' +
                '<div id="whatsapp-owner-fields">' + field('owner_numbers', t('config.whatsapp.owner_numbers'), {
                    value: (c.owner_numbers || []).join(', '), help: t('help.whatsapp.owner_numbers'), placeholder: '+491701234567'
                }) + '</div>'
            },
            { title: t('config.whatsapp.link_title'), desc: t('config.whatsapp.phone_rule'), className: 'whatsapp-link-panel', content:
                '<p id="whatsapp-linked" class="field-help" aria-live="polite"></p>' +
                // The pairing phone is not a config field: data-cfg-dirty-bound keeps it out of the save dock.
                '<label class="field-group pw-field"><span class="field-label">' + escapeHtml(t('config.whatsapp.link_phone')) + '</span>' +
                '<input id="whatsapp-phone" class="field-input" type="tel" autocomplete="off" inputmode="tel" placeholder="+491701234567" aria-describedby="whatsapp-action-reason" data-cfg-dirty-bound="true"></label>' +
                form.actions([
                    { html: button('link-qr', t('config.whatsapp.link_qr'), true) },
                    { html: button('link-phone', t('config.whatsapp.link_phone')) },
                    { html: button('unlink', t('config.whatsapp.unlink')) },
                    { html: button('test', t('config.whatsapp.send_test')) }
                ]) +
                '<p id="whatsapp-action-reason" class="field-help" aria-live="polite" hidden></p>' +
                cfgActionResult.html('whatsapp-result') +
                '<div id="whatsapp-link-view" class="whatsapp-link-view" hidden>' +
                    '<img id="whatsapp-qr" class="whatsapp-qr" alt="' + escapeAttr(t('config.whatsapp.link_qr')) + '" hidden>' +
                    '<p id="whatsapp-code" class="whatsapp-pairing-code" hidden></p>' +
                    '<p id="whatsapp-link-hint" class="field-help" aria-live="polite"></p>' +
                '</div>'
            },
            { titleKey: 'config.refresh.settings', content:
                toggle('readonly', t('config.whatsapp.readonly'), t('help.whatsapp.readonly')) +
                toggle('send_read_receipts', t('config.whatsapp.send_read_receipts'), t('help.whatsapp.send_read_receipts')) +
                field('self_chat_prefix', t('config.whatsapp.self_chat_prefix'), { help: t('help.whatsapp.self_chat_prefix') })
            }
        ]
    });
    const root = document.querySelector('#content .cfg-section');
    root.id = 'whatsapp-config';
    root.querySelector('#whatsapp-owner_numbers').dataset.type = 'array';
    const lifecycle = new AbortController();
    const alive = () => root.isConnected && !lifecycle.signal.aborted;
    let status = {};
    let linkState = 'idle';
    let linkTimer = 0;
    let linkFailures = 0;
    let settleTimer = 0;
    let settleCount = 0;
    // Literal maps keep every key visible to the static i18n lint. Codes that are
    // not listed (invalid_method, invalid_request, request_too_large,
    // method_not_allowed, whatsapp_failed, link_failed, connect_failed,
    // pair_failed, qr_failed) show err_failed.
    const ERRORS = {
        link_in_progress: () => t('config.whatsapp.err_link_in_progress'),
        already_linked: () => t('config.whatsapp.err_already_linked'),
        not_configured: () => t('config.whatsapp.err_not_configured'),
        invalid_phone: () => t('config.whatsapp.err_invalid_phone'),
        not_connected: () => t('config.whatsapp.err_not_connected'),
        recipient_not_on_whatsapp: () => t('config.whatsapp.err_recipient_not_on_whatsapp'),
        rate_limited: () => t('config.whatsapp.err_rate_limited'),
        csrf_check_failed: () => t('config.whatsapp.err_csrf'),
        admin_required: () => t('config.whatsapp.err_forbidden'),
        unauthorized: () => t('config.whatsapp.err_forbidden'),
        login_required_for_host: () => t('config.whatsapp.err_forbidden'),
        passkey_unsupported: () => t('config.whatsapp.err_passkey_unsupported'),
        cancelled: () => t('config.whatsapp.err_cancelled'),
        outdated: () => t('config.whatsapp.state_outdated')
    };
    // One entry per runtime state of internal/integrationstatus/whatsapp.go.
    const STATES = {
        disabled: () => ({ state: 'off' }),
        not_configured: () => ({ state: 'unconfigured', detail: t('config.whatsapp.err_not_configured') }),
        not_linked: () => ({ state: 'unconfigured', detail: t('config.whatsapp.not_linked') }),
        linking: () => ({ state: 'checking', label: t('config.whatsapp.state_linking') }),
        connecting: () => ({ state: 'checking', label: t('config.whatsapp.state_connecting') }),
        healthy: () => ({ state: 'connected' }),
        degraded: () => ({ state: 'degraded', detail: t('config.whatsapp.state_degraded') }),
        logged_out: () => ({ state: 'error', detail: t('config.whatsapp.state_logged_out') }),
        conflict: () => ({ state: 'error', detail: t('config.whatsapp.state_conflict') }),
        temp_banned: () => ({ state: 'error', detail: tempBanDetail() }),
        outdated: () => ({ state: 'error', detail: t('config.whatsapp.state_outdated') })
    };
    const ACTIVE = ['waiting', 'qr', 'code'];
    const ACTIONS = ['link-qr', 'link-phone', 'unlink', 'test'];
    // Every action locks the others while it runs, so link, unlink and test never overlap.
    const LOCK = '#whatsapp-config .whatsapp-link-panel .cfg-actions-row button';
    // Expiry and reason code of a temporary ban; the code is a fixed machine code such as temp_ban_101.
    function tempBanDetail() {
        const parts = [t('config.whatsapp.state_temp_banned')];
        const until = Date.parse(status.temp_ban_expires_at);
        if (Number.isFinite(until)) parts.push(t('config.whatsapp.state_temp_banned_until').replace('{time}', new Date(until).toLocaleString()));
        if (/^[a-z0-9_]{1,40}$/.test(status.last_error || '')) parts.push('(' + status.last_error + ')');
        return parts.join(' ');
    }
    const errorText = code => (typeof code === 'string' && Object.hasOwn(ERRORS, code) ? ERRORS[code]() : t('config.whatsapp.err_failed'));
    const reasonText = body => (body && body.error ? errorText(body.error) : '');

    function reason(action) {
        const saved = window.AuraConfigState.get('whatsapp') || {};
        if (hasUnsavedConfigChanges()) return t('config.whatsapp.save_first');
        if (action === 'unlink') return status.linked ? '' : t('config.whatsapp.not_linked');
        if (!saved.enabled || (saved.mode !== 'self_chat' && !(saved.owner_numbers || []).length)) return t('config.whatsapp.err_not_configured');
        if (action === 'link-phone' && !/^(\+|00)[0-9 ().-]{6,}$/.test(root.querySelector('#whatsapp-phone').value.trim())) return t('config.whatsapp.err_invalid_phone');
        if (action === 'test' && status.state !== 'healthy') return t('config.whatsapp.err_not_connected');
        return '';
    }

    function sync() {
        if (!alive()) return;
        const mode = root.querySelector('#whatsapp-mode').value;
        root.querySelector('#whatsapp-self-chat-notice').hidden = mode !== 'self_chat';
        root.querySelector('#whatsapp-owner-fields').hidden = mode === 'self_chat';
        root.querySelector('#whatsapp-linked').textContent = status.linked
            ? t('config.whatsapp.linked_as').replace('{number}', status.linked_jid_masked || '')
            : t('config.whatsapp.not_linked');
        const busy = !!root.querySelector('[data-config-action-busy="true"]');
        const reasons = ACTIONS.map(action => reason(action));
        ACTIONS.forEach((action, index) => {
            const control = root.querySelector('#whatsapp-' + action);
            control.disabled = !!reasons[index] || busy;
            control.title = reasons[index];
        });
        // The visible line names the first reason that disables an action; each button's title carries its own.
        const why = reasons.find(Boolean) || '';
        const note = root.querySelector('#whatsapp-action-reason');
        note.textContent = why;
        note.hidden = !why;
    }

    function describeStatus(body) {
        status = (body && body.whatsapp) || {};
        return Object.hasOwn(STATES, status.state) ? STATES[status.state]() : { state: 'unknown' };
    }

    // A state that is still settling (linking, connecting) is re-read quietly, a
    // bounded number of times, so the panel catches up without a reload.
    function armSettle() {
        clearTimeout(settleTimer);
        if (!alive()) return;
        if (status.state !== 'connecting' && status.state !== 'linking') {
            settleCount = 0;
            return;
        }
        if (++settleCount > 20) return;
        settleTimer = setTimeout(async () => {
            if (!alive()) return;
            try {
                const response = await fetch('/api/whatsapp/status', { credentials: 'same-origin', cache: 'no-store', signal: lifecycle.signal });
                if (response.ok) {
                    const body = await response.json();
                    if (!alive()) return;
                    const { state, ...extra } = describeStatus(body);
                    cfgStatusBanner(root.querySelector('#whatsapp-status'), state, extra);
                    sync();
                }
            } catch (_) { /* the next round or the user retries */ }
            armSettle();
        }, 3000);
    }

    async function refreshStatus() {
        if (!alive()) return;
        settleCount = 0;
        // A refused request (admin_required, ...) shows its translated text, never the raw code.
        await cfgStatusBanner.check(root.querySelector('#whatsapp-status'), signal => fetch('/api/whatsapp/status', { signal, credentials: 'same-origin', cache: 'no-store' }), describeStatus,
            { reasonText: body => errorText(body && body.error) });
        sync();
        armSettle();
    }

    function showLink(snap) {
        const state = snap && typeof snap.state === 'string' ? snap.state : 'idle';
        linkState = state;
        const active = ACTIVE.includes(state);
        const qr = root.querySelector('#whatsapp-qr');
        const code = root.querySelector('#whatsapp-code');
        const showQR = state === 'qr' && typeof snap.qr_png === 'string' && snap.qr_png.startsWith('data:image/png;base64,');
        const showCode = state === 'code' && typeof snap.pairing_code === 'string' && snap.pairing_code !== '';
        root.querySelector('#whatsapp-link-view').hidden = !active;
        if (showQR) qr.src = snap.qr_png; else qr.removeAttribute('src');
        qr.hidden = !showQR;
        code.hidden = !showCode;
        code.textContent = showCode ? snap.pairing_code : '';
        root.querySelector('#whatsapp-link-hint').textContent = showQR ? t('config.whatsapp.qr_hint') : (showCode ? t('config.whatsapp.code_hint') : (active ? t('config.whatsapp.state_linking') : ''));
        if (state === 'success') cfgActionResult.show('whatsapp-result', 'success', t('config.whatsapp.link_success'));
        else if (state === 'timeout') cfgActionResult.show('whatsapp-result', 'warning', t('config.whatsapp.link_timeout'));
        else if (state === 'error') cfgActionResult.show('whatsapp-result', 'error', errorText(snap.error));
    }

    // Polls only while a session is waiting, showing a QR code or a pairing code.
    // A final state, a few failed reads or leaving the section end it.
    function poll(delay = 2000) {
        clearTimeout(linkTimer);
        if (!alive()) return;
        linkTimer = setTimeout(async () => {
            if (!alive()) return;
            try {
                const response = await fetch('/api/whatsapp/link', { credentials: 'same-origin', cache: 'no-store', signal: lifecycle.signal });
                if (!response.ok) throw new Error('link read failed');
                const snap = await response.json();
                if (!alive()) return;
                linkFailures = 0;
                showLink(snap);
                if (ACTIVE.includes(linkState)) poll(); else refreshStatus();
            } catch (_) {
                if (!alive()) return;
                if (++linkFailures < 5) poll();
                else showLink({ state: 'error', error: 'link_failed' });
            }
        }, delay);
    }

    // force replaces a running session (switching method) and an existing device (relinking).
    const act = (control, run, options) => cfgActionResult(control, run, { lock: LOCK, ...options }).then(outcome => { sync(); return outcome; });

    function startLink(control, method) {
        const linked = !!status.linked;
        const body = { method, force: linked || ACTIVE.includes(linkState) };
        if (method === 'phone') body.phone = root.querySelector('#whatsapp-phone').value.trim();
        return act(control, () => fetch('/api/whatsapp/link', {
            method: 'POST', credentials: 'same-origin', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body)
        }), {
            target: 'whatsapp-result',
            confirm: linked ? { message: t('config.whatsapp.relink_confirm') } : undefined,
            reasonText,
            onSuccess: outcome => {
                if (!alive()) return;
                linkFailures = 0;
                cfgActionResult.show('whatsapp-result', '', '');
                showLink(outcome.body || {});
                refreshStatus();
                poll(700);
            },
            onError: outcome => {
                refreshStatus();
                // Another tab or browser owns the running session: show its code instead of only the error.
                if (alive() && outcome.body && outcome.body.error === 'link_in_progress') poll(0);
            }
        });
    }

    root.querySelector('#whatsapp-link-qr').addEventListener('click', event => startLink(event.currentTarget, 'qr'));
    root.querySelector('#whatsapp-link-phone').addEventListener('click', event => startLink(event.currentTarget, 'phone'));
    root.querySelector('#whatsapp-unlink').addEventListener('click', event => act(event.currentTarget,
        () => fetch('/api/whatsapp/logout', { method: 'POST', credentials: 'same-origin' }), {
            target: 'whatsapp-result',
            confirm: { message: t('config.whatsapp.unlink_confirm') },
            reasonText,
            // The handler answers {"status":"unlinked"}, which the shared reader does not count as success.
            interpret: outcome => (outcome.httpStatus < 400 && outcome.body && outcome.body.status === 'unlinked' ? { state: 'success' } : null),
            success: outcome => (outcome.body && outcome.body.remote_removed === false ? t('config.whatsapp.unlinked_local') : t('config.feedback.done')),
            onSuccess: () => { if (alive()) { showLink({ state: 'idle' }); refreshStatus(); } },
            onError: () => refreshStatus()
        }));
    root.querySelector('#whatsapp-test').addEventListener('click', event => act(event.currentTarget,
        () => fetch('/api/whatsapp/test', { method: 'POST', credentials: 'same-origin' }), {
            target: 'whatsapp-result',
            reasonText,
            success: t('config.whatsapp.test_sent'),
            onSuccess: () => refreshStatus(),
            onError: () => refreshStatus()
        }));
    root.querySelector('#whatsapp-phone').addEventListener('input', sync);
    root.querySelector('#whatsapp-mode').addEventListener('change', sync);
    document.addEventListener('cfg:statechange', sync, { signal: lifecycle.signal });
    document.addEventListener('aurago:config-saved', () => { refreshStatus(); }, { signal: lifecycle.signal });
    window.addEventListener('cfg:section-leave', () => {
        lifecycle.abort();
        clearTimeout(linkTimer);
        clearTimeout(settleTimer);
        root.querySelector('#whatsapp-qr').removeAttribute('src');
        root.querySelector('#whatsapp-code').textContent = '';
    }, { once: true, signal: lifecycle.signal });
    sync();
    // Resume a session that started before this section was opened.
    fetch('/api/whatsapp/link', { credentials: 'same-origin', cache: 'no-store', signal: lifecycle.signal })
        .then(response => (response.ok ? response.json() : null))
        .then(snap => { if (alive() && snap && ACTIVE.includes(snap.state)) { showLink(snap); poll(); } })
        .catch(() => { /* the panel still offers linking */ });
    return refreshStatus();
}
