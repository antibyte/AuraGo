# hc_sr04 — HC-SR04 ultrasonic module (4-pin) / HC-SR04 Ultraschallmodul (4-polig)

Kind: cross-checked reproductions (two independent module vendors' documents; no original maker document exists that I could find)
Pins: 4; types: power 1, ground 1, signal 2

## Pins
| No. | Name | Function (en) | Function (de) | Type |
| --- | --- | --- | --- | --- |
| VCC | VCC | 5 V supply (4.5 to 5.5 V) | 5-V-Versorgung (4,5 bis 5,5 V) | power |
| Trig | Trig | Trigger input: a high pulse of at least 10 µs starts a measurement | Trigger-Eingang: ein High-Impuls von mindestens 10 µs startet eine Messung | signal |
| Echo | Echo | Echo output: high for a time proportional to the distance (µs / 58 = cm), 5 V level | Echo-Ausgang: High für eine zur Entfernung proportionale Zeit (µs / 58 = cm), 5-V-Pegel | signal |
| GND | GND | Ground | Masse | ground |

The module prints no pin numbers; the pin "numbers" are the names, like `S`, `+`, `-` in the `servo` item, which avoids an invented numbering.

## View
Written for the module seen from the front (the two transducers facing you, crystal "Y1" between them), header pins pointing down: one row VCC, Trig, Echo, GND from left to right. The left transducer is marked "T", the right one "R". `drawn: plug` (the module carries the pins).

Basis:
- ElecFreaks document, page 2: front photo of the module, pins down, silkscreen "Vcc Trig Echo GND" on the board and the same labels printed under the four pins from left to right.
- Cytron manual, page 4 (packing list photo) and page 5 ("3.0 Product Layout"): front photo, pins down, silkscreen "VCC Trig Echo GND" left to right; the back photo on page 5 shows the pins in the reverse place, consistent with the mirror.

The other view (mirror) is the back of the module (component side): GND, Echo, Trig, VCC from left to right.

## Sources
1. ElecFreaks, "Ultrasonic Ranging Module HC - SR04", 3 pages, PDF metadata 2011-05-11, copy hosted by SparkFun: https://cdn.sparkfun.com/datasheets/Sensors/Proximity/HCSR04.pdf — page 1 ("Wire connecting direct as following: 5V Supply, Trigger Pulse Input, Echo Pulse Output, 0V Ground"; working voltage DC 5 V; trigger input "10uS TTL pulse"), page 2 (photo with "Vcc Trig Echo GND"; timing; "uS / 58 = centimeters"; "over 60ms measurement cycle"), page 3 (connect GND first). Rendered and read. Accessed 2026-11-24.
2. Cytron Technologies Sdn. Bhd., "Product User's Manual – HC-SR04 Ultrasonic Sensor", V1.0, May 2013, copy at https://embedded.fel.cvut.cz/sites/default/files/kurzy/lpe/ultrasonic_hc-sr04/HCSR04_DS_Cytron.pdf — page 5 ("VCC = +5VDC, Trig = Trigger input of Sensor, Echo = Echo output of Sensor, GND = GND" and photos), page 6 (operating voltage 4.50 / 5.0 / 5.5 V), page 7 (10 µs trigger, Echo high "(5V)", Time / 58; "Please connect the GND pin first before supplying power to VCC."). Rendered and read. Accessed 2026-11-24.

## Cross-check
Both documents agree on all four pins, on the left-to-right order in the front view, on 5 V supply, the 10 µs trigger and the /58 formula. Differences: ElecFreaks gives only "DC 5 V"; Cytron gives 4.5 to 5.5 V. Both warn to connect GND before VCC. Cytron states that Echo goes "high (5V)", which matters for 3.3 V controllers (a divider or level shifter is needed; that advice is an inference from the 5 V level, not a source statement, and the note says so).

## Choices
- Catalog item: one `rows` row, `outline: rect`, `contact: square`, `drawn: plug`, front view; the pin numbers are the names VCC, Trig, Echo, GND (no printed numbers exist). The note states that Echo is a 5 V level.
- Trig and Echo are `signal` (pulse timing, not a data bus).
- Not covered: HC-SR04 variants with 3.3 V operation or a different board (for example the "RCWL" or "HC-SR04P" types); their makers' documents were not read, and the note says so.

## Spot-check
| No. | Name | Type | Reference |
| --- | --- | --- | --- |
| VCC | VCC | power | ElecFreaks page 2 photo label "Vcc" (leftmost); Cytron page 5 "VCC = +5VDC" and photo |
| Trig | Trig | signal | ElecFreaks page 2 "Trig"; Cytron page 5 "Trig = Trigger input of Sensor" |
| Echo | Echo | signal | ElecFreaks page 2 "Echo"; Cytron page 5 "Echo = Echo output of Sensor", page 7 "set the Echo pin to high (5V)" |
| GND | GND | ground | ElecFreaks page 2 "GND" (rightmost); Cytron page 5 "GND = GND" |

UNVERIFIED: no document from the original module maker (unknown) was found; both sources are vendors. Clone boards with other layouts exist; check the silkscreen.
