(function () {
    'use strict';

    let host, ctx, esc, t, showDesktopNotification, registerWindowCleanup, showContextMenu, wireContextMenuBoundary;

    function resolve(fn, args) {
        if (typeof fn === 'function') return fn.apply(null, args);
        return fn;
    }

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

    function calcButton(def) {
        const label = def.label || def.key;
        const classes = [def.kind || '', def.className || ''].filter(Boolean).join(' ');
        return `<button type="button" class="${esc(classes)}" data-key="${esc(def.key)}">${esc(label)}</button>`;
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
                { key: 'backspace', label: 'Back' },
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
                { key: 'backspace', label: 'Back' },
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
                { key: 'backspace', label: 'Back' },
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
            <div class="vd-calc-tabs">
                <button type="button" class="active" data-mode="standard">${esc(t('desktop.calc_standard'))}</button>
                <button type="button" data-mode="scientific">${esc(t('desktop.calc_scientific'))}</button>
                <button type="button" data-mode="programmer">${esc(t('desktop.calc_programmer'))}</button>
            </div>
            <div class="vd-calc-prog-section" data-prog-section>
                <div class="vd-calc-base" data-base-selector>
                    <button type="button" class="active" data-base="10">${esc(t('desktop.calc_dec'))}</button>
                    <button type="button" data-base="16">${esc(t('desktop.calc_hex'))}</button>
                    <button type="button" data-base="2">${esc(t('desktop.calc_bin'))}</button>
                    <button type="button" data-base="8">${esc(t('desktop.calc_oct'))}</button>
                </div>
                <div class="vd-calc-prog-display" data-prog-display>
                    <div><span>HEX</span><span data-hex>0</span></div>
                    <div><span>DEC</span><span data-dec>0</span></div>
                    <div><span>OCT</span><span data-oct>0</span></div>
                    <div><span>BIN</span><span data-bin>0</span></div>
                </div>
            </div>
            <div class="vd-calc-display"><div data-expression>0</div><strong data-result>0</strong></div>
            <div class="vd-calc-keys">
                ${keySets.standard.map(calcButton).join('')}
            </div>
            <aside class="vd-calc-history"><div>${esc(t('desktop.calc_history'))}</div><ol></ol></aside>
        </div>`;
        const root = host.querySelector('.vd-calc');
        const expressionEl = host.querySelector('[data-expression]');
        const resultEl = host.querySelector('[data-result]');
        const historyEl = host.querySelector('.vd-calc-history ol');
        const keysEl = host.querySelector('.vd-calc-keys');
        const baseSelector = host.querySelector('[data-base-selector]');
        const progDisplay = host.querySelector('[data-prog-display]');
        const progSection = host.querySelector('[data-prog-section]');
        let expression = '';
        let mode = 'standard';
        let progBase = 10;
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
        const update = (result) => {
            expressionEl.textContent = expression || '0';
            const displayResult = result == null ? '0' : String(result);
            resultEl.textContent = displayResult;
            if (mode === 'programmer' && progDisplay) {
                const num = parseInt(displayResult, 10);
                const safeNum = Number.isFinite(num) ? num : 0;
                progDisplay.querySelector('[data-hex]').textContent = safeNum.toString(16).toUpperCase();
                progDisplay.querySelector('[data-dec]').textContent = String(safeNum);
                progDisplay.querySelector('[data-oct]').textContent = safeNum.toString(8);
                progDisplay.querySelector('[data-bin]').textContent = safeNum.toString(2);
            }
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
            historyEl.innerHTML = history.map(item => `<li>${esc(item)}</li>`).join('');
            expression = String(result);
            update(result);
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
                else if (key === 'CE') expression = '';
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
                else if (key === 'negate') expression = expression ? `(-1*(${expression}))` : '-';
                else if (key === 'square') expression += '^2';
                else if (key === 'power') expression += '^';
                else if (key === 'n!') expression += '!';
                else if (['sin', 'cos', 'tan', 'log', 'ln', 'sqrt'].includes(key)) expression += `${key}(`;
                else expression += key;
                update();
                flashDisplay();
            } catch (err) {
                resultEl.textContent = err.message;
            }
        };
        const bindKeyButtons = () => {
            host.querySelectorAll('[data-key]').forEach(btn => btn.addEventListener('click', () => {
                btn.classList.add('pressed');
                setTimeout(() => btn.classList.remove('pressed'), 120);
                press(btn.dataset.key);
            }));
        };
        const renderCalculatorKeys = () => {
            if (!keysEl) return;
            keysEl.innerHTML = (keySets[mode] || keySets.standard).map(calcButton).join('');
            bindKeyButtons();
        };
        renderCalculatorKeys();
        host.querySelectorAll('[data-mode]').forEach(btn => btn.addEventListener('click', () => {
            host.querySelectorAll('[data-mode]').forEach(item => item.classList.toggle('active', item === btn));
            mode = btn.dataset.mode;
            root.classList.toggle('scientific-on', mode === 'scientific');
            root.classList.toggle('programmer-on', mode === 'programmer');
            if (progSection) progSection.hidden = mode !== 'programmer';
            renderCalculatorKeys();
            expression = '';
            update();
        }));
        host.querySelectorAll('[data-base]').forEach(btn => btn.addEventListener('click', () => {
            host.querySelectorAll('[data-base]').forEach(item => item.classList.toggle('active', item === btn));
            progBase = parseInt(btn.dataset.base, 10);
            expression = '';
            update();
        }));
        root.addEventListener('keydown', event => {
            const map = { Enter: '=', Backspace: 'backspace', Escape: 'C', '*': '*', '/': '/' };
            const key = map[event.key] || event.key;
            if (mode === 'programmer') {
                const programmerKey = key === 'backspace' ? 'backspace' : (key === 'c' || key === 'C' ? 'HEX_C' : key.toUpperCase());
                if (/^[0-9A-Fa-f]$/.test(key) || ['+','-','*','/','(',')','=','backspace'].includes(key)) {
                    event.preventDefault();
                    animateButton(programmerKey);
                    press(programmerKey);
                    return;
                }
            }
            if (/^[0-9.+\-()%*/]$/.test(key) || ['=', 'backspace', 'C'].includes(key)) {
                event.preventDefault();
                animateButton(key);
                press(key);
            }
        });
        root.focus();
        registerWindowCleanup(id, () => {
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
