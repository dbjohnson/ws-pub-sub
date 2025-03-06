package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

var (
	serverAddr     = flag.String("server", "localhost:8080", "Server address")
	numPublishers  = flag.Int("publishers", 1, "Number of publisher clients")
	numSubscribers = flag.Int("subscribers", 10, "Number of subscriber clients")
	numTopics      = flag.Int("topics", 5, "Number of topics")
	duration       = flag.Int("duration", 30, "Test duration in seconds")
	messageSize    = flag.Int("size", 256, "Message size in bytes")
	messagesPerSec = flag.Int("rate", 10, "Messages per second per publisher")
)

type Message struct {
	Action  string      `json:"action"`
	Topic   string      `json:"topic"`
	Payload interface{} `json:"payload"`
}

func main() {
	flag.Parse()

	// Print test configuration
	fmt.Printf("WebSocket Pub/Sub Benchmark\n")
	fmt.Printf("---------------------------\n")
	fmt.Printf("Server:            %s\n", *serverAddr)
	fmt.Printf("Publishers:        %d\n", *numPublishers)
	fmt.Printf("Subscribers:       %d\n", *numSubscribers)
	fmt.Printf("Topics:            %d\n", *numTopics)
	fmt.Printf("Message size:      %d bytes\n", *messageSize)
	fmt.Printf("Messages per sec:  %d per publisher\n", *messagesPerSec)
	fmt.Printf("Duration:          %d seconds\n", *duration)
	fmt.Printf("---------------------------\n\n")

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	var wg sync.WaitGroup
	var messagesSent uint64
	var messagesReceived uint64

	// Generate test payload
	payload := make([]byte, *messageSize)
	for i := range payload {
		payload[i] = 'a' + byte(i%26)
	}

	// Create subscriber clients
	for i := 0; i < *numSubscribers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Determine which topics this subscriber listens to
			// Each subscriber listens to a subset of topics based on its ID
			topicsToSubscribe := make([]string, 0)
			for t := 0; t < *numTopics; t++ {
				if id%(*numTopics) == t%(*numTopics) {
					topicsToSubscribe = append(topicsToSubscribe, fmt.Sprintf("topic-%d", t))
				}
			}

			subscribeClient(id, topicsToSubscribe, &messagesReceived, interrupt)
		}(i)
	}

	// Wait for subscribers to connect
	time.Sleep(1 * time.Second)

	// Create publisher clients
	for i := 0; i < *numPublishers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Each publisher publishes to all topics but in a round-robin fashion
			publishClient(id, *numTopics, &messagesSent, interrupt)
		}(i)
	}

	// Wait for the test duration or interrupt
	select {
	case <-interrupt:
		fmt.Println("Interrupted by user")
	case <-time.After(time.Duration(*duration) * time.Second):
		fmt.Println("Test completed")
	}

	// Give time for final messages to be processed
	time.Sleep(2 * time.Second)

	// Calculate results
	elapsed := float64(*duration)
	sentRate := float64(atomic.LoadUint64(&messagesSent)) / elapsed
	receivedRate := float64(atomic.LoadUint64(&messagesReceived)) / elapsed

	// Display results
	fmt.Printf("\nResults:\n")
	fmt.Printf("---------------------------\n")
	fmt.Printf("Total messages sent:     %d\n", atomic.LoadUint64(&messagesSent))
	fmt.Printf("Total messages received: %d\n", atomic.LoadUint64(&messagesReceived))
	fmt.Printf("Send rate:               %.2f msg/sec\n", sentRate)
	fmt.Printf("Receive rate:            %.2f msg/sec\n", receivedRate)
	fmt.Printf("---------------------------\n")

	// Fetch server stats
	fetchStats()
}

func fetchStats() {
	resp, err := http.Get(fmt.Sprintf("http://%s/stats", *serverAddr))
	if err != nil {
		log.Printf("Error fetching stats: %v", err)
		return
	}
	defer resp.Body.Close()

	var stats struct {
		Topics             int                `json:"topics"`
		Subscribers        int                `json:"subscribers"`
		MessageRateTotal   float64            `json:"message_rate_total"`
		MessageRateByTopic map[string]float64 `json:"message_rate_by_topic"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		log.Printf("Error decoding stats: %v", err)
		return
	}

	fmt.Printf("\nServer Stats:\n")
	fmt.Printf("---------------------------\n")
	fmt.Printf("Topics:           %d\n", stats.Topics)
	fmt.Printf("Subscribers:      %d\n", stats.Subscribers)
	fmt.Printf("Message rate:     %.2f msg/sec\n", stats.MessageRateTotal)
	fmt.Printf("---------------------------\n")
	fmt.Printf("Message rates by topic:\n")
	for topic, rate := range stats.MessageRateByTopic {
		fmt.Printf("  %s: %.2f msg/sec\n", topic, rate)
	}
}

func subscribeClient(id int, topics []string, messagesReceived *uint64, interrupt chan os.Signal) {
	u := url.URL{Scheme: "ws", Host: *serverAddr, Path: "/ws"}

	// Connect to the server
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Printf("Subscriber %d: Error connecting: %v", id, err)
		return
	}
	defer conn.Close()

	// Subscribe to topics
	for _, topic := range topics {
		subscribeMsg := Message{
			Action: "subscribe",
			Topic:  topic,
		}
		if err := conn.WriteJSON(subscribeMsg); err != nil {
			log.Printf("Subscriber %d: Error subscribing to %s: %v", id, topic, err)
			return
		}
	}

	// Message receiver
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("Subscriber %d: Error reading: %v", id, err)
				}
				return
			}
			atomic.AddUint64(messagesReceived, 1)
		}
	}()

	// Wait for interrupt
	for {
		select {
		case <-done:
			return
		case <-interrupt:
			// Gracefully close the connection
			err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Printf("Subscriber %d: Error closing: %v", id, err)
			}
			return
		}
	}
}

func publishClient(id int, numTopics int, messagesSent *uint64, interrupt chan os.Signal) {
	u := url.URL{Scheme: "ws", Host: *serverAddr, Path: "/ws"}

	// Connect to the server
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Printf("Publisher %d: Error connecting: %v", id, err)
		return
	}
	defer conn.Close()

	// Create a ticker for publishing messages
	ticker := time.NewTicker(time.Second / time.Duration(*messagesPerSec))
	defer ticker.Stop()

	topicIndex := 0
	done := make(chan struct{})

	// Message publisher
	go func() {
		defer close(done)
		for {
			select {
			case <-ticker.C:
				topic := fmt.Sprintf("topic-%d", topicIndex)
				topicIndex = (topicIndex + 1) % numTopics

				// Generate a random payload
				payload := make(map[string]interface{})
				payload["id"] = id
				payload["timestamp"] = time.Now().UnixNano()
				payload["data"] = make([]byte, *messageSize)

				publishMsg := Message{
					Action:  "publish",
					Topic:   topic,
					Payload: payload,
				}

				if err := conn.WriteJSON(publishMsg); err != nil {
					log.Printf("Publisher %d: Error publishing: %v", id, err)
					return
				}
				atomic.AddUint64(messagesSent, 1)
			case <-interrupt:
				return
			}
		}
	}()

	// Wait for interrupt
	select {
	case <-done:
		return
	case <-interrupt:
		// Gracefully close the connection
		err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			log.Printf("Publisher %d: Error closing: %v", id, err)
		}
		<-done
	}
}
