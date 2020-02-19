package server

import (
	"fmt"
	"github.com/artpar/go-guerrilla"
	"github.com/artpar/go-guerrilla/backends"
	"github.com/daptin/daptin/server/resource"
	"io/ioutil"
	"log"
	"path/filepath"
	"strconv"
)

func StartSMTPMailServer(resource *resource.DbResource, certificateManager *resource.CertificateManager) (*guerrilla.Daemon, error) {

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

		authRequired, ok := server["authentication_required"].(bool)
		if !ok {
			authRequiredString := "1"
			authRequiredString, ok = server["authentication_required"].(string)
			authRequired = authRequiredString == "1"
		}

		//authTypes := strings.Split(server["authentication_types"].(string), ",")

		hostnames := server["hostname"].(string)
		_, certBytes, privatePEMBytes, publicKeyBytes, err := certificateManager.GetTLSConfig(hostnames)

		if err != nil {
			log.Printf("Failed to generate Certificates for SMTP server for %s", hostnames)
		}

		//certFilePath := filepath.Join(tempDirectoryPath, hostnames+".cert.pem")
		privateKeyFilePath := filepath.Join(tempDirectoryPath, hostnames+".private.cert.pem")
		publicKeyFilePath := filepath.Join(tempDirectoryPath, hostnames+".public.cert.pem")

		//err = ioutil.WriteFile(certFilePath, certPEMBytes, 0666)
		//if err != nil {
		//	log.Printf("Failed to generate Certificates for SMTP server for %s", hostnames)
		//}

		err = ioutil.WriteFile(publicKeyFilePath, []byte(string(publicKeyBytes)+"\n"+string(certBytes)), 0666)
		if err != nil {
			log.Printf("Failed to generate public key for SMTP server for %s", hostnames)
		}

		err = ioutil.WriteFile(privateKeyFilePath, privatePEMBytes, 0666)
		//err = ioutil.WriteFile(publicKeyFilePath, publicPEMBytes, 0666)

		if err != nil {
			log.Printf("Failed to generate Certificates for SMTP server for %s", hostnames)
		}

		serverTlsConfig = guerrilla.ServerTLSConfig{
			StartTLSOn:               true,
			AlwaysOn:                 true,
			PrivateKeyFile:           privateKeyFilePath,
			PublicKeyFile:            publicKeyFilePath,
			ClientAuthType:           "NoClientCert",
			PreferServerCipherSuites: true,
			Curves:                   []string{"P521", "P384"},
			Ciphers:                  []string{"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384", "TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA", "TLS_RSA_WITH_3DES_EDE_CBC_SHA"},
			Protocols:                []string{"tls1.0", "tls1.3"},
		}

		config := guerrilla.ServerConfig{
			IsEnabled:       fmt.Sprintf("%v", server["is_enabled"]) == "1",
			ListenInterface: server["listen_interface"].(string),
			Hostname:        hostnames,
			MaxSize:         maxSize,
			Timeout:         30,
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
			LogLevel:     "debug",
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

	d.AddProcessor("DaptinSql", DaptinSmtpDbResource(resource, certificateManager))
	d.AddAuthenticator(DaptinSmtpAuthenticatorCreator(resource))

	return &d, nil
}
