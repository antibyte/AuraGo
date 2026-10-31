// TrueNAS UI Controller

// Escape HTML entities to prevent XSS when inserting server-provided data into the DOM.
function escapeHtmlTruenas(str) {
    if (str === null || str === undefined) return '';
    return String(str)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#039;');
}

function poolBadgeStatus(status) {
    const s = String(status || '').toUpperCase();
    if (s === 'ONLINE' || s === 'HEALTHY') return 'ok';
    if (s === 'DEGRADED') return 'warn';
    return 'error';
}

function badgeHtml(status, text) {
    return `<span class="pw-badge" data-status="${escapeHtmlTruenas(status)}">${text}</span>`;
}

function truenasLoadingHtml(key) {
    return `<div class="pw-loading-state" role="status"><span class="pw-skeleton skeleton" aria-hidden="true"></span><span>${t(key)}</span></div>`;
}

function truenasEmptyHtml(name) {
    return `<div class="pw-empty-state"><h3>${t('truenas.empty_' + name)}</h3><p>${t('truenas.empty_' + name + '_desc')}</p></div>`;
}

function truenasNotConfiguredHtml() {
    return '<div class="pw-empty-state"><h3>' + t('truenas.not_configured_title') + '</h3><p>' + t('truenas.not_configured_desc') +
        '</p><a class="btn btn-primary" href="/config#truenas">' + t('truenas.not_configured_link') + '</a></div>';
}

// Parse a JSON response; a non-JSON body (proxy page, empty reply) becomes a plain HTTP error, never a raw parse message.
async function truenasJSON(response) {
    try {
        return await response.json();
    } catch (_) {
        throw new Error('HTTP ' + response.status);
    }
}

class TrueNASUI {
    constructor() {
        this.baseUrl = '/api/truenas';
        this.currentTab = 'overview';
        this.pools = [];
        this.datasets = [];
        this.notConfigured = false;
        this.init();
    }
    
    init() {
        if (typeof window._auragoApplySharedI18n === 'function') {
            window._auragoApplySharedI18n();
        }
        this.bindEvents();
        this.checkStatus();
        this.startHealthCheck();
    }
    
    bindEvents() {
        // Tab Navigation
        const tabButtons = Array.from(document.querySelectorAll('.nav-btn[role="tab"]'));
        tabButtons.forEach(btn => {
            btn.addEventListener('click', () => this.switchTab(btn.dataset.tab));
            btn.addEventListener('keydown', (event) => {
                const index = tabButtons.indexOf(btn);
                let next = -1;
                if (event.key === 'ArrowRight') next = (index + 1) % tabButtons.length;
                else if (event.key === 'ArrowLeft') next = (index - 1 + tabButtons.length) % tabButtons.length;
                else if (event.key === 'Home') next = 0;
                else if (event.key === 'End') next = tabButtons.length - 1;
                if (next < 0) return;
                event.preventDefault();
                tabButtons[next].focus();
                this.switchTab(tabButtons[next].dataset.tab);
            });
        });

        document.getElementById('refresh-pools-btn')?.addEventListener('click', () => this.loadPools());
        document.getElementById('create-dataset-btn')?.addEventListener('click', () => this.showCreateDataset());
        document.getElementById('create-snapshot-btn')?.addEventListener('click', () => this.showCreateSnapshot());
        document.getElementById('create-share-btn')?.addEventListener('click', () => this.showCreateShare());
        document.getElementById('test-connection-btn')?.addEventListener('click', () => this.testConnection());
        document.getElementById('snapshot-filter')?.addEventListener('change', () => this.loadSnapshots());
        document.querySelectorAll('[data-close-modal]').forEach(btn => {
            btn.addEventListener('click', () => this.closeModal());
        });
        // Esc, backdrop click and the focus trap come from the shared modal controller;
        // only clear the per-dialog error boxes when a dialog closes that way.
        document.querySelectorAll('.modal-overlay').forEach(modal => {
            new MutationObserver(() => {
                if (!modal.classList.contains('active')) modal.querySelectorAll('[id$="-error"]').forEach(el => { el.innerHTML = ''; });
            }).observe(modal, { attributes: true, attributeFilter: ['class'] });
        });
        document.addEventListener('click', (event) => {
            const actionBtn = event.target.closest('[data-truenas-action]');
            if (!actionBtn) return;

            switch (actionBtn.dataset.truenasAction) {
                case 'scrub-pool':
                    this.scrubPool(actionBtn.dataset.poolId);
                    break;
                case 'delete-dataset':
                    this.deleteDataset(actionBtn.dataset.name);
                    break;
                case 'rollback-snapshot':
                    this.rollbackSnapshot(actionBtn.dataset.name);
                    break;
                case 'delete-snapshot':
                    this.deleteSnapshot(actionBtn.dataset.name);
                    break;
                case 'delete-share':
                    this.deleteShare(actionBtn.dataset.shareId);
                    break;
                case 'delete-nfs-share':
                    this.deleteNFSShare(actionBtn.dataset.shareId);
                    break;
            }
        });
        
        // Forms
        document.getElementById('truenas-settings-form')?.addEventListener('submit', (e) => this.saveSettings(e));
        document.getElementById('dataset-form')?.addEventListener('submit', (e) => this.createDataset(e));
        document.getElementById('snapshot-form')?.addEventListener('submit', (e) => this.createSnapshot(e));
        document.getElementById('share-form')?.addEventListener('submit', (e) => this.createShare(e));
        document.getElementById('share-type')?.addEventListener('change', () => this.toggleShareFields());
        
    }
    
    async switchTab(tabName) {
        this.currentTab = tabName;
        
        // Update nav
        document.querySelectorAll('.nav-btn[role="tab"]').forEach(b => {
            const selected = b.dataset.tab === tabName;
            b.classList.toggle('active', selected);
            b.setAttribute('aria-selected', selected ? 'true' : 'false');
            b.setAttribute('tabindex', selected ? '0' : '-1');
        });
        
        // Update content
        document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
        document.getElementById(`tab-${tabName}`)?.classList.add('active');
        
        // Without a configured TrueNAS the data tabs show the empty state and never fetch.
        const containers = { pools: 'pools-container', datasets: 'datasets-container', snapshots: 'snapshots-container', shares: 'shares-container' };
        if (this.notConfigured && containers[tabName]) {
            const box = document.getElementById(containers[tabName]);
            if (box) box.innerHTML = truenasNotConfiguredHtml();
            return;
        }

        // Load data
        switch(tabName) {
            case 'overview': 
                await this.loadOverview(); 
                break;
            case 'pools': 
                await this.loadPools(); 
                break;
            case 'datasets': 
                await this.loadDatasets(); 
                break;
            case 'snapshots': 
                await this.loadSnapshots(); 
                break;
            case 'shares': 
                await this.loadShares(); 
                break;
            case 'settings': 
                await this.loadSettings(); 
                break;
        }
    }
    
    setNotConfigured(on) {
        this.notConfigured = !!on;
        document.getElementById('truenas-not-configured')?.classList.toggle('is-hidden', !on);
        document.getElementById('truenas-stats-grid')?.classList.toggle('is-hidden', on);
        if (on) this.resetOverviewStats();
    }

    resetOverviewStats() {
        ['pool-count', 'total-storage', 'snapshot-count', 'share-count'].forEach(id => {
            const el = document.getElementById(id);
            if (el) el.textContent = '-';
        });
        const progressEl = document.getElementById('storage-progress');
        if (progressEl) {
            progressEl.style.width = '0%';
            progressEl.className = 'progress-fill';
        }
    }

    async checkStatus() {
        try {
            const response = await fetch(`${this.baseUrl}/status`);
            let data = null;
            try { data = await response.json(); } catch (_) { data = null; }
            if (!data && (response.status === 403 || response.status === 404)) {
                data = { enabled: false, status: 'disabled' };
            }
            if (!data) throw new Error('HTTP ' + response.status);

            const indicator = document.getElementById('status-indicator');
            this.setNotConfigured(data.enabled === false);
            if (data.enabled === false) {
                indicator.innerHTML = badgeHtml('idle', t('truenas.status_not_configured'));
            } else if (data.status === 'online') {
                indicator.innerHTML = badgeHtml('ok', t('truenas.status_online')) + ` (${escapeHtmlTruenas(data.version)})`;
                this.loadOverview();
            } else if (data.status === 'error') {
                indicator.innerHTML = badgeHtml('error', `${t('truenas.status_error_prefix')} ${escapeHtmlTruenas(data.error)}`);
            } else {
                indicator.innerHTML = badgeHtml('error', t('truenas.status_offline'));
            }
            return data;
        } catch (err) {
            document.getElementById('status-indicator').innerHTML = 
                badgeHtml('error', t('truenas.status_connection_error'));
            return { status: 'error', error: t('truenas.status_connection_error') };
        }
    }
    
    async loadOverview() {
        try {
            const response = await fetch(`${this.baseUrl}/health`);
            const data = await truenasJSON(response);
            
            if (data.error) {
                this.showError('overview-error', data.error);
                return;
            }
            
            // Update stats
            const pools = data.pools || [];
            const alerts = data.alerts || [];
            
            document.getElementById('pool-count').textContent = pools.length;
            
            // Pool status summary
            const degraded = pools.filter(p => p.status !== 'ONLINE').length;
            const poolStatusEl = document.getElementById('pool-status');
            if (degraded > 0) {
                poolStatusEl.innerHTML = badgeHtml('error', `${degraded} ${t('truenas.pool_degraded')}`);
            } else {
                poolStatusEl.innerHTML = badgeHtml('ok', t('truenas.pool_all_ok'));
            }
            
            // Storage calculation
            let total = 0, used = 0;
            pools.forEach(p => {
                total += p.size?.total || 0;
                used += p.size?.allocated || 0;
            });
            
            document.getElementById('total-storage').textContent = this.formatBytes(total);
            const percent = total > 0 ? (used / total * 100).toFixed(1) : 0;
            const progressEl = document.getElementById('storage-progress');
            progressEl.style.width = total > 0 ? `${percent}%` : '0%';
            progressEl.className = `progress-fill ${percent > 90 ? 'danger' : percent > 70 ? 'warning' : ''}`;
            
            // Alerts
            const activeAlerts = alerts.filter(a => !a.dismissed);
            const alertsSection = document.getElementById('alerts-section');
            if (activeAlerts.length > 0) {
                alertsSection.classList.remove('is-hidden');
                const container = document.getElementById('alerts-container');
                container.innerHTML = activeAlerts.map(a => `
                    <div class="alert-item ${escapeHtmlTruenas(a.level.toLowerCase())}">
                        <h4>${escapeHtmlTruenas(a.title)}</h4>
                        <p>${escapeHtmlTruenas(a.message)}</p>
                        <div class="alert-date">${new Date(a.date).toLocaleString()}</div>
                    </div>
                `).join('');
            } else if (alertsSection) {
                alertsSection.classList.add('is-hidden');
            }

            await this.updateShareCounts();

        } catch (err) {
            this.showError('overview-error', t('truenas.error_load_overview') + ' ' + err.message);
        }
    }
    
    async loadPools() {
        const container = document.getElementById('pools-container');
        container.innerHTML = truenasLoadingHtml('truenas.loading_pools');
        
        try {
            const response = await fetch(`${this.baseUrl}/pools`);
            const data = await truenasJSON(response);
            
            if (!data.pools || data.pools.length === 0) {
                container.innerHTML = truenasEmptyHtml('pools');
                return;
            }
            
            this.pools = data.pools;
            container.innerHTML = data.pools.map(pool => {
                const usage = pool.size?.total > 0 ? (pool.size.allocated / pool.size.total * 100).toFixed(1) : 0;
                const safeStatus = escapeHtmlTruenas(pool.status.toLowerCase());
                const scrubLabel = pool.scan?.state === 'SCANNING' ? t('truenas.scrub_running') : t('truenas.scrub_start');
                return `
                    <div class="pool-card ${safeStatus}">
                        <div class="pool-header">
                            <h3>${escapeHtmlTruenas(pool.name)}</h3>
                            ${badgeHtml(poolBadgeStatus(pool.status), escapeHtmlTruenas(pool.status))}
                        </div>
                        <div class="pool-stats">
                            <div class="stat">
                                <label>${t('truenas.pool_size')}</label>
                                <value>${this.formatBytes(pool.size?.total)}</value>
                            </div>
                            <div class="stat">
                                <label>${t('truenas.pool_allocated')}</label>
                                <value>${this.formatBytes(pool.size?.allocated)}</value>
                            </div>
                            <div class="stat">
                                <label>${t('truenas.pool_free')}</label>
                                <value>${this.formatBytes(pool.size?.free)}</value>
                            </div>
                        </div>
                        <div class="progress-bar">
                            <div class="progress-fill ${usage > 90 ? 'danger' : usage > 70 ? 'warning' : ''}" style="width: ${usage}%"></div>
                        </div>
                        <div class="pool-actions">
                            <button class="btn btn-secondary" data-truenas-action="scrub-pool" data-pool-id="${escapeHtmlTruenas(pool.id)}" ${pool.scan?.state === 'SCANNING' ? 'disabled' : ''}>
                                ${scrubLabel}
                            </button>
                        </div>
                    </div>
                `;
            }).join('');
            
        } catch (err) {
            container.innerHTML = `<div class="alert error">${t('truenas.error_connection')} ${escapeHtmlTruenas(err.message)}</div>`;
        }
    }
    
    async loadDatasets() {
        const container = document.getElementById('datasets-container');
        container.innerHTML = truenasLoadingHtml('truenas.loading_datasets');
        
        try {
            const response = await fetch(`${this.baseUrl}/datasets`);
            const data = await truenasJSON(response);
            
            if (!data.datasets || data.datasets.length === 0) {
                container.innerHTML = truenasEmptyHtml('datasets');
                return;
            }
            
            this.datasets = data.datasets;
            container.innerHTML = data.datasets.map(ds => {
                const used = ds.used?.parsed || 0;
                const available = ds.available?.parsed || 0;
                const total = used + available;
                const usage = total > 0 ? (used / total * 100).toFixed(1) : 0;
                
                return `
                    <div class="dataset-item">
                        <div class="dataset-info">
                            <h4>${escapeHtmlTruenas(ds.name)}</h4>
                            <p>${this.formatBytes(used)} / ${this.formatBytes(total)} ${t('truenas.dataset_used')} (${usage}%) • ${t('truenas.dataset_compression')}: ${escapeHtmlTruenas(ds.compression?.parsed || 'off')}</p>
                        </div>
                        <div class="dataset-actions">
                            <button class="btn btn-danger" data-truenas-action="delete-dataset" data-name="${escapeHtmlTruenas(ds.name)}">${t('truenas.btn_delete')}</button>
                        </div>
                    </div>
                `;
            }).join('');
            
            // Update snapshot filter dropdown
            const filterSelect = document.getElementById('snapshot-filter');
            if (filterSelect) {
                filterSelect.innerHTML = `<option value="">${t('truenas.filter_all_datasets')}</option>` + 
                    data.datasets.map(ds => `<option value="${escapeHtmlTruenas(ds.name)}">${escapeHtmlTruenas(ds.name)}</option>`).join('');
            }
            // Update snapshot create dropdown
            const createSelect = document.getElementById('snapshot-dataset');
            if (createSelect) {
                createSelect.innerHTML = `<option value="">${t('truenas.select_placeholder')}</option>` + 
                    data.datasets.map(ds => `<option value="${escapeHtmlTruenas(ds.name)}">${escapeHtmlTruenas(ds.name)}</option>`).join('');
            }
            
        } catch (err) {
            container.innerHTML = `<div class="alert error">${t('truenas.error_connection')} ${escapeHtmlTruenas(err.message)}</div>`;
        }
    }
    
    async loadSnapshots() {
        const container = document.getElementById('snapshots-container');
        container.innerHTML = truenasLoadingHtml('truenas.loading_snapshots');
        
        try {
            const filter = document.getElementById('snapshot-filter')?.value || '';
            const url = filter ? `${this.baseUrl}/snapshots?dataset=${encodeURIComponent(filter)}` : `${this.baseUrl}/snapshots`;
            const response = await fetch(url);
            const data = await truenasJSON(response);
            
            if (!data.snapshots || data.snapshots.length === 0) {
                container.innerHTML = truenasEmptyHtml('snapshots');
                return;
            }
            
            container.innerHTML = data.snapshots.map(snap => {
                const age = this.snapshotAge(snap);
                return `
                    <div class="snapshot-item">
                        <div class="snapshot-info">
                            <h4>${escapeHtmlTruenas(snap.name)}</h4>
                            <p>${escapeHtmlTruenas(snap.dataset)} • ${this.formatBytes(snap.properties?.used?.parsed || 0)} • ${t('truenas.snapshot_ago')} ${age}</p>
                        </div>
                        <div class="snapshot-actions">
                            <button class="btn btn-secondary" data-truenas-action="rollback-snapshot" data-name="${escapeHtmlTruenas(snap.name)}">${t('truenas.btn_rollback')}</button>
                            <button class="btn btn-danger" data-truenas-action="delete-snapshot" data-name="${escapeHtmlTruenas(snap.name)}">${t('truenas.btn_delete')}</button>
                        </div>
                    </div>
                `;
            }).join('');
            
        } catch (err) {
            container.innerHTML = `<div class="alert error">${t('truenas.error_connection')} ${escapeHtmlTruenas(err.message)}</div>`;
        }
    }
    
    async loadShares() {
        const container = document.getElementById('shares-container');
        container.innerHTML = truenasLoadingHtml('truenas.loading_shares');

        try {
            const [smbResponse, nfsResponse] = await Promise.all([
                fetch(`${this.baseUrl}/shares/smb`),
                fetch(`${this.baseUrl}/shares/nfs`)
            ]);
            const smbData = await truenasJSON(smbResponse);
            const nfsData = await truenasJSON(nfsResponse);
            const smbShares = smbData.shares || [];
            const nfsShares = nfsData.shares || [];
            const shares = [
                ...smbShares.map(share => ({ ...share, share_type: 'smb' })),
                ...nfsShares.map(share => ({ ...share, share_type: 'nfs' }))
            ];

            this.setShareCounts(smbShares, nfsShares);
            
            if (shares.length === 0) {
                container.innerHTML = truenasEmptyHtml('shares');
                return;
            }
            
            container.innerHTML = shares.map(share => {
                const isNFS = share.share_type === 'nfs';
                const guestLabel = !isNFS && share.guestok ? ` • ${t('truenas.share_guest')}` : '';
                const tmLabel = !isNFS && share.timemachine ? ` • ${t('truenas.share_timemachine')}` : '';
                const networks = isNFS && share.networks?.length ? ` • ${escapeHtmlTruenas(share.networks.join(', '))}` : '';
                const hosts = isNFS && share.hosts?.length ? ` • ${escapeHtmlTruenas(share.hosts.join(', '))}` : '';
                const title = isNFS ? share.path : share.name;
                const action = isNFS ? 'delete-nfs-share' : 'delete-share';
                return `
                    <div class="share-item">
                        <div class="share-info">
                            <h4>${escapeHtmlTruenas(title)} <span class="pw-badge">${isNFS ? 'NFS' : 'SMB'}</span></h4>
                            <p>${escapeHtmlTruenas(share.path)}${guestLabel}${tmLabel}${networks}${hosts}</p>
                        </div>
                        <div class="share-actions">
                            <button class="btn btn-danger" data-truenas-action="${action}" data-share-id="${escapeHtmlTruenas(share.id)}">${t('truenas.btn_delete')}</button>
                        </div>
                    </div>
                `;
            }).join('');
            
        } catch (err) {
            container.innerHTML = `<div class="alert error">${t('truenas.error_connection')} ${escapeHtmlTruenas(err.message)}</div>`;
        }
    }
    
    async loadSettings() {
        try {
            // Load current config from main config API
            const response = await fetch('/api/config');
            const data = await truenasJSON(response);
            
            if (data.truenas) {
                document.getElementById('setting-host').value = data.truenas.host || '';
                document.getElementById('setting-https').checked = data.truenas.use_https !== false;
                document.getElementById('setting-insecure').checked = data.truenas.insecure_ssl || false;
                document.getElementById('setting-readonly').checked = data.truenas.readonly || false;
                document.getElementById('setting-destructive').checked = data.truenas.allow_destructive || false;
            }
        } catch (err) {
            console.error('Failed to load settings:', err);
        }
    }
    
    async saveSettings(e) {
        e.preventDefault();
        
        const settings = {
            truenas: {
                enabled: true,
                host: document.getElementById('setting-host').value,
                use_https: document.getElementById('setting-https').checked,
                insecure_ssl: document.getElementById('setting-insecure').checked,
                readonly: document.getElementById('setting-readonly').checked,
                allow_destructive: document.getElementById('setting-destructive').checked
            }
        };
        
        // API Key to vault
        const apiKey = document.getElementById('setting-apikey').value;
        if (apiKey) {
            try {
                const vaultResponse = await fetch('/api/vault/secrets', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ key: 'truenas_api_key', value: apiKey })
                });
                if (!vaultResponse.ok) {
                    const data = await vaultResponse.json().catch(() => ({}));
                    throw new Error(data.error || data.message || `HTTP ${vaultResponse.status}`);
                }
            } catch (err) {
                this.showError('settings-error', t('truenas.error_save_apikey') + ' ' + err.message);
                return;
            }
        }
        
        try {
            const response = await fetch('/api/config', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(settings)
            });
            
            if (response.ok) {
                showToast(t('truenas.settings_saved'), 'success');
                this.checkStatus();
            } else {
                const data = await response.json();
                this.showError('settings-error', data.error || t('truenas.error_save_settings'));
            }
        } catch (err) {
            this.showError('settings-error', t('truenas.error_connection') + ' ' + err.message);
        }
    }
    
    async testConnection() {
        const btn = document.getElementById('test-connection-btn');
        btn.disabled = true;
        btn.textContent = t('truenas.btn_testing');
        
        try {
            const status = await this.checkStatus();
            if (!status || status.status !== 'online') {
                throw new Error(status.error || t('truenas.connection_failed'));
            }
            showToast(t('truenas.connection_success'), 'success');
        } catch (err) {
            this.showError('settings-error', t('truenas.connection_failed') + ' ' + err.message);
        } finally {
            btn.disabled = false;
            btn.textContent = t('truenas.btn_test_connection');
        }
    }
    
    async createDataset(e) {
        e.preventDefault();
        
        const name = document.getElementById('dataset-name').value;
        const compression = document.getElementById('dataset-compression').value;
        const quota = parseInt(document.getElementById('dataset-quota').value) || 0;
        
        try {
            const response = await fetch(`${this.baseUrl}/datasets`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ name, compression, quota_gb: quota })
            });
            
            const data = await response.json();
            
            if (response.ok) {
                this.closeModal();
                this.loadDatasets();
            } else {
                this.showError('dataset-error', data.error || t('truenas.error_create'));
            }
        } catch (err) {
            this.showError('dataset-error', t('truenas.error_connection') + ' ' + err.message);
        }
    }
    
    async createSnapshot(e) {
        e.preventDefault();
        
        const dataset = document.getElementById('snapshot-dataset').value;
        const name = document.getElementById('snapshot-name').value;
        const recursive = document.getElementById('snapshot-recursive').checked;
        const retention = parseInt(document.getElementById('snapshot-retention').value) || 0;
        
        if (!dataset) {
            this.showError('snapshot-error', t('truenas.select_dataset_first'));
            return;
        }
        
        try {
            const response = await fetch(`${this.baseUrl}/snapshots`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ dataset, name, recursive, retention_days: retention })
            });
            
            const data = await response.json();
            
            if (response.ok) {
                this.closeModal();
                this.loadSnapshots();
            } else {
                this.showError('snapshot-error', data.error || t('truenas.error_create'));
            }
        } catch (err) {
            this.showError('snapshot-error', t('truenas.error_connection') + ' ' + err.message);
        }
    }
    
    async createShare(e) {
        e.preventDefault();

        const type = document.getElementById('share-type')?.value || 'smb';
        if (type === 'nfs') {
            return this.createNFSShare();
        }
        
        const name = document.getElementById('share-name').value;
        const path = document.getElementById('share-path').value;
        const guest_ok = document.getElementById('share-guest').checked;
        const timemachine = document.getElementById('share-timemachine').checked;
        
        try {
            const response = await fetch(`${this.baseUrl}/shares/smb`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ name, path, guest_ok, timemachine })
            });
            
            const data = await response.json();
            
            if (response.ok) {
                this.closeModal();
                this.loadShares();
            } else {
                this.showError('share-error', data.error || t('truenas.error_create'));
            }
        } catch (err) {
            this.showError('share-error', t('truenas.error_connection') + ' ' + err.message);
        }
    }

    async createNFSShare() {
        const path = document.getElementById('share-path').value;
        const networks = this.csvValues(document.getElementById('nfs-networks').value);
        const hosts = this.csvValues(document.getElementById('nfs-hosts').value);
        const ro = document.getElementById('nfs-readonly').checked;

        try {
            const response = await fetch(`${this.baseUrl}/shares/nfs`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ path, networks, hosts, ro, enabled: true })
            });

            const data = await response.json();

            if (response.ok) {
                this.closeModal();
                this.loadShares();
            } else {
                this.showError('share-error', data.error || t('truenas.error_create'));
            }
        } catch (err) {
            this.showError('share-error', t('truenas.error_connection') + ' ' + err.message);
        }
    }
    
    async scrubPool(poolId) {
        if (!(await showConfirm(t('truenas.confirm_scrub_title'), t('truenas.confirm_scrub_msg')))) return;
        
        try {
            const response = await fetch(`${this.baseUrl}/pools/${poolId}/scrub`, { method: 'POST' });
            if (response.ok) {
                showToast(t('truenas.scrub_started'), 'success');
                this.loadPools();
            } else {
                const data = await response.json();
                showToast(data.error || t('truenas.error_connection'), 'error');
            }
        } catch (err) {
            showToast(err.message, 'error');
        }
    }
    
    async deleteDataset(name) {
        // Permanent and recursive (?recursive=true): the dataset name has to be typed.
        const { confirmed } = await showConfirmDialog({
            title: t('truenas.confirm_dataset_delete_title'),
            message: t('truenas.confirm_dataset_delete_msg', {name}),
            danger: true,
            requireText: name,
            confirmText: t('truenas.btn_delete')
        });
        if (!confirmed) return;
        
        try {
            const response = await fetch(`${this.baseUrl}/datasets/${encodeURIComponent(name)}?recursive=true`, {
                method: 'DELETE'
            });
            
            if (response.ok) {
                this.loadDatasets();
            } else {
                const data = await response.json();
                showToast(data.error || t('truenas.error_connection'), 'error');
            }
        } catch (err) {
            showToast(err.message, 'error');
        }
    }
    
    async deleteSnapshot(name) {
        if (!(await showConfirm(t('truenas.confirm_snapshot_delete_title'), t('truenas.confirm_snapshot_delete_msg', {name}), {danger: true, confirmText: t('truenas.btn_delete')}))) return;
        
        try {
            const response = await fetch(`${this.baseUrl}/snapshots/${encodeURIComponent(name)}`, {
                method: 'DELETE'
            });
            
            if (response.ok) {
                this.loadSnapshots();
            } else {
                const data = await response.json();
                showToast(data.error || t('truenas.error_connection'), 'error');
            }
        } catch (err) {
            showToast(err.message, 'error');
        }
    }
    
    async rollbackSnapshot(name) {
        if (!(await showConfirm(t('truenas.confirm_rollback_title'), t('truenas.confirm_rollback_msg', {name}), {danger: true, confirmText: t('truenas.btn_rollback')}))) return;
        
        try {
            const response = await fetch(`${this.baseUrl}/snapshots/${encodeURIComponent(name)}/rollback`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ force: false })
            });
            
            if (response.ok) {
                showToast(`${t('truenas.rollback_success')}: ${t('truenas.rollback_complete')}`, 'success');
                this.loadSnapshots();
            } else {
                const data = await response.json();
                showToast(data.error || t('truenas.error_connection'), 'error');
            }
        } catch (err) {
            showToast(err.message, 'error');
        }
    }
    
    async deleteShare(shareId) {
        if (!(await showConfirm(t('truenas.confirm_share_delete_title'), t('truenas.confirm_share_delete_msg'), {danger: true, confirmText: t('truenas.btn_delete')}))) return;
        
        try {
            const response = await fetch(`${this.baseUrl}/shares/smb/${shareId}`, {
                method: 'DELETE'
            });
            
            if (response.ok) {
                this.loadShares();
            } else {
                const data = await response.json();
                showToast(data.error || t('truenas.error_connection'), 'error');
            }
        } catch (err) {
            showToast(err.message, 'error');
        }
    }

    async deleteNFSShare(shareId) {
        if (!(await showConfirm(t('truenas.confirm_share_delete_title'), t('truenas.confirm_share_delete_msg'), {danger: true, confirmText: t('truenas.btn_delete')}))) return;

        try {
            const response = await fetch(`${this.baseUrl}/shares/nfs/${shareId}`, {
                method: 'DELETE'
            });

            if (response.ok) {
                this.loadShares();
            } else {
                const data = await response.json();
                showToast(data.error || t('truenas.error_connection'), 'error');
            }
        } catch (err) {
            showToast(err.message, 'error');
        }
    }

    toggleShareFields() {
        const type = document.getElementById('share-type')?.value || 'smb';
        const nameField = document.getElementById('smb-share-name-field');
        const smbFields = document.getElementById('smb-share-fields');
        const nfsFields = document.getElementById('nfs-share-fields');
        if (nameField) nameField.style.display = type === 'smb' ? '' : 'none';
        if (smbFields) smbFields.style.display = type === 'smb' ? '' : 'none';
        if (nfsFields) nfsFields.classList.toggle('is-hidden', type !== 'nfs');
    }
    
    // UI Helpers
    showCreateDataset() { openModal('modal-dataset'); }
    showCreateSnapshot() { openModal('modal-snapshot'); }
    showCreateShare() {
        openModal('modal-share');
        this.toggleShareFields();
    }
    
    closeModal() {
        document.querySelectorAll('#modal-dataset.active, #modal-snapshot.active, #modal-share.active').forEach(m => closeModal(m.id));
        document.querySelectorAll('[id$="-error"]').forEach(el => el.innerHTML = '');
    }
    
    showError(elementId, message) {
        const el = document.getElementById(elementId);
        if (el) el.innerHTML = `<div class="alert error" role="alert">${escapeHtmlTruenas(message)}</div>`;
    }
    
    formatBytes(bytes) {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    }
    
    formatDuration(ms) {
        if (!Number.isFinite(ms) || ms < 0) return '-';
        const seconds = Math.floor(ms / 1000);
        const minutes = Math.floor(seconds / 60);
        const hours = Math.floor(minutes / 60);
        const days = Math.floor(hours / 24);
        
        if (days > 0) return `${days}d`;
        if (hours > 0) return `${hours}h`;
        if (minutes > 0) return `${minutes}m`;
        return `${seconds}s`;
    }

    snapshotAge(snap) {
        const hours = Number(snap.age_hours);
        if (Number.isFinite(hours) && hours >= 0) {
            return this.formatDuration(hours * 3600000);
        }
        const timestamp = Number(snap.rawcreation?.$date);
        if (Number.isFinite(timestamp) && timestamp > 0) {
            return this.formatDuration(Date.now() - timestamp);
        }
        return '-';
    }

    csvValues(value) {
        return String(value || '')
            .split(',')
            .map(item => item.trim())
            .filter(Boolean);
    }

    async updateShareCounts() {
        try {
            const [smbResponse, nfsResponse] = await Promise.all([
                fetch(`${this.baseUrl}/shares/smb`),
                fetch(`${this.baseUrl}/shares/nfs`)
            ]);
            const smbData = await smbResponse.json();
            const nfsData = await nfsResponse.json();
            this.setShareCounts(smbData.shares || [], nfsData.shares || []);
        } catch (err) {
            this.setShareCounts([], []);
        }
    }

    setShareCounts(smbShares, nfsShares) {
        const totalCount = document.getElementById('share-count');
        const smbCount = document.getElementById('smb-count');
        const nfsCount = document.getElementById('nfs-count');
        if (totalCount) totalCount.textContent = smbShares.length + nfsShares.length;
        if (smbCount) smbCount.textContent = smbShares.length;
        if (nfsCount) nfsCount.textContent = nfsShares.length;
    }
    
    startHealthCheck() {
        if (this._healthCheckInterval) {
            clearInterval(this._healthCheckInterval);
        }
        this._healthCheckInterval = setInterval(() => this.checkStatus(), 30000);
    }

    destroy() {
        if (this._healthCheckInterval) {
            clearInterval(this._healthCheckInterval);
            this._healthCheckInterval = null;
        }
    }
}

// Global instance
const truenasUI = new TrueNASUI();

// Clean up interval on page unload
window.addEventListener('pagehide', () => truenasUI.destroy());
