// Milkdown plugins for Obsidian syntax in the Notes editor: wikilinks / embeds, hashtags, callouts, highlights.
// The markdown side (micromark + mdast) lives in notes-obsidian-markdown.js and is unit-tested in node.
import {$remark, $nodeSchema, $markSchema, $inputRule, $prose, $view} from '@milkdown/kit/utils';
import {InputRule} from '@milkdown/kit/prose/inputrules';
import {Plugin, PluginKey} from '@milkdown/kit/prose/state';
import {Decoration, DecorationSet} from '@milkdown/kit/prose/view';
import {toggleMark} from '@milkdown/kit/prose/commands';
import {obsidianSyntax, obsidianFromMarkdown, obsidianToMarkdown, parseWikiValue} from './notes-obsidian-markdown.js';

export {parseWikiValue};

const HOVER_DELAY_MS = 450;

const CALLOUT_ICONS = {
    note: 'info', info: 'info', todo: 'info',
    tip: 'star', hint: 'star', important: 'star',
    success: 'check', check: 'check', done: 'check',
    question: 'help-circle', help: 'help-circle', faq: 'help-circle',
    warning: 'alert-triangle', caution: 'alert-triangle', attention: 'alert-triangle',
    failure: 'alert-octagon', fail: 'alert-octagon', missing: 'alert-octagon', danger: 'alert-octagon', error: 'alert-octagon', bug: 'alert-octagon',
    example: 'list',
    quote: 'format-quote', cite: 'format-quote',
    abstract: 'file-text', summary: 'file-text', tldr: 'file-text'
};

const CALLOUT_CANONICAL = {
    note: 'note', info: 'info', todo: 'todo',
    tip: 'tip', hint: 'tip', important: 'tip',
    success: 'success', check: 'success', done: 'success',
    question: 'question', help: 'question', faq: 'question',
    warning: 'warning', caution: 'warning', attention: 'warning',
    failure: 'failure', fail: 'failure', missing: 'failure',
    danger: 'danger', error: 'danger',
    bug: 'bug',
    example: 'example',
    quote: 'quote', cite: 'quote',
    abstract: 'abstract', summary: 'abstract', tldr: 'abstract'
};

function iconForCallout(type) {
    return CALLOUT_ICONS[String(type || '').toLowerCase()] || 'info';
}

function canonicalCallout(type) {
    return CALLOUT_CANONICAL[String(type || '').toLowerCase()] || 'note';
}

function chipLabel(value) {
    const {target, subpath, alias} = parseWikiValue(value);
    if (alias) return alias;
    if (target) return target + (subpath ? ' › ' + subpath : '');
    return subpath || value;
}

function wikiNodeView(options) {
    return node => {
        let current = node;
        let hoverTimer = 0;
        const dom = document.createElement('span');

        const render = () => {
            const {value, embed} = current.attrs;
            dom.className = embed ? 'notes-embed' : 'notes-wikilink';
            dom.setAttribute('data-wiki-link', value);
            if (embed) dom.setAttribute('data-embed', ''); else dom.removeAttribute('data-embed');
            const resolved = options.resolve ? options.resolve(value) : null;
            if (resolved && resolved.exists === false) dom.setAttribute('data-unresolved', ''); else dom.removeAttribute('data-unresolved');
            dom.textContent = '';
            if (embed && typeof options.embed === 'function') {
                options.embed(value, dom, parseWikiValue(value));
                return;
            }
            dom.textContent = embed ? '![[' + value + ']]' : chipLabel(value);
        };
        render();

        dom.addEventListener('mousedown', event => event.preventDefault());
        dom.addEventListener('click', event => {
            event.preventDefault();
            if (typeof options.open === 'function') options.open(current.attrs.value, event);
        });
        dom.addEventListener('mouseenter', () => {
            window.clearTimeout(hoverTimer);
            if (typeof options.preview !== 'function') return;
            hoverTimer = window.setTimeout(() => options.preview(current.attrs.value, dom), HOVER_DELAY_MS);
        });
        dom.addEventListener('mouseleave', () => {
            window.clearTimeout(hoverTimer);
            if (typeof options.preview === 'function') options.preview(null, dom);
        });

        return {
            dom,
            update(next) {
                if (next.type !== current.type) return false;
                const changed = next.attrs.value !== current.attrs.value || next.attrs.embed !== current.attrs.embed;
                current = next;
                if (changed) render();
                return true;
            },
            ignoreMutation: () => true,
            stopEvent: event => event.type.startsWith('mouse') || event.type === 'click',
            destroy() { window.clearTimeout(hoverTimer); }
        };
    };
}

function hashtagNodeView(options) {
    return node => {
        let current = node;
        const dom = document.createElement('span');
        const render = () => {
            dom.className = 'notes-hashtag';
            dom.setAttribute('data-hashtag', current.attrs.value);
            dom.textContent = '#' + current.attrs.value;
        };
        render();
        dom.addEventListener('mousedown', event => event.preventDefault());
        dom.addEventListener('click', event => {
            event.preventDefault();
            if (typeof options.tag === 'function') options.tag(current.attrs.value, event);
        });
        return {
            dom,
            update(next) {
                if (next.type !== current.type) return false;
                const changed = next.attrs.value !== current.attrs.value;
                current = next;
                if (changed) render();
                return true;
            },
            ignoreMutation: () => true,
            stopEvent: event => event.type.startsWith('mouse') || event.type === 'click'
        };
    };
}

function calloutMarkerNodeView(options) {
    return (node, view, getPos, decorations) => {
        let current = node;
        let label = null;
        // The callout plugin marks the marker atom when the first line carries a title: the title is the header text then.
        const applyTitle = decos => {
            if (label) label.hidden = !!(decos && decos.some(d => d.spec && d.spec.calloutHasTitle));
        };
        const dom = document.createElement('span');
        const render = () => {
            const {value, fold} = current.attrs;
            dom.className = 'notes-callout-marker';
            dom.setAttribute('data-callout', value);
            if (fold) dom.setAttribute('data-fold', fold); else dom.removeAttribute('data-fold');
            dom.textContent = '';
            const icon = document.createElement('span');
            icon.className = 'notes-callout-icon';
            icon.innerHTML = (typeof options.icon === 'function' && options.icon(iconForCallout(value))) || '';
            label = document.createElement('span');
            label.className = 'notes-callout-label';
            label.textContent = (typeof options.label === 'function' && options.label(value)) || value;
            dom.append(icon, label);
            if (fold) {
                const sign = document.createElement('span');
                sign.className = 'notes-callout-fold';
                sign.textContent = fold;
                dom.append(sign);
            }
        };
        render();
        applyTitle(decorations);
        return {
            dom,
            update(next, decos) {
                if (next.type !== current.type) return false;
                const changed = next.attrs.value !== current.attrs.value || next.attrs.fold !== current.attrs.fold;
                current = next;
                if (changed) render();
                applyTitle(decos);
                return true;
            },
            ignoreMutation: () => true,
            stopEvent: event => event.type.startsWith('mouse') || event.type === 'click'
        };
    };
}

// Header line of a callout: [!type] Title, soft break, body. The Markdown keeps title and body in one paragraph, so the
// layout is done with decorations only: a styled title span, a block-forcing widget at the soft break, and a flag on
// the marker atom that hides the type label when a title is present.
function calloutHeaderDecorations(paragraph, contentStart, decorations) {
    const marker = paragraph.firstChild;
    const titleStart = contentStart + marker.nodeSize;
    let offset = marker.nodeSize;
    let breakAt = -1;
    let hasTitle = false;
    let hasBody = false;
    for (let i = 1; i < paragraph.childCount; i++) {
        const child = paragraph.child(i);
        const blank = child.isText && child.text.trim() === '';
        if (breakAt < 0) {
            if (child.type.name === 'hardbreak') breakAt = contentStart + offset;
            else if (!blank) hasTitle = true;
        } else if (!blank) {
            hasBody = true;
        }
        offset += child.nodeSize;
    }
    const titleEnd = breakAt >= 0 ? breakAt : contentStart + offset;
    if (hasTitle && titleEnd > titleStart) {
        decorations.push(Decoration.inline(titleStart, titleEnd, {class: 'notes-callout-title', style: 'font-weight:650;color:var(--callout, currentColor)'}));
    }
    decorations.push(Decoration.node(contentStart, titleStart, {}, {calloutHasTitle: hasTitle}));
    if (breakAt >= 0 && hasBody) {
        decorations.push(Decoration.widget(breakAt, () => {
            const br = document.createElement('span');
            br.className = 'notes-callout-break';
            br.setAttribute('style', 'display:block;height:0');
            br.setAttribute('aria-hidden', 'true');
            return br;
        }, {side: -1, key: 'notes-callout-break', ignoreSelection: true}));
    }
}

function calloutDecorations(doc) {
    const decorations = [];
    doc.descendants((node, pos) => {
        if (node.type.name !== 'blockquote') return true;
        const paragraph = node.firstChild;
        const marker = paragraph && paragraph.type.name === 'paragraph' ? paragraph.firstChild : null;
        if (marker && marker.type.name === 'callout_marker') {
            const attrs = {class: 'notes-callout', 'data-callout': canonicalCallout(marker.attrs.value)};
            if (marker.attrs.fold) attrs['data-fold'] = marker.attrs.fold;
            decorations.push(Decoration.node(pos, pos + node.nodeSize, attrs));
            calloutHeaderDecorations(paragraph, pos + 2, decorations);
        }
        return true;
    });
    return DecorationSet.create(doc, decorations);
}

export function obsidian(options = {}) {
    const remark = $remark('obsidianSyntax', () => function () {
        const data = this.data();
        (data.micromarkExtensions ||= []).push(obsidianSyntax());
        (data.fromMarkdownExtensions ||= []).push(obsidianFromMarkdown());
        (data.toMarkdownExtensions ||= []).push(obsidianToMarkdown());
    });

    const wikiLink = $nodeSchema('wiki_link', () => ({
        group: 'inline',
        inline: true,
        atom: true,
        selectable: true,
        attrs: {value: {default: ''}, embed: {default: false}},
        leafText: node => (node.attrs.embed ? '!' : '') + '[[' + node.attrs.value + ']]',
        parseDOM: [{tag: 'span[data-wiki-link]', getAttrs: dom => ({value: dom.getAttribute('data-wiki-link') || '', embed: dom.hasAttribute('data-embed')})}],
        toDOM: node => ['span', {'data-wiki-link': node.attrs.value, ...(node.attrs.embed ? {'data-embed': ''} : {})}, node.attrs.value],
        parseMarkdown: {
            match: node => node.type === 'wikiLink',
            runner: (state, node, type) => { state.addNode(type, {value: node.value, embed: !!node.embed}); }
        },
        toMarkdown: {
            match: node => node.type.name === 'wiki_link',
            runner: (state, node) => { state.addNode('wikiLink', undefined, undefined, {value: node.attrs.value, embed: !!node.attrs.embed}); }
        }
    }));

    const hashtag = $nodeSchema('hashtag', () => ({
        group: 'inline',
        inline: true,
        atom: true,
        selectable: true,
        attrs: {value: {default: ''}},
        leafText: node => '#' + node.attrs.value,
        parseDOM: [{tag: 'span[data-hashtag]', getAttrs: dom => ({value: dom.getAttribute('data-hashtag') || ''})}],
        toDOM: node => ['span', {'data-hashtag': node.attrs.value}, '#' + node.attrs.value],
        parseMarkdown: {
            match: node => node.type === 'hashtag',
            runner: (state, node, type) => { state.addNode(type, {value: node.value}); }
        },
        toMarkdown: {
            match: node => node.type.name === 'hashtag',
            runner: (state, node) => { state.addNode('hashtag', undefined, undefined, {value: node.attrs.value}); }
        }
    }));

    const calloutMarker = $nodeSchema('callout_marker', () => ({
        group: 'inline',
        inline: true,
        atom: true,
        selectable: true,
        attrs: {value: {default: 'note'}, fold: {default: ''}},
        leafText: node => '[!' + node.attrs.value + ']' + node.attrs.fold,
        parseDOM: [{tag: 'span[data-callout-marker]', getAttrs: dom => ({value: dom.getAttribute('data-callout-marker') || 'note', fold: dom.getAttribute('data-fold') || ''})}],
        toDOM: node => ['span', {'data-callout-marker': node.attrs.value, ...(node.attrs.fold ? {'data-fold': node.attrs.fold} : {})}, '[!' + node.attrs.value + ']' + node.attrs.fold],
        parseMarkdown: {
            match: node => node.type === 'calloutMarker',
            runner: (state, node, type) => { state.addNode(type, {value: node.value, fold: node.fold || ''}); }
        },
        toMarkdown: {
            match: node => node.type.name === 'callout_marker',
            runner: (state, node) => { state.addNode('calloutMarker', undefined, undefined, {value: node.attrs.value, fold: node.attrs.fold || ''}); }
        }
    }));

    const highlight = $markSchema('highlight', () => ({
        parseDOM: [{tag: 'mark'}],
        toDOM: () => ['mark', {class: 'notes-highlight'}, 0],
        parseMarkdown: {
            match: node => node.type === 'highlight',
            runner: (state, node, type) => {
                state.openMark(type);
                state.next(node.children);
                state.closeMark(type);
            }
        },
        toMarkdown: {
            match: mark => mark.type.name === 'highlight',
            runner: (state, mark) => { state.withMark(mark, 'highlight'); }
        }
    }));

    const views = [
        $view(wikiLink.node, () => wikiNodeView(options)),
        $view(hashtag.node, () => hashtagNodeView(options)),
        $view(calloutMarker.node, () => calloutMarkerNodeView(options))
    ];

    const calloutPlugin = $prose(() => {
        let cachedDoc = null;
        let cachedSet = DecorationSet.empty;
        return new Plugin({
            key: new PluginKey('notesCallouts'),
            props: {
                decorations(state) {
                    if (state.doc !== cachedDoc) {
                        cachedDoc = state.doc;
                        cachedSet = calloutDecorations(state.doc);
                    }
                    return cachedSet;
                }
            }
        });
    });

    // Input rules see inline atoms as U+FFFC and cannot rebuild them from the matched text: a match that holds one
    // (or sits in inline code) must leave the document alone.
    const OBJECT_REPLACEMENT = String.fromCharCode(0xfffc);
    const inInlineCode = (state, from, to) => {
        const code = state.schema.marks.inlineCode;
        return !!code && state.doc.rangeHasMark(from, to, code);
    };

    const wikiRule = $inputRule(ctx => new InputRule(/(!?)\[\[([^\[\]\n]{1,500})\]\]$/, (state, match, start, end) => {
        if (match[2].includes(OBJECT_REPLACEMENT) || inInlineCode(state, start, end)) return null;
        const node = wikiLink.type(ctx).create({value: match[2], embed: match[1] === '!'});
        return state.tr.replaceWith(start, end, node);
    }));

    const tagRule = $inputRule(ctx => new InputRule(/(^|\s|\()#([\p{L}\p{N}_\-\/]*[\p{L}_\-\/][\p{L}\p{N}_\-\/]*)\s$/u, (state, match, start, end) => {
        const nodes = [];
        if (match[1]) nodes.push(state.schema.text(match[1]));
        nodes.push(hashtag.type(ctx).create({value: match[2]}));
        nodes.push(state.schema.text(' '));
        return state.tr.replaceWith(start, end, nodes);
    }));

    const highlightRule = $inputRule(ctx => new InputRule(/(?:^|[^=])==([^=\s](?:[^=]*[^=\s])?)==$/, (state, match, start, end) => {
        const lead = match[0].length - match[1].length - 4;
        const type = highlight.type(ctx);
        const open = start + lead;
        if (inInlineCode(state, open, end)) return null;
        // Keep the typed content as it is (atoms, bold, links): drop the two "==" delimiters and mark what is between.
        // The closing delimiter is one "=" in the document plus the "=" being typed.
        const tr = state.tr.delete(end - 1, end).delete(open, open + 2);
        tr.addMark(open, open + match[1].length, type.create());
        return tr.removeStoredMark(type);
    }));

    const calloutRule = $inputRule(ctx => new InputRule(/^\[!([A-Za-z0-9_-]+)\]([+-]?)\s$/, (state, match, start, end) => {
        const $start = state.doc.resolve(start);
        if ($start.depth < 2 || $start.parent.type.name !== 'paragraph') return null;
        if ($start.node($start.depth - 1).type.name !== 'blockquote' || $start.index($start.depth - 1) !== 0) return null;
        return state.tr.replaceWith(start, end, [
            calloutMarker.type(ctx).create({value: match[1], fold: match[2] || ''}),
            state.schema.text(' ')
        ]);
    }));

    return [remark, wikiLink, hashtag, calloutMarker, highlight, ...views, calloutPlugin, wikiRule, tagRule, highlightRule, calloutRule].flat(Infinity);
}

export const obsidianCommands = {
    toggleHighlight(state, dispatch) {
        const type = state.schema.marks.highlight;
        return type ? toggleMark(type)(state, dispatch) : false;
    },
    insertWiki(view, value, embed = false) {
        const type = view.state.schema.nodes.wiki_link;
        if (!type) return false;
        const tr = view.state.tr.replaceSelectionWith(type.create({value, embed: !!embed}), false);
        tr.insertText(' ');
        view.dispatch(tr.scrollIntoView());
        view.focus();
        return true;
    },
    insertCallout(view, type = 'note', title = '') {
        const {schema} = view.state;
        if (!schema.nodes.callout_marker || !schema.nodes.blockquote) return false;
        const inline = [schema.nodes.callout_marker.create({value: type, fold: ''})];
        if (title) inline.push(schema.text(' ' + title));
        const quote = schema.nodes.blockquote.create(null, schema.nodes.paragraph.create(null, inline));
        view.dispatch(view.state.tr.replaceSelectionWith(quote, false).scrollIntoView());
        view.focus();
        return true;
    }
};
