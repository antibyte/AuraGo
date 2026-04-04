package main
import (
"os"
"strings"
"fmt"
)

func main() {
b, _ := os.ReadFile("internal/services/optimizer/worker.go")
s := string(b)

old1 := "ticker := time.NewTicker(w.checkInterval)"
new1 := "w.runEvaluationCycle(ctx)\n\tw.runCreationCycle(ctx)\n\n\tticker := time.NewTicker(w.checkInterval)"
if !strings.Contains(s, "w.runEvaluationCycle(ctx)\n\tw.runCreationCycle(ctx)") {
s = strings.Replace(s, old1, new1, 1)
}

out := strings.Split(s, "\n")
for i:=0; i<len(out); i++ {
if strings.Contains(out[i], "Scan(&newSuccessRate)") && !strings.Contains(out[i], "err = ") {
out[i] = strings.Replace(out[i], "w.db.db.QueryRow", "err = w.db.db.QueryRow", 1)
out[i] = out[i] + "\n\t\tif err != nil {\n\t\t\tcontinue\n\t\t}"
}
if strings.Contains(out[i], "Scan(&baselineSuccessRate)") && !strings.Contains(out[i], "err = ") {
out[i] = strings.Replace(out[i], "w.db.db.QueryRow", "err = w.db.db.QueryRow", 1)
out[i] = out[i] + "\n\t\tif err != nil {\n\t\t\tcontinue\n\t\t}"
}
}

os.WriteFile("internal/services/optimizer/worker.go", []byte(strings.Join(out, "\n")), 0644)


bApi, _ := os.ReadFile("internal/services/optimizer/api.go")
sApi := string(bApi)
oldApi := "if stats.TotalTraceEvents > 0 {\n\t\tvar succ int\n\t\tdb.db.QueryRow(SELECT COUNT(*) FROM tool_traces WHERE success = 1).Scan(&succ)\n\t\tstats.GlobalSuccessRate = float64(succ) / float64(stats.TotalTraceEvents)\n\t}"

newApi := "var recentTotal int\n\tdb.db.QueryRow(SELECT COUNT(*) FROM tool_traces WHERE timestamp > datetime('now', '-7 days')).Scan(&recentTotal)\n\tif recentTotal > 0 {\n\t\tvar succ int\n\t\tdb.db.QueryRow(SELECT COUNT(*) FROM tool_traces WHERE success = 1 AND timestamp > datetime('now', '-7 days')).Scan(&succ)\n\t\tstats.GlobalSuccessRate = float64(succ) / float64(recentTotal)\n\t} else {\n\t\tstats.GlobalSuccessRate = 0\n\t}"

sApi = strings.ReplaceAll(sApi, "\r\n", "\n")
sApi = strings.ReplaceAll(sApi, oldApi, newApi)
os.WriteFile("internal/services/optimizer/api.go", []byte(sApi), 0644)

fmt.Println("Done")
}
