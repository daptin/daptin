package server

import (
	"context"
	"errors"

	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/dbresourceinterface"
	"github.com/daptin/daptin/server/resource"
	"github.com/fclairamb/ftpserver/server"
	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
)

func InitializeFtpResources(ctx context.Context, configStore *resource.ConfigStore, transaction *sqlx.Tx,
	cruds map[string]*resource.DbResource, crudsInterface map[string]dbresourceinterface.DbResourceInterface,
	certificateManager *resource.CertificateManager, runtimeErrors chan<- error) (*server.FtpServer, *ConnectionTracker, error) {
	ftp_interface, err := configStore.GetConfigValueFor("ftp.listen_interface", "backend", transaction)
	if err != nil {
		ftp_interface = "0.0.0.0:2121"
		err = configStore.SetConfigValueFor("ftp.listen_interface", ftp_interface, "backend", transaction)
		resource.CheckErr(err, "Failed to store default value for ftp.listen_interface")
	}
	ftpServer, connections, err := CreateFtpServers(cruds, crudsInterface, certificateManager, ftp_interface, transaction)
	auth.CheckErr(err, "Failed to create FTP server")
	if err != nil {
		return nil, nil, err
	}
	go func() {
		logrus.Printf("FTP server started at %v", ftp_interface)
		ftpServer.Serve()
		select {
		case <-ctx.Done():
		default:
			select {
			case runtimeErrors <- errors.New("FTP server stopped unexpectedly"):
			default:
			}
		}
	}()
	return ftpServer, connections, nil
}
