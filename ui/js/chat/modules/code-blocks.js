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

        createCodeBlock(code, lang) {
            const lines = code.split('\n');
            const lineNumbers = lines.map((_, i) => i + 1).join('\n');
            
            // Syntax highlighting
            let highlighted = code;
            if (lang && window.hljs && hljs.getLanguage(lang)) {
                try {
                    highlighted = hljs.highlight(code, { language: lang, ignoreIllegals: true }).value;
                } catch (e) {
                    highlighted = this.escapeHtml(code);
                }
            } else {
                highlighted = this.escapeHtml(code);
            }
            
            return `
                <div class="code-block-wrapper">
                    <div class="code-header">
                        <span class="code-lang">${lang || 'text'}</span>
                        <button class="copy-btn" onclick="copyCodeBlock(this)" data-code="${this.encodeHtml(code)}">
                            ${window.chatUiIconMarkup ? window.chatUiIconMarkup('clipboard', 'copy-icon') : ''}
                            <span class="copy-text">${t('chat.code_copy')}</span>
                        </button>
                    </div>
                    <div class="code-body">
                        <div class="line-numbers">${lineNumbers}</div>
                        <pre><code class="hljs ${lang || ''}">${highlighted}</code></pre>
                    </div>
                </div>
            `;
        },

        copyCodeBlock(btn) {
            const code = this.decodeHtml(btn.dataset.code);
            
            navigator.clipboard.writeText(code).then(() => {
                const originalHTML = btn.innerHTML;
                btn.classList.add('copied');
                btn.innerHTML = `
                    ${window.chatUiIconMarkup ? window.chatUiIconMarkup('complete', 'copy-icon') : ''}
                    <span class="copy-text">${t('chat.code_copied')}</span>
                `;
                
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
        },

        decodeHtml(html) {
            const textarea = document.createElement('textarea');
            textarea.innerHTML = html;
            return textarea.value;
        }
    };

    window.CodeBlocks = CodeBlocks;
})();
