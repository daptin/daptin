package resource

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	//"golang.org/x/oauth2"
	"io"
	"io/ioutil"
	"os"
)

type MarketplacePackageInstallActionPerformer struct {
	cruds     map[string]*DbResource
	marketMap map[string]*MarketplaceService
}

func (d *MarketplacePackageInstallActionPerformer) Name() string {
	return "marketplace.package.install"
}

func (d *MarketplacePackageInstallActionPerformer) DoAction(request ActionRequest, inFieldMap map[string]interface{}) ([]ActionResponse, []error) {

	marketReferenceId := inFieldMap["marketplace_id"].(string)
	marketplaceHandler, ok := d.marketMap[marketReferenceId]

	if !ok {
		return nil, []error{fmt.Errorf("Unknown market")}
	}

	packageName := inFieldMap["package_name"].(string)
	if !marketplaceHandler.PackageExists(packageName) {
		return nil, []error{fmt.Errorf("Invalid package name: %v", packageName)}
	}

	pack := marketplaceHandler.GetPackage(packageName)
	if pack == nil {
		return nil, []error{fmt.Errorf("Invalid package name: %v", packageName)}
	}

	files, err := ioutil.ReadDir(pack.Location)
	if err != nil {
		return nil, []error{err}
	}

	packageRoot := pack.Location + "/"
	log.Infof("Package root: [%v]", packageRoot)
	for _, file := range files {
		log.Infof("Copy schema for installation [%v]", file)
		err = CopyFile(packageRoot+file.Name(), file.Name())
		CheckErr(err, "Failed to link file")
	}

	go restart()

	return successResponses, nil
}

func (h *MarketplacePackageInstallActionPerformer) refresh() {

	markets, err := h.cruds["marketplace"].GetAllMarketplaces()
	CheckErr(err, "Failed to get market places")

	for _, market := range markets {
		handler, err := NewMarketplaceService(market)
		CheckErr(err, "Failed to connect to market: %v", market)
		h.marketMap[market.ReferenceId] = handler
	}

}

func NewMarketplacePackageInstaller(initConfig *CmsConfig, cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	services := make(map[string]*MarketplaceService)
	initConfig.MarketplaceHandlers = services

	handler := MarketplacePackageInstallActionPerformer{
		cruds:     cruds,
		marketMap: services,
	}

	go handler.refresh()

	return &handler, nil

}

// CopyFile copies a file from src to dst. If src and dst files exist, and are
// the same, then return success. Otherise, attempt to create a hard link
// between the two files. If that fail, copy the file contents from src to dst.
func CopyFile(src, dst string) (err error) {
	sfi, err := os.Stat(src)
	if err != nil {
		return
	}
	if !sfi.Mode().IsRegular() {
		// cannot copy non-regular files (e.g., directories,
		// symlinks, devices, etc.)
		return fmt.Errorf("CopyFile: non-regular source file %s (%q)", sfi.Name(), sfi.Mode().String())
	}
	dfi, err := os.Stat(dst)
	if err != nil {
		if !os.IsNotExist(err) {
			return
		}
	} else {
		if !(dfi.Mode().IsRegular()) {
			return fmt.Errorf("CopyFile: non-regular destination file %s (%q)", dfi.Name(), dfi.Mode().String())
		}
		if os.SameFile(sfi, dfi) {
			return
		}
	}
	if err = os.Link(src, dst); err == nil {
		return
	}
	err = copyFileContents(src, dst)
	return
}

// copyFileContents copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the contents
// of the source file.
func copyFileContents(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}
