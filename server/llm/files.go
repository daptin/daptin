package llm

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/artpar/api2go/v2"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/llmgateway/contract"
	"github.com/daptin/llmgateway/protocol/openai"
	"github.com/jmoiron/sqlx"
)

type daptinFiles struct {
	cruds map[string]*resource.DbResource
}

func (store daptinFiles) Create(ctx context.Context, _ contract.Principal, request contract.CreateFileRequest) (contract.File, error) {
	if _, err := daptinSessionUser(ctx); err != nil {
		return contract.File{}, err
	}
	transaction, err := store.cruds["document"].Connection().Beginx()
	if err != nil {
		return contract.File{}, fmt.Errorf("begin file create: %w", err)
	}
	defer transaction.Rollback()
	file, err := store.create(ctx, request, transaction)
	if err != nil {
		return contract.File{}, err
	}
	if err := transaction.Commit(); err != nil {
		return contract.File{}, fmt.Errorf("commit file create: %w", err)
	}
	return file, nil
}

func (store daptinFiles) create(ctx context.Context, request contract.CreateFileRequest, transaction *sqlx.Tx) (contract.File, error) {
	contentColumn, configured := store.cruds["document"].TableInfo().GetColumnByName("document_content")
	if !configured || contentColumn.ForeignKeyData.DataSource != "cloud_store" ||
		store.cruds["document"].AssetFolderCache["document"]["document_content"] == nil {
		return contract.File{}, &contract.Error{Code: contract.ErrorUnavailable,
			Message: "document asset storage is not configured", HTTPStatus: http.StatusServiceUnavailable}
	}
	if request.Filename == "" || len(request.Data) == 0 || !request.Purpose.Valid() {
		return contract.File{}, &contract.Error{Code: contract.ErrorInvalidRequest, Message: "file and valid purpose are required", HTTPStatus: http.StatusBadRequest}
	}
	contentType := strings.TrimSpace(request.ContentType)
	if contentType == "" {
		contentType = http.DetectContentType(request.Data)
	}
	createdAt := time.Now().UTC()
	var expiresAt *time.Time
	if request.ExpiresAfter != nil {
		if request.ExpiresAfter.Anchor != "created_at" || request.ExpiresAfter.Seconds < 3600 || request.ExpiresAfter.Seconds > 2592000 {
			return contract.File{}, &contract.Error{Code: contract.ErrorInvalidRequest, Message: "invalid file expiration", HTTPStatus: http.StatusBadRequest}
		}
		value := createdAt.Add(time.Duration(request.ExpiresAfter.Seconds) * time.Second)
		expiresAt = &value
	}

	documentURL, _ := url.Parse("/document")
	apiRequest := api2go.Request{PlainRequest: (&http.Request{Method: http.MethodPost, URL: documentURL}).WithContext(ctx)}
	document, err := store.cruds["document"].CreateWithoutFilter(api2go.NewApi2GoModelWithData("document", nil, 0, nil, map[string]interface{}{
		"document_name": request.Filename, "document_path": "", "document_extension": strings.TrimPrefix(filepath.Ext(request.Filename), "."),
		"mime_type": contentType, "document_content": []interface{}{map[string]interface{}{
			"name": request.Filename, "path": "", "type": contentType, "contents": base64.StdEncoding.EncodeToString(request.Data),
		}},
	}), apiRequest, transaction)
	if err != nil {
		return contract.File{}, fmt.Errorf("create file document: %w", err)
	}
	documentReference := daptinid.InterfaceToDIR(document["reference_id"])
	if documentReference == daptinid.NullReferenceId {
		return contract.File{}, fmt.Errorf("created file document has no reference")
	}
	attributes := map[string]interface{}{
		"document_id": documentReference.String(), "purpose": string(request.Purpose), "status": "processed",
	}
	if expiresAt != nil {
		attributes["expires_at"] = *expiresAt
	}
	created, err := store.cruds["llm_file"].CreateWithoutFilter(
		api2go.NewApi2GoModelWithData("llm_file", nil, 0, nil, attributes), apiRequest, transaction,
	)
	if err != nil {
		return contract.File{}, fmt.Errorf("create LLM file metadata: %w", err)
	}
	return contract.File{ID: contract.ID(daptinid.InterfaceToDIR(created["reference_id"]).String()), Bytes: int64(len(request.Data)),
		CreatedAt: createdAt, Filename: request.Filename, ContentType: contentType, Purpose: request.Purpose,
		Status: "processed", ExpiresAt: expiresAt}, nil
}

func (store daptinFiles) List(ctx context.Context, _ contract.Principal, request contract.ListFilesRequest) (contract.FilePage, error) {
	if _, err := daptinSessionUser(ctx); err != nil {
		return contract.FilePage{}, err
	}
	transaction, err := store.cruds["llm_file"].Connection().Beginx()
	if err != nil {
		return contract.FilePage{}, fmt.Errorf("begin file list: %w", err)
	}
	defer transaction.Rollback()
	query := url.Values{"page[size]": {strconv.Itoa(request.Limit + 1)}, "sort": {"-created_at"}, "included_relations": {"document"}}
	if request.Order == "asc" {
		query["sort"] = []string{"+created_at"}
	}
	if request.After != "" {
		if daptinid.InterfaceToDIR(string(request.After)) == daptinid.NullReferenceId {
			return contract.FilePage{}, &contract.Error{Code: contract.ErrorInvalidRequest, Message: "invalid file cursor", HTTPStatus: http.StatusBadRequest}
		}
		cursor := "page[before]"
		if request.Order == "asc" {
			cursor = "page[after]"
		}
		query[cursor] = []string{string(request.After)}
	}
	if request.Purpose != "" {
		encoded, marshalErr := json.Marshal([]resource.Query{{ColumnName: "purpose", Operator: "is", Value: string(request.Purpose)}})
		if marshalErr != nil {
			return contract.FilePage{}, fmt.Errorf("encode file purpose filter: %w", marshalErr)
		}
		query["query"] = []string{string(encoded)}
	}
	listURL, _ := url.Parse("/llm_file?" + query.Encode())
	apiRequest := api2go.Request{PlainRequest: (&http.Request{Method: http.MethodGet, URL: listURL}).WithContext(ctx)}
	apiRequest.QueryParams = query
	_, responder, err := store.cruds["llm_file"].PaginatedFindAllWithTransaction(apiRequest, transaction)
	if err != nil {
		return contract.FilePage{}, fmt.Errorf("list LLM files: %w", err)
	}
	models, ok := responder.Result().([]api2go.Api2GoModel)
	if !ok {
		return contract.FilePage{}, fmt.Errorf("LLM file list returned [%T]", responder.Result())
	}
	hasMore := len(models) > request.Limit
	if hasMore {
		models = models[:request.Limit]
	}
	files := make([]contract.File, 0, len(models))
	for index := range models {
		document, found := includedDocument(models[index])
		if !found {
			return contract.FilePage{}, fmt.Errorf("LLM file %s has no document", models[index].GetID())
		}
		file, conversionErr := daptinFile(models[index].GetAllAsAttributes(), document)
		if conversionErr != nil {
			return contract.FilePage{}, conversionErr
		}
		files = append(files, file)
	}
	if err := transaction.Commit(); err != nil {
		return contract.FilePage{}, fmt.Errorf("commit file list: %w", err)
	}
	return contract.FilePage{Data: files, HasMore: hasMore}, nil
}

func (store daptinFiles) Get(ctx context.Context, _ contract.Principal, id contract.ID) (contract.File, error) {
	transaction, err := store.cruds["llm_file"].Connection().Beginx()
	if err != nil {
		return contract.File{}, fmt.Errorf("begin file read: %w", err)
	}
	defer transaction.Rollback()
	row, document, err := store.load(ctx, id, http.MethodGet, transaction)
	if err != nil {
		return contract.File{}, err
	}
	file, err := daptinFile(row, document)
	if err != nil {
		return contract.File{}, err
	}
	if err := transaction.Commit(); err != nil {
		return contract.File{}, fmt.Errorf("commit file read: %w", err)
	}
	return file, nil
}

func (store daptinFiles) Content(ctx context.Context, _ contract.Principal, id contract.ID) (contract.FileContent, error) {
	transaction, err := store.cruds["llm_file"].Connection().Beginx()
	if err != nil {
		return contract.FileContent{}, fmt.Errorf("begin file content read: %w", err)
	}
	defer transaction.Rollback()
	_, document, err := store.load(ctx, id, http.MethodGet, transaction)
	if err != nil {
		return contract.FileContent{}, err
	}
	metadata, err := documentFile(document)
	if err != nil {
		return contract.FileContent{}, err
	}
	if err := transaction.Commit(); err != nil {
		return contract.FileContent{}, fmt.Errorf("commit file metadata read: %w", err)
	}
	resolved, err := store.cruds["document"].GetFileFromLocalCloudStore("document", "document_content", []map[string]interface{}{metadata})
	if err != nil || len(resolved) != 1 {
		return contract.FileContent{}, fmt.Errorf("read stored file content: %w", err)
	}
	contents := resource.StringOrEmpty(resolved[0]["contents"])
	data, err := base64.StdEncoding.DecodeString(contents)
	if err != nil || len(data) == 0 {
		return contract.FileContent{}, fmt.Errorf("decode stored file content: %w", err)
	}
	return contract.FileContent{Filename: resource.StringOrEmpty(document["document_name"]),
		ContentType: resource.StringOrEmpty(document["mime_type"]), Data: data}, nil
}

func (store daptinFiles) Delete(ctx context.Context, _ contract.Principal, id contract.ID) error {
	transaction, err := store.cruds["llm_file"].Connection().Beginx()
	if err != nil {
		return fmt.Errorf("begin file delete: %w", err)
	}
	defer transaction.Rollback()
	row, _, err := store.load(ctx, id, http.MethodDelete, transaction)
	if err != nil {
		return err
	}
	fileReference := daptinid.InterfaceToDIR(row["reference_id"])
	documentReference := daptinid.InterfaceToDIR(row["document_id"])
	deleteURL, _ := url.Parse("/llm_file/" + fileReference.String())
	apiRequest := api2go.Request{PlainRequest: (&http.Request{Method: http.MethodDelete, URL: deleteURL}).WithContext(ctx)}
	if err := store.cruds["llm_file"].DeleteWithoutFilters(fileReference, apiRequest, transaction); err != nil {
		return fmt.Errorf("delete LLM file metadata: %w", err)
	}
	apiRequest.PlainRequest.URL.Path = "/document/" + documentReference.String()
	if err := store.cruds["document"].DeleteWithoutFilters(documentReference, apiRequest, transaction); err != nil {
		return fmt.Errorf("delete file document: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit file delete: %w", err)
	}
	return nil
}

func (store daptinFiles) load(ctx context.Context, id contract.ID, method string, transaction *sqlx.Tx) (map[string]interface{}, map[string]interface{}, error) {
	reference := daptinid.InterfaceToDIR(string(id))
	if reference == daptinid.NullReferenceId {
		return nil, nil, fileNotFound()
	}
	query := url.Values{"included_relations": {"document"}}
	readURL, _ := url.Parse("/llm_file/" + reference.String() + "?" + query.Encode())
	apiRequest := api2go.Request{PlainRequest: (&http.Request{Method: method, URL: readURL}).WithContext(ctx), QueryParams: query}
	responder, err := store.cruds["llm_file"].FindOneWithTransaction(reference, apiRequest, transaction)
	if err != nil {
		if status, ok := err.(interface{ Status() int }); ok && (status.Status() == http.StatusForbidden || status.Status() == http.StatusNotFound) {
			return nil, nil, fileNotFound()
		}
		return nil, nil, fmt.Errorf("load LLM file: %w", err)
	}
	model, ok := responder.Result().(api2go.Api2GoModel)
	if !ok || model.GetAllAsAttributes() == nil {
		return nil, nil, fileNotFound()
	}
	document, found := includedDocument(model)
	if !found {
		return nil, nil, fmt.Errorf("file document is unavailable")
	}
	return model.GetAllAsAttributes(), document, nil
}

func includedDocument(model api2go.Api2GoModel) (map[string]interface{}, bool) {
	for _, included := range model.GetReferencedStructs() {
		attributes := included.GetAttributes()
		if resource.StringOrEmpty(attributes["__type"]) == "document" {
			return attributes, true
		}
	}
	return nil, false
}

func daptinFile(row, document map[string]interface{}) (contract.File, error) {
	createdAt, ok := row["created_at"].(time.Time)
	if !ok || createdAt.IsZero() {
		return contract.File{}, fmt.Errorf("LLM file has invalid creation time")
	}
	metadata, err := documentFile(document)
	if err != nil {
		return contract.File{}, err
	}
	bytes, err := resource.ResourceRowInt64(metadata["size"])
	if err != nil || bytes < 0 {
		return contract.File{}, fmt.Errorf("LLM file has invalid byte size")
	}
	purpose := contract.FilePurpose(resource.StringOrEmpty(row["purpose"]))
	if !purpose.Valid() {
		return contract.File{}, fmt.Errorf("LLM file has invalid purpose")
	}
	var expiresAt *time.Time
	if row["expires_at"] != nil {
		value, valid := row["expires_at"].(time.Time)
		if !valid {
			return contract.File{}, fmt.Errorf("LLM file has invalid expiration")
		}
		expiresAt = &value
	}
	return contract.File{ID: contract.ID(daptinid.InterfaceToDIR(row["reference_id"]).String()), Bytes: bytes, CreatedAt: createdAt,
		Filename: resource.StringOrEmpty(document["document_name"]), ContentType: resource.StringOrEmpty(document["mime_type"]),
		Purpose: purpose, Status: resource.StringOrEmpty(row["status"]), StatusDetails: resource.StringOrEmpty(row["status_details"]), ExpiresAt: expiresAt}, nil
}

func documentFile(document map[string]interface{}) (map[string]interface{}, error) {
	files, ok := document["document_content"].([]map[string]interface{})
	if !ok || len(files) != 1 {
		return nil, fmt.Errorf("file document has invalid content metadata")
	}
	return files[0], nil
}

func fileNotFound() *contract.Error {
	return &contract.Error{Code: contract.ErrorPermission, Message: "file not found", HTTPStatus: http.StatusNotFound}
}

var _ openai.FileStore = daptinFiles{}
