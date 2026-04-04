package main
import (
"os"
"strings"
)

func main() {
b, _ := os.ReadFile("internal/services/optimizer/worker.go")
s := string(b)

old1 := 	icker := time.NewTicker(w.checkInterval)
new1 := w.runEvaluationCycle(ctx)
w.runCreationCycle(ctx)

ticker := time.NewTicker(w.checkInterval)
s = strings.Replace(s, old1, new1, 1)

old2 := ar newSuccessRate float64
w.db.db.QueryRow( + "" + 
SELECT CAST(SUM(CASE WHEN success=1 THEN 1 ELSE 0 END) AS FLOAT) / COUNT(*)
FROM tool_traces
WHERE tool_name = ? AND prompt_version = ? + "" + , toolName, versionTag).Scan(&newSuccessRate)

var baselineSuccessRate float64
w.db.db.QueryRow( + "" + 
SELECT CAST(SUM(CASE WHEN success=1 THEN 1 ELSE 0 END) AS FLOAT) / COUNT(*)
FROM (
SELECT success
FROM tool_traces
WHERE tool_name = ? AND prompt_version = 'v1'
ORDER BY timestamp DESC LIMIT 50
) + "" + , toolName).Scan(&baselineSuccessRate)

new2 := ar newSuccessRate float64
err = w.db.db.QueryRow( + "" + 
SELECT CAST(SUM(CASE WHEN success=1 THEN 1 ELSE 0 END) AS FLOAT) / COUNT(*)
FROM tool_traces
WHERE tool_name = ? AND prompt_version = ? + "" + , toolName, versionTag).Scan(&newSuccessRate)
if err != nil {
continue
}

var baselineSuccessRate float64
err = w.db.db.QueryRow( + "" + 
SELECT CAST(SUM(CASE WHEN success=1 THEN 1 ELSE 0 END) AS FLOAT) / COUNT(*)
FROM (
SELECT success
FROM tool_traces
WHERE tool_name = ? AND prompt_version = 'v1'
ORDER BY timestamp DESC LIMIT 50
) + "" + , toolName).Scan(&baselineSuccessRate)
if err != nil {
continue
}
s = strings.Replace(s, old2, new2, 1)

old3 := if stats.TotalTraceEvents > 0 {
var succ int
db.db.QueryRow( + "" + SELECT COUNT(*) FROM tool_traces WHERE success = 1 + "" + ).Scan(&succ)
stats.GlobalSuccessRate = float64(succ) / float64(stats.TotalTraceEvents)
}
new3 := ar recentTotal int
db.db.QueryRow( + "" + SELECT COUNT(*) FROM tool_traces WHERE timestamp > datetime('now', '-7 days') + "" + ).Scan(&recentTotal)
if recentTotal > 0 {
var succ int
db.db.QueryRow( + "" + SELECT COUNT(*) FROM tool_traces WHERE success = 1 AND timestamp > datetime('now', '-7 days') + "" + ).Scan(&succ)
stats.GlobalSuccessRate = float64(succ) / float64(recentTotal)
} else {
stats.GlobalSuccessRate = 0
}

bApi, _ := os.ReadFile("internal/services/optimizer/api.go")
sApi := string(bApi)
sApi = strings.Replace(sApi, old3, new3, 1)
os.WriteFile("internal/services/optimizer/api.go", []byte(sApi), 0644)
os.WriteFile("internal/services/optimizer/worker.go", []byte(s), 0644)
}
