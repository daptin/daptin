package apiblueprint

import (
	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/table_info"
	"strings"
	"testing"
)

func TestBuildApiBlueprint(t *testing.T) {
	// Initialize ColumnManager for testing
	resource.InitialiseColumnManager()
	
	// Create a test config with sample tables
	config := &resource.CmsConfig{
		Hostname: "localhost:6336",
		Tables: []table_info.TableInfo{
			{
				TableName:        "blog_post",
				TableDescription: "Blog posts with comments and tags",
				Columns: []api2go.ColumnInfo{
					{
						Name:              "Title",
						ColumnName:        "title",
						ColumnDescription: "The title of the blog post",
						ColumnType:        "label",
						DataType:          "varchar(255)",
						IsNullable:        false,
					},
					{
						Name:              "Content",
						ColumnName:        "content",
						ColumnDescription: "The main content of the blog post",
						ColumnType:        "text",
						DataType:          "text",
						IsNullable:        false,
					},
					{
						Name:              "Author Email",
						ColumnName:        "author_email",
						ColumnDescription: "Email address of the post author",
						ColumnType:        "email",
						DataType:          "varchar(255)",
						IsNullable:        false,
					},
					{
						Name:              "Published Date",
						ColumnName:        "published_date",
						ColumnDescription: "Date when the post was published",
						ColumnType:        "date",
						DataType:          "date",
						IsNullable:        true,
					},
					{
						Name:              "Status",
						ColumnName:        "status",
						ColumnDescription: "Publication status of the post",
						ColumnType:        "label",
						DataType:          "varchar(50)",
						IsNullable:        false,
						Options: []api2go.ValueOptions{
							{Value: "draft", Label: "Draft"},
							{Value: "published", Label: "Published"},
							{Value: "archived", Label: "Archived"},
						},
					},
				},
				Relations: []api2go.TableRelation{},
				Permission:        auth.DEFAULT_PERMISSION,
				DefaultPermission: auth.DEFAULT_PERMISSION,
			},
		},
		Actions: []actionresponse.Action{
			{
				Name:             "publish",
				Label:            "Publish Blog Post",
				OnType:           "blog_post",
				InstanceOptional: false,
				InFields: []api2go.ColumnInfo{
					{
						Name:              "Publish Date",
						ColumnName:        "publish_date",
						ColumnDescription: "Date to publish the post",
						ColumnType:        "datetime",
						DataType:          "datetime",
					},
				},
			},
		},
	}

	// Generate the OpenAPI spec
	spec := BuildApiBlueprint(config, nil)

	// Verify the spec contains expected elements
	tests := []struct {
		name     string
		contains string
	}{
		{"OpenAPI version", "openapi: 3.0.0"},
		{"API title", "title: Daptin API endpoint"},
		{"Authentication description", "JWT Bearer token authentication"},
		{"Error response schema", "ErrorResponse"},
		{"Rate limit response", "RateLimitResponse"},
		{"Common parameters", "PageNumber"},
		{"Query parameter", "Full-text search across all indexed text columns"},
		{"Filter parameter", "JSON-based filtering"},
		{"Field descriptions", "The title of the blog post"},
		{"Email format", "format: email"},
		{"Date format", "format: date"},
		{"Enum values", "enum:"},
		{"Status options", "draft"},
		{"Example values", "example:"},
		{"Relationship docs", "Relationship object following JSON:API specification"},
		{"Multiple formats", "text/csv"},
		{"Rate limiting info", "Rate Limiting"},
		{"Tags section", "tags:"},
		{"External docs", "externalDocs:"},
		{"Security schemes", "bearerAuth:"},
		{"Blog post tag", "name: blog_post"},
		{"Action endpoint", "/action/blog_post/publish"},
		{"Request example", "data:"},
		{"Pagination status", "PaginationStatus"},
		{"Bad request response", "BadRequest"},
		{"Unauthorized response", "Unauthorized"},
		{"Too many requests", "TooManyRequests"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(spec, tt.contains) {
				t.Errorf("Expected spec to contain '%s', but it was not found", tt.contains)
				// Print a portion of the spec for debugging
				if len(spec) > 500 {
					t.Logf("Spec excerpt: %s...", spec[:500])
				}
			}
		})
	}

	// Verify spec is valid YAML
	if !strings.HasPrefix(spec, "openapi:") {
		t.Error("Generated spec does not start with 'openapi:', may not be valid YAML")
	}

	t.Logf("Generated OpenAPI spec length: %d characters", len(spec))
}

func TestSystemActionsDocumentation(t *testing.T) {
	// Initialize ColumnManager for testing
	resource.InitialiseColumnManager()
	
	// Create a config with system actions
	config := &resource.CmsConfig{
		Hostname: "localhost:6336",
		Tables: []table_info.TableInfo{
			{
				TableName: "world",
				TableDescription: "System configuration table",
				Columns: resource.StandardColumns,
			},
			{
				TableName: "user_account",
				TableDescription: "User accounts table",
				Columns: resource.StandardColumns,
			},
		},
		Actions: resource.SystemActions[:10], // Test with first 10 system actions
	}

	// Generate the OpenAPI spec
	spec := BuildApiBlueprint(config, nil)

	// Test for enhanced documentation features
	tests := []struct {
		name     string
		contains string
	}{
		// Test for action categorization tags
		{"System Actions tag", "System Actions"},
		{"Data Operations tag", "Data Operations"},
		{"Schema Management tag", "Schema Management"},
		{"Storage Management tag", "Storage Management"},
		{"Certificate Management tag", "Certificate Management"},
		{"User Management tag", "User Management"},
		
		// Test for detailed action descriptions
		{"Import files description", "bulk import files stored in cloud storage"},
		{"Install integration description", "third-party integration"},
		{"Certificate download description", "SSL/TLS certificate in PEM format"},
		
		// Test for enhanced field descriptions
		{"Table name field description", "name of the database table"},
		{"Email field description for ACME", "Contact email address for Let's Encrypt"},
		
		// Test for action examples
		{"Action example", "example"},
		{"cURL example", "curl -X POST"},
		
		// Test for response examples
		{"Response example", "ResponseType"},
		{"Action response schema", "ActionResponse"},
		
		// Test for comprehensive error responses
		{"400 error reference", "BadRequest"},
		{"401 error reference", "Unauthorized"},
		{"403 error reference", "Forbidden"},
		{"422 error reference", "UnprocessableEntity"},
		{"429 error reference", "TooManyRequests"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(spec, tt.contains) {
				t.Errorf("Expected spec to contain '%s', but it was not found", tt.contains)
			}
		})
	}

	// Test specific action paths are documented
	actionPaths := []string{
		"/action/world/import_files_from_store",
		"/action/integration/install_integration",
		"/action/certificate/download_certificate",
	}

	for _, path := range actionPaths {
		if !strings.Contains(spec, path) {
			t.Errorf("Missing system action path: %s", path)
		}
	}

	t.Logf("System actions documentation test completed. Spec length: %d characters", len(spec))
}

func TestActionHelperFunctions(t *testing.T) {
	// Test action categorization
	testCases := []struct {
		actionName string
		expected   string
	}{
		{"export_data", "Data Operations"},
		{"remove_column", "Schema Management"},
		{"upload_file", "Storage Management"},
		{"generate_acme_certificate", "Certificate Management"},
		{"signup", "User Management"},
		{"restart_daptin", "System Actions"},
		{"unknown_action", ""},
	}

	for _, tc := range testCases {
		result := categorizeAction(tc.actionName)
		if result != tc.expected {
			t.Errorf("categorizeAction(%s) = %s, expected %s", tc.actionName, result, tc.expected)
		}
	}

	// Test action description generation
	testAction := actionresponse.Action{
		Name:  "export_data",
		Label: "Export data for backup",
	}
	desc := generateActionDescription(testAction)
	if !strings.Contains(desc, "various formats") {
		t.Errorf("Action description missing expected content: %s", desc)
	}

	// Test field description lookup
	if desc, ok := getFieldDescription("export_data", "table_name"); !ok || desc == "" {
		t.Error("Failed to get field description for export_data.table_name")
	}
}