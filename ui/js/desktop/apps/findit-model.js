(function () {
    'use strict';

    const API = '/api/desktop/findit';
    const PAGE_SIZE = 30;
    const ID_PATTERN = /^[A-Za-z0-9_-]{1,128}$/;
    const esc = value => String(value ?? '').replace(/[&<>"']/g, char => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[char]);
    const clone = value => value == null ? value : structuredClone(value);
    const asArray = value => Array.isArray(value) ? value : [];
    const safeId = value => ID_PATTERN.test(String(value || '')) ? String(value) : '';
    const text = value => String(value ?? '').trim();
    const date = value => {
        if (!value) return '';
        const parsed = new Date(value);
        return Number.isNaN(parsed.getTime()) ? '' : new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' }).format(parsed);
    };
    function quantityUnits(value) {
        const raw = String(value ?? '').trim();
        if (!/^-?\d+(?:[.,]\d{1,6})?$/.test(raw) || (raw.includes('.') && raw.includes(','))) return null;
        const negative = raw[0] === '-';
        const [whole, fraction = ''] = (negative ? raw.slice(1) : raw).replace(',', '.').split('.');
        const units = BigInt(whole) * 1000000n + BigInt((fraction + '000000').slice(0, 6));
        return negative ? -units : units;
    }
    function quantityString(units) {
        const negative = units < 0n;
        const absolute = negative ? -units : units;
        const whole = absolute / 1000000n;
        const fraction = (absolute % 1000000n).toString().padStart(6, '0').replace(/0+$/, '');
        return (negative ? '-' : '') + whole.toString() + (fraction ? '.' + fraction : '');
    }
    function quantityAdd(left, right) {
        const a = quantityUnits(left), b = quantityUnits(right);
        return a == null || b == null ? null : quantityString(a + b);
    }
    function quantityCompare(left, right) {
        const a = quantityUnits(left), b = quantityUnits(right);
        return a == null || b == null ? null : a < b ? -1 : a > b ? 1 : 0;
    }
    function fmtQuantity(value) {
        const units = quantityUnits(value);
        if (units == null) return text(value) || '0';
        const negative = units < 0n, absolute = negative ? -units : units;
        const whole = absolute / 1000000n;
        const fraction = (absolute % 1000000n).toString().padStart(6, '0').replace(/0+$/, '');
        const grouped = new Intl.NumberFormat(undefined, { maximumFractionDigits: 0 }).format(whole);
        const decimal = new Intl.NumberFormat().formatToParts(1.1).find(part => part.type === 'decimal')?.value || '.';
        return (negative ? '−' : '') + grouped + (fraction ? decimal + fraction : '');
    }
    function normalizeQuantity(value) { const units = quantityUnits(value); return units == null ? null : quantityString(units); }
    function wholeQuantity(value) { const units = quantityUnits(value); return units != null && units % 1000000n === 0n; }
    async function request(path, options = {}) {
        const response = await fetch(API + path, {
            method: options.method || 'GET', credentials: 'same-origin', cache: 'no-store', signal: options.signal,
            headers: options.body instanceof FormData ? options.headers : { ...(options.body ? { 'Content-Type': 'application/json' } : {}), ...(options.headers || {}) },
            body: options.body instanceof FormData ? options.body : options.body == null ? undefined : JSON.stringify(options.body)
        });
        if (!response.ok) {
            const body = await response.json().catch(() => ({}));
            const error = new Error(String(body.message || body.error || `HTTP ${response.status}`));
            error.status = response.status; error.code = String(body.code || `http_${response.status}`); error.reason = String(body.reason || ''); error.field = String(body.field || ''); error.details = body.details;
            throw error;
        }
        return options.raw ? response : response.status === 204 ? null : response.json();
    }
    const getStatus = signal => request('/status', { signal });
    const getCatalog = signal => request('/catalog', { signal });
    const query = (payload, signal) => request('/query', { method: 'POST', body: payload, signal });
    function command(payload, signal) {
        const body = { ...payload };
        if (!body.request_id) body.request_id = crypto.randomUUID();
        return request('/command', { method: 'POST', body, signal });
    }
    function updateSettings(settings, revision, signal) {
        return request('/settings', { method: 'POST', body: { ...settings, expected_version: revision }, signal });
    }
    function csvPreview(payload, signal) {
        return request('/import/preview', { method: 'POST', body: payload, signal });
    }
    function csvImport(payload, signal) {
        return request('/import', { method: 'POST', body: { ...payload, request_id: payload.request_id || crypto.randomUUID() }, signal });
    }
    function csvExport(entity, projectId, signal) {
        const query = new URLSearchParams({ entity });
        if (projectId) query.set('project_id', projectId);
        return request('/export?' + query.toString(), { signal, raw: true });
    }
    async function uploadImage(file, signal) {
        const body = new FormData(); body.set('file', file, file.name || 'findit-photo');
        return request('/images', { method: 'POST', body, signal });
    }
    function imageURL(id, thumbnail = false) {
        const safe = safeId(id);
        return safe ? `${API}/images/${encodeURIComponent(safe)}${thumbnail ? '/thumbnail' : ''}` : '';
    }
    async function loadManifest(signal) {
        const response = await fetch('/img/findit/manifest.json', { credentials: 'same-origin', cache: 'force-cache', signal });
        if (!response.ok) throw new Error(`FindIt artwork HTTP ${response.status}`);
        const manifest = await response.json();
        if (manifest.version !== 1 || !Array.isArray(manifest.icons) || !manifest.atlas) throw new Error('Invalid FindIt artwork manifest');
        const icons = new Map();
        for (const icon of manifest.icons) {
            if (!icon || typeof icon.id !== 'string' || !Number.isInteger(icon.index) || icon.index < 0 || icon.index >= manifest.columns * manifest.rows) continue;
            icons.set(icon.id, icon.index);
        }
        return { ...manifest, icons, atlasURL: new URL(manifest.atlas, window.location.origin).pathname };
    }
    function packageLabel(item, catalog, t) {
        const family = text(item?.package_family);
        const detail = text(item?.package_size);
        if (!family) return detail;
        const itemFromCatalog = asArray(catalog?.package_families).find(value => (typeof value === 'string' ? value : value?.id) === family);
        const defaultName = typeof itemFromCatalog === 'object' ? (itemFromCatalog.label || itemFromCatalog.name || family) : family.toUpperCase();
        const name = t('package_' + family.toLowerCase().replace(/[^a-z0-9]+/g, '_'), defaultName);
        const pins = Number(item?.pin_count);
        const packageName = `${name}${Number.isFinite(pins) && pins > 0 ? '-' + pins : ''}`;
        return detail && detail.toLowerCase() !== family.toLowerCase() ? `${packageName} · ${detail}` : packageName || t('not_set');
    }
    const CATEGORY_ICONS = Object.freeze({
        electronic_component: 'chip', pc_component: 'cpu', networking: 'network', server: 'storage',
        development: 'devboard', instruments: 'meter', consumable: 'generic', other: 'generic'
    });
    function categoryIconId(category, manifest) {
        const id = safeId(category), mapped = CATEGORY_ICONS[id] || id;
        if (mapped && manifest?.icons?.has(mapped)) return mapped;
        return manifest?.icons?.has('generic') ? 'generic' : '';
    }
    function iconId(item, manifest) {
        const explicit = safeId(item?.icon_id);
        if (explicit && manifest?.icons?.has(explicit)) return explicit;
        const packageFamilies = {
            dip: 'dip', sip: 'sip', soic: 'soic', sop: 'soic', ssop: 'soic', tssop: 'tssop', msop: 'chip', qfp: 'qfp', lqfp: 'qfp', tqfp: 'qfp',
            qfn: 'qfn', dfn: 'qfn', bga: 'bga', lga: 'lga', plcc: 'plcc', sot23: 'sot', sc70: 'sot', sot323: 'sot', sot223: 'sot', sod123: 'sot', sod323: 'sot', sod523: 'sot',
            to92: 'to', to220: 'to', to252_dpak: 'dpak', to263_d2pak: 'dpak', axial: 'axial', radial: 'radial', chip_resistor: 'chip', chip_capacitor: 'chip',
            '01005': 'chip', '0201': 'chip', '0402': 'chip', '0603': 'chip', '0805': 'chip', '1206': 'chip', '1210': 'chip', '1812': 'chip', '2512': 'chip'
        };
        const physicalPackage = packageFamilies[safeId(String(item?.package_family || '').toLowerCase())];
        if (physicalPackage && manifest?.icons?.has(physicalPackage)) return physicalPackage;
        const templateAliases = {
            op_amp: 'opamp', integrated_circuit: 'chip', power_supply: 'psu', pc_case: 'case', cpu_cooler: 'cooler',
            network_card: 'network', network_switch: 'network', access_point: 'radio', sfp_module: 'interface', ups: 'battery',
            printer: 'tool', expansion_card: 'expansion', pc_component: 'cpu', development_board: 'devboard', single_board_computer: 'sbc',
            voltage_regulator: 'regulator', optoisolator: 'optocoupler', memory_ic: 'memory', radio_module: 'radio'
        };
        const template = safeId(item?.template_id), templateIcon = templateAliases[template] || template;
        const categoryIcon = categoryIconId(item?.category, manifest);
        if (templateIcon && manifest?.icons?.has(templateIcon) && !(templateIcon === 'generic' && categoryIcon && categoryIcon !== 'generic')) return templateIcon;
        return categoryIcon || 'generic';
    }
    function itemImage(item, manifest, t, compact = false) {
        const image = item?.image_id && imageURL(item.image_id, compact);
        if (image) return `<img class="fi-item-photo" loading="lazy" src="${esc(image)}" alt="">`;
        const icon = iconId(item, manifest), index = manifest?.icons?.get(icon) ?? 0;
        const columns = Number(manifest?.columns) || 8, rows = Number(manifest?.rows) || 8;
        const x = index % columns, y = Math.floor(index / columns);
        return `<span class="fi-item-sprite" role="img" aria-label="${esc(t('category_' + icon, item?.category || icon))}" style="--fi-atlas:url('${esc(manifest?.atlasURL || '/img/findit/atlas.webp')}');--fi-x:${columns <= 1 ? 0 : x * 100 / (columns - 1)}%;--fi-y:${rows <= 1 ? 0 : y * 100 / (rows - 1)}%;--fi-grid:${columns}"></span>`;
    }
    function categoryIcon(category, manifest) {
        const id = categoryIconId(category, manifest);
        const index = manifest?.icons?.get(id) ?? manifest?.icons?.get('generic') ?? 0;
        const columns = Number(manifest?.columns) || 8, rows = Number(manifest?.rows) || 8;
        return `<span class="fi-category-sprite" aria-hidden="true" style="--fi-atlas:url('${esc(manifest?.atlasURL || '/img/findit/atlas.webp')}');--fi-x:${columns <= 1 ? 0 : (index % columns) * 100 / (columns - 1)}%;--fi-y:${rows <= 1 ? 0 : Math.floor(index / columns) * 100 / (rows - 1)}%;--fi-grid:${columns}"></span>`;
    }
    window.FindItModel = {
        API, PAGE_SIZE, esc, clone, asArray, safeId, text, date,
        quantityUnits, quantityString, quantityAdd, quantityCompare, fmtQuantity, normalizeQuantity, wholeQuantity,
        request, getStatus, getCatalog, query, command, updateSettings, csvPreview, csvImport, csvExport, uploadImage, imageURL, loadManifest,
        packageLabel, iconId, itemImage, categoryIcon
    };
})();
