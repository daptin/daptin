package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	ftpserver "github.com/artpar/ftpserver/server"
	"github.com/artpar/go-guerrilla"
	imapserver "github.com/artpar/go-imap/server"
	"github.com/buraksezer/olric"
	"github.com/daptin/daptin/server/agent"
	"github.com/daptin/daptin/server/llm"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/subsite"
	"github.com/daptin/daptin/server/websockets"
	"github.com/go-redis/redis/v8"
)

// Runtime owns every service created by NewRuntime. The process owns the
// database, Olric, and the public HTTP listeners and closes those after Drain.
type Runtime struct {
	Handler            http.Handler
	ConfigStore        *resource.ConfigStore
	CertificateManager *resource.CertificateManager

	// RestartRequests asks the process owner to drain this runtime and restart.
	RestartRequests <-chan struct{}

	mailDaemon              *guerrilla.Daemon
	mailStatus              *mailServiceRuntimeStatus
	scheduler               *resource.DefaultTaskScheduler
	ftpServer               *ftpserver.FtpServer
	ftpConnections          *ConnectionTracker
	imapServer              *imapserver.Server
	websocketServer         *websockets.Server
	yjs                     *YjsRuntime
	llmGateway              *llm.Gateway
	agentService            *agent.Service
	rateLimitClient         *olric.ClusterClient
	tableSubscription       *redis.PubSub
	integrationSubscription *redis.PubSub
	cancelWork              context.CancelFunc
	stopWorkDeadline        func() bool
	restartState            *resource.RuntimeRestart
	restartCancel           context.CancelFunc
	restartDone             <-chan struct{}
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
// workCtx bounds active work; Drain receives the remaining finalization budget.
func (r *Runtime) Quiesce(workCtx context.Context) error {
	r.quiesceOnce.Do(func() {
		if r.cancelWork != nil {
			r.stopWorkDeadline = context.AfterFunc(workCtx, r.cancelWork)
		}
		if r.llmGateway != nil {
			r.llmGateway.Quiesce()
		}
		if r.restartCancel != nil {
			r.restartCancel()
		}
		if r.agentService != nil {
			r.agentService.Quiesce()
		}
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
			r.mailStatus.imapState("stopping", "")
			r.quiesceErr = r.imapServer.Close()
			if r.quiesceErr != nil {
				r.mailStatus.imapState("failed", "close_failed")
			} else {
				r.mailStatus.imapState("stopped", "")
			}
		}
	})
	return r.quiesceErr
}

// Drain waits for service-owned work and then closes shared handler
// dependencies. The caller must drain HTTP requests after Quiesce and before
// calling Drain. Database and Olric remain usable until Drain returns.
func (r *Runtime) Drain(ctx context.Context) error {
	var errs []error
	errs = append(errs, r.Quiesce(ctx))
	defer func() {
		if r.stopWorkDeadline != nil {
			r.stopWorkDeadline()
		}
		if r.cancelWork != nil {
			r.cancelWork()
		}
	}()
	if r.restartDone != nil {
		select {
		case <-r.restartDone:
		case <-ctx.Done():
			errs = append(errs, ctx.Err())
		}
	}
	if r.scheduler != nil {
		errs = append(errs, r.scheduler.Stop(ctx))
	}
	if r.ftpConnections != nil {
		r.ftpConnections.CloseAll()
	}
	if r.websocketServer != nil {
		errs = append(errs, r.websocketServer.Shutdown(ctx))
	}
	if r.agentService != nil {
		errs = append(errs, r.agentService.Drain(ctx))
	}
	if r.llmGateway != nil {
		errs = append(errs, r.llmGateway.Drain(ctx))
	}
	if r.rateLimitClient != nil {
		errs = append(errs, r.rateLimitClient.Close(ctx))
	}
	errs = append(errs, resource.ShutdownEventWorkerPool(ctx))
	if r.yjs != nil {
		errs = append(errs, r.yjs.Close(ctx))
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

// CheckReady includes the generation loaded before resource initialization.
func (r *Runtime) CheckReady(ctx context.Context) error {
	if err := r.restartState.Check(ctx); err != nil {
		return err
	}
	_, err := r.rateLimitClient.RoutingTable(ctx)
	return err
}

// A started local member may still advertise peers replacing their processes.
// Wait for a usable cluster client before composing services that require it.
func connectRuntimeCache(parent context.Context, address string) (*olric.ClusterClient, error) {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	retry := time.NewTicker(500 * time.Millisecond)
	defer retry.Stop()
	var last error
	for {
		if err := ctx.Err(); err != nil {
			return nil, errors.Join(err, last)
		}
		client, err := olric.NewClusterClient([]string{address}, olric.WithRoutingTableFetchInterval(time.Second))
		if err == nil {
			return client, nil
		}
		last = fmt.Errorf("connect Olric: %w", err)
		select {
		case <-ctx.Done():
			return nil, errors.Join(ctx.Err(), last)
		case <-retry.C:
		}
	}
}
