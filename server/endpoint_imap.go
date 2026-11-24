package server

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/artpar/go-imap/server"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
)

func InitializeImapResources(configStore *resource.ConfigStore, transaction *sqlx.Tx, cruds map[string]*resource.DbResource,
	imapServer *server.Server, certificateManager *resource.CertificateManager, runtimeErrors chan<- error, status *mailServiceRuntimeStatus) (*server.Server, error) {
	imapListenInterface, err := configStore.GetConfigValueFor("imap.listen_interface", "backend", transaction)
	if err != nil {
		err = configStore.SetConfigValueFor("imap.listen_interface", ":1143", "backend", transaction)
		resource.CheckErr(err, "Failed to store default imap listen interface in config")
		imapListenInterface = ":1143"
	}

	hostname := resolveIMAPHostname(configStore, transaction)
	observation := mailProtocolStatus{State: "starting", ConfiguredEnabled: true, Enabled: true,
		ListenInterface: imapListenInterface, Hostname: hostname, TLSMode: "starttls", AuthenticationRequired: true,
		AuthenticationMechanisms: []string{"PLAIN"}}
	if EndsWithCheck(imapListenInterface, ":993") {
		observation.TLSMode = "implicit"
	}
	status.setIMAP(observation)
	imapBackend := resource.NewImapServer(cruds)

	// Create a new server
	imapServer = server.New(imapBackend)
	imapServer.Addr = imapListenInterface
	imapServer.Debug = nil
	imapServer.AllowInsecureAuth = false
	imapServer.MaxLiteralSize = 25 * 1024 * 1024
	imapServer.Enable(&selectedIMAPExtension{})
	imapServer.Enable(&bodyIMAPExtension{})

	cert, err := certificateManager.GetTLSConfig(hostname, true, transaction)
	if err != nil {
		status.imapState("failed", "certificate_failed")
		logrus.Printf("Failed to get certificate for IMAP [%v]: %v — IMAP server will not start", hostname, err)
		return nil, err
	}
	imapServer.TLSConfig = cert.TLSConfig
	observation.CertificateLoaded = len(cert.TLSConfig.Certificates) > 0 && len(cert.TLSConfig.Certificates[0].Certificate) > 0 && cert.TLSConfig.Certificates[0].PrivateKey != nil
	status.setIMAP(observation)

	logrus.Printf("Starting IMAP server at %s: %v", imapListenInterface, hostname)

	listener, err := net.Listen("tcp", imapListenInterface)
	if err != nil {
		status.imapState("failed", "listen_failed")
		return nil, err
	}
	observation.State, observation.BoundAddress = "running", listener.Addr().String()
	status.setIMAP(observation)
	if EndsWithCheck(imapListenInterface, ":993") {
		listener = tls.NewListener(listener, imapServer.TLSConfig)
	}
	go func() {
		serveErr := imapServer.Serve(listener)
		status.imapStopped()
		if serveErr != nil && !errors.Is(serveErr, net.ErrClosed) {
			select {
			case runtimeErrors <- fmt.Errorf("IMAP server: %w", serveErr):
			default:
			}
		}
	}()
	return imapServer, nil
}

func resolveIMAPHostname(configStore *resource.ConfigStore, transaction *sqlx.Tx) string {
	imapHostname, err := configStore.GetConfigValueFor("imap.hostname", "backend", transaction)
	if err == nil && strings.TrimSpace(imapHostname) != "" {
		return configuredIMAPHostname(imapHostname, "")
	}

	backendHostname, err := configStore.GetConfigValueFor("hostname", "backend", transaction)
	if err != nil || strings.TrimSpace(backendHostname) == "" {
		backendHostname = "localhost"
		logrus.Printf("Failed to get hostname config for IMAP, using fallback: %v", err)
	}
	return configuredIMAPHostname("", backendHostname)
}

func configuredIMAPHostname(imapHostname, backendHostname string) string {
	imapHostname = strings.TrimSpace(imapHostname)
	if imapHostname != "" {
		return imapHostname
	}
	return derivedIMAPHostname(backendHostname)
}

func derivedIMAPHostname(backendHostname string) string {
	backendHostname = strings.TrimSpace(backendHostname)
	if backendHostname == "" {
		backendHostname = "localhost"
	}
	return "imap." + backendHostname
}
