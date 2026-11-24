window.ECalcSchematics = Object.assign(window.ECalcSchematics || {}, {
 "rc_lowpass": {
  "w": 27, "h": 12,
  "parts": [
   {"sym": "acsource", "x": 8, "y": 4, "rot": 90},
   {"sym": "resistor", "x": 12, "y": 4},
   {"sym": "capacitor", "x": 19, "y": 4, "rot": 90},
   {"sym": "ground", "x": 13, "y": 10}
  ],
  "wires": [[[8, 4], [12, 4]], [[15, 4], [24, 4]], [[8, 7], [8, 10], [24, 10]], [[19, 7], [19, 10]]],
  "dots": [[19, 4], [19, 10], [13, 10]],
  "terminals": [[24, 4], [24, 10]],
  "texts": [
   {"x": 24.8, "y": 7.35, "text": "V_out", "anchor": "start"}
  ],
  "anchors": [
   {"var": "f", "x": 7.4, "y": 5.5, "side": "left"},
   {"var": "r", "x": 13.5, "y": 3.3, "side": "top"},
   {"var": "c", "x": 18.4, "y": 5.8, "side": "left"}
  ]
 },
 "rc_highpass": {
  "w": 27, "h": 12,
  "parts": [
   {"sym": "acsource", "x": 8, "y": 4, "rot": 90},
   {"sym": "capacitor", "x": 12, "y": 4},
   {"sym": "resistor", "x": 19, "y": 4, "rot": 90},
   {"sym": "ground", "x": 13, "y": 10}
  ],
  "wires": [[[8, 4], [12, 4]], [[15, 4], [24, 4]], [[8, 7], [8, 10], [24, 10]], [[19, 7], [19, 10]]],
  "dots": [[19, 4], [19, 10], [13, 10]],
  "terminals": [[24, 4], [24, 10]],
  "texts": [
   {"x": 24.8, "y": 7.35, "text": "V_out", "anchor": "start"}
  ],
  "anchors": [
   {"var": "f", "x": 7.4, "y": 5.5, "side": "left"},
   {"var": "c", "x": 13.5, "y": 3.3, "side": "top"},
   {"var": "r", "x": 18.4, "y": 6.2, "side": "left"}
  ]
 },
 "lc_resonance": {
  "w": 29, "h": 12,
  "parts": [
   {"sym": "acsource", "x": 8, "y": 4, "rot": 90},
   {"sym": "inductor", "x": 16, "y": 4, "rot": 90},
   {"sym": "capacitor", "x": 21, "y": 4, "rot": 90},
   {"sym": "ground", "x": 12, "y": 10}
  ],
  "wires": [[[8, 4], [21, 4]], [[8, 7], [8, 10], [21, 10]], [[16, 7], [16, 10]], [[21, 7], [21, 10]]],
  "dots": [[16, 4], [16, 10], [12, 10]],
  "anchors": [
   {"var": "l", "x": 15.4, "y": 5.8, "side": "left"},
   {"var": "c", "x": 21.6, "y": 5.8, "side": "right"}
  ]
 },
 "rlc_q_bandwidth": {
  "w": 27, "h": 20,
  "parts": [
   {"sym": "acsource", "x": 8, "y": 4, "rot": 90},
   {"sym": "resistor", "x": 11, "y": 4},
   {"sym": "inductor", "x": 16, "y": 4},
   {"sym": "capacitor", "x": 21, "y": 4},
   {"sym": "acsource", "x": 8, "y": 14, "rot": 90},
   {"sym": "resistor", "x": 13, "y": 14, "rot": 90},
   {"sym": "inductor", "x": 18, "y": 14, "rot": 90},
   {"sym": "capacitor", "x": 23, "y": 14, "rot": 90}
  ],
  "wires": [[[8, 4], [11, 4]], [[14, 4], [16, 4]], [[19, 4], [21, 4]], [[24, 4], [26, 4], [26, 9], [8, 9], [8, 7]], [[8, 14], [23, 14]], [[8, 17], [8, 18], [23, 18]], [[13, 17], [13, 18]], [[18, 17], [18, 18]], [[23, 17], [23, 18]]],
  "dots": [[13, 14], [18, 14], [13, 18], [18, 18]],
  "texts": [
   {"x": 13.8, "y": 15.85, "text": "R", "anchor": "start"},
   {"x": 18.8, "y": 15.85, "text": "L", "anchor": "start"},
   {"x": 23.8, "y": 15.85, "text": "C", "anchor": "start"}
  ],
  "anchors": [
   {"var": "r", "x": 12.5, "y": 3.3, "side": "top"},
   {"var": "l", "x": 17.5, "y": 4.9, "side": "bottom"},
   {"var": "c", "x": 22.5, "y": 3.3, "side": "top"}
  ]
 },
 "sallen_key_lowpass": {
  "w": 32, "h": 14,
  "parts": [
   {"sym": "acsource", "x": 3, "y": 8, "rot": 90},
   {"sym": "resistor", "x": 6, "y": 8},
   {"sym": "resistor", "x": 13, "y": 8},
   {"sym": "capacitor", "x": 13, "y": 4},
   {"sym": "capacitor", "x": 18, "y": 8, "rot": 90},
   {"sym": "opamp", "x": 20, "y": 9, "flip": true},
   {"sym": "ground", "x": 3, "y": 11},
   {"sym": "ground", "x": 18, "y": 11}
  ],
  "wires": [[[3, 8], [6, 8]], [[9, 8], [13, 8]], [[11, 8], [11, 4], [13, 4]], [[16, 8], [20, 8]], [[16, 4], [28, 4], [28, 9]], [[25, 9], [29, 9]], [[27, 9], [27, 12], [20, 12], [20, 10]]],
  "dots": [[11, 8], [18, 8], [27, 9], [28, 9]],
  "terminals": [[29, 9]],
  "texts": [
   {"x": 14.5, "y": 7.2, "text": "R", "anchor": "middle"},
   {"x": 29.8, "y": 9.35, "text": "V_out", "anchor": "start"}
  ],
  "anchors": [
   {"var": "r", "x": 7.5, "y": 7.3, "side": "top"},
   {"var": "c1", "x": 14.5, "y": 3.3, "side": "top"},
   {"var": "c2", "x": 17.4, "y": 10, "side": "left"}
  ]
 },
 "twin_t_notch": {
  "w": 25, "h": 17,
  "parts": [
   {"sym": "resistor", "x": 8, "y": 4},
   {"sym": "resistor", "x": 13, "y": 4},
   {"sym": "capacitor", "x": 12, "y": 4, "rot": 90},
   {"sym": "capacitor", "x": 8, "y": 12},
   {"sym": "capacitor", "x": 13, "y": 12},
   {"sym": "resistor", "x": 12, "y": 12, "rot": 270},
   {"sym": "ground", "x": 14, "y": 8}
  ],
  "wires": [[[4, 8], [6, 8]], [[6, 4], [6, 12]], [[6, 4], [8, 4]], [[6, 12], [8, 12]], [[11, 4], [13, 4]], [[11, 12], [13, 12]], [[12, 7], [12, 9]], [[12, 8], [14, 8]], [[16, 4], [18, 4], [18, 12], [16, 12]], [[18, 8], [22, 8]]],
  "dots": [[6, 8], [12, 4], [12, 12], [12, 8], [18, 8]],
  "terminals": [[4, 8], [22, 8]],
  "texts": [
   {"x": 3.4, "y": 8.35, "text": "V_in", "anchor": "end"},
   {"x": 22.8, "y": 8.35, "text": "V_out", "anchor": "start"},
   {"x": 12.8, "y": 5.85, "text": "2C", "anchor": "start"},
   {"x": 12.8, "y": 10.85, "text": "R/2", "anchor": "start"},
   {"x": 14.5, "y": 3.35, "text": "R", "anchor": "middle"},
   {"x": 14.5, "y": 13.6, "text": "C", "anchor": "middle"}
  ],
  "anchors": [
   {"var": "r", "x": 9.5, "y": 3.3, "side": "top"},
   {"var": "c", "x": 9.5, "y": 12.8, "side": "bottom"}
  ]
 },
 "attenuator_pi": {
  "w": 27, "h": 12,
  "parts": [
   {"sym": "resistor", "x": 13, "y": 4},
   {"sym": "resistor", "x": 9, "y": 4, "rot": 90},
   {"sym": "resistor", "x": 20, "y": 4, "rot": 90},
   {"sym": "ground", "x": 14, "y": 10}
  ],
  "wires": [[[4, 4], [13, 4]], [[16, 4], [25, 4]], [[9, 7], [9, 10]], [[20, 7], [20, 10]], [[4, 10], [25, 10]]],
  "dots": [[9, 4], [20, 4], [9, 10], [20, 10], [14, 10]],
  "terminals": [[4, 4], [25, 4], [4, 10], [25, 10]],
  "texts": [
   {"x": 20.8, "y": 5.85, "text": "R_p", "anchor": "start"}
  ],
  "anchors": [
   {"var": "rs", "x": 14.5, "y": 3.3, "side": "top"},
   {"var": "rp", "x": 8.4, "y": 5.8, "side": "left"}
  ]
 },
 "attenuator_t": {
  "w": 26, "h": 12,
  "parts": [
   {"sym": "resistor", "x": 7, "y": 4},
   {"sym": "resistor", "x": 14, "y": 4},
   {"sym": "resistor", "x": 12, "y": 4, "rot": 90},
   {"sym": "ground", "x": 17, "y": 10}
  ],
  "wires": [[[4, 4], [7, 4]], [[10, 4], [14, 4]], [[17, 4], [24, 4]], [[12, 7], [12, 10]], [[4, 10], [24, 10]]],
  "dots": [[12, 4], [12, 10], [17, 10]],
  "terminals": [[4, 4], [24, 4], [4, 10], [24, 10]],
  "texts": [
   {"x": 15.5, "y": 3.35, "text": "R_s", "anchor": "middle"}
  ],
  "anchors": [
   {"var": "rs", "x": 8.5, "y": 3.3, "side": "top"},
   {"var": "rp", "x": 11.4, "y": 6.2, "side": "left"}
  ]
 },
 "sallen_key_highpass": {
  "w": 32, "h": 14,
  "parts": [
   {"sym": "acsource", "x": 3, "y": 8, "rot": 90},
   {"sym": "capacitor", "x": 6, "y": 8},
   {"sym": "capacitor", "x": 13, "y": 8},
   {"sym": "resistor", "x": 13, "y": 4},
   {"sym": "resistor", "x": 18, "y": 8, "rot": 90},
   {"sym": "opamp", "x": 20, "y": 9, "flip": true},
   {"sym": "ground", "x": 3, "y": 11},
   {"sym": "ground", "x": 18, "y": 11}
  ],
  "wires": [[[3, 8], [6, 8]], [[9, 8], [13, 8]], [[11, 8], [11, 4], [13, 4]], [[16, 8], [20, 8]], [[16, 4], [28, 4], [28, 9]], [[25, 9], [29, 9]], [[27, 9], [27, 12], [20, 12], [20, 10]]],
  "dots": [[11, 8], [18, 8], [27, 9], [28, 9]],
  "terminals": [[29, 9]],
  "texts": [
   {"x": 14.5, "y": 7.2, "text": "C", "anchor": "middle"},
   {"x": 29.8, "y": 9.35, "text": "V_out", "anchor": "start"}
  ],
  "anchors": [
   {"var": "c", "x": 7.5, "y": 7.3, "side": "top"},
   {"var": "r1", "x": 14.5, "y": 3.3, "side": "top"},
   {"var": "r2", "x": 17.4, "y": 10, "side": "left"}
  ]
 },
 "mfb_bandpass": {
  "w": 31, "h": 14,
  "parts": [
   {"sym": "resistor", "x": 5, "y": 7},
   {"sym": "capacitor", "x": 14, "y": 7},
   {"sym": "capacitor", "x": 13, "y": 1},
   {"sym": "resistor", "x": 11, "y": 9, "rot": 90},
   {"sym": "resistor", "x": 20, "y": 4},
   {"sym": "opamp", "x": 21, "y": 8},
   {"sym": "ground", "x": 11, "y": 12},
   {"sym": "ground", "x": 19, "y": 9}
  ],
  "wires": [[[3, 7], [5, 7]], [[8, 7], [14, 7]], [[17, 7], [21, 7]], [[11, 7], [11, 1], [13, 1]], [[11, 7], [11, 9]], [[19, 7], [19, 4], [20, 4]], [[16, 1], [28, 1], [28, 4]], [[23, 4], [28, 4], [28, 8]], [[26, 8], [30, 8]], [[19, 9], [21, 9]]],
  "dots": [[11, 7], [19, 7], [28, 4], [28, 8]],
  "terminals": [[3, 7], [30, 8]],
  "texts": [
   {"x": 14.5, "y": 0.2, "text": "C", "anchor": "middle"}
  ],
  "anchors": [
   {"var": "rin", "x": 6.5, "y": 6.3, "side": "top"},
   {"var": "c", "x": 15.5, "y": 6.3, "side": "top"},
   {"var": "rgnd", "x": 10.4, "y": 10.5, "side": "left"},
   {"var": "rfb", "x": 21.5, "y": 3.3, "side": "top"}
  ]
 },
 "speaker_crossover": {
  "w": 25, "h": 18,
  "parts": [
   {"sym": "acsource", "x": 3, "y": 9, "rot": 90},
   {"sym": "inductor", "x": 8, "y": 4},
   {"sym": "capacitor", "x": 14, "y": 4, "rot": 90},
   {"sym": "resistor", "x": 19, "y": 4, "rot": 90},
   {"sym": "capacitor", "x": 8, "y": 13},
   {"sym": "inductor", "x": 14, "y": 13, "rot": 90},
   {"sym": "resistor", "x": 19, "y": 13, "rot": 90},
   {"sym": "ground", "x": 16, "y": 7},
   {"sym": "ground", "x": 16, "y": 16},
   {"sym": "ground", "x": 3, "y": 12}
  ],
  "wires": [[[3, 9], [6, 9]], [[6, 9], [6, 4], [8, 4]], [[6, 9], [6, 13], [8, 13]], [[11, 4], [19, 4]], [[11, 13], [19, 13]], [[14, 7], [19, 7]], [[14, 16], [19, 16]]],
  "dots": [[6, 9], [14, 4], [16, 7], [14, 13], [16, 16]],
  "texts": [
   {"x": 9.5, "y": 12.2, "text": "C", "anchor": "middle"},
   {"x": 14.8, "y": 14.85, "text": "L", "anchor": "start"},
   {"x": 19.8, "y": 14.85, "text": "R", "anchor": "start"}
  ],
  "anchors": [
   {"var": "f", "x": 3, "y": 8.4, "side": "top"},
   {"var": "l", "x": 9.5, "y": 3.3, "side": "top"},
   {"var": "c", "x": 13.4, "y": 5.8, "side": "left"},
   {"var": "r", "x": 19.6, "y": 5.8, "side": "right"}
  ]
 },
 "l_pad": {
  "w": 26, "h": 12,
  "parts": [
   {"sym": "resistor", "x": 7, "y": 4},
   {"sym": "resistor", "x": 14, "y": 4, "rot": 90},
   {"sym": "resistor", "x": 20, "y": 4, "rot": 90},
   {"sym": "ground", "x": 10, "y": 10}
  ],
  "wires": [[[4, 4], [7, 4]], [[10, 4], [20, 4]], [[14, 7], [14, 10]], [[20, 7], [20, 10]], [[4, 10], [20, 10]]],
  "dots": [[14, 4], [14, 10], [10, 10]],
  "terminals": [[4, 4], [4, 10]],
  "anchors": [
   {"var": "rs", "x": 8.5, "y": 3.3, "side": "top"},
   {"var": "rp", "x": 13.4, "y": 6.2, "side": "left"},
   {"var": "z", "x": 20.6, "y": 6.2, "side": "right"}
  ]
 }
});
