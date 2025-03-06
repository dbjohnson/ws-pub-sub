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

	// Set up test data
	hub.messageCount["topic1"] = 100
	hub.messageCount["topic2"] = 50

	// Set lastCountTime to a specific time in the past
	hub.lastCountTime = time.Now().Add(-10 * time.Second)

	// Update message rates
	hub.updateMessageRates()

	// Check if rates were calculated correctly (approximately)
	if hub.stats.MessageRateByTopic["topic1"] < 9.5 || hub.stats.MessageRateByTopic["topic1"] > 10.5 {
		t.Errorf("Expected message rate for topic1 to be around 10, got %f", hub.stats.MessageRateByTopic["topic1"])
	}

	if hub.stats.MessageRateByTopic["topic2"] < 4.5 || hub.stats.MessageRateByTopic["topic2"] > 5.5 {
		t.Errorf("Expected message rate for topic2 to be around 5, got %f", hub.stats.MessageRateByTopic["topic2"])
	}

	// Check total rate
	if hub.stats.MessageRateTotal < 14.5 || hub.stats.MessageRateTotal > 15.5 {
		t.Errorf("Expected total message rate to be around 15, got %f", hub.stats.MessageRateTotal)
	}

	// Check that message counts were reset
	if hub.messageCount["topic1"] != 0 || hub.messageCount["topic2"] != 0 {
		t.Error("Expected message counts to be reset to 0")
	}
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
