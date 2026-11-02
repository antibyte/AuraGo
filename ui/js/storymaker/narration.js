// Storymaker player: narration text. Cue markers {n} are stripped from the spoken text and remembered as character
// positions; a cue time is its character position over the text length, scaled by the duration of the narration block.
// Without measured audio the duration is estimated as words / 2.3 s (minimum 1 s). Classic script; publishes
// StorymakerCore.narration.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var WORDS_PER_SECOND = 2.3;
    var MIN_SECONDS = 1.0;
    var TAIL_SECONDS = 0.4;

    // parse(text) = {plain, cues: {n: charIndex}, words, length}. Whitespace collapses to single spaces; a cue marker keeps
    // the position in the plain text where it stood.
    function parse(text) {
        var src = String(text === undefined || text === null ? '' : text);
        var plain = '', cues = {}, re = /\{(\d+)\}/g, last = 0, m;
        function append(chunk) {
            chunk = chunk.replace(/\s+/g, ' ');
            if (!plain || plain.charAt(plain.length - 1) === ' ') chunk = chunk.replace(/^ /, '');
            plain += chunk;
        }
        while ((m = re.exec(src)) !== null) {
            append(src.slice(last, m.index));
            var n = parseInt(m[1], 10);
            if (cues[n] === undefined) cues[n] = plain.length;
            last = m.index + m[0].length;
        }
        append(src.slice(last));
        var trimmed = plain.replace(/ $/, '');
        var words = trimmed ? trimmed.split(' ').length : 0;
        Object.keys(cues).forEach(function (k) { if (cues[k] > trimmed.length) cues[k] = trimmed.length; });
        return { plain: trimmed, cues: cues, words: words, length: trimmed.length };
    }

    // estimate(words) = seconds; no words, no narration.
    function estimate(words) {
        return words > 0 ? Math.max(MIN_SECONDS, words / WORDS_PER_SECOND) : 0;
    }

    // cueTimes(parsed, start, seconds) = {n: scene-local seconds}, proportional to the character position.
    function cueTimes(parsed, start, seconds) {
        var out = {}, len = Math.max(1, parsed.length);
        Object.keys(parsed.cues).forEach(function (k) { out[k] = start + parsed.cues[k] / len * seconds; });
        return out;
    }

    SM.narration = {
        parse: parse, estimate: estimate, cueTimes: cueTimes, WORDS_PER_SECOND: WORDS_PER_SECOND,
        MIN_SECONDS: MIN_SECONDS, TAIL_SECONDS: TAIL_SECONDS
    };
})(typeof globalThis !== 'undefined' ? globalThis : self);
