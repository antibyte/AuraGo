/* ECalc E-series: the IEC 60063 value tables E3 to E192, the nearest standard value with its deviation and the
   standard-value combiner (single, series and parallel combinations of a target). A DOM-free classic script that
   publishes window.ECalcCore.eseries; it mirrors internal/ecalc/eseries.go and is checked against the same vectors. The
   decade values are written out because E3 to E24 are not the rounded roots (2.7 is not 2.6). */
(function () {
    'use strict';

    const core = (window.ECalcCore = window.ECalcCore || {});

    const words = text => text.split(' ').map(Number);
    const TABLES = {
        E3: words('1.0 2.2 4.7'),
        E6: words('1.0 1.5 2.2 3.3 4.7 6.8'),
        E12: words('1.0 1.2 1.5 1.8 2.2 2.7 3.3 3.9 4.7 5.6 6.8 8.2'),
        E24: words('1.0 1.1 1.2 1.3 1.5 1.6 1.8 2.0 2.2 2.4 2.7 3.0 3.3 3.6 3.9 4.3 4.7 5.1 5.6 6.2 6.8 7.5 8.2 9.1'),
        E48: words('1.00 1.05 1.10 1.15 1.21 1.27 1.33 1.40 1.47 1.54 1.62 1.69 1.78 1.87 1.96 2.05 2.15 2.26 2.37 2.49 2.61 2.74 2.87 3.01 3.16 3.32 3.48 3.65 3.83 4.02 4.22 4.42 4.64 4.87 5.11 5.36 5.62 5.90 6.19 6.49 6.81 7.15 7.50 7.87 8.25 8.66 9.09 9.53'),
        E96: words('1.00 1.02 1.05 1.07 1.10 1.13 1.15 1.18 1.21 1.24 1.27 1.30 1.33 1.37 1.40 1.43 1.47 1.50 1.54 1.58 1.62 1.65 1.69 1.74 1.78 1.82 1.87 1.91 1.96 2.00 2.05 2.10 2.15 2.21 2.26 2.32 2.37 2.43 2.49 2.55 2.61 2.67 2.74 2.80 2.87 2.94 3.01 3.09 3.16 3.24 3.32 3.40 3.48 3.57 3.65 3.74 3.83 3.92 4.02 4.12 4.22 4.32 4.42 4.53 4.64 4.75 4.87 4.99 5.11 5.23 5.36 5.49 5.62 5.76 5.90 6.04 6.19 6.34 6.49 6.65 6.81 6.98 7.15 7.32 7.50 7.68 7.87 8.06 8.25 8.45 8.66 8.87 9.09 9.31 9.53 9.76'),
        E192: words('1.00 1.01 1.02 1.04 1.05 1.06 1.07 1.09 1.10 1.11 1.13 1.14 1.15 1.17 1.18 1.20 1.21 1.23 1.24 1.26 1.27 1.29 1.30 1.32 1.33 1.35 1.37 1.38 1.40 1.42 1.43 1.45 1.47 1.49 1.50 1.52 1.54 1.56 1.58 1.60 1.62 1.64 1.65 1.67 1.69 1.72 1.74 1.76 1.78 1.80 1.82 1.84 1.87 1.89 1.91 1.93 1.96 1.98 2.00 2.03 2.05 2.08 2.10 2.13 2.15 2.18 2.21 2.23 2.26 2.29 2.32 2.34 2.37 2.40 2.43 2.46 2.49 2.52 2.55 2.58 2.61 2.64 2.67 2.71 2.74 2.77 2.80 2.84 2.87 2.91 2.94 2.98 3.01 3.05 3.09 3.12 3.16 3.20 3.24 3.28 3.32 3.36 3.40 3.44 3.48 3.52 3.57 3.61 3.65 3.70 3.74 3.79 3.83 3.88 3.92 3.97 4.02 4.07 4.12 4.17 4.22 4.27 4.32 4.37 4.42 4.48 4.53 4.59 4.64 4.70 4.75 4.81 4.87 4.93 4.99 5.05 5.11 5.17 5.23 5.30 5.36 5.42 5.49 5.56 5.62 5.69 5.76 5.83 5.90 5.97 6.04 6.12 6.19 6.26 6.34 6.42 6.49 6.57 6.65 6.73 6.81 6.90 6.98 7.06 7.15 7.23 7.32 7.41 7.50 7.59 7.68 7.77 7.87 7.96 8.06 8.16 8.25 8.35 8.45 8.56 8.66 8.76 8.87 8.98 9.09 9.20 9.31 9.42 9.53 9.65 9.76 9.88')
    };
    // The series the pages offer (E3 is too coarse to be useful as a suggestion).
    const SERIES = ['E6', 'E12', 'E24', 'E48', 'E96', 'E192'];
    // The smallest normal double (2^-1022). Go's math.Log10 is wrong for denormals, so nearest and combine refuse a value
    // below it, exactly like Nearest and Combine in Go.
    const MIN_NORMAL = 2.2250738585072014e-308;

    // Nearest standard value of a series, by relative distance, a tie going to the lower value. The candidates of
    // the decade of v include the first value of the next decade, so a v just below a power of ten is handled.
    // Returns {value, dev} with dev in percent ((value - v) / v * 100), or null for a v below MIN_NORMAL (so v <= 0 too),
    // a non-finite v or an unknown series.
    function nearest(series, v) {
        if (!Object.prototype.hasOwnProperty.call(TABLES, series) || !Number.isFinite(v) || !(v >= MIN_NORMAL)) return null;
        const decade = Math.pow(10, Math.floor(Math.log10(v)));
        let best = 0, bestError = Infinity;
        for (const entry of TABLES[series].concat([10])) {
            const candidate = entry * decade;
            const error = Math.abs(candidate - v) / v;
            if (error < bestError) { best = candidate; bestError = error; }
        }
        best = Number(best.toPrecision(12));
        return { value: best, dev: (best - v) / v * 100 };
    }

    // The three-digit values mantissa x 10^k of a series in [lo, hi], ascending; each is the double nearest to the
    // decimal (Number(x.toPrecision(3)), the roundSig of internal/ecalc/eseries.go). A range whose bounds are not usable has
    // no candidates (lo must be at least MIN_NORMAL, hi finite: target / 1000 and target * 1000 leave the doubles for a
    // tiny or a huge target), which also bounds the decade loop; without this guard it never ends for lo = 0 or hi = Infinity.
    function comboCandidates(table, lo, hi) {
        const out = [];
        if (!(lo >= MIN_NORMAL) || !(hi >= lo) || !Number.isFinite(hi)) return out;
        for (let k = Math.floor(Math.log10(lo)) - 1; k <= Math.floor(Math.log10(hi)) + 1; k += 1) {
            const scale = Math.pow(10, k);
            for (const mantissa of table) {
                const candidate = Number((mantissa * scale).toPrecision(3));
                if (candidate >= lo && candidate <= hi && (out.length === 0 || candidate > out[out.length - 1])) out.push(candidate);
            }
        }
        return out;
    }

    // Order of one group: the rank of the absolute deviation (in units of 1e-9 percent, rounded, so that deviations that
    // agree to that precision are equal and floating-point noise cannot reorder exact hits), then r1, then r2.
    // Math.round and Go's math.Round agree for the non-negative values here.
    const COMBO_MAX_LIMIT = 1000;
    const comboBefore = (qa, r1a, r2a, qb, b) => (qa !== qb ? qa < qb : r1a !== b.r1 ? r1a < b.r1 : r2a < b.r2);

    // Keeps the best `limit` entries of a stream of candidates, in order; an entry is only built when it makes the list.
    // The order is total (r1 and r2 differ between candidates), so the result does not depend on the order in which the
    // candidates are offered.
    function bestOf(kind, limit, target) {
        const list = [], ranks = [];
        let worst = Infinity; // the rank of the last entry once the list is full: anything worse is dropped at once
        return {
            list,
            offer(r1, r2, value) {
                const devPct = (value - target) / target * 100;
                const q = Math.round(Math.abs(devPct) * 1e9), n = list.length;
                if (q > worst || (n === limit && !comboBefore(q, r1, r2, ranks[n - 1], list[n - 1]))) return;
                let at = n;
                while (at > 0 && comboBefore(q, r1, r2, ranks[at - 1], list[at - 1])) at -= 1;
                list.splice(at, 0, { kind, r1, r2, value, devPct });
                ranks.splice(at, 0, q);
                if (list.length > limit) { list.pop(); ranks.pop(); }
                if (list.length === limit) worst = ranks[limit - 1];
            }
        };
    }

    // The standard-value combiner (Combine in internal/ecalc/eseries.go): the single nearest value first, then at most
    // `limit` pairs in series (r1 >= r2, both values in [target/1000, target]) and at most `limit` pairs in parallel
    // (r1 <= r2, both in [target, 1000 * target]), each group by |devPct|, then r1, then r2. A limit below one leaves the
    // single entry; a limit above 1000 counts as 1000. Entries are {kind: 'single' | 'series' | 'parallel', r1, r2,
    // value, devPct}; r2 is 0 for the single value. Returns null for an unknown series or a target that is not finite
    // and at least MIN_NORMAL. A group whose range bound is not usable (target / 1000 below MIN_NORMAL, target * 1000
    // beyond the doubles) is skipped.
    function combine(series, target, limit) {
        const single = nearest(series, target);
        if (!single) return null;
        const out = [{ kind: 'single', r1: single.value, r2: 0, value: single.value, devPct: single.dev }];
        if (!(limit >= 1)) return out;
        const cap = Math.min(Math.floor(limit), COMBO_MAX_LIMIT);
        const table = TABLES[series];
        const inSeries = bestOf('series', cap, target), inParallel = bestOf('parallel', cap, target);
        const small = comboCandidates(table, target / 1000, target);
        small.forEach((r1, i) => { for (let j = 0; j <= i; j += 1) inSeries.offer(r1, small[j], r1 + small[j]); });
        const large = comboCandidates(table, target, target * 1000);
        large.forEach((r1, i) => { for (let j = i; j < large.length; j += 1) inParallel.offer(r1, large[j], r1 * large[j] / (r1 + large[j])); });
        return out.concat(inSeries.list, inParallel.list);
    }

    core.eseries = { TABLES, SERIES, nearest, combine };
})();
