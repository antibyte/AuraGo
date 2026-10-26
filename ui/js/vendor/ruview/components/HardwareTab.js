// Hardware Tab Component

export class HardwareTab {
  constructor(containerElement) {
    this.container = containerElement;
    this.antennas = [];
    this.csiUpdateInterval = null;
    this.isActive = false;
  }

  // Initialize component
  init() {
    this.setupAntennas();
    this.startCSISimulation();
  }

  // Set up antenna interactions
  setupAntennas() {
    this.antennas = Array.from(this.container.querySelectorAll('.antenna'));
    
    this.antennas.forEach(antenna => {
      antenna.addEventListener('click', () => {
        antenna.classList.toggle('active');
        this.updateCSIDisplay();
      });
    });
  }

  // Start CSI simulation
  // AuraGo: this view has no CSI source, so no timer runs and nothing is measured.
  startCSISimulation() {
    this.updateCSIDisplay();
  }

  // Check if any antennas are active
  hasActiveAntennas() {
    return this.antennas.some(antenna => antenna.classList.contains('active'));
  }

  // Update CSI display
  // AuraGo: amplitude and phase are never measured here, so the rows stay empty.
  updateCSIDisplay() {
    const activeAntennas = this.antennas.filter(a => a.classList.contains('active'));
    const amplitudeFill = this.container.querySelector('.csi-fill.amplitude');
    const phaseFill = this.container.querySelector('.csi-fill.phase');
    const amplitudeValue = this.container.querySelector('.csi-row:first-child .csi-value');
    const phaseValue = this.container.querySelector('.csi-row:last-child .csi-value');
    if (amplitudeFill) amplitudeFill.style.width = '0%';
    if (phaseFill) phaseFill.style.width = '0%';
    if (amplitudeValue) amplitudeValue.textContent = '—';
    if (phaseValue) phaseValue.textContent = '—';
    this.updateAntennaArray(activeAntennas);
  }

  // Update antenna array visualization
  updateAntennaArray(activeAntennas) {
    const arrayStatus = this.container.querySelector('.array-status');
    if (!arrayStatus) return;
    
    const txActive = activeAntennas.filter(a => a.classList.contains('tx')).length;
    const rxActive = activeAntennas.filter(a => a.classList.contains('rx')).length;
    
    // Clear and rebuild using safe DOM methods to prevent XSS
    arrayStatus.innerHTML = '';
    
    const createInfoDiv = (label, value) => {
      const div = document.createElement('div');
      div.className = 'array-info';
      
      const labelSpan = document.createElement('span');
      labelSpan.className = 'info-label';
      labelSpan.textContent = label;
      
      const valueSpan = document.createElement('span');
      valueSpan.className = 'info-value';
      valueSpan.textContent = value;
      
      div.appendChild(labelSpan);
      div.appendChild(valueSpan);
      return div;
    };
    
    arrayStatus.appendChild(createInfoDiv('Active TX:', `${txActive}/3`));
    arrayStatus.appendChild(createInfoDiv('Active RX:', `${rxActive}/6`));
    arrayStatus.appendChild(createInfoDiv('Signal Quality (example):', `${this.calculateSignalQuality(txActive, rxActive)}%`));
  }

  // Calculate signal quality based on active antennas
  calculateSignalQuality(txCount, rxCount) {
    if (txCount === 0 || rxCount === 0) return 0;
    
    const txRatio = txCount / 3;
    const rxRatio = rxCount / 6;
    const quality = (txRatio * 0.4 + rxRatio * 0.6) * 100;
    
    return Math.round(quality);
  }

  // Toggle all antennas
  toggleAllAntennas(active) {
    this.antennas.forEach(antenna => {
      antenna.classList.toggle('active', active);
    });
    this.updateCSIDisplay();
  }

  // Reset antenna configuration
  resetAntennas() {
    // Set default configuration (all active)
    this.antennas.forEach(antenna => {
      antenna.classList.add('active');
    });
    this.updateCSIDisplay();
  }

  // Clean up
  dispose() {
    if (this.csiUpdateInterval) {
      clearInterval(this.csiUpdateInterval);
      this.csiUpdateInterval = null;
    }
    
    this.antennas.forEach(antenna => {
      antenna.removeEventListener('click', this.toggleAntenna);
    });
  }
}