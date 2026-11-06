(function () {
    'use strict';

    let host, ctx, esc, t, showDesktopNotification, registerWindowCleanup, showContextMenu, wireContextMenuBoundary;

    function tokenizeCalculatorExpression(expression) {
        const tokens = [];
        let index = 0;
        while (index < expression.length) {
            const char = expression[index];
            if (/\s/.test(char)) {
                index += 1;
                continue;
            }
            if ((char >= '0' && char <= '9') || char === '.') {
                const start = index;
                let hasDigit = false;
                let hasDot = false;
                while (index < expression.length) {
                    const current = expression[index];
                    if (current >= '0' && current <= '9') {
                        hasDigit = true;
                        index += 1;
                    } else if (current === '.' && !hasDot) {
                        hasDot = true;
                        index += 1;
                    } else {
                        break;
                    }
                }
                if (!hasDigit) throw new Error('Invalid expression');
                const value = Number(expression.slice(start, index));
                if (!Number.isFinite(value)) throw new Error('Invalid expression');
                tokens.push({ type: 'number', value });
                continue;
            }
            if ((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')) {
                const start = index;
                while (index < expression.length) {
                    const current = expression[index];
                    if ((current >= 'a' && current <= 'z') || (current >= 'A' && current <= 'Z')) {
                        index += 1;
                    } else {
                        break;
                    }
                }
                tokens.push({ type: 'identifier', value: expression.slice(start, index) });
                continue;
            }
            if ('+-*/%^()!'.includes(char)) {
                tokens.push({ type: 'operator', value: char });
                index += 1;
                continue;
            }
            throw new Error('Invalid expression');
        }
        tokens.push({ type: 'eof', value: '' });
        return tokens;
    }

    function calculatorFactorial(value) {
        if (!Number.isFinite(value) || value < 0 || !Number.isInteger(value)) throw new Error('Invalid expression');
        if (value > 170) throw new Error('Invalid expression');
        let result = 1;
        for (let i = 2; i <= value; i += 1) result *= i;
        return result;
    }

    function ensureFiniteCalculatorResult(value) {
        if (!Number.isFinite(value)) throw new Error('Invalid expression');
        return value;
    }

    function rejectZeroDivisor(operator, right) {
        if ((operator === '/' || operator === '%' || operator === 'MOD') && right === 0) throw new Error('Invalid expression');
    }

    function applyCalculatorOperation(name, value) {
        switch (name) {
            case 'sin':
                return Math.sin(value);
            case 'cos':
                return Math.cos(value);
            case 'tan':
                return Math.tan(value);
            case 'sqrt':
                return Math.sqrt(value);
            case 'log':
                return Math.log10(value);
            case 'ln':
                return Math.log(value);
            case 'abs':
                return Math.abs(value);
            case 'factorial':
                return calculatorFactorial(value);
            default:
                throw new Error('Invalid expression');
        }
    }

    function parseCalculatorExpression(tokens) {
        let position = 0;
        const peek = () => tokens[position] || { type: 'eof', value: '' };
        const consume = () => tokens[position++] || { type: 'eof', value: '' };
        const expectOperator = (value) => {
            const token = consume();
            if (token.type !== 'operator' || token.value !== value) throw new Error('Invalid expression');
        };
        const parseExpression = () => parseAdditiveExpression();
        const parseAdditiveExpression = () => {
            let value = parseMultiplicativeExpression();
            while (peek().type === 'operator' && (peek().value === '+' || peek().value === '-')) {
                const operator = consume().value;
                const right = parseMultiplicativeExpression();
                value = operator === '+' ? value + right : value - right;
            }
            return value;
        };
        const parseMultiplicativeExpression = () => {
            let value = parseUnaryExpression();
            while (peek().type === 'operator' && (peek().value === '*' || peek().value === '/' || peek().value === '%')) {
                const operator = consume().value;
                const right = parseUnaryExpression();
                rejectZeroDivisor(operator, right);
                if (operator === '*') value *= right;
                else if (operator === '/') value /= right;
                else value %= right;
            }
            return value;
        };
        const parseUnaryExpression = () => {
            if (peek().type === 'operator' && (peek().value === '+' || peek().value === '-')) {
                const operator = consume().value;
                const value = parseUnaryExpression();
                return operator === '-' ? -value : value;
            }
            return parsePowerExpression();
        };
        const parsePowerExpression = () => {
            let value = parsePostfixExpression();
            if (peek().type === 'operator' && peek().value === '^') {
                consume();
                value = Math.pow(value, parseUnaryExpression());
            }
            return value;
        };
        const parsePostfixExpression = () => {
            let value = parsePrimaryExpression();
            while (peek().type === 'operator' && peek().value === '!') {
                consume();
                value = calculatorFactorial(value);
            }
            return value;
        };
        const parsePrimaryExpression = () => {
            const token = consume();
            if (token.type === 'number') return token.value;
            if (token.type === 'operator' && token.value === '(') {
                const value = parseExpression();
                expectOperator(')');
                return value;
            }
            if (token.type === 'identifier') {
                const name = token.value.toLowerCase();
                if (name === 'pi') return Math.PI;
                if (name === 'e') return Math.E;
                expectOperator('(');
                const value = parseExpression();
                expectOperator(')');
                return applyCalculatorOperation(name, value);
            }
            throw new Error('Invalid expression');
        };
        const value = parseExpression();
        if (peek().type !== 'eof') throw new Error('Invalid expression');
        return value;
    }

    function evaluateCalculatorExpression(expression) {
        const value = parseCalculatorExpression(tokenizeCalculatorExpression(expression));
        return ensureFiniteCalculatorResult(value);
    }

    function calculatorErrorMessage(err) {
        if (err && err.message === 'Invalid expression' && typeof t === 'function') {
            return t('desktop.calc_invalid_expression');
        }
        return err && err.message ? err.message : String(err || '');
    }

    const CALCULATOR_KEY_LABELS = {
        C: 'desktop.calc_key_clear',
        CE: 'desktop.calc_key_clear_entry',
        negate: 'desktop.calc_key_negate',
        square: 'desktop.calc_key_square',
        power: 'desktop.calc_key_power',
        'n!': 'desktop.calc_key_factorial'
    };

    function calculatorKeyLabel(def) {
        const key = CALCULATOR_KEY_LABELS[def.key];
        return key ? t(key) : '';
    }

    function calcButton(def) {
        const label = def.label || def.key;
        const classes = [def.kind || '', def.className || ''].filter(Boolean).join(' ');
        const aria = calculatorKeyLabel(def);
        const ariaAttr = aria ? ` aria-label="${esc(aria)}" title="${esc(aria)}"` : '';
        return `<button type="button" class="${esc(classes)}" data-key="${esc(def.key)}"${ariaAttr}>${esc(label)}</button>`;
    }

    // CE clears the entry that is being typed: the trailing number, or the trailing
    // operator when no number follows it. C clears the whole expression.
    function clearCalculatorEntry(expression, mode, base) {
        const text = String(expression || '');
        const logicalOperatorMatch = text.match(/(?:^|\s)(?:AND|OR|XOR|SHL|SHR|MOD|NOT)\s*$/i);
        if (logicalOperatorMatch) return text.slice(0, text.length - logicalOperatorMatch[0].length).replace(/\s+$/, '');
        let numberMatch;
        if (mode === 'programmer') {
            const digits = base === 16 ? '0-9A-Fa-f' : base === 8 ? '0-7' : base === 2 ? '01' : '0-9';
            numberMatch = text.match(new RegExp(`[${digits}]+$`));
        } else {
            numberMatch = text.match(/[0-9]*\.?[0-9]*$/);
            if (numberMatch && !numberMatch[0]) numberMatch = null;
        }
        if (numberMatch && numberMatch[0]) return text.slice(0, text.length - numberMatch[0].length);
        const operatorMatch = text.match(/[+\-*/^%()]\s*$/);
        if (operatorMatch) return text.slice(0, text.length - operatorMatch[0].length).replace(/\s+$/, '');
        return text;
    }

    // The sign toggle negates the number that is being typed instead of wrapping the
    // whole expression, so the visible expression stays readable.
    function negateCalculatorExpression(expression) {
        const text = String(expression || '');
        if (!text) return '-';
        const numberMatch = text.match(/(-?)([0-9]*\.?[0-9]+)$/);
        if (numberMatch && numberMatch[2]) {
            const head = text.slice(0, text.length - numberMatch[0].length);
            return numberMatch[1] ? head + numberMatch[2] : `${head}-${numberMatch[2]}`;
        }
        return `(-${text})`;
    }

    function render(hostEl, windowId, renderCtx) {
        ctx = renderCtx || {};
        esc = ctx.esc;
        t = ctx.t;
        showDesktopNotification = ctx.showDesktopNotification;
        registerWindowCleanup = ctx.registerWindowCleanup;
        showContextMenu = ctx.showContextMenu;
        wireContextMenuBoundary = ctx.wireContextMenuBoundary;
        host = hostEl;
        if (!host) return;
        renderCalculator(windowId);
    }

    function dispose(windowId) {
        if (host) host.innerHTML = '';
        host = null;
        ctx = null;
    }

    function renderCalculator(id) {
        const keySets = {
            standard: [
                { key: 'C', kind: 'danger' },
                { key: 'CE', kind: 'danger' },
                { key: 'backspace', label: t('desktop.calc_back') },
                { key: '/', kind: 'op' },
                { key: '7' },
                { key: '8' },
                { key: '9' },
                { key: '*', kind: 'op' },
                { key: '4' },
                { key: '5' },
                { key: '6' },
                { key: '-', kind: 'op' },
                { key: '1' },
                { key: '2' },
                { key: '3' },
                { key: '+', kind: 'op' },
                { key: 'negate', label: '+/-' },
                { key: '0' },
                { key: '.' },
                { key: '=', kind: 'eq' }
            ],
            scientific: [
                { key: 'sin', kind: 'fn' },
                { key: 'C', kind: 'danger' },
                { key: 'CE', kind: 'danger' },
                { key: 'backspace', label: t('desktop.calc_back') },
                { key: '/', kind: 'op' },
                { key: 'cos', kind: 'fn' },
                { key: '7' },
                { key: '8' },
                { key: '9' },
                { key: '*', kind: 'op' },
                { key: 'tan', kind: 'fn' },
                { key: '4' },
                { key: '5' },
                { key: '6' },
                { key: '-', kind: 'op' },
                { key: 'log', kind: 'fn' },
                { key: '1' },
                { key: '2' },
                { key: '3' },
                { key: '+', kind: 'op' },
                { key: 'ln', kind: 'fn' },
                { key: 'negate', label: '+/-' },
                { key: '0' },
                { key: '.' },
                { key: '=', kind: 'eq' },
                { key: 'pi', label: 'pi', kind: 'fn' },
                { key: 'e', kind: 'fn' },
                { key: 'square', label: 'x^2', kind: 'fn' },
                { key: 'power', label: 'x^y', kind: 'fn' },
                { key: 'n!', kind: 'fn' },
                { key: 'sqrt', kind: 'fn' },
                { key: '(', kind: 'fn' },
                { key: ')', kind: 'fn' },
                { key: '%', kind: 'op' },
                { key: '00' }
            ],
            programmer: [
                { key: 'AND', kind: 'fn' },
                { key: 'C', kind: 'danger' },
                { key: 'CE', kind: 'danger' },
                { key: 'backspace', label: t('desktop.calc_back') },
                { key: '/', kind: 'op' },
                { key: 'OR', kind: 'fn' },
                { key: '7' },
                { key: '8' },
                { key: '9' },
                { key: '*', kind: 'op' },
                { key: 'XOR', kind: 'fn' },
                { key: '4' },
                { key: '5' },
                { key: '6' },
                { key: '-', kind: 'op' },
                { key: 'NOT', kind: 'fn' },
                { key: '1' },
                { key: '2' },
                { key: '3' },
                { key: '+', kind: 'op' },
                { key: 'MOD', kind: 'fn' },
                { key: '0' },
                { key: 'A', kind: 'fn' },
                { key: 'B', kind: 'fn' },
                { key: '=', kind: 'eq' },
                { key: 'SHL', kind: 'fn' },
                { key: 'SHR', kind: 'fn' },
                { key: 'HEX_C', label: 'C', kind: 'fn' },
                { key: 'D', kind: 'fn' },
                { key: 'E', kind: 'fn' },
                { key: 'F', kind: 'fn' },
                { key: '(', kind: 'fn' },
                { key: ')', kind: 'fn' }
            ]
        };
        host.innerHTML = `<div class="vd-calc" tabindex="0">
            <div class="vd-calc-tabs" role="tablist" aria-label="${esc(t('desktop.calc_mode_label'))}">
                <button type="button" role="tab" class="active" data-mode="standard" aria-selected="true">${esc(t('desktop.calc_standard'))}</button>
                <button type="button" role="tab" data-mode="scientific" aria-selected="false">${esc(t('desktop.calc_scientific'))}</button>
                <button type="button" role="tab" data-mode="programmer" aria-selected="false">${esc(t('desktop.calc_programmer'))}</button>
            </div>
            <div class="vd-calc-prog-section" data-prog-section hidden>
                <div class="vd-calc-base" data-base-selector role="group" aria-label="${esc(t('desktop.calc_base_label'))}">
                    <button type="button" class="active" data-base="10" aria-pressed="true">${esc(t('desktop.calc_dec'))}</button>
                    <button type="button" data-base="16" aria-pressed="false">${esc(t('desktop.calc_hex'))}</button>
                    <button type="button" data-base="2" aria-pressed="false">${esc(t('desktop.calc_bin'))}</button>
                    <button type="button" data-base="8" aria-pressed="false">${esc(t('desktop.calc_oct'))}</button>
                </div>
                <div class="vd-calc-prog-display" data-prog-display>
                    <div class="vd-calc-prog-cell"><span class="vd-calc-prog-label">HEX</span><span class="vd-calc-prog-value" data-hex>0</span></div>
                    <div class="vd-calc-prog-cell"><span class="vd-calc-prog-label">DEC</span><span class="vd-calc-prog-value" data-dec>0</span></div>
                    <div class="vd-calc-prog-cell"><span class="vd-calc-prog-label">OCT</span><span class="vd-calc-prog-value" data-oct>0</span></div>
                    <div class="vd-calc-prog-cell"><span class="vd-calc-prog-label">BIN</span><span class="vd-calc-prog-value" data-bin>0</span></div>
                </div>
            </div>
            <div class="vd-calc-display" aria-label="${esc(t('desktop.calc_display_label'))}"><button type="button" class="vd-calc-copy" data-copy aria-label="${esc(t('desktop.calc_copy'))}" title="${esc(t('desktop.calc_copy'))}"></button><div class="vd-calc-expression" data-expression>0</div><strong class="vd-calc-result" data-result aria-live="polite" aria-atomic="true">0</strong></div>
            <div class="vd-calc-keys" role="group" aria-label="${esc(t('desktop.calc_keys_label'))}">
                ${keySets.standard.map(calcButton).join('')}
            </div>
            <aside class="vd-calc-history"><div class="vd-calc-history-head"><span class="vd-calc-history-title">${esc(t('desktop.calc_history'))}</span><button type="button" class="vd-calc-history-clear" data-clear-history aria-label="${esc(t('desktop.calc_clear_history'))}" title="${esc(t('desktop.calc_clear_history'))}" hidden>&#215;</button></div><ol class="vd-calc-history-list" data-history-list></ol><p class="vd-calc-history-empty" data-history-empty>${esc(t('desktop.calc_history_empty'))}</p></aside>
        </div>`;
        const root = host.querySelector('.vd-calc');
        const expressionEl = host.querySelector('[data-expression]');
        const resultEl = host.querySelector('[data-result]');
        const historyEl = host.querySelector('[data-history-list]');
        const historyEmptyEl = host.querySelector('[data-history-empty]');
        const historyClearEl = host.querySelector('[data-clear-history]');
        const copyEl = host.querySelector('[data-copy]');
        const keysEl = host.querySelector('.vd-calc-keys');
        const baseSelector = host.querySelector('[data-base-selector]');
        const progDisplay = host.querySelector('[data-prog-display]');
        const progSection = host.querySelector('[data-prog-section]');
        let expression = '';
        let mode = 'standard';
        let progBase = 10;
        let lastResult = 0;
        let copyTimer = null;
        const history = [];
        const showCalculatorContextMenu = event => {
            if (!event.target.closest('.vd-calc-display')) return false;
            showContextMenu(event.clientX, event.clientY, [
                { labelKey: 'desktop.fm.copy', icon: 'copy', action: async () => {
                    try {
                        if (navigator.clipboard && navigator.clipboard.writeText) await navigator.clipboard.writeText(resultEl.textContent || '0');
                    } catch (err) {
                        showDesktopNotification({ title: t('desktop.notification'), message: err.message });
                    }
                } }
            ]);
            return true;
        };
        wireContextMenuBoundary(root, { onContextMenu: showCalculatorContextMenu });
        const formatCalculatorValue = value => {
            if (value === 0 || value === -0) return '0';
            return String(value);
        };
        const setResultState = (text, state) => {
            resultEl.textContent = text;
            resultEl.classList.toggle('error', state === 'error');
            resultEl.classList.toggle('pending', state === 'pending');
            const length = String(text || '').length;
            const scale = length > 18 ? 0.5 : length > 14 ? 0.62 : length > 11 ? 0.75 : length > 8 ? 0.88 : 1;
            resultEl.style.setProperty('--calc-result-scale', String(scale));
            updateProgrammerDisplay();
        };
        const updateProgrammerDisplay = () => {
            if (mode !== 'programmer' || !progDisplay) return;
            const num = parseInt(resultEl.textContent || '0', 10);
            const safeNum = Number.isFinite(num) ? num : 0;
            progDisplay.querySelector('[data-hex]').textContent = safeNum.toString(16).toUpperCase();
            progDisplay.querySelector('[data-dec]').textContent = String(safeNum);
            progDisplay.querySelector('[data-oct]').textContent = safeNum.toString(8);
            progDisplay.querySelector('[data-bin]').textContent = safeNum.toString(2);
        };
        const refresh = () => {
            expressionEl.textContent = expression || '0';
            if (expressionEl.scrollWidth > expressionEl.clientWidth) expressionEl.scrollLeft = expressionEl.scrollWidth;
        };
        const previewExpression = () => {
            if (!expression) {
                setResultState('0', '');
                return;
            }
            try {
                const value = mode === 'programmer'
                    ? evaluateProgrammerExpression(expression, progBase)
                    : evaluateCalculatorExpression(expression);
                const result = mode === 'programmer' ? value : Number(value.toFixed(10));
                lastResult = result;
                setResultState(formatCalculatorValue(result), '');
            } catch (err) {
                setResultState(formatCalculatorValue(lastResult), 'pending');
            }
        };
        const renderHistory = () => {
            if (historyEl) {
                historyEl.innerHTML = history.map((item, index) => {
                    const separator = item.lastIndexOf(' = ');
                    const expr = separator >= 0 ? item.slice(0, separator) : item;
                    const value = separator >= 0 ? item.slice(separator + 3) : '';
                    const restore = `${esc(t('desktop.calc_history_restore'))}: ${esc(value)}`;
                    return `<li><button type="button" class="vd-calc-history-entry" data-history-index="${index}" aria-label="${restore}" title="${esc(expr)} = ${esc(value)}"><span class="vd-calc-history-expr">${esc(expr)}</span><span class="vd-calc-history-value">${esc(value)}</span></button></li>`;
                }).join('');
            }
            if (historyEmptyEl) historyEmptyEl.hidden = history.length > 0;
            if (historyClearEl) historyClearEl.hidden = history.length === 0;
        };
        const evaluate = () => {
            if (!expression) return;
            let value;
            if (mode === 'programmer') {
                value = evaluateProgrammerExpression(expression, progBase);
            } else {
                value = evaluateCalculatorExpression(expression);
            }
            let result;
            if (mode === 'programmer') {
                result = value;
            } else {
                result = Number(value.toFixed(10));
            }
            history.unshift(`${expression} = ${result}`);
            history.splice(8);
            renderHistory();
            expression = String(result);
            lastResult = result;
            setResultState(formatCalculatorValue(result), '');
            refresh();
        };
        const animateButton = key => {
            const btn = host.querySelector(`[data-key="${esc(key)}"]`);
            if (!btn) return;
            btn.classList.add('pressed');
            setTimeout(() => btn.classList.remove('pressed'), 120);
        };
        const flashDisplay = () => {
            resultEl.classList.add('typing');
            setTimeout(() => resultEl.classList.remove('typing'), 150);
        };
        const validDigitForBase = ch => {
            if (ch === 'HEX_C') return progBase === 16;
            if (progBase === 2) return /[01]/.test(ch);
            if (progBase === 8) return /[0-7]/.test(ch);
            if (progBase === 10) return /[0-9]/.test(ch);
            if (progBase === 16) return /[0-9A-Fa-f]/.test(ch);
            return true;
        };
        const press = key => {
            try {
                if (key === 'C') expression = '';
                else if (key === 'CE') expression = clearCalculatorEntry(expression, mode, progBase);
                else if (key === 'backspace') expression = expression.slice(0, -1);
                else if (key === '=') {
                    evaluate();
                    animateButton('=');
                    return;
                }
                else if (mode === 'programmer') {
                    if (['AND','OR','XOR','SHL','SHR','MOD'].includes(key)) expression += ` ${key} `;
                    else if (key === 'NOT') expression += 'NOT ';
                    else if (key === 'HEX_C') {
                        if (validDigitForBase(key)) expression += 'C';
                    }
                    else if (/^[0-9A-Fa-f]$/.test(key)) {
                        if (validDigitForBase(key)) expression += key;
                    }
                    else if (['+','-','*','/','(',')'].includes(key)) expression += key;
                }
                else if (key === 'negate') expression = negateCalculatorExpression(expression);
                else if (key === 'square') expression += '^2';
                else if (key === 'power') expression += '^';
                else if (key === 'n!') expression += '!';
                else if (['sin', 'cos', 'tan', 'log', 'ln', 'sqrt'].includes(key)) expression += `${key}(`;
                else expression += key;
                refresh();
                previewExpression();
                flashDisplay();
            } catch (err) {
                resultEl.classList.add('error');
                resultEl.textContent = calculatorErrorMessage(err);
            }
        };
        const bindKeyButtons = () => {
            host.querySelectorAll('[data-key]').forEach(btn => btn.addEventListener('click', () => {
                btn.classList.add('pressed');
                setTimeout(() => btn.classList.remove('pressed'), 120);
                press(btn.dataset.key);
            }));
        };
        const syncProgrammerKeyStates = () => {
            if (!keysEl) return;
            keysEl.querySelectorAll('[data-key]').forEach(btn => {
                const key = btn.dataset.key;
                let disabled = false;
                if (mode === 'programmer') {
                    if (key === 'HEX_C') disabled = progBase !== 16;
                    else if (key === 'A' || key === 'B' || key === 'D' || key === 'E' || key === 'F') disabled = !validDigitForBase(key);
                    else if (/^[0-9]$/.test(key)) disabled = !validDigitForBase(key);
                }
                btn.disabled = disabled;
                btn.setAttribute('aria-disabled', disabled ? 'true' : 'false');
            });
        };
        const renderCalculatorKeys = () => {
            if (!keysEl) return;
            keysEl.innerHTML = (keySets[mode] || keySets.standard).map(calcButton).join('');
            bindKeyButtons();
            syncProgrammerKeyStates();
        };
        renderCalculatorKeys();
        renderHistory();
        const carryCalculatorValue = (text, fromMode, toMode, base) => {
            if (!text) return '';
            let value;
            try {
                value = fromMode === 'programmer'
                    ? evaluateProgrammerExpression(text, progBase)
                    : evaluateCalculatorExpression(text);
            } catch (err) {
                return '';
            }
            if (!Number.isFinite(value)) return '';
            const result = fromMode === 'programmer' ? value : Number(value.toFixed(10));
            if (toMode !== 'programmer') return String(result);
            const integer = Math.trunc(result);
            return integer.toString(base).toUpperCase();
        };
        host.querySelectorAll('[data-mode]').forEach(btn => btn.addEventListener('click', () => {
            const nextMode = btn.dataset.mode;
            if (!nextMode || nextMode === mode) return;
            const carried = carryCalculatorValue(expression, mode, nextMode, progBase);
            host.querySelectorAll('[data-mode]').forEach(item => {
                const active = item === btn;
                item.classList.toggle('active', active);
                item.setAttribute('aria-selected', active ? 'true' : 'false');
            });
            mode = nextMode;
            root.classList.toggle('scientific-on', mode === 'scientific');
            root.classList.toggle('programmer-on', mode === 'programmer');
            if (progSection) progSection.hidden = mode !== 'programmer';
            renderCalculatorKeys();
            expression = carried;
            refresh();
            previewExpression();
        }));
        host.querySelectorAll('[data-base]').forEach(btn => btn.addEventListener('click', () => {
            const nextBase = parseInt(btn.dataset.base, 10);
            if (!Number.isFinite(nextBase) || nextBase === progBase) return;
            const carried = carryCalculatorValue(expression, mode, 'programmer', nextBase);
            host.querySelectorAll('[data-base]').forEach(item => {
                const active = item === btn;
                item.classList.toggle('active', active);
                item.setAttribute('aria-pressed', active ? 'true' : 'false');
            });
            progBase = nextBase;
            expression = carried;
            refresh();
            previewExpression();
            syncProgrammerKeyStates();
        }));
        if (historyEl) historyEl.addEventListener('click', event => {
            const btn = event.target.closest('[data-history-index]');
            if (!btn) return;
            const item = history[Number(btn.dataset.historyIndex)];
            if (!item) return;
            const separator = item.lastIndexOf(' = ');
            expression = separator >= 0 ? item.slice(separator + 3) : item;
            refresh();
            previewExpression();
            flashDisplay();
        });
        if (historyClearEl) historyClearEl.addEventListener('click', () => {
            history.splice(0, history.length);
            renderHistory();
        });
        const setCopyFeedback = state => {
            if (!copyEl) return;
            copyEl.classList.toggle('copied', state === 'copied');
            copyEl.classList.toggle('failed', state === 'failed');
            const label = state === 'copied' ? t('desktop.calc_copied') : state === 'failed' ? t('desktop.calc_copy_failed') : t('desktop.calc_copy');
            copyEl.setAttribute('aria-label', label);
            copyEl.setAttribute('title', label);
            if (copyTimer) clearTimeout(copyTimer);
            copyTimer = setTimeout(() => {
                copyEl.classList.remove('copied', 'failed');
                copyEl.setAttribute('aria-label', t('desktop.calc_copy'));
                copyEl.setAttribute('title', t('desktop.calc_copy'));
            }, 1400);
        };
        const copyResult = async () => {
            const text = resultEl.textContent || '0';
            try {
                if (!navigator.clipboard || !navigator.clipboard.writeText) throw new Error('clipboard unavailable');
                // A clipboard write that never settles must not leave the button without feedback.
                await Promise.race([
                    navigator.clipboard.writeText(text),
                    new Promise((resolve, reject) => setTimeout(() => reject(new Error('clipboard timeout')), 1500))
                ]);
                setCopyFeedback('copied');
            } catch (err) {
                setCopyFeedback('failed');
                if (typeof showDesktopNotification === 'function') {
                    showDesktopNotification({ title: t('desktop.notification'), message: t('desktop.calc_copy_failed') });
                }
            }
        };
        if (copyEl) copyEl.addEventListener('click', copyResult);
        root.addEventListener('keydown', event => {
            if (event.target !== root && (event.key === 'Enter' || event.key === ' ')) return;
            if ((event.ctrlKey || event.metaKey) && (event.key === 'c' || event.key === 'C')) {
                event.preventDefault();
                copyResult();
                return;
            }
            if (event.key === 'Escape') {
                event.preventDefault();
                animateButton('C');
                press('C');
                return;
            }
            const map = { Enter: '=', Backspace: 'backspace', Escape: 'C', '*': '*', '/': '/' };
            const key = map[event.key] || event.key;
            if (mode === 'programmer') {
                const programmerKey = key === 'backspace' ? 'backspace' : (key === 'c' ? 'HEX_C' : key.toUpperCase());
                if (/^[0-9A-Fa-f]$/.test(key) || ['+','-','*','/','(',')','=','backspace'].includes(key)) {
                    event.preventDefault();
                    animateButton(programmerKey);
                    press(programmerKey);
                    return;
                }
            }
            if (/^[0-9.+\-()%*/^!]$/.test(key) || ['=', 'backspace', 'C'].includes(key)) {
                event.preventDefault();
                animateButton(key);
                press(key);
            }
        });
        root.focus();
        registerWindowCleanup(id, () => {
            if (copyTimer) clearTimeout(copyTimer);
            host.innerHTML = '';
        });
    }

    function evaluateProgrammerExpression(expression, base) {
        const tokens = tokenizeProgrammerExpression(expression, base);
        return parseProgrammerExpression(tokens);
    }

    function tokenizeProgrammerExpression(expression, base) {
        const tokens = [];
        let index = 0;
        const isDigit = ch => {
            if (base === 2) return ch === '0' || ch === '1';
            if (base === 8) return ch >= '0' && ch <= '7';
            if (base === 10) return ch >= '0' && ch <= '9';
            if (base === 16) return (ch >= '0' && ch <= '9') || (ch >= 'A' && ch <= 'F') || (ch >= 'a' && ch <= 'f');
            return false;
        };
        while (index < expression.length) {
            const char = expression[index];
            if (/\s/.test(char)) {
                index += 1;
                continue;
            }
            if (isDigit(char)) {
                const start = index;
                while (index < expression.length && isDigit(expression[index])) index += 1;
                const numStr = expression.slice(start, index);
                const value = parseInt(numStr, base);
                if (!Number.isFinite(value)) throw new Error('Invalid expression');
                tokens.push({ type: 'number', value });
                continue;
            }
            if ((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')) {
                const start = index;
                while (index < expression.length) {
                    const current = expression[index];
                    if ((current >= 'a' && current <= 'z') || (current >= 'A' && current <= 'Z')) {
                        index += 1;
                    } else {
                        break;
                    }
                }
                tokens.push({ type: 'identifier', value: expression.slice(start, index) });
                continue;
            }
            if ('+-*/%^()!'.includes(char)) {
                tokens.push({ type: 'operator', value: char });
                index += 1;
                continue;
            }
            throw new Error('Invalid expression');
        }
        tokens.push({ type: 'eof', value: '' });
        return tokens;
    }

    function parseProgrammerExpression(tokens) {
        let position = 0;
        const peek = () => tokens[position] || { type: 'eof', value: '' };
        const consume = () => tokens[position++] || { type: 'eof', value: '' };
        const expectOperator = (value) => {
            const token = consume();
            if (token.type !== 'operator' || token.value !== value) throw new Error('Invalid expression');
        };
        const parseExpression = () => parseBitwiseOr();
        const parseBitwiseOr = () => {
            let value = parseBitwiseXor();
            while (peek().type === 'identifier' && peek().value === 'OR') {
                consume();
                value = value | parseBitwiseXor();
            }
            return value;
        };
        const parseBitwiseXor = () => {
            let value = parseBitwiseAnd();
            while (peek().type === 'identifier' && peek().value === 'XOR') {
                consume();
                value = value ^ parseBitwiseAnd();
            }
            return value;
        };
        const parseBitwiseAnd = () => {
            let value = parseShift();
            while (peek().type === 'identifier' && peek().value === 'AND') {
                consume();
                value = value & parseShift();
            }
            return value;
        };
        const parseShift = () => {
            let value = parseAdditive();
            while (peek().type === 'identifier' && (peek().value === 'SHL' || peek().value === 'SHR')) {
                const op = consume().value;
                const right = parseAdditive();
                value = op === 'SHL' ? value << right : value >> right;
            }
            return value;
        };
        const parseAdditive = () => {
            let value = parseMultiplicative();
            while (peek().type === 'operator' && (peek().value === '+' || peek().value === '-')) {
                const operator = consume().value;
                const right = parseMultiplicative();
                value = operator === '+' ? value + right : value - right;
            }
            return value;
        };
        const parseMultiplicative = () => {
            let value = parseUnary();
            while ((peek().type === 'operator' && (peek().value === '*' || peek().value === '/' || peek().value === '%')) || (peek().type === 'identifier' && peek().value === 'MOD')) {
                const operator = peek().type === 'identifier' ? consume().value : consume().value;
                const right = parseUnary();
                rejectZeroDivisor(operator, right);
                if (operator === '*') value *= right;
                else if (operator === '/') value = Math.floor(value / right);
                else if (operator === '%' || operator === 'MOD') value %= right;
            }
            return value;
        };
        const parseUnary = () => {
            if (peek().type === 'identifier' && peek().value === 'NOT') {
                consume();
                return ~parseUnary();
            }
            if (peek().type === 'operator' && (peek().value === '+' || peek().value === '-')) {
                const operator = consume().value;
                const value = parseUnary();
                return operator === '-' ? -value : value;
            }
            return parsePrimary();
        };
        const parsePrimary = () => {
            const token = consume();
            if (token.type === 'number') return token.value;
            if (token.type === 'operator' && token.value === '(') {
                const value = parseExpression();
                expectOperator(')');
                return value;
            }
            throw new Error('Invalid expression');
        };
        const value = parseExpression();
        if (peek().type !== 'eof') throw new Error('Invalid expression');
        return value;
    }

    window.CalculatorApp = { render, dispose };
}());
