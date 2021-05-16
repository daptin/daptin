package resource

import (
	"github.com/samedi/caldav-go"
	"net/http"
)

func NewCaldavServer(CaldavListenInterface string)*http.Server{
	servermux := http.NewServeMux()
	servermux.HandleFunc("/caldav", caldav.RequestHandler)

	s := &http.Server{
		Addr: CaldavListenInterface,
		Handler: servermux,
	}

	return s
}
