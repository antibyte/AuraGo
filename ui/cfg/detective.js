// The administrator-owned operation allowlist: one "tool: op, op" line per tool. A
// missing colon or an empty operation list is not stored, so a typo cannot save a blank
// policy; such lines are named inline (detectiveAllowlistInvalidLines); the server normalizes the map again.
function detectiveAllowlistText(map) {
    return Object.entries(map || {}).sort(([a], [b]) => a.localeCompare(b))
        .map(([tool, ops]) => tool + ': ' + (ops || []).join(', ')).join('\n');
}
function detectiveAllowlistMap(text) {
    const map = {};
    for (const raw of String(text || '').split('\n')) {
        const line = raw.trim();
        if (!line) continue;
        const at = line.indexOf(':');
        if (at <= 0) continue;
        const tool = line.slice(0, at).trim();
        const operations = line.slice(at + 1).split(',').map(op => op.trim()).filter(Boolean);
        if (tool && operations.length) map[tool] = operations;
    }
    return map;
}
// 1-based numbers of the non-empty lines detectiveAllowlistMap leaves out.
function detectiveAllowlistInvalidLines(text) {
    const invalid = [];
    String(text || '').split('\n').forEach((raw, index) => {
        const line = raw.trim();
        if (!line) return;
        const at = line.indexOf(':');
        const operations = at > 0 ? line.slice(at + 1).split(',').map(op => op.trim()).filter(Boolean) : [];
        if (at <= 0 || !line.slice(0, at).trim() || !operations.length) invalid.push(index + 1);
    });
    return invalid;
}
function detectiveShowAllowlistErrors(area) {
    const message = document.getElementById('detective-allowlist-validation');
    if (!message) return;
    const invalid = detectiveAllowlistInvalidLines(area.value);
    message.textContent = invalid.length ? t(invalid.length === 1 ? 'config.detective.extra_read_invalid_one' : 'config.detective.extra_read_invalid_many', { lines: invalid.join(', ') }) : '';
    message.hidden = !invalid.length;
    if (invalid.length) area.setAttribute('aria-invalid', 'true'); else area.removeAttribute('aria-invalid');
}
function syncDetectiveAllowlist(area) {
    if (!configData.detective) configData.detective = {enabled: true, readonly: false};
    configData.detective.extra_read_operations = detectiveAllowlistMap(area.value);
    detectiveShowAllowlistErrors(area);
    setDirty(true);
}
// Server-owned research profiles. Existing runs keep their accepted budgets.
function renderDetectiveSection(section) {
    const data = configData.detective || (configData.detective = {enabled:true,readonly:false});
    const defaults = {quick:{seconds:300,tools:40,iterations:60,tokens:0},normal:{seconds:900,tools:100,iterations:140,tokens:0},maximum:{seconds:3600,tools:250,iterations:320,tokens:0}};
    const form = window.AuraConfigForm;
    const label = key => t('config.detective.'+key);
    const groups = [{ titleKey: 'config.refresh.operation', fields: ['enabled','readonly'].map(key =>
        form.toggle({ label: label(key), help: label('help_'+key), path: 'detective.'+key, value: !!data[key] })) }];
    for (const effort of ['quick','normal','maximum']) {
        const p = {...defaults[effort],...(data.profiles?.[effort]||{})};
        groups.push({ title: label(effort), fields: [['seconds',30,7200],['tools',1,500],['iterations',1,750],['tokens',0,10000000]].map(([key,min,max]) =>
            form.number({ label: label(effort)+': '+label(key), help: label('help_'+key), path: 'detective.profiles.'+effort+'.'+key, value: Number(p[key]), min, max })) });
    }
    const allowlist = '<div class="field-group pw-field"><div class="field-label" id="detective-allowlist-label">'+escapeHtml(label('extra_read_operations'))+'</div>'+
        '<textarea class="field-input" id="detective-allowlist" rows="4" aria-labelledby="detective-allowlist-label" aria-describedby="detective-allowlist-help detective-allowlist-format detective-allowlist-validation" spellcheck="false" placeholder="mcp_call: archive/read" data-role="detective-allowlist" oninput="syncDetectiveAllowlist(this)">'+
        escapeHtml(detectiveAllowlistText(data.extra_read_operations))+'</textarea>'+
        '<div class="field-help" id="detective-allowlist-help">'+escapeHtml(label('extra_read_help'))+'</div>'+
        '<div class="field-hint" id="detective-allowlist-format">'+escapeHtml(label('extra_read_format'))+'</div>'+
        '<div class="pw-field-error" id="detective-allowlist-validation" role="alert" hidden></div></div>';
    groups.push({ title: label('extra_read_operations'), content: allowlist });
    document.getElementById('content').innerHTML = form.renderSpec({ label: section.label, desc: section.desc, groups,
        beforeHTML: '<div class="cfg-note-banner">'+escapeHtml(label('restart'))+'</div>' });
    attachChangeListeners();
}
