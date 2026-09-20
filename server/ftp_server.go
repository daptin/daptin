package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	ftppath "path"
	"path/filepath"
	"strings"
	"time"

	rclonefs "github.com/artpar/rclone/fs"
	"github.com/daptin/daptin/server/assetcachepojo"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	log "github.com/sirupsen/logrus"

	"github.com/daptin/daptin/server/resource"

	"sync/atomic"

	"github.com/artpar/ftpserver/server"
)

// DaptinFtpDriver defines a very basic ftpserver driver
type DaptinFtpDriver struct {
	Logger                  *log.Logger             // Logger
	BaseDir                 string                  // Base directory from which to serve file
	tlsConfig               *tls.Config             // TLS config (if applies)
	DaptinFtpServerSettings DaptinFtpServerSettings // Our settings
	nbClients               int32                   // Number of clients
	Sites                   map[string]SubSiteAssetCache
	CertManager             *resource.CertificateManager
	cruds                   map[string]*resource.DbResource
}

// ClientDriver defines a very basic client driver
type ClientDriver struct {
	BaseDir     string // Base directory from which to server file
	CurrentDir  string
	FtpDriver   *DaptinFtpDriver
	sessionUser *auth.SessionUser
}

// DaptinFtpServerSettings defines our settings
type DaptinFtpServerSettings struct {
	Server         server.Settings // Server settings (shouldn't need to be filled)
	MaxConnections int32           // Maximum number of clients that are allowed to connect at the same time
}

// NewDaptinFtpDriver creates a new driver
func NewDaptinFtpDriver(cruds map[string]*resource.DbResource,
	certManager *resource.CertificateManager, ftp_interface string, sites []SubSiteAssetCache) (*DaptinFtpDriver, error) {

	siteMap := make(map[string]SubSiteAssetCache)
	for _, site := range sites {
		siteMap[site.Hostname] = site
	}

	ftpLogger := log.New()
	drv := &DaptinFtpDriver{
		Logger:      ftpLogger,
		BaseDir:     "/",
		Sites:       siteMap,
		CertManager: certManager,
		cruds:       cruds,
		DaptinFtpServerSettings: DaptinFtpServerSettings{
			MaxConnections: 100,
			Server: server.Settings{
				Listener:   nil,
				ListenAddr: ftp_interface,
				PublicHost: "",
				PublicIPResolver: func(ctx server.ClientContext) (string, error) {
					return "", nil
				},
				PassiveTransferPortRange: nil,
				ActiveTransferPortNon20:  false,
				IdleTimeout:              5,
				ConnectionTimeout:        5,
				DisableMLSD:              false,
				DisableMLST:              false,
			},
		},
	}

	return drv, nil
}

// GetSettings returns some general settings around the server setup
func (driver *DaptinFtpDriver) GetSettings() (*server.Settings, error) {

	var err error
	// This is the new IP loading change coming from Ray
	if driver.DaptinFtpServerSettings.Server.PublicHost == "" {
		publicIP := ""

		driver.Logger.Printf("Fetching our external IP address...")

		if publicIP, err = externalIP(); err != nil {
			resource.CheckErr(err, "Couldn't fetch an external IP")
		} else {
			driver.Logger.Printf(
				"Fetched our external IP address %v %v %v %v",
				"action", "external_ip.fetched",
				"ipAddress", publicIP)
		}

		// Adding a special case for loopback clients (issue #74)
		driver.DaptinFtpServerSettings.Server.PublicIPResolver = func(cc server.ClientContext) (string, error) {
			driver.Logger.Printf("Resolving public IP %v %v", "remoteAddr", cc.RemoteAddr())

			if strings.HasPrefix(cc.RemoteAddr().String(), "127.0.0.1") {
				return "127.0.0.1", nil
			}

			return publicIP, nil
		}
	}

	return &driver.DaptinFtpServerSettings.Server, nil
}

// GetTLSConfig returns a TLS Certificate to use
func (driver *DaptinFtpDriver) GetTLSConfig() (*tls.Config, error) {

	if driver.tlsConfig != nil {
		return driver.tlsConfig, nil
	}
	firstSite := ""
	for s := range driver.Sites {
		firstSite = s
		break
	}
	if firstSite == "" {
		return nil, errors.New("cannot configure FTP TLS without an enabled site")
	}

	transaction, err := driver.cruds["world"].Connection().Beginx()
	if err != nil {
		resource.CheckErr(err, "Failed to begin transaction [134]")
		return nil, err
	}

	cert, err := driver.CertManager.GetTLSConfig(driver.Sites[firstSite].Hostname, true, transaction)
	if err != nil {
		return nil, err
	}

	transaction.Commit()
	cert.TLSConfig.NextProtos = []string{"ftp"}
	driver.tlsConfig = cert.TLSConfig
	return driver.tlsConfig, nil
}

// WelcomeUser is called to send the very first welcome message
func (driver *DaptinFtpDriver) WelcomeUser(cc server.ClientContext) (string, error) {
	nbClients := atomic.AddInt32(&driver.nbClients, 1)
	if nbClients > driver.DaptinFtpServerSettings.MaxConnections {
		return "Cannot accept any additional client", fmt.Errorf(
			"too many clients: %d > % d",
			driver.nbClients,
			driver.DaptinFtpServerSettings.MaxConnections)
	}

	cc.SetDebug(true)
	// This will remain the official name for now
	return fmt.Sprintf(
			"Welcome on daptin FTP server, you're on dir %s, your ID is %d, your IP:port is %s, we currently have %d clients connected",
			driver.BaseDir,
			cc.ID(),
			cc.RemoteAddr(),
			nbClients),
		nil
}

// AuthUser authenticates the user and selects an handling driver
func (driver *DaptinFtpDriver) AuthUser(cc server.ClientContext, user, pass string) (server.ClientHandlingDriver, error) {

	transaction, err := driver.cruds["user_account"].Connection().Beginx()
	if err != nil {
		resource.CheckErr(err, "Failed to begin transaction [174]")
		return nil, err
	}

	defer transaction.Rollback()
	userAccount, err := driver.cruds["user_account"].GetUserAccountRowByEmail(user, transaction)
	if err != nil {
		return nil, err
	}

	passwordHash, ok := userAccount["password"].(string)
	if !ok || !resource.BcryptCheckStringHash(pass, passwordHash) {
		return nil, fmt.Errorf("could not authenticate you")
	}
	userId, ok := userAccount["id"].(int64)
	if !ok {
		return nil, errors.New("invalid user account id")
	}
	groups := driver.cruds["user_account"].GetObjectUserGroupsByWhereWithTransaction("user_account", transaction, "id", userId)
	sessionUser := &auth.SessionUser{
		UserId:          userId,
		UserReferenceId: daptinid.InterfaceToDIR(userAccount["reference_id"]),
		Groups:          groups,
	}
	if sessionUser.UserReferenceId == daptinid.NullReferenceId {
		return nil, errors.New("invalid user account reference id")
	}
	log.Infof("FTP Login [%s][%s][%s]", driver.BaseDir, user, cc.RemoteAddr())
	return &ClientDriver{
		BaseDir:     "/",
		CurrentDir:  "/",
		FtpDriver:   driver,
		sessionUser: sessionUser,
	}, nil
}

func (driver *ClientDriver) sitePath(ftpPath string) (string, SubSiteAssetCache, string, error) {
	if driver == nil || driver.FtpDriver == nil || driver.sessionUser == nil {
		return "", SubSiteAssetCache{}, "", errors.New("invalid FTP session")
	}
	if strings.ContainsRune(ftpPath, '\x00') {
		return "", SubSiteAssetCache{}, "", errors.New("invalid path")
	}

	cleanPath := ftppath.Clean("/" + strings.TrimPrefix(strings.ReplaceAll(ftpPath, "\\", "/"), "/"))
	pathParts := strings.Split(strings.TrimPrefix(cleanPath, "/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" || pathParts[0] == "." {
		return "", SubSiteAssetCache{}, "", errors.New("invalid site")
	}

	siteName := pathParts[0]
	site, ok := driver.FtpDriver.Sites[siteName]
	if !ok || site.AssetFolderCache == nil || site.LocalSyncPath == "" {
		return "", SubSiteAssetCache{}, "", errors.New("invalid site")
	}
	relativePath := "."
	if len(pathParts) > 1 {
		relativePath = filepath.Join(pathParts[1:]...)
	}
	return siteName, site, relativePath, nil
}

// UserLeft is called when the user disconnects, even if he never authenticated
func (driver *DaptinFtpDriver) UserLeft(cc server.ClientContext) {
	atomic.AddInt32(&driver.nbClients, -1)
}

func (driver *ClientDriver) SetFileMtime(cc server.ClientContext, path string, mtime time.Time) error {
	_, site, relativePath, err := driver.sitePath(path)
	if err != nil {
		return err
	}
	if !site.Permission.CanUpdate(driver.sessionUser.UserReferenceId, driver.sessionUser.Groups, driver.FtpDriver.cruds["site"].AdministratorGroupId) {
		return errors.New("permission denied")
	}
	if err := site.SetStoredModTime(context.Background(), relativePath, mtime); err != nil {
		return err
	}
	invalidateFTPSiteCache(site)
	return nil
}

// ChangeDirectory changes the current working directory
func (driver *ClientDriver) ChangeDirectory(cc server.ClientContext, directory string) error {
	log.Printf("Change directory: [%v]", directory)

	if directory == "/" {
		driver.CurrentDir = "/"
		return nil
	}

	siteName, site, relativePath, err := driver.sitePath(directory)
	if err != nil || !site.Permission.CanPeek(driver.sessionUser.UserReferenceId, driver.sessionUser.Groups, driver.FtpDriver.cruds["site"].AdministratorGroupId) {
		return fmt.Errorf("no such path %v", directory)
	}
	fileInfo, err := site.StatStoredFile(context.Background(), relativePath)
	if err != nil || !fileInfo.IsDir() {
		return fmt.Errorf("no such path %v", directory)
	}
	driver.CurrentDir = siteName
	return nil
}

// MakeDirectory creates a directory
func (driver *ClientDriver) MakeDirectory(cc server.ClientContext, path string) error {
	_, site, relativePath, err := driver.sitePath(path)
	if err != nil {
		return err
	}
	if !site.Permission.CanCreate(driver.sessionUser.UserReferenceId, driver.sessionUser.Groups, driver.FtpDriver.cruds["site"].AdministratorGroupId) {
		return errors.New("permission denied")
	}
	if relativePath == "." {
		return errors.New("cannot create site root")
	}
	if err := site.MakeStoredDirectory(context.Background(), relativePath); err != nil {
		return err
	}
	invalidateFTPSiteCache(site)
	return nil
}

// ListFiles lists the files of a directory
func (driver *ClientDriver) ListFiles(cc server.ClientContext, directory string) ([]fs.FileInfo, error) {
	log.Printf("List files: [%v][%v]", driver.CurrentDir, directory)
	files := make([]fs.FileInfo, 0)

	if directory == "/" {
		if driver == nil || driver.FtpDriver == nil || driver.sessionUser == nil {
			return nil, errors.New("invalid FTP session")
		}
		for siteName, site := range driver.FtpDriver.Sites {
			if !site.Permission.CanPeek(driver.sessionUser.UserReferenceId, driver.sessionUser.Groups, driver.FtpDriver.cruds["site"].AdministratorGroupId) {
				continue
			}
			files = append(files, virtualFileInfo{
				name: siteName,
				mode: os.FileMode(0666) | os.ModeDir,
			})
		}
		return files, nil
	}

	_, site, relativePath, err := driver.sitePath(directory)
	if err != nil {
		return nil, err
	}
	if !site.Permission.CanRead(driver.sessionUser.UserReferenceId, driver.sessionUser.Groups, driver.FtpDriver.cruds["site"].AdministratorGroupId) {
		return nil, errors.New("permission denied")
	}
	return site.ListStoredFiles(context.Background(), relativePath)
}

// OpenFile opens a file in 3 possible modes: read, write, appending write (use appropriate flags)
func (driver *ClientDriver) OpenFile(cc server.ClientContext, path string, flag int) (server.FileStream, error) {
	_, site, relativePath, err := driver.sitePath(path)
	if err != nil {
		return nil, err
	}
	if (flag & (os.O_WRONLY | os.O_RDWR)) != 0 {
		adminGroupID := driver.FtpDriver.cruds["site"].AdministratorGroupId
		canCreate := site.Permission.CanCreate(driver.sessionUser.UserReferenceId, driver.sessionUser.Groups, adminGroupID)
		canUpdate := site.Permission.CanUpdate(driver.sessionUser.UserReferenceId, driver.sessionUser.Groups, adminGroupID)
		if !canCreate && !canUpdate {
			return nil, errors.New("permission denied")
		}
		info, statErr := site.StatStoredFile(context.Background(), relativePath)
		existing := statErr == nil
		if existing && info.IsDir() {
			return nil, errors.New("cannot write a directory")
		}
		if isFTPNotExist(statErr) {
			if !canCreate {
				return nil, errors.New("permission denied")
			}
		} else {
			if statErr != nil {
				return nil, statErr
			}
			if !canUpdate {
				return nil, errors.New("permission denied")
			}
		}
		stage, err := os.CreateTemp("", "daptin-ftp-upload-*")
		if err != nil {
			return nil, err
		}
		if !existing && ftpRestartOffset(cc) != 0 {
			_ = stage.Close()
			_ = os.Remove(stage.Name())
			return nil, os.ErrNotExist
		}
		if existing && (flag&os.O_APPEND != 0 || ftpRestartOffset(cc) != 0) {
			previous, err := site.GetFileByNameContext(context.Background(), relativePath)
			if err != nil {
				_ = stage.Close()
				_ = os.Remove(stage.Name())
				return nil, err
			}
			_, copyErr := io.Copy(stage, previous)
			_ = previous.Close()
			if copyErr != nil {
				_ = stage.Close()
				_ = os.Remove(stage.Name())
				return nil, copyErr
			}
			if flag&os.O_APPEND != 0 {
				if _, err := stage.Seek(0, io.SeekEnd); err != nil {
					_ = stage.Close()
					_ = os.Remove(stage.Name())
					return nil, err
				}
			}
		}
		return &ftpStoredFile{File: stage, site: site.AssetFolderCache, name: relativePath, afterWrite: func() { invalidateFTPSiteCache(site) }}, nil
	} else if !site.Permission.CanRead(driver.sessionUser.UserReferenceId, driver.sessionUser.Groups, driver.FtpDriver.cruds["site"].AdministratorGroupId) {
		return nil, errors.New("permission denied")
	}
	file, err := site.GetFileByNameContext(context.Background(), relativePath)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func isFTPNotExist(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, rclonefs.ErrorObjectNotFound) || errors.Is(err, rclonefs.ErrorDirNotFound)
}

func ftpRestartOffset(cc server.ClientContext) int64 {
	if restart, ok := cc.(interface{ RestartOffset() int64 }); ok {
		return restart.RestartOffset()
	}
	return 0
}

type ftpStoredFile struct {
	*os.File
	site       *assetcachepojo.AssetFolderCache
	name       string
	afterWrite func()
	aborted    bool
	closed     bool
}

func (file *ftpStoredFile) Abort() error {
	file.aborted = true
	return nil
}

func (file *ftpStoredFile) Close() error {
	if file.closed {
		return nil
	}
	file.closed = true
	name := file.File.Name()
	if file.site != nil {
		defer os.Remove(name)
	}
	closeErr := file.File.Close()
	if closeErr != nil {
		return closeErr
	}
	if file.site == nil {
		return nil
	}
	if file.aborted {
		return nil
	}
	reader, err := os.Open(name)
	if err != nil {
		return err
	}
	defer reader.Close()
	if _, err := file.site.PutStoredFile(context.Background(), file.name, reader); err != nil {
		return err
	}
	file.afterWrite()
	return nil
}

func invalidateFTPSiteCache(site SubSiteAssetCache) {
	for _, hostname := range strings.Split(site.Hostname, ",") {
		indexCache.Delete(hostname)
		negativeCache.Range(func(key, _ interface{}) bool {
			if cachedKey, ok := key.(string); ok && strings.HasPrefix(cachedKey, hostname+":") {
				negativeCache.Delete(key)
			}
			return true
		})
	}
}

// GetFileInfo gets some info around a file or a directory
func (driver *ClientDriver) GetFileInfo(cc server.ClientContext, path string) (os.FileInfo, error) {
	_, site, relativePath, err := driver.sitePath(path)
	if err != nil {
		return nil, err
	}
	if !site.Permission.CanRead(driver.sessionUser.UserReferenceId, driver.sessionUser.Groups, driver.FtpDriver.cruds["site"].AdministratorGroupId) {
		return nil, errors.New("permission denied")
	}
	return site.StatStoredFile(context.Background(), relativePath)
}

// CanAllocate gives the approval to allocate some data
func (driver *ClientDriver) CanAllocate(cc server.ClientContext, size int) (bool, error) {
	return true, nil
}

// ChmodFile changes the attributes of the file
func (driver *ClientDriver) ChmodFile(cc server.ClientContext, path string, mode os.FileMode) error {
	_, site, relativePath, err := driver.sitePath(path)
	if err != nil {
		return err
	}
	if !site.Permission.CanUpdate(driver.sessionUser.UserReferenceId, driver.sessionUser.Groups, driver.FtpDriver.cruds["site"].AdministratorGroupId) {
		return errors.New("permission denied")
	}
	return site.ChmodStoredFile(relativePath, mode)
}

// DeleteFile deletes a file or a directory
func (driver *ClientDriver) DeleteFile(cc server.ClientContext, path string) error {
	_, site, relativePath, err := driver.sitePath(path)
	if err != nil {
		return err
	}
	if !site.Permission.CanDelete(driver.sessionUser.UserReferenceId, driver.sessionUser.Groups, driver.FtpDriver.cruds["site"].AdministratorGroupId) {
		return errors.New("permission denied")
	}
	if relativePath == "." {
		return errors.New("cannot delete site root")
	}
	if err := site.RemoveStoredFile(context.Background(), relativePath); err != nil {
		return err
	}
	invalidateFTPSiteCache(site)
	return nil
}

// RenameFile renames a file or a directory
func (driver *ClientDriver) RenameFile(cc server.ClientContext, from, to string) error {
	fromSiteName, fromSite, fromPath, err := driver.sitePath(from)
	if err != nil {
		return err
	}
	toSiteName, toSite, toPath, err := driver.sitePath(to)
	if err != nil {
		return err
	}
	if fromSiteName != toSiteName {
		return errors.New("cannot rename across sites")
	}
	if !fromSite.Permission.CanUpdate(driver.sessionUser.UserReferenceId, driver.sessionUser.Groups, driver.FtpDriver.cruds["site"].AdministratorGroupId) ||
		!toSite.Permission.CanUpdate(driver.sessionUser.UserReferenceId, driver.sessionUser.Groups, driver.FtpDriver.cruds["site"].AdministratorGroupId) {
		return errors.New("permission denied")
	}
	if fromPath == "." || toPath == "." {
		return errors.New("cannot rename site root")
	}
	if err := fromSite.MoveStoredFile(context.Background(), fromPath, toPath); err != nil {
		return err
	}
	invalidateFTPSiteCache(fromSite)
	invalidateFTPSiteCache(toSite)
	return nil
}

// The virtual file is an example of how you can implement a purely virtual file
type virtualFile struct {
	content    []byte // Content of the file
	readOffset int    // Reading offset
}

func (f *virtualFile) Close() error {
	return nil
}

func (f *virtualFile) Read(buffer []byte) (int, error) {
	n := copy(buffer, f.content[f.readOffset:])
	f.readOffset += n

	if n == 0 {
		return 0, io.EOF
	}

	return n, nil
}

func (f *virtualFile) Seek(n int64, w int) (int64, error) {
	return 0, nil
}

func (f *virtualFile) Write(buffer []byte) (int, error) {
	return 0, nil
}

type virtualFileInfo struct {
	name string
	size int64
	mode os.FileMode
}

func (f virtualFileInfo) Type() fs.FileMode {
	//TODO implement me
	return fs.ModeDir
}

func (f virtualFileInfo) Info() (fs.FileInfo, error) {
	//TODO implement me
	return f, nil
}

func (f virtualFileInfo) Name() string {
	return f.name
}

func (f virtualFileInfo) Size() int64 {
	return f.size
}

func (f virtualFileInfo) Mode() os.FileMode {
	return f.mode
}

func (f virtualFileInfo) IsDir() bool {
	return f.mode.IsDir()
}

func (f virtualFileInfo) ModTime() time.Time {
	return time.Now().UTC()
}

func (f virtualFileInfo) Sys() interface{} {
	return nil
}

func externalIP() (string, error) {
	// If you need to take a bet, amazon is about as reliable & sustainable a service as you can get
	rsp, err := http.Get("http://checkip.amazonaws.com")
	if err != nil {
		return "", err
	}

	defer func() {
		if errClose := rsp.Body.Close(); errClose != nil {
			fmt.Println("Problem closing checkip connection, err:", errClose)
		}
	}()

	buf, err := io.ReadAll(rsp.Body)
	if err != nil {
		return "", err
	}

	return string(bytes.TrimSpace(buf)), nil
}
