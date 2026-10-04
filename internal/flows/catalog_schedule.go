package flows

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/robfig/cron/v3"
)

// maxCronBytes bounds a cron expression. Real ones have a few dozen characters; the
// expression is stored in a Mission Control schedule.
const maxCronBytes = 200

// cronParser reads cron expressions exactly like AuraGo's schedulers do (see
// newCronParser in internal/tools/cron.go and validateCronExpr in
// internal/server/mission_v2_handlers.go; flows must not import those packages): an
// optional leading seconds field, descriptors such as @daily, day of week 0-6 only. It
// holds no state, so concurrent use is fine.
var cronParser = cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

var weekdayNumbers = []struct {
	key string
	num int
}{{"mon", 1}, {"tue", 2}, {"wed", 3}, {"thu", 4}, {"fri", 5}, {"sat", 6}, {"sun", 0}}

func scheduleParams() []ParamSpec {
	when := func(modes ...string) *Visibility { return &Visibility{Param: "mode", Equals: modes} }
	days := make([]Option, 0, len(weekdayNumbers))
	for _, d := range weekdayNumbers {
		days = append(days, option(d.key, "weekday_"+d.key))
	}
	return []ParamSpec{
		{Name: "mode", Kind: ParamSelect, LabelKey: "easydrag.param.schedule_mode", Default: "daily", Options: []Option{
			option("interval_minutes", "schedule_interval_minutes"), option("interval_hours", "schedule_interval_hours"),
			option("daily", "schedule_daily"), option("weekdays", "schedule_weekdays"), option("weekly", "schedule_weekly"),
			option("monthly", "schedule_monthly"), option("cron", "schedule_cron")}},
		{Name: "minutes", Kind: ParamNumber, LabelKey: "easydrag.param.schedule_minutes", Required: true, Default: 15.0, VisibleIf: when("interval_minutes")},
		{Name: "hours", Kind: ParamNumber, LabelKey: "easydrag.param.schedule_hours", Required: true, Default: 1.0, VisibleIf: when("interval_hours")},
		{Name: "time", Kind: ParamText, LabelKey: "easydrag.param.schedule_time", Required: true, Default: "07:00", VisibleIf: when("daily", "weekdays", "weekly", "monthly")},
		{Name: "weekdays", Kind: ParamMultiSelect, LabelKey: "easydrag.param.schedule_weekdays", Required: true, Options: days, VisibleIf: when("weekly")},
		{Name: "day", Kind: ParamNumber, LabelKey: "easydrag.param.schedule_day", Required: true, Default: 1.0, VisibleIf: when("monthly")},
		{Name: "cron", Kind: ParamCron, LabelKey: "easydrag.param.schedule_cron", Required: true, VisibleIf: when("cron")},
	}
}

// ScheduleToCron converts schedule parameters into a cron expression. Missing values fall
// back to the parameter defaults (daily at 07:00, every 15 minutes, every hour, day 1).
// The parameters can be of any type; every echo of them in an error is cut.
func ScheduleToCron(p map[string]any) (string, error) {
	mode := textParam(p, "mode")
	if mode == "" {
		mode = "daily"
	}
	number := func(key string, def, lo, hi int) (int, error) {
		v, present := p[key]
		if !present || isEmptyValue(v) {
			return def, nil
		}
		// Compare as floats: converting a huge float to int is implementation-defined.
		f, ok := toNumber(v)
		if !ok || f != math.Trunc(f) || f < float64(lo) || f > float64(hi) {
			return 0, fmt.Errorf("%s must be a whole number from %d to %d", key, lo, hi)
		}
		return int(f), nil
	}
	switch mode {
	case "interval_minutes":
		n, err := scheduleInterval(p, "minutes", 15, 60)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("*/%d * * * *", n), nil
	case "interval_hours":
		n, err := scheduleInterval(p, "hours", 1, 24)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("0 */%d * * *", n), nil
	case "daily", "weekdays", "weekly", "monthly":
		clock := textParam(p, "time")
		if clock == "" {
			clock = "07:00"
		}
		m := clockPattern.FindStringSubmatch(clock)
		if m == nil {
			return "", fmt.Errorf("time must use the format HH:MM, got %s", quoteForError(clock))
		}
		hour, _ := strconv.Atoi(m[1])
		minute, _ := strconv.Atoi(m[2])
		switch mode {
		case "daily":
			return fmt.Sprintf("%d %d * * *", minute, hour), nil
		case "weekdays":
			return fmt.Sprintf("%d %d * * 1-5", minute, hour), nil
		case "weekly":
			days, err := weekdayList(p["weekdays"])
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%d %d * * %s", minute, hour, days), nil
		default:
			day, err := number("day", 1, 1, 31)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%d %d %d * *", minute, hour, day), nil
		}
	case "cron":
		text := textParam(p, "cron")
		if len(text) > maxCronBytes {
			return "", fmt.Errorf("a cron expression can be at most %d characters", maxCronBytes)
		}
		fields := strings.Fields(text)
		if len(fields) == 0 {
			return "", fmt.Errorf("%w: cron", errParamMissing)
		}
		// A TZ= or CRON_TZ= prefix is the parser's, not a field.
		schedule := fields
		if strings.HasPrefix(fields[0], "TZ=") || strings.HasPrefix(fields[0], "CRON_TZ=") {
			schedule = fields[1:]
		}
		if n := len(schedule); n != 5 && n != 6 {
			return "", fmt.Errorf("a cron expression needs 5 or 6 fields, got %d", n)
		}
		expr := strings.Join(fields, " ")
		if err := validateCronExpr(expr); err != nil {
			return "", err
		}
		return expr, nil
	}
	return "", fmt.Errorf("unknown schedule mode %s", quoteForError(mode))
}

// scheduleInterval reads an interval of minutes or hours that repeats evenly inside
// its period (60 minutes, 24 hours): cron's */n restarts at the start of each period, so
// a step that does not divide it gives an uneven gap, for example */7 runs at :56 and
// again at :00.
func scheduleInterval(p map[string]any, key string, def, period int) (int, error) {
	v, present := p[key]
	if !present || isEmptyValue(v) {
		return def, nil
	}
	// Compare as floats: converting a huge float to int is implementation-defined.
	if f, ok := toNumber(v); ok && f == math.Trunc(f) && f >= 1 && f < float64(period) && period%int(f) == 0 {
		return int(f), nil
	}
	var steps []string
	for n := 1; n < period; n++ {
		if period%n == 0 {
			steps = append(steps, strconv.Itoa(n))
		}
	}
	return 0, fmt.Errorf("%s must divide %d (%s)", key, period, strings.Join(steps, ", "))
}

// validateCronExpr checks expr, whitespace-normalized and counted by the caller,
// with cronParser. The error text echoes the parser's message cut to 40 runes. A
// schedule that fires more than once a minute is rejected: Mission Control and the
// editor work in minutes, and the seconds field exists only because the parser accepts it.
func validateCronExpr(expr string) error {
	var (
		sched cron.Schedule
		err   error
	)
	if r := catchPanic(func() { sched, err = cronParser.Parse(expr) }); r != nil {
		// The parser slices a TZ= prefix without checking its bounds.
		return errors.New("a cron expression is not valid: the parser could not read it")
	}
	if err != nil {
		return fmt.Errorf("a cron expression is not valid: %s", quoteForError(err.Error()))
	}
	if spec, ok := sched.(*cron.SpecSchedule); !ok || spec.Second != 1 {
		return errors.New("a cron expression can run at most once a minute: its seconds field must be 0")
	}
	return nil
}

// weekdayList turns the weekdays parameter (a list of mon..sun) into a cron day list
// in Monday-to-Sunday order. An empty value counts as missing; anything else that is
// not a list of known names is an error that names the first offender.
func weekdayList(v any) (string, error) {
	if isEmptyValue(v) {
		return "", fmt.Errorf("%w: weekdays", errParamMissing)
	}
	list, ok := v.([]any)
	if !ok {
		return "", errors.New("weekdays must be a list of weekday names")
	}
	selected := map[string]bool{}
	for _, item := range list {
		name := strings.ToLower(Stringify(item))
		known := false
		for _, d := range weekdayNumbers {
			if d.key == name {
				known = true
				break
			}
		}
		if !known {
			return "", fmt.Errorf("unknown weekday %s", quoteForError(name))
		}
		selected[name] = true
	}
	var parts []string
	for _, d := range weekdayNumbers {
		if selected[d.key] {
			parts = append(parts, strconv.Itoa(d.num))
		}
	}
	return strings.Join(parts, ","), nil
}
