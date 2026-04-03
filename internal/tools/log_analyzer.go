package tools

import (
"bufio"
"encoding/json"
"fmt"
"os"
"regexp"
"strings"
"time"
)

type LogAnalyzer struct{}

func NewLogAnalyzer() *LogAnalyzer {
return &LogAnalyzer{}
}

var syslogRegex = regexp.MustCompile(`^([A-Z][a-z]{2}\s+\d+\s+\d{2}:\d{2}:\d{2})`)

func (a *LogAnalyzer) extractLevel(line string) string {
lower := strings.ToLower(line)
if strings.Contains(lower, "error") || strings.Contains(lower, "fatal") { return "error" }
if strings.Contains(lower, "warn") { return "warn" }
if strings.Contains(lower, "info") { return "info" }
if strings.Contains(lower, "debug") { return "debug" }
return "unknown"
}

func (a *LogAnalyzer) parseJSONLog(line string) (time.Time, string, bool) {
var data map[string]interface{}
if err := json.Unmarshal([]byte(line), &data); err != nil {
return time.Time{}, "", false
}

level := "unknown"
if l, ok := data["level"].(string); ok {
level = strings.ToLower(l)
} else if l, ok := data["severity"].(string);ok {
level = strings.ToLower(l)
} else {
level = a.extractLevel(line)
}

var ts time.Time
if tObj, ok := data["time"].(string); ok {
ts, _ = time.Parse(time.RFC3339, tObj)
} else if tObj, ok := data["timestamp"].(string); ok {
ts, _ = time.Parse(time.RFC3339, tObj)
}

return ts, level, true
}

func (a *LogAnalyzer) parseSyslog(line string) (time.Time, string, bool) {
matches := syslogRegex.FindStringSubmatch(line)
if len(matches) > 1 {
// Use current year
timeStr := fmt.Sprintf("%d %s", time.Now().Year(), matches[1])
ts, err := time.Parse("2006 Jan _2 15:04:05", timeStr)
if err == nil {
return ts, a.extractLevel(line), true
}
}
return time.Time{}, "", false
}

func (a *LogAnalyzer) Analyze(filePath string, startTimeStr, endTimeStr string, levels []string, maxLines int) ([]string, error) {
if maxLines <= 0 {
maxLines = 100
}

var startTime, endTime time.Time
var err error
if startTimeStr != "" {
startTime, err = time.Parse(time.RFC3339, startTimeStr)
if err != nil {
return nil, fmt.Errorf("invalid startTime format. Use RFC3339 (e.g. 2026-04-03T10:00:00Z)")
}
}
if endTimeStr != "" {
endTime, err = time.Parse(time.RFC3339, endTimeStr)
if err != nil {
return nil, fmt.Errorf("invalid endTime format. Use RFC3339")
}
}

filterLevels := make(map[string]bool)
for _, l := range levels {
filterLevels[strings.ToLower(l)] = true
}

file, err := os.Open(filePath)
if err != nil {
return nil, fmt.Errorf("failed to open file: %w", err)
}
defer file.Close()

var results []string
scanner := bufio.NewScanner(file)
for scanner.Scan() {
line := scanner.Text()
if strings.TrimSpace(line) == "" {
continue
}

var ts time.Time
level := "unknown"
isParsed := false

if ts, level, isParsed = a.parseJSONLog(line); !isParsed {
if ts, level, isParsed = a.parseSyslog(line); !isParsed {
// plain text fallback
level = a.extractLevel(line)
// try to find an RFC3339 in the first 30 chars
if len(line) > 30 {
plainParts := strings.Fields(line)
for i:=0; i<len(plainParts) && i<3; i++ {
if t, err := time.Parse(time.RFC3339, plainParts[i]); err == nil {
ts = t
break
}
}
}
}
}

if !startTime.IsZero() && !ts.IsZero() && ts.Before(startTime) {
continue
}
if !endTime.IsZero() && !ts.IsZero() && ts.After(endTime) {
continue
}
if len(filterLevels) > 0 && !filterLevels[level] && !(level == "unknown" && filterLevels["unknown"]) {
continue
}

results = append(results, line)
if len(results) >= maxLines {
break
}
}

if err := scanner.Err(); err != nil {
return nil, fmt.Errorf("error reading file: %w", err)
}

return results, nil
}

