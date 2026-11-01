/* ECalc typesetting: turns the AST of a catalog formula into HTML (fractions for /, superscripts for ^, a radical for
   sqrt, a centred dot for *, pi as the Greek letter, labels such as V_in with a subscript) and renders the live line,
   the same AST with the current values substituted. DOM-free classic script that publishes window.ECalcTypeset.
   Everything that comes from the catalog or from a value is escaped; the only markup is built here.

   The context object of both renderers:
     labelOf(name)  -> display label of a variable (V_in), or '' for an unknown name
     constOf(name)  -> number of a choice const (k, cf), or undefined
     valueOf(name)  -> live line only: {html, prec} of the formatted value of a variable, or null when it is unknown
   prec is the binding strength of the html: 6 atom, 3 a product such as "12 V", 2 a sum or a negative number. */
(function () {
    'use strict';

    const SIGNS = { '<': '&lt;', '<=': '≤', '>': '&gt;', '>=': '≥', '==': '=', '!=': '≠' };
    const escape = text => String(text).replace(/[&<>"']/g, char => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[char]));
    const span = (name, html) => '<span class="ec-' + name + '">' + html + '</span>';

    // V_in -> V<sub>in</sub>: everything behind the first underscore is the subscript.
    function labelHTML(label) {
        const text = String(label == null ? '' : label), at = text.indexOf('_');
        if (at <= 0) return escape(text);
        return escape(text.slice(0, at)) + '<sub>' + escape(text.slice(at + 1)) + '</sub>';
    }

    function render(node, ctx, live) {
        const wrap = (child, minimum) => {
            const part = go(child);
            return part.prec < minimum ? '(' + part.html + ')' : part.html;
        };
        const go = child => render(child, ctx, live);
        switch (node.t) {
            case 'num': return { html: span('n', escape(node.raw)), prec: 6 };
            case 'id': {
                if (node.name === 'pi') return { html: span('n', 'π'), prec: 6 };
                const constant = ctx.constOf ? ctx.constOf(node.name) : undefined;
                if (constant !== undefined) return { html: span('n', escape(constant)), prec: 6 };
                if (live && ctx.valueOf) {
                    const value = ctx.valueOf(node.name);
                    if (value) return { html: span('v', value.html), prec: value.prec };
                }
                const label = (ctx.labelOf && ctx.labelOf(node.name)) || node.name;
                return { html: span(live ? 'u' : 'i', labelHTML(label)), prec: 6 };
            }
            case 'neg': return { html: '−' + wrap(node.a, 4), prec: 4 };
            case 'pos': return { html: '+' + wrap(node.a, 4), prec: 4 };
            case 'bin': {
                const op = node.op;
                if (op === '/') return { html: span('frac', span('fnum', go(node.l).html) + span('fden', go(node.r).html)), prec: 5 };
                if (op === '^') return { html: wrap(node.l, 6) + '<sup class="ec-sup">' + go(node.r).html + '</sup>', prec: 5 };
                if (op === '*') return { html: wrap(node.l, 3) + span('op', '·') + wrap(node.r, 3), prec: 3 };
                if (op === '+') return { html: wrap(node.l, 2) + span('op', '+') + wrap(node.r, 2), prec: 2 };
                if (op === '-') return { html: wrap(node.l, 2) + span('op', '−') + wrap(node.r, 3), prec: 2 };
                return { html: wrap(node.l, 2) + span('op', SIGNS[op] || escape(op)) + wrap(node.r, 2), prec: 1 };
            }
            case 'call': {
                const args = node.args.map(arg => go(arg).html);
                if (node.name === 'sqrt') return { html: span('sqrt', span('rad', '√') + span('radb', args[0])), prec: 5 };
                if (node.name === 'abs') return { html: '|' + args[0] + '|', prec: 6 };
                const name = node.name === 'log10' ? 'log<sub>10</sub>' : escape(node.name);
                return { html: span('fn', name) + '(' + args.join(', ') + ')', prec: 6 };
            }
            case 'eq': return { html: node.parts.map(part => go(part).html).join(span('eq', '=')), prec: 0 };
            default: return { html: escape(node.t), prec: 6 };
        }
    }

    const formulaHTML = (ast, ctx) => render(ast, ctx || {}, false).html;

    // The live line: "label = substituted right side = value". The first part of an equation that is a bare variable
    // keeps its label, and its own value is appended at the end.
    function liveHTML(ast, ctx) {
        const context = ctx || {};
        if (ast.t !== 'eq') return render(ast, context, true).html;
        const lead = ast.parts[0], target = lead.t === 'id' ? lead.name : '';
        const left = render(lead, context, !target).html;
        const rest = ast.parts.slice(1).map(part => render(part, context, true).html);
        const own = target && context.valueOf ? context.valueOf(target) : null;
        return [left].concat(rest, own ? [span('v', own.html)] : []).join(span('eq', '='));
    }

    window.ECalcTypeset = { escape, labelHTML, formulaHTML, liveHTML };
})();
