// Storymaker player: the cue map. One function places cues and caption lines on the audio of a block. With measured pieces
// whose chars sum to the text length, an offset inside a piece maps proportionally over that piece's used audio from its
// at_ms, so a marker at a sentence start lands on the onset of that sentence. Otherwise the whole block is proportional over
// its duration. Offsets are UTF-16 units of the stripped text. Classic script; publishes StorymakerCore.cuemap.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    // prepare(rawBlock, length) = a copy of the block with length, starts[] (cumulative UTF-16 offsets) and placed.
    function prepare(rawBlock, length) {
        var out = {}, k, pieces = Array.isArray(rawBlock.pieces) ? rawBlock.pieces : [], starts = [], sum = 0;
        for (k in rawBlock) out[k] = rawBlock[k];
        for (var i = 0; i < pieces.length; i++) {
            starts.push(sum);
            sum += pieces[i].chars;
        }
        out.pieces = pieces;
        out.duration_ms = rawBlock.duration_ms > 0 ? rawBlock.duration_ms : 0;
        out.length = length;
        out.starts = starts;
        out.placed = rawBlock.speech_ok === true && pieces.length > 0 && sum === length ? 'span' : 'proportion';
        return out;
    }

    // estimated(id, seconds, length) = the block of a text without audio.
    function estimated(id, seconds, length) {
        return { id: id, placed: 'proportion', duration_ms: seconds * 1000, length: length, pieces: [], starts: [] };
    }

    // cueMap(block, utf16Offset) = ms from the block start. An offset equal to the length is the end of the last piece.
    function cueMap(block, offset) {
        if (block.placed !== 'span') return offset / Math.max(1, block.length) * block.duration_ms;
        var pieces = block.pieces, k = pieces.length - 1;
        for (var i = 0; i < pieces.length; i++) {
            if (offset < block.starts[i] + pieces[i].chars) { k = i; break; }
        }
        var p = pieces[k], share = p.chars > 0 ? (offset - block.starts[k]) / p.chars : 0;
        return p.at_ms + share * (p.out_ms - p.in_ms);
    }

    SM.cuemap = { prepare: prepare, estimated: estimated, cueMap: cueMap };
})(typeof globalThis !== 'undefined' ? globalThis : self);
