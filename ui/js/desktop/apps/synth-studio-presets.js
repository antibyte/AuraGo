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
      ["sub-kick", "Sub Kick", 35], ["punch-kick", "Punch Kick", 36], ["snare", "Snare", 38],
      ["clap", "Clap", 39], ["closed-hi-hat", "Closed Hi-Hat", 42], ["open-hi-hat", "Open Hi-Hat", 46],
      ["low-tom", "Low Tom", 45], ["rimshot", "Rimshot", 37], ["shaker", "Shaker", 70], ["crash", "Crash", 49]
    ] },
    { category: "bass", prefix: "bass", items: [
      ["sub", "Sub", { wave: "sine", level: 0.34, attack: 0.008, decay: 0.25, sustain: 0.78, release: 0.12, cutoff: 0.42 }],
      ["round", "Round", { wave: "triangle", mod: "sine", ratio: 2, depth: 0.28, level: 0.25, attack: 0.012, decay: 0.2, sustain: 0.72, release: 0.16, cutoff: 0.5 }],
      ["acid", "Acid", { wave: "sawtooth", mod: "square", ratio: 2, depth: 1.7, level: 0.22, attack: 0.004, decay: 0.13, sustain: 0.52, release: 0.08, cutoff: 0.46 }],
      ["saw", "Saw", { wave: "sawtooth", level: 0.2, second: 0.16, detune: 11, attack: 0.006, decay: 0.24, sustain: 0.7, release: 0.12, cutoff: 0.62 }],
      ["square", "Square", { wave: "square", level: 0.2, attack: 0.006, decay: 0.22, sustain: 0.7, release: 0.12, cutoff: 0.58 }],
      ["fm", "FM", { wave: "sine", mod: "sine", ratio: 3.3, depth: 3.2, level: 0.24, attack: 0.004, decay: 0.18, sustain: 0.62, release: 0.1, cutoff: 0.8 }],
      ["pluck", "Pluck", { wave: "triangle", mod: "sine", ratio: 5, depth: 0.8, level: 0.24, attack: 0.002, decay: 0.12, sustain: 0.12, release: 0.08, cutoff: 0.68 }],
      ["reese", "Reese", { wave: "sawtooth", level: 0.18, second: 0.17, detune: 19, attack: 0.015, decay: 0.32, sustain: 0.76, release: 0.22, cutoff: 0.54 }],
      ["wobble", "Wobble", { wave: "square", mod: "triangle", ratio: 0.5, depth: 1.3, level: 0.18, attack: 0.01, decay: 0.18, sustain: 0.72, release: 0.15, cutoff: 0.44 }],
      ["rubber", "Rubber", { wave: "triangle", mod: "square", ratio: 4, depth: 2.2, level: 0.2, attack: 0.002, decay: 0.1, sustain: 0.4, release: 0.1, cutoff: 0.7 }]
    ] },
    { category: "leads", prefix: "lead", items: [
      ["saw", "Saw", { wave: "sawtooth", level: 0.18, second: 0.12, detune: 8, attack: 0.008, decay: 0.2, sustain: 0.74, release: 0.16, cutoff: 0.72 }],
      ["square", "Square", { wave: "square", level: 0.22, attack: 0.004, decay: 0.18, sustain: 0.7, release: 0.14, cutoff: 0.68 }],
      ["pulse", "Pulse", { wave: "square", mod: "sine", ratio: 2, depth: 0.5, level: 0.2, attack: 0.006, decay: 0.18, sustain: 0.65, release: 0.12, cutoff: 0.38 }],
      ["triangle", "Triangle", { wave: "triangle", level: 0.26, attack: 0.01, decay: 0.2, sustain: 0.7, release: 0.16, cutoff: 0.78 }],
      ["sine", "Sine", { wave: "sine", level: 0.28, attack: 0.015, decay: 0.22, sustain: 0.72, release: 0.18, cutoff: 0.9 }],
      ["fm", "FM", { wave: "sine", mod: "sine", ratio: 2.7, depth: 4.2, level: 0.2, attack: 0.004, decay: 0.18, sustain: 0.66, release: 0.12, cutoff: 0.82 }],
      ["brass", "Brass", { wave: "sawtooth", mod: "square", ratio: 2, depth: 0.7, level: 0.2, attack: 0.035, decay: 0.2, sustain: 0.68, release: 0.18, cutoff: 0.58 }],
      ["chiptune", "Chiptune", { wave: "square", level: 0.28, attack: 0.002, decay: 0.08, sustain: 0.82, release: 0.04, cutoff: 0.9 }],
      ["laser", "Laser", { wave: "sawtooth", mod: "sine", ratio: 5, depth: 2.2, level: 0.22, attack: 0.001, decay: 0.22, sustain: 0.08, release: 0.08, cutoff: 0.86, glide: 1.8 }],
      ["glide", "Glide", { wave: "sawtooth", level: 0.18, second: 0.12, detune: 6, attack: 0.04, decay: 0.2, sustain: 0.72, release: 0.2, cutoff: 0.62, glide: 1.08 }]
    ] },
    { category: "pads", prefix: "pad", items: [
      ["warm", "Warm", { wave: "triangle", level: 0.18, second: 0.16, detune: 5, attack: 0.32, decay: 0.5, sustain: 0.76, release: 0.65, cutoff: 0.5 }],
      ["air", "Air", { wave: "sine", level: 0.22, noise: 0.025, attack: 0.55, decay: 0.5, sustain: 0.7, release: 0.8, cutoff: 0.9 }],
      ["string", "String", { wave: "sawtooth", level: 0.14, second: 0.1, detune: 9, attack: 0.18, decay: 0.35, sustain: 0.72, release: 0.5, cutoff: 0.56 }],
      ["choir", "Choir", { wave: "triangle", mod: "sine", ratio: 2, depth: 0.7, level: 0.18, attack: 0.36, decay: 0.42, sustain: 0.68, release: 0.8, cutoff: 0.54 }],
      ["glass", "Glass", { wave: "sine", mod: "sine", ratio: 4.8, depth: 3.4, level: 0.2, attack: 0.1, decay: 0.5, sustain: 0.46, release: 0.55, cutoff: 0.88 }],
      ["dark", "Dark", { wave: "sawtooth", level: 0.14, second: 0.08, detune: 4, attack: 0.48, decay: 0.4, sustain: 0.76, release: 0.7, cutoff: 0.24 }],
      ["dream", "Dream", { wave: "sine", mod: "triangle", ratio: 1.5, depth: 0.9, level: 0.18, attack: 0.6, decay: 0.55, sustain: 0.66, release: 1, cutoff: 0.72 }],
      ["analog", "Analog", { wave: "sawtooth", level: 0.15, second: 0.12, detune: 13, attack: 0.24, decay: 0.38, sustain: 0.7, release: 0.62, cutoff: 0.48 }],
      ["pulse", "Pulse", { wave: "square", mod: "sine", ratio: 1.01, depth: 0.24, level: 0.16, attack: 0.28, decay: 0.4, sustain: 0.66, release: 0.65, cutoff: 0.4 }],
      ["space", "Space", { wave: "sine", mod: "sine", ratio: 3, depth: 2.6, level: 0.14, noise: 0.02, attack: 0.75, decay: 0.7, sustain: 0.6, release: 1.1, cutoff: 0.78 }]
    ] },
    { category: "keys", prefix: "key", items: [
      ["electric-piano", "Electric Piano", { wave: "sine", mod: "triangle", ratio: 3, depth: 3, level: 0.23, attack: 0.004, decay: 0.42, sustain: 0.2, release: 0.3, cutoff: 0.86 }],
      ["fm-piano", "FM Piano", { wave: "sine", mod: "sine", ratio: 5, depth: 4.2, level: 0.2, attack: 0.002, decay: 0.34, sustain: 0.12, release: 0.24, cutoff: 0.92 }],
      ["bell", "Bell", { wave: "sine", mod: "sine", ratio: 6.2, depth: 5, level: 0.2, attack: 0.002, decay: 0.8, sustain: 0.03, release: 0.7, cutoff: 0.95 }],
      ["celesta", "Celesta", { wave: "sine", mod: "triangle", ratio: 4, depth: 2.5, level: 0.21, attack: 0.001, decay: 0.62, sustain: 0.03, release: 0.55, cutoff: 0.9 }],
      ["music-box", "Music Box", { wave: "sine", mod: "sine", ratio: 7.3, depth: 2.7, level: 0.2, attack: 0.001, decay: 0.7, sustain: 0.025, release: 0.6, cutoff: 0.94 }],
      ["marimba", "Marimba", { wave: "triangle", mod: "sine", ratio: 3, depth: 1.6, level: 0.24, attack: 0.002, decay: 0.33, sustain: 0.05, release: 0.24, cutoff: 0.82 }],
      ["kalimba", "Kalimba", { wave: "triangle", mod: "square", ratio: 5, depth: 1.2, level: 0.23, attack: 0.002, decay: 0.42, sustain: 0.08, release: 0.28, cutoff: 0.78 }],
      ["harp", "Harp", { wave: "triangle", mod: "sine", ratio: 2, depth: 0.6, level: 0.22, attack: 0.003, decay: 0.52, sustain: 0.08, release: 0.4, cutoff: 0.8 }],
      ["clav", "Clav", { wave: "square", mod: "triangle", ratio: 2, depth: 1.5, level: 0.19, attack: 0.001, decay: 0.16, sustain: 0.08, release: 0.12, cutoff: 0.42 }],
      ["soft-pluck", "Soft Pluck", { wave: "triangle", level: 0.2, second: 0.07, detune: 4, attack: 0.008, decay: 0.38, sustain: 0.12, release: 0.3, cutoff: 0.64 }]
    ] }
  ];

  const presets = groups.flatMap(group => group.items.map(([slug, name, data]) => {
    const preset = { id: `${group.prefix}-${slug}`, name, category: group.category };
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
    drums: presets.filter(item => item.category === "drums"),
    get(id) { return byId.get(id); }
  };
})(typeof window !== "undefined" ? window : globalThis);
