package resource

import (
	"errors"
	"net/http"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/jmoiron/sqlx"
)

type addressBookShareAction struct {
	cruds map[string]*DbResource
}

func NewAddressBookShareAction(cruds map[string]*DbResource) actionresponse.ActionPerformerInterface {
	return &addressBookShareAction{cruds: cruds}
}

func (*addressBookShareAction) Name() string { return "address_book.share" }

func (a *addressBookShareAction) base() *davShareAction {
	return &davShareAction{cruds: a.cruds, collectionTable: addressBookTable, objectTable: addressObjectTable, groupPrefix: "cardshare"}
}

func (a *addressBookShareAction) DoAction(_ actionresponse.Outcome, fields map[string]interface{}, tx *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	if tx == nil {
		return nil, nil, []error{errors.New("address book sharing requires an action transaction")}
	}
	caller, ok := fields["sessionUser"].(*auth.SessionUser)
	if !ok || caller == nil || caller.UserReferenceId == daptinid.NullReferenceId {
		return nil, nil, []error{api2go.NewHTTPError(errors.New("authentication required"), "authentication required", http.StatusUnauthorized)}
	}
	bookRef := daptinid.InterfaceToDIR(fields["address_book_id"])
	if bookRef == daptinid.NullReferenceId {
		return nil, nil, []error{api2go.NewHTTPError(errors.New("invalid address book reference ID"), "invalid address book reference ID", http.StatusBadRequest)}
	}
	grant, err := davSharePermission(fields["permission"])
	if err != nil {
		return nil, nil, []error{api2go.NewHTTPError(err, "invalid group permission", http.StatusBadRequest)}
	}
	share := a.base()
	groupRef := daptinid.InterfaceToDIR(fields["usergroup_id"])
	if target, present := fields["target_user_id"]; present {
		if groupRef != daptinid.NullReferenceId {
			return nil, nil, []error{api2go.NewHTTPError(errors.New("choose an account or group"), "choose an account or group", http.StatusBadRequest)}
		}
		accountRef := daptinid.InterfaceToDIR(target)
		if accountRef == daptinid.NullReferenceId {
			return nil, nil, []error{api2go.NewHTTPError(errors.New("invalid account reference ID"), "invalid account reference ID", http.StatusBadRequest)}
		}
		groupRef, err = share.shareWithUser(bookRef, accountRef, grant, caller, tx)
		if err != nil {
			return nil, nil, []error{err}
		}
		return nil, []actionresponse.ActionResponse{NewActionResponse("address_book.share", map[string]interface{}{
			"address_book_id": bookRef.String(), "user_account_id": accountRef.String(), "usergroup_id": groupRef.String(), "permission": grant,
		})}, nil
	}
	if groupRef == daptinid.NullReferenceId {
		return nil, nil, []error{api2go.NewHTTPError(errors.New("invalid group reference ID"), "invalid group reference ID", http.StatusBadRequest)}
	}
	if err := share.apply(bookRef, groupRef, grant, caller, tx); err != nil {
		return nil, nil, []error{err}
	}
	return nil, []actionresponse.ActionResponse{NewActionResponse("address_book.share", map[string]interface{}{
		"address_book_id": bookRef.String(), "usergroup_id": groupRef.String(), "permission": grant,
	})}, nil
}
