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
        normalizeTimestamp,
        formatTimestamp,
        createMarkdownRenderer
    };
})();
