(function () {
    'use strict';
    const instances = new Map();
    const base = '/api/desktop/ocr/';
    function dispose(id) { const cleanup = instances.get(id); if (cleanup) cleanup(); instances.delete(id); }
    function render(host, id, ctx) {
        dispose(id);
        const tr = key => ctx.t('ocr.' + key);
        const esc = ctx.esc;
        const lifetime = new AbortController();
        let dead = false, timer = null, epoch = 0, busy = false, jobs = [], uploads = [], paths = [];
        let selected = '', fileID = '', result = null, pageIndex = 0, pdf = null, pdfTask = null, renderTask = null, resultKey = '';
        host.innerHTML = '<section class="ocr-app"><header class="ocr-toolbar"><h2>OCR</h2><span class="ocr-status"></span>' +
            '<a href="/config#ocr" target="_blank" rel="noopener">' + esc(tr('settings')) + '</a></header>' +
            '<p class="ocr-notice">' + esc(tr('description')) + '</p><div class="ocr-toolbar">' +
            '<button data-action="upload">' + esc(tr('upload')) + '</button><button data-action="pick">' + esc(tr('pick')) + '</button>' +
            '<label>' + esc(tr('mode')) + ' <select class="ocr-mode"><option value="auto">' + esc(tr('auto')) + '</option><option value="ocr">' + esc(tr('force')) + '</option></select></label>' +
            '<button data-action="start" disabled>' + esc(tr('start')) + '</button><button data-action="clear" disabled>' + esc(tr('clear')) + '</button></div>' +
            '<input class="ocr-input" type="file" multiple hidden accept=".pdf,.png,.jpg,.jpeg,.tif,.tiff,.webp,.bmp"><ul class="ocr-queued"></ul>' +
            '<div class="ocr-live" role="status" aria-live="polite"></div><div class="ocr-layout"><aside class="ocr-jobs" aria-label="' + esc(tr('jobs')) + '"></aside>' +
            '<main class="ocr-workspace"><p>' + esc(tr('empty')) + '</p></main></div><p class="ocr-notice">' + esc(tr('retention')) + '</p>' +
            '<p class="ocr-notice">Indra OCR · Datalab · <a href="https://huggingface.co/datalab-to/indra-ocr/blob/bd02167fdf2005a19962cc15188f8496a2c87d3b/LICENSE" target="_blank" rel="noopener">' + esc(tr('license')) + '</a></p></section>';
        const root = host.querySelector('.ocr-app'), live = root.querySelector('.ocr-live'), workspace = root.querySelector('.ocr-workspace');
        const node = (tag, text, parent) => { const el = document.createElement(tag); if (text !== undefined) el.textContent = text; if (parent) parent.append(el); return el; };
        const request = (path, options) => ctx.api(base + path, Object.assign({signal: lifetime.signal}, options || {}));
        const post = (path, data) => request(path, {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify(data)});
        const stateText = state => tr('state_' + state);
        const fileError = code => code === 'ocr_timeout' ? tr('timeout') : code === 'ocr_limit_exceeded' ? tr('limits') : code === 'ocr_cancelled' ? tr('state_cancelled') : tr('file_failed');
        const report = err => { if (!dead && err?.name !== 'AbortError') live.textContent = tr('error'); };
        function queueView() {
            const list = root.querySelector('.ocr-queued'); list.replaceChildren();
            uploads.forEach(item => node('li', item.name, list)); paths.forEach(path => node('li', path, list));
            root.querySelector('[data-action=start]').disabled = busy || ctx.readonly || !uploads.length && !paths.length;
            root.querySelector('[data-action=clear]').disabled = busy || !uploads.length && !paths.length;
            root.querySelector('[data-action=upload]').disabled = busy || ctx.readonly;
            root.querySelector('[data-action=pick]').disabled = busy || ctx.readonly;
        }
        async function uploadFiles(files) {
            if (busy || ctx.readonly) return;
            if (files.length + uploads.length + paths.length > 20 || files.some(file => file.size > 32 * 1024 * 1024)) { live.textContent = tr('limits'); return; }
            busy = true; queueView(); live.textContent = tr('uploading');
            try {
                for (const file of files) {
                    const form = new FormData(); form.append('file', file);
                    const saved = await request('uploads', {method: 'POST', body: form});
                    if (dead) return;
                    uploads.push({id: saved.id, name: saved.name});
                }
                live.textContent = tr('queued');
            } catch (err) { report(err); }
            finally { if (!dead) { busy = false; queueView(); } }
        }
        async function closePDF() {
            renderTask?.cancel(); renderTask = null;
            pdf = null;
            if (pdfTask) { const old = pdfTask; pdfTask = null; await old.destroy(); }
        }
        async function showPage() {
            const token = ++epoch, page = result?.pages?.[pageIndex];
            if (!page) return;
            const preview = workspace.querySelector('.ocr-preview'), output = workspace.querySelector('.ocr-output');
            if (!preview || !output) return;
            preview.replaceChildren(); output.replaceChildren();
            const format = workspace.querySelector('.ocr-format').value;
            if (format === 'md' && window.marked && window.DOMPurify) {
                output.innerHTML = window.DOMPurify.sanitize(window.marked.parse(page.markdown || ''), {USE_PROFILES: {html: true}, FORBID_TAGS: ['img','iframe','style','script','video','audio','source','track','object','embed','link','meta','form','input','button','textarea','select'], FORBID_ATTR: ['style','background','src','srcset','poster','action','formaction','ping']});
                output.querySelectorAll('a').forEach(link => { link.rel = 'noopener noreferrer'; link.target = '_blank'; });
            } else node('pre', format === 'json' ? JSON.stringify(page, null, 2) : page.text || '', output);
            if (page.note) node('p', tr('page_warning') + ': ' + page.note, output);
            workspace.querySelector('.ocr-page-label').textContent = (pageIndex + 1) + ' / ' + result.pages.length;
            workspace.querySelector('[data-action=prev]').disabled = pageIndex === 0;
            workspace.querySelector('[data-action=next]').disabled = pageIndex + 1 >= result.pages.length;
            if (page.preview && /^[A-Za-z0-9+/=]+$/.test(page.preview)) {
                const image = node('img', undefined, preview); image.alt = tr('preview'); image.src = 'data:image/jpeg;base64,' + page.preview;
            } else if (result.mime_type === 'application/pdf' && window.pdfjsLib) {
                try {
                    if (!pdf) {
                        pdfTask ||= window.pdfjsLib.getDocument({url: base + 'jobs/' + encodeURIComponent(selected) + '/source?file_id=' + encodeURIComponent(fileID), withCredentials: true, isEvalSupported: false, enableXfa: false});
                        const loaded = await pdfTask.promise;
                        if (dead || token !== epoch) return;
                        pdf = loaded;
                    }
                    const sourcePage = await pdf.getPage(pageIndex + 1);
                    if (dead || token !== epoch) return;
                    const viewport = sourcePage.getViewport({scale: Math.min(1.5, 700 / sourcePage.getViewport({scale: 1}).width)});
                    const canvas = node('canvas', undefined, preview); canvas.width = viewport.width; canvas.height = viewport.height; canvas.setAttribute('aria-label', tr('preview'));
                    renderTask?.cancel(); renderTask = sourcePage.render({canvasContext: canvas.getContext('2d'), viewport});
                    await renderTask.promise;
                } catch (err) { if (!dead && token === epoch && err?.name !== 'RenderingCancelledException') preview.textContent = tr('preview_error'); }
            }
        }
        async function selectFile(file) {
            fileID = file.id; const token = ++epoch; resultKey = selected + '/' + file.id + '/' + file.state + '/' + file.page_count;
            result = null; pageIndex = 0; await closePDF();
            if (dead || token !== epoch) return;
            const detail = workspace.querySelector('.ocr-detail'); detail.replaceChildren();
            if (!['completed','partial','error','cancelled'].includes(file.state)) { node('p', stateText(file.state), detail); return; }
            try {
                const loaded = await request('jobs/' + encodeURIComponent(selected) + '/result?file_id=' + encodeURIComponent(fileID));
                if (dead || token !== epoch) return;
                result = loaded;
                if (!result.pages?.length) { node('p', result.error ? fileError(result.error) : tr('no_text'), detail); return; }
                detail.innerHTML = '<div class="ocr-toolbar"><button data-action="prev" aria-label="' + esc(tr('previous')) + '">‹</button><span class="ocr-page-label"></span>' +
                    '<button data-action="next" aria-label="' + esc(tr('next')) + '">›</button><select class="ocr-format" aria-label="' + esc(tr('format')) + '"><option value="text">' + esc(tr('text')) + '</option><option value="md">Markdown</option><option value="json">JSON</option></select>' +
                    '<button data-action="copy">' + esc(tr('copy')) + '</button><button data-action="export">' + esc(tr('export')) + '</button><button data-action="save"' + (ctx.readonly ? ' disabled' : '') + '>' + esc(tr('save')) + '</button></div>' +
                    '<div class="ocr-panels"><div class="ocr-preview"></div><div class="ocr-output"></div></div>';
                if (file.state !== 'completed') node('p', tr('partial_notice'), detail);
                if (result.error) node('p', fileError(result.error), detail);
                await showPage();
            } catch (err) { report(err); }
        }
        function jobView() {
            const list = root.querySelector('.ocr-jobs'); list.replaceChildren();
            if (!jobs.length) node('p', tr('empty'), list);
            jobs.forEach(job => {
                const button = node('button', job.files.map(file => file.name).join(', ') + ' · ' + stateText(job.state), list);
                button.dataset.job = job.id; button.setAttribute('aria-pressed', String(job.id === selected));
            });
            const job = jobs.find(item => item.id === selected); if (!job) return;
            let files = workspace.querySelector('.ocr-files');
            if (!files) {
                workspace.innerHTML = '<div class="ocr-toolbar"><span class="ocr-job-state"></span><button data-action="cancel">' + esc(tr('cancel')) + '</button><button data-action="zip">' + esc(tr('export_all')) + '</button></div><div class="ocr-files"></div><div class="ocr-detail"></div>';
                files = workspace.querySelector('.ocr-files');
            }
            workspace.querySelector('.ocr-job-state').textContent = stateText(job.state);
            workspace.querySelector('[data-action=cancel]').disabled = !!job.finished_at;
            workspace.querySelector('[data-action=zip]').disabled = !job.finished_at;
            files.replaceChildren();
            job.files.forEach(file => { const button = node('button', file.name + ' · ' + stateText(file.state) + ' · ' + file.page_count, files); button.dataset.file = file.id; button.setAttribute('aria-pressed', String(file.id === fileID)); });
            const file = job.files.find(item => item.id === fileID) || job.files[0];
            if (file && resultKey !== selected + '/' + file.id + '/' + file.state + '/' + file.page_count) selectFile(file).catch(report);
        }
        async function refresh() {
            try {
                const status = await request('status');
                const loaded = status.state === 'disabled' ? [] : await request('jobs');
                if (dead) return;
                root.querySelector('.ocr-status').textContent = stateText(status.state);
                jobs = loaded; if (!selected && jobs.length) selected = jobs[0].id;
                jobView();
            } catch (err) { report(err); }
        }
        root.addEventListener('click', async event => {
            const button = event.target.closest('button'); if (!button || busy || dead) return;
            try {
                if (button.dataset.job) { selected = button.dataset.job; fileID = ''; resultKey = ''; result = null; workspace.replaceChildren(); ++epoch; await closePDF(); jobView(); return; }
                if (button.dataset.file) { const job = jobs.find(item => item.id === selected); await selectFile(job.files.find(file => file.id === button.dataset.file)); return; }
                switch (button.dataset.action) {
                case 'upload': root.querySelector('.ocr-input').click(); break;
                case 'pick': {
                    const picked = await ctx.openFileDialog({multiple: true, signal: lifetime.signal, filters: [{name: 'OCR', extensions: ['pdf','png','jpg','jpeg','tiff','tif','webp','bmp']}]});
                    if (!dead && picked && !picked.canceled) { const incoming = picked.paths || picked.files?.map(file => file.path) || [picked.path].filter(Boolean); if (incoming.length + uploads.length + paths.length > 20) { live.textContent = tr('limits'); break; } paths = Array.from(new Set(paths.concat(incoming))); queueView(); } break;
                }
                case 'clear': uploads = []; paths = []; queueView(); break;
                case 'start': {
                    busy = true; queueView();
                    const job = await post('jobs', {uploads: uploads.map(file => file.id), paths, mode: root.querySelector('.ocr-mode').value});
                    if (dead) return; uploads = []; paths = []; selected = job.id; resultKey = ''; workspace.replaceChildren(); live.textContent = tr('background'); await refresh(); break;
                }
                case 'cancel': await post('jobs/' + encodeURIComponent(selected) + '/cancel', {}); await refresh(); break;
                case 'prev': --pageIndex; await showPage(); break;
                case 'next': ++pageIndex; await showPage(); break;
                case 'copy': await navigator.clipboard.writeText(result.pages[pageIndex].text || ''); live.textContent = tr('copied'); break;
                case 'zip': ctx.exportDesktopFile({url: base + 'jobs/' + encodeURIComponent(selected) + '/export?format=zip', filename: 'ocr-results.zip'}); break;
                case 'export': {
                    const format = workspace.querySelector('.ocr-format').value;
                    ctx.exportDesktopFile({url: base + 'jobs/' + encodeURIComponent(selected) + '/export?file_id=' + encodeURIComponent(fileID) + '&format=' + format, filename: 'ocr-result.' + (format === 'text' ? 'txt' : format)}); break;
                }
                case 'save': {
                    const format = workspace.querySelector('.ocr-format').value;
                    const response = await fetch(base + 'jobs/' + encodeURIComponent(selected) + '/export?file_id=' + encodeURIComponent(fileID) + '&format=' + format, {signal: lifetime.signal});
                    if (!response.ok) throw new Error('export_failed');
                    const content = await response.text();
                    if (dead) return;
                    await ctx.saveFileDialog({defaultName: 'ocr-result.' + (format === 'text' ? 'txt' : format), content, signal: lifetime.signal}); break;
                }
                }
            } catch (err) { report(err); }
            finally { if (!dead) { busy = false; queueView(); } }
        }, {signal: lifetime.signal});
        root.querySelector('.ocr-input').addEventListener('change', event => { uploadFiles(Array.from(event.target.files)); event.target.value = ''; }, {signal: lifetime.signal});
        root.addEventListener('change', event => { if (event.target.classList.contains('ocr-format')) showPage().catch(report); }, {signal: lifetime.signal});
        root.addEventListener('dragover', event => { event.preventDefault(); root.classList.add('ocr-drag'); }, {signal: lifetime.signal});
        root.addEventListener('dragleave', () => root.classList.remove('ocr-drag'), {signal: lifetime.signal});
        root.addEventListener('drop', event => { event.preventDefault(); root.classList.remove('ocr-drag'); uploadFiles(Array.from(event.dataTransfer.files)); }, {signal: lifetime.signal});
        timer = setInterval(() => { if (!dead && !busy && !document.hidden && (!host.checkVisibility || host.checkVisibility())) refresh(); }, 2000);
        instances.set(id, () => { dead = true; ++epoch; clearInterval(timer); lifetime.abort(); closePDF().catch(() => {}); uploads = []; paths = []; result = null; });
        queueView(); refresh();
    }
    window.OCRApp = {render, dispose};
})();
