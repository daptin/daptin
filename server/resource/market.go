package resource

import (
	"github.com/artpar/go.uuid"
	log "github.com/sirupsen/logrus"
	libgit "github.com/libgit2/git2go"
	"io/ioutil"
	"os"
)

type Marketplace struct {
	Endpoint    string
	RootPath    string `db:"root_path"`
	Name        string `db:"name"`
	Permission  int    `json:"-"`
	UserId      *int   `json:"-" db:"user_account_id"`
	ReferenceId string `json:"-" db:"reference_id"`
}

type MarketplaceService struct {
	gitRepo     *libgit.Repository
	repoPath    string
	Marketplace Marketplace
}

type MarketPackage struct {
	Name     string
	Summary  string
	Location string
}

func (mp *MarketplaceService) RefreshRepository() error {

	err := mp.gitRepo.CheckoutHead(&libgit.CheckoutOpts{
		Strategy: libgit.CheckoutUseTheirs,
	})

	if err != nil {
		return err
	}

	return err

}

func (mp *MarketplaceService) GetPackage(packageName string) *MarketPackage {
	packageList, err := mp.GetPackageList()
	if err != nil {
		return nil
	}

	for _, pack := range packageList {
		if pack.Name == packageName {
			pack.Location = mp.repoPath + mp.Marketplace.RootPath + pack.Name
			return &pack
		}
	}

	return nil

}

func (mp *MarketplaceService) PackageExists(packageName string) bool {

	packageList, err := mp.GetPackageList()
	if err != nil {
		return false
	}

	for _, pack := range packageList {
		if pack.Name == packageName {
			return true
		}
	}

	return false

}

func (mp *MarketplaceService) GetPackageList() ([]MarketPackage, error) {

	packages := []MarketPackage{}

	baseDir := mp.repoPath + mp.Marketplace.RootPath

	files, err := ioutil.ReadDir(baseDir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() {

			packageName := file.Name()
			readmePath := baseDir + "README.md"
			summary := ""

			if _, err := os.Stat(readmePath); err == nil {
				summaryBytes, err := ioutil.ReadFile(readmePath)
				CheckErr(err, "Failed to read readme for [%v]", mp.Marketplace.Endpoint+"/"+packageName)
				summary = string(summaryBytes)
			}

			marketPackage := MarketPackage{
				Name:    packageName,
				Summary: summary,
			}
			packages = append(packages, marketPackage)
		}
	}

	return packages, nil

}

func NewMarketplaceService(marketplace Marketplace) (*MarketplaceService, error) {

	tempDir := os.TempDir()
	u, _ := uuid.NewV4()
	tempRepoDir := tempDir + "/" + u.String()
	log.Infof("Creating directory  [%v] for marketplace [%v]", tempRepoDir, marketplace.Endpoint)
	l := len(marketplace.RootPath)
	if l == 0 || marketplace.RootPath[l-1] != '/' {
		marketplace.RootPath = marketplace.RootPath + "/"
	}

	err := os.Mkdir(tempRepoDir, 0777)
	CheckErr(err, "Failed to create target path for marketplace repo")

	gitRepo, err := libgit.Clone(marketplace.Endpoint, tempRepoDir, &libgit.CloneOptions{

	})

	err = gitRepo.CheckoutHead(&libgit.CheckoutOpts{
		Strategy: libgit.CheckoutUseTheirs,
	})

	//gitRepo, err := git.PlainClone(tempRepoDir, false, &git.CloneOptions{
	//	URL: marketplace.Endpoint,
	//})

	marketPlaceController := MarketplaceService{
		gitRepo:     gitRepo,
		Marketplace: marketplace,
		repoPath:    tempRepoDir,
	}
	return &marketPlaceController, err
}
