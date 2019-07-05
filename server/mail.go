package server

import (
	"encoding/json"
	"fmt"
	"github.com/artpar/go-guerrilla"
	"github.com/artpar/go-guerrilla/backends"
	"github.com/artpar/go-guerrilla/log"
	"github.com/daptin/daptin/server/resource"
	"strconv"
	"strings"
)

func StartSMTPMailServer(resource *resource.DbResource) (*guerrilla.Daemon, error) {

	servers, err := resource.GetAllObjects("mail_server")

	if err != nil {
		return nil, err
	}

	serverConfig := make([]guerrilla.ServerConfig, 0)
	hosts := []string{}

	for _, server := range servers {

		var tlsConfig guerrilla.ServerTLSConfig

		json.Unmarshal([]byte(server["tls"].(string)), &tlsConfig)

		maxSize, _ := strconv.ParseInt(fmt.Sprintf("%v", server["max_size"]), 10, 32)
		maxClients, _ := strconv.ParseInt(fmt.Sprintf("%v", server["max_clients"]), 10, 32)
		authRequiredString := server["authentication_required"].(string)
		authRequired := true
		if authRequiredString == "0" {
			authRequired = false
		}
		authTypes := strings.Split(server["authentication_types"].(string), ",")

		config := guerrilla.ServerConfig{
			IsEnabled:       fmt.Sprintf("%v", server["is_enabled"]) == "1",
			ListenInterface: server["listen_interface"].(string),
			Hostname:        server["hostname"].(string),
			MaxSize:         maxSize,
			TLS:             tlsConfig,
			MaxClients:      int(maxClients),
			XClientOn:       fmt.Sprintf("%v", server["xclient_on"]) == "1",
			AuthRequired:    authRequired,
			AuthTypes:       authTypes,
		}
		hosts = append(hosts, server["hostname"].(string))

		serverConfig = append(serverConfig, config)

	}

	hosts = append(hosts, "*")
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

	d.AddProcessor("DaptinSql", DaptinSmtpDbResource(resource))
	d.AddAuthenticator(DaptinSmtpAuthenticatorCreator(resource))

	return &d, nil
}
