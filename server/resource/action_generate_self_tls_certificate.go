package resource

import (
	"github.com/artpar/api2go"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

type selfTlsCertificateGenerateActionPerformer struct {
	responseAttrs      map[string]interface{}
	cruds              map[string]*DbResource
	configStore        *ConfigStore
	encryptionSecret   []byte
	certificateManager *CertificateManager
}

func (d *selfTlsCertificateGenerateActionPerformer) Name() string {
	return "self.tls.generate"
}

func (d *selfTlsCertificateGenerateActionPerformer) DoAction(request Outcome, inFieldMap map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []ActionResponse, []error) {
	certificateSubject := inFieldMap["certificate"].(map[string]interface{})
	log.Printf("Generate certificate for: %v", certificateSubject)

	hostname := certificateSubject["hostname"].(string)
	_, certPem, _, _, _, err := d.certificateManager.GetTLSConfig(hostname, true, transaction)
	if err != nil {
		return nil, []ActionResponse{}, []error{err}
	}

	log.Printf("Cert generated: %v ", certPem)

	return nil, []ActionResponse{
		NewActionResponse("client.notify", NewClientNotification("message", "Certificate generated for "+hostname, "Success")),
	}, nil
}

func NewSelfTlsCertificateGenerateActionPerformer(cruds map[string]*DbResource, configStore *ConfigStore, certificateManager *CertificateManager, transaction *sqlx.Tx) (ActionPerformerInterface, error) {

	encryptionSecret, _ := configStore.GetConfigValueFor("encryption.secret", "backend", transaction)

	handler := selfTlsCertificateGenerateActionPerformer{
		cruds:              cruds,
		encryptionSecret:   []byte(encryptionSecret),
		configStore:        configStore,
		certificateManager: certificateManager,
	}

	return &handler, nil

}
