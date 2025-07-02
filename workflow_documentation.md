# Daptin Workflow & Automation Features Documentation

## Overview

Daptin provides a comprehensive workflow and automation system with actions, state machines, scheduled tasks, integrations, and data exchange capabilities.

## 1. Actions System

Actions are the core automation primitives in Daptin. They allow you to trigger business logic, integrate with external services, and automate workflows.

### Action Endpoints

- **List Guest Actions**: `GET /actions` - Returns signup/signin actions available without auth
- **Execute Action**: `POST /action/{entity}/{actionName}` - Execute an action on an entity
- **Get Action Info**: `GET /action/{entity}/{actionName}` - Get action details

### Built-in Actions (50+ actions)

#### User Management Actions
- `user_account/signin` - Authenticate user and get JWT token
- `user_account/signup` - Register new user
- `user_account/become_admin` - Become system administrator (one-time)
- `user_account/generate_jwt_token` - Generate new JWT token
- `user_account/otp_generate` - Generate OTP for 2FA
- `user_account/otp_login_verify` - Verify OTP login
- `user_account/generate_password_reset_flow` - Start password reset
- `user_account/generate_password_reset_verify_flow` - Complete password reset
- `user_account/switch_session_user` - Switch user context

#### Data Management Actions
- `{entity}/export_data` - Export entity data as JSON
- `{entity}/export_csv_data` - Export entity data as CSV
- `{entity}/import_data` - Import data into entity
- `{entity}/csv_to_entity` - Import CSV data
- `{entity}/xls_to_entity` - Import Excel data
- `{entity}/generate_random_data` - Generate test data

#### Communication Actions
- `mail/send` - Send email via SMTP
- `mail/send_ses` - Send email via AWS SES
- `mail_servers_sync` - Sync mail server configurations

#### Cloud Storage Actions
- `cloudstore_file_upload` - Upload file to cloud storage
- `cloudstore_file_delete` - Delete file from cloud storage
- `cloudstore_folder_create` - Create folder in cloud storage
- `cloudstore_path_move` - Move/rename paths
- `cloudstore_site_create` - Create new site
- `site_sync_storage` - Sync site with storage
- `column_sync_storage` - Sync column data with storage
- `import_cloudstore_files` - Import files from cloud storage

#### System Actions
- `world/restart_system` - Restart Daptin server
- `world/enable_graphql` - Enable GraphQL endpoint
- `world/download_cms_config` - Download CMS configuration
- `world/delete_table` - Delete entity table
- `world/delete_column` - Delete entity column
- `world/rename_column` - Rename entity column

#### Integration Actions
- `integration_execute` - Execute integration
- `integration_install` - Install integration
- `oauth_login_begin` - Start OAuth flow
- `oauth_login_response` - Handle OAuth callback
- `oauth_profile_exchange` - Exchange OAuth profile
- `generate_oauth2_token` - Generate OAuth2 token

#### Utility Actions
- `network_request` - Make HTTP requests
- `execute_process` - Run system processes
- `render_template` - Render templates
- `make_response` - Create custom responses
- `transaction` - Execute database transactions
- `generate_random_data` - Generate random values
- `random_value_generate` - Generate specific random values
- `generate_self_tls_certificate` - Generate self-signed TLS cert
- `generate_acme_tls_certificate` - Generate Let's Encrypt cert

### Action Request Format

```json
{
  "attributes": {
    "field1": "value1",
    "field2": "value2"
  }
}
```

### Action Response Format

Actions return an array of response directives:

```json
[
  {
    "ResponseType": "client.notify",
    "Attributes": {
      "message": "Action completed successfully",
      "title": "Success",
      "type": "success"
    }
  },
  {
    "ResponseType": "client.redirect",
    "Attributes": {
      "location": "/dashboard",
      "window": "self",
      "delay": 2000
    }
  },
  {
    "ResponseType": "client.file.download",
    "Attributes": {
      "content": "base64-encoded-content",
      "contentType": "application/json",
      "name": "export.json"
    }
  }
]
```

### Response Types

- `client.notify` - Show notification to user
- `client.redirect` - Redirect browser
- `client.file.download` - Download file
- `client.store.set` - Store value in client
- `client.cookie.set` - Set cookie
- `jwt.token` - Return JWT token
- `Restart` - Restart system

### Creating Custom Actions

Custom actions can be defined in the world schema with:
- Input fields specification
- Output/response actions
- Validations
- Conformations (data transformations)
- Permissions

Example action definition:
```json
{
  "Name": "send_notification",
  "Label": "Send Notification",
  "OnType": "notification",
  "InstanceOptional": false,
  "InFields": [
    {
      "Name": "recipient",
      "ColumnType": "email",
      "IsNullable": false
    },
    {
      "Name": "message",
      "ColumnType": "content",
      "IsNullable": false
    }
  ],
  "OutFields": [
    {
      "Type": "notification",
      "Method": "POST",
      "Attributes": {
        "recipient": "~recipient",
        "message": "~message",
        "sent_at": "~now"
      }
    },
    {
      "Type": "client.notify",
      "Method": "ACTIONRESPONSE",
      "Attributes": {
        "message": "Notification sent successfully",
        "type": "success"
      }
    }
  ],
  "Validations": [
    {
      "ColumnName": "recipient",
      "Tags": "email,required"
    }
  ]
}
```

## 2. State Machines

Daptin includes a finite state machine (FSM) system for modeling workflows and business processes.

### State Machine Tables

- `smd` - State Machine Definitions
- `smd_state` - Individual states within machines
- `{entity}_state` - State tracking for entities
- `{entity}_state_audit` - State transition audit logs

### State Machine Features

- Define states and transitions
- Event-driven state changes
- Permission-based transitions
- Automatic audit logging
- State-based validations
- Before/after transition hooks

### FSM Event Endpoint

```
POST /api/event/{entity}/{objectStateId}/{eventName}
```

Triggers state transitions on objects.

## 3. Task Scheduler

The `task` table enables scheduled job execution.

### Task Table Structure

- `name` - Task identifier
- `schedule` - Cron expression
- `action_name` - Action to execute
- `entity_name` - Target entity
- `attributes` - JSON attributes for action
- `active` - Enable/disable flag
- `last_run` - Last execution timestamp
- `next_run` - Next scheduled execution

### Cron Expression Support

Standard cron format:
```
# ┌───────────── minute (0 - 59)
# │ ┌───────────── hour (0 - 23)
# │ │ ┌───────────── day of the month (1 - 31)
# │ │ │ ┌───────────── month (1 - 12)
# │ │ │ │ ┌───────────── day of the week (0 - 6)
# │ │ │ │ │
# * * * * *
```

Examples:
- `0 0 * * *` - Daily at midnight
- `*/15 * * * *` - Every 15 minutes
- `0 9 * * 1` - Every Monday at 9 AM

## 4. Integration System

### OAuth Integration Tables

- `oauth_connect` - OAuth provider configurations
- `oauth_token` - Stored OAuth tokens
- `integration` - Third-party integrations

### OAuth Providers

Configure OAuth providers for:
- Google
- GitHub
- Facebook
- Twitter
- Custom OAuth2 providers

### Integration Features

- OAuth 2.0 flow handling
- Token refresh automation
- Profile data mapping
- Webhook receivers
- API gateway functionality

## 5. Data Exchange System

The `data_exchange` table enables data synchronization and ETL operations.

### Exchange Types

1. **REST API Exchange**
   - Import/export via REST APIs
   - Scheduled sync
   - Field mapping
   - Data transformation

2. **File-based Exchange**
   - CSV/JSON/XML import/export
   - Cloud storage integration
   - Scheduled transfers

3. **Database Exchange**
   - Direct database connections
   - Cross-database sync
   - Schema mapping

### Exchange Configuration

```json
{
  "source": {
    "type": "rest",
    "endpoint": "https://api.example.com/data",
    "auth": {
      "type": "bearer",
      "token": "{{oauth_token}}"
    }
  },
  "destination": {
    "type": "entity",
    "name": "products"
  },
  "mapping": {
    "id": "external_id",
    "title": "name",
    "price": "cost"
  },
  "schedule": "0 */6 * * *"
}
```

## 6. Workflow Patterns

### Common Workflow Examples

#### 1. User Onboarding Flow
```javascript
// 1. User signs up
POST /action/user_account/signup

// 2. Generate OTP if mobile provided
POST /action/user_account/otp_generate

// 3. Send welcome email
POST /action/mail/send

// 4. Create initial user data
POST /api/user_profile
```

#### 2. Data Import Workflow
```javascript
// 1. Upload CSV file
POST /action/file/cloudstore_file_upload

// 2. Import CSV data
POST /action/products/csv_to_entity

// 3. Validate imported data
POST /action/products/validate_data

// 4. Send notification
POST /action/mail/send
```

#### 3. Scheduled Report Generation
```javascript
// Task configuration
{
  "name": "daily_report",
  "schedule": "0 8 * * *",
  "action_name": "generate_report",
  "entity_name": "report",
  "attributes": {
    "format": "pdf",
    "recipients": ["admin@example.com"]
  }
}
```

#### 4. OAuth Integration Flow
```javascript
// 1. Start OAuth flow
POST /action/oauth/oauth_login_begin
{
  "attributes": {
    "provider": "google"
  }
}

// 2. Handle callback (automatic)
// 3. Exchange profile data
POST /action/oauth/oauth_profile_exchange

// 4. Use OAuth token for API calls
POST /action/integration/integration_execute
{
  "attributes": {
    "integration_name": "google_calendar"
  }
}
```

## 7. Best Practices

### Action Design
- Keep actions focused and single-purpose
- Use appropriate response types
- Implement proper error handling
- Add validation for inputs
- Log important operations

### State Machine Design
- Define clear state transitions
- Use meaningful state names
- Implement transition guards
- Add audit logging
- Handle edge cases

### Task Scheduling
- Use appropriate cron expressions
- Monitor task execution
- Handle failures gracefully
- Avoid overlapping executions
- Log task outcomes

### Integration Security
- Store credentials securely
- Use OAuth when possible
- Implement rate limiting
- Validate webhook signatures
- Monitor API usage

## 8. Advanced Features

### Action Chaining
Actions can trigger other actions through their OutFields:

```json
{
  "OutFields": [
    {
      "Type": "mail.send",
      "Method": "EXECUTE",
      "Condition": "email != null",
      "Attributes": {
        "to": ["~email"],
        "subject": "Action completed"
      }
    },
    {
      "Type": "task",
      "Method": "POST",
      "Attributes": {
        "name": "followup_task",
        "schedule": "0 0 * * *",
        "action_name": "check_status"
      }
    }
  ]
}
```

### Conditional Execution
Use conditions in action definitions:

```json
{
  "Condition": "status == 'active' && credit > 0",
  "ContinueOnError": true
}
```

### Transaction Support
Group multiple operations in transactions:

```javascript
POST /action/world/transaction
{
  "attributes": {
    "operations": [
      {
        "type": "create",
        "entity": "order",
        "data": {...}
      },
      {
        "type": "update",
        "entity": "inventory",
        "id": "123",
        "data": {...}
      }
    ]
  }
}
```

## Summary

Daptin's workflow system provides:
- 50+ built-in actions for common tasks
- Custom action creation
- State machine workflows
- Scheduled task execution
- OAuth integrations
- Data exchange/ETL
- Transaction support
- Comprehensive audit logging

This makes Daptin suitable for building complex business applications with sophisticated automation requirements.