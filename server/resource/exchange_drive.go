package resource

import (
  "google.golang.org/api/drive/v3"
  log "github.com/sirupsen/logrus"
  "context"
  "golang.org/x/oauth2"
)

type GdriveExternalExchange struct {
  token      *oauth2.Token
  columnInfo ColumnMapping
  config     *oauth2.Config
}

func (g *GdriveExternalExchange) UpdateDestination(destinationName string, data []map[string]interface{}) (error) {

  return nil
}
func (g *GdriveExternalExchange) ReadDestination(destinationName string) ([]map[string]interface{}, error) {

  ctx := context.Background()
  client := g.config.Client(ctx, g.token)

  srv, err := drive.New(client)

  if err != nil {
    log.Fatalf("Unable to retrieve Gdrive Client %v", err)
  }

  filesList, err := srv.Files.List().Do()

  if err != nil {
    return nil, err
  }

  resp := make([]map[string]interface{}, 0)

  for _, file := range filesList.Files {

    row := make(map[string]interface{})

    row["name"] = file.Name

    for k, v := range file.AppProperties {
      row[k] = v
    }

    row["created_at"] = file.CreatedTime
    row["description"] = file.Description
    row["file_extension"] = file.FileExtension
    row["folder_color_rgs"] = file.FolderColorRgb
    row["reference_id"] = file.Id

    resp = append(resp, row)
  }

  return resp, nil
}

func NewGdriveExternalExchange(columnInfo ColumnMapping, token *oauth2.Token) ExternalExchange {

  return &GdriveExternalExchange{
    token:      token,
    columnInfo: columnInfo,
  }
}
