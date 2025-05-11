package server

import (
	"github.com/artpar/go-imap-idle"
	"github.com/artpar/go-imap/server"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
)

func InitializeImapResources(configStore *resource.ConfigStore, transaction *sqlx.Tx, cruds map[string]*resource.DbResource, imapServer *server.Server, certificateManager *resource.CertificateManager) *server.Server {
	imapListenInterface, err := configStore.GetConfigValueFor("imap.listen_interface", "backend", transaction)
	if err != nil {
		err = configStore.SetConfigValueFor("imap.listen_interface", ":1143", "backend", transaction)
		resource.CheckErr(err, "Failed to store default imap listen interface in config")
		imapListenInterface = ":1143"
	}

	hostname, err := configStore.GetConfigValueFor("hostname", "backend", transaction)
	hostname = "imap." + hostname
	imapBackend := resource.NewImapServer(cruds)

	// Create a new server
	imapServer = server.New(imapBackend)
	imapServer.Addr = imapListenInterface
	imapServer.Debug = nil
	imapServer.AllowInsecureAuth = false
	imapServer.Enable(idle.NewExtension())

	cert, err := certificateManager.GetTLSConfig(hostname, true, transaction)
	resource.CheckErr(err, "Failed to get certificate for IMAP [%v]", hostname)
	imapServer.TLSConfig = cert.TLSConfig

	logrus.Printf("Starting IMAP server at %s: %v", imapListenInterface, hostname)

	go func() {
		if EndsWithCheck(imapListenInterface, ":993") {
			if err := imapServer.ListenAndServeTLS(); err != nil {
				resource.CheckErr(err, "Imap server is not listening anymore 1")
			}
		} else {
			if err := imapServer.ListenAndServe(); err != nil {
				resource.CheckErr(err, "Imap server is not listening anymore 2")
			}
		}
	}()
	return imapServer
}
