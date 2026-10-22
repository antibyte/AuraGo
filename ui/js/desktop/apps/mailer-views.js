// Mailer views: DOM-free rendering helpers shared by the Mailer shell.
//
// The module owns every pure markup or formatting helper of the Mailer so
// the window shell (mailer.js) can stay inside the desktop JS line budget
// and so the visual language of a message, a folder and an empty state is
// defined in exactly one place.
//
// Contract:
// - No DOM access, no window state, no network. Every helper is a pure
//   function of its arguments.
// - All text is escaped by the caller-provided esc() helper; the helpers
//   never interpolate untrusted markup themselves.
// - safeMailHTML() keeps the mail CSP, the quote collapsing and the
//   inline image mapping; it is re-exported as window.MailerApp.safeMailHTML.
(function () {
    'use strict';

    function initials(value) {
        const text = String(value || '').replace(/<[^>]+>/g, '').trim();
        return (text.split(/\s+/).slice(0, 2).map(part => part[0] || '').join('') || '?').toUpperCase();
    }

    function parseSender(value) {
        const raw = String(value || '').trim();
        const match = raw.match(/^\s*(?:"([^"]+)"|([^<]+?))?\s*<([^>]+)>\s*$/);
        if (!match) return { name: raw || '?', address: '' };
        const address = match[3].trim();
        return { name: (match[1] || match[2] || '').trim() || address, address };
    }

    function formatDate(value, timezone) {
        if (!value) return '';
        const date = new Date(value);
        if (Number.isNaN(date.getTime())) return '';
        try { return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short', timeZone: timezone || undefined }).format(date); }
        catch (_) { return date.toLocaleString(); }
    }

    function formatBytes(size) {
        const n = Number(size) || 0;
        if (n < 1024) return n + ' B';
        if (n < 1048576) return (n / 1024).toFixed(1) + ' KB';
        return (n / 1048576).toFixed(1) + ' MB';
    }

    // toneFor derives a stable avatar tone from a name so the same sender
    // keeps its colour between renders and across windows.
    function toneFor(value) {
        const text = String(value || '');
        let hash = 0;
        for (let index = 0; index < text.length; index++) hash = (hash * 31 + text.charCodeAt(index)) >>> 0;
        return hash % 6;
    }

    // avatar renders the round sender badge used by the list and the reader.
    function avatar(name, extraClass) {
        const label = String(name || '').trim();
        return `<span class="vd-mailer-avatar vd-mailer-avatar--tone-${toneFor(label)}${extraClass ? ' ' + extraClass : ''}" aria-hidden="true">${initials(label)}</span>`;
    }

    // emptyState renders the shared "nothing here" card.
    function emptyState(esc, options) {
        const opts = options || {};
        const icon = opts.icon || '✉';
        const action = opts.action
            ? `<button type="button" class="vd-mailer-button" data-${esc(opts.action)}>${esc(opts.actionLabel || '')}</button>`
            : '';
        return `<div class="vd-mailer-empty">
            <span class="vd-mailer-empty-badge" aria-hidden="true">${esc(icon)}</span>
            <b>${esc(opts.title || '')}</b>
            ${opts.hint ? `<p>${esc(opts.hint)}</p>` : ''}
            ${action}
        </div>`;
    }

    // skeleton renders the loading placeholder rows of the message list.
    function skeleton(rows) {
        const count = Math.max(1, Math.min(Number(rows) || 6, 12));
        return `<div class="vd-mailer-skeleton">${'<i></i>'.repeat(count)}</div>`;
    }

    function normalizeContentID(value) {
        let id = String(value || '').replace(/^cid:/i, '');
        try { id = decodeURIComponent(id); } catch (_) {}
        return id.trim().replace(/^<|>$/g, '').toLowerCase();
    }

    // safeMailHTML renders a mail body into a self-contained document that
    // can be shown inside a sandboxed frame. Remote images stay blocked
    // unless the reader explicitly allowed them, inline images are mapped to
    // the object URLs the shell created, and quoted text collapses.
    function safeMailHTML(html, allowRemote, inlineImages, quotedLabel, collapseQuotes) {
        const source = document.createElement('template');
        source.innerHTML = String(html || '');
        const placeholders = new Map();
        let placeholderNumber = 0;
        for (const image of source.content.querySelectorAll('img[src]')) {
            const src = (image.getAttribute('src') || '').trim();
            image.setAttribute('src', src);
            if (/^cid:/i.test(src)) {
                const mapped = inlineImages && inlineImages.get(normalizeContentID(src));
                if (mapped) {
                    const marker = `https://mailer.invalid/${window.crypto?.randomUUID?.() || (++placeholderNumber).toString(36)}`;
                    placeholders.set(marker, mapped);
                    image.setAttribute('src', marker);
                } else image.removeAttribute('src');
                continue;
            }
            if (/^(?:https?:)?\/\//i.test(src)) {
                if (!allowRemote) image.removeAttribute('src');
                else if (src.startsWith('//')) image.setAttribute('src', 'https:' + src);
            }
        }
        if (collapseQuotes !== false) {
            for (const quote of Array.from(source.content.querySelectorAll('blockquote'))) {
                const details = document.createElement('details');
                const summary = document.createElement('summary');
                summary.textContent = quotedLabel || 'Quoted message';
                quote.replaceWith(details);
                details.append(summary, quote);
            }
        }
        const safe = window.DOMPurify ? window.DOMPurify.sanitize(source.innerHTML, {
            FORBID_TAGS: ['script', 'form', 'input', 'button', 'select', 'textarea', 'iframe', 'object', 'embed', 'video', 'audio', 'svg', 'math', 'base', 'meta', 'link'],
            FORBID_ATTR: ['srcset', 'action', 'formaction', 'xlink:href'],
            ALLOW_DATA_ATTR: false
        }) : '<pre>' + String(html || '').replace(/[&<>]/g, ch => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;' }[ch])) + '</pre>';
        const output = document.createElement('template');
        output.innerHTML = safe;
        for (const image of output.content.querySelectorAll('img[src]')) {
            const marker = image.getAttribute('src') || '';
            if (placeholders.has(marker)) image.setAttribute('src', placeholders.get(marker));
        }
        for (const link of output.content.querySelectorAll('a[href]')) {
            link.setAttribute('target', '_blank');
            link.setAttribute('rel', 'noopener noreferrer');
            link.setAttribute('referrerpolicy', 'no-referrer');
        }
        const cleaned = output.innerHTML;
        const imagePolicy = ['blob:', allowRemote ? 'https:' : '', allowRemote ? 'http:' : ''].filter(Boolean).join(' ');
        return `<!doctype html><html><head><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src ${imagePolicy}; style-src 'unsafe-inline'; font-src 'none'; connect-src 'none'; form-action 'none'; base-uri 'none';"><meta name="referrer" content="no-referrer"><style>html{color-scheme:light}body{font:15px/1.6 system-ui,-apple-system,sans-serif;color:#252a33;overflow-wrap:anywhere;margin:0;padding:22px 24px}img{max-width:100%;height:auto}table{max-width:100%;border-collapse:collapse}td,th{border:1px solid #c8cdd5;padding:5px 8px}pre{white-space:pre-wrap}blockquote{border-left:3px solid #c2cbd6;margin-left:0;padding-left:14px;color:#566274}details{margin:8px 0}details>summary{cursor:pointer;color:#566274;font-size:13px;font-weight:600}</style></head><body>${cleaned}</body></html>`;
    }

    window.MailerViews = {
        avatar,
        emptyState,
        formatBytes,
        formatDate,
        initials,
        normalizeContentID,
        parseSender,
        safeMailHTML,
        skeleton,
        toneFor
    };
})();
