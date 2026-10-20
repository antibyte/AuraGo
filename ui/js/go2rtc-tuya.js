(function () {
    'use strict';

    const prefix = 'common.go2rtc_tuya.';
    const secretFields = ['email', 'password', 'client_id', 'client_secret', 'uid'];
    const regions = { smart: ['eu-central', 'eu-east', 'us-west', 'us-east', 'china', 'india'], cloud: ['eu-central', 'eu-west', 'us-west', 'us-east', 'china', 'india', 'singapore'] };
    const escape = value => String(value == null ? '' : value).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
    const slug = value => String(value || '').normalize('NFKD').toLowerCase().replace(/[^a-z0-9_-]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 48);

    function open(options) {
        const translate = (key, params) => options.t(prefix + key, params || {});
        const controller = new AbortController();
        const dialog = document.createElement('dialog');
        dialog.className = 'go2rtc-tuya-dialog';
		dialog.setAttribute('aria-label', translate('title'));
        const state = { tab: options.mode === 'setup' ? 'setup' : 'accounts', accounts: [], editing: '', drafts: new Map(), account: '', device: '', candidates: [], selected: '', token: '', main: 'hd', sub: 'sd', name: '', id: '', tests: {}, busy: false, error: '', notice: '', closed: false };
        const account = () => state.accounts.find(a => a.id === state.account);
        const field = (key, label, value, type, disabled) => '<label>' + escape(translate(label || key)) + '<input data-tuya-field="' + key + '" type="' + (type || 'text') + '" value="' + escape(value) + '" ' + (disabled ? 'disabled ' : '') + (type === 'password' ? 'autocomplete="new-password"' : 'autocomplete="off"') + '></label>';
        const button = (action, label, disabled) => '<button type="button" data-tuya-action="' + action + '" ' + ((disabled || state.busy) && action !== 'close' ? 'disabled' : '') + '>' + escape(translate(label || action)) + '</button>';

        async function request(path, method, body) {
            const init = { method: method || 'GET', signal: controller.signal, headers: { 'Content-Type': 'application/json' } };
            if (body !== undefined) init.body = JSON.stringify(body);
            const value = options.api ? await options.api(path, init) : await fetch(path, Object.assign({ credentials: 'same-origin' }, init)).then(async r => {
                const data = await r.json();
                if (!r.ok) { const error = new Error(data.message || translate('request_failed')); error.body = data; throw error; }
                return data;
            });
            if (state.closed) throw new DOMException('Closed', 'AbortError');
            return value;
        }

        function resetSelection() { state.device = ''; state.selected = ''; state.token = ''; state.candidates = []; state.tests = {}; }
        function testedSelection() { return !!(state.tests[state.main] && state.tests[state.main].status === 'ok' && (!state.sub || state.tests[state.sub] && state.tests[state.sub].status === 'ok')); }
        function draft() {
            const key = state.editing || '+';
            if (!state.drafts.has(key)) state.drafts.set(key, Object.assign({ id: '', name: '', method: 'smart', region: '', email: '', password: '', client_id: '', client_secret: '', uid: '' }, state.accounts.find(a => a.id === key) || {}));
            return state.drafts.get(key);
        }
        function regionOptions(d) {
            return '<option value="">' + escape(translate('choose_region')) + '</option>' + regions[d.method].map(value => '<option value="' + value + '" ' + (d.region === value ? 'selected' : '') + '>' + escape(translate('region_' + value.replaceAll('-', '_'))) + '</option>').join('');
        }
        function accountForm() {
            const d = draft();
            let html = '<div class="gt-form">' + field('draft.id', 'account_id', d.id, 'text', !!state.editing) + field('draft.name', 'account_name', d.name);
            html += '<label>' + escape(translate('method')) + '<select data-tuya-field="draft.method"><option value="smart" ' + (d.method === 'smart' ? 'selected' : '') + '>Tuya Smart</option><option value="cloud" ' + (d.method === 'cloud' ? 'selected' : '') + '>Tuya Cloud</option></select></label>';
            html += '<label>' + escape(translate('region')) + '<select data-tuya-field="draft.region">' + regionOptions(d) + '</select></label>';
            for (const key of d.method === 'smart' ? ['email', 'password'] : ['client_id', 'client_secret', 'uid']) html += field('draft.' + key, key, d[key], key === 'email' ? 'email' : 'password');
            html += '</div><p>' + escape(translate('keep_credentials')) + '</p><p>' + escape(translate(d.method === 'smart' ? 'smart_help' : 'cloud_help')) + '</p>';
            html += '<div class="gt-actions">' + button('save_account') + button('check_account', null, !state.editing) + (state.editing ? button('delete_account') : '') + '</div>';
            return html;
        }
        function setupForm() {
            const a = account();
            let html = '<div class="gt-form"><label>' + escape(translate('account')) + '<select data-tuya-field="account"><option value="">' + escape(translate('choose_account')) + '</option>' + state.accounts.map(a => '<option value="' + escape(a.id) + '" ' + (a.id === state.account ? 'selected' : '') + '>' + escape(a.name) + '</option>').join('') + '</select></label></div>';
            if (!a) return html + '<p>' + escape(translate('account_first')) + '</p>';
            html += '<p>' + escape(translate(a.method === 'smart' ? 'smart_help' : 'cloud_help')) + '</p>';
            if (a.method === 'smart') {
                html += button('discover') + '<label>' + escape(translate('camera')) + '<select data-tuya-field="selected"><option value="">' + escape(translate('choose_camera')) + '</option>' + state.candidates.map(c => '<option value="' + escape(c.id) + '" ' + (c.id === state.selected ? 'selected' : '') + '>' + escape(c.name || translate('camera')) + '</option>').join('') + '</select></label>';
            } else html += field('device', 'device_id', state.device, 'password');
            html += '<div class="gt-form"><label>' + escape(translate('main')) + '<select data-tuya-field="main"><option value="hd" ' + (state.main === 'hd' ? 'selected' : '') + '>HD</option><option value="sd" ' + (state.main === 'sd' ? 'selected' : '') + '>SD</option></select></label><label>' + escape(translate('sub')) + '<select data-tuya-field="sub"><option value="">' + escape(translate('none')) + '</option>' + (state.main !== 'sd' ? '<option value="sd" ' + (state.sub === 'sd' ? 'selected' : '') + '>SD</option>' : '') + '</select></label></div>';
            html += '<div class="gt-actions">' + button('test_hd') + button('test_sd') + '</div>';
            for (const quality of ['hd', 'sd']) {
                const result = state.tests[quality];
                if (result) html += '<p class="gt-result" data-tuya-result="' + quality + '">' + quality.toUpperCase() + ': ' + escape(result.status === 'ok' ? translate('test_ok', { width: result.width, height: result.height }) + ' · ' + (result.video_codecs || []).join(', ') : translate(result.error_code || 'request_failed')) + '</p>';
            }
            if (state.tests.hd && state.tests.hd.status !== 'ok') html += button('use_sd');
            html += '<div class="gt-form">' + field('name', 'camera_name', state.name) + field('id', 'camera_id', state.id) + '</div>';
            return html + button('create_camera', null, !testedSelection()) + '<p>' + escape(translate('cloud_notice')) + '</p>';
        }
        function draw() {
            if (state.closed) return;
            dialog.innerHTML = '<header><h2>' + escape(translate('title')) + '</h2>' + button('close') + '</header><nav aria-label="' + escape(translate('title')) + '">' + button('accounts', 'accounts') + button('setup', 'add_camera') + '</nav><div class="gt-body">' + (state.tab === 'accounts' ? '<div class="gt-account-list">' + state.accounts.map(a => '<button type="button" data-tuya-edit="' + escape(a.id) + '">' + escape(a.name) + ' · ' + a.camera_count + '</button>').join('') + button('new_account') + '</div>' + accountForm() : setupForm()) + '<p class="gt-status" role="status" aria-live="polite">' + escape(state.busy ? translate('working') : state.notice) + '</p><p class="gt-error" role="alert">' + escape(state.error) + '</p></div>';
            dialog.querySelectorAll('[data-tuya-field]').forEach(el => el.addEventListener(el.tagName === 'SELECT' ? 'change' : 'input', () => {
                const key = el.dataset.tuyaField;
                if (key.startsWith('draft.')) { draft()[key.slice(6)] = el.value; if (key === 'draft.method') { if (!regions[draft().method].includes(draft().region)) draft().region = ''; for (const field of secretFields) draft()[field] = ''; draw(); } }
                else {
                    state[key] = el.value;
                    if (key === 'account') { resetSelection(); draw(); }
                    if (key === 'selected' || key === 'device') { state.tests = {}; if (key === 'device') { state.token = ''; dialog.querySelectorAll('[data-tuya-result],[data-tuya-action="use_sd"]').forEach(el => el.remove()); } const selected = state.candidates.find(c => c.id === state.selected); if (selected && !state.name) { state.name = selected.name; state.id = slug(state.name); } if (key === 'selected') draw(); }
                    if (key === 'main') { if (state.main === 'sd') state.sub = ''; draw(); }
                    const create = dialog.querySelector('[data-tuya-action="create_camera"]');
                    if (create) create.disabled = state.busy || !testedSelection();
                }
            }));
            dialog.querySelectorAll('[data-tuya-action]').forEach(el => el.addEventListener('click', () => action(el.dataset.tuyaAction)));
            dialog.querySelectorAll('[data-tuya-edit]').forEach(el => el.addEventListener('click', () => { state.editing = el.dataset.tuyaEdit; state.error = ''; draw(); }));
			if (state.busy) dialog.querySelectorAll('input,select,[data-tuya-edit]').forEach(el => { el.disabled = true; });
        }
        function cameraInput() { return { account_id: state.account, setup_token: state.token, candidate_id: state.token ? state.selected : '', device_id: state.token ? '' : state.device }; }
        async function action(name) {
            if (name === 'close') return close();
            if (state.busy) return;
            state.error = ''; state.notice = '';
            if (name === 'accounts' || name === 'setup') { state.tab = name; return draw(); }
            if (name === 'new_account') { state.editing = ''; state.drafts.delete('+'); return draw(); }
            if (name === 'use_sd') { state.main = 'sd'; state.sub = ''; return draw(); }
            if (name === 'check_account') {
                const d = draft(); const saved = state.accounts.find(a => a.id === state.editing);
                if (secretFields.some(k => !!d[k]) || ['id', 'name', 'method', 'region'].some(k => d[k] !== saved[k])) { state.error = translate('save_before_check'); return draw(); }
                state.account = saved.id; resetSelection(); state.tab = 'setup';
                if (saved.method === 'cloud') { state.notice = translate('device_required'); return draw(); }
                name = 'discover';
            }
            state.busy = true; draw();
            try {
                if (name === 'save_account') {
                    const d = draft(); const body = {};
                    for (const key of ['id', 'name', 'method', 'region', ...secretFields]) body[key] = d[key] || '';
                    const result = await request('/api/go2rtc/tuya/accounts' + (state.editing ? '/' + encodeURIComponent(state.editing) : ''), state.editing ? 'PATCH' : 'POST', body);
                    state.accounts = result.accounts || []; state.account = d.id; state.editing = d.id; state.drafts.clear(); resetSelection();
                    state.notice = translate(result.runtime_reconciled === false ? 'saved_pending' : 'saved');
                    if (options.onChanged) options.onChanged(result);
                } else if (name === 'delete_account') {
                    const result = await request('/api/go2rtc/tuya/accounts/' + encodeURIComponent(state.editing), 'DELETE');
                    state.accounts = result.accounts || []; state.drafts.delete(state.editing); if (state.account === state.editing) { state.account = ''; resetSelection(); } state.editing = ''; state.notice = translate('deleted');
                    if (options.onChanged) options.onChanged(result);
                } else if (name === 'discover') {
                    const result = await request('/api/go2rtc/tuya/discover', 'POST', { account_id: state.account });
                    if (result.status !== 'ok') throw new Error(translate(result.error_code || 'request_failed'));
                    state.candidates = result.candidates || []; state.token = result.setup_token; state.selected = ''; state.tests = {};
                    state.notice = translate(result.incomplete ? 'incomplete' : !state.candidates.length ? 'no_cameras' : 'choose_camera');
                } else if (name === 'test_hd' || name === 'test_sd') {
                    const quality = name.slice(5);
                    const result = await request('/api/go2rtc/tuya/test', 'POST', Object.assign(cameraInput(), { resolution: quality }));
                    state.tests[quality] = result;
                    if (result.status === 'ok' && result.candidates && result.candidates.length) { state.token = result.setup_token; state.selected = result.candidates[0].id; }
                } else if (name === 'create_camera') {
                    const result = await request('/api/go2rtc/streams', 'POST', { id: state.id, name: state.name, tuya: Object.assign(cameraInput(), { main_resolution: state.main, sub_resolution: state.sub }) });
                    if (options.onChanged) options.onChanged(result);
                    if (options.onCreated) options.onCreated(result);
                    return close();
                }
            } catch (error) {
                if (state.closed || error.name === 'AbortError') return;
                const code = error.body && (error.body.code || error.body.error_code);
                const localized = code && translate(code);
                state.error = localized && localized !== prefix + code ? localized : error.message;
				if (error.body && Array.isArray(error.body.cameras) && error.body.cameras.length) state.error += ' ' + error.body.cameras.join(', ');
            } finally { state.busy = false; draw(); }
        }
        function close() {
            if (state.closed) return;
            state.closed = true; controller.abort(); state.drafts.clear(); state.device = ''; dialog.close(); dialog.remove();
            if (options.onClose) options.onClose();
        }
        dialog.addEventListener('cancel', e => { e.preventDefault(); close(); });
        document.body.appendChild(dialog); draw(); dialog.showModal();
        request('/api/go2rtc/tuya/accounts').then(result => { state.accounts = result.accounts || []; if (!state.account && state.accounts.length) state.account = state.accounts[0].id; draw(); }).catch(error => { if (!state.closed) { state.error = error.message; draw(); } });
        return { close };
    }
    window.AuraGoTuya = Object.freeze({ open });
})();
