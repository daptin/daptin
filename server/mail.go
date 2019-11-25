package server

import (
	"fmt"
	"github.com/artpar/go-guerrilla"
	"github.com/artpar/go-guerrilla/backends"
	glog "github.com/artpar/go-guerrilla/log"
	"github.com/daptin/daptin/server/resource"
	"io/ioutil"
	"log"
	"path/filepath"
	"strconv"
)

func StartSMTPMailServer(resource *resource.DbResource) (*guerrilla.Daemon, error) {

	servers, err := resource.GetAllObjects("mail_server")

	if err != nil {
		return nil, err
	}

	serverConfig := make([]guerrilla.ServerConfig, 0)
	hosts := []string{}

	sourceDirectoryName := "daptin-certs"
	tempDirectoryPath, err := ioutil.TempDir("", sourceDirectoryName)

	for _, server := range servers {

		var serverTlsConfig guerrilla.ServerTLSConfig

		//json.Unmarshal([]byte(server["tls"].(string)), &serverTlsConfig)

		maxSize, _ := strconv.ParseInt(fmt.Sprintf("%v", server["max_size"]), 10, 32)
		maxClients, _ := strconv.ParseInt(fmt.Sprintf("%v", server["max_clients"]), 10, 32)
		authRequiredString := server["authentication_required"].(string)
		authRequired := false
		if authRequiredString == "1" {
			authRequired = true
		}
		//authTypes := strings.Split(server["authentication_types"].(string), ",")

		hostnames := server["hostname"].(string)
		_, certPEMBytes, privatePEMBytes, publicPEMBytes, err := GetTLSConfig(hostnames)

		if err != nil {
			log.Printf("Failed to generate Certificates for SMTP server for %s")
		}

		certFilePath := filepath.Join(tempDirectoryPath, hostnames+".cert.pem")
		privateKeyFilePath := filepath.Join(tempDirectoryPath, hostnames+".private.key.pem")
		publicKeyFilePath := filepath.Join(tempDirectoryPath, hostnames+".public.key.pem")

		err = ioutil.WriteFile(certFilePath, certPEMBytes, 0666)
		if err != nil {
			log.Printf("Failed to generate Certificates for SMTP server for %s")
		}

		err = ioutil.WriteFile(privateKeyFilePath, privatePEMBytes, 0666)
		err = ioutil.WriteFile(publicKeyFilePath, publicPEMBytes, 0666)

		if err != nil {
			log.Printf("Failed to generate Certificates for SMTP server for %s")
		}

		serverTlsConfig = guerrilla.ServerTLSConfig{
			StartTLSOn:     true,
			AlwaysOn:       false,
			PrivateKeyFile: privateKeyFilePath,
			PublicKeyFile:  certFilePath,
		}

		config := guerrilla.ServerConfig{
			IsEnabled:       fmt.Sprintf("%v", server["is_enabled"]) == "1",
			ListenInterface: server["listen_interface"].(string),
			Hostname:        hostnames,
			MaxSize:         maxSize,
			TLS:             serverTlsConfig,
			MaxClients:      int(maxClients),
			XClientOn:       fmt.Sprintf("%v", server["xclient_on"]) == "1",
			AuthRequired:    authRequired,
			AuthTypes:       []string{"LOGIN"},
		}
		hosts = append(hosts, hostnames)

		serverConfig = append(serverConfig, config)

	}

	hosts = append(hosts, "*")
	d := guerrilla.Daemon{
		Config: &guerrilla.AppConfig{
			AllowedHosts: hosts,
			LogLevel:     glog.DebugLevel.String(),
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
