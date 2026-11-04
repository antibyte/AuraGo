# atmega328p — ATmega328P microcontroller (DIP-28) / ATmega328P Mikrocontroller (DIP-28)

Kind: authoritative maker document
Pins: 28; types: power 2, ground 2, signal 2, io 22

## View
Shape `dip`, 28 pins: package from above, notch at the top, pin 1 top left, numbers counter-clockwise. Basis: Microchip's Figure 1-1 draws "28 SPDIP" with the notch on top, 1–14 down the left and 28–15 down the right. That one caption does not say "Top View", unlike the three other packages of the same figure (32 TQFP, 28 VQFN, 32 VQFN, all "Top View"). The notch on top with pin 1 at the top left and counter-clockwise numbers only fits a view from above, so I derive the view from that numbering rule. I rendered PDF page 12 to an image and looked at it.

## Sources
1. Microchip, ATmega48A/PA/88A/PA/168A/PA/328/P Data Sheet Complete, DS40002061B (Rev. B, 08/2020), Figure 1-1 (page 12), section 1.1 Pin Descriptions (pages 13–14), Alternate Functions of Ports B, C and D. https://ww1.microchip.com/downloads/aemDocuments/documents/MCU08/ProductDocuments/DataSheets/ATmega48A-PA-88A-PA-168A-PA-328-P-DS-DS40002061B.pdf — covers ATmega328P-PU in package 28P3, the 28-lead SPDIP, accessed 2026-11-04.

## Cross-check
One maker document is enough here. Figure 1-1 and the port tables agree on every alternative function I list. No disagreement, nothing UNVERIFIED.

## Choices
- Pin 1 is named `PC6/RESET`, the heading of section 1.1.5, and typed `signal`: with the RSTDISBL fuse unprogrammed it is the reset input. A pin note covers the fuse case. The port name is still in the name, as the task asks.
- GPIOs (PB, PC0–PC5, PD) are `io`. AVCC is `power`; AREF is `signal`.
- Function text lists every alternative function of the figure, with a short gloss in English and German. `SS` has an overbar in the figure; the text says active low.
- The two GND pins are 8 and 22, each its own table row.
- Left out: ADC6 and ADC7, which exist only in TQFP and VQFN (section 1.1.9), stated in the notes. The PB6/PB7 fuse note is short; the clock chapter holds the details.
- OC outputs are called "compare output", following the datasheet's "Output Compare Match". They are the PWM outputs, which the tag `pwm` covers.
