package ui

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

// Sections that cannot be rendered by the fixture harness. Each entry needs a reason.
var configGuardUnrenderable = map[string]string{}

// configGuardCollectJS lists the visible form controls of the rendered section and reports a missing
// or fallback label and a missing description for each. Closed modals, hidden controls and everything
// outside #content (sidebar search, page chrome, save bar) are ignored.
const configGuardCollectJS = `() => {
	const root = document.getElementById('content');
	const text = el => (el && (el.textContent || '').replace(/\s+/g, ' ').trim()) || '';
	const isShown = el => {
		if (!el.getClientRects().length) return false;
		for (let node = el; node && node !== root; node = node.parentElement) {
			const style = getComputedStyle(node);
			if (style.display === 'none' || style.visibility === 'hidden') return false;
			if (node.matches('dialog:not([open])')) return false;
		}
		return true;
	};
	const labelOf = el => {
		const labelled = (el.getAttribute('aria-labelledby') || '').split(/\s+/).filter(Boolean).map(id => document.getElementById(id)).filter(Boolean);
		for (const node of labelled) {
			if (text(node)) return {text: text(node), fallback: node.hasAttribute('data-label-fallback')};
		}
		if ((el.getAttribute('aria-label') || '').trim()) return {text: el.getAttribute('aria-label').trim(), fallback: false};
		if (el.id) {
			for (const node of document.querySelectorAll('label[for]')) {
				if (node.getAttribute('for') === el.id && text(node)) return {text: text(node), fallback: node.hasAttribute('data-label-fallback')};
			}
		}
		const wrapping = el.closest('label');
		if (wrapping && text(wrapping)) return {text: text(wrapping), fallback: wrapping.hasAttribute('data-label-fallback')};
		const group = el.closest('.field-group, .pw-field');
		const groupLabel = group && group.querySelector('.field-label');
		if (groupLabel && text(groupLabel)) return {text: text(groupLabel), fallback: groupLabel.hasAttribute('data-label-fallback')};
		return {text: '', fallback: false};
	};
	const helpOf = el => {
		for (const id of (el.getAttribute('aria-describedby') || '').split(/\s+/).filter(Boolean)) {
			if (text(document.getElementById(id))) return true;
		}
		const group = el.closest('.field-group, .pw-field');
		return !!group && [...group.querySelectorAll('.field-help')].some(node => text(node));
	};
	const controls = [...root.querySelectorAll('input, select, textarea, .toggle, [role="switch"]')].filter(el => !el.matches('input[type="hidden"]'));
	const rows = [];
	controls.forEach((el, index) => {
		if (!isShown(el)) return;
		const label = labelOf(el);
		const id = el.id && !/^cfg-form-\d+$/.test(el.id) ? el.id : '';
		const kind = el.tagName.toLowerCase() + (el.type ? '.' + el.type : '') + (el.classList.contains('toggle') ? '.toggle' : '');
		rows.push({
			ident: el.dataset.path || id || el.getAttribute('name') || ('nth' + index + ':' + kind),
			label: label.text !== '' && !label.fallback,
			fallback: label.fallback,
			help: helpOf(el),
		});
	});
	return rows;
}`

func TestConfigEveryControlLabeledBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "en", true) + "/config#overview")
	defer page.MustClose()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
	page.MustSetViewport(1440, 900, 1, false)
	sections := page.MustEval(`() => SECTIONS.flatMap(group => group.items.map(item => item.key))`).Arr()
	var gaps []string
	labelGaps := map[string]int{}
	helpGaps := map[string]int{}
	controlCount := map[string]int{}
	var skipped []string
	for _, section := range sections {
		key := section.Str()
		if reason, skip := configGuardUnrenderable[key]; skip {
			skipped = append(skipped, key+": "+reason)
			continue
		}
		page.MustEval(`async key => { await selectSection(key, {scrollBehavior:'auto'}); document.querySelectorAll('#content details').forEach(details => details.open = true); resetDirtySnapshot(); await new Promise(resolve => setTimeout(resolve, 350)); }`, key)
		if page.MustEval(`() => !!document.querySelector('.cfg-error-state') || document.getElementById('content').innerText.trim() === ''`).Bool() {
			skipped = append(skipped, key+": section did not render in the harness")
			continue
		}
		rows := page.MustEval(configGuardCollectJS).Arr()
		seen := map[string]bool{}
		for _, row := range rows {
			ident := row.Get("ident").Str()
			ident = strings.Join(strings.Fields(ident), "_")
			if !row.Get("label").Bool() {
				line := key + " label " + ident
				if !seen[line] {
					seen[line] = true
					gaps = append(gaps, line)
					labelGaps[key]++
				}
			}
			if !row.Get("help").Bool() {
				line := key + " help " + ident
				if !seen[line] {
					seen[line] = true
					gaps = append(gaps, line)
					helpGaps[key]++
				}
			}
			controlCount[key]++
		}
	}
	keys := make([]string, 0, len(controlCount))
	totalLabel, totalHelp := 0, 0
	for key := range controlCount {
		keys = append(keys, key)
		totalLabel += labelGaps[key]
		totalHelp += helpGaps[key]
	}
	sort.Strings(keys)
	var table strings.Builder
	for _, key := range keys {
		fmt.Fprintf(&table, "\n%-28s controls=%-4d label=%-4d help=%d", key, controlCount[key], labelGaps[key], helpGaps[key])
	}
	t.Logf("per section:%s\ntotal label gaps=%d help gaps=%d; skipped sections: %v", table.String(), totalLabel, totalHelp, skipped)
	configBaselineCheck(t, "testdata/config_text_gaps.txt", "AURAGO_UPDATE_TEXT_GAPS", gaps,
		"# <section> <label|help> <data-path|id|name|nth<index>:<element>>: visible config controls without a translated label or description (Welle 2 package Z empties this).\n")
}
