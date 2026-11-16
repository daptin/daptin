package resource

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/table_info"
	"github.com/google/uuid"
)

func TestMeteringDeadlinePreservesQuotaAndSkipsStaleRecovery(t *testing.T) {
	db, cruds, user := newCanonicalMeteringDatabase(t)
	start := time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC)
	now := start
	insertMeteringPlanAndMember(t, cruds, user, start, `[{"metric":"requests","window":"day","maximum":1,"mode":"hard"}]`)
	service := NewMeteringService(&cruds)
	service.now = func() time.Time { return now }
	config := &table_info.MeteringConfig{Enabled: true, MeterType: "requests", CostExpr: "1"}
	tx, err := db.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	decision, err := service.Admit(MeteringContext{User: user, RequestID: "bounded-work", Metering: config}, tx)
	if err != nil {
		t.Fatal(err)
	}
	if err = service.EnsureReservationDeadline(MeteringContext{User: user}, decision, start.Add(10*time.Minute), tx); err != nil {
		t.Fatal(err)
	}
	assertMeteringBucket(t, service, tx, "requests", "", 1, 0)
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	now = start.Add(4 * time.Minute)
	tx, err = db.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	if err = service.EnsureReservationDeadline(MeteringContext{User: user}, decision, now.Add(time.Second), tx); err != nil {
		t.Fatal(err)
	}
	usage, err := service.findUsageByToken(decision.ReservationToken, tx)
	if err != nil {
		t.Fatal(err)
	}
	if got := toTime(usage["reservation_expires_at"]); !got.Equal(start.Add(15 * time.Minute)) {
		t.Fatalf("lease shortened: %v", got)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	// The token represents a recovery candidate selected using the old deadline.
	now = start.Add(6 * time.Minute)
	tx, err = db.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	expired, err := service.expireReservation(decision.ReservationToken, now, tx)
	if err != nil || expired {
		t.Fatalf("stale candidate expired protected work: %v %v", expired, err)
	}
	assertMeteringBucket(t, service, tx, "requests", "", 1, 0)
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	now = start.Add(11 * time.Minute)
	tx, err = db.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err = service.Complete(MeteringContext{User: user, Metering: config, StatusCode: 200}, decision, tx); err != nil {
		t.Fatal(err)
	}
	if err = service.Complete(MeteringContext{User: user, Metering: config, StatusCode: 200}, decision, tx); err != nil {
		t.Fatal(err)
	}
	if expired, err = service.expireReservation(decision.ReservationToken, start.Add(time.Hour), tx); err != nil || expired {
		t.Fatalf("recovery changed completed work: %v %v", expired, err)
	}
	assertMeteringBucket(t, service, tx, "requests", "", 0, 1)
}

func TestMeteringDeadlineFollowsSuccessiveOutcomesAndRecoversOrphans(t *testing.T) {
	db, cruds, user := newCanonicalMeteringDatabase(t)
	start := time.Date(2026, 11, 17, 11, 0, 0, 0, time.UTC)
	now := start
	insertMeteringPlanAndMember(t, cruds, user, start, `[{"metric":"requests","window":"day","maximum":1,"mode":"hard"}]`)
	service := NewMeteringService(&cruds)
	service.now = func() time.Time { return now }
	config := &table_info.MeteringConfig{Enabled: true}
	tx, err := db.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	decision, err := service.Admit(MeteringContext{User: user, RequestID: "workflow-orphan", Metering: config}, tx)
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	for _, elapsed := range []time.Duration{0, 170 * time.Second, 340 * time.Second} {
		now = start.Add(elapsed)
		tx, err = db.Beginx()
		if err != nil {
			t.Fatal(err)
		}
		if err = service.EnsureReservationDeadline(MeteringContext{User: user}, decision, now.Add(180*time.Second), tx); err != nil {
			t.Fatal(err)
		}
		if expired, err := service.expireReservation(decision.ReservationToken, now, tx); err != nil || expired {
			t.Fatalf("workflow expired: %v %v", expired, err)
		}
		assertMeteringBucket(t, service, tx, "requests", "", 1, 0)
		if err = tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	now = start.Add(821 * time.Second)
	tx, err = db.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	expired, err := service.expireReservation(decision.ReservationToken, now, tx)
	if err != nil || !expired {
		t.Fatalf("orphan not recovered: %v %v", expired, err)
	}
	expired, err = service.expireReservation(decision.ReservationToken, now, tx)
	if err != nil || expired {
		t.Fatalf("duplicate recovery: %v %v", expired, err)
	}
	if err = service.EnsureReservationDeadline(MeteringContext{User: user}, decision, now.Add(time.Minute), tx); !errors.Is(err, errMeteringRequestTerminal) {
		t.Fatalf("expired work revived: %v", err)
	}
	if err = service.Complete(MeteringContext{User: user, Metering: config}, decision, tx); !errors.Is(err, errMeteringRequestTerminal) {
		t.Fatalf("expired work completed: %v", err)
	}
	assertMeteringBucket(t, service, tx, "requests", "", 0, 0)
}

func TestMeteringDeadlineFailsClosed(t *testing.T) {
	for _, test := range []string{"owner", "token", "request", "past deadline", "zero deadline", "elapsed lease", "missing expiry", "expired", "cancelled", "completed"} {
		t.Run(test, func(t *testing.T) {
			db, cruds, user := newCanonicalMeteringDatabase(t)
			now := time.Now().UTC()
			insertMeteringPlanAndMember(t, cruds, user, now, `[{"metric":"requests","window":"day","maximum":1,"mode":"hard"}]`)
			service := NewMeteringService(&cruds)
			service.now = func() time.Time { return now }
			config := &table_info.MeteringConfig{Enabled: true}
			tx, err := db.Beginx()
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			decision, err := service.Admit(MeteringContext{User: user, RequestID: "denied", Metering: config}, tx)
			if err != nil {
				t.Fatal(err)
			}
			owner := *user
			copyDecision := *decision
			deadline := now.Add(time.Minute)
			switch test {
			case "owner":
				owner.UserId = 999
			case "token":
				copyDecision.ReservationToken = uuid.NewString()
			case "request":
				copyDecision.RequestID = "different"
			case "past deadline":
				deadline = now.Add(-time.Second)
			case "zero deadline":
				deadline = time.Time{}
			case "elapsed lease":
				now = now.Add(6 * time.Minute)
				deadline = now.Add(time.Minute)
			case "missing expiry":
				usage, lookupErr := service.findUsageByToken(decision.ReservationToken, tx)
				if lookupErr != nil {
					t.Fatal(lookupErr)
				}
				model := api2go.NewApi2GoModelWithData("api_usage", nil, 0, nil, usage)
				model.SetAttributes(map[string]interface{}{"reservation_expires_at": nil})
				request := httptest.NewRequest(http.MethodPatch, "/api_usage", nil).WithContext(context.WithValue(context.Background(), "user", service.internalUser(user)))
				_, err = cruds["api_usage"].UpdateWithoutFilters(model, api2go.Request{PlainRequest: request}, tx)
			case "expired":
				now = now.Add(6 * time.Minute)
				deadline = now.Add(time.Minute)
				_, err = service.ExpireReservations(now, 10, tx)
			case "cancelled":
				err = service.Cancel(MeteringContext{User: user, Measures: map[string]int64{"requests": 0, "bytes": 0}}, decision, tx)
			case "completed":
				err = service.Complete(MeteringContext{User: user, Metering: config}, decision, tx)
			}
			if err != nil {
				t.Fatal(err)
			}
			before, err := service.findUsageByToken(decision.ReservationToken, tx)
			if err != nil {
				t.Fatal(err)
			}
			if err = service.EnsureReservationDeadline(MeteringContext{User: &owner}, &copyDecision, deadline, tx); err == nil {
				t.Fatal("invalid protection succeeded")
			}
			after, err := service.findUsageByToken(decision.ReservationToken, tx)
			if err != nil {
				t.Fatal(err)
			}
			if before["state"] != after["state"] || !toTime(before["reservation_expires_at"]).Equal(toTime(after["reservation_expires_at"])) {
				t.Fatal("denial mutated reservation")
			}
			reserved, consumed := int64(1), int64(0)
			if test == "expired" || test == "cancelled" {
				reserved = 0
			}
			if test == "completed" {
				reserved, consumed = 0, 1
			}
			assertMeteringBucket(t, service, tx, "requests", "", reserved, consumed)

		})
	}
}

func TestActionDeadlineUsesOriginalOwnerAndSkipsInternalWork(t *testing.T) {
	db, cruds, user := newCanonicalMeteringDatabase(t)
	now := time.Now().UTC()
	service := NewMeteringService(&cruds)
	service.now = func() time.Time { return now }
	tx, err := db.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	// Capture-only admission must protect its usage row without plan consumption.
	decision, err := service.Admit(MeteringContext{User: user, RequestID: "capture-deadline", Metering: &table_info.MeteringConfig{Capture: &table_info.PayloadCaptureConfig{Enabled: true}}}, tx)
	if err != nil {
		t.Fatal(err)
	}
	active := &auth.SessionUser{UserId: 999}
	req := httptest.NewRequest(http.MethodPost, "/action/integration/workflow", nil)
	req = req.WithContext(context.WithValue(context.WithValue(req.Context(), actionReservationContextKey{}, &actionReservation{service: service, decision: decision, owner: user}), "user", active))
	if err = EnsureActionReservationDeadline(req, now.Add(10*time.Minute), tx); err != nil {
		t.Fatal(err)
	}
	usage, err := service.findUsageByToken(decision.ReservationToken, tx)
	if err != nil {
		t.Fatal(err)
	}
	if got := toTime(usage["reservation_expires_at"]); !got.Equal(now.Add(15 * time.Minute)) {
		t.Fatalf("capture lease not protected: %v", got)
	}
	owner, err := ResourceRowInt64(usage["user_account_id"])
	if err != nil || owner != user.UserId {
		t.Fatalf("owner changed: %v %v", owner, err)
	}
	if err = service.Complete(MeteringContext{User: user}, decision, tx); err != nil {
		t.Fatal(err)
	}
	internal := req.WithContext(WithMeteringInternal(req.Context()))
	if err = EnsureActionReservationDeadline(internal, now.Add(time.Hour), tx); err != nil {
		t.Fatalf("post-metering work inherited terminal admission: %v", err)
	}
	if err = EnsureActionReservationDeadline(req, now.Add(time.Hour), tx); err == nil {
		t.Fatal("terminal admission reused")
	}
	if err = EnsureActionReservationDeadline(httptest.NewRequest(http.MethodPost, "/", nil), now.Add(time.Hour), nil); err != nil {
		t.Fatal(err)
	}
}
