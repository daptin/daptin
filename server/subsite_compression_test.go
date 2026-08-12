package server

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestAcceptsGzip(t *testing.T) {
	tests := []struct {
		header string
		want   bool
	}{
		{"gzip", true},
		{"br, gzip", true},
		{"GZip; q=0.5", true},
		{"*;q=1", true},
		{"gzip;q=0", false},
		{"br", false},
		{"", false},
	}
	for _, test := range tests {
		if got := acceptsGzip(test.header); got != test.want {
			t.Errorf("acceptsGzip(%q) = %v, want %v", test.header, got, test.want)
		}
	}
}

func TestOpenPrecompressedSubsiteFileConcurrent(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "assets", "app.js")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatal(err)
	}
	content := strings.Repeat("const benchmark = 'daptin';\n", 4096)
	if err := os.WriteFile(sourcePath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	const workers = 32
	errors := make(chan error, workers)
	var group sync.WaitGroup
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			source, err := os.Open(sourcePath)
			if err != nil {
				errors <- err
				return
			}
			defer source.Close()
			info, err := source.Stat()
			if err != nil {
				errors <- err
				return
			}
			compressed, err := openPrecompressedSubsiteFile(source, info, root, "assets/app.js")
			if err != nil {
				errors <- err
				return
			}
			defer compressed.Close()
			reader, err := gzip.NewReader(compressed)
			if err != nil {
				errors <- err
				return
			}
			decompressed, err := io.ReadAll(reader)
			_ = reader.Close()
			if err != nil {
				errors <- err
				return
			}
			if string(decompressed) != content {
				errors <- io.ErrUnexpectedEOF
			}
		}()
	}
	group.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}

	compressedPath := filepath.Join(root, subsiteCompressedCacheDirectory, "assets", "app.js.gz")
	if info, err := os.Stat(compressedPath); err != nil || info.Size() == 0 {
		t.Fatalf("expected reusable compressed sidecar at %s: %v", compressedPath, err)
	}
}
