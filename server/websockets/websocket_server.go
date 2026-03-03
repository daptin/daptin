package websockets

import (
	"net/http"
	"sync"

	"github.com/buraksezer/olric"
	"github.com/daptin/daptin/server/resource"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/websocket"
)

type WebSocketPayload struct {
	Id      string  `json:"id,omitempty"`
	Method  string  `json:"method"`
	Payload Message `json:"attributes"`
}

type Message map[string]interface{}

// Chat server.
type Server struct {
	pattern       string
	clients       map[int]*Client
	addCh         chan *Client
	delCh         chan *Client
	doneCh        chan bool
	errCh         chan error
	dtopicMap     *map[string]*olric.PubSub
	dtopicMapLock sync.RWMutex
	olricDb       *olric.EmbeddedClient
	cruds         map[string]*resource.DbResource
	sharedPubSub  *olric.PubSub
}

// Create new chat server.
func NewServer(pattern string, dtopicMap *map[string]*olric.PubSub, cruds map[string]*resource.DbResource, sharedPubSub *olric.PubSub) *Server {
	clients := make(map[int]*Client)
	addCh := make(chan *Client, 256)
	delCh := make(chan *Client, 16)
	doneCh := make(chan bool)
	errCh := make(chan error, 16)

	return &Server{
		pattern:   pattern,
		clients:   clients,
		addCh:     addCh,
		delCh:     delCh,
		doneCh:    doneCh,
		errCh:     errCh,
		dtopicMap:    dtopicMap,
		olricDb:      cruds["world"].OlricDb,
		cruds:        cruds,
		sharedPubSub: sharedPubSub,
	}
}

func (s *Server) Add(c *Client) {
	s.addCh <- c
}

func (s *Server) Del(c *Client) {
	s.delCh <- c
}

func (s *Server) Done() {
	s.doneCh <- true
}

func (s *Server) Err(err error) {
	s.errCh <- err
}

type WebSocketConnectionHandler interface {
	MessageFromClient(message WebSocketPayload, client *Client)
	Close()
}

// Listen and serve.
// It serves client connection and broadcast request.
func (s *Server) Listen(router *gin.Engine) {

	log.Printf("Listening websocket server at ... %v", s.pattern)

	// websocket handler
	onConnected := func(ws *websocket.Conn) {
		defer func() {
			err := ws.Close()
			if err != nil {
				select {
				case s.errCh <- err:
				default:
				}
			}
		}()

		client, err := NewClient(ws, s)
		if err != nil {
			_, _ = ws.Write([]byte(err.Error()))
			_ = ws.WriteClose(400)
			return
		}
		s.Add(client)
		client.Listen()
	}
	wsHandler := websocket.Server{
		Handler:   onConnected,
		Handshake: func(config *websocket.Config, req *http.Request) error { return nil },
	}
	router.GET(s.pattern, func(ginContext *gin.Context) {
		wsHandler.ServeHTTP(ginContext.Writer, ginContext.Request)
	})

	log.Debugf("Created websocket handler")

	for {
		select {

		// Add new a client
		case c := <-s.addCh:
			s.clients[c.id] = c
			log.Infof("Added new client, %d clients connected", len(s.clients))

			// del a client
		case c := <-s.delCh:
			log.Infof("[126] delete client")
			c.Close()
			delete(s.clients, c.id)

		case err := <-s.errCh:
			log.Infof("[136] error: %s", err.Error())

		case <-s.doneCh:
			return
		}
	}
}
