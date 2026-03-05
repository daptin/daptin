package websockets

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/resource"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/websocket"
)

const readDeadline = 5 * time.Minute

const channelBufSize = 512

var maxId atomic.Int64

type Client struct {
	id                         int
	ws                         *websocket.Conn
	server                     *Server
	ch                         chan resource.WsOutMessage
	doneCh                     chan bool
	user                       *auth.SessionUser
	webSocketConnectionHandler WebSocketConnectionHandler
}

// Create new chat client.
func NewClient(ws *websocket.Conn, server *Server) (*Client, error) {

	if ws == nil {
		panic("ws cannot be nil")
	}

	if server == nil {
		panic("server cannot be nil")
	}

	webSocketConnectionHandler := &WebSocketConnectionHandlerImpl{
		DtopicMap:        server.dtopicMap,
		dtopicMapLock:    &server.dtopicMapLock,
		subscribedTopics: make(map[string]*PubSubEntry),
		olricDb:          server.olricDb,
		cruds:            server.cruds,
		sharedPubSub:     server.sharedPubSub,
	}

	id := int(maxId.Add(1))
	ch := make(chan resource.WsOutMessage, channelBufSize)
	doneCh := make(chan bool, 2)

	u := ws.Request().Context().Value("user")
	if u == nil {
		return nil, errors.New("{\"message\": \"unauthorized\"}")
	}
	user := u.(*auth.SessionUser)

	client := &Client{
		id:                         id,
		ws:                         ws,
		server:                     server,
		ch:                         ch,
		doneCh:                     doneCh,
		user:                       user,
		webSocketConnectionHandler: webSocketConnectionHandler,
	}

	// Send session-open message
	groups := make([]string, 0, len(user.Groups))
	for _, g := range user.Groups {
		groups = append(groups, g.GroupReferenceId.String())
	}
	sessionData, _ := json.Marshal(map[string]interface{}{
		"user":      user.UserReferenceId.String(),
		"groups":    groups,
		"sessionId": id,
	})
	client.Write(resource.WsOutMessage{
		Type:   "session",
		Status: "open",
		Data:   sessionData,
	})

	return client, nil
}

func (c *Client) Conn() *websocket.Conn {
	return c.ws
}

func (c *Client) Write(msg resource.WsOutMessage) {
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

func (c *Client) Close() {
	c.webSocketConnectionHandler.Close()
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
			c.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
			err := websocket.JSON.Send(c.ws, msg)
			if err != nil {
				log.Printf("Failed to send message: %v", err)
				c.server.Del(c)
				return
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
	log.Println("[114] websocket listening read from client")
	for {
		select {

		// receive done request
		case <-c.doneCh:
			c.server.Del(c)
			c.doneCh <- true // for listenWrite method
			return

			// read data from websocket connection
		default:
			c.ws.SetReadDeadline(time.Now().Add(readDeadline))
			var msg WebSocketPayload
			err := websocket.JSON.Receive(c.ws, &msg)
			if err == io.EOF {
				c.doneCh <- true
				return
			} else if err != nil {
				c.server.Err(err)
				c.doneCh <- true
				return
			} else if msg.Method == "ping" {
				c.Write(resource.WsOutMessage{Type: "pong"})
			} else {
				c.webSocketConnectionHandler.MessageFromClient(msg, c)
			}
		}
	}
}
