package main

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/buraksezer/olric"
	"github.com/daptin/daptin/server/cache"
	"golang.org/x/net/html"
)

func TestMarkdownAssetEscapesStoredHTMLRealE2E(t *testing.T) {
	requireRealE2E(t)
	ports := map[int]bool{}
	port, httpsPort := freeTransportE2EPort(t, ports), freeTransportE2EPort(t, ports)
	olricPort := freeTransportE2EPortPair(t, ports)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	process := startTransportE2EDaptin(t, port, httpsPort, baseURL, transportE2EDaptinOptions{
		olricPort: olricPort,
		schema: `Tables:
  - TableName: markdown_asset_probe
    Permission: 15
    DefaultPermission: 16259
    Columns:
      - Name: notes
        ColumnType: markdown
        DataType: text
`,
	})
	defer process.stopProcess()
	client := &http.Client{Timeout: 20 * time.Second}
	admin := accessGroupsE2ESignupSigninAdmin(t, client, baseURL)
	author := accessGroupsE2ESignupSigninUser(t, client, baseURL, admin, "markdown-author")
	reader := accessGroupsE2ESignupSigninUser(t, client, baseURL, admin, "markdown-reader")
	id := accessGroupsE2ECreateRecord(t, client, baseURL, author, "markdown_asset_probe", map[string]interface{}{"notes": "initial"})
	payload := `</pre><script>document.body.dataset.executed='true'</script><img src=x onerror="document.body.dataset.executed='true'"><iframe srcdoc="<script>alert(1)</script>"></iframe>&lt;script&gt;` + "\n日本語 & नमस्ते\n" + strings.Repeat("# markdown text\n", 500)
	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, baseURL+"/api/markdown_asset_probe/"+id, author,
		accessGroupsE2ERecordPayload("markdown_asset_probe", id, map[string]interface{}{"notes": payload}), http.StatusOK)
	// Share the author's row with a distinct reader through the normal resource API.
	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, baseURL+"/api/markdown_asset_probe/"+id, admin,
		accessGroupsE2ERecordPayload("markdown_asset_probe", id, map[string]interface{}{"permission": 16259}), http.StatusOK)
	row := accessGroupsE2ERequestJSON(t, client, http.MethodGet, baseURL+"/api/markdown_asset_probe/"+id, reader, nil, http.StatusOK).(map[string]interface{})
	if row["data"].(map[string]interface{})["attributes"].(map[string]interface{})["notes"] != payload {
		t.Fatal("stored markdown was modified")
	}

	// Seed a pre-fix rendered response in the actual shared cache to exercise upgrades.
	cacheClient, err := olric.NewClusterClient([]string{fmt.Sprintf("127.0.0.1:%d", olricPort)})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = cacheClient.Close(ctx)
	}()
	dmap, err := cacheClient.NewDMap(cache.AssetsCacheNamespace)
	if err != nil {
		t.Fatal(err)
	}
	legacyKey := "markdown_asset_probe:" + id + ":notes::"
	if err := dmap.Put(context.Background(), legacyKey, &cache.CachedFile{
		Data: []byte("<pre>" + payload + "</pre>"), ETag: `"legacy"`,
		MimeType: "text/html; charset=utf-8", Path: "notes.md",
		ExpiresAt: time.Now().Add(time.Hour), AuthzVersion: 1,
	}); err != nil {
		t.Fatal(err)
	}

	get := func(encoding, etag string, want int) ([]byte, string) {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, baseURL+"/asset/markdown_asset_probe/"+id+"/notes.md", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+reader)
		req.Header.Set("Accept-Encoding", encoding)
		req.Header.Set("If-None-Match", etag)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != want {
			t.Fatalf("asset status=%d, want %d", resp.StatusCode, want)
		}
		var body io.Reader = resp.Body
		if want == http.StatusOK {
			if resp.Header.Get("Content-Type") != "text/html; charset=utf-8" || !strings.HasPrefix(resp.Header.Get("Content-Disposition"), "inline;") {
				t.Fatalf("unexpected asset headers: %v", resp.Header)
			}
			if encoding == "gzip" {
				if resp.Header.Get("Content-Encoding") != "gzip" {
					t.Fatal("gzip representation missing")
				}
				gz, err := gzip.NewReader(resp.Body)
				if err != nil {
					t.Fatal(err)
				}
				defer gz.Close()
				body = gz
			}
		}
		data, err := io.ReadAll(body)
		if err != nil {
			t.Fatal(err)
		}
		return data, resp.Header.Get("ETag")
	}
	assertText := func(body []byte) {
		t.Helper()
		doc, err := html.Parse(strings.NewReader(string(body)))
		if err != nil {
			t.Fatal(err)
		}
		preCount := 0
		var walk func(*html.Node)
		walk = func(node *html.Node) {
			if node.Type == html.ElementNode {
				switch node.Data {
				case "html", "head", "body":
				case "pre":
					preCount++
					var text strings.Builder
					for child := node.FirstChild; child != nil; child = child.NextSibling {
						if child.Type != html.TextNode {
							t.Fatalf("markdown produced a non-text node: %s", child.Data)
						}
						text.WriteString(child.Data)
					}
					if text.String() != payload {
						t.Fatal("rendered markdown text differs from stored value")
					}
				default:
					t.Fatalf("stored markup became an HTML element: %s", node.Data)
				}
			}
			for child := node.FirstChild; child != nil; child = child.NextSibling {
				walk(child)
			}
		}
		walk(doc)
		if preCount != 1 {
			t.Fatalf("rendered %d pre elements", preCount)
		}
	}
	body, etag := get("identity", "", http.StatusOK)
	assertText(body)
	warm, warmETag := get("identity", "", http.StatusOK)
	assertText(warm)
	if etag != warmETag || string(body) != string(warm) {
		t.Fatal("cached representation changed")
	}
	compressed, _ := get("gzip", "", http.StatusOK)
	assertText(compressed)
	get("identity", etag, http.StatusNotModified)
	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, baseURL+"/api/markdown_asset_probe/"+id, admin,
		accessGroupsE2ERecordPayload("markdown_asset_probe", id, map[string]interface{}{"permission": 16256}), http.StatusOK)
	get("identity", etag, http.StatusForbidden)
}
