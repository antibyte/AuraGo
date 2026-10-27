/**
 * Code Blocks Module
 * Enhances code blocks with copy button and line numbers
 */

(function() {
    'use strict';

    const CodeBlocks = {
        init() {
            // Copy function exposed globally for onclick handlers
            window.copyCodeBlock = this.copyCodeBlock.bind(this);
        },

        // Returns block markup without layout whitespace: the code body keeps
        // white-space: pre, so template indentation would render as empty lines.
        createCodeBlock(code, lang) {
            // markdown-it fence content ends with the newline before the closing fence.
            const source = String(code == null ? '' : code).replace(/\n$/, '');
            const language = String(lang || '').trim();
            const lineNumbers = source.split('\n').map((_, i) => i + 1).join('\n');

            // Syntax highlighting
            let highlighted = source;
            if (language && window.hljs && hljs.getLanguage(language)) {
                try {
                    highlighted = hljs.highlight(source, { language, ignoreIllegals: true }).value;
                } catch (e) {
                    highlighted = this.escapeHtml(source);
                }
            } else {
                highlighted = this.escapeHtml(source);
            }

            return [
                '<div class="code-block-wrapper">',
                `<div class="code-header"><span class="code-lang">${this.encodeHtml(language || 'text')}</span></div>`,
                '<div class="code-body">',
                `<div class="line-numbers">${lineNumbers}</div>`,
                `<pre><code class="hljs ${this.encodeHtml(language)}">${highlighted}</code></pre>`,
                '</div>',
                '</div>'
            ].join('');
        },

        // The chat sanitizer drops <button> elements and inline handlers, so
        // copy buttons are attached to the rendered blocks afterwards.
        enhance(root) {
            if (!root || !root.querySelectorAll) return;
            root.querySelectorAll('.code-block-wrapper > .code-header').forEach((header) => {
                if (header.querySelector('.copy-btn')) return;
                const btn = document.createElement('button');
                btn.type = 'button';
                btn.className = 'copy-btn';
                btn.innerHTML = this.copyLabelMarkup('clipboard', 'chat.code_copy');
                btn.addEventListener('click', () => this.copyCodeBlock(btn));
                header.appendChild(btn);
            });
        },

        copyLabelMarkup(icon, labelKey) {
            const iconMarkup = window.chatUiIconMarkup ? window.chatUiIconMarkup(icon, 'copy-icon') : '';
            return `${iconMarkup}<span class="copy-text">${this.escapeHtml(t(labelKey))}</span>`;
        },

        copyCodeBlock(btn) {
            const wrapper = btn.closest('.code-block-wrapper');
            const codeEl = wrapper && wrapper.querySelector('.code-body code');
            const code = codeEl ? codeEl.textContent : '';

            navigator.clipboard.writeText(code).then(() => {
                const originalHTML = btn.innerHTML;
                btn.classList.add('copied');
                btn.innerHTML = this.copyLabelMarkup('complete', 'chat.code_copied');

                setTimeout(() => {
                    btn.classList.remove('copied');
                    btn.innerHTML = originalHTML;
                }, 2000);
            }).catch(err => {
                console.error('[CodeBlocks] Copy failed:', err);
                // Fallback for mobile
                this.fallbackCopy(code, btn);
            });
        },

        fallbackCopy(text, btn) {
            // Create temporary textarea for mobile browsers
            const textarea = document.createElement('textarea');
            textarea.value = text;
            textarea.style.cssText = 'position:fixed;left:-9999px;top:0';
            document.body.appendChild(textarea);
            
            try {
                textarea.select();
                textarea.setSelectionRange(0, 99999); // For mobile
                document.execCommand('copy');
                btn.classList.add('copied');
                setTimeout(() => btn.classList.remove('copied'), 2000);
            } catch (e) {
                console.error('[CodeBlocks] Fallback copy failed:', e);
            } finally {
                document.body.removeChild(textarea);
            }
        },

        escapeHtml(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        },

        encodeHtml(text) {
            return text
                .replace(/&/g, '&amp;')
                .replace(/</g, '&lt;')
                .replace(/>/g, '&gt;')
                .replace(/"/g, '&quot;')
                .replace(/'/g, '&#039;');
        }
    };

    window.CodeBlocks = CodeBlocks;
})();
