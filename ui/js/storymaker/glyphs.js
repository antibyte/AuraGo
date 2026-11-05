// Storymaker player: glyph coverage in JS, read from the catalog's coverage file (catalog.glyph_data, schema
// storymaker-glyphs/1). It answers which fallback face draws a code point, which faces a text needs, the family chain for
// ctx.font and the faces a code scene gets for its language. Only the tables of SM.ranges decide, never ICU. Classic script;
// publishes StorymakerCore.glyphs.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var LANGUAGE_FACES = { el: ['sm-greek', 'sm-greek-ext'], hi: ['sm-deva'], ar: ['sm-arab'], zh: ['sm-cjk'], ja: ['sm-cjk'] };

    // decode(d): d is [start0, len0, gap1, len1, ...] with gap = start - previous end and len = end - start. The result is the
    // flat list [start0, end0, start1, end1, ...]. An end is inclusive.
    function decode(d) {
        var out = [], prev = 0;
        for (var i = 0; i + 1 < d.length; i += 2) {
            var start = prev + d[i], end = start + d[i + 1];
            out.push(start, end);
            prev = end;
        }
        return out;
    }

    function covers(ranges, cp) {
        var lo = 0, hi = ranges.length / 2 - 1;
        while (lo <= hi) {
            var mid = (lo + hi) >> 1;
            if (cp < ranges[mid * 2]) hi = mid - 1;
            else if (cp > ranges[mid * 2 + 1]) lo = mid + 1;
            else return true;
        }
        return false;
    }

    // index(glyphData) = {faces, ranges}, in fallback order, or null without glyph data.
    function index(glyphData) {
        if (!glyphData || !Array.isArray(glyphData.faces)) return null;
        return {
            faces: glyphData.faces,
            ranges: glyphData.faces.map(function (face) { return decode(face && Array.isArray(face.d) ? face.d : []); })
        };
    }

    // faceOf(idx, cp) = the index of the first covering face, -1 when none.
    function faceOf(idx, cp) {
        if (!idx) return -1;
        for (var i = 0; i < idx.ranges.length; i++) if (covers(idx.ranges[i], cp)) return i;
        return -1;
    }

    // needsFor(idx, text) = the faces a text draws from, each once. Marks add no need, because compile may not normalize.
    function needsFor(idx, text) {
        var out = [], R = SM.ranges, s = String(text === undefined || text === null ? '' : text);
        if (!idx) return out;
        for (var i = 0; i < s.length;) {
            var cp = s.codePointAt(i);
            i += cp > 0xFFFF ? 2 : 1;
            if (R.isDefaultIgnorable(cp) || R.isMark(cp) || R.isSpace(cp)) continue;
            var at = faceOf(idx, cp);
            if (at >= 0 && out.indexOf(idx.faces[at]) < 0) out.push(idx.faces[at]);
        }
        return out;
    }

    // chain(idx, styleFamily) = the family list for ctx.font, with the style family in the place of Geist.
    function chain(idx, styleFamily) {
        var out = [];
        if (!idx) return out;
        idx.faces.forEach(function (face) {
            var family = face.family === 'Geist' ? styleFamily : face.family;
            if (out.indexOf(family) < 0) out.push(family);
        });
        return out;
    }

    // facesForLanguage(idx, lang): the same base rule as Go's LanguageBase, because the validator accepts any language string.
    function facesForLanguage(idx, lang) {
        if (!idx) return [];
        var base = String(lang === undefined || lang === null ? '' : lang).toLowerCase().split(/[-_]/)[0];
        var wanted = Object.prototype.hasOwnProperty.call(LANGUAGE_FACES, base) ? LANGUAGE_FACES[base] : [];
        return idx.faces.filter(function (face) { return wanted.indexOf(face.id) >= 0; });
    }

    function key(face) { return 'font:' + face.id + '/400'; }

    SM.glyphs = { index: index, faceOf: faceOf, needsFor: needsFor, chain: chain, facesForLanguage: facesForLanguage, key: key };
})(typeof globalThis !== 'undefined' ? globalThis : self);
