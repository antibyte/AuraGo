package cydcreator

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Server-written progress lines (job_step events) and system chat lines carry
// a translation code and its vars next to the English text. The English text
// is always rendered here from the code's template, so text and code cannot
// drift: the client shows cyd_creator.step.<code> or cyd_creator.msg.<code>
// with the same vars and falls back to the text. ui/lang/cyd_creator/en.json
// holds exactly these templates (TextKeys; the ui key test checks all
// locales). Placeholders are {{name}}; counts are phrased without plural
// forms ("screens: 2"), because the desktop i18n has no plural rules.

// textCodePattern is the shape of a code (the error_reason rule).
var textCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// textVarKeyPattern is the shape of a var name.
var textVarKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

// textPlaceholder matches one {{name}} placeholder of a template.
var textPlaceholder = regexp.MustCompile(`\{\{([a-z][a-z0-9_]{0,31})\}\}`)

const (
	// maxStepVarRunes bounds a string var of a job_step (the English line
	// is cut at 200 runes anyway).
	maxStepVarRunes = 160
	// maxMessageVarRunes bounds a string var of a system chat line.
	maxMessageVarRunes = 300
)

// stepTexts maps every job_step code to its English template. Widget and
// screen names, app paths and compiler messages arrive as vars, already
// clipped; model prose never does.
var stepTexts = map[string]string{
	"plan_submitted":        "Plan submitted · screens: {{screens}} · widgets: {{widgets}} · data sources: {{data_sources}}",
	"layout_updated":        "Layout changes ({{count}}): {{changes}}",
	"file_written":          "Wrote {{path}} (lines: {{lines}})",
	"file_deleted":          "Deleted {{path}}",
	"repair_started":        "Repair {{attempt}}/{{max}} started",
	"build_succeeded":       "Build {{n}} succeeded",
	"build_succeeded_flash": "Build {{n}} succeeded (flash {{percent}}%)",
	"build_failed_at":       "Build {{n}} failed: {{location}} {{message}}",
	"build_failed_error":    "Build {{n}} failed: {{message}}",
	"build_failed":          "Build {{n}} failed ({{error_code}})",
	"publish_retry":         "A project file is open; publishing again in {{delay}}.",
	"grace_extended":        "Recent changes are still being built; the job gets {{duration}} more to finish.",
}

// draftCodeSuffix names the variant of a job ending whose working copy became
// the project's draft; its text adds draftSentence.
const (
	draftCodeSuffix = "_draft"
	draftSentence   = " The agent's work is kept as a draft you can resume or discard."
)

// messageTexts maps every system chat line code to its English template. The
// job endings (jobEnding.msg) each have a draft variant.
var messageTexts = map[string]string{
	"job_cancelled":                 "The job was cancelled. The project is unchanged.",
	"job_cancelled_draft":           "The job was cancelled. The project is unchanged." + draftSentence,
	"job_timeout":                   "The job exceeded its time limit of {{limit}}. The project is unchanged.",
	"job_timeout_draft":             "The job exceeded its time limit of {{limit}}. The project is unchanged." + draftSentence,
	"job_interrupted":               "The server stopped while the job was running. The project is unchanged.",
	"job_interrupted_draft":         "The server stopped while the job was running. The project is unchanged." + draftSentence,
	"job_failed":                    "{{error}} The project is unchanged.",
	"job_failed_draft":              "{{error}} The project is unchanged." + draftSentence,
	"job_failed_internal":           "The job failed because of an internal error; the server log has the details. The project is unchanged.",
	"job_failed_internal_draft":     "The job failed because of an internal error; the server log has the details. The project is unchanged." + draftSentence,
	"agent_stopped":                 "The agent stopped: {{reason}} The project is unchanged.",
	"agent_stopped_draft":           "The agent stopped: {{reason}} The project is unchanged." + draftSentence,
	"plan_not_stored":               "The plan could not be stored. The project is unchanged.",
	"plan_not_stored_draft":         "The plan could not be stored. The project is unchanged." + draftSentence,
	"draft_kept":                    "The agent's work of the interrupted job is now kept as a draft you can resume or discard.",
	"server_restarted":              "The server restarted while the job was running. The project is unchanged.",
	"server_restarted_draft":        "The server restarted while the job was running. The project is unchanged." + draftSentence,
	"server_restarted_review":       "The server restarted while the job was running. The project needs a review before it can be changed again; see the server log.",
	"server_restarted_review_draft": "The server restarted while the job was running. The project needs a review before it can be changed again; see the server log." + draftSentence,
	"published_before_restart":      "The app was built and published just before the server restarted.",
}

// TextKeys returns the English template of every job_step and system chat
// line code under its desktop translation key (cyd_creator.step.<code>,
// cyd_creator.msg.<code>).
func TextKeys() map[string]string {
	out := make(map[string]string, len(stepTexts)+len(messageTexts))
	for code, text := range stepTexts {
		out["cyd_creator.step."+code] = text
	}
	for code, text := range messageTexts {
		out["cyd_creator.msg."+code] = text
	}
	return out
}

// textVars keeps the vars a client may interpolate: identifier names with a
// string (folded to one line, at most maxRunes runes) or an integer value.
// Anything else is dropped; no vars is nil.
func textVars(vars map[string]any, maxRunes int) map[string]any {
	var out map[string]any
	for key, value := range vars {
		if !textVarKeyPattern.MatchString(key) {
			continue
		}
		var clean any
		switch v := value.(type) {
		case string:
			s := strings.Join(strings.Fields(v), " ")
			if utf8.RuneCountInString(s) > maxRunes {
				s = string([]rune(s)[:maxRunes-1]) + "…"
			}
			clean = s
		case int:
			clean = int64(v)
		case int32:
			clean = int64(v)
		case int64:
			clean = v
		case json.Number:
			if _, err := v.Int64(); err == nil {
				clean = v
			}
		}
		if clean == nil {
			continue
		}
		if out == nil {
			out = map[string]any{}
		}
		out[key] = clean
	}
	return out
}

// renderText fills the {{name}} placeholders of template in one pass, so a
// value is never read as a placeholder itself; a missing var renders empty.
func renderText(template string, vars map[string]any) string {
	return textPlaceholder.ReplaceAllStringFunc(template, func(ph string) string {
		switch v := vars[ph[2:len(ph)-2]].(type) {
		case string:
			return v
		case int64:
			return strconv.FormatInt(v, 10)
		case int:
			return strconv.Itoa(v)
		case json.Number:
			return v.String()
		}
		return ""
	})
}
