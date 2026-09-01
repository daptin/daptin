package websockets

import (
	"context"
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
	doneCh        chan struct{}
	stoppedCh     chan struct{}
	stopOnce      sync.Once
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
	doneCh := make(chan struct{})
	errCh := make(chan error, 16)

	var olricDb *olric.EmbeddedClient
	if world := cruds["world"]; world != nil {
		olricDb = world.OlricDb
	}
	return &Server{
		pattern:      pattern,
		clients:      clients,
		addCh:        addCh,
		delCh:        delCh,
		doneCh:       doneCh,
		stoppedCh:    make(chan struct{}),
		errCh:        errCh,
		dtopicMap:    dtopicMap,
		olricDb:      olricDb,
		cruds:        cruds,
		sharedPubSub: sharedPubSub,
	}
}

func (s *Server) Add(c *Client) {
	select {
	case s.addCh <- c:
	case <-s.doneCh:
		c.Close()
	}
}

func (s *Server) Del(c *Client) {
	select {
	case s.delCh <- c:
	case <-s.doneCh:
		c.Close()
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.stopOnce.Do(func() { close(s.doneCh) })
	select {
	case <-s.stoppedCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Server) Err(err error) {
	s.errCh <- err
}

type WebSocketConnectionHandler interface {
	MessageFromClient(message WebSocketPayload, client *Client)
	Close()
}

// Register attaches the websocket HTTP route before the router begins serving.
func (s *Server) Register(router *gin.Engine) {
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
}

// Listen serves client connection and broadcast events after route registration.
func (s *Server) Listen() {
	defer close(s.stoppedCh)

	log.Printf("Listening websocket server at ... %v", s.pattern)

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
			for _, client := range s.clients {
				client.Close()
			}
			clear(s.clients)
			return
		}
	}
}
