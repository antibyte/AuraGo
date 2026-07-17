import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const languages = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];

function read(relativePath) {
  return readFileSync(path.join(root, relativePath), 'utf8').replace(/\r\n?/g, '\n');
}

class TestCustomEvent extends Event {
  constructor(type, options = {}) {
    super(type);
    this.detail = options.detail;
  }
}

function loadAudioGate() {
  class MockVAD {
    async load() {}
    async isSpeech(frame) {
      return frame[0] > 0;
    }
    reset() {}
  }
  const window = {
    AuraSileroVAD: { SileroVAD: MockVAD }
  };
  const context = {
    window,
    Event,
    EventTarget,
    CustomEvent: TestCustomEvent,
    Float32Array,
    Promise,
    navigator: {},
    console
  };
  vm.createContext(context);
  vm.runInContext(read('ui/js/realtime-speech/audio-engine.js'), context);
  return window.AuraRealtimeAudio;
}

async function testLocalAudioGate() {
  const audio = loadAudioGate();
  assert.deepEqual(
    { ...audio.constants },
    {
      SAMPLE_RATE: 16000,
      CHUNK_SAMPLES: 512,
      PRE_ROLL_SAMPLES: 4800,
      CONFIRM_SAMPLES: 1536,
      TURN_SILENCE_SAMPLES: 10400,
      WAKE_BUFFER_SAMPLES: 48000
    },
    'V1 timing and wake-buffer constants must remain pinned'
  );

  const gate = new audio.RealtimeAudioGate();
  gate.started = true;
  const events = [];
  for (const type of ['speechstart', 'speechend', 'audio']) {
    gate.addEventListener(type, event => events.push({ type, detail: event.detail }));
  }
  const frame = value => {
    const output = new Float32Array(512);
    output.fill(value);
    return output;
  };

  for (let index = 0; index < 10; index++) await gate.processFrame(frame(0));
  await gate.processFrame(frame(1));
  await gate.processFrame(frame(1));
  assert.equal(events.filter(event => event.type === 'speechstart').length, 0, 'less than 96 ms must not confirm speech');
  await gate.processFrame(frame(1));
  const start = events.find(event => event.type === 'speechstart');
  assert.ok(start, '96 ms of speech must start a turn');
  assert.equal(start.detail.preRollSamples, 4608, 'the retained pre-roll must stay below the 300 ms cap');
  assert.equal(start.detail.audio.length, 6144, 'speech start must include retained pre-roll plus confirmation audio');

  for (let index = 0; index < 20; index++) await gate.processFrame(frame(0));
  assert.equal(events.filter(event => event.type === 'speechend').length, 0, '640 ms of silence must keep the turn open');
  await gate.processFrame(frame(0));
  assert.equal(events.filter(event => event.type === 'speechend').length, 1, 'silence crossing 650 ms must close the turn');
}

function testAudioWorkletResampling() {
  let Processor = null;
  class MockAudioWorkletProcessor {
    constructor() {
      this.port = {
        chunks: [],
        postMessage: chunk => this.port.chunks.push(chunk)
      };
    }
  }
  const context = {
    AudioWorkletProcessor: MockAudioWorkletProcessor,
    Float32Array,
    Math,
    sampleRate: 48000,
    registerProcessor(_name, implementation) {
      Processor = implementation;
    }
  };
  vm.createContext(context);
  vm.runInContext(read('ui/js/realtime-speech/audio-worklet.js'), context);
  assert.ok(Processor, 'the realtime audio worklet must register');
  const processor = new Processor();
  const quantum = new Float32Array(128);
  for (let index = 0; index < 375; index++) {
    processor.process([[quantum]]);
  }
  const emitted = processor.port.chunks.reduce((total, chunk) => total + chunk.length, 0);
  const total = emitted + processor.chunkOffset;
  assert.ok(total >= 15998 && total <= 16000, `one 48 kHz second must produce one 16 kHz second, got ${total} samples`);
}

function loadRuntimeForTimerTest() {
  let now = 0;
  let timer = null;
  let timerDelay = null;
  class FakeDate extends Date {
    static now() {
      return now;
    }
  }
  const listeners = new Map();
  const window = {
    AuraRealtimeProviderCommon: {
      randomID: prefix => `${prefix}-test`,
      safeJSON: JSON.parse
    },
    AuraRealtimeAudio: { constants: { WAKE_BUFFER_SAMPLES: 48000 } },
    addEventListener(type, handler) {
      listeners.set(type, handler);
    },
    dispatchEvent() {},
    clearTimeout() {
      timer = null;
      timerDelay = null;
    },
    setTimeout(handler, delay) {
      timer = handler;
      timerDelay = delay;
      return 1;
    }
  };
  const context = {
    window,
    document: { createElement() { throw new Error('modal not expected'); }, body: {} },
    localStorage: { getItem: () => '', setItem() {} },
    BroadcastChannel: undefined,
    CustomEvent: TestCustomEvent,
    Event,
    EventTarget,
    Date: FakeDate,
    Error,
    Promise,
    JSON,
    String,
    Object,
    Array,
    Number,
    Math,
    fetch: async () => { throw new Error('network not expected'); },
    console,
    TextDecoder
  };
  vm.createContext(context);
  vm.runInContext(read('ui/js/realtime-speech/core.js'), context);
  return {
    Runtime: window.AuraRealtimeSpeechRuntime,
    timer: () => timer,
    timerDelay: () => timerDelay,
    setNow(value) {
      now = value;
    }
  };
}

async function testParkingTimerAndGuards() {
  const harness = loadRuntimeForTimerTest();
  const runtime = new harness.Runtime();
  let parked = 0;
  runtime.sessionId = 'session-test';
  runtime.adapter = {
    connected: true,
    async park() {
      parked++;
    }
  };
  runtime.config = { park_after_seconds: 5 };
  runtime.state = 'listening';
  runtime.lastActivityAt = 0;

  harness.setNow(0);
  runtime.schedulePark();
  assert.equal(harness.timerDelay(), 5000, 'complete inactivity must park at exactly five seconds');

  runtime.actionActive = true;
  runtime.schedulePark();
  assert.equal(harness.timer(), null, 'an AuraGo action must suppress parking');
  runtime.actionActive = false;
  runtime.providerSpeaking = true;
  runtime.schedulePark();
  assert.equal(harness.timer(), null, 'provider output must suppress parking');
  runtime.providerSpeaking = false;
  runtime.userSpeaking = true;
  runtime.schedulePark();
  assert.equal(harness.timer(), null, 'user speech must suppress parking');
  runtime.userSpeaking = false;

  await runtime.park();
  assert.equal(parked, 1);
  assert.equal(runtime.state, 'parked');
}

async function testTakeoverPeerProbeAndModalContrast() {
  const harness = loadRuntimeForTimerTest();
  const runtime = new harness.Runtime();
  runtime.createSession = async () => ({ session_id: 'session-before-handshake' });
  runtime.adapter = {
    async connect(options) {
      await options.createSession({});
      throw new Error('provider handshake failed');
    }
  };
  await assert.rejects(runtime.connectAdapter(false), /provider handshake failed/);
  assert.equal(
    runtime.sessionId,
    'session-before-handshake',
    'a provider handshake failure must leave the backend lease available for stop() to release'
  );
  runtime.sessionId = '';

  assert.equal(
    await runtime.probeActiveSession('session-without-channel'),
    null,
    'a browser without BroadcastChannel support must keep the explicit takeover confirmation'
  );

  let listeners = new Set();
  runtime.channel = {
    addEventListener(type, handler) {
      if (type === 'message') listeners.add(handler);
    },
    removeEventListener(type, handler) {
      if (type === 'message') listeners.delete(handler);
    },
    postMessage(message) {
      if (message.type !== 'probe') return;
      for (const listener of [...listeners]) {
        listener({
          data: {
            type: 'active',
            tabId: 'tab-peer',
            sessionId: message.sessionId
          }
        });
      }
    }
  };

  assert.equal(await runtime.probeActiveSession('session-live'), true, 'a matching live tab must be detected');
  assert.equal(listeners.size, 0, 'the peer probe listener must be removed after a response');

  listeners = new Set();
  runtime.channel = {
    addEventListener(type, handler) {
      if (type === 'message') listeners.add(handler);
    },
    removeEventListener(type, handler) {
      if (type === 'message') listeners.delete(handler);
    },
    postMessage() {}
  };
  const staleProbe = runtime.probeActiveSession('session-stale');
  assert.equal(harness.timerDelay(), 500, 'the live-tab probe must remain short and deterministic');
  harness.timer()();
  assert.equal(await staleProbe, false, 'an unanswered lease must be treated as stale');
  assert.equal(listeners.size, 0, 'the peer probe listener must be removed after timeout');

  const core = read('ui/js/realtime-speech/core.js');
  assert.match(
    core,
    /const peerActive = await this\.probeActiveSession\(conflictSessionId\)/,
    'a lease conflict must verify that another tab is actually alive before showing the takeover modal'
  );

  const styles = read('ui/css/realtime-speech.css');
  assert.match(
    styles,
    /\.realtime-speech-modal-backdrop\s*\{[\s\S]*?--rt-accent:\s*var\(--accent,\s*#2fc5a9\)/,
    'the body-level takeover modal must define its own accent color'
  );
  assert.match(
    styles,
    /\.realtime-speech-modal-actions \.primary\s*\{[\s\S]*?background:\s*var\(--rt-accent,\s*#2fc5a9\)/,
    'the takeover action must retain a visible background when theme variables are absent'
  );
}

async function testGeminiBinarySetupFrames() {
  const sockets = [];
  class MockWebSocket extends EventTarget {
    static OPEN = 1;

    constructor(url) {
      super();
      this.url = url;
      this.readyState = MockWebSocket.OPEN;
      this.binaryType = '';
      sockets.push(this);
      queueMicrotask(() => this.dispatchEvent(new Event('open')));
    }

    send(payload) {
      const message = JSON.parse(payload);
      if (!message.setup) return;
      const bytes = new TextEncoder().encode(JSON.stringify({ setupComplete: {} }));
      const data = sockets.length === 1
        ? bytes.buffer
        : new Blob([bytes], { type: 'application/json' });
      queueMicrotask(() => {
        const event = new Event('message');
        Object.defineProperty(event, 'data', { value: data });
        this.dispatchEvent(event);
      });
    }

    close() {
      this.readyState = 3;
    }
  }

  const window = {
    setTimeout,
    clearTimeout,
    crypto,
    AuraRealtimeProviders: {}
  };
  const context = {
    window,
    WebSocket: MockWebSocket,
    Event,
    EventTarget,
    CustomEvent: TestCustomEvent,
    Blob,
    ArrayBuffer,
    Uint8Array,
    Int16Array,
    Float32Array,
    DataView,
    TextDecoder,
    TextEncoder,
    Promise,
    Error,
    String,
    Object,
    Array,
    Math,
    JSON,
    btoa,
    atob,
    console
  };
  vm.createContext(context);
  vm.runInContext(read('ui/js/realtime-speech/provider-common.js'), context);
  vm.runInContext(read('ui/js/realtime-speech/provider-gemini.js'), context);

  const Adapter = window.AuraRealtimeProviders.gemini;
  const adapter = new Adapter({});
  await adapter.openSocket({
    websocket_url: 'wss://example.invalid/live',
    access_token: 'ephemeral-test-token',
    setup: { model: 'models/test' }
  });

  assert.equal(sockets.length, 1);
  assert.equal(sockets[0].binaryType, 'arraybuffer', 'Gemini must request deterministic binary frame delivery');
  assert.equal(adapter.connected, true, 'a binary setupComplete frame must finish the handshake');

  const blobAdapter = new Adapter({});
  await blobAdapter.openSocket({
    websocket_url: 'wss://example.invalid/live',
    access_token: 'ephemeral-test-token',
    setup: { model: 'models/test' }
  });
  assert.equal(sockets.length, 2);
  assert.equal(blobAdapter.connected, true, 'a Blob setupComplete frame must finish the handshake');

  const transcripts = [];
  blobAdapter.addEventListener('transcript', event => transcripts.push(event.detail));
  blobAdapter.handleEvent({
    serverContent: {
      modelTurn: {
        parts: [{
          text: '**Gathering AI News** I will call aurago_execute.',
          thought: true
        }]
      }
    }
  });
  assert.equal(
    transcripts.length,
    0,
    'Gemini modelTurn text must not expose internal planning as an audio transcript'
  );
  blobAdapter.handleEvent({
    serverContent: {
      outputTranscription: { text: 'Ich prüfe das.' }
    }
  });
  assert.equal(transcripts.at(-1)?.text, 'Ich prüfe das.');
}

function testProviderContractAndSecurityBoundaries() {
  const required = ['connect', 'sendAudio', 'endTurn', 'sendToolResult', 'interruptOutput', 'park', 'resume', 'close'];
  for (const provider of ['openai', 'xai', 'gemini']) {
    const source = read(`ui/js/realtime-speech/provider-${provider}.js`);
    for (const method of required) {
      assert.match(source, new RegExp(`(?:async\\s+)?${method}\\s*\\(`), `${provider} must implement ${method}`);
    }
    assert.doesNotMatch(source, /localStorage|sessionStorage/, `${provider} adapter must not persist credentials`);
  }
  assert.doesNotMatch(
    read('ui/js/realtime-speech/provider-openai.js'),
    /remoteAudio\.addEventListener\(['"]play['"]/,
    'the warm WebRTC media track must not be mistaken for active provider speech'
  );
  assert.match(
    read('ui/js/realtime-speech/provider-xai.js'),
    /player\.whenIdle\(\)\.then/,
    'xAI must wait for queued acknowledgement audio before creating the post-tool response'
  );
  assert.match(
    read('ui/js/realtime-speech/provider-xai.js'),
    /input_audio_transcription\.updated/,
    'xAI must handle its cumulative transcription update event'
  );
  assert.match(
    read('ui/js/realtime-speech/provider-xai.js'),
    /const resuming = !!resumeConversationId[\s\S]*if \(!resuming\) await this\.syncContext/,
    'xAI must not replay visible chat context into a resumed provider conversation'
  );
  assert.match(
    read('ui/js/realtime-speech/provider-gemini.js'),
    /turnCompleteSeen[\s\S]*scheduleTurnFinalize/,
    'Gemini must tolerate transcripts that arrive after turnComplete'
  );
  assert.match(
    read('ui/js/realtime-speech/provider-gemini.js'),
    /const resuming = !!resumeHandle[\s\S]*if \(!resuming\) await this\.syncContext/,
    'Gemini must not replay visible chat context after session resumption'
  );
  assert.match(
    read('ui/js/realtime-speech/provider-gemini.js'),
    /payload\.error[\s\S]*rejectBeforeSetup\(new Error\('Gemini Live rejected session setup:/,
    'Gemini must surface a setup rejection instead of timing out'
  );
  assert.match(
    read('ui/js/realtime-speech/provider-gemini.js'),
    /socket\.addEventListener\('close', event => \{[\s\S]*rejectBeforeSetup\(new Error\('Gemini Live closed before setup completed'/,
    'Gemini must reject immediately when the socket closes before setup completes'
  );
  assert.match(
    read('ui/js/realtime-speech/provider-gemini.js'),
    /data instanceof Blob[\s\S]*data instanceof ArrayBuffer[\s\S]*TextDecoder/,
    'Gemini must decode the binary WebSocket frames used by the Live API'
  );
  assert.doesNotMatch(
    read('ui/js/realtime-speech/provider-gemini.js'),
    /part\.text[\s\S]{0,200}assistantTranscript/,
    'Gemini modelTurn text must never be exposed as the spoken audio transcript'
  );

  const core = read('ui/js/realtime-speech/core.js');
  assert.doesNotMatch(core, /\balert\s*\(/, 'microphone takeover must use an AuraGo modal');
  assert.match(core, /BroadcastChannel\(CHANNEL_NAME\)/, 'same-origin tabs must coordinate microphone ownership');
  assert.match(core, /MAX_WAKE_SAMPLES/, 'wake audio must be bounded');
  assert.match(core, /detail\.speech\s*===\s*true\)\s*this\.handleAudio/, 'VAD hangover silence must remain local');
  assert.match(core, /wake_latency_ms:\s*Math\.max/, 'provider wake latency must be reported without transcript content');
  assert.match(core, /syncServerState\(this\.state,\s*\{\s*usage:\s*this\.usage\s*\}\)/, 'numeric provider usage must be forwarded as lifecycle telemetry');
  assert.match(core, /error_message:\s*this\.lastErrorMessage/, 'provider setup errors must be reported to the backend');
  assert.match(core, /window\.console\.error\('\[RealtimeSpeech\] '/, 'provider setup errors must remain visible in the browser console');
  assert.match(
    read('ui/js/realtime-speech/panel.js'),
    /data-realtime-error-message[\s\S]*message\.textContent = errorMessage/,
    'the live speech panel must preserve the sanitized provider error during incremental updates'
  );
  const panelSource = read('ui/js/realtime-speech/panel.js');
  const refreshAllSource = panelSource.match(/function refreshAll\(\) \{[\s\S]*?\n    \}/)?.[0] || '';
  const updatePanelSource = panelSource.match(/function updatePanel\(root, options\) \{[\s\S]*?\n    \}\n\n    function render/)?.[0] || '';
  assert.doesNotMatch(
    refreshAllSource,
    /\brender\(root,\s*options\)/,
    'state changes must update the existing Live Speech card instead of recreating it'
  );
  assert.doesNotMatch(
    updatePanelSource,
    /root\.innerHTML\s*=/,
    'incremental Live Speech updates must preserve the mounted card DOM node'
  );
  const realtimeStyles = read('ui/css/realtime-speech.css');
  assert.match(
    realtimeStyles,
    /\.realtime-speech-status-row\s*\{[\s\S]*?min-width:\s*0;[\s\S]*?overflow:\s*hidden;/,
    'the Live Speech status row must constrain long captions to the card'
  );
  assert.match(
    realtimeStyles,
    /\.realtime-speech-live-caption\s*\{[\s\S]*?flex:\s*1 1 0;[\s\S]*?text-overflow:\s*ellipsis;/,
    'long Live Speech captions must shrink and use an ellipsis'
  );
  assert.match(core, /action\.cancelled\s*=\s*true/, 'explicit cancellation must mark the in-flight action');
  assert.match(core, /if \(action\.cancelled\) status = 'cancelled'/, 'a completed stream must not overwrite cancellation');
  assert.doesNotMatch(core, /profile\.api_key(?!_set)/, 'the browser runtime must not read a permanent provider key');
  assert.doesNotMatch(core, /['"]api_key['"]\s*:/, 'the browser runtime must not submit a permanent provider key');

  const webChat = read('ui/index.html');
  const desktop = read('ui/desktop.html');
  assert.match(webChat, /id="realtime-speech-btn"/);
  assert.match(desktop, /data-realtime-speech-launcher/);
  assert.match(read('ui/css/desktop-realtime-speech.css'), /data-theme="fruity"/);
  assert.match(read('ui/css/desktop-realtime-speech.css'), /data-theme="standard"/);
}

function testTranslationParity() {
  const categoryFilters = {
    chat: key => key.startsWith('chat.realtime_'),
    config: key => key.includes('realtime_speech'),
    dashboard: key => key.includes('realtime_speech'),
    desktop: key => key.includes('live_speech')
  };
  for (const [category, filter] of Object.entries(categoryFilters)) {
    const english = JSON.parse(read(`ui/lang/${category}/en.json`));
    const keys = Object.keys(english).filter(filter);
    assert.ok(keys.length > 0, `${category} must contain realtime speech translations`);
    for (const language of languages) {
      const locale = JSON.parse(read(`ui/lang/${category}/${language}.json`));
      for (const key of keys) {
        assert.equal(typeof locale[key], 'string', `${category}/${language} is missing ${key}`);
        assert.ok(locale[key].trim(), `${category}/${language} has an empty ${key}`);
      }
    }
  }
}

await testLocalAudioGate();
testAudioWorkletResampling();
await testParkingTimerAndGuards();
await testTakeoverPeerProbeAndModalContrast();
await testGeminiBinarySetupFrames();
testProviderContractAndSecurityBoundaries();
testTranslationParity();

console.log('Realtime Speech browser contract tests passed.');
