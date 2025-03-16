package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	TestChannel = "quotron:test:channel"
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
	
	// Create a pub/sub
	pubsub := client.Subscribe(ctx, TestChannel)
	defer pubsub.Close()
	
	// In a separate goroutine, receive messages
	go func() {
		ch := pubsub.Channel()
		
		fmt.Println("Subscriber is waiting for messages...")
		for msg := range ch {
			fmt.Printf("Received message: %s\n", msg.Payload)
			
			var testMsg TestMessage
			if err := json.Unmarshal([]byte(msg.Payload), &testMsg); err != nil {
				fmt.Printf("Failed to unmarshal message: %v\n", err)
				continue
			}
			
			fmt.Printf("Parsed message: ID=%s, Text=%s, Time=%v\n", 
			    testMsg.ID, testMsg.Text, testMsg.Time)
		}
	}()
	
	// Wait for subscription to be established
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
		
		fmt.Printf("Publishing message: %s\n", string(data))
		
		result := client.Publish(ctx, TestChannel, string(data))
		if err := result.Err(); err != nil {
			log.Fatalf("Failed to publish message: %v", err)
		}
		
		receivers, err := result.Result()
		if err != nil {
			fmt.Printf("Failed to get publish result: %v\n", err)
		} else {
			fmt.Printf("Message published to %d clients\n", receivers)
		}
		
		time.Sleep(1 * time.Second)
	}
	
	// Wait for messages to be processed
	time.Sleep(5 * time.Second)
	fmt.Println("Test completed")
}