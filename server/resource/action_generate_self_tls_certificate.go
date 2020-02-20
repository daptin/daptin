package resource

import (
	"github.com/artpar/api2go"
	"log"
)

type SelfTlsCertificateGenerateActionPerformer struct {
	responseAttrs      map[string]interface{}
	cruds              map[string]*DbResource
	configStore        *ConfigStore
	encryptionSecret   []byte
	certificateManager *CertificateManager
}

func (d *SelfTlsCertificateGenerateActionPerformer) Name() string {
	return "self.tls.generate"
}

func (d *SelfTlsCertificateGenerateActionPerformer) DoAction(request Outcome, inFieldMap map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {
	certificateSubject := inFieldMap["certificate"].(map[string]interface{})
	log.Printf("Generate certificate for: %v", certificateSubject)

	hostname := certificateSubject["hostname"].(string)
	_, certPem, _, _, _, err := d.certificateManager.GetTLSConfig(hostname, true)
	if err != nil {
		return nil, []ActionResponse{}, []error{err}
	}

	log.Printf("Cert generated: %v ", certPem)

	return nil, []ActionResponse{
		NewActionResponse("client.notify", NewClientNotification("message", "Certificate generated for "+hostname, "Success")),
	}, nil
}

func NewSelfTlsCertificateGenerateActionPerformer(cruds map[string]*DbResource, configStore *ConfigStore, certificateManager *CertificateManager) (ActionPerformerInterface, error) {

	encryptionSecret, _ := configStore.GetConfigValueFor("encryption.secret", "backend")

	handler := SelfTlsCertificateGenerateActionPerformer{
		cruds:              cruds,
		encryptionSecret:   []byte(encryptionSecret),
		configStore:        configStore,
		certificateManager: certificateManager,
	}

	return &handler, nil

}
