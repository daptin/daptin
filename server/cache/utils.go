package cache

import "strings"

// ShouldCompress determines if a file should be compressed based on its content type
func ShouldCompress(contentType string) bool {
	compressibleTypes := []string{
		"text/",
		"application/javascript",
		"application/json",
		"application/xml",
		"application/xhtml+xml",
		"image/svg+xml",
		"application/font-woff",
		"application/font-woff2",
		"application/vnd.ms-fontobject",
		"application/x-font-ttf",
		"font/opentype",
		"application/octet-stream",
	}

	// Don't compress already compressed formats
	alreadyCompressed := []string{
		"image/jpeg",
		"image/png",
		"image/gif",
		"image/webp",
		"audio/",
		"video/",
		"application/zip",
		"application/gzip",
		"application/x-gzip",
		"application/x-compressed",
		"application/x-zip-compressed",
	}

	// Check if content is already compressed
	for _, t := range alreadyCompressed {
		if strings.Contains(contentType, t) {
			return false
		}
	}

	// Check if content is compressible
	for _, t := range compressibleTypes {
		if strings.Contains(contentType, t) {
			return true
		}
	}

	return false
}
