package server

import (
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/dbresourceinterface"
	"github.com/daptin/daptin/server/resource"
	"github.com/fclairamb/ftpserver/server"
	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
)

func InitializeFtpResources(configStore *resource.ConfigStore, transaction *sqlx.Tx, ftpServer *server.FtpServer, cruds map[string]*resource.DbResource, crudsInterface map[string]dbresourceinterface.DbResourceInterface, certificateManager *resource.CertificateManager) *server.FtpServer {
	ftp_interface, err := configStore.GetConfigValueFor("ftp.listen_interface", "backend", transaction)
	if err != nil {
		ftp_interface = "0.0.0.0:2121"
		err = configStore.SetConfigValueFor("ftp.listen_interface", ftp_interface, "backend", transaction)
		resource.CheckErr(err, "Failed to store default value for ftp.listen_interface")
	}
	// ftpListener, err := net.Listen("tcp", ftp_interface)
	// resource.CheckErr(err, "Failed to create listener for FTP")
	ftpServer, err = CreateFtpServers(cruds, crudsInterface, certificateManager, ftp_interface, transaction)
	auth.CheckErr(err, "Failed to creat FTP server")
	go func() {
		logrus.Printf("FTP server started at %v", ftp_interface)
		err = ftpServer.ListenAndServe()
		resource.CheckErr(err, "Failed to listen at ftp interface")
	}()
	return ftpServer
}
