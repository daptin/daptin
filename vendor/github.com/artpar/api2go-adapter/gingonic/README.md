# Gin Framework Adapter

This adapter must be used in combination with [api2go](https://github.com/manyminds/api2go) in order to be useful.
It allows you to use api2go within your normal [gin](https://github.com/gin-gonic/gin) application.

## Example

```go
package main

import (
  "github.com/gin-gonic/gin"
  "github.com/manyminds/api2go"
  "github.com/manyminds/api2go-adapter/gingonic"
)

func main() {
  r := gin.Default()
  api := api2go.NewAPIWithRouting(
    "api",
    api2go.NewStaticResolver("/"),
    api2go.DefaultContentMarshalers,
    gingonic.New(r),
  )

  // Add your API resources here...
  // see https://github.com/manyminds/api2go for more information

  r.GET("/ping", func(c *gin.Context) {
    c.String(200, "pong")
  })
  r.Run(":8080") // listen and serve on 0.0.0.0:8080
}
```