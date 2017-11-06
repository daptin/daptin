package websockets

import (
	"golang.org/x/net/websocket"
	log "github.com/sirupsen/logrus"
	"github.com/daptin/daptin/server/auth"
	"gopkg.in/gin-gonic/gin.v1"
	"fmt"
)

type Message struct {
	Method     string                 `json:"method"`
	Id         string                 `json:"id"`
	Type       string                 `json:"type"`
	Attributes map[string]interface{} `json:"attributes"`
}

func (self *Message) String() string {
	return fmt.Sprintf("[%v] %v", self.Type, self.Attributes)
}

// Chat server.
type Server struct {
	pattern   string
	messages  []*Message
	clients   map[int]*Client
	addCh     chan *Client
	delCh     chan *Client
	sendAllCh chan *Message
	doneCh    chan bool
	errCh     chan error
}

// Create new chat server.
func NewServer(pattern string) *Server {
	messages := []*Message{}
	clients := make(map[int]*Client)
	addCh := make(chan *Client)
	delCh := make(chan *Client)
	sendAllCh := make(chan *Message)
	doneCh := make(chan bool)
	errCh := make(chan error)

	return &Server{
		pattern,
		messages,
		clients,
		addCh,
		delCh,
		sendAllCh,
		doneCh,
		errCh,
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

func (s *Server) SendAll(msg *Message) {
	s.sendAllCh <- msg
}

func (s *Server) Done() {
	s.doneCh <- true
}

func (s *Server) Err(err error) {
	s.errCh <- err
}

func (s *Server) sendPastMessages(c *Client) {
	for _, msg := range s.messages {
		c.Write(msg)
	}
}

func (s *Server) sendAll(msg *Message) {
	for _, c := range s.clients {
		c.Write(msg)
	}
}

type WebSocketConnectionHandler interface {
	Authorize(username, password string) auth.SessionUser
	MessageFromClient(message map[string]interface{})
	MessageToClient(message map[string]interface{})
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
			s.sendPastMessages(c)

			// del a client
		case c := <-s.delCh:
			log.Println("Delete client")
			delete(s.clients, c.id)

			// broadcast message for all clients
		case msg := <-s.sendAllCh:
			log.Println("Send all:", msg)
			s.messages = append(s.messages, msg)
			s.sendAll(msg)

		case err := <-s.errCh:
			log.Println("Error:", err.Error())

		case <-s.doneCh:
			return
		}
	}
}
