(function (root) {
  "use strict";

  // Project timeline, live playback and offline WAV rendering. The voice
  // engine (patches, buses, voices, controllers) is synth-studio-voices.js.

  const MAX_TRACKS = 64;
  const MAX_NOTES = 12000;
  const MAX_ACTIVE_NOTES = 256;
  const MAX_RENDER_SECONDS = 300;
  const MAX_EFFECT_TAIL_SECONDS = 3.5;
  const AUTOMATION_PARAMETERS = ["tone", "resonance", "drive", "reverb", "delay"];
  const DEFAULT_LOOKAHEAD = 0.12;
  const MAX_LOOKAHEAD = 1;
  const TICK_MS = 25;

  const {
    DEFAULT_PARAMS,
    clamp,
    operatorsFor,
    voiceReleaseSeconds,
    makeMix,
    createTrackBus,
    updateTrackBus,
    updateTrackParameter,
    scheduleMetronome,
    createVoice,
    disposeVoice,
    releaseVoice,
    makeTrackState,
    applyController,
    releaseTrackState
  } = root.SynthStudioVoices;

  function codedError(code, message) {
    const error = new Error(message);
    error.code = code;
    return error;
  }

  function budgetError(message) {
    return codedError("SYNTH_STUDIO_RENDER_BUDGET", message);
  }

  function exportLimitError(message) {
    return codedError("export_limit", message);
  }

  function audioContextConstructor() {
    return root.AudioContext || root.webkitAudioContext;
  }

  function offlineContextConstructor() {
    return root.OfflineAudioContext || root.webkitOfflineAudioContext;
  }

  const FX_PARAM_LIMITS = {    mix: [0, 1], rate: [0.02, 40], depth: [0, 1], feedback: [0, 0.95], tone: [0, 1],
    amount: [0, 2], drive: [0, 1], freq: [20, 20000], low: [-18, 18], mid: [-18, 18],
    high: [-18, 18], bits: [1, 16], time: [0.005, 2], size: [0, 1], decay: [0, 1], model: [0, 3]
  };

  function normalizeInserts(list) {
    if (!Array.isArray(list)) return [];
    const output = [];
    for (const entry of list) {
      if (!entry || typeof entry !== "object" || typeof entry.type !== "string") continue;
      const spec = { type: entry.type };
      for (const key of Object.keys(FX_PARAM_LIMITS)) {
        if (entry[key] === undefined) continue;
        const [low, high] = FX_PARAM_LIMITS[key];
        const value = Number(entry[key]);
        spec[key] = Number.isFinite(value) ? Math.min(high, Math.max(low, value)) : low;
      }
      output.push(spec);
      if (output.length >= 4) break;
    }
    return output;
  }

  function normalizeMaster(source) {
    const s = source && typeof source === "object" ? source : {};
    return {
      volume: clamp(s.volume, 0, 1.5, 0.78),
      low: clamp(s.low, -18, 18, 0),
      mid: clamp(s.mid, -18, 18, 0),
      high: clamp(s.high, -18, 18, 0),
      threshold: clamp(s.threshold, -60, 0, -8),
      ratio: clamp(s.ratio, 1, 20, 10),
      knee: clamp(s.knee, 0, 40, 8),
      attack: clamp(s.attack, 0, 1, 0.003),
      release: clamp(s.release, 0, 1, 0.18),
      width: clamp(s.width, 0, 2, 1),
      plateSize: clamp(s.plateSize, 0, 1, 0.5),
      plateDecay: clamp(s.plateDecay, 0, 1, 0.65),
      echoTime: clamp(s.echoTime, 0.01, 2, 0.375),
      echoFeedback: clamp(s.echoFeedback, 0, 0.85, 0.28),
      echoSync: typeof s.echoSync === "string" ? s.echoSync : "1/8"
    };
  }

  function normalizeProject(project, mixOnly = false) {
    if (!project || typeof project !== "object" || !Array.isArray(project.tracks)) {
      throw new TypeError("Synth Studio project needs a tracks array.");
    }
    if (project.tracks.length > MAX_TRACKS) throw budgetError(`A project can have at most ${MAX_TRACKS} audio tracks.`);
    const tempo = clamp(project.tempo, 20, 300, 120);
    const ppq = Math.round(clamp(project.ppq, 24, 1920, 480));
    const tracks = [];
    const ids = new Set();
    let noteCount = 0, eventCount = 0;
    let endTick = 0;
    for (let index = 0; index < project.tracks.length; index++) {
      const source = project.tracks[index] || {};
      const id = String(source.id || `track-${index + 1}`);
      if (ids.has(id)) throw new TypeError(`Duplicate audio track id: ${id}`);
      ids.add(id);
      const track = {
        id,
        name: String(source.name || id),
        instrument: String(source.instrument || "lead-saw"),
        volume: clamp(source.volume, 0, 1.5, 0.8),
        pan: clamp(source.pan, -1, 1, 0),
        mute: Boolean(source.mute),
        solo: Boolean(source.solo),
        params: { ...DEFAULT_PARAMS, ...(source.params || {}) },
        fx: { inserts: normalizeInserts(source.fx && source.fx.inserts) },
        clips: [],
        automation: []
      };
      track.params.tone = clamp(track.params.tone, 0, 1, DEFAULT_PARAMS.tone);
      track.params.resonance = clamp(track.params.resonance, 0, 1, DEFAULT_PARAMS.resonance);
      track.params.drive = clamp(track.params.drive, 0, 1, DEFAULT_PARAMS.drive);
      track.params.attack = clamp(track.params.attack, 0, 10, DEFAULT_PARAMS.attack);
      track.params.release = clamp(track.params.release, 0, 30, DEFAULT_PARAMS.release);
      track.params.reverb = clamp(track.params.reverb, 0, 1, DEFAULT_PARAMS.reverb);
      track.params.delay = clamp(track.params.delay, 0, 1, DEFAULT_PARAMS.delay);
      if (mixOnly) { tracks.push(track); continue; }
      for (let clipIndex = 0; clipIndex < (Array.isArray(source.clips) ? source.clips.length : 0); clipIndex++) {
        const input = source.clips[clipIndex] || {};
        const start = Math.max(0, Math.floor(clamp(input.start, 0, 1e9, 0)));
        const length = Math.max(0, Math.floor(clamp(input.length, 0, 1e9, 0)));
        const clip = { id: String(input.id || `${id}-clip-${clipIndex + 1}`), name: String(input.name || ""), start, length, notes: [], controllers: [] };
        for (let noteIndex = 0; noteIndex < (Array.isArray(input.notes) ? input.notes.length : 0); noteIndex++) {
          const note = input.notes[noteIndex] || {};
          const localStart = Math.floor(clamp(note.start, 0, 1e9, 0));
          const duration = Math.max(1, Math.floor(clamp(note.duration, 1, 1e9, 1)));
          const pitch = Math.round(clamp(note.pitch, 0, 127, -1));
          if (pitch < 0) continue;
          const absoluteStart = start + localStart;
          const absoluteEnd = absoluteStart + duration;
          if (absoluteEnd > Number.MAX_SAFE_INTEGER) throw new TypeError("Audio note tick is outside the safe range.");
          clip.notes.push({
            id: String(note.id || `${clip.id}-note-${noteIndex + 1}`),
            pitch, start: localStart, duration,
            velocity: clamp(note.velocity, 0, 1, 0.8),
            absoluteStart, absoluteEnd,
            key: `${id}/${clip.id}/${noteIndex}`
          });
          noteCount++;
          endTick = Math.max(endTick, absoluteEnd);
        }
        for (const controller of Array.isArray(input.controllers) ? input.controllers : []) {
          if (!controller || !["sustain", "bend", "modulation"].includes(controller.type)) continue;
          const tick = Math.max(0, Math.floor(clamp(controller.tick, 0, 1e9, 0)));
          const value = controller.type === "bend"
            ? clamp(controller.value, -1, 1, 0)
            : clamp(controller.value, 0, 1, 0);
          clip.controllers.push({ tick, type: controller.type, value, absoluteTick: start + tick });
          endTick = Math.max(endTick, start + tick);
        }
        track.clips.push(clip);
        endTick = Math.max(endTick, start + length);
      }
      if(source.automation!==undefined&&!Array.isArray(source.automation))throw new TypeError('Invalid automation.');
      track.automation=(source.automation||[]).map(p=>{
        if(!p||!AUTOMATION_PARAMETERS.includes(p.parameter)||!Number.isInteger(p.tick)||p.tick<0||p.tick>10000000||!Number.isFinite(p.value)||p.value<0||p.value>1)throw new TypeError('Invalid automation point.');
        endTick=Math.max(endTick,p.tick);
        return {...p};
      }).sort((a,b)=>a.tick-b.tick);
      eventCount+=track.automation.length+track.clips.reduce((n,c)=>n+c.notes.length+c.controllers.length,0);
      if(eventCount>50000)throw budgetError('The project exceeds 50,000 notes, controllers and automation points.');
      tracks.push(track);
    }
    if (noteCount > MAX_NOTES) throw budgetError(`A project can contain at most ${MAX_NOTES} notes.`);
    const loop = project.loop && typeof project.loop === "object" ? {
      enabled: Boolean(project.loop.enabled),
      start: Math.max(0, Math.floor(clamp(project.loop.start, 0, 1e9, 0))),
      end: Math.max(0, Math.floor(clamp(project.loop.end, 0, 1e9, 0)))
    } : { enabled: false, start: 0, end: 0 };
    if (loop.enabled && loop.end <= loop.start) throw new TypeError("An enabled loop needs an end tick after its start tick.");
    if (loop.enabled && (loop.end - loop.start) * (60 / tempo / ppq) < 0.05) {
      throw new TypeError("An enabled loop must be at least 50 ms long.");
    }
    return { tracks, tempo, ppq, beatsPerBar: Math.round(clamp(project.beatsPerBar, 1, 16, 4)), loop, endTick, noteCount, metronome: project.metronome === true, master: normalizeMaster(project.master) };
  }

  function isAudible(track, anySolo) {
    return !track.mute && (!anySolo || track.solo);
  }

  function allEvents(info) {
    const events = [];
    for (const track of info.tracks) {
      for(const point of track.automation)events.push({type:'automation',tick:point.tick,track,point});
      for (const clip of track.clips) {
        for (const note of clip.notes) {
          events.push({ type: "noteOn", tick: note.absoluteStart, ownerTick: note.absoluteStart, key: note.key, track, note });
          events.push({ type: "noteOff", tick: note.absoluteEnd, ownerTick: note.absoluteStart, key: note.key, track, note });
        }
        for (const controller of clip.controllers) {
          events.push({ type: "controller", tick: controller.absoluteTick, track, controller });
        }
      }
    }
    if (info.metronome) {
      const end = Math.max(info.endTick, info.loop.enabled ? info.loop.end : 0);
      const count = Math.floor(end / info.ppq) + 1;
      if (count > MAX_NOTES * 2) throw budgetError("Metronome output exceeds the event limit.");
      for (let tick = 0; tick <= end; tick += info.ppq) {
        events.push({ type: "metronome", tick, accent: Math.floor(tick / info.ppq) % info.beatsPerBar === 0 });
      }
    }
    const order = { automation:0, metronome: 0, controller: 1, noteOff: 2, noteOn: 3 };
    events.sort((a, b) => a.tick - b.tick || order[a.type] - order[b.type]);
    return events;
  }

  function seedState(events, info, startTick) {
    const seeds = [];
    for (const track of info.tracks) {
      const latest = new Map();
      for(const parameter of ['tone','reverb']){
        let value=track.params[parameter];
        for(const point of track.automation)if(point.parameter===parameter&&point.tick<startTick)value=point.value;
        seeds.push({type:'automation',tick:startTick,track,point:{parameter,value}});
      }
      for (const event of events) {
        if (event.type !== "controller" || event.track.id !== track.id || event.tick >= startTick) continue;
        const previous = latest.get(event.controller.type);
        if (!previous || event.tick >= previous.tick) latest.set(event.controller.type, event);
      }
      for (const event of latest.values()) {
        seeds.push({
          type: "controller",
          tick: startTick,
          track,
          controller: { ...event.controller, absoluteTick: startTick }
        });
      }
    }
    return seeds;
  }

  function loopControllerSeeds(events, info) {
    const defaults = [];
    for (const track of info.tracks) {
      defaults.push(
        { type: "controller", tick: info.loop.start, track, controller: { type: "sustain", value: 0 } },
        { type: "controller", tick: info.loop.start, track, controller: { type: "bend", value: 0 } },
        { type: "controller", tick: info.loop.start, track, controller: { type: "modulation", value: 0 } }
      );
    }
    return [...defaults, ...seedState(events, info, info.loop.start)];
  }

  function eventsForRange(events, startTick, endTick, initialStart) {
    return events.filter(event => {
      if (event.type === "noteOff") return event.ownerTick >= initialStart && event.ownerTick < endTick;
      return event.tick >= initialStart && event.tick < endTick;
    }).filter(event => event.tick >= startTick || event.type === "noteOff");
  }

  function maxReleaseSeconds(info) {
    let maximum = 0;
    for (const track of info.tracks) {
      maximum = Math.max(maximum, track.params.release);
      const patches = new Map();
      for (const clip of track.clips) for (const note of clip.notes) {
        const key = `${track.instrument}:${note.pitch}`;
        if (!patches.has(key)) patches.set(key, operatorsFor(track, note.pitch));
      }
      for (const operators of patches.values()) {
        for (const operator of operators || []) maximum = Math.max(maximum, clamp(operator.r, 0, 30, 0));
      }
    }
    return maximum;
  }

  function activeVoiceLimit(events, startTick, endTick, secPerTick = 0, repeated = null, terminalTick = endTick) {
    let timeline = events.filter(event => event.type === "controller" || event.type === "noteOn" || event.type === "noteOff");
    if (repeated && secPerTick > 0) {
      const span = endTick - startTick;
      const cycles = Math.ceil(repeated.maxReleaseSeconds / (span * secPerTick)) + 2;
      const perCycle = timeline.length + repeated.seeds.length;
      if (cycles * perCycle > 250000) return MAX_ACTIVE_NOTES + 1;
      const expanded = [];
      for (let cycle = 0; cycle < cycles; cycle++) {
        const offset = cycle * span;
        if (cycle > 0) for (const seed of repeated.seeds) expanded.push({ ...seed, tick: startTick + offset });
        for (const event of timeline) {
          if (cycle > 0 && event.tick < startTick) continue;
          const eventTick = event.type === "noteOff" && event.tick > endTick ? endTick : event.tick;
          expanded.push({ ...event, tick: eventTick + offset, key: `${event.key || ""}@${cycle}` });
        }
      }
      timeline = expanded;
      terminalTick = startTick + cycles * span;
    }
    timeline.sort((a, b) => a.tick - b.tick || ({ controller: 0, noteOff: 1, noteOn: 2 }[a.type] - ({ controller: 0, noteOff: 1, noteOn: 2 }[b.type])));
    const states = new Map();
    const notes = new Map();
    const active = [];
    let peak = 0;
    const trackState = id => {
      if (!states.has(id)) states.set(id, { sustain: false, pending: [] });
      return states.get(id);
    };
    const expire = at => {
      for (let index = active.length - 1; index >= 0; index--) if (active[index].releaseAt <= at) active.splice(index, 1);
    };
    const releasePending = (state, at) => {
      for (const voice of state.pending) voice.releaseAt = at + voice.releaseSeconds;
      state.pending.length = 0;
    };
    for (const event of timeline) {
      const at = (event.tick - startTick) * secPerTick;
      expire(at);
      if (event.type === "controller") {
        if (event.controller.type !== "sustain") continue;
        const state = trackState(event.track.id);
        const on = event.controller.value >= 0.5;
        if (state.sustain && !on) releasePending(state, at);
        state.sustain = on;
      } else if (event.type === "noteOn") {
        const voice = { releaseAt: Infinity, releaseSeconds: voiceReleaseSeconds(event.track, event.note) };
        active.push(voice);
        notes.set(event.key, { voice, state: trackState(event.track.id) });
        peak = Math.max(peak, active.length);
        if (peak > MAX_ACTIVE_NOTES) return peak;
      } else {
        const note = notes.get(event.key);
        if (!note) continue;
        notes.delete(event.key);
        if (note.state.sustain) note.state.pending.push(note.voice);
        else note.voice.releaseAt = at + note.voice.releaseSeconds;
      }
    }
    if (secPerTick > 0) {
      const at = (terminalTick - startTick) * secPerTick;
      expire(at);
      for (const state of states.values()) releasePending(state, at);
      for (const voice of active) voice.releaseAt = Math.min(voice.releaseAt, at + voice.releaseSeconds);
    }
    return peak;
  }

  function create(options = {}) {
    // Browser rendering can temporarily starve the UI-thread scheduler. Keep
    // the editor's established reserve by default, while allowing consumers
    // such as Minigolf to keep more notes queued during a render hitch.
    const lookAhead = clamp(options?.lookAhead, 0.05, MAX_LOOKAHEAD, DEFAULT_LOOKAHEAD);
    let context = null;
    let mix = null;
    let buses = new Map();
    let currentInfo = null;
    let currentTickValue = 0;
    let playback = null;
    let resolvePlayback = null;
    let rejectPlayback = null;
    let playGeneration = 0;
    let retireBuses = false;
    let volume = 1, analyser=null, meterSamples=null, spectrumSamples=null;
    const manual=new Set();
    const voices = new Set();
    const liveStates = new Map();

    function ensureContext() {
      if (context) return context;
      const Constructor = audioContextConstructor();
      if (!Constructor) throw codedError("SYNTH_STUDIO_AUDIO_UNAVAILABLE", "Web Audio is unavailable in this browser.");
      context = new Constructor();
      mix = makeMix(context);
      // The app volume scales the master bus; the project master strip keeps
      // its own level inside the mix graph.
      mix.master.gain.value = mix.masterStrip ? volume : 0.78 * volume;
      if (mix.analyser) {
        analyser = mix.analyser;
        meterSamples = new Float32Array(analyser.fftSize);
        spectrumSamples = new Uint8Array(analyser.frequencyBinCount);
      } else if (context.createAnalyser) {
        analyser = context.createAnalyser();
        analyser.fftSize = 1024;
        meterSamples = new Float32Array(1024);
        spectrumSamples = new Uint8Array(analyser.frequencyBinCount);
        mix.master.connect(analyser);
      }
      return context;
    }

    // Push the project master strip and the tempo-synced send buses into the
    // audio graph.
    function applyMaster(info) {
      if (!mix || !context || !info || !info.master) return;
      const now = context.currentTime;
      const beat = 60 / clamp(info.tempo, 20, 300, 120);
      if (mix.masterStrip) mix.masterStrip.update(info.master, now, true);
      if (mix.sends) mix.sends.update(info.master, now, true, beat);
    }

    function setVolume(value) {
      volume = clamp(value, 0, 1, 1);
      if (mix && context) {
        const gain = mix.master.gain;
        const now = context.currentTime;
        gain.cancelScheduledValues(now);
        gain.setTargetAtTime(mix.masterStrip ? volume : 0.78 * volume, now, 0.02);
      }
      return volume;
    }

    function busFor(track) {
      let bus = buses.get(track.id);
      if (!bus) {
        bus = createTrackBus(ensureContext(), mix, track);
        buses.set(track.id, bus);
      }
      return bus;
    }

    function updateTracks(project, smooth = true) {
      const info = normalizeProject(project, true);
      applyMaster(info);
      const solo = info.tracks.some(t => t.solo);
      for (const track of info.tracks) {
        const current = currentInfo?.tracks.find(t => t.id === track.id);
        if (current) Object.assign(current, { volume: track.volume, pan: track.pan, mute: track.mute, solo: track.solo, params: track.params });
        const bus = buses.get(track.id);
        if (bus) updateTrackBus(bus, { ...track, mute: !isAudible(track, solo) }, undefined, smooth);
      }
      playback?.mix?.(info);
      playback?.restore?.();
    }

    function queueProject(project, startTick, onApplied) {
      if (!playback?.queue) return false;
      return playback.queue(normalizeProject(project), startTick, onApplied);
    }

    function pendingChange() { return playback?.pending || null; }

    function disconnectBuses() {
      for (const bus of buses.values()) {
        for (const insert of bus.inserts || []) {
          try { insert.processor && insert.processor.dispose && insert.processor.dispose(); } catch (_) {}
        }
        for (const node of Object.values(bus)) {
          if (node && typeof node.stop === "function") try { node.stop(); } catch (_) {}
          try { node.disconnect(); } catch (_) {}
        }
      }
      buses.clear();
      retireBuses = false;
    }

    function retireBusesIfIdle() {
      if (!retireBuses || !context) return;
      const now = context.currentTime;
      for (const voice of voices) if (voice.endTime > now) return;
      disconnectBuses();
    }

    function resolveTrack(trackValue) {
      if (trackValue && typeof trackValue === "object") {
        return normalizeProject({ tempo: 120, ppq: 480, tracks: [trackValue] }, true).tracks[0];
      }
      const id = String(trackValue || "");
      const existing = currentInfo?.tracks.find(track => track.id === id);
      return existing || {
        id: id || "audition",
        name: id || "Audition",
        instrument: "lead-saw",
        volume: 0.8,
        pan: 0,
        mute: false,
        solo: false,
        params: { ...DEFAULT_PARAMS },
        clips: []
      };
    }

    function voiceOn(track, pitch, velocity, at, state, liveInput = false) {
      if (!isAudible(track, Boolean(currentInfo?.tracks.some(item => item.solo)))) return null;
      let activeCount = 0;
      for (const voice of voices) if (voice.endTime > at) activeCount++;
      if (activeCount >= MAX_ACTIVE_NOTES) throw budgetError(`A maximum of ${MAX_ACTIVE_NOTES} voices can overlap.`);
      const operators = operatorsFor(track, pitch);
      const voice = createVoice(ensureContext(), mix, busFor(track), state, track, pitch, velocity, at, operators);
      voice.liveInput = liveInput;
      state.voices.add(voice);
      voices.add(voice);
      for (const source of voice.sources) {
        source.onended = () => {
          state.voices.delete(voice);
          voices.delete(voice);
          if (!voice.releaseRequested) releaseVoice(voice, context.currentTime);
          else disposeVoice(voice);
          retireBusesIfIdle();
        };
      }
      return voice;
    }

    function stateFor(track) {
      if (!liveStates.has(track.id)) liveStates.set(track.id, makeTrackState());
      return liveStates.get(track.id);
    }

    function clearVoices(at) {
      for (const voice of voices) {
        voice.releaseRequested = true;
        for (const source of voice.sources) {
          try {
            source.stop(at + 0.006);
          } catch (_) {}
        }
        if (voice.vibrato) try { voice.vibrato.oscillator.stop(at + 0.006); } catch (_) {}
        try { voice.voiceGain.gain.cancelScheduledValues(at); voice.voiceGain.gain.setValueAtTime(0, at); } catch (_) {}
      }
      voices.clear();
      if (playback) playback.states.clear();
    }

    function finishPlayback(tick, stopped, error) {
      if (!playback) return;
      const finished = playback;
      currentTickValue = tick;
      clearInterval(playback.timer);
      playback = null;
      if (stopped) {
        clearVoices(context?.currentTime || 0);
        finished.states.clear();
        disconnectBuses();
      } else {
        finished.states.clear();
        retireBuses = true;
        retireBusesIfIdle();
      }
      if (resolvePlayback || rejectPlayback) {
        const resolve = resolvePlayback;
        const reject = rejectPlayback;
        resolvePlayback = null;
        rejectPlayback = null;
        if (error && reject) reject(error);
        else if (resolve) resolve();
      }
    }

    async function unlock() {
      const ctx = ensureContext();
      if (ctx.state === "suspended") await ctx.resume();
      return true;
    }

    function play(project, startTick = 0, onTick, onEnd) {
      let info = normalizeProject(project);
      const requestGeneration = ++playGeneration;
      if (playback) finishPlayback(currentTick(), true);
      clearVoices(ensureContext().currentTime);
      disconnectBuses();
      currentInfo = info;
      applyMaster(info);
      let anySolo = info.tracks.some(track => track.solo);
      for (const track of info.tracks) busFor(track);
      return unlock().then(() => {
        if (requestGeneration !== playGeneration) return;
        return new Promise((resolve, reject) => {
        let all = allEvents(info);
        let secPerTick = 60 / info.tempo / info.ppq;
        let loop = info.loop.enabled;
        let loopStart = loop ? info.loop.start : 0;
        let loopEnd = loop ? info.loop.end : Math.max(info.endTick, 1);
        let tick = Math.max(0, Math.floor(clamp(startTick, 0, 1e9, 0)));
        if (loop) {
          if (tick < loopStart || tick >= loopEnd) tick = loopStart;
        }
        const liveRange = loop
          ? eventsForRange(all, loopStart, loopEnd, loopStart).map(event =>
            event.type === "noteOff" && event.tick > loopEnd ? { ...event, tick: loopEnd } : event)
          : eventsForRange(all, tick, Math.max(info.endTick, tick + 1), tick);
        let loopSeeds = loop ? loopControllerSeeds(all, info) : [];
        const preflightEvents = loop
          ? [...loopSeeds, ...liveRange]
          : [...seedState(all, info, tick), ...liveRange];
        const peak = activeVoiceLimit(
          preflightEvents,
          loop ? loopStart : tick,
          loop ? loopEnd : Math.max(info.endTick, tick + 1),
          secPerTick,
          loop ? { seeds: loopSeeds, maxReleaseSeconds: maxReleaseSeconds(info) } : null,
          loop ? loopEnd : Math.max(info.endTick, tick + 1)
        );
        if (peak > MAX_ACTIVE_NOTES) throw budgetError(`The arrangement has more than ${MAX_ACTIVE_NOTES} overlapping notes.`);
        resolvePlayback = resolve;
        rejectPlayback = reject;
        let startTime = context.currentTime + 0.045;
        const transportStart = startTime;
        let change = null, applied = null;
        const states = new Map(info.tracks.map(track => [track.id, makeTrackState()]));
        let tailSeconds = maxReleaseSeconds(info) + MAX_EFFECT_TAIL_SECONDS;
        const initial = seedState(all, info, tick);
        for (const event of initial) {
          if(event.type==='automation')updateTrackParameter(busFor(event.track),event.track,event.point.parameter,event.point.value,startTime);
          else applyController(context, states.get(event.track.id), null, event.track, event.controller.type, event.controller.value, startTime);
        }
        const scheduleCycle = (baseTime, cycleStart, firstCycle, cycleIndex) => {
          const anchor = firstCycle ? cycleStart : loopStart;
          const cycleEvents = all.filter(event => {
            if (event.type === "noteOff") return event.ownerTick >= cycleStart && event.ownerTick < loopEnd;
            if (firstCycle) return event.tick >= cycleStart && event.tick < loopEnd;
            return event.tick >= loopStart && event.tick < loopEnd;
          }).map(event => {
            const eventTick = event.type === "noteOff" && event.tick > loopEnd ? loopEnd : event.tick;
            const key = event.type === "noteOn" || event.type === "noteOff" ? `${event.key}@${cycleIndex}` : event.key;
            return { ...event, key, at: baseTime + (eventTick - anchor) * secPerTick };
          });
          if (firstCycle) return cycleEvents;
          return [{type:'rangeEnd',at:baseTime}, ...loopSeeds.map(event => ({ ...event, at: baseTime })), ...cycleEvents];
        };
        let pending = [];
        let nextCycleTime = startTime;
        let endTime = startTime + Math.max(0, info.endTick - tick) * secPerTick;
        let cycleIndex = 1;
        if (loop) {
          const firstDuration = loopEnd - tick;
          pending = scheduleCycle(startTime, tick, true, 0);
          nextCycleTime = startTime + firstDuration * secPerTick;
        } else {
          pending = all.filter(event => event.tick >= tick).map(event => ({ ...event, at: startTime + (event.tick - tick) * secPerTick }));
          pending.push({ type: "end", tick: info.endTick, at: endTime });
        }
        const priority = { rangeEnd:-1, automation:0, metronome: 0, controller: 1, noteOff: 2, noteOn: 3, end: 4 };
        let automatic=[],stopAt=Infinity,stopScheduled=false;
        pending.sort((a, b) => a.at - b.at || priority[a.type] - priority[b.type]);
        for (const track of info.tracks) if (isAudible(track, anySolo)) busFor(track);
        const stateKeys = new Map();
        const enqueue = (newEvents) => {
          pending.push(...newEvents);
          pending.sort((a, b) => a.at - b.at || priority[a.type] - priority[b.type]);
        };
        const process = event => {
          if(event.type==='automation'){
            automatic.push(event);
            if(!manual.has(event.track.id+':'+event.point.parameter))updateTrackParameter(busFor(event.track),event.track,event.point.parameter,event.point.value,event.at);
            return;
          }
          if (event.type === "metronome") {
            scheduleMetronome(context, mix, event.at, event.accent);
            return;
          }
          if (event.type === "end" || event.type === "rangeEnd") {
            for (const track of info.tracks) releaseTrackState(context, states.get(track.id), track, event.at);
            if(event.type==="rangeEnd")stateKeys.clear();
            return;
          }
          const state = states.get(event.track.id);
          if (event.type === "controller") {
            applyController(context, state, null, event.track, event.controller.type, event.controller.value, event.at);
            return;
          }
          if (event.type === "noteOn") {
            const voice = voiceOn(event.track, event.note.pitch, event.note.velocity, event.at, state);
            if (voice) stateKeys.set(event.key, { voice, track: event.track, state });
            return;
          }
          const record = stateKeys.get(event.key);
          if (!record) return;
          stateKeys.delete(event.key);
          if (record.state.sustain) record.state.pending.push(record.voice);
          else releaseVoice(record.voice, event.at);
        };
        const renderTick = () => {
          try {
            const now = context.currentTime;
            automatic=automatic.filter(e=>e.at>=now);
            if (change && change.at < stopAt && change.at <= now + lookAhead) {
              // Future voices are cancelled at the bar; held voices release into the existing effect buses.
              while (pending.length && pending[0].at < change.at) process(pending.shift());
              for (const track of info.tracks) releaseTrackState(context, states.get(track.id), track, change.at);
              const previous = playback.previous && now < playback.startTime ? playback.previous : { ...playback, previous: null };
              info = change.info; currentInfo = info; all = allEvents(info);
              anySolo = info.tracks.some(t => t.solo); secPerTick = 60 / info.tempo / info.ppq;
              loop = info.loop.enabled; loopStart = loop ? info.loop.start : 0; loopEnd = loop ? info.loop.end : Math.max(info.endTick, 1);
              tick = Math.max(0, change.tick); if (loop && (tick < loopStart || tick >= loopEnd)) tick = loopStart;
              startTime = change.at; endTime = startTime + Math.max(0, info.endTick - tick) * secPerTick;
              tailSeconds = maxReleaseSeconds(info) + MAX_EFFECT_TAIL_SECONDS;
              loopSeeds = loop ? loopControllerSeeds(all, info) : [];
              states.clear(); stateKeys.clear(); automatic=[];
              for (const track of info.tracks) {
                states.set(track.id, makeTrackState());
                updateTrackBus(busFor(track), { ...track, mute: !isAudible(track, anySolo) }, startTime, true);
              }
              pending = [...seedState(all, info, tick).map(e => ({ ...e, at: startTime })), ...scheduleCycle(startTime, tick, true, cycleIndex++)];
              if (!loop) pending.push({ type: 'end', tick: info.endTick, at: endTime });
              pending.sort((a,b) => a.at-b.at || priority[a.type]-priority[b.type]);
              nextCycleTime = startTime + (loopEnd - tick) * secPerTick;
              Object.assign(playback, { previous, startTime, startTick: tick, loop, loopStart, loopEnd, secPerTick, endTime, tailSeconds });
              applied = { ...change, tick }; change = null;
            }
            if (loop) {
              while (nextCycleTime <= Math.min(now + lookAhead, stopAt - 0.000001, change ? change.at - 0.000001 : Infinity)) {
                const baseTime = nextCycleTime;
                enqueue(scheduleCycle(baseTime, loopStart, false, cycleIndex++));
                nextCycleTime += (loopEnd - loopStart) * secPerTick;
              }
            }
            const horizon = Math.min(now + lookAhead, stopAt - 0.000001, change ? change.at - 0.000001 : Infinity);
            while (pending.length && pending[0].at <= horizon) process(pending.shift());
            if(!stopScheduled&&stopAt<=now+lookAhead){process({type:'end',at:stopAt});stopScheduled=true;}
            const shownTick = currentTick();
            currentTickValue = shownTick;
            if (applied && now >= applied.at) { const done = applied; applied = null; playback.pending = null; playback.previous = null; done.onApplied?.(done.tick,{elapsedSeconds:done.at-transportStart,at:done.at}); }
            if (typeof onTick === "function") onTick(shownTick, { elapsedSeconds: Math.max(0, now - transportStart), pending: playback?.pending });
            if (now >= stopAt || (!loop && !change && now >= endTime + tailSeconds)) {
              finishPlayback(now>=stopAt?shownTick:info.endTick, false);
              if (typeof onEnd === "function") {
                try { onEnd(); } catch (_) {}
              }
            }
          } catch (error) {
            finishPlayback(currentTickValue, true, error);
          }
        };
        playback = { timer: setInterval(renderTick, TICK_MS), states, transportStart, startTime, startTick: tick, loop, loopStart, loopEnd, secPerTick, endTime, tailSeconds };
        playback.stopAfter=seconds=>{stopAt=transportStart+Math.max(0,seconds);stopScheduled=false;};
        playback.mix=next=>{for(const t of next.tracks){const pendingTrack=change?.info.tracks.find(v=>v.id===t.id);if(pendingTrack)Object.assign(pendingTrack,{volume:t.volume,pan:t.pan,mute:t.mute,solo:t.solo,params:t.params});}};
        playback.automation=next=>{
          for(const t of next.tracks){const current=info.tracks.find(v=>v.id===t.id);if(current)current.automation=t.automation;const queued=change?.info.tracks.find(v=>v.id===t.id);if(queued)queued.automation=t.automation;}
          all=allEvents(info);loopSeeds=loop?loopControllerSeeds(all,info):[];
          pending=pending.filter(e=>e.type!=='automation');automatic=[];
          const now=context.currentTime;
          let base=startTime,anchor=tick,end=loopEnd;
          if(loop&&base+(end-anchor)*secPerTick<now){
            base+=(end-anchor)*secPerTick;anchor=loopStart;
            base+=Math.floor(Math.max(0,now-base)/((loopEnd-loopStart)*secPerTick))*((loopEnd-loopStart)*secPerTick);
          }
          do{
            enqueue(all.filter(e=>e.type==='automation'&&e.tick>=anchor&&(!loop||e.tick<end)).map(e=>({...e,at:base+(e.tick-anchor)*secPerTick})).filter(e=>e.at>now));
            if(!loop)break;base+=(end-anchor)*secPerTick;anchor=loopStart;
          }while(base<nextCycleTime-.000001);
          playback.restore();renderTick();
        };
        playback.restore=()=>{
          const now=context.currentTime,atTick=currentTick();
          for(const track of info.tracks)for(const parameter of ['tone','reverb']){
            if(manual.has(track.id+':'+parameter)||!track.automation.some(p=>p.parameter===parameter))continue;
            let value=track.params[parameter];
            for(const point of track.automation)if(point.parameter===parameter&&point.tick<=atTick)value=point.value;
            updateTrackParameter(busFor(track),track,parameter,value,now,true);
            for(const event of automatic)if(event.track.id===track.id&&event.point.parameter===parameter&&event.at>now)updateTrackParameter(busFor(track),track,parameter,event.point.value,event.at);
          }
        };
        playback.queue = (next, target, callback) => {
          if((change&&change.at<=context.currentTime)||(applied&&applied.at<=context.currentTime))renderTick();
          if(!playback)return null;
          const events = allEvents(next), end = next.loop.enabled ? next.loop.end : Math.max(next.endTick, 1);
          const start=next.loop.enabled?next.loop.start:Math.max(0,target??0),seconds=60/next.tempo/next.ppq;
          const seeds=next.loop.enabled?loopControllerSeeds(events,next):seedState(events,next,start);
          const range=eventsForRange(events,start,end,start).map(e=>next.loop.enabled&&e.type==='noteOff'&&e.tick>end?{...e,tick:end}:e);
          if(activeVoiceLimit([...seeds,...range],start,end,seconds,next.loop.enabled?{seeds,maxReleaseSeconds:maxReleaseSeconds(next)}:null,end)>MAX_ACTIVE_NOTES)throw budgetError('Too many overlapping voices.');
          const safeTime = context.currentTime + 0.001;
          const raw = tick + Math.max(0, safeTime - startTime) / secPerTick;
          const bar = info.ppq * info.beatsPerBar;
          const boundary = Math.ceil(raw / bar) * bar;
          const at = change?.at || (applied&&context.currentTime<applied.at?applied.at:startTime + (boundary - tick) * secPerTick);
          for(const voice of voices)if(voice.startTime>=at){voice.voiceGain.gain.cancelScheduledValues(at);voice.voiceGain.gain.setValueAtTime(0,at);voice.endTime=at;voice.releaseRequested=true;voice.releaseAt=at;for(const source of voice.sources)try{source.stop(at);}catch(_){}}
          if(applied&&at===applied.at)applied=null;
          const mapped = loop && boundary >= loopEnd ? loopStart + ((boundary - loopStart) % (loopEnd - loopStart)) : boundary;
          change = { info: next, tick: target ?? mapped, at, onApplied: callback };
          const request={ tick: change.tick, at, elapsedSeconds:at-transportStart };playback.pending=request;
          renderTick();
          return request;
        };
        currentTickValue = tick;
        renderTick();
        });
      });
    }

    function stop() {
      manual.clear();playGeneration++;
      const tick = currentTick();
      if (playback) finishPlayback(tick, true);
      else {
        clearVoices(context?.currentTime || 0);
        disconnectBuses();
      }
      return tick;
    }

    function currentTick() {
      if (!playback || !context) return currentTickValue;
      const clock = playback.previous && context.currentTime < playback.startTime ? playback.previous : playback;
      const elapsed = Math.max(0, context.currentTime - clock.startTime) / clock.secPerTick;
      const raw = clock.startTick + elapsed;
      if (!clock.loop || raw < clock.loopEnd) return raw;
      return clock.loopStart + ((raw - clock.loopStart) % (clock.loopEnd - clock.loopStart));
    }

    function noteOn(trackValue, pitchValue, velocityValue) {
      const track = resolveTrack(trackValue);
      const pitch = Math.round(clamp(pitchValue, 0, 127, 60));
      const velocity = clamp(velocityValue, 0, 1, 0.8);
      if (!context) ensureContext();
      if (!buses.has(track.id)) busFor(track);
      const state = stateFor(track);
      const voice = voiceOn(track, pitch, velocity, context.currentTime + 0.004, state, true);
      let released = false;
      return function release() {
        if (released || !voice) return;
        released = true;
        const at = context.currentTime + 0.005;
        if (state.sustain) state.pending.push(voice);
        else releaseVoice(voice, at);
      };
    }

    function scheduleNote(trackValue,pitch,velocity,at,duration) {
      const track=resolveTrack(trackValue),state=stateFor(track);
      const voice=voiceOn(track,pitch,velocity,Math.max(context.currentTime,at),state,true);
      if(voice)releaseVoice(voice,Math.max(context.currentTime,at)+duration);
      return () => { if(!voice)return;const now=context.currentTime;voice.voiceGain.gain.cancelScheduledValues(now);voice.voiceGain.gain.setValueAtTime(0,now);for(const source of voice.sources)try{source.stop(now+.006);}catch(_){} };
    }

    function addController(trackValue, type, value) {
      if (!["sustain", "bend", "modulation"].includes(type)) return false;
      const track = resolveTrack(trackValue);
      const state = stateFor(track);
      if (!context) ensureContext();
      const normalized = type === "bend" ? clamp(value, -1, 1, 0) : clamp(value, 0, 1, 0);
      applyController(context, state, null, track, type, normalized, context.currentTime + 0.004);
      return true;
    }

    function panic(trackValue) {
      const targetId = trackValue && typeof trackValue === "object"
        ? String(trackValue.id || "")
        : (trackValue == null ? null : String(trackValue));
      const at = context ? context.currentTime + 0.004 : 0;
      for (const [id, state] of liveStates) {
        if (targetId !== null && id !== targetId) continue;
        for (const voice of state.voices) {
          if (voice.liveInput) releaseVoice(voice, at, 0.06);
        }
        state.pending = state.pending.filter(voice => {
          if (voice.liveInput) {
            releaseVoice(voice, at, 0.06);
            return false;
          }
          return true;
        });
        state.sustain = false;
        state.bend = 0;
        state.mod = 0;
        if (context) {
          const track = resolveTrack(id);
          applyController(context, state, null, track, "bend", 0, at);
          applyController(context, state, null, track, "modulation", 0, at);
        }
      }
      return true;
    }

    function dispose() {
      stop();
      if (context) {
        const ctx = context;
        context = null;
        mix = null;
        try { ctx.close(); } catch (_) {}
      }
    }

    function setAutomation(project){const next=normalizeProject(project);playback?.automation?.(next);}
    function setManual(id,parameter,active){const key=id+':'+parameter;if(active)manual.add(key);else{manual.delete(key);playback?.restore?.();}}
    function meter(){if(!analyser)return 0;analyser.getFloatTimeDomainData(meterSamples);return meterSamples.reduce((peak,v)=>Math.max(peak,Math.abs(v)),0);}
    // The master spectrum for the workbench analyser. Feedback only: it never
    // influences the audio path.
    function spectrum(bins = 64){if(!analyser||!spectrumSamples)return null;analyser.getByteFrequencyData(spectrumSamples);const out=new Array(bins).fill(0);const step=spectrumSamples.length/bins;for(let i=0;i<bins;i++){let sum=0;const from=Math.floor(i*step);const to=Math.max(from+1,Math.floor((i+1)*step));for(let j=from;j<to&&j<spectrumSamples.length;j++)sum+=spectrumSamples[j];out[i]=sum/(to-from);}return out;}

    return { unlock, play, stop, currentTick, meter, spectrum, setManual, setAutomation, running:()=>!!playback, elapsed:()=>playback?Math.max(0,context.currentTime-playback.transportStart):0, setStopAfter:seconds=>playback?.stopAfter(seconds), clock:()=>ensureContext().currentTime, scheduleNote, noteOn, addController, panic, setVolume, updateTracks, queueProject, pendingChange, dispose };
  }

  function checkRenderBudget(info, events, startTick, endTick, durationSeconds, secPerTick, seedEvents = events) {
    if (info.noteCount > MAX_NOTES) throw budgetError(`A project can contain at most ${MAX_NOTES} notes.`);
    const seeds = seedState(seedEvents, info, startTick);
    const peak = activeVoiceLimit([...seeds, ...events], startTick, endTick, secPerTick, null, endTick);
    if (peak > MAX_ACTIVE_NOTES) throw budgetError(`The arrangement has more than ${MAX_ACTIVE_NOTES} overlapping notes.`);
    if (!Number.isFinite(durationSeconds) || durationSeconds > MAX_RENDER_SECONDS) {
      throw exportLimitError(`WAV rendering is limited to ${MAX_RENDER_SECONDS} seconds per export, including effect tail.`);
    }
  }

  function encodeWav(audioBuffer) {
    const channels = 2;
    const frames = audioBuffer.length;
    const bytes = new ArrayBuffer(44 + frames * channels * 2);
    const view = new DataView(bytes);
    const writeText = (offset, value) => {
      for (let i = 0; i < value.length; i++) view.setUint8(offset + i, value.charCodeAt(i));
    };
    writeText(0, "RIFF");
    view.setUint32(4, 36 + frames * channels * 2, true);
    writeText(8, "WAVE");
    writeText(12, "fmt ");
    view.setUint32(16, 16, true);
    view.setUint16(20, 1, true);
    view.setUint16(22, channels, true);
    view.setUint32(24, audioBuffer.sampleRate, true);
    view.setUint32(28, audioBuffer.sampleRate * channels * 2, true);
    view.setUint16(32, channels * 2, true);
    view.setUint16(34, 16, true);
    writeText(36, "data");
    view.setUint32(40, frames * channels * 2, true);
    const left = audioBuffer.getChannelData(0);
    const right = audioBuffer.getChannelData(1);
    let offset = 44;
    for (let i = 0; i < frames; i++) {
      for (const sample of [left[i], right[i]]) {
        const bounded = Math.max(-1, Math.min(1, sample || 0));
        view.setInt16(offset, bounded < 0 ? bounded * 32768 : bounded * 32767, true);
        offset += 2;
      }
    }
    return new Blob([bytes], { type: "audio/wav" });
  }

  function prepareRender(project) {
    const info=normalizeProject(project),events=allEvents(info),looping=info.loop.enabled;
    const startTick=looping?info.loop.start:0,endTick=looping?info.loop.end:Math.max(info.endTick,1);
    const selected=eventsForRange(events,startTick,endTick,startTick).map(e=>looping&&e.type==='noteOff'&&e.tick>endTick?{...e,tick:endTick}:e);
    const secPerTick=60/info.tempo/info.ppq;
    let audibleEndTick=endTick;
    for(const event of selected)if(event.type==='noteOff')audibleEndTick=Math.max(audibleEndTick,event.tick);
    const duration=Math.max(.05,(audibleEndTick-startTick)*secPerTick)+maxReleaseSeconds(info)+MAX_EFFECT_TAIL_SECONDS;
    checkRenderBudget(info,selected,startTick,endTick,duration,secPerTick,events);
    return {info,events,selected,startTick,endTick,audibleEndTick,duration,secPerTick};
  }

  async function renderWav(project) {
    const Constructor = offlineContextConstructor();
    if (!Constructor) throw codedError("SYNTH_STUDIO_AUDIO_UNAVAILABLE", "Offline Web Audio is unavailable in this browser.");
    const {info,events,selected,startTick,endTick,audibleEndTick,duration,secPerTick}=prepareRender(project);
    const rate=44100;
    const frames = Math.ceil(duration * rate);
    const context = new Constructor(2, frames, rate);
    const mix = makeMix(context, { master: info.master, secondsPerBeat: 60 / info.tempo });
    if (mix.masterStrip) mix.masterStrip.update(info.master, 0, false);
    if (mix.sends) mix.sends.update(info.master, 0, false, 60 / info.tempo);
    const anySolo = info.tracks.some(track => track.solo);
    const buses = new Map();
    const states = new Map();
    for (const track of info.tracks) {
      buses.set(track.id, createTrackBus(context, mix, track));
      states.set(track.id, makeTrackState());
    }
    const initial = seedState(events, info, startTick);
    const schedule = [
      ...initial.map(event => ({ ...event, tick: startTick })),
      ...selected,
      { type: "end", tick: audibleEndTick }
    ].sort((a, b) => a.tick - b.tick || ({ automation:0, metronome: 0, controller: 1, noteOff: 2, noteOn: 3, end: 4 }[a.type] - ({ automation:0, metronome: 0, controller: 1, noteOff: 2, noteOn: 3, end: 4 }[b.type])));
    const active = new Map();
    for (const event of schedule) {
      const at = Math.max(0, (event.tick - startTick) * secPerTick);
      if(event.type==='automation'){updateTrackParameter(buses.get(event.track.id),event.track,event.point.parameter,event.point.value,at);continue;}
      if (event.type === "metronome") {
        scheduleMetronome(context, mix, at, event.accent);
        continue;
      }
      if (event.type === "end") {
        for (const track of info.tracks) releaseTrackState(context, states.get(track.id), track, at);
        continue;
      }
      const state = states.get(event.track.id);
      if (event.type === "controller") {
        applyController(context, state, null, event.track, event.controller.type, event.controller.value, at);
      } else if (event.type === "noteOn") {
        if (!isAudible(event.track, anySolo)) continue;
        const voice = createVoice(context, mix, buses.get(event.track.id), state, event.track, event.note.pitch, event.note.velocity, at, operatorsFor(event.track, event.note.pitch));
        state.voices.add(voice);
        active.set(event.key, { voice, state, track: event.track });
      } else {
        const record = active.get(event.key);
        if (!record) continue;
        active.delete(event.key);
        if (state.sustain) state.pending.push(record.voice);
        else releaseVoice(record.voice, at);
      }
    }
    const rendered = await context.startRendering();
    return encodeWav(rendered);
  }

  root.SynthStudioAudio = { create, renderWav, prepareRender };
})(typeof window !== "undefined" ? window : globalThis);
