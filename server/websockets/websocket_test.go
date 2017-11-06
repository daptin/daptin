package websockets

import (
	"log"
	"net/http"
	"testing"
)

func TestWebsocket(t *testing.T) {
	server := NewServer("/entry")
	go server.Listen()
	log.Fatal(http.ListenAndServe(":8080", nil))

}
