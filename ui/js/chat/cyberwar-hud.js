(function () {
    'use strict';

    const THEME = 'cyberwar';
    const CHAT_BOX_ID = 'chat-box';
    const HUD_ID = 'cyberwar-hud';
    const HUD_LEFT_ID = 'cyberwar-hud-left';
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

    let leftHud = null;
    let cpuEqBars = null;
    let cpuEqValue = null;
    let cpuEqSub = null;
    let ramEqBars = null;
    let ramEqValue = null;
    let ramEqSub = null;
    let diskEqBars = null;
    let diskEqValue = null;
    let diskEqSub = null;
    let netUpEl = null;
    let netDnEl = null;
    let uptimeEl = null;
    let sseClientsEl = null;
    let systemHostEl = null;
    let systemOsEl = null;
    let systemGoEl = null;

    let counters = { messages: 0, tools: 0 };
    let threatLevel = 0;
    let threatDecayTimer = null;
    let threatBumpDebounce = 0;
    let updateFrame = 0;
    let positionFrame = 0;
    let leftPositionFrame = 0;
    let unsubscribeFns = [];
    let _wired = false;
    let _positionSynced = false;
    let _leftPositionSynced = false;
    let _systemInfoFetched = false;
    let _prevBytesSent = null;
    let _prevBytesRecv = null;
    let _prevNetTime = 0;

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

    function formatBytes(value) {
        var n = Number(value || 0);
        if (!Number.isFinite(n) || n <= 0) return '0 B';
        var units = ['B', 'KB', 'MB', 'GB', 'TB'];
        var size = n;
        var unit = 0;
        while (size >= 1024 && unit < units.length - 1) {
            size /= 1024;
            unit += 1;
        }
        return size.toFixed(unit === 0 ? 0 : 1) + ' ' + units[unit];
    }

    function formatUptime(seconds) {
        var total = Math.max(0, Math.floor(Number(seconds || 0)));
        var days = Math.floor(total / 86400);
        total -= days * 86400;
        var hours = Math.floor(total / 3600);
        total -= hours * 3600;
        var minutes = Math.floor(total / 60);
        var parts = [];
        if (days > 0) parts.push(days + 'd');
        if (hours > 0) parts.push(hours + 'h');
        parts.push(minutes + 'm');
        return parts.join(' ');
    }

    function eqBarColor(pct) {
        if (pct > 70) return 'eq-red';
        if (pct > 40) return 'eq-amber';
        return '';
    }

    function setEqBars(bars, pct) {
        if (!bars || !bars.length) return;
        var colorCls = eqBarColor(pct);
        for (var i = 0; i < bars.length; i++) {
            var bar = bars[i];
            var staggerPct = pct * (0.6 + 0.4 * Math.random());
            if (pct <= 4) staggerPct = pct * 0.5;
            bar.style.transform = 'scaleY(' + (staggerPct / 100).toFixed(3) + ')';
            bar.classList.remove('eq-amber', 'eq-red');
            if (colorCls) bar.classList.add(colorCls);
        }
    }

    function onSystemMetrics(payload) {
        if (!payload) return;
        if (payload.cpu && cpuEqBars) {
            var cpuPct = Number(payload.cpu.usage_percent) || 0;
            setEqBars(cpuEqBars, cpuPct);
            if (cpuEqValue) cpuEqValue.textContent = cpuPct.toFixed(1) + '%';
            if (cpuEqSub) {
                var cores = payload.cpu.cores;
                if (cores == null) cores = '?';
                cpuEqSub.textContent = cores + ' CORES';
            }
        }
        if (payload.memory) {
            var memPct = Number(payload.memory.used_percent) || 0;
            if (ramEqBars) setEqBars(ramEqBars, memPct);
            if (ramEqValue) ramEqValue.textContent = memPct.toFixed(1) + '%';
            if (ramEqSub) {
                ramEqSub.textContent = formatBytes(payload.memory.used) + ' / ' + formatBytes(payload.memory.total);
            }
        }
        if (payload.disk) {
            var diskPct = Number(payload.disk.used_percent) || 0;
            if (diskEqBars) setEqBars(diskEqBars, diskPct);
            if (diskEqValue) diskEqValue.textContent = diskPct.toFixed(1) + '%';
            if (diskEqSub) {
                diskEqSub.textContent = formatBytes(payload.disk.used) + ' / ' + formatBytes(payload.disk.total);
            }
        }
        if (payload.network) {
            if (netUpEl) netUpEl.textContent = formatBytes(payload.network.bytes_sent);
            if (netDnEl) netDnEl.textContent = formatBytes(payload.network.bytes_recv);
        }
        if (uptimeEl && payload.uptime_seconds != null) {
            uptimeEl.textContent = formatUptime(payload.uptime_seconds);
        }
        if (sseClientsEl && payload.sse_clients != null) {
            sseClientsEl.textContent = 'SSE ' + payload.sse_clients;
        }
    }

    function fetchSystemInfo() {
        if (_systemInfoFetched) return;
        _systemInfoFetched = true;
        var xhr = new XMLHttpRequest();
        xhr.open('GET', '/api/system/info', true);
        xhr.onload = function () {
            if (xhr.status !== 200) return;
            try {
                var info = JSON.parse(xhr.responseText);
                if (systemHostEl && info.hostname) systemHostEl.textContent = info.hostname;
                if (systemOsEl) {
                    var osParts = [info.os];
                    if (info.platform_version) osParts.push(info.platform_version);
                    systemOsEl.textContent = osParts.join(' ');
                }
                if (systemGoEl) {
                    systemGoEl.textContent = (info.go_version || '') + ' ' + (info.go_arch || '');
                }
            } catch (_) { /* ignore parse errors */ }
        };
        xhr.send();
    }

    function wireSSE() {
        if (!window.AuraSSE || _wired) return;
        if (typeof window.AuraSSE.on !== 'function') return;
        _wired = true;
        window.AuraSSE.on('agent_action', onAgentAction);
        window.AuraSSE.on('token_update', onTokenUpdate);
        window.AuraSSE.on('_open', onConnectionEvent);
        window.AuraSSE.on('_error', onConnectionEvent);
        window.AuraSSE.on('system_metrics', onSystemMetrics);
        unsubscribeFns.push(function () {
            if (typeof window.AuraSSE.off === 'function') {
                window.AuraSSE.off('agent_action', onAgentAction);
                window.AuraSSE.off('token_update', onTokenUpdate);
                window.AuraSSE.off('_open', onConnectionEvent);
                window.AuraSSE.off('_error', onConnectionEvent);
                window.AuraSSE.off('system_metrics', onSystemMetrics);
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

    function buildLeftHUD() {
        var root = document.createElement('div');
        root.id = HUD_LEFT_ID;
        root.setAttribute('aria-hidden', 'true');
        root.innerHTML = [
            '<div class="hud-panel" data-panel="system-info">',
            '  <div class="hud-panel-title">// SYSTEM</div>',
            '  <div class="hud-node"><span class="hud-node-name">HOST</span><span class="hud-node-status" data-sys-host>-</span></div>',
            '  <div class="hud-node"><span class="hud-node-name">OS</span><span class="hud-node-status" data-sys-os>-</span></div>',
            '  <div class="hud-node"><span class="hud-node-name">GO</span><span class="hud-node-status" data-sys-go>-</span></div>',
            '</div>',
            '<div class="hud-panel" data-panel="cpu">',
            '  <div class="hud-panel-title">// CPU</div>',
            '  <div class="hud-eq"><span class="hud-eq-bar"></span><span class="hud-eq-bar"></span><span class="hud-eq-bar"></span><span class="hud-eq-bar"></span><span class="hud-eq-bar"></span></div>',
            '  <div class="hud-eq-value" data-eq-cpu-value>0.0%</div>',
            '  <div class="hud-eq-sub" data-eq-cpu-sub>? CORES</div>',
            '</div>',
            '<div class="hud-panel" data-panel="ram">',
            '  <div class="hud-panel-title">// RAM</div>',
            '  <div class="hud-eq"><span class="hud-eq-bar"></span><span class="hud-eq-bar"></span><span class="hud-eq-bar"></span><span class="hud-eq-bar"></span><span class="hud-eq-bar"></span></div>',
            '  <div class="hud-eq-value" data-eq-ram-value>0.0%</div>',
            '  <div class="hud-eq-sub" data-eq-ram-sub>0 B / 0 B</div>',
            '</div>',
            '<div class="hud-panel" data-panel="disk">',
            '  <div class="hud-panel-title">// DISK</div>',
            '  <div class="hud-eq"><span class="hud-eq-bar"></span><span class="hud-eq-bar"></span><span class="hud-eq-bar"></span><span class="hud-eq-bar"></span><span class="hud-eq-bar"></span></div>',
            '  <div class="hud-eq-value" data-eq-disk-value>0.0%</div>',
            '  <div class="hud-eq-sub" data-eq-disk-sub>0 B / 0 B</div>',
            '</div>',
            '<div class="hud-panel" data-panel="net">',
            '  <div class="hud-panel-title">// NET</div>',
            '  <div class="hud-counter"><span class="hud-counter-label">UP</span><span class="hud-counter-value" data-net-up>0 B</span></div>',
            '  <div class="hud-counter"><span class="hud-counter-label">DN</span><span class="hud-counter-value" data-net-dn>0 B</span></div>',
            '</div>',
            '<div class="hud-panel" data-panel="runtime">',
            '  <div class="hud-panel-title">// UPTIME</div>',
            '  <div class="hud-counter"><span class="hud-counter-value" data-uptime>0m</span></div>',
            '  <div class="hud-counter"><span class="hud-counter-value" data-sse-clients>SSE 0</span></div>',
            '</div>'
        ].join('');
        return root;
    }

    function captureLeftRefs() {
        systemHostEl = leftHud.querySelector('[data-sys-host]');
        systemOsEl = leftHud.querySelector('[data-sys-os]');
        systemGoEl = leftHud.querySelector('[data-sys-go]');
        cpuEqBars = leftHud.querySelectorAll('[data-panel="cpu"] .hud-eq-bar');
        cpuEqValue = leftHud.querySelector('[data-eq-cpu-value]');
        cpuEqSub = leftHud.querySelector('[data-eq-cpu-sub]');
        ramEqBars = leftHud.querySelectorAll('[data-panel="ram"] .hud-eq-bar');
        ramEqValue = leftHud.querySelector('[data-eq-ram-value]');
        ramEqSub = leftHud.querySelector('[data-eq-ram-sub]');
        diskEqBars = leftHud.querySelectorAll('[data-panel="disk"] .hud-eq-bar');
        diskEqValue = leftHud.querySelector('[data-eq-disk-value]');
        diskEqSub = leftHud.querySelector('[data-eq-disk-sub]');
        netUpEl = leftHud.querySelector('[data-net-up]');
        netDnEl = leftHud.querySelector('[data-net-dn]');
        uptimeEl = leftHud.querySelector('[data-uptime]');
        sseClientsEl = leftHud.querySelector('[data-sse-clients]');
    }

    function ensureLeftHUD() {
        var chatBox = document.getElementById(CHAT_BOX_ID);
        if (!chatBox) return null;
        if (leftHud && leftHud.parentElement !== chatBox) leftHud = null;
        if (!leftHud) {
            leftHud = document.getElementById(HUD_LEFT_ID);
            if (!leftHud) {
                leftHud = buildLeftHUD();
                chatBox.appendChild(leftHud);
            }
            captureLeftRefs();
        }
        return leftHud;
    }

    function destroyLeftHUD() {
        if (leftHud && leftHud.parentElement) leftHud.parentElement.removeChild(leftHud);
        leftHud = null;
        cpuEqBars = null;
        cpuEqValue = null;
        cpuEqSub = null;
        ramEqBars = null;
        ramEqValue = null;
        ramEqSub = null;
        diskEqBars = null;
        diskEqValue = null;
        diskEqSub = null;
        netUpEl = null;
        netDnEl = null;
        uptimeEl = null;
        sseClientsEl = null;
        systemHostEl = null;
        systemOsEl = null;
        systemGoEl = null;
        _systemInfoFetched = false;
        _prevBytesSent = null;
        _prevBytesRecv = null;
        _prevNetTime = 0;
    }

    function syncLeftPosition() {
        if (leftPositionFrame) return;
        var raf = window.requestAnimationFrame || (function (cb) { return window.setTimeout(cb, 16); });
        leftPositionFrame = raf(function () {
            leftPositionFrame = 0;
            if (!leftHud || !leftHud.classList.contains('hud-active')) return;
            var chatBox = document.getElementById(CHAT_BOX_ID);
            if (!chatBox) return;
            var rect = chatBox.getBoundingClientRect();
            leftHud.style.top = rect.top + 'px';
            leftHud.style.left = rect.left + 'px';
            leftHud.style.height = Math.max(0, rect.height) + 'px';
        });
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

    function syncPosition() {
        if (positionFrame) return;
        const raf = window.requestAnimationFrame || ((cb) => window.setTimeout(cb, 16));
        positionFrame = raf(function () {
            positionFrame = 0;
            if (!hud || !hud.classList.contains('hud-active')) return;
            const chatBox = document.getElementById(CHAT_BOX_ID);
            if (!chatBox) return;
            const rect = chatBox.getBoundingClientRect();
            hud.style.top = rect.top + 'px';
            hud.style.right = (window.innerWidth - rect.right) + 'px';
            hud.style.height = Math.max(0, rect.height) + 'px';
        });
    }

    function start() {
        if (!ensureHUD()) return;
        ensureLeftHUD();
        wireSSE();
        hud.classList.add('hud-active');
        if (prefersReducedMotion()) hud.classList.add('hud-reduced-motion');
        if (leftHud) leftHud.classList.add('hud-active');
        scheduleCounterUpdate();
        refreshNodes();
        fetchSystemInfo();
        if (positionFrame) { (window.cancelAnimationFrame || window.clearTimeout)(positionFrame); positionFrame = 0; }
        syncPosition();
        if (!_positionSynced) {
            window.addEventListener('resize', syncPosition);
            window.addEventListener('scroll', syncPosition, { passive: true });
            document.addEventListener('scroll', syncPosition, { passive: true, capture: true });
            _positionSynced = true;
        }
        if (leftPositionFrame) { (window.cancelAnimationFrame || window.clearTimeout)(leftPositionFrame); leftPositionFrame = 0; }
        syncLeftPosition();
        if (!_leftPositionSynced) {
            window.addEventListener('resize', syncLeftPosition);
            window.addEventListener('scroll', syncLeftPosition, { passive: true });
            document.addEventListener('scroll', syncLeftPosition, { passive: true, capture: true });
            _leftPositionSynced = true;
        }
    }

    function stop() {
        unwireSSE();
        if (_positionSynced) {
            window.removeEventListener('resize', syncPosition);
            window.removeEventListener('scroll', syncPosition);
            document.removeEventListener('scroll', syncPosition, { capture: true });
            _positionSynced = false;
        }
        if (_leftPositionSynced) {
            window.removeEventListener('resize', syncLeftPosition);
            window.removeEventListener('scroll', syncLeftPosition);
            document.removeEventListener('scroll', syncLeftPosition, { capture: true });
            _leftPositionSynced = false;
        }
        if (positionFrame) { (window.cancelAnimationFrame || window.clearTimeout)(positionFrame); positionFrame = 0; }
        if (leftPositionFrame) { (window.cancelAnimationFrame || window.clearTimeout)(leftPositionFrame); leftPositionFrame = 0; }
        destroyHUD();
        destroyLeftHUD();
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
            } else if (hud && hud.classList.contains('hud-active')) {
                syncPosition();
                syncLeftPosition();
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
        window.addEventListener('resize', syncPosition);
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