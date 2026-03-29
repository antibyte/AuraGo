#!/usr/bin/env python3
"""
CSS Analysis Script for AuraGo UI
Analyzes CSS files for duplication, inconsistencies, and maintainability issues.
"""

import os
import re
import json
from collections import defaultdict, Counter
from pathlib import Path

CSS_DIR = Path("ui/css")
REPORT_PATH = Path("reports/css_analysis_report.md")

def read_css_files():
    files = {}
    for f in sorted(CSS_DIR.glob("*.css")):
        files[f.name] = f.read_text(encoding="utf-8")
    return files

def extract_selectors(content):
    """Extract CSS selectors (rough but useful for duplication detection)."""
    # Remove comments
    content = re.sub(r"/\*.*?\*/", "", content, flags=re.DOTALL)
    # Extract blocks
    blocks = re.findall(r"([^{]+)\{([^}]*)\}", content)
    result = {}
    for selector, rules in blocks:
        sel = selector.strip()
        if sel.startswith("@"):
            continue
        result[sel] = rules.strip()
    return result

def extract_keyframes(content):
    """Extract @keyframes definitions."""
    return re.findall(r"@keyframes\s+([A-Za-z0-9_-]+)\s*\{[^}]*(?:\{[^}]*\}[^}]*)*\}", content, flags=re.DOTALL)

def extract_css_variables(content):
    """Extract CSS custom property definitions (--*)."""
    defs = re.findall(r"(--[A-Za-z0-9_-]+)\s*:\s*([^;]+);", content)
    uses = re.findall(r"var\((--[A-Za-z0-9_-]+)\)", content)
    return {"definitions": defs, "uses": uses}

def extract_hardcoded_colors(content):
    """Find hardcoded color values."""
    patterns = [
        r"#([0-9A-Fa-f]{3,8})\b",
        r"\brgb\(\s*\d+\s*,\s*\d+\s*,\s*\d+\s*\)",
        r"\brgba\(\s*\d+\s*,\s*\d+\s*,\s*\d+\s*,\s*[\d.]+\s*\)",
        r"\bhsl\(\s*\d+\s*,\s*\d+%\s*,\s*\d+%\s*\)",
        r"\bhsla\(\s*\d+\s*,\s*\d+%\s*,\s*\d+%\s*,\s*[\d.]+\s*\)",
    ]
    colors = []
    for p in patterns:
        colors.extend(re.findall(p, content))
    return colors

def extract_zindex(content):
    return re.findall(r"z-index\s*:\s*(-?\d+)\s*!*important*", content)

def extract_important(content):
    return re.findall(r"!important", content)

def extract_font_families(content):
    return re.findall(r"font-family\s*:\s*([^;]+);", content)

def extract_comments(content):
    return re.findall(r"/\*.*?\*/", content, flags=re.DOTALL)

def extract_units(content):
    """Extract numeric values with units."""
    units = re.findall(r"(?<![#A-Za-z0-9_-])(-?\d*\.?\d+)(px|rem|em|%|vh|vw|pt|cm|mm|in|ex|ch|vmin|vmax|fr)\b", content)
    return units

def extract_media_queries(content):
    return re.findall(r"@media\s+([^{]+)\{", content)

def extract_dead_code(content):
    """Find commented-out CSS rules (blocks inside comments)."""
    comments = re.findall(r"/\*([^*]*(?:\*(?!/)[^*]*)*)\*/", content, flags=re.DOTALL)
    dead = []
    for c in comments:
        if "{" in c and "}" in c:
            dead.append(c.strip()[:200])
    return dead

def analyze():
    files = read_css_files()
    
    report = []
    report.append("# AuraGo CSS Architecture Analysis Report")
    report.append(f"\n**Date:** 2026-03-29")
    report.append(f"**Files Analyzed:** {len(files)}")
    report.append(f"**Total CSS Size:** {sum(len(v) for v in files.values()):,} bytes\n")
    
    # --- Quantitative Analysis ---
    report.append("## 1. Quantitative Findings\n")
    
    total_important = 0
    total_colors = 0
    total_zindex = 0
    total_media = 0
    unit_counter = Counter()
    file_stats = {}
    all_keyframes = defaultdict(list)
    all_vars_defs = defaultdict(list)
    all_vars_uses = defaultdict(list)
    all_selectors = defaultdict(list)
    all_fonts = defaultdict(list)
    all_rules = defaultdict(list)
    zindex_values = []
    
    for fname, content in files.items():
        selectors = extract_selectors(content)
        keyframes = extract_keyframes(content)
        vars_data = extract_css_variables(content)
        colors = extract_hardcoded_colors(content)
        zidx = extract_zindex(content)
        imp = extract_important(content)
        fonts = extract_font_families(content)
        units = extract_units(content)
        media = extract_media_queries(content)
        dead = extract_dead_code(content)
        comments = extract_comments(content)
        
        total_important += len(imp)
        total_colors += len(colors)
        total_zindex += len(zidx)
        total_media += len(media)
        zindex_values.extend([int(x) for x in zidx])
        
        for u in units:
            unit_counter[u[1]] += 1
        
        for kf in keyframes:
            all_keyframes[kf].append(fname)
        
        for vd in vars_data["definitions"]:
            all_vars_defs[vd[0]].append((fname, vd[1].strip()))
        for vu in vars_data["uses"]:
            all_vars_uses[vu].append(fname)
        
        for sel, rules in selectors.items():
            all_selectors[sel].append(fname)
            all_rules[(fname, sel)] = rules
        
        for f in fonts:
            all_fonts[f.strip()].append(fname)
        
        file_stats[fname] = {
            "size": len(content),
            "selectors": len(selectors),
            "keyframes": len(keyframes),
            "var_defs": len(vars_data["definitions"]),
            "var_uses": len(vars_data["uses"]),
            "colors": len(colors),
            "zindex": len(zidx),
            "important": len(imp),
            "fonts": len(fonts),
            "media_queries": len(media),
            "dead_code_snippets": len(dead),
            "comments": len(comments),
        }
    
    # Table of file stats
    report.append("### Per-File Statistics\n")
    report.append("| File | Size (bytes) | Selectors | Keyframes | Var Defs | Var Uses | Hardcoded Colors | z-index | !important | Media Queries | Dead Code Blocks |")
    report.append("|------|-------------:|----------:|----------:|---------:|---------:|-----------------:|--------:|-----------:|--------------:|-----------------:|")
    for fname in sorted(file_stats):
        s = file_stats[fname]
        report.append(f"| {fname} | {s['size']:,} | {s['selectors']} | {s['keyframes']} | {s['var_defs']} | {s['var_uses']} | {s['colors']} | {s['zindex']} | {s['important']} | {s['media_queries']} | {s['dead_code_snippets']} |")
    
    report.append(f"\n**Totals:** {total_important} `!important`, {total_colors} hardcoded colors, {total_zindex} z-index declarations, {total_media} media queries.\n")
    
    # Unit usage
    report.append("### Unit Usage Distribution\n")
    report.append("| Unit | Count |")
    report.append("|------|------:|")
    for unit, count in unit_counter.most_common():
        report.append(f"| {unit} | {count:,} |")
    report.append("")
    
    # --- Duplication Analysis ---
    report.append("## 2. Duplication & Conflicts\n")
    
    # Duplicate keyframes
    dup_keyframes = {k: v for k, v in all_keyframes.items() if len(v) > 1}
    if dup_keyframes:
        report.append("### Duplicate @keyframes\n")
        for kf, fnames in dup_keyframes.items():
            report.append(f"- `@keyframes {kf}` found in: {', '.join(fnames)}")
        report.append("")
    
    # Duplicate selectors across files
    dup_selectors = {k: v for k, v in all_selectors.items() if len(v) > 1}
    if dup_selectors:
        report.append(f"### Duplicate Selectors Across Files ({len(dup_selectors)} instances)\n")
        # Show top 20 most duplicated
        for sel, fnames in sorted(dup_selectors.items(), key=lambda x: -len(x[1]))[:30]:
            report.append(f"- `{sel}` in {len(fnames)} files: {', '.join(fnames)}")
        report.append("")
    
    # CSS variables redefined across files
    dup_vars = {}
    for var, defs in all_vars_defs.items():
        files_with_var = list(set([d[0] for d in defs]))
        if len(files_with_var) > 1:
            dup_vars[var] = defs
    if dup_vars:
        report.append("### CSS Variables Redefined Across Files\n")
        for var, defs in sorted(dup_vars.items()):
            report.append(f"- `{var}`:")
            seen_files = set()
            for fname, val in defs:
                if fname not in seen_files:
                    report.append(f"  - `{fname}`: `{val}`")
                    seen_files.add(fname)
        report.append("")
    
    # --- Inconsistencies ---
    report.append("## 3. Critical Inconsistencies\n")
    
    # z-index analysis
    if zindex_values:
        report.append("### z-index Values\n")
        report.append(f"Range: {min(zindex_values)} to {max(zindex_values)}")
        zc = Counter(zindex_values)
        report.append("Unique values: " + ", ".join([f"{k} ({v}x)" for k, v in zc.most_common(20)]))
        report.append("")
    
    # Font families
    if all_fonts:
        report.append("### font-family Declarations\n")
        for font, fnames in sorted(all_fonts.items(), key=lambda x: -len(x[1])):
            report.append(f"- `{font}` in {len(fnames)} files: {', '.join(sorted(set(fnames)))}")
        report.append("")
    
    # Hardcoded colors by file (top offenders)
    report.append("### Hardcoded Colors by File (Top 10)\n")
    color_by_file = {k: len(extract_hardcoded_colors(v)) for k, v in files.items()}
    for fname, count in sorted(color_by_file.items(), key=lambda x: -x[1])[:10]:
        report.append(f"- {fname}: {count} hardcoded colors")
    report.append("")
    
    # Media query breakpoints
    all_media = []
    for fname, content in files.items():
        all_media.extend(extract_media_queries(content))
    if all_media:
        report.append("### Media Query Breakpoints\n")
        media_counter = Counter([m.strip() for m in all_media])
        for mq, count in media_counter.most_common(20):
            report.append(f"- `{mq}`: {count} occurrences")
        report.append("")
    
    # --- File-level issues ---
    report.append("## 4. File-Level Issues\n")
    for fname, content in files.items():
        issues = []
        
        # Dead code
        dead = extract_dead_code(content)
        if dead:
            issues.append(f"- **Dead/commented-out code:** {len(dead)} blocks")
            for d in dead[:3]:
                issues.append(f"  ```css\n  /* {d[:150]}... */\n  ```")
        
        # !important density
        imp_count = len(extract_important(content))
        if imp_count > 20:
            issues.append(f"- **High `!important` usage:** {imp_count} occurrences")
        
        # No CSS variables but many colors
        vars_data = extract_css_variables(content)
        colors = extract_hardcoded_colors(content)
        if len(vars_data["definitions"]) == 0 and len(colors) > 50:
            issues.append(f"- **No CSS variable definitions but {len(colors)} hardcoded colors** — lacks theming support")
        
        # Large file
        if len(content) > 50000:
            issues.append(f"- **Very large file:** {len(content):,} bytes — consider splitting")
        
        # Mixed units heavily
        units = extract_units(content)
        unit_types = set(u[1] for u in units)
        if len(unit_types) > 6:
            issues.append(f"- **Many different units:** {', '.join(sorted(unit_types))}")
        
        if issues:
            report.append(f"### {fname}")
            report.extend(issues)
            report.append("")
    
    # --- Recommendations ---
    report.append("## 5. Recommendations for Standardization\n")
    report.append("""
1. **Consolidate CSS Variables:** Create a single `variables.css` or `theme.css` that defines all colors, spacing, typography, and z-index scales. Many files define overlapping variables (e.g., `--primary-color`, `--accent-color`) with different values.

2. **Eliminate Duplicated Keyframes:** Move shared animations (e.g., `fadeIn`, `slideIn`, `pulse`) to a dedicated `animations.css` file.

3. **Standardize z-index Scale:** Define a z-index scale in variables (e.g., `--z-dropdown: 100`, `--z-modal: 200`, `--z-toast: 300`) to prevent layering conflicts.

4. **Reduce !important:** With a total of {important_count} `!important` declarations, specificity wars are likely. Refactor to use more specific selectors or utility classes.

5. **Unify Breakpoints:** Media queries use inconsistent breakpoints. Standardize on a set like `480px`, `768px`, `1024px`, `1440px`.

6. **Adopt rem/em for Typography:** px is dominant for font sizes. Switch to rem for accessibility and consistency.

7. **Create a Base CSS File:** Many files repeat base styles (`box-sizing`, `font-family`, `body` resets). A single `base.css` should handle this.

8. **Remove Dead Code:** Several files contain large commented-out CSS blocks. These should be removed to reduce confusion and file size.

9. **Dark/Light Theme Support:** Files with zero CSS variable definitions and many hardcoded colors cannot support theme switching. Migrate colors to variables.

10. **Font Family Consolidation:** Standardize on one or two font stacks and define them as variables.
""".format(important_count=total_important))
    
    # --- Specific snippets ---
    report.append("## 6. Specific Code Snippets of Concern\n")
    
    # Find selectors that appear in many files with potentially different rules
    report.append("### Highly Duplicated Selectors (Top 20)\n")
    for sel, fnames in sorted(dup_selectors.items(), key=lambda x: -len(x[1]))[:20]:
        report.append(f"```css\n/* Found in {len(fnames)} files */\n{sel} {{ ... }}\n```")
    report.append("")
    
    # Find files with most !important
    report.append("### Files with Highest !important Density\n")
    for fname, content in sorted(files.items(), key=lambda x: -len(extract_important(x[1])))[:5]:
        count = len(extract_important(content))
        report.append(f"- **{fname}**: {count} `!important` declarations")
    report.append("")
    
    # Write report
    REPORT_PATH.parent.mkdir(parents=True, exist_ok=True)
    REPORT_PATH.write_text("\n".join(report), encoding="utf-8")
    print(f"Report written to {REPORT_PATH}")

if __name__ == "__main__":
    analyze()
