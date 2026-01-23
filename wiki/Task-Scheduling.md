# Task Scheduling

Cron-like job scheduling for automated tasks.

## Overview

Task scheduling allows:
- Periodic execution of actions
- Background jobs
- Scheduled data sync
- Automated maintenance

## Defining Tasks

### Schema Definition

```yaml
Tasks:
  - Name: daily_report
    Label: Generate Daily Report
    Schedule: "@daily"
    ActionName: generate_report
    EntityName: report
    AsUserEmail: admin@example.com
    Attributes:
      report_type: daily
      format: pdf

  - Name: hourly_sync
    Label: Sync External Data
    Schedule: "@every 1h"
    ActionName: sync_external_data
    EntityName: integration
```

## Schedule Syntax

### Predefined Schedules

| Schedule | Description |
|----------|-------------|
| `@yearly` | Once a year (Jan 1, midnight) |
| `@monthly` | First day of month, midnight |
| `@weekly` | Sunday at midnight |
| `@daily` | Every day at midnight |
| `@hourly` | Every hour |

### Interval Schedules

| Schedule | Description |
|----------|-------------|
| `@every 5m` | Every 5 minutes |
| `@every 1h` | Every hour |
| `@every 30s` | Every 30 seconds |
| `@every 24h` | Every 24 hours |

### Cron Expressions

Standard cron format: `minute hour day month weekday`

```yaml
Tasks:
  # Every day at 3 AM
  - Schedule: "0 3 * * *"

  # Every Monday at 9 AM
  - Schedule: "0 9 * * 1"

  # Every 15 minutes
  - Schedule: "*/15 * * * *"

  # First day of month at noon
  - Schedule: "0 12 1 * *"
```

## Task Properties

| Property | Description |
|----------|-------------|
| Name | Unique task identifier |
| Label | Display name |
| Schedule | Cron expression or predefined |
| ActionName | Action to execute |
| EntityName | Target entity |
| AsUserEmail | Execute as this user |
| Attributes | Action parameters |
| IsActive | Enable/disable task |

## Managing Tasks via API

### List Tasks

```bash
curl http://localhost:6336/api/task \
  -H "Authorization: Bearer $TOKEN"
```

### Create Task

```bash
curl -X POST http://localhost:6336/api/task \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "task",
      "attributes": {
        "name": "cleanup_old_data",
        "label": "Cleanup Old Data",
        "schedule": "@daily",
        "action_name": "cleanup",
        "entity_name": "temp_data",
        "as_user_email": "system@example.com",
        "is_active": true,
        "attributes": {
          "days_old": 30
        }
      }
    }
  }'
```

### Update Task

```bash
curl -X PATCH http://localhost:6336/api/task/TASK_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "task",
      "id": "TASK_ID",
      "attributes": {
        "schedule": "@every 2h",
        "is_active": true
      }
    }
  }'
```

### Disable Task

```bash
curl -X PATCH http://localhost:6336/api/task/TASK_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "task",
      "id": "TASK_ID",
      "attributes": {
        "is_active": false
      }
    }
  }'
```

## Common Use Cases

### Email Sync

```yaml
Tasks:
  - Name: mail_sync
    Schedule: "@every 1h"
    ActionName: mail_servers_sync
    EntityName: mail_server
    Label: Sync Mail Servers
```

### Data Backup

```yaml
Tasks:
  - Name: daily_backup
    Schedule: "0 2 * * *"  # 2 AM daily
    ActionName: export_data
    EntityName: world
    Attributes:
      format: json
      destination: backup_store
```

### Cloud Storage Sync

```yaml
Tasks:
  - Name: storage_sync
    Schedule: "@every 6h"
    ActionName: site_sync_storage
    EntityName: site
    Attributes:
      site_id: SITE_ID
```

### Report Generation

```yaml
Tasks:
  - Name: weekly_report
    Schedule: "0 8 * * 1"  # Monday 8 AM
    ActionName: generate_report
    EntityName: analytics
    AsUserEmail: reports@example.com
    Attributes:
      type: weekly
      recipients: ["manager@example.com"]
```

### Data Cleanup

```yaml
Tasks:
  - Name: cleanup_sessions
    Schedule: "@daily"
    ActionName: cleanup_old_sessions
    EntityName: session
    Attributes:
      max_age_days: 7
```

## Task Execution Context

Tasks run with:
- User context from `AsUserEmail`
- Full action permissions
- Background execution (no client response)

## Monitoring Tasks

### Check Task Status

```bash
curl http://localhost:6336/api/task/TASK_ID \
  -H "Authorization: Bearer $TOKEN"
```

### View Task History

Check timeline for task executions:

```bash
curl 'http://localhost:6336/api/timeline?query=[{"column":"event_type","operator":"is","value":"task"}]' \
  -H "Authorization: Bearer $TOKEN"
```

## Error Handling

Failed tasks:
- Logged to timeline
- Don't affect other scheduled tasks
- Retry on next scheduled run

## Default System Tasks

Daptin includes default tasks:

| Task | Schedule | Purpose |
|------|----------|---------|
| mail_sync | @every 1h | Sync mail servers |

## Best Practices

1. **Use appropriate intervals** - Don't schedule too frequently
2. **Set user context** - Always specify `AsUserEmail`
3. **Monitor task execution** - Check timeline for failures
4. **Test actions first** - Verify action works before scheduling
5. **Use meaningful names** - Clear task identification
