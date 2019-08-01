package resource

import (
	"encoding/json"
	"fmt"
	"github.com/artpar/api2go"
	"github.com/artpar/go-guerrilla"
	"github.com/artpar/go-guerrilla/backends"
	log "github.com/sirupsen/logrus"
	"strconv"
)

type MailServersSyncActionPerformer struct {
	cruds      map[string]*DbResource
	mailDaemon *guerrilla.Daemon
}

func (d *MailServersSyncActionPerformer) Name() string {
	return "mail.servers.sync"
}

func (d *MailServersSyncActionPerformer) DoAction(request Outcome, inFields map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	log.Printf("Sync mail servers")
	responses := make([]ActionResponse, 0)

	servers, err := d.cruds["mail_server"].GetAllObjects("mail_server")

	if err != nil {
		return nil, []ActionResponse{}, []error{err}
	}

	serverConfig := make([]guerrilla.ServerConfig, 0)

	var hosts []string
	for _, server := range servers {

		var tlsConfig guerrilla.ServerTLSConfig

		json.Unmarshal([]byte(server["tls"].(string)), &tlsConfig)

		max_size, _ := strconv.ParseInt(fmt.Sprintf("%v", server["max_size"]), 10, 32)
		max_clients, _ := strconv.ParseInt(fmt.Sprintf("%v", server["max_clients"]), 10, 32)

		config := guerrilla.ServerConfig{
			IsEnabled:       fmt.Sprintf("%v", server["is_enabled"]) == "1",
			ListenInterface: server["listen_interface"].(string),
			Hostname:        server["hostname"].(string),
			MaxSize:         max_size,
			TLS:             tlsConfig,
			MaxClients:      int(max_clients),
			XClientOn:       fmt.Sprintf("%v", server["xclient_on"]) == "1",
		}
		hosts = append(hosts, server["hostname"].(string))

		serverConfig = append(serverConfig, config)

	}

	err = d.mailDaemon.ReloadConfig(guerrilla.AppConfig{
		Servers:      serverConfig,
		AllowedHosts: hosts,
		BackendConfig: backends.BackendConfig{
			"save_process":       "HeadersParser|Debugger|Hasher|Header|Compressor|DaptinSql",
			"log_received_mails": true,
			"save_workers_size":  1,
			"primary_mail_host":  "localhost",
		},
	})

	//err = d.mailDaemon.Start()
	if err != nil {
		log.Printf("Failed to start mail server: %v", err)
	}

	return nil, responses, nil
}

func NewMailServersSyncActionPerformer(cruds map[string]*DbResource, mailDaemon *guerrilla.Daemon) (ActionPerformerInterface, error) {

	handler := MailServersSyncActionPerformer{
		cruds:      cruds,
		mailDaemon: mailDaemon,
	}

	return &handler, nil

}
