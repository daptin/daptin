package resource

import (
	"errors"
	"github.com/artpar/api2go"
	"github.com/flysnow-org/soha"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"html/template"
	"strings"
)

type renderTemplateActionPerformer struct {
	responseAttrs    map[string]interface{}
	cruds            map[string]*DbResource
	configStore      *ConfigStore
	encryptionSecret []byte
}

func (actionPerformer *renderTemplateActionPerformer) Name() string {
	return "template.render"
}

func (actionPerformer *renderTemplateActionPerformer) DoAction(
	request Outcome, inFieldMap map[string]interface{},
	transaction *sqlx.Tx) (api2go.Responder, []ActionResponse, []error) {

	template_name, ok := inFieldMap["template"].(string)
	if !ok {
		return nil, []ActionResponse{}, []error{errors.New("no template found")}
	}
	templateInstance, err := actionPerformer.cruds["template"].GetObjectByWhereClauseWithTransaction(
		"template", "name", template_name, transaction)
	if err != nil {
		return nil, []ActionResponse{}, []error{err}
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
				return nil, []ActionResponse{}, []error{err}
			}
		}
	}
	if !ok {
		return nil, []ActionResponse{}, []error{errors.New("no template content found")}
	}

	sohaFuncMap := soha.CreateFuncMap()

	tmpl, err := template.New(template_name).Funcs(sohaFuncMap).Parse(templateContent)
	if err != nil {
		log.Errorf("Failed to parse tempalte [%s]: %s", template_name, err)
		return nil, []ActionResponse{}, []error{err}
	}
	var buf strings.Builder

	err = tmpl.Execute(&buf, inFieldMap)

	resp := &api2go.Response{}

	responder := api2go.NewApi2GoModelWithData("render",
		nil, 0, nil, map[string]interface{}{
			"content":   Btoa([]byte(buf.String())),
			"mime_type": templateMimeType,
			"headers":   headers,
		})
	resp.Res = responder

	return resp, []ActionResponse{}, nil
}

func NewRenderTemplateActionPerformer(cruds map[string]*DbResource, configStore *ConfigStore, transaction *sqlx.Tx) (ActionPerformerInterface, error) {

	encryptionSecret, _ := configStore.GetConfigValueFor("encryption.secret", "backend", transaction)

	handler := renderTemplateActionPerformer{
		cruds:            cruds,
		encryptionSecret: []byte(encryptionSecret),
		configStore:      configStore,
	}

	return &handler, nil

}
