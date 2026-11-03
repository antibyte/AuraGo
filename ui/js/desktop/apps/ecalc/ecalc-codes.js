/* ECalc component codes: reading and writing the markings of resistors, inductors and capacitors (resistor and inductor
   colour bands, SMD resistor codes, capacitor codes, ceramic temperature codes). A DOM-free classic script that publishes
   window.ECalcCore.codes = { decode(item, code, units), encode(item, req, units) }; it mirrors internal/ecalc/codes*.go and
   is checked against the same vectors (internal/ecalc/testdata/vectors/codes.json, scripts/test-ecalc-codes.mjs).

   `item` is a code item of the catalog JSON (kind "code", code = the code kind, data = the tables of that kind in the
   language of the catalog), `units` an ECalcCore.units instance. Both functions return
   { kind, code, bands, value, dim, fields: [{id, name, text, value, dim}], warnings } with value null when the kind has none;
   the names and the warnings are English. They throw an Error whose `code` is 'invalid_code' (the code to decode is
   malformed), 'invalid_value' (an encode field cannot be read) or 'unsupported' (well formed, but not decodable or not
   codable) and whose `message` is the plain message. Load after ecalc-units.js and ecalc-eseries.js. */
(function () {
    'use strict';

    const core = (window.ECalcCore = window.ECalcCore || {});
    const MAX_TEXT = 64;

    function fail(code, message) {
        const error = new Error(message);
        error.code = code;
        return error;
    }
    const invalidCode = message => fail('invalid_code', message);
    const invalidValue = message => fail('invalid_value', message);
    const unsupported = message => fail('unsupported', message);

    // ---- numbers ---------------------------------------------------------------------------------------------------------
    const fmtNum = x => core.units.fmtNum(x);
    const fmtSig = (x, n) => core.units.fmtSig(x, n);
    const roundSig = (x, n) => core.units.roundSig(x, n);
    const closeTo = (a, b) => a === b || Math.abs(a - b) <= 1e-9 * Math.max(Math.abs(a), Math.abs(b));

    // n * 10^exp as the double nearest to that decimal.
    const decimal = (n, exp) => Number(n + 'e' + exp);
    // The number "whole.frac" times 10^exp for digit strings (whole may be empty).
    const decimalText = (whole, frac, exp) => Number((whole === '' ? '0' : whole) + (frac !== '' ? '.' + frac : '') + 'e' + exp);
    // x * 10^exp as the double nearest to that decimal (x is a short decimal such as 0.25 or 50).
    function scaleDec(x, exp) {
        const a = Math.abs(x);
        if (a === 0 || a < 1e-6 || a >= 1e21) return x * Math.pow(10, exp);
        return Number(String(x) + 'e' + exp);
    }

    // v (> 0) with n significant digits: the integer N of n digits, the exponent e with v = N * 10^e, and whether v equals
    // that number to within 1e-9 relative (so that it is not a rounded value).
    function sigDigits(v, n) {
        const text = v.toExponential(n - 1);
        const at = text.indexOf('e');
        const N = parseInt(text.slice(0, at).replace('.', ''), 10);
        const e = parseInt(text.slice(at + 1), 10) - (n - 1);
        return { N, e, exact: Math.abs(decimal(N, e) - v) <= 1e-9 * v };
    }

    // Whether v, written with 3 significant digits, is a value of the E-series (any decade).
    function seriesHas(series, v) {
        const s = sigDigits(v, 3);
        return s.exact && core.eseries.TABLES[series].some(m => Math.round(m * 100) === s.N);
    }

    function seriesForTolerance(tol) {
        if (tol >= 20) return 'E6';
        if (tol >= 10) return 'E12';
        if (tol >= 5) return 'E24';
        if (tol >= 2) return 'E48';
        if (tol >= 1) return 'E96';
        return 'E192';
    }

    // A tolerance for a value without one: 5 % for an E24 value, 1 % for E96, 0.5 % for E192, otherwise 5 %.
    function assumeTolerance(v) {
        if (seriesHas('E24', v)) return 5;
        if (seriesHas('E96', v)) return 1;
        if (seriesHas('E192', v)) return 0.5;
        return 5;
    }

    // The standard value for tolerance tol (percent) nearest to v: a value of the series that tol implies or, below 5 %, of E24.
    function snapToSeries(v, tol) {
        const first = core.eseries.nearest(seriesForTolerance(tol), v);
        if (!first) throw unsupported('the value is outside the range of the code');
        let best = first.value;
        if (tol < 5) {
            const alt = core.eseries.nearest('E24', v);
            if (alt) {
                const da = Math.abs(alt.value - v), db = Math.abs(best - v);
                if (da < db || (da === db && alt.value < best)) best = alt.value;
            }
        }
        return { value: best, changed: Math.abs(best - v) > 1e-9 * v };
    }

    function signedText(x, sig) {
        if (x === 0) return '0';
        return x < 0 ? '−' + fmtSig(-x, sig) : '+' + fmtSig(x, sig);
    }

    function snapWarning(units, dim, input, output, tol) {
        let label = seriesForTolerance(tol);
        if (tol < 5) label += ' or E24';
        return units.format(input, dim, '', 4) + ' is not a standard value for ±' + fmtSig(tol, 6) + ' % (' + label + '); encoded the nearest, ' +
            units.format(output, dim, '', 4) + ' (' + signedText((output - input) / input * 100, 2) + ' %)';
    }

    // ---- reading request fields ---------------------------------------------------------------------------------------------
    const PERCENT = /^\+?(\d+(?:\.\d+)?|\.\d+)%?$/;
    const ASYM = /^([+\-−])(\d+(?:\.\d+)?)\/([+\-−])(\d+(?:\.\d+)?)%?$/;
    const compact = text => text.replace(/\s+/g, '').replace(/,/g, '.');

    function parsePercent(text) {
        const m = PERCENT.exec(compact(text).replace(/^±/, ''));
        return m ? Number(m[1]) : null;
    }

    // "-20/+80", "+22/-56 %": the sizes of the upward and the downward deviation, or null.
    function parseAsym(text) {
        const m = ASYM.exec(compact(text));
        if (!m || (m[1] === '+') === (m[3] === '+')) return null;
        const a = Number(m[2]), b = Number(m[4]);
        return m[1] === '+' ? { plus: a, minus: b } : { plus: b, minus: a };
    }

    // A temperature coefficient ("50", "50ppm", "±50 ppm/K") in 1/K.
    function parseTempco(units, text) {
        let t = text.trim().replace(/^±/, '');
        if (t.toLowerCase().endsWith('ppm')) t = t.slice(0, -3) + 'ppm/K';
        return units.parse(t, 'tempco', 'ppmK');
    }

    const splitWords = text => text.toLowerCase().split(/[\s,;_\-–—]+/).filter(word => word !== '');
    const blank = text => String(text == null ? '' : text).trim() === '';
    const parseFail = (field, error) => invalidValue(field + ': ' + error.message);

    // ---- results ------------------------------------------------------------------------------------------------------------
    function newResult(kind) {
        return { kind, code: '', bands: [], value: NaN, dim: '', fields: [], warnings: [] };
    }
    function addField(res, id, name, text, value, dim) {
        res.fields.push({ id, name, text, value, dim });
    }
    const nullIfNaN = x => (Number.isNaN(x) ? null : x);
    function finish(res) {
        return Object.assign({}, res, { value: nullIfNaN(res.value), fields: res.fields.map(f => Object.assign({}, f, { value: nullIfNaN(f.value) })) });
    }
    function toleranceField(res, tol) {
        addField(res, 'tolerance', 'Tolerance', '±' + fmtSig(tol, 6) + ' %', scaleDec(tol, -2), 'ratio');
    }
    function tcrField(res, ppm) {
        addField(res, 'tcr', 'Temperature coefficient', '±' + fmtSig(ppm, 6) + ' ppm/K', scaleDec(ppm, -6), 'tempco');
    }
    function assumedWarning(res, tol) { res.warnings.push('no tolerance given; ±' + fmtSig(tol, 6) + ' % assumed'); }

    // The tolerance of an encode request: the percentage given, or one assumed from the value.
    function requestedTolerance(req, v) {
        if (blank(req.tolerance)) return { tol: assumeTolerance(v), assumed: true };
        const p = parsePercent(req.tolerance);
        if (p === null || !(p > 0)) throw invalidValue('tolerance: cannot read ' + JSON.stringify(req.tolerance) + ' as a percentage');
        return { tol: p, assumed: false };
    }

    // ---- resistor_bands -------------------------------------------------------------------------------------------------------------
    const ROLE_LABEL = { digit: 'a digit', multiplier: 'a multiplier', tolerance: 'a tolerance', tcr: 'a temperature coefficient' };
    const indexes = new WeakMap();

    function resistorIndex(item) {
        let ix = indexes.get(item);
        if (ix) return ix;
        const data = item.data;
        ix = { data, byName: new Map(), byID: new Map(), byDigit: new Map(), byExp: new Map(), layout: new Map() };
        for (const c of data.colors) {
            ix.byID.set(c.id, c);
            if (c.digit !== null) ix.byDigit.set(c.digit, c);
            if (c.exp !== null) ix.byExp.set(c.exp, c);
            if (c.id === 'none') continue;
            ix.byName.set(c.id, c);
            if (c.iec) ix.byName.set(c.iec.toLowerCase(), c);
            if (c.id === 'grey') ix.byName.set('gray', c);
        }
        for (const l of data.layouts) ix.layout.set(l.bands, l);
        indexes.set(item, ix);
        return ix;
    }

    function bandsOf(ix, code) {
        const words = splitWords(code);
        if (!words.length) throw invalidCode('the code is empty');
        return words.map(word => {
            const c = ix.byName.get(word);
            if (!c) {
                const ids = [...ix.byName.entries()].filter(([name, colour]) => name === colour.id).map(([name]) => name).sort();
                throw invalidCode('unknown colour ' + JSON.stringify(word) + ' (colours: ' + ids.join(', ') + ')');
            }
            return c;
        });
    }

    // How older charts read a tolerance colour that IEC 60062:2016 assigned later ('' when they agree).
    function legacyWarning(c) {
        if (c.legacy_tol !== null) return c.id + ' tolerance band: ±' + fmtSig(c.tol, 6) + ' % in IEC 60062:2016; older charts and some makers read ±' + fmtSig(c.legacy_tol, 6) + ' %';
        if (c.legacy_tol_none) return c.id + ' tolerance band: ±' + fmtSig(c.tol, 6) + ' % in IEC 60062:2016; older charts give ' + c.id + ' no tolerance';
        return '';
    }

    const JUMPER_WARNING = 'a single black band marks a zero-ohm jumper (no tolerance)';

    function decodeResistor(item, code) {
        const ix = resistorIndex(item);
        const cs = bandsOf(ix, code);
        const res = newResult('resistor_bands');
        res.dim = 'resistance';
        res.bands = cs.map(c => c.id);
        res.code = res.bands.join(' ');
        if (cs.length === 1) {
            if (cs[0].id !== 'black') throw invalidCode('a single band must be black (a zero-ohm jumper)');
            res.value = 0;
            res.warnings.push(JUMPER_WARNING);
            return res;
        }
        const layout = ix.layout.get(cs.length);
        if (!layout) throw invalidCode('a resistor code has 3 to 6 bands (or one black band), got ' + cs.length);
        let digits = 0, exp = 0, tol = null, tcr = null;
        cs.forEach((c, i) => {
            const role = layout.roles[i];
            const ok = role === 'digit' ? c.digit !== null : role === 'multiplier' ? c.exp !== null : role === 'tolerance' ? c.tol !== null : c.tcr !== null;
            if (!ok) {
                if (i === 0) throw invalidCode('the first band cannot be ' + c.id + ' (gold, silver and pink never stand first); the bands may be read from the wrong end');
                throw invalidCode('band ' + (i + 1) + ' (' + c.id + ') cannot be ' + ROLE_LABEL[role]);
            }
            if (role === 'digit') digits = digits * 10 + c.digit;
            else if (role === 'multiplier') exp = c.exp;
            else if (role === 'tolerance') tol = c;
            else tcr = c;
        });
        res.value = decimal(digits, exp);
        if (tol === null) toleranceField(res, ix.byID.get('none').tol);
        else {
            toleranceField(res, tol.tol);
            const w = legacyWarning(tol);
            if (w) res.warnings.push(w);
        }
        if (tcr !== null) tcrField(res, tcr.tcr);
        if (res.value > 0 && !seriesHas('E24', res.value) && !seriesHas('E192', res.value)) res.warnings.push('the value is not an E24 or E192 value; check the reading direction');
        return res;
    }

    function toleranceList(ix) {
        return ix.data.colors.filter(c => c.tol !== null).map(c => c.tol).sort((a, b) => a - b).map(v => fmtSig(v, 6)).join(', ');
    }

    function encodeResistor(item, req, units) {
        const ix = resistorIndex(item);
        let v;
        try { v = units.parse(req.value, 'resistance', ''); } catch (error) { throw parseFail('value', error); }
        if (v < 0) throw invalidValue('value: the resistance must not be negative');
        const res = newResult('resistor_bands');
        res.dim = 'resistance';
        if (v === 0) {
            res.bands = ['black'];
            res.code = 'black';
            res.value = 0;
            res.warnings.push(JUMPER_WARNING);
            return res;
        }
        let bands = req.bands || 0;
        if (bands !== 0 && !ix.layout.has(bands)) throw invalidValue('bands must be 0 (choose), 3, 4, 5 or 6');

        const { tol, assumed } = bands === 3 && blank(req.tolerance) ? { tol: 20, assumed: true } : requestedTolerance(req, v); // the 3-band code without a tolerance band is ±20 %
        const tolC = ix.data.colors.filter(c => c.tol !== null && closeTo(c.tol, tol)).pop() || null;
        if (!tolC) throw unsupported('no band colour for a tolerance of ±' + fmtSig(tol, 6) + ' % (tolerances: ' + toleranceList(ix) + ')');

        let tcrC = null;
        if (!blank(req.tempco)) {
            let x;
            try { x = parseTempco(units, req.tempco); } catch (error) { throw parseFail('tempco', error); }
            tcrC = ix.data.colors.filter(c => c.tcr !== null && closeTo(scaleDec(c.tcr, -6), x)).pop() || null;
            if (!tcrC) throw unsupported('no band colour for a temperature coefficient of ' + fmtSig(x / 1e-6, 6) + ' ppm/K');
            if (bands !== 0 && bands !== 6) throw invalidValue('a temperature coefficient needs the 6-band code');
            bands = 6;
        }
        if (bands === 6 && tcrC === null) throw invalidValue('the 6-band code needs a temperature coefficient (tempco)');
        const noTol = tolC.id === 'none';
        if (bands === 3 && !noTol) throw invalidValue('the 3-band code has no tolerance band (±20 %); give a tolerance of 20% or use 4 bands');
        if (noTol) {
            if (bands > 3) throw unsupported('±20 % has no tolerance colour; the code without a tolerance band has 3 bands');
            bands = 3;
        }

        const snap = snapToSeries(v, tol);
        const vs = snap.value;
        const two = sigDigits(vs, 2).exact;
        if (bands === 0) bands = two ? 4 : 5;
        let n = 3;
        if (bands === 3 || bands === 4) {
            n = 2;
            if (!two) throw unsupported(units.format(vs, 'resistance', '', 4) + ' needs 3 significant digits; a ' + bands + '-band code holds 2 (use 5 or 6 bands)');
        }
        const s = sigDigits(vs, n);
        if (!s.exact) throw unsupported(units.format(vs, 'resistance', '', 4) + ' needs more than ' + n + ' significant digits');
        const mult = ix.byExp.get(s.e);
        if (!mult) {
            const lo = decimal(Math.pow(10, n - 1), -3), hi = decimal(Math.pow(10, n) - 1, 9);
            throw unsupported(units.format(vs, 'resistance', '', 4) + ' is outside the range of a ' + bands + '-band code (' + units.format(lo, 'resistance', '', 4) + ' to ' + units.format(hi, 'resistance', '', 4) + ')');
        }
        for (const ch of String(s.N).padStart(n, '0')) res.bands.push(ix.byDigit.get(Number(ch)).id);
        res.bands.push(mult.id);
        if (!noTol) res.bands.push(tolC.id);
        if (tcrC !== null) res.bands.push(tcrC.id);
        res.code = res.bands.join(' ');
        res.value = decimal(s.N, s.e);
        toleranceField(res, tol);
        if (tcrC !== null) tcrField(res, tcrC.tcr);
        if (assumed) assumedWarning(res, tol);
        if (snap.changed) res.warnings.push(snapWarning(units, 'resistance', v, vs, tol));
        if (!noTol) {
            const w = legacyWarning(tolC);
            if (w) res.warnings.push(w);
        }
        return res;
    }

    // ---- inductor_bands ----------------------------------------------------------------------------------------------------------------
    const LAYOUT_EIA = 'EIA (4 bands)';
    const LAYOUT_MIL = 'MIL-PRF-15305 (5 bands, wide silver first)';

    function inductorIndex(item) {
        let ix = indexes.get(item);
        if (ix) return ix;
        ix = { data: item.data, byName: new Map(), byDigit: new Map(), byEIAExp: new Map(), byMILExp: new Map(), identifier: null, point: null };
        for (const c of item.data.colors) {
            ix.byName.set(c.id, c);
            if (c.id === 'grey') ix.byName.set('gray', c);
            if (c.digit !== null) ix.byDigit.set(c.digit, c);
            if (c.eia_exp !== null) ix.byEIAExp.set(c.eia_exp, c);
            if (c.mil_exp !== null) ix.byMILExp.set(c.mil_exp, c);
            if (c.mil_role === 'point') ix.point = c;
            if (c.mil_role === 'identifier') ix.identifier = c;
        }
        indexes.set(item, ix);
        return ix;
    }

    function decodeInductor(item, code) {
        const ix = inductorIndex(item);
        const words = splitWords(code);
        if (!words.length) throw invalidCode('the code is empty');
        const cs = words.map(word => {
            const c = ix.byName.get(word);
            if (!c) throw invalidCode('unknown colour ' + JSON.stringify(word));
            return c;
        });
        const res = newResult('inductor_bands');
        res.dim = 'inductance';
        res.bands = cs.map(c => c.id);
        res.code = res.bands.join(' ');
        const digit = i => {
            if (cs[i].digit === null) throw invalidCode('band ' + (i + 1) + ' (' + cs[i].id + ') cannot be a digit');
            return cs[i].digit;
        };
        const toleranceError = c => unsupported('the tolerance colour ' + c.id + ' is not cross-checked for inductors (gold ±5 %, silver ±10 %)');
        let value, tol;
        if (cs.length === 4) { // EIA: digit, digit, multiplier, tolerance
            const d1 = digit(0), d2 = digit(1);
            if (cs[2].eia_exp === null) throw invalidCode('band 3 (' + cs[2].id + ') cannot be a multiplier (black, brown, red, orange, gold or silver)');
            if (cs[3].eia_tol === null) throw toleranceError(cs[3]);
            value = decimal(d1 * 10 + d2, cs[2].eia_exp - 6);
            tol = cs[3].eia_tol;
            addField(res, 'layout', 'Layout', LAYOUT_EIA, NaN, '');
        } else if (cs.length === 5) { // military: identifier, three value bands, tolerance
            if (cs[0] !== ix.identifier) throw invalidCode('a 5-band inductor code starts with the wide ' + ix.identifier.id + ' military band');
            if (cs[4].mil_tol === null) throw toleranceError(cs[4]);
            tol = cs[4].mil_tol;
            if ((cs[1] === ix.point && cs[2] === ix.point) || cs[3] === ix.point) {
                throw invalidCode(ix.point.id + ' is the decimal point: only band 2 or 3 can be ' + ix.point.id + ', not both and not band 4');
            } else if (cs[1] === ix.point) { // 0.dd
                const a = digit(2), b = digit(3);
                value = decimal(a * 10 + b, -2 - 6);
            } else if (cs[2] === ix.point) { // d.d
                const a = digit(1), b = digit(3);
                value = decimal(a * 10 + b, -1 - 6);
            } else { // dd x multiplier
                const a = digit(1), b = digit(2);
                if (cs[3].mil_exp === null) throw invalidCode('band 4 (' + cs[3].id + ') cannot be a multiplier (black, brown, red or orange)');
                value = decimal(a * 10 + b, cs[3].mil_exp - 6);
            }
            addField(res, 'layout', 'Layout', LAYOUT_MIL, NaN, '');
        } else {
            throw invalidCode('an inductor code has 4 bands (EIA) or 5 bands (military, wide silver first), got ' + cs.length);
        }
        if (value === 0) throw invalidCode('the bands give an inductance of zero');
        res.value = value;
        toleranceField(res, tol);
        return res;
    }

    function encodeInductor(item, req, units) {
        const ix = inductorIndex(item);
        let v;
        try { v = units.parse(req.value, 'inductance', ''); } catch (error) { throw parseFail('value', error); }
        if (!(v > 0)) throw invalidValue('value: the inductance must be positive');
        const bands = req.bands || 0;
        if (bands !== 0 && bands !== 4 && bands !== 5) throw invalidValue('bands must be 4 (EIA) or 5 (military)');
        const mil = bands === 5;
        const res = newResult('inductor_bands');
        res.dim = 'inductance';
        const { tol, assumed } = requestedTolerance(req, v);
        const tolC = ix.data.colors.filter(c => c.eia_tol !== null && c.mil_tol !== null && closeTo(c.eia_tol, tol)).pop() || null;
        if (!tolC) throw unsupported('no band colour for a tolerance of ±' + fmtSig(tol, 6) + ' % (gold ±5 %, silver ±10 %)');
        const snap = snapToSeries(v, tol);
        const vs = snap.value;
        const s = sigDigits(vs, 2);
        if (!s.exact) throw unsupported(units.format(vs, 'inductance', '', 4) + ' needs more than 2 significant digits');
        const micro = s.e + 6; // N * 10^micro is the value in µH
        const a = ix.byDigit.get(Math.floor(s.N / 10)).id, b = ix.byDigit.get(s.N % 10).id;
        const range = units.format(decimal(10, -8), 'inductance', '', 4) + ' to ' + units.format(decimal(99, -3), 'inductance', '', 4);
        if (!mil) {
            const mult = ix.byEIAExp.get(micro);
            if (!mult) throw unsupported(units.format(vs, 'inductance', '', 4) + ' is outside the range of the EIA code (' + range + ')');
            res.bands = [a, b, mult.id, tolC.id];
            addField(res, 'layout', 'Layout', LAYOUT_EIA, NaN, '');
        } else {
            if (micro >= 0 && micro <= 3) res.bands = [ix.identifier.id, a, b, ix.byMILExp.get(micro).id, tolC.id];
            else if (micro === -1) res.bands = [ix.identifier.id, a, ix.point.id, b, tolC.id]; // d.d
            else if (micro === -2) res.bands = [ix.identifier.id, ix.point.id, a, b, tolC.id]; // 0.dd
            else throw unsupported(units.format(vs, 'inductance', '', 4) + ' is outside the range of the military code (' + range + ')');
            addField(res, 'layout', 'Layout', LAYOUT_MIL, NaN, '');
        }
        res.code = res.bands.join(' ');
        res.value = decimal(s.N, s.e);
        toleranceField(res, tol);
        if (assumed) assumedWarning(res, tol);
        if (snap.changed) res.warnings.push(snapWarning(units, 'inductance', v, vs, tol));
        return res;
    }

    // ---- smd_resistor ----------------------------------------------------------------------------------------------------------------------
    const SMD_EIA96 = /^(\d\d)([A-Z])$/;
    const SMD_RKM = /^(\d*)([LRKMG])(\d*)$/;
    const SMD_DIGITS = /^\d{3,4}$/;
    const SMD_ZERO = /^(0{1,4}|0R0)$/;

    function smdIndex(item) {
        let ix = indexes.get(item);
        if (ix) return ix;
        const data = item.data;
        ix = { data, letter: new Map(data.letters.map(l => [l.letter, l])), rkm: new Map(data.rkm.map(l => [l.letter, l])), amb: new Set(data.ambiguous.map(a => a.letter)) };
        indexes.set(item, ix);
        return ix;
    }

    function decodeSMD(item, code, units) {
        const ix = smdIndex(item);
        const s = code.replace(/\s+/g, '').toUpperCase().replace(/[_|]/g, '');
        if (s === '') throw invalidCode('the code is empty');
        const res = newResult('smd_resistor');
        res.dim = 'resistance';
        res.code = s;
        const family = text => addField(res, 'family', 'Marking', text, NaN, '');
        if (SMD_ZERO.test(s)) {
            res.value = 0;
            family('zero-ohm jumper');
            return res;
        }
        let m = SMD_EIA96.exec(s);
        if (m && !ix.rkm.has(m[2])) { // two digits and a letter: EIA-96, unless the letter is an RKM letter (10R, 47K, 15L)
            const idx = parseInt(m[1], 10);
            if (ix.amb.has(m[2])) throw unsupported('the EIA-96 letter ' + m[2] + ' is ambiguous between makers (conflicting meanings); it is not decoded');
            const l = ix.letter.get(m[2]);
            if (!l) throw invalidCode(m[2] + ' is not an EIA-96 multiplier letter');
            if (idx < 1 || idx > 96) throw invalidCode('the EIA-96 index must be 01 to 96, got ' + m[1]);
            res.value = decimal(ix.data.eia96[idx - 1], l.exp);
            family('EIA-96');
            if (l.rare) res.warnings.push('the letter ' + l.letter + ' (×' + fmtNum(decimal(1, l.exp)) + ') is only used by some makers');
            return res;
        }
        m = SMD_RKM.exec(s);
        if (m && m[1].length + m[3].length > 0) {
            if (m[1].length + m[3].length > 4) throw invalidCode('an RKM code has at most 4 digits');
            const l = ix.rkm.get(m[2]);
            res.value = decimalText(m[1], m[3], l.exp);
            family(l.letter === 'L' || l.letter === 'R' ? 'R notation' : 'RKM notation');
            const idx = parseInt(m[1], 10);
            if (l.letter === 'R' && m[1].length === 2 && m[3] === '' && idx >= 1 && idx <= 96) {
                res.warnings.push('R after two digits is read as the RKM decimal point (' + s + ' = ' + units.format(res.value, 'resistance', '', 4) + '); some makers use R as an EIA-96 multiplier letter, which is not decoded');
            }
            return res;
        }
        if (SMD_DIGITS.test(s)) {
            const n = s.length === 4 ? 3 : 2;
            res.value = decimal(parseInt(s.slice(0, n), 10), parseInt(s.slice(n), 10));
            family(s.length + '-digit code');
            return res;
        }
        throw invalidCode('cannot read ' + JSON.stringify(s) + ' as an SMD resistor code (3 or 4 digits, R notation, RKM or EIA-96)');
    }

    // The 3-character code of v ("472", "4R7", "R47", "R05", "1L5"), '' when it has none.
    function code3(v) {
        const { N, e, exact } = sigDigits(v, 2);
        if (!exact) return '';
        const tens = Math.floor(N / 10), ones = N % 10;
        if (e >= 0 && e <= 9) return String(N) + e;
        if (e === -1) return tens + 'R' + ones;
        if (e === -2) return 'R' + N;
        if (e === -3 && ones === 0) return 'R0' + tens;
        if (e === -4) return tens + 'L' + ones;
        return '';
    }

    // The 4-character code of v ("1001", "10R0", "4R99", "R499", "R020"), '' when it has none.
    function code4(v) {
        const { N, e, exact } = sigDigits(v, 3);
        if (!exact) return '';
        if (e >= 0 && e <= 9) return String(N) + e;
        if (e === -1) return Math.floor(N / 10) + 'R' + (N % 10);
        if (e === -2) return Math.floor(N / 100) + 'R' + String(N % 100).padStart(2, '0');
        if (e === -3) return 'R' + String(N).padStart(3, '0');
        if (e === -4 && N % 10 === 0) return 'R' + String(N / 10).padStart(3, '0');
        return '';
    }

    // The EIA-96 code of v ("01C"), '' when v is not an E96 value from 1.00 ohm to 97.6 Mohm.
    function code96(ix, v) {
        const { N, e, exact } = sigDigits(v, 3);
        if (!exact) return '';
        const at = ix.data.eia96.indexOf(N);
        if (at < 0) return '';
        const l = ix.data.letters.find(x => x.exp === e && !x.rare);
        return l ? String(at + 1).padStart(2, '0') + l.letter : '';
    }

    function encodeSMD(item, req, units) {
        const ix = smdIndex(item);
        let v;
        try { v = units.parse(req.value, 'resistance', ''); } catch (error) { throw parseFail('value', error); }
        if (v < 0) throw invalidValue('value: the resistance must not be negative');
        const res = newResult('smd_resistor');
        res.dim = 'resistance';
        if (v === 0) {
            res.code = '000';
            res.value = 0;
            addField(res, 'family', 'Marking', 'zero-ohm jumper', NaN, '');
            addField(res, 'code_4digit', '4-digit code', '0000', NaN, '');
            return res;
        }
        const { tol, assumed } = requestedTolerance(req, v);
        const snap = snapToSeries(v, tol);
        const vs = snap.value;
        const c3 = code3(vs), c4 = code4(vs), c96 = code96(ix, vs);
        let primary = c3, family = '3-digit code';
        if ((tol <= 1 && c4 !== '') || c3 === '') { primary = c4; family = '4-digit code'; }
        if (primary === '') throw unsupported(units.format(vs, 'resistance', '', 4) + ' has no 3-digit or 4-digit marking');
        const s = sigDigits(vs, 3);
        res.code = primary;
        res.value = decimal(s.N, s.e);
        addField(res, 'family', 'Marking', family, NaN, '');
        if (c3 !== '') addField(res, 'code_3digit', '3-digit code', c3, NaN, '');
        if (c4 !== '') addField(res, 'code_4digit', '4-digit code', c4, NaN, '');
        if (c96 !== '') addField(res, 'code_eia96', 'EIA-96 code', c96, NaN, '');
        toleranceField(res, tol);
        if (assumed) assumedWarning(res, tol);
        if (snap.changed) res.warnings.push(snapWarning(units, 'resistance', v, vs, tol));
        return res;
    }

    // ---- capacitor ----------------------------------------------------------------------------------------------------------------------------------
    const CAP_RKM = /^(\d+)([pnuµm])(\d*)([A-Za-z]?)$/;
    const CAP_RKM_UPPER = /^(\d+)([PNU])(\d+)([A-Za-z]?)$/;
    const CAP_R = /^(\d?)R(\d{1,2})([A-Z]?)$/;
    const CAP_DIGITS = /^(\d{3})([A-Z]?)$/;
    const CAP_VOLTAGE = /^\d[A-Z]$/;
    const CAP_LETTER = /^[A-Z]$/;
    const CAP_PAIR = /^[A-Z]{2}$/;
    const TEN_PF = decimal(10, -12);
    const VOLTAGE_WARNING = 'voltage codes are a maker convention, not part of IEC 60062; check the datasheet (for example Murata writes YA for 35 V)';

    function capacitorIndex(item) {
        let ix = indexes.get(item);
        if (ix) return ix;
        const data = item.data;
        ix = { data, mult: new Map(data.multipliers.map(m => [m.digit, m])), rkm: new Map(), voltage: new Map(data.voltages.map(v => [v.code, v])) };
        for (const r of data.rkm) for (const letter of r.letters) ix.rkm.set(letter, r.exp);
        indexes.set(item, ix);
        return ix;
    }

    // The tolerance rows of a letter that apply to the capacitance (when known).
    function toleranceRows(ix, letter, cap, hasCap) {
        const all = ix.data.tolerances.filter(t => t.letter === letter);
        if (!all.length) {
            if (ix.data.not_cross_checked.tolerance.includes(letter)) throw unsupported('the tolerance letter ' + letter + ' is not cross-checked (found in one source only)');
            throw invalidCode('unknown tolerance letter ' + letter);
        }
        const rows = all.filter(t => !hasCap || t.when === 'any' || (cap < TEN_PF && t.when === 'lt10pf') || (cap >= TEN_PF && t.when === 'ge10pf'));
        if (!rows.length) {
            if (cap < TEN_PF) throw unsupported('the tolerance letter ' + letter + ' is not cross-checked for a capacitance below 10 pF');
            throw unsupported('the tolerance letter ' + letter + ' is not cross-checked for a capacitance of 10 pF or more');
        }
        return rows;
    }

    function toleranceRowText(t) {
        if (t.unit === 'pf') return '±' + fmtSig(t.minus, 6) + ' pF';
        if (t.minus === t.plus) return '±' + fmtSig(t.minus, 6) + ' %';
        return '−' + fmtSig(t.minus, 6) + ' % / +' + fmtSig(t.plus, 6) + ' %';
    }

    function whenSuffix(when) {
        if (when === 'lt10pf') return ' (C < 10 pF)';
        if (when === 'ge10pf') return ' (C ≥ 10 pF)';
        return '';
    }

    // The tolerance field(s) for the rows of a letter: several rows (a letter that means pF below 10 pF and percent above)
    // give the readings in the text; an asymmetric tolerance also gives the two deviations.
    function addTolerance(res, rows, whenKnown) {
        if (rows.length > 1) {
            addField(res, 'tolerance', 'Tolerance', rows.map(t => toleranceRowText(t) + whenSuffix(t.when)).join(' or '), NaN, '');
            return;
        }
        const t = rows[0];
        const text = toleranceRowText(t) + (whenKnown ? '' : whenSuffix(t.when));
        if (t.unit === 'pf') addField(res, 'tolerance', 'Tolerance', text, scaleDec(t.minus, -12), 'capacitance');
        else if (t.minus === t.plus) addField(res, 'tolerance', 'Tolerance', text, scaleDec(t.minus, -2), 'ratio');
        else {
            addField(res, 'tolerance', 'Tolerance', text, NaN, 'ratio');
            addField(res, 'tolerance_minus', 'Lower deviation', '−' + fmtSig(t.minus, 6) + ' %', scaleDec(-t.minus, -2), 'ratio');
            addField(res, 'tolerance_plus', 'Upper deviation', '+' + fmtSig(t.plus, 6) + ' %', scaleDec(t.plus, -2), 'ratio');
        }
    }

    // Recognises the shape of one part of the marking.
    function classifyCapacitor(ix, tok) {
        let m = CAP_RKM.exec(tok);
        if (!m) {
            m = CAP_RKM_UPPER.exec(tok);
            if (m) m[2] = m[2].toLowerCase();
        }
        if (m) { // RKM: 4p7, 100n, 2µ2 (the letter is the decimal point and the prefix)
            const t = { hasValue: true, value: decimalText(m[1], m[3], ix.rkm.get(m[2])), family: 'RKM notation', canon: m[1] + m[2] + m[3], warning: '', letter: '', voltage: '' };
            if (m[4] !== '' && !(m[4] === 'F' && m[3] === '')) { // 100nF is the unit farad, not the tolerance letter F
                t.letter = m[4].toUpperCase();
                t.canon += t.letter;
            }
            return t;
        }
        const up = tok.toUpperCase();
        const base = { hasValue: false, value: NaN, family: '', canon: '', warning: '', letter: '', voltage: '' };
        if (CAP_VOLTAGE.test(up)) {
            if (!ix.voltage.has(up)) throw unsupported('the voltage code ' + up + ' is not a standard code (codes with a single source are left out; makers differ, for example Murata writes YA for 35 V)');
            return Object.assign(base, { voltage: up, canon: up });
        }
        if (CAP_LETTER.test(up)) return Object.assign(base, { letter: up, canon: up });
        if (CAP_PAIR.test(up)) throw unsupported(up + ' looks like a maker-specific voltage code (for example Murata YA = 35 V); only the common two-character codes are decoded');
        m = CAP_R.exec(up);
        if (m && m[1].length + m[2].length > 0) { // R notation in pF: 4R7, R50, 0R5
            return Object.assign(base, { hasValue: true, value: decimalText(m[1], m[2], -12), family: 'R notation (pF)', canon: m[1] + 'R' + m[2] + m[3], letter: m[3] });
        }
        m = CAP_DIGITS.exec(up);
        if (m) { // 3 digits: two significant digits and the multiplier
            const digit = m[1].slice(2);
            const row = ix.mult.get(digit);
            if (!row) {
                if (ix.data.not_cross_checked.multiplier.includes(digit)) throw unsupported('the multiplier digit ' + digit + ' is not cross-checked');
                throw invalidCode('unknown multiplier digit ' + digit);
            }
            const t = Object.assign(base, { hasValue: true, value: decimal(parseInt(m[1].slice(0, 2), 10), row.exp - 12), family: '3-digit pF code', canon: m[1] + m[2], letter: m[2] });
            if (row.maker) t.warning = 'the third digit ' + row.digit + ' (×' + fmtNum(decimal(1, row.exp)) + ') follows the ' + row.maker.toUpperCase() + ' ordering convention for values below 10 pF';
            return t;
        }
        throw invalidCode('cannot read ' + JSON.stringify(tok) + ' as a capacitor marking (value code, tolerance letter or voltage code)');
    }

    // The voltage field and the warnings that come with a voltage code; headline makes the voltage the value.
    function addVoltage(res, units, v, headline) {
        addField(res, 'voltage', 'Rated voltage', units.format(v.volts, 'voltage', '', 4), v.volts, 'voltage');
        if (headline) { res.value = v.volts; res.dim = 'voltage'; }
        res.warnings.push(VOLTAGE_WARNING);
        if (v.note_en) res.warnings.push(v.code + ': ' + v.note_en);
    }

    function decodeCapacitor(item, code, units) {
        const ix = capacitorIndex(item);
        const tokens = code.split(/[\s,;]+/).filter(t => t !== '');
        if (!tokens.length) throw invalidCode('the code is empty');
        if (tokens.length > 3) throw invalidCode('a capacitor marking has at most three parts (value code, tolerance letter, voltage code)');
        const res = newResult('capacitor');
        const whole = { hasValue: false, value: NaN, family: '', letter: '', voltage: '' };
        const canon = [];
        for (const tok of tokens) {
            const t = classifyCapacitor(ix, tok);
            if (t.hasValue && whole.hasValue) throw invalidCode('more than one capacitance value in the code');
            if (t.letter !== '' && whole.letter !== '') throw invalidCode('more than one tolerance letter in the code');
            if (t.voltage !== '' && whole.voltage !== '') throw invalidCode('more than one voltage code in the code');
            if (t.hasValue) {
                if (!(t.value > 0)) throw invalidCode('the code gives a capacitance of zero');
                whole.hasValue = true;
                whole.value = t.value;
                whole.family = t.family;
                if (t.warning) res.warnings.push(t.warning);
            }
            if (t.letter !== '') whole.letter = t.letter;
            if (t.voltage !== '') whole.voltage = t.voltage;
            canon.push(t.canon);
        }
        res.code = canon.join(' ');
        if (whole.hasValue) {
            res.value = whole.value;
            res.dim = 'capacitance';
            addField(res, 'family', 'Marking', whole.family, NaN, '');
        }
        if (whole.letter !== '') addTolerance(res, toleranceRows(ix, whole.letter, whole.value, whole.hasValue), whole.hasValue);
        if (whole.voltage !== '') addVoltage(res, units, ix.voltage.get(whole.voltage), !whole.hasValue);
        return res;
    }

    // The 3-digit code or the R notation (pF) of N * 10^e farad, '' when it has none.
    function capValueCode(N, e) {
        const ep = e + 12; // N * 10^ep is the value in pF
        if (ep >= 0 && ep <= 6) return String(N) + ep;
        if (ep === -1) return Math.floor(N / 10) + 'R' + (N % 10);
        if (ep === -2) return 'R' + N;
        return '';
    }

    // N * 10^e farad in RKM notation (4p7, 100n, 2µ2): the letter is the decimal point.
    function capRKMCode(N, e) {
        const prefixes = [['m', -3], ['µ', -6], ['n', -9], ['p', -12]];
        const digits = String(N);
        const top = e + digits.length - 1; // exponent of the leading digit
        let pick = prefixes[prefixes.length - 1];
        for (const p of prefixes) {
            if (top >= p[1]) { pick = p; break; }
        }
        const shift = e - pick[1]; // the number is digits * 10^shift in the unit of the prefix
        let whole = digits, frac = '';
        if (shift >= 0) whole = digits + '0'.repeat(shift);
        else if (-shift >= digits.length) { whole = '0'; frac = '0'.repeat(-shift - digits.length) + digits; }
        else { whole = digits.slice(0, digits.length + shift); frac = digits.slice(digits.length + shift); }
        return whole + pick[0] + frac.replace(/0+$/, '');
    }

    function letterList(ix) {
        return [...new Set(ix.data.tolerances.map(t => t.letter))].join(' ');
    }

    // The tolerance letter for an encode request given as a letter, "±0.25 pF", "10 %" or "-20/+80": the letter, its rows and
    // the tolerance (percent) that selects the E-series (0 = do not snap).
    function tolerancePick(ix, text, cap, hasCap) {
        const t = text.trim();
        const match = test => {
            for (const x of ix.data.tolerances) {
                if (!test(x)) continue;
                let rows = null;
                try { rows = toleranceRows(ix, x.letter, cap, hasCap); } catch (error) { rows = null; }
                if (rows && rows.includes(x)) return { letter: x.letter, rows: [x], pct: 0 };
            }
            return null;
        };
        const noLetter = () => unsupported('no tolerance letter for ' + JSON.stringify(text) + ' (letters: ' + letterList(ix) + ')');
        if (CAP_LETTER.test(t.toUpperCase())) {
            const letter = t.toUpperCase();
            const rows = toleranceRows(ix, letter, cap, hasCap);
            let pct = 0;
            if (rows.length === 1 && rows[0].unit === 'pct') pct = rows[0].minus === rows[0].plus ? rows[0].minus : 20;
            return { letter, rows, pct };
        }
        if (compact(t).toLowerCase().endsWith('pf')) {
            const c = compact(t);
            const x = parsePercent(c.slice(0, -2));
            if (x === null) throw invalidValue('tolerance: cannot read ' + JSON.stringify(text));
            const found = match(y => y.unit === 'pf' && closeTo(y.minus, x));
            if (!found) throw noLetter();
            return found;
        }
        const asym = parseAsym(t);
        if (asym) {
            const found = match(y => y.unit === 'pct' && closeTo(y.plus, asym.plus) && closeTo(y.minus, asym.minus));
            if (!found) throw noLetter();
            found.pct = 20;
            return found;
        }
        const p = parsePercent(t);
        if (p === null || !(p > 0)) throw invalidValue('tolerance: cannot read ' + JSON.stringify(text) + ' as a letter, a percentage, a value in pF or an asymmetric tolerance such as -20/+80');
        const found = match(y => y.unit === 'pct' && y.minus === y.plus && closeTo(y.minus, p));
        if (!found) throw noLetter();
        found.pct = p;
        return found;
    }

    function encodeCapacitor(item, req, units) {
        const ix = capacitorIndex(item);
        const hasValue = !blank(req.value), hasTol = !blank(req.tolerance), hasVolt = !blank(req.voltage);
        if (!hasValue && !hasTol && !hasVolt) throw invalidValue('give a value, a tolerance or a voltage');
        const res = newResult('capacitor');
        const parts = [];
        let capV = 0;
        if (hasValue) {
            try { capV = units.parse(req.value, 'capacitance', ''); } catch (error) { throw parseFail('value', error); }
            if (!(capV > 0)) throw invalidValue('value: the capacitance must be positive');
        }
        let pick = { letter: '', rows: [], pct: 0 };
        if (hasTol) pick = tolerancePick(ix, req.tolerance, capV, hasValue);
        if (hasValue) {
            let vs = capV, changed = false;
            if (pick.pct > 0) {
                const snap = snapToSeries(capV, pick.pct);
                vs = snap.value;
                changed = snap.changed;
            }
            const s = sigDigits(vs, 2);
            if (!s.exact) throw unsupported(units.format(vs, 'capacitance', '', 4) + ' needs more than 2 significant digits; the 3-digit capacitor code holds 2');
            const vc = capValueCode(s.N, s.e);
            if (vc === '') throw unsupported(units.format(vs, 'capacitance', '', 4) + ' is outside the range of the capacitor code (' + units.format(decimal(10, -13), 'capacitance', '', 4) + ' to ' + units.format(decimal(99, -6), 'capacitance', '', 4) + ')');
            res.value = decimal(s.N, s.e);
            res.dim = 'capacitance';
            addField(res, 'value_code', 'Value code', vc, NaN, '');
            addField(res, 'rkm', 'RKM notation', capRKMCode(s.N, s.e), NaN, '');
            parts.push(vc + pick.letter);
            if (changed) res.warnings.push(snapWarning(units, 'capacitance', capV, vs, pick.pct));
        } else if (pick.letter !== '') {
            parts.push(pick.letter);
        }
        if (pick.letter !== '') addTolerance(res, pick.rows, hasValue);
        if (hasVolt) {
            let v;
            try { v = units.parse(req.voltage, 'voltage', ''); } catch (error) { throw parseFail('voltage', error); }
            const row = ix.data.voltages.filter(x => closeTo(x.volts, v)).pop() || null;
            if (!row) throw unsupported('no standard voltage code for ' + units.format(v, 'voltage', '', 4));
            addVoltage(res, units, row, !hasValue);
            parts.push(row.code);
        }
        res.code = parts.join(' ');
        return res;
    }

    // ---- ceramic_temp ---------------------------------------------------------------------------------------------------------------------------------
    const RANGE = /^(.+?)\s*(?:\.\.|…|\bto\b)\s*(.+)$/i;

    function ceramicIndex(item) {
        let ix = indexes.get(item);
        if (ix) return ix;
        const d = item.data;
        ix = {
            data: d,
            sig: new Map(d.class1.sig.map(x => [x.letter, x])), mult: new Map(d.class1.mult.map(x => [x.digit, x])), tol: new Map(d.class1.tol.map(x => [x.letter, x])),
            low: new Map(d.class2.low.map(x => [x.letter, x])), high: new Map(d.class2.high.map(x => [x.digit, x])), change: new Map(d.class2.change.map(x => [x.letter, x])),
            industry: new Map(d.industry.map(x => [x.code, x.name]))
        };
        indexes.set(item, ix);
        return ix;
    }

    const notChecked = (what, symbol) => unsupported(what + ' ' + symbol + ' is not cross-checked (found in one source only)');

    function decodeCeramic(item, code) {
        const ix = ceramicIndex(item);
        let s = code.replace(/\s+/g, '').toUpperCase().replace(/[-_]/g, '');
        for (const a of ix.data.aliases) if (s === a.name) s = a.code;
        const r = Array.from(s);
        if (r.length === 3 && r[1] === 'O' && !'XYZ'.includes(r[0])) r[1] = '0'; // COG for C0G
        if (r.length !== 3) throw invalidCode('a ceramic temperature code has 3 characters (for example X7R or C0G)');
        const [a, b, c] = r;
        const canon = a + b + c;
        const res = newResult('ceramic_temp');
        res.code = canon;
        const nc = ix.data.not_cross_checked;
        if ('XYZ'.includes(a)) {
            const low = ix.low.get(a), high = ix.high.get(b), change = ix.change.get(c);
            if (!high) {
                if (nc.c2_high.includes(b)) throw notChecked('the upper temperature digit', b);
                throw invalidCode('unknown upper temperature digit ' + b);
            }
            if (!change) {
                if (nc.c2_change.includes(c)) throw notChecked('the capacitance change letter', c);
                throw invalidCode('unknown capacitance change letter ' + c);
            }
            addField(res, 'class', 'Class', '2', NaN, '');
            addField(res, 'temp_low', 'Lower category temperature', signedText(low.c, 6) + ' °C', roundSig(low.c + 273.15, 12), 'temperature');
            addField(res, 'temp_high', 'Upper category temperature', signedText(high.c, 6) + ' °C', roundSig(high.c + 273.15, 12), 'temperature');
            if (change.plus === change.minus) addField(res, 'change', 'Capacitance change', '±' + fmtSig(change.plus, 6) + ' %', scaleDec(change.plus, -2), 'ratio');
            else {
                addField(res, 'change', 'Capacitance change', '+' + fmtSig(change.plus, 6) + ' % / −' + fmtSig(change.minus, 6) + ' %', NaN, 'ratio');
                addField(res, 'change_plus', 'Upper capacitance change', '+' + fmtSig(change.plus, 6) + ' %', scaleDec(change.plus, -2), 'ratio');
                addField(res, 'change_minus', 'Lower capacitance change', '−' + fmtSig(change.minus, 6) + ' %', scaleDec(-change.minus, -2), 'ratio');
            }
            return res;
        }
        const sig = ix.sig.get(a), mult = ix.mult.get(b), tol = ix.tol.get(c);
        if (!sig) {
            if (nc.c1_sig.includes(a)) throw notChecked('the temperature coefficient letter', a);
            throw invalidCode('unknown temperature coefficient letter ' + a);
        }
        if (!mult) {
            if (nc.c1_mult.includes(b)) throw notChecked('the multiplier digit', b);
            throw invalidCode('unknown multiplier digit ' + b);
        }
        if (!tol) throw invalidCode('unknown tolerance letter ' + c);
        let tc = sig.value * mult.value;
        if (tc === 0) tc = 0; // no negative zero
        res.value = scaleDec(tc, -6);
        res.dim = 'tempco';
        addField(res, 'class', 'Class', '1', NaN, '');
        addField(res, 'tc', 'Temperature coefficient', signedText(tc, 6) + ' ppm/K', scaleDec(tc, -6), 'tempco');
        addField(res, 'tolerance', 'Tolerance', '±' + fmtSig(tol.ppm, 6) + ' ppm/K', scaleDec(tol.ppm, -6), 'tempco');
        const name = ix.industry.get(canon);
        if (name) addField(res, 'industry_code', 'Industry code', name, NaN, '');
        return res;
    }

    // A temperature typed in °C (a bare number is in °C), in °C.
    function celsius(units, text) {
        return units.parse(text.replace(/−/g, '-'), 'temperature', 'degC') - 273.15;
    }

    function encodeCeramic(item, req, units) {
        const ix = ceramicIndex(item);
        let cls = (req.class || '').trim();
        if (cls === '') {
            if (RANGE.test(req.value || '')) cls = '2';
            else if (!blank(req.tempco) || !blank(req.value)) cls = '1';
            else throw invalidValue('give the class (1 or 2)');
        }
        let code;
        if (cls === '1') {
            const source = blank(req.tempco) ? req.value : req.tempco;
            if (blank(source) || blank(req.tolerance)) throw invalidValue('class 1 needs the temperature coefficient (tempco, for example -750ppm) and the tolerance (for example 120ppm)');
            let tcSI, tolSI;
            try { tcSI = parseTempco(units, source); } catch (error) { throw parseFail('tempco', error); }
            try { tolSI = parseTempco(units, req.tolerance); } catch (error) { throw parseFail('tolerance', error); }
            const tc = tcSI / 1e-6, tolPPM = Math.abs(tolSI) / 1e-6;
            let sigL = '', multD = '', tolL = '';
            for (const x of ix.data.class1.sig) {
                for (const m of ix.data.class1.mult) {
                    const p = x.value * m.value;
                    if (sigL === '' && ((tc === 0 && p === 0) || (tc !== 0 && p !== 0 && closeTo(p, tc)))) { sigL = x.letter; multD = m.digit; }
                }
            }
            if (sigL === '') throw unsupported('no class 1 code for a temperature coefficient of ' + fmtSig(tc, 6) + ' ppm/K');
            for (const x of ix.data.class1.tol) if (tolL === '' && closeTo(x.ppm, tolPPM)) tolL = x.letter;
            if (tolL === '') throw unsupported('no class 1 tolerance letter for ±' + fmtSig(tolPPM, 6) + ' ppm/K');
            code = sigL + multD + tolL;
        } else if (cls === '2') {
            const m = RANGE.exec(req.value || '');
            if (!m) throw invalidValue('class 2 needs the category temperature range in value, for example -55..125');
            let lo, hi;
            try { lo = celsius(units, m[1]); } catch (error) { throw parseFail('value', error); }
            try { hi = celsius(units, m[2]); } catch (error) { throw parseFail('value', error); }
            const low = ix.data.class2.low.find(x => Math.abs(x.c - lo) < 1e-6);
            if (!low) throw unsupported('no class 2 letter for a lower category temperature of ' + fmtSig(lo, 6) + ' °C');
            const high = ix.data.class2.high.find(x => Math.abs(x.c - hi) < 1e-6);
            if (!high) throw unsupported('no class 2 digit for an upper category temperature of ' + fmtSig(hi, 6) + ' °C');
            if (blank(req.tolerance)) throw invalidValue('class 2 needs the capacitance change in tolerance, for example 15% or +22/-82');
            let plus, minus;
            const asym = parseAsym(req.tolerance);
            if (asym) { plus = asym.plus; minus = asym.minus; } else {
                const p = parsePercent(req.tolerance);
                if (p === null) throw invalidValue('tolerance: cannot read ' + JSON.stringify(req.tolerance) + ' as a percentage or as an asymmetric change such as +22/-82');
                plus = p;
                minus = p;
            }
            const change = ix.data.class2.change.find(x => closeTo(x.plus, plus) && closeTo(x.minus, minus));
            if (!change) throw unsupported('no class 2 letter for a capacitance change of +' + fmtSig(plus, 6) + ' % / −' + fmtSig(minus, 6) + ' %');
            code = low.letter + high.digit + change.letter;
        } else {
            throw invalidValue('class must be 1 or 2');
        }
        return decodeCeramic(item, code);
    }

    // ---- API ---------------------------------------------------------------------------------------------------------------------------------------------
    const KINDS = {
        resistor_bands: { decode: decodeResistor, encode: encodeResistor },
        inductor_bands: { decode: decodeInductor, encode: encodeInductor },
        smd_resistor: { decode: decodeSMD, encode: encodeSMD },
        capacitor: { decode: decodeCapacitor, encode: encodeCapacitor },
        ceramic_temp: { decode: decodeCeramic, encode: encodeCeramic }
    };

    function kindOf(item) {
        const kind = item && item.kind === 'code' ? KINDS[item.code] : null;
        if (!kind) throw unsupported('unknown code kind ' + JSON.stringify(item && item.code) + ' (kinds: ' + Object.keys(KINDS).sort().join(', ') + ')');
        return kind;
    }

    function decode(item, code, units) {
        const kind = kindOf(item);
        const text = String(code == null ? '' : code);
        if (text.length > MAX_TEXT) throw invalidCode('the code is longer than ' + MAX_TEXT + ' characters');
        return finish(kind.decode(item, text, units));
    }

    function encode(item, request, units) {
        const kind = kindOf(item);
        const req = Object.assign({ value: '', bands: 0, tolerance: '', tempco: '', voltage: '', class: '' }, request || {});
        for (const key of ['value', 'tolerance', 'tempco', 'voltage', 'class']) {
            req[key] = String(req[key] == null ? '' : req[key]);
            if (req[key].length > MAX_TEXT) throw invalidValue('a request field is longer than ' + MAX_TEXT + ' characters');
        }
        req.bands = Number(req.bands) || 0;
        if (!Number.isInteger(req.bands) || req.bands < 0 || req.bands > 6) throw invalidValue('bands must be 0 (choose), 3, 4, 5 or 6');
        return finish(kind.encode(item, req, units));
    }

    core.codes = { decode, encode };
})();
