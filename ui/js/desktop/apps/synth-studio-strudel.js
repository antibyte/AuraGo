// SPDX-License-Identifier: AGPL-3.0-or-later
(function () {
    'use strict';
    const KEY = 'aurago:synth-studio:strudel:1', LIMIT = 65536;
    const fits = value => typeof value === 'string' && new TextEncoder().encode(value).byteLength <= LIMIT;
    const examples = {
        synthwave: [
            '// Neon Horizon — 114 BPM Retro Synthwave',
            'setcpm(114/4)',
            'stack(',
            '  // Four-on-the-floor kick, gated 80s snare/clap, hats & 4-bar tom fill',
            '  s("bd*4").gain(.58).orbit(1),',
            '  s("~ [sd,cp] ~ [sd,cp]").room(.45).roomsize(1.6).gain(.48).orbit(1),',
            '  s("hh*8").hpf(6800).gain(".14 .26 .18 .3").orbit(1),',
            '  s("~ ~ ~ [~ oh]").cut(1).gain(.22).orbit(1),',
            '  s("ht mt lt lt").fast(2).mask("<0 0 0 [0 0 0 1]>").room(.35).gain(.42).orbit(1),',
            '  // Driving 16th-note sawtooth bassline',
            '  note("<a1 f1 c2 g1 a1 f1 d2 e1>*8")',
            '    .s("sawtooth").lpf(sine.range(380, 1400).slow(8)).lpq(4)',
            '    .lpenv(2.2).lpdecay(.12).decay(.18).sustain(.15)',
            '    .distort(.35).gain(.52).orbit(2),',
            '  // Wide detuned supersaw pad chords',
            '  chord("<Am9 FM7 CM9 G6 Am9 FM7 Dm9 E7>").voicing()',
            '    .s("supersaw").unison(5).spread(.85).detune(.22)',
            '    .lpf(sine.range(700, 2200).slow(16))',
            '    .attack(.35).decay(.6).sustain(.7).release(.8)',
            '    .room(.55).roomsize(4).gain(.38).orbit(3),',
            '  // Shimmering PWM stereo 16th-note arpeggio',
            '  n("<[0 2 4 7 4 2 4 7] [0 2 4 6 4 2 4 6] [0 2 4 7 9 7 4 2] [0 2 4 7 4 2 1 0]>*2")',
            '    .scale("<a3:minor f3:major c4:major g3:major a3:minor f3:major d4:dorian e3:phrygian:dominant>")',
            '    .s("pulse").pw(.3).pwrate(1.5).pwsweep(.2)',
            '    .lpf(2600).lpenv(2).lpdecay(.09).decay(.14).sustain(0)',
            '    .delay(.42).delaysync(3/16).delayfeedback(.42)',
            '    .pan(sine.range(.2, .8).slow(4)).gain(.16).orbit(4),',
            '  // Soaring vibrato lead melody',
            '  note("<[e5 ~ d5 c5 ~ b4 c5 e5] [a4 ~ ~ g4 a4 ~ c5 ~] [g5 ~ e5 d5 ~ c5 d5 e5] [d5 ~ ~ b4 d5 ~ ~ ~] [e5 ~ g5 a5 ~ g5 e5 d5] [c5 ~ a4 c5 ~ d5 e5 ~] [f5 ~ e5 d5 ~ c5 b4 a4] [b4 ~ ~ ~ gs4 ~ ~ ~]>")',
            '    .s("sawtooth").lpf(2800).lpenv(1.6).lpdecay(.2)',
            '    .vib(5.5).vibmod(.22).attack(.03).decay(.25).sustain(.65).release(.45)',
            '    .delay(.35).delaysync(3/8).delayfeedback(.38)',
            '    .room(.45).roomsize(3.5).gain(.34).orbit(5)',
            ')'
        ].join('\n'),
        acid: [
            '// Acid Tunnel — 134 BPM TB-303 Acid Techno',
            'setcpm(134/4)',
            'stack(',
            '  // Saturated 909 kick, offbeat open hat, 16th hats, clap & Euclidean rim',
            '  s("bd*4").pdecay(.055).penv(32).distort(.4).gain(.64).orbit(1),',
            '  s("[~ oh]*4").cut(1).hpf(6500).decay(.18).gain(.28).orbit(1),',
            '  s("hh*16").hpf(8000).decay(.03).gain(".1 .22 .14 .26").orbit(1),',
            '  s("~ cp ~ cp").room(.25).roomsize(1.5).gain(.42).orbit(1),',
            '  s("rim(5,16)").pan(.65).delay(.25).delaysync(3/16).delayfeedback(.3).gain(.24).orbit(1),',
            '  // Squelching resonant ladder-filter 303 acid line',
            '  note("<[f2 f2 f3 f2 eb2 f2 ab2 f2 c3 f2 f2 eb2 c2 f2 g2 ab2] [f2 f2 f3 c3 eb2 f2 ab2 f2 c3 f3 eb3 c3 ab2 g2 f2 eb2]>")',
            '    .s("sawtooth").ftype("ladder")',
            '    .lpf(sine.range(240, 1900).slow(8)).lpq(10)',
            '    .lpenv(4.5).lpdecay(.11).lpsustain(0).decay(.16).sustain(.1)',
            '    .distort(.75).delay(.22).delaysync(3/16).delayfeedback(.38)',
            '    .gain(.42).orbit(2),',
            '  // Syncopated Euclidean acid counter-pulse',
            '  note("f3 c4 eb4 g3 ab3").struct("t(7,16)")',
            '    .s("pulse").pw(.22).lpf(perlin.range(500, 2800).slow(6)).lpq(6)',
            '    .lpenv(3).lpdecay(.09).decay(.12).sustain(0)',
            '    .delay(.35).delaysync(3/16).delayfeedback(.45)',
            '    .pan(sine.range(.2, .8).slow(3)).gain(.14).orbit(3),',
            '  // Dark warehouse supersaw rave stabs',
            '  chord("<Fm9 ~ ~ Fm9 ~ ~ Eb6 [DbM7 C7]>").voicing()',
            '    .s("supersaw").unison(4).spread(.8)',
            '    .lpf(1400).lpenv(2.5).lpdecay(.18).decay(.22).sustain(0)',
            '    .room(.65).roomsize(4.5).delay(.4).delaysync(3/16).delayfeedback(.5)',
            '    .gain(.28).orbit(4)',
            ')'
        ].join('\n'),
        dnb: [
            '// Midnight Liquid — 172 BPM Liquid Drum & Bass',
            'setcpm(172/4)',
            'stack(',
            '  // Rolling two-step breakbeat with ghost snares & 4-bar hat roll',
            '  s("bd ~ ~ ~ ~ ~ ~ ~ ~ ~ bd ~ ~ ~ ~ ~").pdecay(.045).gain(.62).orbit(1),',
            '  s("~ ~ ~ ~ sd ~ ~ ~ ~ ~ ~ ~ sd ~ ~ ~").room(.22).roomsize(1.3).gain(.54).orbit(1),',
            '  s("~ ~ ~ ~ ~ ~ ~ sd ~ sd ~ ~ ~ ~ ~ sd").decay(.08).gain(.2).orbit(1),',
            '  s("hh*8").hpf(7500).decay(".03 .02 .04 .02").gain(".22 .12 .26 .14").every(4, x => x.fast(2)).orbit(1),',
            '  s("~ ~ oh ~ ~ ~ ~ ~").cut(1).decay(.16).gain(.18).orbit(1),',
            '  // Deep FM-warmed sub-bass + subtle stereo Reese mid layer',
            '  note("<[f1 ~ ~ ~ ~ ~ f1 ~] [db1 ~ ~ ~ ~ ~ eb1 ~] [bb0 ~ ~ ~ ~ ~ c1 ~] [c1 ~ ~ ~ ~ ~ g1 ~]>")',
            '    .s("sine").fm(.6).fmh(2).lpf(220)',
            '    .attack(.02).decay(.4).sustain(.85).release(.15).gain(.66).orbit(2),',
            '  note("<f2 db2 bb1 c2>")',
            '    .s("supersaw").unison(4).spread(.7).detune(.2)',
            '    .hpf(110).lpf(380).attack(.15).sustain(.8).release(.3).gain(.22).orbit(2),',
            '  // Lush jazz-inflected liquid pad chords',
            '  chord("<Fm9 DbM7 Bbm9 C7b9>").voicing()',
            '    .s("supersaw").unison(5).spread(.9).detune(.18)',
            '    .lpf(sine.range(750, 2100).slow(8))',
            '    .attack(.25).decay(.5).sustain(.7).release(.8)',
            '    .room(.65).roomsize(5).gain(.34).orbit(3),',
            '  // Sparkling FM electric piano Rhodes licks',
            '  n("<[0 ~ 2 4 ~ 7 4 ~] [~ 4 2 0 ~ ~ 2 4] [0 ~ 3 5 ~ 7 9 ~] [~ 7 5 4 ~ 2 0 ~]>")',
            '    .scale("f4:minor:pentatonic")',
            '    .s("sine").fm(2.4).fmh(7).fmdecay(.07).fmsustain(0)',
            '    .decay(.45).sustain(.15).release(.6)',
            '    .delay(.45).delaysync(3/16).delayfeedback(.48)',
            '    .room(.55).roomsize(4).pan(sine.range(.25, .75).slow(5))',
            '    .gain(.36).orbit(4)',
            ')'
        ].join('\n'),
        ambient: [
            '// Solar Drift — 68 BPM Deep Space Ambient',
            'setcpm(68/4)',
            'stack(',
            '  // Slow evolving supersaw modal pad',
            '  chord("<CM9 Am9 FM7#11 G69>").voicing().slow(2)',
            '    .s("supersaw").unison(6).spread(.95).detune(.24)',
            '    .lpf(sine.range(420, 1650).slow(16))',
            '    .attack(1.4).decay(1.2).sustain(.8).release(2.2)',
            '    .room(.8).roomsize(7).roomlp(3200).gain(.34).orbit(1),',
            '  // Additive harmonic organ/choir shimmer with slow phaser',
            '  note("<[c3,g3,b3] [a2,e3,b3] [f2,c3,e3] [g2,d3,a3]>").slow(2)',
            '    .s("user").partials([1, .5, .35, .15, .25, .08, .12, .05])',
            '    .lpf(perlin.range(600, 2000).slow(12))',
            '    .attack(1.8).release(2.4).phaser(.18).phaserdepth(.6)',
            '    .room(.85).roomsize(8).gain(.28).orbit(2),',
            '  // Deep sub-bass horizon drone',
            '  note("<c2 a1 f1 g1>").slow(2)',
            '    .s("sine").attack(.9).sustain(.85).release(1.8)',
            '    .lpf(160).gain(.42).orbit(3),',
            '  // Generative FM glass bells drifting across stereo',
            '  n("<[0 ~ 4 ~ 7 ~ 9 ~] [~ 7 ~ 4 ~ 2 ~ 4] [0 ~ 2 ~ 6 ~ 7 ~] [~ 9 ~ 7 ~ 4 ~ 2]>")',
            '    .scale("c5:lydian")',
            '    .s("sine").fm(2.2).fmh(3.5).fmdecay(.18).fmsustain(0)',
            '    .attack(.01).decay(.9).sustain(0).release(1.4)',
            '    .delay(.55).delaysync(3/8).delayfeedback(.58)',
            '    .room(.85).roomsize(7).pan(perlin.range(.15, .85))',
            '    .sometimesBy(.3, x => x.add(note(12))).gain(.26).orbit(4),',
            '  // Soft filtered pink-noise cosmic breeze',
            '  s("pink").slow(2).lpf(sine.range(300, 900).slow(14)).hpf(180)',
            '    .attack(2).release(2).room(.7).roomsize(6).gain(.05).orbit(5)',
            ')'
        ].join('\n'),
        dub_techno: [
            '// Subterranean Echo — 124 BPM Berlin Dub Techno',
            'setcpm(124/4)',
            'stack(',
            '  // Deep muted sub-kick & hypnotic minimal percussion',
            '  s("bd*4").lpf(180).pdecay(.05).penv(24).gain(.65).orbit(1),',
            '  s("[~ hh]*4").hpf(7000).decay(.04).gain(.2).orbit(1),',
            '  s("hh*16").hpf(8500).decay(.02).gain(".06 .14 .08 .18")',
            '    .pan(sine.range(.35, .65).slow(4)).orbit(1),',
            '  s("rim(3,16,2)").delay(.35).delaysync(3/16).delayfeedback(.42)',
            '    .pan(.35).gain(.22).orbit(1),',
            '  // Rolling subterranean sub-bass',
            '  note("~ c2 ~ [c2 eb2] ~ c2 ~ [g1 bb1]")',
            '    .s("triangle").lpf(260).decay(.22).sustain(.25).gain(.56).orbit(2),',
            '  // Iconic phaser + regenerating dotted-eighth dub chord stabs',
            '  chord("<Cm9 ~ Cm9 ~ [~ Cm9] ~ Fm9 ~>").voicing()',
            '    .s("sawtooth").lpf(sine.range(450, 2200).slow(12)).lpq(5)',
            '    .lpenv(3.2).lpdecay(.16).decay(.2).sustain(0)',
            '    .phaser(.35).phaserdepth(.7)',
            '    .delay(.62).delaysync(3/16).delayfeedback(.68)',
            '    .room(.65).roomsize(5.5).gain(.38).orbit(3),',
            '  // Ghostly stereo pulse counter-echoes',
            '  note("<[~ g4 ~ ~ ~ eb5 ~ ~] [~ ~ ~ bb4 ~ ~ c5 ~]>")',
            '    .s("pulse").pw(.35).lpf(perlin.range(600, 2400).slow(8))',
            '    .decay(.14).sustain(0).delay(.55).delaysync(5/16).delayfeedback(.6)',
            '    .room(.6).roomsize(5).pan(sine.range(.2, .8).slow(6)).gain(.11).orbit(4),',
            '  // Sweeping bandpass noise atmosphere',
            '  s("pink").slow(4).bpf(sine.range(500, 3200).slow(16)).bpq(3.5)',
            '    .attack(1.2).release(1.5).room(.8).roomsize(6).gain(.09).orbit(5)',
            ')'
        ].join('\n'),
        melody: [
            '// Glass Garden — 108 BPM Melodic Electronica',
            'setcpm(108/4)',
            'stack(',
            '  // Gentle organic kick, Euclidean rim & crisp hats',
            '  s("bd ~ ~ [~ bd] ~ ~ bd ~").pdecay(.05).gain(.44).orbit(1),',
            '  s("rim(5,16)").hpf(900).pan(.62).room(.3).roomsize(2).gain(.22).orbit(1),',
            '  s("hh*8").hpf(8000).decay(.025).gain(".1 .18 .12 .2").orbit(1),',
            '  // Warm band-limited cello walking bass',
            '  note("<[d2 ~ d2 a1] [bb1 ~ bb1 f1] [f2 ~ f2 c2] [c2 ~ e2 g2]>")',
            '    .s("sawtooth").partials(10).lpf(520).lpenv(1.2).lpdecay(.25)',
            '    .attack(.02).decay(.35).sustain(.55).release(.25).gain(.48).orbit(2),',
            '  // Silky supersaw harmonic cushion',
            '  chord("<Dm9 BbM7 FM7 C69>").voicing()',
            '    .s("supersaw").unison(4).spread(.75).detune(.14).lpf(1350)',
            '    .attack(.3).decay(.5).sustain(.65).release(.7)',
            '    .room(.55).roomsize(4).gain(.3).orbit(3),',
            '  // Expressive FM kalimba/marimba main motif',
            '  n("<[0 2 4 7 ~ 4 2 0] [4 5 7 9 ~ 7 5 4] [2 4 7 5 ~ 4 2 0] [~ 0 2 4 7 5 4 2]>")',
            '    .scale("d4:minor")',
            '    .s("triangle").fm(1.8).fmh(3).fmdecay(.08).fmsustain(0)',
            '    .decay(.32).sustain(.08).release(.35)',
            '    .delay(.38).delaysync(3/16).delayfeedback(.42)',
            '    .room(.48).roomsize(3.5).gain(.48).orbit(4),',
            '  // Stereo-reversed shimmering counter-melody',
            '  n("<[4 ~ 7 ~ 9 ~ 7 ~] [7 ~ 9 ~ 11 ~ 9 ~] [5 ~ 7 ~ 9 ~ 7 ~] [4 ~ 5 ~ 7 ~ 4 ~]>")',
            '    .scale("d4:minor")',
            '    .s("sine").attack(.04).decay(.4).sustain(.1).release(.6)',
            '    .jux(rev).room(.6).roomsize(4.5).gain(.24).orbit(5)',
            ')'
        ].join('\n'),
        groove: [
            '// Velvet Funk — 118 BPM Nu-Disco & Electro-Funk',
            'setcpm(118/4)',
            'stack(',
            '  // Tight disco pocket with 808 cowbell & handclap',
            '  s("bd*4").pdecay(.055).gain(.58).orbit(1),',
            '  s("~ [sd,cp] ~ [sd,cp]").room(.22).roomsize(1.4).gain(.46).orbit(1),',
            '  s("[hh oh]*4").cut(1).hpf(7000).decay(".035 .16").gain(".2 .26").orbit(1),',
            '  s("~ cb ~ ~ ~ ~ cb [~ rim]").pan(.65).gain(.2).orbit(1),',
            '  // Slapped 24dB-filtered octave funk bassline',
            '  note("<[e2 ~ e2 e3 ~ d2 e2 g2] [a1 ~ a1 a2 ~ g1 a1 b1] [c2 ~ c2 c3 ~ b1 c2 d2] [b1 ~ b1 b2 ~ a1 fs2 ds2]>")',
            '    .s("sawtooth").ftype("24db").lpf(420).lpq(5.5)',
            '    .lpenv(3.6).lpdecay(.11).lpsustain(0).decay(.18).sustain(.15)',
            '    .distort(.3).gain(.52).orbit(2),',
            '  // Talk-box vowel funk clavinet stabs',
            '  note("<[~ [e4,g4,b4] ~ [e4,g4,b4] ~ ~ [d4,fs4,a4] ~] [~ [e4,g4,cs5] ~ [e4,g4,cs5] ~ ~ [d4,g4,b4] ~] [~ [e4,g4,b4] ~ [e4,g4,b4] ~ ~ [d4,g4,b4] ~] [~ [ds4,a4,d5] ~ [ds4,a4,d5] ~ ~ [e4,g4,b4] ~]>")',
            '    .s("pulse").pw(.2).vowel("<a o e i>")',
            '    .lpf(2800).decay(.11).sustain(0).pan(.35)',
            '    .delay(.25).delaysync(3/16).delayfeedback(.25).gain(.16).orbit(3),',
            '  // Phaser-swept FM electric piano chords',
            '  chord("<Em7 A9 CM7 B7#9>").voicing()',
            '    .s("triangle").fm(1.6).fmh(4).fmdecay(.08).fmsustain(0)',
            '    .phaser(.5).phaserdepth(.7).attack(.02).decay(.5).sustain(.4).release(.4)',
            '    .room(.38).roomsize(2.5).gain(.44).orbit(4),',
            '  // Funky call-and-response square lead',
            '  note("<[~ ~ ~ ~ e5 g5 a5 b5] [a5 ~ g5 e5 ~ ~ ~ ~] [~ ~ ~ ~ g5 a5 b5 d6] [b5 ~ a5 fs5 ~ ~ ~ ~]>")',
            '    .s("square").lpf(2200).lpenv(2).lpdecay(.12)',
            '    .vib(6).vibmod(.2).decay(.22).sustain(.45).release(.25)',
            '    .delay(.32).delaysync(3/16).delayfeedback(.35).pan(.62).gain(.28).orbit(5)',
            ')'
        ].join('\n'),
        chiptune: [
            '// Starlight Overdrive — 144 BPM 8-Bit Arcade Chiptune',
            'setcpm(144/4)',
            'stack(',
            '  // Bit-crushed 8-bit arcade drums with 4-bar hi-hat rush',
            '  s("bd ~ [~ bd] ~ bd ~ ~ [~ bd]").pdecay(.04).penv(36).crush(6).gain(.52).orbit(1),',
            '  s("~ sd ~ sd").crush(5).decay(.11).gain(.44).orbit(1),',
            '  s("hh*8").crush(6).hpf(6500).decay(.025).gain(".14 .22").every(4, x => x.fast(2)).orbit(1),',
            '  // Punchy NES triangle octave bassline',
            '  note("<[c2 c3] [a1 a2] [f1 f2] [g1 g2] [e1 e2] [f1 f2] [d1 d2] [g1 g2]>*4")',
            '    .s("triangle").lpf(1400).decay(.12).sustain(.3).gain(.55).orbit(2),',
            '  // Rapid C64-style 16th-note PWM chord arpeggios',
            '  n("0 1 2 1").fast(4).chord("<C Am F G Em F Dm G7>").voicing()',
            '    .s("pulse").pw(.125).pwrate(3).pwsweep(.12)',
            '    .decay(.07).sustain(0).pan(sine.range(.25, .75).slow(2))',
            '    .gain(.11).orbit(3),',
            '  // Heroic 8-bar arcade square/pulse lead with vibrato',
            '  note("<[c5 e5 g5 c6 ~ b5 g5 e5] [a4 c5 e5 a5 ~ g5 e5 c5] [f4 a4 c5 f5 ~ e5 d5 c5] [g4 b4 d5 g5 ~ f5 d5 b4] [e5 g5 b5 e6 ~ d6 b5 g5] [f5 a5 c6 f6 ~ e6 c6 a5] [d5 f5 a5 d6 ~ c6 b5 a5] [g5 ~ b5 d6 g6 ~ ~ ~]>")',
            '    .s("pulse").pw(.25).vib(6.5).vibmod(.25)',
            '    .decay(.14).sustain(.55).release(.08)',
            '    .delay(.25).delaysync(3/16).delayfeedback(.28).gain(.17).orbit(4)',
            ')'
        ].join('\n')
    };
    const REF_GROUPS = [
        { key: 'strudel_ref_drums', items: [['bd', 's("bd*4")'], ['sd', 's("~ sd ~ sd")'], ['cp', 's("~ cp ~ cp")'], ['hh', 's("hh*8")'], ['oh', 's("~ ~ ~ oh")'], ['rim', 's("rim(3,8)")'], ['cb', 's("~ cb ~ ~")'], ['lt', 'lt'], ['mt', 'mt'], ['ht', 'ht']] },
        { key: 'strudel_ref_synths', items: [['sawtooth', '.s("sawtooth")'], ['supersaw', '.s("supersaw").unison(4)'], ['pulse', '.s("pulse").pw(.3)'], ['triangle', '.s("triangle")'], ['sine', '.s("sine")'], ['square', '.s("square")']] },
        { key: 'strudel_ref_fx', items: [['lpf', '.lpf(1200)'], ['hpf', '.hpf(240)'], ['lpq', '.lpq(6)'], ['room', '.room(.45)'], ['delay', '.delay(.35)'], ['distort', '.distort(.35)'], ['crush', '.crush(6)'], ['phaser', '.phaser(.4)']] }
    ];

    function create(host, options) {
        const { tr, signal } = options;
        let frame = null, request = 0, timer = 0, loading = false, running = false, disposed = false, refOpen = false;
        const el = (tag, text) => { const node = document.createElement(tag); if (text) node.textContent = text; return node; };
        host.className = 'ss-strudel-workbench';
        const tools = el('div'); tools.className = 'ss-strudel-tools';
        const run = el('button', tr('strudel_run')), stopButton = el('button', tr('stop')), select = el('select');
        run.type = stopButton.type = 'button'; run.dataset.strudelRun = ''; stopButton.dataset.strudelStop = '';
        select.setAttribute('aria-label', tr('strudel_examples'));
        select.append(new Option(tr('strudel_examples'), ''));
        for (const key of Object.keys(examples)) select.append(new Option(tr('strudel_' + key), key));
        const importButton = el('button', tr('strudel_open')), download = el('button', tr('strudel_download'));
        importButton.type = download.type = 'button';
        const refToggle = el('button', tr('strudel_ref'));
        refToggle.type = 'button'; refToggle.dataset.strudelRefToggle = ''; refToggle.setAttribute('aria-expanded', 'false');
        const file = el('input'); file.type = 'file'; file.accept = '.strudel,.txt,text/plain'; file.hidden = true;
        const guide = el('a', tr('strudel_guide')); guide.href = 'https://strudel.cc/workshop/getting-started/'; guide.target = '_blank'; guide.rel = 'noopener noreferrer';
        const source = el('a', tr('strudel_source')); source.href = '/js/vendor/strudel/sources.zip'; source.download = 'strudel-1.3.0-sources.zip';
        tools.append(run, stopButton);
        const code = el('textarea'); code.dataset.strudelCode = ''; code.spellcheck = false; code.maxLength = LIMIT; code.setAttribute('aria-label', tr('strudel_code'));
        code.value = examples.synthwave;
        try { const saved = localStorage.getItem(KEY); if (fits(saved)) code.value = saved; } catch (_) {}
        const hint = el('p', tr('strudel_hint')), status = el('output', tr('strudel_idle'));
        status.dataset.strudelState = 'idle'; status.setAttribute('role', 'status');
        // Toolbar: transport, status, examples, file actions, quick reference.
        tools.append(status, select, importButton, download, refToggle, guide, source);

        const main = el('div'); main.className = 'ss-strudel-main';
        const editor = el('div'), gutter = el('div');
        editor.className = 'ss-strudel-editor'; gutter.className = 'ss-strudel-gutter'; gutter.setAttribute('aria-hidden', 'true');
        code.wrap = 'off'; hint.className = 'ss-strudel-hint';
        const lines = () => {
            const count = code.value.split('\n').length;
            if (gutter.dataset.lines !== String(count)) { gutter.dataset.lines = String(count); gutter.textContent = Array.from({ length: count }, (_, i) => i + 1).join('\n'); }
            gutter.scrollTop = code.scrollTop;
        };
        editor.append(gutter, code);

        const refPane = el('aside'); refPane.className = 'ss-strudel-ref'; refPane.hidden = true;
        refPane.setAttribute('aria-label', tr('strudel_ref'));
        for (const group of REF_GROUPS) {
            const sec = el('div'); sec.className = 'ss-strudel-ref-group';
            const heading = el('strong', tr(group.key));
            const chips = el('div'); chips.className = 'ss-strudel-ref-chips';
            for (const [label, snippet] of group.items) {
                const chip = el('button', label);
                chip.type = 'button';
                chip.dataset.strudelSnippet = snippet;
                chip.title = snippet;
                chips.append(chip);
            }
            sec.append(heading, chips);
            refPane.append(sec);
        }

        main.append(editor, refPane);
        host.append(tools, main, hint, file);
        const message = (key, detail = '') => { status.dataset.strudelState = key; status.textContent = tr('strudel_' + key) + (detail ? ': ' + detail : ''); };
        const persist = () => { lines(); if (!fits(code.value)) { message('error'); return; } try { localStorage.setItem(KEY, code.value); } catch (_) { message('draft_error'); } };
        function refresh() {
            run.disabled = disposed || loading || options.blocked();
            code.readOnly = options.readonly();
            select.disabled = importButton.disabled = options.readonly();
            refPane.querySelectorAll('button').forEach(b => { b.disabled = options.readonly(); });
        }
        function stop() {
            request++; clearTimeout(timer); frame?.remove(); frame = null; loading = running = false;
            if (!disposed) { message('idle'); refresh(); }
        }
        function evaluate() {
            if (disposed || options.blocked() || loading || !code.value.trim()) return;
            if (!fits(code.value)) { message('error'); return; }
            options.beforePlay(); persist(); loading = true; message('loading'); refresh();
            const current = ++request;
            const send = () => frame?.contentWindow?.postMessage({ strudel: 1, type: 'evaluate', request: current, code: code.value, soundLabel: tr('strudel_sound') }, '*');
            clearTimeout(timer);
            timer = setTimeout(() => { stop(); message('error'); }, 15000);
            if (frame) { send(); return; }
            frame = el('iframe'); frame.title = tr('strudel_sound'); frame.sandbox = 'allow-scripts'; frame.allow = 'autoplay'; frame.referrerPolicy = 'no-referrer'; frame.hidden = true;
            frame.src = '/api/desktop/synth-studio/strudel';
            host.insertBefore(frame, main);
            // Ready comes from the initialized engine, rather than the document load event.
            frame._send = send;
        }
        window.addEventListener('message', event => {
            const data = event.data;
            if (!frame || event.source !== frame.contentWindow || event.origin !== 'null' || data?.strudel !== 1) return;
            if (data.type === 'ready') { const send = frame._send; frame._send = null; if (loading) send?.(); return; }
            if (data.request !== request || !['playing', 'sound', 'error'].includes(data.type)) return;
            clearTimeout(timer); loading = false;
            if (data.type === 'error') { const detail = typeof data.message === 'string' ? data.message.slice(0, 500) : ''; stop(); message('error', detail); }
            else { running = true; frame.hidden = data.type !== 'sound'; message(data.type); refresh(); }
        }, { signal });
        run.addEventListener('click', evaluate, { signal });
        stopButton.addEventListener('click', stop, { signal });
        refToggle.addEventListener('click', () => {
            refOpen = !refOpen;
            refPane.hidden = !refOpen;
            refToggle.setAttribute('aria-expanded', String(refOpen));
        }, { signal });
        refPane.addEventListener('click', event => {
            const btn = event.target.closest('[data-strudel-snippet]');
            if (!btn || options.readonly()) return;
            const snippet = btn.dataset.strudelSnippet;
            const start = code.selectionStart ?? code.value.length;
            const end = code.selectionEnd ?? start;
            const next = code.value.slice(0, start) + snippet + code.value.slice(end);
            if (!fits(next)) { message('error'); return; }
            code.value = next;
            code.focus();
            code.setSelectionRange(start + snippet.length, start + snippet.length);
            persist();
        }, { signal });
        code.addEventListener('input', persist, { signal });
        code.addEventListener('keydown', event => { if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') { event.preventDefault(); evaluate(); } }, { signal });
        select.addEventListener('change', async () => {
            const value = examples[select.value], initial = code.value; select.value = '';
            if (!value || options.readonly()) return;
            const isPreset = !initial.trim() || Object.values(examples).includes(initial);
            if (initial !== value && !isPreset && !await options.confirm()) return;
            if (disposed || options.readonly() || code.value !== initial) return;
            code.value = value; persist();
            if (running) evaluate();
        }, { signal });
        importButton.addEventListener('click', () => file.click(), { signal });
        file.addEventListener('change', async () => {
            const selected = file.files[0]; file.value = '';
            if (!selected || options.readonly()) return;
            if (selected.size > LIMIT) { message('error'); return; }
            const generation = request, initial = code.value;
            try { const value = await selected.text(); if (disposed || options.readonly() || generation !== request || !await options.confirm()) return; if (disposed || options.readonly() || generation !== request || code.value !== initial) return; stop(); code.value = value; persist(); }
            catch (_) { if (!disposed) message('error'); }
        }, { signal });
        download.addEventListener('click', () => {
            const url = URL.createObjectURL(new Blob([code.value], { type: 'text/plain;charset=utf-8' })), link = el('a');
            link.href = url; link.download = 'Synth Studio.strudel'; link.click(); setTimeout(() => URL.revokeObjectURL(url), 1000);
        }, { signal });
        signal.addEventListener('abort', () => { disposed = true; stop(); }, { once: true });
        code.addEventListener('scroll', () => { gutter.scrollTop = code.scrollTop; }, { signal });
        lines(); refresh();
        return { stop, refresh, running: () => running };
    }
    window.SynthStudioStrudel = { create, examples };
})();
