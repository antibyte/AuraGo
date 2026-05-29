(function () {
    'use strict';

    const emojiGlyphPattern = /(?:\p{Extended_Pictographic}(?:\uFE0F)?(?:\u200D\p{Extended_Pictographic}(?:\uFE0F)?)*)|[✓✔✕✖✗✘☑☒☐⚠⚡★☆]/gu;

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

    function isSafeHref(url, allowRelative = true) {
        if (!url || typeof url !== 'string') return false;
        const trimmed = url.trim();
        if (!trimmed) return false;
        if (allowRelative && (trimmed.startsWith('/') || trimmed.startsWith('./') || trimmed.startsWith('../'))) {
            return true;
        }
        try {
            const parsed = new URL(trimmed, window.location.origin);
            return parsed.protocol === 'http:' || parsed.protocol === 'https:';
        } catch (_err) {
            return false;
        }
    }

    function sanitizeRenderedHTML(html) {
        const template = document.createElement('template');
        template.innerHTML = html;
        template.content.querySelectorAll('*').forEach((node) => {
            Array.from(node.attributes).forEach((attr) => {
                const name = attr.name.toLowerCase();
                if (name.startsWith('on')) {
                    node.removeAttribute(attr.name);
                    return;
                }
                if ((name === 'href' || name === 'src') && !isSafeHref(attr.value, true)) {
                    node.removeAttribute(attr.name);
                }
            });
        });
        return template.innerHTML;
    }

    function decorateEmojiGlyphs(root) {
        if (!root || typeof document === 'undefined' || typeof document.createTreeWalker !== 'function') return;
        const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
        const textNodes = [];
        let node;
        while ((node = walker.nextNode())) {
            const parent = node.parentElement;
            if (!parent) continue;
            if (parent.closest('code, pre, .hljs, .mermaid-raw, .tool-output-content')) continue;
            emojiGlyphPattern.lastIndex = 0;
            if (!emojiGlyphPattern.test(node.nodeValue || '')) continue;
            textNodes.push(node);
        }

        textNodes.forEach((textNode) => {
            const text = textNode.nodeValue || '';
            emojiGlyphPattern.lastIndex = 0;
            if (!emojiGlyphPattern.test(text)) return;
            emojiGlyphPattern.lastIndex = 0;

            const fragment = document.createDocumentFragment();
            let lastIndex = 0;
            let match;
            while ((match = emojiGlyphPattern.exec(text)) !== null) {
                if (match.index > lastIndex) {
                    fragment.appendChild(document.createTextNode(text.slice(lastIndex, match.index)));
                }
                const glyph = document.createElement('span');
                glyph.className = 'chat-emoji-glyph';
                glyph.textContent = match[0];
                fragment.appendChild(glyph);
                lastIndex = match.index + match[0].length;
            }
            if (lastIndex < text.length) {
                fragment.appendChild(document.createTextNode(text.slice(lastIndex)));
            }
            textNode.parentNode.replaceChild(fragment, textNode);
        });
    }

    function isVideoHref(url) {
        if (!url || typeof url !== 'string') return false;
        const trimmed = url.trim();
        if (!trimmed || !isSafeHref(trimmed, true)) return false;
        try {
            const parsed = new URL(trimmed, window.location.origin);
            if (parsed.origin !== window.location.origin) return false;
            const path = parsed.pathname.toLowerCase();
            return path.startsWith('/files/generated_videos/') ||
                /\.(mp4|m4v|mov|webm|ogv|ogg)$/i.test(path);
        } catch (_err) {
            return false;
        }
    }

    function filenameFromPath(path, fallback = '') {
        const fallbackName = String(fallback || '');
        try {
            const parsed = new URL(String(path || ''), window.location.origin);
            const name = decodeURIComponent((parsed.pathname.split('/').pop() || '').trim());
            return name || fallbackName;
        } catch (_err) {
            const clean = String(path || '').split('?')[0];
            return clean.split('/').pop() || fallbackName;
        }
    }

    function videoMimeTypeForPath(path) {
        const lower = String(path || '').split('?')[0].toLowerCase();
        if (lower.endsWith('.webm')) return 'video/webm';
        if (lower.endsWith('.ogv') || lower.endsWith('.ogg')) return 'video/ogg';
        if (lower.endsWith('.mov')) return 'video/quicktime';
        return 'video/mp4';
    }

    function docFormatIcon(fmt) {
        const markup = window.chatUiIconMarkup || (() => '');
        switch ((fmt || '').toLowerCase()) {
            case 'pdf': return markup('pdf');
            case 'docx': case 'doc': return markup('edit-document');
            case 'xlsx': case 'xls': return markup('spreadsheet');
            case 'pptx': case 'ppt': return markup('presentation');
            case 'csv': return markup('csv');
            case 'md': return markup('markdown');
            case 'txt': return markup('text-file');
            case 'json': return markup('json');
            case 'xml': return markup('xml');
            case 'html': case 'htm': return markup('web');
            case 'stl': return markup('theme-threedee') || markup('attach');
            default: return markup('attach');
        }
    }

    function parseYouTubeTimeValue(raw) {
        const value = String(raw || '').trim().toLowerCase();
        if (!value) return 0;
        if (/^\d+s?$/.test(value)) return parseInt(value, 10) || 0;
        if (value.includes(':')) {
            return value.split(':').reduce((total, part) => {
                const n = parseInt(part, 10);
                return Number.isFinite(n) ? (total * 60) + n : 0;
            }, 0);
        }
        const match = value.match(/^(?:(\d+)h)?(?:(\d+)m)?(?:(\d+)s)?$/);
        if (!match) return 0;
        return ((parseInt(match[1] || '0', 10) || 0) * 3600)
            + ((parseInt(match[2] || '0', 10) || 0) * 60)
            + (parseInt(match[3] || '0', 10) || 0);
    }

    function parseYouTubeVideoLink(raw) {
        if (!raw || typeof raw !== 'string') return null;
        let href = raw.trim();
        if (!href) return null;
        if (!href.includes('://') && (href.includes('youtube.') || href.includes('youtu.be'))) {
            href = `https://${href}`;
        }
        try {
            const parsed = new URL(href, window.location.origin);
            const host = parsed.hostname.toLowerCase().replace(/^www\./, '');
            const segments = parsed.pathname.split('/').filter(Boolean).map((part) => {
                try { return decodeURIComponent(part); } catch (_err) { return part; }
            });
            let videoID = '';
            if (host === 'youtu.be') {
                videoID = segments[0] || '';
            } else if (host === 'youtube.com' || host === 'm.youtube.com' || host === 'music.youtube.com') {
                if (segments.length === 0 || segments[0] === 'watch') {
                    videoID = parsed.searchParams.get('v') || '';
                } else if (['shorts', 'embed', 'live'].includes(segments[0])) {
                    videoID = segments[1] || '';
                }
            } else if (host === 'youtube-nocookie.com' && segments[0] === 'embed') {
                videoID = segments[1] || '';
            }
            if (!/^[A-Za-z0-9_-]{11}$/.test(videoID)) return null;
            const startSeconds = parseYouTubeTimeValue(parsed.searchParams.get('start')) || parseYouTubeTimeValue(parsed.searchParams.get('t'));
            const canonicalURL = `https://www.youtube.com/watch?v=${videoID}${startSeconds > 0 ? `&t=${startSeconds}s` : ''}`;
            const embedURL = `https://www.youtube-nocookie.com/embed/${videoID}${startSeconds > 0 ? `?start=${startSeconds}` : ''}`;
            return { video_id: videoID, url: canonicalURL, embed_url: embedURL, start_seconds: startSeconds };
        } catch (_err) {
            return null;
        }
    }

    function youtubePlayerDedupKey(data) {
        const id = data && data.video_id ? String(data.video_id) : '';
        const rawStart = Number((data && data.start_seconds) || 0);
        const start = Number.isFinite(rawStart) && rawStart > 0 ? Math.floor(rawStart) : 0;
        const url = data && (data.url || data.embed_url || data.path) ? String(data.url || data.embed_url || data.path) : '';
        return id ? `${id}:${start}` : `${url}:${start}`;
    }

    function safeYouTubeEmbedURL(raw, expectedVideoID, expectedStartSeconds) {
        if (!raw || !expectedVideoID) return '';
        try {
            const parsed = new URL(String(raw), window.location.origin);
            const host = parsed.hostname.toLowerCase().replace(/^www\./, '');
            const parts = parsed.pathname.split('/').filter(Boolean).map((part) => {
                try { return decodeURIComponent(part); } catch (_err) { return part; }
            });
            if (host !== 'youtube-nocookie.com' || parts[0] !== 'embed' || parts[1] !== expectedVideoID) return '';
            const start = parseYouTubeTimeValue(parsed.searchParams.get('start'));
            if (start !== (Number(expectedStartSeconds) || 0)) return '';
            return `https://www.youtube-nocookie.com/embed/${expectedVideoID}${start > 0 ? `?start=${start}` : ''}`;
        } catch (_err) {
            return '';
        }
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

    function replaceRedactedMarkers(html, label = '[removed]') {
        const displayLabel = String(label || '[removed]');
        return String(html || '')
            .replace(/\[redacted\]([^<]*)/gi, (_match, reason) => {
                const reasonText = reason.trim();
                if (reasonText) {
                    return `<span class="redacted-badge" title="${escapeAttr(reasonText)}">${displayLabel}</span> <span class="redacted-reason">${escapeHtml(reasonText)}</span>`;
                }
                return `<span class="redacted-badge">${displayLabel}</span>`;
            })
            .replace(/\[sanitized\]([^<]*)/gi, (_match, reason) => {
                const reasonText = reason.trim();
                if (reasonText) {
                    return `<span class="sanitized-badge" title="${escapeAttr(reasonText)}">${displayLabel}</span> <span class="redacted-reason">${escapeHtml(reasonText)}</span>`;
                }
                return `<span class="sanitized-badge">${displayLabel}</span>`;
            });
    }

    function isDebugOnlyHistoryMessage(msg) {
        if (!msg || typeof msg.content !== 'string') return false;
        const text = msg.content.trim();
        if (!text) return false;

        if (msg.role === 'user') {
            if (/^ERROR:\s+/i.test(text)) return true;
            if (/invalid function arguments json|raw JSON object ONLY|markdown fences|tool call/i.test(text)) return true;
            return false;
        }

        if (msg.role !== 'assistant' && msg.role !== 'system') return false;
        if (text === '[TOOL_CALL]') return true;
        if (/^\[TOOL_CALL\]/i.test(text)) return true;
        if (containsLeakedToolMarkup(text)) return true;
        if (/^\{[\s\S]*"(action|tool|tool_call|tool_name)"\s*:/i.test(text)) return true;
        if (/^(Tool Output:|\[Tool Output\])/i.test(text)) return true;

        // Legacy leaked orchestration/progress messages from pre-tool assistant turns.
        const lower = text.toLowerCase();
        const operationalHints = [
            'container', 'build', 'deploy', 'install', 'npm ', 'docker', 'script ',
            'command', 'logs', 'warte', 'wait', 'läuft', 'running', 'fertig',
            'copied', 'kopiert', 'ansatz', 'approach'
        ];
        if (text.length <= 240 && /[:：]\s*$/.test(text) && operationalHints.some(h => lower.includes(h))) {
            return true;
        }

        return false;
    }

    function prepareDisplayContent(text, isUser) {
        const raw = String(text || '');
        if (isUser) {
            return { displayContent: raw.trim(), isTechnical: false };
        }

        const strippedContent = stripLeakedToolMarkup(raw);
        if (!strippedContent && containsLeakedToolMarkup(raw)) {
            return { displayContent: raw.trim(), isTechnical: true };
        }
        return { displayContent: strippedContent.trim(), isTechnical: false };
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

    function removeSeenMarkdownImages(text, seenImages) {
        if (!seenImages || typeof seenImages.has !== 'function' || !seenImages.size) {
            return String(text || '');
        }
        return String(text || '').replace(/!\[[^\]]*\]\(([^)]+)\)/g, (match, url) =>
            seenImages.has(url) ? '' : match
        ).trim();
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
        isSafeHref,
        sanitizeRenderedHTML,
        isVideoHref,
        decorateEmojiGlyphs,
        filenameFromPath,
        videoMimeTypeForPath,
        docFormatIcon,
        parseYouTubeTimeValue,
        parseYouTubeVideoLink,
        youtubePlayerDedupKey,
        safeYouTubeEmbedURL,
        containsLeakedToolMarkup,
        stripLeakedToolMarkup,
        replaceRedactedMarkers,
        isDebugOnlyHistoryMessage,
        prepareDisplayContent,
        prepareMarkdownContent,
        applyMarkdownLinkTargets,
        replaceThinkingPlaceholders,
        removeSeenMarkdownImages,
        normalizeTimestamp,
        formatTimestamp,
        createMarkdownRenderer
    };
})();
