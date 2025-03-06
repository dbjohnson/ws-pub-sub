package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Config holds server configuration
type Config struct {
	Host            string
	Port            string
	ReadBufferSize  int
	WriteBufferSize int
}

// LoadConfigFromEnv loads configuration from environment variables with defaults
func LoadConfigFromEnv() Config {
	config := Config{
		Host:            getEnv("WS_HOST", ""),
		Port:            getEnv("WS_PORT", "8080"),
		ReadBufferSize:  getEnvAsInt("WS_READ_BUFFER_SIZE", 1024),
		WriteBufferSize: getEnvAsInt("WS_WRITE_BUFFER_SIZE", 1024),
	}
	return config
}

// Helper function to read environment variables with defaults
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// Helper function to read environment variables as integers with defaults
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		log.Printf("Warning: Environment variable %s is not a valid integer. Using default value %d", key, defaultValue)
		return defaultValue
	}
	return value
}

// Stats represents server statistics
type Stats struct {
	Topics             int                `json:"topics"`
	Subscribers        int                `json:"subscribers"`
	MessageRateTotal   float64            `json:"message_rate_total"`
	MessageRateByTopic map[string]float64 `json:"message_rate_by_topic"`
}

// Message represents a pub/sub message
type Message struct {
	Topic   string      `json:"topic"`
	Payload interface{} `json:"payload"`
}

// Hub maintains the set of active clients and broadcasts messages
type Hub struct {
	// Registered clients by topic
	subscribers map[string]map[*Client]bool

	// Messages to be sent to clients for a particular topic
	broadcast chan Message

	// Register requests from clients
	register chan *Subscription

	// Unregister requests from clients
	unregister chan *Subscription

	// Stats tracking
	stats         Stats
	messageCount  map[string]int
	lastCountTime time.Time
	mu            sync.Mutex

	// Message history for 1-minute window calculations
	messageHistory [60]map[string]int
	historyIndex   int
	historyFilled  bool
}

// Client is a middleman between the websocket connection and the hub
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan Message
}

// Subscription represents a client subscription to a topic
type Subscription struct {
	client *Client
	topic  string
}

// NewHub creates a new hub
func NewHub() *Hub {
	h := &Hub{
		broadcast:     make(chan Message),
		register:      make(chan *Subscription),
		unregister:    make(chan *Subscription),
		subscribers:   make(map[string]map[*Client]bool),
		messageCount:  make(map[string]int),
		lastCountTime: time.Now(),
		stats: Stats{
			MessageRateByTopic: make(map[string]float64),
		},
		historyIndex:  0,
		historyFilled: false,
	}

	// Initialize message history slots
	for i := 0; i < 60; i++ {
		h.messageHistory[i] = make(map[string]int)
	}

	return h
}

// Run starts the hub's message handling
func (h *Hub) Run() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case subscription := <-h.register:
			if _, ok := h.subscribers[subscription.topic]; !ok {
				h.subscribers[subscription.topic] = make(map[*Client]bool)
			}
			h.subscribers[subscription.topic][subscription.client] = true
			h.updateStats()

		case subscription := <-h.unregister:
			if _, ok := h.subscribers[subscription.topic]; ok {
				if _, ok := h.subscribers[subscription.topic][subscription.client]; ok {
					delete(h.subscribers[subscription.topic], subscription.client)
					if len(h.subscribers[subscription.topic]) == 0 {
						delete(h.subscribers, subscription.topic)
					}
					h.updateStats()
				}
			}

		case message := <-h.broadcast:
			h.mu.Lock()
			h.messageCount[message.Topic]++
			h.mu.Unlock()

			if clients, ok := h.subscribers[message.Topic]; ok {
				for client := range clients {
					select {
					case client.send <- message:
					default:
						close(client.send)
						delete(clients, client)
						if len(clients) == 0 {
							delete(h.subscribers, message.Topic)
						}
						h.updateStats()
					}
				}
			}

		case <-ticker.C:
			h.updateMessageRates()
		}
	}
}

// updateStats updates subscriber and topic counts
func (h *Hub) updateStats() {
	h.mu.Lock()
	defer h.mu.Unlock()

	totalSubs := 0
	for _, clients := range h.subscribers {
		totalSubs += len(clients)
	}

	h.stats.Topics = len(h.subscribers)
	h.stats.Subscribers = totalSubs
}

// updateMessageRates calculates message rates over a 1-minute window
func (h *Hub) updateMessageRates() {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()

	// Store current counts in history
	currentCounts := make(map[string]int)
	for topic, count := range h.messageCount {
		currentCounts[topic] = count
	}

	// Save current counts to history and reset current counts
	h.messageHistory[h.historyIndex] = currentCounts
	h.messageCount = make(map[string]int)

	// Update history index
	h.historyIndex = (h.historyIndex + 1) % 60
	if h.historyIndex == 0 {
		h.historyFilled = true
	}

	// Calculate rates based on 1-minute window
	totalMessages := make(map[string]int)
	for i := 0; i < 60; i++ {
		if !h.historyFilled && i >= h.historyIndex {
			break
		}

		for topic, count := range h.messageHistory[i] {
			totalMessages[topic] += count
		}
	}

	// Calculate window size in seconds
	windowSize := 60.0
	if !h.historyFilled {
		windowSize = float64(h.historyIndex)
	}

	// Only calculate rates if we have some history
	if windowSize > 0 {
		// Calculate rates
		total := 0.0
		for topic, count := range totalMessages {
			rate := float64(count) / windowSize
			h.stats.MessageRateByTopic[topic] = rate
			total += rate
		}
		h.stats.MessageRateTotal = total
	}

	h.lastCountTime = now
}

// GetStats returns the current hub statistics
func (h *Hub) GetStats() Stats {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Return a copy of the stats to avoid race conditions
	statsCopy := Stats{
		Topics:             h.stats.Topics,
		Subscribers:        h.stats.Subscribers,
		MessageRateTotal:   h.stats.MessageRateTotal,
		MessageRateByTopic: make(map[string]float64),
	}

	for k, v := range h.stats.MessageRateByTopic {
		statsCopy.MessageRateByTopic[k] = v
	}

	return statsCopy
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all connections for simplicity
	},
}

// handleWebSocket handles websocket requests from clients
func (h *Hub) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	client := &Client{hub: h, conn: conn, send: make(chan Message, 256)}

	// Start goroutines for reading from and writing to the websocket
	go client.readPump()
	go client.writePump()
}

// readPump pumps messages from the websocket to the hub
func (c *Client) readPump() {
	defer func() {
		// Unsubscribe from all topics when client disconnects
		for topic, clients := range c.hub.subscribers {
			if _, ok := clients[c]; ok {
				c.hub.unregister <- &Subscription{client: c, topic: topic}
			}
		}
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512 * 1024) // 512KB max message size
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		var msg struct {
			Action  string          `json:"action"`
			Topic   string          `json:"topic"`
			Payload json.RawMessage `json:"payload,omitempty"`
		}

		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("error unmarshaling message: %v", err)
			continue
		}

		switch msg.Action {
		case "subscribe":
			c.hub.register <- &Subscription{client: c, topic: msg.Topic}
			log.Printf("Client subscribed to topic: %s", msg.Topic)

		case "unsubscribe":
			c.hub.unregister <- &Subscription{client: c, topic: msg.Topic}
			log.Printf("Client unsubscribed from topic: %s", msg.Topic)

		case "publish":
			c.hub.broadcast <- Message{Topic: msg.Topic, Payload: msg.Payload}
			log.Printf("Message published to topic: %s", msg.Topic)

		default:
			log.Printf("Unknown action: %s", msg.Action)
		}
	}
}

// writePump pumps messages from the hub to the websocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}

			data, err := json.Marshal(message)
			if err != nil {
				log.Printf("error marshaling message: %v", err)
				return
			}
			w.Write(data)

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleStats returns the current server statistics
func (h *Hub) handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	stats := h.GetStats()
	json.NewEncoder(w).Encode(stats)
}

func main() {
	config := LoadConfigFromEnv()

	hub := NewHub()
	go hub.Run()

	upgrader = websocket.Upgrader{
		ReadBufferSize:  config.ReadBufferSize,
		WriteBufferSize: config.WriteBufferSize,
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all connections for simplicity
		},
	}

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		hub.handleWebSocket(w, r)
	})

	http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		hub.handleStats(w, r)
	})

	// Serve a simple HTML page for testing
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "static/index.html")
	})

	addr := config.Host + ":" + config.Port
	log.Printf("Starting server on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
