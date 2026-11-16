package resource

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
)

const RequireCurrentMembershipActionName = "api_member.require_current"

// findActiveAPIMember is the membership selection authority shared by quota
// accounting and explicit eligibility checks. Selection itself is not a gate.
func findActiveAPIMember(userID int64, tx *sqlx.Tx) (map[string]interface{}, error) {
	query, args, err := statementbuilder.Squirrel.Select("*").Prepared(true).From("api_member").
		Where(goqu.Ex{"user_account_id": userID, "status": "active"}).Order(goqu.I("id").Desc()).Limit(1).ToSQL()
	if err != nil {
		return nil, err
	}
	return querySingleMeteringRow("api_member", tx, query, args...)
}

func apiMemberCurrent(member map[string]interface{}, now time.Time) bool {
	if StringOrEmpty(member["status"]) != "active" {
		return false
	}
	start := toTime(member["period_start"])
	if start.IsZero() || start.After(now) {
		return false
	}
	if member["period_end"] == nil {
		return true
	}
	end := toTime(member["period_end"])
	return !end.IsZero() && end.After(now)
}

type requireCurrentMembershipAction struct{}

func NewRequireCurrentMembershipAction() actionresponse.ActionPerformerInterface {
	return &requireCurrentMembershipAction{}
}

func (*requireCurrentMembershipAction) Name() string { return RequireCurrentMembershipActionName }

// DoAction reads the active server-created identity and durable membership in
// the owning action transaction. Inputs and transient action groups cannot
// select a different account, membership, plan, or eligibility policy.
func (*requireCurrentMembershipAction) DoAction(_ actionresponse.Outcome, fields map[string]interface{}, tx *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	deny := func() (api2go.Responder, []actionresponse.ActionResponse, []error) {
		return nil, nil, []error{api2go.NewHTTPError(errors.New("current API membership required"), "current API membership required", http.StatusForbidden)}
	}
	user, ok := fields["sessionUser"].(*auth.SessionUser)
	if !ok || user == nil || user.UserReferenceId == daptinid.NullReferenceId {
		return deny()
	}
	if tx == nil {
		return nil, nil, []error{errors.New("membership preflight requires an action transaction")}
	}
	userID, err := GetReferenceIdToIdWithTransaction(USER_ACCOUNT_TABLE_NAME, user.UserReferenceId, tx)
	if errors.Is(err, sql.ErrNoRows) {
		return deny()
	}
	if err != nil {
		return nil, nil, []error{err}
	}
	member, err := findActiveAPIMember(userID, tx)
	if errors.Is(err, errMeteringRowNotFound) {
		return deny()
	}
	if err != nil {
		return nil, nil, []error{err}
	}
	if !apiMemberCurrent(member, time.Now().UTC()) {
		return deny()
	}
	planID, err := ResourceRowInt64(member["api_plan_id"])
	if err != nil || planID <= 0 {
		return deny()
	}
	planRef, err := GetIdToReferenceIdWithTransaction("api_plan", planID, tx)
	if errors.Is(err, sql.ErrNoRows) {
		return deny()
	}
	if err != nil {
		return nil, nil, []error{err}
	}
	if planRef == daptinid.NullReferenceId {
		return deny()
	}
	return nil, nil, nil
}
