# Chapter 32: ECalc – electronics formulas in the Virtual Desktop

ECalc is a formula collection for electronics in the Virtual Desktop. A calculator page shows the circuit, the formula and the results side by side, and it recalculates while you type. You enter the values you know and ECalc works out the rest. For resistors it also suggests the nearest standard value from an E series, and the standard-value combiner finds standard values that reach a target resistance, in series or in parallel.

ECalc has 77 calculators in ten categories. The agent can calculate with the same formulas and open a calculator for you (see [Using ECalc from the Agent](#using-ecalc-from-the-agent)). Reference tables, component codes and pinouts come in later versions (see [What Comes Next](#what-comes-next)). ECalc runs in your browser. It needs no provider and has no switch of its own in **Config**.

## Table of Contents
1. [Opening ECalc](#opening-ecalc)
2. [Finding a Calculator](#finding-a-calculator)
3. [The Calculator Page](#the-calculator-page)
4. [Entering Values](#entering-values)
5. [Standard Values (E Series)](#standard-values-e-series)
6. [Settings](#settings)
7. [The Calculators](#the-calculators)
8. [Using ECalc from the Agent](#using-ecalc-from-the-agent)
9. [What Comes Next](#what-comes-next)
10. [Privacy and Safety](#privacy-and-safety)
11. [When Something Goes Wrong](#when-something-goes-wrong)

---

## Opening ECalc

Open **ECalc** from the start menu, in the category **Development**. The window opens at about 1100 × 760 pixels and you can
resize it. ECalc reopens on the calculator you used last.

The window has three columns: a **category rail** on the left, a **list** of calculators in the middle and the **calculator
page** on the right. In a narrow window (below about 720 pixels) the three become one stack. Choose a category, then a
calculator, and use **Back** to return to the previous level.

## Finding a Calculator

- **Category rail:** **Favorites** and **Recent** come first, followed by the categories that hold calculators: Basics, Passive,
  Filters, Semiconductors, Power supply, Op-amps, Timers, RF, Digital and Wiring & PCB. Each category shows how many calculators
  it holds. The rail lists a category only when it has content, so Tables, Codes and Pinouts will appear once they are added.
- **Search:** type in the search box at the top to find a calculator by its name in your language, by its English name or by a
  keyword such as "divider". Each calculator has keywords, and the search matches them too.
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

ECalc has 77 calculators in ten categories. Each table lists the calculators of one category, in the order of the category rail.
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

## Using ECalc from the Agent

The AuraGo agent can use ECalc's formulas through the `ecalc` tool. The tool computes on your AuraGo server with the same catalog
and formulas as the app. It reads nothing from the network or from files, and it has no switch of its own in **Config**.

The agent has four operations:

- **search** finds calculators by a topic, a part or a formula, in English or German. Without a search term it lists the categories.
- **describe** shows the variables of a calculator with their units and defaults, its choices, the sets of inputs that work, the
  values it computes and its checks.
- **calculate** solves a calculator from the values you give. Any complete set of inputs works, so it can solve for whichever value
  is missing. It reports every value in SI units and as formatted text, with the calculator's warnings. It also tells you when a value
  you gave differs from the result by more than 1 %. For a computed resistor it names the nearest standard value of the chosen series,
  and the standard-value combiner returns its pairs.
- **open** shows a calculator in the ECalc app on the desktop, with the values filled in as if you had typed them. The answer
  computes nothing; for the numbers the agent uses **calculate**. Names and values are checked one by one, but the values are not
  checked against each other, so a set of values that contradict each other opens as typed.

**open** needs the Virtual Desktop to be switched on, and **Allow agent control** to be enabled, in **Config → Integrations →
Virtual Desktop**. Without them the agent gets a "policy denied" answer and gives you the numbers from **calculate** instead.
Other agents that reach AuraGo through A2A (agent-to-agent requests) may use search, describe and calculate, but they cannot open
ECalc.

Two prompts you can type:

- "What resistor do I need for a red LED at 20 mA on 5 V?" The agent calculates the series resistor with the **LED series resistor**
  calculator and reports the nearest standard value.
- "Open ECalc with the 555 astable for 1 kHz." The agent opens the **NE555 astable (oscillator)** calculator in the Virtual Desktop with
  the frequency filled in, if the Virtual Desktop and agent control are switched on.

## What Comes Next

The catalog grows in later versions:

- **Reference tables**, for example the current capacity of cables and PCB traces.
- **Component codes**, for example the resistor colour code and SMD codes.
- **Pinouts** of connectors and ICs.
- **Finders that read the tables**, for example a cable cross-section for a given current according to VDE, and a trace spacing
  for a given voltage.

This chapter grows with them.

## Privacy and Safety

- **On your device.** The calculations in the app run in your browser. ECalc loads its catalog from your AuraGo server and sends none of
  your values anywhere. It loads no external fonts, libraries or services.
- **Agent tool.** The `ecalc` tool computes on your AuraGo server with the catalog built into AuraGo. It reads nothing from the network
  or from files. **open** works only while the Virtual Desktop is switched on and agent control is allowed.
- **Read-only.** ECalc only reads. A read-only Virtual Desktop does not change how it works.
- **Check what matters.** Each calculator names its source under **Explanation**. The results are only as good as the formula
  and your inputs. Check a design that matters against the datasheets of your parts. Mains voltage is dangerous, and ECalc is
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

---

**See also:** [Chapter 4: Web UI](./04-webui.md) · [Chapter 8: Integrations](./08-integrations.md) · [Chapter 14: Security](./14-security.md) · [Chapter 16: Troubleshooting](./16-troubleshooting.md) · [Chapter 22: Internal Tools](./22-internal-tools.md)

---

**Previous Chapter:** [Chapter 31: Pixel](./31-pixel.md)
