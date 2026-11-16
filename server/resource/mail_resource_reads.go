package resource

import (
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/jmoiron/sqlx"
)

func mailHasReadParents(table string) bool {
	return table == "mail" || table == "mail_box"
}

func mailHasScopedReads(table string) bool {
	return table == "mail_account" || mailHasReadParents(table)
}

func (db *DbResource) isMailReadAdministrator(caller *auth.SessionUser) bool {
	for _, group := range caller.Groups {
		if group.GroupReferenceId != daptinid.NullReferenceId && group.GroupReferenceId == db.AdministratorGroupId {
			return true
		}
	}
	return false
}

func (db *DbResource) checkMailResourceRead(table string, ref daptinid.DaptinReferenceId, caller *auth.SessionUser, tx *sqlx.Tx) error {
	if table == "mail_box" {
		_, _, err := db.readMailBoxForSession(ref, caller, tx)
		return err
	}
	persisted, err := db.MailSessionUser(caller, tx)
	if err != nil {
		return err
	}
	if table == "mail" {
		_, err = db.readMailMetadataForPersistedSession(ref, persisted, tx)
	} else {
		_, err = db.readMailResourceForSession("mail_account", ref, persisted, tx)
	}
	return err
}

// mailParentReadFilter is applied before both page selection and counting.
// Correlated EXISTS keeps group grants from duplicating message rows.
func (db *DbResource) mailParentReadFilter(table string, caller *auth.SessionUser, tx *sqlx.Tx) (exp.Expression, error) {
	if caller.UserReferenceId == daptinid.NullReferenceId {
		return goqu.L("1 = 0"), nil
	}
	var refs []daptinid.DaptinReferenceId
	for _, group := range caller.Groups {
		refs = append(refs, group.GroupReferenceId)
	}
	var groupIDs []int64
	if len(refs) > 0 {
		ids, err := GetReferenceIdListToIdListWithTransaction("usergroup", refs, tx)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			groupIDs = append(groupIDs, id)
		}
	}
	readable := func(name, alias, foreignKey string, ancestors ...exp.Expression) exp.Expression {
		crud := db.Cruds[name]
		rules := crud.GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", name, tx)
		if !rules.CanPeek(caller.UserReferenceId, caller.Groups, crud.AdministratorGroupId) {
			return goqu.L("1 = 0")
		}
		mask := func(column string, bit auth.AuthPermission) exp.Expression {
			return goqu.L("(? & ?) = ?", goqu.I(column), int64(bit), int64(bit))
		}
		grant := []exp.Expression{
			mask(alias+".permission", auth.GuestRead),
			goqu.And(goqu.I(alias+".user_account_id").Eq(caller.UserId), mask(alias+".permission", auth.UserRead)),
		}
		if len(groupIDs) > 0 {
			groups := statementbuilder.Squirrel.Select(goqu.L("1")).
				From(goqu.T(name+"_"+name+"_id_has_usergroup_usergroup_id").As("mail_read_group")).
				Where(goqu.I("mail_read_group."+name+"_id").Eq(goqu.I(alias+".id")),
					goqu.I("mail_read_group.usergroup_id").In(groupIDs), mask("mail_read_group.permission", auth.GroupRead))
			grant = append(grant, goqu.L("EXISTS ?", groups))
		}
		parent := statementbuilder.Squirrel.Select(goqu.L("1")).From(goqu.T(name).As(alias)).
			Where(goqu.I(alias+".id").Eq(goqu.I(foreignKey)), goqu.Or(grant...)).Where(ancestors...)
		return goqu.L("EXISTS ?", parent)
	}
	if table == "mail_box" {
		return readable("mail_account", "mail_read_account", "mail_box.mail_account_id"), nil
	}
	account := readable("mail_account", "mail_read_account", "mail_read_box.mail_account_id")
	return readable("mail_box", "mail_read_box", "mail.mail_box_id", account), nil
}
