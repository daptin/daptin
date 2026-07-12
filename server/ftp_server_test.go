package server

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/daptin/daptin/server/assetcachepojo"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/permission"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/subsite"
	"github.com/google/uuid"
)

func ftpTestReferenceId() daptinid.DaptinReferenceId {
	return daptinid.DaptinReferenceId(uuid.New())
}

func ftpTestClient(sessionUser *auth.SessionUser, adminGroupId daptinid.DaptinReferenceId, sites ...SubSiteAssetCache) *ClientDriver {
	siteMap := make(map[string]SubSiteAssetCache, len(sites))
	for _, site := range sites {
		siteMap[site.Hostname] = site
	}
	return &ClientDriver{
		BaseDir:    "/",
		CurrentDir: "/",
		FtpDriver: &DaptinFtpDriver{
			Sites: siteMap,
			cruds: map[string]*resource.DbResource{
				"site": {AdministratorGroupId: adminGroupId},
			},
		},
		sessionUser: sessionUser,
	}
}

func ftpTestSite(hostname, localSyncPath string, sitePermission permission.PermissionInstance) SubSiteAssetCache {
	return SubSiteAssetCache{
		SubSite: subsite.SubSite{
			Hostname:   hostname,
			Permission: sitePermission,
		},
		AssetFolderCache: &assetcachepojo.AssetFolderCache{LocalSyncPath: localSyncPath},
	}
}

func TestFtpRootListsSitesAllowedByExistingPermissions(t *testing.T) {
	ownerReferenceId := ftpTestReferenceId()
	otherReferenceId := ftpTestReferenceId()
	groupReferenceId := ftpTestReferenceId()
	adminGroupId := ftpTestReferenceId()

	ownerSite := ftpTestSite("owner.example", t.TempDir(), permission.PermissionInstance{
		UserId:     ownerReferenceId,
		Permission: auth.UserPeek,
	})
	groupSite := ftpTestSite("group.example", t.TempDir(), permission.PermissionInstance{
		UserGroupId: auth.GroupPermissionList{{
			GroupReferenceId: groupReferenceId,
			Permission:       auth.GroupPeek,
		}},
	})

	tests := []struct {
		name          string
		sessionUser   *auth.SessionUser
		expectedSites map[string]bool
	}{
		{
			name: "owner",
			sessionUser: &auth.SessionUser{
				UserReferenceId: ownerReferenceId,
			},
			expectedSites: map[string]bool{"owner.example": true},
		},
		{
			name: "related group member",
			sessionUser: &auth.SessionUser{
				UserReferenceId: otherReferenceId,
				Groups: auth.GroupPermissionList{{
					GroupReferenceId: groupReferenceId,
				}},
			},
			expectedSites: map[string]bool{"group.example": true},
		},
		{
			name: "unrelated user",
			sessionUser: &auth.SessionUser{
				UserReferenceId: otherReferenceId,
			},
			expectedSites: map[string]bool{},
		},
		{
			name: "administrator",
			sessionUser: &auth.SessionUser{
				UserReferenceId: otherReferenceId,
				Groups: auth.GroupPermissionList{{
					GroupReferenceId: adminGroupId,
				}},
			},
			expectedSites: map[string]bool{"owner.example": true, "group.example": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := ftpTestClient(tt.sessionUser, adminGroupId, ownerSite, groupSite)
			files, err := client.ListFiles(nil, "/")
			if err != nil {
				t.Fatalf("ListFiles returned an error: %v", err)
			}
			actualSites := make(map[string]bool, len(files))
			for _, file := range files {
				actualSites[file.Name()] = true
			}
			if len(actualSites) != len(tt.expectedSites) {
				t.Fatalf("listed sites = %v, want %v", actualSites, tt.expectedSites)
			}
			for site := range tt.expectedSites {
				if !actualSites[site] {
					t.Fatalf("listed sites = %v, want %v", actualSites, tt.expectedSites)
				}
			}
		})
	}
}

func TestFtpChangeDirectoryUsesSitePeekPermission(t *testing.T) {
	ownerReferenceId := ftpTestReferenceId()
	otherReferenceId := ftpTestReferenceId()
	adminGroupId := ftpTestReferenceId()
	site := ftpTestSite("private.example", t.TempDir(), permission.PermissionInstance{
		UserId:     ownerReferenceId,
		Permission: auth.UserPeek,
	})

	owner := ftpTestClient(&auth.SessionUser{UserReferenceId: ownerReferenceId}, adminGroupId, site)
	if err := owner.ChangeDirectory(nil, "/private.example"); err != nil {
		t.Fatalf("owner could not enter owned site: %v", err)
	}

	unrelated := ftpTestClient(&auth.SessionUser{UserReferenceId: otherReferenceId}, adminGroupId, site)
	if err := unrelated.ChangeDirectory(nil, "/private.example"); err == nil {
		t.Fatal("unrelated user entered private site")
	}
	if unrelated.CurrentDir != "/" {
		t.Fatalf("failed directory change set CurrentDir to %q", unrelated.CurrentDir)
	}
}

func TestFtpFileOperationsUseExistingSitePermissions(t *testing.T) {
	ownerReferenceId := ftpTestReferenceId()
	groupMemberReferenceId := ftpTestReferenceId()
	groupReferenceId := ftpTestReferenceId()
	adminGroupId := ftpTestReferenceId()
	root := t.TempDir()
	filePath := filepath.Join(root, "document.txt")
	if err := os.WriteFile(filePath, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}

	site := ftpTestSite("files.example", root, permission.PermissionInstance{
		UserId:     ownerReferenceId,
		Permission: auth.UserPeek | auth.UserRead,
		UserGroupId: auth.GroupPermissionList{{
			GroupReferenceId: groupReferenceId,
			Permission:       auth.GroupPeek | auth.GroupRead | auth.GroupCreate | auth.GroupUpdate | auth.GroupDelete,
		}},
	})

	owner := ftpTestClient(&auth.SessionUser{UserReferenceId: ownerReferenceId}, adminGroupId, site)
	owner.CurrentDir = "files.example"
	file, err := owner.OpenFile(nil, "/files.example/document.txt", os.O_RDONLY)
	if err != nil {
		t.Fatalf("owner could not read owned site: %v", err)
	}
	file.Close()
	if err := owner.DeleteFile(nil, "/files.example/document.txt"); err == nil {
		t.Fatal("owner without UserDelete deleted a file")
	}

	groupMember := ftpTestClient(&auth.SessionUser{
		UserReferenceId: groupMemberReferenceId,
		Groups: auth.GroupPermissionList{{
			GroupReferenceId: groupReferenceId,
		}},
	}, adminGroupId, site)
	groupMember.CurrentDir = "files.example"
	if err := groupMember.MakeDirectory(nil, "/files.example/new-directory"); err != nil {
		t.Fatalf("related group member could not create a directory: %v", err)
	}
	if err := groupMember.RenameFile(nil, "/files.example/document.txt", "/files.example/renamed.txt"); err != nil {
		t.Fatalf("related group member could not rename a file: %v", err)
	}
	if err := groupMember.DeleteFile(nil, "/files.example/renamed.txt"); err != nil {
		t.Fatalf("related group member could not delete a file: %v", err)
	}
}

func TestFtpListFilesReturnsDirectoryEntries(t *testing.T) {
	ownerReferenceId := ftpTestReferenceId()
	adminGroupId := ftpTestReferenceId()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "listed.txt"), []byte("listed"), 0600); err != nil {
		t.Fatal(err)
	}
	site := ftpTestSite("list.example", root, permission.PermissionInstance{
		UserId:     ownerReferenceId,
		Permission: auth.UserPeek | auth.UserRead,
	})
	client := ftpTestClient(&auth.SessionUser{UserReferenceId: ownerReferenceId}, adminGroupId, site)
	client.CurrentDir = "list.example"

	files, err := client.ListFiles(nil, "/list.example")
	if err != nil {
		t.Fatalf("ListFiles returned an error: %v", err)
	}
	if len(files) != 1 || files[0].Name() != "listed.txt" {
		t.Fatalf("ListFiles returned %#v, want listed.txt", files)
	}
}

func TestFtpPathsCannotEscapeSiteRoot(t *testing.T) {
	ownerReferenceId := ftpTestReferenceId()
	adminGroupId := ftpTestReferenceId()
	root := t.TempDir()
	outsideRoot := t.TempDir()
	outsideFile := filepath.Join(outsideRoot, "outside.txt")
	if err := os.WriteFile(outsideFile, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	site := ftpTestSite("contained.example", root, permission.PermissionInstance{
		UserId:     ownerReferenceId,
		Permission: auth.UserCRUD,
	})
	client := ftpTestClient(&auth.SessionUser{UserReferenceId: ownerReferenceId}, adminGroupId, site)
	client.CurrentDir = "contained.example"

	if _, err := client.OpenFile(nil, "/contained.example/../../outside.txt", os.O_RDONLY); err == nil {
		t.Fatal("parent traversal opened a file outside the site")
	}
	if err := client.DeleteFile(nil, "/contained.example/../../outside.txt"); err == nil {
		t.Fatal("parent traversal deleted a file outside the site")
	}
	if _, err := os.Stat(outsideFile); err != nil {
		t.Fatalf("outside file was changed: %v", err)
	}

	if runtime.GOOS == "windows" {
		return
	}
	linkPath := filepath.Join(root, "outside-link")
	if err := os.Symlink(outsideRoot, linkPath); err != nil {
		t.Fatal(err)
	}
	if _, err := client.OpenFile(nil, "/contained.example/outside-link/outside.txt", os.O_RDONLY); err == nil {
		t.Fatal("symlink traversal opened a file outside the site")
	}
	if _, err := client.OpenFile(nil, "/contained.example/outside-link/new.txt", os.O_WRONLY); err == nil {
		t.Fatal("symlink traversal created a file outside the site")
	}
	if err := client.DeleteFile(nil, "/contained.example/outside-link/outside.txt"); err == nil {
		t.Fatal("symlink traversal deleted a file outside the site")
	}
	if _, err := os.Stat(outsideFile); err != nil {
		t.Fatalf("outside file was changed through symlink: %v", err)
	}
}

func TestFtpRenameCannotCrossSites(t *testing.T) {
	ownerReferenceId := ftpTestReferenceId()
	adminGroupId := ftpTestReferenceId()
	firstRoot := t.TempDir()
	secondRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(firstRoot, "source.txt"), []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	permissions := permission.PermissionInstance{UserId: ownerReferenceId, Permission: auth.UserCRUD}
	client := ftpTestClient(
		&auth.SessionUser{UserReferenceId: ownerReferenceId},
		adminGroupId,
		ftpTestSite("first.example", firstRoot, permissions),
		ftpTestSite("second.example", secondRoot, permissions),
	)
	client.CurrentDir = "first.example"

	err := client.RenameFile(nil, "/first.example/source.txt", "/second.example/destination.txt")
	if err == nil {
		t.Fatal("cross-site rename succeeded")
	}
	if _, err := os.Stat(filepath.Join(firstRoot, "source.txt")); err != nil {
		t.Fatalf("source file was changed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(secondRoot, "destination.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("destination file exists or stat returned an unexpected error: %v", err)
	}
}

func TestFtpSiteRootCannotBeDeletedOrRenamed(t *testing.T) {
	ownerReferenceId := ftpTestReferenceId()
	adminGroupId := ftpTestReferenceId()
	root := t.TempDir()
	site := ftpTestSite("root.example", root, permission.PermissionInstance{
		UserId:     ownerReferenceId,
		Permission: auth.UserCRUD,
	})
	client := ftpTestClient(&auth.SessionUser{UserReferenceId: ownerReferenceId}, adminGroupId, site)

	if err := client.DeleteFile(nil, "/root.example"); err == nil {
		t.Fatal("site root deletion succeeded")
	}
	if err := client.RenameFile(nil, "/root.example", "/root.example/renamed"); err == nil {
		t.Fatal("site root rename succeeded")
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("site root was changed: %v", err)
	}
}

func TestFtpStoreTruncatesWithoutRemovingExistingFile(t *testing.T) {
	ownerReferenceId := ftpTestReferenceId()
	adminGroupId := ftpTestReferenceId()
	root := t.TempDir()
	filePath := filepath.Join(root, "existing.txt")
	if err := os.WriteFile(filePath, []byte("old content that is longer"), 0600); err != nil {
		t.Fatal(err)
	}
	originalInfo, err := os.Stat(filePath)
	if err != nil {
		t.Fatal(err)
	}
	site := ftpTestSite("write.example", root, permission.PermissionInstance{
		UserId:     ownerReferenceId,
		Permission: auth.UserUpdate,
	})
	client := ftpTestClient(&auth.SessionUser{UserReferenceId: ownerReferenceId}, adminGroupId, site)
	client.CurrentDir = "write.example"

	file, err := client.OpenFile(nil, "/write.example/existing.txt", os.O_WRONLY)
	if err != nil {
		t.Fatalf("OpenFile returned an error: %v", err)
	}
	if _, err := file.Write([]byte("new")); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "new" {
		t.Fatalf("file contents = %q, want new", contents)
	}
	newInfo, err := os.Stat(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(originalInfo, newInfo) {
		t.Fatal("STOR removed and recreated the existing file")
	}
}

func TestFtpInvalidStateReturnsErrors(t *testing.T) {
	adminGroupId := ftpTestReferenceId()
	client := ftpTestClient(&auth.SessionUser{UserReferenceId: ftpTestReferenceId()}, adminGroupId)

	if _, err := client.OpenFile(nil, "/missing/file.txt", os.O_RDONLY); err == nil {
		t.Fatal("OpenFile succeeded without a selected valid site")
	}
	if _, err := client.ListFiles(nil, "/missing"); err == nil {
		t.Fatal("ListFiles succeeded without a selected valid site")
	}
	if err := client.MakeDirectory(nil, "/missing/dir"); err == nil {
		t.Fatal("MakeDirectory succeeded without a selected valid site")
	}
	if err := client.DeleteFile(nil, "/missing/file.txt"); err == nil {
		t.Fatal("DeleteFile succeeded without a selected valid site")
	}
	if err := client.RenameFile(nil, "/missing/from", "/missing/to"); err == nil {
		t.Fatal("RenameFile succeeded without a selected valid site")
	}
	if err := client.ChmodFile(nil, "/missing/file.txt", 0600); err == nil {
		t.Fatal("ChmodFile succeeded without a selected valid site")
	}
	if err := client.SetFileMtime(nil, "/missing/file.txt", time.Now()); err == nil {
		t.Fatal("SetFileMtime succeeded without a selected valid site")
	}
}

func TestFtpTLSConfigRequiresSite(t *testing.T) {
	driver := &DaptinFtpDriver{Sites: map[string]SubSiteAssetCache{}}
	if _, err := driver.GetTLSConfig(); err == nil {
		t.Fatal("GetTLSConfig succeeded without an FTP-enabled site")
	}
}
