(function () {
    'use strict';

    function tokenize(expression) {
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
                    if ((current >= 'a' && current <= 'z') || (current >= 'A' && current <= 'Z')) index += 1;
                    else break;
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

    function factorial(value) {
        if (!Number.isFinite(value) || value < 0 || !Number.isInteger(value) || value > 170) throw new Error('Invalid expression');
        let result = 1;
        for (let index = 2; index <= value; index += 1) result *= index;
        return result;
    }

    function applyOperation(name, value) {
        switch (name) {
            case 'sin': return Math.sin(value);
            case 'cos': return Math.cos(value);
            case 'tan': return Math.tan(value);
            case 'sqrt': return Math.sqrt(value);
            case 'log': return Math.log10(value);
            case 'ln': return Math.log(value);
            case 'abs': return Math.abs(value);
            case 'factorial': return factorial(value);
            default: throw new Error('Invalid expression');
        }
    }

    function parse(tokens) {
        let position = 0;
        const peek = () => tokens[position] || { type: 'eof', value: '' };
        const consume = () => tokens[position++] || { type: 'eof', value: '' };
        const expectOperator = value => {
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
                if ((operator === '/' || operator === '%') && right === 0) throw new Error('Invalid expression');
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
                value = factorial(value);
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
                return applyOperation(name, parseExpressionAfterOpen());
            }
            throw new Error('Invalid expression');
        };
        const parseExpressionAfterOpen = () => {
            const value = parseExpression();
            expectOperator(')');
            return value;
        };

        const value = parseExpression();
        if (peek().type !== 'eof' || !Number.isFinite(value)) throw new Error('Invalid expression');
        return value;
    }

    function evaluate(expression) {
        return parse(tokenize(String(expression || '')));
    }

    window.AuraCalculatorExpression = Object.freeze({ evaluate });
}());
