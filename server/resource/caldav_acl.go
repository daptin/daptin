package resource

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"

	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/go-webdav/caldav"
)

var _ caldav.CalendarACLBackend = (*DaptinDAVBackend)(nil)

const calDAVPrivilegeNamespace = "urn:ietf:params:xml:ns:caldav"

func calendarACLPrivileges(grant auth.AuthPermission) []xml.Name {
	var names []xml.Name
	add := func(bit auth.AuthPermission, space, local string) {
		if grant&bit == bit {
			names = append(names, xml.Name{Space: space, Local: local})
		}
	}
	if grant&auth.GroupRead != 0 {
		add(auth.GroupRead, "DAV:", "read")
	} else {
		add(auth.GroupPeek, calDAVPrivilegeNamespace, "read-free-busy")
	}
	add(auth.GroupUpdate, "DAV:", "write-content")
	add(auth.GroupUpdate, "DAV:", "write-properties")
	add(auth.GroupCreate|auth.GroupRefer, "DAV:", "bind")
	add(auth.GroupDelete|auth.GroupUpdate, "DAV:", "unbind")
	add(auth.GroupExecute, "DAV:", "write-acl")
	return names
}

func calendarACLGrant(names []xml.Name) (auth.AuthPermission, error) {
	var grant auth.AuthPermission
	seen := make(map[xml.Name]bool)
	for _, name := range names {
		if seen[name] {
			return 0, errors.New("duplicate DAV privilege")
		}
		seen[name] = true
		switch name {
		case xml.Name{Space: calDAVPrivilegeNamespace, Local: "read-free-busy"}:
			grant |= auth.GroupPeek
		case xml.Name{Space: "DAV:", Local: "read"}:
			grant |= auth.GroupRead | auth.GroupPeek
		case xml.Name{Space: "DAV:", Local: "write-content"}:
			grant |= auth.GroupUpdate
		case xml.Name{Space: "DAV:", Local: "write-properties"}:
			grant |= auth.GroupUpdate
		case xml.Name{Space: "DAV:", Local: "bind"}:
			grant |= auth.GroupCreate | auth.GroupRefer
		case xml.Name{Space: "DAV:", Local: "unbind"}:
			grant |= auth.GroupDelete
		case xml.Name{Space: "DAV:", Local: "write-acl"}:
			grant |= auth.GroupExecute
		default:
			return 0, fmt.Errorf("unsupported DAV privilege %s", name.Local)
		}
	}
	if seen[xml.Name{Space: "DAV:", Local: "write-content"}] != seen[xml.Name{Space: "DAV:", Local: "write-properties"}] {
		return 0, errors.New("DAV content and property writes share one Daptin grant")
	}
	if grant&(auth.GroupUpdate|auth.GroupCreate|auth.GroupDelete|auth.GroupExecute) != 0 && grant&auth.GroupRead == 0 {
		return 0, errors.New("calendar write privileges require read")
	}
	if grant&auth.GroupDelete != 0 && grant&auth.GroupUpdate == 0 {
		return 0, errors.New("DAV:unbind requires DAV:write-content")
	}
	return grant, nil
}

func calendarACLRepresentable(grant auth.AuthPermission) bool {
	actual, err := calendarACLGrant(calendarACLPrivileges(grant))
	return err == nil && actual == grant
}

func equalCalendarACLPrivileges(a, b []xml.Name) bool {
	if len(a) != len(b) {
		return false
	}
	set := make(map[xml.Name]bool, len(a))
	for _, name := range a {
		set[name] = true
	}
	if len(set) != len(a) {
		return false
	}
	for _, name := range b {
		if !set[name] {
			return false
		}
	}
	return true
}

func (b *DaptinDAVBackend) calendarACLAdapter() *davACLAdapter {
	return &davACLAdapter{b: b, prefix: "caldav", collectionTable: calendarCollectionTable, objectTable: calendarObjectTable, groupPrefix: "calshare"}
}

func (b *DaptinDAVBackend) GetGroupPrincipal(ctx context.Context, requestPath string) error {
	return b.calendarACLAdapter().getGroupPrincipal(ctx, requestPath)
}

func (b *DaptinDAVBackend) GetAccountPrincipal(ctx context.Context, requestPath string) error {
	return b.calendarACLAdapter().getAccountPrincipal(ctx, requestPath)
}

func (b *DaptinDAVBackend) GetCalendarACL(ctx context.Context, requestPath string) (*caldav.CalendarACL, error) {
	return b.calendarACLAdapter().getACL(ctx, requestPath)
}

func (b *DaptinDAVBackend) ReplaceCalendarACL(ctx context.Context, requestPath string, entries []caldav.CalendarACE) error {
	return b.calendarACLAdapter().replaceACL(ctx, requestPath, entries)
}
