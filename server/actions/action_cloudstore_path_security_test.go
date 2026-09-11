package actions

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/artpar/rclone/backend/local"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/google/uuid"
)

func TestCloudStoreWriteActionsRejectPathsOutsideLocalRoot(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "outside-canary.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}

	assertRejected := func(name string, errs []error) {
		t.Helper()
		if len(errs) == 0 {
			t.Fatalf("%s accepted an escaping path", name)
		}
		got, err := os.ReadFile(outside)
		if err != nil {
			t.Fatalf("%s changed or removed the outside canary: %v", name, err)
		}
		if string(got) != "outside" {
			t.Fatalf("%s changed outside canary to %q", name, got)
		}
	}

	_, _, errs := (&cloudStoreFileDeleteActionPerformer{}).DoAction(actionresponse.Outcome{}, map[string]interface{}{
		"root_path":      root,
		"store_type":     "local",
		"store_provider": "localstore",
		"path":           "../outside-canary.txt",
	}, nil)
	assertRejected("delete", errs)

	_, _, errs = (&cloudStorePathMoveActionPerformer{}).DoAction(actionresponse.Outcome{}, map[string]interface{}{
		"root_path":      root,
		"store_type":     "local",
		"store_provider": "localstore",
		"source":         "../outside-canary.txt",
		"destination":    "inside.txt",
	}, nil)
	assertRejected("move", errs)

	_, _, errs = (&cloudStoreFolderCreateActionPerformer{}).DoAction(actionresponse.Outcome{}, map[string]interface{}{
		"root_path":      root,
		"store_type":     "local",
		"store_provider": "localstore",
		"path":           "../outside-dir",
		"name":           "created",
	}, nil)
	assertRejected("create folder", errs)

	_, _, errs = (&fileUploadActionPerformer{}).DoAction(actionresponse.Outcome{}, map[string]interface{}{
		"root_path":      root,
		"store_type":     "local",
		"store_provider": "localstore",
		"path":           "../outside-dir",
		"file": []interface{}{map[string]interface{}{
			"name": "created.txt",
			"file": base64.StdEncoding.EncodeToString([]byte("created")),
		}},
	}, nil)
	assertRejected("upload", errs)

	_, _, errs = (&fileUploadActionPerformer{}).DoAction(actionresponse.Outcome{}, map[string]interface{}{
		"root_path":      root,
		"store_type":     "local",
		"store_provider": "localstore",
		"path":           "",
		"file": []interface{}{map[string]interface{}{
			"name": "created.txt",
			"path": "../outside-dir",
			"file": base64.StdEncoding.EncodeToString([]byte("created")),
		}},
	}, nil)
	assertRejected("upload file path", errs)

	_, _, errs = (&cloudStoreSiteCreateActionPerformer{}).DoAction(actionresponse.Outcome{}, map[string]interface{}{
		"cloud_store_id": uuid.NewString(),
		"root_path":      root,
		"store_type":     "local",
		"store_provider": "localstore",
		"path":           "../outside-site",
		"hostname":       "outside.example",
		"site_type":      "static",
	}, nil)
	assertRejected("create site", errs)
}

func TestCloudStoreDeleteRejectsStorageRoot(t *testing.T) {
	root := t.TempDir()
	_, _, errs := (&cloudStoreFileDeleteActionPerformer{}).DoAction(actionresponse.Outcome{}, map[string]interface{}{
		"root_path":      root,
		"store_type":     "local",
		"store_provider": "localstore",
		"path":           "/",
	}, nil)
	if len(errs) == 0 {
		t.Fatal("delete accepted the storage root")
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("storage root was changed: %v", err)
	}
}

func TestCloudStoreActionsUseConfinedLocalStorage(t *testing.T) {
	root := t.TempDir()
	t.Setenv("DAPTIN_CACHE_FOLDER", t.TempDir())

	_, _, errs := (&cloudStoreFolderCreateActionPerformer{}).DoAction(actionresponse.Outcome{}, map[string]interface{}{
		"root_path":      root,
		"store_type":     "local",
		"store_provider": "localstore",
		"path":           "/nested",
		"name":           "folder",
	}, nil)
	if len(errs) != 0 {
		t.Fatalf("create folder: %v", errs)
	}
	waitForPathState(t, filepath.Join(root, "nested", "folder"), true)

	_, _, errs = (&fileUploadActionPerformer{}).DoAction(actionresponse.Outcome{}, map[string]interface{}{
		"root_path":      root,
		"store_type":     "local",
		"store_provider": "localstore",
		"path":           "/nested/folder",
		"file": []interface{}{
			map[string]interface{}{
				"name": "created.txt",
				"path": "first",
				"file": base64.StdEncoding.EncodeToString([]byte("created")),
			},
			map[string]interface{}{
				"name": "second.txt",
				"path": "second",
				"file": base64.StdEncoding.EncodeToString([]byte("second")),
			},
		},
	}, nil)
	if len(errs) != 0 {
		t.Fatalf("upload: %v", errs)
	}
	uploaded := filepath.Join(root, "nested", "folder", "first", "created.txt")
	waitForPathState(t, uploaded, true)
	if got, err := os.ReadFile(uploaded); err != nil || string(got) != "created" {
		t.Fatalf("uploaded file = %q, %v", got, err)
	}
	second := filepath.Join(root, "nested", "folder", "second", "second.txt")
	waitForPathState(t, second, true)
	if got, err := os.ReadFile(second); err != nil || string(got) != "second" {
		t.Fatalf("second uploaded file = %q, %v", got, err)
	}

	moved := filepath.Join(root, "nested", "moved.txt")
	_, _, errs = (&cloudStorePathMoveActionPerformer{}).DoAction(actionresponse.Outcome{}, map[string]interface{}{
		"root_path":      root,
		"store_type":     "local",
		"store_provider": "localstore",
		"source":         "/nested/folder/first/created.txt",
		"destination":    "/nested/moved.txt",
	}, nil)
	if len(errs) != 0 {
		t.Fatalf("move: %v", errs)
	}
	waitForPathState(t, moved, true)
	waitForPathState(t, uploaded, false)

	_, _, errs = (&cloudStoreFileDeleteActionPerformer{}).DoAction(actionresponse.Outcome{}, map[string]interface{}{
		"root_path":      root,
		"store_type":     "local",
		"store_provider": "localstore",
		"path":           "/nested/moved.txt",
	}, nil)
	if len(errs) != 0 {
		t.Fatalf("delete: %v", errs)
	}
	waitForPathState(t, moved, false)
}

func waitForPathState(t *testing.T, name string, wantExists bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		_, err := os.Stat(name)
		exists := err == nil
		if exists == wantExists {
			return
		}
		if err != nil && !os.IsNotExist(err) {
			t.Fatalf("stat %s: %v", name, err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("path %s existence = %v, want %v", name, exists, wantExists)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
