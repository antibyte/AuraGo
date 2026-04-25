/**
 * Smart Scroller Module
 * Handles intelligent auto-scroll that respects user reading position
 */

(function() {
    'use strict';

    const SmartScroller = {
        container: null,
        scrollButton: null,
        isUserScrolledUp: false,
        scrollThreshold: 150,
        newMessagesCount: 0,
        isInitialized: false,

        init(container) {
            if (this.isInitialized || !container) return;
            
            this.container = container;
            this.createScrollButton();
            this.bindEvents();
            this.isInitialized = true;
        },

        createScrollButton() {
            this.scrollButton = document.createElement('button');
            this.scrollButton.className = 'scroll-to-bottom-btn';
            this.scrollButton.innerHTML = `
                ${window.chatUiIconMarkup ? window.chatUiIconMarkup('scroll-down', 'stb-icon') : ''}
                <span class="stb-count"></span>
            `;
            this.scrollButton.style.display = 'none';
            this.scrollButton.setAttribute('aria-label', 'Scroll to new messages');
            
            this.scrollButton.addEventListener('click', () => {
                this.scrollToBottom(true);
                this.newMessagesCount = 0;
                this.updateButton();
            });
            
            document.body.appendChild(this.scrollButton);
        },

        bindEvents() {
            let scrollTimeout;
            this.container.addEventListener('scroll', () => {
                clearTimeout(scrollTimeout);
                scrollTimeout = setTimeout(() => this.onScroll(), 50);
            }, { passive: true });

            this._resizeHandler = () => {
                if (!this.isUserScrolledUp) {
                    this.scrollToBottom(true);
                }
            };
            window.addEventListener('resize', this._resizeHandler);
        },

        onScroll() {
            const { scrollTop, scrollHeight, clientHeight } = this.container;
            const distanceFromBottom = scrollHeight - scrollTop - clientHeight;
            
            const wasScrolledUp = this.isUserScrolledUp;
            this.isUserScrolledUp = distanceFromBottom > this.scrollThreshold;
            
            if (!this.isUserScrolledUp && wasScrolledUp) {
                this.newMessagesCount = 0;
            }
            
            this.updateButton();
        },

        updateButton() {
            if (!this.scrollButton) return;

            if (this.isUserScrolledUp) {
                this.scrollButton.style.display = 'flex';
                const countEl = this.scrollButton.querySelector('.stb-count');
                
                if (this.newMessagesCount > 0) {
                    countEl.textContent = this.newMessagesCount;
                    this.scrollButton.classList.add('has-new');
                } else {
                    countEl.textContent = '';
                    this.scrollButton.classList.remove('has-new');
                }
            } else {
                this.scrollButton.style.display = 'none';
                this.scrollButton.classList.remove('has-new');
            }
        },

        onNewMessage() {
            if (!this.isInitialized) return;
            
            if (this.isUserScrolledUp) {
                this.newMessagesCount++;
                this.updateButton();
            } else {
                this.scrollToBottom(true);
            }
        },

        scrollToBottom(smooth = true) {
            if (!this.container) return;
            
            this.container.scrollTo({
                top: this.container.scrollHeight,
                behavior: smooth ? 'smooth' : 'auto'
            });
        },

        destroy() {
            if (this._resizeHandler) {
                window.removeEventListener('resize', this._resizeHandler);
                this._resizeHandler = null;
            }
            if (this.scrollButton && this.scrollButton.parentNode) {
                this.scrollButton.parentNode.removeChild(this.scrollButton);
                this.scrollButton = null;
            }
            this.isInitialized = false;
        }
    };

    window.SmartScroller = SmartScroller;
})();
