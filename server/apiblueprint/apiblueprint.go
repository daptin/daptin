package apiblueprint

import (
	"bytes"
	"encoding/json"
	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/table_info"
	"github.com/iancoleman/strcase"

	"fmt"
	"strings"
	//"github.com/daptin/daptin/server/fakerservice"
	"github.com/advance512/yaml"
	log "github.com/sirupsen/logrus"
)

func InfoError(err error, args ...interface{}) bool {
	if err != nil {
		if len(args) > 0 {
			fm := args[0].(string) + ": %v"
			args = args[1:]
			args = append(args, err)
			log.Printf(fm, args...)
			return true
		} else {
			log.Printf("%v", err)
			return true
		}
	}
	return false
}

type BlueprintWriter struct {
	buffer bytes.Buffer
}

func NewBluePrintWriter() BlueprintWriter {
	var x = BlueprintWriter{}

	x.buffer = bytes.Buffer{}
	return x
}

func (x *BlueprintWriter) WriteString(s string) {
	x.buffer.WriteString(s + "\n")
}

func (x *BlueprintWriter) WriteStringf(s ...interface{}) {
	x.buffer.WriteString(fmt.Sprintf(s[0].(string)+"\n", s[1:]...))
}

func (x *BlueprintWriter) Markdown() string {
	return x.buffer.String()
}

var skipColumns = map[string]bool{
	"id":         true,
	"permission": true,
}

func CreateColumnLine(colInfo api2go.ColumnInfo) map[string]interface{} {
	columnType := colInfo.ColumnType
	typ := resource.ColumnManager.GetBlueprintType(columnType)

	if typ == "" {
		typ = "string"
	}

	m := map[string]interface{}{
		"type": typ,
	}
	
	// Add description if available
	if colInfo.ColumnDescription != "" {
		m["description"] = colInfo.ColumnDescription
	}
	
	// Add default value if specified
	if colInfo.DefaultValue != "" && colInfo.DefaultValue != "null" {
		m["default"] = colInfo.DefaultValue
	}
	
	// Add format based on column type
	switch columnType {
	case "email":
		m["format"] = "email"
		m["example"] = "user@example.com"
	case "date":
		m["format"] = "date"
		m["example"] = "2024-01-15"
	case "datetime":
		m["format"] = "date-time"
		m["example"] = "2024-01-15T09:30:00Z"
	case "password":
		m["format"] = "password"
		m["writeOnly"] = true
	case "url":
		m["format"] = "uri"
		m["example"] = "https://example.com"
	case "uuid":
		m["format"] = "uuid"
		m["example"] = "550e8400-e29b-41d4-a716-446655440000"
	}
	
	// Add enum values if available
	if len(colInfo.Options) > 0 {
		enumValues := make([]string, 0)
		for _, option := range colInfo.Options {
			if strValue, ok := option.Value.(string); ok {
				enumValues = append(enumValues, strValue)
			} else if option.Value != nil {
				enumValues = append(enumValues, fmt.Sprintf("%v", option.Value))
			}
		}
		if len(enumValues) > 0 {
			m["enum"] = enumValues
		}
	}
	
	// Add nullable property for clarity
	if colInfo.IsNullable {
		m["nullable"] = true
	}
	
	return m
}

func BuildApiBlueprint(config *resource.CmsConfig, cruds map[string]*resource.DbResource) string {

	tableMap := map[string]table_info.TableInfo{}
	for _, table := range config.Tables {
		tableMap[table.TableName] = table
	}

	// Use yaml.MapSlice to preserve key order
	apiDefinition := yaml.MapSlice{
		{Key: "openapi", Value: "3.0.0"},
	}
	
	apiDefinition = append(apiDefinition, yaml.MapItem{
		Key: "info",
		Value: map[string]interface{}{
		"version": "1.0.0",
		"title":   "Daptin API endpoint",
		"license": map[string]interface{}{
			"name": "MIT",
			"url": "https://opensource.org/licenses/MIT",
		},
		"contact": map[string]interface{}{
			"name":  "Daptin Support",
			"url":   "https://dapt.in",
			"email": "artpar@gmail.com",
		},
		"description": `Daptin is a self-discovering headless backend that provides complete CRUD operations, authentication, authorization, and custom actions. This API follows JSON:API specification for resource representation.

## 🚀 Quick Start for Beginners

### Step 1: Discover Available Resources
~~~bash
# Get all available entities/tables
curl http://localhost:6336/api/world

# Get all available actions
curl http://localhost:6336/api/action
~~~

### Step 2: Authentication Setup
~~~bash
# 1. Create your first user account (public endpoint)
curl -X POST http://localhost:6336/action/user_account/signup \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"admin@test.com","password":"testpass123","name":"Admin User","passwordConfirm":"testpass123"}}'

# 2. Sign in to get JWT token
curl -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"admin@test.com","password":"testpass123"}}'

# 3. CRITICAL: Become administrator (one-time action)
curl -X POST http://localhost:6336/action/world/become_an_administrator \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{}'
~~~

### Step 3: Basic Resource Operations
~~~bash
# List all user accounts (using JWT token)
curl -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  http://localhost:6336/api/user_account

# Create a new entity programmatically
curl -X POST http://localhost:6336/api/world \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{"data":{"type":"world","attributes":{"table_name":"books","world_schema_json":"{\"tableName\":\"books\",\"columns\":[{\"name\":\"id\",\"columnType\":\"id\",\"dataType\":\"INTEGER\",\"isPrimaryKey\":true},{\"name\":\"title\",\"columnType\":\"label\",\"dataType\":\"varchar(100)\"}]}"}}}'

# Restart to activate new entity (transparent hot-reload)
curl -X POST http://localhost:6336/action/world/restart_daptin \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{}'
~~~

## 🎓 Advanced Self-Management

### System Discovery
- **GET /api/world**: List all entities (tables) - shows structure, permissions, relationships
- **GET /api/action**: List all available actions - shows parameters, descriptions, permissions  
- **GET /action/world/download_system_schema**: Export complete system configuration (JSON)
- **GET /openapi.yaml**: This OpenAPI spec (29,000+ lines of documentation)

### Dynamic Schema Management
- **Create entities**: POST /api/world with table_name and world_schema_json
- **Add columns**: Use world actions like add_column, remove_column, rename_column
- **Hot-reload**: /action/world/restart_daptin applies schema changes without process restart

### Multi-Admin Support
- **Additional admins**: Add users to "administrators" usergroup
- **Permission inheritance**: Users in admin group get admin privileges
- **Granular control**: Actions have individual execute permissions

### Data Management
- **Export data**: /action/world/export_data (JSON, CSV, XML, PDF formats)
- **Import data**: /action/world/import_data (bulk operations)
- **Cloud integration**: Built-in S3, GCS, Azure storage support

## ⚠️ Security Model

### Admin Bootstrapping (Critical!)
- **Before first admin**: ALL users have full system access (security risk!)
- **become_an_administrator**: One-time action that secures the system
- **After admin setup**: Only admin users can access protected resources
- **Multi-admin**: Add users to "administrators" usergroup for additional admins

### Authentication
- **JWT tokens**: Include "Authorization: Bearer TOKEN" header
- **Token expiry**: Tokens expire, re-authenticate via /action/user_account/signin
- **Action permissions**: Each action has its own permission model (Guest, User, Admin)

## 📊 Monitoring & Troubleshooting

### Common Error Solutions
- **"min and 0 more errors"**: Password must be ≥8 characters
- **"world_schema_json required"**: Entity creation needs JSON string schema
- **403 Forbidden**: Check JWT token validity and admin status
- **HTML response instead of JSON**: Entity not activated, restart required

### Rate Limiting
Response headers indicate limits:
- **X-RateLimit-Limit**: Maximum requests allowed  
- **X-RateLimit-Remaining**: Requests remaining
- **X-RateLimit-Reset**: Unix timestamp when limit resets

### Self-Monitoring
- **GET /api/user_account**: Check your user details and permissions
- **Logs**: Server logs show authentication and permission details
- **Health checks**: All endpoints return structured error messages

## 🔄 Real-time & Communication Features (Operationally Verified)

### ⚠️ WebSocket Integration (Authentication Issues)
Daptin has WebSocket infrastructure at '/live' but currently has authentication challenges.

**Current Status:**
- WebSocket server is running and accessible
- Authentication middleware causes 403 Forbidden during WebSocket handshake
- Requires further investigation for proper token handling in WebSocket connections

**Endpoint Available:** `/live` (returns "not websocket protocol" via HTTP, 403 via WebSocket)

**WebSocket Message Format:**
~~~json
{
  "method": "subscribe|unsubscribe|list-topic|create-topic|destroy-topic|new-message",
  "attributes": {
    "topicName": "user_account,document,calendar",
    "filters": {
      "EventType": "create|update|delete",
      "column_name": "filter_value"
    }
  }
}
~~~

**Real-time Event Example:**
~~~json
{
  "MessageSource": "database",
  "EventType": "create", 
  "ObjectType": "user_account",
  "EventData": {
    "__type": "user_account",
    "email": "user@example.com",
    "reference_id": "uuid-here",
    "created_at": "2024-01-15T09:30:00Z"
  }
}
~~~

### ✅ YJS Collaborative Editing (Infrastructure Verified)
Real-time document collaboration infrastructure powered by YJS protocol.

**YJS Configuration (VERIFIED WORKING):**
~~~bash
# YJS is enabled and configured (TESTED ✅)
curl -H "Authorization: Bearer TOKEN" http://localhost:6336/_config | grep yjs
# Returns: "yjs.enabled": "true", "yjs.storage.path": "./storage/yjs-documents"

# YJS endpoint exists and responds (TESTED ✅)
curl -I http://localhost:6336/yjs/documentName
# Returns: HTTP/1.1 200 OK
~~~

**File Column Setup (VERIFIED WORKING):**
~~~bash
# Create calendar with file.ical content for YJS collaboration (TESTED ✅)
curl -X POST http://localhost:6336/api/calendar \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"data": {"type": "calendar", "attributes": {"rpath": "test-calendar", "content": [{"name": "test.ical", "type": "text/calendar", "contents": "BEGIN:VCALENDAR\\nVERSION:2.0\\nEND:VCALENDAR"}]}}}'
# Returns: Success with reference_id for collaboration
~~~

**YJS Endpoints (Infrastructure Present):**
- **Direct YJS**: `/yjs/:documentName` (HTTP 200 ✅)
- **Live Collaboration**: `/live/calendar/:referenceId/content/yjs` (Requires WebSocket auth fix)
- **File Columns**: Any file-type column gets automatic YJS endpoints

**Status**: YJS infrastructure is ready, collaboration endpoints exist, but require WebSocket authentication resolution for full functionality.

### Live Data Subscription
Subscribe to real-time changes across all entities:
~~~bash
# WebSocket message to subscribe to user account changes
{
  "method": "subscribe",
  "attributes": {
    "topicName": "user_account",
    "filters": {"EventType": "update"}
  }
}
~~~

## ✅ Communication Systems (Configuration Verified)

### ✅ SMTP Email Server (Configuration Tested)
Built-in email server capabilities with working configuration management.

**SMTP Configuration (VERIFIED WORKING):**
~~~bash
# Check SMTP settings (TESTED ✅)
curl -H "Authorization: Bearer TOKEN" http://localhost:6336/_config | grep smtp
# Default: SMTP disabled

# Enable SMTP server (TESTED ✅)
curl -X PUT http://localhost:6336/_config/backend/smtp.enable \
  -H "Authorization: Bearer TOKEN" \
  -d '"true"'
# Result: "smtp.enable": "\"true\"" (Successfully enabled)
~~~

**Mail Server Management:**
~~~bash
# List mail servers
curl -H "Authorization: Bearer TOKEN" http://localhost:6336/api/mail_server

# Mail accounts and mailboxes
curl -H "Authorization: Bearer TOKEN" http://localhost:6336/api/mail_account
curl -H "Authorization: Bearer TOKEN" http://localhost:6336/api/mail_box
~~~

### ✅ CalDAV Calendar Sync (Configuration Tested)
Calendar synchronization via CalDAV protocol with working configuration.

**CalDAV Configuration (VERIFIED WORKING):**
~~~bash
# Check CalDAV status (TESTED ✅)
curl -H "Authorization: Bearer TOKEN" http://localhost:6336/_config | grep caldav
# Default: "caldav.enable": "false"

# Enable CalDAV server (TESTED ✅)
curl -X PUT http://localhost:6336/_config/backend/caldav.enable \
  -H "Authorization: Bearer TOKEN" \
  -d '"true"'
# Result: "caldav.enable": "\"true\"" (Successfully enabled)
~~~

### ✅ FTP File Transfer (Configuration Tested)
Optional FTP server for file transfer operations with working configuration.

**FTP Configuration (VERIFIED WORKING):**
~~~bash
# Check FTP status (TESTED ✅)
curl -H "Authorization: Bearer TOKEN" http://localhost:6336/_config | grep ftp
# Default: "ftp.enable": "false"

# Enable FTP server (TESTED ✅)
curl -X PUT http://localhost:6336/_config/backend/ftp.enable \
  -H "Authorization: Bearer TOKEN" \
  -d '"true"'
# Result: "ftp.enable": "\"true\"" (Successfully enabled)
~~~

## ⚠️ Feed System (Needs Further Testing)

### Feed Infrastructure (Partially Verified)
Feed generation infrastructure exists but requires proper configuration.

**Feed Status (PARTIALLY TESTED):**
~~~bash
# Feed API exists (TESTED ✅)
curl -H "Authorization: Bearer TOKEN" http://localhost:6336/api/feed
# Returns: Empty feed list (successful response)

# Feed endpoint exists (TESTED ✅)
curl http://localhost:6336/feed/test-feed
# Returns: {"error":"Invalid feed request"} (endpoint exists, needs proper feed setup)
~~~

**Current Status:**
- Feed API endpoints are accessible
- Feed entity exists but requires specific schema fields
- Feed generation needs proper configuration before testing
- Public feed access confirmed but requires valid feed setup

## 📊 Operational Testing Summary

### ✅ Verified Working Features
**Configuration Management:**
- SMTP, CalDAV, FTP server enable/disable via `/_config` API
- YJS collaboration infrastructure enabled and responding
- JWT authentication for API endpoints
- Entity creation with file-type columns for collaboration

**Real-time Infrastructure:**
- YJS endpoints accessible (`/yjs/:documentName` returns HTTP 200)
- Calendar entities with file.ical content created successfully
- WebSocket server running (responds to HTTP requests)

### ⚠️ Features Requiring Further Investigation
**WebSocket Authentication:**
- WebSocket handshake returns 403 Forbidden
- Authentication middleware needs WebSocket-specific handling
- Real-time subscriptions blocked by auth issues

**Feed System:**
- Feed endpoints exist but need proper schema understanding
- Feed generation requires valid feed configuration
- Public feed access confirmed but needs setup

**SMTP/CalDAV/FTP Servers:**
- Configuration changes work
- Actual server functionality not tested (requires service restart/verification)

### 🔧 Next Steps for Full Verification
1. Resolve WebSocket authentication for real-time features
2. Create proper feed entities with correct schema
3. Test SMTP/CalDAV/FTP server functionality after enabling
4. Verify YJS collaboration end-to-end once WebSocket auth works

## 🔄 Workflow & Automation Features (Fully Documented)

### ✅ Actions System  
Daptin provides 50+ built-in actions for automation, plus custom action creation.

**Action Endpoints:**
~~~bash
# List available guest actions (signup/signin)
curl http://localhost:6336/actions

# Execute an action
curl -X POST http://localhost:6336/action/{entity}/{actionName} \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"attributes": {...}}'
~~~

**Built-in Action Categories:**
- **User Management**: signin, signup, become_admin, generate_jwt_token, otp_generate, password reset
- **Data Operations**: export_data, export_csv_data, import_data, csv_to_entity, xls_to_entity
- **Communication**: mail.send (SMTP), mail.send_ses (AWS SES), mail_servers_sync
- **Cloud Storage**: cloudstore_file_upload/delete, folder_create, path_move, site_create
- **System**: restart_system, enable_graphql, download_cms_config, delete_table/column
- **Integration**: oauth_login_begin/response, integration_execute/install, generate_oauth2_token
- **Utilities**: network_request, execute_process, render_template, generate_random_data

**Action Response Types:**
- `client.notify` - Display notification
- `client.redirect` - Browser redirect
- `client.file.download` - File download
- `client.token.set` - Store JWT token
- `Restart` - System restart

**Example - Export Data:**
~~~bash
curl -X POST http://localhost:6336/action/user_account/export_data \
  -H "Authorization: Bearer TOKEN" \
  -d '{"attributes": {"format": "json"}}'
# Returns base64-encoded JSON data for download
~~~

### ✅ State Machines (FSM)
Event-driven workflow system with audit trails.

**State Machine Tables:**
- `smd` - State machine definitions
- `smd_state` - Individual states
- `{entity}_state` - Entity state tracking
- `{entity}_state_audit` - Transition audit logs

**FSM Event Endpoint:**
~~~bash
POST /api/event/{entity}/{objectStateId}/{eventName}
~~~

### ✅ Task Scheduler
Background job execution with cron support.

**Task Table Fields:**
- `name` - Task identifier
- `schedule` - Cron expression (e.g., "0 0 * * *" for daily)
- `action_name` - Action to execute
- `entity_name` - Target entity
- `attributes` - JSON parameters
- `active` - Enable/disable flag
- `last_run` / `next_run` - Execution tracking

### ✅ Integration System
OAuth providers and external API connections.

**Integration Tables:**
- `oauth_connect` - OAuth provider configs
- `oauth_token` - Stored tokens
- `integration` - Third-party integrations
- `data_exchange` - ETL configurations

**OAuth Flow:**
~~~bash
# Start OAuth
POST /action/oauth/oauth_login_begin {"attributes": {"provider": "google"}}

# Exchange profile
POST /action/oauth/oauth_profile_exchange

# Use integration
POST /action/integration/integration_execute
~~~

### ✅ Data Exchange System
ETL and synchronization capabilities.

**Exchange Types:**
- REST API sync with auth
- File-based import/export
- Database-to-database sync
- Scheduled transfers

**Exchange Configuration:**
~~~json
{
  "source": {"type": "rest", "endpoint": "https://api.example.com"},
  "destination": {"type": "entity", "name": "products"},
  "mapping": {"external_id": "id", "name": "title"},
  "schedule": "0 */6 * * *"
}
~~~`,
		"x-logo": map[string]interface{}{
			"url": "https://daptin.github.io/daptin/images/logo.png",
			"altText": "Daptin Logo",
		},
	},
	})

	apiDefinition = append(apiDefinition, yaml.MapItem{
		Key: "servers",
		Value: []map[string]interface{}{
		{
			"url":         fmt.Sprintf("http://%v", config.Hostname),
			"description": "Server " + config.Hostname,
		},
	},
	})
	typeMap := make(map[string]map[string]interface{})
	typeMap["RelatedStructure"] = map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{
				"type":        "string",
				"description": "Id of the object",
			},
			"type": map[string]interface{}{
				"type":        "string",
				"description": "Type of the included object",
			},
		},
	}

	paginationObject := make(map[string]interface{})
	paginationObject["type"] = "object"
	paginationObject["properties"] = map[string]interface{}{
		"page[number]": map[string]interface{}{
			"type":        "number",
			"description": "Page number",
		},
		"page[size]": map[string]interface{}{
			"type":        "number",
			"description": "Number of item to return",
		},
		"page[after]": map[string]interface{}{
			"type":        "string",
			"description": "Reference id of the object after which to look for",
		},
	}
	typeMap["Pagination"] = paginationObject

	actionResponse := map[string]interface{}{
		"type": "object",
		"description": `Response from an action execution. Actions return an array of responses, each directing the client to perform specific operations.

## Response Types

### client.notify
Displays a notification message to the user.
- **type**: "success" | "error" | "info" | "warning"
- **title**: Notification title
- **message**: Detailed message

### client.redirect
Redirects the browser to a new location.
- **location**: Target URL or path
- **delay**: Milliseconds before redirect (optional)
- **window**: "self" | "new" (optional)

### client.file.download
Triggers a file download.
- **name**: Filename for download
- **content**: Base64 encoded file content
- **contentType**: MIME type
- **message**: Optional status message

### client.token.set / client.store.set
Stores data in client storage.
- **key**: Storage key name
- **value**: Value to store (usually JWT token)
- **expiry**: Seconds until expiration (optional)

### client.cookie.set
Sets an HTTP cookie.
- **key**: Cookie name
- **value**: Cookie value with attributes

### client.script.run
Executes JavaScript in the client.
- **script**: JavaScript code to execute`,
		"required": []string{"ResponseType", "Attributes"},
		"properties": map[string]interface{}{
			"ResponseType": map[string]interface{}{
				"type": "string",
				"description": "The type of response directing client behavior",
				"enum": []string{
					"client.redirect",
					"client.notify", 
					"client.file.download",
					"client.token.set",
					"client.cookie.set",
					"client.store.set",
					"client.script.run",
				},
				"example": "client.notify",
			},
			"Attributes": map[string]interface{}{
				"type": "object",
				"description": "Response-specific attributes. Structure depends on ResponseType (see schema description for details).",
				"additionalProperties": true,
				"examples": []map[string]interface{}{
					{
						"type": "success",
						"title": "Operation Successful",
						"message": "The action completed successfully",
					},
					{
						"location": "/dashboard",
						"delay": 2000,
					},
					{
						"name": "export.csv",
						"content": "base64_encoded_content",
						"contentType": "text/csv",
					},
				},
			},
		},
		"example": map[string]interface{}{
			"ResponseType": "client.notify",
			"Attributes": map[string]interface{}{
				"type": "success",
				"title": "Success",
				"message": "Action executed successfully",
			},
		},
	}

	paginationStatus := make(map[string]interface{})
	paginationStatus["type"] = "object"
	paginationStatus["properties"] = map[string]interface{}{
		"current_page": map[string]interface{}{
			"type":        "number",
			"description": "The current page, for pagination",
		},
		"from": map[string]interface{}{
			"type":        "number",
			"description": "From page",
		},
		"last_page": map[string]interface{}{
			"type":        "number",
			"description": "The last page number in current query set",
		},
		"per_page": map[string]interface{}{
			"type":        "number",
			"description": "This is the number of results in one page",
		},
		"to": map[string]interface{}{
			"type":        "number",
			"description": "Index of the last record fetched in this result",
		},
		"total": map[string]interface{}{
			"type":        "number",
			"description": "Total number of records",
		},
	}
	typeMap["PaginationStatus"] = paginationStatus
	typeMap["ActionResponse"] = actionResponse
	
	// Add ActionDefinition schema for action metadata
	actionDefinition := map[string]interface{}{
		"type": "object",
		"description": "Defines an action's structure, inputs, outputs, and behavior",
		"properties": map[string]interface{}{
			"Name": map[string]interface{}{
				"type": "string",
				"description": "Internal name of the action",
			},
			"Label": map[string]interface{}{
				"type": "string",
				"description": "User-friendly display name",
			},
			"OnType": map[string]interface{}{
				"type": "string",
				"description": "Entity type this action operates on",
			},
			"InstanceOptional": map[string]interface{}{
				"type": "boolean",
				"description": "Whether an entity instance is required",
			},
			"InFields": map[string]interface{}{
				"type": "array",
				"description": "Input fields required by the action",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"Name": map[string]interface{}{"type": "string"},
						"ColumnName": map[string]interface{}{"type": "string"},
						"ColumnType": map[string]interface{}{"type": "string"},
						"IsNullable": map[string]interface{}{"type": "boolean"},
					},
				},
			},
			"OutFields": map[string]interface{}{
				"type": "array",
				"description": "Output actions and responses",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"Type": map[string]interface{}{"type": "string"},
						"Method": map[string]interface{}{"type": "string"},
						"Attributes": map[string]interface{}{"type": "object"},
					},
				},
			},
			"Validations": map[string]interface{}{
				"type": "array",
				"description": "Input validation rules",
			},
		},
	}
	typeMap["ActionDefinition"] = actionDefinition
	
	// Add comprehensive error response schemas
	errorResponse := map[string]interface{}{
		"type": "object",
		"required": []string{"errors"},
		"properties": map[string]interface{}{
			"errors": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"required": []string{"status", "title"},
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type": "string",
							"description": "A unique identifier for this particular occurrence of the problem",
						},
						"status": map[string]interface{}{
							"type": "string",
							"description": "The HTTP status code applicable to this problem",
							"example": "400",
						},
						"code": map[string]interface{}{
							"type": "string",
							"description": "An application-specific error code",
							"example": "VALIDATION_ERROR",
						},
						"title": map[string]interface{}{
							"type": "string",
							"description": "A short, human-readable summary of the problem",
							"example": "Validation failed",
						},
						"detail": map[string]interface{}{
							"type": "string",
							"description": "A human-readable explanation specific to this occurrence",
							"example": "The email field must be a valid email address",
						},
						"source": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"pointer": map[string]interface{}{
									"type": "string",
									"description": "JSON Pointer to the associated entity in the request",
									"example": "/data/attributes/email",
								},
								"parameter": map[string]interface{}{
									"type": "string",
									"description": "String indicating which query parameter caused the error",
									"example": "filter",
								},
							},
						},
					},
				},
			},
		},
	}
	typeMap["ErrorResponse"] = errorResponse
	
	// Add rate limit error response
	rateLimitResponse := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"errors": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"status": map[string]interface{}{
							"type": "string",
							"example": "429",
						},
						"title": map[string]interface{}{
							"type": "string",
							"example": "Too Many Requests",
						},
						"detail": map[string]interface{}{
							"type": "string",
							"example": "Rate limit exceeded. Please retry after some time.",
						},
					},
				},
			},
			"meta": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"rate_limit": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"limit": map[string]interface{}{
								"type": "integer",
								"description": "The maximum number of requests allowed",
							},
							"remaining": map[string]interface{}{
								"type": "integer",
								"description": "The number of requests remaining in the current window",
							},
							"reset": map[string]interface{}{
								"type": "integer",
								"description": "Unix timestamp when the rate limit window resets",
							},
						},
					},
				},
			},
		},
	}
	typeMap["RateLimitResponse"] = rateLimitResponse

	IncludedRelationship := make(map[string]interface{})
	IncludedRelationship["type"] = "object"
	IncludedRelationship["description"] = "Relationship object following JSON:API specification"
	IncludedRelationship["properties"] = map[string]interface{}{
		"data": map[string]interface{}{
			"oneOf": []map[string]interface{}{
				{
					"$ref": "#/components/schemas/RelatedStructure",
					"description": "Single related resource (has_one/belongs_to)",
				},
				{
					"type": "array",
					"items": map[string]interface{}{
						"$ref": "#/components/schemas/RelatedStructure",
					},
					"description": "Multiple related resources (has_many)",
				},
			},
		},
		"links": map[string]interface{}{
			"type":        "object",
			"description": "Links to fetch or manipulate the relationship",
			"properties": map[string]interface{}{
				"related": map[string]interface{}{
					"type":        "string",
					"format":      "uri",
					"description": "URL to fetch the related resource(s)",
					"example":     "/api/posts/123/author",
				},
				"self": map[string]interface{}{
					"type":        "string",
					"format":      "uri",
					"description": "URL to fetch the relationship itself",
					"example":     "/api/posts/123/relationships/author",
				},
			},
		},
		"meta": map[string]interface{}{
			"type":        "object",
			"description": "Additional metadata about the relationship",
			"additionalProperties": true,
		},
	}
	typeMap["IncludedRelationship"] = IncludedRelationship

	for _, tableInfo := range config.Tables {
		ramlType := make(map[string]interface{})
		// skip join tables
		if strings.Index(tableInfo.TableName, "_has_") > -1 {
			continue
		}

		properties := make(map[string]interface{})
		requiredCols := make([]string, 0)
		ramlType["type"] = "object"
		for _, colInfo := range tableInfo.Columns {
			if colInfo.IsForeignKey {
				continue
			}
			if skipColumns[colInfo.ColumnName] {
				continue
			}

			if !colInfo.IsNullable && !resource.IsStandardColumn(colInfo.ColumnName) {
				requiredCols = append(requiredCols, colInfo.ColumnName)
			}

			properties[colInfo.ColumnName] = CreateColumnLine(colInfo)
		}

		ramlType["properties"] = properties
		ramlType["required"] = requiredCols
		
		// Add table description if available
		if tableInfo.TableDescription != "" {
			ramlType["description"] = tableInfo.TableDescription
		}
		
		// Add example object
		exampleObj := make(map[string]interface{})
		for colName, colDef := range properties {
			if colMap, ok := colDef.(map[string]interface{}); ok {
				if example, exists := colMap["example"]; exists {
					exampleObj[colName] = example
				} else if colMap["type"] == "string" {
					exampleObj[colName] = "example " + colName
				} else if colMap["type"] == "number" {
					exampleObj[colName] = 42
				} else if colMap["type"] == "boolean" {
					exampleObj[colName] = true
				}
			}
		}
		if len(exampleObj) > 0 {
			ramlType["example"] = exampleObj
		}

		typeMap[strcase.ToCamel(tableInfo.TableName)] = ramlType

		//worldActions, err := cruds["action"].GetActionsByType(tableInfo.TableName)
		//if InfoError(err, "Failed to list world actions for raml") {
		//	continue
		//}

	}
	for _, tableInfo := range config.Tables {
		ramlType := make(map[string]interface{})
		// skip join tables
		if strings.Index(tableInfo.TableName, "_has_") > -1 {
			continue
		}

		properties := make(map[string]interface{})
		requiredCols := make([]string, 0)
		ramlType["type"] = "object"
		for _, colInfo := range tableInfo.Columns {
			if colInfo.IsForeignKey {
				continue
			}
			if skipColumns[colInfo.ColumnName] {
				continue
			}
			if resource.IsStandardColumn(colInfo.ColumnName) {
				continue
			}

			if !colInfo.IsNullable && colInfo.DefaultValue == "" {
				requiredCols = append(requiredCols, colInfo.ColumnName)
			}

			properties[colInfo.ColumnName] = CreateColumnLine(colInfo)
		}

		ramlType["properties"] = properties
		ramlType["required"] = requiredCols

		typeMap["New"+strcase.ToCamel(tableInfo.TableName)] = ramlType

	}

	for _, action := range config.Actions {
		ramlActionType := make(map[string]interface{})
		ramlActionType["type"] = "object"

		// Generate comprehensive description for the action schema
		schemaDescription := generateActionSchemaDescription(action)
		ramlActionType["description"] = schemaDescription

		actionProperties := make(map[string]interface{})
		requiredFields := []string{}
		
		for _, colInfo := range action.InFields {
			if colInfo.IsForeignKey {
				continue
			}
			if skipColumns[colInfo.ColumnName] {
				continue
			}

			// Create enhanced column definition with better descriptions
			colDef := CreateColumnLine(colInfo)
			
			// Add field-specific descriptions based on the action context
			if desc, ok := getFieldDescription(action.Name, colInfo.ColumnName); ok {
				colDef["description"] = desc
			}
			
			actionProperties[colInfo.ColumnName] = colDef
			
			// Track required fields
			if !colInfo.IsNullable {
				requiredFields = append(requiredFields, colInfo.ColumnName)
			}
		}
		
		if !action.InstanceOptional {
			actionProperties[action.OnType+"_id"] = map[string]interface{}{
				"type":        "string",
				"format":      "uuid",
				"description": fmt.Sprintf("Reference ID of the %s instance on which to execute this action. This must be a valid UUID of an existing %s record.", action.OnType, action.OnType),
				"example":     "550e8400-e29b-41d4-a716-446655440000",
			}
			requiredFields = append(requiredFields, action.OnType+"_id")
		}

		ramlActionType["properties"] = actionProperties
		
		if len(requiredFields) > 0 {
			ramlActionType["required"] = requiredFields
		}
		
		// Add example object for the action
		if example := generateActionExample(action); example != nil {
			ramlActionType["example"] = example
		}
		
		typeMap[fmt.Sprintf("%sOn%sRequestObject", strcase.ToCamel(action.Name), strcase.ToCamel(action.OnType))] = ramlActionType

	}

	resourcesMap := map[string]map[string]interface{}{}
	tableInfoMap := make(map[string]table_info.TableInfo)
	for _, tableInfo := range config.Tables {
		tableInfoMap[tableInfo.TableName] = tableInfo
	}

	for _, tableInfo := range config.Tables {

		// skip join tables
		if strings.Index(tableInfo.TableName, "_has_") > -1 {
			continue
		}

		resourceInstance := make(map[string]interface{})

		dataInResponse := CreateDataInResponse(tableInfo)

		// BEGIN: POST request
		postMethod := CreatePostMethod(tableInfo, dataInResponse)
		resourceInstance["post"] = &postMethod
		//  END: POST Request

		//  BEGIN: GET Request
		getAllMethod := CreateGetAllMethod(tableInfo, dataInResponse)
		resourceInstance["get"] = &getAllMethod
		//  END: GET Request

		//fakeObject := fakerservice.NewFakeInstance(tableInfo)

		nestedMap := make(map[string]map[string]interface{})

		byIdResource := make(map[string]interface{})

		//  BEGIN: GET ById Request
		getByIdMethod := CreateGetMethod(tableInfo, dataInResponse)
		byIdResource["get"] = getByIdMethod
		//  END: GET ById Request

		// BEGIN: PATCH request
		patchMethod := CreatePatchMethod(tableInfo)
		byIdResource["patch"] = &patchMethod
		//  END: PATCH Request

		// BEGIN: DELETE Request
		deleteByIdMethod := CreateDeleteMethod(tableInfo)
		byIdResource["delete"] = deleteByIdMethod
		// END: DELETE Request

		nestedMap["/api/"+tableInfo.TableName+"/{referenceId}"] = byIdResource

		for _, rel := range tableInfo.Relations {

			// BEGIN: Get Relations Method

			relationsById := make(map[string]interface{})

			if tableInfo.TableName == rel.Subject {
				relatedTable, exists := tableInfoMap[rel.Object]
				if !exists || relatedTable.TableName == "" {
					log.Printf("Warning: Related table '%s' not found for relation %v", rel.Object, rel)
					continue
				}
				getMethod := CreateGetAllMethod(relatedTable, CreateDataInResponse(relatedTable))
				getMethod["description"] = "Returns a list of all " + ProperCase(relatedTable.TableName) + " related to a " + tableInfo.TableName
				getMethod["operationId"] = "Get" + strcase.ToCamel(rel.ObjectName) + "Of" + strcase.ToCamel(rel.SubjectName)
				getMethod["summary"] = "Fetch related " + rel.ObjectName + " of " + tableInfo.TableName

				getMethod["parameters"] = []map[string]interface{}{
					{
						"name": "referenceId",
						"schema": map[string]interface{}{
							"type": "string",
						},
						"required":    true,
						"in":          "path",
						"description": "Reference Id of the " + tableInfo.TableName,
					},
				}

				deleteMethod := CreateDeleteRelationMethod(relatedTable)
				deleteMethod["description"] = fmt.Sprintf("Remove a related %v from the %v", tableInfo.TableName, rel.ObjectName)
				deleteMethod["tags"] = []string{rel.ObjectName, rel.Subject, rel.SubjectName, rel.Object, rel.Relation, "delete"}

				deleteMethod["summary"] = fmt.Sprintf("Delete related %s of %v", rel.ObjectName, tableInfo.TableName)
				deleteMethod["operationId"] = "Delete" + strcase.ToCamel(rel.ObjectName) + "Of" + strcase.ToCamel(tableInfo.TableName)

				relationsById["get"] = getMethod
				relationsById["delete"] = deleteMethod

				patchMethod := CreatePatchRelationMethod(relatedTable)
				patchMethod["description"] = fmt.Sprintf("Add a related %v from the %v", tableInfo.TableName, rel.ObjectName)
				patchMethod["tags"] = []string{rel.ObjectName, rel.Subject, rel.SubjectName, rel.Object, rel.Relation, "patch"}

				patchMethod["summary"] = fmt.Sprintf("Add related %s of %v", rel.ObjectName, tableInfo.TableName)
				patchMethod["operationId"] = "Patch" + strcase.ToCamel(rel.ObjectName) + "Of" + strcase.ToCamel(tableInfo.TableName)

				relationsById["patch"] = patchMethod

				deleteMethod["parameters"] = []map[string]interface{}{
					{
						"name": "referenceId",
						"schema": map[string]interface{}{
							"type": "string",
						},
						"required":    true,
						"in":          "path",
						"description": "Reference Id of the " + tableInfo.TableName,
					},
				}

				nestedMap[fmt.Sprintf("/api/%s/{referenceId}/%s", tableInfo.TableName, rel.Object)] = relationsById
			} else {
				relatedTable, exists := tableInfoMap[rel.Subject]
				if !exists || relatedTable.TableName == "" {
					log.Printf("Warning: Related table '%s' not found for relation %v", rel.Subject, rel)
					continue
				}
				getMethod := CreateGetAllMethod(relatedTable, CreateDataInResponse(relatedTable))
				getMethod["summary"] = "Related " + strcase.ToCamel(rel.SubjectName) + " of a " + strcase.ToCamel(tableInfo.TableName)
				getMethod["operationId"] = "Related" + strcase.ToCamel(rel.SubjectName) + "Of" + strcase.ToCamel(tableInfo.TableName)
				patchMethod["summary"] = fmt.Sprintf("Fetch related %s of %v", rel.SubjectName, tableInfo.TableName)
				patchMethod["tags"] = []string{rel.ObjectName, rel.Subject, rel.SubjectName, rel.Object, rel.Relation, "get"}

				deleteMethod := CreateDeleteRelationMethod(relatedTable)
				deleteMethod["description"] = fmt.Sprintf("Remove a related %v from the %v", rel.SubjectName, rel.ObjectName)
				deleteMethod["operationId"] = "Delete" + strcase.ToCamel(relatedTable.TableName) + "Of" + strcase.ToCamel(tableInfo.TableName)
				patchMethod["summary"] = fmt.Sprintf("Delete related %s of %v", rel.SubjectName, tableInfo.TableName)
				patchMethod["tags"] = []string{rel.ObjectName, rel.Subject, rel.SubjectName, rel.Object, rel.Relation, "delete"}
				relationsById["get"] = getMethod

				getMethod["parameters"] = []map[string]interface{}{
					{
						"name": "referenceId",
						"schema": map[string]interface{}{
							"type": "string",
						},
						"required":    true,
						"in":          "path",
						"description": "Reference Id of the " + tableInfo.TableName,
					},
				}

				deleteMethod["parameters"] = []map[string]interface{}{
					{
						"name": "referenceId",
						"schema": map[string]interface{}{
							"type": "string",
						},
						"required":    true,
						"in":          "path",
						"description": "Reference Id of the " + tableInfo.TableName,
					},
				}

				relationsById["delete"] = deleteMethod
				nestedMap[fmt.Sprintf("/api/%s/{referenceId}/%s", tableInfo.TableName, rel.Subject)] = relationsById
			}
			// END: Get relations method

		}

		for k, v := range nestedMap {
			resourcesMap[k] = v
		}

		if tableInfo.IsStateTrackingEnabled {

			//tableInfo.StateMachines

		}

		resourcesMap["/api/"+tableInfo.TableName] = resourceInstance
	}

	for _, action := range config.Actions {
		// Determine appropriate tags based on action name and type
		actionTags := []string{action.OnType}
		actionCategory := categorizeAction(action.Name)
		if actionCategory != "" {
			actionTags = append(actionTags, actionCategory)
		}

		// Create detailed description for the action
		actionDescription := generateActionDescription(action)

		// Generate example request body
		exampleRequest := generateActionRequestExample(action)
		
		// Generate comprehensive security information
		securityInfo := generateActionSecurityInfo(action)

		resourcesMap[fmt.Sprintf("/action/%s/%s", action.OnType, action.Name)] = map[string]interface{}{
			"post": map[string]interface{}{
				"tags":        actionTags,
				"operationId": "Execute" + strcase.ToCamel(action.Name) + "ActionOn" + strcase.ToCamel(action.OnType),
				"summary":     action.Label,
				"description": actionDescription,
				"x-codeSamples": []map[string]interface{}{
					{
						"lang": "curl",
						"source": generateCurlExample(action),
					},
					{
						"lang": "javascript",
						"source": generateJavaScriptExample(action),
					},
				},
				"security": securityInfo,
				"requestBody": map[string]interface{}{
					"required":    len(action.InFields) > 0,
					"description": fmt.Sprintf("Request body for %s action", action.Label),
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": map[string]interface{}{
								"$ref": "#/components/schemas/" + fmt.Sprintf("%sOn%sRequestObject", strcase.ToCamel(action.Name), strcase.ToCamel(action.OnType)),
							},
							"example": exampleRequest,
						},
					},
				},
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": fmt.Sprintf("Successful execution of %s action", action.Label),
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "array",
									"description": "Array of action responses, each representing an outcome of the action",
									"items": map[string]interface{}{
										"$ref": "#/components/schemas/ActionResponse",
									},
								},
								"examples": map[string]interface{}{
									"success": map[string]interface{}{
										"summary": "Successful execution",
										"value": generateActionResponseExample(action),
									},
									"error": map[string]interface{}{
										"summary": "Common error response",
										"value": generateActionErrorExample(action),
									},
								},
							},
						},
					},
					"400": map[string]interface{}{
						"$ref": "#/components/responses/BadRequest",
					},
					"401": map[string]interface{}{
						"$ref": "#/components/responses/Unauthorized",
					},
					"403": map[string]interface{}{
						"$ref": "#/components/responses/Forbidden",
					},
					"422": map[string]interface{}{
						"$ref": "#/components/responses/UnprocessableEntity",
					},
					"429": map[string]interface{}{
						"$ref": "#/components/responses/TooManyRequests",
					},
				},
			},
		}

	}

	// Add the /actions endpoint for listing guest actions
	resourcesMap["/actions"] = map[string]interface{}{
		"get": map[string]interface{}{
			"tags":        []string{"actions", "authentication"},
			"operationId": "ListGuestActions",
			"summary":     "List available guest actions",
			"description": `Returns a list of actions available without authentication. Currently includes:
- **user:signin** - Authenticate and receive JWT token
- **user:signup** - Register new user account

This endpoint is useful for discovering available authentication methods.`,
			"responses": map[string]interface{}{
				"200": map[string]interface{}{
					"description": "List of available guest actions",
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": map[string]interface{}{
								"type": "object",
								"additionalProperties": map[string]interface{}{
									"$ref": "#/components/schemas/ActionDefinition",
								},
							},
							"example": map[string]interface{}{
								"user:signin": map[string]interface{}{
									"Name": "signin",
									"Label": "Sign in",
									"OnType": "user_account",
									"InFields": []map[string]interface{}{
										{"Name": "email", "ColumnType": "email", "IsNullable": false},
										{"Name": "password", "ColumnType": "password", "IsNullable": false},
									},
								},
								"user:signup": map[string]interface{}{
									"Name": "signup",
									"Label": "Sign up",
									"OnType": "user_account",
									"InFields": []map[string]interface{}{
										{"Name": "name", "ColumnType": "label", "IsNullable": false},
										{"Name": "email", "ColumnType": "email", "IsNullable": false},
										{"Name": "password", "ColumnType": "password", "IsNullable": false},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	actionResource := make(map[string]interface{})

	apiDefinition = append(apiDefinition, yaml.MapItem{
		Key:   "paths",
		Value: resourcesMap,
	})
	for n, v := range actionResource {
		apiDefinition = append(apiDefinition, yaml.MapItem{
			Key:   n,
			Value: v,
		})
	}

	apiDefinition = append(apiDefinition, yaml.MapItem{
		Key: "components",
		Value: map[string]interface{}{
			"schemas": typeMap,
			"securitySchemes": map[string]interface{}{
				"bearerAuth": map[string]interface{}{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT",
					"description": `JWT Bearer token authentication. Obtain tokens via POST /auth/signin.

Permission model:
- **Guest**: Basic read permissions (GuestPeek, GuestRead)
- **User**: Full CRUD on owned resources (UserCRUD)
- **Group**: Shared permissions within groups (GroupCRUD)
- **Execute**: Permission to run actions

Example: Authorization: Bearer <your-jwt-token>`,
				},
				"basicAuth": map[string]interface{}{
					"type":        "http",
					"scheme":      "basic",
					"description": "Basic authentication using email and password",
				},
			},
			"parameters": CreateCommonParameters(),
			"responses": CreateCommonResponses(),
		},
	})
	apiDefinition = append(apiDefinition, yaml.MapItem{
		Key: "security",
		Value: []map[string][]string{
			{
				"bearerAuth": []string{},
			},
		},
	})
	
	// Add external documentation
	apiDefinition = append(apiDefinition, yaml.MapItem{
		Key: "externalDocs",
		Value: map[string]interface{}{
			"description": "Full Daptin Documentation",
			"url":         "https://docs.dapt.in",
		},
	})
	
	// Add tags for better organization
	tags := []map[string]interface{}{
		{
			"name":        "Authentication",
			"description": "Authentication endpoints for obtaining JWT tokens",
		},
		{
			"name":        "System Actions",
			"description": `Core system administration actions that affect the entire Daptin instance.

**Key Actions:**
- restart_daptin - Gracefully restart the system
- become_an_administrator - Elevate user privileges
- download_system_schema - Export complete configuration
- upload_system_schema - Import configuration changes`,
			"x-displayName": "System Actions",
		},
		{
			"name":        "Data Operations",
			"description": `Actions for bulk data manipulation, import/export, and data generation.

**Import/Export:**
- export_data - Export in multiple formats (JSON, CSV, XML, etc.)
- import_data - Import from various file formats
- export_csv_data - Quick CSV export

**Data Generation:**
- generate_random_data - Create test data
- upload_csv_to_system_schema - Import CSV with schema detection
- upload_xls_to_system_schema - Import Excel with schema detection`,
			"x-displayName": "Data Operations",
		},
		{
			"name":        "Schema Management",
			"description": `Database schema modification actions. ⚠️ Use with caution - these are destructive operations!

**Column Operations:**
- rename_column - Safely rename columns
- remove_column - ⚠️ Permanently delete columns

**Table Operations:**
- remove_table - ⚠️ Permanently delete tables
- World table actions for schema updates`,
			"x-displayName": "Schema Management",
		},
		{
			"name":        "Storage Management",
			"description": `Cloud storage and file management actions supporting multiple providers (AWS S3, GCS, Azure).

**File Operations:**
- upload_file - Upload to cloud storage
- list_files - Browse directories
- get_file - Download files
- delete_file - Remove files
- move_path - Rename/move files

**Sync Operations:**
- sync_site_storage - Sync site content
- sync_column_storage - Sync file columns
- import_files_from_store - Import file metadata`,
			"x-displayName": "Storage Management",
		},
		{
			"name":        "Certificate Management",
			"description": `SSL/TLS certificate generation and management for secure HTTPS connections.

**Certificate Types:**
- generate_acme_certificate - Let's Encrypt certificates (free, trusted)
- generate_self_certificate - Self-signed certificates (development)

**Certificate Operations:**
- download_certificate - Export certificates
- download_public_key - Export public keys`,
			"x-displayName": "Certificate Management",
		},
		{
			"name":        "User Management",
			"description": `User account lifecycle management and authentication actions.

**Account Creation:**
- signup - Register new account
- signin - Authenticate user

**Authentication Methods:**
- register_otp - Setup SMS/OTP authentication
- verify_otp - Verify OTP codes
- oauth_login_begin - Start OAuth flow
- oauth.login.response - Complete OAuth

**Account Recovery:**
- reset-password - Request password reset
- reset-password-verify - Complete reset`,
			"x-displayName": "User Management",
		},
	}
	
	// Add tags for each table
	for _, tableInfo := range config.Tables {
		if strings.Index(tableInfo.TableName, "_has_") > -1 {
			continue
		}
		tag := map[string]interface{}{
			"name": tableInfo.TableName,
		}
		if tableInfo.TableDescription != "" {
			tag["description"] = tableInfo.TableDescription
		}
		tags = append(tags, tag)
	}
	
	apiDefinition = append(apiDefinition, yaml.MapItem{
		Key:   "tags",
		Value: tags,
	})

	ym, _ := yaml.Marshal(apiDefinition)
	return string(ym)

}

func CreateDataInResponse(tableInfo table_info.TableInfo) map[string]interface{} {
	relationshipMap := make(map[string]interface{}, 0)
	for _, relation := range tableInfo.Relations {
		if relation.Object == tableInfo.TableName {
			relationshipMap[relation.SubjectName] = "IncludedRelationship"
		} else {
			relationshipMap[relation.ObjectName] = "IncludedRelationship"
		}
	}

	var dataInResponse = map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"attributes": map[string]interface{}{
				"schema": map[string]interface{}{
					"$ref": "#/components/schemas/New" + strcase.ToCamel(tableInfo.TableName),
				},
			},
			"id": map[string]interface{}{
				"type": "string",
			},
			"type": map[string]interface{}{
				"type": "string",
			},
			"relationships": map[string]interface{}{
				"type":       "object",
				"properties": relationshipMap,
			},
		},
	}
	return dataInResponse
}
func CreatePostMethod(tableInfo table_info.TableInfo, dataInResponse map[string]interface{}) map[string]interface{} {
	postMethod := make(map[string]interface{})
	postMethod["operationId"] = fmt.Sprintf("Create%s", strcase.ToCamel(tableInfo.TableName))
	postMethod["summary"] = fmt.Sprintf("Create a new %v", tableInfo.TableName)
	postMethod["tags"] = []string{tableInfo.TableName, "create"}
	postBody := make(map[string]interface{})

	postBody["description"] = tableInfo.TableName + " to create"
	postBody["required"] = true
	postBody["content"] = map[string]interface{}{
		"application/json": map[string]interface{}{
			"schema": map[string]interface{}{
				"type": "object",
				"required": []string{"data"},
				"properties": map[string]interface{}{
					"data": map[string]interface{}{
						"type": "object",
						"required": []string{"type", "attributes"},
						"properties": map[string]interface{}{
							"type": map[string]interface{}{
								"type":  "string",
								"enum": []string{tableInfo.TableName},
								"description": "Resource type identifier",
							},
							"attributes": map[string]interface{}{
								"$ref": "#/components/schemas/New" + strcase.ToCamel(tableInfo.TableName),
							},
							"relationships": map[string]interface{}{
								"type": "object",
								"description": "Related resources to create relationships with",
								"additionalProperties": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"data": map[string]interface{}{
											"type": "object",
											"properties": map[string]interface{}{
												"type": map[string]interface{}{
													"type": "string",
												},
												"id": map[string]interface{}{
													"type": "string",
													"format": "uuid",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			"example": CreatePostRequestExample(tableInfo),
		},
	}
	postMethod["requestBody"] = postBody
	postResponseMap := make(map[string]interface{})
	postResponseBody := make(map[string]interface{})
	postResponseBody["type"] = "object"

	postResponseBody = map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"data": map[string]interface{}{
				"$ref": "#/components/schemas/" + strcase.ToCamel(tableInfo.TableName),
			},
			"links": map[string]interface{}{
				"$ref": "#/components/schemas/PaginationStatus",
			},
		},
	}
	postOkResponse := make(map[string]interface{})
	postOkResponse["description"] = tableInfo.TableName + " response"

	postOkResponse["content"] = map[string]interface{}{
		"application/json": map[string]interface{}{
			"schema": postResponseBody,
		},
	}

	postResponseMap["201"] = postOkResponse
	postResponseMap["400"] = map[string]interface{}{
		"$ref": "#/components/responses/BadRequest",
	}
	postResponseMap["401"] = map[string]interface{}{
		"$ref": "#/components/responses/Unauthorized",
	}
	postResponseMap["403"] = map[string]interface{}{
		"$ref": "#/components/responses/Forbidden",
	}
	postResponseMap["422"] = map[string]interface{}{
		"$ref": "#/components/responses/UnprocessableEntity",
	}
	postResponseMap["429"] = map[string]interface{}{
		"$ref": "#/components/responses/TooManyRequests",
	}
	postMethod["responses"] = postResponseMap
	return postMethod
}
func CreateGetAllMethod(tableInfo table_info.TableInfo, dataInResponse map[string]interface{}) map[string]interface{} {
	getAllMethod := make(map[string]interface{})
	getAllMethod["description"] = "Returns a list of " + ProperCase(tableInfo.TableName)
	getAllMethod["operationId"] = "Get" + strcase.ToCamel(tableInfo.TableName)
	getAllMethod["summary"] = "List all " + tableInfo.TableName
	getAllMethod["tags"] = []string{tableInfo.TableName, "find", "get"}
	getAllMethod["parameters"] = []map[string]interface{}{
		{
			"$ref": "#/components/parameters/Sort",
		},
		{
			"$ref": "#/components/parameters/PageNumber",
		},
		{
			"$ref": "#/components/parameters/PageSize",
		},
		{
			"$ref": "#/components/parameters/Query",
		},
		{
			"$ref": "#/components/parameters/Filter",
		},
		{
			"$ref": "#/components/parameters/IncludedRelations",
		},
		{
			"$ref": "#/components/parameters/Fields",
		},
		{
			"name": "page[after]",
			"in":   "query",
			"schema": map[string]interface{}{
				"type":   "string",
				"format": "uuid",
			},
			"required":    false,
			"description": "Reference ID for cursor-based pagination. Returns results after this ID.",
			"example":     "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			"name": "group",
			"in":   "query",
			"schema": map[string]interface{}{
				"type": "string",
			},
			"required":    false,
			"description": "Base64 encoded JSON for grouping results. Example: {\"column\":\"category\",\"function\":\"count\"}",
			"example":     "eyJjb2x1bW4iOiJjYXRlZ29yeSIsImZ1bmN0aW9uIjoiY291bnQifQ==",
		},
		{
			"name": "accept",
			"in":   "header",
			"schema": map[string]interface{}{
				"type": "string",
				"enum": []string{"application/json", "text/csv", "application/xml"},
				"default": "application/json",
			},
			"required":    false,
			"description": "Response format. Supported: application/json (default), text/csv, application/xml",
		},
	}
	getResponseMap := make(map[string]interface{})
	get200Response := make(map[string]interface{})
	get200Response["description"] = "list of all " + tableInfo.TableName
	get200Response["content"] = map[string]interface{}{
		"application/json": map[string]interface{}{
			"schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"data": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"$ref": "#/components/schemas/" + strcase.ToCamel(tableInfo.TableName),
						},
					},
					"links": map[string]interface{}{
						"$ref": "#/components/schemas/PaginationStatus",
					},
					"included": map[string]interface{}{
						"type": "array",
						"description": "Included related resources when using included_relations parameter",
						"items": map[string]interface{}{
							"type": "object",
						},
					},
				},
			},
		},
		"text/csv": map[string]interface{}{
			"schema": map[string]interface{}{
				"type": "string",
				"description": "CSV formatted data. Use Accept: text/csv header.",
				"example": "id,name,email,created_at\n1,John Doe,john@example.com,2024-01-15T09:30:00Z\n",
			},
		},
		"application/xml": map[string]interface{}{
			"schema": map[string]interface{}{
				"type": "string",
				"description": "XML formatted data. Use Accept: application/xml header.",
				"example": "<data><item><id>1</id><name>John Doe</name></item></data>",
			},
		},
	}

	getResponseMap["200"] = get200Response
	getResponseMap["400"] = map[string]interface{}{
		"$ref": "#/components/responses/BadRequest",
	}
	getResponseMap["401"] = map[string]interface{}{
		"$ref": "#/components/responses/Unauthorized",
	}
	getResponseMap["403"] = map[string]interface{}{
		"$ref": "#/components/responses/Forbidden",
	}
	getResponseMap["429"] = map[string]interface{}{
		"$ref": "#/components/responses/TooManyRequests",
	}
	getResponseMap["500"] = map[string]interface{}{
		"$ref": "#/components/responses/InternalServerError",
	}
	getAllMethod["responses"] = getResponseMap
	return getAllMethod
}
func ProperCase(str string) string {
	if len(str) == 0 {
		return ""
	}
	if len(str) == 1 {
		return strings.ToUpper(str)
	}
	st := str[1:]
	st = strings.Replace(st, "_", " ", -1)
	st = strings.Replace(st, ".", " ", -1)
	return strings.ToUpper(str[0:1]) + st
}

func CreateCommonParameters() map[string]interface{} {
	return map[string]interface{}{
		"PageNumber": map[string]interface{}{
			"name":        "page[number]",
			"in":          "query",
			"description": "Page number for pagination (1-based)",
			"schema": map[string]interface{}{
				"type":    "integer",
				"minimum": 1,
				"default": 1,
			},
			"example": 2,
		},
		"PageSize": map[string]interface{}{
			"name":        "page[size]",
			"in":          "query",
			"description": "Number of items per page",
			"schema": map[string]interface{}{
				"type":    "integer",
				"minimum": 1,
				"maximum": 100,
				"default": 20,
			},
			"example": 20,
		},
		"Sort": map[string]interface{}{
			"name":        "sort",
			"in":          "query",
			"description": "Sort fields. Use - prefix for descending order. Multiple fields can be comma-separated.",
			"schema": map[string]interface{}{
				"type": "string",
			},
			"example": "-created_at,name",
		},
		"Filter": map[string]interface{}{
			"name":        "filter",
			"in":          "query",
			"description": "JSON-based filtering. Supports operators: eq, ne, gt, gte, lt, lte, like, in, between",
			"schema": map[string]interface{}{
				"type": "string",
			},
			"example": `{"name":{"like":"%john%"},"age":{"gte":18}}`,
		},
		"Query": map[string]interface{}{
			"name":        "query",
			"in":          "query",
			"description": "Full-text search across all indexed text columns",
			"schema": map[string]interface{}{
				"type": "string",
			},
			"example": "search term",
		},
		"IncludedRelations": map[string]interface{}{
			"name":        "included_relations",
			"in":          "query",
			"description": "Comma-separated list of relationships to include in the response",
			"schema": map[string]interface{}{
				"type": "string",
			},
			"example": "author,comments,tags",
		},
		"Fields": map[string]interface{}{
			"name":        "fields",
			"in":          "query",
			"description": "Comma-separated list of fields to include in the response. Reduces payload size.",
			"schema": map[string]interface{}{
				"type": "string",
			},
			"example": "id,name,email,created_at",
		},
	}
}

func CreateCommonResponses() map[string]interface{} {
	return map[string]interface{}{
		"BadRequest": map[string]interface{}{
			"description": "Bad Request - Invalid input parameters or malformed request",
			"content": map[string]interface{}{
				"application/json": map[string]interface{}{
					"schema": map[string]interface{}{
						"$ref": "#/components/schemas/ErrorResponse",
					},
					"example": map[string]interface{}{
						"errors": []map[string]interface{}{
							{
								"status": "400",
								"title":  "Bad Request",
								"detail": "The filter parameter contains invalid JSON",
								"source": map[string]string{
									"parameter": "filter",
								},
							},
						},
					},
				},
			},
		},
		"Unauthorized": map[string]interface{}{
			"description": "Unauthorized - Missing or invalid authentication token",
			"content": map[string]interface{}{
				"application/json": map[string]interface{}{
					"schema": map[string]interface{}{
						"$ref": "#/components/schemas/ErrorResponse",
					},
					"example": map[string]interface{}{
						"errors": []map[string]interface{}{
							{
								"status": "401",
								"title":  "Unauthorized",
								"detail": "Invalid or expired JWT token",
							},
						},
					},
				},
			},
		},
		"Forbidden": map[string]interface{}{
			"description": "Forbidden - You don't have permission to access this resource",
			"content": map[string]interface{}{
				"application/json": map[string]interface{}{
					"schema": map[string]interface{}{
						"$ref": "#/components/schemas/ErrorResponse",
					},
					"example": map[string]interface{}{
						"errors": []map[string]interface{}{
							{
								"status": "403",
								"title":  "Forbidden",
								"detail": "You don't have permission to update this resource",
							},
						},
					},
				},
			},
		},
		"NotFound": map[string]interface{}{
			"description": "Not Found - The requested resource doesn't exist",
			"content": map[string]interface{}{
				"application/json": map[string]interface{}{
					"schema": map[string]interface{}{
						"$ref": "#/components/schemas/ErrorResponse",
					},
					"example": map[string]interface{}{
						"errors": []map[string]interface{}{
							{
								"status": "404",
								"title":  "Not Found",
								"detail": "Resource with the specified ID was not found",
							},
						},
					},
				},
			},
		},
		"UnprocessableEntity": map[string]interface{}{
			"description": "Unprocessable Entity - Validation errors",
			"content": map[string]interface{}{
				"application/json": map[string]interface{}{
					"schema": map[string]interface{}{
						"$ref": "#/components/schemas/ErrorResponse",
					},
					"example": map[string]interface{}{
						"errors": []map[string]interface{}{
							{
								"status": "422",
								"title":  "Validation Error",
								"detail": "Email must be a valid email address",
								"source": map[string]string{
									"pointer": "/data/attributes/email",
								},
							},
						},
					},
				},
			},
		},
		"TooManyRequests": map[string]interface{}{
			"description": "Too Many Requests - Rate limit exceeded",
			"headers": map[string]interface{}{
				"X-RateLimit-Limit": map[string]interface{}{
					"description": "The maximum number of requests allowed",
					"schema": map[string]interface{}{
						"type": "integer",
					},
				},
				"X-RateLimit-Remaining": map[string]interface{}{
					"description": "The number of requests remaining",
					"schema": map[string]interface{}{
						"type": "integer",
					},
				},
				"X-RateLimit-Reset": map[string]interface{}{
					"description": "Unix timestamp when rate limit resets",
					"schema": map[string]interface{}{
						"type": "integer",
					},
				},
			},
			"content": map[string]interface{}{
				"application/json": map[string]interface{}{
					"schema": map[string]interface{}{
						"$ref": "#/components/schemas/RateLimitResponse",
					},
				},
			},
		},
		"InternalServerError": map[string]interface{}{
			"description": "Internal Server Error - Something went wrong on the server",
			"content": map[string]interface{}{
				"application/json": map[string]interface{}{
					"schema": map[string]interface{}{
						"$ref": "#/components/schemas/ErrorResponse",
					},
					"example": map[string]interface{}{
						"errors": []map[string]interface{}{
							{
								"status": "500",
								"title":  "Internal Server Error",
								"detail": "An unexpected error occurred. Please try again later.",
							},
						},
					},
				},
			},
		},
	}
}

func CreateDeleteMethod(tableInfo table_info.TableInfo) map[string]interface{} {
	deleteByIdMethod := make(map[string]interface{})
	deleteByIdMethod200Response := make(map[string]interface{})
	deleteByIdResponseMap := make(map[string]interface{})
	deleteByIdMethod200Response["description"] = "delete " + tableInfo.TableName + " by reference id"
	deleteByIdMethod200Response["content"] = map[string]interface{}{
		"application/json": map[string]interface{}{
			"schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"message": map[string]interface{}{
								"type": "string",
								"example": "Resource deleted successfully",
							},
						},
					},
				},
			},
		},
	}
	deleteByIdResponseMap["204"] = map[string]interface{}{
		"description": "No Content - Resource deleted successfully",
	}
	deleteByIdResponseMap["401"] = map[string]interface{}{
		"$ref": "#/components/responses/Unauthorized",
	}
	deleteByIdResponseMap["403"] = map[string]interface{}{
		"$ref": "#/components/responses/Forbidden",
	}
	deleteByIdResponseMap["404"] = map[string]interface{}{
		"$ref": "#/components/responses/NotFound",
	}
	deleteByIdResponseMap["429"] = map[string]interface{}{
		"$ref": "#/components/responses/TooManyRequests",
	}
	deleteByIdMethod["responses"] = deleteByIdResponseMap

	deleteByIdMethod["description"] = fmt.Sprintf("Delete a %v", tableInfo.TableName)

	deleteByIdMethod["summary"] = fmt.Sprintf("Delete %v", tableInfo.TableName)
	deleteByIdMethod["tags"] = []string{tableInfo.TableName, "delete"}
	deleteByIdMethod["parameters"] = []map[string]interface{}{
		{
			"name": "referenceId",
			"schema": map[string]interface{}{
				"type": "string",
			},
			"required":    true,
			"in":          "path",
			"description": "Reference Id of the " + tableInfo.TableName,
		},
	}
	deleteByIdMethod["operationId"] = fmt.Sprintf("Delete%s", strcase.ToCamel(tableInfo.TableName))
	return deleteByIdMethod
}

func CreateDeleteRelationMethod(tableInfo table_info.TableInfo) map[string]interface{} {
	deleteByIdMethod := make(map[string]interface{})
	deleteByIdMethod200Response := make(map[string]interface{})
	deleteByIdMethod200Response["description"] = "Successful deletion of relation " + tableInfo.TableName
	deleteBody := make(map[string]interface{})
	deleteBody["type"] = tableInfo.TableName
	deleteByIdMethod["description"] = "Delete a " + tableInfo.TableName

	deleteByIdResponseMap := make(map[string]interface{})
	deleteByIdResponseMap["200"] = deleteByIdMethod200Response
	deleteByIdMethod["responses"] = deleteByIdResponseMap
	deleteByIdMethod["description"] = fmt.Sprintf("Remove a related %v ", tableInfo.TableName)
	deleteByIdMethod["tags"] = []string{tableInfo.TableName}
	deleteByIdMethod["parameters"] = []map[string]interface{}{
		{
			"name": "referenceId",
			"schema": map[string]interface{}{
				"type": "string",
			},
			"required":    true,
			"in":          "path",
			"description": "Reference Id of the " + tableInfo.TableName,
		},
	}
	deleteByIdMethod["requestBody"] = map[string]interface{}{
		"content": map[string]interface{}{
			"application/json": map[string]interface{}{
				"schema": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"data": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"type": map[string]interface{}{
										"type":    "string",
										"default": tableInfo.TableName,
									},
									"id": map[string]interface{}{
										"type": "string",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	return deleteByIdMethod
}

func CreatePatchRelationMethod(tableInfo table_info.TableInfo) map[string]interface{} {
	patchByIdMethod := make(map[string]interface{})
	patchByIdMethod200Response := make(map[string]interface{})
	patchByIdMethod200Response["description"] = "Add relation " + tableInfo.TableName
	patchBody := make(map[string]interface{})
	patchBody["type"] = tableInfo.TableName
	patchByIdMethod["description"] = "Patch relation to add " + tableInfo.TableName

	patchByIdResponseMap := make(map[string]interface{})
	patchByIdResponseMap["200"] = patchByIdMethod200Response
	patchByIdMethod["responses"] = patchByIdResponseMap
	patchByIdMethod["description"] = fmt.Sprintf("Patch and add related %v ", tableInfo.TableName)
	patchByIdMethod["tags"] = []string{tableInfo.TableName}
	patchByIdMethod["parameters"] = []map[string]interface{}{
		{
			"name": "referenceId",
			"schema": map[string]interface{}{
				"type": "string",
			},
			"required":    true,
			"in":          "path",
			"description": "Reference Id of the " + tableInfo.TableName,
		},
	}
	patchByIdMethod["requestBody"] = map[string]interface{}{
		"content": map[string]interface{}{
			"application/json": map[string]interface{}{
				"schema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"data": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"type": map[string]interface{}{
									"type":    "string",
									"default": tableInfo.TableName,
								},
								"id": map[string]interface{}{
									"type": "string",
								},
							},
						},
					},
				},
			},
		},
	}

	return patchByIdMethod
}

func CreateGetMethod(tableInfo table_info.TableInfo, dataInResponse map[string]interface{}) map[string]interface{} {
	getByIdMethod := make(map[string]interface{})
	getByIdMethod200Response := make(map[string]interface{})
	getByIdMethod["tags"] = []string{tableInfo.TableName}
	getByIdMethod200Response["description"] = "get " + tableInfo.TableName + " by reference id"
	getByIdMethod200Response["content"] = map[string]interface{}{
		"application/json": map[string]interface{}{
			"schema": map[string]interface{}{
				"$ref": "#/components/schemas/" + strcase.ToCamel(tableInfo.TableName),
			},
		},
	}

	getByIdResponseMap := make(map[string]interface{})
	getByIdResponseMap["200"] = getByIdMethod200Response
	getByIdResponseMap["401"] = map[string]interface{}{
		"$ref": "#/components/responses/Unauthorized",
	}
	getByIdResponseMap["403"] = map[string]interface{}{
		"$ref": "#/components/responses/Forbidden",
	}
	getByIdResponseMap["404"] = map[string]interface{}{
		"$ref": "#/components/responses/NotFound",
	}
	getByIdResponseMap["429"] = map[string]interface{}{
		"$ref": "#/components/responses/TooManyRequests",
	}
	getByIdMethod["responses"] = getByIdResponseMap

	getByIdMethod["parameters"] = []map[string]interface{}{
		{
			"name": "referenceId",
			"schema": map[string]interface{}{
				"type": "string",
			},
			"required":    true,
			"in":          "path",
			"description": "Reference Id of the " + tableInfo.TableName,
		},
	}

	getByIdMethod["summary"] = fmt.Sprintf("Get %v by id", tableInfo.TableName)
	getByIdMethod["description"] = fmt.Sprintf("Get %v by id", tableInfo.TableName)
	getByIdMethod["operationId"] = fmt.Sprintf("Get%sByReferenceId", strcase.ToCamel(tableInfo.TableName))
	return getByIdMethod
}
func CreatePostRequestExample(tableInfo table_info.TableInfo) map[string]interface{} {
	attributes := make(map[string]interface{})
	for _, col := range tableInfo.Columns {
		if col.IsForeignKey || skipColumns[col.ColumnName] || resource.IsStandardColumn(col.ColumnName) {
			continue
		}
		
		switch col.ColumnType {
		case "email":
			attributes[col.ColumnName] = "user@example.com"
		case "name":
			attributes[col.ColumnName] = "John Doe"
		case "label":
			attributes[col.ColumnName] = "Example Label"
		case "url":
			attributes[col.ColumnName] = "https://example.com"
		case "date":
			attributes[col.ColumnName] = "2024-01-15"
		case "datetime":
			attributes[col.ColumnName] = "2024-01-15T09:30:00Z"
		case "integer":
			attributes[col.ColumnName] = 42
		case "float":
			attributes[col.ColumnName] = 3.14
		case "boolean":
			attributes[col.ColumnName] = true
		case "text":
			attributes[col.ColumnName] = "This is a sample text content"
		default:
			if col.ColumnType == "string" || col.DataType == "varchar" {
				attributes[col.ColumnName] = "example " + col.ColumnName
			}
		}
	}
	
	return map[string]interface{}{
		"data": map[string]interface{}{
			"type":       tableInfo.TableName,
			"attributes": attributes,
		},
	}
}

func CreatePatchMethod(tableInfo table_info.TableInfo) map[string]interface{} {

	patchMethod := make(map[string]interface{})
	patchMethod["operationId"] = fmt.Sprintf("Update%s", strcase.ToCamel(tableInfo.TableName))
	patchMethod["summary"] = fmt.Sprintf("Update existing %v", tableInfo.TableName)
	patchMethod["description"] = fmt.Sprintf("Edit an existing %s", tableInfo.TableName)
	patchMethod["tags"] = []string{tableInfo.TableName}
	patchBody := make(map[string]interface{})
	patchBody["type"] = tableInfo.TableName
	patchResponseMap := make(map[string]interface{})
	patchOkResponse := make(map[string]interface{})
	patchResponseBody := make(map[string]interface{})
	patchResponseBody["type"] = "object"
	patchRelationshipMap := make(map[string]interface{}, 0)

	patchMethod["requestBody"] = map[string]interface{}{
		"content": map[string]interface{}{
			"application/json": map[string]interface{}{
				"schema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"data": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"attributes": map[string]interface{}{
									"$ref": "#/components/schemas/" + strcase.ToCamel(tableInfo.TableName),
								},
								"id": map[string]interface{}{
									"type": "string",
								},
							},
						},
					},
				},
			},
		},
	}

	for _, relation := range tableInfo.Relations {
		if relation.Object == tableInfo.TableName {
			patchRelationshipMap[relation.SubjectName] = map[string]interface{}{
				"$ref": "#/components/schemas/IncludedRelationship",
			}
		} else {
			patchRelationshipMap[relation.ObjectName] = map[string]interface{}{
				"$ref": "#/components/schemas/IncludedRelationship",
			}
		}
	}
	var patchDataInResponse = map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"attributes": map[string]interface{}{
				"$ref": "#/components/schemas/" + strcase.ToCamel(tableInfo.TableName),
			},
			"id": map[string]interface{}{
				"type": "string",
			},
			"type": map[string]interface{}{
				"type": "string",
			},
			"relationships": map[string]interface{}{
				"type":       "object",
				"properties": patchRelationshipMap,
			},
		},
	}
	patchResponseBody = map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"data": patchDataInResponse,
			"links": map[string]interface{}{
				"$ref": "#/components/schemas/PaginationStatus",
			},
		},
	}
	patchOkResponse["content"] = map[string]interface{}{
		"application/json": map[string]interface{}{
			"schema": patchResponseBody,
		},
	}
	patchOkResponse["description"] = "updated " + tableInfo.TableName
	patchResponseMap["200"] = patchOkResponse
	patchMethod["parameters"] = []map[string]interface{}{
		{
			"name": "referenceId",
			"schema": map[string]interface{}{
				"type": "string",
			},
			"required":    true,
			"in":          "path",
			"description": "Reference Id of the " + tableInfo.TableName,
		},
	}
	patchMethod["responses"] = patchResponseMap
	return patchMethod
}

func categorizeAction(actionName string) string {
	categories := map[string]string{
		// Data Operations
		"import_files_from_store": "Data Operations",
		"export_data": "Data Operations",
		"export_csv_data": "Data Operations",
		"import_data": "Data Operations",
		"generate_random_data": "Data Operations",
		"upload_xls_to_system_schema": "Data Operations",
		"upload_csv_to_system_schema": "Data Operations",
		"add_exchange": "Data Operations",
		
		// Schema Management
		"remove_column": "Schema Management",
		"remove_table": "Schema Management",
		"rename_column": "Schema Management",
		"upload_system_schema": "Schema Management",
		"download_system_schema": "Schema Management",
		
		// Storage Management
		"sync_site_storage": "Storage Management",
		"sync_column_storage": "Storage Management",
		"upload_file": "Storage Management",
		"create_site": "Storage Management",
		"delete_path": "Storage Management",
		"create_folder": "Storage Management",
		"move_path": "Storage Management",
		"list_files": "Storage Management",
		"get_file": "Storage Management",
		"delete_file": "Storage Management",
		"import_cloudstore_files": "Storage Management",
		
		// Certificate Management
		"download_certificate": "Certificate Management",
		"download_public_key": "Certificate Management",
		"generate_acme_certificate": "Certificate Management",
		"generate_self_certificate": "Certificate Management",
		
		// User Management
		"signup": "User Management",
		"signin": "User Management",
		"register_otp": "User Management",
		"verify_otp": "User Management",
		"send_otp": "User Management",
		"reset-password": "User Management",
		"reset-password-verify": "User Management",
		"oauth_login_begin": "User Management",
		"oauth.login.response": "User Management",
		
		// System Actions
		"restart_daptin": "System Actions",
		"become_an_administrator": "System Actions",
		"sync_mail_servers": "System Actions",
		"install_integration": "System Actions",
		"mail_send": "System Actions",
		"mail_send_ses": "System Actions",
	}
	
	if category, ok := categories[actionName]; ok {
		return category
	}
	
	// Try to categorize by patterns
	if strings.Contains(actionName, "mail") || strings.Contains(actionName, "email") {
		return "System Actions"
	}
	if strings.Contains(actionName, "file") || strings.Contains(actionName, "storage") {
		return "Storage Management"
	}
	if strings.Contains(actionName, "oauth") || strings.Contains(actionName, "login") || strings.Contains(actionName, "auth") {
		return "User Management"
	}
	
	return ""
}

func generateActionDescription(action actionresponse.Action) string {
	descriptions := map[string]string{
		"import_files_from_store": `Imports file metadata from cloud storage into a specified database table. This action scans the configured cloud storage path and creates database records for each file found, storing the file path and metadata.

**Side Effects:**
- Creates new records in the target table
- Each record contains file metadata in a JSON field
- Does not copy actual file content, only references

**Required Permissions:** Execute permission on world table

**Common Errors:**
- "invalid table" - Target table doesn't exist
- Cloud storage not configured properly
- Insufficient permissions on target table`,
		
		"install_integration": "Installs and configures a third-party integration. This action sets up external service integrations, enabling Daptin to connect with various APIs, webhooks, and external systems.",
		
		"download_certificate": `Downloads the SSL/TLS certificate in PEM format for a specific hostname. Returns the certificate file as a base64-encoded download.

**Response Type:** client.file.download

**File Format:** PEM certificate (.crt)

**Common Use Cases:**
- Backup SSL certificates
- Certificate inspection and validation
- Deployment to other systems`,
		
		"download_public_key": "Downloads the public key associated with a certificate. This action provides access to the public key component of SSL/TLS certificates for cryptographic operations or verification purposes.",
		
		"generate_acme_certificate": `Generates a Let's Encrypt SSL/TLS certificate using the ACME protocol. Automatically handles domain validation and certificate issuance.

**Prerequisites:**
- Domain must be publicly accessible
- Port 80 must be available for HTTP challenge
- Valid email for notifications

**Side Effects:**
- Creates/updates certificate record
- Configures HTTPS for the hostname
- Stores certificate and private key

**Rate Limits:** Let's Encrypt rate limits apply (5 certificates per domain per week)`,
		
		"generate_self_certificate": "Generates a self-signed SSL/TLS certificate. Useful for development environments or internal services where a trusted certificate authority is not required.",
		
		"register_otp": `Registers a mobile number for OTP-based authentication. Associates a phone number with the current user account for two-factor authentication.

**SMS Provider Required:** Configured SMS gateway (Twilio, AWS SNS, etc.)

**Validation:**
- Mobile number format validation
- Duplicate number check
- User authentication required`,
		
		"verify_otp": `Verifies an OTP code for authentication. Validates the one-time password and returns authentication tokens.

**Response Types:**
- client.token.set - JWT token for authentication
- client.cookie.set - HTTP cookie with token
- client.notify - Success/failure notification
- client.redirect - Redirect after successful login

**OTP Expiry:** Codes expire after 5 minutes`,
		
		"send_otp": "Sends a one-time password to a registered mobile number or email. Use this action to trigger OTP delivery for authentication or verification purposes.",
		
		"remove_column": `Permanently removes a column from a database table. This is a destructive DDL operation that cannot be undone.

**⚠️ WARNING:** All data in the column will be permanently deleted!

**Side Effects:**
- Executes ALTER TABLE DROP COLUMN
- Updates world schema metadata
- Triggers system reload (if configured)

**Validation:**
- Column must exist
- Cannot remove system columns
- Checks for dependent relationships`,
		
		"remove_table": `Permanently deletes an entire database table and all its data. This is an irreversible destructive operation.

**⚠️ CRITICAL WARNING:** 
- All table data will be permanently lost
- All relationships will be broken
- All associated actions will be deleted
- Cannot be undone!

**Required:** Administrator privileges`,
		
		"rename_column": `Renames a column in a database table while preserving all data.

**Side Effects:**
- Executes ALTER TABLE RENAME COLUMN
- Updates world schema metadata
- Updates all references in the system

**Validation:**
- New name cannot be a reserved word
- Column must exist
- New name must be unique in table
- Spaces converted to underscores`,
		
		"sync_site_storage": `Synchronizes files between a site and its configured cloud storage using rclone. Performs bidirectional sync to ensure consistency.

**Sync Direction:** Bidirectional (local ↔ cloud)

**Performance:** May take time for large sites

**Side Effects:**
- Creates/updates/deletes files in cloud storage
- Updates local file cache
- Logs sync operations`,
		
		"sync_column_storage": "Synchronizes file-type column data with external cloud storage. Ensures that files referenced in database columns are properly stored in the configured cloud storage backend.",
		
		"sync_mail_servers": `Synchronizes email configurations with IMAP/SMTP servers. Fetches emails and updates mailbox state.

**Supported Protocols:**
- IMAP for receiving
- SMTP for sending
- OAuth2 authentication

**Side Effects:**
- Creates mail_box records for new emails
- Updates sync timestamps
- May trigger email processing workflows`,
		
		"restart_daptin": `Initiates a graceful system restart. Returns success immediately but actual restart happens asynchronously.

**Response Types:**
- client.notify - "Initiating system update"
- client.redirect - Redirects to home after 5 seconds

**Note:** The actual restart is handled by the process manager (systemd, Docker, etc.)`,
		
		"generate_random_data": `Generates realistic test data for a specified table based on column types.

**Data Generation:**
- Names: Realistic person names
- Emails: Valid format test emails
- Dates: Random dates within reasonable ranges
- Numbers: Random within column constraints
- Text: Lorem ipsum style content

**Batch Size:** Processes in batches of 100 records`,
		
		"export_data": `Exports table data with advanced options for filtering and formatting.

**Supported Formats:**
- JSON (default) - Complete data with relationships
- CSV - Flat tabular format
- XLSX - Excel with formatting
- XML - Structured XML
- PDF - Formatted reports
- HTML - Web-viewable tables

**Options:**
- Column selection
- Include/exclude headers
- Custom page size
- Filter expressions

**Response:** Base64-encoded file download`,
		
		"export_csv_data": "Exports table data specifically in CSV format. Optimized for spreadsheet applications and data analysis tools. Simpler alternative to export_data when only CSV is needed.",
		
		"import_data": `Imports data from uploaded files with automatic format detection and validation.

**Supported Formats:**
- JSON (including JSON arrays)
- CSV (with header detection)
- Excel (.xlsx, .xls)
- YAML
- TOML
- HCL

**Options:**
- truncate_before_insert: Clear existing data
- batch_size: Processing chunk size

**Validation:**
- Column type checking
- Constraint validation
- Foreign key verification`,
		
		"upload_file": `Uploads a file to configured cloud storage with automatic path resolution.

**File Handling:**
- Automatic MIME type detection
- Path sanitization
- Overwrite protection (configurable)
- Progress tracking for large files

**Storage Providers:** AWS S3, Google Cloud Storage, Azure Blob, Local filesystem`,
		
		"create_site": `Creates a new website/application site with hosting configuration.

**Site Types:**
- static - Plain HTML/CSS/JS
- hugo - Hugo static site generator
- jekyll - Jekyll static site generator

**Auto-Configuration:**
- Creates storage directories
- Sets up routing rules
- Configures SSL (if enabled)
- Initializes site templates`,
		
		"delete_path": "Deletes a file or directory from cloud storage. Removes specified paths from the configured storage backend. Supports recursive deletion for directories.",
		
		"create_folder": "Creates a new directory in cloud storage. Establishes folder structures for organizing files in external storage systems. Creates parent directories if needed.",
		
		"move_path": "Moves or renames files/folders in cloud storage. Relocates content within the storage system while preserving file integrity and metadata.",
		
		"list_files": `Lists files and directories at a specified path with detailed metadata.

**Response Format:**
- File/directory names
- Size in bytes
- Last modified timestamp
- MIME types
- Directory indicators

**Supports:** Pagination, sorting, filtering`,
		
		"get_file": "Retrieves a specific file from site storage. Downloads file content for viewing or processing. Returns base64-encoded content for binary files.",
		
		"delete_file": "Removes a specific file from site storage. Permanently deletes the specified file from the site's storage location. Cannot be undone.",
		
		"upload_system_schema": `Uploads and applies a new system configuration schema. Supports incremental updates and full replacements.

**Supported Formats:**
- JSON schema files
- YAML configurations
- SQL schema dumps

**Validation:**
- Schema syntax checking
- Compatibility verification
- Relationship validation

**⚠️ Caution:** Can significantly modify system behavior`,
		
		"download_system_schema": `Downloads the complete system configuration including all tables, columns, actions, and relationships.

**Export Contains:**
- Table definitions
- Column specifications
- Relationships and foreign keys
- Actions and workflows
- Permission settings
- State machines

**Format:** JSON schema compatible with upload_system_schema`,
		
		"become_an_administrator": `Elevates the current user to become the sole system administrator.

**🚨 CRITICAL BOOTSTRAPPING INFORMATION:**
- **Before first admin is set**: ALL users have full admin privileges by default
- **After this action**: Only the user who invoked this becomes admin, all other users lose admin rights
- **This can only be done ONCE**: After an admin exists, no other user can become admin
- **No going back**: Once set, the admin role cannot be transferred or revoked

**⚠️ SECURITY IMPLICATIONS:**
- First user to invoke this action becomes permanent admin
- All other existing users immediately lose admin privileges
- New users created after this will have standard permissions
- System requires restart after admin is set

**Prerequisites:**
- Must be authenticated (have valid JWT token)
- No administrator must exist in the system yet
- Will fail if an admin already exists

**Use Cases:**
- Initial system setup after first user creation
- Securing a fresh Daptin installation
- Converting from development to production mode

**Side Effects:**
- System will restart automatically
- All caches will be cleared
- Other logged-in users will need to re-authenticate

**Response:** Redirect to home page after 7 seconds`,
		
		"signup": `Creates a new user account with email/password authentication.

**🎯 IMPORTANT FOR NEW INSTALLATIONS:**
- If NO admin exists yet: New user will have FULL ADMIN privileges
- If admin exists: New user will have standard permissions
- First user should immediately invoke 'become_an_administrator' to secure the system

**Validation Requirements:**
- **Email**: Valid format, must be unique in system
- **Password**: MINIMUM 8 CHARACTERS (required)
- **Password Confirm**: Must exactly match password field
- **Name**: Required field, will be trimmed of whitespace
- **Mobile**: OPTIONAL - Leave empty if you don't have SMS/OTP configured

**⚠️ COMMON ISSUES:**
- "min and 0 more errors" = Password less than 8 characters
- "Decoding of secret as base32 failed" = OTP not configured, don't include mobile
- "required and 0 more errors" = Name field is missing

**Response:**
- Success notification
- Redirect to /auth/signin after 2 seconds
- User account created but needs to sign in

**Next Steps:**
1. Sign in with the created credentials
2. If first user, invoke 'become_an_administrator'
3. Configure system settings as needed`,
		
		"signin": `Authenticates a user and returns JWT tokens for API access.

**Prerequisites:**
- User must exist (created via signup)
- Account must not be locked
- Email/password combination must be correct

**Request Format:**
- **email**: The email used during signup (case-insensitive)
- **password**: The exact password (case-sensitive)

**Response includes:**
- **JWT Token**: Bearer token for API authentication
- **Cookie**: Same token set as HTTP-only cookie
- **Expiry**: Token valid for 3 days by default
- **User Info**: Embedded in JWT claims (name, email, user ID)

**How to use the token:**
- Header: "Authorization: Bearer YOUR_JWT_TOKEN"
- All subsequent API calls should include this header
- Token contains user identity and permissions

**Response Actions:**
1. client.store.set - Stores token in browser localStorage
2. client.cookie.set - Sets HTTP-only cookie
3. client.notify - Shows success message
4. client.redirect - Redirects to home after 2 seconds

**Security Features:**
- Passwords stored using bcrypt
- Rate limiting on failed attempts
- Account lockout after 5 failures
- JWT includes issuer validation`,
		
		"reset-password": "Initiates the password reset process. Sends a verification code to the user's registered email for password recovery. Codes expire after 15 minutes.",
		
		"reset-password-verify": "Completes password reset with verification code. Validates the reset code and sets a new password for the user account. Invalidates all existing sessions.",
		
		"oauth_login_begin": `Initiates OAuth authentication flow with supported providers.

**Supported Providers:**
- Google
- Facebook  
- GitHub
- Microsoft
- Custom OAuth2

**Flow:** Redirects to provider → User authorizes → Callback to oauth.login.response`,
		
		"oauth.login.response": "Handles OAuth provider callback. Processes the OAuth response and creates/updates user account with provider data. Merges accounts if email matches.",
		
		"upload_xls_to_system_schema": `Imports Excel data with automatic schema detection and table creation.

**Smart Detection:**
- Column types from data
- Header row identification
- Sheet selection
- Data type inference

**Options:**
- Create new table
- Update existing table
- Merge with existing data`,
		
		"upload_csv_to_system_schema": `Imports CSV data with intelligent parsing and schema creation.

**CSV Parsing:**
- Auto-detect delimiter
- Header detection
- Quote handling
- Encoding detection

**Data Types:** Automatically inferred from content analysis`,
		
		"add_exchange": `Configures automated data synchronization with external services.

**Exchange Types:**
- Google Sheets sync
- REST API webhooks
- Database replication
- File system sync

**Sync Options:**
- One-way or bidirectional
- Scheduled or real-time
- Conflict resolution rules`,
	}
	
	if desc, ok := descriptions[action.Name]; ok {
		return desc
	}
	return action.Label
}

func generateActionSchemaDescription(action actionresponse.Action) string {
	baseDesc := fmt.Sprintf("Request schema for the '%s' action. ", action.Label)
	
	// Count required fields
	requiredCount := 0
	for _, field := range action.InFields {
		if !field.IsNullable {
			requiredCount++
		}
	}
	
	if len(action.InFields) == 0 && action.InstanceOptional {
		return baseDesc + `This action requires no input parameters and can be executed without specifying an instance.

**Authentication:** ` + getAuthRequirement(action.Name) + `
**Rate Limiting:** Standard API rate limits apply
**Idempotent:** ` + getIdempotency(action.Name)
	} else if len(action.InFields) == 0 {
		return baseDesc + fmt.Sprintf(`This action requires only the reference ID of the %s instance on which to execute it.

**Authentication:** %s
**Instance Required:** Yes - must specify a valid %s_id
**Permissions:** Execute permission on the instance`, action.OnType, getAuthRequirement(action.Name), action.OnType)
	}
	
	return baseDesc + fmt.Sprintf(`This action operates on %s instances and requires %d input parameter(s) to execute successfully.

**Authentication:** %s
**Instance Required:** %s
**Required Fields:** %d of %d fields are mandatory
**Validation:** Input fields are validated before execution`, 
		action.OnType, 
		len(action.InFields),
		getAuthRequirement(action.Name),
		func() string {
			if action.InstanceOptional {
				return "No"
			}
			return "Yes"
		}(),
		requiredCount,
		len(action.InFields))
}

func getAuthRequirement(actionName string) string {
	publicActions := map[string]bool{
		"signin": true,
		"signup": true,
		"oauth_login_begin": true,
		"oauth.login.response": true,
		"reset-password": true,
		"reset-password-verify": true,
	}
	
	if publicActions[actionName] {
		return "Not required (public endpoint)"
	}
	
	if actionName == "become_an_administrator" {
		return "Required (Bearer token) - Can only be invoked when NO admin exists in the system"
	}
	
	if strings.Contains(actionName, "admin") {
		return "Required (Administrator role needed)"
	}
	
	return "Required (Bearer token)"
}

func getIdempotency(actionName string) string {
	idempotentActions := map[string]bool{
		"download_system_schema": true,
		"download_certificate": true,
		"download_public_key": true,
		"export_data": true,
		"export_csv_data": true,
		"list_files": true,
		"get_file": true,
	}
	
	if idempotentActions[actionName] {
		return "Yes - Safe to retry"
	}
	
	return "No - May have side effects"
}

func getFieldDescription(actionName, fieldName string) (string, bool) {
	fieldDescriptions := map[string]map[string]string{
		"import_files_from_store": {
			"table_name": `The name of the database table where file metadata will be imported. The table must:
- Already exist in the system
- Have at least one file-type column configured with cloud storage
- Have appropriate permissions for the current user

Example: "documents", "media_files", "attachments"`,
		},
		"generate_acme_certificate": {
			"email": `Contact email for Let's Encrypt notifications. This email will receive:
- Certificate expiration warnings (30 days before expiry)
- Renewal notifications
- Important security announcements

Must be a valid, monitored email address. Example: "admin@yourdomain.com"`,
		},
		"register_otp": {
			"mobile_number": `Mobile number for receiving OTP codes via SMS. Format requirements:
- Include country code (e.g., +1 for US, +44 for UK)
- No spaces or special characters except leading +
- Must be a valid mobile number (not landline)

Examples: "+12125551234", "+447700900123"`,
		},
		"verify_otp": {
			"otp": `The one-time password code received via SMS or email. Typically:
- 6 digits for SMS codes
- 8 characters for email codes
- Case-sensitive if alphanumeric
- Valid for 5 minutes from generation

Example: "123456"`,
			"mobile_number": `The mobile number where the OTP was sent. Must exactly match the registered number including country code.

Example: "+12125551234"`,
			"email": `Email address associated with the user account. Required for:
- Account verification
- Matching user identity
- Audit trail

Must be the primary email on the account.`,
		},
		"remove_column": {
			"column_name": `⚠️ DESTRUCTIVE: The exact name of the column to permanently delete.

**WARNING**: This will:
- Delete ALL data in the column
- Remove column from schema
- Break any dependent views/queries
- Cannot be undone!

System columns (id, created_at, etc.) cannot be removed.`,
			"world_name": `The table name containing the column to be removed. Case-sensitive.

Example: "user_profiles", "products"`,
		},
		"rename_column": {
			"table_name": `The table containing the column to rename. Must be an existing table that you have permission to modify.

Example: "products", "customer_data"`,
			"column_name": `Current name of the column. Must exactly match the existing column name (case-sensitive).

Example: "prod_desc", "customerEmail"`,
			"new_column_name": `New name for the column. Requirements:
- Cannot be a reserved SQL keyword
- Must be unique within the table
- Spaces will be converted to underscores
- Should follow naming conventions (lowercase, underscores)

Examples: "product_description", "customer_email_address"`,
		},
		"sync_site_storage": {
			"path": `Directory path to synchronize with cloud storage. Options:
- "/" - Sync entire site (may be slow)
- "/assets" - Sync only assets directory
- "/uploads/2024" - Sync specific subdirectory

Path must exist in the site structure.`,
		},
		"generate_random_data": {
			"count": `Number of random records to generate. Constraints:
- Minimum: 1
- Maximum: 10000 (for performance)
- Recommended: 100-1000 for testing

Larger values may take significant time.`,
			"table_name": `Target table for generating test data. The table must:
- Already exist in the system
- Have a defined schema
- Not have unique constraints that would conflict

Example: "test_users", "sample_products"`,
		},
		"export_data": {
			"table_name": `Name of the table to export. Leave empty to export all tables (requires admin permissions).

Examples: "users", "orders", "products"`,
			"format": `Output format for the exported data:
- "json" (default) - Structured JSON with relationships
- "csv" - Comma-separated values for spreadsheets
- "xlsx" - Excel format with formatting
- "xml" - Structured XML with schema
- "pdf" - Formatted report (limited to 1000 rows)
- "html" - HTML table for web viewing`,
			"columns": `Comma-separated list of columns to export. Features:
- Leave empty for all columns
- Use exact column names
- Order determines export order
- Can exclude sensitive columns

Example: "id,name,email,created_at"`,
			"include_headers": `Include column names as first row (CSV/Excel only):
- true (default) - First row contains column names
- false - Data only, no headers

Recommended: true for human readability`,
			"page_size": `Number of records to process per batch:
- Default: 1000
- Range: 100-10000
- Higher = faster but more memory
- Lower = less memory but slower

Adjust based on record size and available memory.`,
		},
		"import_data": {
			"dump_file": `File containing data to import. Supported formats:
- JSON (.json) - Single object or array
- CSV (.csv) - With or without headers
- Excel (.xlsx, .xls) - First sheet used
- YAML (.yml, .yaml) - Structured data
- TOML (.toml) - Configuration format
- HCL (.hcl) - HashiCorp configuration

File size limit: 100MB`,
			"truncate_before_insert": `⚠️ CAUTION: Delete existing data before import:
- true - Clear all existing records first
- false (default) - Append to existing data

WARNING: true will permanently delete all current data!`,
			"batch_size": `Records processed per database transaction:
- Default: 100
- Range: 10-1000
- Larger batches: Faster but risk losing more on error
- Smaller batches: Slower but safer

Adjust based on data complexity and reliability needs.`,
		},
		"upload_file": {
			"file": `File to upload (multipart/form-data). Specifications:
- Maximum size: Set by cloud provider (typically 5GB)
- Any file type supported
- Automatic MIME type detection
- Virus scanning if configured

Large files may take significant time.`,
			"path": `Destination path in cloud storage:
- Leave empty for root directory
- Must not start with /
- Creates directories if needed
- Overwrites existing files

Examples: "documents/2024/", "images/profile/"`,
		},
		"create_site": {
			"site_type": `Type of site to create:
- "static" - Plain HTML/CSS/JS files
- "hugo" - Hugo static site generator
- "jekyll" - Jekyll for GitHub Pages
- "gatsby" - Gatsby React framework
- "next" - Next.js application

Determines initial template and build process.`,
			"path": `Storage path for site files:
- Relative to cloud storage root
- Created if doesn't exist
- Must be unique per site
- Becomes site identifier

Example: "sites/blog", "websites/corporate"`,
			"hostname": `Domain name for the site:
- Can be subdomain: "blog.example.com"
- Or full domain: "example.com"
- Used for routing and SSL
- Must be DNS-configured

Wildcard SSL supported for subdomains.`,
		},
		"signup": {
			"name": `User's full name for display:
- Used in emails and UI
- Can contain spaces
- Unicode characters supported
- 2-100 characters

Examples: "John Doe", "María García"`,
			"email": `Primary email address:
- Used for login
- Must be unique in system
- Receives notifications
- Case-insensitive
- Valid email format required

Example: "user@example.com"`,
			"mobile": `Mobile number for SMS/OTP features:

**⚠️ IMPORTANT**: Only include if you have SMS/OTP configured!
- Will trigger OTP generation on signup
- Fails with "Decoding of secret as base32 failed" if OTP not set up
- RECOMMENDED: Leave empty for initial setup

**Format if used:**
- Include country code: "+1234567890"
- No spaces or special characters
- Will be trimmed automatically

**When to use:**
- After configuring SMS provider (Twilio, etc.)
- When two-factor auth is required
- For account recovery options`,
			"password": `Account password requirements:
- Minimum 8 characters
- Recommended: 12+ characters
- Mix of letters, numbers, symbols
- Not same as email
- Stored using bcrypt hashing`,
			"passwordConfirm": `Confirm password:
- Must exactly match password field
- Case-sensitive
- Prevents typos
- Required for signup`,
		},
		"signin": {
			"email": `Registered email address:
- Case-insensitive
- Primary account identifier
- Must be verified (if enabled)

Example: "user@example.com"`,
			"password": `Account password:
- Case-sensitive
- No minimum length for signin
- Rate limited after failures
- Account locked after 5 failures`,
		},
		"reset-password": {
			"email": `Email address of the account to reset:
- Must be registered
- Receives reset code
- Case-insensitive

Reset email sent if account exists.`,
		},
		"reset-password-verify": {
			"email": `Email address requesting reset:
- Must match reset request
- Case-insensitive`,
			"code": `Verification code from email:
- 6-8 characters
- Case-sensitive
- Valid for 15 minutes
- Single use only`,
			"password": `New password to set:
- Same requirements as signup
- Cannot be same as previous
- Invalidates all sessions`,
		},
		"add_exchange": {
			"name": `Unique identifier for this exchange:
- Used in logs and UI
- Cannot contain spaces
- Must be unique

Example: "google_sheets_sync"`,
			"sheet_id": `Google Sheets document ID:
- Found in sheet URL
- After /d/ and before /edit
- 44 characters

Example: "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms"`,
			"app_key": `API key or credentials:
- Provider-specific format
- Keep confidential
- Usually JSON for Google
- Stored encrypted`,
		},
	}
	
	if actionFields, ok := fieldDescriptions[actionName]; ok {
		if desc, ok := actionFields[fieldName]; ok {
			return desc, true
		}
	}
	
	return "", false
}

func generateActionExample(action actionresponse.Action) map[string]interface{} {
	examples := map[string]map[string]interface{}{
		"import_files_from_store": {
			"table_name": "documents",
		},
		"generate_acme_certificate": {
			"email": "admin@example.com",
		},
		"register_otp": {
			"mobile_number": "+1234567890",
		},
		"verify_otp": {
			"otp": "123456",
			"mobile_number": "+1234567890",
			"email": "user@example.com",
		},
		"remove_column": {
			"column_name": "deprecated_field",
		},
		"rename_column": {
			"table_name": "products",
			"column_name": "product_desc",
			"new_column_name": "product_description",
		},
		"generate_random_data": {
			"count": 100,
			"table_name": "test_users",
		},
		"export_data": {
			"table_name": "customers",
			"format": "csv",
			"columns": "name,email,created_at",
			"include_headers": true,
		},
		"signup": {
			"name": "John Doe",
			"email": "john.doe@example.com",
			"mobile": "+1234567890",
			"password": "SecurePass123!",
			"passwordConfirm": "SecurePass123!",
		},
		"signin": {
			"email": "john.doe@example.com",
			"password": "SecurePass123!",
		},
	}
	
	if example, ok := examples[action.Name]; ok {
		if !action.InstanceOptional {
			example[action.OnType+"_id"] = "550e8400-e29b-41d4-a716-446655440000"
		}
		return example
	}
	
	// Generate a basic example if not specifically defined
	basicExample := make(map[string]interface{})
	if !action.InstanceOptional {
		basicExample[action.OnType+"_id"] = "550e8400-e29b-41d4-a716-446655440000"
	}
	
	for _, field := range action.InFields {
		switch field.ColumnType {
		case "email":
			basicExample[field.ColumnName] = "user@example.com"
		case "label", "text":
			basicExample[field.ColumnName] = "example " + field.ColumnName
		case "measurement", "integer":
			basicExample[field.ColumnName] = 10
		case "truefalse", "boolean":
			basicExample[field.ColumnName] = true
		default:
			basicExample[field.ColumnName] = "example_value"
		}
	}
	
	return basicExample
}

func generateActionRequestExample(action actionresponse.Action) map[string]interface{} {
	return generateActionExample(action)
}

func generateActionResponseExample(action actionresponse.Action) []map[string]interface{} {
	// Generate response examples based on action type
	responseExamples := map[string][]map[string]interface{}{
		"signin": {
			{
				"ResponseType": "client.store.set",
				"Attributes": map[string]interface{}{
					"key": "token",
					"value": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6InVzZXJAZXhhbXBsZS5jb20iLCJleHAiOjE3NTUwMjk5NTksImlhdCI6MTc1MTQzMzU1OSwibmFtZSI6IkpvaG4gRG9lIiwic3ViIjoiMDE5MjQyMTItZGQ5NC03N2QzLTkyMzMtYjJiYmM1ZmNiZDQ2In0...",
				},
			},
			{
				"ResponseType": "client.cookie.set",
				"Attributes": map[string]interface{}{
					"key": "token",
					"value": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6InVzZXJAZXhhbXBsZS5jb20iLCJleHAiOjE3NTUwMjk5NTksImlhdCI6MTc1MTQzMzU1OSwibmFtZSI6IkpvaG4gRG9lIiwic3ViIjoiMDE5MjQyMTItZGQ5NC03N2QzLTkyMzMtYjJiYmM1ZmNiZDQ2In0...; SameSite=Strict",
				},
			},
			{
				"ResponseType": "client.notify",
				"Attributes": map[string]interface{}{
					"type": "success",
					"title": "Success",
					"message": "Logged in",
				},
			},
			{
				"ResponseType": "client.redirect",
				"Attributes": map[string]interface{}{
					"location": "/",
					"window": "self",
					"delay": 2000,
				},
			},
		},
		"signup": {
			{
				"ResponseType": "client.notify",
				"Attributes": map[string]interface{}{
					"type": "success",
					"title": "Success",
					"message": "Sign-up successful. Redirecting to sign in",
				},
			},
			{
				"ResponseType": "client.redirect",
				"Attributes": map[string]interface{}{
					"location": "/auth/signin",
					"delay": 2000,
				},
			},
		},
		"verify_otp": {
			{
				"ResponseType": "client.store.set",
				"Attributes": map[string]interface{}{
					"key": "token",
					"value": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
				},
			},
			{
				"ResponseType": "client.notify",
				"Attributes": map[string]interface{}{
					"type": "success",
					"title": "Verified",
					"message": "OTP verification successful",
				},
			},
		},
		"download_certificate": {
			{
				"ResponseType": "client.file.download",
				"Attributes": map[string]interface{}{
					"name": "example.com.pem.crt",
					"content": "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUZhekNDQTFPZ0F3SUJBZ0lVT...",
					"contentType": "application/x-x509-ca-cert",
					"message": "Certificate for example.com",
				},
			},
		},
		"download_system_schema": {
			{
				"ResponseType": "client.file.download",
				"Attributes": map[string]interface{}{
					"name": "daptin_schema.json",
					"content": "ewogICJUYWJsZXMiOiBbCiAgICB7CiAgICAgICJUYWJsZU5hbWUiOiAidXNlcl9hY2NvdW50IiwKICAgICAgIkNvbHVtbnMiOiBbLi4uXQogICAgfQogIF0KfQ==",
					"contentType": "application/json",
					"message": "System schema export",
				},
			},
		},
		"export_data": {
			{
				"ResponseType": "client.file.download",
				"Attributes": map[string]interface{}{
					"name": "daptin_export_customers.csv",
					"content": "aWQsbmFtZSxlbWFpbCxjcmVhdGVkX2F0CjEsSm9obiBEb2Usam9obkBleGFtcGxlLmNvbSwyMDI0LTAxLTE1CjIsSmFuZSBTbWl0aCxqYW5lQGV4YW1wbGUuY29tLDIwMjQtMDEtMTY=",
					"contentType": "text/csv",
					"message": "Downloading data as csv",
				},
			},
		},
		"export_csv_data": {
			{
				"ResponseType": "client.file.download",
				"Attributes": map[string]interface{}{
					"name": "export.csv",
					"content": "aWQsbmFtZSxlbWFpbA0KMSxKb2huIERvZSxqb2huQGV4YW1wbGUuY29t",
					"contentType": "text/csv",
					"message": "CSV export completed",
				},
			},
		},
		"import_files_from_store": {
			{
				"ResponseType": "client.notify",
				"Attributes": map[string]interface{}{
					"type": "success",
					"title": "Import Complete",
					"message": "Imported success 25 files, failed 0 files",
				},
			},
		},
		"generate_random_data": {
			{
				"ResponseType": "client.notify",
				"Attributes": map[string]interface{}{
					"type": "success",
					"title": "Success",
					"message": "Created 100 rows in test_users",
				},
			},
		},
		"remove_column": {
			{
				"ResponseType": "client.notify",
				"Attributes": map[string]interface{}{
					"type": "success",
					"title": "Success",
					"message": "Column deleted",
				},
			},
		},
		"rename_column": {
			{
				"ResponseType": "client.notify",
				"Attributes": map[string]interface{}{
					"type": "success",
					"title": "Success",
					"message": "Column renamed",
				},
			},
		},
		"restart_daptin": {
			{
				"ResponseType": "client.notify",
				"Attributes": map[string]interface{}{
					"type": "success",
					"title": "Success",
					"message": "Initiating system update.",
				},
			},
			{
				"ResponseType": "client.redirect",
				"Attributes": map[string]interface{}{
					"location": "/",
					"window": "self",
					"delay": 5000,
				},
			},
		},
		"become_an_administrator": {
			{
				"ResponseType": "client.notify",
				"Attributes": map[string]interface{}{
					"type": "success",
					"title": "Congratulations",
					"message": "You are now an administrator",
				},
			},
		},
		"reset-password": {
			{
				"ResponseType": "client.notify",
				"Attributes": map[string]interface{}{
					"type": "info",
					"title": "Email Sent",
					"message": "If the email exists, a reset code has been sent",
				},
			},
		},
		"upload_file": {
			{
				"ResponseType": "client.notify",
				"Attributes": map[string]interface{}{
					"type": "success",
					"title": "Upload Complete",
					"message": "File uploaded successfully",
				},
			},
		},
		"create_site": {
			{
				"ResponseType": "client.notify",
				"Attributes": map[string]interface{}{
					"type": "success",
					"title": "Site Created",
					"message": "New site created at example.com",
				},
			},
			{
				"ResponseType": "client.redirect",
				"Attributes": map[string]interface{}{
					"location": "/sites",
					"delay": 2000,
				},
			},
		},
		"generate_acme_certificate": {
			{
				"ResponseType": "client.notify",
				"Attributes": map[string]interface{}{
					"type": "success",
					"title": "Certificate Generated",
					"message": "Let's Encrypt certificate generated and installed",
				},
			},
		},
		"sync_site_storage": {
			{
				"ResponseType": "client.notify",
				"Attributes": map[string]interface{}{
					"type": "success",
					"title": "Sync Complete",
					"message": "Site storage synchronized: 15 files uploaded, 3 files downloaded",
				},
			},
		},
	}
	
	if examples, ok := responseExamples[action.Name]; ok {
		return examples
	}
	
	// Default response example
	return []map[string]interface{}{
		{
			"ResponseType": "client.notify",
			"Attributes": map[string]interface{}{
				"type": "success",
				"title": "Action Completed",
				"message": fmt.Sprintf("%s action executed successfully", action.Label),
			},
		},
	}
}

func generateCurlExample(action actionresponse.Action) string {
	example := generateActionExample(action)
	exampleJSON, _ := json.Marshal(example)
	
	return fmt.Sprintf(`curl -X POST \\
  https://your-daptin-instance.com/action/%s/%s \\
  -H 'Authorization: Bearer YOUR_JWT_TOKEN' \\
  -H 'Content-Type: application/json' \\
  -d '%s'`, 
		action.OnType, 
		action.Name, 
		string(exampleJSON))
}

func generateJavaScriptExample(action actionresponse.Action) string {
	example := generateActionExample(action)
	exampleJSON, _ := json.Marshal(example)
	
	return fmt.Sprintf(`const response = await fetch('https://your-daptin-instance.com/action/%s/%s', {
  method: 'POST',
  headers: {
    'Authorization': 'Bearer YOUR_JWT_TOKEN',
    'Content-Type': 'application/json'
  },
  body: JSON.stringify(%s)
});

const result = await response.json();
console.log(result);`, 
		action.OnType, 
		action.Name, 
		string(exampleJSON))
}

func generateActionSecurityInfo(action actionresponse.Action) []map[string][]string {
	// Most actions require authentication
	publicActions := map[string]bool{
		"signin": true,
		"signup": true,
		"oauth_login_begin": true,
		"oauth.login.response": true,
		"reset-password": true,
		"reset-password-verify": true,
	}
	
	if publicActions[action.Name] {
		return []map[string][]string{} // No security required
	}
	
	return []map[string][]string{
		{"bearerAuth": []string{}},
	}
}

func generateActionErrorExample(action actionresponse.Action) []map[string]interface{} {
	errorExamples := map[string][]map[string]interface{}{
		"signin": {
			{
				"ResponseType": "client.notify",
				"Attributes": map[string]interface{}{
					"type": "error",
					"title": "Authentication Failed",
					"message": "Invalid email or password",
				},
			},
		},
		"signup": {
			{
				"ResponseType": "client.notify",
				"Attributes": map[string]interface{}{
					"type": "error",
					"title": "Registration Failed",
					"message": "Email already exists",
				},
			},
		},
		"generate_random_data": {
			{
				"ResponseType": "client.notify",
				"Attributes": map[string]interface{}{
					"type": "error",
					"title": "failed",
					"message": "table not found",
				},
			},
		},
		"remove_column": {
			{
				"ResponseType": "client.notify",
				"Attributes": map[string]interface{}{
					"type": "error",
					"title": "Error",
					"message": "no such column",
				},
			},
		},
		"rename_column": {
			{
				"ResponseType": "client.notify",
				"Attributes": map[string]interface{}{
					"type": "error",
					"title": "Error",
					"message": "new_column_name is a reserved word",
				},
			},
		},
		"import_files_from_store": {
			{
				"ResponseType": "client.notify",
				"Attributes": map[string]interface{}{
					"type": "error",
					"title": "Import Failed",
					"message": "invalid table",
				},
			},
		},
		"verify_otp": {
			{
				"ResponseType": "client.notify",
				"Attributes": map[string]interface{}{
					"type": "error",
					"title": "Verification Failed",
					"message": "Invalid or expired OTP",
				},
			},
		},
		"export_data": {
			{
				"ResponseType": "client.notify",
				"Attributes": map[string]interface{}{
					"type": "error",
					"title": "Export Failed",
					"message": "Table not found or access denied",
				},
			},
		},
		"upload_file": {
			{
				"ResponseType": "client.notify",
				"Attributes": map[string]interface{}{
					"type": "error",
					"title": "Upload Failed",
					"message": "File too large or invalid format",
				},
			},
		},
		"generate_acme_certificate": {
			{
				"ResponseType": "client.notify",
				"Attributes": map[string]interface{}{
					"type": "error",
					"title": "Certificate Generation Failed",
					"message": "Domain validation failed or rate limit exceeded",
				},
			},
		},
	}
	
	if examples, ok := errorExamples[action.Name]; ok {
		return examples
	}
	
	// Default error example
	return []map[string]interface{}{
		{
			"ResponseType": "client.notify",
			"Attributes": map[string]interface{}{
				"type": "error",
				"title": "Action Failed",
				"message": "An error occurred while executing the action",
			},
		},
	}
}
