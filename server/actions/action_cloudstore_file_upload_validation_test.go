package actions

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/artpar/api2go/v2"
	_ "github.com/artpar/rclone/backend/local"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/resource"
)

func TestCloudStoreUploadRejectsInvalidBase64BeforeQueueing(t *testing.T) {
	root := t.TempDir()
	cache := t.TempDir()
	t.Setenv("DAPTIN_CACHE_FOLDER", cache)
	files := []interface{}{
		map[string]interface{}{"name": "valid.txt", "file": "data:text/plain;base64,b2s="},
		map[string]interface{}{"name": "invalid.txt", "file": "data:text/plain;base64,@@@not-base64@@@"},
	}
	_, responses, errs := (&fileUploadActionPerformer{}).DoAction(actionresponse.Outcome{}, map[string]interface{}{
		"root_path": root, "store_type": "local", "store_provider": "localstore", "path": "", "file": files,
	}, nil)
	if len(errs) != 1 || len(responses) != 0 {
		t.Fatalf("invalid upload responses = %v, errors = %v", responses, errs)
	}
	var uploadError api2go.HTTPError
	if !errors.As(errs[0], &uploadError) || uploadError.Status() != http.StatusBadRequest {
		t.Fatalf("invalid upload error = %v, want HTTP 400", errs[0])
	}
	for _, dir := range []string{root, cache} {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) != 0 {
			t.Fatalf("invalid upload left files in %s: %v, %v", dir, entries, err)
		}
	}
}

func TestCloudStoreUploadComputesMetadataFromDecodedBytes(t *testing.T) {
	root := t.TempDir()
	t.Setenv("DAPTIN_CACHE_FOLDER", t.TempDir())
	file := map[string]interface{}{"name": "plain.txt", "file": "data:text/plain;base64,aGVsbG8="}
	empty := map[string]interface{}{"name": "empty.txt", "contents": ""}
	_, _, errs := (&fileUploadActionPerformer{}).DoAction(actionresponse.Outcome{}, map[string]interface{}{
		"root_path": root, "store_type": "local", "store_provider": "localstore", "path": "", "file": []interface{}{file, empty},
	}, nil)
	if len(errs) != 0 {
		t.Fatalf("upload: %v", errs)
	}
	if file["size"] != 5 || file["md5"] != resource.GetMD5Hash([]byte("hello")) {
		t.Fatalf("metadata was not computed from uploaded bytes: %#v", file)
	}
	if _, hasType := file["type"]; hasType {
		t.Fatalf("upload unexpectedly inferred a MIME type: %#v", file)
	}
	if empty["size"] != 0 || empty["md5"] != resource.GetMD5Hash(nil) {
		t.Fatalf("valid empty file metadata = %#v", empty)
	}
	waitForPathState(t, filepath.Join(root, "plain.txt"), true)
	waitForPathState(t, filepath.Join(root, "empty.txt"), true)
	got, err := os.ReadFile(filepath.Join(root, "plain.txt"))
	if err != nil || string(got) != "hello" {
		t.Fatalf("uploaded bytes = %q, %v", got, err)
	}
}

func TestCloudStoreUploadTraversalIsBadRequest(t *testing.T) {
	_, _, errs := (&fileUploadActionPerformer{}).DoAction(actionresponse.Outcome{}, map[string]interface{}{
		"root_path": t.TempDir(), "store_type": "local", "store_provider": "localstore", "path": "",
		"file": []interface{}{map[string]interface{}{"name": "../../escape.txt", "file": "dGVzdA=="}},
	}, nil)
	if len(errs) != 1 {
		t.Fatalf("traversal errors = %v", errs)
	}
	var pathError api2go.HTTPError
	if !errors.As(errs[0], &pathError) || pathError.Status() != http.StatusBadRequest {
		t.Fatalf("traversal error = %v, want HTTP 400", errs[0])
	}
}
