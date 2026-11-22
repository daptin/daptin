package main

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/png"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/daptin/daptin/server/auth"
)

func TestImageProcessingLimitsRealE2E(t *testing.T) {
	options := transportE2EDaptinOptions{connectionString: filepath.Join(t.TempDir(), "images.db"), schema: `Tables:
  - TableName: image_processing_probe
    Permission: 2097151
    DefaultPermission: 0
    Columns:
      - Name: title
        DataType: varchar(100)
        ColumnType: label
      - Name: photo
        DataType: text
        ColumnType: image
        IsNullable: true
        IsForeignKey: true
        ForeignKeyData:
          DataSource: cloud_store
          Namespace: image-processing-store
          KeyName: images
`}
	f := startDaptinE2E(t, options)
	admin := f.SignupAdmin(t)
	accessGroupsE2ECreateRecord(t, f.Client, f.URL, admin, "cloud_store", map[string]interface{}{
		"name": "image-processing-store", "store_type": "local", "store_provider": "local",
		"root_path": t.TempDir(), "store_parameters": "{}",
	})
	f.Process.stopProcess()
	f = startDaptinE2E(t, options)
	reader := accessGroupsE2ESignupSigninUser(t, f.Client, f.URL, admin, "image-processing-reader")
	var original bytes.Buffer
	if err := png.Encode(&original, image.NewNRGBA(image.Rect(0, 0, 1, 2))); err != nil {
		t.Fatal(err)
	}
	createImage := func(name string, data []byte) string {
		t.Helper()
		id := accessGroupsE2ECreateRecord(t, f.Client, f.URL, admin, "image_processing_probe", map[string]interface{}{"title": name})
		requestAssetE2E(t, f.Client, http.MethodPost,
			f.URL+"/asset/image_processing_probe/"+id+"/photo/upload?operation=stream&filename="+name+".png",
			admin, bytes.NewReader(data), "image/png", http.StatusOK)
		return id
	}
	id := createImage("tiny", original.Bytes())
	assetURL := f.URL + "/asset/image_processing_probe/" + id + "/photo"
	get := func(target, token string, want int) []byte {
		t.Helper()
		return requestAssetE2E(t, f.Client, http.MethodGet, target, token, nil, "", want)
	}
	assertSize := func(data []byte, width, height int) {
		t.Helper()
		config, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		if config.Width != width || config.Height != height {
			t.Fatalf("output = %dx%d, want %dx%d", config.Width, config.Height, width, height)
		}
	}
	invalid := []string{
		"resize=30000,30000,Box", "resize=4097,1,Lanczos", "resize=1,4097,Linear",
		"resize=2147483648,1,Box", "resize=-1,1,Box", "resize=oops,1,Box",
		"resize=1,1", "resize=0,0,Box", "resize=4096,0,Box",
		"resize=30000,30000,Box&cropToSize=1,1,Center",
	}
	for _, filter := range []string{"boxblur", "gaussianblur", "edgedetection", "dilate", "erode", "median", "gaussianBlur"} {
		for _, value := range []string{"30000", "1000000000", "32.00000001", "NaN", "%2BInf", "-1", "oops"} {
			invalid = append(invalid, filter+"="+value)
		}
	}
	invalid = append(invalid, "dilate=0.25", "erode=1.25", "median=31.25",
		"resize=2048,2048,Box&gaussianBlur=3&cropToSize=1,1,Center")
	for _, state := range []string{"cold", "warm"} {
		t.Logf("asset cache state: %s", state)
		if state == "warm" {
			if raw := get(assetURL, admin, http.StatusOK); !bytes.Equal(raw, original.Bytes()) {
				t.Fatal("original asset changed")
			}
		}
		for _, query := range invalid {
			get(assetURL+"?processImage=true&"+query, admin, http.StatusBadRequest)
		}
		assertSize(get(assetURL+"?processImage=true&resize=3,5,Box", admin, http.StatusOK), 3, 5)
		assertSize(get(assetURL+"?processImage=true&resize=4096,1,Box", admin, http.StatusOK), 4096, 1)
		for _, filter := range []string{"boxblur", "gaussianblur", "edgedetection", "dilate", "erode", "median", "gaussianBlur"} {
			for _, value := range []string{"0", "1", "32"} {
				assertSize(get(assetURL+"?processImage=true&"+filter+"="+value, admin, http.StatusOK), 1, 2)
			}
		}
	}
	// Reject an oversized source before decoding its mismatched pixel payload,
	// even when the requested output is small. Raw downloads still work.
	oversized := append([]byte(nil), original.Bytes()...)
	binary.BigEndian.PutUint32(oversized[16:20], 30000)
	binary.BigEndian.PutUint32(oversized[20:24], 30000)
	binary.BigEndian.PutUint32(oversized[29:33], crc32.ChecksumIEEE(oversized[12:29]))
	largeID := createImage("large-header", oversized)
	largeURL := f.URL + "/asset/image_processing_probe/" + largeID + "/photo"
	get(largeURL+"?processImage=true&resize=1,1,Box", admin, http.StatusBadRequest)
	if raw := get(largeURL, admin, http.StatusOK); !bytes.Equal(raw, oversized) {
		t.Fatal("oversized original download changed")
	}
	get(largeURL+"?processImage=true&resize=1,1,Box", admin, http.StatusBadRequest)

	for _, tc := range []struct {
		name          string
		width, height uint32
		query         string
	}{
		{"work", 1024, 1024, "boxblur=4&cropToSize=1,1,Center"},
		{"combined-work", 1024, 1024, "boxblur=2&gaussianblur=2&edgedetection=2"},
		{"median-work", 1024, 1024, "median=2"},
		{"padding", 4096, 1, "dilate=1"},
	} {
		header := append([]byte(nil), original.Bytes()...)
		binary.BigEndian.PutUint32(header[16:20], tc.width)
		binary.BigEndian.PutUint32(header[20:24], tc.height)
		binary.BigEndian.PutUint32(header[29:33], crc32.ChecksumIEEE(header[12:29]))
		probeID := createImage(tc.name, header)
		probeURL := f.URL + "/asset/image_processing_probe/" + probeID + "/photo"
		get(probeURL+"?processImage=true&"+tc.query, admin, http.StatusBadRequest)
		get(probeURL, admin, http.StatusOK) // warm the original cache
		get(probeURL+"?processImage=true&"+tc.query, admin, http.StatusBadRequest)
	}

	// Row access is enforced before processing and after permission revocation.
	get(assetURL+"?processImage=true&resize=30000,30000,Box", reader, http.StatusForbidden)
	get(assetURL+"?processImage=true&boxblur=30000", reader, http.StatusForbidden)
	get(assetURL+"?processImage=true&resize=3,5,Box", "", http.StatusForbidden)
	setPermission := func(grant auth.AuthPermission) {
		t.Helper()
		accessGroupsE2EAssertStatus(t, f.Client, http.MethodPatch, f.URL+"/api/image_processing_probe/"+id, admin,
			accessGroupsE2ERecordPayload("image_processing_probe", id, map[string]interface{}{"permission": int64(grant)}), http.StatusOK)
	}
	setPermission(auth.UserCRUD | auth.GuestPeek | auth.GuestRead)
	for _, token := range []string{reader, ""} {
		get(assetURL+"?processImage=true&resize=30000,30000,Box", token, http.StatusBadRequest)
		get(assetURL+"?processImage=true&gaussianBlur=1000000000", token, http.StatusBadRequest)
		assertSize(get(assetURL+"?processImage=true&median=1", token, http.StatusOK), 1, 2)
		assertSize(get(assetURL+"?processImage=true&resize=3,5,Box", token, http.StatusOK), 3, 5)
	}
	setPermission(auth.UserCRUD | auth.GuestPeek)
	for _, token := range []string{reader, ""} {
		get(assetURL+"?processImage=true&resize=3,5,Box", token, http.StatusForbidden)
		get(assetURL+"?processImage=true&dilate=1", token, http.StatusForbidden)
	}
	// A successful transform after all rejections also checks process health.
	assertSize(get(assetURL+"?processImage=true&resize=3,5,Box", admin, http.StatusOK), 3, 5)
}
