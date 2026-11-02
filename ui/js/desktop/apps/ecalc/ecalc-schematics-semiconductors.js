window.ECalcSchematics = Object.assign(window.ECalcSchematics || {}, {
 "led_resistor": {
  "w": 31, "h": 14,
  "parts": [
   {"sym": "source", "x": 9, "y": 6, "rot": 90},
   {"sym": "resistor", "x": 13, "y": 5},
   {"sym": "led", "x": 22, "y": 5, "rot": 90},
   {"sym": "led", "x": 22, "y": 8, "rot": 90},
   {"sym": "ground", "x": 15, "y": 12}
  ],
  "wires": [[[9, 6], [9, 5], [13, 5]], [[16, 5], [22, 5]], [[22, 11], [22, 12], [9, 12], [9, 9]]],
  "dots": [[15, 12]],
  "arrows": [
   {"x": 19.5, "y": 5, "dir": 0}
  ],
  "texts": [
   {"x": 22, "y": 13.4, "text": "× m parallel strings", "anchor": "middle"}
  ],
  "anchors": [
   {"var": "vs", "x": 8.4, "y": 7.5, "side": "left"},
   {"var": "r", "x": 14.5, "y": 4.4, "side": "top"},
   {"var": "m", "x": 23, "y": 4.4, "side": "top"},
   {"var": "i", "x": 17.8, "y": 5.6, "side": "bottom"},
   {"var": "vf", "x": 21.1, "y": 10.4, "side": "left"},
   {"var": "n", "x": 23.8, "y": 8, "side": "right"}
  ]
 },
 "zener_regulator": {
  "w": 32, "h": 14,
  "parts": [
   {"sym": "source", "x": 8, "y": 6, "rot": 90},
   {"sym": "resistor", "x": 11, "y": 5},
   {"sym": "zener", "x": 18, "y": 10, "rot": 270},
   {"sym": "resistor", "x": 24, "y": 7, "rot": 90},
   {"sym": "ground", "x": 13, "y": 12}
  ],
  "wires": [[[8, 6], [8, 5], [11, 5]], [[14, 5], [24, 5], [24, 7]], [[18, 5], [18, 7]], [[8, 9], [8, 12], [24, 12], [24, 10]], [[18, 10], [18, 12]]],
  "dots": [[18, 5], [18, 12], [13, 12]],
  "arrows": [
   {"x": 16, "y": 5, "dir": 0}
  ],
  "texts": [
   {"x": 23.2, "y": 8.7, "text": "load", "anchor": "end"}
  ],
  "anchors": [
   {"var": "vin", "x": 7.4, "y": 7.5, "side": "left"},
   {"var": "r", "x": 12.5, "y": 4.4, "side": "top"},
   {"var": "izmin", "x": 21, "y": 4.4, "side": "top"},
   {"var": "vz", "x": 17.1, "y": 8.5, "side": "left"},
   {"var": "il", "x": 24.9, "y": 8.5, "side": "right"}
  ]
 },
 "transistor_switch": {
  "w": 28, "h": 15,
  "parts": [
   {"sym": "resistor", "x": 11, "y": 8},
   {"sym": "npn", "x": 16, "y": 8},
   {"sym": "resistor", "x": 19, "y": 2, "rot": 90},
   {"sym": "ground", "x": 19, "y": 13}
  ],
  "wires": [[[8, 8], [11, 8]], [[14, 8], [16, 8]], [[19, 1], [19, 2]], [[19, 5], [19, 6]], [[19, 10], [19, 13]]],
  "terminals": [[8, 8], [19, 1]],
  "arrows": [
   {"x": 14.6, "y": 8, "dir": 0}
  ],
  "texts": [
   {"x": 20, "y": 1.4, "text": "V_CC", "anchor": "start"},
   {"x": 17.8, "y": 3.8, "text": "load", "anchor": "end"}
  ],
  "anchors": [
   {"var": "vin", "x": 7.4, "y": 8, "side": "left"},
   {"var": "rb", "x": 12.5, "y": 7.4, "side": "top"},
   {"var": "od", "x": 11, "y": 8.6, "side": "bottom"},
   {"var": "vbe", "x": 15.2, "y": 13.2, "side": "left"},
   {"var": "ic", "x": 19.9, "y": 5.5, "side": "right"},
   {"var": "hfe", "x": 19.9, "y": 11.5, "side": "right"}
  ]
 },
 "bjt_divider_bias": {
  "w": 29, "h": 16,
  "parts": [
   {"sym": "resistor", "x": 10, "y": 4, "rot": 90},
   {"sym": "resistor", "x": 10, "y": 10, "rot": 90},
   {"sym": "npn", "x": 18, "y": 8},
   {"sym": "resistor", "x": 21, "y": 3, "rot": 90},
   {"sym": "resistor", "x": 21, "y": 10, "rot": 90},
   {"sym": "ground", "x": 15, "y": 14}
  ],
  "wires": [[[8, 3], [21, 3]], [[10, 3], [10, 4]], [[10, 7], [10, 10]], [[10, 8], [18, 8]], [[10, 13], [10, 14], [21, 14], [21, 13]]],
  "dots": [[10, 3], [10, 8], [15, 14]],
  "terminals": [[8, 3]],
  "anchors": [
   {"var": "vcc", "x": 7.4, "y": 3, "side": "left"},
   {"var": "r1", "x": 9.4, "y": 5.8, "side": "left"},
   {"var": "r2", "x": 9.4, "y": 11.5, "side": "left"},
   {"var": "beta", "x": 14.5, "y": 7.4, "side": "top"},
   {"var": "vbe", "x": 14.5, "y": 8.6, "side": "bottom"},
   {"var": "rc", "x": 21.9, "y": 4.5, "side": "right"},
   {"var": "re", "x": 21.9, "y": 11.5, "side": "right"}
  ]
 },
 "mosfet_losses": {
  "w": 31, "h": 17,
  "parts": [
   {"sym": "source", "x": 29, "y": 5, "rot": 90},
   {"sym": "isource", "x": 19, "y": 4, "rot": 90},
   {"sym": "nmos", "x": 16, "y": 11},
   {"sym": "ground", "x": 24, "y": 15}
  ],
  "wires": [[[29, 5], [29, 3], [19, 3], [19, 4]], [[19, 7], [19, 9]], [[13, 11], [16, 11]], [[19, 13], [19, 15], [29, 15], [29, 8]]],
  "dots": [[24, 15]],
  "terminals": [[13, 11]],
  "texts": [
   {"x": 13.8, "y": 10.2, "text": "PWM", "anchor": "middle"}
  ],
  "anchors": [
   {"var": "fsw", "x": 12.4, "y": 4.8, "side": "left"},
   {"var": "duty", "x": 12.4, "y": 7.8, "side": "left"},
   {"var": "tr", "x": 12.4, "y": 10.8, "side": "left"},
   {"var": "tf", "x": 12.4, "y": 13.8, "side": "left"},
   {"var": "i_d", "x": 19.9, "y": 5.5, "side": "right"},
   {"var": "rds", "x": 19.9, "y": 9, "side": "right"},
   {"var": "vds", "x": 19.9, "y": 12.4, "side": "right"}
  ]
 },
 "heatsink": {
  "w": 31, "h": 13,
  "parts": [
   {"sym": "isource", "x": 8, "y": 8, "rot": 270},
   {"sym": "resistor", "x": 11, "y": 5},
   {"sym": "resistor", "x": 16, "y": 5},
   {"sym": "resistor", "x": 21, "y": 5},
   {"sym": "ground", "x": 8, "y": 8},
   {"sym": "ground", "x": 27, "y": 8}
  ],
  "wires": [[[8, 5], [11, 5]], [[14, 5], [16, 5]], [[19, 5], [21, 5]], [[24, 5], [27, 5], [27, 8]]],
  "texts": [
   {"x": 27.9, "y": 8.3, "text": "ambient", "anchor": "start"}
  ],
  "anchors": [
   {"var": "p", "x": 7.4, "y": 6.5, "side": "left"},
   {"var": "tj", "x": 8, "y": 4.4, "side": "top"},
   {"var": "rjc", "x": 12.5, "y": 5.6, "side": "bottom"},
   {"var": "rcs", "x": 17.5, "y": 4.4, "side": "top"},
   {"var": "rsa", "x": 22.5, "y": 5.6, "side": "bottom"},
   {"var": "ta", "x": 27, "y": 9.3, "side": "bottom"}
  ]
 }
});
