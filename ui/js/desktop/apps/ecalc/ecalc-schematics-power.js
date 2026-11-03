window.ECalcSchematics = Object.assign(window.ECalcSchematics || {}, {
 "lm317_voltage": {
  "w": 31, "h": 15,
  "parts": [
   {"sym": "source", "x": 4, "y": 4, "rot": 90},
   {"sym": "ic", "x": 9, "y": 3, "w": 4, "h": 5, "text": "LM317", "pins": [{"n": "IN", "s": "l", "y": 1}, {"n": "OUT", "s": "r", "y": 1}, {"n": "ADJ", "s": "r", "y": 4}]},
   {"sym": "resistor", "x": 22, "y": 4, "rot": 90},
   {"sym": "resistor", "x": 22, "y": 7, "rot": 90},
   {"sym": "ground", "x": 13, "y": 13}
  ],
  "wires": [[[4, 4], [9, 4]], [[15, 4], [27, 4]], [[15, 7], [22, 7]], [[4, 7], [4, 13], [27, 13]], [[22, 10], [22, 13]]],
  "dots": [[22, 4], [22, 7], [22, 13], [13, 13]],
  "terminals": [[27, 4], [27, 13]],
  "arrows": [
   {"x": 18, "y": 7, "dir": 0}
  ],
  "anchors": [
   {"var": "vref", "x": 11, "y": 8.6, "side": "bottom"},
   {"var": "iadj", "x": 18.2, "y": 7.6, "side": "bottom"},
   {"var": "r1", "x": 22.4, "y": 5.7, "side": "right"},
   {"var": "r2", "x": 22.4, "y": 8.7, "side": "right"},
   {"var": "vout", "x": 27, "y": 3.7, "side": "top"}
  ]
 },
 "lm317_current": {
  "w": 31, "h": 15,
  "parts": [
   {"sym": "source", "x": 4, "y": 4, "rot": 90},
   {"sym": "ic", "x": 9, "y": 3, "w": 4, "h": 5, "text": "LM317", "pins": [{"n": "IN", "s": "l", "y": 1}, {"n": "OUT", "s": "r", "y": 1}, {"n": "ADJ", "s": "r", "y": 4}]},
   {"sym": "resistor", "x": 22, "y": 4, "rot": 90},
   {"sym": "led", "x": 22, "y": 7, "rot": 90},
   {"sym": "ground", "x": 13, "y": 13}
  ],
  "wires": [[[4, 4], [9, 4]], [[15, 4], [22, 4]], [[15, 7], [22, 7]], [[4, 7], [4, 13], [22, 13]], [[22, 10], [22, 13]]],
  "dots": [[22, 7], [13, 13]],
  "arrows": [
   {"x": 18, "y": 7, "dir": 0}
  ],
  "texts": [
   {"x": 21.4, "y": 11.4, "text": "Load", "anchor": "end"}
  ],
  "anchors": [
   {"var": "vref", "x": 11, "y": 8.6, "side": "bottom"},
   {"var": "iadj", "x": 18.2, "y": 7.6, "side": "bottom"},
   {"var": "r", "x": 22.4, "y": 5.5, "side": "right"},
   {"var": "i", "x": 23.6, "y": 10, "side": "right"}
  ]
 },
 "linear_regulator_loss": {
  "w": 32, "h": 13,
  "parts": [
   {"sym": "source", "x": 4, "y": 4, "rot": 90},
   {"sym": "ic", "x": 9, "y": 3, "w": 4, "h": 4, "text": "REG", "pins": [{"n": "IN", "s": "l", "y": 1}, {"n": "OUT", "s": "r", "y": 1}, {"n": "GND", "s": "r", "y": 3}]},
   {"sym": "isource", "x": 24, "y": 4, "rot": 90},
   {"sym": "ground", "x": 10, "y": 11}
  ],
  "wires": [[[4, 4], [9, 4]], [[15, 4], [24, 4]], [[15, 6], [15, 11]], [[4, 7], [4, 11], [24, 11]], [[24, 7], [24, 11]]],
  "dots": [[15, 11], [10, 11]],
  "arrows": [
   {"x": 15, "y": 8.5, "dir": 90}
  ],
  "anchors": [
   {"var": "vin", "x": 4, "y": 3.7, "side": "top"},
   {"var": "p", "x": 11, "y": 7.6, "side": "bottom"},
   {"var": "iq", "x": 15.4, "y": 8.8, "side": "right"},
   {"var": "vout", "x": 20, "y": 3.7, "side": "top"},
   {"var": "iout", "x": 24.6, "y": 5.5, "side": "right"}
  ]
 },
 "buck_converter": {
  "w": 31, "h": 12,
  "parts": [
   {"sym": "source", "x": 4, "y": 4, "rot": 90},
   {"sym": "switch", "x": 9, "y": 4},
   {"sym": "diode", "x": 13, "y": 7, "rot": 270},
   {"sym": "inductor", "x": 16, "y": 4},
   {"sym": "capacitor", "x": 22, "y": 4, "rot": 90},
   {"sym": "ground", "x": 17, "y": 10}
  ],
  "wires": [[[4, 4], [9, 4]], [[12, 4], [16, 4]], [[19, 4], [29, 4]], [[13, 7], [13, 10]], [[22, 7], [22, 10]], [[4, 7], [4, 10], [29, 10]]],
  "dots": [[13, 4], [22, 4], [13, 10], [22, 10], [17, 10]],
  "terminals": [[29, 4], [29, 10]],
  "arrows": [
   {"x": 25.5, "y": 4, "dir": 0}
  ],
  "anchors": [
   {"var": "vin", "x": 4, "y": 3.7, "side": "top"},
   {"var": "f", "x": 10.5, "y": 3.3, "side": "top"},
   {"var": "l", "x": 17.5, "y": 3.6, "side": "top"},
   {"var": "dil", "x": 17.5, "y": 4.7, "side": "bottom"},
   {"var": "iout", "x": 25.5, "y": 3.7, "side": "top"},
   {"var": "vout", "x": 22.7, "y": 6.2, "side": "right"}
  ]
 },
 "boost_converter": {
  "w": 32, "h": 12,
  "parts": [
   {"sym": "source", "x": 4, "y": 4, "rot": 90},
   {"sym": "inductor", "x": 9, "y": 4},
   {"sym": "switch", "x": 14, "y": 4, "rot": 90},
   {"sym": "diode", "x": 16, "y": 4},
   {"sym": "capacitor", "x": 23, "y": 4, "rot": 90},
   {"sym": "ground", "x": 18, "y": 10}
  ],
  "wires": [[[4, 4], [9, 4]], [[12, 4], [16, 4]], [[19, 4], [30, 4]], [[14, 7], [14, 10]], [[23, 7], [23, 10]], [[4, 7], [4, 10], [30, 10]]],
  "dots": [[14, 4], [23, 4], [14, 10], [23, 10], [18, 10]],
  "terminals": [[30, 4], [30, 10]],
  "arrows": [
   {"x": 26.5, "y": 4, "dir": 0}
  ],
  "anchors": [
   {"var": "vin", "x": 4, "y": 3.7, "side": "top"},
   {"var": "l", "x": 10.5, "y": 3.6, "side": "top"},
   {"var": "dil", "x": 10.5, "y": 4.7, "side": "bottom"},
   {"var": "f", "x": 14.9, "y": 6.2, "side": "right"},
   {"var": "iout", "x": 26.5, "y": 3.7, "side": "top"},
   {"var": "vout", "x": 23.7, "y": 6.2, "side": "right"}
  ]
 },
 "smoothing_capacitor": {
  "w": 32, "h": 12,
  "parts": [
   {"sym": "diode", "x": 9, "y": 7, "rot": 270},
   {"sym": "diode", "x": 9, "y": 10, "rot": 270},
   {"sym": "diode", "x": 15, "y": 7, "rot": 270},
   {"sym": "diode", "x": 15, "y": 10, "rot": 270},
   {"sym": "acsource", "x": 11, "y": 7},
   {"sym": "capacitor", "x": 19, "y": 5, "rot": 90},
   {"sym": "isource", "x": 28, "y": 5, "rot": 90},
   {"sym": "ground", "x": 12, "y": 10}
  ],
  "wires": [[[9, 4], [28, 4], [28, 5]], [[9, 7], [11, 7]], [[14, 7], [15, 7]], [[19, 4], [19, 5]], [[19, 8], [19, 10]], [[28, 8], [28, 10]], [[9, 10], [28, 10]]],
  "dots": [[9, 7], [15, 7], [15, 4], [19, 4], [15, 10], [19, 10], [12, 10]],
  "anchors": [
   {"var": "f", "x": 8.4, "y": 7, "side": "left"},
   {"var": "dv", "x": 19, "y": 3.7, "side": "top"},
   {"var": "i", "x": 28, "y": 3.7, "side": "top"},
   {"var": "c", "x": 19.7, "y": 6.5, "side": "right"}
  ]
 }
});
