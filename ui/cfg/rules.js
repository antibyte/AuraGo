// cfg/rules.js — task rule editor

let rulesState = {
    rules: [],
    selected: null,
    current: null
};

function rulesText(key, fallback, vars) {
    const value = t(key, vars || {});
    return value === key ? fallback : value;
}

function rulesEscape(value) {
    if (typeof esc === 'function') return esc(value == null ? '' : String(value));
    return String(value == null ? '' : value)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#039;');
}

function rulesToCSV(values) {
    return Array.isArray(values) ? values.join(', ') : '';
}

function rulesFromCSV(value) {
    return String(value || '')
        .split(',')
        .map(v => v.trim())
        .filter(Boolean);
}

async function renderRulesSection(section) {
    const content = document.getElementById('content');
    content.innerHTML = `
    <div class="cfg-section active">
        <div class="section-header">${section.icon} ${section.label}</div>
        <div class="section-desc">${section.desc}</div>
        <div id="rules-body" class="rules-body">
            <div class="cfg-loading-state">
                <div class="cfg-loading-icon">📏</div>
                <div class="cfg-loading-text">${rulesText('config.rules.loading', 'Loading rules...')}</div>
            </div>
        </div>
    </div>`;
    await rulesLoad();
}

async function rulesLoad(selectID) {
    const body = document.getElementById('rules-body');
    try {
        const resp = await fetch('/api/config/rules', { credentials: 'same-origin' });
        if (!resp.ok) throw new Error('HTTP ' + resp.status);
        const data = await resp.json();
        rulesState.rules = data.rules || [];
        rulesState.selected = selectID || rulesState.selected || (rulesState.rules[0] && rulesState.rules[0].id) || null;
        rulesRenderBody(data.enabled !== false);
        if (rulesState.selected) {
            await rulesSelect(rulesState.selected);
        }
    } catch (e) {
        if (body) body.innerHTML = `<div class="wh-notice wh-notice-danger">${rulesText('config.rules.load_failed', 'Failed to load rules')}: ${rulesEscape(e.message)}</div>`;
    }
}

function rulesRenderBody(enabled) {
    const body = document.getElementById('rules-body');
    if (!body) return;
    const totalRules = rulesState.rules.length;
    const activeRules = rulesState.rules.filter(rule => rule.enabled !== false).length;
    const customRules = rulesState.rules.filter(rule => !rule.built_in).length;
    const systemStateLabel = enabled ? rulesText('config.rules.enabled', 'Enabled') : rulesText('config.rules.disabled', 'Disabled');
    const systemStateClass = enabled ? 'rules-pill-ok' : 'rules-pill-warning';
    const ruleCards = rulesState.rules.map(rule => {
        const selected = rule.id === rulesState.selected ? ' rules-card-selected' : '';
        const disabled = rule.enabled !== false ? '' : ' rules-card-disabled';
        const origin = rule.built_in ? rulesText('config.rules.built_in', 'Built-in') : rulesText('config.rules.custom', 'Custom');
        const enabledLabel = rule.enabled !== false ? rulesText('config.rules.active', 'Active') : rulesText('config.rules.inactive', 'Inactive');
        const enabledClass = rule.enabled !== false ? 'rules-card-status-on' : 'rules-card-status-off';
        const tags = []
            .concat((rule.tools || []).map(v => `tool:${v}`))
            .concat((rule.workflows || []).map(v => `flow:${v}`))
            .concat((rule.keywords || []).slice(0, 4).map(v => `key:${v}`));
        return `
            <button type="button" class="rules-card${selected}${disabled}" data-rule-id="${rulesEscape(rule.id)}" onclick="rulesSelect('${escapeAttr(rule.id)}')">
                <span class="rules-card-status ${enabledClass}">${rulesEscape(enabledLabel)}</span>
                <span class="rules-card-main">
                    <span class="rules-card-title">${rulesEscape(rule.title || rule.id)}</span>
                    <span class="rules-card-id">${rulesEscape(rule.id)} · ${origin}</span>
                </span>
                <span class="rules-card-tags">${tags.map(v => `<span>${rulesEscape(v)}</span>`).join('')}</span>
            </button>`;
    }).join('');

    body.innerHTML = `
    <div class="rules-shell">
        <div class="rules-summary-card">
            <div class="rules-summary-copy">
                <div class="rules-summary-title">
                    <span class="rules-status-dot ${enabled ? 'rules-status-dot-on' : 'rules-status-dot-off'}"></span>
                    ${rulesText('config.rules.system_status', 'Rules system')}
                    <span class="rules-pill ${systemStateClass}">${rulesEscape(systemStateLabel)}</span>
                </div>
                <div class="rules-summary-desc">${enabled ? rulesText('config.rules.enabled_hint', 'Automatic task-rule injection is enabled.') : rulesText('config.rules.disabled_hint', 'Automatic task-rule injection is disabled in config.yaml.')}</div>
            </div>
            <div class="rules-summary-actions">
                <div class="rules-summary-metrics">
                    <span class="rules-pill">${rulesText('config.rules.total', 'Total')}: ${totalRules}</span>
                    <span class="rules-pill">${rulesText('config.rules.active', 'Active')}: ${activeRules}</span>
                    <span class="rules-pill">${rulesText('config.rules.custom', 'Custom')}: ${customRules}</span>
                </div>
                <button type="button" class="wh-btn wh-btn-primary wh-btn-sm" onclick="rulesNew()">+ ${rulesText('config.rules.new_rule', 'New rule')}</button>
            </div>
        </div>
        <div class="rules-layout">
            <div class="rules-list-panel">
                <div class="rules-list-header">
                    <span>${rulesText('config.rules.rule_list', 'Rule library')}</span>
                    <span class="rules-pill">${totalRules}</span>
                </div>
                <div class="rules-list">${ruleCards || `<div class="rules-empty">${rulesText('config.rules.empty', 'No rules yet.')}</div>`}</div>
            </div>
            <div class="rules-editor" id="rules-editor">
                <div class="field-help">${rulesText('config.rules.select_hint', 'Select a rule or create a new one.')}</div>
            </div>
        </div>
    </div>`;
}

async function rulesSelect(id) {
    rulesState.selected = id;
    document.querySelectorAll('.rules-card').forEach(card => card.classList.remove('rules-card-selected'));
    const editor = document.getElementById('rules-editor');
    if (!editor) return;
    editor.innerHTML = `<div class="field-help">${rulesText('config.rules.loading', 'Loading rules...')}</div>`;
    try {
        const resp = await fetch('/api/config/rules/' + encodeURIComponent(id), { credentials: 'same-origin' });
        if (!resp.ok) throw new Error('HTTP ' + resp.status);
        const data = await resp.json();
        rulesState.current = {
            rule: data.rule || {},
            design: data.design || ''
        };
        rulesRenderEditor(false);
        rulesRefreshSelectedCard();
    } catch (e) {
        editor.innerHTML = `<div class="wh-notice wh-notice-danger">${rulesText('config.rules.load_failed', 'Failed to load rules')}: ${rulesEscape(e.message)}</div>`;
    }
}

function rulesNew() {
    rulesState.selected = null;
    rulesState.current = {
        rule: {
            id: '',
            title: '',
            enabled: true,
            priority: 50,
            tools: [],
            workflows: [],
            keywords: [],
            body: ''
        },
        design: ''
    };
    rulesRenderEditor(true);
    rulesRefreshSelectedCard();
}

function rulesRenderEditor(isNew) {
    const editor = document.getElementById('rules-editor');
    if (!editor || !rulesState.current) return;
    const rule = rulesState.current.rule || {};
    const designOpen = rule.id === 'homepage' || isNew || rulesState.current.design;
    const origin = rule.built_in ? rulesText('config.rules.built_in', 'Built-in') : rulesText('config.rules.custom', 'Custom');
    const state = rule.enabled !== false ? rulesText('config.rules.active', 'Active') : rulesText('config.rules.inactive', 'Inactive');
    editor.innerHTML = `
    <div class="rules-editor-head">
        <div>
            <div class="rules-editor-kicker">${isNew ? rulesText('config.rules.custom', 'Custom') : rulesEscape(origin)} · ${rulesEscape(state)}</div>
            <div class="rules-editor-title">${isNew ? rulesText('config.rules.new_rule', 'New rule') : rulesEscape(rule.title || rule.id)}</div>
            <div class="field-help">${rulesText('config.rules.editor_hint', 'Rules are Markdown guardrails injected before matching tasks begin.')}</div>
        </div>
        <div class="rules-actions">
            <button type="button" class="wh-btn wh-btn-primary wh-btn-sm" onclick="rulesSave()">${rulesText('config.rules.save', 'Save')}</button>
            ${!isNew ? `<button type="button" class="wh-btn wh-btn-sm" onclick="rulesRestore()">${rulesText('config.rules.restore', 'Restore')}</button>` : ''}
            ${!isNew ? `<button type="button" class="wh-btn wh-btn-sm wh-btn-danger" onclick="rulesDelete()">${rulesText('config.rules.delete', 'Delete')}</button>` : ''}
        </div>
    </div>
    <div class="rules-field-panel">
        <div class="rules-grid rules-grid-meta">
            <label class="field-group">
                <span class="field-label">${rulesText('config.rules.rule_id', 'Rule ID')}</span>
                <input id="rules-id-input" class="field-input" value="${rulesEscape(rule.id || '')}" ${isNew ? '' : 'disabled'} placeholder="homepage">
            </label>
            <label class="field-group">
                <span class="field-label">${rulesText('config.rules.rule_title', 'Title')}</span>
                <input id="rules-title-input" class="field-input" value="${rulesEscape(rule.title || '')}">
            </label>
            <label class="field-group rules-priority-field">
                <span class="field-label">${rulesText('config.rules.priority', 'Priority')}</span>
                <input id="rules-priority-input" class="field-input" type="number" step="1" value="${Number.isFinite(rule.priority) ? rule.priority : 50}">
            </label>
            <div class="field-group rules-toggle-field">
                <span class="field-label">${rulesText('config.rules.enabled', 'Enabled')}</span>
                <label class="rules-checkbox"><input id="rules-enabled-input" type="checkbox" ${rule.enabled !== false ? 'checked' : ''}> ${rulesText('config.rules.enabled', 'Enabled')}</label>
            </div>
        </div>
    </div>
    <div class="rules-field-panel">
        <div class="rules-panel-title">${rulesText('config.rules.targeting', 'Matching targets')}</div>
        <div class="rules-grid rules-grid-targeting">
            <label class="field-group">
                <span class="field-label">${rulesText('config.rules.tools', 'Tools')}</span>
                <input id="rules-tools-input" class="field-input" value="${rulesEscape(rulesToCSV(rule.tools))}" placeholder="homepage, shell">
            </label>
            <label class="field-group">
                <span class="field-label">${rulesText('config.rules.workflows', 'Workflows')}</span>
                <input id="rules-workflows-input" class="field-input" value="${rulesEscape(rulesToCSV(rule.workflows))}" placeholder="homepage, website">
            </label>
            <label class="field-group">
                <span class="field-label">${rulesText('config.rules.keywords', 'Keywords')}</span>
                <input id="rules-keywords-input" class="field-input" value="${rulesEscape(rulesToCSV(rule.keywords))}" placeholder="homepage, redesign">
            </label>
        </div>
    </div>
    <label class="rules-field-panel rules-markdown-panel">
        <span class="rules-panel-title">${rulesText('config.rules.markdown_editor', 'Markdown editor')}</span>
        <span class="field-label">${rulesText('config.rules.body_title', 'Rule Markdown')}</span>
        <textarea id="rules-rule-input" class="field-input rules-textarea" rows="14">${rulesEscape(rule.body || '')}</textarea>
    </label>
    <details class="rules-design-panel" ${designOpen ? 'open' : ''}>
        <summary>${rulesText('config.rules.design_title', 'Homepage DESIGN.md')}</summary>
        <div class="field-help">${rulesText('config.rules.design_hint', 'For homepage rules, this design system is injected after task rules. Project DESIGN.md files may add local design context.')}</div>
        <textarea id="rules-design-input" class="field-input rules-textarea rules-design-textarea" rows="12">${rulesEscape(rulesState.current.design || '')}</textarea>
    </details>
    <div id="rules-status" class="field-help rules-status"></div>`;
}

function rulesCollectPayload() {
    const id = (document.getElementById('rules-id-input')?.value || '').trim();
    return {
        id,
        title: document.getElementById('rules-title-input')?.value || id,
        enabled: !!document.getElementById('rules-enabled-input')?.checked,
        priority: parseInt(document.getElementById('rules-priority-input')?.value || '0', 10) || 0,
        tools: rulesFromCSV(document.getElementById('rules-tools-input')?.value || ''),
        workflows: rulesFromCSV(document.getElementById('rules-workflows-input')?.value || ''),
        keywords: rulesFromCSV(document.getElementById('rules-keywords-input')?.value || ''),
        body: document.getElementById('rules-rule-input')?.value || '',
        design: document.getElementById('rules-design-input')?.value || ''
    };
}

async function rulesSave() {
    const status = document.getElementById('rules-status');
    const payload = rulesCollectPayload();
    const isNew = !rulesState.selected;
    if (!payload.id) {
        if (status) status.textContent = rulesText('config.rules.id_required', 'Rule ID is required.');
        return;
    }
    try {
        const url = isNew ? '/api/config/rules' : '/api/config/rules/' + encodeURIComponent(payload.id);
        const resp = await fetch(url, {
            method: isNew ? 'POST' : 'PUT',
            credentials: 'same-origin',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });
        if (!resp.ok) {
            const data = await resp.json().catch(() => ({}));
            throw new Error(data.error || ('HTTP ' + resp.status));
        }
        rulesState.selected = payload.id;
        if (typeof showToast === 'function') showToast(rulesText('config.rules.saved', 'Rule saved'), 'success');
        await rulesLoad(payload.id);
    } catch (e) {
        if (status) status.textContent = rulesText('config.rules.save_failed', 'Failed to save rule') + ': ' + e.message;
        if (typeof showToast === 'function') showToast(rulesText('config.rules.save_failed', 'Failed to save rule'), 'error');
    }
}

async function rulesDelete() {
    const id = rulesState.selected;
    if (!id) return;
    if (!(await showConfirm(rulesText('config.rules.confirm_delete_title', 'Delete rule'), rulesText('config.rules.confirm_delete', 'Delete this rule override?', { id })))) return;
    try {
        const resp = await fetch('/api/config/rules/' + encodeURIComponent(id), {
            method: 'DELETE',
            credentials: 'same-origin'
        });
        if (!resp.ok) throw new Error('HTTP ' + resp.status);
        rulesState.selected = null;
        if (typeof showToast === 'function') showToast(rulesText('config.rules.deleted', 'Rule deleted'), 'success');
        await rulesLoad();
    } catch (e) {
        if (typeof showToast === 'function') showToast(rulesText('config.rules.delete_failed', 'Failed to delete rule') + ': ' + e.message, 'error');
    }
}

async function rulesRestore() {
    const id = rulesState.selected;
    if (!id) return;
    if (!(await showConfirm(rulesText('config.rules.confirm_restore_title', 'Restore rule'), rulesText('config.rules.confirm_restore', 'Restore the built-in version of this rule?', { id })))) return;
    try {
        const resp = await fetch('/api/config/rules/' + encodeURIComponent(id) + '/restore', {
            method: 'POST',
            credentials: 'same-origin'
        });
        if (!resp.ok) throw new Error('HTTP ' + resp.status);
        if (typeof showToast === 'function') showToast(rulesText('config.rules.restored', 'Rule restored'), 'success');
        await rulesLoad(id);
    } catch (e) {
        if (typeof showToast === 'function') showToast(rulesText('config.rules.restore_failed', 'Failed to restore rule') + ': ' + e.message, 'error');
    }
}

function rulesRefreshSelectedCard() {
    document.querySelectorAll('.rules-card').forEach(card => {
        card.classList.toggle('rules-card-selected', card.dataset.ruleId === rulesState.selected);
    });
}
