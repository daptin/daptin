package resource

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/artpar/go-imap"
	"github.com/artpar/go-imap/backend"
	"github.com/buraksezer/olric"
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

	// Brute force protection: check failed login count via Olric
	if OlricCache != nil {
		failKey := fmt.Sprintf("imap-fail-%s", username)
		val, err := OlricCache.Get(context.Background(), failKey)
		if err == nil && val != nil {
			if count, err := val.Int(); err == nil && count >= 5 {
				log.Printf("[IMAP] Login: user %s locked out due to too many failed attempts", username)
				return nil, errors.New("too many failed login attempts, try again later")
			}
		}
	}

	userAccountResource := be.cruds[USER_ACCOUNT_TABLE_NAME]
	transaction, err := userAccountResource.Connection().Beginx()
	if err != nil {
		CheckErr(err, "Failed to begin transaction [51]")
		return nil, err
	}
	defer transaction.Rollback()

	userMailAccount, err := userAccountResource.GetUserMailAccountRowByEmail(username, transaction)
	if err != nil {
		be.recordFailedLogin(username)
		return nil, err
	}

	userAccount, _, err := userAccountResource.GetSingleRowByReferenceIdWithTransaction("user_account",
		daptinid.InterfaceToDIR(userMailAccount["user_account_id"]), nil, transaction)
	if err != nil {
		return nil, fmt.Errorf("failed to get user account: %w", err)
	}
	userId, ok := userAccount["id"].(int64)
	if !ok {
		return nil, errors.New("invalid user account id")
	}
	groups := userAccountResource.GetObjectUserGroupsByWhereWithTransaction("user_account", transaction, "id", userId)

	sessionUser := &auth.SessionUser{
		UserId:          userId,
		UserReferenceId: daptinid.InterfaceToDIR(userAccount["reference_id"]),
		Groups:          groups,
	}

	mailPassword, _ := userMailAccount["password"].(string)
	if BcryptCheckStringHash(password, mailPassword) {
		transaction.Commit()

		// Clear failed login counter on success
		if OlricCache != nil {
			failKey := fmt.Sprintf("imap-fail-%s", username)
			OlricCache.Delete(context.Background(), failKey)
		}

		return &DaptinImapUser{
			username:               username,
			mailAccountId:          userMailAccount["id"].(int64),
			mailAccountReferenceId: daptinid.InterfaceToDIR(userMailAccount["reference_id"]).String(),
			dbResource:             be.cruds,
			sessionUser:            sessionUser,
		}, nil
	}

	be.recordFailedLogin(username)
	return nil, errors.New("bad username or password")
}

func (be *DaptinImapBackend) recordFailedLogin(username string) {
	if OlricCache == nil {
		return
	}
	failKey := fmt.Sprintf("imap-fail-%s", username)
	val, err := OlricCache.Get(context.Background(), failKey)
	count := 0
	if err == nil && val != nil {
		count, _ = val.Int()
	}
	count++
	OlricCache.Put(context.Background(), failKey, count, olric.EX(5*time.Minute))
}

func NewImapServer(cruds map[string]*DbResource) *DaptinImapBackend {
	return &DaptinImapBackend{
		cruds: cruds,
	}
}
