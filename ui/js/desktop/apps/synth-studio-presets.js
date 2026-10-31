(function (root) {
  "use strict";

  const categories = [
    { id: "drums", name: "Drums" },
    { id: "bass", name: "Bass" },
    { id: "leads", name: "Leads" },
    { id: "pads", name: "Pads" },
    { id: "keys", name: "Keys" }
  ];
  const groups = [
    { category: "drums", prefix: "drum", items: [
      ["sub-kick", "Sub Kick", 35, ["dark", "warm"]], ["punch-kick", "Punch Kick", 36, ["modern", "dark"]], ["snare", "Snare", 38, ["bright", "retro"]],
      ["clap", "Clap", 39, ["bright", "warm"]], ["closed-hi-hat", "Closed Hi-Hat", 42, ["bright", "pluck"]], ["open-hi-hat", "Open Hi-Hat", 46, ["bright", "warm"]],
      ["low-tom", "Low Tom", 45, ["dark", "warm"]], ["rimshot", "Rimshot", 37, ["pluck", "retro"]], ["shaker", "Shaker", 70, ["pluck", "modern"]], ["crash", "Crash", 49, ["bright", "warm"]],
      ["909-snare", "909 Snare", 40, ["retro", "dark"]], ["mid-tom", "Mid Tom", 47, ["warm", "retro"]], ["high-tom", "High Tom", 50, ["bright", "warm"]],
      ["ride", "Ride Cymbal", 51, ["bright", "warm"]], ["cowbell", "808 Cowbell", 56, ["retro", "dark"]], ["conga", "Electro Conga", 63, ["warm", "pluck"]],
      ["floor-tom", "Floor Tom", 41, ["warm", "dark"]],
      ["tambora", "Tambora", 85, ["retro", "dark"]],
      ["ride-bell", "Ride Bell", 53, ["bright", "warm"]],
      ["chinese-cymbal", "Chinese Cymbal", 52, ["bright", "retro"]],
      ["tambourine", "Tambourine", 54, ["pluck", "bright"]],
      ["high-bongo", "High Bongo", 58, ["warm", "pluck"]],
      ["low-bongo", "Low Bongo", 57, ["dark", "warm"]],
      ["wood-block", "Wood Block", 77, ["pluck", "retro"]]
    ] },
    { category: "bass", prefix: "bass", items: [
      ["sub", "Sub", { wave: "sine", sub: 0.24, subWave: "sine", subOctave: 1, level: 0.34, attack: 0.008, decay: 0.25, sustain: 0.78, release: 0.12, cutoff: 0.42, filterEnv: 0.18, fx: { drive: 0.15, tone: 0.42 } }, ["dark", "warm"]],
      ["round", "Round", { wave: "triangle", sub: 0.14, mod: "sine", ratio: 2, depth: 0.32, level: 0.25, attack: 0.012, decay: 0.2, sustain: 0.72, release: 0.16, cutoff: 0.52, filterEnv: 0.22, fx: { chorus: 0.12, tone: 0.52 } }, ["warm", "modern"]],
      ["acid", "Acid", { wave: "sawtooth", mod: "square", ratio: 2, depth: 1.7, level: 0.24, attack: 0.004, decay: 0.14, sustain: 0.42, release: 0.08, cutoff: 0.38, filterEnv: 0.75, filterDecay: 0.16, filterQ: 6.5, fx: { resonance: 0.58, drive: 0.32, delay: 0.14 } }, ["acid", "retro"]],
      ["saw", "Saw", { wave: "sawtooth", sub: 0.15, level: 0.2, second: 0.16, detune: 11, attack: 0.006, decay: 0.24, sustain: 0.7, release: 0.12, cutoff: 0.62, filterEnv: 0.35, fx: { drive: 0.18 } }, ["bright", "modern"]],
      ["square", "Square", { wave: "pulse25", sub: 0.14, level: 0.22, attack: 0.006, decay: 0.22, sustain: 0.7, release: 0.12, cutoff: 0.58, filterEnv: 0.3, fx: { chorus: 0.15 } }, ["retro", "bright"]],
      ["fm", "FM", { wave: "sine", mod: "sine", ratio: 3.3, depth: 3.2, mod2: "sine", ratio2: 1, depth2: 1.4, level: 0.24, attack: 0.004, decay: 0.18, sustain: 0.62, release: 0.1, cutoff: 0.8, fx: { drive: 0.14, chorus: 0.1 } }, ["modern", "bright"]],
      ["pluck", "Pluck", { wave: "triangle", mod: "sine", ratio: 5, depth: 0.9, level: 0.25, attack: 0.002, decay: 0.13, sustain: 0.12, release: 0.08, cutoff: 0.62, filterEnv: 0.55, filterDecay: 0.14, fx: { delay: 0.18, reverb: 0.1 } }, ["pluck", "bright"]],
      ["reese", "Reese", { wave: "sawtooth", unison: 3, sub: 0.18, level: 0.18, second: 0.17, detune: 19, attack: 0.015, decay: 0.32, sustain: 0.76, release: 0.22, cutoff: 0.52, filterEnv: 0.32, fx: { chorus: 0.35, drive: 0.28, phaser: 0.16 } }, ["dark", "modern"]],
      ["wobble", "Wobble", { wave: "square", mod: "triangle", ratio: 0.5, depth: 1.6, sub: 0.16, level: 0.2, attack: 0.01, decay: 0.18, sustain: 0.72, release: 0.15, cutoff: 0.44, filterEnv: 0.6, filterQ: 4.2, fx: { resonance: 0.42, phaser: 0.38, drive: 0.22 } }, ["acid", "dark"]],
      ["rubber", "Rubber", { wave: "triangle", mod: "square", ratio: 4, depth: 2.2, pitchEnv: 1.35, level: 0.22, attack: 0.002, decay: 0.11, sustain: 0.38, release: 0.1, cutoff: 0.68, filterEnv: 0.48, fx: { resonance: 0.28, drive: 0.15 } }, ["pluck", "retro"]],
      ["303", "Acid 303", { wave: "sawtooth", mod: "pulse25", ratio: 1, depth: 0.85, level: 0.25, attack: 0.003, decay: 0.16, sustain: 0.18, release: 0.07, cutoff: 0.28, filterEnv: 0.88, filterDecay: 0.15, filterQ: 9.2, glide: 1.06, fx: { resonance: 0.72, drive: 0.45, delay: 0.22 } }, ["acid", "retro"]],
      ["808-glide", "808 Glide", { wave: "sine", sub: 0.26, subWave: "sine", mod: "triangle", ratio: 2, depth: 0.45, pitchEnv: 1.6, level: 0.32, attack: 0.004, decay: 0.55, sustain: 0.58, release: 0.28, cutoff: 0.46, glide: 1.12, fx: { drive: 0.36, tone: 0.44 } }, ["retro", "dark"]],
      ["outrun", "Outrun Pulse", { wave: "pulse25", sub: 0.2, subWave: "sawtooth", second: 0.14, detune: 9, level: 0.22, attack: 0.004, decay: 0.18, sustain: 0.35, release: 0.11, cutoff: 0.56, filterEnv: 0.62, filterDecay: 0.16, fx: { chorus: 0.28, drive: 0.2, reverb: 0.12 } }, ["modern", "bright"]],
      ["slap", "FM Slap Bass", { wave: "sine", mod: "sine", ratio: 2, depth: 3.6, mod2: "sine", ratio2: 7, depth2: 2.1, decay2: 0.08, level: 0.24, attack: 0.002, decay: 0.22, sustain: 0.28, release: 0.12, cutoff: 0.84, filterEnv: 0.45, fx: { chorus: 0.18, drive: 0.16 } }, ["pluck", "modern"]],
      ["neuro", "Neuro Unison", { wave: "supersaw", unison: 5, sub: 0.22, detune: 24, mod: "sawtooth", ratio: 1.5, depth: 1.2, level: 0.18, attack: 0.012, decay: 0.28, sustain: 0.74, release: 0.2, cutoff: 0.48, filterEnv: 0.58, filterQ: 4.8, fx: { drive: 0.48, phaser: 0.32, chorus: 0.26 } }, ["dark", "modern"]],
      ["fretless", "Chorus Fretless", { wave: "triangle", second: 0.14, detune: 6, mod: "sine", ratio: 3, depth: 0.65, level: 0.24, attack: 0.018, decay: 0.32, sustain: 0.68, release: 0.24, cutoff: 0.64, filterEnv: 0.28, glide: 1.05, fx: { chorus: 0.46, reverb: 0.22 } }, ["warm", "modern"]],
      ["acid-808", "Acid 808", { wave: "sine", sub: 0.22, subWave: "sine", mod: "square", ratio: 2, depth: 1.2, pitchEnv: 1.4, level: 0.3, attack: 0.004, decay: 0.2, sustain: 0.4, release: 0.1, cutoff: 0.34, filterEnv: 0.8, filterDecay: 0.18, filterQ: 8.4, fx: { resonance: 0.62, drive: 0.4, delay: 0.18 } }, ["acid", "retro"]],
      ["growl", "Growl Bass", { wave: "sawtooth", unison: 3, detune: 14, mod: "sawtooth", ratio: 1.5, depth: 1.6, level: 0.2, attack: 0.006, decay: 0.24, sustain: 0.66, release: 0.14, cutoff: 0.4, filterEnv: 0.5, filterQ: 5.4, fx: { resonance: 0.5, drive: 0.55 } }, ["dark", "modern"]],
      ["deep-round", "Deep Round", { wave: "triangle", sub: 0.2, subWave: "sine", mod: "sine", ratio: 2, depth: 0.4, level: 0.26, attack: 0.014, decay: 0.26, sustain: 0.76, release: 0.18, cutoff: 0.4, filterEnv: 0.16, fx: { tone: 0.4 } }, ["dark", "warm"]],
      ["metal", "Metal Bass", { wave: "sawtooth", mod: "square", ratio: 5, depth: 2.4, mod2: "square", ratio2: 7, depth2: 1.4, noise: 0.05, noiseWave: "n2", level: 0.2, attack: 0.003, decay: 0.2, sustain: 0.5, release: 0.12, cutoff: 0.5, filterEnv: 0.62, filterDecay: 0.16, filterQ: 6.2, fx: { resonance: 0.46, drive: 0.42 } }, ["dark", "bright"]],
      ["dub-sub", "Dub Sub", { wave: "sine", sub: 0.28, subWave: "sine", subOctave: 1, level: 0.32, attack: 0.006, decay: 0.42, sustain: 0.66, release: 0.26, cutoff: 0.32, filterEnv: 0.1, fx: { tone: 0.36, delay: 0.28, reverb: 0.16 } }, ["dark", "warm"]],
      ["picked", "Picked Bass", { wave: "triangle", mod: "sine", ratio: 3, depth: 0.8, level: 0.26, attack: 0.002, decay: 0.12, sustain: 0.16, release: 0.1, cutoff: 0.66, filterEnv: 0.62, filterDecay: 0.12, filterQ: 3.6, fx: { delay: 0.16, tone: 0.55 } }, ["pluck", "bright"]],
      ["five-string", "Five String", { wave: "sawtooth", sub: 0.2, subWave: "triangle", second: 0.12, detune: 7, level: 0.22, attack: 0.008, decay: 0.24, sustain: 0.74, release: 0.16, cutoff: 0.44, filterEnv: 0.24, fx: { drive: 0.24, tone: 0.46 } }, ["warm", "dark"]],
      ["drone", "Bass Drone", { wave: "sawtooth", unison: 3, detune: 11, sub: 0.24, level: 0.17, attack: 0.4, decay: 0.6, sustain: 0.82, release: 0.9, cutoff: 0.3, filterEnv: 0.08, fx: { tone: 0.34, reverb: 0.24 } }, ["dark", "warm"]]
    ] },
    { category: "leads", prefix: "lead", items: [
      ["saw", "Saw", { wave: "sawtooth", unison: 3, level: 0.18, second: 0.12, detune: 8, attack: 0.008, decay: 0.2, sustain: 0.74, release: 0.12, cutoff: 0.72, filterEnv: 0.3, fx: { delay: 0.18, reverb: 0.16 } }, ["warm", "modern"]],
      ["square", "Square", { wave: "square", sub: 0.08, level: 0.22, attack: 0.004, decay: 0.18, sustain: 0.7, release: 0.14, cutoff: 0.68, filterEnv: 0.25, fx: { delay: 0.16 } }, ["retro", "bright"]],
      ["pulse", "Pulse", { wave: "pulse25", mod: "sine", ratio: 2, depth: 0.5, level: 0.21, attack: 0.006, decay: 0.18, sustain: 0.65, release: 0.12, cutoff: 0.52, filterEnv: 0.42, fx: { chorus: 0.22, delay: 0.15 } }, ["retro", "bright"]],
      ["triangle", "Triangle", { wave: "triangle", second: 0.09, detune: 5, level: 0.26, attack: 0.01, decay: 0.2, sustain: 0.7, release: 0.16, cutoff: 0.78, fx: { reverb: 0.2, delay: 0.14 } }, ["warm", "retro"]],
      ["sine", "Sine", { wave: "sine", sub: 0.08, level: 0.28, attack: 0.015, decay: 0.22, sustain: 0.72, release: 0.18, cutoff: 0.9, fx: { reverb: 0.22, delay: 0.2 } }, ["warm", "air"]],
      ["fm", "FM", { wave: "sine", mod: "sine", ratio: 2.7, depth: 4.2, mod2: "sine", ratio2: 5.4, depth2: 1.3, level: 0.2, attack: 0.004, decay: 0.18, sustain: 0.66, release: 0.12, cutoff: 0.82, fx: { chorus: 0.2, delay: 0.22 } }, ["bright", "modern"]],
      ["brass", "Brass", { wave: "sawtooth", unison: 3, detune: 10, mod: "square", ratio: 2, depth: 0.7, level: 0.2, attack: 0.035, decay: 0.22, sustain: 0.68, release: 0.18, cutoff: 0.54, filterEnv: 0.52, filterDecay: 0.24, fx: { chorus: 0.25, reverb: 0.24 } }, ["warm", "retro"]],
      ["chiptune", "Chiptune", { wave: "pulse12", level: 0.28, attack: 0.002, decay: 0.08, sustain: 0.82, release: 0.04, cutoff: 0.92, fx: { delay: 0.12 } }, ["retro", "bright"]],
      ["laser", "Laser", { wave: "sawtooth", mod: "sine", ratio: 5, depth: 2.2, level: 0.22, attack: 0.001, decay: 0.22, sustain: 0.08, release: 0.08, cutoff: 0.86, glide: 1.8, filterEnv: 0.6, fx: { delay: 0.28, reverb: 0.18 } }, ["bright", "modern"]],
      ["glide", "Glide", { wave: "sawtooth", unison: 3, level: 0.18, second: 0.12, detune: 7, attack: 0.04, decay: 0.2, sustain: 0.72, release: 0.2, cutoff: 0.62, glide: 1.08, filterEnv: 0.35, fx: { chorus: 0.24, delay: 0.24, reverb: 0.2 } }, ["warm", "modern"]],
      ["supersaw", "Hyper SuperSaw", { wave: "supersaw", unison: 5, sub: 0.14, detune: 18, level: 0.18, attack: 0.008, decay: 0.26, sustain: 0.78, release: 0.24, cutoff: 0.78, filterEnv: 0.45, fx: { chorus: 0.42, reverb: 0.3, delay: 0.24, drive: 0.16 } }, ["modern", "bright"]],
      ["pwm", "PWM Cyber", { wave: "pulse25", unison: 3, detune: 11, mod: "triangle", ratio: 1.005, depth: 0.65, level: 0.21, attack: 0.006, decay: 0.2, sustain: 0.7, release: 0.16, cutoff: 0.66, filterEnv: 0.48, fx: { chorus: 0.34, phaser: 0.22, delay: 0.2 } }, ["modern", "bright"]],
      ["sync", "Sync Screamer", { wave: "sawtooth", mod: "sawtooth", ratio: 3.5, depth: 3.8, mod2: "square", ratio2: 7, depth2: 1.6, pitchEnv: 1.18, level: 0.2, attack: 0.003, decay: 0.22, sustain: 0.64, release: 0.14, cutoff: 0.74, filterEnv: 0.55, filterQ: 3.8, fx: { drive: 0.38, resonance: 0.34, delay: 0.22 } }, ["acid", "retro"]],
      ["vox", "Formant Vox", { wave: "formant", unison: 3, detune: 8, mod: "sine", ratio: 2, depth: 0.8, level: 0.22, attack: 0.025, decay: 0.28, sustain: 0.72, release: 0.24, cutoff: 0.7, filterEnv: 0.36, fx: { phaser: 0.35, chorus: 0.28, reverb: 0.32 } }, ["warm", "air"]],
      ["crystal", "Crystal FM", { wave: "bell", mod: "sine", ratio: 3.5, depth: 3.4, mod2: "sine", ratio2: 7.01, depth2: 1.9, decay2: 0.24, level: 0.2, attack: 0.003, decay: 0.32, sustain: 0.45, release: 0.28, cutoff: 0.9, fx: { chorus: 0.3, delay: 0.32, reverb: 0.36 } }, ["bright", "air"]],
      ["arp-pluck", "Neon Pluck", { wave: "supersaw", unison: 3, detune: 9, mod: "triangle", ratio: 4, depth: 1.4, level: 0.22, attack: 0.002, decay: 0.15, sustain: 0.14, release: 0.12, cutoff: 0.58, filterEnv: 0.68, filterDecay: 0.14, filterQ: 3.2, fx: { delay: 0.34, chorus: 0.24, reverb: 0.22 } }, ["pluck", "modern"]],
      ["pwm-sweep", "PWM Sweep", { pulseWidth: 0.5, level: 0.22, attack: 0.006, decay: 0.2, sustain: 0.7, release: 0.14, cutoff: 0.62, filterEnv: 0.3, lfo: { wave: "triangle", rate: 0.6, depth: 0.75, delay: 0, target: "pulse" }, fx: { delay: 0.24, chorus: 0.18 } }, ["modern", "bright"]],
      ["tape-wow", "Tape Wow", { wave: "sawtooth", level: 0.2, attack: 0.03, decay: 0.22, sustain: 0.72, release: 0.18, cutoff: 0.6, filterEnv: 0.26, lfo: { wave: "sine", rate: 0.7, depth: 0.28, delay: 0.12, target: "pitch" }, fx: { drive: 0.18, chorus: 0.16 } }, ["retro", "warm"]],
      ["breathy", "Breathy Lead", { wave: "triangle", noise: 0.04, noiseWave: "n2", level: 0.22, attack: 0.04, decay: 0.24, sustain: 0.7, release: 0.24, cutoff: 0.7, lfo: { wave: "sine", rate: 5.5, depth: 0.22, delay: 0, target: "amp" }, fx: { reverb: 0.26, tone: 0.6 } }, ["air", "warm"]],
      ["filter-sweep", "Filter Sweep", { wave: "sawtooth", unison: 3, detune: 9, level: 0.19, attack: 0.008, decay: 0.22, sustain: 0.72, release: 0.16, cutoff: 0.5, filterType: "bandpass", filterEnv: 0.4, filterQ: 4.6, lfo: { wave: "triangle", rate: 0.35, depth: 0.6, delay: 0, target: "filter" }, fx: { resonance: 0.3, delay: 0.2 } }, ["acid", "modern"]],
      ["sync-ii", "Sync Screamer II", { wave: "sawtooth", mod: "sawtooth", ratio: 4.5, depth: 4.4, mod2: "square", ratio2: 9, depth2: 2.2, mod3: "triangle", ratio3: 1.5, depth3: 0.9, decay3: 0.12, pitchEnv: 1.2, level: 0.19, attack: 0.003, decay: 0.22, sustain: 0.62, release: 0.14, cutoff: 0.72, filterEnv: 0.52, filterQ: 4.2, fx: { drive: 0.42, resonance: 0.36, delay: 0.2 } }, ["acid", "retro"]],
      ["portamento-horn", "Portamento Horn", { wave: "sawtooth", unison: 3, detune: 9, mod: "square", ratio: 2, depth: 0.6, level: 0.19, attack: 0.05, decay: 0.24, sustain: 0.7, release: 0.2, cutoff: 0.52, filterEnv: 0.46, filterDecay: 0.26, glide: 1.1, fx: { chorus: 0.28, reverb: 0.26 } }, ["warm", "retro"]],
      ["digital", "Digital Lead", { wave: "square", level: 0.22, attack: 0.004, decay: 0.18, sustain: 0.68, release: 0.14, cutoff: 0.78, filterEnv: 0.3, fx: { inserts: [{ type: "bitcrush", mix: 0.45, bits: 7, tone: 0.4 }], delay: 0.2 } }, ["modern", "bright"]],
      ["chorus-lead", "Chorus Lead", { wave: "sawtooth", unison: 3, detune: 10, level: 0.2, attack: 0.012, decay: 0.22, sustain: 0.74, release: 0.18, cutoff: 0.62, filterEnv: 0.28, fx: { inserts: [{ type: "chorus", mix: 0.5, rate: 0.8, depth: 0.45, feedback: 0.2, tone: 0.4 }], delay: 0.18 } }, ["warm", "modern"]]
    ] },
    { category: "pads", prefix: "pad", items: [
      ["warm", "Warm", { wave: "triangle", unison: 3, level: 0.18, second: 0.16, detune: 6, attack: 0.32, decay: 0.5, sustain: 0.76, release: 0.65, cutoff: 0.5, filterEnv: 0.2, fx: { chorus: 0.3, reverb: 0.34 } }, ["warm", "air"]],
      ["air", "Air", { wave: "sine", unison: 3, detune: 7, level: 0.22, noise: 0.025, attack: 0.55, decay: 0.5, sustain: 0.7, release: 0.8, cutoff: 0.9, fx: { chorus: 0.28, reverb: 0.45 } }, ["air", "bright"]],
      ["string", "String", { wave: "sawtooth", unison: 3, level: 0.14, second: 0.1, detune: 10, attack: 0.18, decay: 0.35, sustain: 0.72, release: 0.5, cutoff: 0.56, filterEnv: 0.24, fx: { chorus: 0.38, reverb: 0.36 } }, ["warm", "retro"]],
      ["choir", "Choir", { wave: "formant", unison: 3, detune: 7, mod: "sine", ratio: 2, depth: 0.7, level: 0.18, attack: 0.36, decay: 0.42, sustain: 0.68, release: 0.8, cutoff: 0.58, filterEnv: 0.22, fx: { chorus: 0.36, reverb: 0.44 } }, ["warm", "air"]],
      ["glass", "Glass", { wave: "bell", mod: "sine", ratio: 4.8, depth: 3.4, mod2: "sine", ratio2: 2, depth2: 1.1, level: 0.2, attack: 0.1, decay: 0.5, sustain: 0.46, release: 0.55, cutoff: 0.88, fx: { chorus: 0.26, reverb: 0.42, delay: 0.2 } }, ["bright", "air"]],
      ["dark", "Dark", { wave: "sawtooth", sub: 0.12, level: 0.14, second: 0.08, detune: 5, attack: 0.48, decay: 0.4, sustain: 0.76, release: 0.7, cutoff: 0.26, filterEnv: 0.18, fx: { phaser: 0.24, reverb: 0.38 } }, ["dark", "warm"]],
      ["dream", "Dream", { wave: "sine", unison: 3, detune: 8, mod: "triangle", ratio: 1.5, depth: 0.9, level: 0.18, attack: 0.6, decay: 0.55, sustain: 0.66, release: 1, cutoff: 0.72, fx: { chorus: 0.34, delay: 0.24, reverb: 0.48 } }, ["air", "warm"]],
      ["analog", "Analog", { wave: "sawtooth", unison: 3, level: 0.15, second: 0.12, detune: 13, attack: 0.24, decay: 0.38, sustain: 0.7, release: 0.62, cutoff: 0.48, filterEnv: 0.3, fx: { chorus: 0.35, drive: 0.12, reverb: 0.32 } }, ["warm", "retro"]],
      ["pulse", "Pulse", { wave: "pulse25", mod: "sine", ratio: 1.01, depth: 0.28, level: 0.16, attack: 0.28, decay: 0.4, sustain: 0.66, release: 0.65, cutoff: 0.44, filterEnv: 0.25, fx: { chorus: 0.32, phaser: 0.2, reverb: 0.3 } }, ["retro", "warm"]],
      ["space", "Space", { wave: "sine", unison: 3, detune: 9, mod: "sine", ratio: 3, depth: 2.6, level: 0.14, noise: 0.02, attack: 0.75, decay: 0.7, sustain: 0.6, release: 1.1, cutoff: 0.78, fx: { phaser: 0.32, delay: 0.28, reverb: 0.55 } }, ["air", "dark"]],
      ["juno", "Juno Chorus", { wave: "supersaw", unison: 5, sub: 0.12, detune: 14, level: 0.16, attack: 0.22, decay: 0.45, sustain: 0.76, release: 0.68, cutoff: 0.52, filterEnv: 0.34, filterDecay: 0.48, fx: { chorus: 0.58, reverb: 0.38 } }, ["warm", "retro"]],
      ["cs80", "CS-80 Brass Pad", { wave: "sawtooth", unison: 3, detune: 11, mod: "triangle", ratio: 1, depth: 0.55, level: 0.18, attack: 0.16, decay: 0.42, sustain: 0.72, release: 0.6, cutoff: 0.46, filterEnv: 0.54, filterDecay: 0.38, filterQ: 2.2, fx: { chorus: 0.32, reverb: 0.44, drive: 0.14 } }, ["warm", "retro"]],
      ["shimmer", "Shimmer Cathedral", { wave: "bell", unison: 3, detune: 10, mod: "sine", ratio: 2, depth: 2.4, mod2: "sine", ratio2: 4, depth2: 1.5, level: 0.16, attack: 0.45, decay: 0.65, sustain: 0.68, release: 1.15, cutoff: 0.88, fx: { chorus: 0.44, reverb: 0.65, delay: 0.26 } }, ["air", "bright"]],
      ["nebula", "Phaser Nebula", { wave: "supersaw", unison: 5, detune: 16, mod: "triangle", ratio: 1.5, depth: 1.1, level: 0.15, attack: 0.42, decay: 0.55, sustain: 0.72, release: 0.85, cutoff: 0.54, filterEnv: 0.38, filterQ: 3.1, fx: { phaser: 0.56, chorus: 0.34, reverb: 0.46 } }, ["acid", "air"]],
      ["tape", "Mellotron Tape", { wave: "formant", second: 0.11, detune: 9, noise: 0.015, level: 0.19, attack: 0.26, decay: 0.44, sustain: 0.68, release: 0.52, cutoff: 0.42, filterEnv: 0.18, fx: { drive: 0.22, chorus: 0.26, reverb: 0.28 } }, ["retro", "warm"]],
      ["solina", "Solina Ensemble", { wave: "w9999", unison: 5, detune: 15, level: 0.16, attack: 0.28, decay: 0.4, sustain: 0.78, release: 0.64, cutoff: 0.66, filterEnv: 0.22, fx: { phaser: 0.38, chorus: 0.52, reverb: 0.4 } }, ["retro", "warm"]],
      ["slow-tide", "Slow Tide", { wave: "sine", unison: 3, detune: 9, level: 0.18, attack: 0.7, decay: 0.8, sustain: 0.68, release: 1.2, cutoff: 0.78, lfo: { wave: "sine", rate: 0.18, depth: 0.34, delay: 0.2, target: "amp" }, fx: { reverb: 0.5, delay: 0.2 } }, ["air", "warm"]],
      ["air-choir", "Air Choir", { wave: "formant", unison: 3, detune: 8, mod: "sine", ratio: 2, depth: 0.8, noise: 0.02, noiseWave: "n2", level: 0.17, attack: 0.5, decay: 0.6, sustain: 0.7, release: 1, cutoff: 0.64, filterEnv: 0.2, fx: { chorus: 0.4, reverb: 0.52 } }, ["air", "warm"]],
      ["tape-pad", "Tape Pad", { wave: "sawtooth", unison: 3, detune: 12, noise: 0.02, noiseWave: "n3", level: 0.16, attack: 0.34, decay: 0.5, sustain: 0.74, release: 0.7, cutoff: 0.44, filterEnv: 0.18, fx: { inserts: [{ type: "softclip", mix: 0.5, drive: 0.35, tone: 0.55 }], chorus: 0.3, reverb: 0.3 } }, ["retro", "warm"]],
      ["metal-pad", "Metal Pad", { wave: "sawtooth", mod: "square", ratio: 5, depth: 2.6, mod2: "triangle", ratio2: 2.5, depth2: 1.2, level: 0.15, attack: 0.3, decay: 0.5, sustain: 0.7, release: 0.8, cutoff: 0.36, filterEnv: 0.3, filterQ: 3.6, fx: { phaser: 0.3, reverb: 0.34 } }, ["dark", "bright"]],
      ["glass-pad", "Glass Pad", { wave: "bell", mod: "sine", ratio: 5.5, depth: 3.6, mod2: "sine", ratio2: 2.2, depth2: 1.6, decay2: 0.3, level: 0.17, attack: 0.18, decay: 0.6, sustain: 0.6, release: 0.9, cutoff: 0.9, fx: { chorus: 0.34, reverb: 0.48, delay: 0.22 } }, ["bright", "air"]],
      ["sub-pad", "Sub Pad", { wave: "sawtooth", sub: 0.3, subWave: "sine", level: 0.18, attack: 0.45, decay: 0.6, sustain: 0.78, release: 0.85, cutoff: 0.3, filterEnv: 0.12, fx: { tone: 0.36, reverb: 0.28 } }, ["dark", "warm"]],
      ["warm-swell", "Warm Swell", { wave: "triangle", unison: 3, detune: 7, second: 0.14, level: 0.18, attack: 0.85, decay: 0.9, sustain: 0.74, release: 1.1, cutoff: 0.52, filterEnv: 0.34, filterDecay: 0.7, fx: { chorus: 0.34, reverb: 0.4 } }, ["warm", "air"]],
      ["reverse-pad", "Reverse Pad", { wave: "sawtooth", unison: 3, detune: 10, noise: 0.03, noiseWave: "n2", level: 0.16, attack: 0.9, decay: 0.9, sustain: 0.72, release: 1.2, cutoff: 0.78, filterEnv: -0.55, filterDecay: 0.85, fx: { phaser: 0.28, reverb: 0.46 } }, ["air", "modern"]]
    ] },
    { category: "keys", prefix: "key", items: [
      ["electric-piano", "Electric Piano", { wave: "sine", mod: "triangle", ratio: 3, depth: 3, mod2: "sine", ratio2: 14, depth2: 0.9, decay2: 0.09, level: 0.23, attack: 0.004, decay: 0.42, sustain: 0.2, release: 0.3, cutoff: 0.86, fx: { chorus: 0.26, reverb: 0.22 } }, ["warm", "retro"]],
      ["fm-piano", "FM Piano", { wave: "sine", mod: "sine", ratio: 5, depth: 4.2, mod2: "sine", ratio2: 1, depth2: 1.6, level: 0.21, attack: 0.002, decay: 0.34, sustain: 0.12, release: 0.24, cutoff: 0.92, fx: { chorus: 0.32, reverb: 0.26 } }, ["bright", "retro"]],
      ["bell", "Bell", { wave: "bell", mod: "sine", ratio: 6.2, depth: 5, mod2: "sine", ratio2: 2.75, depth2: 2.2, level: 0.2, attack: 0.002, decay: 0.8, sustain: 0.03, release: 0.7, cutoff: 0.95, fx: { reverb: 0.38, delay: 0.2 } }, ["bright", "air"]],
      ["celesta", "Celesta", { wave: "sine", mod: "triangle", ratio: 4, depth: 2.5, mod2: "sine", ratio2: 8, depth2: 1.1, level: 0.21, attack: 0.001, decay: 0.62, sustain: 0.03, release: 0.55, cutoff: 0.9, fx: { reverb: 0.3 } }, ["bright", "air"]],
      ["music-box", "Music Box", { wave: "sine", mod: "sine", ratio: 7.3, depth: 2.7, level: 0.2, attack: 0.001, decay: 0.7, sustain: 0.025, release: 0.6, cutoff: 0.94, fx: { reverb: 0.34, delay: 0.18 } }, ["bright", "pluck"]],
      ["marimba", "Marimba", { wave: "triangle", mod: "sine", ratio: 4, depth: 1.8, decay2: 0.06, level: 0.24, attack: 0.002, decay: 0.33, sustain: 0.05, release: 0.24, cutoff: 0.82, fx: { reverb: 0.22 } }, ["warm", "pluck"]],
      ["kalimba", "Kalimba", { wave: "triangle", mod: "square", ratio: 5, depth: 1.3, level: 0.23, attack: 0.002, decay: 0.42, sustain: 0.08, release: 0.28, cutoff: 0.78, fx: { chorus: 0.18, reverb: 0.28 } }, ["warm", "pluck"]],
      ["harp", "Harp", { wave: "triangle", second: 0.08, detune: 4, mod: "sine", ratio: 2, depth: 0.6, level: 0.22, attack: 0.003, decay: 0.52, sustain: 0.08, release: 0.4, cutoff: 0.8, filterEnv: 0.28, fx: { reverb: 0.36, delay: 0.16 } }, ["pluck", "warm"]],
      ["clav", "Clav", { wave: "pulse12", mod: "triangle", ratio: 2, depth: 1.6, level: 0.21, attack: 0.001, decay: 0.16, sustain: 0.08, release: 0.12, cutoff: 0.48, filterEnv: 0.52, filterDecay: 0.14, filterQ: 3.4, fx: { phaser: 0.34, drive: 0.18 } }, ["pluck", "retro"]],
      ["soft-pluck", "Soft Pluck", { wave: "triangle", unison: 3, level: 0.2, second: 0.07, detune: 5, attack: 0.008, decay: 0.38, sustain: 0.12, release: 0.3, cutoff: 0.64, filterEnv: 0.32, fx: { chorus: 0.24, delay: 0.22, reverb: 0.26 } }, ["pluck", "warm"]],
      ["wurli", "Wurlitzer Reed", { wave: "w9999", mod: "sine", ratio: 2, depth: 2.1, mod2: "triangle", ratio2: 7, depth2: 0.8, decay2: 0.08, level: 0.22, attack: 0.003, decay: 0.46, sustain: 0.24, release: 0.26, cutoff: 0.72, filterEnv: 0.3, fx: { drive: 0.24, phaser: 0.22, reverb: 0.2 } }, ["warm", "retro"]],
      ["dx7-tines", "DX7 Crystal Tines", { wave: "sine", mod: "sine", ratio: 1, depth: 2.4, mod2: "sine", ratio2: 14, depth2: 2.8, decay2: 0.11, level: 0.21, attack: 0.002, decay: 0.52, sustain: 0.18, release: 0.38, cutoff: 0.94, fx: { chorus: 0.42, reverb: 0.32, delay: 0.16 } }, ["bright", "retro"]],
      ["vibes", "Jazz Vibraphone", { wave: "bell", mod: "sine", ratio: 4, depth: 2.2, second: 0.09, detune: 7, level: 0.22, attack: 0.003, decay: 0.68, sustain: 0.14, release: 0.58, cutoff: 0.86, fx: { chorus: 0.34, phaser: 0.18, reverb: 0.36 } }, ["warm", "retro"]],
      ["organ", "Drawbar Organ", { wave: "w9999", sub: 0.18, subWave: "sine", unison: 3, detune: 6, mod: "sine", ratio: 3, depth: 0.85, level: 0.2, attack: 0.005, decay: 0.18, sustain: 0.86, release: 0.09, cutoff: 0.8, fx: { chorus: 0.38, drive: 0.26, reverb: 0.24 } }, ["retro", "warm"]],
      ["harpsi", "Cyber Harpsichord", { wave: "pulse12", second: 0.12, detune: 8, mod: "sawtooth", ratio: 3, depth: 1.9, level: 0.2, attack: 0.001, decay: 0.28, sustain: 0.06, release: 0.18, cutoff: 0.82, filterEnv: 0.48, fx: { chorus: 0.22, reverb: 0.28 } }, ["pluck", "bright"]],
      ["koto", "Synth Koto", { wave: "triangle", mod: "sawtooth", ratio: 3.5, depth: 3.1, decay2: 0.07, pitchEnv: 1.06, level: 0.23, attack: 0.001, decay: 0.32, sustain: 0.04, release: 0.22, cutoff: 0.78, filterEnv: 0.56, filterDecay: 0.12, fx: { delay: 0.26, reverb: 0.32 } }, ["pluck", "bright"]],
      ["rhodes-mk2", "Rhodes Mk2", { wave: "sine", mod: "triangle", ratio: 3, depth: 2.8, mod2: "sine", ratio2: 14, depth2: 1.1, decay2: 0.1, level: 0.23, attack: 0.005, decay: 0.46, sustain: 0.24, release: 0.32, cutoff: 0.84, fx: { chorus: 0.24, reverb: 0.24 } }, ["warm", "retro"]],
      ["wurlitzer-mk2", "Wurlitzer Mk2", { wave: "w9999", mod: "sine", ratio: 2, depth: 2.4, mod2: "triangle", ratio2: 7, depth2: 1.1, decay2: 0.09, level: 0.22, attack: 0.004, decay: 0.5, sustain: 0.28, release: 0.28, cutoff: 0.7, filterEnv: 0.28, fx: { drive: 0.3, phaser: 0.22, reverb: 0.2 } }, ["warm", "retro"]],
      ["clavinet-mk2", "Clavinet Mk2", { wave: "pulse12", mod: "triangle", ratio: 2, depth: 1.8, level: 0.21, attack: 0.001, decay: 0.18, sustain: 0.1, release: 0.14, cutoff: 0.44, filterEnv: 0.6, filterDecay: 0.15, filterQ: 4.2, fx: { phaser: 0.3, drive: 0.2 } }, ["pluck", "retro"]],
      ["vibraphone-mk2", "Vibraphone Mk2", { wave: "bell", mod: "sine", ratio: 4.2, depth: 2.6, second: 0.09, detune: 7, level: 0.21, attack: 0.003, decay: 0.72, sustain: 0.16, release: 0.6, cutoff: 0.88, lfo: { wave: "sine", rate: 4.6, depth: 0.16, delay: 0.3, target: "amp" }, fx: { chorus: 0.3, reverb: 0.36 } }, ["warm", "bright"]],
      ["marimba-deep", "Marimba Deep", { wave: "triangle", mod: "sine", ratio: 3.4, depth: 2.2, mod2: "sine", ratio2: 6, depth2: 0.8, decay2: 0.08, level: 0.24, attack: 0.002, decay: 0.38, sustain: 0.05, release: 0.26, cutoff: 0.68, filterEnv: 0.4, filterDecay: 0.14, fx: { reverb: 0.26, tone: 0.5 } }, ["dark", "warm"]],
      ["music-box-pluck", "Music Box Pluck", { wave: "sine", mod: "sine", ratio: 7.5, depth: 3.1, level: 0.2, attack: 0.001, decay: 0.62, sustain: 0.02, release: 0.5, cutoff: 0.92, fx: { reverb: 0.32, delay: 0.2 } }, ["pluck", "bright"]],
      ["koto-bright", "Koto Bright", { wave: "triangle", mod: "sawtooth", ratio: 4.2, depth: 3.4, pitchEnv: 1.08, level: 0.23, attack: 0.001, decay: 0.3, sustain: 0.04, release: 0.2, cutoff: 0.88, filterEnv: 0.54, filterDecay: 0.11, fx: { delay: 0.26, reverb: 0.3 } }, ["pluck", "bright"]],
      ["organ-mk2", "Organ Mk2", { wave: "w9999", sub: 0.18, subWave: "sine", unison: 3, detune: 6, mod: "sine", ratio: 3, depth: 0.95, level: 0.2, attack: 0.006, decay: 0.2, sustain: 0.84, release: 0.1, cutoff: 0.78, fx: { inserts: [{ type: "rotary", mix: 0.36, rate: 1.2, depth: 0.5, tone: 0.5 }], chorus: 0.3, reverb: 0.24 } }, ["retro", "warm"]]
    ] }
  ];

  const presets = groups.flatMap(group => group.items.map(([slug, name, data, tags]) => {
    const preset = { id: `${group.prefix}-${slug}`, name, category: group.category, tags: tags || [] };
    if (group.category === "drums") preset.drumNote = data;
    else preset.patch = data;
    return preset;
  }));
  const gmData = root.SynthStudioGMData;
  const gm = (gmData?.programs || []).map((program, index) => ({ id: `gm-${index}`, name: program.name, category: "gm", program: index }));
  const byId = new Map([...presets, ...gm, { id: "drums-kit", name: "Synth Studio Kit", category: "drums" }, { id: "gm-drums", name: "GM Drums", category: "gm" }].map(item => [item.id, item]));

  root.SynthStudioPresets = {
    categories,
    presets,
    gm,
    tags: ["acid", "warm", "dark", "bright", "pluck", "retro", "modern", "air"],
    drums: presets.filter(item => item.category === "drums"),
    get(id) { return byId.get(id); }
  };
})(typeof window !== "undefined" ? window : globalThis);
