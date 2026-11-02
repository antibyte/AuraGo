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
    const PREFIX_UNIT = 'prefix:';
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
                const linear = !unit.to;
                const toAst = linear ? null : expr.parseCached(unit.to), fromAst = linear ? null : expr.parseCached(unit.from);
                return {
                    id: unit.id, symbol: unit.symbol,
                    toBase: x => (linear ? x * unit.factor : expr.evaluate(toAst, { x })),
                    fromBase: v => (linear ? v / unit.factor : expr.evaluate(fromAst, { x: v }))
                };
            });
            dims.set(raw.id, {
                id: raw.id, symbol: raw.symbol || '', aliases: raw.aliases || [], positive: !!raw.positive, prefixes, units,
                unitById: new Map(units.map(unit => [unit.id, unit])),
                bySymbolLength: units.slice().sort((a, b) => b.symbol.length - a.symbol.length),
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

            // An explicit unit typed behind the number wins over everything else (longest symbol first).
            for (const unit of d.bySymbolLength) {
                if (text.endsWith(unit.symbol)) {
                    const rest = text.slice(0, text.length - unit.symbol.length);
                    if (NUMBER.test(rest)) return finite(unit.toBase(Number(rest)));
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

            const unit = unitId ? d.unitById.get(unitId) : null;
            if (unit && !suffixed) {
                if (prefix !== '') throw fail('a prefix needs a unit symbol in a ' + unit.symbol + ' field');
                return finite(unit.toBase(value));
            }
            if (!unit && unitId && unitId.startsWith(PREFIX_UNIT) && !suffixed && prefix === '') {
                return finite(value * prefixFactor.get(unitId.slice(PREFIX_UNIT.length)));
            }
            return finite(value * prefixFactor.get(prefix));
        }

        function finite(value) {
            if (!Number.isFinite(value)) throw fail('the value is out of range');
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
                const x = unit.fromBase(value);
                return Number.isFinite(x) ? fmtSig(x, digits) + ' ' + unit.symbol : '';
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

    core.units = { create, roundSig, fmtNum, fmtSig, PREFIX_UNIT, MAX_VALUE_LENGTH, MAX_LIST_LENGTH };
})();
