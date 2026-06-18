package actions

import (
	"errors"
	"strconv"

	"github.com/artpar/api2go/v2"
	"github.com/artpar/go-guerrilla"
	"github.com/artpar/go-guerrilla/backends"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

type mailServersSyncActionPerformer struct {
	cruds              map[string]*resource.DbResource
	mailDaemon         *guerrilla.Daemon
	certificateManager *resource.CertificateManager
}

func (d *mailServersSyncActionPerformer) Name() string {
	return "mail.servers.sync"
}

func (d *mailServersSyncActionPerformer) DoAction(request actionresponse.Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	responses := make([]actionresponse.ActionResponse, 0)

	if d.mailDaemon == nil || d.mailDaemon.Backend == nil {
		err := errors.New("mail daemon was not initialized; restart Daptin to start SMTP listeners")
		log.Debug(err.Error())
		return nil, responses, []error{err}
	}

	servers, err := d.cruds["mail_server"].GetAllObjects("mail_server", transaction)
	if err != nil {
		return nil, responses, []error{err}
	}

	serverConfig, hosts, err := resource.BuildSMTPServerConfigs(servers, d.certificateManager, transaction)
	if err != nil {
		return nil, responses, []error{err}
	}

	saveWorkersSize := 1
	if sws, err := d.cruds["mail"].ConfigStore.GetConfigValueFor("mail.save_workers_size", "backend", transaction); err == nil && sws != "" {
		if parsed, err := strconv.Atoi(sws); err == nil && parsed > 0 {
			saveWorkersSize = parsed
		}
	}

	primaryMailHost := "localhost"
	for _, srv := range serverConfig {
		if srv.IsEnabled && srv.Hostname != "" {
			primaryMailHost = srv.Hostname
			break
		}
	}

	err = d.mailDaemon.ReloadConfig(guerrilla.AppConfig{
		Servers:      serverConfig,
		AllowedHosts: hosts,
		BackendConfig: backends.BackendConfig{
			"save_process":       "HeadersParser|Debugger|Hasher|Header|Compressor|DaptinSql",
			"log_received_mails": true,
			"save_workers_size":  saveWorkersSize,
			"primary_mail_host":  primaryMailHost,
		},
	})
	if err != nil {
		log.Printf("Failed to reload mail server: %v", err)
		return nil, responses, []error{err}
	}

	return nil, responses, nil
}

func NewMailServersSyncActionPerformer(cruds map[string]*resource.DbResource, mailDaemon *guerrilla.Daemon, certificateManager *resource.CertificateManager) (actionresponse.ActionPerformerInterface, error) {
	handler := mailServersSyncActionPerformer{
		cruds:              cruds,
		mailDaemon:         mailDaemon,
		certificateManager: certificateManager,
	}

	return &handler, nil
}
