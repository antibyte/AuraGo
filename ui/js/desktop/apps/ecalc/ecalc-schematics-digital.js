window.ECalcSchematics = Object.assign(window.ECalcSchematics || {}, {
 "i2c_pullup": {
  "w": 32, "h": 12,
  "parts": [
   {"sym": "resistor", "x": 10, "y": 3, "rot": 90},
   {"sym": "capacitor", "x": 17, "y": 7, "rot": 90},
   {"sym": "nmos", "x": 21, "y": 8},
   {"sym": "ground", "x": 17, "y": 10},
   {"sym": "ground", "x": 24, "y": 10}
  ],
  "wires": [[[10, 1.6], [10, 3]], [[10, 6], [28, 6]], [[17, 6], [17, 7]], [[19, 8], [21, 8]]],
  "dots": [[17, 6], [24, 6]],
  "terminals": [[10, 1.6], [28, 6], [19, 8]],
  "arrows": [
   {"x": 10, "y": 2.3, "dir": 90}
  ],
  "texts": [
   {"x": 28.7, "y": 6.35, "text": "SDA / SCL", "anchor": "start"}
  ],
  "anchors": [
   {"var": "vdd", "x": 10.4, "y": 1.6, "side": "right"},
   {"var": "rmax", "x": 9.8, "y": 4, "side": "left"},
   {"var": "rmin", "x": 9.8, "y": 7.1, "side": "left"},
   {"var": "cb", "x": 16.6, "y": 8.7, "side": "left"},
   {"var": "vol", "x": 20.5, "y": 6, "side": "top"},
   {"var": "iol", "x": 24.6, "y": 8.5, "side": "right"}
  ]
 },
 "level_shift_divider": {
  "w": 32, "h": 12,
  "parts": [
   {"sym": "source", "x": 8, "y": 3, "rot": 90},
   {"sym": "resistor", "x": 19, "y": 2, "rot": 90},
   {"sym": "resistor", "x": 19, "y": 5, "rot": 90},
   {"sym": "ground", "x": 12, "y": 9},
   {"sym": "ic", "x": 26, "y": 4, "w": 3, "h": 4, "text": "3.3 V logic", "pins": [{"n": "IN", "s": "l", "y": 1}, {"n": "GND", "s": "l", "y": 3}]}
  ],
  "wires": [[[8, 3], [8, 2], [19, 2]], [[8, 6], [8, 9], [25, 9]], [[19, 8], [19, 9]], [[19, 5], [26, 5]], [[26, 7], [25, 7], [25, 9]]],
  "dots": [[19, 5], [12, 9], [19, 9], [25, 9]],
  "anchors": [
   {"var": "vin", "x": 7.2, "y": 4.5, "side": "left"},
   {"var": "r1", "x": 18.6, "y": 4, "side": "left"},
   {"var": "r2", "x": 18.6, "y": 7.2, "side": "left"},
   {"var": "vout", "x": 22.8, "y": 5, "side": "top"},
   {"var": "vih", "x": 28.5, "y": 8, "side": "bottom"}
  ]
 }
});
