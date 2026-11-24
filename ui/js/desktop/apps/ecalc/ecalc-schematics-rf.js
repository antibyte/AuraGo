window.ECalcSchematics = Object.assign(window.ECalcSchematics || {}, {
 "l_match": {
  "w": 32, "h": 14,
  "parts": [
   {"sym": "acsource", "x": 8, "y": 4, "rot": 90},
   {"sym": "resistor", "x": 9, "y": 4},
   {"sym": "inductor", "x": 14, "y": 4},
   {"sym": "capacitor", "x": 21, "y": 4, "rot": 90},
   {"sym": "resistor", "x": 26, "y": 4, "rot": 90},
   {"sym": "ground", "x": 17, "y": 10}
  ],
  "wires": [[[8, 4], [9, 4]], [[12, 4], [14, 4]], [[17, 4], [26, 4]], [[21, 7], [21, 10]], [[26, 7], [26, 10]], [[8, 7], [8, 10], [26, 10]]],
  "dots": [[21, 4], [21, 10], [17, 10]],
  "texts": [
   {"x": 17, "y": 12.8, "text": "low-pass shown; high-pass swaps L and C", "anchor": "middle"}
  ],
  "anchors": [
   {"var": "f", "x": 7.4, "y": 5.5, "side": "left"},
   {"var": "rs", "x": 9.5, "y": 3.3, "side": "top"},
   {"var": "l", "x": 16.5, "y": 3.3, "side": "top"},
   {"var": "c", "x": 20.4, "y": 6.2, "side": "left"},
   {"var": "rl", "x": 26, "y": 3.3, "side": "top"}
  ]
 }
});
