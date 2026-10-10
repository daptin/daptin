package resource

import (
	"context"
	"encoding/xml"
	"errors"

	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/go-webdav/carddav"
)

var _ carddav.AddressBookACLBackend = (*DaptinDAVBackend)(nil)

func addressBookACLPrivileges(grant auth.AuthPermission) []xml.Name {
	names := calendarACLPrivileges(grant)
	filtered := names[:0]
	for _, name := range names {
		if name.Space != calDAVPrivilegeNamespace {
			filtered = append(filtered, name)
		}
	}
	return filtered
}

func addressBookACLGrant(names []xml.Name) (auth.AuthPermission, error) {
	for _, name := range names {
		if name.Space == calDAVPrivilegeNamespace {
			return 0, errors.New("CalDAV privilege is unavailable on an address book")
		}
	}
	return calendarACLGrant(names)
}

func (b *DaptinDAVBackend) addressBookACLAdapter() *davACLAdapter {
	return &davACLAdapter{b: b, prefix: "carddav", collectionTable: addressBookTable, objectTable: addressObjectTable, groupPrefix: "cardshare"}
}

func (b *DaptinDAVBackend) GetAddressBookGroupPrincipal(ctx context.Context, requestPath string) error {
	return b.addressBookACLAdapter().getGroupPrincipal(ctx, requestPath)
}

func (b *DaptinDAVBackend) GetAddressBookAccountPrincipal(ctx context.Context, requestPath string) error {
	return b.addressBookACLAdapter().getAccountPrincipal(ctx, requestPath)
}

func (b *DaptinDAVBackend) GetAddressBookACL(ctx context.Context, requestPath string) (*carddav.AddressBookACL, error) {
	return b.addressBookACLAdapter().getACL(ctx, requestPath)
}

func (b *DaptinDAVBackend) ReplaceAddressBookACL(ctx context.Context, requestPath string, entries []carddav.AddressBookACE) error {
	return b.addressBookACLAdapter().replaceACL(ctx, requestPath, entries)
}
