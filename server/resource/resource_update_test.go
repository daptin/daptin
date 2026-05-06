package resource

import (
	"reflect"
	"testing"
)

func TestMergeCloudStoreFileSetUpdatesExistingFileMetadata(t *testing.T) {
	existingFiles := []map[string]interface{}{
		{
			"md5":  "old-md5",
			"name": "test_dummy_page.jsx",
			"path": "0195b3d8-7742-7b61-ba87-2ef1056af746/2026-05-06/",
			"size": 1797,
			"src":  "0195b3d8-7742-7b61-ba87-2ef1056af746/2026-05-06//test_dummy_page.jsx",
			"type": "text/javascript",
		},
	}
	seo := []interface{}{
		"<title>Test Dummy Page | 100X Bot</title>",
		"<meta name=\"description\" content=\"A test managed page for development and testing of 100X Bot managed pages functionality.\" />",
	}
	incomingFiles := []interface{}{
		map[string]interface{}{
			"contents": "data:text/javascript;base64,Y29uc29sZS5sb2coJ3Rlc3QnKQ==",
			"md5":      "new-md5",
			"name":     "test_dummy_page.jsx",
			"path":     "0195b3d8-7742-7b61-ba87-2ef1056af746/2026-05-06/",
			"seo":      seo,
			"size":     1800,
			"type":     "text/javascript",
		},
		map[string]interface{}{
			"file": "data:text/plain;base64,bmV3",
			"md5":  "new-file-md5",
			"name": "new_file.txt",
			"path": "0195b3d8-7742-7b61-ba87-2ef1056af746/2026-05-06/",
			"size": 3,
			"type": "text/plain",
		},
	}

	mergedFiles := mergeCloudStoreFileSet(existingFiles, incomingFiles)

	if len(mergedFiles) != 2 {
		t.Fatalf("expected 2 merged files, got %d: %#v", len(mergedFiles), mergedFiles)
	}

	existingFile := mergedFiles[0]
	if existingFile["name"] != "test_dummy_page.jsx" {
		t.Fatalf("expected first file to remain the existing file entry, got %#v", existingFile)
	}
	if existingFile["md5"] != "new-md5" {
		t.Errorf("expected incoming md5 to update existing file, got %v", existingFile["md5"])
	}
	if existingFile["size"] != 1800 {
		t.Errorf("expected incoming size to update existing file, got %v", existingFile["size"])
	}
	if existingFile["src"] != "0195b3d8-7742-7b61-ba87-2ef1056af746/2026-05-06//test_dummy_page.jsx" {
		t.Errorf("expected existing src to be preserved, got %v", existingFile["src"])
	}
	if !reflect.DeepEqual(existingFile["seo"], seo) {
		t.Errorf("expected incoming seo metadata to be stored, got %#v", existingFile["seo"])
	}
	if _, ok := existingFile["contents"]; ok {
		t.Errorf("did not expect contents to be persisted: %#v", existingFile)
	}

	newFile := mergedFiles[1]
	if newFile["name"] != "new_file.txt" {
		t.Fatalf("expected new file to be appended, got %#v", newFile)
	}
	if _, ok := newFile["file"]; ok {
		t.Errorf("did not expect file payload to be persisted: %#v", newFile)
	}
}
