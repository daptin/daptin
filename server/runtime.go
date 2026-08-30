package server

import (
	"context"
	"errors"
	"net/http"
	"sync"

	"github.com/artpar/go-guerrilla"
	imapserver "github.com/artpar/go-imap/server"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/subsite"
	"github.com/daptin/daptin/server/websockets"
	ftpserver "github.com/fclairamb/ftpserver/server"
	"github.com/go-redis/redis/v8"
)

// Runtime owns every service created by NewRuntime. The process owns the
// database, Olric, and the public HTTP listeners and closes those after Drain.
type Runtime struct {
	Handler            http.Handler
	ConfigStore        *resource.ConfigStore
	CertificateManager *resource.CertificateManager

	mailDaemon              *guerrilla.Daemon
	scheduler               *resource.DefaultTaskScheduler
	ftpServer               *ftpserver.FtpServer
	ftpConnections          *ConnectionTracker
	imapServer              *imapserver.Server
	websocketServer         *websockets.Server
	yjs                     *YjsRuntime
	tableSubscription       *redis.PubSub
	integrationSubscription *redis.PubSub
	errors                  chan error
	quiesceOnce             sync.Once
	quiesceErr              error
}

// Errors reports unexpected failures from services after startup.
func (r *Runtime) Errors() <-chan error {
	return r.errors
}

// Quiesce stops non-HTTP services from accepting or scheduling new work while
// leaving shared handler dependencies available to requests already in flight.
func (r *Runtime) Quiesce() error {
	r.quiesceOnce.Do(func() {
		if r.scheduler != nil {
			r.scheduler.Quiesce()
		}
		if r.ftpServer != nil {
			r.ftpServer.Stop()
		}
		if r.mailDaemon != nil {
			r.mailDaemon.Shutdown()
		}
		if r.imapServer != nil {
			r.quiesceErr = r.imapServer.Close()
		}
	})
	return r.quiesceErr
}

// Drain waits for service-owned work and then closes shared handler
// dependencies. The caller must drain HTTP requests after Quiesce and before
// calling Drain. Database and Olric remain usable until Drain returns.
func (r *Runtime) Drain(ctx context.Context) error {
	var errs []error
	errs = append(errs, r.Quiesce())
	if r.scheduler != nil {
		errs = append(errs, r.scheduler.Stop(ctx))
	}
	if r.ftpConnections != nil {
		r.ftpConnections.CloseAll()
	}
	if r.websocketServer != nil {
		errs = append(errs, r.websocketServer.Shutdown(ctx))
	}
	errs = append(errs, resource.ShutdownEventWorkerPool(ctx))
	if r.yjs != nil {
		r.yjs.Close()
	}
	if r.integrationSubscription != nil {
		errs = append(errs, r.integrationSubscription.Close())
	}
	if r.tableSubscription != nil {
		errs = append(errs, r.tableSubscription.Close())
	}
	ShutdownFileCache()
	ShutdownSubsiteCache()
	subsite.ShutdownTemplateCache()
	return errors.Join(errs...)
}
