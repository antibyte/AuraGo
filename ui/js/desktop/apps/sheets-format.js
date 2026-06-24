(function () {
    'use strict';

    function escapeHTML(s) {
        return String(s || '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
    }

    function renderToolbar(host, t, onFormatChange) {
        if (!host) return null;
        const e = escapeHTML;
        const toolbar = document.createElement('div');
        toolbar.className = 'office-format-toolbar';
        toolbar.innerHTML = `
            <button type="button" class="office-fmt-btn" data-fmt="bold" title="${e(t('desktop.sheets_format_bold'))}"><strong>B</strong></button>
            <button type="button" class="office-fmt-btn" data-fmt="italic" title="${e(t('desktop.sheets_format_italic'))}"><em>I</em></button>
            <button type="button" class="office-fmt-btn" data-fmt="underline" title="${e(t('desktop.sheets_format_underline'))}"><u>U</u></button>
            <span class="office-fmt-separator"></span>
            <button type="button" class="office-fmt-btn" data-fmt="align-left" title="${e(t('desktop.sheets_format_align_left'))}">&#8676;</button>
            <button type="button" class="office-fmt-btn" data-fmt="align-center" title="${e(t('desktop.sheets_format_align_center'))}">&#8633;</button>
            <button type="button" class="office-fmt-btn" data-fmt="align-right" title="${e(t('desktop.sheets_format_align_right'))}">&#8677;</button>
            <span class="office-fmt-separator"></span>
            <div class="office-fmt-dropdown" data-dropdown="font-color">
                <button type="button" class="office-fmt-btn office-fmt-color-btn" data-fmt="font-color" title="${e(t('desktop.sheets_format_font_color'))}">
                    <span class="office-fmt-color-indicator" style="border-bottom-color:#f6f7fb">A</span>
                </button>
                <div class="office-color-picker" data-picker="font-color" hidden></div>
            </div>
            <div class="office-fmt-dropdown" data-dropdown="fill-color">
                <button type="button" class="office-fmt-btn office-fmt-color-btn" data-fmt="fill-color" title="${e(t('desktop.sheets_format_fill_color'))}">
                    <span class="office-fmt-color-indicator office-fmt-fill-indicator" style="background:#27c7a6">&#9632;</span>
                </button>
                <div class="office-color-picker" data-picker="fill-color" hidden></div>
            </div>
            <span class="office-fmt-separator"></span>
            <select class="office-fmt-select" data-fmt="number-format" title="${e(t('desktop.sheets_format_number'))}">
                <option value="">General</option>
                <option value="0">${e(t('desktop.sheets_format_number'))}</option>
                <option value="0.00">${e(t('desktop.sheets_format_number'))} (0.00)</option>
                <option value="#,##0">#,##0</option>
                <option value="$#,##0.00">${e(t('desktop.sheets_format_currency'))}</option>
                <option value="0%">${e(t('desktop.sheets_format_percent'))}</option>
                <option value="mm-dd-yy">${e(t('desktop.sheets_format_date'))}</option>
                <option value="@">${e(t('desktop.sheets_format_text'))}</option>
            </select>
            <span class="office-fmt-separator"></span>
            <select class="office-fmt-select" data-fmt="border" title="${e(t('desktop.sheets_format_borders'))}">
                <option value="">${e(t('desktop.sheets_format_border_none'))}</option>
                <option value="outer">${e(t('desktop.sheets_format_border_outer'))}</option>
                <option value="all">${e(t('desktop.sheets_format_border_all'))}</option>
                <option value="inner">${e(t('desktop.sheets_format_border_inner'))}</option>
                <option value="top">${e(t('desktop.sheets_format_border_top'))}</option>
                <option value="bottom">${e(t('desktop.sheets_format_border_bottom'))}</option>
                <option value="left">${e(t('desktop.sheets_format_border_left'))}</option>
                <option value="right">${e(t('desktop.sheets_format_border_right'))}</option>
            </select>`;
        host.appendChild(toolbar);

        const colors = [
            '#000000', '#434343', '#666666', '#999999', '#b7b7b7', '#cccccc', '#d9d9d9', '#efefef', '#f3f3f3', '#ffffff',
            '#980000', '#ff0000', '#ff9900', '#ffff00', '#00ff00', '#00ffff', '#4a86e8', '#0000ff', '#9900ff', '#ff00ff',
            '#e6b8af', '#f4cccc', '#fce5cd', '#fff2cc', '#d9ead3', '#d0e0e3', '#c9daf8', '#cfe2f3', '#d9d2e9', '#ead1dc'
        ];

        toolbar.querySelectorAll('.office-color-picker').forEach(picker => {
            colors.forEach(color => {
                const swatch = document.createElement('button');
                swatch.type = 'button';
                swatch.className = 'office-color-swatch';
                swatch.style.background = color;
                swatch.dataset.color = color;
                swatch.title = color;
                picker.appendChild(swatch);
            });
            const custom = document.createElement('div');
            custom.className = 'office-color-custom';
            custom.innerHTML = `<input type="color" class="office-color-input" value="#000000"><button type="button" class="office-color-apply">OK</button>`;
            picker.appendChild(custom);
        });

        return toolbar;
    }

    function applyFormat(cell, formatType, value) {
        if (!cell) return;
        if (!cell.format) cell.format = {};
        switch (formatType) {
        case 'bold': cell.format.bold = !cell.format.bold; break;
        case 'italic': cell.format.italic = !cell.format.italic; break;
        case 'underline': cell.format.underline = !cell.format.underline; break;
        case 'align-left': cell.format.hAlign = cell.format.hAlign === 'left' ? '' : 'left'; break;
        case 'align-center': cell.format.hAlign = cell.format.hAlign === 'center' ? '' : 'center'; break;
        case 'align-right': cell.format.hAlign = cell.format.hAlign === 'right' ? '' : 'right'; break;
        case 'font-color': cell.format.fontColor = value; break;
        case 'fill-color': cell.format.fillColor = value; break;
        case 'number-format': cell.format.numFormat = value; break;
        case 'border': applyBorderFormat(cell, value); break;
        }
        if (cell.format.bold === false) delete cell.format.bold;
        if (cell.format.italic === false) delete cell.format.italic;
        if (cell.format.underline === false) delete cell.format.underline;
        if (!cell.format.hAlign) delete cell.format.hAlign;
        if (!cell.format.vAlign) delete cell.format.vAlign;
        if (!cell.format.fontColor) delete cell.format.fontColor;
        if (!cell.format.fillColor) delete cell.format.fillColor;
        if (!cell.format.numFormat) delete cell.format.numFormat;
        if (Object.keys(cell.format).length === 0) delete cell.format;
    }

    function applyBorderFormat(cell, type) {
        if (!type) { delete cell.format.borders; return; }
        if (!cell.format.borders) cell.format.borders = {};
        const border = { style: 'thin', color: '#000000' };
        switch (type) {
        case 'outer':
            cell.format.borders = { top: { ...border }, bottom: { ...border }, left: { ...border }, right: { ...border } };
            break;
        case 'all':
            cell.format.borders = { top: { ...border }, bottom: { ...border }, left: { ...border }, right: { ...border } };
            break;
        case 'inner':
            cell.format.borders = { top: { ...border }, bottom: { ...border }, left: { ...border }, right: { ...border } };
            break;
        case 'none':
            cell.format.borders = {};
            break;
        case 'top': cell.format.borders.top = { ...border }; break;
        case 'bottom': cell.format.borders.bottom = { ...border }; break;
        case 'left': cell.format.borders.left = { ...border }; break;
        case 'right': cell.format.borders.right = { ...border }; break;
        }
        if (Object.keys(cell.format.borders).length === 0) delete cell.format.borders;
    }

    function getFormatForCell(cell) {
        return (cell && cell.format) || {};
    }

    function renderFormatStyles(td, input, format) {
        if (!format || !td) return;
        const style = [];
        if (format.bold) style.push('font-weight:bold');
        if (format.italic) style.push('font-style:italic');
        if (format.underline) style.push('text-decoration:underline');
        if (format.fontColor) style.push('color:' + format.fontColor);
        if (format.fillColor) style.push('background:' + format.fillColor);
        if (format.hAlign) style.push('text-align:' + format.hAlign);
        if (style.length) td.style.cssText = style.join(';');
        if (input) {
            const inputStyle = [];
            if (format.bold) inputStyle.push('font-weight:bold');
            if (format.italic) inputStyle.push('font-style:italic');
            if (format.underline) inputStyle.push('text-decoration:underline');
            if (format.fontColor) inputStyle.push('color:' + format.fontColor);
            if (format.hAlign) inputStyle.push('text-align:' + format.hAlign);
            if (inputStyle.length) input.style.cssText = inputStyle.join(';');
        }
    }

    function updateToolbarState(toolbar, format) {
        if (!toolbar) return;
        const f = format || {};
        toolbar.querySelectorAll('.office-fmt-btn').forEach(btn => btn.classList.remove('active'));
        if (f.bold) toolbar.querySelector('[data-fmt="bold"]')?.classList.add('active');
        if (f.italic) toolbar.querySelector('[data-fmt="italic"]')?.classList.add('active');
        if (f.underline) toolbar.querySelector('[data-fmt="underline"]')?.classList.add('active');
        if (f.hAlign === 'left') toolbar.querySelector('[data-fmt="align-left"]')?.classList.add('active');
        if (f.hAlign === 'center') toolbar.querySelector('[data-fmt="align-center"]')?.classList.add('active');
        if (f.hAlign === 'right') toolbar.querySelector('[data-fmt="align-right"]')?.classList.add('active');
        const numSelect = toolbar.querySelector('[data-fmt="number-format"]');
        if (numSelect) numSelect.value = f.numFormat || '';
        const borderSelect = toolbar.querySelector('[data-fmt="border"]');
        if (borderSelect) borderSelect.value = '';
    }

    window.SheetsFormat = {
        renderToolbar: renderToolbar,
        applyFormat: applyFormat,
        getFormatForCell: getFormatForCell,
        renderFormatStyles: renderFormatStyles,
        updateToolbarState: updateToolbarState
    };
})();
