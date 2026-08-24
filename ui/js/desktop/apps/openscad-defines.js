(function () {
    'use strict';

    function parse(text) {
        var defines = [];
        String(text || '').split(/\r?\n/).forEach(function (line) {
            var trimmed = line.trim();
            if (!trimmed || trimmed.charAt(0) === '#' || trimmed.indexOf('//') === 0) return;
            var eq = trimmed.indexOf('=');
            if (eq < 1) return;
            var name = trimmed.slice(0, eq).trim();
            var value = trimmed.slice(eq + 1).trim();
            if (!name) return;
            defines.push({ name: name, value: value });
        });
        return defines;
    }

    function toText(defines) {
        if (!Array.isArray(defines) || !defines.length) return '';
        return defines.map(function (row) {
            return (String(row.name || '').trim()) + '=' + (String(row.value != null ? row.value : '').trim());
        }).join('\n');
    }

    function isNumeric(value) {
        return /^\d+(\.\d+)?$/.test(String(value || ''));
    }

    function sliderRange(value) {
        var n = parseFloat(value);
        if (isNaN(n)) n = 0;
        var integer = Number.isInteger(n);
        var mag = Math.abs(n);
        var max = integer
            ? Math.max(100, Math.ceil(mag * 4 / 10) * 10)
            : Math.max(10, Math.ceil((mag * 4 + 1) * 10) / 10);
        var step = integer ? 1 : 0.1;
        var min = n < 0 ? -max : 0;
        return { min: min, max: max, step: step };
    }

    function uniqueDefineName(used) {
        var i = 1;
        while (used.indexOf('param' + i) >= 0) i++;
        return 'param' + i;
    }

    function translate(ctx, key, fallback) {
        return ctx && typeof ctx.t === 'function' ? ctx.t(key, fallback) : fallback;
    }

    function formatRangeHint(ctx, name, value) {
        return String(translate(ctx, 'desktop.openscad.define_range_hint', 'Set {name} (current: {value})'))
            .replace('{name}', String(name || ''))
            .replace('{value}', String(value != null ? value : ''));
    }

    function render(container, defines, onChange, options) {
        if (!container) return;
        var opts = options || {};
        var ctx = opts.ctx || null;
        var readOnly = !!opts.readonly;
        var mode = opts.mode === 'text' ? 'text' : 'sliders';
        var rows = defines ? defines.map(function (r) { return { name: r.name, value: r.value }; }) : [];
        var defaults = rows.map(function (r) { return String(r.value != null ? r.value : ''); });
        var disabledAttr = readOnly ? ' disabled' : '';
        var html = '<div class="oscad-defines-panel" data-oscad-defines-mode="' + esc(mode) + '">';
        html += '<div class="oscad-defines-mode-toggle">';
        html += '<button type="button" class="oscad-chip' + (mode === 'sliders' ? ' active' : '') + '" data-oscad-defines-mode-btn="sliders"' + disabledAttr + '>' + esc(translate(ctx, 'desktop.openscad.editor_slider_mode', 'Use sliders')) + '</button>';
        html += '<button type="button" class="oscad-chip' + (mode === 'text' ? ' active' : '') + '" data-oscad-defines-mode-btn="text"' + disabledAttr + '>' + esc(translate(ctx, 'desktop.openscad.editor_text_mode', 'Edit as text')) + '</button>';
        html += '</div>';

        if (mode === 'text') {
            html += '<textarea class="oscad-defines-text" data-oscad-defines-text rows="5" placeholder="' + esc(translate(ctx, 'desktop.openscad.defines_placeholder', 'name=value')) + '"' + (readOnly ? ' readonly' : '') + '>' + esc(toText(rows)) + '</textarea>';
        } else if (!rows.length) {
            html += '<div class="oscad-defines-empty">' + esc(translate(ctx, 'desktop.openscad.no_defines', 'No defines')) + '</div>';
        } else {
            rows.forEach(function (row, idx) {
                var name = String(row.name || '').trim();
                var value = String(row.value != null ? row.value : '').trim();
                var numeric = isNumeric(value);
                html += '<div class="oscad-define-row">';
                html += '<label class="oscad-define-label" title="' + esc(name) + '">' + esc(name) + '</label>';
                if (numeric) {
                    var range = sliderRange(value);
                    var hint = formatRangeHint(ctx, name, value);
                    html += '<div class="oscad-define-slider-wrap">';
                    html += '<input type="range" class="oscad-define-slider" data-oscad-slider="' + esc(String(idx)) + '" min="' + range.min + '" max="' + range.max + '" step="' + range.step + '" value="' + parseFloat(value) + '" title="' + esc(hint) + '"' + disabledAttr + '>';
                    html += '<input type="number" class="oscad-define-number" data-oscad-number="' + esc(String(idx)) + '" value="' + parseFloat(value) + '" step="' + range.step + '" title="' + esc(hint) + '"' + disabledAttr + '>';
                    html += '</div>';
                } else {
                    html += '<input type="text" class="oscad-define-text" data-oscad-text="' + esc(String(idx)) + '" value="' + esc(value) + '"' + disabledAttr + '>';
                }
                if (!readOnly) {
                    html += '<button type="button" class="oscad-icon-btn oscad-define-reset" data-oscad-reset="' + esc(String(idx)) + '" title="' + esc(translate(ctx, 'desktop.openscad.reset_defaults', 'Reset to default')) + '" aria-label="' + esc(translate(ctx, 'desktop.openscad.reset_defaults', 'Reset to default')) + '">' + esc('⟲') + '</button>';
                    html += '<button type="button" class="oscad-icon-btn oscad-define-remove" data-oscad-remove="' + esc(String(idx)) + '" title="' + esc(translate(ctx, 'desktop.openscad.remove_define', 'Remove define')) + '" aria-label="' + esc(translate(ctx, 'desktop.openscad.remove_define', 'Remove define')) + '">' + esc('−') + '</button>';
                }
                html += '</div>';
            });
        }
        if (mode === 'sliders' && !readOnly) {
            html += '<button type="button" class="oscad-btn oscad-add-define" data-oscad-add-define>' + esc('+ ') + esc(translate(ctx, 'desktop.openscad.add_define', 'Add define')) + '</button>';
        }
        html += '</div>';
        container.innerHTML = html;

        function emit() {
            if (onChange) onChange(toText(rows));
        }

        function rerender() {
            if (typeof opts.onModeChange === 'function') {
                opts.onModeChange('sliders');
            }
        }

        container.querySelectorAll('[data-oscad-defines-mode-btn]').forEach(function (btn) {
            btn.addEventListener('click', function () {
                if (readOnly || typeof opts.onModeChange !== 'function') return;
                opts.onModeChange(btn.getAttribute('data-oscad-defines-mode-btn') === 'text' ? 'text' : 'sliders');
            });
        });

        if (mode === 'text') {
            var textarea = container.querySelector('[data-oscad-defines-text]');
            if (textarea && !readOnly) {
                textarea.addEventListener('input', function () {
                    if (onChange) onChange(textarea.value);
                });
            }
            return;
        }

        if (!rows.length) return;

        container.querySelectorAll('.oscad-define-slider').forEach(function (slider) {
            var idx = Number(slider.dataset.oscadSlider);
            slider.addEventListener('input', function () {
                var numberEl = container.querySelector('[data-oscad-number="' + idx + '"]');
                if (numberEl && document.activeElement !== numberEl) {
                    numberEl.value = slider.value;
                    numberEl.title = formatRangeHint(ctx, rows[idx].name, slider.value);
                }
                slider.title = formatRangeHint(ctx, rows[idx].name, slider.value);
                rows[idx].value = slider.value;
                emit();
            });
        });
        container.querySelectorAll('.oscad-define-number').forEach(function (input) {
            var idx = Number(input.dataset.oscadNumber);
            input.addEventListener('input', function () {
                var slider = container.querySelector('[data-oscad-slider="' + idx + '"]');
                if (slider) {
                    var v = parseFloat(input.value);
                    if (isFinite(v)) {
                        var min = parseFloat(slider.min);
                        var max = parseFloat(slider.max);
                        if (v < min) { slider.min = v; }
                        if (v > max) { slider.max = v; }
                        if (document.activeElement !== slider) slider.value = v;
                        slider.title = formatRangeHint(ctx, rows[idx].name, input.value);
                    }
                }
                input.title = formatRangeHint(ctx, rows[idx].name, input.value);
                rows[idx].value = input.value;
                emit();
            });
        });
        container.querySelectorAll('.oscad-define-text').forEach(function (input) {
            var idx = Number(input.dataset.oscadText);
            input.addEventListener('input', function () {
                rows[idx].value = input.value;
                emit();
            });
        });
        if (!readOnly) {
            container.querySelectorAll('[data-oscad-reset]').forEach(function (btn) {
                btn.addEventListener('click', function () {
                    var idx = Number(btn.dataset.oscadReset);
                    rows[idx].value = defaults[idx];
                    var slider = container.querySelector('[data-oscad-slider="' + idx + '"]');
                    var numberEl = container.querySelector('[data-oscad-number="' + idx + '"]');
                    var textEl = container.querySelector('[data-oscad-text="' + idx + '"]');
                    if (slider) slider.value = defaults[idx];
                    if (numberEl) numberEl.value = defaults[idx];
                    if (textEl) textEl.value = defaults[idx];
                    emit();
                });
            });
            container.querySelectorAll('[data-oscad-remove]').forEach(function (btn) {
                btn.addEventListener('click', function () {
                    var idx = Number(btn.dataset.oscadRemove);
                    rows.splice(idx, 1);
                    emit();
                    rerender();
                });
            });
            var addBtn = container.querySelector('[data-oscad-add-define]');
            if (addBtn) {
                addBtn.addEventListener('click', function () {
                    var used = rows.map(function (r) { return r.name; });
                    rows.push({ name: uniqueDefineName(used), value: '1' });
                    emit();
                    rerender();
                });
            }
        }
    }

    function esc(value) {
        return String(value == null ? '' : value).replace(/[&<>"']/g, function (ch) {
            return ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[ch];
        });
    }

    window.OpenSCADDefines = { parse: parse, render: render, toText: toText };
})();
