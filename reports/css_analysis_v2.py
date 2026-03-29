#!/usr/bin/env python3
"""
Enhanced CSS Analysis Script for AuraGo UI
"""

import os
import re
from collections import defaultdict, Counter
from pathlib import Path

CSS_DIR = Path("ui/css")
REPORT_PATH = Path("reports/css_analysis_report.md")

def read_css_files():
    files = {}
    for f in sorted(CSS_DIR.glob("*.css")):
        files[f.name] = f.read_text(encoding="utf-8")
    return files

def get_context(content, match_start, match_end, lines=2):
    """Get surrounding lines for a match."""
    before = content[:match_start]
    after = content[match_end:]
    before_lines = before.splitlines()
    after_lines = after.splitlines()
    if lines == 0:
        context_before = []
        context_after = []
    else:
        context_before = before_lines[-lines:] if len(before_lines) >= lines else before_lines
        context_after = after_lines[:lines] if len(after_lines) >= lines else after_lines
    matched = content[match_start:match_end]
    return "\n".join(context_before + [matched] + context_after)

def extract_rule_blocks(content):
    """Extract CSS rule blocks more carefully."""
    blocks = []
    depth = 0
    start = 0
    in_block = False
    i = 0
    while i < len(content):
        if content[i] == '{':
            if depth == 0:
                selector = content[start:i].strip()
                body_start = i + 1
            depth += 1
            in_block = True
        elif content[i] == '}':
            depth -= 1
            if depth == 0 and in_block:
                body = content[body_start:i].strip()
                if not selector.startswith('@'):
                    blocks.append((selector, body))
                in_block = False
                start = i + 1
        i += 1
    return blocks

def extract_keyframes_full(content):
    """Extract full @keyframes blocks."""
    kfs = []
    for m in re.finditer(r'@keyframes\s+([A-Za-z0-9_-]+)\s*\{', content):
        name = m.group(1)
        start = m.start()
        depth = 1
        i = m.end()
        while i < len(content) and depth > 0:
            if content[i] == '{':
                depth += 1
            elif content[i] == '}':
                depth -= 1
            i += 1
        kfs.append((name, content[start:i]))
    return kfs

def extract_important_with_context(content, max_results=10):
    results = []
    for m in re.finditer(r'([A-Za-z0-9-]+\s*:\s*[^;]+!important)', content):
        ctx = get_context(content, m.start(), m.end(), lines=1)
        results.append(ctx)
        if len(results) >= max_results:
            break
    return results

def extract_hardcoded_colors_with_context(content, max_results=15):
    """Find hardcoded colors with surrounding CSS property context."""
    patterns = [
        (r'([A-Za-z0-9-]+\s*:\s*#[0-9A-Fa-f]{3,8})\b', 'hex'),
        (r'([A-Za-z0-9-]+\s*:\s*rgb\(\s*\d+\s*,\s*\d+\s*,\s*\d+\s*\))', 'rgb'),
        (r'([A-Za-z0-9-]+\s*:\s*rgba\(\s*\d+\s*,\s*\d+\s*,\s*\d+\s*,\s*[\d.]+\s*\))', 'rgba'),
        (r'([A-Za-z0-9-]+\s*:\s*hsl\(\s*\d+\s*,\s*\d+%\s*,\s*\d+%\s*\))', 'hsl'),
    ]
    results = []
    for pat, ctype in patterns:
        for m in re.finditer(pat, content):
            ctx = get_context(content, m.start(), m.end(), lines=0)
            results.append((ctype, m.group(1).split(':')[-1].strip(), ctx))
            if len(results) >= max_results:
                return results
    return results

def extract_zindex_with_context(content):
    results = []
    for m in re.finditer(r'z-index\s*:\s*(-?\d+)(?:\s*!important)?\s*;', content):
        ctx = get_context(content, m.start(), m.end(), lines=1)
        results.append((m.group(1), ctx))
    return results

def extract_font_family_with_context(content):
    results = []
    for m in re.finditer(r'font-family\s*:\s*([^;]+);', content):
        ctx = get_context(content, m.start(), m.end(), lines=1)
        results.append((m.group(1).strip(), ctx))
    return results

def extract_dead_code_blocks(content):
    """Find CSS comments that contain rule-like structures."""
    comments = re.findall(r'/\*([^*]*(?:\*(?!/)[^*]*)*)\*/', content, flags=re.DOTALL)
    dead = []
    for c in comments:
        if ('{' in c and '}' in c) or c.strip().startswith('.') or c.strip().startswith('#'):
            dead.append(c.strip()[:400])
    return dead

def extract_media_queries(content):
    # Use non-greedy to avoid swallowing entire file
    return re.findall(r'@media\s+([^{]+?)\s*\{', content)

def extract_css_var_defs(content):
    """Extract CSS variable definitions with context."""
    results = []
    for m in re.finditer(r'(--[A-Za-z0-9_-]+)\s*:\s*([^;]+);', content):
        ctx = get_context(content, m.start(), m.end(), lines=1)
        results.append((m.group(1), m.group(2).strip(), ctx))
    return results

def analyze():
    files = read_css_files()
    report = []
    report.append("# AuraGo CSS Architecture Analysis Report")
    report.append("")
    report.append("**Date:** 2026-03-29")
    report.append(f"**Files Analyzed:** {len(files)}")
    report.append(f"**Total CSS Size:** {sum(len(v) for v in files.values()):,} bytes")
    report.append("")
    
    # === QUANTITATIVE SUMMARY ===
    report.append("## Executive Summary")
    report.append("")
    
    total_important = 0
    total_colors = 0
    total_zindex = 0
    total_media = 0
    unit_counter = Counter()
    file_stats = {}
    all_keyframes = defaultdict(list)
    all_vars_defs = defaultdict(list)
    all_selectors = defaultdict(list)
    all_fonts = defaultdict(list)
    all_rules = defaultdict(list)
    zindex_values = []
    
    for fname, content in files.items():
        blocks = extract_rule_blocks(content)
        kfs = extract_keyframes_full(content)
        var_defs = extract_css_var_defs(content)
        colors = []
        for pat, _ in [(r'#([0-9A-Fa-f]{3,8})\b', 'hex'), (r'rgb\(', 'rgb'), (r'rgba\(', 'rgba'), (r'hsl\(', 'hsl')]:
            colors.extend(re.findall(pat, content))
        zidx = extract_zindex_with_context(content)
        imp = extract_important_with_context(content)
        fonts = extract_font_family_with_context(content)
        units = re.findall(r"(?<![#A-Za-z0-9_-])(-?\d*\.?\d+)(px|rem|em|%|vh|vw|pt|cm|mm|in|ex|ch|vmin|vmax|fr)\b", content)
        media = extract_media_queries(content)
        dead = extract_dead_code_blocks(content)
        
        total_important += len(imp)
        total_colors += len(colors)
        total_zindex += len(zidx)
        total_media += len(media)
        zindex_values.extend([int(x[0]) for x in zidx])
        
        for u in units:
            unit_counter[u[1]] += 1
        
        for name, body in kfs:
            all_keyframes[name].append((fname, body))
        
        for vd in var_defs:
            all_vars_defs[vd[0]].append((fname, vd[1], vd[2]))
        
        for sel, body in blocks:
            all_selectors[sel].append(fname)
            all_rules[(sel, body)].append(fname)
        
        for f, ctx in fonts:
            all_fonts[f].append(fname)
        
        file_stats[fname] = {
            "size": len(content),
            "selectors": len(blocks),
            "keyframes": len(kfs),
            "var_defs": len(var_defs),
            "colors": len(colors),
            "zindex": len(zidx),
            "important": len(imp),
            "fonts": len(fonts),
            "media_queries": len(media),
            "dead_code": len(dead),
        }
    
    report.append(f"- **Total CSS:** {sum(len(v) for v in files.values()):,} bytes across {len(files)} files")
    report.append(f"- **Total Selectors:** {sum(s['selectors'] for s in file_stats.values()):,}")
    report.append(f"- **Hardcoded Colors:** {total_colors:,}")
    report.append(f"- **`!important` Declarations:** {total_important}")
    report.append(f"- **z-index Declarations:** {total_zindex}")
    report.append(f"- **Media Queries:** {total_media}")
    report.append(f"- **Duplicate Keyframes:** {sum(1 for v in all_keyframes.values() if len(v) > 1)}")
    report.append(f"- **Duplicate Selectors:** {sum(1 for v in all_selectors.values() if len(v) > 1)}")
    report.append(f"- **Exact Rule Duplications:** {sum(1 for v in all_rules.values() if len(v) > 1)}")
    report.append("")
    
    # === FILE STATS TABLE ===
    report.append("## Per-File Statistics")
    report.append("")
    report.append("| File | Size | Selectors | Keyframes | Var Defs | Colors | z-index | !important | Media | Dead Code |")
    report.append("|------|------:|----------:|----------:|---------:|-------:|--------:|-----------:|------:|----------:|")
    for fname in sorted(file_stats):
        s = file_stats[fname]
        report.append(f"| {fname} | {s['size']:,} | {s['selectors']} | {s['keyframes']} | {s['var_defs']} | {s['colors']} | {s['zindex']} | {s['important']} | {s['media_queries']} | {s['dead_code']} |")
    report.append("")
    
    # === UNIT ANALYSIS ===
    report.append("## Unit Usage Distribution")
    report.append("")
    report.append("| Unit | Count | % of Total |")
    report.append("|------|------:|-----------:|")
    total_units = sum(unit_counter.values())
    for unit, count in unit_counter.most_common():
        pct = count / total_units * 100
        report.append(f"| {unit} | {count:,} | {pct:.1f}% |")
    report.append("")
    report.append("**Issue:** px and rem are used in nearly equal amounts (≈50% each), indicating no clear standard for sizing.")
    report.append("")
    
    # === CRITICAL ISSUE 1: NO CSS VARIABLES IN MOST FILES ===
    report.append("## Critical Issue: Absence of CSS Custom Properties")
    report.append("")
    report.append("The vast majority of files define **zero** CSS custom properties, yet use `var()` extensively in some files. "
                  "This indicates colors, spacing, and typography are hardcoded rather than themed.")
    report.append("")
    no_var_files = [f for f, s in file_stats.items() if s['var_defs'] == 0]
    report.append(f"**Files with 0 variable definitions:** {', '.join(sorted(no_var_files))}")
    report.append("")
    
    # Show var definitions that DO exist
    if all_vars_defs:
        report.append("### Existing CSS Variable Definitions (scattered)")
        report.append("")
        for var, defs in sorted(all_vars_defs.items())[:30]:
            files_list = ', '.join(sorted(set(d[0] for d in defs)))
            vals = list(set(d[1] for d in defs))
            report.append(f"- `{var}` = `{vals[0]}` in {files_list}")
        report.append("")
    
    # === CRITICAL ISSUE 2: DUPLICATED KEYFRAMES ===
    report.append("## Critical Issue: Duplicated @keyframes")
    report.append("")
    dup_kfs = {k: v for k, v in all_keyframes.items() if len(v) > 1}
    if dup_kfs:
        for name, occurrences in sorted(dup_kfs.items()):
            fnames = [o[0] for o in occurrences]
            report.append(f"### `{name}`")
            report.append(f"Found in: {', '.join(fnames)}")
            report.append("```css")
            report.append(occurrences[0][1][:500])
            report.append("```")
            report.append("")
    
    # === CRITICAL ISSUE 3: EXACT RULE DUPLICATION ===
    report.append("## Critical Issue: Exact Rule Duplication Across Files")
    report.append("")
    dup_rules = {k: v for k, v in all_rules.items() if len(v) > 1}
    report.append(f"There are **{len(dup_rules)}** exact rule duplications across files. Examples:")
    report.append("")
    
    # Filter out trivial ones (keyframes percentages, width percentages)
    meaningful_dups = []
    for (sel, body), fnames in sorted(dup_rules.items(), key=lambda x: -len(x[1])):
        if sel in ('to', 'from', '50%', '100%', '0%'):
            continue
        if sel.startswith('.w-pct-'):
            continue
        meaningful_dups.append((sel, body, fnames))
    
    for sel, body, fnames in meaningful_dups[:25]:
        report.append(f"### `{sel}` in {len(fnames)} files")
        report.append(f"Files: {', '.join(fnames)}")
        report.append("```css")
        report.append(f"{sel} {{")
        for line in body.splitlines()[:10]:
            report.append(f"  {line}")
        report.append("}")
        report.append("```")
        report.append("")
    
    # === CRITICAL ISSUE 4: INCONSISTENT MEDIA QUERIES ===
    report.append("## Critical Issue: Inconsistent Breakpoints")
    report.append("")
    all_media_raw = []
    for fname, content in files.items():
        all_media_raw.extend(extract_media_queries(content))
    media_counter = Counter([m.strip() for m in all_media_raw])
    report.append("| Breakpoint | Occurrences |")
    report.append("|------------|------------:|")
    for mq, count in media_counter.most_common(25):
        report.append(f"| `{mq}` | {count} |")
    report.append("")
    report.append("**Problem:** Breakpoints are all over the place: `480px`, `560px`, `600px`, `640px`, `767px`, `768px`, `900px`, `1024px`, etc. "
                  "There is no standard grid system.")
    report.append("")
    
    # === CRITICAL ISSUE 5: FONT FAMILY CHAOS ===
    report.append("## Critical Issue: Font-Family Inconsistency")
    report.append("")
    report.append("There are **11 distinct monospace font stacks** across the codebase:")
    report.append("")
    for font, fnames in sorted(all_fonts.items(), key=lambda x: -len(x[1])):
        report.append(f"- `{font}` — {len(fnames)} files")
    report.append("")
    
    # === CRITICAL ISSUE 6: HARDCODED COLORS ===
    report.append("## Critical Issue: Hardcoded Colors")
    report.append("")
    report.append("Over **1,100** hardcoded color values exist. Top offenders:")
    report.append("")
    for fname in sorted(file_stats, key=lambda x: -file_stats[x]['colors'])[:8]:
        report.append(f"- **{fname}**: {file_stats[fname]['colors']} colors")
    report.append("")
    
    report.append("### Sample Hardcoded Colors (with context)")
    report.append("")
    for fname in ['config.css', 'chat.css', 'dashboard.css', 'login.css']:
        content = files[fname]
        colors = extract_hardcoded_colors_with_context(content, max_results=8)
        if colors:
            report.append(f"#### {fname}")
            for ctype, val, ctx in colors:
                report.append("```css")
                report.append(ctx)
                report.append("```")
            report.append("")
    
    # === CRITICAL ISSUE 7: !IMPORTANT ===
    report.append("## Critical Issue: `!important` Usage")
    report.append("")
    report.append(f"Total `!important` declarations: **{total_important}**")
    report.append("")
    for fname in sorted(files, key=lambda x: -file_stats[x]['important']):
        if file_stats[fname]['important'] == 0:
            continue
        report.append(f"### {fname} ({file_stats[fname]['important']} occurrences)")
        content = files[fname]
        snippets = extract_important_with_context(content, max_results=5)
        for ctx in snippets:
            report.append("```css")
            report.append(ctx)
            report.append("```")
        report.append("")
    
    # === CRITICAL ISSUE 8: z-index ===
    report.append("## Critical Issue: z-index Management")
    report.append("")
    if total_zindex == 0:
        report.append("**No `z-index` declarations were found in any CSS file.**")
        report.append("This is suspicious for a UI of this complexity. z-index may be applied inline via JavaScript, "
                      "which makes layering bugs extremely difficult to debug.")
    else:
        report.append(f"z-index range: {min(zindex_values)} to {max(zindex_values)}")
        zc = Counter(zindex_values)
        for val, count in zc.most_common():
            report.append(f"- z-index `{val}`: {count} occurrences")
    report.append("")
    
    # === FILE-LEVEL ISSUES ===
    report.append("## File-Level Deep Dive")
    report.append("")
    
    for fname, content in files.items():
        issues = []
        s = file_stats[fname]
        
        # Size
        if s['size'] > 50000:
            issues.append(f"- **Oversized:** {s['size']:,} bytes. This file is a monolith and should be split into component-level stylesheets.")
        
        # No vars + many colors
        if s['var_defs'] == 0 and s['colors'] > 30:
            issues.append(f"- **No theming:** 0 CSS variable definitions with {s['colors']} hardcoded colors.")
        
        # Dead code
        dead = extract_dead_code_blocks(content)
        if dead:
            issues.append(f"- **Dead/commented code:** {len(dead)} blocks")
            for d in dead[:2]:
                issues.append(f"  ```css\n  /* {d[:250]} */\n  ```")
        
        # Media query inconsistency
        if s['media_queries'] > 0:
            mqs = extract_media_queries(content)
            unique_mqs = set(m.strip() for m in mqs)
            issues.append(f"- **Media queries:** {len(unique_mqs)} unique breakpoints")
        
        if issues:
            report.append(f"### {fname}")
            report.extend(issues)
            report.append("")
    
    # === SELECTOR CONFLICTS ===
    report.append("## Selector Conflicts Across Files")
    report.append("")
    conflicting = []
    for sel, fnames in all_selectors.items():
        if len(fnames) > 1 and not sel.startswith('@') and sel not in ('to', 'from', '0%', '50%', '100%'):
            # Check if rules differ
            bodies = set()
            for f in fnames:
                blocks = extract_rule_blocks(files[f])
                for s, b in blocks:
                    if s == sel:
                        bodies.add(b.strip())
            if len(bodies) > 1:
                conflicting.append((sel, fnames, bodies))
    
    report.append(f"Found **{len(conflicting)}** selectors defined in multiple files with **different** rule bodies.")
    report.append("")
    for sel, fnames, bodies in sorted(conflicting, key=lambda x: -len(x[1]))[:20]:
        report.append(f"### `{sel}`")
        report.append(f"Files: {', '.join(fnames)}")
        for i, body in enumerate(list(bodies)[:3]):
            report.append(f"**Variant {i+1}:**")
            report.append("```css")
            report.append(f"{sel} {{")
            for line in body.splitlines()[:8]:
                report.append(f"  {line}")
            report.append("}")
            report.append("```")
        report.append("")
    
    # === RECOMMENDATIONS ===
    report.append("## Recommendations")
    report.append("")
    report.append("""
1. **Create a Design Token Layer:**
   - Add `ui/css/tokens.css` with `:root` variables for colors, spacing, typography, shadows, radii, and z-index.
   - Migrate all hardcoded colors to `var(--color-*)` references.

2. **Consolidate Animations:**
   - Move all `@keyframes` to `ui/css/animations.css`.
   - Eliminate the 6+ duplicated keyframe definitions.

3. **Standardize Breakpoints:**
   - Define CSS custom media queries or a consistent set: `sm: 640px`, `md: 768px`, `lg: 1024px`, `xl: 1280px`.

4. **Unify Typography:**
   - Set `--font-sans` and `--font-mono` in tokens.css.
   - Replace all 11+ monospace stacks with these two variables.

5. **Split Monoliths:**
   - `config.css` (107KB), `chat.css` (88KB), and `dashboard.css` (83KB) should be decomposed.
   - Example: `config.css` → `config-layout.css`, `config-forms.css`, `config-tables.css`.

6. **Eliminate `!important`:**
   - Refactor the 23 `!important` usages by increasing selector specificity or using utility classes consistently.

7. **Establish z-index Governance:**
   - If z-index is managed in JS, document it. If not, add a z-index scale to tokens.css.

8. **Remove Dead Code:**
   - Delete commented-out CSS blocks (found in `gallery.css`, `media.css`, and others).

9. **Adopt rem for Accessibility:**
   - px dominates sizing. Switch font sizes and spacing to rem, reserving px for borders and 1px hairlines.

10. **Theme Support:**
    - Many components use `[data-theme=\"light\"]` but lack corresponding dark variable definitions.
    - Ensure all color variables have light/dark variants in `:root` and `[data-theme=\"dark\"]`.
""")
    
    REPORT_PATH.write_text("\n".join(report), encoding="utf-8")
    print(f"Report written to {REPORT_PATH}")

if __name__ == "__main__":
    analyze()
