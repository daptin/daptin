package subsite

import (
	"strconv"
	"strings"
)

// AcceptsGzip reports whether gzip is permitted by Accept-Encoding.
func AcceptsGzip(value string) bool {
	wildcard := false
	for _, item := range strings.Split(value, ",") {
		parts := strings.Split(strings.TrimSpace(item), ";")
		name := strings.TrimSpace(parts[0])
		if !strings.EqualFold(name, "gzip") && name != "*" {
			continue
		}
		quality := 1.0
		for _, parameter := range parts[1:] {
			keyValue := strings.SplitN(strings.TrimSpace(parameter), "=", 2)
			if len(keyValue) == 2 && strings.EqualFold(strings.TrimSpace(keyValue[0]), "q") {
				parsed, err := strconv.ParseFloat(strings.TrimSpace(keyValue[1]), 64)
				if err != nil || parsed < 0 || parsed > 1 {
					return false
				}
				quality = parsed
			}
		}
		if strings.EqualFold(name, "gzip") {
			return quality > 0
		}
		wildcard = quality > 0
	}
	return wildcard
}

// GzipETag gives encoded bytes a validator distinct from the identity body.
func GzipETag(etag string) string {
	if etag == "" {
		return ""
	}
	return strings.TrimSuffix(etag, `"`) + "-gzip\""
}
