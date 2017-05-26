package server

import (
  "gopkg.in/gin-gonic/gin.v1"
  "github.com/artpar/api2go"
  "github.com/artpar/api2go-adapter/gingonic"
  log "github.com/Sirupsen/logrus"
  _ "github.com/go-sql-driver/mysql"
  //"github.com/itsjamie/gin-cors"
  "io/ioutil"
  "encoding/json"
  "github.com/jmoiron/sqlx"
  "github.com/artpar/gocms/datastore"
  "github.com/pkg/errors"
  "strings"
  "github.com/auth0/go-jwt-middleware"
  "github.com/artpar/gocms/server/resource"
  "github.com/dgrijalva/jwt-go"
  "github.com/gorilla/context"
  "time"
)

type CmsConfig struct {
  Tables    []datastore.TableInfo
  Relations []api2go.TableRelation
}

var ColumnTypes = []string{
  "Id",
  "Alias",
  "Date",
  "Time",
  "Day",
  "Month",
  "Year",
  "Minute",
  "Hour",
  "Email",
  "Name",
  "Value",
  "TrueFalse",
  "DateTime",
  "Location (lat/long)",
  "Color",
  "Measurement",
  "Label",
  "Content",
  "File",
  "Url",
  "Image",
}

var CollectionTypes = []string{
  "Pair",
  "Triplet",
  "Set",
  "OrderedSet",
}

type User struct {
  Name       string
  Email      string
  Password   string
  Id         uint64 `gorm:"PRIMARY KEY"`
  CreatedAt  time.Time `gorm:"not null"`
  UpdatedAt  time.Time
  Permission int
  Status     string
  DeletedAt  *time.Time `sql:"index"`
}

var cruds = make(map[string]*resource.DbResource)

func Main() {

  db, err := sqlx.Open("mysql", "root:parth123@tcp(localhost:3306)/example")
  if err != nil {
    panic(err)
  }

  r := gin.Default()
  r.Use(CorsMiddlewareFunc)

  jwtMiddleware := jwtmiddleware.New(jwtmiddleware.Options{
    ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
      return []byte("nXhlfq1Q6llIOJgUBwGjx2knwRzJQVpSOYbnUmoZNwqBwAtH9IXfKmfbeEYcwFSc"), nil
    },
    Debug: true,
    // When set, the middleware verifies that tokens are signed with the specific signing algorithm
    // If the signing method is not constant the ValidationKeyGetter callback can be used to implement additional checks
    // Important to avoid security issues described here: https://auth0.com/blog/2015/03/31/critical-vulnerabilities-in-json-web-token-libraries/
    SigningMethod: jwt.SigningMethodHS256,
    UserProperty: "user",
  })

  r.Use(func(c *gin.Context) {

    err := jwtMiddleware.CheckJWT(c.Writer, c.Request)

    if err != nil {
      c.AbortWithError(401, err)
    } else {

      user := context.Get(c.Request, "user")

      if (user == nil) {
        c.Next()
      } else {

        userToken := user.(*jwt.Token)
        email := userToken.Claims.(jwt.MapClaims)["email"].(string)

        var referenceId string
        var userId int64
        var userGroups []string
        err := db.QueryRowx("select u.id, u.reference_id from user u where email = ?", email).Scan(&userId, &referenceId)

        if err != nil {
          log.Errorf("Failed to scan user from db: %v", err)

          mapData := make(map[string]interface{})
          mapData["name"] = email
          mapData["email"] = email

          newUser := api2go.NewApi2GoModelWithData("user", nil, 644, nil, mapData)

          req := api2go.Request{

          }

          resp, err := cruds["user"].Create(newUser, req)
          if err != nil {
            log.Errorf("Failed to create new user: %v", err)
          }
          referenceId = resp.Result().(*api2go.Api2GoModel).Data["reference_id"].(string)

          mapData = make(map[string]interface{})
          mapData["name"] = "Home group for  user " + email

          newUserGroup := api2go.NewApi2GoModelWithData("usergroup", nil, 644, nil, mapData)

          resp, err = cruds["usergroup"].Create(newUserGroup, req)
          if err != nil {
            log.Errorf("Failed to create new user group: %v", err)
          }
          userGroupId := resp.Result().(*api2go.Api2GoModel).Data["reference_id"].(string)
          userGroups = []string{userGroupId}

          mapData = make(map[string]interface{})
          mapData["user_id"] = referenceId
          mapData["usergroup_id"] = userGroupId
          newUserUserGroup := api2go.NewApi2GoModelWithData("user_usergroup", nil, 644, nil, mapData)

          uug, err := cruds["user_usergroup"].Create(newUserUserGroup, req)
          log.Infof("Userug: %v", uug)

        } else {
          rows, err := db.Queryx("select ug.reference_id from usergroup ug join user_usergroup uug on uug.usergroup_id = ug.id where uug.user_id = ?", userId)
          if err != nil {

          } else {
            rows.Scan(userGroups)
          }
        }

        context.Set(c.Request, "user_id", referenceId)
        context.Set(c.Request, "usergroup_id", userGroups)

        c.Next()

      }
    }

  })

  api := api2go.NewAPIWithRouting(
    "api",
    api2go.NewStaticResolver("/"),
    gingonic.New(r),
  )

  //r.Use(cors.Default())
  //r.Use()

  configFile := "gocms.json"
  contents, err := ioutil.ReadFile(configFile)
  if err != nil {
    log.Errorf("Failed to read config file: %v", err)
    return
  }

  var initConfig CmsConfig
  json.Unmarshal([]byte(contents), &initConfig)
  //log.Infof("Config: %v", initConfig)


  initConfig.Tables = append(initConfig.Tables, datastore.StandardTables...)
  initConfig.Relations = append(initConfig.Relations, datastore.StandardRelations...)
  CheckRelations(&initConfig, db)
  CheckAllTableStatus(&initConfig, db)
  CreateRelations(&initConfig, db)

  log.Infof("table relations: %v", initConfig.Tables[0])

  CreateUniqueConstraints(&initConfig, db)
  CreateIndexes(&initConfig, db)

  UpdateWorldTable(&initConfig, db)
  //log.Infof("Tables: %v", tables)
  UpdateWorldColumnTable(&initConfig, db)

  //log.Infof("content: %v", initConfig)

  var ms resource.MiddlewareSet

  tpc := &resource.TableAccessPermissionChecker{}

  ms.BeforeFindAll = []resource.DatabaseRequestInterceptor{
    tpc,
  }
  ms.AfterFindAll = []resource.DatabaseRequestInterceptor{
    tpc,
  }

  cruds = AddAllTablesToApi2Go(api, initConfig.Tables, db, &ms)

  r.GET("/ping", func(c *gin.Context) {
    c.String(200, "pong")
  })

  r.GET("/jsmodel/:typename", CreateJsModelHandler(&initConfig))
  r.OPTIONS("/jsmodel/:typename", CreateJsModelHandler(&initConfig))

  r.Run(":6336")

}
func CreateJsModelHandler(initConfig *CmsConfig) func(*gin.Context) {

  return func(c *gin.Context) {
    typeName := strings.Split(c.Param("typename"), ".")[0]
    //resource := resources[typeName]
    var selectedTable *datastore.TableInfo

    for _, t := range initConfig.Tables {
      if t.TableName == typeName {
        selectedTable = &t
        break
      }
    }

    //log.Infof("data: %v", selectedTable)

    if selectedTable == nil {
      c.AbortWithError(404, errors.New("Invalid type"))
      return
    }
    cols := selectedTable.Columns

    res := map[string]interface{}{}

    for _, col := range cols {
      if col.ColumnType == "deleted_at" {
        continue
      }
      if col.ColumnType == "id" {
        continue
      }

      res[col.ColumnName] = col.ColumnType
    }

    for _, rel := range selectedTable.Relations {

      if rel.Subject == selectedTable.TableName {
        res[rel.Object] = NewJsonApiRelation(rel.Object, rel.Relation)
      } else {
        res[rel.Subject] = NewJsonApiRelation(rel.Subject, rel.Relation)
      }

    }

    c.JSON(200, res)
    if true {
      return
    }

    //j, _ := json.Marshal(res)

    //c.String(200, "jsonApi.define('%v', %v)", typeName, string(j))

  }
}

func NewJsonApiRelation(name string, relationType string) JsonApiRelation {

  if relationType == "belongs_to" {
    return JsonApiRelation{
      JsonApi: "toOne",
      Type: name,
    }
  } else if relationType == "has_many" {
    return JsonApiRelation{
      Type: name,
      JsonApi: "hasMany",
    }
  } else {
    return JsonApiRelation{
      Type: name,
      JsonApi: "hasMany",
    }
  }

}

type JsonApiRelation struct {
  JsonApi string `json:"jsonApi"`
  Type    string `json:"type"`
}

func AuthenticationFilter(c *gin.Context) {

}

func CorsMiddlewareFunc(c *gin.Context) {
  //log.Infof("middleware ")

  c.Header("Access-Control-Allow-Origin", "*")
  c.Header("Access-Control-Allow-Methods", "POST,GET,DELETE,PUT,OPTIONS,PATCH")
  c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization")
  c.Next()
}

func AddAllTablesToApi2Go(api *api2go.API, tables []datastore.TableInfo, db *sqlx.DB, ms *resource.MiddlewareSet) map[string]*resource.DbResource {
  m := make(map[string]*resource.DbResource)
  for _, table := range tables {
    log.Infof("Table [%v] Relations: %v", table.TableName, table.Relations)
    model := api2go.NewApi2GoModel(table.TableName, table.Columns, table.DefaultPermission, table.Relations)

    res := resource.NewDbResource(model, db, ms)

    m[table.TableName] = res
    api.AddResource(model, res)
  }
  return m
}

