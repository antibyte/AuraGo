# pinouts — summary

81 items in seven groups. No item was left out, and no pin number or name is unverified. Each file names which view its drawing is
written for and on what that rests.

| Folder | Items | Main sources |
| --- | --- | --- |
| `data/` | 11 | USB-IF specifications (USB 2.0, USB 3.x, Type-C 2.5); makers and cable vendors for HDMI, DisplayPort, VGA, RJ45 and RS-232, whose standards were not read |
| `audio/` | 4 | Neutrik, Amphenol and ITT Cannon drawings for XLR; the MIDI Association for MIDI; maker documents for the jacks |
| `pc_power/` | 9 | PCI Express CEM figures, Intel's 4-wire fan specification, PSU and board makers |
| `boards/` | 6 | Arduino, Espressif, the NodeMCU project and Raspberry Pi documents, including their figures |
| `debug/` | 14 | ARM, SEGGER, Microchip, FTDI, SparkFun, Adafruit and Seeed documents; several independent sources for OBD-II and hobby servos; Ai-Thinker, Ebyte/Nordic, Aosong, MikroElektronika and Digilent for the module and bus connectors, vendor documents for the HC-SR04 |
| `ics/` | 32 | The makers' datasheets for the package drawn |
| `vehicle_mains/` | 5 | Erich Jaeger and M&K for the trailer sockets, TE Connectivity for Type 2, the ISO 10487-1 preview (complete normative text) for the car radio connector |

## Weakest points

- **Standards not read:** HDMI, VESA DisplayPort, TIA-232, TIA-568, IEEE 802.3, SAE J1962, ISO 15031-3 and AES14 are paywalled. The items rest on independent
  reproductions, as each file says.
- **`displayport`:** the view is implied: it rests on device-port pictures plus one caption, and no maker drawing could be opened. The sink side's CONFIG ground rule is left out.
- **`usb_micro_b`:** the receptacle is the mirror image of the labelled plug figure.
- **`rj45`:** pictures only, no text states the view; four documents agree.
- **`hdmi_a`:** the HEAC polarity of pins 14 and 19 differs between sources (+ on 14 chosen). The +5 V current is 50 or 55 mA, and the notes state both.
- **PoE polarity:** sources conflict, so the item states only which pins form each pole.
- **`molex_4`:** one Maxtor drawing plus the mirror rule; the side of the cut corners is the least certain detail.
- **`sata_data`, `sata_power`:** the direction of the L step rests on one Intel figure, one Gigabyte drawing and the SATA-IO row order.
- **`eps_8`, `fan_3`:** the view is derived from the connector family.
- **`xlr`:** the drawn key is the latch. AES14 itself was not read; pin functions rest on Rane, Shure and Mackie, geometry on the makers' drawings.
- **`qwiic`:** both makers' schematics say pin 1 = GND, but SparkFun's cable drawing says pin 1 = SCL. GND was chosen, and a note warns. JST's catalogue could not be read, so
  `qwiic` and `grove` state the sequence of the pins, not which physical end is pin 1.
- **`servo`:** the wire colours rest on a few makers, and the numbers `S`, `+`, `-` are used because Parallax and SparkFun number in opposite directions.
- **`nrf24l01`, `hc_sr04`:** `nrf24l01` shows the order Ebyte documents for its E01 modules (unbranded modules have no maker document); `hc_sr04` rests on two vendor documents, no original maker.
- **`dht22`:** one Aosong edition prints GND at pin 3; it is resolved as NC from the other edition and both connection diagrams.
- **`mikrobus`:** the standard has no pin numbers; 1 to 16 are Microchip's. **`pmod`:** only the generic layout; the current Digilent specification could not be opened, so the per-type tables are left out.
- **`arm_jtag_20`, `arm_swd_10`, `avr_isp_6`:** the side of the key comes from drawings; Arm's overview says it is no specification. SEGGER differs on JTAG pins 14 to 20 (5 V output on 19, in the notes).
- **`esp8266_nodemcu`:** a text file in the project's repository swaps D1 and D2 against the schematic, the pin map and the firmware documents; those three were followed. Clones differ. No ESP-12E datasheet was read.
- **`raspberry_pi_40`:** SPI1 and PWM on GPIO18 to GPIO21 were checked only against the BCM2711.
- **`l78xx`:** st.com could not be reached; ST's PDFs were read on third-party hosts (rev 34 of 2016 and rev 19 of 2008). The lead order agrees in both.
- **`bc547`:** onsemi's sheet is marked obsolete, and no other maker was checked.
- **`lm386`:** the DIP assignment rests on TI's single pin table and the PDIP layout figure (Figure 11-1, which confirms pins 2 to 6) only; TI's pin figure is captioned for the MSOP and SOIC packages alone.
- **`mcp23017`:** the figure was read from the PDF text layer, not from an image; Table 2-1 and the package drawing agree.
- **`to92`, DS18B20 variant:** it rests on Dallas's 2000 preliminary datasheet. The current Analog Devices sheet could not be opened, so the order should be checked against it before the variant is relied on.
- **`sot23`, `sot223`:** the pin numbers of the onsemi parts come from onsemi alone. Diodes Incorporated prints the function letters at the leads but no pin numbers, so it confirms the positions only. The LM1117 in `sot223` rests on TI alone.
- **`tip120`, `bt136`:** one maker each (onsemi for TIP120, WeEn for BT136), each for its own part; there is no second source.
- **`to92`:** TI's table and its generic outline disagree on the TL431's pin numbers; the named figure and onsemi agree, so positions are used and the notes give TI's numbers.

- **`trailer_7`, `trailer_13`:** the positions come from the maker's renders (Jaeger); the key position is unverified, so no key is drawn. The 13-pin angles are approximate (about 7 degrees), the contact numbers exact; contacts 11 and 13 have no wire id (white/black, white/red).
- **`type2`:** one maker document (TE 108-94323); the voltages and currents there are TE's assembly ratings. CP signalling is not covered.
- **`iso10487_a`, `iso10487_b`:** the standard itself; A4 and A7 follow the standard, not the common folklore.

## Judgement calls on types

Read these before relying on the colours: `V-` and `VCC-` of the op-amps and `COM` of the ULN2003A are `power`, `ANODE` of the TL431 is `ground`. The flash lines of the ESP32
(GPIO6 to GPIO11) and the NodeMCU are `data`, not `io`, so they do not look like free pins. GPIO0 and GPIO1 of the Raspberry Pi are `nc` (reserved for the HAT EEPROM, marked
"do not connect"). Manufacturer pins of OBD-II are `nc` although they can be live, which the function text says.
