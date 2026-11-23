// Obsidian syntax for the Notes editor as a pure micromark / mdast layer (no DOM, no Milkdown).
//
//   [[target#sub|alias]]  -> wikiLink {value, embed: false}   (value is the raw inner text)
//   ![[file.png]]         -> wikiLink {value, embed: true}
//   #tag, #a/b            -> hashtag {value}
//   [!type]+/-            -> calloutMarker {value, fold}      (first thing of a paragraph)
//   ==text==              -> highlight {children}
import {markdownLineEnding, markdownLineEndingOrSpace, markdownSpace, unicodeWhitespace} from 'micromark-util-character';
import {splice} from 'micromark-util-chunked';
import {classifyCharacter} from 'micromark-util-classify-character';
import {resolveAll} from 'micromark-util-resolve-all';

const MAX_WIKI_LENGTH = 500;
const TAG_CHAR = /[\p{L}\p{N}_\-\/]/u;
const TAG_DIGIT = /\p{N}/u;

function isTagChar(code) {
    return code !== null && code > 0 && TAG_CHAR.test(String.fromCharCode(code));
}

// ---- wikilinks and embeds -------------------------------------------------

function wikiLinkConstruct(embed) {
    return {name: embed ? 'obsidianEmbed' : 'obsidianWikiLink', tokenize};

    function tokenize(effects, ok, nok) {
        let size = 0;
        let content = false;
        return start;

        function start(code) {
            effects.enter('wikiLink');
            if (embed) {
                if (code !== 33) return nok(code);
                effects.enter('wikiLinkEmbedMarker');
                effects.consume(code);
                effects.exit('wikiLinkEmbedMarker');
                return openFirst;
            }
            return openFirst(code);
        }

        function openFirst(code) {
            if (code !== 91) return nok(code);
            effects.enter('wikiLinkMarker');
            effects.consume(code);
            return openSecond;
        }

        function openSecond(code) {
            if (code !== 91) return nok(code);
            effects.consume(code);
            effects.exit('wikiLinkMarker');
            effects.enter('wikiLinkValue');
            return value;
        }

        function value(code) {
            if (code === null || markdownLineEnding(code) || code === 91) return nok(code);
            if (code === 93) {
                if (!content) return nok(code);
                effects.exit('wikiLinkValue');
                effects.enter('wikiLinkMarker');
                effects.consume(code);
                return closeSecond;
            }
            if (++size > MAX_WIKI_LENGTH) return nok(code);
            if (!markdownLineEndingOrSpace(code)) content = true;
            effects.consume(code);
            return code === 92 ? escaped : value;
        }

        function escaped(code) {
            if (code === null || markdownLineEnding(code)) return nok(code);
            size++;
            content = true;
            effects.consume(code);
            return value;
        }

        function closeSecond(code) {
            if (code !== 93) return nok(code);
            effects.consume(code);
            effects.exit('wikiLinkMarker');
            effects.exit('wikiLink');
            return ok;
        }
    }
}

// ---- hashtags -------------------------------------------------------------

const hashtagConstruct = {name: 'obsidianHashtag', tokenize: tokenizeHashtag};

function tokenizeHashtag(effects, ok, nok) {
    const previous = this.previous;
    let size = 0;
    let named = false;
    return start;

    function start(code) {
        if (previous !== null && previous !== 40 && !markdownLineEndingOrSpace(previous) && !unicodeWhitespace(previous)) return nok(code);
        effects.enter('hashtag');
        effects.enter('hashtagMarker');
        effects.consume(code);
        effects.exit('hashtagMarker');
        effects.enter('hashtagValue');
        return body;
    }

    function body(code) {
        if (isTagChar(code)) {
            size++;
            if (!TAG_DIGIT.test(String.fromCharCode(code))) named = true;
            effects.consume(code);
            return body;
        }
        if (!size || !named) return nok(code);
        effects.exit('hashtagValue');
        effects.exit('hashtag');
        return ok(code);
    }
}

// ---- callout markers ------------------------------------------------------

const calloutConstruct = {name: 'obsidianCallout', tokenize: tokenizeCallout};

function tokenizeCallout(effects, ok, nok) {
    const self = this;
    let size = 0;
    return start;

    function start(code) {
        if (self.previous !== null) return nok(code);
        effects.enter('calloutMarker');
        effects.enter('calloutMarkerOpen');
        effects.consume(code);
        return bang;
    }

    function bang(code) {
        if (code !== 33) return nok(code);
        effects.consume(code);
        effects.exit('calloutMarkerOpen');
        effects.enter('calloutMarkerType');
        return type;
    }

    function isTypeChar(code) {
        return (code >= 48 && code <= 57) || (code >= 65 && code <= 90) || (code >= 97 && code <= 122) || code === 95 || code === 45;
    }

    function type(code) {
        if (code !== null && isTypeChar(code)) {
            size++;
            effects.consume(code);
            return type;
        }
        if (code !== 93 || !size) return nok(code);
        effects.exit('calloutMarkerType');
        effects.enter('calloutMarkerClose');
        effects.consume(code);
        effects.exit('calloutMarkerClose');
        return fold;
    }

    function fold(code) {
        if (code === 43 || code === 45) {
            effects.enter('calloutMarkerFold');
            effects.consume(code);
            effects.exit('calloutMarkerFold');
            return end;
        }
        return end(code);
    }

    function end(code) {
        if (code !== null && !markdownLineEnding(code) && !markdownSpace(code)) return nok(code);
        effects.exit('calloutMarker');
        return ok(code);
    }
}

// ---- highlights (port of micromark-extension-gfm-strikethrough, marker "=") ----

const highlightConstruct = {name: 'obsidianHighlight', tokenize: tokenizeHighlight, resolveAll: resolveAllHighlight};

function resolveAllHighlight(events, context) {
    let index = -1;
    while (++index < events.length) {
        if (events[index][0] === 'enter' && events[index][1].type === 'highlightSequenceTemporary' && events[index][1]._close) {
            let open = index;
            while (open--) {
                if (events[open][0] === 'exit' && events[open][1].type === 'highlightSequenceTemporary' && events[open][1]._open
                    && events[index][1].end.offset - events[index][1].start.offset === events[open][1].end.offset - events[open][1].start.offset) {
                    events[index][1].type = 'highlightSequence';
                    events[open][1].type = 'highlightSequence';
                    const highlight = {type: 'highlight', start: Object.assign({}, events[open][1].start), end: Object.assign({}, events[index][1].end)};
                    const text = {type: 'highlightText', start: Object.assign({}, events[open][1].end), end: Object.assign({}, events[index][1].start)};
                    const nextEvents = [['enter', highlight, context], ['enter', events[open][1], context], ['exit', events[open][1], context], ['enter', text, context]];
                    const insideSpan = context.parser.constructs.insideSpan.null;
                    if (insideSpan) splice(nextEvents, nextEvents.length, 0, resolveAll(insideSpan, events.slice(open + 1, index), context));
                    splice(nextEvents, nextEvents.length, 0, [['exit', text, context], ['enter', events[index][1], context], ['exit', events[index][1], context], ['exit', highlight, context]]);
                    splice(events, open - 1, index - open + 3, nextEvents);
                    index = open + nextEvents.length - 2;
                    break;
                }
            }
        }
    }
    index = -1;
    while (++index < events.length) {
        if (events[index][1].type === 'highlightSequenceTemporary') events[index][1].type = 'data';
    }
    return events;
}

function tokenizeHighlight(effects, ok, nok) {
    const previous = this.previous;
    const events = this.events;
    let size = 0;
    return start;

    function start(code) {
        if (previous === 61 && events[events.length - 1][1].type !== 'characterEscape') return nok(code);
        effects.enter('highlightSequenceTemporary');
        return more(code);
    }

    function more(code) {
        const before = classifyCharacter(previous);
        if (code === 61) {
            if (size > 1) return nok(code);
            effects.consume(code);
            size++;
            return more;
        }
        if (size < 2) return nok(code);
        const token = effects.exit('highlightSequenceTemporary');
        const after = classifyCharacter(code);
        token._open = !after || (after === 2 && Boolean(before));
        token._close = !before || (before === 2 && Boolean(after));
        return ok(code);
    }
}

// ---- public API -----------------------------------------------------------

export function obsidianSyntax() {
    return {
        text: {
            33: [wikiLinkConstruct(true)],
            35: [hashtagConstruct],
            61: [highlightConstruct],
            91: [wikiLinkConstruct(false), calloutConstruct]
        },
        insideSpan: {null: [highlightConstruct]},
        attentionMarkers: {null: [61]}
    };
}

export function obsidianFromMarkdown() {
    return {
        enter: {
            wikiLink(token) { this.enter({type: 'wikiLink', value: '', embed: false}, token); },
            hashtag(token) { this.enter({type: 'hashtag', value: ''}, token); },
            calloutMarker(token) { this.enter({type: 'calloutMarker', value: '', fold: ''}, token); },
            highlight(token) { this.enter({type: 'highlight', children: []}, token); }
        },
        exit: {
            wikiLinkEmbedMarker() { this.stack[this.stack.length - 1].embed = true; },
            wikiLinkValue(token) { this.stack[this.stack.length - 1].value = this.sliceSerialize(token); },
            wikiLink(token) { this.exit(token); },
            hashtagValue(token) { this.stack[this.stack.length - 1].value = this.sliceSerialize(token); },
            hashtag(token) { this.exit(token); },
            calloutMarkerType(token) { this.stack[this.stack.length - 1].value = this.sliceSerialize(token); },
            calloutMarkerFold(token) { this.stack[this.stack.length - 1].fold = this.sliceSerialize(token); },
            calloutMarker(token) { this.exit(token); },
            highlight(token) { this.exit(token); }
        }
    };
}

function handleHighlight(node, _, state, info) {
    const tracker = state.createTracker(info);
    const exit = state.enter('highlight');
    let value = tracker.move('==');
    value += state.containerPhrasing(node, {...tracker.current(), before: '=', after: '='});
    value += tracker.move('==');
    exit();
    return value;
}
handleHighlight.peek = () => '=';

export function obsidianToMarkdown() {
    return {
        handlers: {
            wikiLink: node => (node.embed ? '!' : '') + '[[' + node.value + ']]',
            hashtag: node => '#' + node.value,
            calloutMarker: node => '[!' + node.value + ']' + (node.fold || ''),
            highlight: handleHighlight
        },
        // A literal "==" in text must not turn into a highlight on the next parse. Only "==" directly followed
        // by a non-space, non-"=" character can open one, so prose like "a == b" stays untouched.
        unsafe: [{character: '=', before: '=', after: '[^\\s=]', inConstruct: 'phrasing'}]
    };
}

// "\|" is the alias separator written inside table cells; it splits like a plain "|".
export function parseWikiValue(value) {
    const text = String(value ?? '');
    const split = text.indexOf('|');
    const alias = split >= 0 ? text.slice(split + 1).trim() : '';
    const left = (split >= 0 ? text.slice(0, split) : text).replace(/\x5c$/, '');
    const hash = left.indexOf('#');
    return {
        target: (hash >= 0 ? left.slice(0, hash) : left).trim(),
        subpath: hash >= 0 ? left.slice(hash + 1).trim() : '',
        alias
    };
}
