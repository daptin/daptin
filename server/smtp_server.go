package server

import (
	"strconv"

	"github.com/artpar/go-guerrilla"
	"github.com/artpar/go-guerrilla/backends"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

func StartSMTPMailServer(mailResource *resource.DbResource, certificateManager *resource.CertificateManager, primaryHostname string, transaction *sqlx.Tx) (*guerrilla.Daemon, error) {
	servers, err := mailResource.GetAllObjects("mail_server", transaction)
	if err != nil {
		return nil, err
	}

	serverConfig, hosts, err := resource.BuildSMTPServerConfigs(servers, certificateManager, transaction)
	if err != nil {
		return nil, err
	}
	for _, config := range serverConfig {
		log.Infof("Setup SMTP server at [%v] for hostname [%v] (enabled=%v)", config.ListenInterface, config.Hostname, config.IsEnabled)
	}
	primaryMailHost := primaryHostname
	if len(hosts) > 0 {
		primaryMailHost = hosts[0]
	}

	saveWorkersSize := 1
	if sws, err := mailResource.ConfigStore.GetConfigValueFor("mail.save_workers_size", "backend", transaction); err == nil && sws != "" {
		if parsed, err := strconv.Atoi(sws); err == nil && parsed > 0 {
			saveWorkersSize = parsed
		}
	}

	d := guerrilla.Daemon{
		Config: &guerrilla.AppConfig{
			AllowedHosts: hosts,
			LogLevel:     "debug",
			BackendConfig: backends.BackendConfig{
				"save_process":       "HeadersParser|Debugger|Hasher|Header|Compressor|DaptinSql",
				"log_received_mails": true,
				"mail_table":         "mail",
				"save_workers_size":  saveWorkersSize,
				"primary_mail_host":  primaryMailHost,
			},
			Servers: serverConfig,
		},
	}

	smtpResource := DaptinSmtpDbResource(mailResource, certificateManager)

	d.AddProcessor("DaptinSql", smtpResource)
	d.AddAuthenticator(DaptinSmtpAuthenticatorCreator(mailResource))

	return &d, nil
}
