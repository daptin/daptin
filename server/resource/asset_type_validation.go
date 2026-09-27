package resource

import (
	"fmt"
	"mime"
	"path"
	"strings"
)

// ValidateAssetType checks the metadata supplied for a restricted asset column.
// It does not inspect file contents.
func ValidateAssetType(columnType, name, declaredType, content string) error {
	columnType = strings.ToLower(columnType)
	if columnType != "image" && columnType != "video" && !strings.HasPrefix(columnType, "file.") {
		return nil
	}
	if columnType == "file.*" {
		return nil
	}

	ext := strings.ToLower(path.Ext(name))
	if ext == "" || declaredType == "" {
		return fmt.Errorf("filename extension and MIME type are required")
	}
	mediaType, _, err := mime.ParseMediaType(declaredType)
	if err != nil {
		return fmt.Errorf("invalid MIME type")
	}
	mediaType = strings.ToLower(mediaType)
	expectedType := mime.TypeByExtension(ext)
	if expectedType == "" && (columnType == "image" || columnType == "video") {
		return fmt.Errorf("unsupported file extension %q", ext)
	}
	if expectedType != "" {
		expectedType, _, err = mime.ParseMediaType(expectedType)
		if err != nil || !strings.EqualFold(mediaType, expectedType) {
			return fmt.Errorf("filename extension and MIME type do not match")
		}
	}

	switch columnType {
	case "image", "video":
		if !strings.HasPrefix(mediaType, columnType+"/") {
			return fmt.Errorf("file type is not %s", columnType)
		}
	default:
		allowed := false
		for _, suffix := range strings.Split(strings.TrimPrefix(columnType, "file."), "|") {
			if ext == "."+suffix {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("file extension %q is not allowed", ext)
		}
	}

	if len(content) >= 5 && strings.EqualFold(content[:5], "data:") {
		header, _, ok := strings.Cut(content[5:], ",")
		if !ok {
			return fmt.Errorf("invalid data URL")
		}
		contentType, _, err := mime.ParseMediaType(strings.Split(header, ";")[0])
		if err != nil || !strings.EqualFold(contentType, mediaType) {
			return fmt.Errorf("data URL MIME type does not match file metadata")
		}
	}
	return nil
}

func validateAssetColumnValue(columnType string, value interface{}) error {
	if columnType != "image" && columnType != "video" && !strings.HasPrefix(columnType, "file.") {
		return nil
	}
	if columnType == "file.*" || value == nil {
		return nil
	}
	switch raw := value.(type) {
	case string:
		if !strings.HasPrefix(strings.TrimSpace(raw), "[") {
			return nil // Inline binary values have no file metadata.
		}
	case []byte:
		if !strings.HasPrefix(strings.TrimSpace(string(raw)), "[") {
			return nil
		}
	}
	files, err := assetColumnFiles(value)
	if err != nil {
		return fmt.Errorf("invalid asset value: %w", err)
	}
	for _, file := range files {
		if isInlineFileAsset(file) {
			continue
		}
		name, _ := file["name"].(string)
		mediaType, _ := file["type"].(string)
		content, _ := file["file"].(string)
		if err := ValidateAssetType(columnType, name, mediaType, content); err != nil {
			return err
		}
	}
	return nil
}
