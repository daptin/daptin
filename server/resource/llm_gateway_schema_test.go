package resource

import (
	"testing"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/table_info"
)

func TestLLMGatewayUsesCanonicalResourcesAndRelations(t *testing.T) {
	requiredColumns := map[string][]string{
		"llm_provider":   {"name", "provider_type", "base_url", "provider_parameters", "allow_insecure", "allow_private_network", "enable"},
		"llm_model":      {"name", "operations", "capabilities", "routing_strategy", "fallback_models", "default_parameters", "unsupported_parameter_policy", "enable"},
		"llm_deployment": {"name", "upstream_model", "operations", "priority", "weight", "request_timeout_ms", "connect_timeout_ms", "max_concurrency", "rpm", "tpm", "pricing", "parameters", "health_check", "enable"},
		"llm_file":       {"purpose", "status", "status_details", "expires_at"},
		"llm_batch":      {"endpoint", "completion_window", "status", "metadata", "errors", "request_counts", "output_expiration_seconds", "claim_expires_at"},
		"llm_batch_item": {"line_number", "custom_id", "body", "status", "response_status", "response_request_id", "response_body"},
		"api_plan":       {"name", "limits"},
		"api_usage":      {"request_id", "reservation_token", "state", "reservation_expires_at", "terminal_at", "reserved_measures", "reservation_buckets", "measures"},
		"api_quota":      {"bucket_key", "metric", "window_start", "window_end", "maximum", "reserved", "consumed"},
	}
	for tableName, columns := range requiredColumns {
		var tableFound bool
		for index := range StandardTables {
			if StandardTables[index].TableName != tableName {
				continue
			}
			tableFound = true
			for _, columnName := range columns {
				if _, ok := StandardTables[index].GetColumnByName(columnName); !ok {
					t.Errorf("%s.%s is missing from StandardTables", tableName, columnName)
				}
			}
			break
		}
		if !tableFound {
			t.Errorf("%s is missing from StandardTables", tableName)
		}
	}

	requiredRelations := map[string]bool{
		"llm_provider|has_one|credential|credential_id":          false,
		"llm_deployment|belongs_to|llm_model|llm_model_id":       false,
		"llm_deployment|belongs_to|llm_provider|llm_provider_id": false,
		"llm_file|belongs_to|document|document_id":               false,
		"llm_batch|belongs_to|llm_file|input_file_id":            false,
		"llm_batch|has_one|llm_file|output_file_id":              false,
		"llm_batch_item|belongs_to|llm_batch|llm_batch_id":       false,
	}
	for index := range StandardRelations {
		relation := &StandardRelations[index]
		key := relation.GetSubject() + "|" + relation.GetRelation() + "|" + relation.GetObject() + "|" + relation.GetObjectName()
		if _, required := requiredRelations[key]; required {
			requiredRelations[key] = true
		}
	}
	for relation, found := range requiredRelations {
		if !found {
			t.Errorf("%s is missing from StandardRelations", relation)
		}
	}
}

func TestLLMInvocationUsesResourceMeteringConfiguration(t *testing.T) {
	model := tableFromConfig(t, &CmsConfig{Tables: StandardTables}, "llm_model")
	config := MeteringConfigForAction(model.Metering, "invoke")
	if config == nil || !config.Enabled || config.MeterType != "requests" || config.CostExpr != "1" {
		t.Fatalf("llm_model invoke metering is not configured through TableInfo: %#v", config)
	}
	jsonSchema := tableFromConfig(t, &CmsConfig{Tables: StandardTables}, "json_schema")
	if jsonSchema.Metering != nil {
		t.Fatalf("unrelated json_schema resource acquired LLM metering: %#v", jsonSchema.Metering)
	}
}

func TestLLMGatewayRelationsUseDaptinOwnershipAndRequiredForeignKeys(t *testing.T) {
	config := CmsConfig{Tables: standardTablesForTest(nil)}
	CheckRelations(&config)

	provider := tableFromConfig(t, &config, "llm_provider")
	credential := columnFromTable(t, provider, "credential_id")
	if !credential.IsForeignKey || !credential.IsNullable || credential.ForeignKeyData.Namespace != "credential" {
		t.Fatalf("provider credential must use the optional canonical relation: %#v", credential)
	}

	deployment := tableFromConfig(t, &config, "llm_deployment")
	for columnName, namespace := range map[string]string{"llm_model_id": "llm_model", "llm_provider_id": "llm_provider"} {
		column := columnFromTable(t, deployment, columnName)
		if !column.IsForeignKey || column.IsNullable || column.ForeignKeyData.Namespace != namespace {
			t.Fatalf("deployment relation %s must be required: %#v", columnName, column)
		}
	}

	for _, tableName := range []string{"llm_provider", "llm_model", "llm_deployment", "api_plan", "api_usage", "api_quota"} {
		owner := columnFromTable(t, tableFromConfig(t, &config, tableName), "user_account_id")
		if !owner.IsForeignKey || owner.ForeignKeyData.Namespace != "user_account" {
			t.Fatalf("%s ownership was not supplied by CheckRelations: %#v", tableName, owner)
		}
	}

	file := tableFromConfig(t, &config, "llm_file")
	document := columnFromTable(t, file, "document_id")
	if !document.IsForeignKey || document.IsNullable || document.ForeignKeyData.Namespace != "document" {
		t.Fatalf("llm_file must use the required canonical document relation: %#v", document)
	}
	batch := tableFromConfig(t, &config, "llm_batch")
	for columnName, nullable := range map[string]bool{"input_file_id": false, "output_file_id": true} {
		column := columnFromTable(t, batch, columnName)
		if !column.IsForeignKey || column.IsNullable != nullable || column.ForeignKeyData.Namespace != "llm_file" {
			t.Fatalf("batch file relation %s is invalid: %#v", columnName, column)
		}
	}
	itemBatch := columnFromTable(t, tableFromConfig(t, &config, "llm_batch_item"), "llm_batch_id")
	if !itemBatch.IsForeignKey || itemBatch.IsNullable || itemBatch.ForeignKeyData.Namespace != "llm_batch" {
		t.Fatalf("batch item must use the required canonical batch relation: %#v", itemBatch)
	}
}

func standardTablesForTest(included map[string]bool) []table_info.TableInfo {
	tables := make([]table_info.TableInfo, 0, len(StandardTables))
	for _, table := range StandardTables {
		if included != nil && !included[table.TableName] {
			continue
		}
		copyOfTable := table
		copyOfTable.Columns = append([]api2go.ColumnInfo(nil), table.Columns...)
		copyOfTable.Relations = append([]api2go.TableRelation(nil), table.Relations...)
		tables = append(tables, copyOfTable)
	}
	return tables
}

func tableFromConfig(t *testing.T, config *CmsConfig, name string) *table_info.TableInfo {
	t.Helper()
	for index := range config.Tables {
		if config.Tables[index].TableName == name {
			return &config.Tables[index]
		}
	}
	t.Fatalf("table %s not found", name)
	return nil
}

func columnFromTable(t *testing.T, table *table_info.TableInfo, name string) api2go.ColumnInfo {
	t.Helper()
	for _, column := range table.Columns {
		if column.ColumnName == name {
			return column
		}
	}
	t.Fatalf("column %s.%s not found", table.TableName, name)
	return api2go.ColumnInfo{}
}
