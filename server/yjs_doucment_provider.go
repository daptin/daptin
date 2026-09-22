package server

import (
	"bytes"
	"context"
	"encoding/base64"
	stdjson "encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/artpar/api2go/v2"
	"github.com/artpar/ydb"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
)

const yjsStateMediaType = resource.YjsStateMediaType

type daptinYjsStore struct {
	standalone ydb.Store
	cruds      map[string]*resource.DbResource
}

type resourceYjsTarget struct {
	typeName   string
	reference  daptinid.DaptinReferenceId
	column     string
	columnInfo api2go.ColumnInfo
	crud       *resource.DbResource
}

func yjsStateEntry(files []map[string]interface{}) (map[string]interface{}, error) {
	var stateFile map[string]interface{}
	for _, file := range files {
		if file["type"] != yjsStateMediaType {
			continue
		}
		if stateFile != nil {
			return nil, fmt.Errorf("multiple %s entries", yjsStateMediaType)
		}
		stateFile = file
	}
	return stateFile, nil
}

func decodeYjsState(contents string) ([]byte, error) {
	if contents == "" || strings.HasPrefix(strings.ToLower(contents), "data:") || strings.Contains(contents, ",") {
		return nil, fmt.Errorf("invalid %s contents", yjsStateMediaType)
	}
	state, err := base64.StdEncoding.Strict().DecodeString(contents)
	if err != nil {
		return nil, fmt.Errorf("decode %s contents: %w", yjsStateMediaType, err)
	}
	return state, nil
}

func withYjsState(files []map[string]interface{}, column string, state []byte) []interface{} {
	updated := make([]interface{}, 0, len(files)+1)
	for _, file := range files {
		if file["type"] != yjsStateMediaType {
			updated = append(updated, file)
		}
	}
	return append(updated, map[string]interface{}{
		"name":     column + ".yjs",
		"path":     "",
		"type":     yjsStateMediaType,
		"contents": base64.StdEncoding.EncodeToString(state),
	})
}

func yjsColumnFiles(value interface{}) ([]map[string]interface{}, error) {
	switch files := value.(type) {
	case nil:
		return nil, nil
	case string:
		var decoded []map[string]interface{}
		if err := stdjson.Unmarshal([]byte(files), &decoded); err != nil {
			return nil, err
		}
		return decoded, nil
	case []map[string]interface{}:
		return files, nil
	case []interface{}:
		decoded := make([]map[string]interface{}, 0, len(files))
		for _, item := range files {
			file, ok := item.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid YJS asset entry %T", item)
			}
			decoded = append(decoded, file)
		}
		return decoded, nil
	default:
		return nil, fmt.Errorf("invalid YJS asset column value %T", value)
	}
}

func (s *daptinYjsStore) target(room ydb.YjsRoomName) (*resourceYjsTarget, bool, error) {
	parts, canonical := canonicalYjsRoomParts(string(room))
	if !canonical {
		return nil, false, nil
	}
	crud := s.cruds[parts[0]]
	if crud == nil {
		return nil, true, fmt.Errorf("YJS resource type %q does not exist", parts[0])
	}
	column, ok := crud.TableInfo().GetColumnByName(parts[2])
	if !ok || !BeginsWithCheck(column.ColumnType, "file.") {
		return nil, true, fmt.Errorf("YJS resource column %q.%q is not a file column", parts[0], parts[2])
	}
	parsed, err := uuid.Parse(parts[1])
	if err != nil {
		return nil, true, err
	}
	return &resourceYjsTarget{
		typeName:   parts[0],
		reference:  daptinid.DaptinReferenceId(parsed),
		column:     column.ColumnName,
		columnInfo: *column,
		crud:       crud,
	}, true, nil
}

func (s *daptinYjsStore) resourceState(target *resourceYjsTarget) (map[string]interface{}, []map[string]interface{}, []byte, error) {
	tx, err := target.crud.Connection().Beginx()
	if err != nil {
		return nil, nil, nil, err
	}
	row, _, err := target.crud.GetSingleRowByReferenceIdWithTransaction(target.typeName, target.reference,
		map[string]bool{resource.YjsStateMediaType: true}, tx)
	if err != nil {
		_ = tx.Rollback()
		return nil, nil, nil, err
	}
	files, err := yjsColumnFiles(row[target.column])
	if err != nil {
		_ = tx.Rollback()
		return nil, nil, nil, err
	}
	_ = tx.Rollback()

	stateFile, err := yjsStateEntry(files)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("%w in %s.%s", err, target.typeName, target.column)
	}
	if stateFile == nil {
		return row, files, nil, nil
	}

	contents, _ := stateFile["contents"].(string)
	state, err := decodeYjsState(contents)
	if err != nil {
		return nil, nil, nil, err
	}
	return row, files, state, nil
}

func (s *daptinYjsStore) ReadFrom(room ydb.YjsRoomName, offset uint32) ([]byte, uint32, error) {
	target, canonical, err := s.target(room)
	if err != nil {
		return nil, 0, err
	}
	if !canonical {
		return s.standalone.ReadFrom(room, offset)
	}
	_, _, state, err := s.resourceState(target)
	if err != nil {
		return nil, 0, err
	}
	size := uint32(len(state))
	if offset >= size {
		return nil, size, nil
	}
	return bytes.Clone(state[offset:]), size, nil
}

func (s *daptinYjsStore) Size(room ydb.YjsRoomName) (uint32, error) {
	target, canonical, err := s.target(room)
	if err != nil {
		return 0, err
	}
	if !canonical {
		return s.standalone.Size(room)
	}
	_, _, state, err := s.resourceState(target)
	return uint32(len(state)), err
}

func (s *daptinYjsStore) Append(ctx context.Context, room ydb.YjsRoomName, data []byte) (uint32, error) {
	target, canonical, err := s.target(room)
	if err != nil {
		return 0, err
	}
	if !canonical {
		return s.standalone.Append(ctx, room, data)
	}
	if user, ok := ctx.Value("user").(*auth.SessionUser); !ok || user == nil {
		return 0, errors.New("authenticated YJS user is missing")
	}

	const maxVersionAttempts = 4
	for attempt := 0; attempt < maxVersionAttempts; attempt++ {
		row, files, state, readErr := s.resourceState(target)
		if readErr != nil {
			return 0, readErr
		}
		state = append(state, data...)
		updatedFiles := withYjsState(files, target.column, state)

		var columnValue interface{} = updatedFiles
		if !target.columnInfo.IsForeignKey || target.columnInfo.ForeignKeyData.DataSource != "cloud_store" {
			encodedFiles, encodeErr := stdjson.Marshal(updatedFiles)
			if encodeErr != nil {
				return 0, encodeErr
			}
			columnValue = string(encodedFiles)
		}

		model := api2go.NewApi2GoModelWithData(target.typeName, target.crud.TableInfo().Columns,
			int64(target.crud.TableInfo().DefaultPermission), target.crud.TableInfo().Relations, row)
		model.SetAttributes(map[string]interface{}{target.column: columnValue})
		_ = model.SetID(target.reference.String())
		requestURL := &url.URL{Path: "/api/" + target.typeName + "/" + target.reference.String()}
		request := api2go.Request{PlainRequest: (&http.Request{Method: http.MethodPatch, URL: requestURL}).WithContext(ctx)}
		if _, updateErr := target.crud.Update(model, request); updateErr == nil {
			return uint32(len(state)), nil
		} else if !errors.Is(updateErr, resource.ErrVersionConflict) {
			return 0, updateErr
		}
	}
	return 0, fmt.Errorf("%w: exhausted YJS update retries", resource.ErrVersionConflict)
}

func (s *daptinYjsStore) SetInitialContent(room ydb.YjsRoomName, data []byte) error {
	_, canonical, err := s.target(room)
	if err != nil {
		return err
	}
	if !canonical {
		return s.standalone.SetInitialContent(room, data)
	}
	return errors.New("resource-backed YJS content can only be changed by an authenticated update")
}

// PathExistsAndIsFolder checks if a path exists and is a folder
func PathExistsAndIsFolder(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false // Path does not exist
	}
	if err != nil {
		return false // Other errors
	}
	return info.IsDir() // Check if it's a directory
}

func CreateYjsStore(configStore *resource.ConfigStore, transaction *sqlx.Tx, localStoragePath string, cruds map[string]*resource.DbResource) ydb.Store {
	logrus.Infof("YJS endpoint is enabled in config")
	yjsDir := localStoragePath + "/yjs-documents"
	configStore.SetConfigValueFor("yjs.storage.path", yjsDir, "backend", transaction)

	if !PathExistsAndIsFolder(yjsDir) {
		err := os.MkdirAll(yjsDir, 0777)
		if err != nil {
			resource.CheckErr(err, "Failed to create yjs storage directory")
		}
	}

	standalone := ydb.NewDiskStore(yjsDir, ydb.WithMaxRoomSize(50*1024*1024))
	return &daptinYjsStore{standalone: standalone, cruds: cruds}
}
