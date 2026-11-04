# raspberry_pi_40 — Raspberry Pi 40-pin header / Raspberry Pi 40-Pin-Stiftleiste

Kind: authoritative maker document
Pins: 40; types: power 4; ground 8; io 26; nc 2

## View
Written for the male pin header on the board (`drawn: plug`), looking down at the pins from above, pin 1 at the top left. Basis, both from Raspberry Pi's documentation:
- Figure "Raspberry Pi pinout diagram" (`GPIO-Pinout-Diagram-2.png`, downloaded and looked at): a Pi from the component side with the header at the right edge and USB and Ethernet at the bottom; the enlarged header shows 1 2 / 3 4 … 39 40, odd numbers in the left column.
- Text: the header lies "along the top edge" of classic SBCs and Zero models, with pin 1 in the "lower-left corner" and pin 40 in the "upper-right corner". Figure "The pin labelling for a Raspberry Pi GPIO header" (`GPIO-numbers.png`) shows that landscape view: top row 5V, 5V, GND …, bottom row 3V3, GP2, GP3 ….

The two agree: turning the landscape view by 90° clockwise moves the lower left corner to the top left and the upper left (pin 2) to the top right. It is a rotation of the same side, not a mirror, so the rows `["1","2"] … ["39","40"]` state the plug view correctly. The toggle's mirror then shows a cable socket looking into its holes, with pin 2 at the top left, which is physically right for the face that mates with this header.

## Sources
1. Raspberry Pi, documentation, Raspberry Pi hardware, section "40-pin GPIO header", https://www.raspberrypi.com/documentation/computers/raspberry-pi.html (read through the fetch tool) and its source file https://raw.githubusercontent.com/raspberrypi/documentation/master/documentation/asciidoc/computers/raspberry-pi/gpio-on-raspberry-pi.adoc (read in full for this section) — models, layout text, pin types table, pin functions table, limits, accessed 2026-11-04
2. Figures of that section: https://raw.githubusercontent.com/raspberrypi/documentation/master/documentation/asciidoc/computers/raspberry-pi/images/GPIO-Pinout-Diagram-2.png and https://raw.githubusercontent.com/raspberrypi/documentation/master/documentation/asciidoc/computers/raspberry-pi/images/GPIO-numbers.png, accessed 2026-11-04
3. Raspberry Pi, BCM2711 ARM Peripherals, table 94 "GPIO Pins Alternative Function Assignment", https://datasheets.raspberrypi.com/bcm2711/bcm2711-peripherals.pdf — bus numbers of I2C and SPI, PWM on GPIO18/19, SPI1 on GPIO19–21, accessed 2026-11-04

## Cross-check
The pin table and both figures agree on all 40 positions and names, and the counts (4 power, 8 ground, 26 GPIO, 2 ID EEPROM) match the pin types table. Disagreements:
- **GPIO0 and GPIO1.** The pin functions table gives them type GPIO; the pin types table gives them their own type "ID EEPROM", "reserved"; the labelling figure marks them DNC. I chose `nc` because the catalog schema puts reserved pins there and because the safe reading for someone wiring is to leave them free. The name stays `GPIO0`/`GPIO1` as in the pinout diagram.
- **Label style.** The current documentation writes GP2 in tables and GPIO 2 in the pinout diagram. The task asks for `GPIO2`.
- **Bus numbers.** The documentation writes CE0, CE1, CE2 and MISO without bus numbers. Table 94 of BCM2711 gives SPI0 on GPIO7–11 and SPI1 on GPIO16–21. GPIO18 and GPIO19 also have PWM0_0 and PWM0_1 there, which the documentation's table does not mention; `function` says "on BCM2711". GPIO20 and GPIO21 as SPI1 MOSI and SCLK come only from table 94, as does I2C1 for GPIO2 and GPIO3.
- I did not read the BCM2835 or the Compute Module 4 and 5 datasheets that the documentation points to for other SoCs, so the bus numbers are checked for BCM2711 only. The note in the item says alternatives vary with the SoC.
No UNVERIFIED pin.

## Choices
- `views` as planned: plug is the board's pin header, socket is a cable socket.
- 3.3 V limits are from the documentation; "not 5 V tolerant" is in its own warning.
- `3V3` and `5V` keep their type `power`. The documentation says the 5 V pins can also feed the Pi, so the function says output or input; 3V3 is described only as output.
- Pi models with a 26-pin header, and Compute Modules, are not covered.
