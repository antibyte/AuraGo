/* Aura Drift - the text side of the Underdog Woof voice commentary.
 *
 * The model speaks english only, so the commentary reads the english original of a
 * line in every language of the game. This file holds the character vocabulary of the
 * model, the way numbers are said and the rule that cuts a long line into pieces.
 *
 * The runtime (aura-drift-woof-runtime.js) reads the text from here.
 */
(function () {
    'use strict';

    /* The character vocabulary of the model, in the order of the reference runtime. */
    const CHARS = " abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789.,!?;:'\"-()&/%$\u20ac\u00a3@#+=*_<>[]\u2026\u2014";
    const VOCAB = Object.create(null);
    for (let i = 0; i < CHARS.length; i++) VOCAB[CHARS[i]] = i + 3;

    function encodeText(text) {
        const ids = [1];
        for (const ch of text) {
            const id = VOCAB[ch];
            ids.push(id === undefined ? 2 : id);
        }
        return ids;
    }

    /* The model needs its text the way people say it. The banter lines of the game
     * are plain english sentences, so only the digits are rewritten here. */
    const ONES = ['zero', 'one', 'two', 'three', 'four', 'five', 'six', 'seven', 'eight', 'nine', 'ten',
        'eleven', 'twelve', 'thirteen', 'fourteen', 'fifteen', 'sixteen', 'seventeen', 'eighteen', 'nineteen'];
    const TENS = ['', '', 'twenty', 'thirty', 'forty', 'fifty', 'sixty', 'seventy', 'eighty', 'ninety'];

    function numberToWords(n) {
        if (n < 0) return 'minus ' + numberToWords(-n);
        if (n < 20) return ONES[n];
        if (n < 100) {
            const t = TENS[(n / 10) | 0], r = n % 10;
            return r ? t + ' ' + ONES[r] : t;
        }
        if (n < 1000) {
            const h = ONES[(n / 100) | 0] + ' hundred', r = n % 100;
            return r ? h + ' and ' + numberToWords(r) : h;
        }
        return String(n);
    }

    function normalise(text) {
        return text.replace(/\d+/g, m => numberToWords(parseInt(m, 10)));
    }

    /* Long lines are spoken in pieces, like the reference does. */
    function splitText(text, clauseMax) {
        clauseMax = clauseMax || 120;
        const out = [];
        const sentences = text.trim().split(/(?<=[.!?…])\s+/);
        for (const s of sentences) {
            if (!s) continue;
            if (s.length <= clauseMax) { out.push(s); continue; }
            const clauses = s.split(/(?<=[,;:])\s+|\s*—\s*|\s*--\s*/).filter(Boolean);
            let cur = '';
            for (const c of clauses) {
                if (cur && cur.length + 1 + c.length > clauseMax) { out.push(cur); cur = c; }
                else cur = cur ? cur + ' ' + c : c;
            }
            if (cur) out.push(cur);
        }
        return out;
    }


    self.AuraDriftWoofText = {
        VOCAB: VOCAB,
        encodeText: encodeText,
        normalise: normalise,
        splitText: splitText
    };
})();
