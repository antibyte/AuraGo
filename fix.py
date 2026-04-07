import sys
lines = open(r"c:\Users\Andi\Documents\repo\AuraGo\ui\js\skills\main.js", "r", encoding="utf-8").readlines()
fixed = [line[4:] if i >= 106 and line.startswith("    ") else line for i, line in enumerate(lines)]
open(r"c:\Users\Andi\Documents\repo\AuraGo\ui\js\skills\main.js", "w", encoding="utf-8").writelines(fixed)
print("Done")
