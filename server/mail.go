package server

import (
	"encoding/json"
	"github.com/daptin/daptin/server/resource"
	"github.com/flashmob/go-guerrilla"
	"github.com/flashmob/go-guerrilla/backends"
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

	d := guerrilla.Daemon{
		Config: &guerrilla.AppConfig{
			AllowedHosts: hosts,
			BackendConfig: backends.BackendConfig{
				"save_process":       "HeadersParser|Debugger|Hasher|Header|Compressor|DaptinSql",
				"log_received_mails": true,
				"mail_table":         "mails",
				"save_workers_size":  1,
				"primary_mail_host":  "localhost",
			},
			Servers: serverConfig,
		},
	}

	d.AddProcessor("DaptinSql", DaptinSQLDbResource(resource))

	return &d, nil
}
