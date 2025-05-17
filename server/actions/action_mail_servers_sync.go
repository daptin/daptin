package actions

import (
	"fmt"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/resource"
	"os"
	"path/filepath"
	"strconv"

	"github.com/artpar/api2go/v2"
	"github.com/artpar/go-guerrilla"
	"github.com/artpar/go-guerrilla/backends"
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

	if d.mailDaemon == nil || d.mailDaemon.Backend == nil {
		log.Debugf("mail daemon was not initialized, returning without any outcome")
		return nil, []actionresponse.ActionResponse{}, []error{}
	}
	//log.Printf("Sync mail servers")
	responses := make([]actionresponse.ActionResponse, 0)

	servers, err := d.cruds["mail_server"].GetAllObjects("mail_server", transaction)

	if err != nil {
		return nil, []actionresponse.ActionResponse{}, []error{err}
	}

	serverConfig := make([]guerrilla.ServerConfig, 0)
	sourceDirectoryName := "daptin-certs"
	tempDirectoryPath, err := os.MkdirTemp(os.Getenv("DAPTIN_CACHE_FOLDER"), sourceDirectoryName)

	var hosts []string
	for _, server := range servers {

		var serverTlsConfig guerrilla.ServerTLSConfig

		//json.Unmarshal([]byte(server["tls"].(string)), &serverTlsConfig)

		maxSize, _ := strconv.ParseInt(fmt.Sprintf("%v", server["max_size"]), 10, 32)
		maxClients, _ := strconv.ParseInt(fmt.Sprintf("%v", server["max_clients"]), 10, 32)
		alwaysOnTls := fmt.Sprintf("%v", server["always_on_tls"]) == "1"
		authenticationRequired := fmt.Sprintf("%v", server["authentication_required"]) == "1"

		//authTypes := strings.Split(server["authentication_types"].(string), ",")

		hostname := server["hostname"].(string)
		cert, err := d.certificateManager.GetTLSConfig(hostname, true, transaction)

		if err != nil {
			log.Printf("Failed to generate Certificates for SMTP server for %s", hostname)
		}

		//certFilePath := filepath.Join(tempDirectoryPath, hostname+".cert.pem")
		privateKeyFilePath := filepath.Join(tempDirectoryPath, hostname+".private.cert.pem")
		publicKeyFilePath := filepath.Join(tempDirectoryPath, hostname+".public.cert.pem")
		rootCaFile := filepath.Join(tempDirectoryPath, hostname+".root.cert.pem")

		//err = os.WriteFile(certFilePath, certPEMBytes, 0666)
		//if err != nil {
		//	log.Printf("Failed to generate Certificates for SMTP server for %s", hostname)
		//}

		err = os.WriteFile(publicKeyFilePath, []byte(string(cert.PublicPEMDecrypted)+"\n"+string(cert.CertPEM)+"\n"+string(cert.RootCert)), 0666)
		if err != nil {
			log.Printf("Failed to generate public key for SMTP server for %s", hostname)
		}
		err = os.WriteFile(rootCaFile, []byte(string(cert.RootCert)), 0666)
		if err != nil {
			log.Printf("Failed to generate public key for SMTP server for %s", hostname)
		}

		err = os.WriteFile(privateKeyFilePath, cert.PrivatePEMDecrypted, 0666)
		//err = os.WriteFile(publicKeyFilePath, publicPEMBytes, 0666)

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
			//Ciphers:                  []string{"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384", "TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA", "TLS_RSA_WITH_3DES_EDE_CBC_SHA"},
			//Protocols:                []string{"tls1.0", "tls1.3"},
		}

		config := guerrilla.ServerConfig{
			IsEnabled:       fmt.Sprintf("%v", server["is_enabled"]) == "1",
			ListenInterface: server["listen_interface"].(string),
			Hostname:        hostname,
			MaxSize:         maxSize,
			Timeout:         30,
			TLS:             serverTlsConfig,
			MaxClients:      int(maxClients),
			XClientOn:       fmt.Sprintf("%v", server["xclient_on"]) == "1",
			AuthRequired:    authenticationRequired,
			AuthTypes:       []string{"LOGIN"},
		}

		hosts = append(hosts, server["hostname"].(string))

		serverConfig = append(serverConfig, config)

	}

	hosts = append(hosts, "*")
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

func NewMailServersSyncActionPerformer(cruds map[string]*resource.DbResource, mailDaemon *guerrilla.Daemon, certificateManager *resource.CertificateManager) (actionresponse.ActionPerformerInterface, error) {

	handler := mailServersSyncActionPerformer{
		cruds:              cruds,
		mailDaemon:         mailDaemon,
		certificateManager: certificateManager,
	}

	return &handler, nil

}
