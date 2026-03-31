// TrueNAS UI Controller
class TrueNASUI {
    constructor() {
        this.baseUrl = '/api/truenas';
        this.currentTab = 'overview';
        this.pools = [];
        this.datasets = [];
        this.init();
    }
    
    init() {
        this.bindEvents();
        this.checkStatus();
        this.startHealthCheck();
    }
    
    bindEvents() {
        // Tab Navigation
        document.querySelectorAll('.nav-btn').forEach(btn => {
            btn.addEventListener('click', (e) => this.switchTab(e.target.dataset.tab));
        });
        
        // Forms
        document.getElementById('truenas-settings-form')?.addEventListener('submit', (e) => this.saveSettings(e));
        document.getElementById('dataset-form')?.addEventListener('submit', (e) => this.createDataset(e));
        document.getElementById('snapshot-form')?.addEventListener('submit', (e) => this.createSnapshot(e));
        document.getElementById('share-form')?.addEventListener('submit', (e) => this.createShare(e));
        
        // Modal close on outside click
        document.querySelectorAll('.truenas-modal').forEach(modal => {
            modal.addEventListener('click', (e) => {
                if (e.target === modal) this.closeModal();
            });
        });
    }
    
    async switchTab(tabName) {
        this.currentTab = tabName;
        
        // Update nav
        document.querySelectorAll('.nav-btn').forEach(b => b.classList.remove('active'));
        document.querySelector(`[data-tab="${tabName}"]`)?.classList.add('active');
        
        // Update content
        document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
        document.getElementById(`tab-${tabName}`)?.classList.add('active');
        
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
    
    async checkStatus() {
        try {
            const response = await fetch(`${this.baseUrl}/status`);
            const data = await response.json();
            
            const indicator = document.getElementById('status-indicator');
            if (data.enabled === false) {
                indicator.innerHTML = '<span class="status-offline">● Deaktiviert</span>';
            } else if (data.status === 'online') {
                indicator.innerHTML = `<span class="status-online">● Online</span> (${data.version})`;
                this.loadOverview();
            } else if (data.status === 'error') {
                indicator.innerHTML = `<span class="status-error">● Fehler: ${data.error}</span>`;
            } else {
                indicator.innerHTML = '<span class="status-offline">● Offline</span>';
            }
        } catch (err) {
            document.getElementById('status-indicator').innerHTML = 
                '<span class="status-error">● Verbindungsfehler</span>';
        }
    }
    
    async loadOverview() {
        try {
            const response = await fetch(`${this.baseUrl}/health`);
            const data = await response.json();
            
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
                poolStatusEl.innerHTML = `<span class="status-error">${degraded} DEGRADED</span>`;
            } else {
                poolStatusEl.innerHTML = '<span class="status-online">Alle OK</span>';
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
            progressEl.style.width = `${percent}%`;
            progressEl.className = `progress-fill ${percent > 90 ? 'danger' : percent > 70 ? 'warning' : ''}`;
            
            // Alerts
            const activeAlerts = alerts.filter(a => !a.dismissed);
            if (activeAlerts.length > 0) {
                const alertsSection = document.getElementById('alerts-section');
                alertsSection.style.display = 'block';
                const container = document.getElementById('alerts-container');
                container.innerHTML = activeAlerts.map(a => `
                    <div class="alert-item ${a.level.toLowerCase()}">
                        <h4>${a.title}</h4>
                        <p>${a.message}</p>
                        <div class="alert-date">${new Date(a.date).toLocaleString()}</div>
                    </div>
                `).join('');
            }
            
        } catch (err) {
            this.showError('overview-error', 'Fehler beim Laden der Übersicht: ' + err.message);
        }
    }
    
    async loadPools() {
        const container = document.getElementById('pools-container');
        container.innerHTML = '<div class="loading">Lade Pools...</div>';
        
        try {
            const response = await fetch(`${this.baseUrl}/pools`);
            const data = await response.json();
            
            if (!data.pools || data.pools.length === 0) {
                container.innerHTML = '<div class="empty-state">Keine Pools gefunden</div>';
                return;
            }
            
            this.pools = data.pools;
            container.innerHTML = data.pools.map(pool => {
                const usage = pool.size?.total > 0 ? (pool.size.allocated / pool.size.total * 100).toFixed(1) : 0;
                return `
                    <div class="pool-card ${pool.status.toLowerCase()}">
                        <div class="pool-header">
                            <h3>${pool.name}</h3>
                            <span class="status-badge ${pool.status.toLowerCase()}">${pool.status}</span>
                        </div>
                        <div class="pool-stats">
                            <div class="stat">
                                <label>Größe</label>
                                <value>${this.formatBytes(pool.size?.total)}</value>
                            </div>
                            <div class="stat">
                                <label>Belegt</label>
                                <value>${this.formatBytes(pool.size?.allocated)}</value>
                            </div>
                            <div class="stat">
                                <label>Frei</label>
                                <value>${this.formatBytes(pool.size?.free)}</value>
                            </div>
                        </div>
                        <div class="progress-bar">
                            <div class="progress-fill ${usage > 90 ? 'danger' : usage > 70 ? 'warning' : ''}" style="width: ${usage}%"></div>
                        </div>
                        <div class="pool-actions">
                            <button class="btn btn-secondary" onclick="truenasUI.scrubPool(${pool.id})" ${pool.scan?.state === 'SCANNING' ? 'disabled' : ''}>
                                ${pool.scan?.state === 'SCANNING' ? 'Scrub läuft...' : 'Scrub starten'}
                            </button>
                        </div>
                    </div>
                `;
            }).join('');
            
        } catch (err) {
            container.innerHTML = `<div class="alert error">Fehler: ${err.message}</div>`;
        }
    }
    
    async loadDatasets() {
        const container = document.getElementById('datasets-container');
        container.innerHTML = '<div class="loading">Lade Datasets...</div>';
        
        try {
            const response = await fetch(`${this.baseUrl}/datasets`);
            const data = await response.json();
            
            if (!data.datasets || data.datasets.length === 0) {
                container.innerHTML = '<div class="empty-state">Keine Datasets gefunden</div>';
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
                            <h4>${ds.name}</h4>
                            <p>${this.formatBytes(used)} / ${this.formatBytes(total)} belegt (${usage}%) • Kompression: ${ds.compression?.parsed || 'off'}</p>
                        </div>
                        <div class="dataset-actions">
                            <button class="btn btn-danger" onclick="truenasUI.deleteDataset('${ds.name}')">Löschen</button>
                        </div>
                    </div>
                `;
            }).join('');
            
            // Update snapshot filter dropdown
            const filterSelect = document.getElementById('snapshot-filter');
            if (filterSelect) {
                filterSelect.innerHTML = '<option value="">Alle Datasets</option>' + 
                    data.datasets.map(ds => `<option value="${ds.name}">${ds.name}</option>`).join('');
            }
            // Update snapshot create dropdown
            const createSelect = document.getElementById('snapshot-dataset');
            if (createSelect) {
                createSelect.innerHTML = '<option value="">Wählen...</option>' + 
                    data.datasets.map(ds => `<option value="${ds.name}">${ds.name}</option>`).join('');
            }
            
        } catch (err) {
            container.innerHTML = `<div class="alert error">Fehler: ${err.message}</div>`;
        }
    }
    
    async loadSnapshots() {
        const container = document.getElementById('snapshots-container');
        container.innerHTML = '<div class="loading">Lade Snapshots...</div>';
        
        try {
            const filter = document.getElementById('snapshot-filter')?.value || '';
            const url = filter ? `${this.baseUrl}/snapshots?dataset=${encodeURIComponent(filter)}` : `${this.baseUrl}/snapshots`;
            const response = await fetch(url);
            const data = await response.json();
            
            if (!data.snapshots || data.snapshots.length === 0) {
                container.innerHTML = '<div class="empty-state">Keine Snapshots gefunden</div>';
                return;
            }
            
            container.innerHTML = data.snapshots.map(snap => {
                const age = this.formatDuration(snap.age_hours * 3600000);
                return `
                    <div class="snapshot-item">
                        <div class="snapshot-info">
                            <h4>${snap.name}</h4>
                            <p>${snap.dataset} • ${this.formatBytes(snap.properties?.used?.parsed || 0)} • Vor ${age}</p>
                        </div>
                        <div class="snapshot-actions">
                            <button class="btn btn-secondary" onclick="truenasUI.rollbackSnapshot('${snap.name}')">Rollback</button>
                            <button class="btn btn-danger" onclick="truenasUI.deleteSnapshot('${snap.name}')">Löschen</button>
                        </div>
                    </div>
                `;
            }).join('');
            
        } catch (err) {
            container.innerHTML = `<div class="alert error">Fehler: ${err.message}</div>`;
        }
    }
    
    async loadShares() {
        const container = document.getElementById('shares-container');
        container.innerHTML = '<div class="loading">Lade Freigaben...</div>';
        
        try {
            const response = await fetch(`${this.baseUrl}/shares/smb`);
            const data = await response.json();
            
            if (!data.shares || data.shares.length === 0) {
                container.innerHTML = '<div class="empty-state">Keine SMB-Freigaben gefunden</div>';
                return;
            }
            
            container.innerHTML = data.shares.map(share => `
                <div class="share-item">
                    <div class="share-info">
                        <h4>${share.name}</h4>
                        <p>${share.path} ${share.guestok ? '• Gast erlaubt' : ''} ${share.timemachine ? '• Time Machine' : ''}</p>
                    </div>
                    <div class="share-actions">
                        <button class="btn btn-danger" onclick="truenasUI.deleteShare(${share.id})">Löschen</button>
                    </div>
                </div>
            `).join('');
            
        } catch (err) {
            container.innerHTML = `<div class="alert error">Fehler: ${err.message}</div>`;
        }
    }
    
    async loadSettings() {
        try {
            // Load current config from main config API
            const response = await fetch('/api/config');
            const data = await response.json();
            
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
                await fetch('/api/vault', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ key: 'truenas_api_key', value: apiKey })
                });
            } catch (err) {
                this.showError('settings-error', 'Fehler beim Speichern des API Keys: ' + err.message);
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
                this.showSuccess('settings-error', 'Einstellungen gespeichert');
                this.checkStatus();
            } else {
                const data = await response.json();
                this.showError('settings-error', data.error || 'Fehler beim Speichern');
            }
        } catch (err) {
            this.showError('settings-error', 'Fehler: ' + err.message);
        }
    }
    
    async testConnection() {
        const btn = document.querySelector('button[onclick="testConnection()"]');
        btn.disabled = true;
        btn.textContent = 'Teste...';
        
        try {
            await this.checkStatus();
            this.showSuccess('settings-error', 'Verbindung erfolgreich');
        } catch (err) {
            this.showError('settings-error', 'Verbindung fehlgeschlagen: ' + err.message);
        } finally {
            btn.disabled = false;
            btn.textContent = 'Verbindung testen';
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
                this.showError('dataset-error', data.error || 'Fehler beim Erstellen');
            }
        } catch (err) {
            this.showError('dataset-error', 'Fehler: ' + err.message);
        }
    }
    
    async createSnapshot(e) {
        e.preventDefault();
        
        const dataset = document.getElementById('snapshot-dataset').value;
        const name = document.getElementById('snapshot-name').value;
        const recursive = document.getElementById('snapshot-recursive').checked;
        const retention = parseInt(document.getElementById('snapshot-retention').value) || 0;
        
        if (!dataset) {
            this.showError('snapshot-error', 'Bitte ein Dataset wählen');
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
                this.showError('snapshot-error', data.error || 'Fehler beim Erstellen');
            }
        } catch (err) {
            this.showError('snapshot-error', 'Fehler: ' + err.message);
        }
    }
    
    async createShare(e) {
        e.preventDefault();
        
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
                this.showError('share-error', data.error || 'Fehler beim Erstellen');
            }
        } catch (err) {
            this.showError('share-error', 'Fehler: ' + err.message);
        }
    }
    
    async scrubPool(poolId) {
        if (!(await showConfirm('Scrub starten', 'Dies kann die Performance beeinträchtigen.'))) return;
        
        try {
            const response = await fetch(`${this.baseUrl}/pools/${poolId}/scrub`, { method: 'POST' });
            if (response.ok) {
                await showAlert('Scrub gestartet', 'Der Pool-Scrub wurde erfolgreich gestartet.');
                this.loadPools();
            } else {
                const data = await response.json();
                await showAlert('Fehler', data.error || 'Unbekannter Fehler');
            }
        } catch (err) {
            await showAlert('Fehler', err.message);
        }
    }
    
    async deleteDataset(name) {
        if (!(await showConfirm('Dataset löschen', `Dataset "${name}" wirklich löschen? Alle Daten gehen verloren!`))) return;
        
        try {
            const response = await fetch(`${this.baseUrl}/datasets/${encodeURIComponent(name)}?recursive=true`, {
                method: 'DELETE'
            });
            
            if (response.ok) {
                this.loadDatasets();
            } else {
                const data = await response.json();
                await showAlert('Fehler', data.error || 'Unbekannter Fehler');
            }
        } catch (err) {
            await showAlert('Fehler', err.message);
        }
    }
    
    async deleteSnapshot(name) {
        if (!(await showConfirm('Snapshot löschen', `Snapshot "${name}" wirklich löschen?`))) return;
        
        try {
            const response = await fetch(`${this.baseUrl}/snapshots/${encodeURIComponent(name)}`, {
                method: 'DELETE'
            });
            
            if (response.ok) {
                this.loadSnapshots();
            } else {
                const data = await response.json();
                await showAlert('Fehler', data.error || 'Unbekannter Fehler');
            }
        } catch (err) {
            await showAlert('Fehler', err.message);
        }
    }
    
    async rollbackSnapshot(name) {
        if (!(await showConfirm('Rollback bestätigen', `WIRKLICH zu "${name}" zurücksetzen? ALLE Daten nach diesem Snapshot werden gelöscht!`))) return;
        
        try {
            const response = await fetch(`${this.baseUrl}/snapshots/${encodeURIComponent(name)}/rollback`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ force: false })
            });
            
            if (response.ok) {
                await showAlert('Rollback erfolgreich', 'Der Snapshot-Rollback wurde erfolgreich durchgeführt.');
                this.loadSnapshots();
            } else {
                const data = await response.json();
                await showAlert('Fehler', data.error || 'Unbekannter Fehler');
            }
        } catch (err) {
            await showAlert('Fehler', err.message);
        }
    }
    
    async deleteShare(shareId) {
        if (!(await showConfirm('Freigabe löschen', 'Freigabe wirklich löschen? Die Daten bleiben erhalten.'))) return;
        
        try {
            const response = await fetch(`${this.baseUrl}/shares/smb/${shareId}`, {
                method: 'DELETE'
            });
            
            if (response.ok) {
                this.loadShares();
            } else {
                const data = await response.json();
                await showAlert('Fehler', data.error || 'Unbekannter Fehler');
            }
        } catch (err) {
            await showAlert('Fehler', err.message);
        }
    }
    
    // UI Helpers
    showCreateDataset() { document.getElementById('modal-dataset').classList.add('active'); }
    showCreateSnapshot() { document.getElementById('modal-snapshot').classList.add('active'); }
    showCreateShare() { document.getElementById('modal-share').classList.add('active'); }
    
    closeModal() {
        document.querySelectorAll('.truenas-modal').forEach(m => m.classList.remove('active'));
        document.querySelectorAll('[id$="-error"]').forEach(el => el.innerHTML = '');
    }
    
    showError(elementId, message) {
        const el = document.getElementById(elementId);
        if (el) el.innerHTML = `<div class="alert error">${message}</div>`;
    }
    
    showSuccess(elementId, message) {
        const el = document.getElementById(elementId);
        if (el) el.innerHTML = `<div class="alert success">${message}</div>`;
        setTimeout(() => el.innerHTML = '', 3000);
    }
    
    formatBytes(bytes) {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    }
    
    formatDuration(ms) {
        const seconds = Math.floor(ms / 1000);
        const minutes = Math.floor(seconds / 60);
        const hours = Math.floor(minutes / 60);
        const days = Math.floor(hours / 24);
        
        if (days > 0) return `${days}d`;
        if (hours > 0) return `${hours}h`;
        if (minutes > 0) return `${minutes}m`;
        return `${seconds}s`;
    }
    
    startHealthCheck() {
        if (this._healthCheckInterval) {
            clearInterval(this._healthCheckInterval);
        }
        this._healthCheckInterval = setInterval(() => this.checkStatus(), 30000);
    }
}

// Global instance
const truenasUI = new TrueNASUI();

// Global functions for onclick handlers
function refreshPools() { truenasUI.loadPools(); }
function showCreateDataset() { truenasUI.showCreateDataset(); }
function showCreateSnapshot() { truenasUI.showCreateSnapshot(); }
function showCreateShare() { truenasUI.showCreateShare(); }
function closeModal() { truenasUI.closeModal(); }
function testConnection() { truenasUI.testConnection(); }
function filterSnapshots() { truenasUI.loadSnapshots(); }
