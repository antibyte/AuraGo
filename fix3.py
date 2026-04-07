import sys
lines = open(r"c:\Users\Andi\Documents\repo\AuraGo\ui\js\skills\main.js", "r", encoding="utf-8").readlines()
fixed = []
for i, line in enumerate(lines):
    if i >= 106 and line.startswith("    "):
        fixed.append(line[4:])
    else:
        fixed.append(line)
open(r"c:\Users\Andi\Documents\repo\AuraGo\ui\js\skills\main.js", "w", encoding="utf-8").writelines(fixed)
print("Done")
