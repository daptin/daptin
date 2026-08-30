package resource

import (
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
)

// CleanUpConfigFiles removes transient schema files after they have been read.
func CleanUpConfigFiles() {
	patterns := []string{"*_uploaded_*"}
	if schemaFolder := os.Getenv("DAPTIN_SCHEMA_FOLDER"); schemaFolder != "" {
		patterns = append(patterns, filepath.Join(schemaFolder, "*_uploaded_*"))
	}

	for _, pattern := range patterns {
		files, _ := filepath.Glob(pattern)
		log.Debugf("Clean up uploaded config files: %v", files)
		for _, fileName := range files {
			if err := os.Remove(fileName); err != nil {
				CheckErr(err, "Failed to delete uploaded schema file: %s", fileName)
			}
		}
	}
}
