(function () {
    'use strict';

    function tokenizeFormula(formula) {
        const tokens = [];
        const input = String(formula || '').replace(/^=/, '');
        let i = 0;
        while (i < input.length) {
            const ch = input[i];
            if (/\s/.test(ch)) { i++; continue; }
            if ('+-*/(),:'.includes(ch)) { tokens.push({ type: ch, value: ch }); i++; continue; }
            if (ch === '<' || ch === '>' || ch === '!' || ch === '=') {
                let op = ch;
                i++;
                if (i < input.length && (input[i] === '=' || (ch === '<' && input[i] === '>'))) {
                    op += input[i];
                    i++;
                }
                tokens.push({ type: 'cmp', value: op });
                continue;
            }
            if (ch === '"') {
                i++;
                let str = '';
                while (i < input.length && input[i] !== '"') {
                    if (input[i] === '\\' && i + 1 < input.length) { str += input[++i]; } else { str += input[i]; }
                    i++;
                }
                if (i < input.length) i++;
                tokens.push({ type: 'string', value: str });
                continue;
            }
            if (/[0-9.]/.test(ch)) {
                const start = i;
                while (i < input.length && /[0-9.]/.test(input[i])) i++;
                if (i < input.length && /[eE]/.test(input[i])) {
                    i++;
                    if (i < input.length && /[+-]/.test(input[i])) i++;
                    while (i < input.length && /[0-9]/.test(input[i])) i++;
                }
                const value = Number(input.slice(start, i));
                if (!Number.isFinite(value)) throw new Error('invalid number');
                tokens.push({ type: 'number', value });
                continue;
            }
            if (/[A-Za-z]/.test(ch)) {
                const start = i;
                while (i < input.length && /[A-Za-z0-9]/.test(input[i])) i++;
                const value = input.slice(start, i).toUpperCase();
                tokens.push({ type: /\d/.test(value) ? 'cell' : 'ident', value });
                continue;
            }
            throw new Error('invalid token');
        }
        tokens.push({ type: 'eof', value: '' });
        return tokens;
    }

    function parseFormulaExpression(tokens, sheet) {
        let index = 0;
        const peek = () => tokens[index] || { type: 'eof' };
        const take = type => peek().type === type ? tokens[index++] : null;
        const expect = type => {
            const token = take(type);
            if (!token) throw new Error('expected ' + type);
            return token;
        };

        const comparison = () => {
            let value = expression();
            while (peek().type === 'cmp') {
                const op = tokens[index++].value;
                const right = expression();
                switch (op) {
                case '=': value = value === right ? 1 : 0; break;
                case '<>': value = value !== right ? 1 : 0; break;
                case '<': value = value < right ? 1 : 0; break;
                case '>': value = value > right ? 1 : 0; break;
                case '<=': value = value <= right ? 1 : 0; break;
                case '>=': value = value >= right ? 1 : 0; break;
                default: value = value === right ? 1 : 0;
                }
            }
            return value;
        };

        const expression = () => {
            let value = term();
            while (peek().type === '+' || peek().type === '-') {
                const op = tokens[index++].type;
                const right = term();
                value = op === '+' ? value + right : value - right;
            }
            return value;
        };
        const term = () => {
            let value = unary();
            while (peek().type === '*' || peek().type === '/') {
                const op = tokens[index++].type;
                const right = unary();
                value = op === '*' ? value * right : value / right;
            }
            return value;
        };
        const unary = () => {
            if (take('+')) return unary();
            if (take('-')) return -unary();
            return primary();
        };
        const primary = () => {
            const token = peek();
            if (take('number')) return token.value;
            if (take('string')) return token.value;
            if (take('cell')) {
                if (take(':')) return rangeValues(token.value, expect('cell').value).reduce((sum, value) => sum + value, 0);
                return cellValue(sheet, token.value);
            }
            if (take('ident')) return formulaFunction(token.value);
            if (take('(')) {
                const value = comparison();
                expect(')');
                return value;
            }
            throw new Error('invalid formula');
        };

        const cellValue = (sheetRef, ref) => {
            const pos = parseCellRef(ref);
            const cell = sheetRef && sheetRef.rows && sheetRef.rows[pos.row] && sheetRef.rows[pos.row][pos.col];
            if (!cell) return 0;
            if (cell.formula) {
                const v = evaluateFormulaForSheet(sheetRef, cell.formula);
                const n = Number(v);
                return Number.isFinite(n) ? n : v;
            }
            const num = Number(cell.value);
            return Number.isFinite(num) ? num : (cell.value || 0);
        };

        const formulaFunction = name => {
            expect('(');
            const args = [];
            if (peek().type !== ')') {
                do {
                    if (peek().type === 'cell' && tokens[index + 1] && tokens[index + 1].type === ':') {
                        const start = expect('cell').value;
                        expect(':');
                        args.push(...rangeValues(start, expect('cell').value));
                    } else if (peek().type === 'string') {
                        args.push(tokens[index++].value);
                    } else {
                        args.push(comparison());
                    }
                } while (take(','));
            }
            expect(')');

            if (name === 'SUM') return args.reduce((sum, value) => sum + (Number(value) || 0), 0);
            if (name === 'AVG' || name === 'AVERAGE') return args.reduce((sum, value) => sum + (Number(value) || 0), 0) / Math.max(1, args.length);
            if (name === 'COUNT') return args.filter(a => typeof a === 'number' || (typeof a === 'string' && a !== '')).length;
            if (name === 'COUNTA') return args.filter(a => a !== '' && a != null).length;
            if (name === 'MIN') return Math.min(...args.map(a => Number(a) || 0));
            if (name === 'MAX') return Math.max(...args.map(a => Number(a) || 0));
            if (name === 'ABS') return Math.abs(Number(args[0]) || 0);
            if (name === 'ROUND') return Number((Number(args[0]) || 0).toFixed(Math.max(0, Number(args[1]) || 0)));
            if (name === 'CEIL' || name === 'CEILING') return Math.ceil(Number(args[0]) || 0);
            if (name === 'FLOOR') return Math.floor(Number(args[0]) || 0);
            if (name === 'MEDIAN') {
                const sorted = args.map(a => Number(a) || 0).sort((a, b) => a - b);
                const mid = Math.floor(sorted.length / 2);
                return sorted.length % 2 !== 0 ? sorted[mid] : (sorted[mid - 1] + sorted[mid]) / 2;
            }
            if (name === 'STDEV') {
                const nums = args.map(a => Number(a) || 0);
                const mean = nums.reduce((s, v) => s + v, 0) / Math.max(1, nums.length);
                const variance = nums.reduce((s, v) => s + (v - mean) ** 2, 0) / Math.max(1, nums.length - 1);
                return Math.sqrt(variance);
            }
            if (name === 'IF') {
                const cond = args[0];
                const trueVal = args.length > 1 ? args[1] : 1;
                const falseVal = args.length > 2 ? args[2] : 0;
                return cond ? trueVal : falseVal;
            }
            if (name === 'AND') return args.every(a => !!a) ? 1 : 0;
            if (name === 'OR') return args.some(a => !!a) ? 1 : 0;
            if (name === 'NOT') return args[0] ? 0 : 1;
            if (name === 'CONCAT' || name === 'CONCATENATE') return args.map(a => String(a ?? '')).join('');
            if (name === 'TEXTJOIN') {
                const delim = String(args[0] ?? '');
                return args.slice(1).map(a => String(a ?? '')).join(delim);
            }
            if (name === 'LEFT') return String(args[0] || '').substring(0, Number(args[1]) || 1);
            if (name === 'RIGHT') { const s = String(args[0] || ''); return s.substring(s.length - (Number(args[1]) || 1)); }
            if (name === 'MID') return String(args[0] || '').substring((Number(args[1]) || 1) - 1, (Number(args[1]) || 1) - 1 + (Number(args[2]) || 1));
            if (name === 'LEN') return String(args[0] || '').length;
            if (name === 'UPPER') return String(args[0] || '').toUpperCase();
            if (name === 'LOWER') return String(args[0] || '').toLowerCase();
            if (name === 'TRIM') return String(args[0] || '').trim();
            if (name === 'NOW') return Date.now();
            if (name === 'TODAY') { const d = new Date(); return new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime(); }
            if (name === 'DATE') return new Date(Number(args[0]) || 1970, (Number(args[1]) || 1) - 1, Number(args[2]) || 1).getTime();
            if (name === 'YEAR') return new Date(Number(args[0]) || 0).getFullYear();
            if (name === 'MONTH') return new Date(Number(args[0]) || 0).getMonth() + 1;
            if (name === 'DAY') return new Date(Number(args[0]) || 0).getDate();
            if (name === 'VLOOKUP') {
                const lookup = args[0];
                const rangeStart = args[1];
                const colIdx = Number(args[2]) || 1;
                const exact = args[3] === 0 || args[3] === false ? false : true;
                if (!Array.isArray(rangeStart)) return '#REF!';
                for (const row of rangeStart) {
                    const first = Array.isArray(row) ? row[0] : row;
                    const match = exact ? String(first) === String(lookup) : String(first).toLowerCase().startsWith(String(lookup).toLowerCase());
                    if (match && Array.isArray(row) && row.length >= colIdx) return row[colIdx - 1];
                }
                return '#N/A';
            }
            if (name === 'HLOOKUP') {
                const lookup = args[0];
                const table = args[1];
                const rowIdx = Number(args[2]) || 1;
                if (!Array.isArray(table) || !Array.isArray(table[0])) return '#REF!';
                for (let c = 0; c < table[0].length; c++) {
                    if (String(table[0][c]) === String(lookup) && table.length >= rowIdx) return table[rowIdx - 1][c];
                }
                return '#N/A';
            }
            throw new Error('unknown function: ' + name);
        };

        const rangeValues = (start, end) => {
            const a = parseCellRef(start);
            const b = parseCellRef(end);
            if (b.row < a.row || b.col < a.col) throw new Error('bad range');
            const values = [];
            for (let r = a.row; r <= b.row; r++) {
                for (let c = a.col; c <= b.col; c++) {
                    values.push(numericCellValue(sheet, cellName(r, c)));
                }
            }
            return values;
        };

        const result = comparison();
        expect('eof');
        return result;
    }

    function evaluateFormulaForSheet(sheet, formula) {
        try {
            const value = parseFormulaExpression(tokenizeFormula(formula), sheet || { rows: [] });
            if (typeof value === 'string') return value;
            if (!Number.isFinite(value)) return '#ERR';
            return String(Number.isInteger(value) ? value : Math.round(value * 10000000000) / 10000000000);
        } catch (_) {
            return '#ERR';
        }
    }

    function numericCellValue(sheet, ref) {
        const pos = parseCellRef(ref);
        const cell = sheet && sheet.rows && sheet.rows[pos.row] && sheet.rows[pos.row][pos.col];
        if (!cell) return 0;
        if (cell.formula) return Number(evaluateFormulaForSheet(sheet, cell.formula)) || 0;
        const value = Number(cell.value);
        return Number.isFinite(value) ? value : 0;
    }

    function parseCellRef(ref) {
        const match = /^([A-Z]+)([0-9]+)$/.exec(String(ref || '').toUpperCase());
        if (!match) throw new Error('bad cell');
        let col = 0;
        for (const ch of match[1]) col = col * 26 + ch.charCodeAt(0) - 64;
        return { row: Number(match[2]) - 1, col: col - 1 };
    }

    function cellName(row, col) {
        return columnName(col + 1) + String(row + 1);
    }

    function columnName(index) {
        let name = '';
        let n = index;
        while (n > 0) {
            const mod = (n - 1) % 26;
            name = String.fromCharCode(65 + mod) + name;
            n = Math.floor((n - mod) / 26);
        }
        return name;
    }

    function rangeValuesArray(sheet, start, end) {
        const a = parseCellRef(start);
        const b = parseCellRef(end);
        if (b.row < a.row || b.col < a.col) return [];
        const rows = [];
        for (let r = a.row; r <= b.row; r++) {
            const row = [];
            for (let c = a.col; c <= b.col; c++) {
                const cell = sheet && sheet.rows && sheet.rows[r] && sheet.rows[r][c];
                if (cell) {
                    if (cell.formula) {
                        const v = evaluateFormulaForSheet(sheet, cell.formula);
                        const n = Number(v);
                        row.push(Number.isFinite(n) ? n : v);
                    } else {
                        const n = Number(cell.value);
                        row.push(Number.isFinite(n) ? n : (cell.value || ''));
                    }
                } else {
                    row.push('');
                }
            }
            rows.push(row);
        }
        return rows;
    }

    window.SheetsFormulas = {
        evaluate: evaluateFormulaForSheet,
        tokenize: tokenizeFormula,
        parseCellRef: parseCellRef,
        cellName: cellName,
        columnName: columnName,
        numericCellValue: numericCellValue,
        rangeValues: rangeValuesArray
    };
})();
