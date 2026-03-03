//go:build test
// +build test

package server

import (
	"context"
	"github.com/artpar/api2go/v2"
	"github.com/buraksezer/olric"
	"github.com/buraksezer/olric/config"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/table_info"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"net/http"
	"net/url"
	"testing"
)

func TestStream(t *testing.T) {

	model := api2go.NewApi2GoModel("test", []api2go.ColumnInfo{}, int64(auth.DEFAULT_PERMISSION), []api2go.TableRelation{})

	cruds := make(map[string]*resource.DbResource)

	olricDb1, _ := olric.New(config.New("local"))
	olricDb := olricDb1.NewEmbeddedClient()

	db, err := sqlx.Open("sqlite3", "daptin_test.db")
	if err != nil {
		panic(err)
	}

	wrapper := NewInMemoryTestDatabase(db)

	dBResource, _ := resource.NewDbResource(model, wrapper, &resource.MiddlewareSet{
		BeforeCreate:  []resource.DatabaseRequestInterceptor{},
		BeforeFindAll: []resource.DatabaseRequestInterceptor{},
		BeforeFindOne: []resource.DatabaseRequestInterceptor{},
		BeforeUpdate:  []resource.DatabaseRequestInterceptor{},
		BeforeDelete:  []resource.DatabaseRequestInterceptor{},
		AfterCreate:   []resource.DatabaseRequestInterceptor{},
		AfterFindAll:  []resource.DatabaseRequestInterceptor{},
		AfterFindOne:  []resource.DatabaseRequestInterceptor{},
		AfterUpdate:   []resource.DatabaseRequestInterceptor{},
		AfterDelete:   []resource.DatabaseRequestInterceptor{},
	}, cruds, &resource.ConfigStore{}, olricDb, table_info.TableInfo{
		TableName: "test",
		Columns: []api2go.ColumnInfo{
			{
				ColumnName: "col1",
				Name:       "col1",
			},
		},
	})

	streamContract := resource.StreamContract{
		StreamName:     "test_stream",
		RootEntityName: "test",
		Columns: []api2go.ColumnInfo{
			{
				ColumnName: "col1",
				Name:       "col1",
			},
		},
		QueryParams: map[string][]string{
			"query": {
				"[{\"column\":\"col1\",\"operator\":\"like\",\"value\":\"$query\"}]",
			},
			"page[number]": {
				"$page[number]",
			},
			"page[size]": {
				"$page[size]",
			},
		},
	}
	cruds["test"] = dBResource
	newStream := resource.NewStreamProcessor(streamContract, cruds)

	ur, _ := url.Parse("/world/:referenceId")
	httpPlainRequest := &http.Request{
		Method: "GET",
		URL:    ur,
	}
	httpPlainRequest = httpPlainRequest.WithContext(context.Background())
	findRequest := api2go.Request{
		QueryParams: map[string][]string{
			"query":        {"query1"},
			"page[number]": {"5"},
			"page[size]":   {"20"},
		},
		PlainRequest: httpPlainRequest,
	}
	_, _, err = newStream.PaginatedFindAll(findRequest)
	if err != nil {
		log.Printf("%v", err)
	}
}
