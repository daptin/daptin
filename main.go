package main

import (
  "github.com/artpar/gocms/server"
  "os"
)

func main() {
  server.Main(os.Args[1])
}
