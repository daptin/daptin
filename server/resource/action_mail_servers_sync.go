package resource

import (
	"encoding/json"
	"github.com/artpar/api2go"
	"github.com/flashmob/go-guerrilla"
	"github.com/flashmob/go-guerrilla/backends"
	"log"
	"strconv"
)

type MailServersSyncActionPerformer struct {
	cruds      map[string]*DbResource
	mailDaemon *guerrilla.Daemon
}

func (d *MailServersSyncActionPerformer) Name() string {
	return "mail.servers.sync"
}

func (d *MailServersSyncActionPerformer) DoAction(request ActionRequest, inFields map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	log.Printf("Sync mail servers")
	responses := make([]ActionResponse, 0)

	servers, err := d.cruds["mail_server"].GetAllObjects("mail_server")

	if err != nil {
		return nil, []ActionResponse{}, []error{err}
	}

	serverConfig := make([]guerrilla.ServerConfig, 0)

	hosts := []string{}
	for _, server := range servers {

		var tlsConfig guerrilla.ServerTLSConfig

		json.Unmarshal([]byte(server["tls"].(string)), &tlsConfig)
		max_size, _ := strconv.ParseInt(server["max_size"].(string), 10, 32)
		max_clients, _ := strconv.ParseInt(server["max_clients"].(string), 10, 32)
		config := guerrilla.ServerConfig{
			IsEnabled:       server["is_enabled"].(string) == "1",
			ListenInterface: server["listen_interface"].(string),
			Hostname:        server["hostname"].(string),
			MaxSize:         max_size,
			TLS:             tlsConfig,
			MaxClients:      int(max_clients),
			XClientOn:       server["xclient_on"].(string) == "1",
		}
		hosts = append(hosts, server["hostname"].(string))

		serverConfig = append(serverConfig, config)

	}


	err = d.mailDaemon.ReloadConfig(guerrilla.AppConfig{
		Servers: serverConfig,
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
