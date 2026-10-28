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

    function renderDiagramFence(str, language, options) {
        if (language.toLowerCase() === 'mermaid') {
            return '<div class="mermaid-raw">' + escapeHtml(str) + '</div>';
        }

        if (options.enableCharts && isChartLanguage(language)) {
            return '<div class="chart-raw" data-chart-language="' + escapeHtml(language || 'chart') + '"><pre>' + escapeHtml(str) + '</pre></div>';
        }

        return '';
    }

    // markdown-it wraps highlight output that does not start with <pre> in
    // <pre><code>. Block markup from a code block factory would then inherit
    // white-space: pre and render its own line breaks as empty bands, so these
    // fences are emitted verbatim instead.
    function renderFencesVerbatim(md, options) {
        const renderFence = md.renderer.rules.fence;
        md.renderer.rules.fence = function (tokens, idx, renderOptions, env, self) {
            const token = tokens[idx];
            const info = token.info ? md.utils.unescapeAll(token.info).trim() : '';
            const language = info ? info.split(/\s+/)[0] : '';
            const block = renderDiagramFence(token.content, language, options) ||
                String(options.codeBlockFactory(token.content, language) || '').trim();
            return block ? block + '\n' : renderFence(tokens, idx, renderOptions, env, self);
        };
    }

    function createMarkdownIt(options = {}) {
        if (typeof window.markdownit === 'undefined') return null;

        const md = window.markdownit({
            html: false,
            breaks: true,
            linkify: true,
            highlight(str, lang) {
                const language = String(lang || '').trim();

                const diagram = renderDiagramFence(str, language, options);
                if (diagram) return diagram;

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

        if (typeof options.codeBlockFactory === 'function') {
            renderFencesVerbatim(md, options);
        }
        return md;
    }

    window.AuraMarkdown = {
        createMarkdownIt,
        escapeHtml
    };
})();
