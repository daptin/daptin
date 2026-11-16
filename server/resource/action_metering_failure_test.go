package resource

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/daptin/daptin/server/table_info"
)

func TestFailedActionCancelsOnlyDurableAdmission(t *testing.T) {
	for _, mode := range []string{"rollback", "committed", "completed", "capture only"} {
		t.Run(mode, func(t *testing.T) {
			database, cruds, user := newCanonicalMeteringDatabase(t)
			service := NewMeteringService(&cruds)
			config := &table_info.MeteringConfig{Enabled: true, MeterType: "credits", CostExpr: "7"}
			if mode == "capture only" {
				config = &table_info.MeteringConfig{Capture: &table_info.PayloadCaptureConfig{Enabled: true}}
			} else {
				insertMeteringPlanAndMember(t, cruds, user, time.Now().UTC().Add(-time.Minute), `[{"metric":"credits","window":"day","maximum":100,"mode":"hard"},{"metric":"requests","window":"day","maximum":100,"mode":"hard"}]`)
			}
			ctx := MeteringContext{User: user, Metering: config, StatusCode: http.StatusBadGateway, ErrorMessage: "action failed"}
			tx, err := database.Beginx()
			if err != nil {
				t.Fatal(err)
			}
			decision, err := service.Admit(ctx, tx)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "completed" {
				if err = service.Complete(ctx, decision, tx); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "rollback" {
				err = tx.Rollback()
			} else {
				err = tx.Commit()
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = service.cancelFailedAction(ctx, decision); err != nil {
				t.Fatal(err)
			}
			// Cancellation is idempotent and never turns completed work into free work.
			if err = service.cancelFailedAction(ctx, decision); err != nil {
				t.Fatal(err)
			}
			verify, err := database.Beginx()
			if err != nil {
				t.Fatal(err)
			}
			defer verify.Rollback()
			usage, err := service.findUsageByToken(decision.ReservationToken, verify)
			if mode == "rollback" {
				if !errors.Is(err, errMeteringRowNotFound) {
					t.Fatalf("rolled-back admission survived: %#v %v", usage, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			state := meteringStateCancelled
			if mode == "completed" {
				state = meteringStateCompleted
			}
			if usage["state"] != state {
				t.Fatalf("state=%v, want %s", usage["state"], state)
			}
			if mode != "completed" {
				status, _ := ResourceRowInt64(usage["status_code"])
				if status != 502 || usage["terminal_at"] == nil {
					t.Fatalf("missing failure terminal data: %#v", usage)
				}
				var measures map[string]int64
				if err = json.UnmarshalFromString(StringOrEmpty(usage["measures"]), &measures); err != nil {
					t.Fatal(err)
				}
				for metric, amount := range measures {
					if amount != 0 {
						t.Fatalf("failed action consumed %s=%d", metric, amount)
					}
				}
			}
			if mode != "capture only" {
				credits, requests := int64(0), int64(0)
				if mode == "completed" {
					credits = 7
					requests = 1
				}
				assertMeteringBucket(t, service, verify, "credits", "", 0, credits)
				assertMeteringBucket(t, service, verify, "requests", "", 0, requests)
			}
		})
	}
}
