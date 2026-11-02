window.ECalcSchematics = Object.assign(window.ECalcSchematics || {}, {
 "ne555_astable": {
  "w": 26, "h": 14,
  "parts": [
   {"sym": "ic", "x": 8, "y": 1, "w": 5, "h": 7, "text": "NE555", "pins": [
    {"n": "8 VCC", "s": "r", "y": 1}, {"n": "4 RST", "s": "r", "y": 2}, {"n": "7 DIS", "s": "r", "y": 4}, {"n": "6 THR", "s": "r", "y": 5}, {"n": "2 TRG", "s": "r", "y": 6},
    {"n": "3 OUT", "s": "l", "y": 2}, {"n": "1 GND", "s": "l", "y": 6}
   ]},
   {"sym": "resistor", "x": 18, "y": 2, "rot": 90},
   {"sym": "resistor", "x": 18, "y": 5, "rot": 90},
   {"sym": "capacitor", "x": 18, "y": 9, "rot": 90},
   {"sym": "ground", "x": 18, "y": 12},
   {"sym": "ground", "x": 8, "y": 7}
  ],
  "wires": [[[15, 2], [22, 2]], [[15, 3], [17, 3], [17, 2]], [[15, 5], [18, 5]], [[15, 6], [17, 6], [17, 8], [18, 8]], [[15, 7], [17, 7]], [[18, 8], [18, 9]], [[8, 3], [4, 3]]],
  "dots": [[18, 2], [17, 2], [18, 5], [17, 7], [18, 8]],
  "terminals": [[22, 2], [4, 3]],
  "texts": [
   {"x": 22.8, "y": 1.65, "text": "V_CC", "anchor": "start"},
   {"x": 3.4, "y": 3.35, "text": "V_out", "anchor": "end"}
  ],
  "anchors": [
   {"var": "r1", "x": 18.6, "y": 3.7, "side": "right"},
   {"var": "r2", "x": 18.6, "y": 6.9, "side": "right"},
   {"var": "c", "x": 18.6, "y": 10.8, "side": "right"}
  ]
 },
 "ne555_monostable": {
  "w": 26, "h": 14,
  "parts": [
   {"sym": "ic", "x": 8, "y": 1, "w": 5, "h": 7, "text": "NE555", "pins": [
    {"n": "8 VCC", "s": "r", "y": 1}, {"n": "4 RST", "s": "r", "y": 2}, {"n": "7 DIS", "s": "r", "y": 4}, {"n": "6 THR", "s": "r", "y": 5},
    {"n": "2 TRG", "s": "l", "y": 1}, {"n": "3 OUT", "s": "l", "y": 3}, {"n": "1 GND", "s": "l", "y": 6}
   ]},
   {"sym": "resistor", "x": 18, "y": 2, "rot": 90},
   {"sym": "capacitor", "x": 18, "y": 9, "rot": 90},
   {"sym": "ground", "x": 18, "y": 12},
   {"sym": "ground", "x": 8, "y": 7}
  ],
  "wires": [[[15, 2], [22, 2]], [[15, 3], [17, 3], [17, 2]], [[15, 5], [18, 5]], [[18, 5], [18, 9]], [[15, 6], [17, 6], [17, 9], [18, 9]], [[8, 2], [4, 2]], [[8, 4], [4, 4]]],
  "dots": [[18, 2], [17, 2], [18, 5], [18, 9]],
  "terminals": [[22, 2], [4, 2], [4, 4]],
  "texts": [
   {"x": 22.8, "y": 1.65, "text": "V_CC", "anchor": "start"},
   {"x": 3.4, "y": 2.35, "text": "V_trig", "anchor": "end"},
   {"x": 3.4, "y": 4.35, "text": "V_out", "anchor": "end"}
  ],
  "anchors": [
   {"var": "r", "x": 18.6, "y": 3.7, "side": "right"},
   {"var": "c", "x": 18.6, "y": 10.8, "side": "right"}
  ]
 },
 "crystal_load_caps": {
  "w": 24, "h": 12,
  "parts": [
   {"sym": "crystal", "x": 12, "y": 4},
   {"sym": "capacitor", "x": 9, "y": 4, "rot": 90},
   {"sym": "capacitor", "x": 18, "y": 4, "rot": 90},
   {"sym": "ground", "x": 13, "y": 10}
  ],
  "wires": [[[7, 4], [12, 4]], [[15, 4], [20, 4]], [[9, 7], [9, 10], [18, 10]], [[18, 7], [18, 10]]],
  "dots": [[9, 4], [18, 4], [13, 10]],
  "terminals": [[7, 4], [20, 4]],
  "texts": [
   {"x": 6.4, "y": 4.35, "text": "XIN", "anchor": "end"},
   {"x": 20.8, "y": 4.35, "text": "XOUT", "anchor": "start"},
   {"x": 18.7, "y": 5.85, "text": "C", "anchor": "start"}
  ],
  "anchors": [
   {"var": "c", "x": 8.4, "y": 6, "side": "left"}
  ]
 }
});
