(function () {
    'use strict';
    const Pixel = window.AuraPixelApp;
    Pixel.registerModule('email', function (r) {
        r.on('ready', () => {
            const binding = window.AuraDesktopEmail?.attach(r.windowId, '.pixel-toolbar', () => ({
                key: r.history[r.historyIdx], disabled: !r.imgWidth || !r.imgHeight || !r.layers.length,
                subject: r.fileName || 'image.png', formats: ['png', 'jpeg', 'webp', 'ora'],
                createFiles: async format => {
                    const name = (r.fileName || 'image').replace(/\.[^.]+$/, '') + '.' + (format === 'jpeg' ? 'jpg' : format);
                    if (format === 'ora') return [new File([await r.oraProjectBlob()], name, {type:'image/openraster'})];
                    let quality = 0.92;
                    if (format !== 'png' && r.ctx.promptDialog) {
                        const input = await r.ctx.promptDialog(r.t('pixel.export_quality'), '92');
                        if (input == null) return false;
                        const value = Number(input);
                        if (!Number.isFinite(value) || value < 50 || value > 100) throw new Error(r.t('pixel.export_quality'));
                        quality = value / 100;
                    }
                    const surface = r.flattenDocument(Pixel.acquireTempCanvas(r.imgWidth, r.imgHeight));
                    try {
                        const blob = await new Promise(resolve => surface.toBlob(resolve, 'image/' + format, quality));
                        return blob ? [new File([blob], name, {type:blob.type})] : [];
                    } finally { Pixel.releaseTempCanvas(surface); }
                }
            }));
            const dispose = r.disposeCore;
            r.disposeCore = function () { binding?.dispose(); return dispose.call(this); };
        });
    });
})();
