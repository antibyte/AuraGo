(function () {
    'use strict';
    const Pixel = window.AuraPixelApp;
    const enabled = r => !r.state.disposed && !r.ctx.readonly && !!r.imgWidth && !!r.imgHeight && r.layers.length < r.MAX_LAYERS;

    Pixel.registerModule('qrcode', function installQR(r) {
        const life = new AbortController();
        let opening = false, dialog;
        r.insertQR = Pixel.bindRuntime(r, async function insertQR() {
            if (opening || !enabled(this)) return;
            const layers = this.layers, revision = this.state.documentRevision, width = this.imgWidth, height = this.imgHeight;
            const entry = this.history[this.historyIdx], selection = this.selection;
            const current = () => enabled(this) && !life.signal.aborted && this.layers === layers && this.state.documentRevision === revision &&
                this.imgWidth === width && this.imgHeight === height && this.history[this.historyIdx] === entry && this.selection === selection;
            opening = true;
            try {
                await window.AuraDesktopModules.loadAppAssets('qr');
                if (!current()) return;
                dialog = window.AuraQRDialog.open({ host: this.host, windowId: this.windowId, t: this.t, signal: life.signal, isCurrent: current,
                    insertion: { unit: 'px', width: Math.max(1, Math.floor(Math.min(width, height) * .4)), canvasWidth: width, canvasHeight: height },
                    onInsert: async result => {
                        const image = await createImageBitmap(result.file);
                        try {
                            if (!current()) throw new Error(this.t('desktop.qr_stale'));
                            const w = Math.round(result.insertWidth), h = Math.round(result.insertHeight), x = Math.round(result.x), y = Math.round(result.y);
                            if (![w, h, x, y].every(Number.isFinite) || w < 1 || h < 1 || x < 0 || y < 0 || x + w > width || y + h > height)
                                throw new Error(this.t('desktop.qr_insert_bounds'));
                            const scaled = Pixel.acquireTempCanvas(w, h);
                            try {
                                const context = scaled.getContext('2d');
                                context.clearRect(0, 0, w, h); context.imageSmoothingEnabled = false;
                                context.drawImage(image, 0, 0, w, h);
                                const pixels = await new Promise(resolve => scaled.toBlob(resolve, 'image/png'));
                                if (!pixels) throw new Error(this.t('desktop.qr_failed'));
                                const signal = AbortSignal.any([life.signal, result.signal]);
                                let expected;
                                for (const blob of [result.file, pixels]) {
                                    const response = await fetch('/api/qr/scan', { method: 'POST', credentials: 'same-origin', cache: 'no-store', headers: { 'Content-Type': 'image/png' }, body: blob, signal });
                                    if (!response.ok) throw Object.assign(new Error(this.t('desktop.qr_failed')), { code: response.status === 422 ? 'insert_unreadable' : 'scan_failed' });
                                    const decoded = await response.json();
                                    if (typeof decoded.text !== 'string' || !decoded.text || expected !== undefined && expected !== decoded.text)
                                        throw Object.assign(new Error(this.t('desktop.qr_failed')), { code: 'insert_unreadable' });
                                    expected = decoded.text;
                                }
                                signal.throwIfAborted();
                                if (!current()) throw new Error(this.t('desktop.qr_stale'));
                                const source = Pixel.acquireTempCanvas(width, height);
                                try {
                                    source.getContext('2d').clearRect(0, 0, width, height); source.getContext('2d').drawImage(scaled, x, y);
                                    const layer = this.createLayer({ name: this.t('desktop.qr_title') });
                                    if (!this.commitLayerPixels(layer, source, { respectSelection: false, history: false }))
                                        throw new Error(this.t('desktop.qr_failed'));
                                    this.layers.push(layer); this.activeLayerIdx = this.layers.length - 1;
                                    this.editMaskMode = false;
                                    this.compositeLayers(); this.refreshLayerPanel(); this.captureAdjustmentSource();
                                    this.pushHistory(this.t('desktop.qr_insert')); this.syncActiveLayer();
                                } finally { Pixel.releaseTempCanvas(source); }
                            } finally { Pixel.releaseTempCanvas(scaled); }
                        } finally { image.close(); }
                    }
                });
                await dialog.closed;
            } catch (error) {
                if (!life.signal.aborted && !this.state.disposed && error.name !== 'AbortError')
                    this.notify({ type: 'error', message: error.message || this.t('desktop.qr_failed') });
            } finally { opening = false; dialog = null; }
        });
        const dispose = r.disposeCore;
        r.disposeCore = Pixel.bindRuntime(r, function disposeQR() {
            life.abort(); dialog?.close(); return dispose.apply(this, arguments);
        });
    });
    Pixel.registerMenuItems('generate', [{ id: 'insert-qr', labelKey: 'desktop.qr_insert', icon: 'image', enabled,
        action: r => r.insertQR() }], { order: -10 });
    Pixel.registerCommand({ id: 'insert-qr', labelKey: 'desktop.qr_insert', keywords: ['QR', 'code', 'barcode'],
        group: 'generate', enabled, run: r => r.insertQR() });
})();
