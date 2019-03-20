package server

import (
	"encoding/json"
	"fmt"
	"github.com/daptin/daptin/server/resource"
	"github.com/flashmob/go-guerrilla"
	"github.com/flashmob/go-guerrilla/backends"
	"github.com/flashmob/go-guerrilla/log"
	"strconv"
)

func StartMailServer(resource *resource.DbResource) (*guerrilla.Daemon, error) {

	servers, err := resource.GetAllObjects("mail_server")

	if err != nil {
		return nil, err
	}

	serverConfig := make([]guerrilla.ServerConfig, 0)
	hosts := []string{}

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

	d := guerrilla.Daemon{
		Config: &guerrilla.AppConfig{
			AllowedHosts: hosts,
			LogLevel:     log.DebugLevel.String(),
			BackendConfig: backends.BackendConfig{
				"save_process":       "HeadersParser|Debugger|Hasher|Header|Compressor|DaptinSql",
				"log_received_mails": true,
				"mail_table":         "mail",
				"save_workers_size":  1,
				"primary_mail_host":  "localhost",
			},
			Servers: serverConfig,
		},
	}

	d.AddProcessor("DaptinSql", DaptinSQLDbResource(resource))

	return &d, nil
}
