// Runs only inside the first-party NASSCAD frame; also supports existing workspace copies.
(function installNasscadEmail() {
    'use strict';
    if (window.AuraNasscadEmail) return;
    let pending = null;
    const exporters = {
        project: () => saveProject(), stl: () => expSTL(), 'stl-ascii': () => expSTLascii(),
        '3mf': () => exp3MF(), glb: () => expGLB(false), 'glb-draco': () => expGLB(true), obj: () => expOBJ(), ply: () => expPLY(),
        ifc: () => expIFC(), step: () => exportDialog('step'), svg: () => exportDialog('svg'), dxf: () => exportDialog('dxf')
    };
    function exportDialog(format) {
        const operation = pending, step = format === 'step';
        const modal = document.getElementById(step ? 'step-export-modal' : 'sxdlg');
        if (!modal) throw new Error('export_failed');
        return new Promise((resolve, reject) => {
            const name = step ? 'doStepExport' : '_doSectionExp', native = window[name], nativeStep = window._expSTEPRun;
            let started = false;
            const finish = (value, error) => {
                observer.disconnect();
                operation.signal?.removeEventListener('abort', abort);
                if (window[name] === run) window[name] = native;
                if (step && window._expSTEPRun === runStep) window._expSTEPRun = nativeStep;
                error ? reject(error) : resolve(value);
            };
            const abort = () => { modal.style.display = 'none'; if (!started) finish(false); };
            const observer = new MutationObserver(() => { if (!started && modal.style.display === 'none') finish(false); });
            const runStep = function () { return operation.task = nativeStep.apply(this, arguments); };
            const run = async function () {
                if (started) return;
                started = true;
                try {
                    await native.apply(this, step ? arguments : [format]);
                    if (step) await operation.task;
                    finish(operation.files);
                } catch (error) { finish(null, error); }
            };
            window[name] = run;
            if (step) window._expSTEPRun = runStep;
            observer.observe(modal, {attributes:true,attributeFilter:['style']});
            operation.signal?.addEventListener('abort', abort, {once:true});
            try {
                if (step) openStepExportModal();
                else {
                    doExp('section');
                    // The native dialog keeps its plane setting; only the chosen output is offered.
                    for (const button of modal.querySelectorAll('[onclick*="_doSectionExp"]')) {
                        const visible = button.style.display;
                        button.style.display = button.getAttribute('onclick').includes("'" + format + "'") ? visible : 'none';
                        operation.restore.push(() => { button.style.display = visible; });
                    }
                }
            } catch (error) { finish(null, error); }
        });
    }
    const api = {
        get active() { return !!pending; },
        get ready() { return typeof objs !== 'undefined' && objs.length > 0; },
        get documentKey() { return typeof objs === 'undefined' ? null : objs; },
        formats: Object.keys(exporters).map(id=>({id,label:id==='project'?'NASSCAD':id.toUpperCase()})),
        capture(name, content, type) {
            if (!pending) return false;
            if (!pending.signal?.aborted) pending.files.push(new File([content], name, {type:type || content?.type || 'application/octet-stream'}));
            return true;
        },
        async createFiles(format, signal) {
            if (pending || !exporters[format]) throw new Error('export_failed');
            const operation = {files:[], signal, restore:[]}, download = window._nasDownload, picker = window.showSaveFilePicker;
            const capture = function (name, content, type) { if (api.capture(name,content,type)) return Promise.resolve(); return download.apply(this,arguments); };
            pending = operation;
            try {
                signal?.throwIfAborted();
                window._nasDownload = capture;
                window.showSaveFilePicker = undefined;
                // Older saveProject defers serialization to RAF but does not return its task.
                // Limit the scheduling adapter to this synchronous call; the actual render loop stays native.
                const raf = window.requestAnimationFrame;
                let task;
                try {
                    if (format === 'project') window.requestAnimationFrame = fn => queueMicrotask(fn);
                    task = exporters[format]();
                } finally { window.requestAnimationFrame = raf; }
                const result = await task;
                signal?.throwIfAborted();
                if (result === false) return false;
                return operation.files;
            } finally {
                operation.restore.forEach(restore => restore());
                if (window._nasDownload === capture) window._nasDownload = download;
                if (window.showSaveFilePicker === undefined) window.showSaveFilePicker = picker;
                if (pending === operation) pending = null;
            }
        }
    };
    window.AuraNasscadEmail = api;
})();
