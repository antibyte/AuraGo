# esp01 — ESP-01 / ESP-01S (ESP8266), 2x4 header / ESP-01 / ESP-01S (ESP8266), 2x4-Stiftleiste

Kind: authoritative maker document (Ai-Thinker, maker of the module)
Pins: 8; types: power 1, ground 1, signal 2, io 2, data 2

## Pins
| No. | Name | Function (en) | Function (de) | Type |
| --- | --- | --- | --- | --- |
| 1 | GND | Ground | Masse | ground |
| 2 | IO2 | GPIO2; must be high at boot (both boot modes) | GPIO2; muss beim Start high sein (beide Startmodi) | io |
| 3 | IO0 | GPIO0; low at boot = download (flash) mode, high = run mode | GPIO0; low beim Start = Download-(Flash-)Modus, high = Normalbetrieb | io |
| 4 | RXD | UART0 receive (GPIO3), input of the module | UART0 Empfang (GPIO3), Eingang des Moduls | data |
| 5 | TXD | UART0 transmit (GPIO1), output of the module | UART0 Senden (GPIO1), Ausgang des Moduls | data |
| 6 | EN | Chip enable (CH_PD), active high; pull up to run | Chip-Freigabe (CH_PD), aktiv high; zum Betrieb hochziehen | signal |
| 7 | RST | Reset, active low | Reset, aktiv low | signal |
| 8 | VCC | Supply 3.0 to 3.6 V (3.3 V typical); not 5 V | Versorgung 3,0 bis 3,6 V (typisch 3,3 V); nicht 5 V | power |

## View
Written for the module seen from its component side, antenna area at the top, header at the bottom edge: `drawn: socket` (this equals the host's female header seen from above, because the module plugs in face up).

Rows, left to right:
- [1 GND, 2 IO2, 3 IO0, 4 RXD]  (inner row, towards the antenna)
- [5 TXD, 6 EN, 7 RST, 8 VCC]   (outer row, at the board edge)

Note the numbering: 5 lies below 1, not counter-clockwise as on a DIP package. The other view (mirror) is the module's pin side seen from below: RXD, IO0, IO2, GND on the inner row.

Basis:
- Ai-Thinker manual V1.3, page 2 of 23, "Figure 2.1 ESP-01/ESP-01S pin dimensions": outline with the antenna area ("天线区域", antenna area) at the top, pin-1 dot at the upper left of the header, upper row "1 2 3 4" labelled GND, IO2, IO0, RXD, lower row "5 6 7 8" labelled TXD, EN, RST, VCC, the lower row 1.5 mm from the bottom edge.
- That figure is the component side: "Figure 4.1 Typical application of ESP-01" (page 11, photo of the component side with the antenna to the right and the header on the left) shows the GND wire on the header pin nearer the chip at the top and ESP_TXD, EN, RST, 3.3V on the outer column from top to bottom, ESP_RXD at the inner bottom. Turned so the antenna points up, this is exactly figure 2.1. (Read from a small photo; the wire ends are clear for GND, ESP_TXD, 3.3V and ESP_RXD.)

## Sources
1. Shenzhen Ai-Thinker Technology Co., Ltd, "ESP-01/07/12 Series Modules User's Manual", version 1.3 (2018-03-10; V1.0 2017-07-01), copy at https://home-assistant-guide.com/wp-content/uploads/2021/04/ESP-01_ESP-07_ESP-12_Series_User_Manual_V1.3.pdf — page 2 (figure 2.1), page 6 ("Table 2.2 ESP Series Module Pin Function Definitions", column "01 / 01S": 7 RST "Reset Pin, Active Low", 6 EN "Chip Enabled Pin, Active High", 8 VCC "Module power supply pin, the voltage range of 3.0 ~ 3.6V", 1 GND, 2 IO2 "U1_TXD, I2C_SDA, I2SO_WS", 3 IO0 "GPIO0, HSPI_MISO, I2SI_DATA", 4 RXD "GPIO3, I2SO_DATA", 5 TXD "GPIO1"), page 7 (table 2.3 boot modes: download = CH_PD high, RST high, GPIO15 low, GPIO0 low, GPIO2 high; running = GPIO0 high), page 8 (tables 3.1 and 3.2: supply 3.0 to 3.6 V, typ 3.3 V), page 11 (figure 4.1 and note: "You can't use USB to TTL 3.3V or 5V power supply"). Rendered and read. Accessed 2026-11-24.

Not opened: Ai-Thinker's single-module "ESP-01 product specification" (the docs.ai-thinker.com URL returns the documentation index page now). Espressif makes the chip, not this module; its datasheet was not needed for the header.

## Cross-check
One maker document, read in two places that agree: the pin table (page 6) and the outline figure (page 2), plus the application photo (page 11) for the side. No second maker exists for this module (Ai-Thinker designed it). Table 2.3 uses "CH_PD (EN)" for pin 6, so both names are the same pin. ESP-01 and ESP-01S share the figure and the table column.

## Choices
- Catalog item: names as printed in figure 2.1 (IO2, IO0, RXD, TXD, EN, RST); functions carry the GPIO numbers and the CH_PD alias; `rows` with two rows of four, `outline: rect`, `contact: square`, `drawn: socket`, no key. The boot-mode facts (table 2.3) are in the item note and the functions of IO0 and IO2, because that table was read.
- IO0 and IO2 `io`; RXD/TXD `data`; EN and RST `signal`.
- VCC function says "not 5 V": the maximum rating table gives +3.0 to +3.6 V.
- The note's "about 170 mA when transmitting" is the typical transmit value of table 3.4; the 3.3 V pin of many USB-serial adapters cannot supply it (Ai-Thinker: do not use the USB-TTL 3.3 V pin). The remark on adapters is a consequence of that, not a source statement.

## Spot-check
| No. | Name | Type | Reference |
| --- | --- | --- | --- |
| 1 | GND | ground | Ai-Thinker manual V1.3, figure 2.1 (pin 1 dot, upper left); table 2.2 "01/01S" column, "1 ... GND" |
| 3 | IO0 | io | figure 2.1 upper row third; table 2.2 "3 ... IO0 GPIO0"; table 2.3 boot modes |
| 6 | EN | signal | figure 2.1 lower row second; table 2.2 "6 ... EN Chip Enabled Pin, Active High" |
| 7 | RST | signal | figure 2.1 lower row third; table 2.2 "7 ... RST Reset Pin, Active Low" |
| 8 | VCC | power | figure 2.1 lower row right; table 2.2 "8 ... VCC ... 3.0 ~ 3.6V"; table 3.2 |

UNVERIFIED: none for the pins. The side of figure 2.1 rests on the application photo (figure 4.1), which is small; a second document with a labelled photo would remove all doubt. Clone ESP-01 boards from other makers are not covered.
