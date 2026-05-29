// AuraGo – index page logic
// Extracted from index.html

/* ── DOM refs ── */
const chatBox = document.getElementById('chat-box');
const chatContent = document.getElementById('chat-content');
const chatForm = document.getElementById('chat-form');
const userInput = document.getElementById('user-input');
const sendBtn = document.getElementById('send-btn');
const stopBtn = document.getElementById('stop-btn');
const composerMoreBtn = document.getElementById('composer-more-btn');
const composerPanel = document.getElementById('composer-panel');
const feedbackToggleBtn = document.getElementById('feedback-toggle-btn');
const moodFeedbackRow = document.getElementById('mood-feedback-row');
const cheatsheetPickerBtn = document.getElementById('cheatsheet-picker-btn');

const cheatsheetPickerOverlay = document.getElementById('cheatsheet-picker-overlay');
const cheatsheetPickerList = document.getElementById('cheatsheet-picker-list');
const cheatsheetPickerSendBtn = document.getElementById('cheatsheet-picker-send');
const cheatsheetPickerCancelBtn = document.getElementById('cheatsheet-picker-cancel');
const cheatsheetPickerCloseXBtn = document.getElementById('cheatsheet-picker-close-x');

let cheatsheetPickerItems = [];
let selectedCheatsheetId = '';

function chatIconMarkup(iconName, className = '') {
    return window.chatUiIconMarkup ? window.chatUiIconMarkup(iconName, className) : '';
}

function applyChatIcon(el, iconName) {
    if (!el) return;
    if (window.AuraChatIcons) {
        window.AuraChatIcons.applyIcon(el, iconName);
    } else {
        el.dataset.chatIcon = iconName;
    }
}

function setIconButton(btn, iconName) {
    if (!btn) return;
    btn.textContent = '';
    if (window.AuraChatIcons) {
        btn.appendChild(window.AuraChatIcons.createIcon(iconName));
    }
}

function setIconPillText(el, iconName, text) {
    if (!el) return;
    el.textContent = '';
    if (window.AuraChatIcons) {
        el.appendChild(window.AuraChatIcons.createIcon(iconName));
        el.appendChild(document.createTextNode(' '));
    }
    el.appendChild(document.createTextNode(text));
}

function bindHeaderActivation(el, handler) {
    if (!el || el.dataset.chatHeaderActivationBound === 'true') return;
    el.dataset.chatHeaderActivationBound = 'true';
    el.dataset.headerTouchBound = 'true';

    let lastDirectActivation = 0;
    let touchStartX = 0;
    let touchStartY = 0;
    let touchStartScroll = 0;
    let touchMoved = false;
    let trackingTouch = false;
    let suppressClickUntil = 0;
    const tapSlop = 12;

    function headerScrollPosition() {
        const scroller = document.getElementById('chat-box');
        return (window.scrollY || 0)
            + (document.documentElement ? document.documentElement.scrollTop || 0 : 0)
            + (document.body ? document.body.scrollTop || 0 : 0)
            + (scroller ? scroller.scrollTop || 0 : 0);
    }

    function rememberTouchStart(clientX, clientY) {
        trackingTouch = true;
        touchMoved = false;
        touchStartX = clientX;
        touchStartY = clientY;
        touchStartScroll = headerScrollPosition();
    }

    function markTouchMove(clientX, clientY) {
        if (!trackingTouch) return;
        if (
            Math.abs(clientX - touchStartX) > tapSlop ||
            Math.abs(clientY - touchStartY) > tapSlop ||
            Math.abs(headerScrollPosition() - touchStartScroll) > 2
        ) {
            touchMoved = true;
            suppressClickUntil = Date.now() + 500;
        }
    }

    function preventDuplicate(event) {
        if (event.cancelable) event.preventDefault();
        event.stopPropagation();
    }

    function activate(event) {
        const now = Date.now();
        if (event.type === 'pointerup' && (!event.pointerType || event.pointerType === 'mouse')) return;
        const isDirectTouch = event.type === 'touchend' || event.type === 'pointerup';

        if (isDirectTouch) {
            if (event.type === 'pointerup') markTouchMove(event.clientX, event.clientY);
            const moved = touchMoved || Math.abs(headerScrollPosition() - touchStartScroll) > 2;
            trackingTouch = false;
            if (moved) {
                suppressClickUntil = Date.now() + 500;
                return;
            }
            preventDuplicate(event);
            if (now - lastDirectActivation < 450) return;
            lastDirectActivation = now;
        } else if (event.type === 'click' && (now - lastDirectActivation < 450 || now < suppressClickUntil)) {
            preventDuplicate(event);
            return;
        }

        handler(event);
    }

    el.addEventListener('pointerdown', (event) => {
        if (event.pointerType === 'mouse') return;
        rememberTouchStart(event.clientX, event.clientY);
    }, { passive: true });
    el.addEventListener('pointermove', (event) => {
        if (event.pointerType === 'mouse') return;
        markTouchMove(event.clientX, event.clientY);
    }, { passive: true });
    el.addEventListener('pointercancel', () => {
        trackingTouch = false;
        touchMoved = true;
        suppressClickUntil = Date.now() + 500;
    }, { passive: true });
    el.addEventListener('touchstart', (event) => {
        const touch = event.changedTouches && event.changedTouches[0];
        if (!touch) return;
        rememberTouchStart(touch.clientX, touch.clientY);
    }, { passive: true });
    el.addEventListener('touchmove', (event) => {
        const touch = event.changedTouches && event.changedTouches[0];
        if (!touch) return;
        markTouchMove(touch.clientX, touch.clientY);
    }, { passive: true });
    el.addEventListener('touchcancel', () => {
        trackingTouch = false;
        touchMoved = true;
        suppressClickUntil = Date.now() + 500;
    }, { passive: true });
    el.addEventListener('click', activate);
    el.addEventListener('pointerup', activate);
    el.addEventListener('touchend', activate, { passive: false });
}
