package llm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/buraksezer/olric"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	gateway "github.com/daptin/llmgateway"
	"github.com/daptin/llmgateway/adapter"
	"github.com/daptin/llmgateway/adapter/openaicompat"
	"github.com/daptin/llmgateway/catalog"
	"github.com/daptin/llmgateway/contract"
	"github.com/daptin/llmgateway/guardrail"
	"github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
)

type Gateway struct {
	engine             *gateway.Engine
	handler            http.Handler
	reloadCancel       context.CancelFunc
	reloadDone         chan struct{}
	reloadSubscription *redis.PubSub
	drainOnce          sync.Once
	drainErr           error
}

func NewGateway(ctx context.Context, cruds map[string]*resource.DbResource, olricClient *olric.EmbeddedClient) (*Gateway, error) {
	if ctx == nil {
		return nil, errors.New("LLM gateway requires a lifecycle context")
	}
	for _, resourceName := range []string{
		"world", "credential", "llm_provider", "llm_model", "llm_deployment", "api_plan", "api_member", "api_usage", "api_quota",
	} {
		if cruds[resourceName] == nil {
			return nil, fmt.Errorf("LLM gateway requires canonical Daptin resource %q", resourceName)
		}
	}
	if olricClient == nil {
		return nil, errors.New("LLM gateway requires Daptin's Olric client")
	}
	counters, err := olricClient.NewDMap("llmgateway-counters")
	if err != nil {
		return nil, fmt.Errorf("create LLM gateway counter map: %w", err)
	}
	leases, err := olricClient.NewDMap("llmgateway-leases")
	if err != nil {
		return nil, fmt.Errorf("create LLM gateway lease map: %w", err)
	}
	cache, err := olricClient.NewDMap("llmgateway-cache")
	if err != nil {
		return nil, fmt.Errorf("create LLM gateway cache map: %w", err)
	}

	adapters := adapter.NewRegistry()
	compatible := openaicompat.Factory{}
	for _, providerType := range []string{"openai-compatible", "openai", "openrouter", "lilac", "google"} {
		if err := adapters.Register(providerType, compatible); err != nil {
			return nil, err
		}
	}
	engine, err := gateway.New(gateway.Dependencies{
		Catalog: &daptinCatalog{cruds: cruds}, Secrets: daptinSecrets{cruds: cruds}, Adapters: adapters,
		Authorizer: daptinAuthorizer{cruds: cruds}, Metering: daptinMetering{cruds: cruds, service: resource.NewMeteringService(&cruds)},
		Counters: olricCounterStore{values: counters, leases: leases}, Cache: olricResponseCache{values: cache},
		Guardrails: guardrail.NewRegistry(), Telemetry: daptinTelemetry{}, Selector: gateway.RandomSelector{}, Clock: gateway.SystemClock{},
	}, gateway.Options{})
	if err != nil {
		return nil, err
	}
	handler, err := engine.Handler(gateway.HTTPOptions{Authenticator: daptinAuthenticator{}})
	if err != nil {
		return nil, err
	}
	if err := engine.Reload(ctx); err != nil {
		return nil, fmt.Errorf("load LLM gateway catalog: %w", err)
	}
	gatewayHost := &Gateway{engine: engine, handler: handler}
	gatewayHost.startReloadWatcher(ctx, cruds["world"].PubSub)
	return gatewayHost, nil
}

func (gatewayHost *Gateway) Handler() http.Handler {
	return gatewayHost.handler
}

func (gatewayHost *Gateway) Invoke(ctx context.Context, user *auth.SessionUser, request contract.Request) (contract.Response, error) {
	if user == nil {
		return contract.Response{}, errors.New("LLM invocation requires an authenticated user")
	}
	ctx = context.WithValue(ctx, "user", user)
	return gatewayHost.engine.Invoke(ctx, gatewayPrincipal(user), request)
}

func (gatewayHost *Gateway) Reload(ctx context.Context) error {
	return gatewayHost.engine.Reload(ctx)
}

func (gatewayHost *Gateway) Status() gateway.Status {
	return gatewayHost.engine.Status()
}

func (gatewayHost *Gateway) Drain(ctx context.Context) error {
	gatewayHost.drainOnce.Do(func() {
		if gatewayHost.reloadCancel != nil {
			gatewayHost.reloadCancel()
		}
		if gatewayHost.reloadSubscription != nil {
			gatewayHost.drainErr = gatewayHost.reloadSubscription.Close()
		}
		if gatewayHost.reloadDone != nil {
			select {
			case <-gatewayHost.reloadDone:
			case <-ctx.Done():
				gatewayHost.drainErr = errors.Join(gatewayHost.drainErr, ctx.Err())
			}
		}
		gatewayHost.drainErr = errors.Join(gatewayHost.drainErr, gatewayHost.engine.Drain(ctx))
	})
	return gatewayHost.drainErr
}

func (gatewayHost *Gateway) startReloadWatcher(parent context.Context, pubsub *olric.PubSub) {
	ctx, cancel := context.WithCancel(parent)
	gatewayHost.reloadCancel = cancel
	gatewayHost.reloadDone = make(chan struct{})
	var events <-chan *redis.Message
	if pubsub != nil {
		gatewayHost.reloadSubscription = pubsub.Subscribe(ctx, "credential", "llm_provider", "llm_model", "llm_deployment")
		events = gatewayHost.reloadSubscription.Channel()
	}
	go func() {
		defer close(gatewayHost.reloadDone)
		poll := time.NewTicker(3 * time.Second)
		defer poll.Stop()
		debounce := time.NewTimer(time.Hour)
		if !debounce.Stop() {
			<-debounce.C
		}
		defer debounce.Stop()
		var debounceChannel <-chan time.Time
		reload := func() {
			reloadContext, reloadCancel := context.WithTimeout(ctx, 30*time.Second)
			err := gatewayHost.Reload(reloadContext)
			reloadCancel()
			if err != nil && !errors.Is(err, catalog.ErrStaleRevision) && !errors.Is(err, context.Canceled) {
				log.WithError(err).Error("[llm] catalog reload rejected; retaining active snapshot")
			}
		}
		for {
			select {
			case <-ctx.Done():
				return
			case _, open := <-events:
				if !open {
					events = nil
					continue
				}
				if !debounce.Stop() && debounceChannel != nil {
					select {
					case <-debounce.C:
					default:
					}
				}
				debounce.Reset(250 * time.Millisecond)
				debounceChannel = debounce.C
			case <-debounceChannel:
				debounceChannel = nil
				reload()
			case <-poll.C:
				reload()
			}
		}
	}()
}

type daptinAuthenticator struct{}

func (daptinAuthenticator) Authenticate(ctx context.Context, _ string) (contract.Principal, error) {
	user, err := daptinSessionUser(ctx)
	if err != nil {
		return contract.Principal{}, err
	}
	return gatewayPrincipal(user), nil
}

func gatewayPrincipal(user *auth.SessionUser) contract.Principal {
	owner := contract.ID(user.UserReferenceId.String())
	groups := make([]contract.ID, 0, len(user.Groups))
	for _, group := range user.Groups {
		if group.GroupReferenceId != daptinid.NullReferenceId {
			groups = append(groups, contract.ID(group.GroupReferenceId.String()))
		}
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i] < groups[j] })
	return contract.Principal{KeyID: owner, OwnerID: owner, GroupIDs: groups}
}

type daptinTelemetry struct{}

func (daptinTelemetry) Record(_ context.Context, event gateway.TelemetryEvent) {
	fields := log.Fields{"event": event.Name, "request_id": event.RequestID, "revision": event.Revision}
	for key, value := range event.Attributes {
		fields[key] = value
	}
	for key, value := range event.Measures {
		fields[key] = value
	}
	log.WithFields(fields).Debug("LLM gateway event")
}
