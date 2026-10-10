package resource

import (
	"context"
	"errors"
	"net/http"
	"strings"

	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/go-webdav"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
)

var _ webdav.DAVPrincipalBackend = (*DaptinDAVBackend)(nil)

func (b *DaptinDAVBackend) davPrincipalAdapter(requestPath string) (*davACLAdapter, error) {
	switch {
	case strings.HasPrefix(requestPath, "/caldav/"):
		return b.calendarACLAdapter(), nil
	case strings.HasPrefix(requestPath, "/carddav/"):
		return b.addressBookACLAdapter(), nil
	default:
		return nil, webdav.NewHTTPError(http.StatusNotFound, errors.New("DAV principal not found"))
	}
}

func (b *DaptinDAVBackend) DAVPrincipal(_ context.Context, requestPath string) (*webdav.DAVPrincipal, error) {
	a, err := b.davPrincipalAdapter(requestPath)
	if err != nil {
		return nil, err
	}
	kind, ref, err := a.principal(requestPath)
	if err != nil {
		return nil, webdav.NewHTTPError(http.StatusNotFound, err)
	}
	if err := a.getPrincipal(requestPath, kind, map[string]string{"account": USER_ACCOUNT_TABLE_NAME, "group": "usergroup"}[kind]); err != nil {
		return nil, err
	}
	table := USER_ACCOUNT_TABLE_NAME
	if kind == "group" {
		table = "usergroup"
	}
	tx, err := b.cruds[table].Connection().Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	row, _, err := b.cruds[table].GetSingleRowByReferenceIdWithTransaction(table, ref, nil, tx)
	if err != nil {
		return nil, err
	}
	info := &webdav.DAVPrincipal{Path: requestPath, DisplayName: StringOrEmpty(row["name"])}
	if info.DisplayName == "" {
		info.DisplayName = StringOrEmpty(row["email"])
	}
	if kind == "group" {
		groupID, err := GetReferenceIdToIdWithTransaction("usergroup", ref, tx)
		if err != nil {
			return nil, err
		}
		members, err := GetObjectByWhereClauseWithTransaction("user_account_user_account_id_has_usergroup_usergroup_id", tx, goqu.Ex{"usergroup_id": groupID})
		if err != nil {
			return nil, err
		}
		for _, member := range members {
			accountID, ok := member["user_account_id"].(int64)
			if !ok {
				return nil, errors.New("invalid DAV principal membership")
			}
			accountRef, err := GetIdToReferenceIdWithTransaction(USER_ACCOUNT_TABLE_NAME, accountID, tx)
			if err != nil {
				return nil, err
			}
			if b.davPrincipalReferable(USER_ACCOUNT_TABLE_NAME, accountRef, tx) {
				info.Members = append(info.Members, "/"+a.prefix+"/"+accountRef.String()+"/")
			}
		}
	} else {
		accountID, err := GetReferenceIdToIdWithTransaction(USER_ACCOUNT_TABLE_NAME, ref, tx)
		if err != nil {
			return nil, err
		}
		memberships, err := GetObjectByWhereClauseWithTransaction("user_account_user_account_id_has_usergroup_usergroup_id", tx, goqu.Ex{"user_account_id": accountID})
		if err != nil {
			return nil, err
		}
		for _, member := range memberships {
			groupID, ok := member["usergroup_id"].(int64)
			if !ok {
				return nil, errors.New("invalid DAV principal membership")
			}
			groupRef, err := GetIdToReferenceIdWithTransaction("usergroup", groupID, tx)
			if err != nil {
				return nil, err
			}
			if b.davPrincipalReferable("usergroup", groupRef, tx) {
				info.Memberships = append(info.Memberships, "/"+a.prefix+"/groups/"+groupRef.String()+"/")
			}
		}
	}
	return info, tx.Commit()
}

func (b *DaptinDAVBackend) davPrincipalReferable(table string, ref daptinid.DaptinReferenceId, tx *sqlx.Tx) bool {
	crud := b.cruds[table]
	admin := crud.AdministratorGroupId
	user := b.sessionUser.UserReferenceId
	groups := b.sessionUser.Groups
	tableGrant := crud.GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", table, tx)
	rowGrant := GetObjectPermissionByReferenceIdWithTransaction(table, ref, tx)
	return tableGrant.CanRefer(user, groups, admin) && rowGrant.CanRefer(user, groups, admin)
}

func (b *DaptinDAVBackend) SearchDAVPrincipals(_ context.Context, prefix, term string, limit int) ([]webdav.DAVPrincipal, error) {
	if prefix != "/caldav/" && prefix != "/carddav/" || limit < 1 || limit > 100 {
		return nil, webdav.NewHTTPError(http.StatusBadRequest, errors.New("invalid DAV principal search"))
	}
	result := make([]webdav.DAVPrincipal, 0, limit)
	for _, table := range []string{USER_ACCOUNT_TABLE_NAME, "usergroup"} {
		tx, err := b.cruds[table].Connection().Beginx()
		if err != nil {
			return nil, err
		}
		rows, err := GetLimitedOrderedRowsWithTransaction(table, []string{"reference_id", "name"}, "name", tx,
			uint(limit+1), goqu.Ex{"name": goqu.Op{"like": "%" + term + "%"}})
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		if len(rows) > limit {
			tx.Rollback()
			return nil, webdav.NewHTTPError(http.StatusInsufficientStorage, errors.New("too many DAV principals"))
		}
		for _, row := range rows {
			ref := daptinid.InterfaceToDIR(row["reference_id"])
			if ref == daptinid.NullReferenceId || !b.davPrincipalReferable(table, ref, tx) {
				continue
			}
			path := prefix + ref.String() + "/"
			if table == "usergroup" {
				path = prefix + "groups/" + ref.String() + "/"
			}
			result = append(result, webdav.DAVPrincipal{Path: path, DisplayName: StringOrEmpty(row["name"])})
			if len(result) > limit {
				tx.Rollback()
				return nil, webdav.NewHTTPError(http.StatusInsufficientStorage, errors.New("too many DAV principals"))
			}
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
	}
	return result, nil
}
