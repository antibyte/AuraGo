(function () {
    'use strict';
    const dialogs = new WeakMap();
    const clone = value => structuredClone(value);
    const esc = value => String(value ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
    const MAX_IMAGE = 10 * 1024 * 1024, MAX_CSV = 5 * 1024 * 1024;
    const IMAGE_TYPES = /^(image\/(png|jpeg|gif|webp|svg\+xml))$/;
    const SCAN_TYPES = /^(image\/(png|jpeg|gif|webp))$/;
    const field = (key, kind = 'text', extra = {}) => ({ key, kind, label: key.split('.').at(-1), ...extra });
    const templates = {
        text: [field('text', 'textarea')], url: [field('url', 'url')],
        email: [field('to', 'list'), field('subject'), field('body', 'textarea'), field('cc', 'list'), field('bcc', 'list')],
        telephone: [field('number', 'tel')], sms: [field('number', 'tel'), field('body', 'textarea')],
        wifi: [field('ssid'), field('password', 'password'), field('security', 'select', { options: ['WPA', 'WEP', 'nopass'] }), field('hidden', 'checkbox', { label: 'hidden_network' })],
        vcard: [field('version', 'select', { options: ['2.1', '3.0', '4.0'] }), field('first_name'), field('last_name'), field('formatted_name'),
            field('middle_name'), field('prefix'), field('suffix'), field('organization'), field('title', 'text', { label: 'job_title' }), field('phone', 'tel'),
            field('phone_work', 'tel'), field('phone_private', 'tel'), field('phone_mobile', 'tel'),
            field('email', 'email'), field('url', 'url'), field('birthday', 'date'), field('address.street'), field('address.city'), field('address.region'),
            field('address.postal_code'), field('address.country'), field('note', 'textarea')],
        location: [field('latitude', 'number', { min: -90, max: 90, step: 'any' }), field('longitude', 'number', { min: -180, max: 180, step: 'any' })],
        calendar: [field('title', 'text', { label: 'event_title' }), field('all_day', 'checkbox'), field('start'), field('end'),
            field('timezone'), field('location'), field('description', 'textarea')],
        epc: [field('version', 'select', { options: ['001', '002'] }), field('name', 'text', { label: 'recipient' }), field('iban'), field('bic'),
            field('amount', 'text', { inputmode: 'decimal' }), field('purpose'), field('reference'), field('text'), field('information')]
    };
    const styleFields = [
        field('size', 'number', { min: 64, max: 4096, step: 1 }), field('quiet_zone', 'number', { min: 0, max: 32, step: 1 }),
        field('error_correction', 'select', { options: ['L', 'M', 'Q', 'H'] }), field('module_shape', 'select', { catalog: 'module_shapes' }),
        field('finder_shape', 'select', { catalog: 'finder_shapes' }), field('center_shape', 'select', { catalog: 'center_shapes' }),
        field('module_color', 'color'), field('finder_color', 'color'), field('center_color', 'color'), field('background_color', 'color'),
        field('background_radius', 'number', { min: 0, max: 2048, step: 1 })
    ];
    const frameFields = [field('text', 'textarea', { label: 'caption' }), field('font', 'select', { catalog: 'fonts' }),
        field('font_size', 'number', { min: 6, max: 256, step: 1 }), field('side_width', 'number', { min: 20, max: 2048, step: 1 }),
        field('text_color', 'color'), field('background_color', 'color', { label: 'frame_background' }), field('border_color', 'color'),
        field('border_width', 'number', { min: 0, max: 256, step: 1 }), field('border_radius', 'number', { min: 0, max: 2048, step: 1 }),
        field('padding', 'number', { min: 0, max: 512, step: 1 })];
    const logoFields = [field('size_percent', 'number', { label: 'logo_size', min: 1, max: 30, step: 1 }),
        field('margin', 'number', { label: 'logo_margin', min: 0, max: 128, step: 1 }), field('hide_background_modules', 'checkbox', { label: 'hide_modules' })];
    const outputFields = [field('quality', 'number', { min: 30, max: 100, step: 1 }), field('jpeg_background', 'color'),
        field('text_mode', 'select', { options: ['ascii', 'unicode', 'unicode_half'] })];
    const defaultStyle = () => ({ size: 512, quiet_zone: 4, error_correction: 'M', module_shape: 'square', finder_shape: 'square', center_shape: 'square',
        module_color: '#152235', finder_color: '#152235', center_color: '#152235', background_color: '#ffffff', background_radius: 0 });
    const defaultFrame = () => ({ position: 'bottom', text: '', font: 'sans', font_size: 24, side_width: 160, text_color: '#152235',
        background_color: '#ffffff', border_color: '#152235', border_width: 0, border_radius: 0, padding: 16 });
    const defaultContent = type => type === 'text' || type === 'url' ? { type, [type]: '' } : { type, [type]: {
        ...(type === 'wifi' ? { security: 'WPA', hidden: false } : {}), ...(type === 'vcard' ? { version: '3.0' } : {}),
        ...(type === 'epc' ? { version: '002' } : {}), ...(type === 'calendar' ? { created_at: new Date().toISOString(), timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC', all_day: false } : {})
    } };
    const get = (obj, path) => path.split('.').reduce((value, key) => value?.[key], obj);
    function set(obj, path, value) {
        const parts = path.split('.'), last = parts.pop();
        for (const key of parts) obj = obj[key] ||= {};
        obj[last] = value;
    }
    function savedView() {
        try {
            const data = JSON.parse(localStorage.getItem('aurago.qr.view.v1') || '{}');
            return { full: data.full === true, pins: Array.isArray(data.pins) ? data.pins.filter(key => typeof key === 'string') : ['style.module_color', 'style.error_correction', 'frame.text'] };
        } catch (_) { return { full: false, pins: ['style.module_color', 'style.error_correction', 'frame.text'] }; }
    }
    async function checked(response) {
        if (response.ok) return response;
        const body = await response.json().catch(() => ({}));
        const error = new Error(body.error || 'QR'); error.code = body.code || 'http_' + response.status;
        error.details = body.details; throw error;
    }
    async function resource(file, signal) {
        if (!file || !IMAGE_TYPES.test(file.type) || file.size > MAX_IMAGE) throw Object.assign(new Error('image'), { code: 'resource_invalid' });
        return new Promise((resolve, reject) => {
            const reader = new FileReader(), abort = () => reader.abort();
            const done = () => signal.removeEventListener('abort', abort);
            reader.onload = () => { done(); resolve({ mime: file.type, data: String(reader.result).split(',')[1] }); };
            reader.onerror = () => { done(); reject(reader.error); };
            reader.onabort = () => { done(); reject(new DOMException('Aborted', 'AbortError')); };
            if (signal.aborted) { reject(new DOMException('Aborted', 'AbortError')); return; }
            signal.addEventListener('abort', abort, { once: true }); reader.readAsDataURL(file);
        });
    }
    function open(options) {
        const host = options.host || document.body;
        dialogs.get(host)?.close();
        const life = new AbortController(), signal = life.signal, urls = new Set(), timers = new Set(), view = savedView();
        const tr = (key, params) => options.t?.('desktop.qr_' + key, params) || key;
        const common = key => options.t?.('desktop.' + key) || key;
        const id = 'qr-' + crypto.randomUUID(), restore = document.activeElement;
        const node = document.createElement('dialog'); node.className = 'vd-qr-dialog'; node.setAttribute('aria-labelledby', id);
        const icon = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><path d="M3 3h6v6H3zM15 3h6v6h-6zM3 15h6v6H3zM15 15h2v2h-2zM19 19h2v2h-2zM19 13h2v3M13 19v2h3M3 12h5M12 3v5M12 11h4v1h5M12 13v3"/></svg>';
        node.innerHTML = '<header class="vd-qr-head">' + icon + '<div><h2 id="' + id + '">' + esc(tr('title')) + '</h2><p>' + esc(tr('subtitle')) + '</p></div>' +
            '<button type="button" class="vd-qr-close" data-action="close" aria-label="' + esc(common('close')) + '">×</button></header>' +
            '<nav class="vd-qr-tabs" role="tablist">' + ['content', 'design', 'batch', 'scan'].map((key, i) => '<button type="button" role="tab" id="' + id + '-' + key + '" aria-controls="' + id + '-panel" aria-selected="' + (i === 0) + '" tabindex="' + (i ? '-1' : '0') + '" data-tab="' + key + '">' + esc(tr(key)) + '</button>').join('') + '</nav>' +
            '<div class="vd-qr-body"><section class="vd-qr-settings" id="' + id + '-panel" role="tabpanel" aria-labelledby="' + id + '-content" data-settings></section>' +
            '<aside class="vd-qr-preview-panel"><h3>' + esc(tr('preview')) + '</h3><div class="vd-qr-preview" data-preview aria-busy="false"><span class="vd-qr-preview-empty">' + esc(tr('no_preview')) + '</span></div><p class="vd-qr-meta" data-meta></p>' +
            '<ul class="vd-qr-diagnostics" data-diagnostics></ul><fieldset class="vd-qr-insertion" data-insertion><legend>' + esc(tr('insertion_size')) + '</legend></fieldset></aside></div>' +
            '<div class="vd-qr-status" role="status" aria-live="polite" data-status></div><footer class="vd-qr-footer"><select data-format aria-label="' + esc(tr('output')) + '"></select>' +
            '<button type="button" data-action="download" data-output>' + esc(tr('export')) + '</button><button type="button" data-action="copy" data-output>' + esc(tr('copy_output')) + '</button>' +
                '<button type="button" data-action="insert" class="vd-qr-primary" data-output' + (options.onInsert ? '' : ' hidden') + '>' + esc(tr('insert')) + '</button></footer>';
        host.append(node);
        const find = selector => node.querySelector(selector), settings = find('[data-settings]');
        let dead = false, catalog, spec = { schema_version: 1, content: defaultContent('text'), style: defaultStyle(), output: { format: 'png', quality: 92, jpeg_background: '#ffffff', text_mode: 'unicode_half' } };
        let tab = 'content', revision = 0, previewRevision = -1, preview, previewMeta, previewController, operation, resourceController, scanController, previewTimer, cameraTimer, stream;
        let frameDraft = defaultFrame(), logoDraft = { size_percent: 20, margin: 4, hide_background_modules: true }, drafts = {}, batchCSV = '', batchType = 'text', batchResult, batchRevision = -1;
        let cameraGeneration = 0, scanText = '', activeScan = false, resourceGeneration = 0, csvGeneration = 0, busy = false, resourceBusy = false, resolveClose;
        let presetID = '', framePresetID = '', emailBinding;
        const invalidFields = new Set();
        const insertion = { unit: options.insertion?.unit || 'mm', width: options.insertion?.width || 40, x: options.insertion?.x, y: options.insertion?.y, ...options.insertion };
        const closed = new Promise(resolve => { resolveClose = resolve; });
        const handle = { close, closed, get spec() { return clone(spec); } };
        dialogs.set(host, handle);
        const current = () => !dead && !signal.aborted && (!options.isCurrent || options.isCurrent());
        function status(key, error = false, params) { if (!dead) { find('[data-status]').textContent = key ? tr(key, params) : ''; find('[data-status]').dataset.error = String(error); } }
        function failure(error) {
            if (dead || error?.name === 'AbortError') return;
            status(error?.code === 'resource_invalid' ? 'resource_invalid' : error?.code === 'stale' ? 'stale' : 'failed', true);
        }
        function trackURL(blob) { const url = URL.createObjectURL(blob); urls.add(url); return url; }
        function releaseURL(url) { if (url && urls.delete(url)) URL.revokeObjectURL(url); }
        function close() {
            if (dead) return;
            dead = true; life.abort(); previewController?.abort(); operation?.abort(); resourceController?.abort(); scanController?.abort();
            emailBinding?.dispose();
            clearTimeout(previewTimer); timers.forEach(timer => clearTimeout(timer)); timers.clear(); stopCamera(); urls.forEach(url => URL.revokeObjectURL(url)); urls.clear();
            drafts = {}; batchCSV = ''; scanText = ''; spec = null; preview = null; node.replaceChildren();
            if (node.open) node.close(); node.remove(); if (dialogs.get(host) === handle) dialogs.delete(host);
            options.signal?.removeEventListener('abort', close); resolveClose();
            if (restore?.isConnected) restore.focus({ preventScroll: true });
        }
        function rememberView() {
            try { localStorage.setItem('aurago.qr.view.v1', JSON.stringify({ full: view.full, pins: view.pins })); } catch (_) { /* Presentation preferences are optional. */ }
        }
        function contentObject() { return ['text', 'url'].includes(spec.content.type) ? spec.content : spec.content[spec.content.type]; }
        function valueAt(scope, key) { return get(scope === 'content' ? contentObject() : scope === 'style' ? spec.style : scope === 'frame' ? frameDraft : scope === 'logo' ? logoDraft : spec.output, key); }
        function optionLabel(value, def) {
            if (def.catalog === 'fonts') return catalog.fonts.find(font => font.id === value)?.name || value;
            if (def.key === 'text_mode') return value === 'ascii' ? 'ASCII' : tr('format_' + value);
            if (def.key === 'error_correction' || def.key === 'version' || ['WPA', 'WEP'].includes(value)) return value;
            return tr((def.key === 'security' ? 'security_' : def.key === 'position' ? 'position_' : 'shape_') + value.replaceAll('-', '_'));
        }
        function control(def, scope, pinnable = false) {
            const name = scope + '.' + def.key, value = valueAt(scope, def.key), pinned = view.pins.includes(name);
            const pin = pinnable ? '<button type="button" class="vd-qr-pin" data-pin="' + name + '" aria-pressed="' + pinned + '" aria-label="' + esc(tr(pinned ? 'unpin' : 'pin')) + '">' + (pinned ? '★' : '☆') + '</button>' : '';
            const label = '<span class="vd-qr-label">' + esc(tr(def.label)) + pin + '</span>';
            let input;
            if (def.kind === 'select') {
                const values = def.options || (def.catalog === 'fonts' ? catalog.fonts.map(font => font.id) : catalog[def.catalog] || []);
                input = '<select data-field="' + name + '" aria-label="' + esc(tr(def.label)) + '">' + values.map(item => '<option value="' + esc(item) + '"' + (item === value ? ' selected' : '') + '>' + esc(optionLabel(item, def)) + '</option>').join('') + '</select>';
            } else if (def.kind === 'checkbox') input = '<span class="vd-qr-check"><input type="checkbox" data-field="' + name + '"' + (value ? ' checked' : '') + '><span>' + esc(tr(def.label)) + '</span></span>';
            else if (def.kind === 'textarea') input = '<textarea data-field="' + name + '" rows="3" autocomplete="off" spellcheck="false">' + esc(value || '') + '</textarea>';
            else {
                const kind = def.kind === 'list' || def.kind === 'color' ? 'text' : def.kind;
                const data = Array.isArray(value) ? value.join(', ') : value ?? '';
                input = '<input type="' + kind + '" data-field="' + name + '" value="' + esc(data) + '" aria-label="' + esc(tr(def.label)) + '" autocomplete="off"' +
                    (def.min !== undefined ? ' min="' + def.min + '"' : '') + (def.max !== undefined ? ' max="' + def.max + '"' : '') + (def.step !== undefined ? ' step="' + def.step + '"' : '') +
                    (def.inputmode ? ' inputmode="' + def.inputmode + '"' : '') + (def.kind === 'color' ? ' pattern="#[0-9a-fA-F]{6}([0-9a-fA-F]{2})?" placeholder="#ffffff" spellcheck="false"' : '') + '>';
                if (def.kind === 'color') input = '<span class="vd-qr-color"><input type="color" data-color-picker="' + name + '" value="' + esc(String(data).slice(0, 7)) + '" aria-label="' + esc(tr(def.label)) + '">' + input + '</span>';
            }
            return '<div class="vd-qr-field' + (['textarea', 'checkbox'].includes(def.kind) ? ' vd-qr-wide' : '') + '"' + (pinnable ? ' data-design-field="' + name + '"' : '') + '><label>' + (def.kind === 'checkbox' ? '' : label) + input + '</label>' + (def.kind === 'checkbox' ? pin : '') + '</div>';
        }
        function group(title, html) { return '<fieldset class="vd-qr-group"><legend>' + esc(tr(title)) + '</legend>' + html + '</fieldset>'; }
        function buttons(actions) { return '<div class="vd-qr-inline">' + actions.map(action => '<button type="button" data-action="' + action + '">' + esc(tr(action)) + '</button>').join('') + '</div>'; }
        function resourceControls(kind) {
            return '<div class="vd-qr-field vd-qr-wide"><label><span class="vd-qr-label">' + esc(tr(kind === 'logo' ? 'logo' : 'background_image')) + '</span><span class="vd-qr-resource"><input type="file" accept="image/png,image/jpeg,image/webp,image/gif,image/svg+xml" data-resource="' + kind + '"><button type="button" data-remove-resource="' + kind + '" aria-label="' + esc(common('remove')) + '">×</button></span></label>' +
                '<div class="vd-qr-inline"><input type="url" data-image-url="' + kind + '" autocomplete="off" placeholder="https://" aria-label="' + esc(tr('image_url')) + '"><button type="button" data-import-url="' + kind + '">' + esc(tr('import_url')) + '</button></div></div>';
        }
        function renderControls() {
            if (!catalog || dead) return;
            if (tab !== 'scan') stopCamera();
            settings.setAttribute('aria-labelledby', id + '-' + tab);
            if (tab === 'content') {
                const type = spec.content.type;
                settings.innerHTML = '<div class="vd-qr-field"><label><span class="vd-qr-label">' + esc(tr('type')) + '</span><select data-content-type>' + catalog.content_types.map(key => '<option value="' + esc(key) + '"' + (key === type ? ' selected' : '') + '>' + esc(tr('type_' + key)) + '</option>').join('') + '</select></label></div>' +
                    '<div class="vd-qr-fields vd-qr-group">' + templates[type].map(def => control(type === 'calendar' && ['start', 'end'].includes(def.key) && contentObject().all_day ? { ...def, kind: 'date' } : def, 'content')).join('') + '</div>' +
                    (type === 'epc' ? '<p class="vd-qr-help">' + esc(tr('epc_hint')) + '</p>' : '') +
                    (type === 'calendar' ? '<p class="vd-qr-help">' + esc(tr('calendar_hint')) + '</p>' : '') +
                    group('pinned', '<div class="vd-qr-fields">' + styleFields.filter(def => view.pins.includes('style.' + def.key)).map(def => control(def, 'style')).join('') +
                    (spec.frame ? frameFields.filter(def => view.pins.includes('frame.' + def.key)).map(def => control(def, 'frame')).join('') : '') +
                    logoFields.filter(def => view.pins.includes('logo.' + def.key)).map(def => control(def, 'logo')).join('') +
                    outputFields.filter(def => view.pins.includes('output.' + def.key)).map(def => control(def, 'output')).join('') + '</div>');
            } else if (tab === 'design') {
                const optionsHTML = (items, prefix) => items.map(item => '<option value="' + esc(item.id) + '">' + esc(tr(prefix + item.id.replaceAll('-', '_'))) + '</option>').join('');
                settings.innerHTML = '<div class="vd-qr-view-toggle"><label class="vd-qr-check"><input type="checkbox" data-full' + (view.full ? ' checked' : '') + '>' + esc(tr('full')) + '</label><button type="button" data-action="random">' + esc(tr('random')) + '</button></div>' +
                    '<div class="vd-qr-field"><label><span class="vd-qr-label">' + esc(tr('presets')) + '</span><select data-preset><option value="">' + esc(tr('choose_preset')) + '</option>' + optionsHTML(catalog.presets, 'preset_') + '</select></label></div>' +
                    group('appearance', '<div class="vd-qr-fields">' + styleFields.map(def => control(def, 'style', true)).join('') + '</div><p class="vd-qr-help">' + esc(tr('color_help')) + '</p>') +
                    group('logo', '<div class="vd-qr-fields">' + resourceControls('logo') + logoFields.map(def => control(def, 'logo', true)).join('') + '</div>') +
                    group('frame', '<div class="vd-qr-fields">' + control(field('position', 'select', { options: ['none', ...catalog.frame_positions] }), 'frame') +
                    '<div class="vd-qr-field"><label><span class="vd-qr-label">' + esc(tr('frame_presets')) + '</span><select data-frame-preset><option value="">' + esc(tr('choose_preset')) + '</option>' + ['label', 'ticket', 'sidebar'].map(key => '<option value="' + key + '">' + esc(tr('frame_preset_' + key)) + '</option>').join('') + '</select></label></div>' +
                    frameFields.map(def => control(def, 'frame', true)).join('') + resourceControls('background') + '</div>') +
                    group('configuration', buttons(['save_config', 'load_config']) + '<input type="file" accept=".json,.qr.json,application/json" data-config-file hidden><p class="vd-qr-help">' + esc(tr('config_privacy')) + '</p>') +
                    group('output', '<div class="vd-qr-fields">' + outputFields.map(def => control(def, 'output', true)).join('') + '</div>');
                const position = settings.querySelector('[data-field="frame.position"]'); position.value = spec.frame ? frameDraft.position : 'none';
                settings.querySelector('[data-preset]').value = presetID;
                settings.querySelector('[data-frame-preset]').value = framePresetID;
                applySimpleView();
            } else if (tab === 'batch') {
                settings.innerHTML = '<div class="vd-qr-fields"><div class="vd-qr-field"><label><span class="vd-qr-label">' + esc(tr('csv_template')) + '</span><select data-batch-type>' + ['text', 'url', 'vcard'].map(type => '<option value="' + type + '"' + (type === batchType ? ' selected' : '') + '>' + esc(tr('type_' + type)) + '</option>').join('') + '</select></label></div>' +
                    '<div class="vd-qr-field"><label><span class="vd-qr-label">' + esc(tr('upload_csv')) + '</span><input type="file" accept=".csv,text/csv" data-csv-file></label></div>' +
                    '<div class="vd-qr-field vd-qr-wide"><label><span class="vd-qr-label">CSV</span><textarea rows="7" data-csv autocomplete="off" spellcheck="false">' + esc(batchCSV) + '</textarea></label></div></div>' +
                    '<p class="vd-qr-help">' + esc(tr('csv_help')) + '</p>' + buttons(['csv_template_download', 'validate_batch', 'export_zip', 'cancel_operation']) + '<div data-batch-results></div>';
                renderBatchResults();
            } else renderScanner();
        }
        function applySimpleView() {
            for (const el of settings.querySelectorAll('[data-design-field]')) el.hidden = !view.full && !view.pins.includes(el.dataset.designField);
            for (const scope of ['logo', 'background']) {
                const el = settings.querySelector('[data-resource="' + scope + '"]')?.closest('.vd-qr-field'); if (el) el.hidden = !view.full;
            }
        }
        function displayDiagnostics(target, diagnostics) {
            target.replaceChildren();
            for (const diagnostic of diagnostics || []) {
                const li = document.createElement('li'), key = 'warning_' + diagnostic.code.replaceAll('-', '_'), label = tr(key);
                li.textContent = label === 'desktop.qr_' + key || label === key ? tr('warning_style') : label; target.append(li);
            }
        }
        function insertionControls() {
            const fieldset = find('[data-insertion]'); fieldset.hidden = !options.onInsert;
            if (!options.onInsert) return;
            const pixel = insertion.unit === 'px';
            const names = pixel ? ['width', 'height', 'x', 'y'] : ['width', 'height'];
            fieldset.innerHTML = '<legend>' + esc(tr('insertion_size')) + '</legend>' + names.map(key => '<label class="vd-qr-field"><span>' + esc(tr((pixel ? 'insert_' : 'physical_') + key)) + '</span><input type="number" data-insert="' + key + '" min="' + (key === 'x' || key === 'y' ? 0 : pixel ? 1 : 1) + '" step="' + (pixel ? 1 : .1) + '"' + (key === 'height' ? ' readonly' : '') + '></label>').join('');
            updateInsertion();
        }
        function updateInsertion() {
            if (!options.onInsert) return;
            const ratio = previewMeta?.height / previewMeta?.width || 1, pixel = insertion.unit === 'px';
            insertion.height = pixel ? Math.max(1, Math.round(insertion.width * ratio)) : Math.round(insertion.width * ratio * 10) / 10;
            if (pixel) {
                if (insertion.x === undefined) insertion.x = Math.max(0, Math.floor((insertion.canvasWidth - insertion.width) / 2));
                if (insertion.y === undefined) insertion.y = Math.max(0, Math.floor((insertion.canvasHeight - insertion.height) / 2));
            }
            for (const control of node.querySelectorAll('[data-insert]')) if (document.activeElement !== control) control.value = insertion[control.dataset.insert];
        }
        function syncButtons() {
            const ready = current() && previewRevision === revision && !!preview && !busy && !resourceBusy;
            for (const button of node.querySelectorAll('[data-output]')) button.disabled = !ready;
            const batch = settings.querySelector('[data-action="export_zip"]'); if (batch) batch.disabled = busy || batchRevision !== revision || !batchResult || !!batchResult.errors?.length;
            const validate = settings.querySelector('[data-action="validate_batch"]'); if (validate) validate.disabled = busy || resourceBusy;
            const cancel = settings.querySelector('[data-action="cancel_operation"]'); if (cancel) cancel.disabled = !busy;
        }
        function changed(repaint = false) {
            revision++; previewRevision = -1; batchRevision = -1; batchResult = null; previewController?.abort();
            operation?.abort(); busy = false; clearTimeout(previewTimer); find('[data-preview]').setAttribute('aria-busy', 'true'); find('[data-preview]').dataset.stale = 'true';
            syncButtons(); if (repaint) renderControls();
            if (tab === 'batch') { renderBatchResults(); status('validate_batch'); }
            else previewTimer = setTimeout(() => void renderPreview(), 250);
        }
        async function renderBinary(snapshot, controller) {
            const response = await checked(await fetch('/api/qr/render', { method: 'POST', credentials: 'same-origin', cache: 'no-store',
                headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(snapshot), signal: controller.signal }));
            const blob = await response.blob();
            let meta = {}; try { meta = JSON.parse(response.headers.get('X-QR-Metadata') || '{}'); } catch (_) { /* Optional metadata must not invalidate image bytes. */ }
            return { blob, meta: { ...meta, width: Number(response.headers.get('X-QR-Width')) || meta.width, height: Number(response.headers.get('X-QR-Height')) || meta.height,
                verified: response.headers.get('X-QR-Verified') === 'true' || meta.verified === true }, snapshot };
        }
        async function renderPreview() {
            if (!catalog || !current()) { if (!dead) { status('stale', true); syncButtons(); } return; }
            if (invalidFields.size) { status('failed', true); find('[data-preview]').setAttribute('aria-busy', 'false'); return; }
            if (spec.content.type === 'text' && !spec.content.text) {
                const panel = find('[data-preview]'), placeholder = document.createElement('span'); releaseURL(panel.querySelector('img')?.src);
                placeholder.className = 'vd-qr-preview-empty'; placeholder.textContent = tr('no_preview'); panel.replaceChildren(placeholder); panel.setAttribute('aria-busy', 'false');
                preview = null; previewMeta = null; previewRevision = -1; find('[data-meta]').textContent = ''; find('[data-diagnostics]').replaceChildren();
                status('no_preview'); syncButtons(); return;
            }
            const token = revision, snapshot = clone(spec); snapshot.output.format = 'png';
            const controller = new AbortController(); previewController?.abort(); previewController = controller;
            status('busy');
            try {
                const result = await renderBinary(snapshot, controller);
                if (!current() || controller.signal.aborted || token !== revision) return;
                if (!result.meta.width || !result.meta.height || !result.meta.verified || !result.blob.type.startsWith('image/png')) throw new Error('render');
                const panel = find('[data-preview]'), old = panel.querySelector('img')?.src, image = document.createElement('img');
                image.alt = tr('preview'); image.src = trackURL(result.blob); panel.replaceChildren(image); releaseURL(old);
                preview = result.blob; previewMeta = result.meta; previewRevision = token; panel.dataset.stale = 'false';
                find('[data-meta]').textContent = tr('dimensions', { width: result.meta.width, height: result.meta.height }) + '\n' + tr(result.meta.verified ? 'verified' : 'not_verified') +
                    (result.meta.modules ? '\n' + tr('capacity', { version: result.meta.qr_version, modules: result.meta.modules }) : '');
                displayDiagnostics(find('[data-diagnostics]'), result.meta.diagnostics);
                updateInsertion(); status('ready');
            } catch (error) {
                if (token === revision && !controller.signal.aborted) { previewRevision = -1; failure(error); }
            } finally { if (!dead && token === revision) { find('[data-preview]').setAttribute('aria-busy', 'false'); syncButtons(); } }
        }
        function download(blob, name) {
            const url = trackURL(blob), link = document.createElement('a'); link.href = url; link.download = name; link.click();
            const timer = setTimeout(() => { timers.delete(timer); releaseURL(url); }, 1000); timers.add(timer);
        }
        async function output(action) {
            if (!current() || previewRevision !== revision || !preview || busy || resourceBusy) { status('stale', true); return; }
            const token = revision, snapshot = clone(spec), controller = new AbortController();
            operation?.abort(); operation = controller; busy = true; syncButtons(); status('busy');
            try {
                if (action === 'insert') {
                    if (![insertion.width, insertion.height].every(value => Number.isFinite(value) && value > 0)) throw new Error('size');
                    if (insertion.unit === 'px' && (![insertion.x, insertion.y].every(Number.isFinite) || insertion.x < 0 || insertion.y < 0 || insertion.x + insertion.width > insertion.canvasWidth || insertion.y + insertion.height > insertion.canvasHeight))
                        throw Object.assign(new Error('bounds'), { code: 'bounds' });
                    const result = { file: new File([preview], 'QR.png', { type: 'image/png' }), spec: snapshot, revision: token, width: previewMeta.width, height: previewMeta.height,
                        physicalWidthMM: insertion.unit === 'mm' ? insertion.width : insertion.width * 25.4 / 96, physicalHeightMM: insertion.unit === 'mm' ? insertion.height : insertion.height * 25.4 / 96,
                        insertWidth: insertion.width, insertHeight: insertion.height, x: insertion.x, y: insertion.y, signal: controller.signal };
                    if (!current() || token !== revision) return;
                    await options.onInsert(result); if (current() || !dead) close(); return;
                }
                const format = find('[data-format]').value;
                snapshot.output.format = format;
                const result = format === 'png' ? { blob: preview } : await renderBinary(snapshot, controller);
                if (!current() || controller.signal.aborted || revision !== token) return;
                const text = !['png', 'jpeg', 'svg'].includes(format), extension = format === 'markdown' ? 'md' : text ? 'txt' : format === 'jpeg' ? 'jpg' : format;
                if (action === 'copy') {
                    try {
                        if (text && navigator.clipboard?.writeText) await navigator.clipboard.writeText(await result.blob.text());
                        else if (!text && navigator.clipboard?.write && window.ClipboardItem) await navigator.clipboard.write([new ClipboardItem({ 'image/png': preview })]);
                        else throw new Error('clipboard');
                        if (!dead) status('copied');
                    } catch (_) { if (!dead) { download(result.blob, 'QR.' + extension); status('clipboard_unavailable'); } }
                } else { download(result.blob, 'QR.' + extension); status('downloaded'); }
            } catch (error) { if (error.code === 'bounds') status('insert_bounds', true); else if (error.code === 'insert_unreadable') status('insert_unreadable', true); else failure(error); }
            finally { if (!dead && operation === controller) { busy = false; syncButtons(); } }
        }
        async function importResource(kind, value) {
            const token = ++resourceGeneration, currentRevision = revision, controller = new AbortController();
            resourceController?.abort(); resourceController = controller; resourceBusy = true; syncButtons();
            try {
                status('busy');
                const image = typeof value === 'string' ? await (await checked(await fetch('/api/qr/import-image', { method: 'POST', credentials: 'same-origin', cache: 'no-store',
                    headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ url: value }), signal: controller.signal }))).json() : await resource(value, controller.signal);
                if (!current() || controller.signal.aborted || token !== resourceGeneration || revision !== currentRevision) return;
                if (!image?.mime || !IMAGE_TYPES.test(image.mime) || typeof image.data !== 'string' || image.data.length > MAX_IMAGE * 4 / 3 + 8) throw Object.assign(new Error('image'), { code: 'resource_invalid' });
                if (kind === 'logo') spec.logo = { ...clone(logoDraft), image };
                else { frameDraft.background_image = image; if (!catalog.frame_positions.includes(frameDraft.position)) frameDraft.position = 'bottom'; spec.frame = clone(frameDraft); }
                changed(); status('imported');
            } catch (error) { failure(error); }
            finally { if (!dead && resourceController === controller) { resourceBusy = false; syncButtons(); } }
        }
        async function importConfig(file) {
            if (!file || file.size > MAX_IMAGE) { status('config_invalid', true); return; }
            const token = revision, controller = new AbortController();
            try {
                if (!current() || token !== revision) return;
                operation?.abort(); operation = controller; busy = true; syncButtons(); status('busy');
                const normalized = await (await checked(await fetch('/api/qr/config', { method: 'POST', credentials: 'same-origin', cache: 'no-store',
                    headers: { 'Content-Type': 'application/json' }, body: file, signal: controller.signal }))).json();
                const check = clone(normalized); check.output.format = 'png';
                await renderBinary(check, controller);
                if (!current() || controller.signal.aborted || token !== revision) return;
                spec = normalized; frameDraft = spec.frame ? clone(spec.frame) : defaultFrame(); if (spec.logo) { logoDraft = clone(spec.logo); delete logoDraft.image; }
                invalidFields.clear(); drafts = {}; find('[data-format]').value = spec.output.format; renderControls(); changed(); status('imported');
            } catch (error) { if (error.name !== 'AbortError') status('config_invalid', true); }
            finally { if (!dead && operation === controller) { busy = false; syncButtons(); } }
        }
        function renderBatchResults() {
            const target = settings.querySelector('[data-batch-results]'); if (!target) return;
            target.replaceChildren(); if (!batchResult) return;
            const summary = document.createElement('p'); summary.className = 'vd-qr-help'; summary.textContent = tr('batch_summary', { total: batchResult.total, errors: batchResult.errors?.length || 0 }); target.append(summary);
            const table = document.createElement('table'); table.className = 'vd-qr-batch-table';
            const head = document.createElement('thead'); head.innerHTML = '<tr>' + ['row', 'filename', 'preview'].map(key => '<th scope="col">' + esc(tr(key)) + '</th>').join('') + '</tr>'; table.append(head);
            const body = document.createElement('tbody');
            const rows = [...(batchResult.entries || []).map(row => ({ ...row, error: false })), ...(batchResult.errors || []).map(row => ({ ...row, error: true }))].sort((a, b) => a.row - b.row);
            for (const row of rows) {
                const trNode = document.createElement('tr'); trNode.dataset.error = String(row.error);
                for (const value of [row.row, row.filename || row.field || '—']) { const cell = document.createElement('td'); cell.textContent = String(value); trNode.append(cell); }
                const cell = document.createElement('td');
                if (row.error) cell.textContent = tr('row_invalid') + (row.code ? ' (' + row.code + ')' : '');
                else if (row.preview?.mime === 'image/png' && typeof row.preview.data === 'string' && row.preview.data.length < 1024 * 1024) {
                    const image = document.createElement('img'); image.alt = tr('preview'); image.width = 80; image.src = 'data:image/png;base64,' + row.preview.data; cell.append(image);
                } else cell.textContent = tr('dimensions', { width: row.metadata?.width || 0, height: row.metadata?.height || 0 });
                trNode.append(cell); body.append(trNode);
            }
            table.append(body); const scroll = document.createElement('div'); scroll.className = 'vd-qr-batch-errors'; scroll.append(table); target.append(scroll); syncButtons();
        }
        async function batch(dryRun, asFile = false, exportSignal) {
            if (!current() || busy || !batchCSV || new TextEncoder().encode(batchCSV).length > MAX_CSV) { status('csv_invalid', true); return; }
            if (!dryRun && (batchRevision !== revision || batchResult?.errors?.length)) return;
            const token = revision, controller = new AbortController(), snapshot = clone(spec);
            const cancelExport = () => controller.abort();
            exportSignal?.addEventListener('abort', cancelExport, {once:true});
            if (exportSignal?.aborted) cancelExport();
            operation?.abort(); operation = controller; busy = true; syncButtons(); status('busy');
            try {
                const response = await checked(await fetch('/api/qr/batch', { method: 'POST', credentials: 'same-origin', cache: 'no-store',
                    headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ spec: snapshot, csv: batchCSV, content_type: batchType, dry_run: dryRun }), signal: controller.signal }));
                if (dryRun) {
                    const result = await response.json(); if (!current() || token !== revision || controller.signal.aborted) return;
                    batchResult = result; batchRevision = revision; renderBatchResults(); status(result.errors?.length ? 'row_invalid' : 'batch_ready', !!result.errors?.length);
                } else {
                    const blob = await response.blob(); if (!current() || token !== revision || controller.signal.aborted) return;
                    if (asFile) return [new File([blob], 'QR.zip', {type:'application/zip'})];
                    download(blob, 'QR.zip'); status('downloaded');
                }
            } catch (error) { if (asFile) throw error; if (controller.signal.aborted && !dead && token === revision) status('batch_cancelled'); else failure(error); }
            finally { exportSignal?.removeEventListener('abort',cancelExport); if (!dead && operation === controller) { busy = false; syncButtons(); } }
        }
        function renderScanner() {
            settings.innerHTML = '<div class="vd-qr-scan"><div class="vd-qr-drop" data-drop><p class="vd-qr-help">' + esc(tr('drop_image')) + '</p><input type="file" accept="image/png,image/jpeg,image/webp,image/gif" data-scan-file aria-label="' + esc(tr('scan_upload')) + '">' + buttons(['camera', 'clipboard']) + '</div>' +
                '<video class="vd-qr-camera" data-camera playsinline muted hidden></video><button type="button" data-action="camera_stop" hidden>' + esc(tr('camera_stop')) + '</button>' +
                '<p class="vd-qr-help">' + esc(tr('scan_safety')) + '</p><p data-scan-type></p><ul class="vd-qr-diagnostics" data-scan-diagnostics></ul><pre class="vd-qr-scan-result" data-scan-result' + (scanText ? '' : ' hidden') + '></pre>' + buttons(['copy_result', 'use_result']) + '</div>';
            find('[data-scan-result]').textContent = scanText;
        }
        function stopCamera() {
            cameraGeneration++; clearTimeout(cameraTimer); stream?.getTracks().forEach(track => track.stop()); stream = null;
            scanController?.abort(); activeScan = false;
            const video = settings.querySelector('[data-camera]'); if (video) { video.srcObject = null; video.hidden = true; }
            const stop = settings.querySelector('[data-action="camera_stop"]'); if (stop) stop.hidden = true;
        }
        async function scanImage(blob, camera = false) {
            if (!current() || activeScan || !blob) return;
            if (blob.size > MAX_IMAGE) { status('resource_invalid', true); return; }
            if (!SCAN_TYPES.test(blob.type)) { status('no_code', true); return; }
            const controller = new AbortController(), generation = cameraGeneration; scanController?.abort(); scanController = controller; activeScan = true;
            try {
                if (!camera) status('busy');
                const result = await (await checked(await fetch('/api/qr/scan', { method: 'POST', credentials: 'same-origin', cache: 'no-store', headers: { 'Content-Type': blob.type }, body: blob, signal: controller.signal }))).json();
                if (!current() || controller.signal.aborted || generation !== cameraGeneration || tab !== 'scan') return;
                scanText = String(result.text || '');
                find('[data-scan-result]').textContent = scanText; find('[data-scan-result]').hidden = !scanText;
                find('[data-scan-type]').textContent = tr('scan_type', { type: tr('type_' + (templates[result.type] ? result.type : 'text')) });
                displayDiagnostics(find('[data-scan-diagnostics]'), result.diagnostics);
                if (camera) stopCamera(); status('scan_found');
            } catch (error) { if (!camera && error.name !== 'AbortError') status('no_code', true); }
            finally { if (scanController === controller) activeScan = false; }
        }
        async function camera() {
            stopCamera(); if (!navigator.mediaDevices?.getUserMedia) { status('camera_unavailable', true); return; }
            const generation = cameraGeneration;
            try {
                const media = await navigator.mediaDevices.getUserMedia({ video: { facingMode: 'environment' }, audio: false });
                if (!current() || generation !== cameraGeneration || tab !== 'scan') { media.getTracks().forEach(track => track.stop()); return; }
                stream = media; const video = find('[data-camera]'); video.srcObject = media; video.hidden = false;
                find('[data-action="camera_stop"]').hidden = false; await video.play(); status('camera_hint');
                const canvas = document.createElement('canvas');
                const sample = async () => {
                    if (!current() || generation !== cameraGeneration || !stream || tab !== 'scan') return;
                    if (video.readyState >= 2 && !activeScan) {
                        const scale = Math.min(1, 960 / Math.max(video.videoWidth, video.videoHeight)); canvas.width = Math.max(1, Math.round(video.videoWidth * scale)); canvas.height = Math.max(1, Math.round(video.videoHeight * scale));
                        canvas.getContext('2d').drawImage(video, 0, 0, canvas.width, canvas.height);
                        const blob = await new Promise(resolve => canvas.toBlob(resolve, 'image/png'));
                        if (generation === cameraGeneration) await scanImage(blob, true);
                    }
                    if (generation === cameraGeneration) cameraTimer = setTimeout(sample, 700);
                };
                void sample();
            } catch (error) { if (!dead && generation === cameraGeneration) { stopCamera(); status('camera_denied', true); } }
        }
        async function scanClipboard() {
            if (!navigator.clipboard?.read) { status('clipboard_unavailable', true); return; }
            try {
                const items = await navigator.clipboard.read(); if (!current()) return;
                for (const item of items) { const mime = item.types.find(type => SCAN_TYPES.test(type)); if (mime) { stopCamera(); await scanImage(await item.getType(mime)); return; } }
                status('no_clipboard_image', true);
            } catch (_) { status('clipboard_unavailable', true); }
        }
        function switchTab(value) {
            if (value === tab) return;
            tab = value; for (const button of node.querySelectorAll('[data-tab]')) { const selected = button.dataset.tab === tab; button.setAttribute('aria-selected', String(selected)); button.tabIndex = selected ? 0 : -1; }
            renderControls(); syncButtons();
        }
        async function action(key) {
            if (key === 'close') return close();
            if (['insert', 'download', 'copy'].includes(key)) return output(key);
            if (key === 'save_config') {
                const token = revision, controller = new AbortController(); operation?.abort(); operation = controller; busy = true; syncButtons(); status('busy');
                try {
                    const response = await checked(await fetch('/api/qr/config', { method: 'POST', credentials: 'same-origin', cache: 'no-store', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(spec), signal: controller.signal }));
                    const blob = await response.blob(); if (current() && token === revision && !controller.signal.aborted) { download(blob, 'design.qr.json'); status('downloaded'); }
                } finally { if (!dead && operation === controller) { busy = false; syncButtons(); } }
                return;
            }
            if (key === 'load_config') return find('[data-config-file]').click();
            if (key === 'random') {
                const palettes = [['#172a46', '#ffffff'], ['#174a3b', '#f9f8ef'], ['#4f224a', '#fff5f8'], ['#3b2b16', '#fff8e8'], ['#182a32', '#edf9ff']];
                const random = new Uint32Array(2); crypto.getRandomValues(random); const palette = palettes[random[0] % palettes.length];
                Object.assign(spec.style, { module_color: palette[0], finder_color: palette[0], center_color: palette[0], background_color: palette[1], quiet_zone: 4,
                    module_shape: catalog.module_shapes[random[1] % catalog.module_shapes.length] }); changed(true); return;
            }
            if (key === 'validate_batch' || key === 'export_zip') return batch(key === 'validate_batch');
            if (key === 'csv_template_download') {
                const caption = '"' + tr('title').replaceAll('"', '""') + '"';
                const sample = batchType === 'vcard' ? 'version,first_name,last_name,phone_work,phone_private,phone_mobile,email,filename,caption,font\n3.0,Aura,Go,+12025550101,+12025550102,+12025550103,aura@example.invalid,qr-001,' + caption + ',sans\n' :
                    batchType + ',filename,caption,font\n' + (batchType === 'url' ? 'https://example.invalid/qr' : caption) + ',qr-001,' + caption + ',sans\n';
                download(new Blob([sample], { type: 'text/csv;charset=utf-8' }), 'QR-' + batchType + '.csv'); return;
            }
            if (key === 'cancel_operation') { operation?.abort(); return; }
            if (key === 'camera') return camera();
            if (key === 'camera_stop') { stopCamera(); status('ready'); return; }
            if (key === 'clipboard') return scanClipboard();
            if (key === 'copy_result') {
                if (!scanText) return;
                try { if (!navigator.clipboard?.writeText) throw new Error('clipboard'); await navigator.clipboard.writeText(scanText); status('copied'); }
                catch (_) { download(new Blob([scanText], { type: 'text/plain;charset=utf-8' }), 'QR-scan.txt'); status('clipboard_unavailable'); }
            }
            if (key === 'use_result' && scanText) { spec.content = { type: 'text', text: scanText }; switchTab('content'); changed(true); }
        }
        node.addEventListener('click', event => {
            event.stopPropagation();
            const button = event.target.closest('button'); if (!button || button.disabled) return;
            if (button.dataset.tab) switchTab(button.dataset.tab);
            else if (button.dataset.pin) {
                event.preventDefault(); const key = button.dataset.pin; view.pins = view.pins.includes(key) ? view.pins.filter(value => value !== key) : [...view.pins, key]; rememberView(); renderControls();
                const restored = settings.querySelector('[data-pin="' + key + '"]'); (restored?.offsetParent ? restored : settings.querySelector('[data-full]'))?.focus();
            } else if (button.dataset.importUrl) {
                const kind = button.dataset.importUrl, url = settings.querySelector('[data-image-url="' + kind + '"]').value.trim(); if (url) void importResource(kind, url);
            } else if (button.dataset.removeResource) {
                resourceGeneration++; resourceController?.abort(); resourceBusy = false;
                const picker = settings.querySelector('[data-resource="' + button.dataset.removeResource + '"]'); if (picker) picker.value = '';
                if (button.dataset.removeResource === 'logo') delete spec.logo; else { delete frameDraft.background_image; if (spec.frame) delete spec.frame.background_image; } changed();
            } else if (button.dataset.action) void action(button.dataset.action).catch(failure);
        }, { signal });
        node.addEventListener('input', event => {
            const el = event.target;
            if (el.dataset.colorPicker) {
                const target = settings.querySelector('[data-field="' + el.dataset.colorPicker + '"]'), alpha = /^#[\da-f]{8}$/i.test(target.value) ? target.value.slice(7) : '';
                target.value = el.value + alpha; target.dispatchEvent(new Event('input', { bubbles: true })); return;
            }
            if (el.dataset.field) {
                const [scope, ...path] = el.dataset.field.split('.'), key = path.join('.');
                const def = (scope === 'content' ? templates[spec.content.type] : scope === 'style' ? styleFields : scope === 'frame' ? frameFields : scope === 'logo' ? logoFields : outputFields).find(item => item.key === key);
                let value = el.type === 'checkbox' ? el.checked : el.type === 'number' ? el.valueAsNumber : el.value;
                if (el.type === 'number' && !Number.isFinite(value)) { invalidFields.add(el.dataset.field); changed(); return; }
                invalidFields.delete(el.dataset.field);
                if (def?.kind === 'list') value = el.value.split(',').map(item => item.trim()).filter(Boolean);
                if (scope === 'content') {
                    if (spec.content.type === 'calendar' && key === 'all_day') for (const part of ['start', 'end']) {
                        const old = contentObject()[part]; if (old) contentObject()[part] = value ? old.slice(0, 10) : old.slice(0, 10) + 'T09:00';
                    }
                    set(contentObject(), key, value);
                }
                else if (scope === 'style') { spec.style[key] = value; presetID = ''; }
                else if (scope === 'frame') { frameDraft[key] = value; if (key === 'position' && value === 'none') delete spec.frame; else if (spec.frame || key === 'position') spec.frame = clone(frameDraft); }
                else if (scope === 'logo') { logoDraft[key] = value; if (spec.logo) spec.logo[key] = value; }
                else spec.output[key] = value;
                if (def?.kind === 'color' && /^#[\da-f]{6}([\da-f]{2})?$/i.test(value)) {
                    const picker = settings.querySelector('[data-color-picker="' + el.dataset.field + '"]'); if (picker) picker.value = value.slice(0, 7);
                }
                changed(key === 'all_day');
            } else if (el.dataset.insert) { insertion[el.dataset.insert] = el.valueAsNumber; if (insertion.unit === 'px' && el.dataset.insert === 'width') { insertion.x = undefined; insertion.y = undefined; } updateInsertion(); }
            else if (el.hasAttribute('data-csv')) { batchCSV = el.value; changed(); }
        }, { signal });
        node.addEventListener('change', event => {
            const el = event.target;
            if (el.hasAttribute('data-content-type')) {
                invalidFields.clear(); drafts[spec.content.type] = clone(spec.content); spec.content = drafts[el.value] || defaultContent(el.value);
                if (el.value === 'epc') spec.style.error_correction = 'M'; changed(true);
            } else if (el.hasAttribute('data-full')) { view.full = el.checked; rememberView(); applySimpleView(); }
            else if (el.hasAttribute('data-preset') && el.value) {
                presetID = el.value;
                const presentation = catalog.presets.find(preset => preset.id === el.value)?.presentation;
                if (presentation) { const caption = frameDraft.text, correction = spec.style.error_correction; invalidFields.clear(); spec.style = { ...clone(presentation.style), error_correction: correction }; if (presentation.logo) spec.logo = clone(presentation.logo); else delete spec.logo;
                    if (presentation.frame) spec.frame = { ...clone(presentation.frame), text: caption }; else delete spec.frame; frameDraft = spec.frame ? clone(spec.frame) : { ...defaultFrame(), text: caption }; changed(true); settings.querySelector('[data-preset]')?.focus(); }
            } else if (el.hasAttribute('data-frame-preset') && el.value) {
                framePresetID = el.value;
                const text = frameDraft.text, font = frameDraft.font; frameDraft = { ...defaultFrame(), text, font,
                    ...(el.value === 'ticket' ? { position: 'top', border_width: 2, padding: 22 } : el.value === 'sidebar' ? { position: 'right', side_width: 180, background_color: '#172a46', text_color: '#ffffff', padding: 20 } : { position: 'bottom', padding: 16 }) };
                spec.frame = clone(frameDraft); changed(true); settings.querySelector('[data-frame-preset]')?.focus();
            } else if (el.dataset.resource && el.files[0]) void importResource(el.dataset.resource, el.files[0]);
            else if (el.hasAttribute('data-config-file') && el.files[0]) void importConfig(el.files[0]);
            else if (el.hasAttribute('data-csv-file') && el.files[0]) {
                const file = el.files[0], token = revision, generation = ++csvGeneration; if (file.size > MAX_CSV) { status('csv_invalid', true); return; }
                void file.arrayBuffer().then(bytes => {
                    if (!current() || token !== revision || generation !== csvGeneration) return;
                    let text; try { text = new TextDecoder('utf-8', { fatal: true }).decode(bytes); } catch (_) { status('csv_encoding', true); return; }
                    batchCSV = text; batchResult = null; changed(true);
                }).catch(failure);
            } else if (el.hasAttribute('data-batch-type')) { batchType = el.value; batchResult = null; changed(); }
            else if (el.hasAttribute('data-scan-file') && el.files[0]) { stopCamera(); void scanImage(el.files[0]); }
            else if (el.hasAttribute('data-format')) { spec.output.format = el.value; }
        }, { signal });
        node.addEventListener('dragover', event => { if (tab === 'scan' && event.dataTransfer?.types.includes('Files')) { event.preventDefault(); find('[data-drop]').dataset.drag = 'true'; } }, { signal });
        node.addEventListener('dragleave', () => { const drop = find('[data-drop]'); if (drop) delete drop.dataset.drag; }, { signal });
        node.addEventListener('drop', event => {
            if (tab !== 'scan') return; event.preventDefault(); delete find('[data-drop]').dataset.drag;
            const file = [...event.dataTransfer.files].find(item => SCAN_TYPES.test(item.type)); if (file) { stopCamera(); void scanImage(file); } else status('no_code', true);
        }, { signal });
        node.addEventListener('paste', event => {
            if (tab !== 'scan') return;
            const file = [...(event.clipboardData?.files || [])].find(item => SCAN_TYPES.test(item.type)); if (file) { event.preventDefault(); stopCamera(); void scanImage(file); }
        }, { signal });
        node.addEventListener('keydown', event => {
            if (event.target.closest('.vd-modal-backdrop')) return;
            event.stopPropagation();
            if (event.target.matches('[role=tab]') && ['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) {
                event.preventDefault(); event.stopPropagation(); const keys = ['content', 'design', 'batch', 'scan'], index = keys.indexOf(tab);
                const next = event.key === 'Home' ? 0 : event.key === 'End' ? 3 : (index + (event.key === 'ArrowRight' ? 1 : 3)) % 4;
                switchTab(keys[next]); find('[data-tab="' + keys[next] + '"]').focus();
            }
            if (event.key === 'Escape') { event.preventDefault(); event.stopImmediatePropagation(); close(); }
        }, { signal });
        document.addEventListener('keydown', event => {
            if (!node.contains(event.target)) return;
            if (event.target.closest('.vd-modal-backdrop')) return;
            if (event.key === 'Escape') { event.preventDefault(); event.stopImmediatePropagation(); close(); }
            else if ((event.ctrlKey || event.metaKey) && ['s', 'o', 'n', 'f', 'h', 'p'].includes(event.key.toLowerCase())) {
                event.preventDefault(); event.stopPropagation();
            }
        }, { capture: true, signal });
        node.addEventListener('cancel', event => { event.preventDefault(); close(); }, { signal });
        node.addEventListener('close', close, { signal });
        window.addEventListener('aurago:auth-ended', close, { signal });
        options.signal?.addEventListener('abort', close, { once: true });
        if (options.signal?.aborted) { close(); return handle; }
        node.showModal(); insertionControls(); syncButtons(); status('busy');
        void (async () => {
            try {
                catalog = await (await checked(await fetch('/api/qr/presets', { credentials: 'same-origin', cache: 'no-store', signal }))).json();
                if (!current()) { if (!dead) status('stale', true); return; }
                if (catalog.schema_version !== 1 || !Array.isArray(catalog.content_types)) throw new Error('catalog');
                find('[data-format]').innerHTML = catalog.output_formats.map(format => '<option value="' + esc(format) + '">' + esc(['png', 'jpeg', 'svg'].includes(format) ? format.toUpperCase() : tr('format_' + format)) + '</option>').join('');
                renderControls(); status('no_preview');
                await renderPreview();
                emailBinding = window.AuraDesktopEmail?.attach(options.windowId, '.vd-qr-footer', () => {
                    const batchReady=!!batchResult&&batchRevision===revision&&!batchResult.errors?.length;
                    const previewReady=previewRevision===revision&&!!preview;
                    return {key:revision,signal,dialogHost:node,onHandoff:close,onSetup:close,disabled:!current()||(!previewReady&&!batchReady)||busy||resourceBusy,
                    formats:[...(previewReady?catalog.output_formats:[]),...(batchReady?['zip']:[])].map(id=>({id,label:['png','jpeg','svg','zip'].includes(id)?id.toUpperCase():tr('format_'+id)})),subject:'QR',
                    createFiles:async(format,exportSignal)=>{
                        if(format==='zip')return batch(false,true,exportSignal);
                        const snapshot=clone(spec);snapshot.output.format=format;
                        const result=format==='png'?{blob:preview}:await renderBinary(snapshot,{signal:exportSignal});
                        const extension=format==='markdown'?'md':format==='jpeg'?'jpg':['png','svg'].includes(format)?format:'txt';
                        return [new File([result.blob],'QR.'+extension,{type:result.blob.type})];
                    }
                };}, node);
            } catch (error) { failure(error); }
        })();
        return handle;
    }
    window.AuraQRDialog = { open };
})();
