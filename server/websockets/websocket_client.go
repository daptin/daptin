package websockets

import (
	"errors"
	"fmt"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/resource"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/websocket"
	"io"
)

const channelBufSize = 100

var maxId int = 0

type Client struct {
	id                         int
	ws                         *websocket.Conn
	server                     *Server
	ch                         chan resource.EventMessage
	doneCh                     chan bool
	user                       *auth.SessionUser
	webSocketConnectionHandler WebSocketConnectionHandlerImpl
}

// Create new chat client.
func NewClient(ws *websocket.Conn, server *Server) (*Client, error) {

	if ws == nil {
		panic("ws cannot be nil")
	}

	if server == nil {
		panic("server cannot be nil")
	}

	webSocketConnectionHandler := WebSocketConnectionHandlerImpl{
		DtopicMap:        server.dtopicMap,
		subscribedTopics: make(map[string]uint64),
	}

	maxId++
	ch := make(chan resource.EventMessage, channelBufSize)
	doneCh := make(chan bool)

	u := ws.Request().Context().Value("user")
	if u == nil {
		return nil, errors.New("unauthorized")
	}
	user := u.(*auth.SessionUser)
	return &Client{
		id:                         maxId,
		ws:                         ws,
		server:                     server,
		ch:                         ch,
		doneCh:                     doneCh,
		user:                       user,
		webSocketConnectionHandler: webSocketConnectionHandler,
	}, nil
}

func (c *Client) Conn() *websocket.Conn {
	return c.ws
}

func (c *Client) Write(msg resource.EventMessage) {
	select {
	case c.ch <- msg:
	default:
		c.server.Del(c)
		err := fmt.Errorf("client %d is disconnected.", c.id)
		c.server.Err(err)
	}
}

func (c *Client) Done() {
	c.doneCh <- true
}

// Listen Write and Read request via chanel
func (c *Client) Listen() {
	go c.listenWrite()
	c.listenRead()
}

// Listen write request via chanel
func (c *Client) listenWrite() {
	for {
		select {

		// send message to the client
		case msg := <-c.ch:
			log.Println("Send:", msg)
			err := websocket.JSON.Send(c.ws, msg)
			if err != nil {
				log.Printf("Failed to to send message: %v", err)
			}

			// receive done request
		case <-c.doneCh:
			c.server.Del(c)
			c.doneCh <- true // for listenRead method
			return
		}
	}
}

// Listen read request via chanel
func (c *Client) listenRead() {
	log.Println("Listening read from client")
	for {
		select {

		// receive done request
		case <-c.doneCh:
			c.server.Del(c)
			c.doneCh <- true // for listenWrite method
			return

			// read data from websocket connection
		default:
			var msg WebSocketPayload
			err := websocket.JSON.Receive(c.ws, &msg)
			if err == io.EOF {
				c.doneCh <- true
			} else if err != nil {
				c.server.Err(err)
			} else {
				// everything went well, we have the message here
				// TODO: process the incoming message
				c.webSocketConnectionHandler.MessageFromClient(msg, c)
			}
		}
	}
}
