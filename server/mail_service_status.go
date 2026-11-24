package server

import (
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/artpar/go-guerrilla"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/permission"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
)

// Only lifecycle owners write observations. Persisted settings remain owned by
// resources/ConfigStore; this snapshot describes this process's active runtime.
type mailServiceRuntimeStatus struct {
	mu        sync.RWMutex
	runtimeID string
	daemon    *guerrilla.Daemon
	smtp      mailSMTPStatus
	imap      mailProtocolStatus
}

type mailProtocolStatus struct {
	State                    string    `json:"state"`
	Reason                   string    `json:"reason,omitempty"`
	ConfiguredEnabled        bool      `json:"configured_enabled"`
	Enabled                  bool      `json:"enabled"`
	ListenInterface          string    `json:"listen_interface,omitempty"`
	BoundAddress             string    `json:"bound_address,omitempty"`
	Hostname                 string    `json:"hostname,omitempty"`
	TLSMode                  string    `json:"tls_mode,omitempty"`
	CertificateLoaded        bool      `json:"certificate_loaded"`
	AuthenticationRequired   bool      `json:"authentication_required"`
	AuthenticationMechanisms []string  `json:"authentication_mechanisms"`
	ChangedAt                time.Time `json:"changed_at"`
	ErrorCode                string    `json:"error_code,omitempty"`
	ErrorMessage             string    `json:"error_message,omitempty"`
}

type mailSMTPStatus struct {
	State             string                     `json:"state"`
	Reason            string                     `json:"reason,omitempty"`
	ConfiguredEnabled bool                       `json:"configured_enabled"`
	Enabled           bool                       `json:"enabled"`
	ChangedAt         time.Time                  `json:"changed_at"`
	ErrorCode         string                     `json:"error_code,omitempty"`
	ErrorMessage      string                     `json:"error_message,omitempty"`
	Listeners         []guerrilla.ListenerStatus `json:"listeners"`
}

type mailRuntimeSnapshot struct {
	RuntimeID  string             `json:"runtime_id"`
	ObservedAt time.Time          `json:"observed_at"`
	SMTP       mailSMTPStatus     `json:"smtp"`
	IMAP       mailProtocolStatus `json:"imap"`
}

func newMailServiceRuntimeStatus(runtimeID string) *mailServiceRuntimeStatus {
	now := time.Now().UTC()
	return &mailServiceRuntimeStatus{runtimeID: runtimeID,
		smtp: mailSMTPStatus{State: "disabled", Reason: "no_enabled_listeners", ChangedAt: now, Listeners: []guerrilla.ListenerStatus{}},
		imap: mailProtocolStatus{State: "disabled", Reason: "backend_disabled", ChangedAt: now},
	}
}

func (s *mailServiceRuntimeStatus) setIMAP(status mailProtocolStatus) {
	status.AuthenticationMechanisms = append([]string{}, status.AuthenticationMechanisms...)
	s.mu.Lock()
	defer s.mu.Unlock()
	status.ChangedAt = s.imap.ChangedAt
	if s.imap.State != status.State || s.imap.ErrorCode != status.ErrorCode {
		status.ChangedAt = time.Now().UTC()
	}
	s.imap = status
}

func (s *mailServiceRuntimeStatus) imapState(state, code string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Quiesce must retain the failure that caused a listener to stop.
	if s.imap.State == "failed" && code == "" && (state == "stopping" || state == "stopped") {
		return
	}
	if s.imap.State != state || s.imap.ErrorCode != code {
		s.imap.ChangedAt = time.Now().UTC()
	}
	s.imap.State, s.imap.ErrorCode = state, code
	if state != "running" && state != "stopping" {
		s.imap.BoundAddress = ""
	}
}

func (s *mailServiceRuntimeStatus) imapStopped() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.imap.State == "failed" {
		return
	}
	code, state := "serve_failed", "failed"
	if s.imap.State == "stopping" || s.imap.State == "stopped" {
		code, state = "", "stopped"
	}
	if s.imap.State != state || s.imap.ErrorCode != code {
		s.imap.ChangedAt = time.Now().UTC()
	}
	s.imap.State, s.imap.ErrorCode, s.imap.BoundAddress = state, code, ""
}

func (s *mailServiceRuntimeStatus) snapshot() mailRuntimeSnapshot {
	s.mu.RLock()
	result := mailRuntimeSnapshot{RuntimeID: s.runtimeID, SMTP: s.smtp, IMAP: s.imap}
	daemon := s.daemon
	s.mu.RUnlock()
	result.IMAP.AuthenticationMechanisms = append([]string{}, result.IMAP.AuthenticationMechanisms...)
	if daemon != nil && result.SMTP.Enabled {
		result.SMTP.Listeners = daemon.ListenerStatuses()
		result.SMTP.State, result.SMTP.Reason = "disabled", "no_enabled_listeners"
		result.SMTP.ConfiguredEnabled, result.SMTP.Enabled = false, false
		priority := map[string]int{"disabled": 0, "stopped": 1, "running": 2, "starting": 3, "stopping": 4, "failed": 5}
		for _, listener := range result.SMTP.Listeners {
			if listener.Enabled {
				if listener.State == "failed" && result.SMTP.ErrorCode == "" {
					result.SMTP.ErrorCode = listener.ErrorCode
				}
				result.SMTP.ConfiguredEnabled, result.SMTP.Enabled = true, true
				result.SMTP.Reason = ""
				if priority[listener.State] > priority[result.SMTP.State] {
					result.SMTP.State = listener.State
				}
			}
			if listener.ChangedAt.After(result.SMTP.ChangedAt) {
				result.SMTP.ChangedAt = listener.ChangedAt
			}
		}
	}
	result.SMTP.ErrorMessage = mailServiceFailureMessage(result.SMTP.ErrorCode)
	result.IMAP.ErrorMessage = mailServiceFailureMessage(result.IMAP.ErrorCode)
	result.ObservedAt = time.Now().UTC()
	return result
}

// Fixed messages prevent configuration paths, keys, and backend error text from
// escaping through diagnostics. Full errors retain their existing server logs.
func mailServiceFailureMessage(code string) string {
	switch code {
	case "configuration_failed":
		return "Mail service configuration could not be loaded."
	case "certificate_failed", "tls_configuration_failed":
		return "Mail service TLS certificate could not be loaded."
	case "listen_failed":
		return "Mail listener could not bind its configured address."
	case "serve_failed", "accept_failed":
		return "Mail listener stopped unexpectedly."
	case "close_failed":
		return "Mail listener could not close cleanly."
	}
	return ""
}

type mailServiceStatusActionPerformer struct {
	status *mailServiceRuntimeStatus
	world  *resource.DbResource
}

func (*mailServiceStatusActionPerformer) Name() string { return "mail.service.status" }

func (p *mailServiceStatusActionPerformer) DoAction(_ actionresponse.Outcome, input map[string]interface{}, tx *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	denied := api2go.NewHTTPError(errors.New("administrator membership required"), "administrator membership required", http.StatusForbidden)
	active, _ := input["sessionUser"].(*auth.SessionUser)
	if active == nil || p.world == nil || p.world.AdministratorGroupId == daptinid.NullReferenceId {
		return nil, nil, []error{denied}
	}
	// Trusted action outcomes temporarily augment Groups. Resolve persisted
	// membership again so wrapper execution cannot grant diagnostic access.
	persisted, err := p.world.MailSessionUser(active, tx)
	if err != nil {
		return nil, nil, []error{err}
	}
	if !(permission.PermissionInstance{}).CanExecute(persisted.UserReferenceId, persisted.Groups, p.world.AdministratorGroupId) {
		return nil, nil, []error{denied}
	}
	return nil, []actionresponse.ActionResponse{resource.NewActionResponse("mail.service.status", p.status.snapshot())}, nil
}
