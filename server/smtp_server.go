package server

import (
	"fmt"
	"github.com/artpar/go-guerrilla"
	"github.com/artpar/go-guerrilla/backends"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"strconv"
)

func toBool(v interface{}) bool {
	s := fmt.Sprintf("%v", v)
	return s == "1" || s == "true"
}

func StartSMTPMailServer(resource *resource.DbResource, certificateManager *resource.CertificateManager, primaryHostname string, transaction *sqlx.Tx) (*guerrilla.Daemon, error) {

	servers, err := resource.GetAllObjects("mail_server", transaction)

	if err != nil {
		return nil, err
	}

	serverConfig := make([]guerrilla.ServerConfig, 0)
	hosts := []string{}

	sourceDirectoryName := "daptin-certs"
	tempDirectoryPath, err := os.MkdirTemp(os.Getenv("DAPTIN_CACHE_FOLDER"), sourceDirectoryName)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory for SMTP certs: %w", err)
	}

	for _, server := range servers {

		var serverTlsConfig guerrilla.ServerTLSConfig

		//json.Unmarshal([]byte(server["tls"].(string)), &serverTlsConfig)

		maxSize, _ := strconv.ParseInt(fmt.Sprintf("%v", server["max_size"]), 10, 32)
		maxClients, _ := strconv.ParseInt(fmt.Sprintf("%v", server["max_clients"]), 10, 32)
		alwaysOnTls := toBool(server["always_on_tls"])
		authenticationRequired := toBool(server["authentication_required"])

		//authTypes := strings.Split(server["authentication_types"].(string), ",")

		hostnameVal, ok := server["hostname"].(string)
		if !ok || hostnameVal == "" {
			log.Printf("Skipping SMTP server entry with missing hostname")
			continue
		}
		hostname := hostnameVal
		cert, err := certificateManager.GetTLSConfig(hostname, true, transaction)

		if err != nil {
			log.Printf("Failed to generate Certificates for SMTP server for %s, skipping", hostname)
			continue
		}

		//certFilePath := filepath.Join(tempDirectoryPath, hostname+".cert.pem")
		privateKeyFilePath := filepath.Join(tempDirectoryPath, hostname+".private.cert.pem")
		publicKeyFilePath := filepath.Join(tempDirectoryPath, hostname+".public.cert.pem")
		rootCaFile := filepath.Join(tempDirectoryPath, hostname+".root.cert.pem")

		//err = ioutil.WriteFile(certFilePath, certPEMBytes, 0666)
		//if err != nil {
		//	log.Printf("Failed to generate Certificates for SMTP server for %s", hostname)
		//}

		err = os.WriteFile(publicKeyFilePath, []byte(string(cert.PublicPEMDecrypted)+"\n"+string(cert.CertPEM)), 0600)
		if err != nil {
			log.Printf("Failed to generate public key for SMTP server for %s", hostname)
		}
		err = os.WriteFile(rootCaFile, []byte(cert.RootCert), 0600)
		if err != nil {
			log.Printf("Failed to generate public key for SMTP server for %s", hostname)
		}

		err = os.WriteFile(privateKeyFilePath, cert.PrivatePEMDecrypted, 0600)
		//err = ioutil.WriteFile(publicKeyFilePath, publicPEMBytes, 0666)

		if err != nil {
			log.Printf("Failed to generate Certificates for SMTP server for %s", hostname)
		}

		serverTlsConfig = guerrilla.ServerTLSConfig{
			StartTLSOn:               true,
			AlwaysOn:                 alwaysOnTls,
			PrivateKeyFile:           privateKeyFilePath,
			PublicKeyFile:            publicKeyFilePath,
			RootCAs:                  rootCaFile,
			ClientAuthType:           "NoClientCert",
			PreferServerCipherSuites: true,
			//Curves:                   []string{"P521", "P384"},
			//Ciphers: []string{
			//	"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
			//	"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
			//	"TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA384",
			//	"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256",
			//	"TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA",
			//	"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA",
			//	"TLS_RSA_WITH_AES_256_GCM_SHA384",
			//	"TLS_RSA_WITH_AES_128_GCM_SHA256",
			//},
			//Protocols: []string{"tls1.0", "tls1.1", "tls1.2", "tls1.3"},
		}

		config := guerrilla.ServerConfig{
			IsEnabled:       toBool(server["is_enabled"]),
			ListenInterface: server["listen_interface"].(string),
			Hostname:        hostname,
			MaxSize:         maxSize,
			Timeout:         30,
			TLS:             serverTlsConfig,
			MaxClients:      int(maxClients),
			XClientOn:       toBool(server["xclient_on"]),
			AuthRequired:    authenticationRequired,
			AuthTypes:       []string{"LOGIN"},
		}
		hosts = append(hosts, hostname)

		log.Infof("Setup SMTP server at [%v] for hostname [%v] (enabled=%v)", server["listen_interface"], hostname, config.IsEnabled)
		serverConfig = append(serverConfig, config)

	}

	// Do not add wildcard "*" — only accept mail for configured hostnames

	saveWorkersSize := 1
	if sws, err := resource.ConfigStore.GetConfigValueFor("mail.save_workers_size", "backend", transaction); err == nil && sws != "" {
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
				"primary_mail_host":  primaryHostname,
			},
			Servers: serverConfig,
		},
	}

	smtpResource := DaptinSmtpDbResource(resource, certificateManager)

	d.AddProcessor("DaptinSql", smtpResource)
	d.AddAuthenticator(DaptinSmtpAuthenticatorCreator(resource))

	return &d, nil
}
