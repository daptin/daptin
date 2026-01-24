# Task Scheduling

Cron-based job scheduling for automated background tasks using the `task` table.

## Overview

Daptin's task scheduling system:
- Executes actions on a schedule (cron expressions or intervals)
- Runs tasks as a specific user with their permissions
- Uses [robfig/cron/v3](https://github.com/robfig/cron) for scheduling
- Supports any action defined in the system
- Runs within database transactions for safety

## The `task` Table

Tasks are stored in the `task` table:

| Column | Type | Description |
|--------|------|-------------|
| `name` | varchar(100) | Unique task identifier |
| `action_name` | varchar(100) | Action to execute |
| `entity_name` | varchar(100) | Target entity/table |
| `schedule` | varchar(100) | Cron expression or interval |
| `active` | bool | Enable/disable the task |
| `attributes` | text (JSON) | Parameters passed to the action |
| `job_type` | varchar(100) | Task category (e.g., backup, sync) |

The task also has a relationship to `user_account` via `as_user_id` to specify execution context.

## Schedule Syntax

### Predefined Schedules

| Schedule | Description |
|----------|-------------|
| `@yearly` or `@annually` | January 1, 00:00 |
| `@monthly` | First day of month, 00:00 |
| `@weekly` | Sunday, 00:00 |
| `@daily` or `@midnight` | Every day, 00:00 |
| `@hourly` | Every hour on the hour |

### Interval Schedules

| Schedule | Description |
|----------|-------------|
| `@every 15s` | Every 15 seconds |
| `@every 5m` | Every 5 minutes |
| `@every 1h` | Every hour |
| `@every 30m` | Every 30 minutes |
| `@every 24h` | Every 24 hours |

### Cron Expressions

Standard 5-field cron format: `minute hour day month weekday`

| Expression | Description |
|------------|-------------|
| `0 0 * * *` | Daily at midnight |
| `0 3 * * *` | Daily at 3 AM |
| `0 9 * * 1` | Every Monday at 9 AM |
| `*/15 * * * *` | Every 15 minutes |
| `0 */6 * * *` | Every 6 hours |
| `0 12 1 * *` | First day of month at noon |
| `0 9-17 * * 1-5` | 9 AM to 5 PM on weekdays |

## Managing Tasks via API

### List All Tasks

```bash
curl http://localhost:6336/api/task \
  -H "Authorization: Bearer $TOKEN"
```

### Create a Task

```bash
curl -X POST http://localhost:6336/api/task \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "task",
      "attributes": {
        "name": "daily_cleanup",
        "action_name": "cleanup_old_data",
        "entity_name": "session",
        "schedule": "@daily",
        "active": true,
        "job_type": "maintenance",
        "attributes": "{\"days_old\": 30}"
      }
    }
  }'
```

### Update a Task

```bash
curl -X PATCH "http://localhost:6336/api/task/$TASK_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "task",
      "id": "'"$TASK_ID"'",
      "attributes": {
        "schedule": "@every 2h",
        "active": true
      }
    }
  }'
```

### Disable a Task

```bash
curl -X PATCH "http://localhost:6336/api/task/$TASK_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "task",
      "id": "'"$TASK_ID"'",
      "attributes": {
        "active": false
      }
    }
  }'
```

### Delete a Task

```bash
curl -X DELETE "http://localhost:6336/api/task/$TASK_ID" \
  -H "Authorization: Bearer $TOKEN"
```

## Task Execution

### Execution Flow

When a task triggers:

1. **Transaction Start** - Database transaction begins
2. **User Context** - Load user from `as_user_id` relationship
3. **Permission Setup** - Apply user's groups and permissions
4. **Action Request** - Build request to `/action/{entity_name}/{action_name}`
5. **Execute** - Call `HandleActionRequest()` on the target resource
6. **Commit/Rollback** - Transaction completes based on result

### Execution Context

Tasks execute with:
- User permissions from `as_user_id` relationship
- Full action capabilities
- Background execution (no HTTP response)
- Database transaction wrapping

## Built-in System Tasks

Daptin creates these tasks automatically:

| Task | Schedule | Entity | Action | Purpose |
|------|----------|--------|--------|---------|
| Mail Server Sync | `@every 1h` | `mail_server` | `sync_mail_servers` | Sync IMAP accounts |
| Column Storage Sync | `@every 30m` | `world` | `sync_column_storage` | Sync asset columns to cloud |
| Site Storage Sync | `@every 1h` | `site` | `sync_site_storage` | Sync subsites to cloud storage |

## Common Task Patterns

### Email Synchronization

```bash
curl -X POST http://localhost:6336/api/task \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "task",
      "attributes": {
        "name": "mail_sync",
        "action_name": "sync_mail_servers",
        "entity_name": "mail_server",
        "schedule": "@every 1h",
        "active": true,
        "job_type": "sync",
        "attributes": "{}"
      }
    }
  }'
```

### Data Export/Backup

```bash
curl -X POST http://localhost:6336/api/task \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "task",
      "attributes": {
        "name": "daily_backup",
        "action_name": "export_data",
        "entity_name": "world",
        "schedule": "0 2 * * *",
        "active": true,
        "job_type": "backup",
        "attributes": "{\"format\": \"json\", \"table_name\": \"orders\"}"
      }
    }
  }'
```

### Cloud Storage Sync

```bash
curl -X POST http://localhost:6336/api/task \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "task",
      "attributes": {
        "name": "storage_sync",
        "action_name": "sync_column_storage",
        "entity_name": "world",
        "schedule": "@every 30m",
        "active": true,
        "job_type": "sync",
        "attributes": "{\"table_name\": \"documents\", \"column_name\": \"attachment\"}"
      }
    }
  }'
```

### Integration Sync

```bash
curl -X POST http://localhost:6336/api/task \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "task",
      "attributes": {
        "name": "crm_sync",
        "action_name": "execute_integration",
        "entity_name": "integration",
        "schedule": "@every 6h",
        "active": true,
        "job_type": "integration",
        "attributes": "{\"integration_name\": \"salesforce\"}"
      }
    }
  }'
```

## Setting User Context

Tasks need to run as a specific user. Link the task to a user account:

### Via Relationship

```bash
# First, get the user reference_id
USER_REF=$(curl -s http://localhost:6336/api/user_account \
  -H "Authorization: Bearer $TOKEN" | \
  jq -r '.data[0].attributes.reference_id')

# Create task with user relationship
curl -X POST http://localhost:6336/api/task \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "task",
      "attributes": {
        "name": "admin_task",
        "action_name": "generate_report",
        "entity_name": "analytics",
        "schedule": "@weekly",
        "active": true
      },
      "relationships": {
        "as_user_id": {
          "data": {
            "type": "user_account",
            "id": "'"$USER_REF"'"
          }
        }
      }
    }
  }'
```

## Error Handling

Task execution errors:
- **Logged** - Errors are logged for debugging
- **Transaction Rollback** - Failed tasks rollback their changes
- **Continue Execution** - Other scheduled tasks continue running
- **Retry on Next Run** - Failed tasks retry on next scheduled execution

No automatic retry mechanism exists between scheduled runs.

## Task Lifecycle

### Startup

1. Daptin loads all tasks from database via `GetAllTasks()`
2. Active tasks are registered with the cron scheduler
3. Scheduler starts running in background

### Runtime

1. Cron triggers task at scheduled time
2. Task executes within transaction
3. Success/failure is logged

### Shutdown

1. `StopTasks()` halts the cron scheduler
2. Running tasks complete their current execution
3. No new tasks are started

## Defining Tasks in Schema

Tasks can be pre-defined in your YAML schema:

```yaml
Tasks:
  - Name: hourly_sync
    Schedule: "@every 1h"
    ActionName: sync_external_data
    EntityName: integration
    AsUserEmail: system@example.com
    Active: true
    Attributes:
      source: external_api

  - Name: daily_report
    Schedule: "0 8 * * *"
    ActionName: generate_report
    EntityName: report
    AsUserEmail: reports@example.com
    Active: true
    Attributes:
      type: daily
      format: pdf
```

## Best Practices

1. **Use appropriate intervals** - Don't schedule tasks more frequently than needed
2. **Set user context** - Always specify `as_user_id` for proper permissions
3. **Test actions first** - Verify the action works manually before scheduling
4. **Use meaningful names** - Clear, descriptive task names
5. **Monitor execution** - Check logs for task failures
6. **Keep attributes minimal** - Only pass necessary parameters
7. **Consider load** - Schedule heavy tasks during off-peak hours

## Troubleshooting

### Task Not Running

1. Check `active` is `true`
2. Verify `schedule` syntax is valid
3. Confirm user relationship exists
4. Check action exists on entity

### Task Fails

1. Check server logs for error details
2. Test action manually with same parameters
3. Verify user has required permissions
4. Check attributes JSON is valid

### View Task Configuration

```bash
curl "http://localhost:6336/api/task?include=as_user_id" \
  -H "Authorization: Bearer $TOKEN"
```

## Related

- [Actions Overview](Actions-Overview.md) - Actions that tasks can execute
- [Custom Actions](Custom-Actions.md) - Define custom actions for tasks
- [Integrations](Integrations.md) - External service integration actions
- [Cloud Storage](Cloud-Storage.md) - Cloud sync task configuration
