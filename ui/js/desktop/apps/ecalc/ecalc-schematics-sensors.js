window.ECalcSchematics = Object.assign(window.ECalcSchematics || {}, {
 "ntc_divider": {
  "w": 27, "h": 11,
  "parts": [
   {"sym": "source", "x": 8, "y": 3, "rot": 90},
   {"sym": "resistor", "x": 16, "y": 2, "rot": 90},
   {"sym": "resistor", "x": 16, "y": 5, "rot": 90},
   {"sym": "ground", "x": 12, "y": 9}
  ],
  "wires": [[[8, 3], [8, 2], [16, 2]], [[8, 6], [8, 9], [16, 9]], [[16, 8], [16, 9]], [[16, 5], [19, 5]], [[16, 9], [19, 9]]],
  "dots": [[16, 5], [12, 9], [16, 9]],
  "terminals": [[19, 5], [19, 9]],
  "texts": [
   {"x": 17.4, "y": 6.9, "text": "NTC", "anchor": "start"}
  ],
  "anchors": [
   {"var": "vcc", "x": 7.4, "y": 4.5, "side": "left"},
   {"var": "rfix", "x": 15.6, "y": 3.65, "side": "left"},
   {"var": "rntc", "x": 15.6, "y": 6.6, "side": "left"},
   {"var": "vout", "x": 19.8, "y": 7, "side": "right"}
  ]
 },
 "current_loop_4_20ma": {
  "w": 24, "h": 11,
  "parts": [
   {"sym": "isource", "x": 8, "y": 3, "rot": 90},
   {"sym": "resistor", "x": 16, "y": 2, "rot": 90}
  ],
  "wires": [[[8, 3], [8, 2], [16, 2]], [[16, 5], [16, 9], [8, 9], [8, 6]], [[16, 2], [19, 2]], [[16, 5], [19, 5]]],
  "dots": [[16, 2], [16, 5]],
  "terminals": [[19, 2], [19, 5]],
  "anchors": [
   {"var": "i", "x": 7.4, "y": 4.5, "side": "left"},
   {"var": "r", "x": 15.6, "y": 3.6, "side": "left"},
   {"var": "v", "x": 19.8, "y": 3.5, "side": "right"}
  ]
 },
 "shunt_current_sense": {
  "w": 31, "h": 11,
  "parts": [
   {"sym": "isource", "x": 4, "y": 3, "rot": 90},
   {"sym": "resistor", "x": 12, "y": 2, "rot": 90},
   {"sym": "ground", "x": 8, "y": 9},
   {"sym": "opamp", "x": 19, "y": 6, "flip": true}
  ],
  "wires": [[[4, 3], [4, 2], [12, 2]], [[12, 5], [12, 9], [4, 9], [4, 6]], [[12, 2], [16, 2], [16, 5], [19, 5]], [[12, 5], [14, 5], [14, 7], [19, 7]], [[24, 6], [27, 6]]],
  "dots": [[12, 2], [12, 5], [8, 9]],
  "terminals": [[27, 6]],
  "anchors": [
   {"var": "i", "x": 3.4, "y": 4.5, "side": "left"},
   {"var": "r", "x": 11.6, "y": 3.6, "side": "left"},
   {"var": "v", "x": 17.2, "y": 3.4, "side": "right"},
   {"var": "gain", "x": 21.5, "y": 8.9, "side": "bottom"},
   {"var": "vout", "x": 27.6, "y": 6, "side": "right"}
  ]
 }
});
