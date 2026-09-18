package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const queryFilterValidationRealE2ESchema = `
Tables:
  - TableName: filter_probe
    IsTopLevel: true
    Permission: 2097151
    DefaultPermission: 2097151
    Columns:
      - Name: name
        DataType: varchar(100)
        ColumnType: label
        IsNullable: false
`

func TestQueryFilterValidationRealE2E(t *testing.T) {
	if os.Getenv("DAPTIN_REAL_E2E") != "1" {
		t.Skip("set DAPTIN_REAL_E2E=1 to run the query-filter validation e2e")
	}

	usedPorts := make(map[int]bool, 2)
	port := freeTransportE2EPort(t, usedPorts)
	httpsPort := freeTransportE2EPort(t, usedPorts)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	process := startTransportE2EDaptin(t, port, httpsPort, baseURL, transportE2EDaptinOptions{
		databaseType: "sqlite3", connectionString: filepath.Join(t.TempDir(), "query-filter-validation.db"), schema: queryFilterValidationRealE2ESchema,
	})
	defer process.stopProcess()
	waitForQueryFilterE2EResource(t, baseURL, process)

	client := &http.Client{Timeout: 20 * time.Second}
	adminToken := accessGroupsE2ESignupSigninAdmin(t, client, baseURL)
	resourceURL := baseURL + "/api/filter_probe"

	validQuery := `[{"column":"name","operator":"is","value":"value"}]`
	accessGroupsE2ERequestJSON(t, client, http.MethodGet, queryFilterE2EURL(resourceURL, validQuery), adminToken, nil, http.StatusOK)

	invalidQueries := []struct {
		name  string
		query string
	}{
		{name: "ordinary", query: `[{"column":"does_not_exist","operator":"is","value":"value"}]`},
		{name: "logical group", query: `[{"column":"name","operator":"is","value":"value","logical_group":"group1"},{"column":"does_not_exist","operator":"is","value":"value","logical_group":"group1"}]`},
		{name: "fuzzy", query: `[{"column":"does_not_exist","operator":"fuzzy","value":"value"}]`},
		{name: "mixed fuzzy columns", query: `[{"column":"name,does_not_exist","operator":"fuzzy","value":"value"}]`},
	}

	for _, test := range invalidQueries {
		t.Run(test.name, func(t *testing.T) {
			response := accessGroupsE2ERequestJSON(t, client, http.MethodGet,
				queryFilterE2EURL(resourceURL, test.query), adminToken, nil, http.StatusBadRequest)
			assertConstraintE2EError(t, response, invalidQueryFilterE2EMessage, "does_not_exist")
		})
	}
}

const invalidQueryFilterE2EMessage = "invalid query filter column"

func queryFilterE2EURL(resourceURL, query string) string {
	return resourceURL + "?query=" + url.QueryEscape(query)
}

func waitForQueryFilterE2EResource(t *testing.T, baseURL string, process *transportE2EDaptinProcess) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		response, err := http.Get(baseURL + "/api/filter_probe")
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("filter_probe resource was not registered\n%s", process.logs.String())
}
