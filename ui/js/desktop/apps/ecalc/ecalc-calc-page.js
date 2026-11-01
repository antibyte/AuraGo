/* ECalc calculator page (layout C): the schematic with the input fields placed on it, the typeset formula with its live
   line, the nearest E-series values, the derived quantities, warnings, the collapsible explanation with its source and
   the actions (favorite, copy results, reset). A field is an input (the user's value), computed (success tint), in
   error (danger tint with a message) and, for inputs, lockable. Typing parses after a short pause; a text that does not
   parse changes nothing. The state that decides which variables are inputs lives in ECalcCore.solve (see there);
   this file only draws it and feeds the typed texts into it. Publishes window.ECalcCalcPage. */
(function () {
    'use strict';

    const core = window.ECalcCore, Typeset = window.ECalcTypeset, Symbols = window.ECalcSymbols;
    const S = core.solve, esc = Typeset.escape;
    const has = (object, key) => Object.prototype.hasOwnProperty.call(object, key);
    const TYPING_PAUSE_MS = 60;
    const FORMULA_COLUMN = 324; // the formula column of the split layout plus its gap
    const STAGE_INSET = 26; // padding and border of the stage around the drawing
    const NARROW_BELOW = 760; // under this width the action buttons show their icon only

    // options: {calc, units, store, t, icon, uid, decimalComma, schematics, onFavorite}
    function create(options) {
        const { calc, units, store, t, icon, uid } = options;
        const varById = new Map(calc.vars.map(v => [v.id, v]));
        const derivedById = new Map(calc.derived.map(item => [item.id, item]));
        const spec = calc.schematic && options.schematics && options.schematics[calc.schematic] ? options.schematics[calc.schematic] : null;
        const anchors = new Map();
        if (spec) for (const anchor of spec.anchors || []) if (varById.has(anchor.var)) anchors.set(anchor.var, anchor);
        const formulas = calc.formula.map(line => core.expr.parseFormula(line));

        // The unit of a field: the stored choice when this catalog still offers it ('auto' is the automatic prefix),
        // else the default unit of the variable.
        const unitOf = id => {
            const v = varById.get(id), stored = store.unitOf(calc.id, id);
            const choice = stored === 'auto' ? '' : stored;
            if (v && stored !== undefined && units.options(v.dim).some(option => option.id === choice)) return choice;
            return v && v.unit ? v.unit : '';
        };
        const sig = () => store.prefs().sig;
        const comma = text => (options.decimalComma ? text.replace(/(\d)\.(\d)/g, '$1,$2') : text);
        // A unit that cannot show a finite value (dBV for a negative voltage) gives way to the automatic prefix.
        const formatted = (entry, x, unitId) => units.format(x, entry.dim, unitId, sig()) || units.format(x, entry.dim, '', sig());
        // Lists keep the decimal point (the comma separates their items), single values follow the language.
        const display = (entry, value, unitId) => (Array.isArray(value)
            ? value.map(x => formatted(entry, x, unitId).replace(/\s+/g, '')).join(', ')
            : comma(formatted(entry, value, unitId)));
        const finite = value => (Array.isArray(value) ? value.length > 0 && value.every(Number.isFinite) : Number.isFinite(value));

        let state = S.restoreState(calc, store.calcState(calc.id), units, unitOf);
        let result = null, parseErrors = {}, typing = {};
        const timers = new Map(), fields = new Map();
        let copyTimer = 0;

        function errorText(code) {
            switch (code) {
                case 'parse': return t('desktop.ecalc_error_parse');
                case 'nonfinite': return t('desktop.ecalc_error_nonfinite');
                case 'range': return t('desktop.ecalc_error_range');
                case 'undetermined': return t('desktop.ecalc_error_undetermined');
                case 'locked': return t('desktop.ecalc_error_locked');
                default: return '';
            }
        }
        const persist = () => store.saveCalc(calc.id, state);

        // ---- markup --------------------------------------------------------------------------------------------

        function fieldMarkup(v) {
            const id = uid + '-' + v.id;
            const choices = units.options(v.dim);
            let unit = '';
            if (choices.length > 1) {
                unit = '<span class="ec-unit-wrap"><select class="ec-unit" aria-label="' + esc(t('desktop.ecalc_dim_' + v.dim)) + '" title="' + esc(t('desktop.ecalc_dim_' + v.dim)) + '">'
                    + choices.map(option => '<option value="' + esc(option.id) + '">' + esc(option.kind === 'auto' ? t('desktop.ecalc_unit_auto') : option.label) + '</option>').join('') + '</select></span>';
            } else {
                const symbol = units.dim(v.dim).symbol;
                if (symbol) unit = '<span class="ec-unit-static">' + esc(symbol) + '</span>';
            }
            return '<div class="ec-field" role="group" data-var="' + esc(v.id) + '" data-state="input" data-dim="' + esc(v.dim) + '"' + (v.list ? ' data-list="true"' : '') + ' aria-labelledby="' + id + '-label">'
                + '<div class="ec-field-head"><label class="ec-field-label" id="' + id + '-label" for="' + id + '" title="' + esc(v.name || '') + '">' + Typeset.labelHTML(v.label || v.id) + '</label>'
                + '<button type="button" class="ec-lock" aria-pressed="false" data-action="lock">' + icon('lock') + '</button></div>'
                + '<div class="ec-field-row"><input class="ec-input" id="' + id + '" type="text" inputmode="decimal" maxlength="' + units.maxLength(!!v.list) + '" autocomplete="off" spellcheck="false" autocapitalize="off" aria-describedby="' + id + '-msg">' + unit + '</div>'
                + '<div class="ec-field-msg" id="' + id + '-msg" aria-live="polite"></div></div>';
        }

        const choiceMarkup = choice => '<fieldset class="ec-choice" data-choice="' + esc(choice.id) + '"><legend>' + esc(choice.name) + '</legend><div class="ec-seg">'
            + choice.options.map(option => '<label><input type="radio" name="' + uid + '-' + esc(choice.id) + '" value="' + esc(option.id) + '"><span>' + esc(option.name) + '</span></label>').join('') + '</div></fieldset>';

        const docMarkup = String(calc.doc || '').split(/\n{2,}/).filter(part => part.trim()).map(part => '<p>' + esc(part.trim()) + '</p>').join('');
        const favoriteLabel = () => (store.isFavorite(calc.id) ? t('desktop.ecalc_favorite_remove') : t('desktop.ecalc_favorite_add'));

        const el = document.createElement('article');
        el.className = 'ec-page';
        el.dataset.calc = calc.id;
        el.dataset.layout = 'stack';
        el.dataset.chips = spec ? 'listed' : 'none';
        el.setAttribute('aria-labelledby', uid + '-title');
        el.innerHTML = '<header class="ec-page-head"><h2 class="ec-title" id="' + uid + '-title">' + esc(calc.name) + '</h2><div class="ec-actions">'
            + '<button type="button" class="ec-btn" data-action="favorite" aria-pressed="false">' + icon('star') + '<span class="ec-btn-label" data-label="favorite"></span></button>'
            + '<button type="button" class="ec-btn" data-action="copy" title="' + esc(t('desktop.ecalc_copy_results')) + '" aria-label="' + esc(t('desktop.ecalc_copy_results')) + '">' + icon('copy') + '<span class="ec-btn-label" data-label="copy">' + esc(t('desktop.ecalc_copy_results')) + '</span></button>'
            + '<button type="button" class="ec-btn" data-action="reset" title="' + esc(t('desktop.ecalc_reset')) + '" aria-label="' + esc(t('desktop.ecalc_reset')) + '">' + icon('reset') + '<span class="ec-btn-label">' + esc(t('desktop.ecalc_reset')) + '</span></button></div></header>'
            + ((calc.choices || []).length ? '<div class="ec-choices">' + calc.choices.map(choiceMarkup).join('') + '</div>' : '')
            + '<div class="ec-main"><section class="ec-stage" aria-label="' + esc(t('desktop.ecalc_inputs')) + '">'
            + (spec ? '<div class="ec-sch-wrap" style="--ec-w:' + spec.w + ';--ec-ar:' + spec.w + '/' + spec.h + '"><div class="ec-sch-art"></div><div class="ec-chips"></div></div>' : '')
            + '<div class="ec-fields"></div></section>'
            + '<section class="ec-math-card"><div class="ec-formulas">' + formulas.map((_, index) => '<div class="ec-formula" data-line="' + index + '"><div class="ec-math" data-math="formula"></div><div class="ec-math ec-live" data-math="live"></div></div>').join('') + '</div></section></div>'
            + '<div class="ec-below"><section class="ec-block" data-block="eseries"><h3 data-label="eseries"></h3><ul class="ec-rows" data-rows="eseries"></ul></section>'
            + '<section class="ec-block" data-block="derived"><h3>' + esc(t('desktop.ecalc_derived')) + '</h3><ul class="ec-rows" data-rows="derived"></ul></section></div>'
            + '<ul class="ec-warnings" aria-label="' + esc(t('desktop.ecalc_warnings')) + '"></ul>'
            + '<details class="ec-doc"><summary>' + esc(t('desktop.ecalc_explanation')) + '</summary><div class="ec-doc-body">' + docMarkup + '<p class="ec-source"><strong>' + esc(t('desktop.ecalc_source')) + ':</strong> ' + esc(calc.source || '') + '</p></div></details>';

        const chipsEl = el.querySelector('.ec-chips'), fieldsEl = el.querySelector('.ec-fields'), artEl = el.querySelector('.ec-sch-art');
        for (const v of calc.vars) {
            const holder = document.createElement('div');
            holder.innerHTML = fieldMarkup(v);
            const root = holder.firstElementChild, anchor = anchors.get(v.id);
            if (anchor && chipsEl) {
                root.dataset.side = anchor.side || 'right';
                root.style.setProperty('--ax', String(Math.round(anchor.x / spec.w * 10000) / 100));
                root.style.setProperty('--ay', String(Math.round(anchor.y / spec.h * 10000) / 100));
                chipsEl.appendChild(root);
            } else {
                fieldsEl.appendChild(root);
            }
            const refs = { v, root, input: root.querySelector('.ec-input'), select: root.querySelector('.ec-unit'), lock: root.querySelector('.ec-lock'), msg: root.querySelector('.ec-field-msg') };
            if (refs.select) refs.select.value = unitOf(v.id);
            fields.set(v.id, refs);
        }
        if (fieldsEl && !fieldsEl.children.length) fieldsEl.remove();
        if (spec && !chipsEl.children.length) chipsEl.remove();
        function drawSchematic() {
            if (spec && artEl) artEl.innerHTML = Symbols.svg(spec, { standard: store.prefs().symbols, label: calc.name });
        }
        drawSchematic();

        // ---- painting ------------------------------------------------------------------------------------------

        function compute() {
            const read = S.readInputs(calc, state, units, unitOf);
            parseErrors = read.errors;
            result = S.solve(calc, read.values, state.choices, units);
        }

        function paintField(refs, keep) {
            const v = refs.v, isInput = has(state.inputs, v.id) && v.input !== false, locked = state.locked.includes(v.id);
            const value = result.values[v.id];
            const code = typing[v.id] || parseErrors[v.id] || result.errors[v.id] || '';
            refs.root.dataset.state = code === 'undetermined' ? 'pending' : code ? 'error' : isInput ? 'input' : 'computed';
            refs.root.dataset.locked = locked ? 'true' : 'false';
            refs.lock.hidden = !isInput;
            refs.lock.setAttribute('aria-pressed', locked ? 'true' : 'false');
            const lockLabel = locked ? t('desktop.ecalc_unlock') : t('desktop.ecalc_lock');
            refs.lock.setAttribute('aria-label', lockLabel);
            refs.lock.title = lockLabel;
            refs.input.readOnly = v.input === false;
            refs.input.setAttribute('aria-invalid', code && code !== 'undetermined' ? 'true' : 'false');
            refs.msg.textContent = errorText(code);
            const focused = refs.input.ownerDocument.activeElement === refs.input;
            if (focused && keep) return;
            let text;
            if (focused) text = isInput ? state.inputs[v.id] : finite(value) ? display(v, value, unitOf(v.id)) : '';
            else if (isInput && !parseErrors[v.id]) text = display(v, value, unitOf(v.id));
            else if (isInput) text = state.inputs[v.id];
            else text = finite(value) ? display(v, value, unitOf(v.id)) : '';
            if (refs.input.value !== text) refs.input.value = text;
        }

        const labelOf = name => {
            const entry = varById.get(name) || derivedById.get(name);
            return entry ? entry.label || name : '';
        };
        function typesetContext(live) {
            const consts = S.choiceConsts(calc, state.choices);
            const ctx = { labelOf, constOf: name => (has(consts, name) ? comma(units.fmtSig(consts[name], 4)) : undefined) };
            if (live) {
                ctx.valueOf = name => {
                    const entry = varById.get(name) || derivedById.get(name);
                    const value = entry && result ? result.values[name] : NaN;
                    if (!entry || !finite(value)) return null;
                    const text = display(entry, value, derivedById.has(name) ? entry.unit || '' : unitOf(name));
                    return { html: esc(text), prec: text.startsWith('-') ? 2 : /\s/.test(text) ? 3 : 6 };
                };
            }
            return ctx;
        }
        function paintFormulas(withFormula) {
            const lines = el.querySelectorAll('.ec-formula');
            formulas.forEach((ast, index) => {
                if (withFormula) lines[index].querySelector('[data-math="formula"]').innerHTML = Typeset.formulaHTML(ast, typesetContext(false));
                lines[index].querySelector('[data-math="live"]').innerHTML = Typeset.liveHTML(ast, typesetContext(true));
            });
        }

        function percent(dev) {
            const size = Math.abs(dev);
            if (size < 0.05) return comma('0.0 %');
            return comma((dev > 0 ? '+' : '−') + (size < 10 ? size.toFixed(1) : size.toFixed(0)) + ' %');
        }
        function paintResults() {
            const series = store.prefs().eseries;
            const eRows = calc.vars.filter(v => v.eseries).map(v => {
                const value = result.values[v.id], near = Number.isFinite(value) && value > 0 && !Array.isArray(value) ? core.eseries.nearest(series, value) : null;
                if (!near) return '';
                const tone = Math.abs(near.dev) < 1 ? 'ok' : Math.abs(near.dev) >= 5 ? 'warn' : 'mid';
                return '<li><span class="ec-row-name">' + Typeset.labelHTML(v.label || v.id) + '</span><span class="ec-row-val">' + esc(display(v, near.value, unitOf(v.id))) + '</span><span class="ec-row-dev" data-tone="' + tone + '">' + esc(percent(near.dev)) + '</span></li>';
            }).join('');
            el.querySelector('[data-block="eseries"]').hidden = !eRows;
            el.querySelector('[data-rows="eseries"]').innerHTML = eRows;
            el.querySelector('[data-label="eseries"]').textContent = t('desktop.ecalc_eseries_nearest', { series });
            const dRows = calc.derived.map(item => {
                const value = result.values[item.id];
                return '<li><span class="ec-row-name" title="' + esc(item.name || '') + '">' + Typeset.labelHTML(item.label || item.id) + '</span><span class="ec-row-val">'
                    + (finite(value) ? esc(display(item, value, item.unit || '')) : '–') + '</span></li>';
            }).join('');
            el.querySelector('[data-block="derived"]').hidden = !dRows;
            el.querySelector('[data-rows="derived"]').innerHTML = dRows;
            el.querySelector('.ec-warnings').innerHTML = result.warnings.map(index => '<li>' + icon('warning') + '<span>' + esc(calc.checks[index].warn) + '</span></li>').join('');
        }

        function paintChoices() {
            for (const choice of calc.choices || []) {
                for (const radio of el.querySelectorAll('[data-choice="' + choice.id + '"] input')) radio.checked = radio.value === state.choices[choice.id];
            }
        }
        function paintActions() {
            const favorite = store.isFavorite(calc.id), button = el.querySelector('[data-action="favorite"]');
            button.setAttribute('aria-pressed', favorite ? 'true' : 'false');
            button.querySelector('[data-label]').textContent = favoriteLabel();
            button.title = favoriteLabel();
            button.setAttribute('aria-label', favoriteLabel());
        }

        function paint(editing) {
            compute();
            for (const refs of fields.values()) paintField(refs, refs.v.id === editing);
            paintFormulas(false);
            paintResults();
        }

        // ---- events --------------------------------------------------------------------------------------------

        function readText(v, text, unitId) {
            return v.list ? units.parseList(text, v.dim, unitId) : units.parse(text, v.dim, unitId);
        }
        function commitText(refs) {
            const v = refs.v, text = refs.input.value;
            clearTimeout(timers.get(v.id));
            timers.delete(v.id);
            try {
                readText(v, text, unitOf(v.id));
            } catch (error) {
                typing[v.id] = 'parse';
                paintField(refs, true);
                return;
            }
            delete typing[v.id];
            if (has(state.inputs, v.id) && state.inputs[v.id] === text) { paintField(refs, true); return; }
            const next = S.applyEdit(calc, state, v.id, text, units, unitOf, result.values);
            // A locked field stays an input: when it would have to give way the edit is refused and says so.
            if (next === state) { typing[v.id] = 'locked'; paintField(refs, true); return; }
            state = next;
            persist();
            paint(v.id);
        }

        el.addEventListener('input', event => {
            const refs = event.target.classList.contains('ec-input') ? fields.get(event.target.closest('.ec-field').dataset.var) : null;
            if (!refs || refs.v.input === false) return;
            clearTimeout(timers.get(refs.v.id));
            timers.set(refs.v.id, setTimeout(() => commitText(refs), TYPING_PAUSE_MS));
        });
        el.addEventListener('focusin', event => {
            if (!event.target.classList.contains('ec-input')) return;
            const refs = fields.get(event.target.closest('.ec-field').dataset.var);
            if (has(state.inputs, refs.v.id) && !typing[refs.v.id]) refs.input.value = state.inputs[refs.v.id];
            refs.input.select();
        });
        el.addEventListener('focusout', event => {
            if (!event.target.classList.contains('ec-input')) return;
            const refs = fields.get(event.target.closest('.ec-field').dataset.var);
            if (timers.has(refs.v.id)) commitText(refs);
            delete typing[refs.v.id];
            paintField(refs, false);
        });
        el.addEventListener('keydown', event => {
            if (event.key === 'Enter' && event.target.classList.contains('ec-input')) event.target.blur();
        });
        el.addEventListener('change', event => {
            const target = event.target;
            if (target.classList.contains('ec-unit')) {
                const refs = fields.get(target.closest('.ec-field').dataset.var), v = refs.v;
                let exact = null;
                if (has(state.inputs, v.id)) {
                    try { exact = readText(v, state.inputs[v.id], unitOf(v.id)); } catch (error) { exact = null; }
                }
                store.setUnit(calc.id, v.id, target.value || 'auto');
                if (exact !== null) state = Object.assign({}, state, { inputs: Object.assign({}, state.inputs, { [v.id]: S.exactText(v, exact, units, unitOf) }) });
                persist();
                paint();
            } else if (target.type === 'radio' && target.closest('.ec-choice')) {
                const id = target.closest('.ec-choice').dataset.choice;
                state = Object.assign({}, state, { choices: Object.assign({}, state.choices, { [id]: target.value }) });
                persist();
                paintFormulas(true);
                paint();
            }
        });

        function plainText() {
            const lines = [calc.name];
            for (const v of calc.vars) {
                const value = result.values[v.id];
                if (finite(value)) lines.push((v.label || v.id) + ' = ' + display(v, value, unitOf(v.id)));
            }
            for (const item of calc.derived) {
                const value = result.values[item.id];
                if (finite(value)) lines.push((item.label || item.id) + ' = ' + display(item, value, item.unit || ''));
            }
            return lines.join('\n');
        }
        function copyText(text) {
            const fallback = () => {
                const area = document.createElement('textarea');
                area.value = text;
                area.setAttribute('readonly', '');
                area.style.position = 'fixed';
                area.style.opacity = '0';
                document.body.appendChild(area);
                area.select();
                try { document.execCommand('copy'); } catch (error) { /* nothing more to try */ }
                area.remove();
            };
            if (navigator.clipboard && navigator.clipboard.writeText) navigator.clipboard.writeText(text).catch(fallback);
            else fallback();
        }

        el.addEventListener('click', event => {
            const button = event.target.closest('[data-action]');
            if (!button || !el.contains(button)) return;
            const action = button.dataset.action;
            if (action === 'lock') {
                state = S.toggleLock(state, button.closest('.ec-field').dataset.var);
                persist();
                paint();
            } else if (action === 'favorite') {
                store.toggleFavorite(calc.id);
                paintActions();
                if (options.onFavorite) options.onFavorite(calc.id);
            } else if (action === 'reset') {
                state = S.defaultState(calc, units, unitOf);
                typing = {};
                persist();
                paintChoices();
                paintFormulas(true);
                paint();
            } else if (action === 'copy') {
                copyText(plainText());
                const label = button.querySelector('[data-label]');
                button.dataset.done = 'true';
                label.textContent = t('desktop.ecalc_copied');
                button.title = t('desktop.ecalc_copied');
                clearTimeout(copyTimer);
                copyTimer = setTimeout(() => {
                    delete button.dataset.done;
                    label.textContent = t('desktop.ecalc_copy_results');
                    button.title = t('desktop.ecalc_copy_results');
                }, 1600);
            }
        });

        // The split layout needs room for the formula next to the stage; the fields sit on the schematic only when the
        // stage is at least as wide as the drawing (so the chips never crowd), otherwise they are listed under it. The
        // shell calls this whenever the page host changes its size (a page must not observe itself: its own layout
        // changes its height).
        function layout() {
            const width = el.clientWidth;
            if (!width) return;
            const need = spec ? spec.w * Symbols.GRID + STAGE_INSET : 0;
            el.dataset.layout = (spec ? width >= need + FORMULA_COLUMN : width >= NARROW_BELOW) ? 'split' : 'stack';
            el.dataset.narrow = width < NARROW_BELOW ? 'true' : 'false';
            if (spec) el.dataset.chips = width >= need ? 'anchored' : 'listed';
        }

        paintChoices();
        paintActions();
        paintFormulas(true);
        paint();

        return {
            el,
            layout,
            // The settings changed (digits, E-series, symbol standard): draw everything again.
            refresh() {
                drawSchematic();
                paintActions();
                paintFormulas(true);
                paint();
            },
            focusFirst() {
                const first = [...fields.values()].find(refs => has(state.inputs, refs.v.id));
                if (first) first.input.focus();
            },
            dispose() {
                for (const timer of timers.values()) clearTimeout(timer);
                timers.clear();
                clearTimeout(copyTimer);
                store.flush();
            }
        };
    }

    window.ECalcCalcPage = { create };
})();
