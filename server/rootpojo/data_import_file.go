package rootpojo

import "fmt"

func (s DataFileImport) String() string {
	return fmt.Sprintf("[%v][%v]", s.FileType, s.FilePath)
}

type DataFileImport struct {
	FilePath string
	Entity   string
	FileType string
}
