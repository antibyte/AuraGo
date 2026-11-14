(function () {
    'use strict';
    // The print pages of a document as one self-contained HTML page for the server's PDF renderer: the engine
    // stylesheet inline, pictures as data: URLs and only the font faces the pages use. The page carries no script
    // and no link; the server renders it under a policy that forbids both.
    const STYLE_URL = '/js/vendor/writer/engine.css';
    let stylePromise = null;

    const bold = weight => weight === 'bold' || Number(weight) >= 600;
    const slanted = style => !!style && style !== 'normal';
    const family = value => String(value || '').split(',')[0].trim().replace(/^["']|["']$/g, '').toLowerCase();
    // key names one face: the family, bold or regular, italic or normal.
    const key = (name, weight, style) => family(name) + '|' + (bold(weight) ? 'b' : 'r') + '|' + (slanted(style) ? 'i' : 'n');
    const requestKey = request => key(request.family, request.weight, request.style);

    // used turns [family list, weight, style] triples of painted text into face keys, each once.
    function used(triples) { return [...new Set(triples.map(([name, weight, style]) => key(name, weight, style)))]; }

    // source finds the font file of a face: the family's own, or the substitute the engine lays the family out with.
    function source(fonts, faceKey) {
        const own = fonts.sources.find(item => requestKey(item.request) === faceKey);
        if (own) return own;
        const swap = fonts.substitutions.find(item => requestKey(item.from) === faceKey);
        return swap ? fonts.sources.find(item => requestKey(item.request) === requestKey(swap.to)) || null : null;
    }

    function base64(bytes) {
        let text = '';
        for (let index = 0; index < bytes.length; index += 32768) text += String.fromCharCode.apply(null, bytes.subarray(index, index + 32768));
        return btoa(text);
    }

    // faces writes one @font-face rule per face that has a file, under the family name the pages ask for.
    function faces(fonts, keys) {
        return keys.map(faceKey => {
            const item = source(fonts, faceKey);
            if (!item) return '';
            const parts = faceKey.split('|'), style = parts.pop(), weight = parts.pop(), name = parts.join('|').replace(/["\\]/g, '');
            return '@font-face{font-family:"' + name + '";font-weight:' + (weight === 'b' ? 700 : 400) + ';font-style:' + (style === 'i' ? 'italic' : 'normal')
                + ';src:url(data:font/ttf;base64,' + base64(item.bytes) + ')}';
        }).join('');
    }

    const escapeText = value => String(value).replace(/[&<>]/g, sign => ({'&': '&amp;', '<': '&lt;', '>': '&gt;'}[sign]));

    // page joins the parts into the document the server renders.
    function page(parts) {
        return '<!doctype html><html><head><meta charset="utf-8"><title>' + escapeText(parts.title || '') + '</title><style>' + parts.engine + '</style><style>'
            + parts.faces + parts.sheets + '</style></head><body>' + parts.body + '</body></html>';
    }

    // fileName proposes the PDF's name from the document's path.
    function fileName(path) { return (String(path || '').split('/').pop().replace(/\.docx$/i, '') || 'document') + '.pdf'; }

    // pending says whether a layout still waits for a picture. The walk stops at the first one.
    function pending(layout) {
        const seen = new Set();
        const walk = (value, depth) => {
            if (!value || typeof value !== 'object' || depth > 16 || ArrayBuffer.isView(value) || seen.has(value)) return false;
            seen.add(value);
            if (value.resource && value.resource.kind === 'pending') return true;
            const items = value instanceof Map ? value.values() : Array.isArray(value) ? value : Object.values(value);
            for (const item of items) if (walk(item, depth + 1)) return true;
            return false;
        };
        return walk(layout, 0);
    }

    // pictures waits until a scratch editor has decoded its pictures, at most limit milliseconds.
    async function pictures(editor, limit = 8000) {
        const until = Date.now() + limit;
        while (pending(editor.surface.layout()) && Date.now() < until) await new Promise(resolve => setTimeout(resolve, 50));
    }

    // graft copies the decoded pictures of the scratch editor (source) into the painted pages. A page painted on
    // its own only gets a placeholder for a picture; the drawing's node id connects the two.
    function graft(source, pages) {
        const ready = new Map();
        for (const drawing of source.querySelectorAll('.docx-drawing-ready[data-drawing-node-id]')) ready.set(drawing.dataset.drawingNodeId, drawing);
        let count = 0;
        for (const hole of pages.querySelectorAll('.docx-drawing-placeholder[data-drawing-node-id]')) {
            const drawing = ready.get(hole.dataset.drawingNodeId);
            if (!drawing) continue;
            const copy = drawing.cloneNode(true);
            for (const side of ['left', 'top', 'width', 'height']) copy.style[side] = hole.style[side];
            hole.replaceWith(copy);
            count++;
        }
        return count;
    }

    function dataURL(blob) {
        return new Promise((resolve, reject) => {
            const reader = new FileReader();
            reader.onload = () => resolve(reader.result);
            reader.onerror = () => reject(reader.error);
            reader.readAsDataURL(blob);
        });
    }

    // inline turns the pictures' blob: addresses into data: ones. A blob: address dies with the editor that made it.
    async function inline(pages) {
        for (const image of pages.querySelectorAll('img')) {
            if (!/^blob:/.test(image.src)) continue;
            image.src = await dataURL(await (await fetch(image.src)).blob());
        }
    }

    // pack reads the painted pages of a print frame (see paint in writer-panels.js) and returns the page.
    async function pack(doc, fonts, title) {
        const pages = doc.getElementById('pages'), view = doc.defaultView, triples = [];
        for (const element of pages.querySelectorAll('*')) {
            if (![...element.childNodes].some(node => node.nodeType === 3 && node.textContent.trim())) continue;
            const style = view.getComputedStyle(element);
            triples.push([style.fontFamily, style.fontWeight, style.fontStyle]);
        }
        stylePromise ||= fetch(STYLE_URL).then(response => {
            if (!response.ok) throw new Error('HTTP ' + response.status);
            return response.text();
        });
        let engine;
        try { engine = await stylePromise; } catch (error) { stylePromise = null; throw error; }
        const sheets = [...doc.head.querySelectorAll('style')].map(style => style.textContent).join('');
        return page({title, engine, faces: faces(fonts, used(triples)), sheets, body: pages.outerHTML});
    }

    window.WriterPdf = {used, source, faces, page, fileName, pending, pictures, graft, inline, pack};
})();
