// Honesty transformations for the pinned upstream UI: nothing invented may be shown as measured.
// Pure text transforms. Every anchor must exist (and be unique) or the vendoring run fails.
// Never edit vendor outputs; change this module and regenerate.

export const upstreamClaimNote = 'Upstream RuView project claims — not measured or verified by AuraGo.';
export const exampleConfigNote = 'Example configuration — not read from your hardware.';
const antennaNote = 'Example configuration — antenna selection is cosmetic and not read from your hardware.';

const noteHtml = (text, indent) => indent + '<p class="aurago-upstream-claim">' + text + '</p>\n';

// `patch` is the checked replacement of vendor-ui.mjs. A function replacer keeps `$` literal in `after`.
function swap(patch, text, before, after) {
    const next = patch(text, before, () => after);
    if (text.indexOf(before) !== text.lastIndexOf(before)) throw Error('Pinned RuView honesty patch is ambiguous: ' + before.trim().slice(0, 60));
    return next;
}

// Replaces text[from, to) where `to` is the first occurrence of `until` after `from`.
function block(patch, text, from, until, replacement) {
    const start = text.indexOf(from), end = start < 0 ? -1 : text.indexOf(until, start + from.length);
    if (start < 0 || end < 0) throw Error('Pinned RuView honesty block no longer matches: ' + from.trim().slice(0, 60));
    return swap(patch, text, text.slice(start, end), replacement);
}

function hardwareTab(patch, text) {
    // The upstream view invented amplitude/phase with Math.random once a second.
    text = block(patch, text, '  // Start CSI simulation\n  startCSISimulation() {', '  // Check if any antennas are active', `  // Start CSI simulation
  // AuraGo: this view has no CSI source, so no timer runs and nothing is measured.
  startCSISimulation() {
    this.updateCSIDisplay();
  }

`);
    // The boundary names the next method: a bare comment would also match the one inside updateCSIDisplay.
    text = block(patch, text, '  // Update CSI display\n  updateCSIDisplay() {', '  // Update antenna array visualization\n  updateAntennaArray(', `  // Update CSI display
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

`);
    return swap(patch, text, "createInfoDiv('Signal Quality:'", "createInfoDiv('Signal Quality (example):'");
}

function poseDetectionCanvas(patch, text) {
    // The animated demo fabricates walking/waving figures; it is offered only for an explicitly simulated source.
    text = swap(patch, text, '<button class="btn btn-demo" id="demo-btn-${this.containerId}">&#9881; Demo</button>',
        '<button class="btn btn-demo" id="demo-btn-${this.containerId}" style="display:none">&#9881; Demo</button>');
    text = swap(patch, text, '    this.setupPoseServiceSubscription();\n',
        `    this.setupPoseServiceSubscription();

    // AuraGo learns the data source asynchronously; keep the demo entry points in step with it.
    this.updateDemoGate();
    this.demoGateTimer = setInterval(() => this.updateDemoGate(), 2000);
`);
    text = swap(patch, text, '  updateControls() {\n', `  // AuraGo: the demo is available only while the source is explicitly simulated.
  updateDemoGate() {
    const allowed = window.auragoRuViewSimulationAllowed?.() === true;
    for (const button of [document.getElementById('demo-btn-' + this.containerId), document.getElementById('run-offline-demo')]) {
      if (button) button.style.display = allowed ? '' : 'none';
    }
    if (!allowed && this.demoState && this.demoState.isRunning) {
      this.stopDemo();
      this.updateDemoButton(false);
    }
    return allowed;
  }

  updateControls() {
    this.updateDemoGate();
`);
    text = swap(patch, text, "      this.logger.info('Stopping pose detection');\n      this.state.isActive = false;\n",
        `      this.logger.info('Stopping pose detection');
      this.state.isActive = false;
      // AuraGo: a running demo animation stops with the stream.
      if (this.demoState && this.demoState.isRunning) {
        this.stopDemo();
        this.updateDemoButton(false);
      }
`);
    text = swap(patch, text, "  runDemo() {\n    this.logger.info('Running animated demo mode');\n",
        `  runDemo() {
    // AuraGo: never fabricate pose frames unless the source is explicitly simulated.
    if (window.auragoRuViewSimulationAllowed?.() !== true) return;
    this.logger.info('Running animated demo mode');
`);
    text = swap(patch, text, '      this.runDemo();\n      this.updateDemoButton(true);\n',
        '      this.runDemo();\n      this.updateDemoButton(!!(this.demoState && this.demoState.isRunning));\n');
    text = swap(patch, text, '  startDemoAnimation() {\n    if (!this.demoState || !this.demoState.isRunning) return;\n',
        `  startDemoAnimation() {
    if (!this.demoState || !this.demoState.isRunning) return;
    if (window.auragoRuViewSimulationAllowed?.() !== true) {
      this.stopDemo();
      this.updateDemoButton(false);
      return;
    }
`);
    text = swap(patch, text, "      demoBtn.style.borderColor = isRunning ? '#dc3545' : '#6f42c1';\n    }\n  }\n",
        `      demoBtn.style.borderColor = isRunning ? '#dc3545' : '#6f42c1';
    }
    // AuraGo: the hidden canvas button is not the one users see; keep the Live Demo tab button in step.
    const offlineBtn = document.getElementById('run-offline-demo');
    if (offlineBtn) offlineBtn.textContent = isRunning ? 'Stop Demo' : 'Demo';
  }
`);
    return swap(patch, text, '      // Stop demo animation\n      this.stopDemo();\n',
        '      // Stop demo animation\n      clearInterval(this.demoGateTimer);\n      this.stopDemo();\n');
}

function indexHtml(patch, text) {
    const note = (anchor, noteText, indent) => { text = swap(patch, text, anchor, anchor + '\n' + noteHtml(noteText, indent).trimEnd()); };
    // Dashboard hero, feature cards and headline numbers.
    note('<h2 data-i18n="dashboard.title">Revolutionary WiFi-Based Human Pose Detection</h2>', upstreamClaimNote, '                ');
    text = swap(patch, text, '<div class="key-benefits">', noteHtml(upstreamClaimNote, '').trimEnd() + '\n                <div class="key-benefits">');
    text = swap(patch, text, '<div class="system-stats">', noteHtml(upstreamClaimNote, '').trimEnd() + '\n                <div class="system-stats">');
    note('<h2>Performance Analysis</h2>', upstreamClaimNote, '            ');
    note('<h2>Real-World Applications</h2>', upstreamClaimNote, '            ');
    // Hardware tab: example configuration only, no measured CSI.
    note('<p class="help-text">Click antennas to toggle their state</p>', antennaNote, '                    ');
    note('<h3>WiFi Configuration</h3>', exampleConfigNote, '                    ');
    text = swap(patch, text, '<h4>Real-time CSI Data</h4>', '<h4>CSI data (not measured in this view)</h4>');
    text = swap(patch, text, '<div class="csi-fill amplitude" style="width: 75%"></div>', '<div class="csi-fill amplitude" style="width: 0%"></div>');
    text = swap(patch, text, '<span class="csi-value">0.75</span>', '<span class="csi-value">—</span>');
    text = swap(patch, text, '<div class="csi-fill phase" style="width: 60%"></div>', '<div class="csi-fill phase" style="width: 0%"></div>');
    text = swap(patch, text, '<span class="csi-value">1.2π</span>', '<span class="csi-value">—</span>');
    // Live Demo tab: no script ever writes these two (perf-monitor.js fills [data-metric="latency"], the HUD fills #hud-latency).
    text = swap(patch, text, '<span id="signalStrength">-45 dBm</span>', '<span id="signalStrength">—</span>');
    return swap(patch, text, '<span id="latency">12 ms</span>', '<span id="latency">—</span>');
}

export function patchHonesty(name, text, patch) {
    if (name === 'components/HardwareTab.js') text = hardwareTab(patch, text);
    if (name === 'components/PoseDetectionCanvas.js') text = poseDetectionCanvas(patch, text);
    if (name === 'components/LiveDemoTab.js') {
        // The embedded canvas hides its own controls; this is the Demo button users actually see.
        text = swap(patch, text, '<button class="btn btn--accent" id="run-offline-demo">Demo</button>', '<button class="btn btn--accent" id="run-offline-demo" style="display:none">Demo</button>');
        // app.js calls stopDemo() when the tab is left; an offline demo may be running without a started detection,
        // so it must be stopped before the isActive guard returns.
        text = swap(patch, text, "  stopDemo() {\n    if (!this.state.isActive) {\n", `  stopDemo() {
    const canvas = this.components.poseCanvas;
    if (canvas && canvas.demoState && canvas.demoState.isRunning) {
      canvas.stopDemo();
      canvas.updateDemoButton(false);
    }
    if (!this.state.isActive) {
`);
    }
    if (name === 'index.html') text = indexHtml(patch, text);
    if (name === 'style.css') {
        text = swap(patch, text, '/* Help text */\n.help-text {', `/* AuraGo: labels unverified upstream claims and example data; inherits the active palette */
.aurago-upstream-claim {
  margin: 4px auto 12px;
  max-width: 800px;
  font-size: 0.8125rem;
  font-style: italic;
  opacity: 0.75;
}

/* Help text */
.help-text {`);
    }
    return text;
}
