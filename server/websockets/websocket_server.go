package websockets

import (
	"fmt"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/websocket"
	"net/http"
)

type WebSocketPayload struct {
	Method   string  `json:"method"`
	TypeName string  `json:"type"`
	Payload  Message `json:"payload"`
}

type Message struct {
	Id         string                 `json:"id"`
	Type       string                 `json:"type"`
	Attributes map[string]interface{} `json:"attributes"`
}

func (self *Message) String() string {
	return fmt.Sprintf("[%v] %v", self.Type, self.Attributes)
}

// Chat server.
type Server struct {
	pattern        string
	clients        map[int]*Client
	addCh          chan *Client
	delCh          chan *Client
	doneCh         chan bool
	errCh          chan error
	messageHandler WebSocketConnectionHandler
}

// Create new chat server.
func NewServer(pattern string, messageHandler WebSocketConnectionHandler) *Server {
	clients := make(map[int]*Client)
	addCh := make(chan *Client)
	delCh := make(chan *Client)
	doneCh := make(chan bool)
	errCh := make(chan error)

	return &Server{
		pattern:        pattern,
		clients:        clients,
		addCh:          addCh,
		delCh:          delCh,
		doneCh:         doneCh,
		errCh:          errCh,
		messageHandler: messageHandler,
	}
}

func (s *Server) Add(c *Client) {
	//sessionUser := auth.SessionUser{}
	//token, _, ok := c.ws.Request().BasicAuth()
	//token  := c.ws.Request().FormValue("token")
	//if ok {
	//	log.Infof("New web socket connection token: %v", token)
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

func (s *Server) sendAll(msg *WebSocketPayload) {
	for _, c := range s.clients {
		c.Write(msg)
	}
}

type WebSocketConnectionHandler interface {
	MessageFromClient(message WebSocketPayload, request *http.Request)
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

		client := NewClient(ws, s)
		s.Add(client)
		client.Listen()
	}
	wsHandler := websocket.Handler(onConnected)
	router.GET(s.pattern, func(ginContext *gin.Context) {
		wsHandler.ServeHTTP(ginContext.Writer, ginContext.Request)
	})

	log.Println("Created handler")

	for {
		select {

		// Add new a client
		case c := <-s.addCh:
			log.Println("Added new client")
			s.clients[c.id] = c
			log.Println("Now", len(s.clients), "clients connected.")
			//s.sendPastMessages(c)

			// del a client
		case c := <-s.delCh:
			log.Println("Delete client")
			delete(s.clients, c.id)

			//	// broadcast message for all clients
			//case msg := <-s.sendAllCh:
			//	log.Println("Send all:", msg)
			//	s.messages = append(s.messages, msg)
			//	s.sendAll(msg)

		case err := <-s.errCh:
			log.Println("Error:", err.Error())

		case <-s.doneCh:
			return
		}
	}
}
