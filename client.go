package main

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

type ClientList map[*Client]bool

type Client struct {
	// The websocket connection.
	conn *websocket.Conn

	// server is the websocket server.
	manager *Manager

	// egress is the channel on which messages are sent.
	egress chan Event

	// channel for chat
	channel string
}

var (
	pongWait     = 10 * time.Second
	pingInterval = (pongWait * 9) / 10
)

func NewClient(conn *websocket.Conn, manager *Manager) *Client {
	return &Client{
		conn:    conn,
		manager: manager,
		egress:  make(chan Event),
	}
}

func (c *Client) readMessages() {
	defer func() {
		c.manager.removeClient(c)
	}()

	// Set Max message size to 512 bytes
	c.conn.SetReadLimit(512)
	
	// Set read deadline
	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		log.Println(err)
		return
	}

	// Set pong handler
	c.conn.SetPongHandler(c.conn.PongHandler())

	for {
		_, payload, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error reading message: %v", err)
			}
			break
		}

		// Marshal incoming message into an Event.
		var event Event
		if err := json.Unmarshal(payload, &event); err != nil {
			log.Printf("error unmarshaling event: %v", err)
			break
		}
		
		// Route the event
		if err := c.manager.routeEvent(event, c); err != nil {
			log.Printf("error routing event: %v", err)
			break
		}
	}
}

func (c *Client) writeMessages() {
	ticker := time.NewTicker(pingInterval)
	defer func() {
		ticker.Stop()
		c.manager.removeClient(c)
	}()

	for {
		select {
		case event, ok := <-c.egress:
			if !ok {
				if err := c.conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
					log.Printf("error writing close message: %v", err)
				}

				return
			}

			// Marshal the event into a JSON payload.
			payload, err := json.Marshal(event)
			if err != nil {
				log.Printf("error marshaling event: %v", err)
				return
			}

			// Write the event to the websocket.
			if err := c.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				log.Printf("error writing message: %v", err)
				return
			}

			log.Printf("sent event: %v", event)
		case <-ticker.C:
			log.Println("pinging client")
			if err := c.conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				log.Printf("error writing ping message: %v", err)
				return
			}
		}
	}
}