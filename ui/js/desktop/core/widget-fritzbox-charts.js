    /* Fritz!Box widget chart helpers. Every function here is pure: it turns
       numeric series into SVG markup or formatted strings and never touches
       the DOM or data from the router other than numbers. Text that comes
       from the Fritz!Box (host names, SSIDs, caller names) is rendered by the
       runtime through textContent only. */
    function fritzNumberFormat(maximumFractionDigits) {
        return new Intl.NumberFormat(document.documentElement.lang || undefined, {
            maximumFractionDigits,
            minimumFractionDigits: 0
        });
    }

    /* Bit rates use decimal SI prefixes like the FRITZ!OS web interface. */
    function fritzSplitBits(bps) {
        const value = Math.max(0, Number(bps) || 0);
        const units = ['bit/s', 'kbit/s', 'Mbit/s', 'Gbit/s'];
        let size = value;
        let unit = 0;
        while (size >= 1000 && unit < units.length - 1) {
            size /= 1000;
            unit += 1;
        }
        const digits = unit === 0 ? 0 : (size >= 100 ? 0 : (size >= 10 ? 1 : 2));
        return { number: fritzNumberFormat(digits).format(size), unit: units[unit] };
    }

    function fritzFormatBits(bps) {
        const parts = fritzSplitBits(bps);
        return `${parts.number} ${parts.unit}`;
    }

    /* Short axis labels: "50 Mbit", "800 kbit", "2,5 Gbit" (per second is implied by the legend). */
    function fritzFormatBitsShort(bps) {
        const value = Math.max(0, Number(bps) || 0);
        if (value === 0) return '0';
        const units = ['bit', 'kbit', 'Mbit', 'Gbit'];
        let size = value;
        let unit = 0;
        while (size >= 1000 && unit < units.length - 1) {
            size /= 1000;
            unit += 1;
        }
        return `${fritzNumberFormat(size >= 10 ? 0 : 1).format(size)} ${units[unit]}`;
    }

    /* Nice ceiling (1, 2, 5 x 10^n) so grid lines land on readable values. */
    function fritzNiceMax(value) {
        const v = Math.max(1, Number(value) || 0);
        const magnitude = Math.pow(10, Math.floor(Math.log10(v)));
        const normalized = v / magnitude;
        let nice = 10;
        if (normalized <= 1) nice = 1;
        else if (normalized <= 2) nice = 2;
        else if (normalized <= 2.5) nice = 2.5;
        else if (normalized <= 5) nice = 5;
        return nice * magnitude;
    }

    function fritzFormatPercent(ratio) {
        const clamped = Math.max(0, Math.min(1, Number(ratio) || 0));
        return new Intl.NumberFormat(document.documentElement.lang || undefined, {
            style: 'percent',
            maximumFractionDigits: clamped < 0.1 ? 1 : 0
        }).format(clamped);
    }

    /* "H:MM" call durations from the box become minutes/hours. */
    function fritzFormatCallDuration(raw) {
        const match = /^(\d+):(\d{1,2})$/.exec(String(raw || '').trim());
        if (!match) return String(raw || '').trim();
        const hours = Number(match[1]);
        const minutes = Number(match[2]);
        if (hours > 0) return `${hours} h ${minutes} min`;
        return `${minutes} min`;
    }

    /* Relative time for call timestamps; falls back to the raw date text. */
    function fritzRelativeTime(timestamp, fallback, now) {
        const time = Date.parse(timestamp || '');
        if (!Number.isFinite(time)) return String(fallback || '');
        const diffMs = time - (now || Date.now());
        const abs = Math.abs(diffMs);
        const lang = document.documentElement.lang || undefined;
        if (abs >= 6 * 24 * 3600 * 1000) {
            return new Intl.DateTimeFormat(lang, { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' }).format(new Date(time));
        }
        const rtf = new Intl.RelativeTimeFormat(lang, { numeric: 'auto' });
        if (abs < 60 * 1000) return rtf.format(Math.round(diffMs / 1000), 'second');
        if (abs < 3600 * 1000) return rtf.format(Math.round(diffMs / 60000), 'minute');
        if (abs < 24 * 3600 * 1000) return rtf.format(Math.round(diffMs / 3600000), 'hour');
        return rtf.format(Math.round(diffMs / 86400000), 'day');
    }

    function fritzSvgNumber(value) {
        return Number.isFinite(value) ? Number(value.toFixed(2)) : 0;
    }

    /* Monotone cubic interpolation (Fritsch-Carlson) keeps curves smooth without
       overshooting below zero. Segments split at null gaps. */
    function fritzSmoothPath(points) {
        let d = '';
        let segment = [];
        const flush = () => {
            if (segment.length === 1) {
                d += `M${segment[0].x},${segment[0].y} L${segment[0].x},${segment[0].y}`;
            } else if (segment.length > 1) {
                const n = segment.length;
                const slopes = new Array(n).fill(0);
                const deltas = [];
                for (let i = 0; i < n - 1; i++) {
                    const dx = segment[i + 1].x - segment[i].x || 1e-6;
                    deltas.push((segment[i + 1].y - segment[i].y) / dx);
                }
                slopes[0] = deltas[0];
                slopes[n - 1] = deltas[n - 2];
                for (let i = 1; i < n - 1; i++) {
                    slopes[i] = deltas[i - 1] * deltas[i] <= 0 ? 0 : (deltas[i - 1] + deltas[i]) / 2;
                }
                d += `M${segment[0].x},${segment[0].y}`;
                for (let i = 0; i < n - 1; i++) {
                    const p0 = segment[i];
                    const p1 = segment[i + 1];
                    const dx = (p1.x - p0.x) / 3;
                    d += ` C${fritzSvgNumber(p0.x + dx)},${fritzSvgNumber(p0.y + slopes[i] * dx)} ${fritzSvgNumber(p1.x - dx)},${fritzSvgNumber(p1.y - slopes[i + 1] * dx)} ${p1.x},${p1.y}`;
                }
            }
            segment = [];
        };
        for (const point of points) {
            if (point === null) {
                flush();
            } else {
                segment.push(point);
            }
        }
        flush();
        return d;
    }

    /* Closes a smooth line path down to the baseline for area fills. */
    function fritzAreaPath(points, baseline) {
        let d = '';
        let segment = [];
        const flush = () => {
            if (segment.length) {
                const line = fritzSmoothPath(segment);
                const first = segment[0];
                const last = segment[segment.length - 1];
                d += `${line} L${last.x},${baseline} L${first.x},${baseline} Z `;
            }
            segment = [];
        };
        for (const point of points) {
            if (point === null) flush();
            else segment.push(point);
        }
        flush();
        return d.trim();
    }

    /* Projects {t, down, up} samples onto chart coordinates. Gaps larger than
       gapMs become null breaks so pauses (hidden tab) are visible, not faked. */
    function fritzProjectSeries(samples, key, range, width, height, max, gapMs) {
        const span = Math.max(1, range.end - range.start);
        const points = [];
        let previous = null;
        for (const sample of samples) {
            const value = Number(sample[key]);
            if (!Number.isFinite(value)) continue;
            if (previous !== null && sample.t - previous > gapMs) points.push(null);
            const x = fritzSvgNumber(((sample.t - range.start) / span) * width);
            const y = fritzSvgNumber(height - Math.min(1, Math.max(0, value / max)) * height);
            points.push({ x, y, value });
            previous = sample.t;
        }
        return points;
    }

    /* Dual area chart (download + upload) with grid lines and value labels.
       Returns markup plus the scale max so the runtime can label the legend. */
    function fritzAreaChartSVG(options) {
        const samples = Array.isArray(options.samples) ? options.samples : [];
        const width = options.width || 300;
        const height = options.height || 110;
        const padTop = 6;
        const padRight = options.compact ? 4 : 50;
        const plotW = Math.max(10, width - padRight);
        const plotH = Math.max(10, height - padTop - 4);
        const gapMs = options.gapMs || 15000;
        const idPrefix = String(options.idPrefix || 'fritz').replace(/[^a-z0-9_-]/gi, '');
        const observed = samples.reduce((acc, sample) => Math.max(acc, Number(sample.down) || 0, Number(sample.up) || 0), 0);
        const max = fritzNiceMax(Math.max(observed * 1.15, options.minMax || 1000));
        const range = options.range || {
            start: samples.length ? samples[0].t : 0,
            end: samples.length ? samples[samples.length - 1].t : 1
        };
        const down = fritzProjectSeries(samples, 'down', range, plotW, plotH, max, gapMs);
        const up = fritzProjectSeries(samples, 'up', range, plotW, plotH, max, gapMs);
        const gridLevels = options.compact ? [0.5] : [0.25, 0.5, 0.75, 1];
        let grid = '';
        for (const level of gridLevels) {
            const y = fritzSvgNumber(padTop + plotH - level * plotH);
            grid += `<line class="vd-fritz-chart-grid" x1="0" y1="${y}" x2="${plotW}" y2="${y}"></line>`;
            if (!options.compact) {
                grid += `<text class="vd-fritz-chart-tick" x="${plotW + 4}" y="${y + 3}">${fritzFormatBitsShort(level * max)}</text>`;
            }
        }
        const shift = points => points.map(point => point === null ? null : { x: point.x, y: fritzSvgNumber(point.y + padTop), value: point.value });
        const downShifted = shift(down);
        const upShifted = shift(up);
        const baseline = fritzSvgNumber(padTop + plotH);
        let peak = '';
        let peakPoint = null;
        for (const point of downShifted) {
            if (point && (!peakPoint || point.value > peakPoint.value)) peakPoint = point;
        }
        if (peakPoint && peakPoint.value > 0 && !options.compact) {
            peak = `<circle class="vd-fritz-chart-peak" cx="${peakPoint.x}" cy="${peakPoint.y}" r="2.6"></circle>`;
        }
        const svg = `<svg class="vd-fritz-chart-svg" viewBox="0 0 ${width} ${height}" width="100%" height="${height}" preserveAspectRatio="none" role="img" aria-hidden="true">
            <defs>
                <linearGradient id="${idPrefix}-down" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" class="vd-fritz-grad-down-start"></stop>
                    <stop offset="100%" class="vd-fritz-grad-down-end"></stop>
                </linearGradient>
                <linearGradient id="${idPrefix}-up" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" class="vd-fritz-grad-up-start"></stop>
                    <stop offset="100%" class="vd-fritz-grad-up-end"></stop>
                </linearGradient>
            </defs>
            ${grid}
            <path class="vd-fritz-chart-area is-down" fill="url(#${idPrefix}-down)" d="${fritzAreaPath(downShifted, baseline)}"></path>
            <path class="vd-fritz-chart-area is-up" fill="url(#${idPrefix}-up)" d="${fritzAreaPath(upShifted, baseline)}"></path>
            <path class="vd-fritz-chart-line is-up" d="${fritzSmoothPath(upShifted)}"></path>
            <path class="vd-fritz-chart-line is-down" d="${fritzSmoothPath(downShifted)}"></path>
            ${peak}
        </svg>`;
        return { svg, max, plotWidth: plotW, plotHeight: plotH, padTop, range };
    }

    /* Tiny trend line for the connection page KPIs. */
    function fritzSparkSVG(values, width, height, className) {
        const list = (Array.isArray(values) ? values : []).map(v => Number(v)).filter(v => Number.isFinite(v));
        if (list.length < 2) return `<svg class="vd-fritz-spark-svg ${className || ''}" viewBox="0 0 ${width} ${height}" preserveAspectRatio="none" aria-hidden="true"></svg>`;
        const max = Math.max(1, ...list);
        const step = width / (list.length - 1);
        const points = list.map((value, index) => ({
            x: fritzSvgNumber(index * step),
            y: fritzSvgNumber(height - 1 - (value / max) * (height - 2))
        }));
        return `<svg class="vd-fritz-spark-svg ${className || ''}" viewBox="0 0 ${width} ${height}" preserveAspectRatio="none" aria-hidden="true">
            <path class="vd-fritz-spark-area" d="${fritzAreaPath(points, height)}"></path>
            <path class="vd-fritz-spark-line" d="${fritzSmoothPath(points)}"></path>
        </svg>`;
    }

    /* Ring chart for "active of total" counts. Only numbers reach the markup. */
    function fritzRingSVG(value, total, size) {
        const radius = (size - 8) / 2;
        const circumference = 2 * Math.PI * radius;
        const ratio = total > 0 ? Math.max(0, Math.min(1, value / total)) : 0;
        const dash = fritzSvgNumber(circumference * ratio);
        const center = size / 2;
        return `<svg class="vd-fritz-ring-svg" viewBox="0 0 ${size} ${size}" width="${size}" height="${size}" aria-hidden="true">
            <circle class="vd-fritz-ring-track" cx="${center}" cy="${center}" r="${fritzSvgNumber(radius)}"></circle>
            <circle class="vd-fritz-ring-value" cx="${center}" cy="${center}" r="${fritzSvgNumber(radius)}" stroke-dasharray="${dash} ${fritzSvgNumber(circumference)}" transform="rotate(-90 ${center} ${center})"></circle>
            <text class="vd-fritz-ring-number" x="${center}" y="${center + 1}" text-anchor="middle" dominant-baseline="middle">${Math.max(0, Math.round(Number(value) || 0))}</text>
        </svg>`;
    }

    /* Semi-circular gauge for line utilization. */
    function fritzGaugeSVG(ratio, size, className) {
        const clamped = Math.max(0, Math.min(1, Number(ratio) || 0));
        const stroke = 7;
        const radius = (size - stroke) / 2;
        const center = size / 2;
        const arcLength = Math.PI * radius;
        const start = `${fritzSvgNumber(center - radius)},${fritzSvgNumber(center)}`;
        const end = `${fritzSvgNumber(center + radius)},${fritzSvgNumber(center)}`;
        const path = `M${start} A${fritzSvgNumber(radius)},${fritzSvgNumber(radius)} 0 0 1 ${end}`;
        return `<svg class="vd-fritz-gauge-svg ${className || ''}" viewBox="0 0 ${size} ${center + stroke}" width="${size}" height="${center + stroke}" aria-hidden="true">
            <path class="vd-fritz-gauge-track" d="${path}"></path>
            <path class="vd-fritz-gauge-value" d="${path}" stroke-dasharray="${fritzSvgNumber(arcLength * clamped)} ${fritzSvgNumber(arcLength)}"></path>
        </svg>`;
    }

    /* Maps a pointer x position inside the chart to the nearest sample index. */
    function fritzChartIndexAt(samples, range, plotWidth, x) {
        if (!samples.length) return -1;
        const span = Math.max(1, range.end - range.start);
        const time = range.start + (Math.max(0, Math.min(plotWidth, x)) / plotWidth) * span;
        let best = 0;
        let bestDistance = Infinity;
        for (let i = 0; i < samples.length; i++) {
            const distance = Math.abs(samples[i].t - time);
            if (distance < bestDistance) {
                bestDistance = distance;
                best = i;
            }
        }
        return best;
    }

    /* Ring buffer helper: merges monitor samples (oldest first, fixed interval)
       ending at `end` into the existing history without duplicating times. */
    function fritzMergeMonitorSamples(history, downSeries, upSeries, intervalMs, end, capacity) {
        const count = Math.min(downSeries.length, upSeries.length);
        const tolerance = intervalMs / 2;
        let last = history.length ? history[history.length - 1].t : -Infinity;
        for (let i = 0; i < count; i++) {
            const t = end - (count - 1 - i) * intervalMs;
            if (t <= last + tolerance) continue;
            history.push({ t, down: Math.max(0, Number(downSeries[i]) || 0), up: Math.max(0, Number(upSeries[i]) || 0) });
            last = t;
        }
        while (history.length > capacity) history.shift();
        return history;
    }
