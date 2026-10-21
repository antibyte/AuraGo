// Pixel module "commands": the command palette (Ctrl+K / Ctrl+Shift+P). It lists everything r.listCommands() returns at the
// moment it opens, so commands that other modules register (filters, generators, cutout, ...) show up without any wiring.
// The matcher (Pixel.fuzzyScore / Pixel.searchCommands) is DOM-free and covered by scripts/test-pixel-inpaint.mjs.
(function () {
    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};

    const RECENT_KEY = 'pixel.recent_commands';
    const RECENT_MAX = 20;
    const MAX_RESULTS = 60;
    const MAX_BROWSE = 240;
    const OWN_COMMAND = 'menu:view:command-palette';

    // --- fuzzy matching ---

    // One character in, one character out, so match positions still index the original text.
    function foldChar(c) {
        const lower = c.toLowerCase();
        const base = lower.normalize ? lower.normalize('NFD').charAt(0) : lower;
        return base || lower;
    }

    function fold(text) {
        let out = '';
        const source = String(text == null ? '' : text);
        for (let i = 0; i < source.length; i++) out += foldChar(source.charAt(i));
        return out;
    }

    const letterOrDigit = /[\p{L}\p{N}]/u;

    function isBoundary(text, i) {
        if (i === 0) return true;
        const prev = text.charAt(i - 1), cur = text.charAt(i);
        if (!letterOrDigit.test(prev)) return true;
        return prev === prev.toLowerCase() && prev !== prev.toUpperCase() && cur === cur.toUpperCase() && cur !== cur.toLowerCase();
    }

    // Subsequence match with bonuses for the start of the text, for word starts and for consecutive letters, and a penalty
    // for gaps. Returns { score, positions } or null. Spaces in the query are ignored.
    function fuzzyMatch(query, text) {
        const q = fold(query).replace(/\s+/g, '');
        const original = String(text == null ? '' : text);
        const t = fold(original);
        const n = q.length, m = t.length;
        if (!n) return { score: 0, positions: [] };
        if (n > m) return null;
        let qi = 0;
        for (let j = 0; j < m && qi < n; j++) if (t.charAt(j) === q.charAt(qi)) qi++;
        if (qi < n) return null;
        const NEG = -1e9;
        const boundary = [];
        for (let j = 0; j < m; j++) boundary.push(isBoundary(original, j));
        let prev = new Float64Array(m).fill(NEG);
        const back = [];
        for (let i = 0; i < n; i++) {
            const cur = new Float64Array(m).fill(NEG);
            const from = new Int16Array(m).fill(-1);
            let gapBest = NEG, gapAt = -1;
            for (let j = 0; j < m; j++) {
                if (i > 0 && j >= 2) {
                    const v = prev[j - 2] + (j - 2);
                    if (v > gapBest) { gapBest = v; gapAt = j - 2; }
                }
                if (t.charAt(j) !== q.charAt(i)) continue;
                let gain = 10;
                if (j === 0) gain += 12;
                else if (boundary[j]) gain += 8;
                if (i === 0) {
                    cur[j] = gain - Math.min(6, j * 0.3);
                    continue;
                }
                let best = NEG, source = -1;
                if (j >= 1 && prev[j - 1] > NEG / 2) { best = prev[j - 1] + 6; source = j - 1; }
                if (gapBest > NEG / 2 && gapBest - (j - 1) > best) { best = gapBest - (j - 1); source = gapAt; }
                if (source < 0) continue;
                cur[j] = best + gain;
                from[j] = source;
            }
            back.push(from);
            prev = cur;
        }
        let end = -1, top = NEG;
        for (let j = 0; j < m; j++) if (prev[j] > top) { top = prev[j]; end = j; }
        if (end < 0) return null;
        const positions = new Array(n);
        let at = end;
        for (let i = n - 1; i >= 0; i--) {
            positions[i] = at;
            at = back[i][at];
        }
        let score = top - 0.15 * (m - n);
        if (t.startsWith(q)) score += 15;
        if (t === q) score += 30;
        return { score: Math.max(0.01, score), positions };
    }

    // The score alone: -1 when the query is not a subsequence of the text.
    function fuzzyScore(query, text) {
        const match = fuzzyMatch(query, text);
        return match ? match.score : -1;
    }

    // --- synonyms (English and German) ---
    // Every group lists words that mean the same thing; a query word that is, or starts like, a member also searches
    // for the other members. This works both ways, so English queries find German labels and the other way round.
    const SYNONYM_GROUPS = [
        ['layer', 'layers', 'ebene', 'ebenen'],
        ['blur', 'weichzeichnen', 'weichzeichner', 'unscharf', 'verwischen'],
        ['noise', 'rauschen', 'grain', 'koernung', 'körnung', 'filmkorn'],
        ['sharpen', 'sharpness', 'schärfen', 'scharfzeichnen', 'schärfe', 'scharf'],
        ['remove background', 'background removal', 'cutout', 'cut out', 'freistellen', 'freisteller', 'hintergrund entfernen', 'hintergrund', 'background'],
        ['crop', 'trim', 'zuschneiden', 'beschneiden', 'trimmen'],
        ['resize', 'scale', 'scaling', 'skalieren', 'größe ändern', 'bildgröße', 'size', 'größe'],
        ['canvas size', 'canvas', 'arbeitsfläche', 'leinwand', 'zeichenfläche'],
        ['rotate', 'rotation', 'drehen', 'rotieren', 'drehung'],
        ['flip', 'mirror', 'spiegeln', 'spiegelung'],
        ['select', 'selection', 'auswahl', 'auswählen', 'markieren'],
        ['deselect', 'abwählen', 'auswahl aufheben'],
        ['invert', 'negative', 'invertieren', 'umkehren', 'negativ'],
        ['feather', 'soften edge', 'weiche kante', 'kante weichzeichnen'],
        ['fill', 'füllen', 'füllung', 'ausfüllen', 'bucket', 'farbeimer'],
        ['content aware', 'content-aware', 'remove object', 'inpaint', 'heal', 'healing', 'objekt entfernen', 'inhaltsbasiert', 'inhaltsbasierte füllung', 'retusche', 'reparieren', 'heilen'],
        ['color', 'colour', 'colors', 'farbe', 'farben'],
        ['brightness', 'helligkeit', 'light', 'licht'],
        ['contrast', 'kontrast'],
        ['saturation', 'sättigung', 'vibrance', 'lebendigkeit'],
        ['hue', 'farbton'],
        ['levels', 'tonwert', 'tonwertkorrektur', 'tonwerte'],
        ['curves', 'kurven', 'gradationskurve', 'gradationskurven'],
        ['gradient', 'verlauf', 'farbverlauf'],
        ['history', 'verlauf', 'protokoll', 'historie'],
        ['pattern', 'muster', 'texture', 'textur'],
        ['text', 'type', 'schrift', 'beschriftung'],
        ['brush', 'pinsel', 'paint', 'malen'],
        ['eraser', 'erase', 'radierer', 'radieren'],
        ['delete', 'remove', 'löschen', 'entfernen'],
        ['undo', 'rückgängig', 'zurücknehmen'],
        ['redo', 'wiederholen', 'wiederherstellen'],
        ['save', 'speichern', 'sichern'],
        ['open', 'öffnen', 'laden'],
        ['export', 'exportieren', 'ausgeben'],
        ['new', 'neu', 'neue', 'create', 'erstellen', 'anlegen'],
        ['copy', 'kopieren', 'duplicate', 'duplizieren', 'kopie'],
        ['paste', 'einfügen'],
        ['cut', 'ausschneiden'],
        ['zoom', 'vergrößern', 'verkleinern', 'lupe'],
        ['opacity', 'deckkraft', 'transparency', 'transparenz', 'alpha'],
        ['mask', 'maske', 'masken'],
        ['palette', 'farbpalette', 'swatches', 'farbfelder', 'farbfeld'],
        ['enhance', 'auto enhance', 'verbessern', 'optimieren', 'automatisch', 'auto'],
        ['flatten', 'reduzieren', 'auf eine ebene reduzieren'],
        ['merge', 'zusammenführen', 'vereinen', 'verbinden'],
        ['sticker', 'aufkleber'],
        ['shadow', 'schatten', 'schlagschatten'],
        ['glow', 'leuchten', 'schein'],
        ['outline', 'stroke', 'border', 'kontur', 'umrandung', 'rahmen'],
        ['grayscale', 'greyscale', 'gray', 'grey', 'black and white', 'graustufen', 'schwarzweiß', 'monochrom'],
        ['pixelate', 'mosaic', 'verpixeln', 'mosaik', 'pixeln'],
        ['emboss', 'relief', 'prägen'],
        ['edge', 'edges', 'kante', 'kanten', 'konturen'],
        ['vignette', 'vignettierung', 'randabdunklung'],
        ['shape', 'shapes', 'form', 'formen', 'rectangle', 'rechteck', 'ellipse', 'circle', 'kreis'],
        ['clone', 'stamp', 'klonen', 'kopierstempel', 'stempel'],
        ['smudge', 'wischen', 'verwischen'],
        ['dodge', 'burn', 'abwedeln', 'nachbelichten'],
        ['transform', 'transformieren', 'verzerren'],
        ['move', 'verschieben', 'bewegen'],
        ['generate', 'generator', 'erzeugen', 'generieren'],
        ['filter', 'filters', 'effekt', 'effekte', 'effects']
    ];

    let synonymIndex = null;

    function buildSynonymIndex() {
        const index = [];
        SYNONYM_GROUPS.forEach(group => index.push(group.map(fold)));
        return index;
    }

    // Other words that mean what the query word means (the word itself is not included).
    function synonymsFor(word) {
        if (!synonymIndex) synonymIndex = buildSynonymIndex();
        const w = fold(word).trim();
        if (w.length < 2) return [];
        const found = [];
        synonymIndex.forEach((group, gi) => {
            const hit = group.some(member => member === w || (w.length >= 3 && member.startsWith(w)));
            if (!hit) return;
            SYNONYM_GROUPS[gi].forEach(member => {
                if (fold(member) !== w && !found.includes(member)) found.push(member);
            });
        });
        return found.slice(0, 14);
    }

    // --- searching commands ---

    function tokensOf(query) {
        return fold(query).split(/\s+/).filter(Boolean);
    }

    // Score of one query word against one command: best over the word itself and its synonyms, over label, keywords,
    // group and id (the label counts most). Returns { score, labelPositions } or null.
    function scoreToken(token, fields, label) {
        const alternatives = [{ text: token, weight: 1, strict: false }].concat(synonymsFor(token).map(text => ({ text, weight: 0.6, strict: true })));
        let best = null;
        alternatives.forEach(alt => {
            const length = fold(alt.text).replace(/\s+/g, '').length;
            fields.forEach(field => {
                const match = fuzzyMatch(alt.text, field.text);
                if (!match || match.score < 5 * length) return;
                // A synonym has to appear (nearly) as one piece: loose letter matches of short words are noise.
                if (alt.strict && match.positions[length - 1] - match.positions[0] + 1 > length + 1) return;
                const score = match.score * field.weight * alt.weight;
                if (!best || score > best.score) best = { score, labelPositions: field.isLabel ? match.positions : null };
            });
        });
        if (best && !best.labelPositions) {
            const own = fuzzyMatch(token, label);
            best.labelPositions = own ? own.positions : [];
        }
        return best;
    }

    // commands: [{ id, label, group, keywords: [], ... }]. Returns [{ command, score, positions }] best first.
    // options: { boost: { id: number } } adds to the score (recent commands).
    function searchCommands(commands, query, options) {
        const tokens = tokensOf(query);
        const boost = (options && options.boost) || {};
        if (!tokens.length) return commands.map(command => ({ command, score: 0, positions: [] }));
        const results = [];
        commands.forEach((command, order) => {
            const fields = [{ text: command.label || '', weight: 1, isLabel: true }];
            (command.keywords || []).forEach(word => { if (word) fields.push({ text: String(word), weight: 0.85 }); });
            if (command.group) fields.push({ text: String(command.group), weight: 0.45 });
            fields.push({ text: String(command.id || ''), weight: 0.35 });
            let total = 0;
            const positions = new Set();
            for (let i = 0; i < tokens.length; i++) {
                const hit = scoreToken(tokens[i], fields, command.label || '');
                if (!hit) { total = -1; break; }
                total += hit.score;
                (hit.labelPositions || []).forEach(p => positions.add(p));
            }
            if (total < 0) return;
            results.push({ command, score: total + (boost[command.id] || 0), positions: Array.from(positions).sort((a, b) => a - b), order });
        });
        results.sort((a, b) => b.score - a.score || a.order - b.order);
        return results;
    }

    // --- recent commands ---

    function loadRecent() {
        try {
            const list = JSON.parse(localStorage.getItem(RECENT_KEY) || '[]');
            return Array.isArray(list) ? list.filter(id => typeof id === 'string').slice(0, RECENT_MAX) : [];
        } catch (_) {
            return [];
        }
    }

    function rememberCommand(id) {
        if (!id) return;
        const list = [id].concat(loadRecent().filter(x => x !== id)).slice(0, RECENT_MAX);
        try { localStorage.setItem(RECENT_KEY, JSON.stringify(list)); } catch (_) { /* storage may be blocked */ }
    }

    // --- context-aware suggestions (shown while the query is empty) ---

    // True for a freshly opened photo: one layer, no edits since opening.
    function isFreshPhoto(r) {
        return !!(r.imgWidth >= 160 && r.imgHeight >= 160 && r.layers.length === 1 && r.history.length === 1 && r.history[0].label === 'open');
    }

    // True when the composite has fully transparent rows or columns at its edges (a proxy is enough to tell).
    function hasTransparentEdges(r) {
        if (!r.canvas || !r.canvas.width || !r.canvas.height || typeof r.makeCanvas !== 'function') return false;
        try {
            const scale = Math.min(1, 160 / Math.max(r.canvas.width, r.canvas.height));
            const w = Math.max(1, Math.round(r.canvas.width * scale)), h = Math.max(1, Math.round(r.canvas.height * scale));
            const proxy = r.makeCanvas(w, h);
            const ctx = proxy.getContext('2d');
            ctx.drawImage(r.canvas, 0, 0, w, h);
            const d = ctx.getImageData(0, 0, w, h).data;
            const rowClear = y => { for (let x = 0; x < w; x++) if (d[(y * w + x) * 4 + 3] > 0) return false; return true; };
            const colClear = x => { for (let y = 0; y < h; y++) if (d[(y * w + x) * 4 + 3] > 0) return false; return true; };
            let anyContent = false;
            for (let y = 0; y < h && !anyContent; y++) if (!rowClear(y)) anyContent = true;
            if (!anyContent) return false;
            return rowClear(0) || rowClear(h - 1) || colClear(0) || colClear(w - 1);
        } catch (_) {
            return false;
        }
    }

    const SUGGESTION_RULES = [
        { when: r => !!r.selection, picks: [
            { id: 'menu:edit:content-aware-fill' },
            { re: /feather|weiche kante/i },
            { re: /(copy|via copy|kopier)[^|]*(new layer|neue ebene)|(new layer|neue ebene)[^|]*(copy|selection|auswahl)|layer via/i }
        ] },
        { when: isFreshPhoto, picks: [
            { re: /auto.?(enhance|level|tone|contrast|colou?r|white)|automatisch.*(verbess|tonwert|kontrast|farb)/i },
            { re: /remove.?(bg|background)|background.?remov|cut.?out|hintergrund.*entfern|freistell/i }
        ] },
        { when: hasTransparentEdges, picks: [{ id: 'menu:image:trim' }] }
    ];

    // The commands that make sense for the current document state, in rule order.
    function contextSuggestions(r, commands) {
        const out = [];
        SUGGESTION_RULES.forEach(rule => {
            let applies = false;
            try { applies = !!rule.when(r); } catch (_) { applies = false; }
            if (!applies) return;
            rule.picks.forEach(pick => {
                const found = commands.find(cmd => !out.includes(cmd) && !(cmd.enabled && cmd.enabled(r) === false) &&
                    (pick.id ? cmd.id === pick.id : pick.re.test(cmd.label + ' ' + cmd.id)));
                if (found) out.push(found);
            });
        });
        return out;
    }

    // --- the palette window ---

    const GROUP_LABELS = {
        file: ['desktop.menu_file', 'File'], edit: ['desktop.menu_edit', 'Edit'], image: ['pixel.menu_image', 'Image'],
        layer: ['pixel.menu_layer', 'Layer'], select: ['pixel.menu_select', 'Select'], filter: ['pixel.menu_filter', 'Filter'],
        generate: ['pixel.menu_generate', 'Generate'], view: ['desktop.menu_view', 'View'], ai: ['pixel.ai_generate', 'AI'],
        tools: ['pixel.cmd_group_tools', 'Tools'], panels: ['pixel.cmd_group_panels', 'Panels'], colors: ['pixel.tab_colors', 'Colors']
    };
    const BROWSE_GROUP_ORDER = ['file', 'edit', 'image', 'layer', 'select', 'filter', 'generate', 'view', 'ai', 'tools', 'panels'];

    function groupLabel(r, group) {
        const known = GROUP_LABELS[group];
        if (known) return r.t(known[0], known[1]);
        return group ? String(group).charAt(0).toUpperCase() + String(group).slice(1) : '';
    }

    // Commands as the palette shows them: own entry removed, same label in the same group listed once.
    function paletteCommands(r) {
        const seen = new Set();
        const list = [];
        r.listCommands().forEach(cmd => {
            if (cmd.id === OWN_COMMAND || !cmd.label) return;
            const key = (cmd.group || '') + '|' + cmd.label.toLowerCase();
            if (seen.has(key)) return;
            seen.add(key);
            list.push(Object.assign({}, cmd, { groupId: cmd.group, group: groupLabel(r, cmd.group) }));
        });
        return list;
    }

    function highlight(r, label, positions) {
        if (!positions || !positions.length) return r.esc(label);
        const marked = new Set(positions);
        let html = '', open = false;
        for (let i = 0; i < label.length; i++) {
            const on = marked.has(i);
            if (on && !open) { html += '<mark>'; open = true; }
            if (!on && open) { html += '</mark>'; open = false; }
            html += r.esc(label.charAt(i));
        }
        return html + (open ? '</mark>' : '');
    }

    const SEARCH_ICON = '<svg width="16" height="16" viewBox="0 0 16 16" aria-hidden="true"><circle cx="6.7" cy="6.7" r="4.4" fill="none" stroke="currentColor" stroke-width="1.5"/><path d="M10.2 10.2L14 14" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/></svg>';

    Pixel.registerModule('commands', function install(r) {
        const palette = { el: null, input: null, list: null, items: [], active: -1, previousFocus: null, commands: [] };

        function closePalette() {
            if (!palette.el) return;
            const el = palette.el;
            palette.el = palette.input = palette.list = null;
            palette.items = [];
            palette.active = -1;
            el.remove();
            const back = palette.previousFocus;
            palette.previousFocus = null;
            if (back && typeof back.focus === 'function' && back.isConnected !== false) {
                try { back.focus(); } catch (_) { /* element is gone */ }
            }
        }

        function enabledOf(cmd) {
            try { return !(cmd.enabled && cmd.enabled(r) === false); } catch (_) { return true; }
        }

        // Rows of the list for the current query: headings and selectable commands.
        function buildRows(query) {
            const commands = palette.commands;
            const q = String(query || '').trim();
            if (q) {
                const recent = loadRecent();
                const boost = {};
                recent.forEach((id, i) => { boost[id] = (RECENT_MAX - i) * 0.4; });
                return searchCommands(commands, q, { boost }).slice(0, MAX_RESULTS).map(hit => ({ cmd: hit.command, positions: hit.positions }));
            }
            const rows = [];
            const used = new Set();
            const push = (title, list) => {
                const fresh = list.filter(cmd => !used.has(cmd.id));
                if (!fresh.length) return;
                rows.push({ heading: title });
                fresh.forEach(cmd => { used.add(cmd.id); rows.push({ cmd, positions: [] }); });
            };
            push(r.t('pixel.cmd_suggested', 'Suggested'), contextSuggestions(r, commands));
            const byId = new Map(commands.map(cmd => [cmd.id, cmd]));
            push(r.t('pixel.cmd_recent', 'Recent'), loadRecent().map(id => byId.get(id)).filter(Boolean));
            const rest = commands.filter(cmd => !used.has(cmd.id)).sort((a, b) => {
                const ga = BROWSE_GROUP_ORDER.indexOf(a.groupId), gb = BROWSE_GROUP_ORDER.indexOf(b.groupId);
                return (ga < 0 ? 99 : ga) - (gb < 0 ? 99 : gb) || a.label.localeCompare(b.label);
            }).slice(0, MAX_BROWSE);
            push(r.t('pixel.cmd_all', 'All commands'), rest);
            return rows;
        }

        function rowHTML(row, index) {
            if (row.heading) return `<div class="pixel-palette-heading" role="presentation">${r.esc(row.heading)}</div>`;
            const cmd = row.cmd, ok = enabledOf(cmd);
            const shortcut = cmd.shortcut ? `<kbd class="pixel-kbd pixel-palette-kbd">${r.esc(cmd.shortcut)}</kbd>` : '';
            return `<div class="pixel-palette-item${ok ? '' : ' is-disabled'}" role="option" id="pixel-palette-opt-${index}" data-pal-index="${index}" aria-selected="false"${ok ? '' : ' aria-disabled="true"'}>
                <span class="pixel-palette-label">${highlight(r, cmd.label, row.positions)}</span>
                <span class="pixel-palette-group">${r.esc(cmd.group || '')}</span>${shortcut}</div>`;
        }

        function setActive(index, scroll) {
            const rows = palette.items;
            if (!rows.length) { palette.active = -1; return; }
            let next = Math.max(0, Math.min(rows.length - 1, index));
            while (next < rows.length && rows[next].heading) next++;
            if (next >= rows.length) return;
            palette.active = next;
            if (!palette.list) return;
            palette.list.querySelectorAll('.pixel-palette-item').forEach(el => {
                const on = Number(el.dataset.palIndex) === next;
                el.classList.toggle('is-active', on);
                el.setAttribute('aria-selected', on ? 'true' : 'false');
                if (on && scroll && el.scrollIntoView) el.scrollIntoView({ block: 'nearest' });
            });
            if (palette.input) palette.input.setAttribute('aria-activedescendant', 'pixel-palette-opt-' + next);
        }

        function firstSelectable() {
            return palette.items.findIndex(row => !row.heading);
        }

        function move(delta) {
            const rows = palette.items;
            if (!rows.length) return;
            let i = palette.active;
            for (let step = 0; step < rows.length; step++) {
                i = (i + delta + rows.length) % rows.length;
                if (!rows[i].heading) break;
            }
            setActive(i, true);
        }

        function renderRows() {
            if (!palette.list) return;
            palette.items = buildRows(palette.input ? palette.input.value : '');
            if (!palette.items.length) {
                palette.list.innerHTML = `<div class="pixel-palette-empty">${r.esc(r.t('pixel.cmd_no_results', 'No matching commands'))}</div>`;
                palette.active = -1;
                return;
            }
            palette.list.innerHTML = palette.items.map(rowHTML).join('');
            setActive(firstSelectable(), false);
            if (palette.list) palette.list.scrollTop = 0;
        }

        function runActive() {
            const row = palette.items[palette.active];
            if (!row || !row.cmd) return;
            const cmd = row.cmd;
            if (!enabledOf(cmd)) {
                if (palette.list) {
                    const el = palette.list.querySelector('[data-pal-index="' + palette.active + '"]');
                    if (el) { el.classList.add('is-denied'); setTimeout(() => el.classList.remove('is-denied'), 300); }
                }
                return;
            }
            rememberCommand(cmd.id);
            closePalette();
            // After the palette has given the focus back: dialogs a command opens must not lose theirs again.
            Promise.resolve().then(() => {
                try { cmd.run(r); } catch (err) { if (window.console) console.error('pixel command ' + cmd.id, err); }
            });
        }

        function openPalette() {
            if (palette.el) { closePalette(); return; }
            const root = r.appEl || r.host;
            if (!root) return;
            palette.commands = paletteCommands(r);
            palette.previousFocus = document.activeElement || null;
            const el = document.createElement('div');
            el.className = 'pixel-palette-backdrop';
            el.innerHTML = `<div class="pixel-palette" role="dialog" aria-modal="true" aria-label="${r.esc(r.t('pixel.cmd_palette', 'Command palette'))}">
                <div class="pixel-palette-head">${SEARCH_ICON}<input class="pixel-palette-input" type="text" role="combobox" aria-expanded="true" aria-autocomplete="list" aria-controls="pixel-palette-list" autocomplete="off" spellcheck="false" data-pal-input placeholder="${r.esc(r.t('pixel.cmd_placeholder', 'Type a command, tool or filter'))}"></div>
                <div class="pixel-palette-list" id="pixel-palette-list" role="listbox" data-pal-list></div>
                <div class="pixel-palette-foot"><span><kbd class="pixel-kbd">↑</kbd><kbd class="pixel-kbd">↓</kbd> ${r.esc(r.t('pixel.cmd_hint_navigate', 'navigate'))}</span><span><kbd class="pixel-kbd">Enter</kbd> ${r.esc(r.t('pixel.cmd_hint_run', 'run'))}</span><span><kbd class="pixel-kbd">Esc</kbd> ${r.esc(r.t('pixel.cmd_hint_close', 'close'))}</span></div>
            </div>`;
            root.appendChild(el);
            palette.el = el;
            palette.input = el.querySelector('[data-pal-input]');
            palette.list = el.querySelector('[data-pal-list]');
            el.addEventListener('mousedown', e => { if (e.target === el) { e.preventDefault(); closePalette(); } });
            palette.list.addEventListener('mousemove', e => {
                const item = e.target.closest ? e.target.closest('[data-pal-index]') : null;
                if (item && Number(item.dataset.palIndex) !== palette.active) setActive(Number(item.dataset.palIndex), false);
            });
            palette.list.addEventListener('click', e => {
                const item = e.target.closest ? e.target.closest('[data-pal-index]') : null;
                if (!item) return;
                setActive(Number(item.dataset.palIndex), false);
                runActive();
            });
            palette.input.addEventListener('input', renderRows);
            palette.input.addEventListener('keydown', e => {
                const key = e.key;
                const toggle = (e.ctrlKey || e.metaKey) && (key.toLowerCase() === 'k' || (e.shiftKey && key.toLowerCase() === 'p'));
                if (key === 'Escape' || toggle) { e.preventDefault(); e.stopPropagation(); closePalette(); return; }
                if (key === 'ArrowDown') { e.preventDefault(); move(1); } else if (key === 'ArrowUp') { e.preventDefault(); move(-1); }
                else if (key === 'PageDown') { e.preventDefault(); move(8); } else if (key === 'PageUp') { e.preventDefault(); move(-8); }
                else if (key === 'Home' && e.ctrlKey) { e.preventDefault(); setActive(firstSelectable(), true); }
                else if (key === 'Enter') { e.preventDefault(); runActive(); }
                else if (key === 'Tab') { e.preventDefault(); move(e.shiftKey ? -1 : 1); }
                // Keys that belong to the field must not reach the window's shortcut handler.
                if (key !== 'Escape') e.stopPropagation();
            });
            renderRows();
            palette.input.focus();
        }

        Object.assign(r, {
            openCommandPalette: Pixel.bindRuntime(r, function openCommandPalette() { if (!palette.el) openPalette(); }),
            closeCommandPalette: Pixel.bindRuntime(r, function closeCommandPalette() { closePalette(); }),
            toggleCommandPalette: Pixel.bindRuntime(r, function toggleCommandPalette() { openPalette(); }),
            isCommandPaletteOpen: Pixel.bindRuntime(r, function isCommandPaletteOpen() { return !!palette.el; })
        });

        // A search button at the right end of the toolbar (the markup exists once the shell is built).
        r.on('ready', () => {
            const bar = r.host.querySelector('.pixel-toolbar');
            if (!bar || typeof document.createElement !== 'function') return;
            const spacer = document.createElement('span');
            spacer.className = 'pixel-toolbar-spacer';
            const button = document.createElement('button');
            button.type = 'button';
            button.className = 'pixel-btn-icon pixel-palette-btn';
            button.title = r.t('pixel.cmd_palette_title', 'Command palette (Ctrl+K)');
            button.setAttribute('aria-label', r.t('pixel.cmd_palette', 'Command palette'));
            button.innerHTML = SEARCH_ICON;
            button.addEventListener('click', () => r.toggleCommandPalette());
            bar.appendChild(spacer);
            bar.appendChild(button);
        });
    });

    Pixel.registerMenuItems('view', [
        { id: 'command-palette', labelKey: 'pixel.cmd_palette_item', fallback: 'Command palette…', icon: 'search', shortcut: 'Ctrl+K', shortcuts: ['Ctrl+Shift+P'], action: r => r.toggleCommandPalette() }
    ], { order: 60 });

    Object.assign(Pixel, { fuzzyMatch, fuzzyScore, searchCommands, synonymsFor, contextSuggestions });
})();
