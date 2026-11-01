# Chapter 32: ECalc – electronics formulas in the Virtual Desktop

ECalc is a formula collection for electronics in the Virtual Desktop. A calculator page shows the circuit, the formula and the
results side by side, and it recalculates while you type. You enter the values you know and ECalc works out the rest. For
resistors it also suggests the nearest standard value from an E series.

This is the first version. It has 11 calculators in the **Basics** category. Tables, component codes, pinouts and an agent tool
follow in later phases (see [What Comes Next](#what-comes-next)). ECalc runs in your browser. It needs no provider and has no
switch in **Config**.

## Table of Contents
1. [Opening ECalc](#opening-ecalc)
2. [Finding a Calculator](#finding-a-calculator)
3. [The Calculator Page](#the-calculator-page)
4. [Entering Values](#entering-values)
5. [Standard Values (E Series)](#standard-values-e-series)
6. [Settings](#settings)
7. [The Calculators](#the-calculators)
8. [What Comes Next](#what-comes-next)
9. [Privacy and Safety](#privacy-and-safety)
10. [When Something Goes Wrong](#when-something-goes-wrong)

---

## Opening ECalc

Open **ECalc** from the start menu, in the category **Development**. The window opens at about 1100 × 760 pixels and you can
resize it. ECalc reopens on the calculator you used last.

The window has three columns: a **category rail** on the left, a **list** of calculators in the middle and the **calculator
page** on the right. In a narrow window (below about 720 pixels) the three become one stack. Choose a category, then a
calculator, and use **Back** to return to the previous level.

## Finding a Calculator

- **Category rail:** **Favorites** and **Recent** come first, followed by the calculator categories: Basics, Passive, Filters,
  Semiconductors, Power supply, Op-amps, Timers, RF, Digital and Wiring & PCB. In this version only **Basics** holds
  calculators.
- **Search:** type in the search box at the top to find a calculator by its name in your language, by its English name or by a
  keyword such as "divider".
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
- **Derived values:** further quantities that follow from the result, for example the current through a divider and its power.
- **Warnings:** a note when the values do not make sense, for example an output voltage that is not below the input voltage.
- **Explanation:** a collapsible text about the calculator, with its **Source**: the standard, datasheet or textbook behind the
  formula.

**Copy results** puts the values of the page on the clipboard. **Reset to defaults** restores the starting values of the
calculator.

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
- **Units:** the dropdown next to a field chooses the unit. **Auto** picks a suitable prefix, such as mA or kΩ. Some fields offer
  special units, such as dBm, dBV and dBu for levels, or mAh, Ah and Wh for charge and energy.
- **Lists:** the series and parallel calculator takes a list such as `4k7, 10k, 2k2`. Separate the entries with commas,
  semicolons or spaces. Lists use a decimal point only.
- **Display:** the results show a decimal comma in German, Czech, Danish, Greek, Spanish, French, Italian, Dutch, Norwegian,
  Polish, Portuguese and Swedish, and a decimal point in the other languages.

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

## Settings

The settings menu has two options:

- **Significant digits:** how many digits the results show, from 3 to 6. The default is 4.
- **Schematic symbols:** **IEC** draws the resistor as a rectangle (the default). **ANSI** draws the zigzag resistor.

ECalc keeps its settings, your favorites, the recent list, the units you chose and the last values of each calculator in the
browser, under the key `aurago.ecalc.v1`. They stay in that browser. Nothing is stored on the AuraGo server, and clearing the
browser data resets them. ECalc works without browser storage too, it just forgets everything when you close the window.

## The Calculators

All calculators of this version are in the **Basics** category.

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

## What Comes Next

The catalog grows in later phases:

- **More calculators** in the categories Passive, Filters, Semiconductors, Power supply, Op-amps, Timers, RF, Digital and
  Wiring & PCB.
- **Reference tables**, for example the current capacity of cables and PCB traces.
- **Component codes**, for example the resistor colour code and SMD codes.
- **Pinouts** of connectors and ICs.
- **The `ecalc` agent tool.** The AuraGo agent will use the same formulas to calculate values, and it will be able to open
  ECalc on a page with values already filled in.

This chapter grows with them.

## Privacy and Safety

- **On your device.** The calculations run in your browser. ECalc loads its catalog from your AuraGo server and sends none of
  your values anywhere. It loads no external fonts, libraries or services.
- **Read-only.** ECalc only reads. A read-only Virtual Desktop does not change how it works.
- **Check what matters.** Each calculator names its source under **Explanation**. The results are only as good as the formula
  and your inputs. Check a design that matters against the datasheets of your parts. Mains voltage is dangerous, and ECalc is
  no substitute for qualified electrical work.

## When Something Goes Wrong

| Problem | What helps |
|---------|------------|
| "The ECalc catalog could not be loaded." | Click **Try again**. If it keeps failing, check that AuraGo is running and that you are still signed in, then reload the Virtual Desktop. |
| A field shows a message instead of a value | See the table in [Entering Values](#entering-values). |
| The favorites or the last values are gone | They live in the browser. A different browser, a private window or cleared browser data starts with empty ones. |
| The E-series value is far from the calculated one | Choose a series with more values, such as E96 or E192. |

---

**See also:** [Chapter 4: Web UI](./04-webui.md) · [Chapter 14: Security](./14-security.md) · [Chapter 16: Troubleshooting](./16-troubleshooting.md)

---

**Previous Chapter:** [Chapter 31: Pixel](./31-pixel.md)
