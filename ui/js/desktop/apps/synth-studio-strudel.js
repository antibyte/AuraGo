// SPDX-License-Identifier: AGPL-3.0-or-later
(function () {
    'use strict';
    const KEY = 'aurago:synth-studio:strudel:1', LIMIT = 65536;
    const fits = value => typeof value === 'string' && new TextEncoder().encode(value).byteLength <= LIMIT;
    const examples = {
        melody: 'setcpm(120/4)\nnote("<c3 a2 f3 g2> [e4 g4]*2")\n  .s("triangle").decay(.2).sustain(.2).gain(.4)',
        groove: 'setcpm(120/4)\nstack(\n  s("bd*4, [~ sd]*2, hh*8").gain(.3),\n  note("<c2 c2 ab1 bb1>*4").s("sawtooth")\n    .lpf(600).decay(.12).sustain(0).gain(.25)\n)',
        ambient: 'setcpm(72/4)\nnote("<c3,e3,g3> <a2,c3,e3> <f2,a2,c3> <g2,b2,d3>")\n  .s("sine").slow(2).attack(.4).release(1).gain(.25)'
    };
    function create(host, options) {
        const { tr, signal } = options;
        let frame = null, request = 0, timer = 0, loading = false, running = false, disposed = false;
        const el = (tag, text) => { const node = document.createElement(tag); if (text) node.textContent = text; return node; };
        host.className = 'ss-strudel-workbench';
        const tools = el('div'); tools.className = 'ss-strudel-tools';
        const run = el('button', tr('strudel_run')), stopButton = el('button', tr('stop')), select = el('select');
        run.type = stopButton.type = 'button'; run.dataset.strudelRun = ''; stopButton.dataset.strudelStop = '';
        select.setAttribute('aria-label', tr('strudel_examples'));
        select.append(new Option(tr('strudel_examples'), ''));
        for (const key of Object.keys(examples)) select.append(new Option(tr('strudel_' + key), key));
        const importButton = el('button', tr('strudel_open')), download = el('button', tr('strudel_download'));
        importButton.type = download.type = 'button';
        const file = el('input'); file.type = 'file'; file.accept = '.strudel,.txt,text/plain'; file.hidden = true;
        const guide = el('a', tr('strudel_guide')); guide.href = 'https://strudel.cc/workshop/getting-started/'; guide.target = '_blank'; guide.rel = 'noopener noreferrer';
        const source = el('a', tr('strudel_source')); source.href = '/js/vendor/strudel/sources.zip'; source.download = 'strudel-1.3.0-sources.zip';
        tools.append(run, stopButton, select, importButton, download, guide, source);
        const code = el('textarea'); code.dataset.strudelCode = ''; code.spellcheck = false; code.maxLength = LIMIT; code.setAttribute('aria-label', tr('strudel_code'));
        code.value = examples.melody;
        try { const saved = localStorage.getItem(KEY); if (fits(saved)) code.value = saved; } catch (_) {}
        const hint = el('p', tr('strudel_hint')), status = el('output', tr('strudel_idle'));
        status.dataset.strudelState = 'idle'; status.setAttribute('role', 'status');
        host.append(tools, hint, status, code, file);
        const message = (key, detail = '') => { status.dataset.strudelState = key; status.textContent = tr('strudel_' + key) + (detail ? ': ' + detail : ''); };
        const persist = () => { if (!fits(code.value)) { message('error'); return; } try { localStorage.setItem(KEY, code.value); } catch (_) { message('draft_error'); } };
        function refresh() { run.disabled = disposed || loading || options.blocked(); code.readOnly = options.readonly(); select.disabled = importButton.disabled = options.readonly(); }
        function stop() {
            request++; clearTimeout(timer); frame?.remove(); frame = null; loading = running = false;
            if (!disposed) { message('idle'); refresh(); }
        }
        function evaluate() {
            if (disposed || options.blocked() || loading || !code.value.trim()) return;
            if (!fits(code.value)) { message('error'); return; }
            options.beforePlay(); persist(); loading = true; message('loading'); refresh();
            const current = ++request;
            const send = () => frame?.contentWindow?.postMessage({ strudel: 1, type: 'evaluate', request: current, code: code.value, soundLabel: tr('strudel_sound') }, '*');
            clearTimeout(timer);
            timer = setTimeout(() => { stop(); message('error'); }, 15000);
            if (frame) { send(); return; }
            frame = el('iframe'); frame.title = tr('strudel_sound'); frame.sandbox = 'allow-scripts'; frame.allow = 'autoplay'; frame.referrerPolicy = 'no-referrer'; frame.hidden = true;
            frame.src = '/api/desktop/synth-studio/strudel';
            host.insertBefore(frame, code);
            // Ready comes from the initialized engine, rather than the document load event.
            frame._send = send;
        }
        window.addEventListener('message', event => {
            const data = event.data;
            if (!frame || event.source !== frame.contentWindow || event.origin !== 'null' || data?.strudel !== 1) return;
            if (data.type === 'ready') { const send = frame._send; frame._send = null; if (loading) send?.(); return; }
            if (data.request !== request || !['playing', 'sound', 'error'].includes(data.type)) return;
            clearTimeout(timer); loading = false;
            if (data.type === 'error') { const detail = typeof data.message === 'string' ? data.message.slice(0, 500) : ''; stop(); message('error', detail); }
            else { running = true; frame.hidden = data.type !== 'sound'; message(data.type); refresh(); }
        }, { signal });
        run.addEventListener('click', evaluate, { signal });
        stopButton.addEventListener('click', stop, { signal });
        code.addEventListener('input', persist, { signal });
        code.addEventListener('keydown', event => { if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') { event.preventDefault(); evaluate(); } }, { signal });
        select.addEventListener('change', async () => {
            const value = examples[select.value], initial = code.value; select.value = '';
            if (!value || options.readonly() || code.value !== value && !await options.confirm()) return;
            if (disposed || options.readonly() || code.value !== initial) return;
            code.value = value; persist();
        }, { signal });
        importButton.addEventListener('click', () => file.click(), { signal });
        file.addEventListener('change', async () => {
            const selected = file.files[0]; file.value = '';
            if (!selected || options.readonly()) return;
            if (selected.size > LIMIT) { message('error'); return; }
            const generation = request, initial = code.value;
            try { const value = await selected.text(); if (disposed || options.readonly() || generation !== request || !await options.confirm()) return; if (disposed || options.readonly() || generation !== request || code.value !== initial) return; stop(); code.value = value; persist(); }
            catch (_) { if (!disposed) message('error'); }
        }, { signal });
        download.addEventListener('click', () => {
            const url = URL.createObjectURL(new Blob([code.value], { type: 'text/plain;charset=utf-8' })), link = el('a');
            link.href = url; link.download = 'Synth Studio.strudel'; link.click(); setTimeout(() => URL.revokeObjectURL(url), 1000);
        }, { signal });
        signal.addEventListener('abort', () => { disposed = true; stop(); }, { once: true });
        refresh();
        return { stop, refresh, running: () => running };
    }
    window.SynthStudioStrudel = { create, examples };
})();
