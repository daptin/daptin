package server

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"github.com/daptin/daptin/server/resource"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"sync/atomic"

	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"

	"github.com/fclairamb/ftpserver/server"
	"github.com/fclairamb/ftpserver/server/log"
)

// DaptinFtpDriver defines a very basic ftpserver driver
type DaptinFtpDriver struct {
	Logger           log.Logger              // Logger
	BaseDir          string                  // Base directory from which to serve file
	tlsConfig        *tls.Config             // TLS config (if applies)
	config           DaptinFtpServerSettings // Our settings
	nbClients        int32                   // Number of clients
	AssetFolderCache resource.AssetFolderCache
	Subsite          resource.SubSite
	CertManager      resource.CertificateManager
}

// ClientDriver defines a very basic client driver
type ClientDriver struct {
	BaseDir string // Base directory from which to server file
}

// DaptinFtpServerSettings defines our settings
type DaptinFtpServerSettings struct {
	Server         server.Settings // Server settings (shouldn't need to be filled)
	MaxConnections int32           // Maximum number of clients that are allowed to connect at the same time
}

// NewDaptinFtpDriver creates a new driver
func NewDaptinFtpDriver(assetCacheFolder resource.AssetFolderCache, subsite resource.SubSite, certManager resource.CertificateManager) (*DaptinFtpDriver, error) {

	drv := &DaptinFtpDriver{
		Logger:           log.NewNopGKLogger(),
		BaseDir:          assetCacheFolder.LocalSyncPath,
		AssetFolderCache: assetCacheFolder,
		Subsite:          subsite,
		CertManager:      certManager,
		config: DaptinFtpServerSettings{
			MaxConnections: 100,
		},
	}

	return drv, nil
}

// GetSettings returns some general settings around the server setup
func (driver *DaptinFtpDriver) GetSettings() (*server.Settings, error) {

	var err error
	// This is the new IP loading change coming from Ray
	if driver.config.Server.PublicHost == "" {
		publicIP := ""

		driver.Logger.Debug("msg", "Fetching our external IP address...")

		if publicIP, err = externalIP(); err != nil {
			driver.Logger.Warn("msg", "Couldn't fetch an external IP", "err", err)
		} else {
			driver.Logger.Debug(
				"msg", "Fetched our external IP address",
				"action", "external_ip.fetched",
				"ipAddress", publicIP)
		}

		// Adding a special case for loopback clients (issue #74)
		driver.config.Server.PublicIPResolver = func(cc server.ClientContext) (string, error) {
			driver.Logger.Debug("msg", "Resolving public IP", "remoteAddr", cc.RemoteAddr())

			if strings.HasPrefix(cc.RemoteAddr().String(), "127.0.0.1") {
				return "127.0.0.1", nil
			}

			return publicIP, nil
		}
	}

	return &driver.config.Server, nil
}

// GetTLSConfig returns a TLS Certificate to use
func (driver *DaptinFtpDriver) GetTLSConfig() (*tls.Config, error) {
	if driver.tlsConfig == nil {
		driver.Logger.Info("msg", "Loading certificate")

		if cert, err := driver.getCertificate(); err == nil {
			driver.tlsConfig = &tls.Config{
				NextProtos:   []string{"ftp"},
				Certificates: []tls.Certificate{*cert},
			}
		} else {
			return nil, err
		}
	}

	return driver.tlsConfig, nil
}

// Live generation of a self-signed certificate
// This implementation of the driver doesn't load a certificate from a file on purpose. But it any proper implementation
// should most probably load the certificate from a file using tls.LoadX509KeyPair("cert_pub.pem", "cert_priv.pem").
func (driver *DaptinFtpDriver) getCertificate() (*tls.Certificate, error) {
	driver.Logger.Info("msg", "Creating certificate")

	priv, err := rsa.GenerateKey(rand.Reader, 2048)

	if err != nil {
		driver.Logger.Error("msg", "Could not generate key", "err", err)
		return nil, err
	}

	now := time.Now().UTC()

	subject := pkix.Name{
		CommonName:   "localhost",
		Organization: []string{"FTPServer"},
	}

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1337),
		Subject:               subject,
		DNSNames:              []string{"localhost"},
		SignatureAlgorithm:    x509.SHA256WithRSA,
		PublicKeyAlgorithm:    x509.RSA,
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(time.Hour * 24 * 7),
		SubjectKeyId:          []byte{1, 2, 3, 4, 5},
		BasicConstraintsValid: true,
		IsCA:                  false,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature,
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)

	if err != nil {
		driver.Logger.Error("msg", "Could not create cert", "err", err)
		return nil, err
	}

	var certPem, keyPem bytes.Buffer
	if err2 := pem.Encode(&certPem, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err2 != nil {
		return nil, err2
	}

	if err2 := pem.Encode(
		&keyPem,
		&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}); err2 != nil {
		return nil, err2
	}

	c, err := tls.X509KeyPair(certPem.Bytes(), keyPem.Bytes())

	return &c, err
}

// WelcomeUser is called to send the very first welcome message
func (driver *DaptinFtpDriver) WelcomeUser(cc server.ClientContext) (string, error) {
	nbClients := atomic.AddInt32(&driver.nbClients, 1)
	if nbClients > driver.config.MaxConnections {
		return "Cannot accept any additional client", fmt.Errorf(
			"too many clients: %d > % d",
			driver.nbClients,
			driver.config.MaxConnections)
	}

	cc.SetDebug(true)
	// This will remain the official name for now
	return fmt.Sprintf(
			"Welcome on ftpserver, you're on dir %s, your ID is %d, your IP:port is %s, we currently have %d clients connected",
			driver.BaseDir,
			cc.ID(),
			cc.RemoteAddr(),
			nbClients),
		nil
}

// AuthUser authenticates the user and selects an handling driver
func (driver *DaptinFtpDriver) AuthUser(cc server.ClientContext, user, pass string) (server.ClientHandlingDriver, error) {
	for _, act := range driver.config.Users {
		if act.User == user && act.Pass == pass {
			// If we are authenticated, we can return a client driver containing *our* basedir
			baseDir := driver.BaseDir + string(os.PathSeparator) + act.Dir
			if err := os.MkdirAll(baseDir, 0750); err != nil {
				return nil, fmt.Errorf("could not create user dir: %v", err)
			}

			return &ClientDriver{BaseDir: baseDir}, nil
		}
	}

	return nil, fmt.Errorf("could not authenticate you")
}

// UserLeft is called when the user disconnects, even if he never authenticated
func (driver *DaptinFtpDriver) UserLeft(cc server.ClientContext) {
	atomic.AddInt32(&driver.nbClients, -1)
}

// ChangeDirectory changes the current working directory
func (driver *ClientDriver) ChangeDirectory(cc server.ClientContext, directory string) error {
	if directory == DirDebug {
		cc.SetDebug(!cc.Debug())
		return nil
	} else if directory == DirVirtual {
		return nil
	}

	_, err := os.Stat(driver.BaseDir + directory)

	return err
}

// MakeDirectory creates a directory
func (driver *ClientDriver) MakeDirectory(cc server.ClientContext, directory string) error {
	return os.Mkdir(driver.BaseDir+directory, 0750)
}

// ListFiles lists the files of a directory
func (driver *ClientDriver) ListFiles(cc server.ClientContext, directory string) ([]os.FileInfo, error) {
	if directory == DirVirtual {
		files := make([]os.FileInfo, 0)
		files = append(files,
			virtualFileInfo{
				name: "localpath.txt",
				mode: os.FileMode(0666),
				size: 1024,
			},
			virtualFileInfo{
				name: "file2.txt",
				mode: os.FileMode(0666),
				size: 2048,
			},
		)

		return files, nil
	} else if directory == DirDebug {
		return make([]os.FileInfo, 0), nil
	}

	files, err := ioutil.ReadDir(directory)

	// We add a virtual dir
	if directory == "/" && err == nil {
		files = append(files, virtualFileInfo{
			name: "virtual",
			mode: os.FileMode(0666) | os.ModeDir,
			size: 4096,
		})
	}

	return files, err
}

// OpenFile opens a file in 3 possible modes: read, write, appending write (use appropriate flags)
func (driver *ClientDriver) OpenFile(cc server.ClientContext, path string, flag int) (server.FileStream, error) {
	if strings.HasPrefix(path, DirVirtual) {
		if path == DirVirtual+"/localpath.txt" {
			return &virtualFile{content: []byte(driver.BaseDir)}, nil
		}

		return nil, fmt.Errorf("this is a virtual directory, only reading of localpath.txt has been implemented")
	}

	path = driver.BaseDir + path

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
	switch path {
	case DirVirtual:
		return &virtualFileInfo{name: "virtual", size: 4096, mode: os.ModeDir}, nil
	case DirDebug:
		return &virtualFileInfo{name: "debug", size: 4096, mode: os.ModeDir}, nil
	}

	path = driver.BaseDir + path

	return os.Stat(path)
}

// CanAllocate gives the approval to allocate some data
func (driver *ClientDriver) CanAllocate(cc server.ClientContext, size int) (bool, error) {
	return true, nil
}

// ChmodFile changes the attributes of the file
func (driver *ClientDriver) ChmodFile(cc server.ClientContext, path string, mode os.FileMode) error {
	path = driver.BaseDir + path

	return os.Chmod(path, mode)
}

// DeleteFile deletes a file or a directory
func (driver *ClientDriver) DeleteFile(cc server.ClientContext, path string) error {
	path = driver.BaseDir + path

	return os.Remove(path)
}

// RenameFile renames a file or a directory
func (driver *ClientDriver) RenameFile(cc server.ClientContext, from, to string) error {
	from = driver.BaseDir + from
	to = driver.BaseDir + to

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
