package resource

import (
  "google.golang.org/api/sheets/v4"
  log "github.com/sirupsen/logrus"
  "fmt"
  "context"
  "net/http"
  "golang.org/x/oauth2"
)

type ExternalExchange interface {
  UpdateDestination(destinationName string, data []map[string]interface{}) error
  ReadDestination(destinationName string) ([]map[string]interface{}, error)
}

type GsheetExternalExchange struct {
  token      *oauth2.Token
  columnInfo ColumnMapping
  config     *oauth2.Config
}

func getClient(ctx context.Context, config *oauth2.Config, token *oauth2.Token) *http.Client {
  return config.Client(ctx, token)
}

func (g *GsheetExternalExchange) UpdateDestination(destinationName string, data []map[string]interface{}) error {

  ctx := context.Background()
  client := g.config.Client(ctx, g.token)

  srv, err := sheets.New(client)
  if err != nil {
    log.Fatalf("Unable to retrieve Sheets Client %v", err)
  }

  // Prints the names and majors of students in a sample spreadsheet:
  // https://docs.google.com/spreadsheets/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit
  spreadsheetId := "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms"
  readRange := "Class Data!A2:E"
  resp, err := srv.Spreadsheets.Values.Get(spreadsheetId, readRange).Do()
  if err != nil {
    log.Fatalf("Unable to retrieve data from sheet. %v", err)
  }

  if len(resp.Values) > 0 {
    fmt.Println("Name, Major:")
    for _, row := range resp.Values {
      // Print columns A and E, which correspond to indices 0 and 4.
      fmt.Printf("%s, %s\n", row[0], row[4])
    }
  } else {
    fmt.Print("No data found.")
  }

  return nil
}
func (g *GsheetExternalExchange) ReadDestination(destinationName string) ([]map[string]interface{}, error) {

  ctx := context.Background()
  client := g.config.Client(ctx, g.token)

  srv, err := sheets.New(client)
  if err != nil {
    log.Fatalf("Unable to retrieve Sheets Client %v", err)
  }

  // Prints the names and majors of students in a sample spreadsheet:
  // https://docs.google.com/spreadsheets/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit
  spreadsheetId := "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms"
  readRange := "Class Data!A2:E"
  resp, err := srv.Spreadsheets.Values.Get(spreadsheetId, readRange).Do()
  if err != nil {
    log.Fatalf("Unable to retrieve data from sheet. %v", err)
  }

  if len(resp.Values) > 0 {
    fmt.Println("Name, Major:")
    for _, row := range resp.Values {
      // Print columns A and E, which correspond to indices 0 and 4.
      fmt.Printf("%s, %s\n", row[0], row[4])
    }
  } else {
    fmt.Print("No data found.")
  }

  return nil, nil
}

func NewGsheetExternalExchange(columnInfo ColumnMapping, token *oauth2.Token) ExternalExchange {

  return &GsheetExternalExchange{
    token:      token,
    columnInfo: columnInfo,
  }
}
