(function () {
    'use strict';

    const DesktopChatRenderer = {
        seenSSEImages: new Set(),
        seenSSEVideos: new Set(),
        seenSSELiveStreams: new Set(),
        seenSSEAudios: new Set(),
        seenSSEDocuments: new Set(),
        _md: null,
        _lightbox: null,
        _audioQueue: [],
        _audioPlaying: false,
        _currentAudio: null,

        escapeHtml(str) {
            if (window.AuraChatCore && typeof window.AuraChatCore.escapeHtml === 'function') {
                return window.AuraChatCore.escapeHtml(str);
            }
            return String(str)
                .replace(/&/g, '&amp;')
                .replace(/</g, '&lt;')
                .replace(/>/g, '&gt;')
                .replace(/"/g, '&quot;');
        },

        escapeAttr(s) {
            if (window.AuraChatCore && typeof window.AuraChatCore.escapeAttr === 'function') {
                return window.AuraChatCore.escapeAttr(s);
            }
            return String(s)
                .replace(/&/g, '&amp;')
                .replace(/"/g, '&quot;')
                .replace(/'/g, '&#39;')
                .replace(/</g, '&lt;')
                .replace(/>/g, '&gt;');
        },

        translate(key, fallback) {
            const translator = window.t || (() => '');
            const translated = translator(key);
            return translated && translated !== key ? translated : fallback;
        },

        normalizeChatTimestamp(timestamp) {
            if (window.AuraChatCore && typeof window.AuraChatCore.normalizeTimestamp === 'function') {
                return window.AuraChatCore.normalizeTimestamp(timestamp);
            }
            const date = timestamp ? new Date(timestamp) : new Date();
            return Number.isNaN(date.getTime()) ? new Date() : date;
        },

        formatChatTimestamp(timestamp) {
            if (window.AuraChatCore && typeof window.AuraChatCore.formatTimestamp === 'function') {
                return window.AuraChatCore.formatTimestamp(timestamp);
            }
            const date = this.normalizeChatTimestamp(timestamp);
            try {
                return new Intl.DateTimeFormat(undefined, {
                    dateStyle: 'short',
                    timeStyle: 'short'
                }).format(date);
            } catch (_) {
                return date.toLocaleString();
            }
        },

        appendTimestamp(chatLog, role, timestamp) {
            if (!chatLog) return null;
            const previous = chatLog.lastElementChild;
            if (previous && previous.classList && previous.classList.contains('vd-chat-entry-time')) {
                return previous;
            }
            const date = this.normalizeChatTimestamp(timestamp);
            const el = document.createElement('div');
            el.className = 'vd-chat-entry-time ' + (role === 'user' ? 'user' : 'agent');
            el.setAttribute('data-chat-timestamp', date.toISOString());
            el.textContent = this.formatChatTimestamp(date);
            chatLog.appendChild(el);
            return el;
        },

        getMarkdown() {
            if (this._md) return this._md;
            if (window.AuraChatCore && typeof window.AuraChatCore.createMarkdownRenderer === 'function') {
                this._md = window.AuraChatCore.createMarkdownRenderer();
                return this._md;
            }
            if (window.AuraMarkdown) {
                this._md = window.AuraMarkdown.createMarkdownIt();
                return this._md;
            }
            if (typeof window.markdownit === 'undefined') return null;
            this._md = window.markdownit({ html: false, breaks: true, linkify: true });
            return this._md;
        },

        stripLeakedToolMarkup(text) {
            if (window.AuraChatCore && typeof window.AuraChatCore.stripLeakedToolMarkup === 'function') {
                return window.AuraChatCore.stripLeakedToolMarkup(text);
            }
            if (!text || typeof text !== 'string') return '';
            return text
                .replace(/<tool_call[\s\S]*?<\/tool_call>/gi, '')
                .replace(/<\/?tool_call[^>]*>/gi, '')
                .replace(/<invoke\b[^>]*>[\s\S]*?<\/invoke>/gi, '')
                .replace(/<parameter\b[^>]*>[\s\S]*?<\/parameter>/gi, '')
                .replace(/<done\s*\/?>/gi, '')
                .replace(/```(?:json)?\s*\{\s*"(?:action|tool|tool_call|tool_name)"[\s\S]*?\}\s*```/gi, '')
                .replace(/\n{3,}/g, '\n\n')
                .trim();
        },

        containsLeakedToolMarkup(text) {
            if (window.AuraChatCore && typeof window.AuraChatCore.containsLeakedToolMarkup === 'function') {
                return window.AuraChatCore.containsLeakedToolMarkup(text);
            }
            if (!text || typeof text !== 'string') return false;
            return [
                /<\/?tool_call[^>]*>/i,
                /<invoke\b[^>]*>/i,
                /<parameter\b[^>]*>/i,
                /^\[Tool Output\]/im
            ].some(p => p.test(text));
        },

        sanitizeHTML(html) {
            const template = document.createElement('template');
            template.innerHTML = html;
            const all = template.content.querySelectorAll('*');
            const allowed = new Set([
                'a', 'b', 'br', 'code', 'details', 'div', 'em', 'h1', 'h2', 'h3',
                'h4', 'h5', 'h6', 'hr', 'i', 'img', 'li', 'mark', 'ol', 'p',
                'pre', 's', 'span', 'strong', 'sub', 'summary', 'sup', 'table',
                'tbody', 'td', 'th', 'thead', 'tr', 'u', 'ul', 'blockquote',
                'del', 'ins', 'kbd', 'abbr', 'cite', 'dl', 'dt', 'dd', 'figure',
                'figcaption', 'picture', 'source', 'video', 'audio', 'track',
                'iframe', 'ruby', 'rt', 'rp', 'bdi', 'bdo', 'wbr', 'time',
                'small', 'var', 'samp', 'dfn', 'q', 'address', 'footer',
                'header', 'main', 'section', 'article', 'aside', 'nav'
            ]);
            const allowedAttrs = new Set([
                'href', 'src', 'alt', 'title', 'class', 'id', 'target', 'rel',
                'loading', 'decoding', 'width', 'height', 'colspan', 'rowspan',
                'data-language', 'data-line', 'start', 'type', 'download',
                'open', 'name', 'value', 'disabled', 'data-persona-icon'
            ]);
            for (let i = all.length - 1; i >= 0; i--) {
                const el = all[i];
                if (!allowed.has(el.tagName.toLowerCase())) {
                    while (el.firstChild) el.parentNode.insertBefore(el.firstChild, el);
                    el.parentNode.removeChild(el);
                    continue;
                }
                const attrs = Array.from(el.attributes);
                for (const attr of attrs) {
                    const name = attr.name.toLowerCase();
                    if (name.startsWith('data-')) continue;
                    if (!allowedAttrs.has(name)) {
                        el.removeAttribute(attr.name);
                    }
                }
                if (el.tagName.toLowerCase() === 'a') {
                    const href = el.getAttribute('href') || '';
                    if (href && !href.startsWith('/') && !href.startsWith('./')) {
                        try {
                            const parsed = new URL(href, window.location.origin);
                            if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
                                el.removeAttribute('href');
                            }
                        } catch (_) {
                            el.removeAttribute('href');
                        }
                    }
                    el.setAttribute('target', '_blank');
                    el.setAttribute('rel', 'noopener noreferrer');
                }
                if (el.tagName.toLowerCase() === 'img') {
                    el.setAttribute('loading', 'lazy');
                }
                if (el.tagName.toLowerCase() === 'iframe') {
                    const src = el.getAttribute('src') || '';
                    if (src) {
                        try {
                            const parsed = new URL(src, window.location.origin);
                            if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
                                el.removeAttribute('src');
                            }
                        } catch (_) {
                            el.removeAttribute('src');
                        }
                    }
                    if (!el.getAttribute('sandbox')) {
                        el.setAttribute('sandbox', 'allow-scripts allow-same-origin');
                    }
                }
                if (el.tagName.toLowerCase() === 'video' || el.tagName.toLowerCase() === 'audio') {
                    const src = el.getAttribute('src') || '';
                    if (src) {
                        try {
                            const parsed = new URL(src, window.location.origin);
                            if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:' && !parsed.protocol.startsWith('blob:')) {
                                el.removeAttribute('src');
                            }
                        } catch (_) {
                            el.removeAttribute('src');
                        }
                    }
                }
            }
            return template.innerHTML;
        },

        renderMarkdown(text) {
            const md = this.getMarkdown();
            if (!md) return this.escapeHtml(text);
            let displayContent = text;
            const strippedContent = this.stripLeakedToolMarkup(displayContent);
            const isTechnical = !strippedContent && this.containsLeakedToolMarkup(text);
            if (isTechnical) {
                return '<pre>' + this.escapeHtml(displayContent) + '</pre>';
            }
            displayContent = strippedContent.trim();
            if (!displayContent) return '';
            if (this.seenSSEImages.size > 0) {
                displayContent = (window.AuraChatCore && typeof window.AuraChatCore.removeSeenMarkdownImages === 'function')
                    ? window.AuraChatCore.removeSeenMarkdownImages(displayContent, this.seenSSEImages)
                    : displayContent.replace(/!\[[^\]]*\]\(([^)]+)\)/g, (match, url) =>
                        this.seenSSEImages.has(url) ? '' : match
                    ).trim();
                if (!displayContent) return '';
            }
            const prepared = (window.AuraChatCore && typeof window.AuraChatCore.prepareMarkdownContent === 'function')
                ? window.AuraChatCore.prepareMarkdownContent(displayContent)
                : (function () {
                    const contentStripped = displayContent.replace(
                        /<external_data>([\s\S]*?)<\/external_data>/gi,
                        (_match, inner) => inner.trim()
                    );
                    const thinkingBlocks = [];
                    const contentForRender = contentStripped.replace(
                        /<(thinking|think)>([\s\S]*?)<\/\1>/gi,
                        (_match, _tag, inner) => {
                            const idx = thinkingBlocks.length;
                            thinkingBlocks.push(inner.trim());
                            return '\n\n%%THINKING_BLOCK_' + idx + '%%\n\n';
                        }
                    );
                    return { contentForRender, thinkingBlocks };
                })();
            const contentForRender = prepared.contentForRender;
            const thinkingBlocks = prepared.thinkingBlocks;
            let finalHTML = md.render(contentForRender);
            finalHTML = (window.AuraChatCore && typeof window.AuraChatCore.applyMarkdownLinkTargets === 'function')
                ? window.AuraChatCore.applyMarkdownLinkTargets(finalHTML)
                : finalHTML.replace(/<a(\s+[^>]*)?\s+href="([^"]+)"/g,
                    '<a$1href="$2" target="_blank" rel="noopener noreferrer"');
            const renderThinkingBlock = (innerText) => {
                const innerHtml = md.render(innerText);
                const label = this.translate('chat.thinking_label', 'Reasoning');
                return '<details class="vd-thinking-block"><summary>' + label + '</summary><div class="vd-thinking-content">' + innerHtml + '</div></details>';
            };
            finalHTML = (window.AuraChatCore && typeof window.AuraChatCore.replaceThinkingPlaceholders === 'function')
                ? window.AuraChatCore.replaceThinkingPlaceholders(finalHTML, thinkingBlocks, renderThinkingBlock)
                : thinkingBlocks.reduce((html, innerText, idx) => {
                    const detailsHtml = renderThinkingBlock(innerText, idx);
                    html = html.replace(new RegExp('<p>%%THINKING_BLOCK_' + idx + '%%</p>', 'g'), detailsHtml);
                    return html.replace(new RegExp('%%THINKING_BLOCK_' + idx + '%%', 'g'), detailsHtml);
                }, finalHTML);
            finalHTML = this.sanitizeHTML(finalHTML);
            return finalHTML;
        },

        createBubble(role, innerHTML) {
            const bubble = document.createElement('div');
            bubble.className = 'vd-chat-bubble ' + role;
            if (typeof innerHTML === 'string') {
                bubble.innerHTML = innerHTML;
            }
            return bubble;
        },

        appendRichBubble(chatLog, role, text) {
            const bubble = this.createBubble(role, '');
            if (role === 'user') {
                bubble.textContent = text;
            } else {
                bubble.innerHTML = this.renderMarkdown(text);
                this.processImages(bubble);
                if (window.MermaidLoader) {
                    window.MermaidLoader.processBlocks(bubble);
                }
            }
            chatLog.appendChild(bubble);
            const stamp = this.appendTimestamp(chatLog, role);
            (stamp || bubble).scrollIntoView({ block: 'end', behavior: 'smooth' });
            return bubble;
        },

        processImages(container) {
            const images = container.querySelectorAll('img');
            images.forEach(img => {
                img.style.cursor = 'pointer';
                img.classList.add('vd-chat-zoomable');
                img.addEventListener('click', () => this.openLightbox(img.src, img.alt || ''));
            });
        },

        ensureLightbox() {
            if (this._lightbox) return this._lightbox;
            const lb = document.createElement('div');
            lb.className = 'vd-lightbox-overlay';
            lb.hidden = true;
            lb.innerHTML = '<div class="vd-lightbox-backdrop"></div><div class="vd-lightbox-content"><img class="vd-lightbox-img" src="" alt=""><button class="vd-lightbox-close" type="button">&times;</button></div>';
            document.body.appendChild(lb);
            lb.querySelector('.vd-lightbox-backdrop').addEventListener('click', () => this.closeLightbox());
            lb.querySelector('.vd-lightbox-close').addEventListener('click', () => this.closeLightbox());
            document.addEventListener('keydown', (e) => {
                if (e.key === 'Escape' && !lb.hidden) this.closeLightbox();
            });
            this._lightbox = lb;
            return lb;
        },

        openLightbox(src, alt) {
            const lb = this.ensureLightbox();
            const img = lb.querySelector('.vd-lightbox-img');
            img.src = src;
            img.alt = alt;
            lb.hidden = false;
        },

        closeLightbox() {
            if (this._lightbox) this._lightbox.hidden = true;
        },

        appendImageMessage(chatLog, imgData) {
            if (!imgData || !imgData.path || this.seenSSEImages.has(imgData.path)) return;
            this.seenSSEImages.add(imgData.path);
            const cap = this.escapeHtml(imgData.caption || '');
            const bubble = this.createBubble('agent', '');
            const img = document.createElement('img');
            img.className = 'vd-chat-zoomable';
            img.src = imgData.path;
            img.alt = cap;
            img.title = cap;
            img.loading = 'lazy';
            img.style.maxWidth = '100%';
            img.style.borderRadius = '8px';
            img.style.cursor = 'pointer';
            img.addEventListener('click', () => this.openLightbox(imgData.path, cap));
            bubble.appendChild(img);
            chatLog.appendChild(bubble);
            const stamp = this.appendTimestamp(chatLog, 'agent');
            (stamp || bubble).scrollIntoView({ block: 'end', behavior: 'smooth' });
        },

        appendVideoMessage(chatLog, videoData) {
            if (!videoData || !videoData.path || this.seenSSEVideos.has(videoData.path)) return;
            this.seenSSEVideos.add(videoData.path);
            const bubble = this.createBubble('agent', '');
            const video = document.createElement('video');
            video.src = videoData.path;
            video.controls = true;
            video.style.maxWidth = '100%';
            video.style.borderRadius = '8px';
            if (videoData.title) video.title = videoData.title;
            bubble.appendChild(video);
            chatLog.appendChild(bubble);
            const stamp = this.appendTimestamp(chatLog, 'agent');
            (stamp || bubble).scrollIntoView({ block: 'end', behavior: 'smooth' });
        },

        appendLiveStreamMessage(chatLog, streamData) {
            const key = streamData && (streamData.path || streamData.stream_url || streamData.message);
            if (!streamData || !key || this.seenSSELiveStreams.has(key)) return;
            this.seenSSELiveStreams.add(key);

            const bubble = this.createBubble('agent', '');
            const wrapper = document.createElement('div');
            wrapper.className = 'chat-video-wrapper chat-live-stream-wrapper';

            const title = String(streamData.title || 'Live stream').trim();
            if (title) {
                const titleEl = document.createElement('div');
                titleEl.className = 'chat-video-title';
                titleEl.textContent = title;
                wrapper.appendChild(titleEl);
            }

            if (streamData.path) {
                const img = document.createElement('img');
                img.className = 'chat-video-player chat-live-stream';
                img.src = streamData.path;
                img.alt = title || 'Live stream';
                img.style.maxWidth = '100%';
                img.style.borderRadius = '8px';
                wrapper.appendChild(img);
            } else {
                const msg = document.createElement('div');
                msg.className = 'chat-video-actions';
                msg.textContent = String(streamData.message || streamData.error || streamData.stream_url || '').trim();
                wrapper.appendChild(msg);
            }

            bubble.appendChild(wrapper);
            chatLog.appendChild(bubble);
            const stamp = this.appendTimestamp(chatLog, 'agent');
            (stamp || bubble).scrollIntoView({ block: 'end', behavior: 'smooth' });
        },

        _playNextAudio() {
            if (!this._audioQueue.length) {
                this._audioPlaying = false;
                this._currentAudio = null;
                return;
            }
            this._audioPlaying = true;
            const src = this._audioQueue.shift();
            const audio = new Audio(src);
            this._currentAudio = audio;
            let advanced = false;
            const next = () => {
                if (advanced) return;
                advanced = true;
                if (this._currentAudio === audio) this._currentAudio = null;
                this._playNextAudio();
            };
            audio.addEventListener('ended', next, { once: true });
            audio.addEventListener('error', next, { once: true });
            audio.play().catch(next);
        },

        enqueueAudioAutoPlay(src) {
            if (!src) return;
            this._audioQueue.push(src);
            if (!this._audioPlaying) this._playNextAudio();
        },

        appendAudioMessage(chatLog, audioData) {
            if (!audioData || !audioData.path || this.seenSSEAudios.has(audioData.path)) return;
            this.seenSSEAudios.add(audioData.path);
            this.enqueueAudioAutoPlay(audioData.path);
        },

        appendDocumentMessage(chatLog, docData) {
            if (!docData || !docData.path || this.seenSSEDocuments.has(docData.path)) return;
            this.seenSSEDocuments.add(docData.path);
            const title = this.escapeHtml(docData.title || docData.filename || this.translate('desktop.chat_document', 'Document'));
            const fmt = this.escapeHtml((docData.format || '').toUpperCase() || 'FILE');
            const downloadPath = docData.path;
            const card = document.createElement('div');
            card.className = 'vd-chat-document-card';
            card.innerHTML =
                '<div class="vd-chat-document-icon"><span class="vd-chat-doc-fmt">' + fmt + '</span></div>' +
                '<div class="vd-chat-document-info"><div class="vd-chat-document-title">' + title + '</div>' +
                '<div class="vd-chat-document-format">' + fmt + '</div></div>' +
                '<div class="vd-chat-document-actions">' +
                (downloadPath ? '<a href="' + this.escapeAttr(downloadPath) + '" download="' + this.escapeHtml(docData.filename || 'document') + '" title="' + this.escapeHtml(this.translate('desktop.media_download', 'Download')) + '">&#11015;</a>' : '') +
                '</div>';
            const bubble = this.createBubble('agent', '');
            bubble.appendChild(card);
            chatLog.appendChild(bubble);
            const stamp = this.appendTimestamp(chatLog, 'agent');
            (stamp || bubble).scrollIntoView({ block: 'end', behavior: 'smooth' });
        },

        createThinkingStatus() {
            const el = document.createElement('div');
            el.className = 'vd-chat-status';
            el.innerHTML = '<span class="vd-chat-status-dot"></span><span class="vd-chat-status-text">' + this.escapeHtml(this.translate('desktop.thinking', 'Working...')) + '</span>';
            return el;
        },

        updateStatus(statusEl, text) {
            if (!statusEl) return;
            const textEl = statusEl.querySelector('.vd-chat-status-text');
            if (textEl) textEl.textContent = text;
        },

        formatAgentActionStatus(data) {
            if (!data) return '';
            const event = data.event || data.type || '';
            const detail = String(data.detail || data.message || '').trim();
            if (event === 'thinking') {
                return detail || this.translate('chat.sse_thinking', this.translate('desktop.chat_thinking', 'Reasoning...'));
            }
            if (event === 'tool_start') {
                if (detail === 'co_agent' || detail === 'co_agents') return '';
                if (detail === 'list_skills') {
                    return this.translate('chat.sse_list_skills', 'Checking available skills...');
                }
                if (detail === 'execute_skill') {
                    return this.translate('chat.sse_execute_skill', 'Running skill: ') + detail;
                }
                return this.translate('chat.sse_tool_start', this.translate('desktop.chat_using_tool', 'Using tool') + ': ') + detail;
            }
            if (event === 'tool_end') {
                if (detail === 'co_agent' || detail === 'co_agents') return '';
                return this.translate('chat.sse_tool_end', 'Tool completed: ') + detail;
            }
            if (event === 'co_agent_spawn') {
                return this.translate('chat.sse_co_agent_spawn', 'Co-Agent started: ') + detail;
            }
            if (event === 'workflow_plan') {
                return this.translate('chat.sse_workflow_plan', 'Planning next steps');
            }
            if (event === 'coding') {
                return this.translate('chat.sse_coding', 'Writing and testing a script...');
            }
            if (event === 'error_recovery') {
                return this.translate('chat.sse_error_recovery', 'Script had an error. Fixing code...');
            }
            return '';
        },

        extractToolCallNarration(text) {
            let narration = String(text || '')
                .replace(/```json[\s\S]*?```/g, '')
                .replace(/`[^`]*`/g, '')
                .replace(/\{[\s\S]*"action"\s*:[\s\S]*/g, '')
                .replace(/\{[\s\S]*"tool"\s*:[\s\S]*/g, '')
                .replace(/\{[\s\S]*"tool_call"\s*:[\s\S]*/g, '')
                .replace(/\{[\s\S]*"tool_name"\s*:[\s\S]*/g, '')
                .replace(/\{[\s\S]*"parameters"\s*:[\s\S]*/g, '')
                .trim();
            narration = this.stripLeakedToolMarkup(narration);
            return narration && narration.split(/\s+/).filter(Boolean).length >= 6 ? narration : '';
        },

        createOpenInAppButton(appId, path) {
            const appNames = {
                writer: this.translate('desktop.app_writer', 'Writer'),
                sheets: this.translate('desktop.app_sheets', 'Sheets'),
                'code-studio': this.translate('desktop.app_code_studio', 'Code Studio')
            };
            const label = appNames[appId] || appId;
            const btn = document.createElement('button');
            btn.className = 'vd-chat-open-app-btn';
            btn.textContent = this.translate('desktop.chat_open_in', 'Open in') + ' ' + label;
            btn.dataset.appId = appId;
            if (path) btn.dataset.path = path;
            return btn;
        },

        resetDedupSets() {
            this.seenSSEImages.clear();
            this.seenSSEVideos.clear();
            this.seenSSELiveStreams.clear();
            this.seenSSEAudios.clear();
            this.seenSSEDocuments.clear();
        }
    };

    window.DesktopChatRenderer = DesktopChatRenderer;
})();
