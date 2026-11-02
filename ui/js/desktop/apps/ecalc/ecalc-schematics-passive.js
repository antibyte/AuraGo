window.ECalcSchematics = Object.assign(window.ECalcSchematics || {}, {
 "reactance_c": {
  "w": 27, "h": 11,
  "parts": [
   {"sym": "acsource", "x": 8, "y": 5, "rot": 90},
   {"sym": "capacitor", "x": 19, "y": 5, "rot": 90}
  ],
  "wires": [[[8, 5], [8, 4], [19, 4], [19, 5]], [[8, 8], [8, 9], [19, 9], [19, 8]]],
  "anchors": [
   {"var": "f", "x": 7.4, "y": 6.5, "side": "left"},
   {"var": "c", "x": 19, "y": 3.6, "side": "top"},
   {"var": "xc", "x": 19.9, "y": 6.5, "side": "right"}
  ]
 },
 "reactance_l": {
  "w": 27, "h": 11,
  "parts": [
   {"sym": "acsource", "x": 8, "y": 5, "rot": 90},
   {"sym": "inductor", "x": 19, "y": 5, "rot": 90}
  ],
  "wires": [[[8, 5], [8, 4], [19, 4], [19, 5]], [[8, 8], [8, 9], [19, 9], [19, 8]]],
  "anchors": [
   {"var": "f", "x": 7.4, "y": 6.5, "side": "left"},
   {"var": "l", "x": 19, "y": 3.6, "side": "top"},
   {"var": "xl", "x": 19.9, "y": 6.5, "side": "right"}
  ]
 },
 "impedance_series": {
  "w": 29, "h": 11,
  "parts": [
   {"sym": "acsource", "x": 8, "y": 5, "rot": 90},
   {"sym": "resistor", "x": 11, "y": 4},
   {"sym": "inductor", "x": 16, "y": 4},
   {"sym": "capacitor", "x": 21, "y": 4}
  ],
  "wires": [[[8, 5], [8, 4], [11, 4]], [[14, 4], [16, 4]], [[19, 4], [21, 4]], [[24, 4], [27, 4], [27, 9], [8, 9], [8, 8]]],
  "anchors": [
   {"var": "f", "x": 7.4, "y": 6.5, "side": "left"},
   {"var": "r", "x": 12.5, "y": 3.3, "side": "top"},
   {"var": "l", "x": 17.5, "y": 4.7, "side": "bottom"},
   {"var": "c", "x": 22.5, "y": 3.3, "side": "top"}
  ]
 },
 "impedance_parallel": {
  "w": 32, "h": 11,
  "parts": [
   {"sym": "acsource", "x": 8, "y": 6, "rot": 90},
   {"sym": "resistor", "x": 14, "y": 6, "rot": 90},
   {"sym": "inductor", "x": 21, "y": 6, "rot": 90},
   {"sym": "capacitor", "x": 28, "y": 6, "rot": 90}
  ],
  "wires": [[[8, 6], [8, 5], [28, 5], [28, 6]], [[8, 9], [8, 10], [28, 10], [28, 9]], [[14, 5], [14, 6]], [[14, 9], [14, 10]], [[21, 5], [21, 6]], [[21, 9], [21, 10]]],
  "dots": [[14, 5], [21, 5], [14, 10], [21, 10]],
  "anchors": [
   {"var": "f", "x": 7.4, "y": 7.5, "side": "left"},
   {"var": "r", "x": 14, "y": 4.7, "side": "top"},
   {"var": "l", "x": 21, "y": 4.7, "side": "top"},
   {"var": "c", "x": 28, "y": 4.7, "side": "top"}
  ]
 },
 "rc_time_constant": {
  "w": 31, "h": 11,
  "parts": [
   {"sym": "source", "x": 8, "y": 5, "rot": 90},
   {"sym": "switch", "x": 11, "y": 4},
   {"sym": "resistor", "x": 16, "y": 4},
   {"sym": "capacitor", "x": 23, "y": 5, "rot": 90}
  ],
  "wires": [[[8, 5], [8, 4], [11, 4]], [[14, 4], [16, 4]], [[19, 4], [23, 4], [23, 5]], [[23, 8], [23, 9], [8, 9], [8, 8]]],
  "anchors": [
   {"var": "v0", "x": 7.4, "y": 6.5, "side": "left"},
   {"var": "t", "x": 12.5, "y": 4.7, "side": "bottom"},
   {"var": "r", "x": 17.5, "y": 3.3, "side": "top"},
   {"var": "c", "x": 23.9, "y": 5, "side": "right"},
   {"var": "vc", "x": 23.9, "y": 8, "side": "right"}
  ]
 },
 "capacitors_series_parallel": {
  "w": 23, "h": 14,
  "parts": [
   {"sym": "capacitor", "x": 5, "y": 5},
   {"sym": "capacitor", "x": 10, "y": 5},
   {"sym": "capacitor", "x": 15, "y": 5},
   {"sym": "capacitor", "x": 9, "y": 9},
   {"sym": "capacitor", "x": 9, "y": 11},
   {"sym": "capacitor", "x": 9, "y": 13}
  ],
  "wires": [[[3, 5], [5, 5]], [[8, 5], [10, 5]], [[13, 5], [15, 5]], [[18, 5], [20, 5]], [[5, 11], [7, 11]], [[7, 9], [7, 13]], [[7, 9], [9, 9]], [[7, 13], [9, 13]], [[7, 11], [9, 11]], [[12, 9], [14, 9]], [[12, 13], [14, 13]], [[12, 11], [14, 11]], [[14, 9], [14, 13]], [[14, 11], [16, 11]]],
  "dots": [[7, 11], [14, 11]],
  "terminals": [[3, 5], [20, 5], [5, 11], [16, 11]],
  "texts": [
   {"x": 20.8, "y": 5.35, "text": "C_S", "anchor": "start"},
   {"x": 16.8, "y": 11.35, "text": "C_P", "anchor": "start"}
  ],
  "anchors": [
   {"var": "cs", "x": 11.5, "y": 3.6, "side": "top"}
  ]
 },
 "inductors_series_parallel": {
  "w": 23, "h": 14,
  "parts": [
   {"sym": "inductor", "x": 5, "y": 5},
   {"sym": "inductor", "x": 10, "y": 5},
   {"sym": "inductor", "x": 15, "y": 5},
   {"sym": "inductor", "x": 9, "y": 9},
   {"sym": "inductor", "x": 9, "y": 11},
   {"sym": "inductor", "x": 9, "y": 13}
  ],
  "wires": [[[3, 5], [5, 5]], [[8, 5], [10, 5]], [[13, 5], [15, 5]], [[18, 5], [20, 5]], [[5, 11], [7, 11]], [[7, 9], [7, 13]], [[7, 9], [9, 9]], [[7, 13], [9, 13]], [[7, 11], [9, 11]], [[12, 9], [14, 9]], [[12, 13], [14, 13]], [[12, 11], [14, 11]], [[14, 9], [14, 13]], [[14, 11], [16, 11]]],
  "dots": [[7, 11], [14, 11]],
  "terminals": [[3, 5], [20, 5], [5, 11], [16, 11]],
  "texts": [
   {"x": 20.8, "y": 5.35, "text": "L_S", "anchor": "start"},
   {"x": 16.8, "y": 11.35, "text": "L_P", "anchor": "start"}
  ],
  "anchors": [
   {"var": "ls", "x": 11.5, "y": 3.6, "side": "top"}
  ]
 },
 "capacitor_energy": {
  "w": 27, "h": 11,
  "parts": [
   {"sym": "source", "x": 8, "y": 5, "rot": 90},
   {"sym": "capacitor", "x": 19, "y": 5, "rot": 90}
  ],
  "wires": [[[8, 5], [8, 4], [19, 4], [19, 5]], [[8, 8], [8, 9], [19, 9], [19, 8]]],
  "anchors": [
   {"var": "u", "x": 7.4, "y": 6.5, "side": "left"},
   {"var": "c", "x": 19.9, "y": 6.5, "side": "right"}
  ]
 }
});
