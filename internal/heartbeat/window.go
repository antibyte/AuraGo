package heartbeat

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
)

func parseTime(t string) (hour, min int, err error) {
	parts := strings.Split(t, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid time format: %s", t)
	}
	hour, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}
	min, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, err
	}
	if hour < 0 || hour > 23 || min < 0 || min > 59 {
		return 0, 0, fmt.Errorf("invalid time value: %s", t)
	}
	return hour, min, nil
}

func timeInMinutes(t string) int {
	h, m, _ := parseTime(t)
	return h*60 + m
}

func isInWindow(now time.Time, start, end string) bool {
	nowMin := now.Hour()*60 + now.Minute()
	startMin := timeInMinutes(start)
	endMin := timeInMinutes(end)

	if startMin <= endMin {
		return nowMin >= startMin && nowMin < endMin
	}
	// Overnight window (e.g. 22:00 - 08:00)
	return nowMin >= startMin || nowMin < endMin
}

func getActiveWindow(now time.Time, hb config.HeartbeatConfig) (*config.HeartbeatTimeWindow, string) {
	if isInWindow(now, hb.DayTimeWindow.Start, hb.DayTimeWindow.End) {
		return &hb.DayTimeWindow, hb.DayTimeWindow.Interval
	}
	if isInWindow(now, hb.NightTimeWindow.Start, hb.NightTimeWindow.End) {
		return &hb.NightTimeWindow, hb.NightTimeWindow.Interval
	}
	return nil, ""
}

func parseInterval(iv string) time.Duration {
	switch iv {
	case "15m":
		return 15 * time.Minute
	case "30m":
		return 30 * time.Minute
	case "1h":
		return 1 * time.Hour
	case "2h":
		return 2 * time.Hour
	case "4h":
		return 4 * time.Hour
	case "6h":
		return 6 * time.Hour
	case "12h":
		return 12 * time.Hour
	default:
		return 1 * time.Hour
	}
}
