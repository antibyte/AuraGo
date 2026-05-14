// AuraGo shared Markdown rendering helpers.
(function () {
    'use strict';

    function escapeHtml(value) {
        const div = document.createElement('div');
        div.textContent = String(value == null ? '' : value);
        return div.innerHTML;
    }

    function isChartLanguage(lang) {
        const normalized = String(lang || '').trim().toLowerCase();
        return normalized === 'chart' || normalized === 'chartjs';
    }

    function createMarkdownIt(options = {}) {
        if (typeof window.markdownit === 'undefined') return null;

        return window.markdownit({
            html: false,
            breaks: true,
            linkify: true,
            highlight(str, lang) {
                const language = String(lang || '').trim();

                if (language.toLowerCase() === 'mermaid') {
                    return '<div class="mermaid-raw">' + escapeHtml(str) + '</div>';
                }

                if (options.enableCharts && isChartLanguage(language)) {
                    return '<div class="chart-raw" data-chart-language="' + escapeHtml(language || 'chart') + '"><pre>' + escapeHtml(str) + '</pre></div>';
                }

                if (typeof options.codeBlockFactory === 'function') {
                    const html = options.codeBlockFactory(str, language);
                    if (html) return html;
                }

                if (language && window.hljs && window.hljs.getLanguage(language)) {
                    try {
                        return '<pre class="hljs"><code>' +
                            window.hljs.highlight(str, { language, ignoreIllegals: true }).value +
                            '</code></pre>';
                    } catch (_) { }
                }

                return '<pre class="hljs"><code>' + escapeHtml(str) + '</code></pre>';
            }
        });
    }

    window.AuraMarkdown = {
        createMarkdownIt,
        escapeHtml
    };
})();
