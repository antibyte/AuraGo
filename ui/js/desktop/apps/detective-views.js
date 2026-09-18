(function () {
    'use strict';
    const esc = value => String(value ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
    const url = value => { try { const u = new URL(value); return ['https:', 'http:'].includes(u.protocol) && !u.username && !u.password ? esc(u.href) : ''; } catch (_) { return ''; } };
    function sources(items, tr) {
        return (items || []).map(s => `<details class="dt-source"><summary><span>${esc(s.title || s.url)}</span><small>${esc(tr(s.status))} · ${esc(new Date(s.retrieved_at).toLocaleDateString())}</small></summary>${s.url ? `<a href="${url(s.url)}" target="_blank" rel="noopener noreferrer">${esc(s.url)}</a>` : `<span>${esc(s.locator)}</span>`}<blockquote>${esc(s.excerpt)}</blockquote></details>`).join('') || `<p class="dt-empty">${esc(tr('no_sources'))}</p>`;
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
        return `<article class="dt-report">${r.partial ? `<p class="dt-notice">${esc(tr('partial'))}</p>` : ''}<h1>${esc(r.title)}</h1><small>${esc(new Date(r.created_at).toLocaleDateString())} · ${esc(tr('revision'))} ${r.revision}</small><p class="dt-lead">${esc(r.summary)}</p>${blocks}${r.limitations ? `<h3>${esc(tr('limitations'))}</h3><p>${esc(r.limitations)}</p>` : ''}<h3>${esc(tr('sources'))}</h3><ol>${cited.map(s => `<li>${s.url ? `<a href="${url(s.url)}" target="_blank" rel="noopener noreferrer">${esc(s.title || s.url)}</a>` : esc(s.title || s.locator)}</li>`).join('')}</ol></article>`;
    }
    function activity(events, tr) {
        return `<ol class="dt-activity">${events.map(e => `<li><time>${esc(new Date(e.at).toLocaleTimeString())}</time><strong>${esc(tr(e.kind))}</strong><span>${esc(['status', 'phase', 'model', 'recovery'].includes(e.kind) ? tr(e.text) : e.text)}</span></li>`).join('')}</ol>`;
    }
    window.DetectiveViews = { esc, report, sources, activity };
})();
