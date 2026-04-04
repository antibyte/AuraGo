package main
import (
"os"
"strings"
"fmt"
)

func main() {
b, err := os.ReadFile("internal/services/optimizer/worker.go")
if err != nil { panic(err) }

lines := strings.Split(string(b), "\n")
for i, line := range lines {
// Fix 2.3
if strings.Contains(line, "ticker := time.NewTicker(w.checkInterval)") {
if !strings.Contains(lines[i-1], "w.runCreationCycle") {
lines[i] = "\tw.runEvaluationCycle(ctx)\n\tw.runCreationCycle(ctx)\n\n\t" + strings.TrimSpace(line)
}
}

// Fix 1.4: add err = and if err != nil check
if strings.Contains(line, "w.db.db.QueryRow(") && strings.Contains(line, "Scan(&newSuccessRate)") {
lines[i] = "\t\terr = w.db.db.QueryRow(" + strings.Split(line, "w.db.db.QueryRow(")[1]
}

// insert new check after the statement finishing the Scan(&newSuccessRate)
if strings.Contains(line, ").Scan(&newSuccessRate)") {
lines[i] = line + "\n\t\tif err != nil {\n\t\t\tcontinue\n\t\t}"
}

if strings.Contains(line, "w.db.db.QueryRow(") && strings.Contains(line, "Scan(&baselineSuccessRate)") {
            parts := strings.Split(line, "w.db.db.QueryRow(")
            if len(parts) > 1 {
    lines[i] = "\t\terr = w.db.db.QueryRow(" + parts[1]
            } else {
                lines[i] = strings.Replace(line, "w.db.db.QueryRow", "err = w.db.db.QueryRow", 1)
            }
}

if strings.Contains(line, ").Scan(&baselineSuccessRate)") {
lines[i] = line + "\n\t\tif err != nil {\n\t\t\tcontinue\n\t\t}"
}
}

out := strings.Join(lines, "\n")
os.WriteFile("internal/services/optimizer/worker.go", []byte(out), 0644)
fmt.Println("Worker rewritten")


bApi, err := os.ReadFile("internal/services/optimizer/api.go")
if err != nil { panic(err) }

linesApi := strings.Split(string(bApi), "\n")
for i, line := range linesApi {
if strings.Contains(line, "if stats.TotalTraceEvents > 0 {") {
            linesApi[i] = "\tvar recentTotal int\n\tdb.db.QueryRow(\"SELECT COUNT(*) FROM tool_traces WHERE timestamp > datetime('now', '-7 days')\").Scan(&recentTotal)\n\tif recentTotal > 0 {"
            continue
}
        if strings.Contains(line, "db.db.QueryRow(") && strings.Contains(line, "SELECT COUNT(*) FROM tool_traces WHERE success = 1") {
            linesApi[i] = "\t\tdb.db.QueryRow(\"SELECT COUNT(*) FROM tool_traces WHERE success = 1 AND timestamp > datetime('now', '-7 days')\").Scan(&succ)"
            continue
        }
        if strings.Contains(line, "stats.GlobalSuccessRate = ") {
if !strings.Contains(line, "stats.GlobalSuccessRate = 0") {
            linesApi[i] = "\t\tstats.GlobalSuccessRate = float64(succ) / float64(recentTotal)\n\t} else {\n\t\tstats.GlobalSuccessRate = 0"
}
            continue
        }
}

outApi := strings.Join(linesApi, "\n")
os.WriteFile("internal/services/optimizer/api.go", []byte(outApi), 0644)
fmt.Println("API rewritten")
}
