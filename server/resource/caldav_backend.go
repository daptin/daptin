package resource

import (
	"github.com/daptin/daptin/server/auth"
	"github.com/emersion/go-webdav"
)

// DaptinCaldavBackend implements CalDAV storage using database
// Pattern: follows DaptinImapBackend (imap_backend.go:12-97)
type DaptinCaldavBackend struct {
	cruds       map[string]*DbResource
	certManager *CertificateManager
}

// NewCaldavBackend creates a new CalDAV backend with database access
// Pattern: like NewImapServer (imap_backend.go:93-97)
func NewCaldavBackend(cruds map[string]*DbResource, certManager *CertificateManager) *DaptinCaldavBackend {
	return &DaptinCaldavBackend{
		cruds:       cruds,
		certManager: certManager,
	}
}

// CreateFileSystemForUser creates a per-user filesystem after authentication
// Pattern: like IMAP Login creating DaptinImapUser (imap_backend.go:48-91)
func (dcb *DaptinCaldavBackend) CreateFileSystemForUser(sessionUser *auth.SessionUser) webdav.FileSystem {
	return &DaptinCaldavFileSystem{
		cruds:       dcb.cruds,
		sessionUser: sessionUser,
	}
}
