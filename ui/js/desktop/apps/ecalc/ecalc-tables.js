/* ECalc tables: the table items of the catalog JSON (kind "table") indexed for the table functions of the expression
   language (lookup, lookup_ge, interp) and for the table page. A DOM-free classic script that publishes
   window.ECalcCore.tables, a mirror of the lookups in internal/ecalc/tables.go that scripts/test-ecalc-core.mjs runs
   against the shared vectors.

   A table item is {id, key, columns[{id, dim, unit, ...}], rows[[cell]]}: a cell of a numeric column is a number in SI
   units or null (empty), a cell of a text column (dim "text") is a string or null. The shell calls register(items) once
   the catalog has loaded (every item that is not a table is skipped; a table registered again replaces the old one).
   Table and column arguments name a table id and a column id; an unknown table, an unknown column and a text column
   throw an error with code 'eval', which the solver turns into NaN for the quantity. */
(function () {
    'use strict';

    const core = (window.ECalcCore = window.ECalcCore || {});
    const REL = 1e-9;
    const registry = new Map();

    function fail(message) {
        const error = new Error(message);
        error.code = 'eval';
        return error;
    }

    // Equal within 1e-9 of the larger magnitude; exact equality counts, NaN and infinities never match unless identical.
    function closeEnough(a, b) {
        if (a === b) return true;
        if (!Number.isFinite(a) || !Number.isFinite(b)) return false;
        return Math.abs(a - b) <= REL * Math.max(Math.abs(a), Math.abs(b));
    }

    function build(item) {
        const index = new Map(item.columns.map((column, i) => [column.id, i]));
        return { id: item.id, item, columns: item.columns, rows: item.rows, index, keyIndex: index.has(item.key) ? index.get(item.key) : 0 };
    }

    function register(items) {
        let count = 0;
        for (const item of items || []) {
            if (item && item.kind === 'table') { registry.set(item.id, build(item)); count += 1; }
        }
        return count;
    }

    const reset = () => registry.clear();
    const get = id => registry.get(id) || null;

    function table(id) {
        const found = registry.get(id);
        if (!found) throw fail('unknown table ' + id);
        return found;
    }

    // The index of a numeric column of a table.
    function numericColumn(t, id) {
        if (!t.index.has(id)) throw fail('table ' + t.id + ' has no column ' + id);
        const i = t.index.get(id);
        if (t.columns[i].dim === 'text') throw fail('column ' + id + ' of table ' + t.id + ' is text');
        return i;
    }

    const cellValue = (t, row, column) => (t.rows[row][column] === null ? NaN : t.rows[row][column]);

    // The first row (table order) whose numeric column equals x within 1e-9 relative, or -1.
    function rowOf(t, column, x) {
        for (let r = 0; r < t.rows.length; r += 1) {
            const value = t.rows[r][column];
            if (value !== null && closeEnough(value, x)) return r;
        }
        return -1;
    }

    // findRow(tableId, columnId, value): the row index for the finder highlight and the table page, or -1.
    function findRow(id, columnId, value) {
        const t = table(id);
        return rowOf(t, numericColumn(t, columnId), value);
    }

    // lookup(T, C, key): the value of column C in the row whose key column equals key; NaN when no row matches or the cell is empty.
    function lookup(id, columnId, key) {
        const t = table(id), c = numericColumn(t, columnId);
        if (t.columns[t.keyIndex].dim === 'text') throw fail('table ' + t.id + ' has a text key column, lookup needs a numeric key');
        const row = rowOf(t, t.keyIndex, key);
        return row < 0 ? NaN : cellValue(t, row, c);
    }

    // lookup_ge(T, C, x, OUT): column OUT of the first row whose column C is filled and at least x; NaN when there is none.
    function lookupGe(id, columnId, x, outId) {
        const t = table(id), c = numericColumn(t, columnId), o = numericColumn(t, outId);
        if (Number.isNaN(x)) return NaN;
        for (let r = 0; r < t.rows.length; r += 1) {
            const value = t.rows[r][c];
            if (value !== null && value >= x) return cellValue(t, r, o);
        }
        return NaN;
    }

    // interp(T, X, Y, x): the linear interpolation of Y over X through the rows where both are filled; exact at the nodes,
    // NaN outside the range of X.
    function interp(id, xId, yId, x) {
        const t = table(id), xc = numericColumn(t, xId), yc = numericColumn(t, yId);
        const xs = [], ys = [];
        for (const row of t.rows) {
            if (row[xc] !== null && row[yc] !== null) { xs.push(row[xc]); ys.push(row[yc]); }
        }
        for (let i = 0; i < xs.length; i += 1) if (xs[i] === x) return ys[i];
        for (let i = 0; i + 1 < xs.length; i += 1) {
            const x0 = xs[i], x1 = xs[i + 1];
            if ((x > x0 && x < x1) || (x < x0 && x > x1)) return ys[i] + (ys[i + 1] - ys[i]) * ((x - x0) / (x1 - x0));
        }
        return NaN;
    }

    core.tables = { register, reset, get, findRow, lookup, lookupGe, interp, closeEnough };
})();
