// AuraGo – config page shared utilities
/* global t */
'use strict';

const EYE_OPEN_SVG = '<svg viewBox="0 0 24 24"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>';
const EYE_CLOSED_SVG = '<svg viewBox="0 0 24 24"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg>';

const CFG_TEXT_AUTOFILL_ATTRS = ' autocomplete="off" autocapitalize="off" autocorrect="off" spellcheck="false" data-lpignore="true" data-1p-ignore="true" data-bwignore="true" data-form-type="other"';
const CFG_SENSITIVE_AUTOFILL_ATTRS = ' autocomplete="new-password" autocapitalize="off" autocorrect="off" spellcheck="false" data-lpignore="true" data-1p-ignore="true" data-bwignore="true" data-form-type="other"';

const CFG_OPTION_OTHER_CUSTOM = 'Other / Custom';

function escapeAttr(s) { return String(s).replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/'/g, '&#39;').replace(/</g, '&lt;').replace(/>/g, '&gt;'); }
function escapeHtml(s) { return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&#39;'); }

function formatKey(key) {
    return key.split('_').map(w => w.charAt(0).toUpperCase() + w.slice(1)).join(' ');
}

function cfgNoAutofillAttrs(sensitive = false) {
    return sensitive ? CFG_SENSITIVE_AUTOFILL_ATTRS : CFG_TEXT_AUTOFILL_ATTRS;
}

function cfgFieldOptionLabel(option) {
    if (option === 'disabled') return '\u{1F6AB} ' + (t('config.field.disabled_option'));
    if (option === CFG_OPTION_OTHER_CUSTOM) return t('config.field.other_custom_option') || CFG_OPTION_OTHER_CUSTOM;
    return option;
}

function cfgToggleCustomInput(selectEl) {
    const customInput = selectEl.nextElementSibling;
    if (!customInput || !customInput.classList.contains('cfg-custom-input')) return;
    const showCustom = selectEl.value === CFG_OPTION_OTHER_CUSTOM;
    customInput.classList.toggle('is-hidden', !showCustom);
    if (showCustom) customInput.focus();
}

/** Set a deep value on an object using a dot-separated path, e.g. "tools.web_scraper.enabled". */
function setNestedValue(obj, path, value) {
    const parts = path.split('.');
    let cur = obj;
    for (let i = 0; i < parts.length - 1; i++) {
        if (cur[parts[i]] === undefined || cur[parts[i]] === null || typeof cur[parts[i]] !== 'object') {
            cur[parts[i]] = {};
        }
        cur = cur[parts[i]];
    }
    cur[parts[parts.length - 1]] = value;
}

function getNestedValue(obj, path) {
    return String(path || '').split('.').reduce((acc, part) => {
        if (acc === undefined || acc === null) return undefined;
        return acc[part];
    }, obj);
}

function guessType(val) {
    if (typeof val === 'boolean') return 'bool';
    if (typeof val === 'number') return Number.isInteger(val) ? 'int' : 'float';
    if (Array.isArray(val)) return 'array';
    return 'string';
}

function isNumericPathPart(part) {
    return /^\d+$/.test(String(part));
}
