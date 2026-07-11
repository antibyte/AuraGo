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
        if (Number.isInteger(n)) {
            var max = Math.max(100, Math.abs(n) * 4);
            return { min: 0, max: max, step: 1 };
        }
        var max = Math.max(Math.abs(n) * 4, Math.abs(n) + 10);
        return { min: 0, max: max, step: 0.1 };
    }

    function render(container, defines, onChange) {
        if (!container) return;
        var rows = defines || [];
        var html = '<div class="oscad-defines-panel">';
        if (!rows.length) {
            html += '<div class="oscad-defines-empty">No defines</div>';
        } else {
            rows.forEach(function (row, idx) {
                var name = String(row.name || '').trim();
                var value = String(row.value != null ? row.value : '').trim();
                var numeric = isNumeric(value);
                html += '<div class="oscad-define-row">';
                html += '<label class="oscad-define-label" title="' + esc(name) + '">' + esc(name) + '</label>';
                if (numeric) {
                    var range = sliderRange(value);
                    html += '<div class="oscad-define-slider-wrap">';
                    html += '<input type="range" class="oscad-define-slider" data-oscad-slider="' + esc(String(idx)) + '" min="' + range.min + '" max="' + range.max + '" step="' + range.step + '" value="' + parseFloat(value) + '">';
                    html += '<input type="number" class="oscad-define-number" data-oscad-number="' + esc(String(idx)) + '" value="' + parseFloat(value) + '" step="' + range.step + '">';
                    html += '</div>';
                } else {
                    html += '<input type="text" class="oscad-define-text" data-oscad-text="' + esc(String(idx)) + '" value="' + esc(value) + '">';
                }
                html += '</div>';
            });
        }
        html += '</div>';
        container.innerHTML = html;

        if (!rows.length) return;

        container.querySelectorAll('.oscad-define-slider').forEach(function (slider) {
            var idx = Number(slider.dataset.oscadSlider);
            slider.addEventListener('input', function () {
                var numberEl = container.querySelector('[data-oscad-number="' + idx + '"]');
                if (numberEl && document.activeElement !== numberEl) {
                    numberEl.value = slider.value;
                }
                rows[idx].value = slider.value;
                if (onChange) onChange(toText(rows));
            });
        });
        container.querySelectorAll('.oscad-define-number').forEach(function (input) {
            var idx = Number(input.dataset.oscadNumber);
            input.addEventListener('input', function () {
                var slider = container.querySelector('[data-oscad-slider="' + idx + '"]');
                if (slider && document.activeElement !== slider) {
                    slider.value = input.value;
                }
                rows[idx].value = input.value;
                if (onChange) onChange(toText(rows));
            });
        });
        container.querySelectorAll('.oscad-define-text').forEach(function (input) {
            var idx = Number(input.dataset.oscadText);
            input.addEventListener('input', function () {
                rows[idx].value = input.value;
                if (onChange) onChange(toText(rows));
            });
        });
    }

    function esc(value) {
        return String(value == null ? '' : value).replace(/[&<>"']/g, function (ch) {
            return ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[ch];
        });
    }

    window.OpenSCADDefines = { parse: parse, render: render, toText: toText };
})();