package websockets

import (
	"github.com/buraksezer/olric"
	"github.com/daptin/daptin/server/resource"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/websocket"
)

type WebSocketPayload struct {
	Method  string  `json:"method"`
	Payload Message `json:"attributes"`
}

type Message map[string]interface{}

// Chat server.
type Server struct {
	pattern   string
	clients   map[int]*Client
	addCh     chan *Client
	delCh     chan *Client
	doneCh    chan bool
	errCh     chan error
	dtopicMap *map[string]*olric.PubSub
	olricDb   *olric.EmbeddedClient
	cruds     map[string]*resource.DbResource
}

// Create new chat server.
func NewServer(pattern string, dtopicMap *map[string]*olric.PubSub, cruds map[string]*resource.DbResource) *Server {
	clients := make(map[int]*Client)
	addCh := make(chan *Client)
	delCh := make(chan *Client)
	doneCh := make(chan bool)
	errCh := make(chan error)

	return &Server{
		pattern:   pattern,
		clients:   clients,
		addCh:     addCh,
		delCh:     delCh,
		doneCh:    doneCh,
		errCh:     errCh,
		dtopicMap: dtopicMap,
		olricDb:   cruds["world"].OlricDb,
		cruds:     cruds,
	}
}

func (s *Server) Add(c *Client) {
	//sessionUser := auth.SessionUser{}
	//token, _, ok := c.ws.Request().BasicAuth()
	//token  := c.ws.Request().FormValue("token")
	//if ok {
	//	log.Printf("New web socket connection token: %v", token)
	//}
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

func (s *Server) sendAll(msg resource.EventMessage) {
	for _, c := range s.clients {
		c.Write(msg)
	}
}

type WebSocketConnectionHandler interface {
	MessageFromClient(message WebSocketPayload, client *Client)
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
				s.errCh <- err
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
	wsHandler := websocket.Handler(onConnected)
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
			//s.sendPastMessages(c)

			// del a client
		case c := <-s.delCh:
			log.Infof("[126] delete client")
			delete(s.clients, c.id)

			//	// broadcast message for all clients
			//case msg := <-s.sendAllCh:
			//	log.Println("Send all:", msg)
			//	s.messages = append(s.messages, msg)
			//	s.sendAll(msg)

		case err := <-s.errCh:
			log.Infof("[136] error: %s", err.Error())

		case <-s.doneCh:
			return
		}
	}
}
