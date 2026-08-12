package server

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const subsiteCompressedCacheDirectory = ".daptin-compressed"

var subsiteGzipExcludedExtensions = map[string]struct{}{
	".pdf": {}, ".mp4": {}, ".jpg": {}, ".jpeg": {}, ".png": {},
	".wav": {}, ".gif": {}, ".mp3": {}, ".webp": {}, ".zip": {},
	".gz": {}, ".br": {}, ".7z": {}, ".rar": {},
}

type subsiteCompressedFileState struct {
	mu            sync.Mutex
	sourceSize    int64
	sourceModNano int64
	path          string
}

var subsiteCompressedFiles sync.Map

func acceptsGzip(value string) bool {
	for _, item := range strings.Split(value, ",") {
		parts := strings.Split(strings.TrimSpace(item), ";")
		if !strings.EqualFold(strings.TrimSpace(parts[0]), "gzip") && strings.TrimSpace(parts[0]) != "*" {
			continue
		}
		quality := 1.0
		for _, parameter := range parts[1:] {
			keyValue := strings.SplitN(strings.TrimSpace(parameter), "=", 2)
			if len(keyValue) == 2 && strings.EqualFold(keyValue[0], "q") {
				parsed, err := strconv.ParseFloat(keyValue[1], 64)
				if err != nil {
					return false
				}
				quality = parsed
			}
		}
		return quality > 0
	}
	return false
}

func shouldGzipSubsitePath(path string) bool {
	_, excluded := subsiteGzipExcludedExtensions[strings.ToLower(filepath.Ext(path))]
	return !excluded
}

func gzipETag(etag string) string {
	return strings.TrimSuffix(etag, `"`) + "-gzip\""
}

func appendVary(headerValue, value string) string {
	for _, existing := range strings.Split(headerValue, ",") {
		if strings.EqualFold(strings.TrimSpace(existing), value) {
			return headerValue
		}
	}
	if headerValue == "" {
		return value
	}
	return headerValue + ", " + value
}

func openPrecompressedSubsiteFile(source *os.File, sourceInfo os.FileInfo, cacheRoot, relativePath string) (*os.File, error) {
	if cacheRoot == "" {
		return nil, fmt.Errorf("subsite cache root is empty")
	}
	relativePath = strings.TrimPrefix(filepath.Clean(relativePath), string(filepath.Separator))
	if relativePath == "" || relativePath == "." || strings.HasPrefix(relativePath, "..") {
		return nil, fmt.Errorf("invalid subsite compression path %q", relativePath)
	}

	key := cacheRoot + "\x00" + source.Name()
	stateValue, _ := subsiteCompressedFiles.LoadOrStore(key, &subsiteCompressedFileState{})
	state := stateValue.(*subsiteCompressedFileState)
	state.mu.Lock()
	defer state.mu.Unlock()

	if state.sourceSize == sourceInfo.Size() &&
		state.sourceModNano == sourceInfo.ModTime().UnixNano() && state.path != "" {
		if compressed, err := os.Open(state.path); err == nil {
			return compressed, nil
		}
	}

	compressedPath := filepath.Join(cacheRoot, subsiteCompressedCacheDirectory, relativePath+".gz")
	if err := os.MkdirAll(filepath.Dir(compressedPath), 0o755); err != nil {
		return nil, err
	}
	temporary, err := os.CreateTemp(filepath.Dir(compressedPath), ".gzip-*")
	if err != nil {
		return nil, err
	}
	temporaryPath := temporary.Name()
	removeTemporary := true
	defer func() {
		_ = temporary.Close()
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()

	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	compressor, err := gzip.NewWriterLevel(temporary, gzip.DefaultCompression)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(compressor, source); err != nil {
		_ = compressor.Close()
		return nil, err
	}
	if err := compressor.Close(); err != nil {
		return nil, err
	}
	if err := temporary.Close(); err != nil {
		return nil, err
	}
	if err := os.Rename(temporaryPath, compressedPath); err != nil {
		return nil, err
	}
	removeTemporary = false
	state.sourceSize = sourceInfo.Size()
	state.sourceModNano = sourceInfo.ModTime().UnixNano()
	state.path = compressedPath
	return os.Open(compressedPath)
}
