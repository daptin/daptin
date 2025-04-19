package subsite

import "net/http"

type StaticFsWithDefaultIndex struct {
	system    http.FileSystem
	pageOn404 string
}

func (spf *StaticFsWithDefaultIndex) Open(name string) (http.File, error) {
	//log.Printf("Service file from static path: %s/%s", spf.subPath, name)

	f, err := spf.system.Open(name)
	if err != nil {
		return spf.system.Open(spf.pageOn404)
	}
	return f, nil
}
