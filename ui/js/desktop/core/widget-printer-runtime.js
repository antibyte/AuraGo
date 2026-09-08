    function printerWidgetData(data) {
        const number = value => typeof value === 'number' && Number.isFinite(value) && value >= 0 ? value : null;
        const status = data.Status || (data.Data && data.Data.Status);
        if (status) {
            const info = status.PrintInfo || {};
            const code = number(info.Status);
            const state = ({ 0: 'ready', 6: 'paused', 8: 'stopped', 9: 'finished', 13: 'printing', 14: 'error' })[code]
                || (code === null ? 'unknown' : 'busy');
            const current = number(info.CurrentTicks), total = number(info.TotalTicks);
            return { state, filename: info.Filename || '', progress: number(info.Progress),
                remaining: ['printing', 'paused', 'busy'].includes(state) && total > 0 && current !== null ? Math.max(0, total - current) : null,
                layer: number(info.CurrentLayer), layers: number(info.TotalLayer),
                nozzle: number(status.TempOfNozzle), bed: number(status.TempOfHotbed) };
        }
        const objects = data.result && data.result.status;
        if (!objects) throw new Error('missing_status');
        const info = objects.print_stats || {};
        const progress = number(objects.virtual_sdcard && objects.virtual_sdcard.progress);
        return { state: ({ standby: 'ready', printing: 'printing', paused: 'paused', complete: 'finished', cancelled: 'stopped', error: 'error' })[info.state] || 'unknown',
            filename: info.filename || '', progress: progress === null ? null : progress * 100, remaining: null,
            layer: number(info.info && info.info.current_layer), layers: number(info.info && info.info.total_layer),
            nozzle: number(objects.extruder && objects.extruder.temperature), bed: number(objects.heater_bed && objects.heater_bed.temperature) };
    }

    function renderPrinterWidget(container) {
        const label = key => t('desktop.widget_printer_' + key);
        container.innerHTML = `<div class="vd-printer">
            <select data-printer="select" aria-label="${esc(label('title'))}"></select>
            <div class="vd-printer-camera" data-printer="camera"><img data-printer="image" alt="${esc(label('camera'))}" hidden><span data-printer="camera-error">${esc(label('camera'))}</span></div>
            <div class="vd-printer-line"><span data-printer="state" role="status"></span><span data-printer="percent"></span></div>
            <progress data-printer="progress" max="100" value="0" aria-label="${esc(label('progress'))}" hidden></progress>
            <div class="vd-printer-file" data-printer="file"></div>
            <dl><div><dt>${esc(label('remaining'))}</dt><dd data-printer="remaining">—</dd></div>
                <div><dt>${esc(label('layers'))}</dt><dd data-printer="layers">—</dd></div>
                <div><dt>${esc(label('nozzle'))}</dt><dd data-printer="nozzle">—</dd></div>
                <div><dt>${esc(label('bed'))}</dt><dd data-printer="bed">—</dd></div></dl>
            <div class="vd-printer-actions"><button type="button" data-printer="expand" disabled>${esc(label('enlarge'))}</button><button type="button" data-printer="refresh">${esc(label('refresh'))}</button></div>
            <dialog class="vd-printer-dialog" aria-label="${esc(label('camera'))}"><button type="button" data-printer="close">${esc(t('desktop.close'))}</button><div data-printer="large"></div></dialog>
        </div>`;
        const refs = Object.fromEntries([...container.querySelectorAll('[data-printer]')].map(el => [el.dataset.printer, el]));
        const dialog = container.querySelector('dialog');
        let disposed = false, controller = null;
        function clearData() {
            refs.state.textContent = t('desktop.loading');
            refs.progress.hidden = true;
            for (const key of ['percent', 'file', 'remaining', 'layers', 'nozzle', 'bed']) refs[key].textContent = key === 'file' ? '' : '—';
        }
        function stopCamera() {
            refs.image.removeAttribute('src');
            refs.image.hidden = true;
            refs.expand.disabled = true;
            refs['camera-error'].hidden = false;
        }
        function startCamera() {
            if (disposed || document.hidden || !refs.select.value) return;
            refs.image.src = '/api/3d-printers/' + encodeURIComponent(refs.select.value) + '/camera/stream?t=' + Date.now();
        }
        refs.image.addEventListener('load', () => {
            refs.image.hidden = false;
            refs['camera-error'].hidden = true;
            refs.expand.disabled = false;
        });
        refs.image.addEventListener('error', () => {
            stopCamera();
            refs['camera-error'].textContent = t('desktop.load_failed');
            if (dialog.open) dialog.close();
        });
        refs.expand.addEventListener('click', () => { refs.large.appendChild(refs.image); dialog.showModal(); });
        refs.close.addEventListener('click', () => dialog.close());
        dialog.addEventListener('close', () => refs.camera.prepend(refs.image));

        async function refresh() {
            if (disposed || document.hidden || controller) return;
            const request = new AbortController();
            controller = request;
            try {
                if (!refs.select.options.length) {
                    const list = await api('/api/3d-printers/status', { signal: request.signal });
                    if (disposed || request.signal.aborted) return;
                    for (const printer of list.printers || []) refs.select.add(new Option(printer.name || printer.id, printer.id));
                    const preferred = localStorage.getItem('aurago.desktop.printer_id') || list.default_printer;
                    if ([...refs.select.options].some(option => option.value === preferred)) refs.select.value = preferred;
                    if (!refs.select.value) {
                        clearData();
                        refs.state.textContent = label('empty');
                        return;
                    }
                    startCamera();
                }
                const raw = await api('/api/3d-printers/status?printer_id=' + encodeURIComponent(refs.select.value), { signal: request.signal });
                if (disposed || request.signal.aborted) return;
                const data = printerWidgetData(raw);
                if (!refs.image.hasAttribute('src')) startCamera();
                refs.state.textContent = label(data.state);
                refs.file.textContent = String(data.filename);
                refs.file.title = refs.file.textContent;
                refs.progress.hidden = data.progress === null;
                refs.progress.value = Math.min(100, data.progress || 0);
                refs.percent.textContent = data.progress === null ? '—' : new Intl.NumberFormat(document.documentElement.lang, { style: 'percent', maximumFractionDigits: 0 }).format(Math.min(100, data.progress) / 100);
                refs.remaining.textContent = data.remaining === null ? '—' : new Intl.NumberFormat(document.documentElement.lang, { style: 'unit', unit: 'minute', unitDisplay: 'short' }).format(Math.ceil(data.remaining / 60));
                refs.layers.textContent = data.layer !== null && data.layers > 0 ? data.layer + ' / ' + data.layers : '—';
                for (const key of ['nozzle', 'bed']) refs[key].textContent = data[key] === null ? '—' : new Intl.NumberFormat(document.documentElement.lang, { style: 'unit', unit: 'celsius', maximumFractionDigits: 0 }).format(data[key]);
            } catch (_) {
                if (!disposed && !request.signal.aborted) { clearData(); stopCamera(); refs.state.textContent = t('desktop.load_failed'); }
            } finally {
                if (controller === request) controller = null;
            }
        }
        refs.select.addEventListener('change', () => {
            if (controller) controller.abort();
            controller = null;
            clearData();
            if (dialog.open) dialog.close();
            stopCamera();
            localStorage.setItem('aurago.desktop.printer_id', refs.select.value);
            startCamera();
            refresh();
        });
        refs.refresh.addEventListener('click', () => { startCamera(); refresh(); });
        const onVisibility = () => {
            if (document.hidden) { if (controller) controller.abort(); controller = null; stopCamera(); if (dialog.open) dialog.close(); }
            else { startCamera(); refresh(); }
        };
        document.addEventListener('visibilitychange', onVisibility);
        const timer = setInterval(refresh, 30000);
        clearData();
        refresh();
        registerWidgetCleanup(() => {
            disposed = true;
            if (controller) controller.abort();
            clearInterval(timer);
            document.removeEventListener('visibilitychange', onVisibility);
            if (dialog.open) dialog.close();
            stopCamera();
        });
    }
