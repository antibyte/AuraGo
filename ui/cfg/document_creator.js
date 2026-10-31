// cfg/document_creator.js — Document Creator config section module

let _dcSection = null;

async function renderDocumentCreatorSection(section) {
    if (section) _dcSection = section; else section = _dcSection;
    const toolsCfg = configData.tools || {};
    const data = toolsCfg.document_creator || {};
    const enabled = data.enabled === true;
    const backend = data.backend || 'maroto';
    const gotCfg = data.gotenberg || {};
    const form = window.AuraConfigForm;
    const key = name => t('config.document_creator.' + name);

    // The switch re-renders the section: the backend settings only exist while the feature is on.
    const enableToggle = form.toggle({ label: key('enabled_label'), help: key('enabled_help'), path: 'tools.document_creator.enabled', value: enabled, onclick: "toggleBool(this);dcUpdateEnabled(this.classList.contains('on'))" });
    const groups = [{ titleKey: 'config.refresh.operation', fields: [enableToggle] }];
    let after = '';

    if (!enabled) {
        after = form.emptyState({ title: key('disabled_notice'), desc: key('disabled_desc') });
    } else {
        const radio = (value, labelKey) => '<label class="dc-radio-label">'
            + '<input type="radio" name="dc_backend" data-path="tools.document_creator.backend" value="' + value + '"' + (backend === value ? ' checked' : '') + ' onchange="dcSwitchBackend(this.value)">'
            + '<span class="dc-radio-text">' + escapeHtml(key(labelKey)) + '</span></label>';
        const helpBackend = t('help.document_creator.backend');
        const backendField = '<div class="field-group dc-field-group-top" role="radiogroup" aria-labelledby="dc-backend-label"' + (helpBackend ? ' aria-describedby="dc-backend-help"' : '') + '>'
            + '<div class="field-label" id="dc-backend-label">' + escapeHtml(key('backend_label')) + '</div>'
            + (helpBackend ? '<div class="field-help" id="dc-backend-help">' + escapeHtml(helpBackend) + '</div>' : '')
            + '<div class="dc-backend-row">' + radio('maroto', 'backend_maroto') + radio('gotenberg', 'backend_gotenberg') + '</div></div>';
        groups.push({ titleKey: 'config.refresh.settings', fields: [
            backendField,
            form.field({ label: key('output_dir_label'), help: t('help.document_creator.output_dir'), path: 'tools.document_creator.output_dir', value: data.output_dir || 'data/documents', placeholder: 'data/documents' })
        ] });
        // Hidden, not removed, while another backend is selected (dcSwitchBackend toggles it).
        const gotenberg = form.panel({ title: key('gotenberg_title'), content:
            form.toggle({ label: key('allow_remote_resources_label'), help: t('help.document_creator.allow_remote_resources'), path: 'tools.document_creator.allow_remote_resources', value: data.allow_remote_resources === true })
            + form.field({ type: 'url', label: key('gotenberg_url_label'), help: t('help.document_creator.gotenberg_url'), path: 'tools.document_creator.gotenberg.url', value: gotCfg.url || '' })
            + form.number({ label: key('gotenberg_timeout_label'), help: t('help.document_creator.gotenberg_timeout'), path: 'tools.document_creator.gotenberg.timeout', value: gotCfg.timeout || 120, min: 5, max: 600, unit: key('seconds') })
            + form.actions([
                { html: '<button class="btn-save cfg-test-btn" onclick="dcTestGotenberg()" id="dc-test-btn" data-config-action-draft="true">' + cfgIcon('plug') + '<span>' + escapeHtml(key('test_gotenberg')) + '</span></button>' },
                { html: cfgActionResult.html('dc-test-result') }
            ])
            + '<div class="field-help">' + escapeHtml(key('test_saved_settings')) + '</div>' });
        after = '<div id="dc-gotenberg-fields"' + (backend === 'gotenberg' ? '' : ' class="is-hidden"') + '>' + gotenberg + '</div>';
    }

    document.getElementById('content').innerHTML = form.renderSpec({ label: section.label, desc: section.desc, groups, afterHTML: after });
    attachChangeListeners();
}

function dcUpdateEnabled(on) {
    if (!configData.tools) configData.tools = {};
    if (!configData.tools.document_creator) configData.tools.document_creator = {};
    configData.tools.document_creator.enabled = on;
    setDirty(true);
    renderDocumentCreatorSection(null);
}

function dcSwitchBackend(val) {
    if (!configData.tools) configData.tools = {};
    if (!configData.tools.document_creator) configData.tools.document_creator = {};
    configData.tools.document_creator.backend = val;
    setDirty(true);
    var el = document.getElementById('dc-gotenberg-fields');
    setHidden(el, val !== 'gotenberg');
}

// Checks the URL typed in (unsaved included). The restart hint for the managed
// container still looks at the saved configuration on the server.
async function dcTestGotenberg() {
    return cfgActionResult(document.getElementById('dc-test-btn'), () => fetch('/api/document-creator/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ draft: cfgDraftPatch(['tools.document_creator']) })
    }), {
        target: 'dc-test-result',
        pending: t('config.document_creator.loading'),
        success: t('config.document_creator.test_verified'),
        failure: outcome => {
            const body = outcome.body || {};
            if (outcome.httpStatus >= 400) return t('config.document_creator.status_error');
            return t('config.document_creator.' + (body.reachable === true ? 'test_unverified' : 'test_unreachable'));
        },
        interpret: outcome => {
            const body = outcome.body || {};
            if (outcome.httpStatus >= 400) return null;
            // A running but outdated managed container is a warning, not a failed URL.
            if (body.restart_required) return { state: 'warning', reason: t('config.document_creator.restart_required') };
            if (body.status === 'success' && body.security_verified === true) return { state: 'success', reason: '' };
            return { state: 'error' };
        }
    });
}
