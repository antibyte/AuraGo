// Network Cameras – person detection: constants, pure helpers and markup
// builders shared by network-cameras-detection-zones.js and
// network-cameras-detection.js. Server strings reach markup only through
// ctx.esc; SVG overlays are built from numbers only. No DOM access here.
(function () {
    'use strict';

    const API = '/api/desktop/integrations/go2rtc/detection/';
    const THUMBNAIL_API = '/api/desktop/integrations/go2rtc/thumbnail/';
    const NS = 'desktop.network_cameras.detection.';
    const POLL_MS = 5000;
    const PAGE_SIZE = 30;
    const MAX_ZONES = 8;
    const MIN_POINTS = 3;
    const MAX_POINTS = 32;
    const MAX_ZONE_NAME = 48;
    const MAX_INSTRUCTION = 500;
    const DAY_KEYS = ['mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun'];
    const CHANNELS = ['all', 'telegram', 'discord', 'ntfy', 'pushover', 'push', 'telnyx', 'cyd'];
    const CHANNEL_NAMES = Object.freeze({ telegram: 'Telegram', discord: 'Discord', ntfy: 'ntfy', pushover: 'Pushover', telnyx: 'Telnyx', cyd: 'CYD' });
    const ACTIONS = ['message', 'agent', 'desktop', 'mission'];
    const ACTION_GLYPHS = Object.freeze({ message: '✉', agent: '✦', desktop: '▣', mission: '⚑' });
    const ALERT_STATES = ['sent', 'failed', 'skipped'];
    const ARM_MODES = ['off', 'on', 'schedule'];
    const ARM_ACTORS = ['user', 'agent', 'schedule'];
    const DETECTOR_STATES = ['off', 'downloading', 'preparing', 'ready', 'degraded', 'error'];
    const CAMERA_STATES = ['active', 'unreachable', 'waiting', 'error'];
    const PHASES = ['downloading_model', 'downloading_runtime', 'preparing_gpu_model', 'starting_worker', 'self_test'];
    const MODEL_MB = Object.freeze({ standard: 15, accurate: 41 });
    const RUNTIME_MB = Object.freeze({ cpu: 9, gpu: 25 });
    const GPU_HINT_PATTERN = /gpu|render|dri|webgpu|adapter|vulkan/i;
    const TIME_PATTERN = /^([01]\d|2[0-3]):[0-5]\d$/;
    const JPEG_DATA_URL = /^data:image\/jpeg;base64,[A-Za-z0-9+/]+=*$/;
    const ZONE_COLORS = 8;
    // C5 zone names: control (Cc) and format (Cf) characters are rejected, except the
    // joiners ZWNJ U+200C and ZWJ U+200D; a name needs one visible character.
    const NAME_FORBIDDEN = /[\p{Cc}\p{Cf}]/u;
    const NAME_JOINERS = [0x200c, 0x200d];
    const EDGE_SNAP = 0.015;
    const FLAT_AREA = 0.0001;
    const ZONE_ABBREVIATION = /^[A-Za-z0-9+_-]{1,16}$/;
    const UTC_OFFSET = /^([+-])(\d{2}):(\d{2})$/;

    function T(ctx, key, params) {
        return ctx.t(NS + key, params || {});
    }

    function finite(value, fallback) {
        const n = Number(value);
        return Number.isFinite(n) ? n : fallback;
    }

    function clamp(value, min, max) {
        return Math.min(max, Math.max(min, value));
    }

    function round4(value) {
        return Math.round(value * 10000) / 10000;
    }

    function runeLength(value) {
        return Array.from(String(value == null ? '' : value)).length;
    }

    function percent(score) {
        return Math.round(clamp(finite(score, 0), 0, 1) * 100);
    }

    // Event, frame and arm-log ids are int64 values around 1.8e12, below 2^53: a JSON
    // number or a decimal string (data attribute) is exact; anything else is dropped.
    function idOf(value) {
        if (typeof value === 'string' && !/^[1-9]\d{0,15}$/.test(value)) return null;
        if (typeof value !== 'string' && typeof value !== 'number') return null;
        const n = Number(value);
        return Number.isSafeInteger(n) && n > 0 ? n : null;
    }

    // domKey turns a window id into a safe id/name fragment for markup.
    function domKey(value) {
        return String(value == null ? '' : value).replace(/[^A-Za-z0-9_-]/g, '') || 'w';
    }

    // zoneNameFold mirrors the server's NFC + case-folding comparison closely enough;
    // the server stays authoritative.
    function zoneNameFold(name) {
        const text = String(name == null ? '' : name).trim();
        return (typeof text.normalize === 'function' ? text.normalize('NFC') : text).toLowerCase();
    }

    function validZoneName(name) {
        const chars = Array.from(String(name == null ? '' : name).trim());
        if (!chars.length || chars.length > MAX_ZONE_NAME) return false;
        let visible = false;
        for (const char of chars) {
            const code = char.codePointAt(0);
            if (code >= 0xd800 && code <= 0xdfff) return false; // lone surrogate: not valid UTF-8
            if (NAME_JOINERS.includes(code)) continue;
            if (NAME_FORBIDDEN.test(char)) return false;
            if (!/\s/u.test(char)) visible = true;
        }
        return visible;
    }

    // serverZone reads ArmState.time_zone/utc_offset: the zone the schedule runs in.
    function serverZone(arm) {
        const match = arm && typeof arm.utc_offset === 'string' ? UTC_OFFSET.exec(arm.utc_offset) : null;
        if (!match) return null;
        const minutes = (match[1] === '-' ? -1 : 1) * (Number(match[2]) * 60 + Number(match[3]));
        const name = typeof arm.time_zone === 'string' && ZONE_ABBREVIATION.test(arm.time_zone) ? arm.time_zone : '';
        return { minutes, label: (name ? name + ', ' : '') + 'UTC' + arm.utc_offset };
    }

    function uiLocale() {
        return (typeof document !== 'undefined' && document.documentElement && document.documentElement.lang) || undefined;
    }

    function parseTime(value) {
        if (!value) return null;
        const date = new Date(value);
        return Number.isNaN(date.getTime()) ? null : date;
    }

    function sameDay(a, b) {
        return a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate();
    }

    function formatClock(date) {
        return date.toLocaleTimeString(uiLocale(), { hour: '2-digit', minute: '2-digit' });
    }

    function formatMoment(date, now) {
        if (sameDay(date, now)) return formatClock(date);
        return date.toLocaleDateString(uiLocale(), { weekday: 'short' }) + ' ' + formatClock(date);
    }

    function formatDateTime(date) {
        return date.toLocaleString(uiLocale(), { dateStyle: 'medium', timeStyle: 'medium' });
    }

    function formatDay(date) {
        return date.toLocaleDateString(uiLocale(), { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' });
    }

    function formatDuration(seconds) {
        const total = Math.max(0, Math.round(finite(seconds, 0)));
        return Math.floor(total / 60) + ':' + String(total % 60).padStart(2, '0');
    }

    function dayKey(date) {
        return date.getFullYear() + '-' + String(date.getMonth() + 1).padStart(2, '0') + '-' + String(date.getDate()).padStart(2, '0');
    }

    function dayRange(key) {
        const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(String(key || ''));
        if (!match) return null;
        const from = new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]));
        if (Number.isNaN(from.getTime())) return null;
        return { from, to: new Date(from.getFullYear(), from.getMonth(), from.getDate() + 1) };
    }

    function weekdayLabel(index) {
        // 2024-01-01 was a Monday.
        return new Date(2024, 0, 1 + index).toLocaleDateString(uiLocale(), { weekday: 'short' });
    }

    function framesOf(detail) {
        return detail && Array.isArray(detail.frames) ? detail.frames.filter(frame => frame && idOf(frame.id)) : [];
    }

    function cameraDefaults(detection) {
        const source = detection && typeof detection === 'object' ? detection : {};
        const actions = source.actions && typeof source.actions === 'object' ? source.actions : {};
        const fresh = !(Number(source.interval_seconds) > 0);
        return {
            enabled: source.enabled === true,
            interval_seconds: fresh ? 2 : Math.round(Number(source.interval_seconds)),
            min_score: fresh ? 0.5 : round4(finite(source.min_score, 0.5)),
            cooldown_seconds: fresh ? 300 : Math.round(finite(source.cooldown_seconds, 300)),
            zones: (Array.isArray(source.zones) ? source.zones : []).slice(0, MAX_ZONES).map(zone => ({
                name: String(zone && zone.name || ''),
                points: (Array.isArray(zone && zone.points) ? zone.points : []).slice(0, MAX_POINTS).map(point => [
                    round4(clamp(finite(Array.isArray(point) ? point[0] : NaN, 0), 0, 1)),
                    round4(clamp(finite(Array.isArray(point) ? point[1] : NaN, 0), 0, 1))
                ])
            })),
            actions: {
                message: fresh ? false : actions.message === true,
                channel: CHANNELS.includes(actions.channel) ? actions.channel : 'all',
                agent: fresh ? false : actions.agent === true,
                agent_instruction: String(actions.agent_instruction || ''),
                desktop: fresh ? true : actions.desktop === true
            }
        };
    }

    function cameraPayload(draft) {
        const source = draft && typeof draft === 'object' ? draft : {};
        const actions = source.actions && typeof source.actions === 'object' ? source.actions : {};
        return {
            enabled: source.enabled === true,
            interval_seconds: Number(source.interval_seconds),
            min_score: round4(Number(source.min_score)),
            cooldown_seconds: Number(source.cooldown_seconds),
            zones: (Array.isArray(source.zones) ? source.zones : []).map(zone => ({
                name: String(zone && zone.name || '').trim(),
                points: (Array.isArray(zone && zone.points) ? zone.points : []).map(point => [round4(Number(point[0])), round4(Number(point[1]))])
            })),
            actions: {
                message: actions.message === true,
                channel: CHANNELS.includes(actions.channel) ? actions.channel : 'all',
                agent: actions.agent === true,
                agent_instruction: String(actions.agent_instruction || ''),
                desktop: actions.desktop === true
            }
        };
    }

    function validateCameraSettings(detection) {
        const d = detection && typeof detection === 'object' ? detection : {};
        const interval = Number(d.interval_seconds);
        if (!Number.isInteger(interval) || interval < 1 || interval > 30) return { key: 'invalid_interval' };
        const score = Number(d.min_score);
        if (!Number.isFinite(score) || score < 0.3 - 1e-9 || score > 0.95 + 1e-9) return { key: 'invalid_min_score' };
        const cooldown = Number(d.cooldown_seconds);
        if (!Number.isInteger(cooldown) || cooldown < 0 || cooldown > 86400) return { key: 'invalid_cooldown' };
        const zones = Array.isArray(d.zones) ? d.zones : [];
        if (zones.length > MAX_ZONES) return { key: 'zone_limit' };
        const names = new Set();
        for (const zone of zones) {
            const name = String(zone && zone.name || '');
            const folded = zoneNameFold(name);
            if (!validZoneName(name) || names.has(folded)) return { key: 'invalid_zone_name' };
            names.add(folded);
            const points = Array.isArray(zone.points) ? zone.points : [];
            if (points.length < MIN_POINTS || points.length > MAX_POINTS) return { key: 'invalid_zone_points' };
            for (const point of points) {
                if (!Array.isArray(point) || point.length !== 2 || !point.every(value => Number.isFinite(Number(value)) && Number(value) >= 0 && Number(value) <= 1)) return { key: 'invalid_zone_points' };
            }
        }
        if (runeLength(d.actions && d.actions.agent_instruction || '') > MAX_INSTRUCTION) return { key: 'invalid_instruction' };
        return null;
    }

    function globalDefaults(settings) {
        const source = settings && typeof settings === 'object' ? settings : {};
        const retention = Number(source.retention_days);
        const storage = Number(source.max_storage_mb);
        return {
            enabled: source.enabled === true,
            model: source.model === 'accurate' ? 'accurate' : 'standard',
            backend: source.backend === 'gpu' || source.backend === 'cpu' ? source.backend : 'auto',
            retention_days: Number.isInteger(retention) && retention > 0 ? retention : 14,
            max_storage_mb: Number.isInteger(storage) && storage > 0 ? storage : 2048,
            schedule: (Array.isArray(source.schedule) ? source.schedule : []).map(slot => ({
                days: DAY_KEYS.filter(day => Array.isArray(slot && slot.days) && slot.days.includes(day)),
                from: TIME_PATTERN.test(String(slot && slot.from)) ? String(slot.from) : '22:00',
                to: TIME_PATTERN.test(String(slot && slot.to)) ? String(slot.to) : '06:00'
            }))
        };
    }

    function settingsPayload(values) {
        const source = values && typeof values === 'object' ? values : {};
        return {
            enabled: source.enabled === true,
            model: source.model === 'accurate' ? 'accurate' : 'standard',
            backend: source.backend === 'gpu' || source.backend === 'cpu' ? source.backend : 'auto',
            retention_days: Number(source.retention_days),
            max_storage_mb: Number(source.max_storage_mb),
            schedule: (Array.isArray(source.schedule) ? source.schedule : []).map(slot => ({
                days: DAY_KEYS.filter(day => Array.isArray(slot && slot.days) && slot.days.includes(day)),
                from: String(slot && slot.from || ''),
                to: String(slot && slot.to || '')
            }))
        };
    }

    function validateGlobalSettings(settings) {
        const s = settings && typeof settings === 'object' ? settings : {};
        const retention = Number(s.retention_days);
        if (!Number.isInteger(retention) || retention < 1 || retention > 365) return { key: 'invalid_retention' };
        const storage = Number(s.max_storage_mb);
        if (!Number.isInteger(storage) || storage < 100 || storage > 1000000) return { key: 'invalid_storage' };
        for (const slot of Array.isArray(s.schedule) ? s.schedule : []) {
            const days = Array.isArray(slot && slot.days) ? slot.days : [];
            if (!days.length || !days.every(day => DAY_KEYS.includes(day)) || !TIME_PATTERN.test(String(slot.from || '')) || !TIME_PATTERN.test(String(slot.to || ''))) return { key: 'invalid_window' };
        }
        return null;
    }

    function armSummary(ctx, arm, now) {
        const mode = arm && arm.mode;
        const armed = !!(arm && arm.armed);
        if (mode === 'on') return T(ctx, 'armed_now');
        if (mode !== 'schedule') return T(ctx, 'disarmed_now');
        const next = parseTime(arm.next_change);
        if (!next) return T(ctx, armed ? 'armed_schedule' : 'disarmed_schedule_empty');
        return T(ctx, armed ? 'armed_schedule_until' : 'disarmed_schedule_from', { time: formatMoment(next, now || new Date()) });
    }

    function detectorSummary(ctx, detector) {
        const d = detector && typeof detector === 'object' ? detector : {};
        const state = DETECTOR_STATES.includes(d.state) ? d.state : 'off';
        const summary = { state, text: '', fallback: '', error: '', progress: null };
        if (state === 'off') {
            summary.text = T(ctx, 'state_off');
            return summary;
        }
        if (state === 'error') {
            summary.text = T(ctx, 'state_error');
            summary.error = String(d.error || '');
            return summary;
        }
        if (state === 'downloading' || state === 'preparing') {
            summary.text = PHASES.includes(d.phase) ? T(ctx, 'phase_' + d.phase) : T(ctx, 'state_preparing');
            const download = d.download && typeof d.download === 'object' ? d.download : {};
            const total = finite(download.total, 0);
            if (state === 'downloading' && total > 0) {
                const done = clamp(finite(download.downloaded, 0), 0, total);
                summary.progress = {
                    percent: clamp(Math.round(finite(download.percent, done / total * 100)), 0, 100),
                    done: (done / 1048576).toFixed(1),
                    total: (total / 1048576).toFixed(1)
                };
            }
            return summary;
        }
        const parts = [String(d.model_name || d.model || ''), d.backend === 'gpu' ? 'GPU' : 'CPU'];
        if (d.runtime) parts.push(String(d.runtime));
        if (finite(d.wall_ms, 0) > 0) parts.push(T(ctx, 'timing', { wall: Math.round(finite(d.wall_ms, 0)), cpu: Math.round(finite(d.cpu_ms, 0)) }));
        summary.text = parts.filter(Boolean).join(' · ');
        if (state === 'degraded') summary.text = T(ctx, 'state_degraded') + ' · ' + summary.text;
        if (d.fallback_reason) summary.fallback = T(ctx, 'fallback', { reason: String(d.fallback_reason) });
        return summary;
    }

    function cameraChipLabel(ctx, camera) {
        const state = CAMERA_STATES.includes(camera && camera.state) ? camera.state : 'waiting';
        if (state === 'active') return T(ctx, 'camera_active', { rate: (Math.round(finite(camera.analyses_per_minute, 0) * 10) / 10).toLocaleString(uiLocale()) });
        return T(ctx, 'camera_' + state);
    }

    function channelLabel(ctx, channel) {
        if (channel === 'all') return T(ctx, 'channel_all');
        if (channel === 'push') return T(ctx, 'channel_push');
        return CHANNEL_NAMES[channel] || String(channel);
    }

    function upsertEvents(list, incoming) {
        const byId = new Map();
        for (const event of (list || []).concat(incoming || [])) {
            const id = idOf(event && event.id);
            if (id) byId.set(id, event);
        }
        return Array.from(byId.values()).sort((a, b) => ((parseTime(b.started_at) || 0) - (parseTime(a.started_at) || 0)) || (idOf(b.id) - idOf(a.id)));
    }

    function mergeArmLog(list, incoming) {
        const byId = new Map();
        for (const row of (list || []).concat(incoming || [])) {
            const id = idOf(row && row.id);
            if (id) byId.set(id, row);
        }
        return Array.from(byId.values()).sort((a, b) => ((parseTime(b.at) || 0) - (parseTime(a.at) || 0)) || (idOf(b.id) - idOf(a.id))).slice(0, 500);
    }

    function eventMatches(event, filter) {
        if (!event) return false;
        if (filter && filter.camera && event.camera_id !== filter.camera) return false;
        if (filter && filter.day) {
            const started = parseTime(event.started_at);
            if (!started || dayKey(started) !== filter.day) return false;
        }
        return true;
    }

    function timelineEntries(events, armLog, filter, hasMore) {
        const entries = [];
        let oldest = null;
        for (const event of events || []) {
            const at = parseTime(event && event.started_at);
            if (!at || !idOf(event.id)) continue;
            entries.push({ kind: 'event', at, item: event });
            if (!oldest || at < oldest) oldest = at;
        }
        if (!(filter && filter.camera)) {
            const range = filter && filter.day ? dayRange(filter.day) : null;
            for (const row of armLog || []) {
                const at = parseTime(row && row.at);
                if (!at || !idOf(row.id)) continue;
                if (range && (at < range.from || at >= range.to)) continue;
                if (hasMore && oldest && at < oldest) continue;
                entries.push({ kind: 'arm', at, item: row });
            }
        }
        entries.sort((a, b) => (b.at - a.at) || (a.kind === 'event' ? -1 : 1));
        return entries;
    }

    function mainKind(view) {
        const status = view.status;
        if (!status) return view.connectionLost ? 'error' : 'loading';
        if (!view.loaded) return 'loading';
        const filtered = !!(view.filter && (view.filter.camera || view.filter.day));
        if (filtered || (Array.isArray(view.events) && view.events.length)) return 'workspace';
        if (!status.enabled) return 'off';
        const state = status.detector && status.detector.state;
        if (state === 'downloading' || state === 'preparing') return 'preparing';
        if (!Array.isArray(status.cameras) || !status.cameras.length) return 'no_camera';
        return 'workspace';
    }

    function svgNumber(value) {
        return String(Math.round(value * 10) / 10);
    }

    function boxesSvg(boxes, width, height, mode) {
        const w = Math.max(1, Math.round(finite(width, 1)));
        const h = Math.max(1, Math.round(finite(height, 1)));
        const fontSize = h * 0.035;
        let out = '';
        for (const box of Array.isArray(boxes) ? boxes : []) {
            if (!box || typeof box !== 'object') continue;
            const x1 = clamp(Number(box.x1), 0, 1);
            const y1 = clamp(Number(box.y1), 0, 1);
            const x2 = clamp(Number(box.x2), 0, 1);
            const y2 = clamp(Number(box.y2), 0, 1);
            if (![x1, y1, x2, y2].every(Number.isFinite) || x2 <= x1 || y2 <= y1) continue;
            const cls = mode === 'test' ? (box.counts === true ? 'ncd-box is-counted' : 'ncd-box is-ignored') : 'ncd-box';
            out += '<rect class="' + cls + '" x="' + svgNumber(x1 * w) + '" y="' + svgNumber(y1 * h) + '" width="' + svgNumber((x2 - x1) * w) + '" height="' + svgNumber((y2 - y1) * h) + '" vector-effect="non-scaling-stroke"></rect>' +
                '<text class="ncd-box-label" x="' + svgNumber(x1 * w + fontSize * 0.3) + '" y="' + svgNumber(Math.max(fontSize, y1 * h - fontSize * 0.3)) + '" font-size="' + svgNumber(fontSize) + '" stroke-width="' + svgNumber(fontSize * 0.18) + '">' + percent(box.score) + ' %</text>';
        }
        return out;
    }

    function pointsAttr(points, w, h) {
        return points.map(point => svgNumber(clamp(Number(point[0]), 0, 1) * w) + ',' + svgNumber(clamp(Number(point[1]), 0, 1) * h)).join(' ');
    }

    function validPoints(points) {
        return (Array.isArray(points) ? points : []).filter(point => Array.isArray(point) && Number.isFinite(Number(point[0])) && Number.isFinite(Number(point[1])));
    }

    function zoneEditorSvg(view) {
        const w = Math.max(1, Math.round(finite(view.width, 1280)));
        const h = Math.max(1, Math.round(finite(view.height, 720)));
        const radius = svgNumber(Math.max(w, h) * 0.012);
        const zones = Array.isArray(view.zones) ? view.zones : [];
        const selected = Number.isInteger(view.selected) ? view.selected : -1;
        const drawing = Array.isArray(view.drawing) ? validPoints(view.drawing) : null;
        let out = '';
        zones.forEach((zone, index) => {
            const points = validPoints(zone && zone.points);
            if (points.length < 2) return;
            out += '<polygon class="ncd-zone ncd-zone-c' + (index % ZONE_COLORS) + (index === selected ? ' is-selected' : '') + '" points="' + pointsAttr(points, w, h) + '" vector-effect="non-scaling-stroke"></polygon>';
        });
        if (!drawing && zones[selected]) {
            validPoints(zones[selected].points).forEach((point, index) => {
                out += '<circle class="ncd-vertex" data-zone="' + selected + '" data-point="' + index + '" cx="' + svgNumber(clamp(Number(point[0]), 0, 1) * w) + '" cy="' + svgNumber(clamp(Number(point[1]), 0, 1) * h) + '" r="' + radius + '" vector-effect="non-scaling-stroke"></circle>';
            });
        }
        if (drawing && drawing.length) {
            out += '<polyline class="ncd-drawing" points="' + pointsAttr(drawing, w, h) + '" vector-effect="non-scaling-stroke"></polyline>';
            drawing.forEach((point, index) => {
                out += '<circle class="ncd-drawing-point" data-drawing-point="' + index + '" cx="' + svgNumber(clamp(Number(point[0]), 0, 1) * w) + '" cy="' + svgNumber(clamp(Number(point[1]), 0, 1) * h) + '" r="' + radius + '" vector-effect="non-scaling-stroke"></circle>';
            });
        }
        return out + boxesSvg(view.boxes, w, h, 'test');
    }

    function nextZoneName(ctx, zones) {
        const used = new Set((zones || []).map(zone => zoneNameFold(zone && zone.name)));
        for (let n = 1; n <= MAX_ZONES + 1; n += 1) {
            const name = T(ctx, 'zone_default_name', { n });
            if (!used.has(zoneNameFold(name))) return name;
        }
        return T(ctx, 'zone_default_name', { n: (zones || []).length + 1 });
    }

    // snapEdge pulls a coordinate within EDGE_SNAP of a frame edge onto it, so a zone
    // drawn to the edge reaches it and persons cut off by the frame still hit it (the
    // backend clamps the foot point to 1-1e-6). Zone polygons keep the SVG default
    // fill rule (nonzero), the winding rule the backend uses for its hit test.
    function snapEdge(value) {
        if (value <= EDGE_SNAP) return 0;
        if (value >= 1 - EDGE_SNAP) return 1;
        return value;
    }

    function normalizedPoint(clientX, clientY, rect) {
        const width = rect && rect.width > 0 ? rect.width : 1;
        const height = rect && rect.height > 0 ? rect.height : 1;
        const left = rect ? rect.left : 0;
        const top = rect ? rect.top : 0;
        return [round4(snapEdge(clamp((clientX - left) / width, 0, 1))), round4(snapEdge(clamp((clientY - top) / height, 0, 1)))];
    }

    // isFlatZone flags a zone whose points (nearly) lie on one line, which no person
    // can stand in. It sums the unsigned fan triangles from the first point, so a
    // self-crossing zone (filled by the nonzero rule) is never mistaken for flat.
    function isFlatZone(points) {
        const list = validPoints(points).map(point => [Number(point[0]), Number(point[1])]);
        let area = 0;
        for (let index = 1; index + 1 < list.length; index += 1) {
            const [a, b, c] = [list[0], list[index], list[index + 1]];
            area += Math.abs((b[0] - a[0]) * (c[1] - a[1]) - (c[0] - a[0]) * (b[1] - a[1])) / 2;
        }
        return area < FLAT_AREA;
    }

    function isNearPoint(a, b) {
        return !!a && !!b && Math.abs(a[0] - b[0]) < 0.006 && Math.abs(a[1] - b[1]) < 0.006;
    }

    // launchTarget reads a launch context ({event_id, camera_id}, e.g. from a notification click).
    function launchTarget(launch) {
        const eventId = idOf(launch && launch.event_id);
        if (!eventId) return null;
        const camera = typeof launch.camera_id === 'string' && /^[a-z0-9_-]{1,64}$/.test(launch.camera_id) ? launch.camera_id : '';
        return { eventId, camera };
    }

    function progressMarkup(ctx, progress) {
        return '<span class="ncd-progress" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow="' + progress.percent + '"><span style="width:' + progress.percent + '%"></span></span>' +
            '<span class="ncd-progress-text">' + ctx.esc(T(ctx, 'download_progress', { done: progress.done, total: progress.total })) + '</span>';
    }

    function chipMarkup(ctx, camera) {
        const state = CAMERA_STATES.includes(camera && camera.state) ? camera.state : 'waiting';
        const error = camera && camera.last_error ? String(camera.last_error) : '';
        return '<span class="ncd-chip is-' + state + '" role="listitem"' + (error ? ' title="' + ctx.esc(error) + '"' : '') + '><span class="ncd-dot" aria-hidden="true"></span><strong>' + ctx.esc(camera && (camera.name || camera.id) || '') + '</strong><small>' + ctx.esc(cameraChipLabel(ctx, camera)) + '</small></span>';
    }

    function stripMarkup(ctx, view) {
        const status = view.status || {};
        const arm = status.arm || {};
        const detector = detectorSummary(ctx, status.detector);
        const cameras = Array.isArray(status.cameras) ? status.cameras : [];
        let html = '<div class="ncd-strip"><div class="ncd-arm" role="group" aria-label="' + ctx.esc(T(ctx, 'arm_label')) + '">';
        if (view.canManage && status.enabled) {
            html += '<div class="ncd-arm-buttons">' + ARM_MODES.map(mode => '<button type="button" data-arm="' + mode + '" data-focus-key="arm-' + mode + '" aria-pressed="' + (arm.mode === mode) + '"' + (view.armBusy ? ' disabled' : '') + '>' + ctx.esc(T(ctx, 'arm_' + mode)) + '</button>').join('') + '</div>';
        }
        html += '<span class="ncd-arm-summary ' + (arm.armed ? 'is-armed' : 'is-disarmed') + '"><span class="ncd-dot" aria-hidden="true"></span>' + ctx.esc(armSummary(ctx, arm, view.now)) + '</span>';
        // next_change is shown in the browser's zone; the schedule runs in the server's.
        const zone = arm.mode === 'schedule' ? serverZone(arm) : null;
        if (zone && zone.minutes !== -(view.now || new Date()).getTimezoneOffset()) html += '<small class="ncd-server-zone">' + ctx.esc(T(ctx, 'arm_server_zone', { zone: zone.label })) + '</small>';
        html += '</div>';
        html += '<div class="ncd-detector is-' + detector.state + '"><span class="ncd-dot" aria-hidden="true"></span><span class="ncd-detector-text">' + ctx.esc(detector.text) + '</span>';
        if (detector.progress) html += progressMarkup(ctx, detector.progress);
        if (detector.error) html += '<span class="ncd-error-text">' + ctx.esc(detector.error) + '</span>';
        if (detector.state === 'error' && view.canManage) html += '<button type="button" class="ncd-button" data-ncd-action="retry" data-focus-key="retry"' + (view.busy ? ' disabled' : '') + '>' + ctx.esc(T(ctx, 'retry')) + '</button>';
        if (detector.fallback) html += '<span class="ncd-fallback">' + ctx.esc(detector.fallback) + '</span>';
        html += '</div>';
        if (cameras.length) html += '<div class="ncd-chips" role="list" aria-label="' + ctx.esc(T(ctx, 'cameras_label')) + '">' + cameras.map(camera => chipMarkup(ctx, camera)).join('') + '</div>';
        if (view.canManage) html += '<div class="ncd-strip-actions"><button type="button" class="ncd-button" data-ncd-action="settings" data-focus-key="settings">' + ctx.iconMarkup('settings', '⚙', 'vd-sprite-icon nc-glyph', 15) + '<span>' + ctx.esc(T(ctx, 'settings')) + '</span></button></div>';
        const storage = status.storage && typeof status.storage === 'object' ? status.storage : {};
        if (storage.warning) html += '<div class="ncd-notice is-warning" role="status">' + ctx.esc(storage.warning === 'storage_full' ? T(ctx, 'storage_full') : String(storage.warning)) + '</div>';
        if (view.connectionLost) html += '<div class="ncd-notice is-warning" role="status">' + ctx.esc(T(ctx, 'connection_lost')) + '</div>';
        return html + '</div>';
    }

    function emptyMarkup(ctx, view) {
        const kind = view.kind;
        if (kind === 'loading') return '<div class="ncd-empty" role="status"><span class="nc-spinner" aria-hidden="true"></span><span>' + ctx.esc(T(ctx, 'loading')) + '</span></div>';
        if (kind === 'error') return '<div class="ncd-empty"><h2>' + ctx.esc(T(ctx, 'load_failed')) + '</h2><div class="ncd-empty-actions"><button type="button" class="nc-primary" data-ncd-action="reload" data-focus-key="reload">' + ctx.esc(T(ctx, 'retry')) + '</button></div></div>';
        if (kind === 'off') {
            let html = '<div class="ncd-empty"><h2>' + ctx.esc(T(ctx, 'off_title')) + '</h2><p>' + ctx.esc(T(ctx, view.canManage ? 'off_body_admin' : 'off_body_viewer')) + '</p>';
            if (view.canManage) {
                const settings = globalDefaults(view.settings);
                html += '<p>' + ctx.esc(T(ctx, 'off_download', { model: MODEL_MB[settings.model], runtime: RUNTIME_MB[settings.backend === 'cpu' ? 'cpu' : 'gpu'] })) + '</p>';
                html += '<div class="ncd-empty-actions"><button type="button" class="nc-primary" data-ncd-action="activate" data-focus-key="activate"' + (view.busy ? ' disabled' : '') + '>' + ctx.esc(T(ctx, 'activate')) + '</button><button type="button" class="ncd-button" data-ncd-action="settings" data-focus-key="settings-empty">' + ctx.esc(T(ctx, 'settings')) + '</button></div>';
            }
            return html + '</div>';
        }
        if (kind === 'preparing') {
            const detector = detectorSummary(ctx, view.status && view.status.detector);
            return '<div class="ncd-empty" role="status"><span class="nc-spinner" aria-hidden="true"></span><h2>' + ctx.esc(T(ctx, 'preparing_title')) + '</h2><p>' + ctx.esc(detector.text) + '</p>' + (detector.progress ? '<div class="ncd-empty-progress">' + progressMarkup(ctx, detector.progress) + '</div>' : '') + '</div>';
        }
        let html = '<div class="ncd-empty"><h2>' + ctx.esc(T(ctx, 'no_camera_title')) + '</h2><p>' + ctx.esc(T(ctx, view.canManage ? 'no_camera_body_admin' : 'no_camera_body_viewer')) + '</p>';
        if (view.canManage) {
            const streams = (view.streams || []).filter(stream => stream && stream.id);
            if (streams.length) html += '<div class="ncd-empty-actions">' + streams.map(stream => '<button type="button" class="ncd-button" data-ncd-camera="' + ctx.esc(stream.id) + '" data-focus-key="camera-' + ctx.esc(stream.id) + '">' + ctx.esc(T(ctx, 'setup_camera', { camera: stream.name || stream.id })) + '</button>').join('') + '</div>';
        }
        return html + '</div>';
    }

    function cameraOptions(view) {
        const seen = new Map();
        for (const stream of view.streams || []) if (stream && stream.id) seen.set(String(stream.id), String(stream.name || stream.id));
        const cameras = view.status && Array.isArray(view.status.cameras) ? view.status.cameras : [];
        for (const camera of cameras) if (camera && camera.id && !seen.has(String(camera.id))) seen.set(String(camera.id), String(camera.name || camera.id));
        return Array.from(seen.entries());
    }

    function alertBadgesMarkup(ctx, alerts) {
        const source = alerts && typeof alerts === 'object' ? alerts : {};
        return ACTIONS.filter(action => source[action]).map(action => {
            const result = source[action] && typeof source[action] === 'object' ? source[action] : {};
            const state = ALERT_STATES.includes(result.status) ? result.status : 'skipped';
            const label = T(ctx, 'action_' + action) + ': ' + T(ctx, 'alert_' + state) + (result.detail ? ' – ' + String(result.detail) : '');
            return '<span class="ncd-alert is-' + state + '" role="img" title="' + ctx.esc(label) + '" aria-label="' + ctx.esc(label) + '">' + ACTION_GLYPHS[action] + '</span>';
        }).join('');
    }

    function eventRowMarkup(ctx, event, started, selectedId) {
        const id = idOf(event.id);
        const cover = idOf(event.cover_frame_id);
        const ended = parseTime(event.ended_at);
        const zones = Array.isArray(event.zones) && event.zones.length ? event.zones.map(String).join(', ') : T(ctx, 'whole_frame');
        const meta = [
            formatClock(started),
            ended ? formatDuration((ended - started) / 1000) : T(ctx, 'running'),
            percent(event.max_score) + ' %',
            T(ctx, 'persons', { count: Math.max(0, Math.round(finite(event.max_persons, 0))) })
        ].join(' · ');
        return '<div role="listitem"><button type="button" class="ncd-row" data-event="' + id + '" data-focus-key="event-' + id + '" aria-current="' + (id === selectedId) + '">' +
            '<span class="ncd-thumb" aria-hidden="true">' + (cover ? '<img src="' + API + 'images/' + cover + '?thumb=1" alt="" loading="lazy" decoding="async">' : '') + '</span>' +
            '<span class="ncd-row-main"><strong>' + ctx.esc(event.camera_name || event.camera_id || '') + '</strong><span class="ncd-row-meta">' + ctx.esc(meta) + '</span><span class="ncd-row-zones">' + ctx.esc(zones) + '</span></span>' +
            '<span class="ncd-row-side"><span class="ncd-badge ' + (event.armed ? 'is-armed' : 'is-disarmed') + '">' + ctx.esc(T(ctx, event.armed ? 'armed_badge' : 'disarmed_badge')) + '</span><span class="ncd-alerts">' + alertBadgesMarkup(ctx, event.alerts) + '</span></span>' +
            '</button></div>';
    }

    function armRowMarkup(ctx, row, at) {
        const mode = ARM_MODES.includes(row.mode) ? row.mode : 'off';
        const actor = ARM_ACTORS.includes(row.actor) ? row.actor : 'user';
        const text = T(ctx, 'arm_log_row', { mode: T(ctx, 'arm_' + mode), state: T(ctx, row.armed ? 'armed_now' : 'disarmed_now'), actor: T(ctx, 'actor_' + actor) });
        return '<div role="listitem" class="ncd-arm-row ' + (row.armed ? 'is-armed' : 'is-disarmed') + '"><span>' + ctx.esc(text) + '</span><time datetime="' + ctx.esc(at.toISOString()) + '">' + ctx.esc(formatClock(at)) + '</time></div>';
    }

    function timelineMarkup(ctx, view) {
        const filter = view.filter || { camera: '', day: '' };
        let html = '<section class="ncd-timeline" aria-label="' + ctx.esc(T(ctx, 'timeline_label')) + '"><div class="ncd-filters">';
        html += '<label>' + ctx.esc(T(ctx, 'filter_camera')) + '<select data-ncd-filter="camera" data-focus-key="filter-camera"><option value="">' + ctx.esc(T(ctx, 'filter_all_cameras')) + '</option>' +
            cameraOptions(view).map(([id, name]) => '<option value="' + ctx.esc(id) + '"' + (id === filter.camera ? ' selected' : '') + '>' + ctx.esc(name) + '</option>').join('') + '</select></label>';
        html += '<label>' + ctx.esc(T(ctx, 'filter_day')) + '<input type="date" data-ncd-filter="day" data-focus-key="filter-day" value="' + ctx.esc(filter.day || '') + '"></label>';
        if (filter.day) html += '<button type="button" class="ncd-button" data-ncd-action="clear-day" data-focus-key="clear-day">' + ctx.esc(T(ctx, 'filter_clear_day')) + '</button>';
        html += '</div><div class="ncd-list" data-scroll role="list">';
        const entries = timelineEntries(view.events, view.armLog, filter, view.hasMore);
        let lastDay = '';
        for (const entry of entries) {
            const key = dayKey(entry.at);
            if (key !== lastDay) {
                html += '<div class="ncd-day" role="presentation">' + ctx.esc(formatDay(entry.at)) + '</div>';
                lastDay = key;
            }
            html += entry.kind === 'event' ? eventRowMarkup(ctx, entry.item, entry.at, view.selectedId) : armRowMarkup(ctx, entry.item, entry.at);
        }
        if (!entries.length) {
            const message = view.eventsError ? view.eventsError : (filter.camera || filter.day ? T(ctx, 'no_events_filtered') : T(ctx, 'no_events'));
            html += '<p class="ncd-empty-list">' + ctx.esc(message) + '</p>';
        }
        html += '</div>';
        if (view.eventsError && entries.length) html += '<p class="ncd-empty-list is-error" role="alert">' + ctx.esc(view.eventsError) + '</p>';
        if (view.hasMore) html += '<button type="button" class="ncd-button ncd-more" data-ncd-action="more" data-focus-key="more"' + (view.loadingMore ? ' disabled' : '') + '>' + ctx.esc(T(ctx, 'load_more')) + '</button>';
        return html + '</section>';
    }

    function detailMarkup(ctx, view) {
        const keysId = 'ncd-keys-' + domKey(view.windowKey);
        let html = '<section class="ncd-detail" tabindex="0" data-focus-key="detail" aria-label="' + ctx.esc(T(ctx, 'detail_label')) + '" aria-describedby="' + keysId + '">';
        if (!view.selectedId) return html + '<p class="ncd-detail-empty" id="' + keysId + '">' + ctx.esc(T(ctx, 'detail_empty')) + '</p></section>';
        if (view.error) return html + '<p class="ncd-detail-empty is-error" role="alert" id="' + keysId + '">' + ctx.esc(view.error) + '</p></section>';
        const detail = view.detail;
        if (!detail) return html + '<div class="ncd-detail-empty" id="' + keysId + '"><span class="nc-spinner" role="status" aria-label="' + ctx.esc(T(ctx, 'loading')) + '"></span></div></section>';
        const frames = framesOf(detail);
        const index = clamp(Math.round(finite(view.frameIndex, 0)), 0, Math.max(0, frames.length - 1));
        const frame = frames[index];
        const started = parseTime(detail.started_at);
        const ended = parseTime(detail.ended_at);
        const camera = String(detail.camera_name || detail.camera_id || '');
        if (frame) {
            const w = Math.max(1, Math.round(finite(frame.width, 16)));
            const h = Math.max(1, Math.round(finite(frame.height, 9)));
            const captured = parseTime(frame.captured_at);
            html += '<div class="ncd-frame" style="aspect-ratio: ' + w + ' / ' + h + '"><img src="' + API + 'images/' + idOf(frame.id) + '" alt="' + ctx.esc(camera + (captured ? ' ' + formatDateTime(captured) : '')) + '">' +
                '<svg viewBox="0 0 ' + w + ' ' + h + '" preserveAspectRatio="xMidYMid meet" aria-hidden="true">' + boxesSvg(frame.boxes, w, h, 'event') + '</svg></div>';
            html += '<div class="ncd-frame-nav"><button type="button" class="ncd-button" data-ncd-action="prev-frame" data-focus-key="prev-frame" aria-label="' + ctx.esc(T(ctx, 'previous_frame')) + '"' + (index <= 0 ? ' disabled' : '') + '>←</button>' +
                '<span class="ncd-frame-position" aria-live="polite">' + ctx.esc(T(ctx, 'frame_position', { current: index + 1, total: frames.length })) + '</span>' +
                '<button type="button" class="ncd-button" data-ncd-action="next-frame" data-focus-key="next-frame" aria-label="' + ctx.esc(T(ctx, 'next_frame')) + '"' + (index >= frames.length - 1 ? ' disabled' : '') + '>→</button></div>';
        }
        html += '<p class="ncd-hint" id="' + keysId + '">' + ctx.esc(T(ctx, 'frame_keys')) + '</p>';
        const zones = Array.isArray(detail.zones) && detail.zones.length ? detail.zones.map(String).join(', ') : T(ctx, 'whole_frame');
        const rows = [
            ['meta_camera', camera],
            ['meta_start', started ? formatDateTime(started) : ''],
            ['meta_end', ended ? formatDateTime(ended) : T(ctx, 'running')],
            ['meta_duration', started && ended ? formatDuration((ended - started) / 1000) : T(ctx, 'running')],
            ['meta_score', percent(detail.max_score) + ' %'],
            ['meta_persons', String(Math.max(0, Math.round(finite(detail.max_persons, 0))))],
            ['meta_zones', zones],
            ['meta_armed', T(ctx, detail.armed ? 'armed_badge' : 'disarmed_badge')]
        ];
        html += '<dl class="ncd-meta">' + rows.map(([key, value]) => '<dt>' + ctx.esc(T(ctx, key)) + '</dt><dd>' + ctx.esc(value) + '</dd>').join('') + '</dl>';
        html += '<h3>' + ctx.esc(T(ctx, 'alerts_label')) + '</h3>';
        const alerts = detail.alerts && typeof detail.alerts === 'object' ? detail.alerts : {};
        const present = ACTIONS.filter(action => alerts[action]);
        if (present.length) {
            html += '<ul class="ncd-alert-list">' + present.map(action => {
                const result = alerts[action] && typeof alerts[action] === 'object' ? alerts[action] : {};
                const state = ALERT_STATES.includes(result.status) ? result.status : 'skipped';
                return '<li class="is-' + state + '"><span class="ncd-alert is-' + state + '" aria-hidden="true">' + ACTION_GLYPHS[action] + '</span><span>' + ctx.esc(T(ctx, 'action_' + action) + ': ' + T(ctx, 'alert_' + state)) + '</span>' + (result.detail ? '<small>' + ctx.esc(String(result.detail)) + '</small>' : '') + '</li>';
            }).join('') + '</ul>';
        } else {
            html += '<p class="ncd-hint">' + ctx.esc(T(ctx, 'alert_none')) + '</p>';
        }
        if (view.canManage) html += '<div class="ncd-detail-actions"><button type="button" class="ncd-button is-danger" data-ncd-action="delete-event" data-focus-key="delete-event"' + (view.deleting ? ' disabled' : '') + '>' + ctx.iconMarkup('trash', '×', 'vd-sprite-icon nc-glyph', 15) + '<span>' + ctx.esc(T(ctx, 'delete_event')) + '</span></button></div>';
        return html + '</section>';
    }

    function scheduleRowsMarkup(ctx, schedule) {
        const rows = Array.isArray(schedule) ? schedule : [];
        if (!rows.length) return '<p class="ncd-hint">' + ctx.esc(T(ctx, 'schedule_empty')) + '</p>';
        return rows.map((slot, index) => '<div class="ncd-window" data-window="' + index + '"><div class="ncd-days" role="group" aria-label="' + ctx.esc(T(ctx, 'schedule_days')) + '">' +
            DAY_KEYS.map((day, dayIndex) => '<button type="button" class="ncd-day-chip" data-window-day="' + index + ':' + day + '" data-focus-key="day-' + index + '-' + day + '" aria-pressed="' + slot.days.includes(day) + '">' + ctx.esc(weekdayLabel(dayIndex)) + '</button>').join('') + '</div>' +
            '<label>' + ctx.esc(T(ctx, 'schedule_from')) + '<input type="time" data-window-from="' + index + '" value="' + ctx.esc(slot.from) + '"></label>' +
            '<label>' + ctx.esc(T(ctx, 'schedule_to')) + '<input type="time" data-window-to="' + index + '" value="' + ctx.esc(slot.to) + '"></label>' +
            '<button type="button" class="ncd-button is-danger" data-window-remove="' + index + '" data-focus-key="remove-window-' + index + '" aria-label="' + ctx.esc(T(ctx, 'schedule_remove')) + '">' + ctx.iconMarkup('trash', '×', 'vd-sprite-icon nc-glyph', 14) + '</button></div>').join('');
    }

    function settingsMarkup(ctx, view) {
        const dialog = view.dialog || {};
        const values = dialog.values;
        const key = domKey(view.windowKey);
        let body = '';
        if (!values) {
            body = '<p class="ncd-error-text" role="alert">' + ctx.esc(T(ctx, 'settings_load_failed')) + '</p>';
        } else {
            body += '<label class="nc-check"><input type="checkbox" data-set-field="enabled"' + (values.enabled ? ' checked' : '') + '><span>' + ctx.esc(T(ctx, 'settings_enabled')) + '</span></label>';
            body += '<fieldset class="ncd-fieldset"><legend>' + ctx.esc(T(ctx, 'settings_model')) + '</legend>' + ['standard', 'accurate'].map(model => '<label class="nc-check"><input type="radio" name="ncd-model-' + key + '" value="' + model + '" data-set-field="model"' + (values.model === model ? ' checked' : '') + '><span>' + ctx.esc(T(ctx, 'model_' + model)) + '</span></label>').join('') + '<small class="ncd-hint">' + ctx.esc(T(ctx, 'runtime_sizes')) + '</small></fieldset>';
            body += '<label>' + ctx.esc(T(ctx, 'settings_backend')) + '<select data-set-field="backend">' + ['auto', 'gpu', 'cpu'].map(backend => '<option value="' + backend + '"' + (values.backend === backend ? ' selected' : '') + '>' + ctx.esc(T(ctx, 'backend_' + backend)) + '</option>').join('') + '</select></label>';
            const reason = view.status && view.status.detector ? String(view.status.detector.fallback_reason || '') : '';
            if (reason && GPU_HINT_PATTERN.test(reason)) body += '<div class="ncd-gpu-hint" role="note"><p>' + ctx.esc(T(ctx, 'gpu_hint')) + '</p><code>sudo usermod -aG render aurago</code><p>' + ctx.esc(T(ctx, 'gpu_hint_user')) + '</p><small>' + ctx.esc(reason) + '</small></div>';
            body += '<div class="nc-form-row"><label>' + ctx.esc(T(ctx, 'settings_retention')) + '<input type="number" min="1" max="365" step="1" inputmode="numeric" data-set-field="retention_days" value="' + ctx.esc(values.retention_days) + '"></label>' +
                '<label>' + ctx.esc(T(ctx, 'settings_storage')) + '<input type="number" min="100" max="1000000" step="100" inputmode="numeric" data-set-field="max_storage_mb" value="' + ctx.esc(values.max_storage_mb) + '"></label></div>';
            const storage = view.status && view.status.storage && typeof view.status.storage === 'object' ? view.status.storage : null;
            if (storage && finite(storage.limit_bytes, 0) > 0) body += '<small class="ncd-hint">' + ctx.esc(T(ctx, 'storage_usage', { used: (finite(storage.bytes, 0) / 1048576).toFixed(1), limit: Math.round(finite(storage.limit_bytes, 0) / 1048576) })) + '</small>';
            const zone = serverZone(view.status && view.status.arm);
            const zoneHint = zone ? '<small class="ncd-hint ncd-server-zone">' + ctx.esc(T(ctx, 'schedule_server_zone', { zone: zone.label })) + '</small>' : '';
            body += '<fieldset class="ncd-fieldset"><legend>' + ctx.esc(T(ctx, 'settings_schedule')) + '</legend><small class="ncd-hint">' + ctx.esc(T(ctx, 'schedule_hint')) + '</small>' + zoneHint + '<div class="ncd-windows" data-schedule-rows>' + scheduleRowsMarkup(ctx, values.schedule) + '</div>' +
                '<button type="button" class="ncd-button" data-ncd-action="add-window" data-focus-key="add-window">+ ' + ctx.esc(T(ctx, 'schedule_add')) + '</button></fieldset>';
        }
        const titleId = 'ncd-settings-title-' + key;
        return '<div class="nc-modal-backdrop ncd-dialog-backdrop"><section class="nc-modal ncd-settings" role="dialog" aria-modal="true" aria-labelledby="' + titleId + '" data-ncd-dialog>' +
            '<header><div><strong id="' + titleId + '">' + ctx.esc(T(ctx, 'settings_title')) + '</strong></div><button type="button" data-ncd-action="close-settings" data-focus-key="close-settings-x" aria-label="' + ctx.esc(ctx.t('desktop.close')) + '">×</button></header>' +
            '<div class="nc-modal-body"><div class="nc-form">' + body + '</div>' + (dialog.error ? '<div class="nc-modal-error" role="alert">' + ctx.esc(dialog.error) + '</div>' : '') + '</div>' +
            '<footer><button type="button" data-ncd-action="close-settings" data-focus-key="close-settings">' + ctx.esc(ctx.t('desktop.network_cameras.cancel')) + '</button>' +
            '<button type="button" class="nc-primary" data-ncd-action="save-settings" data-focus-key="save-settings"' + (dialog.busy || !values ? ' disabled' : '') + '>' + ctx.esc(ctx.t('desktop.network_cameras.save')) + '</button></footer></section></div>';
    }

    function zoneListMarkup(ctx, zones, selected) {
        return (zones || []).map((zone, index) => '<li class="ncd-zone-item" data-zone-item="' + index + '"><span class="ncd-swatch ncd-zone-c' + (index % ZONE_COLORS) + '" aria-hidden="true"></span>' +
            '<input type="text" maxlength="48" data-zone-name="' + index + '" value="' + ctx.esc(zone.name) + '" aria-label="' + ctx.esc(T(ctx, 'zone_name_label', { n: index + 1 })) + '">' +
            '<small>' + ctx.esc(T(ctx, 'zone_points', { count: validPoints(zone.points).length })) + '</small>' +
            '<button type="button" class="ncd-button" data-zone-select="' + index + '" aria-pressed="' + (index === selected) + '">' + ctx.esc(T(ctx, 'zone_select')) + '</button>' +
            '<button type="button" class="ncd-button is-danger" data-zone-delete="' + index + '" aria-label="' + ctx.esc(T(ctx, 'zone_delete') + ': ' + zone.name) + '">' + ctx.iconMarkup('trash', '×', 'vd-sprite-icon nc-glyph', 14) + '</button></li>').join('');
    }

    function cameraSettingsMarkup(stream, ctx) {
        const draft = cameraDefaults(stream && stream.detection);
        const actions = draft.actions;
        const id = String(stream && stream.id || '');
        const idKey = id.replace(/[^A-Za-z0-9_-]/g, '') || 'camera';
        const helpId = 'ncd-zone-help-' + idKey;
        const restartId = 'ncd-restart-hint-' + idKey;
        let html = '<div class="nc-form ncd-cam" data-ncd-camera-settings>';
        if (stream && stream.detection_enabled === false) html += '<div class="ncd-notice is-warning" role="note">' + ctx.esc(T(ctx, 'cam_global_off')) + '</div>';
        html += '<label class="nc-check"><input type="checkbox" data-cam-field="enabled" aria-describedby="' + restartId + '"' + (draft.enabled ? ' checked' : '') + '><span>' + ctx.esc(T(ctx, 'cam_enabled')) + '</span></label>';
        html += '<p class="ncd-hint ncd-restart-hint" id="' + restartId + '">' + ctx.esc(T(ctx, 'cam_restart_hint')) + '</p>';
        html += '<div class="nc-form-row"><label>' + ctx.esc(T(ctx, 'cam_interval')) + '<input type="number" min="1" max="30" step="1" inputmode="numeric" data-cam-field="interval_seconds" value="' + draft.interval_seconds + '"></label>' +
            '<label>' + ctx.esc(T(ctx, 'cam_cooldown')) + '<input type="number" min="0" max="86400" step="1" inputmode="numeric" data-cam-field="cooldown_seconds" value="' + draft.cooldown_seconds + '"></label></div>';
        html += '<label>' + ctx.esc(T(ctx, 'cam_min_score')) + ' <output data-cam-out="min_score">' + percent(draft.min_score) + ' %</output><input type="range" min="0.3" max="0.95" step="0.05" data-cam-field="min_score" value="' + draft.min_score + '"></label>';
        html += '<fieldset class="ncd-fieldset"><legend>' + ctx.esc(T(ctx, 'cam_actions')) + '</legend>';
        html += '<label class="nc-check"><input type="checkbox" data-cam-field="actions.desktop"' + (actions.desktop ? ' checked' : '') + '><span>' + ctx.esc(T(ctx, 'cam_action_desktop')) + '</span></label>';
        html += '<label class="nc-check"><input type="checkbox" data-cam-field="actions.message"' + (actions.message ? ' checked' : '') + '><span>' + ctx.esc(T(ctx, 'cam_action_message')) + '</span></label>';
        html += '<label>' + ctx.esc(T(ctx, 'cam_channel')) + '<select data-cam-field="actions.channel"' + (actions.message ? '' : ' disabled') + '>' + CHANNELS.map(channel => '<option value="' + channel + '"' + (actions.channel === channel ? ' selected' : '') + '>' + ctx.esc(channelLabel(ctx, channel)) + '</option>').join('') + '</select></label>';
        html += '<label class="nc-check"><input type="checkbox" data-cam-field="actions.agent"' + (actions.agent ? ' checked' : '') + '><span>' + ctx.esc(T(ctx, 'cam_action_agent')) + '</span></label>';
        html += '<label>' + ctx.esc(T(ctx, 'cam_instruction')) + '<textarea maxlength="500" rows="3" data-cam-field="actions.agent_instruction" placeholder="' + ctx.esc(T(ctx, 'cam_instruction_placeholder')) + '"' + (actions.agent ? '' : ' disabled') + '>' + ctx.esc(actions.agent_instruction) + '</textarea>' +
            '<small class="ncd-hint" data-cam-out="instruction_count">' + runeLength(actions.agent_instruction) + '/' + MAX_INSTRUCTION + '</small></label>';
        html += '<p class="ncd-hint">' + ctx.esc(T(ctx, 'cam_mission_hint')) + '</p></fieldset>';
        html += '<fieldset class="ncd-fieldset ncd-zones"><legend>' + ctx.esc(T(ctx, 'zones_title')) + '</legend><p class="ncd-hint">' + ctx.esc(T(ctx, 'zones_hint')) + '</p>';
        html += '<div class="ncd-zone-stage" data-zone-stage tabindex="0" role="application" aria-label="' + ctx.esc(T(ctx, 'zone_editor_label')) + '" aria-describedby="' + helpId + '">' +
            '<img data-zone-bg alt="" draggable="false" src="' + ctx.esc(THUMBNAIL_API + encodeURIComponent(id) + '.jpg?width=1280&height=720&cache=0') + '">' +
            '<svg data-zone-svg viewBox="0 0 1280 720" preserveAspectRatio="none" aria-hidden="true">' + zoneEditorSvg({ zones: draft.zones, selected: -1, drawing: null, width: 1280, height: 720, boxes: [] }) + '</svg></div>';
        html += '<p class="ncd-hint" id="' + helpId + '">' + ctx.esc(T(ctx, 'zone_help')) + '</p>';
        html += '<div class="ncd-zone-tools"><button type="button" class="ncd-button" data-zone-action="close" disabled>' + ctx.esc(T(ctx, 'zone_close')) + '</button>' +
            '<button type="button" class="ncd-button" data-zone-action="cancel" disabled>' + ctx.esc(T(ctx, 'zone_cancel')) + '</button>' +
            '<button type="button" class="ncd-button" data-zone-action="test">' + ctx.esc(T(ctx, 'test')) + '</button>' +
            '<span class="ncd-legend" aria-hidden="true"><span class="is-counted">' + ctx.esc(T(ctx, 'legend_counts')) + '</span><span class="is-ignored">' + ctx.esc(T(ctx, 'legend_ignored')) + '</span></span></div>';
        html += '<p class="ncd-zone-status" data-zone-status role="status" aria-live="polite"></p>';
        html += '<ol class="ncd-zone-list" data-zone-list>' + zoneListMarkup(ctx, draft.zones, -1) + '</ol></fieldset></div>';
        return html;
    }
    window.NetworkCamerasDetectionViews = Object.freeze({
        API,
        THUMBNAIL_API,
        NS,
        POLL_MS,
        PAGE_SIZE,
        MAX_ZONES,
        MIN_POINTS,
        MAX_POINTS,
        MAX_ZONE_NAME,
        MAX_INSTRUCTION,
        DAY_KEYS,
        CHANNELS,
        CHANNEL_NAMES,
        ACTIONS,
        ACTION_GLYPHS,
        ALERT_STATES,
        ARM_MODES,
        ARM_ACTORS,
        DETECTOR_STATES,
        CAMERA_STATES,
        PHASES,
        MODEL_MB,
        RUNTIME_MB,
        GPU_HINT_PATTERN,
        TIME_PATTERN,
        JPEG_DATA_URL,
        ZONE_COLORS,
        EDGE_SNAP,
        T,
        finite,
        clamp,
        round4,
        runeLength,
        percent,
        idOf,
        domKey,
        zoneNameFold,
        validZoneName,
        serverZone,
        uiLocale,
        parseTime,
        sameDay,
        formatClock,
        formatMoment,
        formatDateTime,
        formatDay,
        formatDuration,
        dayKey,
        dayRange,
        weekdayLabel,
        framesOf,
        cameraDefaults,
        cameraPayload,
        validateCameraSettings,
        globalDefaults,
        settingsPayload,
        validateGlobalSettings,
        armSummary,
        detectorSummary,
        cameraChipLabel,
        channelLabel,
        upsertEvents,
        mergeArmLog,
        eventMatches,
        timelineEntries,
        mainKind,
        svgNumber,
        boxesSvg,
        pointsAttr,
        validPoints,
        zoneEditorSvg,
        nextZoneName,
        snapEdge,
        normalizedPoint,
        isFlatZone,
        isNearPoint,
        launchTarget,
        progressMarkup,
        chipMarkup,
        stripMarkup,
        emptyMarkup,
        cameraOptions,
        alertBadgesMarkup,
        eventRowMarkup,
        armRowMarkup,
        timelineMarkup,
        detailMarkup,
        scheduleRowsMarkup,
        settingsMarkup,
        zoneListMarkup,
        cameraSettingsMarkup
    });
})();
