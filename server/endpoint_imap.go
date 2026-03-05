package server

import (
	"github.com/artpar/go-imap"
	"github.com/artpar/go-imap-idle"
	"github.com/artpar/go-imap/backend"
	"github.com/artpar/go-imap/responses"
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
	if err != nil || hostname == "" {
		hostname = "localhost"
		logrus.Printf("Failed to get hostname config for IMAP, using fallback: %v", err)
	}
	hostname = "imap." + hostname
	imapBackend := resource.NewImapServer(cruds)

	// Create a new server
	imapServer = server.New(imapBackend)
	imapServer.Addr = imapListenInterface
	imapServer.Debug = nil
	imapServer.AllowInsecureAuth = false
	imapServer.Enable(idle.NewExtension())
	imapServer.Enable(&noopPollExtension{})

	cert, err := certificateManager.GetTLSConfig(hostname, true, transaction)
	if err != nil {
		logrus.Printf("Failed to get certificate for IMAP [%v]: %v — IMAP server will not start", hostname, err)
		return nil
	}
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

// noopPollExtension overrides the default NOOP handler to send untagged EXISTS
// responses when Poll() detects new messages. This works without BackendUpdater
// so all inline fallbacks (APPEND, EXPUNGE, STORE) remain active.
type noopPollExtension struct{}

func (ext *noopPollExtension) Capabilities(_ server.Conn) []string {
	return nil
}

func (ext *noopPollExtension) Command(name string) server.HandlerFactory {
	if name == "NOOP" {
		return func() server.Handler { return &noopPollHandler{} }
	}
	return nil
}

type noopPollHandler struct{}

func (cmd *noopPollHandler) Parse(fields []interface{}) error {
	return nil
}

func (cmd *noopPollHandler) Handle(conn server.Conn) error {
	ctx := conn.Context()
	if ctx.Mailbox == nil {
		return nil
	}

	// Call Poll() if the mailbox supports it
	if poller, ok := ctx.Mailbox.(backend.MailboxPoller); ok {
		if err := poller.Poll(); err != nil {
			return err
		}
	}

	// Check if Poll() found new messages
	dimb, ok := ctx.Mailbox.(*resource.DaptinImapMailBox)
	if !ok {
		return nil
	}

	pendingStatus := dimb.ConsumePollUpdate()
	if pendingStatus == nil {
		return nil
	}

	// Send untagged EXISTS/RECENT via Select response
	mbs := imap.NewMailboxStatus(pendingStatus.Name, []imap.StatusItem{imap.StatusMessages, imap.StatusRecent})
	mbs.Messages = pendingStatus.Messages
	mbs.Recent = pendingStatus.Recent
	res := &responses.Select{Mailbox: mbs}
	return conn.WriteResp(res)
}
