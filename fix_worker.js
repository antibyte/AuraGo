const fs = require('fs');
let content = fs.readFileSync('internal/services/optimizer/worker.go', 'utf8');

// Fix 2.3
content = content.replace(
    /        ticker := time\.NewTicker\(w\.checkInterval\);\n        defer ticker\.Stop\(\);\n\n        for \{/g,
    \        w.runEvaluationCycle(ctx)
        w.runCreationCycle(ctx)

        ticker := time.NewTicker(w.checkInterval)
        defer ticker.Stop()

        for {\
);

// Fix 1.4
const oldScan = \                var newSuccessRate float64
                w.db.db.QueryRow(\\\
                        SELECT CAST(SUM(CASE WHEN success=1 THEN 1 ELSE 0 END) AS FLOAT) / COUNT(*)
                        FROM tool_traces
                        WHERE tool_name = ? AND prompt_version = ?\\\, toolName, versionTag).Scan(&newSuccessRate)

                var baselineSuccessRate float64
                w.db.db.QueryRow(\\\
                        SELECT CAST(SUM(CASE WHEN success=1 THEN 1 ELSE 0 END) AS FLOAT) / COUNT(*)
                        FROM (
                                SELECT success
                                FROM tool_traces
                                WHERE tool_name = ? AND prompt_version = 'v1'   
                                ORDER BY timestamp DESC LIMIT 50
                        )\\\, toolName).Scan(&baselineSuccessRate)\

const newScan = \                var newSuccessRate float64
                err = w.db.db.QueryRow(\\\
                        SELECT CAST(SUM(CASE WHEN success=1 THEN 1 ELSE 0 END) AS FLOAT) / COUNT(*)
                        FROM tool_traces
                        WHERE tool_name = ? AND prompt_version = ?\\\, toolName, versionTag).Scan(&newSuccessRate)
                if err != nil {
                        continue
                }

                var baselineSuccessRate float64
                err = w.db.db.QueryRow(\\\
                        SELECT CAST(SUM(CASE WHEN success=1 THEN 1 ELSE 0 END) AS FLOAT) / COUNT(*)
                        FROM (
                                SELECT success
                                FROM tool_traces
                                WHERE tool_name = ? AND prompt_version = 'v1'
                                ORDER BY timestamp DESC LIMIT 50
                        )\\\, toolName).Scan(&baselineSuccessRate)
                if err != nil {
                        continue
                }\

// remove trailing spaces that might exist in the old pattern by using regex for flexibility
content = content.replace(/var newSuccessRate float64[\\s\\S]*?Scan\\(&baselineSuccessRate\\)/g, newScan);

fs.writeFileSync('internal/services/optimizer/worker.go', content, 'utf8');
