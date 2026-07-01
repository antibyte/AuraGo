(function () {
    'use strict';

    const THEME = 'cyberwar';
    const CHAT_BOX_ID = 'chat-box';
    const HUD_ID = 'cyberwar-hud';
    const BLIP_LIMIT = 5;

    let hud = null;
    let messageCountEl = null;
    let tokenCountEl = null;
    let toolCountEl = null;
    let threatFillEl = null;
    let threatLevelEl = null;
    let radarFieldEl = null;
    let agentNodeDot = null;
    let llmNodeDot = null;
    let vaultNodeDot = null;
    let webhooksNodeDot = null;

    let counters = { messages: 0, tools: 0 };
    let threatLevel = 0;
    let threatDecayTimer = null;
    let threatBumpDebounce = 0;
    let updateFrame = 0;
    let unsubscribeFns = [];
    let _wired = false;

    const motionQuery = window.matchMedia ? window.matchMedia('(prefers-reduced-motion: reduce)') : null;
    const mobileQuery = window.matchMedia ? window.matchMedia('(max-width: 767px)') : null;

    function currentTheme() {
        return document.documentElement.getAttribute('data-theme') || 'dark';
    }

    function shouldRun() {
        return currentTheme() === THEME && window.innerWidth >= 768;
    }

    function prefersReducedMotion() {
        return !!(motionQuery && motionQuery.matches);
    }

    function getConversationLength() {
        try {
            if (typeof conversation !== 'undefined' && Array.isArray(conversation)) {
                return conversation.length;
            }
        } catch (_) { /* ignore */ }
        return 0;
    }

    function getTokenText() {
        const el = document.getElementById('tokenCounter');
        if (!el) return '';
        return (el.textContent || '').trim();
    }

    function formatTokenShort() {
        const raw = getTokenText();
        const match = raw.replace(/[^\d.,]/g, '');
        const num = parseInt(match.replace(/[.,]/g, ''), 10);
        if (!Number.isFinite(num) || num <= 0) return '0';
        if (num >= 1000000) return (num / 1000000).toFixed(1).replace(/\.0$/, '') + 'M';
        if (num >= 1000) return (num / 1000).toFixed(1).replace(/\.0$/, '') + 'K';
        return String(num);
    }

    function readConnectionPillState() {
        const pill = document.getElementById('connectionPill');
        if (!pill) return 'reconnecting';
        if (pill.classList.contains('pill-active')) return 'connected';
        if (pill.classList.contains('pill-reconnecting')) return 'reconnecting';
        return 'disconnected';
    }

    function setNodeDot(dot, status, label) {
        if (!dot) return;
        dot.classList.remove('node-online', 'node-offline', 'node-pulsing');
        dot.classList.add(`node-${status}`);
        const text = dot.parentElement && dot.parentElement.querySelector('.hud-node-status');
        if (text) {
            text.textContent = label;
            text.classList.remove('status-online', 'status-offline', 'status-pulsing');
            text.classList.add(`status-${status}`);
        }
    }

    function refreshNodes() {
        const sseConnected = !!(window.AuraSSE && typeof window.AuraSSE.isConnected === 'function' && window.AuraSSE.isConnected());
        const pillState = readConnectionPillState();
        let linkState = 'reconnecting';
        let linkLabel = 'NO LINK';
        if (pillState === 'connected' || sseConnected) {
            linkState = 'online';
            linkLabel = 'ONLINE';
        } else if (pillState === 'reconnecting') {
            linkState = 'pulsing';
            linkLabel = 'LINKING';
        } else {
            linkState = 'offline';
            linkLabel = 'OFFLINE';
        }
        setNodeDot(agentNodeDot, linkState, linkLabel);

        const llmOk = sseConnected || pillState === 'connected';
        setNodeDot(llmNodeDot, llmOk ? 'online' : 'offline', llmOk ? 'READY' : 'IDLE');

        setNodeDot(vaultNodeDot, 'online', 'SEALED');
        setNodeDot(webhooksNodeDot, 'online', 'ARMED');
    }

    function updateCounter(el, value) {
        if (!el) return;
        const display = typeof value === 'number' ? value.toLocaleString() : String(value || '');
        if (el.textContent !== display) el.textContent = display;
    }

    function scheduleCounterUpdate() {
        if (updateFrame) return;
        const raf = window.requestAnimationFrame || ((cb) => window.setTimeout(cb, 16));
        updateFrame = raf(() => {
            updateFrame = 0;
            const next = getConversationLength();
            if (next !== counters.messages) {
                counters.messages = next;
                updateCounter(messageCountEl, counters.messages);
            }
            const tk = formatTokenShort();
            if (tokenCountEl && tokenCountEl.textContent !== tk) tokenCountEl.textContent = tk;
            updateCounter(toolCountEl, counters.tools);
            updateThreatGauge();
            refreshNodes();
        });
    }

    function updateThreatGauge() {
        if (!threatFillEl || !threatLevelEl) return;
        const pct = Math.max(8, Math.min(100, 8 + threatLevel * 14));
        threatFillEl.style.width = pct + '%';
        let label = 'GREEN';
        let cls = 'threat-green';
        if (threatLevel >= 4) { label = 'RED'; cls = 'threat-red'; }
        else if (threatLevel >= 2) { label = 'AMBER'; cls = 'threat-amber'; }
        else if (threatLevel >= 1) { label = 'YELLOW'; cls = 'threat-yellow'; }
        threatLevelEl.textContent = label;
        threatFillEl.classList.remove('threat-green', 'threat-yellow', 'threat-amber', 'threat-red');
        threatFillEl.classList.add(cls);
    }

    function bumpThreat(amount) {
        threatLevel = Math.min(6, threatLevel + amount);
        if (threatDecayTimer) clearTimeout(threatDecayTimer);
        threatDecayTimer = window.setTimeout(() => {
            threatLevel = Math.max(0, threatLevel - 1);
            updateThreatGauge();
        }, 12000);
        updateThreatGauge();
    }

    function addBlip(toolName, isError) {
        if (!radarFieldEl) return;
        while (radarFieldEl.children.length >= BLIP_LIMIT) {
            radarFieldEl.removeChild(radarFieldEl.firstChild);
        }
        const blip = document.createElement('span');
        blip.className = 'hud-blist-item' + (isError ? ' hud-blist-error' : '');
        blip.title = toolName || '';
        const dot = document.createElement('span');
        dot.className = 'hud-blist-dot';
        blip.appendChild(dot);
        radarFieldEl.appendChild(blip);
        blip.addEventListener('animationend', () => {
            if (blip.parentElement === radarFieldEl) radarFieldEl.removeChild(blip);
        }, { once: true });
    }

    function onAgentAction(payload) {
        if (!payload) return;
        const state = String(payload.state || '').toLowerCase();
        const toolName = payload.tool_name || payload.toolName || 'tool';
        if (state === 'started') {
            counters.tools += 1;
            addBlip(toolName, false);
        } else if (state === 'failed' || state === 'blocked' || state === 'cancelled') {
            addBlip(toolName, true);
            bumpThreat(2);
        } else if (state === 'succeeded' || state === 'sanitized') {
            addBlip(toolName, false);
        }
        scheduleCounterUpdate();
    }

    function onTokenUpdate() {
        scheduleCounterUpdate();
    }

    function onConnectionEvent() {
        refreshNodes();
        if (!window.AuraSSE || !window.AuraSSE.isConnected()) {
            const now = Date.now();
            if (now - threatBumpDebounce > 8000) {
                threatBumpDebounce = now;
                bumpThreat(1);
            }
        }
    }

    function wireSSE() {
        if (!window.AuraSSE || _wired) return;
        if (typeof window.AuraSSE.on !== 'function') return;
        _wired = true;
        window.AuraSSE.on('agent_action', onAgentAction);
        window.AuraSSE.on('token_update', onTokenUpdate);
        window.AuraSSE.on('_open', onConnectionEvent);
        window.AuraSSE.on('_error', onConnectionEvent);
        unsubscribeFns.push(() => {
            if (typeof window.AuraSSE.off === 'function') {
                window.AuraSSE.off('agent_action', onAgentAction);
                window.AuraSSE.off('token_update', onTokenUpdate);
                window.AuraSSE.off('_open', onConnectionEvent);
                window.AuraSSE.off('_error', onConnectionEvent);
            }
            _wired = false;
        });
    }

    function unwireSSE() {
        unsubscribeFns.forEach((fn) => { try { fn(); } catch (_) { /* ignore */ } });
        unsubscribeFns = [];
    }

    function buildHUD() {
        const root = document.createElement('div');
        root.id = HUD_ID;
        root.setAttribute('aria-hidden', 'true');
        root.innerHTML = [
            '<div class="hud-panel system-nodes" data-panel="nodes">',
            '  <div class="hud-panel-title">// NODES</div>',
            '  <div class="hud-node" data-node="agent"><span class="hud-node-dot"></span><span class="hud-node-name">AGENT</span><span class="hud-node-status">OFFLINE</span></div>',
            '  <div class="hud-node" data-node="llm"><span class="hud-node-dot"></span><span class="hud-node-name">LLM</span><span class="hud-node-status">IDLE</span></div>',
            '  <div class="hud-node" data-node="vault"><span class="hud-node-dot"></span><span class="hud-node-name">VAULT</span><span class="hud-node-status">SEALED</span></div>',
            '  <div class="hud-node" data-node="webhooks"><span class="hud-node-dot"></span><span class="hud-node-name">HOOKS</span><span class="hud-node-status">ARMED</span></div>',
            '</div>',
            '<div class="hud-panel threat-level" data-panel="threat">',
            '  <div class="hud-panel-title">// THREAT</div>',
            '  <div class="hud-threat-row"><span class="hud-threat-level" data-threat-level>GREEN</span></div>',
            '  <div class="hud-threat-gauge"><div class="hud-threat-fill threat-green" data-threat-fill></div></div>',
            '</div>',
            '<div class="hud-panel data-counters" data-panel="counters">',
            '  <div class="hud-panel-title">// DATA</div>',
            '  <div class="hud-counter"><span class="hud-counter-label">MSG</span><span class="hud-counter-value" data-counter="messages">0</span></div>',
            '  <div class="hud-counter"><span class="hud-counter-label">TOK</span><span class="hud-counter-value" data-counter="tokens">0</span></div>',
            '  <div class="hud-counter"><span class="hud-counter-label">OPS</span><span class="hud-counter-value" data-counter="tools">0</span></div>',
            '</div>',
            '<div class="hud-panel activity-radar" data-panel="radar">',
            '  <div class="hud-panel-title">// RADAR</div>',
            '  <div class="hud-blist" data-radar-field></div>',
            '</div>',
            '<div class="hud-panel security-status" data-panel="security">',
            '  <div class="hud-panel-title">// CRYPTO</div>',
            '  <div class="hud-security-badge"><span class="hud-security-dot"></span><span>AES-256-GCM</span></div>',
            '</div>'
        ].join('');
        return root;
    }

    function captureRefs() {
        messageCountEl = hud.querySelector('[data-counter="messages"]');
        tokenCountEl = hud.querySelector('[data-counter="tokens"]');
        toolCountEl = hud.querySelector('[data-counter="tools"]');
        threatFillEl = hud.querySelector('[data-threat-fill]');
        threatLevelEl = hud.querySelector('[data-threat-level]');
        radarFieldEl = hud.querySelector('[data-radar-field]');
        agentNodeDot = hud.querySelector('[data-node="agent"] .hud-node-dot');
        llmNodeDot = hud.querySelector('[data-node="llm"] .hud-node-dot');
        vaultNodeDot = hud.querySelector('[data-node="vault"] .hud-node-dot');
        webhooksNodeDot = hud.querySelector('[data-node="webhooks"] .hud-node-dot');
    }

    function ensureHUD() {
        const chatBox = document.getElementById(CHAT_BOX_ID);
        if (!chatBox) return null;
        if (hud && hud.parentElement !== chatBox) hud = null;
        if (!hud) {
            hud = document.getElementById(HUD_ID);
            if (!hud) {
                hud = buildHUD();
                chatBox.insertBefore(hud, chatBox.firstChild);
            }
            captureRefs();
        }
        return hud;
    }

    function destroyHUD() {
        if (hud && hud.parentElement) hud.parentElement.removeChild(hud);
        hud = null;
        messageCountEl = null;
        tokenCountEl = null;
        toolCountEl = null;
        threatFillEl = null;
        threatLevelEl = null;
        radarFieldEl = null;
        agentNodeDot = null;
        llmNodeDot = null;
        vaultNodeDot = null;
        webhooksNodeDot = null;
        counters = { messages: 0, tools: 0 };
        threatLevel = 0;
        if (threatDecayTimer) { clearTimeout(threatDecayTimer); threatDecayTimer = null; }
        if (updateFrame) { (window.cancelAnimationFrame || window.clearTimeout)(updateFrame); updateFrame = 0; }
    }

    function start() {
        if (!ensureHUD()) return;
        wireSSE();
        hud.classList.add('hud-active');
        if (prefersReducedMotion()) hud.classList.add('hud-reduced-motion');
        scheduleCounterUpdate();
        refreshNodes();
    }

    function stop() {
        unwireSSE();
        destroyHUD();
    }

    function sync() {
        if (shouldRun()) {
            const chatBox = document.getElementById(CHAT_BOX_ID);
            if (chatBox) {
                start();
            } else if (!sync._waiting) {
                sync._waiting = { attempts: 0, timer: 0 };
                const tryStart = () => {
                    const w = sync._waiting;
                    if (!shouldRun()) { sync._waiting = null; return; }
                    if (document.getElementById(CHAT_BOX_ID)) {
                        sync._waiting = null;
                        start();
                        return;
                    }
                    w.attempts += 1;
                    if (w.attempts > 50) {
                        sync._waiting = null;
                        return;
                    }
                    w.timer = window.setTimeout(tryStart, 100);
                };
                tryStart();
            }
        } else if (hud) {
            stop();
        }
    }

    function init() {
        if (document.readyState === 'loading') {
            document.addEventListener('DOMContentLoaded', init, { once: true });
            return;
        }
        window.addEventListener('aurago:themechange', sync);
        window.addEventListener('aurago:theme-effects-loaded', function (e) {
            if (e && e.detail && e.detail.theme === THEME) sync();
        });
        window.addEventListener('resize', sync);
        if (motionQuery) {
            if (typeof motionQuery.addEventListener === 'function') {
                motionQuery.addEventListener('change', sync);
            } else if (typeof motionQuery.addListener === 'function') {
                motionQuery.addListener(sync);
            }
        }
        if (mobileQuery) {
            if (typeof mobileQuery.addEventListener === 'function') {
                mobileQuery.addEventListener('change', sync);
            } else if (typeof mobileQuery.addListener === 'function') {
                mobileQuery.addListener(sync);
            }
        }
        if (typeof MutationObserver !== 'undefined') {
            new MutationObserver(sync).observe(document.documentElement, {
                attributes: true,
                attributeFilter: ['data-theme']
            });
        }
        sync();
    }

    window.AuraGoCyberwarHud = { start, stop, sync };
    init();
})();