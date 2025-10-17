package actions

import (
	"encoding/base64"
	"errors"
	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/assetcachepojo"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/flysnow-org/soha"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"html/template"
	"io/ioutil"
	"os"
	"strings"
)

type renderTemplateActionPerformer struct {
	responseAttrs    map[string]interface{}
	cruds            map[string]*resource.DbResource
	configStore      *resource.ConfigStore
	encryptionSecret []byte
}

func (actionPerformer *renderTemplateActionPerformer) Name() string {
	return "template.render"
}

func (actionPerformer *renderTemplateActionPerformer) DoAction(
	request actionresponse.Outcome, inFieldMap map[string]interface{},
	transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	template_name, ok := inFieldMap["template"].(string)
	if !ok {
		return nil, []actionresponse.ActionResponse{}, []error{errors.New("no template found")}
	}
	templateInstance, err := actionPerformer.cruds["template"].GetObjectByWhereClauseWithTransaction(
		"template", "name", template_name, transaction)
	if err != nil {
		return nil, []actionresponse.ActionResponse{}, []error{err}
	}
	templateContent, ok := templateInstance["content"].(string)
	templateMimeType, ok := templateInstance["mime_type"].(string)
	headersString, headersOk := templateInstance["headers"].(string)
	var headers = make(map[string]string)
	if headersOk {
		if len(headersString) > 0 {
			err = json.Unmarshal([]byte(headersString), &headers)
			if err != nil {
				log.Errorf("Failed to unmarshal headers from `%s`", headersString)
				return nil, []actionresponse.ActionResponse{}, []error{err}
			}
		}
	}
	if !ok {
		return nil, []actionresponse.ActionResponse{}, []error{errors.New("no template content found")}
	}

	// Check if templateContent is base64 encoded and decode it
	decodedContent, err := base64.StdEncoding.DecodeString(templateContent)
	if err == nil {
		// Successfully decoded, use the decoded content
		templateContent = string(decodedContent)
	}
	// If decoding fails, assume it's not base64 and use as-is

	// Check if the template content is a reference to a subsite file
	if strings.HasPrefix(templateContent, "subsite://") {
		// Parse the subsite path: "subsite://<site_reference_id>/path/to/file"
		pathParts := strings.SplitN(strings.TrimPrefix(templateContent, "subsite://"), "/", 2)
		if len(pathParts) != 2 {
			return nil, []actionresponse.ActionResponse{}, []error{errors.New("invalid subsite file path format, expected: subsite://<site_reference_id>/path/to/file")}
		}

		siteReferenceIdStr := pathParts[0]
		filePath := pathParts[1]

		// Convert site reference ID string to DaptinReferenceId
		siteReferenceId := daptinid.InterfaceToDIR(siteReferenceIdStr)
		if siteReferenceId == daptinid.NullReferenceId {
			return nil, []actionresponse.ActionResponse{}, []error{errors.New("invalid site reference ID: " + siteReferenceIdStr)}
		}

		// Get the subsite folder cache
		assetFolderCache, ok := actionPerformer.cruds["template"].SubsiteFolderCache(siteReferenceId)
		if !ok {
			return nil, []actionresponse.ActionResponse{}, []error{errors.New("subsite not found for reference ID: " + siteReferenceIdStr)}
		}

		// Load the file content from the subsite
		fileContent, err := loadFileFromSubsite(assetFolderCache, filePath)
		if err != nil {
			return nil, []actionresponse.ActionResponse{}, []error{errors.New("failed to load file from subsite: " + err.Error())}
		}

		// Use the file content as the template content
		templateContent = fileContent
	}

	// Check if the template content is a reference to a site file
	if strings.HasPrefix(templateContent, "site://") {
		// Parse the site path: "site://<site_reference_id>/path/to/file"
		pathParts := strings.SplitN(strings.TrimPrefix(templateContent, "site://"), "/", 2)
		if len(pathParts) != 2 {
			return nil, []actionresponse.ActionResponse{}, []error{errors.New("invalid site file path format, expected: site://<site_reference_id>/path/to/file")}
		}

		siteReferenceIdStr := pathParts[0]
		filePath := pathParts[1]

		// Convert site reference ID string to DaptinReferenceId
		siteReferenceId := daptinid.InterfaceToDIR(siteReferenceIdStr)
		if siteReferenceId == daptinid.NullReferenceId {
			return nil, []actionresponse.ActionResponse{}, []error{errors.New("invalid site reference ID: " + siteReferenceIdStr)}
		}

		// Get the site folder cache
		assetFolderCache, ok := actionPerformer.cruds["template"].SubsiteFolderCache(siteReferenceId)
		if !ok {
			return nil, []actionresponse.ActionResponse{}, []error{errors.New("site not found for reference ID: " + siteReferenceIdStr)}
		}

		// Load the file content from the site
		fileContent, err := loadFileFromSubsite(assetFolderCache, filePath)
		if err != nil {
			return nil, []actionresponse.ActionResponse{}, []error{errors.New("failed to load file from site: " + err.Error())}
		}

		// Use the file content as the template content
		templateContent = fileContent
	}

	sohaFuncMap := soha.CreateFuncMap()

	tmpl, err := template.New(template_name).Funcs(sohaFuncMap).Parse(templateContent)
	if err != nil {
		log.Errorf("Failed to parse tempalte [%s]: %s", template_name, err)
		return nil, []actionresponse.ActionResponse{}, []error{err}
	}
	var buf strings.Builder

	err = tmpl.Execute(&buf, inFieldMap)
	if err != nil {
		log.Errorf("Failed to execute tempalte [%s]: %s", template_name, err)
		return nil, []actionresponse.ActionResponse{}, []error{err}
	}

	resp := &api2go.Response{}

	responder := api2go.NewApi2GoModelWithData("render",
		nil, 0, nil, map[string]interface{}{
			"content":   resource.Btoa([]byte(buf.String())),
			"mime_type": templateMimeType,
			"headers":   headers,
		})
	resp.Res = responder

	return resp, []actionresponse.ActionResponse{}, nil
}

// loadFileFromSubsite loads a file from a subsite's local sync path
func loadFileFromSubsite(assetFolderCache *assetcachepojo.AssetFolderCache, filePath string) (string, error) {
	// Construct the full path to the file
	fullPath := assetFolderCache.LocalSyncPath + string(os.PathSeparator) + filePath

	// Check if the file exists
	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		return "", err
	}

	// Check if it's a regular file
	if fileInfo.IsDir() {
		return "", errors.New("path is a directory, not a file")
	}

	// Read the file content
	fileBytes, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return "", err
	}

	return string(fileBytes), nil
}

func NewRenderTemplateActionPerformer(cruds map[string]*resource.DbResource, configStore *resource.ConfigStore, transaction *sqlx.Tx) (actionresponse.ActionPerformerInterface, error) {

	encryptionSecret, _ := configStore.GetConfigValueFor("encryption.secret", "backend", transaction)

	handler := renderTemplateActionPerformer{
		cruds:            cruds,
		encryptionSecret: []byte(encryptionSecret),
		configStore:      configStore,
	}

	return &handler, nil

}
