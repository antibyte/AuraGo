// HomeDash undo/redo (pure): command stack with coalescing (same meta.coalesce within meta.window ms), limit 100.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.doc) throw new Error('hd-history.js: load hd-doc.js first');
    function create(options) {
        const limit = (options && options.limit) || 100;
        let undoStack = [], redoStack = [], sealed = false;
        function merge(top, cmd) {
            const byPath = new Map(top.ops.map(o => [E.doc.keyOf(o.path), o]));
            for (const o of cmd.ops) { const prev = byPath.get(E.doc.keyOf(o.path)); if (prev) prev.after = o.after; else { const c = Object.assign({}, o); top.ops.push(c); byPath.set(E.doc.keyOf(o.path), c); } }
            top.meta = Object.assign({}, top.meta, { at: cmd.meta.at });
        }
        return {
            push(cmd) {
                const top = undoStack[undoStack.length - 1], m = cmd.meta || {};
                redoStack = [];
                if (!sealed && top && m.coalesce && top.meta.coalesce === m.coalesce && m.at - top.meta.at <= (m.window || 0)) { merge(top, cmd); return top; }
                const entry = { label: cmd.label, ops: cmd.ops.map(o => Object.assign({}, o)), meta: Object.assign({}, m) };
                undoStack.push(entry); sealed = false; if (undoStack.length > limit) undoStack.shift(); return entry;
            },
            undo(doc) { const c = undoStack.pop(); if (!c) return null; redoStack.push(c); sealed = true; return { doc: E.doc.apply(doc, c.ops, 'before'), command: c }; },
            redo(doc) { const c = redoStack.pop(); if (!c) return null; undoStack.push(c); sealed = true; return { doc: E.doc.apply(doc, c.ops, 'after'), command: c }; },
            seal() { sealed = true; }, clear() { undoStack = []; redoStack = []; sealed = false; },
            canUndo: () => undoStack.length > 0, canRedo: () => redoStack.length > 0, size: () => undoStack.length, peek: () => undoStack[undoStack.length - 1] || null
        };
    }
    E.history = Object.freeze({ create, LIMIT: 100 });
})();
