(function () {
    'use strict';
    const esc = value => String(value ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
    const url = value => { try { const u = new URL(value); return ['https:', 'http:'].includes(u.protocol) && !u.username && !u.password ? esc(u.href) : ''; } catch (_) { return ''; } };
    function sources(items, tr) {
        return (items || []).map(s => `<details class="dt-source" data-id="${esc(s.id)}"><summary><span>${esc(s.title || s.url)}</span><small>${esc(tr(s.status))} · ${esc(new Date(s.retrieved_at).toLocaleDateString())}</small></summary>${url(s.url) ? `<a href="${url(s.url)}" target="_blank" rel="noopener noreferrer">${esc(s.url)}</a>` : `<span>${esc(s.url || s.locator)}</span>`}<blockquote>${esc(s.excerpt)}</blockquote></details>`).join('') || `<p class="dt-empty">${esc(tr('no_sources'))}</p>`;
    }
    function report(r, tr) {
        if (!r) return `<div class="dt-empty"><span class="dt-symbol" aria-hidden="true">⌕</span><h2>${esc(tr('empty_title'))}</h2><p>${esc(tr('empty_report'))}</p></div>`;
        const ids = new Map((r.findings || []).map(f => [f.id, f.source_id]));
        const used = new Set((r.blocks || []).flatMap(b => (b.evidence || []).map(id => ids.get(id))));
        const cited = (r.sources || []).filter(s => used.has(s.id));
        const ref = b => [...new Set((b.evidence || []).map(id => cited.findIndex(s => s.id === ids.get(id)) + 1))].filter(n => n > 0).map(n => `<a href="#" data-ref="${esc(cited[n-1].id)}" aria-label="${esc(tr('source'))} ${n}">[${n}]</a>`).join(' ');
        const blocks = (r.blocks || []).map(b => {
            if (b.type === 'heading') return `<h3>${esc(b.text)}</h3>`;
            if (b.type === 'table') return `<div class="dt-table"><table>${(b.rows || []).map((row, i) => `<tr>${row.map(cell => `<${i ? 'td' : 'th'}>${esc(cell)}</${i ? 'td' : 'th'}>`).join('')}</tr>`).join('')}</table></div><p class="dt-refs">${ref(b)}</p>`;
            if (b.type === 'list') return `<ul>${(b.items || []).map(item => `<li>${esc(item)}</li>`).join('')}</ul><p class="dt-refs">${ref(b)}</p>`;
            return `<${b.type === 'quote' ? 'blockquote' : 'p'}>${esc(b.text)} ${ref(b)}</${b.type === 'quote' ? 'blockquote' : 'p'}>`;
        }).join('');
        return `<article class="dt-report">${r.partial ? `<p class="dt-notice">${esc(tr('partial'))}</p>` : ''}<h1>${esc(r.title)}</h1><small>${esc(new Date(r.created_at).toLocaleDateString())} · ${esc(tr('revision'))} ${r.revision}</small><p class="dt-lead">${esc(r.summary)}</p>${blocks}${r.limitations ? `<h3>${esc(tr('limitations'))}</h3><p>${esc(r.limitations)}</p>` : ''}<h3>${esc(tr('sources'))}</h3><ol>${cited.map(s => `<li>${url(s.url) ? `<a href="${url(s.url)}" target="_blank" rel="noopener noreferrer">${esc(s.title || s.url)}</a>` : esc(s.title || s.locator || s.url)}</li>`).join('')}</ol></article>`;
    }
    function activityItems(events, tr) {
        return (events || []).map(e => `<li data-event="${esc(String(e.id))}"><time>${esc(new Date(e.at).toLocaleTimeString())}</time><strong>${esc(tr(e.kind === 'auto_update' ? 'auto_title' : e.kind))}</strong><span>${esc(e.kind === 'auto_update' ? tr('auto_' + e.text) : ['status', 'phase', 'model', 'recovery'].includes(e.kind) ? tr(e.text) : e.text)}</span></li>`).join('');
    }
    function activity(events, tr) {
        return `<ol class="dt-activity">${activityItems(events, tr)}</ol>`;
    }
    function autoSettings(value, tr, readonly) {
        const a = value || {}, hours = a.interval_hours || 24;
        const days = hours % 24 === 0;
        const options = (values, selected) => values.map(x => `<option value="${x}"${x === selected ? ' selected' : ''}>${esc(tr(x === 'append' || x === 'rewrite' ? 'auto_' + x : x))}</option>`).join('');
        return `<fieldset class="dt-auto-settings"${readonly ? ' disabled' : ''}><legend>${esc(tr('auto_title'))}</legend><label class="dt-auto-toggle"><input type="checkbox" name="auto_enabled"${a.enabled ? ' checked' : ''}>${esc(tr('auto_enabled'))}</label><div class="dt-auto-grid"><label>${esc(tr('auto_interval'))}<input type="number" name="auto_interval" min="1" max="${days ? 365 : 8760}" step="1" required value="${days ? hours / 24 : hours}"></label><label>${esc(tr('auto_unit'))}<select name="auto_unit"><option value="hours"${!days ? ' selected' : ''}>${esc(tr('auto_hours'))}</option><option value="days"${days ? ' selected' : ''}>${esc(tr('auto_days'))}</option></select></label><label>${esc(tr('auto_style'))}<select name="auto_style">${options(['append','rewrite'], a.report_style || 'append')}</select></label><label>${esc(tr('auto_effort'))}<select name="auto_effort">${options(['quick','normal','maximum'], a.effort || 'quick')}</select></label></div><p class="dt-auto-help">${esc(tr('auto_help'))}</p></fieldset>`;
    }
    function autoStatus(a, tr) {
        a = a || {};
        const date = raw => raw && !raw.startsWith('0001-') && Number.isFinite(Date.parse(raw)) ? new Date(raw).toLocaleString() : '—';
        const status = a.enabled ? (a.outcome || 'scheduled') : 'paused';
        return `<strong>${esc(tr('auto_' + status))}</strong><dl><div><dt>${esc(tr('auto_next'))}</dt><dd>${esc(date(a.next_run_at))}</dd></div><div><dt>${esc(tr('auto_checked'))}</dt><dd>${esc(date(a.last_checked_at))}</dd></div><div><dt>${esc(tr('auto_changed'))}</dt><dd>${esc(date(a.last_changed_at))}</dd></div></dl>${a.error ? `<p>${esc(tr(a.error))}</p>` : ''}`;
    }
    // Revision selection is pure: the app owns polling, the view owns which revision is shown.
    // A revision that is not in the known list falls back to the newest one, so a stale
    // selection (case switch, "New", an archived revision) can never leave the export bar
    // without a report.
    function knownReports(current, history, selected) {
        const map = new Map([...(history || []), ...(selected ? [selected] : []), ...(current?.reports || [])].map(report => [report.revision, report]));
        return [...map.values()].sort((a, b) => a.revision - b.revision);
    }
    function chosenReport(current, revision, history, selected) {
        const reports = knownReports(current, history, selected);
        if (!revision) return reports.at(-1);
        return selected || reports.find(report => report.revision === revision) || reports.at(-1);
    }
    // Stop is a state write on the server, not provider work: it stays available whenever the
    // integration is enabled, even if the research runner is not ready yet.
    function canStop(caps) { return !!(caps && caps.enabled); }
    window.DetectiveViews = { esc, report, sources, activity, activityItems, autoSettings, autoStatus, knownReports, chosenReport, canStop };
})();
