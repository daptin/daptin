package resource

import (
	"encoding/base64"
	"errors"
	"path/filepath"
	"strings"
)

func (dbResource *DbResource) binaryColumnValueForStorage(tableName, columnName string, content []byte, name, contentType string) interface{} {
	return dbResource.binaryColumnValue(tableName, columnName, content, binaryStorageFileName(name, content), contentType)
}

func (dbResource *DbResource) binaryColumnValue(tableName, columnName string, content []byte, name, contentType string) interface{} {
	encoded := base64.StdEncoding.EncodeToString(content)
	tableResource := dbResource.Cruds[tableName]
	if tableResource == nil || tableResource.TableInfo() == nil {
		return encoded
	}
	column, ok := tableResource.TableInfo().GetColumnByName(columnName)
	if !ok || column == nil || !column.IsForeignKey || column.ForeignKeyData.DataSource != "cloud_store" {
		return encoded
	}

	return []interface{}{
		map[string]interface{}{
			"name":     name,
			"path":     "",
			"type":     contentType,
			"contents": encoded,
		},
	}
}

func (dbResource *DbResource) binaryColumnBytes(tableName, columnName string, columnValue interface{}) ([]byte, error) {
	tableResource := dbResource.Cruds[tableName]
	if tableResource != nil && tableResource.TableInfo() != nil {
		column, ok := tableResource.TableInfo().GetColumnByName(columnName)
		if ok && column != nil && column.IsForeignKey && column.ForeignKeyData.DataSource == "cloud_store" {
			switch value := columnValue.(type) {
			case []map[string]interface{}:
				return binaryFileContents(value)
			case []interface{}:
				files := make([]map[string]interface{}, 0, len(value))
				for _, file := range value {
					fileMap, ok := file.(map[string]interface{})
					if !ok {
						return nil, errors.New("binary file metadata is invalid")
					}
					files = append(files, fileMap)
				}
				return binaryFileContents(files)
			case string:
				return base64.StdEncoding.DecodeString(value)
			case []byte:
				return base64.StdEncoding.DecodeString(string(value))
			default:
				return nil, errors.New("binary file contents are not included")
			}
		}
	}

	switch value := columnValue.(type) {
	case string:
		return base64.StdEncoding.DecodeString(value)
	case []byte:
		return base64.StdEncoding.DecodeString(string(value))
	default:
		return nil, errors.New("binary column has unsupported value")
	}
}

func binaryFileContents(files []map[string]interface{}) ([]byte, error) {
	if len(files) == 0 {
		return nil, errors.New("binary file list is empty")
	}
	contents, ok := files[0]["contents"].(string)
	if !ok {
		return nil, errors.New("binary file contents are not included")
	}
	return base64.StdEncoding.DecodeString(contents)
}

func binaryStorageFileName(nameHint string, content []byte) string {
	name := filepath.Base(strings.TrimSpace(nameHint))
	extension := filepath.Ext(name)
	storageKey := strings.TrimSpace(nameHint)
	if storageKey == "" {
		storageKey = GetMD5Hash(content)
	}
	return GetMD5Hash([]byte(storageKey)) + extension
}
