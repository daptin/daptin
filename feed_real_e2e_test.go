package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const feedRealE2ESchema = `
Tables:
  - TableName: article
    Columns:
      - Name: title
        DataType: varchar(500)
        ColumnType: label
      - Name: link
        DataType: varchar(1000)
        ColumnType: label
      - Name: description
        DataType: text
        ColumnType: label
      - Name: author_name
        DataType: varchar(500)
        ColumnType: label
      - Name: author_email
        DataType: varchar(500)
        ColumnType: label
`

func TestFeedFormatsRealE2E(t *testing.T) {
	if os.Getenv("DAPTIN_REAL_E2E") != "1" {
		t.Skip("set DAPTIN_REAL_E2E=1 to run the feed e2e")
	}
	usedPorts := map[int]bool{}
	databasePath := filepath.Join(t.TempDir(), "feeds.db")
	databaseType, connectionString := "sqlite3", databasePath
	if postgresDSN := os.Getenv("DAPTIN_TEST_POSTGRES_DSN"); postgresDSN != "" {
		databaseType, connectionString = "postgres", postgresDSN
	}
	client := &http.Client{Timeout: 20 * time.Second}
	start := func() (string, *transportE2EDaptinProcess) {
		port := freeTransportE2EPort(t, usedPorts)
		httpsPort := freeTransportE2EPort(t, usedPorts)
		olricPort := freeTransportE2EPortPair(t, usedPorts)
		baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
		process := startTransportE2EDaptin(t, port, httpsPort, baseURL, transportE2EDaptinOptions{
			databaseType: databaseType, connectionString: connectionString, olricPort: olricPort, schema: feedRealE2ESchema,
		})
		return baseURL, process
	}
	fetch := func(baseURL, token, path string) (int, string, string) {
		t.Helper()
		request, err := http.NewRequest(http.MethodGet, baseURL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer "+token)
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return response.StatusCode, response.Header.Get("Content-Type"), string(body)
	}
	firstURL, first := start()
	token := accessGroupsE2ESignupSigninAdmin(t, client, firstURL)
	accessGroupsE2ECreateRecord(t, client, firstURL, token, "article", map[string]interface{}{
		"title": "Daptin feed example", "link": "https://example.test/articles/feed-example",
		"description": "A source row rendered by a Daptin stream.",
		"author_name": "Example Publisher", "author_email": "publisher@example.test",
	})
	streamID := accessGroupsE2ECreateRecord(t, client, firstURL, token, "stream", map[string]interface{}{
		"stream_name": "article_feed_stream", "enable": true,
		"stream_contract": `{"StreamName":"article_feed_stream","RootEntityName":"article","Columns":[{"Name":"title","ColumnName":"title"},{"Name":"link","ColumnName":"link"},{"Name":"description","ColumnName":"description"},{"Name":"author_name","ColumnName":"author_name"},{"Name":"author_email","ColumnName":"author_email"},{"Name":"created_at","ColumnName":"created_at"}],"QueryParams":{}}`,
	})
	createFeed := func(name string, attributes map[string]interface{}) {
		t.Helper()
		values := map[string]interface{}{
			"feed_name": name, "title": "Example Articles", "description": "Recent example articles",
			"link": "https://example.test/articles", "author_name": "Example Publisher",
			"author_email": "publisher@example.test", "enable": true,
			"enable_rss": true, "enable_atom": true, "enable_json": true, "page_size": 50,
		}
		for key, value := range attributes {
			values[key] = value
		}
		accessGroupsE2ERequestJSON(t, client, http.MethodPost, firstURL+"/api/feed", token, map[string]interface{}{
			"data": map[string]interface{}{
				"type": "feed", "attributes": values,
				"relationships": map[string]interface{}{
					"stream_id": map[string]interface{}{"data": map[string]interface{}{"type": "stream", "id": streamID}},
				},
			},
		}, http.StatusCreated)
	}
	createFeed("articles", nil)
	createFeed("no-atom", map[string]interface{}{"enable_atom": false})
	createFeed("disabled-feed", map[string]interface{}{"enable": false})
	createFeed("bad-size", map[string]interface{}{"page_size": 0})
	for _, extension := range []string{"rss", "atom", "json"} {
		status, _, _ := fetch(firstURL, token, "/feed/articles."+extension)
		if status != http.StatusNotFound {
			t.Fatalf("before restart %s status=%d, want 404", extension, status)
		}
	}
	first.stopProcess()
	secondURL, second := start()
	defer second.stopProcess()
	for _, format := range []struct{ extension, contentType, root string }{
		{"rss", "application/xml", "rss"},
		{"atom", "application/xml", "feed"},
		{"json", "application/json", ""},
	} {
		t.Run(format.extension, func(t *testing.T) {
			status, contentType, body := fetch(secondURL, token, "/feed/articles."+format.extension)
			if status != http.StatusOK || !strings.HasPrefix(contentType, format.contentType) || !strings.Contains(body, "Daptin feed example") {
				t.Fatalf("feed response: status=%d content-type=%q body=%q logs=%s", status, contentType, body, second.logs.String())
			}
			if format.root != "" {
				var document struct{ XMLName xml.Name }
				if err := xml.Unmarshal([]byte(body), &document); err != nil || document.XMLName.Local != format.root {
					t.Fatalf("invalid %s: root=%q error=%v", format.extension, document.XMLName.Local, err)
				}
			} else {
				var document map[string]interface{}
				if err := json.Unmarshal([]byte(body), &document); err != nil {
					t.Fatalf("invalid JSON feed: body=%q error=%v", body, err)
				}
				items, ok := document["items"].([]interface{})
				if !ok || len(items) != 1 {
					t.Fatalf("invalid JSON feed items: body=%q", body)
				}
			}
		})
	}
	for _, path := range []string{"/feed/no-atom.atom", "/feed/disabled-feed.rss", "/feed/articles.txt"} {
		status, _, _ := fetch(secondURL, token, path)
		if status != http.StatusNotFound {
			t.Errorf("%s status=%d, want 404", path, status)
		}
	}
	status, contentType, body := fetch(secondURL, token, "/feed/bad-size.rss")
	if status != http.StatusInternalServerError || !strings.HasPrefix(contentType, "application/json") {
		t.Fatalf("bad configuration response: status=%d content-type=%q body=%q", status, contentType, body)
	}
	var failure map[string]interface{}
	if err := json.Unmarshal([]byte(body), &failure); err != nil || failure["error"] != "Invalid feed page_size" {
		t.Fatalf("bad configuration error: body=%q error=%v", body, err)
	}
	if strings.Contains(second.logs.String(), "interface conversion:") {
		t.Fatalf("feed requests panicked: %s", second.logs.String())
	}
}
