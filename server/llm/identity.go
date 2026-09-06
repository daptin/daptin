package llm

import (
	"errors"
	"fmt"

	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/llmgateway/contract"
)

type daptinIdentityResolver struct {
	users *resource.DbResource
}

func (resolver daptinIdentityResolver) Resolve(user *auth.SessionUser) (contract.Principal, error) {
	if user == nil || user.UserId <= 0 || user.UserReferenceId == daptinid.NullReferenceId {
		return contract.Principal{}, errors.New("authenticated Daptin session is required")
	}
	if resolver.users == nil {
		return contract.Principal{}, errors.New("user_account resource is unavailable")
	}
	transaction, err := resolver.users.Connection().Beginx()
	if err != nil {
		return contract.Principal{}, fmt.Errorf("begin LLM identity resolution: %w", err)
	}
	defer transaction.Rollback()

	groups := resolver.users.GetObjectUserGroupsByWhereWithTransaction(
		resource.USER_ACCOUNT_TABLE_NAME, transaction, "id", user.UserId,
	)
	if err := transaction.Commit(); err != nil {
		return contract.Principal{}, fmt.Errorf("commit LLM identity resolution: %w", err)
	}

	return gatewayPrincipal(&auth.SessionUser{
		UserReferenceId: user.UserReferenceId,
		Groups:          groups,
	}), nil
}
