# Chapter 11: Mission Control

Automate recurring tasks with AuraGo's Mission Control system. From backups to monitoring to scheduled reports – missions let you run tasks on autopilot.

## What are Missions?

**Missions** are automated, scheduled tasks that AuraGo executes based on:
- **Time schedules** (Cron expressions)
- **Manual triggers** (Run on demand)
- **System events** (Startup, specific conditions)

Think of missions as your personal cron jobs with built-in intelligence – they can execute shell commands, Python scripts, API calls, and even trigger other missions.

### Use Cases

| Use Case | Description |
|----------|-------------|
| **Backups** | Automated database dumps, file backups, cloud sync |
| **Monitoring** | Health checks, disk space alerts, service status |
| **Reports** | Daily summaries, weekly analytics, system status emails |
| **Maintenance** | Log rotation, temp file cleanup, index rebuilding |
| **Integration** | Sync with external APIs, data imports/exports |

## Concepts: Nests & Eggs

AuraGo's mission system uses two key concepts:

### Nests

A **Nest** is a target location where missions run. This can be:
- **Local** – The AuraGo host itself
- **Remote SSH** – Any server accessible via SSH
- **Docker** – Containers managed by AuraGo

> 🔍 **Deep Dive:** Nests are shared between Mission Control and Invasion Control. A Nest configured for remote deployment can also run missions. See [Chapter 12: Invasion Control](12-invasion.md) for details.

### Eggs

An **Egg** is a reusable configuration template that defines:
- What command/script to run
- Environment variables
- Working directory
- Timeout settings
- Retry policies

```
┌─────────────────────────────────────────────────────────┐
│  Egg (Template)                                         │
│  ├─ Command: backup.sh                                  │
│  ├─ Environment: DB_HOST, DB_PASS                       │
│  ├─ Working Dir: /opt/backups                           │
│  └─ Timeout: 3600s                                      │
└─────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│  Mission (Instance)                                     │
│  ├─ Egg: backup-egg                                     │
│  ├─ Nest: production-server                             │
│  ├─ Schedule: 0 2 * * * (daily at 2 AM)                 │
│  └─ Status: Active                                      │
└─────────────────────────────────────────────────────────┘
```

## Creating Missions

### Step 1: Create an Egg (Template)

Navigate to **Mission Control → Eggs → New Egg**:

```yaml
Name: database-backup
Description: Daily PostgreSQL backup
Type: shell

Command: |
  pg_dump -h $DB_HOST -U $DB_USER $DB_NAME > backup_$(date +%Y%m%d).sql
  gzip backup_$(date +%Y%m%d).sql

Environment Variables:
  DB_HOST: localhost
  DB_USER: postgres
  DB_NAME: myapp

Working Directory: /var/backups/postgres
Timeout: 1800
Max Retries: 3
Retry Delay: 300
```

### Step 2: Create a Mission

Navigate to **Mission Control → Missions → New Mission**:

```yaml
Name: nightly-db-backup
Egg: database-backup
Nest: local
Schedule: 0 2 * * *
Enabled: true
Notifications:
  On Failure: email
  On Success: none
```

> 💡 **Tip:** Use descriptive names that include the schedule and purpose, like `daily-backup-2am` or `weekly-report-monday`.

### Mission Configuration Options

| Option | Description | Example |
|--------|-------------|---------|
| `Name` | Unique mission identifier | `cleanup-temp-files` |
| `Egg` | Template to use | `cleanup-egg` |
| `Nest` | Where to run | `local`, `web-server-01` |
| `Schedule` | Cron expression | `0 */6 * * *` (every 6 hours) |
| `Enabled` | Active/inactive | `true` / `false` |
| `Timeout` | Max execution time (seconds) | `3600` |
| `Max Retries` | Retry attempts on failure | `3` |
| `Retry Delay` | Seconds between retries | `300` |

## Scheduling with Cron

AuraGo uses standard **Cron expressions** for scheduling. The format is:

```
┌───────────── Minute (0-59)
│ ┌───────────── Hour (0-23)
│ │ ┌───────────── Day of month (1-31)
│ │ │ ┌───────────── Month (1-12)
│ │ │ │ ┌───────────── Day of week (0-7, both 0 and 7 = Sunday)
│ │ │ │ │
* * * * *
```

### Common Patterns

| Schedule | Cron Expression | Description |
|----------|-----------------|-------------|
| Every minute | `* * * * *` | Continuous execution |
| Every 5 minutes | `*/5 * * * *` | Frequent checks |
| Every hour | `0 * * * *` | Top of the hour |
| Every 6 hours | `0 */6 * * *` | Four times daily |
| Daily at 2 AM | `0 2 * * *` | Nightly maintenance |
| Weekdays at 9 AM | `0 9 * * 1-5` | Business hours only |
| Weekly on Sunday | `0 0 * * 0` | Weekly reports |
| Monthly 1st at midnight | `0 0 1 * *` | Monthly cleanup |

### Advanced Cron Examples

```
# Every 30 minutes during business hours
*/30 9-17 * * 1-5

# Every 2 hours on weekends
0 */2 * * 0,6

# First Monday of each month
0 0 1-7 * 1

# Every 15 minutes, but not at night (9 PM - 6 AM)
*/15 6-21 * * *
```

> ⚠️ **Warning:** Be careful with `* * * * *` (every minute). It can overwhelm your system if the task takes longer than a minute to execute.

## Manual Execution

Sometimes you need to run a mission immediately:

### Via Web UI

1. Navigate to **Mission Control → Missions**
2. Find your mission in the list
3. Click the **▶️ Run** button
4. Monitor execution in real-time

### Via Chat

```
You: Run mission nightly-db-backup
Agent: 🚀 Executing mission "nightly-db-backup"...
     Target: local
     Egg: database-backup
     
     ⏳ Running...
     
     ✅ Mission completed successfully
     Duration: 45 seconds
     Exit code: 0
```

### Via API

```bash
curl -X POST http://localhost:8088/api/missions/nightly-db-backup/run \
  -H "Authorization: Bearer YOUR_TOKEN"
```

## Monitoring Missions

### Mission Dashboard

The **Mission Control** dashboard provides:

| Metric | Description |
|--------|-------------|
| **Status Overview** | Active, paused, failed missions |
| **Last Run** | Timestamp of most recent execution |
| **Next Run** | Scheduled time for next execution |
| **Success Rate** | Percentage of successful runs (last 30 days) |
| **Average Duration** | Mean execution time |

### Execution History

Each mission maintains a detailed log:

```
┌─────────────────────────────────────────────────────────┐
│ Mission: nightly-db-backup                              │
├─────────────────────────────────────────────────────────┤
│ Run #1245 - 2024-01-15 02:00:03                         │
│ Status: ✅ SUCCESS                                      │
│ Duration: 45.2s                                         │
│ Output: backup_20240115.sql.gz (2.3 MB)                 │
├─────────────────────────────────────────────────────────┤
│ Run #1244 - 2024-01-14 02:00:01                         │
│ Status: ✅ SUCCESS                                      │
│ Duration: 44.8s                                         │
├─────────────────────────────────────────────────────────┤
│ Run #1243 - 2024-01-13 02:00:02                         │
│ Status: ❌ FAILED                                       │
│ Duration: 1800.0s (timeout)                             │
│ Error: Connection timeout to database                   │
└─────────────────────────────────────────────────────────┘
```

### Real-time Logs

Click on any execution to see detailed logs:

```
[2024-01-15 02:00:03] INFO: Starting mission nightly-db-backup
[2024-01-15 02:00:03] INFO: Connecting to nest 'local'
[2024-01-15 02:00:03] INFO: Executing egg 'database-backup'
[2024-01-15 02:00:03] INFO: pg_dump started
[2024-01-15 02:00:45] INFO: pg_dump completed (42s)
[2024-01-15 02:00:47] INFO: Compression completed
[2024-01-15 02:00:48] INFO: Backup size: 2.3 MB
[2024-01-15 02:00:48] INFO: Mission completed successfully
```

## Mission Statuses and Lifecycle

### Status Flow

```
┌─────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐
│ PENDING │───▶│ RUNNING  │───▶│ SUCCESS  │    │          │
└─────────┘    └──────────┘    └──────────┘    │          │
                    │                          │          │
                    ▼                          │          │
              ┌──────────┐    ┌──────────┐    │  FINAL   │
              │ RETRYING │───▶│  FAILED  │───▶│  STATES  │
              └──────────┘    └──────────┘    │          │
                                              │          │
┌─────────┐                                   │          │
│ PAUSED  │──────────────────────────────────▶│          │
└─────────┘                                   └──────────┘
```

### Status Descriptions

| Status | Description | Action Required |
|--------|-------------|-----------------|
| **Pending** | Waiting for scheduled time | None |
| **Running** | Currently executing | Monitor progress |
| **Success** | Completed without errors | None |
| **Failed** | Error occurred, retries exhausted | Check logs |
| **Retrying** | Failed but will retry | Monitor retries |
| **Paused** | Manually disabled | Resume when ready |
| **Timeout** | Exceeded max execution time | Review timeout setting |

## Best Practices for Automation

### 1. Start Simple

Begin with infrequent, low-risk missions:
```
# Weekly instead of every minute
0 0 * * 0  # Weekly on Sunday
```

### 2. Use Appropriate Timeouts

```yaml
# Short tasks: 5 minutes
Timeout: 300

# Database backups: 30 minutes
Timeout: 1800

# Large data processing: 2 hours
Timeout: 7200
```

### 3. Implement Retry Logic

```yaml
Max Retries: 3
Retry Delay: 300  # 5 minutes between retries
```

### 4. Set Up Notifications

```yaml
Notifications:
  On Failure: telegram  # Immediate alert
  On Success: none      # Silent on success (or log only)
```

### 5. Monitor Disk Space

> ⚠️ **Warning:** Log files and mission outputs can accumulate quickly. Always include cleanup in your missions or create separate cleanup missions.

### 6. Test Before Scheduling

Always run missions manually first:
```
You: Run mission test-backup
Agent: Testing backup mission...
```

### 7. Use Meaningful Names

| ❌ Bad | ✅ Good |
|--------|---------|
| `backup` | `postgres-daily-backup-2am` |
| `check` | `disk-space-check-6am` |
| `report` | `weekly-analytics-report-monday` |

## Examples: Backup, Monitoring, Reports

### Example 1: Database Backup

**Egg: postgres-backup**
```yaml
Type: shell
Command: |
  #!/bin/bash
  BACKUP_DIR="/var/backups/postgres"
  DATE=$(date +%Y%m%d_%H%M%S)
  FILENAME="postgres_${DATE}.sql"
  
  # Create backup
  pg_dump -h localhost -U postgres mydb > "$BACKUP_DIR/$FILENAME"
  
  # Compress
  gzip "$BACKUP_DIR/$FILENAME"
  
  # Keep only last 7 days
  find "$BACKUP_DIR" -name "postgres_*.sql.gz" -mtime +7 -delete
  
  echo "Backup completed: ${FILENAME}.gz"

Environment:
  PGPASSWORD: ${vault.postgres.password}

Timeout: 3600
```

**Mission: daily-postgres-backup**
```yaml
Egg: postgres-backup
Nest: local
Schedule: 0 2 * * *  # Daily at 2 AM
Notifications:
  On Failure: email
```

### Example 2: System Monitoring

**Egg: health-check**
```yaml
Type: python
Command: |
  import psutil
  import requests
  
  # Check disk space
  disk = psutil.disk_usage('/')
  disk_percent = disk.percent
  
  # Check memory
  memory = psutil.virtual_memory()
  memory_percent = memory.percent
  
  # Check load
  load = psutil.getloadavg()[0]
  
  alerts = []
  
  if disk_percent > 90:
      alerts.append(f"🚨 Disk usage critical: {disk_percent}%")
  elif disk_percent > 80:
      alerts.append(f"⚠️ Disk usage high: {disk_percent}%")
  
  if memory_percent > 95:
      alerts.append(f"🚨 Memory usage critical: {memory_percent}%")
  
  if alerts:
      message = "System Health Check\\n\\n" + "\\n".join(alerts)
      # Send notification via AuraGo's notify tool
      print(f"ALERT: {message}")
      exit(1)
  else:
      print(f"✅ All systems healthy")
      print(f"   Disk: {disk_percent}%")
      print(f"   Memory: {memory_percent}%")
      print(f"   Load: {load}")

Timeout: 60
```

**Mission: system-health-monitor**
```yaml
Egg: health-check
Nest: local
Schedule: */15 * * * *  # Every 15 minutes
Notifications:
  On Failure: telegram
```

### Example 3: Weekly Report

**Egg: weekly-analytics**
```yaml
Type: shell
Command: |
  #!/bin/bash
  REPORT_DATE=$(date +"%Y-%m-%d")
  REPORT_FILE="/tmp/weekly_report_${REPORT_DATE}.txt"
  
  echo "Weekly System Report - ${REPORT_DATE}" > "$REPORT_FILE"
  echo "======================================" >> "$REPORT_FILE"
  echo "" >> "$REPORT_FILE"
  
  # System uptime
  echo "Uptime:" >> "$REPORT_FILE"
  uptime >> "$REPORT_FILE"
  echo "" >> "$REPORT_FILE"
  
  # Disk usage
  echo "Disk Usage:" >> "$REPORT_FILE"
  df -h >> "$REPORT_FILE"
  echo "" >> "$REPORT_FILE"
  
  # Memory usage
  echo "Memory Usage:" >> "$REPORT_FILE"
  free -h >> "$REPORT_FILE"
  echo "" >> "$REPORT_FILE"
  
  # Docker container status
  echo "Docker Containers:" >> "$REPORT_FILE"
  docker ps --format "table {{.Names}}\\t{{.Status}}" >> "$REPORT_FILE"
  
  # Email the report (if email tool configured)
  echo "Report generated at: $REPORT_FILE"

Timeout: 300
```

**Mission: weekly-system-report**
```yaml
Egg: weekly-analytics
Nest: local
Schedule: 0 9 * * 1  # Mondays at 9 AM
Notifications:
  On Success: email
  On Failure: email
```

### Example 4: API Data Sync

**Egg: sync-external-api**
```yaml
Type: python
Command: |
  import json
  import urllib.request
  from datetime import datetime
  
  API_URL = "https://api.example.com/data"
  OUTPUT_FILE = f"/data/sync_{datetime.now().strftime('%Y%m%d')}.json"
  
  try:
      with urllib.request.urlopen(API_URL, timeout=30) as response:
          data = json.loads(response.read())
          
      with open(OUTPUT_FILE, 'w') as f:
          json.dump(data, f, indent=2)
          
      print(f"✅ Synced {len(data)} records to {OUTPUT_FILE}")
      
  except Exception as e:
      print(f"❌ Sync failed: {e}")
      exit(1)

Environment:
  API_KEY: ${vault.api.key}

Timeout: 120
Max Retries: 3
Retry Delay: 60
```

**Mission: hourly-api-sync**
```yaml
Egg: sync-external-api
Nest: local
Schedule: 0 * * * *  # Every hour
```

---

> 💡 **Tip:** Combine multiple missions for complex workflows. Mission A can trigger Mission B via the `mission_run` tool, creating powerful automation chains.

> 🔍 **Deep Dive:** For advanced mission orchestration, see [Chapter 15: Co-Agents](15-coagents.md) to learn how parallel agents can coordinate complex multi-step missions.

---

## Next Steps

- **[Chapter 12: Invasion Control](12-invasion.md)** – Deploy missions to remote servers
- **[Chapter 13: Dashboard](13-dashboard.md)** – Monitor mission metrics
- **[Chapter 15: Co-Agents](15-coagents.md)** – Advanced mission orchestration
