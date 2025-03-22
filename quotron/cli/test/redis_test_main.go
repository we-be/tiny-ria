package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	TestStream = "quotron:test:stream" // Test stream name
	StreamMaxLen = 1000               // Maximum number of messages to keep in the stream
	TestConsumerGroup = "quotron:test:group" // Consumer group for testing
	TestConsumer = "test-consumer"    // Consumer ID for testing
)

type TestMessage struct {
	ID    string    `json:"id"`
	Text  string    `json:"text"`
	Time  time.Time `json:"time"`
}

func main() {
	// Connect to Redis
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	
	ctx := context.Background()
	
	// Test Redis connection
	pong, err := client.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	fmt.Printf("Redis connection established: %s\n", pong)
	
	// Create stream and consumer group if they don't exist
	_, err = client.XGroupCreateMkStream(ctx, TestStream, TestConsumerGroup, "0").Result()
	if err != nil {
		if !redis.HasErrorPrefix(err, "BUSYGROUP") {
			log.Printf("Warning: Could not create consumer group: %v", err)
		} else {
			fmt.Printf("Using existing consumer group %s for stream %s\n", TestConsumerGroup, TestStream)
		}
	} else {
		fmt.Printf("Created consumer group %s for stream %s\n", TestConsumerGroup, TestStream)
	}
	
	// In a separate goroutine, receive messages
	go func() {
		for {
			// Read from stream with 1-second blocking timeout
			streams, err := client.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    TestConsumerGroup,
				Consumer: TestConsumer,
				Streams:  []string{TestStream, ">"}, // ">" means only undelivered messages
				Count:    10, // Process up to 10 messages at a time
				Block:    1 * time.Second,
			}).Result()
			
			if err != nil {
				if err == redis.Nil || err.Error() == "redis: nil" {
					// No messages available, just continue
					continue
				}
				
				fmt.Printf("Error reading from stream: %v\n", err)
				time.Sleep(1 * time.Second) // Avoid tight loops on errors
				continue
			}
			
			// Process messages
			for _, stream := range streams {
				for _, message := range stream.Messages {
					// Extract payload
					data, ok := message.Values["data"].(string)
					if !ok {
						fmt.Printf("Invalid message format, no data field: %v\n", message.Values)
						// Acknowledge message even if invalid
						client.XAck(ctx, TestStream, TestConsumerGroup, message.ID)
						continue
					}
					
					fmt.Printf("Received message with ID %s: %s\n", message.ID, data)
					
					var testMsg TestMessage
					if err := json.Unmarshal([]byte(data), &testMsg); err != nil {
						fmt.Printf("Failed to unmarshal message: %v\n", err)
					} else {
						fmt.Printf("Parsed message: ID=%s, Text=%s, Time=%v\n", 
							testMsg.ID, testMsg.Text, testMsg.Time)
					}
					
					// Acknowledge message
					client.XAck(ctx, TestStream, TestConsumerGroup, message.ID)
				}
			}
		}
	}()
	
	// Wait for consumer setup
	time.Sleep(1 * time.Second)
	
	// Publish a few test messages
	for i := 1; i <= 3; i++ {
		testMsg := TestMessage{
			ID:    fmt.Sprintf("msg-%d", i),
			Text:  fmt.Sprintf("Test message #%d", i),
			Time:  time.Now(),
		}
		
		data, err := json.Marshal(testMsg)
		if err != nil {
			log.Fatalf("Failed to marshal message: %v", err)
		}
		
		fmt.Printf("Publishing message to stream: %s\n", string(data))
		
		// Publish to stream
		result, err := client.XAdd(ctx, &redis.XAddArgs{
			Stream: TestStream,
			ID:     "*", // Auto-generate ID
			Values: map[string]interface{}{
				"data": string(data),
			},
			MaxLen: StreamMaxLen,
		}).Result()
		
		if err != nil {
			log.Fatalf("Failed to publish message to stream: %v", err)
		}
		
		fmt.Printf("Message published with ID: %s\n", result)
		
		time.Sleep(1 * time.Second)
	}
	
	// Wait for messages to be processed
	time.Sleep(5 * time.Second)
	fmt.Println("Test completed")
}