package cydcreator

import (
	"encoding/json"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Server-written progress lines (job_step events) and system chat lines carry
// a translation code and its vars next to the English text. The English text
// is always rendered here from the code's template, so text and code cannot
// drift: the client shows cyd_creator.step.<code> or cyd_creator.msg.<code>
// with the same vars and falls back to the text. ui/lang/cyd_creator/en.json
// holds exactly these templates (TextKeys; the ui key test checks all
// locales). Placeholders are {{name}}; counts are phrased without plural
// forms ("screens: 2"), because the desktop i18n has no plural rules, and
// durations are integer vars with unit symbols ({{seconds}} s, {{minutes}}
// min, {{milliseconds}} ms), never Go duration strings.

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
// clipped; model prose never does. build_failed carries error_code next to
// error (the code itself in the English text), so the client shows its own
// sentence for the code like job_failed.
var stepTexts = map[string]string{
	"plan_submitted":         "Plan submitted · screens: {{screens}} · widgets: {{widgets}} · data sources: {{data_sources}}",
	"layout_updated":         "Layout changes ({{count}}): {{changes}}",
	"file_written":           "Wrote {{path}} (lines: {{lines}})",
	"file_deleted":           "Deleted {{path}}",
	"repair_started":         "Repair {{attempt}}/{{max}} started",
	"build_succeeded":        "Build {{n}} succeeded",
	"build_succeeded_flash":  "Build {{n}} succeeded (flash {{percent}}%)",
	"build_failed_at":        "Build {{n}} failed: {{location}} {{message}}",
	"build_failed_error":     "Build {{n}} failed: {{message}}",
	"build_failed":           "Build {{n}} failed: {{error}}",
	"publish_retry":          "A project file is open; publishing again in {{milliseconds}} ms.",
	"grace_extended":         "Recent changes are still being built; the job gets {{seconds}} s more to finish.",
	"grace_extended_minutes": "Recent changes are still being built; the job gets {{minutes}} min {{seconds}} s more to finish.",
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
	"job_timeout":                   "The job exceeded its time limit of {{seconds}} s. The project is unchanged.",
	"job_timeout_draft":             "The job exceeded its time limit of {{seconds}} s. The project is unchanged." + draftSentence,
	"job_timeout_minutes":           "The job exceeded its time limit of {{minutes}} min {{seconds}} s. The project is unchanged.",
	"job_timeout_minutes_draft":     "The job exceeded its time limit of {{minutes}} min {{seconds}} s. The project is unchanged." + draftSentence,
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

// textSeconds is d in whole seconds, rounded up (a duration var: Go's
// duration strings such as "30m0s" would show untranslated). Callers with
// d >= 60 s use the code's _minutes variant with minutes and the remaining
// seconds.
func textSeconds(d time.Duration) int64 {
	return int64(math.Ceil(d.Seconds()))
}

// textVars keeps the vars a client may interpolate: identifier names with a
// string (folded to one line, at most maxRunes runes) or a number (every
// integer kind becomes int64, a uint above MaxInt64 a json.Number, finite
// floats float64). Anything else (bool, slices, NaN) is dropped, so its
// placeholder renders empty and renderText warns; no vars is nil.
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
		case json.Number:
			if _, err := v.Float64(); err == nil {
				clean = v
			}
		default:
			rv := reflect.ValueOf(value)
			switch {
			case !rv.IsValid():
			case rv.CanInt():
				clean = rv.Int()
			case rv.CanUint() && rv.Uint() <= math.MaxInt64:
				clean = int64(rv.Uint())
			case rv.CanUint():
				clean = json.Number(strconv.FormatUint(rv.Uint(), 10))
			case rv.CanFloat() && !math.IsNaN(rv.Float()) && !math.IsInf(rv.Float(), 0):
				clean = rv.Float()
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

// fillText fills the {{name}} placeholders of template in one pass, so a
// value is never read as a placeholder itself. A placeholder without a var
// renders empty and is returned in missing.
func fillText(template string, vars map[string]any) (text string, missing []string) {
	text = textPlaceholder.ReplaceAllStringFunc(template, func(ph string) string {
		name := ph[2 : len(ph)-2]
		switch v := vars[name].(type) {
		case string:
			return v
		case int64:
			return strconv.FormatInt(v, 10)
		case int:
			return strconv.Itoa(v)
		case float64:
			return strconv.FormatFloat(v, 'f', -1, 64)
		case json.Number:
			return v.String()
		}
		missing = append(missing, name)
		return ""
	})
	return text, missing
}

// renderText is fillText for the code's template; a placeholder without a
// var (a call site that forgot it, or a value textVars dropped) is logged as
// a warning.
func (s *Service) renderText(code, template string, vars map[string]any) string {
	text, missing := fillText(template, vars)
	if len(missing) > 0 {
		s.logger.Warn("CYD Creator text placeholder without a var", "code", code, "missing", missing)
	}
	return text
}
