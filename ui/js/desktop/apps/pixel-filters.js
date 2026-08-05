(function () {
    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};

    // --- pixel-level filter kernels (operate on ImageData in place) ---

    function convolveKernel(imgData, w, h, kernel, offset) {
        const src = new Uint8ClampedArray(imgData.data);
        const d = imgData.data;
        const kSize = Math.sqrt(kernel.length) | 0;
        const half = (kSize / 2) | 0;
        for (let y = half; y < h - half; y++) {
            for (let x = half; x < w - half; x++) {
                let r = 0, g = 0, b = 0;
                for (let ky = 0; ky < kSize; ky++) {
                    for (let kx = 0; kx < kSize; kx++) {
                        const idx = ((y + ky - half) * w + (x + kx - half)) * 4;
                        const kv = kernel[ky * kSize + kx];
                        r += src[idx] * kv; g += src[idx + 1] * kv; b += src[idx + 2] * kv;
                    }
                }
                const idx = (y * w + x) * 4;
                d[idx] = Pixel.clamp255(r + offset);
                d[idx + 1] = Pixel.clamp255(g + offset);
                d[idx + 2] = Pixel.clamp255(b + offset);
            }
        }
    }

    function grayscaleInPlace(d) {
        for (let i = 0; i < d.length; i += 4) {
            const lum = 0.299 * d[i] + 0.587 * d[i + 1] + 0.114 * d[i + 2];
            d[i] = lum; d[i + 1] = lum; d[i + 2] = lum;
        }
    }

    function fxPosterize(imgData) {
        const d = imgData.data;
        const step = 255 / 4;
        for (let i = 0; i < d.length; i += 4) {
            d[i] = Pixel.clamp255(Math.round(d[i] / step) * step);
            d[i + 1] = Pixel.clamp255(Math.round(d[i + 1] / step) * step);
            d[i + 2] = Pixel.clamp255(Math.round(d[i + 2] / step) * step);
        }
    }

    function fxDuotone(imgData, w, h, opts) {
        const d = imgData.data;
        const c1 = Pixel.hexToRgb((opts && opts.secondary) || '#000000');
        const c2 = Pixel.hexToRgb((opts && opts.primary) || '#ffffff');
        for (let i = 0; i < d.length; i += 4) {
            const lum = (0.299 * d[i] + 0.587 * d[i + 1] + 0.114 * d[i + 2]) / 255;
            d[i] = Pixel.clamp255(c1.r + (c2.r - c1.r) * lum);
            d[i + 1] = Pixel.clamp255(c1.g + (c2.g - c1.g) * lum);
            d[i + 2] = Pixel.clamp255(c1.b + (c2.b - c1.b) * lum);
        }
    }

    function fxSolarize(imgData) {
        const d = imgData.data;
        for (let i = 0; i < d.length; i += 4) {
            d[i] = d[i] < 128 ? 255 - d[i] : d[i];
            d[i + 1] = d[i + 1] < 128 ? 255 - d[i + 1] : d[i + 1];
            d[i + 2] = d[i + 2] < 128 ? 255 - d[i + 2] : d[i + 2];
        }
    }

    function fxSharpen(imgData, w, h) {
        convolveKernel(imgData, w, h, [0, -1, 0, -1, 5, -1, 0, -1, 0], 0);
    }

    function fxEdge(imgData, w, h) {
        convolveKernel(imgData, w, h, [-1, -1, -1, -1, 8, -1, -1, -1, -1], 0);
    }

    function fxEmboss(imgData, w, h) {
        convolveKernel(imgData, w, h, [-2, -1, 0, -1, 1, 1, 0, 1, 2], 128);
    }

    function fxSketch(imgData, w, h) {
        grayscaleInPlace(imgData.data);
        convolveKernel(imgData, w, h, [-1, -1, -1, -1, 8, -1, -1, -1, -1], 255);
    }

    function fxGrain(imgData) {
        const d = imgData.data;
        for (let i = 0; i < d.length; i += 4) {
            const n = (Math.random() * 2 - 1) * 26;
            d[i] = Pixel.clamp255(d[i] + n);
            d[i + 1] = Pixel.clamp255(d[i + 1] + n);
            d[i + 2] = Pixel.clamp255(d[i + 2] + n);
        }
    }

    function fxAberration(imgData, w, h) {
        const src = new Uint8ClampedArray(imgData.data);
        const d = imgData.data;
        const shift = Math.max(1, Math.round(w / 320));
        for (let y = 0; y < h; y++) {
            for (let x = 0; x < w; x++) {
                const idx = (y * w + x) * 4;
                const xr = Math.min(w - 1, x + shift);
                const xb = Math.max(0, x - shift);
                d[idx] = src[(y * w + xr) * 4];
                d[idx + 2] = src[(y * w + xb) * 4 + 2];
            }
        }
    }

    function fxPixelate(imgData, w, h) {
        const d = imgData.data;
        const s = Math.max(4, Math.min(32, Math.round(Math.min(w, h) / 72)));
        for (let by = 0; by < h; by += s) {
            for (let bx = 0; bx < w; bx += s) {
                let r = 0, g = 0, b = 0, n = 0;
                for (let y = by; y < Math.min(h, by + s); y++) {
                    for (let x = bx; x < Math.min(w, bx + s); x++) {
                        const idx = (y * w + x) * 4;
                        r += d[idx]; g += d[idx + 1]; b += d[idx + 2]; n++;
                    }
                }
                r = Pixel.clamp255(r / n); g = Pixel.clamp255(g / n); b = Pixel.clamp255(b / n);
                for (let y = by; y < Math.min(h, by + s); y++) {
                    for (let x = bx; x < Math.min(w, bx + s); x++) {
                        const idx = (y * w + x) * 4;
                        d[idx] = r; d[idx + 1] = g; d[idx + 2] = b;
                    }
                }
            }
        }
    }

    // --- canvas-level filters (need compositing / gradients) ---

    function fxVignette(ctx, w, h) {
        const grd = ctx.createRadialGradient(w / 2, h / 2, Math.min(w, h) * 0.25, w / 2, h / 2, Math.max(w, h) * 0.7);
        grd.addColorStop(0, 'rgba(0,0,0,0)');
        grd.addColorStop(1, 'rgba(0,0,0,0.6)');
        ctx.fillStyle = grd;
        ctx.fillRect(0, 0, w, h);
    }

    function fxBloom(ctx, w, h) {
        const copy = Pixel.acquireTempCanvas(w, h);
        copy.getContext('2d').drawImage(ctx.canvas, 0, 0);
        ctx.save();
        ctx.filter = 'blur(' + Math.max(2, Math.round(Math.min(w, h) / 160)) + 'px) brightness(1.2)';
        ctx.globalCompositeOperation = 'screen';
        ctx.drawImage(copy, 0, 0);
        ctx.restore();
        Pixel.releaseTempCanvas(copy);
    }

    // --- filter catalog ---

    Pixel.FILTER_CATEGORIES = ['color', 'light', 'style', 'detail'];
    Pixel.FILTERS = [
        { id: 'grayscale', cat: 'color', css: 'grayscale(100%)' },
        { id: 'sepia', cat: 'color', css: 'sepia(100%)' },
        { id: 'invert', cat: 'color', css: 'invert(100%)' },
        { id: 'warm', cat: 'color', css: 'saturate(1.3) brightness(1.05)' },
        { id: 'cool', cat: 'color', css: 'saturate(0.8) hue-rotate(20deg)' },
        { id: 'tint', cat: 'color', css: 'hue-rotate(90deg) saturate(1.15)' },
        { id: 'posterize', cat: 'color', pixel: fxPosterize },
        { id: 'duotone', cat: 'color', pixel: fxDuotone },
        { id: 'solarize', cat: 'color', pixel: fxSolarize },
        { id: 'vintage', cat: 'light', css: 'sepia(60%) contrast(80%) brightness(90%)' },
        { id: 'high-contrast', cat: 'light', css: 'contrast(150%)' },
        { id: 'vignette', cat: 'light', canvas: fxVignette },
        { id: 'bloom', cat: 'light', canvas: fxBloom },
        { id: 'blur', cat: 'detail', css: 'blur(2px)' },
        { id: 'sharpen', cat: 'detail', pixel: fxSharpen },
        { id: 'edge', cat: 'detail', pixel: fxEdge },
        { id: 'emboss', cat: 'detail', pixel: fxEmboss },
        { id: 'pixelate', cat: 'style', pixel: fxPixelate },
        { id: 'grain', cat: 'style', pixel: fxGrain },
        { id: 'sketch', cat: 'style', pixel: fxSketch },
        { id: 'aberration', cat: 'style', pixel: fxAberration }
    ];

    Pixel.installFilters = function installFilters(runtime) {
        Object.assign(runtime, {
            filterDef: Pixel.bindRuntime(runtime, function filterDef(id) {
                return Pixel.FILTERS.find(f => f.id === id) || null;
            }),
            filterOpts: Pixel.bindRuntime(runtime, function filterOpts() {
                return { primary: this.primaryColor, secondary: this.secondaryColor };
            }),
            paintFiltered: Pixel.bindRuntime(runtime, function paintFiltered(ctx, src, w, h, def, opts) {
                ctx.save();
                if (def.css) ctx.filter = def.css;
                ctx.drawImage(src, 0, 0, w, h);
                ctx.restore();
                if (def.canvas) def.canvas(ctx, w, h, opts);
                if (def.pixel) {
                    const imgData = ctx.getImageData(0, 0, w, h);
                    def.pixel(imgData, w, h, opts);
                    ctx.putImageData(imgData, 0, 0);
                }
            }),
            renderFiltered: Pixel.bindRuntime(runtime, function renderFiltered(src, w, h, id, strength) {
                const def = this.filterDef(id);
                if (!def || !src || w < 1 || h < 1) return null;
                const opts = this.filterOpts();
                const out = this.acquireTempCanvas(w, h);
                const octx = out.getContext('2d');
                if (strength >= 100) {
                    this.paintFiltered(octx, src, w, h, def, opts);
                } else {
                    octx.drawImage(src, 0, 0, w, h);
                    const fc = this.acquireTempCanvas(w, h);
                    this.paintFiltered(fc.getContext('2d'), src, w, h, def, opts);
                    octx.globalAlpha = Math.max(0, Math.min(1, strength / 100));
                    octx.drawImage(fc, 0, 0);
                    octx.globalAlpha = 1;
                    this.releaseTempCanvas(fc);
                }
                return out;
            }),
            buildFilterGalleryHTML: Pixel.bindRuntime(runtime, function buildFilterGalleryHTML() {
                const chips = ['all'].concat(Pixel.FILTER_CATEGORIES).map(cat =>
                    `<button class="pixel-chip${this.activeFilterCat === cat ? ' active' : ''}" type="button" data-filter-cat="${cat}">${this.esc(this.t('pixel.cat_' + cat, cat))}</button>`
                ).join('');
                const cards = Pixel.FILTERS.map(f =>
                    `<button class="pixel-filter-card" type="button" data-filter-id="${f.id}" data-filter-card-cat="${f.cat}" title="${this.esc(this.t('pixel.filter_' + f.id, f.id))}">
                        <canvas class="pixel-filter-thumb" data-filter-thumb="${f.id}" width="96" height="64"></canvas>
                        <span class="pixel-filter-name">${this.esc(this.t('pixel.filter_' + f.id, f.id))}</span>
                    </button>`
                ).join('');
                return `<div class="pixel-panel-section pixel-panel-filters" data-section="filters" hidden>
                    <div class="pixel-filter-chips" data-filter-chips>${chips}</div>
                    <div class="pixel-filter-gallery" data-filter-gallery>${cards}</div>
                    <div class="pixel-slider-row"><span class="pixel-slider-label">${this.esc(this.t('pixel.filter_strength'))}</span><input type="range" class="pixel-slider" data-filter-strength min="0" max="100" value="${this.filterStrength}"><span class="pixel-slider-value" data-filter-strength-val>${this.filterStrength}%</span></div>
                    <div class="pixel-panel-actions">
                        <button class="pixel-btn pixel-btn-primary" type="button" data-action="apply-filter">${this.esc(this.t('pixel.apply_filter'))}</button>
                        <button class="pixel-btn" type="button" data-action="reset-filter">${this.esc(this.t('pixel.reset'))}</button>
                    </div>
                </div>`;
            }),
            wireFilterPanel: Pixel.bindRuntime(runtime, function wireFilterPanel() {
                this.host.querySelectorAll('[data-filter-cat]').forEach(btn => {
                    btn.addEventListener('click', () => {
                        this.activeFilterCat = btn.dataset.filterCat;
                        this.host.querySelectorAll('[data-filter-cat]').forEach(b => b.classList.toggle('active', b === btn));
                        this.host.querySelectorAll('[data-filter-id]').forEach(card => {
                            card.hidden = this.activeFilterCat !== 'all' && card.dataset.filterCardCat !== this.activeFilterCat;
                        });
                    });
                });
                this.host.querySelectorAll('[data-filter-id]').forEach(card => {
                    card.addEventListener('click', () => this.selectFilter(card.dataset.filterId));
                });
                const slider = this.host.querySelector('[data-filter-strength]');
                if (slider) {
                    slider.addEventListener('input', () => {
                        this.filterStrength = Number(slider.value);
                        const val = this.host.querySelector('[data-filter-strength-val]');
                        if (val) val.textContent = this.filterStrength + '%';
                        this.scheduleFilterPreview();
                    });
                }
            }),
            selectFilter: Pixel.bindRuntime(runtime, function selectFilter(id) {
                if (!this.canvas.width || !this.filterDef(id)) return;
                this.activeFilterId = id;
                this.host.querySelectorAll('[data-filter-id]').forEach(c => c.classList.toggle('active', c.dataset.filterId === id));
                if (!this.filterPreview) {
                    const target = this.getActiveCtx();
                    const snap = this.acquireTempCanvas(this.canvas.width, this.canvas.height);
                    snap.getContext('2d').drawImage(target.canvas, 0, 0);
                    this.filterPreview = { snap, filtered: null };
                }
                if (this.filterPreview.filtered) this.releaseTempCanvas(this.filterPreview.filtered);
                this.filterPreview.filtered = this.renderFiltered(this.filterPreview.snap, this.canvas.width, this.canvas.height, id, 100);
                this.previewFilterBlend();
            }),
            scheduleFilterPreview: Pixel.bindRuntime(runtime, function scheduleFilterPreview() {
                if (!this.filterPreview || !this.activeFilterId) return;
                if (this._filterPreviewRAF) cancelAnimationFrame(this._filterPreviewRAF);
                this._filterPreviewRAF = requestAnimationFrame(() => {
                    this._filterPreviewRAF = null;
                    this.previewFilterBlend();
                });
            }),
            previewFilterBlend: Pixel.bindRuntime(runtime, function previewFilterBlend() {
                if (!this.filterPreview || !this.activeFilterId || !this.canvas.width) return;
                const target = this.getActiveCtx();
                const w = this.canvas.width, h = this.canvas.height;
                target.clearRect(0, 0, w, h);
                target.drawImage(this.filterPreview.snap, 0, 0);
                if (this.filterPreview.filtered) {
                    target.globalAlpha = Math.max(0, Math.min(1, this.filterStrength / 100));
                    target.drawImage(this.filterPreview.filtered, 0, 0);
                    target.globalAlpha = 1;
                }
                if (this.layers.length > 1) this.compositeLayers();
            }),
            applyFilter: Pixel.bindRuntime(runtime, function applyFilter() {
                if (!this.filterPreview) return;
                this.releaseTempCanvas(this.filterPreview.snap);
                if (this.filterPreview.filtered) this.releaseTempCanvas(this.filterPreview.filtered);
                this.filterPreview = null;
                this.pushHistory('filter:' + this.activeFilterId);
                this.activeFilterId = null;
                this.host.querySelectorAll('[data-filter-id]').forEach(c => c.classList.remove('active'));
                this.filterThumbKey = '';
            }),
            resetFilterPreview: Pixel.bindRuntime(runtime, function resetFilterPreview() {
                if (!this.filterPreview) return;
                const target = this.getActiveCtx();
                target.clearRect(0, 0, this.canvas.width, this.canvas.height);
                target.drawImage(this.filterPreview.snap, 0, 0);
                if (this.layers.length > 1) this.compositeLayers();
                this.releaseTempCanvas(this.filterPreview.snap);
                if (this.filterPreview.filtered) this.releaseTempCanvas(this.filterPreview.filtered);
                this.filterPreview = null;
                this.activeFilterId = null;
                this.host.querySelectorAll('[data-filter-id]').forEach(c => c.classList.remove('active'));
            }),
            applyFilterPreview: Pixel.bindRuntime(runtime, function applyFilterPreview(name) {
                if (!this.canvas.width) return;
                this.selectFilter(name);
                this.filterStrength = 100;
                this.applyFilter();
            }),
            refreshFilterThumbnails: Pixel.bindRuntime(runtime, function refreshFilterThumbnails() {
                const gallery = this.host.querySelector('[data-filter-gallery]');
                if (!gallery) return;
                if (!this.canvas.width) return;
                const key = this.canvas.width + 'x' + this.canvas.height + ':' + this.history.length + ':' + this.historyIdx;
                if (key === this.filterThumbKey) return;
                this.filterThumbKey = key;
                const scale = Math.min(1, 96 / this.canvas.width);
                const tw = Math.max(1, Math.round(this.canvas.width * scale));
                const th = Math.max(1, Math.round(this.canvas.height * scale));
                const base = this.acquireTempCanvas(tw, th);
                base.getContext('2d').drawImage(this.canvas, 0, 0, tw, th);
                const thumbs = Array.from(gallery.querySelectorAll('[data-filter-thumb]'));
                const renderNext = () => {
                    const el = thumbs.shift();
                    if (!el) { this.releaseTempCanvas(base); return; }
                    const out = this.renderFiltered(base, tw, th, el.dataset.filterThumb, 100);
                    if (out) {
                        el.width = tw; el.height = th;
                        el.getContext('2d').drawImage(out, 0, 0);
                        this.releaseTempCanvas(out);
                    }
                    setTimeout(renderNext, 0);
                };
                setTimeout(renderNext, 0);
            })
        });
    };
})();
