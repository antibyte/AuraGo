// AuraGo Chat — Message rendering & DOM utilities

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

function appendMessage(role, text) {
    if (!text || typeof text !== 'string') text = '';

    const greet = chatContent.querySelector('[data-greeting]');
    if (greet && window.ChatRobotMascot && typeof window.ChatRobotMascot.launchToAnchor === 'function') {
        window.ChatRobotMascot.launchToAnchor();
    }
    if (greet) greet.remove();

    const isUser = role === 'user';
    let isTechnical = false;

    let displayContent = text;
    if (!isUser) {
        // Pre-emptively strip to see if there's any conversational text left
        const strippedContent = stripLeakedToolMarkup(displayContent);

        if (!strippedContent && containsLeakedToolMarkup(text)) {
            // Entire message was just a tool output or tool call with no other text
            isTechnical = true;
        } else {
            // If there's conversational text, we show the stripped version
            displayContent = strippedContent;
        }
    }

    displayContent = displayContent.trim();
    if (!displayContent) return;

    // Remove markdown images already shown live via SSE 'image' event
    if (!isTechnical && seenSSEImages.size > 0) {
        displayContent = displayContent.replace(/!\[[^\]]*\]\(([^)]+)\)/g, (match, url) =>
            seenSSEImages.has(url) ? '' : match
        ).trim();
        if (!displayContent) return; // nothing left to show
    }

    let finalHTML = displayContent;
    if (isTechnical) {
        finalHTML = `<pre>${escapeHtml(displayContent)}</pre>`;
        finalHTML = replaceRedactedMarkers(finalHTML);
    } else {
        try {
            if (typeof window.markdownit !== 'undefined') {
                const md = window.markdownit({
                    html: false,
                    breaks: true,
                    linkify: true,
                    highlight: function (str, lang) {
                        // Handle mermaid diagrams first
                        if (lang === 'mermaid') {
                            return `<div class="mermaid-raw">${escapeHtml(str)}</div>`;
                        }
                        
                        // Use enhanced code blocks for other languages
                        if (window.CodeBlocks) {
                            return window.CodeBlocks.createCodeBlock(str, lang);
                        }
                        
                        // Fallback to basic highlighting
                        if (lang && window.hljs && hljs.getLanguage(lang)) {
                            try {
                                return '<pre class="hljs"><code>' +
                                    hljs.highlight(str, { language: lang, ignoreIllegals: true }).value +
                                    '</code></pre>';
                            } catch (__) { }
                        }
                        return '<pre class="hljs"><code>' + escapeHtml(str) + '</code></pre>';
                    }
                });

                // Strip <external_data> wrapper tags — keep their inner content.
                // These are security wrappers the LLM occasionally mixes into its own output.
                const contentStripped = displayContent.replace(
                    /<external_data>([\s\S]*?)<\/external_data>/gi,
                    (match, inner) => inner.trim()
                );

                // Extract <thinking>/<think> blocks and replace with block-level placeholders
                // so markdown-it doesn't wrap them in <p> tags.
                const thinkingBlocks = [];
                const contentForRender = contentStripped.replace(
                    /<(thinking|think)>([\s\S]*?)<\/\1>/gi,
                    (match, _tag, inner) => {
                        const idx = thinkingBlocks.length;
                        thinkingBlocks.push(inner.trim());
                        return `\n\n%%THINKING_BLOCK_${idx}%%\n\n`;
                    }
                );

                finalHTML = md.render(contentForRender);

                // Add target="_blank" to all links (external and internal)
                finalHTML = finalHTML.replace(/<a(\s+[^>]*)?\s+href="([^"]+)"/g, '<a$1href="$2" target="_blank" rel="noopener noreferrer"');

                // Replace placeholders with collapsible <details> elements
                thinkingBlocks.forEach((innerText, idx) => {
                    const innerHtml = md.render(innerText);
                    const label = (typeof t === 'function') ? t('chat.thinking_label') : 'Reasoning';
                    const icon = window.chatUiIconMarkup ? window.chatUiIconMarkup('mood-brain', 'thinking-block-icon') : '';
                    const detailsHtml = `<details class="thinking-block"><summary>${icon} ${label}</summary><div class="thinking-content">${innerHtml}</div></details>`;
                    // Replace whether it is wrapped in paragraph or not
                    finalHTML = finalHTML.replace(new RegExp(`<p>%%THINKING_BLOCK_${idx}%%</p>`, 'g'), detailsHtml);
                    finalHTML = finalHTML.replace(new RegExp(`%%THINKING_BLOCK_${idx}%%`, 'g'), detailsHtml);
                });

                finalHTML = replaceRedactedMarkers(finalHTML);
                finalHTML = sanitizeRenderedHTML(finalHTML);
                finalHTML = renderVideoLinksAsPlayers(finalHTML);
                finalHTML = renderYouTubeLinksAsPlayers(finalHTML);

                // If the rendered content contains only thinking blocks with no visible
                // text, add a subtle "done" indicator so the bubble is never blank.
                if (thinkingBlocks.length > 0) {
                    const visibleText = contentForRender
                        .replace(/%%THINKING_BLOCK_\d+%%/g, '')
                        .trim();
                    if (!visibleText) {
                        const doneLabel = (typeof t === 'function') ? t('chat.thinking_only_done') : 'Done.';
                        finalHTML += `<p class="thinking-done-hint">${escapeHtml(doneLabel)}</p>`;
                    }
                }
            }
        } catch (e) {
            console.error("Markdown parsing failed:", e);
        }
    }

    const side = isUser ? 'user' : 'bot';
    const avatarIcon = window.chatUiIconMarkup ? window.chatUiIconMarkup(isUser ? 'user' : 'bot') : '';
    const bubbleClass = isTechnical ? 'bubble bot technical' : `bubble ${side}`;

    const msgHTML = `
                <div class="msg-row ${side}">
                    <div class="avatar ${isUser ? 'human' : 'bot'}">${avatarIcon}</div>
                    <div class="${bubbleClass}">${finalHTML}</div>
                </div>
            `;
    chatContent.insertAdjacentHTML('beforeend', msgHTML);
    const newMessage = chatContent.lastElementChild;
    const renderedBubble = newMessage && newMessage.querySelector('.bubble');
    if (renderedBubble) {
        decorateEmojiGlyphs(renderedBubble);
    }
    
    // Render mermaid diagrams if available
    if (window.MermaidLoader) {
        if (newMessage) {
            window.MermaidLoader.processBlocks(newMessage);
        }
    }
    
    // Use SmartScroller or fallback
    if (window.SmartScroller) {
        window.SmartScroller.onNewMessage();
    } else {
        chatBox.scrollTop = chatBox.scrollHeight;
    }
}

function appendToolOutput(text, label) {
    if (!text || !debugMode) return;
    const greet = chatContent.querySelector('[data-greeting]');
    if (greet) greet.remove();
    const escaped = escapeHtml(text);
    const lbl = label || t('chat.tool_output_label');
    const settingsIcon = window.chatUiIconMarkup ? window.chatUiIconMarkup('settings') : '';
    const row = document.createElement('div');
    row.className = 'tool-output-row';
    row.innerHTML = `
                <div class="avatar bot">${settingsIcon}</div>
                <div class="tool-output-block">
                    <details>
                        <summary>${settingsIcon} ${lbl}</summary>
                        <div class="tool-output-content">${escaped}</div>
                    </details>
                </div>
            `;
    chatContent.appendChild(row);
    chatBox.scrollTop = chatBox.scrollHeight;
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
    // Keep this conservative and only hide short operational updates, not normal answers.
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

function escapeHtml(str) {
    return String(str)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;');
}

function replaceRedactedMarkers(html) {
    const label = (typeof t === 'function') ? t('chat.redacted_label') : '[removed]';
    return html
        .replace(/\[redacted\]([^<]*)/gi, (match, reason) => {
            const reasonText = reason.trim();
            if (reasonText) {
                return `<span class="redacted-badge" title="${escapeAttr(reasonText)}">${label}</span> <span class="redacted-reason">${escapeHtml(reasonText)}</span>`;
            }
            return `<span class="redacted-badge">${label}</span>`;
        })
        .replace(/\[sanitized\]([^<]*)/gi, (match, reason) => {
            const reasonText = reason.trim();
            if (reasonText) {
                return `<span class="sanitized-badge" title="${escapeAttr(reasonText)}">${label}</span> <span class="redacted-reason">${escapeHtml(reasonText)}</span>`;
            }
            return `<span class="sanitized-badge">${label}</span>`;
        });
}

function escapeAttr(s) {
    return String(s)
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

function videoMimeTypeForPath(path) {
    const lower = String(path || '').split('?')[0].toLowerCase();
    if (lower.endsWith('.webm')) return 'video/webm';
    if (lower.endsWith('.ogv') || lower.endsWith('.ogg')) return 'video/ogg';
    if (lower.endsWith('.mov')) return 'video/quicktime';
    return 'video/mp4';
}

function filenameFromPath(path) {
    try {
        const parsed = new URL(path, window.location.origin);
        const name = decodeURIComponent((parsed.pathname.split('/').pop() || '').trim());
        return name || '';
    } catch (_err) {
        const clean = String(path || '').split('?')[0];
        return clean.split('/').pop() || '';
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

function createChatVideoElement(videoData) {
    const path = videoData && videoData.path ? String(videoData.path) : '';
    const wrapper = document.createElement('div');
    wrapper.className = 'chat-video-wrapper';

    const title = String((videoData && (videoData.title || videoData.filename)) || filenameFromPath(path) || '').trim();
    if (title) {
        const titleEl = document.createElement('div');
        titleEl.className = 'chat-video-title';
        titleEl.textContent = title;
        wrapper.appendChild(titleEl);
    }

    const video = document.createElement('video');
    video.className = 'chat-video-player';
    video.controls = true;
    video.preload = 'metadata';
    video.playsInline = true;

    const source = document.createElement('source');
    source.src = path;
    source.type = String((videoData && videoData.mime_type) || videoMimeTypeForPath(path));
    video.appendChild(source);
    wrapper.appendChild(video);

    const actions = document.createElement('div');
    actions.className = 'chat-video-actions';
    const link = document.createElement('a');
    link.href = path;
    link.download = filenameFromPath(path) || 'video.mp4';
    link.title = 'Download';
    link.textContent = 'Download';
    actions.appendChild(link);
    wrapper.appendChild(actions);

    return wrapper;
}

function createChatYouTubeElement(youtubeData) {
    const parsed = parseYouTubeVideoLink((youtubeData && (youtubeData.url || youtubeData.embed_url || youtubeData.path)) || '');
    const videoID = (youtubeData && youtubeData.video_id && /^[A-Za-z0-9_-]{11}$/.test(youtubeData.video_id))
        ? youtubeData.video_id
        : (parsed && parsed.video_id);
    if (!videoID) return null;
    const rawStartSeconds = Number((youtubeData && youtubeData.start_seconds) || (parsed && parsed.start_seconds) || 0);
    const startSeconds = Number.isFinite(rawStartSeconds) && rawStartSeconds > 0 ? Math.floor(rawStartSeconds) : 0;
    const url = `https://www.youtube.com/watch?v=${videoID}${startSeconds > 0 ? `&t=${startSeconds}s` : ''}`;
    const fallbackEmbedURL = `https://www.youtube-nocookie.com/embed/${videoID}${startSeconds > 0 ? `?start=${startSeconds}` : ''}`;
    const embedURL = safeYouTubeEmbedURL(youtubeData && youtubeData.embed_url, videoID, startSeconds) || fallbackEmbedURL;

    const wrapper = document.createElement('div');
    wrapper.className = 'chat-youtube-wrapper';
    wrapper.dataset.youtubeId = videoID;

    const title = String((youtubeData && youtubeData.title) || 'YouTube').trim();
    if (title) {
        const titleEl = document.createElement('div');
        titleEl.className = 'chat-video-title';
        titleEl.textContent = title;
        wrapper.appendChild(titleEl);
    }

    const iframe = document.createElement('iframe');
    iframe.className = 'chat-youtube-player';
    iframe.src = embedURL;
    iframe.title = title || 'YouTube';
    iframe.loading = 'lazy';
    iframe.allow = 'accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share';
    iframe.allowFullscreen = true;
    wrapper.appendChild(iframe);

    const actions = document.createElement('div');
    actions.className = 'chat-video-actions';
    const link = document.createElement('a');
    link.href = url;
    link.target = '_blank';
    link.rel = 'noopener noreferrer';
    link.textContent = 'YouTube';
    actions.appendChild(link);
    wrapper.appendChild(actions);

    return wrapper;
}

function appendVideoMessage(videoData) {
    if (!videoData || !videoData.path || !isVideoHref(videoData.path)) return false;
    const greet = chatContent.querySelector('[data-greeting]');
    if (greet) greet.remove();

    const row = document.createElement('div');
    row.className = 'msg-row bot';
    const botIcon = window.chatUiIconMarkup ? window.chatUiIconMarkup('bot') : '';
    row.innerHTML = `<div class="avatar bot">${botIcon}</div><div class="bubble bot"></div>`;
    row.querySelector('.bubble').appendChild(createChatVideoElement(videoData));
    chatContent.appendChild(row);
    chatBox.scrollTop = chatBox.scrollHeight;
    return true;
}

function appendYouTubeMessage(youtubeData) {
    const element = createChatYouTubeElement(youtubeData);
    if (!element) return false;
    const greet = chatContent.querySelector('[data-greeting]');
    if (greet) greet.remove();

    const row = document.createElement('div');
    row.className = 'msg-row bot';
    const botIcon = window.chatUiIconMarkup ? window.chatUiIconMarkup('bot') : '';
    row.innerHTML = `<div class="avatar bot">${botIcon}</div><div class="bubble bot"></div>`;
    row.querySelector('.bubble').appendChild(element);
    chatContent.appendChild(row);
    chatBox.scrollTop = chatBox.scrollHeight;
    return true;
}

function renderVideoLinksAsPlayers(html) {
    const template = document.createElement('template');
    template.innerHTML = html;
    template.content.querySelectorAll('a[href]').forEach((anchor) => {
        const href = anchor.getAttribute('href') || '';
        if (!isVideoHref(href)) return;
        if (typeof seenSSEVideos !== 'undefined' && seenSSEVideos.has(href)) {
            anchor.remove();
            return;
        }
        anchor.replaceWith(createChatVideoElement({
            path: href,
            title: anchor.textContent || filenameFromPath(href),
            mime_type: videoMimeTypeForPath(href)
        }));
    });
    const walker = document.createTreeWalker(template.content, NodeFilter.SHOW_TEXT);
    const textNodes = [];
    let node;
    while ((node = walker.nextNode())) {
        const parent = node.parentElement;
        if (!parent || parent.closest('a, code, pre, .chat-video-wrapper')) continue;
        if (/\/files\/[^\s<>()"']+\.(?:mp4|m4v|mov|webm|ogv|ogg)/i.test(node.nodeValue || '')) {
            textNodes.push(node);
        }
    }
    textNodes.forEach((textNode) => {
        const text = textNode.nodeValue || '';
        const fragment = document.createDocumentFragment();
        const pattern = /\/files\/[^\s<>()"']+\.(?:mp4|m4v|mov|webm|ogv|ogg)/ig;
        let lastIndex = 0;
        let match;
        while ((match = pattern.exec(text)) !== null) {
            if (match.index > lastIndex) {
                fragment.appendChild(document.createTextNode(text.slice(lastIndex, match.index)));
            }
            const path = match[0];
            if (!(typeof seenSSEVideos !== 'undefined' && seenSSEVideos.has(path)) && isVideoHref(path)) {
                fragment.appendChild(createChatVideoElement({
                    path,
                    title: filenameFromPath(path),
                    mime_type: videoMimeTypeForPath(path)
                }));
            }
            lastIndex = match.index + path.length;
        }
        if (lastIndex < text.length) {
            fragment.appendChild(document.createTextNode(text.slice(lastIndex)));
        }
        textNode.parentNode.replaceChild(fragment, textNode);
    });
    return template.innerHTML;
}

function renderYouTubeLinksAsPlayers(html) {
    const template = document.createElement('template');
    template.innerHTML = html;
    template.content.querySelectorAll('a[href]').forEach((anchor) => {
        const href = anchor.getAttribute('href') || '';
        const parsed = parseYouTubeVideoLink(href);
        if (!parsed) return;
        const key = youtubePlayerDedupKey(parsed);
        if (typeof seenSSEYouTubeVideos !== 'undefined' && seenSSEYouTubeVideos.has(key)) {
            anchor.remove();
            return;
        }
        const element = createChatYouTubeElement({
            ...parsed,
            title: anchor.textContent || 'YouTube'
        });
        if (element) anchor.replaceWith(element);
    });
    return template.innerHTML;
}

const emojiGlyphPattern = /(?:\p{Extended_Pictographic}(?:\uFE0F)?(?:\u200D\p{Extended_Pictographic}(?:\uFE0F)?)*)|[✓✔✕✖✗✘☑☒☐⚠⚡★☆]/gu;

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

window.decorateEmojiGlyphs = decorateEmojiGlyphs;

/** Returns image icon markup for common document formats. */
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
        default: return markup('attach');
    }
}
