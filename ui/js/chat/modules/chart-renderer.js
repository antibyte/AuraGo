// AuraGo Chat — lazy Chart.js renderer for fenced ```chart code blocks.
(function () {
    'use strict';

    const ChartRenderer = {
        loadingPromise: null,

        async loadChartJS() {
            if (window.Chart) return;
            if (this.loadingPromise) return this.loadingPromise;
            this.loadingPromise = new Promise((resolve, reject) => {
                const script = document.createElement('script');
                script.src = '/chart.min.js';
                script.async = true;
                script.onload = () => resolve();
                script.onerror = () => reject(new Error('Failed to load Chart.js'));
                document.head.appendChild(script);
            });
            return this.loadingPromise;
        },

        processBlocks(container = document) {
            const blocks = Array.from(container.querySelectorAll('.chart-raw:not([data-chart-rendered])'));
            blocks.forEach(block => {
                block.setAttribute('data-chart-rendered', 'pending');
                this.renderBlock(block);
            });
        },

        async renderBlock(block) {
            const raw = (block.querySelector('pre')?.textContent || block.textContent || '').trim();
            const config = this.parseConfig(raw);
            if (!config) {
                block.innerHTML = '<pre class="chart-error">' + this.escapeHtml(raw || 'Invalid chart data') + '</pre>';
                block.setAttribute('data-chart-rendered', 'error');
                return;
            }

            block.className = 'chart-container';
            block.innerHTML = '<div class="chart-canvas-wrap"><canvas></canvas></div>';

            try {
                await this.loadChartJS();
                const canvas = block.querySelector('canvas');
                if (!canvas || !window.Chart) throw new Error('Chart.js unavailable');
                new window.Chart(canvas.getContext('2d'), config);
                block.setAttribute('data-chart-rendered', 'done');
            } catch (err) {
                block.className = 'chart-container chart-container-error';
                block.innerHTML = '<pre class="chart-error">' + this.escapeHtml(err.message || 'Chart render failed') + '</pre>';
                block.setAttribute('data-chart-rendered', 'error');
            }
        },

        parseConfig(raw) {
            let parsed;
            try {
                parsed = JSON.parse(raw);
            } catch (_) {
                return null;
            }
            if (!parsed || typeof parsed !== 'object') return null;

            let config = parsed;
            if (!parsed.type && Array.isArray(parsed.labels) && Array.isArray(parsed.values)) {
                config = {
                    type: parsed.chart_type || 'bar',
                    data: {
                        labels: parsed.labels,
                        datasets: [{
                            label: parsed.label || 'Value',
                            data: parsed.values
                        }]
                    },
                    options: parsed.options || {}
                };
            }
            if (!config.type || !config.data || typeof config.data !== 'object') return null;

            return {
                type: config.type,
                data: config.data,
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: {
                        legend: {
                            display: this.hasMultipleDatasets(config.data)
                        }
                    },
                    ...(config.options || {})
                }
            };
        },

        hasMultipleDatasets(data) {
            return Array.isArray(data?.datasets) && data.datasets.length > 1;
        },

        escapeHtml(value) {
            const div = document.createElement('div');
            div.textContent = String(value == null ? '' : value);
            return div.innerHTML;
        }
    };

    window.ChatChartRenderer = ChartRenderer;
})();
