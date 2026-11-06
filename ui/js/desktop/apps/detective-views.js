(function () {
    'use strict';
    const esc = value => String(value ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
    const url = value => { try { const u = new URL(value); return ['https:', 'http:'].includes(u.protocol) && !u.username && !u.password ? esc(u.href) : ''; } catch (_) { return ''; } };
    // One inline glyph set for the whole app. Every icon is decorative: the label next
    // to it carries the meaning, so the glyphs stay aria-hidden.
    const ICONS = {
        mark: '<circle cx="10.5" cy="10.5" r="7"/><path d="m20 20-3.6-3.6"/><path d="M8 8.5h5M8 12h3.5"/>',
        plus: '<path d="M12 5v14M5 12h14"/>',
        report: '<path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z"/><path d="M14 3v5h5"/><path d="M9 13h6M9 17h4"/>',
        sources: '<path d="M10.5 13.5a4.5 4.5 0 0 0 6.4 0l2-2a4.5 4.5 0 0 0-6.4-6.4l-1 1"/><path d="M13.5 10.5a4.5 4.5 0 0 0-6.4 0l-2 2a4.5 4.5 0 0 0 6.4 6.4l1-1"/>',
        activity: '<path d="M3 12h4l2.5-7 4 14L16 12h5"/>',
        archive: '<path d="M3 7h18v4H3z"/><path d="M5 11v9h14v-9"/><path d="M10 15h4"/>',
        play: '<path d="M7.5 4.8 19 12 7.5 19.2z"/>',
        stop: '<rect x="6.5" y="6.5" width="11" height="11" rx="2"/>',
        check: '<path d="m5 12.5 4.5 4.5L19 7"/>',
        deepen: '<circle cx="10.5" cy="10.5" r="6.5"/><path d="m20 20-4.2-4.2M10.5 7.5v6M7.5 10.5h6"/>',
        trash: '<path d="M4 7h16"/><path d="M10 11v6M14 11v6"/><path d="M6.5 7 7.5 20h9L17.5 7"/><path d="M9.5 7V4h5v3"/>',
        download: '<path d="M12 4v10"/><path d="m8 11 4 4 4-4"/><path d="M5 19h14"/>',
        pen: '<path d="M4 20h4L19.5 8.5a2.8 2.8 0 0 0-4-4L4 16z"/>',
        mail: '<rect x="3" y="5" width="18" height="14" rx="2"/><path d="m3.5 7 8.5 6 8.5-6"/>',
        refresh: '<path d="M20 12a8 8 0 1 1-2.6-5.9"/><path d="M20 4v4.5h-4.5"/>',
        clock: '<circle cx="12" cy="12" r="8"/><path d="M12 7.5V12l3 2"/>',
        search: '<circle cx="11" cy="11" r="7"/><path d="m20 20-3.6-3.6"/>',
        external: '<path d="M14 4h6v6"/><path d="M20 4 11 13"/><path d="M18 14v4a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4"/>',
        plan: '<path d="M9 6h11M9 12h11M9 18h11"/><path d="M4.5 6h.01M4.5 12h.01M4.5 18h.01"/>',
        flag: '<path d="M5 21V4"/><path d="M5 5h11l-2 3.5L16 12H5z"/>',
        finding: '<path d="M12 3v3M12 18v3M3 12h3M18 12h3M5.6 5.6l2.1 2.1M16.3 16.3l2.1 2.1M18.4 5.6l-2.1 2.1M7.7 16.3l-2.1 2.1"/><circle cx="12" cy="12" r="3"/>',
        tool: '<path d="M15 4a5 5 0 0 0-4.6 7L4 17.4 6.6 20l6.4-6.4A5 5 0 1 0 15 4z"/>',
        question: '<circle cx="12" cy="12" r="9"/><path d="M9.5 9.5a2.5 2.5 0 1 1 3.4 2.3c-.6.3-.9.8-.9 1.4v.3"/><path d="M12 17h.01"/>',
        recovery: '<path d="M4 12a8 8 0 1 0 2.6-5.9"/><path d="M4 4v4.5h4.5"/>',
        auto: '<path d="M12 7v5l3 2"/><circle cx="12" cy="12" r="8"/><path d="M12 2v2M12 20v2M2 12h2M20 12h2"/>'
    };
    const icon = (name, className) => `<svg class="dt-ico${className ? ' ' + className : ''}" viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false">${ICONS[name] || ''}</svg>`;
    const ACTIVITY_ICON = {plan: 'plan', status: 'activity', phase: 'flag', finding: 'finding', tool: 'tool', question: 'question', recovery: 'recovery', auto_update: 'auto', model: 'activity'};
    function sources(items, tr) {
        // The case payload carries metadata only; an opened disclosure fetches its text.
        const text = s => s.excerpt ? `<blockquote>${esc(s.excerpt)}</blockquote>` : `<blockquote aria-busy="true">…</blockquote>`;
        return (items || []).map(s => `<details class="dt-source" data-id="${esc(s.id)}"><summary><span>${esc(s.title || s.url)}</span><small>${esc(tr(s.status))} · ${esc(new Date(s.retrieved_at).toLocaleDateString())}</small></summary>${url(s.url) ? `<a href="${url(s.url)}" target="_blank" rel="noopener noreferrer">${icon('external')}<span>${esc(s.url)}</span></a>` : `<span>${esc(s.url || s.locator)}</span>`}${text(s)}</details>`).join('') || `<p class="dt-empty">${esc(tr('no_sources'))}</p>`;
    }
    function report(r, tr) {
        if (!r) return `<div class="dt-empty">${icon('search', 'dt-empty-mark')}<h2>${esc(tr('empty_title'))}</h2><p>${esc(tr('empty_report'))}</p></div>`;
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
        return `<article class="dt-report">${r.partial ? `<p class="dt-notice">${icon('flag')}<span>${esc(tr('partial'))}</span></p>` : ''}<h2>${esc(r.title)}</h2><small>${esc(new Date(r.created_at).toLocaleDateString())} · ${esc(tr('revision'))} ${r.revision}</small><p class="dt-lead">${esc(r.summary)}</p>${blocks}${r.limitations ? `<h3>${esc(tr('limitations'))}</h3><p>${esc(r.limitations)}</p>` : ''}<h3>${esc(tr('sources'))}</h3><ol>${cited.map(s => `<li>${url(s.url) ? `<a href="${url(s.url)}" target="_blank" rel="noopener noreferrer">${esc(s.title || s.url)}</a>` : esc(s.title || s.locator || s.url)}</li>`).join('')}</ol></article>`;
    }
    function activityItems(events, tr) {
        return (events || []).map(e => `<li data-event="${esc(String(e.id))}"><time>${esc(new Date(e.at).toLocaleTimeString())}</time><strong>${icon(ACTIVITY_ICON[e.kind] || 'activity')}<span>${esc(tr(e.kind === 'auto_update' ? 'auto_title' : e.kind))}</span></strong><span>${esc(e.kind === 'auto_update' ? tr('auto_' + e.text) : ['status', 'phase', 'model', 'recovery'].includes(e.kind) ? tr(e.text) : e.text)}</span></li>`).join('');
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
    // Token counts are the only cost figure Detective shows; the budget tracker owns
    // pricing. An absent or zero count renders nothing.
    function tokenText(usage, tr) {
        const input = (usage && usage.prompt_tokens) || 0;
        const output = (usage && usage.completion_tokens) || 0;
        if (!input && !output) return '';
        const cached = (usage && usage.cached_tokens) || 0;
        const parts = [`${input.toLocaleString()} ${tr('tokens_in')}`, `${output.toLocaleString()} ${tr('tokens_out')}`];
        if (cached) parts.push(`${cached.toLocaleString()} ${tr('tokens_cached')}`);
        return `<span class="dt-tokens">${esc(tr('tokens'))}: ${esc(parts.join(' · '))}</span>`;
    }
    function autoStatus(a, tr, usage) {
        a = a || {};
        const date = raw => raw && !raw.startsWith('0001-') && Number.isFinite(Date.parse(raw)) ? new Date(raw).toLocaleString() : '—';
        const status = a.enabled ? (a.outcome || 'scheduled') : 'paused';
        return `<strong>${esc(tr('auto_' + status))}</strong><dl><div><dt>${esc(tr('auto_next'))}</dt><dd>${esc(date(a.next_run_at))}</dd></div><div><dt>${esc(tr('auto_checked'))}</dt><dd>${esc(date(a.last_checked_at))}</dd></div><div><dt>${esc(tr('auto_changed'))}</dt><dd>${esc(date(a.last_changed_at))}</dd></div></dl>${a.error ? `<p>${esc(tr(a.error))}</p>` : ''}${usage ? `<p>${tokenText(usage, tr)}</p>` : ''}`;
    }
    // Revision selection is pure: the app owns polling, the view owns which revision is shown.
    // A revision that is not in the known list falls back to the newest one, so a stale
    // selection (case switch, "New", an archived revision) can never leave the export bar
    // without a report.
    function knownReports(current, history, selected) {
        const map = new Map([...(history || []), ...(selected ? [selected] : []), ...(current?.reports || [])].map(report => [report.revision, report]));
        return [...map.values()].sort((a, b) => a.revision - b.revision);
    }
    // A provisional revision is the partial report a failed continue/decrease keeps; it
    // never becomes the default, so the newest full report stays on screen (audit 4.14).
    function chosenReport(current, revision, history, selected) {
        const reports = knownReports(current, history, selected);
        if (!revision) return reports.filter(report => !report.provisional).at(-1) || reports.at(-1);
        return selected || reports.find(report => report.revision === revision) || reports.at(-1);
    }
    // Stop is a state write on the server, not provider work: it stays available whenever the
    // integration is enabled, even if the research runner is not ready yet.
    function canStop(caps) { return !!(caps && caps.enabled); }
    // Polling: a case that is queued, running or waiting for an answer needs a short
    // interval; anything else can wait. A hidden window does not poll at all.
    const ACTIVE_RUN_STATUSES = ['queued', 'running', 'waiting_for_user'];
    function pollDelay(status) { return ACTIVE_RUN_STATUSES.includes(status) ? 3000 : 10000; }
    // An API refusal carries a stable code; the app shows a translated sentence for a
    // code it knows and keeps the server text only as the fallback (audit 4.14).
    function errorText(tr, err) {
        const body = (err && err.body) || {};
        const code = body.code || '';
        if (code) {
            const key = 'error_' + code;
            const text = tr(key);
            if (text && text !== key) return text;
        }
        return body.error || (err && err.message) || tr('error');
    }
    // The built-in PDF writer has no Arabic, Devanagari, Hebrew or emoji support, so Word
    // comes first and the PDF link names the limit before the download fails.
    function formats(caps) {
        const markdown = {id: 'md', label: 'Markdown'};
        const word = {id: 'docx', label: 'Word'};
        const pdf = {id: 'pdf', label: 'PDF', hint: caps && caps.pdf_scripts === false ? 'pdf_hint' : ''};
        return caps && caps.pdf_scripts === false ? [markdown, word, pdf] : [markdown, pdf, word];
    }
    window.DetectiveViews = { esc, icon, report, sources, activity, activityItems, autoSettings, autoStatus, knownReports, chosenReport, canStop, pollDelay, errorText, formats, tokenText };
})();
