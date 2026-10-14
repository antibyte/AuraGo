import {controlResult} from '../control-contracts.js';
// Training Service for WiFi-DensePose UI
// Manages training lifecycle, progress streaming, and CSI recordings.

import { buildWsUrl } from '../config/api.config.js';
import { apiService } from './api.service.js';

export class TrainingService {
  constructor() {
    this.progressSocket = null;
    this.recordingState = 'unknown';
    this.recordingBusy = false;
    this.listeners = {};
    this.logger = this.createLogger();
  }

  createLogger() {
    return {
      debug: (...args) => console.debug('[TRAIN-DEBUG]', new Date().toISOString(), ...args),
      info: (...args) => console.info('[TRAIN-INFO]', new Date().toISOString(), ...args),
      warn: (...args) => console.warn('[TRAIN-WARN]', new Date().toISOString(), ...args),
      error: (...args) => console.error('[TRAIN-ERROR]', new Date().toISOString(), ...args)
    };
  }

  // --- Event emitter helpers ---

  on(event, callback) {
    if (!this.listeners[event]) {
      this.listeners[event] = [];
    }
    this.listeners[event].push(callback);
    return () => this.off(event, callback);
  }

  off(event, callback) {
    if (!this.listeners[event]) return;
    this.listeners[event] = this.listeners[event].filter(cb => cb !== callback);
  }

  emit(event, data) {
    if (!this.listeners[event]) return;
    this.listeners[event].forEach(cb => {
      try { cb(data); } catch (err) { this.logger.error('Listener error', { event, err }); }
    });
  }

  // --- Training API methods ---

  async startTraining(config) {
    try {
      this.logger.info('Starting training', { config });
      const data = controlResult(await apiService.post('/api/v1/train/start', config), 'started');
      this.emit('training-started', data);
      return data;
    } catch (error) {
      this.logger.error('Failed to start training', { error: error.message });
      throw error;
    }
  }

  async stopTraining() {
    try {
      this.logger.info('Stopping training');
      const data = controlResult(await apiService.post('/api/v1/train/stop', {}), 'stopping');
      this.emit('training-stopping', data);
      return data;
    } catch (error) {
      this.logger.error('Failed to stop training', { error: error.message });
      throw error;
    }
  }

  async getTrainingStatus() {
    try {
      const data = await apiService.get('/api/v1/train/status');
      if (typeof data?.active !== 'boolean') throw Error('Invalid training status');
      return data;
    } catch (error) {
      this.logger.error('Failed to get training status', { error: error.message });
      throw error;
    }
  }

  async startPretraining(config) {
    try {
      this.logger.info('Starting pretraining', { config });
      const data = controlResult(await apiService.post('/api/v1/train/pretrain', config), 'started');
      this.emit('training-started', data);
      return data;
    } catch (error) {
      this.logger.error('Failed to start pretraining', { error: error.message });
      throw error;
    }
  }

  async startLoraTraining(config) {
    try {
      this.logger.info('Starting LoRA training', { config });
      const data = controlResult(await apiService.post('/api/v1/train/lora', config), 'started');
      this.emit('training-started', data);
      return data;
    } catch (error) {
      this.logger.error('Failed to start LoRA training', { error: error.message });
      throw error;
    }
  }

  // --- Recording API methods ---

  async listRecordings() {
    try {
      const data = await apiService.get('/api/v1/recording/list');
      if (!Array.isArray(data?.recordings)) throw Error('Invalid recordings response');
      return data.recordings;
    } catch (error) {
      this.logger.error('Failed to list recordings', { error: error.message });
      throw error;
    }
  }

  async startRecording(config = {}) {
    return this._recordingRequest('start', config);
  }

  async _recordingRequest(action, config = {}) {
    if (this.recordingBusy) throw Error('A recording request is already pending');
    this.recordingBusy = true;
    this.emit('recording-state');
    try {
      const data = controlResult(await apiService.post('/api/v1/recording/' + action, config), 'success');
      this.recordingState = action === 'start' ? 'recording' : 'idle';
      this.emit(action === 'start' ? 'recording-started' : 'recording-stopped', data);
      return data;
    } catch (error) {
      this.recordingState = 'unknown';
      throw error;
    } finally {
      this.recordingBusy = false;
      this.emit('recording-state');
    }
  }

  async stopRecording() {
    return this._recordingRequest('stop');
  }

  async deleteRecording(id) {
    try {
      this.logger.info('Deleting recording', { id });
      const data = await apiService.delete(
        `/api/v1/recording/${encodeURIComponent(id)}`
      );
      return controlResult(data, 'success');
    } catch (error) {
      this.logger.error('Failed to delete recording', { id, error: error.message });
      throw error;
    }
  }

  // --- WebSocket progress stream ---

  connectProgressStream() {
    if (this.progressSocket) {
      this.logger.warn('Progress stream already connected');
      return this.progressSocket;
    }

    const url = buildWsUrl('/ws/train/progress');
    this.logger.info('Connecting progress stream', { url });

    const ws = new WebSocket(url);

    ws.onopen = () => {
      this.logger.info('Progress stream connected');
      this.emit('progress-connected', {});
    };

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        this.emit('progress', data);
      } catch (err) {
        this.logger.warn('Failed to parse progress message', { error: err.message });
      }
    };

    ws.onerror = (error) => {
      this.logger.error('Progress stream error', { error });
      this.emit('progress-error', { error });
    };

    ws.onclose = () => {
      this.logger.info('Progress stream disconnected');
      this.progressSocket = null;
      this.emit('progress-disconnected', {});
    };

    this.progressSocket = ws;
    return ws;
  }

  disconnectProgressStream() {
    if (this.progressSocket) {
      this.progressSocket.close();
      this.progressSocket = null;
    }
  }

  dispose() {
    this.disconnectProgressStream();
    this.listeners = {};
    this.logger.info('TrainingService disposed');
  }
}

// Create singleton instance
export const trainingService = new TrainingService();
