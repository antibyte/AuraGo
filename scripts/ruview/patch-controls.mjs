// Source transformations for the pinned upstream control UI. Never edit vendor outputs.
function method(text, name, replacement) {
    const marker = new RegExp('^  (?:async )?' + name + '\\(', 'm').exec(text);
    if (!marker) throw Error('Missing pinned RuView method: ' + name);
    const end = text.indexOf('\n  }', marker.index);
    if (end < 0) throw Error('Missing pinned RuView method end: ' + name);
    return text.slice(0, marker.index) + replacement.trimEnd() + text.slice(end + 4);
}

export function patchControls(name, text, patch) {
    if (name === 'services/model.service.js') {
        text = "import {controlResult} from '../control-contracts.js';\n" + text;
        text = patch(text, "      const data = await apiService.get('/api/v1/models');", "      const data = await apiService.get('/api/v1/models');\n      if (data?.error || data?.success === false || !Array.isArray(data?.models)) throw Error('Invalid model list');");
        text = patch(text, "      if (error.status === 404) {\n        this.activeModel = null;\n        return null;\n      }", '');
        text = patch(text, "      return data?.profiles ?? [];", "      if (data?.error || data?.success === false || !Array.isArray(data?.profiles)) throw Error('Invalid profile list');\n      return data.profiles;");
        text = patch(text, "      this.activeModel = data || null;", "      if (data?.error || data?.success === false) throw Error('Invalid active model response');\n      this.activeModel = data || null;");
        text = patch(text, "      return data;\n    } catch (error) {\n      this.logger.error('Failed to delete model'", "      return controlResult(data, 'success');\n    } catch (error) {\n      this.logger.error('Failed to delete model'");
    }
    if (name === 'services/training.service.js') {
        text = "import {controlResult} from '../control-contracts.js';\n" + text;
        text = patch(text, '    this.progressSocket = null;', "    this.progressSocket = null;\n    this.recordingState = 'unknown';\n    this.recordingBusy = false;");
        text = method(text, 'startRecording', `  async startRecording(config = {}) {
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
  }`);
        text = method(text, 'stopRecording', `  async stopRecording() {
    return this._recordingRequest('stop');
  }`);
        text = patch(text, "      return data?.recordings ?? [];", "      if (data?.error || data?.success === false || !Array.isArray(data?.recordings)) throw Error('Invalid recordings response');\n      return data.recordings;");
        text = patch(text, "      return data;\n    } catch (error) {\n      this.logger.error('Failed to delete recording'", "      return controlResult(data, 'success');\n    } catch (error) {\n      this.logger.error('Failed to delete recording'");
        for (const [endpoint, expected] of [['start','started'],['stop','stopping'],['pretrain','started'],['lora','started']]) {
            const body = endpoint === 'stop' ? '{}' : 'config';
            text = patch(text, `const data = await apiService.post('/api/v1/train/${endpoint}', ${body});`, `const data = controlResult(await apiService.post('/api/v1/train/${endpoint}', ${body}), '${expected}');`);
        }
        text = patch(text, "this.emit('training-stopped', data)", "this.emit('training-stopping', data)");
        text = patch(text, "      const data = await apiService.get('/api/v1/train/status');", "      const data = await apiService.get('/api/v1/train/status');\n      if (data?.error || data?.success === false || typeof data?.active !== 'boolean') throw Error('Invalid training status');");
    }
    if (name === 'components/LiveDemoTab.js') {
        text="import {readError} from '../control-contracts.js';\n"+text;
        text=patch(text,'<div class="model-status-text" id="model-status-text">No model loaded</div>','<div class="model-status-text" id="model-status-text" role="status">Model status unknown</div><button id="retry-models-btn" class="btn-ld btn-ld-muted">Refresh models</button>');
        text=method(text,'fetchModels',`  async fetchModels() {
    if (!modelService || this.modelsPending) return;
    this.modelsPending=true;this.setModelStatus('Loading models…');
    const retry=this.container.querySelector('#retry-models-btn');if(retry)retry.disabled=true;
    try {
      const data=await modelService.listModels();
      this.modelState.models=data.models;this.populateModelSelector();
      const active=await modelService.getActiveModel();
      this.modelState.activeModelId=active?.model_id||null;this.modelState.activeModelInfo=active;
      this.updateModelUI();
    } catch(error) { this.setModelStatus(readError(error)+' Model status unknown; previous data may be stale.'); }
    finally {this.modelsPending=false;if(retry)retry.disabled=false;}
  }`);
        text=patch(text,"    if (quickRecordBtn) quickRecordBtn.addEventListener('click', () => this.handleQuickRecord());", "    this.container.querySelector('#retry-models-btn')?.addEventListener('click',()=>this.fetchModels());\n    if (quickRecordBtn) quickRecordBtn.addEventListener('click', () => this.handleQuickRecord());");
        for (const [id, action] of [['load-model-btn','model_load'],['unload-model-btn','model_unload'],['lora-profile-selector','lora_activate']]) text=patch(text, 'id="'+id+'"', 'id="'+id+'" data-ruview-action="'+action+'"');
        text = patch(text, '<button class="btn-ld btn-ld-muted" id="quick-record-btn">Record 60s</button>', `<button class="btn-ld btn-ld-muted" id="quick-record-btn" data-ruview-action="recording_start">Start Recording</button>
                  <button class="btn-ld btn-ld-muted" id="stop-record-btn" data-ruview-action="recording_stop">Stop Recording</button>
                  <p>Recording continues until you stop it, even if you close this view.</p>
                  <p id="recording-feedback" role="status" aria-live="polite">Recording status unknown</p>`);
        text = method(text, 'handleQuickRecord', `  async handleQuickRecord(stop = false) {
    if (!trainingService) return;
    const feedback = this.container.querySelector('#recording-feedback');
    if (feedback) feedback.textContent = stop ? 'Stopping recording…' : 'Starting recording…';
    try {
      if (stop) await trainingService.stopRecording();
      else await trainingService.startRecording({id: 'quick'});
      if (feedback?.isConnected) feedback.textContent = stop ? 'Recording stopped' : 'Recording started — stop it manually';
    } catch (error) {
      if (feedback?.isConnected) feedback.textContent = error.message + ' — recording status unknown';
    }
  }

  updateRecordingControls() {
    const service = trainingService;
    const start = this.container.querySelector('#quick-record-btn');
    const stop = this.container.querySelector('#stop-record-btn');
    if (start) start.disabled = !service || service.recordingBusy || service.recordingState === 'recording';
    if (stop) stop.disabled = !service || service.recordingBusy;
    for (const button of [start, stop]) if (button) button.setAttribute('aria-busy', String(!!service?.recordingBusy));
    const badge = this.container.querySelector('#training-status-badge');
    if (badge && !['training','stopping'].includes(this.trainingState.status)) badge.textContent = service?.recordingState === 'recording' ? 'Recording' : service?.recordingState === 'idle' ? 'Recording stopped' : 'Recording status unknown';
  }`);
        text = method(text, 'updateTrainingStatus', `  updateTrainingStatus() {
    const badge = this.container.querySelector('#training-status-badge');
    if (!badge) return;
    badge.classList.remove('training', 'recording');
    if (this.trainingState.status === 'training') badge.textContent = 'Training epoch ' + this.trainingState.epoch + '/' + this.trainingState.totalEpochs;
    else if (this.trainingState.status === 'stopping') badge.textContent = 'Stopping training…';
    else this.updateRecordingControls();
  }`);
        text = patch(text, "          this.trainingState.status = 'training';", "          this.trainingState.status = data.active === false ? 'idle' : data.phase === 'stopping' ? 'stopping' : 'training';");
        text = patch(text, "      this.subscriptions.push(unsub3, unsub4);", "      this.subscriptions.push(unsub3, unsub4, trainingService.on('recording-state', () => this.updateRecordingControls()));\n      this.updateRecordingControls();");
        text = patch(text, "      const unsub4 = trainingService.on('training-stopped', () => {\n        this.trainingState.status = 'idle';", "      const unsub4 = trainingService.on('training-stopping', () => {\n        this.trainingState.status = 'stopping';");
        text = patch(text, "    if (quickRecordBtn) quickRecordBtn.addEventListener('click', () => this.handleQuickRecord());", "    if (quickRecordBtn) quickRecordBtn.addEventListener('click', () => this.handleQuickRecord());\n    this.container.querySelector('#stop-record-btn')?.addEventListener('click', () => this.handleQuickRecord(true));\n    this.updateRecordingControls();");
    }
    if (name === 'components/TrainingPanel.js') {
        text = "import {trainingPayload, readError, preserveFocus} from '../control-contracts.js';\nimport {modelService} from '../services/model.service.js';\nlet panelSerial = 0;\n" + text;
        text = patch(text, "    this.state = {", "    this.fieldPrefix = 'ruview-training-' + (++panelSerial);\n    this.refreshEpoch = 0;\n    this.destroyed = false;\n    this.state = {");
        text = patch(text, "recordings: [], trainingStatus: null, isRecording: false,", "recordings: [], trainingStatus: null, models: [], recordingsLoaded: false, statusLoaded: false, modelsLoaded: false,");
        text = patch(text, "selectedRecordings: [], base_model: '', lora_profile_name: ''", "selectedRecordings: [], base_model: '', lora_profile_name: 'default', method: 'startTraining'");
        text = patch(text, "trainingService.on('training-stopped', () => {\n        trainingService.disconnectProgressStream();", "trainingService.on('recording-state', () => this.render()),\n      trainingService.on('training-stopping', () => {");
        text = method(text, 'refresh', `  async refresh() {
    const epoch = ++this.refreshEpoch;
    this._set({loading: true});
    const results = await Promise.allSettled([trainingService.listRecordings(), trainingService.getTrainingStatus(), modelService.listModels()]);
    if (this.destroyed || epoch !== this.refreshEpoch) return;
    const next = {loading: false, readErrors: []};
    for (const [index, key, loaded] of [[0,'recordings','recordingsLoaded'],[1,'trainingStatus','statusLoaded'],[2,'models','modelsLoaded']]) {
      const result = results[index];
      next[loaded] = result.status === 'fulfilled';
      if (result.status === 'fulfilled') next[key] = index === 2 ? result.value.models : result.value;
      else next.readErrors.push((index === 0 ? 'Recordings: ' : index === 1 ? 'Training: ' : 'Models: ') + readError(result.reason));
    }
    this._set(next);
  }`);
        text = method(text, '_startRec', `  async _startRec() {
    this._set({error: null});
    try { await trainingService.startRecording({id: 'rec'}); await this.refresh(); }
    catch (e) { this._set({error: 'Recording failed: ' + e.message}); }
  }`);
        text = method(text, '_stopRec', `  async _stopRec() {
    this._set({error: null});
    try { await trainingService.stopRecording(); await this.refresh(); }
    catch (e) { this._set({error: 'Stop recording failed: ' + e.message}); }
  }`);
        text = method(text, '_launchTraining', `  async _launchTraining() {
    if (!this.state.recordingsLoaded || !this.state.statusLoaded || this.state.trainingStatus?.enabled === false) return;
    this._set({loading: true, error: null});
    try {
      const payload = trainingPayload(this.config.method, this.config, this.state.modelsLoaded ? this.state.models : []);
      await trainingService[this.config.method](payload);
      this.progressData = {losses: [], pcks: []};
      trainingService.connectProgressStream();
      await this.refresh();
    } catch (e) {
      this._set({loading: false, error: 'Training failed: ' + e.message});
    }
  }`);
        text = method(text, '_stopTraining', `  async _stopTraining() {
    if (this.state.stopPending) return;
    this._set({stopPending: true, error: null});
    try { await trainingService.stopTraining(); await this.refresh(); }
    catch (e) { this._set({error: 'Stop failed: ' + e.message}); }
    finally { this._set({stopPending: false}); }
  }`);
        text = patch(text, "  _set(p) { Object.assign(this.state, p); this.render(); }", "  _set(p) { if (this.destroyed) return; Object.assign(this.state, p); this.render(); }");
        text = patch(text, "    const el = this.container;\n    el.innerHTML = '';", "    if (this.destroyed) return;\n    const el = this.container, restore = preserveFocus(el);\n    el.innerHTML = '';");
        text = patch(text, "    el.appendChild(panel);", `    if (this.state.readErrors?.length) {
      const error = this._el('p', 'tp-error', this.state.readErrors.join(' ') + ' Previously loaded data may be stale.');
      error.setAttribute('role', 'status'); panel.appendChild(error);
    }
    const retry = this._btn('Refresh', 'tp-btn tp-btn-secondary', () => this.refresh());
    retry.disabled = this.state.loading; retry.dataset.ruviewFocus = 'refresh'; panel.appendChild(retry);
    const stop = this._btn('Stop Training', 'tp-btn tp-btn-danger', () => this._stopTraining());
    stop.dataset.ruviewAction = 'training_stop'; stop.dataset.ruviewFocus='training_stop'; stop.disabled=!!this.state.stopPending; panel.appendChild(stop);
    el.appendChild(panel); restore();`);
        text = patch(text, "let cls = 'tp-badge tp-badge-idle', txt = 'Idle';", "let cls = 'tp-badge tp-badge-idle', txt = this.state.statusLoaded ? 'Idle' : 'Status unknown';");
        text = patch(text, "    if (ts && ts.active) {", "    if (this.state.statusLoaded && ts && ts.active) {");
        text = patch(text, "if (this.state.recordings.length === 0 && !this.state.loading)", "if (this.state.recordingsLoaded && this.state.recordings.length === 0 && !this.state.loading)");
        const start = text.indexOf('    if (this.state.isRecording) {');
        const end = text.indexOf('    s.appendChild(acts);', start);
        if (start < 0 || end < 0) throw Error('Pinned recording controls changed');
        text = text.slice(0,start) + `    const state = trainingService.recordingState;
    const note = this._el('p', 'tp-empty', state === 'recording' ? 'Recording — stop it manually' : state === 'idle' ? 'Recording stopped' : 'Recording status unknown');
    note.setAttribute('role', 'status'); s.appendChild(note);
    s.appendChild(this._el('p', 'tp-empty', 'Recording continues until you stop it, even if you close this view.'));
    for (const [label, action, run] of [['Start Recording','recording_start',()=>this._startRec()],['Stop Recording','recording_stop',()=>this._stopRec()]]) {
      const button = this._btn(label, 'tp-btn tp-btn-rec', run);
      button.dataset.ruviewAction = action; button.dataset.ruviewFocus = action;
      button.disabled = trainingService.recordingBusy || action === 'recording_start' && state === 'recording';
      button.setAttribute('aria-busy', String(trainingService.recordingBusy)); acts.appendChild(button);
    }
` + text.slice(end);
        text = method(text, '_renderConfig', `  _renderConfig() {
    const s = this._el('div', 'tp-section');
    s.appendChild(this._el('div', 'tp-section-title', 'Training Configuration'));
    const form = this._el('div', 'tp-config-form');
    const field = (key, label, input) => {
      input.id = this.fieldPrefix + '-' + key; input.dataset.ruviewFocus = key; input.className = 'tp-input';
      const row = this._el('div', 'tp-input-row'), lb = this._el('label', 'tp-label', label);
      lb.htmlFor = input.id; row.append(lb, input); form.appendChild(row); return input;
    };
    const select = (key,label,entries) => {
      const input = document.createElement('select');
      for (const [value,label] of entries) { const option = this._el('option', null, label); option.value=value; input.appendChild(option); }
      input.value = this.config[key]; input.addEventListener('change',()=>{this.config[key]=input.value;this.render();});
      return field(key,label,input);
    };
    select('method','Training type',[['startTraining','Training'],['startPretraining','Pretrain'],['startLoraTraining','LoRA']]);
    const datasets = this._el('fieldset', 'tp-ds-container'); datasets.appendChild(this._el('legend', 'tp-label', 'Datasets'));
    for (const rec of this.state.recordings) {
      const label = this._el('label','tp-ds-item'), cb = document.createElement('input');
      cb.type='checkbox'; cb.id=this.fieldPrefix+'-dataset-'+datasets.children.length; label.htmlFor=cb.id; cb.dataset.ruviewFocus='dataset-' + rec.id; cb.checked=this.config.selectedRecordings.includes(rec.id); cb.disabled=!this.state.recordingsLoaded;
      cb.addEventListener('change',()=>{this.config.selectedRecordings=cb.checked?[...new Set([...this.config.selectedRecordings,rec.id])]:this.config.selectedRecordings.filter(id=>id!==rec.id);});
      label.append(cb,this._el('span',null,rec.name||rec.id));datasets.appendChild(label);
    }
    form.appendChild(datasets);
    const numeric = (key,label,integer=true) => {
      const input=document.createElement('input');input.type='number';input.min=integer?'1':'0';input.step=integer?'1':'any';input.value=this.config[key];
      input.addEventListener('input',()=>{this.config[key]=input.valueAsNumber;});field(key,label,input);
    };
    numeric('epochs','Epochs');
    if (this.config.method !== 'startLoraTraining') numeric('learning_rate','Learning Rate',false);
    if (this.config.method === 'startTraining') { numeric('batch_size','Batch Size'); numeric('patience','Early Stop Patience'); }
    if (this.config.method === 'startLoraTraining') {
      select('base_model','Base Model',[['','Select a model'],...this.state.models.map(m=>[m.id,m.name||m.id])]).disabled=!this.state.modelsLoaded;
      const input=document.createElement('input');input.type='text';input.value=this.config.lora_profile_name;input.maxLength=128;
      input.addEventListener('input',()=>{this.config.lora_profile_name=input.value;});field('lora_profile_name','LoRA Profile',input);
      form.appendChild(this._el('p','tp-empty','LoRA rank: 8. Learning rate is controlled by this server.'));
    }
    s.appendChild(form);
    const disabled=this.state.trainingStatus?.enabled===false;
    if (disabled) s.appendChild(this._el('p','tp-empty','In-server training is disabled on this build.'));
    const button=this._btn('Start Training','tp-btn tp-btn-success',()=>this._launchTraining());
    button.dataset.ruviewAction='training_start';button.dataset.ruviewFocus='training_start';
    button.disabled=this.state.loading||disabled||!this.state.recordingsLoaded||!this.state.statusLoaded||(this.config.method==='startLoraTraining'&&!this.state.modelsLoaded);
    s.appendChild(button);return s;
  }`);
        text = patch(text, "    this.unsubscribers.forEach(fn => fn());", "    this.destroyed = true; ++this.refreshEpoch;\n    this.unsubscribers.forEach(fn => fn());");
        text = patch(text, "    if (this.state.error) panel.appendChild(this._el('div', 'tp-error', this.state.error));", "    if (this.state.error) { const error=this._el('div','tp-error',this.state.error); error.setAttribute('role','alert'); panel.appendChild(error); }");
        text = patch(text, "    const stop = this._btn('Stop Training', 'tp-btn tp-btn-danger', () => this._stopTraining());\n    stop.disabled = this.state.loading; stop.style.marginTop = '10px'; s.appendChild(stop);", '');
        text = patch(text, "        del.disabled = this.state.loading;", "        del.dataset.ruviewAction='recording_delete'; del.disabled = this.state.loading || !this.state.recordingsLoaded;");
        text = patch(text, "    const active = ts && ts.active;", "    const active = this.state.statusLoaded && ts && ts.active;");
        // Upstream repeats this branch, so keep replaceAll semantics but fail when the anchor is gone.
        const progressBranch = 'else if (ts && !ts.active && this.progressData.losses.length > 0)';
        if (!text.includes(progressBranch)) throw Error('Pinned RuView adapter patch no longer matches');
        text = text.replaceAll(progressBranch, 'else if (this.state.statusLoaded && ts && !ts.active && this.progressData.losses.length > 0)');
        text = patch(text, "    panel.appendChild(this._renderHeader());", "    panel.setAttribute('aria-busy', String(this.state.loading));\n    panel.appendChild(this._renderHeader());\n    if (this.state.loading) { const pending=this._el('p','tp-empty','Loading…'); pending.setAttribute('role','status'); panel.appendChild(pending); }");
        text = patch(text, "    h.appendChild(this._el('span', cls, txt));", "    const badge=this._el('span',cls,txt);badge.setAttribute('role','status');h.appendChild(badge);");
        text = patch(text, "txt = 'Training';", "txt = ts.phase === 'stopping' ? 'Stopping training…' : 'Training';");
        text = patch(text, "    this._set({ trainingStatus: { ...this.state.trainingStatus, ...data } });", "    this._set({ trainingStatus: { ...this.state.trainingStatus, ...data }, statusLoaded: typeof data?.active === 'boolean' || this.state.statusLoaded });");
    }
    if (name === 'components/ModelPanel.js') {
        text="import {readError, preserveFocus} from '../control-contracts.js';\n"+text;
        text=patch(text,'    this.state = { models:', '    this.refreshEpoch=0; this.destroyed=false;\n    this.state = { modelsLoaded:false, activeLoaded:false, profilesLoaded:false, models:');
        text=method(text,'refresh',`  async refresh() {
    const epoch=++this.refreshEpoch;this._set({loading:true});
    const results=await Promise.allSettled([modelService.listModels(),modelService.getActiveModel(),modelService.getLoraProfiles()]);
    if(this.destroyed||epoch!==this.refreshEpoch)return;
    const next={loading:false,readErrors:[]};
    for(const [index,key,loaded] of [[0,'models','modelsLoaded'],[1,'activeModel','activeLoaded'],[2,'loraProfiles','profilesLoaded']]) {
      const result=results[index];next[loaded]=result.status==='fulfilled';
      if(result.status==='fulfilled')next[key]=index===0?result.value.models:result.value;
      else next.readErrors.push(['Models: ','Active model: ','LoRA profiles: '][index]+readError(result.reason));
    }
    this._set(next);
  }`);
        text=patch(text,'  _set(p) { Object.assign(this.state, p); this.render(); }','  _set(p) { if(this.destroyed)return; Object.assign(this.state, p); this.render(); }');
        text=patch(text,'    const el = this.container;','    if(this.destroyed)return;\n    const el = this.container, restore=preserveFocus(el);');
        text=patch(text,"    const panel = this._el('div', 'mp-panel');",`    const panel = this._el('div', 'mp-panel');
    panel.setAttribute('aria-busy',String(this.state.loading));
    const message=this.state.loading?'Loading…':this.state.readErrors?.length?this.state.readErrors.join(' ')+' Previously loaded data may be stale.':'';
    if(message){const note=this._el('p','mp-error',message);note.setAttribute('role','status');panel.appendChild(note);}`);
        text=patch(text,"String(this.state.models.length)","this.state.modelsLoaded?String(this.state.models.length):'Unknown'");
        text=patch(text,"    if (this.state.error) panel.appendChild(this._el('div', 'mp-error', this.state.error));","    if (this.state.error) {const error=this._el('div','mp-error',this.state.error);error.setAttribute('role','alert');panel.appendChild(error);}");
        text=patch(text,'if (models.length === 0 && !this.state.loading)','if (this.state.modelsLoaded && models.length === 0 && !this.state.loading)');
        text=patch(text,'    rb.disabled = this.state.loading;',"    rb.dataset.ruviewFocus='model-refresh';rb.disabled = this.state.loading;");
        text=patch(text,'    el.appendChild(panel);','    el.appendChild(panel);restore();');
        text=patch(text,"      sel.className = 'mp-lora-select';","      sel.className = 'mp-lora-select';sel.dataset.ruviewAction='lora_activate';sel.setAttribute('aria-label','LoRA Profile');");
        for(const [variable,action] of [['ub','model_unload'],['lb','model_load'],['db','model_delete']]) {
            const focus=variable==='ub'?`'${action}'`:`'${action}-'+model.id`;
            text=patch(text,`    ${variable}.disabled = this.state.loading;`,`    ${variable}.dataset.ruviewAction='${action}';${variable}.dataset.ruviewFocus=${focus};\n    ${variable}.disabled = this.state.loading || !this.state.modelsLoaded;`);
        }
        text=patch(text,'    this.unsubs.forEach(fn => fn());','    this.destroyed=true;++this.refreshEpoch;\n    this.unsubs.forEach(fn => fn());');
    }
    if(name==='pose-fusion/js/main.js')text=patch(text,"let mode = 'dual';","let mode = 'csi';");
    if(name==='pose-fusion.html') {
        text=patch(text,'<div class="header-title">Dual-Modal Pose Estimation — Live Video + WiFi CSI Fusion</div>','<div class="header-title" id="camera-unavailable">CSI Only — camera and video modes are unavailable in AuraGo.</div>');
        text=patch(text,'<select id="mode-select" class="mode-select">','<select id="mode-select" class="mode-select" aria-label="Sensing mode" aria-describedby="camera-unavailable">');
        text=patch(text,'<option value="dual">','<option value="dual" disabled>');
        text=patch(text,'<option value="video">','<option value="video" disabled>');
        text=patch(text,'<option value="csi">','<option value="csi" selected>');
        text=patch(text,'<button id="start-camera-btn">','<button id="start-camera-btn" data-ruview-action="camera" disabled>');
    }
    return text;
}
