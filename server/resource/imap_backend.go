package resource

import (
	"errors"
	"github.com/artpar/go-imap"
	"github.com/artpar/go-imap/backend"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	log "github.com/sirupsen/logrus"
)

type DaptinImapBackend struct {
	cruds map[string]*DbResource
}

func (be *DaptinImapBackend) LoginMd5(conn *imap.ConnInfo, username, challenge string, response string) (backend.User, error) {

	//userMailAccount, err := be.cruds[USER_ACCOUNT_TABLE_NAME].GetUserMailAccountRowByEmail(username)
	//if err != nil {
	//	return nil, err
	//}

	//userAccount, _, err := be.cruds[USER_ACCOUNT_TABLE_NAME].GetSingleRowByReferenceId("user_account", userMailAccount["user_account_id"].(string))
	//userId, _ := userAccount["id"].(int64)
	//groups := be.cruds[USER_ACCOUNT_TABLE_NAME].GetObjectUserGroupsByWhere("user_account", "id", userId)

	//sessionUser := &auth.SessionUser{
	//	UserId:          userId,
	//	UserReferenceId: userAccount["reference_id"].(string),
	//	Groups:          groups,
	//}

	//if HmacCheckStringHash(response, challenge, userMailAccount["password_md5"].(string)) {
	//
	//	return &DaptinImapUser{
	//		username:               username,
	//		mailAccountId:          userMailAccount["id"].(int64),
	//		mailAccountReferenceId: userMailAccount["reference_id"].(string),
	//		dbResource:             be.cruds,
	//		sessionUser:            sessionUser,
	//	}, nil
	//}

	return nil, errors.New("md5 based login not supported")

}

func (be *DaptinImapBackend) Login(conn *imap.ConnInfo, username, password string) (backend.User, error) {
	log.Printf("[IMAP] Login: starting for user %s", username)
	userAccountResource := be.cruds[USER_ACCOUNT_TABLE_NAME]
	log.Printf("[IMAP] Login: attempting to begin transaction")
	transaction, err := userAccountResource.Connection().Beginx()
	if err != nil {
		log.Printf("[IMAP] Login: failed to begin transaction: %v", err)
		CheckErr(err, "Failed to begin transaction [51]")
		return nil, err
	}
	log.Printf("[IMAP] Login: transaction started")
	defer func() {
		transaction.Commit()
		log.Printf("[IMAP] Login: transaction committed")
	}()
	userMailAccount, err := userAccountResource.GetUserMailAccountRowByEmail(username, transaction)
	if err != nil {
		return nil, err
	}

	userAccount, _, err := userAccountResource.GetSingleRowByReferenceIdWithTransaction("user_account",
		daptinid.InterfaceToDIR(userMailAccount["user_account_id"]), nil, transaction)
	userId, _ := userAccount["id"].(int64)
	groups := userAccountResource.GetObjectUserGroupsByWhereWithTransaction("user_account", transaction, "id", userId)

	sessionUser := &auth.SessionUser{
		UserId:          userId,
		UserReferenceId: daptinid.InterfaceToDIR(userAccount["reference_id"]),
		Groups:          groups,
	}

	if BcryptCheckStringHash(password, userMailAccount["password"].(string)) {

		return &DaptinImapUser{
			username:               username,
			mailAccountId:          userMailAccount["id"].(int64),
			mailAccountReferenceId: daptinid.InterfaceToDIR(userMailAccount["reference_id"]).String(),
			dbResource:             be.cruds,
			sessionUser:            sessionUser,
		}, nil
	}

	return nil, errors.New("bad username or password")
}

func NewImapServer(cruds map[string]*DbResource) *DaptinImapBackend {
	return &DaptinImapBackend{
		cruds: cruds,
	}
}
