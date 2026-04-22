// AuraGo Chat — Message rendering & DOM utilities

function containsLeakedToolMarkup(text) {
    if (!text || typeof text !== 'string') return false;
    return [
        /<\/?tool_call[^>]*>/i,
        /<\/?minimax:tool_call[^>]*>/i,
        /(?:^|\n)\s*minimax:tool_call\s*(?:\n|$)/i,
        /<invoke\b[^>]*>/i,
        /<parameter\b[^>]*>/i,
        /^\[Tool Output\]/im,
        /^Tool Output:/im,
        /\[Suggested next step\]/i,
        /"(action|tool_call|tool_name)"\s*:/i
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
        .replace(/<done\s*\/?>/gi, '')
        .replace(/```(?:json)?\s*\{\s*"action"[\s\S]*?\}\s*```/gi, '')
        .replace(/^```(?:json)?\n\{[\s\S]*?\}\n```$/gim, '')
        .replace(/^\{"action"\s*:[^\n]*\}\s*$/gim, '')
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
                    const detailsHtml = `<details class="thinking-block"><summary>🧠 ${label}</summary><div class="thinking-content">${innerHtml}</div></details>`;
                    // Replace whether it is wrapped in paragraph or not
                    finalHTML = finalHTML.replace(new RegExp(`<p>%%THINKING_BLOCK_${idx}%%</p>`, 'g'), detailsHtml);
                    finalHTML = finalHTML.replace(new RegExp(`%%THINKING_BLOCK_${idx}%%`, 'g'), detailsHtml);
                });

                finalHTML = replaceRedactedMarkers(finalHTML);
                finalHTML = sanitizeRenderedHTML(finalHTML);

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
    const avatarIcon = isUser ? '🧑' : '🤖';
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
    const row = document.createElement('div');
    row.className = 'tool-output-row';
    row.innerHTML = `
                <div class="avatar bot">⚙️</div>
                <div class="tool-output-block">
                    <details>
                        <summary>⚙️ ${lbl}</summary>
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
    if (/^\{[\s\S]*"(action|tool_call|tool_name)"\s*:/i.test(text)) return true;
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

const emojiGlyphPattern = /(?:\p{Extended_Pictographic}(?:\uFE0F)?(?:\u200D\p{Extended_Pictographic}(?:\uFE0F)?)*)|[✓✔✕✖✗✘☑☒☐⚠⚡★☆]/gu;

function decorateEmojiGlyphs(root) {
    if (!root || typeof document === 'undefined' || typeof TreeWalker === 'undefined') return;
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

/** Returns an emoji icon for common document formats. */
function docFormatIcon(fmt) {
    switch ((fmt || '').toLowerCase()) {
        case 'pdf': return '📄';
        case 'docx': case 'doc': return '📝';
        case 'xlsx': case 'xls': return '📊';
        case 'pptx': case 'ppt': return '📑';
        case 'csv': return '📋';
        case 'md': return '📓';
        case 'txt': return '📃';
        case 'json': return '🔧';
        case 'xml': return '🗂️';
        case 'html': case 'htm': return '🌐';
        default: return '📎';
    }
}
