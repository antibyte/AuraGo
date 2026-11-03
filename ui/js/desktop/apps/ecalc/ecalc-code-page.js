/* ECalc code page: reading and writing component markings. One page creator for the five code items of the catalog
   (resistor colour bands, inductor colour bands, SMD resistor codes, capacitor codes, ceramic temperature codes); the
   `code` kind of the item picks the controller, the tables come from `item.data`, and every decode and encode goes through
   ECalcCore.codes (ecalc-codes.js). A page has two directions that are live at the same time: the marking (colour bands,
   a printed code) decodes into the fields, and the fields (a value, a tolerance, a voltage ...) encode into the marking.
   The colour bands of the two band kinds are buttons in the drawing (ECalcCodeDraw): a click, Enter, Space or ArrowDown
   opens a menu of the colours that fit the role of the band, with a swatch in the colour of the data. An error of the codes
   engine (invalid_code, invalid_value, unsupported) is shown in the result block and tints the field it came from; nothing
   is thrown. What the user entered (the colours, the value, the tolerance ...) is kept per item in the ECalc store under the
   item id, like the inputs of a calculator. Warnings and the names of the notations come from the engine in English.
   options: {item, units, store, t, icon, uid, decimalComma, onFavorite} (the options object of every page creator, see
   ecalc-shell.js); returns {el, layout, refresh, dispose}. Publishes window.ECalcCodePage = { create }. */
(function () {
    'use strict';

    const core = window.ECalcCore, Typeset = window.ECalcTypeset, Draw = window.ECalcCodeDraw;
    const esc = Typeset.escape;
    const TYPING_PAUSE_MS = 120; // the engine runs after a pause in the typing
    const NARROW_BELOW = 760; // under this width the action buttons show their icon only
    const COPIED_MS = 1600;
    const MAX_TEXT = 64; // the engine takes at most 64 characters per field
    const ERROR_CODES = ['invalid_code', 'invalid_value', 'unsupported'];
    const SUPERSCRIPT = { '-': '⁻', 0: '⁰', 1: '¹', 2: '²', 3: '³', 4: '⁴', 5: '⁵', 6: '⁶', 7: '⁷', 8: '⁸', 9: '⁹' };

    const KEY = {
        bands: 'desktop.ecalc_code_bands', bandAria: 'desktop.ecalc_code_band_aria', menu: 'desktop.ecalc_code_menu', hint: 'desktop.ecalc_code_hint',
        auto: 'desktop.ecalc_code_auto', none: 'desktop.ecalc_code_none', marking: 'desktop.ecalc_code_marking', colors: 'desktop.ecalc_code_colors',
        result: 'desktop.ecalc_code_result', empty: 'desktop.ecalc_code_empty', common: 'desktop.ecalc_code_common', print: 'desktop.ecalc_code_print',
        notes: 'desktop.ecalc_code_notes', class1: 'desktop.ecalc_code_class_1', class2: 'desktop.ecalc_code_class_2'
    };
    const ROLE_KEY = {
        digit: 'desktop.ecalc_code_role_digit', multiplier: 'desktop.ecalc_code_role_multiplier', tolerance: 'desktop.ecalc_code_role_tolerance',
        tcr: 'desktop.ecalc_code_role_tcr', identifier: 'desktop.ecalc_code_role_identifier', value: 'desktop.ecalc_code_role_value', point: 'desktop.ecalc_code_role_point'
    };
    const ERROR_KEY = {
        invalid_code: 'desktop.ecalc_code_error_invalid_code', invalid_value: 'desktop.ecalc_code_error_invalid_value', unsupported: 'desktop.ecalc_code_error_unsupported'
    };
    // The names of the decoded fields are English in the engine; the page translates them by field id.
    const FIELD_KEY = {
        tolerance: 'desktop.ecalc_code_field_tolerance', tcr: 'desktop.ecalc_code_field_tcr', layout: 'desktop.ecalc_code_field_layout', family: 'desktop.ecalc_code_field_family',
        code_3digit: 'desktop.ecalc_code_field_code_3digit', code_4digit: 'desktop.ecalc_code_field_code_4digit', code_eia96: 'desktop.ecalc_code_field_code_eia96',
        value_code: 'desktop.ecalc_code_field_value_code', rkm: 'desktop.ecalc_code_field_rkm', voltage: 'desktop.ecalc_code_field_voltage',
        tolerance_minus: 'desktop.ecalc_code_field_tolerance_minus', tolerance_plus: 'desktop.ecalc_code_field_tolerance_plus', class: 'desktop.ecalc_code_field_class',
        tc: 'desktop.ecalc_code_field_tc', industry_code: 'desktop.ecalc_code_field_industry_code', temp_low: 'desktop.ecalc_code_field_temp_low',
        temp_high: 'desktop.ecalc_code_field_temp_high', change: 'desktop.ecalc_code_field_change', change_plus: 'desktop.ecalc_code_field_change_plus',
        change_minus: 'desktop.ecalc_code_field_change_minus'
    };
    const DIM_KEY = {
        resistance: 'desktop.ecalc_dim_resistance', inductance: 'desktop.ecalc_dim_inductance', capacitance: 'desktop.ecalc_dim_capacitance',
        voltage: 'desktop.ecalc_dim_voltage', tempco: 'desktop.ecalc_dim_tempco'
    };

    const closeTo = (a, b) => a === b || Math.abs(a - b) <= 1e-9 * Math.max(Math.abs(a), Math.abs(b));
    const power = exp => (exp === 0 ? '×1' : '×10' + String(exp).replace(/[-\d]/g, char => SUPERSCRIPT[char]));
    const text64 = value => (typeof value === 'string' ? value.slice(0, MAX_TEXT) : '');

    // The error of the engine as the page shows it: a known code, the English message.
    function describe(error) {
        const code = error && ERROR_CODES.includes(error.code) ? error.code : 'invalid_value';
        return { code, message: String(error && error.message ? error.message : error) };
    }

    // ---- the shared parts of a controller ----------------------------------------------------------------------------------
    // ctx: what the page offers a controller (the item, the helpers of the number format, the engine calls and the painting
    // of the result block).
    function swatch(color) {
        const fill = Draw.safeColor(color);
        return '<svg class="ec-swatch" viewBox="0 0 16 16" aria-hidden="true" focusable="false"><rect x="1.5" y="1.5" width="13" height="13" rx="3"' + (fill && fill !== 'transparent' ? ' fill="' + fill + '"' : ' fill="none" stroke-dasharray="2 2"') + '/></svg>';
    }

    // A select inside the wrapper that draws the chevron; the options are filled by the controller (or given).
    const selectHTML = (uid, id, optionsMarkup) => '<span class="ec-select-wrap"><select class="ec-code-select" id="' + uid + '-' + id + '" data-input="' + id + '">' + (optionsMarkup || '') + '</select></span>';

    // Sets the markup of a host only when it changed. A click starts on an element and ends on it; when the blur of a field has
    // drawn the same drawing again in between, the click would find the element replaced and be lost.
    const shownMarkup = new WeakMap();
    function setMarkup(host, markup) {
        if (shownMarkup.get(host) === markup) return false;
        shownMarkup.set(host, markup);
        host.innerHTML = markup;
        return true;
    }

    // ---- band kinds: resistor and inductor ---------------------------------------------------------------------------------
    // cfg: {kind, dim, layouts: [{n, label, roles: [role]}], defaultBands, sample: {value, tolerance}, tolerances(n), tempcos}
    // A role is {role, key, wide, pick(color), meaning(color, colors, index), short(color, colors, index)}.
    function bandsController(ctx, cfg) {
        const { item, data, units, t, codes } = ctx;
        const colorById = new Map(data.colors.map(color => [color.id, color]));
        const st = { n: cfg.defaultBands, colors: [], value: '', tolerance: '', tempco: '', valueSI: NaN };
        let root = null, artEl = null, menuEl = null, stageEl = null, valueInput = null, toleranceSelect = null, tempcoSelect = null;
        let openIndex = -1, timer = 0;
        const layout = () => cfg.layouts.find(entry => entry.n === st.n) || cfg.layouts[0];
        const roleOf = index => layout().roles[index];
        const percentText = x => '±' + units.fmtSig(x, 6) + ' %';

        function toleranceOptions() {
            if (st.n === 3) return [{ value: '20', label: ctx.comma(percentText(20)) }];
            return [{ value: '', label: t(KEY.auto) }].concat(cfg.tolerances(st.n).map(x => ({ value: String(x), label: ctx.comma(percentText(x)) })));
        }
        function tempcoOptions() {
            if (!cfg.tempcos) return [];
            return st.n === 6 ? cfg.tempcos.map(x => ({ value: String(x), label: ctx.comma(x + ' ppm/K') })) : [{ value: '', label: t(KEY.none) }];
        }
        const optionsHTML = (options, selected) => options.map(option => '<option value="' + esc(option.value) + '"' + (option.value === selected ? ' selected' : '') + '>' + esc(option.label) + '</option>').join('');
        // The option of a list whose number equals x (a decoded tolerance or tempco), or ''.
        const matchOption = (options, x) => {
            const found = options.find(option => option.value !== '' && closeTo(Number(option.value), x));
            return found ? found.value : '';
        };

        function markup() {
            const bandsChoice = '<fieldset class="ec-choice" data-choice="bands"><legend>' + esc(t(KEY.bands)) + '</legend><div class="ec-seg">'
                + cfg.layouts.map(entry => '<label><input type="radio" name="' + ctx.uid + '-bands" value="' + entry.n + '"><span>' + esc(entry.label) + '</span></label>').join('') + '</div></fieldset>';
            const stage = '<div class="ec-code-stage ec-stage" data-block="stage"><div class="ec-code-art-host"></div><div class="ec-code-menu" role="listbox" hidden></div>'
                + '<p class="ec-code-hint">' + esc(t(KEY.hint)) + '</p></div>';
            const fields = '<div class="ec-code-fields">'
                + ctx.field('value', t(DIM_KEY[cfg.dim]), '<input class="ec-input" id="' + ctx.uid + '-value" data-input="value" type="text" inputmode="text" maxlength="' + MAX_TEXT + '" autocomplete="off" spellcheck="false" autocapitalize="off" placeholder="' + esc(cfg.placeholder) + '">')
                + ctx.field('tolerance', t(FIELD_KEY.tolerance), selectHTML(ctx.uid, 'tolerance'))
                + (cfg.tempcos ? ctx.field('tempco', t(FIELD_KEY.tcr), selectHTML(ctx.uid, 'tempco')) : '') + '</div>';
            return bandsChoice + stage + fields;
        }

        // ---- drawing and menu ----
        function bandSpecs() {
            return st.colors.map((id, index) => {
                const color = colorById.get(id), role = roleOf(index);
                const roleLabel = t(ROLE_KEY[role.key]);
                return {
                    color: color.hex, name: color.name, role: role.role, roleLabel, wide: !!role.wide,
                    caption: ctx.comma((role.short || role.meaning)(color, st.colors, index)),
                    aria: t(KEY.bandAria, { n: index + 1, role: roleLabel, color: color.name })
                };
            });
        }
        function paintArt() {
            const names = st.colors.map(id => colorById.get(id).name).join(', ');
            const doc = artEl.ownerDocument, active = doc.activeElement && artEl.contains(doc.activeElement) ? doc.activeElement.closest('.ec-band') : null;
            const focused = active ? Number(active.dataset.index) : -1;
            if (setMarkup(artEl, Draw.bandsSVG({ kind: cfg.kind, label: item.name + ': ' + names, interactive: true, open: openIndex, bands: bandSpecs() }))) focusBand(focused);
        }
        function focusBand(index) {
            const band = index >= 0 ? artEl.querySelector('.ec-band[data-index="' + index + '"]') : null;
            if (band) band.focus();
        }
        function setExpanded(index) {
            for (const band of artEl.querySelectorAll('.ec-band')) band.setAttribute('aria-expanded', Number(band.dataset.index) === index ? 'true' : 'false');
        }
        function optionHTML(color, role, index, selected) {
            const meaning = ctx.comma(role.meaning(color, st.colors, index));
            return '<button type="button" role="option" class="ec-code-opt" data-color="' + esc(color.id) + '" aria-selected="' + (selected ? 'true' : 'false') + '" tabindex="-1">'
                + swatch(color.hex) + '<span class="ec-opt-name">' + esc(color.name) + '</span>' + (meaning ? '<span class="ec-opt-meaning">' + esc(meaning) + '</span>' : '') + '</button>';
        }
        function openMenu(index) {
            const role = roleOf(index);
            menuEl.innerHTML = data.colors.filter(role.pick).map(color => optionHTML(color, role, index, color.id === st.colors[index])).join('');
            menuEl.setAttribute('aria-label', t(KEY.menu, { n: index + 1 }));
            menuEl.hidden = false;
            openIndex = index;
            setExpanded(index);
            place(index);
            document.addEventListener('pointerdown', onOutside, true);
            (menuEl.querySelector('[aria-selected="true"]') || menuEl.querySelector('.ec-code-opt')).focus();
        }
        function closeMenu(restoreFocus) {
            if (openIndex < 0) return;
            const index = openIndex;
            openIndex = -1;
            menuEl.hidden = true;
            document.removeEventListener('pointerdown', onOutside, true);
            setExpanded(-1);
            if (restoreFocus) focusBand(index);
        }
        function onOutside(event) {
            if (!menuEl.contains(event.target) && !event.target.closest('.ec-band')) closeMenu(false);
        }
        // The menu hangs under the band, inside the stage.
        function place(index) {
            const fill = artEl.querySelector('.ec-band[data-index="' + index + '"] .ec-band-fill');
            const stageBox = stageEl.getBoundingClientRect();
            if (!fill || !stageBox.width) return;
            const bandBox = fill.getBoundingClientRect(), menuBox = menuEl.getBoundingClientRect();
            const x = Math.max(0, Math.min(bandBox.left + bandBox.width / 2 - stageBox.left - menuBox.width / 2, stageBox.width - menuBox.width));
            menuEl.style.setProperty('--ec-menu-x', Math.round(x) + 'px');
            menuEl.style.setProperty('--ec-menu-y', Math.round(bandBox.bottom - stageBox.top + 12) + 'px');
        }

        // ---- the two directions ----
        function syncControls() {
            for (const radio of root.querySelectorAll('input[name="' + ctx.uid + '-bands"]')) radio.checked = Number(radio.value) === st.n;
            toleranceSelect.innerHTML = optionsHTML(toleranceOptions(), st.tolerance);
            toleranceSelect.disabled = st.n === 3;
            if (tempcoSelect) {
                tempcoSelect.innerHTML = optionsHTML(tempcoOptions(), st.tempco);
                tempcoSelect.disabled = st.n !== 6;
            }
            if (valueInput !== valueInput.ownerDocument.activeElement) valueInput.value = st.value;
        }
        // The fields a result reports (the value and the tolerance, the temperature coefficient of the 6-band code).
        function adopt(result) {
            st.valueSI = result.value;
            st.value = Number.isFinite(result.value) ? ctx.fmt(result.value, cfg.dim) : st.value;
            const tolerance = result.fields.find(field => field.id === 'tolerance');
            if (tolerance && st.n !== 3) st.tolerance = matchOption(toleranceOptions(), tolerance.value * 100) || st.tolerance;
            const tcr = result.fields.find(field => field.id === 'tcr');
            if (tcr) st.tempco = matchOption(tempcoOptions(), tcr.value / 1e-6) || st.tempco;
        }
        // The colours decode into the value, the tolerance and the temperature coefficient.
        function fromColors() {
            const out = ctx.run(() => codes.decode(item, st.colors.join(' '), units), 'bands');
            if (out.result) adopt(out.result);
            syncControls();
            paintArt();
            ctx.persist();
        }
        // The fields encode into the colours of the nearest standard value.
        function fromFields() {
            if (!st.value.trim()) { ctx.clear(); ctx.persist(); return; }
            const request = { value: st.value, bands: st.n, tolerance: st.n === 3 ? '20' : st.tolerance, tempco: st.n === 6 ? st.tempco : '' };
            const out = ctx.run(() => codes.encode(item, request, units), 'value');
            if (out.result) {
                st.colors = out.result.bands.slice();
                st.valueSI = out.result.value;
            }
            paintArt();
            ctx.persist();
        }
        function pickColor(id) {
            const index = openIndex;
            closeMenu(false);
            if (index < 0) return;
            st.colors[index] = id;
            fromColors();
            focusBand(index);
        }

        // The first colour of each role when no colour is known (a layout the colours cannot be taken over into).
        function defaultColor(role) {
            const candidates = data.colors.filter(role.pick);
            return (candidates.find(color => (role.role === 'tcr' ? color.tcr === 100 : role.role === 'tolerance' ? color.tol === 5 : false)) || candidates[0]).id;
        }
        function setBands(n) {
            closeMenu(false);
            const before = layout().roles, queues = {};
            st.colors.forEach((id, index) => { (queues[before[index].role] = queues[before[index].role] || []).push(id); });
            st.n = n;
            // The colours move over role by role; a role without a colour (or a colour it cannot hold) takes its default.
            st.colors = layout().roles.map(role => {
                const id = queues[role.role] && queues[role.role].length ? queues[role.role].shift() : '';
                const color = colorById.get(id);
                return color && role.pick(color) ? id : defaultColor(role);
            });
            if (n === 6 && !st.tempco) st.tempco = String(cfg.tempcos.find(x => x === 100) || cfg.tempcos[0]);
            if (n !== 6) st.tempco = '';
            if (n !== 3 && !toleranceOptions().some(option => option.value === st.tolerance)) st.tolerance = '';
            if (n === 3) st.tolerance = '20';
            else if (st.tolerance === '20') st.tolerance = '';
            syncControls();
            if (st.value.trim()) { fromFields(); syncControls(); } else fromColors();
        }

        function applyDefaults() {
            st.n = cfg.defaultBands;
            st.value = cfg.sample.value;
            st.tolerance = cfg.sample.tolerance;
            st.tempco = '';
            st.colors = [];
            const out = ctx.run(() => codes.encode(item, { value: st.value, bands: st.n, tolerance: st.tolerance }, units), 'value');
            if (out.result) { st.colors = out.result.bands.slice(); st.valueSI = out.result.value; }
            if (!st.colors.length) st.colors = layout().roles.map(defaultColor);
            syncControls();
            paintArt();
        }

        return {
            markup,
            mount(host) {
                root = host;
                stageEl = root.querySelector('[data-block="stage"]');
                artEl = root.querySelector('.ec-code-art-host');
                menuEl = root.querySelector('.ec-code-menu');
                valueInput = root.querySelector('[data-input="value"]');
                toleranceSelect = root.querySelector('[data-input="tolerance"]');
                tempcoSelect = root.querySelector('[data-input="tempco"]');
                root.addEventListener('click', event => {
                    const band = event.target.closest('.ec-band');
                    if (band && artEl.contains(band)) {
                        const index = Number(band.dataset.index);
                        if (openIndex === index) closeMenu(true); else { closeMenu(false); openMenu(index); }
                        return;
                    }
                    const option = event.target.closest('.ec-code-opt');
                    if (option && menuEl.contains(option)) pickColor(option.dataset.color);
                });
                root.addEventListener('keydown', event => {
                    const band = event.target.closest('.ec-band');
                    if (band && artEl.contains(band)) {
                        const index = Number(band.dataset.index);
                        if (event.key === 'Enter' || event.key === 'ArrowDown') {
                            event.preventDefault();
                            if (openIndex !== index) { closeMenu(false); openMenu(index); }
                        } else if (event.key === ' ') {
                            event.preventDefault(); // the menu opens when the key is released, so that the release does not pick a colour
                        } else if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') {
                            event.preventDefault();
                            const next = artEl.querySelector('.ec-band[data-index="' + (index + (event.key === 'ArrowRight' ? 1 : -1)) + '"]');
                            if (next) next.focus();
                        }
                        return;
                    }
                    const option = event.target.closest('.ec-code-opt');
                    if (!option || !menuEl.contains(option)) return;
                    const options = [...menuEl.querySelectorAll('.ec-code-opt')], at = options.indexOf(option);
                    const moves = { ArrowDown: at + 1, ArrowUp: at - 1, Home: 0, End: options.length - 1 };
                    if (event.key in moves) {
                        event.preventDefault();
                        options[Math.max(0, Math.min(options.length - 1, moves[event.key]))].focus();
                    } else if (event.key === 'Escape') {
                        event.preventDefault();
                        event.stopPropagation();
                        closeMenu(true);
                    } else if (event.key === 'Tab') {
                        closeMenu(false);
                    }
                });
                root.addEventListener('keyup', event => {
                    const band = event.key === ' ' ? event.target.closest('.ec-band') : null;
                    if (!band || !artEl.contains(band)) return;
                    const index = Number(band.dataset.index);
                    if (openIndex !== index) { closeMenu(false); openMenu(index); }
                });
                root.addEventListener('input', event => {
                    if (event.target !== valueInput) return;
                    st.value = text64(valueInput.value);
                    clearTimeout(timer);
                    timer = setTimeout(fromFields, TYPING_PAUSE_MS);
                });
                root.addEventListener('change', event => {
                    const target = event.target;
                    if (target === valueInput) {
                        clearTimeout(timer);
                        fromFields();
                        // The field shows the value the colours stand for (the nearest standard value) once the typing is over.
                        const shown = ctx.last();
                        if (shown.result && Number.isFinite(shown.result.value)) { st.value = ctx.fmt(shown.result.value, cfg.dim); valueInput.value = st.value; }
                    } else if (target === toleranceSelect) {
                        st.tolerance = toleranceSelect.value;
                        fromFields();
                    } else if (target === tempcoSelect) {
                        st.tempco = tempcoSelect.value;
                        fromFields();
                    } else if (target.name === ctx.uid + '-bands') {
                        setBands(Number(target.value));
                    }
                });
            },
            // The saved state: the colours, the fields and the number of bands.
            state() {
                return { inputs: { colors: st.colors.join(' '), value: st.value, tolerance: st.tolerance, tempco: st.tempco }, choices: { bands: String(st.n) } };
            },
            restore(saved) {
                const n = Number(saved && saved.choices && saved.choices.bands);
                const inputs = saved && saved.inputs ? saved.inputs : {};
                const colors = text64(inputs.colors).split(' ').filter(Boolean);
                if (cfg.layouts.some(entry => entry.n === n) && colors.length === n && colors.every(id => colorById.has(id))) {
                    st.n = n;
                    st.colors = colors;
                    st.value = text64(inputs.value);
                    st.tolerance = text64(inputs.tolerance);
                    st.tempco = text64(inputs.tempco);
                    const options = toleranceOptions();
                    if (!options.some(option => option.value === st.tolerance)) st.tolerance = n === 3 ? '20' : '';
                    if (n === 6 && !tempcoOptions().some(option => option.value === st.tempco)) st.tempco = String(cfg.tempcos.find(x => x === 100) || cfg.tempcos[0]);
                    if (n !== 6) st.tempco = '';
                    const out = ctx.run(() => codes.decode(item, st.colors.join(' '), units), 'bands');
                    if (out.result) {
                        st.valueSI = out.result.value;
                        st.value = ctx.fmt(out.result.value, cfg.dim);
                    }
                    syncControls();
                    paintArt();
                    return;
                }
                applyDefaults();
            },
            reset() { closeMenu(false); applyDefaults(); },
            valueHTML() { return ''; },
            handleClick() { return false; },
            // The digits setting changed: the value is written again.
            refresh() {
                if (Number.isFinite(st.valueSI) && valueInput !== valueInput.ownerDocument.activeElement) st.value = ctx.fmt(st.valueSI, cfg.dim);
                syncControls();
                paintArt();
            },
            layout() { if (openIndex >= 0) place(openIndex); },
            dispose() { clearTimeout(timer); closeMenu(false); }
        };
    }

    // The roles of a resistor band: IEC 60062 colours; the menu lists the colours that have the field of the role.
    function resistorRoles(ctx, layoutRoles) {
        const { units } = ctx;
        const percent = x => '±' + units.fmtSig(x, 6) + ' %';
        return layoutRoles.map(role => {
            switch (role) {
                case 'digit': return { role, key: 'digit', pick: c => c.digit !== null, meaning: c => String(c.digit) };
                case 'multiplier': return { role, key: 'multiplier', pick: c => c.exp !== null, meaning: c => power(c.exp) };
                case 'tolerance': return { role, key: 'tolerance', wide: true, pick: c => c.tol !== null && c.id !== 'none', meaning: c => percent(c.tol) };
                default: return { role: 'tcr', key: 'tcr', pick: c => c.tcr !== null, meaning: c => c.tcr + ' ppm/K', short: c => String(c.tcr) };
            }
        });
    }

    function resistorController(ctx) {
        const { data, units } = ctx;
        const tolerances = data.colors.filter(c => c.tol !== null && c.id !== 'none').map(c => c.tol).sort((a, b) => a - b);
        return bandsController(ctx, {
            kind: 'resistor', dim: 'resistance', defaultBands: 4, placeholder: '4k7',
            layouts: data.layouts.map(layout => ({ n: layout.bands, label: String(layout.bands), roles: resistorRoles(ctx, layout.roles) })),
            sample: { value: units.format(1000, 'resistance', '', 4), tolerance: '5' },
            tolerances: () => tolerances,
            tempcos: data.colors.filter(c => c.tcr !== null).map(c => c.tcr).sort((a, b) => a - b)
        });
    }

    // The inductor kinds: the EIA code (digit, digit, multiplier, tolerance) and the military code (a wide silver band first, a
    // gold band inside the value bands is the decimal point).
    function inductorRoles(ctx, n) {
        const { data, t, units } = ctx;
        const point = data.colors.find(c => c.mil_role === 'point'), identifier = data.colors.find(c => c.mil_role === 'identifier');
        const percent = x => '±' + units.fmtSig(x, 6) + ' %';
        if (n === 4) {
            return [
                { role: 'digit', key: 'digit', pick: c => c.digit !== null, meaning: c => String(c.digit) },
                { role: 'digit', key: 'digit', pick: c => c.digit !== null, meaning: c => String(c.digit) },
                { role: 'multiplier', key: 'multiplier', pick: c => c.eia_exp !== null, meaning: c => power(c.eia_exp) },
                { role: 'tolerance', key: 'tolerance', wide: true, pick: c => c.eia_tol !== null, meaning: c => percent(c.eia_tol) }
            ];
        }
        const valueRole = c => (c === point ? t(ROLE_KEY.point) : String(c.digit));
        const hasPoint = colors => colors[1] === point.id || colors[2] === point.id;
        return [
            { role: 'identifier', key: 'identifier', wide: true, pick: c => c === identifier, meaning: () => '', short: () => '' },
            { role: 'value', key: 'value', pick: c => c.digit !== null || c === point, meaning: valueRole, short: c => (c === point ? '.' : String(c.digit)) },
            { role: 'value', key: 'value', pick: c => c.digit !== null || c === point, meaning: valueRole, short: c => (c === point ? '.' : String(c.digit)) },
            {
                role: 'value', key: 'value', pick: c => c.digit !== null,
                meaning: c => String(c.digit) + (c.mil_exp !== null ? ' / ' + power(c.mil_exp) : ''),
                short: (c, colors) => (hasPoint(colors) ? String(c.digit) : c.mil_exp !== null ? power(c.mil_exp) : '?')
            },
            { role: 'tolerance', key: 'tolerance', wide: true, pick: c => c.mil_tol !== null, meaning: c => percent(c.mil_tol) }
        ];
    }

    function inductorController(ctx) {
        const { data, units } = ctx;
        const tolerances = data.colors.filter(c => c.eia_tol !== null).map(c => c.eia_tol).sort((a, b) => a - b);
        return bandsController(ctx, {
            kind: 'inductor', dim: 'inductance', defaultBands: 4, placeholder: '100u',
            layouts: [{ n: 4, label: '4 · EIA', roles: inductorRoles(ctx, 4) }, { n: 5, label: '5 · MIL', roles: inductorRoles(ctx, 5) }],
            sample: { value: units.format(100e-6, 'inductance', '', 4), tolerance: '10' },
            tolerances: () => tolerances, tempcos: null
        });
    }

    // ---- SMD resistor ------------------------------------------------------------------------------------------------------
    function smdController(ctx) {
        const { item, units, t, codes } = ctx;
        const st = { code: '', value: '', tolerance: '', printed: '', valueSI: NaN };
        let root = null, artEl = null, codeInput = null, valueInput = null, toleranceInput = null, timer = 0, pending = null;

        function markup() {
            const input = (id, placeholder, extra) => '<input class="ec-input" id="' + ctx.uid + '-' + id + '" data-input="' + id + '" type="text" inputmode="text" maxlength="' + MAX_TEXT + '" autocomplete="off" spellcheck="false" autocapitalize="off" placeholder="' + esc(placeholder) + '"' + (extra || '') + '>';
            return '<div class="ec-code-stage ec-stage" data-block="stage"><div class="ec-code-art-host"></div></div>'
                + '<div class="ec-code-fields">' + ctx.field('code', t(KEY.marking), input('code', '472')) + ctx.field('value', t(DIM_KEY.resistance), input('value', '4k7'))
                + ctx.field('tolerance', t(FIELD_KEY.tolerance), input('tolerance', '1 %')) + '</div>';
        }
        function paintArt() {
            const shown = st.printed || st.code.toUpperCase();
            setMarkup(artEl, Draw.chipSVG({ shape: 'chip', lines: [shown], label: item.name + ': ' + shown, valid: !ctx.last().error }));
        }
        function sync() {
            if (codeInput !== codeInput.ownerDocument.activeElement) codeInput.value = st.code;
            if (valueInput !== valueInput.ownerDocument.activeElement) valueInput.value = st.value;
            if (toleranceInput !== toleranceInput.ownerDocument.activeElement) toleranceInput.value = st.tolerance;
        }
        function fromCode() {
            const out = ctx.run(() => codes.decode(item, st.code, units), 'code');
            if (out.result) {
                st.valueSI = out.result.value;
                st.value = ctx.fmt(out.result.value, 'resistance');
                st.printed = out.result.code;
            } else {
                st.printed = '';
            }
            sync();
            paintArt();
            ctx.persist();
        }
        function fromFields() {
            if (!st.value.trim()) { ctx.clear(); paintArt(); ctx.persist(); return; }
            const out = ctx.run(() => codes.encode(item, { value: st.value, tolerance: st.tolerance }, units), 'value');
            if (out.result) {
                st.valueSI = out.result.value;
                st.code = out.result.code;
                st.printed = out.result.code;
                ctx.repaint(); // the button of the printed code is pressed
            }
            sync();
            paintArt();
            ctx.persist();
        }
        // The typed text is read after a pause, or at once when the field is left (the pause is cut short).
        const flush = () => { clearTimeout(timer); const run = pending; pending = null; if (run) run(); };
        const later = fn => { clearTimeout(timer); pending = fn; timer = setTimeout(flush, TYPING_PAUSE_MS); };

        return {
            markup,
            mount(host) {
                root = host;
                artEl = root.querySelector('.ec-code-art-host');
                codeInput = root.querySelector('[data-input="code"]');
                valueInput = root.querySelector('[data-input="value"]');
                toleranceInput = root.querySelector('[data-input="tolerance"]');
                root.addEventListener('input', event => {
                    if (event.target === codeInput) { st.code = text64(codeInput.value); later(fromCode); }
                    else if (event.target === valueInput) { st.value = text64(valueInput.value); later(fromFields); }
                    else if (event.target === toleranceInput) { st.tolerance = text64(toleranceInput.value); later(fromFields); }
                });
                root.addEventListener('change', () => { flush(); sync(); });
            },
            // A code of the result block (3-digit, 4-digit, EIA-96) is a button: it prints that code on the chip.
            valueHTML(field) {
                if (!/^code_/.test(field.id)) return '';
                const pressed = st.printed === field.text;
                return '<button type="button" class="ec-code-print" data-print="' + esc(field.text) + '" aria-pressed="' + (pressed ? 'true' : 'false') + '" title="' + esc(t(KEY.print)) + '">' + esc(field.text) + '</button>';
            },
            handleClick(event) {
                const button = event.target.closest('[data-print]');
                if (!button) return false;
                st.printed = button.dataset.print;
                st.code = st.printed;
                sync();
                paintArt();
                ctx.repaint();
                ctx.persist();
                return true;
            },
            state() { return { inputs: { code: st.code, value: st.value, tolerance: st.tolerance }, choices: {} }; },
            restore(saved) {
                const inputs = saved && saved.inputs ? saved.inputs : {};
                st.tolerance = text64(inputs.tolerance);
                st.code = text64(inputs.code) || '472';
                fromCode();
            },
            reset() { st.tolerance = ''; st.code = '472'; fromCode(); },
            refresh() {
                if (Number.isFinite(st.valueSI) && valueInput !== valueInput.ownerDocument.activeElement) st.value = ctx.fmt(st.valueSI, 'resistance');
                sync();
            },
            layout() {},
            dispose() { clearTimeout(timer); }
        };
    }

    // ---- capacitor ---------------------------------------------------------------------------------------------------------
    function capacitorController(ctx) {
        const { item, data, units, t, codes } = ctx;
        const st = { code: '', value: '', tolerance: '', voltage: '', valueSI: NaN };
        let root = null, artEl = null, codeInput = null, valueInput = null, toleranceSelect = null, voltageSelect = null, timer = 0;
        const voltageCodes = new Set(data.voltages.map(v => v.code));

        const rowText = row => {
            if (row.unit === 'pf') return '±' + units.fmtSig(row.minus, 6) + ' pF';
            return row.minus === row.plus ? '±' + units.fmtSig(row.minus, 6) + ' %' : '−' + units.fmtSig(row.minus, 6) + ' % / +' + units.fmtSig(row.plus, 6) + ' %';
        };
        function toleranceOptions() {
            const letters = [...new Set(data.tolerances.map(row => row.letter))];
            return [{ value: '', label: t(KEY.none) }].concat(letters.map(letter => ({ value: letter, label: letter + ' · ' + ctx.comma(data.tolerances.filter(row => row.letter === letter).map(rowText).join(' / ')) })));
        }
        function voltageOptions() {
            return [{ value: '', label: t(KEY.none) }].concat(data.voltages.map(v => ({ value: String(v.volts), label: ctx.comma(units.format(v.volts, 'voltage', '', 4)) + ' · ' + v.code })));
        }
        const optionsHTML = (options, selected) => options.map(option => '<option value="' + esc(option.value) + '"' + (option.value === selected ? ' selected' : '') + '>' + esc(option.label) + '</option>').join('');

        function markup() {
            const input = (id, placeholder) => '<input class="ec-input" id="' + ctx.uid + '-' + id + '" data-input="' + id + '" type="text" inputmode="text" maxlength="' + MAX_TEXT + '" autocomplete="off" spellcheck="false" autocapitalize="off" placeholder="' + esc(placeholder) + '">';
            const select = id => selectHTML(ctx.uid, id);
            return '<div class="ec-code-stage ec-stage" data-block="stage"><div class="ec-code-art-host"></div></div>'
                + '<div class="ec-code-fields">' + ctx.field('code', t(KEY.marking), input('code', '104K 1H')) + ctx.field('value', t(DIM_KEY.capacitance), input('value', '100n'))
                + ctx.field('tolerance', t(FIELD_KEY.tolerance), select('tolerance')) + ctx.field('voltage', t(FIELD_KEY.voltage), select('voltage')) + '</div>';
        }
        // The printed marking on two lines: the value code with the tolerance letter, then the voltage code.
        function lines(code) {
            const parts = String(code).split(' ').filter(Boolean);
            const voltage = parts.filter(part => voltageCodes.has(part.toUpperCase())), rest = parts.filter(part => !voltageCodes.has(part.toUpperCase()));
            return [rest.join(''), voltage.join('')].filter(Boolean);
        }
        function paintArt() {
            const printed = lines(ctx.last().result ? ctx.last().result.code : st.code);
            setMarkup(artEl, Draw.chipSVG({ shape: 'disc', lines: printed, label: item.name + ': ' + printed.join(' '), valid: !ctx.last().error }));
        }
        function sync() {
            if (codeInput !== codeInput.ownerDocument.activeElement) codeInput.value = st.code;
            if (valueInput !== valueInput.ownerDocument.activeElement) valueInput.value = st.value;
            toleranceSelect.innerHTML = optionsHTML(toleranceOptions(), st.tolerance);
            voltageSelect.innerHTML = optionsHTML(voltageOptions(), st.voltage);
        }
        // The tolerance letter of a canonical code: a lone letter or the letter behind the value code (104K).
        function letterOf(code) {
            for (const part of String(code).split(' ')) {
                if (/^[A-Z]$/.test(part)) return part;
                const behind = /^(?:\d{3}|\d?R\d{1,2}|\d+[pnuµm]\d*)([A-Z])$/.exec(part);
                if (behind) return behind[1];
            }
            return '';
        }
        function fromCode() {
            const out = ctx.run(() => codes.decode(item, st.code, units), 'code');
            if (out.result) {
                const result = out.result, voltage = result.fields.find(field => field.id === 'voltage');
                st.valueSI = result.dim === 'capacitance' ? result.value : NaN;
                st.value = Number.isFinite(st.valueSI) ? ctx.fmt(st.valueSI, 'capacitance') : '';
                st.tolerance = letterOf(result.code);
                const volts = voltage ? voltageOptions().find(option => option.value !== '' && closeTo(Number(option.value), voltage.value)) : null;
                st.voltage = volts ? volts.value : '';
            }
            sync();
            paintArt();
            ctx.persist();
        }
        function fromFields() {
            if (!st.value.trim() && !st.tolerance && !st.voltage) { ctx.clear(); st.code = ''; sync(); paintArt(); ctx.persist(); return; }
            const out = ctx.run(() => codes.encode(item, { value: st.value, tolerance: st.tolerance, voltage: st.voltage }, units), 'value');
            if (out.result) {
                st.valueSI = out.result.dim === 'capacitance' ? out.result.value : NaN;
                st.code = out.result.code;
            }
            sync();
            paintArt();
            ctx.persist();
        }
        const later = fn => { clearTimeout(timer); timer = setTimeout(fn, TYPING_PAUSE_MS); };

        return {
            markup,
            mount(host) {
                root = host;
                artEl = root.querySelector('.ec-code-art-host');
                codeInput = root.querySelector('[data-input="code"]');
                valueInput = root.querySelector('[data-input="value"]');
                toleranceSelect = root.querySelector('[data-input="tolerance"]');
                voltageSelect = root.querySelector('[data-input="voltage"]');
                root.addEventListener('input', event => {
                    if (event.target === codeInput) { st.code = text64(codeInput.value); later(fromCode); }
                    else if (event.target === valueInput) { st.value = text64(valueInput.value); later(fromFields); }
                });
                root.addEventListener('change', event => {
                    if (event.target === toleranceSelect) { st.tolerance = toleranceSelect.value; fromFields(); }
                    else if (event.target === voltageSelect) { st.voltage = voltageSelect.value; fromFields(); }
                    else if (event.target === valueInput) { clearTimeout(timer); fromFields(); }
                    else if (event.target === codeInput) { clearTimeout(timer); fromCode(); }
                });
            },
            valueHTML() { return ''; },
            handleClick() { return false; },
            state() { return { inputs: { code: st.code, value: st.value, tolerance: st.tolerance, voltage: st.voltage }, choices: {} }; },
            restore(saved) {
                const inputs = saved && saved.inputs ? saved.inputs : {};
                st.code = text64(inputs.code) || '104K 1H';
                fromCode();
            },
            reset() { st.code = '104K 1H'; fromCode(); },
            refresh() {
                if (Number.isFinite(st.valueSI) && valueInput !== valueInput.ownerDocument.activeElement) st.value = ctx.fmt(st.valueSI, 'capacitance');
                sync();
            },
            layout() {},
            dispose() { clearTimeout(timer); }
        };
    }

    // ---- ceramic temperature codes ------------------------------------------------------------------------------------------
    // Class 2 (X7R): the letter and the digit give the lowest and the highest temperature, the last letter the change of the
    // capacitance. Class 1 (C0G): a temperature coefficient, a multiplier digit and a tolerance letter.
    function ceramicController(ctx) {
        const { item, data, units, t, codes } = ctx;
        const st = { code: '', cls: '2', low: 'X', high: '7', change: 'R', tempco: '', tol: '' };
        let root = null, artEl = null, codeInput = null, timer = 0;
        const celsius = c => (c > 0 ? '+' : c < 0 ? '−' : '') + units.fmtSig(Math.abs(c), 6) + ' °C';
        const changeText = row => (row.plus === row.minus ? '±' + units.fmtSig(row.plus, 6) + ' %' : '+' + units.fmtSig(row.plus, 6) + ' % / −' + units.fmtSig(row.minus, 6) + ' %');
        const select = (id, options) => selectHTML(ctx.uid, id, options.map(option => '<option value="' + esc(option.value) + '">' + esc(option.label) + '</option>').join(''));
        const lowOptions = data.class2.low.map(row => ({ value: row.letter, label: row.letter + ' · ' + celsius(row.c) }));
        const highOptions = data.class2.high.map(row => ({ value: row.digit, label: row.digit + ' · ' + celsius(row.c) }));
        const changeOptions = data.class2.change.map(row => ({ value: row.letter, label: row.letter + ' · ' + ctx.comma(changeText(row)) }));
        const tolOptions = data.class1.tol.map(row => ({ value: String(row.ppm), label: row.letter + ' · ±' + ctx.comma(units.fmtSig(row.ppm, 6) + ' ppm/K'), letter: row.letter }));
        // The coefficients that class 1 codes can express, for the suggestions of the input.
        const coefficients = [...new Set(data.class1.sig.flatMap(sig => data.class1.mult.map(mult => sig.value * mult.value)))].sort((a, b) => a - b);

        function markup() {
            const input = (id, placeholder, extra) => '<input class="ec-input" id="' + ctx.uid + '-' + id + '" data-input="' + id + '" type="text" inputmode="text" maxlength="' + MAX_TEXT + '" autocomplete="off" spellcheck="false" autocapitalize="off" placeholder="' + esc(placeholder) + '"' + (extra || '') + '>';
            return '<div class="ec-code-stage ec-stage" data-block="stage"><div class="ec-code-art-host"></div></div>'
                + '<div class="ec-code-fields"><fieldset class="ec-choice" data-choice="class"><legend>' + esc(t(FIELD_KEY.class)) + '</legend><div class="ec-seg">'
                + '<label><input type="radio" name="' + ctx.uid + '-class" value="1"><span>' + esc(t(KEY.class1)) + '</span></label>'
                + '<label><input type="radio" name="' + ctx.uid + '-class" value="2"><span>' + esc(t(KEY.class2)) + '</span></label></div></fieldset>'
                + ctx.field('code', t(KEY.marking), input('code', 'X7R'))
                + '<div class="ec-code-class" data-class="2">' + ctx.field('low', t(FIELD_KEY.temp_low), select('low', lowOptions)) + ctx.field('high', t(FIELD_KEY.temp_high), select('high', highOptions))
                + ctx.field('change', t(FIELD_KEY.change), select('change', changeOptions)) + '</div>'
                + '<div class="ec-code-class" data-class="1">' + ctx.field('tempco', t(FIELD_KEY.tc), input('tempco', '-750 ppm', ' list="' + ctx.uid + '-tempco-list"') + '<datalist id="' + ctx.uid + '-tempco-list">'
                    + coefficients.map(c => '<option value="' + esc(String(c) + ' ppm') + '"></option>').join('') + '</datalist>')
                + ctx.field('tol', t(FIELD_KEY.tolerance), select('tol', tolOptions)) + '</div></div>'
                + '<div class="ec-code-common"><span class="ec-code-common-title">' + esc(t(KEY.common)) + '</span>'
                + data.common.map(code => '<button type="button" class="ec-chip-btn" data-pick="' + esc(code) + '">' + esc(code) + '</button>').join('') + '</div>';
        }
        const control = id => root.querySelector('[data-input="' + id + '"]');
        function sync() {
            if (codeInput !== codeInput.ownerDocument.activeElement) codeInput.value = st.code;
            for (const radio of root.querySelectorAll('input[name="' + ctx.uid + '-class"]')) radio.checked = radio.value === st.cls;
            for (const panel of root.querySelectorAll('.ec-code-class')) panel.hidden = panel.dataset.class !== st.cls;
            control('low').value = st.low;
            control('high').value = st.high;
            control('change').value = st.change;
            control('tol').value = st.tol;
            if (control('tempco') !== control('tempco').ownerDocument.activeElement) control('tempco').value = st.tempco;
        }
        function paintArt() {
            const shown = ctx.last().result ? ctx.last().result.code : st.code.toUpperCase();
            setMarkup(artEl, Draw.chipSVG({ shape: 'disc', lines: [shown], label: item.name + ': ' + shown, valid: !ctx.last().error }));
        }
        // A decoded code sets the fields: the characters of the canonical code are the letters and digits of the selects.
        function fromCode() {
            const out = ctx.run(() => codes.decode(item, st.code, units), 'code');
            if (out.result) {
                const canon = Array.from(out.result.code), cls = out.result.fields.find(field => field.id === 'class');
                st.cls = cls && cls.text === '1' ? '1' : '2';
                if (st.cls === '2') {
                    st.low = canon[0];
                    st.high = canon[1];
                    st.change = canon[2];
                } else {
                    const tc = out.result.fields.find(field => field.id === 'tc'), row = data.class1.tol.find(entry => entry.letter === canon[2]);
                    st.tempco = tc ? String(Number((tc.value / 1e-6).toPrecision(8))) + ' ppm' : st.tempco;
                    st.tol = row ? String(row.ppm) : st.tol;
                }
            }
            sync();
            paintArt();
            ctx.persist();
        }
        function fromFields() {
            let request;
            if (st.cls === '2') {
                const low = data.class2.low.find(row => row.letter === st.low) || data.class2.low[0], high = data.class2.high.find(row => row.digit === st.high) || data.class2.high[0];
                const change = data.class2.change.find(row => row.letter === st.change) || data.class2.change[0];
                const tolerance = change.plus === change.minus ? change.plus + '%' : '+' + change.plus + '/-' + change.minus;
                request = { class: '2', value: low.c + '..' + high.c, tolerance };
            } else {
                request = { class: '1', tempco: st.tempco, tolerance: st.tol + 'ppm' };
            }
            const out = ctx.run(() => codes.encode(item, request, units), 'fields');
            if (out.result) st.code = out.result.code;
            sync();
            paintArt();
            ctx.persist();
        }
        const later = fn => { clearTimeout(timer); timer = setTimeout(fn, TYPING_PAUSE_MS); };

        return {
            markup,
            mount(host) {
                root = host;
                artEl = root.querySelector('.ec-code-art-host');
                codeInput = control('code');
                root.addEventListener('input', event => {
                    if (event.target === codeInput) { st.code = text64(codeInput.value); later(fromCode); }
                    else if (event.target === control('tempco')) { st.tempco = text64(event.target.value); later(fromFields); }
                });
                root.addEventListener('change', event => {
                    const target = event.target;
                    if (target === codeInput) { clearTimeout(timer); fromCode(); }
                    else if (target.name === ctx.uid + '-class') { st.cls = target.value; fromFields(); }
                    else if (target === control('low')) { st.low = target.value; fromFields(); }
                    else if (target === control('high')) { st.high = target.value; fromFields(); }
                    else if (target === control('change')) { st.change = target.value; fromFields(); }
                    else if (target === control('tol')) { st.tol = target.value; fromFields(); }
                    else if (target === control('tempco')) { clearTimeout(timer); fromFields(); }
                });
            },
            valueHTML() { return ''; },
            handleClick(event) {
                const pick = event.target.closest('[data-pick]');
                if (!pick) return false;
                st.code = pick.dataset.pick;
                fromCode();
                return true;
            },
            state() { return { inputs: { code: st.code, tempco: st.tempco, low: st.low, high: st.high, change: st.change, tol: st.tol }, choices: { class: st.cls } }; },
            restore(saved) {
                const inputs = saved && saved.inputs ? saved.inputs : {};
                st.tempco = text64(inputs.tempco) || '-750 ppm';
                st.tol = tolOptions.some(option => option.value === inputs.tol) ? inputs.tol : tolOptions[0].value;
                st.code = text64(inputs.code) || 'X7R';
                fromCode();
            },
            reset() { st.tempco = '-750 ppm'; st.tol = tolOptions[0].value; st.code = 'X7R'; fromCode(); },
            refresh() { sync(); },
            layout() {},
            dispose() { clearTimeout(timer); }
        };
    }

    const CONTROLLERS = { resistor_bands: resistorController, inductor_bands: inductorController, smd_resistor: smdController, capacitor: capacitorController, ceramic_temp: ceramicController };

    // ---- the page ----------------------------------------------------------------------------------------------------------------
    function create(options) {
        const { item, units, store, t, icon, uid } = options;
        const data = item.data || {};
        const comma = text => (options.decimalComma ? String(text).replace(/(\d)\.(\d)/g, '$1,$2') : String(text));
        const sig = () => store.prefs().sig;
        const fmt = (value, dim) => (Number.isFinite(value) ? comma(units.format(value, dim, '', sig())) : '');
        const make = CONTROLLERS[item.code];
        const el = document.createElement('article');
        el.className = 'ec-page ec-code-page';
        el.dataset.code = item.id;
        el.dataset.kind = String(item.code);
        el.dataset.narrow = 'false';
        el.setAttribute('aria-labelledby', uid + '-title');
        const favoriteLabel = () => (store.isFavorite(item.id) ? t('desktop.ecalc_favorite_remove') : t('desktop.ecalc_favorite_add'));
        const paragraphs = text => String(text || '').split(/\n{2,}/).filter(part => part.trim()).map(part => '<p>' + esc(part.trim()) + '</p>').join('');
        const head = '<header class="ec-page-head"><h2 class="ec-title" id="' + uid + '-title">' + esc(item.name) + '</h2><div class="ec-actions">'
            + '<button type="button" class="ec-btn" data-action="favorite" aria-pressed="false">' + icon('star') + '<span class="ec-btn-label" data-label="favorite"></span></button>'
            + '<button type="button" class="ec-btn" data-action="copy" title="' + esc(t('desktop.ecalc_copy_results')) + '" aria-label="' + esc(t('desktop.ecalc_copy_results')) + '">' + icon('copy') + '<span class="ec-btn-label" data-label="copy">' + esc(t('desktop.ecalc_copy_results')) + '</span></button>'
            + '<button type="button" class="ec-btn" data-action="reset" title="' + esc(t('desktop.ecalc_reset')) + '" aria-label="' + esc(t('desktop.ecalc_reset')) + '">' + icon('reset') + '<span class="ec-btn-label">' + esc(t('desktop.ecalc_reset')) + '</span></button></div></header>';
        const foot = (Array.isArray(data.notes) && data.notes.length ? '<section class="ec-code-notes"><h3>' + esc(t(KEY.notes)) + '</h3><ul>' + data.notes.map(note => '<li>' + esc(note) + '</li>').join('') + '</ul></section>' : '')
            + (item.doc ? '<details class="ec-doc"><summary>' + esc(t('desktop.ecalc_explanation')) + '</summary><div class="ec-doc-body">' + paragraphs(item.doc) + '</div></details>' : '')
            + '<p class="ec-source"><strong>' + esc(t('desktop.ecalc_source')) + ':</strong> ' + esc(item.source || '') + '</p>';
        // An item the page cannot use (a code kind without a controller, data that lacks what its controller reads) says so.
        const unavailable = () => {
            el.innerHTML = head + '<p class="ec-empty-page">' + esc(t('desktop.ecalc_page_unavailable')) + '</p>' + foot;
            return { el, dispose() {} };
        };
        if (!make || !core.codes || !Draw) return unavailable();

        let last = { result: null, error: null, source: '' };
        let ctrl = null, copyTimer = 0;
        const colorById = new Map((data.colors || []).map(color => [color.id, color]));

        // A labelled field: the same chip as the fields of a calculator (label, control, message).
        function field(id, label, control) {
            return '<div class="ec-field ec-code-field" data-field="' + id + '" data-state="input"><div class="ec-field-head"><label class="ec-field-label" for="' + uid + '-' + id + '" title="' + esc(label) + '">' + esc(label) + '</label></div>'
                + '<div class="ec-field-row">' + control + '</div></div>';
        }
        const ctx = {
            item, data, units, t, codes: core.codes, uid, comma, fmt, field,
            last: () => last,
            run(fn, source) {
                try { last = { result: fn(), error: null, source }; } catch (error) { last = { result: null, error: describe(error), source }; }
                paintOutcome();
                return last;
            },
            clear() { last = { result: null, error: null, source: '' }; paintOutcome(); },
            repaint() { paintOutcome(); },
            persist() { const state = ctrl.state(); store.saveCalc(item.id, { inputs: state.inputs, choices: state.choices }); }
        };
        let body = '';
        try {
            ctrl = make(ctx);
            body = ctrl.markup();
        } catch (error) {
            return unavailable();
        }

        el.innerHTML = head + '<div class="ec-code-main" data-kind="' + esc(item.code) + '">' + body + '</div>'
            + '<section class="ec-block ec-code-result" data-block="result"><h3>' + esc(t(KEY.result)) + '</h3><p class="ec-code-error" id="' + uid + '-error" role="alert" hidden></p>'
            + '<div class="ec-code-reading" aria-live="polite"></div><ul class="ec-warnings" aria-label="' + esc(t('desktop.ecalc_warnings')) + '"></ul></section>' + foot;
        const main = el.querySelector('.ec-code-main'), errorEl = el.querySelector('.ec-code-error'), readingEl = el.querySelector('.ec-code-reading'), warningsEl = el.querySelector('.ec-warnings');

        // The rows of a result: the colours or the marking, the value, then the decoded fields with their names translated.
        function rows(result) {
            const out = [];
            if (result.bands.length) {
                out.push({ name: t(KEY.colors), text: result.bands.map(id => (colorById.get(id) || { name: id }).name).join(', '),
                    html: result.bands.map(id => { const color = colorById.get(id); return '<span class="ec-code-colour">' + swatch(color ? color.hex : '') + esc(color ? color.name : id) + '</span>'; }).join(' ') });
            } else if (result.code) {
                out.push({ name: t(KEY.marking), text: result.code, html: '<code class="ec-code-text">' + esc(result.code) + '</code>' });
            }
            const repeated = result.fields.some(entry => entry.dim === result.dim && entry.value === result.value);
            if (Number.isFinite(result.value) && DIM_KEY[result.dim] && !repeated) {
                const text = fmt(result.value, result.dim);
                out.push({ name: t(DIM_KEY[result.dim]), text, html: '<strong class="ec-code-value">' + esc(text) + '</strong>' });
            }
            for (const entry of result.fields) {
                const text = comma(entry.text), special = ctrl.valueHTML(entry);
                out.push({ name: FIELD_KEY[entry.id] ? t(FIELD_KEY[entry.id]) : entry.name, text, html: special || esc(text) });
            }
            return out;
        }
        function paintOutcome() {
            const { result, error, source } = last;
            errorEl.hidden = !error;
            if (error) errorEl.innerHTML = '<strong>' + esc(t(ERROR_KEY[error.code])) + '</strong> ' + esc(error.message);
            errorEl.dataset.code = error ? error.code : '';
            for (const holder of main.querySelectorAll('.ec-code-field')) {
                const broken = !!error && holder.dataset.field === source;
                holder.dataset.state = broken ? 'error' : 'input';
                for (const control of holder.querySelectorAll('input, select')) {
                    if (broken) { control.setAttribute('aria-invalid', 'true'); control.setAttribute('aria-describedby', uid + '-error'); } else { control.removeAttribute('aria-invalid'); control.removeAttribute('aria-describedby'); }
                }
            }
            readingEl.hidden = !!error;
            setMarkup(readingEl, result
                ? '<dl class="ec-code-rows">' + rows(result).map(row => '<div class="ec-code-row"><dt>' + esc(row.name) + '</dt><dd>' + row.html + '</dd></div>').join('') + '</dl>'
                : (error ? '' : '<p class="ec-code-empty">' + esc(t(KEY.empty)) + '</p>'));
            setMarkup(warningsEl, result ? result.warnings.map(text => '<li>' + icon('warning') + '<span>' + esc(text) + '</span></li>').join('') : '');
            el.dataset.state = error ? 'error' : result ? 'ok' : 'empty';
            el.querySelector('[data-action="copy"]').disabled = !result;
        }
        function plainText() {
            if (!last.result) return '';
            return [item.name].concat(rows(last.result).map(row => row.name + ': ' + row.text), last.result.warnings).join('\n');
        }
        function paintActions() {
            const button = el.querySelector('[data-action="favorite"]'), on = store.isFavorite(item.id);
            button.setAttribute('aria-pressed', on ? 'true' : 'false');
            button.querySelector('[data-label]').textContent = favoriteLabel();
            button.title = favoriteLabel();
            button.setAttribute('aria-label', favoriteLabel());
        }
        const copy = text => {
            const Page = window.ECalcCalcPage;
            if (Page && typeof Page.copyText === 'function') Page.copyText(text); else if (navigator.clipboard) navigator.clipboard.writeText(text).catch(() => {});
        };

        try {
            ctrl.mount(main);
            ctrl.restore(store.calcState(item.id));
        } catch (error) {
            ctrl.dispose();
            return unavailable();
        }
        paintActions();

        el.addEventListener('click', event => {
            if (ctrl.handleClick(event)) return;
            const button = event.target.closest('[data-action]');
            if (!button || !el.contains(button)) return;
            if (button.dataset.action === 'favorite') {
                store.toggleFavorite(item.id);
                paintActions();
                if (options.onFavorite) options.onFavorite(item.id);
            } else if (button.dataset.action === 'reset') {
                ctrl.reset();
                ctx.persist();
            } else if (button.dataset.action === 'copy') {
                copy(plainText());
                const label = button.querySelector('[data-label]');
                button.dataset.done = 'true';
                label.textContent = t('desktop.ecalc_copied');
                button.title = t('desktop.ecalc_copied');
                clearTimeout(copyTimer);
                copyTimer = setTimeout(() => {
                    delete button.dataset.done;
                    label.textContent = t('desktop.ecalc_copy_results');
                    button.title = t('desktop.ecalc_copy_results');
                }, COPIED_MS);
            }
        });

        return {
            el,
            layout() {
                const width = el.clientWidth;
                if (!width) return;
                el.dataset.narrow = width < NARROW_BELOW ? 'true' : 'false';
                ctrl.layout();
            },
            // The settings changed (digits): the numbers are written again.
            refresh() {
                paintActions();
                ctrl.refresh();
                paintOutcome();
            },
            dispose() {
                clearTimeout(copyTimer);
                ctrl.dispose();
                store.flush();
            }
        };
    }

    window.ECalcCodePage = { create };
})();
