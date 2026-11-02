/* ECalc expression language: lexer, parser, AST, evaluation and dependency lists. A DOM-free classic script that
   publishes window.ECalcCore.expr, so scripts/test-ecalc-core.mjs can run it in a Node vm against the shared vectors of
   internal/ecalc/testdata/vectors. The grammar below is the whole language (it mirrors internal/ecalc/expr.go): no
   assignment, no loops, a fixed function whitelist, and no dynamic code evaluation of any kind.

     expr   := cmp
     cmp    := add (('<' | '<=' | '>' | '>=' | '==' | '!=') add)?
     add    := mul (('+' | '-') mul)*
     mul    := unary (('*' | '/') unary)*
     unary  := ('-' | '+') unary | pow
     pow    := atom ('^' unary)?
     atom   := number | ident '(' args? ')' | ident | '(' expr ')'

   AST nodes: {t:'num',v,raw} {t:'id',name} {t:'neg',a} {t:'pos',a} {t:'bin',op,l,r} {t:'call',name,args}; a formula is
   {t:'eq',parts}. Errors carry code 'parse' or 'eval'. */
(function () {
    'use strict';

    const core = (window.ECalcCore = window.ECalcCore || {});

    // Exact arity per function; a negative number means "at least that many".
    const ARITY = { sqrt: 1, ln: 1, log10: 1, exp: 1, sin: 1, cos: 1, tan: 1, atan: 1, abs: 1, round: 1, min: -2, max: -2, if: 3, sum: 1, par: 1 };
    const COMPARISONS = { '<': true, '<=': true, '>': true, '>=': true, '==': true, '!=': true };
    const has = (object, key) => Object.prototype.hasOwnProperty.call(object, key);
    const isDigit = char => char >= '0' && char <= '9';
    const isIdentStart = char => (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || char === '_';
    const isIdentPart = char => isIdentStart(char) || isDigit(char);

    function fail(message, code) {
        const error = new Error(message);
        error.code = code || 'parse';
        return error;
    }

    function tokenize(source) {
        const text = String(source == null ? '' : source);
        const tokens = [];
        let i = 0;
        while (i < text.length) {
            const char = text[i];
            if (char === ' ' || char === '\t' || char === '\n' || char === '\r') { i += 1; continue; }
            if (isDigit(char) || (char === '.' && isDigit(text[i + 1]))) {
                let j = i;
                while (isDigit(text[j])) j += 1;
                if (text[j] === '.') {
                    if (!isDigit(text[j + 1])) throw fail('digits expected after the decimal point at ' + j);
                    j += 1;
                    while (isDigit(text[j])) j += 1;
                }
                if (text[j] === 'e' || text[j] === 'E') {
                    let k = j + 1;
                    if (text[k] === '+' || text[k] === '-') k += 1;
                    if (isDigit(text[k])) {
                        while (isDigit(text[k])) k += 1;
                        j = k;
                    }
                }
                const raw = text.slice(i, j);
                tokens.push({ k: 'num', v: Number(raw), raw });
                i = j;
                continue;
            }
            if (isIdentStart(char)) {
                let j = i + 1;
                while (j < text.length && isIdentPart(text[j])) j += 1;
                tokens.push({ k: 'id', v: text.slice(i, j) });
                i = j;
                continue;
            }
            const pair = text.slice(i, i + 2);
            if (pair === '<=' || pair === '>=' || pair === '==' || pair === '!=') {
                tokens.push({ k: 'op', v: pair });
                i += 2;
                continue;
            }
            if ('+-*/^(),<>='.indexOf(char) >= 0) {
                tokens.push({ k: 'op', v: char });
                i += 1;
                continue;
            }
            throw fail('unexpected character ' + JSON.stringify(char) + ' at ' + i);
        }
        tokens.push({ k: 'eof', v: '' });
        return tokens;
    }

    function parseTokens(tokens, formula) {
        let pos = 0;
        const peek = () => tokens[pos];
        const isOp = value => peek().k === 'op' && peek().v === value;
        const take = () => tokens[pos++];

        function parseCmp() {
            const left = parseAdd();
            const token = peek();
            if (token.k === 'op' && has(COMPARISONS, token.v)) {
                take();
                return { t: 'bin', op: token.v, l: left, r: parseAdd() };
            }
            return left;
        }
        function parseAdd() {
            let left = parseMul();
            while (isOp('+') || isOp('-')) {
                const op = take().v;
                left = { t: 'bin', op, l: left, r: parseMul() };
            }
            return left;
        }
        function parseMul() {
            let left = parseUnary();
            while (isOp('*') || isOp('/')) {
                const op = take().v;
                left = { t: 'bin', op, l: left, r: parseUnary() };
            }
            return left;
        }
        function parseUnary() {
            if (isOp('-')) { take(); return { t: 'neg', a: parseUnary() }; }
            if (isOp('+')) { take(); return { t: 'pos', a: parseUnary() }; }
            return parsePow();
        }
        function parsePow() {
            const base = parseAtom();
            if (isOp('^')) {
                take();
                return { t: 'bin', op: '^', l: base, r: parseUnary() };
            }
            return base;
        }
        function parseAtom() {
            const token = take();
            if (token.k === 'num') return { t: 'num', v: token.v, raw: token.raw };
            if (token.k === 'id') {
                if (!isOp('(')) return { t: 'id', name: token.v };
                take();
                const args = [];
                if (!isOp(')')) {
                    args.push(parseCmp());
                    while (isOp(',')) { take(); args.push(parseCmp()); }
                }
                if (!isOp(')')) throw fail('")" expected after the arguments of ' + token.v);
                take();
                if (!has(ARITY, token.v)) throw fail('unknown function ' + token.v);
                const arity = ARITY[token.v];
                if (arity >= 0 ? args.length !== arity : args.length < -arity) throw fail('wrong number of arguments for ' + token.v);
                return { t: 'call', name: token.v, args };
            }
            if (token.k === 'op' && token.v === '(') {
                const inner = parseCmp();
                if (!isOp(')')) throw fail('")" expected');
                take();
                return inner;
            }
            throw fail('unexpected ' + (token.k === 'eof' ? 'end of input' : JSON.stringify(token.v)));
        }

        const first = parseCmp();
        let result = first;
        if (formula) {
            const parts = [first];
            while (isOp('=')) { take(); parts.push(parseCmp()); }
            result = { t: 'eq', parts };
        }
        if (peek().k !== 'eof') throw fail('unexpected ' + JSON.stringify(peek().v));
        return result;
    }

    const parse = source => parseTokens(tokenize(source), false);
    const parseFormula = source => parseTokens(tokenize(source), true);

    // IEEE-754 power with the special cases of the Go standard library (math.Pow): 1^y and x^0 are 1 for every y,
    // including NaN, and (-1)^(+-Inf) is 1. Math.pow yields NaN for those.
    function power(base, exponent) {
        if (base === 1 || exponent === 0) return 1;
        if (base === -1 && (exponent === Infinity || exponent === -Infinity)) return 1;
        return Math.pow(base, exponent);
    }

    // Round half away from zero (math.Round).
    function roundHalfAway(x) {
        const magnitude = Math.abs(x);
        let rounded = Math.floor(magnitude);
        if (magnitude - rounded >= 0.5) rounded += 1;
        return Math.sign(x) * rounded;
    }

    function listArgument(node, env, name) {
        if (node.t === 'id' && has(env, node.name) && Array.isArray(env[node.name])) return env[node.name];
        throw fail(name + '() needs a list variable', 'eval');
    }

    function evaluate(node, env) {
        switch (node.t) {
            case 'num': return node.v;
            case 'id': {
                if (node.name === 'pi') return Math.PI;
                if (!has(env, node.name)) throw fail('unknown identifier ' + node.name, 'eval');
                const value = env[node.name];
                if (Array.isArray(value)) throw fail('list ' + node.name + ' used outside sum() or par()', 'eval');
                return value;
            }
            case 'neg': return -evaluate(node.a, env);
            case 'pos': return evaluate(node.a, env);
            case 'bin': {
                const l = evaluate(node.l, env), r = evaluate(node.r, env);
                switch (node.op) {
                    case '+': return l + r;
                    case '-': return l - r;
                    case '*': return l * r;
                    case '/': return l / r;
                    case '^': return power(l, r);
                    case '<': return l < r ? 1 : 0;
                    case '<=': return l <= r ? 1 : 0;
                    case '>': return l > r ? 1 : 0;
                    case '>=': return l >= r ? 1 : 0;
                    case '==': return l === r ? 1 : 0;
                    case '!=': return l !== r ? 1 : 0;
                    default: throw fail('unknown operator ' + node.op, 'eval');
                }
            }
            case 'call': {
                const args = node.args;
                if (node.name === 'if') {
                    const condition = evaluate(args[0], env);
                    return condition !== 0 && !Number.isNaN(condition) ? evaluate(args[1], env) : evaluate(args[2], env);
                }
                if (node.name === 'sum') return listArgument(args[0], env, 'sum').reduce((total, x) => total + x, 0);
                if (node.name === 'par') return 1 / listArgument(args[0], env, 'par').reduce((total, x) => total + 1 / x, 0);
                const x = args.map(arg => evaluate(arg, env));
                switch (node.name) {
                    case 'sqrt': return Math.sqrt(x[0]);
                    case 'ln': return Math.log(x[0]);
                    case 'log10': return Math.log10(x[0]);
                    case 'exp': return Math.exp(x[0]);
                    case 'sin': return Math.sin(x[0]);
                    case 'cos': return Math.cos(x[0]);
                    case 'tan': return Math.tan(x[0]);
                    case 'atan': return Math.atan(x[0]);
                    case 'abs': return Math.abs(x[0]);
                    case 'round': return roundHalfAway(x[0]);
                    case 'min': return Math.min(...x);
                    case 'max': return Math.max(...x);
                    default: throw fail('unknown function ' + node.name, 'eval');
                }
            }
            default:
                throw fail('cannot evaluate a ' + node.t + ' node', 'eval');
        }
    }

    // Sorted unique identifiers that are neither function names nor the constant pi.
    function deps(node) {
        const found = new Set();
        (function walk(n) {
            switch (n.t) {
                case 'id': if (n.name !== 'pi' && !has(ARITY, n.name)) found.add(n.name); break;
                case 'neg': case 'pos': walk(n.a); break;
                case 'bin': walk(n.l); walk(n.r); break;
                case 'call': n.args.forEach(walk); break;
                case 'eq': n.parts.forEach(walk); break;
                default: break;
            }
        })(node);
        return [...found].sort();
    }

    // Parsed expressions are cached by source text: the solver evaluates the same rules on every keystroke.
    const cache = new Map();
    function parseCached(source) {
        let ast = cache.get(source);
        if (!ast) {
            ast = parse(source);
            cache.set(source, ast);
        }
        return ast;
    }

    core.expr = { ARITY, parse, parseFormula, parseCached, evaluate, deps, power, roundHalfAway };
})();
