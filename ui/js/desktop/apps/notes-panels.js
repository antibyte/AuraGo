/* Right-sidebar panels of the Notes app that need the link index: backlinks and outgoing links.
   Outline, info and assistant stay in notes.js. Rows use data-note-path (opens the note), data-create-wiki
   (creates the missing note) and data-attachment-path (opens an attachment), handled by the app's click handler. */
(function () {
    'use strict';
    const DIR = 'Documents/Notes';
    const generations = new WeakMap(), unlinkedOpen = new WeakMap();
    const escapeHTML = value => String(value ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
    const basename = path => String(path || '').split('/').pop();
    const folderOf = path => { const parts = String(path || '').split('/'); parts.pop(); const rel = parts.join('/'); return rel.startsWith(DIR + '/') ? rel.slice(DIR.length + 1) : rel === DIR ? '' : rel; };
    const noteName = path => basename(path).replace(/\.md$/i, '');
    // A new render of the same container invalidates every async step of the previous one.
    function begin(container) { const gen = (generations.get(container) || 0) + 1; generations.set(container, gen); return () => generations.get(container) === gen && container.isConnected; }

    function stripFrontmatter(content) { return window.NotesFrontmatter ? window.NotesFrontmatter.strip(String(content || '')) : String(content || ''); }

    // Lines of a note that contain a link resolving to target (code blocks skipped), trimmed for display.
    function contexts(content, from, target, index, limit = 3) {
        const lines = stripFrontmatter(content).split('\n'), found = [];
        let fence = '';
        for (const raw of lines) {
            const line = raw.replace(/\r$/, ''), trimmed = line.trim(), mark = /^(```+|~~~+)/.exec(trimmed);
            if (mark) { fence = !fence ? mark[1][0] : trimmed[0] === fence ? '' : fence; continue; }
            if (fence || !trimmed) continue;
            let hit = false;
            for (const match of line.matchAll(/(!?)\[\[([^\[\]\n]{1,500})\]\]/g)) if (index.resolve(from, match[2], !!match[1], 'wiki') === target) { hit = true; break; }
            if (!hit) for (const match of line.matchAll(/(!?)\[[^\]\n]*\]\(([^)\s]+)/g)) if (index.resolve(from, match[2].replace(/^<|>$/g, ''), !!match[1], 'markdown') === target) { hit = true; break; }
            if (hit) { found.push(trimmed.length > 180 ? trimmed.slice(0, 177) + '…' : trimmed); if (found.length >= limit) break; }
        }
        return found;
    }

    // One context line as safe HTML: everything is escaped; wikilinks show their display text, light inline marks are stripped,
    // #tags become small chips. Links resolving to `target` (the note whose backlinks are shown) are emphasised.
    const INLINE = /(!?)\[\[([^\[\]\n]{1,500})\]\]|(!?)\[([^\]\n]*)\]\(([^)\s]+)[^)]*\)|`([^`\n]+)`|\*\*([^*\n]+)\*\*|__([^_\n]+)__|~~([^~\n]+)~~|==([^=\n]+)==|\*([^*\n]+)\*|(^|[\s(])#([\p{L}\p{N}_\/-]*[\p{L}_\/-][\p{L}\p{N}_\/-]*)/gu;
    function contextHtml(line, from, target, index, esc = escapeHTML) {
        const text = String(line == null ? '' : line);
        let out = '', pos = 0;
        const resolves = (value, embed, kind) => { try { return !!index && !!target && index.resolve(from, value, !!embed, kind) === target; } catch (_) { return false; } };
        for (const m of text.matchAll(INLINE)) {
            out += esc(text.slice(pos, m.index));
            pos = m.index + m[0].length;
            if (m[2] !== undefined) {
                const parsed = window.NotesIndex ? window.NotesIndex.parseWikiValue(m[2]) : {target: m[2], subpath: '', alias: ''};
                const shown = (parsed.alias || (parsed.target ? parsed.target + (parsed.subpath ? ' › ' + parsed.subpath : '') : parsed.subpath) || m[2]).trim();
                out += '<span class="notes-context-link' + (resolves(m[2], m[1], 'wiki') ? ' is-current' : '') + '">' + esc(shown) + '</span>';
            } else if (m[5] !== undefined) {
                const href = m[5].replace(/^<|>$/g, '');
                out += '<span class="notes-context-link' + (resolves(href, m[3], 'markdown') ? ' is-current' : '') + '">' + esc(m[4] || href) + '</span>';
            } else if (m[13] !== undefined) {
                out += esc(m[12]) + '<span class="notes-context-tag">#' + esc(m[13]) + '</span>';
            } else {
                out += esc(m[6] ?? m[7] ?? m[8] ?? m[9] ?? m[10] ?? m[11]);
            }
        }
        return out + esc(text.slice(pos));
    }

    function row(esc, attrs, title, sub, extra = '') {
        return '<button type="button" class="notes-link-row" ' + attrs + '><span class="notes-link-main"><strong>' + esc(title) + '</strong>' + extra + '</span>' + (sub ? '<span class="notes-link-sub">' + esc(sub) + '</span>' : '') + '</button>';
    }

    // opts: {tr, esc, index, path, request, signal}
    // Identity of what the backlinks list shows: the linking notes with their modified time and mention count.
    function backlinksSignature(index, path) {
        if (!path || !index) return '';
        return path + '\n' + index.backlinks(path).map(item => [item.path, index.note?.(item.path)?.modified || '', item.count].join('|')).join('\n');
    }
    function backlinks(container, opts) {
        const alive = begin(container), esc = opts.esc || escapeHTML, tr = opts.tr, index = opts.index, path = opts.path;
        if (!path || !index) { container.textContent = ''; return; }
        const linked = index.backlinks(path);
        container.innerHTML = '<section class="notes-linked" data-linked><h3>' + esc(tr('linked_mentions')) + ' <span class="notes-count">' + linked.length + '</span></h3>' +
            (linked.length ? linked.map(item => '<div class="notes-link-item" data-linked-item="' + esc(item.path) + '">' + row(esc, 'data-note-path="' + esc(item.path) + '"', item.title || noteName(item.path), folderOf(item.path), '<span class="notes-count" title="' + esc(tr('mentions_count', {count: item.count})) + '">' + item.count + '</span>') + '<div class="notes-link-contexts" data-contexts></div></div>').join('') : '<p class="notes-muted">' + esc(tr('no_backlinks')) + '</p>') + '</section>' +
            '<details class="notes-unlinked" data-unlinked><summary>' + esc(tr('unlinked_mentions')) + '</summary><div data-unlinked-body></div></details>';
        // Context lines are fetched lazily, one note at a time; opts.cache keeps them per (target, note, modified, count).
        const cache = opts.cache, show = (item, lines) => {
            const host = [...container.querySelectorAll('[data-linked-item]')].find(el => el.dataset.linkedItem === item.path)?.querySelector('[data-contexts]');
            if (host) host.innerHTML = lines.map(line => '<p class="notes-link-context">' + contextHtml(line, item.path, path, index, esc) + '</p>').join('');
        };
        (async () => {
            for (const item of linked) {
                if (!alive()) return;
                const key = cache ? [path, item.path, index.note?.(item.path)?.modified || '', item.count].join('|') : '';
                if (key && cache.has(key)) { show(item, cache.get(key)); continue; }
                try {
                    const note = await opts.request('/api/desktop/notes?path=' + encodeURIComponent(item.path), {signal: opts.signal});
                    if (!alive()) return;
                    const lines = contexts(note.content, item.path, path, index);
                    if (key) { cache.set(key, lines); if (cache.size > 200) cache.delete(cache.keys().next().value); }
                    show(item, lines);
                } catch (error) { if (error?.name === 'AbortError') return; }
            }
        })();
        const details = container.querySelector('[data-unlinked]'), body = container.querySelector('[data-unlinked-body]');
        let loaded = false;
        const kept = unlinkedOpen.get(container);
        details.addEventListener('toggle', async () => {
            unlinkedOpen.set(container, {path, open: details.open});
            if (!details.open || loaded) return;
            loaded = true;
            const title = String(opts.title || noteName(path)).trim();
            if (!title) return;
            body.innerHTML = '<p class="notes-muted">' + esc(tr('loading')) + '</p>';
            try {
                const result = await opts.request('/api/desktop/notes?' + new URLSearchParams({q: title, limit: '30'}), {signal: opts.signal});
                if (!alive()) return;
                const skip = new Set([path, ...linked.map(item => item.path)]);
                const hits = (result.notes || []).filter(note => !skip.has(note.path)).slice(0, 12);
                body.innerHTML = hits.length ? hits.map(note => '<div class="notes-link-item">' + row(esc, 'data-note-path="' + esc(note.path) + '"', note.title || noteName(note.path), folderOf(note.path)) + (note.snippet ? '<div class="notes-link-contexts"><p class="notes-link-context">' + esc(note.snippet) + '</p></div>' : '') + '</div>').join('') : '<p class="notes-muted">' + esc(tr('no_unlinked')) + '</p>';
            } catch (error) { if (alive() && error?.name !== 'AbortError') { loaded = false; body.innerHTML = '<p class="notes-muted">' + esc(tr('request_failed')) + '</p>'; } }
        });
        // A refresh of the same note keeps the section open.
        if (kept && kept.path === path && kept.open) details.open = true;
    }

    // opts: {tr, esc, index, path, content}
    function outgoing(container, opts) {
        const alive = begin(container), esc = opts.esc || escapeHTML, tr = opts.tr, index = opts.index;
        if (!opts.path || !index) { container.textContent = ''; return; }
        const seen = new Set(), items = [];
        for (const link of index.outgoing(opts.path, opts.content || '')) {
            const key = link.kind + '|' + link.value + '|' + link.embed;
            if (!seen.has(key)) { seen.add(key); items.push(link); }
        }
        if (!alive()) return;
        const resolved = items.filter(link => link.path), missing = items.filter(link => !link.path);
        const embedTag = link => link.embed ? '<span class="notes-tag-chip">' + esc(tr('embed')) + '</span>' : '';
        const label = link => window.NotesIndex ? (window.NotesIndex.parseWikiValue(link.value).target || link.value) : link.value;
        container.innerHTML = !items.length ? '<p class="notes-muted">' + esc(tr('no_outgoing')) + '</p>' :
            '<section data-resolved><h3>' + esc(tr('resolved_links')) + ' <span class="notes-count">' + resolved.length + '</span></h3>' +
            resolved.map(link => {
                const isNote = /\.md$/i.test(link.path), note = index.note(link.path);
                return row(esc, (isNote ? 'data-note-path="' : 'data-attachment-path="') + esc(link.path) + '"', isNote ? (note?.title || noteName(link.path)) : basename(link.path), folderOf(link.path), embedTag(link));
            }).join('') + '</section>' +
            (missing.length ? '<section data-unresolved><h3>' + esc(tr('unresolved_links')) + ' <span class="notes-count">' + missing.length + '</span></h3>' +
                missing.map(link => row(esc, 'data-unresolved-link data-create-wiki="' + esc(link.value) + '" title="' + esc(tr('create_note_hint')) + '"', label(link), tr('create_note'), embedTag(link))).join('') + '</section>' : '');
    }

    window.NotesPanels = {backlinks, backlinksSignature, outgoing, contexts, contextHtml};
})();
