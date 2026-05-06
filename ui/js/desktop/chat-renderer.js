(function () {
    'use strict';

    const DesktopChatRenderer = {
        seenSSEImages: new Set(),
        seenSSEVideos: new Set(),
        seenSSEAudios: new Set(),
        seenSSEDocuments: new Set(),
        _md: null,
        _lightbox: null,

        escapeHtml(str) {
            return String(str)
                .replace(/&/g, '&amp;')
                .replace(/</g, '&lt;')
                .replace(/>/g, '&gt;')
                .replace(/"/g, '&quot;');
        },

        escapeAttr(s) {
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

        getMarkdown() {
            if (this._md) return this._md;
            if (typeof window.markdownit === 'undefined') return null;
            const self = this;
            this._md = window.markdownit({
                html: false,
                breaks: true,
                linkify: true,
                highlight: function (str, lang) {
                    if (lang === 'mermaid') {
                        return '<div class="mermaid-raw">' + self.escapeHtml(str) + '</div>';
                    }
                    if (lang && window.hljs && hljs.getLanguage(lang)) {
                        try {
                            return '<pre class="hljs"><code>' +
                                hljs.highlight(str, { language: lang, ignoreIllegals: true }).value +
                                '</code></pre>';
                        } catch (__) {}
                    }
                    return '<pre class="hljs"><code>' + self.escapeHtml(str) + '</code></pre>';
                }
            });
            return this._md;
        },

        stripLeakedToolMarkup(text) {
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
                displayContent = displayContent.replace(/!\[[^\]]*\]\(([^)]+)\)/g, (match, url) =>
                    this.seenSSEImages.has(url) ? '' : match
                ).trim();
                if (!displayContent) return '';
            }
            const contentStripped = displayContent.replace(
                /<external_data>([\s\S]*?)<\/external_data>/gi,
                (_, inner) => inner.trim()
            );
            const thinkingBlocks = [];
            const contentForRender = contentStripped.replace(
                /<(thinking|think)>([\s\S]*?)<\/\1>/gi,
                (match, _tag, inner) => {
                    const idx = thinkingBlocks.length;
                    thinkingBlocks.push(inner.trim());
                    return '\n\n%%THINKING_BLOCK_' + idx + '%%\n\n';
                }
            );
            let finalHTML = md.render(contentForRender);
            finalHTML = finalHTML.replace(/<a(\s+[^>]*)?\s+href="([^"]+)"/g,
                '<a$1href="$2" target="_blank" rel="noopener noreferrer"');
            thinkingBlocks.forEach((innerText, idx) => {
                const innerHtml = md.render(innerText);
                const label = this.translate('chat.thinking_label', 'Reasoning');
                const detailsHtml = '<details class="vd-thinking-block"><summary>' + label + '</summary><div class="vd-thinking-content">' + innerHtml + '</div></details>';
                finalHTML = finalHTML.replace(new RegExp('<p>%%THINKING_BLOCK_' + idx + '%%</p>', 'g'), detailsHtml);
                finalHTML = finalHTML.replace(new RegExp('%%THINKING_BLOCK_' + idx + '%%', 'g'), detailsHtml);
            });
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
            bubble.scrollIntoView({ block: 'end', behavior: 'smooth' });
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
            lb.querySelector('.vd-lightbox-img').src = src;
            lb.querySelector('.vd-lightbox-img').alt = alt;
            lb.hidden = false;
        },

        closeLightbox() {
            if (this._lightbox) this._lightbox.hidden = true;
        },

        appendImageMessage(chatLog, imgData) {
            if (!imgData || !imgData.path || this.seenSSEImages.has(imgData.path)) return;
            this.seenSSEImages.add(imgData.path);
            const cap = this.escapeHtml(imgData.caption || '');
            const safePath = this.escapeAttr(imgData.path);
            const bubble = this.createBubble('agent', '');
            const img = document.createElement('img');
            img.className = 'vd-chat-zoomable';
            img.src = safePath;
            img.alt = cap;
            img.title = cap;
            img.loading = 'lazy';
            img.style.maxWidth = '100%';
            img.style.borderRadius = '8px';
            img.style.cursor = 'pointer';
            img.addEventListener('click', () => this.openLightbox(safePath, cap));
            bubble.appendChild(img);
            chatLog.appendChild(bubble);
            bubble.scrollIntoView({ block: 'end', behavior: 'smooth' });
        },

        appendVideoMessage(chatLog, videoData) {
            if (!videoData || !videoData.path || this.seenSSEVideos.has(videoData.path)) return;
            this.seenSSEVideos.add(videoData.path);
            const bubble = this.createBubble('agent', '');
            const video = document.createElement('video');
            video.src = this.escapeAttr(videoData.path);
            video.controls = true;
            video.style.maxWidth = '100%';
            video.style.borderRadius = '8px';
            if (videoData.title) video.title = videoData.title;
            bubble.appendChild(video);
            chatLog.appendChild(bubble);
            bubble.scrollIntoView({ block: 'end', behavior: 'smooth' });
        },

        appendAudioMessage(chatLog, audioData) {
            if (!audioData || !audioData.path || this.seenSSEAudios.has(audioData.path)) return;
            this.seenSSEAudios.add(audioData.path);
            const bubble = this.createBubble('agent', '');
            const wrapper = document.createElement('div');
            wrapper.className = 'vd-chat-audio-wrapper';
            if (audioData.title) {
                const titleEl = document.createElement('div');
                titleEl.className = 'vd-chat-audio-title';
                titleEl.textContent = audioData.title;
                wrapper.appendChild(titleEl);
            }
            const audio = document.createElement('audio');
            audio.controls = true;
            audio.src = this.escapeAttr(audioData.path);
            audio.style.width = '100%';
            wrapper.appendChild(audio);
            bubble.appendChild(wrapper);
            chatLog.appendChild(bubble);
            bubble.scrollIntoView({ block: 'end', behavior: 'smooth' });
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
            bubble.scrollIntoView({ block: 'end', behavior: 'smooth' });
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
            this.seenSSEAudios.clear();
            this.seenSSEDocuments.clear();
        }
    };

    window.DesktopChatRenderer = DesktopChatRenderer;
})();
