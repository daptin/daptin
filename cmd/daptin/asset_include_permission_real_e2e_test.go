package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/daptin/daptin/server/auth"
)

func TestAssetIncludePermissionRealE2E(t *testing.T) {
	requireRealE2E(t)
	options := transportE2EDaptinOptions{connectionString: filepath.Join(t.TempDir(), "include-permission.db"), schema: assetUploadLocalE2ESchema}
	f := startDaptinE2E(t, options)
	admin := f.SignupAdmin(t)
	accessGroupsE2ECreateRecord(t, f.Client, f.URL, admin, "cloud_store", map[string]interface{}{
		"name": "asset-upload-local-e2e-store", "store_type": "local", "store_provider": "local",
		"root_path": t.TempDir(), "store_parameters": "{}",
	})
	f.Process.stopProcess()
	f = startDaptinE2E(t, options)
	reader := accessGroupsE2ESignupSigninUser(t, f.Client, f.URL, admin, "asset-include-reader")
	const secret = "%PDF-private-include-advisory-secret"
	const public = "%PDF-public-include-control"
	privateID := accessGroupsE2ECreateRecord(t, f.Client, f.URL, admin, "asset_upload_probe", map[string]interface{}{
		"title": "a-private",
	})
	publicID := accessGroupsE2ECreateRecord(t, f.Client, f.URL, admin, "asset_upload_probe", map[string]interface{}{
		"title": "b-public",
	})
	for _, item := range []struct {
		id    string
		grant auth.AuthPermission
	}{
		{privateID, auth.UserCRUD | auth.GuestPeek},
		{publicID, auth.UserCRUD | auth.GuestRead | auth.GuestPeek},
	} {
		accessGroupsE2ERequestJSON(t, f.Client, http.MethodPatch, f.URL+"/api/asset_upload_probe/"+item.id, admin,
			accessGroupsE2ERecordPayload("asset_upload_probe", item.id, map[string]interface{}{"permission": int64(item.grant)}), http.StatusOK)
	}
	// Upload through the asset endpoint so the local storage cache is populated.
	for _, item := range []struct{ id, name, contents string }{
		{privateID, "private.pdf", secret}, {publicID, "public.pdf", public},
	} {
		requestAssetE2E(t, f.Client, http.MethodPost, f.URL+"/asset/asset_upload_probe/"+item.id+"/document/upload?operation=stream&filename="+item.name,
			admin, strings.NewReader(item.contents), "application/pdf", http.StatusOK)
	}
	// Confirm bytes are available through an authorized include request.
	authorized := requestAssetE2E(t, f.Client, http.MethodGet, f.URL+"/api/asset_upload_probe/"+privateID+"?included_relations=document", admin, nil, "", http.StatusOK)
	if !strings.Contains(string(authorized), base64.StdEncoding.EncodeToString([]byte(secret))) {
		t.Fatalf("authorized include did not resolve file: %s\n%s", authorized, f.Process.logs.String())
	}
	for _, token := range []string{reader, ""} {
		for _, query := range []string{"include=document", "include=*", "included_relations=document", "included_relations=*"} {
			for _, order := range []string{"title", "-title"} {
				target := fmt.Sprintf("%s/api/asset_upload_probe?%s&sort=%s", f.URL, query, order)
				body := requestAssetE2E(t, f.Client, http.MethodGet, target, token, nil, "", http.StatusOK)
				var response interface{}
				if err := json.Unmarshal(body, &response); err != nil {
					t.Fatal(err)
				}
				rows := accessGroupsE2EDataArray(t, response)
				if len(rows) != 1 || rows[0].(map[string]interface{})["id"] != publicID {
					t.Fatalf("wrong visible rows (%s): %s", target, body)
				}
				if strings.Contains(string(body), "private.pdf") || strings.Contains(string(body), base64.StdEncoding.EncodeToString([]byte(secret))) {
					t.Fatalf("denied file leaked: %s", body)
				}
				if strings.HasPrefix(query, "included_relations=") && !strings.Contains(string(body), base64.StdEncoding.EncodeToString([]byte(public))) {
					t.Fatalf("public file control missing (%s): %s", target, body)
				}
				t.Logf("token present=%t, %s, sort=%s: no private file", token != "", query, order)
			}
		}
		requestAssetE2E(t, f.Client, http.MethodGet, f.URL+"/api/asset_upload_probe/"+privateID+"?included_relations=document", token, nil, "", http.StatusForbidden)
	}
	// Revoking row Read must remove both the row and its already-cached bytes.
	accessGroupsE2ERequestJSON(t, f.Client, http.MethodPatch, f.URL+"/api/asset_upload_probe/"+publicID, admin,
		accessGroupsE2ERecordPayload("asset_upload_probe", publicID, map[string]interface{}{"permission": int64(auth.UserCRUD | auth.GuestPeek)}), http.StatusOK)
	for _, token := range []string{reader, ""} {
		for _, query := range []string{"included_relations=document", "included_relations=*"} {
			body := requestAssetE2E(t, f.Client, http.MethodGet, f.URL+"/api/asset_upload_probe?"+query, token, nil, "", http.StatusOK)
			var response interface{}
			if err := json.Unmarshal(body, &response); err != nil {
				t.Fatal(err)
			}
			if len(accessGroupsE2EDataArray(t, response)) != 0 || strings.Contains(string(body), ".pdf") ||
				strings.Contains(string(body), base64.StdEncoding.EncodeToString([]byte(secret))) ||
				strings.Contains(string(body), base64.StdEncoding.EncodeToString([]byte(public))) {
				t.Fatalf("revoked attachment access survived: %s", body)
			}
		}
	}

}
