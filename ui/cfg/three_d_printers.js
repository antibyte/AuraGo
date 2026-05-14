// cfg/three_d_printers.js — 3D printer integration section module

function renderThreeDPrintersSection(section) {
    if (!section && Array.isArray(SECTIONS)) {
        for (const group of SECTIONS) {
            const found = group.items.find(item => item.key === 'three_d_printers');
            if (found) { section = found; break; }
        }
    }
    const cfg = configData.three_d_printers || {};
    const elegoo = cfg.elegoo_centauri_carbon || {};
    const klipper = cfg.klipper || {};
    const printers = Array.isArray(elegoo.printers) ? elegoo.printers : [];
    const klipperPrinters = Array.isArray(klipper.printers) ? klipper.printers : [];

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    html += threeDPrinterToggle('three_d_printers.enabled', cfg.enabled === true, t('config.three_d_printers.enabled_label'), t('help.three_d_printers.enabled'), 'renderThreeDPrintersSection');
    html += threeDPrinterToggle('three_d_printers.readonly', cfg.readonly !== false, t('config.three_d_printers.readonly_label'), t('help.three_d_printers.readonly'), 'renderThreeDPrintersSection');
    html += threeDPrinterField('three_d_printers.default_printer', cfg.default_printer || '', t('config.three_d_printers.default_printer_label'), t('help.three_d_printers.default_printer'), 'lab-printer');
    html += threeDPrinterToggle('three_d_printers.elegoo_centauri_carbon.enabled', elegoo.enabled === true, t('config.three_d_printers.elegoo_enabled_label'), t('help.three_d_printers.elegoo_enabled'), 'renderThreeDPrintersSection');

    html += '<div class="cfg-group-title cfg-group-title-top">' + t('config.three_d_printers.elegoo_printers_title') + '</div>';
    html += '<div class="field-help">' + t('help.three_d_printers.elegoo_printers') + '</div>';
    html += '<div class="three-d-printer-list">';
    printers.forEach((printer, idx) => {
        const base = 'three_d_printers.elegoo_centauri_carbon.printers.' + idx;
        html += '<div class="cfg-card three-d-printer-card">';
        html += '<div class="cfg-card-title">' + escapeHtml(printer.name || printer.id || (t('config.three_d_printers.printer_title') + ' ' + (idx + 1))) + '</div>';
        html += threeDPrinterField(base + '.id', printer.id || '', t('config.three_d_printers.printer_id_label'), t('help.three_d_printers.printer_id'), 'lab-printer');
        html += threeDPrinterField(base + '.name', printer.name || '', t('config.three_d_printers.printer_name_label'), t('help.three_d_printers.printer_name'), 'Elegoo Centauri Carbon');
        html += threeDPrinterField(base + '.url', printer.url || '', t('config.three_d_printers.printer_url_label'), t('help.three_d_printers.printer_url'), 'ws://192.168.6.50/websocket');
        html += threeDPrinterField(base + '.mainboard_id', printer.mainboard_id || '', t('config.three_d_printers.mainboard_id_label'), t('help.three_d_printers.mainboard_id'), '');
        html += threeDPrinterNumber(base + '.timeout_seconds', printer.timeout_seconds || 10, t('config.three_d_printers.timeout_label'), t('help.three_d_printers.timeout'), 1, 120);
        html += '<div class="field-group">';
        html += '<button type="button" class="btn-save" onclick="threeDPrinterTest(\'elegoo\', ' + idx + ')" id="three-d-printer-test-elegoo-' + idx + '">' + t('config.three_d_printers.test_button') + '</button> ';
        html += '<button type="button" class="btn-secondary" onclick="threeDPrinterRemove(\'elegoo\', ' + idx + ')">' + t('config.three_d_printers.remove_button') + '</button>';
        html += '<span id="three-d-printer-test-result-elegoo-' + idx + '" class="adg-test-result"></span>';
        html += '</div>';
        html += '</div>';
    });
    html += '</div>';
    html += '<div class="field-group"><button type="button" class="btn-save" onclick="threeDPrinterAdd(\'elegoo\')">' + t('config.three_d_printers.add_button') + '</button></div>';

    html += threeDPrinterToggle('three_d_printers.klipper.enabled', klipper.enabled === true, t('config.three_d_printers.klipper_enabled_label'), t('help.three_d_printers.klipper_enabled'), 'renderThreeDPrintersSection');
    html += '<div class="cfg-group-title cfg-group-title-top">' + t('config.three_d_printers.klipper_printers_title') + '</div>';
    html += '<div class="field-help">' + t('help.three_d_printers.klipper_printers') + '</div>';
    html += '<div class="three-d-printer-list">';
    klipperPrinters.forEach((printer, idx) => {
        const base = 'three_d_printers.klipper.printers.' + idx;
        html += '<div class="cfg-card three-d-printer-card">';
        html += '<div class="cfg-card-title">' + escapeHtml(printer.name || printer.id || (t('config.three_d_printers.klipper_printer_title') + ' ' + (idx + 1))) + '</div>';
        html += threeDPrinterField(base + '.id', printer.id || '', t('config.three_d_printers.printer_id_label'), t('help.three_d_printers.printer_id'), 'voron');
        html += threeDPrinterField(base + '.name', printer.name || '', t('config.three_d_printers.printer_name_label'), t('help.three_d_printers.printer_name'), 'Voron 2.4');
        html += threeDPrinterField(base + '.url', printer.url || '', t('config.three_d_printers.klipper_url_label'), t('help.three_d_printers.klipper_url'), 'http://192.168.6.60:7125');
        html += threeDPrinterSecret(base + '.api_key', printer.api_key || '', t('config.three_d_printers.klipper_api_key_label'), t('help.three_d_printers.klipper_api_key'), '');
        html += threeDPrinterNumber(base + '.timeout_seconds', printer.timeout_seconds || 10, t('config.three_d_printers.timeout_label'), t('help.three_d_printers.timeout'), 1, 120);
        html += threeDPrinterField(base + '.webcam_name', printer.webcam_name || '', t('config.three_d_printers.klipper_webcam_name_label'), t('help.three_d_printers.klipper_webcam_name'), 'toolhead');
        html += '<div class="field-group">';
        html += '<button type="button" class="btn-save" onclick="threeDPrinterTest(\'klipper\', ' + idx + ')" id="three-d-printer-test-klipper-' + idx + '">' + t('config.three_d_printers.test_button') + '</button> ';
        html += '<button type="button" class="btn-secondary" onclick="threeDPrinterRemove(\'klipper\', ' + idx + ')">' + t('config.three_d_printers.remove_button') + '</button>';
        html += '<span id="three-d-printer-test-result-klipper-' + idx + '" class="adg-test-result"></span>';
        html += '</div>';
        html += '</div>';
    });
    html += '</div>';
    html += '<div class="field-group"><button type="button" class="btn-save" onclick="threeDPrinterAdd(\'klipper\')">' + t('config.three_d_printers.klipper_add_button') + '</button></div>';
    html += '</div>';

    document.getElementById('content').innerHTML = html;
}

function threeDPrinterField(path, value, label, help, placeholder) {
    let html = '<div class="field-group">';
    html += '<div class="field-label">' + label + '</div>';
    if (help) html += '<div class="field-help">' + help + '</div>';
    html += '<input class="field-input" type="text" data-path="' + escapeAttr(path) + '" value="' + escapeAttr(value || '') + '" placeholder="' + escapeAttr(placeholder || '') + '">';
    html += '</div>';
    return html;
}

function threeDPrinterSecret(path, value, label, help, placeholder) {
    let html = '<div class="field-group">';
    html += '<div class="field-label">' + label + '</div>';
    if (help) html += '<div class="field-help">' + help + '</div>';
    html += '<input class="field-input" type="password" autocomplete="off" data-path="' + escapeAttr(path) + '" value="' + escapeAttr(value || '') + '" placeholder="' + escapeAttr(placeholder || '') + '">';
    html += '</div>';
    return html;
}

function threeDPrinterNumber(path, value, label, help, min, max) {
    let html = '<div class="field-group">';
    html += '<div class="field-label">' + label + '</div>';
    if (help) html += '<div class="field-help">' + help + '</div>';
    html += '<input class="field-input" type="number" min="' + min + '" max="' + max + '" data-path="' + escapeAttr(path) + '" value="' + escapeAttr(value || '') + '">';
    html += '</div>';
    return html;
}

function threeDPrinterToggle(path, on, label, help, renderFn) {
    let html = '<div class="field-group">';
    html += '<div class="field-label">' + label + '</div>';
    if (help) html += '<div class="field-help">' + help + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (on ? ' on' : '') + '" data-path="' + escapeAttr(path) + '" onclick="toggleBool(this);setNestedValue(configData,\'' + path + '\',this.classList.contains(\'on\'));' + renderFn + '(null)"></div>';
    html += '<span class="toggle-label">' + (on ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';
    return html;
}

function threeDPrinterEnsurePrinters(protocol) {
    if (!configData.three_d_printers) configData.three_d_printers = {};
    if (protocol === 'klipper') {
        if (!configData.three_d_printers.klipper) configData.three_d_printers.klipper = {};
        if (!Array.isArray(configData.three_d_printers.klipper.printers)) {
            configData.three_d_printers.klipper.printers = [];
        }
        return configData.three_d_printers.klipper.printers;
    }
    if (!configData.three_d_printers.elegoo_centauri_carbon) configData.three_d_printers.elegoo_centauri_carbon = {};
    if (!Array.isArray(configData.three_d_printers.elegoo_centauri_carbon.printers)) {
        configData.three_d_printers.elegoo_centauri_carbon.printers = [];
    }
    return configData.three_d_printers.elegoo_centauri_carbon.printers;
}

function threeDPrinterAdd(protocol) {
    const printers = threeDPrinterEnsurePrinters(protocol);
    if (protocol === 'klipper') {
        printers.push({ id: 'klipper-' + (printers.length + 1), name: 'Klipper', url: 'http://192.168.6.60:7125', api_key: '', timeout_seconds: 10, webcam_name: '' });
    } else {
        printers.push({ id: 'printer-' + (printers.length + 1), name: 'Elegoo Centauri Carbon', url: 'ws://192.168.6.50/websocket', timeout_seconds: 10 });
    }
    setDirty(true);
    renderThreeDPrintersSection(null);
}

function threeDPrinterRemove(protocol, index) {
    const printers = threeDPrinterEnsurePrinters(protocol);
    printers.splice(index, 1);
    setDirty(true);
    renderThreeDPrintersSection(null);
}

async function threeDPrinterTest(protocol, index) {
    const printers = threeDPrinterEnsurePrinters(protocol);
    const printer = printers[index] || {};
    const result = document.getElementById('three-d-printer-test-result-' + protocol + '-' + index);
    const btn = document.getElementById('three-d-printer-test-' + protocol + '-' + index);
    if (!result || !btn) return;
    if (!threeDPrinterURLLooksValid(protocol, printer.url || '')) {
        result.textContent = t('config.three_d_printers.test_failed');
        return;
    }
    btn.disabled = true;
    result.textContent = t('config.three_d_printers.testing');
    try {
        const res = await fetch('/api/3d-printers/test', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                operation: 'test_connection',
                protocol: protocol === 'klipper' ? 'klipper' : 'elegoo_centauri_carbon',
                printer_id: printer.id || '',
                url: printer.url || '',
                api_key: printer.api_key || '',
                mainboard_id: printer.mainboard_id || '',
                timeout_seconds: printer.timeout_seconds || 10,
                webcam_name: printer.webcam_name || ''
            })
        });
        const data = await res.json();
        result.textContent = data.status === 'error'
            ? t('config.three_d_printers.test_failed') + ': ' + (data.message || '')
            : t('config.three_d_printers.test_ok');
    } catch (err) {
        result.textContent = t('config.three_d_printers.test_failed');
    } finally {
        btn.disabled = false;
    }
}

function threeDPrinterURLLooksValid(protocol, value) {
    try {
        const parsed = new URL(value || '');
        const allowed = protocol === 'klipper'
            ? ['http:', 'https:']
            : ['ws:', 'wss:'];
        return allowed.includes(parsed.protocol);
    } catch (err) {
        return false;
    }
}
