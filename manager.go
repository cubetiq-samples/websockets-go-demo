package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	websocketUpgrader = websocket.Upgrader{
		CheckOrigin:     checkOrigin,
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

var (
	ErrEventNotSupported = errors.New("event not supported")
)

func checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")

	switch origin {
	case "https://localhost:8080":
		return true
	default:
		return false
	}
}

type Manager struct {
	clients ClientList
	sync.RWMutex
	handlers map[string]EventHandler
	opts     RetentionMap
}

func NewManager(ctx context.Context) *Manager {
	m := &Manager{
		clients:  make(ClientList),
		handlers: make(map[string]EventHandler),
		opts:     NewRetentionMap(ctx, 5*time.Minute),
	}

	m.setupHandlers()
	return m
}

func (m *Manager) setupHandlers() {
	m.handlers[EventSendMessage] = SendMessageHandler
	m.handlers[EventChangeChannel] = ChangeChannelHandler
}

func (m *Manager) loginHandler(w http.ResponseWriter, r *http.Request) {
	type loginRequest struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Username != "admin" || req.Password != "admin" {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	type loginResponse struct {
		OTP string `json:"otp"`
	}

	// Add a new OTP
	otp := m.opts.NewOTP()
	resp := loginResponse{
		OTP: otp.Key,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (m *Manager) wsHandler(w http.ResponseWriter, r *http.Request) {
	otp := r.URL.Query().Get("otp")
	if otp == "" {
		http.Error(w, "otp is required", http.StatusBadRequest)
		return
	}

	if !m.opts.VerifyOTP(otp) {
		http.Error(w, "invalid otp", http.StatusUnauthorized)
		return
	}

	log.Println("websocket connection established")
	conn, err := websocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("websocket connection error:", err)
		return
	}

	// Create a new client
	client := NewClient(conn, m)
	m.addClient(client)

	// Start the client
	go client.readMessages()
	go client.writeMessages()

}

func (m *Manager) addClient(client *Client) {
	m.Lock()
	defer m.Unlock()

	m.clients[client] = true
}

func (m *Manager) removeClient(client *Client) {
	m.Lock()
	defer m.Unlock()

	if _, ok := m.clients[client]; ok {
		// close connection
		client.conn.Close()

		// remove
		delete(m.clients, client)
		close(client.egress)
	}
}

func (m *Manager) routeEvent(event Event, c *Client) error {
	if handler, ok := m.handlers[event.Type]; ok {
		if err := handler(event, c); err != nil {
			log.Println("error handling event:", err)
			return err
		}

		return nil
	} else {
		return ErrEventNotSupported
	}
}