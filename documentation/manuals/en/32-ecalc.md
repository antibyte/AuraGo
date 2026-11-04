# Chapter 32: ECalc – electronics formulas in the Virtual Desktop

ECalc is a formula collection for electronics in the Virtual Desktop. A calculator page shows the circuit, the formula and the results side by side, and it recalculates while you type. You enter the values you know and ECalc works out the rest. For resistors it also suggests the nearest standard value from an E series, and the standard-value combiner finds standard values that reach a target resistance, in series or in parallel.

ECalc has 79 calculators in ten categories, 44 reference tables (see [Tables](#tables)), five component code pages (see [Component Codes](#component-codes)) and 47 pinouts of connectors, board headers and ICs (see [Pinouts](#pinouts)). The agent can calculate with the same formulas, read the tables, codes and pinouts, and open a calculator for you (see [Using ECalc from the Agent](#using-ecalc-from-the-agent)). ECalc runs in your browser. It needs no provider and has no switch of its own in **Config**.

## Table of Contents
1. [Opening ECalc](#opening-ecalc)
2. [Finding a Calculator](#finding-a-calculator)
3. [The Calculator Page](#the-calculator-page)
4. [Entering Values](#entering-values)
5. [Standard Values (E Series)](#standard-values-e-series)
6. [Settings](#settings)
7. [The Calculators](#the-calculators)
8. [Tables](#tables)
9. [Component Codes](#component-codes)
10. [Pinouts](#pinouts)
11. [Sources and Honesty](#sources-and-honesty)
12. [Using ECalc from the Agent](#using-ecalc-from-the-agent)
13. [Privacy and Safety](#privacy-and-safety)
14. [When Something Goes Wrong](#when-something-goes-wrong)

---

## Opening ECalc

Open **ECalc** from the start menu, in the category **Development**. The window opens at about 1100 × 760 pixels and you can
resize it. ECalc reopens on the calculator you used last.

The window has three columns: a **category rail** on the left, a **list** of calculators in the middle and the **calculator
page** on the right. In a narrow window (below about 720 pixels) the three become one stack. Choose a category, then a
calculator, and use **Back** to return to the previous level.

## Finding a Calculator

- **Category rail:** **Favorites** and **Recent** come first, followed by the categories that hold items: Basics, Passive,
  Filters, Semiconductors, Power supply, Op-amps, Timers, RF, Digital, Wiring & PCB, **Tables**, **Codes** and **Pinouts**. Each
  category shows how many items it holds. The rail lists a category only when it has content.
- **Search:** type in the search box at the top to find a calculator, a table, a component code or a pinout by its name in your language, by
  its English name or by a keyword such as "divider". Each item has keywords, and the search matches them too.
- **Favorites:** the favorite button on a calculator page adds it to **Favorites**. Click it again to remove it.
- **Recent:** ECalc remembers the last 12 calculators you opened.

## The Calculator Page

A calculator page has these parts:

- **Title bar:** the name of the calculator and the buttons **Add to favorites**, **Copy results** and **Reset to defaults**.
- **Schematic:** the circuit, with an input field next to each component it belongs to. A calculator without a schematic shows
  its fields in a plain list.
- **Formula:** the formula in typeset form, and below it a live line with your values filled in, for example
  *V_out = 12 V · 2.2 kΩ / (4.7 kΩ + 2.2 kΩ) = 3.83 V*.
- **Nearest standard values:** the E-series suggestions for the resistors (see [Standard Values (E Series)](#standard-values-e-series)).
- **Combinations:** only on the **Standard-value combiner**. It shows the nearest single standard value to the target, and up to
  five pairs in series and up to five pairs in parallel, each with its value and its deviation from the target. The block is
  headed with the series you chose, for example "Combinations of E24 values".
- **Derived values:** further quantities that follow from the result, for example the current through a divider and its power.
- **Chart:** a curve under the formula. It plots one value against another from 0 to an end value, and every other value stays at
  the value on the page. The RC time constant, for example, draws the capacitor voltage over time from 0 to 5 τ. The chart stays
  hidden while the values it needs are unknown.
- **Warnings:** a note when the values do not make sense, for example an output voltage that is not below the input voltage.
- **Explanation:** a collapsible text about the calculator, with its **Source**: the standard, datasheet or textbook behind the
  formula.

**Copy results** puts the values of the page on the clipboard. **Reset to defaults** restores the starting values of the
calculator.

**Dimmed fields:** some calculators offer an option, such as the power system of a cable (DC, single-phase AC or three-phase
AC). A field that the selected option does not use is dimmed, and its tooltip reads "Not used with the selected option". For
DC in the **Cable voltage drop** calculator, for example, cos φ is dimmed. Dimmed fields are left out of **Copy results** and of
the E-series suggestions.

## Entering Values

Every calculator needs a fixed number of known values. Ohm's law, for example, needs two of the four quantities voltage,
current, resistance and power. The fields you fill in are **inputs**. ECalc fills the other fields with **computed** values and
tints them.

- **Change what is known:** type into a computed field. It becomes an input, and the input you edited longest ago becomes
  computed. This way you can work from whichever value you have at hand.
- **Lock a value:** the lock icon on a field pins it. A locked field stays an input until you unlock it, even when you edit
  other fields.
- **Number formats:** you can type `4k7` (4.7 kΩ), `2u2` (2.2 µ), `10 mA`, `1e-6` or `0,5`. A decimal comma and a decimal point
  both work. A unit after the number is optional.
- **Number systems:** the four fields of the **Number systems** calculator (dec, hex, bin, oct) show the same whole number, each in
  its own base. Type the number into any of them, using the digits of that field's base. The prefix of that base is optional: `0x`
  for hexadecimal (`0xFF`), `0b` for binary (`0b1111_1111`) and `0o` for octal (`0o377`). Decimal takes plain digits (`255`), and
  `_` or a space may separate the digits. Long hexadecimal and binary results are grouped for readability, by four digits in
  hexadecimal (`0xDEAD_BEEF`) and by eight in binary (`0b1_00000000`).
- **Units:** the dropdown next to a field chooses the unit, and it offers only the units that suit the field. **Auto** picks a
  suitable prefix, such as mA or kΩ. A cross-section offers mm², AWG and kcmil, a temperature offers °C, °F and K, and the copper
  thickness of a PCB trace offers oz (copper weight). Some fields offer special units, such as dBm, dBV and dBu for levels, or
  mAh, Ah and Wh for charge and energy.
- **Lists:** the series and parallel calculator takes a list such as `4k7, 10k, 2k2`. Separate the entries with commas,
  semicolons or spaces. A comma directly in front of a digit is a decimal comma, so `4,7k, 10k` is two entries (4.7 kΩ and
  10 kΩ) while `1,2,3` is one entry that cannot be read; put a space after a comma that separates entries.
- **Display:** the results show a decimal comma in German, Czech, Danish, Greek, Spanish, French, Italian, Dutch, Norwegian,
  Polish, Portuguese and Swedish, and a decimal point in the other languages. Lists always show a decimal point, because the
  comma separates their entries.

When something is wrong, the field shows a message instead of a number:

| Message | Meaning |
|---------|---------|
| "Enter more values to calculate this." | The known values do not determine this field yet. |
| "These values give no finite result." | The calculation divides by zero or leaves the range of numbers. |
| "This value must be greater than zero." | Resistance, for example, cannot be zero or negative. |
| "Could not read this value. Examples: 4k7, 10 mA, 1e-6." | The text is not a number with an optional prefix and unit. Your previous values stay as they were. |

## Standard Values (E Series)

Resistors come in standard values. Every resistor field of a calculator shows **Nearest E24 value**: the closest standard
value and how far it is from the calculated one, in percent. For example, 4.7 kΩ is in the E24 series, and a calculated 4.6 kΩ
would show 4.7 kΩ with a deviation of about +2.2 %.

Choose another series (E6 to E192) in the **E series** selector. The default is E24. Series with more values, such as E96, give
a smaller deviation. ECalc remembers your choice.

**Standard-value combiner.** The calculator **Standard-value combiner** in the Basics category turns a target resistance into
standard values. It uses the same **E series** selector. It shows the nearest single value of the chosen series, and up to five
pairs in series and up to five pairs in parallel. Each pair shows its combined value and its deviation from the target in
percent. For example, 5 kΩ from E12 gives 4.7 kΩ + 270 Ω (−0.6 %), or 10 kΩ in parallel with 10 kΩ (exactly 5 kΩ).

## Settings

The settings menu has two options:

- **Significant digits:** how many digits the results show, from 3 to 6. The default is 4.
- **Schematic symbols:** **IEC** draws the resistor as a rectangle (the default). **ANSI** draws the zigzag resistor.

ECalc keeps its settings, your favorites, the recent list, the units you chose and the last values of each calculator in the
browser, under the key `aurago.ecalc.v1`. They stay in that browser. Nothing is stored on the AuraGo server, and clearing the
browser data resets them. ECalc works without browser storage too, it just forgets everything when you close the window.

## The Calculators

ECalc has 79 calculators in ten categories. Each table lists the calculators of one category, in the order of the category rail.
The source behind each formula, the standard, datasheet or textbook, is shown on the calculator's page under **Explanation**, so
the tables leave it out.

### Basics

| Calculator | What it does |
|------------|--------------|
| Ohm's law and power | Voltage, current, resistance and power. Enter any two and get the other two. |
| Voltage divider | The output voltage of a divider of two resistors. It also finds a resistor or the input voltage for a target output, and shows current and power. |
| Loaded voltage divider | The same divider with a load resistor on the output. It shows the unloaded output for comparison and warns when the load makes the target impossible. |
| Current divider | How a current splits between two parallel resistors. |
| Series and parallel resistors | The total resistance of a list of resistors, in series and in parallel. |
| Decibel ratio | Converts between a ratio and decibels. Choose voltage (20 log) or power (10 log). |
| dB levels | Voltage, resistance and power with the levels in dBm, dBV and dBu. |
| Wheatstone bridge | The bridge voltage from the supply and four resistors, or the resistor that balances the bridge. |
| Star-delta transform | Converts three delta resistors to the equivalent star, and back. |
| RMS, peak and average | Converts between the RMS, peak, peak-to-peak and average value of a sine, square or triangle wave. |
| Charge and energy | Converts between charge (for example the mAh of a battery), voltage and energy (Wh or J). |
| Standard-value combiner | The nearest single standard value to a target resistance, and series or parallel pairs that come closest. |

### Passive

| Calculator | What it does |
|------------|--------------|
| Capacitive reactance | Reactance of a capacitor. Any two of frequency, capacitance and reactance give the third. |
| Inductive reactance | Reactance of a coil. Any two of frequency, inductance and reactance give the third. |
| Series impedance (RC, RL, RLC) | Net reactance, impedance and phase of a resistor in series with a coil, a capacitor or both. |
| Parallel impedance (RC, RL, RLC) | Impedance, phase and equivalent parallel reactance of a resistor in parallel with a coil, a capacitor or both. |
| RC time constant | Charging and discharging of a capacitor through a resistor, with the time constant and the voltage curve. |
| Series and parallel capacitors | Total capacitance of a list of capacitors, in series and in parallel. |
| Series and parallel inductors | Total inductance of a list of coils, in series and in parallel, without mutual coupling. |
| Capacitor energy and charge | Energy, charge, voltage and capacitance of a capacitor. Any two of them give the other two. |
| Air-core coil (Wheeler) | Inductance of a single-layer air-core coil from its diameter, length and turns, or any one of them for a wanted inductance. |
| Toroid inductance (A_L) | Inductance of a toroid core from its A_L value and the number of turns, or any one of the three. |
| Wire resistance | DC resistance of a copper, aluminium or silver conductor, with temperature correction. Also solves for the length, cross-section or temperature. |

### Filters

| Calculator | What it does |
|------------|--------------|
| RC low-pass filter | Cutoff frequency and response of a first-order low-pass at a test frequency. |
| RC high-pass filter | Cutoff frequency and response of a first-order high-pass at a test frequency. |
| LC resonance | Resonant frequency of an LC circuit and its characteristic impedance. |
| RLC circuit: Q and bandwidth | Quality factor and −3 dB bandwidth of a series or parallel RLC circuit. |
| Sallen-Key low-pass (unity gain) | Second-order active low-pass with equal resistors, and the capacitor values for a Butterworth response. |
| Twin-T notch filter | Notch frequency of the passive twin-T network. |
| Pi attenuator | Resistors of a matched π pad for a system impedance and an attenuation in dB. |
| T attenuator | Resistors of a matched T pad for a system impedance and an attenuation in dB. |

### Semiconductors

| Calculator | What it does |
|------------|--------------|
| LED series resistor | Series resistor of an LED string from the supply voltage, the forward voltage and the current. Parallel strings too. |
| Zener regulator | Series resistor of a Zener shunt regulator for a load current, with the Zener power and the efficiency. |
| Transistor switch | Base resistor of an NPN transistor used as a saturated switch, with the base current. |
| BJT divider bias | DC operating point of a common-emitter stage with a four-resistor voltage-divider bias. |
| MOSFET losses | Conduction loss and switching loss of a hard-switched MOSFET, and their sum. |
| Heatsink | Thermal chain from junction to ambient. Finds the one missing value, for example the heatsink you need. |

### Power supply

| Calculator | What it does |
|------------|--------------|
| LM317 output voltage | Output voltage of an LM317 regulator from its two resistors, or the resistor for a wanted voltage. |
| LM317 constant-current source | Load current of an LM317 current source, or the resistor for a wanted current. |
| Linear regulator power loss | Heat in a linear regulator, with input power, output power and efficiency. |
| Buck converter (step-down) | Ideal step-down converter: duty cycle, ripple and peak currents, or the inductance or frequency for a wanted ripple. |
| Boost converter (step-up) | Ideal step-up converter: duty cycle, input current, ripple and peak currents, or the inductance or frequency for a wanted ripple. |
| Smoothing capacitor | Capacitor behind a half-wave or full-wave rectifier for a wanted ripple. |
| Battery runtime | Runtime of a battery from its capacity and the load current. |
| Battery charge time | Charge time from the capacity, the charge current and the charge efficiency. |
| Battery pack (series/parallel) | Voltage, capacity and energy of cells in series and in parallel, for example 3S2P. |
| Cable voltage drop | Voltage drop of a copper or aluminium cable for DC and AC, with guide values for mains installations. |

### Op-amps

| Calculator | What it does |
|------------|--------------|
| Inverting amplifier | Gain and output of an ideal inverting op-amp stage. |
| Non-inverting amplifier | Gain and output of an ideal non-inverting op-amp stage. |
| Difference amplifier | Output of a difference amplifier with matched resistor pairs, from its input voltages. |
| Summing amplifier | Output of an inverting summing amplifier for a list of input voltages. |
| Integrator | Output of an ideal inverting integrator over time, with its time constant. |
| Schmitt trigger | Thresholds, hysteresis and centre of an inverting Schmitt trigger, and the resistors for wanted thresholds. |
| Instrumentation amplifier | Gain and output of a three-op-amp instrumentation amplifier. |

### Timers

| Calculator | What it does |
|------------|--------------|
| NE555 astable (oscillator) | Frequency, high and low time and duty cycle of the NE555 oscillator. Can also size both resistors from frequency, duty cycle and capacitor. |
| NE555 monostable (one-shot) | Pulse width of the NE555 one-shot. Solves for the resistor, the capacitor or the width. |
| Crystal load capacitors | The two load capacitors of a crystal in a Pierce oscillator, with the nearest standard value. |
| Frequency, period and duty cycle | Converts between frequency, period, duty cycle, high time and low time of a rectangular signal. |
| PWM average voltage | Average and RMS voltage of a PWM signal from the duty cycle and the two levels. |

### RF

| Calculator | What it does |
|------------|--------------|
| Wavelength | Converts between frequency, velocity factor and wavelength, with the half and quarter wavelength. |
| Half-wave dipole length | Length of a half-wave dipole with its shortening factor, and the quarter-wave length. |
| Microstrip impedance | Characteristic impedance of a PCB microstrip line from its geometry and permittivity. |
| Coaxial cable impedance | Characteristic impedance of a coaxial cable from its diameters and permittivity, with the velocity factor. |
| SWR and return loss | Converts between SWR, reflection coefficient and return loss, with the reflected power. |
| RF power and voltage (dBm, dBµV) | Power, voltage and resistance of a sine wave, with the levels in dBm and dBµV. |
| Free-space path loss | Loss of a radio link from distance and frequency, or the distance or frequency for a given loss. |
| Skin depth | Skin depth of copper, silver, gold or aluminium at a frequency, with the surface resistance. |

### Digital

| Calculator | What it does |
|------------|--------------|
| ADC (voltage to code) | Output code of an ideal N-bit converter, in decimal, hexadecimal and binary. |
| DAC (code to voltage) | Output voltage of an ideal DAC for a code, or the nearest code for a wanted voltage. |
| I²C pull-up resistors | Smallest and largest pull-up resistor for a bus, with the nearest standard values. |
| Level shifter (resistor divider) | One-way divider from 5 V to 3.3 V, checked against the input-high threshold of the receiver. |
| UART baud rate error | Divisor and baud rate error from the clock and the wanted baud rate, at 16× or 8× oversampling. |
| Number systems (dec, hex, bin, oct) | One whole number shown in decimal, hexadecimal, binary and octal. |

### Wiring & PCB

| Calculator | What it does |
|------------|--------------|
| PCB trace width (IPC-2221) | Width, current or temperature rise of a copper trace on an outer or inner layer. |
| PCB trace resistance | Resistance, voltage drop and power of a copper trace. Also solves for its length, width or thickness. |
| Via current capacity | Copper cross-section, resistance and current of a plated-through via. |
| AWG wire size conversion | Converts wire diameter and cross-section between AWG, mm, mil, inch, kcmil and mil². |
| Cable cross-section finder | Finds the smallest standard cross-section for a design current from the cable current-rating tables, with the ambient temperature and grouping factors of the installation method. The result row is highlighted in the table. |
| PCB conductor spacing finder (IPC-2221) | Finds the minimum conductor spacing for a voltage from IPC-2221B or IPC-2221C table 6-1, including the rule per volt above 500 V. The result band is highlighted in the table. |

## Tables

The **Tables** category holds 44 reference tables. 40 are transcribed from standards and published sources. The other 4 are generated by ECalc from formulas: the AWG wire gauges, the PCB trace width after IPC-2221 and the two E-series tables. Open a table from the category. The list is sorted into the groups **Conductors and Cables**, **Components**, **Materials** and **General**.

On the table page:

- **Filter:** type in the filter box, or press `/` to jump to it. The filter keeps the rows in which a shown cell contains the text.
- **Sort:** click a column header to sort the rows ascending or descending, or back in table order. Empty cells always come last.
- **Units:** each column has its own unit dropdown, with the units that suit it (a cross-section can be shown as mm², kcmil or AWG). ECalc remembers your choice for that column of that table.
- **Digits:** the numbers of a table transcribed from a standard are shown with every digit they were written with (434.79 MHz, 2.4835 GHz), whatever the significant-digit setting says; the setting applies to the computed tables (AWG, IPC-2221 widths, E24 and E192) and to a value you convert into another unit.
- **Empty columns:** a column without any value is hidden. The line under the table says how many columns are hidden: "Columns without verified data are hidden: N".
- **Copy table:** copies the visible rows, in the units shown, as tab-separated text that you can paste into a spreadsheet.
- **Banners and notes:** a table marked as guide values shows a banner: the values are guide values from the cited source, and the standard itself is authoritative. A table with mains installation data shows a mains note: the applicable national rules decide, and the installation work is for qualified persons. Each table names its source and shows its notes.
- **Finders:** two calculators select a row of a table, the **Cable cross-section finder** and the **PCB conductor spacing finder (IPC-2221)**. Their card sits on the table page, shows the inputs that are still free and hides the fixed choice of the finder. The row that matches the result is highlighted and scrolled into view, and the highlight moves when you change the inputs. If the inputs lie outside the verified data, the finder shows the warning "Not covered by the verified data" and highlights no row.

### Conductors and Cables

| Table | What it shows |
|-------|---------------|
| Cable current ratings, copper, PVC (70 °C) | Current-carrying capacity of copper cables with PVC insulation for the installation methods A1 to F. |
| Cable current ratings, copper, XLPE/EPR (90 °C) | Current-carrying capacity of copper cables with XLPE or EPR insulation (90 °C). |
| Cable current ratings, aluminium, PVC (70 °C) | Current-carrying capacity of aluminium cables from 16 to 240 mm² with PVC insulation; only the confirmed cells are filled. |
| Cable current ratings, aluminium, XLPE/EPR (90 °C) | Current-carrying capacity of aluminium cables from 16 to 240 mm² with XLPE or EPR insulation; only the confirmed cells are filled. |
| Ambient temperature factors for cables in air | Correction factors for ambient temperatures from 10 to 85 °C. |
| Grouping factors for cables | Reduction factors for 1 to 20 circuits in one group, by installation arrangement. |
| Factors for multi-core cables and cords | Factors for cables and cords up to 10 mm² with more than three loaded cores. |
| Flexible cord current ratings (DIN VDE 0298-4) | Current-carrying capacity of flexible copper cords from 0.5 to 400 mm², by cord type. |
| NEC ampacities by AWG (table 310.16) | Allowable ampacities of copper and aluminium conductors from 18 AWG to 500 kcmil at 60, 75 and 90 °C. |
| Standard conductor sizes (IEC 60228) | The standard cross-sections from 0.5 to 2500 mm² with the maximum DC resistance per km. |
| PCB conductor spacing by voltage (IPC-2221B) | Minimum spacing between conductors on a printed board by voltage band. Functional spacing, not safety isolation. |
| PCB conductor spacing by voltage (IPC-2221C) | The revised IPC-2221C table (December 2023). Functional spacing, not safety isolation. |
| PCB copper weight and thickness | Copper weight in oz/ft² from 1/4 to 4 oz, with the area weight and the thickness by four conventions. |
| AWG wire gauges | Generated: the AWG sizes from 4/0 to 40, from the AWG formula of ASTM B258. |
| PCB trace width (IPC-2221) | Generated: trace widths for a grid of currents, outer and inner layers, copper weights and temperature rises. |

### Components

| Table | What it shows |
|-------|---------------|
| SMD chip sizes (resistors, capacitors) | EIA and metric size codes from 01005 to 2512 with the body size, and the typical power and voltage of chip resistors. |
| Ceramic capacitor dielectrics (C0G, X7R …) | Temperature range and capacitance behaviour of C0G, X8R, X7R, X7S, X7T, X6S, X5R and Y5V. |
| Fuse time-current characteristics (FF, F, M, T, TT) | The characteristic letters of miniature fuses with their speed order and, for F and T, the pre-arcing time at 10 times the rated current. |
| Fuse pre-arcing times (IEC 60127-2, 5×20 mm) | Minimum and maximum melting times of 5×20 mm fuses at 1.5 to 10 times the rated current. |
| Miniature fuse formats (5×20, 6.3×32, TR5) | Body diameter, length and lead spacing of three common formats. |
| Battery chemistries (voltages and energy density) | Nominal and cut-off voltages per cell and the specific energy of common chemistries. |
| Battery sizes (AA, 18650, CR2032 …) | IEC and ANSI designations, dimensions and nominal voltage of common cells. |
| DC barrel connectors (power plugs) | Outer and inner diameter, centre pin, voltage class and rated current of common DC plugs. |
| Preferred values E3 to E24 | Generated: the IEC 60063 values, with the series each value belongs to. |
| Preferred values E48 to E192 | Generated: the same for E48 to E192. |

### Materials

| Table | What it shows |
|-------|---------------|
| Resistivity of conductor and resistance materials | Resistivity at 20 °C and temperature coefficient of 22 materials, with the spread between the sources. |
| Relative permittivity and loss tangent of dielectrics | εr and tan δ of 14 materials (circuit board laminates, cable insulation, air, glass and alumina), each with its test frequency. |
| Thermocouple types (IEC 60584) | The eight standard types B, E, J, K, N, R, S and T with their materials, tolerance classes, voltages and cable colours. |
| Solder alloys | Seven common soft solders with their ISO 9453 numbers, solidus and liquidus, and their typical use. |

### General

| Table | What it shows |
|-------|---------------|
| SI prefixes | The 24 prefixes from quetta to quecto with symbol and factor. |
| SI units | The seven base units, the named units and the common derived units, each with its expression in base units. |
| Physical constants | 30 constants for electronics with the CODATA 2022 value, uncertainty and unit. |
| Greek alphabet | The 24 letters with their variant forms, names and typical uses as symbols. |
| IP codes (IEC 60529) | The characteristic numerals of the IP code for solids and water, with their meaning. |
| Protection classes (IEC 61140) | Protection classes 0, I, II and III against electric shock, with protection principle and marking. |
| Mains voltage, frequency and plugs by country | Nominal voltage, frequency and plug types of 48 countries. Guide values. |
| ISM and SRD bands | The ITU ISM bands and the EU short-range-device bands, with their limits, power and duty cycle. |
| LoRaWAN regional channel plans (EU868, US915, AS923) | Default channels and RX2 and beacon settings of the regional plans. |
| Wi-Fi channels 2.4 and 5 GHz | Centre frequencies, channel edges and availability in Germany/EU and the USA. |
| Amateur radio bands in Germany (AFuV) | The 45 frequency ranges of AFuV Annex 1 with their status and the maximum power per licence class. |
| Logic levels | Input and output voltages, currents and switching thresholds of TTL and CMOS families. |
| RS-232, RS-485 and LVDS signal levels | Driver and receiver voltages, thresholds, loads and terminations. |
| ASCII table | The codes 0 to 127 in decimal and hexadecimal, with the character or control name. |
| Wire colours (IEC/DE and US NEC) | Conductor colours for L1, L2, L3, N and PE in Germany and the EU, and for the grounded and ungrounded conductors in the USA. |

## Component Codes

The **Codes** category decodes component markings and writes them from a value. There is one page for each kind of code:

- **Resistor colour code** and **Inductor colour code**: the colour bands. Click a band to choose its colour in the menu, or use the keyboard: Tab moves to a band, Enter, Space or the down arrow opens its menu, the arrow keys move through the colours, Enter picks one and Escape closes the menu. The page shows the value, the tolerance and, for six-band resistors, the temperature coefficient, and it gives the nearest standard value of the chosen series. Inductors have four or five bands.
- **SMD resistor codes**: the three-digit code (472 is 4.7 kΩ), the four-digit code, the EIA-96 code (01C) and the R notation (4R7 is 4.7 Ω). A value that you enter shows its three-digit, four-digit and EIA-96 marking.
- **Capacitor codes**: the picofarad code (104 is 100 nF, 4p7 is 4.7 pF), the tolerance letter and the voltage code. A marking can have up to three parts, for example `104K 1H`: 100 nF, ±10 % and 50 V.
- **Ceramic capacitor temperature codes**: class 1 (temperature compensating, for example C0G or NP0) with its temperature coefficient in ppm/K and its tolerance, and class 2 (high permittivity, for example X7R or U2J) with its temperature range and the permitted capacitance change. A class 2 code has no value of its own, so the page shows no number for it.

Resistor colours follow IEC 60062:2016. Where makers still print an older tolerance reading, the page shows it as a warning. Voltage codes are a maker convention, so the page says so and the datasheet decides. Warnings and error messages come from the engine and are in English in every language.

## Pinouts

The **Pinouts** category holds 47 pinouts of connectors, board headers, IC packages and other parts with pins. Open a pinout from the category. The list is sorted into the groups **Data and video**, **Audio**, **PC power and fans**, **Boards**, **Debug and interfaces** and **ICs**; the groups are listed at the end of this section.

A pinout page shows:

- **The drawing:** ECalc draws the part from its shape and its pin list. It is a drawing, not a photo.
- **The title bar:** the name, the favorite button and **Copy pinout**.
- **Variant:** a choice of variants, on the pinouts that have them.
- **View:** a choice of plug or socket, on the pinouts that have two views. The caption next to it reads **Looking at the mating face**.
- **Pin types:** a legend of the pin types that the pinout uses.
- **Detail panel:** the function, type, wire and notes of the selected pin.
- **Pin table:** the columns **Pin**, **Name**, **Function**, **Type**, **Wire** and **Notes**. The Wire column appears only when a pin of the variant has a wire colour.
- **Notes and source:** the notes of the pinout and the source it was taken from, at the foot of the page.

**Two views.** 31 of the 47 pinouts have a **View** choice. Their two views carry the names of the item, for example *Plug (cable end)* and *Receptacle (on the host or hub)*. The drawing shows the contacts as you see them when you look at the mating face, the face that meets the other part. A plug and its socket face each other when they are mated, so seen from their mating faces they are mirror images: left and right swap. ECalc draws the view that the pinout stores as its drawn view as it is, and the other view as the mirror image. Numbers and names stay readable, but the contacts move to the other side of the drawing. Boards and ICs have one fixed view and no choice. The Raspberry Pi 40-pin header is the one board with a View choice.

**Pin colours.** Each contact is filled with the colour of its pin type. The legend names the types the pinout uses:

| Type | Colour | What it means |
|------|--------|---------------|
| Power | red | A supply voltage, such as VBUS or +5 V |
| Ground | dark slate | The return or earth connection, GND |
| Signal | amber | A control, status or analogue line, such as an audio channel, a reset or the speed signal of a fan |
| Data | blue | A line of a data link, such as the USB D+ and D- lines |
| General-purpose I/O | green | A pin that the user programs, such as a GPIO pin of a board |
| Not connected | grey | A pin that is not connected or reserved, or a key position without a pin |

**Wire colours.** A wire colour is shown only where a standard or guide names one, for example for USB Type-A, RJ45, ATX, FTDI, Qwiic, Grove and the servo cable. Some cables differ from those colours. The ATX notes say to go by position, not by colour.

**Variants.** Four pinouts have variants. RJ45 (8P8C) has T568A and T568B, which differ only in the wire colours of the green and orange pairs. The 3.5 mm headset jack (TRRS) has CTIA (AHJ) and OMTP. DisplayPort has the Source side (output) and the Sink side (monitor input), which number pins 1 to 12 differently. TO-92: examples has seven part examples: BC547, PN2222A, 2N3904, 2N7000, LM35, 78L05 and TL431.

**Hover, click and filter.** Hovering a contact in the drawing or a row in the table highlights both. A click selects the pin: its details appear in the detail panel and its row is scrolled into view. A second click clears the selection. The drawing reacts to the pointer only; with the keyboard, use the pin number button at the start of each row. The filter box above the table matches the pin number, name, function, type name, wire colour name and notes. Pins that do not match are dimmed in the drawing and their rows are hidden. Press `/` to jump to the filter box, and Escape to clear it.

**Copy pinout** copies the rows that are shown, after the filter, for the chosen variant. The text is tab-separated and starts with a header line, so you can paste it into a spreadsheet. ECalc remembers the variant and the view of each pinout in the browser, with its other settings.

**Check the view before you wire anything.** A connector seen from the other side is mirrored, and a mirrored connector is the classic mistake: left and right swap, and pin 1 sits at the other end of its row. Check which view you are looking at, and that it shows the mating face of the part you are wiring to. Clone boards can differ from the drawing; the NodeMCU entry says so and asks you to compare with the silk screen before wiring. The TO-92 page lists examples only. The pin order depends on the part and the maker, so check the datasheet of your exact part.

### Data and video

- USB 2.0 Type-A
- USB 3.x Type-A
- USB 2.0 Type-B
- USB Mini-B
- USB Micro-B
- USB Type-C
- HDMI Type A
- DisplayPort
- VGA (DE-15)
- RJ45 (8P8C)
- RS-232 (DE-9)

### Audio

- 3.5 mm jack, stereo (TRS)
- 3.5 mm headset jack (TRRS)
- XLR, 3-pin (balanced audio)
- MIDI (DIN-5, 180°)

### PC power and fans

- ATX main power, 24-pin
- EPS12V CPU power, 8-pin
- PCIe power, 6-pin (2x3)
- PCIe power, 8-pin (2x4)
- SATA data, 7-pin
- SATA power, 15-pin
- Peripheral power, 4-pin (Molex)
- Fan, 3-pin
- Fan, 4-pin PWM

### Boards

- Arduino Uno R3
- Arduino Nano
- ESP32-DevKitC V4
- NodeMCU DevKit 1.0 (ESP8266)
- Raspberry Pi 40-pin header
- Raspberry Pi Pico / Pico W

### Debug and interfaces

- ARM JTAG (20-pin)
- Cortex Debug / SWD (10-pin)
- AVR ISP (6-pin)
- FTDI TTL-232R (6-pin)
- Qwiic / STEMMA QT
- Grove (4-pin)
- Hobby servo (3-pin)
- OBD-II (SAE J1962)

### ICs

- NE555 (DIP-8)
- LM317 (TO-220)
- L78xx (TO-220)
- LM358 (DIP-8)
- LM324 (DIP-14)
- SN74HC595 (DIP-16)
- ULN2003A (DIP-16)
- ATmega328P (DIP-28)
- TO-92: examples

## Sources and Honesty

Transcribed standard tables are guide values, cross-checked against two independent sources. Cells that could not be cross-checked are left empty, and a column without any verified value is hidden. Each table names its source; check a design that matters against the standard or the datasheet. Pinouts name their sources the same way, at the foot of each page. Check the pin you wire against the datasheet of your part.

## Using ECalc from the Agent

The AuraGo agent can use ECalc's formulas, reference tables, component codes and pinouts through the `ecalc` tool. The tool computes on your AuraGo server with the same catalog
and formulas as the app. It reads nothing from the network or from files, and it has no switch of its own in **Config**.

The agent has eight operations:

- **search** finds calculators, tables and component codes by a topic, a part or a formula, in English or German. Without a search term it lists the categories.
- **describe** shows the variables of a calculator with their units and defaults, its choices, the sets of inputs that work, the
  values it computes and its checks. For a table it shows the columns and the key; for a component code, the forms it reads and writes; for a pinout, the connector, the view, the pin counts and the variants, but not the pins.
- **calculate** solves a calculator from the values you give. Any complete set of inputs works, so it can solve for whichever value
  is missing. It reports every value in SI units and as formatted text, with the calculator's warnings. It also tells you when a value
  you gave differs from the result by more than 1 %. For a computed resistor it names the nearest standard value of the chosen series,
  and the standard-value combiner returns its pairs.
- **table** reads the rows of a reference table. It filters them, pages through them (`offset`, `limit`) and answers one lookup:
  `exact` (the row whose column equals the value), `ge` (the row with the smallest cell that is at least the value) or `interp` (a value
  between the neighbouring rows). Rows come in the column units, with the SI values for further calculation.
- **pinout** lists the pins of a connector, a board header or an IC package, in pin order. Each pin has its number, name, function,
  type and, where a standard names one, its wire colour. Give `pin` for one pin, by number (`7`) or by name (`GND`; several pins can
  share a name), `variant` for a variant such as `t568b`, or `filter` for a word. The answer also says what the drawing shows (`view`);
  read it before you tell the user where a pin sits.
- **decode** reads a component code, for example the colour bands of a resistor, and reports its value and fields. **encode** writes
  one from a value: it picks the nearest standard value of the series that the tolerance implies and returns the matching marking
  or bands.
- **open** shows a calculator in the ECalc app on the desktop, with the values filled in as if you had typed them. The answer
  computes nothing; for the numbers the agent uses **calculate**. Names and values are checked one by one, but the values are not
  checked against each other, so a set of values that contradict each other opens as typed. For a pinout, **open** opens the pinout's page.

**open** needs the Virtual Desktop to be switched on, and **Allow agent control** to be enabled, in **Config → Integrations →
Virtual Desktop**. Without them the agent gets a "policy denied" answer and gives you the numbers from **calculate** instead.
Other agents that reach AuraGo through A2A (agent-to-agent requests) may use search, describe, calculate, table, pinout, decode and encode, all of which only read, but they cannot open ECalc.

Prompts you can type:

- "What resistor do I need for a red LED at 20 mA on 5 V?" The agent calculates the series resistor with the **LED series resistor**
  calculator and reports the nearest standard value.
- "Open ECalc with the 555 astable for 1 kHz." The agent opens the **NE555 astable (oscillator)** calculator in the Virtual Desktop with
  the frequency filled in, if the Virtual Desktop and agent control are switched on.
- "Which AWG size has a diameter of about 2 mm?" The agent reads the **AWG wire gauges** table with a lookup and names the matching size.
- "What resistor is brown black red gold?" The agent decodes the colour bands with `decode` and reports the value and tolerance.
- "Which pin of a USB-A plug is ground?" The agent reads the pins of **USB 2.0 Type-A** with `pinout` and names pin 4, GND.

## Privacy and Safety

- **On your device.** The calculations in the app run in your browser. ECalc loads its catalog from your AuraGo server and sends none of
  your values anywhere. It loads no external fonts, libraries or services.
- **Agent tool.** The `ecalc` tool computes on your AuraGo server with the catalog built into AuraGo. It reads nothing from the network
  or from files. **open** works only while the Virtual Desktop is switched on and agent control is allowed.
- **Read-only.** ECalc only reads. A read-only Virtual Desktop does not change how it works.
- **Check what matters.** Each calculator names its source under **Explanation**. The results are only as good as the formula
  and your inputs. Check a design that matters against the datasheets of your parts. For a pinout, check the pin against the datasheet of your part before you wire or solder. Mains voltage is dangerous, and ECalc is
  no substitute for qualified electrical work.
- **Mains installations.** The **Cable voltage drop** calculator gives guide values for mains installations. Its page says that the
  design of a mains installation must follow the installation standard (for example DIN VDE 0100-520 or IEC 60364-5-52) and the local
  rules, and that a qualified electrician must carry it out and check it.

## When Something Goes Wrong

| Problem | What helps |
|---------|------------|
| "The ECalc catalog could not be loaded." | Click **Try again**. If it keeps failing, check that AuraGo is running and that you are still signed in, then reload the Virtual Desktop. |
| A field shows a message instead of a value | See the table in [Entering Values](#entering-values). |
| A number-system field shows an error | The digits must match the base: 0 and 1 for binary, 0 to 7 for octal, 0 to 9 and A to F for hexadecimal. The value must stay between 0 and 2^53 − 1 (9 007 199 254 740 991). The prefix of the field's base (`0x`, `0b` or `0o`) is optional. |
| A field is dimmed | The selected option does not use it. For DC in the **Cable voltage drop** calculator, cos φ is dimmed. Choose an option that uses the field if you need it. |
| The agent cannot open ECalc ("policy denied") | Switch on the Virtual Desktop and **Allow agent control** in **Config → Integrations → Virtual Desktop**. Until then the agent can still give you the numbers from **calculate**. |
| The favorites or the last values are gone | They live in the browser. A different browser, a private window or cleared browser data starts with empty ones. |
| The E-series value is far from the calculated one | Choose a series with more values, such as E96 or E192. |
| A table column is hidden | The column has no verified data: its cells could not be cross-checked, so they are left empty and the column is hidden. The line under the table says how many columns are hidden. |
| A finder shows a warning ("Not covered by the verified data") | The inputs lie outside the verified data, or no second source confirmed a value for this combination. Choose another combination; the finder highlights no row. |
| A code is rejected as "not cross-checked" or "ambiguous" | The symbol is not in the verified data ("not cross-checked"), or makers read it in conflicting ways ("ambiguous"). Read the marking on the part and check the datasheet. In the agent tool the answer has the code `unsupported`. |
| The pin table says "No pin matches the filter." | The filter text is not in the number, name, function, type, wire colour or notes of any pin of this variant. Clear the box, or press Escape in it. |
| The **Wire** column is missing | No pin of the variant has a wire colour, because the source gives none. The column appears only when a pin has one. |

---

**See also:** [Chapter 4: Web UI](./04-webui.md) · [Chapter 8: Integrations](./08-integrations.md) · [Chapter 14: Security](./14-security.md) · [Chapter 16: Troubleshooting](./16-troubleshooting.md) · [Chapter 22: Internal Tools](./22-internal-tools.md)

---

**Previous Chapter:** [Chapter 31: Pixel](./31-pixel.md)
