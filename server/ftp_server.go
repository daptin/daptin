package server

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/daptin/daptin/server/resource"

	"sync/atomic"

	"github.com/fclairamb/ftpserver/server"
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
	BaseDir    string // Base directory from which to server file
	CurrentDir string
	FtpDriver  *DaptinFtpDriver
}

// DaptinFtpServerSettings defines our settings
type DaptinFtpServerSettings struct {
	Server         server.Settings // Server settings (shouldn't need to be filled)
	MaxConnections int32           // Maximum number of clients that are allowed to connect at the same time
}

// NewDaptinFtpDriver creates a new driver
func NewDaptinFtpDriver(cruds map[string]*resource.DbResource, certManager *resource.CertificateManager, ftp_interface string, sites []SubSiteAssetCache) (*DaptinFtpDriver, error) {

	siteMap := make(map[string]SubSiteAssetCache)
	for _, site := range sites {
		siteMap[site.Hostname] = site
	}

	drv := &DaptinFtpDriver{
		Logger:      log.New(os.Stdout, "[FTP] ", 1),
		BaseDir:     "/",
		Sites:       siteMap,
		CertManager: certManager,
		cruds:       cruds,
		DaptinFtpServerSettings: DaptinFtpServerSettings{
			MaxConnections: 100,
			Server: server.Settings{
				Listener:                 nil,
				ListenAddr:               ftp_interface,
				PublicHost:               "",
				PublicIPResolver:         func(ctx server.ClientContext) (string, error) {
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

	tls1, _, _, _, _, err := driver.CertManager.GetTLSConfig(driver.Sites[firstSite].Hostname, true)
	if err != nil {
		return nil, err
	}
	tls1.NextProtos = []string{"ftp"}
	driver.tlsConfig = tls1
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

	userAccount, err := driver.cruds["user_account"].GetUserAccountRowByEmail(user)
	if err != nil {
		return nil, err
	}

	if !resource.BcryptCheckStringHash(pass, userAccount["password"].(string)) {
		return nil, fmt.Errorf("could not authenticate you")
	}
	return &ClientDriver{
		BaseDir:    "/",
		CurrentDir: "/",
		FtpDriver:  driver,
	}, nil
}

// UserLeft is called when the user disconnects, even if he never authenticated
func (driver *DaptinFtpDriver) UserLeft(cc server.ClientContext) {
	atomic.AddInt32(&driver.nbClients, -1)
}

func (driver *ClientDriver) SetFileMtime(cc server.ClientContext, path string, mtime time.Time) error {

	dirParts := strings.Split(path, string(os.PathSeparator))

	if len(dirParts) == 2 {
		subsiteName := dirParts[1]
		_, ok := driver.FtpDriver.Sites[subsiteName]
		if !ok {
			return errors.New("invalid path " + subsiteName)
		}
		driver.CurrentDir = subsiteName
	}

	path = driver.FtpDriver.Sites[driver.CurrentDir].LocalSyncPath + string(os.PathSeparator) + strings.Join(dirParts[2:], string(os.PathSeparator))
	return os.Chtimes(path, mtime, mtime)
}

// ChangeDirectory changes the current working directory
func (driver *ClientDriver) ChangeDirectory(cc server.ClientContext, directory string) error {

	var err error
	log.Printf("Change directory: [%v]", directory)

	if directory == "/" {
		driver.CurrentDir = "/"
		return nil
	}

	dirParts := strings.Split(directory, string(os.PathSeparator))
	if len(dirParts) == 2 {
		subsiteName := dirParts[1]
		_, ok := driver.FtpDriver.Sites[subsiteName]
		if !ok {
			return errors.New("invalid path " + subsiteName)
		}
		driver.CurrentDir = subsiteName
	} else {
		newDirName := dirParts[1]
		_, ok := driver.FtpDriver.Sites[newDirName]
		if !ok {
			err = errors.New(fmt.Sprintf("no such path %v", directory))
		} else {
			driver.CurrentDir = newDirName
			cdPath := driver.FtpDriver.Sites[driver.CurrentDir].LocalSyncPath + string(os.PathSeparator) + strings.Join(dirParts[2:], string(os.PathSeparator))
			log.Printf("CD Path: %v", cdPath)
			_, err = os.Stat(cdPath)
		}
	}
	//driver.CurrentDir = directory
	return err
}

// MakeDirectory creates a directory
func (driver *ClientDriver) MakeDirectory(cc server.ClientContext, path string) error {

	path = driver.FtpDriver.Sites[driver.CurrentDir].LocalSyncPath + string(os.PathSeparator) +
		strings.Join(strings.Split(path, string(os.PathSeparator))[2:], string(os.PathSeparator))

	if len(strings.Split(path, string(os.PathSeparator))) == 2 {
		return errors.New("cannot create new directory in /")
	}

	if driver.CurrentDir == "/" {
		return errors.New("cannot create new directory in /")
	}

	return os.Mkdir(path, 0750)
}

// ListFiles lists the files of a directory
func (driver *ClientDriver) ListFiles(cc server.ClientContext, directory string) ([]os.FileInfo, error) {

	var err error
	log.Printf("List files: [%v][%v]", driver.CurrentDir, directory)
	files := make([]os.FileInfo, 0)
	//files, err := ioutil.ReadDir(directory)

	// We add a virtual dir
	if directory == "/" {
		for site := range driver.FtpDriver.Sites {
			files = append(files, virtualFileInfo{
				name: site,
				mode: os.FileMode(0666) | os.ModeDir,
				//size: 4096,
			})
		}

	} else {
		path := driver.FtpDriver.Sites[driver.CurrentDir].LocalSyncPath + string(os.PathSeparator) +
			strings.Join(strings.Split(directory, string(os.PathSeparator))[2:], string(os.PathSeparator))
		files, err = ioutil.ReadDir(path)
	}
	log.Printf("list Path: %v", files)

	return files, err
}

// OpenFile opens a file in 3 possible modes: read, write, appending write (use appropriate flags)
func (driver *ClientDriver) OpenFile(cc server.ClientContext, path string, flag int) (server.FileStream, error) {

	path = driver.FtpDriver.Sites[driver.CurrentDir].LocalSyncPath + string(os.PathSeparator) +
		strings.Join(strings.Split(path, string(os.PathSeparator))[2:], string(os.PathSeparator))

	// If we are writing and we are not in append mode, we should remove the file
	if (flag & os.O_WRONLY) != 0 {
		flag |= os.O_CREATE
		if (flag & os.O_APPEND) == 0 {
			if err := os.Remove(path); err != nil {
				fmt.Println("Problem removing file", path, "err:", err)
			}
		}
	}

	return os.OpenFile(path, flag, 0600)
}

// GetFileInfo gets some info around a file or a directory
func (driver *ClientDriver) GetFileInfo(cc server.ClientContext, path string) (os.FileInfo, error) {

	path = driver.FtpDriver.Sites[driver.CurrentDir].LocalSyncPath + string(os.PathSeparator) +
		strings.Join(strings.Split(path, string(os.PathSeparator))[2:], string(os.PathSeparator))

	log.Printf("Get file info [%v]", path)

	return os.Stat(path)
}

// CanAllocate gives the approval to allocate some data
func (driver *ClientDriver) CanAllocate(cc server.ClientContext, size int) (bool, error) {
	return true, nil
}

// ChmodFile changes the attributes of the file
func (driver *ClientDriver) ChmodFile(cc server.ClientContext, path string, mode os.FileMode) error {
	path = driver.FtpDriver.Sites[driver.CurrentDir].LocalSyncPath + string(os.PathSeparator) +
		strings.Join(strings.Split(path, string(os.PathSeparator))[2:], string(os.PathSeparator))

	return os.Chmod(path, mode)
}

// DeleteFile deletes a file or a directory
func (driver *ClientDriver) DeleteFile(cc server.ClientContext, path string) error {
	path = driver.FtpDriver.Sites[driver.CurrentDir].LocalSyncPath + string(os.PathSeparator) +
		strings.Join(strings.Split(path, string(os.PathSeparator))[2:], string(os.PathSeparator))

	return os.Remove(path)
}

// RenameFile renames a file or a directory
func (driver *ClientDriver) RenameFile(cc server.ClientContext, from, to string) error {
	from = driver.FtpDriver.Sites[driver.CurrentDir].LocalSyncPath + string(os.PathSeparator) +
		strings.Join(strings.Split(from, string(os.PathSeparator))[2:], string(os.PathSeparator))
	to = driver.FtpDriver.Sites[driver.CurrentDir].LocalSyncPath + string(os.PathSeparator) +
		strings.Join(strings.Split(to, string(os.PathSeparator))[2:], string(os.PathSeparator))

	return os.Rename(from, to)
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

	buf, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return "", err
	}

	return string(bytes.TrimSpace(buf)), nil
}
