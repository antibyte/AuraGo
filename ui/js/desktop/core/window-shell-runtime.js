    // AUDIT(Task27 / 2026-06-02): All 28 `.style.*` assignments in this file were reviewed.
    // No tokenizable hard-coded colors/sizes/radii were found. Every match is one of:
    //   - Reading current inline state (parseInt on style.left/top) for drag/persist math
    //   - Writing a dynamic computed value (drag clamp, cascade placement, min/max size)
    //   - Writing a CSS custom property via setProperty('--vd-*' / '--dock-index')
    //   - CSS keyword ('none' for resize, '0px'/'100%' for fullscreen layout)
    // No migration needed for Task 28. See reports/virtual_desktop_js_audit_2026-06-01.md
    // for the line-by-line breakdown (lines 442-1001).

    function renderBuiltinWidget(card, widget) {
        const container = card.querySelector('.vd-widget-builtin');
        if (!container) return;
        if (widget.id === 'builtin-analog-clock') {
            renderAnalogClockWidget(container);
        } else if (widget.id === 'builtin-quickchat') {
            renderQuickChatWidget(container);
        } else if (widget.id === 'builtin-weather') {
            renderWeatherWidget(container);
        } else {
            container.innerHTML = `<div class="vd-widget-body">${esc(widget.title)}</div>`;
        }
    }

    function ensureAnalogClockSvg(container, svgSize) {
        let svg = container.querySelector('.vd-analog-clock-svg');
        if (!svg) {
            container.innerHTML = `<svg class="vd-analog-clock-svg" viewBox="0 0 200 200" width="${svgSize}" height="${svgSize}">
            <circle cx="100" cy="100" r="95" fill="none" stroke="var(--vd-border)" stroke-width="2"/>
            <g class="vd-clock-ticks"></g>
            <line class="vd-clock-hour" x1="100" y1="100" x2="100" y2="50" stroke="var(--vd-text)" stroke-width="4" stroke-linecap="round"/>
            <line class="vd-clock-minute" x1="100" y1="100" x2="100" y2="30" stroke="var(--vd-text)" stroke-width="2.5" stroke-linecap="round"/>
            <line class="vd-clock-second" x1="100" y1="100" x2="100" y2="25" stroke="var(--vd-accent)" stroke-width="1.2" stroke-linecap="round"/>
            <circle cx="100" cy="100" r="4" fill="var(--vd-accent)"/>
        </svg>`;
            svg = container.querySelector('.vd-analog-clock-svg');
            const ticksG = container.querySelector('.vd-clock-ticks');
            for (let i = 0; i < 12; i++) {
                const angle = (i * 30) * Math.PI / 180, isMain = i % 3 === 0, r1 = isMain ? 78 : 84, r2 = 90;
                const line = document.createElementNS('http://www.w3.org/2000/svg', 'line');
                [['x1', 100 + r1 * Math.sin(angle)], ['y1', 100 - r1 * Math.cos(angle)], ['x2', 100 + r2 * Math.sin(angle)], ['y2', 100 - r2 * Math.cos(angle)], ['stroke', isMain ? 'var(--vd-text)' : 'var(--vd-muted)'], ['stroke-width', isMain ? '2.5' : '1.2'], ['stroke-linecap', 'round']].forEach(([name, value]) => line.setAttribute(name, value));
                ticksG.appendChild(line);
            }
        }
        if (svg && svg.dataset.clockSize !== String(svgSize)) {
            svg.dataset.clockSize = String(svgSize);
            svg.setAttribute('width', svgSize);
            svg.setAttribute('height', svgSize);
        }
        return svg;
    }

    function updateClockHands(container) {
        const now = new Date();
        const h = now.getHours() % 12, m = now.getMinutes(), s = now.getSeconds();
        const hourAngle = (h + m / 60) * 30, minuteAngle = (m + s / 60) * 6, secondAngle = s * 6;
        const hourHand = container.querySelector('.vd-clock-hour'), minuteHand = container.querySelector('.vd-clock-minute'), secondHand = container.querySelector('.vd-clock-second');
        if (hourHand) hourHand.setAttribute('transform', `rotate(${hourAngle}, 100, 100)`);
        if (minuteHand) minuteHand.setAttribute('transform', `rotate(${minuteAngle}, 100, 100)`);
        if (secondHand) secondHand.setAttribute('transform', `rotate(${secondAngle}, 100, 100)`);
    }

    function renderAnalogClockWidget(container) {
        const size = Math.min(container.parentElement.offsetWidth || 200, container.parentElement.offsetHeight || 200);
        const svgSize = Math.max(80, size - 20);
        ensureAnalogClockSvg(container, svgSize);
        updateClockHands(container);
        if (container._clockTimer) return;
        container._clockTimer = setInterval(() => updateClockHands(container), 1000);
        registerWidgetCleanup(() => {
            clearInterval(container._clockTimer);
            container._clockTimer = 0;
        });
    }

    function renderQuickChatWidget(container) {
        container.innerHTML = `<div class="vd-quickchat vd-quickchat-collapsed">
            <div class="vd-quickchat-response"></div>
            <form class="vd-quickchat-form">
                <input class="vd-quickchat-input" autocomplete="off" placeholder="${esc(t('desktop.chat_placeholder'))}">
                <button class="vd-quickchat-send" type="submit">${iconMarkup('chat', 'S', 'vd-quickchat-send-icon', 14)}</button>
            </form>
        </div>`;
        const input = container.querySelector('.vd-quickchat-input');
        const responseEl = container.querySelector('.vd-quickchat-response');
        const wrapper = container.querySelector('.vd-quickchat');
        container.querySelector('form').addEventListener('submit', async (event) => {
            event.preventDefault();
            if (state.chatBusy) return;
            const message = input.value.trim();
            if (!message) return;
            input.value = '';
            state.chatBusy = true;
            responseEl.textContent = t('desktop.thinking');
            responseEl.classList.add('vd-quickchat-active');
            wrapper.classList.remove('vd-quickchat-collapsed');
            try {
                await sendQuickChatStream(responseEl, message);
            } catch (err) {
                responseEl.textContent = err.message || 'Error';
            } finally {
                state.chatBusy = false;
            }
        });
    }

    function normalizeQuickChatResponseForPetBubble(text) {
        return String(text || '')
            .replace(/```[\s\S]*?```/g, ' ')
            .replace(/\s+/g, ' ')
            .trim();
    }

    function announceQuickChatResponseToPet(text) {
        const message = normalizeQuickChatResponseForPetBubble(text);
        if (!message) return;
        if (window.PetRuntime && typeof window.PetRuntime.announceAgentResponse === 'function') {
            window.PetRuntime.announceAgentResponse(message);
        } else if (window.PetRuntime && typeof window.PetRuntime.say === 'function') {
            window.PetRuntime.say(message, 'info');
        }
    }

    async function sendQuickChatStream(responseEl, message) {
        let streamingContent = '';
        let petAnnouncementText = '';
        let finalized = false;
        return new Promise((resolve, reject) => {
            const ctrl = new AbortController();
            function doFinalize() {
                if (finalized) return;
                finalized = true;
                announceQuickChatResponseToPet(petAnnouncementText || streamingContent);
                resolve();
            }
            function doReject(err) {
                if (finalized) return;
                finalized = true;
                reject(err);
            }
            fetch('/api/desktop/chat/stream', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ message }),
                signal: ctrl.signal
            }).then(response => {
                if (!response.ok) return response.text().then(text => { throw new Error(text || ('HTTP ' + response.status)); });
                const reader = response.body.getReader();
                const decoder = new TextDecoder();
                let buffer = '';
                function processChunk() {
                    reader.read().then(({ done, value }) => {
                        if (done) { doFinalize(); return; }
                        buffer += decoder.decode(value, { stream: true });
                        const lines = buffer.split('\n');
                        buffer = lines.pop();
                        for (const line of lines) {
                            if (line.startsWith('data: ')) {
                                const data = line.slice(6).trim();
                                if (data === '[DONE]') { doFinalize(); reader.cancel().catch(() => {}); return; }
                                try {
                                    const parsed = JSON.parse(data);
                                    const event = parsed.event || parsed.type;
                                    if (event === 'llm_stream_delta') {
                                        const content = parsed.content || '';
                                        if (content) {
                                            streamingContent += content;
                                            responseEl.textContent = streamingContent;
                                            responseEl.classList.add('vd-quickchat-active');
                                        }
                                    } else if (event === 'final_response') {
                                        const text = parsed.detail || parsed.message || '';
                                        if (text.trim()) {
                                            petAnnouncementText = text;
                                        }
                                        if (!streamingContent.trim() && text.trim()) {
                                            streamingContent = text;
                                            responseEl.textContent = text;
                                            responseEl.classList.add('vd-quickchat-active');
                                        }
                                    }
                                } catch (_) {}
                            }
                        }
                        processChunk();
                    }).catch(doReject);
                }
                processChunk();
            }).catch(doReject);
        });
    }

    const WMO_WEATHER = {
        0:  { icon: '\u2600\ufe0f', label: 'Clear sky' },
        1:  { icon: '\ud83c\udf24\ufe0f', label: 'Mainly clear' },
        2:  { icon: '\u26c5', label: 'Partly cloudy' },
        3:  { icon: '\u2601\ufe0f', label: 'Overcast' },
        45: { icon: '\ud83c\udf2b\ufe0f', label: 'Foggy' },
        48: { icon: '\ud83c\udf2b\ufe0f', label: 'Rime fog' },
        51: { icon: '\ud83c\udf26\ufe0f', label: 'Light drizzle' },
        53: { icon: '\ud83c\udf26\ufe0f', label: 'Moderate drizzle' },
        55: { icon: '\ud83c\udf27\ufe0f', label: 'Dense drizzle' },
        56: { icon: '\ud83c\udf27\ufe0f', label: 'Freezing drizzle' },
        57: { icon: '\ud83c\udf27\ufe0f', label: 'Dense freezing drizzle' },
        61: { icon: '\ud83c\udf27\ufe0f', label: 'Slight rain' },
        63: { icon: '\ud83c\udf27\ufe0f', label: 'Moderate rain' },
        65: { icon: '\ud83c\udf27\ufe0f', label: 'Heavy rain' },
        66: { icon: '\ud83c\udf27\ufe0f', label: 'Freezing rain' },
        67: { icon: '\ud83c\udf27\ufe0f', label: 'Heavy freezing rain' },
        71: { icon: '\ud83c\udf28\ufe0f', label: 'Slight snow' },
        73: { icon: '\ud83c\udf28\ufe0f', label: 'Moderate snow' },
        75: { icon: '\u2744\ufe0f', label: 'Heavy snow' },
        77: { icon: '\u2744\ufe0f', label: 'Snow grains' },
        80: { icon: '\ud83c\udf26\ufe0f', label: 'Slight showers' },
        81: { icon: '\ud83c\udf27\ufe0f', label: 'Moderate showers' },
        82: { icon: '\ud83c\udf27\ufe0f', label: 'Heavy showers' },
        85: { icon: '\ud83c\udf28\ufe0f', label: 'Slight snow showers' },
        86: { icon: '\ud83c\udf28\ufe0f', label: 'Heavy snow showers' },
        95: { icon: '\u26c8\ufe0f', label: 'Thunderstorm' },
        96: { icon: '\u26c8\ufe0f', label: 'Thunderstorm + hail' },
        99: { icon: '\u26c8\ufe0f', label: 'Thunderstorm + heavy hail' }
    };

    function wmoInfo(code) {
        return WMO_WEATHER[code] || { icon: '\ud83c\udf21\ufe0f', label: 'Unknown' };
    }

    async function renderWeatherWidget(container) {
        const WEATHER_REFRESH_MS = 30 * 60 * 1000;
        const STORAGE_KEY = 'vd-weather-location';

        container.innerHTML = `<div class="vd-weather">
            <div class="vd-weather-location-row">
                <div class="vd-weather-location-name">${esc('\u2014')}</div>
                <button class="vd-weather-geo-btn" type="button" title="Use my location">\ud83d\udccd</button>
                <button class="vd-weather-edit-btn" type="button" title="Change location">\u270f\ufe0f</button>
            </div>
            <div class="vd-weather-search-row" hidden>
                <input class="vd-weather-search-input" type="text" placeholder="Search city\u2026" autocomplete="off" spellcheck="false" inputmode="search" enterkeyhint="search" autocapitalize="off">
                <button class="vd-weather-search-btn" type="button">Set</button>
            </div>
            <div class="vd-weather-suggestions" hidden></div>
            <div class="vd-weather-main">
                <div class="vd-weather-loading">Loading weather\u2026</div>
            </div>
            <div class="vd-weather-forecast"></div>
            <div class="vd-weather-updated"></div>
        </div>`;

        const $ = sel => container.querySelector(sel);
        const root = $('.vd-weather');
        const locationName = $('.vd-weather-location-name');
        const searchRow = $('.vd-weather-search-row');
        const searchInput = $('.vd-weather-search-input');
        const searchBtn = $('.vd-weather-search-btn');
        const suggestions = $('.vd-weather-suggestions');
        const mainArea = $('.vd-weather-main');
        const forecastArea = $('.vd-weather-forecast');
        const editBtn = $('.vd-weather-edit-btn');
        const geoBtn = $('.vd-weather-geo-btn');
        const updatedEl = $('.vd-weather-updated');

        let location = null;
        let refreshTimer = null;
        let searchDebounce = null;

        function saveLocation(loc) {
            location = loc;
            try { localStorage.setItem(STORAGE_KEY, JSON.stringify(loc)); } catch (_) {}
            locationName.textContent = loc.name + (loc.country && loc.country !== loc.name ? ', ' + loc.country : '');
        }

        async function fetchWeather() {
            if (!location) return;
            const url = 'https://api.open-meteo.com/v1/forecast?latitude=' +
                encodeURIComponent(location.lat) + '&longitude=' + encodeURIComponent(location.lon) +
                '&current=temperature_2m,relative_humidity_2m,apparent_temperature,weather_code,wind_speed_10m' +
                '&daily=weather_code,temperature_2m_max,temperature_2m_min&timezone=auto&forecast_days=6';
            try {
                const res = await fetch(url);
                if (!res.ok) throw new Error('HTTP ' + res.status);
                const data = await res.json();
                renderWeatherData(data);
            } catch (err) {
                mainArea.innerHTML = '<div class="vd-weather-error">Could not load weather: ' + esc(err.message || 'network error') + '</div>';
            }
        }

        function renderWeatherData(data) {
            const c = data.current;
            const d = data.daily;
            if (!c || !d) return;
            const wmo = wmoInfo(c.weather_code);
            mainArea.innerHTML = '<div class="vd-weather-icon">' + wmo.icon + '</div>' +
                '<div class="vd-weather-info">' +
                    '<div class="vd-weather-temp">' + Math.round(c.temperature_2m) + '\u00b0</div>' +
                    '<div class="vd-weather-condition">' + esc(wmo.label) + '</div>' +
                    '<div class="vd-weather-meta">' +
                        '<div class="vd-weather-meta-item"><span class="vd-weather-meta-icon">\ud83c\udf21\ufe0f</span>' + Math.round(c.apparent_temperature) + '\u00b0</div>' +
                        '<div class="vd-weather-meta-item"><span class="vd-weather-meta-icon">\ud83d\udca7</span>' + Math.round(c.relative_humidity_2m) + '%</div>' +
                        '<div class="vd-weather-meta-item"><span class="vd-weather-meta-icon">\ud83d\udca8</span>' + Math.round(c.wind_speed_10m) + ' km/h</div>' +
                    '</div>' +
                '</div>';
            const days = (d.time || []).slice(0, 6);
            forecastArea.innerHTML = days.map((date, i) => {
                const dt = new Date(date + 'T12:00:00');
                const dayName = dt.toLocaleDateString(undefined, { weekday: 'short' }).slice(0, 2).toUpperCase();
                const dwm = wmoInfo(d.weather_code[i]);
                return '<div class="vd-weather-day' + (i === 0 ? ' is-today' : '') + '">' +
                    '<div class="vd-weather-day-name">' + esc(dayName) + '</div>' +
                    '<div class="vd-weather-day-icon">' + dwm.icon + '</div>' +
                    '<div class="vd-weather-day-temps">' +
                        '<div class="vd-weather-day-temp-high">' + Math.round(d.temperature_2m_max[i]) + '\u00b0</div>' +
                        '<div class="vd-weather-day-temp-low">' + Math.round(d.temperature_2m_min[i]) + '\u00b0</div>' +
                    '</div>' +
                '</div>';
            }).join('');
            updatedEl.textContent = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
            root.classList.add('is-ready');
        }

        async function searchLocations(query) {
            if (!query || query.length < 2) {
                suggestions.hidden = true;
                suggestions.innerHTML = '';
                return;
            }
            try {
                const url = 'https://geocoding-api.open-meteo.com/v1/search?name=' +
                    encodeURIComponent(query) + '&count=6&language=en&format=json';
                const res = await fetch(url);
                const data = await res.json();
                const results = (data.results || []).slice(0, 6);
                suggestions.innerHTML = results.map(r =>
                    '<button class="vd-weather-suggestion" type="button" data-lat="' + esc(String(r.latitude)) + '" data-lon="' + esc(String(r.longitude)) + '" data-name="' + esc(r.name) + '" data-country="' + esc(r.country || '') + '">' +
                        esc(r.name) + (r.admin1 ? ', ' + esc(r.admin1) : '') + (r.country ? ', ' + esc(r.country) : '') +
                    '</button>'
                ).join('');
                suggestions.hidden = results.length === 0;
            } catch (_) {
                suggestions.hidden = true;
            }
        }

        async function geolocate() {
            if (!navigator.geolocation) {
                showDesktopNotification({ title: t('desktop.notification'), message: 'Geolocation not available' });
                return;
            }
            geoBtn.disabled = true;
            navigator.geolocation.getCurrentPosition(async (pos) => {
                geoBtn.disabled = false;
                const lat = pos.coords.latitude.toFixed(3);
                const lon = pos.coords.longitude.toFixed(3);
                saveLocation({ lat: parseFloat(lat), lon: parseFloat(lon), name: lat + ', ' + lon, country: '' });
                fetchWeather();
            }, () => {
                geoBtn.disabled = false;
            }, { timeout: 10000, maximumAge: 300000 });
        }

        root.addEventListener('click', function (event) {
            const btn = event.target.closest('button');
            if (!btn) return;
            if (btn.classList.contains('vd-weather-edit-btn')) {
                const opening = searchRow.hidden;
                searchRow.hidden = !opening;
                if (opening) {
                    searchInput.value = '';
                    setTimeout(function () { searchInput.focus(); }, 10);
                }
                suggestions.hidden = true;
            } else if (btn.classList.contains('vd-weather-geo-btn')) {
                geolocate();
            } else if (btn.classList.contains('vd-weather-search-btn')) {
                searchLocations(searchInput.value.trim());
            } else if (btn.classList.contains('vd-weather-suggestion')) {
                saveLocation({
                    lat: parseFloat(btn.dataset.lat),
                    lon: parseFloat(btn.dataset.lon),
                    name: btn.dataset.name,
                    country: btn.dataset.country
                });
                searchRow.hidden = true;
                suggestions.hidden = true;
                searchInput.value = '';
                fetchWeather();
            }
        });

        searchInput.addEventListener('input', () => {
            clearTimeout(searchDebounce);
            searchDebounce = setTimeout(() => searchLocations(searchInput.value.trim()), 300);
        });

        searchBtn.addEventListener('click', () => searchLocations(searchInput.value.trim()));

        searchInput.addEventListener('keydown', event => {
            if (event.key === 'Enter') {
                event.preventDefault();
                searchLocations(searchInput.value.trim());
            } else if (event.key === 'Escape') {
                searchRow.hidden = true;
                suggestions.hidden = true;
            }
        });

        try {
            const saved = localStorage.getItem(STORAGE_KEY);
            if (saved) location = JSON.parse(saved);
        } catch (_) {}

        if (!location) {
            location = { lat: 52.52, lon: 13.41, name: 'Berlin', country: 'Germany' };
            try { localStorage.setItem(STORAGE_KEY, JSON.stringify(location)); } catch (_) {}
        }

        locationName.textContent = location.name + (location.country && location.country !== location.name ? ', ' + location.country : '');
        fetchWeather();

        refreshTimer = setInterval(fetchWeather, WEATHER_REFRESH_MS);
        registerWidgetCleanup(() => {
            if (refreshTimer) clearInterval(refreshTimer);
            clearTimeout(searchDebounce);
        });
    }

    function defaultWidgetBounds(index) {
        const workspace = $('vd-workspace');
        const width = 320;
        const height = 56;
        const x = Math.max(18, ((workspace && workspace.clientWidth) || window.innerWidth) - width - 18);
        return { x, y: 18 + index * (height + 12), w: width, h: height };
    }

    function widgetBounds(widget, index) {
        const fallback = defaultWidgetBounds(index);
        const w = Number(widget.w || widget.W || 0);
        const h = Number(widget.h || widget.H || 0);
        return {
            x: Number(widget.x || widget.X || fallback.x) || fallback.x,
            y: Number(widget.y || widget.Y || fallback.y) || fallback.y,
            w: w > 16 ? w : fallback.w,
            h: h > 16 ? h : fallback.h
        };
    }

    function isWidgetInteractiveTarget(target) { return !!(target && target.closest && target.closest('button, input, textarea, select, option, a[href], [contenteditable="true"], [contenteditable=""]')); }

    function wireDraggableWidget(card, widget) {
        const handle = card; let drag = null;
        function finishDrag(event) {
            if (!drag) return;
            if (event && event.pointerId != null && event.pointerId !== drag.pointerId) return;
            if (drag.holdTimer) window.clearTimeout(drag.holdTimer);
            if (event && handle.hasPointerCapture && handle.hasPointerCapture(drag.pointerId)) {
                handle.releasePointerCapture(drag.pointerId);
            }
            card.classList.remove('vd-dragging');
            document.body.classList.remove('vd-touch-drag-active');
            if (drag.moved) {
                persistWidgetBounds(card._widgetData || widget, card);
                if (event) event.preventDefault();
            }
            drag = null;
        }
        handle.addEventListener('pointerdown', event => {
            if (event.button !== 0 || isWidgetInteractiveTarget(event.target)) return;
            const touchDrag = isTouchLikePointer(event);
            drag = {
                pointerId: event.pointerId,
                x: event.clientX,
                y: event.clientY,
                left: parseInt(card.style.left, 10) || 0,
                top: parseInt(card.style.top, 10) || 0,
                moved: false,
                ready: !touchDrag,
                touchDrag,
                holdTimer: 0
            };
            if (touchDrag) {
                drag.holdTimer = window.setTimeout(() => {
                    if (drag) drag.ready = true;
                }, TOUCH_DRAG_HOLD_MS);
            }
            handle.setPointerCapture(event.pointerId);
        });
        handle.addEventListener('pointermove', event => {
            if (!drag) return;
            const dx = event.clientX - drag.x;
            const dy = event.clientY - drag.y;
            if (card.__vdLongPressTriggered) return;
            if (drag.touchDrag && !drag.ready) {
                if (Math.hypot(dx, dy) > LONG_PRESS_MOVE_TOLERANCE) finishDrag(event);
                return;
            }
            if (!drag.moved && Math.hypot(dx, dy) < DRAG_THRESHOLD) return;
            drag.moved = true;
            card.classList.add('vd-dragging');
            if (drag.touchDrag) document.body.classList.add('vd-touch-drag-active');
            const pos = clampToWorkspace(drag.left + dx, drag.top + dy, card.offsetWidth, card.offsetHeight);
            card.style.left = pos.x + 'px';
            card.style.top = pos.y + 'px';
        });
        handle.addEventListener('pointerup', finishDrag);
        handle.addEventListener('pointercancel', finishDrag);
    }

    async function persistWidgetBounds(widget, card) {
        const updated = Object.assign({}, widget, {
            x: parseInt(card.style.left, 10) || 0,
            y: parseInt(card.style.top, 10) || 0,
            w: Math.round(card.offsetWidth),
            h: Math.round(card.offsetHeight)
        });
        if (widgetShouldAutoSize(widget)) {
            delete updated.w;
            delete updated.h;
        }
        try {
            await api('/api/desktop/widgets', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(updated)
            });
            await loadBootstrap();
        } catch (err) {
            showDesktopNotification({ title: t('desktop.notification'), message: err.message });
        }
    }

    async function renderWidgetFrame(card, widget) {
        card.innerHTML = `<div class="vd-widget-body">${esc(t('desktop.loading'))}</div>`;
        const path = widgetFramePath(widget);
        try {
            const src = await desktopEmbedURL(path, { widget_id: widget.id });
            await ensureDesktopEmbedHasContent(src);
            card.replaceChildren(makeSandboxedFrame(src, widget.app_id, widget.id, '', 'vd-widget-frame', widget.title || widget.id));
            scheduleWidgetAutoSize(card.closest('.vd-widget'), widget);
        } catch (err) {
            card.innerHTML = `<div class="vd-widget-body">${esc(err.message)}</div>`;
            scheduleWidgetAutoSize(card.closest('.vd-widget'), widget);
        }
    }

    function widgetFramePath(widget) {
        return widget.app_id
            ? 'Apps/' + widget.app_id + '/' + widget.entry
            : 'Widgets/' + widget.entry;
    }

    function appIsBroken(app) {
        return !!(app && (app.health === 'broken' || app.health_reason));
    }

    function brokenAppLabel(app) {
        return appIsBroken(app) ? `<span class="vd-app-health" title="${esc(t('desktop.app_missing_entry'))}">!</span>` : '';
    }

    function renderStartApps() {
        const query = state.startQuery.trim().toLowerCase();
        const allApps = startMenuApps();
        const apps = allApps.filter(app => !query || appName(app).toLowerCase().includes(query));
        const recentKey = 'aurago.desktop.recentApps.v1';
        const recentIds = readJSONStorage(recentKey, []).slice(0, 5);
        const recentApps = query ? [] : recentIds.map(id => allApps.find(app => app.id === id)).filter(Boolean);
        const nonRecentApps = apps.filter(app => !recentApps.some(r => r.id === app.id));

        let html = '';
        if (recentApps.length > 0) {
            html += `<div class="vd-start-recent-label">${esc(t('desktop.recent_apps'))}</div>`;
            html += recentApps.map(app => `<button class="vd-start-item vd-start-recent-item" type="button" data-app-id="${esc(app.id)}">
                ${iconMarkup(iconForApp(app), iconGlyph(app), 'vd-sprite-start-item', 30)}
                <span>${esc(appName(app))}${brokenAppLabel(app)}</span>
            </button>`).join('');
        }
        html += nonRecentApps.map(app => `<button class="vd-start-item" type="button" data-app-id="${esc(app.id)}">
            ${iconMarkup(iconForApp(app), iconGlyph(app), 'vd-sprite-start-item', 30)}
            <span>${esc(appName(app))}${brokenAppLabel(app)}</span>
        </button>`).join('');

        $('vd-start-apps').innerHTML = html;
        $('vd-start-apps').querySelectorAll('[data-app-id]').forEach(btn => {
            btn.addEventListener('click', () => {
                closeStartMenu();
                const appId = btn.dataset.appId;
                const recent = readJSONStorage(recentKey, []);
                const updated = [appId, ...recent.filter(id => id !== appId)].slice(0, 5);
                writeJSONStorage(recentKey, updated);
                openApp(appId);
            });
            btn.addEventListener('contextmenu', event => showStartAppContextMenu(event, btn.dataset.appId));
        });
    }

    function renderTaskbar() {
        const host = $('vd-taskbar-apps');
        if (!host) return;
        host.classList.toggle('vd-dock', isFruityTheme());
        if (isFruityTheme()) {
            renderFruityDock();
            scheduleFruityDockOcclusionCheck();
            return;
        }
        document.body.classList.remove('fruity-dock-collapsed');
        state.fruityDockFootprint = null;
        host.classList.remove('vd-dock-overflowing', 'vd-dock-at-start', 'vd-dock-at-end');
        if (host._fruityDockResizeObserver) {
            host._fruityDockResizeObserver.disconnect();
            host._fruityDockResizeObserver = null;
        }
        renderStandardTaskbar();
        ensureMobileTaskbarSwipe();
    }

    function taskbarButtonHTML(win) {
        const app = appById(win.appId);
        const icon = iconMarkup(win.icon || iconForApp(app), win.iconGlyph || iconGlyph(app), 'vd-task-icon', 16);
        return `${icon}<span class="vd-task-label">${esc(win.title)}</span>`;
    }

    function updateTaskbarButton(btn, win, index) {
        btn.className = 'vd-task-button';
        btn.classList.toggle('active', win.id === state.activeWindowId);
        btn.dataset.windowId = win.id;
        btn.style.setProperty('--dock-index', index);
        const nextHTML = taskbarButtonHTML(win);
        if (btn.dataset.renderedHtml !== nextHTML) {
            btn.innerHTML = nextHTML;
            btn.dataset.renderedHtml = nextHTML;
        }
    }

    function bindTaskbarButton(btn) {
        if (btn.getAttribute('data-taskbar-bound') === 'true') return;
        btn.setAttribute('data-taskbar-bound', 'true');
        btn.addEventListener('click', () => focusWindow(btn.dataset.windowId));
        btn.addEventListener('contextmenu', event => showWindowContextMenu(event, btn.dataset.windowId));
        wireLongPress(btn, event => showWindowContextMenu(event, btn.dataset.windowId));
    }

    function reconcileStandardTaskbar() {
        const host = $('vd-taskbar-apps');
        if (!host) return;
        if (host.querySelector('[data-fruity-dock-track]')) host.replaceChildren();
        const seenWindowIds = new Set();
        [...state.windows.values()].forEach((win, index) => {
            seenWindowIds.add(win.id);
            let btn = host.querySelector(`[data-window-id="${cssSel(win.id)}"]`);
            if (!btn) {
                btn = document.createElement('button');
                btn.type = 'button';
                host.insertBefore(btn, host.children[index] || null);
                bindTaskbarButton(btn);
            } else if (btn !== host.children[index]) {
                host.insertBefore(btn, host.children[index] || null);
            }
            updateTaskbarButton(btn, win, index);
        });
        host.querySelectorAll('[data-window-id]').forEach(btn => {
            if (!seenWindowIds.has(btn.dataset.windowId)) btn.remove();
        });
    }

    function renderStandardTaskbar() { reconcileStandardTaskbar(); }

    function ensureFruityDockShell(host) {
        let track = host && host.querySelector('[data-fruity-dock-track]');
        if (track) return track;
        host.replaceChildren();
        host.innerHTML = `<button type="button" class="vd-dock-orb" data-fruity-dock-orb title="${esc(t('desktop.start_menu'))}">
            ${iconMarkup('home', 'A', 'vd-dock-orb-icon', 34)}
        </button>
        <button type="button" class="vd-dock-scroll-button vd-dock-scroll-button-left" data-fruity-dock-scroll-button="left" aria-label="${esc(t('desktop.dock_scroll_left'))}">
            ${iconMarkup('arrow-left', '<', 'vd-dock-scroll-icon', 18)}
        </button>
        <div class="vd-dock-scroll" data-fruity-dock-scroll-region>
            <div class="vd-dock-track" data-fruity-dock-track></div>
        </div>
        <button type="button" class="vd-dock-scroll-button vd-dock-scroll-button-right" data-fruity-dock-scroll-button="right" aria-label="${esc(t('desktop.dock_scroll_right'))}">
            ${iconMarkup('arrow-right', '>', 'vd-dock-scroll-icon', 18)}
        </button>`;
        const orb = host.querySelector('[data-fruity-dock-orb]');
        if (orb) orb.addEventListener('click', event => { event.stopPropagation(); toggleStartMenu(); });
        wireFruityDockScroll(host);
        return host.querySelector('[data-fruity-dock-track]');
    }

    function dockButtonHTML(app) {
        return `${iconMarkup(iconForApp(app), iconGlyph(app), 'vd-dock-icon', 34)}<span class="vd-dock-label">${esc(appName(app))}${brokenAppLabel(app)}</span>`;
    }

    function updateDockButton(btn, app, index, runningWindows) {
        const running = runningWindows.some(win => win.appId === app.id);
        const active = runningWindows.some(win => win.appId === app.id && win.id === state.activeWindowId);
        const broken = appIsBroken(app);
        btn.type = 'button';
        btn.className = 'vd-dock-button';
        btn.classList.toggle('running', running);
        btn.classList.toggle('active', active);
        btn.classList.toggle('broken', broken);
        btn.dataset.appId = app.id;
        btn.title = broken ? `${appName(app)} - ${t('desktop.app_missing_entry')}` : appName(app);
        btn.style.setProperty('--dock-index', index);
        const nextHTML = dockButtonHTML(app);
        if (btn.dataset.renderedHtml !== nextHTML) {
            btn.innerHTML = nextHTML;
            btn.dataset.renderedHtml = nextHTML;
        }
    }

    function bindDockButton(btn) {
        if (btn.getAttribute('data-dock-bound') === 'true') return;
        btn.setAttribute('data-dock-bound', 'true');
        btn.addEventListener('click', () => {
            const existing = [...state.windows.values()].find(win => win.appId === btn.dataset.appId);
            if (existing) focusWindow(existing.id);
            else openApp(btn.dataset.appId);
        });
        btn.addEventListener('contextmenu', event => showStartAppContextMenu(event, btn.dataset.appId));
        wireLongPress(btn, event => showStartAppContextMenu(event, btn.dataset.appId));
    }

    function reconcileFruityDock() {
        const host = $('vd-taskbar-apps');
        const runningWindows = [...state.windows.values()];
        const track = ensureFruityDockShell(host);
        if (!track) return;
        const seenDockAppIds = new Set();
        dockApps().forEach((app, index) => {
            seenDockAppIds.add(app.id);
            let btn = track.querySelector(`[data-app-id="${cssSel(app.id)}"]`);
            if (!btn) {
                btn = document.createElement('button');
                track.insertBefore(btn, track.children[index] || null);
                bindDockButton(btn);
            } else if (btn !== track.children[index]) {
                track.insertBefore(btn, track.children[index] || null);
            }
            updateDockButton(btn, app, index, runningWindows);
        });
        track.querySelectorAll('[data-app-id]').forEach(btn => {
            if (!seenDockAppIds.has(btn.dataset.appId)) btn.remove();
        });
        updateFruityDockScrollControls(host);
    }

    function windowTitle(appId) {
        if (appId === 'system-info') return t('desktop.system_info_title');
        if (appId === 'virtual-computers') return t('desktop.virtual_computers_title');
        if (appId === 'people') return t('desktop.app_people');
        const app = allApps().find(item => item.id === appId);
        return app ? appName(app) : appId;
    }

    function appWindowSize(appId) {
        const presets = {
            files: { width: 920, height: 600 },
            writer: { width: 960, height: 700 },
            sheets: { width: 1040, height: 690 },
            calculator: { width: 380, height: 640 },
            todo: { width: 900, height: 600 },
            'music-player': { width: 430, height: 260 },
            radio: { width: 960, height: 680 },
            openscad: { width: 1120, height: 720 },
            teevee: { width: 1120, height: 720 },
            gallery: { width: 1040, height: 700 },
            calendar: { width: 950, height: 650 },
            'quick-connect': { width: 960, height: 680 },
            'virtual-computers': { width: 980, height: 680 },
            'code-studio': { width: 1280, height: 850 },
            launchpad: { width: 1100, height: 700 },
            'system-info': { width: 800, height: 600 },
            'agent-chat': { width: 800, height: 620 },
            'looper': { width: 900, height: 750 },
            camera: { width: 720, height: 600 },
            viewer: { width: 900, height: 700 },
            'viewer-3d': { width: 900, height: 700 },
            pixel: { width: 1100, height: 750 },
            'galaxa-deluxe': { width: 600, height: 800 },
            chess: { width: 980, height: 680 },
            nasscad: { width: 1280, height: 850 },
            people: { width: 1020, height: 700 },
            'mission-control': { width: 1100, height: 750 },
            'pet-picker': { width: 760, height: 620 }
        };
        if (presets[appId]) return presets[appId];
        return defaultWindowSize();
    }

    function shouldUseMobileWideWindow(appId) { return !!{ files: true, writer: true, sheets: true, todo: true, radio: true, openscad: true, teevee: true, gallery: true, calendar: true, 'quick-connect': true, 'virtual-computers': true, 'code-studio': true, launchpad: true, looper: true, viewer: true, 'viewer-3d': true, chess: true, nasscad: true, 'mission-control': true }[appId]; }

    function appWindowMinSize(appId) {
        const mins = { 'system-info': { width: 560, height: 460 }, 'virtual-computers': { width: 640, height: 480 }, calculator: { width: 280, height: 420 }, gallery: { width: 640, height: 480 }, pixel: { width: 700, height: 500 }, chess: { width: 720, height: 520 } };
        return mins[appId] || { width: WINDOW_MIN_W, height: WINDOW_MIN_H };
    }

    function shouldOpenMaximized(app) {
        return !!(app && app.metadata && (app.metadata.open_maximized === 'true' || app.metadata.store_app_id === 'quakejs-rootless'));
    }

    function clampWindowSize(size) {
        const workspace = $('vd-workspace') || document.body;
        const workspaceRect = workspace.getBoundingClientRect();
        const margin = 16;
        const taskbar = document.querySelector('.vd-taskbar');
        const taskbarReserve = (!isFruityTheme() && taskbar) ? taskbar.offsetHeight : 0;
        return {
            width: Math.min(size.width, Math.max(1, workspaceRect.width - margin * 2)),
            height: Math.min(size.height, Math.max(1, workspaceRect.height - taskbarReserve - margin * 2))
        };
    }

    function nextWindowPosition(size) {
        const workspace = $('vd-workspace') || document.body;
        const workspaceRect = workspace.getBoundingClientRect();
        const margin = 16;
        const topStart = 72;
        const stepX = 28;
        const stepY = 24;
        const taskbar = document.querySelector('.vd-taskbar');
        const taskbarReserve = (!isFruityTheme() && taskbar) ? taskbar.offsetHeight : 0;
        const maxLeft = Math.max(margin, workspaceRect.width - size.width - margin);
        const maxTop = Math.max(margin, workspaceRect.height - taskbarReserve - size.height - margin);
        const slotsX = Math.max(1, Math.floor((maxLeft - margin) / stepX) + 1);
        const slotsY = Math.max(1, Math.floor((maxTop - topStart) / stepY) + 1);
        const index = state.windows.size;
        const left = margin + (index % slotsX) * stepX;
        const top = topStart + (Math.floor(index / slotsX) % slotsY) * stepY;
        return {
            left: Math.min(maxLeft, Math.max(margin, left)),
            top: Math.min(maxTop, Math.max(margin, top))
        };
    }

    function normalizeDesktopPath(path) {
        return String(path || '').replace(/\\/g, '/').replace(/\/+/g, '/').replace(/^\.\//, '').trim();
    }

    function updateWindowContext(windowId, patch) {
        const win = state.windows.get(windowId);
        if (!win) return;
        win.context = Object.assign({}, win.context || {}, patch || {});
        if (win.context.path != null) win.context.path = normalizeDesktopPath(win.context.path);
    }

    function findExistingAppWindow(appId, context) {
        const dead = [];
        state.windows.forEach((win, id) => {
            if (!win || !win.element || !win.element.isConnected) {
                dead.push(id);
            }
        });
        dead.forEach(id => {
            clearWindowMenus(id);
            disposeAppWindow(state.windows.get(id));
            state.windows.delete(id);
            if (state.activeWindowId === id) state.activeWindowId = '';
        });
        return [...state.windows.values()].find(win => {
            if (win.appId !== appId) return false;
            if ((appId === 'editor' || appId === 'writer' || appId === 'sheets') && context && context.path != null) {
                const requestedPath = normalizeDesktopPath(context.path);
                return win.context && normalizeDesktopPath(win.context.path) === requestedPath;
            }
            return appId !== 'editor' && appId !== 'writer' && appId !== 'sheets';
        });
    }

    function isStandaloneWidgetPath(path) {
        const normalized = normalizeDesktopPath(path);
        return /^Widgets\/[^/]+\.html$/i.test(normalized);
    }

    function standaloneWidgetIDFromPath(path) {
        const file = normalizeDesktopPath(path).split('/').pop() || '';
        return file.replace(/\.html$/i, '').toLowerCase().replace(/[^a-z0-9_-]+/g, '_').replace(/^[_-]+|[_-]+$/g, '');
    }

    function standaloneWidgetById(widgetId) {
        const boot = state.bootstrap || {};
        const widgets = [...(boot.widgets || []), ...(boot.all_widgets || [])];
        return widgets.find(item => item && item.id === widgetId);
    }

    function findExistingStandaloneWidgetWindow(path, widgetId) {
        const normalizedPath = normalizeDesktopPath(path);
        return [...state.windows.values()].find(win => {
            const context = win.context || {};
            return context.standaloneWidget === true &&
                normalizeDesktopPath(context.path) === normalizedPath &&
                (!widgetId || context.widgetId === widgetId);
        });
    }

    function openStandaloneWidget(path, widgetId, options) {
        const normalizedPath = normalizeDesktopPath(path);
        if (!isStandaloneWidgetPath(normalizedPath)) {
            showDesktopNotification({ title: t('desktop.notification'), message: t('desktop.app_missing') });
            return;
        }
        const safeWidgetId = widgetId || standaloneWidgetIDFromPath(normalizedPath);
        const existing = findExistingStandaloneWidgetWindow(normalizedPath, safeWidgetId);
        if (existing) {
            focusWindow(existing.id);
            return;
        }
        const widget = standaloneWidgetById(safeWidgetId) || {};
        const title = (options && options.title) || widget.title || windowTitle(safeWidgetId);
        const icon = (options && options.icon) || widget.icon || 'apps';
        const id = 'w-widget-' + safeWidgetId + '-' + Date.now();
        const win = document.createElement('section');
        win.className = 'vd-window';
        win.dataset.windowId = id;
        const size = clampWindowSize({ width: 900, height: 650 });
        const position = nextWindowPosition(size);
        win.style.left = position.left + 'px';
        win.style.top = position.top + 'px';
        win.style.width = size.width + 'px';
        win.style.height = size.height + 'px';
        win.style.minWidth = Math.min(WINDOW_MIN_W, size.width) + 'px';
        win.style.minHeight = Math.min(WINDOW_MIN_H, size.height) + 'px';
        win.style.zIndex = String(++state.z);
        win.innerHTML = `<header class="vd-window-titlebar">
            <div class="vd-window-title-group">
                <span class="vd-window-header-icon-wrap">${iconMarkup(icon, '', 'vd-window-header-icon', 16)}</span>
                <div class="vd-window-title">${esc(title)}</div>
                <div class="vd-window-subtitle"></div>
            </div>
            <div class="vd-window-actions">
                ${aiButtonMarkup('widget:' + safeWidgetId)}<button class="vd-window-button" type="button" data-action="minimize" title="${esc(t('desktop.minimize'))}" aria-label="${esc(t('desktop.minimize'))}"></button>
                <button class="vd-window-button" type="button" data-action="maximize" title="${esc(t('desktop.maximize'))}" aria-label="${esc(t('desktop.maximize'))}"></button>
                <button class="vd-window-button" type="button" data-action="close" title="${esc(t('desktop.close'))}" aria-label="${esc(t('desktop.close'))}"></button>
            </div>
        </header>
        <div class="vd-window-content" data-window-content><div class="vd-empty">${esc(t('desktop.loading'))}</div></div>
        ${resizeHandleMarkup()}`;
        $('vd-window-layer').appendChild(win);
        const context = { path: normalizedPath, widgetId: safeWidgetId, standaloneWidget: true };
        state.windows.set(id, { id, appId: 'widget:' + safeWidgetId, title, element: win, maximized: false, restoreBounds: null, context, icon, iconGlyph: '' });
        wireWindow(win, id);
        animateThen(win, 'vd-window-opening', 240);
        focusWindow(id);
        renderStandaloneWidgetContent(id, normalizedPath, safeWidgetId, title);
        renderTaskbar();
    }

    function renderStandaloneWidgetContent(id, path, widgetId, title) {
        const host = contentEl(id);
        if (!host) return;
        host.innerHTML = `<div class="vd-empty">${esc(t('desktop.loading'))}</div>`;
        desktopEmbedURL(path, { widget_id: widgetId })
            .then(async src => {
                await ensureDesktopEmbedHasContent(src);
                if (!contentEl(id)) return;
                host.replaceChildren(makeSandboxedFrame(src, '', widgetId, id, 'vd-generated-frame', title));
            })
            .catch(err => {
                if (!contentEl(id)) return;
                host.innerHTML = `<div class="vd-empty">${esc(err.message)}</div>`;
            });
    }

    function openApp(appId, context) {
        if (context && context.path && isStandaloneWidgetPath(context.path) && !appById(appId)) {
            openStandaloneWidget(context.path, context.widgetId || appId, context);
            return;
        }
        if (appId === 'music-player') {
            launchStandaloneWebamp(context).catch(err => {
                showDesktopNotification({ title: t('desktop.notification'), message: (err && err.message) || String(err) });
            });
            return;
        }
        const existing = findExistingAppWindow(appId, context || {});
        if (existing) {
            focusWindow(existing.id);
            if (appId === 'files' && context && context.path != null) {
                if (window.FileManager && typeof window.FileManager.navigateTo === 'function') window.FileManager.navigateTo(existing.id, context.path);
                else renderFiles(existing.id, context.path);
            }
            if (appId === 'editor' && context && context.path != null) renderEditor(existing.id, context.path, context.content || '');
            if (appId === 'code-studio' && context && context.path != null && window.CodeStudio && typeof window.CodeStudio.openFile === 'function') window.CodeStudio.openFile(context.path, true, existing.id);
            if (appId === 'agent-chat' && context && typeof applyChatLaunchContext === 'function') applyChatLaunchContext(existing.id, context); return;
        }
        const title = windowTitle(appId);
        const app = appById(appId);
        if (appIsBroken(app)) {
            showDesktopNotification({ title: t('desktop.app_broken'), message: t('desktop.app_missing_entry') + ' ' + t('desktop.app_recreate_hint') });
            return;
        }
        const id = 'w-' + appId + '-' + Date.now();
        const win = document.createElement('section');
        win.className = 'vd-window';
        win.dataset.windowId = id;

        const isMobileMode = window.useMobileDesktopMode && window.useMobileDesktopMode();
        const forceMaximized = window.shouldForceMobileMaximizedWindow && window.shouldForceMobileMaximizedWindow(appId);

        // On mobile, most apps open maximized (single window experience)
        if (isMobileMode && forceMaximized) {
            win.classList.add('maximized', 'vd-mobile-forced-maximized');
        } else {
            win.classList.toggle('vd-mobile-wide-window', shouldUseMobileWideWindow(appId));
        }

        const requestedSize = appWindowSize(appId);
        const size = clampWindowSize(requestedSize);
        const position = nextWindowPosition(size);

        if (isMobileMode && forceMaximized) {
            // Mobile forced maximized windows take full available space
            win.style.left = '0px';
            win.style.top = '0px';
            win.style.width = '100%';
            win.style.height = '100%';

            // Enforce Single Window behavior on mobile:
            // Minimize all other open windows when opening a new normal app
            state.windows.forEach((existingWin, existingId) => {
                if (existingId !== id && !existingWin.minimized) {
                    minimizeWindow(existingId);
                }
            });
        } else {
            win.style.left = position.left + 'px';
            win.style.top = position.top + 'px';
            win.style.width = size.width + 'px';
            win.style.height = size.height + 'px';
        }
        const isResizable = appId !== 'calculator' && appId !== 'galaxa-deluxe';

        win.style.minWidth = Math.min(WINDOW_MIN_W, size.width) + 'px';
        win.style.minHeight = Math.min(WINDOW_MIN_H, size.height) + 'px';
        const minSize = appWindowMinSize(appId);
        win.style.minWidth = Math.min(minSize.width, size.width) + 'px';
        win.style.minHeight = Math.min(minSize.height, size.height) + 'px';

        if (!isResizable || (isMobileMode && forceMaximized)) {
            win.style.maxWidth = size.width + 'px';
            win.style.maxHeight = size.height + 'px';
            win.style.resize = 'none';
        }
        win.style.zIndex = String(++state.z);
        win.innerHTML = `<header class="vd-window-titlebar">
            <div class="vd-window-title-group">
                <span class="vd-window-header-icon-wrap">${iconMarkup(iconForApp(app), iconGlyph(app), 'vd-window-header-icon', 16)}</span>
                <div class="vd-window-title">${esc(title)}</div>
                <div class="vd-window-subtitle"></div>
            </div>
            <div class="vd-window-actions">
                ${aiButtonMarkup(appId)}<button class="vd-window-button" type="button" data-action="minimize" title="${esc(t('desktop.minimize'))}" aria-label="${esc(t('desktop.minimize'))}"></button>
                ${isResizable ? `<button class="vd-window-button" type="button" data-action="maximize" title="${esc(t('desktop.maximize'))}" aria-label="${esc(t('desktop.maximize'))}"></button>` : ''}
                <button class="vd-window-button" type="button" data-action="close" title="${esc(t('desktop.close'))}" aria-label="${esc(t('desktop.close'))}"></button>
            </div>
        </header>
        <div class="vd-window-content" data-window-content></div>
        ${isResizable ? resizeHandleMarkup() : ''}`;
        $('vd-window-layer').appendChild(win);
        const windowContext = Object.assign({}, context || {});
        if (windowContext.path != null) windowContext.path = normalizeDesktopPath(windowContext.path);
        state.windows.set(id, { id, appId, title, element: win, maximized: false, restoreBounds: null, context: windowContext });
        wireWindow(win, id);
        animateThen(win, 'vd-window-opening', 240);
        if (shouldOpenMaximized(app)) toggleMaximizeWindow(id);
        focusWindow(id);
        renderAppContent(id, appId, windowContext);
        renderTaskbar();
    }

    function resizeHandleMarkup() {
        return ['n', 's', 'e', 'w', 'ne', 'nw', 'se', 'sw']
            .map(edge => `<span class="vd-resize-handle vd-resize-${edge}" data-resize="${edge}"></span>`)
            .join('');
    }
