(function () {
    'use strict';

    function escapeHtml(value) {
        return String(value)
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;');
    }

    function escapeAttr(value) {
        return String(value)
            .replace(/&/g, '&amp;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#39;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;');
    }

    function containsLeakedToolMarkup(text) {
        if (!text || typeof text !== 'string') return false;
        return [
            /<\/?tool_call[^>]*>/i,
            /<\/?minimax:tool_call[^>]*>/i,
            /(?:^|\n)\s*minimax:tool_call\s*(?:\n|$)/i,
            /<invoke\b[^>]*>/i,
            /<parameter\b[^>]*>/i,
            /<\/?tts\b[^>]*>/i,
            /^\[Tool Output\]/im,
            /^Tool Output:/im,
            /\[Suggested next step\]/i,
            /"(action|tool|tool_call|tool_name)"\s*:/i,
            /"parameters"\s*:\s*\{/i
        ].some((pattern) => pattern.test(text));
    }

    function stripLeakedToolMarkup(text) {
        if (!text || typeof text !== 'string') return '';

        return text
            .replace(/<tool_call>[\s\S]*?<\/tool_call>/gi, '')
            .replace(/<\/?tool_call[^>]*>/gi, '')
            .replace(/<minimax:tool_call\b[^>]*>[\s\S]*?<\/minimax:tool_call>/gi, '')
            .replace(/<\/?minimax:tool_call[^>]*>/gi, '')
            .replace(/(?:^|\n)\s*minimax:tool_call\s*(?=\n|$)/gi, '\n')
            .replace(/<invoke\b[^>]*>[\s\S]*?<\/invoke>/gi, '')
            .replace(/<parameter\b[^>]*>[\s\S]*?<\/parameter>/gi, '')
            .replace(/<\/?(invoke|parameter)\b[^>]*>/gi, '')
            .replace(/<tts\b[^>]*>([\s\S]*?)<\/tts>/gi, (_, inner) => inner.trim())
            .replace(/<\/?tts\b[^>]*>/gi, '')
            .replace(/<done\s*\/?>/gi, '')
            .replace(/```(?:json)?\s*\{\s*"(?:action|tool|tool_call|tool_name)"[\s\S]*?\}\s*```/gi, '')
            .replace(/^```(?:json)?\n\{[\s\S]*?\}\n```$/gim, '')
            .replace(/^\{\s*"(?:action|tool|tool_call|tool_name)"[\s\S]*?\}\s*$/gim, '')
            .replace(/^\[Tool Output\]\s*$/gim, '')
            .replace(/^Tool Output:.*$/gim, '')
            .replace(/\n?\[Suggested next step\][\s\S]*$/i, '')
            .replace(/\n{3,}/g, '\n\n')
            .trim();
    }

    function prepareMarkdownContent(text) {
        const contentStripped = String(text || '').replace(
            /<external_data>([\s\S]*?)<\/external_data>/gi,
            (_match, inner) => inner.trim()
        );
        const thinkingBlocks = [];
        const contentForRender = contentStripped.replace(
            /<(thinking|think)>([\s\S]*?)<\/\1>/gi,
            (_match, _tag, inner) => {
                const idx = thinkingBlocks.length;
                thinkingBlocks.push(inner.trim());
                return `\n\n%%THINKING_BLOCK_${idx}%%\n\n`;
            }
        );
        return { contentForRender, thinkingBlocks };
    }

    function applyMarkdownLinkTargets(html) {
        return String(html || '').replace(
            /<a(\s+[^>]*)?\s+href="([^"]+)"/g,
            '<a$1href="$2" target="_blank" rel="noopener noreferrer"'
        );
    }

    function replaceThinkingPlaceholders(html, thinkingBlocks, renderBlock) {
        let output = String(html || '');
        const blocks = Array.isArray(thinkingBlocks) ? thinkingBlocks : [];
        blocks.forEach((innerText, idx) => {
            const detailsHtml = typeof renderBlock === 'function'
                ? renderBlock(innerText, idx)
                : String(innerText || '');
            output = output.replace(new RegExp(`<p>%%THINKING_BLOCK_${idx}%%</p>`, 'g'), detailsHtml);
            output = output.replace(new RegExp(`%%THINKING_BLOCK_${idx}%%`, 'g'), detailsHtml);
        });
        return output;
    }

    function normalizeTimestamp(timestamp) {
        const date = timestamp ? new Date(timestamp) : new Date();
        return Number.isNaN(date.getTime()) ? new Date() : date;
    }

    function formatTimestamp(timestamp) {
        const date = normalizeTimestamp(timestamp);
        try {
            return new Intl.DateTimeFormat(undefined, {
                dateStyle: 'short',
                timeStyle: 'short'
            }).format(date);
        } catch (_) {
            return date.toLocaleString();
        }
    }

    function createMarkdownRenderer(options) {
        if (window.AuraMarkdown && typeof window.AuraMarkdown.createMarkdownIt === 'function') {
            return window.AuraMarkdown.createMarkdownIt(options || {});
        }
        if (typeof window.markdownit === 'undefined') return null;
        return window.markdownit({ html: false, breaks: true, linkify: true });
    }

    window.AuraChatCore = {
        escapeHtml,
        escapeAttr,
        containsLeakedToolMarkup,
        stripLeakedToolMarkup,
        prepareMarkdownContent,
        applyMarkdownLinkTargets,
        replaceThinkingPlaceholders,
        normalizeTimestamp,
        formatTimestamp,
        createMarkdownRenderer
    };
})();
