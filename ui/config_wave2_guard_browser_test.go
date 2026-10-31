package ui

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

// Sections that cannot be rendered by the fixture harness. Each entry needs a reason; a section that shows
// an error state or renders empty and is not listed here fails the guard.
var configGuardUnrenderable = map[string]string{}

// Sections that legitimately render no form control on first load. Each entry needs a reason; any other
// section without a visible control fails the guard (it would otherwise pass unchecked).
var configGuardNoControls = map[string]string{
	"rules":      "rule files are listed and edited only after the async file list is fetched and a file is selected",
	"updates":    "version check and update actions appear only after the async update status request answers",
	"web_config": "the editor controls appear only after the async web config fetch and a selection",
}

// configGuardEnableNested sets every boolean "enabled" key at any depth to true so sub-features render in
// the guard's own fixture copy (the shared fixture used by other tests stays untouched).
func configGuardEnableNested(node any) {
	switch value := node.(type) {
	case map[string]any:
		for key, child := range value {
			if _, isBool := child.(bool); isBool && key == "enabled" {
				value[key] = true
				continue
			}
			configGuardEnableNested(child)
		}
	case []any:
		for _, child := range value {
			configGuardEnableNested(child)
		}
	}
}

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
	// Label text without the description that may be nested in the same <label>/group label.
	const labelText = node => {
		if (!node) return '';
		const clone = node.cloneNode(true);
		clone.querySelectorAll('.field-help').forEach(help => help.remove());
		return text(clone);
	};
	const selector = 'input, select, textarea, .toggle, [role="switch"]';
	const allControls = [...root.querySelectorAll(selector)].filter(el => !el.matches('input[type="hidden"]'));
	const shownControls = allControls.filter(isShown);
	// A styled toggle that wraps a real checkbox counts as that one checkbox when controls are counted.
	const countable = shownControls.filter(el => !el.querySelector('input, select, textarea'));
	const controlsIn = container => countable.filter(control => container.contains(control));
	const groupOf = el => el.closest('.field-group, .pw-field');
	const labelOf = el => {
		const labelled = (el.getAttribute('aria-labelledby') || '').split(/\s+/).filter(Boolean).map(id => document.getElementById(id)).filter(Boolean);
		for (const node of labelled) {
			if (labelText(node)) return {text: labelText(node), fallback: node.hasAttribute('data-label-fallback')};
		}
		if ((el.getAttribute('aria-label') || '').trim()) return {text: el.getAttribute('aria-label').trim(), fallback: false};
		if (el.id) {
			for (const node of document.querySelectorAll('label[for]')) {
				if (node.getAttribute('for') === el.id && labelText(node)) return {text: labelText(node), fallback: node.hasAttribute('data-label-fallback')};
			}
		}
		const wrapping = el.closest('label');
		if (wrapping && labelText(wrapping)) return {text: labelText(wrapping), fallback: wrapping.hasAttribute('data-label-fallback')};
		// A group-level label names the control only when the control is alone in its group.
		const group = groupOf(el);
		if (group && controlsIn(group).length === 1) {
			const groupLabel = [...group.querySelectorAll('.field-label')].find(node => groupOf(node) === group && labelText(node));
			if (groupLabel) return {text: labelText(groupLabel), fallback: groupLabel.hasAttribute('data-label-fallback')};
		}
		return {text: '', fallback: false};
	};
	const helpOf = el => {
		for (const id of (el.getAttribute('aria-describedby') || '').split(/\s+/).filter(Boolean)) {
			if (text(document.getElementById(id))) return true;
		}
		// A group-level description belongs to the control only when the control is alone in its group.
		const group = groupOf(el);
		if (group && controlsIn(group).length === 1 && [...group.querySelectorAll('.field-help')].some(node => groupOf(node) === group && text(node))) return true;
		// Or the description is the control's own sibling in the same immediate wrapper.
		const wrapper = el.parentElement;
		if (wrapper && controlsIn(wrapper).length === 1 && [...wrapper.children].some(node => node !== el && node.classList.contains('field-help') && text(node))) return true;
		return false;
	};
	const rows = [];
	allControls.forEach((el, index) => {
		if (!isShown(el)) return;
		const label = labelOf(el);
		const id = el.id && !/^cfg-form-\d+$/.test(el.id) ? el.id : '';
		const kind = el.tagName.toLowerCase() + (el.type ? '.' + el.type : '') + (el.classList.contains('toggle') ? '.toggle' : '');
		rows.push({
			ident: el.dataset.path || el.dataset.sip || id || el.getAttribute('name') || ('nth' + index + ':' + kind),
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
	page := browser.MustPage(configRefreshFixtureOriginWith(t, "en", true, func(configuration map[string]any) {
		configGuardEnableNested(configuration)
	}) + "/config#overview")
	defer page.MustClose()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
	page.MustSetViewport(1440, 900, 1, false)
	sections := page.MustEval(`() => SECTIONS.flatMap(group => group.items.map(item => item.key))`).Arr()
	var gaps []string
	labelGaps := map[string]int{}
	helpGaps := map[string]int{}
	controlCount := map[string]int{}
	var skipped, failures []string
	for _, section := range sections {
		key := section.Str()
		if reason, skip := configGuardUnrenderable[key]; skip {
			skipped = append(skipped, key+": "+reason)
			continue
		}
		if reason, listed := configGuardNoControls[key]; listed && reason == "" {
			t.Errorf("configGuardNoControls[%q] needs a reason", key)
		}
		page.MustEval(`async key => { await selectSection(key, {scrollBehavior:'auto'}); document.querySelectorAll('#content details').forEach(details => details.open = true); resetDirtySnapshot(); await new Promise(resolve => setTimeout(resolve, 350)); }`, key)
		if page.MustEval(`() => !!document.querySelector('.cfg-error-state') || document.getElementById('content').innerText.trim() === ''`).Bool() {
			failures = append(failures, key+": shows an error state or renders empty (fix it or list it in configGuardUnrenderable with a reason)")
			continue
		}
		rows := page.MustEval(configGuardCollectJS).Arr()
		if _, listed := configGuardNoControls[key]; len(rows) == 0 && !listed {
			failures = append(failures, key+": renders no visible control (fix the fixture or list it in configGuardNoControls with a reason)")
		}
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
	for _, failure := range failures {
		t.Error(failure)
	}
	configBaselineCheck(t, "testdata/config_text_gaps.txt", "AURAGO_UPDATE_TEXT_GAPS", gaps,
		"# <section> <label|help> <data-path|id|name|nth<index>:<element>>: visible config controls without a translated label or description (Welle 2 package Z empties this).\n")
}
