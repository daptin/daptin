package resource

import (
	stdjson "encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/artpar/api2go/v2"
	log "github.com/sirupsen/logrus"
)

const invalidJSONValueMessage = "invalid JSON value"

// jsonColumnStorageValue converts a public JSON value into the database-backed
// representation used by Daptin resources. Pre-serialized values are decoded
// and encoded again so malformed or double-encoded input cannot reach SQL.
func jsonColumnStorageValue(value interface{}) (interface{}, error) {
	if value == nil {
		return nil, nil
	}

	var encoded []byte
	var err error
	switch typed := value.(type) {
	case string:
		encoded = []byte(typed)
	case []byte:
		encoded = typed
	case stdjson.RawMessage:
		encoded = typed
	default:
		encoded, err = stdjson.Marshal(value)
		if err != nil {
			return nil, err
		}
	}

	var decoded interface{}
	if err := stdjson.Unmarshal(encoded, &decoded); err != nil {
		return nil, err
	}
	if _, object := decoded.(map[string]interface{}); !object {
		if _, array := decoded.([]interface{}); !array {
			return nil, fmt.Errorf("expected a JSON object or array")
		}
	}

	encoded, err = stdjson.Marshal(decoded)
	if err != nil {
		return nil, err
	}
	return string(encoded), nil
}

func invalidJSONColumnError(columnName string, err error) api2go.HTTPError {
	message := fmt.Sprintf("%s for %s", invalidJSONValueMessage, columnName)
	log.Errorf("Invalid JSON column value for %s: %v", columnName, err)
	httpErr := api2go.NewHTTPError(nil, message, http.StatusUnprocessableEntity)
	httpErr.Errors = []api2go.Error{{
		Status: strconv.Itoa(http.StatusUnprocessableEntity),
		Title:  message,
	}}
	return httpErr
}

// publicJSONColumnData copies a resource row and decodes JSON columns for
// public REST, GraphQL, and action responses. Internal database consumers keep
// using the encoded durable representation.
func publicJSONColumnData(data map[string]interface{}, columns []api2go.ColumnInfo) (map[string]interface{}, error) {
	if data == nil {
		return nil, nil
	}

	result := make(map[string]interface{}, len(data))
	for key, value := range data {
		result[key] = value
	}

	for _, column := range columns {
		if column.ColumnType != "json" {
			continue
		}
		value, ok := result[column.ColumnName]
		if !ok || value == nil {
			continue
		}

		switch typed := value.(type) {
		case string:
			var decoded interface{}
			if err := stdjson.Unmarshal([]byte(typed), &decoded); err != nil {
				return nil, fmt.Errorf("decode stored JSON column %s: %w", column.ColumnName, err)
			}
			result[column.ColumnName] = decoded
		case []byte:
			var decoded interface{}
			if err := stdjson.Unmarshal(typed, &decoded); err != nil {
				return nil, fmt.Errorf("decode stored JSON column %s: %w", column.ColumnName, err)
			}
			result[column.ColumnName] = decoded
		}
	}

	return result, nil
}

func newPublicApi2GoModel(
	typeName string,
	columns []api2go.ColumnInfo,
	defaultPermission int64,
	relations []api2go.TableRelation,
	data map[string]interface{},
) (api2go.Api2GoModel, error) {
	publicData, err := publicJSONColumnData(data, columns)
	if err != nil {
		return api2go.Api2GoModel{}, err
	}
	return api2go.NewApi2GoModelWithData(typeName, columns, defaultPermission, relations, publicData), nil
}
