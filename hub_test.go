package main

import (
	"reflect"
	"testing"
	"time"
)

func TestNewHub(t *testing.T) {
	hub := NewHub()

	if hub.broadcast == nil {
		t.Error("Expected broadcast channel to be initialized")
	}

	if hub.register == nil {
		t.Error("Expected register channel to be initialized")
	}

	if hub.unregister == nil {
		t.Error("Expected unregister channel to be initialized")
	}

	if hub.subscribers == nil {
		t.Error("Expected subscribers map to be initialized")
	}

	if hub.messageCount == nil {
		t.Error("Expected messageCount map to be initialized")
	}

	if hub.stats.MessageRateByTopic == nil {
		t.Error("Expected MessageRateByTopic map to be initialized")
	}

	if reflect.TypeOf(hub.lastCountTime) != reflect.TypeOf(time.Time{}) {
		t.Error("Expected lastCountTime to be a time.Time")
	}
}

func TestUpdateStats(t *testing.T) {
	hub := NewHub()

	// Add some test subscribers
	client1 := &Client{}
	client2 := &Client{}
	client3 := &Client{}

	// Initialize subscriber maps
	topic1 := "test-topic-1"
	topic2 := "test-topic-2"

	hub.subscribers[topic1] = make(map[*Client]bool)
	hub.subscribers[topic2] = make(map[*Client]bool)

	// Add subscribers to topics
	hub.subscribers[topic1][client1] = true
	hub.subscribers[topic1][client2] = true
	hub.subscribers[topic2][client3] = true

	// Update stats
	hub.updateStats()

	// Verify stats
	if hub.stats.Topics != 2 {
		t.Errorf("Expected 2 topics, got %d", hub.stats.Topics)
	}

	if hub.stats.Subscribers != 3 {
		t.Errorf("Expected 3 subscribers, got %d", hub.stats.Subscribers)
	}
}

func TestUpdateMessageRates(t *testing.T) {
	hub := NewHub()

	// Helper function to compare floating point values with a delta
	assertFloat := func(expected, actual float64, delta float64, msg string) {
		if actual < expected-delta || actual > expected+delta {
			t.Errorf("%s: expected %.2f (±%.2f), got %.2f", msg, expected, delta, actual)
		}
	}

	// Simulate message processing over several time intervals

	// First interval: 100 messages for topic1, 50 for topic2
	hub.messageCount["topic1"] = 100
	hub.messageCount["topic2"] = 50
	hub.updateMessageRates() // This stores the first second of data

	// At this point, we have 1 second of history
	// Expected rates: topic1=100/1, topic2=50/1
	assertFloat(100.0, hub.stats.MessageRateByTopic["topic1"], 0.001, "First interval: Rate for topic1")
	assertFloat(50.0, hub.stats.MessageRateByTopic["topic2"], 0.001, "First interval: Rate for topic2")
	assertFloat(150.0, hub.stats.MessageRateTotal, 0.001, "First interval: Total rate")

	// Check that message counts were reset
	if len(hub.messageCount) != 0 {
		t.Error("Expected message counts to be reset to empty map")
	}

	// Second interval: 50 messages for topic1, 0 for topic2, 25 for topic3
	hub.messageCount["topic1"] = 50
	hub.messageCount["topic3"] = 25
	hub.updateMessageRates() // This stores the second second of data

	// At this point, we have 2 seconds of history
	// Expected average rates:
	// topic1: (100+50)/2 = 75 per second
	// topic2: (50+0)/2 = 25 per second
	// topic3: (0+25)/2 = 12.5 per second
	assertFloat(75.0, hub.stats.MessageRateByTopic["topic1"], 0.001, "Second interval: Rate for topic1")
	assertFloat(25.0, hub.stats.MessageRateByTopic["topic2"], 0.001, "Second interval: Rate for topic2")
	assertFloat(12.5, hub.stats.MessageRateByTopic["topic3"], 0.001, "Second interval: Rate for topic3")
	assertFloat(112.5, hub.stats.MessageRateTotal, 0.001, "Second interval: Total rate")

	// Third interval: simulate a longer time period with multiple updates
	// Add 5 more updates with varying message counts
	messagePatterns := []map[string]int{
		{"topic1": 30, "topic2": 20},               // 3rd second
		{"topic1": 40, "topic3": 10},               // 4th second
		{"topic2": 60, "topic3": 20},               // 5th second
		{"topic1": 20, "topic2": 30, "topic3": 40}, // 6th second
		{"topic1": 10, "topic2": 10, "topic3": 10}, // 7th second
	}

	for _, pattern := range messagePatterns {
		// Set message counts for this interval
		for topic, count := range pattern {
			hub.messageCount[topic] = count
		}
		hub.updateMessageRates()
	}

	// At this point, we have 7 seconds of history
	// Calculate expected rates manually
	topic1Total := 100 + 50 + 30 + 40 + 0 + 20 + 10 // = 250
	topic2Total := 50 + 0 + 20 + 0 + 60 + 30 + 10   // = 170
	topic3Total := 0 + 25 + 0 + 10 + 20 + 40 + 10   // = 105

	expectedTopic1Rate := float64(topic1Total) / 7.0 // = 35.71
	expectedTopic2Rate := float64(topic2Total) / 7.0 // = 24.29
	expectedTopic3Rate := float64(topic3Total) / 7.0 // = 15.0
	expectedTotalRate := expectedTopic1Rate + expectedTopic2Rate + expectedTopic3Rate

	// Verify rates
	assertFloat(expectedTopic1Rate, hub.stats.MessageRateByTopic["topic1"], 0.01, "Multi-interval: Rate for topic1")
	assertFloat(expectedTopic2Rate, hub.stats.MessageRateByTopic["topic2"], 0.01, "Multi-interval: Rate for topic2")
	assertFloat(expectedTopic3Rate, hub.stats.MessageRateByTopic["topic3"], 0.01, "Multi-interval: Rate for topic3")
	assertFloat(expectedTotalRate, hub.stats.MessageRateTotal, 0.01, "Multi-interval: Total rate")

	// Test that old data rolls off when we exceed 60 seconds
	// For simplicity, let's just overwrite our history index
	// and set historyFilled to simulate a complete buffer
	hub.historyIndex = 0
	hub.historyFilled = true

	// Clear all history slots
	for i := 0; i < 60; i++ {
		hub.messageHistory[i] = make(map[string]int)
	}

	// Add a consistent pattern over the full 60-second window
	// Each second has 60 messages for topic1
	for i := 0; i < 60; i++ {
		hub.messageHistory[i] = map[string]int{"topic1": 60}
	}

	// Now add a new data point - this should push out the oldest entry
	hub.messageCount["topic1"] = 120 // Double the rate for latest second
	hub.updateMessageRates()

	// We replaced one second of 60 messages with one second of 120 messages
	// So the average should be slightly higher: (59*60 + 120)/60 = 61
	assertFloat(61.0, hub.stats.MessageRateByTopic["topic1"], 0.01, "Sliding window: Rate for topic1")
}

func TestGetStats(t *testing.T) {
	hub := NewHub()

	// Set up test data
	hub.stats.Topics = 5
	hub.stats.Subscribers = 10
	hub.stats.MessageRateTotal = 15.5
	hub.stats.MessageRateByTopic["topic1"] = 7.5
	hub.stats.MessageRateByTopic["topic2"] = 8.0

	// Get stats
	stats := hub.GetStats()

	// Verify stats
	if stats.Topics != 5 {
		t.Errorf("Expected 5 topics, got %d", stats.Topics)
	}

	if stats.Subscribers != 10 {
		t.Errorf("Expected 10 subscribers, got %d", stats.Subscribers)
	}

	if stats.MessageRateTotal != 15.5 {
		t.Errorf("Expected total message rate of 15.5, got %f", stats.MessageRateTotal)
	}

	if stats.MessageRateByTopic["topic1"] != 7.5 {
		t.Errorf("Expected message rate for topic1 to be 7.5, got %f", stats.MessageRateByTopic["topic1"])
	}

	if stats.MessageRateByTopic["topic2"] != 8.0 {
		t.Errorf("Expected message rate for topic2 to be 8.0, got %f", stats.MessageRateByTopic["topic2"])
	}

	// Modify original stats to verify we got a copy
	hub.stats.Topics = 100

	if stats.Topics == 100 {
		t.Error("Expected GetStats to return a copy, not a reference")
	}
}
