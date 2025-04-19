package actions

import (
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

type selfTlsCertificateGenerateActionPerformer struct {
	responseAttrs      map[string]interface{}
	cruds              map[string]*resource.DbResource
	configStore        *resource.ConfigStore
	encryptionSecret   []byte
	certificateManager *resource.CertificateManager
}

func (d *selfTlsCertificateGenerateActionPerformer) Name() string {
	return "self.tls.generate"
}

func (d *selfTlsCertificateGenerateActionPerformer) DoAction(request actionresponse.Outcome, inFieldMap map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	certificateSubject := inFieldMap["certificate"].(map[string]interface{})
	log.Printf("Generate certificate for: %v", certificateSubject)

	hostname := certificateSubject["hostname"].(string)
	cert, err := d.certificateManager.GetTLSConfig(hostname, true, transaction)
	if err != nil {
		return nil, []actionresponse.ActionResponse{}, []error{err}
	}

	log.Printf("Cert generated: %v ", cert.CertPEM)

	return nil, []actionresponse.ActionResponse{
		resource.NewActionResponse("client.notify", resource.NewClientNotification("message", "Certificate generated for "+hostname, "Success")),
	}, nil
}

func NewSelfTlsCertificateGenerateActionPerformer(cruds map[string]*resource.DbResource, configStore *resource.ConfigStore, certificateManager *resource.CertificateManager, transaction *sqlx.Tx) (actionresponse.ActionPerformerInterface, error) {

	encryptionSecret, _ := configStore.GetConfigValueFor("encryption.secret", "backend", transaction)

	handler := selfTlsCertificateGenerateActionPerformer{
		cruds:              cruds,
		encryptionSecret:   []byte(encryptionSecret),
		configStore:        configStore,
		certificateManager: certificateManager,
	}

	return &handler, nil

}
