package resource

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
)

func TestAPIMemberCurrentWindow(t *testing.T) {
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name, status string
		start, end   interface{}
		want         bool
	}{
		{"current", "active", now.Add(-time.Hour), now.Add(time.Hour), true},
		{"inclusive start", "active", now, now.Add(time.Hour), true},
		{"exclusive end", "active", now.Add(-time.Hour), now, false},
		{"future", "active", now.Add(time.Second), now.Add(time.Hour), false},
		{"expired", "active", now.Add(-time.Hour), now.Add(-time.Second), false},
		{"cancelled", "cancelled", now.Add(-time.Hour), now.Add(time.Hour), false},
		{"open ended", "active", now.Add(-time.Hour), nil, true},
		{"missing start", "active", nil, nil, false},
		{"malformed start", "active", "invalid", nil, false},
		{"malformed end", "active", now, "invalid", false},
		{"empty end", "active", now, "", false},
		{"timestamp strings", "active", now.Format(time.RFC3339), []byte(now.Add(time.Hour).Format(time.RFC3339)), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := apiMemberCurrent(map[string]interface{}{"status": test.status, "period_start": test.start, "period_end": test.end}, now); got != test.want {
				t.Fatalf("current = %v, want %v", got, test.want)
			}
		})
	}
}

func TestRequireCurrentMembershipUsesPersistedActiveAccount(t *testing.T) {
	database, cruds, user := newCanonicalMeteringDatabase(t)
	performer := NewRequireCurrentMembershipAction()
	check := func(identity interface{}, want bool) {
		t.Helper()
		tx, err := database.Beginx()
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		_, responses, failures := performer.DoAction(actionresponse.Outcome{}, map[string]interface{}{
			"sessionUser": identity, "user_reference_id": user.UserReferenceId.String(), "status": "active", "require_membership": false,
		}, tx)
		if (len(failures) == 0) != want {
			t.Fatalf("allowed=%v, want %v; errors=%v", len(failures) == 0, want, failures)
		}
		if !want {
			var httpErr api2go.HTTPError
			if len(failures) != 1 || !errors.As(failures[0], &httpErr) || httpErr.Status() != http.StatusForbidden {
				t.Fatalf("membership denial must be HTTP 403: %v", failures)
			}
		}
		if len(responses) != 0 {
			t.Fatalf("preflight exposed membership: %#v", responses)
		}
	}
	check(nil, false)
	transientAdmin := *user
	transientAdmin.Groups = auth.GroupPermissionList{{GroupReferenceId: cruds["api_member"].AdministratorGroupId}}
	check(&transientAdmin, false)
	now := time.Now().UTC().Add(-time.Minute)
	plan := insertMeteringPlanAndMember(t, cruds, user, now, `[]`)
	check(user, true)
	// Plan archival hides a plan from new purchases; it does not cancel an
	// existing membership. Exercise that distinction through the resource API.
	archiveTx, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	archive := api2go.NewApi2GoModelWithData("api_plan", nil, 0, nil, map[string]interface{}{"archived_at": time.Now().UTC()})
	archive.SetID(plan.String())
	archiveRequest := api2go.Request{PlainRequest: (&http.Request{Method: http.MethodPatch, URL: &url.URL{Path: "/api/api_plan/" + plan.String()}}).WithContext(context.WithValue(context.Background(), "user", &transientAdmin))}
	if _, err = cruds["api_plan"].UpdateWithoutFilters(archive, archiveRequest, archiveTx); err != nil {
		t.Fatal(err)
	}
	if err = archiveTx.Commit(); err != nil {
		t.Fatal(err)
	}
	check(user, true)
	check(&auth.SessionUser{}, false)
	// A forged numeric ID cannot change the public reference selected by the server.
	other := *user
	other.UserId = 1
	check(&other, true)
	// Selection remains the same as metering: a newer future active row does not
	// silently fall back to an older valid membership.
	insertMeteringPlanAndMember(t, cruds, user, time.Now().UTC().Add(time.Hour), `[]`)
	check(user, false)
	tx, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	member, err := findActiveAPIMember(user.UserId, tx)
	if err != nil {
		t.Fatal(err)
	}
	update := api2go.NewApi2GoModelWithData("api_member", nil, 0, nil, map[string]interface{}{"status": "cancelled"})
	update.SetID(daptinid.InterfaceToDIR(member["reference_id"]).String())
	request := api2go.Request{PlainRequest: (&http.Request{Method: http.MethodPatch, URL: &url.URL{Path: "/api/api_member"}}).WithContext(context.WithValue(context.Background(), "user", &transientAdmin))}
	if _, err = cruds["api_member"].UpdateWithoutFilters(update, request, tx); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	check(user, true)
	// Closed database transactions fail instead of granting access.
	tx, err = database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	_ = tx.Rollback()
	if _, _, failures := performer.DoAction(actionresponse.Outcome{}, map[string]interface{}{"sessionUser": user}, tx); len(failures) == 0 {
		t.Fatal("closed transaction allowed membership")
	}
}
