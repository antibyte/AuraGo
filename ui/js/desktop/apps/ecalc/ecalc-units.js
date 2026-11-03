/* ECalc units: dimensions, SI prefixes, input parsing ("4k7", "2u2", "10 mA", "1e-6", comma decimals) and output
   formatting (engineering notation with a configurable number of significant digits). A DOM-free classic script that
   publishes window.ECalcCore.units; it mirrors internal/ecalc/units.go and is checked against the same vectors.

   ECalcCore.units.create(catalog.units) returns the API used by the solver and the pages:
     parse(input, dim, unit) -> base value        parseList(input, dim, unit) -> [base value]
     format(value, dim, unit, sig) -> text        dim(id) -> {symbol, positive, ...}     options(dim) -> unit choices
     maxLength(list) -> the longest text a field accepts (64 characters, 256 for a list); parse and parseList reject more
   `unit` is '' (automatic prefix), the id of an explicit unit (dBm, Ah ...) or, as a display choice of the pages only,
   'prefix:<id>' for a fixed prefix (prefix:m shows milli-units, prefix: the plain unit). Every value is in SI base units. */
(function () {
    'use strict';

    const core = (window.ECalcCore = window.ECalcCore || {});
    const expr = core.expr;

    const NUMBER = /^[+-]?(\d+(?:\.\d*)?|\.\d+)([eE][+-]?\d+)?$/;
    const NUMBER_PREFIX = /^([+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?)([pnuµmkKMG])$/;
    const INFIX = /^(\d+)([pnuµmkKMG])(\d+)$/;
    const INFIX_R = /^(\d+)R(\d+)$/;
    const LIST_SEPARATOR = /[;\s]+|,(?!\d)/;
    // The text fields of the pages carry these as maxlength; parse() and parseList() reject longer text outright, which
    // also bounds the work an arbitrary string can cause. Lengths are UTF-16 code units (internal/ecalc/units.go counts
    // the same way).
    const MAX_VALUE_LENGTH = 64;
    const MAX_LIST_LENGTH = 256;
    // A parsed value is 0 or has a magnitude within these limits (MinMagnitude and MaxMagnitude in units.go): the
    // combiner, the chart axis and the E-series lookup only ever see ordinary doubles.
    const MIN_MAGNITUDE = 1e-300;
    const MAX_MAGNITUDE = 1e300;
    const PREFIX_UNIT = 'prefix:';
    // Number systems (radix units of an integer dimension): the optional prefix letter, the digit patterns (underscores
    // between digits) and the largest whole number a double holds exactly, 2^53 - 1.
    const RADIX_LETTER = { 2: 'b', 8: 'o', 16: 'x' };
    const RADIX_DIGITS = { 2: /^[01]+(?:_+[01]+)*$/, 8: /^[0-7]+(?:_+[0-7]+)*$/, 10: /^[0-9]+(?:_+[0-9]+)*$/, 16: /^[0-9a-fA-F]+(?:_+[0-9a-fA-F]+)*$/ };
    const MAX_SAFE = Number.MAX_SAFE_INTEGER;
    // U+03BC (Greek mu) is typed for the micro sign U+00B5, U+2126 (ohm sign) for the Greek capital omega U+03A9.
    const fold = text => text.split(String.fromCharCode(0x3bc)).join(String.fromCharCode(0xb5)).split(String.fromCharCode(0x2126)).join(String.fromCharCode(0x3a9));

    function fail(message) {
        const error = new Error(message);
        error.code = 'parse';
        return error;
    }

    // Round to n significant digits; fmtNum prints the shortest round-trip decimal (JS switches to an exponent below
    // 1e-6 and from 1e21, the same limits the contract sets for the Go side).
    const roundSig = (x, n) => Number(x.toPrecision(n));
    const fmtNum = x => String(x);
    const fmtSig = (x, n) => fmtNum(roundSig(x, n));

    // The digits of a whole number in the base of `unit` (see internal/ecalc/units.go parseRadix): an optional 0x, 0b or
    // 0o prefix, digits with underscores between them, no sign, at most 2^53 - 1.
    function parseRadix(unit, text) {
        let digits = text;
        const letter = RADIX_LETTER[unit.radix];
        if (letter && digits.length >= 2 && digits[0] === '0' && digits[1].toLowerCase() === letter) digits = digits.slice(2);
        if (!RADIX_DIGITS[unit.radix].test(digits)) throw fail('cannot read ' + JSON.stringify(text) + ' as a ' + unit.id + ' number');
        let value = 0;
        for (const char of digits) {
            if (char === '_') continue;
            value = value * unit.radix + parseInt(char, 16);
            if (value > MAX_SAFE) throw fail('the value ' + JSON.stringify(text) + ' is out of range (at most ' + MAX_SAFE + ')');
        }
        return value;
    }

    // Groups of `size` characters from the right, joined by underscores (1F_FFFF).
    function groupDigits(digits, size) {
        let out = '';
        for (let i = 0; i < digits.length; i += 1) {
            if (i > 0 && (digits.length - i) % size === 0) out += '_';
            out += digits[i];
        }
        return out;
    }

    // A whole number in 0 .. 2^53 - 1 written in the given base: decimal plain, hex 0xDEAD_BEEF (groups of four), binary
    // 0b1_00000000 (groups of eight, so 2^53 - 1 is 61 characters, within the 64 that parse() accepts), octal 0o377;
    // anything else gives ''. The significant-digit setting does not apply.
    function formatRadix(radix, value) {
        if (!Number.isInteger(value) || value < 0 || value > MAX_SAFE) return '';
        if (radix === 16) return '0x' + groupDigits(value.toString(16).toUpperCase(), 4);
        if (radix === 2) return '0b' + groupDigits(value.toString(2), 8);
        if (radix === 8) return '0o' + value.toString(8);
        return value.toString(10);
    }

    function create(catalogUnits) {
        const source = catalogUnits || {};
        const prefixFactor = new Map([['', 1]]);
        for (const prefix of source.prefixes || []) prefixFactor.set(prefix.id, prefix.factor);
        // The letter u stands for the micro prefix, whatever character the catalog uses for it.
        let microId = 'µ';
        for (const [id, factor] of prefixFactor) if (factor === 1e-6) microId = id;
        const inputPrefix = letter => (letter === 'u' ? microId : letter === 'K' ? 'k' : letter);

        const dims = new Map();
        for (const raw of source.dims || []) {
            const prefixes = (raw.prefixes || []).map(id => ({ id, factor: prefixFactor.has(id) ? prefixFactor.get(id) : 1 })).sort((a, b) => a.factor - b.factor);
            const units = (raw.units || []).map(unit => {
                const radix = unit.radix || 0;
                const linear = !radix && !unit.to;
                const toAst = linear || radix ? null : expr.parseCached(unit.to), fromAst = linear || radix ? null : expr.parseCached(unit.from);
                return {
                    id: unit.id, symbol: unit.symbol, aliases: unit.aliases || [], radix,
                    toBase: x => (radix ? x : linear ? x * unit.factor : expr.evaluate(toAst, { x })),
                    fromBase: v => (radix ? v : linear ? v / unit.factor : expr.evaluate(fromAst, { x: v }))
                };
            });
            dims.set(raw.id, {
                id: raw.id, symbol: raw.symbol || '', aliases: raw.aliases || [], positive: !!raw.positive, integer: !!raw.integer, prefixes, units,
                unitById: new Map(units.map(unit => [unit.id, unit])),
                // The spellings of the explicit units (symbol and aliases) that may follow a number, longest first;
                // a number system is selected, never written behind a number. Array.sort is stable.
                unitNames: units.filter(unit => !unit.radix).flatMap(unit => [unit.symbol].concat(unit.aliases).map(text => ({ text, unit }))).sort((a, b) => b.text.length - a.text.length),
                names: [raw.symbol || ''].concat(raw.aliases || []).filter(Boolean).sort((a, b) => b.length - a.length)
            });
        }

        function dim(id) {
            const found = dims.get(id);
            if (!found) throw fail('unknown dimension ' + id);
            return found;
        }

        function parse(input, dimId, unitId) {
            const d = dim(dimId);
            if (unitId && !d.unitById.has(unitId) && !(unitId.startsWith(PREFIX_UNIT) && prefixFactor.has(unitId.slice(PREFIX_UNIT.length)))) throw fail('unknown unit ' + unitId + ' for ' + dimId);
            const typed = String(input == null ? '' : input);
            if (typed.length > MAX_VALUE_LENGTH) throw fail('the input is longer than ' + MAX_VALUE_LENGTH + ' characters');
            // The Greek mu and the ohm sign are folded into the micro sign and the Greek capital omega.
            let text = fold(typed.trim().replace(/\s+/g, ''));
            if (text === '') throw fail('empty input');
            text = text.replace(/,/g, '.');

            // A selected number system reads the digits of its base and nothing else.
            const selected = unitId ? d.unitById.get(unitId) : null;
            if (selected && selected.radix) return finite(parseRadix(selected, text));

            // An explicit unit typed behind the number wins over everything else (symbol or alias, longest first).
            for (const entry of d.unitNames) {
                if (text.endsWith(entry.text)) {
                    const rest = text.slice(0, text.length - entry.text.length);
                    if (NUMBER.test(rest)) return finite(entry.unit.toBase(Number(rest)));
                }
            }
            // The base symbol or an alias may follow (4.7kΩ, 10mA, 100R); a prefix in front of it stays in the text.
            let suffixed = false;
            for (const name of d.names) {
                if (text.endsWith(name)) {
                    text = text.slice(0, text.length - name.length);
                    suffixed = true;
                    break;
                }
            }
            let value = NaN, prefix = '';
            let match;
            if (NUMBER.test(text)) value = Number(text);
            else if ((match = NUMBER_PREFIX.exec(text))) { value = Number(match[1]); prefix = inputPrefix(match[2]); }
            else if ((match = INFIX.exec(text))) { value = Number(match[1] + '.' + match[3]); prefix = inputPrefix(match[2]); }
            else if (d.aliases.includes('R') && (match = INFIX_R.exec(text))) value = Number(match[1] + '.' + match[2]);
            if (Number.isNaN(value)) throw fail('cannot read ' + JSON.stringify(input) + ' as a ' + dimId + ' value');
            if (prefix !== '' && !d.prefixes.some(entry => entry.id === prefix)) throw fail('the prefix ' + prefix + ' does not fit ' + dimId);

            const unit = selected;
            if (unit && !suffixed) {
                if (prefix !== '') throw fail('a prefix needs a unit symbol in a ' + unit.symbol + ' field');
                return finite(unit.toBase(value));
            }
            if (!unit && unitId && unitId.startsWith(PREFIX_UNIT) && !suffixed && prefix === '') {
                return finite(value * prefixFactor.get(unitId.slice(PREFIX_UNIT.length)));
            }
            return finite(value * prefixFactor.get(prefix));
        }

        // A parsed value is finite and 0 or of a magnitude within MIN_MAGNITUDE .. MAX_MAGNITUDE (after the unit conversion).
        function finite(value) {
            const magnitude = Math.abs(value);
            if (!Number.isFinite(value) || (magnitude !== 0 && (magnitude < MIN_MAGNITUDE || magnitude > MAX_MAGNITUDE))) throw fail('the value is out of range');
            return value;
        }

        // List fields: the items are separated by semicolons, white space and commas that are not followed by a digit
        // (a comma in front of a digit is a decimal comma: "4,7k, 10k" is two items); every item is parsed like parse().
        function parseList(input, dimId, unitId) {
            const typed = String(input == null ? '' : input);
            if (typed.length > MAX_LIST_LENGTH) throw fail('the list is longer than ' + MAX_LIST_LENGTH + ' characters');
            const items = typed.trim().split(LIST_SEPARATOR).filter(Boolean);
            if (!items.length) throw fail('empty list');
            return items.map(item => parse(item, dimId, unitId));
        }

        function format(value, dimId, unitId, sig) {
            const d = dims.get(dimId);
            if (!d || !Number.isFinite(value)) return '';
            const digits = sig || 4;
            const unit = unitId ? d.unitById.get(unitId) : null;
            if (unit) {
                if (unit.radix) return formatRadix(unit.radix, value);
                const x = unit.fromBase(value);
                return Number.isFinite(x) ? fmtSig(x, digits) + ' ' + unit.symbol : '';
            }
            // No number system selected: decimal digits (never rounded to `digits`) when value is a whole number in range.
            if (d.integer) {
                const text = formatRadix(10, value);
                if (text) return text;
            }
            if (unitId && unitId.startsWith(PREFIX_UNIT)) {
                const id = unitId.slice(PREFIX_UNIT.length);
                if (prefixFactor.has(id)) return (fmtSig(value / prefixFactor.get(id), digits) + ' ' + id + d.symbol).trim();
            }
            if (!d.prefixes.length) return fmtSig(value, digits) + (d.symbol ? ' ' + d.symbol : '');
            if (value === 0) return '0' + (d.symbol ? ' ' + d.symbol : '');
            let index = -1;
            d.prefixes.forEach((entry, i) => { if (entry.factor <= Math.abs(value)) index = i; });
            if (index < 0) index = 0;
            let mantissa = roundSig(value / d.prefixes[index].factor, digits);
            if (Math.abs(mantissa) >= 1000 && index + 1 < d.prefixes.length) {
                index += 1;
                mantissa = roundSig(value / d.prefixes[index].factor, digits);
            }
            return (fmtNum(mantissa) + ' ' + d.prefixes[index].id + d.symbol).trim();
        }

        // The choices of a unit dropdown: automatic, the fixed prefixes, then the explicit units of the dimension.
        function options(dimId) {
            const d = dim(dimId);
            const list = [{ id: '', label: '', kind: 'auto' }];
            for (const entry of d.prefixes) list.push({ id: PREFIX_UNIT + entry.id, label: entry.id + d.symbol, kind: 'prefix' });
            for (const unit of d.units) list.push({ id: unit.id, label: unit.symbol, kind: 'explicit' });
            return d.prefixes.length || d.units.length ? list : [list[0]];
        }

        // The longest text a field of a single value (or of a list) accepts.
        const maxLength = list => (list ? MAX_LIST_LENGTH : MAX_VALUE_LENGTH);

        return { dim, parse, parseList, format, options, maxLength, roundSig, fmtNum, fmtSig };
    }

    core.units = { create, roundSig, fmtNum, fmtSig, PREFIX_UNIT, MAX_VALUE_LENGTH, MAX_LIST_LENGTH, MIN_MAGNITUDE, MAX_MAGNITUDE };
})();
